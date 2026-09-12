-- T03: rollback of the open-connector persistence. Refuses to run while
-- any binding rows exist — live or revoked, they are audit records; the
-- operational rollback path is disabling the feature, not deleting audit.
DO $$
BEGIN
    IF to_regclass('connector_connection_bindings') IS NOT NULL AND
       EXISTS (SELECT 1 FROM connector_connection_bindings) THEN
        RAISE EXCEPTION 'connector_connection_bindings is not empty: disable the feature instead of dropping binding audit';
    END IF;
END
$$;

DROP INDEX IF EXISTS idx_oc_outbox_next_at;
DROP INDEX IF EXISTS idx_oc_attempts_tenant_connection;
DROP TABLE IF EXISTS connector_operations_outbox;
DROP TABLE IF EXISTS connector_authorization_attempts;
DROP TABLE IF EXISTS connector_connection_bindings;
DROP TABLE IF EXISTS connector_action_definitions;
DROP TABLE IF EXISTS connector_runtimes;
