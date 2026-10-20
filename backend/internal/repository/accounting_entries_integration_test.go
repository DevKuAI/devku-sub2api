//go:build integration

package repository

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (f *accountingFixture) addEntry(t *testing.T, order, kind, currency, total string, details map[string]any, parent string) *accounting.Record {
	t.Helper()
	payload := map[string]any{"kind": kind, "currency": currency, "amount": total, "details": details, "parent_id": parent}
	var record accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "orders/"+order+"/entries", uuid.NewString(), payload)), &record))
	return &record
}

func (f *accountingFixture) receiptDetails(t *testing.T, total, reference string) map[string]any {
	return map[string]any{"payer_name": "Customer", "receiving_account_id": f.receivingAccount(t), "sales_amount": total, "tax_inclusive": true, "tax_rate": "0.06", "bank_reference": reference}
}

func entryPath(order, kind, id string) string {
	return "orders/" + order + "/entries/" + kind + "/" + id
}

func TestAccountingOrderEntriesTakeEffectWithoutProof(t *testing.T) {
	f := newAccountingFixture(t)
	order := f.newOrder(t, "Claude seats", nil)

	receipt := f.addEntry(t, order.ID, "transfers", "CNY", "1000", f.receiptDetails(t, "1000", uuid.NewString()), "")
	require.Equal(t, "RECEIVED", receipt.Status)

	// Fully digital invoices have no code.
	invoice := f.addEntry(t, order.ID, "invoices", "CNY", "1000", map[string]any{"invoice_number": uuid.NewString(), "issuer_name": "Us", "recipient_name": "Customer", "net_amount": "943.4", "tax_amount": "56.6"}, "")
	require.Equal(t, "VERIFIED", invoice.Status)
	require.Equal(t, "SALES", invoice.Details["invoice_type"])

	card := f.addEntry(t, order.ID, "vcc-cards", "CNY", "0", map[string]any{"alias": "Seat card", "provider": "Anthropic", "card_number": testCardNumber()}, "")
	require.Equal(t, "ACTIVE", card.Status)
	// The provider's transaction number is optional.
	spend := f.addEntry(t, order.ID, "vcc-transactions", "CNY", "300", map[string]any{"transaction_type": "CONSUME", "merchant": "Anthropic"}, card.ID)
	require.Equal(t, "CONFIRMED", spend.Status)
	var funded string
	require.NoError(t, integrationDB.QueryRow("SELECT transfer_id::text FROM accounting_allocations WHERE card_id=$1", card.ID).Scan(&funded))
	require.Equal(t, receipt.ID, funded)

	bill := f.addEntry(t, order.ID, "vendor-invoices", "CNY", "300", map[string]any{"vendor": "Anthropic", "period_start": "2026-10-01", "period_end": "2026-10-31"}, "")
	require.Equal(t, "CONFIRMED", bill.Status)
	// Proof can still be attached after the AI invoice took effect.
	f.proof(t, "vendor-invoices", bill)

	current := f.readOrder(t, order.ID)
	require.Equal(t, "COMPLETED", current.State)
	require.Equal(t, "1", current.Items["vendor-invoices"][0].Details["attachment_count"])
	require.Equal(t, "0", current.Items["transfers"][0].Details["attachment_count"])
	require.Equal(t, accounting.OrderProgress{Receipt: true, Invoice: true, Purchase: true, AIInvoice: true}, current.Progress)

	// Entries of other kinds are refused.
	res := f.request("POST", "orders/"+order.ID+"/entries", uuid.NewString(), map[string]any{"kind": "expenses", "currency": "CNY", "amount": "1"})
	require.Equal(t, 400, res.Code)
}

func TestAccountingOrderEntriesCanBeCorrectedAndRemoved(t *testing.T) {
	f := newAccountingFixture(t)
	order := f.newOrder(t, "GPT seats", nil)
	reference := uuid.NewString()
	receipt := f.addEntry(t, order.ID, "transfers", "CNY", "1000", f.receiptDetails(t, "1000", reference), "")
	card := f.addEntry(t, order.ID, "vcc-cards", "CNY", "0", map[string]any{"alias": "Seat card", "provider": "OpenAI", "card_number": testCardNumber()}, "")
	spend := f.addEntry(t, order.ID, "vcc-transactions", "CNY", "300", map[string]any{"transaction_type": "CONSUME", "merchant": "OpenAI"}, card.ID)
	bill := f.addEntry(t, order.ID, "vendor-invoices", "CNY", "300", map[string]any{"vendor": "OpenAI", "period_start": "2026-10-01", "period_end": "2026-10-31"}, "")

	// Receipts take descriptive and amount corrections in place, but never below what was spent from them.
	var corrected accounting.Record
	update := map[string]any{"amount": "1200", "version": receipt.Version, "details": map[string]any{"bank_reference": reference + "-B", "sales_amount": "1200"}}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("PATCH", entryPath(order.ID, "transfers", receipt.ID), uuid.NewString(), update)), &corrected))
	require.Equal(t, receipt.ID, corrected.ID)
	require.Equal(t, "1200", corrected.Amount)
	require.Equal(t, reference+"-B", corrected.Details["bank_reference"])
	tooLow := map[string]any{"amount": "100", "version": corrected.Version, "details": map[string]any{"sales_amount": "100"}}
	require.Equal(t, 409, f.request("PATCH", entryPath(order.ID, "transfers", receipt.ID), uuid.NewString(), tooLow).Code)
	stale := map[string]any{"version": receipt.Version, "details": map[string]any{"remark": "stale"}}
	require.Equal(t, 409, f.request("PATCH", entryPath(order.ID, "transfers", receipt.ID), uuid.NewString(), stale).Code)
	locked := map[string]any{"version": corrected.Version, "details": map[string]any{"receiving_account_id": uuid.NewString()}}
	require.Equal(t, 409, f.request("PATCH", entryPath(order.ID, "transfers", receipt.ID), uuid.NewString(), locked).Code)

	// A different spend amount replaces the spend and the AI invoice is matched again.
	var replaced accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("PATCH", entryPath(order.ID, "vcc-transactions", spend.ID), uuid.NewString(), map[string]any{"amount": "250", "version": spend.Version})), &replaced))
	require.NotEqual(t, spend.ID, replaced.ID)
	require.Equal(t, "250", replaced.Amount)
	var status string
	require.NoError(t, integrationDB.QueryRow("SELECT status FROM accounting_vcc_transactions WHERE id=$1", spend.ID).Scan(&status))
	require.Equal(t, "VOID", status)
	current := f.readOrder(t, order.ID)
	require.True(t, current.Progress.AIInvoice)
	require.Equal(t, "250", current.Totals[0].Spent)

	// A receipt that funds card spend stays; once the spend is removed the receipt can go too.
	require.Equal(t, 409, f.request("DELETE", entryPath(order.ID, "transfers", receipt.ID), uuid.NewString(), nil).Code)
	accountingData(t, f.request("DELETE", entryPath(order.ID, "vcc-transactions", replaced.ID), uuid.NewString(), nil))
	accountingData(t, f.request("DELETE", entryPath(order.ID, "transfers", receipt.ID), uuid.NewString(), nil))
	// The removed receipt's bank reference can be entered again.
	f.addEntry(t, order.ID, "transfers", "CNY", "1200", f.receiptDetails(t, "1200", reference+"-B"), "")

	accountingData(t, f.request("DELETE", entryPath(order.ID, "vendor-invoices", bill.ID), uuid.NewString(), nil))
	var active int
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM accounting_cost_allocations WHERE invoice_id=$1 AND status='ACTIVE'", bill.ID).Scan(&active))
	require.Zero(t, active)

	// Removing a card closes it.
	accountingData(t, f.request("DELETE", entryPath(order.ID, "vcc-cards", card.ID), uuid.NewString(), nil))
	require.NoError(t, integrationDB.QueryRow("SELECT status FROM accounting_vcc_cards WHERE id=$1", card.ID).Scan(&status))
	require.Equal(t, "CLOSED", status)

	// Entries of another order are not reachable through this one.
	other := f.newOrder(t, "Other", nil)
	require.Equal(t, 404, f.request("DELETE", entryPath(other.ID, "vendor-invoices", bill.ID), uuid.NewString(), nil).Code)
}

func TestAccountingOrdersListAcrossOrganizations(t *testing.T) {
	f := newAccountingFixture(t)
	name := "Gemini " + uuid.NewString()
	order := f.newOrder(t, name, nil)
	var page struct {
		Items []accounting.Order `json:"items"`
		Total int                `json:"total"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "orders?search="+url.QueryEscape(order.Name), "", nil)), &page))
	require.Equal(t, 1, page.Total)
	require.Equal(t, order.ID, page.Items[0].ID)
	require.Equal(t, f.desktop.organization.Name, page.Items[0].Organization)
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "orders?search="+uuid.NewString(), "", nil)), &page))
	require.Zero(t, page.Total)
}
