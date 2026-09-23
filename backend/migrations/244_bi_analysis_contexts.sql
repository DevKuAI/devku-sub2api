CREATE TABLE IF NOT EXISTS bi_analysis_contexts (
    id VARCHAR(128) PRIMARY KEY,
    manager_id VARCHAR(128) NOT NULL REFERENCES bi_managers(id),
    organization_id VARCHAR(128) NOT NULL REFERENCES bi_organizations(id),
    data_revision BIGINT REFERENCES bi_data_revisions(id),
    snapshot JSONB NOT NULL,
    scope JSONB NOT NULL,
    source_state JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS bi_analysis_contexts_owner ON bi_analysis_contexts(manager_id,organization_id,expires_at);
