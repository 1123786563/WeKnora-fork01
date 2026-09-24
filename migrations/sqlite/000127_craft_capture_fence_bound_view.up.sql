-- R4 Task3 收敛：终态 Run 的采集义务以“存在 bound RunView”为准。
-- 000123 的 block 触发器把“无 capture 行”一律视为未决义务，导致从未绑定
-- RunView（无容器输出、无物可采）的终态 Run 永久封锁同 Workspace 的新 Run
-- 提交，与 #120 “失败/无输出后继续使用其他材料”的旅程语义冲突。采集义务的
-- 权威定义与 enqueue 触发器一致：只有 state='bound' 的 RunView 才存在
-- 容器输出需要被封存；未绑定 RunView 的终态 Run 没有可修复的草稿捕获。
DROP TRIGGER IF EXISTS trg_craft_capture_block_new_run;
CREATE TRIGGER trg_craft_capture_block_new_run BEFORE INSERT ON agent_runs
WHEN EXISTS (
  SELECT 1 FROM agent_runs prior
    JOIN craft_workspaces w ON w.tenant_id=prior.tenant_id AND w.session_id=prior.session_id AND w.owner_id=prior.owner_id
    JOIN craft_run_views rv ON rv.tenant_id=prior.tenant_id AND rv.run_id=prior.run_id AND rv.state='bound'
    LEFT JOIN craft_run_captures c ON c.tenant_id=prior.tenant_id AND c.workspace_id=w.id AND c.run_id=prior.run_id
   WHERE prior.tenant_id=NEW.tenant_id AND prior.session_id=NEW.session_id
     AND prior.run_id<>NEW.run_id AND prior.status IN ('succeeded','failed','canceled')
     AND json_type(prior.snapshot,'$.craft_workspace_seed')='object'
     AND w.id=json_extract(prior.snapshot,'$.craft_workspace_seed.workspace_id')
     AND (c.run_id IS NULL OR c.state <> 'advanced')
)
BEGIN SELECT RAISE(ABORT, 'Craft draft capture is unresolved'); END;
