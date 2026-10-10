ALTER TABLE desktop_organizations
    ADD COLUMN conversation_summary_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN analysis_model VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN summary_enabled_at TIMESTAMPTZ;
ALTER TABLE desktop_organizations ADD CONSTRAINT desktop_summary_configuration
    CHECK (NOT conversation_summary_enabled OR (conversation_reporting_enabled AND btrim(analysis_model) <> ''));
ALTER TABLE api_keys ADD COLUMN desktop_analysis_organization_id BIGINT REFERENCES desktop_organizations(id);
CREATE UNIQUE INDEX idx_desktop_analysis_key_active ON api_keys(desktop_analysis_organization_id)
    WHERE desktop_analysis_organization_id IS NOT NULL AND deleted_at IS NULL;

-- Source dates use Asia/Shanghai stopped_at boundaries, independently of server timezone.
CREATE INDEX idx_desktop_conversation_stopped ON desktop_conversation_records(organization_id, stopped_at, member_id, id);
CREATE TABLE desktop_daily_report_tasks (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    report_date DATE NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    reason VARCHAR(64) NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}',
    revision INTEGER NOT NULL DEFAULT 1,
    lease_token VARCHAR(40),
    lease_until TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, report_date),
    CHECK (status IN ('pending','waiting','running','completed','partial','failed','skipped'))
);
CREATE INDEX idx_desktop_daily_report_due ON desktop_daily_report_tasks(next_run_at, id)
    WHERE status IN ('pending','waiting','running');
