# conversation — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/conversation.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/feedback.go` | Feedback (application/repository) | `B-conversation` |
| `internal/application/repository/message.go` | Message (application/repository) | `B-conversation` |
| `internal/application/repository/message_suggestion.go` | Message suggestion (application/repository) | `B-conversation` |
| `internal/application/repository/query_history_export.go` | Query history export (application/repository) | `B-conversation` |
| `internal/application/repository/session.go` | Session (application/repository) | `B-conversation` |
| `internal/application/repository/temporary_document.go` | Temporary document (application/repository) | `B-conversation` |
| `internal/application/service/artifact_collector.go` | Artifact collector (application/service) | `B-conversation` |
| `internal/application/service/artifact_collector_store.go` | Artifact collector store (application/service) | `B-conversation` |
| `internal/application/service/feedback.go` | Feedback (application/service) | `B-conversation` |
| `internal/application/service/fork_bootstrapper.go` | Fork bootstrapper (application/service) | `B-conversation` |
| `internal/application/service/message.go` | Message (application/service) | `B-conversation` |
| `internal/application/service/message_artifact_versions.go` | Message artifact versions (application/service) | `B-conversation` |
| `internal/application/service/message_suggestion.go` | Message suggestion (application/service) | `B-conversation` |
| `internal/application/service/pinned_session_sandbox.go` | Pinned session sandbox (application/service) | `B-conversation` |
| `internal/application/service/query_history_export.go` | Query history export (application/service) | `B-conversation` |
| `internal/application/service/query_history_policy.go` | Query history policy (application/service) | `B-conversation` |
| `internal/application/service/session.go` | Session (application/service) | `B-conversation` |
| `internal/application/service/session_agent_qa.go` | Session agent qa (application/service) | `B-conversation` |
| `internal/application/service/session_attachment_staging.go` | Session attachment staging (application/service) | `B-conversation` |
| `internal/application/service/session_fork.go` | Session fork (application/service) | `B-conversation` |
| `internal/application/service/session_knowledge_qa.go` | Session knowledge qa (application/service) | `B-conversation` |
| `internal/application/service/session_qa_helpers.go` | Session qa helpers (application/service) | `B-conversation` |
| `internal/application/service/session_sandbox_pin.go` | Session sandbox pin (application/service) | `B-conversation` |
| `internal/application/service/session_share.go` | Session share (application/service) | `B-conversation` |
| `internal/application/service/temporary_document.go` | Temporary document (application/service) | `B-conversation` |
| `internal/application/service/workspace_checkpointer.go` | Workspace checkpointer (application/service) | `B-conversation` |
| `internal/handler/feedback.go` | Feedback (handler) | `B-conversation` |
| `internal/handler/message.go` | Message (handler) | `B-conversation` |
| `internal/handler/message_suggestion.go` | Message suggestion (handler) | `B-conversation` |
| `internal/handler/session/attachment_processor.go` | Attachment processor (handler/session) | `B-conversation` |
| `internal/handler/session/fork.go` | Fork (handler/session) | `B-conversation` |
| `internal/handler/session/handler.go` | Handler (handler/session) | `B-conversation` |
| `internal/handler/session/helpers.go` | Helpers (handler/session) | `B-conversation` |
| `internal/handler/session/image_upload.go` | Image upload (handler/session) | `B-conversation` |
| `internal/handler/session/qa.go` | Qa (handler/session) | `B-conversation` |
| `internal/handler/session/query_history_admin.go` | Query history admin (handler/session) | `B-conversation` |
| `internal/handler/session/quick_answer_timeline.go` | Quick answer timeline (handler/session) | `B-conversation` |
| `internal/handler/session/resource_urls.go` | Resource urls (handler/session) | `B-conversation` |
| `internal/handler/session/share.go` | Share (handler/session) | `B-conversation` |
| `internal/handler/session/steer.go` | Steer (handler/session) | `B-conversation` |
| `internal/handler/session/stream.go` | Stream (handler/session) | `B-conversation` |
| `internal/handler/session/temporary_document.go` | Temporary document (handler/session) | `B-conversation` |
| `internal/handler/session/title.go` | Title (handler/session) | `B-conversation` |
| `internal/handler/session/types.go` | Types (handler/session) | `B-conversation` |
