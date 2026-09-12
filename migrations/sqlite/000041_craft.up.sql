-- R02: Craft workspaces and delegation journals, SQLite dialect of PG
-- 000121_craft (same logical constraints and the same referential design:
-- the workspace FK targets the sessions primary key, and delegations do not
-- physically FK the agent journal tables because the recovery program
-- replays the 000014 down script verbatim; run/tool call identity is
-- enforced by the repository under the live run row lock).
CREATE TABLE craft_workspaces (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    sandbox_id VARCHAR(128) NOT NULL DEFAULT '',
    generation VARCHAR(64) NOT NULL DEFAULT '0',
    oc_session_id VARCHAR(64) NOT NULL DEFAULT '',
    runtime_digest VARCHAR(128) NOT NULL DEFAULT '',
    revision BIGINT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_workspaces_scope UNIQUE (tenant_id, session_id),
    FOREIGN KEY (session_id)
        REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_workspaces_owner ON craft_workspaces (tenant_id, owner_id);

CREATE TABLE craft_delegations (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    tool_call_id VARCHAR(255) NOT NULL,
    workspace_id VARCHAR(64) NOT NULL,
    prompt_message_id VARCHAR(64) NOT NULL DEFAULT '',
    request_hash VARCHAR(64) NOT NULL,
    task_json TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'prepared',
    result_json TEXT,
    revision BIGINT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_delegations_call UNIQUE (tenant_id, run_id, tool_call_id),
    FOREIGN KEY (workspace_id)
        REFERENCES craft_workspaces (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_delegations_workspace ON craft_delegations (workspace_id);
CREATE INDEX idx_craft_delegations_status ON craft_delegations (tenant_id, status);
