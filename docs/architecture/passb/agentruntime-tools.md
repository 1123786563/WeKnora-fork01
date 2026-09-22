# Pass B Brief — B-agentruntime：工具边界（tools）

> Pass B 任务：`B-agentruntime`。本 brief 覆盖 agent 工具子系统的边界收敛。
> 范围事实源：`docs/architecture/moves/agentruntime.yaml` 与
> `docs/architecture/integration/agentruntime.md`。

## Goal（目标）

`internal/modules/agentruntime/agent/tools` 成为工具子系统唯一入口：对 execution/sandbox、
execution/browserskill、airesource/mcp、airesource/models/rerank、appconnector 的依赖
从"深 import 内部包"收敛为对模块根公共门面（或注入端口）的依赖，删除 integration §10
中 tools 侧全部预存 forbidden-import 边。

## Scope（范围）

**模块内已有包**：`internal/modules/agentruntime/agent/tools`（约 60 文件，含
shell/沙箱文件操作、MCP 工具暴露与 OAuth、浏览器技能、知识检索、工作区读取、
技能文件、schema 等工具实现与 journal）。

**无遗留文件迁入**：tools 子系统的横向 host 文件已在 Pass A 前不存在
（manifest legacy_files 无 tools 项）。

## Obligations（义务）

1. **耦合收敛（integration/agentruntime.md §10 tools 侧边）**：
   - → `modules/execution/sandbox` ×10：`tools/{output_links,sandbox_edit,sandbox_ls,
     sandbox_write,shell_exec,skill_file,workspace_reader}.go`；
   - → `modules/execution/browserskill` ×2：`tools/{browserskill,browserskill_result}.go`；
   - → `modules/airesource/mcp` ×2：`tools/{mcp_oauth,mcp_tool}.go`；
   - → `modules/airesource/models/rerank` ×1：`tools/knowledge_search.go`；
   - → `modules/appconnector` ×1：`tools/app_connector.go`。
   收敛手段限：改经对方模块根门面、抽端口由 container 注入、或下沉为模块内接口；
   不得复制对方实现。
2. **工具外部契约不变**：工具名、JSON schema、审批标记（`approval` flag）、
   journal 事件为外部契约；`sanitize_messages.go`、`mcp_exposure.go` 的
   `airesource/models/chat` 消费面一并收敛。
3. **验证**：`go test ./internal/modules/agentruntime/agent/tools/... -count=1`、
   `go build ./...`、`make check-backend-architecture` 中 tools 侧 forbidden-import
   归零；工具注册面（engine 工具数）不变。
