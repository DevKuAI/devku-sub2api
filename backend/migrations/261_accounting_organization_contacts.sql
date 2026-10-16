CREATE TABLE IF NOT EXISTS accounting_organization_contacts (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id) ON DELETE RESTRICT,
    name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'BUSINESS',
    phone VARCHAR(40) NOT NULL DEFAULT '',
    email VARCHAR(254) NOT NULL DEFAULT '',
    wechat VARCHAR(100) NOT NULL DEFAULT '',
    remark VARCHAR(1000) NOT NULL DEFAULT '',
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT accounting_organization_contacts_role_check
        CHECK (role IN ('BUSINESS', 'FINANCE', 'LEGAL', 'TECHNICAL', 'OTHER'))
);

CREATE INDEX IF NOT EXISTS idx_accounting_contacts_org
    ON accounting_organization_contacts(organization_id, is_primary DESC, name, id)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_contacts_primary_org
    ON accounting_organization_contacts(organization_id)
    WHERE is_primary AND deleted_at IS NULL;
