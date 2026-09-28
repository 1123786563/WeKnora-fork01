-- SQLite twin of the 000190 down: plugin-derived rows only; manual services
-- survive untouched. The index goes FIRST: SQLite refuses to drop an indexed
-- column (PostgreSQL drops dependent indexes implicitly — its down needs no
-- explicit DROP INDEX).
-- The derived-row deletes mirror the PG down one-for-one where the table
-- exists on the SQLite side: mcp_tool_approvals, mcp_oauth_tokens and
-- mcp_oauth_clients all key rows by service_id. SQLite FK declarations do
-- NOT back this up — production migrate DSNs never enable _foreign_keys
-- (mattn/go-sqlite3 defaults to OFF), so DELETE FROM mcp_services cascades
-- nowhere and any omitted table would strand orphan rows forever.
-- (mcp_metadata has no SQLite twin — the PG-only delete has no counterpart
-- here.)
DELETE FROM mcp_tool_approvals WHERE service_id IN (SELECT id FROM mcp_services WHERE plugin_installation_id IS NOT NULL);
DELETE FROM mcp_oauth_tokens WHERE service_id IN (SELECT id FROM mcp_services WHERE plugin_installation_id IS NOT NULL);
DELETE FROM mcp_oauth_clients WHERE service_id IN (SELECT id FROM mcp_services WHERE plugin_installation_id IS NOT NULL);
DELETE FROM mcp_services WHERE plugin_installation_id IS NOT NULL;
DROP TABLE IF EXISTS plugin_installations;
DROP INDEX IF EXISTS idx_mcp_services_plugin_installation;
ALTER TABLE mcp_services DROP COLUMN plugin_installation_id;
