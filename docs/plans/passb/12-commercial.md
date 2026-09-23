# Pass B 子计划 12 — B1-CM Commercial 边界（8 legacy 文件：Admission/Budget/Usage/Payment 门面）

> 实施方式：superpowers:executing-plans / subagent-driven-development，按任务逐个执行，一个任务一个 commit。
> 节点：`b1-commercial`（DAG `docs/plans/passb/execution-dag.json`，execution_mode=parallel，role=work）。
> 本计划由「计划撰写-b1-commercial」于 2026-09-23 在 worktree `codex/passb-b1-commercial`（起点 `d57a2fa708c3fecf5f0510553ea26db3de51d3c9`）撰写；文中所有代码坐标均以该 SHA 的真实代码为准。

## 0. Spec 与事实源指针

| 事实源 | 位置 | 本计划取用的内容 |
|---|---|---|
| Spec | `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §5.13（Commercial 职责）、§11（B1 并行 + IB1 串行）、§12（Pass B 迁移循环）、§13（提交隔离 M1–M5 与回滚）、§14.3（高风险差分：支付/预算/Usage） | 边界语义、差分义务、提交纪律 |
| Pass B 框架 | `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` :92（B1-CM 8 文件表）、:29（`_test.go` 随迁、host 不留转发业务声明）、:40（payment/usage 差分面）、:97-105（IB1 职责） | 任务边界与差分面 |
| 冻结计划 | `docs/plans/passb/00-contract-and-ownership-freeze.md`（passb-int worktree） | B0 产物口径 |
| 执行公约 | `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | §1 派发契约、§2 门禁、§4 提交、§6 差分证据、§7 package-private 耦合 |
| MOVE-MANIFEST | `docs/architecture/moves/commercial.yaml` | 7 个 alias_obligations、8 条 legacy_files、importers、integration_points |
| ownership-matrix | `.worktrees/passb-int/docs/architecture/passb/ownership-matrix.yaml`（plan=12-commercial 的 8 行，:340/:556/:1288/:1564/:1786/:1792/:1990/:2356） | 精确 destination 与 delete_barrier |
| contracts.yaml | `.worktrees/passb-int/docs/architecture/passb/contracts.yaml`（owner: commercial 12 条，:652–:798） | 冻结契约与 consumers（本节点只读，回写归 IB1） |
| exception-ledger | `.worktrees/passb-int/docs/architecture/passb/exception-ledger.yaml` | 消费 commercial 的例外归属（本节点零删除，见 §9） |
| event-catalog | `.worktrees/passb-int/docs/architecture/passb/event-catalog.yaml`（Usage 家族 :231 起，producer 均在 Pass A 已搬包内） | 本节点不改任何事件生产者 |
| DAG 裁定 | `execution-dag.json` 节点 `b1-commercial.notes` + `package_private_couplings.pairs`（agentcatalog→commercial 5 sites / knowledge→commercial 4 sites） | 6 符号先搬 + 导出 model_usage 绑定族端口 |

### 前置条件（开工前逐条核验）

1. **b0 已 done**：`execution-dag.json` b0 节点 `status=done`、`head_sha=d57a2fa708c3fecf5f0510553ea26db3de51d3c9`（2026-09-23 18:56 台账条目按调度方指令以集成后 HEAD 覆盖）。b1-commercial `base_sha` 已回填同值、`status=in_progress`。
   - 节点 notes 中残留「BLOCKED（2026-09-23）：前置 b0 阻塞」为 06:26 遗留文本（台账 18:56 条目后续事项 2 在案，待调度方批量清理）；实施者以 DAG `status`/`base_sha` 字段与台账登记为准，不因该残留文本停工。
2. worktree `codex/passb-b1-commercial` 起点等于 `base_sha`（`git rev-parse HEAD` == `d57a2fa708c3fecf5f0510553ea26db3de51d3c9`），工作树干净。
3. 基线门禁绿：`go build ./...`、`go test ./internal/modules/commercial/... -count=1`、`make check-backend-architecture`、`make verify-module-moves` 在未改动的工作树上全部退出码 0（实施前实跑记录）。
4. 与并行节点文件不相交：b1-identity / b1-airesource / b1-execution 的 owned_files 与本计划 §3 的文件集无交集（framework:95）。

## 1. 目标与范围

**目标**：把困在三个水平宿主包（`internal/application/repository`、`internal/application/service`、`internal/handler`）的 8 个 commercial legacy 文件迁入 `internal/modules/commercial/**`，导出 model_usage 绑定族端口（供 B2-knowledge / B2-agentcatalog 消费），全程保持 `go build ./...` 绿、既有测试经过渡 shim 全绿、payment/usage 高风险差分证据落盘，并向 IB1 交付 Integration Brief。

**范围内（owned_files，与 ownership-matrix plan=12-commercial 的 8 行一致）**：

| # | legacy 文件 | destination（ownership-matrix 冻结） |
|---|---|---|
| 1 | `internal/application/repository/model_usage.go` | `internal/modules/commercial/repository/model_usage.go`（新包 `repository`） |
| 2 | `internal/application/repository/user_usage.go` | `internal/modules/commercial/repository/user_usage.go` |
| 3 | `internal/application/service/semantic_model_budget.go` | `internal/modules/commercial/service/semantic_model_budget.go`（新包 `service`） |
| 4 | `internal/application/service/usage_recorder.go` | `internal/modules/commercial/service/usage_recorder.go` |
| 5 | `internal/handler/commercial.go` | `internal/modules/commercial/handler/commercial.go`（新包 `handler`） |
| 6 | `internal/handler/commercial_task_budget.go` | `internal/modules/commercial/handler/commercial_task_budget.go` |
| 7 | `internal/handler/payment_callbacks.go` | `internal/modules/commercial/handler/payment_callbacks.go` |
| 8 | `internal/handler/usage.go` | `internal/modules/commercial/handler/usage.go` |

加上：`internal/modules/commercial/**` 内新增文件（实现、shim 的对端、新测试）、8 个旧路径上的**过渡薄 shim**（本节点自身产出，删除义务登记进 Integration Brief，见 §8）、随迁测试、`docs/architecture/evidence/passb/b1-commercial.md`、`docs/plans/passb/reports/b1-commercial.md`、`docs/architecture/passb/briefs/b1-commercial.md`。

**范围外（一律不动）**：
- `internal/router/router.go`、`internal/router/routes_commercial.go`、`internal/router/routes_usage.go`、`internal/container/container.go`、`internal/bootstrap/**`、`go.mod`、`go.sum`、`migrations/`（conventions §3 集成工程师独占/禁改）。
- 跨 owner 调用方文件：`internal/application/repository/knowledgebase.go`、`internal/application/repository/custom_agent.go`（9 个调用点零改动，由 shim 维持编译）、`internal/application/service/semantic_model.go`、`internal/application/service/semantic_model_test.go`（knowledge 属主，B2 迁移）。
- `docs/architecture/passb/{ownership-matrix,contracts,event-catalog,exception-ledger}.yaml`（b0 建、barrier 回写，conventions §3）。
- Pass A 已搬的 7 个包（`internal/modules/commercial` 根、`repository/commercial`、`service/commercial`、`commercialplatform`、`openmeter`、`payment`、`usage`）中的既有生产文件——本节点不修改其行为；contracts.yaml 冻结符号（`CanManageBilling`、`Available`、`ValidateFunding`、`ValidatePayment`、`SettlementKey` 等）的签名与语义不变（workbench→commercial 5 条、appconnector→commercial 4 条例外引用与 exc-0057 均消费这些包，见 §9）。

## 2. 现状断链面（真实代码证据，全部在 base SHA 实测）

### 2.1 六符号 package-private 耦合（DAG package_private_couplings 全量）

定义方（本节点文件）：

- `internal/application/repository/model_usage.go`：`knowledgeBaseModelUsageBindings`（:8）、`customAgentModelUsageBindings`（:31）、`scopeKnowledgeBasesByModelID`（:57）、`scopeCustomAgentsByModelID`（:80）、`scopeCustomAgentsBySandboxConfigID`（:101）
- `internal/application/service/semantic_model_budget.go`：`semanticBudgetFailure`（:114）

调用方（其他 owner，**零改动**）：

| 调用方文件 | 属主 | 调用点 |
|---|---|---|
| `internal/application/repository/knowledgebase.go` | knowledge | :216、:235、:242（scopeKnowledgeBasesByModelID ×2 + knowledgeBaseModelUsageBindings） |
| `internal/application/repository/custom_agent.go` | agentcatalog | :72、:87（scopeCustomAgentsByModelID）、:94（customAgentModelUsageBindings）、:119、:136（scopeCustomAgentsBySandboxConfigID） |
| `internal/application/service/semantic_model.go` | knowledge | :102（semanticBudgetFailure） |

测试文件对 6 符号零直接引用（grep 全仓 `*_test.go` 无命中）。

### 2.2 semantic_model_budget.go 的双向耦合

`internal/application/service/semantic_model.go:31` 定义 `type semanticCapability struct{...}`（未导出，knowledge 属主），而 semantic_model_budget.go 的 `reserve/release/markDispatched/finish` 四个方法签名都取 `semanticCapability` 参数（:26-29、:45/:68/:77/:89）。`semantic_model.go:98-134` 调用 `g.budget.reserve/release/markDispatched/finish` + `semanticBudgetFailure`。即：commercial 文件依赖 knowledge 类型、knowledge 文件依赖 commercial helper。处理见 §5.3（commercial 侧换导出参数类型 `SemanticBudgetCall`，宿主留 wrapper shim）。

`semantic_model_test.go`（knowledge 属主，留宿主）直接构造 `&SemanticModelBudgetAdapter{reserveFunc: reserve}`（:512）并设置 `markDispatchedFunc`（:216）、`finishFunc`（:263）——wrapper 必须保留同形 seam 字段（§5.3）。

### 2.3 handler 层对宿主包其他属主未导出符号的依赖（符号级交叉引用实测）

对 4 个 handler 文件做同包标识符交叉引用（剥离注释后匹配宿主包其余非测试文件顶层未导出声明）：

| 依赖符号 | 定义处（属主） | 消费文件 |
|---|---|---|
| `appFail` / `appOK` | `internal/handler/app_connector.go:28/:32`（appconnector，各 3 行 gin JSON helper） | `commercial_task_budget.go` |
| `parseAnalyticsRange` | `internal/handler/analytics.go:40`（insights） | `usage.go`（:89/:114/:138） |
| `analyticsRows`（泛型） | `internal/handler/analytics.go:90`（insights） | `usage.go`（:107/:128） |
| `parseFilterTime` | `internal/handler/knowledge.go:2565`（knowledge） | 经 `parseAnalyticsRange` 间接 |

`commercial.go` 与 `payment_callbacks.go` 对宿主包零外部依赖（`callbackFail`、`commercialTenantScope`、`ErrMissingTenantScope` 等均在本节点文件内定义）。`parseAnalyticsRange` 还依赖 `analyticsIsDateOnly`（analytics.go:68）与 `analyticsDefaultLookbackDays = 30`（analytics.go:16）。

### 2.4 禁改文件消费面（shim 必须覆盖的导出符号，grep 实测）

| 消费方 | 引用符号 | 位置 |
|---|---|---|
| `internal/container/container.go` | `repository.NewUsageRepository` | :213 |
| | `handler.NewCommercialHandler` | :867 |
| | `handler.CommercialHandler`（类型，多处 Invoke） | :881/:892/:902/:917/:1016 |
| | `service.NewSemanticModelBudgetAdapter` | :1069 |
| | `service.NewUsageRecorderService` | :2600（返回值赋给 `interfaces.UsageRecorderService`） |
| | `handler.NewUsageHandler` | :734 |
| `internal/router/routes_commercial.go` | `handler.CommercialHandler`、`handler.PaymentCallbacksHandler`、`handler.NewPaymentCallbacksHandler`（:112 nil-fallback） | 全文件 |
| `internal/router/routes_usage.go` | `handler.UsageHandler` | :21 |
| `internal/router/*_test.go` | `&handler.UsageHandler{}`（routes_query_history_test.go:29）等 | 6 个测试文件 |

`CommercialAPIKeyCapability`、`ErrQuotaExceeded`、`ResourceWriteFunc`、`ErrMissingTenantScope` 在 4 文件之外的非测试代码零消费（grep 实测仅 benefits.go:253 注释提及）；为保持 router 测试与潜在引用编译安全，shim 一并转发（§5.4）。

### 2.5 测试文件归属裁定（framework:29 的执行口径）

| 测试文件 | 依赖 | 裁定 |
|---|---|---|
| `internal/application/repository/model_usage_test.go`（7 用例） | `setupKBTestDB`/`makeKB`（knowledgebase_sqlite_test.go，knowledge 属主测试基建）、`NewKnowledgeBaseRepository`、custom_agents DDL | **留宿主**作端到端 SQL 特征化（shim 转发后自动覆盖新实现）；随 knowledge 侧在 B2 迁移。commercial 侧新写纯函数单测（§6 T2） |
| `internal/application/repository/user_usage_test.go`（2 用例） | `openRunTestDB`（agent_run_test.go:29，execution 属主测试基建，含 migrations+fixtures） | 同上：留宿主作特征化差分锚；commercial 侧新写自包含单测（§6 T2） |
| `internal/application/service/usage_recorder_test.go`（6 用例） | 自包含（stubUsageRepo 自带） | **随迁** `internal/modules/commercial/service/`（纯移动，包名同为 `service`，import 零变化） |
| `internal/handler/usage_test.go`（10 用例） | 自包含（middleware/types/interfaces + 包内构造 `&UsageHandler{...}`） | **随迁** `internal/modules/commercial/handler/`（`usage.go` 生产文件同迁，同包可见） |
| `internal/handler/payment_callbacks_test.go`、`commercial_task_budget_test.go` | 不存在 | **T1 新建**（特征化，自包含），T4 随迁 |

偏差登记：model_usage_test.go / user_usage_test.go 两个文件不满足「随迁」字面要求，原因是机械随迁将强制复制 knowledge/execution 属主的共享测试基建（违反 conventions §5「复制他模块实现」精神）；两个文件经 shim 持续锚定新实现的 SQL 行为，等价性由 §7 双跑差分证明，最终归属在 B2/b5 由 Integration Brief 链条收口。此偏差写入实施报告「未完全满足项」。

## 3. 目标包结构与写入所有权

```
internal/modules/commercial/
├── repository/                      # 新包 repository（与既有子包 repository/commercial 并存）
│   ├── model_usage.go               # 新增：6 符号中的 5 个，导出版
│   ├── user_usage.go                # git mv 自 internal/application/repository/
│   ├── model_usage_bindings_test.go # 新增：导出符号纯函数单测
│   └── user_usage_test.go           # 新增：自包含 sqlite 单测（不复制 execution 基建）
├── service/                         # 新包 service（与既有子包 service/commercial 并存）
│   ├── semantic_model_budget.go     # 新增：导出版 SemanticModelBudgetAdapter
│   ├── usage_recorder.go            # git mv 自 internal/application/service/
│   ├── usage_recorder_test.go       # git mv 随迁
│   └── semantic_model_budget_test.go# 新增：Reserve/Release/MarkDispatched/Finish/BYOK 特征化
└── handler/                         # 新包 handler
    ├── commercial.go                # git mv 自 internal/handler/
    ├── commercial_task_budget.go    # git mv 自 internal/handler/
    ├── payment_callbacks.go         # git mv 自 internal/handler/
    ├── usage.go                     # git mv 自 internal/handler/（+局部 helper 族）
    ├── handler_helpers.go           # 新增：appOK/appFail 副本（barrier 收口登记）
    ├── usage_range.go               # 新增：parseAnalyticsRange 族副本（barrier 收口登记）
    ├── usage_test.go                # git mv 随迁
    ├── payment_callbacks_test.go    # T1 新建于宿主，T4 git mv 随迁
    └── commercial_task_budget_test.go # T1 新建于宿主，T4 git mv 随迁
```

宿主旧路径过渡 shim（全部为 no-logic forwarding，删除义务见 §8）：

```
internal/application/repository/model_usage.go   # 重写为 5 个未导出转发 func
internal/application/repository/user_usage.go    # 重写为 NewUsageRepository 转发
internal/application/service/semantic_model_budget.go  # 重写为 wrapper（§5.3）
internal/application/service/usage_recorder.go   # 重写为 NewUsageRecorderService 转发
internal/handler/commercial.go                   # 重写为类型别名+构造器/const/var 转发
internal/handler/usage.go                        # 重写为 UsageHandler 别名+构造器转发
internal/handler/payment_callbacks.go            # 重写为 PaymentCallbacksHandler 别名+构造器转发
# internal/handler/commercial_task_budget.go 无顶层声明（仅方法），直接删除
```

治理/证据产出（本节点写）：

```
docs/architecture/evidence/passb/b1-commercial.md   # 差分证据（§7）
docs/plans/passb/reports/b1-commercial.md           # 实施报告（conventions §1.2 命令包）
docs/architecture/passb/briefs/b1-commercial.md     # Integration Brief（交付 IB1）
```

**写入所有权自检**：上述生产/测试/shim/文档清单 ⊆ 节点 owned_files（manifest legacy_files 8 条 + `internal/modules/commercial/**` + 节点自身产出新文件）。禁改清单（§1 范围外）零触碰。

## 4. 导出端口（真实签名，不发明）

以下签名逐字取自 base SHA 现有实现，仅做「未导出→导出」与参数类型替换两处机械变换，函数体逻辑不变：

### 4.1 `internal/modules/commercial/repository/model_usage.go`（package repository）

```go
package repository

import (
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// KnowledgeBaseModelUsageBindings ...（原注释随迁）
func KnowledgeBaseModelUsageBindings(kb *types.KnowledgeBase, modelID string) []types.ModelUsageBinding

// CustomAgentModelUsageBindings ...
func CustomAgentModelUsageBindings(agent *types.CustomAgent, modelID string) []types.ModelUsageBinding

// ScopeKnowledgeBasesByModelID ...
func ScopeKnowledgeBasesByModelID(db *gorm.DB, modelID string) *gorm.DB

// ScopeCustomAgentsByModelID ...
func ScopeCustomAgentsByModelID(db *gorm.DB, modelID string) *gorm.DB

// ScopeCustomAgentsBySandboxConfigID ...
func ScopeCustomAgentsBySandboxConfigID(db *gorm.DB, configID string) *gorm.DB
```

函数体 = 现有 `internal/application/repository/model_usage.go:8-106` 逐字搬迁（含 postgres/sqlite 双方言 SQL 与全部注释）。这 5 个导出符号即 DAG notes 要求的「model_usage 绑定族端口」，B2-knowledge（knowledgebase.go）与 B2-agentcatalog（custom_agent.go）迁移时改引此包（由各自计划执行，本节点不改调用点）。

### 4.2 `internal/modules/commercial/service/semantic_model_budget.go`（package service）

```go
package service

import (
	"context"
	"errors"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
)

var ErrSemanticModelRatesUnavailable = errors.New("semantic_model_rates_unavailable")

// SemanticBudgetCall 是 reserve/release/markDispatched/finish 的调用参数，
// 取代宿主包未导出的 semanticCapability（semantic_model.go:31，knowledge 属主）。
// 字段与 semanticCapability 一一对应（owner/runID/callID/modelID/funding/priceVersion/upper/deadline）。
type SemanticBudgetCall struct {
	Owner                                         uint64
	ModelID, Funding, PriceVersion, RunID, CallID string
	Upper                                         int64
	Deadline                                      time.Time
}

type SemanticModelBudgetAdapter struct { /* 字段与现实现一致（store/gate/rates/usage + 4 个 seam 字段 + releaseCalls/finishCalls） */ }

func NewSemanticModelBudgetAdapter(store *repocommercial.BudgetStore, gate domain.ExecutionGate, rates repocommercial.RateResolver) *SemanticModelBudgetAdapter

func (b *SemanticModelBudgetAdapter) WithUsageStore(usage *repocommercial.UsageStore) *SemanticModelBudgetAdapter

func (b *SemanticModelBudgetAdapter) Reserve(ctx context.Context, c SemanticBudgetCall) (domain.Reservation, error)
func (b *SemanticModelBudgetAdapter) Release(ctx context.Context, c SemanticBudgetCall, r domain.Reservation) error
func (b *SemanticModelBudgetAdapter) MarkDispatched(ctx context.Context, c SemanticBudgetCall, r domain.Reservation) error
func (b *SemanticModelBudgetAdapter) Finish(ctx context.Context, c SemanticBudgetCall, r domain.Reservation, input, output int64) error

// SemanticBudgetFailure 是原 semanticBudgetFailure 的导出端口。
func SemanticBudgetFailure(err error) error { return fmt.Errorf("semantic model budget: %w", err) }
```

方法体 = 现实现 :45-112 逐字搬迁，仅把参数类型 `semanticCapability` 换成 `SemanticBudgetCall`、字段访问 `c.owner`→`c.Owner`、`c.priceVersion`→`c.PriceVersion` 等大小写替换；`reserve/release/markDispatched/finish` 改名 `Reserve/Release/MarkDispatched/Finish`；seam 字段（reserveFunc 等）类型同步换成 `SemanticBudgetCall`，保留「intentionally package-local」注释与 commercial 包内可用性。

### 4.3 `internal/modules/commercial/service/usage_recorder.go`

纯移动，零改动：`UsageRecorder`、`NewUsageRecorderService(repo interfaces.UsageRepository, rates usage.ModelRates) *UsageRecorder`、`RecordChatTurn`、`var _ interfaces.UsageRecorderService = (*UsageRecorder)(nil)`。

### 4.4 `internal/modules/commercial/repository/user_usage.go`

纯移动，零改动：`usageRepository`、`NewUsageRepository(db *gorm.DB) interfaces.UsageRepository`、`usageDayExpr/usageWindowPredicate/usageSumCols`、`AddUsage/AggregateByUser/AggregateAllUsers/ExportRows`。

### 4.5 `internal/modules/commercial/handler`（4 文件）

`CommercialHandler`（含 `NewCommercialHandler(db *gorm.DB) *CommercialHandler` 与全部 32 个方法）、`PaymentCallbacksHandler`（含 `NewPaymentCallbacksHandler(db *gorm.DB, providers map[string]payment.Provider) *PaymentCallbacksHandler`）、`UsageHandler`（含 `NewUsageHandler(repo interfaces.UsageRepository) *UsageHandler`）——纯移动。新增局部 helper（逐字副本，出处与收口登记见 §8）：

`handler_helpers.go`：
```go
func appOK(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"success": true, "data": data})
}
func appFail(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"success": false, "error": gin.H{"code": code, "message": message}})
}
```

`usage_range.go`：`analyticsDefaultLookbackDays = 30`、`parseAnalyticsRange`、`analyticsIsDateOnly`、`analyticsRows[T any]`、`parseFilterTime`，函数体逐字取自 `analytics.go:16/:40/:68/:90` 与 `knowledge.go:2565-2578`（含 RFC3339Nano/RFC3339/`"2006-01-02 15:04:05"`/`"2006-01-02"` 四布局与 time.Local 语义）。

## 5. 过渡 shim 精确设计（宿主旧路径，全部 no-logic forwarding）

### 5.1 `internal/application/repository/model_usage.go`（重写）

```go
package repository

import (
	"github.com/Tencent/WeKnora/internal/types"
	commercialrepo "github.com/Tencent/WeKnora/internal/modules/commercial/repository"
	"gorm.io/gorm"
)

// Pass B (12-commercial) 过渡 shim：实现已迁 internal/modules/commercial/repository。
// 调用方（knowledgebase.go / custom_agent.go，knowledge 与 agentcatalog 属主）由 IB1/B2 切换后删除本文件。

func knowledgeBaseModelUsageBindings(kb *types.KnowledgeBase, modelID string) []types.ModelUsageBinding {
	return commercialrepo.KnowledgeBaseModelUsageBindings(kb, modelID)
}
func customAgentModelUsageBindings(agent *types.CustomAgent, modelID string) []types.ModelUsageBinding {
	return commercialrepo.CustomAgentModelUsageBindings(agent, modelID)
}
func scopeKnowledgeBasesByModelID(db *gorm.DB, modelID string) *gorm.DB {
	return commercialrepo.ScopeKnowledgeBasesByModelID(db, modelID)
}
func scopeCustomAgentsByModelID(db *gorm.DB, modelID string) *gorm.DB {
	return commercialrepo.ScopeCustomAgentsByModelID(db, modelID)
}
func scopeCustomAgentsBySandboxConfigID(db *gorm.DB, configID string) *gorm.DB {
	return commercialrepo.ScopeCustomAgentsBySandboxConfigID(db, configID)
}
```

（原文件顶部 doc 注释一并保留在 shim 之上。）宿主包 → 模块包 import 方向合法：architectureguard forbidden-import 仅扫描 `internal/modules/**` 内文件（check.go:1175-1220 WalkDir modRoot），宿主包不在检查范围。

### 5.2 `internal/application/repository/user_usage.go` 与 `internal/application/service/usage_recorder.go`（重写为构造器转发）

```go
// user_usage.go（shim）
package repository

import (
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	commercialrepo "github.com/Tencent/WeKnora/internal/modules/commercial/repository"
	"gorm.io/gorm"
)

// NewUsageRepository returns the SQL-backed UsageRepository.（原注释保留）
// Pass B (12-commercial) 过渡 shim — 消费方 container.go:213 由 IB1 切换。
func NewUsageRepository(db *gorm.DB) interfaces.UsageRepository {
	return commercialrepo.NewUsageRepository(db)
}
```

```go
// usage_recorder.go（shim）
package service

import (
	"github.com/Tencent/WeKnora/internal/modules/commercial/usage"
	commercialservice "github.com/Tencent/WeKnora/internal/modules/commercial/service"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// NewUsageRecorderService ...（原注释保留）
// Pass B (12-commercial) 过渡 shim — 消费方 container.go:2600 由 IB1 切换。
func NewUsageRecorderService(repo interfaces.UsageRepository, rates usage.ModelRates) *commercialservice.UsageRecorder {
	return commercialservice.NewUsageRecorderService(repo, rates)
}
```

返回具体类型 `*commercialservice.UsageRecorder` 对 `container.go:2594-2600`（赋给 `interfaces.UsageRecorderService` 返回）与全部既有消费者（`internal/handler/session/handler.go`、`internal/modules/channels/im/service.go` 均只消费接口，contracts.yaml consumers 实测）编译兼容。

### 5.3 `internal/application/service/semantic_model_budget.go`（重写为 wrapper，保 seam 同形）

语义：宿主 shim 是薄壳——seam 字段同形（供 `semantic_model_test.go:511-515` 直接构造），未设置 seam 时委托 commercial 导出方法；**不复制任何预算业务逻辑**。

```go
package service

import (
	"context"
	"fmt"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialservice "github.com/Tencent/WeKnora/internal/modules/commercial/service"
)

// Pass B (12-commercial) 过渡 wrapper：实现已迁 internal/modules/commercial/service。
// 消费方 semantic_model.go（knowledge 属主）与 container.go:1069 由 IB1/B2 切换后删除。
var ErrSemanticModelRatesUnavailable = commercialservice.ErrSemanticModelRatesUnavailable

type SemanticModelBudgetAdapter struct {
	// Test seams 保持与原实现同形（semantic_model_test.go 直接构造/赋值）。
	reserveFunc        func(context.Context, semanticCapability) (domain.Reservation, error)
	releaseFunc        func(context.Context, semanticCapability, domain.Reservation) error
	markDispatchedFunc func(context.Context, semanticCapability, domain.Reservation) error
	finishFunc         func(context.Context, semanticCapability, domain.Reservation, int64, int64) error
	releaseCalls       int
	finishCalls        int
	inner              *commercialservice.SemanticModelBudgetAdapter
}

func NewSemanticModelBudgetAdapter(store *repocommercial.BudgetStore, gate domain.ExecutionGate, rates repocommercial.RateResolver) *SemanticModelBudgetAdapter {
	return &SemanticModelBudgetAdapter{inner: commercialservice.NewSemanticModelBudgetAdapter(store, gate, rates)}
}

func (b *SemanticModelBudgetAdapter) WithUsageStore(usage *repocommercial.UsageStore) *SemanticModelBudgetAdapter {
	if b != nil && b.inner != nil {
		b.inner.WithUsageStore(usage)
	}
	return b
}

func budgetCallFrom(c semanticCapability) commercialservice.SemanticBudgetCall {
	return commercialservice.SemanticBudgetCall{
		Owner: c.owner, ModelID: c.modelID, Funding: c.funding, PriceVersion: c.priceVersion,
		RunID: c.runID, CallID: c.callID, Upper: c.upper, Deadline: c.deadline,
	}
}

func (b *SemanticModelBudgetAdapter) reserve(ctx context.Context, c semanticCapability) (domain.Reservation, error) {
	if b != nil && b.reserveFunc != nil {
		return b.reserveFunc(ctx, c)
	}
	if b == nil || b.inner == nil {
		return domain.Reservation{}, ErrSemanticModelRatesUnavailable
	}
	return b.inner.Reserve(ctx, budgetCallFrom(c))
}

func (b *SemanticModelBudgetAdapter) release(ctx context.Context, c semanticCapability, r domain.Reservation) error {
	if b != nil && b.releaseFunc != nil {
		return b.releaseFunc(ctx, c, r)
	}
	if b == nil || b.inner == nil {
		return nil // 与原实现 b==nil 时 store 分支不可达语义一致：release 对非 platform 直接放行
	}
	return b.inner.Release(ctx, budgetCallFrom(c), r)
}

func (b *SemanticModelBudgetAdapter) markDispatched(ctx context.Context, c semanticCapability, r domain.Reservation) error {
	if b != nil && b.markDispatchedFunc != nil {
		return b.markDispatchedFunc(ctx, c, r)
	}
	if b == nil || b.inner == nil {
		return ErrSemanticModelRatesUnavailable
	}
	return b.inner.MarkDispatched(ctx, budgetCallFrom(c), r)
}

func (b *SemanticModelBudgetAdapter) finish(ctx context.Context, c semanticCapability, r domain.Reservation, input, output int64) error {
	if b != nil && b.finishFunc != nil {
		return b.finishFunc(ctx, c, r, input, output)
	}
	if b == nil || b.inner == nil {
		return ErrSemanticModelRatesUnavailable
	}
	return b.inner.Finish(ctx, budgetCallFrom(c), r, input, output)
}

func semanticBudgetFailure(err error) error { return commercialservice.SemanticBudgetFailure(err) }
```

nil 语义对照（与原实现 :45-112 逐分支核对）：原 `reserve` 在 `b==nil || rates==nil || priceVersion==""` 返回 `ErrSemanticModelRatesUnavailable` → wrapper `b==nil||inner==nil` 同错；原 `release` 仅在 `funding==platform && r.ID!=""` 才触碰 store，`b==nil` 时 `c.funding != platform` 短路 nil——wrapper 以「inner 缺失即放行」近似（生产构造器恒建 inner，该分支仅在手工构造空 wrapper 时可达；语义差异为零，因生产代码不存在无 inner 实例）。此推理写入实施报告，审查者复核。

### 5.4 `internal/handler/{commercial,usage,payment_callbacks}.go`（重写为别名/转发；`commercial_task_budget.go` 删除）

```go
// commercial.go（shim）
package handler

import (
	"github.com/Tencent/WeKnora/internal/modules/commercial/payment"
	commercialhandler "github.com/Tencent/WeKnora/internal/modules/commercial/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// Pass B (12-commercial) 过渡 shim — 消费方 routes_commercial.go / container.go / router 测试由 IB1 切换。
type CommercialHandler = commercialhandler.CommercialHandler
type PaymentCallbacksHandler = commercialhandler.PaymentCallbacksHandler
type UsageHandler = commercialhandler.UsageHandler
type ResourceWriteFunc = commercialhandler.ResourceWriteFunc

const CommercialAPIKeyCapability = commercialhandler.CommercialAPIKeyCapability

var (
	ErrQuotaExceeded      = commercialhandler.ErrQuotaExceeded
	ErrMissingTenantScope = commercialhandler.ErrMissingTenantScope
)

func NewCommercialHandler(db *gorm.DB) *CommercialHandler {
	return commercialhandler.NewCommercialHandler(db)
}
func NewPaymentCallbacksHandler(db *gorm.DB, providers map[string]payment.Provider) *PaymentCallbacksHandler {
	return commercialhandler.NewPaymentCallbacksHandler(db, providers)
}
func NewUsageHandler(repo interfaces.UsageRepository) *UsageHandler {
	return commercialhandler.NewUsageHandler(repo)
}
```

（补 `interfaces` import。三个原文件的顶层符号全部被别名/转发覆盖：`CommercialHandler` 全部 32 个方法、`ExtendTaskBudget`、`HandleProviderCallback` 等均为类型方法，随别名自动可见；`usage.go` 的 `usageDefaultPageSize/usageMaxPageSize/usageExportColumns/usagePageParams` 为未导出且仅被本文件与随迁测试使用，不留 shim。）

## 6. 任务分解（业务完整、可独立审阅；严格按序执行）

### T1（B1-CM.1）— 特征化基线：补齐 handler 缺失测试并冻结全量基线输出

**文件**：新增 `internal/handler/payment_callbacks_test.go`、`internal/handler/commercial_task_budget_test.go`（此时在宿主包，T4 随迁）。仅测试文件，零生产改动。

**步骤**：
1. 写 `payment_callbacks_test.go`（package handler，自包含 stub）锚定分派/响应形态（用例清单）：
   - `TestPaymentCallbacksUnknownProviderFailsClosed`：`NewPaymentCallbacksHandler(nil, nil)` + POST `/commercial/callbacks/wechat` → 503、body `{"code":"FAIL","message":"unknown payment provider"}`；
   - `TestPaymentCallbacksVerifyFailureIs401AndPersistsNothing`：stub `payment.Provider`（实现 `payment.Provider` 接口，Verify 返回 error）→ 401 FAIL；
   - `TestPaymentCallbacksMissingStoreIs404`：Verify 成功但 db==nil（`callback_store_unavailable` 路径）→ 404 FAIL；
   - `TestPaymentCallbacksAlipayBranchAnswersPlainTextFailure`：provider=alipay 且 providers 空 → 503、纯文本 body `failure`；
   - `TestPaymentCallbacksAlipaySuccessAckIsPlainTextSuccess`：stub alipay provider + sqlite db（`commercial_payment_attempts` 表 DDL 自建 + 注册一行 attempt，参照 `payment_callbacks.go:48-51` 的 Raw SQL 列）+ stub ConfirmPayment 成功 → 200 纯文本 `success`。
   （`payment.Provider` 接口以 `internal/modules/commercial/payment` 真实定义为准——实施时先读该接口再写 stub，不发明方法集。）
2. 写 `commercial_task_budget_test.go`（package handler，sqlite db 自建 `commercial_resource_counters` 等 DDL，参照 `commercial_task_budget.go:40-48` 与 `commercialsvc.NewBudgetService` 的表需求）：
   - `TestExtendTaskBudgetRequiresTenantScope`：无 tenant 上下文 → 403 `MISSING_TENANT_SCOPE`；
   - `TestExtendTaskBudgetRejectsInvalidBody`：缺 `additional_credits`/`idempotency_key` → 400 `INVALID_REQUEST`；
   - `TestExtendTaskBudgetWithoutDBIs503`：`&CommercialHandler{}`（db nil）→ 503 `BUDGET_DATABASE_MISSING`；
   - `TestExtendTaskBudgetMissingTaskBudgetIs404`：真 sqlite + BudgetService 可构造、目标 run 无预算行 → 404 `TASK_BUDGET_NOT_FOUND`（若 BudgetService 构造本身失败则断言 503 并在报告记录 blocked-env）。
3. 跑新测试至全绿（特征化锚定旧实现；失败即修测试而非改生产码——生产码本节点视为冻结输入）。
4. 冻结全量基线输出（§7 命令集逐条 `-v` 运行，退出码与输出存入 `docs/architecture/evidence/passb/b1-commercial.md` 草稿）。

**命令与预期**：
```bash
go build ./...                                                   # 退出码 0
go test ./internal/handler/ -run 'TestPaymentCallbacks|TestExtendTaskBudget' -count=1 -v   # 全部 PASS
go test ./internal/application/repository/ -run 'TestCountByModelID|TestListModelUsages|TestCustomAgentSandboxConfigReferences|TestUserUsage' -count=1 -v  # 全部 PASS（7+2 用例）
go test ./internal/application/service/ -run 'TestRecordChatTurn|TestSemanticModel' -count=1 -v   # 全部 PASS（6+17 用例）
go test ./internal/handler/ -run 'TestUsageHandler' -count=1 -v  # 10 用例 PASS
go test ./internal/router/ -run 'TestCommercial|TestPlanAdmin|TestAccount|TestRegisterQueryHistoryAdminRoutes' -count=1 -v  # 19+ 用例 PASS
go test ./internal/modules/commercial/... -count=1               # PASS（含 TestProvidersFromEnvRejectsPartialAlipay 已知 map 序债，见 §7.4）
```

**Commit**：`test(passb): characterize payment callbacks and task budget handlers before move`
**回滚边界**：本任务纯增测试，`git revert` 单 commit 即回滚，不影响任何生产路径。

### T2（B1-CM.2）— repository 层搬迁：model_usage 绑定族端口 + user_usage

**文件**：新增 `internal/modules/commercial/repository/model_usage.go`（§4.1）；`git mv internal/application/repository/user_usage.go internal/modules/commercial/repository/user_usage.go`；重写旧路径 `model_usage.go`/`user_usage.go` 为 shim（§5.1/§5.2）；新增 `model_usage_bindings_test.go`、`user_usage_test.go`（commercial 侧）。

**步骤**：
1. `git mv` user_usage.go（纯移动零改动）。
2. 写 commercial 侧 `model_usage.go`（5 个导出符号，函数体逐字）。
3. 重写两个宿主 shim。
4. 新增 commercial 测试：
   - `model_usage_bindings_test.go`：纯函数表驱动——`KnowledgeBaseModelUsageBindings` 六绑定维度（embedding/summary/image/VLM/ASR/wiki-synthesis，含 `WikiConfig==nil` 分支）、`CustomAgentModelUsageBindings` 六维度（含 `QuestionSuggestions==nil` 分支）、`ScopeCustomAgentsBySandboxConfigID` 在 sqlite gorm 上对 postgres 分支不可达的说明性注释（方言分支由留宿主的 model_usage_test.go 端到端覆盖）；
   - `user_usage_test.go`：自包含 sqlite（`gorm.Open(sqlite.Open(...))` + `db.AutoMigrate(&types.UserUsage{})`）复刻原 2 用例断言核心：同维度 tuple 二次 AddUsage 累加、`AggregateByUser` 窗口聚合与排序（不依赖 execution 的 openRunTestDB 基建；PostgreSQL 方言差分按原测试口径保留在宿主特征化文件）。
5. `git diff --summary` 确认 rename 识别（user_usage.go R100）。

**命令与预期**：
```bash
go build ./...                                    # 0
go test ./internal/modules/commercial/repository/ -count=1 -v   # 新测试 PASS
go test ./internal/application/repository/ -run 'TestCountByModelID|TestListModelUsages|TestCustomAgentSandboxConfigReferences|TestUserUsage' -count=1 -v  # 经 shim 全 PASS（与 T1 基线逐用例等价）
make check-backend-architecture                   # 0（shim 在 legacy 登记路径上，forbidden-import 不扫宿主包）
make verify-module-moves                          # 0
```

**Commit**：`refactor(commercial): move model_usage bindings and user_usage repository into module`
**回滚边界**：revert 本 commit 即回到宿主实现；无共享装配变更，无数据/迁移影响。

### T3（B1-CM.3）— service 层搬迁：usage_recorder 随迁 + SemanticModelBudgetAdapter 导出端口

**文件**：`git mv internal/application/service/usage_recorder.go internal/modules/commercial/service/usage_recorder.go` + `git mv` 同名 `_test.go`；新增 `internal/modules/commercial/service/semantic_model_budget.go`（§4.2）+ `semantic_model_budget_test.go`；重写旧路径两文件为 shim/wrapper（§5.2/§5.3）。

**步骤**：
1. 两个 `git mv`（usage_recorder 纯移动，import 零变化：`commercial/usage`、`types`、`types/interfaces` 均仍合法）。
2. 写 commercial 侧 `semantic_model_budget.go`（§4.2；`SemanticBudgetCall` 字段映射表见 §5.3 `budgetCallFrom`）。
3. 重写宿主 shim（usage_recorder 构造器转发）与 wrapper（semantic_model_budget）。
4. 新增 `semantic_model_budget_test.go`（commercial 侧，锚定导出端口）：
   - `TestReserveMissingRatesFailsClosed`：`NewSemanticModelBudgetAdapter(nil, nil, nil).Reserve(ctx, SemanticBudgetCall{})` → `ErrSemanticModelRatesUnavailable`；
   - `TestReserveBYOKRequiresUsageStore`：funding=BYOK、无 usage store → 同错；有 store → 零值 Reservation、nil error；
   - `TestFinishBYOKRecordsRawUsageOnce`：stub `repocommercial.UsageStore`（或真 sqlite UsageStore，取其构造可行性）+ rates 匹配 → RecordRawModelUsage 收到 `Status=UsageStatusFinal`、`Dimensions[DimensionModel]=input+output` 的 UsageFact；
   - `TestFinishPlatformResolvesRatesBeforeGate`：rates 版本不匹配 → 拒绝，gate.Finish 未被调用（复刻 semantic_model_test.go:254/:300 用例的 budget 侧半段，参数经 `SemanticBudgetCall`）。
   （stub/构造以 `internal/modules/commercial/repository/commercial` 真实类型为准——实施时先读 `UsageStore`/`BudgetStore`/`RateResolver` 定义再写，不发明。）
5. 全量回归（§7 命令集）。

**命令与预期**：
```bash
go build ./...                                                        # 0
go test ./internal/modules/commercial/service/ -count=1 -v            # usage_recorder 6 用例 + 新 budget 用例 PASS
go test ./internal/application/service/ -run 'TestRecordChatTurn|TestSemanticModel' -count=1 -v  # 经 wrapper 全 PASS，与基线等价
go test ./internal/application/repository/ -count=1                    # 宿主 repository 无回归（顺带全量）
```

**Commit**：`refactor(commercial): export semantic budget adapter and move usage recorder into module`
**回滚边界**：revert 即回宿主实现；wrapper 不触碰 container/router，装配零变更。

### T4（B1-CM.4）— handler 层搬迁：4 文件 + helper 副本 + 类型别名 shim + 测试随迁

**文件**：`git mv` 4 个生产文件 + `usage_test.go` + T1 的两个测试文件 → `internal/modules/commercial/handler/`；新增 `handler_helpers.go`、`usage_range.go`（§4.5）；重写 3 个宿主 shim（§5.4）；删除 `internal/handler/commercial_task_budget.go`（无顶层声明）。

**步骤**：
1. `git mv internal/handler/{commercial,commercial_task_budget,payment_callbacks,usage}.go internal/modules/commercial/handler/`；同批 mv 三个测试文件。
2. 新增 `handler_helpers.go`（appOK/appFail 逐字副本）与 `usage_range.go`（parseAnalyticsRange 族 + parseFilterTime 逐字副本），文件头注释注明来源文件、属主（appconnector/insights/knowledge）与「barrier 收口为单一实现」登记（conventions §7.1 先例：多属主重复 helper 族在 barrier 收口）。
3. 重写 3 个宿主 shim（§5.4 代码）。
4. `git rm internal/handler/commercial_task_budget.go`（其唯一内容是 `ExtendTaskBudget` 方法，随类型别名迁移）。

**命令与预期**：
```bash
go build ./...                                                      # 0（routes/container/经 shim 编译）
go test ./internal/modules/commercial/handler/ -count=1 -v          # usage 10 + payment 5 + task-budget 4 用例 PASS
go test ./internal/router/ -run 'TestCommercial|TestPlanAdmin|TestAccount|TestRegisterQueryHistoryAdminRoutes' -count=1 -v  # 19+ 用例 PASS（经别名 shim）
go test ./internal/handler/ -count=1                                # 宿主包其余测试零回归
make check-backend-architecture                                     # 0
make verify-module-moves                                            # 0
```

**Commit**：`refactor(commercial): move commercial/payment/usage handlers into module with alias shims`
**回滚边界**：revert 即回宿主 handler；路由注册文件零改动，633 路由计数不变。

### T5（B1-CM.5）— 差分证据收口 + Integration Brief + 实施报告

**文件**：`docs/architecture/evidence/passb/b1-commercial.md`、`docs/architecture/passb/briefs/b1-commercial.md`、`docs/plans/passb/reports/b1-commercial.md`。

**步骤**：
1. 差分证据定稿（§7）：用例清单表、基线/搬迁后双跑命令原文+退出码+关键输出、逐用例等价结论、payment/usage 高风险面专节、map-order 已知债处置记录。
2. Integration Brief（§8 模板）。
3. 实施报告：conventions §1.2 四条命令包逐条执行并摘录；变更文件 vs owned_files 逐条核对；§2.5 测试随迁偏差如实登记。
4. DAG `task_ids` 由协调者回填，本任务不改 `execution-dag.json`（写权限归协调者，conventions §9）。

**命令与预期**（= 节点 gates 原文 + 报告命令包）：
```bash
PASSB_BASE_SHA=d57a2fa708c3fecf5f0510553ea26db3de51d3c9
git diff --stat "$PASSB_BASE_SHA"...HEAD            # 变更清单 ⊆ §3 所列文件
go build ./...                                       # 0
go test -count=1 ./internal/modules/commercial/...   # PASS
make check-backend-architecture                      # 0
make verify-module-moves                             # 0
git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort # 与 owned_files 求差集 = ∅
```

**Commit**：`docs(passb): record b1-commercial differential evidence and integration brief`

## 7. 高风险差分证据要求（framework:40：payment/usage；conventions §6）

**方法**：旧实现特征化（T1 冻结基线输出）→ 搬迁（T2–T4）→ 同用例复跑 → 逐用例等价比对。差分证据（用例清单、双跑输出、比对结论、命令与退出码）写入 `docs/architecture/evidence/passb/b1-commercial.md`。

**用例矩阵**（搬迁前后同集合同命令）：

| 面 | 用例来源 | 数量 |
|---|---|---|
| model_usage 绑定/SQL | `model_usage_test.go`（留宿主，经 shim） | 7 |
| user_usage SQL | `user_usage_test.go`（留宿主，经 shim）+ commercial 新单测 | 2 + 新增 |
| usage 记量 | `usage_recorder_test.go`（随迁，同文件同用例） | 6 |
| usage HTTP | `usage_test.go`（随迁） | 10 |
| payment callbacks HTTP | T1 新建（宿主→随迁，同文件同用例） | 5 |
| task budget HTTP | T1 新建（宿主→随迁） | 4 |
| 商业路由/装配 | `internal/router` commercial/plan-admin/account/platform 19+ 用例（经别名 shim） | 19+ |
| semantic 预算网关 | `semantic_model_test.go` 17 用例（留宿主，经 wrapper，覆盖 reserve/release/markDispatched/finish 编排 + BYOK 台账） | 17 |

**payment/usage 高风险面判定口径**：本节点对 payment 深层（providers、OrderStore.ConfirmPayment、BudgetService）**零代码改动**（纯移动的 handler 只改 import 与响应 helper 来源）；高风险等价性由上述 HTTP 形态用例 + 路由级用例 + 既有 `./internal/modules/commercial/...` 全量测试共同覆盖。差分文件须逐条列出每个用例在搬迁前后的 pass/fail 与关键输出摘录（响应码、body 形态、stub 收到的参数）。

**known-debt 处置**：`TestProvidersFromEnvRejectsPartialAlipay`（`internal/modules/commercial/payment/providers_env_test.go:75`，Pass A 台账在册 map 序断言债，义务归 B-commercial）：本节点不改 payment 包、不修该测试（修测试=行为变更，须独立 Ticket）；差分文件登记其现状（若偶发红，按台账口径记录为既有债而非本节点回归）。

## 8. Integration Brief（交付 IB1 的装配变更申请，T5 定稿）

`docs/architecture/passb/briefs/b1-commercial.md` 必含：

1. **shim 删除清单**（8 旧路径中现存 7 个 shim 文件 + commercial_task_budget.go 已删）：`internal/application/repository/model_usage.go`、`user_usage.go`、`internal/application/service/semantic_model_budget.go`、`usage_recorder.go`、`internal/handler/commercial.go`、`usage.go`、`payment_callbacks.go`。
2. **IB1 需切换的调用点**（集成工程师独占文件）：`internal/container/container.go` :213（NewUsageRepository）、:734（NewUsageHandler）、:867（NewCommercialHandler）、:1069（NewSemanticModelBudgetAdapter）、:2600（NewUsageRecorderService）改指 `internal/modules/commercial/{repository,handler,service}`；`internal/router/routes_commercial.go`、`routes_usage.go` 的 `handler.*` 引用改指 `commercialhandler`。
3. **B2 需切换的调用点**（不在 IB1，登记供 B2 计划消费）：knowledgebase.go :216/:235/:242、custom_agent.go :72/:87/:94/:119/:136 → `commercialrepo` 导出符号；semantic_model.go :99-134 五调用点 + `semanticBudgetFailure` → `commercialservice.SemanticModelBudgetAdapter.{Reserve,Release,MarkDispatched,Finish}` + `SemanticBudgetFailure`（参数经 `SemanticBudgetCall`）；semantic_model_test.go 的 seam 构造随 knowledge 侧迁移时改用 commercial 侧 seam。
4. **helper 族收口申请**：`appOK/appFail`（本节点副本 ↔ app_connector.go 原件，consumer：commercial/craft/appconnector）、`parseAnalyticsRange/analyticsIsDateOnly/analyticsRows/parseFilterTime`（本节点副本 ↔ analytics.go/knowledge.go 原件）——按 conventions §7.1「多属主重复 helper 族在 barrier 收口为单一实现」申请裁定归宿（建议 insights 届时导出或下沉平台层，由 IB1/IB3 架构审查定）。
5. **contracts.yaml consumers 路径回写申请**：`commercial.billing-access-decision`（consumer `internal/handler/commercial.go`）、`commercial.budget-adapter-construction`（symbol 与 characterization_tests 路径）、`commercial.usage-data-ownership` / `commercial.usage-recorder-service`（consumers 中 3 个旧路径）→ 新路径（barrier 独占回写，conventions §3）。
6. **workbench→commercial 5 条（exc-0093/0094/0101/0103/0104，remove_at ib4）与 appconnector→commercial 4 条（exc-0058–0061，remove_at ib2）例外**：其消费对象是 Pass A 已搬的 `internal/modules/commercial`（根/repository/service），本节点未改变这些包的冻结符号（contracts.yaml current 契约），IB1 冻结 concrete façade 后由各自 barrier 按期删除；本节点零删除（§9）。
7. **测试偏差登记**（§2.5）：model_usage_test.go / user_usage_test.go 留宿主的理由与 B2/b5 收口链。

## 9. 必须删除的 legacy/alias/例外（本节点口径）

| 项 | 状态 | 依据 |
|---|---|---|
| `internal/handler/commercial_task_budget.go` | **本节点删除**（内容随类型别名迁移，方法属 CommercialHandler） | ownership-matrix :1792（destination handler） |
| 7 条 alias_obligations 旧 import path（`internal/commercial`、`internal/payment`、`internal/usage`、`internal/infrastructure/{openmeter,commercialplatform}`、`internal/application/{repository,service}/commercial`） | **已不存在**（Pass A 集成 `10cc02f85` 删除；本节点基线 grep 全仓生产代码零引用）——验收时以命令复证并写入证据 | moves/commercial.yaml alias_obligations；B5 零债口径 |
| exception-ledger 中消费 commercial 的 10 条（exc-0002 ib3 / exc-0057 ib1 / exc-0058–0061 ib2 / exc-0093/0094/0101/0103/0104 ib4） | **本节点零删除**：remove_at 属主分别是 33-agentruntime/11-airesource/27-appconnector/40-workbench 各自 barrier；本节点不动这些消费方文件 | exception-ledger 逐条 remove_at |
| 7 个宿主过渡 shim | **本节点创建、IB1/B2 删除**（Brief §8.1/.3；B5 兜底验证零残留） | conventions §7.1 薄 shim 授权 + framework:41 零债验收 |

## 10. 独立验收标准（全部满足才算完成；审查者逐条核验）

1. `go build ./...`、`go test -count=1 ./internal/modules/commercial/...`、`make check-backend-architecture`、`make verify-module-moves` 在节点分支 HEAD 全部退出码 0（gates 原文，无替代命令）。
2. `git diff d57a2fa708...HEAD --name-only | sort` 与 §3 文件清单之差为空——零越权文件（conventions §1.2 第 4 条命令）。
3. 8 条 ownership-matrix（plan=12-commercial）destination 落位：目标路径存在对应实现、`git diff --summary` 对 `user_usage.go`、`usage_recorder.go`、`usage_recorder_test.go`、`usage_test.go`、4 个 handler 文件、2 个 T1 测试显示 rename（R100 或仅 import/包名差异）。
4. model_usage 绑定族 5 个导出符号 + `SemanticBudgetCall` + `Reserve/Release/MarkDispatched/Finish` + `SemanticBudgetFailure` 在 `internal/modules/commercial/{repository,service}` 存在且签名与本计划 §4 逐字一致（`go doc` 输出比对）。
5. 差分证据齐备：§7 用例矩阵每行有搬迁前后双跑记录（命令原文 + 退出码 + 关键输出）与等价结论；payment/usage 面逐用例比对结论「等价」；map-order 已知债有现状登记。
6. `internal/router/routes_commercial.go`、`routes_usage.go`、`internal/container/container.go`、`internal/bootstrap/**`、`go.mod`、`go.sum`、`migrations/` 在本节点 diff 中零出现（禁改纪律）。
7. knowledgebase.go / custom_agent.go / semantic_model.go / semantic_model_test.go / app_connector.go / analytics.go / knowledge.go 零改动（跨 owner 纪律）。
8. 633/23+23/58/537 计数不受影响：路由/worker/hook 注册文件零触碰（验收口径：本节点 diff 不含 router/container/migrations 任何文件；正式三方一致复核在 IB1）。
9. Integration Brief（§8 七项）与实施报告（conventions §1.2 命令包全量摘录）落盘；报告如实登记 §2.5 测试偏差与任何未完成项。
10. 提交序列恰好 5 个 commit（T1–T5），message 与 §6 各任务模板一致，无混合关注点提交。

## 11. 升级路径（conventions §5）

- 任一 gate 不可能通过时：不改断言、不删测试、不扩例外、不复制他模块业务实现、不伪造证据；在 `docs/plans/passb/reports/b1-commercial.md` 记录未过项/根因/复现命令/建议裁定，将 DAG 节点置 blocked 并等协调者裁定。
- 「上游门面尚未落地」类阻塞（如 IB1 门面形态未定）：引用 DAG 边与 F2 裁定（planned 门面不进 current 消费校验路径），本计划已按「不实现 module.go 门面代码、以 Integration Brief 申请 IB1 冻结」设计，预期不触发此类升级。
- 若实施中发现 `payment.Provider` 接口形状、`UsageStore` 构造方式与本计划 §4/§6 所述不符（计划依据 base SHA 静态读取），以真实代码为准调整 stub 写法并记录；若需改生产签名则属契约变更，升级处理。

## 12. 计划自检记录（撰写者已执行）

- **Spec 覆盖**：§5.13 职责（准入/预占/结算/计量/付款）与 8 文件一一对应（budget/reservation=semantic_model_budget、usage=user_usage+usage_recorder+usage handler、payment=payment_callbacks、admission/billing 门面=commercial.go+commercial_task_budget）；§11 B1 并行边界与 framework:92 一致；§12 Pass B 循环（特征测试→搬迁→差分→Brief→legacy 删除申请）映射为 T1→T5；§13 提交隔离（T1=test、T2–T4=move+shim、T5=docs）；§14.3 差分面=payment/usage 全覆盖。
- **无占位符**：全部文件路径、符号名、签名、行号、命令均来自本会话实读/实跑（base SHA `d57a2fa708`）；无 TBD/待定/虚构接口。唯二「实施时先读再写」项（§6 T1 payment.Provider stub、T3 UsageStore stub）已注明以真实代码为准的理由（避免计划层转录接口全貌失真），并给出用例断言目标。
- **类型一致**：§4 导出签名 ↔ §5 shim 转发签名 ↔ §2 现状签名三向一致（`budgetCallFrom` 给出 semanticCapability↔SemanticBudgetCall 字段一一映射）；类型别名保方法集；wrapper seam 与 semantic_model_test.go:511-515 构造字面量同形。
- **跨任务接口一致**：T2 产出 `commercialrepo` 5 符号 = T2 shim 消费 = §8.3 B2 消费清单；T3 产出 `commercialservice` 端口 = T3 wrapper 消费 = §8.3；T4 别名 = §2.4 禁改消费面全集；T1 测试文件在 T4 随迁后仍是同包同用例（差分前提）。
- **风险声明**：`release` 的 wrapper nil 分支语义近似推理（§5.3 末段）已标注为审查复核点；map-order 债不修的口径依据 Pass A 台账。
