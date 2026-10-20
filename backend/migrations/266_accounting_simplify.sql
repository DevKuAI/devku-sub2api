-- Order entries can be removed and entered again, so voided records no longer hold their reference numbers.
DROP INDEX IF EXISTS idx_accounting_bank_reference;
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_bank_reference ON accounting_transfers ((details->>'bank_reference'))
    WHERE COALESCE(details->>'bank_reference','') <> '' AND status <> 'VOID';

DROP INDEX IF EXISTS idx_accounting_card_reference;
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_card_reference ON accounting_vcc_transactions(parent_id, (details->>'provider_transaction_id'))
    WHERE COALESCE(details->>'provider_transaction_id','') <> '' AND status <> 'VOID';

-- Fully digital invoices have no invoice code; the number alone identifies them.
DROP INDEX IF EXISTS idx_accounting_invoice_number;
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_invoice_number ON accounting_invoices ((COALESCE(details->>'invoice_code','')), (details->>'invoice_number'))
    WHERE status <> 'VOID';
