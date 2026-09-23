# 实施报告 — b2-appconnector（Pass B 27 计划）

> 节点：`b2-appconnector`（7 legacy handlers 迁入 `internal/modules/appconnector/handler` + 过渡 shim + 差分证据 + 别名/例外核销登记）。
> 分支：`codex/passb-b2-appconnector`；节点分叉点（base）`8c45a8815`（ib1 登记提交，= 计划前置条件 2 所述派发 base）；
> 本报告定稿于 T4（B2-AC.4），2026-09-24。
> 证据源：`docs/architecture/evidence/passb/b2-appconnector.md`（T1 基线 §1、差分复跑 §2、别名/例外 §3、Ruling 登记 §4、计数 §5）。
> Integration Brief：`docs/architecture/passb/briefs/b2-appconnector.md`（T4 定稿，§7 七项齐备）。

## 1. 提交序列（T4 定稿时刻）

| # | commit | 任务 | 内容 |
|---|---|---|---|
| 1 | `ced88ecb1` | 派发流程 | `docs(plan): passb b2-appconnector`（27 计划文档落盘，计划撰写节点产出） |
| 2 | `67ac22c96` | T1（B2-AC.1） | `test(passb): characterize app installation connection sync handlers before move` |
| 3 | `f545d7d06` | T2（B2-AC.2） | `refactor(appconnector): move app connector handlers into module with alias shims` |
| 4 | `33f8c3ea3` | 计划外（Ruling） | `docs(passb): register legacy baseline change 396-390 per ruling 2026-09-23-LEGACY-ROW-OWNERSHIP` |
| 5 | `9003b687c` | T3（B2-AC.3） | `docs(passb): record b2-appconnector differential and alias-expiry evidence` |
| 6 | （本提交） | T4（B2-AC.4） | `docs(passb): record b2-appconnector integration brief and node report` |

**与 27 计划验收标准 10 的偏差登记**：标准原文「提交序列恰好 4 个 commit（T1–T4）」。实际为 6 个：+`ced88ecb1`（计划文档独立提交，派发流程要求，先于 T1）+`33f8c3ea3`（Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP §3/§4 明确要求「独立 docs commit 登记 396→390 基线变更」，协调者裁定批准）。T1–T4 四个任务提交的 message 与计划 §5 模板逐字一致，无混合关注点提交。

## 2. 节点 gates（DAG `b2-appconnector.gates` 原文，T4 于 HEAD `9003b687c` 实跑）

| # | gate（argv 原文展开） | 退出码 | 关键输出 |
|---|---|---|---|
| 1 | `go build ./...` | 0 | 仅 `cmd/desktop`/`cmd/server` 链接期预存 `ld: warning: ignoring duplicate libraries: '-lc++'` 非致命警告 |
| 2 | `go test -count=1 ./internal/modules/appconnector/...` | 0 | 6 包全 ok：root 3.179s / connectorcontrol 3.844s / **handler 0.931s（本节点新增包）** / openconnector 0.170s / repository/appconnector 0.326s / service/appconnector 2.386s |
| 3 | `make check-backend-architecture` | 0 | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16` + `OK (0 violations)` |
| 4 | `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |

四门禁全绿；计数 633/23+23/58/16 与 T1 基线（evidence §1.1 行 7/8）逐字一致（正式三方复核归 IB2/B5）。

## 3. conventions §1.2 命令包（T4 实跑摘录）

```bash
$ PASSB_BASE_SHA=$(git merge-base origin/main HEAD)     # → b1a3d6dd825e3263e12b1daac2a52dab80ac5813
$ git diff --stat "$PASSB_BASE_SHA"...HEAD              # 退出码 0
76 files changed, 19538 insertions(+), 157 deletions(-)
```

**口径说明**：`merge-base origin/main HEAD`（b1a3d6dd8）早于节点分叉点，该 diff 包含 passb-int 集成分支上 ib1 期间的 passbguard 工具与 contractrepo fixtures 等非本节点内容，**不能**直接作 owned_files 核对。节点级核对以下列节点分叉点 diff 为准（= 计划验收标准 2 的 `<base_sha>` 语义）：

```bash
$ git diff 8c45a8815...HEAD --name-only | sort          # 退出码 0（19 文件，T4 提交前）
docs/architecture/evidence/pass-a-acceptance.md
docs/architecture/evidence/passb/b2-appconnector.md
docs/architecture/moves/appconnector.yaml
docs/architecture/passb/ownership-matrix.yaml
docs/plans/passb/27-appconnector.md
internal/handler/app_connector.go
internal/modules/appconnector/handler/app_connector.go
internal/modules/appconnector/handler/app_connector_action.go
internal/modules/appconnector/handler/app_connector_connection.go
internal/modules/appconnector/handler/app_connector_installation.go
internal/modules/appconnector/handler/app_connector_lifecycle_test.go
internal/modules/appconnector/handler/app_connector_oauth.go
internal/modules/appconnector/handler/app_connector_oauth_test.go
internal/modules/appconnector/handler/app_connector_oc.go
internal/modules/appconnector/handler/app_connector_oc_test.go
internal/modules/appconnector/handler/app_connector_sync.go
internal/modules/appconnector/handler/handler_helpers.go
internal/modules/appconnector/legacy/README.md
tools/passbguard/ownership_test.go

$ git diff 8c45a8815...HEAD --name-only | grep -E 'internal/router/|internal/container/|internal/bootstrap/|^go\.(mod|sum)$|^migrations/|execution-dag\.json'
（零命中；grep 退出码 1）                                # 禁改面零触碰
```

T4 提交后节点全量 diff 再增 2 个本节点产出文件：`docs/architecture/passb/briefs/b2-appconnector.md`、`docs/plans/passb/reports/b2-appconnector.md`（合计 21 文件）。

## 4. 变更文件 vs owned_files 逐条核对

**owned_files（DAG）**：`manifest:docs/architecture/moves/appconnector.yaml legacy_files 7 条` + `internal/modules/appconnector/**`；加 27 计划 §3 授权的节点自身产出新文件。

| 变更文件（21） | 归属核对 | 结论 |
|---|---|---|
| `internal/handler/app_connector.go` | legacy_files 7 条之一；重写为过渡 shim（conventions §7.1 薄 shim 授权，删除义务登记 Brief §1） | ⊆ owned |
| `internal/modules/appconnector/handler/app_connector{,_action,_connection,_installation,_oauth,_oc,_sync}.go`（7） | legacy_files 7 条 destination（rename R100，`git show f545d7d06 --summary` 实证 9 个 rename 100% + 2 个 create=handler_helpers.go 与 T1 测试文件经由宿主中转） | ⊆ owned |
| `internal/modules/appconnector/handler/app_connector_{oauth,oc,lifecycle}_test.go`（3） | framework:29 测试随迁 | ⊆ owned |
| `internal/modules/appconnector/handler/handler_helpers.go` | 27 计划 §4.2 授权新增 | ⊆ owned |
| `internal/modules/appconnector/legacy/README.md` | 27 计划 §3 授权回填（7 行迁移状态已回填） | ⊆ owned |
| `docs/architecture/evidence/passb/b2-appconnector.md` | 27 计划 §3 治理/证据产出 | ⊆ owned |
| `docs/architecture/passb/briefs/b2-appconnector.md` | 同上（T4） | ⊆ owned |
| `docs/plans/passb/reports/b2-appconnector.md` | 同上（T4，本文件） | ⊆ owned |
| `docs/plans/passb/27-appconnector.md` | 节点计划文档（提交 1，派发流程产出） | 节点自身产出 |
| `docs/architecture/evidence/pass-a-acceptance.md`（+7 行） | Ruling §3/§4 基线变更台账（conventions §8 流程，独立 commit 33f8c3ea3） | Ruling 批准 |
| `docs/architecture/moves/appconnector.yaml` / `docs/architecture/passb/ownership-matrix.yaml` | Ruling §1 同窗规则：6 条已迁行删除（`app_connector.go` 行保留至 shim 删除） | Ruling 批准 |
| `tools/passbguard/ownership_test.go` | Ruling §2：字面量计数 396→390 同窗纯数字修正 | Ruling 批准 |

**差集结论**：21 文件中 14 个 ⊆ owned_files 字面范围，1 个为计划 §3 授权的宿主 shim 重写，3 个为本节点产出文档，3 个为 Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 批准的治理/台账修正（超出 27 计划 §9 验收标准 2 原文「moves/appconnector.yaml 零出现」的部分，由协调者裁定显式批准，见 evidence §4；除此之外验收标准 2 的禁改清单——router/container/bootstrap/go.mod/go.sum/migrations/治理另三 YAML/execution-dag.json——零出现）。**非 Ruling 批准的越权文件：0。**

## 5. T4 本任务（B2-AC.4）自查

- **Integration Brief（27 计划 §7 七项）**：已定稿 `docs/architecture/passb/briefs/b2-appconnector.md`——§1 shim 删除清单、§2 IB2 调用点切换表（router.go :131-134/:413-417、routes_app_connectors.go :22-25、container.go :933-936/:981-999、open_connector_test.go :386-389）、§3 helper 三处副本收口申请、§4 exc-0058..0061 解除提案（a/b/c 三选项）、§5 airesource 过渡依赖、§6 别名 5 行核销申请、§7 契约回写申请 + IB2 执行顺序建议。
- **shim 面复核（27 计划验收 4）**：`go doc ./internal/handler` 输出 5 类型别名（`type AppActionHandler = appconnectorhandler.AppActionHandler` 等）+ 4 构造器 + `DefaultAppOAuthProviderConfigs`，与 `go doc ./internal/modules/appconnector/handler`（退出码 0；GOPROXY=off 提示为离线环境噪音，相对路径形式可解析）签名逐字一致；`newAppID`/`appRequireWriteCapability`/`OCPrepareInput`/`DecodeOCPrepare`/`appActionStates`/`appActionRisks` 未进 shim（shim 文件实读 :1-69 确认）。
- **模块根包导出面零变化（27 计划验收 5）**：evidence §3.3 已记录 `go doc ./internal/modules/appconnector` base/HEAD 双跑 diff 为空 + agentruntime 消费方 `go build` 退出码 0。
- **DAG 回填**：`task_ids`/`head_sha`/`status`/`review_status` 由协调者回填（conventions §9）；本节点未改 `execution-dag.json`（§3 grep 零命中实证）。

## 6. 未完成项与遗留义务（如实列出）

1. **exc-0058..0061 未删行**：3 文件 4 行 import 原样在位（evidence §3.2 实录）；解除提案登记 Brief §4，remove_at=ib2 期限裁决权归 IB2（裁定选项 a=列入 IB2 串行适配清单 / b=改期登记 / c=升级串行契约任务）。
2. **宿主 shim 未删**：`internal/handler/app_connector.go` 及其治理行保留至 IB2 完成装配切换（Brief §1/§2）。
3. **helper 三处副本未收口**：`appOK`/`appFail`/`appTenantScope`（模块原件 + 宿主 shim 副本 + 12-commercial 侧）与 `ErrMissingTenantScope`（宿主 commercial.go:34 + 模块 handler_helpers.go 副本）收口裁定归 IB2（Brief §3）。
4. **mcp_oauth 族过渡依赖**：模块 handler 包消费 11-airesource 属主 `internal/application/repository/mcp_oauth.go` 导出面，落位后由 airesource Integration Brief 承接（Brief §5）。
5. **别名 5 行核销回写**：空义务证据已落盘（evidence §3.1），回写归 IB2、B5 终验零残留（Brief §6）。
6. **契约回写**：`appconnector.facade/routes/lifecycle` 状态回写归 IB2（Brief §7）；门面 `module.go` 归 IB2 实现（本节点零变化）。
7. **DAG 字段回填**：归协调者（conventions §9）。
8. **正式三方计数复核**：633/23+23/58/16 本节点双跑一致，三方一致正式复核归 IB2/B5（conventions §8）。

## 7. 高风险差分证据指针（conventions §6，Tenant/RBAC 面）

`docs/architecture/evidence/passb/b2-appconnector.md`：§1 T1 宿主基线（8 命令 + 4 用例族断言锚点）；§2 T3 双跑比对（9 对命令 0/0、§2.3 用例矩阵逐行「等价」——OAuth 流 1、connection 分支族 6 断言组、installation 生命周期/写门/跨租户 404 3 断言组、sync 三态 3、action fail-closed 1、OC 全链 21 顶层用例、U05 计量 3、装配面 4、模块 6 包回归；§2.4 Tenant/RBAC 零改动判定口径）；§4 搬迁文件 R100 字节同一证据（cmp 逐文件，含 1 处转写误差抓出修正记录）。
