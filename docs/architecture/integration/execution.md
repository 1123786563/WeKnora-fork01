# Integration Brief — execution 模块（Pass A batch A2, Task A5）

> 读者：Integration Agent（IA2）与 Pass B worker。本文是 Task A5 搬迁后 execution
> 模块对外集成面的事实快照。范围事实源：`docs/architecture/moves/execution.yaml`；
> 行为与结构在 Pass A 中未改变，仅包路径变化。

## 1. 本次搬迁（A5 已完成，SHAs 见 evidence 文档）

| from（旧导入路径） | to（新导入路径） |
|---|---|
| `internal/execution` | `internal/modules/execution`（并入模块根，与 module.go 骨架同包） |
| `internal/sandbox` | `internal/modules/execution/sandbox` |
| `internal/browserskill` | `internal/modules/execution/browserskill` |

共 142 个文件（execution 17、sandbox 107、browserskill 18；含全部 `_test.go` 72 个与
`execution/testdata/start-command.json` fixture）。move commit 为纯 rename
（`git diff --summary` 142/142 `rename ... (100%)`，fixture 在 repair commit 内二次
`git mv` 进 `testdata/` 子目录，同为 100% rename）。

包名不变：`execution` / `sandbox` / `browserskill`。三个包互相之间**无**导入；
它们只导入 platform 包（`internal/types`、`internal/logger`、
`internal/tracing/langfuse`、`internal/common/redislock`）和 `internal/ipclass`（见 §7）。

## 2. 旧路径别名（alias_obligations，3 处）

每个旧路径留了一个**零逻辑**转发别名包（type alias + var 转发；文件头部注明
`Deleted by Pass B task B-execution`）：

| 旧路径（别名包目录） | 转发目标 | 别名文件（覆盖符号） |
|---|---|---|
| `internal/execution` | `internal/modules/execution` | `alias.go`（1：`NewRegistrationService`） |
| `internal/sandbox` | `internal/modules/execution/sandbox` | `alias.go`（1：`ConfigureDockerResourceProtection`） |
| `internal/browserskill` | `internal/modules/execution/browserskill` | `alias.go`（3：`Manager`、`NewManager`、`NewStore`） |

**覆盖面说明（与 A1 先例不同，刻意收窄）**：任务指令要求别名只服务禁改共享文件。
本模块的唯一禁改引用方是 `internal/container/container.go`（`internal/router/router.go`、
`task.go`、`sync_task.go` 经 re-grep 确认零引用），其全部引用即上表 5 个符号。
若 IA2 发现新的禁改方引用更多符号，可用 `go/types` 枚举搬迁包导出符号补齐别名；
这不改变别名"零逻辑"性质。

## 3. 禁改共享文件中仍指向旧路径的精确行（file:line 清单 + 切换指引）

本分支 HEAD 上，禁改共享文件中引用旧导入路径的行**只剩**：

| 文件:行 | 现内容 | 切换为 |
|---|---|---|
| `internal/container/container.go:57` | `"github.com/Tencent/WeKnora/internal/browserskill"` | `"github.com/Tencent/WeKnora/internal/modules/execution/browserskill"` |
| `internal/container/container.go:63` | `"github.com/Tencent/WeKnora/internal/execution"` | `"github.com/Tencent/WeKnora/internal/modules/execution"` |
| `internal/container/container.go:106` | `"github.com/Tencent/WeKnora/internal/sandbox"` | `"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"` |

切换步骤（Pass B task **B-execution**，非 Pass A）：
1. 修改上述 3 行 import 路径；
2. 确认 `browserskill.{Manager,NewManager,NewStore}`、`execution.NewRegistrationService`、
   `sandbox.ConfigureDockerResourceProtection` 引用可解析（新包同名同签名，无需改调用点）；
3. `git rm -r internal/execution internal/sandbox internal/browserskill`（三个别名目录）；
4. `go build ./...` + `go vet ./internal/container/...` 验证。

## 4. 路由（8 项入口，manifest 行号已在 A1 后漂移，以下为本分支实测行号）

handler 侧实现仍在横向 host 包（`internal/handler/*`、`internal/handler/session/*` 的
legacy 文件，Pass B 拆分），路由**调用点**全部位于 `internal/router`（已修复 import）：

| 注册入口 | 本分支调用点 | 说明 |
|---|---|---|
| `RegisterMyEnvVarRoutes` | `internal/router/router.go:383`（定义 `routes_auth_tenant.go:179`） | `/me/env-vars`（me_env_var handler） |
| `RegisterSandboxTerminalRoutes` | `internal/router/router.go:227`（定义 `routes_chat.go:258`） | 交互终端 WebSocket `/api/v1/sessions/:id/sandbox/terminal`（routes_chat.go:272） |
| `RegisterLocalBrowserRoutes` | `internal/router/router.go:241`（定义 `routes_chat.go:275`） | 扩展网关 ×3（routes_chat.go:283-285） |
| `RegisterMyBrowserRoutes` | `internal/router/router.go:325`（定义 `routes_chat.go:288`） | `/me/browser` ×2（routes_chat.go:293-294） |
| `RegisterSandboxConfigRoutes` | `internal/router/router.go:382`（定义 `routes_infra.go`） | 租户 sandbox 配置 |
| `RegisterExecutionRegistrationRoutes` | `internal/router/router.go:368` | target 注册挑战/完成/吊销 |
| execution target groups | `routes_workbench.go:60,65`（`RegisterWorkbenchRoutes` 内，经 `router.go:366` 挂载）；`routes_workbench.go:107,110`（registrations 组） | `/execution-targets` ×2、`/execution-workspaces` ×1 |
| session 子路由 ×3 | `routes_chat.go:115,116`（local-browser GET/POST）、`routes_chat.go:129`（terminal-ticket POST） | `RegisterSessionRoutes` 内 |

architectureguard 实测：`routes total=633`，与基线一致，0 violations。

## 5. Workers（asynq Redis + Lite）

manifest `workers: []` —— execution 模块**不注册任何 asynq 任务处理器**。
`internal/router/task.go`、`sync_task.go` 经 grep 确认无 sandbox/execution/browserskill
引用。清理（cleanup/orphan-reaper/idle-sweeper）均为进程内 goroutine 或调用方驱动，
不入 asynq 队列。

## 6. Lifecycle hooks（container.Invoke）

manifest `lifecycle_hooks: []`。`internal/container/container.go` 对本模块只有 3 处
**依赖装配**（非生命周期挂点）：

- `container.go:548-550`：Provide `*browserskill.Manager`（`browserskill.NewManager(browserskill.NewStore(db))`）；
- `container.go:740`：Provide `execution.NewRegistrationService`；
- `container.go:2464,2467`：启动时调用 `sandbox.ConfigureDockerResourceProtection(repo/nil)`（Docker 资源防护一次性装配）。

## 7. 外部依赖与配置键

- **platform 依赖**：`internal/types`、`internal/logger`、`internal/tracing/langfuse`、`internal/common/redislock`。
- **跨模块依赖（IA2 关注点）**：`internal/modules/execution/sandbox/url_guard.go` 导入
  `internal/ipclass`——该包归 **policy 模块**（`moves/policy.yaml`：`internal/ipclass` →
  `internal/modules/policy/ipclass`，Pass A batch A2 的姊妹任务 A8 搬迁）。A8 搬迁后需
  同步修复本文件这一行 import（属 ipclass 的 importer 修复面，非本模块动作）。
- **配置键**（`internal/config`，仅列组）：
  - Docker：`Docker`、`DockerHost`、`DockerImage`、`DockerCPULimit`、`DockerMemoryBytes`、`DockerPidsLimit`、`DockerRuntime`、`DockerNetworkMode`、`DockerHTTPTimeout`、`DockerTLSCertPath`、`DockerIdleTTL`；
  - Cube：`CubeAPIURL`、`CubeAPIKey`、`CubeProxyURL`、`CubeSandboxDomain`、`CubeSandboxTTL`、`CubeTemplate`、`CubeDNSServers`、`CubeHTTPTimeout`；
  - E2B：`E2BAPIURL`、`E2BAPIKey`、`E2BProxyURL`、`E2BSandboxDomain`、`E2BSandboxTTL`、`E2BTemplate`、`E2BHTTPTimeout`；
  - 其他：`SkillDir`、`SkillImage`、`TerminalIdleDisconnect`、`AllowPrivateEndpoints`。

## 8. 横向包内遗留文件（Pass B 拆分，本任务未动）

`internal/application/repository/{execution_cleanup,execution_dispatch,execution_observation,
execution_target,tenant_sandbox_config}.go`、`internal/application/service/{sandbox_terminal_*,
tenant_sandbox_*,user_env*}.go`、`internal/handler/{execution_registration,execution_target,
me_env_var,sandbox_check,sandbox_config,sandbox_skill}.go`、
`internal/handler/session/{browserskill,sandbox_terminal_bridge,sandbox_terminal_ws}.go` ——
见 manifest `legacy_files`（22 文件），Pass B task B-execution 按文件拆出。
