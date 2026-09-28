# Pass B 子计划 32-agentruntime-tools — R2 AgentRuntime 工具边界收敛

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. 步骤使用 checkbox（`- [ ]`）跟踪。
>
> **节点：** b3-r-tools（DAG id，execution_mode=parallel，depends_on=ib2）。分支：`codex/passb-b3-r-tools`，基线 = 派发时 integration HEAD（base_sha 以 DAG 回填为准，本计划撰写时 = `a2fbcf55e`）。
> **程序协调计划：** `docs/plans/passb/30-agentruntime-program.md`（R0–R5 总序；本计划为其 R2 子计划）。
> **职责一句话：** `internal/modules/agentruntime/agent/tools` 成为工具子系统唯一入口——把 19 个工具生产文件对 execution/sandbox、execution/browserskill、airesource/mcp、airesource/models/rerank、airesource/models/chat、knowledge/searchutil 的深 import 收敛为包内消费侧 seam（spec §4.4 过渡薄别名），并落盘 craft→agentruntime 未导出调用（runScope 族）的导出端口裁定；工具外部契约（工具名/JSON schema/审批语义/journal 事件/注册面计数）零变化。

## 0. 事实源与前置条件

### 0.1 事实源指针（撰写本计划时全部实读）

- Spec：`docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §4.2/§4.4（依赖方向与两遍迁移）、§5.5（Agent Runtime 职责）、§14（差分门禁）、§15（治理规则）。
- 框架计划：`docs/plans/2026-09-23-backend-modularization-pass-b-framework.md`（B3 节、Child Plan Authoring Order 第 4 条）。
- 冻结计划：`docs/plans/passb/00-contract-and-ownership-freeze.md`（B0.3 裁定 1/2；:193-196 R 面 ownership）。
- 公约：`.superpowers/sdd/passb/conventions.md`（§1 派发契约、§1.2 报告包命令、§3 禁改清单、§5 升级契约、§6 差分证据、§7 package-private 耦合、§8 计数基线、§10 裁定族）。
- B0 冻结产物（integration 分支 `.worktrees/passb-int` 同 SHA）：`docs/architecture/passb/ownership-matrix.yaml`（agentruntime 45 行 = 38×33 + 5×31 + 2×34，**无 plan=32 行**，python Counter 实证）、`contracts.yaml`、`event-catalog.yaml`、`exception-ledger.yaml`（142 行，其中 plan=32-agentruntime-tools 恰 20 行 = exc-0028..exc-0047）。
- Brief：`docs/architecture/passb/agentruntime-tools.md`（本计划直接依据，三条义务逐条落为任务）。
- 集成 brief：`docs/architecture/passb/briefs/ib2.md`（B3 前置提醒 1–4：计数基线 633/23+23/58、matrix legacy 358、exceptions 142、aliases 69；门面合法化契约任务非 B3 单方面可删）。
- manifest：`docs/architecture/moves/agentruntime.yaml`（legacy_files 45 行全属 R1/R3/R4；alias_obligations 20 行的旧 alias 包已物理删除，passbguard `aliases=69` 不含 agentruntime 20 行）。
- integration 报告：`docs/architecture/integration/agentruntime.md` §10（49 条预存 forbidden-import 清单，tools 侧边）。
- 调度台账：`docs/architecture/passb/execution-ledger.md`（IB1 屏障执行 2026-09-23 20:47 条目：B1 四支实施产出止于计划文档、无生产代码，四模块 module.go 仍为零逻辑骨架）。

### 0.2 前置条件（开工前逐条核对，任一不满足即按 conventions §5 上报）

1. **ib2 done**：DAG `ib2.status=done`，head=`a2fbcf55e`（2026-09-28 10:28 收口；本节点 2026-09-28 阻塞句解除条件已满足）。核对命令：`cd .worktrees/passb-int && python3 -c "import json;d=json.load(open('docs/plans/passb/execution-dag.json'));print([ (n['id'],n['status']) for n in d['nodes'] if n['id'] in ('ib2','b3-r-tools')])"` → 期望 `[('ib2','done'),('b3-r-tools','in_progress')]`。
2. **R0 复核（freeze:193-196）**：native_archive→34、native_recovery 与 native repository 状态族→33、approval/run/attempt/checkpoint/decisions/events/inputs/lifecycle/tools journal→33。对 R2 的推论（grep 实证）：`ownership-matrix.yaml` 中 **plan=32-agentruntime-tools 的 legacy 行数为 0**——R2 无遗留文件迁入（brief Scope 同句）。`internal/application/repository/agent_run_tools.go`（toolCallScope 定义处）属 33-engine，**本节点禁写**。
3. **基线对齐**：本 worktree HEAD 必须包含 ib2 收口提交 `a2fbcf55e`（`git merge-base --is-ancestor a2fbcf55e HEAD && echo OK`）。若协调者后续追加了新的 integration 提交，按 Ruling 2026-09-24-WAVE-DEP-BASELINE 先做基线对齐 merge 再开工。
4. **门面现实核验（关键，决定收敛路径）**：`internal/modules/execution/module.go` 与 `internal/modules/airesource/module.go` 均为 22 行零逻辑骨架（`head -30` 实证），**IB1 冻结门面未实装**；`internal/modules/knowledge/module.go` 五操作门面已实装但**不 re-export searchutil 符号**；`internal/modules/appconnector` 与 `internal/modules/craft` 根包是实包（access/action/oc_binding/contracts 等）。且 architectureguard 的 forbidden-import 对**模块根 import 同样报诊断**（本计划撰写会话以探针文件实证：tools 包内新建仅 import `internal/modules/execution` 根的文件 → `forbidden-import: ... 导入了模块 execution 的内部包 "github.com/Tencent/WeKnora/internal/modules/execution"`，随后已删除探针、工作树干净）。推论：本节点写权限内**无法把任何一条边改成"经对方模块根门面"的合法形态**——被导入包根 re-export 与 guard 根合法化均属 barrier 契约任务（ib2.md 前置提醒 3 同口径）。
5. **T0 基线（撰写会话已跑，实施者开工时复跑刷新）**：
   - `go test ./internal/modules/agentruntime/agent/tools/ -count=1` → `ok ... 15.030s`（本会话实测，仅既有 `-lc++` 链接警告）；
   - `make check-backend-architecture` → `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16` + `OK (0 violations)`（实测）；
   - `make verify-module-moves` → `modulemove: OK (16 manifests verified)`（实测）；
   - `make check-passb-readiness` → `pass-b readiness: legacy=358 aliases=69 exceptions=142 contracts=125 events=29 overlaps=0 missing=0`（实测）；
   - `go test ./internal/modules/agentruntime/... -count=1`（节点 gate 全量）→ 本会话实测 **EXIT=0，20 包 ok / 0 FAIL**（opencode/recoverytest 慢包均过，仅既有 `-lc++` 链接警告）；实施者开工时复跑刷新。

### 0.3 写入所有权（精确）

**可写（owned_files 展开）：**
- `internal/modules/agentruntime/agent/tools/**`——生产与测试文件，含新建 seam 文件与特征化测试；
- 节点自身产物：`docs/plans/passb/reports/b3-r-tools.md`、`docs/architecture/evidence/passb/b3-r-tools.md`、`docs/architecture/passb/briefs/b3-r-tools.md`；
- **数据行级**（Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP / 2026-09-24-IMPORT-EXCEPTION-REGISTRY）：`tools/architectureguard/check.go` 中 `importExceptions` 复合字面量里 **from=internal/modules/agentruntime/agent/tools/** 的行（仅数据行，禁改判定逻辑），与 `docs/architecture/passb/exception-ledger.yaml` 中 **plan=32-agentruntime-tools 的行**——两侧同窗同 commit 增删。

**禁写（conventions §3）：** `internal/router/**`、`internal/container/**`、`internal/bootstrap/**`、`migrations/**`、`go.mod/go.sum`、其他模块任何文件（execution/airesource/knowledge/appconnector/craft 的根与子包）、`internal/application/**` 宿主包（含 agent_run.go、agent_run_tools.go、agent_service.go、craft_workspace.go、handler/session/**）、`ownership-matrix.yaml`/`contracts.yaml`/`event-catalog.yaml`（b0 建、barrier 回写）、`internal/modules/agentruntime/module.go` 门面注释（b0 与 R5 集成节点专属）。

### 0.4 现实基线裁定：收敛路径取"消费侧 seam + Integration Brief 交接"

Brief 义务 1 给出三种合法收敛手段。逐一对照现实（§0.2 条 4）：

| 手段 | 本节点可行性 |
|---|---|
| 改经对方模块根门面 | 不可行：execution/airesource 根为骨架、knowledge 根无 searchutil re-export；写对方根文件越权；guard 对根 import 亦报诊断（探针实证） |
| 抽端口由 container 注入 | 端口定义可在 tools 内先行，但装配切换在 container（集成工程师独占）；构造点在 `internal/application/service/agent_service.go`（matrix 归 25-agentcatalog），禁写 |
| 下沉为模块内接口（消费侧 seam） | **可行且本节点独占可完成**：spec §4.4"确需过渡时只能保留无业务逻辑的薄别名或 adapter，并记录删除阶段"；K1 先例 `internal/modules/knowledge/ingest/seams.go`（消费侧承载 chat.Chat 签名） |

**裁定：** R2 在 tools 包内建立三个消费侧 seam 文件（execution/airesource/knowledge 各一，§0.8 精确符号表），19 个生产文件的深 import 全部消除；seam 文件自身的新增边按 Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY 登记（精确 file→package、RemoveAt=ib3、同窗更新 exception-ledger 与 guard 数据行）；ib3 门面合法化契约任务落地时**每个家族只翻一个 seam 文件**。工具构造签名**零变化**（Go 类型别名透明，调用方 `agent_service.go` 无感）。appconnector/craft 两条边已是"模块根公共门面"形态（brief 目标形态），保留在册至根合法化，不为本节点消除对象。

### 0.5 兼容残差（seam）规则

- seam 只允许三种无逻辑形态：`type X = pkg.X`（类型别名，同一性保持）、`const C = pkg.C`、`func F(参数逐字同签名) 返回值 { return pkg.F(参数) }`（薄委托，单一真源）。**禁止**复制函数体、禁止 dot-import、禁止在 seam 内加业务逻辑。
- 每个 seam 符号带行注释 `// R2 seam（remove_at: ib3）— 门面合法化后随本文件消除`；seam 文件头注明属主计划与删除期限。
- seam 文件位于模块树内（非宿主包），**不触发** Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION 的 manifest/matrix 补行（该裁定只覆盖宿主包过渡 shim）；只需 guard importExceptions 数据行 + exception-ledger 行 + §8 计数登记。

### 0.6 runScope/toolCallScope/runView 导出端口裁定（节点裁定落盘；执行属主 = 33-agentruntime-engine 与 b4-craft，非本节点）

**事实（本会话 grep 实证，行号以基线 `a2fbcf55e` 为准）：**

| 符号 | 定义（R3 属主文件） | craft 消费点（B4 属主文件，B3 期间仍在宿主包） |
|---|---|---|
| `runScope` | `internal/application/repository/agent_run.go:83`，`func runScope(db *gorm.DB, key agentruntime.RunKey) *gorm.DB`（未导出） | `internal/application/repository/craft_workspace.go:266/:412` |
| `toolCallScope` | `internal/application/repository/agent_run_tools.go:50`，`func toolCallScope(tx *gorm.DB, key agentruntime.RunKey, callID string) *gorm.DB`（未导出） | `internal/application/repository/craft_workspace.go:284` |
| `runView` | `internal/handler/session/agent_run.go:124`，`func runView(r agentruntime.Run) gin.H`（未导出） | `internal/handler/session/craft.go:432/:486` |

三个定义文件在 ownership-matrix 均归 33-agentruntime-engine（agent_run.go/agent_run_tools.go → `internal/modules/agentruntime/repository`；handler/session/agent_run.go → 33）。

**裁定（沿 DAG b3-r-tools/b4-craft notes 原文"为 B4-craft 消费者导出 runScope 等端口或留 shim"具体化）：**

1. R3（33 计划）把上述文件搬出宿主包时，**必须**在目标包导出窄端口或留薄 shim，二选一或并用：
   - 目标包 `internal/modules/agentruntime/repository` 导出 `func RunScope(db *gorm.DB, key agentruntime.RunKey) *gorm.DB` 与 `func ToolCallScope(tx *gorm.DB, key agentruntime.RunKey, callID string) *gorm.DB`（薄委托单真源），宿主侧可另留 `runScope/toolCallScope` 同体委托 shim 过渡；
   - handler 侧目标包导出 `func RunView(r agentruntime.Run) gin.H`，或在 `internal/handler/session` 留 `runView` shim。
2. craft 侧调用点（craft_workspace.go/craft.go）的改写由 **b4-craft（41 计划）** 或集成工程师按其 Integration Brief 执行；本节点与 R3 均不改 craft 文件（conventions §7.2）。
3. 宿主 shim 若存在，按 Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION 成对补 manifest/matrix 行，**删除期限 = ib4**（41-craft 宿主文件搬迁屏障），不得拖至 b5。
4. **本节点的义务仅是落盘与交接**：本节写入计划（本节）+ Integration Brief `docs/architecture/passb/briefs/b3-r-tools.md`（R2.5 任务），供 33 计划撰写者与 b4-craft 直接引用；本节点不动上述任何文件（§0.3 禁写清单）。

### 0.7 公共可观察行为与兼容要求（不变式，全部差分/特征化锚点）

1. **工具外部契约冻结**（brief 义务 2）：工具名常量、`BaseTool.schema` JSON 字节、审批/门控语义（MCP approval gate、app connector 人审前置、`RequireApproval` 类标记）、journal 事件（registry journal 面）一概不变。特征化锚：`AvailableToolDefinitions()`（definitions.go:85）、`DefaultAllowedTools()`（definitions.go:116）、`persistStripFields*` 三张表（persist.go:10-33）。
2. **工具注册面（engine 工具数）不变**（brief 义务 3）：`TestEveryBuiltInToolDeclaresAModelHandlePolicy`（tool_policy_coverage_test.go:15）的内置清单与 `DefaultAllowedTools` 集合不得增减。
3. **构造签名零变化**：`NewShellExecTool(executor SandboxCommandExecutor, envResolver skills.SkillEnvResolver)`（shell_exec.go:262）、`NewMCPTool(service *types.MCPService, mcpTool *types.MCPTool, mcpManager *mcp.MCPManager, gate approval.MCPApproval, authWaitTimeoutSeconds int)`（mcp_tool.go:41）、`NewKnowledgeSearchTool(... rerankModel rerank.Reranker, cfg *config.Config)`（knowledge_search.go:139）、`NewCraftDelegateTool(cfg CraftDelegateToolConfig)`（craft_delegate.go:91）、`NewAppConnectorTool(actions OCActionFacade)`（app_connector.go:66）、`SanitizeMessages(messages []chat.Message) []chat.Message`（sanitize_messages.go:14）等——**形态不变**；seam 化后签名中的类型以别名书写（`rerank.Reranker`→`Reranker`），类型同一性不变，调用方（agent_service.go 等）零改动、零重编译差异。
4. **路由/Worker/生命周期零触碰**：本节点无 M4 装配变更（不写 router/container/bootstrap）；计数基线 633/23+23/58 逐 gate 复核。
5. **错误同一性**：`sandbox.ErrTimeout`、`craft.ErrUnknown`、`mcp.OAuthReauthorizationRequiredError` 经 `errors.Is/As` 判等的调用点，seam 化后仍指向同一真源值（var 别名/类型别名不产生第二实例）。
6. **高风险差分面**：本节点不触碰 Agent Run recovery / session stream / payment 主链；差分聚焦"工具行为等价"（§1 R2.6）：特征化测试迁移前后同用例双跑 + 全量 tools 包测试对比，证据入 evidence 差分章节（conventions §6）。

### 0.8 seam 精确符号清单（真实代码签名，实施时逐字对照）

**A. `execution_seams.go`（新建，包 tools）——import `execution/sandbox` + `execution/browserskill`：**

类型别名（定义位置实证）：
```go
type SessionBoundManager = sandbox.SessionBoundManager     // sandbox/session_manager.go:77
type ExecuteResult = sandbox.ExecuteResult                 // sandbox/sandbox.go:219
type ShellExecOptions = sandbox.ShellExecOptions           // sandbox/session_manager.go:709
type ShellOutputSnapshot = sandbox.ShellOutputSnapshot     // sandbox/session_manager.go:768
type RemoteDirEntry = sandbox.RemoteDirEntry               // sandbox/remote_client.go:370
type RemoteStatEntry = sandbox.RemoteStatEntry             // sandbox/remote_client.go:390
type SessionInstallShellExecutor = sandbox.SessionInstallShellExecutor // sandbox/capabilities.go:96（接口）
type Manager = browserskill.Manager                        // browserskill/manager.go:99
type Scope = browserskill.Scope                            // browserskill/manager.go:34
type Status = browserskill.Status                          // browserskill/manager.go:46
type AccountStatus = browserskill.AccountStatus            // browserskill/authorization.go:158
type RPCError = browserskill.RPCError                      // browserskill/errors.go:7
```
> 命名冲突注意：`Scope`/`Manager`/`Status` 在 tools 包内不得与现有标识符冲突（现状 grep：tools 包无同名顶层标识符；`browserskill.Scope` 仅 browserskill*.go 使用）。若实施时发现冲突，seam 内改用带前缀名（如 `BrowserScope`）并同步改写两个 browserskill 文件——以编译器为准，计划不预设。

常量/变量别名：
```go
const SessionWorkspaceRoot = sandbox.SessionWorkspaceRoot  // "/workspace"，session_manager.go:61
const SessionInputRoot = sandbox.SessionInputRoot          // "/workspace/input"，:42
const SessionOutputRoot = sandbox.SessionOutputRoot        // "/workspace/output"，:48
const SkillsImageRoot = sandbox.SkillsImageRoot            // skill_paths.go:14
var ErrTimeout = sandbox.ErrTimeout                        // sandbox.go:101
```
薄委托函数（签名逐字取自源）：
```go
func ResolveWorkspacePath(value string) string                        // workspace_path.go:10
func ShellQuote(s string) string                                     // shell_quote.go:12
func WithCommandOutput(ctx context.Context, callback func(string, []byte)) context.Context // command_output.go:9
func WithSessionFileOperation(ctx context.Context) context.Context   // session_file_operation.go:25
func IsValidSkillName(name string) bool                              // skill_paths.go:30
func SkillDirFor(skillName string) (string, error)                   // skill_paths.go:45
func ValidatedImageSkillDir(skillDir string) (string, bool)          // skill_paths.go:106
func SkillNameFromImagePath(p string) (name string, inImage bool)    // skill_paths.go:133
func NavigationIncomplete(method string, raw json.RawMessage) bool   // browserskill/result.go:9
```

**B. `airesource_seams.go`（新建）——import `airesource/mcp` + `airesource/models/rerank` + `airesource/models/chat`：**
```go
type MCPManager = mcp.MCPManager                             // mcp/manager.go:16
type MCPClient = mcp.MCPClient                               // mcp/client.go:25（接口）
type ContentItem = mcp.ContentItem                           // mcp/types.go:51
type CallToolResult = mcp.CallToolResult                     // mcp/types.go:45
type OAuthReauthorizationRequiredError = mcp.OAuthReauthorizationRequiredError // mcp/oauth_lifecycle.go:39
type Reranker = rerank.Reranker                              // rerank/reranker.go:14（接口）
type RankResult = rerank.RankResult                          // rerank/reranker.go:25
type Message = chat.Message                                  // chat/chat.go:79
```
> 符号级消费实证：mcp_tool.go 用 `mcp.CallToolResult`、`mcp.ContentItem`、`mcp.MCPManager`；mcp_oauth.go 用 `mcp.MCPClient`、`mcp.MCPManager`、`mcp.OAuthReauthorizationRequiredError`。
> `Message` 别名与现有 `chat.Message` 用法：仅 sanitize_messages.go/mcp_exposure.go 使用 chat；改写后签名 `func SanitizeMessages(messages []Message) []Message`。
> 命名冲突实证：tools 包现无 `type Scope/Manager/Status/Message/Reranker` 顶层标识符（grep 实证），别名可安全使用。

**C. `knowledge_seams.go`（新建）——import `knowledge/searchutil`（全部为薄委托函数）：**
```go
func BuildContentSignature(content string) string                                  // textutil.go:14
func TokenizeSimple(text string) map[string]struct{}                               // textutil.go:40
func Jaccard(a, b map[string]struct{}) float64                                     // textutil.go:77
func ClampFloat(v, minV, maxV float64) float64                                     // textutil.go:154
func CollectImageInfoByChunkIDs(ctx context.Context, chunkRepo interfaces.ChunkRepository, tenantID uint64, chunkIDs []string) map[string]string // imageinfo.go:55
func EnrichSearchResultsImageInfo(ctx context.Context, chunkRepo interfaces.ChunkRepository, tenantID uint64, results []*types.SearchResult)        // imageinfo.go:164
func BuildImageInfoMarkdownWithURL(url string, img *types.ImageInfo) string        // imageinfo.go:434
```

**D. 保留原样的两条根边（不 seam 化）：** `app_connector.go:10`（`appconn "…/internal/modules/appconnector"`，用 `OCSubject`（oc_binding.go:53）与 `Action*` 状态常量族（action.go:32 起））、`craft_delegate.go:17`（用 `ErrUnknown/Scope/Input/InputPath（craft/input.go:62 的包级 func）/Task/Result`，contracts.go:27-103）。二者已是 brief 目标形态"模块根公共门面"，例外行 exc-0028/exc-0031 保留至 guard 根合法化（barrier 契约任务），在 Brief 登记。

## 1. 任务分解（R2.1–R2.6；一任务一 commit，commit 模板见各任务尾）

### Task R2.1: 前置核验与特征化基线冻结（contract snapshot + T0）

**文件：** 新建 `internal/modules/agentruntime/agent/tools/contract_snapshot_test.go`；新建 `docs/architecture/evidence/passb/b3-r-tools.md`（§1 基线章节）。

**步骤：**
- [ ] 逐条执行 §0.2 前置 1–4 核对命令，结果（含命令原文/退出码）写入 evidence §1。
- [ ] 复跑 §0.2 条 5 的 T0 四条命令 + 节点 gate `go test ./internal/modules/agentruntime/... -count=1`，结果入 evidence §1（§6 差分的前拍）。
- [ ] 盘点 20 条例外边（`grep -n "agentruntime/agent/tools" .worktrees/passb-int/docs/architecture/passb/exception-ledger.yaml` + guard check.go 同名行），与 §0.8 清单逐符号比对（每文件 `grep -o "<pkg>\.[A-Z][A-Za-z]*"`），差异如实登记——计划清单若有遗漏符号，以实测为准补入 seam（记入报告勘误节，不改判定逻辑）。
- [ ] RED→GREEN：新建 `contract_snapshot_test.go`（表驱动）：
  - `TestAvailableToolDefinitionsContractSnapshot`：断言 `AvailableToolDefinitions()`（definitions.go:85，21 项 `{Name,Label,Description}` 三元组）整表逐字段相等；
  - `TestDefaultAllowedToolsSnapshot`：断言 `DefaultAllowedTools()` 精确切片（knowledge_search/grep_chunks/list_knowledge_chunks/get_document_info/search_conversations）；
  - `TestPersistStripTablesSnapshot`：断言 `persistStripFields`/`persistStripFieldsByTool`/`clientStripFieldsByTool` 三张映射的键集与值切片；
  - `TestConstructibleToolSchemaBytesSnapshot`：对可零依赖构造的工具断言 `Parameters()`（tool.go:37，`func (t *BaseTool) Parameters() json.RawMessage`）JSON 字节的 SHA-256：`NewAppConnectorTool(nil)`、`NewListSandboxFilesTool(nil)`、`NewWriteSandboxFileTool(nil, 0)`、`NewEditSandboxFileTool(nil)`、`NewShellExecTool(nil, nil)`、`NewWriteSkillFileTool(nil, "")`、`NewEditSkillFileTool(nil, "")`、`NewCraftDelegateTool(CraftDelegateToolConfig{Scope: 工具包内最小合法 Scope（TenantID=1,UserID="u",SessionID="s"）, WorkspaceID: "w"})`（schema 断言不触发 Delegate 调用）；MCP/wiki 族由既有 `mcp_catalog_regression_test.go`/`wiki_*_test.go` 覆盖，不重复造轮子。
  先以当前值生成期望（特征化 = 锚定旧行为），跑 `go test ./internal/modules/agentruntime/agent/tools/ -run 'Snapshot' -count=1 -v` → 全 PASS（GREEN 基线）。
- [ ] commit：`test(passb): R2.1 freeze tools contract snapshot and T0 baseline (b3-r-tools)`

**预期：** 新测试全 PASS；既有 tools 包测试无回归；evidence §1 落盘。

### Task R2.2: execution 家族 seam（sandbox×7 + browserskill×2 → execution_seams.go）

**文件：** 新建 `internal/modules/agentruntime/agent/tools/execution_seams.go`；改写 9 个生产文件去掉深 import：`output_links.go`、`sandbox_edit.go`、`sandbox_ls.go`、`sandbox_write.go`、`shell_exec.go`、`skill_file.go`、`workspace_reader.go`、`browserskill.go`、`browserskill_result.go`；数据行增删：`tools/architectureguard/check.go` importExceptions（删 9 行：from 为上述文件、to 为 execution/sandbox 或 execution/browserskill 的行 = exc-0029/0030/0039/0040/0041/0042/0044/0045/0047 对应行）+ `docs/architecture/passb/exception-ledger.yaml`（同窗同 commit 删同 9 行、按头注续号新增 2 行：`execution_seams.go→execution/sandbox`、`execution_seams.go→execution/browserskill`，plan=32-agentruntime-tools、remove_at=ib3、reason 注明"R2 消费侧 seam 收敛（32 计划 §0.4/§0.5），ib3 门面合法化后随 seam 文件消除"+ Ruling ID）；guard 同步新增 2 条对应数据行。

**步骤：**
- [ ] 按 §0.8-A 创建 seam（类型别名/常量/薄委托逐字对照源签名）。
- [ ] 9 个文件机械改写：删 sandbox/browserskill import 行，`sandbox.X`→`X`、`browserskill.X`→`X`（`gofmt -w`；不改任何函数体逻辑、不动 `_test.go`——测试文件自带 import，别名透明不受影响）。
- [ ] GREEN：`go test ./internal/modules/agentruntime/agent/tools/ -count=1` → ok；`go build ./...` → 退出码 0。
- [ ] 边核验（差分关键证据）：`grep -rn "modules/execution" internal/modules/agentruntime/agent/tools/*.go | grep -v _test | grep -v execution_seams.go` → **空输出**（§10 tools 侧 execution 边归零）；`grep -c "execution/sandbox\|execution/browserskill" internal/modules/agentruntime/agent/tools/execution_seams.go` → 2（import 行）。
- [ ] guard/ledger 数据行同窗同 commit 增删（§0.3 数据行级权限）；`make check-backend-architecture` → `OK (0 violations)`；`make check-passb-readiness` → `exceptions=135`（本任务时点计数 142−9+2；R2.3/R2.4 完成后依次为 133/130，实施者在 evidence §8 逐窗登记）。
- [ ] §8 计数登记：evidence 追加"exceptions 142→135（R2.2 窗口，−9+2，Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY + §8 三方一致）"。
- [ ] commit：`refactor(agentruntime): R2.2 converge execution deep imports behind tools seam (exc-0029/0030/0039-0047 -> 2 seam rows)`

**预期：** tools 包测试全 PASS（含 R2.1 快照——证明外部契约零变化）；guard 0 violations；grep 证据空输出。

### Task R2.3: airesource 家族 seam（mcp×2 + rerank×1 + chat×2 → airesource_seams.go）

**文件：** 新建 `airesource_seams.go`；改写 `mcp_oauth.go`、`mcp_tool.go`、`mcp_exposure.go`、`sanitize_messages.go`、`knowledge_search.go`（后者同时处理 rerank 边；其 searchutil 边留 R2.4）；数据行：删 exc-0033/0036/0037/0038/0043 对应 guard+ledger 行，新增 3 行（seam→mcp、seam→rerank、seam→chat）。

**步骤：** 同 R2.2 模式（§0.8-B seam；`mcp.X`→`X`、`rerank.X`→`X`、`chat.Message`→`Message`）。
- [ ] GREEN：`go test ./internal/modules/agentruntime/agent/tools/ -count=1` → ok（重点观察 mcp_catalog/mcp_tool/sanitize_messages 既有测试）。
- [ ] 边核验：`grep -rn "modules/airesource" internal/modules/agentruntime/agent/tools/*.go | grep -v _test | grep -v airesource_seams.go` → 空输出。
- [ ] guard/ledger 同窗增删；`make check-backend-architecture` → OK；`make check-passb-readiness` → `exceptions=133`（135−5+3）。
- [ ] §8 登记：exceptions 135→133。
- [ ] commit：`refactor(agentruntime): R2.3 converge airesource deep imports behind tools seam (exc-0033/0036/0037/0038/0043 -> 3 seam rows)`

### Task R2.4: knowledge searchutil 家族 seam（×4 → knowledge_seams.go）

**文件：** 新建 `knowledge_seams.go`；改写 `grep_chunks.go`、`knowledge_search.go`（searchutil 部分）、`list_knowledge_chunks.go`、`wiki_read_source_doc.go`；数据行：删 exc-0032/0034/0035/0046 对应行，新增 1 行（seam→knowledge/searchutil）。

**步骤：** 同 R2.2 模式（§0.8-C 全薄委托；`searchutil.X`→`X`）。
- [ ] GREEN：`go test ./internal/modules/agentruntime/agent/tools/ -count=1` → ok（重点 grep_chunks_*、knowledge_search_*、list_knowledge_chunks、wiki_* 既有测试）。
- [ ] 边核验：`grep -rn "modules/knowledge" internal/modules/agentruntime/agent/tools/*.go | grep -v _test | grep -v knowledge_seams.go` → 空输出。
- [ ] guard/ledger 同窗增删；`make check-backend-architecture` → OK；`make check-passb-readiness` → `exceptions=130`（133−4+1）。**终态：plan=32 名下 20 行 → 8 行（exc-0028、exc-0031 保留 + 6 seam 行）。**
- [ ] §8 登记：exceptions 133→130；Brief（R2.5）同步该终态。
- [ ] commit：`refactor(agentruntime): R2.4 converge knowledge searchutil imports behind tools seam (exc-0032/0034/0035/0046 -> 1 seam row)`

### Task R2.5: runScope 族导出端口裁定交接 + Integration Brief 落盘

**文件：** 新建 `docs/architecture/passb/briefs/b3-r-tools.md`（节点 Integration Brief，conventions §1.1 路径）。

**步骤：**
- [ ] Brief 必含五节：
  1. **装配变更申请（M4 面）**：本节点 **无** router/container/bootstrap 变更（纯消费侧收敛，构造签名零变化——§0.7 条 3）；声明 R5/ib3 无需为本节点做任何装配切换。
  2. **ib3 门面合法化契约任务申请**（ib2.md 前置提醒 3 接续）：逐模块 re-export 清单——execution 根：§0.8-A 全部符号（12 类型 + 4 常量 + 1 变量 + 9 函数）；airesource 根：§0.8-B 全部符号（8 类型）；knowledge 根：§0.8-C 全部函数（7 个）；落地后 ib3 翻转 3 个 seam 文件的 import 至根并删除 6 条 seam 例外行。另申请：architectureguard forbidden-import 的"模块根合法化"策略裁定（与 passbguard scanConsumerImports 根合法口径对齐），以解除 exc-0028（app_connector→appconnector 根）/exc-0031（craft_delegate→craft 根）两条已处目标形态的边。
  3. **runScope/toolCallScope/runView 导出端口裁定**（§0.6 全文引用）：执行属主 33-agentruntime-engine（导出/留 shim）与 b4-craft（调用点改写），期限 ib4；本节点仅交接。
  4. **例外台账终态**：plan=32 名下 20→8 行明细（保留 exc-0028/0031；新增 6 seam 行 exc-id 续号与撞号重排规则引头注）；计数 142→130 的 §8 登记。
  5. **程序内协同提示**：chat.Message 的程序级收敛（engine 面 26 条边归 33）与 tools seam 的关系——tools seam 是 R2 局部过渡，若 R3/R5 采纳 agentruntime 根 re-export 方案（b2-ac-definition Brief #9 同型），ib3 可一并翻转；不冲突。
- [ ] commit：`docs(passb): R2.5 record runScope-family port ruling and tools Integration Brief (b3-r-tools)`

### Task R2.6: 差分证据、实施报告与节点门禁收口

**文件：** `docs/architecture/evidence/passb/b3-r-tools.md`（补差分章节）、`docs/plans/passb/reports/b3-r-tools.md`。

**步骤：**
- [ ] 差分（conventions §6 口径，工具行为等价）：
  - 用例集 = R2.1 四个快照测试（AvailableToolDefinitions 表 / DefaultAllowedTools 表 / persist 三表 / 可构造工具 schema 字节）+ 既有高信号测试（`tool_policy_coverage_test.go`、`registry_journal_test.go`、`sanitize_messages_test.go`、`mcp_tool_test.go`、`app_connector_test.go`、`craft_delegate_test.go`、`shell_exec_test.go`、`sandbox_write_test.go`、`knowledge_search_rerank_test.go`、`grep_chunks_*`）；
  - 前拍（T0，R2.1 已存）与后拍（本任务）同命令双跑：`go test ./internal/modules/agentruntime/agent/tools/ -count=1`，输出（包级 ok + 用例计数 `go test ... -v 2>&1 | grep -c "^--- PASS"`）逐项比对入 evidence 差分章节；结论必须逐用例等价。
  - 高风险面声明：本节点不触碰 recovery/stream/payment；工具契约等价即本节点差分边界（§0.7 条 6）。
- [ ] 报告包（conventions §1.2 命令逐条执行并摘录）：`PASSB_BASE_SHA=$(git merge-base origin/main HEAD)`（或协调者给定基线）→ `git diff --stat`、`go build ./...`、逐条 gates、`git diff --name-only | sort` 与 owned_files 求差集（差集须为空——注意 `.worktrees` 探针等临时物不得残留，撰写会话已清理并 `git status` 验证）。
- [ ] 节点 gates（DAG 原文 argv，不得替代）：
  - `go build ./...` → 退出码 0（仅既有 `-lc++` 警告）；
  - `go test ./internal/modules/agentruntime/... -count=1` → 全 ok（T0 基线对照，0 FAIL）；
  - `make check-backend-architecture` → `... total=633 | redis=23 lite=23 | hooks=58 | modules=16` + `OK (0 violations)`；
  - `make verify-module-moves` → `modulemove: OK (16 manifests verified)`；
  - 附加：`make check-passb-readiness` → `legacy=358 aliases=69 exceptions=130 contracts=125 events=29 overlaps=0 missing=0`。
- [ ] 未完成项/勘误如实列出（Ruling 环境、推迟件口径参照 §5 升级契约，禁止省略）。
- [ ] commit：`docs(passb): R2.6 tools differential evidence, report and node gates (b3-r-tools)`

## 2. 集成与回滚边界

- **无 M4 装配切换**：本节点不写 router/container/bootstrap，ib3 对本节点无装配动作（Brief 第 1 节声明）；6 条 seam 例外行的删除动作属 ib3 门面合法化契约任务（翻转 3 个 seam 文件后删行）。
- **回滚边界**：R2.2/R2.3/R2.4 各自独立成 commit 且互不依赖顺序执行也可（推荐按序）；任一任务失败回滚 = `git revert` 该任务 commit（seam 文件 + 数据行同 commit，revert 即整体回到前一收敛状态）；guard/ledger 行与代码在同一 commit 内，不存在"代码在、行不在"的中间态。R2.1/R2.5/R2.6 为纯 docs/test，回滚无生产影响。
- **禁止的回滚方式**：不得通过改 guard 判定逻辑、删测试、扩例外通配来"恢复绿色"（conventions §5）。
- **merge 语义**：节点分支合入集成分支由集成工程师执行（`merge: passb b3-r-tools`，一次一支、审后合并）。

## 3. 必须删除的 legacy/alias/例外（本节点口径）

| 类别 | 明细 | 动作 |
|---|---|---|
| legacy 文件迁入 | 无（matrix plan=32 行数 = 0，§0.2 条 2） | 无动作 |
| alias 包删除 | agentruntime 20 个旧 alias 已在先前批次物理删除（passbguard aliases=69 不含之） | 无动作，报告复核一句带过 |
| guard importExceptions 数据行 | from=tools/ 的 18 行（exc-0029/0030/0032/0033/0034/0035/0036/0037/0038/0039/0040/0041/0042/0043/0044/0045/0046/0047） | R2.2–R2.4 各自同窗删除 |
| exception-ledger 行 | 同上 18 行 | 同窗删除；新增 6 条 seam 行（续号 exc-0143.. 起，撞号按头注 (from,to) 边键重排） |
| 保留例外 | exc-0028（→appconnector 根）、exc-0031（→craft 根）：已是目标形态，解除前置 = guard 根合法化（Brief 第 2 节申请） | 不删，Brief 登记 |
| seam 文件 | 3 个 seam 文件本体 | **ib3 删除**（门面合法化后），本节点只建不删 |

## 4. 节点级独立验收标准

1. §0.2 前置 1–5 核对记录在 evidence §1（命令原文+退出码）。
2. R2.1 四个快照测试存在且 GREEN；T0 四命令 + 全量 gate（`go test ./internal/modules/agentruntime/... -count=1`，基线 20 ok/0 FAIL）结果落盘。
3. R2.2–R2.4 完成后：`grep -rn "modules/\(execution\|airesource\|knowledge\)" internal/modules/agentruntime/agent/tools/*.go | grep -v _test | grep -v "_seams.go"` → **空输出**；3 个 seam 文件是 tools 生产代码中仅有的跨模块 import 持有点（另 app_connector.go 根 import、craft_delegate.go 根 import 两条保留边）。
4. exception-ledger 中 plan=32-agentruntime-tools 行数 = 8（2 保留 + 6 seam），guard 数据行与之一致（`make check-passb-readiness` 零诊断）。
5. tools 包测试全 PASS 且 R2.1 快照测试字节级不变（外部契约零变化的机器证明）。
6. 节点四条 gates 全绿 + 附加 readiness 计数 `exceptions=130`；`git diff --name-only` 与 owned_files 差集为空。
7. Brief `docs/architecture/passb/briefs/b3-r-tools.md` 五节齐备（§1 R2.5 清单），含 runScope 族裁定全文。
8. evidence 含差分章节（前拍/后拍双跑输出 + 逐用例等价结论）；报告含 §1.2 全部命令摘录与未完成项如实清单。
9. 计数基线：路由 633 / Redis 23 / Lite 23 / hooks 58 逐 gate 不变；exceptions 142→130 已按 §8 登记（三方一致：guard 实测 = ledger 行数 = passbguard 输出）。

## 5. 计划自检记录（撰写会话）

- **Spec 覆盖：** brief 三条义务逐一落任务——义务 1（耦合收敛）→ R2.2/R2.3/R2.4 + Brief 第 2 节；义务 2（外部契约不变）→ §0.7 + R2.1 快照 + R2.6 差分；义务 3（验证与归零口径）→ R2.6 gates + §4 验收 3/4（"tools 侧 forbidden-import 归零"的机器口径 = §4 条 3 的 grep 空输出 + readiness 零诊断）。Spec §4.4 过渡别名、§15 例外精确路径、§14.3 差分均覆盖。
- **无占位符：** 全部文件路径、行号、签名、命令、期望输出为本会话实读/实跑所得（行号锚定基线 `a2fbcf55e`）：mcp `ContentItem`/`CallToolResult` 定义（mcp/types.go:51/:45）、`BaseTool.Parameters()`（tool.go:37）、tools 包无 `Scope/Manager/Status/Message/Reranker` 同名顶层标识符（grep 实证）均已在撰写中逐字核验；R2.1 的"以实测为准补入 seam"仅为实施期防漏兜底（基线漂移时的勘误通道），不是未决项。
- **类型一致：** seam 全部为别名/薄委托（类型同一性保持），构造签名形态不变（§0.7 条 3）；跨任务接口一致（R2.1 快照 = R2.2–R2.4 的等价判据；R2.2–R2.4 产出的 seam 行 = R2.5 Brief 第 4 节数据）。
- **现实基线诚实性：** IB1 门面缺位（B1 零实施）与 guard 根 import 报诊断均以本会话实证写入 §0.2/§0.4，收敛路径据此裁定为消费侧 seam + barrier 交接，未发明上游门面（conventions §5"不自行实现上游门面"）。
- **任务归属：** 本计划全部任务属 b3-r-tools；§0.6 runScope 族裁定的**执行**属主（33-agentruntime-engine 导出/b4-craft 改写）已在节内标注，不产生本节点 tasks 之外的任务节。
- **已知风险：** (1) seam 导出名与包内现有标识符冲突 → 以编译器为准加前缀（§0.8-A 注）；(2) tools 包外溢效应零（别名透明），但 R2.2–R2.4 后需跑全 agentruntime 树 gate 而非仅 tools 包（R2.6 已排）；(3) 撞号重排规则引 ledger 头注，集成侧处理。
