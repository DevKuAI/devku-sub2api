ALTER TABLE bi_reports DROP CONSTRAINT bi_reports_ready_payload_check;
ALTER TABLE bi_reports ADD CONSTRAINT bi_reports_ready_payload_check CHECK (
    (text IS NULL AND generated_at IS NULL AND status <> 'ready')
    OR (text IS NOT NULL AND generated_at IS NOT NULL AND status IN ('ready', 'archived'))
);
