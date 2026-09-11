ALTER TABLE agent_tool_calls ADD COLUMN plan_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE agent_tool_calls ADD COLUMN approved_plan_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_tool_calls ADD COLUMN plan_history TEXT NOT NULL DEFAULT '[]';
