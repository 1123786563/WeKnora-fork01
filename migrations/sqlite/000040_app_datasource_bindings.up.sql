-- A07: app_datasource_bindings maps each team-synced data source to the app
-- installation and space connection its writes execute under (A01 relation).
-- Same logical constraints as the PG 000120 dialect.
CREATE TABLE app_datasource_bindings (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    datasource_id TEXT NOT NULL,
    installation_id TEXT NOT NULL,
    connection_id TEXT NOT NULL,
    auth_version INTEGER NOT NULL DEFAULT 1,
    requires_reauthorization BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_app_ds_binding UNIQUE (tenant_id, datasource_id)
);

CREATE INDEX idx_app_ds_bindings_tenant ON app_datasource_bindings (tenant_id);
CREATE INDEX idx_app_ds_bindings_connection ON app_datasource_bindings (connection_id);
