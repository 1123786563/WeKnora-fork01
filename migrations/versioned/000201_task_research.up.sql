-- T17 (#47): Lead Agent read-only research delegations and version-pinned
-- material annotations. Delegation rows never touch sessions.active_agent_run_id
-- (the single write slot stays owned by AgentRunStore.Admit); annotations are
-- append-only and bind the exact artifact version identity they reviewed.
CREATE TABLE task_research_delegations (
    tenant_id INTEGER NOT NULL,
    id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    parent_run_id VARCHAR(64) NOT NULL,
    objective TEXT NOT NULL,
    sources_json TEXT NOT NULL DEFAULT '[]',
    status VARCHAR(16) NOT NULL DEFAULT 'assigned',
    summary TEXT NOT NULL DEFAULT '',
    created_by VARCHAR(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_task_research_session ON task_research_delegations (tenant_id, session_id);

CREATE TABLE task_artifact_annotations (
    tenant_id INTEGER NOT NULL,
    id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    material_id TEXT NOT NULL,
    base_version TEXT NOT NULL,
    body TEXT NOT NULL,
    author_id VARCHAR(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_task_annotation_session ON task_artifact_annotations (tenant_id, session_id);
CREATE INDEX idx_task_annotation_material ON task_artifact_annotations (tenant_id, session_id, material_id);
