DROP TABLE IF EXISTS agent_run_inputs;
DROP INDEX IF EXISTS uq_agent_run_decisions_applied_pending;
DROP TABLE IF EXISTS agent_run_decisions;
DROP TABLE IF EXISTS agent_run_events;
DROP TABLE IF EXISTS agent_tool_attempts;
DROP TABLE IF EXISTS agent_tool_calls;
DROP TABLE IF EXISTS agent_run_checkpoints;
DROP TABLE IF EXISTS agent_runs;

DROP INDEX IF EXISTS idx_sessions_active_agent_run;
DROP INDEX IF EXISTS uq_sessions_tenant_id_id;
ALTER TABLE sessions DROP COLUMN active_agent_run_id;
ALTER TABLE sessions DROP COLUMN engine_type;
