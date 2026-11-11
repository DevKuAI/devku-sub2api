package accounting

import "context"

// A sales invoice may name the receipt it was issued for (details.transfer_id): one invoice per customer payment, so
// an order paid month by month shows which payments are still to be invoiced. The link stays inside the income side
// of the books; card spend is not involved.

// checkInvoiceReceipt keeps the named receipt a real customer payment of the same organization, currency and order.
func checkInvoiceReceipt(ctx context.Context, q dbQuery, r *Record) error {
	id := detail(r, "transfer_id")
	if id == "" {
		return nil
	}
	if detail(r, "invoice_type") != "SALES" {
		return invalid("transfer_id")
	}
	receipt, err := get(ctx, q, "transfers", id, r.Org, false)
	if err != nil || receipt.Status == "VOID" || internalFunding(receipt) {
		return invalid("transfer_id")
	}
	if receipt.Currency != r.Currency {
		return ErrCurrency
	}
	invoiceOrder, err := orderOf(ctx, q, "invoices", r.ID)
	if err != nil || invoiceOrder == "" {
		return err
	}
	receiptOrder, err := orderOf(ctx, q, "transfers", id)
	if err != nil {
		return err
	}
	if receiptOrder != invoiceOrder {
		return ErrState.WithMetadata(map[string]string{"field": "receipt_other_order"})
	}
	return nil
}

// receiptInvoiced refuses to take back a receipt an effective invoice was issued for.
func receiptInvoiced(ctx context.Context, q dbQuery, receipt string) error {
	var invoiced bool
	if err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounting_invoices WHERE details->>'transfer_id'=$1 AND status<>'VOID')", receipt).Scan(&invoiced); err != nil {
		return err
	}
	if invoiced {
		return ErrState.WithMetadata(map[string]string{"field": "receipt_invoiced"})
	}
	return nil
}
