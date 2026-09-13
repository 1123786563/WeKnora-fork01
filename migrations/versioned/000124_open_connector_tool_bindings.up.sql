-- T14: agent tool-call bindings for open-connector app actions (PG; sqlite
-- twin is migrations/sqlite/000044, offset -80).
--
-- connector_tool_bindings binds ONE logical agent tool call — the composite
-- primary key (tenant_id, session_id, tool_call_id) — to the single persisted
-- app action it prepared. args_digest is the digest over the FULL logical
-- arguments (bound identity + connection + action + normalized input, digest
-- material generation 2): an identical replay of the same tool call reads the
-- same action back; ANY changed argument is a conflict, never a rebind. The
-- rows are immutable — there is no update path, only insert-once and read.
--
-- UNIQUE (tenant_id, action_id) pins the 1:1 between a tool call and its
-- action; the FK keeps a binding from ever pointing at a nonexistent action
-- (app_actions rows are never deleted, so the constraint is an integrity
-- guard, not a lifecycle coupling).
CREATE TABLE connector_tool_bindings (
    tenant_id BIGINT NOT NULL,
    session_id TEXT NOT NULL,
    tool_call_id TEXT NOT NULL,
    connection_id TEXT NOT NULL,
    action_name TEXT NOT NULL DEFAULT '',
    action_id TEXT NOT NULL,
    args_digest TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, session_id, tool_call_id),
    UNIQUE (tenant_id, action_id),
    FOREIGN KEY (action_id) REFERENCES app_actions (id)
);
