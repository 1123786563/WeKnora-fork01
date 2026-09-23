# Pass B B2 — Knowledge Mini-program 协调计划（K0 冻结 + K5 集成）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**节点：** `b2-k0`（DAG `docs/plans/passb/execution-dag.json`，base_sha=`8c45a88153d0b20088252dbb29d2fb3815b253c2`，gates=`go build ./...` / `make check-backend-architecture` / `make verify-module-moves`）。本文件同时是 K5（`b2-k-integration`）的计划载体（两节点 `plan_path` 均为本文件）。

**Goal:** 冻结知识子程序（K0–K5）的共享类型归属与跨 plan 未导出符号联动裁定，使 K1/K2/K3 可在互不相交的 worktree 中并行搬迁 84 个 legacy 文件；随后串行 K4（process/状态机 + 18 worker 实现），最后 K5 完成装配切换、别名/例外删除记录与差分汇总。

**Architecture:** B2 知识子程序为五节点串并混合（framework:115-132）：K0 SERIAL 冻结 → K1(9)|K2(29)|K3(18) PARALLEL → K4(28) SERIAL（K1+K2 之后）→ K5 SERIAL 汇聚。K0 不搬迁任何生产文件、不改任何共享装配；其产出是本文件的冻结表 + 守卫测试 + 证据/报告。

**Tech Stack:** Go 1.26、Gin、GORM、dig、asynq、YAML v3、Testify、Git worktrees。

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`（§4.2 依赖方向、§5.2 Knowledge 所有权、§5.18 覆盖矩阵、§11 Pass B 串并图、§13 提交隔离与回滚、§14.3 高风险差分、§17.2 完成标准）。

---

## 1. 事实源指针（全部只读输入）

| 事实源 | 用途 |
|---|---|
| `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` | 行为边界、完成标准（§11–§17.2） |
| `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` | 程序分解与全局约束（:25-33、:109-153、:132 并行前提）；行号以 base `8c45a8815` 集成分支副本为准（主 checkout 副本行号有 3 行偏移） |
| `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | 派发契约、门禁、升级、差分、计数基线（§1–§9） |
| `docs/plans/passb/execution-dag.json`（集成 worktree） | b2-k0/b2-k-* 六节点 owned_files、gates、required_contracts、notes |
| `docs/architecture/moves/knowledge.yaml` | 84 legacy 文件、18 alias_obligations（:41-77）、integration_points（:493-526） |
| `docs/architecture/passb/knowledge-ingest.md` / `knowledge-retrieval.md` / `knowledge-wikifaq.md` / `knowledge-process.md` | 四域 brief（集成 worktree 版含 B0.2/B0.3 消歧注记） |
| B0 冻结产物（`.worktrees/passb-int/docs/architecture/passb/`）：`ownership-matrix.yaml`、`contracts.yaml`、`event-catalog.yaml`、`exception-ledger.yaml`、`b0-evidence.md` | 文件属主/落位/删除屏障、契约签名、事件 v1、例外 removal owner |
| 实测代码（base `8c45a8815` 工作树） | 本计划全部签名与调用点行号的出处（见 §5–§7 各表） |

注意：主 checkout 的 `docs/architecture/passb/knowledge-wikifaq.md`、`knowledge-process.md` 为 B0 前旧版；以集成 worktree 版为准（含「child plan：23/24」标注、`escapeLikeKeyword` 消歧注记、`KnowledgeHousekeeping` 端口裁定）。

---

## 2. 节点裁定与前置条件

**节点裁定（调度方原文，照录）：**

1. K1-K3 仅在 K0 分配共享 Chunk/KnowledgeBase/Tag/semantic 类型后方可并行（framework:132）。
2. 知识 84 legacy 文件 = K1 9 + K2 29 + K3 18 + K4 28（framework:120-121；与 `ownership-matrix.yaml` 逐行核对一致：21→9、22→29、23→18、24→28，实测脚本输出 `{'21-knowledge-ingest': 9, '22-knowledge-retrieval': 29, '24-knowledge-process': 28, '23-knowledge-wikifaq': 18} total 84`）。
3. BLOCKED（2026-09-23）：前置 b0 阻塞；解除条件：修复并 done b0 后恢复（DAG b2-k0.notes）。

**前置条件核对（K0 开工门禁，逐条给出实证）：**

- [ ] **P1 b0 冻结产物在位**：`.worktrees/passb-int/docs/architecture/passb/` 下 `ownership-matrix.yaml`（2705 行）、`contracts.yaml`（139313 字节）、`event-catalog.yaml`、`exception-ledger.yaml`、`b0-evidence.md` 均存在（2026-09-23 18:50 时间戳，实测 `ls`）。DAG b0 节点 `status=done`、`review_status=approved`。
- [ ] **P2 b0 已登记未闭环修复工单**：DAG b0.notes 登记 `ocr-r3-a2-f1`（`tools/passbguard/check.go:688-690` 事件诊断措辞）与 `b0-ocr-r3-1`（`check.go:510-521` consumer 路径归一化不对称）两张修复工单，登记时 b0 分支无对应修复提交。**裁定**：K0 及 K1-K5 的门禁不依赖 passbguard 代码改动，本程序可在工单收口前撰写与评审计划；但协调者在派发 K1-K3 实施前必须确认两工单收口且 BLOCKED 解除（裁定 3），否则按 conventions §5 保持 blocked。
- [ ] **P3 ib1 已合并但为零实施**：集成头 `8c45a8815`（ib1 登记提交）的 diff 仅含 `docs/architecture/passb/execution-ledger.md` 与 `docs/plans/passb/19-foundation-integration.md`（实测 `git show 8c45a8815 --stat`）。因此：
  - knowledge→airesource 消费保持 Pass A 现状：模块包直连 `internal/modules/airesource/models/{embedding,chat,utils,vlm,utils/ollama}`（合法：模块公开包；实测引用清单见 §7.4）；
  - DAG b2-k0.required_contracts 声称「knowledge→commercial 3 条未导出已由 IB1 导出」**与代码不符**（实测三符号仍未导出，见 §6.2 组 E）——按 conventions §5 在 K0.3 上报，不由本程序自行实现导出。
- [ ] **P4 门禁工具可用**：`Makefile:250 verify-module-moves`、`Makefile:255 check-backend-architecture`、`Makefile:262 check-passb-readiness` 目标存在（实测 grep）。
- [ ] **P5 工作树干净**：`codex/passb-b2-k0` worktree 在 `8c45a8815`，`git status` 干净（实测）。

---

## 3. 程序结构与派发序

```text
K0 本文件（SERIAL，status=in_progress，base 8c45a8815）
  ↓ 分配表评审通过（本文件 §5/§6/§7）
K1 ingest(9) | K2 retrieval(29) | K3 wiki+faq(18)   [PARALLEL，各自 worktree/分支]
  ↓（K4 仅依赖 K1+K2）
K4 process/状态机(28) + 18 worker handler 实现       [SERIAL]
  ↓
K5 knowledge 集成（装配 Brief + 别名/shim 删除记录 + 差分汇总）[SERIAL，depends: K4+K3]
  ↓
IB2（29-core-capability-integration，contracts.yaml knowledge 契约区状态回写在此——机制补落盘见 K0.3 差异③）
```

- 子计划文件：`21-knowledge-ingest.md`、`22-knowledge-retrieval.md`、`23-knowledge-wikifaq.md`、`24-knowledge-process.md`（各自节点的计划撰写产物，义务清单见 §9）。
- K5 计划即本文件 §8 任务 K5.1–K5.3。
- 集成分支合并顺序缺省 **K1 → K2 → K3 → K4**（一次一支、审后合并，framework:28）。§6.1 的 R1/R2 裁定使任一子分支独立编译通过，故该顺序只约束集成分支逐步合并后的全仓绿色，不阻塞子分支并行。

---

## 4. 写入所有权与禁改清单

**K0（本节点）可写：**

| 路径 | 依据 |
|---|---|
| `docs/plans/passb/20-knowledge-program.md`（本文件） | DAG b2-k0 `plan_path` |
| `internal/modules/knowledge/kbfreeze/**`（K0.2 新增守卫测试包） | DAG b2-k0 owned_files「K0 节产出的知识共享类型/端口冻结文件…按 20 计划 File Structure」+「internal/modules/knowledge/**（仅 K0 范围）」 |
| `docs/architecture/evidence/passb/b2-k0.md`、`docs/plans/passb/reports/b2-k0.md` | conventions §1.1；DAG b2-k0 evidence_paths |

**K0 禁改（违者节点失败，conventions §1.2/§3）：**

- 84 个 legacy 文件本体（`internal/application/{repository,service,handler}/…`、`internal/handler/session/wiki_fixer_scope.go`）——归 K1-K4；
- 治理文件 `ownership-matrix.yaml` / `contracts.yaml` / `event-catalog.yaml`（只读；knowledge 契约区状态回写归 ib2——注：F2 裁定的 `status: current|planned` 字段在 contracts.yaml 落盘副本中尚不存在，实测 0 处 `status:`、仅 125 处 `stability:`、`knowledge.facade` 记 `stability: frozen`；回写机制须先补落盘，差异③见 K0.3）、`exception-ledger.yaml`（本程序无属主例外的删除动作，删除在 K1/K2/ib2）；
- `internal/modules/knowledge/module.go`（门面注释仅 b0 与该模块集成节点可写，conventions §3）；
- `internal/router/router.go`、`internal/router/task.go`、`internal/router/sync_task.go`、`internal/container/**`、`internal/bootstrap/**`、migration 编号、`go.mod`、`go.sum`（conventions §3 集成工程师独占）；
- `cmd/desktop`、`docreader`、`client`、生产 SQL、既有迁移文件（framework:33）。

**K1-K5 写权限（本计划预先冻结，供子计划引用）：**

- 各子计划可写：`knowledge.yaml` 中归属本 plan 的 legacy 文件条目（含随迁 `_test.go`，framework:29）+ `internal/modules/knowledge/**` 中本域面包 + 该节点自身产出的新文件（含 §6.1 R1 裁定要求的宿主薄 shim 新文件与消费侧 seam 文件）+ 该节点 evidence/report/brief；
- 宿主薄 shim 的删除与跨 owner 宿主调用点改写：仅集成工程师在 ib2 按 Integration Brief 执行（conventions §1.3、§7.2）；
- DAG `status`/`base_sha`/`head_sha`/`task_ids` 回填：仅协调者（conventions §9）。

---

## 5. K0 冻结一：共享类型分配表（Chunk / KnowledgeBase / Tag / semantic）

**裁定 R0（类型单一事实源）**：K1-K4 期间，下表全部共享类型的唯一事实源是 `internal/types`（含 `internal/types/interfaces`）。任何子计划不得在 `internal/modules/knowledge/**` 新增同名/同形影子定义、不得复制类型、不得修改 `internal/types`（不在任何 K 节点 owned_files）；需要类型变更走 conventions §5 升级（ADR/Spec 修订 + 串行契约任务，framework:26）。`internal/types` 的最终拆分不在本程序范围（无 moves manifest 条目、无 plan 属主，spec §5.18 的 types 归属拆分留给后续独立计划）。K0.2 守卫测试强制 R0。

| 类型/常量族 | 现定义位置（base 实测） | K1-K4 期间事实源 | 语义归属（最终 owner plan） | 主要跨域消费者 |
|---|---|---|---|---|
| `types.Chunk` / `ChunkType` / `ChunkStatus` / `ChunkFlags` / `ImageInfo` / `VideoInfo` | `internal/types/chunk.go:113` / `:13` / `:43` / `:54` / `:86` / `:102` | `internal/types` | K1（21-ingest） | K2/K3/K4、conversation、agentruntime tools、router/rbac |
| `types.KnowledgeBase` / `KnowledgeBaseConfig` / `AutoTagConfig` / `ChunkingConfig` / `StorageConfig` | `internal/types/knowledgebase.go:59` / `:152` / `:178` / `:244` / `:372` | `internal/types` | K2（22-retrieval） | K1/K4、datasource、airesource、commercial（model_usage 绑定） |
| `types.Knowledge` / `KnowledgeListFilter` / `ManualKnowledgePayload` / `KnowledgeSearchScope` / `KnowledgeCheckParams` | `internal/types/knowledge.go:125` / `:96` / `:268` / `:278` / `:485` | `internal/types` | K4（24-process） | K1/K2/K3、conversation、craft、insights |
| `types.KnowledgeTag` / `KnowledgeTagWithStats` / `TagReferenceCounts` / `KnowledgeTagRelation` | `internal/types/tag.go:12` / `:56` / `:63` / `:70` | `internal/types` | K2（22-retrieval） | K4（knowledge_tag 关联）、handler/tag |
| `types.TagScope` / `types.SearchResult` / `types.SearchParams` | `internal/types/search.go:19` / `:151` / `:230` | `internal/types` | K2（检索面）；消费只读 | conversation chat_pipeline、agentruntime tools、agentcatalog 推荐位 |
| `types.WikiPage` / `types.WikiFolder`（及 wiki_page.go 内 Revision/Issue/Stats/Graph 族） | `internal/types/wiki_page.go:191` / `:419` | `internal/types` | K3（23-wikifaq） | K4、agentruntime wiki tools（10 个消费文件，contracts.yaml:1866-1882） |
| `types.SemanticModelWireRequest` / `SemanticModelMessage` / `SemanticModelParameters` / `SemanticModelCapability` / `SemanticModelIssuedCapability` / `SemanticModelInvocationResult` / `SemanticModelInvocationDisposition` / `SemanticModelInvocationClaim` 等 | `internal/types/semantic_model.go:7` 起 | `internal/types` | K2（22-retrieval）；语义引擎包 `internal/modules/knowledge/semantic`（Pass A 已就位，client.go/client_test.go）不动 | commercial（预算）、airesource、handler/semantic_* |

**端口接口归属（B0 已冻结，K0 确认归属并纳入守卫）：** `ChunkService`（`internal/types/interfaces/chunk.go`）、`KnowledgeBaseService`（`…/knowledgebase.go`）、`KnowledgeService`（`…/knowledge.go`）、`KnowledgeTagService`（`…/tag.go`）、`WikiPageService`（`…/wiki_page.go`）、`RetrieveEngineService`（`…/retriever.go`）——六个 `contracts.yaml` `capability-port`（id：`knowledge.chunk-service`:1576、`knowledge.knowledge-base-service`:1627、`knowledge.service`:1773、`knowledge.tag-service`:1843、`knowledge.wiki-page-service`:1860、`knowledge.retrieve-engine-service`:1726，`stability: frozen`）。实现体的落位：ChunkService→K1、KnowledgeBaseService/KnowledgeTagService/RetrieveEngineService→K2、WikiPageService→K3、KnowledgeService→K4（与 §6 文件归属同源）。接口签名变化 = 契约违约，须 ADR/Spec 修订（framework:26）。

**知识门面（`knowledge.facade`，contracts.yaml:1613，items：NewModule/RegisterRoutes/RegisterWorkers/Start/Stop）**：目标态口径（F2 裁定；注：F2 要求的 `status: current|planned` 字段在 contracts.yaml 落盘副本中尚不存在——实测 0 处 `status:`、125 处 `stability:`，facade 记 `stability: frozen`，差异③见 K0.3）。K0 **不**发明具体 Go 签名（「不发明接口」纪律；门面今日零实现，`internal/modules/knowledge/module.go` 为 22 行注释骨架）；精确签名由 K5（b2-k-integration）在实装时按当时的 `internal/router`/`internal/router/task.go`、`sync_task.go`、`internal/container/container.go:1053` 真实 seam 推导并在其 Integration Brief 中给出，交集成工程师切换。

---

## 6. K0 冻结二：跨 plan 未导出符号联动裁定

**背景（实证）**：DAG `package_private_couplings` 只登记跨模块 owner 对；本节是 K0 对**知识域内部（K1↔K2↔K3↔K4）同宿主包未导出符号互耦**的实测与裁定（符号级 grep 核实，方法与 DAG ppc 同口径）。这是 framework:132「K1-K3 并行前提」的实质内容。

### 6.1 通用裁定

**R1（定义方三件套）**——跨 plan/跨 owner 消费的未导出符号，其**定义文件属主**搬迁该文件时，同一分支内完成：
1. 实现体 `git mv` 至 `ownership-matrix.yaml` 登记的 destination（随迁 `_test.go`）；
2. 落位包内为下表符号添加**导出薄包装**（一行委托，不复制实现），命名 = 首字母大写同义（如 `previewText`→`PreviewText`）；
3. 宿主包仍存在的其他 owner 调用方需要继续编译时，在宿主原目录创建**薄 shim 新文件**（仅转发声明，如 `func resolveDeadSlug(…) (string, bool) { return kbretrievalapp.ResolveDeadSlug(…) }`，宿主→模块 import 方向合法，Pass A 同例），文件头注释 `// Pass B 过渡 shim：删除点 ib2（Integration Brief 登记）`。shim 登记入该节点 Integration Brief 与报告。

**R2（调用方消费侧 seam）**——调用方属主搬迁自己文件时，对他 plan 未导出符号的调用点改为**消费侧注入 seam**（服务结构体函数字段或构造注入接口，spec §4.2「接口优先定义在使用方模块」；禁止包级可变变量绕过注入，spec §4.3）。seam 的生产实现由 K5 装配按 Integration Brief 接到定义方导出包装；子计划测试以 fake 注入（行为由既有特征化测试锚定，见 §10）。跨 owner 宿主调用点的最终改写由集成工程师在 ib2 执行（conventions §1.3）。

**R3（同属主调用点直接改）**——定义与调用同属一个 K plan 时，随该 plan 的搬迁直接更新调用点到落位包，无需 shim/seam。实测例（K2 内）：`service/kbshare.go:311/387/511`（`applyTenantRoleCap`，定义同文件 :70）、`repository/tag.go:110/121`（`escapeLikeKeyword`）、`service/tag.go:85`（`resolveKBReadTenant`）；`recordKBActivity` 的 K2 内调用文件（grep -rln 实证）：`service/kbshare.go`、`service/tag.go`、`service/knowledgebase.go`。

### 6.2 联动符号全表（定义 file:line 与签名均为 base 实测）

**组 A——K3 定义、K1 消费：**

| 符号 | 定义 | 签名 | 调用点（属主） | 裁定 |
|---|---|---|---|---|
| `previewText` | `internal/application/service/wiki_ingest.go:1383`（K3） | `func previewText(s string, maxRunes int) string` | `extract.go:309`、`image_multimodal.go:280`、`image_multimodal.go:296`（均 K1）；`wiki_ingest.go:1410/1436/1438/1496`（K3 内部，R3） | K1 按 R2 建 seam（K5 接 K3 导出 `PreviewText`）；K3 搬 `wiki_ingest.go` 时宿主已无 K1 调用方（K1 先并），仍按 R1.3 留 shim 以保任意合并序绿色 |

**组 B——K2 定义、K3 消费：**

| 符号 | 定义 | 签名 | 调用点（属主） | 裁定 |
|---|---|---|---|---|
| `recordKBActivity` | `service/kb_activity.go:95`（K2） | `func recordKBActivity(ctx context.Context, audit interfaces.AuditLogService, tenantID uint64, kbID string, action types.AuditAction, targetType string, targetID string, outcome types.AuditOutcome, details map[string]any)` | K3：`knowledge_faq.go:252/475/667/853/1309`、`knowledge_faq_import.go:241/2237`；K4（5 文件，grep -rln 实证）：`knowledge_create.go`、`knowledge_clone_move.go`、`knowledge_delete.go`、`knowledge_replace.go`、`knowledge_process.go`；K2 内 R3：`kbshare.go`、`tag.go`、`knowledgebase.go(service)`、`kb_activity.go:195`；datasource：`datasource_service.go` 17 处（:258/:374/:420/:462/:525/:778/:915/:1065/:1111/:1127/:1201/:1301/:1325/:1461/:1529/:1853/:2425） | K3/K4 按 R2 建 seam；K2 按 R1 导出 + 宿主 shim（datasource 未搬至 ib2 改写前） |
| `kbActivityTrigger` | `service/kb_activity.go:36`（K2） | `func kbActivityTrigger(ctx context.Context) string` | K3：`knowledge_faq_import.go:245/2249`；K4：`knowledge_create.go:285/489/742/882/1020/1148`、`knowledge_clone_move.go:441/1057`、`knowledge_delete.go:728`、`knowledge_process.go:4034`、`knowledge_replace.go:206`；datasource：`datasource_service.go:837` | 同上 |
| `withKBActivityTask` | `service/kb_activity.go:27`（K2） | `func withKBActivityTask(ctx context.Context, taskID, trigger string) context.Context` | K3：`knowledge_faq_import.go:2249`；K4：`knowledge_clone_move.go:441/1057`、`knowledge_delete.go:728`、`knowledge_process.go:4034`；datasource：`datasource_service.go:837/1405` | 同上 |
| `kbActivityAppendSampleTitles` | `service/kb_activity.go:49`（K2） | `func kbActivityAppendSampleTitles(details map[string]any, titles ...string)` | K3：`knowledge_faq.go:1308` | 同上 |
| `withKBActivitySuppressed` | `service/kb_activity.go:87`（K2） | `func withKBActivitySuppressed(ctx context.Context) context.Context` | datasource：`datasource_service.go:1642/1790/2106`（全仓实测仅此 3 处，无 K3/K4 调用方） | K2 按 R1 导出 + 宿主 shim（datasource 调用点 ib2 改写） |
| `resolveDeadSlug` | `service/slug_fuzzy.go:91`（K2） | `func resolveDeadSlug(deadSlug string, displayText string, liveSlugs map[string]struct{}, titleToSlug map[string]string) (string, bool)` | K3：`wiki_ingest.go:1670`、`wiki_page.go:1191` | K3 按 R2 建 seam；K2 按 R1 导出 + 宿主 shim（K3 未并前） |
| `resolveKBReadTenant` | `service/knowledgebase_access.go:21`（K2） | `func resolveKBReadTenant(ctx context.Context, kb *types.KnowledgeBase, shares access.KBShareLookup) (uint64, error)` | K3：`knowledge_faq.go:36`；K2 内：`tag.go:85`（R3） | K3 按 R2 建 seam；K2 按 R1 导出 + 宿主 shim |
| `kbReadPermissions` | `service/knowledgebase_access.go:14`（K2） | `func kbReadPermissions(ctx context.Context, shares access.KBShareLookup) *access.KBPermissions` | K3：`knowledge_faq.go:36` 经 `resolveKBReadTenant`；K4：`knowledge.go:830`；conversation：`session_knowledge_qa.go:459`；K2 内：`knowledgebase_search_shared.go:45/83`（R3） | K4 按 R2 建 seam；conversation 调用点由集成工程师 ib2 改写（R1 shim 过渡） |

**组 C——K4 定义、跨 plan 消费（K4 串行在后，R1 即可）：**

| 符号 | 定义 | 签名 | 调用点（属主） | 裁定 |
|---|---|---|---|---|
| `escapeLikeKeyword` | `repository/knowledge.go:22`（K4；B0.2 消歧注记显式归 24） | `func escapeLikeKeyword(keyword string) string` | identity：`repository/tenant.go:85/90`；conversation：`repository/message.go:215`、`repository/session.go:209`；K2：`repository/tag.go:110/121`（R2 seam）；K4 内 R3 | K2 对 `repository/tag.go` 搬迁按 R2 建 seam；K4 搬出时导出窄端口 + 宿主薄 shim（identity/conversation 调用点 ib2 改写，b0 消歧注记同律） |
| `withKnowledgeCleanup` | `service/knowledge_delete_plan.go:22`（K4） | `func withKnowledgeCleanup(ctx context.Context, tenant uint64, bindings map[string]string) context.Context` | datasource：`datasource_service.go:885`；K4 内：`knowledge_delete.go:766`、`knowledge_delete_plan.go:71`（R3） | K4 导出 + 宿主 shim，datasource 调用点 ib2 改写 |
| `deleteReferencedKnowledge` | `service/knowledge_delete_plan.go:36`（K4） | `func deleteReferencedKnowledge(ctx context.Context, svc interfaces.KnowledgeService, expectedKB string, ids []string) error` | conversation：`message.go:449/469`、`session.go:687/764/821`、`web_search_state.go:124`；insights：`evaluation.go:366`；airesource：1 处（ppc 登记，实施时定位）；K4 内：`knowledge_transfer.go:357`（R3） | K4 导出 + 宿主 shim，conversation/insights/airesource 调用点 ib2 改写 |
| `isValidFileType` | `service/knowledge_util.go:62`（K4） | `func isValidFileType(filename string) bool` | K4 内：`knowledge_create.go:76`（R3）。**无跨 owner 调用方**：`handler/session/attachment_processor.go:80` 调用的是同文件 :305 的 package session 本地 `func isValidFileType(fileName string) bool`（conversation 属主，与 K4 符号同名无耦合，实测） | K4 直接随文件迁移，无需导出/shim；同名并存事实登记 Integration Brief 防后续误判 |
| `RecordWikiContentActivity`（已导出） | `service/kb_activity.go:181`（K2） | 见源码 | K3 wiki 域调用 | K3 搬迁后经 K2 落位包导入，无需 seam |

**组 D——K1 定义、跨 owner 消费（DAG b2-k-ingest notes 义务）：**

| 符号 | 定义 | 签名 | 调用点（属主） | 裁定 |
|---|---|---|---|---|
| `buildVLMCaptionPrompt` | `service/image_multimodal.go:53`（K1） | `func buildVLMCaptionPrompt(ctx context.Context, cfg types.VLMConfig) string` | conversation：`temporary_document.go:560`；K1 内：`image_multimodal.go:289`（R3） | K1 导出 + 宿主 shim，conversation 调用点 ib2 改写 |
| `sanitizeOCRText` | `service/ocr_sanitizer.go:29`（K1） | `func sanitizeOCRText(raw string) string` | conversation：`temporary_document.go:541`；K1 内：`image_multimodal.go:276`（R3） | 同上 |

**组 E——知识文件出向（定义属其他模块；与 b2-k0 required_contracts 直接相关）：**

| 符号 | 定义（属主） | 调用点（属主） | 实测导出状态 | 裁定 |
|---|---|---|---|---|
| `knowledgeBaseModelUsageBindings` | `repository/model_usage.go:8`（12-commercial，destination `internal/modules/commercial/repository`） | K4：`repository/knowledgebase.go:242` | **未导出** | K4 按 R2 建 seam；导出义务属 12-commercial/ib1 补课，K0.3 上报；ib2 装配接线 |
| `scopeKnowledgeBasesByModelID` | `repository/model_usage.go:57`（12-commercial） | K4：`repository/knowledgebase.go:216/235` | **未导出** | 同上 |
| `semanticBudgetFailure` | `service/semantic_model_budget.go:114`（12-commercial，destination `internal/modules/commercial/service`） | K2：`semantic_model.go:102` | **未导出** | K2 按 R2 建 seam；其余同上 |
| `auditActor` / `auditActorRole` | identity 属主文件（`tenant_member.go:162` 等） | K2：`kb_activity.go:157/160` | 未导出 | K2 按 R2 建 seam（随 kb_activity 搬迁），K5/ib2 接 identity 导出 |
| `getParserEngineOverridesFromContext` | conversation 属主（`attachment_processor.go:334` 附近） | K4：`knowledge.go`、`knowledge_process.go` | 未导出 | K4 按 R2 建 seam |

**裁定约束汇总**：R1/R2 使任一子分支独立 `go build ./...` 绿色；宿主 shim 仅在「宿主仍有其他 owner 调用方」时创建，全部登记删除点 **ib2**（与 `ownership-matrix.yaml` 全部 84 行 `delete_barrier: ib2` 一致）。

---

## 7. K0 冻结三：契约、路由、worker、hook、别名、例外清单指针

### 7.1 路由（11 项，`knowledge.routes`，contracts.yaml:1750；切换归 K5 Brief）

`RegisterChunkerDebugRoutes`（routes_knowledge.go:18）、`RegisterChunkRoutes`（:28）、`RegisterKnowledgeRoutes`（:67）、`RegisterFAQRoutes`（:143）、`RegisterKnowledgeBaseRoutes`（:185）、`RegisterSemanticModelPolicyRoutes`（:254）、`RegisterKnowledgeBaseActivityRoutes`（:266）、`RegisterKnowledgeTagRoutes`（:279）、`RegisterWikiPageRoutes`（:309）、`RegisterSemanticInternalRoutes`（routes_infra.go:14）、`serveKBScopedFiles`（files.go，router.go:332 调用）。

### 7.2 Worker（18 项，`knowledge.workers`，contracts.yaml:1893；注册行禁改，K4 交付 RegisterWorkers、K5 出 Brief）

`TypeChunkExtract`、`TypeDataTableSummary`、`TypeDocumentProcess`、`TypeManualProcess`、`TypeFAQImport`、`TypeQuestionGeneration`、`TypeSummaryGeneration`、`TypeKBClone`、`TypeKnowledgeMove`、`TypeKnowledgeListDelete`、`TypeKnowledgeListReparse`、`TypeIndexDelete`、`TypeKBDelete`、`TypeImageMultimodal`、`TypeKnowledgePostProcess`、`TypeKnowledgeAutoTag`、`TypeWikiIngest`、`TypeWikiFinalize`（注册位点：`internal/router/task.go:266-320`、`internal/router/sync_task.go:143-163`；K5 装配时按当时真实行号复核）。

### 7.3 生命周期与事件

- Hook 1 项：`recoverPendingWikiTasks`（`internal/container/container.go:1053` `must(container.Invoke(...))` 挂接，func at `internal/container/recover_pending_wiki_tasks.go:32`）→ K3/K5 切模块 Start/Stop 时恢复行为零变化（knowledge-wikifaq brief 边界目标）。
- 事件 4 项 v1（`event-catalog.yaml`「Knowledge 家族」区，`replay: source-query` 口径，F4 裁定）：`knowledge.processing.completed`（producer `repository/knowledge.go:CompleteProcessingWithoutSubtasks`）、`knowledge.processing.failed`（:147，producer `knowledge_housekeeping.go:runSweep`）、`knowledge.index.completed`（:157，producer `chunk.go:CreateChunks`）、`knowledge.deletion.completed`（:168，producer `repository/knowledge.go:HardDeleteKnowledge`）。K1-K4 搬迁不得改 producer 语义、metadata 键与 ordering 语义。

### 7.4 AI Resource 消费（现状即合规，禁新增内部包 import）

knowledge 范围生产代码对 `internal/modules/airesource` 的 import（实测）：`docparser/weknoracloud_http_reader.go:16`（models/utils）、`retriever/composite.go:13`（models/embedding）、`retriever/keywords_vector_hybrid_indexer.go:13/14`（models/embedding、models/utils）、`service/extract.go:15/16`（chat、embedding，K1）、`service/graph.go:16/17`（chat、utils，K2）、`service/image_multimodal.go:15/16`（utils/ollama、vlm，K1）、`service/knowledge_auto_tag.go:12`（chat，K4）、`service/knowledge_faq_import.go:17`（embedding，K3）、`service/knowledge_process.go:17/18`（chat、embedding，K4）、`service/knowledgebase_search.go:9`（embedding，K2）、`service/semantic_model.go:13`（chat，K2）、`service/wiki_ingest.go:18`、`wiki_ingest_batch.go:17`、`wiki_ingest_cite.go:14`、`wiki_ingest_taxonomy.go:13`（chat，K3）。其中前 4 处即例外台账 exc-0088/0089/0090/0091（见 §7.6），其余为对模块公开包的合法消费，搬迁时随文件走、不改语义。

### 7.5 别名（18 条，`knowledge.yaml:41-77`，删除在 K5/ib2）

`internal/application/repository/retriever/{doris,elasticsearch,elasticsearch/v7,elasticsearch/v8,milvus,neo4j,opensearch,postgres,qdrant,sqlite,tencentvectordb,weaviate}`、`internal/application/service/retriever`、`internal/infrastructure/chunker`、`internal/infrastructure/docparser`、`internal/infrastructure/docparser/anydoc`、`internal/infrastructure/semantic`、`internal/searchutil`（全部 `passb_task: B-knowledge`）。K5 逐条 `rg` 证明零非测试 importer 后记录删除；`container.go:36-38` 的 3 行旧路径 import（`internal/application/repository`、`internal/application/service`、`internal/application/service/file`，base 实测；knowledge-process brief:58 的「:40-64 13 行」表述已过时，以实测为准）改写由集成工程师按 K5 Brief 执行（knowledge-process brief 删除义务 3）。

### 7.6 例外（本程序属主 4 条，remove_at=ib2；removal owner 在 K1/K2）

| id | from | to | plan |
|---|---|---|---|
| exc-0088 | `internal/modules/knowledge/docparser/weknoracloud_http_reader.go` | `internal/modules/airesource/models/utils` | 21-knowledge-ingest |
| exc-0089 | `internal/modules/knowledge/retriever/composite.go` | `internal/modules/airesource/models/embedding` | 22-knowledge-retrieval |
| exc-0090 | `internal/modules/knowledge/retriever/keywords_vector_hybrid_indexer.go` | `internal/modules/airesource/models/embedding` | 22-knowledge-retrieval |
| exc-0091 | `internal/modules/knowledge/retriever/keywords_vector_hybrid_indexer.go` | `internal/modules/airesource/models/utils` | 22-knowledge-retrieval |

消费方例外（非本程序属主，K0 不动）：conversation→`knowledge/searchutil` 9 对（exc-0070/0074/0075/0076/0077/0078/0083/0084/0085，owner 35-conversation-program，remove_at ib3）、agentruntime→`knowledge/searchutil` 等（owner 32/33，ib3）。K2 保持 `searchutil` 包路径与 API 稳定即可，无删除动作。

---

## 8. 任务（业务完整、可独立审阅；一任务一 commit，conventions §4）

### Task K0.1 — 冻结分配表与联动裁定表（本文件 §5–§7）

- [ ] 以 base `8c45a8815` 工作树复核 §5 表内每个 file:line（`grep -n "^type …" internal/types/…`）与 §6 表内每个签名/调用点（本计划撰写时已实测一轮，复核命令见 §8.3 报告要求）。
- [ ] 复核 §5 分配表与 `ownership-matrix.yaml` 的 84 行（plan/destination）零漂移：`python3 -c` 读 yaml 统计 `plan` 计数 = `{21:9, 22:29, 23:18, 24:28}`（预期输出与 §2 裁定 2 一致）。
- [ ] 评审通过后 §5/§6/§7 即为 K1-K5 的约束事实源；子计划与本表冲突时以本表 + B0 治理 yaml 为准，仍冲突走 conventions §5 升级。
- **产出文件**：本文件。**验收**：评审 approved；84 行计数核对记录写入报告。

### Task K0.2 — kbfreeze 守卫测试包（R0 与六 port 签名的机器强制）

**文件**：新建 `internal/modules/knowledge/kbfreeze/freeze_test.go`（package `kbfreeze`，仅测试文件，无生产代码）。包依赖仅 `internal/types/interfaces`（module 公开契约），不 import 任何 legacy 宿主包（架构守护合规）。

**测试 1：`TestKnowledgeSharedTypesHaveNoShadowDefinitions`**——遍历 `internal/modules/knowledge/**` 全部 `.go` 文件（跳过 `kbfreeze` 目录自身与 `_test.go`），对以下冻结类型名做 `^type\s+(Name)\b` 行级匹配；命中且不在豁免表内即 `t.Errorf`：`Chunk, ChunkType, ChunkStatus, ChunkFlags, ImageInfo, VideoInfo, KnowledgeBase, KnowledgeBaseConfig, AutoTagConfig, ChunkingConfig, StorageConfig, Knowledge, KnowledgeListFilter, KnowledgeSearchScope, KnowledgeTag, KnowledgeTagWithStats, TagReferenceCounts, TagScope, SearchResult, SearchParams, WikiPage, WikiFolder, SemanticModelWireRequest, SemanticModelMessage, SemanticModelParameters, SemanticModelCapability, SemanticModelIssuedCapability, SemanticModelInvocationResult, SemanticModelInvocationDisposition, SemanticModelInvocationClaim`（清单 = §5 表）。**豁免表（唯一初始条目，包内路径→类型名）**：`internal/modules/knowledge/chunker/splitter.go → Chunk`——该类型是 docreader 递归文本切分器的分段概念（splitter.go:14 注释「Chunk represents a piece of split text with position tracking」；字段 Content/ContextHeader/Seq/Start；chunker 包实测零 import `internal/types`，与 `types.Chunk` 无引用关系），属合法同名异义，须显式登记防止 K0.2 无法 GREEN；豁免表新增条目 = 本计划冻结表修订，走评审。

**测试 2：`TestFrozenCapabilityPortInterfacesUnchanged`**——用 `reflect.TypeOf((*interfaces.X)(nil)).Elem()` 断言六个 B0 冻结端口签名不漂移：
- `ChunkService`：方法名集合精确等于 18 项冻结清单（CreateChunks, GetChunkByID, GetChunkByIDOnly, ListChunksByKnowledgeID, ListPagedChunksByKnowledgeID, UpdateChunk, UpdateChunks, DeleteChunk, DeleteChunks, DeleteChunksByKnowledgeID, DeleteByKnowledgeList, ListChunkByParentID, GetRepository, DeleteGeneratedQuestion, UpdateDocumentChunk, ListChunkRevisions, RevertDocumentChunk, UpsertGeneratedQuestion——转录自 contracts.yaml:1580，并与 `internal/types/interfaces/chunk.go:142-185` 实测一致）；
- `KnowledgeTagService`：方法名集合精确等于 7 项（ListTags, CreateTag, UpdateTag, DeleteTag, FindOrCreateTagByName, DeleteOrphanTagByName, ProcessIndexDelete——contracts.yaml:1847）；
- `KnowledgeService`：`MethodByName` 断言存在 `CreateKnowledgeFromFile`、`ProcessDocument`、`SearchKnowledgeForScopes`（create/worker/search 三锚点）；
- `KnowledgeBaseService`：存在 `HybridSearch`、`ResolveEmbeddingModelKeys`、`ProcessKBDelete`；
- `WikiPageService`：存在 `RepairContentLinks`、`RevertPageToVersion`、`PruneEmptyFolderChains`；
- `RetrieveEngineService`：存在自有 9 方法 `Index, BatchIndex, EstimateStorageSize, CopyIndices, DeleteByChunkIDList, DeleteBySourceIDList, DeleteByKnowledgeIDList, BatchUpdateChunkEnabledStatus, BatchUpdateChunkTagID`（内嵌 `RetrieveEngine` 的方法不参与断言）。

**步骤与命令（worktree 根执行）：**

```bash
# 测试 1 现状即 GREEN：冻结名在 internal/modules/knowledge 内唯一命中
# chunker/splitter.go:26 Chunk，已入豁免表（base 实测，见测试 1 规格）。
# RED 验证方式：临时从豁免表移除该条目或临时注入一个假想影子名断言，确认测试会 FAIL，随后恢复。
go test ./internal/modules/knowledge/kbfreeze/ -count=1 -v
# 预期：2 个测试 PASS，exit 0
go build ./...
# 预期：exit 0（kbfreeze 仅测试文件，不参与 build 产物）
make check-backend-architecture
# 预期：exit 0
make verify-module-moves
# 预期：exit 0
```

- **节点 DAG gates（三项）与上述一致，全绿才算节点完成**（conventions §2：不得以更快等价检查替代）。
- **验收**：两条测试 PASS；守卫包未 import `internal/application/*`、`internal/handler/*`（`go list -deps` 抽查）；diff 仅含 `internal/modules/knowledge/kbfreeze/freeze_test.go`。commit：`test(passb): b2-k0 知识共享类型与端口冻结守卫`。

### Task K0.3 — 前置差异上报、证据与报告

- [ ] **差异①（commercial 三符号未导出）**：将 §6.2 组 E 实测（`repository/model_usage.go:8/:57`、`service/semantic_model_budget.go:114` 均小写；`git show 8c45a8815 --stat` 证明 ib1 零实施）写入报告，并按 conventions §5 在报告中给出裁定建议（导出义务归 12-commercial / ib1 补课；ib2 装配接线；K2/K4 以 R2 seam 解耦，不阻塞本程序）。禁止自行导出对方文件符号。
- [ ] **差异②（b0 两张未闭环修复工单 + BLOCKED 记录）**：报告照录 §2 裁定 3 与 P2，标注「派发 K1-K3 前协调者须确认收口」。
- [ ] **差异③（F2 status 机制未落盘）**：F2 裁定要求 contracts.yaml 每条契约记录含必填 `status: current|planned` 字段（facade 五操作应记 planned、由 barrier 回写），但落盘副本实测 0 处 `status:`（仅 125 处 `stability:`，`knowledge.facade`:1613 记 `stability: frozen`，DAG b2-k0 shared_resources 亦以「状态回写归 ib2」为前提）。据此：K5 的「门面 current 化」与 ib2 的状态回写在字段补落盘前无载体。报告按 conventions §5 上报裁定建议：由协调者裁决补落盘归属（b0 修复工单批次或 ib2 首个回写动作先行补字段）；补落盘前 K5 仅交付门面实装与装配 Brief，不声称契约状态变更。
- [ ] **执行证据命令包（conventions §1.2，base 用 DAG 给定值）：**

```bash
PASSB_BASE_SHA=8c45a88153d0b20088252dbb29d2fb3815b253c2
git diff --stat "$PASSB_BASE_SHA"...HEAD      # 逐条对照 owned_files
go build ./...                                 # 预期 exit 0
go test -count=1 ./internal/modules/knowledge/kbfreeze/   # 预期 exit 0
git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort      # 与 owned_files 求差集，差集非空即失败
```

- [ ] 写 `docs/architecture/evidence/passb/b2-k0.md`：前置条件 P1-P5 核对结果（命令原文+退出码）、§5/§6 冻结表复核记录、84 行计数输出、K0.2 测试输出、差异①②③登记。
- [ ] 写 `docs/plans/passb/reports/b2-k0.md`：执行命令台账、变更清单 vs owned_files 逐条核对结论、未完成项如实列出（conventions §1.2）。
- [ ] 向协调者回填报据：DAG `b2-k0` 建议置 `review`、`task_ids=[K0.1,K0.2,K0.3,K5.1,K5.2,K5.3]`（回填动作本身归协调者，conventions §9）。
- **验收**：证据/报告双文件在节点分支提交；差集为空；`go test -count=1 ./tools/passbguard ./tools/modulemove` 仍绿（确认 K0 未破坏治理校验）。commit：`docs(passb): b2-k0 冻结证据与前置差异上报`。

### Task K5.1 — 装配切换 Integration Brief（b2-k-integration 交付物 1）

- [ ] 汇总 K1-K4 的 Brief/报告，产出 `docs/architecture/passb/briefs/b2-k-integration.md`，至少含：(a) 11 路由注册的模块门面切换表（§7.1 全表，宿主→门面逐一列出）；(b) 18 worker 的 RegisterWorkers 交付与 `router/task.go:266-320`、`sync_task.go:143-163` 注册行切换申请（集成工程师独占执行）；(c) `recoverPendingWikiTasks`（container.go:1053 挂接）切模块 Start/Stop 的等价性说明；(d) §6 全部 seam 的装配接线表（seam → 定义方导出符号 → 落位包）；(e) 宿主薄 shim 清单与 ib2 删除批；(f) 推导后的五操作精确签名（按当时真实 seam，不发明）。
- [ ] Brief 评审通过后，装配切换由集成工程师串行执行（conventions §3）；K5 不直接改 `router/container/task/sync_task` 文件。
- **验收**：Brief 文件评审 approved；切换后 `go build ./...` 与 `go test ./internal/modules/knowledge/... -count=1` 绿。commit：`docs(passb): b2-k-integration 装配 Brief`。

### Task K5.2 — 别名/例外/shim 删除记录与收口核对

- [ ] §7.5 的 18 别名逐条执行 `rg -l "<old import path>" --type go | grep -v _test`，零非测试 importer 者记录回收（有 importer 者如实登记残留与属主）；`container.go:36-38` 3 行旧 import 改写申请并入 K5.1 Brief。
- [ ] 核对 exc-0088/0089/0090/0091 的删除状态（removal owner 21/22 已完成的收集证据；未完成的登记 ib2 收口）。
- [ ] 宿主薄 shim（§6 R1.3 全部产物）清点：逐一登记「定义落位包 + 导出符号 + 宿主余留调用方（应仅剩集成工程师 ib2 改写项）」。
- **验收**：删除记录入 evidence；18 别名状态逐条有结论（回收/残留+属主）。commit：`docs(passb): b2-k-integration 别名与 shim 删除记录`。

### Task K5.3 — 差分汇总、门禁与节点收口

- [ ] 复核 K1-K4 差分证据（§10 高风险面四项）双跑与等价比对结论齐全（conventions §6；缺项即 K5 不完成并上报）。
- [ ] 执行 b2-k-integration DAG gates：`go build ./...`、`go test ./internal/modules/knowledge/... -count=1`、`make check-backend-architecture`、`make check-passb-readiness`、`make verify-module-moves`（全绿）。
- [ ] 计数奇偶三方一致复核（conventions §8）：633 路由 / 23+23 worker / 58 hook / 537 migration——`passbguard`/`architectureguard` 实测 == `docs/architecture/evidence/pass-a-acceptance.md` 台账 == manifests 发现值；装配切换后 18 knowledge worker 在 Redis/Lite 双栈各注册一次（IB2 全量回归在此）。
- [ ] 写 `docs/architecture/evidence/passb/b2-k-integration.md`。commit：`docs(passb): b2-k-integration 差分与门禁证据`。

---

## 9. K1-K4 子计划派发义务（子计划撰写者必须内嵌以下冻结约束）

| 子计划 | 文件数 | 必须内嵌 |
|---|---:|---|
| `21-knowledge-ingest.md`（b2-k-ingest） | 9 | destination `internal/modules/knowledge/ingest`（matrix 逐行）；组 D 两个符号的 R1（导出+宿主 shim，conversation 调用点 `temporary_document.go:541/560`）；组 A 的 R2 seam（`previewText` 3 调用点）；随迁测试：`ocr_sanitizer_test.go`（已确认存在，随 `ocr_sanitizer.go` 迁移），其余 8 文件的关联 `_test.go` 以 `ls` + `grep -l` 在实施第一步逐文件判定；差分：`TypeChunkExtract` + `knowledge.index.completed` 事件等价；例外 exc-0088 删除 |
| `22-knowledge-retrieval.md`（b2-k-retrieval） | 29 | destination `internal/modules/knowledge/retrieval/app`；组 B 全部 R1/R2/R3 裁定与组 E `semanticBudgetFailure` seam；组 C `escapeLikeKeyword` 的 K2 侧 seam（`repository/tag.go:110/121`）；随迁：`kb_activity_test.go`、`slug_fuzzy_test.go`（已确认存在）、`handler/tag_delete_test.go`（随 `handler/tag.go`）；`datasource_service_test.go` **不随迁**——其属主文件 `datasource_service.go` 归 datasource（`moves/datasource.yaml:59`，DAG 独立节点 b2-datasource），仅作差分锚点引用（contracts.yaml:1857 登记）；差分：`TypeIndexDelete` tag 侧 + HybridSearch/融合/FAQ 混排行为等价（「检索/融合/排序行为是外部契约」，knowledge-retrieval brief）；例外 exc-0089/0090/0091 删除 |
| `23-knowledge-wikifaq.md`（b2-k-wikifaq） | 18 | destination `internal/modules/knowledge/wiki`（12 文件）与 `…/faq`（6 文件，matrix 逐行）；组 B 的 R2 seam（recordKBActivity 族 13 调用点＝recordKBActivity 7 + kbActivityTrigger 2 + withKBActivityTask 1 + kbActivityAppendSampleTitles 1 + resolveKBReadTenant 经 knowledge_faq.go:36、resolveDeadSlug 2 调用点）；`wiki_fixer_scope.go`（`internal/handler/session/`，`*Handler` 方法 `resolveWikiFixerTenantScope:18` + 包级 `resolveBuiltinWikiFixerTenantScope:36`）去方法化裁定（B0.3 Step 3 + conventions §7.4）；`recoverPendingWikiTasks` 行为不变；随迁：`wiki_ingest_test.go`、`knowledge_faq_create_guard_test.go`、`wiki_ingest_dedup_test.go`、`wiki_page_revision_test.go`、`wiki_folder_prune_finalize_test.go`（`knowledge_move_wiki_test.go` **不随迁**——白盒耦合 K4 未导出符号，见 K4 行）；差分：`TypeWikiIngest`/`TypeWikiFinalize`/`TypeFAQImport` + hook 恢复等价 |
| `24-knowledge-process.md`（b2-k-process） | 28 | destination `internal/modules/knowledge/process`；组 C 全部 R1（导出+宿主 shim；`isValidFileType` 无跨 owner 调用方、直接随迁）与组 E 两个 seam（commercial 2、conversation 1）；随迁含 `knowledge_move_wiki_test.go`——该测试是 `package service` 白盒测试（:163 调 `svc.moveOneKnowledge`，:167/:235 直取 `svc.repo`，被测未导出方法 `moveOneKnowledge` 定义于 K4 属主文件 `knowledge_clone_move.go:1194`；contracts.yaml:1884 将其登记于 wiki-page-service characterization_tests，但按白盒耦合随 K4 迁移，K3 wiki 断言部分由 K4 保持或拆分，残差登记 Brief）；18 worker handler 实现交付 RegisterWorkers（注册行禁改，集成工程师独占）；`knowledge_housekeeping.go` 独占清扫规则、System 经 `KnowledgeHousekeeping` 窄端口调度（B0.3 Step 4 注记）；差分：`TypeKnowledgeListDelete`/`TypeIndexDelete`/`TypeKBDelete` 删除级联 + `ProcessDocument` 重试/幂等 + `knowledge.deletion.completed` 事件等价 |

共性义务：随迁全部 `_test.go`（framework:29）；随迁测试若白盒引用他 plan 属主文件的未导出符号（如 `knowledge_move_wiki_test.go` 之于 K4），以**定义文件属主**为随迁归属，不得为编译把他 plan 文件拉入本计划（framework:25：manifest 是文件所有权唯一事实源），残差登记 Integration Brief；禁止双写/双注册/复制实现（framework:30）；每个搬迁 commit 按 M2/M3 分离（spec §13）；节点 gates = `go build ./...` + `go test ./internal/modules/knowledge/... -count=1` + `make check-backend-architecture` + `make verify-module-moves`（DAG 各节点）。

---

## 10. 测试与高风险差分要求

**复用现有（contracts.yaml characterization_tests 登记；分两类处置）：**
- **随属主文件迁移（白盒/同属主）**：`embed_channel_chunk_test.go`、`knowledge_auto_tag_test.go`、`knowledge_post_process_wiki_enqueue_test.go`、`knowledge_replace_test.go`、`knowledge_write_access_test.go`、`handler/knowledge_mutation_admission_test.go`、`handler/rbac_lookups_test.go`（chunk-service / knowledge-service 面，按其被测文件随 K1-K4 迁移）；`knowledge_move_wiki_test.go`（**随 K4**，白盒耦合 `knowledge_clone_move.go:1194 moveOneKnowledge`，见 §9）、`wiki_folder_prune_finalize_test.go`、`wiki_ingest_dedup_test.go`、`wiki_page_revision_test.go`（K3）、`handler/tag_delete_test.go`（K2，随 `handler/tag.go`）。
- **跨 plan 差分锚点（文件不迁移，双跑时引用）**：`agentruntime tools/scope_authorization_test.go` 与 wiki tools 测试 6 件（agentruntime 属主，B3 才迁移）、`datasource_service_test.go`（datasource 属主，DAG 节点 b2-datasource）。

**K0 自身测试**：K0.2 两条守卫测试（新写）。

**高风险差分（framework:40 knowledge deletion/indexing；spec §14.3 同输入双跑比对输出/错误/DB 结果/任务参数/副作用；证据入各节点 evidence 差分章节，conventions §6）：**

| 面 | owner 节点 | 锚定用例（现状基线） |
|---|---|---|
| 分块索引写入与 `knowledge.index.completed` | K1 | `chunk.go:CreateChunks` 事件 producer 语义；`ChunkStatusIndexed` 可见性（event-catalog ordering 字段） |
| 索引清理 tag 侧与检索融合排序 | K2 | `TypeIndexDelete`（tag 侧）双栈行为；`HybridSearch` fanout/fusion/FAQ 混排（contracts.yaml:1627 消费面） |
| Wiki/FAQ 摄取与恢复 | K3 | `TypeWikiIngest`/`TypeWikiFinalize`/`TypeFAQImport` 幂等；`recoverPendingWikiTasks` 重启恢复 |
| 删除级联与重试幂等 | K4 | `TypeKnowledgeListDelete`/`TypeIndexDelete`/`TypeKBDelete` 删除计划先行序；`knowledge.deletion.completed` 恰一次；asynq 重试语义零变化 |

差分失败只修新实现，不得改期望值迎合（spec §14.3）。legacy 删除前差分必须已通过（framework:31）。

---

## 11. 集成与回滚边界

- **集成边界**：K1-K4 分支在 K5 汇聚，一次一支、审后合并（framework:28），缺省序 K1→K2→K3→K4；装配切换（router/container/task/sync_task/bootstrap）仅集成工程师按 K5.1 Brief 串行执行；contracts.yaml knowledge 区状态回写仅 ib2（conventions §3；F2 status 字段未落盘，先决条件见差异③）；宿主 shim 与跨 owner 宿主调用点改写仅 ib2。
- **回滚边界**（spec §13）：
  - K0 产物（本文件 + kbfreeze 测试 + 证据）为 M1 类提交，回滚 = revert 对应 commit，无装配影响；
  - K1-K4 各子分支的搬迁为 M2/M3 类提交，分支内回滚不触碰集成；已并入集成分支的 M2/M3 保留（新路径未接线不影响运行）；
  - K5 装配切换（M4 类）失败先回退切换 commit；M2/M3 新路径保留为未接线代码；
  - 本程序无 schema 变更、无 migration，回滚不需要数据修复；shim/别名删除（ib2）在任何残留 importer 未清零前禁止执行（framework B5 口径前移）。
- **升级边界**：门禁不可能通过、或需改 §5/§6 冻结表时，按 conventions §5 上报协调者（报告 + DAG notes），禁止现场改判所有权或扩大例外。

---

## 12. 独立验收标准

**K0 节点（本节点）：**
1. DAG b2-k0 gates 三项全绿，命令原文+退出码记录于 `docs/plans/passb/reports/b2-k0.md`；
2. `git diff --name-only 8c45a8815...HEAD | sort` 与 §4 K0 可写清单差集为空；
3. §5 分配表与 `ownership-matrix.yaml` 84 行零漂移（计数 9/29/18/28 复核输出在报告）；
4. `go test -count=1 ./internal/modules/knowledge/kbfreeze/` PASS（2 测试）；
5. 治理文件（ownership-matrix/contracts/event-catalog/exception-ledger）零改动（`git diff --name-only` 验证）；
6. 差异①②如实登记于 evidence 与报告（禁止省略未完成项）；
7. 计划评审 approved（reviewer 独立出 `docs/plans/passb/reviews/b2-k0.md`）。

**K5 节点：** §8 K5.1-K5.3 验收全项 + b2-k-integration gates 五项全绿 + 计数三方一致。

**程序级（IB2 前提）：** 84 legacy 文件全部搬迁且宿主删除完成至 matrix `delete_barrier: ib2` 口径；18 别名状态有逐条结论；4 条属主例外删除或登记 ib2；§6 全部联动符号经 R1/R2 收敛且有差分证据；`go test ./internal/... -count=1` 于 ib2 全绿。

---

## 13. 计划自检记录（撰写时执行；含 2026-09-23 审校修复轮）

- **Spec 覆盖**：ask 九要素 → §1（Spec 指针）、§2（前置）、§4（文件与写权）、§5-§7（接口签名与冻结指针）、§10（行为兼容与差分）、§8（步骤/命令/预期）、§11（集成与回滚）、§7.5-7.6（删除义务）、§12（验收）——全覆盖。
- **无占位符**：全部符号含 file:line 与真实签名（撰写与修复轮逐条 grep/读源码实证）；无 TBD；「实施时定位」仅剩组 E `deleteReferencedKnowledge` 的 airesource 1 调用点一处，且给出权威登记源（DAG package_private_couplings）与定位义务人（24 子计划实施第一步），属登记事实而非未决设计（`withKBActivitySuppressed` 调用点已于修复轮全量实测补齐：datasource_service.go:1642/1790/2106）。
- **类型一致性**：§5 表类型名/位置与 `internal/types/*.go` 实测一致（修复轮纠正 chunk.go 四处行号：ChunkStatus:43、ChunkFlags:54、ImageInfo:86、VideoInfo:102）；六 port 方法名清单转录自 contracts.yaml 冻结签名并与接口源码核对（ChunkService 18 方法 = `interfaces/chunk.go:142-185`）。
- **跨任务接口一致性**：§6 联动表 ↔ DAG b2-k-* notes 义务（kb_activity 归 K2、withKnowledgeCleanup/deleteReferencedKnowledge/isValidFileType 归 K4、buildVLMCaptionPrompt/sanitizeOCRText 归 K1、kbReadPermissions/applyTenantRoleCap 归 K2）逐一对应；§7.6 与 exception-ledger 四行一致；§7.1/7.2 与 contracts.yaml knowledge.routes/workers 一致。
- **与上游冲突如实登记**：DAG required_contracts「commercial 3 符号已由 IB1 导出」与代码不符（§6.2 组 E 实测，差异①）；b0 两修复工单未闭环（差异②）；F2 status 字段未落盘（差异③）——均按 conventions §5 升级路径处理，不在实现期静默重新设计。
- **审校修复轮（2026-09-23）已处置**：① K0.2 影子扫描增加 chunker/splitter.go:26 Chunk 豁免（实测唯一命中、chunker 包零 import types，测试现状可 GREEN）；② `knowledge_move_wiki_test.go` 随迁归属 K3→K4（白盒耦合 `knowledge_clone_move.go:1194`）；③ `datasource_service_test.go` 改为不随迁的差分锚点（datasource 属主，framework:25）；④ §5 chunk.go 行号四处纠正；⑤ `isValidFileType` 撤销误派的宿主 shim（attachment_processor.go:80/:305 为 session 包本地同名函数）；⑥ 组 B 调用点全量枚举补齐；⑦ container.go 旧 import 坐标 :40-64→:36-38（3 行）；⑧ framework 行号以集成副本为准（:132、:120-121）；⑨ `recoverPendingWikiTasks` 挂接行 :1052→:1053；⑩ F2 status 未落盘登记为差异③并修订 §3/§4/§11/K5 相关表述。
