# Evidence — b2-appconnector（Pass B 27 计划）

> 节点：`b2-appconnector`（7 legacy handlers 搬迁 + shim + 差分 + 别名/例外核销证据）。
> 分支：`codex/passb-b2-appconnector`；BASE=`ced88ecb17ed560a02a15464500a4b2fa8e2d508`。
> 状态：**T1 基线冻结（草稿）** —— 本文件由 T1（B2-AC.1）创建，T3（B2-AC.3）差分复跑比对后定稿提交。

## 1. T1 特征化基线（搬迁前，宿主 `internal/handler` 实跑）

日期：2026-09-23；基线 HEAD：`ced88ecb17ed560a02a15464500a4b2fa8e2d508`（工作树仅新增未提交测试文件 `internal/handler/app_connector_lifecycle_test.go`，零生产改动）。

### 1.1 命令与退出码

| # | 命令（原文） | 退出码 | 结果 |
|---|---|---|---|
| 1 | `go build ./...` | 0 | OK（仅 cmd/desktop、cmd/server 链接期 `ld: warning: ignoring duplicate libraries: '-lc++'` 非致命警告） |
| 2 | `go test ./internal/handler/ -run 'TestAppInstallationLifecycle|TestAppConnectionCreate|TestAppSyncStatus|TestAppActionPipeline' -count=1 -v` | 0 | 新增 4 用例族全 PASS（§1.2） |
| 3 | `go test ./internal/handler/ -run 'TestAppConnectionOAuthFlow|TestOC|TestDecodeOCPrepare' -count=1 -v` | 0 | 既有宿主 OC/OAuth 20 个顶层用例全 PASS（TestOCExecuteA02DenialsMap4xxWithExplicitCodes 内含 4 子测试），0 FAIL |
| 4 | `go test ./internal/modules/appconnector/... -count=1` | 0 | 5 包全 ok：root(0.495s)/connectorcontrol(3.569s)/openconnector(0.554s)/repository/appconnector(0.885s)/service/appconnector(3.033s) |
| 5 | `go test ./internal/modules/appconnector/ -run 'TestActionExecuteWrapsIntentBudgetInner|TestActionIntentDenialBlocksEverything|TestActionBudgetDenialNeverDispatches' -count=1 -v` | 0 | U05 计量副作用 3 用例全 PASS |
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
| 模块既有 5 包 | `go test ./internal/modules/appconnector/...` | 全 ok | 5 |

## 2. T3 差分复跑（待 T2 搬迁后回填）

（占位：T2 完成后同命令复跑，逐用例与 §1 基线比对，回填等价结论。）

## 3. 别名/例外核销证据（待 T3 回填）

（占位：§2.4 三条命令复证、exc-0058..0061 现状登记、`go doc` 导出面零变化比对。）

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
| `internal/handler/app_connector_action.go` | `internal/modules/appconnector/handler/app_connector_action.go` | `f545d7d06` |
| `internal/handler/app_connector_connection.go` | `internal/modules/appconnector/handler/app_connector_connection.go` | `f545d7d06` |
| `internal/handler/app_connector_installation.go` | `internal/modules/appconnector/handler/app_connector_installation.go` | `f545d7d06` |
| `internal/handler/app_connector_oauth.go` | `internal/modules/appconnector/handler/app_connector_oauth.go` | `f545d7d06` |
| `internal/handler/app_connector_oc.go` | `internal/modules/appconnector/handler/app_connector_oc.go` | `f545d7d06` |
| `internal/handler/app_connector_sync.go` | `internal/modules/appconnector/handler/app_connector_sync.go` | `f545d7d06` |

`internal/handler/app_connector.go` 行（manifest + matrix）**保留**：宿主路径重写为过渡
shim，文件仍在盘上；IB2 切换装配删除 shim 时按 Brief 同窗删行。

**字节同一性证据**：10 个搬迁文件逐一 `cmp <(git show HEAD:internal/handler/<f>)
internal/modules/appconnector/handler/<f>` 全部一致（其中 oc_test 初次转写出现 1 处
`Provider: "slack"`→`"github"` 误差，cmp 抓出后已修正复验一致）。shim 转发签名与
moved 原签名逐字一致（§4.1）；`ErrMissingTenantScope` 模块副本消息逐字一致
（`handler_helpers.go`），宿主 shim 沿用宿主 commercial.go:34 原哨兵。

**变更后守卫实跑**（迁移 commit `f545d7d06`）：

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go build ./...` | 0 | OK（仅 cmd 链接期预存 `ld: warning: duplicate libraries` 非致命警告） |
| `go test ./internal/modules/appconnector/handler/ -count=1 -v` | 0 | 25 顶层 + 4 子测试全 PASS（21 既有 + T1 4 用例） |
| `go test ./internal/handler/ -count=1` | 0 | 宿主包零回归（经 shim） |
| `go test ./internal/container/ -run 'TestOCProductRoutes\|TestNewOCArmedActionService' -count=1` | 0 | 装配面 3 用例 PASS |
| `go test ./internal/modules/appconnector/... -count=1` | 0 | 6 包全 ok（新增 handler 包） |
| `make check-backend-architecture` | 0 | `total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`、`OK (0 violations)` |
| `make verify-module-moves` | 0 | `OK (16 manifests verified)`（行删除生效） |

