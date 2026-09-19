-- P1.3 user state is added after the P1.2 namespace is already deployed.
-- owner_id stores the frozen SessionOwnerID and therefore may represent an
-- account user, tenant API key, API external user, or embed session.
CREATE TABLE IF NOT EXISTS native_user_state (
    tenant_id INTEGER NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    owner_id TEXT NOT NULL CHECK (length(trim(owner_id)) > 0),
    state_key TEXT NOT NULL CHECK (length(trim(state_key)) > 0),
    revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0),
    state_value TEXT NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, state_key)
);
CREATE INDEX IF NOT EXISTS idx_native_user_state_scope_revision ON native_user_state (tenant_id, owner_id, revision);
