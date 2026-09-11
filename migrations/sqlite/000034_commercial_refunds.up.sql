CREATE TABLE commercial_refunds (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    order_id TEXT NOT NULL,
    order_line_id TEXT NOT NULL,
    amount_fen INTEGER NOT NULL,
    credits_micro INTEGER NOT NULL,
    reviewer TEXT NOT NULL DEFAULT '',
    provider_refund_id TEXT UNIQUE,
    state TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    channel_attempts INTEGER NOT NULL DEFAULT 0,
    review_basis TEXT NOT NULL DEFAULT '',
    review_note TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_commercial_refunds_order ON commercial_refunds (order_id);

CREATE TABLE commercial_refund_allocations (
    id TEXT PRIMARY KEY,
    refund_id TEXT NOT NULL,
    tenant_id INTEGER NOT NULL,
    lot_id TEXT NOT NULL,
    locked_micro INTEGER NOT NULL
);

CREATE INDEX idx_commercial_refund_allocations_refund ON commercial_refund_allocations (refund_id);
CREATE INDEX idx_commercial_refund_allocations_lot ON commercial_refund_allocations (lot_id);
