-- C05: Craft recovery snapshots, SQLite dialect of PG
-- 000130_craft_snapshots (same logical constraints; craft_workspaces is the
-- retention root).
--
-- craft_snapshots stores one immutable recovery snapshot per quiescent
-- capture: the workspace, the version whose manifest pins the files, both
-- content digests (files manifest + exported OpenCode session chain), the
-- runtime digest the capture is only ever comparable against, the quiescent
-- fact, and the manifest (runtime/schema/skill summary) as JSON under its
-- own manifest version. The row cascades away with the workspace
-- (craft_workspaces is the retention root, same as the version tables).
-- craft_snapshot_objects pins every object the snapshot references with its
-- per-object checksum: file objects reuse the version's immutable storage
-- objects, session objects are the exported OpenCode persistent data the
-- capture uploaded into controlled storage. Rows are write-once: a snapshot
-- that was once stored never changes its objects afterwards (W01 semantics).
CREATE TABLE craft_snapshots (
    id VARCHAR(69) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    workspace_id VARCHAR(64) NOT NULL,
    version_id VARCHAR(68) NOT NULL,
    files_digest VARCHAR(64) NOT NULL,
    session_digest VARCHAR(64) NOT NULL,
    runtime_digest VARCHAR(128) NOT NULL,
    quiescent INTEGER NOT NULL,
    manifest_version VARCHAR(16) NOT NULL,
    manifest_json TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    CONSTRAINT fk_craft_snapshots_workspace
        FOREIGN KEY (workspace_id)
        REFERENCES craft_workspaces (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_snapshots_workspace ON craft_snapshots (workspace_id);

CREATE TABLE craft_snapshot_objects (
    snapshot_id VARCHAR(69) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    name VARCHAR(512) NOT NULL,
    resource_ref TEXT NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    bytes INTEGER NOT NULL,
    created_at DATETIME NOT NULL,
    PRIMARY KEY (snapshot_id, kind, name),
    CONSTRAINT fk_craft_snapshot_objects_snapshot
        FOREIGN KEY (snapshot_id)
        REFERENCES craft_snapshots (id) ON DELETE CASCADE
);
