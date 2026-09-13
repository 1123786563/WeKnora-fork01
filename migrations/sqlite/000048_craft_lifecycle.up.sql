-- O03: Craft sandbox residency, snapshots and resource reclamation,
-- SQLite dialect of PG 000128_craft_lifecycle (same logical constraints).
--
-- craft_lifecycle_states is the durable sweep ledger of ONE reclaimable
-- resource. A row is the tombstone that blocks new dispatch/restore the
-- moment session deletion starts (state 'deleting' written FIRST), the
-- candidate record that lets an orphan object wait out its 24h window, the
-- CAS mark that serializes the provider delete, and the retry/risk record a
-- failure or an unknown remote task leaves behind. No physical FK to
-- sessions or craft_workspaces on purpose: a tombstone must outlive the
-- session row it references and the sweep queries rows whose session may
-- already be gone; identity is enforced at write time (the craft_delegations
-- precedent).
CREATE TABLE craft_lifecycle_states (
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    resource_kind TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    sandbox_id TEXT NOT NULL DEFAULT '',
    generation TEXT NOT NULL DEFAULT '',
    resource_ref TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    eligible_at DATETIME NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, session_id, resource_kind, sandbox_id, resource_ref)
);

CREATE INDEX idx_craft_lifecycle_states_sweep
    ON craft_lifecycle_states (state, eligible_at);
CREATE INDEX idx_craft_lifecycle_states_session
    ON craft_lifecycle_states (tenant_id, session_id, state);

-- craft_lifecycle_events is the deduplicated usage fact stream of sandbox
-- residency and storage: sandbox_start/sandbox_stop pair into dwell time,
-- storage_bytes observations carry the bytes held. id IS the content-identity
-- dedup key (craft.LifecycleEventKey), so an at-least-once redelivery is a
-- no-op instead of double-counted usage — the O01 UsageKey discipline.
CREATE TABLE craft_lifecycle_events (
    id TEXT NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    kind TEXT NOT NULL,
    session_id TEXT NOT NULL DEFAULT '',
    sandbox_id TEXT NOT NULL DEFAULT '',
    bytes INTEGER NOT NULL DEFAULT 0,
    occurred_at DATETIME NOT NULL,
    recorded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_craft_lifecycle_events_usage
    ON craft_lifecycle_events (tenant_id, kind, occurred_at);
