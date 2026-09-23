# Pass B 子计划 27 — B2-AC App Connector 边界（7 legacy handlers：install/OAuth/action/sync 路由 + 别名核销）

> 实施方式：superpowers:executing-plans / subagent-driven-development，按任务逐个执行，一个任务一个 commit。
> 节点：`b2-appconnector`（DAG `docs/plans/passb/execution-dag.json`，execution_mode=parallel，role=work，depends_on=ib1）。
> 本计划由「计划撰写-b2-appconnector」于 2026-09-23 在 worktree `codex/passb-b2-appconnector`（起点 `8c45a8815`，ib1 登记提交）撰写；文中全部代码坐标、命令输出均为该 SHA 实读/实跑结果。

## 0. Spec 与事实源指针

| 事实源 | 位置 | 本计划取用的内容 |
|---|---|---|
| Spec | `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §5.10（App Connector 职责：应用目录、安装、OAuth、连接、授权、动作、同步、Open Connector）、§5.18（Agent 工具只调用公开动作接口，不读取连接表）、§11（B2 并行 + IB2 串行）、§12（Pass B 循环）、§13（提交隔离 M1–M5 与回滚）、§14.3（高风险差分：Tenant/RBAC） | 边界语义、差分义务、提交纪律 |
| Pass B 框架 | `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` :146（27 号计划=7 legacy handlers、install/OAuth/action/sync 路由、别名删除）、:149（与 Knowledge/AgentCatalog 并行的依据：B1 已冻结外部能力依赖）、:29（`_test.go` 随迁、host 不留转发业务声明）、:153-155（IB2 职责） | 任务边界与并行裁定 |
| 执行公约 | `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | §1 派发契约、§2 门禁、§3 禁改清单、§4 提交规范、§5 升级、§6 差分证据、§7 package-private 耦合与 helper 收口 |
| ownership-matrix | `.worktrees/passb-int/docs/architecture/passb/ownership-matrix.yaml` :1706-1746（plan=27-appconnector 的 7 行：destination 全部为 `internal/modules/appconnector/handler`，integration_owner=ib2，delete_barrier=ib2）、:2463-2543（5 条 alias 行，delete_barrier=ib2） | 精确 destination 与删除屏障 |
| contracts.yaml | 同目录 `contracts.yaml` :553-598（`appconnector.facade` :553 / `appconnector.lifecycle` :567 / `appconnector.routes` :583 / `appconnector.workers` :594，stability=frozen，本节点只读） | 冻结契约（workers=空集、routes=RegisterAppConnectorRoutes、lifecycle 5 挂点） |
| event-catalog | 同目录 `event-catalog.yaml` | `grep "appconnector\|app_connector\|AppConnector"` 零命中——本模块无版本化事件，本节点零事件面改动 |
| exception-ledger | 同目录 `exception-ledger.yaml` :350-373（exc-0058/0059/0060/0061，plan=27-appconnector，remove_at=ib2）、:170-175（exc-0028：agentruntime→appconnector，plan=32-agentruntime-tools，remove_at=ib3，**非本节点行**） | 例外现状与本节点裁定（§2.5、§8） |
| MOVE-MANIFEST | `docs/architecture/moves/appconnector.yaml` | 5 条 alias_obligations、7 条 legacy_files、importers、integration_points（本文件只读；迁移状态登记进报告/Brief，manifest 本体由 B5 台账收口） |
| DAG 裁定 | `execution-dag.json` 节点 `b2-appconnector`：required_contracts（Commercial 门面 3-4 处引用、AgentRuntime 引用裁定）、notes（夹具随迁：commercial(7)/agentruntime(2)）、gates、produced_artifacts | 契约前提、夹具随迁口径、门禁 argv |
| 先例计划 | `docs/plans/passb/12-commercial.md`（shim/别名/差分格式）、`docs/plans/passb/13-execution.md` EX-8 Step 4（自身例外无法节点内解除时「不删行、登记提案留 barrier」的先例） | 格式与裁定先例 |

### 前置条件（开工前逐条核验）

1. **b0 已 done、ib1 到达可派发状态**：撰写时实读 DAG——b0 `status=done head_sha=d57a2fa708c3fe… review=approved`；b1 四节点 done/approved（B1 产出止于计划文档，四个 merge 提交 `1900e038a`/no-op/`f66dfb188`/`ca38afb7e` 的提交说明自证「无生产代码变更」）；ib1 `status=in_progress`（`8c45a8815` 登记「装配切换因 B1 零实施上报缺位」）。本节点派发以协调者把 `b2-appconnector.status` 置 `in_progress` 并回填 `base_sha` 为准（conventions §9）。
   - 节点 notes 残留「BLOCKED（2026-09-23）：前置 b0 阻塞」为 b0 受阻期遗留文本（b0 现已 done/approved）；实施者以 DAG `status`/`base_sha` 字段为准，不因残留文本停工。
   - **Commercial 门面未落地的含义**（本计划 §2.5 裁定的前提）：B1-CM 零实施 ⇒ `internal/modules/commercial` 仍为 Pass A 布局、无根门面代码；appconnector→commercial 的 3 个生产文件引用（exc-0058..0061）按 conventions §5「上游门面尚未落地」处置——本节点不解除、不新增例外、不自行实现上游门面，登记提案留 IB2。
2. worktree `codex/passb-b2-appconnector` 起点等于派发时 `base_sha`（`git rev-parse HEAD` == 协调者回填值），工作树干净。
3. 基线门禁绿（撰写者于 2026-09-23 在起点 `8c45a8815` 实跑，实施者须在自身 base SHA 复跑并记录）：
   - `go build ./...` → 退出码 0；
   - `go test ./internal/modules/appconnector/... -count=1` → 5 包全 ok（root/connectorcontrol/openconnector/repository/service）；
   - `go test ./internal/handler/ -run 'TestAppConnectionOAuthFlow|TestOC|TestDecodeOCPrepare' -count=1` → ok（21 用例）；
   - `make check-backend-architecture` → `total=633 | redis=23 lite=23 | hooks=58 | modules=16`、`OK (0 violations)`；
   - `make verify-module-moves` → `OK (16 manifests verified)`。
4. 与并行节点文件不相交：20-knowledge-program（`internal/modules/knowledge/**`）、25-agentcatalog-program、26-datasource 与本计划 §3 文件集无交集（framework:149；知识/目录/datasource 的宿主 legacy 文件与本节点 7 个 `app_connector*.go` 无重叠）。

## 1. 目标与范围

**目标**：把困在水平宿主包 `internal/handler` 的 7 个 app_connector legacy 文件迁入 `internal/modules/appconnector/handler`（ownership-matrix 冻结 destination），全部路由/容器装配经宿主过渡 shim 保持零改动编译，Tenant/RBAC 与 install/OAuth/action/sync HTTP 行为差分等价证据落盘，5 条别名行与 4 条例外行的核销状态向 IB2 交付登记，全程节点四门禁绿色。

**范围内（owned_files = manifest legacy_files 7 条 + `internal/modules/appconnector/**`）**：

| # | legacy 文件（`internal/handler/`） | destination（ownership-matrix :1706-1746 冻结） | 行数 |
|---|---|---|---|
| 1 | `app_connector.go` | `internal/modules/appconnector/handler/app_connector.go` | 81 |
| 2 | `app_connector_action.go` | `…/handler/app_connector_action.go` | 367 |
| 3 | `app_connector_connection.go` | `…/handler/app_connector_connection.go` | 208 |
| 4 | `app_connector_installation.go` | `…/handler/app_connector_installation.go` | 207 |
| 5 | `app_connector_oauth.go` | `…/handler/app_connector_oauth.go` | 254 |
| 6 | `app_connector_oc.go` | `…/handler/app_connector_oc.go` | 379 |
| 7 | `app_connector_sync.go` | `…/handler/app_connector_sync.go` | 128 |

随迁测试（framework:29）：`internal/handler/app_connector_oauth_test.go`（159 行，1 用例）、`internal/handler/app_connector_oc_test.go`（880 行，20 用例）→ 同目录随迁。

加上：`internal/modules/appconnector/handler/handler_helpers.go`（新增，ErrMissingTenantScope 本地副本，§4.2）、`internal/handler/app_connector.go` 旧路径**过渡薄 shim**（本节点自身产出，重写文件，删除义务登记进 Brief）、T1 新建的宿主特征化测试文件（T2 随迁）、`internal/modules/appconnector/legacy/README.md`（迁移状态回填）、`docs/architecture/evidence/passb/b2-appconnector.md`、`docs/plans/passb/reports/b2-appconnector.md`、`docs/architecture/passb/briefs/b2-appconnector.md`。

**范围外（一律不动）**：

- `internal/router/router.go`（:131-134 RouterParams 字段、:413-417 调用点）、`internal/router/routes_app_connectors.go`、`internal/container/container.go`（:250、:933-936、:980-999）、`internal/container/open_connector.go`（:605-700）、`internal/container/*_test.go`、`internal/bootstrap/**`、`go.mod`、`go.sum`、`migrations/`（conventions §3 集成工程师独占/禁改）。
- 跨 owner 宿主文件（shim 维持其编译，零改动）：`internal/handler/commercial_task_budget.go`（12-commercial 属主，:27 起消费 appFail）、`internal/handler/craft_model_gateway.go`（41-craft 属主，:270/:282/:296/:309/:318/:330/:334/:356/:359/:388 消费 appFail/appTenantScope/appOK）、`internal/handler/commercial.go`（12-commercial 属主，:34 定义 ErrMissingTenantScope）、`internal/handler/analytics.go` 等。
- `internal/application/repository/mcp_oauth.go`（11-airesource 属主；本节点消费其导出面，见 §2.2，不改其文件）。
- `internal/modules/appconnector/**` 内 Pass A 已搬的既有生产文件（adapter.go、service/、repository/、openconnector/、connectorcontrol/ 等）——本节点不改其行为；3 个文件的 commercial 引用按 §2.5 裁定原样保留（活跃例外覆盖）。
- `docs/architecture/passb/{ownership-matrix,contracts,event-catalog,exception-ledger}.yaml`（b0 建、barrier 回写，conventions §3）、`execution-dag.json`（协调者独占）、`docs/architecture/moves/appconnector.yaml`（B5 台账收口）。
- `internal/modules/appconnector/module.go` 零逻辑骨架（门面实现归 IB2，13-execution §2 先例：本节点不写门面）；`cmd/connector-control`、`internal/agent`、agentruntime 全部文件。

## 2. 现状断链面（真实代码证据，全部在起点 SHA 实测）

### 2.1 宿主包与装配层对本组符号的消费面（shim 必须覆盖的全集）

| 消费方 | 符号 | 位置 |
|---|---|---|
| `internal/handler/commercial_task_budget.go`（commercial 属主） | `appFail` | :27 起（多处） |
| `internal/handler/craft_model_gateway.go`（craft 属主） | `appFail` | :270、:282、:309、:318、:330、:334、:356、:388 |
| 同上 | `appTenantScope` | :296 |
| 同上 | `appOK` | :359 |
| `internal/container/container.go` | `handler.NewAppInstallationHandler/NewAppConnectionHandler/NewAppSyncHandler/NewAppActionHandler`（dig Provide） | :933-936 |
| `internal/container/container.go` | `handler.AppActionHandler`（3 个 Invoke 注入块）、`handler.AppConnectionHandler` | :981-987、:995 |
| `internal/container/container.go` | `handler.DefaultAppOAuthProviderConfigs`、`handler.AppOAuthProviderConfig` | :996、:999 |
| `internal/container/open_connector_test.go` | `handler.NewApp{Installation,Connection,Sync,Action}Handler` | :386-389 |
| `internal/router/router.go` | `RouterParams` 4 字段类型 `*handler.App{…}Handler` | :28、:131-134 |
| `internal/router/router.go` | `RegisterAppConnectorRoutes(v1, params.App{…}Handler…)` | :413-417 |
| `internal/router/routes_app_connectors.go` | 4 个 handler 参数类型 | :22-25 |

宿主包内无其他消费者（grep `internal/handler/*.go` 实测）：`newAppID`（app_connector.go:36）、`appRequireWriteCapability`（:60）、`OCPrepareInput`（oc.go:38）、`DecodeOCPrepare`（oc.go:51，仅 `expert.go:174` 注释提及）、`appActionStates`/`appActionRisks`（action.go:61/:73）均零外部消费——**不进 shim**（shim 最小化，未导出零消费符号不留转发）。

### 2.2 本组文件对宿主其他属主符号的依赖（搬迁后须就地解决）

| 依赖符号 | 定义处（属主） | 消费文件 | 搬迁处置 |
|---|---|---|---|
| `ErrMissingTenantScope` | `internal/handler/commercial.go:34`（12-commercial） | `app_connector.go:49`、`app_connector_action.go:69` | 模块内新文件 `handler_helpers.go` 建本地副本 `errors.New("missing_tenant_scope")`（消息逐字一致，响应字节不变）；宿主侧 shim 的 appTenantScope 副本继续用宿主原符号，**不新增宿主副本**（§4.2） |
| `repocommercialmcp.MCPOAuthBindingStore`、`NewMCPOAuthBindingStore` | `internal/application/repository/mcp_oauth.go`（11-airesource 属主 legacy 文件） | `app_connector_connection.go:25/:35` | 随文件迁入模块 handler 包，import 路径不变（`internal/application/repository` 为宿主横向包；architectureguard forbidden-import 只扫 `internal/modules/**` 文件的 `internal/modules/<他模块>` 导入，host→module 方向与 module→host 方向均不在该检查范围，check.go:1175-1220 WalkDir modRoot 实读） |
| `mcprepo.ErrOAuthBindingInvalid`、`ErrOAuthBindingActorNotMember` | 同上 | `app_connector_oauth.go:241/:243` | 同上，零改动 |

package-private 耦合：`execution-dag.json` 顶层 `package_private_couplings.pairs`（29 对）grep `appconnector` 零命中——本节点无 §7 未导出互耦义务。

**Go import 环核验**：搬迁后 `modules/appconnector/handler → application/repository → modules/appconnector`（root）为 A→B→C 无环（`mcp_oauth.go` 导入模块根包而非 handler 包，grep 实测）。

### 2.3 测试夹具现状（DAG notes「commercial(7)/agentruntime(2)」实测）

模块内测试对跨模块包的引用（全部在 `_test.go`，architectureguard 明确跳过 `_test.go`（check.go :1186 实读），零例外义务；属 DAG notes 所指「架构事实」）：

- **commercial 7 行**：`http_policy_test.go:16`、`service/appconnector/oc_integration_test.go:70`、`service/appconnector/oc_recovery_test.go:20-22`（3 行，含 `repocommercial`/`commsvc` 别名）、`service/appconnector/action_test.go:15`、`service/appconnector/oc_limiter_test.go:14`；
- **agentruntime 2 行**：`service/appconnector/oc_tool_binding_test.go:13-14`（消费 `agentruntime.OCActionWaitError` :339 与 `agent/tools`，均已是 `internal/modules/agentruntime/**` 真实路径而非别名，零改动）。

宿主测试 2 文件依赖（随迁可行性）：`app_connector_oauth_test.go` import context/json/http/sqlite/gorm/types/gin，自包含；`app_connector_oc_test.go` 另 import `internal/middleware`（:15）与 `internal/modules/commercial`（:19，复刻 gate 场景），in-memory sqlite（:25/:223）+ `AutoMigrate(&appconnectorrepo.InstallationRow{}, &appconnectorrepo.ConnectionRow{}, …)`（:32）+ `&types.MCPOAuthToken{}`（:39）。两文件对宿主其他文件符号（`ErrMissingTenantScope`/`parseAnalyticsRange`/`commercialTenantScope` 等）grep 零命中——**可纯随迁**，包名同为 `handler`，import 零变化。

### 2.4 别名现状：5 条 alias 行已在 Pass A 集成期物理删除（本计划关键事实）

- 集成提交 `b0ef8895a`（2026-09-21，「refactor(integration): switch appconnector composition to module paths and drop pass-a aliases」）删除了全部 5 个 alias 文件（`internal/appconnector/alias.go` 151 行、`internal/appconnector/openconnector/alias.go` 29 行、`internal/application/repository/appconnector/alias.go` 52 行、`internal/application/service/appconnector/alias.go` 122 行、`internal/connectorcontrol/alias.go` 62 行，共 418 行）并把 `internal/container/container.go` 4 行装配切到模块路径；`git merge-base --is-ancestor b0ef8895a HEAD` 实测为真。
- 起点 SHA 实测：`git ls-tree HEAD internal/appconnector internal/connectorcontrol internal/application/repository/appconnector internal/application/service/appconnector` 全空；`grep -rn "WeKnora/internal/appconnector\"|WeKnora/internal/connectorcontrol\"|application/repository/appconnector\"|application/service/appconnector\"" --include='*.go' internal/ cmd/` 零命中；`cmd/connector-control/main.go:39-40` 已直接 import 模块路径。
- **结论**：ownership-matrix :2463-2543 的 5 条 alias 行（plan=27，delete_barrier=ib2）与 moves/appconnector.yaml 的 5 条 alias_obligations 在当前树上已是**空义务**（路径不存在、零 importer）。本节点不重建、不改治理 YAML；以命令复证 + 证据落盘 + Brief 登记核销申请（IB2 在 guard 口径下收口回写），B5 以「零残留」终验。

### 2.5 例外现状与裁定（exc-0058..0061：本节点不删行，登记提案留 IB2）

| 例外 | from（本模块文件） | to | remove_at |
|---|---|---|---|
| exc-0058（:350） | `internal/modules/appconnector/adapter.go` | `internal/modules/commercial` | ib2 |
| exc-0059 | `internal/modules/appconnector/service/appconnector/action.go` | `internal/modules/commercial` | ib2 |
| exc-0060 | `internal/modules/appconnector/service/appconnector/oc_recovery.go` | `internal/modules/commercial` | ib2 |
| exc-0061 | `internal/modules/appconnector/service/appconnector/oc_recovery.go` | `internal/modules/commercial/service/commercial`（消费 `commsvc.ErrGateReservationUnknown`，:339） | ib2 |

消费符号全量（grep 实测）：`commercial.ExecutionGate`（接口，action.go:163 字段、:193 构造器参数）、`commercial.BudgetRequest`（adapter.go:63/:76、action.go:396）、`commercial.UsageFact`（adapter.go:105、action.go:461/:518、oc_recovery.go:310-322）、`commercial.Credits`（action.go:167）、哨兵/常量 `ErrInsufficientBudgetGate`（adapter.go:100）、`FundingPlatform`/`ServiceConnector`/`DimensionConnector`/`UsageStatusFinal`。

**裁定**：这些引用的解除需要「appconnector 消费方端口 + commercial 类型适配器」，而适配器唯一合法落点是宿主装配层（`internal/container`，集成工程师独占）——模块树内任何文件 import commercial 都触发 forbidden-import（check.go :1203-1213：`target != owner` 即违规，含根包），且 conventions §5 禁止新增例外、禁止自行实现上游门面；B1-CM 零实施（前置条件 1）使商业门面在当前树上不存在。按 **13-execution EX-8 Step 4 先例**（「不删行、不改 guard，登记提案留 barrier 执行」）：本节点原样保留 3 文件的 4 条例外覆盖引用，在 Integration Brief 登记解除提案（消费方端口形态 + IB2 适配点），remove_at=ib2 的期限裁决权归 IB2（按提案执行则列入 IB2 串行适配清单，或裁定改期/升级串行契约任务）。conventions §3「各属主节点删除自己的行」的前提是引用已合法消除——此处不满足，故不删。

`internal/modules/agentruntime/agent/tools/app_connector.go`（exc-0028，agentruntime 消费**本模块**根包）方向相反、属主 32-agentruntime-tools：本节点保证模块根包现有导出面零变化，该消费方在 B3 前持续编译。

## 3. 目标包结构与写入所有权

```
internal/modules/appconnector/
├── handler/                                  # 新包 handler（ownership-matrix 冻结 destination）
│   ├── app_connector.go                      # git mv（helpers：appOK/appFail/newAppID/appTenantScope/appRequireWriteCapability）
│   ├── app_connector_action.go               # git mv（AppActionHandler 全部方法）
│   ├── app_connector_connection.go           # git mv（AppConnectionHandler 全部方法）
│   ├── app_connector_installation.go         # git mv（AppInstallationHandler 全部方法）
│   ├── app_connector_oauth.go                # git mv（OAuth provider 配置/state 兑换/callback）
│   ├── app_connector_oc.go                   # git mv（OC catalog/attempt/prepare + OCPrepareInput）
│   ├── app_connector_sync.go                 # git mv（AppSyncHandler + rowStore 适配器）
│   ├── handler_helpers.go                    # 新增：ErrMissingTenantScope 本地副本（§4.2）
│   ├── app_connector_oauth_test.go           # git mv 随迁
│   ├── app_connector_oc_test.go              # git mv 随迁
│   └── app_connector_lifecycle_test.go       # T1 新建于宿主，T2 git mv 随迁
└── legacy/README.md                          # 回填：7 行标记已迁（模块内文件，写入权归本节点）
```

宿主旧路径（7 文件中 6 个随 `git mv` 消失；唯一保留路径重写为 shim）：

```
internal/handler/app_connector.go            # 重写为过渡 shim（§4.1：5 类型别名 + 5 转发 + 3 helper 副本）
internal/handler/app_connector_{action,connection,installation,oauth,oc,sync}.go   # git mv 后不存在
```

治理/证据产出（本节点写）：

```
docs/architecture/evidence/passb/b2-appconnector.md   # 差分证据（§6）+ 别名/例外核销证据（§2.4/§2.5）
docs/plans/passb/reports/b2-appconnector.md           # 实施报告（conventions §1.2 命令包）
docs/architecture/passb/briefs/b2-appconnector.md     # Integration Brief（交付 IB2）
```

**写入所有权自检**：上述清单 ⊆ 节点 owned_files（manifest legacy_files 7 条 + `internal/modules/appconnector/**` + 节点自身产出新文件）。禁改清单（§1 范围外）零触碰。

## 4. 迁移设计（真实签名，不发明）

### 4.1 宿主过渡 shim（`internal/handler/app_connector.go` 重写，全部 no-logic forwarding）

```go
package handler

import (
	"net/http"

	appconnectorhandler "github.com/Tencent/WeKnora/internal/modules/appconnector/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Pass B (27-appconnector) 过渡 shim：实现已迁 internal/modules/appconnector/handler。
// 消费方：container.go:933-936/:980-999、router.go:131-134/:413-417、routes_app_connectors.go:22-25、
// open_connector_test.go:386-389（类型别名+构造器转发）；commercial_task_budget.go（12-commercial）
// 与 craft_model_gateway.go（41-craft）（helper 副本）。
// shim 删除义务见 docs/architecture/passb/briefs/b2-appconnector.md（IB2 切换后删除）。

type (
	AppInstallationHandler = appconnectorhandler.AppInstallationHandler
	AppConnectionHandler   = appconnectorhandler.AppConnectionHandler
	AppSyncHandler         = appconnectorhandler.AppSyncHandler
	AppActionHandler       = appconnectorhandler.AppActionHandler
	AppOAuthProviderConfig = appconnectorhandler.AppOAuthProviderConfig
)

func NewAppInstallationHandler(db *gorm.DB) *AppInstallationHandler {
	return appconnectorhandler.NewAppInstallationHandler(db)
}
func NewAppConnectionHandler(db *gorm.DB) *AppConnectionHandler {
	return appconnectorhandler.NewAppConnectionHandler(db)
}
func NewAppSyncHandler(db *gorm.DB) *AppSyncHandler {
	return appconnectorhandler.NewAppSyncHandler(db)
}
func NewAppActionHandler(db *gorm.DB) *AppActionHandler {
	return appconnectorhandler.NewAppActionHandler(db)
}

func DefaultAppOAuthProviderConfigs() map[string]AppOAuthProviderConfig {
	return appconnectorhandler.DefaultAppOAuthProviderConfigs()
}

// ---- 宿主 helper 副本（conventions §7.1 多属主重复 helper 族，IB2 收口裁定）----
// 与模块内原件（…/handler/app_connector.go:28-55）逐字等价；消费方：
// commercial_task_budget.go:27+（appFail）、craft_model_gateway.go:270-388（appOK/appFail/appTenantScope）。
// ErrMissingTenantScope 沿用宿主 commercial.go:34 现有定义，未新增副本。
func appOK(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"success": true, "data": data})
}
func appFail(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"success": false, "error": gin.H{"code": code, "message": message}})
}
func appTenantScope(c *gin.Context) (uint64, string, string, bool) {
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		appFail(c, http.StatusForbidden, "MISSING_TENANT_SCOPE", ErrMissingTenantScope.Error())
		return 0, "", "", false
	}
	role := string(types.TenantRoleFromContext(c.Request.Context()))
	userID, _ := types.UserIDFromContext(c.Request.Context())
	return tenantID, role, userID, true
}
```

要点：

- **签名冻结**：4 个构造器签名与 `NewAppInstallationHandler(db *gorm.DB)` 等逐字一致（app_connector_installation.go:21、connection.go:31、sync.go:21、action.go:35），dig Provide（container.go:933-936）经别名解析到同一类型，装配零变化。
- **纯移动可证**：7 个生产文件与 2 个既有测试文件 `git mv` 后 `git diff --summary` 显示 rename（R100；模块内 helper 副本只在 `handler_helpers.go`，moved 文件零函数体改动）。
- **错误值语义**：模块内 `appTenantScope`/`appRequireWriteCapability` 用 `handler_helpers.go` 的本地哨兵（消息 `"missing_tenant_scope"` 与 commercial.go:34 逐字一致 ⇒ 403 响应体 `{"code":"MISSING_TENANT_SCOPE","message":"missing_tenant_scope"}` 字节不变）；全仓无 `errors.Is` 跨这两值比较（grep 实测两值只经 `.Error()` 进响应），副本不引入可观察差异。宿主 shim 的 `appTenantScope` 用宿主原哨兵。
- **无环**：宿主 shim → 模块 handler 包；模块 handler 包 → `internal/application/repository`（mcp_oauth 族）→ 模块根包；宿主 commercial.go 不被模块 handler 包导入（§2.2 表）。

### 4.2 `internal/modules/appconnector/handler/handler_helpers.go`（新增，唯一模块侧新生产文件）

```go
package handler

import "errors"

// ErrMissingTenantScope 是宿主 commercial.go:34 同名哨兵的模块内副本（Pass B 27-appconnector）：
// 消息逐字一致，供 appTenantScope/appRequireWriteCapability 的 403 MISSING_TENANT_SCOPE 响应使用。
// 与宿主原件及 12-commercial 未来 modules/commercial/handler 副本的收口裁定登记于
// docs/architecture/passb/briefs/b2-appconnector.md，由 IB2 收口为单一实现。
var ErrMissingTenantScope = errors.New("missing_tenant_scope")
```

（7 个 moved 文件引用 `ErrMissingTenantScope` 的两处调用点 app_connector.go:49、app_connector_action.go:69 零改动——同包可见。）

### 4.3 路由与生命周期不变式（本节点零装配改动的外部契约）

- `RegisterAppConnectorRoutes`（routes_app_connectors.go:20）注册 17 条路由：installations 组 4（GET/POST `""`、POST `/:id/upgrade`、POST `/:id/disable`）+ GET `/apps/catalog` + connections 组 3（GET/POST `""`、POST `/:id/revoke`）+ POST `/apps/connections/:id/authorization-attempts` + GET `/apps/authorization-attempts/:id` + GET `/apps/connections/oauth/callback`（公开 leg，无 bearer）+ GET `/apps/datasources/:id/sync-status` + POST `/apps/oc/actions/prepare` + actions 组 4（POST `/prepare`、GET `/:id`、POST `/:id/approve`、POST `/:id/execute`）。本节点 diff 不含 router/container/bootstrap ⇒ contracts.yaml `appconnector.routes`（consumers: routes_app_connectors.go）与 `appconnector.lifecycle`（container.go 5 挂点 :980/:983/:986/:989/:994，含 `startOCRecoveryRunner` @ open_connector.go:683）的注册面零变化，633/23+23/58 计数不变（正式三方复核在 IB2）。
- `/api/v1/apps` 面不经 API-key authorizer（routes_app_connectors.go:9-12 注释契约：X-API-Key 一律 403 默认拒绝），锚定用例 `TestOCProductRoutesDefaultDenyAPIKeys`（oc_test）随迁保留。

## 5. 任务分解（业务完整、可独立审阅；严格按序执行）

### T1（B2-AC.1）— 特征化基线：补齐 installation/connection/sync/action 缺失测试并冻结基线

**文件**：新增 `internal/handler/app_connector_lifecycle_test.go`（package handler；此时在宿主包，T2 随迁）。仅测试文件，零生产改动。

**步骤**：
1. 按既有 oc_test 模式（in-memory sqlite `gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"))` + `AutoMigrate(&appconnectorrepo.InstallationRow{}, &appconnectorrepo.ConnectionRow{})`，sync 用例按 `app_connector_sync.go:50-67` 的裸表查询自建 `data_sources`（id/status/tenant_id 列）与 `app_datasource_bindings` DDL 或 `AutoMigrate(&appconnector.StoredSyncBinding{})`）构造 `NewAppInstallationHandler(db)`/`NewAppConnectionHandler(db)`/`NewAppSyncHandler(db)`/`NewAppActionHandler(db)` 与 gin test router，挂到 `RegisterAppConnectorRoutes` 同形分组。写以下用例（断言目标全部取自已读实现）：
   - `TestAppInstallationLifecycleAndWriteGate`：owner 角色 create→list→upgrade→disable 全链 200 且响应视图含 `id/app_key/version/state/scopes`（installation.go:37-46）；member 角色写操作 403 `MEMBER_MUST_REQUEST_INSTALLATION`（installation.go:29-33 逐字）；跨租户 id 与不存在 id 同为 404（app_connector.go:20-26 不变式 + installation.go:98 by-tenant 查询）。
   - `TestAppConnectionCreateBranches`：缺字段 400 `INVALID_REQUEST`（connection.go:118-127）；installation 不存在 404 `INSTALLATION_NOT_FOUND`（:132）；版本不匹配 409 `VERSION_CONFLICT`（:140）；已知 app 未配置 provider 501 `OAUTH_NOT_CONFIGURED`（:148）；未知 app 400 `UNKNOWN_APP`（:151）；成功流 201 且 body 含 `authorization_state/authorize_url/expires_at/installation_id/kind`（:158-164）。
   - `TestAppSyncStatusThreeStates`：数据源不存在/跨租户 404 `DATASOURCE_NOT_FOUND`（sync.go:57）；存在但无绑定行 200 且 `binding=null`、`requires_reauthorization=false`、`state=<ds.status>`（:75）；有绑定行时 200 且 binding 视图含 `installation_id/connection_id/auth_version`，需重授权时 `pause_reason="permission"`（:103-116）。
   - `TestAppActionPipelineUnwiredFailsClosed`：未调 `SetActionService` 时 POST `/apps/actions/prepare` 501 `ACTION_PIPELINE_NOT_CONFIGURED`（action.go:165/:230/:280）。
2. 跑新测试至全绿（锚定旧实现；失败即修测试不改生产码）。
3. 冻结全量基线输出（§6 命令集逐条 `-v` 运行，退出码与输出存入 evidence 文件草稿）。

**命令与预期**：
```bash
go build ./...                                                    # 退出码 0
go test ./internal/handler/ -run 'TestAppInstallationLifecycle|TestAppConnectionCreate|TestAppSyncStatus|TestAppActionPipeline' -count=1 -v   # 新用例全部 PASS
go test ./internal/handler/ -run 'TestAppConnectionOAuthFlow|TestOC|TestDecodeOCPrepare' -count=1 -v   # 既有 21 用例 PASS
go test ./internal/modules/appconnector/... -count=1              # 5 包 PASS
```

**Commit**：`test(passb): characterize app installation connection sync handlers before move`
**回滚边界**：纯增测试，`git revert` 单 commit 回滚，不影响任何生产路径。

### T2（B2-AC.2）— handler 层搬迁：7+2 文件 `git mv` + ErrMissingTenantScope 副本 + 宿主 shim

**文件**：`git mv internal/handler/app_connector{,_action,_connection,_installation,_oauth,_oc,_sync}.go internal/modules/appconnector/handler/`；同批 `git mv` 两个既有测试文件与 T1 的 `app_connector_lifecycle_test.go`；新增 `internal/modules/appconnector/handler/handler_helpers.go`（§4.2）；重写 `internal/handler/app_connector.go` 为 §4.1 shim；更新 `internal/modules/appconnector/legacy/README.md`（7 行标记已迁 + 目标包 + commit SHA，格式沿用现表）。

**步骤**：
1. `git mv` 10 个文件（7 生产 + 3 测试），零内容改动；`git diff --summary` 确认全部 rename（R100）。
2. 新增 `handler_helpers.go`（§4.2 全文）。
3. 重写宿主 `internal/handler/app_connector.go` 为 §4.1 shim（原文 81 行实现全部随 mv 迁出，shim 为新内容——非 rename）。
4. 回填 `legacy/README.md` 迁移状态。
5. 全量回归：§6 命令集 + conventions §1.2 命令包前两条。

**命令与预期**：
```bash
go build ./...                                             # 0（router/container/宿主经 shim 编译）
go test ./internal/modules/appconnector/handler/ -count=1 -v   # 21 既有 + T1 新用例全部 PASS（同包同用例）
go test ./internal/handler/ -count=1                       # 宿主包其余测试零回归（经 shim）
go test ./internal/container/ -run 'TestOCProductRoutes|TestNewOCArmedActionService' -count=1   # 装配面 PASS（open_connector_test.go:386-389 经构造器转发）
go test ./internal/modules/appconnector/... -count=1       # 5 包 PASS
make check-backend-architecture                            # 0（shim 在 legacy 登记路径上；forbidden-import 不扫宿主包与 _test.go）
make verify-module-moves                                   # 0
git diff --summary HEAD~1                                  # 10 个 rename + 1 个新文件 + 1 个重写（app_connector.go）+ legacy/README
```

**Commit**：`refactor(appconnector): move app connector handlers into module with alias shims`
**回滚边界**：revert 本 commit 即回宿主实现；路由/容器注册文件零改动，633 路由计数不变，无数据/迁移影响。

### T3（B2-AC.3）— 差分复跑比对 + 别名/例外核销证据，evidence 定稿

**文件**：`docs/architecture/evidence/passb/b2-appconnector.md`。

**步骤**：
1. 差分复跑：§6 用例矩阵逐行，T1 基线输出 vs T2 后同命令输出，逐用例等价结论（响应码、body code、stub 收到参数）。
2. 别名核销证据（§2.4 三条命令复跑：`git ls-tree` 5 路径全空、旧 import path 全仓 grep 零命中、`git show b0ef8895a --stat` 摘录），结论「5 条 alias 行 = 空义务」。
3. 例外核销登记（§2.5）：exc-0058..0061 现状（3 文件 4 行引用原样在位）+ 解除提案 + IB2 期限裁决申请；exc-0028 消费方（agentruntime 工具）零受影响声明（模块根包导出面零变化的 grep 对比：T2 前后 `go doc github.com/Tencent/WeKnora/internal/modules/appconnector` 输出 diff 为空）。
4. 计数核验：`make check-backend-architecture` 输出（633/23+23/58/16）与本节点 diff 不含 router/container/bootstrap 的 `git diff --name-only` 证明。

**命令与预期**：§6 命令集逐条 + `git diff <base_sha>...HEAD --name-only | sort`（router/container/bootstrap/go.mod/go.sum/migrations 零出现）。全部输出摘录进 evidence 文件。

**Commit**：`docs(passb): record b2-appconnector differential and alias-expiry evidence`
**回滚边界**：纯文档，revert 即回。

### T4（B2-AC.4）— Integration Brief + 实施报告 + 节点门禁收口

**文件**：`docs/architecture/passb/briefs/b2-appconnector.md`、`docs/plans/passb/reports/b2-appconnector.md`。

**步骤**：
1. Integration Brief（§7 七项）。
2. 实施报告：conventions §1.2 四条命令包逐条执行并摘录（命令原文 + 退出码 + 关键输出）；变更文件 vs owned_files 逐条核对（差集 = ∅）；未完成项如实列出（§2.5 例外提案、宿主 helper 副本收口义务、mcp_oauth 族对 airesource 的过渡依赖）。
3. 节点 gates 原文逐条执行（DAG `b2-appconnector.gates`）：`go build ./...`、`go test -count=1 ./internal/modules/appconnector/...`、`make check-backend-architecture`、`make verify-module-moves`。
4. DAG `task_ids`/`head_sha` 由协调者回填，本任务不改 `execution-dag.json`（conventions §9）。

**命令与预期**：
```bash
PASSB_BASE_SHA=$(git merge-base origin/main HEAD)    # 或协调者派发时给定的基线
git diff --stat "$PASSB_BASE_SHA"...HEAD             # 变更清单 ⊆ §3 所列文件
go build ./...                                        # 0
go test -count=1 ./internal/modules/appconnector/...  # 5 包 PASS
make check-backend-architecture                       # 0
make verify-module-moves                              # 0
git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort  # 与 owned_files 求差集 = ∅
```

**Commit**：`docs(passb): record b2-appconnector integration brief and node report`

## 6. 高风险差分证据要求（conventions §6：Tenant/RBAC 面；framework:40）

**方法**：T1 冻结基线输出 → T2 搬迁 → 同用例复跑 → 逐用例等价比对，证据写入 `docs/architecture/evidence/passb/b2-appconnector.md`。

**用例矩阵**（搬迁前后同集合同命令）：

| 面 | 用例来源 | 基线位置 | 复跑位置 | 数量 |
|---|---|---|---|---|
| OAuth 连接流（state 签发/兑换、公开 callback leg） | `app_connector_oauth_test.go`（随迁同文件） | 宿主 | 模块 | 1 |
| connection create 分支族（400/404/409/501/201） | T1 `TestAppConnectionCreateBranches`（随迁） | 宿主 | 模块 | 6 断言组 |
| installation 生命周期 + 写门 + 跨租户 404 | T1 `TestAppInstallationLifecycleAndWriteGate`（随迁） | 宿主 | 模块 | 3 断言组 |
| sync status 三态 | T1 `TestAppSyncStatusThreeStates`（随迁） | 宿主 | 模块 | 3 |
| 原生 action 未接线 fail-closed | T1 `TestAppActionPipelineUnwiredFailsClosed`（随迁） | 宿主 | 模块 | 1 |
| OC 全链路（catalog 可达性/租户隔离/成员可读、attempt 生命周期/API-key handoff/门禁错误、prepare 端点族、execute 4xx/409/429/happy、跨租户不可见、API-key 默认拒绝） | `app_connector_oc_test.go`（随迁同文件） | 宿主 | 模块 | 20 |
| U05 计量副作用（intent→budget→call→settle 编排、预算拒绝不派发） | `internal/modules/appconnector/http_policy_test.go` `TestActionExecuteWrapsIntentBudgetInner`/`TestActionIntentDenialBlocksEverything`/`TestActionBudgetDenialNeverDispatches`（:323-372，模块内不动） | 模块 | 同左 | 3 |
| 路由注册不冲突/装配 | `internal/container/open_connector_test.go` `TestOCProductRoutesRegisterWithoutConflict` 等（宿主不动，经 shim 编译） | 宿主 | 同左 | 3+ |
| 模块既有 5 包回归 | `go test ./internal/modules/appconnector/... -count=1` | 起点 | HEAD | 全量 |

**Tenant/RBAC 判定口径**：本节点对写门（`appRequireWriteCapability` + `CanInstallInstallation`/`CanManageConnections`/`CanManageActions` 谓词）、tenant-from-context 读取、by-tenant 查找与 404 防枚举**零代码改动**（纯移动 + shim 转发）；等价性由上述 HTTP 形态用例（403 写门族、跨租户 404 族、MISSING_TENANT_SCOPE 响应字节）逐用例证明。差分文件逐条列出每个用例搬迁前后 pass/fail 与关键输出摘录。

## 7. Integration Brief（交付 IB2 的装配变更申请，T4 定稿）

`docs/architecture/passb/briefs/b2-appconnector.md` 必含：

1. **shim 删除清单**：`internal/handler/app_connector.go`（唯一宿主残件；`appOK`/`appFail`/`appTenantScope` 副本 + 5 类型别名 + 5 转发）。
2. **IB2 需切换的调用点**（集成工程师独占文件）：`internal/router/router.go` :131-134（RouterParams 字段类型）与 :413-417、`internal/router/routes_app_connectors.go` :22-25（改指 `appconnectorhandler`）；`internal/container/container.go` :933-936（4 个 Provide）、:981-999（Invoke 块 + `DefaultAppOAuthProviderConfigs`/`AppOAuthProviderConfig`）；`internal/container/open_connector_test.go` :386-389。切换后 shim 与宿主 helper 副本删除。
3. **宿主 helper 副本收口申请**（conventions §7.1）：`appOK`/`appFail`/`appTenantScope` 现存三处——模块原件（本节点）、宿主 shim 副本（本节点）、12-commercial 的 `handler_helpers.go` 副本（其计划 §4.5）；`ErrMissingTenantScope` 同理（宿主 commercial.go:34 原件 + 本节点模块副本 + 12-commercial 未来副本）。申请 IB2 裁定单一归宿并登记删除批次。
4. **exc-0058..0061 解除提案**（remove_at=ib2，期限裁决归 IB2）：appconnector 侧定义消费方端口（`ports` 形态：`ExecutionGate` 同形接口 + 本模块 `BudgetRequest`/`UsageFact` 值类型，接口优先定义在使用方模块，Spec §4.2），adapter.go/action.go/oc_recovery.go 改依赖端口；commercial 类型适配器落点为宿主装配（IB2 在 container 装配处提供，或裁定 commercial 门面落地后走串行契约任务）。若 IB2 判定本批不可行，按 conventions §5 走基线变更/改期登记。
5. **airesource 过渡依赖登记**：模块 handler 包消费 `internal/application/repository/mcp_oauth.go` 的 `MCPOAuthBindingStore`/`NewMCPOAuthBindingStore`/`ErrOAuthBindingInvalid`/`ErrOAuthBindingActorNotMember`（该文件属主 11-airesource，B 期迁入 `internal/modules/airesource`）；请求 airesource 计划落位后由其 Integration Brief 提供导出面，本节点零改动该文件。
6. **别名行核销申请**：ownership-matrix :2463-2543 的 5 条 alias 行与 moves/appconnector.yaml 5 条 alias_obligations 为空义务（§2.4 证据；`b0ef8895a` 于 Pass A 集成期已删除并切装配），请 IB2 在 guard 口径下回写核销，B5 终验零残留。
7. **契约回写申请**（barrier 独占）：`appconnector.facade` 的 characterization/consumers 若需登记新 handler 包路径、`appconnector.routes` consumers 注释路径——由 IB2 按 contracts.yaml 回写规则处理，本节点不写。

## 8. 必须删除的 legacy/alias/例外（本节点口径）

| 项 | 状态 | 依据 |
|---|---|---|
| `internal/handler/app_connector_{action,connection,installation,oauth,oc,sync}.go`（6 文件） | **本节点删除**（`git mv` 迁出，路径消失） | ownership-matrix :1712-1746（destination handler） |
| `internal/handler/app_connector.go` | **本节点重写为 shim**；实现随 mv 迁出；shim 由 IB2 删除（Brief §7.1/.2） | ownership-matrix :1706；conventions §7.1 薄 shim 授权 |
| 5 条 alias 行（`internal/appconnector`、`internal/appconnector/openconnector`、`internal/application/repository/appconnector`、`internal/application/service/appconnector`、`internal/connectorcontrol`） | **已不存在**（Pass A 集成 `b0ef8895a` 删除；本节点 T3 命令复证零路径、零 importer）——核销登记进 Brief | ownership-matrix :2463-2543；moves/appconnector.yaml alias_obligations |
| exc-0058/0059/0060/0061（plan=27-appconnector，remove_at=ib2） | **本节点零删除**：引用原样保留（活跃例外覆盖）；解除提案登记留 IB2（§2.5 裁定） | exception-ledger :351-371；13-execution EX-8 Step 4 先例 |
| exc-0028（agentruntime→appconnector） | **非本节点行**（属主 32-agentruntime-tools）：本节点保证模块根包导出面零变化，消费方持续编译 | exception-ledger :170-175 |

## 9. 独立验收标准（全部满足才算完成；审查者逐条核验）

1. 节点门禁（DAG gates 原文，无替代命令）在节点分支 HEAD 全部退出码 0：`go build ./...`、`go test -count=1 ./internal/modules/appconnector/...`、`make check-backend-architecture`、`make verify-module-moves`。
2. `git diff <base_sha>...HEAD --name-only | sort` 与 §3 文件清单之差为空——零越权文件；router/container/bootstrap/go.mod/go.sum/migrations/治理四 YAML/`execution-dag.json`/`moves/appconnector.yaml` 零出现。
3. 7 条 legacy destination 落位：`internal/modules/appconnector/handler/` 存在 7 个生产文件 + 3 个测试文件；`git diff --summary` 对 7 个生产文件与 3 个测试文件显示 rename（R100）；`internal/handler/` 下 app_connector*.go 仅剩 shim 一个文件。
4. shim 面 = §2.1 消费面全集（5 类型别名、4 构造器、`DefaultAppOAuthProviderConfigs`、`appOK`/`appFail`/`appTenantScope`）且签名逐字一致（`go doc` 比对）；`newAppID`/`appRequireWriteCapability`/`OCPrepareInput`/`DecodeOCPrepare`/`appActionStates`/`appActionRisks` 未进 shim（零消费，§2.1）。
5. 模块根包导出面零变化：`go doc github.com/Tencent/WeKnora/internal/modules/appconnector` 在 base 与 HEAD 输出 diff 为空（exc-0028 消费方兼容证据）。
6. 差分证据齐备：§6 用例矩阵每行有搬迁前后双跑记录（命令原文 + 退出码 + 关键输出）与逐用例等价结论；Tenant/RBAC 面（403 写门族、MISSING_TENANT_SCOPE 字节、跨租户 404 族）逐条「等价」。
7. 别名/例外核销证据齐备：§8 表 5 条 alias 行的零存活命令输出；exc-0058..0061 的现状与提案登记；`legacy/README.md` 7 行已回填迁移状态。
8. `internal/modules/appconnector/module.go` 零变化（门面归 IB2）；adapter.go/action.go/oc_recovery.go 零变化（活跃例外覆盖面，`git diff` 为空）。
9. Integration Brief（§7 七项）与实施报告（conventions §1.2 命令包全量摘录、owned_files 核对、未完成项如实）落盘。
10. 提交序列恰好 4 个 commit（T1–T4），message 与 §5 各任务模板一致，无混合关注点提交（T1=test、T2=move+shim、T3/T4=docs）。

## 10. 升级路径（conventions §5）

- 任一 gate 不可能通过时：不改断言、不删测试、不扩例外、不复制他模块业务实现、不伪造证据；在 `docs/plans/passb/reports/b2-appconnector.md` 记录未过项/根因/复现命令/建议裁定，将 DAG 节点置 blocked 并等协调者裁定。
- 「上游门面尚未落地」类阻塞（Commercial 门面）：本计划已按不依赖门面设计（§2.5 原样保留例外 + 提案登记），预期不触发此类升级；若 IB2 对 §7.4 提案的裁定要求本节点追加端口化改造，属新串行契约任务，不在本节点内自行实施。
- 若实施中发现宿主消费面/测试依赖与 §2 实测不符（计划依据起点 SHA 静态读取），以真实代码为准修正 shim 面并在报告登记；若需改生产签名或跨 owner 文件则属契约变更，升级处理。

## 11. 计划自检记录（撰写者已执行）

- **Spec 覆盖**：§5.10 职责（安装/OAuth/连接/授权/动作/同步/Open Connector）与 7 文件一一对应（installation=④、OAuth+connection=⑤③、action=②⑥、sync=⑦、公共 helper=①）；§11 B2 并行边界与 framework:146-149 一致（depends_on=ib1，与 Knowledge/AgentCatalog/Datasource 文件不相交）；§12 Pass B 循环（特征测试→搬迁→差分→Brief→legacy 删除申请）映射为 T1→T4；§13 提交隔离（T1=M1、T2=M2+M3、T3/T4=docs）；§14.3 差分面=Tenant/RBAC 全覆盖；§5.18「Agent 工具只调用公开动作接口」由验收 5（模块根导出面零变化）保障。
- **无占位符**：全部文件路径、符号名、签名、行号、命令、响应码均来自本会话实读/实跑（起点 `8c45a8815`；基线 5 条命令输出见前置条件 3）；无 TBD/待定/虚构接口。唯二「实施时按已读实现自建 DDL/stub」项（T1 的 sync 裸表 DDL 按 sync.go:50-67 列集、gin router 按 oc_test 既有模式）已给出精确参照坐标与断言目标，不属发明。
- **类型一致**：§4.1 shim 转发签名 ↔ §2.1 消费面 ↔ moved 文件原签名三向一致；类型别名保方法集（`SetActionService`/`SetOCPreparer`/`SetOCConnectionService`/`SetAppOAuthProviders` 等注入方法随别名可见，container.go :981-999 的 Invoke 块零改动可编译）；`ErrMissingTenantScope` 副本消息逐字一致且无 `errors.Is` 跨值比较（§4.1 论证）。
- **跨任务接口一致**：T1 产出 `app_connector_lifecycle_test.go` = T2 随迁对象（同包同用例，差分前提）；T2 产出模块 handler 包 = §7.2 IB2 切换目标 = T3 `go doc` 核验对象；T3 evidence = T4 报告引用源；T4 gates = DAG 节点 gates 原文。
- **风险声明**：①本节点是 Pass B 首个把文件从 `internal/handler` 宿主包迁出的节点（B1 计划先行但零实施），shim 模式依据 12-commercial 设计 + conventions §7.1 授权，实际编译/门禁结果以 T2 实跑为准；②exception-ledger 4 行不删的裁定若与协调者口径冲突，按 §10 升级而非现场改判；③`internal/handler` 宿主包在 12-commercial/41-craft 后续搬迁时与本项目 shim 存在时序耦合（helper 副本三方收口），已登记 Brief §7.3。
