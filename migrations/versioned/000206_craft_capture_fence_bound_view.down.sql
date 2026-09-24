-- 回滚到 000202 的原始 block 触发器（无 bound RunView 的终态 Run 也封锁新 Run）。
DROP FUNCTION IF EXISTS craft_capture_block_new_run();
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
DROP TRIGGER IF EXISTS trg_craft_capture_block_new_run ON agent_runs;
CREATE TRIGGER trg_craft_capture_block_new_run BEFORE INSERT ON agent_runs
  FOR EACH ROW EXECUTE FUNCTION craft_capture_block_new_run();
