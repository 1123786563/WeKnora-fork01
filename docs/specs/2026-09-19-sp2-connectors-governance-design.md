# SP2 · Connectors 治理补洞（attempt 级进度/心跳/取消、删源级联清理、targeted reindex、凭据动态续期）设计

- 日期：2026-09-19
- 状态：待用户审阅
- 上游输入：[GAP-MATRIX.md](../../onyx-parity/GAP-MATRIX.md)（K-11、K-12、K-29、K-3）、[ROADMAP.md](../../onyx-parity/ROADMAP.md)（SP2）
- 参照系：Onyx `/Users/wuyongjun/trea/onyx`（IndexAttempt 状态机/targeted reindex/connector_deletion/credentials_provider 续期语义）；本设计不移植 Onyx 代码，用 WeKnora 既有基建实现等价能力
- 组织决策（方案 B）：一份 spec，两阶段实施——**SP2-a = 子项1+子项3**（取消基建+级联删除，强耦合）、**SP2-b = 子项2+子项4**（定向重抓+凭据续期），两个独立分支周期先后走

## 1. 背景与目标

datasource 同步管道在治理面有四个洞（GAP-MATRIX）：SyncLog 无 attempt 级进度/心跳/取消（K-11）、失败文档无定向重抓入口（K-12）、删源后文档永久残留（K-29）、凭据无动态续期通道（K-3）。调研另发现两个现存 bug 随子项 1 一并修复。

**验收口径**：
- 长同步任务可见实时进度、可中途取消（优雅退出且断点可续）；stall 任务不再永久阻塞调度；多副本长任务不再被启动窗口误杀
- 删源弹窗双选，勾选同删时该源同步的全部文档（含向量/图谱/wiki/文件/tag/绑定）被清理
- 失败样本在 React 端可见（带 external_id），可勾选批量重抓；11 个连接器全部支持定向重抓
- 凭据含时效 token 时：临期自动刷新、加密回写、审计留痕、AuthVersion 递增使旧 cursor 失效（以构造性 mock OAuth 连接器验收）

## 2. 用户决策记录（2026-09-19）

| 决策点 | 结论 |
|---|---|
| 凭据续期范围 | **本期完整做**（token 存储+刷新回写+审计；当前无真实消费者，以构造性测试验收，为 SP8 OAuth 连接器铺路） |
| 删源语义 | **双选交互**（默认保留文档=向后兼容；勾选"同时删除同步的 N 篇文档"才级联） |
| FetchByExternalID 覆盖 | **全量实装 11 个连接器** |
| 实施组织 | 方案 B：一 spec 两阶段（SP2-a / SP2-b） |

## 3. 子项 1：attempt 级进度 / 心跳 / 取消（SP2-a）

### 3.1 数据模型

迁移（PG versioned + SQLite 双轨，提交时取当时下一可用号——**多 lane 撞号教训：合并前重查**）：`sync_logs` 加三列：
- `heartbeat_at timestamptz`（活性时间，NULL=无心跳）
- `asynq_task_id varchar`（任务反查；ManualSync 入队与调度器入队两处写入）
- `cancel_requested boolean not null default false`

`DataSourceSyncPayload`（`internal/types/datasource.go:496-516`）已有 `ForceFull/MaxItems/Trigger/Initiator`；本子项加 `Scope`（子项 2 用）与 `Attempt` 显式字段（`asynq.GetRetryCount` 语义提升）。

### 3.2 心跳与 stall 判活

- 流式路径：`streamSyncHandler.Checkpoint`（`internal/application/service/datasource_service.go:1183-1226`）在既有 `UpdateResult` 处顺带刷 `heartbeat_at`（该调用已刷 `updated_at`，`datasource_repo.go:339-365`）
- 批量路径（FetchAll/FetchIncremental 非流式）：补批次边界中间回写（现只在结束 `updateSyncRunResult` `datasource_service.go:1336-1389` 落盘）
- **bug 修复 ①**：启动兜底 `resetPendingStaleTasks`（`internal/container/reset_pending_tasks.go:123-169`）判活源从 `started_at < now-30min` 改为 `heartbeat_at/started_at 取 COALESCE 后 < now-STALL_WINDOW`，`STALL_WINDOW = 任务超时(2h) + 15min 缓冲`——多副本不再误杀长任务
- **bug 修复 ②**：调度防重叠 `HasRunningSync`（`internal/datasource/scheduler.go:167`、`datasource_repo.go:306-319`）排除心跳超窗的 running 行——stall 不再永久阻塞该源的定时同步

### 3.3 取消（分层）

- **运行中（协作式）**：`POST /datasource/:id/sync/:log_id/cancel`（Viewer+）置 `cancel_requested`；同步循环在**每个 checkpoint/批次边界**检查本 ds 标记 → 优雅退出：status=canceled、cursor 保留（断点续传语义 `streamStartCursor` 已有）、错误信息注明 "canceled by user"
- **排队中（硬取消）**：`TypeDataSourceSync` 加入 `taskTypesForKnowledgeCancel` 集合（`internal/router/task_inspector.go:87-95`）；DataSourceService 注入 TaskInspector，`DeleteDataSource`/`Pause` 调用 `CancelTasksForKnowledgeBase(ctx, kbID, nil, []string{dsID})`（matcher 已支持 payload 探针按 DataSourceID 匹配，`task_inspector.go:63,1054-1057`；KB 删除路径 `knowledgebase.go:986` 是现成样板）
- **Pause 增强**：语义变为"暂停并取消进行中/排队的同步"（现只改状态+摘 cron，`datasource_service.go:640-659`）
- 调度器 `ErrTaskIDConflict → canceled` 分支（`scheduler.go:217-225`）改记 `skipped_duplicate` 类独立错误信息，不再与用户取消混淆

### 3.4 前端

- api-client：`DataSourceSyncLog` 类型补 `heartbeat_at/cancel_requested`；加 `cancelSyncLog(dsID, logID)` 方法
- React DataSourcesPage：运行中的行显示取消按钮（点击→cancel→轮询自然收敛 canceled）；运行中进度计数实时性由现有 3 秒轮询承接（`DataSourcesPage.tsx:102-130`）

## 4. 子项 3：删源级联清理（SP2-a，双选交互）

### 4.1 API

- `GET /datasource/:id/documents-count` → `{count}`（双选弹窗显示 N 篇）。KB 归属：DataSource 与 KnowledgeBase 是 1:1 绑定（`types/datasource.go:73` KnowledgeBaseID），count 按 `(tenant, ds.KnowledgeBaseID, dsID)` 统计，级联删除作用域同此
- `DELETE /datasource/:id?purge_documents=true` → 级联；不带参数 = 现行为（保留文档），完全向后兼容

### 4.2 级联管道（勾选同删时，顺序硬约束）

1. **先取消**该 ds 排队+运行中任务（§3.3 基建；顺序不可换——否则边删边同步重建）
2. 新 repo 方法 `FindKnowledgeIDsByDataSourceID(tenantID, kbID, dsID, limit, offset)`：`metadata->>'datasource_id' = ?` 等值查询；表达式索引照抄 `idx_knowledges_kb_metadata_external_id` 模式（`internal/application/repository/knowledge.go:796-812`，新索引 `idx_knowledges_kb_metadata_datasource_id`）
3. 分批（200/批，对齐既有 batch-delete 上限 `internal/handler/knowledge.go:1323-1327`）走**既有异步删除管道** `enqueueKnowledgeListDelete → executeKnowledgeDelete`（`knowledge_delete.go:427-578`）：向量库（按 EmbeddingModelID/Type 分组）/知识图谱/wiki 页/chunks/物理文件全部复用
4. 软删+硬删两步（tombstone 契约，`repository/knowledge.go:841-858`）；作用域按 `(tenant, kb, datasource_id)` 三元组限定——绝不误伤其他源同 external_id 文档
5. 尾巴清理：per-ds auto-tag（`resolveAutoTagIDs` 创建的 `datasource:<id>` 类标签）、`app_datasource_bindings` 该 ds 行
6. 审计：`AuditActionDataSourceDeleted` 扩展 `purged_documents` 计数字段

### 4.3 前端（双端 + 6 locale）

- 删除确认从 `window.confirm`（`DataSourcesPage.tsx:209`）升级为双选弹窗：默认不勾选（保留文档）；勾选"同时删除同步的 N 篇文档"时显示红色警示文案；确认按钮文案随勾选态变化
- Vue 端同步等价交互（`frontend/src/views/knowledge/settings/` 删除入口）
- 6 locale 文案：保留现有"不会删除"语义用于未勾选态；新增同删警示文案

## 5. 子项 2：targeted reindex（SP2-b，全量 11 连接器）

### 5.1 失败可定位

- `SyncItemError`（`internal/types/datasource.go:452-493`）加 `ExternalID string`（omitempty）；抓取失败路径（`applyFetchedItem` `datasource_service.go:1003-1117` 的 fetch 失败/ingest 失败分支）带出 external_id；`UnmarshalJSON` 兼容旧数据（无 ExternalID 字段）
- 失败样本上限维持 100（`maxSyncResultErrors`）

### 5.2 定向重抓接口

新可选接口（`internal/datasource/connector.go`，按 `FullSyncWithCursor` 的"可选接口 + service 类型断言降级"既有模式）：

```go
// TargetedFetcher refetches one source item by its external id for a
// targeted reindex round.
type TargetedFetcher interface {
    FetchByExternalID(ctx context.Context, config DataSourceConfig, externalID string) (FetchedItem, error)
}
```

**全量 11 个实装**：feishu wiki（`connector/wiki`，按 obj_token 取文档）、feishu drive、lark（区域化复用）、lark_drive、notion（page id）、yuque（doc slug）、gitlab（file path@ref）、ima、rss（feed 条目 GUID 重拉）、confluence（page id，cloud/server 双 edition）、dingtalk（知识库文档 id）。未实现该接口的连接器（未来新增）service 降级为"拒绝 scoped reindex 并提示下次增量自然重试"。

### 5.3 scoped 重跑

- `POST /datasource/:id/reindex`，body `{"external_ids": ["..."], "request_id": "..."}`（request_id 幂等，防双击重复入队）
- 生成一次 scoped run：`DataSourceSyncPayload.Scope.ExternalIDs` + Trigger=`manual_reindex` → 每项 `FetchByExternalID → ingestItem`
- **SubtreeKeep 契约**：ingestItem 的先删后建必须遵守子树协议（`types/datasource.go:349-381` PRECONDITIONS），单文档重抓不误删附件子树；返回值含 per-item 成功/失败结果
- **双写收敛**：scoped run 写自己的 SyncLog（Trigger 区分）；`DataSource.LastSyncResult` 只在整源 run 刷新——scoped run 不污染
- `ManualSync` API 放开 `force_full` 参数（payload 字段已有，补 handler 入口），供"整源强制全量对账"

### 5.4 前端

- api-client：`DataSourceSyncLog` 类型补 `result` 字段声明（含 `errors[]` 的 ExternalID）；加 `reindexItems(dsID, externalIDs, requestID)` 方法
- React DataSourcesPage：日志抽屉补失败样本渲染（移植 Vue `DataSourceSyncLogs.vue:136-156`：`result.errors` 列表 + `datasource.syncError.<code>` i18n 本地化 + title—reason 格式）；失败项复选框 + "重试选中项"按钮 → reindex → 显示 scoped run 结果
- i18n：6 locale 补 syncError code 集（现有 3 个 code 扩充至调研发现的全部 code：deletion_lookup_failed/deletion_failed/ingest_failed/fetch 类 code）

## 6. 子项 4：凭据动态续期（SP2-b，完整实现）

**安全红线**（实现验收条件）：凭据值只经 `SYSTEM_AES_KEY`（环境变量）加密通道读写；**源码、示例、测试不写入任何可用凭据字面量**——测试一律构造性假值。

### 6.1 存储协议

- token 类凭据字段（`access_token` / `refresh_token` / `expires_at` ISO 时间）作为 Credentials map 的普通键值，经既有 `ToJSON/ParseConfig` AES-256-GCM 通道加密（`internal/types/datasource.go:531-619` 唯一读写路径纪律不变）
- 无 schema 变更（JSONB 内字段约定）；`ParseConfig` 对 `expires_at` 做宽松解析（缺失/坏值=视为长效）

### 6.2 机器回写通道

新 service 方法（与用户版 `UpdateDataSourceCredentials` `datasource_service.go:339-378` 的三点区别）：

```
RefreshDataSourceCredential(ctx, dsID, key, value):
  1. 读 ds + ParseConfig（解密）
  2. HasConfiguredCredentials == false → 拒绝回写（防密钥轮换后空值永久覆盖真实凭据——调研坑）
  3. 单 key 更新（非整体替换）→ ToJSON 加密 → 保存
  4. 审计动作 credential_auto_refreshed（独立于用户 PUT 的 changed_fields 审计）
  5. 不触发用户级 live validate（续期结果由下一次同步自然验证）
```

### 6.3 续期触发协议

- 连接器可选接口：`CredentialsRefresher`（`RefreshCredentials(ctx, config) (updated map[string]string, nextRefreshAt time.Time, err error)`）
- 同步启动时（`ProcessSync` 读 config 后）：凭据含未过期 `expires_at` 且临期（<5min）→ 调 Refresher → 机器通道回写 → **AuthVersion 递增**（binding 行 `ConnectionAuthVer` 体系已有，`appconnector/sync.go:62-148`；cursor 的 `_sync_auth_version` 语义使旧 cursor 失效 → 下次自动全量对账，防 token 切换期漏抓）
- 过期未续成功 → `SyncPausedError(permission)` 语义暂停（`datasource_service.go:98-131` 现成状态机），不产生 error 噪音

### 6.4 验收载体与元数据

- **mock OAuth 型连接器**（测试专用，注册在测试装配）：构造性假 token + 假刷新端点，验证"临期→刷新→加密回写→审计→AuthVersion 递增→旧 cursor 失效"全链路；SP8 接 Google Drive 时换真实实现
- credentials 子资源响应（`dto.CredentialsResponse`）加字段级元数据：`expires_at` / `last_refreshed_at` / `needs_reauthorization`（MCP/Model sibling 子资源的字段级模型平移，`datasource_credentials.go:17-22`）

## 7. 错误处理与安全

- 取消优先协作式：数据一致性最好（checkpoint 边界退出+cursor 保留）；硬取消仅用于排队任务（尚未开始，无中间态）
- 级联删除的三元组限定 + 分批异步管道复用（不新造删除逻辑）；先取消后删除的顺序由代码结构保证（同一 service 方法内顺序调用，不靠调用方自觉）
- 机器回写通道的防覆盖守卫（§6.2 步骤 2）是硬约束，测试锚定
- SyncLog 新列全部 nullable/default，迁移可逆；`pending` 裸字符串顺手收编为常量（`datasource_repo.go:376`）

## 8. 测试与验收清单

**子项 1**：心跳回写（流式+批量两路径）；判活单测（长任务不被启动兜底误杀=bug①回归；stalled 行不阻塞调度=bug②回归）；协作式取消（checkpoint 处取消→canceled+cursor 保留）；TaskInspector 集成（排队任务取消）；Pause 增强语义。
**子项 3**：级联端到端（sqlite 迁移库：文档+向量 mock+tag+binding 全清、顺序保证）；`purge_documents` 参数矩阵（带/不带）；分批上限；三元组限定（其他源同 external_id 不受伤）；auto-tag/binding 尾巴清理；双端弹窗交互测试。
**子项 2**：11 连接器各自 FetchByExternalID 单测（构造性 fixture）；SubtreeKeep 契约回归；scoped run 独立 SyncLog 且不污染 LastSyncResult；request_id 幂等；React 失败样本渲染+i18n；force_full API。
**子项 4**：mock OAuth 连接器全链路；空值防覆盖（SYSTEM_AES_KEY 缺失时不回写）；审计动作断言；AuthVersion 递增与 cursor 失效；credentials 元数据字段。
**端到端（合并后源码栈，两阶段各自做）**：SP2-a 真删一个有文档的源（双选）+ 长任务取消；SP2-b 失败项 reindex + mock 续期链路。

## 9. 阶段划分与依赖

| 阶段 | 内容 | 依赖 |
|---|---|---|
| SP2-a | 子项 1（§3）+ 子项 3（§4） | 无（取消基建是级联的前置，同阶段内先后） |
| SP2-b | 子项 2（§5）+ 子项 4（§6） | SP2-a（scoped run 载体=payload/attempt 扩展；reindex 前先取消排队） |

两阶段各自走完整 subagent-driven 流程（worktree/审查/终审/合并），迁移号在各自合并前重查当下占用。
