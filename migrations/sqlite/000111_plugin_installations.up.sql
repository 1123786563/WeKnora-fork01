-- SQLite twin of versioned migration 000190 (type conventions: JSONB→TEXT,
-- TIMESTAMPTZ→DATETIME, BIGINT→INTEGER).
CREATE TABLE plugin_installations (
 id VARCHAR(36) PRIMARY KEY,
 tenant_id INTEGER NOT NULL,
 plugin_id VARCHAR(128) NOT NULL,
 name VARCHAR(255) NOT NULL,
 description TEXT NOT NULL DEFAULT '',
 manifest_url VARCHAR(512) NOT NULL,
 accepted_version VARCHAR(64) NOT NULL,
 transport_type VARCHAR(50) NOT NULL,
 endpoint_url VARCHAR(512) NOT NULL,
 tools_snapshot TEXT NOT NULL,
 tools_digest VARCHAR(64) NOT NULL,
 service_id VARCHAR(36) NOT NULL DEFAULT '',
 drift_state VARCHAR(16) NOT NULL DEFAULT 'none',
 drift_detail TEXT,
 state VARCHAR(16) NOT NULL DEFAULT 'active',
 created_by VARCHAR(255) NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_plugin_installations_tenant_plugin ON plugin_installations(tenant_id, plugin_id);
CREATE INDEX idx_plugin_installations_service ON plugin_installations(service_id);

ALTER TABLE mcp_services ADD COLUMN plugin_installation_id VARCHAR(36);
CREATE INDEX idx_mcp_services_plugin_installation ON mcp_services(plugin_installation_id);
