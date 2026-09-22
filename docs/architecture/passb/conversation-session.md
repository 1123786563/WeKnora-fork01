# Pass B Brief — conversation-session（任务 B-conversation 之会话面）

来源：`docs/architecture/moves/conversation.yaml`（scope 事实源）+ Pass A task A10 交付。
本文是 Pass B 义务拆分之一；Query History/审计面见
`docs/architecture/passb/conversation-queryhistory.md`。

## Scope（scope）

横向 host 包内归属 conversation 会话面的 legacy 文件（manifest `legacy_files`，
共 40 个非测试文件；同目录 `_test.go` 随主题文件一并处理）：

- `internal/application/repository/`（5）：`feedback.go`、`message.go`、
  `message_suggestion.go`、`session.go`、`temporary_document.go`
- `internal/application/service/`（18）：`artifact_collector.go`、`artifact_collector_store.go`、
  `feedback.go`、`fork_bootstrapper.go`、`message.go`、`message_artifact_versions.go`、
  `message_suggestion.go`、`pinned_session_sandbox.go`、`session.go`、`session_agent_qa.go`、
  `session_attachment_staging.go`、`session_fork.go`、`session_knowledge_qa.go`、
  `session_qa_helpers.go`、`session_sandbox_pin.go`、`session_share.go`、
  `temporary_document.go`、`workspace_checkpointer.go`
  （注：`query_history_export.go`、`query_history_policy.go` 归 queryhistory 简报）
- `internal/handler/`（3）：`feedback.go`、`message.go`、`message_suggestion.go`
- `internal/handler/session/`（14）：`attachment_processor.go`、`fork.go`、`handler.go`、
  `helpers.go`、`image_upload.go`、`qa.go`、`quick_answer_timeline.go`、`resource_urls.go`、
  `share.go`、`steer.go`、`stream.go`、`temporary_document.go`、`title.go`、`types.go`
  （注：`query_history_admin.go` 归 queryhistory 简报）

已就位：`internal/modules/conversation/chat_pipeline`（Pass A A10 整包搬迁，
含 EventManager 与 17 插件）。

## Goal（goal）

把上述文件从横向 host 包拆出，收进 `internal/modules/conversation/`（建议子包：
`repository`、`service`、`handler` 或按领域再分），使 conversation 模块自持
Session/Message/Feedback/分享/建议/临时文档的完整纵向切片；横向 host 包只留
platform 本体（`list_pagination.go`、`upload_limit.go` 等）与其他模块尚未拆完的 legacy 文件。

## Obligations（obligations）

1. **删除别名**：删除 `internal/application/service/chat_pipeline/alias.go`
   （前置：IA3 已把 `internal/container/container.go:52` 切到新路径；删除前 grep 确认零 importer）。
2. **行为零变化的外部契约**（回归必测）：
   - Session/message SSE 流式端点与事件序列（`internal/handler/session/stream.go`）；
   - Share token：仅 `POST /share` 一次性揭示、`Session.ShareToken` json:"-"、
     owner-or-Admin+ 鉴权（`session_share.go` / `share.go`）；
   - 附件 staging TTL/等待超时 env 键
     （`WEKNORA_CHAT_ATTACHMENT_TTL_HOURS` / `WEKNORA_CHAT_ATTACHMENT_WAIT_TIMEOUT_SEC`）语义；
   - Feedback 提交与 RBAC 分支（`config.Tenant.IsRBACEnforced`）。
3. **收敛模块门面**：按 `internal/modules/conversation/module.go` 声明的装配契约填充
   `NewModule/RegisterRoutes/RegisterWorkers/Start/Stop`；路由仍挂
   routes_chat.go 的 5 个会话面入口（manifest integration_points.routes），
   临时文档 worker 与清理 hook 挂点不变（见 integration/conversation.md §5/§6）。
4. **横向包纪律**：拆出的文件在 host 包的残留声明（类型/函数）随之删除，不得留转发；
   `internal/handler/session` 为 6 模块共享 host 包（F0 plurality owner=conversation），
   拆分需与其他模块的 Pass B 文件拆分协调顺序，最后清空。
5. **禁改面**：`internal/container/container.go`、router 三件、go.mod/go.sum、migrations/
   的任何结构改动都要在 Pass B 计划中单列评审（A10 之后 container.go 对本模块仅剩 ：52 一行 import）。
