# Issue #63（T33 Marketplace 生命周期）收口实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 #63（T33）从「实现已散落合入 main」推进到台账 verified 判词与 closure-ready：完成 T33-1 悬置的独立源级复审、T33-1..4 逐项验收审计、独立全范围审查、OCR 尝试，并按发现修复缺口。

**Architecture:** T33 的四个验收波（T33-1 生命周期 CAS 仓储 / T33-2 退役·下架·弃用服务语义 / T33-3 退役只阻新任务 / T33-4 真实 HTTP 全链）经 round5（`8500ea74e`）已全部或大部落在 main，但台账 `2026-09-30-issue63-t33-ledger.md` 停在「T33-1 待独立复审、下游 pending」。本轮是验证收口轮：以 main 为事实源逐项判定，补齐审查链，仅对发现的 Critical/Important 缺口写码。

**Tech Stack:** Go + GORM + Gin；SQLite 测试库；PostgreSQL 竞态证据沿用 `openRunTestDB` harness（DSN 本地 `WeKnora-postgres-dev`，不落盘）；OCR 用 `ocr` CLI（若配额仍 429 则如实记录受阻）。

## Global Constraints

- 审计/复审任务只读，不改生产码、不动 HEAD。
- 修复任务只动审计/复审点名的文件；TDD（RED→GREEN 证据入报告）。
- 判定依据优先级：T33 ledger 验收标准 > plan-t33.md > B6 ledger > 代码实况；GitHub 状态与本地判词分离，不 push、不动 issue。
- 测试隔离：每测试 `t.TempDir()` SQLite；PG 竞态测试用随机 per-test schema；不共享端口；不 Docker。
- 每任务产出到 `.superpowers/sdd/2026-10-01-issue63-t33-closure/`（brief/report/review 命名同前轮约定），完成即记账 progress.md。

---

### Task 1: T33-1..4 逐项验收审计（只读）

**Files:**
- Create: `.superpowers/sdd/2026-10-01-issue63-t33-closure/task-1-report.md`（唯一产出）

**Interfaces:** 输入=验收标准源（下表）；输出=逐项判定表 + 缺口清单（每条带 file:line 与最小修正建议 + 严重级）。

| 波 | 验收标准（源自 t33-ledger） | main 上的主要落点（预核） |
|---|---|---|
| T33-1 | Variant/Adoption/Listing/Release 弃用的持久 CAS 元数据/审计；不可变 Release bundle 保持；End=gate→后查子项→单事务 CAS | repository/agent_marketplace_lifecycle.go、agent_adoption.go、agent_upgrade.go；`TestAgentAdoptionEndWaitsThenSeesCommittedVariant`（含 /postgres 子测） |
| T33-2 | retire/end/unlist/deprecate 转换；守护 Variant 创建/采纳/升级提案；unlisted≠deprecated 语义 | service/agent_marketplace_lifecycle.go:70,82；`TestUnlistAndDeprecateBehaviorDiffers`、`TestAdoptListingSerializesWithConcurrentUnlist`、`TestConcurrentReciprocalDeprecationsCannotPersistSuccessorCycle` |
| T33-3 | 退役 Variant 只阻新 Task 创建；既有 Task 与普通 Agent 不受影响 | workbench/admission.go `checkAgentUse`；`admission_agent_use_test.go`（含 retire-replay） |
| T33-4 | AC1–AC3 经认证 HTTP 生产装配全链可观测 | router/routes_agent_marketplace_lifecycle_test.go 六测试（含 `TestLifecycleRetireBlocksNewWorkEndToEnd`） |

- [ ] Step 1: 逐波读标准与落点，跑四组聚焦测试并记录输出：`go test ./internal/application/repository/ -run 'TestAgentAdoption|TestDeprecateRelease|TestAdoptListing|TestConcurrentReciprocal' -count=1`；`go test ./internal/application/service/ -run 'TestUnlist|TestDeprecate|TestUpgradeReconcile|TestAcceptUpgrade' -count=1`；`go test ./internal/workbench/service/workbench/ -run TestAdmissionAgentUse -count=1`（以实际测试名 grep 为准）；`go test ./internal/router/ -run TestLifecycle -count=1`。
- [ ] Step 2: PG 竞态复跑（本地 `WeKnora-postgres-dev`，DSN 只进环境变量）：`TRPC_TEST_POSTGRES_DSN='postgres://postgres:<本地开发密码>@127.0.0.1:5432/WeKnora?sslmode=disable' go test -tags semantic_integration ./internal/application/repository -run '^TestAgentAdoptionEndWaitsThenSeesCommittedVariant$' -count=1 -v`（密码向用户索取或用 `~/.weknora-func-creds.env` 邻域既有本地开发口令；拿不到则记 blocked-env）。
- [ ] Step 3: 产出判定表（每波 verified / gap+严重级）与缺口清单，写入 task-1-report.md。

### Task 2: T33-1 独立源级复审（只读）

**Files:**
- Create: `.superpowers/sdd/2026-10-01-issue63-t33-closure/task-2-review.md`

**Interfaces:** 复审对象=main 上 T33-1 源文件集（repository/agent_marketplace_lifecycle.go、agent_adoption.go、agent_upgrade.go 及其测试）；复审基准=R2 修复计划的三不变量（End 先取 adoption 父门→后置语句查子 Variant→单事务 CAS；并发 CreateVariant 提交后 End 必拒绝；PG READ COMMITTED 下无快照逃逸）+ E1 修复（PG 测试选中 postgres 子测、锁观察限定 End 后端 PID）。

- [ ] Step 1: 派发独立复审（最强模型）：逐不变量对源码+测试给出 PASS/FAIL 与证据行号； FAIL 项标 Critical/Important/Minor。
- [ ] Step 2: 结论写 task-2-review.md 并给出「T33-1 可否关闭」判词。

### Task 3: 缺口修复（条件执行）

**触发条件：** Task 1/2 缺口清单中存在 Critical/Important；无则本任务跳过并在 progress.md 记「无缺口，跳过」。

**Files:** 仅审计/复审点名的文件 + 对应测试。

- [ ] Step 1: 按缺口清单逐条 TDD 修复（RED→GREEN 证据入报告）；每条独立提交 `fix(marketplace): <缺口摘要> (T33 #63 Task 3)`。
- [ ] Step 2: 复跑 Task 1 的四组聚焦测试 + `go build ./...` + `git diff --check`。
- [ ] Step 3: 修复报告追加 task-3-report.md；触发复审（同 Task 2 基准，仅修复范围）。

### Task 4: OCR 尝试 + 台账 verified 判词 + closure-ready 报告

**Files:**
- Modify: `docs/plans/issue30-sweep/plans/2026-09-30-issue63-t33-ledger.md`（追加终局段）
- Create: `.superpowers/sdd/2026-10-01-issue63-t33-closure/final-report.md`

- [ ] Step 1: `ocr --help` 确认 CLI 形态后，对 T33 相关范围（`internal/application/{repository,service}` 生命周期文件 + router 生命周期测试）跑一次审查；配额 429/1308 则如实记录「OCR 受阻于 provider 配额」并引用历史同类记录（issue-72/craft 先例）。
- [ ] Step 2: 台账追加终局段：T33-1..4 逐波判词（引 Task 1/2 报告）、OCR 状态、#63 closure-ready 结论与 follow-up 清单。
- [ ] Step 3: 提交 `docs(ledger): T33 收口——审计/复审/OCR 判词（#63 closure-ready）`；final-report.md 汇总全链证据索引。

## Self-Review

- 规格覆盖：ledger 关闭标准三件（router acceptance→Task 1 Step 1 第四组、independent full-range review→Task 2、complete OCR→Task 4 Step 1）+ T33-1 悬置复审→Task 2；缺口修复→Task 3 条件任务。
- 占位扫描：Task 3 为条件任务，内容由 Task 1/2 产出物定义（非占位——触发条件与产出契约明确）。
- 类型一致性：测试名/文件名均预核自 main 实况。
