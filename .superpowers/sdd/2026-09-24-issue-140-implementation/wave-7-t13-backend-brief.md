# Wave 7 — T13 后端子任务：可控的持续找岗规则（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端；Web 子任务（search_rule 页面）等你的合同 reviewed+integrated 后另行派发。T13 无专用计划——本简报即权威任务书。**本波唯一的 career 后端任务**（office.go/handler.go/routes_career.go/career_migration_test.go 独占）。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t13-search-rule/WeKnora-fork01 --detach 067aa1d0e
cd /Users/wuyongjun/.codex/worktrees/issue-140-t13-search-rule/WeKnora-fork01
```
BASE = `067aa1d0e`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 13 文本（verbatim）

### Task 13: T13/#154 可控的持续找岗规则
**Depends:** #152（T11 verified）。**Produces:** `CareerRemote.act({kind: "set_rule", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。
**验收：** **未开启不后台运行**，暂停后下一次触发被取消或不再入队；**启用前展示条件、频率与预计消耗**；同一新岗位只产生一个发现待办；重复触发和结果未知经同一请求身份对账
**原始证据：** Career Office 定时触发测试覆盖开关、重复与未知状态。**失败处理：** **预算不足或来源不完整形成可见状态，不静默跳过。**
（注：主计划 `internal/modules/career/service/` 为旧稿路径，实际在 `internal/modules/career/` 包根——沿用先例。）

## 3. 设计约定（遵循既有先例，结构由你实现并在报告冻结）

**Files**：create `internal/modules/career/search_rule.go`、`search_rule_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`（聚焦）；modify `internal/router/routes_career.go` + 聚焦 route tests；create `migrations/versioned/000203_career_search_rules.up/.down.sql`、`migrations/sqlite/000124_career_search_rules.up/.down.sql`；extend `internal/database/career_migration_test.go`；更新 architectureguard Career 清单。

**迁移编号（本轮唯一分配）**：versioned **000203** / sqlite **000124**（调度员实核空闲：目录现有最大 202/123）。

**Consumes**：T11 searches 基础设施（SourcePolicy/SourceTransport seam、SearchQuotaGate 注入 seam——`internal/modules/career/search_once.go`）；T10 评估 seam（发现岗位的资格状态）。**真实额度仍属 T21**：沿用 T11 的注入式 gate，生产默认实现不得虚构额度状态；"预计消耗"按规则参数的确定性估算（如 触发频率 × 单次搜索成本常数），如实标注估算口径。

**行为要点**：
1. **规则生命周期**：set_rule 创建/更新规则（条件、频率、启停）；启用/暂停/关闭为显式状态；**未开启不后台运行**——disabled/paused 规则零触发、零入队。
2. **触发机制（受约束设计）**：规则触发用**注入时钟 + 显式触发 seam**（如 `TriggerDueRules(ctx, now)`），不启动后台 goroutine/定时器做生产路径（服务器无常驻调度进程的实现先例为零，报告披露该取舍；真实调度若需常驻由主控在 T33 前另行裁决）。暂停后"下一次触发被取消或不再入队"= due 评估时跳过 paused 并将到期的下次运行计划顺延/标记取消（语义二选一并冻结，报告说明）。
3. **发现待办去重**：同一新岗位（按 opportunity 身份）只产生一个发现待办（durable、幂等键）；重复触发同岗位不重复建待办。
4. **对账**：规则触发的搜索执行沿用 T11 幂等语义（trigger 自带确定性 request ID，如 ruleID+周期序号——同周期重复触发同 ID 对账，不复制）。
5. **可见状态**：预算不足（quota gate typed 拒绝）或来源不完整（空 allowlist）→ 规则执行记录落**可见状态**（如 blocked_no_quota / no_vetted_sources），不静默跳过；执行历史可查。
6. **house 语义**：set_rule 的 request ID replay/conflict、expectedRevision、tenant/owner scope。

**命名 RED 测试**：
```go
func TestSetRuleStoresRuleAndReplayIsIdempotent(t *testing.T)
func TestDisabledRuleNeverTriggersOrEnqueues(t *testing.T)
func TestPausedRuleCancelsOrDefersNextTrigger(t *testing.T)
func TestRuleEnableSurfacesConditionsFrequencyAndEstimatedCost(t *testing.T)
func TestRuleTriggerSameNewJobProducesSingleDiscoveryTodo(t *testing.T)
func TestRuleTriggerDuplicateReconcilesBySameRequestId(t *testing.T)
func TestRuleBudgetInsufficientFormsVisibleStatusNotSilentSkip(t *testing.T)
func TestRuleIncompleteSourcesFormVisibleStatus(t *testing.T)
func TestRuleScopeRejectsOtherTenantAndOwner(t *testing.T)
func TestSearchRuleMigrationUpAndDown(t *testing.T)
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
architectureguard：当前基线 **606/675**（T16 后），你的新路由按精确新值更新。

## 5. 全局约束（verbatim 摘录，违反即 Spec FAIL）

- 搜索规则由用户显式启停并经预算准入；额度耗尽仍可读取既有档案和申请。
- 只在 assigned isolated Worktree 内改本任务所有权文件；local commits authorized, no push/merge/deploy/GitHub action。
- 所有写入使用 request ID 与 expected revision；同一 request ID 内容变化拒绝；scope 从认证上下文派生；数据库查询一律参数绑定。
- 外部网络 I/O 只经既有 SourceTransport（SSRF 防护沿用），且不得在 DB 事务内。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 6. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t13-search-rule/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD、触发机制取舍披露、RED/GREEN 证据、接口冻结说明、已知局限。COMMIT：`feat(career): run user-controlled recurring search rules`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成。
