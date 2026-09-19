# tRPC 原生迁移功能基线

盘点日期：2026-09-19。此文档是 P0 Task 1 的静态覆盖审查；`passed` 只表示清单、入口和消费者已经审查，不能表示任何运行验收已经通过。所有 TSV 行的 `evidence_status` 均为 `unverified`，直到后续任务按其环境执行行为测试。

## 工作树与可复现性

| 项目 | 事实 |
| --- | --- |
| worktree | `/Users/wuyongjun/.codex/worktrees/trpc-native-agent-p0/WeKnora-fork01` |
| branch | detached HEAD (managed isolated worktree) |
| HEAD | `5c77dd3a`（本任务提交前） |
| dirty status | clean；未复制其他工作树的前端未提交修改 |
| Go / SDK | `go 1.26.0`；`trpc.group/trpc-go/trpc-agent-go v1.10.0`，见 `go.mod:3,99` |

本次执行并读取了任务规定的只读命令：`git rev-parse HEAD`、`git status --short`、对 `go.mod` 的依赖查询、对 `CreateAgentEngine|ExecuteDurableRun|EngineType` 的查询，以及 `internal/agent/{tools,skills}`、`internal/models/chat`、`internal/sandbox` 和客户端 agent/chat/stream/session/memory 文件清单查询。下列条目来自调用路径与相邻测试的阅读，不是仅由文件名推断。

## 当前执行与配置边界

* `types.Session.EngineType`（`internal/types/session.go:88`）默认 `builtin`。`sessionService.AgentQA`（`internal/application/service/session_agent_qa.go:27`）拒绝在 builtin session 上执行 custom agent；`EngineType == "trpc"` 时转入 durable admission（:161）。`TestAgentQARejectsCustomAgentOnBuiltinSession` 固定该失败语义。
* builtin 路径通过 `agentService.CreateAgentEngine`（`agent_service.go:191`）构造现有 `agent.NewAgentEngine`。该工厂装配 model、知识、工具、MCP、Skills、sandbox、memory 和 Craft；它仍是旧 ReAct/AgentEngine 入口，不能被标记为原生 Runner 已验证。
* trpc 路径由 `submitDurableAgentRun`（`agent_run_graph.go:170`）在 `RecoveryEnabled`、`AdmissionEnabled` 和未 drain 时提交。它在 HTTP 断开后使用 `context.WithoutCancel` 提交；`TestSubmitDurableAgentRunSurvivesDisconnect` 是已有 SQLite 行为测试。
* `ExecuteDurableRun`（`agent_run_graph.go:314`）按 fence 重读 run，恢复快照配置、重新解析当前模型/工具/权限，使用 `trpcagent.NewGraphRunner`；模型重新解析发生在 `agent_run_graph.go:349`。容器的 `newAgentRuntime`（`internal/container/agent_runtime.go:24`）经 `RegisteredGraphExecutor` 延迟接线，缺少 executor 时在启用恢复的配置下 fail closed。
* 环境开关保留：`Agent.Recovery.Enabled` 控制 worker，`AdmissionEnabled` 控制新 run；workbench worker drain 拒绝新准入。缺少 run service、模型、checkpoint、journal、event store 或 graph executor 均返回明确错误。事件 append 失败目前只告警（`agent_run_graph.go` 的 `emit`）；它不能作为可靠投影的验收证据。

## 已注册能力与失败状态

| 范围 | 当前注册/调用入口 | 配置和失败边界 | 基线测试或后续责任 |
| --- | --- | --- | --- |
| 模型与流 | `AgentQA` 解析 model；builtin 将 `chat.Chat` 传给 `CreateAgentEngine`；durable 将其适配到 `trpcagent.NewModel` | model、rerank 不可用直接失败；快照固定 model identity 但重新解析凭据/可用性 | `internal/agent/trpc/model_test.go`；P2/P3 |
| builtin 工具 | `prepareAgentCapabilities` / 工具 switch（`agent_service.go:840-1136`） | allowlist、共享只读、KB scope、wiki/vector 能力、web switch、memory switch 会删减工具；未知工具告警 | `agent_service_test.go`；P2 |
| MCP | `registerMCPTools`（:229）→ `tools.RegisterMCPTools` | `none`、selected-empty、缺 tenant/metadata、disabled service 均不注册；OAuth/approval 形成 durable wait | `agent_connector_test.go`、`mcp_exposure_test.go`；P2/P4 |
| Open Connector | `registerOpenConnectorTool`（:311）→ action facade | 仅 active installation 可见；每 action 仍经过授权/审批，unknown outcome 保持 unknown 或查询 | `agent_connector_test.go`、`appconnector/action_test.go`；P2/P4 |
| 知识与 web | `prepareAgentCapabilities` 组装 KB/doc；工具 switch 注册 RAG、wiki、web、data tools | 无 scope、无 wiki/vector 能力、web 未启用时过滤；shared-agent 写入工具移除 | `agent_service_test.go`；P2/P4 |
| Skills / Sandbox | `initializeSkillsManager`、`registerSandboxFileTools`、`registerSandboxShellIfAllowed` | SkillsEnabled/install mode、已验证 image dir、session-bound sandbox capability 是前提；不支持时不注册并记录日志 | `agent_service_test.go`、`tenant_skill_install_test.go`；P4 |
| Craft 专业委派 | `registerCraftDelegateTool` 在 tRPC + bound workspace 条件下注册 | 无 Craft assembly/工作区不暴露；runtime recovery 在 sandbox recovery 后接 Craft hook | `craft_delegate_test.go`；P2/P4 |
| 会话、历史和 memory | builtin `LoadAgentHistory` + `memoryService.Recall`；durable snapshot/history/model messages | multi-turn 和 memory preference/scope 都可关闭；新 SDK Session 尚未成为权威 | `agent_history_test.go`；P1/P3 |
| 审批、计费、恢复 | `ToolExecutor` journal、approval gate、`AgentRunService`/worker | 未知写操作默认 `wait_user`；OAuth、MCP approval、OC action 为不同 wait kind；预算耗尽为终态事件 | `tool_recovery_test.go`、`agent_run_budget_notification_test.go`；P1/P2/P7 |

## 恢复事实（不扩大为运行验收）

`internal/agent/runtime/tool_executor.go` 的持久化状态包含 planned、dispatching、succeeded、failed、unknown。`RecoveryAction` 的已有单元测试确认：有结果复用、可查询优先查询、已验证幂等键或只读才重试，其他未知写入进入 `wait_user`。这证明当前项目的策略实现存在；真实 Provider 查询、恰好一次外部效果和跨进程恢复仍为 P3/P7 的 `unverified` 项。

## 存储和客户端

主业务数据库的实际支持是 PostgreSQL 与 SQLite：`internal/container/container.go:996-1057` 只接受 `DB_DRIVER=postgres|sqlite`；SQLite 使用 WAL、busy timeout 和 foreign keys。`docker-compose.yml:139-145` 默认 PostgreSQL，并记录 SQLite 可用的 `DB_PATH`。迁移与恢复必须分别覆盖二者；检索引擎或 MySQL 协议的 Doris 依赖不等同于主业务数据库支持。

| 客户端 | 当前消费者依据 | 可用脚本 / 验收状态 |
| --- | --- | --- |
| Web | pnpm workspace `apps/web`；`@weknora/api-client` chat stream | `dev`、`build`、`test`；未执行 |
| Desktop renderer | `apps/desktop` 依赖平台 adapter，并复用 Web renderer | `dev`、`build`、`typecheck`、`test`；未执行 |
| Embed | `apps/embed` 依赖 api-client/contracts/views | `dev`、`build`、`test`；未执行 |
| Mini Program | pnpm workspace `apps/miniprogram` | `dev:weapp`、`build:weapp`、`typecheck`、`test`；未执行 |
| CLI / Go client | `cli/cmd/session/ask.go` 和 `client/agent.go` 调用 `/agent-chat/:id` SSE | Go 测试入口存在；未执行 |
| DSH package | `packages/dsh-weknora` 用 agent id 决定 `/agent-chat` | `build`、`typecheck`、`test`；未执行 |
| `apps/mobile` | Flutter `pubspec.yaml`，直接适配 `/agent-chat` 和 `/knowledge-chat` | Flutter 活跃客户端；未执行设备/编译 |
| `apps/mobile-next` | 独立 Expo/npm 工程，`ChatService.ts` 调 `/agent-chat/:session_id` | `start`、`android`、`ios`、`typecheck`、`test`、`check:isolation`、`export:web`；未执行 |

`apps/mobile-next` 不在 `pnpm-workspace.yaml`，但有独立 `package.json`、锁文件、原生目录和实际 API 消费者，因此不能因 pnpm 清单排除。旧 `apps/mobile` 没有 package.json，而有 `pubspec.yaml` 和 agent-chat 适配器，也不能排除。没有发现 `apps/mobile-next` 依赖旧 mobile 的生产路径；其 isolation 脚本明确禁止这种依赖。这只是源码消费者依据，非端到端证明。

## 历史归档与保留边界

已确认规格（`docs/superpowers/specs/2026-09-19-trpc-native-agent-migration-design.md:12,16,92,128`）要求旧历史只能在当前授权下只读查询；新会话、执行状态、长期记忆和 checkpoint 从零开始，旧会话不得恢复执行，也不得隐式注入新 Runner 上下文。归档不构成删除授权：历史会话、消息、工具/审批记录、附件、产物引用、审计记录和旧长期记忆均须保留，并继续由当前空间成员资格与资源权限控制读取。当前仓库尚未实现新旧归档读模型的最终路由；P6 负责归档/切换实现，P7 负责权限、附件访问、备份恢复与发布演练证据。

## 覆盖判定

每个当前注册入口至少对应 `features.tsv` 一行；每一行指定 P1/P2/P3/P4/P5/P6/P7 责任工作包。无当前消费者的 SDK Session/Memory 采用条目保留为“待迁移”，并写明搜索范围（`apps`、`packages`、`cli`、`client` 的 agent/chat/stream/session/memory 路径）。Task 1 已通过文档覆盖审查；没有 synthetic RED，也没有把静态、Mock、SQLite 或单元结果升级为真实提供商、浏览器、原生设备、PostgreSQL 或发布验收。
