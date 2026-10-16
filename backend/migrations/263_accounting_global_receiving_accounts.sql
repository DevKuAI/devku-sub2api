DO $$
BEGIN
    IF to_regclass('public.accounting_receiving_accounts') IS NULL
       AND to_regclass('public.accounting_organization_receiving_accounts') IS NOT NULL THEN
        ALTER TABLE accounting_organization_receiving_accounts RENAME TO accounting_receiving_accounts;
    END IF;
END $$;

DROP INDEX IF EXISTS idx_accounting_receiving_accounts_org_currency;

WITH ranked AS (
    SELECT
        id,
        FIRST_VALUE(id) OVER (
            PARTITION BY currency, bank_account
            ORDER BY created_at, id
        ) AS canonical_id
    FROM accounting_receiving_accounts
), duplicates AS (
    SELECT id, canonical_id FROM ranked WHERE id <> canonical_id
)
UPDATE accounting_transfers AS transfer
SET details = jsonb_set(
    transfer.details,
    '{receiving_account_id}',
    to_jsonb(duplicates.canonical_id::text),
    false
)
FROM duplicates
WHERE transfer.details->>'receiving_account_id' = duplicates.id::text;

WITH ranked AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY currency, bank_account
            ORDER BY created_at, id
        ) AS duplicate_rank
    FROM accounting_receiving_accounts
)
DELETE FROM accounting_receiving_accounts AS account
USING ranked
WHERE account.id = ranked.id AND ranked.duplicate_rank > 1;

ALTER TABLE accounting_receiving_accounts
    DROP CONSTRAINT IF EXISTS accounting_organization_receiving_accounts_organization_id_fkey;

ALTER TABLE accounting_receiving_accounts
    DROP COLUMN IF EXISTS organization_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_receiving_accounts_currency_bank_account
    ON accounting_receiving_accounts(currency, bank_account);
