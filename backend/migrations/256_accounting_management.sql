CREATE TABLE IF NOT EXISTS accounting_transfers (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    parent_id UUID ,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    currency VARCHAR(3) NOT NULL,
    amount NUMERIC(24,6) NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    customer_visible BOOLEAN NOT NULL DEFAULT FALSE,
    shared_at TIMESTAMPTZ,
    shared_by BIGINT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_accounting_transfers_org_date ON accounting_transfers(organization_id, occurred_at DESC, id);

CREATE TABLE IF NOT EXISTS accounting_vcc_cards (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    parent_id UUID ,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    currency VARCHAR(3) NOT NULL,
    amount NUMERIC(24,6) NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    customer_visible BOOLEAN NOT NULL DEFAULT FALSE,
    shared_at TIMESTAMPTZ,
    shared_by BIGINT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_accounting_vcc_cards_org_date ON accounting_vcc_cards(organization_id, occurred_at DESC, id);

CREATE TABLE IF NOT EXISTS accounting_vcc_transactions (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    parent_id UUID REFERENCES accounting_vcc_cards(id),
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    currency VARCHAR(3) NOT NULL,
    amount NUMERIC(24,6) NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    customer_visible BOOLEAN NOT NULL DEFAULT FALSE,
    shared_at TIMESTAMPTZ,
    shared_by BIGINT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_accounting_vcc_transactions_org_date ON accounting_vcc_transactions(organization_id, occurred_at DESC, id);

CREATE TABLE IF NOT EXISTS accounting_contracts (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    parent_id UUID ,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    currency VARCHAR(3) NOT NULL,
    amount NUMERIC(24,6) NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    customer_visible BOOLEAN NOT NULL DEFAULT FALSE,
    shared_at TIMESTAMPTZ,
    shared_by BIGINT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_accounting_contracts_org_date ON accounting_contracts(organization_id, occurred_at DESC, id);

CREATE TABLE IF NOT EXISTS accounting_invoices (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    parent_id UUID ,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    currency VARCHAR(3) NOT NULL,
    amount NUMERIC(24,6) NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    customer_visible BOOLEAN NOT NULL DEFAULT FALSE,
    shared_at TIMESTAMPTZ,
    shared_by BIGINT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_accounting_invoices_org_date ON accounting_invoices(organization_id, occurred_at DESC, id);

CREATE TABLE IF NOT EXISTS accounting_vendor_invoices (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    parent_id UUID ,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    currency VARCHAR(3) NOT NULL,
    amount NUMERIC(24,6) NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    customer_visible BOOLEAN NOT NULL DEFAULT FALSE,
    shared_at TIMESTAMPTZ,
    shared_by BIGINT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_accounting_vendor_invoices_org_date ON accounting_vendor_invoices(organization_id, occurred_at DESC, id);

CREATE TABLE IF NOT EXISTS accounting_expenses (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    parent_id UUID ,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    currency VARCHAR(3) NOT NULL,
    amount NUMERIC(24,6) NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    customer_visible BOOLEAN NOT NULL DEFAULT FALSE,
    shared_at TIMESTAMPTZ,
    shared_by BIGINT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_accounting_expenses_org_date ON accounting_expenses(organization_id, occurred_at DESC, id);

CREATE TABLE IF NOT EXISTS accounting_expense_payments (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    parent_id UUID REFERENCES accounting_expenses(id),
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    currency VARCHAR(3) NOT NULL,
    amount NUMERIC(24,6) NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    customer_visible BOOLEAN NOT NULL DEFAULT FALSE,
    shared_at TIMESTAMPTZ,
    shared_by BIGINT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_accounting_expense_payments_org_date ON accounting_expense_payments(organization_id, occurred_at DESC, id);

ALTER TABLE accounting_transfers ADD COLUMN IF NOT EXISTS sales_amount NUMERIC(24,6) GENERATED ALWAYS AS (COALESCE(NULLIF(details->>'sales_amount','')::numeric,0)) STORED;
ALTER TABLE accounting_transfers ADD COLUMN IF NOT EXISTS tax_amount NUMERIC(24,6) GENERATED ALWAYS AS (COALESCE(NULLIF(details->>'tax_amount','')::numeric,0)) STORED;
ALTER TABLE accounting_transfers ADD COLUMN IF NOT EXISTS net_sales_amount NUMERIC(24,6) GENERATED ALWAYS AS (COALESCE(NULLIF(details->>'net_sales_amount','')::numeric,0)) STORED;
ALTER TABLE accounting_transfers ADD COLUMN IF NOT EXISTS receivable_amount NUMERIC(24,6) GENERATED ALWAYS AS (COALESCE(NULLIF(details->>'receivable_amount','')::numeric,0)) STORED;
ALTER TABLE accounting_invoices ADD COLUMN IF NOT EXISTS net_amount NUMERIC(24,6) GENERATED ALWAYS AS (COALESCE(NULLIF(details->>'net_amount','')::numeric,0)) STORED;
ALTER TABLE accounting_invoices ADD COLUMN IF NOT EXISTS tax_amount NUMERIC(24,6) GENERATED ALWAYS AS (COALESCE(NULLIF(details->>'tax_amount','')::numeric,0)) STORED;
ALTER TABLE accounting_invoices ADD COLUMN IF NOT EXISTS total_amount NUMERIC(24,6) GENERATED ALWAYS AS (COALESCE(NULLIF(details->>'total_amount','')::numeric,0)) STORED;
ALTER TABLE accounting_vcc_cards ADD COLUMN IF NOT EXISTS credit_limit NUMERIC(24,6) GENERATED ALWAYS AS (COALESCE(NULLIF(details->>'credit_limit','')::numeric,0)) STORED;
ALTER TABLE accounting_transfers ADD COLUMN IF NOT EXISTS tax_rate NUMERIC(10,6) GENERATED ALWAYS AS (COALESCE(NULLIF(details->>'tax_rate','')::numeric,0)) STORED;
ALTER TABLE accounting_contracts ADD COLUMN IF NOT EXISTS tax_rate NUMERIC(10,6) GENERATED ALWAYS AS (COALESCE(NULLIF(details->>'tax_rate','')::numeric,0)) STORED;
ALTER TABLE accounting_vcc_cards ADD COLUMN IF NOT EXISTS secret_ciphertext TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_bank_reference ON accounting_transfers ((details->>'bank_reference')) WHERE COALESCE(details->>'bank_reference','') <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_card_reference ON accounting_vcc_transactions(parent_id, (details->>'provider_transaction_id')) WHERE COALESCE(details->>'provider_transaction_id','') <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_invoice_number ON accounting_invoices ((details->>'invoice_code'), (details->>'invoice_number'));
CREATE TABLE IF NOT EXISTS accounting_allocations (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    transfer_id UUID NOT NULL REFERENCES accounting_transfers(id),
    card_id UUID NOT NULL REFERENCES accounting_vcc_cards(id),
    amount NUMERIC(24,6) NOT NULL CHECK(amount > 0),
    used_amount NUMERIC(24,6) NOT NULL DEFAULT 0 CHECK(used_amount >= 0),
    released_amount NUMERIC(24,6) NOT NULL DEFAULT 0 CHECK(released_amount >= 0),
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by BIGINT NOT NULL,
    CHECK(used_amount + released_amount <= amount)
);
CREATE INDEX IF NOT EXISTS idx_accounting_allocations_card ON accounting_allocations(card_id, created_at, id);
CREATE TABLE IF NOT EXISTS accounting_funding (
    id BIGSERIAL PRIMARY KEY,
    transaction_id UUID NOT NULL REFERENCES accounting_vcc_transactions(id),
    allocation_id UUID REFERENCES accounting_allocations(id),
    source_funding_id BIGINT REFERENCES accounting_funding(id),
    amount NUMERIC(24,6) NOT NULL CHECK(amount >= 0),
    refunded_amount NUMERIC(24,6) NOT NULL DEFAULT 0 CHECK(refunded_amount >= 0 AND refunded_amount <= amount),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS accounting_adjustments (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    object_kind VARCHAR(32) NOT NULL,
    object_id UUID NOT NULL,
    amount NUMERIC(24,6) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by BIGINT NOT NULL
);
CREATE TABLE IF NOT EXISTS accounting_reversals (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    object_kind VARCHAR(32) NOT NULL,
    object_id UUID NOT NULL,
    amount NUMERIC(24,6) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by BIGINT NOT NULL
);
CREATE TABLE IF NOT EXISTS accounting_attachments (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    object_kind VARCHAR(32) NOT NULL,
    object_id UUID NOT NULL,
    purpose VARCHAR(40) NOT NULL,
    filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK(size_bytes > 0 AND size_bytes <= 20971520),
    sha256 VARCHAR(64) NOT NULL,
    object_key TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    customer_visible BOOLEAN NOT NULL DEFAULT FALSE,
    shared_at TIMESTAMPTZ,
    shared_by BIGINT,
    expires_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_accounting_attachments_object ON accounting_attachments(organization_id, object_kind, object_id);
CREATE TABLE IF NOT EXISTS accounting_reconciliation (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    object_kind VARCHAR(32) NOT NULL,
    object_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL CHECK(status IN ('MATCHED','DIFFERENCE','EXCLUDED')),
    reference VARCHAR(200) NOT NULL DEFAULT '',
    reference_amount NUMERIC(24,6) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    attachment_id UUID REFERENCES accounting_attachments(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_accounting_reconciliation_object ON accounting_reconciliation(object_kind, object_id, created_at DESC, id);
CREATE TABLE IF NOT EXISTS accounting_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    object_kind VARCHAR(32) NOT NULL,
    object_id TEXT NOT NULL,
    action VARCHAR(80) NOT NULL,
    actor_id BIGINT NOT NULL,
    request_id VARCHAR(200) NOT NULL,
    summary JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_accounting_audit_org ON accounting_audit_logs(organization_id, created_at DESC, id);
CREATE TABLE IF NOT EXISTS accounting_operation_results (
    actor_id BIGINT NOT NULL,
    operation TEXT NOT NULL,
    key_hash VARCHAR(64) NOT NULL,
    fingerprint VARCHAR(64) NOT NULL,
    response JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(actor_id, operation, key_hash)
);
