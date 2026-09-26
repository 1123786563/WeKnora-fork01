# Pass B2 — 22-knowledge-retrieval（K2 检索域 29 legacy 文件归位）

**DAG 节点：** `b2-k-retrieval`（phase B2，role work，execution_mode parallel，depends_on `b2-k0`）。
**Brief：** `docs/architecture/passb/knowledge-retrieval.md`（29 文件子集）。**程序协调计划：** `docs/plans/passb/20-knowledge-program.md`（K0 冻结 §5–§7 为本计划约束事实源）。
**Architecture：** Go 1.26、Gin、GORM、dig、Testify、SQLite 内存测试；门禁工具 `tools/modulemove` / `tools/architectureguard` / `tools/passbguard`。
**本节点不实现 module.go 门面接线**（`internal/modules/knowledge/module.go` 零逻辑骨架与装配归 K5/ib2，conventions §3）；本节点产出模块包、导出端口、宿主兼容层、差分证据与 Integration Brief。

## 1. Spec 与事实源指针（全部只读输入）

| 事实源 | 用途 |
|---|---|
| `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §5.2（Knowledge 所有权）、§11（B2 并行图）、§12（Pass B 循环：特征测试→domain/application→高风险差分→装配→legacy 删除）、§13（提交隔离 M1–M5 与回滚）、§14（测试梯度与差分门禁）、§15（治理规则：例外精确路径）、§16（停止条件）、§17.2（完成标准） | 行为与流程约束 |
| `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md:29`（每搬一生产文件随迁 `_test.go`）、`:112-133`（Knowledge mini-program：K2=29 文件，K0 后与 K1/K3 并行）、Global Constraints（禁双写/禁复制/禁改 router/container/go.mod） | 结构约束 |
| `.superpowers/sdd/passb/conventions.md` §1（派发契约）、§2（gates）、§3（禁改清单）、§4（提交规范）、§5（升级契约）、§6（高风险差分）、§7（package-private 耦合）、§8（计数基线三方一致）、§10 Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP / 2026-09-24-IMPORT-EXCEPTION-REGISTRY / 2026-09-24-WAVE-DEP-BASELINE | 执行公约 |
| `docs/architecture/passb/ownership-matrix.yaml`（plan=22-knowledge-retrieval 共 29 行，destination 全部 `internal/modules/knowledge/retrieval/app`，integration_owner/delete_barrier 全 ib2） | 文件所有权唯一事实源（framework:25） |
| `docs/architecture/passb/contracts.yaml`：`knowledge.knowledge-base-service`（KnowledgeBaseService）、`knowledge.tag-service`（KnowledgeTagService）、`knowledge.retrieve-engine-service`（RetrieveEngineService）、`knowledge.routes`、`knowledge.workers`（全部 `stability: frozen`） | 签名冻结，变化=ADR/Spec 修订（framework:26） |
| `docs/architecture/passb/exception-ledger.yaml` exc-0089/0090/0091（plan=22-knowledge-retrieval，remove_at=ib2）、exc-0070/0074-0078/0083-0085（conversation→searchutil，owner 35-conversation-program） | 例外台账 |
| `docs/architecture/passb/event-catalog.yaml`「Knowledge 家族」4 事件（producer 均不在本节点 29 文件内，见 20 计划 §7.3） | 只读核对 |
| `docs/plans/passb/20-knowledge-program.md` §5（R0 类型单一事实源）、§6.2 组 B/C/E（联动符号签名与裁定）、§7.4-7.6、§9（22 行派发义务）、§10-§11 | K0 冻结约束（位于 `codex/passb-b2-k0` 分支，K2.1 基线对齐后可读） |
| `docs/architecture/moves/knowledge.yaml`（:87 repository/knowledge.go、:155 kb_activity.go 等行）+ `docs/architecture/passb/b0-evidence.md:86`（B0.2 裁定：`repository/knowledge.go`（escapeLikeKeyword 宿主）显式归 `24-knowledge-process`） | manifest 行删除义务与消歧 |
| 先例计划 `docs/plans/passb/10-identity.md`（已评审 approved）：§3.3 semanticScopeGuard 本地 seam、§5 宿主兼容模板、B1-ID.2 Step 推迟件编排 | 机制先例（下文逐处引用） |

## 2. 前置条件

1. `b2-k0` done（K0.1–K0.3 完成：20 计划 §5–§7 冻结表 + `internal/modules/knowledge/kbfreeze/` 守卫测试落盘）。2026-09-24 的 BLOCKED 已按 DAG 调度解除。
2. **基线对齐 merge**（Ruling 2026-09-24-WAVE-DEP-BASELINE）：开工前把 `codex/passb-b2-k0` 合入本节点分支（独立 merge commit，性质=基线对齐非集成合并）；对齐后重跑 §2.3 前置核验。module.go 冲突=两侧注册全保留；涉冻结签名取舍的冲突停下升级（conventions §5）。
3. K1（`codex/passb-b2-k-ingest`）、K3（`codex/passb-b2-k-wikifaq`）与本节点并行，互不依赖（framework:117-119）；本节点不需要其产物。
4. 门禁工具三件在本分支可运行：`go run ./tools/modulemove verify --all`、`go run ./tools/architectureguard`、`go run ./tools/passbguard -root .`（K2.1 Step 2 实测）。
5. 节点 gates（DAG `b2-k-retrieval.gates`，逐条原样执行，禁止替代命令）：`go build ./...`、`go test ./internal/modules/knowledge/... -count=1`、`make check-backend-architecture`、`make verify-module-moves`。

## 3. 范围、目标布局与推迟件

### 3.1 本节点 29 文件（ownership-matrix plan=22-knowledge-retrieval 全量，与 knowledge-retrieval.md:5-18 逐行一致）

| 层 | 文件（`internal/application/` 前缀省略） |
|---|---|
| repository 6 | `repository/kbshare.go`、`repository/semantic_model_invocation.go`、`repository/semantic_model_policy.go`、`repository/semantic_outbox.go`、`repository/semantic_scope_epoch.go`、`repository/tag.go` |
| service 19 | `service/graph.go`、`service/kb_activity.go`、`service/kbshare.go`、`service/knowledgebase.go`、`service/knowledgebase_access.go`、`service/knowledgebase_search.go`、`service/knowledgebase_search_fanout.go`、`service/knowledgebase_search_faq.go`、`service/knowledgebase_search_fusion.go`、`service/knowledgebase_search_results.go`、`service/knowledgebase_search_shared.go`、`service/knowledgebase_search_storegroup.go`、`service/semantic_model_capability.go`、`service/semantic_model_policy.go`、`service/semantic_scope.go`、`service/slug_fuzzy.go`、`service/tag.go`、`service/tag_access.go`、`service/semantic_model.go`（**推迟，见 3.3**） |
| handler 4 | `handler/knowledgebase.go`（**推迟，见 3.3**）、`handler/semantic_internal.go`、`handler/semantic_model_policy.go`、`handler/tag.go` |

### 3.2 目标布局（matrix destination `internal/modules/knowledge/retrieval/app` 的具体化）

29 行 destination 同为 `…/retrieval/app`，但 `tag.go`（repository/service/handler 各一）与 `semantic_model_policy.go`（同）等同名文件不可同目录，故按宿主同构分三个子包（同 `internal/application/{repository,service,handler}` → `internal/modules/knowledge/retrieval/app/{repository,app,handler}`；与 b1 分层先例一致，落位包名对齐宿主便于 diff 审查）：

| 宿主包 | 落位包（import path） |
|---|---|
| `internal/application/repository` | `internal/modules/knowledge/retrieval/app/repository`（package repository） |
| `internal/application/service` | `internal/modules/knowledge/retrieval/app`（package app） |
| `internal/handler` | `internal/modules/knowledge/retrieval/app/handler`（package handler） |

3 个落位包均在节点 owned_files `internal/modules/knowledge/**（retrieval 面）`内；布局具体化在 K2.2 首个搬迁 commit 落定并写入 Integration Brief 与报告（matrix destination 词干不变）。`git mv` 保留原文件名，`git diff --summary --find-renames` 须逐文件 R100。

### 3.3 推迟件（2 个；物理迁移推迟至 ib2，先例=10-identity.md §3.2 八推迟件 + conventions §1.3/§5）

| 推迟文件 | 根因（base 实测证据） | 解除编排（写入 Integration Brief，ib2 执行） |
|---|---|---|
| `internal/application/service/semantic_model.go` | 与 `semantic_model_budget.go`（12-commercial，matrix delete_barrier=ib1，本分支仍在宿主）**双向未导出互耦**：① `SemanticModelGateway.budget` 字段与 `NewSemanticModelGateway`（semantic_model.go:46/:50）形参类型为宿主类型 `*SemanticModelBudgetAdapter`；② :96 构造 `semanticCapability{}`、:102 调 `semanticBudgetFailure`（semantic_model_budget.go:114）；③ 反向：adapter 四方法（semantic_model_budget.go:45/68/77/89）形参与方法体读 `semanticCapability` 未导出字段（semantic_model.go:31，`owner/modelID/funding/priceVersion/runID/callID/upper/deadline`）。导出/seam 两路均不可行：改 `NewSemanticModelGateway` 签名=断 container.go:1067（集成工程师独占文件）；导出字段=必须同步改 commercial 文件方法体（禁改他 owner 文件）；module↛宿主 import（宿主 compat→module 成环）。 | ib2 把 `semantic_model.go` 与 `semantic_model_budget.go` 同批切割：commercial 侧落位 `internal/modules/commercial/service` 并导出窄端口（budget ops 接口 + `SemanticBudgetFailure`），gateway 形参改端口，装配由集成工程师按 Brief 改 container.go 一处；随后本文件按 K2.4 同法迁移（含 manifest 行删除）。矩阵/manifest 行保留（Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP：行随物理迁移删除）。期间 `semantic_model.go` 留守宿主零改动，全部既有引用（container.go:1059-1069 `newUnavailableSemanticModelGateway`、宿主 compat 类型别名，见 K2.3 Step 5）天然编译。 |
| `internal/handler/knowledgebase.go` | `rbac_lookups.go`（conversation 属主、b1 推迟件）在 `*KnowledgeBaseHandler` 上声明方法：`KBCreatorLookup`（:34）、`KBCreatorLookupFromKbIDParam`（:49）——Go 不允许跨包定义方法（"cannot define new methods on non-local type"），且 rbac_lookups.go 与 router 调用形参均为本节点禁改文件（conventions §3、§7.4；先例=10-identity.md:79 同型裁定）。另其构造器 `NewKnowledgeBaseHandler`（:50）宿主服务类型形参链（`*knowledgeService`/`*userService`/`*agentShareService`/`*vectorStoreService` 等）依赖宿主未导出类型。 | ib2 先对 `rbac_lookups.go` 两条方法去方法化（改包级函数 `func KBCreatorLookup(h *KnowledgeBaseHandler, c *gin.Context)`，router 调用形参同步改——integrator/conversation 双侧由集成工程师执行），再按 K2.6 同法迁移本文件（宿主 compat type/var 别名 → 直连）。`handler/knowledgebase*_test.go` 7 件随该文件届时迁移；本节点期间全部留宿主，零改动。 |

推迟不改变两文件的 ownership-matrix 属主（仍 22-knowledge-retrieval）与 `delete_barrier: ib2`；推迟项在报告「未完成项」如实列出（conventions §1.2），不视为节点失败（b1 八推迟件先例，评审 approved）。

## 4. 写入所有权与禁改清单

**可写**：§3.1 的 27 个物理搬迁文件及其随迁 `_test.go`（§7.1 表）；3 个落位新包及包内新文件（本地 seam 文件，见 §5）；3 个宿主兼容文件 `internal/application/{repository,service}/kbretrieval_passb_compat.go`、`internal/handler/kbretrieval_passb_compat.go`（**横向目录新生产文件必须同 commit 登记进 `docs/architecture/moves/knowledge.yaml` legacy_files，`passb_task: B-knowledge`，否则 legacy-guard 报诊断**——先例 10-identity.md:94 实测）；`docs/architecture/moves/knowledge.yaml`（仅本节点 29 行的删除与 compat 行登记，Ruling LEGACY-ROW-OWNERSHIP 行级删除权）；`internal/modules/knowledge/legacy/README.md` 镜像；`tools/architectureguard/check.go` importExceptions 数据行 + `docs/architecture/passb/exception-ledger.yaml` 属主行（仅限 K2.7 按 Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY 执行的精确登记）；本节点自身产出：`docs/architecture/evidence/passb/b2-k-retrieval.md`、`docs/plans/passb/reports/b2-k-retrieval.md`、`docs/architecture/passb/briefs/b2-k-retrieval.md`。

**禁改**（conventions §3）：`internal/router/router.go`、`internal/router/routes_*.go`、`internal/router/files.go`、`internal/container/container.go`、`internal/bootstrap/{routes,workers,lifecycle}.go`、`internal/router/task.go`、`internal/router/sync_task.go`、migration 编号、`go.mod`/`go.sum`、生产 SQL、既有迁移文件；`docs/architecture/passb/{ownership-matrix,contracts,event-catalog}.yaml`（barrier 回写）；`internal/modules/knowledge/module.go`；他 owner 宿主文件（含 `semantic_model_budget.go`、`rbac_lookups.go`、`repository/knowledge.go`、`tenant_member.go`、K1/K3/K4 与 datasource/airesource 等全部未搬迁文件）；`cmd/desktop`、`docreader`、`client`。

**搬迁机械律**：每个生产文件同 commit 删除 manifest `legacy_files` 对应行（modulemove 对"行在盘缺"报 `legacy-file: not found` 退 1，10-identity.md:37 实测）；函数体一行不改（M2 纯移动），兼容层重线独立 commit（M3），禁止混合关注点（conventions §4）。

## 5. 耦合面裁定（核心机制；全部 base 实测，签名与 20 计划 §6.2 一致）

### 5.1 R1 导出义务：K2 定义的未导出符号被留守方消费 → 落位包导出改名 + 宿主 compat 一行委托

| 符号（定义 file:line，搬迁后落位包） | 签名（base 实测） | 留守消费方（grep 实证） | 导出名 |
|---|---|---|---|
| `kbActivityTrigger`（service/kb_activity.go:36） | `func kbActivityTrigger(ctx context.Context) string` | K3：knowledge_faq_import.go:245/2249；K4：knowledge_create/clone_move/delete/process/replace.go；datasource：datasource_service.go:837 | `KBActivityTrigger` |
| `withKBActivityTask`（kb_activity.go:27） | `func withKBActivityTask(ctx context.Context, taskID, trigger string) context.Context` | K3：knowledge_faq_import.go:2249；K4：knowledge_clone_move.go:441/1057 等；datasource：datasource_service.go:837/1405 | `WithKBActivityTask` |
| `kbActivityAppendSampleTitles`（kb_activity.go:49） | `func kbActivityAppendSampleTitles(details map[string]any, titles ...string)` | K3：knowledge_faq.go:1308；K4：knowledge_delete.go | `KBActivityAppendSampleTitles` |
| `withKBActivitySuppressed`（kb_activity.go:87） | `func withKBActivitySuppressed(ctx context.Context) context.Context` | datasource：datasource_service.go:1642/1790/2106（全仓仅 3 处，20 计划 §6.2 组 B 同结论） | `WithKBActivitySuppressed` |
| `recordKBActivity`（kb_activity.go:95） | `func recordKBActivity(ctx context.Context, audit interfaces.AuditLogService, tenantID uint64, kbID string, action types.AuditAction, targetType string, targetID string, outcome types.AuditOutcome, details map[string]any)` | datasource 17 处（:258…:2425）；K3 knowledge_faq*.go 7 处；K4 5 文件；K2 内 R3：kbshare.go、tag.go、knowledgebase.go、kb_activity.go:195 | `RecordKBActivity` |
| `RecordWikiContentActivity`（kb_activity.go:181，已导出） | `func RecordWikiContentActivity(ctx context.Context, audit interfaces.AuditLogService, tenantID uint64, kbID string, actions map[string]int)` | K3：wiki_ingest_batch.go（宿主裸名调用） | compat 函数别名（不改名） |
| `resolveDeadSlug`（slug_fuzzy.go:91） | `func resolveDeadSlug(deadSlug string, displayText string, liveSlugs map[string]struct{}, titleToSlug map[string]string) (string, bool)` | K3：wiki_ingest.go:1670、wiki_page.go:1191 | `ResolveDeadSlug` |
| `kbReadPermissions`（knowledgebase_access.go:14） | `func kbReadPermissions(ctx context.Context, shares access.KBShareLookup) *access.KBPermissions` | K4：knowledge.go:830；conversation：handler/session/session_knowledge_qa.go:459（会话宿主调用点 ib2 改写，R1 shim 过渡）；K2 内 R3：knowledgebase_search_shared.go:45/83 | `KBReadPermissions` |
| `resolveKBReadTenant`（knowledgebase_access.go:21） | `func resolveKBReadTenant(ctx context.Context, kb *types.KnowledgeBase, shares access.KBShareLookup) (uint64, error)` | K3：knowledge_faq.go:36；K2 内：tag.go:85（R3） | `ResolveKBReadTenant` |
| `requireKBWrite`（knowledgebase_access.go:31） | `func requireKBWrite(ctx context.Context, kb *types.KnowledgeBase) (context.Context, error)` | K4：knowledge.go、knowledge_delete_plan.go、knowledge_write.go | `RequireKBWrite` |
| `withKBWriteTenantInfo`（knowledgebase_access.go:55） | `func withKBWriteTenantInfo(ctx context.Context, kb *types.KnowledgeBase, tenants interfaces.TenantRepository) (context.Context, error)` | K4：knowledge_create.go、knowledge_delete_plan.go | `WithKBWriteTenantInfo` |
| `applyTenantRoleCap`（kbshare.go:70） | `func applyTenantRoleCap(p types.OrgMemberRole, callerTenantRole types.TenantRole) types.OrgMemberRole` | agentcatalog：agent_share.go（25；DAG ppc agentcatalog→knowledge 3 调用点）；K2 内 R3：kbshare.go:311/387/511 | `ApplyTenantRoleCap` |

机制（b1 §3.1 先例）：落位包内改名为首字母大写同义（函数体一行不改，包内调用点随 R3 清单同步改名），宿主 compat 文件留一行委托（如 `func recordKBActivity(ctx context.Context, audit interfaces.AuditLogService, tenantID uint64, kbID string, action types.AuditAction, targetType string, targetID string, outcome types.AuditOutcome, details map[string]any) { return app.RecordKBActivity(ctx, audit, tenantID, kbID, action, targetType, targetID, outcome, details) }`），**跨 owner 调用点文件本身不改**，重线与 shim 删除走 Integration Brief 由集成工程师在 ib2 执行（conventions §1.3、§7.2）。已导出符号（`RecordWikiContentActivity`、`ErrInvalidTenantID`）留 compat 函数/var 别名（b1「2 var」先例）。

### 5.2 R2 seam：K2 文件消费他属主未导出符号 → 本地同形 seam（先例=10-identity.md §3.3「同形本地 seam」，ib2 按 conventions §7.1 收口）

| K2 消费方（file:line） | 他属主符号（定义） | seam 形态（本节点） |
|---|---|---|
| `repository/tag.go:110/:121` | `escapeLikeKeyword`（repository/knowledge.go:22，**24-knowledge-process 属主**，b0-evidence.md:86 B0.2 消歧） | 落位 `app/repository` 新文件 `escape_like_seam.go`：同签名 `func escapeLikeKeyword(keyword string) string`（5 行 strings.ReplaceAll 逐字对齐 knowledge.go:24-28），文件头注 `// Pass B 过渡 seam（R2）：K4 搬出导出窄端口后由 ib2 收口（10-identity.md §3.3 先例）`。tag.go 调用点零改动。 |
| `service/kb_activity.go:157/:160` | `auditActor` / `auditActorRole`（tenant_member.go:162/:156，identity 属主 b1 推迟件；K0 §6.2 组 E 裁定「K2 按 R2 建 seam，K5/ib2 接 identity 导出」） | 落位 `app` 内随 `kb_activity.go` 同包提供同签名函数（体=对冻结公共 API 的同一委托：`types.UserIDFromContext` / `types.TenantRoleFromContext`，与 tenant_member.go:157-165 逐字等价），kb_activity.go 调用点零改动。K2.5 在 evidence 登记「多属主重复 helper 族」，ib2 与 identity 导出的 `AuditActor`/`AuditActorRole` 收口（conventions §7.1）。 |
| `service/knowledgebase_access.go:38` | （留守接收者）`func (s *knowledgeService) writableFAQKnowledgeBase`（接收者类型 knowledgeService 定义于 knowledge.go，K4 留守；全仓唯一"K2 文件定义在留守类型上的方法"，扫描脚本见 §13 自检） | **方法再归置**：该方法文本随 K2.4 从 knowledgebase_access.go 迁出，落宿主 compat 文件（接收者类型仍在宿主，方法体一行不改；K4 调用点 knowledge_faq.go/knowledge_faq_import.go 经 R2 由其属主处理，本节点不改）。迁移是再归置非复制（单一实现）。 |

**无需 seam 的澄清**：`semanticBudgetFailure`/`SemanticModelBudgetAdapter` 互耦随 `semantic_model.go` 推迟整体消失（两文件同留宿主，零改动）；`slug_fuzzy.go` 对 `slugify`/`stripDeadWikiLinks`/`surfaceGrams` 的命中均为注释引用（slug_fuzzy.go:29/31/48/53/171 实测），无代码耦合。

### 5.3 R3 同属主调用点（随搬迁直接改名/改 import，无 shim）

`service/kbshare.go:311/387/511`（applyTenantRoleCap）、`service/tag.go:85`（resolveKBReadTenant）、`service/knowledgebase_search_shared.go:45/83`（kbReadPermissions）、`kb_activity.go:195`（recordKBActivity）；`service/semantic_scope.go:114`（`repository.SemanticControlRepository` 形参——import 从 `internal/application/repository` 改指落位 `app/repository`）；`handler/semantic_model_policy.go:8`（`policy "internal/application/service"` import 改指落位 `app`，别名保留）；`handler/tag.go`/`handler/semantic_internal.go` 的 `interfaces.*` 冻结形参零改动。

### 5.4 semanticScopeGuard 内嵌族（K2 自搬 + 宿主留守双轨）

`semanticScopeGuard`（semantic_scope.go:31，struct 单字段 `semanticInvalidator interfaces.SemanticScopeInvalidator` + `SetSemanticScopeInvalidator`（:35）+ 5 个未导出 invalidate 方法）。嵌入方：K2 自有 `knowledgebase.go`/`kbshare.go`（随迁，内嵌声明零改动）+ 留守 `tenant.go:57`/`organization.go:47`/`tenant_member.go:78`/`user.go:100`（identity 推迟件）与 `knowledge.go`（K4）。裁定（先例=10-identity.md §3.3，b1 已在 identity 包建同形 seam 并登记 ib2 收口）：

1. 落位 `app` 新建 `semantic_scope_guard.go`：结构体名 `semanticScopeGuard`，成员与语义域内使用面一致（`SetSemanticScopeInvalidator` + `invalidateSemanticKB/Tenant/User/Organization/Transfer`，方法体逐字复制 semantic_scope.go:35-66 的委托逻辑，委托目标=冻结端口 `interfaces.SemanticScopeInvalidator`，internal/types/interfaces/semantic_scope.go:11-17）；
2. 原定义从 semantic_scope.go 迁出，落宿主 `internal/application/service/kbretrieval_passb_compat.go`（留守嵌入方继续编译；容器装饰器 container.go:362-368 的 `SetSemanticScopeInvalidator` 类型断言面不变）；
3. 双方均登记 ib2「多属主重复 helper 族」收口（与 identity 的同形 seam、b1 报告偏差合并裁定）。

### 5.5 宿主兼容文件（3 个，模板=10-identity.md §5；先 `go build ./...` 采证再写）

| 文件 | 内容（种子清单，执行时以构建断链证据补全） |
|---|---|
| `internal/application/repository/kbretrieval_passb_compat.go` | var 别名：`NewKBShareRepository`、`NewKnowledgeTagRepository`、`NewSemanticControlRepository`、`NewSemanticModelInvocationStore`、`NewSemanticModelPolicyRepository`；type 别名：`SemanticControlRepository`、`SemanticModelInvocationStore`（container.go:192-195/:303/:1064 引用面）。 |
| `internal/application/service/kbretrieval_passb_compat.go` | §5.1 的 11 个一行委托函数 + `RecordWikiContentActivity`/`ErrInvalidTenantID` 别名 + `semanticScopeGuard` 兼容定义（§5.4.2）+ `writableFAQKnowledgeBase` 方法再归置（§5.2）+ semantic 面 type/var 别名（`SemanticScopeService`、`SemanticScopeResolver`、`SemanticScopeSnapshot`、`NewSemanticScopeService`、`NewSemanticModelPolicyService`、`NewUnavailableSemanticModelPolicyService`、`NewSemanticModelCapabilityIssuer`、`ErrSemanticModel*`/`ErrSemanticScope*` 哨兵——以 container.go:197/:712/:717/:1059-1069 与宿主留守文件构建断链为准逐个补入）。`semantic_model.go`（推迟留守）经这些别名继续编译，其自身零改动。 |
| `internal/handler/kbretrieval_passb_compat.go` | type 别名 `TagHandler`/`SemanticModelPolicyHandler`/`SemanticInternalHandler` + var 别名三个 `New*Handler`（router/routes_knowledge.go:254/:266/:279、routes_infra.go:14、container.go:197/:717/:721 引用面；router 文件零改动）。 |

3 个 compat 文件同 commit 登记进 `docs/architecture/moves/knowledge.yaml` legacy_files（`passb_task: B-knowledge`），镜像 README 同步；删除点 ib2（Brief 指令：直连后删 compat 3 文件 + manifest 3 行，barrier 写权限）。

### 5.6 新显形跨模块 import（K2.7 按 Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY 登记精确豁免）

搬迁使下列既有横向包 import 变为 module→module（architectureguard forbidden-import，check.go:1175-1220 判定「跨模块只能经模块根公共门面」，精确路径豁免是唯一合法通道）。已实证的种子对（importer=落位文件精确路径，imported=精确 import 路径）：

| 落位文件 | import（base 行号实测） |
|---|---|
| `app/knowledgebase.go` | `internal/modules/datasource`（:15）、`internal/modules/policy/access`（:17）、`internal/modules/policy/storageallowlist`（:18） |
| `app/knowledgebase_access.go`、`app/kbshare.go`、`app/tag_access.go` | `internal/modules/policy/access`（knowledgebase_access.go:7 实测；其余执行时 grep） |
| `app/knowledgebase_search.go` | `internal/modules/airesource/models/embedding`（:9，20 计划 §7.4 同录） |
| `app/graph.go` | `internal/modules/airesource/models/chat`（:16）、`internal/modules/airesource/models/utils`（:17） |
| `app/semantic_model_capability.go` | `internal/modules/commercial`（domain，:13） |

程序：独立 commit 在 `tools/architectureguard/check.go` importExceptions 追加精确 file→package 条目（Reason 一行、PassBTask=`K2.7`），同窗在 exception-ledger.yaml 追加对应行（owner=`22-knowledge-retrieval`、remove_at=ib2、reason=预存横向包耦合，Pass B 搬迁显形），机械计数修正（105→105+N）+ §8 基线登记（b2-k-retrieval evidence 登记原因与批准者）+ 纯计数断言随窗机械修正。禁通配、禁新逻辑（Ruling 原文）。

## 6. 公共可观察行为与兼容要求

1. **HTTP 面**：633 条路由、方法、路径、RBAC/APIKey 门、状态码映射零变化——本节点不触 router/middleware；`RegisterKnowledgeBaseRoutes`（routes_knowledge.go:185）、`RegisterSemanticModelPolicyRoutes`（:254）、`RegisterKnowledgeBaseActivityRoutes`（:266）、`RegisterKnowledgeTagRoutes`（:279）、`RegisterSemanticInternalRoutes`（routes_infra.go:14）经 compat 别名继续解析到同一实现。
2. **检索/融合/排序是外部契约**（knowledge-retrieval.md:30-31）：`HybridSearch`/`GetQueryEmbedding`/`ResolveEmbeddingModelKeys`（contracts.yaml `knowledge.knowledge-base-service` frozen）、fanout/fusion/FAQ 混排/分组（knowledgebase_search_fanout.go、_fusion.go、_faq.go、_storegroup.go）、`TypeIndexDelete` tag 侧（`KnowledgeTagService.ProcessIndexDelete`，contracts.yaml `knowledge.tag-service`）行为零变化。
3. **审计面**：`recordKBActivity` 族的去重/抑制/上下文标注/processing_status 映射（kb_activity.go:25-173）逐行为等价；审计条目 `ScopeType=knowledge_base`（kb_activity.go:13）不变。
4. **错误文案与哨兵**：`ErrSemanticScope*`、`ErrSemanticModelCapability*`、`ErrInvalidTenantID`、`apperrors.NewForbiddenError("无权访问该知识库")`（knowledgebase_access.go:28）等既有文本零变化。
5. **装配面**：dig Provider/Decorator 集合零变化——`container.Decorate` 的 `SetSemanticScopeInvalidator` 断言（container.go:362-368）仍成立（§5.4.2 compat）；18 个 worker 注册行（router/task.go:266-320、sync_task.go:143-163）零改动，`KnowledgeTagService.ProcessIndexDelete` 实现搬迁后经冻结接口继续解析。
6. **事件面**：Knowledge 家族 4 事件 producer 均不在本节点文件（20 计划 §7.3），零改动。

## 7. 测试与高风险差分

### 7.1 随迁 `_test.go`（同 commit `git mv`；判定规则：同名主题测试 + 被测对象仅本节点文件；先例=10-identity.md §3.2 表）

| 生产文件 | 随迁 _test.go（已 `ls` 证实存在） | 归属任务 |
|---|---|---|
| repository/semantic_model_invocation.go | `semantic_model_invocation_test.go` | K2.2 |
| repository/semantic_model_policy.go | `semantic_model_policy_test.go` | K2.2 |
| repository/semantic_outbox.go | `semantic_outbox_test.go`、`semantic_outbox_pg_test.go` | K2.2 |
| repository/semantic_scope_epoch.go | `semantic_scope_epoch_test.go` | K2.2 |
| service/semantic_model_capability.go | `semantic_model_capability_test.go` | K2.3 |
| service/semantic_model_policy.go | `semantic_model_policy_test.go`（service 版） | K2.3 |
| service/semantic_scope.go | `semantic_scope_test.go`、`semantic_scope_mutation_test.go` | K2.3 |
| service/knowledgebase.go | `knowledgebase_pr3_test.go`、`knowledgebase_search_budget_test.go`、`knowledgebase_task_cancel_test.go`、`knowledgebase_delete_datasource_test.go` | K2.4 |
| service/knowledgebase_search*.go | `knowledgebase_search_fanout_test.go`、`knowledgebase_search_fusion_test.go`、`knowledgebase_search_dimension_test.go`、`knowledgebase_search_matchcount_test.go`、`knowledgebase_search_results_edit_test.go` | K2.4 |
| service/knowledgebase_search_shared.go / kbshare.go | `knowledge_caller_scope_test.go`、`knowledge_shared_storage_failure_test.go` | K2.4 |
| service/kb_activity.go | `kb_activity_test.go`（20 计划 §9 点名） | K2.5 |
| service/kbshare.go | `knowledge_shared_access_test.go` | K2.5 |
| service/tag.go | `knowledge_write_access_test.go`（tag:5 命中为主；contracts KB/retrieve-engine 特征化测试） | K2.5 |
| service/slug_fuzzy.go | `slug_fuzzy_test.go`（20 计划 §9 点名） | K2.4 |
| handler/semantic_internal.go | `semantic_internal_test.go` | K2.6 |
| handler/semantic_model_policy.go | `semantic_model_policy_test.go`（handler 版） | K2.6 |
| handler/tag.go | `tag_delete_test.go`（20 计划 §9 点名，contracts tag-service 特征化） | K2.6 |

（`semantic_model_test.go` 随推迟件 `semantic_model.go` 留宿主；`repository/kbshare.go` 无同名测试；`service/graph.go`、`tag_access.go`、`knowledgebase_search_faq/storegroup/results/shared.go` 无独立同名测试，行为由上表 fanout/matchcount 等主题测试覆盖。）

### 7.2 留宿主（被测对象为他 owner 文件/推迟件，禁止为编译拉入本计划，framework:25；20 计划 §9/§10 同裁定）

`datasource_service_test.go`（属主 26-datasource，**仅作差分锚点引用**，contracts.yaml tag-service 登记在案）、`vectorstore_test.go`（11-airesource）、`knowledge_tag_test.go`（repository，K4）、`session_tag_targets_test.go`（conversation session.go）、`custom_agent_suggestion_scope_test.go`（25）、`craft_knowledge*_test.go`（41）、`knowledgebase_copy_preflight/hybrid_search*/not_found/pr3_response/pr5_list/request_test.go` 与 `rbac_lookups_test.go`（handler，随推迟件 `handler/knowledgebase.go` 届时迁移）、`knowledge_download/folder/move_gate` 等（K4/conversation）、`chat_pipeline/search_error_test.go`（35）。

### 7.3 高风险差分（framework:40「knowledge deletion/indexing」；spec §14.3 同输入双跑；证据入 `docs/architecture/evidence/passb/b2-k-retrieval.md` 差分章节，conventions §6）

| 面 | 锚定用例（现状基线，随迁后同用例双跑） | 等价判据 |
|---|---|---|
| `TypeIndexDelete` tag 侧索引清理 | `knowledge_write_access_test.go`（tag 5 用例）、`tag_delete_test.go`（force/contentOnly/excludeIDs 分支）、`semantic_scope_epoch_test.go` | 删除路径、索引清理调用序列、返回错误零差异 |
| HybridSearch / 融合 / FAQ 混排 / 分组 | `knowledgebase_search_fanout_test.go`（7+5+4 命中）、`knowledgebase_search_fusion_test.go`、`knowledgebase_search_matchcount_test.go`、`knowledgebase_search_budget_test.go`、`knowledgebase_pr3_test.go`、`knowledgebase_search_dimension_test.go`、`knowledgebase_task_cancel_test.go` | 排序、命中数、预算/取消语义、DB 结果逐用例一致 |
| KB 活动审计流 | `kb_activity_test.go`（6 命中）、`knowledge_shared_access_test.go` | 审计条目字段/details 键序无关比对一致 |
| KB 读权限/租户解析 | `knowledge_caller_scope_test.go`、`knowledge_shared_storage_failure_test.go`、`semantic_scope_test.go`、`semantic_scope_mutation_test.go` | 许可判定与租户切换结果一致 |

差分失败只修新实现、禁改期望值（spec §14.3）。T1/T2 梯度：每任务搬完跑落位包+宿主包测试与 T0 台账比对（新失败即停，定位首个破坏提交，spec §14.1）。

### 7.4 复用与新增

全部复用上表现有测试（contracts.yaml characterization_tests 已登记大半），**不新写行为测试**；新写仅两类机械守卫：① K2.7 后 `go test ./tools/architectureguard/`（若该包有测试）或 `make check-backend-architecture` 零诊断输出采证；② §5.5 compat 完整性以 `go build ./...` + 宿主包测试编译通过为判据（不设独立测试）。

## 8. 任务（业务完整、可独立审阅；一任务一 commit，conventions §4）

> 全部任务属主节点：`b2-k-retrieval`（本计划不承载其他节点任务节；K0/K5 任务见 20-knowledge-program.md）。

### Task K2.1 — 基线对齐、前置核验与 T0 特征化基线

**Files:** 无生产文件改动；产出 `docs/plans/passb/reports/b2-k-retrieval.md` T0 章节。

- [ ] **Step 1: 基线对齐 merge**——`git merge --no-ff codex/passb-b2-k0 -m "merge: baseline alignment with b2-k0 (Ruling 2026-09-24-WAVE-DEP-BASELINE)"`；冲突仅允许 module.go 注释双侧保留，涉冻结签名停下升级。
- [ ] **Step 2: 前置核验（全部留痕）**——`go build ./...` 退出 0；`go test -count=1 ./internal/modules/knowledge/kbfreeze/` PASS（K0 守卫在位）；`ls docs/plans/passb/20-knowledge-program.md` 存在；`make verify-module-moves && make check-backend-architecture` 零诊断。
- [ ] **Step 3: T0 台账**——记录基线 SHA（=`PASSB_BASE_SHA=$(git merge-base origin/main HEAD)` 派发给定值）；逐包跑并记录：`go test -count=1 ./internal/application/repository/ ./internal/application/service/ ./internal/handler/ ./internal/modules/knowledge/...`；输出（含 skip/blocked-env）写入报告 T0 表。
- [ ] **Commit:** 无（台账随 K2.8 报告提交）；如有 merge commit 即本任务产物。

### Task K2.2 — repository 层归位（5 文件物理搬迁 + escapeLikeKeyword seam）

**Files:** Move §7.1 repository 5 行（semantic_model_invocation/semantic_model_policy/semantic_outbox/semantic_scope_epoch/tag + 5 测试）→ `internal/modules/knowledge/retrieval/app/repository/`；Create 落位包 `escape_like_seam.go`（§5.2）、宿主 `internal/application/repository/kbretrieval_passb_compat.go`（§5.5 行 1）；Edit `docs/architecture/moves/knowledge.yaml`（删 5 行、登记 compat 1 行）、镜像 README。

- [ ] **Step 1: 特征化确认**——T0 台账已含 5 个随迁测试结果（K2.1 Step 3）；无需新写。
- [ ] **Step 2: git mv（M2）**——逐文件 `git mv internal/application/repository/semantic_outbox.go internal/modules/knowledge/retrieval/app/repository/` 等（5 生产 + 5 测试）；`git diff --summary --find-renames HEAD` 全 R100。
- [ ] **Step 3: seam 与 import 重线（M3）**——创建 `escape_like_seam.go`（§5.2 行 1，逐字对齐 repository/knowledge.go:24-28）；`repository/tag.go:110/:121` 调用点零改动；包内 import 核对（gorm/interfaces/types 非宿主路径零改动）。
- [ ] **Step 4: 宿主 compat**——先 `go build ./...` 记录断链证据（预期 container.go:192-195/:206/:303 等），写 `kbretrieval_passb_compat.go`（var/type 别名种子=§5.5 行 1）。
- [ ] **Step 5: manifest**——删 5 行加 1 compat 行；镜像 README 5 行标已迁移；`go run ./tools/modulemove verify --module knowledge` 预期 OK。
- [ ] **Step 6: GREEN**——`go build ./...` OK；`go test -count=1 ./internal/modules/knowledge/... ./internal/application/repository/ ./internal/application/service/` 与 T0 台账一致；`make verify-module-moves && make check-backend-architecture` 双绿。
- [ ] **Commit:** `refactor(knowledge): move repository layer of retrieval domain into module (5 files, escapeLikeKeyword seam, host compat)`

### Task K2.3 — service 层归位一：语义模型能力/策略/作用域（3 文件 + guard 双轨）

**Files:** Move `service/semantic_model_capability.go`、`service/semantic_model_policy.go`、`service/semantic_scope.go` + 4 测试（§7.1）→ `internal/modules/knowledge/retrieval/app/`；Create 落位包 `semantic_scope_guard.go`（§5.4.1）、宿主 compat 文件 service 版初版（§5.5 行 2，本任务先落 semantic 面别名与 guard 兼容定义）；Edit manifest（删 3 行加 1 compat 行）、镜像。

- [ ] **Step 1: git mv（M2）**——3 生产 + 4 测试（semantic_model_capability_test、semantic_model_policy_test、semantic_scope_test、semantic_scope_mutation_test）；R100 核验。
- [ ] **Step 2: guard 双轨（M3）**——落位 `semantic_scope_guard.go`（§5.4.1，逐字复制 semantic_scope.go:36-66 委托逻辑）；原定义段迁入宿主 compat（§5.4.2）；`knowledgebase.go`/`kbshare.go` 仍宿主，此时零改动（它们 K2.4/K2.5 才搬，期间经宿主内同名类型照常编译）。
- [ ] **Step 3: semantic 面别名**——宿主 compat 落 `SemanticScopeService`/`SemanticScopeResolver`/`SemanticScopeSnapshot` type 别名 + `NewSemanticScopeService`/`NewUnavailableSemanticModelPolicyService`/`NewSemanticModelPolicyService`/`NewSemanticModelCapabilityIssuer` var 别名 + `ErrSemanticScope*`/`ErrSemanticModelCapability*` 哨兵别名；核验推迟件 `semantic_model.go`（引用 `SemanticScopeResolver`/`SemanticScopeSnapshot`/capability 构造器）与 container.go:197/:712/:717/:1059-1069 零改动编译。
- [ ] **Step 4: 包内重线**——`semantic_scope.go:114` import 改指落位 `app/repository`（K2.2 产物）；`semantic_model_policy.go` 对 `repository.SemanticModelPolicyRepository` 同法。
- [ ] **Step 5: GREEN + manifest**——同 K2.2 Step 5/6（宿主加 `./internal/handler/`）；`go test -count=1 ./internal/application/service/` 与 T0 一致。
- [ ] **Commit:** `refactor(knowledge): move semantic capability/policy/scope services into module (semanticScopeGuard dual-track seam)`

### Task K2.4 — service 层归位二：KB 检索与访问栈（10 文件 + 读权限导出 + 方法再归置）

**Files:** Move `service/knowledgebase.go`、`knowledgebase_access.go`、`knowledgebase_search.go`、`knowledgebase_search_fanout.go`、`knowledgebase_search_faq.go`、`knowledgebase_search_fusion.go`、`knowledgebase_search_results.go`、`knowledgebase_search_shared.go`、`knowledgebase_search_storegroup.go`、`slug_fuzzy.go`、`graph.go` + §7.1 对应 10 测试 → 落位 `app/`；Edit 宿主 compat（追加 §5.1 读权限族 shim：`kbReadPermissions`/`resolveKBReadTenant`/`requireKBWrite`/`withKBWriteTenantInfo` + `resolveDeadSlug` + `ErrInvalidTenantID`）、manifest（删 11 行）、镜像。

- [ ] **Step 1: git mv（M2）**——11 生产 + 10 测试；R100 核验。
- [ ] **Step 2: 导出改名 + 包内 R3（M3）**——`kbReadPermissions`→`KBReadPermissions`、`resolveKBReadTenant`→`ResolveKBReadTenant`、`requireKBWrite`→`RequireKBWrite`、`withKBWriteTenantInfo`→`WithKBWriteTenantInfo`（knowledgebase_access.go）；`resolveDeadSlug`→`ResolveDeadSlug`（slug_fuzzy.go）；包内 R3 调用点同步改名：knowledgebase_search_shared.go:45/83、tag.go:85 不在本批（K2.5）、knowledgebase.go 内部调用点。
- [ ] **Step 3: 方法再归置**——`writableFAQKnowledgeBase`（knowledgebase_access.go:38-51）文本迁入宿主 compat（方法体一行不改；接收者 `*knowledgeService` 留宿主）。
- [ ] **Step 4: 宿主 compat 追加**——4+1 个一行委托函数 + `ErrInvalidTenantID` var 别名；核验 K4 留守调用点（knowledge.go:830、knowledge_delete_plan.go、knowledge_write.go、knowledge_create.go）与 K3（knowledge_faq.go:36）、conversation（session_knowledge_qa.go:459，handler/session 宿主）零改动编译。
- [ ] **Step 5: GREEN + manifest**——`go build ./...`；`go test -count=1 ./internal/modules/knowledge/... ./internal/application/service/` 与 T0 一致（重点：fanout/fusion/matchcount/budget/pr3/task_cancel 双跑一致=§7.3 第 2 行差分首跑）；双守卫绿；manifest 删 11 行。
- [ ] **Commit:** `refactor(knowledge): move KB search/access stack into module (11 files, export read-permission ports)`

### Task K2.5 — service 层归位三：标签、KB 活动与共享（4 文件 + 活动族导出 + auditActor seam）

**Files:** Move `service/kb_activity.go`、`kbshare.go`、`tag.go`、`tag_access.go` + §7.1 对应 3 测试 → 落位 `app/`；Create/Append 落位包内 `auditActor`/`auditActorRole` seam（§5.2 行 2，可并入 kb_activity.go 同 commit 新段）；Edit 宿主 compat（追加活动族 5 shim + `RecordWikiContentActivity` 别名 + `applyTenantRoleCap` shim）、manifest（删 4 行）、镜像。

- [ ] **Step 1: git mv（M2）**——4 生产 + 3 测试（kb_activity_test、knowledge_shared_access_test、knowledge_write_access_test）；R100。
- [ ] **Step 2: 导出改名 + seam + R3（M3）**——`recordKBActivity`→`RecordKBActivity`、`kbActivityTrigger`→`KBActivityTrigger`、`withKBActivityTask`→`WithKBActivityTask`、`kbActivityAppendSampleTitles`→`KBActivityAppendSampleTitles`、`withKBActivitySuppressed`→`WithKBActivitySuppressed`（kb_activity.go）；`applyTenantRoleCap`→`ApplyTenantRoleCap`（kbshare.go）；R3 改名点：kbshare.go:311/387/511、tag.go:85、kb_activity.go:195、kbshare/tag/knowledgebase(service) 内 recordKBActivity 调用点；auditActor/auditActorRole seam 函数随包落位（§5.2 行 2）。
- [ ] **Step 3: 宿主 compat 追加**——5+2 shim/别名；核验留守消费方零改动编译：datasource_service.go（17+4 处）、knowledge_faq*.go（K3）、knowledge_create/clone_move/delete/process/replace.go（K4）、agent_share.go（agentcatalog）、wiki_ingest_batch.go（`RecordWikiContentActivity`）、organization/tenant/tenant_member/user/knowledge.go（guard 兼容已在 K2.3 就位）。
- [ ] **Step 4: GREEN + manifest**——同前模式；`knowledge_write_access_test.go`/`tag` 面双跑一致（§7.3 第 1 行首跑）；双守卫绿。
- [ ] **Commit:** `refactor(knowledge): move tag/kb-activity/kbshare services into module (export activity ports, auditActor seam)`

### Task K2.6 — handler 层归位（3 文件 + 路由兼容别名；knowledgebase handler 推迟落地）

**Files:** Move `handler/semantic_internal.go`、`handler/semantic_model_policy.go`、`handler/tag.go` + 3 测试 → `internal/modules/knowledge/retrieval/app/handler/`；Create `internal/handler/kbretrieval_passb_compat.go`（§5.5 行 3）；Edit manifest（删 3 行加 1 compat 行）、镜像。

- [ ] **Step 1: 推迟件核验（先于搬迁）**——`grep -n "func (h \*KnowledgeBaseHandler)" internal/handler/rbac_lookups.go` 输出 :34/:49 两条采证入报告；`handler/knowledgebase.go` 及其 7 个测试文件确认不动（§3.3）。
- [ ] **Step 2: git mv（M2）**——3 生产 + 3 测试；R100。
- [ ] **Step 3: 重线（M3）**——`semantic_model_policy.go` import `policy` 改指落位 `app`（§5.3）；宿主 compat 落 3 type + 3 var 别名；核验 routes_knowledge.go:254/:279、routes_infra.go:14、container.go:197/:717/:721 零改动编译。
- [ ] **Step 4: GREEN + manifest**——`go test -count=1 ./internal/modules/knowledge/... ./internal/handler/` 与 T0 一致（tag_delete/semantic_internal/semantic_model_policy 双跑）；双守卫绿。
- [ ] **Commit:** `refactor(knowledge): move retrieval handlers into module (route compat aliases, knowledgebase handler deferred)`

### Task K2.7 — 跨模块 import 豁免登记与例外处置（独立 commit）

**Files:** Edit `tools/architectureguard/check.go`（importExceptions 数据行）、`docs/architecture/passb/exception-ledger.yaml`（属主行）；产出豁免登记章节（报告）。

- [ ] **Step 1: 全量清点**——`grep -rn "internal/modules/" internal/modules/knowledge/retrieval/ --include="*.go" | grep -v "modules/knowledge"` 逐条列出 file→package 对；与 §5.6 种子表核对（多退少补）。
- [ ] **Step 2: 登记豁免**——按 §5.6 程序追加 check.go 精确条目 + ledger 行（owner=22-knowledge-retrieval、remove_at=ib2）；机械计数修正（105→105+N）在 evidence 登记 §8 基线变更（原因、清单、批准者=协调者裁定引用 Ruling 原文）。
- [ ] **Step 3: exc-0089/0090/0091 处置结论**——实测阻断证据：`internal/types/interfaces/retriever.go:105/:112/:119`（冻结契约 `RetrieveEngineService` 形参直接使用 `embedding.Embedder`，contracts.yaml `knowledge.retrieve-engine-service` stability=frozen，framework:26 签名变化须 ADR）；`keywords_vector_hybrid_indexer.go:106/:121` 的 `utils.ChunkSlice` 无法独立于该族删除（同文件同 import 面）。本节点**不改这两个 Pass A 文件、不删三行**，在报告按 conventions §5 上报裁定建议：「三行随 ib2 与 airesource 门面任务收口，或走 ADR 修订 RetrieveEngineService 签名后由 barrier 删除」，remove_at=ib2 维持不变。K0 §9 的「删除」义务按此执行到机制边界并留痕（禁止现场改判，conventions §7.3）。
- [ ] **Step 4: GREEN**——`make check-backend-architecture` 零诊断；`go build ./...` 退出 0。
- [ ] **Commit:** `chore(passb): register exact cross-module import exceptions surfaced by K2 retrieval moves (Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY)`

### Task K2.8 — 高风险差分、Integration Brief 与节点收口

**Files:** Create `docs/architecture/evidence/passb/b2-k-retrieval.md`、`docs/plans/passb/reports/b2-k-retrieval.md`（补全 K2.1 T0 表）、`docs/architecture/passb/briefs/b2-k-retrieval.md`。

- [ ] **Step 1: 差分执行**——按 §7.3 表逐面双跑（T0 基线输出 vs 搬迁后同用例），用例清单、双跑输出摘录、比对结论、命令与退出码入 evidence 差分章节；失败只修新实现。
- [ ] **Step 2: 节点 gates 逐条执行并摘录**——`go build ./...`、`go test ./internal/modules/knowledge/... -count=1`、`make check-backend-architecture`、`make verify-module-moves`（命令原文+退出码入报告）。
- [ ] **Step 3: 变更清单 vs owned_files**——`PASSB_BASE_SHA=$(git merge-base origin/main HEAD); git diff --stat "$PASSB_BASE_SHA"...HEAD; git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort`，与 §4 可写清单逐条核对，差集非空即失败（conventions §1.2）。
- [ ] **Step 4: Integration Brief**——内容：3 落位包布局与 import path；ib2 装配切换清单（container.go 直连点、routes_*.go 直连点、`newUnavailableSemanticModelGateway` 形参随 semantic_model.go 迁移的改法）；宿主 shim/compat 3 文件删除编排；跨 owner 宿主调用点改写清单（datasource 24 点、conversation session_knowledge_qa.go:459、agentcatalog agent_share.go、K3/K4 各点——ib2 执行）；推迟件 2 个的解除编排（§3.3 列）；semanticScopeGuard/auditActor/escapeLikeKeyword 三族 ib2 收口；exc-0089/90/91 处置建议（K2.7 Step 3）；计数基线变更记录指针。
- [ ] **Step 5: 报告**——含执行命令台账、owned_files 核对结论、推迟项（§3.3 两个）、未完成项如实列出（conventions §1.2）。
- [ ] **Commit:** `docs(passb): b2-k-retrieval differential evidence, integration brief and node report`

## 9. 集成与回滚边界

- **集成**：本节点产物经 review 后由集成工程师以 `merge: passb b2-k-retrieval` 一次一支合入（缺省序 K1→K2→K3→K4，framework:28）；IB2 只从 `docs/architecture/passb/briefs/b2-k-retrieval.md` 切换共享装配（framework:103）；router/container/task/sync_task/bootstrap 改写仅集成工程师按 Brief 串行执行；contracts.yaml knowledge 区状态回写仅 ib2（conventions §3）。
- **回滚**（spec §13）：K2.1 merge commit 回滚=revert 单 commit 无装配影响；K2.2–K2.6 为 M2/M3 提交，分支内逐 commit revert，已并入集成分支的 M2/M3 保留（新路径未接线不影响运行，M4 失败先回退切换 commit）；K2.7 为数据行 commit，revert 需同步还原计数登记（§8 基线流程）；无 schema 变更、无 migration，回滚不需要数据修复；compat/shim 删除（ib2）在任何残留 importer 未清零前禁止执行。
- **升级**：门禁不可能通过或需改 20 计划 §5/§6 冻结表时，按 conventions §5 上报（报告 + DAG notes），禁止现场改判所有权、扩大例外或复制实现。

## 10. 必须删除的 legacy / alias / 例外

| 对象 | 动作 | 时点 |
|---|---|---|
| `docs/architecture/moves/knowledge.yaml` 中 27 个已物理搬迁文件的 `legacy_files` 行 | 同搬迁 commit 删除（modulemove 机械强制） | K2.2–K2.6 |
| `ownership-matrix.yaml` 对应行 | 不由本节点删（barrier ib2 回写窗口；行级删除权已声明，物理迁移证据以 manifest+git 为准，Ruling LEGACY-ROW-OWNERSHIP） | ib2 |
| `semantic_model.go`、`handler/knowledgebase.go` 的 manifest/matrix 行 | **保留**（推迟件，行随物理迁移删除；20 计划 §11 同律） | ib2 |
| 3 个宿主 compat 文件 + manifest 中登记的 3 行 | 直连完成后删除（Brief 指令，barrier 执行） | ib2 |
| 新登记 importExceptions 精确豁免 + ledger 行（K2.7） | 保留至对应跨模块消费经门面/端口合法化；remove_at=ib2，barrier 复核 | ib2 |
| exc-0089/0090/0091 | 冻结契约阻断（K2.7 Step 3 证据），本节点不删；裁定建议已上报 | ib2（建议） |
| conversation→knowledge/searchutil 9 对例外（exc-0070/0074-0078/0083-0085） | **无删除动作**（owner=35-conversation-program，remove_at=ib3；K2 保持 `internal/modules/knowledge/searchutil` 包路径与 API 稳定即可，20 计划 §7.6 同裁定；DAG produced_artifacts 表述以本行消歧） | ib3（35 节点） |
| 别名 18 条（`internal/searchutil` 等） | 不在本节点 29 文件范围，无动作（K5/ib2 收口） | K5/ib2 |

## 11. 独立验收标准

1. DAG `b2-k-retrieval` gates 四项全绿，命令原文+退出码记录于 `docs/plans/passb/reports/b2-k-retrieval.md`；
2. `git diff --name-only "$PASSB_BASE_SHA"...HEAD | sort` 与 §4 可写清单差集为空；27+2 中的 27 个文件物理落位三个子包，逐文件 `git diff --summary --find-renames` R100、函数体 diff 为零（package/import/改名除外）；
3. manifest `legacy_files`：27 行已删、3 行 compat 在册、2 行推迟件保留；`make verify-module-moves` OK；
4. §5.1 导出义务 12 符号全部在落位包以导出名可解析，宿主 compat 一行委托逐一在册（报告核对表）；
5. §7.3 四个差分面双跑逐用例一致，证据入 `docs/architecture/evidence/passb/b2-k-retrieval.md`；
6. 计数基线：633/23+23/58/537 三方一致不因本节点变化；importExceptions 增量 N 条与 ledger 行一一对应、remove_at=ib2、基线变更登记在案；
7. 推迟件 2 个：原位零改动（`git diff` 为空）、根因证据与解除编排写入 Brief 与报告；
8. 禁改清单（§4）零触碰：`git diff --name-only` 不含 router/container/bootstrap/migration/go.mod/他 owner 文件；
9. 计划评审 approved（reviewer 独立产出 `docs/plans/passb/reviews/b2-k-retrieval.md`）。

## 12. 计划自检记录（撰写时执行）

1. **spec 覆盖**：§5.2/§12（交付包）、§13（提交隔离 M2/M3、回滚）、§14.1-14.3（测试梯度、双跑差分）、§15（例外精确路径）、§17.2（单写者/例外清零路径）逐条映射到 §5-§11 对应节。✅
2. **文件存在性**：29 生产文件 + §7.1 全部测试文件在本 worktree base 树 `ls` 实测存在（2026-09-24 会话执行）。✅
3. **签名一致性**：§5.1/§5.2 全部签名与源码逐字核对（kb_activity.go、knowledgebase_access.go、kbshare.go、slug_fuzzy.go、semantic_model*.go、repository/{knowledge,tag}.go、semantic_model_budget.go、tenant_member.go 直读）；与 20 计划 §6.2 冻结表零漂移。✅
4. **跨任务接口一致**：K2.3 guard/seam 先于 K2.4/K2.5 嵌入方搬迁；K2.2 repo 落位先于 K2.3 的 `app/repository` import 重线；宿主 compat 分四批追加与各任务 GREEN 门禁闭环；推迟件不产生任何跨任务接口义务。✅
5. **无占位符/TBD**：2 个推迟件、1 组例外处置均为「证据 + 双分支裁定路径」完整给出，非悬空待定。✅
6. **虚警消歧记录**：§5.2 注（slug_fuzzy 注释引用）、agent_share.go:224 `callerCanManageShare` 注释引用、knowledgebase.go service 结构体字段全为冻结接口（knowledgebase.go:33-55 直读）——D2 扫描三类假阳性已排除，方法级双向扫描（§13 脚本口径）各仅 1 处实锤并已裁定。✅
