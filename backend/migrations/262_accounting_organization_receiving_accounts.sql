CREATE TABLE IF NOT EXISTS accounting_organization_receiving_accounts (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id) ON DELETE RESTRICT,
    currency VARCHAR(3) NOT NULL,
    account_name VARCHAR(200) NOT NULL,
    bank_name VARCHAR(200) NOT NULL,
    bank_account VARCHAR(100) NOT NULL,
    bank_branch VARCHAR(200) NOT NULL DEFAULT '',
    remark VARCHAR(1000) NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT accounting_receiving_accounts_currency_check
        CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_receiving_accounts_org_currency
    ON accounting_organization_receiving_accounts(organization_id, currency);
