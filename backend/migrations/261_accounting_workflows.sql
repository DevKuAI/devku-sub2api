CREATE TABLE accounting_receivables (LIKE accounting_contracts INCLUDING DEFAULTS INCLUDING CONSTRAINTS);
ALTER TABLE accounting_receivables ADD PRIMARY KEY (id);
ALTER TABLE accounting_receivables ADD FOREIGN KEY (organization_id) REFERENCES desktop_organizations(id);
CREATE INDEX accounting_receivables_org_date ON accounting_receivables(organization_id, occurred_at, id);
CREATE TABLE accounting_fx_exchanges (LIKE accounting_expenses INCLUDING DEFAULTS INCLUDING CONSTRAINTS);
ALTER TABLE accounting_fx_exchanges ADD PRIMARY KEY (id);
ALTER TABLE accounting_fx_exchanges ADD FOREIGN KEY (organization_id) REFERENCES desktop_organizations(id);
ALTER TABLE accounting_fx_exchanges ADD FOREIGN KEY (parent_id) REFERENCES accounting_transfers(id);
CREATE INDEX accounting_fx_org_date ON accounting_fx_exchanges(organization_id, occurred_at, id);
CREATE TABLE accounting_receipt_settlements (
 id UUID PRIMARY KEY, organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
 receivable_id UUID NOT NULL REFERENCES accounting_receivables(id), transfer_id UUID NOT NULL REFERENCES accounting_transfers(id),
 amount NUMERIC(24,6) NOT NULL CHECK(amount>0), currency VARCHAR(3) NOT NULL,
 status VARCHAR(12) NOT NULL CHECK(status IN ('ACTIVE','VOID')), reason TEXT NOT NULL DEFAULT '',
 occurred_at TIMESTAMPTZ NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), created_by BIGINT NOT NULL
);
CREATE INDEX accounting_settlements_receivable ON accounting_receipt_settlements(receivable_id,status);
CREATE INDEX accounting_settlements_transfer ON accounting_receipt_settlements(transfer_id,status);
CREATE TABLE accounting_cost_allocations (
 id UUID PRIMARY KEY, organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
 invoice_id UUID NOT NULL REFERENCES accounting_vendor_invoices(id), source_kind VARCHAR(32) NOT NULL CHECK(source_kind IN ('expenses','vcc-transactions')),
 source_id UUID NOT NULL, cost_identity TEXT NOT NULL, amount NUMERIC(24,6) NOT NULL CHECK(amount>0),
 currency VARCHAR(3) NOT NULL, source_amount NUMERIC(24,6) NOT NULL CHECK(source_amount>0), source_currency VARCHAR(3) NOT NULL, exchange_rate NUMERIC(30,12) NOT NULL CHECK(exchange_rate>0), status VARCHAR(12) NOT NULL CHECK(status IN ('ACTIVE','VOID')),
 reason TEXT NOT NULL DEFAULT '', occurred_at TIMESTAMPTZ NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), created_by BIGINT NOT NULL
);
CREATE INDEX accounting_cost_source ON accounting_cost_allocations(cost_identity,status);
CREATE INDEX accounting_cost_invoice ON accounting_cost_allocations(invoice_id,status);
CREATE TABLE accounting_external_actions (
 id UUID PRIMARY KEY, organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
 object_kind VARCHAR(32) NOT NULL, object_id UUID NOT NULL, card_id UUID NOT NULL REFERENCES accounting_vcc_cards(id),
 action VARCHAR(20) NOT NULL CHECK(action IN ('FUND','CLOSE')), amount NUMERIC(24,6) NOT NULL,
 currency VARCHAR(3) NOT NULL, status VARCHAR(12) NOT NULL DEFAULT 'PENDING' CHECK(status IN ('PENDING','CONFIRMED','VOID')),
 reference TEXT NOT NULL DEFAULT '', attachment_id UUID REFERENCES accounting_attachments(id),
 occurred_at TIMESTAMPTZ NOT NULL, confirmed_at TIMESTAMPTZ, confirmed_by BIGINT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX accounting_external_pending ON accounting_external_actions(organization_id,status,occurred_at);
CREATE TABLE accounting_periods (
 id UUID PRIMARY KEY, organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
 month DATE NOT NULL CHECK(EXTRACT(DAY FROM month)=1), status VARCHAR(12) NOT NULL CHECK(status IN ('OPEN','CLOSED')),
 revision BIGINT NOT NULL DEFAULT 0, reason TEXT NOT NULL DEFAULT '', updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(organization_id,month)
);
CREATE TABLE accounting_statements (
 id UUID PRIMARY KEY, organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id), period_id UUID NOT NULL REFERENCES accounting_periods(id),
 revision BIGINT NOT NULL, snapshot JSONB NOT NULL, public_snapshot JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), created_by BIGINT NOT NULL, UNIQUE(period_id,revision)
);
ALTER TABLE accounting_receipt_settlements ADD COLUMN voided_at TIMESTAMPTZ;
ALTER TABLE accounting_cost_allocations ADD COLUMN voided_at TIMESTAMPTZ;
CREATE TABLE accounting_position_baselines (
 organization_id BIGINT PRIMARY KEY REFERENCES desktop_organizations(id), covered_from TIMESTAMPTZ NOT NULL,
 balances JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE accounting_position_events (
 id UUID PRIMARY KEY, organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
 currency VARCHAR(3) NOT NULL, cash_delta NUMERIC(24,6) NOT NULL, card_delta NUMERIC(24,6) NOT NULL,
 occurred_at TIMESTAMPTZ NOT NULL, object_kind VARCHAR(32) NOT NULL, object_id TEXT NOT NULL, action TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX accounting_positions_asof ON accounting_position_events(organization_id,occurred_at,currency);
ALTER TABLE accounting_external_actions ADD COLUMN reason TEXT NOT NULL DEFAULT '';
CREATE INDEX accounting_funding_transfer ON accounting_allocations(transfer_id);
CREATE INDEX accounting_transaction_parent_status ON accounting_vcc_transactions(parent_id,status);
CREATE INDEX accounting_transaction_original_status ON accounting_vcc_transactions((details->>'original_transaction_id'),status);
CREATE INDEX accounting_payment_parent_status ON accounting_expense_payments(parent_id,status);
CREATE INDEX accounting_adjustment_object ON accounting_adjustments(object_kind,object_id);
CREATE INDEX accounting_fx_parent_status ON accounting_fx_exchanges(parent_id,status);
CREATE INDEX accounting_cost_source_record ON accounting_cost_allocations(source_kind,source_id,status);
CREATE INDEX accounting_external_card ON accounting_external_actions(card_id,created_at);
CREATE UNIQUE INDEX accounting_expense_card_mapping ON accounting_expenses((details->>'transaction_id')) WHERE status<>'VOID' AND COALESCE(details->>'transaction_id','')<>'';
