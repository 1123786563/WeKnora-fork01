-- T20: one durable server-owned extension action for the current budget pause.
-- A successful resume marks it completed; a later pause replaces the completed
-- action with a new key and configured quantum.
CREATE TABLE craft_budget_extension_intents (
    tenant_id BIGINT NOT NULL,
    session_id VARCHAR(128) NOT NULL,
    run_id VARCHAR(128) NOT NULL,
    intent_key VARCHAR(128) NOT NULL,
    extra_calls INTEGER NOT NULL,
    extra_credits BIGINT NOT NULL,
    status VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, session_id, run_id),
    UNIQUE (tenant_id, run_id, intent_key)
);
