-- Execution history survives report regeneration and configuration changes.
CREATE TABLE desktop_daily_report_executions (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT NOT NULL REFERENCES desktop_daily_report_tasks(id),
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    report_date DATE NOT NULL,
    revision INTEGER NOT NULL,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('member', 'summary')),
    member_id VARCHAR(40) NOT NULL DEFAULT '',
    model VARCHAR(200) NOT NULL,
    round INTEGER NOT NULL,
    chunk INTEGER NOT NULL,
    attempt INTEGER NOT NULL,
    request_id VARCHAR(128) NOT NULL UNIQUE,
    status VARCHAR(16) NOT NULL DEFAULT 'running' CHECK (status IN ('running','completed','failed','blocked')),
    error VARCHAR(64) NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    CHECK ((kind = 'member' AND member_id <> '') OR (kind = 'summary' AND member_id = ''))
);
CREATE INDEX idx_desktop_report_execution_history ON desktop_daily_report_executions(organization_id, report_date, id);
