-- Period reports consume enterprise daily summaries, never employee source content.
ALTER TABLE desktop_organizations ADD COLUMN weekly_summary_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE desktop_organizations ADD COLUMN monthly_summary_enabled BOOLEAN NOT NULL DEFAULT FALSE;
CREATE TABLE desktop_period_report_tasks (
 id BIGSERIAL PRIMARY KEY,
 public_id VARCHAR(64) NOT NULL UNIQUE,
 organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
 kind VARCHAR(8) NOT NULL CHECK(kind IN ('week','month')),
 period_start DATE NOT NULL,
 period_end DATE NOT NULL,
 revision INTEGER NOT NULL,
 status VARCHAR(24) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','waiting','running','completed','partial','failed','skipped')),
 reason VARCHAR(64) NOT NULL DEFAULT '',
 sources JSONB NOT NULL DEFAULT '[]',
 source_versions JSONB NOT NULL DEFAULT '[]',
 coverage JSONB NOT NULL DEFAULT '{}',
 work JSONB NOT NULL DEFAULT '{}',
 configuration JSONB NOT NULL DEFAULT '{}',
 content TEXT,
 generated_at TIMESTAMPTZ,
 lease_token VARCHAR(40),
 lease_until TIMESTAMPTZ,
 next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(organization_id,kind,period_start,revision),
 CHECK(period_end>period_start)
);
CREATE UNIQUE INDEX desktop_period_single_active ON desktop_period_report_tasks(organization_id,kind,period_start)
 WHERE status IN ('pending','waiting','running');
CREATE INDEX desktop_period_due ON desktop_period_report_tasks(next_run_at,id)
 WHERE status IN ('pending','waiting','running');
CREATE TABLE desktop_period_report_executions (
 id BIGSERIAL PRIMARY KEY,
 task_id BIGINT REFERENCES desktop_period_report_tasks(id) ON DELETE SET NULL,
 organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
 kind VARCHAR(8) NOT NULL CHECK(kind IN ('week','month')),
 period_start DATE NOT NULL,
 period_end DATE NOT NULL,
 revision INTEGER NOT NULL,
 model VARCHAR(200) NOT NULL,
 round INTEGER NOT NULL,
 chunk INTEGER NOT NULL,
 attempt INTEGER NOT NULL,
 request_id VARCHAR(128) NOT NULL UNIQUE,
 status VARCHAR(16) NOT NULL DEFAULT 'running' CHECK(status IN ('running','completed','failed','blocked')),
 error VARCHAR(64) NOT NULL DEFAULT '',
 started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 finished_at TIMESTAMPTZ,
 total_tokens BIGINT,
 actual_cost NUMERIC(20,10)
);
CREATE INDEX desktop_period_execution_history ON desktop_period_report_executions(organization_id,period_start,id DESC);
