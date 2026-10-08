# Integration Brief — b2-k-process（K4）

> 交付对象：Integration Barrier ib2（集成工程师唯一切换事实源，framework:103）。本 Brief 按 24 计划 K4.5 Step 4 (a)–(i) 结构书写，**全部条目以本分支终态实测为准**（节点经 Ruling 2026-09-25-DEFERRED-FILE-SPLIT 收缩，实际迁移集 = service 3 生产 + handler 2 生产 + 随迁测试 4；repository 4 生产为模块侧副本、宿主原件保留；详见 §F 推迟件登记与 `docs/architecture/evidence/passb/b2-k-process.md`）。
> 节点分支：`codex/passb-b2-k-process`（含 P2 基线对齐 K1+K2+K3 内容，集成分支按缺省序合并时已合入者跳过）。

## (a) 装配切换表（container.go / routes_knowledge.go，集成工程师独占面）

本批已迁移文件**不产生即时切换**：container/router 引用的 K4 面符号全部仍由宿主解析（service 3 件为包级 helper 非容器构造器；handler 2 件经宿主 compat 委托）。切换表按目标时点分两窗：

| 装配点（当前树实测行号） | 现状解析 | ib2/补迁窗切换目标 |
|---|---|---|
| `internal/container/container.go:204` `Provide(repository.NewKnowledgeRepository)` | 宿主原件（K4.1 收缩：模块副本在场、原件保留） | 补迁轮收口（删宿主原件+写 repository compat）后 ib2 直连 `processrepository.NewKnowledgeRepository` |
| `container.go:205` `Provide(repository.NewKnowledgeSpanRepository)` | 同上 | 同上 → `processrepository.NewKnowledgeSpanRepository` |
| `container.go:417` `Provide(service.NewKnowledgePostProcessService, dig.Name("knowledgePostProcess"))` | 宿主（推迟件 #14 knowledge_post_process.go 留守） | §5.3 蓝本第 5 点补迁后 ib2 切 `process.NewKnowledgePostProcessService` |
| `container.go:418` `Provide(service.NewKnowledgeAutoTagService, dig.Name("knowledgeAutoTag"))` | 宿主（推迟件 #13） | 同上 → `process.NewKnowledgeAutoTagService` |
| `container.go:646` `Provide(service.NewHousekeepingService)` + `:647 Invoke(startHousekeepingService)`（func at `:2395`，形参 `*service.HousekeepingService`） | 宿主（span_tracker/housekeeping 对推迟，K4.2 §5） | 补迁后经宿主 type 别名窗口过渡 → ib2 直连 `process.NewHousekeepingService`（窄端口 `KnowledgeHousekeeping` 归 42-system-policy 消费方定义，spec §4.2；**不得出现第二套清扫实现**） |
| `container.go:723` `Provide(handler.NewKnowledgeHandler)` + `internal/router/routes_knowledge.go:71 RegisterKnowledgeRoutes(r *gin.RouterGroup, handler *handler.KnowledgeHandler, …)` | 宿主原生（KnowledgeHandler 随推迟件 #16 留守，零 compat 需求） | #16 补迁（前置=他属主白盒测试处置 + 7 同包符号导出，§5.3 第 6 点）后 ib2 切换 |
| container.go:91-98 既有 `modules/knowledge/*` import（docparser/retriever/wiki 别名族） | K1/K3 先例在册 | 本节点零新增（container 不在第一父链变更并集） |

计划原文行号 `container.go:202-204/:374/:641/:642/:718`、`routes_knowledge.go:67` 为撰写时点值；K3 插行与 K4.2/K4.3 删行后漂移如上表（K4.0 §6 勘误登记 412/413→417/418 同源）。

## (b) 18 worker 的 K4 面注册行核验（本批实现体迁移 0 个）

- 注册行 `internal/router/task.go:44-45`（`KnowledgePostProcess/KnowledgeAutoTag interfaces.TaskHandler` dig.Name 字段）、`:248`（`newDeadLetterKnowledgeFailer(params.KnowledgeService, params.SpanTracker)`）、`:270-320`（`mux.HandleFunc(types.TypeDocumentProcess/TypeManualProcess/TypeFAQImport/TypeQuestionGeneration/TypeSummaryGeneration/TypeKBClone/TypeKnowledgeMove…, params.KnowledgeService.ProcessX)`）；`internal/router/sync_task.go:130-131/:145-163` 同构 Lite 侧——**本会话实测引用形态，文件零改动**（二者不在第一父链变更并集，对 BASE `git diff` 为空）。
- K4 面 11 worker：9 个经 `params.KnowledgeService`（interfaces）分发——实现体随推迟批 #2-12 留守宿主，接口满足性=现状即满足；`TypeKnowledgePostProcess`/`TypeKnowledgeAutoTag` 经 `params.KnowledgePostProcess`/`KnowledgeAutoTag`（container.go:417/:418 直指宿主构造器）——两实现体（knowledge_post_process.go/knowledge_auto_tag.go）随推迟件留守，container 侧零改动。切换随 (a) 表 417/418 行同窗。
- 计数基线 23+23 不变（evidence §2.1）。

## (c) 推迟批解除窗口执行蓝本（指针 + 逐测试属主编排 + 六项收口授权原文）

**蓝本全文**：24 计划 §5.3（六项：① faq_delegate 收口 ② writableFAQKnowledgeBase 接收 ③ guard 第四轨 ④ K3 测试垫片删除 ⑤ service 独立面 3 件补迁 ⑥ handler 2 件补迁）+ §3.3 表 17 文件根因与解除条件。**授权原文引用**：

- K3 Brief (c) :78「K4 knowledge.go 落位 process 后，KnowledgeService 实现体的 FAQ 面归宿按 24 计划收口（接口不变，delegates 换指向）」（蓝本第 1 点）；
- K2 Brief §3.3 :78「方法体须随 K4 knowledge.go 补迁或由 K4 属主接收」（writableFAQKnowledgeBase，蓝本第 2 点；**注意 :82 所称 K2 测试垫片 manifest「垫片行」经实测不存在——勘误在案，删除权在 barrier**）；
- K3 Brief (b) :57「K2/K4 域改写测试后垫片删除」（蓝本第 4 点）。

**逐测试属主编排**（§3.3 解除条件，K4.0 Step 4 分类清单为事实源）：`craft_knowledge_test.go`/`craft_knowledge_tool_test.go`→41-craft；`datasource_purge_test.go`→26-datasource/ib2；`knowledge_shared_access_test.go`/`knowledge_shared_storage_failure_test.go`/`semantic_scope_mutation_test.go`→K2 推迟件补迁窗；`knowledge_caller_scope_test.go`→K4 补迁随迁；`rbac_lookups_test.go`→identity；`shared_agent_access_test.go`→agentcatalog。

**环境阻断推迟件的解除通道（本节点新增事实）**：span_tracker/housekeeping 两对 + repository 11 测试的根因（Mimosa DDL 测试常量确定性误报全通道拦截）中，**3 个 DDL 测试文件已获用户裁定放行（2026-09-26 选项 1）**——批准通道=协调者经工作流 world.run 执行 git mv 纯重命名 + 引用批准的独立 commit（R100 由构造保证）+ R100 复验；其余 8 个 setupKnowledgeTestDB 依赖测试随 finalize_test 补迁同窗逐字复现（K4.1 §4.1 已验证逐字可复现）。

## (d) 本批导出符号的跨 owner 宿主调用点 ib2 改写清单（P2 对齐后树净留守面）

| 符号（落位包导出名） | 宿主留守调用点（实测） | ib2 动作 |
|---|---|---|
| `process/repository.EscapeLikeKeyword`、`LikeEscapeChar` | identity `internal/application/repository/tenant.go:85/:90`；conversation `repository/message.go:200`（注释）+:215、`repository/session.go:209`（净留守 3 文件 4 实点；`tag.go:110/:121` 已随 K2 迁至 `retrieval/app/repository/` 消费同包 seam 副本 escape_like_seam.go） | **现状由宿主原件解析（K4.1 收缩：repository 原件保留、compat 未写）**——补迁轮收口时落 §5.2 第一清单 compat（escapeLikeKeyword 委托 + likeEscapeChar const 别名）保护；ib2 与 K2 Brief §6 :117 的 escape seam 单一实现收口**合并处置**为 `process/repository.EscapeLikeKeyword` 直连改写 4 点 |

（write-family/indexContent/task-options/kb_access 5 helper/RequireTaskProgressTenant 的导出消费面全部在宿主 service/handler 同包或 K1 过渡物——无 identity/conversation 级跨包调用点，见 (e) importer 表。）

## (e) 宿主 compat 删除编排（残留 importer 清零前置；Ruling TRANSITION-SHIM-ROW-REGISTRATION 成对行随文件同 commit 删）

**现状 2 个 compat 文件（计划原案 3 个，repository 侧顺延）**。逐符号残留 importer 实测：

- `internal/application/service/kbprocess_passb_compat.go`（8 委托）：
  - `writeResourceIDs` → knowledge_delete.go、knowledge_transfer.go、knowledge_reparse_scope.go、knowledge_delete_plan.go、**span_trace_seam_adapter.go（K1）**
  - `writeExecutionTenant` → knowledge_delete_plan.go、span_trace_seam_adapter.go
  - `knowledgeWriteKB` → knowledge_clone_move.go、knowledge_delete.go、knowledge_delete_plan.go、knowledge_process.go、span_trace_seam_adapter.go
  - `loadKnowledgeWrite` → knowledge_create.go、knowledge_replace.go、knowledge.go、knowledge_process.go、span_trace_seam_adapter.go
  - `loadKnowledgeWriteBatch` → knowledge.go、span_trace_seam_adapter.go
  - `buildKnowledgeIndexContent` → knowledge_process.go、span_trace_seam_adapter.go、**knowledge_index_content_test.go（留守测试，差分锚点）**
  - `documentProcessTaskOptions` → knowledge_create.go、knowledge_clone_move.go、knowledge_process.go
  - `knowledgePostProcessTaskOptions` → knowledge_process.go、**chunk_ingest_shim.go（K1）**
- `internal/handler/kbprocess_passb_compat.go`（5 委托）：全部 5 符号 → knowledge.go、knowledgebase.go（K2 推迟件）；`resolveHandlerKBAccess` 另→ **kb_access_test.go（留守测试）**；`resolveHandlerKBAccessFor` 另→ knowledge_download.go；`requireTaskProgressTenant` 另→ **faq_k3_compat.go（K3，零改动义务物）**。

**删除前置**：上述 importer 全部改直连落位包导出名（推迟件 #2-12/#16/#17 与 K2 推迟件补迁同窗；K1 adapter/shim 见 (g)；faq_k3_compat.go 随 (c) 蓝本第 6 点 parseCommaSeparatedTagIDs 导出同窗；kb_access_test.go 随 #16 补迁窗）。**repository 侧 compat（§5.2 第一清单：哨兵 var + 两构造器 var + KnowledgeSpanRepository 接口别名 + escapeLikeKeyword 委托 + likeEscapeChar 别名）随补迁轮「删宿主原件」同窗落地，ib2 按同表收口**。任何残留 importer 未清零前禁止删 compat（framework B5 口径前移）。

## (f) 推迟件登记与 K2 推迟件 3-10 的 K4 侧条件现状

- **计划 §3.3 表 17 文件**（repository/knowledgebase.go + service 核心批 11 + service 独立面 3 + handler 2）行保留、属主不变（plan=24）、delete_barrier=ib2；根因与解除条件以 §3.3/§5.3 为准，K4.0 Step 5 逐条重验在册。
- **环境阻断新增推迟件**（K4.1/K4.2 报告 §4/§5）：① repository 4 生产宿主原件删除 + manifest/matrix 行删除 + repository compat 落地（模块副本已在）；② repository 11 测试（3 DDL 阻断——**已获用户裁定放行，待协调者 world.run 通道执行**；8 个 setupKnowledgeTestDB 闭包依赖——随 finalize_test 补迁同窗）；③ span_tracker/housekeeping 两对（DDL 阻断 + runSweep 未导出方法/fitSpanName 15 行超 seam 上限的不可垫片论证）；④ K4 测试装置垫片 kbprocess_test_support_shim_test.go **未创建**（两成因消费者文件随推迟留守宿主、原定义原生可达，垫片前提不成立——顺延至任一成因文件补迁时重评）；⑤ K2 垫片消费者**未清零**（knowledge_tag_test.go:219 推迟后仍消费 kbretrieval_passb_compat_test.go 的 knowledgeTagRepository——K4.1 Step 4 变体如实改记；ib2 删除建议以补迁为前置）。
- **K2 推迟件 3-10 的 K4 侧条件现状**：K2 Brief 对 knowledge_delete.go 族的「K4 迁移后收口」前置**未满足**（#2-12 整批推迟），writableFAQKnowledgeBase 接收/D1 收口/guard 第四轨随 §5.3 蓝本第 1-3 点在补迁窗口兑现——ib2 时集成工程师按蓝本核对前置而非预期已迁。

## (g) K1 `span_trace_seam_adapter.go`/R1-9 shim 的 ib2 切换目标

- **已就绪**：write-family 4 函数（`WriteResourceIDs/WriteExecutionTenant/LoadKnowledgeWrite/LoadKnowledgeWriteBatch`）、`KnowledgeWriteKB`+`KnowledgeBaseWriteLookup`、`BuildKnowledgeIndexContent`、`DocumentProcessTaskOptions`/`KnowledgePostProcessTaskOptions` 已在 `internal/knowledge/process` 导出（K4.2）。ib2 把 `span_trace_seam_adapter.go:103-108`（hostKnowledgeWriteGuard 四方法）、`:126-127`（BuildKnowledgeIndexContentProvider）、`:142-148`（KnowledgeWriteKBProvider）与 `chunk_ingest_shim.go:62` 等提供器/函数值改直连 process 导出名（现经宿主 compat 委托解析，(e) 表所列 K1 importer）。
- **随推迟批**：`getFileType`/`normalizeFileExtension`/`isDataTableFileType`（R1-9 shim 函数值消费，knowledge_util.go 留守——链路原样编译零动作）、`enqueueSummaryRefresh`（K1 adapter 消费）、`attemptSuperseded`（同）、`ResolveProcessConfig`（chunk_ingest_shim 传宿主函数值，knowledge_process_config.go 留守——K4.1 §4.3「比原方案更简」口径）。`NoopSpanTracker` 导出随 span_tracker 对推迟顺延（K4.2 §5）。
- K1 shim 链路本节点零触碰（span_trace_seam_adapter.go/chunk_ingest_shim.go 不在变更并集，`go build ./...` 0 反证）。

## (h) 测试锚点残差与垫片台账

- **service 留守锚点**：knowledge_index_content_test.go（经 compat 消费 buildKnowledgeIndexContent 委托，②面锚点）、knowledge_write_access_test.go、knowledge_quota_guard_test.go（构造 `&knowledgeService{}`，随 #2-12）；auto_tag/post_process 族/process_config/move_wiki（勘误：随推迟批补迁窗——20 计划 §9 原判随 K4 不可行，白盒 svc.moveOneKnowledge/svc.repo 耦合，K4.0 Step 5 上报）；document_write_access_test.go（同因）；7 他属主白盒测试（(c) 编排表）。
- **handler 留守测试族**：kb_access_test.go/knowledge_transfer_test.go（兼测 KnowledgeBaseHandler，与 K2 推迟件 knowledgebase.go 补迁同窗拆分）/tag_ids/api_key_scope/download/folder/move_gate/mutation_admission/ownership/preview_security/spans（被测面 #16/#17 推迟）；rbac_lookups_test.go（identity）/list_pagination_test.go（platform）/shared_agent_access_test.go（agentcatalog）/document_write_scope_test.go。
- **housekeeping 差分锚点**（§8.3 第一面随迁双跑）随 `knowledge_housekeeping_test.go` 成对推迟——补迁窗双跑义务（evidence §1.2 ①）。
- **K2 垫片**（kbretrieval_passb_compat_test.go）：消费者未清零（(f) ⑤），ib2 随 K2 三 compat 同窗删除的前置=knowledge_tag_test.go 补迁。
- **K4 垫片**：未创建（(f) ④）；若补迁窗两成因文件（knowledge_post_process_trace_test/knowledge_replace_test）仍分属两窗，届时按 Ruling 2026-09-24-TEST-SUPPORT-SHIM 就地登记（「与 K3 垫片同包并存按孤儿成因逐件读解」的原拟 DAG notes 追认随垫片实际创建一并报）。
- **K3 垫片**（knowledge_faq_k3_test_support_shim_test.go）：消费者全在场（4 方法→knowledge_write_access_test.go:200/:208/:362/:364、2 纯函数→kb_activity_test.go:68/:79），本节点零触碰，删除随 (c) 蓝本第 4 点。

## (i) §8 计数奇偶记录

见 `docs/architecture/evidence/passb/b2-k-process.md` §2：633/23+23/58/537 全程零变化；matrix 388（knowledge 76 == ownership_test 断言）；pass-a-acceptance §6 两行（391→389→388）；exception-ledger **131**（P2 重编号终值 0129 + K4.4 exc-0130/0131，映射表与登记依据在 evidence §2.3）；manifests 发现与 matrix 的 23 行差为 BASE 既有 K2/K3 漂移（ib2 回写批，passbguard 诊断条目级 IDENTICAL 实证）。**本节点无路由/worker/hook/migration 变化。**
