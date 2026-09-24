-- Issue #110 down (GAP-3 rollback safety): remove ONLY plugin-derived rows;
-- manual (plugin_installation_id IS NULL) mcp_services survive untouched.
DO $$ BEGIN RAISE NOTICE '[Migration 000190 DOWN] removing plugin installations'; END $$;
DELETE FROM mcp_tool_approvals WHERE service_id IN (SELECT id FROM mcp_services WHERE plugin_installation_id IS NOT NULL);
DELETE FROM mcp_oauth_tokens WHERE service_id IN (SELECT id FROM mcp_services WHERE plugin_installation_id IS NOT NULL);
DELETE FROM mcp_oauth_clients WHERE service_id IN (SELECT id FROM mcp_services WHERE plugin_installation_id IS NOT NULL);
DELETE FROM mcp_metadata WHERE service_id IN (SELECT id FROM mcp_services WHERE plugin_installation_id IS NOT NULL);
DELETE FROM mcp_services WHERE plugin_installation_id IS NOT NULL;
DROP TABLE IF EXISTS plugin_installations;
ALTER TABLE mcp_services DROP COLUMN IF EXISTS plugin_installation_id;
