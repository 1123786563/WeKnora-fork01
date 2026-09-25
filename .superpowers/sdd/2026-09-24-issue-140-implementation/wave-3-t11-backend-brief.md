# Wave 3 — T11 后端子任务：C 对话入口的一次性真实找岗（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端；Web 子任务（search_once 页面）等你的合同 reviewed+integrated 后另行派发，**不写任何 Web 文件**。T11 无专用计划——本简报即你的权威任务书，实现约定遵循已集成的 T08/T09/T10/T14 后端先例。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t11-search-once/WeKnora-fork01 --detach d88cd513a
cd /Users/wuyongjun/.codex/worktrees/issue-140-t11-search-once/WeKnora-fork01
```
BASE = `d88cd513a`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 11 文本（verbatim）

### Task 11: T11/#152 C 对话入口的一次性真实找岗
**Depends:** #149、#150。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Produces:** `CareerRemote.act({kind: "search_once", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。
**验收：** 找岗默认一次性 Task，指令不自动创建持续规则；首批来源明确实际可用方式与城市覆盖，不宣称全国完整；结果列出检查时间、资格状态、原始链接及不确定性；执行失败、未知回执和额度拒绝均可恢复，不复制搜索 Task
**原始证据：** 固定来源与真实允许来源各跑一次合同场景。**失败处理：** 外部来源不可用时说明范围与失败，不以演示数据替代。
（Step 1-4 框架：先 RED 服务端公共 seam 测试覆盖验收+request ID+revision+Tenant 隔离 → 最小实现封闭 intent → 集成验证。注：主计划写的 `internal/modules/career/service/` 路径与现行布局不符，实际在 `internal/modules/career/` 包根——沿用 T09/T14 先例。）

## 3. Step 0（先于主任务，单独 commit）：偿还 T09 Wave1 遗留 medium

集成台账明确披露"至今未偿还"：`internal/modules/career/source_import_test.go` 缺 **not_found / blocked / unsupported_content / empty_content / response_too_large** 五个冻结 failure 分类的断言覆盖（集成台账原文："补断言即可"）。为五分类各补最小断言（可并入既有分类测试函数或新增 focused 测试），先确认当前无覆盖（RED 或实核既有覆盖后在报告说明），GREEN 后单独 commit `test(career): cover frozen source failure classifications`。验证：`go test ./internal/modules/career -run 'TestImportURL' -count=1`。

## 4. 主任务设计约定（遵循既有先例，冻结结构由你实现并测试固定）

**Files**：create `internal/modules/career/search_once.go`、`search_once_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`（聚焦）；modify `internal/router/routes_career.go` + 聚焦 route tests；create `migrations/versioned/000199_career_searches.up/.down.sql`、`migrations/sqlite/000120_career_searches.up/.down.sql`；extend `internal/database/career_migration_test.go`；更新 architectureguard Career 清单。

**迁移编号（本轮唯一分配）**：versioned **000199** / sqlite **000120**（调度员实核空闲：目录现有最大 198/119）。

**HTTP（沿用 opportunity/evaluation/application 三件套约定）**：
- `POST /api/v1/career/searches` → `Office.SearchOnce`（body：requestId、指令/payload、expectedRevision）
- `GET /api/v1/career/searches/receipt?requestId=...` → 收据查询
- `GET /api/v1/career/searches/:searchId` → 结果读取
- reconcile 路由仅在确需异步对账时增设（若同步一次执行可完成则不设，报告说明取舍）。

**行为要点**：
1. **一次性**：每个 request ID 恰好一个 durable search（含结果集）；exact replay 返回原收据；同 ID 变更 payload 为 typed conflict；**绝不创建任何持续规则**（那是 T13 set_rule 的封闭 intent，本任务不实现其任何触发/入队结构）。
2. **来源诚实**：只经 `Office` 已有 `SourcePolicy`/`SourceTransport` seam（source_import.go:96/141，生产 allowlist 为空 = 无已核验来源）。搜索结果只含真实抓取到的岗位；来源不可用/无已核验来源时，返回显式范围说明与失败码，**不以演示/伪造数据替代**。响应携带"首批来源实际可用方式与城市覆盖"的如实清单（当前生产为空清单），不出现"全国"表述。
3. **结果行**：每个发现的岗位记录 检查时间、资格状态（消费 T10 评估 seam：未评估=needs_review，不得虚构符合/不符合）、原始链接、不确定性标注（如 partial/低置信）。发现结果落库为不可变快照明细，不修改既有 opportunity 语义。
4. **可恢复失败**：执行失败 / 结果未知（外部调用不确定）/ 额度拒绝 三类都是 typed error + 原 request ID 可重放恢复；额度经**注入式窄 seam**（如 `SearchQuotaGate`），生产默认实现必须诚实（不虚构额度状态——真实额度是 T21 的票，报告注明该边界），测试用 fake 注入断言 typed 拒绝可恢复。
5. **事务边界**：外部网络 I/O 在 DB 事务外；claim scoped request → fetch → reconcile → 原子落 search+results+terminal receipt（T09 模式）。
6. **scope**：一切查询带 authenticated tenant/owner；跨 User/Tenant 拒绝且不泄漏存在性；expected revision 冲突返回当前 revision。

**命名 RED 测试（覆盖主计划验收 + request ID/revision/Tenant）**：
```go
func TestSearchOnceCreatesSingleDurableSearchPerRequest(t *testing.T)          // 一次性+replay+变更冲突+不建规则（断言无 rule 类结构被创建）
func TestSearchOnceResultsCarryCheckTimeQualificationLinkAndUncertainty(t *testing.T)
func TestSearchOnceDoesNotFabricateWhenSourceUnavailable(t *testing.T)         // 来源不可用→范围说明+失败码，零伪造行
func TestSearchOnceCoverageListingIsTruthfulWithoutNationalClaim(t *testing.T) // 覆盖清单=实际来源（生产空），无全国表述
func TestSearchOnceFailureUnknownAndQuotaRefusalAreRecoverable(t *testing.T)
func TestSearchOnceScopeRejectsOtherTenantAndOwner(t *testing.T)
func TestSearchOnceRevisionConflictReturnsCurrentRevision(t *testing.T)
func TestSearchOnceMigrationUpAndDown(t *testing.T)
```
固定来源与真实允许来源各跑一次合同场景：fake policy/transport 下 (a) 固定 fixture 响应 (b) httptest 模拟的"已允许来源"两套各跑一遍核心链路（对应主计划原始证据要求的服务端部分）。

## 5. 验证（全跑附原始输出）

```bash
gofmt -w internal/modules/career internal/router
go test ./internal/modules/career/... -count=1
go test ./internal/database/... -count=1
go test ./internal/router/... -count=1
go test ./tools/architectureguard/... -count=1
git diff --check
```
architectureguard：当前基线 **587/656**（Wave 2 后），你的新路由落地后按精确新值更新清单（如 3 条路由→590/659，以实际为准）。

## 6. 全局约束（verbatim 摘录，违反即 Spec FAIL）

- 只在 assigned isolated Worktree 内改本任务所有权文件；local commits authorized, no push/merge/deploy/GitHub action。
- 搜索规则由用户显式启停并经预算准入（本任务不实现规则；额度只做 seam + typed 拒绝路径）。
- 不绕过 CAPTCHA/登录/反爬；服务端请求 URL 仅 http/https、校验 host、拒绝环回/私有/保留地址（复用 T09 transport 的 SSRF 防护，不另造 fetch）。
- 数据库查询一律参数绑定；网络 I/O 不得在 DB 事务内；同一 request ID 内容变化拒绝。
- Career 不 import Workbench repositories、不写 Workbench 表（search_once 不经 CareerApplicationTaskLinker）。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 7. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t11-search-once/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD、Step 0 与主任务各自的 RED/GREEN 证据、接口冻结说明（HTTP+JSON 形态，供 Web 子任务消费）、已知局限（生产无已核验来源是设计使然，真实来源核验属 T33）。COMMIT：Step 0 与主任务分开（`feat(career): run one-shot job searches`）。最终消息报告两个 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成。
