ALTER TABLE workbench_interactions DROP COLUMN external_pending_id;
ALTER TABLE workbench_interactions DROP COLUMN credential_version;
DROP TABLE IF EXISTS execution_workspace_leases;
