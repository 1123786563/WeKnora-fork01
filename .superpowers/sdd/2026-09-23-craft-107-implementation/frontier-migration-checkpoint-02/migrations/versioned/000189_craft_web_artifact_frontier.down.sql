DROP INDEX IF EXISTS idx_craft_task_grants_tenant_user;
DROP TABLE IF EXISTS craft_task_grants;
DROP TABLE IF EXISTS craft_knowledge_records;
ALTER TABLE craft_workspace_inputs DROP COLUMN recognition_reason;
ALTER TABLE craft_workspace_inputs DROP COLUMN recognition_understood;
ALTER TABLE craft_workspace_inputs DROP COLUMN recognition_accepted;
