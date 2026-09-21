# Integration Brief — conversation (IA3)

适用对象：Pass A 集成者（Integrator）。任务：把 `bm-passa-a10` 分支的搬迁成果合入集成线。
本文件只描述 conversation 模块（manifest：`docs/architecture/moves/conversation.yaml`），
不动 knowledge（`bm-passa-a9`）与 agentruntime（`bm-passa-a11`）领地。IA3 顺序固定 K→C→AR。

## 1. 本次搬迁（A10 已完成，SHAs 见 evidence 文档）

| from（旧导入路径） | to（新导入路径） |
|---|---|
| `internal/application/service/chat_pipeline` | `internal/modules/conversation/chat_pipeline` |

共 47 个文件（38 个非测试 `.go` + 9 个 `_test.go`），move commit 为纯 rename
（47/47 相似度 100%，0 insertions/deletions，`git diff --summary` 全 R，零函数体改动、
零 test 相对路径修补）。

**未搬迁**：`internal/handler/session`（15 个非测试文件）与 `internal/application/service`、
`internal/application/repository` 内的 Session/Message/Feedback/Query History 混合文件
（共 44 个 legacy 文件）按 manifest `legacy_files` 留在横向 host 包内，Pass B `B-conversation`
逐文件拆分——见 `docs/architecture/passb/conversation-session.md` 与
`conversation-queryhistory.md`。

## 2. 旧路径别名（alias_obligations，1 处）

| 旧路径（别名包目录） | 转发目标 | 别名文件 |
|---|---|---|
| `internal/application/service/chat_pipeline` | `internal/modules/conversation/chat_pipeline` | `alias.go`（18 符号） |

零逻辑 var 转发（无 type alias），文件头注明 `Deleted by Pass B task B-conversation`。
**别名面收窄至禁改文件实际引用**：`NewEventManager` + 17 个 `NewPlugin*` 构造器
（container.go:644-661 全部引用面）。**当前唯一 importer 是禁改文件
`internal/container/container.go`**。

为什么不是 container.go 直接改 import：container.go 只调用 `chatpipeline.New*`
构造器、**不赋值任何 chatpipeline 导出 var**（已 grep 验证），A6
`chat.LocalImageResolver` 可变 var 翻转先例不适用——该 var 属
`internal/modules/airesource/models/chat`（A6 领地），与本次搬迁无关。

## 3. 禁改共享文件 file:line 清单与切换步骤

Pass A 结束时（本分支 HEAD），禁改共享文件中与本模块相关的精确行：

**需要切换的（唯一 1 个文件）**：

- `internal/container/container.go:52` — `chatpipeline "github.com/Tencent/WeKnora/internal/application/service/chat_pipeline"`（import 行）
- `internal/container/container.go:644` — `must(container.Provide(chatpipeline.NewEventManager))`
- `internal/container/container.go:645-661` — 17 行 `must(container.Invoke(chatpipeline.NewPluginXxx))`（Search/Rerank/WebFetch/Merge/DataAnalysis/IntoChatMessage/ChatCompletion/ChatCompletionStream/FilterTopK/QueryUnderstand/LoadHistory/MemoryRecall/ExtractEntity/SearchEntity/SearchParallel/WikiBoost/MemoryAffinity）
- `internal/container/container.go:700` — 仅注释提及 `*chatpipeline.EventManager`（非代码引用，切换后语义不变）

:644-661 与 :700 不需要改行——import 切换后 qualifier 不变自动解析。

**不需要切换的（零 chat_pipeline 引用，已 grep 验证）**：

- `internal/router/router.go` — 零处引用本模块包
- `internal/router/task.go:271,326`、`internal/router/sync_task.go:146,165` — 注册
  `types.TypeTemporaryDocumentProcess` / `types.TypeQueryHistoryExport`
  两个 worker（常量在 platform `internal/types/task.go:256,258`，不经 chat_pipeline）
- `go.mod`、`go.sum`、`migrations/` — 零处引用

**给集成者的操作**：
1. 在 `internal/container/container.go:52` 把导入改为
   `chatpipeline "github.com/Tencent/WeKnora/internal/modules/conversation/chat_pipeline"`
   （唯一改动行；qualifier `chatpipeline` 不变，其余行零改动）；
2. `go build ./...` 通过后，删除 `internal/application/service/chat_pipeline/` 别名目录
   （连同 alias.go）——此刻已无任何 importer；
3. 回归：`go test ./internal/modules/conversation/... -count=1` +
   `go test ./internal/container/... ./internal/application/service/... ./internal/handler/... ./internal/router/... -count=1` +
   `go run ./tools/modulemove verify --module conversation` +
   `go run ./tools/architectureguard`（路由 633 / worker 23+23 / hooks 58 不得变化）。

## 4. 路由集成点（routes，7 项；基线总数 633 不变）

全部经 **非禁改** 的 `internal/router/routes_chat.go` / `routes_query_history.go` /
`files.go` 注册（这些文件 import 的是 `internal/handler` 与 `internal/handler/session`，
不 import chat_pipeline，Pass A 未触碰、IA3 亦无需触碰）：

| 注册入口 | 定义 | 挂载（router.go） |
|---|---|---|
| RegisterMessageRoutes | routes_chat.go:20 | router.go:376 |
| RegisterFeedbackRoutes | routes_chat.go:49 | router.go:377 |
| RegisterSessionShareRoutes | routes_chat.go:72 | router.go:364 |
| RegisterSessionRoutes（38 条中 conversation 27；craft 分组归 craft） | routes_chat.go:95 | router.go:363 |
| RegisterChatRoutes（knowledge-chat/agent-chat/knowledge-search） | routes_chat.go:237 | router.go:375 |
| RegisterQueryHistoryAdminRoutes（Admin+ 审计查询） | routes_query_history.go:16 | router.go:378 |
| serveMessageScopedFiles（会话消息附件代理） | files.go:485 | router.go:346 |

Session/message 流式端点（SSE）与分享 token 语义为外部契约：share token 仅由
`POST /share` 一次性揭示（`internal/handler/session/share.go:38-61`，
`Session.ShareToken` json:"-"），Pass B 拆分时不得改变。

## 5. Workers（2 项；Redis/Lite 同集合，基线 23+23 不变）

| 任务类型 | 常量 | 注册（async / Lite） | 处理器 |
|---|---|---|---|
| TypeTemporaryDocumentProcess = `temporary_document:process` | internal/types/task.go:256 | router/task.go:271 / sync_task.go:146 | params.TemporaryDocument.Process |
| TypeQueryHistoryExport = `query_history:export` | internal/types/task.go:258 | router/task.go:326 / sync_task.go:165 | params.QueryHistoryExport.ProcessExport |

任务类型常量在 platform `internal/types`（非本模块文件）；Pass B 收敛门面时再议归属。

## 6. container.Invoke 生命周期挂点（3 项；基线 hooks 58 不变）

- `startTemporaryDocumentCleanup` — container.go:618 Invoke（函数定义 container.go:2421）：
  临时文档过期清理循环。
- `registerChatLocalImageResolver` — container.go:1034 Invoke（函数定义 container.go:1123）：
  为 chat pipeline 本地图片解析注册回调。**注意**：该回调赋值的是 airesource
  `models/chat.LocalImageResolver` 可变 var（A6 领地、batch-a2 冻结面 §models/chat），
  F0 因服务 chat 图片流而把挂点记在本模块名下——IA3 切换时本挂点**不需要**改行，
  该 var 的归属见 `frozen-entrypoints-batch-a2.md`。
- chatpipeline plugins ×17 — container.go:645-661（见第 3 节；随 ：52 import 切换自动解析）。

## 7. 配置键（模块读取）

- env：`WEKNORA_CHAT_ATTACHMENT_TTL_HOURS`、`WEKNORA_CHAT_ATTACHMENT_WAIT_TIMEOUT_SEC`
  （会话附件 staging，`internal/application/service/session_attachment_staging.go`）。
- config 结构体字段：`config.FollowUps.{Enabled,AllowRegenerate,SuppressWhenAnswerAsksQuestion,SuppressOnFallback,ModelID}`（追问建议）、
  `config.Conversation.{RewritePromptUser,RewritePromptSystem,IntentSystemPrompts}`（查询改写/意图）、
  `config.ExtractManager.ExtractEntity`（实体抽取开关）、`config.Tenant.IsRBACEnforced`（会话 RBAC 分支）。
- Query History CSV 导出与 share token 无自有 env 键（列集合 `queryHistoryExportColumns`
  为代码内契约，`internal/application/service/query_history_export.go:21`）。

## 8. Guard 新增发现（A10 Pass A 不修，交 IA3 处置）

architectureguard 在本分支报 **6 条新 forbidden-import** 诊断（搬迁前同分支基线 0 违规；
均为**先于本次改造**的耦合被搬迁暴露——同 6 个文件在旧路径
`internal/application/service/chat_pipeline/` 下就有完全相同的 import 行）：

1. `internal/modules/conversation/chat_pipeline/common.go` → `internal/modules/airesource/models/chat`
2. `internal/modules/conversation/chat_pipeline/data_analysis.go` → `internal/modules/airesource/models/chat`
3. `internal/modules/conversation/chat_pipeline/extract_entity.go` → `internal/modules/airesource/models/chat`
4. `internal/modules/conversation/chat_pipeline/query_understand.go` → `internal/modules/airesource/models/chat`
5. `internal/modules/conversation/chat_pipeline/references.go` → `internal/modules/airesource/models/chat`
6. `internal/modules/conversation/chat_pipeline/rerank.go` → `internal/modules/airesource/models/rerank`

张力点：这些 import 走的正是 batch-a2 冻结面**文档化的入口路径**
（`frozen-entrypoints-batch-a2.md` §airesource 列出 `models/chat`、`models/rerank`，
并注明 chat 模型门面"A10/A11 最大消费面"），而 guard 的跨模块规则要求
"只经模块根公共门面"。**处置建议（IA3）**：参照 appconnector 先例为这 6 条精确路径
加临时例外（挂 `B-conversation`），或由 airesource 门面收敛（Pass B）；
不得为过 guard 而回退本搬迁。计数器不受影响：633/23+23/58/16 与 F0 基线一致。

## 9. 验收命令（集成后必跑）

- `go build ./...`
- `go test ./internal/modules/conversation/... -count=1`（manifest test_commands）
- `go test ./internal/application/service/... ./internal/handler/... ./internal/router/... ./internal/container/... -count=1`（直接消费方回归，含 session/message 流式与路由 characterisation）
- `go run ./tools/modulemove verify --module conversation`
- `go run ./tools/architectureguard`（633/23+23/58/16 不得变化；第 8 节 6 条诊断处置前允许存在，其余必须 0）
