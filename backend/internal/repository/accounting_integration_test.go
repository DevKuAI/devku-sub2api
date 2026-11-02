//go:build integration

package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type accountingTestStorage struct {
	available bool
	mu        sync.Mutex
	uploaded  int
}

func (s *accountingTestStorage) Ready() bool { return s.available }
func (s *accountingTestStorage) Upload(_ context.Context, _ string, body io.Reader, _ int64, _ string) error {
	_, err := io.Copy(io.Discard, body)
	s.mu.Lock()
	s.uploaded++
	s.mu.Unlock()
	return err
}
func (s *accountingTestStorage) Presign(_ context.Context, key, _, _ string, _ time.Duration) (string, error) {
	return "https://private.example.test/" + key + "?signature=short", nil
}

type accountingFixture struct {
	desktop            *desktopRepositoryFixture
	router             *gin.Engine
	storage            *accountingTestStorage
	receivingAccountID string
	orderID            string
	statementAccount   bool
}

func newAccountingFixture(t *testing.T) *accountingFixture {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	f := newDesktopRepositoryFixture(t, "accounting", 10)
	cfg := &config.Config{}
	cfg.Desktop.Enabled = true
	cfg.Totp.EncryptionKey = strings.Repeat("ab", 32)
	cfg.Totp.EncryptionKeyConfigured = true
	encryptor, err := NewAESEncryptor(cfg)
	require.NoError(t, err)
	storage := &accountingTestStorage{available: true}
	desktop := service.NewDesktopService(f.repo, nil, nil, nil, nil, nil, cfg, nil, nil, nil)
	engine := accounting.NewEngine(integrationDB, desktop, storage, encryptor, cfg)
	handler := accounting.NewHandler(engine)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		id := f.user.ID
		if value := c.GetHeader("X-Test-Actor"); value != "" {
			id, _ = strconv.ParseInt(value, 10, 64)
		}
		if c.GetHeader("X-No-Web-Session") != "true" {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: id})
		}
		role := c.GetHeader("X-Test-Role")
		if c.GetHeader("Authorization") == "Bearer accounting-qa-user" {
			role = "user"
		}
		if c.GetHeader("Authorization") == "Bearer accounting-qa-finance" {
			role = "finance"
		}
		if role == "" {
			role = "admin"
		}
		c.Set(string(middleware.ContextKeyUserRole), role)
		c.Next()
	})
	handler.RegisterAdmin(router.Group("/api/v1/admin"))
	handler.RegisterOrganization(router.Group("/api/v1"))
	t.Cleanup(func() {
		ctx := context.Background()
		for _, query := range []string{
			// Statement lines and cards outlive the organization's records; let go of them before those are deleted.
			"UPDATE accounting_channel_lines SET transaction_id=NULL WHERE transaction_id IN (SELECT id FROM accounting_vcc_transactions WHERE organization_id=$1)",
			"UPDATE accounting_channel_cards SET organization_id=NULL,vcc_card_id=NULL WHERE organization_id=$1 OR vcc_card_id IN (SELECT id FROM accounting_vcc_cards WHERE organization_id=$1)",
			"DELETE FROM accounting_operation_results WHERE actor_id=$1",
			"DELETE FROM accounting_statements WHERE organization_id=$1",
			"DELETE FROM accounting_periods WHERE organization_id=$1",
			"DELETE FROM accounting_external_actions WHERE organization_id=$1",
			"DELETE FROM accounting_receipt_settlements WHERE organization_id=$1",
			"DELETE FROM accounting_cost_allocations WHERE organization_id=$1",
			"DELETE FROM accounting_fx_exchanges WHERE organization_id=$1",
			"DELETE FROM accounting_receivables WHERE organization_id=$1",
			"DELETE FROM accounting_position_events WHERE organization_id=$1",
			"DELETE FROM accounting_position_baselines WHERE organization_id=$1",
			"DELETE FROM accounting_organization_contacts WHERE organization_id=$1",
			"DELETE FROM accounting_receiving_accounts WHERE created_by=$1",
			"DELETE FROM accounting_order_items WHERE organization_id=$1",
			"DELETE FROM accounting_orders WHERE organization_id=$1",
			"DELETE FROM accounting_funding WHERE transaction_id IN (SELECT id FROM accounting_vcc_transactions WHERE organization_id=$1)",
			"DELETE FROM accounting_allocations WHERE organization_id=$1",
			"DELETE FROM accounting_reconciliation WHERE organization_id=$1",
			"DELETE FROM accounting_attachments WHERE organization_id=$1",
			"DELETE FROM accounting_adjustments WHERE organization_id=$1",
			"DELETE FROM accounting_reversals WHERE organization_id=$1",
			"DELETE FROM accounting_audit_logs WHERE organization_id=$1",
			"DELETE FROM accounting_expense_payments WHERE organization_id=$1",
			"DELETE FROM accounting_vcc_transactions WHERE organization_id=$1",
			"DELETE FROM accounting_transfers WHERE organization_id=$1",
			"DELETE FROM accounting_vcc_cards WHERE organization_id=$1",
			"DELETE FROM accounting_contracts WHERE organization_id=$1",
			"DELETE FROM accounting_invoices WHERE organization_id=$1",
			"DELETE FROM accounting_vendor_invoices WHERE organization_id=$1",
			"DELETE FROM accounting_expenses WHERE organization_id=$1",
		} {
			value := f.organization.ID
			if strings.Contains(query, "operation_results") || strings.Contains(query, "accounting_receiving_accounts") {
				value = f.user.ID
			}
			_, cleanupErr := integrationDB.ExecContext(ctx, query, value)
			require.NoError(t, cleanupErr)
		}
	})
	return &accountingFixture{desktop: f, router: router, storage: storage}
}

func TestAccountingOrganizationContactsAndRecordSnapshots(t *testing.T) {
	f := newAccountingFixture(t)
	input := accounting.OrganizationContactInput{
		OrganizationID: f.desktop.organization.PublicID,
		Name:           "Finance Contact",
		Role:           "FINANCE",
		Phone:          "+8613800000000",
		Email:          "finance@example.test",
		WeChat:         "finance-contact",
		IsPrimary:      true,
	}
	var contact accounting.OrganizationContact
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "organization-contacts", uuid.NewString(), input)), &contact))
	require.Equal(t, input.Name, contact.Name)
	require.True(t, contact.IsPrimary)
	receivingAccountID := f.receivingAccount(t)

	transfer := f.create(t, "transfers", "100", map[string]any{
		"payer_name":           "Customer",
		"contact_id":           contact.ID,
		"receiving_account_id": receivingAccountID,
		"sales_amount":         "100",
		"tax_inclusive":        true,
		"tax_rate":             "0",
		"bank_reference":       uuid.NewString(),
	}, "")
	require.Equal(t, "Finance Contact", transfer.Details["contact_name"])
	require.Equal(t, "finance@example.test", transfer.Details["contact_email"])
	require.Equal(t, "6225888800001234", transfer.Details["receiving_account"])
	require.Equal(t, "Test Bank", transfer.Details["receiving_bank_name"])

	input.Name = "Updated Contact"
	input.Email = "updated@example.test"
	accountingData(t, f.request("PATCH", "organization-contacts/"+contact.ID, uuid.NewString(), input))
	accountingData(t, f.request("PATCH", "receiving-accounts/"+receivingAccountID, uuid.NewString(), accounting.ReceivingAccountInput{
		Currency:    "CNY",
		AccountName: f.desktop.organization.Name,
		BankName:    "Updated Bank",
		BankAccount: "6225888800009999",
		BankBranch:  "Updated Branch",
	}))
	var stored accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "transfers/"+transfer.ID, "", nil)), &stored))
	require.Equal(t, "Finance Contact", stored.Details["contact_name"])
	require.Equal(t, "finance@example.test", stored.Details["contact_email"])
	require.Equal(t, "6225888800001234", stored.Details["receiving_account"])
	require.Equal(t, "Test Bank", stored.Details["receiving_bank_name"])

	contract := f.create(t, "contracts", "100", map[string]any{
		"name":            "Sales Contract",
		"contract_number": uuid.NewString(),
		"contact_id":      contact.ID,
	}, "")
	require.Equal(t, f.desktop.organization.Name, contract.Details["party_name"])
	require.Equal(t, "Updated Contact", contract.Details["contact_name"])
	require.Equal(t, "updated@example.test", contract.Details["contact_email"])
}
func (f *accountingFixture) request(method, path, key string, payload any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	if method == "POST" || method == "PATCH" {
		body = withSeededVendors(path, body)
	}
	return f.rawRequest(method, path, key, body)
}

func (f *accountingFixture) rawRequest(method, path, key string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/admin/accounting/"+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	return res
}
func accountingData(t *testing.T, res *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	require.Equal(t, 200, res.Code, res.Body.String())
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &envelope))
	return envelope.Data
}
func (f *accountingFixture) create(t *testing.T, kind, amount string, details map[string]any, parent string) *accounting.Record {
	t.Helper()
	in := accounting.Input{OrganizationID: f.desktop.organization.PublicID, Currency: "CNY", Amount: amount, Details: details, ParentID: parent}
	if kind == "vcc-cards" || kind == "vcc-transactions" {
		in.OrderID = f.order(t)
	}
	if kind == "vcc-cards" && details["provider_card_id"] == nil {
		details["provider_card_id"] = testCardID()
	}
	if kind == "vcc-cards" && details["last_four"] == nil {
		details["last_four"] = "4242"
	}
	res := f.request("POST", kind, uuid.NewString(), in)
	var record accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, res), &record))
	return &record
}

// order returns the fixture's working order; cards and card spend can only be created inside one.
func (f *accountingFixture) order(t *testing.T) string {
	t.Helper()
	if f.orderID != "" {
		return f.orderID
	}
	var order accounting.Order
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "orders", uuid.NewString(), map[string]string{"organization_id": f.desktop.organization.PublicID, "name": "Fixture order"})), &order))
	f.orderID = order.ID
	return order.ID
}

// testCardID returns a distinct platform card ID so card registrations never collide between tests.
func testCardID() string { return "card-" + uuid.NewString() }
func (f *accountingFixture) receivingAccount(t *testing.T) string {
	t.Helper()
	if f.receivingAccountID != "" {
		return f.receivingAccountID
	}
	input := accounting.ReceivingAccountInput{
		Currency:    "CNY",
		AccountName: f.desktop.organization.Name,
		BankName:    "Test Bank",
		BankAccount: "6225888800001234",
		BankBranch:  "Test Branch",
	}
	var account accounting.ReceivingAccount
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "receiving-accounts", uuid.NewString(), input)), &account))
	f.receivingAccountID = account.ID
	return account.ID
}
func (f *accountingFixture) transfer(t *testing.T, total string) *accounting.Record {
	return f.create(t, "transfers", total, map[string]any{"payer_name": "Customer", "receiving_account_id": f.receivingAccount(t), "sales_amount": total, "tax_inclusive": true, "tax_rate": "0.06", "bank_reference": uuid.NewString(), "remark": "INTERNAL-REMARK"}, "")
}
func (f *accountingFixture) card(t *testing.T) *accounting.Record {
	return f.create(t, "vcc-cards", "0", map[string]any{"alias": "AI card", "provider": "INTERNAL-PROVIDER"}, "")
}

// statementLine imports one card line for the card through a statement template, the only way card spend enters the
// books, and reports what the import did with it.
func (f *accountingFixture) statementLine(t *testing.T, card *accounting.Record, kind, total string, original *accounting.Record) (booking, transaction string) {
	t.Helper()
	account := "test-" + f.desktop.organization.PublicID
	if !f.statementAccount {
		f.statementAccount = true
		cleanChannelAccounts(t, account)
		t.Cleanup(func() { cleanChannelAccounts(t, account) })
	}
	amount := "-" + total
	related := ""
	if kind == "REFUND" {
		amount = total
		related = original.Details["provider_transaction_id"].(string)
	}
	external := uuid.NewString()
	at := time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05Z07:00")
	row := strings.Join([]string{"TESTPAY", account, "CARD", external, at, kind, amount, card.Currency, "", "", card.Details["provider_card_id"].(string), "4242", "", "AI merchant", related, "", "", "statement line"}, ",")
	accountingData(t, f.upload(t, "channel-imports", uuid.NewString(), "", templateUpload("card.csv", row)))
	require.NoError(t, integrationDB.QueryRow("SELECT booking,COALESCE(transaction_id::text,'') FROM accounting_channel_lines WHERE external_id=$1 AND status='ACTIVE'", external).Scan(&booking, &transaction))
	return booking, transaction
}

// spend books a card line and returns the card transaction it became.
func (f *accountingFixture) spend(t *testing.T, card *accounting.Record, kind, total string, original *accounting.Record) *accounting.Record {
	t.Helper()
	booking, id := f.statementLine(t, card, kind, total, original)
	require.Equal(t, "BOOKED", booking)
	var r accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "vcc-transactions/"+id, "", nil)), &r))
	return &r
}
func (f *accountingFixture) confirm(t *testing.T, kind string, r *accounting.Record) {
	t.Helper()
	accountingData(t, f.request("POST", kind+"/"+r.ID+"/confirm", uuid.NewString(), map[string]any{}))
}
func (f *accountingFixture) proof(t *testing.T, kind string, r *accounting.Record) string {
	t.Helper()
	body := []byte("%PDF-1.7\nInvoice proof\n%%EOF")
	sha := fmt.Sprintf("%x", sha256.Sum256(body))
	payload := map[string]any{"organization_id": r.OrganizationID, "object_kind": kind, "object_id": r.ID, "purpose": "INVOICE", "filename": "proof.pdf", "content_type": "application/pdf", "size_bytes": len(body), "sha256": sha}
	var intent struct {
		Attachment accounting.Attachment `json:"attachment"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "attachments/upload-intents", uuid.NewString(), payload)), &intent))
	id := intent.Attachment.ID
	req := httptest.NewRequest("PUT", "/api/v1/admin/accounting/attachments/"+id+"/content", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", uuid.NewString())
	req.Header.Set("Content-Type", "application/pdf")
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	accountingData(t, res)
	accountingData(t, f.request("POST", "attachments/"+id+"/complete", uuid.NewString(), map[string]any{}))
	return id
}

func TestAccountingCardRefundCloseAndIdempotency(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	consume := f.spend(t, card, "CONSUME", "70", nil)
	// Authorization fees are our own cost and stay in the channel ledger.
	booking, transaction := f.statementLine(t, card, "FEE", "10", nil)
	require.Equal(t, "INTERNAL", booking)
	require.Empty(t, transaction)
	f.spend(t, card, "REFUND", "20", consume)
	require.Equal(t, "50", f.cardSpent(t, card.ID))
	res := f.request("POST", "vcc-transactions/"+consume.ID+"/void", uuid.NewString(), map[string]string{"reason": "wrong"})
	require.Equal(t, 409, res.Code)
	require.Contains(t, res.Body.String(), "card_spend_from_import")

	path := "vcc-cards/" + card.ID + "/close"
	key := uuid.NewString()
	first := f.request("POST", path, key, map[string]any{})
	body := accountingData(t, first)
	replay := f.request("POST", path, key, map[string]any{})
	require.JSONEq(t, string(body), string(accountingData(t, replay)))
	require.Equal(t, "true", replay.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, 409, f.request("POST", path, key, map[string]any{"reason": "changed"}).Code)
	var actions []string
	rows, err := integrationDB.Query("SELECT action FROM accounting_external_actions WHERE card_id=$1", card.ID)
	require.NoError(t, err)
	for rows.Next() {
		var action string
		require.NoError(t, rows.Scan(&action))
		actions = append(actions, action)
	}
	require.NoError(t, rows.Close())
	require.Equal(t, []string{"CLOSE"}, actions)

	// Statement spend still arrives after the card is closed: refunds, and charges the platform let through.
	f.spend(t, card, "REFUND", "10", consume)
	require.Equal(t, "40", f.cardSpent(t, card.ID))
	f.spend(t, card, "CONSUME", "5", nil)
	require.Equal(t, "45", f.cardSpent(t, card.ID))

	// The receipt is not tied to the card's spend.
	accountingData(t, f.request("POST", "transfers/"+receipt.ID+"/void", uuid.NewString(), map[string]string{"reason": "wrong"}))
}
func TestAccountingStatementRefundLimitAndHandEntryRefused(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	consume := f.spend(t, card, "CONSUME", "100", nil)
	f.spend(t, card, "REFUND", "60", consume)
	// A refund above what is left of its purchase waits for review instead of failing the statement.
	booking, transaction := f.statementLine(t, card, "REFUND", "60", consume)
	require.Equal(t, "PENDING_REVIEW", booking)
	require.Empty(t, transaction)
	require.Equal(t, "40", f.cardSpent(t, card.ID))
	var line, note string
	require.NoError(t, integrationDB.QueryRow("SELECT id::text,booking_note FROM accounting_channel_lines WHERE booking='PENDING_REVIEW' AND channel_card_id=(SELECT id FROM accounting_channel_cards WHERE vcc_card_id=$1)", card.ID).Scan(&line, &note))
	require.Equal(t, "amount", note)
	retried := accountingObject(t, f.request("POST", "channel-lines/"+line+"/rebook", uuid.NewString(), map[string]any{}))
	require.Equal(t, "PENDING_REVIEW", retried["booking"])

	// Card spend has no other entry point.
	for _, path := range []string{"vcc-transactions", "vcc-cards/" + card.ID + "/transactions"} {
		res := f.request("POST", path, uuid.NewString(), accounting.Input{OrganizationID: receipt.OrganizationID, ParentID: card.ID, Currency: "CNY", Amount: "1", Details: map[string]any{"transaction_type": "CONSUME", "merchant": "Vendor"}, OrderID: f.order(t)})
		require.Equal(t, 409, res.Code, path)
		require.Contains(t, res.Body.String(), "card_spend_from_import")
	}
	entry := f.request("POST", "orders/"+f.order(t)+"/entries", uuid.NewString(), map[string]any{"kind": "vcc-transactions", "currency": "CNY", "amount": "1", "parent_id": card.ID, "details": map[string]any{"transaction_type": "CONSUME", "merchant": "Vendor"}})
	require.Equal(t, 409, entry.Code)
	require.Equal(t, 409, f.request("POST", "transfers", uuid.NewString(), accounting.Input{OrganizationID: receipt.OrganizationID, Currency: "CNY", Amount: receipt.Amount, Details: receipt.Details}).Code)
	require.Equal(t, 400, f.request("POST", "transfers", "", accounting.Input{OrganizationID: receipt.OrganizationID, Currency: "CNY", Amount: "1", Details: receipt.Details}).Code)
}
func TestAccountingClosedCardRefundsAndCurrencyIsolation(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	// Spend beyond what the organization paid is recorded as spend; there is no overdraft to cover.
	consume := f.spend(t, card, "CONSUME", "130", nil)
	accountingData(t, f.request("POST", "vcc-cards/"+card.ID+"/close", uuid.NewString(), map[string]any{}))
	f.spend(t, card, "REFUND", "20", consume)
	f.spend(t, card, "REFUND", "30", consume)
	require.Equal(t, "80", f.cardSpent(t, card.ID))
	// A line in another currency than the card is not booked onto it.
	dollars := *card
	dollars.Currency = "USD"
	booking, _ := f.statementLine(t, &dollars, "CONSUME", "1", nil)
	require.Equal(t, "PENDING_REVIEW", booking)
	require.Equal(t, "80", f.cardSpent(t, card.ID))
	var workbench struct {
		Counts map[string]int64 `json:"counts"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "workbench?organization_id="+receipt.OrganizationID, "", nil)), &workbench))
	require.NotContains(t, workbench.Counts, "overdraft")
	require.NotContains(t, workbench.Counts, "unallocated")
}
func TestAccountingOrganizationReadOnlySharingAndSecrets(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	file := f.proof(t, "transfers", receipt)
	read := func(path string, actor int64) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/v1/organization/accounting/"+path, nil)
		req.Header.Set("X-Test-Role", "user")
		req.Header.Set("X-Test-Actor", strconv.FormatInt(actor, 10))
		res := httptest.NewRecorder()
		f.router.ServeHTTP(res, req)
		return res
	}
	// Drafts and their shared proofs stay internal until the receipt is confirmed.
	accountingData(t, f.request("PATCH", "attachments/"+file+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}))
	draft := read("transfers", f.desktop.user.ID)
	require.Equal(t, 200, draft.Code)
	require.NotContains(t, draft.Body.String(), receipt.ID)
	require.Equal(t, 404, read("attachments/"+file+"/download-url", f.desktop.user.ID).Code)
	require.Equal(t, 404, read("attachments?object_kind=transfers&object_id="+receipt.ID, f.desktop.user.ID).Code)
	accountingData(t, f.request("PATCH", "attachments/"+file+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": false}))
	f.confirm(t, "transfers", receipt)
	own := read("transfers?organization_id=forged", f.desktop.user.ID)
	require.Equal(t, 200, own.Code)
	require.Contains(t, own.Body.String(), receipt.ID)
	require.NotContains(t, own.Body.String(), "INTERNAL-BANK")
	require.NotContains(t, own.Body.String(), "INTERNAL-REMARK")
	require.Equal(t, 404, read("attachments/"+file+"/download-url", f.desktop.user.ID).Code)
	accountingData(t, f.request("PATCH", "attachments/"+file+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}))
	require.Equal(t, 200, read("attachments/"+file+"/download-url", f.desktop.user.ID).Code)
	other := newDesktopRepositoryFixture(t, "acctother", 1)
	outside := read("transfers?organization_id="+receipt.OrganizationID, other.user.ID)
	require.Equal(t, 200, outside.Code)
	require.NotContains(t, outside.Body.String(), receipt.ID)
	require.Equal(t, 404, read("attachments/"+file+"/download-url", other.user.ID).Code)
	// Full card numbers, CVVs and tokens are refused rather than stored.
	for _, secret := range []map[string]any{{"card_number": "4111111111111111"}, {"cvv": "123"}, {"provider_token": "tok"}} {
		details := map[string]any{"alias": "Secure", "provider": "INTERNAL-PROVIDER", "provider_card_id": testCardID(), "last_four": "1111"}
		for k, v := range secret {
			details[k] = v
		}
		in := accounting.Input{OrganizationID: f.desktop.organization.PublicID, Currency: "CNY", Amount: "0", Details: details, OrderID: f.order(t)}
		require.Equal(t, 400, f.request("POST", "vcc-cards", uuid.NewString(), in).Code)
	}
	card := f.create(t, "vcc-cards", "0", map[string]any{"alias": "Secure", "provider": "INTERNAL-PROVIDER", "provider_card_id": "INTERNAL-CARD-ID-" + uuid.NewString(), "last_four": "1111", "expiry": "2030-12"}, "")
	body := read("vcc-card", f.desktop.user.ID).Body.String()
	require.Contains(t, body, "1111")
	for _, private := range []string{"INTERNAL-PROVIDER", "INTERNAL-CARD-ID", "2030-12"} {
		require.NotContains(t, body, private)
	}
	var cipher string
	require.NoError(t, integrationDB.QueryRow("SELECT secret_ciphertext FROM accounting_vcc_cards WHERE id=$1", card.ID).Scan(&cipher))
	require.Empty(t, cipher)
	req := httptest.NewRequest("GET", "/api/v1/organization/accounting/summary", nil)
	req.Header.Set("X-No-Web-Session", "true")
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	require.Equal(t, 401, res.Code)
	_, err := integrationDB.Exec("UPDATE desktop_organizations SET deleted_at=NOW() WHERE id=$1", f.desktop.organization.ID)
	require.NoError(t, err)
	require.Equal(t, 404, read("transfers", f.desktop.user.ID).Code)
}
func TestAccountingPaymentsAIInvoiceAndAdjustments(t *testing.T) {
	f := newAccountingFixture(t)
	expense := f.create(t, "expenses", "100", map[string]any{"category": "AI_VENDOR", "supplier": "Vendor", "payment_account": "Bank", "ai_vendor": true}, "")
	proofID := f.proof(t, "expenses", expense)
	payment := accounting.Input{Amount: "60", Currency: "CNY", Details: map[string]any{"payment_account": "Bank", "transaction_number": uuid.NewString(), "proof_attachment_id": proofID}}
	require.Equal(t, 400, f.request("POST", "expenses/"+expense.ID+"/payments", uuid.NewString(), payment).Code)
	invoice := f.create(t, "vendor-invoices", "100", map[string]any{"vendor": "Vendor", "period_start": "2026-10-01", "period_end": "2026-10-31", "expense_id": expense.ID}, "")
	f.proof(t, "vendor-invoices", invoice)
	f.confirm(t, "vendor-invoices", invoice)
	var current accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "expenses/"+expense.ID, "", nil)), &current))
	current.Details["vendor_invoice_id"] = invoice.ID
	accountingData(t, f.request("PATCH", "expenses/"+expense.ID, uuid.NewString(), accounting.Input{Amount: "100", Currency: "CNY", Version: current.Version, Details: current.Details}))
	var first accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "expenses/"+expense.ID+"/payments", uuid.NewString(), payment)), &first))
	require.Equal(t, 409, f.request("POST", "expenses/"+expense.ID+"/void", uuid.NewString(), map[string]string{"reason": "Wrong"}).Code)
	payment.Amount = "50"
	payment.Details["transaction_number"] = uuid.NewString()
	require.Equal(t, 409, f.request("POST", "expenses/"+expense.ID+"/payments", uuid.NewString(), payment).Code)
	payment.Details["override"] = true
	payment.Details["reason"] = "Approved overpayment"
	accountingData(t, f.request("POST", "expenses/"+expense.ID+"/payments", uuid.NewString(), payment))
	accountingData(t, f.request("POST", "expense-payments/"+first.ID+"/void", uuid.NewString(), map[string]string{"reason": "Reverse first payment"}))
	accountingData(t, f.request("POST", "reconciliation/adjustments", uuid.NewString(), map[string]string{"object_kind": "expenses", "object_id": expense.ID, "amount": "10", "currency": "CNY", "reason": "Correct amount"}))
	require.Equal(t, 409, f.request("POST", "vendor-invoices/"+invoice.ID+"/void", uuid.NewString(), map[string]string{"reason": "Wrong"}).Code)
}
func TestAccountingAuditRollbackAndSafeRetry(t *testing.T) {
	f := newAccountingFixture(t)
	ctx := context.Background()
	receivingAccountID := f.receivingAccount(t)
	name := fmt.Sprintf("accounting_audit_fail_%d", f.desktop.user.ID)
	_, err := integrationDB.ExecContext(ctx, fmt.Sprintf("CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.actor_id=%d THEN RAISE EXCEPTION 'test audit unavailable'; END IF; RETURN NEW; END $$; CREATE TRIGGER %s BEFORE INSERT ON accounting_audit_logs FOR EACH ROW EXECUTE FUNCTION %s()", name, f.desktop.user.ID, name, name))
	require.NoError(t, err)
	cleanup := func() {
		_, _ = integrationDB.ExecContext(ctx, "DROP TRIGGER IF EXISTS "+name+" ON accounting_audit_logs; DROP FUNCTION IF EXISTS "+name+"()")
	}
	t.Cleanup(cleanup)
	key := uuid.NewString()
	in := accounting.Input{OrganizationID: f.desktop.organization.PublicID, Currency: "CNY", Amount: "100", Details: map[string]any{"payer_name": "Customer", "receiving_account_id": receivingAccountID, "sales_amount": "100", "tax_inclusive": true, "tax_rate": "0", "bank_reference": uuid.NewString()}}
	require.Equal(t, 500, f.request("POST", "transfers", key, in).Code)
	var count int
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM accounting_transfers WHERE organization_id=$1", f.desktop.organization.ID).Scan(&count))
	require.Zero(t, count)
	cleanup()
	first := f.request("POST", "transfers", key, in)
	accountingData(t, first)
	replayed := f.request("POST", "transfers", key, in)
	require.JSONEq(t, first.Body.String(), replayed.Body.String())
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM accounting_transfers WHERE organization_id=$1", f.desktop.organization.ID).Scan(&count))
	require.Equal(t, 1, count)
}

// TestAccountingBrowserFixture exposes only disposable test data for manual browser acceptance.
func TestAccountingBrowserFixture(t *testing.T) {
	if os.Getenv("SUB2API_ACCOUNTING_BROWSER_QA") != "1" {
		t.Skip("browser fixture is opt-in")
	}
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "58000")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	f.spend(t, card, "CONSUME", "2480", nil)
	desktop := service.NewDesktopService(f.desktop.repo, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil)
	f.router.GET("/api/v1/desktop/organization", func(c *gin.Context) {
		org, err := desktop.GetManagedOrganization(c.Request.Context(), f.desktop.user.ID)
		if response.ErrorFrom(c, err) {
			return
		}
		response.Success(c, map[string]any{"public_id": org.PublicID, "code": org.Code, "name": org.Name, "status": org.Status, "member_count": 0, "member_limit": 10, "gateway_user": map[string]any{"id": f.desktop.user.ID, "email": "accounting-qa@example.test"}})
	})
	f.router.GET("/api/v1/settings/public", func(c *gin.Context) {
		response.Success(c, map[string]any{"site_name": "Accounting QA", "desktop_enabled": true, "subscription_enabled": false, "payment_enabled": false, "backend_mode_enabled": false, "public_dashboard_enabled": false, "custom_menu_items": []any{}})
	})
	f.router.GET("/api/v1/auth/me", func(c *gin.Context) {
		role := "admin"
		if c.GetHeader("Authorization") == "Bearer accounting-qa-user" {
			role = "user"
		}
		if c.GetHeader("Authorization") == "Bearer accounting-qa-finance" {
			role = "finance"
		}
		response.Success(c, map[string]any{"id": f.desktop.user.ID, "role": role, "email": "accounting-qa@example.test", "username": "Accounting QA", "status": "active", "balance": 0, "concurrency": 1, "run_mode": "standard"})
	})
	f.router.GET("/setup/status", func(c *gin.Context) { c.JSON(200, map[string]any{"needs_setup": false}) })
	f.router.GET("/api/v1/admin/compliance", func(c *gin.Context) { response.Success(c, map[string]any{"required": false}) })
	f.router.GET("/api/v1/admin/settings", func(c *gin.Context) {
		response.Success(c, map[string]any{"ops_monitoring_enabled": false, "custom_menu_items": []any{}})
	})
	f.router.GET("/api/v1/admin/payment/config", func(c *gin.Context) { response.Success(c, map[string]any{"enabled": false}) })
	f.router.GET("/api/v1/subscription-accounts", func(c *gin.Context) { response.Paginated(c, []any{}, 0, 1, 20) })
	f.router.GET("/api/v1/announcements", func(c *gin.Context) { response.Success(c, []any{}) })
	f.router.GET("/api/v1/version", func(c *gin.Context) { response.Success(c, map[string]any{"version": "accounting-qa"}) })
	server := httptest.NewUnstartedServer(f.router)
	if port := os.Getenv("SUB2API_ACCOUNTING_QA_PORT"); port != "" {
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
		require.NoError(t, err)
		require.NoError(t, server.Listener.Close())
		server.Listener = listener
	}
	server.Start()
	defer server.Close()
	state, err := json.Marshal(map[string]any{"url": server.URL, "user_id": f.desktop.user.ID, "organization_id": f.desktop.organization.PublicID})
	require.NoError(t, err)
	qaDir := os.Getenv("SUB2API_ACCOUNTING_QA_DIR")
	if qaDir == "" {
		qaDir = "/tmp"
	}
	require.NoError(t, os.WriteFile(filepath.Join(qaDir, "sub2api-accounting-qa.json"), state, 0600))
	t.Logf("Accounting browser fixture ready: %s", server.URL)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	deadline := time.After(60 * time.Minute)
	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(filepath.Join(qaDir, "sub2api-accounting-qa-complete")); err == nil {
				return
			}
		case <-deadline:
			t.Fatal("browser fixture timed out")
		}
	}
}

func TestAccountingReconciliationPaginationAndSummaryPeriods(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	_, err := integrationDB.Exec("UPDATE accounting_vcc_cards SET occurred_at='2020-01-01' WHERE id=$1", card.ID)
	require.NoError(t, err)
	consume := f.spend(t, card, "CONSUME", "60", nil)
	late := f.spend(t, card, "CONSUME", "5", nil)
	// Keep the receipt and the first purchase inside the fixed reporting period regardless of when this test runs;
	// the summary counts records by their own dates, not the card's.
	_, err = integrationDB.Exec("UPDATE accounting_transfers SET occurred_at='2026-10-15T00:00:00Z' WHERE id=$1", receipt.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec("UPDATE accounting_vcc_transactions SET occurred_at=CASE WHEN id=$1 THEN '2026-10-15T00:00:00Z'::timestamptz ELSE '2026-11-15T00:00:00Z'::timestamptz END WHERE id IN ($1,$2)", consume.ID, late.ID)
	require.NoError(t, err)
	path := "summary?organization_id=" + receipt.OrganizationID + "&from=2026-10-01T00:00:00Z&to=2026-11-01T00:00:00Z"
	var summary []accounting.CurrencySummary
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", path, "", nil)), &summary))
	require.Len(t, summary, 1)
	require.Equal(t, "100.000000", summary[0].Received)
	require.Equal(t, "60.000000", summary[0].Consumed)
	payload := map[string]string{"object_kind": "transfers", "object_id": receipt.ID, "currency": "CNY", "amount": "99", "reference": "BANK", "reason": "Bank fee"}
	accountingData(t, f.request("POST", "reconciliation/match", uuid.NewString(), payload))
	var page struct {
		Items []any `json:"items"`
		Total int   `json:"total"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "reconciliation?organization_id="+receipt.OrganizationID+"&status=DIFFERENCE&page_size=1", "", nil)), &page))
	require.Len(t, page.Items, 1)
	require.Equal(t, 1, page.Total)
	payload["amount"] = "100"
	payload["reason"] = ""
	accountingData(t, f.request("POST", "reconciliation/match", uuid.NewString(), payload))
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "reconciliation?organization_id="+receipt.OrganizationID+"&status=DIFFERENCE", "", nil)), &page))
	require.Zero(t, page.Total)
}
func TestAccountingAttachmentFailureExpiryAndSoftDeletion(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	file := f.proof(t, "transfers", receipt)
	deleteKey := uuid.NewString()
	first := accountingData(t, f.request("DELETE", "attachments/"+file, deleteKey, map[string]any{}))
	require.JSONEq(t, string(first), string(accountingData(t, f.request("DELETE", "attachments/"+file, deleteKey, map[string]any{}))))
	require.Equal(t, 404, f.request("GET", "attachments/"+file+"/download-url", "", nil).Code)
	var deleted bool
	require.NoError(t, integrationDB.QueryRow("SELECT deleted_at IS NOT NULL FROM accounting_attachments WHERE id=$1", file).Scan(&deleted))
	require.True(t, deleted)
	body := []byte("%PDF-1.7\nproof\n%%EOF")
	payload := map[string]any{"organization_id": receipt.OrganizationID, "object_kind": "transfers", "object_id": receipt.ID, "purpose": "PROOF", "filename": "receipt.pdf", "content_type": "application/pdf", "size_bytes": len(body), "sha256": fmt.Sprintf("%x", sha256.Sum256(body))}
	var intent struct {
		Attachment accounting.Attachment `json:"attachment"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "attachments/upload-intents", uuid.NewString(), payload)), &intent))
	_, err := integrationDB.Exec("UPDATE accounting_attachments SET expires_at=NOW()-interval '1 second' WHERE id=$1", intent.Attachment.ID)
	require.NoError(t, err)
	req := httptest.NewRequest("PUT", "/api/v1/admin/accounting/attachments/"+intent.Attachment.ID+"/content", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", uuid.NewString())
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	require.Equal(t, 409, res.Code)
	payload["size_bytes"] = accounting.MaxAttachmentBytes + 1
	require.Equal(t, 400, f.request("POST", "attachments/upload-intents", uuid.NewString(), payload).Code)
	f.storage.available = false
	require.Equal(t, 503, f.request("POST", "attachments/upload-intents", uuid.NewString(), payload).Code)
}

func TestAccountingBindingReassignmentRevokesOldAccountImmediately(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	replacement := mustCreateUser(t, integrationEntClient, &service.User{Email: "accounting-replacement-" + uuid.NewString() + "@example.test"})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("UPDATE desktop_organizations SET gateway_user_id=$2 WHERE id=$1", f.desktop.organization.ID, f.desktop.user.ID)
		_, _ = integrationDB.Exec("DELETE FROM users WHERE id=$1", replacement.ID)
	})
	_, _, err := f.desktop.repo.UpdateOrganization(context.Background(), f.desktop.organization.PublicID, service.DesktopUpdateOrganizationInput{GatewayUserID: &replacement.ID})
	require.NoError(t, err)
	read := func(actor int64) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/v1/organization/accounting/transfers?organization_id="+receipt.OrganizationID, nil)
		req.Header.Set("X-Test-Actor", strconv.FormatInt(actor, 10))
		req.Header.Set("X-Test-Role", "user")
		res := httptest.NewRecorder()
		f.router.ServeHTTP(res, req)
		return res
	}
	require.Equal(t, 404, read(f.desktop.user.ID).Code)
	current := read(replacement.ID)
	require.Equal(t, 200, current.Code)
	require.Contains(t, current.Body.String(), receipt.ID)
}
func TestAccountingCardIdentityCannotOverwriteSpendHistory(t *testing.T) {
	f := newAccountingFixture(t)
	card := f.card(t)
	f.spend(t, card, "CONSUME", "100", nil)
	var other accounting.Vendor
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "vendors", uuid.NewString(), accounting.VendorInput{Kind: "CARD_PROVIDER", Name: "Provider " + uuid.NewString()[:8]})), &other))
	require.Equal(t, 409, f.request("PATCH", "vcc-cards/"+card.ID, uuid.NewString(), accounting.Input{Version: card.Version, Details: map[string]any{"provider_id": other.ID}}).Code)
	// The provider name follows the chosen provider; free text cannot rename it.
	require.Equal(t, 200, f.request("PATCH", "vcc-cards/"+card.ID, uuid.NewString(), accounting.Input{Version: card.Version, Details: map[string]any{"provider": "different-provider"}}).Code)
	var current accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "vcc-cards/"+card.ID, "", nil)), &current))
	require.Equal(t, "SUNRATE", current.Details["provider"])
}
func TestAccountingContractAndInvoiceSharingLifecycle(t *testing.T) {
	f := newAccountingFixture(t)
	contract := f.create(t, "contracts", "100", map[string]any{"name": "Contract", "contract_number": uuid.NewString(), "party_name": "Customer", "tax_rate": "0.06", "tax_inclusive": true}, "")
	f.proof(t, "contracts", contract)
	f.confirm(t, "contracts", contract)
	accountingData(t, f.request("PATCH", "contracts/"+contract.ID+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}))
	accountingData(t, f.request("POST", "contracts/"+contract.ID+"/expire", uuid.NewString(), map[string]any{}))
	sales := f.create(t, "invoices", "106", map[string]any{"invoice_type": "SALES", "invoice_code": "CODE", "invoice_number": uuid.NewString(), "issuer_name": "Us", "recipient_name": "Customer", "net_amount": "100", "tax_amount": "6"}, "")
	proof := f.proof(t, "invoices", sales)
	f.confirm(t, "invoices", sales)
	accountingData(t, f.request("PATCH", "attachments/"+proof+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}))
	req := httptest.NewRequest("GET", "/api/v1/organization/accounting/attachments/"+proof+"/download-url", nil)
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	require.Equal(t, 404, res.Code)
	accountingData(t, f.request("PATCH", "invoices/"+sales.ID+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}))
	res = httptest.NewRecorder()
	f.router.ServeHTTP(res, httptest.NewRequest("GET", "/api/v1/organization/accounting/attachments/"+proof+"/download-url", nil))
	require.Equal(t, 200, res.Code)
	internal := f.create(t, "invoices", "106", map[string]any{"invoice_type": "PURCHASE", "invoice_code": "CODE", "invoice_number": uuid.NewString(), "issuer_name": "Vendor", "recipient_name": "Us", "net_amount": "100", "tax_amount": "6"}, "")
	require.Equal(t, 409, f.request("PATCH", "invoices/"+internal.ID+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}).Code)
	require.Equal(t, 409, f.request("POST", "invoices", uuid.NewString(), accounting.Input{OrganizationID: sales.OrganizationID, Currency: sales.Currency, Amount: sales.Amount, Details: sales.Details}).Code)
}

// seededVendor returns the ID of a vendor the vendor migration creates.
func seededVendor(kind, name string) string {
	var id string
	if err := integrationDB.QueryRow("SELECT id::text FROM accounting_vendors WHERE kind=$1 AND name=$2", kind, name).Scan(&id); err != nil {
		panic(err)
	}
	return id
}

// withSeededVendors fills the vendor every card, card spend and AI invoice must name, so tests about other
// behaviour need not repeat it. Tests about vendors send their bodies through rawRequest instead.
func withSeededVendors(path string, body []byte) []byte {
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return body
	}
	details, ok := payload["details"].(map[string]any)
	if !ok {
		return body
	}
	segments := strings.Split(strings.SplitN(path, "?", 2)[0], "/")
	kind, _ := payload["kind"].(string)
	if kind == "" {
		kind = segments[0]
		if len(segments) >= 4 && segments[0] == "orders" && segments[2] == "entries" {
			kind = segments[3]
		}
	}
	field, vendorKind, vendorName := "", "", ""
	switch {
	case kind == "vcc-cards":
		field, vendorKind, vendorName = "provider_id", "CARD_PROVIDER", "SUNRATE"
	case kind == "vcc-transactions" && details["transaction_type"] == "CONSUME":
		field, vendorKind, vendorName = "vendor_id", "AI_VENDOR", "OpenAI"
	case kind == "vendor-invoices":
		field, vendorKind, vendorName = "vendor_id", "AI_VENDOR", "OpenAI"
	default:
		return body
	}
	if _, set := details[field]; set {
		return body
	}
	details[field] = seededVendor(vendorKind, vendorName)
	out, _ := json.Marshal(payload)
	return out
}
