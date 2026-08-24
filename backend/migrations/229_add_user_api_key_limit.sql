ALTER TABLE users
    ADD COLUMN IF NOT EXISTS api_key_limit INTEGER NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_api_key_limit_non_negative'
          AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_api_key_limit_non_negative
            CHECK (api_key_limit >= 0);
    END IF;
END $$;

COMMENT ON COLUMN users.api_key_limit IS
    'Maximum number of non-deleted API keys owned by the user; 0 means unlimited';
