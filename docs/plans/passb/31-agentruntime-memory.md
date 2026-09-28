# Pass B3 — 31-agentruntime-memory（R1 memory/modelcontext 边界归位）

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development（或 executing-plans）逐任务执行；步骤用 checkbox（`- [ ]`）跟踪。
> 本计划文件只承载 **b3-r-memory（R1）** 节点职责范围内的任务（R1.1–R1.5）。相邻职责不在本计划派发：例外台账 exc-0049..0051 三行的删除属主是 **b3-r-engine（33）**；agentruntime→airesource 收敛契约任务与 contracts.yaml 回写属 **IB3（39）**；`internal/modules/agentruntime/module.go` 门面接线与 alias_obligations 收口属 **b3-r-integration（R5，30 计划）**；`internal/application/repository/tenant_member.go` 的 forUpdateClause 导出属 **identity（B1-ID 推迟件，10-identity.md:458）**。

**Goal:** 把 ownership-matrix 归属 plan=31 的 5 个 legacy 文件（repository memory×4 + handler memory×1）物理迁入 `internal/modules/agentruntime/memory`，宿主消费方经 compat 别名零改动续编译；worker（TypeMemoryExtract 双栈）与路由（RegisterMemoryRoutes）注册面零变化；memory/modelcontext 面的 airesource 耦合按冻结台账口径处置（本节点零新增/零删除例外，收敛登记移交 IB3 契约任务）；交付特征化差分证据与 Integration Brief。

**节点：** `b3-r-memory`（phase B3，role work，execution_mode parallel，与 b3-r-tools 并行，framework:165）。DAG 事实源：`.worktrees/passb-int/docs/plans/passb/execution-dag.json`（本会话实测读取）。

**分支/worktree：** `codex/passb-b3-r-memory` @ `.worktrees/passb-b3-r-memory`；基线 `PASSB_BASE_SHA=a2fbcf55e`（= ib2 收口 HEAD，2026-09-28 10:34 派发时集成分支头；本会话实测 worktree HEAD 与 passb-int HEAD 同为 a2fbcf55e）。

## 1. Spec 与事实源指针（全部只读输入）

| 事实源 | 关键行 | 用途 |
|---|---|---|
| Spec `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` | §5.5（Agent Runtime 拥有 Memory/Model Context）、§11（B3 并行图）、§13（M1–M5 提交隔离与回滚）、§14.1–14.4（T0–T4 梯度/纯移动/高风险差分/屏障验证）、§15（例外精确路径+删除批次、横向目录禁新增生产文件须登记）、§16（停止条件） | 行为等价与提交纪律的最高准绳 |
| 框架 `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` | :25–:33（manifest 所有权/禁改/随迁测试）、:38（禁止挪 common/通配例外/复制实现）、:40（高风险面）、:165（R1‖R2） | 节点边界 |
| 冻结计划 `docs/plans/passb/00-contract-and-ownership-freeze.md` | :193–:196（engine/protocol 所有权裁定） | R0 前置复核 |
| Brief `docs/architecture/passb/agentruntime-memory.md` | 全文（义务 1–5） | 本节点范围定义 |
| manifest `docs/architecture/moves/agentruntime.yaml` | legacy_files :123/:127/:131/:135（repository memory×4）、:251（handler/memory.go）；alias_obligations :82–:85 | 行级删除义务 |
| 冻结产物 `.worktrees/passb-int/docs/architecture/passb/ownership-matrix.yaml` | :266–:289（4 行 plan=31，destination=internal/modules/agentruntime/memory）、:1652–:1656（handler 行） | 写入所有权 |
| 冻结产物 `.worktrees/passb-int/docs/architecture/passb/contracts.yaml` | :218–:243（agentruntime.memory-service，frozen）、:266–:280（agentruntime.routes：RegisterMemoryRoutes — routes_memory.go:17）、:293–:304（agentruntime.workers：TypeMemoryExtract）、:348–:408（airesource.model-service，consumers 含 memory/service.go、characterization 含 memory/stubs_test.go） | 契约不可变面 |
| 冻结产物 `.worktrees/passb-int/docs/architecture/passb/exception-ledger.yaml` | exc-0052..0056（:328/:334/:340/:346/:352，plan=31，modelcontext→airesource/models/chat，remove_at=ib3）；exc-0049..0051（:310/:316/:322，plan=33，memory/{consolidate,extract,topic_resolve}.go→chat） | 例外属主与期限 |
| 冻结产物 `.worktrees/passb-int/docs/architecture/passb/event-catalog.yaml` | 无 memory 条目（本会话 grep 实测） | 本节点无事件面 |
| 公约 `.superpowers/sdd/passb/conventions.md`（主 checkout git-ignored 区） | §1（只改 owned_files/TDD/随迁测试）、§2（门禁）、§3（禁改清单）、§4（一任务一 commit）、§5（升级契约）、§6（差分）、§7（package-private 耦合）、§8（计数 633/23+23/58/537）、§10 Ruling 族（LEGACY-ROW-OWNERSHIP / IMPORT-EXCEPTION-REGISTRY / TRANSITION-SHIM-ROW-REGISTRATION / DEFERRED-FILE-SPLIT） | 执行纪律 |
| 先例计划 `docs/plans/passb/22-knowledge-retrieval.md`（已执行 done） | §3.2 落位、§5.1–§5.6 耦合机制（compat 别名/同形 seam）、§7 测试与差分 | 机制模板（本文逐处引用） |
| 先例计划 `docs/plans/passb/10-identity.md`（已评审） | :73（tenant_member.go 推迟，forUpdateClause 一并推迟导出）、:458（"forUpdateClause 随 tenant_member.go 迁移时导出并给 memory_extraction.go（31-agentruntime）留 shim"） | forUpdateClause seam 的冻结口径 |

## 2. 前置条件（开工自核，全部命令在 worktree 根执行）

- [ ] **P1 基线对齐**：`git -C .worktrees/passb-b3-r-memory rev-parse HEAD` 输出 `a2fbcf55e…`（= ib2 head；depends_on 仅 ib2，无波内兄弟依赖，无需 WAVE-DEP-BASELINE merge）。
- [ ] **P2 R0 所有权复核**（DAG required_contracts，freeze:193–196）：`grep -n "native_memory.go\|native_archive" .worktrees/passb-int/docs/architecture/passb/ownership-matrix.yaml` 确认 `internal/application/repository/native_memory.go` → plan `33-agentruntime-engine`（实测 :356）、`internal/handler/session/native_archive.go` → plan `34-agentruntime-protocol`；`internal/application/service/native_recovery.go` → 33。三行均**不在**本节点 5 行之内，本计划不得触碰。
- [ ] **P3 ib2 冻结门面在案**：`contracts.yaml` 的 airesource.model-service（:348）与 agentruntime 三契约（memory-service/routes/workers）`stability: frozen`；`internal/types/interfaces/model.go` 存在 `GetChatModel(ctx context.Context, modelId string) (chat.Chat, error)`（实测 :42）。R1 的模型获取继续走该端口，本节点不新建门面。
- [ ] **P4 别名已删核验**（brief 义务 1 的现状）：`ls internal/application/service/memory internal/modelcontext` 均不存在；`grep -rn "application/service/memory\"\|\"github.com/Tencent/WeKnora/internal/modelcontext\"" internal/ cmd/ --include="*.go"` 仅命中注释（memory/eval_test.go:27、topic_eval_test.go:29）。Pass A 集成提交 `ae37070e7` 已删别名包，container.go:49 已直引模块路径（实测）。**本节点无别名删除物**；manifest alias_obligations :82–:85 陈旧行留给 R5 收口（见 §10）。
- [ ] **P5 T0 特征化基线**（spec §14.1；K2 先例 §2）：记录基线 SHA 后执行
  ```bash
  go test -count=1 ./internal/application/repository/ ./internal/handler/ ./internal/modules/agentruntime/memory/... ./internal/modules/agentruntime/modelcontext/...
  go test -count=1 -v ./internal/modules/agentruntime/memory/... > "$SDD/b3-r-memory/R1-t0-memory-verbose.txt" 2>&1
  go test -count=1 -v ./internal/handler/ -run 'TestMemory' > "$SDD/b3-r-memory/R1-t0-handler-memory-verbose.txt" 2>&1
  go test -count=1 -v ./internal/application/repository/ -run 'TestMemory' >> "$SDD/b3-r-memory/R1-t0-repo-memory-verbose.txt" 2>&1
  ```
  预期：全部退出码 0；`WEKNORA_MEMORY_TEST_POSTGRES_DSN` 未设时 PG 用例按既有 `t.Skip("set WEKNORA_MEMORY_TEST_POSTGRES_DSN …")` 跳过（postgres_consistency_test.go:24 实测），登记为 blocked-env 非 PASS（spec §14.4）。原始输出存会话区 `$SDD = .superpowers/sdd/passb/b3-r-memory/`，摘录进 R1.4 evidence。

## 3. 范围与落位布局

### 3.1 迁入（ownership-matrix plan=31 全量 5 行，本会话逐行实测）

| # | 源文件（宿主包） | 落位（matrix destination，字面） | 随迁测试 |
|---|---|---|---|
| 1 | `internal/application/repository/memory.go`（987 行） | `internal/modules/agentruntime/memory/memory.go` | 宿主无同名主题测试（实测 `ls internal/application/repository | grep -i memory` 仅 native_memory_test.go，属 33 面） |
| 2 | `internal/application/repository/memory_extraction.go`（230 行） | 同上 `memory_extraction.go` | 无 |
| 3 | `internal/application/repository/memory_lifecycle.go`（125 行） | 同上 `memory_lifecycle.go` | 无 |
| 4 | `internal/application/repository/memory_vector.go`（259 行） | 同上 `memory_vector.go` | 无 |
| 5 | `internal/handler/memory.go`（465 行） | `internal/modules/agentruntime/memory/memory.go` → **改名 `memory_handler.go`**（同目录落位第 1 项已占 `memory.go` 基名，仓库先例：K2 §3.2 同名文件不可同目录） | `internal/handler/memory_consistency_test.go` → `memory_handler_test.go`（package handler → package memory；被测对象仅 MemoryHandler，符合 conventions §1.5 判定规则） |

**落位不做子包**（区别于 K2 的 repository/app/handler 三分）：matrix destination 字面即 `internal/modules/agentruntime/memory` 包根；本会话实测包级声明零冲突（incoming `memoryRepository`/`NewMemoryRepository`/`notExpired`/`saveExtractionState`/`extractionRows`/`enqueueExtractionSession`/`importLegacySessions`/`hasPendingExtraction`/`validExtractionLease`/`fallbackVectorScanCap`/`vectorColumnReady`/`writeVectorColumn`/`vectorHitRow`/`sortVectorHitsDesc`/`MemoryHandler`/`NewMemoryHandler`/`memoryListPaging`/`memoryExport{PageSize,MaxItems}` 与既有 9 个非测试文件的声明无一名重叠；engine 面（33）的 repository 落位是独立的 `internal/modules/agentruntime/repository`，不与本包合并）。

### 3.2 不迁（留在原地，本节点零改动）

- `internal/application/repository/native_memory.go` + `native_memory_test.go`（33 面，matrix :356）。
- `internal/application/repository/tenant_member.go`（identity 属主，B1-ID 推迟件）。
- `internal/modules/agentruntime/memory`（9 非测试 + 23 测试 .go + evalset.json/topic_evalset.json）与 `internal/modules/agentruntime/modelcontext`（11 非测试 + 6 测试 .go）既有文件：**除 §5.2 列明的 4 个测试文件机械改写与 2 处注释路径修正外零改动**。

## 4. 写入所有权与禁改清单

**可写**（= DAG owned_files 展开 + Ruling 授权）：
1. §3.1 的 5 个物理迁移文件 + 1 个随迁测试（git mv）；
2. 落位包内新 seam 文件 `internal/modules/agentruntime/memory/for_update_seam.go`（§5.1）；
3. §3.2 列明的 4 个模块测试文件机械改写 + 2 处注释路径修正；
4. 宿主兼容文件 2 个：`internal/application/repository/agentruntime_memory_passb_compat.go`、`internal/handler/agentruntime_memory_passb_compat.go`（§5.3；横向新生产文件，同 commit 登记 manifest+matrix 行，Ruling TRANSITION-SHIM-ROW-REGISTRATION）;
5. `docs/architecture/moves/agentruntime.yaml`（仅本节点 5 行删除 + 2 行 compat 登记，行级权限 Ruling LEGACY-ROW-OWNERSHIP）；
6. `.worktrees/passb-int` 冻结产物镜像中 ownership-matrix.yaml 的对应行删除/成对补行（**注意**：矩阵工作副本随集成分支演进，本节点在自己的 worktree 内改本 worktree 的 `docs/architecture/passb/ownership-matrix.yaml`，集成时随分支合并；K2 同法）；
7. `internal/modules/agentruntime/legacy/README.md` 镜像同步；
8. 本节点自身产出：`docs/architecture/evidence/passb/b3-r-memory.md`、`docs/plans/passb/reports/b3-r-memory.md`、`docs/architecture/passb/briefs/b3-r-memory.md`。

**禁改**（conventions §3 + K2 §4 先例）：`internal/router/router.go`、`internal/router/routes_memory.go`（及全部 `routes_*.go`/`files.go`）、`internal/router/task.go`、`internal/router/sync_task.go`、`internal/container/container.go`、`internal/bootstrap/{routes,workers,lifecycle}.go`、`go.mod`/`go.sum`、`migrations/`（含 000084_memory/000153_memory_consistency/000154_memory_vector_search 三对迁移脚本）、生产 SQL、`docs/architecture/passb/{ownership-matrix,contracts,event-catalog}.yaml` 的**非本节点行**、`tools/architectureguard/check.go`、`docs/architecture/passb/exception-ledger.yaml`（本节点零行增删，§5.2）、`internal/modules/agentruntime/module.go`（R5/ib3 属主）、他 owner 宿主与模块文件（含 tenant_member.go、native_*、agent/*、tools/*）、`cmd/desktop`、`docreader`、`client`。

## 5. 耦合面裁定（全部 base 实测；机制均有先例编号）

### 5.1 R-seam：memory_extraction.go 消费 identity 属主未导出 `forUpdateClause`

- **事实**：调用点 `internal/application/repository/memory_extraction.go:17`（`Clauses(forUpdateClause())`，`withSubject` 方法内）；定义 `internal/application/repository/tenant_member.go:25`（`func forUpdateClause() clause.Expression { return clause.Locking{Strength: "UPDATE"} }`）。DAG `package_private_couplings` 在案：from=agentruntime → to=identity，symbols=[forUpdateClause]，sites=1。
- **DAG note 口径 vs 地面真值**：note 称"identity B1 已搬，须消费其 IB1 导出端口"——实测 identity 模块仅剩占位 `module.go`，tenant_member.go 仍在宿主（10-identity.md:73 推迟件），**IB1 导出端口不存在**（IB1 计划 :25 明文"contracts.yaml 四门面 current 化…跳过而非伪造"）。属 conventions §5「上游门面尚未落地」类：引用 DAG 边与 F2 裁定，不自行实现上游门面。
- **裁定（本节点执行）**：落位包建**同形本地 seam** `for_update_seam.go`（先例 10-identity.md §3.3、22-knowledge-retrieval.md §5.2；conventions §7.1 在 barrier 收口）：

```go
package memory

import "gorm.io/gorm/clause"

// forUpdateClause returns the gorm SELECT ... FOR UPDATE clause.
//
// 同形本地 seam（b3-r-memory / R1.2；先例 10-identity.md §3.3、22-knowledge-retrieval.md §5.2）：
// 原符号定义于宿主 internal/application/repository/tenant_member.go:25（identity 属主、
// B1-ID 推迟件），memory_extraction.go 迁出宿主后不可再裸名调用。冻结口径
// 10-identity.md:458：tenant_member.go 迁移时导出该 helper 并给本包留 shim；
// ib3 按 conventions §7.1 收口为单一实现后删除本文件改消费 identity 导出端口。
func forUpdateClause() clause.Expression {
	return clause.Locking{Strength: "UPDATE"}
}
```

  迁移后 `memory_extraction.go:17` 调用文本零改动（同包解析到 seam）。函数体与定义方逐字一致（单行 GORM 惯用语），非业务实现复制；IB3 收口义务写入 Integration Brief（§9）。

### 5.2 airesource/models/chat 耦合（8 文件、冻结台账属主行）

- **事实**（本会话实测）：memory/modelcontext 非测试文件的跨模块 import 全集 = 8 条 `airesource/models/chat`（memory/{consolidate.go:11, extract.go:12, topic_resolve.go:10}；modelcontext/{mcp.go:8, registry.go:18, resources.go:11, sources.go:16, tool_policy.go:8}）。台账在册：exc-0049..0051（plan=**33**）+ exc-0052..0056（plan=**31**），全部 remove_at=ib3；guard 侧 `tools/architectureguard/check.go` importExceptions 同步在册，`make check-backend-architecture` 现状零 violations。
- **不可能性证明（为什么不在本节点删除例外）**：
  1. 守卫判定面（check.go:1445-1488）：`internal/modules/<owner>/` 下文件 import 任何他模块包（含 airesource 模块根）均为 forbidden-import，除非精确豁免。即"改走模块根门面"同样需要例外。
  2. 合法消费面只剩契约区 `internal/types/interfaces`（守卫不扫非 modules 目录）；其中 `ModelService.GetChatModel` 返回 `chat.Chat`（model.go:42）——**模型实例获取已走该端口**（extract.go:936 `s.modelService.GetChatModel`）；但调用面所需类型 `chat.Message/chat.ChatOptions/chat.Tool/chat.ToolCall/chat.MessageContentPart/chat.Chat` 在契约区**无再导出**（实测 interfaces 仅 agent.go/model.go 两处 import chat 用于自身签名）。补再导出 = 改契约区签名面 = framework:26 串行契约任务 + barrier 回写，超出本节点 owned_files。
  3. IB1 已明文推迟门面 current 化（19-foundation-integration.md:25）；IB2 先例：45 条属主例外"删除前置=门面合法化契约任务，保留登记"（ib2 node notes）。
- **裁定（本节点执行）**：8 条 import **保持现状**；exception-ledger.yaml 与 check.go importExceptions **零行增删**（属主行中 5 行 plan=31、3 行 plan=33，本节点均不改）；**零新增** agentruntime→airesource import（R1.4 命令核验）；IB3 契约任务提案（契约区 chat 类型再导出 → 8 文件切换 import → 属主删行）写入 Integration Brief 与节点报告偏差登记（conventions §5：如实记录 + 建议裁定，不扩大例外、不自行实现门面）。派发 note"禁止保留/新增例外"的满足路径 = IB3 契约任务，非本节点单方面删行——此偏差在 R1.5 报告 §0 显式上报协调者裁定。

### 5.3 宿主兼容文件（2 个；模板 = 22-knowledge-retrieval.md §5.5 / kbretrieval_passb_compat.go 实物）

留守宿主消费方全集（本会话 grep 实测，无其他）：

| 宿主符号 | 消费方（禁改文件） | compat 形态 |
|---|---|---|
| `repository.NewMemoryRepository` | `internal/container/container.go:321`（`must(container.Provide(repository.NewMemoryRepository))`） | var 函数别名 |
| `handler.MemoryHandler`（类型） | `internal/router/router.go:122`（params 字段 `MemoryHandler *handler.MemoryHandler`）、`internal/router/routes_memory.go:17`（形参） | type 别名 |
| `handler.NewMemoryHandler` | `internal/container/container.go:867` | var 函数别名 |

`internal/application/repository/agentruntime_memory_passb_compat.go`：

```go
package repository

import (
	agentmemory "github.com/Tencent/WeKnora/internal/modules/agentruntime/memory"
)

// Pass B 宿主兼容层（b3-r-memory / R1.2）：memory 仓储 4 文件已物理迁移至
// internal/modules/agentruntime/memory（ownership-matrix plan=31 行）。
// 留守宿主消费方：internal/container/container.go:321。
// 删除点：ib3 集成屏障按 Integration Brief 直连模块构造器后，随 manifest/matrix
// compat 行同 commit 删除（Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION）。
var NewMemoryRepository = agentmemory.NewMemoryRepository
```

`internal/handler/agentruntime_memory_passb_compat.go`：

```go
package handler

import (
	agentmemory "github.com/Tencent/WeKnora/internal/modules/agentruntime/memory"
)

// Pass B 宿主兼容层（b3-r-memory / R1.3）：internal/handler/memory.go 已物理迁移至
// internal/modules/agentruntime/memory/memory_handler.go。
// 留守宿主消费方：internal/router/router.go:122、internal/router/routes_memory.go:17、
// internal/container/container.go:867。删除点：ib3 直连后同 commit 删除。
type MemoryHandler = agentmemory.MemoryHandler

var NewMemoryHandler = agentmemory.NewMemoryHandler
```

type 别名保方法集与 dig 类型同一性（alias 即同一类型），routes_memory.go 的方法值（`memoryHandler.GetSettings` 等 16 个挂载点）与 router 注入零改动。

### 5.4 模块内测试的反向依赖收敛（shared_resources 面缩窄）

实测 4 个模块测试文件 import 宿主 repository：

| 文件 | 现引用 | R1 处置 |
|---|---|---|
| `internal/modules/agentruntime/memory/service_test.go:10,:42` | `repository.NewMemoryRepository(db)` | 改本包 `NewMemoryRepository(db)`，删 import（R1.2） |
| `internal/modules/agentruntime/memory/postgres_consistency_test.go:10,:43` | 同上 | 同上（R1.2） |
| `internal/modules/agentruntime/memory/vector_postgres_test.go:9,:63` | 同上 | 同上（R1.2） |
| `internal/modules/agentruntime/memory/persistence_consistency_test.go:10,:28` | `repository.NewMessageRepository(db)`（conversation 属主 35b 留守） | **保持宿主引用零改动**（35b 迁移时自留 compat） |

另：`eval_test.go:27`、`topic_eval_test.go:29` 注释中陈旧路径 `internal/application/service/memory/` → `internal/modules/agentruntime/memory/`（纯注释，R1.2 顺带，机械无行为）。agent/、tools/、native/、trpc/ 下其余 7 个引用宿主的测试文件属 32/33/34 面，本节点不动。

## 6. 公共可观察行为与兼容要求（验收红线）

1. **worker 一比一**：`internal/router/task.go:290`（`mux.HandleFunc(types.TypeMemoryExtract, params.MemoryService.Handle)`，asynq）与 `internal/router/sync_task.go:159`（`params.Executor.RegisterHandler(types.TypeMemoryExtract, params.MemoryService.Handle)`，Lite；brief 写 :164 系树漂移，以实测行为准）两行**字节不变**；`MemoryService.Handle` 经 `interfaces.MemoryService` 契约（contracts.yaml agentruntime.memory-service，frozen）解析，handler 实现随包迁移不改签名。
2. **路由不变**：`RegisterMemoryRoutes`（routes_memory.go:17，16 个端点、`/memory` 组、`g.Viewer()` + `apiKeyFullAccess()` guard 装配）字节不变；router.go:411 挂载点不变；`make check-backend-architecture` 输出 `total=633 | redis=23 lite=23 | hooks=58`。
3. **契约签名不变**：`interfaces.MemoryRepository`（types/interfaces/memory.go）/`interfaces.MemoryService` 全方法签名零改动；`NewMemoryService`（memory/service.go:52）构造器零改动；container.go 三处 Provide（:321/:571/:867）经 compat 后语义等价（同一函数值/类型）。
4. **纯移动**（spec §14.2）：`git diff --summary` 识别 rename；除 package 子句、import 增删、`memory.` 限定符去前缀、§5.4 测试构造器限定符外**无函数体变化**；不新增数据库写入、goroutine 或全局状态。
5. **迁移脚本不变**：`migrations/versioned/000084_memory.*`、`000153_memory_consistency.*`、`000154_memory_vector_search.*` 零改动；包内 `TestMemoryConsistency*` 族（含 `execMemoryMigration` 驱动，persistence_consistency_test.go:110 等）T1 全绿。

## 7. 测试与高风险差分（conventions §6 + spec §14.3）

**全部复用现有测试，不新写行为测试**；新写仅机械守卫采证（§8 各任务内）。

### 7.1 差分面（3 面；memory 不在 framework:40 清单，但覆盖 §14.3「Worker 状态机、重试和幂等」「HTTP/流式响应/错误码」两条）

| 面 | 锚定测试（既有） | 等价判据 |
|---|---|---|
| F1 抽取 worker 状态机（claim/lease/checkpoint/record-failure/watermark） | `memory/persistence_consistency_test.go`、`memory/consistency_test.go`、`memory/coverage_test.go`、`memory/service_test.go` 中 extraction 族 | T0 vs T1 逐用例 verdict 一致 |
| F2 memory HTTP 错误映射（409/404/400 语义，spec §14.3 错误码） | `memory_handler_test.go`（原 handler/memory_consistency_test.go，2 用例 ×子用例） | 迁移前后同用例同状态码断言通过 |
| F3 向量检索排序（rankInDatabase/rankInProcess/融合） | `memory/vector_test.go`、`memory/vector_scope_test.go`、`memory/search_test.go`；`vector_postgres_test.go`/`postgres_consistency_test.go` 按 blocked-env skip 登记 | T0 vs T1 一致；PG 用例若环境可用则双跑 |

### 7.2 比对方法（K2 §3 同法）

```bash
awk '/^=== RUN|^--- PASS|^--- FAIL|^--- SKIP|^    --- PASS|^    --- FAIL|^    --- SKIP/ {print}' "$T0" | sort -u > /tmp/t0.txt
awk '/^=== RUN|^--- PASS|^--- FAIL|^--- SKIP|^    --- PASS|^    --- FAIL|^    --- SKIP/ {print}' "$T1" | sort -u > /tmp/t1.txt
diff /tmp/t0.txt /tmp/t1.txt   # 期望：空（模块包用例集不变；宿主 handler/repo 用例迁移后于模块包复跑）
```

差分失败只修新实现、禁改期望值（spec §14.3）。T2 消费者面：`go test -count=1 ./internal/application/repository/ ./internal/handler/`（宿主包留守测试，含 container/router 编译面由 `go build ./...` 覆盖）。

## 8. 实施任务（一任务一 commit；R1.1/R1.4 为采证任务无仓内提交，产出并入 R1.5 交付物）

### R1.1 前置自核 + T0 台账（无仓内提交）

- [ ] 执行 §2 P1–P5 全部命令，逐条记录退出码与关键输出到 `$SDD/b3-r-memory/R1.1-precheck.md`。
- [ ] 预期：P1 输出 a2fbcf55e…；P2 三行属主 = 33/34/33；P3 frozen 三处；P4 两目录不存在、grep 仅 2 处注释；P5 全 0 退出码。
- [ ] 失败处置：任一项不符 → 停止，按 conventions §5 上报（DAG 置 blocked + notes），不得带病开工。

### R1.2 repository 层物理迁移（commit 1：`refactor(agentruntime): R1.2 memory repository 4 文件迁入模块 + forUpdateClause 同形 seam + 宿主 compat（b3-r-memory）`）

- [ ] Step 1（RED 采证）：`go test -count=1 ./internal/application/repository/ -run 'TestMemory'` 记录当前输出（宿主无 memory 主题测试 → `testing: warning: no tests to run`，预期即如此，作为"无随迁遗漏"证据）。
- [ ] Step 2（M2 纯移动）：`git mv internal/application/repository/memory.go internal/application/repository/memory_extraction.go internal/application/repository/memory_lifecycle.go internal/application/repository/memory_vector.go internal/modules/agentruntime/memory/`；四个文件 package 子句 `repository` → `memory`；import 块零语义增删（types/interfaces/uuid/gorm/clause 原样）。
- [ ] Step 3（seam）：新建 `internal/modules/agentruntime/memory/for_update_seam.go`（§5.1 全文）；`memory_extraction.go:17` 调用零改动。
- [ ] Step 4（模块测试切换）：§5.4 前三行（service_test.go / postgres_consistency_test.go / vector_postgres_test.go：删 repository import、构造器去限定符）+ 2 处注释路径修正；persistence_consistency_test.go 零改动。
- [ ] Step 5（compat）：新建 `internal/application/repository/agentruntime_memory_passb_compat.go`（§5.3 全文）。
- [ ] Step 6（行级登记，同 commit）：`docs/architecture/moves/agentruntime.yaml` 删 legacy_files 4 行（:123/:127/:131/:135）+ 增 compat 行（path=compat 文件，reason=`Pass B 过渡 shim，ib3 同 commit 随文件删行`，navigation_label=`Memory host compat (application/repository)`，passb_task=B-agentruntime）；`docs/architecture/passb/ownership-matrix.yaml` 删 :266–:289 四行 + 成对补 compat 行（module=agentruntime、plan=31-agentruntime-memory、destination=internal/modules/agentruntime/memory、integration_owner=ib3、delete_barrier=ib3）；`internal/modules/agentruntime/legacy/README.md` 镜像同步（删 4 行加 1 行）。
- [ ] Step 7（GREEN）：`go build ./...` 退出码 0；`go test -count=1 ./internal/modules/agentruntime/memory/... ./internal/application/repository/` 全 ok；`make check-backend-architecture` 零 violations 且 `total=633 | redis=23 lite=23 | hooks=58`；`make verify-module-moves` OK（16 manifests）。
- [ ] Step 8（纯移动核验）：`git diff --summary HEAD~1` 四文件均 `rename`（相似度 ≥96%，包句/限定符级差异）；逐一 `git diff -M HEAD~1 -- <旧路径> <新路径>` 确认唯一 hunk 为 package 子句（memory_extraction.go 另含 :17 调用解析到 seam 的零文本变化，即无 hunk）。
- [ ] 回滚边界：单 commit `git revert <sha>` 即回到迁移前（宿主符号原样恢复；无装配切换需要回退，spec §13 M2/M3 可独立回滚）。

### R1.3 handler 层物理迁移（commit 2：`refactor(agentruntime): R1.3 memory handler 迁入模块 + 宿主 compat（b3-r-memory）`）

- [ ] Step 1（M2）：`git mv internal/handler/memory.go internal/modules/agentruntime/memory/memory_handler.go`；`git mv internal/handler/memory_consistency_test.go internal/modules/agentruntime/memory/memory_handler_test.go`。
- [ ] Step 2（M3 编译修复，两文件）：package `handler` → `memory`；删自引用 import `"github.com/Tencent/WeKnora/internal/modules/agentruntime/memory"`；限定符去前缀：`memory.ErrNoMemoryScope`/`memory.ErrItemNotFound`/`memory.ErrSensitiveContent`/`memory.ErrMemoryDisabled` → 裸名（memory_handler.go:451–:459）；测试文件 `memory.ErrSensitiveContent`（:16/:20 用例）→ 裸名；gin/apperrors/logger/types/interfaces/middleware import 原样。
- [ ] Step 3（compat）：新建 `internal/handler/agentruntime_memory_passb_compat.go`（§5.3 全文）。
- [ ] Step 4（行级登记，同 commit）：manifest 删 :251 handler 行 + 增 handler compat 行；matrix 删 :1652–:1656 + 成对补行；legacy/README.md 镜像同步。
- [ ] Step 5（GREEN + T2）：`go build ./...` 0；`go test -count=1 ./internal/modules/agentruntime/memory/... ./internal/handler/` 全 ok（memory_handler_test 2 用例 PASS）；`make check-backend-architecture` 零 violations、633/23+23/58 不变；`make verify-module-moves` OK。
- [ ] Step 6（路由面字节不变采证）：`git diff HEAD~2..HEAD -- internal/router/ internal/container/` 输出为空（本节点两 commit 合计零触碰禁改面）。
- [ ] 回滚边界：同 R1.2，单 commit revert。

### R1.4 T1 差分 + 门禁复跑 + airesource 面清点（无仓内提交；产出并入 evidence）

- [ ] Step 1（T1）：重跑 P5 三条命令于迁移后树，输出存 `$SDD/b3-r-memory/R1.4-t1-*.txt`；§7.2 awk/diff 三面比对，diff 为空 = 逐用例等价；PG 用例 blocked-env 如实登记。
- [ ] Step 2（节点 gates，DAG 原样命令，无替代）：`go build ./...`；`go test ./internal/modules/agentruntime/... -count=1`；`make check-backend-architecture`；`make verify-module-moves`——四条退出码 0，输出摘录进报告。
- [ ] Step 3（airesource 面清点）：
  ```bash
  git diff --name-only "$PASSB_BASE_SHA"...HEAD | sort > /tmp/changed.txt   # 对照 §4 可写清单求差集，差集非空即失败（conventions §1.2）
  for f in $(git diff --name-only "$PASSB_BASE_SHA"...HEAD | grep '\.go$'); do git diff "$PASSB_BASE_SHA"...HEAD -- "$f"; done | grep -c '^+.*internal/modules/airesource'   # 期望 0：零新增 airesource import
  grep -rn "airesource/models/chat" internal/modules/agentruntime/memory/*.go internal/modules/agentruntime/modelcontext/*.go | grep -v _test | wc -l   # 期望 8：与台账 8 行一一对应，无新增无消失
  grep -c "id: exc-" docs/architecture/passb/exception-ledger.yaml   # 与基线行数一致：本节点零行增删
  ```
- [ ] Step 4（readiness 影响登记）：`make check-passb-readiness` 允许出现且仅允许出现 `agentruntime.memory-service` 的 consumers/characterization 路径类诊断（`internal/handler/memory.go`、`internal/handler/memory_consistency_test.go` 路径已不存在——passbguard check.go:650/:694 实测校验磁盘存在性）；contracts.yaml 回写是 barrier 独占（conventions §3），**禁止本节点改 contracts.yaml 消红**；诊断清单原文进报告并写入 Brief。readiness 非本节点 gate（DAG gates 四条为准）。

### R1.5 交付物落盘（commit 3：`docs(passb): b3-r-memory evidence + Integration Brief + 节点报告`）

- [ ] Step 1：写 `docs/architecture/evidence/passb/b3-r-memory.md`：§1 前置自核、§2 T0/T1 台账与三面差分（用例清单、双跑输出摘录、diff 空 结论、命令与退出码）、§3 airesource 面清点（8 行对应表 + 零新增证据）、§4 seam 台账（for_update_seam.go，收口点 ib3）、§5 计数奇偶（633/23+23/58 三方一致声明 + architectureguard 输出）。
- [ ] Step 2：写 `docs/architecture/passb/briefs/b3-r-memory.md`（Integration Brief，交 IB3）：(a) 装配切换申请——container.go:321/:867 两处 Provide 直连模块构造器、routes_memory.go/router.go 直连模块 handler 类型，切换后删 2 个 compat 文件及其 manifest/matrix 行；(b) contracts.yaml 回写申请——agentruntime.memory-service consumers `internal/handler/memory.go` → `internal/modules/agentruntime/memory/memory_handler.go`、characterization `internal/handler/memory_consistency_test.go` → `…/memory_handler_test.go`；(c) airesource 收敛契约任务提案——契约区 chat 类型再导出（串行契约任务，framework:26）→ 8 文件 import 切换 → exc-0049..0051（属主 33）与 exc-0052..0056（属主 31 行，由 IB3 属主线收口）删行；(d) forUpdateClause seam 收口——identity 侧导出后（10-identity.md:458）本包改消费端口删 seam。
- [ ] Step 3：写 `docs/plans/passb/reports/b3-r-memory.md`：§0 结论摘要（含 §5.2 偏差上报：派发 note"禁止保留/新增例外"在本节点的满足口径 = 零新增 + IB3 契约任务路径，请协调者裁定）；§1 任务-提交映射；§2 T0 转录；§3 三面差分；§4 四 gates 原文+退出码+摘录；§5 变更清单 vs owned_files 逐条核对（`git diff --name-only "$PASSB_BASE_SHA"...HEAD | sort` 求差集结论）；§6 未完成项如实（PG blocked-env、readiness 诊断移交、airesource 收敛移交）。
- [ ] Step 4：报告包生成命令（conventions §1.2）原文执行并摘录。
- [ ] Step 5：commit 3 提交三份文档。

## 9. 集成与回滚边界

- **本节点零装配切换**：不触碰 router/container/bootstrap/task/sync_task（§4 禁改）；全部宿主连续性由 compat 别名承担，IB3 从已评审 Brief 单写切换（framework:28,101-105）。
- **回滚**：R1.2/R1.3/R1.5 各自独立 revert 即恢复（无 M4 切换需先回退；无 schema 变更无需数据修复，spec §13）。节点级失败回滚顺序：R1.5 → R1.3 → R1.2。
- **计数基线**：legacy 计数随行删除/补行机械变动（本节点净 -5+2=-3 行），三方一致口径与正式基线登记归 IB3 evidence（conventions §8）；路由/worker/hook/迁移计数 633/23+23/58/537 本节点不变（零触碰对应文件）。

## 10. 必须删除的 legacy/alias/例外（本节点收口物）

| 类别 | 物 | 动作 | 依据 |
|---|---|---|---|
| manifest legacy_files 行 | :123/:127/:131/:135（repository memory×4）、:251（handler/memory.go） | 随物理迁移同 commit 删除 | Ruling LEGACY-ROW-OWNERSHIP |
| ownership-matrix 行 | :266–:289（4 行）、:1652–:1656（1 行） | 同 commit 删除 | 同上 |
| alias 包 | `internal/application/service/memory`、`internal/modelcontext` | **已不存在**（ae37070e7 删除，§2 P4 证据）；manifest alias_obligations :82–:85 陈旧行与 move_packages 区收口归 R5（30 计划），本节点仅在报告留痕 | brief 义务 1 现状 |
| import 例外 | exception-ledger / check.go importExceptions | **零行增删**：8 行在册对应 import 全部保留（§5.2 不可能性证明 + IB2 先例）；5 行 plan=31 属主行的删除随 IB3 契约任务，3 行 plan=33 属 b3-r-engine | 冻结台账 + conventions §3/§5 |
| 过渡 shim | 2 个 compat 文件 | **新增**（非删除）并同 commit 登记 manifest+matrix 行，ib3 随直连删除 | Ruling TRANSITION-SHIM-ROW-REGISTRATION |

## 11. 独立验收标准（全部满足才算节点完成）

1. `git diff --summary "$PASSB_BASE_SHA"...HEAD`：5 个 rename（4 repo + 1 handler）+ 1 测试 rename；函数体零改动（§6.4 口径，抽查 diff 证据在报告）。
2. 变更文件全集 == §4 可写清单（求差集为空；conventions §1.2 命令留痕）。
3. 四条 DAG gates 原样命令退出码 0（build / agentruntime 全测 / architectureguard 零 violations 且 633/23+23/58 / modulemove 16 manifests OK）。
4. 宿主消费方字节不变：`git diff "$PASSB_BASE_SHA"...HEAD -- internal/router/ internal/container/ internal/bootstrap/` 为空。
5. worker 与路由不变式：task.go:290、sync_task.go:159、routes_memory.go、router.go:411 零改动（并入 4 的空 diff 证据）。
6. 三面差分 T0≡T1（awk/diff 空），PG 用例 blocked-env 登记非 PASS。
7. exception-ledger 行数与基线一致（零增删）；新增 airesource import 计数 0；在册 8 import 与 8 台账行一一对应。
8. 三份交付物（evidence/brief/report）存在于 §4 路径，报告含全部命令原文+退出码+owned_files 逐条核对+未完成项。
9. manifest/matrix 行删除与补行与物理迁移同 commit（git log --stat 核对）。

## 12. 计划自检记录（撰写人本会话执行）

- **Spec 覆盖**：brief 义务 1（别名）→ §2 P4 现状核验+§10；义务 2（耦合收敛）→ §5.1/§5.2（seam + 冻结台账口径 + IB3 移交）；义务 3（worker 双栈不变）→ §6.1；义务 4（路由不变）→ §6.2；义务 5（验证命令）→ §2 P5/§7/§8 gates。Spec §11/§13/§14/§15 逐条落 §6/§7/§8/§9。
- **无占位符**：所有文件路径、行号、签名为本会话实测（worktree a2fbcf55e）；compat/seam 文件给出全文。
- **类型一致**：`NewMemoryRepository(db *gorm.DB) interfaces.MemoryRepository`（memory.go:26）、`NewMemoryHandler(memoryService interfaces.MemoryService) *MemoryHandler`（handler/memory.go:27）、`NewMemoryService(repo, tenantRepo, messageRepo, modelService, enqueuer, cfg)`（service.go:52）三构造器签名迁移前后不变，compat 别名类型同一。
- **跨任务接口一致**：R1.2 产出 `NewMemoryRepository`（模块）+ 宿主 var 别名；R1.3 产出 `MemoryHandler`/`NewMemoryHandler`（模块）+ 宿主 type/var 别名；R1.4 清点与 R1.5 Brief 消费同一符号面；无任务间悬空引用。
- **已知偏差（如实）**：① §5.2 派发 note 与冻结台账的口径冲突按 conventions 前言以冻结产物为准并上报；② brief 所写 sync_task.go:164/container.go:54 行号与当前树漂移（实测 :159/:49），以实测为准；③ brief"×6 modelcontext"实为 5 文件（台账 exc-0052..0056 同为 5 行）。
