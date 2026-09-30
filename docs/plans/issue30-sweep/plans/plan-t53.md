# Issue #53（T23: GitHub 空间连接与团队归因）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地「代码平台连接」的团队归因与空间授权模型——补齐空间连接（Tenant GitHub App 形态）的 grant 存储与 A02 裁决接线、暴露交付记录的发起者归因，并用真实全链 HTTP 端到端证据证明「个人与空间连接不能互相替代」「Collaborator 不能继承 Owner 个人连接」。

**Architecture:** 全部改动收敛在既有深模块的既有接缝上：`appconnector.CanUseConnection`（`internal/modules/appconnector/access.go:7`）是 AC1/AC2 的权威谓词，本计划不改其编码；新增 `SpaceConnectionGrantStore`（`app_space_connection_grants` 表）实现既有 `appconnectorsvc.SpaceGrantSource` 接口并替换 `internal/container/container.go:985` 的 `nil`（今日空间连接因 nil-grants fail closed，这正是 What to build「按成员角色授权」的核心缺口）；`codedelivery.DeliveryView` 补发起者字段（批准者/远端身份已在）；端到端证据走本仓库最高稳定 Interface——真实全量迁移 sqlite + 真实 handler/service/store + gin httptest，GitHub 远端用 HTTP 替身（外部 provider 边界，真实 GitHub 属 blocked-env opt-in）。

**Tech Stack:** Go（go.mod 模块 `github.com/Tencent/WeKnora`）、gin、gorm、golang-migrate（versioned/sqlite 双轨道）、sqlite（`github.com/mattn/go-sqlite3`）、testify。

**Spec:** `docs/plans/issue30-sweep/issues/issue-53.md`（验收标准原文）；领域事实源 `CONTEXT.md:224`（代码平台连接：个人连接只能由其所有者使用；空间连接按仓库、成员角色和操作策略授权。每次远端写入记录发起成员、批准成员与实际远端身份）、`CONTEXT.md:194-200`（私有任务/任务所有者/任务协作者）；ADR-0004（Task = Session）。

## 调查差异记录（以代码现状为准）

调查摘要与 worktree HEAD（`d52270a0f`）代码现状的偏差，实施者以此节为准：

1. **调查称 #52「absent」、归因「零实现」——过时。** `internal/modules/codedelivery/` 完整存在：`DeliveryRow.OwnerID`（发起者，`repository/codedelivery/store.go:25`）、`DeliveryView.Approver`（批准者，`service.go:392`，由 `viewOf` 经 `LatestApproverForAction` join `app_action_approvals`）、`DeliveryView.RemoteLogin`（远端身份，`service.go:388`）均已落地。真实缺口收窄为：**`DeliveryView` 未暴露发起者**（`viewOf`（`service.go:403-427`）没有把 `row.OwnerID` 投影出去）→ Task 1。
2. **调查称「谓词未接入 GitHub 交付端到端路径」——部分成立。** `CanUseConnection` 谓词与 `codedelivery.authorize`（`service.go:433-447`，交付硬编码 personal-only）已有单元级覆盖；但 `internal/handler/session/workbench_delivery_test.go` 的服务层是 `deliveryServiceStub`（mock）——正是 AC3 所指「mock 不冒充真实集成证据」的形态。真实全链 e2e 缺失 → Task 4。
3. **空间连接授权模型的真实缺口：** `SpaceGrantSource` 接口已声明（`internal/modules/appconnector/service/appconnector/oc_authorizer.go:21-24`），`NewOCSubjectGuard` 支持全量接线，但生产装配传 `nil`——`internal/container/container.go:980-986` 注释原文："No space-grant store exists yet, so space connections keep failing closed (the authorizer's nil-grant semantics)"。全仓库无任何 `SpaceConnectionGranted` 的持久化实现 → Task 2（存储+裁决）与 Task 3（管理端点+接线）。
4. **迁移轨道损坏实测复现**（2026-09-26，本 worktree）：`go test ./internal/database/` FAIL，错误 `failed to create sqlite migrate instance: failed to open source, "file://migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql`。`migrations/sqlite/000114_mobile_device_app.*` 与 `000114_public_agent_marketplace.*` 同号；`migrations/versioned/000193_mobile_device_app.*` 与 `000193_public_agent_marketplace.*` 同号。marketplace 迁移被生产代码钉死（`internal/database/migration.go:123` `os.Stat("migrations/sqlite/000114_public_agent_marketplace.up.sql")` + `:30` `sqliteAdoptionFKRelaxationMigrationVersion = 114`），不可移动；mobile_device_app 仅被 5 个测试文件中的 7 处文件名引用钉住 → Task 0 顺延 mobile_device_app。

## Global Constraints

以下逐字引自批准需求与领域事实源，每个任务隐含全部条目：

- 「个人与空间连接不能互相替代。」（issue-53.md 验收标准第 1 条）
- 「Collaborator 不能继承 Owner 个人连接。」（issue-53.md 验收标准第 2 条）
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（issue-53.md 验收标准第 3 条）
- 「代码平台连接：由 WeKnora 管理、用于访问 GitHub 或 GitLab 的个人连接或空间连接。个人连接只能由其所有者使用；空间连接按仓库、成员角色和操作策略授权。每次远端写入记录发起成员、批准成员与实际远端身份。」（CONTEXT.md:224）
- 「任务协作者（Task Collaborator）：……协作权不会转移任务所有权，也不会授予使用任务所有者个人连接或批准其外部副作用的权限。」（CONTEXT.md:200）
- 真实 GitHub 属外部凭据验收，本环境无凭据：既有门控 `WEKNORA_GITHUB_TEST_TOKEN` / `WEKNORA_GITHUB_TEST_REPO`（`internal/modules/codedelivery/github_real_test.go:15-21`）保持 opt-in skip，不得伪造通过。
- 谓词冻结边界：`appconnector.CanUseConnection` / `CanInstallInstallation` / `CanManageConnections` / `CanDriveActionWrites`（`internal/modules/appconnector/access.go`）的行为与编码一律不改——本计划只消费它们。
- 参数绑定纪律：所有新增 SQL 经 gorm 参数化查询（`Where("tenant_id = ? AND ...", ...)`），禁止字符串拼接。
- 并行批次纪律：改动收敛到本计划独有文件；共享文件（`container.go`、`router.go`、`routes_app_connectors.go`）只做单点插入，位置在计划中精确定位。

## Review Focus

规范隐含但单测最易漏掉的五类输入，按杀伤力排序（每行的钉住测试在括号内）：

1. **grant 行撤销后的下一次使用**：授权每次实时查表，撤销即刻收敛——oc_authorizer 不缓存任何正向结论（`oc_authorizer.go:96-99` 契约注释）。（Task 2 `TestSpaceGrantAuthorizerGrantAdmitsAndRevokeConverges`）
2. **对 personal 连接误授 space grant**：grant 行对 personal 连接必须零效果——`oc_authorizer.go:148-150` 只在 `Kind == space` 时读 grant，`CanUseConnection` 的 personal 分支只认 owner。（Task 2 `TestSpaceGrantNeverOpensPersonalConnection`；Task 3 管理端点直接拒绝对 personal 连接建 grant）
3. **跨租户 grant 行探测**：grant 表 PK 含 `tenant_id`，authorizer 的 Check 以 subject.TenantID 绑定，另一租户的 grant 行不可达。（Task 2 `TestSpaceGrantAuthorizerCrossTenantGrantUnreachable`；Task 4 e2e 跨租户 404）
4. **迁移重编号后的旧库升级/回滚路径**：mobile_device_app 顺延后，down 迁移与既有 down 测试必须仍逐文件可跑。（Task 0 的 `go test ./internal/application/repository/ -run TestMobileDevice` 全绿 + `go test ./internal/database/` 全绿）
5. **归因面泄漏凭据**：`DeliveryView` 只允许 initiator/approver/remote_login 三类归因 + 交付元数据，token/credential 字段结构性不存在。（Task 4 e2e `require.NotContains(t, w.Body.String(), deliveryToken)`；`writeDeliveryError`（`workbench_delivery.go:208-245`）已保证错误面无上游文本——不改）

---

## 任务总览

| Task | 交付物 | 依赖 |
|---|---|---|
| 0 | 迁移去重（mobile_device_app 顺延 000197/000118），全量迁移轨道恢复可装载 | 无 |
| 1 | `DeliveryView.Initiator` 发起者归因 | 无 |
| 2 | `SpaceConnectionGrantStore` + 迁移 000198/000119 + A02 authorizer 真实裁决集成测试 | Task 0（全量迁移） |
| 3 | 空间连接 grant 管理 HTTP 端点 + router/container 生产接线 | Task 2 |
| 4 | 端到端证据：真实全链 httptest（AC1/AC2/AC3 + 归因三元组） | Task 0、1、2 |

---

### Task 0: 迁移去重——mobile_device_app 顺延（前置修复）

**Files:**
- Modify（重命名）: `migrations/versioned/000193_mobile_device_app.up.sql` → `migrations/versioned/000197_mobile_device_app.up.sql`
- Modify（重命名）: `migrations/versioned/000193_mobile_device_app.down.sql` → `migrations/versioned/000197_mobile_device_app.down.sql`
- Modify（重命名）: `migrations/sqlite/000114_mobile_device_app.up.sql` → `migrations/sqlite/000118_mobile_device_app.up.sql`
- Modify（重命名）: `migrations/sqlite/000114_mobile_device_app.down.sql` → `migrations/sqlite/000118_mobile_device_app.down.sql`
- Modify: `internal/handler/mobile_device_test.go:33`
- Modify: `internal/application/repository/mobile_device_test.go:31`
- Modify: `internal/application/repository/mobile_push_isolation_test.go:67`
- Modify: `internal/application/repository/mobile_device_app_test.go:37,188,189`
- Modify: `internal/modules/workbench/service/workbench/notification_app_policy_test.go:50`

**Interfaces:**
- Consumes: 现状（已核实）——`migrations/versioned/` 现有最大序号 000196（code_deliveries），`migrations/sqlite/` 现有最大序号 000117（code_deliveries）；`000197`/`000118` 均空闲。
- Produces: 可装载的全量迁移轨道（`go test ./internal/database/` 全绿）；本计划后续任务的端到端测试依赖此轨道。迁移 DDL 零变化，仅文件名与文件内注释。

**方向论证（为什么顺延 mobile_device_app 而不是 marketplace）：**
- `internal/database/migration.go:123` 用 `os.Stat("migrations/sqlite/000114_public_agent_marketplace.up.sql")` 探测 sqlite 特判文件，`:30` 把 `sqliteAdoptionFKRelaxationMigrationVersion` 硬编码为 `114`（该文件自带 PRAGMA+事务，必须 NoTxWrap 跑）——移动 marketplace 需要改生产逻辑。
- mobile_device_app 的引用全部是测试文件（grep 实测 7 处，见上 Files），无生产代码钉住。
- 语义顺序安全：mobile_device_app 只 `ALTER TABLE mobile_devices / mobile_notification_intents`（两表由更早迁移创建），放到 000197/000118 不产生新依赖；且 000197 > 000194/195/196、000118 > 000115/116/117，golang-migrate 按版本号排序后它仍在下游。

- [ ] **Step 1: git mv 四个迁移文件**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep
git mv migrations/versioned/000193_mobile_device_app.up.sql migrations/versioned/000197_mobile_device_app.up.sql
git mv migrations/versioned/000193_mobile_device_app.down.sql migrations/versioned/000197_mobile_device_app.down.sql
git mv migrations/sqlite/000114_mobile_device_app.up.sql migrations/sqlite/000118_mobile_device_app.up.sql
git mv migrations/sqlite/000114_mobile_device_app.down.sql migrations/sqlite/000118_mobile_device_app.down.sql
```

- [ ] **Step 2: 同步迁移文件头注释里的自引用版本号（四个文件逐一核对，已实测）**

实测的注释自引用全集（其余文件无版本号自引用）：

- `migrations/versioned/000197_mobile_device_app.up.sql`（原 000193）行 1：`（同 sqlite 000114
-- 语义；PostgreSQL 具名约束直接 ALTER）。…` → 把注释里的 `sqlite 000114` 改为 `sqlite 000118`。
- `migrations/versioned/000197_mobile_device_app.down.sql`（原 000193）行 1：`-- 与 sqlite 000114 down 对称（review round 1 修复）：…` → `000114` 改为 `000118`；行 3：`…直接失败，回滚卡死在 000193。` → `000193` 改为 `000197`。
- `migrations/sqlite/000118_mobile_device_app.up.sql`（原 000114）：首行注释为 T37 描述（"app_id 进入 mobile_devices 主键…"），无版本号自引用——不改。
- `migrations/sqlite/000118_mobile_device_app.down.sql`（原 000114）：首行注释引用的是 `000058/000059+000060` 形状，无本迁移版本号自引用——不改。

全部只改注释，DDL 一字不动。检查命令（Step 2 自检）：

```bash
grep -rn "000114\|000193" migrations/versioned/000197_mobile_device_app.up.sql migrations/versioned/000197_mobile_device_app.down.sql migrations/sqlite/000118_mobile_device_app.up.sql migrations/sqlite/000118_mobile_device_app.down.sql
```

预期：无任何 `000114`/`000193` 残留（退出码 1）。

- [ ] **Step 3: 更新 7 处测试引用（逐字替换）**

| 文件 | 行 | 旧 | 新 |
|---|---|---|---|
| `internal/handler/mobile_device_test.go` | 33 | `"000114_mobile_device_app.up.sql",` | `"000118_mobile_device_app.up.sql",` |
| `internal/application/repository/mobile_device_test.go` | 31 | `"000114_mobile_device_app.up.sql",` | `"000118_mobile_device_app.up.sql",` |
| `internal/application/repository/mobile_push_isolation_test.go` | 67 | `"migrations/sqlite/000114_mobile_device_app.up.sql",` | `"migrations/sqlite/000118_mobile_device_app.up.sql",` |
| `internal/application/repository/mobile_device_app_test.go` | 37 | `"migrations/sqlite/000114_mobile_device_app.up.sql",` | `"migrations/sqlite/000118_mobile_device_app.up.sql",` |
| `internal/application/repository/mobile_device_app_test.go` | 188 | `{file: "migrations/sqlite/000114_mobile_device_app.down.sql", deviceRebuild: "CREATE TABLE mobile_devices_rebuilt"},` | `{file: "migrations/sqlite/000118_mobile_device_app.down.sql", deviceRebuild: "CREATE TABLE mobile_devices_rebuilt"},` |
| `internal/application/repository/mobile_device_app_test.go` | 189 | `{file: "migrations/versioned/000193_mobile_device_app.down.sql", deviceRebuild: "ADD PRIMARY KEY (tenant_id, owner_id, device_id, environment)"},` | `{file: "migrations/versioned/000197_mobile_device_app.down.sql", deviceRebuild: "ADD PRIMARY KEY (tenant_id, owner_id, device_id, environment)"},` |
| `internal/modules/workbench/service/workbench/notification_app_policy_test.go` | 50 | `"migrations/sqlite/000114_mobile_device_app.up.sql",` | `"migrations/sqlite/000118_mobile_device_app.up.sql",` |

替换后全仓库确认无残留引用：

```bash
grep -rn "000114_mobile_device_app\|000193_mobile_device_app" --include="*.go" . ; grep -rn "000114_mobile_device_app\|000193_mobile_device_app" migrations/
```

预期：两个 grep 均无输出（退出码 1）。

- [ ] **Step 4: 运行迁移轨道与受影响测试**

```bash
go build ./... \
&& go test ./internal/database/ -count=1 \
&& go test ./internal/application/repository/ -run 'TestMobileDevice|TestValidateMobileAppID|TestBindIsolatesOfficialAndEnterprise|TestTokenExclusivityIsPerApp|TestNotificationIntentFanOutPerApp|TestClaimJoinsAppID|TestMobilePush' -count=1 \
&& go test ./internal/handler/ -run 'TestMobileDeviceHandler' -count=1 \
&& go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicy|TestHTTPNotificationProvider|TestAppRouting|TestDisabledNotificationProvider|TestDisallowed' -count=1
```

（`-run` 列表逐一覆盖全部 7 处引用所在文件的实际测试名——已逐文件核实：`internal/application/repository/mobile_device_test.go` 与 `mobile_push_isolation_test.go` 的测试均以 `TestMobileDevice`/`TestMobilePush` 开头（后者为 `TestMobilePushIsolationOfficialAndEnterpriseNeverMix`、`TestMobilePushDisabledKeepsIntentsDurable`）；`mobile_device_app_test.go` 的 `TestValidateMobileAppID`/`TestBindIsolatesOfficialAndEnterprise`/`TestTokenExclusivityIsPerApp`/`TestNotificationIntentFanOutPerApp`/`TestClaimJoinsAppIDSoRevokedAppDoesNotResurrect`/`TestMobileDeviceAppDownMigrationsDeleteEnterpriseRows`；`notification_app_policy_test.go` 的 `TestPushPayloadPolicyBlindStripsKind`、`TestHTTPNotificationProviderBlindOmitsKind`、`TestAppRoutingProviderDispatchesByApp`、`TestDisabledNotificationProviderPausesDurablyWithoutRetryStorm`、`TestAppRoutingProviderRevokesOnlyOwnAppRegistration`、`TestDisallowedPushEndpointHostRejectsNonPublicTargets`、`TestAppRoutingConfiguredKeepsLiveRouteAliveAcrossDisabledPause`、`TestAppRoutingConfiguredRequiresAnyLiveChannel`（`TestAppRouting` 前缀同时命中 Provider/Configured 两族）。文件名引用替换错误会让这些测试在运行时 fatal，因此必须真正命中而非被 `-run` 过滤掉。）

预期：全部 PASS（迁移轨不再是 `duplicate migration file`）。

- [ ] **Step 5: Commit**

```bash
git add migrations/ internal/handler/mobile_device_test.go internal/application/repository/mobile_device_test.go internal/application/repository/mobile_push_isolation_test.go internal/application/repository/mobile_device_app_test.go internal/modules/workbench/service/workbench/notification_app_policy_test.go
git commit -m "fix(migrations): renumber mobile_device_app to 000197/000118 — dedupe with public_agent_marketplace (#53 task 0)"
```

---

### Task 1: DeliveryView 暴露发起者归因（initiator）

**Files:**
- Modify: `internal/modules/codedelivery/service.go:377-397`（DeliveryView 结构体）与 `:403-427`（viewOf）
- Test: `internal/modules/codedelivery/service_prepare_test.go`（文件末尾追加一个测试函数）

**Interfaces:**
- Consumes: `DeliveryRow.OwnerID string`（`repository/codedelivery/store.go:25`，prepare 时已写入 `in.CallerID`，见 `service.go:240`）。
- Produces: `codedelivery.DeliveryView` 新增字段 `Initiator string`（json tag `"initiator"`）——Task 4 e2e 断言 `initiator == "u1"`。wire 契约：GET `/workbench/executions/:run_id/delivery` 响应 `data.delivery.initiator`。

- [ ] **Step 1: 写失败测试（追加到 `internal/modules/codedelivery/service_prepare_test.go` 文件末尾）**

```go
// TestDeliveryViewCarriesInitiatorAttribution pins the initiator leg of the
// traceability triple (CONTEXT.md 代码平台连接: 每次远端写入记录发起成员、
// 批准成员与实际远端身份). Approver/RemoteLogin already surface; the row's
// OwnerID must reach the read face too.
func TestDeliveryViewCarriesInitiatorAttribution(t *testing.T) {
	f := newDeliveryFixture(t, nil)

	view, err := f.svc.PrepareDelivery(context.Background(), prepareInput())

	require.NoError(t, err)
	require.Equal(t, "u1", view.Initiator,
		"交付读面必须携带发起者（DeliveryRow.OwnerID 在 prepare 时已落库）")
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/modules/codedelivery/ -run TestDeliveryViewCarriesInitiatorAttribution -count=1
```

预期：FAIL——编译错误 `view.Initiator undefined (type codedelivery.DeliveryView has no field or method Initiator)`（Go 中字段缺失即编译失败，这是本步的 RED 形态）。

- [ ] **Step 3: 最小实现（`internal/modules/codedelivery/service.go` 两处）**

(a) `DeliveryView` 结构体（`:377` 起）在 `RunID` 字段后插入一行（其余字段与既有 json tag 全部不动）：

```go
// DeliveryView is the wire/read projection carrying the full traceability
// chain: approval anchor (action/digest/approver) + remote receipts.
type DeliveryView struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	RunID       string `json:"run_id"`
	Initiator   string `json:"initiator"`
	State       string `json:"state"`
```

(b) `viewOf`（`:403` 起）的字面量第一行补 `Initiator: row.OwnerID`：

```go
	view := DeliveryView{
		ID: row.ID, TaskID: row.TaskID, RunID: row.RunID, Initiator: row.OwnerID,
		State: row.State,
```

- [ ] **Step 4: 运行确认通过 + 包级回归**

```bash
go test ./internal/modules/codedelivery/ -count=1
```

预期：全部 PASS（含既有 service_prepare/service_dispatch/github_wire 套件；`internal/handler/session/workbench_delivery_test.go` 的 `deliveryServiceStub.lastView` 走零值 Initiator，不受影响）。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/service.go internal/modules/codedelivery/service_prepare_test.go
git commit -m "feat(codedelivery): surface the delivery initiator on the read face (#53 task 1)"
```

---

### Task 2: 空间连接 grant 存储 + A02 裁决真实接线

**Files:**
- Create: `migrations/versioned/000198_space_connection_grants.up.sql`
- Create: `migrations/versioned/000198_space_connection_grants.down.sql`
- Create: `migrations/sqlite/000119_space_connection_grants.up.sql`
- Create: `migrations/sqlite/000119_space_connection_grants.down.sql`
- Create: `internal/modules/appconnector/repository/appconnector/space_grant.go`
- Test: `internal/modules/appconnector/repository/appconnector/space_grant_test.go`
- Test: `internal/modules/appconnector/service/appconnector/oc_space_grant_test.go`

**Interfaces:**
- Consumes: `appconnectorsvc.SpaceGrantSource` 接口（`oc_authorizer.go:21-24`）——`SpaceConnectionGranted(ctx context.Context, tenantID uint64, connectionID, actorID string) (bool, error)`；`NewOCAuthorizer(src, installations, grants, bindings)`（`oc_authorizer.go:51`）；`Check(ctx, subject appconn.OCSubject, connectionID string, expectedVersion int64) (appconn.OCBinding, error)`；包级哨兵 `ErrConnectionForbidden`（`credentials.go:56`）；同包既有测试 helper `seedOCSubjectFixture(t, db)`（`oc_authorizer_test.go:116`，种 tenant 7 + alice/bob + `c-personal`/`c-space`，auth_version 2）、`dbInstallationSource`（`oc_authorizer_test.go:36`）、`newResolverFixture(t)`（`credentials_test.go:24`，返回 `(resolver, db)` 且已 AutoMigrate `MCPOAuthBindingRow/AppVersion/InstallationRow/ConnectionRow/TenantMember/MCPOAuthToken`）。
- Produces: `appconnectorrepo.NewSpaceConnectionGrantStore(db *gorm.DB) *SpaceConnectionGrantStore`，方法：
  - `GrantSpaceConnection(ctx context.Context, tenantID uint64, connectionID, actorID, grantedBy string) error`（幂等 upsert）
  - `RevokeSpaceConnection(ctx context.Context, tenantID uint64, connectionID, actorID string) error`（幂等 delete）
  - `ListSpaceConnectionGrants(ctx context.Context, tenantID uint64, connectionID string) ([]SpaceConnectionGrantRow, error)`
  - `SpaceConnectionGranted(ctx context.Context, tenantID uint64, connectionID, actorID string) (bool, error)`——结构化满足 `SpaceGrantSource`
  - `SpaceConnectionGrantRow` 类型（表 `app_space_connection_grants`）
  Task 3 的 handler 与 container 接线消费上述签名。

- [ ] **Step 1: 写四份迁移文件**

`migrations/versioned/000198_space_connection_grants.up.sql`：

```sql
-- T23 (#53): explicit per-actor grants on SPACE connections. CONTEXT.md
-- 代码平台连接: 个人连接只能由其所有者使用；空间连接按仓库、成员角色和
-- 操作策略授权。Personal connections never carry a row here — the A02
-- authorizer only consults this table for Kind='space' connections.
CREATE TABLE app_space_connection_grants (
    tenant_id     BIGINT       NOT NULL,
    connection_id VARCHAR(64)  NOT NULL,
    actor_id      VARCHAR(512) NOT NULL,
    granted_by    VARCHAR(512) NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, connection_id, actor_id)
);
```

`migrations/versioned/000198_space_connection_grants.down.sql`：

```sql
DROP TABLE IF EXISTS app_space_connection_grants;
```

`migrations/sqlite/000119_space_connection_grants.up.sql`：

```sql
-- T23 (#53): sqlite twin of versioned 000198 (per-actor grants on SPACE
-- connections). See the versioned file for the policy rationale.
CREATE TABLE app_space_connection_grants (
    tenant_id     INTEGER      NOT NULL,
    connection_id VARCHAR(64)  NOT NULL,
    actor_id      VARCHAR(512) NOT NULL,
    granted_by    VARCHAR(512) NOT NULL DEFAULT '',
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, connection_id, actor_id)
);
```

`migrations/sqlite/000119_space_connection_grants.down.sql`：

```sql
DROP TABLE IF EXISTS app_space_connection_grants;
```

- [ ] **Step 2: 写 store 失败测试 `internal/modules/appconnector/repository/appconnector/space_grant_test.go`（整个新文件）**

```go
package appconnector

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openSpaceGrantDB opens a REAL fully-migrated sqlite database (the full
// migrations/sqlite track, same shape as the task-grant suite's
// openTaskGrantDB) — this pins the migration↔projection alignment for
// app_space_connection_grants, not just the gorm model.
func openSpaceGrantDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "space-grants.db") + "?_foreign_keys=on&_busy_timeout=5000"
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
	return db
}

func TestSpaceConnectionGrantStoreUpsertListRevoke(t *testing.T) {
	db := openSpaceGrantDB(t)
	store := NewSpaceConnectionGrantStore(db)
	ctx := context.Background()

	require.NoError(t, store.GrantSpaceConnection(ctx, 7, "conn-space", "bob", "alice"))
	require.NoError(t, store.GrantSpaceConnection(ctx, 7, "conn-space", "carol", "alice"))
	// Upsert is idempotent: re-granting bob rewrites granted_by, never duplicates.
	require.NoError(t, store.GrantSpaceConnection(ctx, 7, "conn-space", "bob", "alice"))

	grants, err := store.ListSpaceConnectionGrants(ctx, 7, "conn-space")
	require.NoError(t, err)
	require.Len(t, grants, 2, "同一 (tenant, connection, actor) 只能有一行")

	granted, err := store.SpaceConnectionGranted(ctx, 7, "conn-space", "bob")
	require.NoError(t, err)
	require.True(t, granted)

	// Tenant scope is part of every read: tenant 8's rows are unreachable.
	granted, err = store.SpaceConnectionGranted(ctx, 8, "conn-space", "bob")
	require.NoError(t, err)
	require.False(t, granted, "跨租户 grant 行不可达")

	// Revoke is idempotent and immediately converges.
	require.NoError(t, store.RevokeSpaceConnection(ctx, 7, "conn-space", "bob"))
	require.NoError(t, store.RevokeSpaceConnection(ctx, 7, "conn-space", "bob"), "重复撤销是成功")
	granted, err = store.SpaceConnectionGranted(ctx, 7, "conn-space", "bob")
	require.NoError(t, err)
	require.False(t, granted, "撤销后下一次判定立即为 false")

	grants, err = store.ListSpaceConnectionGrants(ctx, 7, "conn-space")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, "carol", grants[0].ActorID)
	require.Equal(t, "alice", grants[0].GrantedBy)
}
```

- [ ] **Step 3: 运行确认失败**

```bash
go test ./internal/modules/appconnector/repository/appconnector/ -run TestSpaceConnectionGrantStore -count=1
```

预期：FAIL——`undefined: NewSpaceConnectionGrantStore`（编译失败即 RED）。

- [ ] **Step 4: 最小实现 `internal/modules/appconnector/repository/appconnector/space_grant.go`（整个新文件）**

```go
package appconnector

// Per-actor grants on SPACE connections (T23, #53). CONTEXT.md 代码平台连接:
// 个人连接只能由其所有者使用；空间连接按仓库、成员角色和操作策略授权。
// This store is the persistence half; the decision half lives in
// appconnector.CanUseConnection (access.go) and the A02 authorizer's Check
// (service/appconnector/oc_authorizer.go) — this package never decides.

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// SpaceConnectionGrantRow is one explicit actor's grant on one space
// connection. Personal connections never carry a row: the management
// endpoint refuses them and the authorizer never reads this table for
// Kind='personal'.
type SpaceConnectionGrantRow struct {
	TenantID     uint64    `gorm:"primaryKey;column:tenant_id"`
	ConnectionID string    `gorm:"primaryKey;column:connection_id;type:varchar(64)"`
	ActorID      string    `gorm:"primaryKey;column:actor_id;type:varchar(512)"`
	GrantedBy    string    `gorm:"column:granted_by;type:varchar(512);not null;default:''"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

// TableName binds SpaceConnectionGrantRow to app_space_connection_grants
// (versioned 000198 / sqlite 000119).
func (SpaceConnectionGrantRow) TableName() string { return "app_space_connection_grants" }

// SpaceConnectionGrantStore persists and adjudicates space-connection
// grants. It structurally satisfies appconnectorsvc.SpaceGrantSource via
// SpaceConnectionGranted.
type SpaceConnectionGrantStore struct{ db *gorm.DB }

func NewSpaceConnectionGrantStore(db *gorm.DB) *SpaceConnectionGrantStore {
	return &SpaceConnectionGrantStore{db: db}
}

// GrantSpaceConnection upserts one grant (idempotent per
// tenant+connection+actor). All inputs are bound parameters.
func (s *SpaceConnectionGrantStore) GrantSpaceConnection(ctx context.Context, tenantID uint64, connectionID, actorID, grantedBy string) error {
	row := SpaceConnectionGrantRow{
		TenantID: tenantID, ConnectionID: connectionID, ActorID: actorID,
		GrantedBy: grantedBy, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

// RevokeSpaceConnection removes one grant; revoking an absent grant is a
// success (idempotent).
func (s *SpaceConnectionGrantStore) RevokeSpaceConnection(ctx context.Context, tenantID uint64, connectionID, actorID string) error {
	return s.db.WithContext(ctx).
		Where("tenant_id = ? AND connection_id = ? AND actor_id = ?", tenantID, connectionID, actorID).
		Delete(&SpaceConnectionGrantRow{}).Error
}

// ListSpaceConnectionGrants returns the connection's grants, tenant-scoped.
func (s *SpaceConnectionGrantStore) ListSpaceConnectionGrants(ctx context.Context, tenantID uint64, connectionID string) ([]SpaceConnectionGrantRow, error) {
	var rows []SpaceConnectionGrantRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND connection_id = ?", tenantID, connectionID).
		Order("actor_id ASC").Find(&rows).Error
	return rows, err
}

// SpaceConnectionGranted answers the A02 authorizer's per-call question.
// Tenant scope is part of the predicate: another tenant's grant row is
// indistinguishable from no grant.
func (s *SpaceConnectionGrantStore) SpaceConnectionGranted(ctx context.Context, tenantID uint64, connectionID, actorID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&SpaceConnectionGrantRow{}).
		Where("tenant_id = ? AND connection_id = ? AND actor_id = ?", tenantID, connectionID, actorID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
```

- [ ] **Step 5: 运行 store 测试确认通过**

```bash
go test ./internal/modules/appconnector/repository/appconnector/ -run TestSpaceConnectionGrantStore -count=1
```

预期：PASS（全量迁移轨道此时已由 Task 0 修复；若报 duplicate migration 错误说明 Task 0 未完成，先回 Task 0）。

- [ ] **Step 6: 写 authorizer 集成测试 `internal/modules/appconnector/service/appconnector/oc_space_grant_test.go`（整个新文件）**

```go
package appconnector

// T23 (#53): the REAL grant store rides the A02 authorizer. These tests pin
// the three adjudication facts that matter to a person using the product:
// a space connection needs its own grant; a revoked (or foreign-tenant)
// grant stops resolving on the very next Check; and a space grant NEVER
// opens a personal connection (AC1: 个人与空间连接不能互相替代).

import (
	"context"
	"errors"
	"testing"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"gorm.io/gorm"
)

// newDBGrantAuthorizer is newAuthorizerFixture's twin, with the REAL
// SpaceConnectionGrantStore in place of the in-memory map source. Reuses the
// same AutoMigrate table set (newResolverFixture) plus the grant model.
func newDBGrantAuthorizer(t *testing.T) (appconn.OCAuthorizer, *appconnectorrepo.SpaceConnectionGrantStore, *gorm.DB) {
	t.Helper()
	_, db := newResolverFixture(t)
	if err := db.AutoMigrate(&appconnectorrepo.SpaceConnectionGrantRow{}); err != nil {
		t.Fatal(err)
	}
	authz := NewOCAuthorizer(
		apprepo.NewMCPOAuthBindingStore(db),
		&dbInstallationSource{db: db},
		appconnectorrepo.NewSpaceConnectionGrantStore(db),
		appconnectorrepo.NewOCStore(db),
	)
	return authz, appconnectorrepo.NewSpaceConnectionGrantStore(db), db
}

func TestSpaceGrantAuthorizerGrantAdmitsAndRevokeConverges(t *testing.T) {
	authz, grants, db := newDBGrantAuthorizer(t)
	seedOCSubjectFixture(t, db) // tenant 7: alice(owner) + bob, c-personal + c-space, auth_version 2
	ctx := context.Background()
	bob := appconn.OCSubject{TenantID: 7, ActorID: "bob"}

	// No grant: the space connection fails closed (the pre-#53 default).
	_, err := authz.Check(ctx, bob, "c-space", 2)
	if !errors.Is(err, ErrConnectionForbidden) {
		t.Fatalf("want ErrConnectionForbidden, got %v", err)
	}

	// An explicit grant admits bob's very next Check (native branch: the
	// binding miss returns an empty binding and no error).
	if err := grants.GrantSpaceConnection(ctx, 7, "c-space", "bob", "alice"); err != nil {
		t.Fatal(err)
	}
	binding, err := authz.Check(ctx, bob, "c-space", 2)
	if err != nil {
		t.Fatalf("granted space connection must pass: %v", err)
	}
	if binding.ConnectionID != "" {
		t.Fatalf("native branch returns an empty binding, got %+v", binding)
	}

	// Revocation converges on the very next Check (no cached positive).
	if err := grants.RevokeSpaceConnection(ctx, 7, "c-space", "bob"); err != nil {
		t.Fatal(err)
	}
	_, err = authz.Check(ctx, bob, "c-space", 2)
	if !errors.Is(err, ErrConnectionForbidden) {
		t.Fatalf("revoked grant must stop resolving immediately, got %v", err)
	}
}

func TestSpaceGrantNeverOpensPersonalConnection(t *testing.T) {
	authz, grants, db := newDBGrantAuthorizer(t)
	seedOCSubjectFixture(t, db)
	ctx := context.Background()
	bob := appconn.OCSubject{TenantID: 7, ActorID: "bob"}

	// bob holds a grant row on the SPACE connection; it must not leak onto
	// alice's personal connection (the authorizer only reads grants for
	// Kind='space'; CanUseConnection's personal branch only admits the owner).
	if err := grants.GrantSpaceConnection(ctx, 7, "c-space", "bob", "alice"); err != nil {
		t.Fatal(err)
	}
	_, err := authz.Check(ctx, bob, "c-personal", 2)
	if !errors.Is(err, ErrConnectionForbidden) {
		t.Fatalf("AC1: 空间连接的 grant 不能替代个人连接的 owner 谓词, got %v", err)
	}

	// The owner still passes her own personal connection with NO grant row.
	alice := appconn.OCSubject{TenantID: 7, ActorID: "alice"}
	if _, err := authz.Check(ctx, alice, "c-personal", 2); err != nil {
		t.Fatalf("owner's personal connection must pass: %v", err)
	}
}

func TestSpaceGrantAuthorizerCrossTenantGrantUnreachable(t *testing.T) {
	authz, grants, db := newDBGrantAuthorizer(t)
	seedOCSubjectFixture(t, db)
	ctx := context.Background()

	// A grant written under another tenant id never admits tenant 7's check.
	if err := grants.GrantSpaceConnection(ctx, 8, "c-space", "bob", "alice"); err != nil {
		t.Fatal(err)
	}
	_, err := authz.Check(ctx, appconn.OCSubject{TenantID: 7, ActorID: "bob"}, "c-space", 2)
	if !errors.Is(err, ErrConnectionForbidden) {
		t.Fatalf("cross-tenant grant row must be unreachable, got %v", err)
	}
}
```

实现依赖说明（执行者现场核实一次）：`newResolverFixture` 的签名/返回值以 `credentials_test.go:24` 现场为准（本计划写作时实测其返回 `(*credentialResolver, *gorm.DB)` 双值，测试用 `_, db :=` 接收）；`ErrConnectionForbidden` 是包级哨兵（`credentials.go:56`）。

- [ ] **Step 7: 运行确认通过 + 包级回归**

```bash
go test ./internal/modules/appconnector/service/appconnector/ -run 'TestSpaceGrant' -count=1 && go test ./internal/modules/appconnector/... -count=1
```

预期：全部 PASS。**RED 说明（如实声明）**：本测试套的「无 grant 拒绝 / 跨租户拒绝 / personal 不放行」三支在现状（nil-grants fail closed）下本就通过——真正的新 GREEN 是「grant admits」与「revoke 收敛」两支；本任务的 TDD RED 已在 Step 3（store 编译失败）完成。此处不伪造额外的 RED。

- [ ] **Step 8: Commit**

```bash
git add migrations/versioned/000198_space_connection_grants.up.sql migrations/versioned/000198_space_connection_grants.down.sql migrations/sqlite/000119_space_connection_grants.up.sql migrations/sqlite/000119_space_connection_grants.down.sql internal/modules/appconnector/repository/appconnector/space_grant.go internal/modules/appconnector/repository/appconnector/space_grant_test.go internal/modules/appconnector/service/appconnector/oc_space_grant_test.go
git commit -m "feat(appconnector): persist per-actor space-connection grants and ride the real A02 authorizer (#53 task 2)"
```

---

### Task 3: 空间连接 grant 管理端点 + router/container 生产接线

**Files:**
- Create: `internal/handler/app_connection_grants.go`
- Test: `internal/handler/app_connection_grants_test.go`
- Modify: `internal/router/routes_app_connectors.go`（文件末尾追加注册函数）
- Modify: `internal/router/router.go:141` 附近（RouterParams 加一字段）与 `:436` 附近（加一行调用）
- Modify: `internal/container/container.go:956` 附近（加一行 Provide）与 `:983-986`（闭包 nil → 真实 store）

**Interfaces:**
- Consumes: Task 2 的 `NewSpaceConnectionGrantStore` 四方法；`appconnector.CanManageConnections(role string) bool`（`access.go:33`，owner/admin）；`appTenantScope(c) (tenantID uint64, role string, userID string, ok bool)` 与 `appFail(c, status, code, message)`（`internal/handler/app_connector.go:46`、`:32`，同包私有）；`interfaces.TenantMemberRepository.Get(ctx, userID string, tenantID uint64) (*types.TenantMember, error)`（`internal/types/interfaces/tenant_member.go:23`，miss 返回 `(nil, nil)`），构造器 `repository.NewTenantMemberRepository(db)`（`tenant_member.go:35`）。
- Produces: HTTP 面 `POST|GET /api/v1/apps/connections/:id/grants`、`DELETE /api/v1/apps/connections/:id/grants/:grantee_id`（错误码 `SPACE_GRANT_FORBIDDEN`/`SPACE_GRANT_NOT_APPLICABLE`/`CONNECTION_NOT_FOUND`/`GRANTEE_NOT_ACTIVE_MEMBER`）；`handler.NewAppConnectionGrantHandler(db *gorm.DB) *AppConnectionGrantHandler`；container 的 A02 guard 以真实 grant store 装配（`SpaceGrantSource` 不再是 nil）。

- [ ] **Step 1: 写 handler 失败测试 `internal/handler/app_connection_grants_test.go`（整个新文件）**

```go
package handler

// T23 (#53) task 3: the space-grant management endpoint. The management
// predicate is appconnector.CanManageConnections (owner/admin), the target
// must be a SPACE connection, and the grantee must be an active same-tenant
// member. Personal connections are refused outright (AC1).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSpaceGrantEnv(t *testing.T) (*gin.Engine, *appconnectorrepo.SpaceConnectionGrantStore) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&appconnectorrepo.InstallationRow{}, &appconnectorrepo.ConnectionRow{},
		&appconnectorrepo.SpaceConnectionGrantRow{}, &types.TenantMember{},
	))
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-1", TenantID: 7, AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, Version: 1}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 7, ID: "conn-space", InstallationID: "inst-1", Kind: appconnector.ConnectionKindSpace, OwnerID: "u1", CredentialRef: "mcp:conn-space:github", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 7, ID: "conn-personal", InstallationID: "inst-1", Kind: appconnector.ConnectionKindPersonal, OwnerID: "u1", CredentialRef: "mcp:conn-personal:github", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1}).Error)
	for _, m := range []struct {
		user string
		role types.TenantRole
	}{
		{"u1", types.TenantRoleOwner},
		{"u2", types.TenantRoleContributor},
	} {
		require.NoError(t, db.Create(&types.TenantMember{UserID: m.user, TenantID: 7, Role: m.role, Status: types.TenantMemberStatusActive, JoinedAt: time.Now().UTC()}).Error)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAppConnectionGrantHandler(db)
	r.POST("/api/v1/apps/connections/:id/grants", h.Grant)
	r.GET("/api/v1/apps/connections/:id/grants", h.List)
	r.DELETE("/api/v1/apps/connections/:id/grants/:grantee_id", h.Revoke)
	return r, appconnectorrepo.NewSpaceConnectionGrantStore(db)
}

// spaceGrantDo injects the authenticated identity the same way the auth
// middleware's context keys do.
func spaceGrantDo(t *testing.T, r *gin.Engine, method, path, body, userID string, role types.TenantRole) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAppConnectionGrantLifecycleOwnerAdmitsMemberRefuses(t *testing.T) {
	r, grants := newSpaceGrantEnv(t)

	// Tenant owner grants an active member on the space connection.
	w := spaceGrantDo(t, r, http.MethodPost, "/api/v1/apps/connections/conn-space/grants", `{"grantee_id":"u2"}`, "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	granted, err := grants.SpaceConnectionGranted(context.Background(), 7, "conn-space", "u2")
	require.NoError(t, err)
	require.True(t, granted)

	// A regular member cannot manage grants (CanManageConnections).
	w = spaceGrantDo(t, r, http.MethodDelete, "/api/v1/apps/connections/conn-space/grants/u2", "", "u2", types.TenantRoleContributor)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "SPACE_GRANT_FORBIDDEN")

	// Owner lists, then revokes; revoke is idempotent.
	w = spaceGrantDo(t, r, http.MethodGet, "/api/v1/apps/connections/conn-space/grants", "", "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"grantee_id":"u2"`)
	w = spaceGrantDo(t, r, http.MethodDelete, "/api/v1/apps/connections/conn-space/grants/u2", "", "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusOK, w.Code)
	w = spaceGrantDo(t, r, http.MethodDelete, "/api/v1/apps/connections/conn-space/grants/u2", "", "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusOK, w.Code, "重复撤销幂等")

	granted, err = grants.SpaceConnectionGranted(context.Background(), 7, "conn-space", "u2")
	require.NoError(t, err)
	require.False(t, granted)
}

func TestAppConnectionGrantRefusesPersonalConnectionAndForeignTargets(t *testing.T) {
	r, _ := newSpaceGrantEnv(t)

	// AC1: a personal connection never takes a space grant.
	w := spaceGrantDo(t, r, http.MethodPost, "/api/v1/apps/connections/conn-personal/grants", `{"grantee_id":"u2"}`, "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "SPACE_GRANT_NOT_APPLICABLE")

	// Another tenant's connection id is indistinguishable from a missing one.
	w = spaceGrantDo(t, r, http.MethodPost, "/api/v1/apps/connections/conn-elsewhere/grants", `{"grantee_id":"u2"}`, "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "CONNECTION_NOT_FOUND")

	// A grantee who is not an active member of the tenant is refused.
	w = spaceGrantDo(t, r, http.MethodPost, "/api/v1/apps/connections/conn-space/grants", `{"grantee_id":"ghost"}`, "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "GRANTEE_NOT_ACTIVE_MEMBER")
}
```

实现依赖的既有事实（已核实）：`types.TenantRole` 四值 `owner/admin/contributor/viewer`（`internal/types/tenant_member.go:19-32`，无 `TenantRoleMember`——member 语义由 `TenantRoleContributor` 承担）；`types.TenantIDContextKey`/`types.UserIDContextKey`/`types.TenantRoleContextKey` 与 `task_collaboration_http_test.go:68-70` 同名。

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/handler/ -run TestAppConnectionGrant -count=1
```

预期：FAIL——`undefined: NewAppConnectionGrantHandler`（编译失败即 RED）。

- [ ] **Step 3: 最小实现 `internal/handler/app_connection_grants.go`（整个新文件）**

```go
package handler

// Space-connection grant management (T23, #53). CONTEXT.md 代码平台连接:
// 空间连接按仓库、成员角色和操作策略授权. The management predicate is
// appconnector.CanManageConnections (owner/admin) — the same vocabulary as
// connection management itself; the target must be a Kind='space'
// connection; the grantee must be an active same-tenant member. Identity
// always comes from the authenticated context (appTenantScope), never from
// the URL or body.

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppConnectionGrantHandler manages per-actor grants on space connections.
type AppConnectionGrantHandler struct {
	db      *gorm.DB
	grants  *appconnectorrepo.SpaceConnectionGrantStore
	members interfaces.TenantMemberRepository
}

// NewAppConnectionGrantHandler builds the handler over the shared db.
func NewAppConnectionGrantHandler(db *gorm.DB) *AppConnectionGrantHandler {
	return &AppConnectionGrantHandler{
		db:      db,
		grants:  appconnectorrepo.NewSpaceConnectionGrantStore(db),
		members: repository.NewTenantMemberRepository(db),
	}
}

// grantScope resolves identity + the management predicate shared by all
// three methods. A non-manager gets 403 before any row is read.
func (h *AppConnectionGrantHandler) grantScope(c *gin.Context) (uint64, string, bool) {
	tenantID, role, userID, ok := appTenantScope(c)
	if !ok {
		return 0, "", false
	}
	if !appconnector.CanManageConnections(role) {
		appFail(c, http.StatusForbidden, "SPACE_GRANT_FORBIDDEN",
			"managing space-connection grants requires the tenant owner or an admin")
		return 0, "", false
	}
	return tenantID, userID, true
}

// loadSpaceConnection loads the tenant-scoped connection and refuses
// anything that is not a space connection. A cross-tenant id is one uniform
// 404.
func (h *AppConnectionGrantHandler) loadSpaceConnection(c *gin.Context, tenantID uint64, id string) bool {
	var row appconnectorrepo.ConnectionRow
	err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		appFail(c, http.StatusNotFound, "CONNECTION_NOT_FOUND", "connection not found")
		return false
	}
	if err != nil {
		appFail(c, http.StatusInternalServerError, "CONNECTION_LOOKUP_FAILED", "connection lookup failed")
		return false
	}
	if row.Kind != appconnector.ConnectionKindSpace {
		// AC1: a personal connection is its owner's alone — grants do not
		// apply and would imply a sharing shape that must not exist.
		appFail(c, http.StatusBadRequest, "SPACE_GRANT_NOT_APPLICABLE",
			"only space connections take member grants")
		return false
	}
	return true
}

// requireActiveMember refuses grantees without an active same-tenant
// membership row.
func (h *AppConnectionGrantHandler) requireActiveMember(c *gin.Context, tenantID uint64, userID string) bool {
	member, err := h.members.Get(c.Request.Context(), userID, tenantID)
	if err != nil {
		appFail(c, http.StatusInternalServerError, "GRANTEE_LOOKUP_FAILED", "grantee membership lookup failed")
		return false
	}
	if member == nil || member.Status != types.TenantMemberStatusActive {
		appFail(c, http.StatusBadRequest, "GRANTEE_NOT_ACTIVE_MEMBER",
			"grantee must be an active member of this tenant")
		return false
	}
	return true
}

// Grant POST /apps/connections/:id/grants — body {"grantee_id": "..."}.
func (h *AppConnectionGrantHandler) Grant(c *gin.Context) {
	tenantID, userID, ok := h.grantScope(c)
	if !ok {
		return
	}
	if !h.loadSpaceConnection(c, tenantID, c.Param("id")) {
		return
	}
	var input struct {
		GranteeID string `json:"grantee_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.GranteeID) == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "grantee_id is required")
		return
	}
	if !h.requireActiveMember(c, tenantID, input.GranteeID) {
		return
	}
	if err := h.grants.GrantSpaceConnection(c.Request.Context(), tenantID, c.Param("id"), input.GranteeID, userID); err != nil {
		appFail(c, http.StatusInternalServerError, "SPACE_GRANT_FAILED", "failed to record the grant")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{
		"connection_id": c.Param("id"), "grantee_id": input.GranteeID, "granted_by": userID,
	}})
}

// Revoke DELETE /apps/connections/:id/grants/:grantee_id — idempotent.
func (h *AppConnectionGrantHandler) Revoke(c *gin.Context) {
	tenantID, _, ok := h.grantScope(c)
	if !ok {
		return
	}
	if !h.loadSpaceConnection(c, tenantID, c.Param("id")) {
		return
	}
	if err := h.grants.RevokeSpaceConnection(c.Request.Context(), tenantID, c.Param("id"), c.Param("grantee_id")); err != nil {
		appFail(c, http.StatusInternalServerError, "SPACE_GRANT_REVOKE_FAILED", "failed to revoke the grant")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"connection_id": c.Param("id"), "grantee_id": c.Param("grantee_id"), "revoked": true,
	}})
}

// List GET /apps/connections/:id/grants — manager-only member list.
func (h *AppConnectionGrantHandler) List(c *gin.Context) {
	tenantID, _, ok := h.grantScope(c)
	if !ok {
		return
	}
	if !h.loadSpaceConnection(c, tenantID, c.Param("id")) {
		return
	}
	grants, err := h.grants.ListSpaceConnectionGrants(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		appFail(c, http.StatusInternalServerError, "SPACE_GRANT_LIST_FAILED", "failed to list grants")
		return
	}
	items := make([]gin.H, 0, len(grants))
	for _, g := range grants {
		items = append(items, gin.H{"grantee_id": g.ActorID, "granted_by": g.GrantedBy, "created_at": g.CreatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"connection_id": c.Param("id"), "grants": items}})
}
```

- [ ] **Step 4: 运行 handler 测试确认通过**

```bash
go test ./internal/handler/ -run TestAppConnectionGrant -count=1
```

预期：PASS。

- [ ] **Step 5: router 注册 + container 接线（三处单点插入）**

(a) `internal/router/routes_app_connectors.go` 文件末尾追加：

```go
// RegisterAppConnectionGrantRoutes exposes the space-connection grant
// surface (T23 #53). The management predicate (CanManageConnections) lives
// in the handler's grantScope, mirroring the ApproveAction precedent: no
// extra group gate, the authenticated /api/v1 group plus the in-handler
// role check. Nil handler skips registration (same nil-guard convention as
// RegisterWorkbenchDeliveryRoutes).
func RegisterAppConnectionGrantRoutes(r *gin.RouterGroup, h *handler.AppConnectionGrantHandler) {
	if h == nil {
		return
	}
	r.POST("/apps/connections/:id/grants", h.Grant)
	r.GET("/apps/connections/:id/grants", h.List)
	r.DELETE("/apps/connections/:id/grants/:grantee_id", h.Revoke)
}
```

(b) `internal/router/router.go`：`RouterParams` 结构体（`:28` 起）在 `AppActionHandler` 字段（`:141`）后加一行：

```go
	AppConnectionGrantHandler *handler.AppConnectionGrantHandler
```

并在 `RegisterAppConnectorRoutes(v1, ...)` 调用块（`:431-436`）的右括号后加一行：

```go
		RegisterAppConnectionGrantRoutes(v1, params.AppConnectionGrantHandler)
```

(c) `internal/container/container.go`：在 `must(container.Provide(handler.NewAppActionHandler))`（`:956`）后加一行：

```go
	must(container.Provide(handler.NewAppConnectionGrantHandler))
```

并把 `:983-986` 的 guard 闭包从：

```go
	must(container.Provide(func(src appconnectorsvc.ConnectionCredentialSource,
		installs *repoappconn.InstallationStore, oc *repoappconn.OCStore,
	) appconnectorsvc.A02Guard {
		return appconnectorsvc.NewOCSubjectGuard(src, appconnectorsvc.NewInstallationStateSource(installs), nil, oc)
	}))
```

改为（闭包签名加 `grants` 形参，`nil` → 真实 store；`*SpaceConnectionGrantStore` 经 `SpaceConnectionGranted` 结构化满足 `SpaceGrantSource` 接口形参）：

```go
	must(container.Provide(func(src appconnectorsvc.ConnectionCredentialSource,
		installs *repoappconn.InstallationStore, grants *repoappconn.SpaceConnectionGrantStore,
		oc *repoappconn.OCStore,
	) appconnectorsvc.A02Guard {
		// T23 (#53): the space-grant store replaces the pre-#53 nil —
		// space connections are admitted per explicit grant row, and every
		// Check still re-queries the row live (revocation converges
		// immediately; the nil semantics could only fail closed, so this
		// wiring strictly widens toward the CONTEXT.md 授权模型).
		return appconnectorsvc.NewOCSubjectGuard(src, appconnectorsvc.NewInstallationStateSource(installs), grants, oc)
	}))
```

同时把 `:979-982` 的旧注释（"R11 carry (T07): the interim permission-only NewSubjectGuard(src) is replaced by the FULL subject guard … No space-grant store exists yet, so space connections keep failing closed …"）末句 `No space-grant store exists yet, so space connections keep failing closed (the authorizer's nil-grant semantics), which only tightens the interim behavior.` 替换为 `Since #53 the space-grant store is wired below — space connections are admitted per explicit grant row.`（注释与现状一致，行为描述不再过时）。

gin 路由共存说明（论据以版本为准）：gin v1.12.0（`go.mod:23`；1.7 起支持同一路由树中 static 与 param 兄弟节点共存）。现状事实（已核实）：GET 树的 `/apps/connections` 下只有静态 `oauth/callback`（`routes_app_connectors.go:69`），POST 树下只有 `:id/*` 参数段（`:47-60`）——即新增 GET `/apps/connections/:id/grants` 将首次让 `oauth` 静态段与 `:id` 参数段在同一 GET 树共存；这在 gin v1.12.0 是受支持的形状，若注册冲突会在 Task 3 Step 6 的 `go test ./internal/router/` 挂载时即时 panic 暴露，不会静默。

- [ ] **Step 6: 全链验证**

```bash
go build ./... && go test ./internal/router/ ./internal/container/ ./internal/handler/ -count=1
```

预期：build 通过、三包全部 PASS（含既有 wiring 测试；新路由不在 API-key authorizer 词表内——`/api/v1` gate 对 X-API-Key 默认拒绝，与 `/apps/*` 家族一致，`assertAPIKeyPoliciesMatchRoutes` 不受影响）。

- [ ] **Step 7: Commit**

```bash
git add internal/handler/app_connection_grants.go internal/handler/app_connection_grants_test.go internal/router/routes_app_connectors.go internal/router/router.go internal/container/container.go
git commit -m "feat(appconnector): manage space-connection grants over HTTP and wire the real grant store into the A02 guard (#53 task 3)"
```

---

### Task 4: 端到端证据——真实全链 httptest（AC1 / AC2 / AC3）

**Files:**
- Test: `internal/application/repository/delivery_collaboration_http_test.go`（整个新文件，package `repository_test`，与 `task_collaboration_http_test.go` 同目录同模式）

**Interfaces:**
- Consumes:
  - 同包既有 fixture：`openTaskGrantDB(t)`（`task_grant_store_test.go:27`，真实全量迁移 sqlite + seed tenants 1/2、users u1–u5、tenant_members、session `s1`(owner u1)）、`taskGrantAdmission()`（`task_grant_read_test.go:16`，run `r1` / session `s1` / owner `u1`）。
  - `session.NewWorkbenchDeliveryHandler(runs OwnedRunReader, granted GrantedRunReader, service DeliveryService)`（`workbench_delivery.go:34`）；`session.NewWorkbenchTaskGrantsHandler(grants TaskGrantManager)`（`workbench_task_grants.go:29`）。
  - `service.NewTaskGrantService(grants, sessions, members)`（`task_grant.go:41`）；`repository.NewTaskGrantStore/NewSessionRepository/NewTenantMemberRepository/NewAgentRunStore(db)`。
  - `codedelivery.NewCodeDeliveryService(CodeDeliveryDeps{...})`（`service.go:67`）；`appconnectorsvc.NewActionService(store, guard, gate, dispatcher, unknown)`（`action.go:192`，gate 传 nil——生产 `newCodeDeliveryService` 同形）；`codedelivery.NewGitHubClientFactory(httpClient, baseURL)`；`codedelivery.NewLocalWorkspaceSource(root)`；`codedelivery.NewDeliveryDispatcher(DispatcherDeps{...})`；`appconnectorsvc.NewSubjectGuard(src)`（单参 permission-only guard，与 `service_prepare_test.go:124` 夹具同形；全量 guard 的裁决已由 Task 2 服务级集成覆盖）。
  - `handler.NewAppActionHandler(db)` + `SetActionService(s)`（`app_connector_action.go:35,43`）——approve 走真实 HTTP（成功状态码 `http.StatusOK`，`app_connector_action.go:284`）。
- Produces: AC1/AC2/AC3 的端到端证据（测试名即证据名）；归因三元组 `initiator`/`approver`/`remote_login` 在同一读回中断言。

**替身论证（AC3 合规声明）：** 被测系统 = gin 路由 + 真实 handler + 真实 `CodeDeliveryService`/`ActionService`/`TaskGrantService` + 真实 gorm store + 真实全量迁移 sqlite。GitHub 是进程外 provider（本环境无凭据，blocked-env），其替身是文件内的 httptest Server——`gitHubRestClient`（`github_client.go`）的真实 HTTP 代码全部在跑，wire 形状与 `github_wire_test.go` 钉住的契约一致。mock 没有触碰任何被测授权/归因组件。

- [ ] **Step 1: 写端到端测试文件 `internal/application/repository/delivery_collaboration_http_test.go`（整个新文件）**

```go
package repository_test

// End-to-end delivery collaboration evidence (T23 #53). Everything runs
// through real HTTP handlers over a REAL fully-migrated sqlite database:
// the grants API, the delivery write/read surface, the A03 approve endpoint
// and the traceability read-back. GitHub is the only test double (an
// httptest server speaking the exact wire contract pinned by
// codedelivery/github_wire_test.go) — the provider is out of process and
// credential-gated in this environment (blocked-env); every authorization
// and attribution component under test is real. This is the AC3 evidence.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/codedelivery"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	deliveryToken   = "gho_testtoken"
	deliveryBaseSHA = "b00000000000000000000000000000000000000" // 40 hex chars
)

// githubStub speaks the minimal GitHub REST subset the delivery chain
// drives on the happy path (prepare: repo/branches/commits/trees/blobs;
// dispatch: ref create + PR + authenticated user). Response shapes mirror
// the emulator in codedelivery/github_wire_test.go.
type githubStub struct {
	srv *httptest.Server

	mu    sync.Mutex
	blobs map[string][]byte
	refs  map[string]string
	prs   []map[string]any
	prSeq int64
}

func newGitHubStub(t *testing.T) *githubStub {
	s := &githubStub{
		blobs: map[string][]byte{"blob-main": []byte("package main\n")},
		refs:  map[string]string{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serve)
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *githubStub) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *githubStub) serve(w http.ResponseWriter, r *http.Request) {
	if got := r.Header.Get("Authorization"); got != "Bearer "+deliveryToken {
		s.writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "bad credentials"})
		return
	}
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && path == "/user":
		s.writeJSON(w, http.StatusOK, map[string]any{"login": "octocat-remote"})
	case r.Method == http.MethodGet && path == "/repos/octocat/hello":
		s.writeJSON(w, http.StatusOK, map[string]any{"default_branch": "main", "private": false})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/branches/"):
		s.writeJSON(w, http.StatusOK, map[string]any{"protected": false})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/commits/"):
		s.writeJSON(w, http.StatusOK, map[string]any{"tree": "tree-baseline"})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/trees/"):
		// The fixed baseline tree: one blob file, as the stub seeded it.
		s.writeJSON(w, http.StatusOK, map[string]any{"truncated": false, "tree": []map[string]any{
			{"path": "main.go", "sha": "blob-main", "type": "blob"},
		}})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/blobs/"):
		sha := filepath.Base(path)
		content, ok := s.blobs[sha]
		if !ok {
			s.writeJSON(w, http.StatusNotFound, map[string]any{"message": "no such blob"})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{
			"encoding": "base64", "content": base64.StdEncoding.EncodeToString(content),
		})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/ref/heads/"):
		branch := strings.TrimPrefix(path, "/repos/octocat/hello/git/ref/heads/")
		sha, ok := s.refs[branch]
		if !ok {
			s.writeJSON(w, http.StatusNotFound, map[string]any{"message": "no such ref"})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"object": map[string]any{"sha": sha}})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/blobs":
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "blob-changed"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/trees":
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "tree-delivery"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/commits":
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "commit-delivery"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/refs":
		var body struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.mu.Lock()
		s.refs[strings.TrimPrefix(body.Ref, "refs/heads/")] = body.SHA
		s.mu.Unlock()
		s.writeJSON(w, http.StatusCreated, map[string]any{"ref": body.Ref})
	case r.Method == http.MethodGet && path == "/repos/octocat/hello/pulls":
		s.mu.Lock()
		out := append([]map[string]any{}, s.prs...)
		s.mu.Unlock()
		s.writeJSON(w, http.StatusOK, out)
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/pulls":
		s.mu.Lock()
		s.prSeq++
		pr := map[string]any{"number": s.prSeq, "html_url": fmt.Sprintf("https://github.com/octocat/hello/pull/%d", s.prSeq), "draft": true, "state": "open"}
		s.prs = append(s.prs, pr)
		s.mu.Unlock()
		s.writeJSON(w, http.StatusCreated, pr)
	default:
		s.writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected " + r.Method + " " + path})
	}
}

// deliveryCredentialSource adapts the real gorm rows onto the credential
// source + resolver the delivery chain needs (the same dual role the
// production MCPOAuthBindingStore plays; this adapter reads the SAME
// production connections table through bound parameters, so the e2e never
// stubs the authorization path. The token bytes are fixture data — the
// encrypted-token lifecycle belongs to the OAuth domain and is covered
// there).
type deliveryCredentialSource struct {
	db      *gorm.DB
	members map[string]bool
}

func newDeliveryCredentialSource(db *gorm.DB) *deliveryCredentialSource {
	return &deliveryCredentialSource{db: db, members: map[string]bool{"u1": true, "u2": true, "u3": true, "u4": true}}
}

func (s *deliveryCredentialSource) FindConnectionByID(ctx context.Context, id string) (appconnector.Connection, error) {
	var row appconnectorrepo.ConnectionRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", 1, id).First(&row).Error; err != nil {
		return appconnector.Connection{}, err
	}
	return appconnector.Connection{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind,
		OwnerID: row.OwnerID, CredentialRef: row.CredentialRef,
		State: row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}

func (s *deliveryCredentialSource) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	return []byte(deliveryToken), nil
}

func (s *deliveryCredentialSource) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return s.members[userID], nil
}

func (s *deliveryCredentialSource) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return true, nil
}

// Resolve satisfies appconnectorsvc.CredentialResolver: the delivery chain
// resolves the token AFTER the permission chain has passed.
func (s *deliveryCredentialSource) Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error) {
	return []byte(deliveryToken), nil
}

// deliveryCollabEnv is the fully-real HTTP rig: real migrated db, real
// handlers (task grants + delivery + A03 approve), real services.
type deliveryCollabEnv struct {
	db      *gorm.DB
	engine  *gin.Engine
	wsRoot  string
}

func newDeliveryCollabEnv(t *testing.T) *deliveryCollabEnv {
	t.Helper()
	db := openTaskGrantDB(t) // real full migrations/sqlite track + tenant/user/session fixtures

	// The task's admitted run: r1 on session s1, owner u1.
	runs := repository.NewAgentRunStore(db)
	_, err := runs.Admit(context.Background(), taskGrantAdmission())
	require.NoError(t, err)

	// Tenant GitHub App installation + u1's personal connection + the
	// tenant's space connection (both active, same installation).
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gh", TenantID: 1, AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, Version: 1}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 1, ID: "conn-gh", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal, OwnerID: "u1", CredentialRef: "mcp:conn-gh:github", State: appconnector.ConnectionActive, TenantID: 1, AuthVersion: 1}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 1, ID: "conn-space", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindSpace, OwnerID: "u1", CredentialRef: "mcp:conn-space:github", State: appconnector.ConnectionActive, TenantID: 1, AuthVersion: 1}).Error)

	// Local workspace + GitHub HTTP double + real delivery chain.
	github := newGitHubStub(t)
	wsRoot := t.TempDir()
	workspace, err := codedelivery.NewLocalWorkspaceSource(wsRoot)
	require.NoError(t, err)

	actionStore := appconnectorrepo.NewActionStore(db)
	store := deliveryrepo.NewDeliveryStore(db)
	connections := newDeliveryCredentialSource(db)
	guard := appconnectorsvc.NewSubjectGuard(connections)
	factory := codedelivery.NewGitHubClientFactory(http.DefaultClient, github.srv.URL)
	dispatcher := codedelivery.NewDeliveryDispatcher(codedelivery.DispatcherDeps{
		Connections: connections, Creds: connections, Guard: guard,
		GitHub: factory, Workspace: workspace, Store: store,
		ActionRows: actionStore, Runs: runs,
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcher, dispatcher)
	svc := codedelivery.NewCodeDeliveryService(codedelivery.CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: connections,
		GitHub: factory, Workspace: workspace, Runs: runs,
		Dispatcher: dispatcher,
	})

	// Handlers + routes (the manual mount mirrors
	// RegisterWorkbenchDeliveryRoutes / the task-grant mounts in
	// task_collaboration_http_test.go and the ApproveAction mount in
	// routes_app_connectors.go).
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	grantsSvc := service.NewTaskGrantService(
		repository.NewTaskGrantStore(db),
		repository.NewSessionRepository(db),
		repository.NewTenantMemberRepository(db),
	)
	v1.POST("/workbench/tasks/:task_id/grants", session.NewWorkbenchTaskGrantsHandler(grantsSvc).Grant)
	deliveryHandler := session.NewWorkbenchDeliveryHandler(runs, runs, svc)
	v1.GET("/workbench/executions/:run_id/delivery", deliveryHandler.GetDelivery)
	v1.POST("/workbench/executions/:run_id/baseline", deliveryHandler.MaterializeBaseline)
	v1.POST("/workbench/executions/:run_id/delivery", deliveryHandler.PrepareDelivery)
	v1.POST("/workbench/executions/:run_id/delivery/:delivery_id/dispatch", deliveryHandler.DispatchDelivery)
	actionHandler := handler.NewAppActionHandler(db)
	actionHandler.SetActionService(actions)
	v1.POST("/apps/actions/:id/approve", actionHandler.ApproveAction)
	return &deliveryCollabEnv{db: db, engine: r, wsRoot: wsRoot}
}

// do injects the authenticated identity exactly like the collaboration e2e
// does (request context keys the auth middleware writes in lockstep).
func (e *deliveryCollabEnv) do(t *testing.T, method, path, body, userID string, tenant uint64, role types.TenantRole) *httptest.ResponseRecorder {
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

// prepareDelivery drives the owner's real baseline→edit→prepare loop and
// returns the persisted delivery id / action id / digest.
func (e *deliveryCollabEnv) prepareDelivery(t *testing.T) (deliveryID, actionID, digest string) {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/baseline",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+deliveryBaseSHA+`"}`,
		"u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// The developer's edit on top of the fixed baseline (the bytes the
	// stub's CreateBlob acks).
	target := filepath.Join(e.wsRoot, "octocat/hello")
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "main.go"), []byte("package main\n// changed\n"), 0o644))

	w = e.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/delivery",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+deliveryBaseSHA+`","commit_message":"fix: greeting","pr_title":"WeKnora task s1"}`,
		"u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var prepared struct {
		Data struct {
			Delivery struct {
				ID        string `json:"id"`
				ActionID  string `json:"action_id"`
				Digest    string `json:"digest"`
				Initiator string `json:"initiator"`
				State     string `json:"state"`
			} `json:"delivery"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &prepared))
	require.Equal(t, "u1", prepared.Data.Delivery.Initiator, "发起者归因在 prepare 读回即在场")
	require.Equal(t, "prepared", prepared.Data.Delivery.State)
	return prepared.Data.Delivery.ID, prepared.Data.Delivery.ActionID, prepared.Data.Delivery.Digest
}

// approveThroughHTTP records u1's approval over the REAL A03 endpoint
// (digest + expected_version fence read from the persisted action row).
func (e *deliveryCollabEnv) approveThroughHTTP(t *testing.T, actionID, digest string) {
	t.Helper()
	var fence int64
	require.NoError(t, e.db.Raw(`SELECT fence FROM app_actions WHERE id = ?`, actionID).Scan(&fence).Error)
	body := fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, digest, fence)
	w := e.do(t, http.MethodPost, "/api/v1/apps/actions/"+actionID+"/approve", body, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestDeliveryCollaborationEndToEndAC1PersonalLoopAndAttribution(t *testing.T) {
	env := newDeliveryCollabEnv(t)
	deliveryID, actionID, digest := env.prepareDelivery(t)

	// AC1 negative half: the SAME owner cannot deliver through the SPACE
	// connection — a space connection never substitutes for the owner's
	// personal one on the delivery face (structural personal-only in
	// codedelivery.authorize; the rejection happens before any GitHub call).
	w := env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/delivery",
		`{"connection_id":"conn-space","repo":"octocat/hello","baseline_sha":"`+deliveryBaseSHA+`","commit_message":"fix: greeting","pr_title":"WeKnora task s1"}`,
		"u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "code_delivery_forbidden", "AC1: 空间连接不能替代个人连接")

	// Approve over the real A03 endpoint, then dispatch through the owner's
	// personal connection.
	env.approveThroughHTTP(t, actionID, digest)
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/delivery/"+deliveryID+"/dispatch",
		"", "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// The traceability read-back carries the FULL triple: initiator,
	// approver and the actual remote identity (the stub's GET /user).
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/delivery", "", "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"initiator":"u1"`, "发起成员（CONTEXT.md 代码平台连接）")
	require.Contains(t, w.Body.String(), `"approver":"u1"`, "批准成员")
	require.Contains(t, w.Body.String(), `"remote_login":"octocat-remote"`, "实际远端身份（provider GET /user）")
	require.Contains(t, w.Body.String(), `"pr_url":"https://github.com/octocat/hello/pull/1"`)
	require.NotContains(t, w.Body.String(), deliveryToken, "归因面永不携带凭据")
}

func TestDeliveryCollaborationEndToEndAC2CollaboratorCannotInheritPersonalConnection(t *testing.T) {
	env := newDeliveryCollabEnv(t)
	deliveryID, _, _ := env.prepareDelivery(t) // the owner's delivery exists first
	require.NotEmpty(t, deliveryID)

	// The owner shares the task: u3 becomes a collaborator.
	w := env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u3","role":"collaborator"}`, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// The collaborator CAN read the delivery (the #42 granted-read face).
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/delivery", "", "u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// ...but the write face stays owner-only: u3 cannot baseline-materialize
	// nor prepare a delivery through u1's personal connection. The strict
	// owner predicate answers a uniform 404 — a shared task never delegates
	// the owner's connection (CONTEXT.md 任务协作者). NOTE: the WRITE path's
	// miss is resolveOwnedRun's bare AbortWithStatus(404) (workbench_read.go:193-195,
	// no JSON body) — assert the status code only; the `run_not_found` JSON
	// envelope exists solely on the read face (resolveReadable).
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/baseline",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+deliveryBaseSHA+`"}`,
		"u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusNotFound, w.Code)

	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/delivery",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+deliveryBaseSHA+`","commit_message":"x","pr_title":"y"}`,
		"u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusNotFound, w.Code)

	// A member without any grant cannot even read the delivery.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/delivery", "", "u4", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeliveryCollaborationEndToEndCrossTenantIsolated(t *testing.T) {
	env := newDeliveryCollabEnv(t)
	env.prepareDelivery(t)

	// Tenant 2's principal probes tenant 1's run: one uniform 404 on the
	// read face — the tenant scope is bound from the authenticated context,
	// and no cross-tenant probe learns anything.
	w := env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/delivery", "", "outsider", 2, types.TenantRoleContributor)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "run_not_found")
}
```

- [ ] **Step 2: 运行端到端测试**

```bash
go test ./internal/application/repository/ -run 'TestDeliveryCollaboration' -count=1 -v 2>&1 | tail -30
```

预期：三个测试全部 PASS。失败排查表：
- `prepared.Data.Delivery.Initiator` 断言失败 → Task 1 未实施或字段名漂移，回 Task 1。
- approve 返回 409 → `fence` 列名漂移，核实 `internal/modules/appconnector/repository/appconnector/action.go:46`（`gorm:"column:fence"`）。
- dispatch 5xx 且日志显示 stub 404（`unexpected …`）→ stub 端点与 `github_client.go` 的请求路径漂移，以 `github_client.go` 现场路径为准修 stub（不改编解码器）。
- `openTaskGrantDB` 报迁移错误 → Task 0 未完成，回 Task 0。
- `GetDelivery` 404 出现在 AC2 的 u3 读 → prepare 未先执行（本计划已把 prepareDelivery 放在 grant 之前——保持该顺序）。
- u3 写路径（baseline/prepare）断言失败且响应体为空 → 正确行为就是裸 404：写面走 `resolveOwnedRun`（`workbench_read.go:193-195`，`c.AbortWithStatus(http.StatusNotFound)` 无 JSON body），只断言状态码；`run_not_found` JSON 仅在 GET 读面的 `resolveReadable`（`workbench_delivery.go:188-204`）输出。若把 Contains 断言加回写路径必然红灯。

- [ ] **Step 3: AC3 完整性复核（人工核对项，不写代码）**

对照 issue-53.md 验收标准逐条确认：
1. 「个人与空间连接不能互相替代」→ `TestDeliveryCollaborationEndToEndAC1PersonalLoopAndAttribution`（space 连接 403 + personal loop 全链 200）+ Task 2 `TestSpaceGrantNeverOpensPersonalConnection`（grant 不外溢到 personal）。
2. 「Collaborator 不能继承 Owner 个人连接」→ `TestDeliveryCollaborationEndToEndAC2CollaboratorCannotInheritPersonalConnection`（读 200 / 写 404）。
3. 「端到端行为通过最高稳定 Interface 验证」→ 全链 HTTP + 真实迁移 sqlite + 真实 service/store；GitHub 替身论证见文件头注释与本任务替身论证节。

- [ ] **Step 4: 计划级验证命令**

```bash
go build ./... && go test ./internal/database/ ./internal/modules/codedelivery/ ./internal/modules/appconnector/... ./internal/handler/ ./internal/application/repository/ ./internal/modules/workbench/service/workbench/ -count=1
```

预期：全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/application/repository/delivery_collaboration_http_test.go
git commit -m "test(delivery): end-to-end collaboration evidence over the real HTTP face — AC1/AC2/AC3 + traceability triple (#53 task 4)"
```

---

## blocked-env 验收项声明

| 验收面 | 状态 | 本地等价证据 | 真实环境门控 |
|---|---|---|---|
| 真实 GitHub API（远端写入、真实远端身份） | 本环境无凭据 | Task 4 的 httptest GitHub 替身（wire 契约与 `github_wire_test.go` 一致，`gitHubRestClient` 真实 HTTP 代码在跑） | 既有 `TestGitHubClientAgainstRealGitHub`（`github_real_test.go:15`，`WEKNORA_GITHUB_TEST_TOKEN`/`WEKNORA_GITHUB_TEST_REPO`）保持 opt-in，本计划不新增真实外呼 |
| Tenant GitHub App 的 OAuth 安装流（provider 侧） | 本环境无凭据 | 安装/连接行经真实迁移表 + 真实 store 落库（Task 4 e2e）；`CanInstallInstallation`/`CanManageConnections` 谓词矩阵由既有 `access_test.go:76-93` 钉住 | appconnector OAuth 既有 fail-closed 501 路径（`routes_app_connectors.go:45-48` 注释）不变 |

## 计划差异记录

1. 调查摘要四处与代码现状不符（#52 absent、归因零实现、github 仅测试夹具、谓词未接入交付路径），已按代码现状修正——见「调查差异记录」节。本计划的全部编辑锚定在实测文件行号上。
2. Task 2 的 RED 形态如实声明：无 grant 行时的拒绝是现状行为（nil-grants fail closed），本任务的新 GREEN 是「grant admits」与「revoke 收敛」两支；TDD RED 由 store 测试的编译失败（Step 3）承担。不伪造额外 RED。
3. Task 4 的 approve 走真实 HTTP（`/apps/actions/:id/approve`，成功 200）；审批谓词本身属 #42 证据域，此处只消费。digest/fence 从持久行读取，不注入。
4. Task 4 的 credential source 用本地适配器（读同一生产 `connections` 表 + 固定 token 字节）而非 `MCPOAuthBindingStore`——token 密文的加密器装配属 OAuth 域夹具，与本计划被测的授权/归因链正交；适配器只覆盖 delivery 链实际调用的 5 个方法，全部参数绑定。
5. 空间连接的「正向使用路径」（OC action execute 经 A02 Check 放行）依赖 OC runtime 的 dispatcher 接线（fail closed 503，`routes_app_connectors.go:82-92` 语境），不属本计划；Task 2 已在 authorizer.Check 服务级 seam 完成裁决集成证据。
6. **范围裁决：grant 粒度为 (tenant, connection, actor) 三元，不含 repo/操作策略维度——刻意收窄，非遗漏。** What to build 首句「Tenant GitHub App 按仓库、角色和策略授权」中，「按仓库」与「按操作」的细粒度策略需要授权判定携带 repo/action 上下文；而既有 `SpaceGrantSource` 接口签名（`oc_authorizer.go:21-24`，`(tenantID, connectionID, actorID)`）是 A02 Check 的固定接缝，结构上不携带 repo/action 参数——扩接口属跨域接口变更（影响 oc_authorizer 契约与其全部调用方），超出本 Issue 三条验收标准的必要范围。本计划落地的是消除「空间连接恒 fail closed」现状的最小授权模型：owner/admin 显式把 space 连接授权给 active 成员（Task 2/3），repo 维度的既有保障不变（GitHubClientFactory 以审批材料 RepoRef 钉仓 + protected-branch/default-branch 护栏 + ApproveAction 审批者谓词）。**残余范围（按仓库/按操作的细粒度空间连接策略，即 SpaceGrantSource 签名扩展 + 授权行加维度）归属后续 ticket**，验收对照本 Issue 三条 AC 判定：三条 AC 均不要求 repo 粒度授权（AC1 是互不替代的隔离断言、AC2 是 grant 不放宽写面、AC3 是 Interface 等级），本计划范围判定满足。
