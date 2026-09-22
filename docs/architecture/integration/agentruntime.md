# Integration Brief — agentruntime 模块（Pass A batch A3, Task A11）

> 读者：Integration Agent（IA3）与 Pass B worker。本文是 Task A11 搬迁后 agentruntime
> 模块对外集成面的事实快照。范围事实源：`docs/architecture/moves/agentruntime.yaml`；
> 行为与结构在 Pass A 中未改变，仅包路径变化。冻结契约遵守
> `docs/architecture/frozen-entrypoints-batch-a2.md`（本模块是其最大消费者）。

## 1. 本次搬迁（A11 已完成，SHAs 见 evidence 文档）

| from（旧导入路径） | to（新导入路径） |
|---|---|
| `internal/agent`（根包） | `internal/modules/agentruntime/agent` |
| `internal/agent/approval` | `internal/modules/agentruntime/agent/approval` |
| `internal/agent/compaction` | `internal/modules/agentruntime/agent/compaction` |
| `internal/agent/experts` | `internal/modules/agentruntime/agent/experts` |
| `internal/agent/native` | `internal/modules/agentruntime/agent/native` |
| `internal/agent/nativecontract` | `internal/modules/agentruntime/agent/nativecontract` |
| `internal/agent/nativeprobe` | `internal/modules/agentruntime/agent/nativeprobe` |
| `internal/agent/opencode` | `internal/modules/agentruntime/agent/opencode` |
| `internal/agent/persona` | `internal/modules/agentruntime/agent/persona` |
| `internal/agent/recoverytest` | `internal/modules/agentruntime/agent/recoverytest` |
| `internal/agent/recoverytest/provider` | `internal/modules/agentruntime/agent/recoverytest/provider` |
| `internal/agent/runtime` | `internal/modules/agentruntime/agent/runtime` |
| `internal/agent/skills` | `internal/modules/agentruntime/agent/skills` |
| `internal/agent/skills/skillhub` | `internal/modules/agentruntime/agent/skills/skillhub` |
| `internal/agent/subagents` | `internal/modules/agentruntime/agent/subagents` |
| `internal/agent/token` | `internal/modules/agentruntime/agent/token` |
| `internal/agent/tools` | `internal/modules/agentruntime/agent/tools` |
| `internal/agent/trpc` | `internal/modules/agentruntime/agent/trpc` |
| `internal/application/service/memory` | `internal/modules/agentruntime/memory` |
| `internal/modelcontext` | `internal/modules/agentruntime/modelcontext` |

共 20 个包、371 个文件（含全部 `_test.go`；`internal/agent/testdata/`、
`opencode/testdata/protocol-lock.json` 随树搬迁）。move commit 为纯 rename
（`git diff --summary` 371/371 rename，无内容编辑）。包名全部不变。
`recoverytest/provider` 保持嵌套 package-main 结构（F1 修复后的
`.../agentruntime/agent/recoverytest/provider`）。

## 2. 旧路径别名（alias_obligations，4 处零逻辑别名）

别名**只服务禁改共享文件**（A5 收窄先例）。`internal/router/router.go`、`task.go`、
`sync_task.go`、`go.mod`、`go.sum`、`migrations/` 经 re-grep 确认**零引用**本模块旧路径；
唯一禁改引用方是 `internal/container/container.go`。因此只在 4 个旧路径留了别名：

| 旧路径（别名包目录） | 转发目标 | 别名符号（覆盖 container.go 全部引用面） |
|---|---|---|
| `internal/agent/approval` | `…/agentruntime/agent/approval` | 4：`Gate`、`MCPApproval`、`Adapter`（type alias）、`NewGate`（var） |
| `internal/agent/experts` | `…/agentruntime/agent/experts` | 3：`LoadBuiltinExperts`、`MarketDataRoot`、`PublishedDataRoot`（var） |
| `internal/agent/subagents` | `…/agentruntime/agent/subagents` | 1：`LoadBuiltinSubagents`（var） |
| `internal/application/service/memory` | `…/agentruntime/memory` | 1：`NewMemoryService`（var） |

全部为 type alias + var 转发，零逻辑；文件头注明 `Deleted by Pass B task B-agentruntime`。
container.go 对这 4 个包的 9 处引用均为只读（无对可变导出 var 的赋值），
因此**不需要** A6 LocalImageResolver 式的 import 翻转豁免。
`internal/agent` 其余 16 个旧路径无禁改引用方、不留别名（目录已随 move 消失）。

## 3. 禁改共享文件中仍指向旧路径的精确行（file:line 清单 + 切换指引）

本分支 HEAD 上，禁改共享文件中引用旧导入路径的行**只剩**：

| 文件:行 | 现内容 | 切换为 |
|---|---|---|
| `internal/container/container.go:36` | `"github.com/Tencent/WeKnora/internal/agent/approval"` | `"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/approval"` |
| `internal/container/container.go:37` | `"github.com/Tencent/WeKnora/internal/agent/experts"` | `"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/experts"` |
| `internal/container/container.go:38` | `"github.com/Tencent/WeKnora/internal/agent/subagents"` | `"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/subagents"` |
| `internal/container/container.go:54` | `"github.com/Tencent/WeKnora/internal/application/service/memory"` | `"github.com/Tencent/WeKnora/internal/modules/agentruntime/memory"` |

切换步骤（Pass B task **B-agentruntime**，非 Pass A）：
1. 修改上述 4 行 import 路径；
2. 确认 `approval.{Gate,MCPApproval,Adapter,NewGate}`、`experts.{LoadBuiltinExperts,MarketDataRoot,PublishedDataRoot}`、
   `subagents.LoadBuiltinSubagents`、`memory.NewMemoryService` 引用可解析（新包同名同签名，无需改调用点）；
3. `git rm -r internal/agent internal/application/service/memory`（两个别名目录树，4 个 alias.go）；
4. `go build ./...` + `go vet ./internal/container/...` 验证。

## 4. 路由（manifest 3 项；调用点全部位于 internal/router，已修复 import 或本来无引用）

| 注册入口 | 本分支调用点 | 说明 |
|---|---|---|
| `RegisterMemoryRoutes` | `internal/router/router.go:411`（定义 `routes_memory.go:17`） | 个人长期记忆端点（`*handler.MemoryHandler`，legacy 文件 `internal/handler/memory.go`） |
| `RegisterNativeArchiveRoutes` | `internal/router/router.go:365`（定义 `routes_native_archive.go:11`） | 不可变历史命名空间（`*session.NativeArchiveHandler`，legacy 文件） |
| agent-run session 子路由 ×4 | `internal/router/routes_chat.go:125-128`（`RegisterSessionRoutes` 内 `/sessions/:id/runs/*`） | `GetAgentRun`、`GetAgentRunEvents`、`PostAgentRunDecision`、`CancelAgentRun` |

handler 侧实现仍在横向 host 包（`internal/handler`、`internal/handler/session` 的
legacy 文件，Pass B 拆分）；router 文件本身不 import 本模块任何包。

## 5. Workers（asynq Redis + Lite，manifest 1 项）

| 任务类型 | Redis 注册 | Lite 注册 | 处理器 |
|---|---|---|---|
| `TypeMemoryExtract` | `internal/router/task.go:323` | `internal/router/sync_task.go:164` | `MemoryService.Handle`（`internal/modules/agentruntime/memory`） |

两处均经 `params.MemoryService`（container 装配的类型）间接引用，router/task 文件
零 import 本模块。architectureguard 实测：`redis=23 lite=23`，与 F0 基线一致。

## 6. Lifecycle hooks（container.Invoke，manifest 2 项）

| 挂点 | 位置 | 说明 |
|---|---|---|
| `registerAgentRunResourceProtection` | `internal/container/container.go:257`（`Invoke`）→ 定义 `container.go:2462` | Run 资源保护（`*service.GormAgentRunResourceRepository`） |
| AgentRunStore cleaner（inline） | `internal/container/container.go:693` | `store.SetCleanupFilePurger(...)` + `StartExecutionCleanupSweep` |

另有两处**依赖装配**（非生命周期挂点）：`internal/container/agent_runtime.go`
（`NewAgentRuntime` 引擎装配，已修复 import 指向 `…/agentruntime/agent/runtime`）；
`container.go:471-475,562`（`approval.NewGate` / `memory.NewMemoryService` Provide，经别名）。
architectureguard 实测 `hooks=58`，与基线一致。

## 7. 引擎入口与协议面（供 IA3 差异化验证与 Pass B 导航）

- **引擎入口**：`internal/modules/agentruntime/agent`（`Engine`/`Execute`，ReAct 循环：
  think → act → finalize）；durable worker 装配在 `internal/container/agent_runtime.go`
  （`agentruntime.Fence`、`RunKey`、Cancel/Recovery hook）。
- **Run/Attempt 状态机**：`…/agent/runtime`（fence/lease/run key 契约）；持久化在
  `internal/application/repository/agent_run*.go`、`native_*.go`（legacy，Pass B 拆分）。
- **审批**：`…/agent/approval`（`Gate`/`MCPApproval`；容器 Provide 于 container.go:471-475）。
- **tRPC/OpenCode/native 协议**：`…/agent/trpc`（durable run 图，消费方
  `internal/application/service/agent_run_graph.go:22`、`agent_capabilities.go:15`，均为
  legacy 文件）；`…/agent/opencode`（本地编码代理协议）；`…/agent/native` +
  `…/agent/nativecontract` + `…/agent/nativeprobe`（native 会话/契约/探针，仅模块内消费）。
- **Memory / Model Context**：`…/agentruntime/memory`（抽取/整合/向量化 + `TypeMemoryExtract` worker）；
  `…/agentruntime/modelcontext`（MCP 资源/工具策略注册表）。
- **Recovery 验收**：`…/agent/recoverytest`（SIGKILL 恢复矩阵；CI 镜像
  `.github/workflows/agent-recovery.yml`）；`recoverytest/provider` 为真实图提供者的
  package-main（嵌套结构保留）。PG 依赖用例在无 `TRPC_RECOVERY_GRAPH_PROVIDER` 时
  skip（blocked-env，非 PASS）。

## 8. 外部依赖与配置键

- **冻结模块消费（只经 frozen-entrypoints-batch-a2.md §1 路径）**：
  `execution/sandbox`（工具 shell/文件 ×10 文件 + skills ×4）、`execution/browserskill`（×2）、
  `airesource/models/chat`（引擎/压缩/记忆/modelcontext 等 ×26）、`airesource/mcp`（×2）、
  `airesource/models/rerank`（×1）、`airesource/models/utils/ollama`（recoverytest/provider ×1）；
  另有 legacy 期的 `commercial`（`commercial_adapter.go`）与 `appconnector`
  （`tools/app_connector.go`）各 ×1。
- **配置键**（`internal/config`）：`agent.llm_call_timeout`、`agent.tool_approval_timeout_seconds`、
  `agent.recovery.{enabled,admission_enabled,lease,heartbeat,scan_interval,max_workers}`
  （`config.AgentConfig` / `config.AgentRecoveryConfig`）；运行期经 `config.Config`
  会话字段（`MaxIterations`、`MaxContextTokens`、`Temperature`、`WebSearchEnabled`、
  `MemoryEnabled`、`ExtractModelID` 等）。
- **环境变量**（模块内直读）：`WEKNORA_AGENT_TOOL_APPROVAL_FAIL_OPEN`、
  `WEKNORA_REDIS_NAMESPACE`、`LOCAL_STORAGE_BASE_DIR`；
  recoverytest 专用：`TRPC_RECOVERY_USE_PG`、`TRPC_RECOVERY_PG_DSN`、
  `TRPC_RECOVERY_GRAPH_PROVIDER`、`TRPC_RECOVERY_REPO_ROOT`、`TRPC_RECOVERY_TEST_NAMESPACE`、
  `TRPC_RECOVERY_COUNTER_URL`、`TRPC_RECOVERY_DEBUG`。

## 9. 横向包内遗留文件（Pass B 拆分，本任务未动；45 个）

`internal/application/repository/`（23：agent_checkpoint、agent_run ×7、
mcp_tool_approval_repository、memory ×4、native ×10）；
`internal/application/service/`（18：agent_run ×7、agent_capabilities、agent_history、
agent_web_pages、mcp_tool_approval_service、native ×6、subagent_delegate）；
`internal/handler/memory.go`；`internal/handler/session/`（3：agent_run、
agent_stream_handler、native_archive）—— 见 manifest `legacy_files` 与
`internal/modules/agentruntime/legacy/README.md`，Pass B task B-agentruntime 按文件拆出。

## 10. IA3 关注：预存横向耦合显形（49 条 forbidden-import，未加豁免）

本搬迁把 `internal/agent/**` 置于 `internal/modules/` 之下后，architectureguard 的
forbidden-import 扫描（只扫 `internal/modules/<owner>/`）首次覆盖到这些文件，
暴露出 **搬迁前即存在** 的模块间 import（守卫此前扫不到非模块目录）：

| 方向 | 边数 | 代表文件 |
|---|---|---|
| agentruntime → airesource/models/chat | 26 | `agent/engine.go`、`compaction/*`、`memory/*`、`modelcontext/*` |
| agentruntime → execution/sandbox | 10 | `agent/tools/{shell_exec,skill_file,sandbox_*...}`、`agent/skills/*` |
| agentruntime → airesource/mcp | 2 | `agent/tools/{mcp_tool,mcp_oauth}.go` |
| agentruntime → execution/browserskill | 2 | `agent/tools/browserskill*.go` |
| agentruntime → airesource/models/rerank | 1 | `agent/tools/knowledge_search.go` |
| agentruntime → airesource/models/utils/ollama | 1 | `agent/recoverytest/provider/main.go` |
| agentruntime → commercial | 1 | `agent/commercial_adapter.go` |
| agentruntime → appconnector | 1 | `agent/tools/app_connector.go` |
| channels/im → agentruntime/agent/tools | 2 | `internal/modules/channels/im/{service,cmd_search}.go` |

按任务规程**未登记任何豁免、未改守卫工具**；全部移交 IA3 Integrator 裁量
（登记精确豁免或在 Pass B 收敛）。route/worker/hook 计数不变（633/23+23/58）。
