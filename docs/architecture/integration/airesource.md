# Integration Brief — airesource (IA2)

适用对象：Pass A 集成者（Integrator）。任务：把 bm-passa-a6 分支的搬迁成果合入集成线。
本文件只描述 airesource 模块（manifest：`docs/architecture/moves/airesource.yaml`），
不动其他模块领地。

## 1. 本次搬迁（A6 已完成，SHAs 见 evidence 文档）

| from（旧导入路径） | to（新导入路径） |
|---|---|
| `internal/infrastructure/web_search` | `internal/modules/airesource/web_search` |
| `internal/mcp` | `internal/modules/airesource/mcp` |
| `internal/models/asr` | `internal/modules/airesource/models/asr` |
| `internal/models/chat` | `internal/modules/airesource/models/chat` |
| `internal/models/embedding` | `internal/modules/airesource/models/embedding` |
| `internal/models/limiter` | `internal/modules/airesource/models/limiter` |
| `internal/models/provider` | `internal/modules/airesource/models/provider` |
| `internal/models/rerank` | `internal/modules/airesource/models/rerank` |
| `internal/models/utils` | `internal/modules/airesource/models/utils` |
| `internal/models/utils/ollama` | `internal/modules/airesource/models/utils/ollama` |
| `internal/models/vlm` | `internal/modules/airesource/models/vlm` |
| `internal/storageurl` | `internal/modules/airesource/storageurl` |

共 188 个文件（含全部 `_test.go`；`internal/models/*` 按 manifest ruling 保留 `models/` 父层），
move commit 为纯 rename（188/188 相似度 100%，0 insertions / 0 deletions）。

## 2. 旧路径别名（alias_obligations，12 处）

每个旧路径留了一个**零逻辑**转发别名包（type alias + var/const/func 值转发；
`internal/models/utils` 另含 2 个泛型函数的一行委托转发；每文件头部注明
`Deleted by Pass B task B-airesource`）：

| 旧路径（别名包目录） | 转发目标 | 别名文件（符号数） |
|---|---|---|
| `internal/infrastructure/web_search` | `internal/modules/airesource/web_search` | `alias.go`（38） |
| `internal/mcp` | `internal/modules/airesource/mcp` | `alias.go`（33） |
| `internal/models/asr` | `internal/modules/airesource/models/asr` | `alias.go`（9） |
| `internal/models/chat` | `internal/modules/airesource/models/chat` | `alias.go`（40） |
| `internal/models/embedding` | `internal/modules/airesource/models/embedding` | `alias.go`（50） |
| `internal/models/limiter` | `internal/modules/airesource/models/limiter` | `alias.go`（10） |
| `internal/models/provider` | `internal/modules/airesource/models/provider` | `alias.go`（110） |
| `internal/models/rerank` | `internal/modules/airesource/models/rerank` | `alias.go`（49） |
| `internal/models/utils` | `internal/modules/airesource/models/utils` | `alias.go`（3） |
| `internal/models/utils/ollama` | `internal/modules/airesource/models/utils/ollama` | `alias.go`（4） |
| `internal/models/vlm` | `internal/modules/airesource/models/vlm` | `alias.go`（11） |
| `internal/storageurl` | `internal/modules/airesource/storageurl` | `alias.go`（28） |

别名面 = 搬迁包的全部导出符号（go/ast 枚举生成，共 385），保证禁改文件及任何集成期
引用可解析。**注意**：`chat.LocalImageResolver` 的别名 `var` 只是值拷贝 —— Go 无 var
alias，无法转发"赋值"语义；见第 3 节，`internal/container/container.go` 对该符号的引用
已在 Pass A 直接切到新路径，其余场景请勿再通过旧路径**赋值**该变量（只读使用无害）。

## 3. 禁改共享文件中仍指向旧路径的精确行（file:line 清单，A6 分支 HEAD）

Pass A 结束时（本分支 HEAD），禁改共享文件中引用旧导入路径的行**只剩**：

- `internal/container/container.go:67` — `infra_web_search "github.com/Tencent/WeKnora/internal/infrastructure/web_search"`
- `internal/container/container.go:69` — `"github.com/Tencent/WeKnora/internal/mcp"`
- `internal/container/container.go:70` — `"github.com/Tencent/WeKnora/internal/models/embedding"`
- `internal/container/container.go:71` — `"github.com/Tencent/WeKnora/internal/models/limiter"`
- `internal/container/container.go:72` — `"github.com/Tencent/WeKnora/internal/models/utils/ollama"`

**偏差说明（A6 已完成的一步）**：`internal/container/container.go` 原第 70 行
`"github.com/Tencent/WeKnora/internal/models/chat"` 已在 A6 repair commit 中**改为**
`"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"`。原因：`container.go:1133`
（`registerChatLocalImageResolver`）对 `chat.LocalImageResolver` 赋值，该符号是可变导出 var
（`internal/modules/airesource/models/chat/image_resolve.go:63`），var 转发别名无法保留赋值
语义，会静默破坏多租户 `local://` 图片解析。除这一行 import 外 `container.go` 零 diff；
这正是 IA2 原本要做的机械步骤之一，且消除了别名桥接期的行为回归窗口。

`internal/router/router.go`、`internal/router/task.go`、`internal/router/sync_task.go`、
`go.mod`、`go.sum`、`migrations/`：零处引用（已 grep 验证；router 经 `internal/handler`
间接使用本模块，无直接导入）。

**非禁改但非本模块领地的残留**（channels 模块文件，A6 未触碰，别名包当前为其解析）：

- `internal/modules/channels/im/service.go:30` — `mcppkg "github.com/Tencent/WeKnora/internal/mcp"`
- `internal/modules/channels/im/service.go:32` — `"github.com/Tencent/WeKnora/internal/storageurl"`

（来源：manifest `owned_files.importers` 中的 `internal/im` 在 batch A1 已迁至
`internal/modules/channels/im`；按并行批次边界规则 A6 不修改其他模块 owned 文件。）

**给集成者的操作**：
1. 在 `internal/container/container.go:67/69/70/71/72` 把 5 行 import 改为对应
   `internal/modules/airesource/...` 路径（qualifier 不变，其余行零改动）；
2. 顺手把 `internal/modules/channels/im/service.go:30/32` 两行 import 切到
   `internal/modules/airesource/mcp`、`internal/modules/airesource/storageurl`
   （该文件属 channels 领地，亦可留给 channels owner）；
3. 改完后运行 `go build ./...` 通过，再删除第 2 节所列 12 个别名目录（连同 alias.go），
   再次 `go build ./...` + `go test ./internal/modules/airesource/... -count=1` 确认。
   （当前仍在使用别名的只有上述 container.go 5 行与 channels/im 2 行；其余 5 个别名包
   `asr/chat/embedding/rerank/vlm`、`utils`、`utils/ollama`、`storageurl`*、`web_search`*
   无任何 importer，可先删。*storageurl/web_search 仅 channels/im / container 各自引用。）

## 4. 路由集成点（routes，共 7 项入口 / 61 条路由）

全部在 `internal/router/routes_infra.go`（禁改文件；Pass A 未触碰），按入口统计：

| 注册入口（routes_infra.go） | 路由数 | 使用的 legacy handler（Pass B `B-airesource` 拆出） |
|---|---|---|
| `RegisterModelRoutes` — :23 | 9 | `internal/handler/model.go`（`NewModelHandler` :35）、`internal/handler/model_credentials.go`（`NewModelCredentialsHandler` :25） |
| `RegisterMCPServiceRoutes` — :145 | 22 | `internal/handler/mcp_service.go`（:31）、`internal/handler/mcp_credentials.go`（:35）、`internal/handler/mcp_oauth.go`（:30） |
| `RegisterWebSearchRoutes` — :215 | 1 | `internal/handler/web_search.go`（:14） |
| `RegisterWebSearchProviderRoutes` — :230 | 10 | `internal/handler/web_search_provider.go`（:26）、`internal/handler/web_search_provider_credentials.go`（:25） |
| `RegisterVectorStoreRoutes` — :261 | 8 | `internal/handler/vectorstore.go`（:22） |
| `RegisterStorageBackendRoutes` — :280 | 9 | `internal/handler/storagebackend.go`（:31） |
| `RegisterWeKnoraCloudRoutes` — :363 | 2 | `internal/handler/weknoracloud.go`（:16） |

注：A6 只对上述 legacy handler 文件做了 **import 行修复**（`internal/handler` 是 manifest
声明的 importer；构造器与路由挂载在 Pass A 不动）。模型/MCP/WebSearchProvider 的凭据
子资源写路由全部 Admin+，凭据读写边界未变。

## 5. Workers（Redis + Lite 任务类型）

**0 项**（manifest `integration_points.workers: []`；已验证搬迁树零 `hibiken/asynq` import，
相关注释仅描述 limiter 与 asynq 队列层的协作关系）。无需改 router/task.go、router/sync_task.go。

## 6. container.Invoke 生命周期挂点（3 项，禁改文件内，集成时不要动语义）

| 挂点 | Invoke 位置 | 函数体位置 |
|---|---|---|
| `registerWebSearchProviders`（14 个 provider 注册进 `infra_web_search.Registry`） | `internal/container/container.go:439` | `container.go:2263` |
| `registerModelConcurrencyLimiter`（Redis governor） | `internal/container/container.go:604` | `container.go:1216` |
| `registerLiteModelConcurrencyLimiter`（Local governor） | `internal/container/container.go:615` | `container.go:1232` |

相关第 4 项挂点 `registerChatLocalImageResolver`（`container.go:1039` Invoke，函数体
`container.go:1128`）给 `chat.LocalImageResolver` 赋值 —— 见第 3 节偏差说明，其 import
已指向新路径，集成时只需保持不动。

## 7. 配置键（模块读取，env）

搬迁树内直接读取的环境变量（grep `os.Getenv` 全量）：
`ALIYUN_API_KEY`、`APP_EXTERNAL_URL`、`BATCH_EMBED_SIZE`、`DEEPSEEK_API_KEY`、
`LOCAL_STORAGE_BASE_DIR`、`OLLAMA_BASE_URL`、`OLLAMA_OPTIONAL`、
`VLM_HTTP_TIMEOUT_SECONDS`、`WEKNORA_LLM_STREAM_RAW_DUMP`、
`WEKNORA_LLM_STREAM_RAW_DUMP_DIR`、`WEKNORA_REDIS_NAMESPACE`。

**凭据/秘密边界（外部契约，Pass A 未动）**：模型与 Provider 凭据的**存储**仍在既有
storage 边界（`internal/application/{service,repository}` 的 model/resource/mcp 等 legacy
文件 + storage backend 配置），`internal/modules/airesource/models/provider` 只承载
Provider 协议适配（鉴权头构造等），不落盘、不迁移任何秘密。Pass B `B-airesource` 拆分
legacy 文件时不得改变凭据存取模式。

## 8. 验收命令（集成后必跑）

- `go build ./...`
- `go test ./internal/modules/airesource/... -count=1`
- `go test ./internal/container/... ./internal/handler/... ./internal/agent/... ./internal/application/... ./internal/modelcontext/... ./internal/modules/channels/im/... -count=1`
- `go run ./tools/modulemove verify --module airesource`
- `go run ./tools/architectureguard`（基线：633 路由 / 23+23 worker / 58 hooks，不得变化；
  已知遗留：`airesource/models/chat/usage.go` 对 commercial 模块内部的 pre-existing
  耦合，见 evidence 文档 §6，归 IA2 裁定，勿在工具加例外）
