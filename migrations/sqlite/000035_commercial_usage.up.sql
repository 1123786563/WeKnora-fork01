CREATE TABLE commercial_usage_facts (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL DEFAULT '',
    delegation_id TEXT NOT NULL DEFAULT '',
    call_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    funding TEXT NOT NULL,
    service TEXT NOT NULL,
    price_version TEXT NOT NULL,
    revision INTEGER NOT NULL,
    occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    dimensions_json TEXT NOT NULL,
    status TEXT NOT NULL,
    charge_micro INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT uq_commercial_usage_revision UNIQUE (tenant_id, call_id, attempt_id, revision)
);

CREATE INDEX idx_commercial_usage_attempt ON commercial_usage_facts (tenant_id, call_id, attempt_id);

CREATE TABLE commercial_usage_current (
    tenant_id INTEGER NOT NULL,
    call_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, call_id, attempt_id)
);
