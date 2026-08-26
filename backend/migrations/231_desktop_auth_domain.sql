CREATE TABLE IF NOT EXISTS desktop_organizations (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(40) NOT NULL UNIQUE,
    code VARCHAR(16) NOT NULL,
    name VARCHAR(200) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    auth_version BIGINT NOT NULL DEFAULT 1,
    gateway_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    target_config JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT desktop_organizations_status_check CHECK (status IN ('active', 'disabled')),
    CONSTRAINT desktop_organizations_auth_version_check CHECK (auth_version > 0),
    CONSTRAINT desktop_organizations_code_check CHECK (code ~ '^[a-z0-9]{2,16}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_desktop_organizations_code_active
    ON desktop_organizations(code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_desktop_organizations_gateway_user_active
    ON desktop_organizations(gateway_user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_desktop_organizations_group_active
    ON desktop_organizations(group_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_desktop_organizations_status_updated_active
    ON desktop_organizations(status, updated_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS desktop_members (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(40) NOT NULL UNIQUE,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id) ON DELETE RESTRICT,
    name VARCHAR(100) NOT NULL,
    name_normalized VARCHAR(100) NOT NULL,
    phone VARCHAR(16) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    auth_version BIGINT NOT NULL DEFAULT 1,
    api_key_suspended_by_organization BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT desktop_members_status_check CHECK (status IN ('active', 'disabled')),
    CONSTRAINT desktop_members_auth_version_check CHECK (auth_version > 0),
    CONSTRAINT desktop_members_phone_check CHECK (phone ~ '^[+][1-9][0-9]{7,14}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_desktop_members_org_phone_active
    ON desktop_members(organization_id, phone) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_desktop_members_org_status_active
    ON desktop_members(organization_id, status) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS desktop_member_api_keys (
    id BIGSERIAL PRIMARY KEY,
    member_id BIGINT NOT NULL REFERENCES desktop_members(id) ON DELETE RESTRICT,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retired_at TIMESTAMPTZ,
    CONSTRAINT desktop_member_api_keys_api_key_unique UNIQUE (api_key_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_desktop_member_api_keys_current
    ON desktop_member_api_keys(member_id) WHERE retired_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_desktop_member_api_keys_history
    ON desktop_member_api_keys(member_id, assigned_at);

CREATE OR REPLACE FUNCTION bump_desktop_auth_version_for_gateway_user()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.status IS DISTINCT FROM NEW.status OR OLD.deleted_at IS DISTINCT FROM NEW.deleted_at THEN
        UPDATE desktop_organizations
        SET auth_version = auth_version + 1, updated_at = NOW()
        WHERE gateway_user_id = NEW.id AND deleted_at IS NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_users_bump_desktop_auth_version ON users;
CREATE TRIGGER trg_users_bump_desktop_auth_version
AFTER UPDATE OF status, deleted_at ON users
FOR EACH ROW EXECUTE FUNCTION bump_desktop_auth_version_for_gateway_user();
