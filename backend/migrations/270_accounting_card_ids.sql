-- Cards are identified by the issuing platform's card ID and the last four digits; full card numbers, CVVs and
-- provider tokens are no longer kept. The accounting engine clears stored secrets and fingerprints at start-up
-- (it alone can decrypt them to keep the expiry), so this migration only moves uniqueness to the card ID.
DROP INDEX IF EXISTS idx_accounting_vcc_card_fingerprint;
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_vcc_card_provider_id ON accounting_vcc_cards ((details->>'provider_card_id'))
    WHERE COALESCE(details->>'provider_card_id','') <> '';
