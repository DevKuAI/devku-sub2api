package channel

import (
	"bytes"
	"encoding/csv"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

// Columns of the standard template, in the order written by WriteCSV.
var Columns = []string{
	"platform", "account_ref", "wallet", "external_id", "occurred_at", "type", "amount", "currency",
	"balance_after", "fee_amount", "card_ref", "card_last4", "card_name", "merchant", "related_id",
	"original_amount", "original_currency", "remark",
}

var requiredColumns = []string{"platform", "account_ref", "wallet", "external_id", "occurred_at", "type", "amount", "currency"}

var (
	codePattern     = regexp.MustCompile(`^[A-Z0-9_]{1,32}$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	last4Pattern    = regexp.MustCompile(`^[0-9]{4}$`)
	timeLayouts     = []string{"2006-01-02 15:04:05Z07:00", "2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05.999999999Z07:00", time.RFC3339Nano}
)

func allTemplates(files []SourceFile) bool {
	for _, f := range files {
		if f.Ext() != ".csv" && f.Ext() != ".xlsx" {
			return false
		}
		t, err := readTable(f)
		if err != nil || !t.has(requiredColumns...) {
			return false
		}
	}
	return true
}

func readTemplates(files []SourceFile) (*Result, error) {
	result := &Result{Converter: TemplateName, Accounts: map[string]string{}}
	var problems Problems
	for _, f := range files {
		t, err := readTable(f)
		if err != nil {
			return nil, err
		}
		known := map[string]bool{}
		for _, c := range Columns {
			known[c] = true
		}
		for _, h := range t.header {
			if h != "" && !known[h] {
				problems = append(problems, Problem{File: f.Name, Row: 1, Column: h, Message: "unknown column"})
			}
		}
		for _, c := range requiredColumns {
			if t.column(c) < 0 {
				problems = append(problems, Problem{File: f.Name, Row: 1, Column: c, Message: "missing column"})
			}
		}
		if len(problems) > 0 {
			continue
		}
		for i, row := range t.rows {
			line, rowProblems := parseTemplateRow(f.Name, i+2, t.record(row))
			problems = append(problems, rowProblems...)
			if line != nil {
				result.Lines = append(result.Lines, *line)
			}
			if len(problems) >= 50 {
				return nil, problems
			}
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return result, nil
}

func parseTemplateRow(file string, row int, v map[string]string) (*Line, Problems) {
	var problems Problems
	bad := func(column, message string) {
		problems = append(problems, Problem{File: file, Row: row, Column: column, Message: message})
	}
	l := &Line{
		Platform: v["platform"], AccountRef: v["account_ref"], Wallet: v["wallet"], ExternalID: v["external_id"],
		Type: strings.ToUpper(v["type"]), Currency: strings.ToUpper(v["currency"]), CardRef: v["card_ref"],
		CardLast4: v["card_last4"], CardName: v["card_name"], Merchant: v["merchant"], RelatedID: v["related_id"],
		OriginalCurrency: strings.ToUpper(v["original_currency"]), Remark: v["remark"], File: file, Row: row,
	}
	if !codePattern.MatchString(l.Platform) {
		bad("platform", "must be uppercase letters, digits or underscore")
	}
	if !codePattern.MatchString(l.Wallet) {
		bad("wallet", "must be uppercase letters, digits or underscore")
	}
	if l.AccountRef == "" || len(l.AccountRef) > 64 {
		bad("account_ref", "required, at most 64 characters")
	}
	if l.ExternalID == "" || len(l.ExternalID) > 128 {
		bad("external_id", "required, at most 128 characters")
	}
	if _, ok := Sign[l.Type]; !ok {
		bad("type", "unknown type")
	}
	if !currencyPattern.MatchString(l.Currency) {
		bad("currency", "must be a three-letter ISO code")
	}
	at, err := ParseTime(v["occurred_at"])
	if err != nil {
		bad("occurred_at", "must be a date-time with time zone, e.g. 2026-10-22 18:16:39+08:00")
	}
	l.OccurredAt = at
	if l.Amount, err = Money(v["amount"], l.Currency); err != nil {
		bad("amount", err.Error())
	}
	optional := func(column, currency string, target **decimal.Decimal) {
		if v[column] == "" {
			return
		}
		d, parseErr := Money(v[column], currency)
		if parseErr != nil {
			bad(column, parseErr.Error())
			return
		}
		*target = &d
	}
	optional("balance_after", l.Currency, &l.BalanceAfter)
	optional("fee_amount", l.Currency, &l.FeeAmount)
	if l.OriginalCurrency != "" && !currencyPattern.MatchString(l.OriginalCurrency) {
		bad("original_currency", "must be a three-letter ISO code")
	}
	optional("original_amount", l.OriginalCurrency, &l.OriginalAmount)
	if l.CardLast4 != "" && !last4Pattern.MatchString(l.CardLast4) {
		bad("card_last4", "must be four digits")
	}
	for column, limit := range map[string]int{"card_ref": 128, "card_name": 128, "merchant": 255, "related_id": 128, "remark": 500} {
		if len(v[column]) > limit {
			bad(column, "too long")
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return l, nil
}

// ParseTime accepts ISO 8601 date-times that carry a zone offset.
func ParseTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	var err error
	for _, layout := range timeLayouts {
		var t time.Time
		if t, err = time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, err
}

// Money parses a plain decimal (no thousands separators) within the currency's precision.
func Money(raw, currency string) (decimal.Decimal, error) {
	raw = strings.TrimSpace(raw)
	d, err := decimal.NewFromString(raw)
	if err != nil || raw == "" || strings.ContainsAny(raw, ",eE") || len(raw) > 40 {
		return decimal.Zero, errInvalidAmount
	}
	digits := 2
	if currency != "" {
		digits = payment.CurrencyMaxFractionDigits(currency)
	}
	if !d.Truncate(int32(digits)).Equal(d) {
		return decimal.Zero, errPrecision
	}
	if d.Abs().GreaterThanOrEqual(decimal.New(1, 18)) {
		return decimal.Zero, errInvalidAmount
	}
	return d, nil
}

type amountError string

func (e amountError) Error() string { return string(e) }

const (
	errInvalidAmount = amountError("must be a plain decimal number")
	errPrecision     = amountError("has more decimal places than the currency allows")
)

// WriteCSV renders lines as a standard template with a UTF-8 BOM so spreadsheet tools read it correctly.
func WriteCSV(lines []Line) []byte {
	var buf bytes.Buffer
	_, _ = buf.WriteString("\xef\xbb\xbf")
	w := csv.NewWriter(&buf)
	_ = w.Write(Columns)
	text := func(d *decimal.Decimal) string {
		if d == nil {
			return ""
		}
		return d.StringFixed(2)
	}
	for _, l := range lines {
		digits := int32(payment.CurrencyMaxFractionDigits(l.Currency))
		fixed := func(d *decimal.Decimal) string {
			if d == nil {
				return ""
			}
			return d.StringFixed(digits)
		}
		original := text(l.OriginalAmount)
		if l.OriginalAmount != nil && l.OriginalCurrency != "" {
			original = l.OriginalAmount.StringFixed(int32(payment.CurrencyMaxFractionDigits(l.OriginalCurrency)))
		}
		_ = w.Write([]string{
			l.Platform, l.AccountRef, l.Wallet, l.ExternalID, l.OccurredAt.Format("2006-01-02 15:04:05Z07:00"), l.Type,
			l.Amount.StringFixed(digits), l.Currency, fixed(l.BalanceAfter), fixed(l.FeeAmount), l.CardRef, l.CardLast4,
			l.CardName, l.Merchant, l.RelatedID, original, l.OriginalCurrency, l.Remark,
		})
	}
	w.Flush()
	return buf.Bytes()
}
