-- T14: agent tool-call bindings for open-connector app actions (sqlite twin
-- of PG 000124, offset +80). Same composite primary key, same 1:1 action
-- uniqueness and the same FK to app_actions (the test fixture enables
-- foreign_keys).
CREATE TABLE connector_tool_bindings (
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    tool_call_id TEXT NOT NULL,
    connection_id TEXT NOT NULL,
    action_name TEXT NOT NULL DEFAULT '',
    action_id TEXT NOT NULL,
    args_digest TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, session_id, tool_call_id),
    UNIQUE (tenant_id, action_id),
    FOREIGN KEY (action_id) REFERENCES app_actions (id)
);
