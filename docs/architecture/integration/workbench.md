# Integration Brief — workbench 模块（Pass A batch A4 二批, Task A12）

> 读者：Integration Agent（IA4）与 Pass B worker。本文是 Task A12 搬迁后 workbench
> 模块对外集成面的事实快照。范围事实源：`docs/architecture/moves/workbench.yaml`；
> 行为与结构在 Pass A 中未改变，仅包路径变化。

## 1. 本次搬迁（A12 已完成，SHAs 见 evidence 文档）

| from（旧导入路径） | to（新导入路径） | 文件数 |
|---|---|---|
| `internal/application/service/workbench` | `internal/modules/workbench/service/workbench` | 22 |
| `internal/notification` | `internal/modules/workbench/notification` | 4 |
| `internal/voice` | `internal/modules/workbench/voice` | 3 |
| `internal/workbench` | `internal/modules/workbench`（并入模块根，与 module.go 骨架同包） | 7 |

共 36 个文件（含全部 `_test.go` 17 个），无非 Go 文件。move commit 为纯 rename
（`git diff --summary` 36/36 `rename`：30×100%、5×99%、1×97%；低于 100% 的 6 个
文件全是 import 行/测试相对路径修复所致，逐项见 evidence §4）。包名不变：
`workbench` / `notification` / `voice`。四个包之间在搬迁后**无**互相导入遗留
（service/workbench 经 `internal/modules/workbench` 别名前路径引用 contracts 的两处
已在 repair commit 修复为新路径）。

## 2. 旧路径别名（alias_obligations，4 处，与 manifest 一一对应）

每个旧路径留了一个**零逻辑**转发别名包（type alias + var 转发；文件头部注明
`Deleted by Pass B task B-workbench`）：

| 旧路径（别名包目录） | 转发目标 | 别名文件（覆盖符号） | 存活原因（不可修复引用方） |
|---|---|---|---|
| `internal/application/service/workbench` | `internal/modules/workbench/service/workbench` | `alias.go`（10：`NotificationProvider`、`NotificationDeliveryWorker` 类型别名；`NewNotificationProjector`、`NewNotificationWorker`、`NewRemoteUsageServiceWithDB`、`NewPushNotificationProvider`、`NewHTTPNotificationProvider`、`NewNotificationDeliveryWorkerWithHealth`、`NewGormInteractionStore`、`NewInteractionServiceWithApproval` var 转发） | `container.go:39`（禁改）+ `internal/modules/agentruntime/agent/engine_test.go:11`（frozen batch-a3 模块，消费 `NewGormInteractionStore`、`NewInteractionServiceWithApproval` 两符号） |
| `internal/notification` | `internal/modules/workbench/notification` | `alias.go`（1：`NewExpoProvider`） | `container.go:106`（禁改） |
| `internal/voice` | `internal/modules/workbench/voice` | `alias.go`（2：`Config` 类型别名、`NewManagedProvider` var） | `container.go:113`（禁改） |
| `internal/workbench` | `internal/modules/workbench` | `alias.go`（**0 符号**，纯文档 stub） | 无 —— 所有引用方（application/repository、handler/session、service/workbench 自身）均在 manifest `owned_files.importers` 内，已全部修复 |

**覆盖面说明（沿 A5 "Forbidden files only" 先例收窄）**：别名只转发禁改文件与
frozen 模块的实际引用面。`internal/workbench` 无任何不可修复引用方，故为满足
manifest ruling 2 的 1:1 义务仅留零符号 stub；IA4 可在切换时直接删除、无需任何
import 改动。若 IA4 需要全量符号别名，可用 `go/types` 枚举搬迁包导出符号补齐，
不改变别名零逻辑性质。

## 3. 禁改共享文件中仍指向旧路径的精确行（file:line 清单 + 切换指引）

本分支 HEAD 上，禁改共享文件中引用旧导入路径的行**只剩**（re-grep 全量核对）：

| 文件:行 | 现内容 | 切换为 |
|---|---|---|
| `internal/container/container.go:39` | `workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"` | `"github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"` |
| `internal/container/container.go:106` | `pushnotification "github.com/Tencent/WeKnora/internal/notification"` | `"github.com/Tencent/WeKnora/internal/modules/workbench/notification"` |
| `internal/container/container.go:113` | `"github.com/Tencent/WeKnora/internal/voice"` | `"github.com/Tencent/WeKnora/internal/modules/workbench/voice"` |

`internal/router/router.go`、`internal/router/task.go`、`internal/router/sync_task.go`
经 grep 确认**零**旧路径引用（router.go:233-235 的 workbench artifact grant 下载挂载
经 `params.SessionHandler`，不导入 workbench 包）。`go.mod`、`go.sum`、`migrations/`
零 diff。

切换步骤（Pass B task **B-workbench** / IA4 集成，非本任务）：
1. 修改上述 3 行 import 路径；
2. 确认引用可解析：新包同名同签名，container.go 调用点（`:225`、`:226`、`:253`、
   `:1076`、`:1090`、`:1106`、`:1111`、`:1114-1115`、`:2561`）与
   `agentruntime/agent/engine_test.go:298-299` 无需改动
   （engine_test.go 两符号已由别名转发，切换 container.go 后须同步把该测试文件
   import 改为新路径，属 frozen 模块侧一行修复，建议由 IA4 统一切换）；
3. `git rm -r internal/application/service/workbench internal/notification internal/voice internal/workbench`（四个别名目录）；
4. `go build ./...` + `go vet ./internal/container/...` 验证。

## 4. 路由（manifest 9 项入口 + session 子路由；行号为分支 HEAD 实测，与 manifest 记载一致或注明漂移）

handler 侧实现仍在横向 host 包（`internal/handler/*`、`internal/handler/session/*`
的 legacy 文件，Pass B 拆分），路由**调用点**全部位于 `internal/router`（已修复 import；
`Register*` 定义在 `internal/router/routes_workbench.go`，经 `router.go:366-374` 挂载）：

| 注册入口 | 定义（routes_workbench.go） | 挂载点（router.go） | 说明 |
|---|---|---|---|
| `RegisterWorkbenchRoutes` | :33 | :366 | workbench executions 读+写（10 条中 workbench 5）+ execution targets/workspaces |
| `RegisterWorkbenchOverviewRoutes` | :73 | :370 | Task 视图 overview |
| `RegisterWorkbenchInboxRoutes` | :84 | :371 | Inbox/Attention |
| `RegisterWorkbenchStartRoutes` | :127 | :369 | Task 启动 |
| `RegisterWorkbenchArtifactRoutes` | :142 | :367 | Workbench artifacts |
| `RegisterWorkbenchCommandRoutes` | :159 | :372 | Task 命令 |
| `RegisterMobileVoiceRoutes` | :177 | :373 | 移动语音 3 端点（W30） |
| `RegisterMobileDeviceRoutes` | :191 | :374 | 移动设备注册 |

handler 侧委托（manifest integration_points 其余两项）：

- `RegisterArtifactPreviewRoutes` — `internal/handler/artifact_preview.go:284`
  （隔离预览源 `/ap/:token`，router.go 在全局 Auth 中间件**之前**挂载）；票据签发经
  `RegisterArtifactPreviewIssueRoute`（`artifact_preview.go:271`），由
  `routes_chat.go:189-191` 委托挂载（manifest 记 :187，漂移 +2 行，逻辑未变）；
- session/message artifacts 子路由 ×4 — `routes_chat.go:165,166,167`（list ×2 +
  download ×1）与 `routes_chat.go:180`（W26 artifact-version download，条件挂载；
  manifest 未逐行记载，属同一 RegisterSessionRoutes 内）；
- workbench artifact grant 下载挂载点（域归 workbench、机制归 platform）—
  `internal/router/router.go:233-235`（`/api/v1/workbench/artifacts/download`，Auth 前）。

architectureguard 实测：`routes total=633`（literal=564 apiKeyRoute=69 handle=0），
与基线一致。

## 5. Workers（asynq Redis + Lite）

manifest `workers: []` —— workbench 模块**不注册任何 asynq 任务处理器**。
`internal/router/task.go`、`sync_task.go` 经 grep 确认零引用（workbench/notification/voice
均 0 命中）。guard 实测 `redis=23 lite=23` 与基线一致。移动通知投递 worker
（`NotificationDeliveryWorker`，container.go:1114 装配）为**进程内** worker，不入
asynq/Lite 队列。

## 6. Lifecycle hooks（container.Invoke）

| 挂点 | Invoke 行 | 实现函数 | 说明 |
|---|---|---|---|
| `registerArtifactVersionHTTPHandlers` | `container.go:671` | `container.go:2710`（manifest 记 2709，漂移 +1） | W26 不可变 artifact 版本下载源注册（fail-closed：未装配则路由不存在） |
| `registerArtifactPreviewHTTPHandlers` | `container.go:676` | `container.go:2726`（manifest 记 2725，漂移 +1） | W27 隔离预览 handler 注册 |

另有**依赖装配**（非生命周期挂点，IA4 切换 container.go import 时一并生效）：
`container.go:225-226`（Provide `NotificationProjector`/`NotificationWorker`）、
`:253`（Provide `RemoteUsageService`，`NewRemoteUsageServiceWithDB`）、
`:1076-1115`（`newMobileNotificationProvider`/`newMobileNotificationDeliveryWorker`，
消费 `pushnotification.NewExpoProvider` 与 workbenchservice 8 符号）、
`:2561`（`voice.NewManagedProvider`，移动语音 provider 装配）。

## 7. 外部依赖与配置键

- **platform 依赖**：`internal/types`、`internal/logger`、`internal/config`、
  `internal/database`（migrations fixture）等，无删除。
- **跨模块依赖（IA4 关注点）**：搬迁后的 service/workbench 6 个文件导入
  agentruntime/commercial/execution 模块内部包 —— guard 报 **14 条 forbidden-import**，
  全部为**先于模块化改造的既有耦合**（同一 import 行在 base `8fbc030a8` 的
  `internal/application/service/workbench/*.go` 逐行相同，纯 rename 证实），被
  "platform 路径 → workbench 模块路径"的搬迁暴露。按任务边界未加例外、未改工具；
  完整清单见 `docs/architecture/evidence/workbench.md` §7，原始输出在 `/tmp/a12-guard.log`。
- **配置键**（container 装配 + config）：
  - 语音 provider（container.go:2561-2569 env）：`WEKNORA_VOICE_PROVIDER_API_KEY`、
    `WEKNORA_VOICE_PROVIDER_MODEL`、`WEKNORA_VOICE_PROVIDER_TOKEN_URL`、
    `WEKNORA_VOICE_PROVIDER_TRANSCRIBE_URL`、`WEKNORA_VOICE_PROXY_SIGNING_KEY`；
  - 语音准入开关：`WEKNORA_WORKBENCH_VOICE_ADMISSION`（`internal/config/config.go:1436`，
    `Config.Workbench.VoiceAdmission`）；语音计价版本 `WEKNORA_VOICE_PRICE_VERSION`；
  - 移动通知（container.go:1078-1086 env + `cfg.MobileNotification`）：
    `MOBILE_NOTIFICATION_PROVIDER_URL`、`MOBILE_NOTIFICATION_PROVIDER`（expo/gateway/http，
    未知模式 fail-closed 到 gateway）、`MOBILE_NOTIFICATION_ACCESS_TOKEN`
    （`Config.MobileNotification.{ProviderURL,Provider,AccessToken}`）。

## 8. 横向包内遗留文件（Pass B 拆分，本任务未动）

manifest `legacy_files` 共 19 个非测试文件：`internal/application/repository/`
7 个（artifact_version、mobile_device、mobile_notification、mobile_notification_provider、
voice_session、workbench_list、workbench_request）、`internal/handler/` 3 个
（artifact_preview、mobile_device、mobile_voice）、`internal/handler/session/` 9 个
（artifact_download、artifact_reference、workbench_artifacts、workbench_commands、
workbench_inbox、workbench_list、workbench_overview、workbench_read、workbench_start）——
Pass B task **B-workbench** 按文件拆出。见 `docs/architecture/passb/workbench.md`。
