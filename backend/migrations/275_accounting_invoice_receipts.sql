-- A sales invoice may name the receipt it was issued for; each receipt is invoiced by at most one effective invoice.
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_invoice_transfer ON accounting_invoices ((details->>'transfer_id'))
    WHERE COALESCE(details->>'transfer_id','') <> '' AND status <> 'VOID';
