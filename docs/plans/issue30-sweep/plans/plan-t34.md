# T04：首页 Attention 与统一 Task 列表（Issue #34）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移动端首页一次聚合读展示「需要我处理 / 正在运行 / 最近完成」三段视图，统一 Task 列表支持文本搜索、状态筛选与归档（含归档写路径），全部读自当前 Tenant 的授权聚合读模型（无逐会话 N+1、无跨租户泄露），并通过新建的 Task Office 深模块（最高稳定 Interface）+ opt-in 真实 HTTP 集成证据端到端验证。

**Architecture:** 后端补全既有 B 类读模型：`/workbench/overview` 增加 `recently_completed` 段、任务标题投影（`agent_runs JOIN sessions`，ADR-0004 `taskId = sessionId`）、排除已归档任务，并把 `unread_notifications` 接到本计划补建的 `workbench_notifications` 表（该表此前仅存在于 MX-021 的 gorm 投影与测试 AutoMigrate，生产迁移从未建表——见差异记录 11）真实计数；`/workbench/executions` 列表增加 `q` 标题搜索（LIKE 通配符转义）、`archived` 筛选、`title`/`attention`/`archived_at` 投影，cursor 绑定新 facet；新增 `POST/DELETE /workbench/tasks/:task_id/archive` 归档写路径（owner 谓词与读模型一致）。归档状态以 `sessions.archived_at` 列承载（Task 沿用 Session 唯一身份）。客户端新建 `packages/mobile-core` 的 Task Office Module（`home`/`tasks`/`moreTasks`/`archive`/`restore`，内部拥有 cursor、查询身份、重复键守卫、Scope Lease 迟到拒绝），经 `MobileRuntime.authorizedRequest`（本计划新增的授权读通道，token 不出 Runtime）取数；apps/mobile 新增 Home/Tasks 两屏消费模块视图，不触碰 wire 层。

**Tech Stack:** Go 1.26（gin + gorm，sqlite/postgres 双方言迁移 `migrations/{sqlite,versioned}`）、TypeScript（`packages/contracts`、`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN）、node:test + tsx（TS 测试运行器，与 `mobile-runtime.test.ts` 一致）、`testify`（Go）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置：已执行过 `pnpm install`（本计划作者已实跑）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-34.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Stories 7/8/16、Implementation Decisions、Testing Decisions）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§5 Task Office Module、§10 App Shell、§13 Interface 测试面、§16 已知差距）
- ADR：`docs/adr/0004-task-is-session.md`（taskId = sessionId）、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`、`docs/adr/0005-weknora-native-mobile-client.md`
- 领域术语：`CONTEXT.md`（「任务生命周期（Task Lifecycle）」：进行中/已完成/已取消/已归档；「关注状态（Attention State）」）
- Parent：Issue #30；Blocked by：#32（T02——其产出的 `MobileRuntime.activateTenant`、`RuntimeSnapshot.identity.tenants`、`scope-lease.ts` 包内 lease 内省、`RuntimeSurfaceProps.onActivateTenant` 由本计划消费。**执行切分**：仅 Task 6/8 需要 #32 产出、必须等其合并；Task 1-5、7、9 不依赖 #32 可先行——见「任务结构与文件地图」末尾的切分说明）

## Global Constraints

以下为批准 Spec / ADR 的项目级约束，逐字引用，所有任务隐含遵守：

- 「Task Office owns Home/Task projections, durable submission identity, reconciliation, Snapshot/SSE recovery, intervention, decisions, budget and Task lifecycle.」（mobile-ai-office-design.md · Implementation Decisions）
- 「The mobile information architecture is Home, Tasks, New, Resources and Me.」（同上；本计划交付 Home 与 Tasks 两个一级入口的读侧，New/Resources/Me 不在本 Issue）
- 「Task lifecycle, Run status and Attention status are separate. Agent-specific progress phases never replace these canonical dimensions.」（同上）
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」（同上）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上 · Testing Decisions）
- 「Remote-owned dependencies use in-memory scenario Adapters for Module tests and real HTTP/SSE contract tests for production Adapters.」（同上）
- 「移动 AI Office 将现有 WeKnora Session 呈现为 Task，并保持 `taskId = sessionId`，不新增第二个 Task 聚合身份。」（ADR-0004）
- 「Screen 不调用 start、lookup、snapshot、events、interaction、command 等多个 wire 方法。Module 内部决定顺序、幂等、重连、revision 和错误呈现。」（mobile-module-seams.md §5.2）
- 「禁止：Screen 直接导入 packages/contracts 或 packages/api-client；Screen 自己维护 request_id、cursor、revision、scope generation；每个 Screen 建独立 query cache 或 token refresh」（mobile-module-seams.md §10）
- 「Interface 不暴露 token、query key、generation number 或 SecureStore key。Scope Lease 是不透明、可撤销的能力对象，子 Module 每次异步提交前检查其有效性。」（mobile-module-seams.md §4.2）
- 安全约束（会话注入）：配置凭据只从环境变量或密钥服务读取；源码、示例和测试都不得写入可用的凭据字面量。Task 9 的真实 HTTP 集成证据沿用 T01/T02 的 opt-in 环境变量模式（`WEKNORA_MOBILE_TEST_*`），无任何回退凭据。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #34 验收标准原文（docs/plans/issue30-sweep/issues/issue-34.md）：**

1. 「首页和列表不做逐会话 N+1，也不泄露其他 Tenant 内容。」
2. 「迟到查询、分页重复键和空/错/加载状态均可验证。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：真实端到端（生产 JSON transport + 具体 Remote Adapter + Task Office 编排 + 真后端聚合读模型 + 归档写回路）沿用 T01 已合并的 opt-in 真实 HTTP 测试模式，需要「一个真实 WeKnora Deployment（HTTPS origin）+ 一个测试账号」（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 环境变量）。本地无此环境时 Task 9 的真实 HTTP 用例以 `t.skip` 跳过（**不得伪造通过**），本地替代证据为：Task Office Interface 级测试（真实编排 + in-memory scenario Adapter）+ Go 集成测试（真实 sqlite 迁移库上的谓词/分页/归档回路）+ contracts/api-client wire 契约测试（真实序列化字节）。凡具备环境的运行都会自动产出端到端证据。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **迟到的旧 scope 响应**：切租户/退出时 `home()`/`tasks()` 仍在途，旧响应返回后不得把旧空间内容写进新视图。——Task 6 测试「a late home response after the scope lease was revoked never resolves with foreign-scope data」与「a newer tasks query supersedes an in-flight older one」。
2. **新增读段的跨租户/跨 owner 泄露**：`recently_completed`、`unread_notifications`、搜索与归档谓词若漏掉 tenant+owner 绑定，会泄露同库邻居内容。——Task 2 测试 `TestWorkbenchListSearchFiltersByTaskTitleWithinOwnerScope`（含同租户他 owner、异租户邻居）与 `TestWorkbenchListArchiveFilterAndAttentionProjection` 末段（异 owner 归档行不可见）+ Task 3 扩展的 `TestMX013OverviewOwnerScope`（unread 只计本人、recently_completed 只含本人终态）。
3. **搜索词含 LIKE 通配符 / 全空白 / 超长**：`%`、`_`、`\` 若不转义会把搜索变成全表通配；全空白不得静默变成无过滤之外的语义；超长输入需截断。——Task 2 测试 `TestWorkbenchListSearchFiltersByTaskTitleWithinOwnerScope`（`100%`/`status_report`/反斜杠字面匹配 + 全空白归一）+ Task 6 测试「queries are normalized before they reach the backend」（模块侧截断与归一）。
4. **已归档任务漏回默认视图**：归档后 home 三段与列表默认视图必须立即排除（否则「归档」是假的）。——Task 2 测试 `TestWorkbenchListArchiveFilterAndAttentionProjection`（默认视图排除、ArchivedOnly 才可见）+ Task 3 测试 `TestMX013OverviewExcludesArchivedTasksFromEverySegment`（三段全排除）。
5. **分页重复键跨页重现**：新 facet（q/archived）下的 cursor 若不绑定筛选，重放会静默放宽筛选；服务端重复行不得被渲染两次。——Task 2 测试 `TestWorkbenchListCursorBindsSearchAndArchiveFacets`（换 facet 重放一律拒绝）+ Task 6 测试「moreTasks continues the owned cursor and duplicate run keys are reported, never rendered twice」。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | Go：Task 归档迁移与写路径 | `sessions.archived_at` 列（pg+sqlite 迁移）、`WorkbenchTaskStateStore`、`POST/DELETE /workbench/tasks/:task_id/archive` |
| 2 | Go：统一列表搜索/归档/attention/标题 | `q`+`archived` 筛选、`title`/`attention`/`archived_at` 投影、cursor 绑定新 facet |
| 3 | Go：workbench_notifications 建表 + overview 最近完成段与真实 unread | 建表迁移（versioned 000190 / sqlite 000111）+ 迁移对齐测试、`recently_completed`、标题投影、归档排除、`unread_notifications` 真实计数 |
| 4 | contracts：读模型契约扩展 | `WorkbenchOverview.recently_completed`、`ExecutionSummary.attention` + 解析测试 |
| 5 | mobile-core：`MobileRuntime.authorizedRequest` | 授权读通道（token 不出 Runtime、401 单飞刷新、迟到丢弃） |
| 6 | mobile-core：Task Office Module | `home`/`tasks`/`moreTasks`/`archive`/`restore` + scenario Adapter + Interface 测试 |
| 7 | api-client：Task Office Remote 适配 | 列表参数/字段扩展、`createTaskOfficeRemote` wire 适配 + 契约测试 |
| 8 | apps/mobile：Home/Tasks 屏 | 三段首页、搜索/筛选/归档列表、组合根接线、屏测试 |
| 9 | 真实 HTTP 集成证据（AC3） | opt-in 端到端回路 + `TaskOfficeIntegrationEvidence` |

任务依赖：1 → 2/3（迁移先行）；4 → 7；5 → 8；6 → 8；7 → 8/9。所有测试命令在 worktree 根执行。

**#32 前置的执行切分**（作者实跑 `git ls-files packages/mobile-core/src/runtime/` 核实：当前 worktree 无 `scope-lease.ts`，`types.ts:26` 的 identity 仍为 `{ userId; activeTenantId? }` 无 `tenants`——#32 尚未落地）：Task 1-5、7、9 不依赖 #32 产出，可先行（Task 5 所需 `refreshedCredential`/`current`/`signOut`/`scopeLease` 已核实存在于 mobile-runtime.ts:119/:98/:170/:278）；**Task 6 与 Task 8 必须等 #32 合并**——Task 6 测试 import 包内 `RuntimeScopeLease`（scope-lease.ts），Task 8 消费 `identity.tenants` 与 `RuntimeSurfaceProps.onActivateTenant`（两处签名已与 plan-t32.md:322-339/:406 逐字核对一致，无跨计划漂移）。

---

### Task 1: Go — Task 归档迁移与写路径

**Files:**
- Create: `migrations/versioned/000189_task_archive.up.sql`
- Create: `migrations/versioned/000189_task_archive.down.sql`
- Create: `migrations/sqlite/000110_task_archive.up.sql`
- Create: `migrations/sqlite/000110_task_archive.down.sql`
- Create: `internal/application/repository/workbench_task_state.go`
- Test: `internal/application/repository/workbench_task_state_test.go`
- Create: `internal/handler/session/workbench_task_state.go`
- Test: `internal/handler/session/workbench_task_state_test.go`
- Modify: `internal/router/routes_workbench.go`（文件末尾追加注册函数）
- Modify: `internal/router/router.go`（`RouterParams` 增字段；`RegisterWorkbenchOverviewRoutes` 调用之后追加一行注册）
- Modify: `internal/container/container.go`（`must(container.Provide(NewWorkbenchInboxHandler))` 之后追加两个 Provide）
- Modify: `internal/container/workbench.go`（文件末尾追加 handler provider）

**Interfaces:**
- Consumes: 既有 `openRunTestDB`/`insertWorkbenchRun`/`seedWorkbenchListFixtures` 测试夹具（`internal/application/repository/agent_run_test.go:29`、`workbench_list_test.go:22`）；既有双通道身份提取模式（`internal/handler/session/workbench_list.go:38-52`）；`agentruntime.ErrNotFound`（`internal/modules/agentruntime/agent/runtime`）。
- Produces: `repository.NewWorkbenchTaskStateStore(db *gorm.DB) *WorkbenchTaskStateStore`；`(*WorkbenchTaskStateStore).SetTaskArchived(ctx context.Context, tenantID uint64, ownerID, taskID string, archived bool, now time.Time) error`（归属校验失败返回 `repository.ErrWorkbenchTaskNotFound`，非法身份返回 `agentruntime.ErrNotFound`）；`session.NewWorkbenchTaskStateHandler(states OwnedTaskStateMutator) *WorkbenchTaskStateHandler`（`Archive`/`Restore` gin handler）；路由 `POST /api/v1/workbench/tasks/:task_id/archive`、`DELETE /api/v1/workbench/tasks/:task_id/archive`（响应 `{"success":true,"data":{"task_id":"...","archived":true|false}}`）。Task 2/3 消费 `sessions.archived_at` 列；Task 7/9 消费该路由。

- [ ] **Step 1: 写失败的仓储测试**

`internal/application/repository/workbench_task_state_test.go`（新文件，完整内容）：

```go
package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func archivedAt(t *testing.T, db *gorm.DB, sessionID string) *time.Time {
	t.Helper()
	var row struct {
		ArchivedAt *time.Time
	}
	require.NoError(t, db.Raw("SELECT archived_at FROM sessions WHERE id = ?", sessionID).Scan(&row).Error)
	return row.ArchivedAt
}

// TestWorkbenchTaskStateArchiveOwnershipAndIsolation: 归档谓词与读模型一致——
// 只有「该 tenant 内拥有该 session 至少一个 run」的 caller 才能归档/恢复；
// 同租户他人 session、异租户 session 一律 ErrWorkbenchTaskNotFound。
func TestWorkbenchTaskStateArchiveOwnershipAndIsolation(t *testing.T) {
	db := openRunTestDB(t)
	seedWorkbenchListFixtures(t, db)
	base := time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC)
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-a1", session: "s1", status: "running", agent: "agent-x", target: "platform", at: base})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u2", runID: "r-b1", session: "s3", status: "running", agent: "agent-x", target: "platform", at: base.Add(time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 2, owner: "v1", runID: "r-c1", session: "t1", status: "running", agent: "agent-x", target: "platform", at: base.Add(2 * time.Second)})

	store := NewWorkbenchTaskStateStore(db)
	ctx := context.Background()
	now := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)

	require.NoError(t, store.SetTaskArchived(ctx, 1, "u1", "s1", true, now))
	at := archivedAt(t, db, "s1")
	require.NotNil(t, at)
	require.Equal(t, now.UTC(), at.UTC())

	// 同租户他人 session、异租户 session：归属谓词拒绝，不写任何行。
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, "u1", "s3", true, now), ErrWorkbenchTaskNotFound)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, "u1", "t1", true, now), ErrWorkbenchTaskNotFound)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 2, "v1", "s1", true, now), ErrWorkbenchTaskNotFound)
	require.Nil(t, archivedAt(t, db, "s3"))
	require.Nil(t, archivedAt(t, db, "t1"))

	// 恢复：archived_at 置 NULL，幂等可重复。
	require.NoError(t, store.SetTaskArchived(ctx, 1, "u1", "s1", false, now))
	require.Nil(t, archivedAt(t, db, "s1"))
	require.NoError(t, store.SetTaskArchived(ctx, 1, "u1", "s1", false, now))

	// 非法身份：零租户/空 owner/空 taskId 一律 ErrNotFound，零查询副作用。
	require.ErrorIs(t, store.SetTaskArchived(ctx, 0, "u1", "s1", true, now), agentruntime.ErrNotFound)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, " ", "s1", true, now), agentruntime.ErrNotFound)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, "u1", "  ", true, now), agentruntime.ErrNotFound)
	require.False(t, errors.Is(ErrWorkbenchTaskNotFound, agentruntime.ErrNotFound), "sentinel must stay distinct from ErrNotFound")
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run TestWorkbenchTaskStateArchiveOwnershipAndIsolation -count=1`
Expected: FAIL——编译错误 `undefined: NewWorkbenchTaskStateStore`（及 `ErrWorkbenchTaskNotFound`）。

- [ ] **Step 3: 最小实现（迁移 + 仓储）**

(a) 迁移文件（四个，内容完整）：

`migrations/versioned/000189_task_archive.up.sql`：
```sql
-- Task lifecycle archived state (T04). Task = Session is the single task
-- identity (ADR-0004), so the archive timestamp lives on the task itself.
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
```

`migrations/versioned/000189_task_archive.down.sql`：
```sql
ALTER TABLE sessions DROP COLUMN IF EXISTS archived_at;
```

`migrations/sqlite/000110_task_archive.up.sql`：
```sql
-- Task lifecycle archived state (T04). Task = Session is the single task
-- identity (ADR-0004), so the archive timestamp lives on the task itself.
ALTER TABLE sessions ADD COLUMN archived_at DATETIME;
```

`migrations/sqlite/000110_task_archive.down.sql`：
```sql
ALTER TABLE sessions DROP COLUMN archived_at;
```

（sqlite `DROP COLUMN` 先例：`migrations/sqlite/000002_knowledge_folder_path.down.sql` 等。**编号惯例**：`migrations/sqlite/` 与 `migrations/versioned/` 是两套独立序列——作者实跑 `ls migrations/sqlite | sort -n | tail -1` 为 `000109_tenant_agent_marketplace`、versioned 止于 `000188`；golang-migrate 按各目录自身版本号排序执行（openRunTestDB 即走 migrations/sqlite，agent_run_test.go:41-49），sqlite 侧必须从 000110 续编而非沿用 versioned 的 000189，否则后续按 sqlite 惯例编号的迁移会排在已应用的更大版本之前造成乱序倒挂。）

(b) `internal/application/repository/workbench_task_state.go`（新文件，完整内容）：

```go
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"gorm.io/gorm"
)

// ErrWorkbenchTaskNotFound marks an archive/restore request whose task the
// caller does not own in this tenant. The read model's ownership predicate
// (agent_runs.tenant_id + owner_id) is the single authority.
var ErrWorkbenchTaskNotFound = errors.New("workbench task not found for owner")

type WorkbenchTaskStateStore struct{ db *gorm.DB }

func NewWorkbenchTaskStateStore(db *gorm.DB) *WorkbenchTaskStateStore {
	return &WorkbenchTaskStateStore{db: db}
}

// SetTaskArchived archives or restores the caller's task (taskId = sessionId,
// ADR-0004). Ownership follows the read model exactly: the caller must own at
// least one run of this session inside this tenant. Identity always comes from
// the authenticated context at the handler layer, never from the request body.
func (s *WorkbenchTaskStateStore) SetTaskArchived(ctx context.Context, tenantID uint64, ownerID, taskID string, archived bool, now time.Time) error {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(taskID) == "" {
		return agentruntime.ErrNotFound
	}
	taskID = strings.TrimSpace(taskID)
	var owned int64
	if err := s.db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND session_id = ? AND owner_id = ?", tenantID, taskID, ownerID).
		Limit(1).Count(&owned).Error; err != nil {
		return err
	}
	if owned == 0 {
		return ErrWorkbenchTaskNotFound
	}
	var value any
	if archived {
		value = now.UTC()
	}
	return s.db.WithContext(ctx).Table("sessions").
		Where("tenant_id = ? AND id = ?", tenantID, taskID).
		Update("archived_at", value).Error
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/application/repository/ -run TestWorkbenchTaskStateArchiveOwnershipAndIsolation -count=1`
Expected: PASS。

- [ ] **Step 5: 写失败的 handler 测试**

`internal/handler/session/workbench_task_state_test.go`（新文件，完整内容）：

```go
package session

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeTaskStateMutator struct {
	err     error
	lastArg struct {
		tenantID uint64
		ownerID  string
		taskID   string
		archived bool
	}
}

func (f *fakeTaskStateMutator) SetTaskArchived(_ context.Context, tenantID uint64, ownerID, taskID string, archived bool, _ time.Time) error {
	f.lastArg.tenantID, f.lastArg.ownerID, f.lastArg.taskID, f.lastArg.archived = tenantID, ownerID, taskID, archived
	return f.err
}

func taskStateContext(tenantID uint64, userID string) (context.Context, *gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := context.Background()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/workbench/tasks/s-1/archive", nil).WithContext(ctx)
	return ctx, c, recorder
}

func TestWorkbenchTaskStateHandlerMapsIdentityAndErrors(t *testing.T) {
	mutator := &fakeTaskStateMutator{}
	handler := &WorkbenchTaskStateHandler{states: mutator}

	// 无身份：401，mutator 未被调用。
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/workbench/tasks/s-1/archive", nil)
	handler.Archive(c)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Empty(t, mutator.lastArg.taskID)

	// 归属不存在：404（ErrWorkbenchTaskNotFound）。
	notFound := &fakeTaskStateMutator{err: repository.ErrWorkbenchTaskNotFound}
	nfHandler := &WorkbenchTaskStateHandler{states: notFound}
	_, c, recorder = taskStateContext(7, "u1")
	c.Params = gin.Params{{Key: "task_id", Value: "s-1"}}
	nfHandler.Archive(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)

	// 成功归档：身份取自 context（非路由参数），task_id 取自路由。
	_, c, recorder = taskStateContext(7, "u1")
	c.Params = gin.Params{{Key: "task_id", Value: "s-1"}}
	handler.Archive(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, uint64(7), mutator.lastArg.tenantID)
	require.Equal(t, "u1", mutator.lastArg.ownerID)
	require.Equal(t, "s-1", mutator.lastArg.taskID)
	require.True(t, mutator.lastArg.archived)
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			TaskID    string `json:"task_id"`
			Archived  bool   `json:"archived"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	require.Equal(t, "s-1", envelope.Data.TaskID)
	require.True(t, envelope.Data.Archived)

	// 恢复：DELETE 同一路径，archived=false。
	_, c, recorder = taskStateContext(7, "u1")
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/workbench/tasks/s-1/archive", nil).WithContext(c.Request.Context())
	c.Params = gin.Params{{Key: "task_id", Value: "s-1"}}
	handler.Restore(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.False(t, mutator.lastArg.archived)

	// 其它仓储错误：500，不吞错。
	broken := &fakeTaskStateMutator{err: errors.New("db down")}
	brokenHandler := &WorkbenchTaskStateHandler{states: broken}
	_, c, recorder = taskStateContext(7, "u1")
	c.Params = gin.Params{{Key: "task_id", Value: "s-1"}}
	brokenHandler.Archive(c)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
```

- [ ] **Step 6: 运行确认失败**

Run: `go test ./internal/handler/session/ -run TestWorkbenchTaskStateHandlerMapsIdentityAndErrors -count=1`
Expected: FAIL——编译错误 `undefined: WorkbenchTaskStateHandler`。

- [ ] **Step 7: 最小实现（handler + 路由 + 装配）**

(a) `internal/handler/session/workbench_task_state.go`（新文件，完整内容）：

```go
package session

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// OwnedTaskStateMutator is the ownership-scoped archive port. Tenant and owner
// are always derived from the authenticated context; implementations bind
// every mutation to that identity.
type OwnedTaskStateMutator interface {
	SetTaskArchived(ctx context.Context, tenantID uint64, ownerID, taskID string, archived bool, now time.Time) error
}

// WorkbenchTaskStateHandler serves the task archive lifecycle (T04). Archive
// is a write, so it is NOT mounted behind the W34 read gate — one switch must
// never cut query and archive at the same time.
type WorkbenchTaskStateHandler struct {
	states OwnedTaskStateMutator
}

func NewWorkbenchTaskStateHandler(states OwnedTaskStateMutator) *WorkbenchTaskStateHandler {
	return &WorkbenchTaskStateHandler{states: states}
}

func (h *WorkbenchTaskStateHandler) mutate(c *gin.Context, archived bool) {
	if h == nil || h.states == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	tenantID, tenantOK := types.TenantIDFromContext(c.Request.Context())
	if !tenantOK || tenantID == 0 {
		if value, exists := c.Get(types.TenantIDContextKey.String()); exists {
			tenantID, tenantOK = value.(uint64)
		}
	}
	userID, userOK := types.UserIDFromContext(c.Request.Context())
	if !userOK || userID == "" {
		if value, exists := c.Get(types.UserIDContextKey.String()); exists {
			userID, userOK = value.(string)
		}
	}
	if !tenantOK || !userOK || tenantID == 0 || userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return
	}
	taskID := c.Param("task_id")
	err := h.states.SetTaskArchived(c.Request.Context(), tenantID, userID, taskID, archived, time.Now().UTC())
	switch {
	case errors.Is(err, repository.ErrWorkbenchTaskNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "error": "task not found for owner"})
	case err != nil:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
	default:
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"task_id": taskID, "archived": archived}})
	}
}

func (h *WorkbenchTaskStateHandler) Archive(c *gin.Context)  { h.mutate(c, true) }
func (h *WorkbenchTaskStateHandler) Restore(c *gin.Context) { h.mutate(c, false) }
```

(b) `internal/router/routes_workbench.go` 末尾追加：

```go
// RegisterWorkbenchTaskStateRoutes exposes the task archive lifecycle (T04).
// Same Viewer/API-key boundary as the other workbench lanes; the handler
// applies the tenant+owner ownership predicate. Archive is a write and is
// deliberately NOT behind the W34 read gate.
func RegisterWorkbenchTaskStateRoutes(r *gin.RouterGroup, h *session.WorkbenchTaskStateHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	tasks := g.apiKeyGroup(r.Group("/workbench/tasks", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	tasks.POST("/:task_id/archive", h.Archive)
	tasks.DELETE("/:task_id/archive", h.Restore)
}
```

(c) `internal/router/router.go`：`RouterParams` 中 `WorkbenchOverviewHandler` 字段之后追加：

```go
	WorkbenchTaskStateHandler    *session.WorkbenchTaskStateHandler    `optional:"true"`
```

`RegisterWorkbenchOverviewRoutes(v1, params.WorkbenchOverviewHandler, rbacGuards)`（router.go:370）之后追加一行：

```go
		RegisterWorkbenchTaskStateRoutes(v1, params.WorkbenchTaskStateHandler, rbacGuards)
```

(d) `internal/container/container.go`：`must(container.Provide(NewWorkbenchInboxHandler))`（container.go:282 附近）之后追加：

```go
	must(container.Provide(repository.NewWorkbenchTaskStateStore))
	must(container.Provide(NewWorkbenchTaskStateHandler))
```

(e) `internal/container/workbench.go` 末尾追加：

```go
// NewWorkbenchTaskStateHandler wires the task archive lifecycle to the same
// ownership predicate the read model uses; tenant/owner always come from the
// authenticated context.
func NewWorkbenchTaskStateHandler(states *repository.WorkbenchTaskStateStore) *session.WorkbenchTaskStateHandler {
	return session.NewWorkbenchTaskStateHandler(states)
}
```

- [ ] **Step 8: 运行确认通过（含编译装配）**

Run: `go test ./internal/handler/session/ -run TestWorkbenchTaskStateHandlerMapsIdentityAndErrors -count=1 && go build ./internal/router/ ./internal/container/`
Expected: 双双 PASS / 无输出（编译成功）。

- [ ] **Step 9: Commit**

```bash
git add migrations/versioned/000189_task_archive.up.sql migrations/versioned/000189_task_archive.down.sql migrations/sqlite/000110_task_archive.up.sql migrations/sqlite/000110_task_archive.down.sql internal/application/repository/workbench_task_state.go internal/application/repository/workbench_task_state_test.go internal/handler/session/workbench_task_state.go internal/handler/session/workbench_task_state_test.go internal/router/routes_workbench.go internal/router/router.go internal/container/container.go internal/container/workbench.go
git commit -m "feat(workbench): task archive lifecycle with owner-scoped write path (T04)"
```

---

### Task 2: Go — 统一列表搜索、归档筛选与 attention/标题投影

**Files:**
- Modify: `internal/application/repository/workbench_list.go`
- Test: `internal/application/repository/workbench_list_test.go`（追加测试函数）
- Modify: `internal/handler/session/workbench_list.go`（Step 3(f)：解析 `q`/`archived` 查询参数）

**Interfaces:**
- Consumes: Task 1 的 `sessions.archived_at` 列；既有 `workbenchListCursor`/`snapshotAgentExpr`/`listOrderExpr`/`listKeysetPredicate`（workbench_list.go:60-131）。
- Produces: `WorkbenchExecutionFilter` 新字段 `Query string`（标题子串搜索）与 `ArchivedOnly bool`（默认 false=仅未归档；true=仅已归档）；`WorkbenchExecutionSummary` 新字段 `Title string`（json `title,omitempty`）、`Attention string`（json `attention`，`"none"|"required"`；`waiting_user` 或存在 pending interaction 即 `required`）、`ArchivedAt string`（json `archived_at,omitempty`，RFC3339Nano）；cursor payload 新键 `q`/`archived_only` 并纳入绑定校验。Task 7/9 消费这些 wire 字段与 `q`/`archived` 查询参数（handler 层解析见 Step 3(f)）。

- [ ] **Step 1: 写失败的列表测试**

`internal/application/repository/workbench_list_test.go` 末尾追加（完整代码）：

```go
// seedSearchableSessions adds titled sessions that pin LIKE-escape semantics:
// '%' and '_' must match literally, never as wildcards.
func seedSearchableSessions(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES
		 ('s-pct', 1, '100% done', 'u1', 'trpc'),
		 ('s-pct-wild', 1, '100x done', 'u1', 'trpc'),
		 ('s-und', 1, 'status_report', 'u1', 'trpc'),
		 ('s-und-wild', 1, 'statusXreport', 'u1', 'trpc'),
		 ('s-long', 1, 'quarterly review', 'u1', 'trpc')`,
	).Error)
}

// TestWorkbenchListSearchFiltersByTaskTitleWithinOwnerScope: 搜索按任务标题
// 子串、忽略大小写，且始终在 tenant+owner 谓词内——同租户他人、异租户的
// 同名邻居不可见；LIKE 通配符按字面匹配；全空白与空串等价于无搜索。
func TestWorkbenchListSearchFiltersByTaskTitleWithinOwnerScope(t *testing.T) {
	db := openRunTestDB(t)
	seedWorkbenchListFixtures(t, db)
	seedSearchableSessions(t, db)
	base := time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-pct", session: "s-pct", status: "running", agent: "agent-x", target: "platform", at: base})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-pct-wild", session: "s-pct-wild", status: "running", agent: "agent-x", target: "platform", at: base.Add(time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-und", session: "s-und", status: "succeeded", agent: "agent-y", target: "platform", at: base.Add(2 * time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-und-wild", session: "s-und-wild", status: "succeeded", agent: "agent-y", target: "platform", at: base.Add(3 * time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-long", session: "s-long", status: "failed", agent: "agent-z", target: "platform", at: base.Add(4 * time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u2", runID: "r-neighbor", session: "s3", status: "running", agent: "agent-x", target: "platform", at: base.Add(5 * time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 2, owner: "v1", runID: "r-foreign", session: "t1", status: "running", agent: "agent-x", target: "platform", at: base.Add(6 * time.Second)})

	store := NewWorkbenchListStore(db)
	ctx := context.Background()

	// 子串、忽略大小写。
	page, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Query: "quarterly"})
	require.NoError(t, err)
	require.Equal(t, []string{"r-long"}, runIDs(page))
	require.Equal(t, "quarterly review", page.Items[0].Title)

	// '%' 字面匹配：不得把 '100x done' 通配进来。
	page, err = store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Query: "100%"})
	require.NoError(t, err)
	require.Equal(t, []string{"r-pct"}, runIDs(page))

	// '_' 字面匹配：不得把 'statusXreport' 通配进来。
	page, err = store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Query: "status_report"})
	require.NoError(t, err)
	require.Equal(t, []string{"r-und"}, runIDs(page))

	// 反斜杠字面量不破坏查询。
	page, err = store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Query: `100\%`})
	require.NoError(t, err)
	require.Empty(t, runIDs(page))

	// 搜索始终在 owner 谓词内：u2 搜 'session-1'（u1 的标题）为空。
	page, err = store.ListOwnedExecutions(ctx, 1, "u2", WorkbenchExecutionFilter{Query: "session-1"})
	require.NoError(t, err)
	require.Empty(t, runIDs(page))
	// 异租户同理：v1 搜 'quarterly' 为空（其会话标题为 session-t1）。
	page, err = store.ListOwnedExecutions(ctx, 2, "v1", WorkbenchExecutionFilter{Query: "quarterly"})
	require.NoError(t, err)
	require.Empty(t, runIDs(page))

	// 全空白与空串等价：归一化后是无搜索，返回全部 5 条。
	page, err = store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Query: "   "})
	require.NoError(t, err)
	require.Len(t, page.Items, 5)
}

// TestWorkbenchListArchiveFilterAndAttentionProjection: 默认视图只含未归档；
// ArchivedOnly 只含已归档并携带 archived_at；attention 由 waiting_user 或
// pending interaction 派生，其余为 none。
func TestWorkbenchListArchiveFilterAndAttentionProjection(t *testing.T) {
	db := openRunTestDB(t)
	seedWorkbenchListFixtures(t, db)
	base := time.Date(2026, 9, 23, 7, 0, 0, 0, time.UTC)
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-keep", session: "s1", status: "running", agent: "agent-x", target: "platform", at: base})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-gone", session: "s2", status: "succeeded", agent: "agent-y", target: "platform", at: base.Add(time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-wait", session: "s3", status: "waiting_user", agent: "agent-x", target: "platform", at: base.Add(2 * time.Second)})
	archived := base.Add(time.Hour)
	require.NoError(t, db.Exec("UPDATE sessions SET archived_at = ? WHERE id = 's2'", archived).Error)
	// r-keep 有 pending interaction → attention=required（即使状态不是 waiting_user）。
	require.NoError(t, db.Exec(
		`INSERT INTO workbench_interactions (tenant_id, id, run_id, owner_id, kind, args_hash, status, expected_revision, created_at, updated_at)
		 VALUES (1, 'ix-1', 'r-keep', 'u1', 'tool_approval', 'h1', 'pending', 1, ?, ?)`, archived, archived,
	).Error)

	store := NewWorkbenchListStore(db)
	ctx := context.Background()

	page, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{})
	require.NoError(t, err)
	require.Equal(t, []string{"r-wait", "r-keep"}, runIDs(page))
	require.Equal(t, "session-1", page.Items[1].Title)

	byID := func(items []WorkbenchExecutionSummary) map[string]WorkbenchExecutionSummary {
		out := map[string]WorkbenchExecutionSummary{}
		for _, item := range items {
			out[item.RunID] = item
		}
		return out
	}
	all := byID(page.Items)
	require.Equal(t, "required", all["r-wait"].Attention, "waiting_user derives attention")
	require.Equal(t, "required", all["r-keep"].Attention, "a pending interaction derives attention")
	require.Empty(t, all["r-wait"].ArchivedAt)

	archivedOnly, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{ArchivedOnly: true})
	require.NoError(t, err)
	require.Equal(t, []string{"r-gone"}, runIDs(archivedOnly))
	require.Equal(t, "session-2", archivedOnly.Items[0].Title)
	require.Equal(t, archived.UTC().Format(time.RFC3339Nano), archivedOnly.Items[0].ArchivedAt)
	require.Equal(t, "none", archivedOnly.Items[0].Attention)

	// 归档的行对异 owner 依然不可见（u2 在同租户）。
	foreign, err := store.ListOwnedExecutions(ctx, 1, "u2", WorkbenchExecutionFilter{ArchivedOnly: true})
	require.NoError(t, err)
	require.Empty(t, runIDs(foreign))
}

// TestWorkbenchListCursorBindsSearchAndArchiveFacets: cursor 携带并校验
// q/archived_only——换 facet 重放一律拒绝，原 facet 续页有效。
func TestWorkbenchListCursorBindsSearchAndArchiveFacets(t *testing.T) {
	db := openRunTestDB(t)
	seedSearchableSessions(t, db)
	base := time.Date(2026, 9, 23, 6, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		insertWorkbenchRun(t, db, workbenchRunSeed{
			tenant: 1, owner: "u1", runID: fmt.Sprintf("rs-%d", i), session: "s-und",
			status: "succeeded", agent: "agent-y", target: "platform", at: base.Add(time.Duration(i) * time.Second),
		})
	}
	store := NewWorkbenchListStore(db)
	ctx := context.Background()

	page, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Query: "status_report", Limit: 2})
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.NotEmpty(t, page.NextCursor)

	for name, filter := range map[string]WorkbenchExecutionFilter{
		"replayed without search":  {Cursor: page.NextCursor},
		"replayed with other term": {Query: "quarterly", Cursor: page.NextCursor},
		"replayed archived":        {Query: "status_report", ArchivedOnly: true, Cursor: page.NextCursor},
	} {
		_, err := store.ListOwnedExecutions(ctx, 1, "u1", filter)
		require.ErrorIs(t, err, ErrWorkbenchCursor, "cursor case %q must be rejected", name)
	}

	rest, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Query: "status_report", Limit: 2, Cursor: page.NextCursor})
	require.NoError(t, err)
	require.Equal(t, []string{"rs-0"}, runIDs(rest))
	require.Empty(t, rest.NextCursor)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run 'TestWorkbenchListSearchFiltersByTaskTitleWithinOwnerScope|TestWorkbenchListArchiveFilterAndAttentionProjection|TestWorkbenchListCursorBindsSearchAndArchiveFacets' -count=1`
Expected: FAIL——编译错误 `unknown field 'Query' in struct literal`（`WorkbenchExecutionFilter.Query`、`ArchivedOnly` 不存在）。

- [ ] **Step 3: 最小实现**

(a) `internal/application/repository/workbench_list.go`——`WorkbenchExecutionFilter` 改为：

```go
type WorkbenchExecutionFilter struct {
	Status      string
	AgentID     string
	Query       string // 任务标题子串搜索（忽略大小写；全空白归一为无搜索）
	ArchivedOnly bool  // false（默认）=仅未归档；true=仅已归档
	Cursor      string
	Limit       int
}
```

`WorkbenchExecutionSummary` 的 `Status` 字段之后追加三行：

```go
	Title      string `json:"title,omitempty"`
	Attention  string `json:"attention"` // "none" | "required"：waiting_user 或存在 pending interaction
	ArchivedAt string `json:"archived_at,omitempty"`
```

(b) 同文件追加常量与辅助（放在 `ErrWorkbenchCursor` 块之后）：

```go
const workbenchSearchMaxLen = 200

// normalizeWorkbenchSearch collapses whitespace and caps length so a hostile
// or accidental giant term cannot balloon the LIKE pattern.
func normalizeWorkbenchSearch(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len(value) > workbenchSearchMaxLen {
		value = value[:workbenchSearchMaxLen]
	}
	return value
}

// likeEscaped escapes LIKE wildcards so user input matches literally.
func likeEscaped(term string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term)
}

// searchPredicate is dialect-specific only in the ESCAPE literal: MySQL parses
// a single backslash as the empty string, PostgreSQL/SQLite do not.
func searchPredicate(db *gorm.DB) string {
	if db.Dialector.Name() == "mysql" {
		return "LOWER(sessions.title) LIKE LOWER(?) ESCAPE '\\\\'"
	}
	return "LOWER(sessions.title) LIKE LOWER(?) ESCAPE '\\'"
}

// attentionPendingExpr is a portable EXISTS projection: one query, no
// per-session follow-up reads (the N+1 rule applies to reads, not SQL).
const attentionPendingExpr = `EXISTS (SELECT 1 FROM workbench_interactions wi
	WHERE wi.tenant_id = agent_runs.tenant_id AND wi.run_id = agent_runs.run_id
	AND wi.owner_id = agent_runs.owner_id AND wi.status = 'pending')`

func attentionOf(status string, pendingInteraction bool) string {
	if status == "waiting_user" || pendingInteraction {
		return "required"
	}
	return "none"
}
```

(c) `workbenchListCursor` 结构体追加两个字段（`Status` 之后）：

```go
	Query       string `json:"q,omitempty"`
	ArchivedOnly bool  `json:"archived_only,omitempty"`
```

`workbenchListRow` 追加（`Snapshot` 之后）：

```go
	Title            string
	ArchivedAt       *time.Time
	AttentionPending bool
```

`listOrderExpr` 与 `listKeysetPredicate` 的 `created_at`/`run_id` 全部加 `agent_runs.` 前缀（join 后列名歧义）：

```go
func listOrderExpr(db *gorm.DB) string {
	if db.Dialector.Name() == "sqlite" {
		return "julianday(agent_runs.created_at) DESC, agent_runs.run_id DESC"
	}
	return "agent_runs.created_at DESC, agent_runs.run_id DESC"
}

func listKeysetPredicate(db *gorm.DB, anchor time.Time, runID string) string {
	if db.Dialector.Name() == "sqlite" {
		return "(julianday(agent_runs.created_at) < julianday(?) OR (julianday(agent_runs.created_at) = julianday(?) AND agent_runs.run_id < ?))"
	}
	return "(agent_runs.created_at < ? OR (agent_runs.created_at = ? AND agent_runs.run_id < ?))"
}
```

(d) `ListOwnedExecutions` 中：`filter.AgentID = strings.TrimSpace(filter.AgentID)` 之后追加 `filter.Query = normalizeWorkbenchSearch(filter.Query)`；查询构造改为（替换现有 `query := ...` 到 `filter.AgentID != ""` 块）：

```go
	query := s.db.WithContext(ctx).Table("agent_runs").
		Select("agent_runs.tenant_id, agent_runs.run_id, agent_runs.session_id, agent_runs.status, agent_runs.wait_reason, agent_runs.snapshot, agent_runs.created_at, agent_runs.updated_at, sessions.title AS title, sessions.archived_at AS archived_at, " + attentionPendingExpr + " AS attention_pending").
		Joins("JOIN sessions ON sessions.tenant_id = agent_runs.tenant_id AND sessions.id = agent_runs.session_id").
		Where("agent_runs.tenant_id = ? AND agent_runs.owner_id = ?", tenantID, ownerID)
	if filter.Status != "" {
		query = query.Where("agent_runs.status = ?", filter.Status)
	}
	if filter.AgentID != "" {
		query = query.Where(snapshotAgentExpr(s.db)+" = ?", filter.AgentID)
	}
	if filter.Query != "" {
		query = query.Where(searchPredicate(s.db), "%"+likeEscaped(filter.Query)+"%")
	}
	if filter.ArchivedOnly {
		query = query.Where("sessions.archived_at IS NOT NULL")
	} else {
		query = query.Where("sessions.archived_at IS NULL")
	}
```

cursor 校验块中的绑定比较改为：

```go
		if cursor.TenantID != tenantID || cursor.OwnerID != ownerID ||
			cursor.AgentID != filter.AgentID || cursor.Status != filter.Status ||
			cursor.Query != filter.Query || cursor.ArchivedOnly != filter.ArchivedOnly {
			return WorkbenchExecutionPage{}, fmt.Errorf("%w: cursor does not match the active filter", ErrWorkbenchCursor)
		}
```

`NextCursor` 编码处追加 `Query: filter.Query, ArchivedOnly: filter.ArchivedOnly,`。

(e) `workbenchSummaryFromRow` 的赋值块改为：

```go
	summary := WorkbenchExecutionSummary{
		RunID: row.RunID, SessionID: row.SessionID,
		Status: row.Status, WaitReason: row.WaitReason,
		Title:     strings.TrimSpace(row.Title),
		Attention: attentionOf(row.Status, row.AttentionPending),
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if row.ArchivedAt != nil {
		summary.ArchivedAt = row.ArchivedAt.UTC().Format(time.RFC3339Nano)
	}
```

(f) `internal/handler/session/workbench_list.go`：`filter := repository.WorkbenchExecutionFilter{...}` 改为携带 `Query: c.Query("q"),`，并在 limit 解析块之后追加：

```go
	if raw := strings.TrimSpace(c.Query("archived")); raw != "" {
		archived, err := strconv.ParseBool(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "archived must be a boolean"})
			return
		}
		filter.ArchivedOnly = archived
	}
```

- [ ] **Step 4: 运行确认通过（新测试 + 既有回归）**

Run: `go test ./internal/application/repository/ -run TestWorkbenchList -count=1 && go test ./internal/handler/session/ -run TestWorkbenchList -count=1`
Expected: PASS（新增 3 个 + 既有全部列表/handler 测试）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/repository/workbench_list.go internal/application/repository/workbench_list_test.go internal/handler/session/workbench_list.go
git commit -m "feat(workbench): unified list gains title search, archive facet and attention projection (T04)"
```

---

### Task 3: Go — workbench_notifications 建表迁移、overview 最近完成段、标题投影与真实 unread 计数

**Files:**
- Create: `migrations/versioned/000190_workbench_notifications.up.sql`
- Create: `migrations/versioned/000190_workbench_notifications.down.sql`
- Create: `migrations/sqlite/000111_workbench_notifications.up.sql`
- Create: `migrations/sqlite/000111_workbench_notifications.down.sql`
- Test: `internal/application/repository/workbench_notifications_migration_test.go`（新文件：迁移↔投影列集对齐的 RED→GREEN 测试）
- Modify: `internal/modules/workbench/service/workbench/overview.go`
- Test: `internal/handler/session/workbench_overview_mx013_test.go`（扩展既有测试 + 新增测试）

**Interfaces:**
- Consumes: Task 1 的 `sessions.archived_at`；既有 `workbench_interactions` 表（`OverviewInteractionRow` 投影先例）与 `InboxNotificationRow` 投影（`internal/handler/session/workbench_inbox.go:21-33`，TableName `workbench_notifications`）；终态结算投影先例 `agent_run_snapshot.go:130`（terminal → `"settled"`）；迁移测试夹具 `openRunTestDB`（`internal/application/repository/agent_run_test.go:29`，真实跑 `migrations/sqlite`）。
- Produces: 生产库新表 `workbench_notifications`（versioned 000190 / sqlite 000111，列集对齐 `InboxNotificationRow`：tenant_id/id/owner_id/kind/title/body/deep_link/read/created_at——同时修复既有 `InboxService.Inbox/MarkRead` 在生产库缺表的问题，见差异记录 11）；`Overview.RecentlyCompleted []OverviewRunSummary`（json `recently_completed`，恒非 nil）；`OverviewRunSummary.Title string`（json `title,omitempty`）与 `Attention string`（json `attention`，`"none"|"required"`，`waiting_user` → `required`）；`Counts.UnreadNotifications` 从 `workbench_notifications` 真实计数；三段查询均 JOIN sessions 排除已归档。Task 4（contracts）与 Task 7/9 消费该 wire 形状。

- [ ] **Step 1: 写失败的迁移对齐测试（先证明生产库缺表）**

`internal/application/repository/workbench_notifications_migration_test.go`（新文件，完整内容；`openRunTestDB` 真实执行 `migrations/sqlite` 全量迁移，是「生产库由迁移建出」的同一事实源）：

```go
package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWorkbenchNotificationsTableExistsAfterMigrations: workbench_notifications
// 是 InboxService（MX-021）与 overview unread 计数的读模型底表，必须由生产迁移
// 建出（此前仅存在于测试 AutoMigrate，生产库缺表——见计划差异记录 11）。
// 列集与 InboxNotificationRow（internal/handler/session/workbench_inbox.go:21-33）
// 逐列对齐。
func TestWorkbenchNotificationsTableExistsAfterMigrations(t *testing.T) {
	db := openRunTestDB(t)
	require.True(t, db.Migrator().HasTable("workbench_notifications"),
		"workbench_notifications must be created by the production migrations")
	for _, column := range []string{"tenant_id", "id", "owner_id", "kind", "title", "body", "deep_link", "read", "created_at"} {
		require.True(t, db.Migrator().HasColumn("workbench_notifications", column),
			"workbench_notifications.%s must exist (aligned with InboxNotificationRow)", column)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run TestWorkbenchNotificationsTableExistsAfterMigrations -count=1`
Expected: FAIL——`workbench_notifications must be created by the production migrations`（迁移目录无此表，作者已实跑 `grep -rln workbench_notifications migrations/sqlite/ migrations/versioned/` 无输出）。

- [ ] **Step 3: 建表迁移（versioned + sqlite 双份）**

`migrations/versioned/000190_workbench_notifications.up.sql`：

```sql
-- Workbench notification read model (MX-021 backing store for GET /workbench/inbox
-- and the overview unread count). Column set aligns with InboxNotificationRow
-- (internal/handler/session/workbench_inbox.go): tenant_id/id/owner_id/kind/title/
-- body/deep_link/read/created_at.
CREATE TABLE workbench_notifications (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    kind VARCHAR(64) NOT NULL,
    title VARCHAR(255) NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    deep_link VARCHAR(512) NOT NULL DEFAULT '',
    read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workbench_notifications_owner
    ON workbench_notifications (tenant_id, owner_id, read, created_at);
```

`migrations/versioned/000190_workbench_notifications.down.sql`：

```sql
DROP TABLE IF EXISTS workbench_notifications;
```

`migrations/sqlite/000111_workbench_notifications.up.sql`：

```sql
-- Workbench notification read model (MX-021 backing store for GET /workbench/inbox
-- and the overview unread count). Column set aligns with InboxNotificationRow.
CREATE TABLE workbench_notifications (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    owner_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    deep_link TEXT NOT NULL DEFAULT '',
    read BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_workbench_notifications_owner
    ON workbench_notifications (tenant_id, owner_id, read, created_at);
```

`migrations/sqlite/000111_workbench_notifications.down.sql`：

```sql
DROP TABLE workbench_notifications;
```

（sqlite 侧编号续 000111，与 Task 1 的 000110 同理——两套独立序列，见 Task 1 Step 3(a) 的编号惯例说明。）

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/application/repository/ -run TestWorkbenchNotificationsTableExistsAfterMigrations -count=1`
Expected: PASS。

- [ ] **Step 5: 写失败的 overview 测试**

`internal/handler/session/workbench_overview_mx013_test.go`——(a) 在既有 `TestMX013OverviewOwnerScope` 中：AutoMigrate 行（第 29 行）改为：

```go
	require.NoError(t, db.AutoMigrate(&workbenchservice.OverviewRunRow{}, &workbenchservice.OverviewInteractionRow{}, &workbenchservice.OverviewTaskRow{}, &workbenchservice.OverviewNotificationRow{}))
```

在其后追加 sessions 种子（`now` 变量定义之前）：

```go
	require.NoError(t, db.Create([]*workbenchservice.OverviewTaskRow{
		{TenantID: 7, ID: "s-1", Title: "weekly report"},
		{TenantID: 7, ID: "s-2", Title: "other user task"},
		{TenantID: 7, ID: "s-3", Title: "finished research"},
	}).Error)
	require.NoError(t, db.Create([]*workbenchservice.OverviewNotificationRow{
		{TenantID: 7, ID: "n-1", OwnerID: "u1", Read: false},
		{TenantID: 7, ID: "n-2", OwnerID: "u2", Read: false},
		{TenantID: 7, ID: "n-3", OwnerID: "u1", Read: true},
	}).Error)
```

并在 `require.Equal(t, int64(1), envelope.Data.Counts.PendingInteractions)` 之后追加断言：

```go
	require.Equal(t, int64(1), envelope.Data.Counts.UnreadNotifications, "unread counts only the caller's unread rows")
	require.Equal(t, "weekly report", first.Title)
	require.Equal(t, "none", first.Attention)
	completed := make([]string, 0, len(envelope.Data.RecentlyCompleted))
	for _, run := range envelope.Data.RecentlyCompleted {
		completed = append(completed, run.RunID)
	}
	require.Equal(t, []string{"owned-terminal"}, completed)
	require.Equal(t, "finished research", envelope.Data.RecentlyCompleted[0].Title)
	require.Equal(t, "settled", envelope.Data.RecentlyCompleted[0].SettlementStatus, "terminal runs project settlement per the snapshot precedent")
```

(b) 文件末尾新增测试（完整代码）：

```go
// TestMX013OverviewExcludesArchivedTasksFromEverySegment: 归档任务从
// in_progress、recently_completed 与 pending_interactions 全部落面消失。
func TestMX013OverviewExcludesArchivedTasksFromEverySegment(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/overview-archived.db?_foreign_keys=on&_busy_timeout=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&workbenchservice.OverviewRunRow{}, &workbenchservice.OverviewInteractionRow{}, &workbenchservice.OverviewTaskRow{}, &workbenchservice.OverviewNotificationRow{}))
	now := time.Now().UTC()
	archivedAt := now.Add(-time.Hour)
	require.NoError(t, db.Create([]*workbenchservice.OverviewTaskRow{
		{TenantID: 7, ID: "s-live", Title: "live task"},
		{TenantID: 7, ID: "s-archived", Title: "archived task", ArchivedAt: &archivedAt},
	}).Error)
	require.NoError(t, db.Create([]*workbenchservice.OverviewRunRow{
		{TenantID: 7, RunID: "live-run", SessionID: "s-live", OwnerID: "u1", Status: "running", UpdatedAt: now},
		{TenantID: 7, RunID: "archived-run", SessionID: "s-archived", OwnerID: "u1", Status: "running", UpdatedAt: now},
		{TenantID: 7, RunID: "archived-done", SessionID: "s-archived", OwnerID: "u1", Status: "succeeded", UpdatedAt: now},
	}).Error)
	require.NoError(t, db.Create(&workbenchservice.OverviewInteractionRow{
		TenantID: 7, ID: "i-archived", RunID: "archived-run", OwnerID: "u1", Kind: "tool_approval", ArgsHash: "h1", Status: "pending", ExpectedRevision: 1, CreatedAt: now, UpdatedAt: now,
	}).Error)

	service := workbenchservice.NewWorkbenchOverviewService(db, func() time.Time { return now })
	result, err := service.Overview(context.Background(), 7, "u1")
	require.NoError(t, err)

	require.Equal(t, []string{"live-run"}, runIDsOfOverview(result.InProgress))
	require.Empty(t, result.RecentlyCompleted, "an archived task's terminal runs stay out of recently completed")
	require.Empty(t, result.PendingInteractions, "an archived task's pending interactions stay out of needs-me")
}

func runIDsOfOverview(runs []workbenchservice.OverviewRunSummary) []string {
	ids := make([]string, 0, len(runs))
	for _, run := range runs {
		ids = append(ids, run.RunID)
	}
	return ids
}
```

- [ ] **Step 6: 运行确认失败**

Run: `go test ./internal/handler/session/ -run 'TestMX013' -count=1`
Expected: FAIL——编译错误 `undefined: workbenchservice.OverviewTaskRow`/`workbenchservice.OverviewNotificationRow`（测试包先于实现引用，编译即失败退出）。

- [ ] **Step 7: 最小实现**

`internal/modules/workbench/service/workbench/overview.go` 全量改为：

```go
package workbench

import (
	"context"
	"time"

	"gorm.io/gorm"
)

/**
 * 工作台聚合读模型（MX-013，B 类 /workbench/overview）。
 * 授权后固定少量聚合查询给出：计数、进行中执行、待处理交互、最近完成与 as_of
 * ——客户端不做逐会话 N+1。unread_notifications 自本计划补建的
 * workbench_notifications（迁移 000190/000111，列集对齐 InboxNotificationRow）
 * 真实计数；recent_artifacts 在 MX-024 接入前如实为空。
 * 三段任务视图均排除已归档任务（Task lifecycle archived，T04）。
 */

// OverviewRunRow 映射 agent_runs 物理列 + sessions.title 投影（单查询 JOIN；
// taskId = sessionId，ADR-0004）。
type OverviewRunRow struct {
	TenantID  uint64
	RunID     string
	SessionID string
	OwnerID   string
	Status    string
	Title     string
	UpdatedAt time.Time
}

func (OverviewRunRow) TableName() string { return "agent_runs" }

// OverviewInteractionRow 映射既有 workbench_interactions 表（只读投影列）。
type OverviewInteractionRow struct {
	TenantID         uint64
	ID               string
	RunID            string
	OwnerID          string
	Kind             string
	ArgsHash         string
	Status           string
	ExpectedRevision int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (OverviewInteractionRow) TableName() string { return "workbench_interactions" }

// OverviewTaskRow projects the task identity columns the overview joins on
// (sessions). Test fixtures AutoMigrate it; production uses the real table
// after the task-archive migration (versioned 000189 / sqlite 000110).
type OverviewTaskRow struct {
	TenantID   uint64
	ID         string
	Title      string
	ArchivedAt *time.Time
}

func (OverviewTaskRow) TableName() string { return "sessions" }

// OverviewNotificationRow projects workbench_notifications (created by
// migration 000190/000111; column-aligned with InboxNotificationRow) for the
// unread count. Test fixtures AutoMigrate it.
type OverviewNotificationRow struct {
	TenantID uint64
	ID       string
	OwnerID  string
	Read     bool
}

func (OverviewNotificationRow) TableName() string { return "workbench_notifications" }

type OverviewCounts struct {
	ActiveRuns          int64 `json:"active_runs"`
	PendingInteractions int64 `json:"pending_interactions"`
	UnreadNotifications int64 `json:"unread_notifications"`
}

type OverviewRunSummary struct {
	RunID            string    `json:"run_id"`
	SessionID        string    `json:"session_id"`
	Title            string    `json:"title,omitempty"`
	RunStatus        string    `json:"run_status"`
	ExecutionStatus  string    `json:"execution_status"`
	SettlementStatus string    `json:"settlement_status"`
	Attention        string    `json:"attention"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type OverviewInteractionSummary struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	CreatedAt string `json:"created_at"`
}

type OverviewArtifactSummary struct {
	ArtifactID string `json:"artifact_id"`
	Title      string `json:"title"`
	Kind       string `json:"kind"`
}

type Overview struct {
	Counts              OverviewCounts               `json:"counts"`
	InProgress          []OverviewRunSummary         `json:"in_progress"`
	PendingInteractions []OverviewInteractionSummary `json:"pending_interactions"`
	RecentlyCompleted   []OverviewRunSummary         `json:"recently_completed"`
	RecentArtifacts     []OverviewArtifactSummary    `json:"recent_artifacts"`
	AsOf                string                       `json:"as_of"`
}

const (
	overviewInProgressLimit       = 20
	overviewRecentCompletedLimit = 10
)

var (
	overviewActiveStatuses  = []string{"queued", "running", "waiting_user", "reconciling"}
	overviewTerminalStatuses = []string{"succeeded", "failed", "canceled"}
)

func overviewAttention(status string) string {
	if status == "waiting_user" {
		return "required"
	}
	return "none"
}

type OverviewService struct {
	db    *gorm.DB
	clock func() time.Time
}

func NewWorkbenchOverviewService(db *gorm.DB, clock func() time.Time) *OverviewService {
	if clock == nil {
		clock = time.Now
	}
	return &OverviewService{db: db, clock: clock}
}

// runSegmentQuery issues one joined query for a status set: tenant+owner
// predicate, archived tasks excluded, task title projected. No per-session
// follow-up reads ever happen (the N+1 rule).
func (s *OverviewService) runSegmentQuery(ctx context.Context, tenantID uint64, ownerID string, statuses []string, limit int) ([]OverviewRunRow, error) {
	var runs []OverviewRunRow
	err := s.db.WithContext(ctx).
		Select("agent_runs.tenant_id, agent_runs.run_id, agent_runs.session_id, agent_runs.owner_id, agent_runs.status, agent_runs.updated_at, sessions.title AS title").
		Joins("JOIN sessions ON sessions.tenant_id = agent_runs.tenant_id AND sessions.id = agent_runs.session_id").
		Where("agent_runs.tenant_id = ? AND agent_runs.owner_id = ? AND agent_runs.status IN ? AND sessions.archived_at IS NULL", tenantID, ownerID, statuses).
		Order("agent_runs.updated_at DESC").Limit(limit).Find(&runs).Error
	return runs, err
}

func runSummaryOf(run OverviewRunRow, settlement string) OverviewRunSummary {
	return OverviewRunSummary{
		RunID:            run.RunID,
		SessionID:        run.SessionID,
		Title:            run.Title,
		RunStatus:        run.Status,
		ExecutionStatus:  run.Status, // 活动态执行观察与 run 状态同源（快照先例）
		SettlementStatus: settlement,
		Attention:        overviewAttention(run.Status),
		UpdatedAt:        run.UpdatedAt,
	}
}

// Overview 以 tenant+owner 谓词聚合；上限限制防大租户拖垮（进行中 20 / 最近完成 10）。
func (s *OverviewService) Overview(ctx context.Context, tenantID uint64, ownerID string) (Overview, error) {
	result := Overview{
		InProgress:          []OverviewRunSummary{},
		PendingInteractions: []OverviewInteractionSummary{},
		RecentlyCompleted:   []OverviewRunSummary{},
		RecentArtifacts:     []OverviewArtifactSummary{},
		AsOf:                s.clock().UTC().Format(time.RFC3339),
	}
	if tenantID == 0 || ownerID == "" {
		return result, gorm.ErrRecordNotFound
	}
	runs, err := s.runSegmentQuery(ctx, tenantID, ownerID, overviewActiveStatuses, overviewInProgressLimit)
	if err != nil {
		return result, err
	}
	for _, run := range runs {
		result.InProgress = append(result.InProgress, runSummaryOf(run, "pending"))
	}
	// 计数为截断查询长度（上限 20）——首页计数语义按“进行中卡片数”展示（D-025）
	result.Counts.ActiveRuns = int64(len(runs))

	completed, err := s.runSegmentQuery(ctx, tenantID, ownerID, overviewTerminalStatuses, overviewRecentCompletedLimit)
	if err != nil {
		return result, err
	}
	for _, run := range completed {
		// 终态结算投影沿用 agent_run_snapshot.go 的先例：终态即已结算出证。
		result.RecentlyCompleted = append(result.RecentlyCompleted, runSummaryOf(run, "settled"))
	}

	var interactions []OverviewInteractionRow
	if err := s.db.WithContext(ctx).
		Table("workbench_interactions").
		Select("workbench_interactions.id, workbench_interactions.kind, workbench_interactions.created_at").
		Joins("LEFT JOIN agent_runs ar ON ar.tenant_id = workbench_interactions.tenant_id AND ar.run_id = workbench_interactions.run_id").
		Joins("LEFT JOIN sessions ON sessions.tenant_id = ar.tenant_id AND sessions.id = ar.session_id").
		Where("workbench_interactions.tenant_id = ? AND workbench_interactions.owner_id = ? AND workbench_interactions.status = ? AND sessions.archived_at IS NULL", tenantID, ownerID, "pending").
		Order("workbench_interactions.created_at ASC").Limit(overviewInProgressLimit).Find(&interactions).Error; err != nil {
		return result, err
	}
	for _, row := range interactions {
		result.PendingInteractions = append(result.PendingInteractions, OverviewInteractionSummary{
			ID:        row.ID,
			Kind:      row.Kind,
			CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	result.Counts.PendingInteractions = int64(len(interactions))

	var unread int64
	if err := s.db.WithContext(ctx).Model(&OverviewNotificationRow{}).
		Where("tenant_id = ? AND owner_id = ? AND read = ?", tenantID, ownerID, false).
		Count(&unread).Error; err != nil {
		return result, err
	}
	result.Counts.UnreadNotifications = unread
	return result, nil
}
```

- [ ] **Step 8: 运行确认通过（含 Task 1/2 回归与迁移对齐）**

Run: `go test ./internal/handler/session/ -run 'TestMX013' -count=1 && go test ./internal/application/repository/ -run 'TestWorkbench' -count=1`
Expected: PASS（`MX013-OBSERVATION` 仍输出 `perSessionHTTPRequests:0`；`TestWorkbench` 模式同时覆盖 `TestWorkbenchNotificationsTableExistsAfterMigrations`）。

- [ ] **Step 9: Commit**

```bash
git add migrations/versioned/000190_workbench_notifications.up.sql migrations/versioned/000190_workbench_notifications.down.sql migrations/sqlite/000111_workbench_notifications.up.sql migrations/sqlite/000111_workbench_notifications.down.sql internal/application/repository/workbench_notifications_migration_test.go internal/modules/workbench/service/workbench/overview.go internal/handler/session/workbench_overview_mx013_test.go
git commit -m "feat(workbench): notification read-model table, overview recently-completed segment and real unread counts (T04)"
```

---

### Task 4: contracts — 读模型契约扩展（recently_completed / attention）

**Files:**
- Modify: `packages/contracts/src/mobile/read-models.ts`
- Test: `packages/contracts/test/mobile-read-models.test.ts`（新文件；`pnpm test:shared` 的 glob `packages/contracts/test/mobile-*.test.ts` 自动覆盖）

**Interfaces:**
- Consumes: 既有 `ContractError`（`../index.ts`）、`executionSummary` 解析器（read-models.ts:105）。
- Produces: `export type AttentionState = 'none' | 'required'`；`ExecutionSummary.attention?: AttentionState`（缺省容忍、存在即校验枚举）；`WorkbenchOverview.recently_completed: ExecutionSummary[]`（必填数组，与服务端 Task 3 输出一致）；`parseWorkbenchOverview` 相应扩展。Task 7 的 `createTaskOfficeRemote` 消费。

- [ ] **Step 1: 写失败的契约测试**

`packages/contracts/test/mobile-read-models.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { ContractError } from '../src/index.ts';
import { parseWorkbenchOverview, type WorkbenchOverview } from '../src/mobile/read-models.ts';

// 代表性 wire 字节（与 internal/modules/workbench/service/workbench/overview.go
// 的 JSON 标签逐字段对齐；真实序列化形状的固定样张）。
const overviewWire = {
  counts: { active_runs: 1, pending_interactions: 1, unread_notifications: 2 },
  in_progress: [{
    run_id: 'r1', session_id: 't1', title: 'weekly report', run_status: 'waiting_user',
    execution_status: 'waiting_user', settlement_status: 'pending', attention: 'required',
    updated_at: '2026-09-23T00:00:00Z',
  }],
  pending_interactions: [{ id: 'i1', kind: 'tool_approval', created_at: '2026-09-23T00:00:00Z' }],
  recently_completed: [{
    run_id: 'r2', session_id: 't2', title: 'finished research', run_status: 'succeeded',
    execution_status: 'succeeded', settlement_status: 'settled', attention: 'none',
    updated_at: '2026-09-22T00:00:00Z',
  }],
  recent_artifacts: [],
  as_of: '2026-09-23T00:00:01Z',
};

test('parseWorkbenchOverview maps the three home segments including recently completed', () => {
  const overview: WorkbenchOverview = parseWorkbenchOverview(overviewWire);
  assert.equal(overview.in_progress.length, 1);
  assert.equal(overview.in_progress[0]!.title, 'weekly report');
  assert.equal(overview.in_progress[0]!.attention, 'required');
  assert.equal(overview.recently_completed.length, 1);
  assert.equal(overview.recently_completed[0]!.run_id, 'r2');
  assert.equal(overview.recently_completed[0]!.settlement_status, 'settled');
  assert.equal(overview.counts.unread_notifications, 2);
});

test('attention is optional but validated when present; title falls back to empty string', () => {
  const withoutAttention = parseWorkbenchOverview({
    ...overviewWire,
    in_progress: [{ ...overviewWire.in_progress[0]!, title: undefined, attention: undefined }],
  });
  assert.equal(withoutAttention.in_progress[0]!.title, '');
  assert.equal(withoutAttention.in_progress[0]!.attention, undefined);

  assert.throws(() => parseWorkbenchOverview({
    ...overviewWire,
    in_progress: [{ ...overviewWire.in_progress[0]!, attention: 'urgent' }],
  }), (error: unknown) => error instanceof ContractError);
});

test('recently_completed is a required array, matching the server contract', () => {
  const { recently_completed: _omitted, ...withoutSegment } = overviewWire;
  assert.throws(() => parseWorkbenchOverview(withoutSegment), /recently_completed/);
  const empty = parseWorkbenchOverview({ ...overviewWire, recently_completed: [] });
  assert.deepEqual(empty.recently_completed, []);
});

test('overview rejects malformed envelopes without inventing data', () => {
  assert.throws(() => parseWorkbenchOverview({ counts: { active_runs: -1 } }), /active_runs/);
  assert.throws(() => parseWorkbenchOverview([]), /expected an object/);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/contracts/test/mobile-read-models.test.ts`
Expected: FAIL——运行时 `overview.recently_completed` 为 undefined（`overview.recently_completed.length` 抛 TypeError）或 attention 断言不等（tsx 只转译不查类型；类型层错误由 `pnpm --filter @weknora/mobile typecheck` 在 Task 8 收口）。

- [ ] **Step 3: 最小实现**

`packages/contracts/src/mobile/read-models.ts`：

(a) `ExecutionSummary` 的 `updated_at` 之前加类型、`ExecutionSummary` 内加字段：

```ts
export type AttentionState = 'none' | 'required';

export interface ExecutionSummary {
  run_id: string;
  session_id: string;
  title: string;
  run_status: string;
  execution_status: string;
  settlement_status: string;
  /** 需要成员介入与否；旧读模型可缺省（展示层按 'none' 兜底）。 */
  attention?: AttentionState;
  updated_at: string;
}
```

(b) `count` 辅助函数之后追加：

```ts
function attentionState(value: unknown, path: string): AttentionState | undefined {
  if (value === undefined || value === null) return undefined;
  if (value === 'none' || value === 'required') return value;
  throw new ContractError(path, 'expected "none" or "required"');
}
```

(c) `executionSummary` 返回对象 `updated_at` 之前加：

```ts
    ...(attentionState(row.attention, `${path}.attention`) === undefined ? {} : { attention: attentionState(row.attention, `${path}.attention`) }),
```

(d) `WorkbenchOverview` 接口 `recent_artifacts` 之后加：

```ts
  recently_completed: ExecutionSummary[];
```

(e) `parseWorkbenchOverview` 中 `recent_artifacts` 映射之后、`as_of` 之前加：

```ts
    recently_completed: Array.isArray(row.recently_completed) ? row.recently_completed.map((item, i) => executionSummary(item, `recently_completed[${i}]`)) : (() => { throw new ContractError('recently_completed', 'expected an array'); })(),
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/contracts/test/mobile-read-models.test.ts packages/contracts/test/mobile-execution.test.ts packages/contracts/test/mobile-interactions.test.ts`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/mobile/read-models.ts packages/contracts/test/mobile-read-models.test.ts
git commit -m "feat(contracts): overview recently-completed segment and attention state on wire summaries (T04)"
```

---

### Task 5: mobile-core — `MobileRuntime.authorizedRequest` 授权读通道

**Files:**
- Modify: `packages/mobile-core/src/runtime/types.ts`
- Modify: `packages/mobile-core/src/runtime/ports.ts`
- Modify: `packages/mobile-core/src/runtime/mobile-runtime.ts`
- Test: `packages/mobile-core/src/runtime/mobile-runtime.test.ts`（追加测试）

**Interfaces:**
- Consumes: 既有 `createMobileRuntime`/`refreshedCredential`/`current`（mobile-runtime.ts:98-137）、`fakeStore`/`remote`/`ports` 测试助手（mobile-runtime.test.ts:26-56）、401 侦测依据 `ApiError { name: 'ApiError', status?: number }`（`packages/api-client/src/errors.ts:24`，结构侦测、不 import）。
- Produces: `RuntimeAuthorizedRequest { method: string; path: string; headers?: Record<string, string>; body?: unknown; signal?: AbortSignal }`（types.ts，与 api-client `ClientRequest` 结构可赋值）；`MobileRuntime.authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>`（未授权面/无 port 抛 `RUNTIME_UNAUTHORIZED`；scope 变化后抛 `RUNTIME_SCOPE_CHANGED`；401 经 Runtime 单飞刷新后重试一次）；`MobileRuntimePorts.authorizedTransport?: (deploymentOrigin: string) => AuthorizedTransport`，`AuthorizedTransport = (input: { method: string; path: string; headers?: Record<string, string>; body?: unknown; signal?: AbortSignal }, accessToken: string) => Promise<unknown>`（token 只下渗 transport、不上抛）。Task 8/9 消费（Task Office 远端取数通道；#33/#35+ 同样可复用）。

- [ ] **Step 1: 写失败的测试**

`packages/mobile-core/src/runtime/mobile-runtime.test.ts` 末尾追加（完整代码；`authorizedTransportCalls` 为局部助手）：

```ts
test('authorizedRequest rejects before any transport call when unauthorized', async () => {
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote()));
  await assert.rejects(runtime.authorizedRequest({ method: 'GET', path: '/api/v1/workbench/overview' }), /RUNTIME_UNAUTHORIZED/);
});

test('authorizedRequest carries the credential and refreshes exactly once on a 401', async () => {
  const sentTokens: Array<string | undefined> = [];
  const store = fakeStore();
  const runtime = createMobileRuntime({
    credentialStore: store,
    remoteFor: () => remote({
      me: async (token) => ({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }),
      refresh: async () => ({ access_token: 'access-2', refresh_token: 'refresh-2' }),
    }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedTransport: () => async (input, accessToken) => {
      sentTokens.push(`${input.method} ${input.path} ${accessToken}`);
      if (accessToken === 'access-1') {
        const error = new Error('HTTP 401');
        error.name = 'ApiError';
        (error as unknown as { status?: number }).status = 401;
        throw error;
      }
      return { success: true, data: { ok: true } };
    },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const response = await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/workbench/overview' });
  assert.deepEqual(response, { success: true, data: { ok: true } });
  assert.deepEqual(sentTokens, [
    'GET /api/v1/workbench/overview access-1',
    'GET /api/v1/workbench/overview access-2',
  ]);
});

test('a late authorized response after a scope change is dropped', async () => {
  const release = deferred<void>();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedTransport: () => async () => {
      await release.promise;
      return { success: true, data: { stale: true } };
    },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const pending = runtime.authorizedRequest({ method: 'GET', path: '/api/v1/workbench/overview' });
  await runtime.signOut();
  release.resolve();
  await assert.rejects(pending, /RUNTIME_SCOPE_CHANGED/);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts`
Expected: FAIL——运行时 `runtime.authorizedRequest is not a function`（TypeError；tsx 只转译不查类型，类型层由 Task 8 的 typecheck 收口）。

- [ ] **Step 3: 最小实现**

(a) `packages/mobile-core/src/runtime/types.ts`：`RuntimeSnapshot` 之前加：

```ts
/** Authorized channel input for child modules; structurally assignable from api-client ClientRequest. */
export interface RuntimeAuthorizedRequest {
  method: string;
  path: string;
  headers?: Record<string, string>;
  body?: unknown;
  signal?: AbortSignal;
}
```

`MobileRuntime` 接口 `scopeLease()` 之前加：

```ts
  /** Sends one request through the active deployment with the current credential (refresh-once on 401). Tokens never escape the Runtime. */
  authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>;
```

(b) `packages/mobile-core/src/runtime/ports.ts`：`PendingOidc` 之前加：

```ts
/** Receives the access token; never exposes it upward. */
export type AuthorizedTransport = (
  input: { method: string; path: string; headers?: Record<string, string>; body?: unknown; signal?: AbortSignal },
  accessToken: string,
) => Promise<unknown>;
```

`MobileRuntimePorts` 的 `randomBytes?` 之后加：

```ts
  /** Authorized channel for child modules (Task Office &c.); omitted = fail closed. */
  authorizedTransport?: (deploymentOrigin: string) => AuthorizedTransport;
```

(c) `packages/mobile-core/src/runtime/mobile-runtime.ts`：import 行补 `RuntimeAuthorizedRequest`；`serverOidcCallback` 函数之后加：

```ts
function unauthorizedStatus(error: unknown): boolean {
  return typeof error === 'object' && error !== null &&
    (error as { name?: unknown }).name === 'ApiError' &&
    (error as { status?: unknown }).status === 401;
}
```

返回对象中 `scopeLease: () => lease,` 之前加：

```ts
    async authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown> {
      const deployment = activeDeployment;
      const transport = deployment && state.surface === 'authorized' ? ports.authorizedTransport?.(deployment.origin) : undefined;
      if (!deployment || !transport) throw new Error('RUNTIME_UNAUTHORIZED');
      const requestEpoch = epoch;
      const send = async (token: string): Promise<unknown> => transport(input, token);
      const credential = await ports.credentialStore.read(deployment.origin);
      if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
      if (!credential) throw new Error('RUNTIME_UNAUTHORIZED');
      try {
        const response = await send(credential.token);
        if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
        return response;
      } catch (error) {
        if (!unauthorizedStatus(error)) throw error;
        const refreshed = await refreshedCredential(requestEpoch, deployment, credential);
        if (!refreshed) throw new Error('RUNTIME_UNAUTHORIZED');
        if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
        const retried = await send(refreshed.token);
        if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
        return retried;
      }
    },
```

- [ ] **Step 4: 运行确认通过（#32 回归一并验证）**

Run: `npx tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts`
Expected: 全部 PASS（含 #32 的 activateTenant/vault 用例与本任务 3 个新用例）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/runtime/types.ts packages/mobile-core/src/runtime/ports.ts packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts
git commit -m "feat(mobile-core): runtime authorized request channel with single-flight refresh and late-drop (T04)"
```

---

### Task 6: mobile-core — Task Office Module（home / tasks / archive）

**Files:**
- Create: `packages/mobile-core/src/task-office/task-office.ts`
- Create: `packages/mobile-core/src/task-office/in-memory-task-backend.ts`
- Test: `packages/mobile-core/src/task-office/task-office.test.ts`
- Modify: `packages/mobile-core/src/index.ts`

**Interfaces:**
- Consumes: #32 产出的包内 lease 内省 `leaseActive`（`packages/mobile-core/src/runtime/scope-lease.ts`）与 `ScopeLease`（`../runtime/types.ts`）；`RuntimeScopeLease`（测试内直接构造可撤销 lease）。
- Produces（公共导出，自 `@weknora/mobile-core`）：`createTaskOffice(ports: TaskOfficePorts): TaskOffice`；类型 `TaskOffice`/`TaskOfficePorts`/`TaskBackendPort`/`TaskBackendListInput`/`TaskBackendOverview`/`TaskBackendPage`/`TaskBackendRun`/`TaskOfficeQuery`/`TaskStatusFilter`/`TaskCard`/`InteractionCard`/`HomeView`/`TaskListPage`/`AttentionState`/`TaskOfficeError`/`TaskOfficeErrorCode`；`createScenarioTaskBackend(handlers)` + `emptyOverview(asOf?)`（in-memory scenario Adapter）。`MobileRuntime.authorizedRequest`（Task 5）为远端通道。Task 7（api-client 适配）、Task 8（屏）、Task 9（集成证据）及 #35/#38/#68 消费。

- [ ] **Step 1: 写失败的 Interface 测试**

`packages/mobile-core/src/task-office/task-office.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createScenarioTaskBackend, emptyOverview } from './in-memory-task-backend.ts';
import { createTaskOffice, TaskOfficeError, type TaskBackendOverview, type TaskBackendPage } from './task-office.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

function officeWith(leaseRef: { lease?: ScopeLease }, handlers: Parameters<typeof createScenarioTaskBackend>[0] = {}) {
  const backend = createScenarioTaskBackend(handlers);
  return { backend, office: createTaskOffice({ backend, lease: () => leaseRef.lease }) };
}

function backendRun(runId: string, taskId = `task-${runId}`) {
  return { runId, taskId, title: `title-${runId}`, runStatus: 'running', attention: 'none' as const, updatedAt: '2026-09-23T00:00:00Z' };
}

test('home aggregates needs-me, running and recently-completed from one backend call', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const overview: TaskBackendOverview = {
    needsMe: [{ interactionId: 'i1', kind: 'tool_approval', createdAt: '2026-09-23T00:00:00Z' }],
    running: [backendRun('r1')],
    recentlyCompleted: [{ ...backendRun('r2'), runStatus: 'succeeded' }],
    unreadNotifications: 3,
    asOf: '2026-09-23T00:00:01Z',
  };
  const { backend, office } = officeWith(leaseRef, { overview: async () => overview });
  const view = await office.home();
  assert.deepEqual(view.running, [backendRun('r1')]);
  assert.deepEqual(view.recentlyCompleted, [{ ...backendRun('r2'), runStatus: 'succeeded' }]);
  assert.deepEqual(view.needsMe, [{ interactionId: 'i1', kind: 'tool_approval', createdAt: '2026-09-23T00:00:00Z' }]);
  assert.equal(view.unreadNotifications, 3);
  assert.equal(backend.calls.length, 1, 'one aggregate call, no per-session follow-ups');
});

test('a late home response after the scope lease was revoked never resolves with foreign-scope data', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const gate = deferred<TaskBackendOverview>();
  const { office } = officeWith(leaseRef, { overview: () => gate.promise });
  const pending = office.home();
  revocable.revoke();
  gate.resolve({ ...emptyOverview(), running: [backendRun('foreign')] });
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});

test('a newer tasks query supersedes an in-flight older one', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const first = deferred<TaskBackendPage>();
  const second = deferred<TaskBackendPage>();
  let call = 0;
  const { office } = officeWith(leaseRef, { list: () => (call += 1) === 1 ? first.promise : second.promise });
  const older = office.tasks({ search: 'old' });
  const newer = office.tasks({ search: 'new' });
  first.resolve({ items: [backendRun('r-old')] });
  second.resolve({ items: [backendRun('r-new')] });
  await assert.rejects(older, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED');
  const page = await newer;
  assert.deepEqual(page.items.map((item) => item.runId), ['r-new']);
});

test('moreTasks continues the owned cursor and duplicate run keys are reported, never rendered twice', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const pages: Array<TaskBackendPage | Error> = [
    { items: [backendRun('r-a'), backendRun('r-b')], nextCursor: 'cursor-1' },
    { items: [backendRun('r-b'), backendRun('r-c')] }, // 服务端重复键：r-b 重现
  ];
  let call = 0;
  const { backend, office } = officeWith(leaseRef, { list: () => Promise.resolve(pages[call++]!) });
  const first = await office.tasks({});
  assert.deepEqual(first.items.map((item) => item.runId), ['r-a', 'r-b']);
  assert.equal(first.nextCursor, 'cursor-1');
  const second = await office.moreTasks();
  assert.deepEqual(second.items.map((item) => item.runId), ['r-c'], 'the repeated key is not rendered again');
  assert.deepEqual(second.duplicateRunIds, ['r-b'], 'the duplicate is observable');
  // 已耗尽：不再发后端请求，返回空页且保留重复键观测。
  const exhausted = await office.moreTasks();
  assert.deepEqual(exhausted, { items: [], duplicateRunIds: ['r-b'] });
  assert.equal(backend.calls.filter((entry) => entry.kind === 'list').length, 2);
});

test('empty and failing backends surface as an empty view and a typed backend error', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWith(leaseRef);
  assert.deepEqual(await office.home(), emptyOverview());
  assert.deepEqual((await office.tasks({})).items, []);

  const failing = officeWith(leaseRef, { list: () => Promise.reject(new Error('HTTP 500')) });
  await assert.rejects(failing.office.tasks({}), (error: unknown) =>
    error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_BACKEND' && (error.cause as Error).message === 'HTTP 500');
});

test('archive and restore invalidate the accumulated query and fail closed without a lease', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const archived: string[] = [];
  const restored: string[] = [];
  const { office } = officeWith(leaseRef, {
    list: async () => ({ items: [backendRun('r-a')], nextCursor: 'cursor-1' }),
    archive: async (taskId) => { archived.push(taskId); },
    restore: async (taskId) => { restored.push(taskId); },
  });
  await office.tasks({});
  await office.archive('task-r-a ');
  assert.deepEqual(archived, ['task-r-a'], 'task ids are trimmed before hitting the backend');
  await assert.rejects(office.moreTasks(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_NO_ACTIVE_QUERY');
  await office.tasks({});
  await office.restore('task-r-a');
  assert.deepEqual(restored, ['task-r-a']);
  await assert.rejects(office.moreTasks(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_NO_ACTIVE_QUERY');

  revocable.revoke();
  await assert.rejects(office.archive('task-r-a'), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  await assert.rejects(office.home(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});

test('queries are normalized before they reach the backend', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { backend, office } = officeWith(leaseRef, { list: async () => ({ items: [] }) });
  await office.tasks({ search: '   quarterly   review ', limit: 5000, status: 'running' });
  const listCall = backend.calls.find((entry) => entry.kind === 'list');
  assert.ok(listCall && listCall.kind === 'list');
  assert.deepEqual({ search: listCall.input.search, limit: listCall.input.limit, status: listCall.input.status, archived: listCall.input.archived },
    { search: 'quarterly review', limit: 100, status: 'running', archived: undefined });
  await office.tasks({ search: '   ' });
  const blank = backend.calls.filter((entry) => entry.kind === 'list')[1]!;
  assert.ok(blank.kind === 'list');
  assert.equal(blank.input.search, undefined, 'blank search is normalized away');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/task-office/task-office.test.ts`
Expected: FAIL——模块文件不存在（Cannot find module './task-office.ts' / './in-memory-task-backend.ts'）。

- [ ] **Step 3: 最小实现**

(a) `packages/mobile-core/src/task-office/task-office.ts`（新文件，完整内容）：

```ts
import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

/**
 * Task Office 深模块（module-seams §5）——T04 交付读侧与归档生命周期：
 * - home()：一次聚合读返回三段视图（需要我处理 / 正在运行 / 最近完成）；
 * - tasks()/moreTasks()：模块拥有 cursor 与查询身份，屏不维护分页状态；
 * - archive()/restore()：归档写路径，成功后累积查询失效；
 * - 迟到拒绝：scope lease 撤销（切租户/换部署/退出）后的一切结果按
 *   TASK_OFFICE_SCOPE_CHANGED 拒绝；更新的查询使旧查询按 SUPERSEDED 拒绝；
 * - 重复键：跨页重复的 runId 不再渲染，经 duplicateRunIds 观测。
 * start(goal)/open(taskID) 属 #36/#35，不在本模块当前 Interface。
 */

export type AttentionState = 'none' | 'required';
export type TaskStatusFilter = 'running' | 'waiting_user' | 'succeeded' | 'failed' | 'canceled';

export interface TaskCard {
  taskId: string;
  runId: string;
  title: string;
  runStatus: string;
  attention: AttentionState;
  updatedAt: string;
}

export interface InteractionCard {
  interactionId: string;
  kind: string;
  createdAt: string;
}

export interface HomeView {
  needsMe: InteractionCard[];
  running: TaskCard[];
  recentlyCompleted: TaskCard[];
  unreadNotifications: number;
  asOf: string;
}

export interface TaskOfficeQuery {
  status?: TaskStatusFilter;
  agentId?: string;
  search?: string;
  archived?: boolean;
  limit?: number;
}

export interface TaskListPage {
  items: TaskCard[];
  nextCursor?: string;
  duplicateRunIds: string[];
}

export interface TaskBackendRun {
  runId: string;
  taskId: string;
  title: string;
  runStatus: string;
  attention: AttentionState;
  updatedAt: string;
}

export interface TaskBackendOverview {
  needsMe: InteractionCard[];
  running: TaskBackendRun[];
  recentlyCompleted: TaskBackendRun[];
  unreadNotifications: number;
  asOf: string;
}

export interface TaskBackendPage {
  items: TaskBackendRun[];
  nextCursor?: string;
}

export interface TaskBackendListInput {
  status?: string;
  agentId?: string;
  search?: string;
  archived?: boolean;
  cursor?: string;
  limit?: number;
}

export interface TaskBackendPort {
  overview(): Promise<TaskBackendOverview>;
  list(input: TaskBackendListInput): Promise<TaskBackendPage>;
  archive(taskId: string): Promise<void>;
  restore(taskId: string): Promise<void>;
}

export interface TaskOfficePorts {
  backend: TaskBackendPort;
  lease(): ScopeLease | undefined;
}

export type TaskOfficeErrorCode =
  | 'TASK_OFFICE_SCOPE_CHANGED'
  | 'TASK_OFFICE_SUPERSEDED'
  | 'TASK_OFFICE_NO_ACTIVE_QUERY'
  | 'TASK_OFFICE_INVALID_INPUT'
  | 'TASK_OFFICE_BACKEND';

export class TaskOfficeError extends Error {
  constructor(readonly code: TaskOfficeErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'TaskOfficeError';
  }
}

export interface TaskOffice {
  home(): Promise<HomeView>;
  tasks(query: TaskOfficeQuery): Promise<TaskListPage>;
  moreTasks(): Promise<TaskListPage>;
  archive(taskId: string): Promise<void>;
  restore(taskId: string): Promise<void>;
}

const searchMaxLen = 200;
const statusFilters: ReadonlySet<TaskStatusFilter> = new Set(['running', 'waiting_user', 'succeeded', 'failed', 'canceled']);

function normalizeQuery(query: TaskOfficeQuery): TaskBackendListInput {
  const search = typeof query.search === 'string' ? query.search.trim().slice(0, searchMaxLen) : '';
  const agentId = typeof query.agentId === 'string' ? query.agentId.trim() : '';
  const status = query.status !== undefined && statusFilters.has(query.status) ? query.status : undefined;
  const limit = typeof query.limit === 'number' && Number.isSafeInteger(query.limit) && query.limit > 0 ? Math.min(query.limit, 100) : 20;
  return {
    ...(status === undefined ? {} : { status }),
    ...(agentId === '' ? {} : { agentId }),
    ...(search === '' ? {} : { search }),
    ...(query.archived === true ? { archived: true } : {}),
    limit,
  };
}

interface listAccumulation {
  input: TaskBackendListInput;
  cursor?: string;
  seen: Set<string>;
  duplicates: string[];
}

export function createTaskOffice(ports: TaskOfficePorts): TaskOffice {
  let homeEpoch = 0;
  let listEpoch = 0;
  let accumulated: listAccumulation | undefined;

  const requireLease = (): ScopeLease => {
    const lease = ports.lease();
    if (!lease || !leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    return lease;
  };
  const callBackend = async <T>(action: () => Promise<T>): Promise<T> => {
    try {
      return await action();
    } catch (error) {
      if (error instanceof TaskOfficeError) throw error;
      throw new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
    }
  };
  const settle = <T>(epoch: number, kind: 'home' | 'list', lease: ScopeLease, value: T): T => {
    const currentEpoch = kind === 'home' ? homeEpoch : listEpoch;
    if (epoch !== currentEpoch) throw new TaskOfficeError('TASK_OFFICE_SUPERSEDED');
    if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    return value;
  };
  const toCard = (run: TaskBackendRun): TaskCard => ({ ...run });
  const accumulate = (state: listAccumulation, page: TaskBackendPage): TaskListPage => {
    const items: TaskCard[] = [];
    for (const run of page.items) {
      if (state.seen.has(run.runId)) {
        state.duplicates.push(run.runId);
        continue;
      }
      state.seen.add(run.runId);
      items.push(toCard(run));
    }
    state.cursor = page.nextCursor;
    return { items, ...(page.nextCursor === undefined ? {} : { nextCursor: page.nextCursor }), duplicateRunIds: [...state.duplicates] };
  };
  const mutate = async (taskId: string, action: (id: string) => Promise<void>): Promise<void> => {
    const lease = requireLease();
    const trimmed = taskId.trim();
    if (trimmed === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
    await callBackend(() => action(trimmed));
    if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    accumulated = undefined;
  };

  return {
    async home(): Promise<HomeView> {
      const epoch = ++homeEpoch;
      const lease = requireLease();
      const overview = settle(epoch, 'home', lease, await callBackend(() => ports.backend.overview()));
      return {
        needsMe: overview.needsMe,
        running: overview.running.map(toCard),
        recentlyCompleted: overview.recentlyCompleted.map(toCard),
        unreadNotifications: overview.unreadNotifications,
        asOf: overview.asOf,
      };
    },
    async tasks(query: TaskOfficeQuery): Promise<TaskListPage> {
      const epoch = ++listEpoch;
      const lease = requireLease();
      const input = normalizeQuery(query);
      const page = settle(epoch, 'list', lease, await callBackend(() => ports.backend.list(input)));
      const state: listAccumulation = { input, seen: new Set<string>(), duplicates: [] };
      accumulated = state;
      return accumulate(state, page);
    },
    async moreTasks(): Promise<TaskListPage> {
      const state = accumulated;
      if (state === undefined) throw new TaskOfficeError('TASK_OFFICE_NO_ACTIVE_QUERY');
      if (state.cursor === undefined) return { items: [], duplicateRunIds: [...state.duplicates] };
      const epoch = ++listEpoch;
      const lease = requireLease();
      const page = settle(epoch, 'list', lease, await callBackend(() => ports.backend.list({ ...state.input, cursor: state.cursor })));
      return accumulate(state, page);
    },
    archive(taskId: string): Promise<void> {
      return mutate(taskId, (id) => ports.backend.archive(id));
    },
    restore(taskId: string): Promise<void> {
      return mutate(taskId, (id) => ports.backend.restore(id));
    },
  };
}
```

(b) `packages/mobile-core/src/task-office/in-memory-task-backend.ts`（新文件，完整内容）：

```ts
import type { TaskBackendListInput, TaskBackendOverview, TaskBackendPage, TaskBackendPort } from './task-office.ts';

/** Scriptable scenario Adapter（module-seams §12：remote-owned 依赖的 in-memory 场景）。 */
export interface ScenarioTaskBackendHandlers {
  overview?: () => Promise<TaskBackendOverview>;
  list?: (input: TaskBackendListInput) => Promise<TaskBackendPage>;
  archive?: (taskId: string) => Promise<void>;
  restore?: (taskId: string) => Promise<void>;
}

export interface ScenarioTaskBackend extends TaskBackendPort {
  calls: Array<{ kind: 'overview' } | { kind: 'list'; input: TaskBackendListInput } | { kind: 'archive' | 'restore'; taskId: string }>;
}

export function emptyOverview(asOf = '2026-09-23T00:00:00Z'): TaskBackendOverview {
  return { needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf };
}

export function createScenarioTaskBackend(handlers: ScenarioTaskBackendHandlers = {}): ScenarioTaskBackend {
  const calls: ScenarioTaskBackend['calls'] = [];
  const scenario: ScenarioTaskBackend = {
    calls,
    async overview() {
      calls.push({ kind: 'overview' });
      return handlers.overview ? handlers.overview() : emptyOverview();
    },
    async list(input) {
      calls.push({ kind: 'list', input });
      return handlers.list ? handlers.list(input) : { items: [] };
    },
    async archive(taskId) {
      calls.push({ kind: 'archive', taskId });
      await handlers.archive?.(taskId);
    },
    async restore(taskId) {
      calls.push({ kind: 'restore', taskId });
      await handlers.restore?.(taskId);
    },
  };
  return scenario;
}
```

(c) `packages/mobile-core/src/index.ts` 末尾追加（在既有 4 行 export 之后）：

```ts
export { createTaskOffice, TaskOfficeError } from './task-office/task-office.ts';
export { createScenarioTaskBackend, emptyOverview } from './task-office/in-memory-task-backend.ts';
export type {
  AttentionState, HomeView, InteractionCard, TaskBackendListInput, TaskBackendOverview, TaskBackendPage,
  TaskBackendPort, TaskBackendRun, TaskCard, TaskListPage, TaskOffice, TaskOfficeErrorCode, TaskOfficePorts,
  TaskOfficeQuery, TaskStatusFilter,
} from './task-office/task-office.ts';
export type { ScenarioTaskBackend, ScenarioTaskBackendHandlers } from './task-office/in-memory-task-backend.ts';
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/mobile-core/src/task-office/task-office.test.ts`
Expected: 全部 PASS（7 个用例）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-office.ts packages/mobile-core/src/task-office/in-memory-task-backend.ts packages/mobile-core/src/task-office/task-office.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): task office module with home/list projections, archive lifecycle and late-response guards (T04)"
```

---

### Task 7: api-client — 列表参数扩展与 `createTaskOfficeRemote` 适配

**Files:**
- Modify: `packages/api-client/src/mobile/executions.ts`
- Modify: `packages/api-client/src/mobile/executions.test.ts`（追加参数断言）
- Create: `packages/api-client/src/mobile/task-office.ts`
- Test: `packages/api-client/src/mobile/task-office.test.ts`
- Modify: `packages/api-client/package.json`（exports 登记 `./mobile/task-office` 子路径——不登记则 Task 8/9 的 `@weknora/api-client/mobile/task-office` 导入报 `ERR_PACKAGE_PATH_NOT_EXPORTED`，见差异记录 13）

**Interfaces:**
- Consumes: Task 4 的 `parseWorkbenchOverview`（经既有 `createOverviewApi`）；既有 `createExecutionsApi().list`（executions.ts:252）；Task 1/2/3 的 wire 契约（`q`/`archived` 参数与 `title`/`attention`/`archived_at`/`recently_completed` 字段）。
- Produces: `ExecutionListParams` 新字段 `search?: string; archived?: boolean`（wire `q`/`archived`）；`WorkbenchExecutionItem` 新字段 `title?: string; attention?: 'none' | 'required'; archived_at?: string`；`createTaskOfficeRemote(options: { origin: string; request: (input: ClientRequest) => Promise<unknown> })`，返回对象与 mobile-core `TaskBackendPort`（Task 6）结构逐字一致（`overview/list/archive/restore`，DTO 字段 `runId/taskId/title/runStatus/attention/updatedAt`、`needsMe/running/recentlyCompleted/unreadNotifications/asOf`、`items/nextCursor`）；包 exports 新子路径 `"./mobile/task-office": "./src/mobile/task-office.ts"`。Task 8 组合根与 Task 9 集成证据消费；`pnpm --filter @weknora/mobile typecheck` 即结构可赋值与子路径可达的双重证明。

- [ ] **Step 1: 写失败的适配测试**

(a) `packages/api-client/src/mobile/executions.test.ts` 末尾追加：

```ts
test('list forwards search and archived facets on the wire', async () => {
  const requests: ClientRequest[] = [];
  const api = createExecutionsApi(async (input) => {
    requests.push(input);
    return { success: true, data: { items: [] } };
  });
  await api.list({ search: 'quarterly review', archived: true, status: 'succeeded' });
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions?status=succeeded&q=quarterly+review&archived=true');
  // 无筛选时不携带 q/archived 参数（与上面的参数序列化断言互补）。
  await api.list({});
  assert.equal(requests[1]!.path, '/api/v1/workbench/executions');
});
```

(b) `packages/api-client/src/mobile/task-office.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import type { ClientRequest } from '../client.ts';
import { createTaskOfficeRemote } from './task-office.ts';

const overviewData = {
  counts: { active_runs: 1, pending_interactions: 2, unread_notifications: 3 },
  in_progress: [{
    run_id: 'r1', session_id: 't1', title: 'weekly report', run_status: 'waiting_user',
    execution_status: 'waiting_user', settlement_status: 'pending', attention: 'required', updated_at: '2026-09-23T00:00:00Z',
  }],
  pending_interactions: [
    { id: 'i1', kind: 'tool_approval', created_at: '2026-09-23T00:00:00Z' },
    { id: 'i2', kind: 'budget', created_at: '2026-09-23T00:00:01Z' },
  ],
  recently_completed: [{
    run_id: 'r2', session_id: 't2', title: 'finished research', run_status: 'succeeded',
    execution_status: 'succeeded', settlement_status: 'settled', attention: 'none', updated_at: '2026-09-22T00:00:00Z',
  }],
  recent_artifacts: [],
  as_of: '2026-09-23T00:00:02Z',
};

const listData = {
  items: [{
    run_id: 'r1', session_id: 't1', title: 'weekly report', status: 'running',
    attention: 'required', archived_at: '2026-09-23T01:00:00Z', created_at: '2026-09-22T00:00:00Z', updated_at: '2026-09-23T00:00:00Z',
  }],
  next_cursor: 'cursor-2',
};

test('task office remote maps the overview wire into the module DTO', async () => {
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: overviewData }),
  });
  const overview = await remote.overview();
  assert.deepEqual(overview, {
    needsMe: [
      { interactionId: 'i1', kind: 'tool_approval', createdAt: '2026-09-23T00:00:00Z' },
      { interactionId: 'i2', kind: 'budget', createdAt: '2026-09-23T00:00:01Z' },
    ],
    running: [{ runId: 'r1', taskId: 't1', title: 'weekly report', runStatus: 'waiting_user', attention: 'required', updatedAt: '2026-09-23T00:00:00Z' }],
    recentlyCompleted: [{ runId: 'r2', taskId: 't2', title: 'finished research', runStatus: 'succeeded', attention: 'none', updatedAt: '2026-09-22T00:00:00Z' }],
    unreadNotifications: 3,
    asOf: '2026-09-23T00:00:02Z',
  });
});

test('task office remote forwards list facets and maps rows', async () => {
  const requests: ClientRequest[] = [];
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return input.path.includes('/workbench/executions?') ? { success: true, data: listData } : { success: true, data: {} };
    },
  });
  const page = await remote.list({ search: 'quarterly', archived: true, status: 'succeeded', limit: 50 });
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions?status=succeeded&q=quarterly&archived=true&limit=50');
  assert.deepEqual(page, {
    items: [{ runId: 'r1', taskId: 't1', title: 'weekly report', runStatus: 'running', attention: 'required', updatedAt: '2026-09-23T00:00:00Z' }],
    nextCursor: 'cursor-2',
  });
});

test('archive and restore hit the task lifecycle routes with encoded ids', async () => {
  const requests: ClientRequest[] = [];
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return { success: true, data: { task_id: 'ignored', archived: true } };
    },
  });
  await remote.archive('task/9');
  await remote.restore('task/9');
  assert.deepEqual(requests.map((input) => `${input.method} ${input.path}`), [
    'POST /api/v1/workbench/tasks/task%2F9/archive',
    'DELETE /api/v1/workbench/tasks/task%2F9/archive',
  ]);
});

test('non-success envelopes and malformed origins fail loudly', async () => {
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: false, error: 'nope' }),
  });
  await assert.rejects(remote.overview(), /success/);
  await assert.rejects(remote.archive('t1'), /success/);
  assert.throws(() => createTaskOfficeRemote({ origin: 'http://insecure.example.test', request: async () => ({}) }), /HTTPS/);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/executions.test.ts`
Expected: FAIL——`Cannot find module './task-office.ts'`；executions 新用例 path 断言不等（无 `q`/`archived` 参数）。

- [ ] **Step 3: 最小实现**

(a) `packages/api-client/src/mobile/executions.ts`：

`WorkbenchExecutionItem` 的 `wait_reason?` 之后追加：

```ts
  title?: string;
  attention?: 'none' | 'required';
  archived_at?: string;
```

`ExecutionListParams` 的 `limit?` 之前追加：

```ts
  search?: string;
  archived?: boolean;
```

`buildListQuery` 中 `agent_id` 行之后追加：

```ts
  if (params.search && params.search.trim() !== '') query.set('q', params.search.trim());
  if (params.archived === true) query.set('archived', 'true');
```

`parseExecutionItem` 返回对象追加（`wait_reason` 展开之后）：

```ts
    ...(optionalText(row.title, 'title') === undefined ? {} : { title: row.title as string }),
    ...(row.attention === undefined || row.attention === null ? {} : (row.attention === 'none' || row.attention === 'required' ? { attention: row.attention } : (() => { throw new ContractError('attention', 'must be "none" or "required" when present'); })())),
    ...(optionalText(row.archived_at, 'archived_at') === undefined ? {} : { archived_at: row.archived_at as string }),
```

(b) `packages/api-client/src/mobile/task-office.ts`（新文件，完整内容）：

```ts
import { createExecutionsApi } from './executions.ts';
import { createOverviewApi } from './overview.ts';
import type { ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface TaskOfficeRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
}

/** 与 mobile-core TaskBackendRun 逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface RemoteTaskRun { runId: string; taskId: string; title: string; runStatus: string; attention: 'none' | 'required'; updatedAt: string }
export interface RemoteTaskOverview {
  needsMe: Array<{ interactionId: string; kind: string; createdAt: string }>;
  running: RemoteTaskRun[];
  recentlyCompleted: RemoteTaskRun[];
  unreadNotifications: number;
  asOf: string;
}
export interface RemoteTaskPage { items: RemoteTaskRun[]; nextCursor?: string }

function requireDeploymentOrigin(origin: string): string {
  let parsed: URL;
  if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
  try { parsed = new URL(origin); } catch { throw new Error(`deployment origin must be an absolute URL: ${origin}`); }
  if (parsed.protocol !== 'https:') throw new Error('deployment origin must use HTTPS');
  if (parsed.username !== '' || parsed.password !== '') throw new Error('deployment origin must not embed user info');
  if (parsed.pathname !== '/') throw new Error('deployment origin must not include a path');
  if (parsed.search !== '' || parsed.hash !== '') throw new Error('deployment origin must not include a query or fragment');
  return parsed.origin;
}

function unwrap(value: unknown): void {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('task office response must be a success envelope');
  const envelope = value as { success?: unknown; data?: unknown };
  if (envelope.success !== true || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    throw new Error('task office response.success must be true with data');
  }
}

export function createTaskOfficeRemote(options: TaskOfficeRemoteOptions) {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  const overviewApi = createOverviewApi(request);
  const executionsApi = createExecutionsApi(request);

  const runFromSummary = (summary: { run_id: string; session_id: string; title?: string; run_status: string; attention?: 'none' | 'required'; updated_at: string }): RemoteTaskRun => ({
    runId: summary.run_id,
    taskId: summary.session_id,
    title: summary.title ?? '',
    runStatus: summary.run_status,
    attention: summary.attention ?? 'none',
    updatedAt: summary.updated_at,
  });
  const runFromItem = (item: { run_id: string; session_id: string; title?: string; status: string; attention?: 'none' | 'required'; updated_at: string }): RemoteTaskRun => ({
    runId: item.run_id,
    taskId: item.session_id,
    title: item.title ?? '',
    runStatus: item.status,
    attention: item.attention ?? 'none',
    updatedAt: item.updated_at,
  });

  return {
    async overview(): Promise<RemoteTaskOverview> {
      const value = await overviewApi.overview();
      return {
        needsMe: value.pending_interactions.map((item) => ({ interactionId: item.id, kind: item.kind, createdAt: item.created_at })),
        running: value.in_progress.map(runFromSummary),
        recentlyCompleted: value.recently_completed.map(runFromSummary),
        unreadNotifications: value.counts.unread_notifications,
        asOf: value.as_of,
      };
    },
    async list(input: { status?: string; agentId?: string; search?: string; archived?: boolean; cursor?: string; limit?: number }): Promise<RemoteTaskPage> {
      const page = await executionsApi.list({
        ...(input.status === undefined ? {} : { status: input.status }),
        ...(input.agentId === undefined ? {} : { agent_id: input.agentId }),
        ...(input.search === undefined ? {} : { search: input.search }),
        ...(input.archived === undefined ? {} : { archived: input.archived }),
        ...(input.cursor === undefined ? {} : { cursor: input.cursor }),
        ...(input.limit === undefined ? {} : { limit: input.limit }),
      });
      return {
        items: page.items.map(runFromItem),
        ...(page.next_cursor === undefined ? {} : { nextCursor: page.next_cursor }),
      };
    },
    async archive(taskId: string): Promise<void> {
      unwrap(await request({ method: 'POST', path: `/api/v1/workbench/tasks/${encodeURIComponent(taskId)}/archive` }));
    },
    async restore(taskId: string): Promise<void> {
      unwrap(await request({ method: 'DELETE', path: `/api/v1/workbench/tasks/${encodeURIComponent(taskId)}/archive` }));
    },
  };
}
```

(c) `packages/api-client/package.json`：`exports` 对象内 `"./mobile/runtime"` 一行之后追加（Task 8 组合根与 Task 9 集成测试经此子路径导入；不登记则 Node 报 `ERR_PACKAGE_PATH_NOT_EXPORTED`）：

```json
    "./mobile/task-office": "./src/mobile/task-office.ts",
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/executions.test.ts packages/api-client/src/mobile/interactions.test.ts packages/api-client/src/mobile/runtime.test.ts && node -p "require('./packages/api-client/package.json').exports['./mobile/task-office']"`
Expected: 测试 PASS（含既有 mobile 套件回归；`overview.ts` 无既有测试文件——作者已核实该目录仅含 executions/interactions/runtime 三组测试）；node -p 输出 `./src/mobile/task-office.ts`。子路径的真实可达性（exports 解析）由 Task 8 的 `pnpm --filter @weknora/mobile test && typecheck` 端到端证明（tsx 按相对路径导入不经过 exports）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/executions.ts packages/api-client/src/mobile/executions.test.ts packages/api-client/src/mobile/task-office.ts packages/api-client/src/mobile/task-office.test.ts packages/api-client/package.json
git commit -m "feat(api-client): list search/archive facets and task office remote adapter (T04)"
```

---

### Task 8: apps/mobile — Home 屏、Tasks 屏与组合根接线

**Files:**
- Create: `apps/mobile/src/screens/HomeScreen.tsx`
- Create: `apps/mobile/src/screens/TasksScreen.tsx`
- Create: `apps/mobile/src/app/tasks.tsx`
- Modify: `apps/mobile/src/composition.ts`
- Test: `apps/mobile/src/app-smoke.test.tsx`（更新既有断言 + 追加屏测试）

**Interfaces:**
- Consumes: Task 5 的 `MobileRuntime.authorizedRequest`；Task 6 的 `createTaskOffice`/`TaskOffice`/`HomeView`/`TaskListPage`/`TaskOfficeError`（`@weknora/mobile-core`）；Task 7 的 `createTaskOfficeRemote`；#32 的 `RuntimeSnapshot.identity.tenants` 与 `RuntimeSurfaceProps.onActivateTenant`（AuthorizedLandingScreen 上的租户切换语义迁移至 HomeScreen 头部，保留同形 props）；既有 `createWeKnoraClient`/`createJsonTransport`/`nativeFetch`（composition.ts:20-45）。
- Produces: `HomeScreen`（三段视图 + unread 徽标 + 租户切换 + 「查看全部任务」入口）；`TasksScreen`（搜索/状态筛选/归档开关/归档与恢复操作/翻页/重复键提示/空错加载态）；composition 的 `taskOfficeFor(runtime, origin)` 工厂与 `MobileTasks` 应用根；expo-router 路由 `/tasks`。Task 9 复用组合根接线方式。

（屏测试经 `app-smoke.test.tsx` 既有 react 桩渲染——该桩的 `useEffect` 为 no-op，因此初始装载取数由可见的刷新/重试按钮驱动断言；真实装载路径由 Task 9 真实运行覆盖。）

- [ ] **Step 1: 写失败的屏测试**

`apps/mobile/src/app-smoke.test.tsx`：

(a) stub 更新——`NATIVE_MODULE_STUBS` 的 `expo-router` 行改为（补 `push`），并新增两个惰性 `require` 目标的 stub（authorized 分支现在会调用 `runtime()`，其 adapter 工厂内部 `require('expo-secure-store')`/`require('expo-web-browser')` 必须可解析；resolve hook 对 import 与转译后的 require 都生效——真实文件）：

```ts
  'expo-router': "module.exports = { Stack: function Stack() { return null; }, router: { replace() {}, push() {} } }",
  'expo-secure-store': "module.exports = { getItemAsync: async () => null, setItemAsync: async () => {}, deleteItemAsync: async () => {} }",
  'expo-web-browser': "module.exports = { openAuthSessionAsync: async () => ({ type: 'dismiss' }) }",
```

(b) 既有测试 `surface routing keeps upgrade-required free of authorized controls and guards incomplete identity`（app-smoke.test.tsx:130）中，authorized 分支断言改为（HomeScreen 的三段文案在数据装载后才渲染——surface 测试只断言组件选择与静态入口；三段渲染断言在 (c) 的 HomeScreen 专项测试里）：

```ts
  const authorized = RuntimeSurface({
    snapshot: { surface: 'authorized', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1', activeTenantId: 'tenant-1', tenants: [{ id: 'tenant-1', name: 'Acme' }] } },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
  });
  assert.equal(authorized.type.name, 'HomeScreen');
  const homeText = descendants(render(authorized.type, authorized.props)).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(homeText.includes('View all tasks'), true);
  assert.equal(homeText.includes('Acme'), true);
```

（`upgrade-required` 与 `identity-guard` 断言保持原样；若 #32 已调整该测试，仅替换 authorized 分支。）

(c) 文件末尾追加屏测试（完整代码）：

```tsx
function fakeTaskOffice(script: {
  homeView?: import('@weknora/mobile-core').HomeView;
  homeError?: Error;
  pages?: Array<import('@weknora/mobile-core').TaskListPage>;
  listError?: Error;
  archived?: string[];
}) {
  const calls: string[] = [];
  let page = 0;
  return {
    calls,
    async home() {
      calls.push('home');
      if (script.homeError) throw script.homeError;
      return script.homeView ?? { needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf: '2026-09-23T00:00:00Z' };
    },
    async tasks(query: { search?: string; archived?: boolean; status?: string }) {
      calls.push(`tasks:${query.search ?? ''}:${query.archived ? 'archived' : 'active'}:${query.status ?? ''}`);
      if (script.listError) throw script.listError;
      return script.pages?.[0] ?? { items: [], duplicateRunIds: [] };
    },
    async moreTasks() {
      calls.push(`more:${page}`);
      page += 1;
      return script.pages?.[page] ?? { items: [], duplicateRunIds: [] };
    },
    async archive(taskId: string) { calls.push(`archive:${taskId}`); script.archived?.push(taskId); },
    async restore(taskId: string) { calls.push(`restore:${taskId}`); },
  };
}

test('HomeScreen renders the three aggregate segments, then error and retry states', async () => {
  hooks().__reset();
  const { HomeScreen } = await import('./screens/HomeScreen.tsx');
  const office = fakeTaskOffice({
    homeView: {
      needsMe: [{ interactionId: 'i1', kind: 'tool_approval', createdAt: '2026-09-23T00:00:00Z' }],
      running: [{ taskId: 't1', runId: 'r1', title: 'weekly report', runStatus: 'running', attention: 'none', updatedAt: '2026-09-23T00:00:00Z' }],
      recentlyCompleted: [{ taskId: 't2', runId: 'r2', title: 'research', runStatus: 'succeeded', attention: 'none', updatedAt: '2026-09-22T00:00:00Z' }],
      unreadNotifications: 4,
      asOf: '2026-09-23T00:00:01Z',
    },
  });
  const props = {
    deploymentLabel: 'WeKnora', tenants: [{ id: 'tenant-1', name: 'Acme' }], activeTenantId: 'tenant-1',
    onActivateTenant: () => {}, onSignOut: async () => {}, taskOffice: office as unknown as import('@weknora/mobile-core').TaskOffice,
  };
  let tree = render(HomeScreen, props);
  const textOf = (node: unknown): string => descendants(node).filter(({ type }) => type === 'Text').flatMap(({ props: p }) => p.children).join(' ');
  assert.equal(textOf(tree).includes('WeKnora'), true, 'the header renders before any data');
  const load = descendants(tree).find(({ type, props: button }) => type === 'Button' && button.title === 'Load home');
  (load!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  tree = render(HomeScreen, props);
  const text = textOf(tree);
  assert.equal(text.includes('weekly report'), true);
  assert.equal(text.includes('research'), true);
  assert.equal(text.includes('tool_approval'), true);
  assert.equal(text.includes('Unread 4'), true, 'the unread badge is visible');

  const failing = fakeTaskOffice({ homeError: new Error('TASK_OFFICE_BACKEND') });
  const errorProps = { ...props, taskOffice: failing as unknown as import('@weknora/mobile-core').TaskOffice };
  let errorTree = render(HomeScreen, errorProps);
  const retryLoad = descendants(errorTree).find(({ type, props: button }) => type === 'Button' && button.title === 'Load home');
  (retryLoad!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  errorTree = render(HomeScreen, errorProps);
  assert.equal(textOf(errorTree).includes('TASK_OFFICE_BACKEND'), true);
});

test('TasksScreen drives search, filters, archive and pagination through the module', async () => {
  hooks().__reset();
  const { TasksScreen } = await import('./screens/TasksScreen.tsx');
  const archived: string[] = [];
  const office = fakeTaskOffice({
    pages: [
      { items: [{ taskId: 't1', runId: 'r1', title: 'weekly report', runStatus: 'running', attention: 'required', updatedAt: '2026-09-23T00:00:00Z' }], nextCursor: 'c1', duplicateRunIds: [] },
      { items: [{ taskId: 't2', runId: 'r2', title: 'research', runStatus: 'succeeded', attention: 'none', updatedAt: '2026-09-22T00:00:00Z' }], duplicateRunIds: [] },
    ],
    archived,
  });
  const props = { taskOffice: office as unknown as import('@weknora/mobile-core').TaskOffice };
  let tree = render(TasksScreen, props);
  const search = descendants(tree).find(({ type }) => type === 'TextInput');
  (search!.props.onChangeText as (value: string) => void)('quarterly');
  tree = render(TasksScreen, props);
  const submit = descendants(tree).find(({ type, props: button }) => type === 'Button' && button.title === 'Search');
  (submit!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(office.calls, ['tasks:quarterly:active:']);

  tree = render(TasksScreen, props);
  const more = descendants(tree).find(({ type, props: button }) => type === 'Button' && button.title === 'Load more');
  (more!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(office.calls, ['tasks:quarterly:active:', 'more:0']);
  tree = render(TasksScreen, props);
  const text = descendants(tree).filter(({ type }) => type === 'Text').flatMap(({ props: p }) => p.children).join(' ');
  assert.equal(text.includes('weekly report'), true);
  assert.equal(text.includes('research'), true);

  const archive = descendants(render(TasksScreen, props)).find(({ type, props: button }) => type === 'Button' && button.title === 'Archive');
  (archive!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(archived, ['t1']);
  assert.equal(office.calls.includes('tasks:quarterly:active:'), true, 'the list reloads through the module after archiving');

  const emptyOffice = fakeTaskOffice({});
  const emptyTree = render(TasksScreen, { taskOffice: emptyOffice as unknown as import('@weknora/mobile-core').TaskOffice });
  const emptySearch = descendants(emptyTree).find(({ type }) => type === 'TextInput');
  (emptySearch!.props.onChangeText as (value: string) => void)('nothing');
  const emptySubmit = descendants(render(TasksScreen, { taskOffice: emptyOffice as unknown as import('@weknora/mobile-core').TaskOffice })).find(({ type, props: button }) => type === 'Button' && button.title === 'Search');
  (emptySubmit!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  const emptyText = descendants(render(TasksScreen, { taskOffice: emptyOffice as unknown as import('@weknora/mobile-core').TaskOffice })).filter(({ type }) => type === 'Text').flatMap(({ props: p }) => p.children).join(' ');
  assert.equal(emptyText.includes('No tasks yet'), true, 'empty is an empty state, never a silent success');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`Cannot find module './screens/HomeScreen.tsx'`（及 TasksScreen；authorized 分支 `type.name` 断言不等）。

- [ ] **Step 3: 最小实现**

(a) `apps/mobile/src/screens/HomeScreen.tsx`（新文件，完整内容）：

```tsx
import { useEffect, useState } from 'react';
import { Button, Text, View } from 'react-native';
import { router } from 'expo-router';
import type { HomeView, TaskOffice } from '@weknora/mobile-core';

export interface HomeScreenProps {
  deploymentLabel: string;
  tenants: Array<{ id: string; name?: string }>;
  activeTenantId: string;
  onActivateTenant(tenantId: string): void;
  onSignOut(): Promise<void>;
  taskOffice: TaskOffice;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/** Home 一级入口：三段聚合视图（module-seams §5.2 home(query)），只消费 Task Office 视图。 */
export function HomeScreen({ deploymentLabel, tenants, activeTenantId, onActivateTenant, onSignOut, taskOffice }: HomeScreenProps) {
  const [view, setView] = useState<HomeView | undefined>(undefined);
  const [error, setError] = useState<string | undefined>(undefined);
  const [loading, setLoading] = useState(false);
  const load = (): void => {
    setLoading(true);
    setError(undefined);
    void taskOffice.home().then(setView, (failure: unknown) => setError(errorMessage(failure))).finally(() => setLoading(false));
  };
  useEffect(load, []);
  return (
    <View>
      <Text>{deploymentLabel}</Text>
      {tenants.length > 1
        ? tenants.map((tenant) => (
            <Button key={tenant.id} title={tenant.name ?? tenant.id} onPress={() => onActivateTenant(tenant.id)} />
          ))
        : <Text>{tenants[0]?.name ?? activeTenantId}</Text>}
      <Button title="Sign out" onPress={() => { void onSignOut(); }} />
      <Button title="View all tasks" onPress={() => router.push('/tasks')} />
      {loading && <Text>Loading</Text>}
      {error !== undefined && (
        <View>
          <Text>{error}</Text>
          <Button title="Retry" onPress={load} />
        </View>
      )}
      {error === undefined && !loading && view !== undefined && (
        <View>
          <Text>{`Needs me (${view.needsMe.length})`}</Text>
          <Text>{`Unread ${view.unreadNotifications}`}</Text>
          {view.needsMe.map((item) => (
            <Text key={item.interactionId}>{`${item.kind} · ${item.createdAt}`}</Text>
          ))}
          {view.needsMe.length === 0 && view.unreadNotifications === 0 && <Text>Nothing needs you</Text>}
          <Text>{`Running (${view.running.length})`}</Text>
          {view.running.map((card) => (
            <Text key={card.runId}>{`${card.title || card.taskId} · ${card.runStatus}${card.attention === 'required' ? ' · needs you' : ''}`}</Text>
          ))}
          <Text>{`Recently completed (${view.recentlyCompleted.length})`}</Text>
          {view.recentlyCompleted.map((card) => (
            <Text key={card.runId}>{`${card.title || card.taskId} · ${card.runStatus}`}</Text>
          ))}
        </View>
      )}
      {/* 显式装载/重试控制：与测试桩（no-op useEffect）和真机刷新共用同一路径。 */}
      <Button title="Load home" onPress={load} />
    </View>
  );
}
```

(b) `apps/mobile/src/screens/TasksScreen.tsx`（新文件，完整内容）：

```tsx
import { useEffect, useState } from 'react';
import { Button, Text, TextInput, View } from 'react-native';
import type { TaskCard, TaskListPage, TaskOffice, TaskStatusFilter } from '@weknora/mobile-core';

export interface TasksScreenProps {
  taskOffice: TaskOffice;
}

const STATUS_FILTERS: Array<{ id: '' | TaskStatusFilter; label: string }> = [
  { id: '', label: 'All' },
  { id: 'running', label: 'Running' },
  { id: 'waiting_user', label: 'Waiting' },
  { id: 'succeeded', label: 'Done' },
  { id: 'failed', label: 'Failed' },
];

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/** Tasks 一级入口：搜索/筛选/归档与翻页；cursor 与查询身份归 Task Office，本屏只持有渲染态。 */
export function TasksScreen({ taskOffice }: TasksScreenProps) {
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState<'' | TaskStatusFilter>('');
  const [archived, setArchived] = useState(false);
  const [items, setItems] = useState<TaskCard[]>([]);
  const [duplicateRunIds, setDuplicateRunIds] = useState<string[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);

  const reload = (): void => {
    setLoading(true);
    setError(undefined);
    void taskOffice.tasks({ ...(search === '' ? {} : { search }), ...(status === '' ? {} : { status }), ...(archived ? { archived: true } : {}) })
      .then((page: TaskListPage) => {
        setItems(page.items);
        setDuplicateRunIds(page.duplicateRunIds);
        setHasMore(page.nextCursor !== undefined);
      }, (failure: unknown) => setError(errorMessage(failure)))
      .finally(() => setLoading(false));
  };
  useEffect(reload, []);

  const loadMore = (): void => {
    setLoading(true);
    void taskOffice.moreTasks()
      .then((page: TaskListPage) => {
        setItems((current) => [...current, ...page.items]);
        setDuplicateRunIds(page.duplicateRunIds);
        setHasMore(page.nextCursor !== undefined);
      }, (failure: unknown) => setError(errorMessage(failure)))
      .finally(() => setLoading(false));
  };

  const archive = (taskId: string, restore: boolean): void => {
    setLoading(true);
    void (restore ? taskOffice.restore(taskId) : taskOffice.archive(taskId))
      .then(reload, (failure: unknown) => setError(errorMessage(failure)))
      .finally(() => setLoading(false));
  };

  return (
    <View>
      <Text>Tasks</Text>
      <TextInput value={search} onChangeText={setSearch} placeholder="Search tasks" />
      {STATUS_FILTERS.map((filter) => (
        <Button key={filter.id} title={filter.label} onPress={() => { setStatus(filter.id); }} />
      ))}
      <Button title={archived ? 'Showing archived' : 'Showing active'} onPress={() => { setArchived(!archived); }} />
      <Button title="Search" onPress={reload} />
      {loading && <Text>Loading</Text>}
      {error !== undefined && <Text>{error}</Text>}
      {duplicateRunIds.length > 0 && <Text>{`Duplicated keys observed: ${duplicateRunIds.join(', ')}`}</Text>}
      {items.length === 0 && error === undefined && !loading && <Text>No tasks yet</Text>}
      {items.map((card) => (
        <View key={card.runId}>
          <Text>{`${card.title || card.taskId} · ${card.runStatus}${card.attention === 'required' ? ' · needs you' : ''}`}</Text>
          <Button title={archived ? 'Restore' : 'Archive'} onPress={() => { archive(card.taskId, archived); }} />
        </View>
      ))}
      {hasMore && <Button title="Load more" onPress={loadMore} />}
    </View>
  );
}
```

(c) `apps/mobile/src/app/tasks.tsx`（新文件，完整内容）：

```tsx
import { MobileTasks } from '../composition.ts';

/** /tasks 一级入口；Surface 仍由 Runtime 快照裁决。 */
export default function Tasks() {
  return <MobileTasks />;
}
```

(d) `apps/mobile/src/composition.ts`：

1. import 区：删除 `import { AuthorizedLandingScreen } from './screens/AuthorizedLandingScreen.tsx';` 一行（该屏由 HomeScreen 取代），既有 `import { createMobileRuntime } from '@weknora/mobile-core';` 与 `import type { MobileRuntime, RuntimeSnapshot } from '@weknora/mobile-core';` 保持，并追加：

```ts
import { createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { HomeScreen } from './screens/HomeScreen.tsx';
import { TasksScreen } from './screens/TasksScreen.tsx';
```

2. `createNativeMobileRuntime` 的 ports 对象中（`remoteFor(origin) {...}` 块之后）追加授权通道：

```ts
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(nativeFetch) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
```

3. `RuntimeSurface` 之前追加 Task Office 工厂与 `MobileTasks`：

```ts
const taskOffices = new Map<string, TaskOffice>();

/** Task Office 按 deployment origin 记忆化；lease 由 Runtime 提供，切租户即 fail closed。 */
function taskOfficeFor(activeRuntime: MobileRuntime, origin: string): TaskOffice {
  let office = taskOffices.get(origin);
  if (!office) {
    office = createTaskOffice({
      backend: createTaskOfficeRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
      lease: () => activeRuntime.scopeLease(),
    });
    taskOffices.set(origin, office);
  }
  return office;
}

/** /tasks 应用根：授权面才渲染列表屏，其余面回到 Runtime 裁决的 Surface。 */
export function MobileTasks() {
  const activeRuntime = runtime();
  const snapshot = useSyncExternalStore(activeRuntime.subscribe, activeRuntime.snapshot, activeRuntime.snapshot);
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return null;
  return createElement(TasksScreen, { taskOffice: taskOfficeFor(activeRuntime, snapshot.deployment.origin) });
}
```

4. `RuntimeSurface` 的 authorized 分支改为（保留 #32 增加的 `onActivateTenant`，透传给 HomeScreen）：

```ts
  if (snapshot.surface === 'authorized' && snapshot.deployment && snapshot.identity?.userId && snapshot.identity.activeTenantId) {
    return createElement(HomeScreen, {
      deploymentLabel: snapshot.deployment.label,
      tenants: snapshot.identity.tenants ?? [{ id: snapshot.identity.activeTenantId }],
      activeTenantId: snapshot.identity.activeTenantId,
      onActivateTenant: (tenantId: string) => { void onActivateTenant(tenantId); },
      onSignOut,
      taskOffice: taskOfficeFor(runtime(), snapshot.deployment.origin),
    });
  }
```

并新增 import `HomeScreen`、在 `RuntimeSurfaceProps` 保留 #32 的 `onActivateTenant?: (tenantId: string) => Promise<void> | void;`（若缺失则补上）。`AuthorizedLandingScreen` 从 authorized 分支移除后不再被引用：删除 `apps/mobile/src/screens/AuthorizedLandingScreen.tsx` 文件（replace-don't-layer，spec §14；其租户切换语义已迁入 HomeScreen 头部）。

- [ ] **Step 4: 运行确认通过（含类型检查 = 结构可赋值证明）**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: 全部 PASS / 0 错（typecheck 同时证明 `createTaskOfficeRemote` 返回类型可赋给 `TaskBackendPort`）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/screens/HomeScreen.tsx apps/mobile/src/screens/TasksScreen.tsx apps/mobile/src/app/tasks.tsx apps/mobile/src/composition.ts apps/mobile/src/app-smoke.test.tsx
git rm apps/mobile/src/screens/AuthorizedLandingScreen.tsx
git commit -m "feat(mobile): home attention segments and unified task list screens behind the task office (T04)"
```

---

### Task 9: 真实 HTTP 集成证据（AC3，opt-in）

**Files:**
- Create: `apps/mobile/src/task-office-integration-smoke.ts`
- Test: `packages/api-client/src/mobile/task-office.integration.test.ts`

**Interfaces:**
- Consumes: T01 的 opt-in 模式（`apps/mobile/src/runtime-integration-smoke.ts` 的 env 校验语义，本文件自包含复制，不 import #32 正在改动的文件）；Task 5/6/7/8 的 Runtime + Task Office + Remote 适配。
- Produces: `TaskOfficeIntegrationConfig`/`taskOfficeIntegrationConfig(env)`（环境变量 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`，与 T01/T02 相同集合）；`TaskOfficeIntegrationEvidence`（`home`、`sections`、`listSearch`、`archiveRoundtrip`、`commandTimestamp`——无任何凭据字段）；`runTaskOfficeIntegration(config)`。证据模式供 #38/#41/#44/#68 复用。

- [ ] **Step 1: 写失败的集成测试**

`packages/api-client/src/mobile/task-office.integration.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { emitTaskOfficeIntegrationEvidence, runTaskOfficeIntegration, taskOfficeIntegrationConfig } from '../../../../apps/mobile/src/task-office-integration-smoke.ts';

/**
 * Opt-in real HTTP check（AC3 端到端）。无回退凭据、无 mock：
 *
 * WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://deployment.example \
 * WEKNORA_MOBILE_TEST_EMAIL=mobile-test@example.test \
 * WEKNORA_MOBILE_TEST_PASSWORD=short-lived-secret \
 * pnpm exec tsx --test packages/api-client/src/mobile/task-office.integration.test.ts
 *
 * 证据只包含：部署 origin、三段计数、搜索/归档回路结果与时间戳；绝不包含 token/凭据。
 */
test('real HTTP home, list, search and archive roundtrip through the task office', async (t) => {
  const config = taskOfficeIntegrationConfig(process.env);
  if (!config.enabled) {
    if (config.disposition === 'skip') t.skip(`TASK_OFFICE_HTTP_SKIPPED: ${config.reason}`);
    else assert.fail(`TASK_OFFICE_HTTP_INVALID: ${config.reason}`);
    return;
  }
  const evidence = await runTaskOfficeIntegration(config);
  emitTaskOfficeIntegrationEvidence(evidence, (record) => t.diagnostic(record));
  assert.equal(evidence.home, 'loaded', 'the aggregate overview must load end to end');
  assert.equal(evidence.sections !== 'unavailable', true);
  assert.notEqual(evidence.listSearch, 'failed');
  assert.notEqual(evidence.archiveRoundtrip, 'failed');
  assert.doesNotMatch(JSON.stringify(evidence), /short-lived-secret|password|token|email/i);
});

test('a path-bearing deployment URL is invalid, not skippable', () => {
  const config = taskOfficeIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example/api/v1',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'short-lived-secret',
  });
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'invalid');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/api-client/src/mobile/task-office.integration.test.ts`
Expected: FAIL——`Cannot find module .../task-office-integration-smoke.ts`。

- [ ] **Step 3: 最小实现**

`apps/mobile/src/task-office-integration-smoke.ts`（新文件，完整内容）：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, type TaskOffice } from '@weknora/mobile-core';

export type TaskOfficeIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface TaskOfficeIntegrationEvidence {
  deploymentOrigin: string;
  home: 'loaded' | 'failed';
  sections: { needsMe: number; running: number; recentlyCompleted: number; unreadNotifications: number } | 'unavailable';
  listSearch: 'matched' | 'no-match' | 'failed';
  archiveRoundtrip: 'archived-restored' | 'unavailable' | 'failed';
  commandTimestamp: string;
}

/** 与 T01 mobileRuntimeIntegrationConfig 相同的 opt-in 语义（自包含，不跨计划 import）。 */
export function taskOfficeIntegrationConfig(env: Record<string, string | undefined>): TaskOfficeIntegrationConfig {
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
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

/**
 * 真实生产 JSON transport + 具体 Remote Adapter + Runtime 授权通道 + Task Office
 * 编排。归档回路只在账号确有任务时执行（否则如实记 'unavailable'，不伪造）。
 */
export async function runTaskOfficeIntegration(config: Extract<TaskOfficeIntegrationConfig, { enabled: true }>): Promise<TaskOfficeIntegrationEvidence> {
  const evidence: TaskOfficeIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    home: 'failed',
    sections: 'unavailable',
    listSearch: 'failed',
    archiveRoundtrip: 'unavailable',
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
  const snapshot = await runtime.signIn({
    deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
    email: config.email,
    password: config.password,
  });
  if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;

  const office: TaskOffice = createTaskOffice({
    backend: createTaskOfficeRemote({
      origin: config.deploymentOrigin,
      request: (input) => runtime.authorizedRequest(input),
    }),
    lease: () => runtime.scopeLease(),
  });

  const homeView = await office.home();
  evidence.home = 'loaded';
  evidence.sections = {
    needsMe: homeView.needsMe.length,
    running: homeView.running.length,
    recentlyCompleted: homeView.recentlyCompleted.length,
    unreadNotifications: homeView.unreadNotifications,
  };

  // 搜索：用一个必然不存在的随机词，验证参数贯通服务端过滤（no-match 是合法结果）。
  const nonce = `zz-t04-${Date.now().toString(36)}`;
  const searched = await office.tasks({ search: nonce });
  evidence.listSearch = searched.items.length === 0 ? 'no-match' : 'matched';

  // 归档回路：只在账号确有任务时执行，否则如实 'unavailable'。
  const activePage = await office.tasks({});
  if (activePage.items.length === 0) return evidence;
  const target = activePage.items[0]!;
  await office.archive(target.taskId);
  const archivedPage = await office.tasks({ archived: true });
  const archivedVisible = archivedPage.items.some((card) => card.taskId === target.taskId);
  await office.restore(target.taskId);
  const restoredPage = await office.tasks({});
  const restoredVisible = restoredPage.items.some((card) => card.taskId === target.taskId);
  evidence.archiveRoundtrip = archivedVisible && restoredVisible ? 'archived-restored' : 'failed';
  return evidence;
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitTaskOfficeIntegrationEvidence(evidence: TaskOfficeIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

- [ ] **Step 4: 运行确认通过（本地默认 skip 路径）**

Run: `npx tsx --test packages/api-client/src/mobile/task-office.integration.test.ts`
Expected: PASS——第一个用例输出 `TASK_OFFICE_HTTP_SKIPPED: missing ...`（无环境变量，诚实跳过）；第二个用例 PASS。若具备真实环境，按文件头注释设置环境变量重跑，断言全绿且 `t.diagnostic` 输出脱敏后的证据 JSON。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/task-office-integration-smoke.ts packages/api-client/src/mobile/task-office.integration.test.ts
git commit -m "test(mobile): opt-in real HTTP task office integration evidence (T04 AC3)"
```

---

## 计划级验证

在 worktree 根（`.worktrees/issue30-sweep`）执行（作者已实跑各分量的既有部分：`go test ./internal/handler/session/ -run TestMX013` PASS、`go test ./internal/application/repository/ -run 'TestMX013|TestWorkbenchList'` PASS、`npx tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts` 24/24、`pnpm --filter @weknora/mobile test` 11/11 + typecheck 0 错、contracts mobile 三组测试 PASS；新增测试由执行者按任务逐步实跑）：

```bash
go test ./internal/application/repository/ ./internal/handler/session/ -run 'TestWorkbenchList|TestWorkbenchTask|TestMX013|TestWorkbenchNotifications' -count=1 && npx tsx --test packages/contracts/test/mobile-read-models.test.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/task-office/task-office.test.ts packages/api-client/src/mobile/executions.test.ts packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/task-office.integration.test.ts && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck
```

覆盖说明：Task 1-3 的 Go 谓词/分页/归档/overview/迁移对齐测试（真实 sqlite 迁移库，含 `TestWorkbenchNotificationsTableExistsAfterMigrations`）+ Task 4 契约测试 + Task 5/6 mobile-core Interface 测试 + Task 7/9 api-client wire 与集成测试（无环境变量时真实 HTTP 用例 skip，其余断言全跑）+ Task 8 apps/mobile 测试与类型检查。有意不含 `pnpm test:shared` 全量、web/desktop 套件与 `pnpm --filter @weknora/miniprogram test`（后者在当前 worktree 有 2 个预存失败用例，见差异记录 1，与本计划无关）。

## 差异记录（调查结论 vs 代码现状）

1. 调查称「迟到查询/空错加载仅有 Taro 层测试证据（core.test.mjs 9/9）」——属实：作者实跑 `node --experimental-strip-types --test tests/core.test.mjs` 9/9 pass。但同一 miniprogram 套件的 `assembly.test.mjs` 在当前 worktree 有 **2 个预存失败**（`watchExecution installs the snapshot...` 与 `wire paths used by the miniprogram match the Go route table`，后者死于 `parseExecutionSnapshot` 的 `incomplete: expected a boolean`），与移动 v2 无关的本计划改动无法修复亦不应顺手修（属 #68 Taro 维护波次），故计划级验证排除 miniprogram 套件并在此留痕。
2. 调查称「统一列表无文本搜索、无归档」——属实：`WorkbenchExecutionFilter` 仅 `Status/AgentID/Cursor/Limit`（workbench_list.go:29-34），`packages/contracts/src/mobile` 无 archived 概念。本计划补齐：搜索（sessions.title，LIKE 转义）、归档（`sessions.archived_at`，ADR-0004 taskId=sessionId 落在任务身份上，#44 legacy 投影直接继承）。
3. 调查称「overview 无最近完成段、unread 恒为零」——属实（overview.go:82 仅活动态；:13 注释自认）。本计划以一条新的终态聚合查询与 `workbench_notifications` 真实计数补齐（建表见差异记录 11）；`recent_artifacts` 仍如实为空（MX-024/#46）。
4. 调查称「小程序首页用 sessions.list（pages.tsx:12）」——属实。小程序首页/任务列表向聚合读模型的迁移属 #68（T38 Taro Adapter 复用深 Module），本计划不改小程序页面；Task Office 的 Interface 不依赖 Expo 已由 in-memory scenario 测试证明（module-seams §14 步骤 5 的场景化部分）。
5. 调查称「授权聚合读模型是 owner-only 谓词，缺 Collaborator/Viewer 共享读投影」——属实且为 spec §16 已知差距；共享读投影属 #42（T12），本计划维持 tenant+owner 谓词并在全部新段复用（不静默扩大授权面）。
6. 调查称「Task/Run/Attention 三层状态在 wire 契约无字段」——部分属实：overview/list 已有 run/execution/settlement 三态，本计划补 `attention` 派生字段（`waiting_user` 或 pending interaction → `required`）；完整 Task lifecycle 投影（completed 可重启等）属 #35/#44。
7. 调查称「最高稳定 Interface（Task Office Module）不存在」——属实：`packages/mobile-core` 仅 `src/runtime/`。本计划创建模块并交付 `home/tasks/moreTasks/archive/restore`；`start(goal)`/`open(taskID)`（module-seams §5.2 全集）分属 #36/#35，属记录在案的分段交付而非接口重设计。
8. 仓库中 `/agent-archive`（NativeArchiveHandler）是「不可变历史命名空间」，与 Task 归档无关；本计划不复用该名字，路由用 `/workbench/tasks/:task_id/archive`。
9. `parseExecutionListPage`（contracts read-models.ts:150）是 MX-014 冻结提案，与真实列表 wire（`status` 而非 `run_status`、无 `as_of`）不一致且无消费方（作者核实：仅 `parseWorkbenchOverview` 被 api-client overview.ts 消费）。本计划不改动该冻结函数，Task Office 经 api-client 的真实 wire parser 取数；提案与 wire 的收敛留给 #35/#68，此处留痕。
10. Task Office 需要授权读通道而 #32 未产出：本计划新增 `MobileRuntime.authorizedRequest`（token 不出 Runtime、401 单飞刷新、迟到丢弃），成为 #33/#35/#38+ 可复用的 Produces；这符合 module-seams §4.3「credential refresh single-flight 隐藏在 Runtime」。
11. **（独立审查 F1，作者已复核属实）`workbench_notifications` 在生产迁移中不存在**：作者实跑 `grep -rln workbench_notifications migrations/sqlite/ migrations/versioned/` 无输出——全仓只有 gorm 投影 `InboxNotificationRow`（workbench_inbox.go:21-33，TableName "workbench_notifications"）与 MX-021 测试的 AutoMigrate（workbench_inbox_mx021_test.go:26）；migrations 里仅有的通知表是 `mobile_notification_intents/checkpoints`（sqlite 000059 / versioned 000146），是另一套推送投递表，无 read 状态、不可承载 unread 计数。这意味着既有 `InboxService.Inbox/MarkRead` 在生产库本就缺表，初稿让 overview unread 也查该表会把缺表扩大到首页核心读路径。修复：Task 3 增补建表迁移 `versioned/000190_workbench_notifications` + `sqlite/000111_workbench_notifications`（列集逐列对齐 `InboxNotificationRow`），并以 `TestWorkbenchNotificationsTableExistsAfterMigrations`（经 `openRunTestDB` 跑真实 `migrations/sqlite`）钉住「迁移↔投影」列集对齐。
12. **（独立审查 F3，作者已复核属实）sqlite 与 versioned 迁移是两套独立编号序列**：`migrations/sqlite/` 止于 000109、`migrations/versioned/` 止于 000188。归档迁移 sqlite 侧用 000110（不沿用 versioned 的 000189）、通知建表 sqlite 侧用 000111（不沿用 000190），避免后续惯例编号倒挂在已应用版本之前。
13. **（独立审查 F2，作者已复核属实）`@weknora/api-client` 的 package.json `exports` 是封闭子路径集合**（`.`/`./chat/native`/`./embed`/`./mobile/runtime`/`./transport`）：既有 `@weknora/api-client/mobile/runtime` 导入能跑正因为 `./mobile/runtime` 在 exports 里；Task 8/9 引用的 `@weknora/api-client/mobile/task-office` 必须在 Task 7 同步登记 `"./mobile/task-office": "./src/mobile/task-office.ts"`，否则 Node 报 `ERR_PACKAGE_PATH_NOT_EXPORTED`、typecheck/tsx 全挂。

## Consumes / Produces 汇总

**Consumes（前序计划 #32 的产出，逐字签名）：**
- `MobileRuntime.activateTenant(tenantId: string): Promise<RuntimeSnapshot>`；`RuntimeSnapshot.identity.tenants?: Array<{ id: string; name?: string }>`
- 包内 lease 内省：`RuntimeScopeLease`/`leaseActive(lease: ScopeLease | undefined): boolean`/`leaseScopeOf`/`LeaseScope`（`packages/mobile-core/src/runtime/scope-lease.ts`，不入公共导出，mobile-core 包内可 import）
- `RuntimeSurfaceProps.onActivateTenant`（apps/mobile 组合根语义：HomeScreen 头部沿用）
- #32 的 opt-in 集成证据模式（环境变量集合 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`；本计划 Task 9 复用同一集合，文件自包含）

**Consumes（已合并主干能力）：**
- T01 Mobile Runtime：`createMobileRuntime`/`MobileRuntimePorts`/`RuntimeSnapshot`/`ScopeLease`（`packages/mobile-core/src/runtime/*`）
- 既有 Go workbench：overview（MX-013）、列表 keyset 分页与坏 cursor 拒绝、`workbench_notifications`（MX-021）、`/auth/login` 与 `/system/capabilities`
- api-client：`createWeKnoraClient`/`createJsonTransport`/`createOverviewApi`/`createExecutionsApi`/`createMobileRuntimeRemote`

**Produces（后续 #35-#46、#68 等消费）：**
- Go wire：`GET /workbench/overview` 响应新增 `recently_completed`（终态、限 10、排除归档、含 `title`/`attention`）与真实 `counts.unread_notifications`；`GET /workbench/executions` 新增 `q`/`archived` 查询参数与 `title`/`attention`/`archived_at` 行字段；`POST/DELETE /api/v1/workbench/tasks/:task_id/archive`（owner 谓词，404 = 非本人任务）
- 迁移：`sessions.archived_at`（versioned `000189_task_archive` / sqlite `000110_task_archive`，两套独立编号序列）；`workbench_notifications` 建表（versioned `000190` / sqlite `000111`，列集对齐 `InboxNotificationRow`，同时修复既有 InboxService 生产缺表）+ `TestWorkbenchNotificationsTableExistsAfterMigrations` 列集对齐测试
- Go API：`repository.NewWorkbenchTaskStateStore`/`SetTaskArchived`/`ErrWorkbenchTaskNotFound`；`session.WorkbenchTaskStateHandler`
- contracts：`WorkbenchOverview.recently_completed`、`ExecutionSummary.attention?`、`AttentionState`
- mobile-core：`createTaskOffice(ports: TaskOfficePorts): TaskOffice`（`home/tasks/moreTasks/archive/restore`；`TaskOfficeError.code ∈ {'TASK_OFFICE_SCOPE_CHANGED'|'TASK_OFFICE_SUPERSEDED'|'TASK_OFFICE_NO_ACTIVE_QUERY'|'TASK_OFFICE_INVALID_INPUT'|'TASK_OFFICE_BACKEND'}`）；`TaskBackendPort` 及全部 DTO；`createScenarioTaskBackend`/`emptyOverview`
- `MobileRuntime.authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>` 与 `MobileRuntimePorts.authorizedTransport?`（授权读通道，#33/#35/#38 复用）
- api-client：`ExecutionListParams.search/archived`、`WorkbenchExecutionItem.title/attention/archived_at`、`createTaskOfficeRemote({ origin, request })`
- apps/mobile：`HomeScreen`/`TasksScreen`/`app/tasks.tsx`、composition 的 `taskOfficeFor(runtime, origin)` 工厂与 `MobileTasks`
- 集成证据：`TaskOfficeIntegrationEvidence` + `taskOfficeIntegrationConfig(env)`（`WEKNORA_MOBILE_TEST_*` 同集合）

## 验收标准 → 证据映射

| 验收标准 | 证据 |
|---|---|
| 1. 首页和列表不做逐会话 N+1，也不泄露其他 Tenant 内容 | Task 3（三段各一条 JOIN 聚合查询 + tenant+owner 谓词 + `MX013-OBSERVATION perSessionHTTPRequests:0` 保持）+ Task 2（搜索/归档/attention 全部在既有 keyset 查询内，同 owner/tenant 断言矩阵）+ Task 6（模块单次聚合调用断言 `backend.calls.length === 1`）+ Task 1（归档写路径 owner 谓词 404 矩阵） |
| 2. 迟到查询、分页重复键和空/错/加载状态均可验证 | Task 6（lease 撤销迟到拒绝、新查询 SUPERSEDED、跨页重复键 `duplicateRunIds` 观测且不重复渲染、空视图/`TASK_OFFICE_BACKEND` 类型化错误、pending promise 即加载态）+ Task 5（授权通道迟到丢弃）+ Task 2（cursor 绑定新 facet，跨筛选重放拒绝；既有并发插入翻页测试继续覆盖无重复无丢行）+ Task 8（屏级空态 `No tasks yet`/错误文本/加载文本渲染断言） |
| 3. 端到端行为通过最高稳定 Interface 验证 | 最高稳定 Interface = Task Office Module（Task 6 Interface 测试：真实编排 + scenario Adapter）+ Task 9 opt-in 真实 HTTP 回路（生产 transport + 具体 Remote Adapter + Runtime 授权通道 + 真后端聚合读模型与归档写回路），blocked-env 声明见 Global Constraints 末段；本地替代证据 = Task 4/7 wire 契约测试 + Task 1-3 Go 真库集成测试 |
