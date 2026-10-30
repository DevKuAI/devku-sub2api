//go:build integration

package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

const channelTestdata = "../accounting/channel/testdata"

type uploadFile struct {
	name string
	data []byte
}

func sunrateUpload(t *testing.T) []uploadFile {
	t.Helper()
	entries, err := os.ReadDir(channelTestdata)
	require.NoError(t, err)
	var files []uploadFile
	for _, e := range entries {
		data, readErr := os.ReadFile(filepath.Join(channelTestdata, e.Name()))
		require.NoError(t, readErr)
		files = append(files, uploadFile{name: e.Name(), data: data})
	}
	return files
}

func templateUpload(name string, rows ...string) []uploadFile {
	header := "platform,account_ref,wallet,external_id,occurred_at,type,amount,currency,balance_after,fee_amount,card_ref,card_last4,card_name,merchant,related_id,original_amount,original_currency,remark"
	return []uploadFile{{name: name, data: []byte(header + "\n" + strings.Join(rows, "\n") + "\n")}}
}

func (f *accountingFixture) upload(t *testing.T, path, key, role string, files []uploadFile) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, file := range files {
		part, err := w.CreateFormFile("files", file.name)
		require.NoError(t, err)
		_, err = part.Write(file.data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	req := httptest.NewRequest("POST", "/api/v1/admin/accounting/"+path, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Idempotency-Key", key)
	if role != "" {
		req.Header.Set("X-Test-Role", role)
	}
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	return res
}

func (f *accountingFixture) requestAs(role, method, path, key string, payload any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(method, "/api/v1/admin/accounting/"+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("X-Test-Role", role)
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	return res
}

func cleanChannelAccounts(t *testing.T, refs ...string) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{
		"UPDATE accounting_channel_lines SET pair_line_id=NULL WHERE channel_account_id IN (SELECT id FROM accounting_channel_accounts WHERE account_ref=ANY($1))",
		"DELETE FROM accounting_channel_lines WHERE channel_account_id IN (SELECT id FROM accounting_channel_accounts WHERE account_ref=ANY($1))",
		"DELETE FROM accounting_channel_cards WHERE channel_account_id IN (SELECT id FROM accounting_channel_accounts WHERE account_ref=ANY($1))",
		"DELETE FROM accounting_channel_attachments WHERE channel_account_id IN (SELECT id FROM accounting_channel_accounts WHERE account_ref=ANY($1))",
		"DELETE FROM accounting_channel_imports WHERE account_ids ?| ARRAY(SELECT id::text FROM accounting_channel_accounts WHERE account_ref=ANY($1))",
		"DELETE FROM accounting_channel_accounts WHERE account_ref=ANY($1)",
	} {
		_, err := integrationDB.ExecContext(ctx, q, pq.Array(refs))
		require.NoError(t, err)
	}
}

func newChannelFixture(t *testing.T, refs ...string) *accountingFixture {
	f := newAccountingFixture(t)
	cleanChannelAccounts(t, refs...)
	t.Cleanup(func() { cleanChannelAccounts(t, refs...) })
	return f
}

func wallets(summary accounting.ChannelAccountSummary) map[string]accounting.ChannelWallet {
	out := map[string]accounting.ChannelWallet{}
	for _, w := range summary.Wallets {
		out[w.Wallet+"/"+w.Currency] = w
	}
	return out
}

func TestAccountingChannelImportReconcilesSunrateExports(t *testing.T) {
	f := newChannelFixture(t, "72778936")
	key := uuid.NewString()
	var result accounting.ChannelImportResult
	require.NoError(t, json.Unmarshal(accountingData(t, f.upload(t, "channel-imports", key, "", sunrateUpload(t))), &result))
	require.Equal(t, "SUNRATE", result.Import.Converter)
	require.Equal(t, 36, result.Import.NewRows)
	require.Zero(t, result.Import.DuplicateRows)
	require.Len(t, result.Import.Files, 7)
	require.Len(t, result.Accounts, 1)
	summary := result.Accounts[0]
	require.Equal(t, "Hong Kong Rich Sum Technology Co. Limited", summary.Account.Name)
	ws := wallets(summary)
	for k, want := range map[string]string{"PAYMENT/HKD": "0.00", "PAYMENT/USD": "2172.87", "CARD/USD": "1026.50"} {
		require.Equal(t, want, ws[k].Balance, k)
		require.NotNil(t, ws[k].Reported, k)
		require.True(t, ws[k].Reported.Matched, k)
		require.Empty(t, ws[k].Gaps, k)
	}
	// Funds flow order: HKD collections first, then the USD payment balance, then the card balance.
	require.Equal(t, []string{"PAYMENT/HKD", "PAYMENT/USD", "CARD/USD"}, []string{
		summary.Wallets[0].Wallet + "/" + summary.Wallets[0].Currency,
		summary.Wallets[1].Wallet + "/" + summary.Wallets[1].Currency,
		summary.Wallets[2].Wallet + "/" + summary.Wallets[2].Currency,
	})
	require.Len(t, ws["CARD/USD"].NegativeSpans, 1)
	require.Equal(t, "-1.00", ws["CARD/USD"].NegativeSpans[0].Lowest)
	require.Equal(t, []accounting.ChannelAmount{{Currency: "HKD", Amount: "0.00"}, {Currency: "USD", Amount: "3199.37"}}, summary.Positions)
	require.Equal(t, []accounting.ChannelAmount{{Currency: "HKD", Amount: "34.64"}, {Currency: "USD", Amount: "4.25"}}, summary.FeeCosts)
	require.Zero(t, summary.UnpairedTransfers)
	require.Equal(t, 4, summary.Cards)

	replay := f.upload(t, "channel-imports", key, "", sunrateUpload(t))
	require.Equal(t, 200, replay.Code)
	require.Equal(t, "true", replay.Header().Get("X-Idempotency-Replayed"))

	var again accounting.ChannelImportResult
	require.NoError(t, json.Unmarshal(accountingData(t, f.upload(t, "channel-imports", uuid.NewString(), "", sunrateUpload(t))), &again))
	require.Zero(t, again.Import.NewRows)
	require.Equal(t, 36, again.Import.DuplicateRows)

	account := summary.Account.ID
	var detail accounting.ChannelAccountDetail
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "channel-accounts/"+account, "", nil)), &detail))
	require.Len(t, detail.Conversions, 3)
	require.Equal(t, "7.8599", detail.Conversions[0].Rate)
	require.Equal(t, "23439.81", detail.Conversions[0].Sold)
	require.Equal(t, "2982.20", detail.Conversions[0].Bought)
	require.Len(t, detail.CardSpend, 4)
	require.Equal(t, "400.00", detail.CardSpend[0].Spend)
	require.Empty(t, detail.Unpaired)
	require.Len(t, detail.Imports, 2)
	var october accounting.ChannelMonth
	for _, m := range detail.Months {
		if m.Month == "2026-10" && m.Currency == "USD" {
			october = m
		}
	}
	require.Equal(t, "2982.20", october.Totals["FX_IN"])
	require.Equal(t, "-1429.00", october.Totals["TRANSFER_OUT"])

	var lines []accounting.ChannelLine
	var page struct {
		Items json.RawMessage `json:"items"`
		Total int             `json:"total"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "channel-accounts/"+account+"/lines?type=TRANSFER_OUT", "", nil)), &page))
	require.NoError(t, json.Unmarshal(page.Items, &lines))
	require.Equal(t, 5, page.Total)
	for _, l := range lines {
		require.NotNil(t, l.Pair)
		require.Equal(t, "CARD", l.Pair.Wallet)
	}

	// Only administrators revoke, newest batch first: the duplicate batch relied on the first one.
	revoke := map[string]string{"reason": "re-export"}
	require.Equal(t, 403, f.requestAs("finance", "POST", "channel-imports/"+again.Import.ID+"/revoke", uuid.NewString(), revoke).Code)
	require.Equal(t, 409, f.request("POST", "channel-imports/"+result.Import.ID+"/revoke", uuid.NewString(), revoke).Code)
	var revoked accounting.ChannelImport
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "channel-imports/"+again.Import.ID+"/revoke", uuid.NewString(), revoke)), &revoked))
	require.Equal(t, "REVOKED", revoked.Status)
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "channel-imports/"+result.Import.ID+"/revoke", uuid.NewString(), revoke)), &revoked))
	require.Equal(t, "REVOKED", revoked.Status)
	var empty []accounting.ChannelAccountSummary
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "channel-accounts", "", nil)), &empty))
	for _, s := range empty {
		if s.Account.ID == account {
			require.Empty(t, s.Wallets)
		}
	}
	var reimported accounting.ChannelImportResult
	require.NoError(t, json.Unmarshal(accountingData(t, f.upload(t, "channel-imports", uuid.NewString(), "", sunrateUpload(t))), &reimported))
	require.Equal(t, 36, reimported.Import.NewRows)
	require.True(t, wallets(reimported.Accounts[0])["PAYMENT/USD"].Reported.Matched)
	require.Equal(t, 409, f.request("POST", "channel-imports/"+result.Import.ID+"/revoke", uuid.NewString(), revoke).Code)
}

func TestAccountingChannelTemplateConflictsGapsAndPairing(t *testing.T) {
	f := newChannelFixture(t, "T-1")
	first := templateUpload("first.csv",
		"TEST,T-1,MAIN,a,2026-10-01 10:00:00+08:00,FUNDING_IN,100.00,USD,100.00,0.50,,,,,,,,",
		"TEST,T-1,MAIN,b,2026-10-02 10:00:00+08:00,TRANSFER_OUT,-40.00,USD,60.00,,,,,,,,,",
		"TEST,T-1,CARD,c,2026-10-02 11:00:00+08:00,TRANSFER_IN,40.00,USD,40.00,,,,,,,,,",
	)
	var result accounting.ChannelImportResult
	require.NoError(t, json.Unmarshal(accountingData(t, f.upload(t, "channel-imports", uuid.NewString(), "", first)), &result))
	require.Equal(t, "TEMPLATE", result.Import.Converter)
	require.Zero(t, result.Accounts[0].UnpairedTransfers, "transfers without references pair by amount and day")

	conflict := templateUpload("conflict.csv", "TEST,T-1,MAIN,a,2026-10-01 10:00:00+08:00,FUNDING_IN,101.00,USD,101.00,,,,,,,,,")
	require.Equal(t, 409, f.upload(t, "channel-imports", uuid.NewString(), "", conflict).Code)

	// A later batch that does not continue from the books is accepted but shows the gap.
	later := templateUpload("later.csv", "TEST,T-1,MAIN,e,2026-10-05 10:00:00+08:00,FUNDING_IN,10.00,USD,90.00,,,,,,,,,")
	require.NoError(t, json.Unmarshal(accountingData(t, f.upload(t, "channel-imports", uuid.NewString(), "", later)), &result))
	main := wallets(result.Accounts[0])["MAIN/USD"]
	require.Len(t, main.Gaps, 1)
	require.Equal(t, "b", main.Gaps[0].AfterID)
	require.Equal(t, "20.00", main.Gaps[0].Difference)
	require.Equal(t, "70.00", main.Balance)

	fx := templateUpload("fx.csv", "TEST,T-1,MAIN,fx-1,2026-10-06 10:00:00+08:00,FX_OUT,-10.00,USD,80.00,,,,,,fx-1,,,")
	res := f.upload(t, "channel-imports", uuid.NewString(), "", fx)
	require.Equal(t, 400, res.Code)
	require.Contains(t, res.Body.String(), "conversion fx-1 is missing")

	invalid := templateUpload("bad.csv", "TEST,T-1,MAIN,x,2026-10-07 10:00:00+08:00,CONSUME,10.00,USD,,,c,,,,,,,")
	res = f.upload(t, "channel-imports", uuid.NewString(), "", invalid)
	require.Equal(t, 400, res.Code)
	require.Contains(t, res.Body.String(), "ACCOUNTING_CHANNEL_FILES_INVALID")
}

func (f *accountingFixture) uploadProof(t *testing.T, line, name string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", name)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	req := httptest.NewRequest("POST", "/api/v1/admin/accounting/channel-lines/"+line+"/proofs", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Idempotency-Key", uuid.NewString())
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	return res
}

func (f *accountingFixture) collections(t *testing.T, account, filter string) []accounting.ChannelLine {
	t.Helper()
	var page struct {
		Items []accounting.ChannelLine `json:"items"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "channel-accounts/"+account+"/lines?type="+filter, "", nil)), &page))
	return page.Items
}

func TestAccountingChannelProofsFollowThePlatformReference(t *testing.T) {
	f := newChannelFixture(t, "72778936")
	var result accounting.ChannelImportResult
	require.NoError(t, json.Unmarshal(accountingData(t, f.upload(t, "channel-imports", uuid.NewString(), "", sunrateUpload(t))), &result))
	account := result.Accounts[0].Account.ID
	require.Equal(t, 2, result.Accounts[0].MissingProofs)
	missing := f.collections(t, account, "MISSING_PROOF")
	require.Len(t, missing, 2)
	var line accounting.ChannelLine
	for _, l := range missing {
		if l.ExternalID == "C26A26JRZ9" {
			line = l
		}
	}
	require.NotEmpty(t, line.ID)

	// The stored type comes from the content, so a renamed text file is refused.
	require.Equal(t, 400, f.uploadProof(t, line.ID, "transfer.png", []byte("not an image")).Code)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	var proof accounting.ChannelProof
	require.NoError(t, json.Unmarshal(accountingData(t, f.uploadProof(t, line.ID, "银行转账截图.png", png)), &proof))
	require.Equal(t, "image/png", proof.ContentType)
	require.Len(t, f.collections(t, account, "MISSING_PROOF"), 1)

	// Revoking and re-importing creates new lines; the proof stays with the platform reference.
	revoke := map[string]string{"reason": "re-export"}
	require.Equal(t, 200, f.request("POST", "channel-imports/"+result.Import.ID+"/revoke", uuid.NewString(), revoke).Code)
	require.NoError(t, json.Unmarshal(accountingData(t, f.upload(t, "channel-imports", uuid.NewString(), "", sunrateUpload(t))), &result))
	require.Equal(t, 1, result.Accounts[0].MissingProofs)
	var kept *accounting.ChannelLine
	for _, l := range f.collections(t, account, "FUNDING_IN") {
		if l.ExternalID == "C26A26JRZ9" {
			kept = &l
		}
	}
	require.NotNil(t, kept)
	require.NotEqual(t, line.ID, kept.ID)
	require.Len(t, kept.Proofs, 1)
	require.Equal(t, "银行转账截图.png", kept.Proofs[0].Filename)

	var download struct {
		URL string `json:"url"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "channel-proofs/"+proof.ID+"/download-url", "", nil)), &download))
	require.Contains(t, download.URL, "accounting/channel/proofs/"+proof.ID)
	require.Equal(t, 200, f.request("DELETE", "channel-proofs/"+proof.ID, uuid.NewString(), nil).Code)
	require.Len(t, f.collections(t, account, "MISSING_PROOF"), 2)
	require.Equal(t, 404, f.request("DELETE", "channel-proofs/"+proof.ID, uuid.NewString(), nil).Code)
}
