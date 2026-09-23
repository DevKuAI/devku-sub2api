-- BI identities do not grant access to model credentials or employee sessions.
CREATE TABLE IF NOT EXISTS bi_managers (
    id VARCHAR(128) PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(id),
    authorization_version BIGINT NOT NULL DEFAULT 1 CHECK (authorization_version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bi_organizations (
    id VARCHAR(128) PRIMARY KEY,
    desktop_organization_id BIGINT NOT NULL UNIQUE REFERENCES desktop_organizations(id),
    authorization_version BIGINT NOT NULL DEFAULT 1 CHECK (authorization_version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bi_manager_grants (
    manager_id VARCHAR(128) NOT NULL REFERENCES bi_managers(id),
    organization_id VARCHAR(128) NOT NULL REFERENCES bi_organizations(id),
    role VARCHAR(32) NOT NULL CHECK (role IN ('org_admin', 'team_manager', 'viewer')),
    all_teams BOOLEAN NOT NULL DEFAULT FALSE,
    team_ids JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(team_ids) = 'array'),
    capabilities JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(capabilities) = 'array'),
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    revoked_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (manager_id, organization_id)
);

CREATE TABLE IF NOT EXISTS bi_wechat_bindings (
    id VARCHAR(128) PRIMARY KEY,
    appid VARCHAR(128) NOT NULL,
    openid_hash CHAR(64) NOT NULL,
    manager_id VARCHAR(128) NOT NULL REFERENCES bi_managers(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	last_login_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS bi_wechat_bindings_active_identity
    ON bi_wechat_bindings(appid, openid_hash) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS bi_binding_challenges (
    ticket_hash CHAR(64) PRIMARY KEY,
    code_hash CHAR(64) NOT NULL UNIQUE,
    appid VARCHAR(128) NOT NULL,
    openid_hash CHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'consumed', 'revoked')),
    binding_id VARCHAR(128) REFERENCES bi_wechat_bindings(id),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (expires_at > created_at),
    CHECK (status <> 'approved' OR binding_id IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS bi_binding_challenges_identity
    ON bi_binding_challenges(appid, openid_hash);

CREATE TABLE IF NOT EXISTS bi_wechat_codes (
    code_hash CHAR(64) PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bi_sessions (
    id VARCHAR(128) PRIMARY KEY,
    binding_id VARCHAR(128) NOT NULL REFERENCES bi_wechat_bindings(id),
    credential_version CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);
CREATE INDEX IF NOT EXISTS bi_sessions_binding ON bi_sessions(binding_id);

CREATE TABLE IF NOT EXISTS bi_refresh_tokens (
    token_hash CHAR(64) PRIMARY KEY,
    session_id VARCHAR(128) NOT NULL REFERENCES bi_sessions(id),
    consumed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS bi_refresh_tokens_session ON bi_refresh_tokens(session_id);

CREATE TABLE IF NOT EXISTS bi_security_events (
    id BIGSERIAL PRIMARY KEY,
    manager_id VARCHAR(128),
    action VARCHAR(64) NOT NULL,
    target_id VARCHAR(128) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS bi_security_events_created ON bi_security_events(created_at);

CREATE TABLE IF NOT EXISTS bi_list_snapshots (
    id VARCHAR(128) PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    operation VARCHAR(64) NOT NULL,
    items JSONB NOT NULL CHECK (jsonb_typeof(items) = 'array'),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS bi_list_snapshots_expiry ON bi_list_snapshots(expires_at);
