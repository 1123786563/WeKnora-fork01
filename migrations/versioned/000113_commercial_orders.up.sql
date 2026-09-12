CREATE TABLE commercial_orders (
    id TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    quote_id TEXT NOT NULL UNIQUE,
    amount_fen BIGINT NOT NULL,
    currency TEXT NOT NULL,
    state TEXT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1
);

CREATE TABLE commercial_payment_attempts (
    id TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    order_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    merchant TEXT NOT NULL,
    merchant_order_id TEXT NOT NULL,
    provider_transaction_id TEXT,
    amount_fen BIGINT NOT NULL,
    currency TEXT NOT NULL,
    state TEXT NOT NULL,
    CONSTRAINT uq_attempt_merchant_order UNIQUE (provider, merchant, merchant_order_id),
    CONSTRAINT uq_attempt_provider_txn UNIQUE (provider, merchant, provider_transaction_id)
);

CREATE TABLE commercial_outbox_events (
    event_key TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    kind TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    lease_token TEXT NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempt_count BIGINT NOT NULL DEFAULT 0
);
