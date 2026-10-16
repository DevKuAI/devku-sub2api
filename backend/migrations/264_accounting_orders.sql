-- Orders group one customer deal: receipt, sales invoice, card purchases and AI vendor invoices.
CREATE TABLE IF NOT EXISTS accounting_orders (
    id UUID PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    name VARCHAR(200) NOT NULL,
    remark TEXT NOT NULL DEFAULT '',
    status VARCHAR(12) NOT NULL DEFAULT 'OPEN' CHECK(status IN ('OPEN','CLOSED')),
    close_reason TEXT NOT NULL DEFAULT '',
    closed_at TIMESTAMPTZ,
    closed_by BIGINT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_accounting_orders_org_created ON accounting_orders(organization_id, created_at DESC, id);

-- A record belongs to at most one order.
CREATE TABLE IF NOT EXISTS accounting_order_items (
    object_kind VARCHAR(32) NOT NULL CHECK(object_kind IN ('transfers','invoices','vcc-cards','vcc-transactions','vendor-invoices')),
    object_id UUID NOT NULL,
    order_id UUID NOT NULL REFERENCES accounting_orders(id),
    organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (object_kind, object_id)
);
CREATE INDEX IF NOT EXISTS idx_accounting_order_items_order ON accounting_order_items(order_id, object_kind);

-- Keyed fingerprint of the full card number so one physical card is registered to one organization only.
ALTER TABLE accounting_vcc_cards ADD COLUMN IF NOT EXISTS card_fingerprint TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_vcc_card_fingerprint ON accounting_vcc_cards(card_fingerprint) WHERE card_fingerprint IS NOT NULL;
