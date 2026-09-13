-- O01: Craft physical model-call usage ledger. One row per REVISION of one
-- physical attempt observation of one tenant: the main agent runtime and the
-- OpenCode child runtime each record their own real attempts (runtime
-- column), a retry is a new attempt id and therefore a new fact, and
-- (tenant_id, call_id, attempt_id, revision) is UNIQUE so a redelivered
-- observation can never be counted twice. Corrections append a new revision
-- (status corrected) and never overwrite recorded history: the revision
-- trail is the reconciliation record for stream breaks (unknown) and
-- post-cancellation late arrivals. Token columns follow the fixed provider
-- contract: cached_tokens is the cache-hit portion of input_tokens, never an
-- additive line. An unknown fact carries no token counts — it is its own
-- line item, never a fabricated zero. The table carries no FK into the
-- agent journal or craft tables: usage identity is enforced at write time by
-- the repository under the server-issued call identity, matching the
-- commercial usage ledger's referential design.
CREATE TABLE craft_usage_facts (
    id TEXT NOT NULL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    run_id TEXT NOT NULL DEFAULT '',
    delegation_id TEXT NOT NULL DEFAULT '',
    call_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    revision BIGINT NOT NULL,
    runtime TEXT NOT NULL,
    model_id TEXT NOT NULL,
    funding TEXT NOT NULL,
    status TEXT NOT NULL,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cached_tokens BIGINT NOT NULL DEFAULT 0,
    fact_json TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_usage_revision UNIQUE (tenant_id, call_id, attempt_id, revision)
);

CREATE INDEX idx_craft_usage_attempt ON craft_usage_facts (tenant_id, call_id, attempt_id);
CREATE INDEX idx_craft_usage_run ON craft_usage_facts (tenant_id, run_id);
CREATE INDEX idx_craft_usage_delegation ON craft_usage_facts (tenant_id, delegation_id);

-- Delivery outbox for the usage ledger, one durable event per fact revision
-- keyed by the stable identity usage:<usage_key>:<revision> so the official
-- OpenMeter pipeline (G4 specialty) can map CloudEvent fields/meters and
-- REDELIVER on failure without ever double-emitting. The payload carries the
-- raw UsageFact plus observed_at and revision; state/attempt_count track
-- delivery (pending/sent/dead, retry count). Unknown and corrected
-- revisions are enqueued too: downstream reconciliation must see them, and
-- the payload status tells the mapper an unknown fact must not become a
-- zero-usage meter event.
CREATE TABLE craft_usage_outbox (
    event_key TEXT NOT NULL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    call_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    revision BIGINT NOT NULL,
    payload_json TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    attempt_count BIGINT NOT NULL DEFAULT 0,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_usage_outbox_revision UNIQUE (tenant_id, call_id, attempt_id, revision)
);

CREATE INDEX idx_craft_usage_outbox_state ON craft_usage_outbox (state, updated_at);
