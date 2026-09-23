CREATE TABLE IF NOT EXISTS bi_reports (
    id VARCHAR(128) PRIMARY KEY,
    organization_id VARCHAR(128) NOT NULL REFERENCES bi_organizations(id),
    manager_id VARCHAR(128) NOT NULL REFERENCES bi_managers(id),
    title TEXT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','ready','failed')),
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    failure_code VARCHAR(32) CHECK (failure_code IN ('DATA_UNAVAILABLE','CONTEXT_REVOKED','GENERATION_FAILED')),
    snapshot JSONB NOT NULL,
    scope JSONB NOT NULL,
    source_state JSONB NOT NULL,
    data_revision BIGINT REFERENCES bi_data_revisions(id),
    dependencies JSONB NOT NULL DEFAULT '[]',
    required_capabilities JSONB NOT NULL DEFAULT '[]',
    evidence JSONB NOT NULL DEFAULT '[]',
    text TEXT,
    generated_at TIMESTAMPTZ,
    lease_owner VARCHAR(128),
    lease_until TIMESTAMPTZ,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    CHECK ((status='ready')=(text IS NOT NULL AND generated_at IS NOT NULL)),
    CHECK ((status='failed')=(failure_code IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS bi_reports_list ON bi_reports(organization_id,created_at DESC,id);
CREATE INDEX IF NOT EXISTS bi_reports_queue ON bi_reports(created_at) WHERE status IN ('queued','running');

CREATE TABLE IF NOT EXISTS bi_report_shares (
    id VARCHAR(128) PRIMARY KEY,
    report_id VARCHAR(128) NOT NULL REFERENCES bi_reports(id),
    organization_id VARCHAR(128) NOT NULL REFERENCES bi_organizations(id),
    manager_id VARCHAR(128) NOT NULL REFERENCES bi_managers(id),
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (expires_at>created_at)
);
CREATE INDEX IF NOT EXISTS bi_report_shares_owner ON bi_report_shares(manager_id,organization_id,report_id,created_at DESC);

CREATE TABLE IF NOT EXISTS bi_command_receipts (
    manager_id VARCHAR(128) NOT NULL REFERENCES bi_managers(id),
    organization_id VARCHAR(128) NOT NULL REFERENCES bi_organizations(id),
    operation VARCHAR(64) NOT NULL,
    key_hash CHAR(64) NOT NULL,
    payload_hash CHAR(64) NOT NULL,
    result JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (manager_id,organization_id,operation,key_hash)
);
CREATE INDEX IF NOT EXISTS bi_command_receipts_expiry ON bi_command_receipts(expires_at);
