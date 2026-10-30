-- Channel accounts hold our own money at payment and card platforms; they belong to no organization.
CREATE TABLE IF NOT EXISTS accounting_channel_accounts (
    id UUID PRIMARY KEY,
    platform VARCHAR(32) NOT NULL,
    account_ref VARCHAR(64) NOT NULL,
    name VARCHAR(200) NOT NULL DEFAULT '',
    remark TEXT NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (platform, account_ref)
);

-- Cards seen in channel lines; binding to an organization's card comes with card spend booking.
CREATE TABLE IF NOT EXISTS accounting_channel_cards (
    id UUID PRIMARY KEY,
    channel_account_id UUID NOT NULL REFERENCES accounting_channel_accounts(id),
    card_ref VARCHAR(128) NOT NULL,
    card_last4 VARCHAR(4) NOT NULL DEFAULT '',
    card_name VARCHAR(128) NOT NULL DEFAULT '',
    organization_id BIGINT REFERENCES desktop_organizations(id),
    vcc_card_id UUID REFERENCES accounting_vcc_cards(id),
    bind_method VARCHAR(16) NOT NULL DEFAULT '' CHECK(bind_method IN ('','AUTO_CARD','AUTO_NAME','MANUAL')),
    bound_at TIMESTAMPTZ,
    bound_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (channel_account_id, card_ref)
);

-- One upload. Original files live in private storage; reported_balances keeps the closing balances the
-- platform printed so the books can be compared with them. account_ids lists every account the upload
-- covered, including accounts where every line was already booked.
CREATE TABLE IF NOT EXISTS accounting_channel_imports (
    id UUID PRIMARY KEY,
    converter VARCHAR(32) NOT NULL,
    files JSONB NOT NULL DEFAULT '[]',
    reported_balances JSONB NOT NULL DEFAULT '[]',
    account_ids JSONB NOT NULL DEFAULT '[]',
    total_rows INT NOT NULL DEFAULT 0,
    new_rows INT NOT NULL DEFAULT 0,
    duplicate_rows INT NOT NULL DEFAULT 0,
    pending_rows INT NOT NULL DEFAULT 0,
    status VARCHAR(12) NOT NULL DEFAULT 'APPLIED' CHECK(status IN ('APPLIED','REVOKED')),
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_by BIGINT,
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_accounting_channel_imports_created ON accounting_channel_imports(created_at DESC, id);
CREATE INDEX IF NOT EXISTS idx_accounting_channel_imports_accounts ON accounting_channel_imports USING GIN (account_ids);

-- One money movement in one balance (account + wallet + currency). seq breaks ties between lines that share
-- a timestamp, in the order the platform listed them.
CREATE TABLE IF NOT EXISTS accounting_channel_lines (
    id UUID PRIMARY KEY,
    seq BIGSERIAL NOT NULL,
    channel_account_id UUID NOT NULL REFERENCES accounting_channel_accounts(id),
    import_id UUID NOT NULL REFERENCES accounting_channel_imports(id),
    wallet VARCHAR(32) NOT NULL,
    external_id VARCHAR(128) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    type VARCHAR(16) NOT NULL CHECK(type IN ('FUNDING_IN','FUNDING_OUT','TRANSFER_IN','TRANSFER_OUT','FX_IN','FX_OUT','CONSUME','REFUND','FEE','ADJUSTMENT')),
    amount NUMERIC(24,6) NOT NULL CHECK(amount <> 0),
    currency VARCHAR(3) NOT NULL,
    balance_after NUMERIC(24,6),
    fee_amount NUMERIC(24,6) CHECK(fee_amount IS NULL OR fee_amount >= 0),
    card_ref VARCHAR(128) NOT NULL DEFAULT '',
    card_last4 VARCHAR(4) NOT NULL DEFAULT '',
    card_name VARCHAR(128) NOT NULL DEFAULT '',
    merchant VARCHAR(255) NOT NULL DEFAULT '',
    related_id VARCHAR(128) NOT NULL DEFAULT '',
    original_amount NUMERIC(24,6),
    original_currency VARCHAR(3) NOT NULL DEFAULT '',
    remark TEXT NOT NULL DEFAULT '',
    source_row JSONB NOT NULL DEFAULT '{}',
    pair_line_id UUID REFERENCES accounting_channel_lines(id),
    channel_card_id UUID REFERENCES accounting_channel_cards(id),
    transaction_id UUID REFERENCES accounting_vcc_transactions(id),
    booking VARCHAR(16) NOT NULL DEFAULT 'INTERNAL' CHECK(booking IN ('INTERNAL','BOOKED','LINKED','PENDING_CARD','PENDING_ORDER')),
    status VARCHAR(12) NOT NULL DEFAULT 'ACTIVE' CHECK(status IN ('ACTIVE','REVOKED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_channel_line_external ON accounting_channel_lines(channel_account_id, wallet, currency, external_id) WHERE status = 'ACTIVE';
CREATE INDEX IF NOT EXISTS idx_accounting_channel_lines_balance ON accounting_channel_lines(channel_account_id, wallet, currency, occurred_at, seq) WHERE status = 'ACTIVE';
CREATE INDEX IF NOT EXISTS idx_accounting_channel_lines_import ON accounting_channel_lines(import_id);
CREATE INDEX IF NOT EXISTS idx_accounting_channel_lines_card ON accounting_channel_lines(channel_card_id) WHERE channel_card_id IS NOT NULL;

-- Proofs such as transfer screenshots. They follow the platform reference rather than one imported line, so a
-- revoked and re-imported line keeps its proofs.
CREATE TABLE IF NOT EXISTS accounting_channel_attachments (
    id UUID PRIMARY KEY,
    channel_account_id UUID NOT NULL REFERENCES accounting_channel_accounts(id),
    wallet VARCHAR(32) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    external_id VARCHAR(128) NOT NULL,
    filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK(size_bytes > 0 AND size_bytes <= 20971520),
    sha256 VARCHAR(64) NOT NULL,
    object_key TEXT NOT NULL,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_by BIGINT,
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_accounting_channel_attachments_line ON accounting_channel_attachments(channel_account_id, wallet, currency, external_id) WHERE deleted_at IS NULL;
