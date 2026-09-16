-- Older binaries cannot safely execute or decode remote-driver runs.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM agent_runs WHERE driver = 'paseo') THEN
        RAISE EXCEPTION 'cannot downgrade workbench runs while paseo data exists';
    END IF;
END
$$;

ALTER TABLE agent_runs DROP CONSTRAINT ck_agent_runs_engine;
DROP INDEX idx_agent_runs_driver_scan;
ALTER TABLE agent_runs
    DROP COLUMN budget_ref,
    DROP COLUMN target_id,
    DROP COLUMN driver;
ALTER TABLE agent_runs ADD CONSTRAINT ck_agent_runs_engine CHECK (engine_type = 'trpc');
