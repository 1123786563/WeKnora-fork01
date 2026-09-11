-- Bind approved argument edits to a new immutable logical-call version.
ALTER TABLE agent_tool_calls
    ADD COLUMN plan_version BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN approved_plan_version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN plan_history JSONB NOT NULL DEFAULT '[]'::jsonb;
