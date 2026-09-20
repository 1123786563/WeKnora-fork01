# SP2-b · Connectors 治理第二阶段（targeted reindex + 凭据动态续期）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 失败样本可定位（ExternalID）+ 定向重抓（TargetedFetcher 全量实装+scoped run）+ force_full 放开；凭据动态续期全链路（机器回写+临期触发+AuthVersion cursor 失效+mock OAuth 验收）（GAP-MATRIX K-12/K-3）。

**Architecture:** TargetedFetcher 仿 FullSyncWithCursor 可选接口模式（service 断言降级）；scoped run 走 payload.Scope.ExternalIDs 独立分支（独立 SyncLog、不污染 LastSyncResult/ds.Status）；续期走 CredentialsRefresher 可选接口 + RefreshDataSourceCredential 机器通道（防覆盖守卫）+ `_sync_auth_version` 从只写补消费端（不符丢 cursor 走全量对账）。

**Tech Stack:** Go（gin/GORM/asynq）、React 19、node:test。**无新迁移**（凭据是 JSONB 字段约定）。

**Spec:** `docs/specs/2026-09-19-sp2-connectors-governance-design.md` §5+§6（执行者需同时读 spec 与本计划）。

## Global Constraints

- **安全红线（§6）**：凭据值只经 SYSTEM_AES_KEY 加密通道；**源码/示例/测试零凭据字面量**——测试一律构造性假值（`test-token-<n>` 风格）
- SubtreeKeep 契约：单文档重抓复用现有先删后建逻辑天然重建 keep；**任何 fallback/export 路径不得设 ReplacesSubtree**（feishu/core/shared.go:340 先例语义）
- scoped run 双写收敛：只写自己的 SyncLog（Trigger=manual_reindex）；**不写 LastSyncResult、不翻 ds.Status**（不复用 updateSyncRunResult 的连带副作用——写 scoped 专用收敛）
- request_id 幂等（防双击重复入队）；迁移号本期无新迁移；MaxItems 顺带消费（scoped=ExternalIDs 数量）
- 测试命令照 SP2-a；提交纪律 porcelain+diff --cached --stat 双核对；预存失败基线甄别（stash/worktree 对照）

## 已核实的代码事实（调研产出，执行者直接依赖）

- 可选接口模式：`internal/datasource/connector.go:54-66`（FullSyncWithCursor）+ service 断言先例 `datasource_service.go:1099-1117`
- **lark×4=2 实现**：wiki（core.Region Feishu/Lark）+ drive（FeishuDrive/LarkDrive）——TargetedFetcher 实装 9 个即覆盖 11 type（container.go:2134-2147）
- 9 连接器重取路径（调研第 2 项逐个结论——brief 会带）
- payload 现状 `types/datasource.go:512-533`（无 Scope；MaxItems 未消费）；ProcessSync 分支插点=流式断言（1099）之前
- applyFetchedItem 失败分支 1393-1432（fetch 失败 metadata["error"]→fetchFailureSyncError；ingest 失败→ingest_failed）；SyncItemError 落盘 result.Errors（cap 100）
- updateSyncRunResult 1759-1796 是 LastSyncResult 唯一刷新点（连带 ds.Status/审计——scoped 不能用）
- force_full：handler 466-496 无参数 + service ManualSync:819-826 固定 false + 消费处 1108-1117/1465-1470/1452-1456
- 凭据：ToJSON/ParseConfig（548-636，DecryptStoredSecretLenient 三态）；HasConfiguredCredentials 267-282；用户版 UpdateDataSourceCredentials 384-423（整替换+live validate）；AuthorizeSyncExecution 149-176（SyncPausedError 状态机）；`_sync_auth_version` 只写（1528-1531）**无读取方**；binding AuthVersion（appconnector/sync.go:62-148 + service currentSyncAuthVersion:200-209）
- credentials 元数据先例：dto/mcp.go:134-182（字段级 CredentialsResponse）；datasource_credentials.go:89-93（单逻辑字段用法）
- React：log-state.ts/card.ts/DataSourcesPage.tsx:419 无 errors 渲染；Vue 移植源 DataSourceSyncLogs.vue:120-156+271-300；i18n 现有 3 code（zh-CN.ts:958-962）待扩充（feishu 6+confluence 5+dingtalk 2+scoped 新 code）

---

### Task 1: 基建三合一（ExternalID + Scope 字段 + force_full 放开）

**Files:**
- Modify: `internal/types/datasource.go`（SyncItemError 加 `ExternalID string json:"external_id,omitempty"` + UnmarshalJSON 兼容旧数据【若 SyncItemError 无自定义 Unmarshal 则确认默认零值兼容即可】；DataSourceSyncPayload 加 `Scope *SyncScope`，`type SyncScope struct{ ExternalIDs []string }`）
- Modify: `internal/application/service/datasource_service.go`（applyFetchedItem 两失败分支带出 item.ExternalID——fetchFailureSyncError 与 ingest 分支的 recordSyncError 调用处补字段）
- Modify: `internal/application/service/datasource_service.go`（ManualSync 签名加 `forceFull bool` 透传 payload；唯一调用方 handler 同步）
- Modify: `internal/handler/datasource.go`（ManualSync 读 body `{"force_full": bool}` 可选默认 false）
- Test: service/handler 追加（ExternalID 落盘/旧 JSON 兼容/force_full 透传矩阵）

**Interfaces — Produces:** `SyncItemError.ExternalID`（Task 6 前端消费）；`payload.Scope.ExternalIDs`（Task 4 scoped 分支消费）；`ManualSync(ctx, dsID, forceFull)`（Task 4 无关但同签名面）。

### Task 2: TargetedFetcher 接口 + 高复用四连装（wiki/drive/notion/yuque）

**Files:**
- Modify: `internal/datasource/connector.go`（`TargetedFetcher` 接口声明，注释照 FullSyncWithCursor 风格）
- Modify: `internal/datasource/connector/wiki/connector.go`（FetchByExternalID：`GetWikiNode(spaceID, token)`→`fetchNodeContent` 薄封装；spaceID 从 config.ResourceIDs 或 item metadata——**签名只收 (ctx, config, externalID)**，space_id 未知时遍历 config 资源或返回明确错误让上层提示）
- Modify: `internal/datasource/connector/drive/connector.go`（metadata 不可得——external_id 是 file token，type 未知：实现为按 token 轮询三路径 docx→file→export，或 `ListDriveFilesAllPages`+filter 兜底；选后者稳【列表接口已有分页复用】）
- Modify: `internal/datasource/connector/notion/connector.go`（`GetPage(pageID)`→`fetchPage(ctx, client, page, map[new]{})`）
- Modify: `internal/datasource/connector/yuque/connector.go`（`GetDocDetail(docID)`→现有 item 构造——book_id/title 从 detail 响应自足或配置推导）
- Test: 各连接器 FetchByExternalID 单测（构造性 fixture——假 client 接口注入，断言 item 字段与 SubtreeKeep 语义：不设 ReplacesSubtree 的路径保持）

### Task 3: TargetedFetcher 五连装（gitlab/rss/confluence/ima/dingtalk）

**Files:**
- Modify: `internal/datasource/connector/gitlab/connector.go`（external_id 解析出 project/ref/file【格式 `gitlab:<base>:<pid>:<ref>:<file>`】→ `client.project`+`(c).item` 原样重建）
- Modify: `internal/datasource/connector/rss/connector.go`（重解析 feeds→按 GUID/Link/Title 匹配 itemID→`resolveItem`）
- Modify: `internal/datasource/connector/confluence/connector.go`（`client.body(ctx, id)`+title 从 KB metadata 恢复或单页 GET 补【选：metadata 无 title 时返回错误提示先跑一次全量——YAGNI 单页 GET 可后补】→`markdownItem`）
- Modify: `internal/datasource/connector/ima/connector.go`（metadata 缺失不可逆——external_id 是 hash：实现返回 ErrUnsupportedPrecondition（"ima 需要 metadata，请跑一次增量同步后重试"）**或**从 KB 行 metadata 恢复 media_id 走 GetMediaInfo；选前者简单+spec 允许降级提示）
- Modify: `internal/datasource/connector/dingtalk/connector.go`（documentBlocks+renderDocument；title 从 KB metadata node 恢复）
- Test: 同 Task 2 模式（ima 降级路径也要测）

### Task 4: scoped 重跑 service + reindex API

**Files:**
- Modify: `internal/application/service/datasource_service.go`（processSync 在流式断言前插 scoped 分支：`payload.Scope != nil && len(ExternalIDs)>0` → `runScopedReindex(ctx, ds, syncLog, config, payload)`：逐项 `connector.(TargetedFetcher)` 断言（不支持→该 item 记 targeted_unsupported 失败）→ FetchByExternalID → applyFetchedItem（tagIDs 复用 auto-tag 解析）→ scoped 收敛（只 updateSyncRunResult 的 SyncLog 部分：自己写 Result/终态，**不动 ds**；Trigger=manual_reindex 审计）；request_id 幂等：body 带的 request_id 作 asynq TaskID 后缀（重入队撞 ErrTaskIDConflict 即幂等拒绝））
- Modify: `internal/handler/datasource.go` + `internal/router/routes_infra.go`（`POST /:id/reindex` body `{"external_ids":[],"request_id":""}` Admin；空 ids 400；归属校验照 getOwnedDataSource；返回 202 {sync_log_id}）
- Test: service（scoped 全链：成功项/失败项计数、不支持连接器降级、LastSyncResult 不被污染【断言前后一致】、ds.Status 不翻、SubtreeKeep 契约【docx 重抓 keep 重建】、request_id 幂等）+ handler（202/400/404）

### Task 5: 凭据存储协议 + 机器回写通道 + 元数据

**Files:**
- Modify: `internal/types/datasource.go`（ParseConfig 侧加 helper `CredentialsExpiry(config) (time.Time, bool)`——宽松解析 `expires_at` ISO，缺失/坏值=长效 false）
- Modify: `internal/application/service/datasource_service.go`（`RefreshDataSourceCredential(ctx, dsID, key, value) error`：读 ds→ParseConfig→**HasConfiguredCredentials==false 拒绝**（防覆盖守卫，测试锚定）→单 key 更新→ToJSON→保存→审计 credential_auto_refreshed（独立动作）→不触发 live validate）
- Modify: `internal/handler/dto/datasource.go` + `internal/handler/datasource_credentials.go`（CredentialsResponse 扩字段级：expires_at/last_refreshed_at/needs_reauthorization——照 dto/mcp.go 先例；响应从 config 解析【解密后只回元数据不回值】）
- Test: 回写五步矩阵（守卫拒绝/单 key 不动他键/加密落库可解回/审计动作）；元数据三字段（临期 needs_reauthorization 推导）

### Task 6: 续期触发协议 + mock OAuth 验收载体

**Files:**
- Modify: `internal/datasource/connector.go`（`CredentialsRefresher` 可选接口 `RefreshCredentials(ctx, config) (updated map[string]string, nextRefreshAt time.Time, err error)`）
- Modify: `internal/application/service/datasource_service.go`（processSync ParseConfig 后插触发：`CredentialsExpiry` 临期（<5min）且实现 Refresher → 调用 → 逐 key RefreshDataSourceCredential 回写 → **binding 行 ConnectionAuthVer 递增**（照 currentSyncAuthVersion 写路径——grep binding 更新方法）→ 继续本次同步（新凭据生效）；过期未续成功→SyncPausedError(permission) 语义暂停；**`_sync_auth_version` 消费端**：streamingFetch/批量取 cursor 后比对 `cursor.ConnectorCursor["_sync_auth_version"]` 与 currentSyncAuthVersion——不符→丢弃 cursor 走全量（ForceFull 语义），并 Log）
- Create: `internal/datasource/connector/moauth/`（测试专用 mock OAuth 连接器：FetchAll 返回空+实现 CredentialsRefresher【构造性假 token：临期→刷新端点假 URL→返回新假 token+nextRefreshAt】；**不注册 container**——仅测试装配使用）
- Test: **全链路验收**（mock 连接器：临期→Refresher 调用→回写加密落库→审计→AuthVersion 递增→旧 cursor 失效→全量对账触发；过期未续→暂停不翻 error；守卫：SYSTEM_AES_KEY 缺失时 ToJSON 不加密【既有行为】+HasConfiguredCredentials=false 拒绝回写）

### Task 7: React 失败样本渲染 + 重试选中项 + i18n 扩充

**Files:**
- Modify: `packages/api-client/src/datasource.ts`（DataSourceSyncLog 补 `result?: {errors?: SyncItemError[]}` 类型；`reindexItems(dsID, externalIDs, requestID)` 方法）
- Modify: `apps/web/src/data-sources/`（日志面板补失败样本渲染【移植 Vue DataSourceSyncLogs.vue:120-156+271-300：cap 50、title—reason 格式、datasource.syncError.<code> 本地化 fallback message】；失败项复选框+"重试选中项"按钮→reindexItems→toast+轮询看 scoped run；request_id 用 crypto.randomUUID()）
- Modify: i18n（**apps/web 5 locale** + Vue 侧若同步渲染也补——syncError code 集扩充：feishu 6+confluence 5+dingtalk 2+新 targeted_unsupported/reindex 入队文案）
- Test: api-client 路径断言+面板渲染/交互（既有 harness）

### Task 8: 回归、台账与 SP2 收官

- Go/前端回归（本线全绿+预存判归属）
- 冒烟尽力（共享栈：reindex 一轮/凭据元数据端点；起不动记欠账）
- evidence `docs/migrations/react/evidence/onyx-parity/2026-09-20-sp2b-connectors-reindex-credentials.md` + ROADMAP（SP2 行整体 ✅ SP2-a+b）+ GAP-MATRIX（K-12/K-3 → ✅）+ **SP2 收官变更记录**
- 遗留预填：ima 需 metadata 前置一次增量（降级语义）；confluence 单页 title 依赖 metadata；真实 OAuth 连接器留 SP8

---

## Self-Review 记录

- Spec 覆盖：§5.1→T1；§5.2→T2/T3；§5.3→T4；§5.4→T7；§6.1→T5；§6.2→T5；§6.3→T6；§6.4→T5/T6 ✓
- 类型一致性：ExternalID/Scope/TargetedFetcher/CredentialsRefresher/RefreshDataSourceCredential/reindexItems 在任务间闭环
- 执行期确认点（brief 带）：9 连接器逐个重取路径细节（调研第 2 项）、binding AuthVersion 递增的写方法、UpdateSyncState 之外的 ds 保存方法（scoped 不动 ds 的实现细节）、crypto.randomUUID 可用性（浏览器目标）
- Rulings 预定：R-P1 ima 降级返回前置条件错误（spec 允许）；R-P2 drive 重取走列表过滤（放弃三路径轮询）；R-P3 confluence title 从 metadata（不做单页 GET）
