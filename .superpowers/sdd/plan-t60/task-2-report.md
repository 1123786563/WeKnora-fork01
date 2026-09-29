# Task 2 报告：Public Marketplace 持久化层（六张新表 + 三条 FK 放宽）

状态：DONE（commit `34565aa41`，7 files changed, 472 insertions）

## 实现内容

前置接口核验（占用前实测）：
- `ls migrations/versioned/` 最高为 `000192_agent_adoption_variants.*`（Task 1 产物），000193 可占用 ✓
- `ls migrations/sqlite/` 最高为 `000113_agent_adoption_variants.*`，000114 可占用 ✓
- `migrations/versioned/000188_tenant_agent_marketplace.up.sql` 中 `agent_releases` / `agent_marketplace_listings` 主键均为 `(id, tenant_id)`，brief 中复合 FK 引用有效 ✓
- `migrations/sqlite/000113_agent_adoption_variants.up.sql` 的 DDL 与 brief 中 000114 重建表（去 FK 版）逐列一致 ✓

按 brief 六步逐字实现：

1. **Step 2/3 迁移四件套**（逐字取自 brief）：
   - `migrations/versioned/000193_public_agent_marketplace.up.sql`：六张新表 + 7 个索引 + `fk_public_marketplace_current_release` 约束 + `DO $$` 块按约束定义模糊匹配（`pg_get_constraintdef ILIKE '%REFERENCES ...%'`）动态删除三条 #59 FK（不依赖自动生成的 FK 名）。
   - `migrations/versioned/000193_public_agent_marketplace.down.sql`：按 `fk_agent_adoptions_listing` / `fk_agent_adoptions_release` / `fk_agent_adoption_variants_release` 重建 FK 后逆序 DROP 六表。
   - `migrations/sqlite/000114_public_agent_marketplace.up.sql`：自持事务（`PRAGMA foreign_keys=OFF` → `BEGIN IMMEDIATE` → … → `COMMIT` → `PRAGMA foreign_keys=ON`），#59 两表按 000113 DDL 去 FK 重建（rebuild → INSERT SELECT → DROP → RENAME → 补索引）。
   - `migrations/sqlite/000114_public_agent_marketplace.down.sql`：同构 restore，恢复 000113 原始 FK。
2. **Step 4 生产入口 NoTxWrap 三段式**（`internal/database/migration.go`，镜像 v55 先例）：
   - `:28-33` 新增常量 `sqliteAdoptionFKRelaxationMigrationVersion = 114`（`migration.go:33`）；
   - `:117-125` presence 检查追加 `os.Stat("migrations/sqlite/000114_public_agent_marketplace.up.sql")`（`migration.go:121`）；
   - `:229-255` 在 v55 三段式块之后、`// Run all pending migrations` 之前插入 000114 三段式：`Migrate(113)` 补齐 → Close → `newSQLiteMigrator(..., noTxWrap=true)` → `Steps(1)` → Close → 恢复 per-file 包装 migrator。
3. **Step 5 实体** `internal/types/public_marketplace_persistence.go`：六个 GORM 实体（`VerifiedPublisherEntity` / `PublicMarketplaceListingEntity` / `PublicReleaseSubmissionEntity` / `PublicReleaseReviewEntity` / `PublicAgentReleaseEntity` / `TenantIntroducedReleaseEntity`），TableName 与六表一一对应，逐字取自 brief（仅一处 gofmt 对齐修正，见自检）。

## TDD 证据

**RED** — 先只改测试清单（`internal/database/migration_sqlite_versioned_schema_test.go:47-52` 追加六表名）再实现迁移：

```
$ go test ./internal/database/ -run 'TestSQLiteMigrationsCreateVersionedSchema' -count=1
INFO [...] | Database migrated from version 0 to 113
--- FAIL: TestSQLiteMigrationsCreateVersionedSchema (1.21s)
    Messages: SQLite migrations must create table public_marketplace_verified_publishers
FAIL
```

失败原因即预期：六张表尚未创建。

**GREEN** — 四件套 + migration.go 三段式 + 实体就位后：

```
$ go test ./internal/database/ -count=1
ok  github.com/Tencent/WeKnora/internal/database  23.843s
```

`-v` 明细（同一提交内容的第二次运行取证）：

```
--- PASS: TestSQLiteMigrationsIncludeAutoTagConfig (3.90s)
--- PASS: TestSQLiteMigrationsCreateVersionedSchema (3.03s)      ← 六张新表清单 + 生产入口 up 跨 114 执行三段式
--- PASS: TestSQLiteMigrationsUpgradeV4PreservesData (2.13s)     ← legacy v4 fixture 经 presence 检查自动跳过 000114 块
--- PASS: TestSemanticMigrationSQLiteUpDownUp (3.98s)            ← 000193/000114 down 迁移真实可逆（up/down/up 回放）
--- PASS: TestWorkbenchSQLiteMigrationPreservesRunChildren (5.45s)
--- PASS: TestWorkbenchSQLiteURLAndLaterMigrationTransaction (4.98s)
--- PASS: TestWorkbenchSQLiteV16FailureRollsBackAndRecovers (2.32s)
--- SKIP: TestWorkbenchSQLiteURLUpgradeAndDownUpPreserveChildren (0.00s)  ← 既有 blocked-dependency（W04 000017 不在此隔离基线，workbench_migration_test.go:137），非本次引入
--- PASS: TestWorkbenchSQLiteURLPreservesMigrationTableQuery (2.83s)
--- PASS: TestWorkbenchSQLiteInteractionActionCheck (2.49s)
--- PASS: TestExecutionTargetSQLiteFullMigrationDownUp (8.88s)
--- PASS: TestWorkbenchSQLiteDownRefusesPaseo (2.83s)
PASS
ok  github.com/Tencent/WeKnora/internal/database  44.270s
```

额外检查（同一提交上运行）：

```
$ go build ./...            # exit 0（仅 ld "ignoring duplicate libraries: -lc++" 链接警告，非错误）
$ go vet ./internal/database/ ./internal/types/   # exit 0，无输出
$ go test ./internal/types/ -count=1              # ok  4.751s
```

## 文件变更（commit 34565aa41）

- 新增 `migrations/versioned/000193_public_agent_marketplace.up.sql` / `.down.sql`
- 新增 `migrations/sqlite/000114_public_agent_marketplace.up.sql` / `.down.sql`
- 新增 `internal/types/public_marketplace_persistence.go`
- 修改 `internal/database/migration.go`（+37 行：常量、presence 检查、000114 三段式）
- 修改 `internal/database/migration_sqlite_versioned_schema_test.go`（+6 行：六表清单）

## 自检发现

1. **gofmt 修正**：brief Step 5 的 `PublicAgentReleaseEntity` 结构体 tag 列对齐未过 gofmt，`gofmt -w` 做了对齐修正（不影响语义；实体字段/类型/tag 与 brief 逐字一致）。提交前 `gofmt -l` 对本任务三个 Go 文件全部合规。
2. **`.gitignore:96` 忽略 `migrations/`**：553 个既有迁移文件均在 git 追踪中（ignore 不影响已追踪文件），但**新**迁移文件会被该规则静默忽略；brief Step 7 的 `git add` 未带 `-f`，直接执行会静默漏掉 4 个迁移 SQL。已用 `git add -f` 按brief 意图完整入库。**后续 Task 3-8 若需新建 migrations/ 下文件，同样需要 `-f`。**
3. **如实声明（brief 已预声明，此处确认）**：Go 测试不会直接暴露 NoTxWrap 修复的必要性（测试库为空库且 DSN 未带 `_foreign_keys=on`）；其正确性依据是 `migration.go:200-204` 既有注释的同一结论与 v55 先例。生产 down 路径（`scripts/migrate.sh` 裸 migrate CLI）与既有 000055 down 同样受限，brief 已声明不在本计划内解决。
4. types 目录另有 `agent_adoption_persistence.go`、`context_clone.go` 两个既有文件 gofmt 不合规——仓库既有状态，不在本任务授权范围，未改动。

## 疑虑

无阻塞性疑虑。提交 `34565aa41` 未推送远端。

---

# 修复轮 1 报告：versioned 000193 down 循环 FK 阻塞（review finding, important）

## 修复内容

`migrations/versioned/000193_public_agent_marketplace.down.sql` 开头（重建 #59 FK 之前）补一行，并更新文件头注释说明原因：

```sql
ALTER TABLE public_marketplace_listings DROP CONSTRAINT IF EXISTS fk_public_marketplace_current_release;
```

原因：up 孪生（000193 up:107-108）为 `public_marketplace_listings.current_release_id` 加了指向 `public_agent_releases(id)` 的循环 FK；down 的第 8 行 `DROP TABLE public_agent_releases` 被该 FK 阻塞。先删该约束后六表可按逆序干净删除。`IF EXISTS` 保证对未达 000193 状态幂等无害。

## TDD 证据（本机一次性 docker postgres:17-alpine 实证，容器已删）

桩表用真实 `000187_agent_versions.up.sql`（PK `(id, tenant_id)`），其余全部真实迁移文件；回放协议 = 桩 000187 → up 体（000188→000192→000193）→ 000193 down → 000193 up（等价 migrate `Up → Steps(-1) → Up`）。

**RED（修复前）** —— 执行未修复的 down：

```
$ docker exec -i t60-pg-verify psql -U postgres -v ON_ERROR_STOP=1 -q < 000193_public_agent_marketplace.down.sql
ERROR:  cannot drop table public_agent_releases because other objects depend on it
DETAIL:  constraint fk_public_marketplace_current_release on table public_marketplace_listings depends on table public_agent_releases
HINT:  Use DROP ... CASCADE to drop the dependent objects too.
EXIT=3
```

与审查发现逐字一致（本次为本人自行复现，非转述）。

**GREEN（修复后）** —— 重置后完整回放：

```
== UP(1) to 000193 OK ==
== DOWN(000193) OK ==
== UP(2) 000193 OK — UP/DOWN/UP GREEN ==
```

**DOWN 后状态断言**（第二次 DOWN 后查询）：

```
           tbl           |                      conname
-------------------------+----------------------------------------------------
 agent_adoptions         | fk_agent_adoptions_listing
 agent_adoptions         | fk_agent_adoptions_release
 agent_adoption_variants | agent_adoption_variants_adoption_id_tenant_id_fkey   ← 000192 inline，始终存在
 agent_adoption_variants | fk_agent_adoption_variants_release
-------------------------
 COUNT(public/tenant_introduced 六表) = 0
```

3 条被放宽的 #59 FK（listing / release / variants_release）在 down 后全部恢复 ✓，六张新表全部删除 ✓。

**sqlite 套件回归**（修复只动 versioned down，不影响 sqlite 流；按 brief Step 6 清单回归）：

```
$ go test ./internal/database/ -count=1
ok  github.com/Tencent/WeKnora/internal/database  42.591s
$ go test ./internal/database/ -count=1 -run 'TestSemanticMigrationSQLiteUpDownUp' -v
--- PASS: TestSemanticMigrationSQLiteUpDownUp (4.38s)
```

## 未运行项（如实声明）

- tagged PG 集成测试 `internal/database/semantic_migration_pg_test.go`（build tag `semantic_integration`）未运行：本环境 `TRPC_TEST_POSTGRES_DSN` 未设置。另发现该测试在当前基线上存在**既有滞后**：其对 versioned 头部的硬断言 `require.Equal(t, uint(180), version)`（:66、:101）已落后于现头部 000193，即使有 DSN 也会先挂在版本断言——它能抓住本缺陷（`m.Steps(-1)` 全量 down），但需先更新断言，超出本修复轮授权（该测试文件不在 Task 2 授权文件清单内）。本修复的实证依据为上述 docker PG17 真实回放。

## 修复轮文件变更

- `migrations/versioned/000193_public_agent_marketplace.down.sql`（+4/-1：1 行 DROP CONSTRAINT + 注释更新）
