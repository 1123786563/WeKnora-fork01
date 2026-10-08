# Integration Brief — b2-k-retrieval（K2 检索域）

> 提交方：work 节点 `b2-k-retrieval`（plan `docs/plans/passb/22-knowledge-retrieval.md`）。
> 消费方：ib2 集成屏障（集成工程师按本 Brief 串行执行共享装配切换；framework:103「IB 只从已评审 Brief 切换共享装配」）。
> 事实源：节点分支 `codex/passb-b2-k-retrieval`（K2.8 收口 HEAD）；差分/治理证据 `docs/architecture/evidence/passb/b2-k-retrieval.md`；节点报告 `docs/plans/passb/reports/b2-k-retrieval.md`。
> 写权限边界：本 Brief 只登记装配变更申请；`internal/router/**`、`internal/container/container.go`、`internal/bootstrap/**`、`contracts.yaml`、`ownership-matrix.yaml` 的实际写入由集成工程师/barrier 独占（conventions §3）。

## 1. 落位包布局与 import path（已交付，K2.2–K2.6 物理迁移完成）

| 落位包（import path） | 包名 | 内容（15 生产 + 11 测试 + 3 seam 文件） |
|---|---|---|
| `github.com/Tencent/WeKnora/internal/knowledge/retrieval/app/repository` | `repository` | semantic_model_invocation / semantic_model_policy / semantic_outbox / semantic_scope_epoch / tag（5 生产 + 5 测试）+ `escape_like_seam.go`（R2 seam） |
| `github.com/Tencent/WeKnora/internal/knowledge/retrieval/app` | `app` | semantic_model_capability / semantic_model_policy / semantic_scope / knowledgebase_access / slug_fuzzy / graph / kb_activity（7 生产 + 3 测试：capability/policy/slug_fuzzy）+ `semantic_scope_guard.go`（§5.4 双轨）+ `audit_actor_seam.go`（§5.2 R2 seam） |
| `github.com/Tencent/WeKnora/internal/knowledge/retrieval/app/handler` | `handler` | semantic_internal / semantic_model_policy / tag（3 生产 + 3 测试） |

- `git diff --summary --find-renames` 全 26 文件 rename 在案（72%–100%；<100% 均为 package/声明改名/R1 导出改名所致，函数体逻辑零改动，各任务报告逐块 diff 核验）。
- 本节点**未触碰** `internal/knowledge/module.go`（K5/ib2 装配，conventions §3）；`module.go` 门面如需暴露本域符号，导入路径以本表为准。

## 2. 已交付的导出端口（ib2 直连改写的目标符号）

落位包 `app`（K2 R1 导出义务，宿主 compat 一行委托在册）：

| 导出符号 | 原宿主名 | 留守消费方（ib2 改写点，见 §4） |
|---|---|---|
| `app.KBReadPermissions` | kbReadPermissions | K4 knowledge.go:830；K2 内 search_shared:45/83；conversation session_knowledge_qa.go:459 |
| `app.ResolveKBReadTenant` | resolveKBReadTenant | K3 knowledge_faq.go:36；K2 内 tag.go:85（推迟件） |
| `app.RequireKBWrite` | requireKBWrite | K4 knowledge.go / knowledge_delete_plan.go / knowledge_write.go |
| `app.WithKBWriteTenantInfo` | withKBWriteTenantInfo | K4 knowledge_create.go / knowledge_delete_plan.go |
| `app.ResolveDeadSlug` | resolveDeadSlug | K3 wiki_ingest.go:1670 / wiki_page.go:1191 |
| `app.RecordKBActivity` | recordKBActivity | datasource_service.go 17 处；K3 knowledge_faq*.go 7 处；K4 5 文件；K2 内 kbshare/tag/knowledgebase（推迟件） |
| `app.KBActivityTrigger` | kbActivityTrigger | datasource_service.go:837；K3/K4 |
| `app.WithKBActivityTask` | withKBActivityTask | datasource_service.go:837/:1405；K3/K4 |
| `app.KBActivityAppendSampleTitles` | kbActivityAppendSampleTitles | K3 knowledge_faq.go:1308；K4 knowledge_delete.go |
| `app.WithKBActivitySuppressed` | withKBActivitySuppressed | datasource_service.go:1642/:1790/:2106（全仓仅 3 处） |
| `app.RecordWikiContentActivity`（已导出，宿主 var 别名） | 同名 | K3 wiki_ingest_batch.go:719；internal/handler/wiki_page.go:401 |
| `app.AuditScopeKnowledgeBase`（表外机械增项） | auditScopeKnowledgeBase | 宿主 kb_activity_test.go:130（特征化锚定） |
| `SemanticScopeService` 字段 `Knowledge`/`Shares`（R1 字段导出，K2.3 §4.4） | knowledge/shares | 宿主 semantic_scope_test.go:61/:84 白盒 |

落位包 `app/repository`：`NewKBShareRepository` 剟——**kbshare.go 推迟未迁**，本包现导出 `NewKnowledgeTagRepository`、`NewSemanticControlRepository`、`NewSemanticModelPolicyRepository`、`NewSemanticModelInvocationStore`、`SemanticControlRepository`、`SemanticModelInvocationStore`（宿主 compat var/type 别名在册，§3）。

**尚未交付的导出义务（随推迟件顺延）**：`app.ApplyTenantRoleCap`（kbshare.go 推迟；消费方 agent_share.go:297/:356/:419）；`ErrInvalidTenantID`（定义于推迟件 knowledgebase.go:28，宿主原生可解析，ib2 补迁后随包导出）。

## 3. ib2 装配切换清单（集成工程师独占文件的直连点）

### 3.1 `internal/container/container.go`（本节点零改动；以下引用现经宿主 compat 别名解析）

| 行（现态实读） | 现引用（compat 别名） | ib2 直连目标 |
|---|---|---|
| :192 `repository.NewSemanticControlRepository` | repository compat var | `kbrepo "…/retrieval/app/repository"` 同名构造器 |
| :193 `repository.NewSemanticModelPolicyRepository` | 同上 | 同上 |
| :194 `repository.NewSemanticModelInvocationStore` | 同上 | 同上 |
| :195 `*repository.SemanticControlRepository`（SemanticScopeInvalidator 适配） | repository compat type | `*kbrepo.SemanticControlRepository` |
| :196 `service.NewSemanticScopeService` | service compat var | `kbapp "…/retrieval/app"` 同名构造器 |
| :197 `*service.SemanticScopeService` + `handler.NewSemanticInternalHandler` | service/handler compat | `*kbapp.SemanticScopeService` + `kbhandler.NewSemanticInternalHandler` |
| :206 `repository.NewKnowledgeTagRepository` | repository compat var | `kbrepo.NewKnowledgeTagRepository` |
| :303 `repository.NewKBShareRepository` | repository compat var（**kbshare.go 推迟件仍在宿主原生定义**，本行现为宿主直连） | kbshare.go 补迁后切 `kbrepo.NewKBShareRepository` |
| :341 `service.NewKBShareService` | 宿主原生（kbshare.go 推迟） | 补迁后直连 |
| :344-:391 `SetSemanticScopeInvalidator` 装饰器 ×6（TenantMember/Organization/KB/User/Share/Transfer 面） | 断言经宿主 compat 的 `semanticScopeGuard` 同形定义 | 直连后：嵌入方（tenant.go:57/organization.go:47/tenant_member.go:78/user.go:100 等 identity 推迟件）仍需宿主 guard 或 identity 导出版本——见 §6 三族收口，**非机械替换** |
| :712 `service.NewUnavailableSemanticModelPolicyService` | service compat var | `kbapp.NewUnavailableSemanticModelPolicyService` |
| :716 + :1059-:1069 `newUnavailableSemanticModelGateway`（形参 `*service.SemanticScopeService`/`*service.SemanticModelPolicyService`/`*repository.SemanticModelInvocationStore`；体内 `service.NewSemanticModelCapabilityIssuer`/`service.NewSemanticModelGateway`/`service.NewSemanticModelBudgetAdapter`） | 前三者经 compat；后两者为推迟件 semantic_model.go / 他属主 semantic_model_budget.go 宿主原生 | **随 §5.1 semantic_model.go 补迁同窗切换**：gateway 形参改指 `*kbapp.SemanticScopeService` 等落位类型（或届时商业侧端口，见 §5.1），`NewSemanticModelBudgetAdapter` 按 12-commercial 落位端口改写 |

### 3.2 `internal/router/routes_*.go`（本节点零改动）

| 行（现态实读） | 现引用 | ib2 直连目标 |
|---|---|---|
| routes_knowledge.go:254 `RegisterSemanticModelPolicyRoutes(…, policyHandler *handler.SemanticModelPolicyHandler, …)` | handler compat type | `*kbhandler.SemanticModelPolicyHandler` |
| routes_knowledge.go:279 `RegisterKnowledgeTagRoutes(…, tagHandler *handler.TagHandler, …)` | handler compat type | `*kbhandler.TagHandler` |
| routes_infra.go:14 `RegisterSemanticInternalRoutes(…, h *handler.SemanticInternalHandler)` | handler compat type | `*kbhandler.SemanticInternalHandler` |

（routes_knowledge.go:266 `RegisterKnowledgeBaseActivityRoutes` 形参为 `*handler.AuditLogHandler`，不属本节点文件，无切换。）
router 侧测试引用两处亦经别名解析：`internal/router/router_api_key_capabilities_test.go:301`（`&handler.TagHandler{}`）、`internal/router/semantic_internal_test.go:18`——ib2 直连时同步改写或随 compat 删除一并处理。

### 3.3 宿主 shim/compat 3 文件 + 1 测试垫片删除编排（barrier 写权限）

| 文件 | 内容 | 删除前置条件 |
|---|---|---|
| `internal/application/repository/kbretrieval_passb_compat.go` | var/type 别名（§3.1 前四行引用面） | §3.1 container 直连完成 |
| `internal/application/service/kbretrieval_passb_compat.go` | §2 表 12 符号一行委托 + semantic 面 type/var/哨兵别名 + `semanticScopeGuard` 同形定义（§5.4 双轨）+ `writableFAQKnowledgeBase` 方法再归置（接收者 `*knowledgeService` 留宿主） | §3.1 直连 + §4 跨 owner 调用点改写完成 + guard 三族收口（§6）；`writableFAQKnowledgeBase` 方法体须随 K4 knowledge.go 补迁或由 K4 属主接收 |
| `internal/handler/kbretrieval_passb_compat.go` | 3 type + 3 var 别名 | §3.2 直连完成 |
| `internal/application/repository/kbretrieval_passb_compat_test.go` | Ruling 2026-09-24-TEST-SUPPORT-SHIM 垫片（服务 K4 属主 knowledge_tag_test.go:219/:241） | knowledge_tag_test.go 随 K4 迁移或直连改写 |

删除动作=同 commit 删 4 文件 + manifest `docs/architecture/moves/knowledge.yaml` 对应 4 行（compat 3 行 Ruling TRANSITION-SHIM-ROW-REGISTRATION 在册 + 垫片行）；evidence §5 台账同步销账。**任何残留 importer 未清零前禁止删除**（plan §9）。

## 4. 跨 owner 宿主调用点改写清单（ib2 执行；本节点未改这些文件）

| 消费方（属主） | 文件:行（现态实读） | 现解析 | ib2 改写 |
|---|---|---|---|
| datasource（26-datasource） | `internal/application/service/datasource_service.go`：activity 族 22 处（recordKBActivity 17 + :837 trigger/task + :1642/:1790/:2106 suppressed） | service compat 委托 | `kbapp.RecordKBActivity` 等导出名直连（context key 类型随实现整体在 app 包，无跨包失配） |
| datasource（26） | `datasource_purge_test.go` | 同上 | 同上（测试同步改写） |
| conversation（35） | `internal/application/service/session_knowledge_qa.go:459`（**路径勘误**：plan §5.1 写 handler/session，实际在 application/service 会话文件） | service compat `kbReadPermissions` | `kbapp.KBReadPermissions` |
| agentcatalog（25） | `internal/application/service/agent_share.go:297/:356/:419` | 宿主原生 `applyTenantRoleCap`（kbshare.go 推迟件定义） | kbshare.go 补迁导出 `ApplyTenantRoleCap` 后直连 |
| K3（23-knowledge-wikifaq） | knowledge_faq.go:36（resolveKBReadTenant）、knowledge_faq.go:1308（appendSampleTitles）、knowledge_faq_import.go:245/:2249（trigger/task）、wiki_ingest.go:1670 + wiki_page.go:1191（resolveDeadSlug）、wiki_ingest_batch.go:719（RecordWikiContentActivity）、knowledge_faq*.go recordKBActivity 7 处 | compat 委托 / 宿主 var 别名 | 导出名直连（kb_activity_test.go 中 2 个 FAQ 面用例随 K3 knowledge_faq_import.go 迁移时拆分随迁，K2.5 报告 §5） |
| K4（24-knowledge-process） | knowledge.go:830（kbReadPermissions）、knowledge_create.go / knowledge_delete_plan.go / knowledge_write.go（requireKBWrite/withKBWriteTenantInfo）、knowledge_create/clone_move/delete/process/replace.go（activity 族） | compat 委托 | 导出名直连 |
| identity（b1 推迟件） | organization.go:47 / tenant.go:57 / tenant_member.go:78 / user.go:100 的 `semanticScopeGuard` 内嵌 | 宿主 compat 同形定义 | §6 guard 三族收口后切 identity/共享实现 |

## 5. 推迟件清单与解除编排（Ruling 2026-09-25-DEFERRED-FILE-SPLIT；manifest/matrix 行保留未删）

**合计 14 个**（plan §3.3 原 2 + K2.4 裁定收缩 8 + K2.5 收缩 3 + 计划空位 1）。逐文件根因与解除条件以 K2.4 报告 §3、K2.5 报告 §5 为事实源；本 Brief 载编排：

| # | 文件 | 解除编排（ib2 窗口） |
|---|---|---|
| 1 | `internal/application/service/semantic_model.go`（+`semantic_model_test.go`） | plan §3.3 原编排：ib2 把本文件与 `semantic_model_budget.go`（12-commercial）同批切割——commercial 侧落位 `internal/modules/commercial/service` 并导出窄端口（budget ops 接口 + `SemanticBudgetFailure`），gateway 形参改端口，集成工程师改 container.go:1059-1069 一处；随后按 K2.4 同法迁移（含 manifest 行删除） |
| 2 | `internal/handler/knowledgebase.go`（+7 测试 + rbac_lookups.go 联动） | plan §3.3 原编排：ib2 先对 rbac_lookups.go 两方法去方法化（`KBCreatorLookup`/`KBCreatorLookupFromKbIDParam` :34/:49 改包级函数，router 形参同步），再按 K2.6 同法迁移 |
| 3-10 | `service/knowledgebase.go`、`knowledgebase_search.go`、`_fanout`、`_faq`、`_fusion`、`_results`、`_shared`、`_storegroup`（**勘误：K2.4 报告 §3 表列 7 漏计 `_shared`，实际 8 文件**；`git ls-files` 实证） | 解除条件：K4 迁移 knowledge_delete.go（消除 collectImageURLs/knowledgeResourceOwners/deleteExtractedImages 白盒耦合）+ kb_activity/kbshare 同包家族已就位（前者已完成）+ 他属主宿主白盒测试（craft×2、resource_review、datasource_delete_sqlite、semantic_scope_mutation 等）随各属主处置；随后按 K2.4 同法补迁（含 5 对跨模块 import 豁免按 Ruling 登记：knowledgebase.go:15/:17/:18、kbshare.go:10、knowledgebase_search.go:9） |
| 11 | `service/kbshare.go` | 解除条件：identity 导出 `ErrOrgNotFound`/`ErrTenantNotInOrg`/`ErrInvalidRole` 哨兵端口（或 ib2 裁定直连改写）；届时导出 `ApplyTenantRoleCap`（§2 顺延义务） |
| 12-13 | `service/tag.go`、`service/tag_access.go` | 解除条件：knowledge_write_access_test.go 按被测对象拆分（tag 面 vs FAQ 面，涉 K3/K4 用例归属）；随后同族同迁 |
| 14 | `internal/application/repository/kbshare.go` | **计划空位**（K2.5 报告 §7.3：§3.1 列 repository 6 文件但任务节未点名）——请协调者在 ib2 前裁定归属任务（建议并入 kbshare.go 补迁同窗） |

推迟件 396/B5 口径不变（Ruling 6 第 4 点）：仍计入未迁集，B5 清零时必须有去向。

## 6. 三族 seam 的 ib2 收口（多属主重复 helper 族，conventions §7.1）

| 族 | 现状（双轨/多份） | ib2 收口 |
|---|---|---|
| `semanticScopeGuard` | 落位 `app/semantic_scope_guard.go`（§5.4.1）+ 宿主 compat 同形定义（§5.4.2）+ identity 包同形 seam（b1）三份 | 与 identity 导出版本统一为单一实现（identity 服务不入 knowledge 模块；guard 语义属 identity/共享层，建议 identity 侧导出、两侧消费） |
| `auditActor`/`auditActorRole` | 落位 `app/audit_actor_seam.go`（体=对 `types.UserIDFromContext`/`types.TenantRoleFromContext` 的同一委托）+ tenant_member.go 原定义（identity） | K5/ib2 接 identity 导出的 `AuditActor`/`AuditActorRole` 后删 seam（K0 冻结组 E 裁定） |
| `escapeLikeKeyword` | 落位 `app/repository/escape_like_seam.go`（逐字对齐 repository/knowledge.go:24-28） | K4 迁移 knowledge.go（24-knowledge-process 属主，b0-evidence.md:86）导出窄端口后删 seam |

## 7. 例外与计数基线（指针）

- **importExceptions 增量 5 条**（exc-0106..0110，105→110）：两侧登记/计数/回收编排见 evidence §1/§2/§3.2；`remove_at: ib2`——ib2 经门面/端口合法化（§5 推迟件补迁 + airesource 门面）或 ADR 修订后删除，计数回落 110→105。
- **exc-0089/0090/0091 处置建议**（K2.7 Step 3，evidence §3.3）：两分支择一——① 随 ib2 与 airesource 门面任务（embedding/utils 经模块根公共门面导出）收口后删除；② ADR 修订 `RetrieveEngineService` 签名（去 `embedding.Embedder` 形参依赖）后删除。本节点未改未删。
- **计数基线 633/23+23/58 三方一致**不因本节点变化（K2.8 gates 实测：`literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16`）。
- **PassBTask 口径偏差待确认**（K2.3 起登记，evidence §1）：计划 §5.6 模板 `K2.7` 与 passbguard `PassBTaskModule` 映射（仅模块级 id）不兼容，5 条豁免沿用 `B-knowledge`——请 barrier 复核追认或修订计划模板。

## 8. 治理文件回写请求（barrier 窗口）

1. `contracts.yaml`（本节点禁改）：`knowledge.tag-service` 的 characterization_tests/consumers 路径 `internal/handler/tag_delete_test.go` → `internal/knowledge/retrieval/app/handler/tag_delete_test.go`（passbguard `contract-characterization-missing` 中间态根源，K2.6 报告 §4.2）；knowledge 区其余 status/consumers 面随 ib2 统一回写。
2. `ownership-matrix.yaml`：本节点 29 行中 15 行已物理迁移（行删除按 Ruling LEGACY-ROW-OWNERSHIP 归 barrier 回写窗口；物理迁移证据=manifest+git），14 推迟件行保留。
3. `internal/knowledge/module.go` 门面注释：归 K5/ib2（本节点零触碰）。
4. 遗留裁定请求：`app/graph.go` 构造器 `NewGraphBuilder` 全仓零消费（K2.4 grep 实证）——若属死装配面，请在 K5 门面设计时裁定（导出或删除）。
