# Integration Brief — system (IA2)

适用对象：Pass A 集成者（Integrator）。任务：把 `bm-passa-a8` 分支的成果合入集成线。
本文件只描述 system 模块（manifest：`docs/architecture/moves/system.yaml`），
不动其他模块领地。

## 0. 集成动作：零搬迁（documentation-only）

**system 模块 Pass A 无任何整包搬迁**（manifest `move_packages: []`，`alias_obligations: []`）。
F0 判定 system 今日拥有 0 个专属包——其代码全部落在横向包（`internal/handler`、
`internal/application/service`、`internal/application/repository`）内，属 Pass B `B-system`
文件级拆分范畴。Pass A 禁止编辑共享横向包做结构拆分，故本模块对集成线的**代码 diff 为零**：
集成时本分支只新增本文档与 `docs/architecture/evidence/system.md`，无 move/repair commit，
无别名包，无 import 修复，`go.mod`/`go.sum` 无变化。共享文件（config/middleware/router/
container）一个都未触碰。

## 1. 当前资产位置（legacy_files，5 个非测试 .go；Pass B `B-system` 拆出）

| 文件 | 行数 | 承载内容 |
|---|---|---|
| `internal/application/repository/system_setting.go` | 90 | SystemSetting GORM 仓储 |
| `internal/application/service/system_setting.go` | 1398 | System Setting 服务（类型校验注册表、Redis pub/sub 失效广播） |
| `internal/handler/initialization.go` | 2612 | 初始化 / Ollama / 远程模型检测 / 抽取 handler |
| `internal/handler/system.go` | 2399 | 系统信息 / 存储引擎 / 系统管理 / runtime 队列 / settings handler |
| `internal/handler/deployment_capabilities.go` | 180 | 部署能力矩阵（GET /system/capabilities） |

随主题文件一并走的 `_test.go`（Pass B 同批搬，manifest 规则 3，不单列）：
- handler 侧 13 个：`deployment_capabilities{,_protocol}_test.go`、
  `initialization_{config_redaction,fill_secrets,model_tenant}_test.go`、
  `system_{admin_audit,info,parser_check,password_reset,runtime,storage_engine,storage_ssrf,user_create}_test.go`
- service 侧 1 个：`system_setting_validation_test.go`
- repository 侧：无（`system_setting.go` 无同名测试文件）

## 2. 路由集成点（routes；基线 per-owner 归 system 共 43 条）

| 注册入口 | 位置 | 条数 | 计数口径 |
|---|---|---|---|
| `RegisterInitializationRoutes` | `internal/router/routes_infra.go:102` | 17 | 全部 17 条为 `g.apiKeyRoute` helper（计入基线 69 条 apiKeyRoute） |
| `RegisterSystemRoutes` | `internal/router/routes_auth_tenant.go:245` | 8 | `apiKeyGroup` 上的字面量 gin 方法调用（/system 组，Viewer+/Admin+ 守卫） |
| `RegisterSystemAdminRoutes` | `internal/router/routes_auth_tenant.go:279` | 18 | 8 条字面量（promote/revoke/list/users/reset-password/users/create/api-keys GET+POST+DELETE，`g.SystemAdmin()` 组守卫）+ 10 条 `g.apiKeyRoute`（settings ×4、runtime/queues ×4、`/tenants/apply-default-storage-quota` ×1、`/audit-log` ×1） |

- 挂载点：`internal/router/router.go:385-388`（三条注册函数在 v1 组顺序调用）。
- **审计端点细分裂（F0 既定）**：`RegisterSystemAdminRoutes` 的 `GET /system/admin/audit-log`
  （`auditLogHandler.ListSystemAuditLog`，routes_auth_tenant.go 内 `if auditLogHandler != nil` 分支）
  域归 **identity**；基线 per-owner split（43）把该函数整体计入 system 并注明此分裂
  （backend-baseline.md §3.2 "RegisterSystemAdminRoutes (18)"）。
- handler 文件 ↔ 路由归属：`initialization.go` → 17 条（/initialization/*）；
  `system.go` → 其余 25 条中的 24 条（/system/info、parser-engines、storage-engine、
  sandbox-check、/system/admin/* 全部、runtime/*、settings/*、tenants/apply-default-storage-quota）；
  `deployment_capabilities.go` → 1 条（GET /system/capabilities，经 `GetDeploymentCapabilities`）。
- 注册函数本体位于 `internal/router/routes_infra.go` / `routes_auth_tenant.go`——两文件
  **不在禁改清单**（禁改仅 router.go/task.go/sync_task.go），Pass B 拆分时经 router 侧
  模块化改造迁移，本模块 Pass A 未触碰。

## 3. Workers

**0 项**（manifest `integration_points.workers: []`；system 无 asynq/Lite 任务类型）。
注意区分：`/system/admin/runtime/*` 路由是**观测** asynq 队列的 admin API（读 queue 深度、
安全投影、状态受控操作），不注册任何 worker；`system.go` 的 runtime 函数族
（GetRuntimeQueues/ListRuntimeTasks/MutateRuntimeTask/PurgeArchivedRuntimeTasks）只调用
queue inspector 端口。router/task.go、router/sync_task.go 无本模块条目。

## 4. container.Invoke 生命周期挂点（1 项，禁改文件内，集成时不要动语义）

| 挂点 | 位置 |
|---|---|
| `Invoke(startHousekeepingService)` | `internal/container/container.go:642`（前置 `Provide(service.NewHousekeepingService)` 在 :641） |

函数体在 `container.go:2384-2403`：`svc.Start(context.Background())` 启动 housekeeping cron，
`cleaner.RegisterWithName("KnowledgeHousekeeping", …)` 注册优雅停止；启动失败仅告警不阻断容器。

**跨域观察（记录给 IA2/Pass B，非本模块动作）**：该挂点经 `service.HousekeepingService` 落到
`internal/application/service/knowledge_housekeeping.go`——该文件的 legacy_files 归属
**knowledge** 模块（`docs/architecture/moves/knowledge.yaml:203`）。即 manifest 把 hook
集成点记在 system 名下（F0 §5.15/§5.18 的域职责），而实现文件按 F0 plurality 归 knowledge。
Pass A 双方都不动 container.go；Pass B 拆分时需要按此口径协调（hook 入口归 system 门面、
cron 实现随 knowledge 走）。

## 5. 配置键（模块读取/管理）

System Setting 服务（DB `system_settings` 表，运行时可调）：
- 键空间为通用 string/int/bool/string_list 四类型注册表；当前已知键：
  - `ssrf.whitelist`（service/system_setting.go:497）
  - `model.max_concurrency`（service/system_setting.go:499）
  - `sandbox.docker_enabled`（const `sandbox.DockerBackendEnabledSettingKey`，
    internal/sandbox/docker_enabled.go:22；读取点 service/system_setting.go:501,547）
- 失效广播 pub/sub channel：`weknora:system_settings:changed`（前缀
  `pubsubChannelBase`，service/system_setting.go:33）。
- API-key capability 对应：`types.APIKeyCapabilitySystemSettingsRead/Manage`、
  `SystemRuntimeRead/Manage`、`SystemTenantsManage`、`SystemAuditRead`（routes_auth_tenant.go
  的 `apiKeyPlatform(...)` 调用）。

handler 直读环境变量（system.go，探测/展示用）：
`DOCREADER_ADDR`、`DOCREADER_TRANSPORT`、`RETRIEVE_DRIVER`、`MINIO_ENDPOINT`、
`MINIO_ACCESS_KEY_ID`、`MINIO_SECRET_ACCESS_KEY` 及 TOS/COS/OSS/KS3/OBS/S3 同构键族
（is*Configured/is*EnvAvailable 探测，storage-engine-status/check 路由消费）。

## 6. 禁改共享文件中指向本模块资产的精确清单（file:line）

Pass A 未搬迁任何包，故**不存在**"旧导入路径引用"；此处列出的是禁改文件中本模块资产的
现位置，作为 Pass B `B-system` 的切换点清单：

- `internal/container/container.go:641` — `must(container.Provide(service.NewHousekeepingService))`
- `internal/container/container.go:642` — `must(container.Invoke(startHousekeepingService))`
- `internal/container/container.go:2384-2403` — `func startHousekeepingService(...)` 函数体
- `internal/router/router.go:385` — `RegisterInitializationRoutes(v1, params.InitializationHandler, rbacGuards)`
- `internal/router/router.go:387` — `RegisterSystemRoutes(v1, params.SystemHandler, rbacGuards)`
- `internal/router/router.go:388` — `RegisterSystemAdminRoutes(v1, params.SystemHandler, params.AuditLogHandler, rbacGuards)`

grep 已验证：`internal/router/task.go`、`internal/router/sync_task.go`、`go.mod`、`go.sum`
对本模块资产零引用（无专属包，无任务类型）。`migrations/` 内含本模块域 schema——
`migrations/versioned/000053_system_admin_and_settings.{up,down}.sql`（`users.is_system_admin`
列 + `system_settings` 表）——但 `migrations/` 全目录为本模块禁改项，Pass A 零触碰。

**给集成者的操作**：无需任何代码操作。按 §0，本分支合入后 system 相关代码与集成线逐字节一致。

## 7. 验收命令（集成后必跑）

- `go build ./...`
- `go test ./internal/modules/system/... -count=1`（manifest test_commands）
- `go test ./internal/handler/... ./internal/application/service/... ./internal/application/repository/... -count=1`（legacy 文件所在横向包回归）
- `go run ./tools/modulemove verify --module system`
- `go run ./tools/architectureguard`（基线：633 路由 / 23+23 worker / 58 hooks，不得变化）
