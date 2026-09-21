# Integration Brief — appconnector (IA1)

适用对象：Pass A 集成者（Integrator）。任务：把 bm-passa-a1 分支的搬迁成果合入集成线。
本文件只描述 appconnector 模块（manifest：`docs/architecture/moves/appconnector.yaml`），
不动其他模块领地。

## 1. 本次搬迁（A1 已完成，SHAs 见 evidence 文档）

| from（旧导入路径） | to（新导入路径） |
|---|---|
| `internal/appconnector` | `internal/modules/appconnector` |
| `internal/appconnector/openconnector` | `internal/modules/appconnector/openconnector` |
| `internal/application/repository/appconnector` | `internal/modules/appconnector/repository/appconnector` |
| `internal/application/service/appconnector` | `internal/modules/appconnector/service/appconnector` |
| `internal/connectorcontrol` | `internal/modules/appconnector/connectorcontrol` |

共 65 个文件（含全部 `_test.go`），move commit 为纯 rename（65/65 相似度 100%）。

## 2. 旧路径别名（alias_obligations，5 处）

每个旧路径留了一个**零逻辑**转发别名包（type alias + var/const 转发；每文件头部注明
`Deleted by Pass B task B-appconnector`）：

| 旧路径（别名包目录） | 转发目标 | 别名文件 |
|---|---|---|
| `internal/appconnector` | `internal/modules/appconnector` | `alias.go`（141 符号） |
| `internal/appconnector/openconnector` | `internal/modules/appconnector/openconnector` | `alias.go`（19 符号） |
| `internal/application/repository/appconnector` | `internal/modules/appconnector/repository/appconnector` | `alias.go`（42 符号） |
| `internal/application/service/appconnector` | `internal/modules/appconnector/service/appconnector` | `alias.go`（112 符号） |
| `internal/connectorcontrol` | `internal/modules/appconnector/connectorcontrol` | `alias.go`（52 符号） |

别名面 = 搬迁包的全部导出符号（`go/types` 枚举生成），覆盖禁改文件的全部引用
（已核对 `internal/container/container.go` 引用的 repoappconn.{InstallationStore, OCStore,
NewActionStore, NewInstallationStore, NewOCStore} 与 appconnectorsvc.{A02Guard, ActionService,
ActionStoreSource, ConnectionCredentialSource, NewInstallationStateSource, NewOCSubjectGuard,
OCConnectionService, OCPreparer} 全部可解析）。

## 3. 禁改共享文件中仍指向旧路径的精确行（file:line 清单）

Pass A 结束时（本分支 HEAD），禁改共享文件中引用旧导入路径的行**只剩**：

- `internal/container/container.go:40` — `repoappconn "github.com/Tencent/WeKnora/internal/application/repository/appconnector"`
- `internal/container/container.go:54` — `appconnectorsvc "github.com/Tencent/WeKnora/internal/application/service/appconnector"`

`internal/router/router.go`、`internal/router/task.go`、`internal/router/sync_task.go`、
`go.mod`、`go.sum`、`migrations/`：零处引用（已 grep 验证；router 经 `internal/handler`
间接使用本模块，无直接导入）。

**给集成者的操作**：
1. 在 `internal/container/container.go:40` 把导入改为
   `repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"`；
2. 在 `internal/container/container.go:54` 把导入改为
   `appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"`；
   仅改 import 行，qualifier 不变，`container.go` 其余行零改动。
3. 改完后运行 `go build ./...` 通过，再删除第 2 节所列 5 个别名目录（连同 alias.go），
   再次 `go build ./...` + `go test ./internal/modules/appconnector/... -count=1` 确认。
   （其余 4 个别名目录当前无任何 importer，也可在集成时直接删除；仅
   `internal/application/{repository,service}/appconnector` 两个别名在步骤 1-2 之前必须保留。）

## 4. 路由集成点（routes）

唯一入口：`RegisterAppConnectorRoutes` — `internal/router/routes_app_connectors.go:20`，
挂载于 `/api/v1` 下的 `/apps` 组（注意：该组**不在** API-key 路由授权器中，默认对
X-API-Key 主体 403）。共 **17** 条路由，按 handler 文件归属：

| handler 文件（legacy，Pass B 拆分） | 方法数 | 承载路由 |
|---|---|---|
| `internal/handler/app_connector_installation.go` | 9 methods / 1 exported ctor | 4：installations GET""、POST""、`:id/upgrade`、`:id/disable`；另有 catalog GET `/apps/catalog` |
| `internal/handler/app_connector_connection.go` | 4 / 1 | 4：connections GET""、POST""、`:id/authorization-attempts`、`:id/revoke` |
| `internal/handler/app_connector_action.go` | 8 / 1 | 3：actions GET `:id`、POST `:id/approve`、POST `:id/execute`；另 oc prepare POST `/apps/oc/actions/prepare` |
| `internal/handler/app_connector_sync.go` | 2 / 1 | 1：GET `/apps/datasources/:id/sync-status` |
| `internal/handler/app_connector_oauth.go` | 5 / 1 | 1：GET `/apps/connections/oauth/callback` |
| `internal/handler/app_connector_oc.go` | 6 / 1 | 2：GET `/apps/authorization-attempts/:id`、GET `/apps/catalog`（与 installation 共享 1 条） |
| `internal/handler/app_connector.go` | 0 / 0 | 共享 helper（appOK/appFail 等），无路由 |

注：`handler.NewAppInstallationHandler`/`NewAppConnectionHandler`/`NewAppSyncHandler`/
`NewAppActionHandler` 构造器在 Pass A 不动（ trapped in 横向 `internal/handler`，
Pass B `B-appconnector` 拆出）；`internal/handler` 包内 7 个 legacy 文件只做了 import
行修复（A1 已完成），集成时无需再动。

## 5. Workers（Redis + Lite 任务类型）

**0 项**（manifest `integration_points.workers: []`，已验证搬迁代码无 asynq 引用）。
模块无 asynq 任务类型，无 mux.HandleFunc 注册，无需改 router/task.go、router/sync_task.go。

## 6. container.Invoke 生命周期挂点（5 项，禁改文件内，集成时不要动语义）

| 挂点 | 位置 |
|---|---|
| `Invoke` wire AppActionHandler+ActionService（`h.SetActionService`） | `internal/container/container.go:980` |
| `Invoke` wire AppActionHandler+OCPreparer（`h.SetOCPreparer`） | `internal/container/container.go:983` |
| `Invoke` wire AppActionHandler+OCConnectionService（`h.SetOCConnectionService`） | `internal/container/container.go:986` |
| `Invoke` startOCRecoveryRunner（函数体在 `internal/container/open_connector.go:683`） | `internal/container/container.go:989` |
| `Invoke` wire AppConnectionHandler（A02 OAuth provider 注册） | `internal/container/container.go:994` |

相关 Provide（同为 container.go，非 Invoke）：`container.go:952`（NewActionStore→ActionStoreSource）、
`container.go:953`（MCPOAuthBindingStore→ConnectionCredentialSource）、`container.go:962-965`
（A02Guard/subject guard）。以上行在 Pass A 后仍通过别名解析；集成者按第 3 节仅改两行 import
即可全部切到新路径。

## 7. 配置键（模块读取）

运行时（server 进程）：
- `WEKNORA_OC_TRANSIENT_KEY` — `internal/modules/appconnector/service/appconnector/oc_connections.go:154`（const `TransientCipherEnvKey`）
- `WEKNORA_APP_OAUTH_<APP>_CLIENT_ID` / `WEKNORA_APP_OAUTH_<APP>_SECRET` — `internal/container/container.go:996` 附近（OAuth provider 注册，机制属 container，键归本模块域）

伴生命令行工具 `cmd/connector-control`（out of F0 scope，仅 import 修复）：
`DB_DRIVER`、`DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD`、`DB_NAME`、
`CONNECTOR_CONTROL_RUNTIME_ALLOWLIST`、`CONNECTOR_CONTROL_SECRET_KEY_FILE`。

## 8. 验收命令（集成后必跑）

- `go build ./...`
- `go test ./internal/modules/appconnector/... -count=1`
- `go test ./internal/container/... ./internal/handler/... ./internal/application/repository/... ./internal/application/service/... ./internal/agent/tools/... -count=1`
- `go run ./tools/modulemove verify --module appconnector`
- `go run ./tools/architectureguard`（基线：633 路由 / 23+23 worker / 58 hooks，不得变化）
