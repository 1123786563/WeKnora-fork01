# Integration Brief — b2-appconnector（交付 IB2 的装配变更申请）

> 节点：`b2-appconnector`（Pass B 27 计划：7 legacy handlers 迁入 `internal/modules/appconnector/handler` + 过渡 shim + 差分证据 + 别名/例外核销登记）。
> 分支：`codex/passb-b2-appconnector`；节点分叉点（base）`8c45a8815`（ib1 登记提交）；
> 本 Brief 定稿于节点 HEAD（T4 提交，见报告提交序列）。
> 证据源：`docs/architecture/evidence/passb/b2-appconnector.md`（差分 §2、别名/例外 §3、计数 §5、Ruling §4）；
> 实施报告：`docs/plans/passb/reports/b2-appconnector.md`。
> IB2 按本 Brief 在 guard 口径下串行单写集成独占文件（conventions §3），并回写治理 YAML 状态。

## 1. shim 删除清单

`internal/handler/app_connector.go`（唯一宿主残件，本节点重写为过渡 shim）：

- 5 类型别名：`AppInstallationHandler` / `AppConnectionHandler` / `AppSyncHandler` / `AppActionHandler` / `AppOAuthProviderConfig`（= `appconnectorhandler.*`，shim :22-28）；
- 4 构造器转发：`NewAppInstallationHandler` / `NewAppConnectionHandler` / `NewAppSyncHandler` / `NewAppActionHandler`（shim :30-41）；
- `DefaultAppOAuthProviderConfigs` 转发（shim :43-45）；
- 宿主 helper 副本 3 个：`appOK` / `appFail` / `appTenantScope`（shim :54-69；`ErrMissingTenantScope` 沿用宿主 `commercial.go:34` 原哨兵，未新增副本）。

IB2 完成 §2 调用点切换后**同窗删除本文件**（连同 moves/appconnector.yaml 与 ownership-matrix.yaml 中保留的 `internal/handler/app_connector.go` 行——该行按 Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP §1 保留至 shim 删除）。

## 2. IB2 需切换的调用点（集成工程师独占文件，本节点零触碰）

| 文件 | 位置 | 现状（经 shim） | 切换为 |
|---|---|---|---|
| `internal/router/router.go` | :131-134 `RouterParams` 4 字段 | `*handler.App{…}Handler` | `*appconnectorhandler.App{…}Handler` |
| `internal/router/router.go` | :413-417 `RegisterAppConnectorRoutes` 调用 | 同上类型经别名 | 同上 |
| `internal/router/routes_app_connectors.go` | :22-25 4 个 handler 参数 | `*handler.App{…}Handler` | 同上 + import 路径 |
| `internal/container/container.go` | :933-936 | 4 个 dig `Provide(handler.NewApp{…}Handler)` | `Provide(appconnectorhandler.NewApp{…}Handler)` |
| `internal/container/container.go` | :981-999 | Invoke 块 `handler.AppActionHandler`/`AppConnectionHandler` + `handler.DefaultAppOAuthProviderConfigs`/`handler.AppOAuthProviderConfig`（:996/:999） | 模块路径 |
| `internal/container/open_connector_test.go` | :386-389 | 4 个构造器引用 | 模块路径 |

切换依据：shim 面与模块包导出面经 `go doc` 双跑逐字一致（evidence §4/报告验收 4；别名解析到同一类型 ⇒ dig Provide/Invoke 装配语义零变化）。切换后 `appOK`/`appFail`/`appTenantScope` 宿主副本随 shim 文件一起删除（消费方 commercial_task_budget.go、craft_model_gateway.go 的宿主内副本收口见 §3）。

## 3. 宿主 helper 副本收口申请（conventions §7.1 多属主重复 helper 族）

现存三处 `appOK`/`appFail`/`appTenantScope` 同族副本（+`ErrMissingTenantScope` 哨兵两处）：

| 处 | 位置 | 写入者 |
|---|---|---|
| 模块原件 | `internal/modules/appconnector/handler/app_connector.go`（helpers）+ `handler_helpers.go` 的 `ErrMissingTenantScope` 副本 | 本节点 |
| 宿主 shim 副本 | `internal/handler/app_connector.go:54-69` | 本节点 |
| 12-commercial 副本 | 宿主 `internal/handler/commercial.go:34`（`ErrMissingTenantScope` 原件）+ 其 `handler_helpers.go` 宿主副本；未来 `modules/commercial/handler` 副本归 12-commercial 计划 §4.5 | 12-commercial（B2-CM） |

申请 IB2 裁定单一归宿并登记删除批次。裁定建议输入：三族符号仅经 `.Error()`/`c.JSON` 进响应体，全仓无 `errors.Is` 跨值比较（evidence §2.4）⇒ 副本合并无可观察行为差异；语义归属 appconnector/commercial 各自 handler 模块 + 宿主 shim 过渡期并存。宿主 shim 删除（§1）即消去宿主副本之一；`commercial.go:34` 原件与 12-commercial 未来副本的合并属 12-commercial 边界，IB2 统筹两节点批次即可。

## 4. exc-0058..0061 解除提案（remove_at=ib2，期限裁决权归 IB2）

现状：3 文件 4 行 import 原样在位（evidence §3.2 grep 实录），消费符号面 `commercial.ExecutionGate`/`BudgetRequest`/`UsageFact`/`Credits`/`ErrInsufficientBudgetGate`/`FundingPlatform`/`ServiceConnector`/`DimensionConnector`/`UsageStatusFinal` + `commsvc.ErrGateReservationUnknown` 零变化。B1-CM 零实施 ⇒ 商业门面在当前树不存在；conventions §5 禁止本节点新增例外/自行实现上游门面 ⇒ 本节点不删行（13-execution EX-8 Step 4 先例）。

**解除提案（供 IB2 裁定执行方式）**：

1. **appconnector 侧消费方端口**（Spec §4.2「接口优先定义在使用方模块」）：在 `internal/modules/appconnector` 定义 `ExecutionGate` 同形接口（方法集按 action.go:163 字段实际消费面）与本模块 `BudgetRequest`/`UsageFact` 值类型；adapter.go/action.go/oc_recovery.go 改依赖端口，import commercial 全部消除。此改造属**新串行契约任务**（需 commercial 侧类型适配器在装配处桥接），不在本节点范围（27 计划 §10 声明）。
2. **commercial 适配器落点**：宿主装配层 `internal/container`（IB2 在装配处提供 `commercial.ExecutionGate` → appconnector 端口的适配器），或裁定 commercial 门面落地后由其提供适配实现。
3. **IB2 裁定选项**：(a) 按提案列入 IB2 串行适配清单（本节点请求此选项，remove_at=ib2 到期）；(b) 裁定改期（更新 remove_at，走 conventions §8 基线变更登记）；(c) 升级串行契约任务（framework:26）。若 (b)/(c)，请 IB2 在 exception-ledger 回写状态并在 DAG notes 登记。

## 5. airesource 过渡依赖登记

模块 handler 包消费 `internal/application/repository/mcp_oauth.go`（11-airesource 属主 legacy 文件）导出面：`repocommercialmcp.MCPOAuthBindingStore`、`NewMCPOAuthBindingStore`（connection.go:25/:35）、`mcprepo.ErrOAuthBindingInvalid`、`ErrOAuthBindingActorNotMember`（oauth.go:241/:243）。该文件 B 期将迁入 `internal/airesource`；请求 airesource 计划落位后由其 Integration Brief 提供导出面（或别名义务），本节点零改动该文件。import 链 `modules/appconnector/handler → application/repository → modules/appconnector` 为 A→B→C 无环（27 计划 §2.2 实读）。

## 6. 别名行核销申请

ownership-matrix :2463-2543 的 5 条 alias 行（`internal/appconnector`、`internal/appconnector/openconnector`、`internal/application/repository/appconnector`、`internal/application/service/appconnector`、`internal/connectorcontrol`，plan=27，delete_barrier=ib2）与 moves/appconnector.yaml 5 条 alias_obligations 在当前树为**空义务**：

- `git ls-tree HEAD` 5 路径全空；旧 import path 全仓 grep 零命中；Pass A 集成提交 `b0ef8895a` 删除 418 行 alias 并切装配（evidence §3.1 三命令实录）。

本节点不重建、不改治理 YAML。**申请 IB2 在 guard 口径下回写核销**（ownership-matrix 行状态置已核销/删除），B5 以「零残留」终验。

## 7. 契约回写申请（barrier 独占，本节点不写）

- `appconnector.facade`：characterization/consumers 如需登记新 handler 包路径（`internal/modules/appconnector/handler`，7 生产文件 + 3 测试文件 + helpers），由 IB2 按 contracts.yaml 回写规则处理。门面实现归 IB2（13-execution §2 先例），本节点未写 `module.go`（零变化）。
- `appconnector.routes`：consumers 注释路径（routes_app_connectors.go 经 shim 引模块类型）——17 条注册面零变化（evidence §5.2：router/container/bootstrap 零出现；`make check-backend-architecture` 633/23+23/58/16 与基线逐字一致），由 IB2 复核回写。
- `appconnector.lifecycle`：5 挂点（container.go :980/:983/:986/:989/:994，含 `startOCRecoveryRunner` @ open_connector.go:683）注册面零变化，同上复核。
- workers：空集契约不受影响（本节点零 worker 改动）。

---

**附：IB2 执行顺序建议**：§2 调用点切换（单 commit）→ shim 文件 + §1 清单 + `app_connector.go` 治理行同窗删除（单 commit）→ §3 helper 收口裁定登记 → §4 提案裁定（a/b/c 三选一）→ §6 别名核销回写 → §7 契约回写。
