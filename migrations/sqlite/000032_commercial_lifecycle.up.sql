CREATE TABLE commercial_subscriptions (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL UNIQUE,
    plan_key TEXT NOT NULL,
    plan_version INTEGER NOT NULL,
    plan_snapshot_json TEXT NOT NULL,
    anchor DATETIME NOT NULL,
    paid_until DATETIME NOT NULL,
    future_interval_json TEXT NOT NULL DEFAULT '{}',
    version INTEGER NOT NULL DEFAULT 1,
    projection_plan_json TEXT NOT NULL DEFAULT '',
    downgrade_reason TEXT NOT NULL DEFAULT ''
);

CREATE TABLE commercial_benefit_jobs (
    key TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    subscription_id TEXT NOT NULL,
    month_start DATETIME NOT NULL,
    credits INTEGER NOT NULL,
    state TEXT NOT NULL,
    external_ref TEXT NOT NULL DEFAULT '',
    order_line_id TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
