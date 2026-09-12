CREATE TABLE app_versions (
    app_id TEXT NOT NULL,
    version TEXT NOT NULL,
    schema_json TEXT NOT NULL DEFAULT '',
    risk_json TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (app_id, version)
);

CREATE TABLE installations (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    app_id TEXT NOT NULL,
    app_version TEXT NOT NULL,
    state TEXT NOT NULL,
    config_json TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_installations_tenant_app UNIQUE (tenant_id, app_id)
);

CREATE TABLE connections (
    tenant_id INTEGER NOT NULL,
    id TEXT NOT NULL,
    installation_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    owner_id TEXT NOT NULL DEFAULT '',
    credential_ref TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL,
    auth_version INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_connections_installation ON connections (tenant_id, installation_id);
