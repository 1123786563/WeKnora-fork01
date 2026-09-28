-- Issue #110: tenant-scoped plugin installations (confirm slice, T06).
-- accepted_version + tools_snapshot form the runtime verification baseline
-- (issue #116). service_id points at the materialized mcp_services row.
-- manifest_url is copied from the consumed preview at confirm time: it is
-- the long-lived source the upgrade-preview / upgrade-accept flows re-fetch
-- (issue #114/#115) — the preview row itself is TTL-bound and consumed once.
-- up() writes NO mcp_tool_approvals rows: legacy manual tools are never
-- auto-promoted into approved plugin versions by migration (spec line 55).
CREATE TABLE plugin_installations (
 id VARCHAR(36) PRIMARY KEY,
 tenant_id BIGINT NOT NULL,
 plugin_id VARCHAR(128) NOT NULL,
 name VARCHAR(255) NOT NULL,
 description TEXT NOT NULL DEFAULT '',
 manifest_url VARCHAR(512) NOT NULL,
 accepted_version VARCHAR(64) NOT NULL,
 transport_type VARCHAR(50) NOT NULL,
 endpoint_url VARCHAR(512) NOT NULL,
 tools_snapshot JSONB NOT NULL,
 tools_digest VARCHAR(64) NOT NULL,
 service_id VARCHAR(36) NOT NULL DEFAULT '',
 drift_state VARCHAR(16) NOT NULL DEFAULT 'none',
 drift_detail JSONB,
 state VARCHAR(16) NOT NULL DEFAULT 'active',
 created_by VARCHAR(255) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX uq_plugin_installations_tenant_plugin ON plugin_installations(tenant_id, plugin_id);
CREATE INDEX idx_plugin_installations_service ON plugin_installations(service_id);

ALTER TABLE mcp_services ADD COLUMN plugin_installation_id VARCHAR(36);
CREATE INDEX idx_mcp_services_plugin_installation ON mcp_services(plugin_installation_id);
