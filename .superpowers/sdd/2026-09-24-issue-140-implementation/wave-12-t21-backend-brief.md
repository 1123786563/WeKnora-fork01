# Wave 12 — T21 后端子任务：搜索与生成的额度预估及阻断（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端；Web 子任务等你的合同 reviewed+integrated 后另行派发。T21 无专用计划——本简报即权威任务书。**本波唯一的 career 后端任务**（office.go/handler.go/routes_career.go/career_migration_test.go 独占）。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t21-usage/WeKnora-fork01 --detach 14b81d24c
cd /Users/wuyongjun/.codex/worktrees/issue-140-t21-usage/WeKnora-fork01
```
BASE = `14b81d24c`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 21 文本（verbatim）

### Task 21: T21/#161 搜索与生成的额度预估及阻断
**Depends:** #154（T13 verified）、#153（T15 verified）。**Produces:** `CareerRemote.act({kind: "search_once", payload}, ...)` 的 admission 语义。
**验收：** **执行前展示将消耗的额度与触发条件**；**超额阻止新的收费 Run 而非删除既有档案或申请**；**重复请求不会重复预占或收费**；**付费状态不改变岗位排序或资格判定**
**原始证据：** admission 合同测试额度边界、重复请求和只读访问。**失败处理：** **预估不可得时不得先执行后补报，显示可理解的不可用原因。**
（注：主计划 `internal/modules/career/service/` 为旧稿路径，实际在 `internal/modules/career/` 包根——沿用先例。）

## 3. 设计约定（遵循既有先例，结构由你实现并在报告冻结）

**Files**：create `internal/modules/career/usage.go`、`usage_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`（聚焦）；modify `internal/router/routes_career.go` + 聚焦 route tests；如需持久表 create `migrations/versioned/000208_career_usage.up/.down.sql`、`migrations/sqlite/000129_career_usage.up/.down.sql`（预占/消耗记录建议建表）；extend `internal/database/career_migration_test.go`（如建表，**并纳入 career_export.go 的 purge 清单与 boundary 视图——T19/T20 先例，本任务所有权含此两处**）；更新 architectureguard Career 清单。

**迁移编号（本轮唯一分配，如建表）**：versioned **000208** / sqlite **000129**（调度员实核空闲：目录现有最大 207/128）。

**Consumes/升级**：T11 的 `SearchQuotaGate` 注入 seam（生产此前为诚实空实现）——本任务把它升级为**真实 admission 机制**（预占/消耗/余额，确定性成本常量或按操作类型冻结成本表）；T13 规则触发路径同受 admission；materials/evaluations 只读访问零额度（免费语义冻结）。

**行为要点**：
1. **执行前预估**：任何收费 Run（search_once/规则触发/材料生成类收费操作——范围冻结于报告）执行前返回"将消耗额度+触发条件"（预估端点或 intent 响应内嵌，冻结形态）；**预估不可得 → typed 不可用原因，绝不先执行后补报**。
2. **超额阻断**：余额不足 → 阻止**新的收费 Run**（typed 拒绝，可恢复）；**既有档案/申请/评估永远可读**（只读零额度）。
3. **不重复预占/收费**：预占与消耗都以 request ID 幂等（replay 不二次扣）；预占-执行-结算三段可对账。
4. **公平性不变**：付费状态（余额/套餐）**不改变岗位排序或资格判定**（评估与排序输入零付费维度——测试断言）。
5. **house 语义**：request ID replay/conflict、expectedRevision、scope、未知恢复。

**命名 RED 测试**：
```go
func TestAdmissionShowsCostAndConditionsBeforeExecution(t *testing.T)
func TestAdmissionEstimateUnavailableNeverExecutesFirst(t *testing.T)
func TestOverQuotaBlocksNewChargedRunsOnly(t *testing.T)
func TestOverQuotaKeepsExistingRecordsAndApplicationsReadable(t *testing.T)
func TestDuplicateRequestDoesNotDoubleReserveOrCharge(t *testing.T)
func TestPaymentStatusDoesNotAlterRankingOrQualification(t *testing.T)
func TestReadOnlyAccessConsumesNoQuota(t *testing.T)
func TestAdmissionScopeRejectsOtherTenantAndOwner(t *testing.T)
func TestUsageMigrationUpAndDown(t *testing.T)                 // 如建表
func TestUsageTableIncludedInDeletionPurgeAndBoundary(t *testing.T) // 如建表
```

## 4. 验证（全跑附原始输出）

```bash
gofmt -w internal/modules/career internal/router
go test ./internal/modules/career/... -count=1
go test ./internal/database/... -count=1
go test ./internal/router/... -count=1
go test ./tools/architectureguard/... -count=1
git diff --check
```
architectureguard 基线以清单当前实值为准（T20 后 692 路由），新路由按精确值更新。

## 5. 全局约束（verbatim 摘录，违反即 Spec FAIL）

- 额度耗尽仍可读取既有档案和申请；搜索规则由用户显式启停并经预算准入。
- 只在 assigned isolated Worktree 内改本任务所有权文件；local commits authorized, no push/merge/deploy/GitHub action。
- 所有写入使用 request ID 与 expected revision；scope 从认证上下文派生；数据库查询一律参数绑定。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 6. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t21-usage/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD、成本模型冻结（哪些操作收费/成本常量/预占语义）、RED/GREEN 证据、接口冻结说明、已知局限。COMMIT：`feat(career): enforce quota admission with honest estimates`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成。
