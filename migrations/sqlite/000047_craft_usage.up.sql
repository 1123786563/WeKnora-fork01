-- O01: Craft physical model-call usage ledger, SQLite dialect of PG
-- 000127_craft_usage (same logical constraints: one row per revision of one
-- physical attempt per tenant, main and OC child runtimes recorded
-- separately, retries are new attempt facts, redelivery is idempotent,
-- corrections append revisions without overwriting, unknown facts carry no
-- token counts, and no FK into the agent journal or craft tables — usage
-- identity is enforced at write time by the repository).
CREATE TABLE craft_usage_facts (
    id TEXT NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL DEFAULT '',
    delegation_id TEXT NOT NULL DEFAULT '',
    call_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    runtime TEXT NOT NULL,
    model_id TEXT NOT NULL,
    funding TEXT NOT NULL,
    status TEXT NOT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER NOT NULL DEFAULT 0,
    fact_json TEXT NOT NULL,
    observed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_usage_revision UNIQUE (tenant_id, call_id, attempt_id, revision)
);

CREATE INDEX idx_craft_usage_attempt ON craft_usage_facts (tenant_id, call_id, attempt_id);
CREATE INDEX idx_craft_usage_run ON craft_usage_facts (tenant_id, run_id);
CREATE INDEX idx_craft_usage_delegation ON craft_usage_facts (tenant_id, delegation_id);

-- Delivery outbox: one durable event per fact revision under the stable
-- identity usage:<usage_key>:<revision>; payload carries the raw UsageFact
-- with observed_at and revision, state/attempt_count track delivery and
-- retries for the OpenMeter mapping (G4 specialty).
CREATE TABLE craft_usage_outbox (
    event_key TEXT NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    call_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    payload_json TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    observed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_usage_outbox_revision UNIQUE (tenant_id, call_id, attempt_id, revision)
);

CREATE INDEX idx_craft_usage_outbox_state ON craft_usage_outbox (state, updated_at);
