# airesource — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/airesource.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/mcp_metadata.go` | Mcp metadata (application/repository) | `B-airesource` |
| `internal/application/repository/mcp_oauth.go` | Mcp oauth (application/repository) | `B-airesource` |
| `internal/application/repository/mcp_service.go` | Mcp service (application/repository) | `B-airesource` |
| `internal/application/repository/model.go` | Model (application/repository) | `B-airesource` |
| `internal/application/repository/resource.go` | Resource (application/repository) | `B-airesource` |
| `internal/application/repository/resource_references.go` | Resource references (application/repository) | `B-airesource` |
| `internal/application/repository/storagebackend.go` | Storagebackend (application/repository) | `B-airesource` |
| `internal/application/repository/vectorstore.go` | Vectorstore (application/repository) | `B-airesource` |
| `internal/application/repository/web_search_provider.go` | Web search provider (application/repository) | `B-airesource` |
| `internal/application/service/mcp_metadata.go` | Mcp metadata (application/service) | `B-airesource` |
| `internal/application/service/mcp_service.go` | Mcp service (application/service) | `B-airesource` |
| `internal/application/service/model.go` | Model (application/service) | `B-airesource` |
| `internal/application/service/resource.go` | Resource (application/service) | `B-airesource` |
| `internal/application/service/storagebackend.go` | Storagebackend (application/service) | `B-airesource` |
| `internal/application/service/vectorstore.go` | Vectorstore (application/service) | `B-airesource` |
| `internal/application/service/vectorstore_healthcheck.go` | Vectorstore healthcheck (application/service) | `B-airesource` |
| `internal/application/service/web_search.go` | Web search (application/service) | `B-airesource` |
| `internal/application/service/web_search_provider.go` | Web search provider (application/service) | `B-airesource` |
| `internal/application/service/web_search_state.go` | Web search state (application/service) | `B-airesource` |
| `internal/application/service/weknoracloud.go` | Weknoracloud (application/service) | `B-airesource` |
| `internal/handler/mcp_credentials.go` | Mcp credentials (handler) | `B-airesource` |
| `internal/handler/mcp_metadata.go` | Mcp metadata (handler) | `B-airesource` |
| `internal/handler/mcp_oauth.go` | Mcp oauth (handler) | `B-airesource` |
| `internal/handler/mcp_service.go` | Mcp service (handler) | `B-airesource` |
| `internal/handler/mcp_usage_instructions.go` | Mcp usage instructions (handler) | `B-airesource` |
| `internal/handler/model.go` | Model (handler) | `B-airesource` |
| `internal/handler/model_credentials.go` | Model credentials (handler) | `B-airesource` |
| `internal/handler/storagebackend.go` | Storagebackend (handler) | `B-airesource` |
| `internal/handler/vectorstore.go` | Vectorstore (handler) | `B-airesource` |
| `internal/handler/web_search.go` | Web search (handler) | `B-airesource` |
| `internal/handler/web_search_provider.go` | Web search provider (handler) | `B-airesource` |
| `internal/handler/web_search_provider_credentials.go` | Web search provider credentials (handler) | `B-airesource` |
| `internal/handler/weknoracloud.go` | Weknoracloud (handler) | `B-airesource` |
