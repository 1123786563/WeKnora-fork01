CREATE TABLE craft_workspace_draft_origins (
    workspace_id TEXT PRIMARY KEY NOT NULL,
    tenant_id BIGINT NOT NULL,
    origin_revision BIGINT NOT NULL DEFAULT 0 CHECK (origin_revision = 0),
    origin_state TEXT NOT NULL DEFAULT 'empty' CHECK (origin_state = 'empty'),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, workspace_id),
    FOREIGN KEY (tenant_id, workspace_id) REFERENCES craft_workspaces(tenant_id, id) ON DELETE CASCADE
);

-- Backfill only structurally complete S1 lineages. Missing heads or revision
-- gaps stay unresolved; migration must not invent an empty predecessor.
INSERT INTO craft_workspace_draft_origins(workspace_id, tenant_id, origin_revision, origin_state)
SELECT h.workspace_id, h.tenant_id, 0, 'empty'
FROM craft_workspace_draft_heads h
JOIN craft_workspaces w ON w.id = h.workspace_id AND w.tenant_id = h.tenant_id
WHERE
    (h.state = 'empty' AND h.revision = 0 AND h.source_run_id IS NULL AND h.manifest_digest IS NULL
     AND NOT EXISTS (SELECT 1 FROM craft_workspace_draft_revisions r WHERE r.workspace_id = h.workspace_id))
 OR (h.state = 'selected' AND h.revision > 0 AND h.source_run_id IS NOT NULL AND h.manifest_digest IS NOT NULL
     AND EXISTS (
         SELECT 1 FROM craft_workspace_draft_revisions current_revision
         WHERE current_revision.workspace_id = h.workspace_id AND current_revision.tenant_id = h.tenant_id
           AND current_revision.revision = h.revision
           AND current_revision.source_run_id = h.source_run_id
           AND current_revision.manifest_digest = h.manifest_digest
     )
     AND (SELECT COUNT(*) FROM craft_workspace_draft_revisions r
          WHERE r.workspace_id = h.workspace_id AND r.tenant_id = h.tenant_id) = h.revision
     AND NOT EXISTS (
         SELECT 1 FROM craft_workspace_draft_revisions r
         LEFT JOIN agent_runs run ON run.tenant_id = r.tenant_id AND run.run_id = r.source_run_id
         WHERE r.workspace_id = h.workspace_id AND r.tenant_id = h.tenant_id
           AND (r.revision < 1 OR r.revision > h.revision OR run.run_id IS NULL
                OR run.owner_id <> w.owner_id OR run.session_id <> w.session_id
                OR run.status NOT IN ('succeeded', 'failed', 'canceled')
                OR NOT EXISTS (SELECT 1 FROM craft_workspace_draft_files f
                               WHERE f.workspace_id = r.workspace_id AND f.revision = r.revision))
     ));

CREATE TRIGGER trg_craft_draft_origin_no_update BEFORE UPDATE ON craft_workspace_draft_origins
BEGIN SELECT RAISE(ABORT, 'Craft draft origin is immutable'); END;
CREATE TRIGGER trg_craft_draft_origin_no_delete BEFORE DELETE ON craft_workspace_draft_origins
WHEN EXISTS (SELECT 1 FROM craft_workspaces w WHERE w.id = OLD.workspace_id)
BEGIN SELECT RAISE(ABORT, 'Craft draft origin can only be removed with its Workspace'); END;
CREATE TRIGGER trg_craft_draft_revision_no_update BEFORE UPDATE ON craft_workspace_draft_revisions
BEGIN SELECT RAISE(ABORT, 'Craft draft revisions are immutable'); END;
CREATE TRIGGER trg_craft_draft_revision_no_delete BEFORE DELETE ON craft_workspace_draft_revisions
WHEN EXISTS (SELECT 1 FROM craft_workspaces w WHERE w.id = OLD.workspace_id)
BEGIN SELECT RAISE(ABORT, 'Craft draft revisions can only be removed with their Workspace'); END;
CREATE TRIGGER trg_craft_draft_file_no_update BEFORE UPDATE ON craft_workspace_draft_files
BEGIN SELECT RAISE(ABORT, 'Craft draft files are immutable'); END;
CREATE TRIGGER trg_craft_draft_file_no_delete BEFORE DELETE ON craft_workspace_draft_files
WHEN EXISTS (SELECT 1 FROM craft_workspaces w WHERE w.id = OLD.workspace_id)
BEGIN SELECT RAISE(ABORT, 'Craft draft files can only be removed with their Workspace'); END;
