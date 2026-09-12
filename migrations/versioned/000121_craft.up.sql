-- R02: Craft workspaces bind one sandboxed OpenCode runtime per session so a
-- second round on the same artwork reuses the same working directory, and
-- craft_delegations persist sub-executions under real tRPC agent runs.
--
-- Referential design: the workspace FK targets sessions(id) — the primary
-- key — so deleting a session cascades the whole Craft chain (retention
-- policy). craft_delegations does NOT physically FK agent_runs or
-- agent_tool_calls: the recovery program's own contract test replays the
-- 000093 down script verbatim, and PostgreSQL refuses to drop journal
-- tables that later migrations reference (SQLite fails resolving the
-- dropped parents similarly). Run, tool call and session identity are
-- instead enforced at write time by the repository under the live run row
-- lock (PrepareTask/SaveResult), and delegations cascade away with their
-- workspace when the session is deleted.
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
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_workspaces_scope UNIQUE (tenant_id, session_id),
    CONSTRAINT fk_craft_workspaces_session
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
    task_json JSONB NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'prepared',
    result_json JSONB,
    revision BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_delegations_call UNIQUE (tenant_id, run_id, tool_call_id),
    CONSTRAINT fk_craft_delegations_workspace
        FOREIGN KEY (workspace_id)
        REFERENCES craft_workspaces (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_delegations_workspace ON craft_delegations (workspace_id);
CREATE INDEX idx_craft_delegations_status ON craft_delegations (tenant_id, status);
