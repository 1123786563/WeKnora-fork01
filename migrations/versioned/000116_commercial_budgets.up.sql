CREATE TABLE commercial_budget_accounts (
    tenant_id BIGINT PRIMARY KEY,
    verified_micro BIGINT NOT NULL DEFAULT 0,
    unreflected_micro BIGINT NOT NULL DEFAULT 0,
    held_micro BIGINT NOT NULL DEFAULT 0,
    refund_locked_micro BIGINT NOT NULL DEFAULT 0,
    watermark TEXT NOT NULL DEFAULT '',
    version BIGINT NOT NULL DEFAULT 1,
    verified_until TIMESTAMPTZ NOT NULL
);

CREATE TABLE commercial_task_budgets (
    tenant_id BIGINT NOT NULL,
    run_id TEXT NOT NULL,
    root_run_id TEXT NOT NULL DEFAULT '',
    limit_micro BIGINT NOT NULL DEFAULT 0,
    spent_micro BIGINT NOT NULL DEFAULT 0,
    held_micro BIGINT NOT NULL DEFAULT 0,
    deadline TIMESTAMPTZ NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_commercial_task_budget_run UNIQUE (tenant_id, run_id)
);

CREATE TABLE commercial_reservations (
    tenant_id BIGINT NOT NULL,
    key TEXT NOT NULL,
    run_id TEXT NOT NULL,
    upper_micro BIGINT NOT NULL,
    state TEXT NOT NULL,
    deadline TIMESTAMPTZ NOT NULL,
    fence TEXT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_commercial_reservation_key UNIQUE (tenant_id, key)
);

CREATE INDEX idx_commercial_reservations_run ON commercial_reservations (tenant_id, run_id);

CREATE TABLE commercial_budget_lots (
    tenant_id BIGINT NOT NULL,
    lot_id TEXT NOT NULL,
    remaining_micro BIGINT NOT NULL DEFAULT 0,
    held_micro BIGINT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    issued_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, lot_id)
);

CREATE TABLE commercial_budget_lot_allocations (
    tenant_id BIGINT NOT NULL,
    lot_id TEXT NOT NULL,
    reservation_key TEXT NOT NULL,
    micro BIGINT NOT NULL,
    PRIMARY KEY (tenant_id, lot_id, reservation_key)
);
