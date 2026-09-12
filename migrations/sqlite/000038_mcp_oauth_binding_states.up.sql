CREATE TABLE mcp_oauth_binding_states (
    state TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    actor_id TEXT NOT NULL,
    installation_id TEXT NOT NULL,
    service_id TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    used BOOLEAN NOT NULL DEFAULT 0,
    used_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_mcp_oauth_binding_states_tenant ON mcp_oauth_binding_states (tenant_id);
