-- Split durable execution dispatch from the legacy tRPC worker.
ALTER TABLE agent_runs
    ADD COLUMN driver VARCHAR(16) NOT NULL DEFAULT 'platform',
    ADD COLUMN target_id VARCHAR(512) NOT NULL DEFAULT '',
    ADD COLUMN budget_ref VARCHAR(512) NOT NULL DEFAULT '';

ALTER TABLE agent_runs DROP CONSTRAINT ck_agent_runs_engine;
ALTER TABLE agent_runs ADD CONSTRAINT ck_agent_runs_engine CHECK (
    (driver = 'platform' AND engine_type = 'trpc')
    OR (driver = 'paseo' AND engine_type = '')
);

CREATE INDEX idx_agent_runs_driver_scan
    ON agent_runs (driver, status, lease_until, created_at);
