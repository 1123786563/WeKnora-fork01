ALTER TABLE agent_run_decisions ADD COLUMN args_hash VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE agent_run_decisions ADD COLUMN resource_ref VARCHAR(1024) NOT NULL DEFAULT '';
