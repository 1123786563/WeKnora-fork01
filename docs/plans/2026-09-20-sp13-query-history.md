# SP13 查询历史审计+导出+隐私+分享（Onyx 平台运营面对齐）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** admin 查询历史审计（全 source+过滤+快照）、asynq 异步 CSV 导出三段式、租户级隐私三档、会话租户内登录分享（GAP-MATRIX P-6~P-9）。

**Architecture:** 复用现有 admin 渠道会话视图骨架（SessionListQuery 扩维）；导出照 datasource sync_log 模式新建 `query_history_export_jobs` 表 + asynq 任务双注册（mux+Lite executor）+ FileService 落地；隐私三档为 tenants 新 JSONB `query_history_config`（照 MemoryConfig）经现有 KV 路由读写；分享为 sessions 新列 `share_token`（32B crypto/rand base64url）+ 同租户登录校验只读端点。

**Tech Stack:** Go（gin+GORM+golang-migrate+asynq+dig）、React 19 + TanStack Router + Tailwind v4、node:test。

**Spec:** `docs/superpowers/specs/2026-09-19-onyx-platform-parity-design.md` 第 4 节 SP13 部分；GAP-MATRIX P-6~P-9。

## Global Constraints

- 成功 `gin.H{"success":true,"data":...}`；错误 `c.Error(errors.NewXxx)` 交 ErrorHandler。
- 迁移成对：versioned 下一个 **000163**、sqlite 下一个 **000084**（先 ls 防并行撞号；sqlite 头注释 `-- SQLite dialect of PG 000163`）。
- 新路由经 `g.apiKeyGroup(...)` 声明；admin 端点 `g.Admin()`；时间参数 `start_time/end_time`（parseFilterTime 惯例）。
- 隐私三档在**审计面 service 层统一拦截**：`disabled`→403（文案含"query history disabled"）；`anonymized`→响应中 user_id/owner 邮箱类标识替换 `"anonymous"`；读取 `tenant.QueryHistoryConfig`（nil 视为 normal）。
- 分享访问 404 不泄露存在性（无 token/错租户/撤销一律 404）；只读快照不含反馈者身份。
- repository 测试 `openRunTestDB(t)`；前端测试 node:test。
- commit 前 porcelain 核对显式路径，**绝不 add -A**（并行会话整树提交前科）。

---

### Task 1: 迁移 + 模型（share_token 列 / QueryHistoryConfig / QueryHistoryExportJob 表）

**Files:**
- Create: `migrations/versioned/000163_query_history_share.up.sql` / `.down.sql`、`migrations/sqlite/000084_query_history_share.up.sql` / `.down.sql`
- Modify: `internal/types/session.go`（Session 加 ShareToken）、`internal/types/tenant.go`（Tenant 加 QueryHistoryConfig）
- Create: `internal/types/query_history.go`（QueryHistoryConfig/QueryHistoryExportJob/JobStatus 常量）
- Test: `internal/application/repository/query_history_test.go`（建表烟测+share_token 唯一性）

**Interfaces — Produces:**
```go
// types/query_history.go
const (QueryHistoryModeNormal = "normal"; QueryHistoryModeAnonymized = "anonymized"; QueryHistoryModeDisabled = "disabled")
const (QueryHistoryExportPending = "pending"; QueryHistoryExportRunning = "running"; QueryHistoryExportDone = "done"; QueryHistoryExportFailed = "failed")
type QueryHistoryConfig struct{ Mode string `json:"mode" yaml:"mode"` } // Normalize(): 空/非法→normal
type QueryHistoryExportJob struct {
    ID uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
    TenantID uint64 `json:"tenant_id" gorm:"index"`
    RequestedBy string `json:"requested_by" gorm:"type:varchar(512)"`
    Status string `json:"status" gorm:"type:varchar(16);not null;default:'pending'"`
    FilePath string `json:"file_path" gorm:"type:varchar(512);not null;default:''"`
    ErrorMessage string `json:"error_message" gorm:"type:text;not null;default:''"`
    CreatedAt time.Time `json:"created_at"`; UpdatedAt time.Time `json:"updated_at"`
}
func (QueryHistoryExportJob) TableName() string { return "query_history_export_jobs" }
// types.Session 加：ShareToken string `json:"-" gorm:"type:varchar(64);uniqueIndex"`（json:"-" 不外泄；快照端点显式组装时也不含）
// types.Tenant 加：QueryHistoryConfig *QueryHistoryConfig `yaml:"query_history_config" json:"query_history_config" gorm:"type:jsonb"`
```

迁移内容：`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS share_token VARCHAR(64) DEFAULT NULL;` + `CREATE UNIQUE INDEX IF NOT EXISTS uq_sessions_share_token ON sessions(share_token) WHERE share_token IS NOT NULL;`（sqlite 同构支持部分索引）；`ALTER TABLE tenants ADD COLUMN IF NOT EXISTS query_history_config JSONB/JSON DEFAULT NULL;`；`CREATE TABLE IF NOT EXISTS query_history_export_jobs (...)`（id BIGSERIAL/tenant_id/requested_by VARCHAR(512)/status VARCHAR(16) DEFAULT 'pending'/file_path VARCHAR(512) DEFAULT ''/error_message TEXT DEFAULT ''/created_at/updated_at TIMESTAMPTZ）。down：DROP INDEX/COLUMN/TABLE 对称。

测试：迁移后 INSERT 两行同 share_token → 第二条违反唯一索引报错；NULL 不受唯一约束（两行 NULL 合法）。

### Task 2: admin 审计列表（SessionListQuery 扩维 + 隐私拦截）

**Files:**
- Modify: `internal/types/session.go`（SessionListQuery 加 StartTime/EndTime/FeedbackRating time.Time/string）、`internal/application/repository/session.go`（applyBase 追加过滤）、`internal/application/service/session.go`（admin 全量视图 gate + 隐私拦截）、`internal/handler/session/handler.go`（ListSessions 参数）
- Create: `internal/application/service/query_history_policy.go`（三档拦截 helper）+ 同名测试
- Test: repository/service/handler 三层追加测试

**设计：**
- **admin 全量视图触发**：`SessionListQuery.Source = "all"`（新常量 `SessionListSourceAll`）且 caller Admin+ → `query.UserID=""` 不再限定（复用 SessionListSourceRequiresAdmin 语义：把 "all" 加进 requires-admin 集合）。user_id 过滤参数照旧（admin 显式传 user_id 时 applyBase 生效）。
- applyBase 追加：`q.StartTime`/`q.EndTime` 非零 → `s.created_at >= ? AND s.created_at < ?`；`q.FeedbackRating ∈ {like,dislike}` → `EXISTS (SELECT 1 FROM message_feedback f WHERE f.session_id = s.id AND f.rating = ? AND f.tenant_id = s.tenant_id)`。
- handler ListSessions 补 `user_id/start_time/end_time/feedback` query 参数（parseFilterTime 惯例；feedback 只接受 like/dislike，其他值 400）。
- **query_history_policy.go**：`func CheckQueryHistoryAccess(ctx, tenantRepo, tenantID) (mode string, err error)`——disabled 返回 `NewForbiddenError("query history is disabled for this tenant")`；service 层 ListSessions（仅 Source=all 分支）/快照/导出入口调用。`AnonymizeSessionRows([]types.Session)`：把 UserID 置 `"anonymous"`（anonymized 模式）。

### Task 3: 会话快照端点 + 反馈 ListBySession + 隐私 KV

**Files:**
- Modify: `internal/types/interfaces/feedback.go` + `internal/application/repository/feedback.go`（加 `ListBySession(ctx, tenantID, sessionID) ([]types.MessageFeedback, error)`）
- Create: `internal/handler/session/query_history_admin.go`（快照端点 + Task 4 的导出三端点都放此文件）
- Modify: `internal/router/routes_chat.go` 或新建 `routes_query_history.go`（挂 admin 端点）
- Modify: `internal/handler/tenant.go`（KV switch 两处加 `case "query-history-config"`）+ 测试

**快照端点**：`GET /api/v1/admin/sessions/:session_id/snapshot`（g.Admin）：
- service：`GetQueryHistorySnapshot(ctx, tenantID, sessionID)`——隐私拦截（disabled 403）→ `sessionRepo.GetByID`（tenant 过滤）→ `messageService.GetRecentMessagesBySession(ctx, sessionID, 200)`（大 limit；若不足循环 before_time 翻页拼全量——首版 200 上限，超限截断并在响应标 `truncated:true`）→ `feedbackRepo.ListBySession` → 组装：
```go
type QueryHistorySnapshot struct {
    Session types.Session `json:"session"`            // anonymized 时 UserID 已置 anonymous
    Messages []types.Message `json:"messages"`        // 复用现有序列化（knowledge_references 已内嵌）
    Feedback []types.MessageFeedback `json:"feedback"` // anonymized 时 UserID 置 anonymous
    Truncated bool `json:"truncated"`
}
```
- KV：GET/PUT `/tenants/kv/query-history-config`（Viewer 读/Admin 写，照 memory-config case 逐模式：Get 校验 Normalize+返回；Put 校验 Mode 枚举非法 400）。

### Task 4: asynq 异步 CSV 导出三段式

**Files:**
- Create: `internal/application/repository/query_history_export.go`（job CRUD：Create/Get/UpdateStatus）+ interfaces 声明
- Create: `internal/application/service/query_history_export.go`（StartExport 入队 / ProcessExport 任务体）+ 测试
- Modify: `internal/types/task.go`（`TypeQueryHistoryExport = "query_history:export"` + QueueDefinitions 登记到 maintenance 队列——先看现有队列命名）、`internal/router/task.go`（mux.HandleFunc）、`internal/router/sync_task.go`（Lite 双注册）、`internal/container/container.go`（Provide）
- Modify: `internal/handler/session/query_history_admin.go`（三端点）

**三端点**（均 g.Admin + 隐私拦截）：
- `POST /api/v1/admin/sessions/export`（body 可选 user_id/start_time/end_time/feedback 同列表过滤）→ 建 job(pending) → 组 payload{JobID,TenantID,Filter,RequestedBy} → `asynq.NewTask(TypeQueryHistoryExport, payloadJSON, Queue(...), MaxRetry(3), Timeout(10m))` 入队失败→job failed → 返回 `{job_id}`
- `GET /api/v1/admin/sessions/export/:job_id/status` → `{status, error_message}`
- `GET /api/v1/admin/sessions/export/:job_id/download` → status=done 且 job.TenantID==caller tenant → `fileService.GetFile(job.FilePath)` 流式回吐 + CSV header/BOM（照 usage.go 模式）；非 done 400

**ProcessExport 任务体**：job→running → 查询（复用 Task 2 的过滤 SQL：repo 加 `QueryPagedForExport` 或直接循环 QueryPaged 大页拼全量，上限 10000 行防炸）→ CSV 列：session_id,title,user_id,source,engine_type,created_at,updated_at,message_count,feedback_summary（like/dislike 计数）——每会话一行汇总（Onyx 是逐问答对；我们首版按会话行，问答明细在快照端点）→ `fileService.SaveBytes(ctx, data, tenantID, "query_history_export_<jobid>.csv", true)`（temp=true）→ 更新 job done+file_path；任何失败→failed+error_message。

### Task 5: 会话分享（生成/撤销/只读访问）

**Files:**
- Modify: `internal/application/service/session.go` + `internal/types/interfaces/session.go`（加 `ShareSession/UnshareSession/GetSharedSession`）
- Modify: `internal/handler/session/handler.go` 或 query_history_admin.go 同级新文件（三端点）
- Modify: `internal/router/routes_chat.go`（sessions 组挂分享路由）
- Test: service/handler 测试

**端点：**
- `POST /api/v1/sessions/:session_id/share`（Viewer+，仅 owner 本人或 Admin——复用 SP11 canFeedback 同款判定逻辑放 service）→ 生成 token（照 mintOAuthState：32B crypto/rand → base64.RawURLEncoding 43 字符）→ `UPDATE sessions SET share_token=? WHERE id=? AND tenant_id=?`（撞唯一索引极小概率重试一次）→ 返回 `{share_token}`（Session.ShareToken json:"-"，仅此端点显式返回）
- `DELETE /api/v1/sessions/:session_id/share` → 置 NULL
- `GET /api/v1/shared/sessions/:token`（Viewer+，**apiKeyGroup 声明 chat 档**）→ `SELECT * FROM sessions WHERE share_token=? AND tenant_id=caller.tenant AND deleted_at IS NULL` → 无命中 404（`NewNotFoundError("shared session not found")`，不区分撤销/错租户/不存在）→ 命中则返回与 Task 3 快照同构的只读数据（复用 GetQueryHistorySnapshot 的组装但跳过 admin 判定、Feedback 不含 UserID——直接置空）。

### Task 6: contracts + api-client

**Files:**
- Create: `packages/contracts/src/query-history.ts` + 测试（QueryHistorySessionRow 扩展 Session 契约的审计行/QueryHistorySnapshot/QueryHistoryExportJob 状态/SharedSnapshot + parse 函数）
- Create: `packages/api-client/src/queryHistory/index.ts` + 测试（createQueryHistoryApi：adminList(params)/snapshot(sessionId)/startExport(body)/exportStatus(jobId)/downloadExport(jobId)（requestBinary）/share(sessionId)/unshare(sessionId)/shared(token)/queryHistoryConfig KV 读写——KV 若已有通用 tenants kv client 方法则复用，先 grep）
- Modify: contracts/index.ts、api-client client.ts/index.ts 装配

### Task 7: 前端 admin 审计分区（query-history）

**Files:**
- Create: `apps/web/src/settings/QueryHistoryPanel.tsx` + `query-history-panel.test.tsx`（纯函数）
- Modify: registry.ts（`{ key: 'query-history', viewId: 'QueryHistorySettings', apiDomain: 'queryHistory', scope: 'tenant', minRole: 'admin', operations: ['read'], ported: true }`）、surface.ts、SettingsPage.tsx 分支、i18n 五 locale

**面板**（照 SystemAuditLogPanel 表格+抽屉模式）：过滤行（user_id 输入/日期范围 feedback 下拉/source=all 固定）+ 表格（title/user/source/engine/created/反馈计数列）+ 行点击开抽屉（快照：消息流渲染复用现有 markdown 渲染或纯文本首版+引用列表+反馈列表）+ 导出按钮（startExport→轮询 exportStatus 每 2s→done 变下载按钮 downloadExport 触发浏览器下载）+ **disabled 模式**：面板挂载时拉 KV config，disabled 显示"已禁用"占位（后端 403 也兜底显示）。

### Task 8: 前端分享 + 隐私设置 UI

**Files:**
- Modify: `packages/views/src/chat/session-sidebar.tsx`（菜单加"分享"项 + 分享面板照内联 dialog 模式：生成后展示链接 `location.origin + '/platform/shared/' + token` + 复制按钮 + 撤销按钮；onShare/onUnshare 可选 props）
- Modify: `packages/views/src/chat/page.tsx`（props 透传）、apps/web 聊天宿主（接线 client.sessions.share/unshare——api-client sessions 域加方法或 queryHistory 域，统一放 queryHistory 域）
- Create: `apps/web/src/shared/SharedSessionPage.tsx`（只读渲染：session 标题+消息流只读+引用；无输入框）+ router.tsx 挂 `platformRoute` 下 `path: 'shared/$token'`（复用登录守卫与 shell）
- Modify: apps/web settings 隐私三档 UI（并进 Task 7 的 QueryHistoryPanel 顶部或独立小区块：三选一 radio 读写 query-history-config KV，Admin 可写）
- i18n 五 locale（分享菜单/弹窗/只读页/隐私设置文案）

### Task 9: 回归、冒烟、证据与登记

- 后端全量（本线 query_history 相关全绿+预存判归属）+ 前端全量（同法）
- 冒烟（共享 dev 栈）：审计列表过滤/快照抽屉/导出三段式全链路（触发→轮询→下载文件内容正确）/分享生成→只读页打开→撤销后 404/隐私 disabled 后审计 403+面板隐藏/anonymized 后 user 列匿名
- evidence `docs/migrations/react/evidence/onyx-parity/2026-09-20-sp13-query-history.md` + ROADMAP SP13 ✅ + GAP-MATRIX P-6~P-9 ✅
- 遗留预填：CSV 按会话行（非 Onyx 逐问答对——明细走快照）；快照 200 条截断；分享 token 无过期时间（撤销即失效）；export 上限 10000 行

---

## Self-Review 记录

- Spec 覆盖：P-6（审计列表/快照）→ Task 2/3；P-7（导出三段式）→ Task 4；P-8（隐私三档）→ Task 1/2/3+Task 7/8 UI；P-9（租户内分享）→ Task 1/5+Task 8 ✓
- 类型一致性：QueryHistorySnapshot/ExportJob/ShareToken 在 Go/contracts/前端三端对齐；feedback ListBySession 供 Task 3/5 复用
- 执行期确认点（已给 grep 指引）：QueueDefinitions 队列名、tenants kv client 复用性、GetRecentMessagesBySession 翻页语义、SQLite 部分索引版本支持（现代c/sqlite 均支持，测试兜底）
- 复用清单：SP11 feedback 表/canFeedback 判定、SP12 usage.go CSV header 模式、mintOAuthState token、MemoryConfig KV 模式、SystemAuditLogPanel/TenantAuditDrawer 抽屉、datasource sync_log 任务模式
