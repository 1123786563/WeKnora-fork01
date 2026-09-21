# Pass B Brief — workbench（任务 B-workbench）

来源：`docs/architecture/moves/workbench.yaml`（scope 事实源）+ Pass A task A12 交付。
集成面事实快照见 `docs/architecture/integration/workbench.md`。

## Scope（scope）

模块门面已就位：`internal/modules/workbench/module.go`（零逻辑骨架，契约
`NewModule/RegisterRoutes/RegisterWorkers/Start/Stop`）+ 4 个业务包
（`service/workbench` 22 文件、`notification` 4、`voice` 3、模块根 contracts 7，
Pass A A12 整包搬迁）。B-workbench 的义务是**横向包拆分 + 别名清退**：

1. **横向 host 包内归属 workbench 的 legacy 文件**（manifest `legacy_files`，19 个
   非测试文件；同目录 `_test.go` 随主题文件一并处理）：
   - `internal/application/repository/`：`artifact_version.go`、`mobile_device.go`、
     `mobile_notification.go`、`mobile_notification_provider.go`、`voice_session.go`、
     `workbench_list.go`、`workbench_request.go`（7 个，host 包 plurality-owner 为
     agentruntime，拆分需与其协调）；
   - `internal/handler/`：`artifact_preview.go`、`mobile_device.go`、`mobile_voice.go`（3 个）；
   - `internal/handler/session/`：`artifact_download.go`、`artifact_reference.go`、
     `workbench_artifacts.go`、`workbench_commands.go`、`workbench_inbox.go`、
     `workbench_list.go`、`workbench_overview.go`、`workbench_read.go`、
     `workbench_start.go`（9 个；`internal/handler/session` 为多模块共享 host 包，
     plurality-owner 为 conversation，拆分顺序需协调）。
2. **删除旧路径别名**（IA4 切换后清退，见 integration brief §3）：
   `internal/application/service/workbench/alias.go`、`internal/notification/alias.go`、
   `internal/voice/alias.go`、`internal/workbench/alias.go`（零符号 stub）。

## Goal（goal）

把 Task/Timeline/Inbox/Artifact/通知/移动投影的 service/repository/handler 面拆进
`internal/modules/workbench/`，使模块门面（module.go 契约）可以接管
`router.go:366-374` 的 8 个注册入口与 2 个 container.Invoke 挂点
（`registerArtifactVersionHTTPHandlers`、`registerArtifactPreviewHTTPHandlers`），
workbench 成为不依赖横向包的自洽纵向切片。

## Obligations（obligations）

1. **Task/Timeline/Artifact/Inbox/Notification 语义是外部契约**：admission 状态机、
   revision/lease 语义、artifact HMAC grant（tenant/session/message/index/expiry，
   constant-time 校验）、W26 不可变版本下载 fail-closed 挂载、W27 隔离预览票据
   （主 origin 签发 / 隔离 origin 兑换）、通知投递 revalidate-before-send 均不得改变；
   回归必测 `go test ./internal/modules/workbench/... -count=1` + handler/session、
   repository 直接消费方套件。
2. **frozen Task/Timeline/Artifact contracts 不改签名**：`internal/workbench`（现
   `internal/modules/workbench` 模块根）的 contracts.go 是跨模块消费的冻结契约
   （application/repository、handler/session 消费），拆分只增不破。
3. **注册点为禁改文件**：路由调用点 `internal/router/router.go:366-374`、grant 挂载
   `router.go:233-235`、hooks Invoke `internal/container/container.go:671/676`——
   拆分只动 handler/service/repository 归属；门面接管注册时需在 Pass B 计划单列
   router/container 装配评审。
4. **asynq 面保持空集**：manifest `workers: []`；`NotificationDeliveryWorker` 是
   进程内 worker，不得顺手改造成 asynq 任务。
5. **删除别名前置**：IA4 完成container.go:39/106/113 三行切换 +
   `agentruntime/agent/engine_test.go:11` import 切换后，`git rm` 四个别名目录；
   `internal/workbench/alias.go` 无消费方，可独立先行删除。
6. **横向包纪律**：拆出后 host 包不留转发声明；与其他模块的 legacy 拆分共享 host
   包时按 manifest 归属逐文件搬移，不得整包搬 `internal/handler`、
   `internal/handler/session`、`internal/application/repository`。
7. **跨模块耦合收敛**：service/workbench 6 文件对 agentruntime/commercial/execution
   模块内部包的 14 条既有耦合（A12 guard 发现，见 evidence §7）在 Pass B 收敛为
   模块根公共门面依赖或窄端口；收敛前若 guard 仍报，按集成期例外机制登记，不得
   在业务代码里绕过。
