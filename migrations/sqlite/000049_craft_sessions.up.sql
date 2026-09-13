-- W03: Craft sessions and the HTTP entrance's durable state, SQLite dialect
-- of PG 000129_craft_sessions (same logical constraints and the same
-- referential design: sessions rows are the retention root).
CREATE TABLE craft_sessions (
    session_id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    kind VARCHAR(32) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_sessions_tenant ON craft_sessions (tenant_id);

CREATE TABLE craft_session_requests (
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(512) NOT NULL,
    purpose VARCHAR(16) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id, purpose, request_id),
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE TABLE craft_workspace_inputs (
    workspace_id VARCHAR(64) NOT NULL,
    tenant_id INTEGER NOT NULL,
    ref TEXT NOT NULL,
    name VARCHAR(512) NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    bytes INTEGER NOT NULL,
    citation_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (workspace_id, ref),
    FOREIGN KEY (workspace_id) REFERENCES craft_workspaces (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_workspace_inputs_workspace ON craft_workspace_inputs (workspace_id);
