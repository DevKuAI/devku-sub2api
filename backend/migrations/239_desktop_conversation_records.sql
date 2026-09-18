CREATE TABLE IF NOT EXISTS desktop_conversation_records (
    id BIGSERIAL PRIMARY KEY,
    record_id UUID NOT NULL,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id) ON DELETE RESTRICT,
    member_id BIGINT NOT NULL REFERENCES desktop_members(id) ON DELETE RESTRICT,
    installation_id UUID NOT NULL,
    client VARCHAR(32) NOT NULL CHECK (client IN ('workbuddy', 'chatgpt_codex')),
    source_session_id VARCHAR(512) NOT NULL CHECK (char_length(source_session_id) > 0),
    source_turn_id VARCHAR(512),
    started_at TIMESTAMPTZ NOT NULL,
    stopped_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cwd TEXT,
    prompts JSONB NOT NULL,
    response JSONB,
    capture_status VARCHAR(32) NOT NULL,
    prompt_count INTEGER NOT NULL CHECK (prompt_count BETWEEN 1 AND 64),
    CHECK (started_at <= stopped_at),
    CHECK (jsonb_typeof(prompts) = 'array' AND jsonb_array_length(prompts) = prompt_count),
    CHECK ((capture_status = 'captured' AND response IS NOT NULL AND jsonb_typeof(response) = 'object')
        OR (capture_status = 'response_missing' AND response IS NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_desktop_conversation_record_unique
    ON desktop_conversation_records (organization_id, record_id);
CREATE INDEX IF NOT EXISTS idx_desktop_conversation_received
    ON desktop_conversation_records (organization_id, received_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_desktop_conversation_thread
    ON desktop_conversation_records (organization_id, member_id, client, installation_id, source_session_id, received_at, id);
