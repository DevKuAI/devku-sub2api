-- Cards no longer hold balances in the organization books: they only attribute spend, and the money on the card
-- provider is tracked by the channel ledger. Receipt-to-card allocations, funding rows and legacy card top-ups stay
-- as history but no longer count anywhere.

-- Provider top-ups queued by allocations will never be confirmed from the organization books.
UPDATE accounting_external_actions
SET status = 'VOID', reason = 'Cards no longer hold balances; card funding is recorded in the channel ledger', confirmed_at = NOW()
WHERE action = 'FUND' AND status = 'PENDING';

-- Statement positions change meaning: cash is receipts less what was exchanged away, card is net card spend.
-- Rebuild every baseline and position event from the records, dated as the running engine dates them.
CREATE TEMP TABLE accounting_position_rebuild ON COMMIT DROP AS
SELECT r.organization_id, r.currency, r.occurred_at,
       r.amount + COALESCE((SELECT SUM(x.amount) FROM accounting_adjustments x WHERE x.object_kind = 'transfers' AND x.object_id = r.id), 0) AS cash,
       0::numeric AS card, 'transfers'::text AS object_kind, r.id::text AS object_id
FROM accounting_transfers r
WHERE r.status = 'RECEIVED' AND COALESCE(r.details->>'funding_type', '') <> 'TOPUP'
UNION ALL
SELECT r.organization_id, r.currency, x.occurred_at,
       -(x.amount + COALESCE(NULLIF(x.details->>'fee_amount', '')::numeric, 0)), 0::numeric, 'fx-exchanges', x.id::text
FROM accounting_fx_exchanges x
JOIN accounting_transfers r ON r.id = x.parent_id
WHERE x.status = 'CONFIRMED' AND r.status = 'RECEIVED' AND COALESCE(r.details->>'funding_type', '') <> 'TOPUP'
UNION ALL
SELECT t.organization_id, t.currency, t.occurred_at, 0::numeric,
       CASE WHEN t.details->>'transaction_type' = 'REFUND' OR (t.details->>'transaction_type' = 'ADJUSTMENT' AND t.details->>'direction' = 'CREDIT')
            THEN -t.amount ELSE t.amount END,
       'vcc-transactions', t.id::text
FROM accounting_vcc_transactions t
WHERE t.status = 'CONFIRMED';

UPDATE accounting_position_baselines b
SET balances = COALESCE((
    SELECT jsonb_object_agg(p.currency, jsonb_build_object('cash', p.cash::text, 'card', p.card::text))
    FROM (
        SELECT r.currency, SUM(r.cash) AS cash, SUM(r.card) AS card
        FROM accounting_position_rebuild r
        WHERE r.organization_id = b.organization_id AND r.occurred_at < b.covered_from
        GROUP BY r.currency
    ) p
), '{}'::jsonb);

DELETE FROM accounting_position_events e
WHERE EXISTS (SELECT 1 FROM accounting_position_baselines b WHERE b.organization_id = e.organization_id);

INSERT INTO accounting_position_events (id, organization_id, currency, cash_delta, card_delta, occurred_at, object_kind, object_id, action)
SELECT gen_random_uuid(), r.organization_id, r.currency, r.cash, r.card, r.occurred_at, r.object_kind, r.object_id, 'rebuild'
FROM accounting_position_rebuild r
JOIN accounting_position_baselines b ON b.organization_id = r.organization_id
WHERE r.occurred_at >= b.covered_from AND (r.cash <> 0 OR r.card <> 0);
