-- T10: durable dispatch idempotency records and the distributed concurrency
-- lease (PG; sqlite twin is migrations/sqlite/000043, offset -80).
--
-- connector_dispatch_records is the local linearization point of one approved
-- open-connector dispatch: PRIMARY KEY (tenant_id, action_id) makes the claim
-- single-shot per action, UNIQUE (runtime_id, key) pins the operation-scoped
-- idempotency key ("wk-oc-" + uuid, NEVER a connection id) to one runtime.
-- first_sent_at defaults to database time; the claim writes replay_until as
-- first_sent_at + 23h50m (upstream 24h window headroom, T01) and NO later
-- update ever advances either timestamp. Result updates run under
-- tenant/action/fence/state guards only.
CREATE TABLE connector_dispatch_records (
    tenant_id BIGINT NOT NULL,
    action_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL,
    key TEXT NOT NULL,
    execution_id TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'dispatched' CHECK (state IN ('dispatched','succeeded','failed','unknown')),
    reservation_id TEXT NOT NULL DEFAULT '',
    first_sent_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    replay_until TIMESTAMP NOT NULL,
    fence BIGINT NOT NULL DEFAULT 1 CHECK (fence > 0),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, action_id),
    UNIQUE (runtime_id, key)
);

CREATE INDEX idx_oc_dispatch_state ON connector_dispatch_records (state);

-- The concurrency lease: one row per held slot. connector_dispatch_lease_scopes
-- serializes acquisitions per scope (SELECT ... FOR UPDATE) so N replicas can
-- never overshoot a limit; expired (until <= now) or released rows are free
-- slots reclaimed by the next acquisition. No single-process semaphore.
CREATE TABLE connector_dispatch_lease_scopes (
    scope TEXT PRIMARY KEY,
    acquired_total BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE connector_dispatch_leases (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL,
    tenant_id BIGINT NOT NULL,
    owner TEXT NOT NULL,
    action_id TEXT NOT NULL DEFAULT '',
    acquired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    until TIMESTAMP NOT NULL,
    released_at TIMESTAMP,
    fence BIGINT NOT NULL DEFAULT 1 CHECK (fence > 0)
);

CREATE INDEX idx_oc_dispatch_leases_scope ON connector_dispatch_leases (scope, until);

-- Durable provider Retry-After (429): new dispatches fail closed until the
-- stored instant passes; the value only ever moves forward.
CREATE TABLE connector_provider_retry_state (
    provider TEXT PRIMARY KEY,
    retry_after TIMESTAMP NOT NULL,
    observed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
