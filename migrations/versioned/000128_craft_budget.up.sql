-- O02: Craft budget admission ledger, PostgreSQL dialect of SQLite
-- 000048_craft_budget (same logical constraints). craft_budget_grants is the
-- durable admission verdict of ONE run: unique per (tenant, run), grant_id is
-- the opaque identity the BudgetPort addresses, deadline/max_calls mirror the
-- grant snapshot, allowed flips to false on revoke and never resurrects. The
-- table carries NO money columns: funds, holds and settlement live
-- exclusively in the commercial tables (commercial_task_budgets,
-- commercial_reservations).
CREATE TABLE craft_budget_grants (
    tenant_id BIGINT NOT NULL,
    run_id TEXT NOT NULL,
    grant_id TEXT NOT NULL,
    deadline TIMESTAMPTZ NOT NULL,
    max_calls INTEGER NOT NULL,
    allowed BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id)
);

CREATE UNIQUE INDEX uq_craft_budget_grant_id ON craft_budget_grants (grant_id);

-- craft_budget_calls is the durable authorization ledger of ONE logical call:
-- unique per (tenant, call_key) so the same callID retried never admits twice,
-- and unique per binding sequence (tenant, run, delegation, model, funding,
-- call_seq) so the gateway's per-binding call sequence stays strictly
-- monotonic across restarts and recovery (call identities are re-derived from
-- this table, never restarted at zero). call_key is also the commercial
-- reservation key: one row here pairs with exactly one commercial hold.
CREATE TABLE craft_budget_calls (
    tenant_id BIGINT NOT NULL,
    call_key TEXT NOT NULL,
    grant_id TEXT NOT NULL,
    run_id TEXT NOT NULL DEFAULT '',
    delegation_id TEXT NOT NULL DEFAULT '',
    model_id TEXT NOT NULL,
    funding TEXT NOT NULL,
    call_seq BIGINT NOT NULL,
    call_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, call_key)
);

CREATE INDEX idx_craft_budget_calls_grant ON craft_budget_calls (tenant_id, grant_id);
CREATE UNIQUE INDEX uq_craft_budget_calls_call_id ON craft_budget_calls (tenant_id, call_id);
CREATE UNIQUE INDEX uq_craft_budget_calls_seq ON craft_budget_calls (tenant_id, run_id, delegation_id, model_id, funding, call_seq);
