DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM native_agent_tenants
        UNION ALL SELECT 1 FROM native_agent_sessions
        UNION ALL SELECT 1 FROM native_agent_runs
        UNION ALL SELECT 1 FROM native_agent_inputs
        UNION ALL SELECT 1 FROM native_agent_config_bindings
        UNION ALL SELECT 1 FROM native_agent_memory_scopes
        UNION ALL SELECT 1 FROM native_agent_memory_entries
        UNION ALL SELECT 1 FROM native_agent_attempts
        UNION ALL SELECT 1 FROM native_agent_tool_calls
        UNION ALL SELECT 1 FROM native_agent_tool_results
        UNION ALL SELECT 1 FROM native_agent_pending_decisions
        UNION ALL SELECT 1 FROM native_agent_commit_intents
        UNION ALL SELECT 1 FROM native_agent_checkpoints
        UNION ALL SELECT 1 FROM native_agent_session_events
        UNION ALL SELECT 1 FROM native_agent_events
        UNION ALL SELECT 1 FROM native_agent_usage_observations
    ) THEN
        RAISE EXCEPTION 'native agent schema rollback refused: namespace contains data';
    END IF;
END $$;

DROP TABLE native_agent_usage_observations;
DROP TABLE native_agent_events;
DROP TABLE native_agent_session_events;
DROP TABLE native_agent_checkpoints;
DROP TABLE native_agent_commit_intents;
DROP TABLE native_agent_pending_decisions;
DROP TABLE native_agent_tool_results;
DROP TABLE native_agent_tool_calls;
DROP TABLE native_agent_attempts;
DROP TABLE native_agent_memory_entries;
DROP TABLE native_agent_memory_scopes;
DROP TABLE native_agent_config_bindings;
DROP TABLE native_agent_inputs;
DROP TABLE native_agent_runs;
DROP TABLE native_agent_sessions;
DROP TABLE native_agent_tenants;
