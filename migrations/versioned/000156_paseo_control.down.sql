ALTER TABLE workbench_interactions DROP CONSTRAINT IF EXISTS ck_workbench_interaction_credential_version;
ALTER TABLE workbench_interactions DROP COLUMN IF EXISTS credential_version;
ALTER TABLE workbench_interactions DROP COLUMN IF EXISTS external_pending_id;
DROP TABLE IF EXISTS execution_workspace_leases;
