-- Issue #108: verified manifest previews (admin review artifact, TTL-bound,
-- consumed exactly once by installation confirm in 000190).
CREATE TABLE plugin_previews (
 id VARCHAR(36) PRIMARY KEY,
 tenant_id BIGINT NOT NULL,
 manifest_url VARCHAR(512) NOT NULL,
 plugin_id VARCHAR(128) NOT NULL,
 version VARCHAR(64) NOT NULL,
 name VARCHAR(255) NOT NULL,
 transport_type VARCHAR(50) NOT NULL,
 endpoint_url VARCHAR(512) NOT NULL,
 tools_snapshot JSONB NOT NULL,
 tools_digest VARCHAR(64) NOT NULL,
 identity_fingerprint VARCHAR(64) NOT NULL,
 created_by VARCHAR(255) NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 consumed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_plugin_previews_tenant ON plugin_previews(tenant_id, plugin_id);
