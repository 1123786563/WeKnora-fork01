CREATE TABLE commercial_budget_accounts (
    tenant_id INTEGER PRIMARY KEY,
    verified_micro INTEGER NOT NULL DEFAULT 0,
    unreflected_micro INTEGER NOT NULL DEFAULT 0,
    held_micro INTEGER NOT NULL DEFAULT 0,
    refund_locked_micro INTEGER NOT NULL DEFAULT 0,
    watermark TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    verified_until DATETIME NOT NULL
);

CREATE TABLE commercial_task_budgets (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    root_run_id TEXT NOT NULL DEFAULT '',
    limit_micro INTEGER NOT NULL DEFAULT 0,
    spent_micro INTEGER NOT NULL DEFAULT 0,
    held_micro INTEGER NOT NULL DEFAULT 0,
    deadline DATETIME NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_commercial_task_budget_run UNIQUE (tenant_id, run_id)
);

CREATE TABLE commercial_reservations (
    tenant_id INTEGER NOT NULL,
    key TEXT NOT NULL,
    run_id TEXT NOT NULL,
    upper_micro INTEGER NOT NULL,
    state TEXT NOT NULL,
    deadline DATETIME NOT NULL,
    fence TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_commercial_reservation_key UNIQUE (tenant_id, key)
);

CREATE INDEX idx_commercial_reservations_run ON commercial_reservations (tenant_id, run_id);

CREATE TABLE commercial_budget_lots (
    tenant_id INTEGER NOT NULL,
    lot_id TEXT NOT NULL,
    remaining_micro INTEGER NOT NULL DEFAULT 0,
    held_micro INTEGER NOT NULL DEFAULT 0,
    expires_at DATETIME,
    issued_at DATETIME NOT NULL,
    PRIMARY KEY (tenant_id, lot_id)
);

CREATE TABLE commercial_budget_lot_allocations (
    tenant_id INTEGER NOT NULL,
    lot_id TEXT NOT NULL,
    reservation_key TEXT NOT NULL,
    micro INTEGER NOT NULL,
    PRIMARY KEY (tenant_id, lot_id, reservation_key)
);
