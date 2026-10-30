-- Maintained lists of card providers (who issues our cards) and AI vendors (who we buy from). Records keep the
-- chosen vendor's ID plus its name at the time, so renaming a vendor does not rewrite history. Aliases are
-- lower-case fragments used to classify free text from older records and platform statements.
CREATE TABLE IF NOT EXISTS accounting_vendors (
    id UUID PRIMARY KEY,
    kind VARCHAR(16) NOT NULL CHECK(kind IN ('CARD_PROVIDER','AI_VENDOR')),
    name VARCHAR(100) NOT NULL,
    aliases JSONB NOT NULL DEFAULT '[]',
    status VARCHAR(12) NOT NULL DEFAULT 'ACTIVE' CHECK(status IN ('ACTIVE','DISABLED')),
    sort INT NOT NULL DEFAULT 0,
    remark TEXT NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_vendors_name ON accounting_vendors(kind, lower(name));

INSERT INTO accounting_vendors(id, kind, name, aliases, sort) VALUES
    (gen_random_uuid(), 'CARD_PROVIDER', 'SUNRATE', '["sunrate"]', 10),
    (gen_random_uuid(), 'AI_VENDOR', 'OpenAI', '["openai","open ai","chatgpt"]', 10),
    (gen_random_uuid(), 'AI_VENDOR', 'Anthropic', '["anthropic","claude"]', 20),
    (gen_random_uuid(), 'AI_VENDOR', 'Google', '["google","gemini"]', 30),
    (gen_random_uuid(), 'AI_VENDOR', 'xAI', '["xai","x.ai","grok"]', 40),
    (gen_random_uuid(), 'AI_VENDOR', 'Cursor', '["cursor","anysphere"]', 50),
    (gen_random_uuid(), 'AI_VENDOR', 'GitHub', '["github","copilot"]', 60)
ON CONFLICT DO NOTHING;
