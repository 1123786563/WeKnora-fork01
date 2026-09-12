CREATE TABLE commercial_refunds (
    id TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    order_id TEXT NOT NULL,
    order_line_id TEXT NOT NULL,
    amount_fen BIGINT NOT NULL,
    credits_micro BIGINT NOT NULL,
    reviewer TEXT NOT NULL DEFAULT '',
    provider_refund_id TEXT UNIQUE,
    state TEXT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    channel_attempts BIGINT NOT NULL DEFAULT 0,
    review_basis TEXT NOT NULL DEFAULT '',
    review_note TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_commercial_refunds_order ON commercial_refunds (order_id);

CREATE TABLE commercial_refund_allocations (
    id TEXT PRIMARY KEY,
    refund_id TEXT NOT NULL,
    tenant_id BIGINT NOT NULL,
    lot_id TEXT NOT NULL,
    locked_micro BIGINT NOT NULL
);

CREATE INDEX idx_commercial_refund_allocations_refund ON commercial_refund_allocations (refund_id);
CREATE INDEX idx_commercial_refund_allocations_lot ON commercial_refund_allocations (lot_id);
