CREATE TABLE craft_candidates (
    id TEXT PRIMARY KEY NOT NULL,
    tenant_id BIGINT NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    workspace_id VARCHAR(64) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    generation VARCHAR(128) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    manifest_digest CHAR(64) NOT NULL,
    checks_json TEXT NOT NULL,
    evidence_json TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, workspace_id, run_id),
    UNIQUE (tenant_id, workspace_id, id),
    FOREIGN KEY (tenant_id, workspace_id) REFERENCES craft_workspaces(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);

CREATE TABLE craft_candidate_files (
    candidate_id TEXT NOT NULL,
    path VARCHAR(512) NOT NULL,
    resource_ref TEXT NOT NULL,
    file_hash CHAR(64) NOT NULL,
    file_bytes BIGINT NOT NULL CHECK (file_bytes >= 0),
    mime TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (candidate_id, path),
    FOREIGN KEY (candidate_id) REFERENCES craft_candidates(id) ON DELETE RESTRICT
);

CREATE INDEX idx_craft_candidates_scope_workspace
    ON craft_candidates (tenant_id, owner_id, session_id, workspace_id, created_at DESC);

CREATE TRIGGER trg_craft_candidate_no_update
BEFORE UPDATE ON craft_candidates
BEGIN
    SELECT RAISE(ABORT, 'craft candidate is immutable');
END;

CREATE TRIGGER trg_craft_candidate_no_delete
BEFORE DELETE ON craft_candidates
BEGIN
    SELECT RAISE(ABORT, 'craft candidate is immutable');
END;

CREATE TRIGGER trg_craft_candidate_file_no_update
BEFORE UPDATE ON craft_candidate_files
BEGIN
    SELECT RAISE(ABORT, 'craft candidate file is immutable');
END;

CREATE TRIGGER trg_craft_candidate_file_no_delete
BEFORE DELETE ON craft_candidate_files
BEGIN
    SELECT RAISE(ABORT, 'craft candidate file is immutable');
END;
