# T12: Task Owner、Collaborator、Viewer 协作（Issue #42）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立 Task 级显式协作授权（Owner 授予同租户成员 Viewer/Collaborator）并在扩额、个人连接、副作用审批三个通道落地"读取、参与和所有权权限严格分离"的门禁，全部行为在 HTTP wire 级集成测试中验证。

**Architecture:** Task = Session（ADR-0004），任务所有者权威谓词是 `sessions.user_id`。新增 `task_grants` 表（两套迁移序列）承载 Owner 的显式授予；`TaskGrantService` 提供 owner-only 的授予/撤销/列表与 `ResolveTaskAccess` 解析；被授予者通过 `AgentRunStore.GetRunForGrantedReader`（SQL EXISTS 子查询实时 JOIN `tenant_members` active 成员，成员停用即时收敛）读取任务详情/快照；运行入口（KnowledgeQA/AgentQA）保持结构性 owner-scope 不放宽；扩额门禁改为"任务 owner 或 billing 授权"；`/apps/actions` 的 prepare/approve 增加个人连接与审批者谓词。SP13 的 token 链接分享面保持原状（owner-or-Admin+），与本计划的成员显式授予面并存。

**Tech Stack:** Go 1.26（go.mod `github.com/Tencent/WeKnora`）、gin、GORM、golang-migrate（versioned/PostgreSQL + sqlite 两套序列）、SQLite 真库集成测试（httptest + gin + 真实迁移）。

**Spec:** `docs/plans/issue30-sweep/issues/issue-42.md`（验收标准原文）；领域术语 `CONTEXT.md`（私有任务/任务所有者/任务协作者/任务查看者/任务预算）；`docs/specs/2026-09-20-mobile-ai-office-design.md`（用户故事 49-53、59；Implementation/Testing Decisions）；`docs/adr/0004-task-is-session.md`。

## Global Constraints

逐字引用批准需求与项目级约束（每个任务的要求都隐含本节）：

- 「Viewer 不能运行；Collaborator 不能扩额、用 Owner 个人连接或审批其副作用。」（Issue #42 验收标准 1）
- 「共享资源不会自动扩大 Task 可见范围。」（Issue #42 验收标准 2）
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（Issue #42 验收标准 3）
- 「私有任务：默认仅创建者可访问的任务；使用共享知识、空间 Agent 或空间连接不会自动扩大任务可见范围，协作访问必须显式授予。」（CONTEXT.md）
- 「任务所有者（Task Owner）：创建并对任务承担管理责任的成员，控制任务共享、取消与归档，并负责涉及其个人连接、代码交付或其他外部副作用的授权。」（CONTEXT.md）
- 「任务协作者（Task Collaborator）：经任务所有者显式授权，可以评论、追加指令和请求新运行的成员；协作权不会转移任务所有权，也不会授予使用任务所有者个人连接或批准其外部副作用的权限。」（CONTEXT.md）
- 「任务查看者（Task Viewer）：经任务所有者显式授权，只能查看任务过程、引用、结果和允许下载的产物的成员。」（CONTEXT.md）
- 「任务预算（Task Budget）：……达到上限后运行持久暂停，只有任务所有者或获授权的账单管理员可以增加上限。……避免：……任务协作者可自行提高的预算。」（CONTEXT.md）
- 「Task 沿用 Session 作为唯一身份，并保持 `taskId = sessionId`，不新增第二个 Task 聚合身份。」（ADR-0004）
- 「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, partial external success, unknown outcomes and durable checkpoints.」（Spec Testing Decisions）
- 安全约束（Mimosa）：所有 SQL 一律参数绑定，不得拼接；不得在源码/测试写入可用凭据字面量；服务端请求仅 http/https 且拒绝内网/环回/保留地址（本计划不新增出站请求，仅数据库参数化查询）。
- 测试归属：Task 1-6 各自的单元/仓储测试钉实现；Task 7 的 wire 级集成测试是三条验收标准的最终证据（复用仓库既有"真实 sqlite 迁移 + 真实 store + httptest"模式，见 `internal/application/repository/workbench_http_integration_test.go`）。
- 任务结构：严格 RED → GREEN → REFACTOR；每任务以 commit 结束（编排层统一提交的批次除外——本计划执行时若由编排层接管提交，则各任务的 Commit 步骤改为"确认工作区仅含本任务文件"）。
- 迁移编号：versioned 下一号为 000191、sqlite 下一号为 000112（当前尾部 `migrations/versioned/000190_workbench_notifications`、`migrations/sqlite/000111_workbench_notifications`）。本计划与其余 6 个 B3 计划并行，**若集成时编号已被占用，整体顺延为下一个可用编号（内容不变）**，不得挤占他人编号。

## Review Focus

规格隐含、但任务测试未直接覆盖时最可能咬人的五类输入/失败模式（每条标注归属任务的测试）：

1. **停用/离租成员的 grant 残留访问**：grantee 被 suspend 或软删后，其 grant 行未清理，仍能读取任务详情。合理预期：读取即刻失效。—— Task 4 测试 `TestGetRunForGrantedReaderRespectsGrantsAndMembership`（u5 为 suspended 成员且持 grant 行的断言：`GetRunForGrantedReader` 的 EXISTS 子查询实时 JOIN `tenant_members` active 行）。
2. **跨租户探测泄露存在性**：tenant-2 成员用 tenant-1 的 taskID 调 grants API / 读快照。合理预期：统一 404，与"任务不存在"不可区分。—— Task 2 测试 `TestResolveTaskAccessCrossTenantIsUniform404` + Task 7 剧本 B（AC2 测试的跨租户断言段）。
3. **撤销后旧权限滞留**：Owner 撤销 grant 后，被撤销者下一次读取仍 200。合理预期：无缓存、下一次请求即 404。—— Task 7 测试 `TestTaskCollaborationEndToEndAC2SharedDoesNotWidenVisibility` 的末段"撤销立即生效"断言。
4. **Collaborator 借道幂等键扩额**：collaborator 重放 owner 已用过的 idempotency key 试图绕过角色门禁。合理预期：角色门禁先于幂等逻辑，仍 403。—— Task 5 测试 `TestExtendTaskBudgetRejectsCollaboratorAndBystander`（owner 先应用该 key 成功扩额，collaborator 重放同一 key 仍 403）。
5. **畸形输入与空 owner**：grantee_id 空白、role 非法（如 `"owner"`）、task 无 owner（sessions.user_id 为空）。合理预期：400 拒绝，绝不落库成可提权行。—— Task 2 测试 `TestGrantTaskAccessValidatesInputs`（含 role="owner" 必须被拒——任务角色由授权模型分配，不能自封；含 ownerless session（UserID=""）的独立用例）。

---

## 与调查结论的差异记录（以代码现状为准）

1. 调查称"扩额仅校验租户 scope 与幂等键，未区分请求者角色"。代码现状：路由组上已有 `RequireManageBillingForWrites`（`internal/router/routes_commercial.go:25`，要求租户 owner 角色或 billing grant），普通成员已被 403。真实缺口是反方向的：**任务所有者（contributor 角色）自己也不能扩自己的额**，且门禁与任务无关。Task 5 落地 CONTEXT.md 原文语义"任务所有者或获授权的账单管理员"。
2. 调查称"`Collaborator` 不能用 Owner 个人连接或审批其副作用无角色门禁"。代码现状：`/apps/actions` 写端点已被 `CanDriveActionWrites`（owner/admin 租户角色）挡住普通成员；真实缺口是**任务/连接维度**无谓词——任何 admin 可用他人个人连接 prepare、可审批他人发起的个人连接 action（`internal/handler/app_connector_action.go` 的 `PrepareAction`/`ApproveAction` 无任何连接所有权检查；对照 `BeginOCAuthorization` 已有同形谓词 `internal/handler/app_connector_oc.go:218`）。Task 6 补齐。
3. 现有 SP13 token 分享允许 Admin+ 铸造分享链接（`internal/application/service/session_share.go:27-32`，有测试钉住）。CONTEXT.md"任务所有者控制任务共享"的偏差在**本计划不修改该面**（改它会推翻 SP13 已交付行为与 11 项测试），而是在新的 task_grants 面从第一天实现 Owner-only 显式授予；两面并存（链接分享 = 匿名只读快照；grants = 具名成员角色）。
4. "Collaborator 可评论、追加指令、请求新运行"的**通道本体**不在本计划：运行提交入口归 #36（start(goal)）、运行中调整归 #37（B4）、批注归 #46（B3 并行，Task Material）。本计划交付它们将消费的授权模型（`types.TaskRoleCanRun` 谓词 + `ResolveTaskAccess`），并在 Consumes-Produces 中声明。运行入口现状是结构性 owner-scope（`parseQARequest` → `GetOwnedSession`，`internal/handler/session/qa.go:186-192`），本计划用回归测试钉住"引入 grant 不放宽运行面"（Viewer/Collaborator POST /agent-chat → 404）。
5. 移动端（apps/mobile、mobile-core、contracts）不在本计划：验收标准是服务端行为门禁，移动 UI 协作面属于 #46 及后续。本计划纯 Go 侧，最大化降低与 B3 并行计划的共享写面。

---

## 文件结构总览

| 文件 | 职责 | 任务 |
|---|---|---|
| `migrations/versioned/000191_task_grants.{up,down}.sql` | PostgreSQL/versioned 迁移 | 1 |
| `migrations/sqlite/000112_task_grants.{up,down}.sql` | sqlite 迁移 | 1 |
| `internal/types/task_grant.go` | TaskGrantRole/TaskAccessRole/谓词/TaskGrant/TaskAccess | 1 |
| `internal/application/repository/task_grant_store.go` | task_grants CRUD + TaskOwnerID | 1 |
| `internal/application/repository/task_grant_store_test.go` | 仓储测试（真实迁移库） | 1 |
| `internal/application/service/task_grant.go` | 授权语义（owner-only、成员校验、ResolveTaskAccess） | 2 |
| `internal/application/service/task_grant_test.go` | 服务测试（stub 仓储） | 2 |
| `internal/handler/session/workbench_task_grants.go` | grants API handler | 3 |
| `internal/handler/session/workbench_task_grants_test.go` | handler 测试（fake manager） | 3 |
| `internal/router/routes_workbench.go`（修改：追加函数） | 路由注册 | 3 |
| `internal/router/router.go`（修改：params + 1 行注册） | 挂载 | 3 |
| `internal/container/container.go`（修改：1 行 Provide） | DI | 3 |
| `internal/container/workbench.go`（修改：wiring 函数 + read handler 注入） | DI | 3、4 |
| `internal/application/repository/agent_run.go`（修改：追加方法） | GetRunForGrantedReader | 4 |
| `internal/handler/session/workbench_read.go`（修改：seam + 2 端点） | 被授予者读取 | 4 |
| `internal/application/repository/task_grant_read_test.go` | granted read 仓储测试 | 4 |
| `internal/handler/session/workbench_read_grants_test.go` | read handler seam 测试 | 4 |
| `internal/handler/commercial_task_budget.go`（修改） | 扩额门禁 | 5 |
| `internal/router/routes_commercial.go`（修改：路由移组） | 扩额路由 | 5 |
| `internal/handler/commercial_task_budget_test.go` | 扩额门禁测试（真库 wire 级） | 5 |
| `internal/handler/app_connector_action.go`（修改） | 个人连接/审批谓词 | 6 |
| `internal/handler/app_connector_action_grant_test.go` | action 谓词测试（复用 oc 脚手架） | 6 |
| `internal/application/repository/task_collaboration_http_test.go` | 端到端剧本（AC1/AC2/AC3） | 7 |
| `internal/handler/session/task_collaboration_run_test.go` | 运行入口 404 回归（真实 QA handler） | 7 |

共享文件修改均是最小追加（新函数/新行），位置明确，便于与 B3 其余计划合并。

---

### Task 1: task_grants 迁移、类型与仓储

**Files:**
- Create: `migrations/versioned/000191_task_grants.up.sql`
- Create: `migrations/versioned/000191_task_grants.down.sql`
- Create: `migrations/sqlite/000112_task_grants.up.sql`
- Create: `migrations/sqlite/000112_task_grants.down.sql`
- Create: `internal/types/task_grant.go`
- Create: `internal/application/repository/task_grant_store.go`
- Test: `internal/application/repository/task_grant_store_test.go`

**Interfaces:**
- Consumes: #34 的两套迁移序列尾部（versioned 000190 / sqlite 000111）；`types.Session`（`internal/types/session.go:76`，`UserID` 为 owner 权威）。
- Produces:
  - `types.TaskGrantRole string`（`TaskGrantRoleViewer TaskGrantRole = "viewer"`、`TaskGrantRoleCollaborator TaskGrantRole = "collaborator"`）、`func (r TaskGrantRole) IsValid() bool`
  - `types.TaskAccessRole string`（`TaskAccessNone/TaskAccessViewer/TaskAccessCollaborator/TaskAccessOwner`）、`types.TaskRoleCanRun(r TaskAccessRole) bool`、`types.TaskRoleCanManage(r TaskAccessRole) bool`
  - `types.TaskGrant`（gorm 表 `task_grants`，PK = tenant_id+task_id+grantee_id）
  - `types.TaskAccess{TaskID string; OwnerID string; Role TaskAccessRole; GrantRole TaskGrantRole}`
  - `repository.NewTaskGrantStore(db *gorm.DB) *TaskGrantStore`，方法：`UpsertGrant(ctx context.Context, tenantID uint64, taskID, granteeID string, role types.TaskGrantRole, grantedBy string) (types.TaskGrant, error)`、`DeleteGrant(ctx context.Context, tenantID uint64, taskID, granteeID string) error`、`ListGrants(ctx context.Context, tenantID uint64, taskID string) ([]types.TaskGrant, error)`、`RoleForGrantee(ctx context.Context, tenantID uint64, taskID, granteeID string) (types.TaskGrantRole, bool, error)`、`TaskOwnerID(ctx context.Context, tenantID uint64, taskID string) (string, error)`
  - `var repository.ErrTaskGrantTaskNotFound = errors.New("task not found in tenant")`

- [ ] **Step 1: Write the failing test**

创建 `internal/application/repository/task_grant_store_test.go`（package `repository_test`，与 `task_collaboration_http_test.go`（Task 7）共享 `openTaskGrantDB`）：

```go
package repository_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openTaskGrantDB opens a REAL migrated sqlite database (full migrations/sqlite
// track, same pattern as openWorkbenchHTTPDB) and seeds the collaboration
// fixtures: tenant 1 with owner u1, viewer-candidate u2, collaborator-candidate
// u3 and bystander u4, plus u1's task s1 and a suspended member u5.
func openTaskGrantDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "task-grants.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	seedTaskGrantFixtures(t, db)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func seedTaskGrantFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')`).Error)
	for _, u := range []string{"u1", "u2", "u3", "u4", "u5"} {
		require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', 1)`, u, u, u+"@example.test").Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'task-1', 'u1', 'trpc')`).Error)
	// u5 is seeded suspended: an existing grant for a deactivated member must
	// stop resolving (the granted-read SQL joins active memberships).
	for _, m := range []struct {
		user   string
		role   types.TenantRole
		status types.TenantMemberStatus
	}{
		{"u1", types.TenantRoleContributor, types.TenantMemberStatusActive},
		{"u2", types.TenantRoleViewer, types.TenantMemberStatusActive},
		{"u3", types.TenantRoleContributor, types.TenantMemberStatusActive},
		{"u4", types.TenantRoleContributor, types.TenantMemberStatusActive},
		{"u5", types.TenantRoleViewer, types.TenantMemberStatusSuspended},
	} {
		require.NoError(t, db.Create(&types.TenantMember{
			UserID: m.user, TenantID: 1, Role: m.role, Status: m.status, JoinedAt: time.Now().UTC(),
		}).Error)
	}
}

func TestTaskGrantsTableExistsAfterMigrations(t *testing.T) {
	db := openTaskGrantDB(t)
	require.True(t, db.Migrator().HasTable("task_grants"),
		"task_grants must be created by the production migrations")
	for _, column := range []string{"tenant_id", "task_id", "grantee_id", "role", "granted_by", "created_at", "updated_at"} {
		require.True(t, db.Migrator().HasColumn("task_grants", column),
			"task_grants.%s must exist (aligned with types.TaskGrant)", column)
	}
}

func TestTaskGrantStoreUpsertListRevokeAndRoleLookup(t *testing.T) {
	db := openTaskGrantDB(t)
	store := repository.NewTaskGrantStore(db)
	ctx := context.Background()

	grant, err := store.UpsertGrant(ctx, 1, "s1", "u2", types.TaskGrantRoleViewer, "u1")
	require.NoError(t, err)
	require.Equal(t, types.TaskGrantRoleViewer, grant.Role)
	require.Equal(t, "u1", grant.GrantedBy)

	// Upsert flips the role in place (one row per (tenant, task, grantee)).
	grant, err = store.UpsertGrant(ctx, 1, "s1", "u2", types.TaskGrantRoleCollaborator, "u1")
	require.NoError(t, err)
	require.Equal(t, types.TaskGrantRoleCollaborator, grant.Role)

	grants, err := store.ListGrants(ctx, 1, "s1")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, "u2", grants[0].GranteeID)

	role, found, err := store.RoleForGrantee(ctx, 1, "s1", "u2")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, types.TaskGrantRoleCollaborator, role)

	// Cross-tenant and unknown lookups are plain misses.
	_, found, err = store.RoleForGrantee(ctx, 2, "s1", "u2")
	require.NoError(t, err)
	require.False(t, found)

	// Delete is idempotent.
	require.NoError(t, store.DeleteGrant(ctx, 1, "s1", "u2"))
	require.NoError(t, store.DeleteGrant(ctx, 1, "s1", "u2"))
	grants, err = store.ListGrants(ctx, 1, "s1")
	require.NoError(t, err)
	require.Empty(t, grants)
}

func TestTaskGrantStoreTaskOwnerIDResolvesSessionOwner(t *testing.T) {
	db := openTaskGrantDB(t)
	store := repository.NewTaskGrantStore(db)
	ctx := context.Background()

	owner, err := store.TaskOwnerID(ctx, 1, "s1")
	require.NoError(t, err)
	require.Equal(t, "u1", owner, "the task owner authority is sessions.user_id (ADR-0004)")

	_, err = store.TaskOwnerID(ctx, 2, "s1")
	require.ErrorIs(t, err, repository.ErrTaskGrantTaskNotFound, "cross-tenant task id is a uniform miss")
	_, err = store.TaskOwnerID(ctx, 1, "missing")
	require.ErrorIs(t, err, repository.ErrTaskGrantTaskNotFound)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/application/repository/ -run 'TestTaskGrant' -count=1`
Expected: FAIL（编译错误 `undefined: repository.NewTaskGrantStore` / `types.TaskGrantRoleViewer`）。

- [ ] **Step 3: Write minimal implementation**

`internal/types/task_grant.go`：

```go
package types

import "time"

// TaskGrantRole is the per-task collaboration role a Task Owner assigns to a
// tenant member. It is strictly distinct from the tenant-level TenantRole:
// a grant never carries tenant authority, and tenant authority never implies
// a grant (CONTEXT.md: 任务协作者/任务查看者).
type TaskGrantRole string

const (
	// TaskGrantRoleViewer: read-only access to the task (任务查看者).
	TaskGrantRoleViewer TaskGrantRole = "viewer"
	// TaskGrantRoleCollaborator: may comment, add instructions and request
	// new runs (任务协作者) — never budget, personal connections or approval
	// of the owner's side effects.
	TaskGrantRoleCollaborator TaskGrantRole = "collaborator"
)

// IsValid reports whether r is one of the two defined grant roles. "owner" is
// deliberately NOT a grantable value: ownership is inherited from creating
// the task and cannot be assigned here.
func (r TaskGrantRole) IsValid() bool {
	return r == TaskGrantRoleViewer || r == TaskGrantRoleCollaborator
}

// TaskAccessRole is the resolved per-task role of one caller, including the
// two states that never live in task_grants rows (owner, none).
type TaskAccessRole string

const (
	TaskAccessNone         TaskAccessRole = "none"
	TaskAccessViewer       TaskAccessRole = "viewer"
	TaskAccessCollaborator TaskAccessRole = "collaborator"
	TaskAccessOwner        TaskAccessRole = "owner"
)

// TaskRoleCanRun reports whether the role may request a new run. Only the
// owner and collaborators can (Issue #42 AC1: Viewer 不能运行).
func TaskRoleCanRun(r TaskAccessRole) bool {
	return r == TaskAccessOwner || r == TaskAccessCollaborator
}

// TaskRoleCanManage reports whether the role may perform ownership actions:
// managing grants, archiving, raising the task budget, approving the task's
// external side effects (Issue #42 AC1).
func TaskRoleCanManage(r TaskAccessRole) bool {
	return r == TaskAccessOwner
}

// TaskGrant persists one explicit per-task role assignment. Task = Session
// (ADR-0004), so TaskID references sessions.id; the owner authority itself
// is sessions.user_id and NEVER lives in this table.
type TaskGrant struct {
	TenantID  uint64        `json:"tenant_id" gorm:"primaryKey;column:tenant_id"`
	TaskID    string        `json:"task_id" gorm:"primaryKey;column:task_id;type:varchar(36)"`
	GranteeID string        `json:"grantee_id" gorm:"primaryKey;column:grantee_id;type:varchar(512)"`
	Role      TaskGrantRole `json:"role" gorm:"column:role;type:varchar(16);not null"`
	GrantedBy string        `json:"granted_by" gorm:"column:granted_by;type:varchar(512);not null"`
	CreatedAt time.Time     `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time     `json:"updated_at" gorm:"column:updated_at"`
}

// TableName binds TaskGrant to the task_grants table.
func (TaskGrant) TableName() string { return "task_grants" }

// TaskAccess is the resolved access of one caller to one task.
type TaskAccess struct {
	TaskID    string
	OwnerID   string
	Role      TaskAccessRole
	GrantRole TaskGrantRole
}
```

`migrations/versioned/000191_task_grants.up.sql`：

```sql
-- Task collaboration grants (T12, #42): the task owner's explicit per-task
-- Viewer/Collaborator role assignments. Task = Session (ADR-0004), so task_id
-- references sessions.id; task ownership itself (sessions.user_id) never
-- lives here. Grant rows carry no tenant authority.
CREATE TABLE task_grants (
    tenant_id INTEGER NOT NULL,
    task_id VARCHAR(36) NOT NULL,
    grantee_id VARCHAR(512) NOT NULL,
    role VARCHAR(16) NOT NULL,
    granted_by VARCHAR(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, task_id, grantee_id)
);

CREATE INDEX idx_task_grants_grantee ON task_grants (tenant_id, grantee_id);
```

`migrations/versioned/000191_task_grants.down.sql`：

```sql
DROP TABLE IF EXISTS task_grants;
```

`migrations/sqlite/000112_task_grants.up.sql`：

```sql
-- Task collaboration grants (T12, #42) — sqlite track. Same shape as the
-- versioned migration; task_id references sessions.id (ADR-0004).
CREATE TABLE task_grants (
    tenant_id INTEGER NOT NULL,
    task_id TEXT NOT NULL,
    grantee_id TEXT NOT NULL,
    role TEXT NOT NULL,
    granted_by TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, task_id, grantee_id)
);

CREATE INDEX idx_task_grants_grantee ON task_grants (tenant_id, grantee_id);
```

`migrations/sqlite/000112_task_grants.down.sql`：

```sql
DROP TABLE IF EXISTS task_grants;
```

`internal/application/repository/task_grant_store.go`：

```go
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrTaskGrantTaskNotFound marks a task-id lookup that found no session row
// inside the tenant. Grant surfaces answer a uniform miss so a cross-tenant
// probe learns nothing.
var ErrTaskGrantTaskNotFound = errors.New("task not found in tenant")

// TaskGrantStore persists task_grants rows. Every method binds to the exact
// (tenant, task) pair; the task owner authority is sessions.user_id and is
// resolved by TaskOwnerID, never by a grant row.
type TaskGrantStore struct{ db *gorm.DB }

func NewTaskGrantStore(db *gorm.DB) *TaskGrantStore {
	return &TaskGrantStore{db: db}
}

// UpsertGrant inserts or rewrites one grant. Re-granting an existing grantee
// flips the role in place (one row per grantee), mirroring "share-again
// rotates" semantics without a second row.
func (s *TaskGrantStore) UpsertGrant(
	ctx context.Context, tenantID uint64, taskID, granteeID string,
	role types.TaskGrantRole, grantedBy string,
) (types.TaskGrant, error) {
	now := time.Now().UTC()
	grant := types.TaskGrant{
		TenantID: tenantID, TaskID: taskID, GranteeID: granteeID,
		Role: role, GrantedBy: grantedBy, CreatedAt: now, UpdatedAt: now,
	}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"}, {Name: "task_id"}, {Name: "grantee_id"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"role":       role,
			"granted_by": grantedBy,
			"updated_at": now,
		}),
	}).Create(&grant).Error
	if err != nil {
		return types.TaskGrant{}, err
	}
	return grant, nil
}

// DeleteGrant removes one grant. Deleting an absent grant is a success
// (idempotent revoke).
func (s *TaskGrantStore) DeleteGrant(ctx context.Context, tenantID uint64, taskID, granteeID string) error {
	return s.db.WithContext(ctx).
		Where("tenant_id = ? AND task_id = ? AND grantee_id = ?", tenantID, taskID, granteeID).
		Delete(&types.TaskGrant{}).Error
}

// ListGrants returns the task's grants, ordered deterministically by grantee.
func (s *TaskGrantStore) ListGrants(ctx context.Context, tenantID uint64, taskID string) ([]types.TaskGrant, error) {
	var grants []types.TaskGrant
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND task_id = ?", tenantID, taskID).
		Order("grantee_id ASC").Find(&grants).Error
	return grants, err
}

// RoleForGrantee resolves one grantee's role on the task. found=false covers
// no grant, cross-tenant task and unknown task alike.
func (s *TaskGrantStore) RoleForGrantee(
	ctx context.Context, tenantID uint64, taskID, granteeID string,
) (types.TaskGrantRole, bool, error) {
	if s == nil || s.db == nil || tenantID == 0 ||
		strings.TrimSpace(taskID) == "" || strings.TrimSpace(granteeID) == "" {
		return "", false, nil
	}
	var grant types.TaskGrant
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND task_id = ? AND grantee_id = ?", tenantID, taskID, granteeID).
		Take(&grant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return grant.Role, true, nil
}

// TaskOwnerID resolves the task's owner: sessions.user_id inside the tenant
// (ADR-0004). A miss — unknown task, cross-tenant probe — is one uniform
// ErrTaskGrantTaskNotFound.
func (s *TaskGrantStore) TaskOwnerID(ctx context.Context, tenantID uint64, taskID string) (string, error) {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(taskID) == "" {
		return "", ErrTaskGrantTaskNotFound
	}
	var owner string
	err := s.db.WithContext(ctx).Table("sessions").
		Where("tenant_id = ? AND id = ?", tenantID, taskID).
		Select("user_id").Scan(&owner).Error
	if err != nil {
		return "", err
	}
	if owner == "" {
		return "", ErrTaskGrantTaskNotFound
	}
	return owner, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/application/repository/ -run 'TestTaskGrant' -count=1`
Expected: PASS（3 项测试全绿；`TestTaskGrantsTableExistsAfterMigrations` 证明迁移↔投影对齐）。

- [ ] **Step 5: Commit**

```bash
git add migrations/versioned/000191_task_grants.up.sql migrations/versioned/000191_task_grants.down.sql migrations/sqlite/000112_task_grants.up.sql migrations/sqlite/000112_task_grants.down.sql internal/types/task_grant.go internal/application/repository/task_grant_store.go internal/application/repository/task_grant_store_test.go
git commit -m "feat(collaboration): task_grants migration, types and store (T12 #42)"
```

---

### Task 2: TaskGrantService 授权语义（owner-only、成员校验、ResolveTaskAccess）

**Files:**
- Create: `internal/application/service/task_grant.go`
- Test: `internal/application/service/task_grant_test.go`

**Interfaces:**
- Consumes: Task 1 的 `repository.TaskGrantStore` 方法集、`types.TaskGrantRole/TaskAccess`、`interfaces.SessionRepository.GetByID(ctx, tenantID, id)`（`internal/application/repository/session.go:65`）、`interfaces.TenantMemberRepository.Get(ctx, userID, tenantID)`（active 行，miss 返回 `(nil, nil)`，`internal/application/repository/tenant_member.go:54`）。
- Produces:
  - `service.TaskGrantStorePort`（上面 5 个方法签名的接口）
  - `service.TaskMemberLookupPort` 接口：`Get(ctx context.Context, userID string, tenantID uint64) (*types.TenantMember, error)`（`interfaces.TenantMemberRepository` 天然满足）
  - `service.NewTaskGrantService(grants TaskGrantStorePort, sessions interfaces.SessionRepository, members TaskMemberLookupPort) *TaskGrantService`
  - `(*TaskGrantService).GrantTaskAccess(ctx context.Context, caller types.Caller, taskID, granteeID string, role types.TaskGrantRole) (*types.TaskGrant, error)`
  - `(*TaskGrantService).RevokeTaskAccess(ctx context.Context, caller types.Caller, taskID, granteeID string) error`
  - `(*TaskGrantService).ListTaskGrants(ctx context.Context, caller types.Caller, taskID string) ([]types.TaskGrant, error)`
  - `(*TaskGrantService).ResolveTaskAccess(ctx context.Context, caller types.Caller, taskID string) (types.TaskAccess, error)`
  - 错误用 `apperrors`：`NewForbiddenError("only the task owner may manage task access")` / `NewNotFoundError("task not found")` / `NewBadRequestError(...)`。

- [ ] **Step 1: Write the failing test**

创建 `internal/application/service/task_grant_test.go`（照 `session_share_test.go` 的 stub 模式）：

```go
package service

// Task-grant service tests (T12 #42): the owner-only gate (tenant Admin is
// NOT admitted — the grant surface is Owner-explicit by design, unlike the
// SP13 share-token surface), grantee must be an active same-tenant member,
// malformed roles are refused, and ResolveTaskAccess separates owner /
// collaborator / viewer / none. Cross-tenant probes are a uniform 404.

import (
	"context"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type stubTaskGrantRepo struct {
	grants map[string]types.TaskGrant // key: taskID + "|" + granteeID
}

func taskGrantKey(taskID, granteeID string) string { return taskID + "|" + granteeID }

func (r *stubTaskGrantRepo) UpsertGrant(
	_ context.Context, tenantID uint64, taskID, granteeID string,
	role types.TaskGrantRole, grantedBy string,
) (types.TaskGrant, error) {
	grant := types.TaskGrant{
		TenantID: tenantID, TaskID: taskID, GranteeID: granteeID,
		Role: role, GrantedBy: grantedBy,
	}
	if r.grants == nil {
		r.grants = map[string]types.TaskGrant{}
	}
	r.grants[taskGrantKey(taskID, granteeID)] = grant
	return grant, nil
}

func (r *stubTaskGrantRepo) DeleteGrant(_ context.Context, _ uint64, taskID, granteeID string) error {
	delete(r.grants, taskGrantKey(taskID, granteeID))
	return nil
}

func (r *stubTaskGrantRepo) ListGrants(_ context.Context, _ uint64, taskID string) ([]types.TaskGrant, error) {
	var grants []types.TaskGrant
	for _, g := range r.grants {
		if g.TaskID == taskID {
			grants = append(grants, g)
		}
	}
	return grants, nil
}

func (r *stubTaskGrantRepo) RoleForGrantee(
	_ context.Context, _ uint64, taskID, granteeID string,
) (types.TaskGrantRole, bool, error) {
	g, ok := r.grants[taskGrantKey(taskID, granteeID)]
	return g.Role, ok, nil
}

func (r *stubTaskGrantRepo) TaskOwnerID(_ context.Context, tenantID uint64, taskID string) (string, error) {
	return "", apperrors.ErrSessionNotFound
}

type stubTaskGrantSessions struct {
	interfaces.SessionRepository
	session *types.Session
}

func (r *stubTaskGrantSessions) GetByID(_ context.Context, tenantID uint64, id string) (*types.Session, error) {
	if r.session != nil && r.session.ID == id && r.session.TenantID == tenantID {
		return r.session, nil
	}
	return nil, apperrors.ErrSessionNotFound
}

type stubTaskGrantMembers struct {
	members map[string]*types.TenantMember // key: userID
}

func (m *stubTaskGrantMembers) Get(_ context.Context, userID string, _ uint64) (*types.TenantMember, error) {
	return m.members[userID], nil
}

func newTaskGrantServiceForTest(
	grants *stubTaskGrantRepo, session *types.Session, members *stubTaskGrantMembers,
) *TaskGrantService {
	return NewTaskGrantService(grants, &stubTaskGrantSessions{session: session}, members)
}

func taskGrantOwner() types.Caller {
	return types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleContributor}
}

func TestGrantTaskAccessOwnerExplicitOnly(t *testing.T) {
	ctx := context.Background()
	grants := &stubTaskGrantRepo{}
	members := &stubTaskGrantMembers{members: map[string]*types.TenantMember{
		"member-2": {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusActive},
	}}
	svc := newTaskGrantServiceForTest(grants, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}, members)

	// Owner grants a viewer.
	grant, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "member-2", types.TaskGrantRoleViewer)
	require.NoError(t, err)
	require.Equal(t, types.TaskGrantRoleViewer, grant.Role)
	require.Equal(t, "owner-1", grant.GrantedBy)

	// A tenant Admin is NOT admitted on the grant surface: the explicit
	// member-role assignment is the owner's alone (CONTEXT.md 任务所有者;
	// contrast canShareSession for the SP13 token surface).
	admin := types.Caller{TenantID: 1, UserID: "admin-1", Role: types.TenantRoleAdmin}
	_, err = svc.GrantTaskAccess(ctx, admin, "s1", "member-2", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrForbidden, errorCodeOf(t, err))

	// Another member cannot grant either.
	other := types.Caller{TenantID: 1, UserID: "member-2", Role: types.TenantRoleContributor}
	_, err = svc.GrantTaskAccess(ctx, other, "s1", "member-2", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrForbidden, errorCodeOf(t, err))
}

func TestGrantTaskAccessValidatesInputs(t *testing.T) {
	ctx := context.Background()
	members := &stubTaskGrantMembers{members: map[string]*types.TenantMember{
		"member-2": {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusActive},
		"suspended": {UserID: "suspended", TenantID: 1, Status: types.TenantMemberStatusSuspended},
	}}
	svc := newTaskGrantServiceForTest(&stubTaskGrantRepo{}, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}, members)

	for _, tc := range []struct {
		name      string
		granteeID string
		role      types.TaskGrantRole
	}{
		{"blank grantee", "   ", types.TaskGrantRoleViewer},
		{"invalid role", "member-2", "owner"},
		{"invalid role empty", "member-2", ""},
	} {
		_, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", tc.granteeID, tc.role)
		require.Equal(t, apperrors.ErrBadRequest, errorCodeOf(t, err), tc.name)
	}

	// Granting the owner themself is refused.
	_, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "owner-1", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrBadRequest, errorCodeOf(t, err))

	// A suspended member cannot hold a grant.
	_, err = svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "suspended", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrBadRequest, errorCodeOf(t, err))

	// A user with no membership row cannot hold a grant.
	_, err = svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "stranger", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrBadRequest, errorCodeOf(t, err))

	// An ownerless task (sessions.user_id = '') cannot anchor a grant — the
	// grant has no authority to hang from.
	ownerless := newTaskGrantServiceForTest(
		&stubTaskGrantRepo{}, &types.Session{ID: "s9", TenantID: 1, UserID: ""},
		&stubTaskGrantMembers{members: map[string]*types.TenantMember{
			"member-2": {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusActive},
		}},
	)
	_, err = ownerless.GrantTaskAccess(ctx, taskGrantOwner(), "s9", "member-2", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrBadRequest, errorCodeOf(t, err))

	// Unknown task is a 404 miss.
	_, err = svc.GrantTaskAccess(ctx, taskGrantOwner(), "missing", "member-2", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrNotFound, errorCodeOf(t, err))
}

func TestRevokeAndListTaskGrantsOwnerOnly(t *testing.T) {
	ctx := context.Background()
	grants := &stubTaskGrantRepo{}
	members := &stubTaskGrantMembers{members: map[string]*types.TenantMember{
		"member-2": {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusActive},
	}}
	svc := newTaskGrantServiceForTest(grants, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}, members)

	_, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "member-2", types.TaskGrantRoleViewer)
	require.NoError(t, err)

	// A grantee cannot revoke their own grant.
	err = svc.RevokeTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "member-2"}, "s1", "member-2")
	require.Equal(t, apperrors.ErrForbidden, errorCodeOf(t, err))

	// Owner revokes; revoking again is still a success (idempotent).
	require.NoError(t, svc.RevokeTaskAccess(ctx, taskGrantOwner(), "s1", "member-2"))
	require.NoError(t, svc.RevokeTaskAccess(ctx, taskGrantOwner(), "s1", "member-2"))
	list, err := svc.ListTaskGrants(ctx, taskGrantOwner(), "s1")
	require.NoError(t, err)
	require.Empty(t, list)

	// Listing is owner-only too.
	_, err = svc.ListTaskGrants(ctx, types.Caller{TenantID: 1, UserID: "member-2"}, "s1")
	require.Equal(t, apperrors.ErrForbidden, errorCodeOf(t, err))
}

func TestResolveTaskAccessSeparatesRoles(t *testing.T) {
	ctx := context.Background()
	grants := &stubTaskGrantRepo{}
	members := &stubTaskGrantMembers{members: map[string]*types.TenantMember{
		"member-2": {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusActive},
		"member-3": {UserID: "member-3", TenantID: 1, Status: types.TenantMemberStatusActive},
	}}
	svc := newTaskGrantServiceForTest(grants, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}, members)

	_, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "member-2", types.TaskGrantRoleViewer)
	require.NoError(t, err)
	_, err = svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "member-3", types.TaskGrantRoleCollaborator)
	require.NoError(t, err)

	access, err := svc.ResolveTaskAccess(ctx, taskGrantOwner(), "s1")
	require.NoError(t, err)
	require.Equal(t, types.TaskAccessOwner, access.Role)
	require.Equal(t, "owner-1", access.OwnerID)
	require.True(t, types.TaskRoleCanRun(access.Role))
	require.True(t, types.TaskRoleCanManage(access.Role))

	access, err = svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "member-2"}, "s1")
	require.NoError(t, err)
	require.Equal(t, types.TaskAccessViewer, access.Role)
	require.False(t, types.TaskRoleCanRun(access.Role), "Viewer 不能运行（AC1）")
	require.False(t, types.TaskRoleCanManage(access.Role))

	access, err = svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "member-3"}, "s1")
	require.NoError(t, err)
	require.Equal(t, types.TaskAccessCollaborator, access.Role)
	require.True(t, types.TaskRoleCanRun(access.Role), "Collaborator 可以请求新运行")
	require.False(t, types.TaskRoleCanManage(access.Role), "Collaborator 不能扩额/审批（AC1）")

	access, err = svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "bystander"}, "s1")
	require.NoError(t, err)
	require.Equal(t, types.TaskAccessNone, access.Role)
	require.False(t, types.TaskRoleCanRun(access.Role))
}

func TestResolveTaskAccessCrossTenantIsUniform404(t *testing.T) {
	ctx := context.Background()
	svc := newTaskGrantServiceForTest(
		&stubTaskGrantRepo{}, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"},
		&stubTaskGrantMembers{},
	)
	// A tenant-2 caller probing tenant-1's task id sees the same 404 as an
	// unknown task id — the miss leaks nothing.
	_, err := svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 2, UserID: "owner-1"}, "s1")
	require.Equal(t, apperrors.ErrNotFound, errorCodeOf(t, err))
	_, err = svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 2, UserID: "owner-1"}, "missing")
	require.Equal(t, apperrors.ErrNotFound, errorCodeOf(t, err))
	_, err = svc.GrantTaskAccess(ctx, types.Caller{TenantID: 2, UserID: "owner-1"}, "s1", "member-2", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrNotFound, errorCodeOf(t, err))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/application/service/ -run 'TestGrantTaskAccess|TestRevokeAndListTaskGrants|TestResolveTaskAccess' -count=1`
Expected: FAIL（编译错误 `undefined: NewTaskGrantService`）。

- [ ] **Step 3: Write minimal implementation**

`internal/application/service/task_grant.go`：

```go
package service

// Task collaboration grants (T12, #42). The grant surface is OWNER-EXPLICIT:
// only sessions.user_id may assign or revoke per-task Viewer/Collaborator
// roles (CONTEXT.md 任务所有者：控制任务共享). This is deliberately narrower
// than the SP13 share-token surface (canShareSession admits Admin+); the two
// surfaces coexist — token links serve anonymous read-only snapshots, grants
// serve named-member roles.

import (
	"context"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TaskGrantStorePort is the persistence seam for task_grants.
type TaskGrantStorePort interface {
	UpsertGrant(ctx context.Context, tenantID uint64, taskID, granteeID string, role types.TaskGrantRole, grantedBy string) (types.TaskGrant, error)
	DeleteGrant(ctx context.Context, tenantID uint64, taskID, granteeID string) error
	ListGrants(ctx context.Context, tenantID uint64, taskID string) ([]types.TaskGrant, error)
	RoleForGrantee(ctx context.Context, tenantID uint64, taskID, granteeID string) (types.TaskGrantRole, bool, error)
	TaskOwnerID(ctx context.Context, tenantID uint64, taskID string) (string, error)
}

// TaskMemberLookupPort resolves same-tenant membership. Misses return
// (nil, nil) — the active-membership decision stays here.
type TaskMemberLookupPort interface {
	Get(ctx context.Context, userID string, tenantID uint64) (*types.TenantMember, error)
}

type TaskGrantService struct {
	grants   TaskGrantStorePort
	sessions interfaces.SessionRepository
	members  TaskMemberLookupPort
}

func NewTaskGrantService(
	grants TaskGrantStorePort,
	sessions interfaces.SessionRepository,
	members TaskMemberLookupPort,
) *TaskGrantService {
	return &TaskGrantService{grants: grants, sessions: sessions, members: members}
}

// loadTaskForGrantManagement loads the task and enforces the owner-only gate
// shared by grant/revoke/list. A miss is one uniform 404.
func (s *TaskGrantService) loadTaskForGrantManagement(
	ctx context.Context, caller types.Caller, taskID string,
) (*types.Session, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, apperrors.NewBadRequestError("task id is required")
	}
	if caller.TenantID == 0 || strings.TrimSpace(caller.UserID) == "" {
		return nil, apperrors.NewForbiddenError("authenticated tenant identity is required")
	}
	session, err := s.sessions.GetByID(ctx, caller.TenantID, taskID)
	if err != nil {
		return nil, apperrors.NewNotFoundError("task not found")
	}
	if strings.TrimSpace(session.UserID) == "" {
		// A task with no owner row cannot be shared — there is no authority
		// to anchor the grant to.
		return nil, apperrors.NewBadRequestError("task has no owner")
	}
	if session.UserID != caller.UserID {
		return nil, apperrors.NewForbiddenError("only the task owner may manage task access")
	}
	return session, nil
}

// activeMember refuses anything but an active same-tenant membership row.
func (s *TaskGrantService) activeMember(ctx context.Context, tenantID uint64, userID string) (*types.TenantMember, error) {
	if s.members == nil {
		return nil, apperrors.NewBadRequestError("grantee is not an active member of this tenant")
	}
	member, err := s.members.Get(ctx, userID, tenantID)
	if err != nil {
		return nil, err
	}
	if member == nil || member.Status != types.TenantMemberStatusActive {
		return nil, apperrors.NewBadRequestError("grantee is not an active member of this tenant")
	}
	return member, nil
}

// GrantTaskAccess assigns (or rewrites) one member's role on one task.
// Owner-only; the grantee must be an active member of the same tenant and
// must not be the owner themself; "owner" is not a grantable role.
func (s *TaskGrantService) GrantTaskAccess(
	ctx context.Context, caller types.Caller, taskID, granteeID string, role types.TaskGrantRole,
) (*types.TaskGrant, error) {
	granteeID = strings.TrimSpace(granteeID)
	if granteeID == "" {
		return nil, apperrors.NewBadRequestError("grantee_id is required")
	}
	if !role.IsValid() {
		return nil, apperrors.NewBadRequestError("role must be viewer or collaborator")
	}
	session, err := s.loadTaskForGrantManagement(ctx, caller, taskID)
	if err != nil {
		return nil, err
	}
	if granteeID == session.UserID {
		return nil, apperrors.NewBadRequestError("the task owner already holds every task permission")
	}
	if _, err := s.activeMember(ctx, caller.TenantID, granteeID); err != nil {
		return nil, err
	}
	return s.grants.UpsertGrant(ctx, caller.TenantID, session.ID, granteeID, role, caller.UserID)
}

// RevokeTaskAccess removes one grant. Owner-only; revoking an absent grant is
// a success as long as the task exists and the caller owns it.
func (s *TaskGrantService) RevokeTaskAccess(
	ctx context.Context, caller types.Caller, taskID, granteeID string,
) error {
	granteeID = strings.TrimSpace(granteeID)
	if granteeID == "" {
		return apperrors.NewBadRequestError("grantee_id is required")
	}
	session, err := s.loadTaskForGrantManagement(ctx, caller, taskID)
	if err != nil {
		return err
	}
	return s.grants.DeleteGrant(ctx, caller.TenantID, session.ID, granteeID)
}

// ListTaskGrants returns the task's grants. Owner-only.
func (s *TaskGrantService) ListTaskGrants(
	ctx context.Context, caller types.Caller, taskID string,
) ([]types.TaskGrant, error) {
	session, err := s.loadTaskForGrantManagement(ctx, caller, taskID)
	if err != nil {
		return nil, err
	}
	return s.grants.ListGrants(ctx, caller.TenantID, session.ID)
}

// ResolveTaskAccess derives one caller's full access to one task: owner
// (sessions.user_id), collaborator, viewer or none. A task miss — unknown or
// cross-tenant — is one uniform 404 so the probe learns nothing.
func (s *TaskGrantService) ResolveTaskAccess(
	ctx context.Context, caller types.Caller, taskID string,
) (types.TaskAccess, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return types.TaskAccess{}, apperrors.NewBadRequestError("task id is required")
	}
	if caller.TenantID == 0 {
		return types.TaskAccess{}, apperrors.NewNotFoundError("task not found")
	}
	session, err := s.sessions.GetByID(ctx, caller.TenantID, taskID)
	if err != nil {
		return types.TaskAccess{}, apperrors.NewNotFoundError("task not found")
	}
	access := types.TaskAccess{TaskID: session.ID, OwnerID: session.UserID, Role: types.TaskAccessNone}
	if strings.TrimSpace(caller.UserID) != "" && caller.UserID == session.UserID {
		access.Role = types.TaskAccessOwner
		return access, nil
	}
	if role, found, rErr := s.grants.RoleForGrantee(ctx, caller.TenantID, session.ID, caller.UserID); rErr != nil {
		return types.TaskAccess{}, rErr
	} else if found {
		access.GrantRole = role
		switch role {
		case types.TaskGrantRoleCollaborator:
			access.Role = types.TaskAccessCollaborator
		case types.TaskGrantRoleViewer:
			access.Role = types.TaskAccessViewer
		}
	}
	return access, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/application/service/ -run 'TestGrantTaskAccess|TestRevokeAndListTaskGrants|TestResolveTaskAccess' -count=1`
Expected: PASS（5 项测试全绿）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/task_grant.go internal/application/service/task_grant_test.go
git commit -m "feat(collaboration): owner-explicit task grant service with role separation (T12 #42)"
```

---

### Task 3: grants HTTP API（handler + 路由 + 容器装配）

**Files:**
- Create: `internal/handler/session/workbench_task_grants.go`
- Test: `internal/handler/session/workbench_task_grants_test.go`
- Modify: `internal/router/routes_workbench.go`（文件末尾追加 `RegisterWorkbenchTaskGrantRoutes`，不动既有函数）
- Modify: `internal/router/router.go:67`（params 区追加一行字段）与 `internal/router/router.go:372`（注册区追加一行）
- Modify: `internal/container/container.go:284`（`must(container.Provide(...))` 区追加一行）
- Modify: `internal/container/workbench.go:125`（追加 wiring 函数）

**Interfaces:**
- Consumes: Task 2 的 `*TaskGrantService`；#34 的 workbench tasks 路由树与 handler 模式（`WorkbenchTaskStateHandler`、`RegisterWorkbenchTaskStateRoutes`）；`rbacGuards.Viewer()` 与 `apiKeyGroup/apiKeyChat` 链。
- Produces:
  - `session.TaskGrantManager` 接口（Task 2 service 三个方法签名）
  - `session.NewWorkbenchTaskGrantsHandler(grants TaskGrantManager) *WorkbenchTaskGrantsHandler`
  - `(*WorkbenchTaskGrantsHandler).Grant(c *gin.Context)` / `.Revoke(c *gin.Context)` / `.List(c *gin.Context)`
  - `router.RegisterWorkbenchTaskGrantRoutes(r *gin.RouterGroup, h *session.WorkbenchTaskGrantsHandler, g *rbacGuards)`：POST `/workbench/tasks/:task_id/grants`、GET `/workbench/tasks/:task_id/grants`、DELETE `/workbench/tasks/:task_id/grants/:grantee_id`
  - wire 契约：POST body `{"grantee_id": string, "role": "viewer"|"collaborator"}` → 201 `{success:true,data:{grant}}`；DELETE → 200 `{success:true,data:{task_id,grantee_id,revoked:true}}`；GET → 200 `{success:true,data:{grants:[...]}}`

- [ ] **Step 1: Write the failing test**

创建 `internal/handler/session/workbench_task_grants_test.go`（照 `workbench_task_state_test.go` 的 handler 级模式）：

```go
package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeTaskGrantManager struct {
	grant     *types.TaskGrant
	grantErr  error
	revokeErr error
	list      []types.TaskGrant
	listErr   error
	lastGrant struct{ taskID, granteeID string; role types.TaskGrantRole }
	lastRevoke struct{ taskID, granteeID string }
}

func (f *fakeTaskGrantManager) GrantTaskAccess(
	_ context.Context, _ types.Caller, taskID, granteeID string, role types.TaskGrantRole,
) (*types.TaskGrant, error) {
	f.lastGrant.taskID, f.lastGrant.granteeID, f.lastGrant.role = taskID, granteeID, role
	if f.grantErr != nil {
		return nil, f.grantErr
	}
	return f.grant, nil
}

func (f *fakeTaskGrantManager) RevokeTaskAccess(
	_ context.Context, _ types.Caller, taskID, granteeID string,
) error {
	f.lastRevoke.taskID, f.lastRevoke.granteeID = taskID, granteeID
	return f.revokeErr
}

func (f *fakeTaskGrantManager) ListTaskGrants(
	_ context.Context, _ types.Caller, _ string,
) ([]types.TaskGrant, error) {
	return f.list, f.listErr
}

func taskGrantContext(t *testing.T, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := context.Background()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "owner-1")
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleContributor)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	c.Request = httptest.NewRequest(method, path, reader).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	return c, recorder
}

func TestTaskGrantsHandlerGrantMapsInputsAndErrors(t *testing.T) {
	manager := &fakeTaskGrantManager{grant: &types.TaskGrant{
		TenantID: 1, TaskID: "s1", GranteeID: "member-2",
		Role: types.TaskGrantRoleViewer, GrantedBy: "owner-1",
	}}
	handler := &WorkbenchTaskGrantsHandler{grants: manager}

	// Happy path: 201 + the persisted grant on the wire.
	c, recorder := taskGrantContext(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"member-2","role":"viewer"}`)
	c.Params = gin.Params{{Key: "task_id", Value: "s1"}}
	handler.Grant(c)
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"grantee_id":"member-2"`)
	require.Equal(t, "s1", manager.lastGrant.taskID)
	require.Equal(t, "member-2", manager.lastGrant.granteeID)
	require.Equal(t, types.TaskGrantRoleViewer, manager.lastGrant.role)
	// Identity comes from the context, never from the body.
	require.NotContains(t, recorder.Body.String(), "granted_by\":\"\"")

	// Malformed body / role: 400 before the service is reached.
	c, recorder = taskGrantContext(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"member-2","role":"owner"}`)
	c.Params = gin.Params{{Key: "task_id", Value: "s1"}}
	handler.Grant(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	c, recorder = taskGrantContext(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants", `not-json`)
	c.Params = gin.Params{{Key: "task_id", Value: "s1"}}
	handler.Grant(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	// Service 403/404 keep their AppError status; other errors are 500.
	for _, tc := range []struct {
		err  error
		code int
	}{
		{apperrors.NewForbiddenError("no"), http.StatusForbidden},
		{apperrors.NewNotFoundError("no"), http.StatusNotFound},
		{apperrors.NewBadRequestError("no"), http.StatusBadRequest},
	} {
		denied := &WorkbenchTaskGrantsHandler{grants: &fakeTaskGrantManager{grantErr: tc.err}}
		c, recorder = taskGrantContext(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
			`{"grantee_id":"member-2","role":"viewer"}`)
		c.Params = gin.Params{{Key: "task_id", Value: "s1"}}
		denied.Grant(c)
		require.Equal(t, tc.code, recorder.Code)
	}
}

func TestTaskGrantsHandlerRevokeAndList(t *testing.T) {
	manager := &fakeTaskGrantManager{}
	handler := &WorkbenchTaskGrantsHandler{grants: manager}

	c, recorder := taskGrantContext(t, http.MethodDelete, "/api/v1/workbench/tasks/s1/grants/member-2", "")
	c.Params = gin.Params{{Key: "task_id", Value: "s1"}, {Key: "grantee_id", Value: "member-2"}}
	handler.Revoke(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"revoked":true`)
	require.Equal(t, "member-2", manager.lastRevoke.granteeID)

	listed := &fakeTaskGrantManager{list: []types.TaskGrant{{
		TenantID: 1, TaskID: "s1", GranteeID: "member-2", Role: types.TaskGrantRoleCollaborator, GrantedBy: "owner-1",
	}}}
	listHandler := &WorkbenchTaskGrantsHandler{grants: listed}
	c, recorder = taskGrantContext(t, http.MethodGet, "/api/v1/workbench/tasks/s1/grants", "")
	c.Params = gin.Params{{Key: "task_id", Value: "s1"}}
	listHandler.List(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"role":"collaborator"`)

	// No identity: 401 without touching the service.
	bare := &WorkbenchTaskGrantsHandler{grants: manager}
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/tasks/s1/grants", nil)
	bare.List(c)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	// Unwired handler fails closed: 503, no route-mounted shims.
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	unwired := &WorkbenchTaskGrantsHandler{}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/tasks/s1/grants", nil)
	unwired.List(c)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/session/ -run 'TestTaskGrantsHandler' -count=1`
Expected: FAIL（编译错误 `undefined: WorkbenchTaskGrantsHandler`）。

- [ ] **Step 3: Write minimal implementation**

`internal/handler/session/workbench_task_grants.go`：

```go
package session

import (
	"context"
	"net/http"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// TaskGrantManager is the task-grant service port. Identity always arrives
// from the authenticated context; implementations bind every operation to
// the caller's tenant and the owner-only predicate.
type TaskGrantManager interface {
	GrantTaskAccess(ctx context.Context, caller types.Caller, taskID, granteeID string, role types.TaskGrantRole) (*types.TaskGrant, error)
	RevokeTaskAccess(ctx context.Context, caller types.Caller, taskID, granteeID string) error
	ListTaskGrants(ctx context.Context, caller types.Caller, taskID string) ([]types.TaskGrant, error)
}

// WorkbenchTaskGrantsHandler serves the per-task collaboration grants (T12).
// Same Viewer+/chat-capability route boundary as the other workbench lanes;
// the owner-only predicate lives in the service.
type WorkbenchTaskGrantsHandler struct {
	grants TaskGrantManager
}

func NewWorkbenchTaskGrantsHandler(grants TaskGrantManager) *WorkbenchTaskGrantsHandler {
	return &WorkbenchTaskGrantsHandler{grants: grants}
}

// taskGrantWriteError maps service AppErrors onto the workbench envelope.
func taskGrantWriteError(c *gin.Context, err error) {
	if appErr, ok := apperrors.IsAppError(err); ok {
		status := http.StatusInternalServerError
		switch appErr.Code {
		case apperrors.ErrBadRequest:
			status = http.StatusBadRequest
		case apperrors.ErrForbidden:
			status = http.StatusForbidden
		case apperrors.ErrNotFound:
			status = http.StatusNotFound
		}
		c.AbortWithStatusJSON(status, gin.H{"success": false, "error": appErr.Message})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "task grant operation failed"})
}

// taskGrantCaller derives the authenticated caller the same way the other
// workbench lanes do (resolveOwnedRun / workbench_task_state shape): tenant
// and user come only from the request context, never from the URL or body.
// An empty identity is refused before the service is reached.
func taskGrantCaller(c *gin.Context) (types.Caller, bool) {
	caller := types.CallerFromContext(c.Request.Context())
	if caller.TenantID == 0 || strings.TrimSpace(caller.UserID) == "" {
		return types.Caller{}, false
	}
	return caller.Normalize(), true
}

// Grant POST /workbench/tasks/:task_id/grants — assign or rewrite one
// member's role. grantee_id and role come from the body; identity never does.
func (h *WorkbenchTaskGrantsHandler) Grant(c *gin.Context) {
	if h == nil || h.grants == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	caller, ok := taskGrantCaller(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return
	}
	var input struct {
		GranteeID string `json:"grantee_id"`
		Role      string `json:"role"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "grantee_id and role are required"})
		return
	}
	role := types.TaskGrantRole(input.Role)
	if !role.IsValid() {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "role must be viewer or collaborator"})
		return
	}
	grant, err := h.grants.GrantTaskAccess(c.Request.Context(), caller, c.Param("task_id"), input.GranteeID, role)
	if err != nil {
		taskGrantWriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"grant": grant}})
}

// Revoke DELETE /workbench/tasks/:task_id/grants/:grantee_id — idempotent.
func (h *WorkbenchTaskGrantsHandler) Revoke(c *gin.Context) {
	if h == nil || h.grants == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	caller, ok := taskGrantCaller(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return
	}
	taskID := c.Param("task_id")
	granteeID := c.Param("grantee_id")
	if err := h.grants.RevokeTaskAccess(c.Request.Context(), caller, taskID, granteeID); err != nil {
		taskGrantWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"task_id": taskID, "grantee_id": granteeID, "revoked": true}})
}

// List GET /workbench/tasks/:task_id/grants — owner-only member list.
func (h *WorkbenchTaskGrantsHandler) List(c *gin.Context) {
	if h == nil || h.grants == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	caller, ok := taskGrantCaller(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return
	}
	grants, err := h.grants.ListTaskGrants(c.Request.Context(), caller, c.Param("task_id"))
	if err != nil {
		taskGrantWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"grants": grants}})
}
```

`internal/router/routes_workbench.go` 文件末尾追加：

```go
// RegisterWorkbenchTaskGrantRoutes exposes the per-task collaboration grants
// (T12). Same Viewer/API-key boundary as the other workbench lanes; the
// owner-only predicate lives in the service (a non-owner caller gets a 403,
// a cross-tenant probe a uniform 404). Wildcard names: the POST tree already
// binds :task_id via /workbench/tasks/:task_id/archive, and the DELETE tree
// likewise, so every route here reuses :task_id (gin requires identical
// wildcard names per verb tree).
func RegisterWorkbenchTaskGrantRoutes(r *gin.RouterGroup, h *session.WorkbenchTaskGrantsHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	tasks := g.apiKeyGroup(r.Group("/workbench/tasks", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	tasks.POST("/:task_id/grants", h.Grant)
	tasks.GET("/:task_id/grants", h.List)
	tasks.DELETE("/:task_id/grants/:grantee_id", h.Revoke)
}
```

`internal/router/router.go:67` 之后追加 params 字段（与 `WorkbenchTaskStateHandler` 相邻）：

```go
	WorkbenchTaskGrantsHandler   *session.WorkbenchTaskGrantsHandler   `optional:"true"`
```

`internal/router/router.go:372`（`RegisterWorkbenchTaskStateRoutes` 行后）追加：

```go
		RegisterWorkbenchTaskGrantRoutes(v1, params.WorkbenchTaskGrantsHandler, rbacGuards)
```

`internal/container/container.go:284`（`must(container.Provide(NewWorkbenchTaskStateHandler))` 行后）追加：

```go
	must(container.Provide(NewWorkbenchTaskGrantsHandler))
```

`internal/container/workbench.go:125`（`NewWorkbenchTaskStateHandler` 函数后）追加：

```go
// NewWorkbenchTaskGrantsHandler wires the task collaboration grants to the
// durable store; tenant/owner always come from the authenticated context.
func NewWorkbenchTaskGrantsHandler(
	grants *repository.TaskGrantStore,
	sessions interfaces.SessionRepository,
	members interfaces.TenantMemberRepository,
) *session.WorkbenchTaskGrantsHandler {
	return session.NewWorkbenchTaskGrantsHandler(service.NewTaskGrantService(grants, sessions, members))
}
```

（`internal/container/workbench.go` 头部已 import `repository`、`session`、`interfaces`；需补 `"github.com/Tencent/WeKnora/internal/application/service"`。）

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/handler/session/ -run 'TestTaskGrantsHandler' -count=1 && go build ./internal/router/ ./internal/container/`
Expected: PASS + BUILD_OK。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/session/workbench_task_grants.go internal/handler/session/workbench_task_grants_test.go internal/router/routes_workbench.go internal/router/router.go internal/container/container.go internal/container/workbench.go
git commit -m "feat(collaboration): workbench task grants API with owner-only wiring (T12 #42)"
```

---

### Task 4: 被授予者读取任务详情（granted read 通道）

**Files:**
- Modify: `internal/application/repository/agent_run.go`（`GetOwnedRun` 后追加 `GetRunForGrantedReader`，约 :113）
- Modify: `internal/handler/session/workbench_read.go`（接口区追加 `GrantedRunReader`、struct 追加字段、`WithGrantedRuns`、`resolveReadableRun`；`GetWorkbenchExecution`/`GetWorkbenchSnapshot` 换用 `resolveReadableRun`）
- Modify: `internal/container/workbench.go:22`（`NewWorkbenchReadHandler` 链上追加 `.WithGrantedRuns(runs)`——注意此处 `runs` 即 `*repository.AgentRunStore`，它实现该接口）
- Test: `internal/application/repository/task_grant_read_test.go`
- Test: `internal/handler/session/workbench_read_grants_test.go`

**Interfaces:**
- Consumes: Task 1 的 `task_grants` 表与 `types.TaskGrantRole`；#35 的 `resolveOwnedRun`/`GetWorkbenchSnapshot`/`taskFacts`（`internal/handler/session/workbench_read.go:123-197`）；`AgentRunStore.GetOwnedRun`（`internal/application/repository/agent_run.go:100`）。
- Produces:
  - `(*AgentRunStore).GetRunForGrantedReader(ctx context.Context, tenantID uint64, readerID, runID string) (agentruntime.Run, error)`——EXISTS 子查询同时要求 reader 持有 grant **且** 是同租户 active 成员（Review Focus 1）
  - `session.GrantedRunReader` 接口：`GetRunForGrantedReader(ctx context.Context, tenantID uint64, readerID, runID string) (agentruntime.Run, error)`
  - `(*WorkbenchReadHandler).WithGrantedRuns(reader GrantedRunReader) *WorkbenchReadHandler`（nil reader = fail closed，行为与现状逐字相同）
  - 行为契约：GET `/workbench/executions/:run_id` 与 `/:run_id/snapshot` 对 owner 照旧；对 grant 持有者（viewer/collaborator）200；对无 grant 成员 404。SSE `/events` 与 source-events 写回调**保持 owner-only**（`resolveOwnedRun` 不变——恢复流与写回调不因共享放宽）。

- [ ] **Step 1: Write the failing test**

创建 `internal/application/repository/task_grant_read_test.go`（package `repository_test`，复用 Task 1 的 `openTaskGrantDB`）：

```go
package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// taskGrantAdmission returns a minimal admitted run for task s1 owned by u1.
func taskGrantAdmission() agentruntime.Admission {
	return agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, SessionID: "s1", UserID: "u1",
		RequestID: "q1", AssistantMessageID: "a1", RequestHash: "hash-1",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	}
}

func TestGetRunForGrantedReaderRespectsGrantsAndMembership(t *testing.T) {
	db := openTaskGrantDB(t)
	grants := repository.NewTaskGrantStore(db)
	runs := repository.NewAgentRunStore(db)
	ctx := context.Background()
	_, err := runs.Admit(ctx, taskGrantAdmission())
	require.NoError(t, err)

	_, err = grants.UpsertGrant(ctx, 1, "s1", "u2", types.TaskGrantRoleViewer, "u1")
	require.NoError(t, err)
	_, err = grants.UpsertGrant(ctx, 1, "s1", "u5", types.TaskGrantRoleViewer, "u1")
	require.NoError(t, err, "u5 holds a grant but is suspended — row kept to prove the read converges")

	// A grant holder reads the run (viewer and collaborator alike).
	run, err := runs.GetRunForGrantedReader(ctx, 1, "u2", "r1")
	require.NoError(t, err)
	require.Equal(t, "r1", run.Key.RunID)
	require.Equal(t, "s1", run.SessionID)

	// The owner also resolves through the granted path (same predicate shape).
	_, err = runs.GetRunForGrantedReader(ctx, 1, "u1", "r1")
	require.ErrorIs(t, err, agentruntime.ErrNotFound, "the owner holds no grant row; owner reads stay on GetOwnedRun")

	// A bystander without a grant cannot.
	_, err = runs.GetRunForGrantedReader(ctx, 1, "u4", "r1")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)

	// A suspended member's grant stops resolving immediately.
	_, err = runs.GetRunForGrantedReader(ctx, 1, "u5", "r1")
	require.ErrorIs(t, err, agentruntime.ErrNotFound,
		"deactivated membership kills the granted read without touching the grant row")

	// Cross-tenant reader cannot.
	_, err = runs.GetRunForGrantedReader(ctx, 2, "u2", "r1")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)

	// Unknown run id is a plain miss.
	_, err = runs.GetRunForGrantedReader(ctx, 1, "u2", "missing")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)

	// Revoking the grant closes the read on the next request.
	require.NoError(t, grants.DeleteGrant(ctx, 1, "s1", "u2"))
	_, err = runs.GetRunForGrantedReader(ctx, 1, "u2", "r1")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
}
```

再创建 `internal/handler/session/workbench_read_grants_test.go`（package `session`，handler seam 单测）：

```go
package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeOwnedRunReader struct {
	run agentruntime.Run
	err error
}

func (f *fakeOwnedRunReader) GetOwnedRun(context.Context, uint64, string, string) (agentruntime.Run, error) {
	return f.run, f.err
}

type fakeGrantedRunReader struct {
	run agentruntime.Run
	err error
}

func (f *fakeGrantedRunReader) GetRunForGrantedReader(context.Context, uint64, string, string) (agentruntime.Run, error) {
	return f.run, f.err
}

// grantedSnapshots is the minimal snapshot reader for the seam tests: it
// returns one fixed execution snapshot for any key.
type grantedSnapshots struct{}

func (grantedSnapshots) ReadRunSnapshot(ctx context.Context, key agentruntime.RunKey) (workbench.ExecutionSnapshot, error) {
	return workbench.ExecutionSnapshot{Execution: workbench.ExecutionDTO{RunID: key.RunID}}, nil
}

func (grantedSnapshots) ReadRunEvents(ctx context.Context, key agentruntime.RunKey, cursor int64, limit int) ([]workbench.ExecutionEvent, int64, error) {
	return nil, 0, nil
}

func grantedReadContext(t *testing.T, userID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/executions/r1/snapshot", nil).WithContext(ctx)
	c.Params = gin.Params{{Key: "run_id", Value: "r1"}}
	return c, recorder
}

func TestGetWorkbenchSnapshotFallsBackToGrantedReader(t *testing.T) {
	run := agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, SessionID: "s1", UserID: "u1"}

	// Owner miss + grantee hit: the snapshot is served to the grantee.
	owned := &fakeOwnedRunReader{err: agentruntime.ErrNotFound}
	granted := &fakeGrantedRunReader{run: run}
	h := NewWorkbenchReadHandler(owned, grantedSnapshots{}).WithGrantedRuns(granted)
	c, recorder := grantedReadContext(t, "member-2")
	h.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	// No granted reader wired: fail closed, exactly today's behavior.
	hClosed := NewWorkbenchReadHandler(owned, grantedSnapshots{})
	c, recorder = grantedReadContext(t, "member-2")
	hClosed.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)

	// Granted reader wired but the reader holds no grant: still 404.
	hMiss := NewWorkbenchReadHandler(owned, grantedSnapshots{}).WithGrantedRuns(&fakeGrantedRunReader{err: agentruntime.ErrNotFound})
	c, recorder = grantedReadContext(t, "member-4")
	hMiss.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)

	// The owner keeps the direct path (no fallback involved).
	ownerOK := &fakeOwnedRunReader{run: run}
	hOwner := NewWorkbenchReadHandler(ownerOK, grantedSnapshots{}).WithGrantedRuns(&fakeGrantedRunReader{err: agentruntime.ErrNotFound})
	c, recorder = grantedReadContext(t, "u1")
	hOwner.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	// A repository error on the granted path is a 500, never a silent 404.
	hErr := NewWorkbenchReadHandler(owned, grantedSnapshots{}).WithGrantedRuns(&fakeGrantedRunReader{err: errors.New("db down")})
	c, recorder = grantedReadContext(t, "member-2")
	hErr.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
```

（import 列表：`context`、`errors`、`net/http`、`net/http/httptest`、`testing`、`agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"`、`"github.com/Tencent/WeKnora/internal/modules/workbench"`、`"github.com/Tencent/WeKnora/internal/types"`、`"github.com/gin-gonic/gin"`、`"github.com/stretchr/testify/require"`。）

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/application/repository/ -run 'TestGetRunForGrantedReader' -count=1 && go test ./internal/handler/session/ -run 'TestGetWorkbenchSnapshotFallsBack' -count=1`
Expected: FAIL（前者 `undefined: (*AgentRunStore).GetRunForGrantedReader`；后者 `undefined: (*WorkbenchReadHandler).WithGrantedRuns`）。

- [ ] **Step 3: Write minimal implementation**

`internal/application/repository/agent_run.go` 的 `GetOwnedRun` 之后追加：

```go
// GetRunForGrantedReader reads a run for a task-grant holder: a Viewer or
// Collaborator whose grant row exists for the run's task (= session, ADR-0004)
// AND whose tenant membership is still active. The membership join makes a
// suspended or removed member's read fail closed on the very next request,
// without needing a grant sweep. Owners keep using GetOwnedRun; the owner
// holds no grant row by construction. The complete predicate is one query so
// no unscoped run can leak between checks.
func (s *AgentRunStore) GetRunForGrantedReader(
	ctx context.Context, tenantID uint64, readerID, runID string,
) (agentruntime.Run, error) {
	if tenantID == 0 || readerID == "" || runID == "" {
		return agentruntime.Run{}, agentruntime.ErrNotFound
	}
	var row agentRunRow
	err := s.db.WithContext(ctx).Table("agent_runs").
		Where(`tenant_id = ? AND run_id = ? AND EXISTS (
			SELECT 1 FROM task_grants tg
			WHERE tg.tenant_id = agent_runs.tenant_id
			  AND tg.task_id = agent_runs.session_id
			  AND tg.grantee_id = ?
			  AND EXISTS (
				SELECT 1 FROM tenant_members tm
				WHERE tm.tenant_id = tg.tenant_id
				  AND tm.user_id = tg.grantee_id
				  AND tm.status = 'active'
				  AND tm.deleted_at IS NULL))`, tenantID, runID, readerID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agentruntime.Run{}, agentruntime.ErrNotFound
	}
	return row.view(), err
}
```

`internal/handler/session/workbench_read.go` 三处修改：

(a) 接口区（`OwnedTaskFactsReader` 之后）追加：

```go
// GrantedRunReader resolves a run for a task-grant holder (Viewer or
// Collaborator) after the strict owner predicate misses. Nil (not wired)
// keeps every read owner-only — the fail-closed default.
type GrantedRunReader interface {
	GetRunForGrantedReader(ctx context.Context, tenantID uint64, readerID, runID string) (agentruntime.Run, error)
}
```

(b) struct 与构造区：`WorkbenchTaskGrantsHandler` 无关；在 `WorkbenchReadHandler` struct 的 `taskFacts OwnedTaskFactsReader` 字段后追加 `granted GrantedRunReader`，并在 `WithTaskFacts` 之后追加：

```go
// WithGrantedRuns attaches the task-grant fallback for the task detail and
// snapshot reads (T12): a Viewer/Collaborator grant holder may READ the task
// while run submission, SSE recovery and source-event ingestion stay
// owner-only. Nil keeps the strict owner predicate.
func (h *WorkbenchReadHandler) WithGrantedRuns(reader GrantedRunReader) *WorkbenchReadHandler {
	h.granted = reader
	return h
}
```

(c) `resolveOwnedRun` 之后追加 `resolveReadableRun`，并把 `GetWorkbenchExecution`/`GetWorkbenchSnapshot` 开头的 `resolveOwnedRun(c)`/`h.owned(c)` 换成 `resolveReadableRun(c)`：

```go
// resolveReadableRun is the read-only ownership predicate: the strict owner
// first, then — only when a granted reader is wired — the task-grant
// fallback for Viewer/Collaborator holders. Write surfaces (source-event
// ingestion, SSE recovery) keep resolveOwnedRun directly. The fallback only
// engages when the owner predicate missed with a 404: a 401 or 500 already
// wrote its response and stands.
func (h *WorkbenchReadHandler) resolveReadableRun(c *gin.Context) (agentruntime.Run, bool) {
	run, ok := resolveOwnedRun(c, h.runs)
	if ok {
		return run, true
	}
	if h.granted == nil || c.Writer.Status() != http.StatusNotFound {
		return agentruntime.Run{}, false
	}
	tenantID, okT := types.TenantIDFromContext(c.Request.Context())
	if !okT || tenantID == 0 {
		if value, exists := c.Get(types.TenantIDContextKey.String()); exists {
			tenantID, _ = value.(uint64)
		}
	}
	readerID, okU := types.UserIDFromContext(c.Request.Context())
	if !okU || readerID == "" {
		if value, exists := c.Get(types.UserIDContextKey.String()); exists {
			readerID, _ = value.(string)
		}
	}
	if tenantID == 0 || readerID == "" {
		return agentruntime.Run{}, false
	}
	grantedRun, err := h.granted.GetRunForGrantedReader(c.Request.Context(), tenantID, readerID, strings.TrimSpace(c.Param("run_id")))
	if errors.Is(err, agentruntime.ErrNotFound) {
		return agentruntime.Run{}, false
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return agentruntime.Run{}, false
	}
	return grantedRun, true
}
```

两个端点的最终形态（唯一变化是第一行的解析调用）：

```go
func (h *WorkbenchReadHandler) GetWorkbenchExecution(c *gin.Context) {
	run, ok := h.resolveReadableRun(c)
	if !ok || h.snapshots == nil {
		return
	}
	snapshot, err := h.snapshots.ReadRunSnapshot(c.Request.Context(), run.Key)
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	writeWorkbenchJSON(c, snapshot.Execution)
}
```

```go
func (h *WorkbenchReadHandler) GetWorkbenchSnapshot(c *gin.Context) {
	run, ok := h.resolveReadableRun(c)
	if !ok || h.snapshots == nil {
		return
	}
	snapshot, err := h.snapshots.ReadRunSnapshot(c.Request.Context(), run.Key)
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	if h.taskFacts != nil {
		// Facts are scoped by the business owner (agent_runs.owner_id). run.Owner
		// is the lease owner: empty once settled, a worker id while leased — it
		// never matches the facts guard and would 404 every request.
		facts, factsErr := h.taskFacts.ReadTaskFactsForRun(c.Request.Context(), run.Key.TenantID, run.UserID, run.Key.RunID)
		if factsErr != nil {
			writeWorkbenchError(c, factsErr)
			return
		}
		task := workbench.TaskSnapshotFacts{TaskID: facts.TaskID, Title: facts.Title, Attention: facts.Attention, ArchivedAt: facts.ArchivedAt}
		if err := task.Validate(); err != nil {
			writeWorkbenchError(c, err)
			return
		}
		snapshot.Task = &task
	}
	writeWorkbenchJSON(c, snapshot)
}
```

（`StreamWorkbenchEvents` 与 `IngestWorkbenchSourceEvent` 保持 `h.owned(c)`/`resolveOwnedRun` 不变——SSE 恢复与写回调不因共享放宽。）

`internal/container/workbench.go:22-26`（`NewWorkbenchReadHandler`）链式追加：

```go
	return session.NewWorkbenchReadHandler(runs, snapshots, ingestor).WithTaskFacts(lists).WithGrantedRuns(runs)
```

（`runs *repository.AgentRunStore` 即 `GrantedRunReader` 实现。）

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/application/repository/ -run 'TestGetRunForGrantedReader' -count=1 && go test ./internal/handler/session/ -run 'TestGetWorkbenchSnapshotFallsBack|TestWorkbenchHTTPOwnership' -count=1 && go build ./internal/container/`
Expected: PASS + BUILD_OK（`TestWorkbenchHTTPOwnershipUsesRealStoreAndProjection` 必须仍然全绿——owner 面零回归）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/repository/agent_run.go internal/application/repository/task_grant_read_test.go internal/handler/session/workbench_read.go internal/handler/session/workbench_read_grants_test.go internal/container/workbench.go
git commit -m "feat(collaboration): granted task detail reads for viewer/collaborator with active-membership convergence (T12 #42)"
```

---

### Task 5: 扩额门禁——任务 owner 或 billing 授权

**Files:**
- Modify: `internal/handler/commercial_task_budget.go:24-76`（`ExtendTaskBudget` 增加门禁与 `isTaskRunOwner` 私有方法）
- Modify: `internal/router/routes_commercial.go:64`（budget/extend 路由移出 billing 写门禁组）
- Test: `internal/handler/commercial_task_budget_test.go`

**Interfaces:**
- Consumes: `commercial.CanManageBilling(role string, active, billingGrant bool) bool`（`internal/modules/commercial/access.go:6`）、`(*CommercialHandler).hasBillingGrant`、`commercialUserID`（`internal/handler/commercial.go:99-134`）；`agent_runs` 表（`owner_id` 为 run 的业务 owner）。
- Produces:
  - 门禁语义（wire 契约）：POST `/commercial/tasks/:id/budget/extend` 放行当且仅当 (a) 调用者有 billing 权（租户 owner 角色或 commercial_grants billing capability，既有语义不变）**或** (b) `:id` 命中的 run 属于调用者（`agent_runs.owner_id`，新增）；run 不存在时保持既有 404 `TASK_BUDGET_NOT_FOUND` 语义（先 billing 检查、再 run 存在性、最后 owner 谓词）；其余 403 `BUDGET_FORBIDDEN`。
  - `(*CommercialHandler).taskRunOwner(ctx context.Context, tenantID uint64, runID string) (ownerID string, found bool, err error)`（私有；参数化 SQL）。

- [ ] **Step 1: Write the failing test**

创建 `internal/handler/commercial_task_budget_test.go`（package `handler`；真实 sqlite 内存库 + 真实路由，复用 `appTenantScope` 的 context 注入模式）：

```go
package handler

// Budget-extension gate tests (T12 #42): raising a task budget admits the
// TASK OWNER or a billing-authorized caller — never a collaborator or
// bystander — and keeps the 404 TASK_BUDGET_NOT_FOUND contract for unknown
// runs. All on a real sqlite database through the real handler + route.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newBudgetGateDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	// The owner predicate only reads agent_runs(tenant_id, run_id, owner_id).
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_budget_accounts (
		tenant_id INTEGER PRIMARY KEY, verified_micro BIGINT NOT NULL, unreflected_micro BIGINT NOT NULL DEFAULT 0,
		held_micro BIGINT NOT NULL DEFAULT 0, refund_locked_micro BIGINT NOT NULL DEFAULT 0,
		verified_until DATETIME NOT NULL, version BIGINT NOT NULL DEFAULT 0)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_task_budgets (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, limit_micro BIGINT NOT NULL,
		spent_micro BIGINT NOT NULL DEFAULT 0, held_micro BIGINT NOT NULL DEFAULT 0,
		deadline DATETIME NOT NULL, version BIGINT NOT NULL DEFAULT 1,
		PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (7, 'r1', 'u1')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_budget_accounts
		(tenant_id, verified_micro, verified_until, version) VALUES (7, 100000000, ?, 0)`,
		time.Now().Add(time.Hour).UTC()).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, limit_micro, deadline, version) VALUES (7, 'r1', 1000, ?, 1)`,
		time.Now().Add(time.Hour).UTC()).Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func postBudgetExtend(t *testing.T, db *gorm.DB, role, userID, runID, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewCommercialHandler(db)
	r := gin.New()
	r.POST("/commercial/tasks/:id/budget/extend", h.ExtendTaskBudget)
	body := `{"additional_credits": 10, "idempotency_key": "` + idempotencyKey + `"}`
	req := httptest.NewRequest(http.MethodPost, "/commercial/tasks/"+runID+"/budget/extend", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	if role != "" {
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(role))
	}
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestExtendTaskBudgetAdmitsTaskOwner(t *testing.T) {
	db := newBudgetGateDB(t)
	// The task owner is a plain contributor: no tenant-owner role, no billing
	// grant. Before T12 this caller was 403'd by the route-level billing gate.
	w := postBudgetExtend(t, db, "contributor", "u1", "r1", "k-owner-1")
	require.Equal(t, http.StatusOK, w.Code, "task owner may raise their own budget: %s", w.Body.String())

	// Idempotent replay of the same key stays a success.
	w = postBudgetExtend(t, db, "contributor", "u1", "r1", "k-owner-1")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestExtendTaskBudgetRejectsCollaboratorAndBystander(t *testing.T) {
	db := newBudgetGateDB(t)
	// The owner applies an idempotency key FIRST, so the replay below hits an
	// ALREADY-APPLIED key: the role gate must still refuse the collaborator
	// (gate order: role before idempotency).
	w := postBudgetExtend(t, db, "contributor", "u1", "r1", "k-owner-1")
	require.Equal(t, http.StatusOK, w.Code, "owner applies the key first: %s", w.Body.String())

	// The collaborator (contributor role, not the run owner) replaying the
	// owner's applied key is still refused with the role-gate 403 — never the
	// idempotent 200 a legitimate replayer would see.
	w = postBudgetExtend(t, db, "contributor", "u2", "r1", "k-owner-1")
	require.Equal(t, http.StatusForbidden, w.Code, "collaborator replaying an applied key is refused")
	require.Contains(t, w.Body.String(), "BUDGET_FORBIDDEN")

	// A viewer member is refused too.
	w = postBudgetExtend(t, db, "viewer", "u3", "r1", "k-fresh-1")
	require.Equal(t, http.StatusForbidden, w.Code)

	// An admin without a billing grant is refused (administrative role alone
	// never grants purchase authority — unchanged semantics).
	w = postBudgetExtend(t, db, "admin", "u4", "r1", "k-fresh-2")
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestExtendTaskBudgetBillingGrantAndNotFoundKeepContract(t *testing.T) {
	db := newBudgetGateDB(t)
	// Billing authority (tenant owner role) keeps the existing admission.
	w := postBudgetExtend(t, db, "owner", "boss", "r1", "k-boss-1")
	require.Equal(t, http.StatusOK, w.Code, "billing-authorized caller keeps admission: %s", w.Body.String())

	// Unknown run keeps the explicit 404 TASK_BUDGET_NOT_FOUND (no 403 leak
	// of run existence to a non-owner).
	w := postBudgetExtend(t, db, "contributor", "u2", "missing", "k-fresh-3")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "TASK_BUDGET_NOT_FOUND")

	// Cross-tenant run is equally a 404.
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (8, 'r8', 'u1')`).Error)
	w = postBudgetExtend(t, db, "contributor", "u1", "r8", "k-fresh-4")
	require.Equal(t, http.StatusNotFound, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run 'TestExtendTaskBudget' -count=1`
Expected: FAIL——`TestExtendTaskBudgetAdmitsTaskOwner` 403（当前路由组 billing 门禁挡住 contributor；本测试直接挂 handler，故实际是 handler 无门禁直接 200？**注意**：直接挂 handler 时路由组中间件不存在，当前 handler 对任何 caller 都 200，因此 `TestExtendTaskBudgetRejectsCollaboratorAndBystander` FAIL（期望 403 实得 200）。这是本任务的 RED 信号：先跑、记录真实失败形态。

Run 输出确认：`TestExtendTaskBudgetRejectsCollaboratorAndBystander` 与 `TestExtendTaskBudgetBillingGrantAndNotFoundKeepContract` 中 404 用例 FAIL（200），其余可能通过。以实际输出为准。

- [ ] **Step 3: Write minimal implementation**

`internal/router/routes_commercial.go:64` —— 把 budget/extend 一行从 `commercialGroup`（带 `RequireManageBillingForWrites`）移到独立的、只保留 commercial capability 门禁的子组（保持注释链）：

```go
	// W05 → T12 (#42): raising one task run's budget admits the TASK OWNER
	// as well as billing authority (CONTEXT.md 任务预算：只有任务所有者或获
	// 授权的账单管理员可以增加上限). The owner arm lives inside the handler
	// (it needs the run row), so this route leaves the group-level billing
	// write gate and keeps only the explicit commercial capability gate —
	// the same direct-on-parent shape the platform refund review uses below.
	budgetGroup := r.Group("/commercial", commercialHandler.RequireExplicitCommercialCapability())
	budgetGroup.POST("/tasks/:id/budget/extend", commercialHandler.ExtendTaskBudget)
```

（原 `commercialGroup` 内的 `commercialGroup.POST("/tasks/:id/budget/extend", ...)` 行删除，其上的 W05 注释随行迁移。）

`internal/handler/commercial_task_budget.go` —— 在 `runID := c.Param("id")` 之后、`svc.Extend` 之前插入门禁，并在文件末尾追加 `taskRunOwner`：

```go
	runID := c.Param("id")
	// T12 (#42): a budget raise admits the TASK OWNER or a billing-authorized
	// caller (CONTEXT.md 任务预算). The gate runs BEFORE the service so a
	// collaborator is refused ahead of any idempotency replay. found=false
	// (unknown or cross-tenant run) deliberately falls through so the service
	// keeps its explicit TASK_BUDGET_NOT_FOUND 404 contract.
	if !commercial.CanManageBilling(role, true, h.hasBillingGrant(c, tenantID)) {
		ownerID, found, ownerErr := h.taskRunOwner(c.Request.Context(), tenantID, runID)
		if ownerErr != nil {
			appFail(c, http.StatusInternalServerError, "BUDGET_OWNER_LOOKUP_FAILED", "failed to resolve the task owner")
			return
		}
		if found && ownerID != userID {
			appFail(c, http.StatusForbidden, "BUDGET_FORBIDDEN",
				"raising a task budget requires the task owner or billing authority")
			return
		}
	}
	if err := svc.Extend(c.Request.Context(), tenantID, runID, input.IdempotencyKey,
```

（`commercialTenantScope` 目前返回 `(tenantID, role, ok)`——现有代码 `tenantID, _, ok := commercialTenantScope(c)` 改为 `tenantID, role, ok := commercialTenantScope(c)`；`userID := commercialUserID(c)`。）

文件末尾追加：

```go
// taskRunOwner resolves the business owner of one run (agent_runs.owner_id)
// inside the tenant. found=false covers unknown and cross-tenant runs alike.
func (h *CommercialHandler) taskRunOwner(ctx context.Context, tenantID uint64, runID string) (string, bool, error) {
	if h == nil || h.db == nil || tenantID == 0 || runID == "" {
		return "", false, nil
	}
	var ownerID string
	err := h.db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND run_id = ?", tenantID, runID).
		Select("owner_id").Scan(&ownerID).Error
	if err != nil {
		return "", false, err
	}
	if ownerID == "" {
		return "", false, nil
	}
	return ownerID, true, nil
}
```

（import 补 `"context"`；`commercial` 模块已在 import 中——`"github.com/Tencent/WeKnora/internal/modules/commercial"` 已存在于该文件。）

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/handler/ -run 'TestExtendTaskBudget' -count=1 && go build ./internal/router/`
Expected: PASS + BUILD_OK。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/commercial_task_budget.go internal/router/routes_commercial.go internal/handler/commercial_task_budget_test.go
git commit -m "feat(collaboration): budget extension admits the task owner beside billing authority (T12 #42)"
```

---

### Task 6: 个人连接与副作用审批门禁

**Files:**
- Modify: `internal/handler/app_connector_action.go`（`PrepareAction` 加个人连接谓词；`ApproveAction` 加审批者谓词）
- Test: `internal/handler/app_connector_action_grant_test.go`

**Interfaces:**
- Consumes: `appconnector.ConnectionKindPersonal`（`internal/modules/appconnector/model.go:25`）、`ConnectionRow.OwnerID`（`internal/modules/appconnector/repository/appconnector/install.go:56`）、`appconnector.CanDriveActionWrites(role string) bool`（`internal/modules/appconnector/access.go:34`）；既有 oc 测试脚手架 `newOCProductEngine`/`ocDo`/`ocActionDetail`（`internal/handler/app_connector_oc_test.go:216-370`，seed 的 `conn-gh` 是 `user-a` 的个人连接、`conn-gl` 是空间连接）。
- Produces（wire 契约）:
  - POST `/apps/actions/prepare`：连接为 personal 且 `conn.OwnerID != caller.UserID` → 403 `NOT_CONNECTION_OWNER`（与 `BeginOCAuthorization` 的既有谓词同形；空间连接不受影响）。
  - POST `/apps/actions/:id/approve`：审批者放行当且仅当 `row.ActorID == caller`（发起者）或（个人连接时）`conn.OwnerID == caller` 或 `CanDriveActionWrites(role)`（租户 owner/admin 保留既有审批能力，authorized actor 语义）；其余 403 `ACTION_APPROVAL_FORBIDDEN`。

- [ ] **Step 1: Write the failing test**

创建 `internal/handler/app_connector_action_grant_test.go`（package `handler`，复用 oc 脚手架——脚手架在 `app_connector_oc_test.go`，同包可用）：

```go
package handler

// Personal-connection and approval gates (T12 #42): a collaborator (or any
// other member) may neither USE the owner's personal connection to prepare
// an action nor APPROVE an action prepared by the owner. Space connections
// keep their existing shape; tenant owner/admin keep approval authority
// (authorized actor). Runs on the real OC engine fixture: sqlite + real
// routes + real action service.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareActionRejectsAnotherMembersPersonalConnection(t *testing.T) {
	// One engine for the whole scenario: the fixture seeds each row exactly
	// once, so the env must not be rebuilt between requests.
	env := newOCProductEngine(t)
	prepare := func(user, connectionID string) (*httptest.ResponseRecorder, string) {
		body := fmt.Sprintf(`{"connection_id":%q,"target":"t1","risk":"write","content":"{\"target\":\"t1\"}"}`, connectionID)
		w := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/prepare", body, "X-Test-User", user)
		return w, w.Body.String()
	}

	// conn-gh is user-a's PERSONAL connection. user-b (an admin in the test
	// fixture — the strongest non-owner case) is refused.
	w, body := prepare("user-b", "conn-gh")
	require.Equal(t, http.StatusForbidden, w.Code, body)
	require.Contains(t, body, "NOT_CONNECTION_OWNER")

	// The connection owner still prepares fine.
	w, body = prepare("user-a", "conn-gh")
	require.Equal(t, http.StatusCreated, w.Code, body)

	// A SPACE connection (conn-gl, owner_id='') is NOT affected: any
	// action-write-capable role keeps the existing shape.
	w, body = prepare("user-b", "conn-gl")
	require.Equal(t, http.StatusCreated, w.Code, body)
}

func TestApproveActionRejectsCollaboratorAndAdmitsOwner(t *testing.T) {
	env := newOCProductEngine(t)
	// user-a prepares on their personal connection.
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/prepare",
		`{"connection_id":"conn-gh","target":"t1","risk":"write","content":"{\"target\":\"t1\"}"}`,
		"X-Test-User", "user-a")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	id, digest, _, _, version := ocActionDetail(t, w.Body.String())

	approveBody := fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, digest, int(version))

	// A different member in the collaborator shape (contributor role) cannot
	// approve the owner's action — sharing never delegates side-effect
	// approval (AC1).
	w = ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/approve", approveBody,
		"X-Test-User", "user-b", "X-Test-Role", "contributor")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "ACTION_APPROVAL_FORBIDDEN")

	// The actor (owner) approves their own action.
	w = ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/approve", approveBody,
		"X-Test-User", "user-a", "X-Test-Role", "contributor")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// A tenant admin keeps the authorized-actor approval arm (existing
	// capability preserved).
	w = ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/prepare",
		`{"connection_id":"conn-gh","target":"t2","risk":"send","content":"{\"target\":\"t2\"}"}`,
		"X-Test-User", "user-a")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	id2, digest2, _, _, version2 := ocActionDetail(t, w.Body.String())
	w = ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id2+"/approve",
		fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, digest2, int(version2)),
		"X-Test-User", "user-b", "X-Test-Role", "admin")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run 'TestPrepareActionRejects|TestApproveActionRejects' -count=1`
Expected: FAIL——personal-connection prepare 用例得 201（无谓词），collaborator approve 得 200（无谓词）。记录真实失败输出。

- [ ] **Step 3: Write minimal implementation**

`internal/handler/app_connector_action.go` 两处：

(a) `PrepareAction` 中 `First(&conn)` 成功之后、`if h.actions == nil` 之前插入：

```go
	// T12 (#42): a personal connection is its owner's identity — no member,
	// however privileged, may drive actions through it (CONTEXT.md 代码平台
	// 连接：个人连接只能由其所有者使用). Same predicate shape as
	// BeginOCAuthorization.
	if conn.Kind == appconnector.ConnectionKindPersonal && conn.OwnerID != userID {
		appFail(c, http.StatusForbidden, "NOT_CONNECTION_OWNER",
			"a personal connection may only be used by its owner")
		return
	}
```

（import 已含 `appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"`——确认文件头；若无需补。）

(b) `ApproveAction` 中 `row, ok := h.appActionByID(c, tenantID, c.Param("id"))` 之后、fence 检查之前插入（并把函数开头的 `tenantID, _, userID, ok := appTenantScope(c)` 改为 `tenantID, role, userID, ok := appTenantScope(c)`）：

```go
	// T12 (#42): approval authority for an action follows the actor, the
	// personal connection's owner, or a tenant owner/admin (authorized
	// actor). Sharing a task never delegates approval of its owner's side
	// effects (CONTEXT.md 任务协作者：不会授予批准其外部副作用的权限).
	var conn appconnectorrepo.ConnectionRow
	connFound := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, row.ConnectionID).
		First(&conn).Error == nil
	personalOwner := connFound && conn.Kind == appconnector.ConnectionKindPersonal && conn.OwnerID == userID
	if row.ActorID != userID && !personalOwner && !appconnector.CanDriveActionWrites(role) {
		appFail(c, http.StatusForbidden, "ACTION_APPROVAL_FORBIDDEN",
			"approving this action requires its initiator, the personal connection owner, or tenant owner/admin")
		return
	}
```

（`appconnectorrepo` 已在该文件 import 中——`PrepareAction` 现有代码即以 `appconnectorrepo.ConnectionRow` 加载连接。）

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/handler/ -run 'TestPrepareActionRejects|TestApproveActionRejects|TestOC' -count=1`
Expected: PASS（新测试 + 既有 OC 矩阵全绿——`TestOC*` 证明个人连接谓词没有破坏 OC 面：OC prepare 默认以连接所有者 `user-a` 调用）。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/app_connector_action.go internal/handler/app_connector_action_grant_test.go
git commit -m "feat(collaboration): personal-connection use and approval gates on /apps/actions (T12 #42)"
```

---

### Task 7: 端到端验收——三条 AC 的 wire 级剧本

**Files:**
- Test: `internal/application/repository/task_collaboration_http_test.go`（AC1 扩额/读面 + AC2 + AC3 证据；真实 sqlite 迁移 + 真实 store/service/handler + gin + httptest）
- Test: `internal/handler/session/task_collaboration_run_test.go`（AC1"Viewer/Collaborator 不能运行"：真实 QA handler 走真实 owner-scope 谓词）

**Interfaces:**
- Consumes: Task 1-6 全部产出（`NewTaskGrantStore`、`NewTaskGrantService`、`NewWorkbenchTaskGrantsHandler`、`AgentRunStore.GetRunForGrantedReader`、`WithGrantedRuns`、`NewCommercialHandler` 门禁、action 谓词）；#34 的 `NewWorkbenchListStore`/`NewWorkbenchListHandler`、`NewAgentRunStore.Admit`；`repository.NewSessionRepository`（`internal/application/repository/session.go:29`）。
- Produces: 验收证据测试（无新生产接口）。AC3 的"最高稳定 Interface"即 HTTP wire：gin 路由 + 真实 handler + 真实 service + 真实迁移库，无 mock。blocked-env 项：无——三条 AC 全部本地可验证；`/apps/actions` 谓词的 wire 级证据在 Task 6（同为真库真路由），本任务引用不重复搭建。

- [ ] **Step 1: Write the tests**

`internal/application/repository/task_collaboration_http_test.go`：

```go
package repository_test

// End-to-end collaboration evidence (T12 #42). Everything here runs through
// real HTTP handlers over a fully migrated sqlite database: grants API,
// granted task reads, budget extension gate and the owner-scoped list. This
// is the AC3 evidence — no mocked service or hand-written projection.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type taskCollabEnv struct {
	db     *gorm.DB
	engine *gin.Engine
}

func newTaskCollabEnv(t *testing.T) *taskCollabEnv {
	t.Helper()
	db := openTaskGrantDB(t)
	runs := repository.NewAgentRunStore(db)
	_, err := runs.Admit(context.Background(), taskGrantAdmission())
	require.NoError(t, err)

	// Budget fixtures so the extension path reaches its terminal state.
	require.NoError(t, db.Exec(`INSERT INTO commercial_budget_accounts
		(tenant_id, verified_micro, verified_until, version) VALUES (1, 100000000, ?, 0)`,
		time.Now().Add(time.Hour).UTC()).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, limit_micro, deadline, version) VALUES (1, 'r1', 1000, ?, 1)`,
		time.Now().Add(time.Hour).UTC()).Error)

	grantsSvc := service.NewTaskGrantService(
		repository.NewTaskGrantStore(db),
		repository.NewSessionRepository(db),
		repository.NewTenantMemberRepository(db),
	)
	grantsHandler := session.NewWorkbenchTaskGrantsHandler(grantsSvc)
	readHandler := session.NewWorkbenchReadHandler(runs, repository.NewAgentRunSnapshotRepository(db)).WithGrantedRuns(runs)
	listHandler := session.NewWorkbenchListHandler(repository.NewWorkbenchListStore(db))
	commercialHandler := handler.NewCommercialHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.POST("/workbench/tasks/:task_id/grants", grantsHandler.Grant)
	v1.GET("/workbench/tasks/:task_id/grants", grantsHandler.List)
	v1.DELETE("/workbench/tasks/:task_id/grants/:grantee_id", grantsHandler.Revoke)
	v1.GET("/workbench/executions", listHandler.ListWorkbenchExecutions)
	v1.GET("/workbench/executions/:run_id", readHandler.GetWorkbenchExecution)
	v1.GET("/workbench/executions/:run_id/snapshot", readHandler.GetWorkbenchSnapshot)
	v1.POST("/commercial/tasks/:id/budget/extend", commercialHandler.ExtendTaskBudget)
	return &taskCollabEnv{db: db, engine: r}
}

func (e *taskCollabEnv) do(t *testing.T, method, path, body, userID string, tenant uint64, role types.TenantRole) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, tenant)
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func TestTaskCollaborationEndToEndAC1(t *testing.T) {
	env := newTaskCollabEnv(t)

	// The owner explicitly grants viewer(u2) and collaborator(u3).
	w := env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u2","role":"viewer"}`, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w = env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u3","role":"collaborator"}`, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// AC1 gate: the collaborator cannot raise the budget; the owner can.
	w = env.do(t, http.MethodPost, "/api/v1/commercial/tasks/r1/budget/extend",
		`{"additional_credits":10,"idempotency_key":"e2e-k1"}`, "u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusForbidden, w.Code, "Collaborator 不能扩额（AC1）: %s", w.Body.String())
	w = env.do(t, http.MethodPost, "/api/v1/commercial/tasks/r1/budget/extend",
		`{"additional_credits":10,"idempotency_key":"e2e-k1"}`, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, "任务 owner 可以扩额: %s", w.Body.String())

	// Read separation: viewer and collaborator read the task snapshot.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/snapshot", "", "u2", 1, types.TenantRoleViewer)
	require.Equal(t, http.StatusOK, w.Code, "Viewer 可读任务详情: %s", w.Body.String())
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1", "", "u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, "Collaborator 可读任务详情: %s", w.Body.String())

	// Grants management stays owner-only end to end.
	w = env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u4","role":"viewer"}`, "u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusForbidden, w.Code, "grants 管理仅 owner: %s", w.Body.String())
}

func TestTaskCollaborationEndToEndAC2SharedDoesNotWidenVisibility(t *testing.T) {
	env := newTaskCollabEnv(t)

	w := env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u2","role":"viewer"}`, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// The grantee's workbench list stays empty: a shared task does not widen
	// the list's owner scope (AC2).
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions", "", "u2", 1, types.TenantRoleViewer)
	require.Equal(t, http.StatusOK, w.Code)
	var listEnvelope struct {
		Data struct {
			Items []json.RawMessage `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listEnvelope))
	require.Empty(t, listEnvelope.Data.Items, "grantee 列表不含 owner 任务（AC2）")

	// The owner's list still shows the task.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions", "", "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listEnvelope))
	require.Len(t, listEnvelope.Data.Items, 1)

	// A bystander without a grant cannot read the task at all.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/snapshot", "", "u4", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusNotFound, w.Code, "无 grant 成员默认不可见（私有任务）")

	// Cross-tenant probe is a uniform 404.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/snapshot", "", "u2", 2, types.TenantRoleViewer)
	require.Equal(t, http.StatusNotFound, w.Code)
	w = env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u2","role":"viewer"}`, "u1", 2, types.TenantRoleOwner)
	require.Equal(t, http.StatusNotFound, w.Code)

	// Revoking closes the read on the very next request (no caching).
	w = env.do(t, http.MethodDelete, "/api/v1/workbench/tasks/s1/grants/u2", "", "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/snapshot", "", "u2", 1, types.TenantRoleViewer)
	require.Equal(t, http.StatusNotFound, w.Code, "撤销立即生效")
}
```

（`taskCollabEnv.db` 的类型写 `*gorm.DB`——补 import `"gorm.io/gorm"`，`fmt` 若未用到则从 import 去掉。）

`internal/handler/session/task_collaboration_run_test.go`（package `session`——只有这里能构造 `&Handler{sessionService: ...}`）：

```go
package session

// "Viewer 不能运行"的 wire 级回归（T12 #42 AC1）：运行入口（agent-chat）的
// owner-scope 谓词必须不因 task grant 放宽——viewer、collaborator、无 grant
// 成员与跨租户调用者的 POST /agent-chat 全部 404。真实 sqlite 迁移库 +
// 真实 SessionRepository 支撑的 SessionService stub + 真实 QA handler。

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// runGateSessions is a SessionService stub whose GetOwnedSession delegates to
// the REAL session repository — the production owner predicate (session.go
// GetOwnedSession → sessionRepo.Get, tenant+user scoped).
type runGateSessions struct {
	interfaces.SessionService
	repo interfaces.SessionRepository
}

func (s *runGateSessions) GetOwnedSession(ctx context.Context, id string) (*types.Session, error) {
	tenantID, _ := types.TenantIDFromContext(ctx)
	userID, _ := types.UserIDFromContext(ctx)
	return s.repo.Get(ctx, tenantID, userID, id)
}

func TestAgentQARunGateStaysOwnerScopedForGrantHolders(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "run-gate.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })

	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (1, 't1', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'task-1', 'u1', 'builtin')`).Error)
	// Seed REAL grant rows so u2/u3 are genuine viewer/collaborator holders:
	// the run gate must stay owner-scoped even for granted members (AC1).
	// The migrated track (full migrations/sqlite Up above) already created
	// task_grants.
	require.NoError(t, db.Exec(`INSERT INTO task_grants (tenant_id, task_id, grantee_id, role, granted_by) VALUES (1, 's1', 'u2', 'viewer', 'u1')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO task_grants (tenant_id, task_id, grantee_id, role, granted_by) VALUES (1, 's1', 'u3', 'collaborator', 'u1')`).Error)

	h := &Handler{sessionService: &runGateSessions{repo: repository.NewSessionRepository(db)}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/agent-chat/:session_id", h.AgentQA)

	post := func(userID string, tenant uint64) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/agent-chat/s1", strings.NewReader(`{"query":"hi"}`))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), types.TenantIDContextKey, tenant)
		ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// Viewer grant holder (u2), collaborator grant holder (u3), grant-less
	// member (u4) and cross-tenant caller: the run surface stays owner-scoped
	// (404) — grants never widen the run channel.
	for _, userID := range []string{"u2", "u3", "u4"} {
		require.Equal(t, http.StatusNotFound, post(userID, 1).Code,
			"POST /agent-chat 对非 owner 一律 404（Viewer 不能运行，AC1）user=%s", userID)
	}
	require.Equal(t, http.StatusNotFound, post("u2", 2).Code, "跨租户 404")
}
```

- [ ] **Step 2: Run the tests**

这是验收整合步骤（Task 1-6 已实现对应行为），预期 PASS；任何 FAIL 都指向前置任务的真实缺陷，回到对应任务修复后重跑。

Run: `go test ./internal/application/repository/ -run 'TestTaskCollaborationEndToEnd' -count=1 && go test ./internal/handler/session/ -run 'TestAgentQARunGateStaysOwnerScoped' -count=1`
Expected: PASS。若 FAIL：按失败断言回到对应任务修实现（不得改断言迁就实现）。

- [ ] **Step 3: Run the full plan verification**

Run（worktree 根，覆盖本计划全部测试 + 既有回归）：

```bash
go build ./internal/... && go test ./internal/application/repository/ -run 'TaskGrant|TaskCollaboration|GrantedReader|WorkbenchHTTP' -count=1 && go test ./internal/application/service/ -run 'TaskGrant|ShareSession|UnshareSession|GetSharedSession' -count=1 && go test ./internal/handler/session/ -run 'TaskGrantsHandler|GetWorkbenchSnapshotFallsBack|AgentQARunGate|WorkbenchTaskState|WorkbenchRead' -count=1 && go test ./internal/handler/ -run 'TestExtendTaskBudget|TestPrepareActionRejects|TestApproveActionRejects|TestOC' -count=1
```

Expected: 全部 PASS（`WorkbenchHTTP*`/`WorkbenchTaskState*`/`WorkbenchRead*`/`TestOC*` 为既有回归，证明零破坏）。

- [ ] **Step 4: Commit**

```bash
git add internal/application/repository/task_collaboration_http_test.go internal/handler/session/task_collaboration_run_test.go
git commit -m "test(collaboration): end-to-end wire evidence for owner/collaborator/viewer separation (T12 #42)"
```

---

## Consumes-Produces 总表（供后续计划）

**Consumes（本计划使用的前批接口）**：#34 的 `WorkbenchListStore`/`WorkbenchTaskStateStore` 模式、workbench tasks 路由树、迁移双序列尾部；#35 的 `WorkbenchReadHandler`/`resolveOwnedRun`/`GetWorkbenchSnapshot`/taskFacts 通道。

**Produces（后续计划消费）**：
- `types.TaskGrantRole`/`TaskAccessRole`/`TaskRoleCanRun`/`TaskRoleCanManage`/`TaskGrant`/`TaskAccess`
- `repository.NewTaskGrantStore`（UpsertGrant/DeleteGrant/ListGrants/RoleForGrantee/TaskOwnerID）+ `ErrTaskGrantTaskNotFound`
- `repository.(*AgentRunStore).GetRunForGrantedReader`
- `service.NewTaskGrantService(...)`：GrantTaskAccess/RevokeTaskAccess/ListTaskGrants/ResolveTaskAccess（**#36/#37 接运行入口时用 `ResolveTaskAccess` + `TaskRoleCanRun` 决定 collaborator 放行**；#46 Material 面用同一解析扩读面；#43 合规访问、#53 空间连接团队归因消费 grants 数据）
- `session.NewWorkbenchTaskGrantsHandler` + `RegisterWorkbenchTaskGrantRoutes`（POST/GET/DELETE `/workbench/tasks/:task_id/grants[...]`）
- `session.(*WorkbenchReadHandler).WithGrantedRuns(GrantedRunReader)`
- 扩额门禁语义（task owner OR billing）与 `/apps/actions` 个人连接/审批谓词

## 验收对照

| 验收标准 | 证据 |
|---|---|
| Viewer 不能运行 | `TestAgentQARunGateStaysOwnerScopedForGrantHolders`（wire 级：viewer/collaborator/无 grant/跨租户 POST /agent-chat 全 404）+ `types.TaskRoleCanRun` 单元语义（Task 2） |
| Collaborator 不能扩额 | `TestTaskCollaborationEndToEndAC1`（collaborator 403 / owner 200，真实库）+ `TestExtendTaskBudgetRejectsCollaboratorAndBystander`（含幂等键重放仍 403） |
| Collaborator 不能用 Owner 个人连接 | `TestPrepareActionRejectsAnotherMembersPersonalConnection`（wire 级 403 NOT_CONNECTION_OWNER；空间连接不受影响） |
| Collaborator 不能审批其副作用 | `TestApproveActionRejectsCollaboratorAndAdmitsOwner`（wire 级：contributor 403、发起者 200、admin 保留 authorized-actor 200） |
| 共享资源不自动扩大 Task 可见范围 | `TestTaskCollaborationEndToEndAC2SharedDoesNotWidenVisibility`（grantee 列表为空、owner 列表 1 项、无 grant 成员 404、撤销立即 404） |
| 端到端最高稳定 Interface 验证 | 上述全部测试均为「真实 sqlite 迁移库 + 真实 store/service + 真 gin 路由 + httptest」的 HTTP wire 级；无 mock 服务、无手写投影。无 blocked-env 验收项（三条 AC 全部本地可验证；`/apps/actions` 的外部 provider 不被触达——谓词在 dispatch 之前拒绝，审批测试止步于审批记录，不伪造外部副作用） |
