# workbench — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/workbench.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/artifact_version.go` | Artifact version (application/repository) | `B-workbench` |
| `internal/application/repository/mobile_device.go` | Mobile device (application/repository) | `B-workbench` |
| `internal/application/repository/mobile_notification.go` | Mobile notification (application/repository) | `B-workbench` |
| `internal/application/repository/mobile_notification_provider.go` | Mobile notification provider (application/repository) | `B-workbench` |
| `internal/application/repository/voice_session.go` | Voice session (application/repository) | `B-workbench` |
| `internal/application/repository/workbench_list.go` | Workbench list (application/repository) | `B-workbench` |
| `internal/application/repository/workbench_request.go` | Workbench request (application/repository) | `B-workbench` |
| `internal/handler/artifact_preview.go` | Artifact preview (handler) | `B-workbench` |
| `internal/handler/mobile_device.go` | Mobile device (handler) | `B-workbench` |
| `internal/handler/mobile_voice.go` | Mobile voice (handler) | `B-workbench` |
| `internal/handler/session/artifact_download.go` | Artifact download (handler/session) | `B-workbench` |
| `internal/handler/session/artifact_reference.go` | Artifact reference (handler/session) | `B-workbench` |
| `internal/handler/session/workbench_artifacts.go` | Workbench artifacts (handler/session) | `B-workbench` |
| `internal/handler/session/workbench_commands.go` | Workbench commands (handler/session) | `B-workbench` |
| `internal/handler/session/workbench_inbox.go` | Workbench inbox (handler/session) | `B-workbench` |
| `internal/handler/session/workbench_list.go` | Workbench list (handler/session) | `B-workbench` |
| `internal/handler/session/workbench_overview.go` | Workbench overview (handler/session) | `B-workbench` |
| `internal/handler/session/workbench_read.go` | Workbench read (handler/session) | `B-workbench` |
| `internal/handler/session/workbench_start.go` | Workbench start (handler/session) | `B-workbench` |
