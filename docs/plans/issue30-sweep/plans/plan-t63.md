# Issue #63 — T33: Variant 退役、Adoption 终止与 Listing 生命周期 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Tenant Marketplace 交付 Retire Variant、End Adoption、Unlist Listing、Deprecate Release 四类生命周期操作——停止新使用但保留全部历史（版本、Task、Artifact、许可、审计）。

**Architecture:** 全部新逻辑收敛到 lifecycle 独有文件（service/repository/handler/router-route 各一个新文件 + 一对新迁移）；对既有共享文件的修改仅为：4 个实体加可空生命周期列、两个 repository 接口各加少量方法声明、Adopt/CreateVariant/reconcile/Accept 四处 deprecate 拒绝、workbench admission 一个可注入 agent-use 闸、router/container 各一处接线。读面收窄（catalog 过滤 unlisted、available-agents 只读 published、Adopt 拒非 listed）已由 #59/#61 既有代码天然承载，本计划只写转换端点与闸，并用端到端测试钉死行为。

**Tech Stack:** Go 1.26（go.mod）、gin、gorm、golang-migrate（sqlite/versioned 两轨独立编号）、httptest + 真实 sqlite 迁移流端到端测试。

**Spec:** `docs/specs/2026-09-20-agent-marketplace-domain-model.md` §9（退出分两层）、§10（目录与 Release 状态表）、§13（retire_variant / end_adoption / manage_listing 权限）；`CONTEXT.md`「Listing 下架（Unlisted）」「Release 弃用（Deprecated）」「Agent 变体退役」「Agent 引入终止（Agent Adoption End）」四条术语；Issue `docs/plans/issue30-sweep/issues/issue-63.md` 验收标准原文。

## Global Constraints

逐字引用 Issue #63 验收标准（`docs/plans/issue30-sweep/issues/issue-63.md`）：

- 退出不删除版本、Task、Artifact、许可或审计。
- Unlisted 与 Deprecated 的行为差异可验证。
- 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

Spec 约束（§9/§10 原文摘录）：

- 「Retire Variant：禁止新 Task，保留本地版本、历史 Task、Artifact 和审计」
- 「End Adoption：全部 Variant 退役后，禁止新 Variant 和升级建议，并归档 Adoption」
- 「退出不删除 Listing、Release、许可证或 lineage。安全撤回是更强的独立流程。」
- 「Unlisted | 否 | 否 | 已有本地版本可继续 | 保留」「Deprecated | 可见但不推荐 | 默认阻止或警告 | 已有本地版本可继续 | 保留」「任何状态都不静默升级或删除历史」
- 权限（§13）：「retire_variant 与 end_adoption：退出使用」「manage_listing：发布、下架或标记弃用」

仓库级约束：

- 数据库查询一律参数绑定，禁止拼接 SQL（Mimosa 约束；本计划全部 gorm Where 绑定）。
- 迁移号两轨独立计数、全仓唯一（`TestMigrationVersionsUniquePerTrack` 常驻守卫）；实测当前最大 sqlite=000123、versioned=000202，本计划取 **sqlite 000124 / versioned 000203**（实测命令：`ls migrations/sqlite | sort -t_ -k1 -n | tail -3`、`ls migrations/versioned | sort -t_ -k1 -n | tail -3`）。
- 与同批并行计划独立 worktree 后合并：共享文件修改保持最小且位置在计划中逐一写明。

## Review Focus

1. **已退役变体的迟到并发写**（map/test/publish 撞上 retire）：必须 409 fail closed，绝不静默降级或复活——`UpdateVariantState` 的 CAS 前态集合不含 `retired`，测试钉死（Task 3）。
2. **End Adoption 与并发 CreateVariant 的竞态**：「全部 Variant 已退役」检查与 adoption 状态 CAS 必须在同一事务内闭合（检查在事务外就会漏掉竞态新建的 draft，使其落进 ended adoption）——`EndAdoption` 事务原语的结构约束 + precondition/CAS 拒绝路径测试（Task 2）。
3. **Deprecate 的 successor 指向畸形**（自身 / 异 listing / 不存在 / 已弃用）：一律显式 400/409，不得落半行弃用记录（Task 3）。
4. **运行闸的误伤面**：不选 agent 的 start（AgentID 为空）不得被闸拦截；闸只在 durable 写（CreatePending）之前咨询（Task 5）。
5. **跨租户谓词**：他租户对同 id 的 variant/adoption/listing/release 调四个 lifecycle 端点必须统一 404，不泄漏存在性（Task 4/6）。

---

### Task 1: 迁移与实体生命周期列

**Files:**
- Create: `migrations/sqlite/000124_agent_marketplace_lifecycle.up.sql`
- Create: `migrations/sqlite/000124_agent_marketplace_lifecycle.down.sql`
- Create: `migrations/versioned/000203_agent_marketplace_lifecycle.up.sql`
- Create: `migrations/versioned/000203_agent_marketplace_lifecycle.down.sql`
- Create: `internal/application/repository/agent_marketplace_lifecycle_test.go`（本任务只写迁移对齐测试；Task 2 续用该文件）
- Modify: `internal/types/agent_adoption_persistence.go:16-51`（两个实体加列）
- Modify: `internal/types/agent_marketplace_persistence.go:5-17,58-83`（两个实体加列）

**Interfaces:**
- Consumes: 既有实体 `types.AgentAdoptionEntity` / `types.AgentAdoptionVariantEntity` / `types.AgentMarketplaceListingEntity` / `types.AgentReleaseEntity`。
- Produces（后续任务依赖的字段，全部为本任务新增）：
  - `types.AgentAdoptionVariantEntity.RetiredBy string`（gorm `type:varchar(255);not null;default:''`）与 `RetiredAt *time.Time`
  - `types.AgentAdoptionEntity.EndedBy string`（同上 default ''）与 `EndedAt *time.Time`
  - `types.AgentMarketplaceListingEntity.UnlistedBy string` 与 `UnlistedAt *time.Time`
  - `types.AgentReleaseEntity.DeprecatedBy string`、`DeprecatedAt *time.Time`、`SuccessorReleaseID string`（gorm `type:varchar(36);not null;default:''`）

- [x] **Step 1: 写失败的迁移对齐测试**

在 `internal/application/repository/agent_marketplace_lifecycle_test.go` 写（RED：迁移尚不存在，`m.Up()` 在 000124 文件缺席时仍会成功但新列不存在 → 断言失败；同时该测试走**真实全量 sqlite 迁移流**，天然验证两轨号位不冲突）：

```go
package repository

// T33 #63 lifecycle primitives + migration alignment tests.

import (
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
)

// openLifecycleMigrationDB applies the REAL sqlite migration stream so the
// lifecycle columns are the production schema, not an AutoMigrate sketch.
func openLifecycleMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "lifecycle.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, m.Up())
	_, _ = m.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestAgentMarketplaceLifecycleMigrationColumns(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	expect := map[string]map[string]bool{
		"agent_marketplace_listings": {"unlisted_at": true, "unlisted_by": true},
		"agent_releases":             {"deprecated_at": true, "deprecated_by": true, "successor_release_id": true},
		"agent_adoptions":            {"ended_at": true, "ended_by": true},
		"agent_adoption_variants":    {"retired_at": true, "retired_by": true},
	}
	for table, columns := range expect {
		rows, err := db.Raw("SELECT name FROM pragma_table_info(?)", table).Rows()
		require.NoError(t, err)
		present := map[string]bool{}
		for rows.Next() {
			var name string
			require.NoError(t, rows.Scan(&name))
			present[name] = true
		}
		require.NoError(t, rows.Close())
		for column := range columns {
			require.Truef(t, present[column], "%s.%s 必须由迁移创建", table, column)
		}
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/repository/ -run TestAgentMarketplaceLifecycleMigrationColumns -count=1`
Expected: FAIL（`agent_marketplace_listings.unlisted_at 必须由迁移创建`）

- [x] **Step 3: 写两对迁移文件与实体列**

sqlite `000124_agent_marketplace_lifecycle.up.sql`（镜像 000122 的 twin 风格）：

```sql
-- T33 Variant 退役 / Adoption 终止 / Listing 下架 / Release 弃用（#63, spec §9-§10）：
-- 生命周期只加列不改内容——Release 的 bundle/manifest/lock 恒不可变，退出不删行。
-- SQLite twin of versioned migration 000203.
ALTER TABLE agent_marketplace_listings ADD COLUMN unlisted_at DATETIME;
ALTER TABLE agent_marketplace_listings ADD COLUMN unlisted_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN deprecated_at DATETIME;
ALTER TABLE agent_releases ADD COLUMN deprecated_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN successor_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_adoptions ADD COLUMN ended_at DATETIME;
ALTER TABLE agent_adoptions ADD COLUMN ended_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_adoption_variants ADD COLUMN retired_at DATETIME;
ALTER TABLE agent_adoption_variants ADD COLUMN retired_by VARCHAR(255) NOT NULL DEFAULT '';
```

`000124_agent_marketplace_lifecycle.down.sql`：

```sql
ALTER TABLE agent_adoption_variants DROP COLUMN retired_by;
ALTER TABLE agent_adoption_variants DROP COLUMN retired_at;
ALTER TABLE agent_adoptions DROP COLUMN ended_by;
ALTER TABLE agent_adoptions DROP COLUMN ended_at;
ALTER TABLE agent_releases DROP COLUMN successor_release_id;
ALTER TABLE agent_releases DROP COLUMN deprecated_by;
ALTER TABLE agent_releases DROP COLUMN deprecated_at;
ALTER TABLE agent_marketplace_listings DROP COLUMN unlisted_by;
ALTER TABLE agent_marketplace_listings DROP COLUMN unlisted_at;
```

versioned `000203_agent_marketplace_lifecycle.up.sql`（TIMESTAMPTZ twin，头部注释 `-- Versioned twin of sqlite migration 000124.`）：

```sql
ALTER TABLE agent_marketplace_listings ADD COLUMN unlisted_at TIMESTAMPTZ;
ALTER TABLE agent_marketplace_listings ADD COLUMN unlisted_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN deprecated_at TIMESTAMPTZ;
ALTER TABLE agent_releases ADD COLUMN deprecated_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN successor_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_adoptions ADD COLUMN ended_at TIMESTAMPTZ;
ALTER TABLE agent_adoptions ADD COLUMN ended_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_adoption_variants ADD COLUMN retired_at TIMESTAMPTZ;
ALTER TABLE agent_adoption_variants ADD COLUMN retired_by VARCHAR(255) NOT NULL DEFAULT '';
```

versioned down 与 sqlite down 同语句。实体加列（逐字）：`AgentAdoptionVariantEntity` 在 `PublishedAt *time.Time`（`internal/types/agent_adoption_persistence.go:46`）后加 `RetiredBy string \`gorm:"type:varchar(255);not null;default:''"\`` 与 `RetiredAt *time.Time`；`AgentAdoptionEntity` 在 `CreatedBy` 后加 `EndedBy`/`EndedAt` 同形；`AgentMarketplaceListingEntity` 在 `CurrentReleaseID` 后加 `UnlistedBy`/`UnlistedAt`；`AgentReleaseEntity` 在 `PublishedBy`（`internal/types/agent_marketplace_persistence.go:79`）后加 `DeprecatedBy string`、`DeprecatedAt *time.Time`、`SuccessorReleaseID string`（三者 gorm tag 同上）。

- [x] **Step 4: 运行测试确认通过 + 迁移唯一性守卫**

Run: `go test ./internal/application/repository/ -run TestAgentMarketplaceLifecycleMigrationColumns -count=1 && go test ./internal/database/ -run TestMigrationVersionsUniquePerTrack -count=1 && go build ./...`
Expected: 全部 PASS / ok

- [x] **Step 5: Commit**

```bash
git add migrations/sqlite/000124_agent_marketplace_lifecycle.*.sql migrations/versioned/000203_agent_marketplace_lifecycle.*.sql internal/types/agent_adoption_persistence.go internal/types/agent_marketplace_persistence.go internal/application/repository/agent_marketplace_lifecycle_test.go
git commit -m "feat(marketplace): lifecycle 迁移与实体列——retire/end/unlist/deprecate 只加列不删行"
```

---

### Task 2: repository 生命周期原语（EndAdoption 事务 / Listing CAS / Release 弃用 CAS / 退役归属查询）

**Files:**
- Create: `internal/application/repository/agent_marketplace_lifecycle.go`
- Modify: `internal/application/repository/agent_adoption.go:39-52`（接口块尾加两行声明）
- Modify: `internal/application/repository/agent_marketplace.go:29-46`（接口块尾加两行声明）
- Test: `internal/application/repository/agent_marketplace_lifecycle_test.go`（续用 Task 1 文件）

**Interfaces:**
- Consumes: Task 1 的实体列；既有 `agentAdoptionRepository` / `agentMarketplaceRepository` 结构（方法实现挂既有 struct，与 #62 `agent_marketplace_lineage.go` 同款先例）。
- Produces:
  - `EndAdoption(ctx context.Context, tenantID uint64, adoptionID string, expectedFrom, nextState string, updates map[string]any) (*types.AgentAdoptionEntity, error)`——单事务：读 adoption（miss→`ErrAgentAdoptionNotFound`）→ state 不符→`ErrAgentAdoptionTransition`（本计划新哨兵：agent adoption state transition failed）→`COUNT(*) WHERE tenant_id=? AND adoption_id=? AND state <> 'retired'`（>0→`ErrAgentAdoptionEndPrecondition`，消息点名残留数与首个残留 variant id）→ 参数化 UPDATE state+updates+`updated_at`。
  - `TransitionListingState(ctx context.Context, tenantID uint64, listingID, expectedFrom, nextState string, updates map[string]any) (*types.AgentMarketplaceListingEntity, error)`——CAS `WHERE tenant_id=? AND id=? AND state=?`；RowsAffected!=1 时重读：miss→`ErrAgentMarketplaceNotFound`，否则→`ErrAgentMarketplaceListingTransition`。
  - `DeprecateRelease(ctx context.Context, tenantID uint64, releaseID, deprecatedBy, successorReleaseID string) (*types.AgentReleaseEntity, error)`——CAS `WHERE tenant_id=? AND id=? AND deprecated_at IS NULL`，SET `deprecated_at=now, deprecated_by=?, successor_release_id=?`；RowsAffected!=1 时重读区分 404/`ErrAgentReleaseDeprecateConflict`；成功后回读整行。
  - `RetiredVariantAgentExists(ctx context.Context, tenantID uint64, localAgentID string) (bool, error)`——`SELECT COUNT(*) > 0 FROM agent_adoption_variants WHERE tenant_id=? AND local_agent_id=? AND state='retired'`。
  - 新哨兵：`ErrAgentAdoptionEndPrecondition`、`ErrAgentAdoptionTransition`、`ErrAgentMarketplaceListingTransition`、`ErrAgentReleaseDeprecateConflict`（均 `errors.New`，与既有哨兵同放新文件顶部）。

- [ ] **Step 1: 写失败的 repository 测试**

在 `agent_marketplace_lifecycle_test.go` 追加（RED：方法未声明，编译失败即 RED）：

```go
func TestEndAdoptionRequiresAllVariantsRetiredAndIsTransactional(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "ad1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active", CreatedBy: "admin"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{TenantID: 1, ID: "v1", AdoptionID: "ad1", ReleaseID: "r1", Name: "sales", State: "published"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{TenantID: 1, ID: "v2", AdoptionID: "ad1", ReleaseID: "r1", Name: "legal", State: "retired", RetiredBy: "admin"}).Error)

	// 残留 published 变体：拒绝且不改行。
	_, err := repo.EndAdoption(ctx, 1, "ad1", "active", "ended", map[string]any{"ended_by": "admin"})
	require.ErrorIs(t, err, ErrAgentAdoptionEndPrecondition)
	var unchanged types.AgentAdoptionEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, "ad1").First(&unchanged).Error)
	require.Equal(t, "active", unchanged.State)

	// 全部退役后：CAS 成功，ended_by/ended_at 成对落列。
	require.NoError(t, db.Exec("UPDATE agent_adoption_variants SET state='retired' WHERE id='v1'").Error)
	ended, err := repo.EndAdoption(ctx, 1, "ad1", "active", "ended", map[string]any{"ended_by": "admin"})
	require.NoError(t, err)
	require.Equal(t, "ended", ended.State)
	require.Equal(t, "admin", ended.EndedBy)
	require.NotNil(t, ended.EndedAt)

	// 状态不符（重复 end）：哨兵冲突，不是静默成功。
	_, err = repo.EndAdoption(ctx, 1, "ad1", "active", "ended", map[string]any{"ended_by": "admin"})
	require.ErrorIs(t, err, ErrAgentAdoptionTransition)

	// 跨租户谓词：tenant 2 对同 id 一律 not found。
	_, err = repo.EndAdoption(ctx, 2, "ad1", "active", "ended", nil)
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound)
}

func TestTransitionListingStateIsCAS(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	row, err := repo.TransitionListingState(ctx, 1, "l1", "listed", "unlisted", map[string]any{"unlisted_by": "admin"})
	require.NoError(t, err)
	require.Equal(t, "unlisted", row.State)
	require.Equal(t, "admin", row.UnlistedBy)
	require.NotNil(t, row.UnlistedAt)
	_, err = repo.TransitionListingState(ctx, 1, "l1", "listed", "unlisted", nil)
	require.ErrorIs(t, err, ErrAgentMarketplaceListingTransition) // 重复下架拒绝
	_, err = repo.TransitionListingState(ctx, 2, "l1", "listed", "unlisted", nil)
	require.ErrorIs(t, err, ErrAgentMarketplaceNotFound) // 跨租户 404
}

func TestDeprecateReleaseIsCASAndPointsAtSuccessor(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	for _, id := range []string{"r1", "r2"} {
		require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: id, ListingID: "l1", SubmissionID: "s" + id, AgentVersionID: "av", SourceAgentID: "a", ReleaseNumber: 1, SemanticVersion: "1.0.0", BundleDigest: "d", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")}).Error)
	}
	row, err := repo.DeprecateRelease(ctx, 1, "r1", "admin", "r2")
	require.NoError(t, err)
	require.Equal(t, "r2", row.SuccessorReleaseID)
	require.Equal(t, "admin", row.DeprecatedBy)
	require.NotNil(t, row.DeprecatedAt)
	_, err = repo.DeprecateRelease(ctx, 1, "r1", "admin", "r2")
	require.ErrorIs(t, err, ErrAgentReleaseDeprecateConflict) // 重复弃用拒绝
	_, err = repo.DeprecateRelease(ctx, 2, "r1", "admin", "r2")
	require.ErrorIs(t, err, ErrAgentMarketplaceNotFound) // 跨租户 404
}

func TestRetiredVariantAgentExists(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{TenantID: 1, ID: "v1", AdoptionID: "ad1", ReleaseID: "r1", Name: "sales", State: "retired", LocalAgentID: "agent-x"}).Error)
	ok, err := repo.RetiredVariantAgentExists(ctx, 1, "agent-x")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = repo.RetiredVariantAgentExists(ctx, 1, "agent-other")
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = repo.RetiredVariantAgentExists(ctx, 2, "agent-x")
	require.NoError(t, err)
	require.False(t, ok) // 跨租户不命中
}
```

文件头补 import：`"context"`、`"github.com/Tencent/WeKnora/internal/types"`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/repository/ -run 'TestEndAdoption|TestTransitionListingState|TestDeprecateRelease|TestRetiredVariantAgentExists' -count=1`
Expected: 编译 FAIL（`repo.EndAdoption undefined` 等）

- [ ] **Step 3: 实现 `agent_marketplace_lifecycle.go`**

新文件实现 Produces 列出的四个方法：`EndAdoption`/`RetiredVariantAgentExists` 挂 `agentAdoptionRepository`，`TransitionListingState`/`DeprecateRelease` 挂 `agentMarketplaceRepository`（全部 `r.db.WithContext(ctx)` 参数绑定；时间戳 `time.Now().UTC()`）。接口声明插入：`AgentAdoptionRepository`（`internal/application/repository/agent_adoption.go:52` `}` 前）加 `EndAdoption(...)` 与 `RetiredVariantAgentExists(...)` 两行；`AgentMarketplaceRepository`（`internal/application/repository/agent_marketplace.go:45` `GetListing` 行后）加 `TransitionListingState(...)` 与 `DeprecateRelease(...)` 两行。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/repository/ -run 'AgentMarketplaceLifecycle|EndAdoption|TransitionListingState|DeprecateRelease|RetiredVariantAgentExists' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/application/repository/agent_marketplace_lifecycle.go internal/application/repository/agent_adoption.go internal/application/repository/agent_marketplace.go internal/application/repository/agent_marketplace_lifecycle_test.go
git commit -m "feat(marketplace): 生命周期仓储原语——EndAdoption 事务闭合 + listing/release CAS + 退役归属查询"
```

---

### Task 3: lifecycle service 与 deprecate 行为闸（Adopt/CreateVariant/reconcile/Accept 拒绝弃用 Release）

**Files:**
- Create: `internal/types/interfaces/agent_marketplace_lifecycle.go`
- Create: `internal/application/service/agent_marketplace_lifecycle.go`
- Create: `internal/application/service/agent_marketplace_lifecycle_test.go`
- Modify: `internal/application/service/agent_adoption.go:81-87`（Adopt 的 release 读出后加弃用拒绝）与 `:135-141`（CreateVariant 同款）
- Modify: `internal/application/service/agent_upgrade.go:119-126`（AcceptUpgradeProposal 的 toRelease 读出后加弃用拒绝）与 `:270-276`（reconcileProposals 的 toRelease 读出后 skip）

**Interfaces:**
- Consumes: Task 2 全部仓储原语；既有 `interfaces.AdoptionView`/`AdoptionVariantView`/`TenantListingView`；既有 `service.UpdateVariantState` 的 paired-stamps 机制（`internal/application/repository/agent_adoption.go:242-249`：`retired_by` 自动补 `retired_at`）。
- Produces:
  - `interfaces.AgentMarketplaceLifecycleService` 接口（新文件）：
    - `RetireVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (interfaces.AdoptionVariantView, error)`
    - `EndAdoption(ctx context.Context, tenantID uint64, actorID, adoptionID string) (interfaces.AdoptionView, error)`
    - `UnlistListing(ctx context.Context, tenantID uint64, actorID, listingID string) (interfaces.TenantListingView, error)`
    - `DeprecateRelease(ctx context.Context, tenantID uint64, actorID, releaseID, successorReleaseID string) (*types.AgentReleaseEntity, error)`
  - `service.NewAgentMarketplaceLifecycleService(adoptions repository.AgentAdoptionRepository, listings repository.AgentMarketplaceRepository) *AgentMarketplaceLifecycleService`（第二参数用 repository 包接口而非 `interfaces.AgentMarketplaceRepository`——Task 2 只把 `TransitionListingState`/`DeprecateRelease` 加进 `internal/application/repository/agent_marketplace.go:29-46` 的接口，interfaces 版本（`internal/types/interfaces/agent_marketplace.go:8-26`）不含它们；且 `internal/container/container.go:321` 已 `Provide(repository.NewAgentMarketplaceRepository)`，DI 可解析）
  - 新哨兵：`service.ErrAgentReleaseDeprecated`（Adopt/CreateVariant/Accept 命中弃用 Release 时的 409 载体，消息携带 successor 提示）、`service.ErrAgentMarketplaceLifecycleInvalidInput`。
  - 行为闸（无新签名）：`AgentAdoptionService.Adopt`/`CreateVariant` 在 release 读出后、`DeprecatedAt != nil` 时拒绝；`AgentUpgradeService.AcceptUpgradeProposal` 在 toRelease `DeprecatedAt != nil` 时拒绝（`ErrAgentUpgradeStateConflict`）；`reconcileProposals` 对 `DeprecatedAt != nil` 的 toRelease `continue`。

- [ ] **Step 1: 写失败的 service 测试**

`internal/application/service/agent_marketplace_lifecycle_test.go`（RED：类型未定义，编译失败）：

```go
package service

// T33 #63 lifecycle service tests over the REAL migration stream
// (openAgentVersionServiceTestDB) + the REAL repositories. Lower-interface
// evidence below the Task 6 HTTP e2e — labeled as such.

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newLifecycleServiceForTest(t *testing.T) (*AgentMarketplaceLifecycleService, *AgentAdoptionService, *AgentUpgradeService, *gorm.DB) {
	t.Helper()
	db := openAgentVersionServiceTestDB(t)
	adoptions := repository.NewAgentAdoptionRepository(db)
	customAgents := NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil)
	versions := NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	return NewAgentMarketplaceLifecycleService(adoptions, repository.NewAgentMarketplaceRepository(db)),
		NewAgentAdoptionService(adoptions, customAgents, versions),
		NewAgentUpgradeService(repository.NewAgentUpgradeRepository(db)),
		db
}

// seedLifecycleFixture plants one adopted world on tenant 1: listing l1
// (current release r1), adoption ad1 (accepted r1) and — for the variants
// named in wantVariants — variant rows in the given states. v1 carries
// local_agent_id "agent-x" (published shape).
func seedLifecycleFixture(t *testing.T, db *gorm.DB, wantVariants map[string]string) {
	t.Helper()
	r1 := "r1"
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed", CurrentReleaseID: &r1}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "r1", ListingID: "l1", SubmissionID: "s1", AgentVersionID: "av1", SourceAgentID: "a", ReleaseNumber: 1, SemanticVersion: "1.0.0", BundleDigest: "d1", ManifestJSON: `{"capability_requirements":[]}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b")}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "ad1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active", CreatedBy: "admin"}).Error)
	for id, state := range wantVariants {
		localAgentID := ""
		if id == "v1" {
			localAgentID = "agent-x" // published shape
		}
		require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{TenantID: 1, ID: id, AdoptionID: "ad1", ReleaseID: "r1", Name: id, State: state, LocalAgentID: localAgentID}).Error)
	}
}

// seedListingTwo plants a second listing l2 with releases r2 (current) and
// r2b on tenant 1 — the deprecate pair.
func seedListingTwo(t *testing.T, db *gorm.DB) {
	t.Helper()
	r2 := "r2"
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l2", SourceAgentID: "a2", DisplayName: "d2", State: "listed", CurrentReleaseID: &r2}).Error)
	for _, row := range []types.AgentReleaseEntity{
		{TenantID: 1, ID: "r2", ListingID: "l2", SubmissionID: "s2", AgentVersionID: "av2", SourceAgentID: "a2", ReleaseNumber: 1, SemanticVersion: "2.0.0", BundleDigest: "d2", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")},
		{TenantID: 1, ID: "r2b", ListingID: "l2", SubmissionID: "s2b", AgentVersionID: "av2b", SourceAgentID: "a2", ReleaseNumber: 2, SemanticVersion: "2.1.0", BundleDigest: "d2b", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")},
	} {
		require.NoError(t, db.Create(&row).Error)
	}
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "ad-l2", ListingID: "l2", AcceptedReleaseID: "r2", State: "active", CreatedBy: "admin"}).Error)
}
```

```go
func TestRetireVariantIsCASAndKeepsRows(t *testing.T) {
	lifecycle, adoptions, _, db := newLifecycleServiceForTest(t)
	ctx := context.Background()
	seedLifecycleFixture(t, db, map[string]string{"v1": "published"})
	retired, err := lifecycle.RetireVariant(ctx, 1, "admin", "v1")
	require.NoError(t, err)
	require.Equal(t, "retired", retired.State)
	require.Equal(t, "admin", retired.RetiredBy)
	require.NotNil(t, retired.RetiredAt)

	// 已 retired 再 retire：CAS 冲突 409，绝不静默成功。
	_, err = lifecycle.RetireVariant(ctx, 1, "admin", "v1")
	require.ErrorIs(t, err, repository.ErrAgentAdoptionVariantTransition)

	// 迟到并发 map/test/publish 全部 409（Review Focus 1）。
	_, err = adoptions.UpdateCapabilityMapping(ctx, 1, "admin", "v1", nil)
	require.ErrorIs(t, err, repository.ErrAgentAdoptionRemapStateConflict)
	_, err = adoptions.TestVariant(ctx, 1, "admin", "v1")
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)

	// 行数不删（AC1 存储侧）：variant/adoption/release 行仍在。
	var variantCount, adoptionCount, releaseCount int64
	require.NoError(t, db.Model(&types.AgentAdoptionVariantEntity{}).Where("tenant_id = ?", 1).Count(&variantCount).Error)
	require.NoError(t, db.Model(&types.AgentAdoptionEntity{}).Where("tenant_id = ?", 1).Count(&adoptionCount).Error)
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("tenant_id = ?", 1).Count(&releaseCount).Error)
	require.EqualValues(t, 1, variantCount)
	require.EqualValues(t, 1, adoptionCount)
	require.EqualValues(t, 1, releaseCount)
}

func TestEndAdoptionGateAndPostconditions(t *testing.T) {
	lifecycle, adoptions, upgrades, db := newLifecycleServiceForTest(t)
	ctx := context.Background()
	seedLifecycleFixture(t, db, map[string]string{"v1": "draft"})
	// 残留 draft：EndAdoption 409（service 把 precondition 包为 StateConflict）。
	_, err := lifecycle.EndAdoption(ctx, 1, "admin", "ad1")
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)
	// 退役后 end 成功；返回视图 state=ended。
	require.NoError(t, db.Exec("UPDATE agent_adoption_variants SET state='retired' WHERE id='v1'").Error)
	view, err := lifecycle.EndAdoption(ctx, 1, "admin", "ad1")
	require.NoError(t, err)
	require.Equal(t, "ended", view.State)
	// ended 后：CreateVariant 409（#59 既有闸，钉死）。
	_, err = adoptions.CreateVariant(ctx, 1, "admin", "ad1", interfaces.VariantDraftInput{Name: "x"})
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)
	// ended 后：accept 既有 open 建议 409（#61 既有闸，钉死）。种 to=r1 的 open
	// proposal 一行（FindOrCreateProposal 不看 adoption 状态）：
	_, _, err = repository.NewAgentUpgradeRepository(db).FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
		TenantID: 1, AdoptionID: "ad1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r1", State: "open",
	})
	require.NoError(t, err)
	_, _, err = upgrades.AcceptUpgradeProposal(ctx, 1, "admin", "p1", interfaces.UpgradeVariantInput{Name: "y"})
	require.ErrorIs(t, err, ErrAgentUpgradeNotFound) // 无效 id 先证 404 面，再证真 id 的 409 面
	proposals, err := upgrades.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	_, _, err = upgrades.AcceptUpgradeProposal(ctx, 1, "admin", proposals[0].ID, interfaces.UpgradeVariantInput{Name: "y"})
	require.ErrorIs(t, err, ErrAgentUpgradeStateConflict) // 真 id：adoption 非 active → 409
}

func TestUnlistAndDeprecateBehaviorDiffers(t *testing.T) {
	lifecycle, adoptions, _, db := newLifecycleServiceForTest(t)
	ctx := context.Background()
	seedLifecycleFixture(t, db, nil)
	seedListingTwo(t, db)
	view, err := lifecycle.UnlistListing(ctx, 1, "admin", "l1")
	require.NoError(t, err)
	require.Equal(t, "unlisted", view.State)
	// Unlisted：新 Adoption 拒（既有闸 agent_adoption.go:74）。
	_, _, err = adoptions.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: "l1"})
	require.ErrorIs(t, err, ErrAgentAdoptionInvalidInput)
	// Deprecated（service 面行为闸；catalog 差异由 Task 6 e2e 断言）：
	release, err := lifecycle.DeprecateRelease(ctx, 1, "admin", "r2", "r2b")
	require.NoError(t, err)
	require.Equal(t, "r2b", release.SuccessorReleaseID)
	// 显式 adopt 弃用 release → 409 且消息带 successor（行为闸）。
	_, _, err = adoptions.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: "l2", ReleaseID: "r2"})
	require.ErrorIs(t, err, ErrAgentReleaseDeprecated)
	require.Contains(t, err.Error(), "r2b")
	// CreateVariant 显式指向弃用 release 同拒。
	_, err = adoptions.CreateVariant(ctx, 1, "admin", "ad-l2", interfaces.VariantDraftInput{Name: "z", ReleaseID: "r2"})
	require.ErrorIs(t, err, ErrAgentReleaseDeprecated)
}

func TestDeprecateSuccessorValidation(t *testing.T) {
	lifecycle, _, _, db := newLifecycleServiceForTest(t)
	ctx := context.Background()
	seedLifecycleFixture(t, db, nil)
	seedListingTwo(t, db)
	// 异 listing r3 与已弃用 r4（successor=r2b）各一行：
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l3", SourceAgentID: "a3", DisplayName: "d3", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "r3", ListingID: "l3", SubmissionID: "s3", AgentVersionID: "av3", SourceAgentID: "a3", ReleaseNumber: 1, SemanticVersion: "3.0.0", BundleDigest: "d3", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")}).Error)
	now := time.Now().UTC()
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "r4", ListingID: "l2", SubmissionID: "s4", AgentVersionID: "av4", SourceAgentID: "a2", ReleaseNumber: 3, SemanticVersion: "2.2.0", BundleDigest: "d4", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b"), DeprecatedAt: &now, DeprecatedBy: "admin", SuccessorReleaseID: "r2b"}).Error)
	for _, tc := range []struct{ name, successor string }{
		{"missing", "nope"}, {"self", "r1"}, {"cross-listing", "r3"}, {"already-deprecated", "r4"}, {"empty", ""},
	} {
		_, err := lifecycle.DeprecateRelease(ctx, 1, "admin", "r1", tc.successor)
		require.ErrorIs(t, err, ErrAgentMarketplaceLifecycleInvalidInput, tc.name)
	}
	_, err := lifecycle.DeprecateRelease(ctx, 1, "admin", "missing-release", "r2b")
	require.ErrorIs(t, err, repository.ErrAgentMarketplaceNotFound)
}

func TestUpgradeReconcileSkipsDeprecatedTarget(t *testing.T) {
	lifecycle, _, upgrades, db := newLifecycleServiceForTest(t)
	ctx := context.Background()
	// l1 上前进到 r2（同 listing 新 release，current 指向它）。
	seedLifecycleFixture(t, db, nil)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "r2", ListingID: "l1", SubmissionID: "s2", AgentVersionID: "av2", SourceAgentID: "a", ReleaseNumber: 2, SemanticVersion: "1.1.0", BundleDigest: "d2", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")}).Error)
	require.NoError(t, db.Exec("UPDATE agent_marketplace_listings SET current_release_id='r2' WHERE id='l1'").Error)
	// 弃用 r2（successor=r1：同 listing、未弃用）→ reconcile 不生成建议。
	_, err := lifecycle.DeprecateRelease(ctx, 1, "admin", "r2", "r1")
	require.NoError(t, err)
	proposals, err := upgrades.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, proposals)
	// 对照：解除 r2 弃用后同一前进会生成 to=r2 的建议。
	require.NoError(t, db.Exec("UPDATE agent_releases SET deprecated_at=NULL, successor_release_id='' WHERE id='r2'").Error)
	proposals, err = upgrades.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	require.Equal(t, "r2", proposals[0].ToReleaseID)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/service/ -run 'TestRetireVariantIsCAS|TestEndAdoptionGate|TestUnlistAndDeprecate|TestDeprecateSuccessor|TestUpgradeReconcileSkipsDeprecated' -count=1`
Expected: 编译 FAIL（`undefined: NewAgentMarketplaceLifecycleService` / `ErrAgentReleaseDeprecated`）

- [ ] **Step 3: 实现 service 与行为闸**

`internal/types/interfaces/agent_marketplace_lifecycle.go`：定义 Produces 所列四方法接口。`internal/application/service/agent_marketplace_lifecycle.go`（struct：`type AgentMarketplaceLifecycleService struct { adoptions repository.AgentAdoptionRepository; listings repository.AgentMarketplaceRepository; now func() time.Time }`，`now` 缺省 `time.Now`）：

- `RetireVariant`：trim 输入→`GetVariant`（nil→`ErrAgentAdoptionNotFound`）→`s.adoptions.UpdateVariantState(ctx, tenantID, variantID, []string{draft,mapped,tested,published}, "retired", map[string]any{"retired_by": actorID})`（paired stamps 自动补 `retired_at`）→`variantViewOf` 返回。
  - **view 构造裁定（实现者不得自行发明结构）**：`variantView`/`adoptionView` 是 `*AgentAdoptionService` 的私有方法（`internal/application/service/agent_adoption.go:358/:374`），lifecycle service 调不到。在 lifecycle service 上**复制**这两个方法为私有 `variantViewOf(ctx, tenantID, variant *types.AgentAdoptionVariantEntity) (interfaces.AdoptionVariantView, error)` 与 `adoptionViewOf(ctx, tenantID, row *types.AgentAdoptionEntity) (interfaces.AdoptionView, error)`（镜像 agent_adoption.go:358-380 的实现形状：后者 `ListVariantsByAdoption` 逐个前者），依赖零新增——`missingCapabilities` 纯函数与 `releaseManifest(ctx, repo adoptionReleaseReader, ...)`（仅要求 `GetRelease`，`s.adoptions` 满足）均为包级可复用。不修改 agent_adoption.go 的私有方法（并行冲突最小）。
- `EndAdoption`：trim→`s.adoptions.EndAdoption(ctx, tenantID, adoptionID, AgentAdoptionStateActive, AgentAdoptionStateEnded, map[string]any{"ended_by": actorID})`，`ErrAgentAdoptionEndPrecondition` 上抛时包成 `fmt.Errorf("%w: %v", ErrAgentAdoptionStateConflict, err)`（HTTP 409 单一映射面）→`adoptionViewOf`。
- `UnlistListing`：trim→`s.listings.TransitionListingState(ctx, tenantID, listingID, "listed", "unlisted", map[string]any{"unlisted_by": actorID})`→`interfaces.TenantListingView{AgentMarketplaceListingEntity: *row}`。
- `DeprecateRelease`：trim→successor 必填非空、≠releaseID→`s.listings.GetRelease(releaseID)`（nil→`ErrAgentMarketplaceNotFound`）→`s.listings.GetRelease(successor)`（nil→`ErrAgentMarketplaceLifecycleInvalidInput: successor release ... not found`）→successor.ListingID==release.ListingID、successor.DeprecatedAt==nil→`s.listings.DeprecateRelease`。 新常量 `AgentAdoptionStateEnded = "ended"`（放本文件，与 `AgentAdoptionStateActive` 相邻语义）。
- 行为闸三处（各 ~4 行）：`agent_adoption.go` Adopt 在 `:87` release 归属校验后、CreateVariant 在 `:141` 后加 `if release.DeprecatedAt != nil { return ..., fmt.Errorf("%w: %w: release %s is deprecated; successor: %s", ErrAgentReleaseDeprecated, ErrAgentAdoptionStateConflict, releaseID, successorHint(release.SuccessorReleaseID)) }`（multi-`%w` 双哨兵：service 测试 `errors.Is(err, ErrAgentReleaseDeprecated)` 成立，而既有 `adoptionClientError`（`internal/handler/agent_adoption.go:117-123`）命中 `ErrAgentAdoptionStateConflict` → 409 且消息透传 successor——零 handler 改动；`successorHint` 为包内小 helper：空时返回 `"none declared"`）；`agent_upgrade.go` AcceptUpgradeProposal 在 `:126` 后同款拒绝（哨兵 `ErrAgentUpgradeStateConflict` 前置包裹，`internal/handler/agent_upgrade.go:71-87` 的 `upgradeClientError` 已把该哨兵映射 `NewConflictError(err.Error())` 409 透传）、reconcileProposals 在 `:276` 后 `if toRelease.DeprecatedAt != nil { continue }`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/service/ -run 'Lifecycle|RetireVariant|EndAdoption|UnlistAndDeprecate|DeprecateSuccessor|UpgradeReconcileSkipsDeprecated|AgentAdoption|AgentUpgrade' -count=1`
Expected: PASS（含 #59/#61 既有套件不回归）

- [ ] **Step 5: Commit**

```bash
git add internal/types/interfaces/agent_marketplace_lifecycle.go internal/application/service/agent_marketplace_lifecycle.go internal/application/service/agent_marketplace_lifecycle_test.go internal/application/service/agent_adoption.go internal/application/service/agent_upgrade.go
git commit -m "feat(marketplace): lifecycle 服务与 deprecate 行为闸——弃用 Release 阻断新引入、reconcile 跳过弃用目标"
```

---

### Task 4: HTTP 面（四端点 + 治理地板 + 容器接线 + e2e happy path）

**Files:**
- Create: `internal/handler/agent_marketplace_lifecycle.go`
- Create: `internal/router/routes_agent_marketplace_lifecycle.go`
- Create: `internal/router/routes_agent_marketplace_lifecycle_test.go`
- Modify: `internal/router/router.go:107-108`（RouterParams 加字段）与 `:430`（注册块加一行）
- Modify: `internal/container/container.go:460`（`must(container.Provide(handler.NewAgentUpgradeHandler))` 行后加两段 Provide）

**Interfaces:**
- Consumes: Task 3 的 `*service.AgentMarketplaceLifecycleService`；handler 包既有 `decodeAgentMarketplaceBody`/`invalidMarketplaceBody`/`agentMarketplaceMaxRequestBytes`/`sandboxConfigTenantID`（`internal/handler/agent_marketplace.go:21,153,169`）与 `adoptionVariantDTO`/`adoptionDTO`（`internal/handler/agent_adoption.go:91,101`）；router 包既有 `apiKeyFullAccess`/`g.apiKeyRoute`/`g.Admin()`。
- Produces:
  - `handler.NewAgentMarketplaceLifecycleHandler(lifecycle *service.AgentMarketplaceLifecycleService) *AgentMarketplaceLifecycleHandler`
  - 四方法 `RetireVariant(c *gin.Context)` / `EndAdoption(c *gin.Context)` / `UnlistListing(c *gin.Context)` / `DeprecateRelease(c *gin.Context)`——前三无 body；deprecate body `{"successor_release_id": string}`（必填）。
  - `router.RegisterAgentMarketplaceLifecycleRoutes(r *gin.RouterGroup, lifecycleHandler *handler.AgentMarketplaceLifecycleHandler, g *rbacGuards)`——四条 POST 全 `apiKeyFullAccess()` + `g.Admin()`，nil handler 不挂载（fail closed）。
  - 路由：`/marketplace/tenant/variants/:id/retire`、`/marketplace/tenant/adoptions/:id/end`、`/marketplace/tenant/listings/:id/unlist`、`/marketplace/tenant/releases/:id/deprecate`。
  - `lifecycleClientError(err) error` 映射：`ErrAgentAdoptionNotFound`/`ErrAgentMarketplaceNotFound`→404；`ErrAgentAdoptionStateConflict`/`ErrAgentAdoptionVariantNotRunnable`/`ErrAgentReleaseDeprecated`/`ErrAgentAdoptionVariantTransition`/`ErrAgentAdoptionTransition`/`ErrAgentAdoptionRemapStateConflict`/`ErrAgentAdoptionEndPrecondition`/`ErrAgentMarketplaceListingTransition`/`ErrAgentReleaseDeprecateConflict`→409（消息透传）；`ErrAgentAdoptionInvalidInput`/`ErrAgentMarketplaceInvalidInput`/`ErrAgentMarketplaceLifecycleInvalidInput`→400。
  - deprecate 响应 DTO：`gin.H{"success":true,"data":lifecycleReleaseResponse{ID, ListingID, SemanticVersion, DeprecatedAt, DeprecatedBy, SuccessorReleaseID}}`（json tag snake_case，时间 `*time.Time` omitempty）。

- [ ] **Step 1: 写失败的路由治理与 e2e happy path 测试**

`internal/router/routes_agent_marketplace_lifecycle_test.go`（RED：路由未注册，404）：

```go
package router

// T33 #63 lifecycle HTTP e2e. Real sqlite migration stream + real services
// + real handlers + httptest — the AC3 highest-stable-interface evidence.

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAgentMarketplaceLifecycleRoutesRequireAdminAndFullAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")
	RegisterAgentMarketplaceLifecycleRoutes(v1, handler.NewAgentMarketplaceLifecycleHandler(nil), g)
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/marketplace/tenant/variants/:id/retire"},
		{http.MethodPost, "/api/v1/marketplace/tenant/adoptions/:id/end"},
		{http.MethodPost, "/api/v1/marketplace/tenant/listings/:id/unlist"},
		{http.MethodPost, "/api/v1/marketplace/tenant/releases/:id/deprecate"},
	} {
		policy := mustLookupAPIKeyPolicy(t, g, route.method, route.path)
		require.Truef(t, policy.RequireFullAccess, "%s %s 必须要求 full-access", route.method, route.path)
	}
	bare := gin.New()
	RegisterAgentMarketplaceLifecycleRoutes(bare.Group("/api/v1"), nil, &rbacGuards{})
	require.Empty(t, bare.Routes()) // nil handler fail closed
}

// newLifecycleTestApp mirrors newAgentUpgradeTestApp (routes_agent_upgrade_test.go:64)
// without modifying it: real migration DB, real marketplace/adoption/upgrade/
// lifecycle services and handlers, identity middleware from X-Test-* headers.
func newLifecycleTestApp(t *testing.T) (*gin.Engine, *rbacGuards, *gorm.DB) {
	t.Helper()
	db := openTenantAgentMarketplaceHTTPTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{ID: "agent-owned", Name: "Lifecycle helper", TenantID: 1, CreatedBy: "contributor", Config: types.CustomAgentConfig{AgentMode: "smart-reasoning", SystemPrompt: "Be useful."}}).Error)
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil)
	versions := service.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	market := service.NewAgentMarketplaceService(versions, marketplaceHTTPResolver{}, repository.NewAgentMarketplaceRepository(db), t.TempDir())
	adoptionsRepo := repository.NewAgentAdoptionRepository(db)
	adoptions := service.NewAgentAdoptionService(adoptionsRepo, customAgents, versions)
	upgrades := service.NewAgentUpgradeService(repository.NewAgentUpgradeRepository(db))
	lifecycle := service.NewAgentMarketplaceLifecycleService(adoptionsRepo, repository.NewAgentMarketplaceRepository(db))

	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}, agentCreator: func(c *gin.Context) (string, error) {
		if c.Param("id") == "agent-owned" {
			return "contributor", nil
		}
		return "", nil
	}}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(newLifecycleIdentityMiddleware())
	v1 := r.Group("/api/v1")
	RegisterAgentVersionRoutes(v1, handler.NewAgentVersionHandler(versions), g)
	RegisterAgentMarketplaceRoutes(v1, handler.NewAgentMarketplaceHandler(market, versions), g)
	RegisterAgentAdoptionRoutes(v1, handler.NewAgentAdoptionHandler(adoptions), g)
	RegisterAgentUpgradeRoutes(v1, handler.NewAgentUpgradeHandler(upgrades), g)
	RegisterAgentMarketplaceLifecycleRoutes(v1, handler.NewAgentMarketplaceLifecycleHandler(lifecycle), g)
	return r, g, db
}

// newLifecycleIdentityMiddleware：逐字复制 routes_agent_upgrade_test.go:88-101
// 的匿名 middleware（tenant 1/2、X-Test-Actor、X-Test-Role），独立函数以便
// workbench start 路由共用（Task 6）。

// lifecycleMetadata is the shared release-metadata fixture (Task 4/6 复用)。
func lifecycleMetadata() map[string]any {
	return map[string]any{"supported_languages": []string{"en"}, "use_cases": []string{"support"},
		"capability_requirements": []string{"model", "knowledge"}, "minimum_weknora_capability": "1", "license_id": "MIT"}
}

func TestLifecycleEndpointsHappyPathOverRealStack(t *testing.T) {
	r, _, _ := newLifecycleTestApp(t)
	listingID, v1 := freezeAndPublishUpgradeRelease(t, r, "1.0.0", lifecycleMetadata())
	_, v2 := freezeAndPublishUpgradeRelease(t, r, "1.1.0", lifecycleMetadata())

	// adopt + variant 全链到 published（复用既有 helper）。
	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code)
	var adoptionBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptionBody))
	variantID, _ := publishUpgradeVariant(t, r, adoptionBody.Data.ID, "Sales", "gpt-x", "kb-sales")

	// retire：200 + state=retired。
	retired := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/retire", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, retired.Code)
	require.Contains(t, retired.Body.String(), `"state":"retired"`)

	// end adoption：全部退役后 200 + state=ended。
	ended := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/end", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, ended.Code)
	require.Contains(t, ended.Body.String(), `"state":"ended"`)

	// unlist：200 + state=unlisted；catalog 随即不含该 listing。
	unlisted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/listings/"+listingID+"/unlist", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, unlisted.Code)
	require.Contains(t, unlisted.Body.String(), `"state":"unlisted"`)
	catalog := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/catalog", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, catalog.Code)
	require.NotContains(t, catalog.Body.String(), listingID)

	// deprecate：200 + successor 回显；重复弃用 409。
	deprecated := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/releases/"+v1+"/deprecate", "admin", "admin", map[string]any{"successor_release_id": v2})
	require.Equal(t, http.StatusOK, deprecated.Code)
	require.Contains(t, deprecated.Body.String(), `"successor_release_id":"`+v2+`"`)
	repeat := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/releases/"+v1+"/deprecate", "admin", "admin", map[string]any{"successor_release_id": v2})
	require.Equal(t, http.StatusConflict, repeat.Code)

	// 跨租户谓词（Review Focus 5）：tenant 2 对同 id 四端点全 404。
	require.Equal(t, http.StatusNotFound, adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/retire", "admin", "admin", nil).Code)
	require.Equal(t, http.StatusNotFound, adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/end", "admin", "admin", nil).Code)
	require.Equal(t, http.StatusNotFound, adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/listings/"+listingID+"/unlist", "admin", "admin", nil).Code)
	require.Equal(t, http.StatusNotFound, adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/releases/"+v1+"/deprecate", "admin", "admin", map[string]any{"successor_release_id": v2}).Code)
}
```

（import 追加：`"encoding/json"`。`newLifecycleIdentityMiddleware` 的函数体逐字取自 `routes_agent_upgrade_test.go` `newAgentUpgradeTestApp` 内 `r.Use(func(c *gin.Context){...})` 的匿名 middleware（tenant 1/2、`X-Test-Actor`、`X-Test-Role`，末尾 `c.Next()`），提为具名函数供 workbench start 路由共用。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/router/ -run 'TestAgentMarketplaceLifecycleRoutesRequireAdminAndFullAccess|TestLifecycleEndpointsHappyPathOverRealStack' -count=1`
Expected: 编译 FAIL（`RegisterAgentMarketplaceLifecycleRoutes` / `NewAgentMarketplaceLifecycleHandler` undefined）

- [ ] **Step 3: 实现 handler、路由注册与接线**

`internal/handler/agent_marketplace_lifecycle.go`：按 Produces 定义 handler 四方法（`c.Param("id")` 取路径 id；deprecate 解析 body 后 `strings.TrimSpace(successor) == ""` → `invalidMarketplaceBody`）；`lifecycleClientError` 映射（错误集见 Produces；`errors.Is` 逐哨兵）。`internal/router/routes_agent_marketplace_lifecycle.go`：注册函数（四路由全 admin 地板，nil fail closed）。`router.go`：`RouterParams` 在 `AgentUpgradeHandler`（:108）后加 `AgentMarketplaceLifecycleHandler *handler.AgentMarketplaceLifecycleHandler`；注册块在 `RegisterAgentUpgradeRoutes`（:430）后加 `RegisterAgentMarketplaceLifecycleRoutes(v1, params.AgentMarketplaceLifecycleHandler, rbacGuards)`。`container.go` 在 `:460` 后加：

```go
must(container.Provide(service.NewAgentMarketplaceLifecycleService))
must(container.Provide(handler.NewAgentMarketplaceLifecycleHandler))
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/router/ -run 'AgentMarketplaceLifecycle|LifecycleEndpoints' -count=1 && go build ./...`
Expected: PASS + build ok

- [ ] **Step 5: Commit**

```bash
git add internal/handler/agent_marketplace_lifecycle.go internal/router/routes_agent_marketplace_lifecycle.go internal/router/routes_agent_marketplace_lifecycle_test.go internal/router/router.go internal/container/container.go
git commit -m "feat(marketplace): lifecycle 四端点——Admin+ full-access 治理地板 + 容器接线"
```

---

### Task 5: 运行闸——retired 变体的本地 Agent 不得 admit 新工作

**Files:**
- Create: `internal/modules/workbench/service/workbench/admission_agent_use_test.go`
- Modify: `internal/modules/workbench/service/workbench/admission.go:141-160`（AdmissionCoordinator 加 `agentUseGate` 字段 + `SetAgentUseGate`）与 `:304-312`（Start 中 TargetID 规范化后、`requestHash` 前插入检查）
- Modify: `internal/handler/session/workbench_start.go:88-100`（writeWorkbenchAdmissionError 加一条映射）
- Modify: `internal/container/workbench.go:61-70`（NewWorkbenchAdmissionCoordinator 加 `adoptions repository.AgentAdoptionRepository` 参数并注入 gate）

**Interfaces:**
- Consumes: Task 2 的 `AgentAdoptionRepository.RetiredVariantAgentExists(ctx, tenantID, localAgentID) (bool, error)`；既有 `AdmissionCoordinator` / `SetAdmissionGate` seam 先例（`admission.go:155-166`）。
- Produces:
  - `workbenchservice.ErrAgentUseDenied = errors.New("agent use denied")`（新哨兵，`admission.go` 顶部 var 块）。
  - `(*AdmissionCoordinator).SetAgentUseGate(gate func(ctx context.Context, tenantID uint64, agentID string) error)`——nil 清除。
  - Start 行为：`agentUseGate != nil && strings.TrimSpace(in.AgentID) != ""` 时在 `CreatePending` 之前咨询，错误原样返回（不落任何 durable 行）。
  - HTTP 映射：`writeWorkbenchAdmissionError` 中 `errors.Is(err, workbenchservice.ErrAgentUseDenied)` → 409。
  - 容器：`NewWorkbenchAdmissionCoordinator(cfg *config.Config, db *gorm.DB, runs *repository.AgentRunStore, targets repository.ExecutionTargetStore, adoptions repository.AgentAdoptionRepository)` 尾部注入 `coordinator.SetAgentUseGate(...)`：闭包内 `RetiredVariantAgentExists` 为真时 `fmt.Errorf("%w: agent %s belongs to a retired variant", workbenchservice.ErrAgentUseDenied, agentID)`。

- [ ] **Step 1: 写失败的 admission 单测**

`admission_agent_use_test.go`（RED：SetAgentUseGate 未定义，编译失败）：

```go
package workbench

// T33 #63: the agent-use gate seam. A retired variant's local agent must
// not admit NEW work; starts without an agent (or with a live agent) pass
// untouched. Store-level evidence; the HTTP face is Task 6.

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestAdmissionAgentUseGateBlocksRetiredBeforePersisting(t *testing.T) {
	db := openAdmissionConcurrencyDB(t) // admission_concurrency_test.go:164，真实迁移流
	coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), nil, nil) // budget nil → NoopTaskBudget（admission_test.go:33 同款）
	calls := 0
	coordinator.SetAgentUseGate(func(_ context.Context, _ uint64, agentID string) error {
		calls++
		if agentID == "agent-retired" {
			return ErrAgentUseDenied
		}
		return nil
	})
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")

	// 退役变体的 agent：拒绝且零 durable 写（Review Focus 4——CreatePending 前）。
	_, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "agent-retired", TargetID: "platform", RequestID: "r1", Text: "hi", BudgetUpper: 1})
	require.ErrorIs(t, err, ErrAgentUseDenied)
	var pending int64
	require.NoError(t, db.Table("workbench_requests").Where("request_id = ?", "r1").Count(&pending).Error)
	require.EqualValues(t, 0, pending)

	// 健康agent：正常 admit。
	run, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "agent-live", TargetID: "platform", RequestID: "r2", Text: "hi", BudgetUpper: 1})
	require.NoError(t, err)
	require.NotEmpty(t, run.Key.RunID)

	// 无 agent 的 start：不咨询 gate（calls 不增）。
	before := calls
	_, err = coordinator.Start(ctx, StartInput{SessionID: "s1", TargetID: "platform", RequestID: "r3", Text: "hi", BudgetUpper: 1})
	require.NoError(t, err)
	require.Equal(t, before, calls)

	// nil gate：直接放行（既有部署不受影响）。
	coordinator.SetAgentUseGate(nil)
	_, err = coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "agent-retired", TargetID: "platform", RequestID: "r4", Text: "hi", BudgetUpper: 1})
	require.NoError(t, err)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/workbench/service/workbench/ -run TestAdmissionAgentUseGate -count=1`
Expected: 编译 FAIL（`coordinator.SetAgentUseGate undefined` / `ErrAgentUseDenied undefined`）

- [ ] **Step 3: 实现 seam、映射与容器注入**

`admission.go`：var 块加 `ErrAgentUseDenied`；`AdmissionCoordinator` struct 加字段 `agentUseGate func(context.Context, uint64, string) error`；`SetAdmissionGate` 后加 `SetAgentUseGate`（同形）；`Start` 在 `if in.TargetID == "" { in.TargetID = "platform" }`（:309-311）后插入：

```go
	// T33 #63: a retired variant's local agent stops admitting NEW work. The
	// gate is consulted before the durable request row so a refusal leaves
	// zero side effects (mirrors the W34 gate discipline above).
	if a.agentUseGate != nil && strings.TrimSpace(in.AgentID) != "" {
		if err := a.agentUseGate(ctx, tenant, in.AgentID); err != nil {
			return agentruntime.Run{}, err
		}
	}
```

`workbench_start.go` 的 `writeWorkbenchAdmissionError` switch 加 `case errors.Is(err, workbenchservice.ErrAgentUseDenied): status = http.StatusConflict`。`workbench.go` 的 `NewWorkbenchAdmissionCoordinator` 签名加 `adoptions repository.AgentAdoptionRepository`，`SetAdmissionGate` 行后加 `coordinator.SetAgentUseGate(...)`（闭包见 Produces；查询失败原样返回 error——fail closed）。

- [ ] **Step 4: 运行测试确认通过 + 既有 admission 套件不回归**

Run: `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmission' -count=1 && go build ./...`
Expected: PASS + build ok

- [ ] **Step 5: Commit**

```bash
git add internal/modules/workbench/service/workbench/admission_agent_use_test.go internal/modules/workbench/service/workbench/admission.go internal/handler/session/workbench_start.go internal/container/workbench.go
git commit -m "feat(workbench): admission agent-use 闸——退役变体的本地 Agent 拒绝新工作、零 durable 写"
```

---

### Task 6: 验收证据——AC1 保留计数 / AC2 Unlisted-vs-Deprecated 差异 / 运行闸端到端

**Files:**
- Test: `internal/router/routes_agent_marketplace_lifecycle_test.go`（追加三个测试；本任务无生产代码——若断言失败说明 Task 2-5 有缺陷，回改对应任务文件）

**Interfaces:**
- Consumes: Task 4 的 `newLifecycleTestApp`/`newLifecycleIdentityMiddleware`、既有 `freezeAndPublishUpgradeRelease`/`publishUpgradeVariant`/`adoptionCall`；Task 5 的 `SetAgentUseGate`；既有 `workbenchservice.NewAdmissionCoordinator` + `repository.NewAgentRunStore`（`workbench_start_integration_test.go:53` 同款装配）。
- Produces: AC1/AC2/AC3 的端到端证据测试三件（本 Issue 收口物）。

- [ ] **Step 1: 追加三个验收测试与本文件私有 helper**

先写文件私有 helper（完整代码，追加在 `routes_agent_marketplace_lifecycle_test.go`）：

```go
// decodeAdoptionID reads the adoption id from an Adopt/List response row.
func decodeAdoptionID(t *testing.T, raw []byte) string {
	t.Helper()
	var body struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.NotEmpty(t, body.Data.ID)
	return body.Data.ID
}

// adoptAndPublishFirstVariant adopts listingID then drives the variant to
// published via the existing publishUpgradeVariant helper; returns the
// adoption id, variant id and instantiated local agent id.
func adoptAndPublishFirstVariant(t *testing.T, r *gin.Engine, listingID string) (adoptionID, variantID, localAgentID string) {
	t.Helper()
	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	adoptionID = decodeAdoptionID(t, adopted.Body.Bytes())
	variantID, localAgentID = publishUpgradeVariant(t, r, adoptionID, "Sales", "gpt-x", "kb-sales")
	return adoptionID, variantID, localAgentID
}

// httptestPostJSON posts a JSON body through the real engine.
func httptestPostJSON(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", "admin")
	req.Header.Set("X-Test-Actor", "admin")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// newRealAgentUseGate wraps the Task 2 repository query into the Task 5
// seam — the exact closure production wiring installs
// (internal/container/workbench.go).
func newRealAgentUseGate(db *gorm.DB) func(context.Context, uint64, string) error {
	adoptions := repository.NewAgentAdoptionRepository(db)
	return func(ctx context.Context, tenantID uint64, agentID string) error {
		retired, err := adoptions.RetiredVariantAgentExists(ctx, tenantID, agentID)
		if err != nil {
			return err
		}
		if retired {
			return fmt.Errorf("%w: agent %s belongs to a retired variant", workbenchservice.ErrAgentUseDenied, agentID)
		}
		return nil
	}
}
```

import 追加：`context`、`fmt`、`net/http/httptest`、`strings`、`workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"`、`session "github.com/Tencent/WeKnora/internal/handler/session"`（已核实 `internal/handler/session` 不 import `internal/router`，无环）。再追加三个验收测试：

```go
// AC1：四类退出操作前后，版本/许可/审计(审核)行零删除；本地 agent 行零删除零软删。
func TestLifecycleExitDeletesNothingAcrossGovernanceRows(t *testing.T) {
	r, _, db := newLifecycleTestApp(t)
	require.NoError(t, db.Exec("INSERT INTO agent_licenses (id,name,allows_redistribution,created_at,updated_at) VALUES ('lic-1','MIT',1,DATETIME('now'),DATETIME('now'))").Error)
	listingID, v1 := freezeAndPublishUpgradeRelease(t, r, "1.0.0", lifecycleMetadata())
	adoptionID, variantID, localAgentID := adoptAndPublishFirstVariant(t, r, listingID)
	// deprecate 的 successor=v1 需同 listing 另有一个 release：先再发布 2.0.0，
	// 再对治理行集取样（发布本身会增加行数，取样必须在其后）。
	_, v2 := freezeAndPublishUpgradeRelease(t, r, "2.0.0", lifecycleMetadata())

	count := func(table string) int64 {
		var n int64
		// agent_licenses 是平台级表（无 tenant_id 列，#62 DDL），其余全部租户谓词。
		if table == "agent_licenses" {
			require.NoError(t, db.Table(table).Count(&n).Error)
		} else {
			require.NoError(t, db.Table(table).Where("tenant_id = ?", 1).Count(&n).Error)
		}
		return n
	}
	before := map[string]int64{
		"agent_releases": count("agent_releases"), "agent_release_reviews": count("agent_release_reviews"),
		"agent_versions": count("agent_versions"), "agent_adoptions": count("agent_adoptions"),
		"agent_adoption_variants": count("agent_adoption_variants"), "custom_agents": count("custom_agents"),
		"agent_licenses": count("agent_licenses"), "agent_release_submissions": count("agent_release_submissions"),
		// AC1 的「Task」字面：Task 载体表（agent_runs 000014:13 / workbench_requests
		// 000056:2 均有 tenant_id 列）——四类退出生产路径不触碰它们，计数断言
		// 提供直接证据（本测试未创建 run 时 0==0 平凡真，真值承载见 Task 5 拒绝
		// 路径零 durable 写断言）。
		"agent_runs": count("agent_runs"), "workbench_requests": count("workbench_requests"),
	}

	// 四类退出全做一遍：retire → end → unlist → deprecate。
	require.Equal(t, http.StatusOK, adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/retire", "admin", "admin", nil).Code)
	require.Equal(t, http.StatusOK, adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionID+"/end", "admin", "admin", nil).Code)
	require.Equal(t, http.StatusOK, adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/listings/"+listingID+"/unlist", "admin", "admin", nil).Code)
	deprecated := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/releases/"+v2+"/deprecate", "admin", "admin", map[string]any{"successor_release_id": v1})
	require.Equal(t, http.StatusOK, deprecated.Code, deprecated.Body.String())

	for table, n := range before {
		require.EqualValues(t, n, count(table), "退出不得删除 %s 行", table)
	}
	// 本地 agent 零软删且 GET /api/v1/agents 仍返回（已有本地版本可继续，spec §10）。
	var deletedCount int64
	require.NoError(t, db.Table("custom_agents").Where("id = ? AND deleted_at IS NOT NULL", localAgentID).Count(&deletedCount).Error)
	require.EqualValues(t, 0, deletedCount)
	found, _, _ := agentsRowByName(t, r, "Sales")
	require.True(t, found)
}

// AC2：Unlisted 与 Deprecated 的行为差异（同一稳定 Interface 上的对照矩阵）。
func TestLifecycleUnlistedVersusDeprecatedBehaviorDiffers(t *testing.T) {
	r, _, _ := newLifecycleTestApp(t)
	meta := lifecycleMetadata()
	listingID, _ := freezeAndPublishUpgradeRelease(t, r, "1.0.0", meta)

	// —— Unlisted 面：catalog 消失 + 新 Adoption 拒 + 既有 Adoption 的升级路径不断。
	unlistedAdopter := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, unlistedAdopter.Code) // 下架前 adopt 成功
	require.Equal(t, http.StatusOK, adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/listings/"+listingID+"/unlist", "admin", "admin", nil).Code)
	catalog := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/catalog", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, catalog.Code)
	require.NotContains(t, catalog.Body.String(), listingID)    // 新发现=否（差异点：catalog 消失）
	reAdopt := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusBadRequest, reAdopt.Code)       // 新 Adoption=否（listing.state 门走既有 ErrAgentAdoptionInvalidInput→400）
	_, v3 := freezeAndPublishUpgradeRelease(t, r, "1.2.0", meta)
	proposals := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Contains(t, proposals.Body.String(), `"to_release_id":"`+v3+`"`) // 既有 Adoption 升级建议继续（CONTEXT.md：不撤销 Release/不影响既有 Adoption）

	// —— Deprecated 面（第二个 listing，避免上一面的 unlisted 干扰）：catalog 仍
	// 可见 + 显式引入拒且指向 successor + 默认 adopt（current 未弃用）成功。
	listing2, w1 := freezeAndPublishUpgradeRelease(t, r, "2.0.0", meta)
	_, w2 := freezeAndPublishUpgradeRelease(t, r, "2.1.0", meta)
	require.Equal(t, http.StatusOK, adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/releases/"+w1+"/deprecate", "admin", "admin", map[string]any{"successor_release_id": w2}).Code)
	catalog2 := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/catalog", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, catalog2.Code)
	require.Contains(t, catalog2.Body.String(), listing2)       // 可见但不推荐（与 Unlisted 的差异点 1）
	adoptCurrent := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listing2})
	require.Equal(t, http.StatusCreated, adoptCurrent.Code)     // current=2.1.0 未弃用：默认 adopt 成功
	adoptDeprecated := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listing2, "release_id": w1})
	require.Equal(t, http.StatusConflict, adoptDeprecated.Code)
	require.Contains(t, adoptDeprecated.Body.String(), w2)      // 拒绝消息指向 successor（差异点 2）
	variantOnDeprecated := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+decodeAdoptionID(t, adoptCurrent.Body.Bytes())+"/variants", "admin", "admin", map[string]any{"name": "x", "release_id": w1})
	require.Equal(t, http.StatusConflict, variantOnDeprecated.Code) // CreateVariant 显式弃用 release 同拒（差异点 3）
}

// AC3 的运行面：retire 后 available-agents 收窄 + 真实 POST /workbench/executions 409。
func TestLifecycleRetireBlocksNewWorkEndToEnd(t *testing.T) {
	r, _, db := newLifecycleTestApp(t)
	listingID, _ := freezeAndPublishUpgradeRelease(t, r, "1.0.0", lifecycleMetadata())
	_, variantID, localAgentID := adoptAndPublishFirstVariant(t, r, listingID)

	// 退役前：available-agents 含该 agent；真实 start 正常 admit。
	available := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Contains(t, available.Body.String(), localAgentID)
	coordinator := workbenchservice.NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), nil, nil)
	coordinator.SetAgentUseGate(newRealAgentUseGate(db))
	r.POST("/api/v1/workbench/executions", newLifecycleIdentityMiddleware(), session.NewWorkbenchStartHandler(coordinator).Start)
	resp := httptestPostJSON(t, r, "/api/v1/workbench/executions", `{"session_id":"s1","agent_id":"`+localAgentID+`","target_id":"platform","request_id":"req-1","text":"hi","budget_upper":100}`)
	require.Equal(t, http.StatusAccepted, resp.Code, resp.Body.String())

	// 退役后：available-agents 收窄（#59 读模型只查 published）+ 同 agent start 409。
	require.Equal(t, http.StatusOK, adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/retire", "admin", "admin", nil).Code)
	available2 := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.NotContains(t, available2.Body.String(), localAgentID)
	resp2 := httptestPostJSON(t, r, "/api/v1/workbench/executions", `{"session_id":"s1","agent_id":"`+localAgentID+`","target_id":"platform","request_id":"req-2","text":"hi","budget_upper":100}`)
	require.Equal(t, http.StatusConflict, resp2.Code)
	require.Contains(t, resp2.Body.String(), "retired")
}
```

- [ ] **Step 2: 运行三个验收测试确认通过**

Run: `go test ./internal/router/ -run 'TestLifecycleExitDeletesNothing|TestLifecycleUnlistedVersusDeprecated|TestLifecycleRetireBlocksNewWork' -count=1 -v`
Expected: 三个测试 PASS。说明：本任务是验收聚合，实现已在 Task 2-5 各自 RED→GREEN 落地；若此处 FAIL，缺陷回改对应任务的生产文件（不许在本任务打补丁绕过断言）。

- [ ] **Step 3: 全计划验证命令**

Run（worktree 根）：
```bash
go build ./... && go test ./internal/application/repository/ -run 'AgentMarketplaceLifecycle|EndAdoption|TransitionListingState|DeprecateRelease|RetiredVariantAgentExists' -count=1 && go test ./internal/application/service/ -run 'Lifecycle|RetireVariant|EndAdoption|UnlistAndDeprecate|DeprecateSuccessor|UpgradeReconcileSkipsDeprecated|AgentAdoption|AgentUpgrade' -count=1 && go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmission' -count=1 && go test ./internal/router/ -run 'AgentMarketplaceLifecycle|Lifecycle' -count=1 && go test ./internal/database/ -run TestMigrationVersionsUniquePerTrack -count=1
```
Expected: 全部 ok。

- [ ] **Step 4: Commit**

```bash
git add internal/router/routes_agent_marketplace_lifecycle_test.go
git commit -m "test(marketplace): #63 验收证据——退出零删除计数、Unlisted/Deprecated 差异矩阵、退役运行闸端到端"
```

---

## 差异与范围记录（调查摘要 vs 代码现状 + 范围裁决）

1. 调查摘要称 Listing.State 写死于 `agent_marketplace.go:75` / 目录过滤在 `repository/agent_marketplace.go:252`；实测为 service `agent_marketplace.go:79`（`State: "listed"`）与 repository `agent_marketplace.go:270`（`state = 'listed'` 过滤）。以代码为准，结论不变。
2. **「禁止新 Task」的落闸范围**：spec §9 原文「Retire Variant：禁止新 Task」。本计划把闸落在 workbench admission（`POST /api/v1/workbench/executions`，新 Task/Run 的 durable 创建点，#36 语义）+ available-agents 读模型收窄（#59 既有）。`agent-chat` 追问面（`POST /agent-chat/:session_id`）**不在闸内**——追问是既有会话的 follow-up 而非新 Task（CONTEXT.md「任务生命周期」），且 `.Start does not validate AgentID` 是既有架构事实（`command_queue_next.go:94` 注释）；把 agent 归属检查塞进共享 `parseQARequest` 超出本 Issue 收敛要求，记为范围外。
3. **Deprecate 的 successor 必填**：CONTEXT.md 允许「指向替代 Release 或升级建议」。本计划实现「指向替代 Release」分支（successor 必填、同 listing、非自身、未弃用），使指向关系服务端可验证；「或升级建议」分支是前端呈现选择，不属治理面最小闭环。
4. **Unlist 的可逆性**：spec 未定义 relist；CAS 单向（listed→unlisted），重复 unlist 409。公共 Marketplace（#60 域）的 listing 治理与 introduced release 的弃用（`GetRelease` 本地表 only，`agent_marketplace.go:274-284`）不属本计划——introduced release id 调 deprecate 得 404，属平台治理范围。
5. **审计**：AC1 的「审计」按保留性语义验证——marketplace 域的审核记录是 `agent_release_reviews`（Task 6 计数断言承载）；`audit_logs` 无 marketplace 域写入（#59-#62 零先例），故无行可保。四类退出操作不新增审计写入，加审计属独立增强。
6. **AC1「Task 不删除」的承载**：四类退出的生产路径不写不删 workbench 表（Task 1 只加列；Task 5 的闸只在 `CreatePending` 之前咨询，拒绝路径零 durable 写由 Task 5 断言钉死）。AC1 计数矩阵同时纳入 `agent_runs`/`workbench_requests` 两行提供「Task 表行数不变」的直接字面证据；已有 Run 的继续可用由 spec §10「已有本地版本可继续」的 GET /api/v1/agents 断言（AC1 尾段）与运行闸仅拦新 start 的语义共同承载。
7. **B5 OCR 已知项核对**（ask 第 2/4 条）：本 Issue 验收不与 codedelivery 域（gitlab_client.go 缺口）相交；agent 域两个已知项（agent_upgrade.go open 建议 from_release 脱节不关闭、marketplace_lineage UpsertLicense created_by 覆写）均不在本计划修改路径上（本计划不改 FindOrCreateProposal/UpsertLicense），维持原状不处置，由对应后续批次裁决。#51 排除集冻结 TOCTOU 与本计划无消费关系。
8. **本计划零 TS 改动**：retire 后 available-agents 少一行由 #59 既有 contracts wire（`parseAvailableAgentListResponse`）自然表达；lifecycle 治理端点是 Admin 服务端面，移动端无消费需求（#65 信任信号消费时再补）。

## 自我审查记录（writing-plans 四项检查）

1. **Spec coverage**：AC1→Task 6 计数测试 + Task 1 迁移只加列；AC2→Task 6 差异矩阵（差异点 1/2/3）+ Task 3 行为闸；AC3→Task 4/6 全部走真实迁移流 + 真实 services + httptest 真实 HTTP；spec §9 两层退出→Task 3 Retire/End；§10 状态表→Task 3 闸 + Task 6 矩阵；§13 权限→Task 4 Admin+ full-access 地板。无缺口。
2. **Step scan**：每步一个可核对动作；无 TBD/「适当校验」类空话；service/handler 体由签名+测试决定，仅 CAS/事务/gate 插入点给出代码块（算法非签名可定）。
3. **Type consistency**：`RetiredVariantAgentExists`（Task 2 定义、Task 5/6 消费）、`ErrAgentUseDenied`（Task 5 定义、Task 6 断言 `"retired"` 消息子串）、`AgentMarketplaceLifecycleService` 四方法在 interfaces/服务/handler/路由四处签名逐字一致；构造器第二参数为 `repository.AgentMarketplaceRepository`（Task 2 接口声明与 `TransitionListingState`/`DeprecateRelease` 所在接口一致，container.go:321 已 Provide）；view 构造走 lifecycle 自有 `variantViewOf`/`adoptionViewOf`（复用包级 releaseManifest/missingCapabilities）；实体新列名迁移 DDL 与 gorm tag 逐字一致。审查修复轮（F1-F4）：F1 构造器参数类型改 repository 接口、F2 补 view 构造裁定、F3 upgrade handler 行号引用改 handler/agent_upgrade.go:71-87、F4 AC1 矩阵补 agent_runs/workbench_requests 两行并加差异记录第 6 条裁定。
4. **Review Focus**：五条各有归属测试（1→Task 3 Step1 第一用例；2→Task 2 Step1 第一用例；3→Task 3 Step1 第四用例；4→Task 5 Step1；5→Task 4 Step1 跨租户四断言 + Task 2 各用例尾段）。
5. **Proportion**：计划长度与六任务交付面相称；测试代码为验收主体（AC3 要求），生产代码步以签名+插入点表达。
