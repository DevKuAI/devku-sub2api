ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS bound_user_id BIGINT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'accounts_bound_user_id_fkey'
          AND conrelid = 'accounts'::regclass
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT accounts_bound_user_id_fkey
            FOREIGN KEY (bound_user_id)
            REFERENCES users(id)
            ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_accounts_bound_user_id
    ON accounts(bound_user_id)
    WHERE bound_user_id IS NOT NULL AND deleted_at IS NULL;
