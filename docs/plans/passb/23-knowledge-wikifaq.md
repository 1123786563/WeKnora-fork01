# Pass B B2 — 23-knowledge-wikifaq（K3 Knowledge Wiki+FAQ 域 18 文件搬迁）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**节点：** `b2-k-wikifaq`（DAG `docs/plans/passb/execution-dag.json`，`execution_mode: parallel`，`depends_on: [b2-k0]`，gates=`go build ./...` / `go test ./internal/modules/knowledge/...` / `make check-backend-architecture` / `make verify-module-moves`）。本文件只承载 K3 本节点任务 K3.1–K3.4；同程序 K0/K5 任务节属 `b2-k0`/`b2-k-integration`，载体为 `20-knowledge-program.md`（K3 撰写时该文件已随 b2-k0 冻结产出落库，本计划只消费其 §5–§7 冻结表，不复述、不改写）。

**Goal:** 把 knowledge 域的 Wiki+FAQ 子集 18 个 legacy 文件（ownership-matrix `plan: 23-knowledge-wikifaq` 全部 18 行）从横向宿主包迁入 `internal/modules/knowledge/wiki`（12 文件）与 `internal/modules/knowledge/faq`（6 文件），`internal/handler/session/wiki_fixer_scope.go` 按 B0.3 Step 3 独占归本域去方法化拆出；冻结端口 `interfaces.KnowledgeService` 的 FAQ 方法切片经宿主委托文件保持零行为变化；`recoverPendingWikiTasks` 恢复行为不变；全部节点门禁绿色并产出差分证据与 Integration Brief。

**Architecture:** K3 与 K1/K2 并行（均只依赖 b2-k0 冻结产出，framework:132）。本节点不搬任何 K1/K2/K4 文件：对仍驻宿主包的他 owner 未导出符号一律按 20 计划 §6.1 R2 建消费侧 seam；K3 自己定义、被宿主他 owner 消费的符号按 R1 导出薄包装 + 宿主薄兼容文件（登记 `docs/architecture/moves/knowledge.yaml`，删除点 ib2）。装配切换（router/container）不在本节点（集成工程师独占）。

**Tech Stack:** Go 1.26、Gin、GORM、redis、asynq、dig、singleflight、Testify、Git worktrees。

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`（§4.2 依赖方向与接口优先定义在使用方、§4.3 模块装配、§5.2 Knowledge 所有权、§11 Pass B 串并图、§13 提交隔离 M1–M5 与回滚、§14.2 纯移动验证、§14.3 高风险差分、§17.2 完成标准）。

---

## 1. 事实源指针（全部只读输入）

| 事实源 | 用途 |
|---|---|
| `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` | 行为边界、M2/M3 提交隔离（§13）、差分门禁（§14）、完成标准（§17.2） |
| `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md`（:25-41 全局约束、:117-124 知识子程序图） | manifest=文件所有权唯一事实源（:25）、随迁 `_test.go`（:29）、禁双写/复制（:30）、先差分后删 legacy（:31） |
| `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | 派发契约 §1、门禁 §2、禁改 §3、提交 §4、升级 §5、差分 §6、package-private §7、计数基线 §8、DAG 回填 §9、裁定族 §10 |
| `docs/plans/passb/20-knowledge-program.md`（b2-k0 分支 `5bcb798621` 冻结产出） | §5 共享类型分配表（R0：`internal/types` 唯一事实源）、§6.1 R1/R2/R3 通用裁定、§7 路由/worker/hook/别名/例外指针、§9 K3 行义务清单 |
| B0 冻结产物（`.worktrees/passb-int/docs/architecture/passb/`）：`ownership-matrix.yaml`（18 行 plan=23）、`contracts.yaml`（`knowledge.wiki-page-service`、`knowledge.service`、`knowledge.routes`、`knowledge.workers`、`knowledge.lifecycle`、`knowledge.facade`）、`event-catalog.yaml`（knowledge 家族 4 事件 v1）、`exception-ledger.yaml` | destination/delete_barrier、冻结签名、replay 口径、例外 removal owner |
| `docs/architecture/passb/knowledge-wikifaq.md`（集成 worktree 版，含 B0.3 标注） | 域 brief：scope 18 文件、边界目标、删除义务 3 条 |
| `docs/architecture/moves/knowledge.yaml` | 18 条 legacy_files 行（行级删除权 `manifest:` 前缀）、alias_obligations、integration_points |
| `docs/plans/passb/10-identity.md`（:79/:155/:427/:449/:456）、`12-commercial.md`（:103/:449/:635/:646） | 宿主兼容文件/类型别名/构造器 shim 与 rbac_lookups 推迟件的执行先例 |
| 实测代码（本 worktree `6bda27b1d` 工作树，撰写时逐条 grep/sed 实证） | 本计划全部签名与调用点行号出处（见 §5–§7 各表） |

注意：主 checkout 的 `docs/architecture/passb/knowledge-wikifaq.md` 为 B0 前旧版，以集成 worktree 版为准（20 计划 §1 同律）。

## 2. 节点裁定与前置条件

**节点裁定（调度方原文，照录）：**

1. wiki_fixer_scope.go 在 handler/session 定义于 conversation 属主 `*Handler` 上的方法——与 b1-execution 同型断链风险，按 B0.3+IB1 裁定处理（DAG b2-k-wikifaq.notes；B0.3 Step 3：`wiki_fixer_scope.go` 独占归 Knowledge Wiki/FAQ，freeze:202；conventions §7.4 第 4 条：去方法化或推迟，残差登记 Integration Brief）。本计划裁定（§5.4）：**去方法化落地**——包级函数 `resolveBuiltinWikiFixerTenantScope`（wiki_fixer_scope.go:36，自带依赖入参、无 receiver 耦合）随文件迁入 `internal/modules/knowledge/wiki` 并导出；`*Handler` 方法 `resolveWikiFixerTenantScope`（:18，仅薄委托 `h.knowledgebaseService`/`h.kbShareService` 两个字段）以宿主过渡文件保留至 ib2，唯一调用方 `qa.go:205`（conversation 属主，`moves/conversation.yaml:147`）不改，由集成工程师在 ib2 切换。
2. 既有 BLOCKED 记录解除实证：
   - BLOCKED（2026-09-23）：前置 b0——已解除。集成 worktree DAG b0 节点 `status=done`、`review_status=approved`（实测 python3 读 `execution-dag.json`）。
   - BLOCKED（2026-09-24）：前置 b2-k0——已解除。DAG b2-k0 `status=done`、`review_status=approved`、`head_sha=5bcb798621b856f7ff6497986c7ca79dba8c338e`（同上实测）；冻结产物 `docs/plans/passb/20-knowledge-program.md`（381 行）与 `internal/modules/knowledge/kbfreeze/freeze_test.go` 均在 `5bcb798621` 树上（实测 `git ls-tree`）。
3. 本节点 status 经协调者置 `in_progress`、base_sha 待派发回填（conventions §9）；本 worktree `codex/passb-b2-k-wikifaq` 在 `6bda27b1d`，本地 DAG 副本滞后（仍记 pending）属基线对齐范畴，以下一条为准。

**前置条件核对（K3 开工门禁，逐条给出实证/命令）：**

- [ ] **P1 b0/b2-k0 冻结产物在场且 approved**：`.worktrees/passb-int/docs/architecture/passb/` 下 `ownership-matrix.yaml`、`contracts.yaml`、`event-catalog.yaml`、`exception-ledger.yaml` 存在；DAG `b0`、`b2-k0` 均 `status=done`+`review_status=approved`。命令：`python3 -c "import json;d=json.load(open('docs/plans/passb/execution-dag.json'));print([(n['id'],n['status'],n['review_status']) for n in d['nodes'] if n['id'] in ('b0','b2-k0')])"`，预期 `[('b0','done','approved'), ('b2-k0','done','approved')]`。
- [ ] **P2 基线对齐 merge**（Ruling 2026-09-24-WAVE-DEP-BASELINE）：按协调者派发回填的 `base_sha` 起，把 `codex/passb-b2-k0`（head `5bcb798621`）以**独立 merge commit** 并入 `codex/passb-b2-k-wikifaq`，使 kbfreeze 守卫与 20 计划冻结表在场；对齐后 `go build ./...` 与 `go test ./internal/modules/knowledge/kbfreeze/ -count=1` 绿。本节点 `depends_on=[b2-k0]` 仅此一条入边，无波内兄弟 merge。
- [ ] **P3 门禁工具可用**：`Makefile:250 verify-module-moves`（`go run ./tools/modulemove verify --all`）、`Makefile:255 check-backend-architecture`（`go run ./tools/architectureguard`）目标存在（实测 grep）。
- [ ] **P4 worktree 干净**：`git -C .worktrees/passb-b2-k-wikifaq status --short` 为空。
- [ ] **P5 特征化测试基线（T0，spec §14.1）**：搬迁前先跑 §8 全部随迁测试包并记录通过集合与计数，作为纯移动等价与高风险差分的旧实现基线。

## 3. 范围、精确文件与写入所有权

### 3.1 范围内（owned_files，= ownership-matrix plan=23-knowledge-wikifaq 全部 18 行，逐行实测）

| # | legacy 文件 | destination（matrix 冻结） | 随迁 `_test.go`（framework:29，实测存在） |
|---|---|---|---|
| 1 | `internal/application/repository/wiki_page.go` | `internal/modules/knowledge/wiki` | `internal/application/repository/wiki_page_test.go` |
| 2 | `internal/application/service/wiki_ingest.go` | `…/knowledge/wiki` | `wiki_ingest_test.go`、`wiki_ingest_language_test.go`、`wiki_ingest_retry_test.go`、`wiki_deleted_kb_guard_test.go` |
| 3 | `internal/application/service/wiki_ingest_batch.go` | `…/knowledge/wiki` | （无专属测试，实测 ls） |
| 4 | `internal/application/service/wiki_ingest_cite.go` | `…/knowledge/wiki` | `wiki_ingest_cite_test.go` |
| 5 | `internal/application/service/wiki_ingest_dedup.go` | `…/knowledge/wiki` | `wiki_ingest_dedup_test.go`（contracts.yaml characterization 登记） |
| 6 | `internal/application/service/wiki_ingest_taxonomy.go` | `…/knowledge/wiki` | `wiki_ingest_taxonomy_test.go` |
| 7 | `internal/application/service/wiki_linkify.go` | `…/knowledge/wiki` | `wiki_linkify_test.go` |
| 8 | `internal/application/service/wiki_lint.go` | `…/knowledge/wiki` | （无专属测试） |
| 9 | `internal/application/service/wiki_page.go` | `…/knowledge/wiki` | `wiki_page_test.go`、**wiki_page_revision_test.go**（contracts.yaml 登记）、**wiki_folder_prune_finalize_test.go**（contracts.yaml 登记，白盒 `wikiPageService.PruneEmptyFolderChains`） |
| 10 | `internal/application/service/wiki_slug_handles.go` | `…/knowledge/wiki` | `wiki_slug_handles_test.go` |
| 11 | `internal/handler/wiki_page.go` | `…/knowledge/wiki` | （rbac_lookups_test.go **不随迁**，见 §3.3） |
| 12 | `internal/handler/session/wiki_fixer_scope.go` | `…/knowledge/wiki` | `internal/handler/session/wiki_fixer_scope_test.go`（5 用例，全部测包级函数） |
| 13 | `internal/application/service/faq_clone_sync.go` | `…/knowledge/faq` | （无专属测试） |
| 14 | `internal/application/service/knowledge_faq.go` | `…/knowledge/faq` | （无专属测试） |
| 15 | `internal/application/service/knowledge_faq_batch.go` | `…/knowledge/faq` | （无专属测试） |
| 16 | `internal/application/service/knowledge_faq_create_guard.go` | `…/knowledge/faq` | `knowledge_faq_create_guard_test.go` |
| 17 | `internal/application/service/knowledge_faq_import.go` | `…/knowledge/faq` | （无专属测试） |
| 18 | `internal/handler/faq.go` | `…/knowledge/faq` | `internal/handler/faq_enabled_filter_test.go`（白盒 `&FAQHandler{knowledgeService:…}` 字面量 @ :95/:122，随 faq.go 同迁） |

计数：wiki 12（repository 1 + service 9 + handler 2）+ faq 6（service 5 + handler 1）= 18，与 20 计划 §9 K3 行、brief scope 一致。

**不随迁的同名域测试（防误迁，20 计划 §10 实测归属）**：`knowledge_move_wiki_test.go`（package service 白盒，:163 调 `svc.moveOneKnowledge`、:167/:235 直取 `svc.repo`，被测未导出方法定义于 K4 `knowledge_clone_move.go:1194` → **K4**）；`knowledge_post_process_wiki_enqueue_test.go`（被测 `knowledge_post_process.go` K4 → **K4**）；`rbac_lookups_test.go`（10-identity 推迟件留驻对照，10-identity.md:155）。

### 3.2 本节点可写

- 上表 18 文件 + 16 个随迁 `_test.go`（wiki 侧 14 + faq 侧 2，原地 `git mv` 到 destination，含 package 子句改名）；
- `internal/modules/knowledge/wiki/**`、`internal/modules/knowledge/faq/**`（本域新包，DAG owned_files「internal/modules/knowledge/**（wikifaq 面）」）；
- 本节点产出的宿主过渡兼容新文件（§6.2 全清单；每个新文件**同 commit** 在 `docs/architecture/moves/knowledge.yaml` legacy_files 新增登记行——architectureguard legacy-guard 对横向目录无归属新生产文件报诊断，check.go:1264 实测；先例 10-identity.md:94）；
- `docs/architecture/moves/knowledge.yaml` 中归属本节点的 18 条 legacy_files 行（Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP：物理迁移 commit **同 commit** 删除对应 manifest 行）；
- `docs/architecture/evidence/passb/b2-k-wikifaq.md`、`docs/plans/passb/reports/b2-k-wikifaq.md`、`docs/architecture/passb/briefs/b2-k-wikifaq.md`（conventions §1.1）。

### 3.3 禁改（违者节点失败，conventions §1.2/§3）

- 他 owner 文件一律不改，重点防误碰：K1 `extract.go`/`image_multimodal.go`、K2 `kb_activity.go`/`slug_fuzzy.go`/`knowledgebase_access.go`/`semantic_model_capability.go`、K4 `knowledge.go`/`knowledge_process.go`/`knowledge_post_process.go`/`knowledge_delete.go`/`knowledge_clone_move.go`/`knowledge_span_tracker.go`、conversation `handler/session/qa.go`、10-identity 推迟件 `internal/handler/rbac_lookups.go`（:105/:183 在 `*WikiPageHandler` 上，**保持字节不变**，见 §5.4）；
- `internal/router/router.go`、`internal/router/task.go`、`internal/router/sync_task.go`、`internal/router/routes_knowledge.go`、`internal/container/**`、`internal/bootstrap/**`、migration 编号、`go.mod`/`go.sum`、`cmd/desktop`、`docreader`、`client`、生产 SQL、既有迁移文件；
- `internal/modules/knowledge/module.go`（门面注释仅 b0 与集成节点可写）、`internal/modules/knowledge/kbfreeze/**`（K0 产物）、`internal/types/**`（R0 唯一事实源）；
- 治理文件 `ownership-matrix.yaml`/`contracts.yaml`/`event-catalog.yaml`（只读；矩阵 18 行删除属 barrier 在 delete_barrier=ib2 收口，manifest 行删除才属本节点——10-identity.md:43/:476 先例）、`exception-ledger.yaml`（本节点无属主例外删除动作，见 §10）；
- DAG `status`/`base_sha`/`head_sha`/`task_ids` 回填仅协调者（conventions §9）。

## 4. 公共可观察行为与兼容要求（冻结面，零变化）

| 冻结面 | 事实源 | 兼容要求 |
|---|---|---|
| `interfaces.WikiPageService` 全方法集 | contracts.yaml `knowledge.wiki-page-service`（stability: frozen），实现体 `wikiPageService`（service/wiki_page.go:44） | 实现随包迁入 wiki，接口不动；消费方（agentruntime wiki tools 10 文件、knowledge.go、agent_service.go）经接口不受影响 |
| `interfaces.KnowledgeService` FAQ 方法切片 | contracts.yaml `knowledge.service`（frozen）：`ListFAQEntries`、`UpsertFAQEntries`、`CreateFAQEntry`、`GetFAQEntry`、`UpdateFAQEntry`、`AddSimilarQuestions`、`UpdateFAQEntryFieldsBatch`、`DeleteFAQEntries`、`SearchFAQEntries`、`ExportFAQEntries`、`ExportFAQEntriesJSON`、`UpdateFAQEntryTagBatch`、`ProcessFAQImport`、`GetFAQImportProgress` | `knowledgeService`（K4 属主类型）必须继续实现全接口：宿主委托文件（§6.2 D1）为每个 FAQ 面方法保留一行委托；`router/task.go:277`、`sync_task.go:148` 经 `params.KnowledgeService.ProcessFAQImport` 的 worker 注册零变化 |
| Worker：`TypeWikiIngest`/`TypeWikiFinalize`（`wikiIngestService.Handle`，wiki_ingest.go 接收者）、`TypeFAQImport`（`ProcessFAQImport`） | contracts.yaml `knowledge.workers`；注册行 task.go:266-320、sync_task.go:143-163（集成工程师独占） | 不双注册、不双写（framework:30）；注册行零改动 |
| 生命周期钩子 `recoverPendingWikiTasks` | contracts.yaml `knowledge.lifecycle`；func at `internal/container/recover_pending_wiki_tasks.go:32`，挂接 `container.go:1053` | 该函数对 K3 的唯一引用是 `service.WikiIngestPayload`（recover_pending_wiki_tasks.go:75）；宿主类型别名 shim（§6.2 W6）保持其编译与行为，`asynq.TaskID("wiki-finalize-"+scope.ScopeID)` 去重语义、MaxRetry 10、Timeout 60/30 分钟全部不变 |
| 路由 `RegisterFAQRoutes`（routes_knowledge.go:143）、`RegisterWikiPageRoutes`（:309） | contracts.yaml `knowledge.routes` | 宿主 handler 包保留 `FAQHandler`/`WikiPageHandler` 兼容名（§6.2 H1/H2），routes 形参类型不变；633 路由计数不变（conventions §8） |
| 事件 4 项 v1（event-catalog `knowledge.*`） | `knowledge.processing.completed/.failed/.index.completed/.deletion.completed` | K3 文件均非 producer；`RecordWikiContentActivity`（kb_activity.go:181，K2 导出符号）经 K2 落位包/宿主 shim 继续可用（20 计划 §6.2 组 C 行） |
| 错误哨兵 | `ErrWikiIngestConcurrent`（wiki_ingest.go:36）被 `router/task.go:124` `errors.Is` 判定；`repository.ErrKnowledgeBaseNotFound`/`ErrKnowledgeNotFound` 被 knowledge_faq_import.go 判定 | 前者宿主 `var` 别名 shim 保持**同一错误实例**（`errors.Is` 语义不变）；后者 faq 包直接 import 宿主 `internal/application/repository`（仓内 12 处模块→application import 先例，实测 grep；若 guard 报诊断按 Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY 处置，见 §10） |
| wiki fixer 共享租户作用域 | `resolveBuiltinWikiFixerTenantScope`（wiki_fixer_scope.go:36）：shared-KB 编辑者以源租户运行 builtin-wiki-fixer，viewer 不切换，单 KB 限定 | `qa.go:205` 调用点行为逐分支不变；wiki_fixer_scope_test.go 5 用例双跑等价（§8） |

## 5. 跨 owner 联动符号全表（本节点实测修订，标注与 20 计划 §6 的差异）

撰写时以本 worktree `6bda27b1d` 工作树按 20 计划 §6.1 口径（非测试文件、剥注释、裸标识符+选择器双扫）全量复核。**实测修订**：组 A 在 20 计划登记的 previewText 3 个 K1 调用点之外，新增 8 个 K3 定义符号与 previewText 的 5 个 K4 调用点（20 计划组 A/B 均未登记）；组 B 新增 hash、writableFAQKnowledgeBase、contains、finalizeSubtaskDetached、isLikelyRateLimitError、removeSourceRef、spanTracker/noopSpanTracker 类型引用。**该修订按 conventions §9 属计划事实登记（非 DAG/基线变更）：R1/R2 裁定框架不变，只是本节点义务清单变长**；同步上报协调者把差异记入 DAG b2-k-wikifaq.notes。

### 5.1 组 A——K3 定义、宿主他 owner 消费（R1：导出薄包装 + 宿主薄兼容）

| 符号 | 定义（实测） | 宿主调用点（属主） | 处置 |
|---|---|---|---|
| `previewText` | `func previewText(s string, maxRunes int) string` @ wiki_ingest.go:1383 | `extract.go:309`、`image_multimodal.go:280,296`（K1）；`knowledge_process.go:1226,1289,1336,1759,2097`（K4） | wiki 包导出 `PreviewText`；宿主兼容文件 W1 一行转发。K1/K4 调用点 ib2 改写 |
| `enqueueWikiIngestTrigger` | `func` @ wiki_ingest.go:560 | `knowledge_post_process.go:268,435`（K4） | 同上（导出 `EnqueueWikiIngestTrigger`） |
| `enqueueWikiRetract` | `func` @ wiki_ingest.go:608 | `knowledge_delete.go:233`（K4） | 同上 |
| `extractRealText` | `func` @ wiki_ingest.go:3167 | `knowledge_post_process.go:729`（K4） | 同上 |
| `newWikiIngestPendingOp` | `func` @ wiki_ingest.go:534 | `knowledge_post_process.go:317`（K4） | 同上 |
| `minTextContentRunes` | `var` @ wiki_ingest.go:2969 | `knowledge_process.go:844,847`（K4） | 同上（导出 `MinTextContentRunes`，var 别名同一值） |
| `realTextRuneCount` | `func` @ wiki_ingest.go:3182 | `knowledge_process.go:843,942`（K4） | 同上 |
| `uniqueWikiFolderIDs` | `func` @ wiki_ingest.go:780 | `knowledge_delete.go:236`（K4） | 同上 |
| `EnqueueWikiIngest`（已导出） | `func` @ wiki_ingest.go:502 | `knowledge_clone_move.go`（K4，同包裸调用，grep -l 实测唯一） | 宿主兼容文件 W1 一行转发 |
| `ErrWikiIngestConcurrent`（已导出） | `var` @ wiki_ingest.go:36 | `internal/router/task.go:124`（集成工程师文件） | 宿主 `var ErrWikiIngestConcurrent = wiki.ErrWikiIngestConcurrent`（同一实例，errors.Is 语义不变） |
| `WikiIngestPayload`（已导出） | `type` @ wiki_ingest.go:294 | `internal/container/recover_pending_wiki_tasks.go:75` | 宿主 `type WikiIngestPayload = wiki.WikiIngestPayload`（类型别名，JSON 字段不变） |

K3 方法反向耦合实测：**零**（K3 定义的方法无任何非 K3 生产文件以 `.name(` 形式调用；初扫 `tracker` 命中为各类型自有同名方法，假阳性）。

### 5.2 组 B——K3 消费、定义仍在宿主（R2：消费侧 seam，禁 import 宿主 service 包——W1 兼容文件在 service 包 import wiki，反向 import 即成环）

| 符号 | 定义（实测） | K3 调用点 | seam 落点 |
|---|---|---|---|
| `recordKBActivity` | kb_activity.go:95（K2） | knowledge_faq.go:252,475,667,853,1309；knowledge_faq_import.go:241,2237 | faq.Service 函数字段（签名照录 20 计划 §6.2 组 B） |
| `kbActivityTrigger` | kb_activity.go:36（K2） | knowledge_faq_import.go:245,2249 | 同上 |
| `withKBActivityTask` | kb_activity.go:27（K2） | knowledge_faq_import.go:2249 | 同上 |
| `kbActivityAppendSampleTitles` | kb_activity.go:49（K2） | knowledge_faq.go:1308 | 同上 |
| `resolveKBReadTenant` | knowledgebase_access.go:21（K2） | knowledge_faq.go:36 | 同上 |
| `writableFAQKnowledgeBase` | knowledgebase_access.go:38（K2 方法，`func (s *knowledgeService) writableFAQKnowledgeBase(ctx, kbID) (*types.KnowledgeBase, context.Context, error)`） | knowledge_faq*.go（2 文件，实测 s.X 引用表） | faq.Service 函数字段（生产接线：宿主委托构造闭包捕获 knowledgeService 实例） |
| `hash` | semantic_model_capability.go:147（K2） | knowledge_faq_import.go:1177,1221,1222,1223,1224 | 同上 |
| `resolveDeadSlug` | slug_fuzzy.go:91（K2） | wiki_ingest.go:1670；wiki_page.go:1191 | wiki 包函数字段/注入（两个接收者：wikiIngestService、wikiPageService） |
| `contains` | knowledge_span_tracker.go:702（K4） | wiki_ingest.go:2169 | wiki 包 seam |
| `finalizeSubtaskDetached` | knowledge.go:235（K4） | wiki_ingest.go:1172 | wiki 包 seam |
| `isLikelyRateLimitError` | knowledge_process.go:3902（K4） | wiki_ingest_batch.go:520,664 | wiki 包 seam |
| `removeSourceRef` | knowledge_delete.go:346（K4） | wiki_lint.go:409 | WikiLintService seam |
| `SpanTracker`（接口）、`noopSpanTracker` | knowledge_span_tracker.go:85 / :858（K4） | wiki_ingest.go:381,415,429,439 | wiki 包自定义窄接口（spec §4.2 接口优先定义在使用方；K4 `SpanTracker` 结构性满足）+ 本地 no-op 实现（Brief 登记，ib2 可与 K4 noopSpanTracker 收口） |

FAQ 域对 K4 定义的**方法**依赖实测为**零**：初扫嫌疑 `deleteFAQChunkVectors`/`executeFAQMergeOperations`/`incrementalIndexFAQEntry`/`recordFAQImportKBActivity`/`getRunningFAQImportTaskID`/`setRunningFAQImportInfo`/`clearRunningFAQImportInfoIfMatches`/`finalizeFAQValidation`/`buildFAQImportResultMessage` 逐一 grep 定位，定义全部在 knowledge_faq_import.go 自身（:2132/:2605/:1838/:2226/:1768/:1780/:1803/:2493/:2725），随文件同迁。

### 5.3 组 C——K3 内部（R3：同计划随迁直接改）

- `service/wiki_page.go` 对 `repository.ErrWikiPageNotFound/ErrWikiFolderNotFound/ErrWikiFolderNotEmpty/ErrWikiFolderConflict` 的引用（wiki_page.go service 侧 grep 实测 4 符号）：repository/wiki_page.go 同迁入 package wiki（`NewWikiPageRepository` @ repository/wiki_page.go:29、`wikiPageRepository` @ :24），哨兵变包内符号，调用点随迁直改；
- `span`（type @ wiki_linkify.go:20）、`wikiPromptWarmup` 等 wiki 域内部符号：初扫中 `knowledge_span_tracker.go`/`knowledge.go` 等文件的 `span` 命中经核对全部为方法参数名（实类型为大写 `Span`），无跨 owner 耦合；
- FAQ 五文件互调与对自有导出方法（`UpdateFAQEntryStatus` knowledge_faq.go:633、`UpdateFAQEntryTag` :861 等）的调用：同包随迁直改。

### 5.4 wiki_fixer_scope.go 去方法化（B0.3 Step 3 + conventions §7.4，节点裁定落地）

实测结构：`resolveWikiFixerTenantScope`（:18，`*Handler` 方法，仅把 `h.knowledgebaseService`（session/handler.go:30）、`h.kbShareService`（:34）传给包级函数）；`resolveBuiltinWikiFixerTenantScope`（:36，无 receiver，依赖以 `wikiFixerKBLookup` 接口（:12）与 `access.KBShareLookup`（:16）入参注入——**已是去方法化形态**）。处置：

1. 包级函数 + 两个类型 + wiki_fixer_scope_test.go（5 用例）整体迁入 `internal/modules/knowledge/wiki`，函数导出为 `ResolveBuiltinWikiFixerTenantScope`（签名不变）；
2. 宿主 `internal/handler/session/` 新增过渡文件（§6.2 S1）：保留 `func (h *Handler) resolveWikiFixerTenantScope(...)` 薄委托至 wiki 包——`qa.go:205`（conversation 属主）零改动编译；
3. 残差登记 Integration Brief：`qa.go:205` 调用点改写 + S1 删除 = ib2 集成工程师执行（10-identity.md:79 同型推迟先例）。

## 6. 文件结构

### 6.1 新包落位

```text
internal/modules/knowledge/wiki/     # package wiki（12 迁入文件 + 11 迁入测试）
  wiki_page_repository.go            # ← repository/wiki_page.go（git mv + package wiki；含 NewWikiPageRepository:29、4 哨兵）
  wiki_page.go                       # ← service/wiki_page.go（wikiPageService:44 实现冻结 WikiPageService）
  wiki_ingest.go                     # ← service/wiki_ingest.go（wikiIngestService、WikiIngestPayload:294、ErrWikiIngestConcurrent:36、NewWikiIngestService:403、EnqueueWikiIngest:502）
  wiki_ingest_batch.go / wiki_ingest_cite.go / wiki_ingest_dedup.go /
  wiki_ingest_taxonomy.go / wiki_linkify.go / wiki_lint.go（WikiLintService:56） / wiki_slug_handles.go
  wiki_fixer_scope.go                # ← handler/session/wiki_fixer_scope.go（导出 ResolveBuiltinWikiFixerTenantScope）
  wiki_page_handler.go               # ← handler/wiki_page.go（WikiPageHandler:19、NewWikiPageHandler:31）
  + 对应 14 个 _test.go；`seams.go`（W0，模块侧 seam 声明、**非** ib2 删除物）：窄接口 `wikiSpanTracker`（照录 wiki_ingest.go 对 `SpanTracker`（knowledge_span_tracker.go:85）的调用子集，K2/K4 实现结构性满足）+ 本地 no-op + `resolveDeadSlug`/`contains`/`finalizeSubtaskDetached`/`isLikelyRateLimitError`/`removeSourceRef` 函数字段类型——保 wiki 包不 import 宿主 service 包（防与 W1 反向成环，spec §4.2 接口优先定义在使用方）
internal/modules/knowledge/faq/      # package faq（6 迁入文件 + 2 迁入测试）
  service.go                         # 新增（F0，模块侧、非 ib2 删除物）：faq.Service 结构体（字段面见 K3.2 Step 2）+ NewService(deps Deps) 构造器；保 faq 包不 import 宿主 service 包（D1 在宿主侧构造注入）
  knowledge_faq.go / knowledge_faq_batch.go / knowledge_faq_create_guard.go /
  knowledge_faq_import.go / faq_clone_sync.go   # 接收者 (s *knowledgeService) → (s *Service)
  faq_handler.go                     # ← handler/faq.go（FAQHandler:24、NewFAQHandler:30）
  + knowledge_faq_create_guard_test.go、faq_enabled_filter_test.go
```

同包合并的命名冲突检查义务：K3.1 第一步 `git mv` 后 `go build ./internal/modules/knowledge/wiki/` 若报重声明，属机械同名（如 handler 文件与 service 文件的局部 helper），以「最小重命名 + 差分注记」就地消除，不得改逻辑。

### 6.2 宿主过渡兼容文件（本节点产出、同 commit 登记 manifest、删除点 ib2）

| id | 文件（新建） | 内容（零业务逻辑） | 保护的不改方 |
|---|---|---|---|
| W1 | `internal/application/service/wiki_k3_compat.go` | 包级转发：`PreviewText`/`EnqueueWikiIngestTrigger`/`EnqueueWikiRetract`/`ExtractRealText`/`NewWikiIngestPendingOp`/`MinTextContentRunes`/`RealTextRuneCount`/`UniqueWikiFolderIDs`/`EnqueueWikiIngest`/`ErrWikiIngestConcurrent`/`WikiIngestPayload`（var/别名/一行委托，文件头 `// Pass B (23-knowledge-wikifaq) 过渡 shim — 删除点 ib2`，先例 12-commercial.md:449） | §5.1 全部宿主调用点（K1/K4 文件、router/task.go、container/recover_pending_wiki_tasks.go） |
| W2 | `internal/application/service/wiki_k3_ctor_compat.go` | `func NewWikiPageService(...)`/`NewWikiIngestService(...)`/`NewWikiLintService(...)` 转发：以闭包把 §5.2 wiki 侧 seam（resolveDeadSlug/contains/finalizeSubtaskDetached/isLikelyRateLimitError/removeSourceRef/SpanTracker 实例）接到宿主现行符号后调 wiki 包构造器 | `container.go:431/432/433` dig Provide（集成工程师 ib2 切换） |
| H1 | `internal/handler/wiki_page_k3_compat.go` | `type WikiPageHandler struct { *wiki.WikiPageHandler; kbService interfaces.KnowledgeBaseService }`（嵌入保方法集提升 + 保留 `kbService` 字段）+ `func NewWikiPageHandler(...)` 转发 | `routes_knowledge.go:309`；**rbac_lookups.go:105/:183 与 rbac_lookups_test.go:327/344/356 字节不变**（:183 在本地 wrapper 类型上定义方法合法、:105 方法表达式经提升合法、字面量 `&WikiPageHandler{kbService:…}` 字段仍在——Go 规范：与提升方法同名显式方法为合法遮蔽） |
| H2 | `internal/handler/faq_k3_compat.go` | `type FAQHandler = faq.FAQHandler`（类型别名）+ `func NewFAQHandler(...)` 转发 | `routes_knowledge.go:143`、`container.go:720` |
| S1 | `internal/handler/session/wiki_fixer_scope_compat.go` | 保留 `func (h *Handler) resolveWikiFixerTenantScope(...)` 一行委托 `wiki.ResolveBuiltinWikiFixerTenantScope(...)` | `handler/session/qa.go:205`（conversation 属主） |
| D1 | `internal/application/service/knowledge_faq_k3_delegate.go` | `*knowledgeService` 上的委托方法集：(a) 冻结接口 FAQ 面 14 方法（§4 表）各一行委托 `s.faqSvc().X(...)`；(b) §5.1 之外被宿主他 owner 调用的 4 个未导出方法：`validateFAQKnowledgeBase`（knowledgebase_access.go:42 调用，K2）、`buildFAQStatusSyncPlan`（knowledge_clone_move.go:700）、`indexFAQChunks`（:855）、`syncFAQChunkStatusBatch`（:885）；(c) 私有构造 `faqSvc()`：以 `s.repo`/`s.chunkRepo`/`s.chunkService`/`s.tagRepo`/`s.tagService`/`s.tenantRepo`/`s.kbService`/`s.kbShareService`/`s.fileSvc`/`s.modelService`/`s.task`/`s.audit`/`s.redisClient`/`s.retrieveEngine`/`s.ownership`/`&s.memFAQProgress`/`&s.memFAQRunningImport`（实测 s.X 字段使用面，knowledge.go:49-88 字段清单）+ seam 闭包构造 `*faq.Service`——sync.Map 以指针共享，FAQ 导入进度状态跨调用不丢 | 冻结端口满足性（编译器强制）、K2/K4 四个调用点、router worker 注册 |

## 7. 任务（业务完整、可独立审阅；一任务一 commit，conventions §4）

### Task K3.1 — Wiki 域 12 文件迁入 internal/modules/knowledge/wiki（M2+M3）

**前置**：P1–P5 全绿；基线对齐 merge commit 已入库。

- [ ] **Step 1（T0 基线）**：`go test -count=1 ./internal/application/repository/ -run 'Wiki' ./internal/application/service/ -run 'Wiki|Linkify|Slug'` 不可行时按包全跑：记录 §3.1 wiki 侧 11 个测试文件的通过集合（逐文件 `-run` 输出摘录进 evidence 草稿）。
- [ ] **Step 2（M2 纯移动）**：`git mv` 上表 #1–#12 及其 14 个 `_test.go` 到 `internal/modules/knowledge/wiki/`（handler/wiki_page.go 更名 wiki_page_handler.go 防与 service/wiki_page.go 同目录同名冲突；repository/wiki_page.go 更名 wiki_page_repository.go）；`package` 子句统一为 `wiki`；session/wiki_fixer_scope.go 包级函数改名导出 `ResolveBuiltinWikiFixerTenantScope`（调用点 :25 同步）。**纯移动验收（spec §14.2）**：`git diff --summary` 识别全部 rename；除 package/import/接收者包名外零函数体变化。
- [ ] **Step 3（M3 编译修复）**：落地 §6.1 W0 seam 声明（wiki 包窄接口 `wikiSpanTracker` + no-op + 函数字段：resolveDeadSlug/contains/finalizeSubtaskDetached/isLikelyRateLimitError/removeSourceRef）；落地 W1/W2/H1/S1 四个宿主兼容文件；`wiki_page_handler.go` 的 `*service.WikiLintService` 字段改包内 `*WikiLintService`、`service.ErrWikiRevertToCurrentVersion`（handler/wiki_page.go:670）改包内符号（定义随 wiki_page.go 同迁，实测于 service 侧 grep）；同 commit 执行 §3.2 的 manifest 行删除（12 行 wiki 侧）+ W0/W1/W2/H1/S1 新增登记行（H1 文件头注明 rbac_lookups.go:105/:183 兼容义务）。
- [ ] **Step 4（T1/T2 验证）**：

```bash
go build ./...
# 预期 exit 0（K1/K4 宿主调用点经 W1 编译通过；container/recover 经 W1 别名通过）
go test -count=1 ./internal/modules/knowledge/wiki/...
# 预期：14 个迁入测试全 PASS，与 Step 1 基线集合一致（零新增失败/零跳过）
go test -count=1 ./internal/handler/session/... ./internal/application/service/ -run 'Wiki'
# 预期：留驻宿主的 K4 测试（knowledge_post_process_wiki_enqueue_test.go、knowledge_move_wiki_test.go）PASS（经 W1/W2）
make verify-module-moves
# 预期 exit 0（13 行已删，磁盘已无对应文件）
```

- [ ] **Step 5（差分，conventions §6）**：wiki 侧双跑比对（用例清单、双跑输出、逐用例结论）写入 `docs/architecture/evidence/passb/b2-k-wikifaq.md` 差分章节草稿；`TypeWikiIngest`/`TypeWikiFinalize` 幂等断言（wiki_ingest_test.go、wiki_ingest_retry_test.go、wiki_ingest_dedup_test.go）逐用例等价。
- **产出**：上述文件 + manifest 行增删。**验收**：Step 4 四命令全绿；`git diff 6bda27b1d...HEAD -- internal/application/service/wiki_*.go internal/application/repository/wiki_page.go internal/handler/wiki_page.go` 仅剩删除。commit：`refactor(knowledge): K3 wiki 域 12 文件迁入 internal/modules/knowledge/wiki`。

### Task K3.2 — FAQ 域 6 文件迁入 internal/modules/knowledge/faq 与冻结端口委托（M2+M3，本计划最重任务）

**前置**：K3.1 审阅通过（同分支串行，任务间无文件交集：faq 六文件 vs wiki 十二文件，唯一共享面 = knowledge.yaml 行区互不重叠）。

- [ ] **Step 1（T0 基线）**：`go test -count=1 ./internal/application/service/ -run 'FAQ'` 与 `go test -count=1 ./internal/handler/ -run 'FAQ'`，记录 `knowledge_faq_create_guard_test.go`、`faq_enabled_filter_test.go` 及宿主其余 FAQ 面测试（作为跨 plan 差分锚点的 faq 相关用例）通过集合。
- [ ] **Step 2（faq.Service 接收者重构，先写后搬）**：新建 `internal/modules/knowledge/faq/service.go`：`type Service struct` 字段 = §6.2 D1(c) 列举的 17 个依赖字段（接口/具型照录 knowledge.go:49-88 对应字段类型：`repo interfaces.KnowledgeRepository`、`chunkRepo interfaces.ChunkRepository`、`chunkService interfaces.ChunkService`、`tagRepo interfaces.KnowledgeTagRepository`、`tagService interfaces.KnowledgeTagService`、`tenantRepo interfaces.TenantRepository`、`kbService interfaces.KnowledgeBaseService`、`kbShareService interfaces.KBShareService`、`fileSvc interfaces.FileService`、`modelService interfaces.ModelService`、`task interfaces.TaskEnqueuer`、`audit interfaces.AuditLogService`、`redisClient *redis.Client`、`retrieveEngine interfaces.RetrieveEngineRegistry`、`ownership retriever.TenantStoreOwnership`、`memFAQProgress *sync.Map`、`memFAQRunningImport *sync.Map`）+ seam 函数字段（§5.2 前 7 行：recordKBActivity 族 4 + resolveKBReadTenant + writableFAQKnowledgeBase + hash）；构造器 `NewService(deps Deps) *Service`。**TDD**：先在 faq 包写一条锚定测试（复用 knowledge_faq_create_guard_test.go 迁移前后同用例），RED→GREEN。
- [ ] **Step 3（M2 纯移动 + 接收者改写）**：`git mv` 上表 #13–#18 与 2 个 `_test.go` 入 package faq；全文件接收者 `(s *knowledgeService)` → `(s *Service)`（实测 57 处方法：knowledge_faq.go 24 + knowledge_faq_batch.go 3 + knowledge_faq_create_guard.go 1 + knowledge_faq_import.go 27 + faq_clone_sync.go 2，撰写时 `grep -c` 实测）；`s.writableFAQKnowledgeBase(...)` 调用点改 `s.seams.writableFAQKnowledgeBase(...)`；`resolveKBReadTenant`/`recordKBActivity` 族/`hash` 调用点改经 seam 字段；`repository.ErrKnowledgeBaseNotFound/ErrKnowledgeNotFound` 改 `import "github.com/Tencent/WeKnora/internal/application/repository"`（过渡 import，§10 处置待命）；handler/faq.go → `faq_handler.go`（FAQHandler 字段 `knowledgeService interfaces.KnowledgeService` 不变，调用的全部是接口方法——实测 :515 `GetFAQImportProgress`、:542 一带均接口面）。
- [ ] **Step 4（D1 委托文件）**：按 §6.2 D1 落地 `knowledge_faq_k3_delegate.go`（14 接口方法 + 4 未导出方法委托 + `faqSvc()` 构造）；`knowledgeService` 结构体零改动（不碰 K4 的 knowledge.go）。**编译器即验收**：接口 FAQ 面若有本表未列方法，编译错误枚举后按同样的一行委托补齐并把方法名补录进 §4 表（报告登记），**禁止**改接口本身。
- [ ] **Step 5（manifest 同步）**：同 commit 删除 faq 侧 6 条 legacy_files 行（service 5 + handler/faq.go）+ D1/H2 新增登记行（同 W1 文件头格式）。
- [ ] **Step 6（T1/T2/T3 验证）**：

```bash
go build ./...
# 预期 exit 0：knowledgeService 仍满足 interfaces.KnowledgeService（冻结端口）；
#   router/task.go:277、sync_task.go:148 经接口的 ProcessFAQImport 注册零变化
go vet ./internal/modules/knowledge/faq/... ./internal/application/service/
# 预期 exit 0（sync.Map 指针共享无复制告警）
go test -count=1 ./internal/modules/knowledge/faq/... ./internal/handler/ -run 'FAQ'
# 预期：与 Step 1 基线同集合 PASS
go test -count=1 ./internal/application/service/ -run 'FAQ|Knowledge'
# 预期：K2/K4 留驻测试（经 D1 委托四方法）PASS
go test -count=1 ./internal/modules/knowledge/kbfreeze/
# 预期 exit 0（R0 守卫：faq 包无影子类型）
```

- [ ] **Step 7（差分）**：`TypeFAQImport` 处理路径（executeFAQImport @ knowledge_faq_import.go:1373、dry-run、断点续跑 memFAQProgress 指针共享等价）双跑比对入 evidence 草稿。
- **验收**：Step 6 全绿；`git diff --summary` rename 识别；`grep -c "func (s \*knowledgeService)" internal/application/service/knowledge_faq*.go internal/application/service/faq_clone_sync.go` = 0。commit：`refactor(knowledge): K3 faq 域 6 文件迁入 internal/modules/knowledge/faq 并保持冻结端口委托`。

### Task K3.3 — Integration Brief、断链登记与节点报告

- [ ] 产出 `docs/architecture/passb/briefs/b2-k-wikifaq.md`（交付 b2-k-integration/ib2）：(a) 装配切换表——`container.go:311/431/432/433/720/859` 六个 Provide 逐行改指 wiki/faq 包；`routes_knowledge.go:143/309` 宿主类型改模块包；`recover_pending_wiki_tasks.go:75` `service.WikiIngestPayload`→`wiki.WikiIngestPayload`；`router/task.go:124` 哨兵改 wiki 包；`qa.go:205` 改调 `wiki.ResolveBuiltinWikiFixerTenantScope`；(b) §5.1/§5.2 全部宿主调用点改写清单（ib2 属主：集成工程师）；(c) W1/W2/H1/H2/S1/D1 六个兼容文件 + rbac_lookups.go:183 方法与 wrapper 的 ib2 收口说明；(d) faq 包 seam 生产接线表（seam → 定义方导出符号 → 落位包，K2/K4 合并后按 20 计划 §6.1 R1.3 shim 状态核对）；(e) 模块内同包合并时的就地重命名清单（K3.1 Step 3 实际发生项）。
- [ ] 写 `docs/plans/passb/reports/b2-k-wikifaq.md`：§1.2 命令包输出（base_sha 用派发回填值）、变更清单 vs owned_files 逐条核对、差集结论、未完成项如实列出。
- [ ] 核对并登记：brief 删除义务 2（docparser `IsImageFormat`/`IsSimpleFormat`/`SimpleFormatReader` 与 wiki/faq 摄取相关消费）实测结论——本节点 18 文件零消费（撰写时 grep 实测），无回收动作，结论写入 Brief 防后续误判。
- **验收**：三文件评审可读、无 TBD；Brief 覆盖 (a)–(e)。commit：`docs(passb): b2-k-wikifaq Integration Brief 与断链登记`。

### Task K3.4 — 高风险差分证据、节点门禁与收口

- [ ] evidence 终稿 `docs/architecture/evidence/passb/b2-k-wikifaq.md`：§8 差分表四行（Wiki 摄取/Finalize/FAQ 导入/hook 恢复）逐行给用例清单、双跑命令与输出、等价比对结论；T0 基线 vs T1/T2 终态对照。
- [ ] 执行 DAG gates 四项 + conventions §1.2 命令包：

```bash
PASSB_BASE_SHA=<派发回填 base_sha>
go build ./...                                   # 预期 exit 0
go test -count=1 ./internal/modules/knowledge/... # 预期 exit 0（含 kbfreeze 2 守卫 + wiki/faq 迁入测试）
make check-backend-architecture                   # 预期 exit 0
make verify-module-moves                          # 预期 exit 0
git diff --stat "$PASSB_BASE_SHA"...HEAD          # 逐条对照 owned_files
git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort   # 与 owned_files 求差集，非空即失败
```

- [ ] 计数奇偶自查（conventions §8）：`go run ./tools/architectureguard` 输出的路由/worker/hook 计数与 `docs/architecture/evidence/pass-a-acceptance.md` 台账一致（633/23+23/58）；本节点零路由/worker/钩子增删。
- [ ] 向协调者回报：建议 DAG b2-k-wikifaq 置 `review`、`task_ids=[K3.1,K3.2,K3.3,K3.4]`；附 §5 组 A/B 对 20 计划 §6 的修订差异（协调者回填 DAG notes，本节点不改 DAG）。
- **验收**：差集为空；四 gates 全绿且命令原文+退出码在报告；evidence/报告/Brief 三产物随分支提交。commit：`docs(passb): b2-k-wikifaq 差分证据与门禁收口`。

## 8. 测试与高风险差分要求

**复用现有（全部随迁，contracts.yaml characterization 登记项加粗）**：wiki 侧 11 文件（§3.1）——**wiki_ingest_dedup_test.go**、**wiki_page_revision_test.go**、**wiki_folder_prune_finalize_test.go**；faq 侧 2 文件——knowledge_faq_create_guard_test.go、faq_enabled_filter_test.go。新写仅 2 处：faq/service.go 构造器锚定测试（K3.2 Step 2 TDD）；wiki 包 seam 的 no-op tracker 行为锚定（可并入 wiki_ingest_test.go 迁移用例，不新增独立文件）。

**跨 plan 差分锚点（文件不迁移，双跑引用）**：agentruntime `wiki_link_mutation_test.go`/`wiki_replace_text_test.go`/`wiki_tools_test.go`/`wiki_write_page_test.go`（agentruntime 属主，B3 迁移，contracts.yaml wiki-page-service characterization_tests 登记）、`datasource_service_test.go`（datasource 属主）、`knowledge_move_wiki_test.go` 与 `knowledge_post_process_wiki_enqueue_test.go`（K4 随迁面，本节点只保证经 W1/W2 宿主侧 PASS）。

**高风险差分（spec §14.3 Knowledge 索引/删除面 + framework:40；同输入双跑比对输出/错误/DB 结果/任务参数/副作用；证据入 evidence，conventions §6）：**

| 面 | 锚定用例（现状基线） | 等价断言 |
|---|---|---|
| `TypeWikiIngest`/`TypeWikiFinalize` 状态机 | wiki_ingest_test.go、wiki_ingest_retry_test.go（ErrWikiIngestConcurrent 指数退避重试语义，task.go:117-124 注释锚）、wiki_ingest_dedup_test.go、wiki_ingest_taxonomy_test.go | claim/peek/finalize 合并（`wiki-finalize-<kbID>` TaskID 去重）、5-docs-per-batch fan-out、MaxRetry 10 不变 |
| FAQ 导入 `TypeFAQImport` | knowledge_faq_create_guard_test.go（创建守卫 acquireFAQCreateGuard）、faq_enabled_filter_test.go、executeFAQImport 双跑 | dry-run 零写入、进度 memFAQProgress 跨调用共享（指针）、失败 CSV 生成一致 |
| `recoverPendingWikiTasks` 重启恢复 | 手工差分：构造 task_pending_ops 两行（active KB + deleted KB），容器启动断言 trigger 重建/清除行为（recover_pending_wiki_tasks.go:45-58 fail-closed 语义） | 重建数、`asynq.TaskID` 去重、Timeout 60/30min 逐项一致 |
| Wiki 页面/文件夹操作 | wiki_page_test.go、wiki_page_revision_test.go、wiki_folder_prune_finalize_test.go、wiki_slug_handles_test.go、wiki_linkify_test.go | 冻结 WikiPageService 方法集输出逐用例一致；4 哨兵错误标识不变 |

差分失败只修新实现，不得改期望值迎合（spec §14.3）；legacy 删除（本节点=manifest 行）前差分必须已通过（framework:31）。

## 9. 集成与回滚边界

- **集成边界**：本节点分支 `codex/passb-b2-k-wikifaq` 只做基线对齐 merge（P2）+ 自身任务 commit；集成分支合并按 20 计划 §3 缺省序 K1→K2→K3→K4 一次一支、审后合并（framework:28）；装配切换（router/container/recover/task/sync_task/qa.go 调用点）仅集成工程师按 K3.3 Brief 在 K5/ib2 串行执行；`ownership-matrix.yaml` 18 行与 6 兼容文件删除在 ib2（delete_barrier 口径）；K2/K4 合并后其 R1.3 宿主 shim 使 D1/W1/W2 的宿主符号引用持续有效（链路：宿主委托→宿主 shim→K2 落位包）。
- **回滚边界**（spec §13）：K3.1/K3.2 为 M2+M3 混合 commit——回滚 = revert 对应 commit，宿主恢复原文件，无装配影响；兼容文件随搬迁 commit 生灭，不存在半态；本节点无 schema/migration/路由/worker 计数变化，回滚不需数据修复与基线变更登记；K3.3/K3.4 为 M1 类 docs commit，revert 无代码影响。
- **升级边界**：同包合并重声明无法机械消除、冻结接口出现 §4 表外 FAQ 方法、guard 对 faq→repository import 报不可登记诊断时，按 conventions §5 停手上报（报告 + DAG notes 建议），禁止现场改判所有权、删测试或扩大例外。

## 10. 必须删除的 legacy/alias/例外（本节点口径）

1. **manifest legacy_files 18 行**（knowledge.yaml）：随 K3.1（13 行）/K3.2（5 行）物理迁移 commit 同 commit 删除（Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP；`modulemove verify` 对「行在文件无」退出 1，10-identity.md:37 实测先例）。
2. **新增过渡兼容 6 文件的 6 行**（W1/W2/H1/H2/S1/D1）：同 commit 新增登记（passb_task: B-knowledge，reason 注明 ib2 删除）——它们是新增的待删项，不是永久物。
3. **alias_obligations 18 条**（knowledge.yaml:41-77）：全部属 Pass A retriever/docparser 等包别名，删除归 K5/ib2（20 计划 §7.5），本节点无动作。
4. **例外台账**：本节点无属主例外（exc-0088→21、exc-0089/0090/0091→22，exception-ledger 实测 grep）。唯一潜在新增例外：faq 包→`internal/application/repository` 哨兵 import 若被 architectureguard 诊断，按 Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY 以独立 commit 登记 file→package 精确豁免 + 同窗更新 exception-ledger（owner=23-knowledge-wikifaq，RemoveAt=ib2）+ 机械计数修正 + §8 基线登记；优先替代方案是改 seam 注入 `matchNotFound func(error) bool`（二选一，报告记录所选路径）。
5. **docparser 别名消费**：实测本节点 18 文件零消费 `IsImageFormat`/`IsSimpleFormat`/`SimpleFormatReader`（grep 输出空），brief 删除义务 2 对本节点=登记核实结论，无回收动作。

## 11. 独立验收标准

1. DAG b2-k-wikifaq 四项 gates 全绿，命令原文+退出码记录于 `docs/plans/passb/reports/b2-k-wikifaq.md`；
2. `git diff --name-only "$PASSB_BASE_SHA"...HEAD | sort` 与 §3.2 可写清单差集为空；
3. 18 个 legacy 文件在宿主路径零残留（`git ls-files internal/application/service/wiki_*.go internal/application/service/knowledge_faq*.go internal/application/service/faq_clone_sync.go internal/application/repository/wiki_page.go internal/handler/faq.go internal/handler/wiki_page.go internal/handler/session/wiki_fixer_scope.go` 输出为空），destination 两侧文件数 = 12+6（+1 service.go/+若干 seam 声明）；
4. `go test -count=1 ./internal/modules/knowledge/...` PASS 且含 kbfreeze 2 守卫；wiki/faq 迁入测试集合与 T0 基线一致（零失败、零新增 Skip）；
5. 冻结端口满足性：`go build ./...` 即证明 `knowledgeService` 仍实现 `interfaces.KnowledgeService`（接口零改动：`git diff --name-only` 不含 `internal/types/`）；
6. `internal/handler/rbac_lookups.go` 与 `internal/handler/session/qa.go` 字节不变（`git diff --name-only` 不含）；
7. 高风险差分四行证据齐全（用例清单、双跑输出、比对结论、命令+退出码）；
8. 六个宿主兼容文件全部登记进 knowledge.yaml 且文件头注 Ruling/删除点 ib2；manifest 18 行删除与搬迁同 commit（`git log --stat` 核对）；
9. 治理文件（ownership-matrix/contracts/event-catalog）零改动（`git diff --name-only` 验证）；
10. 计划评审 approved（reviewer 独立出 `docs/plans/passb/reviews/b2-k-wikifaq.md`）。

## 12. 计划自检记录（撰写时执行）

- **Spec 覆盖**：ask 十要素 → §1（Spec 指针）、§2（前置与 BLOCKED 解除实证）、§3（精确文件与写权）、§4–§6（真实接口签名/行为兼容/文件结构）、§7（步骤+命令+预期）、§9（集成与回滚）、§10（删除义务）、§11（验收）——全覆盖。
- **无占位符**：全部符号含 file:line 与实测签名（previewText:1383、enqueueWikiIngestTrigger:560、newWikiIngestPendingOp:534、extractRealText:3167、realTextRuneCount:3182、minTextContentRunes:2969、uniqueWikiFolderIDs:780、resolveDeadSlug slug_fuzzy.go:91、hash semantic_model_capability.go:147、SpanTracker knowledge_span_tracker.go:85、spanTracker/noopSpanTracker :152/:858 等，撰写时逐条 sed/grep 命中）；无 TBD；「编译器枚举补齐」（K3.2 Step 4）是确定性验收机制而非未决设计，§4 已给 14 方法实测清单。
- **类型一致性**：`faq.Service` 字段类型逐一照录 knowledge.go:49-88 实测字段声明；`faqStatusSyncPlan` 返回类型经 D1(b) 委托在宿主保持可见性（字段 Pairs/SrcByID/DstByID 导出，faq_clone_sync.go:14-18 实测）；sync.Map 以 `*sync.Map` 共享（go vet 把关）。
- **跨任务/跨计划接口一致性**：§5 与 20 计划 §6.1 R1/R2/R3、§9 K3 义务行逐条对应；与 20 计划的**两处实测差异**（组 A 新增 8 符号 + previewText 5 个 K4 调用点；组 B 新增 8 项）已按节点裁定标注上报路径，不影响 K1/K2/K4 的义务面（它们的义务表不变，本表只是把 K3 自身义务补全）；H1 对 rbac_lookups.go:105/:183 的兼容性按 Go 方法集规则论证（显式方法合法遮蔽提升方法、方法表达式含提升方法）并在 K3.1 Step 4 以宿主测试实跑验证。
- **对 20 计划 §9 K3 行的一处勘误登记**：该行「组 B 的 R2 seam（recordKBActivity 族 13 调用点）」按本计划实测为**裸符号调用点 24 处**（recordKBActivity 7 + kbActivityTrigger 2 + withKBActivityTask 1 + kbActivityAppendSampleTitles 1 + resolveKBReadTenant 1 + hash 5 + wiki 侧 resolveDeadSlug 2 + contains 1 + finalizeSubtaskDetached 1 + isLikelyRateLimitError 2 + removeSourceRef 1），另有 `writableFAQKnowledgeBase` 方法调用 2 文件与 `SpanTracker`/`noopSpanTracker` 类型引用 4 处——差异不改变裁定方向，报告上报协调者随 §5 修订一并记 DAG notes。
- **不可行/未做项如实登记**：基线对齐 merge 未执行（依赖协调者派发回填 base_sha，P2 待开工核验）；`go test` 各命令为计划预期（本计划撰写为 docs 变更，未在主工作树跑全量搬迁后测试——T0/T1/T2 的实测义务在任务步骤内）。
