# Pass B B2 — 24-knowledge-process（K4 Knowledge 处理流水线/状态机域 28 legacy 文件归位）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**节点：** `b2-k-process`（DAG `docs/plans/passb/execution-dag.json`，phase B2，role work，execution_mode serial，`depends_on: [b2-k-ingest, b2-k-retrieval]`，gates=`go build ./...` / `go test ./internal/modules/knowledge/... -count=1` / `make check-backend-architecture` / `make verify-module-moves`）。本文件只承载 K4 本节点任务 K4.0–K4.5；同程序 K0/K5 任务节属 `b2-k0`/`b2-k-integration`，载体为 `20-knowledge-program.md`（本计划只消费其 §5–§7 冻结表与 §9 K4 行义务，不复述、不改写）。

**Goal:** 把 knowledge 域处理流水线子集 28 个 legacy 文件（ownership-matrix `plan: 24-knowledge-process` 全部 28 行）中**已验证自包含的 16 个**（repository 4 + service 独立面 8 + handler 4）从横向宿主包迁入 `internal/modules/knowledge/process` 三层子包；**knowledgeService 核心批 11 个 service 文件与 `repository/knowledgebase.go` 共 12 个按 Ruling 2026-09-25-DEFERRED-FILE-SPLIT 推迟**（根因与解除编排见 §3.3，先例=K2 对 `knowledgebase.go` 族的推迟——他属主宿主白盒测试未随各属主处置）；本批交付 `escapeLikeKeyword`、write-family、`buildKnowledgeIndexContent`、`ParseCommaSeparatedTagIDs`/`RequireTaskProgressTenant` 导出与 K1/K2/K3 已落位消费面的直连；worker 注册行零改动（K4 面经 `TypeKnowledgePostProcess`/`TypeKnowledgeAutoTag` 的独立类型 dig.Name 分发与经接口留守实现两部分均不触注册行）；全部节点门禁绿色并产出差分证据与 Integration Brief（含推迟批解除窗口的完整执行蓝本 §5.3）。

**Architecture:** K4 在 K1+K2 契约稳定后串行执行（framework:118-119）；本节点不实现 `internal/modules/knowledge/module.go` 门面接线（归 K5/ib2，conventions §3）；worker 注册行（`internal/router/task.go:266-320`、`sync_task.go:143-163`）禁改（与 conversation-queryhistory.md:39-42 义务 3 同律，本会话实测 task.go:269-320、sync_task.go:145-163：K4 面 11 个 worker 中 9 个经 `params.KnowledgeService`（interfaces）分发——实现体随推迟批留守，零变化；`TypeKnowledgePostProcess`/`TypeKnowledgeAutoTag` 经 `params.KnowledgePostProcess`/`KnowledgeAutoTag`（`interfaces.TaskHandler`，dig.Name `knowledgePostProcess`/`knowledgeAutoTag` 消歧，container.go:412/:413 Provide 经宿主 compat var 别名）——实现体随本批迁移，注册行零改动）。

**Tech Stack:** Go 1.26、Gin、GORM、dig、asynq、cron/v3、redis、Testify、SQLite 内存测试；门禁工具 `tools/modulemove` / `tools/architectureguard` / `tools/passbguard`。

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`（§4.2 依赖方向、§4.3 模块装配、§5.2 Knowledge 所有权、§11 Pass B 串并图、§12 Pass B 循环、§13 提交隔离 M1–M5 与回滚、§14.1–14.3 测试梯度与差分门禁、§15 治理规则、§16 停止条件、§17.2 完成标准）。

---

## 1. Spec 与事实源指针（全部只读输入）

| 事实源 | 用途 |
|---|---|
| `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` | 行为边界、M2/M3 提交隔离与回滚（§13）、差分门禁（§14）、完成标准（§17.2） |
| `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md`（:25-41 全局约束、:115-133 Knowledge mini-program、:118-119 K4 串行依据） | manifest=文件所有权唯一事实源（:25）、随迁 `_test.go`（:29）、禁双写/复制（:30）、先差分后删 legacy（:31） |
| `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | 派发契约 §1、门禁 §2、禁改 §3、提交 §4、升级 §5、差分 §6、package-private §7、计数基线 §8、DAG 回填 §9、裁定族 §10（Ruling LEGACY-ROW-OWNERSHIP / IMPORT-EXCEPTION-REGISTRY / TEST-SUPPORT-SHIM / WAVE-DEP-BASELINE / CYCLE-FORCED-COMPOSITION / DEFERRED-FILE-SPLIT / TRANSITION-SHIM-ROW-REGISTRATION） |
| `docs/plans/passb/20-knowledge-program.md`（b2-k0 冻结产出，`5bcb798621` 起在各前置分支在场） | §5 R0 类型单一事实源、§6.1 R1/R2/R3 通用裁定、§6.2 组 B/C/E 联动符号签名、§7 路由/worker/hook/事件/别名/例外指针、§9 K4 行义务、§10 差分表 K4 行、§11 集成与回滚 |
| B0 冻结产物（`.worktrees/passb-int/docs/architecture/passb/`）：`ownership-matrix.yaml`（28 行 plan=24，destination 全部 `internal/modules/knowledge/process`，integration_owner=ib2、delete_barrier=ib2，实测 python3 解析）、`contracts.yaml`（`knowledge.service`:1773 全签名 frozen、`knowledge.chunk-service`:1576、`knowledge.workers`:1893 18 项、`knowledge.facade`:1613、`knowledge.routes`:1750）、`event-catalog.yaml`（knowledge 家族 4 事件 v1；K4 域 3 producer 中 `knowledge.processing.failed`（producer `knowledge_housekeeping.go:runSweep`:112，随本批迁移）与 `knowledge.processing.completed`/`deletion.completed`（producer `repository/knowledge.go:707/:906`，随推迟批留守，零变化））、`exception-ledger.yaml`（实测零 plan=24 属主行）、`b0-evidence.md:86`（repository/knowledge.go 显式归 24） | 签名冻结、事件口径、destination、例外 |
| `docs/architecture/passb/knowledge-process.md`（集成 worktree 版，含 B0.2/B0.3 注记） | 域 brief：scope 28 文件、边界目标、删除义务 3 条、`KnowledgeHousekeeping` 端口裁定 |
| 先行节点计划与 Brief（各分支 approved 产物，本节点前置输入）：`21-knowledge-ingest.md` + `briefs/b2-k-ingest.md`（R1-6/7/9 shim、`span_trace_seam_adapter.go` 接线义务）、`22-knowledge-retrieval.md` + `briefs/b2-k-retrieval.md`（§2 导出表、§3.3 compat 删除编排——**`writableFAQKnowledgeBase` 方法体须随 K4 knowledge.go 补迁或由 K4 属主接收**（:78）、§4 K4 调用点直连表（:93）、§5 推迟件 3-10 解除条件含 K4 迁移 knowledge_delete.go（:104）、§6 escapeLikeKeyword seam 收口（:117））、`23-knowledge-wikifaq.md` + `briefs/b2-k-wikifaq.md`（(a) A1-A11 装配切换表、(b) W1/W2 宿主调用点表与 K3 测试垫片行（:57）、(c) D1 行（:78）、(d) faq/wiki seam 接线表） | K4 义务的授权事实源与接口 |
| 实测代码（本 worktree `codex/passb-b2-k-process` @ `37eae710f`（=集成分支头，K1/K2/K3 未合入）；基线对齐后本批 16 文件本体行号不变——前置分支只动自己文件与新增 compat，实测 K1/K2 分支 container.go diff 0 行、K3 分支仅 :93/:316 两处（Ruling 5 提前落地）） | 本计划全部签名与调用点行号出处（见 §5–§7 各表，撰写时逐条 grep/sed 实证） |

注意：主 checkout 的 `docs/architecture/passb/knowledge-process.md` 为 B0 前旧版，以集成 worktree 版为准（20 计划 §1 同律）。

## 2. 节点裁定与前置条件

**节点裁定（调度方原文，照录）：**

1. K4 在 ingest/retrieval 契约稳定后串行执行（framework:118-119）；worker 注册行禁改（与 conversation-queryhistory.md:39-42 义务 3 同律，本会话验证）。
2. BLOCKED（2026-09-23）：前置 b0 阻塞；解除条件：修复并 done b0 后恢复。BLOCKED（2026-09-24）：前置 b2-k0 阻塞；解除条件：修复并 done b2-k0 后恢复。BLOCKED（2026-09-25）：前置 b2-k-retrieval 阻塞；解除条件：修复并 done b2-k-retrieval 后恢复。

**BLOCKED 解除实证（撰写时实测，python3 读集成 worktree `execution-dag.json`）：** `b2-k-ingest` `status=done`+`review_status=approved`（head `20b9a7ca3`）、`b2-k-retrieval` `status=done`+`approved`（head `968d3d655`）；另 `b2-k-wikifaq` `status=done`+`approved`（head `ed156cd85`）、`b2-k0` done/approved（head `5bcb798621`）。三条 BLOCKED 记录对应前置均已闭环。

**前置条件核对（K4 开工门禁，逐条命令）：**

- [ ] **P1 冻结产物与前置分支在场**：`python3 -c "import json;d=json.load(open('docs/plans/passb/execution-dag.json'));print([(n['id'],n['status']) for n in d['nodes'] if n['id'] in ('b2-k0','b2-k-ingest','b2-k-retrieval','b2-k-wikifaq')])"` 预期全 `done`。
- [ ] **P2 基线对齐 merge**（Ruling 2026-09-24-WAVE-DEP-BASELINE）：按框架顺序以**独立 merge commits** 把 `codex/passb-b2-k-ingest` → `codex/passb-b2-k-retrieval` → `codex/passb-b2-k-wikifaq` 并入 `codex/passb-b2-k-process`（K1/K2 为 DAG 直接前置；**K3 一并纳入的依据**：framework:117-124 与 20 计划 §3 的集成分支合并缺省序 K1→K2→K3→K4，且 K3 Brief (c)/(d) 的收口目标与 K4 推迟批的解除编排互指——不合入 K3 则 K4 无法对 D1/W1/W2/H2 与 K3 垫片的现态采证，推迟件解除蓝本（§5.3）无从核对）。**边扩展登记义务（conventions §9）**：DAG `depends_on` 不含 `b2-k-wikifaq`，本次纳入属基线对齐边扩展——K4.0 在报告中登记并在回报时建议协调者在 DAG `b2-k-process` 的 `notes`/`corrections` 记录「基线对齐纳入 K3（框架合并序 + K3 Brief 收口依赖，非 DAG 前置变更）」；本节点不修改 `execution-dag.json`。对齐后 `go build ./...`、`go test -count=1 ./internal/modules/knowledge/kbfreeze/` 绿；`git status` 干净再开工。module.go 冲突=两侧注册全保留；涉冻结签名取舍的冲突停下升级（conventions §5）。合并冲突一般处置：同一文件两侧改动时以「两侧意图并存」为方向逐处裁定并留痕，无法机械并存的停下升级。
- [ ] **P3 门禁工具可用**：`Makefile` 的 `verify-module-moves` / `check-backend-architecture` 目标存在（实测 grep）。
- [ ] **P4 K1/K2/K3 落位包与宿主过渡物在场**（基线对齐后核验）：`ls internal/modules/knowledge/{ingest,retrieval/app,wiki,faq,kbfreeze}` 存在；宿主 `internal/application/service/` 下 `wiki_k3_compat.go`、`wiki_k3_ctor_compat.go`、`knowledge_faq_k3_delegate.go`、`kbretrieval_passb_compat.go`、`span_trace_seam_adapter.go`、`chunk_ingest_shim.go`（K1）、`knowledge_faq_k3_test_support_shim_test.go`（K3 垫片）、`kb_activity_test.go`（实测 K2/K3 分支 `git ls-tree` 均在场——K2 只迁走生产文件 `kb_activity.go`，其测试留守且为垫片 2 个纯函数的活消费者 :68/:79）存在；`internal/handler/` 下 `faq_k3_compat.go`、`wiki_page_k3_compat.go` 存在；`internal/application/repository/` 下 `kbretrieval_passb_compat.go`、`kbretrieval_passb_compat_test.go`、`wiki_k3_repo_compat.go` 存在。
- [ ] **P5 特征化测试基线（T0，spec §14.1）**：搬迁前跑 §8 随迁测试所在宿主包并记录通过集合与计数（K4.0 Step 3）。

## 3. 范围、目标布局与推迟件

### 3.1 本节点 28 文件（ownership-matrix plan=24-knowledge-process 全量，与 knowledge-process.md:7-28 逐行一致；本 worktree `ls` 实测全部在场，行数附后）

| 层 | 文件（`internal/application/` 前缀省略；行数为撰写时实测） | 批次 |
|---|---|---|
| repository 5 | `knowledge.go`（1169）、`knowledge_span_repo.go`（271）、`knowledge_tag.go`（159）、`knowledge_transfer.go`（74）、`knowledgebase.go`（253） | 前 4 迁移；`knowledgebase.go` 推迟 |
| service 19 | **独立面批 8（迁移）**：`knowledge_auto_tag.go`（380）、`knowledge_housekeeping.go`（399）、`knowledge_index_content.go`（21）、`knowledge_post_process.go`（730）、`knowledge_process_config.go`（328）、`knowledge_span_tracker.go`（879）、`knowledge_task_options.go`（27）、`knowledge_write.go`（141）——实测对 `knowledgeService` 类型零引用（逐文件 grep）；**核心批 11（推迟）**：`knowledge.go`（1129，**knowledgeService 定义 :49**）、`knowledge_process.go`（4091）、`knowledge_create.go`（1419）、`knowledge_delete.go`（777）、`knowledge_delete_plan.go`（169）、`knowledge_clone_move.go`（1572）、`knowledge_transfer.go`（530）、`knowledge_replace.go`（252）、`knowledge_reparse_scope.go`（60）、`knowledge_summary_refresh.go`（151）、`knowledge_util.go`（521）——实测挂 `*knowledgeService` 方法（`grep -l "func (s \*knowledgeService)"`） | 8 迁移；11 推迟 |
| handler 4 | `kb_access.go`（81）、`knowledge.go`（2770，**KnowledgeHandler 定义 :30**）、`knowledge_download.go`（361，`*KnowledgeHandler` 方法扩展文件，:59 BatchDownloadKnowledge）、`task_progress_auth.go`（27） | 4 迁移 |

**批次判定依据（实测）**：独立面 8 文件对 `knowledgeService` 零引用；核心批 11 文件必须与类型定义同进退（Go 方法集规则：同接收者方法必须同包）。

### 3.2 目标布局（matrix destination `internal/modules/knowledge/process` 的三层具体化，K2 计划 §3.2 先例）

同名文件不可同目录（repository/knowledge.go、service/knowledge.go、handler/knowledge.go 三者；推迟批的 service/knowledge.go 不在本批迁移，但布局为 28 行 destination 统一预留），按宿主同构分三个子包，落位包名对齐宿主便于 diff 审查：

| 宿主包 | 落位包（import path，package 名） | 本批文件 |
|---|---|---|
| `internal/application/repository` | `internal/modules/knowledge/process/repository`（package repository） | 4 文件 |
| `internal/application/service` | `internal/modules/knowledge/process`（package process） | 8 文件 |
| `internal/handler` | `internal/modules/knowledge/process/handler`（package handler） | 4 文件 |

`git mv` 保留原文件名；`git diff --summary --find-renames` 须逐文件 R100。布局具体化在 K4.1 首个搬迁 commit 落定并写入 Integration Brief 与报告（matrix destination 词干不变）。

### 3.3 推迟件（12 个；Ruling 2026-09-25-DEFERRED-FILE-SPLIT，先例=22 计划 §3.3 两推迟件 + K2 实际执行时按同 Ruling 收缩为 13 个推迟件、评审 approved；**根因类 A 与 K2 推迟件 3-10（`knowledgebase.go` 族）的「他属主宿主白盒测试未处置」完全同型**——K2 Brief §5 :104 原文「他属主宿主白盒测试（craft×2、resource_review、datasource_delete_sqlite、semantic_scope_mutation 等）随各属主处置」）

| # | 推迟文件 | 根因（base 实测证据） | 解除编排（写入 Integration Brief；§5.3 为解除窗口的执行蓝本） |
|---|---|---|---|
| 1 | `internal/application/repository/knowledgebase.go`（253 行） | 与 `repository/model_usage.go`（12-commercial 属主，ib1 零实施未迁移，K0 差异①）**单向未导出消费**：`CountModelUsages`（:216 经 `scopeKnowledgeBasesByModelID`）、`ListModelUsages`（:235 同上；:242 经 `knowledgeBaseModelUsageBindings`）——两符号定义于宿主 `model_usage.go:8/:57` 且未导出；K4 不得自行导出对方文件符号（20 计划组 E 裁定）。推迟后其余 4 个 repository 文件零断链（实测 grep 四文件对 `knowledgeBaseRepository`/`ErrKnowledgeBaseNotFound`/model_usage 族零引用）；哨兵 `ErrKnowledgeBaseNotFound`（:13）留守保护 wiki 闭包（K3 Brief (d)）、`handler/knowledge.go:2349/:2368`、faq 包哨兵 import 三处消费面零改动 | ib2 与 12-commercial 补迁同窗：commercial 侧落位导出窄端口后按 K4.1 同法迁移 |
| 2-12 | service 核心批 11 文件：`knowledge.go`（含 knowledgeService 定义 :49、`attemptSuperseded`:202、`finalizeSubtaskDetachedTimeout`:211、`finalizeSubtaskDetached`:235）、`knowledge_process.go`、`knowledge_create.go`、`knowledge_delete.go`、`knowledge_delete_plan.go`（`withKnowledgeCleanup`:22、`deleteReferencedKnowledge`:36）、`knowledge_clone_move.go`（`moveOneKnowledge`:1194）、`knowledge_transfer.go`、`knowledge_replace.go`、`knowledge_reparse_scope.go`、`knowledge_summary_refresh.go`（`enqueueSummaryRefresh`:83）、`knowledge_util.go`（`isValidFileType`:62、`getFileType`:89、`normalizeFileExtension`:47、`isDataTableFileType`:67） | **根因类 A（决定性）**：7 个他属主宿主白盒测试构造 `&knowledgeService{}`（实测 `grep -ln "&knowledgeService{" internal/application/service/*_test.go` 共 25 文件：17 个 K4 属主可随迁、1 个（knowledge_faq_create_guard_test.go）随 K3 已迁走、**7 个他属主无法处置**：`craft_knowledge_test.go`（:114/:460/:512）、`craft_knowledge_tool_test.go`（:99）→ 41-craft；`datasource_purge_test.go`（:225）→ 26-datasource/ib2；`knowledge_shared_access_test.go`、`knowledge_shared_storage_failure_test.go`、`semantic_scope_mutation_test.go` → K2 推迟件补迁窗（K2 Brief §5 已排）；`knowledge_caller_scope_test.go`（:153，K1 Brief 判「被测 knowledgeService，K4」留宿主）→ K4 补迁时随迁）。类型一走 7 文件断链且 K4 无权改（conventions §1.3）；**根因类 B（连带）**：宿主过渡物 K3 D1（19 方法+`faqSvc()`）、K2 compat `writableFAQKnowledgeBase` 段（:190/:194）、K3 测试垫片（4 方法消费者=`knowledge_write_access_test.go` 留守、2 纯函数消费者=`kb_activity_test.go:68/:79` 留守）全部挂 `*knowledgeService` | 解除条件=7 个白盒测试随各属主处置完毕；随后 K4 补迁（ib2 窗口或协调者指派）按 §5.3 蓝本整批执行。**本节点期间上述过渡物与 7 测试零触碰、天然编译** |

推迟不改变 12 文件的 ownership-matrix 属主（仍 24-knowledge-process）与 `delete_barrier: ib2`；manifest 行保留（Ruling 6 第 4 点：未迁移不删行）；推迟项在报告「未完成项」如实列出（conventions §1.2），不视为节点失败。**B5 的 396 口径不减免**。

## 4. 写入所有权与禁改清单

**可写**：

- §3.1 迁移批的 16 个物理搬迁文件及其随迁 `_test.go`（§8.1，判定规则+已确认清单；K4.0 落盘最终清单）；
- 三个落位新包及包内新文件（§5.2 导出包装、§5.4b 同形 seam/常量照录、R3 改名）；
- 3 个宿主兼容新文件 `internal/application/repository/kbprocess_passb_compat.go`、`internal/application/service/kbprocess_passb_compat.go`、`internal/handler/kbprocess_passb_compat.go`（**横向目录新生产文件必须同 commit 登记进 `docs/architecture/moves/knowledge.yaml` legacy_files（`passb_task: B-knowledge`，Ruling TRANSITION-SHIM-ROW-REGISTRATION 成对补行），否则 legacy-guard 报诊断——K2 计划 §4 实测先例**）；
- `docs/architecture/moves/knowledge.yaml`（仅本节点 16 行的删除 + compat 行登记，Ruling LEGACY-ROW-OWNERSHIP 行级删除权）+ `internal/modules/knowledge/legacy/README.md` 镜像同步；
- **K2 测试垫片 `internal/application/repository/kbretrieval_passb_compat_test.go` 的删除**（K2 Brief :80「knowledge_tag_test.go 随 K4 迁移或直连改写」为删除前置——knowledge_tag_test.go 本批随迁（实测其白盒构造 `knowledgeRepository`/`knowledgeTagRepository`（:217-243），不涉 knowledgeService），前置满足）；
- `tools/architectureguard/check.go` importExceptions 数据行 + `docs/architecture/passb/exception-ledger.yaml` 属主行（仅限 K4.4 按 Ruling IMPORT-EXCEPTION-REGISTRY 执行的精确登记）；
- 本节点产物：`docs/architecture/evidence/passb/b2-k-process.md`、`docs/plans/passb/reports/b2-k-process.md`、`docs/architecture/passb/briefs/b2-k-process.md`（conventions §1.1）。

**禁改**（conventions §3，违者节点失败）：`internal/router/router.go`、`internal/router/routes_knowledge.go`、`internal/router/task.go`、`internal/router/sync_task.go`、`internal/router/files.go`、`internal/container/**`（含 `recover_pending_wiki_tasks.go`）、`internal/bootstrap/**`、migration 编号、`go.mod`/`go.sum`、生产 SQL、既有迁移文件；`docs/architecture/passb/{ownership-matrix,contracts,event-catalog}.yaml`（barrier 回写）；`internal/modules/knowledge/module.go`（门面注释仅 b0 与集成节点可写）；`internal/types/**`（R0 唯一事实源）；`internal/modules/knowledge/{kbfreeze,ingest,retrieval,wiki,faq}/**`（K0/K1/K2/K3 产物）；K1 宿主产物 `span_trace_seam_adapter.go`、`chunk_ingest_shim.go`、`parser_url_security.go`（K4 只经宿主 compat 保持其编译，改写归 ib2）；**推迟批 12 文件与全部挂 `*knowledgeService` 的宿主过渡物（K3 D1 `knowledge_faq_k3_delegate.go`、K3 垫片 `knowledge_faq_k3_test_support_shim_test.go`、K2 compat `kbretrieval_passb_compat.go`——含其 `writableFAQKnowledgeBase` 段与 `semanticScopeGuard` 段、`kb_activity_test.go`）零触碰**；K2 推迟件（`semantic_model.go`、`handler/knowledgebase.go`、knowledgebase_search 族 8 文件、`kbshare.go`、`tag.go`、`tag_access.go`、`repository/kbshare.go`）；7 个他属主白盒测试（craft×2、datasource_purge、shared_access×2、semantic_scope_mutation、caller_scope）；conversation/identity/insights/datasource/airesource 全部属主文件；`cmd/desktop`、`docreader`、`client`。

**搬迁机械律**：每个生产文件同 commit 删除 manifest `legacy_files` 对应行（modulemove 对「行在盘缺」报 `legacy-file: not found` 退 1，10-identity.md:37 实测）；函数体一行不改（M2 纯移动），导出改名/兼容层重线独立于纯移动说明但可同 commit 呈现（M3，22 计划先例）；禁止混合关注点（conventions §4）。

## 5. 耦合面裁定（核心机制；全部实测，签名与 20 计划 §6.2 冻结表一致处不重述）

### 5.1 R1 导出义务一：本批符号（K4 定义、宿主他 owner 消费 → 落位包导出改名 + 宿主 compat 一行委托）

| 符号（定义 file:line，实测） | 签名（实测） | 留守消费方（grep 实证） | 导出名 |
|---|---|---|---|
| `escapeLikeKeyword`（repository/knowledge.go:22） | `func escapeLikeKeyword(keyword string) string` | identity `repository/tenant.go:85/:90`；conversation `repository/message.go:215`、`repository/session.go:209`；K2 落位包 `retrieval/app/repository/escape_like_seam.go` 同形副本（ib2 收口为单一实现，K2 Brief §6 :117） | `EscapeLikeKeyword` |

**顺延符号（定义于推迟批，本节点零动作——留守原位即编译，现状消费方无变化）**：`withKnowledgeCleanup`（knowledge_delete_plan.go:22；消费 datasource_service.go:885）、`deleteReferencedKnowledge`（:36；消费 conversation message.go:449/:469、session.go:687/:764/:821、web_search_state.go:124、insights evaluation.go:366、K4 属主留守测试 document_write_access_test.go:525/:530；**airesource 调用点实测为零**（全仓 `grep -rn deleteReferencedKnowledge internal/` 无 airesource 命中）——DAG ppc `airesources→knowledge 1 site` 为符号级扫描伪影（与 K0.1 已消歧的 `getParserEngineOverridesFromContext` 同型），K4.0 上报协调者修订 ppc）、`attemptSuperseded`（knowledge.go:202；K1 adapter 消费）、`finalizeSubtaskDetached`（:235；K3 W2 闭包消费）、`isLikelyRateLimitError`（knowledge_process.go:3902；K3 W2）、`removeSourceRef`（knowledge_delete.go:346；K3 W2）、`getFileType`/`normalizeFileExtension`/`isDataTableFileType`（knowledge_util.go:89/:47/:67；K1 R1-9 shim 以函数值消费——**本批 process_config 落位后经 §5.4b seam 消费同一宿主实现**，K1 shim 链路不变）、`enqueueSummaryRefresh`（knowledge_summary_refresh.go:83；K1 adapter 消费）、`isValidFileType`（knowledge_util.go:62；无跨 owner 调用方，K0.1 消歧）。全部登记 Integration Brief「推迟批解除窗口执行」。

### 5.2 R1 导出义务二：本批承载的 K1/K2/K3 已落位消费面导出

| 符号（定义 file:line，实测） | 消费方（落位包/宿主过渡物） | 导出名 |
|---|---|---|
| write-family 4 函数：`writeResourceIDs`（knowledge_write.go:17）、`writeExecutionTenant`（:32）、`loadKnowledgeWrite`（:60）、`loadKnowledgeWriteBatch`（:93） | K1 `ingest.KnowledgeWriteGuard` seam（21 计划 §6.3 表；宿主 `span_trace_seam_adapter.go` 的 `KnowledgeWriteGuardProvider()`，文件头注释明文「K4 搬迁 … 后指到其导出包装」） | `WriteResourceIDs` / `WriteExecutionTenant` / `LoadKnowledgeWrite` / `LoadKnowledgeWriteBatch` |
| `buildKnowledgeIndexContent`（knowledge_index_content.go:12，包级 `func buildKnowledgeIndexContent(knowledge *types.Knowledge, content string) string`） | K1 adapter 同文件头点名 | `BuildKnowledgeIndexContent` |
| `parseCommaSeparatedTagIDs`（handler/knowledge.go:2546，包级） | K3 `faq_k3_compat.go`（H2）构造参 seam（K3 Brief (d)「process 落位导出」） | `ParseCommaSeparatedTagIDs` |
| `requireTaskProgressTenant`（handler/task_progress_auth.go:14，包级 `func requireTaskProgressTenant(ctx context.Context, taskID string) error`） | 同上（K3 Brief (d)「最小动作 = faq_handler 构造参保留直引」→ 直引前提=宿主 compat 委托在位） | `RequireTaskProgressTenant` |
| 已导出符号（无需改名）：`NewKnowledgePostProcessService`（knowledge_post_process.go:34）、`NewKnowledgeAutoTagService`（knowledge_auto_tag.go:49）、`NewHousekeepingService`（knowledge_housekeeping.go:57）、`NewKnowledgeRepository`（repository/knowledge.go:51）、`NewKnowledgeSpanRepository`（knowledge_span_repo.go:54）、`SpanTracker`/`NewSpanTracker`（knowledge_span_tracker.go:85/:172）、`ErrKnowledgeNotFound`（repository/knowledge.go:15）等 | container.go:202-204/:374/:412/:413/:641、K1 adapter、K2 垫片、K3 W2、router（经接口） | 落位包原名；**宿主 compat 留 var/type 别名保 container 与 K1/K3 过渡物编译**（K2 §5.5 模式；K3 container.go:316 提前落地先例证明 K4 无需动 container） |

### 5.3 推迟批解除窗口执行蓝本（本节点不执行；完整保留供 ib2/补迁窗口按此实施——K3/K2 Brief 的收口授权在此兑现）

**背景（实测）**：`knowledgeService`（service/knowledge.go:49，字段清单 :49 起约 40 项，含 `semanticScopeGuard` 内嵌（:50；宿主同形定义在 K2 compat :29，未导出）、`audit interfaces.AuditLogService`、`quotaGuard commercial.ResourceQuotaGuard`、`memFAQProgress/memFAQRunningImport sync.Map` 等）。§3.3 解除条件达成后，补迁 commit 内完成以下四项（同 commit 闭环）：

1. **faq_delegate 收口**（授权：K3 Brief (c) :78「K4 knowledge.go 落位 process 后，KnowledgeService 实现体的 FAQ 面归宿按 24 计划收口（接口不变，delegates 换指向）」）：落位包新建 `process/faq_delegate.go`，按 D1 现文本迁入——(a) 15 个 FAQ 接口方法一行委托 `s.faqSvc().X(...)`；(b) 4 个未导出方法（`validateFAQKnowledgeBase`——实际调用方实测为 K2 宿主 compat `writableFAQKnowledgeBase` 方法体内 `kbretrieval_passb_compat.go:198`（该段随第 2 点迁出后此调用随段消失；K3 计划 D1 表原引 `knowledgebase_access.go:42` 为 K2.4 迁出前旧宿主坐标，勘误）与 K4 `knowledge_clone_move.go:700/:855/:885`）；(c) `faqSvc()` 构造照录（seam 闭包中 K2 符号改调落位包导出名）。宿主 D1 整文件删除 + manifest 行删除。
2. **`writableFAQKnowledgeBase` 接收**（授权：K2 Brief §3.3 :78「方法体须随 K4 knowledge.go 补迁或由 K4 属主接收」）：方法文本（函数体一行不改）从 K2 compat 的 :190 注释/:194 func 段迁出，落 process 包同名方法；K2 compat 该段删除，其余段零触碰。
3. **guard 第四轨**：`knowledgeService` 的 `semanticScopeGuard` 内嵌（:50）随类型迁移后，process 包新建 `semantic_scope_guard.go`——结构体+`SetSemanticScopeInvalidator`+5 个 invalidate 方法，逐字对齐 K2 落位包 `retrieval/app/semantic_scope_guard.go` 现文本（实测该文件 :20 type/:24 Set/:27-:53 五方法在册，自带注释「多属主重复 helper 族，ib2 收口为单一定义」）——guard 收口模式的第四轨（K2 落位包/宿主 compat/identity seam 之后），ib2 四轨统一；宿主 compat guard 段不动（留守嵌入方 identity 4 文件 + K2 推迟件继续依赖）。
4. **K3 测试垫片删除**（授权：K3 Brief (b) :57「K2/K4 域改写测试后垫片删除」；Ruling TEST-SUPPORT-SHIM）：垫片 `knowledge_faq_k3_test_support_shim_test.go` 6 符号消费者实测——4 方法（`UpdateFAQEntryStatus`/`UpdateFAQEntryTag`/`resolveTagID`/`buildFAQTagResolver`）唯一调用方=留驻白盒测试 `knowledge_write_access_test.go:200/:208/:362/:364`（随补迁拆分：write 面用例随迁，调用点改同包白盒 `svc.faqSvc().X(...)` 等价形态；tag 面留宿主待 K2 补迁）；2 纯函数（`faqImportCompletedOutcome`/`faqImportActivityDetails`）消费方=`kb_activity_test.go:68/:79`（实测 K2/K3 分支均在场留守——K2 只迁走生产文件）→ 随补迁把 2 用例迁至 `faq` 包侧或 `kb_activity_test.go` 归属窗口处置；消费者清零后垫片删除。

补迁完成断言（BSD grep 形态）：`grep -rc "func (s \*knowledgeService)" internal/application/service/ 2>/dev/null | awk -F: '{s+=$NF} END{print s}'` 输出 0；`go build ./...` 证明 `*process.knowledgeService` 满足 `interfaces.KnowledgeService`（编译器强制，FAQ 切片经 faq_delegate；表外方法编译错误枚举补齐，禁止改接口）。

### 5.4 R2/seam 与直连义务（本批 16 文件）

**5.4a K4 消费他属主未导出/已导出符号**：

| K4 消费面（本批文件） | 他属主符号 | 处置 |
|---|---|---|
| `repository/knowledgebase.go:216/:235/:242`（推迟件 #1） | `scopeKnowledgeBasesByModelID`/`knowledgeBaseModelUsageBindings`（12-commercial，未导出） | 整文件推迟（§3.3），替代 R2 seam（K0 组 E 允许的更保守路径；Ruling 6 先例） |
| `knowledge_write.go:84/:132` 调 `requireKBWrite`（K2 符号，经宿主 compat 委托现可解析） | K2 落位包已导出 `RequireKBWrite`（K2 Brief §2 表，K2 分支实测在册；同表 K4 面共 7 符号：RecordKBActivity/KBActivityTrigger/WithKBActivityTask/KBActivityAppendSampleTitles/KBReadPermissions/RequireKBWrite/WithKBWriteTenantInfo，其余 6 个的调用点全在推迟批，本节点顺延） | K4.2 搬迁时直接改 import + 导出名（文件属主是 K4；K2 Brief §4 :93 预排的「K4 调用点 ib2 直连」由 K4 在自己文件内提前完成）；**禁 process import 宿主 service 包**（宿主 compat→process 反向成环） |
| `knowledge_post_process.go` 调 K3 符号裸名：`previewText`、`enqueueWikiIngestTrigger`（:268/:435）、`extractRealText`（:729）、`newWikiIngestPendingOp`（:317） | K3 `wiki` 包导出 `PreviewText`/`EnqueueWikiIngestTrigger`/`ExtractRealText`/`NewWikiIngestPendingOp`（K3 Brief (b) 表） | 同上直连 `internal/modules/knowledge/wiki`（模块内合法） |
| `knowledge_housekeeping_test.go` 用 `wikiTaskType`/`wikiTaskScope`/`WikiOpIngest`/`WikiOpRetract`（K3 Brief (b) :46 宿主测试） | K3 `wiki` 包导出常量族 | 随迁测试改直连 `wiki.WikiTaskType` 等 |

**5.4b 本批文件对推迟批留守符号的同包裸名断链（3 个，seam/常量照录先例处置）**：

| 断链点（本批文件:行） | 留守符号（推迟批定义） | 处置（先例） |
|---|---|---|
| `knowledge_post_process.go:485` | `finalizeSubtaskDetachedTimeout`（const，knowledge.go:211，`10 * time.Second`） | process 包常量照录（先例=K3 Brief (b) `wikiDeletedTTL`「常量照录，计划外增量」）；文件头注释登记「值对齐 knowledge.go:211，推迟批补迁时收口单一常量」；Brief 登记 |
| `knowledge_process_config.go:109` | `normalizeFileExtension`（knowledge_util.go:47） | process 包同形 seam `normalize_extension_seam.go`（先例=K2 `escape_like_seam.go`：5 行级纯函数逐字对齐宿主文本、文件头注「Pass B 过渡 seam，推迟批补迁/ib2 收口」）；**禁 import 宿主 service**（成环） |
| `knowledge_process_config.go:231` | `getFileType`（knowledge_util.go:89） | 同上同形 seam |

### 5.5 新显形跨模块 import（K4.4 按 Ruling IMPORT-EXCEPTION-REGISTRATION 登记精确豁免）

搬迁使下列既有横向包 import 变为 module→横向（architectureguard forbidden-import；faq→repository 先例已登记同类豁免，K3 计划 §10.4）。实测种子对（base 行号；K4.4 Step 1 全量清点多退少补）：

| 落位文件 | import（base 行号实测） |
|---|---|
| `process/knowledge_span_tracker.go` | `internal/application/repository`（:35） |
| `process/repository/knowledge.go`、`process/repository/knowledge_span_repo.go` | `internal/common`（:9/:8） |
| `process/handler/kb_access.go`、`process/handler/knowledge.go` | `internal/application/repository`（:6/:13——消费 `repository.KnowledgeSpanRepository` 接口形参（:37/:48）与 `repository.ErrKnowledgeBaseNotFound` 哨兵（:2349/:2368）；哨兵与接口随推迟件 #1 留守宿主，直引合法需豁免登记） |

（`knowledge.go`/`knowledge_process.go`/`knowledge_util.go` 的横向 import 随推迟批顺延；`internal/modules/knowledge/searchutil`（knowledge_process.go:22）为模块内合法，顺延后同判。）程序：独立 commit 在 `tools/architectureguard/check.go` importExceptions 追加精确 file→package 条目（Reason 一行、PassBTask=`K4.4`），同窗 exception-ledger.yaml 追加行（owner=`24-knowledge-process`、remove_at=ib2），机械计数修正 + §8 基线登记。禁通配、禁新逻辑。**优先替代评估义务**：哨兵/接口型消费（handler 侧）可评估改为 seam/别名注入以减少豁免条目——二选一，报告记录所选路径（K3 计划 §10.4 同款义务）。

## 6. 公共可观察行为与兼容要求（冻结面，零变化）

| 冻结面 | 事实源 | 兼容要求 |
|---|---|---|
| **18 knowledge worker 注册** | contracts.yaml `knowledge.workers`（frozen）；注册位 task.go:266-320、sync_task.go:143-163 | 注册行零改动（DAG shared_resources「worker 注册行——集成工程师独占」）。本批实现体迁移 2 个（`TypeKnowledgePostProcess`/`TypeKnowledgeAutoTag` 经 `interfaces.TaskHandler`+dig.Name 消歧，container.go:412/:413 Provide 经宿主 compat var 别名零改动）；其余 9 个 K4 面 worker 经 `params.KnowledgeService` 接口分发，实现体随推迟批留守零变化；`TypeIndexDelete`（TagService）、`TypeKBDelete`（KnowledgeBaseService）非 K4 文件实现 |
| **`interfaces.KnowledgeService` 全签名** | contracts.yaml `knowledge.service`:1773（frozen） | 接口零改动（`git diff --name-only` 不含 `internal/types/`）；实现体 `*knowledgeService` 随推迟批留守——接口满足性现状即满足，本节点零风险 |
| **asynq 语义** | knowledge-process.md:50-51「删除与索引语义是外部契约：asynq 任务类型、幂等/重试语义、Redis/Lite 双池注册行为零变化」；spec §7 | 任务 payload 结构（R0 冻结）、MaxRetry/Timeout/TaskID 去重、幂等键零变化；双池（Redis mux.HandleFunc + Lite Executor.RegisterHandler）各注册一次 |
| **事件 producer** | event-catalog「Knowledge 家族」v1：`knowledge.processing.failed`（producer `knowledge_housekeeping.go:runSweep`:112，**随本批迁移**）——`required_metadata`（含 failure_reason）、ordering（per-knowledge 终态跃迁一次）零变化；`knowledge.processing.completed`（repository/knowledge.go:707）/`deletion.completed`（:906）随推迟批留守零变化 | 本批对 runSweep 为纯移动（M2），producer 语义与 metadata 键不改；差分锚点见 §8.3 |
| **633 路由** | contracts.yaml `knowledge.routes`；`RegisterKnowledgeRoutes`（routes_knowledge.go:67，形参 `*handler.KnowledgeHandler`） | 本节点不触 router；宿主 handler compat 留 `KnowledgeHandler` 兼容形态 + `var NewKnowledgeHandler` 别名（container.go:718、routes_knowledge.go:67 零改动）。KnowledgeHandler 字段全为冻结接口（knowledge.go:32-36 实测）+ `spanRepo repository.KnowledgeSpanRepository`（:37，宿主接口留守，经 §5.5 豁免直引） |
| **Housekeeping 调度** | B0.3 Step 4（freeze:207-209）：清扫规则独占归 K4/24；System（42）经窄端口 `KnowledgeHousekeeping` 触发调度；挂点 `startHousekeepingService`（container.go:642 Invoke，func at :2390，形参 `*service.HousekeepingService`——宿主 type 别名保编译） | **不得出现第二套清扫实现**；K4 交付 `process.NewHousekeepingService` + `Start/Stop`（:57/:73/:99 已导出）即端口面（接口定义归使用方 42-system-policy，spec §4.2；K4 不在 module.go 暴露——门面注释禁写）；调度切换走 Integration Brief 由集成工程师执行 |
| **错误哨兵与文案** | `ErrKnowledgeNotFound`（repository/knowledge.go:15）、`ErrKnowledgeBaseNotFound`（repository/knowledgebase.go:13，随推迟件留守）、`ErrWikiIngestConcurrent`（K3 W1 var 别名同一实例）等 | 宿主 compat 留 `var ErrKnowledgeNotFound = repository.ErrKnowledgeNotFound`（同一实例，`errors.Is` 语义不变）；既有错误文本零变化 |
| **事件消费方** | `processing.failed`→knowledge_transfer.go（推迟批留守同包）；`processing.completed`→knowledge_post_process.go（**随本批迁移**，in_process 同包消费转为 process 包内——post_process 对留守符号仅 §5.4b 三点，seam 后编译） | 消费链语义零变化 |

## 7. 任务（业务完整、可独立审阅；一任务一 commit，conventions §4）

> 全部任务属主节点：`b2-k-process`。依赖序 K4.0 → K4.1 → K4.2 → K4.3 → K4.4 → K4.5 串行（同分支）。

### Task K4.0 — 基线对齐、前置核验、T0 台账、白盒测试全量分类与推迟批登记

**Files:** 无生产文件改动；产出报告 T0/分类清单/推迟登记章节草稿。

- [ ] **Step 1: 基线对齐 merge**——按 P2 顺序三个独立 merge commit（K1→K2→K3）；每个 merge 后 `go build ./...` 退出 0（三方 approved 分支；冲突处置见 P2）。
- [ ] **Step 2: 前置核验**——P1/P3/P4 逐条执行并留痕（命令原文+退出码）；实测核对 P4 清单中每个过渡物文件在场。
- [ ] **Step 3: T0 台账与基线口径登记**——**基线口径（波内依赖节点特化，与 conventions §1.2 模板的偏差如实登记）**：`git merge-base origin/main HEAD` 实测返回 main 尖 `a42179135`（main 是本分支祖先），**不是**派发基线（分支创建点=集成分支头 `37eae710f`）；且 P2 三次基线对齐 merge 后，任何对基线 SHA 的三点 diff 必然混入 K1+K2+K3 的自有 commit（`--no-merges` 只剔除 merge commit 本身、不剔除被合入分支的普通 commit），无法用于 owned_files 核对。因此本节点固定口径：(i) `PRE_MERGE_SHA=$(git rev-parse HEAD)`（Step 1 三个 merge 之前取值，预期 `37eae710f`，报告登记；若协调者派发时已回填 `base_sha` 则以其为准并与 `PRE_MERGE_SHA` 比对，不一致停下上报）；(ii) `PASSB_BASE_SHA` 按派发回填值或 `PRE_MERGE_SHA` 记录（仅用于 conventions §1.2 命令包的形式执行与基线台账）；(iii) **owned_files 核对一律用本分支第一父链自有 commit 的变更并集**（K4.5 Step 3 专用命令：`git rev-list --first-parent --no-merges "$PRE_MERGE_SHA"..HEAD | while read c; do git diff-tree --no-commit-id --name-only -r "$c"; done | sort -u`——`--first-parent` 只沿本分支第一父链回溯，前置分支自有 commit 不在链上，天然排除）。之后逐包跑并记录：`go test -count=1 ./internal/application/repository/ ./internal/application/service/ ./internal/handler/ ./internal/modules/knowledge/...`（含 skip/blocked-env 如实记录，spec §14.1）。
- [ ] **Step 4: 白盒测试全量分类清单落盘**——`grep -ln "&knowledgeService{" internal/application/service/*_test.go`（当前树实测 25 文件）在对齐后树上复跑，逐文件按 §3.3 根因类 A 三栏（K4 属主随迁/他属主留守/已随 K3 迁走）+ 「引用 knowledgeService 但经构造器/接口/局部变量名」组（实测 9 文件：agent_service_test（局部变量名 `fakeAgentKnowledgeService`）、datasource_{sweep_wiring,result_cap,reindex,stream,sync_cancel,service,purge}_test（`DataSourceService` 接口字段名）、session_tag_targets_test 等——经接口形态，compat var 别名即可保护，逐一核验确认无 `*knowledgeService` 类型标注）定性，产出清单写入报告。
- [ ] **Step 5: 推迟批登记（Ruling 6）**——§3.3 表 12 文件 + 根因证据（7 白盒测试 file:line、过渡物清单）+ 解除编排（§5.3 蓝本指针）落盘报告；上报协调者记 DAG notes（ppc airesource 伪影 + 边扩展登记 + knowledge_move_wiki_test 勘误一并）。
- [ ] **Step 6: 耦合全景复核**——§5.1–§5.5 全部 file:line 与签名在对齐后树上重跑 grep 核对（重点：§5.2 K1/K3 消费面签名、§5.4a 直连目标导出名、§5.4b 三断链点、§5.5 种子对）；与 20 计划 §6.2 组 C/E 差异如实登记上报。
- **Commit:** 无独立 commit（台账随 K4.5 报告提交）；三个 merge commit 即本任务产物。**验收**：三 merge 后 build 绿；T0 表、分类清单、推迟登记在报告草稿。

### Task K4.1 — repository 层 4 文件归位（escapeLikeKeyword 导出 + 哨兵别名 + K2 测试垫片删除）

**Files:** Move `repository/knowledge.go`、`knowledge_span_repo.go`、`knowledge_tag.go`、`knowledge_transfer.go` + 随迁测试（§8.1 repository 侧：`knowledge_tag_test.go`、`knowledge_finalize_test.go`、`knowledge_span_repo_test.go`、`knowledge_transfer_test.go`、`knowledge_datasource_external_id_test.go`、`knowledge_datasource_test.go`、`knowledge_duplicate_test.go`、`knowledge_folder_move_test.go`、`knowledge_folder_test.go`、`knowledge_list_filter_test.go`、`knowledge_metadata_prefix_test.go`、`knowledge_source_schema_test.go`、`knowledge_create_test.go`——K4.0 Step 4 清单为准）→ `internal/modules/knowledge/process/repository/`；Create 宿主 `internal/application/repository/kbprocess_passb_compat.go`；Delete `internal/application/repository/kbretrieval_passb_compat_test.go`（K2 垫片，删除前置满足，K2 Brief :80）；Edit `docs/architecture/moves/knowledge.yaml`（删 4 行 + compat 1 行新增 + 垫片行删除）、镜像 README。

- [ ] **Step 1: git mv（M2）**——4 生产 + 随迁测试；`git diff --summary --find-renames HEAD` 全 R100；`package repository` 子句不变（落位包名对齐宿主）。
- [ ] **Step 2: 导出改名（M3）**——`escapeLikeKeyword`→`EscapeLikeKeyword`（knowledge.go:22，函数体一行不改）；宿主同包调用方（tenant.go/tag.go）均他属主，经 shim，包内无 R3 改名点。
- [ ] **Step 3: 宿主 compat**——先 `go build ./...` 记录断链证据（预期 container.go:203/:204 与 tenant.go/message.go/session.go 编译错误）；写 `kbprocess_passb_compat.go`：`var ErrKnowledgeNotFound = repository.ErrKnowledgeNotFound`（哨兵同一实例）+ `var NewKnowledgeRepository = repository.NewKnowledgeRepository`、`var NewKnowledgeSpanRepository = repository.NewKnowledgeSpanRepository`（container.go:203/:204 引用面）+ `func escapeLikeKeyword(keyword string) string { return repository.EscapeLikeKeyword(keyword) }`（identity/conversation 调用点保护）。
- [ ] **Step 4: 垫片删除**——`kbretrieval_passb_compat_test.go` 删除 + manifest 行删除（同 commit；其 `knowledgeTagRepository` 委托由随迁后的 knowledge_tag_test.go 直连 `kbretrieval.NewKnowledgeTagRepository` 替代——测试内构造改写属 K4 属主文件，合法）。
- [ ] **Step 5: manifest**——删 4 行 + compat 登记 + 垫片行删除；`go run ./tools/modulemove verify --module knowledge` OK。
- [ ] **Step 6: GREEN**——`go build ./...` OK；`go test -count=1 ./internal/modules/knowledge/process/... ./internal/application/repository/ ./internal/application/service/` 与 T0 一致；`make verify-module-moves && make check-backend-architecture` 双绿（新显形横向 import 若报诊断先记清单，K4.4 统一登记；repository 侧仅 `internal/common` 2 处（§5.5）——本任务可先零豁免完成除 common 外全部）。
- **Commit:** `refactor(knowledge): K4 repository layer into module (escapeLikeKeyword export, host compat, K2 test-shim removal)`

### Task K4.2 — service 独立面 8 文件归位（write-family/indexContent 导出 + 3 处 seam/常量照录 + K3 符号直连）

**Files:** Move `service/knowledge_auto_tag.go`、`knowledge_housekeeping.go`、`knowledge_index_content.go`、`knowledge_post_process.go`、`knowledge_process_config.go`、`knowledge_span_tracker.go`、`knowledge_task_options.go`、`knowledge_write.go` + 随迁测试（实测 8 文件测试均不构造 `&knowledgeService{}`：`knowledge_auto_tag_test.go`、`knowledge_housekeeping_test.go`、`knowledge_post_process_{graph_chunks,summary,trace,wiki_enqueue}_test.go`、`knowledge_process_config_test.go`、`knowledge_span_tracker_test.go`、`knowledge_task_options_test.go`；**留守锚点**：`knowledge_index_content_test.go`、`knowledge_write_access_test.go`、`knowledge_quota_guard_test.go`（三者实测白盒构造 `&knowledgeService{}`，被测链经留守类型，随推迟批窗口处置）——K4.0 Step 4 清单为准）→ `internal/modules/knowledge/process/`（package process）；Create 落位包 `normalize_extension_seam.go`（§5.4b，含 normalizeFileExtension/getFileType 两同形函数）+ 常量照录段（`finalizeSubtaskDetachedTimeout`，并入 seam 文件或独立小文件）+ 宿主 `internal/application/service/kbprocess_passb_compat.go` 初版；Edit manifest（删 8 行）、镜像。

- [ ] **Step 1: git mv（M2）**——8 生产 + 随迁测试；R100。
- [ ] **Step 2: 导出改名 + seam（M3）**——(i) `buildKnowledgeIndexContent`→`BuildKnowledgeIndexContent`（index_content.go:12）、write-family 4 函数→导出名（knowledge_write.go:17/:32/:60/:93）；(ii) 落地 §5.4b：`normalize_extension_seam.go`（逐字对齐 knowledge_util.go:47/:89 现文本）+ `finalizeSubtaskDetachedTimeout` 常量照录（值=knowledge.go:211 的 `10 * time.Second`）；(iii) `knowledge_write.go:84/:132` 的 `requireKBWrite` 改调 `kbretrieval.RequireKBWrite`（import 落位包）；(iv) `knowledge_post_process.go` 的 `previewText`/`enqueueWikiIngestTrigger`/`extractRealText`/`newWikiIngestPendingOp` 调用点改 import `wiki` 包导出名。
- [ ] **Step 3: 宿主 compat 初版**——`var NewKnowledgePostProcessService = process.NewKnowledgePostProcessService`（container.go:412，dig.Name 不变）、`var NewKnowledgeAutoTagService = …`（:413）、`var NewHousekeepingService = …`（:641）+ `type HousekeepingService = process.HousekeepingService`（startHousekeepingService 形参 :2390）+ `type SpanTracker = process.SpanTracker`、`var NewSpanTracker = process.NewSpanTracker`（container.go:374、K1 adapter）+ 本批导出符号一行委托（write-family/BuildKnowledgeIndexContent——保护 K1 `span_trace_seam_adapter.go` 引用面）。核验 K1 adapter 与 K1 R1-9 shim 零改动编译（其对 getFileType 族的函数值引用仍解析宿主原符号——util 留守 ✓）。
- [ ] **Step 4: housekeeping 测试联动**——随迁 `knowledge_housekeeping_test.go` 的 wiki 常量族（wikiTaskType/wikiTaskScope/WikiOpIngest/WikiOpRetract）改直连 `wiki.WikiTaskType` 等。
- [ ] **Step 5: GREEN + manifest**——`go build ./...`；`go test -count=1 ./internal/modules/knowledge/process/... ./internal/application/service/` 与 T0 一致（留守锚点测试经宿主现状编译）；双守卫（common/暂红登记纪律同 K4.1 Step 6）。
- **Commit:** `refactor(knowledge): K4 standalone service facet into module (write/index export ports, seam for deferred-batch symbols, wiki/kbretrieval direct imports)`

### Task K4.3 — handler 层 4 文件归位（路由兼容别名 + faq seam 导出）

**Files:** Move `handler/kb_access.go`、`knowledge.go`、`knowledge_download.go`、`task_progress_auth.go` + 随迁测试（kb_access_test、knowledge_api_key_scope、knowledge_download、knowledge_folder（handler 版）、knowledge_move_gate（handler 版）、knowledge_mutation_admission（contracts 登记）、knowledge_ownership、knowledge_preview_security（contracts 登记）、knowledge_spans、knowledge_tag_ids、knowledge_transfer（handler 版）、task_progress_auth_test；**rbac_lookups_test.go 不随迁**（identity 推迟件，K2 计划 §7.2））→ `internal/modules/knowledge/process/handler/`；Create 宿主 `internal/handler/kbprocess_passb_compat.go`；Edit manifest（删 4 行）、镜像。

- [ ] **Step 1: git mv（M2）**——4 生产 + 测试；R100；`package handler` 子句不变。
- [ ] **Step 2: 导出改名（M3）**——`parseCommaSeparatedTagIDs`→`ParseCommaSeparatedTagIDs`（knowledge.go:2546）、`requireTaskProgressTenant`→`RequireTaskProgressTenant`（task_progress_auth.go:14）；包内 R3 调用点随改；`resolvedKBAccess`（kb_access.go:18）、`resolveHandlerKBAccess`（:28）、`resolveHandlerKBAccessFor`（:34）三个包内 helper 的 handler 包内其他属主文件调用点处置以 `go build` 断链清单为准——**若宿主 handler 包存在他属主调用方**，宿主 compat 留一行委托；若仅 K4 文件消费则直迁（K4.0 Step 4 判定）。
- [ ] **Step 3: 宿主 compat**——`type KnowledgeHandler struct { *processhandler.KnowledgeHandler }`（嵌入保方法集提升；routes_knowledge.go:67 形参类型不变）+ `var NewKnowledgeHandler`（container.go:718）+ 两导出符号委托——**若构造器形参含宿主具型**则以 type 别名 + var 转发组合（K3 H1/H2 双先例按断链证据择一，报告记录）。
- [ ] **Step 4: GREEN + manifest**——`go test -count=1 ./internal/modules/knowledge/process/... ./internal/handler/` 与 T0 一致（mutation_admission/preview_security/download 双跑）；双守卫。
- **Commit:** `refactor(knowledge): K4 handlers into module (route compat aliases, faq seam exports)`

### Task K4.4 — 跨模块 import 豁免登记与计数基线（独立 commit）

**Files:** Edit `tools/architectureguard/check.go`（importExceptions 数据行）、`docs/architecture/passb/exception-ledger.yaml`（属主行）；报告豁免章节。

- [ ] **Step 1: 全量清点**——`grep -rn "internal/application/\|internal/common\|internal/infrastructure" internal/modules/knowledge/process/ --include="*.go" | grep -v "_test.go"` 逐条列出 file→package 对；与 §5.5 种子表核对（多退少补）；每对登记前评估 seam 替代（§5.5 末段义务）。
- [ ] **Step 2: 登记豁免**——按 Ruling 程序追加 check.go 精确条目 + ledger 行（owner=24-knowledge-process、remove_at=ib2）；机械计数修正 + §8 基线登记（evidence 登记原因与 Ruling 引用）。
- [ ] **Step 3: GREEN**——`make check-backend-architecture` 零诊断；`go build ./...` 退出 0。
- **Commit:** `chore(passb): register exact cross-module import exceptions surfaced by K4 process moves (Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY)`

### Task K4.5 — 高风险差分、Integration Brief、节点门禁与收口

**Files:** Create `docs/architecture/evidence/passb/b2-k-process.md`、`docs/plans/passb/reports/b2-k-process.md`（补全 K4.0 T0 表/分类清单/推迟登记）、`docs/architecture/passb/briefs/b2-k-process.md`。

- [ ] **Step 1: 差分执行**——§8.3 表逐面双跑（T0 基线输出 vs 搬迁后同用例），用例清单、双跑输出摘录、比对结论、命令与退出码入 evidence 差分章节；失败只修新实现。
- [ ] **Step 2: 节点 gates 逐条执行并摘录**——`go build ./...`、`go test ./internal/modules/knowledge/... -count=1`、`make check-backend-architecture`、`make verify-module-moves`（命令原文+退出码入报告；conventions §2 禁替代）。
- [ ] **Step 3: 变更清单 vs owned_files（第一父链自有 commit 并集口径，K4.0 Step 3 (iii) 定义）**——`git rev-list --first-parent --no-merges "$PRE_MERGE_SHA"..HEAD | while read c; do git diff-tree --no-commit-id --name-only -r "$c"; done | sort -u`（`--first-parent` 排除基线对齐带入的前置分支自有 commit——`--no-merges` 单用不排除）；另按 conventions §1.2 形式执行 `git diff --stat "$PASSB_BASE_SHA"...HEAD` 并在报告注明「含前置基线对齐内容，owned_files 判定以上一并集为准」。并集与 §4 可写清单逐条核对，差集非空即失败。
- [ ] **Step 4: Integration Brief**——内容：(a) 装配切换表——container.go:202-204/:374/:412/:413/:641/:718 七组 Provide 与 :642 Invoke、routes_knowledge.go:67 形参的 ib2 直连目标；(b) 18 worker 的 K4 面注册行核验说明（本批 2 个 dig.Name + 推迟批 9 个接口分发的零改动证据）；(c) **推迟批解除窗口执行蓝本（§5.3 全文指针 + §3.3 解除条件逐测试属主编排：craft×2→41、datasource_purge→26/ib2、shared_access×2+semantic_scope_mutation→K2 补迁窗、caller_scope→K4 补迁随迁）与四项收口授权原文引用（K3 Brief (c) :78 / K2 Brief §3.3 :78 / K3 Brief (b) :57）**；(d) 本批导出符号的跨 owner 宿主调用点 ib2 改写清单（identity tenant.go:85/:90、conversation message.go:215、session.go:209）；(e) 宿主 compat 3 文件删除编排（残留 importer 清零前置）；(f) 推迟件登记与 K2 推迟件 3-10 的 K4 侧条件现状（knowledge_delete.go 未迁，解除条件未满足，如实登记）；(g) K1 `span_trace_seam_adapter.go`/R1-9 shim 的 ib2 切换目标（本批 write-family/indexContent 导出已就绪、getFileType 族与 enqueueSummaryRefresh/attemptSuperseded 随推迟批）；(h) §5.4b 三 seam/常量照录的 ib2 收口编排；(i) 测试锚点残差（knowledge_index_content/write_access/quota_guard 三留守测试 + 7 他属主白盒测试 + knowledge_move_wiki_test/document_write_access_test 勘误）与 K2 垫片删除记录；(j) §8 计数奇偶记录。
- [ ] **Step 5: 报告**——执行命令台账、owned_files 核对结论、推迟项 12 个、airesource ppc 伪影上报、未完成项如实列出（conventions §1.2）。
- **Commit:** `docs(passb): b2-k-process differential evidence, integration brief and node report`

## 8. 测试与高风险差分要求

### 8.1 随迁 `_test.go` 判定（规则 + 已确认清单；K4.0 Step 4 落盘终版）

**判定规则**（K2 计划 §7.1 先例）：同名主题测试 + 被测对象仅本批文件；**白盒构造 `&knowledgeService{}` 的测试一律留守**（类型随推迟批在宿主，测试与类型同进退）；白盒引用他 plan 属主未导出符号时以定义文件属主为随迁归属（20 计划 §9 共性义务），残差登记 Brief。

**已确认随迁（实测不构造 `&knowledgeService{}`）**：service 侧 8 文件对应测试（auto_tag、housekeeping、post_process 四件、process_config、span_tracker、task_options）；repository 侧（`knowledge_tag_test.go`（K2 垫片服务对象，:217-243 构造 repository 白盒）、`knowledge_finalize_test.go`（事件 producer 差分锚点）、span_repo/transfer/datasource_external_id/datasource/duplicate/folder_move/folder/list_filter/metadata_prefix/source_schema/create）；handler 侧（§7 K4.3 Files 清单，rbac_lookups_test 除外）。

**留守锚点（防误迁，实测白盒构造 `&knowledgeService{}` 或他属主）**：`knowledge_index_content_test.go`、`knowledge_write_access_test.go`、`knowledge_quota_guard_test.go`（构造留守类型）；`knowledge_move_wiki_test.go`（20 计划 §9 原判随 K4——**勘误登记**：其白盒 `svc.moveOneKnowledge`（:163）/`svc.repo`（:167/:235）耦合的 knowledgeService 与 moveOneKnowledge 均随推迟批留守，本节点随迁不可行，改随推迟批补迁窗口；上报协调者记 DAG notes）、`document_write_access_test.go`（同因）；7 个他属主白盒测试（§3.3 根因类 A）；K2 推迟件关联测试（knowledgebase_*、knowledge_shared_*、knowledge_caller_scope 等）；`rbac_lookups_test.go`（identity）。

### 8.2 复用与新增

全部复用现有测试（不新写行为测试）；新写仅机械守卫：K4.2 Step 3 的 K1 adapter 编译核验（`go build ./...` 即断言）与 seam 对齐注记。**TDD 顺序**（conventions §1.4）：每任务先跑 T0 基线再动文件；seam/常量照录以「宿主原文本 diff 为零（值/逐字）」为 GREEN 判据。

### 8.3 高风险差分（framework:40「knowledge deletion/indexing」+ 20 计划 §10 K4 行的**本批可执行子集**；spec §14.3 同输入双跑；证据入 evidence 差分章节；删除级联/ProcessDocument/克隆迁移四面随推迟批顺延，Brief 登记补迁窗口义务）

| 面 | 锚定用例（现状基线，随迁后同用例双跑） | 等价判据 |
|---|---|---|
| 巡检置败（`knowledge.processing.failed` producer，`TypeManualProcess` 清扫注释锚） | `knowledge_housekeeping_test.go`（stale 阈值、filterByLastSpanActivity:220/filterOutQueued:295、cron Start:73/Stop:99） | sweep 候选集、置败路径、failure_reason metadata 一致 |
| 后处理链（`TypeKnowledgePostProcess`/`TypeKnowledgeAutoTag`/`TypeSummaryGeneration` 消费面） | `knowledge_post_process_{graph_chunks,summary,trace,wiki_enqueue}_test.go`、`knowledge_auto_tag_test.go` | 子任务收敛/图谱块/摘要刷新/wiki 入队触发（经 wiki 导出直连后）逐用例一致 |
| write-family 守卫（K1 KnowledgeWriteGuard seam 依赖面） | `knowledge_task_options_test.go`、`knowledge_span_tracker_test.go`（span 树/LookupStage/BeginSubSpan）+ K1 落位包 `extract_constructor_guard_test.go`（K1.4 产物，跨 plan 锚点只跑不迁） | 守卫判定序列、span 语义一致 |
| handler 面（下载/预览安全/变更准入/task 进度防枚举） | `knowledge_download_test.go`、`knowledge_preview_security_test.go`、`knowledge_mutation_admission_test.go`、`kb_access_test.go`、`task_progress_auth_test.go` | 状态码映射、RBAC/apiKey 门、跨租户 404 防枚举逐用例一致 |
| repository 面（模型写入/查询/事件 producer 纯函数面） | `knowledge_finalize_test.go`（CompleteProcessingWithoutSubtasks:707 原子 promote 语义）、`knowledge_span_repo_test.go`、`knowledge_tag_test.go`（BatchCountReferences 经直连 kbretrieval 构造） | DB 结果、跃迁恰一次语义一致 |

差分失败只修新实现、禁改期望值（spec §14.3）；legacy 删除（manifest 行）前对应面差分必须已通过（framework:31）。T1/T2 梯度：每任务搬完跑落位包+宿主包测试与 T0 台账比对（新失败即停，spec §14.1）。

## 9. 集成与回滚边界

- **集成**：本节点分支经 review 后按缺省序 K1→K2→K3→K4 由集成工程师以 `merge: passb b2-k-process` 合入（framework:28）；IB2 只从 `docs/architecture/passb/briefs/b2-k-process.md` 切换共享装配（framework:103）；router/container/task/sync_task/bootstrap 改写仅集成工程师按 Brief 串行执行（本节点零触碰，K1/K2 先例 diff 0 行）；contracts.yaml knowledge 区状态回写仅 ib2（conventions §3；F2 status 字段未落盘，K0 差异③先例——本节点不声称契约状态变更）。
- **回滚**（spec §13）：K4.0 三个基线对齐 merge commit 回滚=revert 单 commit 无装配影响；K4.1–K4.3 为 M2+M3 提交，分支内逐 commit revert（宿主恢复原文件、垫片复活），已并入集成分支的 M2/M3 保留（新路径未接线不影响运行，M4 失败先回退切换 commit）；K4.4 为数据行 commit，revert 需同步还原计数登记（§8 基线流程）；本节点无 schema/migration/路由/worker 计数变化，回滚不需数据修复；compat/shim 删除（ib2）在任何残留 importer 未清零前禁止执行（framework B5 口径前移）。
- **升级边界**：门禁不可能通过、冻结接口表外方法、或需改 20 计划 §5/§6 冻结表时，按 conventions §5 停手上报（报告 + DAG notes 建议），禁止现场改判所有权、删测试或扩大例外。

## 10. 必须删除的 legacy/alias/例外（本节点口径）

| 对象 | 动作 | 时点 |
|---|---|---|
| `docs/architecture/moves/knowledge.yaml` 中 16 个已物理搬迁文件的 `legacy_files` 行 | 同搬迁 commit 删除（modulemove 机械强制；Ruling LEGACY-ROW-OWNERSHIP） | K4.1–K4.3 |
| `ownership-matrix.yaml` 对应 16 行 | 不由本节点删（barrier ib2 回写窗口） | ib2 |
| 推迟批 12 文件的 manifest/matrix 行 | **保留**（未迁移不删行；Ruling 6 第 4 点；B5 的 396 口径不减免） | 补迁窗口 |
| K2 垫片 `kbretrieval_passb_compat_test.go` + manifest 行 | K4.1 同 commit 删除（K2 Brief :80 删除前置满足） | K4.1 |
| **K3 垫片 `knowledge_faq_k3_test_support_shim_test.go`** | **本节点零动作**（实测消费者全在场：4 方法→knowledge_write_access_test.go:200/:208/:362/:364 留守、2 纯函数→kb_activity_test.go:68/:79 留守；删除随推迟批补迁窗口，§5.3 蓝本第 4 点） | 补迁窗口 |
| 3 个宿主 compat 文件（kbprocess_passb_compat ×3）+ manifest 登记行 | 新增为待删项（Ruling TRANSITION-SHIM-ROW-REGISTRATION 成对补行）；删除=ib2 直连完成后 | ib2 |
| 新登记 importExceptions 精确豁免 + ledger 行（K4.4，预期 §5.5 种子 5 对左右） | 保留至跨模块消费经门面/端口合法化；remove_at=ib2 | ib2 |
| exc-0088/0089/0090/0091 与 exc-0106..0110 | 非本节点属主（21/22），无删除动作；K2 Brief §6 escapeLikeKeyword seam 的收口条件（「K4 迁移 knowledge.go 导出窄端口后删 seam」）**前半已由 K4.1 满足**（repository/knowledge.go 已迁 + EscapeLikeKeyword 已导出），seam 删除动作仍归 ib2 复核 | ib2 |
| 别名 18 条（knowledge.yaml:41-77）与 container.go:36-38 旧 import | 不在本节点文件范围（K5/ib2 收口，20 计划 §7.5） | K5/ib2 |

## 11. 独立验收标准

1. DAG `b2-k-process` gates 四项全绿，命令原文+退出码记录于 `docs/plans/passb/reports/b2-k-process.md`（conventions §2 禁替代命令）；
2. 本分支第一父链自有 commit 变更并集（K4.5 Step 3 命令）与 §4 可写清单差集为空（`PASSB_BASE_SHA` 三点 diff 仅作台账，K4.0 Step 3 口径）；16 文件物理落位三个子包，逐文件 `git diff --summary --find-renames` R100、函数体 diff 为零（package/import/R3 改名/R2 直连/§5.4b seam 三点除外，逐类登记）；
3. manifest `legacy_files`：16 行已删、compat 3 行在册、推迟批 12 行保留、K2 垫片行随删除；`make verify-module-moves` OK；
4. 推迟批 12 文件与全部挂 `*knowledgeService` 的宿主过渡物（D1、K3 垫片、K2 compat、kb_activity_test.go、7 他属主白盒测试）原位零改动（`git diff --name-only` 并集不含）；§5.3 蓝本与 §3.3 解除编排完整写入 Brief；
5. §5.1/§5.2 导出符号在落位包以导出名可解析，宿主 compat 一行委托逐一在册（报告核对表）；identity/conversation 调用点文件零改动；
6. §8.3 五个差分面双跑逐用例一致，证据入 `docs/architecture/evidence/passb/b2-k-process.md`（conventions §6）；
7. worker 注册行 `internal/router/task.go`、`internal/router/sync_task.go` 与 container/router/bootstrap/migration/go.mod 零触碰；计数基线 633/23+23/58/537 三方一致不因本节点变化（conventions §8；importExceptions 增量与 ledger 行一一对应、remove_at=ib2、基线变更登记在案）；
8. 治理文件（ownership-matrix/contracts/event-catalog）零改动；K1/K2/K3 产物零触碰（除 §4 授权一处：K2 测试垫片删除——附授权原文引用）；
9. 计划评审 approved（reviewer 独立产出 `docs/plans/passb/reviews/b2-k-process.md`）。

## 12. 计划自检记录（撰写时执行）

- **Spec 覆盖**：ask 九要素 → §1（Spec 指针）、§2（前置与 BLOCKED 解除实证）、§3-§4（精确文件与写权）、§5-§6（真实接口签名/行为兼容）、§7（步骤+命令+预期）、§8（测试与差分）、§9（集成与回滚）、§10（删除义务）、§11（验收）——全覆盖。
- **无占位符**：全部符号含 file:line 与实测签名（escapeLikeKeyword:22、withKnowledgeCleanup:22、deleteReferencedKnowledge:36、isValidFileType:62、moveOneKnowledge:1194、attemptSuperseded:202、finalizeSubtaskDetached:235、finalizeSubtaskDetachedTimeout:211、isLikelyRateLimitError:3902、removeSourceRef:346、enqueueSummaryRefresh:83、buildKnowledgeIndexContent:12、parseCommaSeparatedTagIDs:2546、requireTaskProgressTenant:14、runSweep:112、CompleteProcessingWithoutSubtasks:707、HardDeleteKnowledge:906、NewKnowledgeService:100、NewSpanTracker:172 等，撰写时逐条 grep/sed 命中）；无 TBD；「断链证据先行」「以 K4.0 Step 4 清单为准」是 Ruling 6 框架下的机械判定义务（规则+初判清单已给）。
- **类型一致性**：§5 全部签名与 20 计划 §6.2 冻结表零漂移（差异两处如实登记：airesources deleteReferencedKnowledge 调用点实测 0（ppc 伪影，K4.0 上报）；getParserEngineOverridesFromContext/isValidFileType 同名伪影沿用 K0.1 消歧结论）；`interfaces.KnowledgeService` 全签名转录自 contracts.yaml:1773；worker 分发路径与 task.go:266-320/sync_task.go:143-163 逐行实测对应。
- **跨任务接口一致性**：K4.1 垫片删除前置（knowledge_tag_test 随迁）任务内闭环；K4.2 的 seam 三点先于 GREEN 门禁；K4.3 的 faq seam 导出与 K3 Brief (d) 表逐行对应；推迟批不产生任何跨任务接口义务（留守即现状编译）；§5.3 蓝本与 §3.3 解除条件/K4.5 Brief (c) 三处互指一致。
- **不可行/未做项如实登记**：基线对齐 merge 未执行（K4.0 开工核验）；`go test` 各命令为计划预期（计划撰写为 docs 变更，未跑搬迁后测试——T0/T1/T2 实测义务在任务步骤内）；`knowledge_move_wiki_test.go` 随迁判定与 20 计划 §9 冲突（白盒耦合留守类型，勘误登记上报）；K1 落位包部分导出符号实际签名以 K4.0 Step 6 复核为准。
- **审校修复轮一（2026-09-25，8 项 findings 全处置）**：guard 第四轨（§5.3 蓝本第 3 点，实测 retrieval/app/semantic_scope_guard.go :20/:24/:27-:53 在册）；K3 测试垫片收口（§5.3 蓝本第 4 点）；PASSB_BASE_SHA 口径（K4.0 Step 3）；enqueueSummaryRefresh 批次（顺延符号，§5.1）；P2 边扩展登记（§2）；kb_access 行号 :18/:28/:34（K4.3 Step 2）；D1 (b) 调用方勘误（kbretrieval_passb_compat.go:198，§5.3 蓝本第 1 点）；K2 符号计数（§5.4a 点名 7 个）。
- **审校修复轮二（2026-09-25，5 项 findings 全处置，逐条实测复核）**：① critical——上轮「kb_activity_test.go 已随 K2 迁走」断言实测为假（`git ls-tree codex/passb-b2-k-retrieval -- …/kb_activity_test.go` 与 K3 分支均在场；K2 只迁走生产文件 kb_activity.go）→ K3 垫片 6 符号消费者全部在场，垫片本节点零触碰（§4 禁改/§10/§5.3 蓝本第 4 点解除前置更新）；② critical——实测 25 个宿主测试白盒构造 `&knowledgeService{}`（17 K4 属主 + 1 已随 K3 走 + **7 他属主**：craft_knowledge_test.go:114/:460/:512、craft_knowledge_tool_test.go:99、datasource_purge_test.go:225、knowledge_shared_access_test.go、knowledge_shared_storage_failure_test.go、semantic_scope_mutation_test.go、knowledge_caller_scope_test.go:153）→ **knowledgeService 核心批 11 文件整体推迟**（Ruling 6；根因类 A 与 K2 Brief §5 :104 推迟件 3-10「他属主宿主白盒测试」同型；独立面 8 文件实测零类型引用仍迁移；§3.3/§5.3 重构为推迟登记+解除蓝本；任务 K4.0–K4.5 重排；§8.1/§8.3/§10/§11 同步；knowledge_move_wiki_test/document_write_access_test 随迁判定勘误）；③ important——`--no-merges` 不排前置自有 commit（审校实测复现）→ owned_files 核对改 `git rev-list --first-parent --no-merges` 第一父链口径（K4.0 Step 3 (iii)/K4.5 Step 3/§11.2）；④ minor——目录 `grep -c` 在 BSD grep 报错 → 断言命令改 `grep -rc … | awk` 聚合形态（§5.3 蓝本末）；⑤ minor——复审确认项（guard 第四轨/enqueueSummaryRefresh 等）随方案重构并入蓝本与顺延清单。
