CREATE TABLE craft_run_captures (
    tenant_id BIGINT NOT NULL,
    workspace_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    generation TEXT NOT NULL,
    predecessor_revision BIGINT NOT NULL CHECK (predecessor_revision >= 0),
    predecessor_state TEXT NOT NULL CHECK (predecessor_state IN ('empty','selected')),
    predecessor_run_id TEXT NOT NULL DEFAULT '',
    predecessor_digest TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (state IN ('pending','capturing','sealed','advanced','blocked')),
    manifest_digest TEXT NOT NULL DEFAULT '',
    draft_revision BIGINT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, workspace_id, run_id),
    FOREIGN KEY (tenant_id, workspace_id) REFERENCES craft_workspaces(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);
CREATE INDEX idx_craft_run_captures_recovery ON craft_run_captures(state, updated_at);

CREATE TABLE craft_run_capture_files (
    tenant_id BIGINT NOT NULL,
    workspace_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    path TEXT NOT NULL,
    object_ref TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    bytes BIGINT NOT NULL CHECK (bytes >= 0),
    mime TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (tenant_id, workspace_id, run_id, path),
    FOREIGN KEY (tenant_id, workspace_id, run_id)
        REFERENCES craft_run_captures(tenant_id, workspace_id, run_id) ON DELETE CASCADE
);

CREATE TRIGGER trg_craft_capture_identity_immutable BEFORE UPDATE OF tenant_id,workspace_id,run_id,owner_id,session_id,generation,predecessor_revision,predecessor_state,predecessor_run_id,predecessor_digest ON craft_run_captures
BEGIN SELECT RAISE(ABORT, 'Craft capture identity is immutable'); END;
CREATE TRIGGER trg_craft_capture_advanced_immutable BEFORE UPDATE ON craft_run_captures
WHEN OLD.state='advanced'
BEGIN SELECT RAISE(ABORT, 'Advanced Craft capture is immutable'); END;
CREATE TRIGGER trg_craft_capture_sealed_digest_immutable BEFORE UPDATE OF manifest_digest ON craft_run_captures
WHEN OLD.state IN ('sealed','advanced') AND NEW.manifest_digest IS NOT OLD.manifest_digest
BEGIN SELECT RAISE(ABORT, 'Sealed Craft capture digest is immutable'); END;
CREATE TRIGGER trg_craft_capture_file_no_insert BEFORE INSERT ON craft_run_capture_files
WHEN EXISTS (SELECT 1 FROM craft_run_captures c WHERE c.tenant_id=NEW.tenant_id AND c.workspace_id=NEW.workspace_id AND c.run_id=NEW.run_id AND c.state IN ('sealed','advanced'))
BEGIN SELECT RAISE(ABORT, 'Sealed Craft capture files are immutable'); END;
CREATE TRIGGER trg_craft_capture_file_no_update BEFORE UPDATE ON craft_run_capture_files
WHEN EXISTS (SELECT 1 FROM craft_run_captures c WHERE c.tenant_id=OLD.tenant_id AND c.workspace_id=OLD.workspace_id AND c.run_id=OLD.run_id AND c.state IN ('sealed','advanced'))
BEGIN SELECT RAISE(ABORT, 'Sealed Craft capture files are immutable'); END;
CREATE TRIGGER trg_craft_capture_file_no_delete BEFORE DELETE ON craft_run_capture_files
WHEN EXISTS (SELECT 1 FROM craft_run_captures c WHERE c.tenant_id=OLD.tenant_id AND c.workspace_id=OLD.workspace_id AND c.run_id=OLD.run_id AND c.state IN ('sealed','advanced'))
BEGIN SELECT RAISE(ABORT, 'Sealed Craft capture files are immutable'); END;

-- Terminal transitions are durably discoverable even if the worker exits
-- immediately after committing the Run. Missing RunViews are handled by the
-- recovery scan and remain fenced until an authoritative source is available.
CREATE TRIGGER trg_craft_capture_terminal_enqueue AFTER UPDATE OF status ON agent_runs
WHEN NEW.status IN ('succeeded','failed','canceled') AND OLD.status NOT IN ('succeeded','failed','canceled')
BEGIN
  INSERT INTO craft_run_captures
    (tenant_id, workspace_id, run_id, owner_id, session_id, generation,
     predecessor_revision, predecessor_state, predecessor_run_id, predecessor_digest, state)
  SELECT NEW.tenant_id, w.id, NEW.run_id, NEW.owner_id, NEW.session_id, rv.generation,
         CAST(json_extract(NEW.snapshot,'$.craft_workspace_seed.draft_revision') AS INTEGER),
         json_extract(NEW.snapshot,'$.craft_workspace_seed.state'),
         COALESCE(json_extract(NEW.snapshot,'$.craft_workspace_seed.source_run_id'),''),
         COALESCE(json_extract(NEW.snapshot,'$.craft_workspace_seed.manifest_digest'),''), 'pending'
    FROM craft_workspaces w
    JOIN craft_run_views rv ON rv.tenant_id=NEW.tenant_id AND rv.run_id=NEW.run_id AND rv.state='bound'
    JOIN craft_workspace_draft_heads h ON h.tenant_id=w.tenant_id AND h.workspace_id=w.id
   WHERE w.tenant_id=NEW.tenant_id AND w.session_id=NEW.session_id AND w.owner_id=NEW.owner_id
     AND w.id=json_extract(NEW.snapshot,'$.craft_workspace_seed.workspace_id')
     AND h.revision=CAST(json_extract(NEW.snapshot,'$.craft_workspace_seed.draft_revision') AS INTEGER)
     AND h.state=json_extract(NEW.snapshot,'$.craft_workspace_seed.state')
     AND COALESCE(h.source_run_id,'')=COALESCE(json_extract(NEW.snapshot,'$.craft_workspace_seed.source_run_id'),'')
     AND COALESCE(h.manifest_digest,'')=COALESCE(json_extract(NEW.snapshot,'$.craft_workspace_seed.manifest_digest'),'')
  ON CONFLICT(tenant_id,workspace_id,run_id) DO NOTHING;
END;

-- Keep the frozen predecessor serial: a later Run cannot be admitted while
-- an earlier terminal Run still owns a repairable draft capture.
CREATE TRIGGER trg_craft_capture_block_new_run BEFORE INSERT ON agent_runs
WHEN EXISTS (
  SELECT 1 FROM agent_runs prior
    JOIN craft_workspaces w ON w.tenant_id=prior.tenant_id AND w.session_id=prior.session_id AND w.owner_id=prior.owner_id
    LEFT JOIN craft_run_captures c ON c.tenant_id=prior.tenant_id AND c.workspace_id=w.id AND c.run_id=prior.run_id
   WHERE prior.tenant_id=NEW.tenant_id AND prior.session_id=NEW.session_id
     AND prior.run_id<>NEW.run_id AND prior.status IN ('succeeded','failed','canceled')
     AND json_type(prior.snapshot,'$.craft_workspace_seed')='object'
     AND w.id=json_extract(prior.snapshot,'$.craft_workspace_seed.workspace_id')
     AND (c.run_id IS NULL OR c.state <> 'advanced')
)
BEGIN SELECT RAISE(ABORT, 'Craft draft capture is unresolved'); END;
