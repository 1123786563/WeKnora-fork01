ALTER TABLE agent_tool_calls
    DROP COLUMN plan_history,
    DROP COLUMN approved_plan_version,
    DROP COLUMN plan_version;
