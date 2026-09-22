# datasource — Integration Brief（Pass A → IA1）

事实源：`docs/architecture/moves/datasource.yaml`。本文是 Pass A worker A3 交给集成任务
IA1 的交接说明：模块搬到了哪里、剩下哪些旧路径引用、IA1 需要按什么清单切换。
行为零变更（spec Pass A 全局约束）：路径/导入/组装之外没有任何改动。

## 1. 搬迁结果（from → to）

| 旧导入路径 | 新导入路径 |
|---|---|
| `internal/datasource` | `internal/modules/datasource` |
| `internal/datasource/connector/confluence` | `internal/modules/datasource/connector/confluence` |
| `internal/datasource/connector/dingtalk` | `internal/modules/datasource/connector/dingtalk` |
| `internal/datasource/connector/feishu/core` | `internal/modules/datasource/connector/feishu/core` |
| `internal/datasource/connector/feishu/drive` | `internal/modules/datasource/connector/feishu/drive` |
| `internal/datasource/connector/feishu/wiki` | `internal/modules/datasource/connector/feishu/wiki` |
| `internal/datasource/connector/gitlab` | `internal/modules/datasource/connector/gitlab` |
| `internal/datasource/connector/ima` | `internal/modules/datasource/connector/ima` |
| `internal/datasource/connector/moauth` | `internal/modules/datasource/connector/moauth` |
| `internal/datasource/connector/notion` | `internal/modules/datasource/connector/notion` |
| `internal/datasource/connector/rss` | `internal/modules/datasource/connector/rss` |
| `internal/datasource/connector/yuque` | `internal/modules/datasource/connector/yuque` |

包名（`package` 子句）全部不变；目录树内文件 100% 相似度纯改名（evidence 见
`docs/architecture/evidence/datasource.md`）。原 `internal/datasource/README.md` 落位为
`internal/modules/datasource/README.pkg.md`（模块骨架 `README.md` 已占用根名）；
`CONNECTOR_IMPLEMENTATION_GUIDE.md` 随树原样搬移。

## 2. 模块能力速览（IA1 装配需要的全部触点）

### 2.1 连接器注册表（connector registry）

- `internal/modules/datasource`：`ConnectorRegistry` / `NewConnectorRegistry()`；
  `registry.Register(connector)` 按 connector 名去重注册。
- 连接器构造（`initConnectorRegistry`，internal/container/container.go:2310）：
  `wiki.NewConnector(core.RegionFeishu|core.RegionLark)`、
  `drive.NewDriveConnector(core.RegionFeishuDrive|core.RegionLarkDrive)`（wiki/drive 共享
  `feishu/core` 的 `Client`/Region/导出逻辑）、`notion/yuque/ima/rss/gitlab/confluence/dingtalk`
  均为 `NewConnector()` 无参构造。
- `feishu/core`：`Client`（client.go）、四个 `Region` **var**（region.go:37/45/55/63，struct
  值、非 const）、`Fetch*` 引擎（engine.go）。

### 2.2 调度器（scheduler）

- `internal/modules/datasource/scheduler.go`：`Scheduler` / `NewScheduler(...)` /
  `Start(ctx)` / `Stop()` / `AddOrUpdate(ds)` / `Remove(id)` / `EntryCount()` /
  `SetSyncGate(SyncGateFunc)`。
- cron 触发 → `triggerSync` → 入队 `datasource:sync`：`asynq.Queue(types.QueueSync)`、
  `MaxRetry(5)`、`Timeout(2h)`、确定性 TaskID `dssync:<dataSourceID>:<UTC分钟>`（同分钟
  幂等去重；`ErrTaskIDConflict` 视为“已被其他实例入队”，SyncLog 置 canceled +
  "skipped: another sync already queued"）。

### 2.3 asynq/Lite worker（Redis 与 Lite 同集合）

| 任务类型 | 处理器 | Redis 注册 | Lite 注册 |
|---|---|---|---|
| `datasource:sync`（`types.TypeDataSourceSync`） | `DataSourceService.ProcessSync` | router/task.go:314 | router/sync_task.go:160 |
| `datasource:purge`（`types.TypeDataSourcePurge`） | `DataSourceService.ProcessDataSourcePurge` | router/task.go:295 | router/sync_task.go:161 |

入队方：scheduler（上表 §2.2）、`DataSourceService` 手动同步/重索引（QueueSync,
MaxRetry(5), Timeout(2h)）、删源级联 purge（QueueMaintenance, MaxRetry(3), Timeout(2h)）。
两个任务类型都在 IA1 的 Redis/Lite 拓扑恒等式内（architectureguard 现值 23+23）。

### 2.4 取消 / 凭据 / 重试钩子

- TaskInspector setter 注入：`DataSourceService.SetTaskInspector(interfaces.TaskInspector)`
  ——删除/暂停前硬取消已入队（及运行中）的 `datasource:sync`；Lite 模式注入 no-op
  inspector，service 内部退化为 sync-log 扫描。由 `injectDataSourceTaskInspector`
  （container.go:2366）在 `startDataSourceScheduler` 之前 Invoke（container.go:636）。
- 凭据刷新：`datasource.ErrCredentialRefreshRejected`（原 internal/datasource/errors.go，
  现 internal/modules/datasource/errors.go）；refresher 回调返回
  `(updated, nextRefreshAt, err)`（application/service/datasource_service.go:627 附近）。
- 重试语义=外部契约：`MaxRetry`/`Timeout`/TaskID 去重/cursor 续传（feishu drive
  `cursor_test.go` 等）一律不得在 Pass A/IA1 改动。

### 2.5 启动 / 生命周期挂点

| 挂点 | 位置 |
|---|---|
| Provide：`initConnectorRegistry` / `datasource.NewScheduler` / `service.NewDataSourceService` | container.go:631-633 |
| Invoke：`injectDataSourceTaskInspector` | container.go:636（func @2366） |
| Invoke：`startDataSourceScheduler` | container.go:637（func @2373；Start 失败仅 Warnf，清理名 `"DataSourceScheduler"` 注册到 ResourceCleaner） |

### 2.6 路由（1 个注册入口，20 条路由，文件计 20/0）

- 入口：`RegisterDataSourceRoutes` — internal/router/routes_infra.go:301（挂在
  `/api/v1/datasource`，`apiKeyManageDataSources` 门禁）。全部路由都在
  routes_infra.go 这一个文件（`internal/handler/datasource*.go` 只有 handler 方法，
  无路由注册）。
- 明细：`GET /types`(Viewer)、`POST /validate-credentials`(Admin)、CRUD
  `POST/GET ""`、`GET/PUT/DELETE /:id`、`GET /:id/documents-count`(Viewer)、
  `PUT /:id/credentials`、`DELETE /:id/credentials/:field`、`POST /:id/validate`、
  `GET /:id/resources`、`POST /:id/resource-ancestors`、`POST /:id/sync`、
  `POST /:id/reindex`、`POST /:id/pause`、`POST /:id/resume`、`GET /:id/logs`(Viewer)、
  `GET /logs/:log_id`(Viewer)、`POST /:id/logs/:log_id/cancel`(Admin) — 共 20 条。
- handler 侧遗留文件（Pass B `B-datasource` 拆分，Pass A 未动）：
  `internal/handler/datasource.go`、`internal/handler/datasource_credentials.go`；
  service 侧：`internal/application/service/datasource_service.go`；
  repo 侧：`internal/application/repository/datasource_repo.go`。

### 2.7 配置键

搬移树自身不读取任何 `DATASOURCE_*` 环境变量；连接器配置经
`types.DataSource.ConfigJSON`（feishu core 的 `config.AppID/AppSecret/Timezone/
ResourceIDs/MultimodalEnabled` 等来自 DB 中的数据源配置，`config.GetBaseURL()` 为
platform 回调基址）。模块无新增 config.go 键。

## 3. 旧路径残留引用清单（IA1 切换指引）

非禁改文件的导入已在 Pass A 修复完毕；**全部**残留旧导入路径的引用都集中在
`internal/container/container.go`（禁改文件，Pass A 未动一字节）：

| 文件:行 | 旧引用（import 或标识符） |
|---|---|
| internal/container/container.go:67 | import `github.com/Tencent/WeKnora/internal/datasource` |
| internal/container/container.go:68 | import `.../internal/datasource/connector/confluence`（别名 `confluenceConnector`） |
| internal/container/container.go:69 | import `.../internal/datasource/connector/dingtalk`（别名 `dingtalkConnector`） |
| internal/container/container.go:70 | import `.../internal/datasource/connector/feishu/core` |
| internal/container/container.go:71 | import `.../internal/datasource/connector/feishu/drive` |
| internal/container/container.go:72 | import `.../internal/datasource/connector/feishu/wiki` |
| internal/container/container.go:73 | import `.../internal/datasource/connector/gitlab`（别名 `gitlabConnector`） |
| internal/container/container.go:74 | import `.../internal/datasource/connector/ima`（别名 `imaConnector`） |
| internal/container/container.go:75 | import `.../internal/datasource/connector/notion`（别名 `notionConnector`） |
| internal/container/container.go:76 | import `.../internal/datasource/connector/rss`（别名 `rssConnector`） |
| internal/container/container.go:77 | import `.../internal/datasource/connector/yuque`（别名 `yuqueConnector`） |
| internal/container/container.go:632 | `datasource.NewScheduler`（Provide） |
| internal/container/container.go:2310 | `*datasource.ConnectorRegistry`（返回类型） |
| internal/container/container.go:2311 | `datasource.NewConnectorRegistry()` |
| internal/container/container.go:2314 | `wiki.NewConnector(core.RegionFeishu)` |
| internal/container/container.go:2318 | `wiki.NewConnector(core.RegionLark)` |
| internal/container/container.go:2324 | `drive.NewDriveConnector(core.RegionFeishuDrive)` |
| internal/container/container.go:2327 | `drive.NewDriveConnector(core.RegionLarkDrive)` |
| internal/container/container.go:2330 | `notionConnector.NewConnector()` |
| internal/container/container.go:2333 | `yuqueConnector.NewConnector()` |
| internal/container/container.go:2336 | `imaConnector.NewConnector()` |
| internal/container/container.go:2339 | `rssConnector.NewConnector()` |
| internal/container/container.go:2342 | `gitlabConnector.NewConnector()` |
| internal/container/container.go:2345 | `confluenceConnector.NewConnector()` |
| internal/container/container.go:2348 | `dingtalkConnector.NewConnector()` |
| internal/container/container.go:2373 | `*datasource.Scheduler`（参数类型） |

注：container.go:2322 注释里出现 `core.Client`，仅为注释，不构成代码引用，未做别名。

### IA1 切换步骤（机械操作）

1. container.go:67-77 的 11 条 import 改为 `github.com/Tencent/WeKnora/internal/modules/datasource`
   前缀，import 别名（`confluenceConnector` 等）保持不变（新包名与旧别名兼容，
   `core`/`drive`/`wiki` 无别名直接可用）。
2. 标识符（§3 表中 632/2310/2311/2314-2348/2373 行）无需改动：别名包已提供同名
   type alias / var / const-型 var 转发，或直接把 `datasource.` 前缀改到新路径导入名。
3. 删除 12 个别名包（manifest `alias_obligations`，Pass B 任务 `B-datasource`）：
   `internal/datasource`（仅 `alias.go`）与 `internal/datasource/connector/{confluence,
   dingtalk,feishu/core,feishu/drive,feishu/wiki,gitlab,ima,moauth,notion,rss,yuque}`
   （各仅 `alias.go`；moauth 为最小包文件，无转发面）。
4. 每删一步跑：`go build ./...` + `go test ./internal/modules/datasource/... -count=1`
   + `go run ./tools/modulemove verify --all`（alias 1:1 校验在别名全删后才不再要求
   datasource 清单的旧路径）。

## 4. 验收命令（后搬迁路径）

- `go test ./internal/modules/datasource/... -count=1`（manifest test_commands）
- 直接消费方：`go test ./internal/application/service/ ./internal/handler/ -count=1`
  （Pass A 用 `-run` 限定了 109 个 datasource 消费测试，见 evidence）
- `go run ./tools/modulemove verify --module datasource`
- `go build ./...`、`go vet ./internal/modules/datasource/...`（touched areas）
