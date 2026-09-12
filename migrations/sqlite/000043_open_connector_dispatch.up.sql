-- T10: dispatch idempotency records + concurrency lease (sqlite twin of PG
-- 000123, offset +80). Same constraints, same DB-time first_sent_at and the
-- same 23h50m replay window via datetime(CURRENT_TIMESTAMP, '+1430 minutes').
CREATE TABLE connector_dispatch_records (
    tenant_id INTEGER NOT NULL,
    action_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL,
    key TEXT NOT NULL,
    execution_id TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'dispatched' CHECK (state IN ('dispatched','succeeded','failed','unknown')),
    reservation_id TEXT NOT NULL DEFAULT '',
    first_sent_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    replay_until DATETIME NOT NULL,
    fence INTEGER NOT NULL DEFAULT 1 CHECK (fence > 0),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, action_id),
    UNIQUE (runtime_id, key)
);

CREATE INDEX idx_oc_dispatch_state ON connector_dispatch_records (state);

CREATE TABLE connector_dispatch_lease_scopes (
    scope TEXT PRIMARY KEY,
    acquired_total INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE connector_dispatch_leases (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL,
    tenant_id INTEGER NOT NULL,
    owner TEXT NOT NULL,
    action_id TEXT NOT NULL DEFAULT '',
    acquired_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    until DATETIME NOT NULL,
    released_at DATETIME,
    fence INTEGER NOT NULL DEFAULT 1 CHECK (fence > 0)
);

CREATE INDEX idx_oc_dispatch_leases_scope ON connector_dispatch_leases (scope, until);

CREATE TABLE connector_provider_retry_state (
    provider TEXT PRIMARY KEY,
    retry_after DATETIME NOT NULL,
    observed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
