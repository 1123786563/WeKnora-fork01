-- SQLite twin of the 000190 down: plugin-derived rows only; manual services
-- survive untouched. The index goes FIRST: SQLite refuses to drop an indexed
-- column (PostgreSQL drops dependent indexes implicitly — its down needs no
-- explicit DROP INDEX).
DELETE FROM mcp_tool_approvals WHERE service_id IN (SELECT id FROM mcp_services WHERE plugin_installation_id IS NOT NULL);
DELETE FROM mcp_services WHERE plugin_installation_id IS NOT NULL;
DROP TABLE IF EXISTS plugin_installations;
DROP INDEX IF EXISTS idx_mcp_services_plugin_installation;
ALTER TABLE mcp_services DROP COLUMN plugin_installation_id;
