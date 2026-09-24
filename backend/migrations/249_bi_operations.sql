-- BI operations metadata and retention policy.
CREATE TABLE IF NOT EXISTS bi_retention_policies (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    report_months INTEGER NOT NULL DEFAULT 24 CHECK (report_months BETWEEN 1 AND 120),
    fact_months INTEGER NOT NULL DEFAULT 24 CHECK (fact_months BETWEEN 1 AND 120),
    audit_days INTEGER NOT NULL DEFAULT 180 CHECK (audit_days BETWEEN 1 AND 3650),
    ephemeral_days INTEGER NOT NULL DEFAULT 7 CHECK (ephemeral_days BETWEEN 1 AND 365),
    updated_by BIGINT REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO bi_retention_policies(id) VALUES (TRUE) ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS bi_cleanup_runs (
    id BIGSERIAL PRIMARY KEY,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    status VARCHAR(16) NOT NULL CHECK (status IN ('running','succeeded','failed')),
    deleted_count BIGINT NOT NULL DEFAULT 0,
    error_message TEXT,
    actor_user_id BIGINT REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS bi_cleanup_runs_finished ON bi_cleanup_runs(finished_at DESC, id DESC);

ALTER TABLE bi_sessions ADD COLUMN IF NOT EXISTS device_id_hash CHAR(64);
ALTER TABLE bi_sessions ADD COLUMN IF NOT EXISTS device_platform VARCHAR(32);
ALTER TABLE bi_sessions ADD COLUMN IF NOT EXISTS client_version VARCHAR(32);
ALTER TABLE bi_sessions ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

ALTER TABLE bi_import_batches ADD COLUMN IF NOT EXISTS retry_of VARCHAR(128) REFERENCES bi_import_batches(id);
CREATE INDEX IF NOT EXISTS bi_import_batches_retry_of ON bi_import_batches(retry_of);

ALTER TABLE bi_reports DROP CONSTRAINT IF EXISTS bi_reports_status_check;
ALTER TABLE bi_reports DROP CONSTRAINT IF EXISTS bi_reports_check;
ALTER TABLE bi_reports DROP CONSTRAINT IF EXISTS bi_reports_check1;
ALTER TABLE bi_reports ADD CONSTRAINT bi_reports_status_check CHECK (status IN ('queued','running','ready','failed','archived'));
ALTER TABLE bi_reports ADD CONSTRAINT bi_reports_ready_payload_check CHECK ((status='ready')=(text IS NOT NULL AND generated_at IS NOT NULL));
ALTER TABLE bi_reports ADD CONSTRAINT bi_reports_failed_code_check CHECK (status <> 'failed' OR failure_code IS NOT NULL);
ALTER TABLE bi_reports ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
ALTER TABLE bi_reports ADD COLUMN IF NOT EXISTS archived_by BIGINT REFERENCES users(id);
ALTER TABLE bi_reports ADD COLUMN IF NOT EXISTS retry_of VARCHAR(128) REFERENCES bi_reports(id);
CREATE INDEX IF NOT EXISTS bi_reports_retry_of ON bi_reports(retry_of);
CREATE INDEX IF NOT EXISTS bi_reports_archived ON bi_reports(organization_id, archived_at) WHERE status='archived';

ALTER TABLE bi_security_events ADD COLUMN IF NOT EXISTS actor_user_id BIGINT REFERENCES users(id);
ALTER TABLE bi_security_events ADD COLUMN IF NOT EXISTS organization_id VARCHAR(128);
ALTER TABLE bi_security_events ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';
CREATE INDEX IF NOT EXISTS bi_security_events_request ON bi_security_events(request_id);
CREATE INDEX IF NOT EXISTS bi_security_events_action_created ON bi_security_events(action, created_at DESC);
CREATE INDEX IF NOT EXISTS bi_security_events_organization_created ON bi_security_events(organization_id, created_at DESC);
