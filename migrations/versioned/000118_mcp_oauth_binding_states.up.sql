CREATE TABLE mcp_oauth_binding_states (
    state TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    actor_id TEXT NOT NULL,
    installation_id TEXT NOT NULL,
    service_id TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    used_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_mcp_oauth_binding_states_tenant ON mcp_oauth_binding_states (tenant_id);
