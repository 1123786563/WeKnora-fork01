# Pass B Brief — B-agentruntime：记忆与模型上下文边界（memory/modelcontext）

> Pass B 任务：`B-agentruntime`。本 brief 覆盖长期记忆与 Model Context 子系统的边界
> 收敛。范围事实源：`docs/architecture/moves/agentruntime.yaml` 与
> `docs/architecture/integration/agentruntime.md`。

## Goal（目标）

`internal/modules/agentruntime/memory` 与 `…/modelcontext` 成为记忆/模型上下文唯一归属：
横向包中的 memory 仓储与 handler 拆入模块，`TypeMemoryExtract` worker 与
`RegisterMemoryRoutes` 的实现面回到模块内，memory 侧别名与 airesource 耦合收敛。

## Scope（范围）

**模块内已有包**：`internal/modules/agentruntime/memory`（service.go、extract、
consolidate、topic_resolve、vector、persistence/postgres 一致性测试等 32 文件）、
`internal/modules/agentruntime/modelcontext`（17 文件）。

**迁入（legacy_files 中归 memory 侧的文件）**：
- `internal/application/repository/`：memory.go、memory_extraction.go、
  memory_lifecycle.go、memory_vector.go（4）；
- `internal/handler/memory.go`（1）。

## Obligations（义务）

1. **别名删除**：`internal/application/service/memory/alias.go` 是 4 个 Pass A 别名中
   memory 侧唯一残留；container.go:54 切换到
   `…/agentruntime/memory` 后删除（与 engine brief 的别名删除同批或独立提交均可，
   不得留半删状态）。
2. **耦合收敛（integration/agentruntime.md §10 memory/modelcontext 侧）**：
   → `modules/airesource/models/chat` ×3（`memory/{extract,consolidate,topic_resolve}.go`）
   与 ×6（`modelcontext/{mcp,registry,resources,sources,tool_policy}.go` 等）；
   收敛手段限：模块根门面、注入端口或下沉接口。
3. **worker 语义不变**：`TypeMemoryExtract` 处理器（`MemoryService.Handle`）在
   asynq Redis（router/task.go:323）与 Lite（sync_task.go:164）双侧注册保持一对一；
   抽取/整合行为与迁移脚本（memory 4 个 versioned migration 测试）不变。
4. **路由不变**：`RegisterMemoryRoutes`（router.go:411 ← routes_memory.go:17）挂载点、
   路径、RBAC guard 装配不变。
5. **验证**：`go test ./internal/modules/agentruntime/memory/...
   ./internal/modules/agentruntime/modelcontext/... -count=1`（PG 用例按既有
   blocked-env skip）、`go test ./internal/application/repository -count=1`、
   `go build ./...`、`make check-backend-architecture` 中 memory 侧 forbidden-import
   归零。
