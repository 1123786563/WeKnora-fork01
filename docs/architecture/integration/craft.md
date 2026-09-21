# Integration Brief — craft (IA4)

适用对象：Pass A 集成者（Integrator）。任务：把 `bm-passa-a13` 分支的搬迁成果合入集成线。
本文件只描述 craft 模块（manifest：`docs/architecture/moves/craft.yaml`），不动 workbench /
insights 领地。计数基线：`docs/architecture/backend-baseline.md`（routes 633 / redis 23+lite 23 /
hooks 58）。

## 1. 本次搬迁（A13 已完成，SHAs 见 evidence 文档）

| from（旧导入路径） | to（新导入路径） |
|---|---|
| `internal/craft` | `internal/modules/craft` |

单包搬迁（41 个 `.go` = 21 非测试 + 20 测试，含 6 个 testdata 文件，共 47 个 rename），
move commit 为纯 rename（47/47 相似度 100%，0 insertions / 0 deletions）。
目标目录原有的 F1 骨架（`module.go`、`README.md`、`legacy/README.md`）与搬入包同为
`package craft`，合并为同一包，无冲突。包内唯一外部依赖是
`internal/modules/agentruntime/agent/runtime` 与 `internal/modules/commercial`（均为既冻结的
模块路径，搬迁前后零变化）。

## 2. 旧路径别名（alias_obligations，1 处）

旧路径留了一个**零逻辑**转发别名包：`internal/craft/alias.go`，头部注明
`Deleted by Pass B task B-craft`。

别名面 = **7 个符号 = `internal/container/container.go` 对旧包的精确引用面**（最小面，
同 A11 口径；A8 曾取全量导出面，两种口径都合规）：

| 符号 | 种类 | container.go 引用行 |
|---|---|---|
| `Store` | type（interface） | :2515 |
| `VersionStore` | type（interface） | :2516、:2538 |
| `PreviewCheckStore` | type（interface） | :2540 |
| `Workspace` | type（struct） | :2624 |
| `Executor` | type（interface） | :2617、:2667 |
| `KindWeb` | const | :2503 |
| `KnownKind` | func（var 转发） | :2498 |

全部为 type alias / const / var 转发，无可变导出 var（A6 的 container.go 赋值翻转豁免
本案**不需要**——container.go 对 craft 符号零赋值，已 grep 验证）。

## 3. 禁改共享文件中仍指向旧路径的精确行（file:line 清单）

Pass A 结束时（本分支 HEAD `e389562a0`），禁改共享文件中引用旧导入路径的行**只剩**：

- `internal/container/container.go:42` — `"github.com/Tencent/WeKnora/internal/craft"`

其余引用行（:2498/:2503/:2515/:2516/:2538/:2540/:2614/:2617/:2624/:2667）经别名解析，
**不需要改**，import 切换后自动解析到新路径。

`internal/router/router.go`、`internal/router/task.go`、`internal/router/sync_task.go`、
`go.mod`、`go.sum`、`migrations/`：**零处引用**（已 grep 验证；craft 的路由入口全部在
非禁改文件内，见 §4）。

**给集成者的操作（IA4 切换）**：
1. 在 `internal/container/container.go:42` 把导入改为
   `"github.com/Tencent/WeKnora/internal/modules/craft"`（import 行唯一改动，包名
   qualifier `craft` 不变，`container.go` 其余行零改动）；
2. `go build ./...` 通过后，删除 `internal/craft/` 整个目录（此刻已无任何 importer）；
3. 回归：`go build ./...` + `go test ./internal/modules/craft/... -count=1` +
   `go test ./internal/container/... ./internal/application/service/... ./internal/application/repository/... ./internal/handler/... ./internal/handler/session/... -count=1`；
4. `go run ./tools/architectureguard`：§8 的 4 条 forbidden-import 诊断按 IA4 例外/切换
   方案处置（不得为过 guard 回退本搬迁）；路由 633 / redis 23+lite 23 / hooks 58 不得变化。

## 4. 路由集成点（routes，F0 计 28 条字面注册）

Pass A 后全部路由注册代码仍在其原有文件内（均为**非禁改**文件或禁改文件中的
非 import 挂载行），集成时不要动语义：

- craft 分组路由（inside `RegisterSessionRoutes`）：`/craft/sessions`、
  `/craft/scheduled-tasks`、`/craft/model-gateway` — `internal/router/routes_chat.go:95-235`
  （三个分组挂在 :103/:123/:135）；
- craft model gateway 顶层（pre-auth）— `internal/router/router.go:262-265`（禁改文件内
  的挂载调用行，本次搬迁未触碰；该处经 handler 包间接消费，无 craft import）；
- handler 侧委托（legacy 文件，Pass B 拆分）：
  `RegisterCraftSessionRoutes`（12 routes）— `internal/handler/session/craft.go:116-154`；
  `RegisterCraftInteractionRoutes`（4 routes）— `internal/handler/session/craft_interaction.go:63`；
  `RegisterCraftScheduledTaskRoutes`（7 routes）— `internal/handler/session/craft_scheduled.go:77`；
  `RegisterCraftPreviewRoutes`（隔离预览源 `/p/:cap/*filepath`）—
  `internal/handler/session/craft_preview.go:55`（`RegisterCraftPreviewIssueRoute` 休眠
  挂载已计数，`craft.go:151-153` 内联注册）。

基线路由总数 633 不变（guard 复核 literal=564 + apiKeyRoute=69 + handle=0）。

## 5. Workers

**0 项**（manifest `integration_points.workers: []`）。craft 无 asynq 任务类型；
`internal/router/task.go`、`internal/router/sync_task.go` 无本模块条目（已 grep 验证）。
后台语义由 §6 的 `startCraftDecisionDelivery` 生命周期挂点承载，不是 worker 池。

## 6. container.Invoke 生命周期挂点（9 项）

全部挂点行仍在 `internal/container`（本次未触碰；其中 :665/:668/:680/:684/:1038 位于禁改的
container.go 内，:681/:682 位于 craft_interaction.go、:686/:689 位于 craft_lifecycle.go——
后四个文件属可改 importer，本次仅 import 行修复）：

| Invoke 行 | 挂点 | 实现位置 |
|---|---|---|
| container.go:665 | registerCraftHTTPHandlers | container.go:2682 |
| container.go:668 | registerCraftScheduledHTTPHandlers | container.go:2701 |
| container.go:680 | wireCraftInteractionRegistrar | container.go:2666 |
| container.go:681 | registerCraftInteractionHTTPHandlers | internal/container/craft_interaction.go:81 |
| container.go:682 | startCraftDecisionDelivery | internal/container/craft_interaction.go:92 |
| container.go:684 | validateCraftKnowledgeAssembly | container.go:2650 |
| container.go:686 | registerCraftUsageHTTPHandlers | internal/container/craft_lifecycle.go:101 |
| container.go:689 | wireCraftLifecycleIntegration | internal/container/craft_lifecycle.go:116 |
| container.go:1038 | wireCraftSessionTombstone | internal/container/craft_lifecycle.go:136 |

## 7. 配置键（模块读取）

**无自有 env/配置键**：`internal/modules/craft` 非测试代码零 `os.Getenv`/`os.LookupEnv`、
零 `config.` 引用（已 grep 验证）。依赖（sandbox、commercial、agentruntime runtime）全部经
接口注入；container 侧装配读取的配置属装配层，不属于本模块。

## 8. Guard 新增发现（A13 Pass A 不修，交 IA4 处置）

architectureguard 在本分支报 **4 条新** forbidden-import 诊断（IA3 基线 0 违规；搬迁把
旧 platform 路径变成模块内部路径，暴露出**先于本次改造**的跨模块耦合；完整输出存档
`/tmp/a13-guard.log`）：

1. `internal/modules/agentruntime/agent/opencode/executor.go` 导入 `internal/modules/craft`
   （搬迁前导入 `internal/craft`，耦合预存；A13 仅按 manifest importers 做了 import 行修复，
   文件属 agentruntime 领地）
2. `internal/modules/agentruntime/agent/opencode/normalizer.go` 导入 `internal/modules/craft`（同上）
3. `internal/modules/agentruntime/agent/tools/craft_delegate.go` 导入 `internal/modules/craft`（同上）
4. `internal/modules/craft/contracts.go` 导入 `internal/modules/agentruntime/agent/runtime`
   （耦合预存：搬迁前 `internal/craft/contracts.go` 即导入同一模块路径；此前 importer
   `internal/craft` 不在模块目录下故不触发；搬迁后 importer 变为模块内部路径而显形——
   IA3 例外登记表按精确 importer 文件路径匹配，需 IA4 增补/迁移对应条目）

**处置建议（IA4/Pass B）**：参照 appconnector/channels 先例，在 guard 例外表加精确路径
例外（临时，标注 Pass B 任务）或改经模块根门面（Pass B 语义收敛）；不得为过 guard 而
回退本搬迁。共享契约 Task/Timeline/Artifact（workbench/insights 同域消费）签名零变化。

## 9. 验收命令（集成后必跑）

- `go build ./...`
- `go test ./internal/modules/craft/... -count=1`（manifest test_commands）
- `go test ./internal/container/... ./internal/application/service/... ./internal/application/repository/... ./internal/handler/... ./internal/handler/session/... ./internal/modules/agentruntime/agent/tools/... -count=1`（直接消费方回归；opencode 全套按 F0 §2.5 已知不稳定处理，可用 `-run Craft` 定点）
- `go run ./tools/modulemove verify --module craft`
- `go run ./tools/architectureguard`（路由 633 / redis 23+lite 23 / hooks 58 不得变化；
  第 8 节 4 条 forbidden-import 诊断在集成处置前允许存在，其余必须 0）
