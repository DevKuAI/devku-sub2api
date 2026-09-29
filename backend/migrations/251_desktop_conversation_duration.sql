ALTER TABLE desktop_conversation_records
    ADD COLUMN IF NOT EXISTS duration_ms BIGINT CHECK (duration_ms >= 0);
