-- T03: inverse drops in safe order (children before parents).
DROP INDEX IF EXISTS idx_oc_outbox_next_at;
DROP INDEX IF EXISTS idx_oc_attempts_tenant_connection;
DROP TABLE IF EXISTS connector_operations_outbox;
DROP TABLE IF EXISTS connector_authorization_attempts;
DROP TABLE IF EXISTS connector_connection_bindings;
DROP TABLE IF EXISTS connector_action_definitions;
DROP TABLE IF EXISTS connector_runtimes;
