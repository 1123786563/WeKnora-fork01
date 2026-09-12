-- T03: open-connector shared-runtime persistence (PG; sqlite twin is
-- migrations/sqlite/000041). Credentials exist here as secret REFERENCES
-- only — never as material. Existing native connections are NOT backfilled
-- into OC bindings.
CREATE TABLE connector_runtimes (
    id TEXT PRIMARY KEY,
    address TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    version TEXT NOT NULL DEFAULT '',
    image_digest TEXT NOT NULL DEFAULT '',
    secret_refs TEXT NOT NULL DEFAULT ''
);

CREATE TABLE connector_action_definitions (
    app_id TEXT NOT NULL,
    app_version TEXT NOT NULL,
    action_id TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    schema_digest TEXT NOT NULL DEFAULT '',
    input_schema TEXT NOT NULL DEFAULT '',
    required_scopes TEXT NOT NULL DEFAULT '',
    risk TEXT NOT NULL DEFAULT '',
    published BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (app_id, app_version, action_id)
);

CREATE TABLE connector_connection_bindings (
    tenant_id BIGINT NOT NULL,
    connection_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    alias TEXT NOT NULL,
    auth_version BIGINT NOT NULL CHECK(auth_version > 0),
    binding_version BIGINT NOT NULL CHECK(binding_version > 0),
    state TEXT NOT NULL CHECK(state IN ('pending','active','revoked','error')),
    PRIMARY KEY(tenant_id, connection_id),
    UNIQUE(runtime_id, external_id),
    UNIQUE(runtime_id, provider, alias),
    FOREIGN KEY(tenant_id, connection_id) REFERENCES connections(tenant_id, id)
);

CREATE TABLE connector_authorization_attempts (
    id TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    actor_id TEXT NOT NULL DEFAULT '',
    connection_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    alias TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL,
    auth_version BIGINT NOT NULL DEFAULT 1 CHECK(auth_version > 0),
    expires_at TIMESTAMP NOT NULL,
    FOREIGN KEY(tenant_id, connection_id) REFERENCES connections(tenant_id, id)
);

CREATE TABLE connector_operations_outbox (
    id TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    resource_id TEXT NOT NULL DEFAULT '',
    resource_version BIGINT NOT NULL DEFAULT 0,
    kind TEXT NOT NULL,
    attempts BIGINT NOT NULL DEFAULT 0,
    next_at TIMESTAMP NOT NULL,
    lease_owner TEXT NOT NULL DEFAULT '',
    fence BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_oc_attempts_tenant_connection ON connector_authorization_attempts (tenant_id, connection_id);
CREATE INDEX idx_oc_outbox_next_at ON connector_operations_outbox (next_at);
