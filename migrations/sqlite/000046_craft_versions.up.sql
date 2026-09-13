-- W01: immutable Craft artifact versions, SQLite dialect of PG
-- 000126_craft_versions (same logical constraints and the same referential
-- design: versions cascade away with their craft_workspaces parent; run
-- identity is logical, not a physical FK onto the recovery journal tables).
CREATE TABLE craft_versions (
    id VARCHAR(71) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    workspace_id VARCHAR(64) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    manifest_hash VARCHAR(64) NOT NULL,
    checks_json TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_versions_identity UNIQUE (workspace_id, run_id, manifest_hash),
    FOREIGN KEY (workspace_id)
        REFERENCES craft_workspaces (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_versions_tenant_run ON craft_versions (tenant_id, run_id);

CREATE TABLE craft_version_files (
    version_id VARCHAR(71) NOT NULL,
    path VARCHAR(512) NOT NULL,
    resource_ref VARCHAR(1024) NOT NULL,
    file_hash VARCHAR(64) NOT NULL,
    file_bytes BIGINT NOT NULL,
    mime VARCHAR(128) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_version_files_path UNIQUE (version_id, path),
    FOREIGN KEY (version_id)
        REFERENCES craft_versions (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_version_files_hash ON craft_version_files (file_hash);
