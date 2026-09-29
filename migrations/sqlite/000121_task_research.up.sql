-- T17 (#47) — sqlite track. Same shape as the versioned migration; delegation
-- rows never touch the single write slot, annotations are append-only.
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
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
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
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_task_annotation_session ON task_artifact_annotations (tenant_id, session_id);
CREATE INDEX idx_task_annotation_material ON task_artifact_annotations (tenant_id, session_id, material_id);
