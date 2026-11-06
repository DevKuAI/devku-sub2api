//go:build integration

package repository

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func invoiceDetails(receipt string, split ...string) map[string]any {
	net, tax := "943.4", "56.6"
	if len(split) == 2 {
		net, tax = split[0], split[1]
	}
	details := map[string]any{"invoice_number": uuid.NewString(), "issuer_name": "Us", "recipient_name": "Customer", "net_amount": net, "tax_amount": tax}
	if receipt != "" {
		details["transfer_id"] = receipt
	}
	return details
}

// An order paid month by month gets one invoice per payment; the order counts as invoiced once every payment has one.
func TestAccountingInvoicesNameTheirPayment(t *testing.T) {
	f := newAccountingFixture(t)
	order := f.newOrder(t, "Monthly seats", nil)
	october := f.addEntry(t, order.ID, "transfers", "CNY", "1000", f.receiptDetails(t, "1000", uuid.NewString()), "")
	november := f.addEntry(t, order.ID, "transfers", "CNY", "1000", f.receiptDetails(t, "1000", uuid.NewString()), "")

	first := f.addEntry(t, order.ID, "invoices", "CNY", "1000", invoiceDetails(october.ID), "")
	require.Equal(t, october.ID, first.Details["transfer_id"])
	require.False(t, f.readOrder(t, order.ID).Progress.Invoice, "November is not invoiced yet")

	// A payment has one invoice; an invoice stays with a payment of its own currency and order.
	again := f.request("POST", "orders/"+order.ID+"/entries", uuid.NewString(), map[string]any{"kind": "invoices", "currency": "CNY", "amount": "1000", "details": invoiceDetails(october.ID)})
	require.Equal(t, 409, again.Code)
	require.Contains(t, again.Body.String(), "receipt_invoiced")
	dollars := f.request("POST", "orders/"+order.ID+"/entries", uuid.NewString(), map[string]any{"kind": "invoices", "currency": "USD", "amount": "140", "details": invoiceDetails(november.ID)})
	require.Equal(t, 400, dollars.Code)
	other := f.newOrder(t, "Other", nil)
	elsewhere := f.request("POST", "orders/"+other.ID+"/entries", uuid.NewString(), map[string]any{"kind": "invoices", "currency": "CNY", "amount": "1000", "details": invoiceDetails(november.ID)})
	require.Equal(t, 409, elsewhere.Code)
	require.Contains(t, elsewhere.Body.String(), "receipt_other_order")

	// An invoice issued before the money arrived names its payment later. Until then it covers the remaining payment
	// by amount, the way invoices recorded before they named payments do.
	second := f.addEntry(t, order.ID, "invoices", "CNY", "1000", invoiceDetails(""), "")
	require.True(t, f.readOrder(t, order.ID).Progress.Invoice)
	var linked accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("PATCH", entryPath(order.ID, "invoices", second.ID), uuid.NewString(), map[string]any{"version": second.Version, "details": map[string]any{"transfer_id": november.ID}})), &linked))
	require.Equal(t, november.ID, linked.Details["transfer_id"])
	require.True(t, f.readOrder(t, order.ID).Progress.Invoice)

	// An invoiced payment stays until its invoice goes.
	removed := f.request("DELETE", entryPath(order.ID, "transfers", october.ID), uuid.NewString(), nil)
	require.Equal(t, 409, removed.Code)
	require.Contains(t, removed.Body.String(), "receipt_invoiced")
	accountingData(t, f.request("DELETE", entryPath(order.ID, "invoices", first.ID), uuid.NewString(), nil))
	require.False(t, f.readOrder(t, order.ID).Progress.Invoice)
	accountingData(t, f.request("DELETE", entryPath(order.ID, "transfers", october.ID), uuid.NewString(), nil))
	require.True(t, f.readOrder(t, order.ID).Progress.Invoice, "the remaining payment is invoiced")
}

// Invoices recorded before they named payments still cover their payments by amount.
func TestAccountingUnlinkedInvoicesCoverPaymentsByAmount(t *testing.T) {
	f := newAccountingFixture(t)
	order := f.newOrder(t, "Older order", nil)
	f.addEntry(t, order.ID, "transfers", "CNY", "1000", f.receiptDetails(t, "1000", uuid.NewString()), "")
	f.addEntry(t, order.ID, "invoices", "CNY", "500", invoiceDetails("", "471.7", "28.3"), "")
	require.False(t, f.readOrder(t, order.ID).Progress.Invoice)
	f.addEntry(t, order.ID, "invoices", "CNY", "500", invoiceDetails("", "471.7", "28.3"), "")
	require.True(t, f.readOrder(t, order.ID).Progress.Invoice)
}

// A purchase may keep the customer's transfer screenshot; it is optional and changes nothing in the books.
func TestAccountingPurchaseKeepsPaymentProof(t *testing.T) {
	f := newAccountingFixture(t)
	card := f.card(t)
	spend := f.spend(t, card, "CONSUME", "200", nil)
	item := func() *accounting.Record {
		for _, r := range f.readOrder(t, f.order(t)).Items["vcc-transactions"] {
			if r.ID == spend.ID {
				return r
			}
		}
		t.Fatal("spend not in its order")
		return nil
	}
	require.Equal(t, "0", item().Details["attachment_count"])
	f.proof(t, "vcc-transactions", spend)
	require.Equal(t, "1", item().Details["attachment_count"])
	require.Equal(t, "200", f.cardSpent(t, card.ID))
}
