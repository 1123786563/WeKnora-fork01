# SP2-a · Connectors 治理第一阶段（sync 心跳/取消 + 删源级联）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** sync_logs 获得 attempt 级心跳/取消能力（修掉两个现存判活 bug），删源支持双选级联清理同步文档。

**Architecture:** 心跳复用 Checkpoint 的 UpdateResult 回写点+新列；取消分两层（运行中=sync_logs.cancel_requested 协作式在批次边界退出，排队中=TaskInspector 硬取消）；级联删除走"同步删源+新 asynq purge 任务分批复用 executeKnowledgeDelete 管道"，先取消后删除的顺序在同一 service 方法内保证。

**Tech Stack:** Go 1.26（gin/dig/GORM/asynq）、SQL 迁移（PG versioned + sqlite 双轨）、React 19（apps/web）、Vue 3（frontend/）、pnpm workspace。

**Spec:** `docs/superpowers/specs/2026-09-19-sp2-connectors-governance-design.md` §3（子项1）+ §4（子项3）——执行者需同时读 spec 与本计划。

## Global Constraints

- 迁移双轨：PG `migrations/versioned/` + SQLite `migrations/sqlite/`，各带 up/down；**迁移号提交时重查当下占用再定**（多 lane 撞号教训，SP1 曾 000158/000079 双撞）
- 新列全部 nullable / default，迁移可逆（带 down）
- 取消优先协作式（checkpoint 边界退出+cursor 保留）；硬取消仅用于排队任务
- 级联删除作用域硬限定 `(tenant, kb, datasource_id)` 三元组；先取消后删除顺序不可换
- 默认不勾选同删 = 完全向后兼容（不带 `purge_documents` 参数行为与现状逐字节一致）
- 删除文档必须软删+硬删两步（tombstone 契约，`repository/knowledge.go:841-858`）
- 测试命令：Go `go build ./...` + `go test <目标包> -count=1`；前端 `pnpm test:web` / `pnpm typecheck:web`；Vue `cd frontend && npm run type-check && npm run check-i18n`
- 提交纪律：每 Task 结束提交一次；`git status --porcelain` 核对暂存区只含本 Task 文件
- 预存失败基线：Go 全量在 main 即有约 18-19 个测试级失败（execution_targets schema 漂移族等）；前端 test:web 38 / test:shared 3 预存——验收以"零新增失败"为准，stash 或 base 对比甄别

## 已核实的代码事实（执行者直接依赖）

- `SyncLog` 模型 `internal/types/datasource.go:144-198`（无心跳列）；建表 `migrations/versioned/000029_datasource_tables.up.sql:34-57` / `migrations/sqlite/000000_init.up.sql:799-814`
- 流式回写点 `streamSyncHandler.Checkpoint` `internal/application/service/datasource_service.go:1183-1226`（已刷 6 计数+updated_at）；批量路径循环内联在 `ProcessSync`（:705）的 :898-901，无分页无取消检查
- 启动兜底 `internal/container/reset_pending_tasks.go:123-136` + `stuckSyncLogQuery` :162-169（分布式模式判 `started_at < now-30min`，常量 :14 `resetPendingStaleWindow`）；lite 模式无条件 reset
- 防重叠 `HasRunningSync` `internal/datasource/scheduler.go:167` → `datasource_repo.go:306-319`（只数 status=running）
- TaskInspector：`taskTypesForKnowledgeCancel` `internal/router/task_inspector.go:87-95`（不含 `types.TypeDataSourceSync`）；`CancelTasksForKnowledgeBase(ctx, kbID, knowledgeIDs, dataSourceIDs)` :130-160（dataSourceIDs 匹配 payload `data_source_id`，matcher :1054-1056；**该函数不受白名单限制**——白名单只限制 `matchesKnowledge`）；接口 `interfaces.TaskInspector`
- `DataSourceService` 结构体 :29-47 **无 inspector 字段**；有 setter 先例 `SetSyncExecution` :87
- 知识删除管道：handler 层 `enqueueKnowledgeListDelete` `internal/handler/knowledge.go:184-207`（asynq `TypeKnowledgeListDelete`，QueueMaintenance，MaxRetry 3，Timeout 2h）；service 入口 `DeleteKnowledgeList` `knowledge_delete.go:418-425`；硬删 `HardDeleteKnowledgeList` `repository/knowledge.go:850-858`；datasource 侧组合用法先例 `datasource_service.go:1649-1655`
- `app_datasource_bindings` 行模型 `datasource_repo.go:156-169`（无 DeleteByDataSource；现有方法 Find/Save :174-223）
- auto-tag：名 = `ds.Name` 原样（`resolveAutoTagIDs` `datasource_service.go:952-961`）；relations 表 `knowledge_tag_relations`（`types/tag.go:69-78`）
- 表达式索引样板 `migrations/versioned/000076_knowledge_metadata_external_id_index.up.sql:20-22`：`CREATE INDEX ... ON knowledges (knowledge_base_id, (metadata->>'external_id') text_pattern_ops) WHERE deleted_at IS NULL`
- 批量删除入队上限 200（`handler/knowledge.go:1323-1327`）
- `DataSourceSyncPayload` `types/datasource.go:495-516`；`DataSource` 与 KB 1:1（KnowledgeBaseID）
- 测试 fixture 样板 `internal/application/service/datasource_delete_sqlite_test.go:18-96`（sqlite 真库+AutoMigrate+字段字面量构造 service）
- 前端：React remove() `apps/web/src/data-sources/DataSourcesPage.tsx:209`（window.confirm）；`DataSourceSyncLog` 类型 `packages/api-client/src/datasource.ts:33-46`；Vue 入口 `frontend/src/views/knowledge/settings/DataSourceSettings.vue:88-96` + API `frontend/src/api/datasource/index.ts:109-111`

---

### Task 1: sync_logs 三列迁移 + 模型/仓库扩展

**Files:**
- Create: `migrations/versioned/0000XX_sync_logs_lifecycle.up.sql` / `.down.sql`（号=提交时 PG 轨下一可用）
- Create: `migrations/sqlite/0000YY_sync_logs_lifecycle.up.sql` / `.down.sql`（号=提交时 sqlite 轨下一可用）
- Modify: `internal/types/datasource.go:144-198`（SyncLog 加三字段）
- Modify: `internal/application/repository/datasource_repo.go`（SyncLogRepository 加三方法）
- Modify: `internal/types/datasource.go`（`SyncLogStatusPending = "pending"` 常量收编，替换 `datasource_repo.go:376` 裸字符串）
- Test: `internal/application/repository/datasource_repo_synclog_test.go`（新建）

**Interfaces:**
- Produces: `SyncLog.HeartbeatAt *time.Time` / `AsynqTaskID string` / `CancelRequested bool`（json: `heartbeat_at`/`asynq_task_id`/`cancel_requested`）；repo 方法：
  - `UpdateHeartbeat(ctx, id string, at time.Time) error`
  - `UpdateAsynqTaskID(ctx, id, taskID string) error`
  - `RequestCancel(ctx, id string) error`（置 cancel_requested=true，仅 running 行生效，返回受影响行数语义：非 running 返回 nil 不报错）
  - `HasRunningSync` 改造后语义（Task 3 消费）
- 迁移 DDL（PG up）：

```sql
ALTER TABLE sync_logs ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMP NULL;
ALTER TABLE sync_logs ADD COLUMN IF NOT EXISTS asynq_task_id VARCHAR(64) NULL;
ALTER TABLE sync_logs ADD COLUMN IF NOT EXISTS cancel_requested BOOLEAN NOT NULL DEFAULT FALSE;
```
（sqlite 同语义 TEXT/DATETIME/INTEGER 风格照该轨惯例；down 为三条对应 DROP/还原。）

- [ ] **Step 1: 写失败测试**（sqlite AutoMigrate 库：建 SyncLog → UpdateHeartbeat 后重读断言 HeartbeatAt 非空且 UpdatedAt 刷新；RequestCancel 对 running 行生效、对 success 行不动；UpdateAsynqTaskID 落库）
- [ ] **Step 2: 跑测试确认失败** — `go test ./internal/application/repository/ -run TestSyncLogLifecycle -count=1`，FAIL：方法未定义
- [ ] **Step 3: 写迁移 + 模型字段 + 三个 repo 方法**（RequestCancel 用 `Where("status = ?", running).Update("cancel_requested", true)`；GORM 零值 bool 用 Update 单列避免 map 的零值陷阱）
- [ ] **Step 4: 跑测试确认通过** + 全包回归 `go test ./internal/application/repository/ -count=1`
- [ ] **Step 5: 提交** `feat(datasource): sync_logs lifecycle columns — heartbeat, asynq task id, cancel flag`

---

### Task 2: 心跳回写（流式 + 批量两路径）

**Files:**
- Modify: `internal/application/service/datasource_service.go:1183-1226`（Checkpoint 顺带刷心跳）
- Modify: `internal/application/service/datasource_service.go:898-901`（批量循环边界回写）
- Test: `internal/application/service/datasource_sync_heartbeat_test.go`（新建）

**Interfaces:**
- Consumes: Task 1 的 `UpdateHeartbeat`
- Produces: 心跳节流助手 `maybeHeartbeat(ctx, syncLogRepo, logID, lastBeat *time.Time)`（每 ≥30 秒刷一次，内存节流防每项一写）——放 datasource_service.go 包内私有函数，Task 4 的取消检查复用同一节流点

- [ ] **Step 1: 写失败测试**：fixture 仿 `datasource_delete_sqlite_test.go` 建库，构造 `streamSyncHandler` 最小形态（svc+ds+syncLog+result），调 `Checkpoint` 两次断言 `heartbeat_at` 已写；批量路径用 ProcessSync 太重——直接测 `maybeHeartbeat`（首次调用即写、30 秒内第二次不写、超窗再写）+ 在 Checkpoint 集成点用轻量断言（Checkpoint 后 DB 行 heartbeat_at 非空）
- [ ] **Step 2: 确认失败** — `go test ./internal/application/service/ -run TestSyncHeartbeat -count=1` FAIL
- [ ] **Step 3: 实现**：Checkpoint 在 `UpdateResult` 调用后追加 `h.svc.syncLogRepo.UpdateHeartbeat(ctx, h.syncLog.ID, time.Now().UTC())`（失败仅 Warn，best-effort 同款注释）；批量循环改为每项后 `maybeHeartbeat(...)` 且**每 20 项**做一次中间 `UpdateResult`（计数镜像同 Checkpoint 的 6 字段赋值）
- [ ] **Step 4: 确认通过** + service 包回归（预存失败集外零新增）
- [ ] **Step 5: 提交** `feat(datasource): sync heartbeat on both streaming and batch paths`

---

### Task 3: 判活修复（两个现存 bug）

**Files:**
- Modify: `internal/container/reset_pending_tasks.go:14,123-136,162-169`（判活源切 COALESCE + 窗口 2h15m）
- Modify: `internal/application/repository/datasource_repo.go:306-319`（HasRunningSync 排除超窗）
- Test: `internal/container/reset_pending_tasks_test.go`（若无既有文件则新建；有则追加）

**Interfaces:**
- Consumes: Task 1 的 `heartbeat_at` 列
- Produces: `stallWindow = 2*time.Hour + 15*time.Minute`（两处共用常量，放 `internal/types/datasource.go` 或 repo 包导出——定在 `types` 包 `SyncStallWindow`）；HasRunningSync 新语义：`status='running' AND COALESCE(heartbeat_at, started_at) > now - SyncStallWindow`

- [ ] **Step 1: 写失败测试**（两个 bug 的回归测试，sqlite 库）：
  - bug①：running 行 `started_at = now-40min`、`heartbeat_at = now-1min`（活跃长任务）→ `resetPendingStaleTasks`（分布式形态）**不**置 failed；`heartbeat_at = now-3h` 的行置 failed
  - bug②：`HasRunningSync` 在 running 行心跳超窗（`heartbeat_at = now-3h`）时返回 false——不再永久阻塞调度
- [ ] **Step 2: 确认失败**（现实现两个测试都红：①误杀活跃行 ②超窗仍算 running）
- [ ] **Step 3: 实现**：`stuckSyncLogQuery` 改 `Where("COALESCE(heartbeat_at, started_at) < ?", staleCutoff)`；`resetPendingStaleWindow` 值改 `types.SyncStallWindow`；HasRunningSync 加同款 COALESCE 条件；lite 模式无条件 reset 语义保留（单机进程内任务必死）
- [ ] **Step 4: 确认通过** + container/repository 包回归
- [ ] **Step 5: 提交** `fix(datasource): liveness from heartbeat — no more killing live long tasks or deadlocking the scheduler`

---

### Task 4: 协作式取消（运行中）

**Files:**
- Modify: `internal/application/service/datasource_service.go`（Checkpoint 与批量循环边界查 cancel_requested；优雅退出）
- Modify: `internal/handler/datasource.go`（新 handler 方法 CancelSyncLog）
- Modify: `internal/router/routes_infra.go`（`POST /:id/logs/:log_id/cancel`，Admin）
- Test: `internal/application/service/datasource_sync_cancel_test.go`（新建）+ handler 测试

**Interfaces:**
- Consumes: Task 1 的 `RequestCancel`/`cancel_requested`
- Produces: service 方法 `CancelSyncLog(ctx, tenantID uint64, dsID, logID string) error`（归属校验：log 属于该 ds+tenant 且 status=running，否则 `ErrNotFound`；置标记）；同步循环消费点：`checkCancelRequested(ctx, syncLogRepo, logID) bool`（每 checkpoint/每 20 项节流查询一次，与 maybeHeartbeat 同节奏）；退出路径：status=canceled、`error_message = "canceled by user"`、finished_at=now、**cursor 不动**（断点续传语义：UpdateResult 后直接 return nil，不触发失败路径）
- handler：`POST /api/v1/datasource/:id/logs/:log_id/cancel`，Admin 权限，成功 202 `{"status":"cancel_requested"}`

- [ ] **Step 1: 写失败测试**：service 层（sqlite）：running log → RequestCancel → 模拟 Checkpoint/批量边界 → 断言返回后 log=canceled 且 error_message="canceled by user"；归属校验（错 ds 的 log 404）；已完成 log 的 RequestCancel 是 no-op。handler 层：路由参数→service 调用→202
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现**：Checkpoint 在心跳节流点先查 `checkCancelRequested`（true 则更新 log 终态并返回哨兵错误 `errSyncCanceled`，`processSyncStreaming` 捕获后转 canceled 成功返回——**不**让 asynq 视为失败重试）；批量循环同点检查；ProcessSync 顶部对 `errSyncCanceled` 归一化
- [ ] **Step 4: 确认通过** + service/handler/router 回归
- [ ] **Step 5: 提交** `feat(datasource): cooperative cancel for running syncs at checkpoint boundaries`

---

### Task 5: 排队任务硬取消 + Pause/Delete 增强

**Files:**
- Modify: `internal/router/task_inspector.go:87-95`（`taskTypesForKnowledgeCancel` 加 `types.TypeDataSourceSync`）
- Modify: `internal/application/service/datasource_service.go`（inspector 注入 setter `SetTaskInspector`；DeleteDataSource/PauseDataSource 调用取消；`ErrTaskIDConflict` 分支错误信息区分）
- Modify: `internal/container/container.go`（装配处注入 inspector；找到 DataSourceService 构造后的装配点追加 `svc.SetTaskInspector(inspector)`——inspector 的 dig provider 名以 container 现状为准，grep `TaskInspector`）
- Modify: `internal/application/service/datasource_service.go:608-634`（ManualSync 入队后 `UpdateAsynqTaskID`）+ `internal/datasource/scheduler.go:208-215`（调度入队同款）
- Test: `internal/application/service/datasource_cancel_enqueue_test.go`（新建）+ task_inspector 测试追加

**Interfaces:**
- Consumes: Task 1 的 `UpdateAsynqTaskID`；`interfaces.TaskInspector.CancelTasksForKnowledgeBase(ctx, kbID string, knowledgeIDs, dataSourceIDs []string) (int, int, error)`（注意 kbID 传 ds 所属 KB——payload 里 dssync 无 knowledge_base_id，matcher 靠 dataSourceIDs 命中；kbID 传空串时行为以 matcher 实现为准，实施时打开 `matchesKnowledgeBase` 核对空 kbID 是否跳过过滤）
- Produces: `DeleteDataSource(ctx, id string) error`（签名不变，行为增强：软删→摘 cron→**inspector 取消排队**→CancelPendingByDataSource→审计）；`PauseDataSource` 增强为也取消运行+排队；`scheduler.go:217-225` ErrTaskIDConflict 的置 canceled 改为 `error_message = "skipped: another sync already queued"`（status 仍 canceled——兼容现有语义，但可区分）；fake inspector 模式（测试）

- [ ] **Step 1: 写失败测试**：fake TaskInspector（记录调用）；DeleteDataSource 断言 inspector 以 `dataSourceIDs=[id]` 被调用且在文档操作前；Pause 同款；ManualSync 后 SyncLog.AsynqTaskID 非空；ErrTaskIDConflict 分支的错误信息断言
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现**（inspector nil 时降级为现状——noop，Warn 一条；空 kbID 传法按 Step 1 核对结果定，在实现注释里写明依据）
- [ ] **Step 4: 确认通过** + 回归（注意 task_inspector 既有测试对集合内容的断言若有需同步）
- [ ] **Step 5: 提交** `feat(datasource): hard-cancel queued syncs on delete/pause + asynq task id linkage`

---

### Task 6: React 取消按钮 + api-client

**Files:**
- Modify: `packages/api-client/src/datasource.ts`（`DataSourceSyncLog` 加 `heartbeat_at?: string | null; cancel_requested?: boolean`；`dataSources` 加 `cancelSyncLog(id, logId)`）
- Modify: `apps/web/src/data-sources/DataSourcesPage.tsx`（运行中行的取消按钮）
- Modify: `apps/web/src/data-sources/log-state.ts`（如需：canceled 状态 pill 键已存在则不动）
- Test: `packages/api-client/src/datasource.test.ts`（追加，文件已存在则并入）+ apps/web 现有 data-sources 测试模式追加

**Interfaces:**
- Consumes: Task 4 的 `POST /datasource/:id/logs/:log_id/cancel`
- Produces: `cancelSyncLog(id: string, logId: string): Promise<void>`；UI：`latest_sync_log.status === 'running'` 的卡片显示取消按钮（`t('dataSource.cancelSync')`），点击→cancelSyncLog→继续既有 3 秒轮询自然收敛

- [ ] **Step 1: 写失败测试**（api-client：路径/方法断言；页面：running 态渲染取消按钮+点击调用——harness 仿 data-sources 既有测试）
- [ ] **Step 2: 确认失败** — `pnpm test:web` / `pnpm test:shared`
- [ ] **Step 3: 实现** + i18n 键 `dataSource.cancelSync`（中：取消同步 / en: Cancel sync；6 locale：zh-CN/en-US/ja-JP/ko-KR/ru-RU——ja/ko/ru 用英文文案，apps/web/src/i18n.ts 与 packages/i18n 按现有组织补）
- [ ] **Step 4: 确认通过** + `pnpm typecheck:web`
- [ ] **Step 5: 提交** `feat(data-sources): cancel button for running syncs (React + api-client)`

---

### Task 7: knowledge 按 datasource_id 查询/计数 + 索引迁移

**Files:**
- Create: `migrations/versioned/0000XX_knowledge_datasource_id_index.up.sql` / `.down.sql`（号=提交时下一可用）
- Create: `migrations/sqlite/0000YY_knowledge_datasource_id_index.up.sql` / `.down.sql`
- Modify: `internal/application/repository/knowledge.go`（新方法）+ `internal/types/interfaces/knowledge.go`（接口声明）
- Test: `internal/application/repository/knowledge_datasource_test.go`（新建）

**Interfaces:**
- Produces（repo 接口新增两方法）：
  - `FindKnowledgeIDsByDataSourceID(ctx, tenantID uint64, kbID, dsID string, limit int) ([]string, error)`（`metadata->>'datasource_id' = ?` 等值 + `deleted_at IS NULL`，ORDER BY id 稳定分批；limit<=0 默认 200；表达式必须 SQL 字面量内联——仿 `FindByDataSourceExternalID` :820-839 写法）
  - `CountKnowledgeByDataSourceID(ctx, tenantID uint64, kbID, dsID string) (int64, error)`
- 索引 DDL（PG）：

```sql
CREATE INDEX IF NOT EXISTS idx_knowledges_kb_metadata_datasource_id
    ON knowledges (knowledge_base_id, (metadata->>'datasource_id') text_pattern_ops)
    WHERE deleted_at IS NULL;
```
（sqlite 轨：sqlite 无 partial 表达式索引的同款能力——sqlite 支持表达式索引但无 text_pattern_ops；按该轨惯例写等价 `CREATE INDEX IF NOT EXISTS ... ON knowledges (knowledge_base_id, json_extract(metadata,'$.datasource_id'))` 或全列普通索引，以 sqlite 迁移既有风格为准并在注释说明查询不走索引也可接受——量级以 KB 为界）

- [ ] **Step 1: 写失败测试**（sqlite 库造 3 篇含 `metadata.datasource_id` 的文档+1 篇其他源+1 篇软删：Find 返回正确 3 个 ID、Count=3、limit 分批稳定、三元组隔离——换 kbID/dsID 查不到）
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现迁移+两方法**（接口与实现同步；sqlite 的 JSON 谓词用 `json_extract` 兼容——**注意**：现有 `FindByDataSourceExternalID` 在 sqlite 测试里用 `metadata->>'datasource_id'` 也能跑（gorm sqlite 驱动兼容 ->> 操作符？以 fixture 实测为准——测试跑不过就把谓词改双写：PG 用 ->>、sqlite 用 json_extract，按 `db.Dialector.Name()` 分支，仓库已有此类先例则照抄））
- [ ] **Step 4: 确认通过**
- [ ] **Step 5: 提交** `feat(knowledge): list/count knowledge by datasource_id with expression index`

---

### Task 8: 级联删除 service + purge 任务

**Files:**
- Modify: `internal/types/task.go`（`TypeDataSourcePurge = "datasource:purge"` 常量 + Payload 类型 `DataSourcePurgePayload{TenantID, KnowledgeBaseID, DataSourceID, TagName string}`；QueueMaintenance 注册检查——若队列 worker 按类型注册 handler，在 `internal/handler` 或 worker 注册处加 mux 条目，位置 grep `TypeKnowledgeListDelete` 的注册点照抄）
- Modify: `internal/application/service/datasource_service.go`（`DeleteDataSource` 加 purge 分支 + `PurgeDataSourceDocuments(ctx, payload)` worker 方法）
- Modify: `internal/application/repository/datasource_repo.go`（`DeleteAppDataSourceBindingsByDataSource(ctx, dsID string) error`）
- Modify: `internal/application/repository/datasource_repo.go` 或 tag repo（孤儿 auto-tag 清理：删除名为 ds.Name 且无 relations 的 tag——tag repo 方法 `DeleteOrphanTagByName(ctx, kbID, name string) error`，实现为两条 SQL：先查 relation count 为 0 的同名 tag 再删，放 `internal/application/repository/` tag 相关文件）
- Test: `internal/application/service/datasource_purge_test.go`（新建）

**Interfaces:**
- Consumes: Task 5 增强后的 DeleteDataSource（先取消）、Task 7 的 FindKnowledgeIDsByDataSourceID、`knowledgeService.DeleteKnowledgeList(ctx, ids)`、`repo.HardDeleteKnowledgeList(ctx, tenantID, ids)`
- Produces:
  - `DeleteDataSource(ctx, id string) error` → 新签名 `DeleteDataSource(ctx, id string, purgeDocuments bool) error`（唯一调用方 handler 同步改；purge=true 时在现有序列末尾追加：入队 purge 任务 + 审计带 `purge_documents: true`）
  - `PurgeDataSourceDocuments(ctx, payload DataSourcePurgePayload) error`（asynq worker）：循环 `FindKnowledgeIDsByDataSourceID(limit 200)` → 空则跳出 → `DeleteKnowledgeList(ids)` → `HardDeleteKnowledgeList(tenantID, ids)` → 循环；收尾：`DeleteOrphanTagByName(kbID, payload.TagName)` + `DeleteAppDataSourceBindingsByDataSource(dsID)` + 每批审计/日志进度（`purged_documents` 累计，最终一条 Info）；批间 `ctx.Err()` 检查（asynq 取消可中断）
  - 入队用 service 现有 `taskEnqueuer` 或直接 asynq client（以 DeleteDataSource 所在文件能拿到的 enqueuer 为准——grep service 里现成的入队先例如 sweepStaleSubtree 用什么）
- [ ] **Step 1: 写失败测试**（sqlite fixture 扩展：ds + KB 文档 N 篇（metadata.datasource_id）+ tag/relation + binding 行；purge=true 删源后：任务入队断言（fake enqueuer）→ 手动调 PurgeDataSourceDocuments → 文档全消失（软删行+硬删行都无）、孤儿 tag 删、binding 行删、其他源文档/手动贴同名 tag 的文档不受伤（三元组+relation 保全）；purge=false 行为与现状逐字节一致（既有 datasource_delete_sqlite_test.go 全部保持绿，仅 service 构造加 purge 参数——该测试文件同步改调用签名）
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现**（顺序：DeleteDataSource 内 inspector 取消已在 Task 5 就位——purge 分支只追加在末尾；worker 方法批间查 ctx）
- [ ] **Step 4: 确认通过** + service/repository 回归（重点：`datasource_delete_sqlite_test.go` 改签名后全绿）
- [ ] **Step 5: 提交** `feat(datasource): cascade purge of synced documents on source deletion (async batched worker)`

---

### Task 9: API 层（documents-count + purge 参数）

**Files:**
- Modify: `internal/handler/datasource.go`（DeleteDataSource 读 query `purge_documents`；新 handler `CountDocuments`）
- Modify: `internal/router/routes_infra.go`（`GET /:id/documents-count`，Viewer——双选弹窗要先看数量）
- Modify: `internal/application/service/datasource_service.go`（`CountDataSourceDocuments(ctx, tenantID, dsID) (int64, error)`：FindByID→kbID→Task 7 的 count）
- Test: handler 测试（新建 `internal/handler/datasource_documents_count_test.go` 或并入既有 handler 测试文件）

**Interfaces:**
- Consumes: Task 7 count、Task 8 签名
- Produces: `GET /api/v1/datasource/:id/documents-count` → `200 {"count": N}`；`DELETE /api/v1/datasource/:id?purge_documents=true` → 204（异步清理已入队；响应不带 body——清理进度看审计/日志，YAGNI 不做进度查询接口）
- 归属校验照抄 `getOwnedDataSource` 前置（`datasource.go:241-262` 现形态）

- [ ] **Step 1: 写失败测试**（count 路由：归属 404/200+数值；purge 参数矩阵：带 true 调 purge 路径 / 不带走现路径——service fake 断言两次调用参数）
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现**（query 解析 `c.Query("purge_documents") == "true"`）
- [ ] **Step 4: 确认通过** + handler/router 回归
- [ ] **Step 5: 提交** `feat(datasource): documents-count endpoint + purge_documents delete flag`

---

### Task 10: React 双选删除 + api-client

**Files:**
- Modify: `packages/api-client/src/datasource.ts`（`remove(id, opts?: {purgeDocuments?: boolean})`；`documentsCount(id): Promise<number>`）
- Modify: `apps/web/src/data-sources/DataSourcesPage.tsx`（remove() 改双选面板）
- Test: apps/web data-sources 测试追加

**Interfaces:**
- Consumes: Task 9 两 API
- Produces: UI 流程——删除按钮 → 受控 Sheet（该文件已 import Sheet）双选面板：标题"删除数据源 {name}"；正文显示"同步的文档：N 篇"（进面板时拉 documentsCount）；默认态文案=现有承诺（文档保留）；Checkbox "同时删除这 N 篇同步文档"（默认不勾）；勾选后正文切红色警示（"将从知识库中永久删除 N 篇文档及其向量/索引，不可恢复"）+ 确认按钮文案变"删除数据源及文档"；确认 → `remove(id, {purgeDocuments})` → 成功 toast 文案区分两种情形
- api-client：`remove` 带 opts 时 query 拼 `?purge_documents=true`

- [ ] **Step 1: 写失败测试**（api-client 路径断言：带/不带 opts 的 DELETE query；面板渲染：默认不勾、勾选后警示文案与按钮文案、count 加载中态、确认调 remove 参数——harness 仿本文件既有测试）
- [ ] **Step 2: 确认失败** — `pnpm test:web`
- [ ] **Step 3: 实现** + i18n 键（`dataSource.deletePanelTitle/deletePanelKeep/deletePanelCount/deletePanelPurgeLabel/deletePanelPurgeWarning/deleteAndPurge/deleteSuccessPurged` 等，zh/en+ja/ko/ru 英文文案，6 locale）
- [ ] **Step 4: 确认通过** + `pnpm typecheck:web`
- [ ] **Step 5: 提交** `feat(data-sources): dual-choice delete panel with document purge (React)`

---

### Task 11: Vue 双选 + i18n + 台账收尾

**Files:**
- Modify: `frontend/src/views/knowledge/settings/DataSourceSettings.vue:88-96`（removeDataSource 双选）
- Modify: `frontend/src/api/datasource/index.ts`（`deleteDataSource(id, purge?: boolean)` 加 query；新增 `getDataSourceDocumentsCount(id)`）
- Modify: `frontend/src/i18n/locales/zh-CN.ts` + `en-US.ts`（+ja/ko/ru 英文）双选文案与 cancelSync 键（Task 6 的 Vue 侧 i18n 若 Vue 也显示取消按钮——本任务一并补 Vue 侧取消按钮入口，等价 React 交互）
- Modify: `docs/onyx-parity/ROADMAP.md` + `docs/onyx-parity/GAP-MATRIX.md`（K-11/K-29 → ✅（SP2-a）；变更记录行）

**Interfaces:**
- Consumes: Task 9 API、Task 6 的 cancel API
- Produces: Vue 等价交互（TDesign Dialog + Checkbox：进面板拉 count、默认不勾、勾选警示、确认带 purge 参数）；台账登记

- [ ] **Step 1: 实现 Vue 双选面板 + 取消按钮**（Dialog/Checkbox 用 tdesign-vue-next 组件，该文件现有 import 风格；`@confirm` 改为自定义确认回调）
- [ ] **Step 2: i18n 补键（6 locale）+ 验证** — `cd frontend && npm run type-check && npm run check-i18n` 双绿
- [ ] **Step 3: 台账登记**（K-11/K-29 行更新；ROADMAP SP2 行注记 SP2-a 完成与验证证据；端到端冒烟留待合并后注明）
- [ ] **Step 4: Go/前端针对性回归全绿后提交** `feat(data-sources): dual-choice delete + running-sync cancel on Vue; SP2-a ledger closeout`

---

## 端到端冒烟（合并后源码栈执行，不在本计划任务内）

1. 造一个 feishu/confluence mock 源同步若干文档 → 删源勾选同删 → 文档/向量/标签全清、审计有 purge 记录
2. 删源不勾选 → 行为与旧版一致（文档保留）
3. 长同步运行中点取消 → SyncLog=canceled、cursor 保留、下次同步从断点续
4. 排队中的 sync 任务在删源后被取消（asynq 队列可见）
5. 心跳超窗的 stall 行不再阻塞下一次调度

## Self-Review 记录

- **Spec 覆盖**：spec §3.1→Task 1、§3.2→Task 2+3、§3.3→Task 4+5、§3.4→Task 6、§4.1→Task 9、§4.2→Task 7+8、§4.3→Task 10+11、§7 红线→各任务约束、§9 阶段→本计划即 SP2-a。§3.1 的 payload `Scope/Attempt` 字段属 SP2-b 载体，本计划不加（YAGNI，spec 亦标注"子项 2 用"）。
- **占位符扫描**：迁移号"提交时重查"是有意的动态指引（Global Constraints 首条），非 TBD；sqlite JSON 谓词给了双方案与判定方法（Task 7 Step 3）；无其他占位。
- **类型一致性**：`SyncStallWindow`/`UpdateHeartbeat`/`RequestCancel`/`FindKnowledgeIDsByDataSourceID`/`CountKnowledgeByDataSourceID`/`DeleteDataSource(ctx,id,purge)`/`PurgeDataSourceDocuments`/`cancelSyncLog`/`documentsCount` 在生产任务与消费任务间逐一对应；`cancelSyncLog` 命名 Task 4 handler 与 Task 6 api-client 一致。
- **已知不确定点**（均给出核对方法）：空 kbID 传 `CancelTasksForKnowledgeBase` 的 matcher 行为（Task 5 Step 1）；sqlite `->>` 兼容性（Task 7 Step 3）；tag repo 方法落点（Task 8 Files）；asynq purge handler 注册点（Task 8 Files，grep TypeKnowledgeListDelete）；@weknora/ui 组件面（Task 10 用已 import 的 Sheet）。
