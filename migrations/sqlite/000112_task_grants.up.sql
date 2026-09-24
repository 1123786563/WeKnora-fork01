-- Task collaboration grants (T12, #42) — sqlite track. Same shape as the
-- versioned migration; task_id references sessions.id (ADR-0004).
CREATE TABLE task_grants (
    tenant_id INTEGER NOT NULL,
    task_id TEXT NOT NULL,
    grantee_id TEXT NOT NULL,
    role TEXT NOT NULL,
    granted_by TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, task_id, grantee_id)
);

CREATE INDEX idx_task_grants_grantee ON task_grants (tenant_id, grantee_id);
