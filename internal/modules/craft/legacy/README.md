# craft — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/craft.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/craft_preview_check.go` | Craft preview check (application/repository) | `B-craft` |
| `internal/application/repository/craft_scheduled.go` | Craft scheduled (application/repository) | `B-craft` |
| `internal/application/repository/craft_snapshot.go` | Craft snapshot (application/repository) | `B-craft` |
| `internal/application/repository/craft_usage.go` | Craft usage (application/repository) | `B-craft` |
| `internal/application/repository/craft_version.go` | Craft version (application/repository) | `B-craft` |
| `internal/application/repository/craft_workspace.go` | Craft workspace (application/repository) | `B-craft` |
| `internal/application/service/craft_artifacts.go` | Craft artifacts (application/service) | `B-craft` |
| `internal/application/service/craft_budget.go` | Craft budget (application/service) | `B-craft` |
| `internal/application/service/craft_control.go` | Craft control (application/service) | `B-craft` |
| `internal/application/service/craft_decision_delivery.go` | Craft decision delivery (application/service) | `B-craft` |
| `internal/application/service/craft_delegate.go` | Craft delegate (application/service) | `B-craft` |
| `internal/application/service/craft_inputs.go` | Craft inputs (application/service) | `B-craft` |
| `internal/application/service/craft_interaction_store.go` | Craft interaction store (application/service) | `B-craft` |
| `internal/application/service/craft_knowledge.go` | Craft knowledge (application/service) | `B-craft` |
| `internal/application/service/craft_knowledge_tool.go` | Craft knowledge tool (application/service) | `B-craft` |
| `internal/application/service/craft_lifecycle.go` | Craft lifecycle (application/service) | `B-craft` |
| `internal/application/service/craft_preview.go` | Craft preview (application/service) | `B-craft` |
| `internal/application/service/craft_recovery.go` | Craft recovery (application/service) | `B-craft` |
| `internal/application/service/craft_scheduled.go` | Craft scheduled (application/service) | `B-craft` |
| `internal/application/service/craft_session.go` | Craft session (application/service) | `B-craft` |
| `internal/application/service/craft_snapshot.go` | Craft snapshot (application/service) | `B-craft` |
| `internal/application/service/craft_usage.go` | Craft usage (application/service) | `B-craft` |
| `internal/application/service/craft_usage_view.go` | Craft usage view (application/service) | `B-craft` |
| `internal/application/service/craft_workspace.go` | Craft workspace (application/service) | `B-craft` |
| `internal/handler/craft_model_gateway.go` | Craft model gateway (handler) | `B-craft` |
| `internal/handler/session/craft.go` | Craft (handler/session) | `B-craft` |
| `internal/handler/session/craft_interaction.go` | Craft interaction (handler/session) | `B-craft` |
| `internal/handler/session/craft_preview.go` | Craft preview (handler/session) | `B-craft` |
| `internal/handler/session/craft_scheduled.go` | Craft scheduled (handler/session) | `B-craft` |
| `internal/handler/session/craft_usage.go` | Craft usage (handler/session) | `B-craft` |
