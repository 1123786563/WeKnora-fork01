-- P1.3 user state is added after the P1.2 namespace is already deployed.
-- owner_id stores the frozen SessionOwnerID and therefore may represent an
-- account user, tenant API key, API external user, or embed session.
CREATE TABLE IF NOT EXISTS native_user_state (
    tenant_id BIGINT NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    owner_id VARCHAR(255) NOT NULL CHECK (length(btrim(owner_id)) > 0),
    state_key VARCHAR(255) NOT NULL CHECK (length(btrim(state_key)) > 0),
    revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
    state_value JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, state_key)
);
CREATE INDEX IF NOT EXISTS idx_native_user_state_scope_revision ON native_user_state (tenant_id, owner_id, revision);
