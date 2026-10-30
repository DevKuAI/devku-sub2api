-- An order may name the organization's sales contract it is done under; one contract can cover many orders.
ALTER TABLE accounting_orders ADD COLUMN IF NOT EXISTS contract_id UUID REFERENCES accounting_contracts(id);
CREATE INDEX IF NOT EXISTS idx_accounting_orders_contract ON accounting_orders(contract_id) WHERE contract_id IS NOT NULL;
