ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS request_body TEXT;
