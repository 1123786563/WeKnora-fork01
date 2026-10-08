# Evidence — b3-r-tools（R2 AgentRuntime 工具边界收敛）

> 节点分支 `codex/passb-b3-r-tools`；任务级基线 BASE=`e2f7e86653db83c4bbf2f30e516c85b71028a647`（本 evidence 由 R2.1 创建）。计划：`docs/plans/passb/32-agentruntime-tools.md`。
> 本章 §1 由 Task R2.1 落盘（前置核验 + 特征化基线冻结）；差分章节由 R2.6 补写。

## §1 前置核验与 T0 特征化基线（R2.1，2026-09-28）

### 1.1 §0.2 前置 1–4 核对

| # | 前置项 | 命令 | 结果 | 退出码 |
|---|---|---|---|---|
| 1 | ib2 done | `python3 -c "import json;d=json.load(open('docs/plans/passb/execution-dag.json'));print([(n['id'],n['status'],n.get('head_sha','')) for n in d['nodes'] if n['id'] in ('ib2','b3-r-tools')])"`（在 `.worktrees/passb-int`） | `[('ib2','done','a2fbcf55eb7ba0dc73f9deecba00c5f2f45e634f'), ('b3-r-tools','in_progress',None)]` | 0 |
| 2 | R0 复核（matrix plan=32 行数 = 0） | python3 逐行解析 `docs/architecture/passb/ownership-matrix.yaml` 统计 plan 含 `32-agentruntime-tools` 的行 | `plan=32 rows: 0 []` —— R2 无遗留文件迁入，与计划 §0.2 条 2 推论一致 | 0 |
| 3 | 基线对齐 | `git merge-base --is-ancestor a2fbcf55e HEAD && echo PRECOND3_OK`（worktree） | `PRECOND3_OK`（ib2 收口提交 a2fbcf55e 是 HEAD 祖先；worktree HEAD=e2f7e866 = 派发 BASE） | 0 |
| 4a | 门面现实核验（execution/airesource 骨架） | `head -30 internal/execution/module.go` / `head -30 internal/airesource/module.go` | 两文件均为零逻辑注释骨架 + `package execution` / `package airesource` 声明，无任何 re-export | 0 |
| 4b | 门面现实核验（knowledge 无 searchutil re-export） | `grep -c "searchutil" internal/knowledge/module.go` | 0（无 searchutil 符号 re-export） | 0（grep 无匹配按 `|| echo` 处理，实测输出 `no searchutil re-export`） |

### 1.2 T0 基线复跑（§0.2 条 5 四命令）

| 命令 | 关键输出 | 退出码 |
|---|---|---|
| `go test ./internal/agentruntime/agent/tools/ -count=1` | `ok github.com/Tencent/WeKnora/internal/agentruntime/agent/tools 18.249s`（仅既有 `-lc++` 链接警告）；复核轮 `11.056s` `tools_test_EXIT=0` | 0 |
| `make check-backend-architecture` | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`；`architectureguard: OK (0 violations)` | 0 |
| `make verify-module-moves` | `modulemove: OK (16 manifests verified)` | 0 |
| `make check-passb-readiness` | `pass-b readiness: legacy=358 aliases=69 exceptions=142 contracts=125 events=29 overlaps=0 missing=0` | 0 |

与计划 §0.2 条 5 撰写时实测完全一致（计数 633/23+23/58/16、legacy=358、aliases=69、exceptions=142、contracts=125、events=29 无漂移）。

### 1.3 节点 gate 全量（T0 前拍）

- 命令：`go test ./internal/agentruntime/... -count=1`
- **轮 1**（干净树，写入快照测试之前，后台运行，任务系统记录 exit code 0）：`20 ok / 0 FAIL / 1 no test files`（`agentruntime` 根 `[no test files]`；ok 包含 tools 20.638s、recoverytest 77.590s、opencode 35.832s 等；仅既有 `-lc++` 链接警告）。
- **轮 2**（写入快照测试后，与前台命令并行跑，EXIT=1）：`agentruntime/agent/opencode` `panic: test timed out after 10m0s`（602.765s，对照轮 1 的 35.832s）+ `agentruntime/agent/recoverytest` `TestCrashMatrixSQLite/unknown_result_user_retry`（子进程 "wait after retry: run did not complete"）；**tools 包本身 `ok 19.668s`**。
- **轮 2 失败包串行复跑**：`go test ./internal/agentruntime/agent/opencode/ ./internal/agentruntime/agent/recoverytest/ -count=1` → `ok opencode 82.995s`、`ok recoverytest 140.355s`，EXIT=0。
- **轮 3（串行）**：`go test -p 1 ./internal/agentruntime/... -count=1` → 仍 EXIT=1：`opencode` `panic: test timed out after 10m0s`（601.426s）+ `recoverytest` `TestCrashMatrixSQLite/oauth_park`（barrier 超时，128.91s，子进程 exited 且 parked=true）；`tools` `ok 88.105s`。18 ok / 2 FAIL。
- **根因裁定（环境负载，非本节点改动）**：轮 3 结束时机器 `uptime` = `load averages: 40.67 88.18 95.42`（1/5/15 分钟），同机有其他会话的重型进程（≥3 个 `opencode serve`、`open-code-review --background`、13 个 go 编译/测试进程）。证据链：① 唯一被改的 tools 包在轮 2/3 均 ok，且轮 2 前后 `go test ./internal/agentruntime/agent/tools/ -count=1` 单独跑 EXIT=0；② 失败两包**单独跑 EXIT=0 全 ok**；③ 失败特征均为 timeout panic / 子进程 barrier 时序超时（非断言失败、非编译失败——若为快照测试改动所致只能是编译或断言失败）；④ 轮 1 为本节点动手前同一棵树全绿。结论：opencode/recoverytest 属负载敏感 flaky，与 R2.1 新增文件零因果。
- **T0 前拍采用值 = 轮 1**（干净树、本节点改动为零时点）：`20 ok / 0 FAIL / 1 no test files`，exit 0。R2.6 收口时在安静窗口复跑 DAG 原文 argv 全量 gate；若届时负载仍高导致同型 flaky，按本节证据链口径处理并如实上报。
- **口径说明**：DAG gate argv 为 `go test ./internal/agentruntime/... -count=1`（无 `-p 1`）；轮 3 加 `-p 1` 是试图消除包间资源争用，仍受同机外部负载影响，如实记录（conventions §2「替代运行必须如实标注实际命令」）。

### 1.4 例外边盘点（20 行）与 §0.8 符号级比对

Ledger（`.worktrees/passb-int/docs/architecture/passb/exception-ledger.yaml`）`plan: 32-agentruntime-tools` 共 **20 行**（exc-0028..exc-0047），与 `tools/architectureguard/check.go` importExceptions 中 `ImporterFile: internal/agentruntime/agent/tools/...` 的 20 条数据行一一对应（19 文件，knowledge_search.go 占 2 行：rerank + searchutil）。

**边 → 文件映射（与计划 §1 R2.2–R2.4 任务核对）：**

| 家族 | exc-id | from 文件 | to 包 |
|---|---|---|---|
| execution/sandbox | exc-0039/0040/0041/0042/0044/0045/0047 | output_links.go、sandbox_edit.go、sandbox_ls.go、sandbox_write.go、shell_exec.go、skill_file.go、workspace_reader.go（7 文件） | execution/sandbox |
| execution/browserskill | exc-0029/0030 | browserskill.go、browserskill_result.go | execution/browserskill |
| airesource/mcp | exc-0037/0038 | mcp_oauth.go、mcp_tool.go | airesource/mcp |
| airesource/models/rerank | exc-0033 | knowledge_search.go | airesource/models/rerank |
| airesource/models/chat | exc-0036/0043 | mcp_exposure.go、sanitize_messages.go | airesource/models/chat |
| knowledge/searchutil | exc-0032/0034/0035/0046 | grep_chunks.go、knowledge_search.go、list_knowledge_chunks.go、wiki_read_source_doc.go | knowledge/searchutil |
| 保留（模块根门面形态） | exc-0028 | app_connector.go | appconnector（根） |
| 保留（模块根门面形态） | exc-0031 | craft_delegate.go | craft（根） |

**符号级消费实测（`grep -o "<pkg>\.[A-Z][A-Za-z0-9]*"` 逐文件）：**

- `sandbox.X`（7 文件并集）：`SessionBoundManager`、`ExecuteResult`、`ShellExecOptions`、`ShellOutputSnapshot`、`RemoteDirEntry`、`RemoteStatEntry`、`SessionInstallShellExecutor`、`ErrTimeout`、`SessionWorkspaceRoot`、`SessionInputRoot`、`SessionOutputRoot`、`SkillsImageRoot`、`ResolveWorkspacePath`、`ShellQuote`、`WithCommandOutput`、`WithSessionFileOperation`、`IsValidSkillName`、`SkillDirFor`、`ValidatedImageSkillDir`、`SkillNameFromImagePath` —— 前两项计划 §0.8-A 已列 ✔；**另实测出计划未列的 3 个符号：`RemoteEntryFile`（output_links/sandbox_write/workspace_reader）、`RemoteEntryDir`（sandbox_edit/skill_file/workspace_reader）及其类型 `RemoteDirEntryType`**（定义 `execution/sandbox/remote_client.go:379` `type RemoteDirEntryType string`；:382-389 常量块 `RemoteEntryFile/RemoteEntryDir/RemoteEntryOther`，tools 未用 Other）→ **勘误登记，R2.2 seam 补入**（`type RemoteDirEntryType = sandbox.RemoteDirEntryType` + 两个常量别名）。
- `browserskill.X`：`Manager`、`Scope`、`Status`、`AccountStatus`、`RPCError`、`NavigationIncomplete` —— 与 §0.8-A 全一致 ✔。
- `mcp.X`：`MCPClient`、`MCPManager`、`OAuthReauthorizationRequiredError`、`CallToolResult`、`ContentItem` —— 与 §0.8-B 全一致 ✔。
- `chat.X`：`Message`（mcp_exposure×1、sanitize_messages×4）—— 一致 ✔。
- `rerank.X`：`Reranker`、`RankResult` —— 一致 ✔。
- `searchutil.X`（4 文件并集）：`BuildContentSignature`、`TokenizeSimple`、`Jaccard`、`ClampFloat`、`CollectImageInfoByChunkIDs`、`EnrichSearchResultsImageInfo`、`BuildImageInfoMarkdownWithURL` —— 与 §0.8-C 全部 7 函数一致 ✔。
- 保留边符号（Brief 参考数据）：`appconn.OCSubject` + `appconn.Action{Unknown,Queued,Dispatched,AwaitingApproval,Authorized,Failed,Succeeded}`；`craft.{ErrUnknown,Scope,Input,InputPath,Task,Result}` —— 与 §0.8-D 一致 ✔。
- seam 函数签名涉及的 `interfaces`/`types` 前缀实测为平台层 `internal/types/interfaces` 与 `internal/types`（knowledge_search.go:16-17 等 import 块），非模块深 import，knowledge_seams.go 签名可直接引用，无需额外边。

**结论：** 计划 §0.8 清单与实测的唯一差异 = execution 家族遗漏 `RemoteDirEntryType` 类型 + `RemoteEntryFile`/`RemoteEntryDir` 两常量（共 3 个符号）。按 R2.1 步骤 3「以实测为准补入 seam」处理，登记于本节；不改任何判定逻辑。

### 1.5 特征化快照基线（R2.1 交付物）

新建 `internal/agentruntime/agent/tools/contract_snapshot_test.go`，四个表驱动快照测试（期望值由基线实现实测输出生成，特征化 = 锚定旧行为）：

| 测试 | 锚定对象 | 断言口径 |
|---|---|---|
| `TestAvailableToolDefinitionsContractSnapshot` | `AvailableToolDefinitions()`（definitions.go:85） | 21 项 `{Name,Label,Description}` 三元组逐字段相等 |
| `TestDefaultAllowedToolsSnapshot` | `DefaultAllowedTools()`（definitions.go:116） | 精确切片 `["knowledge_search","grep_chunks","list_knowledge_chunks","get_document_info","search_conversations"]` |
| `TestPersistStripTablesSnapshot` | `persistStripFields`/`persistStripFieldsByTool`/`clientStripFieldsByTool`（persist.go:11-35） | 三表键集与值切片 DeepEqual（前表 2 键、后两表各 5 键） |
| `TestConstructibleToolSchemaBytesSnapshot` | 8 个可零依赖构造工具的 `BaseTool.Parameters()`（tool.go:37） | JSON 字节 SHA-256：app_connector `dea92991…`、list_sandbox_files `0f855075…`、write_sandbox_file `443aa6f3…`、edit_sandbox_file `5d7f59f0…`、shell_exec `70ff73d6…`、write_skill_file `5300f631…`、edit_skill_file `82f09f98…`、craft_delegate `d6fc02b7…`（craft_delegate 以桩 Delegate 构造，断言不触发委托调用） |

运行记录：

- `go test ./internal/agentruntime/agent/tools/ -run 'Snapshot' -count=1 -v` → 4 个新快照测试全 `--- PASS`（同 pattern 另匹配既有 MCP/webfetch Snapshot 测试 6 个，全 PASS）；包级 `ok … 4.565s`。
- 期望值生成方式：临时 dump 测试（`zz_dump_snapshot_test.go`，跑完即删，未入库）打印基线输出，回填为断言后复跑——特征化测试无人工 RED 阶段，GREEN 基线即交付（计划 R2.1 步骤 4 原文口径）。
- 既有测试回归：`go test ./internal/agentruntime/agent/tools/ -count=1` → `ok … 11.056s`（EXIT=0，无 FAIL）。
- `gofmt -l`（tools 目录）空输出；`go vet ./internal/agentruntime/agent/tools/` 退出码 0。

### 1.6 R2.1 结论

前置 1–4 全部满足；T0 四命令与节点 gate 与计划基线零漂移；20 条例外边盘点完成，唯一勘误（3 个 sandbox 符号遗漏）已登记并转 R2.2 补入；契约快照 GREEN。R2.2–R2.4 可开工。

## §8 计数基线登记（§8 三方一致，逐窗口）

### 8.1 R2.2 窗口（2026-09-28）：exceptions 142→135（−9+2）

**变更：** Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY + conventions §8 三方一致。

- **−9**（tools 消费侧 seam 收敛，guard importExceptions 与 exception-ledger 同窗同 commit 删除）：exc-0029（browserskill.go→execution/browserskill）、exc-0030（browserskill_result.go→execution/browserskill）、exc-0039（output_links.go→execution/sandbox）、exc-0040（sandbox_edit.go）、exc-0041（sandbox_ls.go）、exc-0042（sandbox_write.go）、exc-0044（shell_exec.go）、exc-0045（skill_file.go）、exc-0047（workspace_reader.go）。
- **+2**（seam 登记行，续号 exc-0143/0144，remove_at=ib3，reason="R2 消费侧 seam 收敛（32 计划 §0.4/§0.5），ib3 门面合法化后随 seam 文件消除（Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）"）：exc-0143（execution_seams.go→execution/sandbox）、exc-0144（execution_seams.go→execution/browserskill）。ledger 头注同窗机械计数修正 142→135 并追加 R2.2 增量行。

**三方一致复核（本窗口实跑）：**

| 口径 | 命令 | 实测 |
|---|---|---|
| guard 实测 | `make check-backend-architecture` | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16` + `OK (0 violations)`（路由/任务/挂点计数不变） |
| passbguard 实测 | `make check-passb-readiness` | `pass-b readiness: legacy=358 aliases=69 exceptions=135 contracts=125 events=29 overlaps=0 missing=0` |
| ledger 行数 | `grep -c '^  - id:' docs/architecture/passb/exception-ledger.yaml` | `135`（plan=32 名下 20→13 行：−9+2） |

### 8.2 R2.2 边核验与 GREEN 证据（差分关键项，§6 差分前拍见 §1）

| 检查 | 命令 | 结果 |
|---|---|---|
| tools 生产代码 execution 边归零 | `grep -rn "modules/execution" internal/agentruntime/agent/tools/*.go \| grep -v _test \| grep -v execution_seams.go` | 空输出（EXIT=1） |
| seam 文件仅持 2 条 import 边 | `grep -c "execution/sandbox\|execution/browserskill" internal/agentruntime/agent/tools/execution_seams.go` | `2` |
| GREEN：tools 包 | `go test ./internal/agentruntime/agent/tools/ -count=1` | `ok github.com/Tencent/WeKnora/internal/agentruntime/agent/tools 48.203s`（EXIT=0；含 R2.1 四快照 = 外部契约零变化机器证明） |
| GREEN：全量构建 | `go build ./...` | EXIT=0（仅既有 `-lc++` 链接警告） |
| gofmt/vet | `gofmt -l internal/agentruntime/agent/tools/`；`go vet ./internal/agentruntime/agent/tools/` | 空输出 / EXIT=0 |

**符号勘误落点（§1.4 登记 → 本窗口执行）：** seam 实补 `RemoteEntryFile`/`RemoteEntryDir` 两常量（remote_client.go:382/:383）；`RemoteDirEntryType` 类型经 `RemoteDirEntry.Type` 字段透明传递，tools 包（生产+测试）零直接书写（grep 实证），无需独立别名。seam 终集 = 12 类型别名 + 6 常量别名 + 1 变量别名 + 9 薄委托函数（计划 §0.8-A 的 4 常量 + 勘误 2 常量）。

### 8.3 R2.3 窗口（2026-09-28）：exceptions 135→133（−5+3）

**变更：** Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY + conventions §8 三方一致。

- **−5**（tools 消费侧 seam 收敛，guard importExceptions 与 exception-ledger 同窗同 commit 删除）：exc-0033（knowledge_search.go→airesource/models/rerank）、exc-0036（mcp_exposure.go→airesource/models/chat）、exc-0037（mcp_oauth.go→airesource/mcp）、exc-0038（mcp_tool.go→airesource/mcp）、exc-0043（sanitize_messages.go→airesource/models/chat）。
- **+3**（seam 登记行，续号 exc-0145/0146/0147，remove_at=ib3，reason="R2 消费侧 seam 收敛（32 计划 §0.4/§0.5），ib3 门面合法化后随 seam 文件消除（Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）"）：exc-0145（airesource_seams.go→airesource/mcp）、exc-0146（airesource_seams.go→airesource/models/rerank）、exc-0147（airesource_seams.go→airesource/models/chat）。ledger 头注同窗机械计数修正 135→133 并追加 R2.3 增量行。

**三方一致复核（本窗口实跑）：**

| 口径 | 命令 | 实测 |
|---|---|---|
| guard 实测 | `make check-backend-architecture` | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16` + `OK (0 violations)`（路由/任务/挂点计数不变） |
| passbguard 实测 | `make check-passb-readiness` | `pass-b readiness: legacy=358 aliases=69 exceptions=133 contracts=125 events=29 overlaps=0 missing=0` |
| ledger 行数 | `grep -c '^  - id:' docs/architecture/passb/exception-ledger.yaml` | `133`（plan=32 名下 13→11 行：−5+3） |

### 8.4 R2.3 边核验与 GREEN 证据（§6 差分前拍见 §1，后拍归 R2.6）

| 检查 | 命令 | 结果 |
|---|---|---|
| tools 生产代码 airesource 边归零 | `grep -rn "modules/airesource" internal/agentruntime/agent/tools/*.go \| grep -v _test \| grep -v airesource_seams.go` | 空输出（EXIT=1） |
| seam 文件仅持 3 条 import 边 | `grep -c "airesource/mcp\|airesource/models/rerank\|airesource/models/chat" internal/agentruntime/agent/tools/airesource_seams.go` | `3` |
| GREEN：tools 包 | `go test ./internal/agentruntime/agent/tools/ -count=1` | `ok github.com/Tencent/WeKnora/internal/agentruntime/agent/tools 16.387s`（EXIT=0；含 R2.1 四快照 = 外部契约零变化机器证明；重点观察 mcp_catalog/mcp_tool/sanitize_messages 既有测试无 FAIL） |
| GREEN：全量构建 | `go build ./...` | EXIT=0（仅既有 `-lc++` 链接警告） |
| 模块迁移校验 | `make verify-module-moves` | `modulemove: OK (16 manifests verified)` |
| gofmt/vet | `gofmt -l internal/agentruntime/agent/tools/`；`go vet ./internal/agentruntime/agent/tools/` | 空输出 / EXIT=0 |

**seam 终集（§0.8-B 全量，无勘误）：** 8 类型别名 = mcp 5（MCPManager/MCPClient/ContentItem/CallToolResult/OAuthReauthorizationRequiredError）+ rerank 2（Reranker/RankResult）+ chat 1（Message）。符号消费实证（`grep -o` 逐文件）：mcp_oauth.go 用 MCPManager/MCPClient/OAuthReauthorizationRequiredError；mcp_tool.go 用 MCPManager/CallToolResult/ContentItem；mcp_exposure.go+sanitize_messages.go 用 chat.Message；knowledge_search.go 用 Reranker/RankResult——8 符号与 §0.8-B 清单零差异（无 R2.1 式勘误补入）。knowledge_search.go 的 searchutil 边（exc-0034）按计划留 R2.4。

### 8.5 R2.4 窗口（2026-09-29）：exceptions 133→130（−4+1）

**变更：** Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY + conventions §8 三方一致。

- **−4**（tools 消费侧 seam 收敛，guard importExceptions 与 exception-ledger 同窗同 commit 删除）：exc-0032（grep_chunks.go→knowledge/searchutil）、exc-0034（knowledge_search.go→knowledge/searchutil）、exc-0035（list_knowledge_chunks.go→knowledge/searchutil）、exc-0046（wiki_read_source_doc.go→knowledge/searchutil）。
- **+1**（seam 登记行，续号 exc-0148，remove_at=ib3，reason="R2 消费侧 seam 收敛（32 计划 §0.4/§0.5），ib3 门面合法化后随 seam 文件消除（Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）"）：exc-0148（knowledge_seams.go→knowledge/searchutil）。ledger 头注同窗机械计数修正 133→130 并追加 R2.4 增量行。

**三方一致复核（本窗口实跑）：**

| 口径 | 命令 | 实测 |
|---|---|---|
| guard 实测 | `make check-backend-architecture` | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16` + `OK (0 violations)`（路由/任务/挂点计数不变） |
| passbguard 实测 | `make check-passb-readiness` | `pass-b readiness: legacy=358 aliases=69 exceptions=130 contracts=125 events=29 overlaps=0 missing=0` |
| ledger 行数 | `grep -c '^  - id:' docs/architecture/passb/exception-ledger.yaml` | `130`（plan=32 名下 11→8 行：−4+1，**终态** = exc-0028/0031 保留 + exc-0143..0148 六 seam 行） |

### 8.6 R2.4 边核验与 GREEN 证据（§6 差分前拍见 §1，后拍归 R2.6）

| 检查 | 命令 | 结果 |
|---|---|---|
| tools 生产代码 knowledge 边归零 | `grep -rn "modules/knowledge" internal/agentruntime/agent/tools/*.go \| grep -v _test \| grep -v knowledge_seams.go` | 空输出（EXIT=1） |
| seam 文件仅持 1 条 import 边 | `grep -c "knowledge/searchutil" internal/agentruntime/agent/tools/knowledge_seams.go` | `1` |
| 残留 searchutil. 引用（seam 外） | `grep -rn "searchutil\." internal/agentruntime/agent/tools/*.go \| grep -v _test \| grep -v knowledge_seams.go` | 空输出（EXIT=1） |
| GREEN：tools 包 | `go test ./internal/agentruntime/agent/tools/ -count=1` | `ok github.com/Tencent/WeKnora/internal/agentruntime/agent/tools 55.758s`（EXIT=0；含 R2.1 四快照 = 外部契约零变化机器证明） |
| GREEN：全量构建 | `go build ./...` | EXIT=0（仅既有 `-lc++` 链接警告） |
| 模块迁移校验 | `make verify-module-moves` | `modulemove: OK (16 manifests verified)` |
| gofmt/vet | `gofmt -l internal/agentruntime/agent/tools/`；`go vet ./internal/agentruntime/agent/tools/` | 空输出 / EXIT=0 |

**首跑负载抖动记录（如实登记）：** 本窗口第一次全量 `go test ./internal/agentruntime/agent/tools/ -count=1`（与并发 go build 同机竞争时）FAIL（196.473s）：`TestToolJournalMCPApprovalOutlivesToolTimeout`（registry_journal_test.go:216 "sql: transaction has already been committed or rolled back"，0.75s）与 `TestSkillPythonPackageRecoveryInstallsWithoutPip/ensurepip`（skill_runtime_guard_test.go:69 "signal: killed"，150.38s，真实 python 子进程撞 1 分钟 CommandContext 死线）。归因复核（2×2）：同两用例在**改动树**单跑 `go test ... -run 'TestToolJournalMCPApprovalOutlivesToolTimeout|TestSkillPythonPackageRecoveryInstallsWithoutPip' -count=1` → `ok 54.206s`；在 **BASE 临时 worktree**（8777e026c，已清理）同命令 → `ok 22.173s`；随后串行重跑全量 → `ok 55.758s`。两用例均与 searchutil 7 符号零代码路径交集（MCP journal 事务 + skill venv 子进程），判定为负载敏感环境抖动而非本任务回归。

**seam 终集（§0.8-C 全量，无勘误）：** 7 个薄委托函数 = textutil 4（BuildContentSignature/TokenizeSimple/Jaccard/ClampFloat）+ imageinfo 3（CollectImageInfoByChunkIDs/EnrichSearchResultsImageInfo/BuildImageInfoMarkdownWithURL）。符号消费实证（`grep -n "searchutil\."` 逐文件）：grep_chunks.go 用 3（BuildContentSignature/TokenizeSimple/Jaccard）、knowledge_search.go 用 6（EnrichSearchResultsImageInfo/BuildContentSignature/BuildImageInfoMarkdownWithURL/ClampFloat/TokenizeSimple/Jaccard）、list_knowledge_chunks.go 用 2（CollectImageInfoByChunkIDs/BuildImageInfoMarkdownWithURL）、wiki_read_source_doc.go 用 2（CollectImageInfoByChunkIDs/BuildImageInfoMarkdownWithURL）——7 符号与 §0.8-C 清单零差异（无 R2.1/R2.2 式勘误补入）。签名中 `internal/types` 与 `internal/types/interfaces` 为宿主共享包（tools 四文件既有 import，非模块跨边），seam 引入不产生新模块深 import 边。**R2.2–R2.4 终态达成：§4 验收 3 的 grep 口径** `grep -rn "modules/\(execution\|airesource\|knowledge\)" internal/agentruntime/agent/tools/*.go | grep -v _test | grep -v "_seams.go"` 空输出（3 seam 文件 + app_connector.go/craft_delegate.go 两条保留根边为仅存跨模块 import 持有点）。
