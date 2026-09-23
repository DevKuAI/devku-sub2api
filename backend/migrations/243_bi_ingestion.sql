-- Only published revisions are visible to analytics and content readers.
CREATE TABLE IF NOT EXISTS bi_connector_sources (
    organization_id VARCHAR(128) NOT NULL REFERENCES bi_organizations(id),
    source_id VARCHAR(128) NOT NULL,
    namespace VARCHAR(128) NOT NULL,
    allowed_kinds JSONB NOT NULL CHECK (jsonb_typeof(allowed_kinds) = 'array'),
    checkpoint TEXT,
    complete_through TIMESTAMPTZ,
    history_start_date DATE,
    initial_backfill_complete BOOLEAN NOT NULL DEFAULT FALSE,
    last_applied_at TIMESTAMPTZ,
    data_revision BIGINT,
    PRIMARY KEY (organization_id, source_id),
    UNIQUE (organization_id, namespace),
    CHECK (NOT initial_backfill_complete OR complete_through IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS bi_connector_credentials (
    id VARCHAR(128) PRIMARY KEY,
    token_hash CHAR(64) NOT NULL UNIQUE,
    audience VARCHAR(32) NOT NULL CHECK (audience = 'bi-connector'),
    organization_id VARCHAR(128) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    allowed_kinds JSONB NOT NULL CHECK (jsonb_typeof(allowed_kinds) = 'array'),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (organization_id, source_id) REFERENCES bi_connector_sources(organization_id, source_id)
);

CREATE TABLE IF NOT EXISTS bi_import_batches (
    id VARCHAR(128) PRIMARY KEY,
    organization_id VARCHAR(128) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    idempotency_hash CHAR(64) NOT NULL,
    payload_hash CHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'validating', 'applied', 'rejected')),
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    applied_at TIMESTAMPTZ,
    checkpoint TEXT,
    data_revision BIGINT,
    errors JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(errors) = 'array'),
    lease_owner VARCHAR(128),
    lease_until TIMESTAMPTZ,
    attempt_count INT NOT NULL DEFAULT 0,
    FOREIGN KEY (organization_id, source_id) REFERENCES bi_connector_sources(organization_id, source_id),
    UNIQUE (organization_id, source_id, idempotency_hash),
    CHECK ((status = 'applied' AND applied_at IS NOT NULL AND checkpoint IS NOT NULL AND data_revision IS NOT NULL)
        OR (status <> 'applied' AND applied_at IS NULL AND checkpoint IS NULL AND data_revision IS NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS bi_import_batches_one_per_source
    ON bi_import_batches(organization_id, source_id) WHERE status IN ('queued', 'validating');
CREATE UNIQUE INDEX IF NOT EXISTS bi_import_batches_publisher_per_organization
    ON bi_import_batches(organization_id) WHERE status = 'validating';
CREATE INDEX IF NOT EXISTS bi_import_batches_queue ON bi_import_batches(received_at) WHERE status IN ('queued', 'validating');
CREATE UNIQUE INDEX IF NOT EXISTS bi_import_batches_applied_checkpoint
    ON bi_import_batches(organization_id, source_id, checkpoint) WHERE status = 'applied';

CREATE TABLE IF NOT EXISTS bi_data_revisions (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(128) NOT NULL UNIQUE,
    organization_id VARCHAR(128) NOT NULL REFERENCES bi_organizations(id),
    batch_id VARCHAR(128) NOT NULL UNIQUE REFERENCES bi_import_batches(id),
    status VARCHAR(16) NOT NULL CHECK (status IN ('staged', 'built', 'published', 'rejected')),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((status = 'published') = (published_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS bi_data_revisions_published ON bi_data_revisions(organization_id, id DESC) WHERE status = 'published';

CREATE TABLE IF NOT EXISTS bi_entities (
    organization_id VARCHAR(128) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    id VARCHAR(128) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    PRIMARY KEY (organization_id, kind, id),
    FOREIGN KEY (organization_id, source_id) REFERENCES bi_connector_sources(organization_id, source_id)
);

CREATE TABLE IF NOT EXISTS bi_entity_versions (
    organization_id VARCHAR(128) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    entity_id VARCHAR(128) NOT NULL,
    revision NUMERIC NOT NULL CHECK (revision > 0 AND revision = trunc(revision)),
    data_revision BIGINT NOT NULL REFERENCES bi_data_revisions(id),
    payload JSONB NOT NULL,
    payload_hash CHAR(64) NOT NULL,
    tombstone BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (organization_id, kind, entity_id, data_revision, revision),
    FOREIGN KEY (organization_id, kind, entity_id) REFERENCES bi_entities(organization_id, kind, id)
);
CREATE INDEX IF NOT EXISTS bi_entity_versions_snapshot ON bi_entity_versions(organization_id, kind, entity_id, data_revision DESC, revision DESC);
CREATE INDEX IF NOT EXISTS bi_entity_versions_member ON bi_entity_versions(organization_id, (payload->>'member_id')) WHERE kind IN ('membership','eligibility');
CREATE INDEX IF NOT EXISTS bi_entity_versions_usage ON bi_entity_versions(organization_id, (payload->>'usage_event_id')) WHERE kind IN ('rating','reference');

CREATE TABLE IF NOT EXISTS bi_record_receipts (
    organization_id VARCHAR(128) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    entity_id VARCHAR(128) NOT NULL,
    revision NUMERIC NOT NULL CHECK (revision > 0 AND revision = trunc(revision)),
    payload_hash CHAR(64) NOT NULL,
    batch_id VARCHAR(128) NOT NULL REFERENCES bi_import_batches(id),
    PRIMARY KEY (organization_id,kind,entity_id,revision,batch_id)
);

CREATE TABLE IF NOT EXISTS bi_ingestion_outbox (
    data_revision BIGINT PRIMARY KEY REFERENCES bi_data_revisions(id),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Restrictions survive a failed view build until a repair is published.
CREATE TABLE IF NOT EXISTS bi_acl_denials (
    organization_id VARCHAR(128) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    entity_id VARCHAR(128) NOT NULL,
    data_revision BIGINT NOT NULL REFERENCES bi_data_revisions(id),
    revision NUMERIC NOT NULL,
    acl JSONB,
    deny_all BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (organization_id, kind, entity_id, data_revision)
);

CREATE TABLE IF NOT EXISTS bi_usage_facts (
    organization_id VARCHAR(128) NOT NULL,
    entity_id VARCHAR(128) NOT NULL,
    data_revision BIGINT NOT NULL REFERENCES bi_data_revisions(id),
    entity_revision NUMERIC NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    actor_type VARCHAR(16) NOT NULL CHECK (actor_type IN ('human','automatic','unknown')),
    member_id VARCHAR(128),
    team_id VARCHAR(128),
    application_id VARCHAR(128),
    application_version_id VARCHAR(128),
    scene_id VARCHAR(128),
    requested_model TEXT NOT NULL,
    outcome VARCHAR(16) NOT NULL CHECK (outcome IN ('succeeded','failed','unknown')),
    duration_ms BIGINT,
	clock_skew BOOLEAN NOT NULL DEFAULT FALSE,
    input_tokens NUMERIC,
    output_tokens NUMERIC,
    cache_read_tokens NUMERIC,
    cache_write_tokens NUMERIC,
    tombstone BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (organization_id, entity_id, data_revision, entity_revision),
    CHECK ((actor_type = 'human') = (member_id IS NOT NULL)),
    CHECK ((input_tokens IS NULL AND output_tokens IS NULL AND cache_read_tokens IS NULL AND cache_write_tokens IS NULL)
        OR (input_tokens IS NOT NULL AND output_tokens IS NOT NULL AND cache_read_tokens IS NOT NULL AND cache_write_tokens IS NOT NULL
            AND input_tokens >= 0 AND output_tokens >= 0 AND cache_read_tokens >= 0 AND cache_write_tokens >= 0
            AND input_tokens >= cache_read_tokens + cache_write_tokens))
);
CREATE INDEX IF NOT EXISTS bi_usage_facts_range ON bi_usage_facts(organization_id,occurred_at,data_revision);
CREATE INDEX IF NOT EXISTS bi_usage_facts_member ON bi_usage_facts(organization_id,member_id,occurred_at);

CREATE TABLE IF NOT EXISTS bi_source_version_links (
    organization_id VARCHAR(128) NOT NULL,
    parent_kind VARCHAR(32) NOT NULL CHECK (parent_kind IN ('knowledge_version','case')),
    parent_id VARCHAR(128) NOT NULL,
    parent_data_revision BIGINT NOT NULL REFERENCES bi_data_revisions(id),
    source_id VARCHAR(128) NOT NULL,
    source_version_id VARCHAR(128) NOT NULL,
    source_data_revision BIGINT NOT NULL REFERENCES bi_data_revisions(id),
    PRIMARY KEY (organization_id,parent_kind,parent_id,parent_data_revision,source_id)
);
