-- A07: app_datasource_bindings maps each team-synced data source to the app
-- installation and space connection its writes execute under (A01 relation).
-- Cursor/checkpoint validity is scoped to auth_version: re-consent invalidates
-- cursors produced under the previous credential version.
CREATE TABLE app_datasource_bindings (
    id TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    datasource_id TEXT NOT NULL,
    installation_id TEXT NOT NULL,
    connection_id TEXT NOT NULL,
    auth_version BIGINT NOT NULL DEFAULT 1,
    requires_reauthorization BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_app_datasource_binding UNIQUE (tenant_id, datasource_id)
);

CREATE INDEX idx_app_ds_bindings_tenant ON app_datasource_bindings (tenant_id);
CREATE INDEX idx_app_ds_bindings_connection ON app_datasource_bindings (connection_id);
