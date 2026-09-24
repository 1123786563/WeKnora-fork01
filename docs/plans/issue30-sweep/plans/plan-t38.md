# T08：Attention Inbox 与类型化审批闭环（Issue #38）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移动端从同一 Interaction 身份看到并决定工具审批/预算/恢复请求：新增跨 run 的 Attention Inbox 读端点与移动端 Task Office `inbox()`/`decide()` 深模块接口，决定经冻结 `decision_id` 幂等重放与 revision/digest CAS 只生效一次，receipt 如实区分「已记录 / 已记录但外部派发未知 / 已被他方决定 / 已失效」，绝不把决定 ACK 显示成外部派发完成。

**Architecture:** Go 侧不新增第二套交互聚合——`workbench_interactions` 表已是唯一 Interaction 身份投影，本计划补一条跨 run 的收件箱读（`GET /api/v1/workbench/interactions`，复用 overview 的归档任务 JOIN 谓词，owner/tenant 谓词不变），并把 `ErrCommandRecoveryUnknown` 从无名 500 升级为带机器可读 code 的 502（决定已落地、外部派发未知）。多设备只生效一次与幂等重放由既有 `GormInteractionStore.Decide` 行锁 + decision_id/revision CAS 保证（`internal/modules/workbench/service/workbench/interaction.go:224-284`），本计划补 HTTP 并发黑盒证据。客户端在 `packages/mobile-core` Task Office 上新增 `inbox()`/`decide()`（新文件 `attention-inbox.ts` 承载域逻辑：kind×action 冻结矩阵本地校验、`decision_id` 冻结重放、coded 错误 → receipt 分类），`InteractionBackendPort` 由 api-client 的 `createTaskOfficeRemote` 新增 `inbox`/`decide` 方法实现（同一 remote 结构实现 backend/detail/interactions 三端口，沿 #35 先例），409/400→`INTERACTION_SUPERSEDED`、502+`command_recovery_unknown`→`INTERACTION_DELIVERY_UNKNOWN`、404/403/410→`INTERACTION_GONE` 的跨包契约码错误。apps/mobile 新增收件箱屏（receipt 文案如实）、`/inbox` 路由与 Home 常驻入口。顺带修复既有缺陷：`createInteractionsApi` 路径缺 `/api/v1` 前缀（零消费者，无兼容风险）。

**Tech Stack:** Go 1.26（gin + gorm，sqlite 测试 / postgres 生产，无新迁移）、TypeScript（`packages/contracts`、`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN）、node:test + tsx、testify。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置：`pnpm install` 已完成（作者已实跑基线全绿，见「基线证据」）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-38.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Implementation Decisions、Testing Decisions）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§5.1 Task Office 所有权「Home、Task list、Attention inbox 的一致读投影……运行干预、Interaction decision、预算扩展」、§5.2 Interface、§10 App Shell 禁止项、§4.2 Scope Lease）
- ADR：`docs/adr/0004-task-is-session.md`（taskId = sessionId）、`docs/adr/0006-mobile-transport-by-semantics.md`（REST 提交命令）、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`（深模块承载业务逻辑）
- 领域术语：`CONTEXT.md`（「关注状态（Attention State）」「运行（Run）」「任务生命周期（Task Lifecycle）」）
- Parent：Issue #30；Blocked by：#34（T04，已合并）、#35（T05，已合并）
- 前序批次产出（本计划 Consumes，全部在当前 HEAD 亲眼核实）：#34 的 `createTaskOffice`/`TaskOfficePorts`/`TaskOffice`/`TaskOfficeError`（`packages/mobile-core/src/task-office/task-office.ts:100-239`）、`MobileRuntime.authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>`（`packages/mobile-core/src/runtime/mobile-runtime.ts:383`）、`createTaskOfficeRemote({ origin, request, stream? })`（`packages/api-client/src/mobile/task-office.ts:55`）、`taskOfficeFor(runtime, origin)` 工厂与 `activeTaskOffice()`（`apps/mobile/src/composition.ts:109-144`）、`taskOfficeIntegrationConfig(env)` opt-in 模式（`apps/mobile/src/task-office-integration-smoke.ts:22-37`）；#35 的 `RuntimeScopeLease`/`leaseActive`（`packages/mobile-core/src/runtime/scope-lease.ts`）、`TaskDetailBackendPort` fail-closed 先例与 `TASK_STREAM_CURSOR_EXPIRED` 跨包契约码先例（`packages/api-client/src/mobile/task-office.ts:130-163`）、`taskDetailIntegrationConfig` 的 `disallowedDeploymentHost` 主机防线复用模式（`apps/mobile/src/task-detail-integration-smoke.ts:29-48`）。

## Global Constraints

以下为批准 Spec / ADR / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「WeKnora remains the only business authority. The mobile client does not create parallel identities, Tasks, approvals, balances, credentials or execution state.」（mobile-ai-office-design.md · Implementation Decisions）
- 「Task Office owns Home/Task projections, durable submission identity, reconciliation, Snapshot/SSE recovery, intervention, decisions, budget and Task lifecycle.」（同上）
- 「Each command that can have an unknown outcome uses a durable idempotency identity. Network failure triggers lookup or reconciliation, not silent replay.」（同上）
- 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」（同上）
- 「Offline mode permits approved reads, drafts and annotations. It prohibits Run commands, approval, budget expansion and external Actions.」（同上）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上 · Testing Decisions）
- 「Task Office Interface tests cover durable request identity, lost acknowledgements, unknown reconciliation, Snapshot hydration, SSE gaps, cursor expiry, single-writer admission, intervention routing, decision CAS and three-dimensional state projection.」（同上）
- 「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, partial external success, unknown outcomes and durable checkpoints.」（同上）
- 「Remote-owned dependencies use in-memory scenario Adapters for Module tests and real HTTP/SSE contract tests for production Adapters.」（同上）
- 「移动 AI Office 将现有 WeKnora Session 呈现为 Task，并保持 `taskId = sessionId`，不新增第二个 Task 聚合身份。」（ADR-0004）
- 「Task Office 是移动产品的主 Module，唯一拥有：Home、Task list、Attention inbox 的一致读投影……运行干预、Interaction decision、预算扩展」（mobile-module-seams.md §5.1）
- 「Screen 不调用 start、lookup、snapshot、events、interaction、command 等多个 wire 方法。Module 内部决定顺序、幂等、重连、revision 和错误呈现。」（mobile-module-seams.md §5.2）
- 「禁止：Screen 直接导入 packages/contracts 或 packages/api-client；Screen 自己维护 request_id、cursor、revision、scope generation；每个 Screen 建独立 query cache 或 token refresh」（mobile-module-seams.md §10）
- 「Interface 不暴露 token、query key、generation number 或 SecureStore key。Scope Lease 是不透明、可撤销的能力对象，子 Module 每次异步提交前检查其有效性。」（mobile-module-seams.md §4.2）
- 「TaskHandle 只暴露：snapshot()……act(TaskIntent)：执行 steer、queue-next、stop、decision、share、archive 等受控意图……」（mobile-module-seams.md §5.2）——本计划交付 Task Office 模块级的 `inbox()`/`decide()`（Home/收件箱路径，§5.1 所有权归属 Task Office）；`TaskHandle.act(TaskIntent)` 的 decision 意图属 #37，两者共享同一 `InteractionBackendPort`。
- 安全约束（Mimosa）：服务端 SQL 一律参数绑定（本计划所有新查询均为 `?` 占位 + gorm 绑定）；服务端请求仅 http/https 且 host 校验拒绝 localhost/环回/私网/保留地址（集成冒烟复用 `disallowedDeploymentHost` 防线）；凭据只从环境变量读取（`WEKNORA_MOBILE_TEST_*` opt-in，无回退凭据，源码/测试不写入可用凭据字面量）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #38 验收标准原文（docs/plans/issue30-sweep/issues/issue-38.md）：**

1. 「多设备重复决定只生效一次。」
2. 「审批成功不被误显示成外部派发完成。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

**验收标准 3 的本地可验证性说明（blocked-env 声明）：** 真正的多设备端到端（两台真实设备各自登录同一账号、经真实移动 App 对同一 pending 交互并发提交决定）需要「真机 ×2 + 一个真实 WeKnora Deployment（HTTPS origin）+ 一个测试账号 + 一条真实 pending 审批」，本地无法验证，**不伪造通过**。本地替代证据（按 Interface 层级从高到低）：
- **服务端最高稳定 Interface（真实 HTTP）**：Task 2 的黑盒测试——真实 gin 路由 + 真实 `GormInteractionStore` + 真实 `Service` + 真实 handler，文件级 sqlite（busy_timeout），两 goroutine 并发 POST 不同 `decision_id` 恰一 200 一 409（AC1）；真实 approval Gate + 失败 remote port 下决定落地但 502 `command_recovery_unknown`（AC2）；同 `decision_id` 重放幂等 200。
- **移动模块最高稳定 Interface**：Task 5 的 Task Office Interface 测试（真实模块编排 + 脚本化 backend Adapter，属 seams §4.4 认可的 in-memory Adapter 类别）覆盖 decision_id 冻结重放、delivery-unknown → 同 id 重试 → recorded、矩阵本地校验、scope fail-closed。
- **opt-in 真实部署证据**：Task 7 的集成冒烟（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` + 决定动作额外门控 `WEKNORA_MOBILE_TEST_DECIDE_INTERACTION=1`），具备环境时自动产出端到端证据，无环境时如实 `skipped`。

**与调查结论的差异记录（以代码现状为准）：**

1. **调查未提及的既有 wire 缺陷**：`packages/api-client/src/mobile/interactions.ts:57,69` 的路径为 `/workbench/executions/...`，缺 `/api/v1` 前缀——真实挂载在 v1 组（`internal/router/router.go:374` → `RegisterWorkbenchCommandRoutes`，路由声明测试 `internal/router/routes_workbench_start_test.go:36-37` 断言 `/api/v1/workbench/executions/...`）。因该 SDK 至今零消费者而未暴露。本计划 Task 4 修正（对比同目录 `executions.ts:267` 的 `/api/v1/workbench/executions` 先例）。
2. 调查称「InteractionRecord 无 run_id」——属实（`packages/contracts/src/mobile/interactions.ts:18-26`）。本计划不改冻结的 `InteractionRecord`，新增 `InboxInteractionRecord = InteractionRecord & { run_id: string; created_at?: string }`（新文件），Go 侧 `workbench.InteractionDecision` 追加输出字段 `created_at,omitempty`（additive，决定请求体不受影响）。
3. 调查缺口「InteractionBudget/InteractionRecovery 无生产创建方」——属实（仅 `approval.Gate` 经 `CreatePending` 落 `tool_approval` 行，`service/workbench/interaction.go:173-187`；预算走 `/commercial/tasks/:id/budget/extend`，恢复走 `/sessions/:id/runs/:run_id/decisions`）。本计划**不**替 commercial/chat 模块创建 budget/recovery 行（模块归属与 Write Run 单写者语义属其域）；本计划交付的是「同一 Interaction 身份的展示与决定闭环」：收件箱与决定对三类 kind 类型化处理（冻结矩阵 + 类型化 receipt），这些模块一旦落行即自然出现在收件箱。Go `Service.Decide` 对 budget/recovery 目前仅 CAS 记录、无下游派发（`interaction.go:517-541` 仅 tool_approval 走 gate+remote）——AC2 语义（不冒充外部派发成功）由 receipt 设计统一表达，本计划不为 budget/recovery 增加派发。
4. 调查称「多设备语义仅由服务层并发测试覆盖」——属实（`interaction_test.go:110-154`）。本计划 Task 2 把并发证据提升到真实 HTTP 层（AC1/AC3）。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **双击/重试重复提交同一决定**：用户连点「批准」或网络超时后重试，若每次生成新 `decision_id`，第二次会撞 409 且第一决定被误报冲突。期望：同一 item+action 复用冻结的 `decision_id`，重放幂等 200。——Task 5 测试「decide freezes the decision id: repeats and retries replay the same durable identity」+ Task 2 HTTP 幂等重放断言。
2. **决定已落地但外部派发未知被当失败重发新决定**：502 `command_recovery_unknown` 若被当成普通失败，UI 会显示「失败」诱导用户换 id 重发（撞 409）或误以为未生效。期望：receipt `delivery-unknown`，重试复用同一 `decision_id`，服务端重放 remote 提交。——Task 5 测试「a delivery-unknown outcome keeps the decision id and a retry converts it to recorded」+ Task 2 测试 `TestWorkbenchDecisionRemoteFailureIsDeliveryUnknownNotSuccess`。
3. **过期/撤销/不存在的交互仍被提交**：收件箱行过期（410）、被撤销（403）、已删除（404）时，泛化为后端错误会诱导无意义重试。期望：receipt `gone` + 刷新引导。——Task 4 适配器分类测试 + Task 5 receipt 测试 + Task 1 收件箱不列出已过期行。
4. **收件箱列出不可决定行**：已归档任务的交互、已过期 prompt 若进入收件箱，用户点了必然 410/无效。期望：服务端读排除（归档任务 JOIN 与 overview 同谓词；过期行 Go 侧过滤）。——Task 1 测试 `TestGormInteractionStoreListPendingScopesOwnerAndExcludesUndecidableRows`。
5. **决定后的收件箱/首页读竞态**：决定落地瞬间旧 `inbox()`/`home()` 迟到返回会把已决定行重新显示为 pending。期望：决定终态 bump 读 epoch，迟到读按 `SUPERSEDED` 拒绝。——Task 5 测试「a decided interaction invalidates the home projection and late inbox reads」。

---

### Task 1: Go——`GormInteractionStore.ListPending` 跨 run 收件箱读与 `created_at` wire 字段

**Files:**
- Modify: `internal/modules/workbench/interaction.go:26-36`（`InteractionDecision` 追加输出字段）
- Modify: `internal/modules/workbench/service/workbench/interaction.go:151-153`（`decision()` 投影）、`:194-207` 之后（新增 `ListPending`）、`:461-470` 之后（新增 `Service.ListInbox`）
- Test: `internal/modules/workbench/service/workbench/inbox_test.go`（新建）

**Interfaces:**
- Consumes: 既有 `interactionRow`/`GormInteractionStore`/`identity(ctx)`/`validateInteractionRow`（`service/workbench/interaction.go`）；overview 的归档任务 JOIN 谓词先例（`service/workbench/overview.go:198-204`）。
- Produces: `GormInteractionStore.ListPending(ctx context.Context, tenantID uint64, ownerID string, limit int) ([]workbench.InteractionDecision, error)`；`(*Service).ListInbox(ctx context.Context, limit int) ([]workbench.InteractionDecision, error)`（store 未实现 `ListPending` 时返回 `ErrCapabilityUnavailable`，fail closed）；`workbench.InteractionDecision` 新增输出字段 `CreatedAt string \`json:"created_at,omitempty"\``（list 投影携带 RFC3339 时间戳；决定请求体忽略该字段）。Task 2 的 handler 消费这两者。

- [ ] **Step 1: 写失败测试**

新建 `internal/modules/workbench/service/workbench/inbox_test.go`：

```go
package workbench

import (
	"context"
	"testing"
	"time"

	contract "github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 收件箱读的物理列集与 overview 共用（agent_runs/sessions 投影行），防自造 schema 掩盖缺列。
type inboxRunRow struct {
	TenantID  uint64
	RunID     string
	SessionID string
	OwnerID   string
	Status    string
	UpdatedAt time.Time
}

func (inboxRunRow) TableName() string { return "agent_runs" }

type inboxSessionRow struct {
	TenantID   uint64
	ID         string
	Title      string
	ArchivedAt *time.Time
}

func (inboxSessionRow) TableName() string { return "sessions" }

func TestGormInteractionStoreListPendingScopesOwnerAndExcludesUndecidableRows(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/inbox-pending.db?_foreign_keys=on&_busy_timeout=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&interactionRow{}, &inboxRunRow{}, &inboxSessionRow{}))
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	require.NoError(t, db.Create([]*inboxSessionRow{
		{TenantID: 7, ID: "s-live", Title: "live task"},
		{TenantID: 7, ID: "s-archived", Title: "archived task", ArchivedAt: &now},
	}).Error)
	require.NoError(t, db.Create([]*inboxRunRow{
		{TenantID: 7, RunID: "run-live", SessionID: "s-live", OwnerID: "u1", Status: "waiting_user", UpdatedAt: now},
		{TenantID: 7, RunID: "run-archived", SessionID: "s-archived", OwnerID: "u1", Status: "running", UpdatedAt: now},
		{TenantID: 7, RunID: "run-dangling", SessionID: "s-missing", OwnerID: "u1", Status: "running", UpdatedAt: now},
	}).Error)
	rows := []*interactionRow{
		// 应列出：本人、待处理、活任务、未过期（dangling run 的 LEFT JOIN 使 archived_at 为 NULL，与 overview 同语义，保持可见）
		{TenantID: 7, ID: "i-live", RunID: "run-live", OwnerID: "u1", Kind: "tool_approval", ArgsHash: "h1", Status: "pending", CreatedAt: now, UpdatedAt: now},
		{TenantID: 7, ID: "i-dangling", RunID: "run-dangling", OwnerID: "u1", Kind: "recovery", ArgsHash: "h2", Status: "pending", CreatedAt: now.Add(time.Minute), UpdatedAt: now},
		// 应排除：已归档任务
		{TenantID: 7, ID: "i-archived", RunID: "run-archived", OwnerID: "u1", Kind: "budget", ArgsHash: "h3", Status: "pending", CreatedAt: now, UpdatedAt: now},
		// 应排除：已过期（不可决定）
		{TenantID: 7, ID: "i-expired", RunID: "run-live", OwnerID: "u1", Kind: "tool_approval", ArgsHash: "h4", Status: "pending", ExpiresAt: &past, CreatedAt: now, UpdatedAt: now},
		// 应排除：已解决
		{TenantID: 7, ID: "i-resolved", RunID: "run-live", OwnerID: "u1", Kind: "tool_approval", ArgsHash: "h5", Status: "resolved", CreatedAt: now, UpdatedAt: now},
		// 应排除：他人（owner 隔离）
		{TenantID: 7, ID: "i-other-owner", RunID: "run-live", OwnerID: "u2", Kind: "tool_approval", ArgsHash: "h6", Status: "pending", CreatedAt: now, UpdatedAt: now},
	}
	for _, row := range rows {
		require.NoError(t, db.Create(row).Error)
	}
	store := NewGormInteractionStore(db)
	items, err := store.ListPending(context.Background(), 7, "u1", 50)
	require.NoError(t, err)
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	require.Equal(t, []string{"i-live", "i-dangling"}, ids, "created_at ASC 排序；归档任务/过期/已解决/他人行一律不进收件箱")
	require.Equal(t, "run-live", items[0].RunID)
	require.Equal(t, "h1", items[0].ArgsHash)
	require.NotEmpty(t, items[0].CreatedAt, "收件箱行携带 created_at 供展示与排序核对")

	// limit 钳制：非正数与超上限回退默认 50（不放大租户读）。
	_, err = store.ListPending(context.Background(), 7, "u1", 0)
	require.NoError(t, err)
	_, err = store.ListPending(context.Background(), 7, "u1", 9999)
	require.NoError(t, err)
}

func TestInteractionServiceListInboxScopesIdentityAndFailsClosed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:w08_inbox_service?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// ListPending JOIN agent_runs/sessions（F1）：迁移必须一并建表，否则 SQLite 报 no such table。
	require.NoError(t, db.AutoMigrate(&interactionRow{}, &inboxRunRow{}, &inboxSessionRow{}))
	require.NoError(t, db.Create(&interactionRow{TenantID: 7, ID: "i1", RunID: "r1", OwnerID: "web_user:u1", Kind: "tool_approval", ArgsHash: "h", Status: "pending"}).Error)
	svc := NewInteractionService(NewGormInteractionStore(db), nil, nil)
	items, err := svc.ListInbox(interactionContext(), 50)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "i1", items[0].ID)
	_, err = svc.ListInbox(context.Background(), 50)
	require.Error(t, err, "身份上下文缺失必须拒绝，不回退到全局读")

	// store 未实现 ListPending（如内存桩）→ fail closed，不静默返回空。
	stub := &interactionStoreStub{current: contract.InteractionDecision{ID: "i1", Kind: "budget", ArgsHash: "a"}}
	_, err = NewInteractionService(stub, nil, nil).ListInbox(interactionContext(), 50)
	require.ErrorIs(t, err, ErrCapabilityUnavailable)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/workbench/service/workbench/ -run 'TestGormInteractionStoreListPending|TestInteractionServiceListInbox' -count=1`
Expected: FAIL——编译错误 `store.ListPending undefined` / `svc.ListInbox undefined`（type `*GormInteractionStore` has no field or method `ListPending`）。

- [ ] **Step 3: 最小实现**

3a. `internal/modules/workbench/interaction.go`——`InteractionDecision` 结构体在 `CredentialVersion` 字段后追加：

```go
	CredentialVersion int64  `json:"credential_version,omitempty"`
	// CreatedAt is an output-only projection field carried by list reads
	// (RFC3339). Request bodies ignore it; the service never trusts it.
	CreatedAt string `json:"created_at,omitempty"`
```

3b. `internal/modules/workbench/service/workbench/interaction.go`——`decision()`（:151-153）替换为：

```go
func (r interactionRow) decision() workbench.InteractionDecision {
	return workbench.InteractionDecision{ID: r.ID, RunID: r.RunID, DecisionID: r.DecisionID, Kind: r.Kind, Action: r.Action, ArgsHash: r.ArgsHash, ExpectedRevision: r.ExpectedRevision, ExternalPendingID: r.ExternalPendingID, CredentialVersion: r.CredentialVersion, CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339)}
}
```

3c. 同文件 `List` 方法（:194-207）之后新增：

```go
// ListPending returns the owner's open interactions across runs: the Attention
// Inbox read (T08). It reuses the overview's archived-task LEFT JOIN so the
// inbox and the home projection never disagree, and expired prompts are
// filtered in Go (dialect-safe) so the inbox never offers an undecidable row.
// Interactions whose run row is dangling stay visible: the LEFT JOIN keeps
// sessions.archived_at NULL, matching the overview semantics exactly.
func (s *GormInteractionStore) ListPending(ctx context.Context, tenantID uint64, ownerID string, limit int) ([]workbench.InteractionDecision, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []interactionRow
	err := s.db.WithContext(ctx).
		Table("workbench_interactions").
		Select("workbench_interactions.*").
		Joins("LEFT JOIN agent_runs ar ON ar.tenant_id = workbench_interactions.tenant_id AND ar.run_id = workbench_interactions.run_id").
		Joins("LEFT JOIN sessions ON sessions.tenant_id = ar.tenant_id AND sessions.id = ar.session_id").
		Where("workbench_interactions.tenant_id = ? AND workbench_interactions.owner_id = ? AND workbench_interactions.status = ? AND sessions.archived_at IS NULL", tenantID, ownerID, "pending").
		Order("workbench_interactions.created_at ASC, workbench_interactions.id ASC").
		Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]workbench.InteractionDecision, 0, len(rows))
	for _, row := range rows {
		if row.ExpiresAt != nil && time.Now().After(*row.ExpiresAt) {
			continue
		}
		if err := validateInteractionRow(row); err != nil {
			return nil, err
		}
		out = append(out, row.decision())
	}
	return out, nil
}
```

3d. 同文件 `Service.List`（:461-470）之后新增：

```go
// pendingLister is the narrowed inbox capability: only durable stores that can
// serve the cross-run pending read answer it; anything else fails closed.
type pendingLister interface {
	ListPending(ctx context.Context, tenantID uint64, ownerID string, limit int) ([]workbench.InteractionDecision, error)
}

// ListInbox serves the Attention Inbox: the caller's open interactions across
// runs, ordered oldest-first. Identity always comes from the context.
func (s *Service) ListInbox(ctx context.Context, limit int) ([]workbench.InteractionDecision, error) {
	tenant, owner, err := identity(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil || s.store == nil {
		return nil, ErrInteractionNotFound
	}
	lister, ok := s.store.(pendingLister)
	if !ok {
		return nil, ErrCapabilityUnavailable
	}
	return lister.ListPending(ctx, tenant, owner, limit)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/modules/workbench/service/workbench/ -run 'TestGormInteractionStoreListPending|TestInteractionServiceListInbox' -count=1`
Expected: PASS（2 个测试）。

- [ ] **Step 5: 回归既有面**

Run: `go test ./internal/modules/workbench/service/workbench/ -count=1`
Expected: PASS（全包，含既有 CAS/重试测试——作者基线实跑 ok 11.848s）。

- [ ] **Step 6: Commit**

```bash
git add internal/modules/workbench/interaction.go internal/modules/workbench/service/workbench/interaction.go internal/modules/workbench/service/workbench/inbox_test.go
git commit -m "feat(workbench): cross-run pending interaction read (attention inbox store+service, T08)"
```

---

### Task 2: Go——收件箱 HTTP 端点、delivery-unknown wire 与多设备并发黑盒证据

**Files:**
- Modify: `internal/handler/session/workbench_commands.go:44-55` 之后（新增 handler）、`:99-124`（`writeWorkbenchCommandError` 增 502 分支）、imports（`strconv`）
- Modify: `internal/router/routes_workbench.go:156-170`（`RegisterWorkbenchCommandRoutes` 增收件箱路由）
- Modify: `internal/router/routes_workbench_start_test.go:27-39`（路由声明断言）
- Test: `internal/handler/session/workbench_inbox_test.go`（新建）

**Interfaces:**
- Consumes: Task 1 的 `Service.ListInbox`；既有 `commandContext`/`writeWorkbenchCommandError`（`workbench_commands.go:28-42,99-124`）、`workbenchserviceInteractionRow` 测试投影（`workbench_commands_test.go:183-194`，同包可复用）。
- Produces: `GET /api/v1/workbench/interactions?limit=`（Viewer + chat API-key 边界；响应 `{success:true,data:[]workbench.InteractionDecision}`，行含 `run_id`/`created_at`）；`(*WorkbenchCommandHandler).ListInboxInteractions(c *gin.Context)`；决定/命令错误 wire：`ErrCommandRecoveryUnknown` → HTTP 502 + body `{success:false, code:"command_recovery_unknown", error}`（既有 500/409/404/403/410 映射不变）。Task 4 的 api-client 消费此 wire。

- [ ] **Step 1: 写失败测试**

新建 `internal/handler/session/workbench_inbox_test.go`：

```go
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/approval"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// agent_runs 只读投影（ListPending 的 LEFT JOIN 需要；本包私有，不越包引用 service 测试行）。
type inboxAgentRunRow struct {
	TenantID  uint64
	RunID     string
	SessionID string
	OwnerID   string
	Status    string
	UpdatedAt time.Time
}

func (inboxAgentRunRow) TableName() string { return "agent_runs" }

func inboxRouter(h *WorkbenchCommandHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/workbench/interactions", func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Set(types.UserIDContextKey.String(), "u1")
		c.Set(types.PrincipalContextKey.String(), types.Principal{Type: types.PrincipalWebUser, ID: "u1"})
		h.ListInboxInteractions(c)
	})
	r.POST("/api/v1/workbench/executions/interactions/:id/decisions", func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Set(types.UserIDContextKey.String(), "u1")
		c.Set(types.PrincipalContextKey.String(), types.Principal{Type: types.PrincipalWebUser, ID: "u1"})
		h.DecideInteraction(c)
	})
	return r
}

func newInboxTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "inbox-http.db") + "?_foreign_keys=on&_busy_timeout=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetMaxIdleConns(2)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&workbenchserviceInteractionRow{}, &inboxAgentRunRow{}))
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS sessions (tenant_id INTEGER, id TEXT, title TEXT, archived_at DATETIME)`).Error)
	return db
}

// AC3（服务端最高稳定 Interface）：真实路由 + 真实 GormInteractionStore + 真实 Service。
func TestWorkbenchInboxListsPendingAcrossRunsAndScopesOwner(t *testing.T) {
	db := newInboxTestDB(t)
	require.NoError(t, db.Create([]*workbenchserviceInteractionRow{
		{TenantID: 7, ID: "i-1", RunID: "run-1", OwnerID: "web_user:u1", Kind: "tool_approval", ArgsHash: "h1", Status: "pending", ExpectedRevision: 3},
		{TenantID: 7, ID: "i-2", RunID: "run-2", OwnerID: "web_user:u1", Kind: "recovery", ArgsHash: "h2", Status: "pending", ExpectedRevision: 0},
		{TenantID: 7, ID: "i-3", RunID: "run-1", OwnerID: "web_user:u2", Kind: "budget", ArgsHash: "h3", Status: "pending", ExpectedRevision: 0},
	}).Error)
	h := NewWorkbenchCommandHandler(workbenchservice.NewInteractionService(workbenchservice.NewGormInteractionStore(db), nil, nil))
	r := inboxRouter(h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/workbench/interactions?limit=10", nil))
	require.Equal(t, http.StatusOK, w.Code)
	// data 是数组：逐行按 map 断言（避免双层泛型结构体）
	var raw struct {
		Success bool             `json:"success"`
		Data    []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	require.True(t, raw.Success)
	require.Len(t, raw.Data, 2, "owner 隔离：u2 的行不进 u1 的收件箱")
	require.Equal(t, "i-1", raw.Data[0]["id"])
	require.Equal(t, "run-1", raw.Data[0]["run_id"])
	require.Equal(t, "tool_approval", raw.Data[0]["kind"])
	require.Equal(t, "h1", raw.Data[0]["args_hash"])
	require.Equal(t, float64(3), raw.Data[0]["expected_revision"])
	require.NotEmpty(t, raw.Data[0]["created_at"], "收件箱行携带 created_at（Task 1 输出字段贯通到 wire）")
}

// AC1（多设备并发只生效一次）+ 幂等重放：真实 HTTP 并发 POST。
func TestWorkbenchDecisionHTTPConcurrentDevicesDecideExactlyOnce(t *testing.T) {
	db := newInboxTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO workbench_interactions (tenant_id,id,run_id,owner_id,kind,args_hash,status,expected_revision,created_at,updated_at) VALUES (7,'i-c','run-c','web_user:u1','budget','hash','pending',0,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	service := workbenchservice.NewInteractionService(workbenchservice.NewGormInteractionStore(db), nil, nil)
	h := NewWorkbenchCommandHandler(service)
	r := inboxRouter(h)
	post := func(decisionID string) *httptest.ResponseRecorder {
		body := `{"kind":"budget","action":"extend","args_hash":"hash","decision_id":"` + decisionID + `","expected_revision":0}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions/interactions/i-c/decisions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- post(fmt.Sprintf("d%d", i)).Code
		}(i)
	}
	wg.Wait()
	close(results)
	var ok, conflict int
	for code := range results {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		default:
			t.Fatalf("unexpected status %d", code)
		}
	}
	require.Equal(t, 1, ok, "两台设备并发决定：恰一成功")
	require.Equal(t, 1, conflict, "另一台必须得到 409 冲突而非第二个成功")
	var stored struct {
		DecisionID       string
		Action           string
		Status           string
		ExpectedRevision int64
	}
	require.NoError(t, db.Table("workbench_interactions").Select("decision_id, action, status, expected_revision").Where("id = ?", "i-c").Scan(&stored).Error)
	require.Equal(t, "resolved", stored.Status)
	require.Equal(t, int64(1), stored.ExpectedRevision, "恰一次 revision 前进")
	require.Contains(t, []string{"d0", "d1"}, stored.DecisionID)
	// 落败设备以胜者的 decision_id 重放（幂等重试语义）：200 且不再前进 revision。
	require.Equal(t, http.StatusOK, post(stored.DecisionID).Code)
	require.NoError(t, db.Table("workbench_interactions").Select("expected_revision").Where("id = ?", "i-c").Scan(&stored.ExpectedRevision).Error)
	require.Equal(t, int64(1), stored.ExpectedRevision, "重放不二次生效")
	// 决定后收件箱不再列出该行。
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/workbench/interactions", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "i-c")
}

type inboxRemoteInteraction struct {
	failures int
	calls    int
}

func (p *inboxRemoteInteraction) SubmitInteraction(context.Context, uint64, string, string, string, string, string, string, int64, int64) error {
	p.calls++
	if p.calls <= p.failures {
		return errors.New("provider unavailable")
	}
	return nil
}

// AC2（审批成功不被误显示成外部派发完成）：远程派发失败时决定保持落地，
// wire 如实返回 502 + code=command_recovery_unknown（绝非成功 envelope）；同 decision_id 重放走服务端重试。
func TestWorkbenchDecisionRemoteFailureIsDeliveryUnknownNotSuccess(t *testing.T) {
	db := newInboxTestDB(t)
	store := workbenchservice.NewGormInteractionStore(db)
	gate := approval.NewGate(&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 3}}, approvalHTTPChecker{}, nil)
	service := workbenchservice.NewInteractionServiceWithApproval(store, nil, nil, gate)
	remote := &inboxRemoteInteraction{failures: 1}
	service.SetRemoteInteractionPort(remote)
	h := NewWorkbenchCommandHandler(service)
	r := inboxRouter(h)
	bus := event.NewEventBus()
	pending := make(chan string, 1)
	bus.On(event.EventToolApprovalRequired, func(_ context.Context, evt event.Event) error {
		pending <- evt.Data.(event.ToolApprovalRequiredData).PendingID
		return nil
	})
	approvalCtx := types.WithPrincipal(context.Background(), types.Principal{Type: types.PrincipalWebUser, ID: "u1"})
	resolved := make(chan approval.Decision, 1)
	go func() {
		decision, err := gate.RequestAndWait(approvalCtx, approval.PendingRequest{TenantID: 7, CredentialVersion: 1, UserID: "u1", RunID: "run-a", RequestID: "request-a", EventBus: bus, Args: []byte(`{"x":1}`)})
		require.NoError(t, err)
		resolved <- decision
	}()
	id := <-pending
	var row struct{ ArgsHash string }
	require.NoError(t, db.Table("workbench_interactions").Select("args_hash").Where("id = ?", id).Scan(&row).Error)
	post := func() *httptest.ResponseRecorder {
		body := `{"kind":"tool_approval","action":"approve","args_hash":"` + row.ArgsHash + `","decision_id":"d-stable","expected_revision":0}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions/interactions/"+id+"/decisions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	// 第一次：remote 派发失败 → 502 + 机器可读 code；决定在库中已 resolved。
	first := post()
	require.Equal(t, http.StatusBadGateway, first.Code)
	var body struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
		Error   string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &body))
	require.False(t, body.Success, "绝不冒充外部派发成功")
	require.Equal(t, "command_recovery_unknown", body.Code)
	require.NotEmpty(t, body.Error)
	var status string
	require.NoError(t, db.Table("workbench_interactions").Select("status").Where("id = ?", id).Scan(&status).Error)
	require.Equal(t, "resolved", status, "决定保持落地：delivery-unknown 不回滚 CAS")
	// 第二次：同一 decision_id 重放 → 服务端重试 remote（本 stub 已恢复）→ 200。
	require.Equal(t, 1, remote.calls)
	second := post()
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, 2, remote.calls, "重放必须重发同一 durable provider 身份")
	require.True(t, (<-resolved).Approved)
}
```

（`post` 每次调用重建 `httptest.NewRequest`（含 body reader），避免复用已消费的 body。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/handler/session/ -run 'TestWorkbenchInbox|TestWorkbenchDecision' -count=1`
Expected: FAIL——编译错误 `h.ListInboxInteractions undefined`（`*WorkbenchCommandHandler` has no field or method `ListInboxInteractions`）；且 `TestWorkbenchDecisionRemoteFailureIsDeliveryUnknownNotSuccess` 在 handler 未实现 502 映射前不可能通过。

- [ ] **Step 3: 最小实现**

3a. `internal/handler/session/workbench_commands.go`——imports 增 `"strconv"`；`ListInteractions`（:44-55）之后新增：

```go
// ListInboxInteractions serves GET /api/v1/workbench/interactions: the
// caller's open interactions across runs (the Attention Inbox read, T08).
// limit is clamped server-side; identity always comes from the context.
func (h *WorkbenchCommandHandler) ListInboxInteractions(c *gin.Context) {
	if h == nil || h.interactions == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	limit := 50
	if parsed, err := strconv.Atoi(c.Query("limit")); err == nil && parsed > 0 && parsed <= 200 {
		limit = parsed
	}
	items, err := h.interactions.ListInbox(commandContext(c), limit)
	if err != nil {
		writeWorkbenchCommandError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}
```

3b. 同文件 `writeWorkbenchCommandError` 的 switch（:101 起）内、`case errors.Is(err, workbenchservice.ErrInteractionExpired)` 之前插入：

```go
	case errors.Is(err, workbenchservice.ErrCommandRecoveryUnknown):
		// The durable decision (or command) already landed; only the external
		// dispatch outcome is unknown. 502 + machine-readable code lets clients
		// render "recorded, delivery unconfirmed" instead of a generic failure
		// (T08 AC2) and replay the same decision_id for reconciliation.
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"success": false, "code": "command_recovery_unknown", "error": err.Error()})
		return
```

3c. `internal/router/routes_workbench.go`——`RegisterWorkbenchCommandRoutes`（:159-170）内、`workbench.POST("/:run_id/commands", h.Command)` 之后追加：

```go
	// T08 Attention Inbox: the owner's open interactions across runs. Same
	// Viewer/API-key boundary; the handler applies the tenant+owner predicate.
	inbox := g.apiKeyGroup(r.Group("/workbench/interactions", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	inbox.GET("", h.ListInboxInteractions)
```

3d. `internal/router/routes_workbench_start_test.go`——`TestRegisterWorkbenchCommandRoutesDeclaresTypedEndpoints`（:27-39）断言区追加一行：

```go
	require.True(t, seen[http.MethodGet+" /api/v1/workbench/interactions"])
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/handler/session/ -run 'TestWorkbenchInbox|TestWorkbenchDecision' -count=1`
Expected: PASS（3 个新测试 + 既有 2 个决定面测试：`TestWorkbenchDecisionHTTPUsesGormCASAndIdempotency`、`TestWorkbenchDecisionHTTPResumesDurableApprovalAndScopesIdentity`；`TestWorkbenchCommandDecision*` 不含 `TestWorkbenchDecision` 子串，不在本 pattern 内）。
Run: `go test ./internal/router/ -run 'TestRegisterWorkbenchCommandRoutesDeclaresTypedEndpoints' -count=1`
Expected: PASS。

- [ ] **Step 5: 回归 + 构建**

Run: `go test ./internal/handler/session/ -run 'TestWorkbench' -count=1 && go build ./internal/... ./cmd/...`
Expected: PASS + 构建成功。

- [ ] **Step 6: Commit**

```bash
git add internal/handler/session/workbench_commands.go internal/router/routes_workbench.go internal/router/routes_workbench_start_test.go internal/handler/session/workbench_inbox_test.go
git commit -m "feat(workbench): attention inbox endpoint, delivery-unknown wire code and concurrent-device HTTP proof (T08)"
```

---

### Task 3: contracts——`InboxInteractionRecord`（含 run_id/created_at 的收件箱行契约）

**Files:**
- Create: `packages/contracts/src/mobile/interaction-inbox.ts`
- Modify: `packages/contracts/src/index.ts:654-660`（interactions 导出块之后追加）
- Test: `packages/contracts/test/mobile-interaction-inbox.test.ts`（新建，匹配 `test:shared` 的 `packages/contracts/test/mobile-*.test.ts` glob）

**Interfaces:**
- Consumes: 冻结的 `parseInteraction`/`InteractionRecord`（`packages/contracts/src/mobile/interactions.ts:18-88`）、`ContractError`（`../index.ts`，同 interactions.ts 先例）。
- Produces: `export type InboxInteractionRecord = InteractionRecord & { run_id: string; created_at?: string }`；`export function parseInteractionWithRun(value: unknown): InboxInteractionRecord`（在冻结解析之上要求非空 `run_id`；`created_at` 可选透传）。Task 4 的 api-client 消费。

- [ ] **Step 1: 写失败测试**

新建 `packages/contracts/test/mobile-interaction-inbox.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseInteractionWithRun } from '@weknora/contracts';
import { ContractError } from '@weknora/contracts';

test('parseInteractionWithRun keeps the frozen interaction rules and requires run_id', () => {
  const row = parseInteractionWithRun({
    id: 'i-1', decision_id: '', kind: 'tool_approval', action: '', args_hash: 'sha256:aa',
    expected_revision: 4, run_id: 'run-1', created_at: '2026-09-24T00:00:00Z',
  });
  assert.equal(row.run_id, 'run-1');
  assert.equal(row.created_at, '2026-09-24T00:00:00Z');
  assert.equal(row.kind, 'tool_approval');

  // 冻结矩阵仍生效：budget 域携带 approve 拒绝
  assert.throws(() => parseInteractionWithRun({
    id: 'i-2', decision_id: 'd', kind: 'budget', action: 'approve', args_hash: 'h', expected_revision: 1, run_id: 'r',
  }), ContractError);

  // run_id 缺失/空串/非字符串：收件箱行不可导航、不可决定上下文，拒绝
  for (const runId of [undefined, '', '   ', 7]) {
    assert.throws(() => parseInteractionWithRun({
      id: 'i-3', decision_id: '', kind: 'recovery', action: '', args_hash: 'h', expected_revision: 0, run_id: runId,
    }), (error: unknown) => error instanceof ContractError, `run_id=${String(runId)}`);
  }

  // created_at 可选：老服务端不带该字段仍合法
  const bare = parseInteractionWithRun({
    id: 'i-4', decision_id: '', kind: 'budget', action: '', args_hash: 'h', expected_revision: 0, run_id: 'r2',
  });
  assert.equal(bare.created_at, undefined);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/contracts/test/mobile-interaction-inbox.test.ts`
Expected: FAIL——`SyntaxError: The requested module '@weknora/contracts' does not provide an export named 'parseInteractionWithRun'`。

- [ ] **Step 3: 最小实现**

新建 `packages/contracts/src/mobile/interaction-inbox.ts`：

```ts
import { ContractError, parseInteraction } from '../index.ts';
import type { InteractionRecord } from './interactions.ts';

/**
 * Attention Inbox 行契约（T08）：在 MX-003 冻结的 InteractionRecord 之上
 * 要求非空 run_id——收件箱必须能导航回任务并提供 args_hash/expected_revision
 * 决定上下文；created_at 为可选输出字段（Go list 投影在 T08 起携带）。
 * 不修改冻结的 InteractionRecord 本身。
 */
export type InboxInteractionRecord = InteractionRecord & { run_id: string; created_at?: string };

export function parseInteractionWithRun(value: unknown): InboxInteractionRecord {
  const record = parseInteraction(value);
  const row = (typeof value === 'object' && value !== null ? value : {}) as Record<string, unknown>;
  if (typeof row.run_id !== 'string' || row.run_id.trim() === '') {
    throw new ContractError('run_id', 'expected a non-empty string');
  }
  const createdAt = row.created_at;
  if (createdAt !== undefined && typeof createdAt !== 'string') {
    throw new ContractError('created_at', 'must be a string when present');
  }
  return createdAt === undefined ? { ...record, run_id: row.run_id } : { ...record, run_id: row.run_id, created_at };
}
```

`packages/contracts/src/index.ts` 在既有 interactions 导出块（`from './mobile/interactions.ts'` 的两段 export，:654-660 附近）之后追加：

```ts
export { parseInteractionWithRun } from './mobile/interaction-inbox.ts';
export type { InboxInteractionRecord } from './mobile/interaction-inbox.ts';
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/contracts/test/mobile-interaction-inbox.test.ts packages/contracts/test/mobile-interactions.test.ts`
Expected: PASS（新 1 + 既有 interactions 契约全绿）。

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/mobile/interaction-inbox.ts packages/contracts/src/index.ts packages/contracts/test/mobile-interaction-inbox.test.ts
git commit -m "feat(contracts): inbox interaction row contract with run_id (T08)"
```

---

### Task 4: api-client——修复 interactions 路径前缀、新增 `inbox()`，remote 实现 interactions 端口（含错误分类）

**Files:**
- Modify: `packages/api-client/src/mobile/interactions.ts:53-75`（路径修复 + `inbox()`）
- Modify: `packages/api-client/src/mobile/task-office.ts:1-16`（imports）、`:55-166`（remote 增 `inbox`/`decide`）
- Test: `packages/api-client/src/mobile/interactions.test.ts`（更新既有断言 + 新增 inbox 用例）
- Test: `packages/api-client/src/mobile/task-office.test.ts`（新增 remote inbox/decide 用例）

**Interfaces:**
- Consumes: Task 3 的 `parseInteractionWithRun`；既有 `createInteractionsApi`/`parseItems`（`interactions.ts:44-75`）、`ApiError`（`../errors.ts:24-38`）、`createTaskOfficeRemote`（`task-office.ts:55`）。
- Produces: `createInteractionsApi(request).inbox(limit?: number): Promise<InboxInteractionRecord[]>`（GET `/api/v1/workbench/interactions?limit=`）；`createTaskOfficeRemote` 返回对象新增 `inbox(): Promise<RemoteInboxItem[]>` 与 `decide(input: { item: RemoteInboxItem; decisionId: string; action: InteractionAction }): Promise<RemoteDecisionRecord>`（`action` 为 contracts `InteractionAction` 六值联合——与 mobile-core `InteractionActionValue` 同联合，保证 `ResolvedDecisionRecord` 结构可赋值），其中：
  - `RemoteInboxItem = { interactionId: string; runId: string; kind: 'tool_approval'|'budget'|'recovery'; argsHash: string; expectedRevision: number; createdAt: string }`
  - `RemoteDecisionRecord = { interactionId: string; runId: string; kind: 'tool_approval'|'budget'|'recovery'; decisionId: string; action: InteractionAction; argsHash: string; expectedRevision: number }`
  - 跨包契约码错误（Error 实例带 `code` 属性，沿 `TASK_STREAM_CURSOR_EXPIRED` 先例）：HTTP 409/400 → `INTERACTION_SUPERSEDED`；HTTP 502 且响应 code 为 `command_recovery_unknown` → `INTERACTION_DELIVERY_UNKNOWN`；HTTP 404/403/410 → `INTERACTION_GONE`。Task 5 的 mobile-core `InteractionBackendPort` 与这两个类型逐字一致（结构可赋值由 apps/mobile typecheck 证明）。

- [ ] **Step 1: 写失败测试**

1a. `packages/api-client/src/mobile/interactions.test.ts`——先更新既有用例「interactions list unwraps and validates pending records」中的路径断言（:28）：

```ts
  assert.deepEqual(seen[0].path, '/api/v1/workbench/executions/run%2F1/interactions');
```

再更新既有用例「decide posts the frozen decision body and validates the matrix locally」的 mock ack（:35）——`decide` 的返回解析本任务改为 `parseInteractionWithRun`（要求非空 `run_id`，2xx ack 自 Go `Service.Decide` 起恒携带），mock 须同步补字段（F2）：

```ts
    return { success: true, data: { id: 'i-1', decision_id: 'd-1', kind: 'tool_approval', action: 'approve', args_hash: 'sha256:aa', expected_revision: 5, run_id: 'run-1' } };
```

（第三段既有用例「decide rejects empty decision_id before any request」在本地预检即拒绝、不触发 ack 解析，无需改动。）

然后把新用例追加到文件末尾：
test('inbox unwraps pending rows across runs with run_id and validates limits', async () => {
  const seen: ClientRequest[] = [];
  const api = createInteractionsApi(async (input) => {
    seen.push(input);
    return {
      success: true,
      data: [
        { id: 'i-1', decision_id: '', kind: 'tool_approval', action: '', args_hash: 'sha256:aa', expected_revision: 4, run_id: 'run-1', created_at: '2026-09-24T00:00:00Z' },
        { id: 'i-2', decision_id: '', kind: 'recovery', action: '', args_hash: 'sha256:bb', expected_revision: 0, run_id: 'run-2' },
      ],
    };
  });
  const items = await api.inbox(25);
  assert.equal(items.length, 2);
  assert.equal(items[0].run_id, 'run-1');
  assert.equal(items[0].created_at, '2026-09-24T00:00:00Z');
  assert.equal(items[1].created_at, undefined);
  assert.deepEqual(seen[0].path, '/api/v1/workbench/interactions?limit=25');
  assert.deepEqual(seen[0].method, 'GET');

  // 缺 run_id 的行整包拒绝（收件箱行必须可导航）
  const broken = createInteractionsApi(async () => ({ success: true, data: [{ id: 'i-3', decision_id: '', kind: 'budget', action: '', args_hash: 'h', expected_revision: 0 }] }));
  await assert.rejects(broken.inbox(), (error: unknown) => error instanceof ContractError);

  // 非法 limit 本地拒绝，不发请求
  let sent = 0;
  const strict = createInteractionsApi(async () => { sent += 1; return { success: true, data: [] }; });
  await assert.rejects(strict.inbox(0), /limit/);
  await assert.rejects(strict.inbox(201), /limit/);
  assert.equal(sent, 0);
});
```

1b. `packages/api-client/src/mobile/task-office.test.ts`——文件末尾追加：

```ts
test('task office remote maps the inbox wire into module DTO rows', async () => {
  const requests: ClientRequest[] = [];
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return {
        success: true,
        data: [
          { id: 'i-1', decision_id: '', kind: 'tool_approval', action: '', args_hash: 'sha256:aa', expected_revision: 4, run_id: 'run-1', created_at: '2026-09-24T00:00:00Z' },
        ],
      };
    },
  });
  const items = await remote.inbox();
  assert.deepEqual(items, [{ interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval', argsHash: 'sha256:aa', expectedRevision: 4, createdAt: '2026-09-24T00:00:00Z' }]);
  assert.equal(requests[0].path, '/api/v1/workbench/interactions?limit=50');
});

test('task office remote decide maps the ack and classifies honest outcomes', async () => {
  const requests: ClientRequest[] = [];
  const ack = { id: 'i-1', decision_id: 'd-1', kind: 'tool_approval', action: 'approve', args_hash: 'sha256:aa', expected_revision: 5, run_id: 'run-1' };
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      if (requests.length === 1) return { success: true, data: ack };
      throw new ApiError({ status: 409, code: 'HTTP_409', message: 'conflict' });
    },
  });
  const item = { interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval' as const, argsHash: 'sha256:aa', expectedRevision: 4, createdAt: '' };
  const record = await remote.decide({ item, decisionId: 'd-1', action: 'approve' });
  assert.deepEqual(record, { interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval', decisionId: 'd-1', action: 'approve', argsHash: 'sha256:aa', expectedRevision: 5 });
  assert.deepEqual(requests[0].body, { id: 'i-1', decision_id: 'd-1', kind: 'tool_approval', action: 'approve', args_hash: 'sha256:aa', expected_revision: 4 });

  const coded = async (error: ApiError): Promise<string | undefined> => {
    const failing = createTaskOfficeRemote({ origin: 'https://weknora.example.test', request: async () => { throw error; } });
    try {
      await failing.decide({ item, decisionId: 'd-1', action: 'approve' });
      return undefined;
    } catch (caught) {
      return (caught as { code?: string }).code;
    }
  };
  assert.equal(await coded(new ApiError({ status: 409, code: 'HTTP_409', message: 'conflict' })), 'INTERACTION_SUPERSEDED');
  assert.equal(await coded(new ApiError({ status: 400, code: 'HTTP_400', message: 'interaction_action_mismatch' })), 'INTERACTION_SUPERSEDED');
  assert.equal(await coded(new ApiError({ status: 502, code: 'command_recovery_unknown', message: 'command_recovery_unknown: remote interaction' })), 'INTERACTION_DELIVERY_UNKNOWN');
  assert.equal(await coded(new ApiError({ status: 502, code: 'HTTP_502', message: 'upstream broke' })), undefined, '非 command_recovery_unknown 的 502 不得伪装成 delivery-unknown');
  assert.equal(await coded(new ApiError({ status: 410, code: 'HTTP_410', message: 'interaction_expired' })), 'INTERACTION_GONE');
  assert.equal(await coded(new ApiError({ status: 404, code: 'HTTP_404', message: 'not found' })), 'INTERACTION_GONE');
  assert.equal(await coded(new ApiError({ status: 403, code: 'HTTP_403', message: 'revoked' })), 'INTERACTION_GONE');
  assert.equal(await coded(new ApiError({ status: 500, code: 'HTTP_500', message: 'boom' })), undefined, '未知错误原样上抛（由上层折叠为后端错误）');
});
```

同文件头部 imports 追加：

```ts
import { ApiError } from '../errors.ts';
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/api-client/src/mobile/interactions.test.ts packages/api-client/src/mobile/task-office.test.ts`
Expected: FAIL——`api.inbox is not a function` / `remote.inbox is not a function`；既有路径断言在修复前为 `/workbench/executions/...`（deepEqual 不匹配）。

- [ ] **Step 3: 最小实现**

3a. `packages/api-client/src/mobile/interactions.ts`——头部 import 增：

```ts
import { parseInteractionWithRun, type InboxInteractionRecord } from '@weknora/contracts';
```

`list` 与 `decide` 的 `path` 改为带 `/api/v1` 前缀（真实挂载于 v1 组，`internal/router/router.go:374`）：

```ts
      const data = unwrap(await request({ method: 'GET', path: `/api/v1/workbench/executions/${pathId(runID, 'runID')}/interactions` }));
```

```ts
        path: `/api/v1/workbench/executions/interactions/${pathId(input.id, 'id')}/decisions`,
```

并在 `createInteractionsApi` 返回对象中追加（`decide` 之后）：

```ts
    /** GET /api/v1/workbench/interactions —— Attention Inbox：本人跨 run 待处理交互（T08）。 */
    async inbox(limit?: number): Promise<InboxInteractionRecord[]> {
      const value = typeof limit === 'number' ? limit : 50;
      if (!Number.isSafeInteger(value) || value < 1 || value > 200) {
        throw new Error('limit must be a safe integer between 1 and 200');
      }
      const data = unwrap(await request({ method: 'GET', path: `/api/v1/workbench/interactions?limit=${value}` }));
      if (!Array.isArray(data)) throw new ContractError('items', 'expected an array');
      return data.map((item) => parseInteractionWithRun(item));
    },
```

3b. `packages/api-client/src/mobile/task-office.ts`——头部 import 增：

```ts
import { createInteractionsApi } from './interactions.ts';
import { ApiError } from '../errors.ts';
import type { InteractionAction } from '@weknora/contracts';
```

在 `TaskOfficeStreamOption` 类型（:34）之后追加类型（`action` 收窄为 `InteractionAction` 字面量联合——与 mobile-core `InteractionActionValue` 同一六值联合；`string` 会破坏与 `ResolvedDecisionRecord.action: InteractionActionValue` 的结构可赋值性，F3）：

```ts
/** 与 mobile-core InteractionBackendPort 的 InboxItem 逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface RemoteInboxItem {
  interactionId: string;
  runId: string;
  kind: 'tool_approval' | 'budget' | 'recovery';
  argsHash: string;
  expectedRevision: number;
  createdAt: string;
}
/** 与 mobile-core InteractionBackendPort 的 ResolvedDecisionRecord 逐字一致（action 为六值联合，非 string）。 */
export interface RemoteDecisionRecord {
  interactionId: string;
  runId: string;
  kind: 'tool_approval' | 'budget' | 'recovery';
  decisionId: string;
  action: InteractionAction;
  argsHash: string;
  expectedRevision: number;
}
```

`createTaskOfficeRemote` 内（`const executionsApi = ...` 之后）追加：

```ts
  const interactionsApi = createInteractionsApi(request);
```

返回对象中（`stream` 方法之后）追加：

```ts
    async inbox(): Promise<RemoteInboxItem[]> {
      const rows = await interactionsApi.inbox(50);
      return rows.map((row) => ({
        interactionId: row.id,
        runId: row.run_id,
        kind: row.kind,
        argsHash: row.args_hash,
        expectedRevision: row.expected_revision,
        createdAt: row.created_at ?? '',
      }));
    },
    async decide(input: { item: RemoteInboxItem; decisionId: string; action: InteractionAction }): Promise<RemoteDecisionRecord> {
      try {
        const ack = await interactionsApi.decide({
          id: input.item.interactionId,
          decision_id: input.decisionId,
          kind: input.item.kind,
          action: input.action,
          args_hash: input.item.argsHash,
          expected_revision: input.item.expectedRevision,
        });
        // 2xx ack 的 decision_id 非空 ⇒ action 恒为矩阵内具体动作；空串只可能出现在 pending 行，
        // 此处窄化仅为类型收敛（F3），运行时行为不变。
        if (ack.action === '') throw new Error('decide ack must carry a concrete action');
        return {
          interactionId: ack.id,
          runId: ack.run_id,
          kind: ack.kind,
          decisionId: ack.decision_id,
          action: ack.action,
          argsHash: ack.args_hash,
          expectedRevision: ack.expected_revision,
        };
      } catch (error) {
        if (error instanceof ApiError) {
          const coded = (message: string): Error => {
            const translated = new Error(message);
            // 跨包契约码（沿 TASK_STREAM_CURSOR_EXPIRED 先例）：mobile-core 按 error.code 分类，不得改名。
            (translated as unknown as { code?: string }).code = message;
            throw translated;
          };
          if (error.status === 409 || error.status === 400) coded('INTERACTION_SUPERSEDED');
          if (error.status === 502 && error.code === 'command_recovery_unknown') coded('INTERACTION_DELIVERY_UNKNOWN');
          if (error.status === 404 || error.status === 403 || error.status === 410) coded('INTERACTION_GONE');
        }
        throw error;
      }
    },
```

注意：`decide` 的 ack 解析需携带 `run_id`——把 `interactions.ts` 中 `decide` 的返回解析从 `parseInteraction(data)` 改为 `parseInteractionWithRun(data)`（2xx ack 自 Go `Service.Decide` 起恒带 `run_id`，见 `service/workbench/interaction.go:502-508`）：

```ts
      return parseInteractionWithRun(data);
```

并把 `decide` 的返回类型改为 `Promise<InboxInteractionRecord>`。

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/api-client/src/mobile/interactions.test.ts packages/api-client/src/mobile/task-office.test.ts`
Expected: PASS（新增 2 用例 + 既有全绿；既有 `interactions list ...` 用例的路径断言已同步更新）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/interactions.ts packages/api-client/src/mobile/interactions.test.ts packages/api-client/src/mobile/task-office.ts packages/api-client/src/mobile/task-office.test.ts
git commit -m "feat(api-client): real /api/v1 interaction paths, inbox SDK and honest outcome codes (T08)"
```

---

### Task 5: mobile-core——`attention-inbox.ts` 域模块与 Task Office `inbox()`/`decide()` 接口

**Files:**
- Create: `packages/mobile-core/src/task-office/attention-inbox.ts`
- Modify: `packages/mobile-core/src/task-office/task-office-errors.ts:10-17`（错误码联合新增一项）
- Modify: `packages/mobile-core/src/task-office/task-office.ts:1-30`（imports）、`:100-116`（`TaskOfficePorts`/`TaskOffice` 接口）、`:142-239`（`createTaskOffice` 装配）
- Modify: `packages/mobile-core/src/index.ts:15-31`（task-office 导出区追加）
- Test: `packages/mobile-core/src/task-office/attention-inbox.test.ts`（新建）

**Interfaces:**
- Consumes: `TaskOfficeError`/`TaskOfficeErrorCode`（`task-office-errors.ts`）、`leaseActive`/`ScopeLease`（`../runtime/scope-lease.ts`、`../runtime/types.ts`，沿 task-office.ts 既有 import）、Task 4 的契约码 `INTERACTION_SUPERSEDED`/`INTERACTION_DELIVERY_UNKNOWN`/`INTERACTION_GONE`（字符串字面量镜像，mobile-core 不依赖 api-client——沿 `TASK_STREAM_CURSOR_EXPIRED` 的跨包字符串契约先例）。
- Produces:
  - `type InteractionKindValue = 'tool_approval' | 'budget' | 'recovery'`；`type InteractionActionValue = 'approve' | 'reject' | 'extend' | 'retry' | 'provide_result' | 'terminate'`；`const INTERACTION_ACTIONS: Readonly<Record<InteractionKindValue, readonly InteractionActionValue[]>>`；`function interactionActionAllowed(kind: InteractionKindValue, action: string): boolean`
  - `interface InboxItem { interactionId: string; runId: string; kind: InteractionKindValue; argsHash: string; expectedRevision: number; createdAt: string }`
  - `interface ResolvedDecisionRecord { interactionId: string; runId: string; kind: InteractionKindValue; decisionId: string; action: InteractionActionValue; argsHash: string; expectedRevision: number }`
  - `type AttentionDecisionReceipt = { status: 'recorded'; record: ResolvedDecisionRecord } | { status: 'delivery-unknown'; interactionId: string; decisionId: string } | { status: 'superseded'; interactionId: string } | { status: 'gone'; interactionId: string }`
  - `interface InboxView { items: InboxItem[] }`；`interface AttentionDecisionInput { item: InboxItem; action: InteractionActionValue }`
  - `interface InteractionBackendPort { inbox(input: { limit: number }): Promise<InboxItem[]>; decide(input: { item: InboxItem; decisionId: string; action: InteractionActionValue }): Promise<ResolvedDecisionRecord> }`
  - `function createAttentionDecider(deps: { interactions(): InteractionBackendPort | undefined; lease(): ScopeLease | undefined; onDecided(): void }): { inbox(): Promise<InboxView>; decide(input: AttentionDecisionInput): Promise<AttentionDecisionReceipt> }`
  - Task Office 接口新增：`inbox(): Promise<InboxView>`、`decide(input: AttentionDecisionInput): Promise<AttentionDecisionReceipt>`；`TaskOfficePorts.interactions?: InteractionBackendPort`；错误码 `TASK_OFFICE_INTERACTIONS_UNAVAILABLE`。Task 6 的 apps/mobile 消费。

- [ ] **Step 1: 写失败测试**

新建 `packages/mobile-core/src/task-office/attention-inbox.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createTaskOffice, TaskOfficeError } from '../index.ts';
import type { InboxItem, InteractionActionValue, InteractionBackendPort, ResolvedDecisionRecord, TaskBackendPort } from '../index.ts';

const ITEM: InboxItem = {
  interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval', argsHash: 'sha256:aa', expectedRevision: 4, createdAt: '2026-09-24T00:00:00Z',
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

// leaseActive 判定 instanceof RuntimeScopeLease（scope-lease.ts:26-28）：
// 假 lease 必须用真实类铸造（与 task-office.test.ts 的 leased() 同模式）。
function leaseFixture(): { revocable: RuntimeScopeLease; lease: ScopeLease } {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

function codedError(code: string): Error {
  const error = new Error(code);
  (error as unknown as { code?: string }).code = code;
  return error;
}

function scriptedInteractions(script: {
  inboxItems?: InboxItem[];
  inboxError?: Error;
  decideSteps?: Array<{ record?: ResolvedDecisionRecord; error?: Error }>;
}): InteractionBackendPort & { decideCalls: Array<{ item: InboxItem; decisionId: string; action: InteractionActionValue }>; inboxCalls: number } {
  let decideIndex = 0;
  let inboxCalls = 0;
  const decideCalls: Array<{ item: InboxItem; decisionId: string; action: InteractionActionValue }> = [];
  const port: InteractionBackendPort & { decideCalls: typeof decideCalls; inboxCalls: number } = {
    get inboxCalls() { return inboxCalls; },
    get decideCalls() { return decideCalls; },
    async inbox() {
      inboxCalls += 1;
      if (script.inboxError) throw script.inboxError;
      return script.inboxItems ?? [ITEM];
    },
    async decide(input) {
      decideCalls.push(input);
      const steps = script.decideSteps ?? [{}];
      const step = steps[Math.min(decideIndex, steps.length - 1)]!;
      decideIndex += 1;
      if (step.error) throw step.error;
      return step.record ?? {
        interactionId: input.item.interactionId,
        runId: input.item.runId,
        kind: input.item.kind,
        decisionId: input.decisionId,
        action: input.action,
        argsHash: input.item.argsHash,
        expectedRevision: input.item.expectedRevision + 1,
      };
    },
  };
  return port;
}

const emptyBackend: TaskBackendPort = {
  overview: () => Promise.resolve({ needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf: '2026-09-24T00:00:00Z' }),
  list: () => Promise.resolve({ items: [] }),
  archive: () => Promise.resolve(),
  restore: () => Promise.resolve(),
};

test('inbox fails closed without the interactions port and maps backend failures', async () => {
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease });
  await assert.rejects(office.inbox(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INTERACTIONS_UNAVAILABLE');

  const failing = scriptedInteractions({ inboxError: new Error('network down') });
  const wired = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions: failing });
  await assert.rejects(wired.inbox(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_BACKEND');
});

test('inbox without a live scope lease rejects with SCOPE_CHANGED', async () => {
  const interactions = scriptedInteractions({});
  const office = createTaskOffice({ backend: emptyBackend, lease: () => undefined, interactions });
  await assert.rejects(office.inbox(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  assert.equal(interactions.inboxCalls, 0, 'scope 无效时不得发起后端读');
});

test('a newer inbox read supersedes an in-flight older one', async () => {
  const gate = deferred<InboxItem[]>();
  let calls = 0;
  const interactions: InteractionBackendPort = {
    inbox: () => { calls += 1; return calls === 1 ? gate.promise : Promise.resolve([ITEM]); },
    decide: () => Promise.reject(new Error('not used')),
  };
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions });
  const first = office.inbox();
  const second = office.inbox();
  gate.resolve([ITEM]);
  await assert.rejects(first, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED');
  assert.equal((await second).items.length, 1);
});

test('decide validates the frozen kind-action matrix locally before any backend call', async () => {
  const interactions = scriptedInteractions({});
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions });
  await assert.rejects(office.decide({ item: { ...ITEM, kind: 'budget' }, action: 'approve' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
  assert.equal(interactions.decideCalls.length, 0, '矩阵违规不发网络请求');
});

test('decide freezes the decision id: repeats replay the same durable identity (AC1 mobile)', async () => {
  const interactions = scriptedInteractions({});
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions });
  const first = await office.decide({ item: ITEM, action: 'approve' });
  const second = await office.decide({ item: ITEM, action: 'approve' });
  assert.equal(first.status, 'recorded');
  assert.equal(second.status, 'recorded');
  assert.equal(interactions.decideCalls.length, 2);
  assert.equal(interactions.decideCalls[0].decisionId, interactions.decideCalls[1].decisionId, '双击/重试复用同一 decision_id');
  assert.notEqual(interactions.decideCalls[0].decisionId, '');
});

test('a delivery-unknown outcome keeps the decision id and a retry converts it to recorded (AC2)', async () => {
  const interactions = scriptedInteractions({ decideSteps: [{ error: codedError('INTERACTION_DELIVERY_UNKNOWN') }, {}] });
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions });
  const first = await office.decide({ item: ITEM, action: 'approve' });
  assert.equal(first.status, 'delivery-unknown');
  assert.equal(first.status === 'delivery-unknown' && first.decisionId !== '', true, 'delivery-unknown 携带已冻结的 decision_id');
  const retry = await office.decide({ item: ITEM, action: 'approve' });
  assert.equal(retry.status, 'recorded');
  assert.equal(interactions.decideCalls[0].decisionId, interactions.decideCalls[1].decisionId, '重试重放同一 durable 身份');
});

test('superseded and gone outcomes report honestly instead of throwing', async () => {
  const superseded = scriptedInteractions({ decideSteps: [{ error: codedError('INTERACTION_SUPERSEDED') }] });
  const officeA = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions: superseded });
  const receiptA = await officeA.decide({ item: ITEM, action: 'approve' });
  assert.deepEqual(receiptA, { status: 'superseded', interactionId: 'i-1' });

  const gone = scriptedInteractions({ decideSteps: [{ error: codedError('INTERACTION_GONE') }] });
  const officeB = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions: gone });
  const receiptB = await officeB.decide({ item: ITEM, action: 'approve' });
  assert.deepEqual(receiptB, { status: 'gone', interactionId: 'i-1' });
});

test('a decided interaction invalidates the home projection and late inbox reads (Review Focus 5)', async () => {
  const interactions = scriptedInteractions({});
  const gate = deferred<{ needsMe: []; running: []; recentlyCompleted: []; unreadNotifications: 0; asOf: string }>();
  const backend: TaskBackendPort = {
    ...emptyBackend,
    overview: () => gate.promise,
  };
  const office = createTaskOffice({ backend, lease: () => leaseFixture().lease, interactions });
  const lateHome = office.home();
  await office.decide({ item: ITEM, action: 'approve' });
  gate.resolve({ needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf: '2026-09-24T00:00:00Z' });
  await assert.rejects(lateHome, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED', '决定终态 bump home epoch：迟到首页读不得回填已决定行');
});

test('a decided interaction whose scope was revoked mid-flight reports SCOPE_CHANGED, not recorded', async () => {
  const interactions = scriptedInteractions({});
  const { revocable, lease } = leaseFixture();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseRef.lease, interactions });
  const pending = office.decide({ item: ITEM, action: 'approve' });
  revocable.revoke(); // 决定在途时切租户/登出：lease 撤销
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/task-office/attention-inbox.test.ts`
Expected: FAIL——模块 `../index.ts` 未导出 `InboxItem`/`InteractionBackendPort` 等类型，`office.inbox is not a function`。

- [ ] **Step 3: 最小实现**

3a. `packages/mobile-core/src/task-office/task-office-errors.ts`——错误码联合追加一项：

```ts
export type TaskOfficeErrorCode =
  | 'TASK_OFFICE_SCOPE_CHANGED'
  | 'TASK_OFFICE_SUPERSEDED'
  | 'TASK_OFFICE_NO_ACTIVE_QUERY'
  | 'TASK_OFFICE_INVALID_INPUT'
  | 'TASK_OFFICE_BACKEND'
  | 'TASK_OFFICE_DETAIL_UNAVAILABLE'
  | 'TASK_OFFICE_DETAIL_CLOSED'
  | 'TASK_OFFICE_INTERACTIONS_UNAVAILABLE';
```

3b. 新建 `packages/mobile-core/src/task-office/attention-inbox.ts`：

```ts
import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { TaskOfficeError } from './task-office-errors.ts';

/**
 * Attention Inbox 域（T08，module-seams §5.1「Home、Task list、Attention inbox
 * 的一致读投影……Interaction decision」）：kind×action 冻结矩阵镜像 Go
 * ValidateInteractionAction（internal/modules/workbench/interaction.go:38-50），
 * decision_id 在模块内冻结——同一 item+action 的重复提交与重试重放同一 durable
 * 身份（AC1）；receipt 如实区分 recorded / delivery-unknown / superseded / gone，
 * recorded 只表示决定已记录，绝不解释为外部派发完成（AC2）。
 * mobile-core 不依赖 api-client：INTERACTION_* 是跨包契约码字符串（沿
 * TASK_STREAM_CURSOR_EXPIRED 先例）。
 */
export type InteractionKindValue = 'tool_approval' | 'budget' | 'recovery';
export type InteractionActionValue = 'approve' | 'reject' | 'extend' | 'retry' | 'provide_result' | 'terminate';

export const INTERACTION_ACTIONS: Readonly<Record<InteractionKindValue, readonly InteractionActionValue[]>> = {
  tool_approval: ['approve', 'reject'],
  budget: ['extend'],
  recovery: ['retry', 'provide_result', 'terminate'],
};

export function interactionActionAllowed(kind: InteractionKindValue, action: string): boolean {
  return (INTERACTION_ACTIONS[kind] as readonly string[]).includes(action);
}

export interface InboxItem {
  interactionId: string;
  runId: string;
  kind: InteractionKindValue;
  argsHash: string;
  expectedRevision: number;
  createdAt: string;
}

export interface ResolvedDecisionRecord {
  interactionId: string;
  runId: string;
  kind: InteractionKindValue;
  decisionId: string;
  action: InteractionActionValue;
  argsHash: string;
  expectedRevision: number;
}

export type AttentionDecisionReceipt =
  | { status: 'recorded'; record: ResolvedDecisionRecord }
  | { status: 'delivery-unknown'; interactionId: string; decisionId: string }
  | { status: 'superseded'; interactionId: string }
  | { status: 'gone'; interactionId: string };

export interface InboxView {
  items: InboxItem[];
}

export interface AttentionDecisionInput {
  item: InboxItem;
  action: InteractionActionValue;
}

export interface InteractionBackendPort {
  inbox(input: { limit: number }): Promise<InboxItem[]>;
  decide(input: { item: InboxItem; decisionId: string; action: InteractionActionValue }): Promise<ResolvedDecisionRecord>;
}

/** 跨包契约码（api-client task-office.ts 的 decide 分类写入 error.code）。 */
export const INTERACTION_SUPERSEDED = 'INTERACTION_SUPERSEDED';
export const INTERACTION_DELIVERY_UNKNOWN = 'INTERACTION_DELIVERY_UNKNOWN';
export const INTERACTION_GONE = 'INTERACTION_GONE';

function errorCode(error: unknown): string | undefined {
  if (error instanceof Error && 'code' in error) {
    const code = (error as { code?: unknown }).code;
    return typeof code === 'string' ? code : undefined;
  }
  return undefined;
}

let decisionCounter = 0;

/** decision_id 是幂等身份而非机密：优先 crypto.randomUUID，降级为单调计数+随机后缀。 */
export function newDecisionId(): string {
  const cryptoApi = (globalThis as { crypto?: { randomUUID?: () => string } }).crypto;
  if (cryptoApi?.randomUUID) return cryptoApi.randomUUID();
  decisionCounter += 1;
  return `dec-${Date.now().toString(36)}-${decisionCounter.toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

export interface AttentionDeciderDeps {
  interactions(): InteractionBackendPort | undefined;
  lease(): ScopeLease | undefined;
  /** 决定到达终态（recorded/delivery-unknown/superseded/gone 均已落地或失效）后回调：宿主失效 home/inbox 读。 */
  onDecided(): void;
}

export function createAttentionDecider(deps: AttentionDeciderDeps): {
  inbox(): Promise<InboxView>;
  decide(input: AttentionDecisionInput): Promise<AttentionDecisionReceipt>;
} {
  let inboxEpoch = 0;
  const frozenDecisionIds = new Map<string, string>();
  const requireLease = (): ScopeLease => {
    const lease = deps.lease();
    if (!lease || !leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    return lease;
  };
  const requireInteractions = (): InteractionBackendPort => {
    const backend = deps.interactions();
    if (backend === undefined) throw new TaskOfficeError('TASK_OFFICE_INTERACTIONS_UNAVAILABLE');
    return backend;
  };
  return {
    async inbox(): Promise<InboxView> {
      const backend = requireInteractions();
      const lease = requireLease();
      const epoch = ++inboxEpoch;
      let items: InboxItem[];
      try {
        items = await backend.inbox({ limit: 50 });
      } catch (error) {
        if (error instanceof TaskOfficeError) throw error;
        throw new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
      }
      if (epoch !== inboxEpoch) throw new TaskOfficeError('TASK_OFFICE_SUPERSEDED');
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      return { items };
    },
    async decide(input: AttentionDecisionInput): Promise<AttentionDecisionReceipt> {
      const backend = requireInteractions();
      const lease = requireLease();
      if (!interactionActionAllowed(input.item.kind, input.action)) {
        throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      }
      const key = `${input.item.interactionId}::${input.action}`;
      const decisionId = frozenDecisionIds.get(key) ?? newDecisionId();
      frozenDecisionIds.set(key, decisionId);
      let record: ResolvedDecisionRecord;
      try {
        record = await backend.decide({ item: input.item, decisionId, action: input.action });
      } catch (error) {
        const code = errorCode(error);
        if (code === INTERACTION_DELIVERY_UNKNOWN) {
          // 决定已落地、外部派发未知：不冒充成功，保留同一 decision_id 供重试重放。
          deps.onDecided();
          return { status: 'delivery-unknown', interactionId: input.item.interactionId, decisionId };
        }
        if (code === INTERACTION_SUPERSEDED) {
          deps.onDecided();
          return { status: 'superseded', interactionId: input.item.interactionId };
        }
        if (code === INTERACTION_GONE) {
          deps.onDecided();
          return { status: 'gone', interactionId: input.item.interactionId };
        }
        throw error instanceof TaskOfficeError ? error : new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
      }
      // 与 mutate() 同一守卫：写落地后 scope 已撤销 → SCOPE_CHANGED（receipt 不跨 scope 泄漏）。
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      deps.onDecided();
      return { status: 'recorded', record };
    },
  };
}
```

3c. `packages/mobile-core/src/task-office/task-office.ts`——头部 import 追加：

```ts
import { createAttentionDecider } from './attention-inbox.ts';
import type { AttentionDecisionInput, AttentionDecisionReceipt, InboxView, InteractionBackendPort } from './attention-inbox.ts';
```

`TaskOfficePorts`（:100-107）在 `store?` 之后追加：

```ts
  /** T08: 类型化交互端口（Attention Inbox 读 + 决定）。缺失时 inbox()/decide() fail closed。 */
  interactions?: InteractionBackendPort;
```

`TaskOffice`（:109-116）在 `open(...)` 之前追加：

```ts
  /** T08: 收件箱读——本人全部待处理交互（跨 run、含决定上下文）。 */
  inbox(): Promise<InboxView>;
  /** T08: 类型化决定——冻结 decision_id 幂等重放；receipt 如实区分 recorded / delivery-unknown / superseded / gone。 */
  decide(input: AttentionDecisionInput): Promise<AttentionDecisionReceipt>;
```

`createTaskOffice`（:142 起，`const defaultDetailStore = ...` 之后）追加装配：

```ts
  const attention = createAttentionDecider({
    interactions: () => ports.interactions,
    lease: ports.lease,
    // 决定终态失效一切在途聚合读（与 archive/restore 的 R1-F19 同语义）：
    // 迟到的 home()/inbox() 不得把已决定行回填为 pending。
    onDecided: () => {
      homeEpoch += 1;
      listEpoch += 1;
      accumulated = undefined;
    },
  });
```

返回对象（`archive(...)` 之前）追加：

```ts
    inbox(): Promise<InboxView> {
      return attention.inbox();
    },
    decide(input: AttentionDecisionInput): Promise<AttentionDecisionReceipt> {
      return attention.decide(input);
    },
```

3d. `packages/mobile-core/src/index.ts`——task-office 导出区（:15-31）追加：

```ts
export { createAttentionDecider, interactionActionAllowed, INTERACTION_ACTIONS } from './task-office/attention-inbox.ts';
export type {
  AttentionDecisionInput, AttentionDecisionReceipt, AttentionDeciderDeps, InboxItem, InboxView,
  InteractionActionValue, InteractionBackendPort, InteractionKindValue, ResolvedDecisionRecord,
} from './task-office/attention-inbox.ts';
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test 'packages/mobile-core/src/task-office/*.test.ts'`
Expected: PASS（新 9 用例 + 既有 task-office/task-detail/task-timeline/in-memory 全绿——作者基线实跑 task-office.test.ts 8 pass）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/attention-inbox.ts packages/mobile-core/src/task-office/attention-inbox.test.ts packages/mobile-core/src/task-office/task-office-errors.ts packages/mobile-core/src/task-office/task-office.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): attention inbox domain with frozen decision identity and honest receipts (T08)"
```

---

### Task 6: apps/mobile——收件箱控制器、屏、路由、组合根接线与 Home 常驻入口

**Files:**
- Create: `apps/mobile/src/attention-inbox-view.ts`
- Create: `apps/mobile/src/screens/AttentionInboxScreen.tsx`
- Create: `apps/mobile/src/app/inbox.tsx`
- Create: `apps/mobile/src/attention-inbox-view.test.ts`
- Modify: `apps/mobile/src/composition.ts:109-124`（`taskOfficeFor` 传 `interactions: remote`）
- Modify: `apps/mobile/src/screens/HomeScreen.tsx:46`（"View all tasks" 之后增收件箱入口）
- Modify: `apps/mobile/src/app-smoke.test.tsx:233`（按钮清单期望更新）、`:162-163` 区（authorized home 断言增收件箱入口）、文件末尾（新增 2 个 inbox 守卫测试）

**Interfaces:**
- Consumes: Task 5 的 `TaskOffice.inbox()`/`decide()`/`InboxItem`/`InteractionActionValue`/`INTERACTION_ACTIONS`/`AttentionDecisionReceipt`（`@weknora/mobile-core`）；`activeTaskOffice()`（`composition.ts:139-144`）。
- Produces: `createAttentionInboxController(office: Pick<TaskOffice, 'inbox' | 'decide'>): AttentionInboxController`（`state()/subscribe()/refresh()/decide(item, action)`；`AttentionInboxViewState = { loading: boolean; items?: InboxItem[]; error?: string; receipts: Array<{ key: string; copy: string }> }`）；`ATTENTION_RECEIPT_COPY`（四态文案，AC2 的 UI 语义锚点）；`ATTENTION_INBOX_ERROR_COPY`；`AttentionInboxScreen`（props `{ state, onRefresh, onDecide }`）；`/inbox` Expo Router 路由。Task 7 的集成冒烟消费同接口。

- [ ] **Step 1: 写失败测试**

1a. 新建 `apps/mobile/src/attention-inbox-view.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { TaskOfficeError } from '@weknora/mobile-core';
import type { AttentionDecisionReceipt, InboxItem, TaskOffice } from '@weknora/mobile-core';
import { createAttentionInboxController, ATTENTION_RECEIPT_COPY } from './attention-inbox-view.ts';

const ITEM: InboxItem = {
  interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval', argsHash: 'sha256:aa', expectedRevision: 4, createdAt: '2026-09-24T00:00:00Z',
};

function officeHarness(overrides?: {
  items?: InboxItem[];
  inboxError?: Error;
  decideReceipts?: Array<AttentionDecisionReceipt | Error>;
}): { office: Pick<TaskOffice, 'inbox' | 'decide'>; inboxCalls(): number; decideCalls(): number } {
  const decideReceipts = overrides?.decideReceipts ?? [{ status: 'delivery-unknown', interactionId: ITEM.interactionId, decisionId: 'd-1' } as AttentionDecisionReceipt];
  let inboxCalls = 0;
  let decideCalls = 0;
  let decideIndex = 0;
  return {
    inboxCalls: () => inboxCalls,
    decideCalls: () => decideCalls,
    office: {
      async inbox() {
        inboxCalls += 1;
        if (overrides?.inboxError) throw overrides.inboxError;
        return { items: overrides?.items ?? [ITEM] };
      },
      async decide() {
        decideCalls += 1;
        const step = decideReceipts[Math.min(decideIndex, decideReceipts.length - 1)]!;
        decideIndex += 1;
        if (step instanceof Error) throw step;
        return step;
      },
    },
  };
}

test('the controller loads the inbox, maps errors to copy, and appends honest receipt rows', async () => {
  const harness = officeHarness();
  const controller = createAttentionInboxController(harness.office);
  assert.deepEqual(controller.state(), { loading: true, receipts: [] });
  await controller.refresh();
  assert.deepEqual(controller.state().items, [ITEM]);
  assert.equal(controller.state().loading, false);

  // delivery-unknown 的 receipt 文案必须如实（AC2）：记录已落地 + 外部未知，绝不出现「完成/成功派发」。
  await controller.decide(ITEM, 'approve');
  const receipts = controller.state().receipts;
  assert.equal(receipts.length, 1);
  assert.equal(receipts[0].copy, ATTENTION_RECEIPT_COPY['delivery-unknown']);
  assert.match(receipts[0].copy, /已记录/);
  assert.match(receipts[0].copy, /未知/);
  assert.equal(/完成|成功/.test(receipts[0].copy) && !/不代表/.test(receipts[0].copy), false, 'receipt 文案不得宣称外部派发完成');
  // 决定后自动刷新收件箱（行应离开 pending）
  assert.equal(harness.inboxCalls(), 2);

  // 错误路径映射 TaskOfficeError 文案
  const failing = officeHarness({ inboxError: new TaskOfficeError('TASK_OFFICE_BACKEND') });
  const failingController = createAttentionInboxController(failing.office);
  await failingController.refresh();
  assert.match(failingController.state().error ?? '', /服务端暂时不可用/);
});

test('every receipt status maps to distinct honest copy', () => {
  for (const status of ['recorded', 'delivery-unknown', 'superseded', 'gone'] as const) {
    const copy = ATTENTION_RECEIPT_COPY[status];
    assert.ok(copy && copy.length > 0, status);
  }
  assert.match(ATTENTION_RECEIPT_COPY.recorded, /不代表外部操作已完成/);
});
```

1b. `apps/mobile/src/app-smoke.test.tsx` 更新与追加：

- :233 期望数组更新为：

```ts
  assert.deepEqual(buttons, ['Acme', 'Beta', 'Sign out', 'View all tasks', 'Open Inbox', 'Open Resources', 'Load home'], 'with more than one tenant every tenant is a header switch button');
```

- 「surface routing keeps upgrade-required free of authorized controls…」测试的 authorized home 按钮断言（:162-163 区）追加：

```ts
  assert.equal(homeButtons.includes('Open Inbox'), true, 'the authorized home keeps a resident attention inbox entry (T08)');
```

- 文件末尾追加：

```ts
test('the inbox route, screen and view consume the task office interface only', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  for (const relative of ['screens/AttentionInboxScreen.tsx', 'attention-inbox-view.ts', 'app/inbox.tsx']) {
    const source = readFileSync(join(here, relative), 'utf8');
    assert.equal(/@weknora\/(api-client|contracts)/.test(source), false, `${relative} must consume the Task Office Interface only (T08)`);
  }
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(composition, /interactions:\s*remote/, 'taskOfficeFor must pass the remote as the interactions port; inbox()/decide() fail closed without it (T08)');
});

test('the attention inbox screen renders honest receipt copy and per-kind matrix actions', async () => {
  const { AttentionInboxScreen } = await import('./screens/AttentionInboxScreen.tsx');
  const { ATTENTION_RECEIPT_COPY } = await import('./attention-inbox-view.ts');
  hooks().__reset();
  const state = {
    loading: false,
    items: [{
      interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval' as const, argsHash: 'sha256:aa', expectedRevision: 4, createdAt: '2026-09-24T00:00:00Z',
    }],
    receipts: [{ key: 'i-1:0', copy: ATTENTION_RECEIPT_COPY['delivery-unknown'] }],
  };
  const decided: Array<{ interactionId: string; action: string }> = [];
  const element = render(AttentionInboxScreen, {
    state,
    onRefresh: () => {},
    onDecide: (item: { interactionId: string }, action: string) => { decided.push({ interactionId: item.interactionId, action }); },
  });
  const texts = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(texts.some((text) => text.includes('工具审批')), true);
  assert.equal(texts.some((text) => text.includes('外部执行通道状态未知')), true, 'delivery-unknown 文案必须出现（AC2）');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('批准'), true);
  assert.equal(buttons.includes('拒绝'), true);
  assert.equal(buttons.includes('扩展预算'), false, 'tool_approval 行不得渲染 budget 域动作（矩阵冻结）');
  const approve = descendants(element).find(({ type, props }) => type === 'Button' && props.title === '批准');
  (approve.props.onPress as () => void)();
  assert.deepEqual(decided, [{ interactionId: 'i-1', action: 'approve' }]);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`Cannot find module './attention-inbox-view.ts'` / `./screens/AttentionInboxScreen.tsx`；既有 home 按钮清单断言因新增入口按钮而失败（RED 即证明入口缺失）。

- [ ] **Step 3: 最小实现**

3a. 新建 `apps/mobile/src/attention-inbox-view.ts`：

```ts
import { TaskOfficeError } from '@weknora/mobile-core';
import type { AttentionDecisionReceipt, InboxItem, InteractionActionValue, TaskOffice } from '@weknora/mobile-core';

/** Receipt 四态文案（AC2 语义锚点）：recorded 仅表示决定已记录，绝不解释为外部派发完成。 */
export const ATTENTION_RECEIPT_COPY: Record<AttentionDecisionReceipt['status'], string> = {
  recorded: '决定已记录；执行侧生效情况请以任务详情为准，不代表外部操作已完成。',
  'delivery-unknown': '决定已记录，但外部执行通道状态未知；可重试同步。',
  superseded: '该请求已更新或已由其他设备/会话处理，请刷新收件箱。',
  gone: '该请求已过期、被撤销或不存在，请刷新收件箱。',
};

export const ATTENTION_INBOX_ERROR_COPY: Record<string, string> = {
  TASK_OFFICE_SCOPE_CHANGED: '登录状态或活动空间已变化，请重新进入。',
  TASK_OFFICE_INTERACTIONS_UNAVAILABLE: '当前部署未提供交互决定通道。',
  TASK_OFFICE_INVALID_INPUT: '该动作与请求类型不匹配。',
  TASK_OFFICE_BACKEND: '服务端暂时不可用，请稍后重试。',
};

export interface AttentionReceiptRow {
  key: string;
  copy: string;
}

export interface AttentionInboxViewState {
  loading: boolean;
  items?: InboxItem[];
  error?: string;
  receipts: AttentionReceiptRow[];
}

export interface AttentionInboxController {
  state(): AttentionInboxViewState;
  subscribe(listener: (state: AttentionInboxViewState) => void): () => void;
  refresh(): Promise<void>;
  decide(item: InboxItem, action: InteractionActionValue): Promise<void>;
}

const messageOf = (failure: unknown): string =>
  failure instanceof TaskOfficeError ? (ATTENTION_INBOX_ERROR_COPY[failure.code] ?? failure.code)
    : failure instanceof Error ? failure.message
      : String(failure);

/** 收件箱控制器：load 驱动首帧，decide 追加 receipt 行并自动重读（已决定行应离开 pending）。 */
export function createAttentionInboxController(office: Pick<TaskOffice, 'inbox' | 'decide'>): AttentionInboxController {
  let state: AttentionInboxViewState = { loading: true, receipts: [] };
  let disposed = false;
  const listeners = new Set<(state: AttentionInboxViewState) => void>();
  const publish = (next: AttentionInboxViewState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const load = (): Promise<void> => office.inbox().then(
    (view) => { if (!disposed) publish({ items: view.items, loading: false, receipts: state.receipts }); },
    (failure: unknown) => { if (!disposed) publish({ loading: false, error: messageOf(failure), receipts: state.receipts }); },
  );
  return {
    state: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    refresh(): Promise<void> {
      publish({ ...state, loading: true, error: undefined });
      return load();
    },
    decide(item: InboxItem, action: InteractionActionValue): Promise<void> {
      return office.decide({ item, action }).then(
        async (receipt) => {
          if (!disposed) {
            publish({
              ...state,
              receipts: [...state.receipts, { key: `${receipt.interactionId}:${state.receipts.length}`, copy: ATTENTION_RECEIPT_COPY[receipt.status] }],
            });
          }
          await load();
        },
        (failure: unknown) => { if (!disposed) publish({ ...state, loading: false, error: messageOf(failure), receipts: state.receipts }); },
      );
    },
  };
}
```

3b. 新建 `apps/mobile/src/screens/AttentionInboxScreen.tsx`：

```tsx
import { Button, ScrollView, Text, View } from 'react-native';
import { INTERACTION_ACTIONS } from '@weknora/mobile-core';
import type { InboxItem, InteractionActionValue } from '@weknora/mobile-core';
import type { AttentionInboxViewState } from '../attention-inbox-view.ts';

const ACTION_LABELS: Record<InteractionActionValue, string> = {
  approve: '批准',
  reject: '拒绝',
  extend: '扩展预算',
  retry: '重试',
  provide_result: '提供结果',
  terminate: '终止',
};

const KIND_LABELS: Record<InboxItem['kind'], string> = {
  tool_approval: '工具审批',
  budget: '预算扩展',
  recovery: '恢复请求',
};

export interface AttentionInboxScreenProps {
  state: AttentionInboxViewState;
  onRefresh(): void;
  onDecide(item: InboxItem, action: InteractionActionValue): void;
}

/** Attention Inbox（T08）：同一 Interaction 身份的类型化决定面。动作按冻结矩阵渲染，
 *  receipt 文案如实区分 recorded / delivery-unknown / superseded / gone——绝不把决定
 *  ACK 显示成「外部派发完成」。 */
export function AttentionInboxScreen({ state, onRefresh, onDecide }: AttentionInboxScreenProps) {
  return (
    <ScrollView>
      <Text>Attention Inbox</Text>
      {state.loading && <Text>Loading</Text>}
      {state.error !== undefined && <Text>{state.error}</Text>}
      {state.items !== undefined && state.items.map((item) => (
        <View key={item.interactionId}>
          <Text>{`${KIND_LABELS[item.kind]} · run ${item.runId}`}</Text>
          {INTERACTION_ACTIONS[item.kind].map((action) => (
            <Button key={action} title={ACTION_LABELS[action]} onPress={() => onDecide(item, action)} />
          ))}
        </View>
      ))}
      {state.items !== undefined && state.items.length === 0 && <Text>Nothing needs you</Text>}
      {state.receipts.map((receipt) => (
        <Text key={receipt.key}>{receipt.copy}</Text>
      ))}
      <Button title="Refresh" onPress={onRefresh} disabled={state.loading} />
    </ScrollView>
  );
}
```

3c. 新建 `apps/mobile/src/app/inbox.tsx`：

```tsx
import { useEffect, useRef, useState } from 'react';
import { Text, View } from 'react-native';
import { activeTaskOffice } from '../composition.ts';
import { createAttentionInboxController, type AttentionInboxController, type AttentionInboxViewState } from '../attention-inbox-view.ts';
import { AttentionInboxScreen } from '../screens/AttentionInboxScreen.tsx';

/** /inbox 挂载生命周期宿主：controller 在 effect 内创建，与 /tasks/detail 同一模式。 */
export function InboxRouteLifecycle() {
  const [state, setState] = useState<AttentionInboxViewState>({ loading: true, receipts: [] });
  const controllerRef = useRef<AttentionInboxController | undefined>(undefined);
  useEffect(() => {
    const office = activeTaskOffice();
    if (!office) {
      setState({ loading: false, error: '请先登录并激活空间，再打开收件箱。', receipts: [] });
      return;
    }
    const controller = createAttentionInboxController(office);
    controllerRef.current = controller;
    setState(controller.state());
    void controller.refresh();
    const unsubscribe = controller.subscribe(setState);
    return () => {
      unsubscribe();
      controllerRef.current = undefined;
    };
  }, []);
  if (!controllerRef.current) {
    return (
      <View>
        <Text>{state.error ?? '正在读取收件箱…'}</Text>
      </View>
    );
  }
  return (
    <AttentionInboxScreen
      state={state}
      onRefresh={() => { void controllerRef.current?.refresh(); }}
      onDecide={(item, action) => { void controllerRef.current?.decide(item, action); }}
    />
  );
}

/** Expo Router 文件路由：/inbox。只消费 Task Office Interface。 */
export default function Inbox() {
  return <InboxRouteLifecycle />;
}
```

3d. `apps/mobile/src/composition.ts`——`taskOfficeFor`（:112-120）中 `createTaskOffice({...})` 的 `detail: remote,` 之后追加一行：

```ts
      interactions: remote,
```

3e. `apps/mobile/src/screens/HomeScreen.tsx`——"View all tasks" 按钮（:46）之后追加：

```tsx
      {/* T08 Attention Inbox 常驻入口：首页 needsMe 与收件箱同一 Interaction 身份。 */}
      <Button title="Open Inbox" onPress={() => router.push('/inbox')} />
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS + typecheck exit 0（含 mobile-core/api-client 传递类型检查：`interactions: remote` 的结构可赋值在此证明）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/attention-inbox-view.ts apps/mobile/src/attention-inbox-view.test.ts apps/mobile/src/screens/AttentionInboxScreen.tsx apps/mobile/src/app/inbox.tsx apps/mobile/src/composition.ts apps/mobile/src/screens/HomeScreen.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): attention inbox screen, route and honest receipt copy (T08)"
```

---

### Task 7: opt-in 真实部署集成冒烟（Attention Inbox 证据契约）

**Files:**
- Create: `apps/mobile/src/attention-inbox-integration-smoke.ts`
- Create: `apps/mobile/src/attention-inbox-integration-smoke.test.ts`

**Interfaces:**
- Consumes: Task 5/6 的 `TaskOffice.inbox()`/`decide()`；`taskDetailIntegrationConfig` 的 opt-in + `disallowedDeploymentHost` 主机防线模式（`task-detail-integration-smoke.ts:10-48`）；`createTaskOfficeRemote`（含 Task 4 的 `interactions` 能力）。
- Produces: `attentionInboxIntegrationConfig(env): AttentionInboxIntegrationConfig`（复用 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`；决定动作额外门控 `WEKNORA_MOBILE_TEST_DECIDE_INTERACTION=1`，默认跳过——不在真实账号上擅自决定）；`runAttentionInboxIntegration(config): Promise<AttentionInboxIntegrationEvidence>`；`emitAttentionInboxIntegrationEvidence(evidence, emit)`。证据契约：`{ deploymentOrigin, inbox: 'browsed'|'browse-failed', pendingCount: number, decide: 'skipped'|'no-pending'|'recorded'|'delivery-unknown'|'superseded'|'gone'|'failed', failure?: string, commandTimestamp }`（无凭据字段）。

- [ ] **Step 1: 写失败测试**

新建 `apps/mobile/src/attention-inbox-integration-smoke.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { attentionInboxIntegrationConfig, emitAttentionInboxIntegrationEvidence } from './attention-inbox-integration-smoke.ts';

const here = dirname(fileURLToPath(import.meta.url));

test('the attention inbox integration config stays opt-in and reuses the host defense line', () => {
  assert.equal(attentionInboxIntegrationConfig({}).enabled, false);
  const vars = { WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.org', WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test', WEKNORA_MOBILE_TEST_PASSWORD: 'pw' };
  assert.equal(attentionInboxIntegrationConfig(vars).enabled, true);
  assert.equal(attentionInboxIntegrationConfig(vars).enabled && (attentionInboxIntegrationConfig(vars) as { decideEnabled: boolean }).decideEnabled, false, '决定动作默认关闭');
  const gated = attentionInboxIntegrationConfig({ ...vars, WEKNORA_MOBILE_TEST_DECIDE_INTERACTION: '1' });
  assert.equal(gated.enabled && gated.decideEnabled, true);
  for (const host of ['https://127.0.0.1:8080', 'https://localhost', 'https://10.0.0.5', 'https://192.168.1.4', 'https://169.254.1.1']) {
    const rejected = attentionInboxIntegrationConfig({ ...vars, WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host });
    assert.equal(rejected.enabled === false && rejected.disposition, 'invalid', host);
  }
});

test('emit produces the redacted evidence contract without credential fields', () => {
  const lines: string[] = [];
  emitAttentionInboxIntegrationEvidence({
    deploymentOrigin: 'https://weknora.example.org', inbox: 'browsed', pendingCount: 1,
    decide: 'skipped', commandTimestamp: '2026-09-24T00:00:00Z',
  }, (record) => lines.push(record));
  const evidence = JSON.parse(lines[0]);
  assert.equal(evidence.inbox, 'browsed');
  assert.equal(evidence.decide, 'skipped');
  assert.equal('password' in evidence || 'email' in evidence, false);
});

test('the smoke reuses the shared host defense and is total over failures', () => {
  const source = readFileSync(join(here, 'attention-inbox-integration-smoke.ts'), 'utf8');
  assert.match(source, /disallowedDeploymentHost/, 'config 校验必须复用主机防线');
  const runBody = source.slice(source.indexOf('export async function runAttentionInboxIntegration'));
  assert.match(runBody, /finally\s*\{/, '主流程必须有 finally 收口');
  assert.match(runBody, /dispose\(\)/, 'finally 内必须释放 runtime');
  assert.match(runBody, /browse-failed/, '读失败必须如实记录');
});

test('runs the live attention inbox loop when the environment is present', { skip: attentionInboxIntegrationConfig(process.env).enabled === false ? 'missing WEKNORA_MOBILE_TEST_* credentials' : false }, async () => {
  const { runAttentionInboxIntegration } = await import('./attention-inbox-integration-smoke.ts');
  const config = attentionInboxIntegrationConfig(process.env);
  assert.equal(config.enabled, true);
  const evidence = await runAttentionInboxIntegration(config);
  assert.ok(['browsed', 'browse-failed'].includes(evidence.inbox));
  assert.ok(['skipped', 'no-pending', 'recorded', 'delivery-unknown', 'superseded', 'gone', 'failed'].includes(evidence.decide));
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`Cannot find module './attention-inbox-integration-smoke.ts'`。

- [ ] **Step 3: 最小实现**

新建 `apps/mobile/src/attention-inbox-integration-smoke.ts`：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { INTERACTION_ACTIONS, createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type AttentionInboxIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; decideEnabled: boolean }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface AttentionInboxIntegrationEvidence {
  deploymentOrigin: string;
  inbox: 'browsed' | 'browse-failed';
  pendingCount: number;
  decide: 'skipped' | 'no-pending' | 'recorded' | 'delivery-unknown' | 'superseded' | 'gone' | 'failed';
  /** 异常路径失败摘要（仅 error message，证据契约无凭据字段）。 */
  failure?: string;
  commandTimestamp: string;
}

/** opt-in 语义与 T04/T05 相同（自包含，不跨计划 import）；决定动作额外要求
 * WEKNORA_MOBILE_TEST_DECIDE_INTERACTION=1——不在真实账号上擅自决定。 */
export function attentionInboxIntegrationConfig(env: Record<string, string | undefined>): AttentionInboxIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try { parsed = new URL(deploymentOrigin); } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  return { enabled: true, deploymentOrigin: parsed.origin, email, password, decideEnabled: env.WEKNORA_MOBILE_TEST_DECIDE_INTERACTION === '1' };
}

/** 真实 JSON transport + 授权通道 + 具体 Remote Adapter（含 interactions 端口）+ Task Office 编排。
 *  total 契约：任何步骤异常 → inbox:'browse-failed' + failure 摘要，从不 reject；所有路径经同一 finally 清理。 */
export async function runAttentionInboxIntegration(config: Extract<AttentionInboxIntegrationConfig, { enabled: true }>): Promise<AttentionInboxIntegrationEvidence> {
  const evidence: AttentionInboxIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    inbox: 'browse-failed',
    pendingCount: 0,
    decide: 'skipped',
    commandTimestamp: new Date().toISOString(),
  };
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  try {
    const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;
    // 同一 remote 同时作为 backend/detail/interactions 三个端口（与 composition 相同装配）。
    const remote = createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) });
    const office: TaskOffice = createTaskOffice({ backend: remote, detail: remote, interactions: remote, lease: () => runtime.scopeLease() });

    const view = await office.inbox();
    evidence.inbox = 'browsed';
    evidence.pendingCount = view.items.length;

    if (!config.decideEnabled) return evidence;
    if (view.items.length === 0) {
      evidence.decide = 'no-pending';
      return evidence;
    }
    const target = view.items[0]!;
    const action = INTERACTION_ACTIONS[target.kind][0]!;
    const receipt = await office.decide({ item: target, action });
    evidence.decide = receipt.status;
    return evidence;
  } catch (error) {
    evidence.failure = error instanceof Error ? error.message : String(error);
    if (evidence.decide !== 'skipped') evidence.decide = 'failed';
    return evidence;
  } finally {
    runtime.dispose(); // dispose(): void（packages/mobile-core/src/runtime/types.ts:72）
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitAttentionInboxIntegrationEvidence(evidence: AttentionInboxIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm --filter @weknora/mobile test`
Expected: PASS（4 个新用例：3 个本地 + 1 个无环境时 skip；具备 `WEKNORA_MOBILE_TEST_*` 环境时 live 用例自动运行并断言证据形状）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/attention-inbox-integration-smoke.ts apps/mobile/src/attention-inbox-integration-smoke.test.ts
git commit -m "test(mobile): opt-in attention inbox integration smoke evidence contract (T08)"
```

---

## 计划级验证命令（worktree 根执行）

```bash
go test ./internal/modules/workbench/service/workbench/ -run 'TestGormInteractionStore|TestInteractionService' -count=1 && \
go test ./internal/handler/session/ -run 'TestWorkbenchInbox|TestWorkbenchDecision|TestWorkbenchCommand' -count=1 && \
go test ./internal/router/ -run 'TestRegisterWorkbenchCommandRoutesDeclaresTypedEndpoints' -count=1 && \
go build ./internal/... ./cmd/... && \
npx tsx --test packages/contracts/test/mobile-interaction-inbox.test.ts packages/contracts/test/mobile-interactions.test.ts && \
npx tsx --test 'packages/mobile-core/src/task-office/*.test.ts' && \
npx tsx --test packages/api-client/src/mobile/interactions.test.ts packages/api-client/src/mobile/task-office.test.ts && \
pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck
```

（作者基线实跑：`go test ./internal/modules/workbench/service/workbench/ -count=1` ok 11.848s；`go test ./internal/handler/session/ -run 'TestWorkbenchDecision|TestWorkbenchCommand' -count=1` ok；`go test ./internal/router/ -run 'TestRegisterWorkbenchCommandRoutes' -count=1` ok；`npx tsx --test packages/api-client/src/mobile/interactions.test.ts` 3 pass；`npx tsx --test packages/mobile-core/src/task-office/task-office.test.ts` 8 pass。审查轮实跑复核：`npx tsx --test 'packages/mobile-core/src/task-office/*.test.ts'` 42 pass 全绿、`pnpm --filter @weknora/mobile test` 72 pass。）

## Consumes / Produces 汇总（供后续计划 #39/#48/#52/#68 及同批次计划引用）

**Produces（本计划新增的对外接口）：**
- Go wire：`GET /api/v1/workbench/interactions?limit=`（收件箱读：owner 跨 run pending 行，含 `run_id`/`created_at`，归档任务与过期行排除）；`workbench.InteractionDecision.CreatedAt`（输出字段 `created_at,omitempty`）；决定/命令错误 wire `502 + {"code":"command_recovery_unknown"}`；`GormInteractionStore.ListPending`、`Service.ListInbox`、`WorkbenchCommandHandler.ListInboxInteractions`。
- contracts：`InboxInteractionRecord`/`parseInteractionWithRun`（`packages/contracts/src/mobile/interaction-inbox.ts`）。
- api-client：`createInteractionsApi` 路径修复（`/api/v1/...`）与 `inbox(limit?)`；`createTaskOfficeRemote` 新增 `inbox()`/`decide()` 与 `RemoteInboxItem`/`RemoteDecisionRecord`；跨包契约码 `INTERACTION_SUPERSEDED`/`INTERACTION_DELIVERY_UNKNOWN`/`INTERACTION_GONE`。
- mobile-core：`TaskOffice.inbox()`/`decide(input)`；`InteractionBackendPort`（`TaskOfficePorts.interactions?`）；`InboxItem`/`ResolvedDecisionRecord`/`AttentionDecisionReceipt`（recorded / delivery-unknown / superseded / gone）/`InboxView`/`INTERACTION_ACTIONS`/`interactionActionAllowed`/`createAttentionDecider`/`newDecisionId`；错误码 `TASK_OFFICE_INTERACTIONS_UNAVAILABLE`。**同名注记（F7）**：本计划的 `@weknora/mobile-core` `InboxItem`（Task Office 收件箱行：interactionId/runId/kind/argsHash/expectedRevision/createdAt）与 contracts execution 域既有的 `@weknora/contracts` `InboxItem`（`packages/contracts/src/mobile/read-models.ts:76`，经 `src/index.ts:666` 导出的 Web 收件箱行）同名不同物；本计划各任务引用路径单一（screens/view 只 import mobile-core），无冲突，后续同文件消费两包时需 alias。
- apps/mobile：`AttentionInboxScreen`、`/inbox` 路由、`createAttentionInboxController` + `ATTENTION_RECEIPT_COPY`、Home 常驻入口「Open Inbox」、composition `interactions: remote` 装配、`attentionInboxIntegrationConfig`（决定动作门控 `WEKNORA_MOBILE_TEST_DECIDE_INTERACTION=1`）与证据契约。

**明确不在本计划（后续 Issue / 边界）：**
- `InteractionBudget`/`InteractionRecovery` 的生产创建方（预算扩额落行、恢复决定落 `workbench_interactions`）属 commercial/chat 模块域；`Go Service.Decide` 对 budget/recovery 的下游派发同样不在本计划（当前仅 CAS 记录）。
- `TaskHandle.act(TaskIntent)` 的 decision 意图（#37）——与本计划共享 `InteractionBackendPort`。
- `start(goal)`/request_id 对账（#36）、原生加密持久化（#40）。
- blocked-env 验收项：真机 ×2 经真实移动 App 并发决定同一 pending 交互的端到端证据（需真机 + 真实部署 + 真实 pending 审批 + 测试账号）；本地替代证据见「验收标准 3 的本地可验证性说明」，opt-in 冒烟在具备环境时自动产出该证据（`WEKNORA_MOBILE_TEST_DECIDE_INTERACTION=1` 时对真实 pending 交互执行一次真实决定）。
