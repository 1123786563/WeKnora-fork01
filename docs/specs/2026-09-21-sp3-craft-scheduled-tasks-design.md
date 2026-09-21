# SP3 · Craft 定时任务（ScheduledTask CRUD+执行器）设计

> 状态：2026-09-21 定稿（调研双仓后 controller 依自治授权裁定，重大裁剪已标注）。对应 GAP-MATRIX C-23、ROADMAP SP3。

## 1. 目标与范围

把 Onyx 的 ScheduledTask（"用户自定义 cron + prompt，每次 fire 拉起一个 Craft 会话 run"）语义对齐进 WeKnora craft 域。

**范围内**：任务表+run 记录表、CRUD/run-now/runs 历史 API、30s dispatcher+asynq 执行器、SKIP_IF_RUNNING 重叠策略、stuck sweeper、craft 前端 tasks 子路由族。
**范围外（裁剪裁定 R1-R3，登记台账）**：
- **R1 预授权目标裁掉**：Onyx 的预授权是"外部应用/MCP 的 ASK 门控动作免审批"（gated_app 体系）；WeKnora craft 无此体系（C02 是 approve-once 交互决定），等 C-20/22 外部应用桥落地后再立项对齐。
- **R2 per-task 预算聚合不做**：每 run 经 StartRun 自动 Admit 一个 BudgetGrant（既有语义天然生效）；"每任务/每日调用上限"聚合层留待真实需求。
- **R3 无重试**：run FAILED 即终态（Onyx V1 同款），下次 cron 自然再试。

## 2. 数据模型（新迁移双轨）

```sql
craft_scheduled_tasks (
  id VARCHAR(36) PK, tenant_id BIGINT NOT NULL, owner_id VARCHAR(512) NOT NULL,
  name VARCHAR(128) NOT NULL, prompt TEXT NOT NULL,
  cron_expression VARCHAR(64) NOT NULL,          -- 规范 5 字段；三种 UI 模式保存时编译成它（后端编译）
  editor_mode VARCHAR(16) NOT NULL DEFAULT 'advanced',  -- interval|daily|advanced（UI 提示，cron 才是真源）
  status VARCHAR(16) NOT NULL DEFAULT 'active',  -- active|paused
  next_run_at TIMESTAMPTZ NULL,                  -- 暂停时 NULL；fire/改期重算；UTC
  last_run_at TIMESTAMPTZ NULL,
  deleted_at TIMESTAMPTZ NULL,                   -- 软删（run 历史可看）
  created_at/updated_at
); 索引 ix_craft_scheduled_dispatch(status, deleted_at, next_run_at)、tenant+owner 索引

craft_scheduled_task_runs (
  id VARCHAR(36) PK, task_id VARCHAR(36) NOT NULL REFERENCES ... ON DELETE CASCADE,
  session_id VARCHAR(36) NULL,                   -- FK sessions 逻辑关联（SET NULL 语义：会话删了 run 历史仍在——物理 FK 不加，照 craft_lifecycle 先例）
  status VARCHAR(24) NOT NULL,                   -- queued|running|succeeded|failed|skipped|awaiting_interaction
  trigger_source VARCHAR(16) NOT NULL,           -- scheduled|manual_run_now
  skip_reason VARCHAR(32) NULL,                  -- owner_craft_disabled|prior_in_flight
  error_class VARCHAR(32) NULL, error_detail TEXT NULL,
  started_at/finished_at TIMESTAMPTZ NULL,
  summary TEXT NULL                              -- 最终消息 ~120 字摘要
); task_id+started_at 索引（run 历史分页）
```

Go 类型 `internal/types/craft_scheduled.go`：两 struct+六 RunStatus 常量+两 Trigger 常量+ErrorClass 开放集（字符串，仅文档封闭词表）。cron 校验用 robfig/cron 的 parser（仓库已有依赖）。

## 3. API（`/api/v1/craft/scheduled-tasks`，Viewer+ apiKeyChat+属主校验照 craft 全域模式）

| 端点 | 语义 |
|---|---|
| GET `` | 本人任务 newest-first（无分页，V1 量小——Onyx 同款） |
| POST `` | 创建（editor_payload 三模式后端编译为 cron+next_run_at；可选 run_immediately=true 插 manual QUEUED run 入队，不动 next_run_at） |
| GET `/:id` | 详情+next 3 fires 预览（ACTIVE 时算） |
| PATCH `/:id` | 部分更新（editor_mode/prompt/成对；重算 next_run_at） |
| DELETE `/:id` | 软删幂等 204 |
| POST `/:id/run-now` | 手动触发（PAUSED 也允许；不动 next_run_at） |
| GET `/:id/runs` | run 历史 keyset 分页（before=started_at ISO，limit 1..100 默认 50） |

属主校验：所有读写在 repo 层 `WHERE owner_id=? AND deleted_at IS NULL`（错任务 404 不泄露）。cron 表达式限制：分钟级最小粒度（Onyx 契约：30s dispatch 支持 5 字段 cron 的分钟精度）。

## 4. 调度与执行

**Dispatcher（30s ticker，照 cleanup.go StartPeriodicSweep 模式）**：
1. 扫 `status=active AND deleted_at IS NULL AND next_run_at<=now`（批量 50）
2. **原子认领防多实例双发**：`UPDATE craft_scheduled_tasks SET next_run_at=<下次> WHERE id=? AND next_run_at=<旧值>`（CAS；0 行=别人已认领跳过）——PG/SQLite 均适用，SKIP LOCKED 的等价简化
3. 前次 run 仍在 queued/running→插 SKIPPED(prior_in_flight) 行（可见性）不入队
4. 否则插 QUEUED run 行→asynq 入队 `TypeCraftScheduledRun`（新 task type+QueueSync 队列【复用现有队列避免新 worker 部署面】+MaxRetry 1+TaskID 确定性 `craftsched:<runID>`）
5. next_run_at 计算：robfig Schedule.Next(now)

**Executor（service 方法 ProcessScheduledRun）**：
1. 幂等护栏（run 非 queued 直接 nil）
2. 属主 craft 会话：`craftScope` 内部构造（TenantID/OwnerID 直填任务属主——**R4 身份裁定：属主身份+服务内部调用**，不走 API key 合成用户）；建 craft session（kind 标记 scheduled——craft_sessions 若 kind 是文本列直接用新值，grep 确认后定）
3. QUEUED→RUNNING→StartRun（prompt 落 turn 0；复用 durable worker/幂等/预算 Admit/30min Deadline 全链）
4. 完成事件回流：craft run 终态回调处（grep StartRun 的完成通知路径）更新 run 行——succeeded+summary（最终消息截断 120 字）或 failed(error_class)
5. 交互决定出现（waiting_user）→ awaiting_interaction 终态（显示级，不再跑）

**Stuck sweeper**（并入 craft lifecycle sweep 或独立 1h ticker）：queued>15min 或 running>45min→failed(stuck)。

## 5. 前端（craft 自管路由族扩展）

`parseCraftRoute` 加 `/craft/tasks`（列表：卡片 name/cron 人话/状态/下次运行/最近 run 状态）+ `/craft/tasks/new` + `/craft/tasks/:id`（详情+run 历史表+编辑+run-now+暂停/恢复+删除确认）。表单三模式（interval 分钟/每天时刻/高级 cron——照 Onyx ScheduleTaskForm 交互）。i18n 5 locale。api-client `craftScheduled` 域。

## 6. 验收

- CRUD/run-now/runs 分页全绿（属主隔离/cron 编译三模式/软删幂等）
- dispatcher：CAS 认领并发安全（双实例模拟）、skip 策略、next_run_at 推进
- executor：全链（建会话 kind=scheduled→StartRun→终态回写 run 行 summary）、幂等重入、awaiting_interaction
- stuck sweeper 两窗
- 前端四页+表单三模式+i18n
- 冒烟：造 1 分钟间隔任务看两次 fire（共享栈尽力）

## 7. 遗留登记

R1 预授权目标（等外部应用桥）；R2 per-task 预算聚合；R3 无重试；dispatch 30s 粒度（分钟级 cron 下界）；run 历史无清理策略（V1 跟随任务软删）。
