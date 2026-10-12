ALTER TABLE desktop_organizations ADD COLUMN management_policy JSONB NOT NULL DEFAULT '{}';

ALTER TABLE desktop_daily_report_tasks ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}', ADD COLUMN summary JSONB NOT NULL DEFAULT '{}';
UPDATE desktop_daily_report_tasks SET metadata=jsonb_build_object('model',payload->'model','group_id',payload->'group_id','user_id',payload->'user_id'), summary=COALESCE(payload->'summary','{}');

-- Separate each member checkpoint from the organization-wide task payload.
CREATE TABLE desktop_daily_report_members (
    task_id BIGINT NOT NULL REFERENCES desktop_daily_report_tasks(id) ON DELETE CASCADE,
    member_id VARCHAR(40) NOT NULL,
    member JSONB NOT NULL,
    metadata JSONB NOT NULL,
    sources JSONB NOT NULL DEFAULT '[]',
    work JSONB NOT NULL DEFAULT '{}',
    PRIMARY KEY(task_id, member_id)
);
INSERT INTO desktop_daily_report_members(task_id, member_id, member, metadata, sources, work)
SELECT t.id, s->'member'->>'member_id', s->'member', (s->'member') - ARRAY['content','source_ids','executions']::text[], COALESCE(s->'sources','[]'), COALESCE(s->'work','{}')
FROM desktop_daily_report_tasks t, jsonb_array_elements(COALESCE(NULLIF(t.payload->'snapshots','null'::jsonb),'[]')) s;
UPDATE desktop_daily_report_tasks SET payload = payload - 'snapshots';

CREATE INDEX idx_desktop_report_execution_member ON desktop_daily_report_executions(organization_id, report_date, member_id, id DESC);

-- Safe execution metadata can outlive report content.
ALTER TABLE desktop_daily_report_executions ALTER COLUMN task_id DROP NOT NULL;
ALTER TABLE desktop_daily_report_executions ADD COLUMN total_tokens BIGINT, ADD COLUMN actual_cost NUMERIC(20,10);
ALTER TABLE desktop_daily_report_executions DROP CONSTRAINT desktop_daily_report_executions_task_id_fkey;
ALTER TABLE desktop_daily_report_executions ADD CONSTRAINT desktop_daily_report_executions_task_id_fkey
    FOREIGN KEY(task_id) REFERENCES desktop_daily_report_tasks(id) ON DELETE SET NULL;
