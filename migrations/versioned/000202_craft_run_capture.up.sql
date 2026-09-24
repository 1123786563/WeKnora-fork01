CREATE TABLE craft_run_captures (
    tenant_id BIGINT NOT NULL,
    workspace_id VARCHAR(64) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    generation VARCHAR(64) NOT NULL,
    predecessor_revision BIGINT NOT NULL CHECK (predecessor_revision >= 0),
    predecessor_state VARCHAR(16) NOT NULL CHECK (predecessor_state IN ('empty','selected')),
    predecessor_run_id VARCHAR(64) NOT NULL DEFAULT '',
    predecessor_digest VARCHAR(64) NOT NULL DEFAULT '',
    state VARCHAR(16) NOT NULL CHECK (state IN ('pending','capturing','sealed','advanced','blocked')),
    manifest_digest VARCHAR(64) NOT NULL DEFAULT '',
    draft_revision BIGINT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, workspace_id, run_id),
    FOREIGN KEY (tenant_id, workspace_id) REFERENCES craft_workspaces(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);
CREATE INDEX idx_craft_run_captures_recovery ON craft_run_captures(state, updated_at);

CREATE TABLE craft_run_capture_files (
    tenant_id BIGINT NOT NULL,
    workspace_id VARCHAR(64) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    path TEXT NOT NULL,
    object_ref TEXT NOT NULL,
    sha256 CHAR(64) NOT NULL,
    bytes BIGINT NOT NULL CHECK (bytes >= 0),
    mime TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (tenant_id, workspace_id, run_id, path),
    FOREIGN KEY (tenant_id, workspace_id, run_id)
        REFERENCES craft_run_captures(tenant_id, workspace_id, run_id) ON DELETE CASCADE
);

CREATE FUNCTION craft_capture_reject_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_TABLE_NAME='craft_run_captures' THEN
      IF TG_OP='UPDATE' AND (NEW.tenant_id<>OLD.tenant_id OR NEW.workspace_id<>OLD.workspace_id OR NEW.run_id<>OLD.run_id OR NEW.owner_id<>OLD.owner_id OR NEW.session_id<>OLD.session_id OR NEW.generation<>OLD.generation OR NEW.predecessor_revision<>OLD.predecessor_revision OR NEW.predecessor_state<>OLD.predecessor_state OR NEW.predecessor_run_id<>OLD.predecessor_run_id OR NEW.predecessor_digest<>OLD.predecessor_digest) THEN
      RAISE EXCEPTION 'Craft capture identity is immutable';
    END IF;
    IF TG_OP='UPDATE' AND OLD.state='advanced' THEN
      RAISE EXCEPTION 'Advanced Craft capture is immutable';
    END IF;
    IF TG_OP='UPDATE' AND OLD.state IN ('sealed','advanced') AND NEW.manifest_digest<>OLD.manifest_digest THEN
      RAISE EXCEPTION 'Sealed Craft capture digest is immutable';
    END IF;
    RETURN NEW;
  END IF;
  IF TG_OP='INSERT' AND EXISTS (SELECT 1 FROM craft_run_captures c WHERE c.tenant_id=NEW.tenant_id AND c.workspace_id=NEW.workspace_id AND c.run_id=NEW.run_id AND c.state IN ('sealed','advanced')) THEN
    RAISE EXCEPTION 'Sealed Craft capture files are immutable';
  END IF;
  IF TG_OP<>'INSERT' AND EXISTS (SELECT 1 FROM craft_run_captures c WHERE c.tenant_id=OLD.tenant_id AND c.workspace_id=OLD.workspace_id AND c.run_id=OLD.run_id AND c.state IN ('sealed','advanced')) THEN
    RAISE EXCEPTION 'Sealed Craft capture files are immutable';
  END IF;
  IF TG_OP='DELETE' THEN RETURN OLD; END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_craft_capture_identity_immutable BEFORE UPDATE ON craft_run_captures
  FOR EACH ROW EXECUTE FUNCTION craft_capture_reject_mutation();
CREATE TRIGGER trg_craft_capture_file_immutable BEFORE INSERT OR UPDATE OR DELETE ON craft_run_capture_files
  FOR EACH ROW EXECUTE FUNCTION craft_capture_reject_mutation();

CREATE FUNCTION craft_capture_terminal_enqueue() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.status IN ('succeeded','failed','canceled') AND OLD.status NOT IN ('succeeded','failed','canceled') THEN
    INSERT INTO craft_run_captures
      (tenant_id, workspace_id, run_id, owner_id, session_id, generation,
       predecessor_revision, predecessor_state, predecessor_run_id, predecessor_digest, state)
    SELECT NEW.tenant_id, w.id, NEW.run_id, NEW.owner_id, NEW.session_id, rv.generation,
           CAST(NEW.snapshot->'craft_workspace_seed'->>'draft_revision' AS BIGINT),
           NEW.snapshot->'craft_workspace_seed'->>'state',
           COALESCE(NEW.snapshot->'craft_workspace_seed'->>'source_run_id',''),
           COALESCE(NEW.snapshot->'craft_workspace_seed'->>'manifest_digest',''), 'pending'
      FROM craft_workspaces w
      JOIN craft_run_views rv ON rv.tenant_id=NEW.tenant_id AND rv.run_id=NEW.run_id AND rv.state='bound'
      JOIN craft_workspace_draft_heads h ON h.tenant_id=w.tenant_id AND h.workspace_id=w.id
     WHERE w.tenant_id=NEW.tenant_id AND w.session_id=NEW.session_id AND w.owner_id=NEW.owner_id
       AND jsonb_typeof(NEW.snapshot->'craft_workspace_seed')='object'
       AND w.id=(NEW.snapshot->'craft_workspace_seed'->>'workspace_id')
       AND h.revision=CAST(NEW.snapshot->'craft_workspace_seed'->>'draft_revision' AS BIGINT)
       AND h.state=(NEW.snapshot->'craft_workspace_seed'->>'state')
       AND COALESCE(h.source_run_id,'')=COALESCE(NEW.snapshot->'craft_workspace_seed'->>'source_run_id','')
       AND COALESCE(h.manifest_digest,'')=COALESCE(NEW.snapshot->'craft_workspace_seed'->>'manifest_digest','')
    ON CONFLICT (tenant_id,workspace_id,run_id) DO NOTHING;
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_craft_capture_terminal_enqueue AFTER UPDATE OF status ON agent_runs
  FOR EACH ROW EXECUTE FUNCTION craft_capture_terminal_enqueue();

CREATE FUNCTION craft_capture_block_new_run() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM agent_runs prior
      JOIN craft_workspaces w ON w.tenant_id=prior.tenant_id AND w.session_id=prior.session_id AND w.owner_id=prior.owner_id
      LEFT JOIN craft_run_captures c ON c.tenant_id=prior.tenant_id AND c.workspace_id=w.id AND c.run_id=prior.run_id
      WHERE prior.tenant_id=NEW.tenant_id AND prior.session_id=NEW.session_id
        AND prior.run_id<>NEW.run_id AND prior.status IN ('succeeded','failed','canceled')
        AND jsonb_typeof(prior.snapshot->'craft_workspace_seed')='object'
        AND w.id=(prior.snapshot->'craft_workspace_seed'->>'workspace_id')
        AND (c.run_id IS NULL OR c.state <> 'advanced')) THEN
    RAISE EXCEPTION 'Craft draft capture is unresolved';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_craft_capture_block_new_run BEFORE INSERT ON agent_runs
  FOR EACH ROW EXECUTE FUNCTION craft_capture_block_new_run();
