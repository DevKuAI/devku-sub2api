package channel

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

var updateSample = flag.Bool("update-sample", false, "rewrite the documented SUNRATE sample from the converter output")

const documentedSample = "../../../../docs/accounting-management/samples/sunrate-72778936-standard.csv"

func sunrateFiles(t *testing.T, skip ...string) []SourceFile {
	t.Helper()
	entries, err := os.ReadDir("testdata")
	require.NoError(t, err)
	var files []SourceFile
	for _, e := range entries {
		skipped := false
		for _, prefix := range skip {
			skipped = skipped || strings.HasPrefix(strings.ToUpper(e.Name()), prefix)
		}
		if e.IsDir() || skipped {
			continue
		}
		data, err := os.ReadFile(filepath.Join("testdata", e.Name()))
		require.NoError(t, err)
		files = append(files, SourceFile{Name: e.Name(), Data: data})
	}
	return files
}

func balances(r *Result) map[string]string {
	out := map[string]string{}
	for _, b := range r.Balances {
		out[b.String()] = b.Amount.StringFixed(2)
	}
	return out
}

func TestSunrateExportsReconcileWithStatements(t *testing.T) {
	result, err := Convert(sunrateFiles(t), "")
	require.NoError(t, err)
	require.Equal(t, sunratePlatform, result.Converter)
	require.Len(t, result.Lines, 36)
	require.Equal(t, "Hong Kong Rich Sum Technology Co. Limited", result.Accounts["SUNRATE/72778936"])
	require.Equal(t, map[string]string{
		"SUNRATE/72778936/PAYMENT/HKD": "0.00",
		"SUNRATE/72778936/PAYMENT/USD": "2172.87",
		"SUNRATE/72778936/CARD/USD":    "1026.50",
	}, balances(result))

	totals := map[string]decimal.Decimal{}
	fees := decimal.Zero
	for _, l := range result.Lines {
		totals[l.Type+"/"+l.Currency] = totals[l.Type+"/"+l.Currency].Add(l.Amount)
		if l.FeeAmount != nil {
			fees = fees.Add(*l.FeeAmount)
		}
	}
	require.Equal(t, "34608.63", totals["FUNDING_IN/HKD"].StringFixed(2))
	require.Equal(t, "-34608.63", totals["FX_OUT/HKD"].StringFixed(2))
	require.Equal(t, "4403.62", totals["FX_IN/USD"].StringFixed(2))
	require.Equal(t, "-2230.75", totals["TRANSFER_OUT/USD"].StringFixed(2))
	require.Equal(t, "2230.75", totals["TRANSFER_IN/USD"].StringFixed(2))
	require.Equal(t, "-1200.00", totals["CONSUME/USD"].StringFixed(2))
	require.Equal(t, "-4.25", totals["FEE/USD"].StringFixed(2))
	require.Equal(t, "34.64", fees.StringFixed(2))

	byID := map[string]Line{}
	for _, l := range result.Lines {
		byID[l.Wallet+"/"+l.Currency+"/"+l.ExternalID] = l
	}
	// Transfers pair across balances although the two sides carry different references.
	out := byID["PAYMENT/USD/2114914845280309249"]
	require.Equal(t, TransferOut, out.Type)
	require.Equal(t, "AJ2114914845519384577", out.RelatedID)
	require.Equal(t, out.ExternalID, byID["CARD/USD/AJ2114914845519384577"].RelatedID)
	require.Equal(t, "2026-10-27 10:59:27 +0800", out.OccurredAt.Format("2006-01-02 15:04:05 -0700"))
	// The statement books collections net; the collection record supplies the fee and the credit date stays.
	collection := byID["PAYMENT/HKD/C268277BP9"]
	require.Equal(t, "11168.82", collection.Amount.StringFixed(2))
	require.Equal(t, "11.18", collection.FeeAmount.StringFixed(2))
	require.Equal(t, "2026-08-31", collection.OccurredAt.Format("2006-01-02"))
	spend := byID["CARD/USD/AJ2113212933740826625"]
	require.Equal(t, Consume, spend.Type)
	require.Equal(t, "0777", spend.CardLast4)
	require.Equal(t, "匠心CC", spend.CardName)
	require.Equal(t, "A2113108198094426114", spend.RelatedID)
	require.Equal(t, "ANTHROPIC* CLAUDE SUB SAN FRANCISCO USA", spend.Merchant)

	csv := WriteCSV(result.Lines)
	if *updateSample {
		require.NoError(t, os.WriteFile(documentedSample, csv, 0o644))
	}
	documented, err := os.ReadFile(documentedSample)
	require.NoError(t, err)
	require.Equal(t, string(documented), string(csv), "documented sample is stale; run with -update-sample")
}

func TestSunrateOutputDoesNotDependOnUploadOrder(t *testing.T) {
	files := sunrateFiles(t)
	reversed := make([]SourceFile, len(files))
	for i := range files {
		reversed[len(files)-1-i] = files[i]
	}
	a, err := Convert(files, "")
	require.NoError(t, err)
	b, err := Convert(reversed, "")
	require.NoError(t, err)
	require.Equal(t, string(WriteCSV(a.Lines)), string(WriteCSV(b.Lines)))
}

func TestSunrateWithoutCardLedgerLeavesTransfersForBooking(t *testing.T) {
	result, err := Convert(sunrateFiles(t, sunrateCardPrefix), "")
	require.NoError(t, err)
	require.Len(t, result.Lines, 13)
	for _, l := range result.Lines {
		if l.Type == TransferOut {
			require.Empty(t, l.RelatedID)
		}
	}
}

func TestSunrateRejectsUnknownExports(t *testing.T) {
	files := append(sunrateFiles(t), SourceFile{Name: "SUNRATE_SOMETHING_ELSE.xlsx", Data: []byte("x")})
	_, err := Convert(files, "")
	var problems Problems
	require.ErrorAs(t, err, &problems)
	require.Contains(t, problems.Error(), "not a recognised SUNRATE export")
}

func TestTemplateRoundTrip(t *testing.T) {
	converted, err := Convert(sunrateFiles(t), "")
	require.NoError(t, err)
	csv := WriteCSV(converted.Lines)
	read, err := Convert([]SourceFile{{Name: "standard.csv", Data: csv}}, "")
	require.NoError(t, err)
	require.Equal(t, TemplateName, read.Converter)
	require.Equal(t, string(csv), string(WriteCSV(read.Lines)))
}

func templateFile(rows ...string) []SourceFile {
	body := strings.Join(Columns, ",") + "\n" + strings.Join(rows, "\n") + "\n"
	return []SourceFile{{Name: "lines.csv", Data: []byte(body)}}
}

func TestTemplateValidation(t *testing.T) {
	ok := "P,1,MAIN,a,2026-10-01 10:00:00+08:00,FUNDING_IN,100.00,USD,100.00,,,,,,,,,"
	for _, tt := range []struct {
		name, row, want string
	}{
		{"sign", "P,1,MAIN,b,2026-10-02 10:00:00+08:00,CONSUME,5.00,USD,105.00,,c1,,,,,,,", "sign does not match"},
		{"continuity", "P,1,MAIN,b,2026-10-02 10:00:00+08:00,FUNDING_IN,5.00,USD,104.00,,,,,,,,,", "does not equal"},
		{"precision", "P,1,MAIN,b,2026-10-02 10:00:00+08:00,FUNDING_IN,5.001,USD,105.001,,,,,,,,,", "decimal places"},
		{"zone", "P,1,MAIN,b,2026-10-02 10:00:00,FUNDING_IN,5.00,USD,105.00,,,,,,,,,", "time zone"},
		{"card", "P,1,MAIN,b,2026-10-02 10:00:00+08:00,CONSUME,-5.00,USD,95.00,,,,,,,,,", "card_ref"},
		{"fx", "P,1,MAIN,b,2026-10-02 10:00:00+08:00,FX_IN,5.00,USD,105.00,,,,,,,,,", "pair"},
		{"duplicate", "P,1,MAIN,a,2026-10-02 10:00:00+08:00,FUNDING_IN,5.00,USD,105.00,,,,,,,,,", "duplicated"},
		{"partial balance", "P,1,MAIN,b,2026-10-02 10:00:00+08:00,FUNDING_IN,5.00,USD,,,,,,,,,,", "all or none"},
		{"adjustment", "P,1,MAIN,b,2026-10-02 10:00:00+08:00,ADJUSTMENT,5.00,USD,105.00,,,,,,,,,", "remark"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Convert(templateFile(ok, tt.row), "")
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.want)
		})
	}
	result, err := Convert(templateFile(ok, "P,1,MAIN,b,2026-10-02 10:00:00+08:00,CONSUME,-5.00,USD,95.00,,c1,1234,Org,Shop,auth-1,,,"), "")
	require.NoError(t, err)
	require.Len(t, result.Lines, 2)

	_, err = Convert([]SourceFile{{Name: "lines.csv", Data: []byte(strings.Join(Columns, ",") + ",amout\n")}}, TemplateName)
	require.ErrorContains(t, err, "unknown column")
}
