# Pass B B2 — 24-knowledge-process（K4 Knowledge 处理流水线/状态机域 28 legacy 文件归位）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**节点：** `b2-k-process`（DAG `docs/plans/passb/execution-dag.json`，phase B2，role work，execution_mode serial，`depends_on: [b2-k-ingest, b2-k-retrieval]`，gates=`go build ./...` / `go test ./internal/modules/knowledge/... -count=1` / `make check-backend-architecture` / `make verify-module-moves`）。本文件只承载 K4 本节点任务 K4.0–K4.6；同程序 K0/K5 任务节属 `b2-k0`/`b2-k-integration`，载体为 `20-knowledge-program.md`（本计划只消费其 §5–§7 冻结表与 §9 K4 行义务，不复述、不改写）。

**Goal:** 把 knowledge 域处理流水线子集 28 个 legacy 文件（ownership-matrix `plan: 24-knowledge-process` 全部 28 行；其中 1 个按 Ruling 2026-09-25-DEFERRED-FILE-SPLIT 推迟，见 §3.3）从横向宿主包迁入 `internal/modules/knowledge/process` 三层子包；`knowledgeService` 类型定义与全部挂方法文件同批迁移（Go 方法集规则）并收口 K3 宿主委托文件 D1 与 K2 compat 的 `writableFAQKnowledgeBase` 方法（两 Brief 均已点名授权，见 §5.3）；三个跨 owner 未导出符号按 20 计划 §6.2 组 C 导出；冻结端口 `interfaces.KnowledgeService` 的 worker 方法切片经构造器 var 别名与接口分发保持注册行为零变化；全部节点门禁绿色并产出差分证据与 Integration Brief。

**Architecture:** K4 在 K1+K2 契约稳定后串行执行（framework:118-119）；本节点不实现 `internal/modules/knowledge/module.go` 门面接线（归 K5/ib2，conventions §3）；worker 注册行（`internal/router/task.go:266-320`、`sync_task.go:143-163`）禁改——18 个 knowledge worker 中 K4 面的 11 个（`TypeDocumentProcess`/`TypeManualProcess`/`TypeFAQImport`/`TypeQuestionGeneration`/`TypeSummaryGeneration`/`TypeKBClone`/`TypeKnowledgeMove`/`TypeKnowledgeListDelete`/`TypeKnowledgeListReparse` 经 `params.KnowledgeService` 接口分发，`TypeKnowledgePostProcess`/`TypeKnowledgeAutoTag` 经 `params.KnowledgePostProcess`/`KnowledgeAutoTag`（`interfaces.TaskHandler`，dig.Name 消歧）分发）全部经冻结接口解析到落位实现，注册行零改动（与 conversation-queryhistory.md:39-42 义务 3 同律，本会话实测 task.go:269-320、sync_task.go:145-163）。

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
| B0 冻结产物（`.worktrees/passb-int/docs/architecture/passb/`）：`ownership-matrix.yaml`（28 行 plan=24，destination 全部 `internal/modules/knowledge/process`，integration_owner=ib2、delete_barrier=ib2，实测 python3 解析）、`contracts.yaml`（`knowledge.service`:1773 全签名 frozen、`knowledge.chunk-service`:1576、`knowledge.workers`:1893 18 项、`knowledge.facade`:1613、`knowledge.routes`:1750）、`event-catalog.yaml`（knowledge 家族 4 事件 v1，K4 拥有 3 个 producer，§6 表）、`exception-ledger.yaml`（实测零 plan=24 属主行）、`b0-evidence.md:86`（repository/knowledge.go 显式归 24） | 签名冻结、事件口径、destination、例外 |
| `docs/architecture/passb/knowledge-process.md`（集成 worktree 版，含 B0.2/B0.3 注记） | 域 brief：scope 28 文件、边界目标、删除义务 3 条、`KnowledgeHousekeeping` 端口裁定 |
| 先行节点计划与 Brief（各分支 approved 产物，本节点前置输入）：`21-knowledge-ingest.md` + `briefs/b2-k-ingest.md`（R1-6/7/9 shim、`span_trace_seam_adapter.go` 接线义务）、`22-knowledge-retrieval.md` + `briefs/b2-k-retrieval.md`（§2 导出表、§3.3 compat 删除编排——**`writableFAQKnowledgeBase` 方法体须随 K4 knowledge.go 补迁或由 K4 属主接收**（:78）、§4 K4 调用点直连表（:93）、§5 推迟件 3-10 解除条件含 K4 迁移 knowledge_delete.go（:104）、§6 escapeLikeKeyword seam 收口（:117））、`23-knowledge-wikifaq.md` + `briefs/b2-k-wikifaq.md`（(a) A1-A11 装配切换表、(b) W1 宿主调用点表 11 符号、(c) D1 行——**「K4 knowledge.go 落位 process 后，KnowledgeService 实现体的 FAQ 面归宿按 24 计划收口（接口不变，delegates 换指向）」**（:78）、(d) faq/wiki seam 接线表（K4 目标：FinalizeSubtaskDetached/IsLikelyRateLimitError/RemoveSourceRef→process、parseTagIDs→process 落位导出）） | K4 收口义务的授权事实源与接口 |
| 实测代码（本 worktree `codex/passb-b2-k-process` @ `37eae710f`（=集成分支头，K1/K2/K3 未合入）；基线对齐后宿主 28 文件本体行号不变——前置分支只动自己文件与新增 compat，实测 K1/K2 分支 container.go diff 0 行、K3 分支仅 :93/:316 两处（Ruling 5 提前落地）） | 本计划全部签名与调用点行号出处（见 §5–§7 各表，撰写时逐条 grep/sed 实证） |

注意：主 checkout 的 `docs/architecture/passb/knowledge-process.md` 为 B0 前旧版，以集成 worktree 版为准（20 计划 §1 同律）。

## 2. 节点裁定与前置条件

**节点裁定（调度方原文，照录）：**

1. K4 在 ingest/retrieval 契约稳定后串行执行（framework:118-119）；worker 注册行禁改（与 conversation-queryhistory.md:39-42 义务 3 同律，本会话验证）。
2. BLOCKED（2026-09-23）：前置 b0 阻塞；解除条件：修复并 done b0 后恢复。BLOCKED（2026-09-24）：前置 b2-k0 阻塞；解除条件：修复并 done b2-k0 后恢复。BLOCKED（2026-09-25）：前置 b2-k-retrieval 阻塞；解除条件：修复并 done b2-k-retrieval 后恢复。

**BLOCKED 解除实证（撰写时实测，python3 读集成 worktree `execution-dag.json`）：** `b2-k-ingest` `status=done`+`review_status=approved`（head `20b9a7ca3`）、`b2-k-retrieval` `status=done`+`approved`（head `968d3d655`）；另 `b2-k-wikifaq` `status=done`+`approved`（head `ed156cd85`）、`b2-k0` done/approved（head `5bcb798621`）。三条 BLOCKED 记录对应前置均已闭环。

**前置条件核对（K4 开工门禁，逐条命令）：**

- [ ] **P1 冻结产物与前置分支在场**：`python3 -c "import json;d=json.load(open('docs/plans/passb/execution-dag.json'));print([(n['id'],n['status']) for n in d['nodes'] if n['id'] in ('b2-k0','b2-k-ingest','b2-k-retrieval','b2-k-wikifaq')])"` 预期全 `done`。
- [ ] **P2 基线对齐 merge**（Ruling 2026-09-24-WAVE-DEP-BASELINE）：按框架顺序以**独立 merge commits** 把 `codex/passb-b2-k-ingest` → `codex/passb-b2-k-retrieval` → `codex/passb-b2-k-wikifaq` 并入 `codex/passb-b2-k-process`（K1/K2 为 DAG 直接前置；**K3 一并纳入的依据**：framework:117-124 与 20 计划 §3 的集成分支合并缺省序 K1→K2→K3→K4，且 K3 Brief (c)/(d) 的 D1、W1、W2、H2 收口目标全部指向「K4 落位 process 后」——不合入 K3 则宿主残留 K3 属主 faq 5 文件的 57 个 `*knowledgeService` 方法（实测 `grep -c`：knowledge_faq.go 24 + knowledge_faq_batch.go 3 + knowledge_faq_create_guard.go 1 + knowledge_faq_import.go 27 + faq_clone_sync.go 2），K4 搬迁 `knowledgeService` 定义必然断链 K3 属主文件且无权处置）。**边扩展登记义务（conventions §9）**：DAG `depends_on` 不含 `b2-k-wikifaq`，本次纳入属基线对齐边扩展——K4.0 在报告中登记并在回报时建议协调者在 DAG `b2-k-process` 的 `notes`/`corrections` 记录「基线对齐纳入 K3（框架合并序 + K3 Brief 收口依赖，非 DAG 前置变更）」；本节点不修改 `execution-dag.json`。对齐后 `go build ./...`、`go test -count=1 ./internal/modules/knowledge/kbfreeze/` 绿；`git status` 干净再开工。module.go 冲突=两侧注册全保留；涉冻结签名取舍的冲突停下升级（conventions §5）。**modify/delete 冲突预置裁定**：K2 侧已 `git mv` 的文件（如 `kb_activity_test.go`）若与 K3 侧改动相遇，采纳 K2 侧迁移结果（K3 对此类文件仅注释引用、实测零文本改动）；K3 垫片 `knowledge_faq_k3_test_support_shim_test.go` 的注释引用（`kb_activity_test.go:68/:79`）在合并后成为悬空引用，属预期（该垫片处置见 §5.3b）。
- [ ] **P3 门禁工具可用**：`Makefile` 的 `verify-module-moves` / `check-backend-architecture` 目标存在（实测 grep）。
- [ ] **P4 K1/K2/K3 落位包与宿主过渡物在场**（基线对齐后核验）：`ls internal/modules/knowledge/{ingest,retrieval/app,wiki,faq,kbfreeze}` 存在；宿主 `internal/application/service/` 下 `wiki_k3_compat.go`、`wiki_k3_ctor_compat.go`、`knowledge_faq_k3_delegate.go`、`kbretrieval_passb_compat.go`、`span_trace_seam_adapter.go`、`chunk_ingest_shim.go`（K1）存在；`internal/handler/` 下 `faq_k3_compat.go`、`wiki_page_k3_compat.go` 存在；`internal/application/repository/` 下 `kbretrieval_passb_compat.go`、`kbretrieval_passb_compat_test.go`、`wiki_k3_repo_compat.go` 存在。
- [ ] **P5 特征化测试基线（T0，spec §14.1）**：搬迁前跑 §8 随迁测试所在宿主包并记录通过集合与计数（K4.0 Step 3）。

## 3. 范围、目标布局与推迟件

### 3.1 本节点 28 文件（ownership-matrix plan=24-knowledge-process 全量，与 knowledge-process.md:7-28 逐行一致；本 worktree `ls` 实测全部在场，行数附后）

| 层 | 文件（`internal/application/` 前缀省略；行数为撰写时实测） |
|---|---|
| repository 5 | `knowledge.go`（1169）、`knowledge_span_repo.go`（271）、`knowledge_tag.go`（159）、`knowledge_transfer.go`（74）、`knowledgebase.go`（253，**推迟，§3.3**） |
| service 19 | `knowledge.go`（1129，**knowledgeService 定义 :49**）、`knowledge_auto_tag.go`（380）、`knowledge_clone_move.go`（1572）、`knowledge_create.go`（1419）、`knowledge_delete.go`（777）、`knowledge_delete_plan.go`（169）、`knowledge_housekeeping.go`（399）、`knowledge_index_content.go`（21）、`knowledge_post_process.go`（730）、`knowledge_process.go`（4091）、`knowledge_process_config.go`（328）、`knowledge_reparse_scope.go`（60）、`knowledge_replace.go`（252）、`knowledge_span_tracker.go`（879）、`knowledge_summary_refresh.go`（151）、`knowledge_task_options.go`（27）、`knowledge_transfer.go`（530）、`knowledge_util.go`（521）、`knowledge_write.go`（141） |
| handler 4 | `kb_access.go`（81）、`knowledge.go`（2770，**KnowledgeHandler 定义 :30**）、`knowledge_download.go`（361，`*KnowledgeHandler` 方法扩展文件，:59 BatchDownloadKnowledge）、`task_progress_auth.go`（27） |

**方法分布实测（`grep -l "func (s \*knowledgeService)"`）**：挂 `*knowledgeService` 方法的 11 文件 = knowledge.go、knowledge_clone_move.go、knowledge_create.go、knowledge_delete.go、knowledge_delete_plan.go、knowledge_process.go、knowledge_reparse_scope.go、knowledge_replace.go、knowledge_summary_refresh.go、knowledge_transfer.go、knowledge_util.go；不挂的 8 文件 = auto_tag（独立 `KnowledgeAutoTagService`:39）、housekeeping（独立 `HousekeepingService`，Start:73/Stop:99/runSweep:112）、index_content（包级 `buildKnowledgeIndexContent`:12）、post_process（独立 `KnowledgePostProcessService`:23）、process_config、span_tracker（`SpanTracker`:85/`NewSpanTracker`:172）、task_options、write（包级 write-family）。**Go 方法集规则：11 文件必须与 knowledgeService 定义同批迁移**（K4.3）。

### 3.2 目标布局（matrix destination `internal/modules/knowledge/process` 的三层具体化，K2 计划 §3.2 先例）

同名文件不可同目录（repository/knowledge.go、service/knowledge.go、handler/knowledge.go 三者），按宿主同构分三个子包，落位包名对齐宿主便于 diff 审查：

| 宿主包 | 落位包（import path，package 名） | 文件 |
|---|---|---|
| `internal/application/repository` | `internal/modules/knowledge/process/repository`（package repository） | 4 文件（§3.3 推迟件除外） |
| `internal/application/service` | `internal/modules/knowledge/process`（package process） | 19 文件（含推迟件，见 §3.3 编排） |
| `internal/handler` | `internal/modules/knowledge/process/handler`（package handler） | 4 文件 |

`git mv` 保留原文件名（K3 的 handler/service 同名冲突场景在本布局下不存在）；`git diff --summary --find-renames` 须逐文件 R100。布局具体化在 K4.1 首个搬迁 commit 落定并写入 Integration Brief 与报告（matrix destination 词干不变）。

### 3.3 推迟件（1 个；Ruling 2026-09-25-DEFERRED-FILE-SPLIT，先例=22 计划 §3.3 两推迟件 + K2 实际执行时按同 Ruling 收缩为 13 个推迟件，评审 approved）

| 推迟文件 | 根因（base 实测证据） | 解除编排（写入 Integration Brief，ib2 执行） |
|---|---|---|
| `internal/application/repository/knowledgebase.go`（253 行） | 与 `repository/model_usage.go`（12-commercial 属主，ib1 零实施未迁移，K0 差异①）**单向未导出消费**：`CountModelUsages`（:216 经 `scopeKnowledgeBasesByModelID(query, modelID)`）、`ListModelUsages`（:235 同上；:242 经 `knowledgeBaseModelUsageBindings(row, modelID)`）——两符号定义于宿主 `internal/application/repository/model_usage.go:8/:57` 且未导出（K0 §6.2 组 E 实测）。导出义务属 12-commercial/ib1 补课（20 计划组 E 裁定 + K0.3 差异①上报在案），K4 不得自行导出对方文件符号；process/repository import 宿主包也不可见未导出符号。推迟该文件后其余 4 个 repository 文件零断链（实测 grep：knowledge/span_repo/tag/transfer 四文件对 `knowledgeBaseRepository`/`ErrKnowledgeBaseNotFound`/model_usage 族零引用）。 | ib2 与 12-commercial 补迁同窗：commercial 侧落位并导出窄端口后，本文件按 K4.1 同法迁移（含 manifest 行删除）；期间留守宿主零改动，`NewKnowledgeBaseRepository`（:51 附近，container.go:202 Provide）与哨兵 `ErrKnowledgeBaseNotFound`（:13）天然编译——哨兵留守反而保护 wiki 闭包（K3 Brief (d) `IsKnowledgeBaseNotFound`）、`handler/knowledge.go:2349/:2368`、faq 包哨兵 import 三处消费面零改动。 |

推迟不改变该文件的 ownership-matrix 属主（仍 24-knowledge-process）与 `delete_barrier: ib2`；manifest 行保留（Ruling 6 第 4 点：未迁移不删行）；推迟项在报告「未完成项」如实列出（conventions §1.2），不视为节点失败。**B5 的 396 口径不减免**。

## 4. 写入所有权与禁改清单

**可写**：

- §3.1 的 27 个物理搬迁文件及其随迁 `_test.go`（§8.1，判定规则+已确认清单；K4.0 落盘最终清单）；
- 三个落位新包及包内新文件（`faq_delegate.go`（D1 收口产物）、`semantic_scope_guard.go`（guard 第四轨，§5.3a）、哨兵/函数导出包装与包内 R3 改名；`model_usage_seam` 不需要（推迟件方案））；
- 3 个宿主兼容新文件 `internal/application/repository/kbprocess_passb_compat.go`、`internal/application/service/kbprocess_passb_compat.go`、`internal/handler/kbprocess_passb_compat.go`（**横向目录新生产文件必须同 commit 登记进 `docs/architecture/moves/knowledge.yaml` legacy_files（`passb_task: B-knowledge`，Ruling TRANSITION-SHIM-ROW-REGISTRATION 成对补行），否则 legacy-guard 报诊断——K2 计划 §4 实测先例**）；
- `docs/architecture/moves/knowledge.yaml`（仅本节点 27 行的删除 + compat 行登记，Ruling LEGACY-ROW-OWNERSHIP 行级删除权）+ `internal/modules/knowledge/legacy/README.md` 镜像同步；
- **K3/K2 Brief 点名授权的三处收口**（授权原文见 §5.3）：(1) `internal/application/service/knowledge_faq_k3_delegate.go`（D1，K3 Brief (c) :78「K4 knowledge.go 落位 process 后，KnowledgeService 实现体的 FAQ 面归宿按 24 计划收口」）的同 commit 删除；(2) `internal/application/service/kbretrieval_passb_compat.go`（K2 Brief §3.3 :78「方法体须随 K4 knowledge.go 补迁或由 K4 属主接收」）中 `writableFAQKnowledgeBase` 方法段（:190 注释/:194 func 起）的迁出（**该文件其余段零触碰**——含 :29 `semanticScopeGuard` 宿主同形定义，其留守消费方 identity 4 文件与 K2 推迟件继续依赖）；(3) `internal/application/service/knowledge_faq_k3_test_support_shim_test.go`（K3 测试垫片，K3 Brief (b) :57「K2/K4 域改写测试后垫片删除」为删除前置，§5.3b）随 knowledge_write_access_test.go 拆分改写同 commit 删除；另 `internal/application/repository/kbretrieval_passb_compat_test.go`（K2 测试垫片，K2 Brief :80「knowledge_tag_test.go 随 K4 迁移或直连改写」为删除前置）随 knowledge_tag_test.go 迁移同 commit 删除 + 其 manifest 行删除（两垫片均无 manifest 行或行随文件删，以 K4.0 Step 2 实测为准）；
- `tools/architectureguard/check.go` importExceptions 数据行 + `docs/architecture/passb/exception-ledger.yaml` 属主行（仅限 K4.5 按 Ruling IMPORT-EXCEPTION-REGISTRY 执行的精确登记）；
- 本节点产物：`docs/architecture/evidence/passb/b2-k-process.md`、`docs/plans/passb/reports/b2-k-process.md`、`docs/architecture/passb/briefs/b2-k-process.md`（conventions §1.1）。

**禁改**（conventions §3，违者节点失败）：`internal/router/router.go`、`internal/router/routes_knowledge.go`、`internal/router/task.go`、`internal/router/sync_task.go`、`internal/router/files.go`、`internal/container/**`（含 `recover_pending_wiki_tasks.go`）、`internal/bootstrap/**`、migration 编号、`go.mod`/`go.sum`、生产 SQL、既有迁移文件；`docs/architecture/passb/{ownership-matrix,contracts,event-catalog}.yaml`（barrier 回写）；`internal/modules/knowledge/module.go`（门面注释仅 b0 与集成节点可写）；`internal/types/**`（R0 唯一事实源）；`internal/modules/knowledge/{kbfreeze,ingest,retrieval,wiki,faq}/**`（K0/K1/K2/K3 产物，含其落位包与宿主 compat 中除 §4 授权三处外的全部内容）；K1 宿主产物 `span_trace_seam_adapter.go`、`chunk_ingest_shim.go`、`parser_url_security.go`（K4 只经宿主 compat 保持其编译，改写归 ib2）；K2 推迟件（`semantic_model.go`、`handler/knowledgebase.go`、knowledgebase_search 族 8 文件、`kbshare.go`、`tag.go`、`tag_access.go`、`repository/kbshare.go`）与 K2 compat 除授权段外；conversation/identity/insights/datasource/airesource 全部属主文件（含 `tenant.go`、`message.go`、`session.go`、`web_search_state.go`、`evaluation.go`、`datasource_service.go`——它们的 K4 符号调用点经宿主 shim 保护，ib2 改写）；`cmd/desktop`、`docreader`、`client`。

**搬迁机械律**：每个生产文件同 commit 删除 manifest `legacy_files` 对应行（modulemove 对「行在盘缺」报 `legacy-file: not found` 退 1，10-identity.md:37 实测）；函数体一行不改（M2 纯移动），导出改名/兼容层重线独立于纯移动说明但可同 commit 呈现（M3，22 计划先例：`git diff --summary` rename 识别 + 函数体 diff 为零（package/import/改名除外））；禁止混合关注点（conventions §4）。

## 5. 耦合面裁定（核心机制；全部实测，签名与 20 计划 §6.2 冻结表一致处不重述）

### 5.1 R1 导出义务一：20 计划组 C 三个符号（K4 定义、宿主他 owner 消费 → 落位包导出改名 + 宿主 compat 一行委托）

| 符号（定义 file:line，实测） | 签名（实测） | 留守消费方（grep 实证） | 导出名 |
|---|---|---|---|
| `escapeLikeKeyword`（repository/knowledge.go:22） | `func escapeLikeKeyword(keyword string) string` | identity `repository/tenant.go:85/:90`；conversation `repository/message.go:215`、`repository/session.go:209`；K2 落位包 `retrieval/app/repository/escape_like_seam.go` 同形副本（ib2 收口为单一实现，K2 Brief §6 :117） | `EscapeLikeKeyword` |
| `withKnowledgeCleanup`（service/knowledge_delete_plan.go:22） | `func withKnowledgeCleanup(ctx context.Context, tenant uint64, bindings map[string]string) context.Context` | datasource `datasource_service.go:885`；K4 内 R3（knowledge_delete.go:766、knowledge_delete_plan.go:71） | `WithKnowledgeCleanup` |
| `deleteReferencedKnowledge`（service/knowledge_delete_plan.go:36） | `func deleteReferencedKnowledge(ctx context.Context, svc interfaces.KnowledgeService, expectedKB string, ids []string) error`（多行签名，:36 起） | conversation `message.go:449/:469`、`session.go:687/:764/:821`、`web_search_state.go:124`；insights `evaluation.go:366`；K4 内 R3（knowledge_transfer.go:357）；K4 属主测试 `document_write_access_test.go:525/:530`（随迁）。**airesource 调用点实测为零**（全仓 `grep -rn deleteReferencedKnowledge internal/` 无 airesource 命中）——DAG ppc `airesource→knowledge 1 site` 为符号级扫描伪影（与 K0.1 已消歧的 `getParserEngineOverridesFromContext` 同型），K4.0 上报协调者修订 ppc，不构成额外义务 | `DeleteReferencedKnowledge` |
| `isValidFileType`（service/knowledge_util.go:62） | `func isValidFileType(filename string) bool` | **无跨 owner 调用方**（K0.1 复核消歧：conversation `attachment_processor.go:80` 调同包本地同名函数 :305）；K4 内 R3（knowledge_create.go:76） | 直接随迁，无需导出；同名并存事实已在 Integration Brief 登记（K0 组 C 行） |

### 5.2 R1 导出义务二：K1/K2/K3 已落位消费面点名等待的 process 导出（各 Brief (d)/(b) 表「ib2 后目标」列的 K4 侧前置）

| 符号（定义 file:line，实测） | 消费方（落位包/宿主过渡物） | 导出名 |
|---|---|---|
| `attemptSuperseded`（knowledge.go:202，包级 `func attemptSuperseded(ctx context.Context, tracker SpanTracker, knowledgeID string, attempt int) bool`） | K1 `ingest` seam（21 计划 R1 表）经宿主 `span_trace_seam_adapter.go` 的 `AttemptSupersededProvider` | `AttemptSuperseded` |
| `finalizeSubtaskDetached`（knowledge.go:235，包级多行） | K3 `wiki.Seams.FinalizeSubtaskDetached`（W2 `wikiK3Seams()` 现接宿主符号，K3 Brief (d)） | `FinalizeSubtaskDetached` |
| `isLikelyRateLimitError`（knowledge_process.go:3902，包级 `func isLikelyRateLimitError(err error) bool`） | K3 `wiki.Seams.IsLikelyRateLimitError`（同上） | `IsLikelyRateLimitError` |
| `removeSourceRef`（knowledge_delete.go:346，包级 `func removeSourceRef(refs types.StringArray, knowledgeID string) types.StringArray`） | K3 `wiki.Seams.RemoveSourceRef`（同上） | `RemoveSourceRef` |
| write-family 4 函数：`writeResourceIDs`（knowledge_write.go:17）、`writeExecutionTenant`（:32）、`loadKnowledgeWrite`（:60）、`loadKnowledgeWriteBatch`（:93） | K1 `ingest.KnowledgeWriteGuard` seam（21 计划 §6.3 表；宿主 `span_trace_seam_adapter.go` `KnowledgeWriteGuardProvider()`，其文件头注释明文「K4 搬迁 … 后指到其导出包装」） | `WriteResourceIDs` / `WriteExecutionTenant` / `LoadKnowledgeWrite` / `LoadKnowledgeWriteBatch` |
| `enqueueSummaryRefresh`（knowledge_summary_refresh.go:83，包级） | K1 seam（同上 adapter 的 `EnqueueSummaryRefreshProvider`） | `EnqueueSummaryRefresh` |
| `buildKnowledgeIndexContent`（knowledge_index_content.go:12，包级 `func buildKnowledgeIndexContent(knowledge *types.Knowledge, content string) string`） | K1 adapter 同文件头点名 | `BuildKnowledgeIndexContent` |
| `getFileType`（knowledge_util.go:89）、`normalizeFileExtension`（:47）、`isDataTableFileType`（:67） | K1 R1-9 宿主 shim 以函数值传入（`enqueueDataTableSummaryIfNeeded` 转发，21 计划 R1-9：调用点 knowledge_create.go:295/:746、knowledge_process.go:2629/:2682） | `GetFileType` / `NormalizeFileExtension` / `IsDataTableFileType` |
| `parseCommaSeparatedTagIDs`（handler/knowledge.go:2546，包级） | K3 `faq_k3_compat.go`（H2）构造参 seam（K3 Brief (d)「process 落位导出」） | `ParseCommaSeparatedTagIDs` |
| `requireTaskProgressTenant`（handler/task_progress_auth.go:14，包级 `func requireTaskProgressTenant(ctx context.Context, taskID string) error`） | 同上（K3 Brief (d)「最小动作 = faq_handler 构造参保留直引」→ 直引前提=宿主 compat 委托在位） | `RequireTaskProgressTenant` |
| 已导出符号（无需改名）：`NewKnowledgeService`（service/knowledge.go:100）、`NewKnowledgePostProcessService`（knowledge_post_process.go:34）、`NewKnowledgeAutoTagService`（knowledge_auto_tag.go:49）、`NewHousekeepingService`（knowledge_housekeeping.go:57）、`NewKnowledgeRepository`（repository/knowledge.go:51）、`NewKnowledgeSpanRepository`（knowledge_span_repo.go:54）、`SpanTracker`/`NewSpanTracker`（knowledge_span_tracker.go:85/:172）、`ErrKnowledgeNotFound`（repository/knowledge.go:15）等 | container.go:202-204/:343/:374/:412/:413/:641/:718、K1 adapter、K2 垫片、K3 W2、router（经接口） | 落位包原名；**宿主 compat 留 var/type 别名保 container 与 K1/K3 过渡物编译**（K2 §5.5 模式；K3 container.go:316 提前落地先例证明 K4 无需动 container） |

机制（20 计划 §6.1 R1）：落位包内改名为首字母大写同义（函数体一行不改，包内调用点随 R3 清单同步改名），宿主 compat 文件留一行委托（如 `func escapeLikeKeyword(keyword string) string { return repository.EscapeLikeKeyword(keyword) }`——注意宿主 repository/service/handler 三包各建 compat，委托目标为对应落位子包）。**跨 owner 调用点文件本身零改动**，重线与 shim 删除走 Integration Brief 由集成工程师在 ib2 执行（conventions §1.3、§7.2）。

### 5.3 R1 导出义务三：knowledgeService 迁移的联动收口（本计划最重机制；两处授权的事实源）

**背景（实测）**：`knowledgeService`（service/knowledge.go:49，字段清单 :49 起约 40 项，含 `semanticScopeGuard` 内嵌（K2 compat 宿主同形定义 :29）、`audit interfaces.AuditLogService`、`quotaGuard commercial.ResourceQuotaGuard`、`memFAQProgress/memFAQRunningImport sync.Map` 等）在基线对齐后的宿主中，方法声明残留在两个**过渡兼容文件**（均为前序节点按 Ruling 产出的宿主 shim，非业务文件、manifest 在册、删除点 ib2）：

| 文件 | 挂 `*knowledgeService` 的方法 | 授权（Brief 原文） |
|---|---|---|
| K3 `knowledge_faq_k3_delegate.go`（D1） | (a) 冻结接口 FAQ 面 15 方法（ListFAQEntries…UpdateLastFAQImportResultDisplayStatus）一行委托 `s.faqSvc().X(...)`；(b) 4 个未导出方法 `validateFAQKnowledgeBase`/`buildFAQStatusSyncPlan`/`indexFAQChunks`/`syncFAQChunkStatusBatch`（调用方实测：K2 宿主 compat `writableFAQKnowledgeBase` 方法体内 `kbretrieval_passb_compat.go:198`（该段随本计划 §5.3.2 迁出后此调用随段消失；K3 计划 D1 表原引 `knowledgebase_access.go:42` 为 K2.4 迁出前旧宿主坐标，勘误）与 K4 `knowledge_clone_move.go:700/:855/:885`）；(c) `faqSvc()` 构造（以 knowledgeService 17 个字段 + seam 闭包构造 `*faq.Service`） | K3 Brief (c) D1 行：「K4 `knowledge.go` 落位 process 后，KnowledgeService 实现体的 FAQ 面归宿按 24 计划收口（接口不变，delegates 换指向）」 |
| K2 `kbretrieval_passb_compat.go`（:190 注释/:194 func 起） | `writableFAQKnowledgeBase`（方法体自 K2.4 从 knowledgebase_access.go:38 再归置迁来，接收者类型留守宿主；体内 :198 调 `s.validateFAQKnowledgeBase`——D1 (b) 委托的链路终点） | K2 Brief §3.3 :78：「`writableFAQKnowledgeBase` 方法体须随 K4 knowledge.go 补迁或由 K4 属主接收」 |
| K3 `knowledge_faq_k3_test_support_shim_test.go`（`_test.go` 垫片） | 4 个方法 `UpdateFAQEntryStatus`/`UpdateFAQEntryTag`/`resolveTagID`/`buildFAQTagResolver`（一行委托 `s.faqSvc().X(...)`，不在冻结接口面）+ 2 个包级纯函数 `faqImportCompletedOutcome`/`faqImportActivityDetails`（委托 faq 包导出）。消费者实测（K3 分支 grep）：唯一方法调用方=留驻白盒测试 `knowledge_write_access_test.go:200/:208/:362/:364`；纯函数消费方 `kb_activity_test.go:68/:79` 已随 K2 迁走（合并后悬空引用，见 P2 预置裁定） | K3 Brief (b) :57：「留驻宿主测试……——K2/K4 域改写测试后垫片删除」（Ruling 2026-09-24-TEST-SUPPORT-SHIM，remove_at=ib2 先到先删；K4 拆分改写即到达删除点） |

**收口机制（K4.3 执行，与 11 文件搬迁同 commit）：**

1. **落位包新建 `internal/modules/knowledge/process/faq_delegate.go`（K4 新文件）**：D1 的方法集按原文本迁入——(a) 15 个 FAQ 接口方法一行委托 `s.faqSvc().X(...)`；(b) 4 个未导出方法保留未导出名（`buildFAQStatusSyncPlan` 等在 process 包内被 knowledge_clone_move.go 同包直调，K3 Brief (d) 的 `faq.(*Service).BuildFAQStatusSyncPlan` 等导出名在 faq 包已存在，process 内委托直接调 `s.faqSvc().BuildFAQStatusSyncPlan(...)`——**以 D1 现文本为准逐一迁移，委托目标即 faq 包导出方法**）；(c) `faqSvc()` 构造照录（字段访问随 knowledgeService 同包迁移后天然合法；seam 闭包中 `recordKBActivity`/`kbActivityTrigger`/`withKBActivityTask`/`kbActivityAppendSampleTitles`/`resolveKBReadTenant` 改调 K2 落位包导出名 `kbretrieval.RecordKBActivity` 等（process import `internal/modules/knowledge/retrieval/app`，模块内合法），`writableFAQKnowledgeBase` 闭包改调第 2 点的方法）。**编译器即验收**：`interfaces.KnowledgeService` 冻结接口的 FAQ 切片若有表外方法，编译错误枚举后按同样的一行委托补齐（报告登记），禁止改接口。
2. **`writableFAQKnowledgeBase` 方法接收**：方法文本（函数体一行不改）从 K2 compat 的 :190/:194 段迁出，落 process 包同名方法（挂 `*knowledgeService`——类型已同包；体内 `s.validateFAQKnowledgeBase` 调用经本包 faq_delegate (b) 继续解析）。K2 compat 该段删除（授权见上表），文件其余段零触碰。
3. **宿主 D1 文件整文件删除 + manifest 行删除**（同 commit）：其全部职责已由落位包 `faq_delegate.go` 承接；`interfaces.KnowledgeService` 的宿主实现消失——container.go:343 经宿主 service compat 的 `var NewKnowledgeService = process.NewKnowledgeService` 继续解析（container 零改动），返回类型 `*process.knowledgeService` 未导出但 dig 按接口注入（`NewKnowledgeService` 形参/返回实测：返回 `*knowledgeService`（service/knowledge.go:100 起），dig Provide 后按 `interfaces.KnowledgeService` 消费——与搬迁前同一类型形状，接口满足性由编译器强制）。
4. **K3 测试垫片删除 + `knowledge_write_access_test.go` 拆分改写**（同 commit，K3 Brief (b) :57 删除前置）：该测试按 K2 Brief 推迟件 12-13 既定编排拆分——(i) write 面用例（write-family/loadKnowledgeWrite 守卫语义，含借 FAQ 方法作写入载体的用例 :200/:208/:362/:364）随迁 process 包，垫片 4 方法的调用点改同包白盒 `svc.faqSvc().UpdateFAQEntryStatus(...)`（与垫片一行委托等价形态，断言零变化）；(ii) tag 面用例留宿主原文件（待 K2 推迟件 tag.go 补迁时随迁，残差登记 Brief）。拆分后垫片 6 符号消费者清零（纯函数 2 个的消费方已随 K2 迁走），垫片整文件删除。**判定以 K4.0 Step 4 落盘清单为准**：若 :200/:208/:362/:364 用例经判定属纯 FAQ 行为面（被测 faq.Service 而非写入守卫），则改留宿主并直连 `faq` 包导出构造——两路径均满足「垫片消费者清零后删除」，报告记录所选路径。

### 5.3a `semanticScopeGuard` 第四轨（K4.3 必备前提；确定性编译失败的唯一解除路径）

**实测断链面**：`knowledgeService` 以无字段名内嵌 `semanticScopeGuard`（service/knowledge.go:50）；原类型定义在 `semantic_scope.go:31`（K2.3 已迁出），宿主仅 `kbretrieval_passb_compat.go:29` 留**未导出**同形定义（§4 授权只覆盖该文件 `writableFAQKnowledgeBase` 段，guard 段不可动）。K4.3 `git mv` knowledge.go 至 process 包后，包内 `semanticScopeGuard` 无定义、跨包不可见未导出类型——`go build` 确定性失败。

**收口机制（K4.3 Step 2 落地）**：process 包新建 `semantic_scope_guard.go`——结构体 `semanticScopeGuard`（单字段 `semanticInvalidator interfaces.SemanticScopeInvalidator`）+ `SetSemanticScopeInvalidator` + 5 个 invalidate 方法（KB/Tenant/User/Organization/Transfer），**逐字对齐 K2 落位包 `retrieval/app/semantic_scope_guard.go` 现文本**（该文件自带注释「多属主重复 helper 族，ib2 与 identity 导出的同形 seam 一并收口为单一定义」，22 计划 §5.4；对齐基准以 K4.0 Step 5 实读该文件为准）。这是 guard 收口模式的**第四轨**（K2 落位包轨 + K2 宿主 compat 轨 + identity seam 轨之后的 process 轨），文件头注释登记 `// Pass B 过渡 seam（24-knowledge-process）：guard 第四轨，ib2 与 identity/K2/宿主 compat 三轨一并收口为单一定义`；Integration Brief (c) 项登记四轨统一编排（ib2）。**宿主 compat 的 guard 段不动**：留守嵌入方（identity tenant.go:57/user.go:100/organization.go:47/tenant_member.go:78 + K2 推迟件 knowledgebase.go:34/kbshare.go:38）继续经宿主同形定义编译。

### 5.3b 汇总断链矩阵（基线对齐后宿主挂 `*knowledgeService` 的全部残留 → K4.3 同 commit 收口）

| # | 宿主残留 | 处置（上文机制） | 授权 |
|---|---|---|---|
| 1 | K3 D1 的 19 方法 + `faqSvc()` | 迁入 `process/faq_delegate.go`，宿主 D1 删除 | K3 Brief (c) :78 |
| 2 | K2 compat `writableFAQKnowledgeBase` 段（:190/:194） | 方法文本迁入 process 包同名方法，段删除 | K2 Brief §3.3 :78 |
| 3 | K3 测试垫片 6 符号（4 方法 + 2 纯函数） | `knowledge_write_access_test.go` 拆分改写后垫片删除 | K3 Brief (b) :57（Ruling TEST-SUPPORT-SHIM） |
| 4 | knowledge.go:50 `semanticScopeGuard` 内嵌 | process 第四轨同形定义（§5.3a） | 22 计划 §5.4 多属主重复 helper 族既定收口模式 |

四项全部在同一搬迁 commit 内闭环后，`grep -c "func (s \*knowledgeService)" internal/application/service/` = 0 且 `go build ./...` 绿——K4 分支独立编译不依赖任何 ib2 动作（Ruling 5 无需动用；装配切换仍全部留 ib2）。

### 5.4 R2/seam 义务（K4 消费他属主未导出符号；20 计划组 E）

| K4 消费面 | 他属主符号 | 处置 |
|---|---|---|
| `repository/knowledgebase.go:216/:235/:242` | `scopeKnowledgeBasesByModelID`/`knowledgeBaseModelUsageBindings`（12-commercial，未导出） | **整文件推迟（§3.3）**，替代 R2 seam（K0 组 E 允许的更保守路径；Ruling 6 先例） |
| K4 service 文件对 K2 已导出符号的裸名调用：`recordKBActivity`/`kbActivityTrigger`/`withKBActivityTask`/`kbActivityAppendSampleTitles`（knowledge_create.go:285/:489/:742/:882/:1020/:1148、knowledge_clone_move.go:441/:1057、knowledge_delete.go:728、knowledge_process.go:4034、knowledge_replace.go:206 等，20 计划组 B 全表）、`kbReadPermissions`（knowledge.go:830）、`requireKBWrite`（knowledge.go、knowledge_delete_plan.go、knowledge_write.go）、`withKBWriteTenantInfo`（knowledge_create.go、knowledge_delete_plan.go） | K2 落位包 `kbretrieval` 已导出 `RecordKBActivity`/`KBActivityTrigger`/`WithKBActivityTask`/`KBActivityAppendSampleTitles`/`KBReadPermissions`/`RequireKBWrite`/`WithKBWriteTenantInfo`（K2 Brief §2 表） | K4 串行在 K2 后 → 搬迁时**直接改 import + 改调导出名**（K2 Brief §4 :93 预排的「K4 调用点 ib2 直连」由 K4 在自己文件内提前完成——文件属主是 K4，改自己的调用点不属越权；宿主 compat 委托链只为未搬迁的他 owner 服务） |
| K4 文件对 K3 已导出符号的裸名调用：`previewText`（knowledge_process.go:1226/:1289/:1336/:1759/:2097）、`enqueueWikiIngestTrigger`（knowledge_post_process.go:268/:435）、`enqueueWikiRetract`（knowledge_delete.go:233）、`extractRealText`（knowledge_post_process.go:729）、`newWikiIngestPendingOp`（knowledge_post_process.go:317）、`minTextContentRunes`（knowledge_process.go:844/:847 读点）、`realTextRuneCount`（knowledge_process.go:843/:942）、`uniqueWikiFolderIDs`（knowledge_delete.go:236）、`EnqueueWikiIngest`（knowledge_clone_move.go:1237/:1284）、`WikiRetractPayload`/`WikiPendingOp`/`WikiDeletedTombstoneKey`/`wikiDeletedTTL`（K3 Brief (b) 表） | K3 `wiki` 包导出 `PreviewText`/`EnqueueWikiIngestTrigger`/`EnqueueWikiRetractWithError`/`ExtractRealText`/`NewWikiIngestPendingOp`/`MinTextContentRunes`（导出 var）/`RealTextRuneCount`/`UniqueWikiFolderIDs`/`EnqueueWikiIngest`/`WikiRetractPayload`/`WikiPendingOp`/`WikiDeletedTombstoneKey`/`WikiDeletedTTL` | 同上：process import `internal/modules/knowledge/wiki` 直连；`minTextContentRunes` 读点直连 `wiki.MinTextContentRunes`（**单一事实源**：迁走的 `knowledge_summary_test.go` 写点同改，消除 K3 Brief (b) 登记的 var 双副本——ib2 无需再收口该行） |
| K4 service 文件对 K1 已导出符号的裸名调用：`isFinalAsynqAttempt`（knowledge_process.go:1125/:1497/:1868）、`NewChunkExtractTask`（knowledge_post_process.go:418）、`enqueueDataTableSummaryIfNeeded`（knowledge_create.go:295/:746、knowledge_process.go:2629/:2682）、`ErrChunkRevisionConflict`（knowledge_process.go:2282） | K1 `ingest` 包导出 + 宿主 R1 shim（21 计划 R1-1/6/7/9 表） | 同上直连 `ingest.IsFinalAsynqAttempt`/`ingest.NewChunkExtractTask`/`ingest.EnqueueDataTableSummaryIfNeeded`（或其实参形态，以 K1 落位包实际导出签名与宿主 shim 文本为准，K4.0 Step 4 核对登记）/ `ingest.ErrChunkRevisionConflict`（var 同一实例，`errors.Is` 语义不变） |

### 5.5 新显形跨模块 import（K4.5 按 Ruling IMPORT-EXCEPTION-REGISTRY 登记精确豁免）

搬迁使下列既有横向包 import 变为 module→横向（architectureguard forbidden-import；faq→repository 先例已登记同类豁免，K3 计划 §10.4）。实测种子对（importer=落位文件精确路径，imported=精确 import 路径；K4.5 Step 1 全量清点多退少补）：

| 落位文件 | import（base 行号实测） |
|---|---|
| `process/knowledge.go` | `internal/application/repository`（:15） |
| `process/knowledge_process.go` | `internal/application/repository`（:13）、`internal/common`（:14）、`internal/modules/knowledge/searchutil`（:22，模块内合法，免登记） |
| `process/knowledge_span_tracker.go` | `internal/application/repository`（:35） |
| `process/knowledge_util.go` | `internal/application/service/file`（:15，filesvc） |
| `process/repository/knowledge.go`、`process/repository/knowledge_span_repo.go` | `internal/common`（:9/:8） |
| `process/handler/kb_access.go`、`process/handler/knowledge.go` | `internal/application/repository`（:6/:13——消费 `repository.KnowledgeSpanRepository` 接口形参与 `repository.ErrKnowledgeBaseNotFound` 哨兵（:37/:48/:2349/:2368）；哨兵与接口随推迟件留守宿主，直引合法需豁免登记） |

程序：独立 commit 在 `tools/architectureguard/check.go` importExceptions 追加精确 file→package 条目（Reason 一行、PassBTask=`K4.5`），同窗 exception-ledger.yaml 追加行（owner=`24-knowledge-process`、remove_at=ib2、reason=预存横向包耦合，Pass B 搬迁显形），机械计数修正 + §8 基线登记（evidence 登记原因与 Ruling 引用）。禁通配、禁新逻辑。**优先替代评估义务**：哨兵/接口型消费（handler 侧）可评估改为 process 包内 seam/别名注入以减少豁免条目——二选一，报告记录所选路径与理由（K3 计划 §10.4 同款义务）。

## 6. 公共可观察行为与兼容要求（冻结面，零变化）

| 冻结面 | 事实源 | 兼容要求 |
|---|---|---|
| **18 knowledge worker 注册** | contracts.yaml `knowledge.workers`（frozen）；注册位 task.go:266-320、sync_task.go:143-163 | 注册行零改动（DAG shared_resources「worker 注册行——集成工程师独占，模块侧只交付 RegisterWorkers」）。K4 面 11 个全部经 `params.KnowledgeService`（interfaces）或 `params.KnowledgePostProcess`/`KnowledgeAutoTag`（`interfaces.TaskHandler`，dig.Name `knowledgePostProcess`/`knowledgeAutoTag` 消歧，container.go:412/:413 Provide 不变）分发；`TypeIndexDelete`（TagService）、`TypeKBDelete`（KnowledgeBaseService）非 K4 文件实现（K2 推迟件留守） |
| **`interfaces.KnowledgeService` 全签名** | contracts.yaml `knowledge.service`:1773（frozen，含 `ProcessDocument`/`ProcessManualUpdate`/`ProcessFAQImport`/`ProcessQuestionGeneration`/`ProcessSummaryGeneration`/`ProcessKBClone`/`ProcessKnowledgeMove`/`ProcessKnowledgeListDelete`/`ProcessKnowledgeListReparse` worker 切片 + FAQ 面 + 全部 CRUD/检索） | 接口零改动（`git diff --name-only` 不含 `internal/types/`）；实现体（`*knowledgeService`）随迁后经编译器强制继续满足全接口（FAQ 面经 §5.3 faq_delegate） |
| **asynq 语义** | knowledge-process.md:50-51「删除与索引语义是外部契约：asynq 任务类型、幂等/重试语义、Redis/Lite 双池注册行为零变化」；spec §7 | 任务 payload 结构（`types.KnowledgeProcessOverrides` 等 R0 冻结）、MaxRetry/Timeout/TaskID 去重、幂等键零变化；双池（Redis mux.HandleFunc + Lite Executor.RegisterHandler）各注册一次 |
| **事件 3 producer** | event-catalog「Knowledge 家族」：`knowledge.processing.completed`（producer `repository/knowledge.go:CompleteProcessingWithoutSubtasks`:707 实测）、`knowledge.processing.failed`（`knowledge_housekeeping.go:runSweep`:112 实测）、`knowledge.deletion.completed`（`repository/knowledge.go:HardDeleteKnowledge`:906 实测）——均 v1、`replay: source-query` | producer 语义、`required_metadata` 键（tenant_id/occurred_at/event_id/idempotency_key/actor_origin/knowledge_id[+failure_reason]）、ordering（per-knowledge 恰一次跃迁/删除恰一次·删除计划先行）零变化；K4 搬迁不得改 metadata 键与 ordering（20 计划 §7.3） |
| **633 路由** | contracts.yaml `knowledge.routes`；`RegisterKnowledgeRoutes`（routes_knowledge.go:67，形参 `*handler.KnowledgeHandler`） | 本节点不触 router；宿主 handler compat 留 `type KnowledgeHandler` 兼容形态 + `var NewKnowledgeHandler` 别名（container.go:718、routes_knowledge.go:67 零改动）。KnowledgeHandler 字段全为冻结接口（knowledge.go:32-36 实测）+ `spanRepo repository.KnowledgeSpanRepository`（:37，宿主接口留守） |
| **Housekeeping 调度** | B0.3 Step 4（freeze:207-209）：清扫规则独占归 K4/24；System（42）经窄端口 `KnowledgeHousekeeping` 触发调度；挂点 `startHousekeepingService`（container.go:642 Invoke，func at :2390，形参 `*service.HousekeepingService`——宿主 type 别名保编译） | **不得出现第二套清扫实现**；K4 交付 `process.NewHousekeepingService` + `Start/Stop`（:57/:73/:99 已导出）即端口面（接口定义归使用方 42-system-policy，spec §4.2；K4 不在 module.go 暴露——门面注释禁写）；调度切换走 Integration Brief 由集成工程师执行 |
| **错误哨兵与文案** | `ErrKnowledgeNotFound`（repository/knowledge.go:15）、`ErrKnowledgeBaseNotFound`（repository/knowledgebase.go:13，随推迟件留守）、`ErrWikiIngestConcurrent`（K3 W1 var 别名同一实例）等 | 宿主 compat 留 `var ErrKnowledgeNotFound = repository.ErrKnowledgeNotFound`（同一实例，`errors.Is` 语义不变）；既有错误文本零变化 |
| **事件消费方** | `knowledge.processing.completed`→knowledge_post_process.go（随迁同包）；`processing.failed`→knowledge_transfer.go（随迁同包）；`deletion.completed`→knowledge_delete.go（随迁同包） | in_process transport，消费链随包保持 |

## 7. 任务（业务完整、可独立审阅；一任务一 commit，conventions §4）

> 全部任务属主节点：`b2-k-process`。依赖序 K4.0 → K4.1 → K4.2 → K4.3 → K4.4 → K4.5 → K4.6 串行（同分支）。

### Task K4.0 — 基线对齐、前置核验、T0 台账与耦合全景复核

**Files:** 无生产文件改动；产出报告 T0/判定清单章节草稿。

- [ ] **Step 1: 基线对齐 merge**——按 P2 顺序三个独立 merge commit（K1→K2→K3）；每个 merge 后 `go build ./...` 退出 0（三方 approved 分支，预期无冲突；module.go/manifest 冲突处置见 P2）。
- [ ] **Step 2: 前置核验**——P1/P3/P4 逐条执行并留痕（命令原文+退出码）。
- [ ] **Step 3: T0 台账与基线口径登记**——**基线口径（波内依赖节点特化，与 conventions §1.2 模板的偏差如实登记）**：`git merge-base origin/main HEAD` 实测返回 main 尖 `a42179135`（main 是本分支祖先），**不是**派发基线（分支创建点=集成分支头 `37eae710f`）；且 P2 三次基线对齐 merge 后，任何对基线 SHA 的三点 diff 必然混入 K1+K2+K3 全部变更，无法用于 owned_files 核对。因此本节点固定口径：(i) `PRE_MERGE_SHA=$(git rev-parse HEAD)`（Step 1 三个 merge 之前取值，预期 `37eae710f`，报告登记；若协调者派发时已回填 `base_sha` 则以其为准并与 `PRE_MERGE_SHA` 比对，不一致停下上报）；(ii) `PASSB_BASE_SHA` 按派发回填值或 `PRE_MERGE_SHA` 记录（仅用于 conventions §1.2 命令包的形式执行与基线台账）；(iii) **owned_files 核对一律用本节点非 merge commit 的变更并集**（K4.6 Step 3 专用命令，排除基线对齐 merge 带入的前置内容）。之后逐包跑并记录：`go test -count=1 ./internal/application/repository/ ./internal/application/service/ ./internal/handler/ ./internal/modules/knowledge/...`（含 skip/blocked-env 如实记录，spec §14.1）。
- [ ] **Step 4: 随迁测试判定清单落盘**——按 §8.1 判定规则对宿主 service/repository/handler 三包全部 `knowledge*`/`kb_access`/`task_progress` 前缀 `_test.go`（基线对齐后在场清单，撰写时宿主 service 侧 52 件、repository 侧 14 件、handler 侧 19 件为未对齐状态参考）逐文件 `grep -l` 被测符号 → 产出「随迁/留守/拆分」三栏清单写入报告（残差登记 Brief）。
- [ ] **Step 5: 耦合全景复核**——§5.1–§5.5 全部 file:line 与签名在对齐后树上重跑 grep 核对（重点：§5.1 airesource 伪影结论、§5.3 D1 方法集 15+4+faqSvc、§5.4 K1/K2/K3 导出符号实际签名与宿主 shim 文本）；与 20 计划 §6.2 组 C/E 差异如实登记上报（conventions §9 计划事实登记）。
- **Commit:** 无独立 commit（台账随 K4.6 报告提交）；三个 merge commit 即本任务产物。**验收**：三 merge 后 build 绿；T0 表与判定清单在报告草稿。

### Task K4.1 — repository 层 4 文件归位（escapeLikeKeyword 导出 + 哨兵别名 + K2 测试垫片删除）

**Files:** Move `repository/knowledge.go`、`knowledge_span_repo.go`、`knowledge_tag.go`、`knowledge_transfer.go` + 随迁测试（§8.1 repository 侧：`knowledge_tag_test.go`、`knowledge_finalize_test.go`、`knowledge_span_repo_test.go`、`knowledge_transfer_test.go`、`knowledge_datasource_external_id_test.go`、`knowledge_datasource_test.go`、`knowledge_duplicate_test.go`、`knowledge_folder_move_test.go`、`knowledge_folder_test.go`、`knowledge_list_filter_test.go`、`knowledge_metadata_prefix_test.go`、`knowledge_source_schema_test.go`、`knowledge_create_test.go`——K4.0 Step 4 清单为准）→ `internal/modules/knowledge/process/repository/`；Create 宿主 `internal/application/repository/kbprocess_passb_compat.go`；Delete `internal/application/repository/kbretrieval_passb_compat_test.go`（K2 垫片，其服务对象 knowledge_tag_test.go 已随迁——删除前置满足，K2 Brief :80）；Edit `docs/architecture/moves/knowledge.yaml`（删 4 行 + compat 1 行新增 + 垫片行删除）、镜像 README。

- [ ] **Step 1: git mv（M2）**——4 生产 + 随迁测试；`git diff --summary --find-renames HEAD` 全 R100；`package repository` 子句不变（落位包名对齐宿主）。
- [ ] **Step 2: 导出改名（M3）**——`escapeLikeKeyword`→`EscapeLikeKeyword`（knowledge.go:22，函数体一行不改）；包内无其他调用点（R3 清单：宿主同包调用方 tenant.go/tag.go 均他属主，经 shim）。
- [ ] **Step 3: 宿主 compat**——`kbprocess_passb_compat.go`：`var ErrKnowledgeNotFound = repository.ErrKnowledgeNotFound`（哨兵同一实例）+ `var NewKnowledgeRepository = repository.NewKnowledgeRepository`、`var NewKnowledgeSpanRepository = repository.NewKnowledgeSpanRepository`（container.go:203/:204 引用面）+ `func escapeLikeKeyword(keyword string) string { return repository.EscapeLikeKeyword(keyword) }`（identity/conversation 调用点保护）；`git build` 断链证据先行（预期 container.go:203/:204 与 tenant.go/message.go/session.go 编译错误）。
- [ ] **Step 4: 垫片删除**——`kbretrieval_passb_compat_test.go` 删除 + manifest 行删除（同 commit；其提供的 `knowledgeTagRepository` 委托由随迁后的 knowledge_tag_test.go 直连 `kbretrieval.NewKnowledgeTagRepository` 替代——测试内构造改写属 K4 属主文件，合法）。
- [ ] **Step 5: manifest**——删 4 行 + compat 登记 + 垫片行删除；`go run ./tools/modulemove verify --module knowledge` OK。
- [ ] **Step 6: GREEN**——`go build ./...` OK；`go test -count=1 ./internal/modules/knowledge/process/... ./internal/application/repository/ ./internal/application/service/` 与 T0 一致；`make verify-module-moves && make check-backend-architecture` 双绿（新显形横向 import 若报诊断，先记清单，K4.5 统一登记——本任务允许 guard 暂红则**必须**在报告登记且 K4.5 前不得合并；优先顺序：本任务内可零豁免完成（repository 侧 common 2 处除外，见 §5.5 表））。
- **Commit:** `refactor(knowledge): K4 repository layer into module (escapeLikeKeyword export, host compat, K2 test-shim removal)`

### Task K4.2 — service 层独立面 8 文件归位（K1 adapter 依赖面导出）

**Files:** Move `service/knowledge_auto_tag.go`、`knowledge_housekeeping.go`、`knowledge_index_content.go`、`knowledge_post_process.go`、`knowledge_process_config.go`、`knowledge_span_tracker.go`、`knowledge_task_options.go`、`knowledge_write.go` + 随迁测试（auto_tag_test、housekeeping_test、index_content_test、post_process 三件（graph_chunks/summary/trace）+ wiki_enqueue、process_config_test、span_tracker_test、task_options_test；K4.0 清单为准）→ `internal/modules/knowledge/process/`（package process）；Create 宿主 `internal/application/service/kbprocess_passb_compat.go` 初版；Edit manifest（删 8 行）、镜像。

- [ ] **Step 1: git mv（M2）**——8 生产 + 测试；R100。
- [ ] **Step 2: 导出改名（M3）**——§5.2 表中定义于本批 8 文件的符号：`buildKnowledgeIndexContent`→`BuildKnowledgeIndexContent`（index_content.go:12）、write-family 4 函数（knowledge_write.go:17/:32/:60/:93）。`enqueueSummaryRefresh`（knowledge_summary_refresh.go:83——该文件挂 `*knowledgeService` 方法，属 K4.3 的 11 文件批）、`getFileType`/`normalizeFileExtension`/`isDataTableFileType`（knowledge_util.go，同属 K4.3 批）的导出均在 K4.3 Step 2 执行；本批后 K1 adapter/R1-9 shim 对这批符号经宿主 compat 委托继续编译。包内 R3 调用点同步改名。
- [ ] **Step 3: 宿主 compat 初版**——`var NewKnowledgePostProcessService = process.NewKnowledgePostProcessService`（dig.Name 不变，container.go:412）、`var NewKnowledgeAutoTagService = …`（:413）、`var NewHousekeepingService = …`（:641）+ `type HousekeepingService = process.HousekeepingService`（startHousekeepingService 形参 :2390）+ `type SpanTracker = process.SpanTracker`、`var NewSpanTracker = process.NewSpanTracker`（container.go:374、K1 adapter）+ 本批导出符号一行委托（K1 `span_trace_seam_adapter.go` 引用面：SpanTracker/Span 类型与 write-family/summary/indexContent 函数值）。核验 K1 adapter 与 K1 R1-9 shim 零改动编译。
- [ ] **Step 4: housekeeping 测试联动**——随迁 `knowledge_housekeeping_test.go` 的 W1 依赖（wikiTaskType/wikiTaskScope/WikiOpIngest/WikiOpRetract，K3 Brief (b) :46）改直连 `wiki.WikiTaskType` 等。
- [ ] **Step 5: GREEN + manifest**——同 K4.1 Step 6 模式；`go test -count=1 ./internal/modules/knowledge/process/...` 与 T0 一致。
- **Commit:** `refactor(knowledge): K4 standalone service facet into module (span/write/summary export ports for K1 adapter)`

### Task K4.3 — knowledgeService 核心批 11 文件 + D1/writableFAQKnowledgeBase 收口（本计划最重任务）

**Files:** Move §3.1 方法分布表的 11 文件：`service/knowledge.go`（knowledgeService 定义 :49）、`knowledge_process.go`、`knowledge_create.go`、`knowledge_delete.go`、`knowledge_delete_plan.go`、`knowledge_clone_move.go`、`knowledge_transfer.go`、`knowledge_replace.go`、`knowledge_reparse_scope.go`、`knowledge_summary_refresh.go`、`knowledge_util.go` + 随迁测试（含 **knowledge_move_wiki_test.go**（20 计划 §9 点名随 K4；白盒 moveOneKnowledge:163、svc.repo:167/:235）、**document_write_access_test.go**（白盒 deleteReferencedKnowledge:525/:530、withKnowledgeCleanup:511、knowledgeService 构造；K1 Brief 曾判留宿主因 chunk shim——K4 搬迁后白盒断链，随迁改经 K1 落位包/宿主 shim 引用 chunk 面，残差登记 Brief）、knowledge_process 三件、knowledge_delete_test、knowledge_cleanup_regression、knowledge_image_cleanup、knowledge_resource_release、knowledge_create_test、knowledge_create_folder、knowledge_clone_image、knowledge_batch_reparse、knowledge_reparse、knowledge_manual、knowledge_content_binding、knowledge_quota_guard、knowledge_move_gate（service 侧）、knowledge_owner_chain、knowledge_storage_config、knowledge_update、knowledge_summary 五件（summary_test 写点改 wiki.MinTextContentRunes）、knowledge_transfer_test（service 版）、knowledge_util_filetype、knowledge_util_ssrf、knowledge_replace_test、knowledge_write_access_test（**拆分**：K4 write 面用例随迁；tag 面用例留宿主待 K2 推迟件补迁——K2 Brief 推迟件 12-13 解除条件，残差登记 Brief）；K4.0 清单为准）→ `internal/modules/knowledge/process/`；Create `process/faq_delegate.go` + `process/semantic_scope_guard.go`（§5.3/§5.3a）；Edit K2 compat 删 `writableFAQKnowledgeBase` 段、宿主 service compat 追加；Delete 宿主 `knowledge_faq_k3_delegate.go`（+ manifest 行）与 `knowledge_faq_k3_test_support_shim_test.go`（§5.3b 矩阵 1/3，授权链见 §4）；Edit manifest（删 11 行）、镜像。

- [ ] **Step 1: git mv（M2）**——11 生产 + 随迁/拆分测试；R100；`knowledgeService` 类型名保留（包内私有，接收者零改动）。
- [ ] **Step 2: R3 改名、guard 第四轨与直连（M3）**——(i) **guard 第四轨落地（§5.3a，先于 git mv 采证）**：实测 `retrieval/app/semantic_scope_guard.go` 现文本后新建 `process/semantic_scope_guard.go` 逐字对齐（文件头注释登记第四轨与 ib2 四轨收口）；(ii) §5.1 导出：`withKnowledgeCleanup`→`WithKnowledgeCleanup`、`deleteReferencedKnowledge`→`DeleteReferencedKnowledge`（knowledge_delete_plan.go，包内调用点 knowledge_delete.go:766、knowledge_transfer.go:357、测试随改）；`attemptSuperseded`→`AttemptSuperseded`、`finalizeSubtaskDetached`→`FinalizeSubtaskDetached`（knowledge.go）、`isLikelyRateLimitError`→`IsLikelyRateLimitError`（knowledge_process.go:3902）、`removeSourceRef`→`RemoveSourceRef`（knowledge_delete.go:346）、`enqueueSummaryRefresh`→`EnqueueSummaryRefresh`（knowledge_summary_refresh.go:83，本批文件）、`getFileType`/`normalizeFileExtension`/`isDataTableFileType`→导出名（knowledge_util.go:89/:47/:67）；(iii) §5.4 直连：K2 符号 7 个（RecordKBActivity/KBActivityTrigger/WithKBActivityTask/KBActivityAppendSampleTitles/KBReadPermissions/RequireKBWrite/WithKBWriteTenantInfo）、K3 符号 13 个、K1 符号 4 个——逐调用点改 import + 导出名（`grep -c` 前后计数登记报告）。
- [ ] **Step 3: §5.3b 断链矩阵四项收口**——(1) 落位 `faq_delegate.go`（§5.3 机制 1）；(2) K2 compat 删 `writableFAQKnowledgeBase` 段（:190 注释/:194 func 起至方法末，函数文本迁入 process 包同名方法，其余段零触碰）；(3) 宿主删 `knowledge_faq_k3_delegate.go` + manifest 行删除；(4) `knowledge_write_access_test.go` 拆分（write 面随迁 + tag 面留守；垫片 4 方法调用点改 `svc.faqSvc().X(...)` 同包白盒等价形态）+ 删 `knowledge_faq_k3_test_support_shim_test.go`（§5.3 机制 4，两路径判定以 K4.0 Step 4 清单为准）。**编译器即验收**：`go build ./...` 证明 `*process.knowledgeService` 满足 `interfaces.KnowledgeService` 全签名（FAQ 切片经 faq_delegate）；`grep -c "func (s \*knowledgeService)" internal/application/service/` = 0（宿主方法集清零，K3 计划 K3.2 Step 6 同款断言）。
- [ ] **Step 4: 宿主 compat 追加**——`var NewKnowledgeService = process.NewKnowledgeService`（container.go:343）+ §5.1/§5.2 全部导出符号的一行委托（withKnowledgeCleanup/deleteReferencedKnowledge/attemptSuperseded/finalizeSubtaskDetached/isLikelyRateLimitError/removeSourceRef/enqueueSummaryRefresh/getFileType 族；write-family/buildKnowledgeIndexContent 已在 K4.2 落）——保护 datasource/conversation/insights/identity 调用点与 K1 adapter/K3 W2 闭包编译；断链证据先行（`go build ./...` 错误清单采证）。
- [ ] **Step 5: W1/W2 链路核验**——K3 `wiki_k3_compat.go`（W1）对 K4 已直连符号（previewText 等）的宿主转发继续服务未搬迁方；K3 `wiki_k3_ctor_compat.go`（W2）`wikiK3Seams()` 的 K4 目标（finalizeSubtaskDetached 等）经宿主 compat 委托链解析（宿主→process 导出）；两者零改动编译。
- [ ] **Step 6: GREEN + manifest**——全量测试与 T0 比对（§8.3 差分首跑）；双守卫（K4.5 前允许的暂红登记同 K4.1 Step 6 纪律）。
- **Commit:** `refactor(knowledge): K4 knowledgeService core into module with D1/writableFAQKnowledgeBase closure (export cross-owner ports)`

### Task K4.4 — handler 层 4 文件归位（路由兼容别名）

**Files:** Move `handler/kb_access.go`、`knowledge.go`、`knowledge_download.go`、`task_progress_auth.go` + 随迁测试（kb_access_test、knowledge_api_key_scope、knowledge_download、knowledge_folder（handler 版）、knowledge_move_gate（handler 版）、knowledge_mutation_admission（contracts 登记）、knowledge_ownership、knowledge_preview_security（contracts 登记）、knowledge_spans、knowledge_tag_ids、knowledge_transfer（handler 版）、task_progress_auth_test；**rbac_lookups_test.go 不随迁**（identity 推迟件，K2 计划 §7.2））→ `internal/modules/knowledge/process/handler/`；Create 宿主 `internal/handler/kbprocess_passb_compat.go`；Edit manifest（删 4 行）、镜像。

- [ ] **Step 1: git mv（M2）**——4 生产 + 测试；R100；`package handler` 子句不变。
- [ ] **Step 2: 导出改名（M3）**——`parseCommaSeparatedTagIDs`→`ParseCommaSeparatedTagIDs`（knowledge.go:2546）、`requireTaskProgressTenant`→`RequireTaskProgressTenant`（task_progress_auth.go:14）；包内 R3 调用点随改；`resolvedKBAccess`（kb_access.go:18）、`resolveHandlerKBAccess`（:28）、`resolveHandlerKBAccessFor`（:34）三个包内 helper 的 handler 包内其他属主文件调用点处置以 `go build` 断链清单为准——**若宿主 handler 包存在他属主调用方**，宿主 compat 留一行委托；若仅 K4 文件消费则直迁（K4.0 Step 4 判定）。
- [ ] **Step 3: 宿主 compat**——`type KnowledgeHandler struct { *processhandler.KnowledgeHandler }`（嵌入保方法集提升；routes_knowledge.go:67 形参类型不变、`&KnowledgeHandler{}` 字面量兼容——**若构造器形参含宿主具型**则以 type 别名 + `var NewKnowledgeHandler` 转发组合（K3 H1/H2 双先例按断链证据择一，报告记录）+ `var NewKnowledgeHandler`（container.go:718）+ kb_access/task_progress 两导出符号委托。
- [ ] **Step 4: GREEN + manifest**——`go test -count=1 ./internal/modules/knowledge/process/... ./internal/handler/` 与 T0 一致（mutation_admission/preview_security/download 双跑）；双守卫。
- **Commit:** `refactor(knowledge): K4 handlers into module (route compat aliases, faq seam exports)`

### Task K4.5 — 跨模块 import 豁免登记与计数基线（独立 commit）

**Files:** Edit `tools/architectureguard/check.go`（importExceptions 数据行）、`docs/architecture/passb/exception-ledger.yaml`（属主行）；报告豁免章节。

- [ ] **Step 1: 全量清点**——`grep -rn "internal/application/\|internal/common\|internal/infrastructure\|internal/searchutil" internal/modules/knowledge/process/ --include="*.go" | grep -v "_test.go"` 逐条列出 file→package 对；与 §5.5 种子表核对（多退少补）；每对登记前评估 seam 替代（§5.5 末段义务）。
- [ ] **Step 2: 登记豁免**——按 Ruling 程序追加 check.go 精确条目 + ledger 行（owner=24-knowledge-process、remove_at=ib2）；机械计数修正 + §8 基线登记（evidence 登记原因与 Ruling 引用）。
- [ ] **Step 3: GREEN**——`make check-backend-architecture` 零诊断；`go build ./...` 退出 0。
- **Commit:** `chore(passb): register exact cross-module import exceptions surfaced by K4 process moves (Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY)`

### Task K4.6 — 高风险差分、Integration Brief、节点门禁与收口

**Files:** Create `docs/architecture/evidence/passb/b2-k-process.md`、`docs/plans/passb/reports/b2-k-process.md`（补全 K4.0 T0 表与判定清单）、`docs/architecture/passb/briefs/b2-k-process.md`。

- [ ] **Step 1: 差分执行**——§8.3 表逐面双跑（T0 基线输出 vs 搬迁后同用例），用例清单、双跑输出摘录、比对结论、命令与退出码入 evidence 差分章节；失败只修新实现。
- [ ] **Step 2: 节点 gates 逐条执行并摘录**——`go build ./...`、`go test ./internal/modules/knowledge/... -count=1`、`make check-backend-architecture`、`make verify-module-moves`（命令原文+退出码入报告；conventions §2 禁替代）。
- [ ] **Step 3: 变更清单 vs owned_files（本节点非 merge commit 变更并集口径，K4.0 Step 3 (iii) 定义）**——`git log --no-merges --format=%H "$PRE_MERGE_SHA"..HEAD | while read c; do git diff-tree --no-commit-id --name-only -r "$c"; done | sort -u` 得到本节点自有 commit 的文件并集（基线对齐 merge 带入的 K1/K2/K3 内容天然排除——merge commit 被 `--no-merges` 剔除）；另按 conventions §1.2 形式执行 `git diff --stat "$PASSB_BASE_SHA"...HEAD` 并在报告注明「含前置基线对齐内容，owned_files 判定以上一并集为准」。并集与 §4 可写清单逐条核对，差集非空即失败。
- [ ] **Step 4: Integration Brief**——内容：(a) 装配切换表——container.go:202-204/:343/:374/:412/:413/:641/:718 七组 Provide 与 :642 Invoke、routes_knowledge.go:67 形参的 ib2 直连目标（落位包构造器/类型）；(b) 18 worker 的 K4 面 11 项注册行核验说明（零改动证据 + dig.Name 消歧保留）；(c) D1 收口实录（faq_delegate.go 方法集清单、宿主 D1 删除、K2 compat 段删除）与 `writableFAQKnowledgeBase` 接收记录（两 Brief 授权原文引用）；(d) §5.1/§5.2 导出符号的跨 owner 宿主调用点 ib2 改写清单（identity tenant.go:85/:90、conversation message.go:215/:449/:469、session.go:209/:687/:764/:821、web_search_state.go:124、insights evaluation.go:366、datasource_service.go:885、K2/K3 落位包 seam 直连目标）；(e) 宿主 compat 3 文件删除编排（残留 importer 清零前置）；(f) 推迟件 `repository/knowledgebase.go` 解除编排（§3.3，与 12-commercial 补迁同窗）+ K2 推迟件 3-10 解除条件的 K4 侧完成证据（knowledge_delete.go 已迁）；(g) K1 `span_trace_seam_adapter.go`/R1-9 shim 的 ib2 切换目标（process 导出包装）；(h) 测试拆分残差（knowledge_write_access_test tag 面、document_write_access chunk 面）与 K2 垫片删除记录；(i) minTextContentRunes var 双副本收口记录；(j) §8 计数奇偶记录。
- [ ] **Step 5: 报告**——执行命令台账、owned_files 核对结论、推迟项（§3.3）、airesource ppc 伪影上报、未完成项如实列出（conventions §1.2）。
- **Commit:** `docs(passb): b2-k-process differential evidence, integration brief and node report`

## 8. 测试与高风险差分要求

### 8.1 随迁 `_test.go` 判定（规则 + 已确认清单；K4.0 Step 4 落盘终版）

**判定规则**（K2 计划 §7.1 先例）：同名主题测试 + 被测对象仅本节点文件；白盒引用他 plan 属主未导出符号时以**定义文件属主**为随迁归属（20 计划 §9 共性义务），残差登记 Brief；拆分只按被测对象面切（knowledge_write_access_test 先例处置见 K4.3）。

**已确认（20 计划 §10 点名/contracts.yaml characterization 登记/白盒实证）**：`knowledge_move_wiki_test.go`（随 K4，20 计划 §9）、`knowledge_auto_tag_test.go`、`knowledge_post_process_wiki_enqueue_test.go`、`knowledge_replace_test.go`、`handler/knowledge_mutation_admission_test.go`（以上 contracts chunk-service 登记）、`document_write_access_test.go`（白盒 :511/:525/:530，见 K4.3）、`knowledge_write_access_test.go`（**拆分**：write 面用例随迁 process（含 :200/:208/:362/:364 借 FAQ 方法作写入载体的用例，随迁后调 `svc.faqSvc().X(...)` 等价形态——K3 垫片 4 方法的消费面）；tag 面留宿主待 K2 推迟件补迁；K3 Brief (b) :57 删除前置）、`knowledge_finalize_test.go`（repository，事件 producer 差分锚点）、`knowledge_tag_test.go`（K2 垫片服务对象）。其余以 `ls` + `grep -l` 在 K4.0 逐文件判定（各任务 Files 节的清单为撰写时初判）。

**留宿主（防误迁）**：`rbac_lookups_test.go`（identity 推迟件）、K2 推迟件关联测试（knowledgebase_copy_preflight/hybrid_search*/not_found/pr3_response/pr5_list/request 等 handler 侧、knowledge_shared_access/knowledge_shared_storage_failure/knowledge_caller_scope/knowledge_write_access 的 tag/kbshare/chunk 面——按 K4.0 拆分判定）、`knowledge_faq_create_guard_test.go`/`faq_enabled_filter_test.go`（K3 已随迁，基线对齐后不在宿主）。

### 8.2 复用与新增

全部复用现有测试（不新写行为测试）；新写仅一类机械守卫：K4.3 Step 3 的宿主方法集清零断言（`grep -c` = 0，K3.2 Step 6 先例）与接口满足性编译断言（`go build` 即证明）。**TDD 顺序**（conventions §1.4）：每任务先跑 T0 基线（特征化锚定旧行为）再动文件；K4.3 的 D1 收口以「搬迁前 D1 编译在场 → 搬迁后 faq_delegate 编译在场 + 宿主清零」为红绿判据。

### 8.3 高风险差分（framework:40「knowledge deletion/indexing」+ 20 计划 §10 K4 行；spec §14.3 同输入双跑；证据入 evidence 差分章节）

| 面 | 锚定用例（现状基线，随迁后同用例双跑） | 等价判据 |
|---|---|---|
| 删除级联与删除计划先行序（`TypeKnowledgeListDelete`/`TypeKBDelete` 面） | knowledge_delete_test.go、knowledge_cleanup_regression_test.go、knowledge_image_cleanup_test.go、knowledge_resource_release_test.go、knowledge_manual_test.go（cleanup+re-index 注释锚）、repository/knowledge_finalize_test.go | 删除路径顺序（计划→执行→事件）、DB 结果（硬删/软删/tombstone wikiDeletedTTL）、`knowledge.deletion.completed` 恰一次逐用例一致 |
| `ProcessDocument` 重试/幂等（`TypeDocumentProcess`/`TypeManualProcess`/`TypeKnowledgeListReparse`） | knowledge_process_status_test.go、knowledge_process_parent_child_test.go、knowledge_reparse_test.go、knowledge_batch_reparse_test.go、knowledge_content_binding_test.go | parse_status 状态机跃迁、span 树、重试语义（isFinalAsynqAttempt 分支）、`knowledge.processing.completed` 恰一次（原子 promote）一致 |
| 巡检置败（`knowledge.processing.failed` producer） | knowledge_housekeeping_test.go（stale 阈值、filterByLastSpanActivity/filterOutQueued、cron Start/Stop） | sweep 候选集、置败路径、失败原因 metadata 一致 |
| 克隆/迁移/替换（`TypeKBClone`/`TypeKnowledgeMove`） | knowledge_transfer_test.go（service+repository 双件）、knowledge_clone_image_test.go、knowledge_folder_move_test.go（service+repository）、**knowledge_move_wiki_test.go**（wiki 状态 reconcile）、knowledge_replace_test.go | 进度持久化（memFAQ 类比 span/progress 字段）、FAQ 状态同步计划（buildFAQStatusSyncPlan 委托链）、source_ref 清理一致 |
| 级联删除跨域入口（deleteReferencedKnowledge） | document_write_access_test.go :525/:530 用例（expectedKB 校验/幂等已删） | 返回错误逐分支一致（conversation/insights 消费方语义保护） |
| handler 面（下载/预览安全/变更准入） | knowledge_download_test.go、knowledge_preview_security_test.go、knowledge_mutation_admission_test.go、kb_access_test.go、task_progress_auth_test.go | 状态码映射、RBAC/apiKey 门、跨租户防枚举（requireTaskProgressTenant 404 语义）逐用例一致 |

差分失败只修新实现、禁改期望值（spec §14.3）；legacy 删除（manifest 行）前对应面差分必须已通过（framework:31）。T1/T2 梯度：每任务搬完跑落位包+宿主包测试与 T0 台账比对（新失败即停，定位首个破坏提交，spec §14.1）。

## 9. 集成与回滚边界

- **集成**：本节点分支经 review 后按缺省序 K1→K2→K3→K4 由集成工程师以 `merge: passb b2-k-process` 合入（framework:28）；IB2 只从 `docs/architecture/passb/briefs/b2-k-process.md` 切换共享装配（framework:103）；router/container/task/sync_task/bootstrap 改写仅集成工程师按 Brief 串行执行（本节点零触碰，K1/K2 先例 diff 0 行）；contracts.yaml knowledge 区状态回写仅 ib2（conventions §3；F2 status 字段未落盘，K0 差异③先例——本节点不声称契约状态变更）。
- **回滚**（spec §13）：K4.0 三个基线对齐 merge commit 回滚=revert 单 commit 无装配影响；K4.1–K4.4 为 M2+M3 提交，分支内逐 commit revert（宿主恢复原文件、D1/垫片/compat 段复活），已并入集成分支的 M2/M3 保留（新路径未接线不影响运行，M4 失败先回退切换 commit）；K4.5 为数据行 commit，revert 需同步还原计数登记（§8 基线流程）；本节点无 schema/migration/路由/worker 计数变化，回滚不需数据修复；compat/shim 删除（ib2）在任何残留 importer 未清零前禁止执行（framework B5 口径前移）。
- **升级边界**：门禁不可能通过、冻结接口出现 §6 表外 FAQ 方法、D1 收口遇到 §5.3 表外 knowledgeService 字段依赖、或需改 20 计划 §5/§6 冻结表时，按 conventions §5 停手上报（报告 + DAG notes 建议），禁止现场改判所有权、删测试或扩大例外。

## 10. 必须删除的 legacy/alias/例外（本节点口径）

| 对象 | 动作 | 时点 |
|---|---|---|
| `docs/architecture/moves/knowledge.yaml` 中 27 个已物理搬迁文件的 `legacy_files` 行 | 同搬迁 commit 删除（modulemove 机械强制；Ruling LEGACY-ROW-OWNERSHIP） | K4.1–K4.4 |
| `ownership-matrix.yaml` 对应 27 行 | 不由本节点删（barrier ib2 回写窗口；物理迁移证据以 manifest+git 为准） | ib2 |
| `repository/knowledgebase.go` 的 manifest/matrix 行 | **保留**（推迟件，行随物理迁移删除；Ruling 6 第 4 点） | ib2 |
| K3 D1 `knowledge_faq_k3_delegate.go` + manifest 行 | K4.3 同 commit 删除（K3 Brief (c) 授权；职责由 `process/faq_delegate.go` 承接） | K4.3 |
| K2 垫片 `kbretrieval_passb_compat_test.go` + manifest 行 | K4.1 同 commit 删除（K2 Brief :80 删除前置=knowledge_tag_test.go 随迁，满足） | K4.1 |
| K3 测试垫片 `knowledge_faq_k3_test_support_shim_test.go`（无 manifest 行，K3 报告台账在册） | K4.3 随 knowledge_write_access_test.go 拆分改写同 commit 删除（K3 Brief (b) :57 删除前置=「K2/K4 域改写测试」，K4 侧到达） | K4.3 |
| K2 compat `writableFAQKnowledgeBase` 方法段 | K4.3 迁出（K2 Brief :78 授权；文件其余段零触碰） | K4.3 |
| 3 个宿主 compat 文件（kbprocess_passb_compat ×3）+ manifest 登记行 | 新增为待删项（Ruling TRANSITION-SHIM-ROW-REGISTRATION 成对补行）；删除=ib2 直连完成后 | ib2 |
| 新登记 importExceptions 精确豁免 + ledger 行（K4.5，预期 §5.5 种子 8 对左右） | 保留至跨模块消费经门面/端口合法化；remove_at=ib2 | ib2 |
| exc-0088/0089/0090/0091 与 exc-0106..0110 | 非本节点属主（21/22），无删除动作；K4 落位后 K2 Brief §6 escapeLikeKeyword seam 的收口条件（「K4 迁移 knowledge.go 导出窄端口后删 seam」）已满足前半，删 seam 动作归 ib2 | ib2 |
| 别名 18 条（knowledge.yaml:41-77）与 container.go:36-38 旧 import | 不在本节点 28 文件范围（K5/ib2 收口，20 计划 §7.5） | K5/ib2 |

## 11. 独立验收标准

1. DAG `b2-k-process` gates 四项全绿，命令原文+退出码记录于 `docs/plans/passb/reports/b2-k-process.md`（conventions §2 禁替代命令）；
2. 本节点非 merge commit 变更并集（K4.6 Step 3 命令）与 §4 可写清单差集为空（`PASSB_BASE_SHA` 三点 diff 因基线对齐 merge 混入前置内容，仅作台账不作判定，K4.0 Step 3 口径）；27+1（推迟）中的 27 个文件物理落位三个子包，逐文件 `git diff --summary --find-renames` R100、函数体 diff 为零（package/import/R3 改名/R2 直连除外，逐类登记）；
3. manifest `legacy_files`：27 行已删、compat 3 行在册、推迟件 1 行保留、D1/垫片行随文件删除；`make verify-module-moves` OK；
4. `grep -c "func (s \*knowledgeService)" internal/application/service/` = 0（含 D1 19 方法、K2 compat writableFAQKnowledgeBase 段、K3 垫片 4 方法全部收口——§5.3b 矩阵四项逐一销账，K3/K2 垫片与 D1 文件已删）；`go build ./...` 证明 `*process.knowledgeService` 满足 `interfaces.KnowledgeService` 冻结接口（`internal/types/` 零改动）；`process/semantic_scope_guard.go` 第四轨在册且文件头登记 ib2 四轨收口；
5. §5.1/§5.2 导出义务符号全部在落位包以导出名可解析，宿主 compat 一行委托逐一在册（报告核对表）；datasource/conversation/insights/identity 调用点文件零改动（`git diff --name-only` 不含）；
6. §8.3 六个差分面双跑逐用例一致，证据入 `docs/architecture/evidence/passb/b2-k-process.md`（conventions §6）；
7. worker 注册行 `internal/router/task.go`、`internal/router/sync_task.go` 与 container/router/bootstrap/migration/go.mod 零触碰（`git diff --name-only` 验证）；计数基线 633/23+23/58/537 三方一致不因本节点变化（conventions §8；importExceptions 增量 N 条与 ledger 行一一对应、remove_at=ib2、基线变更登记在案）；
8. 推迟件 1 个：原位零改动（`git diff` 为空）、根因证据与解除编排写入 Brief 与报告；
9. 治理文件（ownership-matrix/contracts/event-catalog）零改动；K1/K2/K3 产物零触碰（除 §4 授权三处：D1 删除、K2 compat 段删除、K2 垫片删除——逐处附授权原文引用）；
10. 计划评审 approved（reviewer 独立产出 `docs/plans/passb/reviews/b2-k-process.md`）。

## 12. 计划自检记录（撰写时执行）

- **Spec 覆盖**：ask 九要素 → §1（Spec 指针）、§2（前置与 BLOCKED 解除实证）、§3-§4（精确文件与写权）、§5-§6（真实接口签名/行为兼容）、§7（步骤+命令+预期）、§8（测试与差分）、§9（集成与回滚）、§10（删除义务）、§11（验收）——全覆盖。
- **无占位符**：全部符号含 file:line 与实测签名（escapeLikeKeyword:22、withKnowledgeCleanup:22、deleteReferencedKnowledge:36、isValidFileType:62、moveOneKnowledge:1194、attemptSuperseded:202、finalizeSubtaskDetached:235、isLikelyRateLimitError:3902、removeSourceRef:346、enqueueSummaryRefresh:83、buildKnowledgeIndexContent:12、parseCommaSeparatedTagIDs:2546、requireTaskProgressTenant:14、runSweep:112、CompleteProcessingWithoutSubtasks:707、HardDeleteKnowledge:906、NewKnowledgeService:100、NewSpanTracker:172 等，撰写时逐条 grep/sed 命中）；无 TBD；「编译器枚举补齐」（K4.3 Step 3）与「断链证据先行」是确定性验收机制而非未决设计；两处「以 K4.0 判定为准」的测试清单是 Ruling 6 框架下的机械判定义务（规则+初判清单已给），非悬空待定。
- **类型一致性**：§5 全部签名与 20 计划 §6.2 冻结表零漂移（差异两处如实登记：airesources deleteReferencedKnowledge 调用点实测 0（ppc 伪影，K4.0 上报）；getParserEngineOverridesFromContext/isValidFileType 同名伪影沿用 K0.1 消歧结论）；`interfaces.KnowledgeService` 全签名转录自 contracts.yaml:1773；worker 分发路径与 task.go:266-320/sync_task.go:143-163 逐行实测对应。
- **跨任务接口一致性**：K4.2 先行导出 K1 adapter 依赖面（span/write/summary/index）→ K4.3 的 D1 收口依赖 knowledgeService 同包（11 文件批内自洽）；K4.1 垫片删除前置（knowledge_tag_test 随迁）在任务内闭环；K4.4 的 faq seam 导出（parseTagIDs/requireTaskProgressTenant）与 K3 Brief (d) 表逐行对应；宿主 compat 分批追加与各任务 GREEN 门禁闭环；推迟件不产生任何跨任务接口义务。
- **不可行/未做项如实登记**：基线对齐 merge 未执行（依赖节点派发，P2 待开工核验）；`go test` 各命令为计划预期（本计划撰写为 docs 变更，未跑搬迁后测试——T0/T1/T2 实测义务在任务步骤内）；K1 落位包部分导出符号的实际签名（R1-9 shim 实参形态）以 K4.0 Step 5 复核为准（已给出核对方法与登记义务）。
- **审校修复轮（2026-09-25，8 项 findings 全处置，各分支/宿主逐条实测复核）**：① critical——knowledge.go:50 无字段名内嵌 `semanticScopeGuard`（宿主同形定义在 K2 compat :29 且未导出、不在授权段）→ 新增 §5.3a process 第四轨机制（逐字对齐 K2 落位包 `retrieval/app/semantic_scope_guard.go`，其自带注释即「多属主重复 helper 族，ib2 收口为单一定义」，四轨统一编排入 Brief；宿主 compat guard 段不动——留守嵌入方 identity 4 文件 + K2 推迟件 2 文件实测在场）；② critical——K3 测试垫片 `knowledge_faq_k3_test_support_shim_test.go`（4 方法 + 2 纯函数，挂 `*knowledgeService`）→ §5.3 表新增第三行 + 收口机制 4（消费者实测：knowledge_write_access_test.go:200/:208/:362/:364 与已随 K2 迁走的 kb_activity_test.go）+ §5.3b 矩阵汇总 + 授权链（K3 Brief (b) :57「K4 域改写测试后垫片删除」）；③ important——`PASSB_BASE_SHA` 口径矛盾（merge-base(main,HEAD) 实测 a42179135≠派发基线；三点 diff 混入前置内容）→ K4.0 Step 3 固定 `PRE_MERGE_SHA` 口径 + owned_files 核对改「非 merge commit 变更并集」专用命令（K4.6 Step 3 / §11.2 同步）；④ important——`enqueueSummaryRefresh` 误列 K4.2 → 移回 K4.3 Step 2（其定义文件 knowledge_summary_refresh.go 属 11 文件批）；⑤ minor——P2 纳入 K3 属边扩展 → 增补 conventions §9 登记义务（DAG notes/corrections 建议，不改 DAG）+ modify/delete 冲突预置裁定（K2 侧迁移结果优先）；⑥ minor——kb_access.go 行号 :20/:29 漂移 → 实测勘误 :18/:28/:34（补列 `resolveHandlerKBAccessFor`）；⑦ minor——D1 (b) 行 `knowledgebase_access.go:42` 失真（K2 分支实测该行为 `WithKBWriteTenantInfo` 形参区）→ 勘误为实际调用方（kbretrieval_passb_compat.go:198 writableFAQKnowledgeBase 方法体 + D1 内部），并注明 K3 计划 D1 表原引为 K2.4 迁出前旧坐标；⑧ minor——「K2 符号 8 个」计数漂移 → §5.4 表实列 7 个逐一在 K2 分支导出面复核（RecordKBActivity/KBActivityTrigger/WithKBActivityTask/KBActivityAppendSampleTitles/KBReadPermissions/RequireKBWrite/WithKBWriteTenantInfo），K4.3 Step 2 计数同步改 7。
