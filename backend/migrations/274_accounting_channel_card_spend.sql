-- Card spend enters the organization books only from imported channel statements. Lines that cannot be booked on
-- their own (a refund above what is left of its purchase, a closed month, a card in another currency) wait for
-- review with the reason kept beside them.
ALTER TABLE accounting_channel_lines DROP CONSTRAINT IF EXISTS accounting_channel_lines_booking_check;
ALTER TABLE accounting_channel_lines ADD CONSTRAINT accounting_channel_lines_booking_check
    CHECK (booking IN ('INTERNAL','BOOKED','LINKED','PENDING_CARD','PENDING_ORDER','PENDING_REVIEW'));
ALTER TABLE accounting_channel_lines ADD COLUMN IF NOT EXISTS booking_note VARCHAR(64) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_accounting_channel_lines_transaction ON accounting_channel_lines(transaction_id) WHERE transaction_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_accounting_channel_lines_booking ON accounting_channel_lines(booking) WHERE status = 'ACTIVE' AND booking LIKE 'PENDING_%';
CREATE INDEX IF NOT EXISTS idx_accounting_channel_cards_vcc ON accounting_channel_cards(vcc_card_id) WHERE vcc_card_id IS NOT NULL;
