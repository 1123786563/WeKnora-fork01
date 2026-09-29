-- Task collaboration grants (T12, #42): the task owner's explicit per-task
-- Viewer/Collaborator role assignments. Task = Session (ADR-0004), so task_id
-- references sessions.id; task ownership itself (sessions.user_id) never
-- lives here. Grant rows carry no tenant authority.
CREATE TABLE task_grants (
    tenant_id INTEGER NOT NULL,
    task_id VARCHAR(36) NOT NULL,
    grantee_id VARCHAR(512) NOT NULL,
    role VARCHAR(16) NOT NULL,
    granted_by VARCHAR(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, task_id, grantee_id)
);

CREATE INDEX idx_task_grants_grantee ON task_grants (tenant_id, grantee_id);
