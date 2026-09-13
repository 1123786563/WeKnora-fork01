-- W03: Craft sessions and the HTTP entrance's durable state.
--
-- craft_sessions registers a session as a Craft artwork session (created with
-- engine_type=trpc) and pins its immutable kind. The row cascades away with
-- the session (the sessions row is the retention root, same as craft_workspaces).
-- craft_session_requests stores the tenant+user scoped idempotency keys: one
-- row per (tenant, user, purpose, request_id) holding the request hash, so a
-- retried create/run replays the original answer and a same-key different
-- parameters retry is a conflict. craft_workspace_inputs persists the inputs
-- associated with a workspace through POST /inputs (uploads completed through
-- the existing session attachment entrance), which is the authorization
-- manifest later runs resolve input_refs against.
CREATE TABLE craft_sessions (
    session_id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    kind VARCHAR(32) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_craft_sessions_session
        FOREIGN KEY (session_id)
        REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_sessions_tenant ON craft_sessions (tenant_id);

CREATE TABLE craft_session_requests (
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(512) NOT NULL,
    purpose VARCHAR(16) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id, purpose, request_id),
    CONSTRAINT fk_craft_session_requests_session
        FOREIGN KEY (session_id)
        REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE TABLE craft_workspace_inputs (
    workspace_id VARCHAR(64) NOT NULL,
    tenant_id INTEGER NOT NULL,
    ref TEXT NOT NULL,
    name VARCHAR(512) NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    bytes BIGINT NOT NULL,
    citation_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (workspace_id, ref),
    CONSTRAINT fk_craft_workspace_inputs_workspace
        FOREIGN KEY (workspace_id)
        REFERENCES craft_workspaces (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_workspace_inputs_workspace ON craft_workspace_inputs (workspace_id);
