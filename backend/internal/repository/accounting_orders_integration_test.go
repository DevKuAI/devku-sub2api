//go:build integration

package repository

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (f *accountingFixture) newOrder(t *testing.T, name string, receipt *accounting.Input) *accounting.Order {
	t.Helper()
	payload := map[string]any{"organization_id": f.desktop.organization.PublicID, "name": name}
	if receipt != nil {
		payload["transfer"] = receipt
	}
	var order accounting.Order
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "orders", uuid.NewString(), payload)), &order))
	return &order
}

func (f *accountingFixture) readOrder(t *testing.T, id string) *accounting.Order {
	t.Helper()
	var order accounting.Order
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "orders/"+id, "", nil)), &order))
	return &order
}

func (f *accountingFixture) createIn(t *testing.T, order, kind, currency, total string, details map[string]any, parent string) *accounting.Record {
	t.Helper()
	in := accounting.Input{OrganizationID: f.desktop.organization.PublicID, Currency: currency, Amount: total, Details: details, ParentID: parent, OrderID: order}
	var record accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", kind, uuid.NewString(), in)), &record))
	return &record
}

func (f *accountingFixture) receiptInput(t *testing.T, total string) *accounting.Input {
	return &accounting.Input{Currency: "CNY", Amount: total, Details: map[string]any{"payer_name": "Customer", "receiving_account_id": f.receivingAccount(t), "sales_amount": total, "tax_inclusive": true, "tax_rate": "0.06", "bank_reference": uuid.NewString()}}
}

func TestAccountingOrderRunsFromReceiptToCompletion(t *testing.T) {
	f := newAccountingFixture(t)
	order := f.newOrder(t, "Q4 ChatGPT seats", f.receiptInput(t, "1000"))
	require.Equal(t, "OPEN", order.State)

	current := f.readOrder(t, order.ID)
	require.Len(t, current.Items["transfers"], 1)
	receipt := current.Items["transfers"][0]
	require.Equal(t, order.ID, receipt.Details["order_id"])
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	require.True(t, f.readOrder(t, order.ID).Progress.Receipt)

	// Cards and card spend only exist inside an order.
	outside := accounting.Input{OrganizationID: f.desktop.organization.PublicID, Currency: "CNY", Details: map[string]any{"alias": "Loose", "provider": "Provider", "provider_card_id": testCardID(), "last_four": "4242"}}
	require.Equal(t, 400, f.request("POST", "vcc-cards", uuid.NewString(), outside).Code)
	card := f.createIn(t, order.ID, "vcc-cards", "CNY", "0", map[string]any{"alias": "Seat card", "provider": "Provider", "provider_card_id": testCardID(), "last_four": "4242"}, "")
	spendOutside := accounting.Input{OrganizationID: f.desktop.organization.PublicID, ParentID: card.ID, Currency: "CNY", Amount: "1", Details: map[string]any{"transaction_type": "CONSUME", "merchant": "OpenAI", "provider_transaction_id": uuid.NewString()}}
	require.Equal(t, 400, f.request("POST", "vcc-transactions", uuid.NewString(), spendOutside).Code)

	// Card spend only counts against the card; it does not draw on the receipt or ask the provider for anything.
	spend := f.createIn(t, order.ID, "vcc-transactions", "CNY", "300", map[string]any{"transaction_type": "CONSUME", "merchant": "OpenAI", "provider_transaction_id": uuid.NewString()}, card.ID)
	f.confirm(t, "vcc-transactions", spend)
	require.Equal(t, "300", f.cardSpent(t, card.ID))
	var allocations, pending int
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM accounting_allocations WHERE card_id=$1", card.ID).Scan(&allocations))
	require.Zero(t, allocations)
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM accounting_external_actions WHERE card_id=$1", card.ID).Scan(&pending))
	require.Zero(t, pending)
	require.True(t, f.readOrder(t, order.ID).Progress.Purchase)

	invoice := f.createIn(t, order.ID, "invoices", "CNY", "1000", map[string]any{"invoice_type": "SALES", "invoice_code": "CODE", "invoice_number": uuid.NewString(), "issuer_name": "Us", "recipient_name": "Customer", "net_amount": "943.4", "tax_amount": "56.6"}, "")
	f.proof(t, "invoices", invoice)
	f.confirm(t, "invoices", invoice)
	purchaseInvoice := accounting.Input{OrganizationID: f.desktop.organization.PublicID, Currency: "CNY", Amount: "106", OrderID: order.ID, Details: map[string]any{"invoice_type": "PURCHASE", "invoice_code": "CODE", "invoice_number": uuid.NewString(), "issuer_name": "Vendor", "recipient_name": "Us", "net_amount": "100", "tax_amount": "6"}}
	require.Equal(t, 400, f.request("POST", "invoices", uuid.NewString(), purchaseInvoice).Code)
	require.False(t, f.readOrder(t, order.ID).Progress.AIInvoice)

	// The AI invoice is matched to the order's purchase as soon as it is confirmed.
	bill := f.createIn(t, order.ID, "vendor-invoices", "CNY", "300", map[string]any{"vendor": "OpenAI", "period_start": "2026-10-01", "period_end": "2026-10-31"}, "")
	f.proof(t, "vendor-invoices", bill)
	f.confirm(t, "vendor-invoices", bill)
	var cost string
	require.NoError(t, integrationDB.QueryRow("SELECT amount::text FROM accounting_cost_allocations WHERE invoice_id=$1 AND source_id=$2 AND status='ACTIVE'", bill.ID, spend.ID).Scan(&cost))
	require.Equal(t, "300.000000", cost)

	done := f.readOrder(t, order.ID)
	require.Equal(t, "COMPLETED", done.State)
	require.Equal(t, accounting.OrderProgress{Receipt: true, Invoice: true, Purchase: true, AIInvoice: true}, done.Progress)
	require.Equal(t, []accounting.OrderTotal{{Currency: "CNY", Received: "1000", Invoiced: "1000", Spent: "300", AICost: "300"}}, done.Totals)
	var list struct {
		Items []accounting.Order `json:"items"`
		Total int                `json:"total"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "orders?state=completed&organization_id="+f.desktop.organization.PublicID, "", nil)), &list))
	require.Equal(t, 1, list.Total)
	require.Equal(t, order.ID, list.Items[0].ID)
}

// cardSpent reads a card's net confirmed spend as the API reports it.
func (f *accountingFixture) cardSpent(t *testing.T, id string) string {
	t.Helper()
	var current accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "vcc-cards/"+id, "", nil)), &current))
	require.NotContains(t, current.Details, "available_balance")
	return current.Details["spent_amount"].(string)
}

func TestAccountingOrderCrossCurrencySpendNeedsNoFunding(t *testing.T) {
	f := newAccountingFixture(t)
	order := f.newOrder(t, "USD seats", f.receiptInput(t, "1000"))
	receipt := f.readOrder(t, order.ID).Items["transfers"][0]
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.createIn(t, order.ID, "vcc-cards", "USD", "0", map[string]any{"alias": "USD card", "provider": "Provider", "provider_card_id": testCardID(), "last_four": "4242"}, "")
	spend := f.createIn(t, order.ID, "vcc-transactions", "USD", "50", map[string]any{"transaction_type": "CONSUME", "merchant": "Anthropic", "provider_transaction_id": uuid.NewString()}, card.ID)
	f.confirm(t, "vcc-transactions", spend)

	// Spend in another currency than the receipt is simply spend; there is no balance to top up.
	require.Equal(t, "50", f.cardSpent(t, card.ID))
	require.Equal(t, 404, f.request("POST", "vcc-cards/"+card.ID+"/top-ups", uuid.NewString(), map[string]string{"amount": "80"}).Code)
	require.Equal(t, 404, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "1"}).Code)
	totals := f.readOrder(t, order.ID).Totals
	require.Equal(t, []accounting.OrderTotal{{Currency: "CNY", Received: "1000", Invoiced: "0", Spent: "0", AICost: "0"}, {Currency: "USD", Received: "0", Invoiced: "0", Spent: "50", AICost: "0"}}, totals)

	// A legacy top-up stays out of every customer figure and list.
	topUp := uuid.NewString()
	_, err := integrationDB.Exec(`INSERT INTO accounting_transfers(id,organization_id,status,currency,amount,occurred_at,details,created_by,version) SELECT $1,organization_id,'RECEIVED','USD',80,NOW(),'{"funding_type":"TOPUP","payer_name":"Card top-up"}'::jsonb,created_by,1 FROM accounting_transfers WHERE id=$2`, topUp, receipt.ID)
	require.NoError(t, err)
	for _, entry := range accountingSummary(t, f) {
		if entry.Currency == "USD" {
			assertAccountingAmount(t, "0", entry.Received)
			assertAccountingAmount(t, "50", entry.Consumed)
		}
	}
	listed := f.request("GET", "transfers?organization_id="+f.desktop.organization.PublicID, "", nil).Body.String()
	require.NotContains(t, listed, topUp)
	req := httptest.NewRequest("GET", "/api/v1/organization/accounting/transfers", nil)
	req.Header.Set("X-Test-Role", "user")
	req.Header.Set("X-Test-Actor", strconv.FormatInt(f.desktop.user.ID, 10))
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	require.Equal(t, 200, res.Code)
	require.NotContains(t, res.Body.String(), topUp)
}

func TestAccountingOrderLinksCloseAndRefund(t *testing.T) {
	f := newAccountingFixture(t)
	order := f.newOrder(t, "Linked order", nil)
	other := f.newOrder(t, "Other order", nil)
	receipt := f.transfer(t, "500")
	link := func(id, kind, object string) int {
		return f.request("POST", "orders/"+id+"/links", uuid.NewString(), map[string]string{"object_kind": kind, "object_id": object}).Code
	}
	require.Equal(t, 200, link(order.ID, "transfers", receipt.ID))
	require.Equal(t, 409, link(other.ID, "transfers", receipt.ID))
	card := f.createIn(t, order.ID, "vcc-cards", "CNY", "0", map[string]any{"alias": "Card", "provider": "Provider", "provider_card_id": testCardID(), "last_four": "4242"}, "")
	require.Equal(t, 400, link(other.ID, "vcc-cards", card.ID))

	spend := f.createIn(t, order.ID, "vcc-transactions", "CNY", "40", map[string]any{"transaction_type": "CONSUME", "merchant": "OpenAI", "provider_transaction_id": uuid.NewString()}, card.ID)
	f.confirm(t, "vcc-transactions", spend)

	require.Equal(t, 400, f.request("POST", "orders/"+order.ID+"/close", uuid.NewString(), map[string]string{}).Code)
	closed := accountingObject(t, f.request("POST", "orders/"+order.ID+"/close", uuid.NewString(), map[string]string{"reason": "Customer needs no invoice"}))
	require.Equal(t, "CLOSED", closed["state"])
	require.Equal(t, 409, f.request("POST", "vcc-cards", uuid.NewString(), accounting.Input{OrganizationID: f.desktop.organization.PublicID, Currency: "CNY", OrderID: order.ID, Details: map[string]any{"alias": "Late", "provider": "Provider", "provider_card_id": testCardID(), "last_four": "4242"}}).Code)

	// A refund follows its purchase into the order, even after the order was closed.
	refund := f.createIn(t, "", "vcc-transactions", "CNY", "10", map[string]any{"transaction_type": "REFUND", "original_transaction_id": spend.ID, "merchant": "OpenAI", "provider_transaction_id": uuid.NewString()}, card.ID)
	require.Equal(t, order.ID, f.readOrder(t, order.ID).Items["vcc-transactions"][1].Details["order_id"])
	require.Equal(t, refund.ID, f.readOrder(t, order.ID).Items["vcc-transactions"][1].ID)

	reopened := accountingObject(t, f.request("POST", "orders/"+order.ID+"/reopen", uuid.NewString(), map[string]string{}))
	require.Equal(t, "OPEN", reopened["state"])
}

func TestAccountingCardBelongsToOneOrganization(t *testing.T) {
	first := newAccountingFixture(t)
	second := newAccountingFixture(t)
	cardID := testCardID()
	first.createIn(t, first.order(t), "vcc-cards", "USD", "0", map[string]any{"alias": "Shared?", "provider": "Provider", "provider_card_id": cardID, "last_four": "4242"}, "")
	again := accounting.Input{OrganizationID: second.desktop.organization.PublicID, Currency: "USD", OrderID: second.order(t), Details: map[string]any{"alias": "Copy", "provider": "Provider", "provider_card_id": cardID, "last_four": "4242"}}
	res := second.request("POST", "vcc-cards", uuid.NewString(), again)
	require.Equal(t, 409, res.Code)
	require.Contains(t, res.Body.String(), "card_registered")
	missing := accounting.Input{OrganizationID: second.desktop.organization.PublicID, Currency: "USD", OrderID: second.order(t), Details: map[string]any{"alias": "No card ID", "provider": "Provider", "last_four": "1234"}}
	require.Equal(t, 400, second.request("POST", "vcc-cards", uuid.NewString(), missing).Code)
}
