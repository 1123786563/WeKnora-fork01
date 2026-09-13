-- W01: immutable Craft artifact versions and their file manifests. One
-- craft_versions row per (workspace, run, manifest digest) records the
-- verification facts of that publish (checks_json); craft_version_files pins
-- every file's storage ref, content digest and size, so a published version's
-- downloads never change when the workspace is modified later — later rounds
-- publish new rows instead.
--
-- Referential design mirrors R02 000121: the version FK targets its
-- craft_workspaces parent and cascades away with it (session retention);
-- run identity is logical (the publishing service writes it under the live
-- delegation), never a physical FK onto the recovery program's journal
-- tables, whose down scripts this migration must not break.
CREATE TABLE craft_versions (
    id VARCHAR(71) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    workspace_id VARCHAR(64) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    manifest_hash VARCHAR(64) NOT NULL,
    checks_json JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_versions_identity UNIQUE (workspace_id, run_id, manifest_hash),
    CONSTRAINT fk_craft_versions_workspace
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
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_version_files_path UNIQUE (version_id, path),
    CONSTRAINT fk_craft_version_files_version
        FOREIGN KEY (version_id)
        REFERENCES craft_versions (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_version_files_hash ON craft_version_files (file_hash);
