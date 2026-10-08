# Evidence — b2-appconnector（Pass B 27 计划）

> 节点：`b2-appconnector`（7 legacy handlers 搬迁 + shim + 差分 + 别名/例外核销证据）。
> 分支：`codex/passb-b2-appconnector`；BASE=`ced88ecb17ed560a02a15464500a4b2fa8e2d508`。
> 状态：**T3 定稿（B2-AC.3）** —— 本文件由 T1（B2-AC.1）创建基线（§1）、B2-AC.2 追加基线变更登记（§4）、
> T3（B2-AC.3）于 2026-09-24 在节点 HEAD `33f8c3ea3370ba6856acac875bb35abd46e1e223`
> （= 派发 BASE；树内容与迁移 commit `f545d7d06` 一致，仅多 docs 提交）完成差分复跑比对（§2）、
> 别名/例外核销证据（§3）与计数核验（§5）后定稿。

## 1. T1 特征化基线（搬迁前，宿主 `internal/handler` 实跑）

日期：2026-09-23；基线 HEAD：`ced88ecb17ed560a02a15464500a4b2fa8e2d508`（工作树仅新增未提交测试文件 `internal/handler/app_connector_lifecycle_test.go`，零生产改动）。

### 1.1 命令与退出码

| # | 命令（原文） | 退出码 | 结果 |
|---|---|---|---|
| 1 | `go build ./...` | 0 | OK（仅 cmd/desktop、cmd/server 链接期 `ld: warning: ignoring duplicate libraries: '-lc++'` 非致命警告） |
| 2 | `go test ./internal/handler/ -run 'TestAppInstallationLifecycle|TestAppConnectionCreate|TestAppSyncStatus|TestAppActionPipeline' -count=1 -v` | 0 | 新增 4 用例族全 PASS（§1.2） |
| 3 | `go test ./internal/handler/ -run 'TestAppConnectionOAuthFlow|TestOC|TestDecodeOCPrepare' -count=1 -v` | 0 | 既有宿主 OC/OAuth 20 个顶层用例全 PASS（TestOCExecuteA02DenialsMap4xxWithExplicitCodes 内含 4 子测试），0 FAIL |
| 4 | `go test ./internal/appconnector/... -count=1` | 0 | 5 包全 ok：root(0.495s)/connectorcontrol(3.569s)/openconnector(0.554s)/repository/appconnector(0.885s)/service/appconnector(3.033s) |
| 5 | `go test ./internal/appconnector/ -run 'TestActionExecuteWrapsIntentBudgetInner|TestActionIntentDenialBlocksEverything|TestActionBudgetDenialNeverDispatches' -count=1 -v` | 0 | U05 计量副作用 3 用例全 PASS |
| 6 | `go test ./internal/container/ -run 'TestOCProductRoutesRegisterWithoutConflict|TestProductionWiringInjectsDispatcherIntoService|TestNewOCArmedActionServiceEnabledRequiresFullConfig|TestNewOCArmedActionServiceDisabledKeepsRefusingDispatcher' -count=1 -v` | 0 | 装配面 4 用例全 PASS（open_connector_test.go:386-389 构造器消费方） |
| 7 | `make check-backend-architecture` | 0 | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16` + `OK (0 violations)` |
| 8 | `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |

### 1.2 T1 新增用例基线输出（`-v` 摘录）

```
=== RUN   TestAppInstallationLifecycleAndWriteGate
--- PASS: TestAppInstallationLifecycleAndWriteGate (0.00s)
=== RUN   TestAppConnectionCreateBranches
--- PASS: TestAppConnectionCreateBranches (0.00s)
=== RUN   TestAppSyncStatusThreeStates
--- PASS: TestAppSyncStatusThreeStates (0.00s)
=== RUN   TestAppActionPipelineUnwiredFailsClosed
--- PASS: TestAppActionPipelineUnwiredFailsClosed (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler	1.594s
```

断言锚点（取自已读实现，T3 差分逐条比对）：

- `TestAppInstallationLifecycleAndWriteGate`：member 写 403 `MEMBER_MUST_REQUEST_INSTALLATION`；owner create 201（view `id/app_key/version/state/scopes`，scopes=`["read:user"]` 来自 app_versions.schema_json）；list 200（1 行）；upgrade 1.0.0→2.0.0（同 schema，非扩权）200 active；disable 200 disabled（CAS 1→2）；跨租户与不存在 id 同为 404 `INSTALLATION_NOT_FOUND`。
- `TestAppConnectionCreateBranches`：缺字段 400 `INVALID_REQUEST`（含缺 redirect_uri）；installation 不存在 404 `INSTALLATION_NOT_FOUND`；`expected_version=3`(实际 4) 409 `VERSION_CONFLICT`；feishu 已知未配置 501 `OAUTH_NOT_CONFIGURED`；github 未知 app 400 `UNKNOWN_APP`；notion 成功流 201（`authorization_state` 非空、`authorize_url` 含 `api.notion.com`、`expires_at`、`installation_id`、`kind`）。
- `TestAppSyncStatusThreeStates`：不存在/跨租户 404 `DATASOURCE_NOT_FOUND`；无绑定行 200 `binding=null`、`requires_reauthorization=false`、`state=completed`；active space 链 200 binding 视图 `installation_id=inst-s/connection_id=conn-space/auth_version="7"`（live 值覆盖持久值 5）；personal 连接 200 `requires_reauthorization=true`、`pause_reason="permission"`、binding 仍携带 ids 与 `auth_version="2"`。
- `TestAppActionPipelineUnwiredFailsClosed`：未调 `SetActionService`，POST `/api/v1/apps/actions/prepare`（connection 存在）→ 501 `ACTION_PIPELINE_NOT_CONFIGURED`。

### 1.3 差分用例矩阵基线计数（§6 对照）

| 面 | 用例 | 基线（宿主/模块） | 结果 |
|---|---|---|---|
| OAuth 连接流 | `TestAppConnectionOAuthFlow`（宿主，随迁同文件） | PASS | 1 |
| connection create 分支族 | `TestAppConnectionCreateBranches`（T1 新增，随迁） | PASS | 6 断言组 |
| installation 生命周期+写门+跨租户 404 | `TestAppInstallationLifecycleAndWriteGate`（T1 新增，随迁） | PASS | 3 断言组 |
| sync status 三态 | `TestAppSyncStatusThreeStates`（T1 新增，随迁） | PASS | 3 |
| 原生 action 未接线 fail-closed | `TestAppActionPipelineUnwiredFailsClosed`（T1 新增，随迁） | PASS | 1 |
| OC 全链路 | 宿主 oc_test 20 顶层用例（随迁同文件） | 全 PASS | 20（内含 4 子测试） |
| U05 计量副作用 | http_policy_test 3 用例（模块内不动） | 全 PASS | 3 |
| 路由注册/装配 | container open_connector_test 4 用例（宿主不动，经 shim 编译） | 全 PASS | 4 |
| 模块既有 5 包 | `go test ./internal/appconnector/...` | 全 ok | 5 |

## 2. T3 差分复跑（B2-AC.3，2026-09-24 @ HEAD `33f8c3ea3`）

复跑环境：worktree `codex/passb-b2-appconnector`，HEAD=`33f8c3ea3`（树内容与迁移 commit
`f545d7d06` 完全一致——`git diff f545d7d06..33f8c3ea3` 仅含 docs：pass-a-acceptance 台账、
本 evidence 文件、`tools/passbguard/ownership_test.go` 字面量计数修正）。10 个搬迁文件为
R100 字节同一（§4），故**同一测试文件 + 同一 `-run` 过滤器在两个位置运行 = 同一断言集
作用于同一代码**；PASS 对（基线 PASS + 复跑 PASS）即逐用例等价。

### 2.1 命令对命令复跑结果（与 §1.1 基线逐条对应）

| # | 基线（§1.1，宿主/搬迁前位置） | 复跑（T3，搬迁后位置） | 退出码对 | 结果对 |
|---|---|---|---|---|
| 1 | `go build ./...` | 同命令 | 0 / 0 | 均仅 cmd 链接期预存 `ld: warning: duplicate libraries` 非致命警告 |
| 2 | `go test ./internal/handler/ -run 'TestAppInstallationLifecycle\|TestAppConnectionCreate\|TestAppSyncStatus\|TestAppActionPipeline' -count=1 -v` | `go test ./internal/appconnector/handler/ -run 'TestAppInstallationLifecycle\|TestAppConnectionCreate\|TestAppSyncStatus\|TestAppActionPipeline' -count=1 -v` | 0 / 0 | T1 新增 4 用例族双跑 PASS（§2.2） |
| 3 | `go test ./internal/handler/ -run 'TestAppConnectionOAuthFlow\|TestOC\|TestDecodeOCPrepare' -count=1 -v` | `go test ./internal/appconnector/handler/ -run 'TestAppConnectionOAuthFlow\|TestOC\|TestDecodeOCPrepare' -count=1 -v` | 0 / 0 | OC/OAuth 顶层 21 用例 + A02 内 4 子测试双跑全 PASS（§2.3；计数更正见 §2.3 注） |
| 4 | `go test ./internal/appconnector/... -count=1` | 同命令 | 0 / 0 | 基线 5 包 ok → 复跑 6 包 ok（新增 handler 包；root 0.394s/connectorcontrol 3.781s/handler 3.694s/openconnector 3.141s/repository 3.304s/service 2.313s） |
| 5 | `go test ./internal/appconnector/ -run 'TestActionExecuteWrapsIntentBudgetInner\|TestActionIntentDenialBlocksEverything\|TestActionBudgetDenialNeverDispatches' -count=1 -v` | 同命令（模块内不动） | 0 / 0 | U05 计量 3 用例双跑 PASS |
| 6 | `go test ./internal/container/ -run 'TestOCProductRoutesRegisterWithoutConflict\|TestProductionWiringInjectsDispatcherIntoService\|TestNewOCArmedActionServiceEnabledRequiresFullConfig\|TestNewOCArmedActionServiceDisabledKeepsRefusingDispatcher' -count=1 -v` | 同命令（宿主不动，经 shim 编译） | 0 / 0 | 装配面 4 用例双跑 PASS（构造器经 `NewApp*Handler` 转发解析到同一模块类型） |
| 7 | `make check-backend-architecture` | 同命令 | 0 / 0 | 计数逐字一致（§5.1） |
| 8 | `make verify-module-moves` | 同命令 | 0 / 0 | `OK (16 manifests verified)` 双跑一致 |
| 9 | —（T2 命令集） | `go test ./internal/handler/ -count=1` | — / 0 | 宿主包全量零回归（其余宿主测试经 shim 编译通过） |

### 2.2 T1 用例族复跑 `-v` 摘录（模块包）

```
--- PASS: TestAppInstallationLifecycleAndWriteGate (0.00s)
--- PASS: TestAppConnectionCreateBranches (0.00s)
--- PASS: TestAppSyncStatusThreeStates (0.00s)
--- PASS: TestAppActionPipelineUnwiredFailsClosed (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/appconnector/handler	0.757s
```

### 2.3 §6 用例矩阵逐行等价结论

判定依据：断言锚点已在 §1.2 逐条登记（响应码 / body code / stub 收到参数 / 视图字段），
搬迁文件 R100 字节同一（§4）⇒ 双跑 PASS = 同一 HTTP 可观察行为。

| 面 | 用例 | 基线 | 复跑 | 等价结论（锚点复验） |
|---|---|---|---|---|
| OAuth 连接流 | `TestAppConnectionOAuthFlow`（随迁同文件） | PASS | PASS | **等价**：state 签发/兑换与公开 callback leg 断言逐字同一 |
| connection create 分支族 | `TestAppConnectionCreateBranches` | PASS | PASS | **等价**：400 `INVALID_REQUEST`/404 `INSTALLATION_NOT_FOUND`/409 `VERSION_CONFLICT`/501 `OAUTH_NOT_CONFIGURED`/400 `UNKNOWN_APP`/201 成功流（`authorization_state`/`authorize_url`/`expires_at`/`installation_id`/`kind`） |
| installation 生命周期+写门+跨租户 404 | `TestAppInstallationLifecycleAndWriteGate` | PASS | PASS | **等价**：member 写 403 `MEMBER_MUST_REQUEST_INSTALLATION`、owner 全链 200/201（视图 `id/app_key/version/state/scopes`）、跨租户与不存在 id 同 404 `INSTALLATION_NOT_FOUND` |
| sync status 三态 | `TestAppSyncStatusThreeStates` | PASS | PASS | **等价**：404 `DATASOURCE_NOT_FOUND`、200 `binding=null`+`state=completed`、200 绑定视图 `installation_id/connection_id/auth_version`+`pause_reason="permission"` |
| 原生 action 未接线 fail-closed | `TestAppActionPipelineUnwiredFailsClosed` | PASS | PASS | **等价**：501 `ACTION_PIPELINE_NOT_CONFIGURED` |
| OC 全链路 | oc_test 随迁同文件 | 全 PASS | 全 PASS | **等价**：catalog 可达性/租户隔离/成员可读、attempt 生命周期/API-key handoff/门禁错误、prepare 端点族、execute 4xx（A02 四子测试 forbidden/not_member/version_stale/revoked）/409/429/happy、跨租户不可见、API-key 默认拒绝（`TestOCProductRoutesDefaultDenyAPIKeys`）——21 顶层用例全部双跑 PASS |
| U05 计量副作用 | http_policy_test 3 用例（模块内不动） | PASS | PASS | **等价**：intent→budget→call→settle 编排、预算拒绝不派发（同文件同位置，未搬迁） |
| 路由注册/装配 | container open_connector_test 4 用例（宿主不动） | PASS | PASS | **等价**：经 shim 类型别名+构造器转发，dig Provide/Invoke 装配路径零变化 |
| 模块既有包回归 | `go test ./internal/appconnector/...` | 5 包 ok | 6 包 ok | **等价**：原 5 包零回归，新增 handler 包含全部随迁用例 |

> 注（计数更正）：§1.1 行 3 草稿记「20 个顶层用例」，实为清点笔误——同一 `-run` 过滤器
> 复跑实测 **21 个顶层用例**（T2 commit `f545d7d06` 说明同为 21；A02 用例另含 4 子测试）。
> 文件字节同一 ⇒ 基线侧集合与复跑侧恒等（21=21），不影响等价结论。§1 作为历史记录不改。

### 2.4 Tenant/RBAC 判定口径（conventions §6）

本节点对写门（`appRequireWriteCapability` + `CanInstallInstallation`/`CanManageConnections`/
`CanManageActions`）、tenant-from-context 读取、by-tenant 查找与 404 防枚举**零代码改动**
（纯 R100 移动 + shim 转发；模块侧 `appTenantScope` 用 `handler_helpers.go` 本地哨兵，
消息与宿主 `commercial.go:34` 逐字一致 ⇒ 403 `MISSING_TENANT_SCOPE` 响应字节不变；全仓无
`errors.Is` 跨值比较）。等价性由 §2.3 的 403 写门族、MISSING_TENANT_SCOPE 锚点、跨租户 404 族
逐用例双跑 PASS 证明。

## 3. 别名/例外核销证据（B2-AC.3 实跑）

### 3.1 别名核销：5 条 alias 行 = 空义务（§2.4 三命令复证）

日期：2026-09-24 @ HEAD `33f8c3ea3`。

**命令 1 — 旧路径零存活**：

```
$ git ls-tree HEAD internal/appconnector internal/connectorcontrol \
    internal/application/repository/appconnector internal/application/service/appconnector
（输出为空；退出码 0）
```

`internal/appconnector` 树不存在即覆盖其子路径 `internal/appconnector/openconnector`——
5 条 alias 路径全部不存在。

**命令 2 — 旧 import path 全仓零命中**：

```
$ grep -rn 'WeKnora/internal/appconnector"\|WeKnora/internal/connectorcontrol"\
    \|application/repository/appconnector"\|application/service/appconnector"' \
    --include='*.go' internal/ cmd/
（零命中；grep 退出码 1）
```

**命令 3 — Pass A 集成删除提交自证**：

```
$ git show b0ef8895a --stat
commit b0ef8895ac774be44b2f4138beec92d72b39401e
    refactor(integration): switch appconnector composition to module paths and drop pass-a aliases
 internal/appconnector/alias.go                     | 151 ---------------------
 internal/appconnector/openconnector/alias.go       |  29 ----
 .../application/repository/appconnector/alias.go   |  52 -------
 internal/application/service/appconnector/alias.go | 122 -----------------
 internal/connectorcontrol/alias.go                 |  62 ---------
 internal/container/container.go                    |   4 +-
 6 files changed, 2 insertions(+), 418 deletions(-)
```

**结论**：ownership-matrix :2463-2543 的 5 条 alias 行（plan=27，delete_barrier=ib2）与
moves/appconnector.yaml 5 条 alias_obligations 在当前树为**空义务**（零路径、零 importer；
Pass A 集成 `b0ef8895a` 已物理删除并切装配）。本节点不重建、不改治理 YAML；核销回写申请
交付 IB2（详见 Brief，T4 定稿），B5 终验零残留。

### 3.2 例外核销登记：exc-0058..0061 原样保留 + 解除提案留 IB2

**现状（2026-09-24 实测）**：3 文件 4 行 import 原样在位——

```
$ grep -n 'modules/commercial' internal/appconnector/adapter.go \
    internal/appconnector/service/appconnector/action.go \
    internal/appconnector/service/appconnector/oc_recovery.go
adapter.go:9:            ".../internal/commercial"                      （exc-0058）
service/appconnector/action.go:12:      ".../internal/commercial"       （exc-0059）
service/appconnector/oc_recovery.go:45: ".../internal/commercial"       （exc-0060）
service/appconnector/oc_recovery.go:46: commsvc ".../internal/commercial/service/commercial" （exc-0061）
```

ledger 四行在位（`.worktrees/passb-int/docs/architecture/passb/exception-ledger.yaml`
:350-373，remove_at=ib2，本节点只读不改）。消费符号面（`commercial.ExecutionGate`/
`BudgetRequest`/`UsageFact`/`Credits`/`ErrInsufficientBudgetGate`/`FundingPlatform`/
`ServiceConnector`/`DimensionConnector`/`UsageStatusFinal`、`commsvc.ErrGateReservationUnknown`）
零变化——adapter.go/action.go/oc_recovery.go 三文件在本节点 diff 中**零出现**（§5.2 文件清单）。

**裁定（13-execution EX-8 Step 4 先例）**：解除需「appconnector 消费方端口 + commercial 类型
适配器」，适配器唯一合法落点是宿主装配层（集成工程师独占），且 B1-CM 零实施 ⇒ 商业门面在
当前树不存在；conventions §5 禁止新增例外/自行实现上游门面。故**本节点不删行**，登记解除提案
（消费方端口形态 + IB2 装配适配点，remove_at=ib2 的期限裁决权归 IB2）——提案全文在
Integration Brief §7.4（T4 定稿交付）。

### 3.3 exc-0028 消费方零受影响声明（agentruntime→appconnector，属主 32-agentruntime-tools）

模块根包导出面 T2 前后零变化，证据三件：

1. **源码零 diff**：`git diff 67ac22c96..HEAD -- internal/appconnector/ ':!internal/appconnector/handler'`
   中 `.go` 文件**零出现**（该区间 handler/ 外仅 `legacy/README.md` 状态回填）。
2. **`go doc` 双跑 diff 为空**（T2 前 `67ac22c96` 临时 worktree vs HEAD `33f8c3ea3`）：
   因离线环境 `GOPROXY=off`，完整 import 路径形式不可解析（两侧同因失败，不计证据），
   改用相对路径形式双跑——

   ```
   $ cd <tree@67ac22c96> && go doc ./internal/appconnector   # 退出码 0
   $ cd <worktree@33f8c3ea3> && go doc ./internal/appconnector  # 退出码 0
   $ diff /tmp/godoc_pre.txt /tmp/godoc_post.txt   # 输出为空（112 行有效内容逐字节一致）
   ```

   输出含包文档 + 全部导出常量/类型/函数（`RiskRead`/`ActionAwaitingApproval`/
   `NewModule` 门面契约等），双跑逐字节一致。
3. **消费方持续编译**：`go build ./internal/agentruntime/agent/tools/` 退出码 0
   （`internal/agentruntime/agent/tools/app_connector.go` 消费模块根包导出面，
   B3 前持续编译成立）。

## 5. 计数核验（B2-AC.3，2026-09-24 @ HEAD `33f8c3ea3`）

### 5.1 守卫实测（与 §1.1 行 7/8 基线逐字一致）

```
$ make check-backend-architecture
architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16
architectureguard: OK (0 violations)                      # 退出码 0
$ make verify-module-moves
modulemove: OK (16 manifests verified)                    # 退出码 0
```

`633 路由 / 23+23 任务 / 58 挂点 / 16 模块` 与基线、pass-a-acceptance 台账（396→390 变更已按
conventions §8 流程登记于 §4）三方一致；正式三方复核在 IB2。

### 5.2 本节点 diff 零装配触碰证明

```
$ git diff ced88ecb1 --name-only | sort        # 节点全部分支提交 → 工作树（含本文件定稿）
docs/architecture/evidence/pass-a-acceptance.md
docs/architecture/evidence/passb/b2-appconnector.md
docs/architecture/moves/appconnector.yaml
docs/architecture/passb/ownership-matrix.yaml
internal/handler/app_connector.go
internal/appconnector/handler/app_connector.go
internal/appconnector/handler/app_connector_action.go
internal/appconnector/handler/app_connector_connection.go
internal/appconnector/handler/app_connector_installation.go
internal/appconnector/handler/app_connector_lifecycle_test.go
internal/appconnector/handler/app_connector_oauth.go
internal/appconnector/handler/app_connector_oauth_test.go
internal/appconnector/handler/app_connector_oc.go
internal/appconnector/handler/app_connector_oc_test.go
internal/appconnector/handler/app_connector_sync.go
internal/appconnector/handler/handler_helpers.go
internal/appconnector/legacy/README.md
tools/passbguard/ownership_test.go
$ git diff ced88ecb1 --name-only | grep -E 'internal/router/|internal/container/|internal/bootstrap/|^go\.(mod|sum)$|^migrations/'
（零命中；grep 退出码 1）
```

18 个文件全部 ⊆ 节点范围（7+3 搬迁、shim、helpers、legacy/README、治理两 YAML 行删除
+台账+guard 字面量修正——三者均按 Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 同窗执行，见 §4）；
**router/container/bootstrap/go.mod/go.sum/migrations 零出现** ⇒ contracts `appconnector.routes`
（17 条注册面）与 `appconnector.lifecycle`（5 挂点）注册面零变化，633/23+23/58 计数不受影响。

## 4. 基线变更登记（Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP，conventions §8 流程；B2-AC.2 写入）

**背景**：T2 实施前核验发现计划内置冲突——6 个 legacy 文件物理迁出后
`tools/modulemove/verify.go`（legacy_files 存在性校验，:326-334）必报
`legacy-file: not found` 并 exit 1（单文件实验证实：仅迁 app_connector_sync.go 即
`modulemove: appconnector: legacy-file: not found: internal/handler/app_connector_sync.go`，
退出码 1）；而 moves manifest 对本节点只读。升级协调者后获裁定
**Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP**（方案 A，适用于 Pass B 全部迁移节点）：
迁移 commit 同窗删除 manifest + ownership-matrix 对应行；独立 docs commit 登记
396→390 基线变更；passbguard 测试字面量计数允许同窗纯数字修正；每条被删行登记
「原路径 → 目标包 + 迁移 commit」。台账条目见
`docs/architecture/evidence/pass-a-acceptance.md` §6。

**逐行去向登记**（迁移 commit：`f545d7d06`
`refactor(appconnector): move app connector handlers into module with alias shims`，
分支 `codex/passb-b2-appconnector`，rename R100）：

| 原路径（行已删） | 目标包 | 迁移 commit |
|---|---|---|
| `internal/handler/app_connector_action.go` | `internal/appconnector/handler/app_connector_action.go` | `f545d7d06` |
| `internal/handler/app_connector_connection.go` | `internal/appconnector/handler/app_connector_connection.go` | `f545d7d06` |
| `internal/handler/app_connector_installation.go` | `internal/appconnector/handler/app_connector_installation.go` | `f545d7d06` |
| `internal/handler/app_connector_oauth.go` | `internal/appconnector/handler/app_connector_oauth.go` | `f545d7d06` |
| `internal/handler/app_connector_oc.go` | `internal/appconnector/handler/app_connector_oc.go` | `f545d7d06` |
| `internal/handler/app_connector_sync.go` | `internal/appconnector/handler/app_connector_sync.go` | `f545d7d06` |

`internal/handler/app_connector.go` 行（manifest + matrix）**保留**：宿主路径重写为过渡
shim，文件仍在盘上；IB2 切换装配删除 shim 时按 Brief 同窗删行。

**字节同一性证据**：10 个搬迁文件逐一 `cmp <(git show HEAD:internal/handler/<f>)
internal/appconnector/handler/<f>` 全部一致（其中 oc_test 初次转写出现 1 处
`Provider: "slack"`→`"github"` 误差，cmp 抓出后已修正复验一致）。shim 转发签名与
moved 原签名逐字一致（§4.1）；`ErrMissingTenantScope` 模块副本消息逐字一致
（`handler_helpers.go`），宿主 shim 沿用宿主 commercial.go:34 原哨兵。

**变更后守卫实跑**（迁移 commit `f545d7d06`）：

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go build ./...` | 0 | OK（仅 cmd 链接期预存 `ld: warning: duplicate libraries` 非致命警告） |
| `go test ./internal/appconnector/handler/ -count=1 -v` | 0 | 25 顶层 + 4 子测试全 PASS（21 既有 + T1 4 用例） |
| `go test ./internal/handler/ -count=1` | 0 | 宿主包零回归（经 shim） |
| `go test ./internal/container/ -run 'TestOCProductRoutes\|TestNewOCArmedActionService' -count=1` | 0 | 装配面 3 用例 PASS |
| `go test ./internal/appconnector/... -count=1` | 0 | 6 包全 ok（新增 handler 包） |
| `make check-backend-architecture` | 0 | `total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`、`OK (0 violations)` |
| `make verify-module-moves` | 0 | `OK (16 manifests verified)`（行删除生效） |

## 6. 重派复验（B2-AC.3 第二轮，2026-09-24 @ 派发 BASE `6e8c84860` = T4 终态）

**背景**：T3 原于 2026-09-24 在 `33f8c3ea3` 定稿（§2/§3/§5），T4 交付 Brief+报告于
`6e8c84860`。协调者在 `6e8c84860`（=本轮派发 BASE）重派 B2-AC.3；本轮按计划 T3
步骤 1–4 在该终态**逐步复跑全部验证命令**，作为对 §2/§3/§5 证据在节点最终状态上的
再验证。本轮 diff 仅含本追加节（evidence 文件本身）。

### 6.1 差分复跑（步骤 1，逐条命令 + 退出码）

| # | 命令（原文，@ `6e8c84860`） | 退出码 | 关键输出 |
|---|---|---|---|
| 1 | `go build ./...` | 0 | 仅 cmd/desktop、cmd/server 预存 `ld: warning: ignoring duplicate libraries: '-lc++'` 非致命警告（同基线） |
| 2 | `go test ./internal/appconnector/handler/ -run 'TestAppInstallationLifecycle\|TestAppConnectionCreate\|TestAppSyncStatus\|TestAppActionPipeline' -count=1 -v` | 0 | T1 用例族 4 用例全 PASS（`ok .../handler 1.903s`） |
| 3 | `go test ./internal/appconnector/handler/ -run 'TestAppConnectionOAuthFlow\|TestOC\|TestDecodeOCPrepare' -count=1 -v` | 0 | `--- PASS` 计数 **21**（顶层 21 用例，含 A02 内 4 子测试；0 FAIL；`ok .../handler`） |
| 4 | `go test ./internal/appconnector/... -count=1` | 0 | **6 包全 ok**：root 0.198s / connectorcontrol 2.824s / handler 4.105s / openconnector 1.270s / repository 1.541s / service 5.520s |
| 5 | `go test ./internal/appconnector/ -run 'TestActionExecuteWrapsIntentBudgetInner\|TestActionIntentDenialBlocksEverything\|TestActionBudgetDenialNeverDispatches' -count=1 -v` | 0 | U05 计量 3 用例全 PASS |
| 6 | `go test ./internal/container/ -run 'TestOCProductRoutesRegisterWithoutConflict\|TestProductionWiringInjectsDispatcherIntoService\|TestNewOCArmedActionServiceEnabledRequiresFullConfig\|TestNewOCArmedActionServiceDisabledKeepsRefusingDispatcher' -count=1 -v` | 0 | 装配面 4 用例全 PASS（经 shim，`ok .../container 5.283s`） |
| 7 | `go test ./internal/handler/ -count=1` | 0 | 宿主包全量零回归（经 shim，`ok .../handler 2.001s`） |

### 6.2 别名/例外/计数复验（步骤 2–4）

- **别名三命令**：`git ls-tree HEAD internal/appconnector internal/connectorcontrol
  internal/application/repository/appconnector internal/application/service/appconnector`
  → 输出空、退出码 0；旧 import path grep（§3.1 命令 2 原文）→ 零命中、退出码 1；
  `git show b0ef8895a --stat` → 摘录与 §3.1 逐字一致（418 deletions）。结论不变：
  **5 条 alias 行 = 空义务**。
- **exc-0058..0061 现状**：grep 实测 3 文件 4 行 import 原样在位
  （adapter.go:9、action.go:12、oc_recovery.go:45、oc_recovery.go:46）——零删除、零改动。
- **exc-0028 消费方**：`go doc ./internal/appconnector` 双跑（临时 worktree
  @`67ac22c96` 802 行 vs HEAD `6e8c84860` 802 行）→ `diff` 输出空、退出码 0
  （导出面零变化）；`go build ./internal/agentruntime/agent/tools/` → 退出码 0。
- **计数门禁**：`make check-backend-architecture` → `literal=564 apiKeyRoute=69 handle=0
  total=633 | redis=23 lite=23 | hooks=58 | modules=16` + `OK (0 violations)`、退出码 0；
  `make verify-module-moves` → `OK (16 manifests verified)`、退出码 0。与 §5.1 逐字一致。
- **节点全量 diff 复核**：`git diff ced88ecb1 --name-only | sort` = 20 文件
  （§5.2 的 18 文件 + T4 交付的 briefs/reports 2 文件）；`grep -E 'internal/router/|
  internal/container/|internal/bootstrap/|^go\.(mod|sum)$|^migrations/'` → 零命中、
  退出码 1。router/container/bootstrap/go.mod/go.sum/migrations 零触碰结论在终态成立。

### 6.3 复验结论

§2 差分矩阵、§3 别名/例外核销、§5 计数核验的全部证据在 T4 终态 `6e8c84860`
（节点 HEAD）上**全部复现成立**；本轮无新发现、无偏差。

