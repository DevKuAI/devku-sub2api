//go:build integration

package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/shopspring/decimal"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func accountingObject(t *testing.T, r *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(accountingData(t, r), &out))
	return out
}
func accountingSummary(t *testing.T, f *accountingFixture) []accounting.CurrencySummary {
	t.Helper()
	var out []accounting.CurrencySummary
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "summary?organization_id="+f.desktop.organization.PublicID, "", nil)), &out))
	return out
}
func TestAccountingWorkflowReceivablesAndCorrectedMetrics(t *testing.T) {
	f := newAccountingFixture(t)
	expense := f.create(t, "expenses", "100", map[string]any{"category": "service", "supplier": "Vendor", "payment_account": "Bank"}, "")
	require.Zero(t, accountingSummary(t, f)[0].Unmatched)
	f.confirm(t, "expenses", expense)
	require.Equal(t, int64(1), accountingSummary(t, f)[0].Unmatched)
	var reconciliation struct {
		Items []any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "reconciliation?organization_id="+expense.OrganizationID, "", nil)), &reconciliation))
	require.Len(t, reconciliation.Items, 1)
	receipt := f.transfer(t, "1000")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	accountingData(t, f.request("POST", "reconciliation/adjustments", uuid.NewString(), map[string]any{"object_kind": "transfers", "object_id": receipt.ID, "currency": "CNY", "amount": "200", "reason": "confirmed additional receipt"}))
	var trend []map[string]string
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "trend?organization_id="+receipt.OrganizationID, "", nil)), &trend))
	assertAccountingAmount(t, "1200", trend[0]["amount"])
	ar := f.create(t, "receivables", "1000", map[string]any{"name": "Service contract", "reference": uuid.NewString(), "due_date": "2026-01-01"}, "")
	f.confirm(t, "receivables", ar)
	path := "receivables/" + ar.ID + "/settlements"
	payload := map[string]any{"transfer_id": receipt.ID, "amount": "600"}
	key := uuid.NewString()
	first := accountingData(t, f.request("POST", path, key, payload))
	require.JSONEq(t, string(first), string(accountingData(t, f.request("POST", path, key, payload))))
	require.Equal(t, 409, f.request("POST", path, uuid.NewString(), map[string]string{"transfer_id": receipt.ID, "amount": "500"}).Code)
	accountingData(t, f.request("POST", path, uuid.NewString(), map[string]string{"transfer_id": receipt.ID, "amount": "400"}))
	require.Equal(t, 409, f.request("POST", "reconciliation/adjustments", uuid.NewString(), map[string]string{"object_kind": "transfers", "object_id": receipt.ID, "currency": "CNY", "amount": "-300", "reason": "must preserve settlements"}).Code)
	record := accountingObject(t, f.request("GET", "receivables/"+ar.ID, "", nil))
	assertAccountingAmount(t, "0", record["details"].(map[string]any)["outstanding_amount"])
	assertAccountingAmount(t, "1200", accountingSummary(t, f)[0].Received)
	require.Equal(t, 409, f.request("POST", "receivables/"+ar.ID+"/void", uuid.NewString(), map[string]string{"reason": "settled"}).Code)
	todo := accountingObject(t, f.request("GET", "workbench?organization_id="+ar.OrganizationID, "", nil))
	require.NotEmpty(t, todo["items"])
	export := f.request("GET", "export?kind=transfers&organization_id="+receipt.OrganizationID, "", nil)
	require.Equal(t, 200, export.Code)
	require.Contains(t, export.Body.String(), "bank_reference")
	require.Contains(t, export.Body.String(), "1200")
}
func TestAccountingWorkflowExchangeFundingAndExternalConfirmation(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "1000")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	fx := f.create(t, "fx-exchanges", "700", map[string]any{"target_currency": "USD", "target_amount": "98", "exchange_rate": "0.14", "fee_amount": "10", "reference": uuid.NewString()}, receipt.ID)
	f.proof(t, "fx-exchanges", fx)
	f.confirm(t, "fx-exchanges", fx)
	var card accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "vcc-cards", uuid.NewString(), accounting.Input{OrganizationID: receipt.OrganizationID, Currency: "USD", Details: map[string]any{"provider": "Provider", "alias": "USD card", "provider_card_id": testCardID(), "last_four": "4242"}, OrderID: f.order(t)})), &card))
	accountingData(t, f.request("POST", "fx-exchanges/"+fx.ID+"/allocations", uuid.NewString(), map[string]string{"source_id": card.ID, "amount": "50"}))
	byCurrency := map[string]accounting.CurrencySummary{}
	for _, s := range accountingSummary(t, f) {
		byCurrency[s.Currency] = s
	}
	assertAccountingAmount(t, "1000", byCurrency["CNY"].Received)
	assertAccountingAmount(t, "290", byCurrency["CNY"].Unallocated)
	assertAccountingAmount(t, "0", byCurrency["USD"].Received)
	assertAccountingAmount(t, "48", byCurrency["USD"].Unallocated)
	assertAccountingAmount(t, "50", byCurrency["USD"].Balance)
	require.Equal(t, 400, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "500"}).Code)
	require.Equal(t, 409, f.request("POST", "fx-exchanges/"+fx.ID+"/void", uuid.NewString(), map[string]string{"reason": "target funds in use"}).Code)
	var actions []map[string]any
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "external-actions?organization_id="+receipt.OrganizationID, "", nil)), &actions))
	require.Len(t, actions, 1)
	require.Equal(t, "PENDING", actions[0]["status"])
	require.Equal(t, 400, f.request("POST", "external-actions/"+actions[0]["id"].(string)+"/confirm", uuid.NewString(), map[string]string{"reference": "provider-1"}).Code)
	proof := f.proof(t, "vcc-cards", &card)
	accountingData(t, f.request("POST", "external-actions/"+actions[0]["id"].(string)+"/confirm", uuid.NewString(), map[string]string{"reference": "provider-1", "attachment_id": proof}))
}
func TestAccountingWorkflowRelatedActivityAndResidualTodos(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	accountingData(t, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "60"}))
	var related []map[string]any
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "related/transfers/"+receipt.ID, "", nil)), &related))
	require.NotEmpty(t, related)
	require.Equal(t, card.ID, related[0]["id"])
	var events []map[string]any
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "activity/vcc-cards/"+card.ID, "", nil)), &events))
	require.GreaterOrEqual(t, len(events), 2)
	todo := accountingObject(t, f.request("GET", "workbench?organization_id="+receipt.OrganizationID+"&reason=unallocated", "", nil))
	items := todo["items"].([]any)
	require.Len(t, items, 1)
	assertAccountingAmount(t, "40", items[0].(map[string]any)["amount"])
}

func TestAccountingWorkflowCostCannotCountTheSamePurchaseTwice(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	accountingData(t, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "100"}))
	purchase := f.transaction(t, card, "CONSUME", "100", "")
	f.confirm(t, "vcc-transactions", purchase)
	invoice := f.create(t, "vendor-invoices", "100", map[string]any{"vendor": "Vendor", "period_start": "2026-01-01", "period_end": "2026-01-31"}, "")
	f.proof(t, "vendor-invoices", invoice)
	f.confirm(t, "vendor-invoices", invoice)
	expense := f.create(t, "expenses", "100", map[string]any{"category": "service", "supplier": "Vendor", "payment_account": "Card", "transaction_id": purchase.ID}, "")
	f.confirm(t, "expenses", expense)
	mapped := accountingObject(t, f.request("GET", "expenses/"+expense.ID, "", nil))
	require.Equal(t, "PAID", mapped["status"])
	payload := map[string]string{"organization_id": receipt.OrganizationID, "invoice_id": invoice.ID, "source_kind": "vcc-transactions", "source_id": purchase.ID, "amount": "60"}
	first := accountingObject(t, f.request("POST", "cost-allocations", uuid.NewString(), payload))
	payload["source_kind"] = "expenses"
	payload["source_id"] = expense.ID
	payload["amount"] = "50"
	require.Equal(t, 409, f.request("POST", "cost-allocations", uuid.NewString(), payload).Code)
	payload["amount"] = "40"
	second := accountingObject(t, f.request("POST", "cost-allocations", uuid.NewString(), payload))
	refund := f.transaction(t, card, "REFUND", "10", purchase.ID)
	require.Equal(t, 409, f.request("POST", "vcc-transactions/"+refund.ID+"/confirm", uuid.NewString(), map[string]any{}).Code)
	require.Equal(t, 409, f.request("POST", "vendor-invoices/"+invoice.ID+"/void", uuid.NewString(), map[string]string{"reason": "allocated"}).Code)
	for _, a := range []map[string]any{first, second} {
		accountingData(t, f.request("POST", "cost-allocations/"+a["id"].(string)+"/void", uuid.NewString(), map[string]string{"reason": "refund reconciliation"}))
	}
	f.confirm(t, "vcc-transactions", refund)
}
func TestAccountingWorkflowCloseProtectsBackdatedWritesAndPreservesStatements(t *testing.T) {
	f := newAccountingFixture(t)
	month := time.Now().UTC().AddDate(0, -2, 0).Format("2006-01")
	receipt := f.transfer(t, "1000")
	accountingData(t, f.request("PATCH", "transfers/"+receipt.ID, uuid.NewString(), accounting.Input{OrganizationID: receipt.OrganizationID, Currency: "CNY", Amount: "1000", OccurredAt: month + "-10T02:00:00Z", Version: receipt.Version, Details: receipt.Details}))
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	accountingData(t, f.request("POST", "reconciliation/match", uuid.NewString(), map[string]string{"object_kind": "transfers", "object_id": receipt.ID, "currency": "CNY", "amount": "1000"}))
	closePayload := map[string]any{"organization_id": receipt.OrganizationID, "month": month, "reason": "period checked"}
	closed := accountingObject(t, f.request("POST", "periods/close", uuid.NewString(), closePayload))
	statementID := closed["statement_id"].(string)
	snapshot := accountingData(t, f.request("GET", "statements/"+statementID, "", nil))
	require.NotContains(t, string(snapshot), "opening\":null")
	require.Equal(t, 409, f.request("POST", "reconciliation/adjustments", uuid.NewString(), map[string]string{"object_kind": "transfers", "object_id": receipt.ID, "currency": "CNY", "amount": "1", "reason": "locked"}).Code)
	in := accounting.Input{OrganizationID: receipt.OrganizationID, Currency: "CNY", Amount: "5", OccurredAt: month + "-15T02:00:00Z", Details: map[string]any{"payer_name": "Customer", "receiving_account": "Bank", "sales_amount": "5", "tax_inclusive": true, "tax_rate": "0", "bank_reference": uuid.NewString()}}
	require.Equal(t, 409, f.request("POST", "transfers", uuid.NewString(), in).Code)
	accountingData(t, f.request("POST", "periods/"+closed["id"].(string)+"/reopen", uuid.NewString(), map[string]string{"reason": "correction authorized"}))
	accountingData(t, f.request("POST", "reconciliation/adjustments", uuid.NewString(), map[string]string{"object_kind": "transfers", "object_id": receipt.ID, "currency": "CNY", "amount": "10", "reason": "correction"}))
	accountingData(t, f.request("POST", "reconciliation/match", uuid.NewString(), map[string]string{"object_kind": "transfers", "object_id": receipt.ID, "currency": "CNY", "amount": "1010"}))
	newClose := accountingObject(t, f.request("POST", "periods/close", uuid.NewString(), closePayload))
	require.Equal(t, float64(2), newClose["revision"])
	require.JSONEq(t, string(snapshot), string(accountingData(t, f.request("GET", "statements/"+statementID, "", nil))))
}
func TestAccountingWorkflowFinanceCannotAuthorizeOverpaymentOrReopen(t *testing.T) {
	f := newAccountingFixture(t)
	expense := f.create(t, "expenses", "100", map[string]any{"category": "service", "supplier": "Vendor", "payment_account": "Bank"}, "")
	proof := f.proof(t, "expenses", expense)
	payload := accounting.Input{Currency: "CNY", Amount: "101", Details: map[string]any{"payment_account": "Bank", "transaction_number": uuid.NewString(), "proof_attachment_id": proof, "override": true, "reason": "needs administrator"}}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "/api/v1/admin/accounting/expenses/"+expense.ID+"/payments", bytes.NewReader(body))
	req.Header.Set("X-Test-Role", "finance")
	req.Header.Set("Idempotency-Key", uuid.NewString())
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	require.Equal(t, 403, w.Code)
	req = httptest.NewRequest("POST", "/api/v1/admin/accounting/periods/close", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Test-Role", "finance")
	w = httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	require.Equal(t, 403, w.Code)
}

func assertAccountingAmount(t *testing.T, expected string, actual any) {
	t.Helper()
	require.True(t, decimal.RequireFromString(expected).Equal(decimal.RequireFromString(fmt.Sprint(actual))), "expected %s, got %v", expected, actual)
}

func TestAccountingWorkflowCrossCurrencyCostsRequireExplicitRate(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	accountingData(t, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "100"}))
	purchase := f.transaction(t, card, "CONSUME", "100", "")
	f.confirm(t, "vcc-transactions", purchase)
	var bill accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "vendor-invoices", uuid.NewString(), accounting.Input{OrganizationID: receipt.OrganizationID, Currency: "USD", Amount: "14", Details: map[string]any{"vendor": "Vendor", "period_start": "2026-01-01", "period_end": "2026-01-31"}})), &bill))
	f.proof(t, "vendor-invoices", &bill)
	f.confirm(t, "vendor-invoices", &bill)
	payload := map[string]string{"organization_id": receipt.OrganizationID, "invoice_id": bill.ID, "source_kind": "vcc-transactions", "source_id": purchase.ID, "amount": "14"}
	require.Equal(t, 400, f.request("POST", "cost-allocations", uuid.NewString(), payload).Code)
	payload["source_amount"] = "100"
	payload["exchange_rate"] = "0.13"
	require.Equal(t, 400, f.request("POST", "cost-allocations", uuid.NewString(), payload).Code)
	payload["exchange_rate"] = "0.14"
	accountingData(t, f.request("POST", "cost-allocations", uuid.NewString(), payload))
	var allocations []map[string]any
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "cost-allocations?organization_id="+receipt.OrganizationID, "", nil)), &allocations))
	require.Len(t, allocations, 1)
	assertAccountingAmount(t, "100", allocations[0]["source_amount"])
	assertAccountingAmount(t, "14", allocations[0]["amount"])
	require.NotEmpty(t, allocations[0]["source_name"])
	var costs []map[string]any
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "cost-summary?organization_id="+receipt.OrganizationID, "", nil)), &costs))
	byCurrency := map[string]map[string]any{}
	for _, c := range costs {
		byCurrency[c["currency"].(string)] = c
	}
	assertAccountingAmount(t, "100", byCurrency["CNY"]["book_cost"])
	assertAccountingAmount(t, "0", byCurrency["USD"]["unallocated_cost"])
}
func TestAccountingWorkflowStatementPrivacyAndPartialCoverage(t *testing.T) {
	f := newAccountingFixture(t)
	month := time.Now().UTC().AddDate(0, -2, 0).Format("2006-01")
	receipt := f.transfer(t, "100")
	accountingData(t, f.request("PATCH", "transfers/"+receipt.ID, uuid.NewString(), accounting.Input{OrganizationID: receipt.OrganizationID, Currency: "CNY", Amount: "100", OccurredAt: month + "-10T02:00:00Z", Version: receipt.Version, Details: receipt.Details}))
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	accountingData(t, f.request("POST", "reconciliation/match", uuid.NewString(), map[string]string{"object_kind": "transfers", "object_id": receipt.ID, "currency": "CNY", "amount": "100"}))
	_, err := integrationDB.Exec("UPDATE accounting_position_baselines SET covered_from=$2 WHERE organization_id=$1", f.desktop.organization.ID, month+"-05T00:00:00Z")
	require.NoError(t, err)
	payload := map[string]any{"organization_id": receipt.OrganizationID, "month": month, "reason": "initial coverage acknowledged"}
	require.Equal(t, 400, f.request("POST", "periods/close", uuid.NewString(), payload).Code)
	payload["accept_partial"] = true
	closed := accountingObject(t, f.request("POST", "periods/close", uuid.NewString(), payload))
	id := closed["statement_id"].(string)
	req := httptest.NewRequest("GET", "/api/v1/organization/accounting/statements/"+id, nil)
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	public := string(accountingData(t, w))
	require.Contains(t, public, "opening\":null")
	require.NotContains(t, public, "INTERNAL")
	require.NotContains(t, public, "receiving_account")
	require.Contains(t, public, receipt.OrganizationID)
	req = httptest.NewRequest("GET", "/api/v1/organization/accounting/periods", nil)
	w = httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	require.NotContains(t, string(accountingData(t, w)), "initial coverage acknowledged")
	require.Equal(t, 409, f.request("DELETE", "attachments/"+f.proof(t, "transfers", receipt), uuid.NewString(), map[string]any{}).Code)
}
