# T34：Release 与依赖安全撤回传播（Issue #64）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在已合并的 #59（Adoption/Variant）、#60（Public Marketplace 与引入台账）、#61（Upgrade Proposal）之上实现 spec §10/§11 的安全撤回层：治理主体（Admin+，spec §13 `security_revoke_release`）可以按**不可变锁定身份**撤回一个 Agent Release 或一条锁定依赖；撤回记录 append-only 保留历史、原因、影响范围与替代版本；被撤回 Release 与被传递阻断（Dependency Security Block）的 Release 在**新引入 / 新变体发布 / 新 Task/Run** 三类入口被服务端拒绝（新 Task/Run 阻断覆盖 workbench executions 准入、agent-chat 轮次入口与 queue_next 重启派发）；在途执行按撤回请求声明的风险处置（默认 cancel，复用 #37 的全保真撤销语义）；任何路径都**不按名称替换依赖、不删除既有 Release/Variant/Task/审计历史**。

**Architecture:** 全部为 Go 后端新增层，零 TS 改动（spec §2：Marketplace 治理不在移动端执行；移动端消费面属 #65 信任信号）。数据层新增两张 append-only 台账表 `agent_release_revocations` / `agent_dependency_revocations`（双迁移流 versioned 000204 / sqlite 000125；#63 已占用 000203 / 000124）；依赖撤回的匹配键是 `types.AgentReleaseDependency` 的**完整四元组（type, id, version, digest）**——与 `DependencyLock` 已有的不可变版本+digest 锁定（internal/types/agent_marketplace.go:91-118）同一身份口径，天然满足「依赖不按名称自动替换」。仓储层新建 `AgentSecurityStore`（不改动既有 `AgentAdoptionRepository` 接口，传播查询直接覆盖 `agent_releases` 与 `tenant_introduced_releases` 两个 Release 来源）；`AgentRunStore` 新文件方法 `CancelRunsByAgents` 复用 #37 `CancelRun` 的同款事务语义（status=canceled + cancellation_requested 事件 + sessions 槽释放 + revision+1）。服务层新建 `AgentSecurityService`：判定面（`VerdictForAgent` / `ReleaseAdmission`）与操作面（`RevokeRelease` / `RevokeDependency`，撤回行与审计行同一事务写入，在途处置在提交后执行并回写 `canceled_run_count`）。运行入口闸门走两条既有缝：workbench 侧 `AdmissionCoordinator` 新增 `SetAgentSecurityGate`（在 W34 capability gate 同款位置——第一个 durable write 之前 consulted）；agent-chat 侧 `session.Handler` 新增 `SetAgentSecurityGate`（照 `SetTaskDeletionGuard`（internal/handler/session/handler.go:125-130）的 nil-safe 先例，在 SSE/消息行/live-run 槽产生之前拒绝）；治理面（Adopt/CreateVariant/PublishVariant/AcceptUpgradeProposal）经两个既有服务上的 `SetReleaseSecurityGate` 可选注入。端到端证据落在真实迁移流上的 router 级 HTTP 测试（与 #59/#61 同判的最高稳定 Interface）。

**Tech Stack:** Go 1.26（`go.mod` `go 1.26.0`，module `github.com/Tencent/WeKnora`）、gin + gorm + golang-migrate、`github.com/google/uuid`、`github.com/stretchr/testify/require`。测试复用既有真实迁移库 helper：repository 包 `openRunTestDB`（internal/application/repository/agent_run_test.go:29）、service 包 `openAgentVersionServiceTestDB`（internal/application/service/agent_version_test.go）与同包 seeder `publishUpgradeServiceRelease`/`adoptUpgradeRelease`（internal/application/service/agent_upgrade_test.go:46/85）、router 包 `openTenantAgentMarketplaceHTTPTestDB`（internal/router/routes_agent_marketplace_test.go:349）与 `adoptionCall`/`publishAdoptionRelease`（internal/router/routes_agent_adoption_test.go:74/93）、workbench 包 `openAdmissionConcurrencyDB`（internal/modules/workbench/service/workbench/admission_concurrency_test.go:168）。本计划无 TS/移动端改动、无外部凭据依赖、无 blocked-env 验收项（全部验收可在本地 sqlite 真实迁移流上验证）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-64.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-agent-marketplace-domain-model.md`（§10 目录和 Release 状态；§11 Dependency 与传递影响；§13 权限能力；§15 验收场景 5/6）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Testing Decisions：最高稳定 Interface 口径）
- 领域术语：`CONTEXT.md:161`（安全撤回）、`CONTEXT.md:185`（依赖安全阻断）——逐字见 Global Constraints
- 相关 ADR：`docs/adr/0011-agent-marketplace-release-adoption-boundary.md`（Release 与 Adoption 边界——撤回只引用不可变 Release 身份，不复制内容）
- Parent：Issue #30；Blocked by：#60（已合并）、#61（已合并）；本 Issue 阻塞 #65（信任信号消费本计划的撤回状态面）

## Global Constraints

以下为批准 Spec / Issue / 领域术语的项目级约束，逐字引用，所有任务隐含遵守：

- 「Release 或锁定依赖被撤回时，准确阻断受影响 Adoption/Variant 的新 Task/Run，并按风险处理在途执行。」（issue-64.md 正文 What to build）
- **验收标准原文（issue-64.md 任务清单）：**
  1. 「依赖不按名称自动替换。」
  2. 「撤回保留历史、原因、影响范围和替代版本。」
  3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」
- 「**安全撤回（Security Revocation）**：治理主体因已知安全风险禁止一个 Agent Release 被新引入或启动新 Task/Run 的强制状态；处置保留已有 Task、产物、审计与来源记录，并明确原因、影响范围和替代版本。」；_避免_：「普通下架、静默删除历史、把受影响 Task 自动切换到新版本。」（CONTEXT.md:161-163）
- 「**依赖安全阻断（Dependency Security Block）**：Agent Release 锁定的依赖被安全撤回后形成的传递状态，禁止该 Release 被新引入、用于发布新变体或启动新 Task/Run，直到新的 Agent Release 锁定安全依赖；它不删除原 Release、Task 或审计记录。」；_避免_：「运行时自动替换依赖、仅显示警告后继续新执行、删除受影响历史。」（CONTEXT.md:185-187）
- §10 状态表：「| Security Revoked | 否 | 否 | 禁止；运行中按风险暂停或终止 | 保留 |」「| Dependency Security Blocked | 否 | 否 | 禁止，直到新 Release 锁定安全依赖 | 保留 |」「任何状态都不静默升级或删除历史。」
- §11：「安全撤回的依赖传递阻断所有引用它的 Agent Release；」「运行时不得自动拉取最新依赖或按同名替换；」「修复依赖需要新 Agent Release、Review 和 Upgrade Proposal。」
- §13 领域能力：「security_revoke_release：安全撤回；」（HTTP 落地为 Admin+ full-access，与 Adoption 治理面同款，routes_agent_adoption.go:22-28 判例）
- §15 验收场景 5/6：「5. Release 安全撤回后，新 Task/Run 被服务端拒绝，历史可审计；6. 锁定 Skill 被撤回，依赖它的 Agent Release 被传递阻断，不能按名称替换；」
- 「移动端只显示已完成映射、测试和本地版本发布的 Available Agent。Marketplace 搜索、Submission、Review、Adoption 和升级不在移动端执行。」（agent-marketplace-domain-model.md:30——本计划零 TS 改动的依据）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（mobile-ai-office-design.md:153 · Testing Decisions）
- 安全约束（会话注入）：数据库查询所有外部输入使用参数绑定，不得拼接 SQL（本计划所有 store 查询 gorm 参数绑定；快照 agent_id 过滤在 Go 侧解码后匹配，不拼 JSON SQL）；服务端不新增外呼请求；无新凭据（源码与测试零凭据字面量）。
- 工作流约束：严格 RED→GREEN→REFACTOR；实现与已批准 Spec 冲突时升级而非静默重设计；只写计划内文件，git 提交由编排层统一执行的话以任务内 commit 步骤为准（执行者按步骤提交）。

## Review Focus

Spec 隐含但易咬人的五类输入/失效模式（每行后在所属任务以测试钉死）：

1. **同名依赖误伤/漏伤（AC1 的两个反面）**：撤回 (skill, web-search, 1.2.3, D1) 后，锁定 web-search **1.2.4/D2**（同名不同版本）的 Release 必须照常可用——阻断键是四元组不是名称；锁定 web-search **1.2.3/D2**（同名同版本不同 digest）的 Release 必须仍被阻断——digest 是身份的一部分。运行时任何路径不得借同名替换「自愈」。——Task 4 `TestAgentSecurityVerdictDependencyBlockedKeysOnExactLockedIdentity` + Task 9 e2e AC1 断言组。
2. **引入式 Release 漏检**：#60 之后一个 Tenant 的 Release 面横跨 `agent_releases` 与 `tenant_introduced_releases` 两张表；传播与判定只查本地表会把引入面漏成放行。——Task 2 `TestAgentSecurityStoreReleaseFactsCoversLocalAndIntroducedReleases` + Task 4 `TestAgentSecurityVerdictCoversIntroducedRelease`。
3. **跨租户枚举**：他租户的 revocation id / release / agent 查询必须与不存在同形（404 / 不阻断 / 空列表），不泄漏存在性。——Task 2 tenant-scope 断言 + Task 9 e2e（tenant-2 列表为空、跨租户 GET 404）。
4. **闸门基础设施故障放行**：安全闸门的 store 查询失败时，新 Task/Run 不得放行——fail closed（500/原错误，绝不 ok-verdict 兜底）。——Task 8 `TestAdmissionAgentSecurityGateFailsClosedOnInfrastructureError` + `TestGuardAgentSecurityFailsClosedOnInfrastructureError`。
5. **在途处置误伤**：终态 Run（succeeded/failed/canceled）、agent 不匹配的 Run、他租户的 Run 不得被取消；重复处置幂等（第二次计数为 0）。——Task 3 `TestCancelRunsByAgentsCancelsOnlyMatchingActiveRuns`。

（畸形治理输入——空 reason / 缺依赖四元体 / 未知 in_flight_disposition / 不可解析 Release——必须在落任何行之前 400 拒绝，由 Task 5 `TestRevokeRejectsMalformedInput` 钉死。）

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | 双流迁移 + 持久化实体 + 审计 Action 常量 | `agent_release_revocations` / `agent_dependency_revocations`（versioned 000204 / sqlite 000125）与实体、2 个 AuditAction |
| 2 | AgentSecurityStore | append-only 台账读写 + ReleaseFacts/ListTenantReleaseLocks（双源覆盖）+ Variant 查询 |
| 3 | AgentRunStore.CancelRunsByAgents | 在途执行批量撤销（#37 CancelRun 同款事务语义） |
| 4 | interfaces 类型 + AgentSecurityService 判定面 | `VerdictForAgent` / `ReleaseAdmission` / scope 纯计算（AC1 身份口径在此钉死） |
| 5 | AgentSecurityService 操作面 | RevokeRelease / RevokeDependency / List / Get（审计同事务、在途处置提交后执行并回写计数） |
| 6 | 治理面闸门接线 | Adoption 服务（Adopt/CreateVariant/PublishVariant）与 Upgrade 服务（Accept）的 `SetReleaseSecurityGate` 可选注入 |
| 7 | HTTP wire + 容器装配 | `/marketplace/tenant/security-revocations` 四端点（Admin+）+ dig 装配 + wiring 源级断言 |
| 8 | 运行入口闸门 | AdmissionCoordinator gate（首个 durable write 前）+ session.Handler AgentQA gate（SSE 前）+ 错误映射与容器接线 |
| 9 | 端到端证据 | `internal/router/routes_agent_security_test.go`（治理 wire + workbench 入口）与 `internal/handler/session/agent_security_e2e_test.go`（agent-chat 入口）：AC1/AC2/AC3 全链（真实迁移 + 真实 stack + 真实 HTTP） |

**新增/修改文件总表**（并行集成预警：除下表 Modify 外全部为本计划独有新增文件；对共享文件的修改均为小幅、位置明确）：

- Create：`migrations/versioned/000204_agent_security_revocations.{up,down}.sql`、`migrations/sqlite/000125_agent_security_revocations.{up,down}.sql`
- Create：`internal/types/agent_security_persistence.go`、`internal/types/interfaces/agent_security.go`
- Create：`internal/application/repository/agent_security.go`、`internal/application/repository/agent_security_test.go`、`internal/application/repository/agent_run_security_cancel.go`、`internal/application/repository/agent_run_security_cancel_test.go`
- Create：`internal/application/service/agent_security.go`、`internal/application/service/agent_security_test.go`、`internal/application/service/agent_security_guard_test.go`
- Create：`internal/handler/agent_security.go`、`internal/handler/agent_security_test.go`
- Create：`internal/router/routes_agent_security.go`、`internal/router/routes_agent_security_test.go`
- Create：`internal/container/agent_security.go`、`internal/container/agent_security_wiring_test.go`
- Create：`internal/handler/session/agent_security_gate.go`、`internal/handler/session/agent_security_gate_test.go`、`internal/handler/session/agent_security_e2e_test.go`
- Create：`internal/modules/workbench/service/workbench/admission_agent_security_test.go`
- Modify：`internal/types/audit_log.go`（T13 常量块后追加 2 个 AuditAction）
- Modify：`internal/application/service/agent_adoption.go`（1 个字段 + 1 个 setter + 3 处 guard 调用）、`internal/application/service/agent_upgrade.go`（1 个字段 + 1 个 setter + 1 处 guard 调用）
- Modify：`internal/modules/workbench/service/workbench/admission.go`（1 个字段 + 1 个 setter + Start 内 1 段 guard + 1 个哨兵）
- Modify：`internal/handler/session/handler.go`（1 个字段 + 1 个接口 + 1 个 setter）、`internal/handler/session/qa.go`（AgentQA 内 1 处 4 行 guard 调用）
- Modify：`internal/handler/session/workbench_start.go`（writeWorkbenchAdmissionError 加 1 个 case）
- Modify：`internal/handler/agent_adoption.go`（adoptionClientError 加 1 个 case）、`internal/handler/agent_upgrade.go`（upgradeClientError 加 1 个 case）
- Modify：`internal/container/container.go`（adoption/upgrade 提供 Concrete 化 + 3 行 Provide/Invoke + 1 行路由注册参数挂接）、`internal/container/workbench.go`（NewWorkbenchAdmissionCoordinator 加 1 参 + 1 行 gate 安装）
- Modify：`internal/router/router.go`（RouterParams 1 字段 + 1 行 RegisterAgentSecurityRoutes）

## 现状核实与差异记录（以本 worktree HEAD db234c5eb171f2dde7427d382b55b503a038f879 亲读为准）

1. **前置已满足**：调查摘要称「依赖 #60/#61 均缺前置」，为调查时点事实；当前 HEAD 两批已合并——`AgentAdoptionRepository.GetMarketplaceListing/GetRelease` 已含引入台账回退（internal/application/repository/agent_adoption.go:320-387），`AgentUpgradeService` 全链在（internal/application/service/agent_upgrade.go）。
2. **数据模型就绪确认**：`DependencyLock` 按 (type, id, version, digest) 锁定且注释声明「Runtime never substitutes a same-name dependency or pulls a newer version」（internal/types/agent_marketplace.go:91-118）——本计划的四元组匹配键与之同口径，不新建身份体系。
3. **撤回零实现确认**：`rg 'AgentSecurity|SecurityRevocation|DependencySecurityBlock|security-revocations|CancelRunsByAgents'` 在 internal/ 与 packages/ 零匹配（除本计划后建的文件），符号名无冲突。
4. **迁移号实测**（迁移号唯一性守卫 `internal/database.TestMigrationVersionsUniquePerTrack` 在 HEAD 实跑 PASS：`go test ./internal/database/ -run TestMigrationVersionsUniquePerTrack -count=1` → `ok ... 0.534s`）：`ls migrations/sqlite | tail` 最大号 **000123**、`ls migrations/versioned | tail` 最大号 **000202** → 本计划选定 **sqlite 000125 / versioned 000204**。执行时若同批次并行计划已占号，先重跑上述守卫测试并顺延取下一空号，DDL 零变化。
5. **波级事实核对（B5 OCR 已知项，指示第 4 条）**：(a) agent_upgrade 的 open 建议 from_release 脱节缺陷——HEAD 的 `reconcileProposals` 已含 stale-open 迁终态逻辑（internal/application/service/agent_upgrade.go:230-246，`AgentUpgradeSystemResolvedBy` 标记），实测已修复；(b) `UpsertLicense` created_by 覆写——HEAD 的 OnConflict DoUpdates 仅 `[name, allows_redistribution, updated_at]`（internal/application/repository/agent_marketplace_lineage.go:88-91），created_by 不被覆写，实测已修复。两项均不与本计划验收相交，无需处置。codedelivery 域 OCR 缺口（波级指示第 2 条）与 #51 Action Plan TOCTOU（第 3 条）不在本 Issue 域内，明确范围外。
6. **技能文件版本差异**：指令给定的 `.../superpowers/6.4.1/skills/writing-plans/SKILL.md` 路径不存在；实际读取 `.../superpowers/6.4.2/skills/writing-plans/SKILL.md` 并按其全部要求（含自我审查四项检查）执行。
7. **HTTP 发布链的锁为空**：生产 HTTP 提交链的 DependencyLock 由 `tenantReleaseDependencyResolver` 服务端解析，当前对含 skill/subagent 的 agent **fail closed 拒绝提交**、其余恒空锁（internal/container/container.go:123-140）。因此「带非空锁的 Release」在本计划测试中一律经真实仓储 `CreateSubmission + ReviewAndPublishTx` 播种（该链逐字落 `DependencyLockJSON`，internal/application/repository/agent_marketplace.go:163）——这是 #61 既有测试判例（`publishUpgradeServiceRelease`），不是 mock。
8. **基线可构建**：`go build ./...` 在本 worktree HEAD 实跑通过（仅链接器重复库警告）。

---

### Task 1: 双流迁移 + 持久化实体 + 审计 Action 常量

**Files:**
- Create: `migrations/versioned/000204_agent_security_revocations.up.sql`、`migrations/versioned/000204_agent_security_revocations.down.sql`
- Create: `migrations/sqlite/000125_agent_security_revocations.up.sql`、`migrations/sqlite/000125_agent_security_revocations.down.sql`
- Create: `internal/types/agent_security_persistence.go`
- Modify: `internal/types/audit_log.go`（T13 常量块（:201-207）之后追加）
- Test: `internal/application/repository/agent_security_test.go`（本任务只放迁移对齐测试）

**Interfaces:**
- Consumes: `types.AgentReleaseDependency`（internal/types/agent_marketplace.go:96-109）；迁移双轨判例 `migrations/{versioned/000200,sqlite/000120}_agent_upgrade_proposals.up.sql`。
- Produces: 表 `agent_release_revocations(id, tenant_id, listing_id, release_id, reason, replacement_release_id, in_flight_disposition, canceled_run_count, revoked_by, revoked_at, created_at)` 与 `agent_dependency_revocations(id, tenant_id, dep_type, dep_id, dep_version, dep_digest, reason, replacement_version, in_flight_disposition, canceled_run_count, revoked_by, revoked_at, created_at)`；实体 `types.AgentReleaseRevocationEntity` / `types.AgentDependencyRevocationEntity`（均 `TableName()` 钉死）；审计常量 `types.AuditActionAgentReleaseRevoked = "agent_security.release_revoked"`、`types.AuditActionAgentDependencyRevoked = "agent_security.dependency_revoked"`。供 Task 2/4/5 消费。

- [ ] **Step 1: 写失败测试（迁移↔投影对齐）**

`internal/application/repository/agent_security_test.go`：

```go
package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// T34 (#64) Task 1: 迁移↔投影对齐——两张撤回台账表必须由生产迁移轨道
// （migrations/sqlite 全量 Up，经 openRunTestDB）创建，列集与
// types.AgentReleaseRevocationEntity / AgentDependencyRevocationEntity 对齐。
func TestAgentSecurityRevocationTablesExistAfterMigrations(t *testing.T) {
	db := openRunTestDB(t)
	require.True(t, db.Migrator().HasTable("agent_release_revocations"),
		"agent_release_revocations 必须由生产迁移创建")
	for _, column := range []string{"id", "tenant_id", "listing_id", "release_id", "reason",
		"replacement_release_id", "in_flight_disposition", "canceled_run_count", "revoked_by", "revoked_at", "created_at"} {
		require.Truef(t, db.Migrator().HasColumn("agent_release_revocations", column),
			"agent_release_revocations.%s 必须存在（与实体列对齐）", column)
	}
	require.True(t, db.Migrator().HasTable("agent_dependency_revocations"),
		"agent_dependency_revocations 必须由生产迁移创建")
	for _, column := range []string{"id", "tenant_id", "dep_type", "dep_id", "dep_version", "dep_digest",
		"reason", "replacement_version", "in_flight_disposition", "canceled_run_count", "revoked_by", "revoked_at", "created_at"} {
		require.Truef(t, db.Migrator().HasColumn("agent_dependency_revocations", column),
			"agent_dependency_revocations.%s 必须存在（与实体列对齐）", column)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run TestAgentSecurityRevocationTablesExistAfterMigrations -count=1`
Expected: FAIL——`agent_release_revocations 必须由生产迁移创建`（表尚不存在）。

- [ ] **Step 3: 实现迁移与实体**

`migrations/versioned/000204_agent_security_revocations.up.sql`：

```sql
-- T34 (#64) security revocation ledgers (spec §10/§11, CONTEXT.md「安全撤回」
-- 「依赖安全阻断」). Append-only history: re-revocation adds a NEW row (the
-- latest per target is authoritative); nothing here deletes releases,
-- variants, tasks or audit rows. release_id / 替代 release 无 agent_releases
-- 外键（引入式 Release 位于 tenant_introduced_releases，#60 在 000114/000193
-- 放宽 adoption FK 的同一原因）；租户隔离由每条查询的 tenant_id 绑定保持。
CREATE TABLE agent_release_revocations (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL,
 listing_id VARCHAR(36) NOT NULL DEFAULT '', release_id VARCHAR(36) NOT NULL,
 reason TEXT NOT NULL,
 replacement_release_id VARCHAR(36) NOT NULL DEFAULT '',
 in_flight_disposition VARCHAR(16) NOT NULL DEFAULT 'cancel',
 canceled_run_count BIGINT NOT NULL DEFAULT 0,
 revoked_by VARCHAR(255) NOT NULL DEFAULT '',
 revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id)
);
CREATE INDEX idx_agent_release_revocations_release ON agent_release_revocations(tenant_id, release_id, created_at);
CREATE TABLE agent_dependency_revocations (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL,
 dep_type VARCHAR(32) NOT NULL, dep_id VARCHAR(255) NOT NULL,
 dep_version VARCHAR(64) NOT NULL, dep_digest VARCHAR(64) NOT NULL,
 reason TEXT NOT NULL,
 replacement_version VARCHAR(64) NOT NULL DEFAULT '',
 in_flight_disposition VARCHAR(16) NOT NULL DEFAULT 'cancel',
 canceled_run_count BIGINT NOT NULL DEFAULT 0,
 revoked_by VARCHAR(255) NOT NULL DEFAULT '',
 revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id)
);
CREATE INDEX idx_agent_dependency_revocations_identity ON agent_dependency_revocations(tenant_id, dep_type, dep_id, dep_version, dep_digest, created_at);
```

down：`DROP TABLE IF EXISTS agent_dependency_revocations; DROP TABLE IF EXISTS agent_release_revocations;`

sqlite 孪生（`migrations/sqlite/000125_agent_security_revocations.up.sql`）：同构，`tenant_id INTEGER`、时间列 `DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`、`canceled_run_count INTEGER`，首行注释改为「SQLite twin of versioned migration 000204.」（对照 000120/000200 判例）；down 同款。

`internal/types/agent_security_persistence.go`：

```go
package types

import "time"

// Agent security revocation persistence entities (T34, Ticket #64).
//
// 两张表都是 append-only 台账：同一 target 的重复撤回追加新行（读取侧以
// 最新行为准），旧行保留为历史（CONTEXT.md「安全撤回」_避免_「静默删除
// 历史」）。匹配键是 AgentReleaseDependency 的完整四元组
// (type, id, version, digest)——与 DependencyLock 的锁定身份同口径，
// 不是名称（spec §11「运行时不得自动拉取最新依赖或按同名替换」）。

type AgentReleaseRevocationEntity struct {
	ID                   string `gorm:"type:varchar(36);primaryKey"`
	TenantID             uint64 `gorm:"primaryKey"`
	ListingID            string `gorm:"type:varchar(36);not null;default:''"`
	ReleaseID            string `gorm:"type:varchar(36);not null"`
	Reason               string `gorm:"type:text;not null"`
	ReplacementReleaseID string `gorm:"type:varchar(36);not null;default:''"`
	InFlightDisposition  string `gorm:"type:varchar(16);not null;default:'cancel'"`
	CanceledRunCount     int64  `gorm:"not null;default:0"`
	RevokedBy            string `gorm:"type:varchar(255);not null;default:''"`
	RevokedAt            time.Time
	CreatedAt            time.Time
}

func (AgentReleaseRevocationEntity) TableName() string { return "agent_release_revocations" }

type AgentDependencyRevocationEntity struct {
	ID                  string `gorm:"type:varchar(36);primaryKey"`
	TenantID            uint64 `gorm:"primaryKey"`
	DepType             string `gorm:"type:varchar(32);not null"`
	DepID               string `gorm:"type:varchar(255);not null"`
	DepVersion          string `gorm:"type:varchar(64);not null"`
	DepDigest           string `gorm:"type:varchar(64);not null"`
	Reason              string `gorm:"type:text;not null"`
	ReplacementVersion  string `gorm:"type:varchar(64);not null;default:''"`
	InFlightDisposition string `gorm:"type:varchar(16);not null;default:'cancel'"`
	CanceledRunCount    int64  `gorm:"not null;default:0"`
	RevokedBy           string `gorm:"type:varchar(255);not null;default:''"`
	RevokedAt           time.Time
	CreatedAt           time.Time
}

func (AgentDependencyRevocationEntity) TableName() string { return "agent_dependency_revocations" }
```

`internal/types/audit_log.go`：在 `AuditActionTaskPurged` 常量块（:201-207）后追加新块：

```go
// T34 (#64) agent security revocation actions. release_revoked /
// dependency_revoked record the governance act (who, why, replacement,
// in-flight disposition) alongside the append-only revocation ledgers; the
// rows are written in the SAME transaction as the ledger insert.
const (
	AuditActionAgentReleaseRevoked   AuditAction = "agent_security.release_revoked"
	AuditActionAgentDependencyRevoked AuditAction = "agent_security.dependency_revoked"
)
```

- [ ] **Step 4: 运行确认通过 + 迁移轨道守卫**

Run: `go test ./internal/application/repository/ -run TestAgentSecurityRevocationTablesExistAfterMigrations -count=1 && go test ./internal/database/ -run TestMigrationVersionsUniquePerTrack -count=1`
Expected: 两个测试均 PASS（新号未与既有任何轨内号冲突）。

- [ ] **Step 5: Commit**

```bash
git add migrations/versioned/000204_agent_security_revocations.up.sql migrations/versioned/000204_agent_security_revocations.down.sql migrations/sqlite/000125_agent_security_revocations.up.sql migrations/sqlite/000125_agent_security_revocations.down.sql internal/types/agent_security_persistence.go internal/types/audit_log.go internal/application/repository/agent_security_test.go
git commit -m "feat(security): agent 安全撤回双台账迁移与实体（T34 #64 Task 1）"
```

---

### Task 2: AgentSecurityStore（append-only 台账 + 双源 Release 读取）

**Files:**
- Create: `internal/application/repository/agent_security.go`
- Test: `internal/application/repository/agent_security_test.go`（追加）

**Interfaces:**
- Consumes: Task 1 实体与表；既有 seeder `seedAdoptionRelease`（internal/application/repository/agent_adoption_test.go:16）。
- Produces（`repository` 包，供 Task 4/5 与 e2e 消费）:

```go
type AgentReleaseLockRow struct{ ReleaseID, ListingID, LockJSON string }

type AgentSecurityStore struct{ /* db *gorm.DB */ }
func NewAgentSecurityStore(db *gorm.DB) *AgentSecurityStore
func (s *AgentSecurityStore) AppendReleaseRevocation(ctx context.Context, row *types.AgentReleaseRevocationEntity) error   // 零值时填 ID=uuid.NewString()/RevokedAt/CreatedAt=now UTC
func (s *AgentSecurityStore) AppendDependencyRevocation(ctx context.Context, row *types.AgentDependencyRevocationEntity) error
func (s *AgentSecurityStore) ListReleaseRevocations(ctx context.Context, tenantID uint64) ([]types.AgentReleaseRevocationEntity, error)     // created_at ASC, id ASC
func (s *AgentSecurityStore) ListDependencyRevocations(ctx context.Context, tenantID uint64) ([]types.AgentDependencyRevocationEntity, error)
func (s *AgentSecurityStore) GetReleaseRevocation(ctx context.Context, tenantID uint64, id string) (*types.AgentReleaseRevocationEntity, error)      // nil,nil = 不存在/跨租户
func (s *AgentSecurityStore) GetDependencyRevocation(ctx context.Context, tenantID uint64, id string) (*types.AgentDependencyRevocationEntity, error)
func (s *AgentSecurityStore) UpdateReleaseRevocationCanceled(ctx context.Context, tenantID uint64, id string, canceled int64) error   // 仅更新 canceled_run_count/updated 时间戳外的单列，参数绑定
func (s *AgentSecurityStore) UpdateDependencyRevocationCanceled(ctx context.Context, tenantID uint64, id string, canceled int64) error
func (s *AgentSecurityStore) ReleaseFacts(ctx context.Context, tenantID uint64, releaseID string) (listingID string, lockJSON string, found bool, err error)
func (s *AgentSecurityStore) ListTenantReleaseLocks(ctx context.Context, tenantID uint64) ([]AgentReleaseLockRow, error)
func (s *AgentSecurityStore) VariantsByLocalAgent(ctx context.Context, tenantID uint64, agentID string) ([]types.AgentAdoptionVariantEntity, error) // local_agent_id = ? AND state = 'published'
func (s *AgentSecurityStore) ListVariants(ctx context.Context, tenantID uint64) ([]types.AgentAdoptionVariantEntity, error)
```

`ReleaseFacts`/`ListTenantReleaseLocks` 的实现要点（签名与测试不决定内部查询时给出）：先查 `agent_releases`（`WHERE tenant_id = ? AND id = ?`，miss 时**继续**查 `tenant_introduced_releases` 同租户条件，`ReleaseFacts` 取其 `public_listing_id`/`dependency_lock_json`，`ListTenantReleaseLocks` 两表结果合并去重 by ReleaseID、本地行优先）——全部参数绑定，无 SQL 拼接。

- [ ] **Step 1: 写失败测试**

追加到 `internal/application/repository/agent_security_test.go`：

```go
func TestAgentSecurityStoreAppendAndListKeepsHistoryTenantScoped(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentSecurityStore(db)
	ctx := context.Background()

	first := &types.AgentReleaseRevocationEntity{TenantID: 1, ListingID: "l1", ReleaseID: "r1",
		Reason: "CVE-2026-0001 prompt exfiltration", ReplacementReleaseID: "r2",
		InFlightDisposition: "cancel", RevokedBy: "sec-admin"}
	require.NoError(t, store.AppendReleaseRevocation(ctx, first))
	second := &types.AgentReleaseRevocationEntity{TenantID: 1, ListingID: "l1", ReleaseID: "r1",
		Reason: "expanded: subagent path also affected", InFlightDisposition: "allow", RevokedBy: "sec-admin-2"}
	require.NoError(t, store.AppendDependencyRevocation(ctx, &types.AgentDependencyRevocationEntity{
		TenantID: 1, DepType: "skill", DepID: "web-search", DepVersion: "1.2.3", DepDigest: "D1",
		Reason: "malicious exfil in pinned skill", ReplacementVersion: "1.2.4", RevokedBy: "sec-admin"}))
	require.NoError(t, store.AppendReleaseRevocation(ctx, second))

	rows, err := store.ListReleaseRevocations(ctx, 1)
	require.NoError(t, err)
	require.Len(t, rows, 2, "撤回历史 append-only：同一 Release 重复撤回保留两条记录")
	require.Equal(t, first.ID, rows[0].ID, "列表按 created_at ASC")
	require.NotEmpty(t, rows[0].ID, "Append 必须填充 ID")
	require.False(t, rows[0].RevokedAt.IsZero(), "Append 必须填充 RevokedAt")

	deps, err := store.ListDependencyRevocations(ctx, 1)
	require.NoError(t, err)
	require.Len(t, deps, 1)
	require.Equal(t, "web-search", deps[0].DepID)

	foreignReleases, err := store.ListReleaseRevocations(ctx, 2)
	require.NoError(t, err)
	require.Empty(t, foreignReleases, "跨租户列表为空")
	foreignDeps, err := store.ListDependencyRevocations(ctx, 2)
	require.NoError(t, err)
	require.Empty(t, foreignDeps)

	got, err := store.GetReleaseRevocation(ctx, 1, second.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	miss, err := store.GetReleaseRevocation(ctx, 2, second.ID)
	require.NoError(t, err)
	require.Nil(t, miss, "跨租户单读与不存在同形")

	require.NoError(t, store.UpdateReleaseRevocationCanceled(ctx, 1, second.ID, 7))
	updated, err := store.GetReleaseRevocation(ctx, 1, second.ID)
	require.NoError(t, err)
	require.EqualValues(t, 7, updated.CanceledRunCount)
}

func TestAgentSecurityStoreReleaseFactsCoversLocalAndIntroducedReleases(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentSecurityStore(db)
	ctx := context.Background()

	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-sec", "1.0.0")
	listing, lockJSON, found, err := store.ReleaseFacts(ctx, 1, releaseID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, listingID, listing)
	require.JSONEq(t, `{"dependencies":[]}`, lockJSON)

	// #60 引入式 Release：tenant_introduced_releases 的行必须同样可解析。
	require.NoError(t, db.Exec(`INSERT INTO tenant_introduced_releases
		(id, tenant_id, public_listing_id, public_release_id, display_name, semantic_version,
		 bundle_digest, manifest_json, dependency_lock_json, bundle, introduced_by, introduced_at)
		VALUES ('intro-r1', 1, 'pub-listing-1', 'pub-rel-1', 'Introduced', '2.0.0', 'd-intro', '{}',
		 '{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.3","digest":"D1","license_id":"MIT"}]}',
		 '{}', 'admin', datetime('now'))`).Error)
	introListing, introLock, found, err := store.ReleaseFacts(ctx, 1, "intro-r1")
	require.NoError(t, err)
	require.True(t, found, "引入式 Release 不得被漏检（#60 台账）")
	require.Equal(t, "pub-listing-1", introListing)
	require.Contains(t, introLock, `"web-search"`)

	_, _, found, err = store.ReleaseFacts(ctx, 2, releaseID)
	require.NoError(t, err)
	require.False(t, found, "跨租户 miss")

	locks, err := store.ListTenantReleaseLocks(ctx, 1)
	require.NoError(t, err)
	byID := map[string]AgentReleaseLockRow{}
	for _, row := range locks {
		byID[row.ReleaseID] = row
	}
	require.Contains(t, byID, releaseID, "本地 Release 进入锁清单")
	require.Contains(t, byID, "intro-r1", "引入式 Release 进入锁清单")
	require.Contains(t, byID["intro-r1"].LockJSON, `"1.2.3"`)
}

func TestAgentSecurityStoreVariantsByLocalAgentTenantScoped(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentSecurityStore(db)
	ctx := context.Background()
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-sec2", "1.0.0")
	adoptions := NewAgentAdoptionRepository(db)
	adoption, _, err := adoptions.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin"})
	require.NoError(t, err)
	variant, err := adoptions.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adoption.ID, ReleaseID: releaseID, Name: "V", State: "draft", CreatedBy: "admin"})
	require.NoError(t, err)
	_, err = adoptions.UpdateVariantState(ctx, 1, variant.ID, []string{"draft"}, "published", map[string]any{"local_agent_id": "local-agent-1", "local_agent_version_id": "ver-1", "published_by": "admin"})
	require.NoError(t, err)

	rows, err := store.VariantsByLocalAgent(ctx, 1, "local-agent-1")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, variant.ID, rows[0].ID)
	foreign, err := store.VariantsByLocalAgent(ctx, 2, "local-agent-1")
	require.NoError(t, err)
	require.Empty(t, foreign, "跨租户查询为空")
	all, err := store.ListVariants(ctx, 1)
	require.NoError(t, err)
	require.Len(t, all, 1)
}
```

（测试文件需补 import：`context`、`github.com/Tencent/WeKnora/internal/types`。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run 'TestAgentSecurityStore' -count=1`
Expected: 编译失败——`undefined: NewAgentSecurityStore`。

- [ ] **Step 3: 实现 `internal/application/repository/agent_security.go`**

按 Interfaces 块的 12 个方法实现；实现纪律：全部查询 `tenant_id` 参数绑定；`ListTenantReleaseLocks` 合并两表（本地行优先去重）；`GetReleaseRevocation`/`GetDependencyRevocation` miss（含跨租户）返回 `(nil, nil)`；Append 在零值时填 `uuid.NewString()` 与 `time.Now().UTC()`（两列同值），不覆写调用方已填值（确定性测试可用）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/application/repository/ -run 'TestAgentSecurity' -count=1`
Expected: PASS（3 个新测试 + Task 1 对齐测试）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/repository/agent_security.go internal/application/repository/agent_security_test.go
git commit -m "feat(security): AgentSecurityStore append-only 台账与双源 Release 读取（T34 #64 Task 2）"
```

---

### Task 3: AgentRunStore.CancelRunsByAgents（在途执行按风险处置）

**Files:**
- Create: `internal/application/repository/agent_run_security_cancel.go`
- Test: `internal/application/repository/agent_run_security_cancel_test.go`

**Interfaces:**
- Consumes: 包内私有 `agentRunRow`（internal/application/repository/agent_run.go:41）、`appendRunEventLocked`（internal/application/repository/agent_run_events.go:170）、#37 `CancelRun` 的事务语义范本（internal/application/repository/agent_run_lifecycle.go:16-53）。
- Produces: `func (s *AgentRunStore) CancelRunsByAgents(ctx context.Context, tenantID uint64, agentIDs []string, reason string) (int64, error)`——一个事务内：选出该租户 `status NOT IN ('succeeded','failed','canceled')` 的 Run，在 Go 侧解码 `snapshot` JSON 的 `agent_id`（无 JSON SQL、无拼接），命中 `agentIDs` 集合的行逐一执行 CancelRun 同款三段（`status='canceled'`+`wait_reason`+`lease` 清空+`revision+1`；`appendRunEventLocked` 写 `cancellation_requested`；释放 `sessions.active_agent_run_id` 槽），返回命中计数；空 agentIDs/tenantID=0 返回 `(0, nil)`。供 Task 5 与 e2e 消费。

- [ ] **Step 1: 写失败测试**

`internal/application/repository/agent_run_security_cancel_test.go`：

```go
package repository

import (
	"context"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
)

// seedSecurityCancelFixture 复用 openRunTestDB 已种的 tenant 1 / u1 / s1 / s2，
// 另种 tenant 2 + s3 与四条 Run：r1=在途且 agent 命中（含会话活动槽）、
// r2=终态同 agent、r3=在途但 agent 不命中、r4=他租户在途同 agent 字符串。
func seedSecurityCancelFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (2, 't2', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('u2','u2','u2@example.test','x',2)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type, active_agent_run_id) VALUES
		('s3', 2, 'task-3', 'u2', 'trpc', 'sec-r4')`).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = 'sec-r1' WHERE tenant_id = 1 AND id = 's1'`).Error)
	blocked := `{"session_id":"s1","agent_id":"local-agent-blocked","request_id":"req-1","text":"hi"}`
	clean := `{"session_id":"s2","agent_id":"local-agent-clean","request_id":"req-2","text":"hi"}`
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, engine_type, status, snapshot, deadline) VALUES
		(1, 'sec-r1', 's1', 'u1', 'req-1', 'm1', 'h1', 'trpc', 'running',  ?, datetime('now','+1 hour')),
		(1, 'sec-r2', 's1', 'u1', 'req-2', 'm2', 'h2', 'trpc', 'succeeded', ?, datetime('now','+1 hour')),
		(1, 'sec-r3', 's2', 'u1', 'req-3', 'm3', 'h3', 'trpc', 'running',  ?, datetime('now','+1 hour')),
		(2, 'sec-r4', 's3', 'u2', 'req-4', 'm4', 'h4', 'trpc', 'running',  ?, datetime('now','+1 hour'))`,
		blocked, blocked, clean, blocked).Error)
}

func TestCancelRunsByAgentsCancelsOnlyMatchingActiveRuns(t *testing.T) {
	db := openRunTestDB(t)
	seedSecurityCancelFixture(t, db)
	runs := NewAgentRunStore(db)
	ctx := context.Background()

	canceled, err := runs.CancelRunsByAgents(ctx, 1, []string{"local-agent-blocked"}, "agent security revocation: CVE-2026-0001")
	require.NoError(t, err)
	require.EqualValues(t, 1, canceled)

	blocked, err := runs.Get(ctx, agentruntime.RunKey{TenantID: 1, RunID: "sec-r1"})
	require.NoError(t, err)
	require.Equal(t, "canceled", blocked.Status)
	require.Equal(t, "agent security revocation: CVE-2026-0001", blocked.WaitReason)

	var events int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_run_events WHERE tenant_id = 1 AND run_id = 'sec-r1' AND event_type = 'cancellation_requested'`).Scan(&events).Error)
	require.EqualValues(t, 1, events, "在途处置必须留下 cancellation_requested 时间线事实（#37 语义）")

	var slot *string
	require.NoError(t, db.Raw(`SELECT active_agent_run_id FROM sessions WHERE tenant_id = 1 AND id = 's1'`).Scan(&slot).Error)
	require.Nil(t, slot, "会话活动 Run 槽必须释放")

	terminal, err := runs.Get(ctx, agentruntime.RunKey{TenantID: 1, RunID: "sec-r2"})
	require.NoError(t, err)
	require.Equal(t, "succeeded", terminal.Status, "终态 Run 不被在途处置触碰")
	other, err := runs.Get(ctx, agentruntime.RunKey{TenantID: 1, RunID: "sec-r3"})
	require.NoError(t, err)
	require.Equal(t, "running", other.Status, "agent 不匹配的 Run 不受影响")
	foreign, err := runs.Get(ctx, agentruntime.RunKey{TenantID: 2, RunID: "sec-r4"})
	require.NoError(t, err)
	require.Equal(t, "running", foreign.Status, "跨租户 Run 不受影响")

	again, err := runs.CancelRunsByAgents(ctx, 1, []string{"local-agent-blocked"}, "again")
	require.NoError(t, err)
	require.EqualValues(t, 0, again, "处置幂等：已终态的行不再计入")
}
```

（该文件 import 需含 `gorm.io/gorm`。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run TestCancelRunsByAgentsCancelsOnlyMatchingActiveRuns -count=1`
Expected: 编译失败——`runs.CancelRunsByAgents undefined`。

- [ ] **Step 3: 实现 `internal/application/repository/agent_run_security_cancel.go`**

一个 `s.db.WithContext(ctx).Transaction`：`Table("agent_runs").Where("tenant_id = ? AND status NOT IN ?", tenantID, []string{"succeeded", "failed", "canceled"}).Find(&rows)`（参数绑定）→ Go 侧 `json.Unmarshal([]byte(row.Snapshot), &struct{ AgentID string `json:"agent_id"` }{})`，解析失败的行跳过（非 coordinator 快照，fail-quiet 跳过与 Restart 的 fail-closed 不同：这里是批量治理扫描，不是准入判定）→ 命中集合的行按 CancelRun 三段更新（`Clauses(clause.Locking{Strength: "UPDATE"})` 逐行 Take 后 Updates，事件经 `appendRunEventLocked`，槽释放 `tx.Table("sessions").Where("tenant_id=? AND id=? AND active_agent_run_id=?", ...)`）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/application/repository/ -run TestCancelRunsByAgents -count=1`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/application/repository/agent_run_security_cancel.go internal/application/repository/agent_run_security_cancel_test.go
git commit -m "feat(security): AgentRunStore.CancelRunsByAgents 在途处置原语（T34 #64 Task 3）"
```

---

### Task 4: interfaces 类型 + AgentSecurityService 判定面（VerdictForAgent / ReleaseAdmission / scope）

**Files:**
- Create: `internal/types/interfaces/agent_security.go`
- Create: `internal/application/service/agent_security.go`
- Test: `internal/application/service/agent_security_test.go`

**Interfaces:**
- Consumes: Task 2 `AgentSecurityStore` 全部方法；`types.DependencyLock`/`types.AgentReleaseDependency`（JSON 反序列化 `DependencyLockJSON`）；同包既有 seeder `publishUpgradeServiceRelease(t, db, versionNumber, semanticVersion, manifest, lock, bundle)` 与 `adoptUpgradeRelease(t, db, listingID, releaseID)`（internal/application/service/agent_upgrade_test.go:46/85）、`openAgentVersionServiceTestDB`。
- Produces:

```go
// internal/types/interfaces/agent_security.go
const (
	AgentSecurityRevocationKindRelease    = "release"
	AgentSecurityRevocationKindDependency = "dependency"
	AgentSecurityVerdictOK                = "ok"
	AgentSecurityVerdictReleaseRevoked    = "release-revoked"
	AgentSecurityVerdictDependencyBlocked = "dependency-blocked"
	AgentSecurityInFlightCancel           = "cancel"
	AgentSecurityInFlightAllow            = "allow"
)

type AgentSecurityVerdict struct {
	State                 string                        `json:"state"`
	Reason                string                        `json:"reason,omitempty"`
	RevocationID          string                        `json:"revocation_id,omitempty"`
	ReleaseID             string                        `json:"release_id,omitempty"`
	Dependency            *types.AgentReleaseDependency `json:"dependency,omitempty"`
	ReplacementReleaseID  string                        `json:"replacement_release_id,omitempty"`
	ReplacementVersion    string                        `json:"replacement_version,omitempty"`
}
func (v AgentSecurityVerdict) Blocked() bool // State != AgentSecurityVerdictOK

type AgentBlockedRelease struct{ ReleaseID, ListingID, BlockedBy string } // BlockedBy: kind 常量
type AgentBlockedVariant struct{ VariantID, AdoptionID, ReleaseID, State, LocalAgentID string }
type AgentRevocationScope struct {
	BlockedReleases     []AgentBlockedRelease  `json:"blocked_releases"`
	AffectedAdoptionIDs []string               `json:"affected_adoption_ids"`
	AffectedVariants    []AgentBlockedVariant  `json:"affected_variants"`
}

type ReleaseRevocationInput struct{ ReleaseID, Reason, ReplacementReleaseID, InFlightDisposition string }
type DependencyRevocationInput struct {
	Dependency           types.AgentReleaseDependency
	Reason, ReplacementVersion, InFlightDisposition string
}

type AgentSecurityRevocationView struct {
	ID                  string                        `json:"id"`
	Kind                string                        `json:"kind"`
	Reason              string                        `json:"reason"`
	RevokedBy           string                        `json:"revoked_by"`
	RevokedAt           time.Time                     `json:"revoked_at"`
	InFlightDisposition string                        `json:"in_flight_disposition"`
	CanceledRunCount    int64                         `json:"canceled_run_count"`
	ListingID           string                        `json:"listing_id,omitempty"`
	ReleaseID           string                        `json:"release_id,omitempty"`
	ReplacementReleaseID string                       `json:"replacement_release_id,omitempty"`
	Dependency          *types.AgentReleaseDependency `json:"dependency,omitempty"`
	ReplacementVersion  string                        `json:"replacement_version,omitempty"`
	Scope               *AgentRevocationScope         `json:"scope,omitempty"`
}

type AgentSecurityService interface {
	RevokeRelease(ctx context.Context, tenantID uint64, actorID string, input ReleaseRevocationInput) (AgentSecurityRevocationView, error)
	RevokeDependency(ctx context.Context, tenantID uint64, actorID string, input DependencyRevocationInput) (AgentSecurityRevocationView, error)
	ListRevocations(ctx context.Context, tenantID uint64) ([]AgentSecurityRevocationView, error) // 不带 scope（列表瘦身），created_at ASC
	GetRevocation(ctx context.Context, tenantID uint64, revocationID string) (AgentSecurityRevocationView, error) // 带 scope
	VerdictForAgent(ctx context.Context, tenantID uint64, agentID string) (AgentSecurityVerdict, error)
	ReleaseAdmission(ctx context.Context, tenantID uint64, releaseID string) error
}
```

```go
// internal/application/service/agent_security.go（本任务交付判定面；操作面方法 Task 5 实现）
var (
	ErrAgentSecurityInvalidInput      = errors.New("invalid agent security revocation request")
	ErrAgentSecurityNotFound          = errors.New("agent security revocation not found")
	ErrAgentSecurityReleaseUnresolvable = errors.New("release is not resolvable in this tenant")
	ErrAgentSecurityReleaseBlocked    = errors.New("agent security policy blocked the release")
)

// ReleaseSecurityGate is the governance seam consumed by AgentAdoptionService
// and AgentUpgradeService (Task 6). Nil keeps those flows unchanged.
type ReleaseSecurityGate interface{ ReleaseAdmission(ctx context.Context, tenantID uint64, releaseID string) error }

type AgentSecurityService struct {
	store repository.AgentSecurityStore
	runs  *repository.AgentRunStore
	now   func() time.Time
}
func NewAgentSecurityService(store repository.AgentSecurityStore, runs *repository.AgentRunStore) *AgentSecurityService
```

判定算法（写死，测试钉住）：
- `VerdictForAgent(tenant, agentID)`：agentID 空 → ok；`store.VariantsByLocalAgent` 为空 → ok（非 adoption 派生 agent，含 builtin）；取首行 variant：① `store.ListReleaseRevocations` 中 `release_id == variant.ReleaseID` 的最新行（created_at 最大）→ `release-revoked`（Reason=`"release <id> security-revoked: <撤回 reason>"`、携带 RevocationID/ReplacementReleaseID）；② 否则 `store.ReleaseFacts` 取 lockJSON → `json.Unmarshal` 为 `types.DependencyLock` → 与 `store.ListDependencyRevocations` 逐条**四元组全等**比较（Type/ID/Version/Digest，大小写敏感字符串比较）→ 命中 → `dependency-blocked`（Reason=`"locked dependency <type>/<id>@<version> sha256:<digest> is security-revoked: <撤回 reason>"`、携带 Dependency 与 RevocationID/ReplacementVersion）；③ 否则 ok。锁 JSON 畸形 → 返回错误（存储缺陷 fail closed，不放行）。
- `ReleaseAdmission(tenant, releaseID)`：同 ①② 针对 releaseID 直接判定，blocked 时返回 `fmt.Errorf("%w: %s", ErrAgentSecurityReleaseBlocked, verdict.Reason)`，ok 返回 nil。

- [ ] **Step 1: 写失败测试**

`internal/application/service/agent_security_test.go`：

```go
package service

import (
	"context "context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

const securityManifest = `{"semantic_version":"%s","display_name":"Sec","summary":"s","supported_languages":["en"],"use_cases":["u"],"capability_requirements":[],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"v","version_number":1,"source_sha256":"sha"}}`

const securityBundle = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"p"},"manifest":` + securityManifest + `,"dependency_lock":{"dependencies":[]}}`

const lockV123 = `{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.3","digest":"D1","license_id":"MIT"}]}`
const lockV124 = `{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.4","digest":"D2","license_id":"MIT"}]}`
const lockV123OtherDigest = `{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.3","digest":"D9","license_id":"MIT"}]}`

// securityBundleWithLock 让 bundle 的 dependency_lock 与播种锁一致（bundle
// 字节只作迁移占位，判定面只读 releases.dependency_lock_json 列）。
func securityBundleWithLock(lock string) string {
	return `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"p"},"manifest":` + securityManifest + `,"dependency_lock":` + lock + `}`
}

func newAgentSecurityServiceForTest(t *testing.T) (*AgentSecurityService, *repository.AgentSecurityStore, *gorm.DB) {
	t.Helper()
	db := openAgentVersionServiceTestDB(t)
	store := repository.NewAgentSecurityStore(db)
	return NewAgentSecurityService(store, repository.NewAgentRunStore(db)), store, db
}

// publishSecurityVariant 经真实仓储播种一个 published Variant 并挂本地 agent。
func publishSecurityVariant(t *testing.T, db *gorm.DB, adoption *types.AgentAdoptionEntity, releaseID, name, localAgentID string) *types.AgentAdoptionVariantEntity {
	t.Helper()
	adoptions := repository.NewAgentAdoptionRepository(db)
	variant, err := adoptions.CreateVariant(context.Background(), &types.AgentAdoptionVariantEntity{
		TenantID: 1, AdoptionID: adoption.ID, ReleaseID: releaseID, Name: name, State: "draft", CreatedBy: "admin"})
	require.NoError(t, err)
	updated, err := adoptions.UpdateVariantState(context.Background(), 1, variant.ID, []string{"draft"}, "published", map[string]any{
		"local_agent_id": localAgentID, "local_agent_version_id": "ver-" + localAgentID, "published_by": "admin"})
	require.NoError(t, err)
	return updated
}

func TestAgentSecurityVerdictBlocksRevokedReleaseAndPassesUnaffected(t *testing.T) {
	svc, store, db := newAgentSecurityServiceForTest(t)
	listingID, r1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	_, r2 := publishUpgradeServiceRelease(t, db, 2, "2.0.0", securityManifest, lockV124, securityBundleWithLock(lockV124))
	adoption := adoptUpgradeRelease(t, db, listingID, r1)
	blockedVariant := publishSecurityVariant(t, db, adoption, r1, "On r1", "local-agent-r1")
	cleanVariant := publishSecurityVariant(t, db, adoption, r2, "On r2", "local-agent-r2")
	ctx := context.Background()

	verdict, err := svc.VerdictForAgent(ctx, 1, "local-agent-r1")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, verdict.State, "撤回前一切照常")
	require.NoError(t, svc.ReleaseAdmission(ctx, 1, r1))
	require.NoError(t, svc.ReleaseAdmission(ctx, 1, r2))

	require.NoError(t, store.AppendReleaseRevocation(ctx, &types.AgentReleaseRevocationEntity{
		TenantID: 1, ListingID: listingID, ReleaseID: r1,
		Reason: "CVE-2026-0001 prompt exfiltration", ReplacementReleaseID: r2, RevokedBy: "sec-admin"}))

	verdict, err = svc.VerdictForAgent(ctx, 1, "local-agent-r1")
	require.NoError(t, err)
	require.True(t, verdict.Blocked())
	require.Equal(t, interfaces.AgentSecurityVerdictReleaseRevoked, verdict.State)
	require.Contains(t, verdict.Reason, "CVE-2026-0001")
	require.Equal(t, r2, verdict.ReplacementReleaseID, "替代版本随判定可达")
	require.NotEmpty(t, verdict.RevocationID)

	require.ErrorIs(t, svc.ReleaseAdmission(ctx, 1, r1), ErrAgentSecurityReleaseBlocked, "撤回 Release 拒绝新引入/新变体（治理面准入）")

	clean, err := svc.VerdictForAgent(ctx, 1, "local-agent-r2")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, clean.State, "未受影响 Variant 照常")
	require.NoError(t, svc.ReleaseAdmission(ctx, 1, r2))

	nonAdoption, err := svc.VerdictForAgent(ctx, 1, "agent-a")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, nonAdoption.State, "非 adoption 派生 agent 不受治理")
	_ = blockedVariant
}

// AC1 核心断言：依赖撤回的匹配键是 (type,id,version,digest) 四元组，
// 不是名称——同名不同版本不阻断（不得借同名替换「自愈」），同名同版本
// 不同 digest 仍阻断（digest 是锁定身份的一部分）。
func TestAgentSecurityVerdictDependencyBlockedKeysOnExactLockedIdentity(t *testing.T) {
	svc, store, db := newAgentSecurityServiceForTest(t)
	listingID, r123 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	_, r124 := publishUpgradeServiceRelease(t, db, 2, "2.0.0", securityManifest, lockV124, securityBundleWithLock(lockV124))
	_, r123d9 := publishUpgradeServiceRelease(t, db, 3, "3.0.0", securityManifest, lockV123OtherDigest, securityBundleWithLock(lockV123OtherDigest))
	adoption := adoptUpgradeRelease(t, db, listingID, r123)
	publishSecurityVariant(t, db, adoption, r123, "v123", "local-agent-v123")
	publishSecurityVariant(t, db, adoption, r124, "v124", "local-agent-v124")
	publishSecurityVariant(t, db, adoption, r123d9, "v123d9", "local-agent-v123d9")
	ctx := context.Background()

	require.NoError(t, store.AppendDependencyRevocation(ctx, &types.AgentDependencyRevocationEntity{
		TenantID: 1, DepType: "skill", DepID: "web-search", DepVersion: "1.2.3", DepDigest: "D1",
		Reason: "malicious exfil in pinned skill content", ReplacementVersion: "1.2.4", RevokedBy: "sec-admin"}))

	blocked, err := svc.VerdictForAgent(ctx, 1, "local-agent-v123")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictDependencyBlocked, blocked.State, "锁定被撤回依赖的 Release 传递阻断")
	require.NotNil(t, blocked.Dependency)
	require.Equal(t, "1.2.3", blocked.Dependency.Version)
	require.Equal(t, "D1", blocked.Dependency.Digest)
	require.Equal(t, "1.2.4", blocked.ReplacementVersion)
	require.ErrorIs(t, svc.ReleaseAdmission(ctx, 1, r123), ErrAgentSecurityReleaseBlocked)

	sameNameNewVersion, err := svc.VerdictForAgent(ctx, 1, "local-agent-v124")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, sameNameNewVersion.State,
		"AC1：依赖不按名称自动替换——同名不同版本/摘要的新 Release 不被阻断也不被顶替")
	require.NoError(t, svc.ReleaseAdmission(ctx, 1, r124))

	sameVersionOtherDigest, err := svc.VerdictForAgent(ctx, 1, "local-agent-v123d9")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, sameVersionOtherDigest.State,
		"digest 是锁定身份的一部分：不同内容不受同名同版本撤回牵连")
}

func TestAgentSecurityVerdictCoversIntroducedRelease(t *testing.T) {
	svc, _, db := newAgentSecurityServiceForTest(t)
	ctx := context.Background()
	require.NoError(t, db.Exec(`INSERT INTO tenant_introduced_releases
		(id, tenant_id, public_listing_id, public_release_id, display_name, semantic_version,
		 bundle_digest, manifest_json, dependency_lock_json, bundle, introduced_by, introduced_at)
		VALUES ('intro-r1', 1, 'pub-listing-1', 'pub-rel-1', 'Introduced', '2.0.0', 'd-intro', '{}',
		 '{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.3","digest":"D1","license_id":"MIT"}]}',
		 '{}', 'admin', datetime('now'))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_adoptions (id, tenant_id, listing_id, accepted_release_id, state, created_by, created_at, updated_at)
		VALUES ('a-intro', 1, 'pub-listing-1', 'intro-r1', 'active', 'admin', datetime('now'), datetime('now'))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_adoption_variants (id, tenant_id, adoption_id, release_id, name, state, local_agent_id, created_by, created_at, updated_at)
		VALUES ('var-intro', 1, 'a-intro', 'intro-r1', 'Intro', 'published', 'local-agent-intro', 'admin', datetime('now'), datetime('now'))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_dependency_revocations
		(id, tenant_id, dep_type, dep_id, dep_version, dep_digest, reason, revoked_by, revoked_at, created_at)
		VALUES ('dep-rev-1', 1, 'skill', 'web-search', '1.2.3', 'D1', 'supply-chain revocation', 'sec-admin', datetime('now'), datetime('now'))`).Error)

	verdict, err := svc.VerdictForAgent(ctx, 1, "local-agent-intro")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictDependencyBlocked, verdict.State,
		"引入式 Release 的锁同样参与传播——只查 agent_releases 会把 #60 引入面漏成放行")
	require.ErrorIs(t, svc.ReleaseAdmission(ctx, 1, "intro-r1"), ErrAgentSecurityReleaseBlocked)

	foreign, err := svc.VerdictForAgent(ctx, 2, "local-agent-intro")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, foreign.State, "跨租户：撤回行不外溢")
}
```

（import 追加：`gorm.io/gorm`。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/service/ -run 'TestAgentSecurityVerdict' -count=1`
Expected: 编译失败——`NewAgentSecurityService` undefined。

- [ ] **Step 3: 实现 interfaces 文件与判定面**

`internal/types/interfaces/agent_security.go` 按 Interfaces 块写全部类型/常量/`Blocked()`；`internal/application/service/agent_security.go` 写服务结构、哨兵、`ReleaseSecurityGate` 接口、`VerdictForAgent`、`ReleaseAdmission`（算法按上文「判定算法」逐条）；`RevokeRelease/RevokeDependency/ListRevocations/GetRevocation` 本任务先返回 `ErrAgentSecurityInvalidInput` 占位（Task 5 RED 步会先写它们的失败测试再实现——避免本任务虚绿）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/application/service/ -run 'TestAgentSecurityVerdict' -count=1`
Expected: PASS（3 个测试）。

- [ ] **Step 5: Commit**

```bash
git add internal/types/interfaces/agent_security.go internal/application/service/agent_security.go internal/application/service/agent_security_test.go
git commit -m "feat(security): AgentSecurityService 判定面——四元组身份口径的撤回传播（T34 #64 Task 4）"
```

---

### Task 5: AgentSecurityService 操作面（撤回记录 + 审计 + 在途处置 + scope）

**Files:**
- Modify: `internal/application/service/agent_security.go`（实现操作面四方法）
- Test: `internal/application/service/agent_security_test.go`（追加）

**Interfaces:**
- Consumes: Task 1 审计常量与 `repository.NewAuditLogRepository(tx).Create`（internal/application/repository/audit_log.go:30——接受 tx 作用域的 db）；Task 2 store 的 Append/List/Get/UpdateCanceled/ReleaseFacts/ListTenantReleaseLocks/ListVariants；Task 3 `CancelRunsByAgents`；Task 4 的类型与哨兵。
- Produces: `interfaces.AgentSecurityService` 的完整实现（RevokeRelease/RevokeDependency/ListRevocations/GetRevocation），供 Task 6/7/8 与 e2e 消费。

实现语义（写死）：
- **校验**（落任何行之前）：`tenantID/actorID/reason` 非空；`InFlightDisposition ∈ {"", "cancel", "allow"}`（空→`cancel`，spec §10「运行中按风险暂停或终止」的 fail-safe 默认）；release 撤回：`ReleaseFacts` 必须 found（否则 `ErrAgentSecurityReleaseUnresolvable`），`ReplacementReleaseID` 非空时也必须 found 且 listing 一致；依赖撤回：`Dependency.Type/ID/Version/Digest` 四元组全非空。违反者返回 `ErrAgentSecurityInvalidInput` 且两表零新增行。
- **写入**：单事务 `{ Append 行 + `repository.NewAuditLogRepository(tx).Create(&types.AuditLog{...})` }`——审计行 Action 用 Task 1 常量，`TenantID/ActorUserID/RevokedBy/ScopeType:"marketplace"/TargetType:"agent_release"|"agent_dependency"/TargetID:release id 或 "type/id@version"`，`Details` JSON 带 `reason/replacement/in_flight_disposition`（**不得**带任何凭据类值），`CreatedAt=now`。任一失败整体回滚（撤回行与审计行同生）。
- **在途处置**（提交后）：`InFlightDisposition == "cancel"` 时——release 撤回：受影响 agent 集 = `ListVariants` 中 `ReleaseID == 目标` 且 `LocalAgentID != ""` 的 published Variant 的 local agent；依赖撤回：先 `ListTenantReleaseLocks` + 四元组匹配得 blocked release 集，再取 pinned 于该集的 published Variant 的 local agent——调 `runs.CancelRunsByAgents(ctx, tenant, agentIDs, "agent security revocation: "+reason)`，并把计数回写 `UpdateReleaseRevocationCanceled/UpdateDependencyRevocationCanceled`（处置结果可观测）。处置失败返回错误（撤回行已在，重试幂等：追加新行是显式新历史，测试钉住）。
- **scope**（`GetRevocation` 时现算，不冻结）：blocked releases（release 撤回=目标自身；依赖撤回=锁匹配集）、`ListVariants` 过滤 pinned 于 blocked 集的全部 Variant（含 draft/mapped/tested——影响范围如实）、去重 adoption id 集。

- [ ] **Step 1: 写失败测试**

追加到 `internal/application/service/agent_security_test.go`：

```go
func TestRevokeReleaseRecordsHistoryReasonScopeReplacementAndDisposesInFlight(t *testing.T) {
	svc, _, db := newAgentSecurityServiceForTest(t)
	listingID, r1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	_, r2 := publishUpgradeServiceRelease(t, db, 2, "2.0.0", securityManifest, `{"dependencies":[]}`, securityBundleWithLock(`{"dependencies":[]}`))
	adoption := adoptUpgradeRelease(t, db, listingID, r1)
	publishSecurityVariant(t, db, adoption, r1, "V1", "local-agent-r1")
	publishSecurityVariant(t, db, adoption, r2, "V2", "local-agent-r2")
	ctx := context.Background()

	// 在途 Run：r1 本地 agent 的活动执行（真实 agent_runs + 会话活动槽）。
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = 'sec-live-1' WHERE tenant_id = 1 AND id = 's1'`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, engine_type, status, snapshot, deadline) VALUES
		(1, 'sec-live-1', 's1', 'u1', 'req-1', 'm1', 'h1', 'trpc', 'running', ?, datetime('now','+1 hour'))`,
		`{"session_id":"s1","agent_id":"local-agent-r1","request_id":"req-1","text":"hi"}`).Error)

	view, err := svc.RevokeRelease(ctx, 1, "sec-admin", interfaces.ReleaseRevocationInput{
		ReleaseID: r1, Reason: "CVE-2026-0001 prompt exfiltration", ReplacementReleaseID: r2})
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityRevocationKindRelease, view.Kind)
	require.Equal(t, "CVE-2026-0001 prompt exfiltration", view.Reason)
	require.Equal(t, r2, view.ReplacementReleaseID)
	require.Equal(t, interfaces.AgentSecurityInFlightCancel, view.InFlightDisposition, "缺省在途处置 = cancel（fail-safe）")
	require.EqualValues(t, 1, view.CanceledRunCount, "在途执行按风险处置：默认终止")
	require.Equal(t, listingID, view.ListingID)

	// AC2：历史/原因/影响范围/替代版本。
	detail, err := svc.GetRevocation(ctx, 1, view.ID)
	require.NoError(t, err)
	require.NotNil(t, detail.Scope)
	require.Len(t, detail.Scope.BlockedReleases, 1)
	require.Equal(t, r1, detail.Scope.BlockedReleases[0].ReleaseID)
	require.Equal(t, []string{adoption.ID}, detail.Scope.AffectedAdoptionIDs)
	require.Len(t, detail.Scope.AffectedVariants, 1, "只影响 pinned 到被撤回 Release 的 Variant")
	require.Equal(t, "local-agent-r1", detail.Scope.AffectedVariants[0].LocalAgentID)

	var runStatus string
	require.NoError(t, db.Raw(`SELECT status FROM agent_runs WHERE tenant_id = 1 AND run_id = 'sec-live-1'`).Scan(&runStatus).Error)
	require.Equal(t, "canceled", runStatus, "在途 Run 已被撤销")
	var varCount int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM audit_logs WHERE tenant_id = 1 AND action = 'agent_security.release_revoked' AND actor_user_id = 'sec-admin'`).Scan(&varCount).Error)
	require.EqualValues(t, 1, varCount, "撤回必须留下审计行（与台账同事务）")

	// 撤回是「保留历史」而非删除：release/variant/adoption 行全部仍在。
	var releaseRows, variantRows int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_releases WHERE tenant_id = 1 AND id = ?`, r1).Scan(&releaseRows).Error)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_adoption_variants WHERE tenant_id = 1 AND release_id = ?`, r1).Scan(&variantRows).Error)
	require.EqualValues(t, 1, releaseRows)
	require.EqualValues(t, 1, variantRows)

	// 重复撤回 = 追加新历史行，旧行保留。
	second, err := svc.RevokeRelease(ctx, 1, "sec-admin-2", interfaces.ReleaseRevocationInput{
		ReleaseID: r1, Reason: "expanded scope", InFlightDisposition: interfaces.AgentSecurityInFlightAllow})
	require.NoError(t, err)
	require.EqualValues(t, 0, second.CanceledRunCount, "allow 处置不动在途（且已无在途）")
	history, err := svc.ListRevocations(ctx, 1)
	require.NoError(t, err)
	require.Len(t, history, 2, "append-only：两次撤回两条历史")
	require.Equal(t, view.ID, history[0].ID)
	require.Equal(t, second.ID, history[1].ID)
	require.Nil(t, history[0].Scope, "列表不带 scope（读取侧瘦身），单读才带")
}

func TestRevokeDependencyPropagatesByLockedIdentity(t *testing.T) {
	svc, _, db := newAgentSecurityServiceForTest(t)
	listingID, r123 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	_, r124 := publishUpgradeServiceRelease(t, db, 2, "2.0.0", securityManifest, lockV124, securityBundleWithLock(lockV124))
	adoption := adoptUpgradeRelease(t, db, listingID, r123)
	publishSecurityVariant(t, db, adoption, r123, "v123", "local-agent-v123")
	publishSecurityVariant(t, db, adoption, r124, "v124", "local-agent-v124")
	ctx := context.Background()

	view, err := svc.RevokeDependency(ctx, 1, "sec-admin", interfaces.DependencyRevocationInput{
		Dependency:          types.AgentReleaseDependency{Type: "skill", ID: "web-search", Version: "1.2.3", Digest: "D1", LicenseID: "MIT"},
		Reason:              "supply-chain compromise",
		ReplacementVersion:  "1.2.4",
	})
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityRevocationKindDependency, view.Kind)
	require.Equal(t, "1.2.4", view.ReplacementVersion)
	require.NotNil(t, view.Dependency)

	detail, err := svc.GetRevocation(ctx, 1, view.ID)
	require.NoError(t, err)
	require.Len(t, detail.Scope.BlockedReleases, 1, "传播只命中锁定该依赖的 Release（AC1：按身份不按名称）")
	require.Equal(t, r123, detail.Scope.BlockedReleases[0].ReleaseID)
	require.Equal(t, interfaces.AgentSecurityRevocationKindDependency, detail.Scope.BlockedReleases[0].BlockedBy)
	require.Len(t, detail.Scope.AffectedVariants, 1)

	require.ErrorIs(t, svc.ReleaseAdmission(ctx, 1, r123), ErrAgentSecurityReleaseBlocked)
	require.NoError(t, svc.ReleaseAdmission(ctx, 1, r124))

	var auditRows int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM audit_logs WHERE tenant_id = 1 AND action = 'agent_security.dependency_revoked'`).Scan(&auditRows).Error)
	require.EqualValues(t, 1, auditRows)

	_, err = svc.GetRevocation(ctx, 2, view.ID)
	require.ErrorIs(t, err, ErrAgentSecurityNotFound, "跨租户单读与不存在同形（404 语义）")
}

func TestRevokeRejectsMalformedInput(t *testing.T) {
	svc, _, db := newAgentSecurityServiceForTest(t)
	listingID, r1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	ctx := context.Background()

	_, err := svc.RevokeRelease(ctx, 1, "sec-admin", interfaces.ReleaseRevocationInput{ReleaseID: r1, Reason: "   "})
	require.ErrorIs(t, err, ErrAgentSecurityInvalidInput, "空 reason 拒绝")
	_, err = svc.RevokeRelease(ctx, 1, "sec-admin", interfaces.ReleaseRevocationInput{ReleaseID: "no-such-release", Reason: "x"})
	require.ErrorIs(t, err, ErrAgentSecurityReleaseUnresolvable, "不可解析 Release 拒绝")
	_, err = svc.RevokeRelease(ctx, 1, "sec-admin", interfaces.ReleaseRevocationInput{ReleaseID: r1, Reason: "x", InFlightDisposition: "maybe"})
	require.ErrorIs(t, err, ErrAgentSecurityInvalidInput, "未知在途处置拒绝")
	_, err = svc.RevokeDependency(ctx, 1, "sec-admin", interfaces.DependencyRevocationInput{
		Dependency: types.AgentReleaseDependency{Type: "skill", ID: "web-search", Version: "1.2.3"}, Reason: "x"})
	require.ErrorIs(t, err, ErrAgentSecurityInvalidInput, "依赖四元组不完整拒绝（digest 缺失）")

	var releaseRows, depRows int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_release_revocations WHERE tenant_id = 1`).Scan(&releaseRows).Error)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_dependency_revocations WHERE tenant_id = 1`).Scan(&depRows).Error)
	require.EqualValues(t, 0, releaseRows, "畸形输入零行落库")
	require.EqualValues(t, 0, depRows)
	_ = listingID
}

func TestRevokeReleaseInFlightAllowLeavesActiveRuns(t *testing.T) {
	svc, _, db := newAgentSecurityServiceForTest(t)
	listingID, r1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	adoption := adoptUpgradeRelease(t, db, listingID, r1)
	publishSecurityVariant(t, db, adoption, r1, "V1", "local-agent-r1")
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = 'sec-live-2' WHERE tenant_id = 1 AND id = 's1'`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, engine_type, status, snapshot, deadline) VALUES
		(1, 'sec-live-2', 's1', 'u1', 'req-2', 'm2', 'h2', 'trpc', 'running', ?, datetime('now','+1 hour'))`,
		`{"session_id":"s1","agent_id":"local-agent-r1","request_id":"req-2","text":"hi"}`).Error)

	view, err := svc.RevokeRelease(context.Background(), 1, "sec-admin", interfaces.ReleaseRevocationInput{
		ReleaseID: r1, Reason: "low-risk issue", InFlightDisposition: interfaces.AgentSecurityInFlightAllow})
	require.NoError(t, err)
	require.EqualValues(t, 0, view.CanceledRunCount)
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM agent_runs WHERE tenant_id = 1 AND run_id = 'sec-live-2'`).Scan(&status).Error)
	require.Equal(t, "running", status, "allow 处置：在途执行按风险保留")
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/service/ -run 'TestRevoke' -count=1`
Expected: FAIL——操作面还是 Task 4 的 `ErrAgentSecurityInvalidInput` 占位（合法输入也被拒）。

- [ ] **Step 3: 实现操作面四方法**

按「实现语义」逐条实现于 `internal/application/service/agent_security.go`。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/application/service/ -run 'TestAgentSecurity|TestRevoke' -count=1`
Expected: PASS（Task 4 的 3 个 + 本任务 4 个）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/agent_security.go internal/application/service/agent_security_test.go
git commit -m "feat(security): 撤回操作面——同事务审计、在途处置、影响范围投影（T34 #64 Task 5）"
```

---

### Task 6: 治理面闸门接线（Adopt / CreateVariant / PublishVariant / AcceptUpgradeProposal）

**Files:**
- Modify: `internal/application/service/agent_adoption.go`（struct 加 1 字段 + setter + 3 处 guard）
- Modify: `internal/application/service/agent_upgrade.go`（struct 加 1 字段 + setter + 1 处 guard）
- Test: `internal/application/service/agent_security_guard_test.go`

**Interfaces:**
- Consumes: Task 4 的 `ReleaseSecurityGate` 接口与 `ErrAgentSecurityReleaseBlocked`。
- Produces:

```go
// internal/application/service/agent_adoption.go 追加：
//   字段 releaseSecurityGate ReleaseSecurityGate（零值 nil = 闸门敞开，既有行为不变）
func (s *AgentAdoptionService) SetReleaseSecurityGate(gate ReleaseSecurityGate)
```

```go
// internal/application/service/agent_upgrade.go 追加（同款）：
func (s *AgentUpgradeService) SetReleaseSecurityGate(gate ReleaseSecurityGate)
```

guard 插入位置（写死，均为「入参校验之后、任何读/写之前」）：
- `Adopt`：在 `listing == nil` 判空之后、`releaseID` 默认化之前改为——解析 `releaseID`（默认 `*listing.CurrentReleaseID`）→ `s.releaseSecurityGate != nil` 时 `ReleaseAdmission(ctx, tenantID, releaseID)` → err 非 nil 直接返回（CONTEXT.md:185「禁止该 Release 被新引入」；注意：被撤回的 current release 也阻断默认 Adopt，spec §10 新 Adoption=否）。
- `CreateVariant`：`adoption` 判空与状态检查之后、`releaseID` 默认化之后同样 guard（「用于发布新变体」被禁）。
- `PublishVariant`：方法体最前（输入校验后）guard `variant` 尚未读——改为在 `variantWithMissing` 取得 variant 之后、`missing` 检查之前 guard `variant.ReleaseID`（顺序不影响语义：发布前必读 variant；此处为最早可得 releaseID 的位置）。
- `AgentUpgradeService.AcceptUpgradeProposal`：在 `toRelease` 非 nil 且 listing 归属校验之后 guard `row.ToReleaseID`（接受被撤回 Release 的建议被拒）。

- [ ] **Step 1: 写失败测试**

`internal/application/service/agent_security_guard_test.go`：

```go
package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type stubReleaseSecurityGate struct{ blockedRelease string }

func (g stubReleaseSecurityGate) ReleaseAdmission(_ context.Context, _ uint64, releaseID string) error {
	if releaseID == g.blockedRelease {
		return fmt.Errorf("%w: release %s is security-revoked", ErrAgentSecurityReleaseBlocked, releaseID)
	}
	return nil
}

func TestAdoptionServiceReleaseSecurityGateRefusesAdoptVariantPublish(t *testing.T) {
	svc, db, _ := newAgentAdoptionServiceForTest(t)
	listingID, r1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	ctx := context.Background()

	// nil gate（默认）：既有行为不变——Adopt 成功。
	before, _, err := svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.NoError(t, err)
	require.NotEmpty(t, before.ID)

	svc.SetReleaseSecurityGate(stubReleaseSecurityGate{blockedRelease: r1})
	_, _, err = svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked, "被撤回 Release 拒绝新引入（显式指定）")

	// 用第二租户视角换一个干净服务实例种一个 adoption 后再装闸门，
	// 避免上面 409 干扰：直接以仓储种 adoption + draft variant。
	adoptions := repository.NewAgentAdoptionRepository(db)
	adoption, _, err := adoptions.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: r1, State: "active", CreatedBy: "admin"})
	require.NoError(t, err)

	_, err = svc.CreateVariant(ctx, 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "Blocked"})
	require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked, "被撤回 Release 拒绝新 Variant 草稿（默认 accepted release）")

	variant, err := adoptions.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adoption.ID, ReleaseID: r1, Name: "Pre-gate", State: "tested", CreatedBy: "admin"})
	require.NoError(t, err)
	_, err = svc.PublishVariant(ctx, 1, "admin", variant.ID)
	require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked, "被撤回 Release 拒绝发布本地变体（新本地 agent = 新运行面）")
}

func TestUpgradeServiceReleaseSecurityGateRefusesAccept(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, `{"dependencies":[]}`, securityBundleWithLock(`{"dependencies":[]}`))
	adoptUpgradeRelease(t, db, listingID, v1)
	_, v2 := publishUpgradeServiceRelease(t, db, 2, "2.0.0", securityManifest, `{"dependencies":[]}`, securityBundleWithLock(`{"dependencies":[]}`))
	ctx := context.Background()

	proposals, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)

	svc.SetReleaseSecurityGate(stubReleaseSecurityGate{blockedRelease: v2})
	_, _, err = svc.AcceptUpgradeProposal(ctx, 1, "admin", proposals[0].ID, interfaces.UpgradeVariantInput{Name: "Up"})
	require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked, "接受指向被撤回 Release 的升级建议被拒")
}
```

（import 需含 `github.com/Tencent/WeKnora/internal/application/repository`。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/service/ -run 'TestAdoptionServiceReleaseSecurityGate|TestUpgradeServiceReleaseSecurityGate' -count=1`
Expected: 编译失败——`svc.SetReleaseSecurityGate undefined`。

- [ ] **Step 3: 实现两处 setter 与 4 处 guard**

按 Interfaces 块的插入位置说明修改 `agent_adoption.go` / `agent_upgrade.go`。

- [ ] **Step 4: 运行确认通过 + 既有回归**

Run: `go test ./internal/application/service/ -run 'TestAdoptionServiceReleaseSecurityGate|TestUpgradeServiceReleaseSecurityGate|TestAgentAdoption|TestAgentUpgrade' -count=1`
Expected: PASS（nil gate 下既有 adoption/upgrade 测试零回归）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/agent_adoption.go internal/application/service/agent_upgrade.go internal/application/service/agent_security_guard_test.go
git commit -m "feat(security): Adoption/Upgrade 治理面挂接 Release 安全闸门（T34 #64 Task 6）"
```

---

### Task 7: HTTP wire + 容器装配 + wiring 源级断言

**Files:**
- Create: `internal/handler/agent_security.go`
- Create: `internal/router/routes_agent_security.go`
- Create: `internal/container/agent_security.go`
- Modify: `internal/router/router.go`（RouterParams 加 `AgentSecurityHandler *handler.AgentSecurityHandler` 字段（:107-108 邻域）+ `RegisterAgentSecurityRoutes(v1, params.AgentSecurityHandler, rbacGuards)`（:429-430 邻域））
- Modify: `internal/container/container.go`（adoption/upgrade 的 Provide 改为「Concrete + interface 适配」两段；追加 2 Provide + 2 Invoke）
- Test: `internal/handler/agent_security_test.go`、`internal/container/agent_security_wiring_test.go`

**Interfaces:**
- Consumes: Task 4/5 的 `interfaces.AgentSecurityService`；既有 handler 包工具 `decodeAgentMarketplaceBody` / `invalidMarketplaceBody` / `agentMarketplaceMaxRequestBytes`（internal/handler/agent_marketplace.go:21/153/169）与 `sandboxConfigTenantID`（internal/handler/sandbox_config.go:92）；既有 Task 6 的 `SetReleaseSecurityGate`。
- Produces:

```go
// internal/handler/agent_security.go
type AgentSecurityHandler struct{ security interfaces.AgentSecurityService }
func NewAgentSecurityHandler(security interfaces.AgentSecurityService) *AgentSecurityHandler
func (h *AgentSecurityHandler) RevokeRelease(c *gin.Context)     // POST /api/v1/marketplace/tenant/security-revocations/releases
func (h *AgentSecurityHandler) RevokeDependency(c *gin.Context)  // POST /api/v1/marketplace/tenant/security-revocations/dependencies
func (h *AgentSecurityHandler) ListRevocations(c *gin.Context)   // GET  /api/v1/marketplace/tenant/security-revocations
func (h *AgentSecurityHandler) GetRevocation(c *gin.Context)     // GET  /api/v1/marketplace/tenant/security-revocations/:id
func securityClientError(err error) error // ErrAgentSecurityInvalidInput/ReleaseUnresolvable→400 NewValidationError；ErrAgentSecurityNotFound→404；其余透传
```

请求体（`MaxBytesReader` 限 `agentMarketplaceMaxRequestBytes`，`decodeAgentMarketplaceBody` 严格解码拒绝多余字段）：

```go
type revokeReleaseBody struct {
	ReleaseID            string `json:"release_id"`
	Reason               string `json:"reason"`
	ReplacementReleaseID string `json:"replacement_release_id,omitempty"`
	InFlightDisposition  string `json:"in_flight_disposition,omitempty"`
}
type revokeDependencyBody struct {
	Dependency          *agentSecurityDependencyBody `json:"dependency"`
	Reason              string                       `json:"reason"`
	ReplacementVersion  string                       `json:"replacement_version,omitempty"`
	InFlightDisposition string                       `json:"in_flight_disposition,omitempty"`
}
type agentSecurityDependencyBody struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}
```

响应信封与 Adoption 面同款：`c.JSON(http.StatusCreated, gin.H{"success": true, "data": <view>})` / 200（GET）。

```go
// internal/router/routes_agent_security.go —— 四端点全部 Admin+ full-access
// （spec §13 security_revoke_release 治理写面；列表/单读同为审计材料）。
// nil handler 挂零路由（fail closed，RegisterAgentUpgradeRoutes 同款）。
func RegisterAgentSecurityRoutes(r *gin.RouterGroup, securityHandler *handler.AgentSecurityHandler, g *rbacGuards)
```

```go
// internal/container/agent_security.go
func NewAgentSecurityService(store *repository.AgentSecurityStore, runs *repository.AgentRunStore) *service.AgentSecurityService
func NewAgentSecurityHandler(security *service.AgentSecurityService) *handler.AgentSecurityHandler
func wireAgentSecurityGates(adoption *service.AgentAdoptionService, upgrade *service.AgentUpgradeService, security *service.AgentSecurityService) // security 非 nil 时对两者 SetReleaseSecurityGate
```

container.go 变更（写死）：
1. :452-454 的 adoption Provide 替换为：
```go
	must(container.Provide(func(repo repository.AgentAdoptionRepository, agents interfaces.CustomAgentService, versions interfaces.AgentVersionService) *service.AgentAdoptionService {
		return service.NewAgentAdoptionService(repo, agents, versions)
	}))
	must(container.Provide(func(adoption *service.AgentAdoptionService) interfaces.AgentAdoptionService { return adoption }))
```
2. :456-458 的 upgrade Provide 同款替换为 Concrete + 适配两段。
3. 追加：`must(container.Provide(NewAgentSecurityService))`、`must(container.Provide(NewAgentSecurityHandler))`、`must(container.Invoke(wireAgentSecurityGates))`。`wireAgentChatSecurityGate` 和它的 Invoke 延后到 Task 8：当前 session.Handler 尚无 AgentSecurityGate setter，提前注册会使 Task 7 无法独立构建。
4. 本任务不改 `internal/container/workbench.go`。Task 8 将在保留当前 `adoptions` 参数和 `SetAgentUseGate` 的前提下，扩展 workbench 装配。

- [ ] **Step 1: 写失败测试（handler wire）**

`internal/handler/agent_security_test.go`：

```go
package handler

// T34 (#64) Task 7: 撤回端点的 wire 契约——信封、错误映射、严格解码。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubSecurityService struct {
	interfaces.AgentSecurityService
	revokeView interfaces.AgentSecurityRevocationView
	revokeErr  error
	getView    interfaces.AgentSecurityRevocationView
	getErr     error
	listRows   []interfaces.AgentSecurityRevocationView
}

func (s *stubSecurityService) RevokeRelease(_ context.Context, _ uint64, _ string, _ interfaces.ReleaseRevocationInput) (interfaces.AgentSecurityRevocationView, error) {
	return s.revokeView, s.revokeErr
}
func (s *stubSecurityService) RevokeDependency(_ context.Context, _ uint64, _ string, _ interfaces.DependencyRevocationInput) (interfaces.AgentSecurityRevocationView, error) {
	return s.revokeView, s.revokeErr
}
func (s *stubSecurityService) ListRevocations(_ context.Context, _ uint64) ([]interfaces.AgentSecurityRevocationView, error) {
	return s.listRows, nil
}
func (s *stubSecurityService) GetRevocation(_ context.Context, _ uint64, _ string) (interfaces.AgentSecurityRevocationView, error) {
	return s.getView, s.getErr
}

func securityHandlerContext(t *testing.T, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	c.Request = httptest.NewRequest(method, path, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "rev-1"}}
	return c, rec
}

func TestRevokeReleaseWireEnvelope(t *testing.T) {
	h := NewAgentSecurityHandler(&stubSecurityService{revokeView: interfaces.AgentSecurityRevocationView{
		ID: "rev-1", Kind: "release", Reason: "CVE", RevokedBy: "sec-admin", RevokedAt: time.Now().UTC(),
		InFlightDisposition: "cancel", CanceledRunCount: 2, ReleaseID: "r1", ReplacementReleaseID: "r2",
	}})
	c, rec := securityHandlerContext(t, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases",
		`{"release_id":"r1","reason":"CVE","replacement_release_id":"r2"}`)
	h.RevokeRelease(c)

	require.Equal(t, http.StatusCreated, c.Writer.Status())
	var body struct {
		Success bool                                   `json:"success"`
		Data    map[string]interface{}                 `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "rev-1", body.Data["id"])
	require.Equal(t, "release", body.Data["kind"])
	require.Equal(t, "r2", body.Data["replacement_release_id"])
	require.EqualValues(t, 2, body.Data["canceled_run_count"])
}

func TestRevokeWireRejectsMalformedBody(t *testing.T) {
	h := NewAgentSecurityHandler(&stubSecurityService{revokeErr: service.ErrAgentSecurityInvalidInput})
	c, _ := securityHandlerContext(t, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", `{"reason":"x"}`)
	h.RevokeRelease(c)
	require.Equal(t, http.StatusBadRequest, c.Writer.Status())

	h2 := NewAgentSecurityHandler(&stubSecurityService{revokeErr: service.ErrAgentSecurityReleaseUnresolvable})
	c2, _ := securityHandlerContext(t, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", `{"release_id":"nope","reason":"x"}`)
	h2.RevokeRelease(c2)
	require.Equal(t, http.StatusBadRequest, c2.Writer.Status(), "不可解析 Release = 400")

	h3 := NewAgentSecurityHandler(&stubSecurityService{getErr: service.ErrAgentSecurityNotFound})
	c3, _ := securityHandlerContext(t, http.MethodGet, "/api/v1/marketplace/tenant/security-revocations/rev-1", "")
	h3.GetRevocation(c3)
	require.Equal(t, http.StatusNotFound, c3.Writer.Status(), "跨租户/不存在同形 404")

	// 多余字段被严格解码拒绝。
	h4 := NewAgentSecurityHandler(&stubSecurityService{})
	c4, _ := securityHandlerContext(t, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", `{"release_id":"r1","reason":"x","extra":1}`)
	h4.RevokeRelease(c4)
	require.Equal(t, http.StatusBadRequest, c4.Writer.Status(), "未知字段拒绝（spoof 面收敛）")
}
```

（import 需含 `encoding/json`。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/handler/ -run 'TestRevoke' -count=1`
Expected: 编译失败——`NewAgentSecurityHandler` undefined。

- [ ] **Step 3: 实现 handler、routes、container**

按 Interfaces 块实现 `internal/handler/agent_security.go`、`internal/router/routes_agent_security.go`（`admin := apiKeyFullAccess()`；四端点 `g.apiKeyRoute(r, method, path, admin, g.Admin(), handler.X)`；nil handler return）、`internal/container/agent_security.go` 与 container.go/router.go 变更（workbench.go 的参数追加**推迟到 Task 8**，本任务不触碰 workbench.go）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/handler/ -run 'TestRevoke' -count=1 && go build ./...`
Expected: PASS 且全仓构建通过。

- [ ] **Step 5: wiring 源级断言（RED→GREEN 内联执行）**

先写 `internal/container/agent_security_wiring_test.go`（task_compliance_wiring_test.go 判例）并确认 FAIL（container.go 尚无对应注册），再补 container.go 注册使其 PASS：

```go
package container

// T34 (#64) Task 7: wiring-guard——安全撤回 handler 是可选 RouterParams 字段、
// 治理闸门是 Invoke 侧装配件：漏注册不会编译失败，只会让路由/闸门静默消失
//（task_compliance_wiring_test.go 同款防线）。
import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentSecurityWiringRegistered(t *testing.T) {
	containerSrc := readRepoFile(t, "container.go")
	for _, want := range []string{
		"must(container.Provide(NewAgentSecurityService))",
		"must(container.Provide(NewAgentSecurityHandler))",
		"must(container.Invoke(wireAgentSecurityGates))",
	} {
		require.Truef(t, strings.Contains(containerSrc, want),
			"container.go 必须注册 %q——漏注册会让撤回路由/治理闸门静默消失", want)
	}
	routerSrc := readRepoFile(t, "../router/router.go")
	require.True(t, strings.Contains(routerSrc, "RegisterAgentSecurityRoutes(v1, params.AgentSecurityHandler, rbacGuards)"),
		"router.go 必须挂载安全撤回路由")
}
```

Run: `go test ./internal/container/ -run TestAgentSecurityWiringRegistered -count=1` → 先 FAIL → 补齐 container.go 注册；本 Task 的断言范围只有 container.go 与 router.go。

Run: `go test ./internal/container/ -run TestAgentSecurityWiringRegistered -count=1`
Expected: PASS. This Task 7 assertion covers only the production Container and Router registrations. Task 8 extends the same assertion after the workbench/session gate setters exist.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/agent_security.go internal/handler/agent_security_test.go internal/router/routes_agent_security.go internal/router/router.go internal/container/agent_security.go internal/container/agent_security_wiring_test.go internal/container/container.go
git commit -m "feat(security): /marketplace/tenant/security-revocations 四端点与容器装配（T34 #64 Task 7）"
```

---

### Task 8: 运行入口闸门（workbench 准入 + agent-chat 轮次）

#### Task 8 architecture amendment — required before implementation

The initial Task 8 sketch below described a plain `VerdictForAgent` precheck. Read-only audit at coordination HEAD `615c015c0` proved that design is unsafe: it races revocation, breaks idempotent replay if placed before request lookup, and in AgentQA runs after request parsing can persist image attachments. Do not implement that precheck.

The implementation must preserve these ordering invariants:

1. For workbench, keep the existing request-hash/existing-request replay path before any new security decision. A previously admitted request returns its existing result under current idempotency rules.
2. A **new** Run admission must make its security decision and durable Run admission under the same tenant guard used by revocation (`withTenantSecurityGuard`), with tenant lock acquired before session/request/run rows. Reuse `checkReleaseAdmissionTx` for exact Release/dependency identity. Do not hold the guard across external budget or network work. If revocation commits first, a new Run is denied; if admission commits first, the existing in-flight disposition policy applies.
3. AgentQA must resolve the applicable custom Agent without writing files or rows, and reject blocked agents before attachment persistence, message writes, live-run allocation, or SSE. Cover quick-answer mode as well as agent mode. Do not keep a tenant lock open across storage extraction or streaming; the implementation must use a durable admission/claim seam whose state participates in revocation ordering and has explicit cleanup on downstream failure.
4. Task 8 preserves `NewWorkbenchAdmissionCoordinator`'s current `adoptions` parameter and `SetAgentUseGate` retirement guard while adding security wiring.

The repository-level transaction seam and AgentQA turn-claim lifecycle require a separate detailed Task8 plan before any Task8 Brief or implementation dispatch. The architecture evidence and blocking findings are recorded in `evidence/t64-task7-task8-preflight.md` and `B6-execution-ledger.md`. Do not implement the pseudocode below literally. Task 8 follows the reviewed schedule in `plan-t64-task8-atomic-admission.md`: 8A exact Version-aware guarded predicate, then 8B durable claim schema/store plus atomic revocation cancellation; both are serial prerequisites and each requires its own Brief, owned files, Review Package, independent review and validation. After 8B is integrated, 8C Run admission and 8D AgentQA handler claim adoption may run concurrently in separate Worktrees with non-overlapping owned files and isolated test databases. 8E Workbench replay/settlement follows verified 8C. Task9 remains downstream of verified 8C–8E. The initial file/interface sketch below is superseded by the linked detailed plan and must not be dispatched.

**Files:**
- Modify: `internal/modules/workbench/service/workbench/admission.go`（字段 + setter + Start guard + 哨兵）
- Modify: `internal/handler/session/handler.go`（字段 + 接口 + setter）
- Create: `internal/handler/session/agent_security_gate.go`（guardAgentSecurity helper）
- Modify: `internal/handler/session/qa.go`（AgentQA 内 1 处 guard 调用）
- Modify: `internal/handler/session/workbench_start.go`（writeWorkbenchAdmissionError 加 case）
- Modify: `internal/handler/agent_adoption.go`（adoptionClientError 加 case）、`internal/handler/agent_upgrade.go`（upgradeClientError 加 case）
- Modify: `internal/container/workbench.go`（NewWorkbenchAdmissionCoordinator 加参 + 安装）
- Test: `internal/modules/workbench/service/workbench/admission_agent_security_test.go`、`internal/handler/session/agent_security_gate_test.go`

**Interfaces:**
- Consumes: Task 4 `interfaces.AgentSecurityVerdict` 与 `Blocked()`；W34 gate 先例（admission.go:148-162、287-295——「consulted BEFORE identity, budget reservation or any durable write」）；`SetTaskDeletionGuard` 先例（handler.go:125-150——nil-safe + infra 错误 fail closed）；`workbenchservice.ErrRequestRejected`→409 映射先例（workbench_start.go:76-93）。
- Produces:

```go
// internal/modules/workbench/service/workbench/admission.go 追加：
var ErrAgentSecurityBlocked = errors.New("agent security policy refused the admission")

type AgentSecurityGate interface {
	VerdictForAgent(ctx context.Context, tenantID uint64, agentID string) (interfaces.AgentSecurityVerdict, error)
}
func (a *AdmissionCoordinator) SetAgentSecurityGate(gate AgentSecurityGate) // nil = 敞开（既有行为）
```

Start 内 guard（写死位置：`if in.TargetID == "" { in.TargetID = "platform" }` 之后、`hash := requestHash(in)` 之前——即 `CreatePending` 第一个 durable write 之前）：

```go
	// T34 (#64): the agent security gate runs BEFORE the first durable write
	// (requests.CreatePending) — a security-revoked Release or a
	// dependency-blocked lock refuses NEW admission with zero side effects,
	// mirroring the W34 capability gate's contract. Empty AgentID
	// (builtin/agentless runs) stays open. A gate infrastructure failure
	// fails closed: the raw error surfaces and no admission happens.
	if agentID := strings.TrimSpace(in.AgentID); agentID != "" && a.agentSecurityGate != nil {
		verdict, err := a.agentSecurityGate.VerdictForAgent(ctx, tenant, agentID)
		if err != nil {
			return agentruntime.Run{}, err
		}
		if verdict.Blocked() {
			return agentruntime.Run{}, fmt.Errorf("%w: %s", ErrAgentSecurityBlocked, verdict.Reason)
		}
	}
```

```go
// internal/handler/session/handler.go 追加（taskDeletionGuard 邻域）：
type AgentSecurityGate interface {
	VerdictForAgent(ctx context.Context, tenantID uint64, agentID string) (interfaces.AgentSecurityVerdict, error)
}
func (h *Handler) SetAgentSecurityGate(gate AgentSecurityGate) // 字段 agentSecurityGate AgentSecurityGate；nil = 敞开
```

```go
// internal/handler/session/agent_security_gate.go（完整实现——错误语义是本任务的判定点）：
// guardAgentSecurity consults the T34 (#64) gate before any message row,
// live-run slot or SSE byte is produced. Blocked → 409 carrying the verdict
// reason; an infrastructure failure fails closed (500) instead of letting a
// possibly-revoked agent run on a broken check. Nil gate / nil customAgent /
// missing tenant keep the flow unchanged (SetTaskDeletionGuard precedent).
func (h *Handler) guardAgentSecurity(c *gin.Context, reqCtx *qaRequestContext) bool {
	if h == nil || h.agentSecurityGate == nil || reqCtx == nil || reqCtx.customAgent == nil {
		return true
	}
	tenant, ok := types.TenantIDFromContext(reqCtx.ctx)
	if !ok || tenant == 0 {
		return true
	}
	verdict, err := h.agentSecurityGate.VerdictForAgent(reqCtx.ctx, tenant, reqCtx.customAgent.ID)
	if err != nil {
		c.Error(errors.NewInternalServerError("agent security check failed: " + err.Error()))
		return false
	}
	if verdict.Blocked() {
		c.Error(errors.NewConflictError("agent security policy refused this agent: " + verdict.Reason))
		return false
	}
	return true
}
```

qa.go 的 `AgentQA`（qa.go:919）在 sanity-gate 块之后、`if agentModeEnabled { h.executeQA(...) }` 之前插入：

```go
	// T34 (#64): refuse the turn BEFORE any message row, live-run slot or SSE
	// byte is produced when the resolved agent's Release is security-revoked
	// or its locked dependency is security-blocked.
	if agentModeEnabled && !h.guardAgentSecurity(c, reqCtx) {
		return
	}
```

错误映射（各 1 个 case）：
- `workbench_start.go` writeWorkbenchAdmissionError：`case errors.Is(err, workbenchservice.ErrAgentSecurityBlocked): status = http.StatusConflict`。
- `handler/agent_adoption.go` adoptionClientError：`case stderrors.Is(err, marketservice.ErrAgentSecurityReleaseBlocked): return apperrors.NewConflictError(err.Error())`。
- `handler/agent_upgrade.go` upgradeClientError：同款 case。

`internal/container/workbench.go`：`NewWorkbenchAdmissionCoordinator(cfg *config.Config, db *gorm.DB, runs *repository.AgentRunStore, targets repository.ExecutionTargetStore, security *service.AgentSecurityService)`，`coordinator.SetAdmissionGate(...)` 之后追加 `coordinator.SetAgentSecurityGate(security)`（nil-safe：setter 内判 nil）。

- [ ] **Step 1: 写失败测试（workbench 准入）**

`internal/modules/workbench/service/workbench/admission_agent_security_test.go`：

```go
package workbench

// T34 (#64) Task 8: 准入闸门——拒绝发生在第一个 durable write 之前、
// 基础设施故障 fail closed、空 AgentID 与 ok verdict 照常准入。

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type stubAdmissionSecurityGate struct {
	verdict interfaces.AgentSecurityVerdict
	err     error
}

func (g stubAdmissionSecurityGate) VerdictForAgent(context.Context, uint64, string) (interfaces.AgentSecurityVerdict, error) {
	return g.verdict, g.err
}

func TestAdmissionAgentSecurityGateBlocksBeforeAnyDurableWrite(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	coordinator, err := NewAdmissionCoordinatorWithBinding(db, repository.NewAgentRunStore(db), NoopTaskBudget{}, nil, NewDatabaseAdmissionBindingResolver(nil))
	require.NoError(t, err)
	coordinator.SetAgentSecurityGate(stubAdmissionSecurityGate{verdict: interfaces.AgentSecurityVerdict{
		State:  interfaces.AgentSecurityVerdictReleaseRevoked,
		Reason: "release r1 security-revoked: CVE-2026-0001"}})
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")

	_, err = coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "local-agent-blocked", TargetID: "platform", RequestID: "sec-1", Text: "hello", BudgetUpper: 100})
	require.ErrorIs(t, err, ErrAgentSecurityBlocked)

	var pending, runRows int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM workbench_requests WHERE tenant_id = 1 AND actor_id = 'u1'`).Scan(&pending).Error)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_runs WHERE tenant_id = 1`).Scan(&runRows).Error)
	require.EqualValues(t, 0, pending, "拒绝发生在 workbench_requests 第一个 durable write 之前")
	require.EqualValues(t, 0, runRows, "拒绝不得创建任何 Run（queue_next 重启派发同经此缝）")

	// 对照 1：ok verdict 照常准入。
	coordinator.SetAgentSecurityGate(stubAdmissionSecurityGate{verdict: interfaces.AgentSecurityVerdict{State: interfaces.AgentSecurityVerdictOK}})
	allowed, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "local-agent-blocked", TargetID: "platform", RequestID: "sec-2", Text: "hello", BudgetUpper: 100})
	require.NoError(t, err)
	require.NotEmpty(t, allowed.Key.RunID)

	// 对照 2：空 AgentID（builtin/agentless）不咨询闸门——换回 blocked verdict 仍放行。
	coordinator.SetAgentSecurityGate(stubAdmissionSecurityGate{verdict: interfaces.AgentSecurityVerdict{State: interfaces.AgentSecurityVerdictDependencyBlocked}})
	agentless, err := coordinator.Start(ctx, StartInput{SessionID: "s1", TargetID: "platform", RequestID: "sec-3", Text: "hello", BudgetUpper: 100})
	require.NoError(t, err)
	require.NotEmpty(t, agentless.Key.RunID)
}

func TestAdmissionAgentSecurityGateFailsClosedOnInfrastructureError(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	coordinator, err := NewAdmissionCoordinatorWithBinding(db, repository.NewAgentRunStore(db), NoopTaskBudget{}, nil, NewDatabaseAdmissionBindingResolver(nil))
	require.NoError(t, err)
	coordinator.SetAgentSecurityGate(stubAdmissionSecurityGate{err: errors.New("store is down")})
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")

	_, err = coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "local-agent-blocked", TargetID: "platform", RequestID: "sec-4", Text: "hello", BudgetUpper: 100})
	require.Error(t, err, "闸门基础设施故障时 fail closed——绝不放行")
	var pending int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM workbench_requests WHERE tenant_id = 1 AND actor_id = 'u1'`).Scan(&pending).Error)
	require.EqualValues(t, 0, pending)
}
```

（import 需含 `github.com/Tencent/WeKnora/internal/application/repository`；`openAdmissionConcurrencyDB` 已种 tenant 1/u1/s1。）

- [ ] **Step 2: 写失败测试（agent-chat 闸门单元）**

`internal/handler/session/agent_security_gate_test.go`：

```go
package session

// T34 (#64) Task 8: agent-chat 轮次闸门单元——blocked 409、infra 500、
// nil gate/无 agent 放行（SetTaskDeletionGuard 判例同款断言面）。

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubAgentSecurityGate struct {
	verdict interfaces.AgentSecurityVerdict
	err     error
}

func (g stubAgentSecurityGate) VerdictForAgent(context.Context, uint64, string) (interfaces.AgentSecurityVerdict, error) {
	return g.verdict, g.err
}

func securityGateTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, context.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/agent-chat/s1", nil)
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
	c.Request = req.WithContext(ctx)
	return c, rec, ctx
}

func TestGuardAgentSecurityBlockedRefusesWithConflict(t *testing.T) {
	c, _, ctx := securityGateTestContext(t)
	h := &Handler{agentSecurityGate: stubAgentSecurityGate{verdict: interfaces.AgentSecurityVerdict{
		State:  interfaces.AgentSecurityVerdictDependencyBlocked,
		Reason: "locked dependency skill/web-search@1.2.3 is security-revoked"}}}
	require.False(t, h.guardAgentSecurity(c, &qaRequestContext{ctx: ctx, customAgent: &types.CustomAgent{ID: "local-agent-1"}}))
	require.Len(t, c.Errors, 1)
	appErr, ok := errors.IsAppError(c.Errors.Last().Err)
	require.True(t, ok)
	require.Equal(t, http.StatusConflict, appErr.HTTPCode)
	require.Contains(t, appErr.Message, "security-revoked")
}

func TestGuardAgentSecurityFailsClosedOnInfrastructureError(t *testing.T) {
	c, _, ctx := securityGateTestContext(t)
	h := &Handler{agentSecurityGate: stubAgentSecurityGate{err: stderrors.New("store down")}}
	require.False(t, h.guardAgentSecurity(c, &qaRequestContext{ctx: ctx, customAgent: &types.CustomAgent{ID: "local-agent-1"}}))
	appErr, ok := errors.IsAppError(c.Errors.Last().Err)
	require.True(t, ok)
	require.Equal(t, http.StatusInternalServerError, appErr.HTTPCode, "基础设施故障 fail closed（500），绝不放行")
}

func TestGuardAgentSecurityNilGateAndNoAgentStayOpen(t *testing.T) {
	c, _, ctx := securityGateTestContext(t)
	open := &Handler{}
	require.True(t, open.guardAgentSecurity(c, &qaRequestContext{ctx: ctx, customAgent: &types.CustomAgent{ID: "a"}}), "nil gate 保持既有行为")

	gated := &Handler{agentSecurityGate: stubAgentSecurityGate{verdict: interfaces.AgentSecurityVerdict{
		State: interfaces.AgentSecurityVerdictReleaseRevoked}}}
	require.True(t, gated.guardAgentSecurity(c, &qaRequestContext{ctx: ctx}), "无 customAgent（普通问答轮）不咨询闸门")
}
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmissionAgentSecurity' -count=1 && go test ./internal/handler/session/ -run 'TestGuardAgentSecurity' -count=1`
Expected: 编译失败——`SetAgentSecurityGate` / `guardAgentSecurity` undefined。

- [ ] **Step 4: 实现全部闸门与映射**

按 Interfaces 块实现 admission.go（字段/setter/哨兵/Start guard）、handler.go（字段/接口/setter）、agent_security_gate.go、qa.go 插入、workbench_start.go / agent_adoption.go / agent_upgrade.go 三个 case、workbench.go 参数与安装；随后在 `internal/container/agent_security_wiring_test.go` 追加 workbench.go 断言（Task 7 Step 5 预告的 `SetAgentSecurityGate(security)`）。

- [ ] **Step 5: 运行确认通过 + 邻近回归**

Run: `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmission' -count=1 && go test ./internal/handler/session/ -run 'TestGuardAgentSecurity|TestAgentQARunGate' -count=1 && go test ./internal/container/ -run TestAgentSecurityWiringRegistered -count=1 && go build ./...`
Expected: 全部 PASS（含既有 admission 并发/幂等与 #42 run-gate 回归）且构建通过。

- [ ] **Step 6: Commit**

```bash
git add internal/modules/workbench/service/workbench/admission.go internal/modules/workbench/service/workbench/admission_agent_security_test.go internal/handler/session/handler.go internal/handler/session/agent_security_gate.go internal/handler/session/agent_security_gate_test.go internal/handler/session/qa.go internal/handler/session/workbench_start.go internal/handler/agent_adoption.go internal/handler/agent_upgrade.go internal/container/workbench.go internal/container/agent_security_wiring_test.go
git commit -m "feat(security): 新 Task/Run 双入口安全闸门——durable write 前拒绝、fail closed（T34 #64 Task 8）"
```

---

### Task 9: 端到端证据（AC1 / AC2 / AC3）

**Files:**
- Test: `internal/router/routes_agent_security_test.go`（文件 A：治理 wire + workbench 运行入口 + 传播/历史/租户隔离/治理地板）
- Test: `internal/handler/session/agent_security_e2e_test.go`（文件 B：agent-chat 运行入口——必须在 session 包内，`Handler` 的装配字段是包内未导出字段，router 包无法赋值）

**Interfaces:**
- Consumes: Task 1-8 全部产出；router 包既有 helper `openTenantAgentMarketplaceHTTPTestDB`、`adoptionCall`、`publishAdoptionRelease`（internal/router/routes_agent_marketplace_test.go:349、routes_agent_adoption_test.go:74/93）；`newAgentAdoptionTestApp` 的真实 stack 装配范本（routes_agent_adoption_test.go:26）；workbench 侧 `session.NewWorkbenchStartHandler`（公开构造器）+ `workbenchservice.NewAdmissionCoordinatorWithBinding`；`middleware.ErrorHandler`；session 包内 `#42` 的 stub 判例 `runGateSessions`（task_collaboration_run_test.go:36-45）与 `openCraftHTTPDB`。
- Produces: AC 级 e2e 证据（供 #65 信任信号与后续移动治理面引用的既有事实源）。

**文件 A 装配**（`newAgentSecurityTestApp(t)`，照 `newAgentAdoptionTestApp` 逐件真实：真实 `CustomAgentService`/`AgentVersionService`/`AgentMarketplaceService`/`AgentAdoptionService`/`AgentSecurityService`/`AgentSecurityStore`/`AgentRunStore`；真实路由注册 `RegisterAgentMarketplaceRoutes` + `RegisterAgentAdoptionRoutes` + `RegisterAgentSecurityRoutes`；另挂 workbench 运行入口：
- `v1.POST("/workbench/executions", session.NewWorkbenchStartHandler(coordinator).Start)`：`coordinator, err := workbenchservice.NewAdmissionCoordinatorWithBinding(db, runs, workbenchservice.NewDurableTaskBudget(db), nil, workbenchservice.NewDatabaseAdmissionBindingResolver(nil))` 后 `coordinator.SetAgentSecurityGate(security)`；
- RBAC 上下文中间件照 newAgentAdoptionTestApp（X-Test-Tenant/X-Test-Actor/X-Test-Role 注入 context keys + gin keys）。
- 种子：`agent-owned`（`Config.AgentMode: "smart-reasoning"`——variant 本地 agent 经 payload 继承）；会话 `s-wb`（tenant 1、owner `contributor`、`engine_type='trpc'`，作 workbench Start 的 session_id）。

agent-chat 轮次入口**不在文件 A 挂载**：其 handler 装配需要设置 `session.Handler` 的未导出字段（`sessionService`/`customAgentService`），只有 session 包内测试可写——归文件 B。

**文件 B 装配**（`newAgentChatSecurityE2E(t)`）：真实全量迁移 sqlite（`openCraftHTTPDB`）；真实 `AgentSecurityStore`/`AgentRunStore`/`AgentSecurityService`/`AgentAdoptionRepository`/`AgentMarketplaceRepository`/`CustomAgentService`；handler 直接用包内结构体字面量 `&Handler{sessionService: sqlBackedChatSessions{db}, customAgentService: customAgents}`（`sqlBackedChatSessions` 为本文件内嵌 `interfaces.SessionService` 的最小 stub，`GetOwnedSession` 委托 `repository.NewSessionRepository(db)`——#42 runGateSessions 判例）+ `SetAgentSecurityGate(security)`；gin 引擎挂 `middleware.ErrorHandler()` + 同款 context 注入中间件 + `r.POST("/agent-chat/:session_id", h.AgentQA)` + 治理端点 `r.POST("/api/v1/marketplace/tenant/security-revocations/releases", handler.NewAgentSecurityHandler(security).RevokeRelease)` 与 `.../dependencies`（直挂 handler 方法，撤回也走 HTTP）。种子：`tenants`/`users`（`u1`）行、会话 `s-chat`（owner `u1`、`engine_type='trpc'`）、真实仓储播种带锁 Release + adoption + published variant（`UpdateVariantState` 挂 `local_agent_id`，镜像 Task 4 测试 seeder）、本地 agent 行 `db.Create(&types.CustomAgent{ID: "local-agent-e2e", TenantID: 1, Config: types.CustomAgentConfig{AgentMode: "smart-reasoning"}})`。

**文件 B 只断言 blocked 方向**（409，guard 在 SSE/消息行/live-run 槽之前返回，深管线零触达——stub 不实现 `AgentQA`/`streamManager` 也不会被调用）；「放行方向不回归」由 #42 既有 `TestAgentQARunGateStaysOwnerScopedForGrantHolders`（无闸门装配）与文件 A 的 workbench 202 对照共同证明——放行路径进入深管线的全链验证属会话服务既有测试域，不在本 Issue。

带锁 Release 的播种（HTTP 链恒空锁，见差异记录第 7 条）：文件 A/B 各自的 helper `publishSecurityReleaseWithLock(t, db, semanticVersion, lock)`——镜像 service 包 `publishUpgradeServiceRelease` 的仓储路径（`agent_versions` 插行 + `repository.NewAgentMarketplaceRepository(db).CreateSubmission`（带 `DependencyLockJSON: lock`）+ `ReviewAndPublishTx`），返回 (listingID, releaseID)。这是 fixture 播种，被测行为（撤回/阻断/处置）全部走真实 HTTP。

- [ ] **Step 1: 写失败测试（文件 A：四个 AC 断言组，一次写成）**

`internal/router/routes_agent_security_test.go`（要点框架 + 完整断言；`adoptionCall` 复用）：

```go
package router

// T34 (#64) end-to-end evidence (file A) — AC3: 每条断言跑在真实全量迁移
// sqlite 库（openTenantAgentMarketplaceHTTPTestDB）+ 真实 store/service/
// handler + 真实 HTTP 路由（含真实 workbench 准入协调器）上。无 mock 服务、
// 无手写投影（AC3：底层单测、静态检查或 mock 不冒充真实集成证据）。
// agent-chat 入口的 e2e 在 internal/handler/session/agent_security_e2e_test.go
// （文件 B——Handler 装配字段是 session 包内未导出字段）。
//
// AC1 = TestAgentSecurityE2EDependencyRevocationNeverSubstitutesByName。
// AC2 = TestAgentSecurityE2ERevocationKeepsHistoryReasonScopeReplacement。
// 阻断/在途 = TestAgentSecurityE2EReleaseRevocationBlocksNewTaskRunAndDisposesInFlight。
// 治理面/租户隔离 = TestAgentSecurityE2EGovernanceFloorAndTenantIsolation。

import (...)

func newAgentSecurityTestApp(t *testing.T) (*gin.Engine, *gorm.DB) { /* 按 Interfaces 装配块实现；返回 engine/db（会话 id 常量 's-wb'） */ }

// publishVariantOverHTTP 走真实治理 HTTP：adopt → create variant →
// capability-mapping（manifest 要求 model/knowledge 时给齐）→ test → publish，
// 返回 published variant 的 local_agent_id。
func publishVariantOverHTTP(t *testing.T, r *gin.Engine, listingID, releaseID, name string) string

func TestAgentSecurityE2EReleaseRevocationBlocksNewTaskRunAndDisposesInFlight(t *testing.T) {
	r, db := newAgentSecurityTestApp(t)
	listingID, releaseID := publishAdoptionRelease(t, r)
	localAgentID := publishVariantOverHTTP(t, r, listingID, releaseID, "Sales")

	// 在途 Run：真实 agent_runs 行 + 会话活动槽（fixture，镜像 #34/#35 形态）。
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = 'e2e-live-1' WHERE tenant_id = 1 AND id = 's-wb'`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, engine_type, status, snapshot, deadline) VALUES
		(1, 'e2e-live-1', 's-wb', 'contributor', 'req-e2e-1', 'me1', 'he1', 'trpc', 'running', ?, datetime('now','+1 hour'))`,
		`{"session_id":"s-wb","agent_id":"`+localAgentID+`","request_id":"req-e2e-1","text":"hi"}`).Error)

	// 撤回前：workbench 新 Run 入口照常（202）。
	preWorkbench := adoptionCall(r, 1, http.MethodPost, "/api/v1/workbench/executions", "contributor", "contributor",
		map[string]any{"session_id": "s-wb", "request_id": "req-e2e-pre", "text": "hi", "agent_id": localAgentID})
	require.Equal(t, http.StatusAccepted, preWorkbench.Code, preWorkbench.Body.String())

	// 撤回（默认在途处置 cancel）。
	revoked := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", "admin", "sec-admin",
		map[string]any{"release_id": releaseID, "reason": "CVE-2026-4321 data exfiltration"})
	require.Equal(t, http.StatusCreated, revoked.Code, revoked.Body.String())

	// 撤回后：workbench 新 Run 被拒（409，零 durable write）。
	postWorkbench := adoptionCall(r, 1, http.MethodPost, "/api/v1/workbench/executions", "contributor", "contributor",
		map[string]any{"session_id": "s-wb", "request_id": "req-e2e-post", "text": "hi", "agent_id": localAgentID})
	require.Equal(t, http.StatusConflict, postWorkbench.Code)
	require.Contains(t, postWorkbench.Body.String(), "agent security policy refused the admission")

	// （agent-chat 轮次入口的同向 409 断言在文件 B。）

	// 在途执行已按风险处置：canceled + 时间线事实 + 槽释放。
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM agent_runs WHERE tenant_id = 1 AND run_id = 'e2e-live-1'`).Scan(&status).Error)
	require.Equal(t, "canceled", status)
	var events int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_run_events WHERE tenant_id = 1 AND run_id = 'e2e-live-1' AND event_type = 'cancellation_requested'`).Scan(&events).Error)
	require.EqualValues(t, 1, events)
	var slot *string
	require.NoError(t, db.Raw(`SELECT active_agent_run_id FROM sessions WHERE tenant_id = 1 AND id = 's-wb'`).Scan(&slot).Error)
	require.Nil(t, slot)

	// 治理面同步收紧：新 Variant 草稿（默认 accepted release）被拒。
	adoptionList := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, adoptionList.Code)
	var adoptions struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adoptions.Body.Bytes(), &adoptions))
	require.NotEmpty(t, adoptions.Data)
	blockedVariant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptions.Data[0].ID+"/variants", "admin", "admin", map[string]any{"name": "Post-revoke"})
	require.Equal(t, http.StatusConflict, blockedVariant.Code, "被撤回 Release 禁止新变体（CONTEXT.md「依赖安全阻断/安全撤回」）")
}

func TestAgentSecurityE2EDependencyRevocationNeverSubstitutesByName(t *testing.T) {
	r, db, _ := newAgentSecurityTestApp(t)
	// 带锁 Release 经真实仓储播种（HTTP 链恒空锁，差异记录第 7 条）。
	listingV123, rLockV123 := publishSecurityReleaseWithLock(t, db, "1.0.0", `{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.3","digest":"D1","license_id":"MIT"}]}`)
	listingV124, rLockV124 := publishSecurityReleaseWithLock(t, db, "1.1.0", `{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.4","digest":"D2","license_id":"MIT"}]}`)
	agentV123 := publishVariantOverHTTP(t, r, listingV123, rLockV123, "On v123")
	agentV124 := publishVariantOverHTTP(t, r, listingV124, rLockV124, "On v124")

	revoked := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/dependencies", "admin", "sec-admin",
		map[string]any{"dependency": map[string]string{"type": "skill", "id": "web-search", "version": "1.2.3", "digest": "D1"},
			"reason": "supply-chain compromise in pinned skill content", "replacement_version": "1.2.4"})
	require.Equal(t, http.StatusCreated, revoked.Code, revoked.Body.String())

	// AC1：锁定被撤回依赖的 Release 的新 Run 被拒（workbench 入口）。
	blockedRun := adoptionCall(r, 1, http.MethodPost, "/api/v1/workbench/executions", "contributor", "contributor",
		map[string]any{"session_id": "s-wb", "request_id": "req-e2e-dep-blocked", "text": "hi", "agent_id": agentV123})
	require.Equal(t, http.StatusConflict, blockedRun.Code)
	require.Contains(t, blockedRun.Body.String(), "agent security policy refused the admission")

	// AC1 反面：同名不同版本（1.2.4/D2）不受牵连、也不被顶替——照常可启动（202）。
	cleanRun := adoptionCall(r, 1, http.MethodPost, "/api/v1/workbench/executions", "contributor", "contributor",
		map[string]any{"session_id": "s-wb", "request_id": "req-e2e-dep-clean", "text": "hi", "agent_id": agentV124})
	require.Equal(t, http.StatusAccepted, cleanRun.Code, "同名不同版本不被阻断（不按名称替换/不误伤），也未被静默换绑")

	// 全链无任何「自动换依赖」痕迹：被阻断 variant 的本地 agent 行未被改写。
	var cfgAgent string
	require.NoError(t, db.Raw(`SELECT id FROM custom_agents WHERE tenant_id = 1 AND id = ?`, agentV123).Scan(&cfgAgent).Error)
	require.Equal(t, agentV123, cfgAgent)
}

func TestAgentSecurityE2ERevocationKeepsHistoryReasonScopeReplacement(t *testing.T) {
	r, db := newAgentSecurityTestApp(t)
	listingID, r1 := publishAdoptionRelease(t, r)
	_, r2 := publishAdoptionRelease(t, r) // 第二个 release（同 listing 前移）
	agentR1 := publishVariantOverHTTP(t, r, listingID, r1, "On r1")

	revoked := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", "admin", "sec-admin",
		map[string]any{"release_id": r1, "reason": "prompt-injection vector via pinned persona", "replacement_release_id": r2})
	require.Equal(t, http.StatusCreated, revoked.Code)
	var revokeBody struct {
		Data struct {
			ID, Reason, ReplacementReleaseID string
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(revoked.Body.Bytes(), &revokeBody))
	require.Equal(t, "prompt-injection vector via pinned persona", revokeBody.Data.Reason)
	require.Equal(t, r2, revokeBody.Data.ReplacementReleaseID)

	// AC2：单读带影响范围（blocked release + 受影响 adoption/variant）。
	detail := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/security-revocations/"+revokeBody.Data.ID, "admin", "admin", nil)
	require.Equal(t, http.StatusOK, detail.Code)
	var detailBody struct {
		Data struct {
			Scope struct {
				BlockedReleases   []map[string]any `json:"blocked_releases"`
				AffectedVariants  []map[string]any `json:"affected_variants"`
			} `json:"scope"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(detail.Body.Bytes(), &detailBody))
	require.Len(t, detailBody.Data.Scope.BlockedReleases, 1)
	require.Equal(t, r1, detailBody.Data.Scope.BlockedReleases[0]["release_id"])
	require.Len(t, detailBody.Data.Scope.AffectedVariants, 1)
	require.Equal(t, agentR1, detailBody.Data.Scope.AffectedVariants[0]["local_agent_id"])

	// AC2：历史保留——release/variant/本地 agent/已完成 Run 全部仍在，审计行在场。
	second := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", "admin", "sec-admin-2",
		map[string]any{"release_id": r1, "reason": "scope expanded"})
	require.Equal(t, http.StatusCreated, second.Code)
	list := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/security-revocations", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, list.Code)
	var listBody struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listBody))
	require.Len(t, listBody.Data, 2, "append-only 历史：两次撤回两条记录")
	var auditRows int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM audit_logs WHERE tenant_id = 1 AND action = 'agent_security.release_revoked'`).Scan(&auditRows).Error)
	require.EqualValues(t, 2, auditRows)
	var releaseRows int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_releases WHERE tenant_id = 1 AND id = ?`, r1).Scan(&releaseRows).Error)
	require.EqualValues(t, 1, releaseRows, "撤回不删除 Release 行")
}

func TestAgentSecurityE2EGovernanceFloorAndTenantIsolation(t *testing.T) {
	r, _, _ := newAgentSecurityTestApp(t)
	// Admin+ full-access 地板（#61 治理路由同款断言面）。	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")
	RegisterAgentSecurityRoutes(v1, handler.NewAgentSecurityHandler(nil), g)
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases"},
		{http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/dependencies"},
		{http.MethodGet, "/api/v1/marketplace/tenant/security-revocations"},
		{http.MethodGet, "/api/v1/marketplace/tenant/security-revocations/:id"},
	} {
		policy := mustLookupAPIKeyPolicy(t, g, route.method, route.path)
		require.Truef(t, policy.RequireFullAccess, "%s %s 必须要求 full-access", route.method, route.path)
	}
	bare := gin.New()
	RegisterAgentSecurityRoutes(bare.Group("/api/v1"), nil, &rbacGuards{})
	require.Empty(t, bare.Routes(), "nil handler fail closed")

	// 跨租户：tenant 2 的列表为空、跨租户单读 404。
	list2 := adoptionCall(r, 2, http.MethodGet, "/api/v1/marketplace/tenant/security-revocations", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, list2.Code)
	var empty struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(list2.Body.Bytes(), &empty))
	require.Empty(t, empty.Data)
	foreign := adoptionCall(r, 2, http.MethodGet, "/api/v1/marketplace/tenant/security-revocations/whatever", "admin", "admin", nil)
	require.Equal(t, http.StatusNotFound, foreign.Code)
}
```

（`publishVariantOverHTTP` 的 mapping body 镜像 routes_agent_adoption_test.go:341 的既有形状。）

- [ ] **Step 2: 写失败测试（文件 B：agent-chat 轮次入口）**

`internal/handler/session/agent_security_e2e_test.go`：

```go
package session

// T34 (#64) end-to-end evidence (file B) — agent-chat 轮次入口：真实全量迁移
// sqlite（openCraftHTTPDB）+ 真实 AgentSecurityService/Store + 真实治理 HTTP
// 撤回端点 + 真实 AgentQA handler（包内装配 SetAgentSecurityGate）。只断言
// blocked 方向——guard 在 SSE/消息行/live-run 槽之前返回，深管线零触达；
// 放行方向由 #42 TestAgentQARunGateStaysOwnerScopedForGrantHolders（无闸门
// 装配不回归）与文件 A 的 workbench 202 对照共同证明。

import (...)

func newAgentChatSecurityE2E(t *testing.T) (*gin.Engine, *gorm.DB) { /* 按 Interfaces「文件 B 装配」块实现 */ }

func TestAgentSecurityE2EAgentChatTurnRefusedForRevokedRelease(t *testing.T) {
	r, db := newAgentChatSecurityE2E(t)
	_, rLocked := publishSecurityReleaseWithLock(t, db, "1.0.0",
		`{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.3","digest":"D1","license_id":"MIT"}]}`)

	chat := func(agentID string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/agent-chat/s-chat", strings.NewReader(`{"query":"hi","agent_id":"`+agentID+`","agent_enabled":true}`))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 本测试不发起「撤回前」的 chat 请求：无撤回行时 guard 放行，请求会进入
	// executeQA 深管线（本装配的 stub sessionService 未带深管线依赖）。
	// 放行方向由文件 A 的 workbench 202 对照 + #42 既有 run-gate 测试证明。
	// Release 撤回 → 新轮次 409（SSE 之前）。
	revoked := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", strings.NewReader(`{"release_id":"`+rLocked+`","reason":"CVE-2026-7777"}`))
	revoked.Header.Set("Content-Type", "application/json")
	rctx := context.WithValue(revoked.Context(), types.TenantIDContextKey, uint64(1))
	rctx = context.WithValue(rctx, types.UserIDContextKey, "sec-admin")
	revoked = revoked.WithContext(rctx)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, revoked)
	require.Equal(t, http.StatusCreated, rw.Code, rw.Body.String())

	blocked := chat("local-agent-e2e")
	require.Equal(t, http.StatusConflict, blocked.Code)
	require.Contains(t, blocked.Body.String(), "agent security policy refused this agent")
}
```

依赖撤回（四元组命中锁）方向的第二个测试：

```go
func TestAgentSecurityE2EAgentChatTurnRefusedForRevokedDependency(t *testing.T) {
	r, db := newAgentChatSecurityE2E(t)
	_, rLocked := publishSecurityReleaseWithLock(t, db, "1.0.0",
		`{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.3","digest":"D1","license_id":"MIT"}]}`)

	dep := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/dependencies",
		strings.NewReader(`{"dependency":{"type":"skill","id":"web-search","version":"1.2.3","digest":"D1"},"reason":"supply-chain"}`))
	dep.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(dep.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "sec-admin")
	dep = dep.WithContext(ctx)
	dw := httptest.NewRecorder()
	r.ServeHTTP(dw, dep)
	require.Equal(t, http.StatusCreated, dw.Code, dw.Body.String())

	req := httptest.NewRequest(http.MethodPost, "/agent-chat/s-chat", strings.NewReader(`{"query":"hi","agent_id":"local-agent-e2e","agent_enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	qctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
	qctx = context.WithValue(qctx, types.UserIDContextKey, "u1")
	req = req.WithContext(qctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "锁定被撤回依赖的 variant 新轮次被拒")
	require.Contains(t, w.Body.String(), "security-revoked")
}
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./internal/router/ -run 'TestAgentSecurityE2E' -count=1 && go test ./internal/handler/session/ -run 'TestAgentSecurityE2EAgentChat' -count=1`
Expected: FAIL——两文件的装配 helper 尚未实现（编译失败）或装配缺件。

- [ ] **Step 4: 实现装配与 helper**

文件 A：`newAgentSecurityTestApp`、`publishSecurityReleaseWithLock`、`publishVariantOverHTTP`；文件 B：`newAgentChatSecurityE2E`、`publishSecurityReleaseWithLock`（session 包内独立副本，镜像 service 包 `publishUpgradeServiceRelease` 仓储路径）、`sqlBackedChatSessions`。

- [ ] **Step 5: 运行确认通过**

Run: `go test ./internal/router/ -run 'TestAgentSecurityE2E' -count=1 && go test ./internal/handler/session/ -run 'TestAgentSecurityE2EAgentChat' -count=1`
Expected: 文件 A 4 个测试 + 文件 B 2 个测试 PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/router/routes_agent_security_test.go internal/handler/session/agent_security_e2e_test.go
git commit -m "test(security): AC1-AC3 端到端证据——真实 stack 双入口阻断与撤回台账（T34 #64 Task 9）"
```

---

## 计划级验证命令（worktree 根执行，覆盖本计划全部测试）

```bash
go build ./... && go test ./internal/database/ -run TestMigrationVersionsUniquePerTrack -count=1 && go test ./internal/application/repository/ -run 'TestAgentSecurity|TestCancelRunsByAgents' -count=1 && go test ./internal/application/service/ -run 'TestAgentSecurity|TestRevoke|TestAdoptionServiceReleaseSecurityGate|TestUpgradeServiceReleaseSecurityGate' -count=1 && go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmission' -count=1 && go test ./internal/handler/session/ -run 'TestGuardAgentSecurity|TestAgentQARunGate|TestAgentSecurityE2EAgentChat' -count=1 && go test ./internal/handler/ -run 'TestRevoke' -count=1 && go test ./internal/container/ -run TestAgentSecurityWiringRegistered -count=1 && go test ./internal/router/ -run 'TestAgentSecurityE2E' -count=1
```

（定向到受影响包/测试名，避免全量 flaky 套件；`go build ./...` 兜底编译面。）

## 边界与声明

- **零 TS / 零移动端改动**：撤回状态对移动端的呈现（信任信号、可用 Agent 面的 blocked 徽标）属 #65 与后续移动治理面；本计划 `ListAvailableAgents` 读模型**不改**（B6 并行批次 #63 将重构同域，冲突最小化）——blocked 判定经 `VerdictForAgent`/`ReleaseAdmission` 与撤回端点的 scope 暴露，#65 直接消费。
- **目录可见性（§10「新发现：否」）**：公共目录 `ListPublicCatalog` 的可见性过滤（撤回 Release 不再出现在目录发现面）属 #65 信任信号面——本计划的 tenant 侧已通过 Adopt 准入拒绝实现「新引入=否」；目录行的安全徽标/过滤不在本 Issue 验收标准内，明确范围外。
- **在途处置的覆盖面**：`CancelRunsByAgents` 处置**durable Task/Run 轨道**（workbench 准入创建的 `agent_runs` 行，含 queue_next 重启派发的前身）；经典 agent-chat SSE 轮次没有 durable run 行（`streamManager.SetLiveRun` 为进程内协调，qa.go:722），其「在途」只能靠**新轮次闸门**阻断——这是如实边界，不是缺口伪装。语音/预算等其它在途面不在本 Issue 范围。
- **撤回的不可逆性**：本计划不提供「撤销撤回」端点（spec 未定义；修复路径是「新 Agent Release、Review 和 Upgrade Proposal」，§11）。重复撤回=追加历史行。
- **发布链空锁现状**：生产 HTTP 提交链当前只产空锁（差异记录第 7 条）；依赖撤回的传播面因此今天实际覆盖「仓储/引入台账播种的锁」，随 #58 resolver 的后续演进自动生效——判定逻辑不假设锁来源。
- **blocked-env**：无。全部验收（AC1/AC2/AC3）在本地真实 sqlite 迁移流 + 真实 HTTP 上可验证；无外部凭据、无真机依赖。
- **并行批次合并注意**：对共享文件的修改清单见「任务结构与文件地图」；`agent_adoption.go`/`agent_upgrade.go`（#63 同域）、`admission.go`、`qa.go`、`container.go`/`workbench.go`/`router.go` 的改动均为数行级且位置在计划中写死。

## 自我审查记录（writing-plans 技能四项检查 + 比例）

1. **Spec 覆盖**：AC1（不按名称替换）→ Task 4 四元组判定 + Task 9 `TestAgentSecurityE2EDependencyRevocationNeverSubstitutesByName`；AC2（历史/原因/范围/替代版本）→ Task 1 append-only 表 + Task 5 `TestRevokeReleaseRecordsHistoryReasonScopeReplacementAndDisposesInFlight` + Task 9 `TestAgentSecurityE2ERevocationKeepsHistoryReasonScopeReplacement`；AC3（最高稳定 Interface）→ Task 9 两个文件六测试（router 侧真实迁移+真实 stack+真实治理 HTTP+真实 workbench 准入；session 侧真实 AgentQA handler wire）；「新 Task/Run 阻断」→ Task 8 双入口 + queue_next 经 `admission.Start` 自动覆盖（command_queue_next.go:96-100 委证实）；「在途按风险处置」→ Task 3 + Task 5 + e2e；「禁止新引入/新变体」→ Task 6。「运行中按风险暂停或终止」的「暂停」选项：本计划实现「终止（cancel）」与「保留（allow）」两个显式处置（spec 允许二选一，处置由治理主体声明）；「暂停」粒度（park 等待人工）未实现——如需要属后续增强，非 AC 要求。
2. **步骤扫描**：每个 RED 步骤含完整测试代码；每个 GREEN 步骤是签名+不变式（实现纪律写死处均已标注「写死」）；无 TBD/占位（`import (...)` 省略号仅出现于 Task 9 两个 e2e 骨架文件，其符号集由测试体与 Interfaces 装配块完全确定）。
3. **类型一致性**：`AgentSecurityStore` 方法名在 Task 2 定义、Task 4/5 消费处逐一核对；`interfaces.AgentSecurityVerdict`/`AgentSecurityRevocationView` 字段在 Task 4 定义、Task 7 handler 体与 Task 9 断言逐字一致；`SetAgentSecurityGate`（AdmissionCoordinator 与 session.Handler 各一）与 `SetReleaseSecurityGate`（两个 service）命名不冲突；哨兵 `ErrAgentSecurityBlocked`（workbenchservice）与 `ErrAgentSecurityReleaseBlocked`（service）各归其位、错误映射 case 与之一致。
4. **Review Focus**：五条各有所属测试（见节内映射）；第六条畸形输入并入 Task 5 测试。
5. **比例**：计划长度与 plan-t61 同量级（仓库既定风格）；测试代码占比高是「测试代码完整写入计划」写作要求的直接结果，实现描述均为签名+纪律而非代码誊写。
