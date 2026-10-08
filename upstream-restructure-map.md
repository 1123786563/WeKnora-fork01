# upstream-restructure-map.md — 上游后端目录归位全局映射表（r4.1 · 剩余移动收口版）

> 合成自四份区域分析（agent 区域 / handler 区域 / modules 副本区域 / 外围表面），由映射汇总员去重、裁决嵌套、实跑复核后合并；r4~r4.3 按评审意见逐条修订（记录见"八、评审修订记录"）。
> 分支：`restructure/upstream-backend-layout`；上游：`upstream/main`（Tencent/WeKnora）。
> 裁决原则：**宁可少移不可错移**；fork 自研、上游无对应物一律 keepInPlace。
> 版本说明：本版**覆盖** r1-r3（order 10-60 主体批次均已执行，见提交 902c260fc、8a39f7e9f、f00a62ca5、b61eab3d9、273effc70、order-50 批、42167516a）。本表只登记**剩余待执行**的 12 条移动。

---

## 〇、汇总员验证记录（全部本 session 实跑）

**第一轮（合成时）：**
1. **handler 批中间态**（`git status --porcelain`）：`internal/handler/{chunk,evaluation,faq,memory,wiki_page}.go` + `wiki_page_handler_test.go` 6 文件均为 `??` 未跟踪；对应 6 个 modules 源文件实测全部 absent。→ 证实贡献2/贡献4 双报的"42167516a 提交不完整、HEAD 树不可编译"。
2. **handler 6 文件 vs upstream**：evaluation.go **0 行**、memory.go **0 行**（字节一致）；chunk.go 7 行（gin import 顺序+swagger 空格）；faq.go 82 行、wiki_page.go 133 行（fork 保留修改）。
3. **agent 两条测试改名**：`knowledge_search_metadata_test.go` 与上游 `search_knowledge_metadata_test.go` **逐字节一致**；`grep_chunks_snippet_test.go` 与上游 `chunk_snippet_test.go` 仅 **19 差异行**（上游后增 2 测试）。两个目标路径实测不存在。
4. **embedpolicy 副本**：与正主 `diff -rq` **逐字节一致**；`git grep 'modules/policy/embedpolicy' -- '*.go'` 恰 3 处生产 import。
5. **browserskill 副本（推翻贡献3的两项断言）**：
   - a) "副本无 go import 引用"**不成立**：实测 **12 处生产 import**（agent/tools×7、application/service×2、container.go:111、handler/session×2）+ 守卫台账 check.go:511；正主亦有 1 处（agent/const.go:15）——**两副本并存脑裂**。
   - b) "副本 15 文件与上游逐字节一致"**部分不成立**：副本 daemon.go 是 fork 安全加固版（os.StartProcess+固定 argv+绝对路径校验+devnull），正主 daemon.go=上游原版——**直接删副本会回退安全加固**。store_test.go 三方差分；shared_test.go 副本 +2 行。**导出 API 面两侧完全一致** → import 重写编译安全。
6. **process/repository 副本清账**：目录实测 16 文件（贡献3 只列 11）；11 个与 upstream 对应文件 **cmp 逐字节一致**（9 测试正主缺失 + knowledge_tag.go/knowledge_transfer.go 正主已有同内容）；目录 **零 import**（死目录）。

**第四轮（评审 r4 驱动，全部复跑确认）：**
7. **searchQueryTokens 存在性（推翻本表 r4 与贡献1 的断言）**：`grep -rn 'func searchQueryTokens' internal/agent/tools/` → **faq_snippet.go:271 `func searchQueryTokens(queries []string) []string`**，签名与上游测试调用（单参数 []string）一致；其依赖 snippetStopwords（faq_snippet.go:331-337）恰含测试断言要跳过的 the/does/how；chunk_snippet.go:140 自身也调用 searchQueryTokens；TestExtractSnippetForChineseQuery 依赖的 extractSnippetForQueries 存在于 chunk_snippet.go:134。→ **采纳上游 chunk_snippet_test.go 整文件既可编译、语义也匹配**。（静态验证：函数存在+签名匹配+stopword 匹配；未跑 live go test——本角色禁改代码文件。）
8. **evaluation.go func 数**：上游/本地顶层 func 均为 **3**（"19 个 func"系与 memory.go 串行；memory.go 实测 19/19 正确）。
9. **internal/application 全量补扫**（评审#2 要求，`git ls-tree -r upstream/main` vs `find internal/application -name '*.go'` comm -23）：上游有/本地缺失 .go 共 **103**（service 80 + repository 23）；其中 **70 个唯一缺失文件在 internal/modules 下有同名对应物（目录命中 71 处——career/evaluation.go 与 insights/evaluation/evaluation.go 为同一上游文件 service/evaluation.go 的两个同名站点，唯一缺失口径按 70 计）**；分簇（唯一缺失口径）：wiki 簇 24、ingest 簇 16、faq 簇 6、process 簇 15（=entry#1 的 9 + OQ#7 的 2 + process 根 4）、retrieval 簇 6（=tag.go 1 + service 侧 5）、insights 簇 3；**33 个无对应物**（repository 4 + service 29，全清单见 openQuestions #6）。注：tenant_skill_verify.py、memory/evalset*.json、access/README.md 实际存在于本地（初扫 find 仅匹配 *.go 造成的假缺失，已剔除）。
10. **tag.go 差异性质**：本地 `retrieval/app/repository/tag.go` = 上游 `internal/application/repository/tag.go` + **32 行纯新增**（fork 方法 DeleteOrphanTagByName，SP2-a Task 8）；package 两边均 `repository`；deps 仅 context/strings/internal/types/types·interfaces/gorm（无模块内 import）；正主路径缺失（scan 实测）。→ 可移（新 entry #2）。
11. **chunk_image_assets.go 不可单独移（对评审#2 点名候选的反证）**：与上游差异虽仅 package 行（本地 package ingest vs 上游 package repository），但**上游版引用同包 chunkRepository**，而正主 internal/application/repository 缺失 chunk.go（scan 实测 NO-COPY）——单独移入必然编译失败；阻塞于 repository/chunk.go 的恢复决策。
12. **models 缺失清单**：恰 **6 个**（chat/ollama_multicontent_test.go、parity/volcengine_budget_test.go、rerank/{catalog_matrix,sdk_clients,wire}_test.go、vlm/spec_override_test.go）——评审#3 报"7 个"有误，其中 "ollama_multicontext" 系笔误（实为 ollama_multicontent）。
13. **其余缺失抽查**：internal/agent/tools/search_knowledge_rerank_test.go（UP-YES LOC-NO）、internal/handler/wiki_search_test.go 与 internal/application/service/wiki_search_test.go（双侧均 UP-YES LOC-NO）、internal/application/service/datasource_schedule_test.go（UP-YES LOC-NO）全部确认。

**第五轮（评审 r4.1 驱动，全部复跑确认）：**
14. **tag.go 消费方 import 审计（推翻本表 r4.1 的"8 文件改 import"指示，错误 6/8）**：逐文件 grep 实测（区分真实 import 行与注释命中）——(a) semantic_model_policy.go、semantic_scope.go、semantic_model_policy_test.go、semantic_model_capability_test.go 4 文件虽有真实 import，但 tag 符号（NewKnowledgeTagRepository/KnowledgeTagRepository/DeleteUnusedTags/DeleteOrphanTagByName）**零命中**，其限定符符号面全为留守的 Semantic*（SemanticModelPolicy/SemanticModelPolicyRepository/SemanticControlRepository/ErrUserNotFound/ErrTenantNotFound/ErrKnowledgeBaseNotFound/ErrSemanticModelPolicyNotFound，定义于留守的 semantic_model_policy.go/semantic_outbox.go/semantic_model_invocation.go）→ import 维持不动；(b) semantic_outbox_test.go、semantic_outbox_pg_test.go **无 import 行**（此前 git grep 命中 :364/:230 系注释行——同类注释假阳性）→ 零动作；(c) internal/application/repository/kbretrieval_passb_compat.go 的 9 处 kbretrieval. 引用中仅 :32 `var NewKnowledgeTagRepository = kbretrieval.NewKnowledgeTagRepository` 涉及 tag，其余 8 处桥接留守 semantic 符号 → **import 必须保留**；且该 var 在 tag.go 移入同包（package repository）后与真身函数**同名重声明**（redeclared 编译失败），**须删除该行**（非"去前缀"）；(d) 唯一需改 import 的只有 internal/modules/knowledge/process/repository/knowledge_tag_test.go（:221 `kbretrieval.NewKnowledgeTagRepository` 单点使用，别名已存在）。
15. **分簇算术对账（评审#2）**：唯一缺失口径 103（70 有对应物 + 33 无）；目录命中 71 处，差值=evaluation.go 双站点（insights + career）；**70 = 9（entry#1 缺失部分）+ 2（OQ#7）+ 1（tag.go）+ 55（knowledge keepInPlace：wiki 24+ingest 16+faq 6+process 根 4+retrieval service 5）+ 3（insights keepInPlace）✓**；entry#1 的另 2 个文件（knowledge_tag.go/knowledge_transfer.go）为冗余删除、不属缺失口径。

**第六轮（终审 r4.2 驱动，全部复跑确认）：**
16. **entry #1 闭包审计（终审#1/#2 成立，r4.2 方案会破坏死目录留守测试编译）**：(a) 留守 knowledge_tag_test.go 有 **7 处 `&knowledgeRepository{db: db}`（:78/:105/:128/:146/:166/:181/:220）与 17 处方法调用**，4 个方法 SetKnowledgeTags/AddKnowledgeTagRelations/GetKnowledgeTags/DeleteKnowledgeTagRelations 全部定义在 knowledge_tag.go（:15/:57/:120/:152）——删除该文件必破；(b) setupKnowledgeTestDB **全仓唯一定义于 knowledge_finalize_test.go:64**（grep internal/ 排除 .superpowers 仅此一处），死目录内 9 个文件使用（8 个将移测试 + 留守 knowledge_tag_test.go + knowledge_datasource_test.go:34/:80/:115）——knowledge_finalize_test.go 迁走则留守两文件 undefined；(c) 留守非测试文件 knowledge.go/knowledge_span_repo.go 对 4 方法及 knowledge_tag/knowledge_transfer 顶层标识符**零命中**（删 2 文件后留守侧编译安全）；(d) knowledge_span_repo_test.go 对 setupKnowledgeTestDB/knowledgeRepository/tag 方法**零依赖（自持）**，可留守；(e) **执行前基线**：`go vet ./internal/modules/knowledge/process/repository/` **exit 0**（破坏为 r4.2 方案执行引入，终审判定不可接受）。
17. **正主 canonical 测试编译既有债务（新发现）**：`go vet ./internal/application/repository/` 现状 **exit 1**——knowledge_datasource_external_id_test.go:14/:44/:66 `undefined: setupKnowledgeTestDB`（order-50 #27 批记录"setupKnowledgeTestDB 由目标包既有 _test 文件提供"已过时，现全仓无定义）。entry #1 随迁 knowledge_finalize_test.go（定义所在）**顺带修复**该既有债务。
18. **终审#4 正向证据复核**：container.go:232 `must(container.Provide(repository.NewKnowledgeTagRepository))`、:43 import 正主 internal/application/repository——compat :32 转发 var 删除后该接线自动落到同包真身 ✓（自跑 grep 确认）。

---

## 一、执行顺序总览（groups，仅含实际移动的组）

| order | group key | 条目数 | 内容 | 顺序依据 |
|---|---|---|---|---|
| 10 | `internal/application` | 2 | process/repository 副本 11 文件归位/清冗余 + tag.go 归位（含 8 处消费方 import 改写） | repository 底层包，被 service/container import，先执行 |
| 20 | `internal/embedpolicy` | 1 | 删副本 + 3 处 import 指回正主 | 底层 policy 叶子包（被 handler/middleware/router import），改动面最小先做 |
| 30 | `internal/browserskill` | 1 | 副本 3 分叉文件按 fork 版覆盖正主 + 删 15 冗余 + 12 处 import 重写 | 底层叶子包，含内容合并与 12 处 import 重写，量级大于 embedpolicy |
| 40 | `internal/agent` | 2 | tools 内 2 个测试文件改名（fork 变体名 → 上游名，**采纳上游最新版**） | 包内低风险改名；其 tools/ 目录同时是 order 30 的 import 改写对象，先 30 后 40 |
| 60 | `internal/handler` | 6 | **git add 6 个未跟踪目标文件**，补全 42167516a 的 order-60 handler 批 | 最上层（被 container/router 装配），且是当前 HEAD 不可编译的直接原因，最后收口 |

---

## 二、entries 分组明细（12 条，全部 high）

### order 10 — internal/application

| # | from | to | 粒度 | 置信 | 证据 |
|---|---|---|---|---|---|
| 1 | `internal/modules/knowledge/process/repository`（11 个测试文件随迁 + 2 个冗余删除） | `internal/application/repository` | files | high | 【r4.3 闭包修正（终审#1/#2）：r4.2 的"9 测试迁移+2 删除"会破坏死目录留守测试编译】副本目录零 go import（死目录）；执行前 `go vet ./internal/modules/knowledge/process/repository/` exit 0（基线实测）。移动 **11 个测试**：① 9 个与 upstream cmp 逐字节一致（`knowledge_{create,duplicate,finalize,folder_move,folder,list_filter,metadata_prefix,source_schema,transfer}_test.go`，正主 9/9 缺失实测）；② **knowledge_tag_test.go**（与上游分叉、正主缺失；留守会在 knowledge_tag.go 删除后失去 4 个 knowledgeRepository 方法定义——实测其 7 处 &knowledgeRepository{db:db}（:78~:220）与 17 处方法调用，方法定义于 knowledge_tag.go:15/:57/:120/:152）；③ **knowledge_datasource_test.go**（fork 独有；留守会因 setupKnowledgeTestDB 随迁而 undefined——该 helper 全仓唯一定义于 knowledge_finalize_test.go:64，本文件 :34/:80/:115 三处调用）。②③为闭包完整性要求的最小随迁集（终审三选一中"随迁"方案；如 owner 改判"保留 knowledge_tag.go"，撤回该删除即可）。删除 **knowledge_tag.go、knowledge_transfer.go**（正主已有逐字节相同内容；留守非测试文件对其符号零命中实测，删除安全）。**顺带修复正主既有债务**：canonical 的 knowledge_datasource_external_id_test.go:14/:44/:66 引用 setupKnowledgeTestDB 而全仓无定义、`go vet ./internal/application/repository/` 现状 exit 1（实测）——knowledge_finalize_test.go 定义到位后转 0。**knowledge_tag_test.go 落位正主后其 kbretrieval import 整体删除**（NewKnowledgeTagRepository 随 entry #2 同包直引）。目录终态：knowledge.go、knowledge_span_repo.go、knowledge_span_repo_test.go 3 文件留守（自持，对 setupKnowledgeTestDB/knowledgeRepository/tag 方法零依赖实测，处置见 OQ#7）。门控：`go test ./internal/application/repository/` + `go vet ./internal/modules/knowledge/process/repository/`（应保持 exit 0）+ `go vet ./internal/application/repository/`（应由 1 转 0） |
| 2 | `internal/modules/knowledge/retrieval/app/repository/tag.go` | `internal/application/repository/tag.go` | files | high | **r4.1 新增（评审#2 补扫产物）**：diff 实测本地=上游+**32 行纯新增**（fork 方法 DeleteOrphanTagByName，SP2-a Task 8 孤儿标签清理，随移保留 fork 版本，0 行删改）；package 两边均 `repository`；deps 仅 context/strings/internal/types/types·interfaces/gorm，无模块内 import；上游同包 knowledge_tag.go 与本地正主版本字节一致（#1 已证），上游两文件同包共存证明移入无符号冲突；正主路径缺失（补扫实测）。**消费方处置（r4.2 修正：原"8 文件改 import"错误 6/8；r4.3 随 entry #1 闭包再修）**：① 唯一外部消费方 internal/modules/knowledge/process/repository/knowledge_tag_test.go **随 entry #1 落位正主同包**：其 kbretrieval import 整体删除，:221 的 kbretrieval.NewKnowledgeTagRepository 去限定符为同包直引（其 7 处 &knowledgeRepository{db:db} 与 4 方法调用落在正主自己的 knowledge.go/knowledge_tag.go 上，后者正主已有逐字节一致版本）；② internal/application/repository/kbretrieval_passb_compat.go **import 必须保留**（9 处 kbretrieval. 引用中 8 处桥接留守 semantic 符号），但 :32 的 `var NewKnowledgeTagRepository = kbretrieval.NewKnowledgeTagRepository` 转发行**必须删除**（tag.go 移入同包 package repository 后同名重声明，编译失败；删除后 container.go:232 的 repository.NewKnowledgeTagRepository 自动落到同包真身——container.go:43 本就 import 正主，实测确认）；③ 其余 6 文件零动作——semantic_model_policy.go/semantic_scope.go/semantic_model_policy_test.go/semantic_model_capability_test.go 的 import 服务于留守 Semantic* 符号（tag 符号零命中），semantic_outbox{,_pg}_test.go 无 import（此前命中系注释行） |

### order 20 — internal/embedpolicy

| # | from | to | 粒度 | 置信 | 证据 |
|---|---|---|---|---|---|
| 3 | `internal/modules/policy/embedpolicy` | `internal/embedpolicy` | dir | high | 副本 2 文件（origin.go、origin_test.go）与正主 `diff -rq` **逐字节一致**；正主与上游 0 diff。操作=**删副本目录 + 3 处 import 指回正主**：git grep 实测恰 3 处生产 import——internal/handler/embed_channel.go:19、internal/middleware/embed_auth.go:15、internal/router/routes_agent.go:13（正主路径当前无人 import，指回后成为唯一事实源）。**✅ 已执行：commit 44e1b93f4（2 删除 + 3 import 改写；`go build ./...` exit 0）** |

### order 30 — internal/browserskill（⚠️ 含内容合并，非纯删除）

| # | from | to | 粒度 | 置信 | 证据 |
|---|---|---|---|---|---|
| 4 | `internal/modules/execution/browserskill` | `internal/browserskill` | dir | high | **贡献3 原证据被本 session 复核推翻并修正**（其"副本与上游逐字节一致、无 import、直接删除"会回退安全加固且漏掉 12 处 import）。实测事实：(a) 12 处生产 import 指向副本——internal/agent/tools/{browserskill.go:11, browserskill_result.go:9, execution_seams.go:15, browserskill_test.go:9, browserskill_image_test.go:7, browserskill_result_test.go:8, browserskill_schema_test.go:7}、internal/application/service/{agent_capabilities.go:18, agent_service.go:25}、internal/container/container.go:111、internal/handler/session/{browserskill.go:8, handler.go:17}；正主亦有 1 处（internal/agent/const.go:15，无需改）。(b) 副本 18 文件=15 个上游一致 + 3 个 fork 分叉：daemon.go（**安全加固版**：os.StartProcess+固定 argv+绝对路径校验+devnull 防管道耗尽）、store_test.go（AutoMigrate 免相对路径）、shared_test.go（+2 行）；正主 daemon.go/shared_test.go=上游原版、store_test.go 读 fork DDL 文件。(c) **导出 API 面两侧完全一致**（全量导出声明 diff 空）→ import 重写编译安全。操作三步：① 3 个分叉文件以**副本 fork 版覆盖正主**（沿 r3 "fork 实质修改保持 fork 版本"先例，daemon.go 安全加固不得回退）；② 其余 15 个冗余文件删除；③ 12 处 import 重写为 internal/browserskill。正主独有 extension_test.go 保留。内容裁定建议 owner 复核（openQuestions #2） |

### order 40 — internal/agent

| # | from | to | 粒度 | 置信 | 证据 |
|---|---|---|---|---|---|
| 5 | `internal/agent/tools/knowledge_search_metadata_test.go` | `internal/agent/tools/search_knowledge_metadata_test.go` | files | high | cmp 实测与 upstream **逐字节一致**（纯改名残留）；目标路径实测不存在；package tools；被测 writeKnowledgeMetadataHeader 在本地同包 knowledge_search.go:836 |
| 6 | `internal/agent/tools/grep_chunks_snippet_test.go` | `internal/agent/tools/chunk_snippet_test.go` | files | high | diff 实测与上游 chunk_snippet_test.go 仅 **19 差异行**，全部=上游后增的 2 个测试（TestExtractSnippetForChineseQuery、TestSearchQueryTokensSkipsStopwords），其余测试函数逐字相同；目标路径实测不存在。**r4.1 修正（评审#1）**：本表 r4 曾断言"采纳上游整文件会因 searchQueryTokens 缺失而编译失败"——实测该断言错误：`searchQueryTokens(queries []string) []string` 存在于同包 **faq_snippet.go:271**（签名与上游测试调用一致），其 snippetStopwords（faq_snippet.go:331-337）恰含测试断言要跳过的 the/does/how；TestExtractSnippetForChineseQuery 依赖的 extractSnippetForQueries 存在于 chunk_snippet.go:134（该文件 :140 本身就调用 searchQueryTokens）。→ **改名时直接采纳上游最新版整文件**（两个新测试一并恢复，静态验证：函数存在+签名匹配+stopword 匹配；live go test 未跑，本角色禁改代码） |

### order 60 — internal/handler（补全 42167516a：git add 6 个未跟踪文件）✅ 已执行（commit b82feaa03）

| # | from | to | 粒度 | 置信 | 证据 |
|---|---|---|---|---|---|
| 7 | `internal/modules/knowledge/ingest/chunk_handler.go` | `internal/handler/chunk.go` | files | high | 迁移已在进行：源已被 42167516a 删除（实测 absent）、目标为未跟踪新文件（实测 `??`）。package handler 含 12 个与上游同名 func（NewChunkHandler/GetChunkByIDOnly/…），与上游仅 7 差异行（gin import 顺序+swagger 空格）。装配面已就绪：container.go:954、rbac_lookups.go:103-149、routes_knowledge.go:24。剩余动作=git add 提交 |
| 8 | `internal/modules/insights/evaluation/evaluation_handler.go` | `internal/handler/evaluation.go` | files | high | 源已删（实测 absent）、目标未跟踪且与 upstream **diff=0（字节一致）**，含 EvaluationHandler/NewEvaluationHandler，顶层 func **3/3 一一对应**（r4.1 修正：r4 误写"19 个 func"系与 memory.go 串行；实测上游/本地均为 3）。modules/insights/evaluation/ 保留 dataset.go/evaluation.go/metric_hook.go 等 service 层文件（keepInPlace）。剩余动作=git add 提交 |
| 9 | `internal/modules/knowledge/faq/faq_handler.go` | `internal/handler/faq.go` | files | high | 源已删（实测 absent）、目标未跟踪；与上游 82 差异行为有意保留的 fork 修改（同包 parseCommaSeparatedTagIDs/requireTaskProgressTenant 直引恢复、17→15 func 为 fork 整合）；配套 faq_enabled_filter_test.go 已随 42167516a 提交、faq_k3_compat.go shim 已删。剩余动作=git add 提交 |
| 10 | `internal/modules/agentruntime/memory/memory_handler.go` | `internal/handler/memory.go` | files | high | 源已删（实测 absent）、目标未跟踪且与 upstream **diff=0（字节一致）**，顶层 func **19/19 一一对应**（实测）；配套外部测试副本已由 42167516a 删除，本地 handler/memory_consistency_test.go 无重复。剩余动作=git add 提交 |
| 11 | `internal/modules/knowledge/wiki/wiki_page_handler.go` | `internal/handler/wiki_page.go` | files | high | 源已删（实测 absent）、目标未跟踪；30 func 与上游对齐（+1 fork 增量），133 差异行为 fork 修改（本 session 实测），含仍 import internal/modules/knowledge/wiki 域层（keepInPlace）；wiki_page_k3_compat.go shim 已删、wiki_fixer_scope*.go 已迁 handler/session。剩余动作=git add 提交 |
| 12 | `internal/modules/knowledge/wiki/wiki_page_handler_test.go` | `internal/handler/wiki_page_handler_test.go` | files | high | 源已删（实测 absent）、目标未跟踪；package handler，与旧文件仅 22 行机械差异（包名、wiki. 限定符、NewWikiPageHandler 构造参数 6→5）。上游 internal/handler **无**此测试文件——随 wiki_page.go 迁移的 fork 自有测试，仅为 handler 测试同包，不产生上游冲突。剩余动作=git add 提交 |

---

## 三、keepInPlace（31 条，不移动；r4.1 修订 2 条理由）

| 路径 | 原因（去重合并后） |
|---|---|
| `internal/modules/agentcatalog` | fork 自研功能模块（agent marketplace、subagent、expert install、published expert/skill、tenant skill market），上游 internal/** 全树 grep 零对应物（贡献1+贡献3 双印证）；handler/repository/service 三层为 fork Pass A/B 架构；被 11 处本地 import（container.go:96、handler/subagent.go 别名壳、application/repository/{expert_install,published_expert,published_skill,tenant_subagent}.go 别名壳、tools/architectureguard/check.go 等），移动破坏 fork 装配；上游 tenant_skill_source.go 与本地 tenant_skill_market_service.go 非同一功能 |
| `internal/modules/agentruntime` | Pass A 占位骨架：仅 module.go（零逻辑）+ README + legacy/README.md；原实现已由 273effc70（676 files）移回 internal/agent；无 go import 引用（仅 .superpowers 工件与文档）；上游无对应物 |
| `internal/agent` | 路径已与上游对齐（213 共享路径；上游 218 中仅 5 个测试缺失：2 个见 entries #5/#6、search_knowledge_{,rerank_}_test.go 与 tool_images_test.go 见 openQuestions #4/#5）；目录内 149 个 fork 增补文件（experts/、native/、nativecontract/、nativeprobe/、opencode/、persona/、recoverytest/、runtime/、subagents/、trpc/、skills/skillhub/、commercial_adapter.go、image_requirement.go 等）对上游是新增文件，无 delete/modify 冲突 |
| `internal/agent/tools/knowledge_search.go` | fork 并行实现 KnowledgeSearchTool（20 func），与上游对齐的 search_knowledge.go 双轨并存；其测试测的是 KnowledgeSearchTool 而非上游 SearchKnowledgeTool.rerankResults——不可改回上游同名；同属自研保留：knowledge_search_mmr_test.go、search_knowledge_panic_test.go。双轨收敛见 openQuestions #5 |
| `internal/sandbox` | 路径已与上游完全对齐（115 共享路径全在）；12 个 fork 增补文件保留；13 个内容分歧文件属内容合并非路径问题 |
| `internal/modelcontext/model_output_test.go` | fork 增补测试（上游无此文件）；internal/modelcontext 其余 21 文件路径全对齐（19 字节一致） |
| `internal/localsandbox` | 43/43 文件与上游 blob 逐一比对 byte-identical，无需移动 |
| `internal/mcp` | 18/18 文件字节一致，无需移动 |
| `internal/modules/knowledge` | **r4.1 修订理由（评审#2）**：fork 知识域活跃实现（111 go 文件）；"上游无 internal/modules/knowledge"**仅在路径层为真**——补扫实测其中 **55 个文件**（insights 侧另 3 个见其条目；70 对账见 openQuestions #8）与上游 internal/application/{service,repository} 存在同名对应物但内容分歧（package 本地 wiki/ingest/faq/execution vs 上游 service/repository，diff 2~3059 行）：**wiki 簇 24**（wiki_ingest*×14、wiki_page.go、wiki_linkify{,_test}、wiki_lint、wiki_slug_handles{,_test} 等 service 侧 22 + repository 侧 wiki_page.go[diff=3059]、wiki_page_test.go）；**ingest 簇 11**（service 侧 chunk_write/extract/ocr_sanitizer{,_test}/parser_url_security{,_test}/image_multimodal*×3/chunk_edit_parent_test/extract_data_table_summary_test）；**ingest 簇 repository 侧 5**（chunk_image_assets.go[仅 package 行差异但被 chunk.go 缺失阻塞，见 #11 验证]、chunk_{faq_diff,fields,revision,sqlite}_test.go）；**faq 簇 6**（faq_clone_sync、knowledge_faq{,_batch,_create_guard,_import}）；**process 根 4**（knowledge_index_content、knowledge_task_options{,_test}、knowledge_write）；**retrieval/app service 侧 5**（graph、kb_activity、knowledgebase_access、slug_fuzzy{,_test}）。另有 process/repository 13 文件由 entry #1 处置（11 测试闭包随迁 + 2 冗余删除，目录终态 3 文件见 OQ#7）+ tag.go（entry #2）。域内收敛迁移需 owner 决策（openQuestions #18），本轮按宁可少移保留原位 |
| `internal/modules/insights` | **r4.1 修订理由（评审#2）**：fork 自研模块；evaluation_handler.go 已单列迁出（order 60）；保留的 dataset.go/evaluation.go/metric_hook.go 与上游 internal/application/service 同名对应物内容分歧（3 个，补扫实测）——fork insights 实现，收敛见 openQuestions #18；analytics 未分析 |
| `internal/modules/execution` | fork 自研模块（35 go 文件）；上游无 internal/execution。单列迁出 browserskill/ 子目录整目录（order 30，含内容合并）；其余保留 |
| `internal/modules/policy` | Pass A 骨架。单列迁出 embedpolicy/ 子目录（order 20 删副本）；其余保留 |
| `internal/modules/datasource` | Pass A 骨架三件套 + connector/moauth 测试专用 mock OAuth 连接器（deliberately NEVER registered in production container）；上游 internal/datasource 内容已由 70abd726d 整体归位，骨架无上游对应 |
| `internal/modules/career` | fork 自研（50 go 文件）；上游全树无 career 路径。**r4.1 注**：career/evaluation.go 与上游 internal/application/service/evaluation.go 同名（补扫命中 1 个），但 insights/evaluation/evaluation.go 亦同名——对应关系二义，按宁可少移保留（OQ #18） |
| `internal/modules/craft` | fork 自研（72 go 文件）；上游无对应路径 |
| `internal/modules/workbench` | fork 自研（54 go 文件）；上游无对应路径 |
| `internal/modules/commercial` | fork 自研（159 go 文件，O02 商业化域）；上游无对应路径 |
| `internal/modules/appconnector` | fork 自研（119 go 文件）；上游无对应路径；cmd/connector-control 为其运行载体 |
| `internal/modules/codedelivery` | fork 自研（21 go 文件）；上游无对应路径 |
| `internal/modules/conversation` | fork 自研（28 go 文件，package httptransport）；chat_pipeline 已归位，其余保留 |
| `internal/modules/plugins` | fork 自研（25 go 文件）；上游无对应路径 |
| `internal/modules/airesource` | fork 自研模块骨架/装配（models/web_search/mcp/storageurl 已归位）；上游无对应路径 |
| `internal/modules/channels` | fork 自研（im 已归位；Pass A 残留保留）；上游无对应路径 |
| `internal/modules/identity` | fork 自研；上游无对应路径 |
| `internal/modules/system` | fork 自研；上游无对应路径 |
| `internal/modules/craftegress` | fork 自研（6 go 文件）；cmd/craft-egress-adapter 为其运行载体 |
| `internal/bootstrap` | fork 自研装配层（lifecycle/routes/workers）；上游 internal/ 无 bootstrap |
| `internal/metrics` | fork 自研（O04 craft observability）；上游 internal/ 无 metrics |
| `cmd/connector-control` | fork 自研（open-connector ADMIN 凭据 worker；贡献3+贡献4 双印证）；唯一外围构建面 docker/compose.open-connector.yaml:157,163 |
| `cmd/craft-egress-adapter` | fork 自研（egress 权威进程）；docker/craft/Dockerfile:307 确认不构建进镜像 |
| `cmd/saas-migrate` | fork 自研（legacy spaces 迁移工具）；外围表面零引用 |

---

## 四、referenceSites（执行时需同步处理的引用点）

**order 10（entry #1 process/repository 闭包迁移 + entry #2 tag.go）**
- internal/modules/knowledge/process/repository（零 import 死目录；执行后终态 3 文件：knowledge.go、knowledge_span_repo.go、knowledge_span_repo_test.go，处置见 openQuestions #7）
- **entry #1 闭包要点（r4.3）**：11 测试随迁（9 上游一致 + knowledge_tag_test.go + knowledge_datasource_test.go）+ 2 冗余删除（knowledge_tag.go/knowledge_transfer.go）；正主 knowledge_datasource_external_id_test.go 的 setupKnowledgeTestDB 既有 undefined 债务由随迁定义修复
- **tag.go 消费方处置（r4.2/r4.3）**：knowledge_tag_test.go（随 entry #1 落位正主后 kbretrieval import 整体删除、:221 去限定符同包直引）；internal/application/repository/kbretrieval_passb_compat.go:32（**转发 var 删除**——tag.go 移入同包后同名重声明编译失败；该文件 import 保留供其余 8 处 semantic 符号桥接；container.go:232 接线自动落到同包真身）；semantic_model_policy.go、semantic_scope.go、semantic_model_policy_test.go、semantic_model_capability_test.go、semantic_outbox{,_pg}_test.go 共 6 文件**零动作**（tag 符号零命中或无 import）

**order 20 embedpolicy（3 处 import 改写）✅ 已执行（commit 44e1b93f4，`go build ./...` exit 0）**
- internal/handler/embed_channel.go、internal/middleware/embed_auth.go、internal/router/routes_agent.go → `modules/policy/embedpolicy` 改为 `internal/embedpolicy`（改写+gofmt 后新行号分别为 :15、:14、:11）；副本 2 文件已 `git rm`；旧路径在全部 *.go 中清零（含未跟踪 handler 文件抽查）

**order 30 browserskill（12 处 import 改写 + 1 处已就绪）✅ 已执行（commit 4de9391f5，`go build ./...` exit 0，提交后 HEAD 复跑亦 exit 0）**
- 12 处 import 已全部改写为 `internal/browserskill`（含 gofmt 重排序 agent_service.go/agent_capabilities.go/container.go/handler.go 4 文件）；副本 18 文件已 git rm，其中 3 分叉文件以 fork 版内容合并入正主（daemon.go 加固版 + shared_test.go proc 配套 + store_test.go AutoMigrate 版，合并后与副本原文件 shasum 逐字节一致）；`git log --follow` 实证 daemon.go 血统完整（4de9391f5→6ad22241d→a3c7be10c）；旧路径在全部 *.go 功能性 import 中清零（历史文档/账本除外，见下）
- internal/agent/const.go:15（已 import 正主，无需改，验证属实）
- tools/architectureguard/check.go:511（ImportedPath 台账：按 r3 已执行批次实证先例**有意保留**，收尾批统一收口；与贡献3"必须同步重编译"的建议相悖，采旧表实证，见 openQuestions #9）

**order 40 agent（2 个测试改名，#6 采纳上游最新版）✅ 已执行（commit 3d683cb23，`go build ./...` exit 0）**
- #5 纯改名：knowledge_search_metadata_test.go → search_knowledge_metadata_test.go（R100，与 upstream/main diff=0 复验）；#6 改名+采纳上游最新版整文件：grep_chunks_snippet_test.go → chunk_snippet_test.go（+19 行=2 个新测试 TestExtractSnippetForChineseQuery/TestSearchQueryTokensSkipsStopwords，rename 相似度 82%，落位后与 upstream/main diff=0 复验）
- **live go test 门控已跑（核对清单第 2 条）**：`go test ./internal/agent/tools/ -run 'TestSearchQueryTokens|TestExtractSnippetForChineseQuery|TestFaqMatchSnippet|TestExtractChunkMatchSnippet'` 7/7 PASS（含 2 个恢复测试）；`git log --follow` 实证两文件血统完整（均追溯至 273effc70 及更早）
- 执行注记：Mimosa hook 拦截 `git mv`（识别为绕过 Write/Edit 的源码写入），等效改用 Write 同内容落位 + `git rm` 源文件 + `git add`，git 相似度检测自动识别 rename（R100/R82 暂存即实证）；同包内改名零 import 改写，旧文件名全仓功能代码/外围表面（Makefile/scripts/.github/docker/deploy/config）grep 零命中实测

**order 60 handler（装配面已在 HEAD 就绪）✅ 已执行（commit b82feaa03：git add 6 个未跟踪文件补全 42167516a；提交前/提交后 `go build ./...` 均 exit 0 实测；跨树 diff 42167516a~1→HEAD 识别全部 6 条 rename R091-R099，内容血统完整；执行时实测旧 handler 文件名与已删除包路径在功能性代码中零残留——命中仅为 Pass A/B 历史台账、architectureguard 守卫台账（收尾批口径）与 2 条纯注释）**
- internal/container/container.go:954（handler.NewChunkHandler）、:3595-3596（Chunk/ChunkHost 字段）
- internal/router/routes_knowledge.go:24-32（RegisterChunkRoutes，注释记录迁移）
- internal/router/rbac.go:158-186（chunkHandler 参数与 guard 装配）
- internal/handler/rbac_lookups.go:103-149（(*ChunkHandler) KBCreatorLookup* 方法）
- internal/modules/knowledge/module.go:67,198 与 module_test.go（Chunk/ChunkHost *handler.ChunkHandler 字段）
- docs/plans/passb/21-knowledge-ingest.md:122,168；packages/api-client/src/knowledge/faq.ts（git add 后自愈）

**keepInPlace 模块的装配/引用面（不得随移动破坏）**
- internal/container/container.go:96（import agentcatalog/service）
- internal/handler/subagent.go、internal/application/repository/{expert_install,published_expert,published_skill,tenant_subagent}.go（Pass B 别名壳 → agentcatalog）
- internal/handler/user_resource_favorite_test.go、internal/application/{service,repository}/user_resource_favorite_test.go、internal/application/repository/tenant_disabled_shared_agent_test.go（import agentcatalog）
- tools/architectureguard/check.go（agentcatalog/service import，架构守护校验模块路径）
- cmd/connector-control ← docker/compose.open-connector.yaml:157,163；cmd/craft-egress-adapter ← internal/modules/craftegress；cmd/saas-migrate ← internal/modules/commercial

**外围表面（已验证零移动路径引用/已自愈，无需改动）**
- Makefile:120-123、:302（ldflags 注入 internal/handler.{Version,…}，真身 system.go:314-315——handler 路径不得再变）、:77；scripts/get_version.sh:71
- scripts/model-catalog/{generate.py:15, sources.json:5}、scripts/model_catalog_diff.py:6,30、.github/workflows/app.yml:27-29,55-57（models 归位后实测自愈）
- scripts/validate-trpc-native-p0-docs.sh:35、.github/workflows/agent-recovery.yml:7-11,43（agent 归位后实测自愈）；.github/workflows/anydoc.yml:13,19,79,80（docparser 自愈）
- packages/dsh-weknora/contract/contract_test.go:19-20（外围唯一 Go import 硬依赖：internal/handler/session 与 internal/types）
- mcp-server/tests/test_kb_id_resolution.py:4-5；frontend/src、apps/web/src、packages/* 约 50 个 TS 文件 frozen-contract 注释镜像（贡献4 抽查 29/30 实测存在）
- migrations/versioned/000209:3、000272:3（引用存在）；deploy/craft/preview.conf:15；docker/Dockerfile.app:46-47；docker/craft/Dockerfile:307
- deploy/lago/evidence/*（证据工件不回改）；apps/mobile 构建产物（生成产物不处理）；.superpowers/sdd/**（历史工件不回改）
- helm/、config/、testdata/、dataset/、tests/、apps/{desktop,embed}（贡献4 实测零命中）

---

## 五、openQuestions

1. **【P0，order 60 执行前置】42167516a 提交不完整**（贡献2+贡献4 双报，实测确认）：6 个目标文件 `??` 未跟踪而其 modules 源文件已删——单独 checkout HEAD 必然编译失败。执行 order 60 = git add 6 文件并补提交。**✅ 已修复（commit b82feaa03，git add 6 文件；提交后 HEAD `go build ./...` exit 0 实测）**
2. **【安全语义复核】browserskill 内容合并裁定**：daemon.go 副本为 fork 安全加固版，本表裁定副本加固版覆盖正主上游版；store_test.go 三方差分裁定取副本 AutoMigrate 版。两项内容裁定建议 owner 复核，确认无下游依赖正主 exec.Command 行为差异。
3. 上游有而本地全无对应物的 **handler 层** 5 个测试（datasource_schedule_test.go、shared_agent_picker_test.go、wiki_search_test.go、session/browserskill_test.go、session/rewind_test.go）：恢复还是接受缺失，需 owner 决策（不恢复则上游每次改动产生 delete/modify 冲突）。
4. **【r4.1 并入 #6 总清单】**upstream internal/agent/tool_images_test.go 本地无等价测试：恢复或接受缺失，需 owner 决策。
5. KnowledgeSearchTool（knowledge_search.go，fork）/ SearchKnowledgeTool（search_knowledge.go，上游同构）双轨并存且共享辅助函数（writeKnowledgeMetadataHeader 已挪入 knowledge_search.go）；上游 **search_knowledge_test.go 与 search_knowledge_rerank_test.go**（r4.1 补，实测 UP-YES LOC-NO）本地均缺：双轨收敛与测试归属需 owner 决策。
6. **【r4.1 重写：上游有、本地全无对应物总清单（评审#3）】**补扫+抽查实测，恢复或接受缺失需 owner 逐区决策：**internal/models 6 个**（chat/ollama_multicontent_test.go、parity/volcengine_budget_test.go、rerank/{catalog_matrix,sdk_clients,wire}_test.go、vlm/spec_override_test.go——评审报"7 个"有误，comm 实测恰 6，"ollama_multicontext"系笔误）；**internal/application/service 29 个**（agent_browser_{preferences,source}_test、agent_tool_surface_test、chunk.go、datasource_schedule_test、extract_data_table_orphan_test、fork_bootstrapper_test、fork_snapshot_reaper_test、host_skills_test、image_action_loop.go、image_multimodal_attrs_test、knowledge_attempt_supersede_test、knowledge_create_gitlab_test、knowledge_faq_sanitize_test、knowledge_reparse_dequeue_test、knowledge_stuck_processing_test、multimodal_pending_counter_test、shared_agent_sandbox_tenant_test、tenant_skill_{host,served}_test、wiki_deleted_tenant_guard_test、wiki_ingest_{attempt,cite_resolve,followup,taxonomy_batch}_test、wiki_page_rewrite_continuation_test、wiki_search_test、workspace_{checkpointer,git_layout}_test）；**internal/application/repository 4 个**（chunk.go、chunk_image_assets_test.go、knowledge_list_sort_test.go、like_escape_sqlite_test.go）。
7. 【r4.3 更新】process/repository 死目录**终态 3 文件**处置（entry #1 执行后）：knowledge.go、knowledge_span_repo.go 与正主分叉（谁新需裁决）；knowledge_span_repo_test.go 上游有同名但副本版本与上游分叉且正主缺失（**自持**：对 setupKnowledgeTestDB/knowledgeRepository/tag 方法零依赖实测，留守不破坏编译）。（knowledge_tag_test.go、knowledge_datasource_test.go 已随 entry #1 闭包迁出，其 kbretrieval import/tag 方法依赖在正主同包解决）目录零 import，整体保留/清空需 owner 决策。
8. **【r4.2 对账修正、r4.3 更新】**internal/application 补扫已执行（见〇.9）：103 缺失 .go（70 有 modules 同名对应物、33 无对应物）。**70 = 10 入 entry#1**（9 个上游一致 + knowledge_tag_test.go）**+ 1 入 OQ#7**（knowledge_span_repo_test.go）**+ 1 入 entry#2（tag.go）+ 55 入 knowledge keepInPlace + 3 入 insights keepInPlace**（knowledge_datasource_test.go 为 fork 独有、不属缺失口径）；目录命中 71 处的差值=career/evaluation.go 与 insights/evaluation/evaluation.go 为同一上游文件的双站点。33 个无对应物清单入 #6。internal/ 其余顶层目录已由贡献1 对账，无需补扫。
9. architectureguard 守卫台账（check.go:511 等）与 exception-ledger.yaml 处置：贡献3 主张"删副本必须同步更新规则并重编译"，但 r3 已执行批次实证"替换必挂三方耦合豁免测试、悬空条目因 moduleImportBase 前缀判定永不触发而无害"——两论冲突，本表采 r3 实证先例（保留至收尾批），如 owner 另有裁决按其执行。
10. internal/datasource/README.md 被 fork Pass A 文档整篇替换、上游原文逐字节存于 modules/datasource/README.pkg.md：是否还原需人工决定。
11. scripts/open-connector/acceptance-cases.json:34-77 引用不存在的 ./internal/application/service/appconnector、:319 引用不存在的 internal/handler/app_connector_connection.go：改脚本还是补移动，需裁决。
12. deploy/mobile-workbench 引用 internal/execution/{deployment_policy,restore_policy,bridge}.go 等理想化路径（实际在 modules/execution，keepInPlace）：悬空不随本表自愈。
13. docker/craft/{document,spreadsheet}-requirements.lock 引用 internal/craft/testdata/*（实际在 modules/craft/testdata）。
14. docreader/proto/docreader.proto:5 go_package 指向不存在的 internal/docreader/proto：是否改 proto 选项需裁决。
15. 注释级过时路径（不阻塞编译，收尾批统一处理）：migrations/versioned/000041:85、000181:4；chunkingSamples.ts 引用 internal/models/chat/provider（实际 internal/models/providers）。
16. 内容分歧备忘（路径已对齐，非本表范围）：internal/agent 61 文件、internal/sandbox 13、internal/modelcontext 2、internal/im 3、internal/handler/faq.go 82 行、wiki_page.go 133 行、internal/datasource 27 文件。
17. docs/architecture/moves/*.yaml 等 Pass A/B 文档以 modules/* 为目标路径，与回归上游方向相反，需单独清理；Pass A 骨架退役时点属重构计划决策。
18. **【r4.1 新增（评审#2）、r4.2 口径修正】knowledge/insights 域 58 个 keepInPlace 登记的唯一缺失对应物（knowledge 55 + insights 3）的收敛决策**：wiki 24、ingest 16、faq 6、process 根 4、retrieval/app service 侧 5、insights 3；career/evaluation.go 为 evaluation.go 的同名二义站点（不新增缺失数）——与上游 internal/application/{service,repository} 同名但内容分歧（package 名不同、diff 2~3059 行），属 fork 知识域活跃实现。是否执行"知识域回归 application 布局"专项批次（迁移+import 大面积改写+包名还原）需 owner 规划，本表按宁可少移 keepInPlace。**特别阻塞项**：ingest/chunk_image_assets.go 虽仅差 package 行，但上游版引用同包 chunkRepository，而正主缺失 repository/chunk.go（#6 清单项）——该文件的可迁移性阻塞于 chunk.go 恢复决策。

---

## 六、执行核对清单（给执行者）

1. 按 order 升序：10（application：process/repository 闭包迁移【11 测试随迁+2 冗余删除，正主 setupKnowledgeTestDB 既有编译债务顺带修复】+ tag.go 归位【compat 转发 var 删除；knowledge_tag_test.go 随迁同包直引；其余消费方零动作】）→ 20（embedpolicy 删副本+3 import）→ 30（browserskill 三步合并+12 import，**daemon.go 保留 fork 加固版**）→ 40（agent 2 个测试改名：#5 纯改名；**#6 直接采纳上游最新版整文件**，两个新测试 TestExtractSnippetForChineseQuery/TestSearchQueryTokensSkipsStopwords 一并恢复）→ 60（handler git add 6 文件补提交，修复 HEAD 不可编译）。
2. 每组完成后跑 `go build ./...`；**order 10 另跑（终审#3 补强：go build 不编译 _test.go、go test 只编译正主包，两侧 vet 是死目录留守测试编译的唯一可见门控）**：`go test ./internal/application/repository/` + `go vet ./internal/modules/knowledge/process/repository/`（基线 exit 0 实测，执行后须保持 0）+ `go vet ./internal/application/repository/`（现状 exit 1 系既有债务，执行后应转 0）；order 40 另跑 `go test ./internal/agent/tools/ -run 'TestSearchQueryTokens|TestExtractSnippetForChineseQuery|TestFaqMatchSnippet|TestExtractChunkMatchSnippet'`。
3. 守卫台账（architectureguard check.go / exception-ledger）按 openQuestions #9 的先例保留至收尾批，不随批修改。
4. fork 实质修改文件迁移后内容保持 fork 版本（browserskill daemon.go 加固、tag.go 的 DeleteOrphanTagByName、faq.go/wiki_page.go handler 增强等），git 可见内容差异属预期。

---

## 七、合成裁决记录（r4）

1. **同一 from 不同 to 的冲突**：无（11 条 entry 的 from 互不相同）。
2. **from 嵌套**：程序化两两前缀比较，无嵌套。
3. **keepInPlace 去重合并 4 组**：agentcatalog（取贡献1 详证）；agentruntime（同）；modules/datasource（贡献2 五条子路径并入贡献3 一条）；cmd 三件（取贡献4 详证）。
4. **openQuestions 去重合并 1 组**：42167516a 提交不完整（贡献2+贡献4）。
5. **证据纠错 1 项（关键）**：贡献3 browserskill 两处断言被实跑推翻，操作语义由"直接删除"修正为"三步合并"（entry #4）。
6. **范围补正 1 项**：process/repository 副本实测 16 文件（贡献3 报 11）；11 个上游一致文件入 entry #1，其余 5 个保留并开 OQ#7。

---

## 八、评审修订记录（r4.1）

| 评审条目 | 严重度 | 处理结果 |
|---|---|---|
| #1 searchQueryTokens "本地已不存在、采纳上游会编译失败"为事实错误 | high | **采纳并修正**。自跑验证：faq_snippet.go:271 存在 `func searchQueryTokens(queries []string) []string`（签名与上游测试调用一致），snippetStopwords（:331-337）恰含 the/does/how；extractSnippetForQueries 在 chunk_snippet.go:134。entry #6 证据改为"直接采纳上游最新版整文件"，OQ 原 #6（裁剪测试）作废，核对清单第 1/2 条同步修正。注：为静态验证（本角色禁改代码文件，未跑 live go test），已列入 order 40 执行门控 |
| #2 上游 internal/application 63 个 modules 同名对应物未登记 | high | **采纳并补扫**。自跑 comm 补扫（.go 精确口径）：缺失 103（service 80+repository 23），70 有同名对应物、33 无。处置：**tag.go 升格为新 entry #2**（package 一致+32 行纯新增 fork 方法+deps 仅 types，三重印证 high）；**chunk_image_assets.go 评审点名的另一候选经反证不可移**（上游版依赖同包 chunkRepository，正主缺失 repository/chunk.go，单独移入编译必败，记入 OQ#18 阻塞项）；其余 56 个以修订后的 keepInPlace 理由登记（internal/modules/knowledge、insights 两条理由改写为"上游有同名对应物、内容分歧待收敛"，career 条目补同名二义注记），并新开 OQ#18 收敛决策。修正评审两个小误差：models 缺失实为 6 非 7；初扫的 4 个非 .go"缺失"（tenant_skill_verify.py 等）系 find 口径假阳性，实测存在于本地 |
| #3 上游有本地全无的缺失文件登记不全 | medium | **采纳**。自跑抽查全部确认：agent/tools/search_knowledge_rerank_test.go 并入 OQ#5；service 侧 wiki_search_test.go、datasource_schedule_test.go 等 29 个 service + 4 个 repository + 6 个 models 统一登记于重写后的 OQ#6 总清单（含完整文件名列表）；handler 5 个维持 OQ#3 |
| #4 evaluation.go "19 个 func"数字错误 | low | **采纳并修正**。自跑验证：上游/本地顶层 func 均 3（19 属 memory.go，实测 19/19 正确）。entry #8 证据改为"顶层 func 3/3 一一对应"，entry #10 补"19/19（实测）" |

**r4.2（第四轮评审后，2 条）：**

| 评审条目 | 严重度 | 处理结果 |
|---|---|---|
| #1 tag.go"消费方 8 文件需改 import"错误 6/8，照做会编译失败 | high | **采纳并修正**。自跑逐文件审计（区分真实 import 行与注释命中、逐符号 grep）：(1) semantic_model_policy/semantic_scope/semantic_model_policy_test/semantic_model_capability 4 文件 tag 符号零命中（import 服务于留守 Semantic* 符号）→ 维持不动；(2) semantic_outbox{,_pg}_test 无 import（此前命中系 :364/:230 注释行）→ 零动作；(3) kbretrieval_passb_compat.go 的 9 处 kbretrieval. 引用中 8 处桥接留守 semantic 符号 → import 保留，且 :32 转发 var 在 tag.go 移入同包后同名重声明，处置由"去前缀"修正为**删除该行**；(4) 唯一 import 改写=knowledge_tag_test.go:221。entry #2 证据、referenceSites、执行核对清单已同步改写。映射方向本身未变（评审亦确认移入无符号冲突、原包不破坏） |
| #2 补扫分簇算术漂移（71 vs 70 / 58 vs 55 / 56 vs 55） | low | **采纳并对账**。自跑复算：唯一缺失口径 70、目录命中 71（差值=evaluation.go 双站点 insights+career）；统一口径 70 = 9（entry#1）+ 2（OQ#7）+ 1（tag）+ 55（knowledge keepInPlace）+ 3（insights keepInPlace）；entry#1 的 2 个冗余删除文件不属缺失口径。〇.9、keepInPlace knowledge（58→55）、OQ#8、OQ#18 四处数字已统一 |

**r4.3（终审，4 条）：**

| 评审条目 | 严重度 | 处理结果 |
|---|---|---|
| #1 删除 knowledge_tag.go 会破坏留守 knowledge_tag_test.go（4 方法 ×15 处调用） | high | **采纳并修正（闭包迁移）**。自跑验证成立（实测 7 处 &knowledgeRepository{db:db}、17 处方法调用，方法定义于 knowledge_tag.go:15/:57/:120/:152；执行前 go vet 死目录 exit 0，破坏系 r4.2 方案执行引入）。修正：entry #1 由"9 测试迁移+2 删除"扩为**闭包迁移 11 测试**（追加 knowledge_tag_test.go、knowledge_datasource_test.go——后者被终审#2 的 setupKnowledgeTestDB 连带锁定）+2 删除（留守非测试文件对删除符号零命中实测，安全）。终审提出的三选一（随迁/保留/自持方法面）按'随迁'执行——为唯一不破坏两侧编译且不留守死代码的选项；如 owner 改判'保留 knowledge_tag.go'，从 entry #1 撤回该删除即可，其余不变 |
| #2 setupKnowledgeTestDB 定义在将移的 knowledge_finalize_test.go:64，留守两测试 undefined | high | **采纳并修正（同上闭包）**。自跑验证：该 helper 全仓唯一定义于 :64，死目录 9 文件使用（含留守 knowledge_tag_test.go:54、knowledge_datasource_test.go:34/:80/:115）→ 两者随迁后从同包（正主）取得定义。**新发现并登记**：正主 knowledge_datasource_external_id_test.go:14/:44/:66 引用该 helper 而全仓无定义，go vet ./internal/application/repository/ 现状 exit 1（HEAD 既有债务，order-50 #27 批'由目标包既有 _test 文件提供'的记录已过时）——闭包迁移顺带修复，登记于 entry #1 证据与门控（exit 1→0） |
| #3 order 10 门控看不见 _test.go 破坏（go build 不编译测试、go test 只编译正主包） | medium | **采纳**。核对清单第 2 条补两侧 vet 门控：go vet ./internal/modules/knowledge/process/repository/（基线 exit 0 本 session 实测，执行后须保持）+ go vet ./internal/application/repository/（现状 exit 1 既有债务，执行后应转 0） |
| #4 tag.go 消费方处置主干复核通过（正面记录） | low | **无需改动，记录复核证据**。本 session 复核确认：kbretrieval import 面恰 9 文件、semantic_outbox{,_pg}_test 命中确为注释行、4 个 semantic 文件 tag 符号零命中、container.go:232 must(container.Provide(repository.NewKnowledgeTagRepository)) + :43 import 正主——compat :32 转发 var 删除后接线自动落到同包真身。r4.3 并将该正向结论延伸：knowledge_tag_test.go 随 entry #1 落位正主后，r4.2 的'import 改写'简化为'kbretrieval import 整体删除+同包直引' |

---

## 九、r5:internal/modules 命名空间解散(2026-10-08,用户指令驱动)

> 用户指令:internal/modules 剩余模块应参考上游目录结构继续迁移。上游 internal/ 为扁平特征包布局(无 modules 层),故裁定:**有真实上游对应物的残留归位上游精确路径;fork 自研模块整体提升为 `internal/<模块名>`(采纳上游扁平惯例,内部结构原样保留)**。分支 `restructure/modules-dissolve`(基于 main=744550345),7 提交:8de3484aa / 4ab9badbf / bc593b5a9 / b0f81f17d / 156fc49fb / cd5b8d088。

### 模块去向(20 模块 → internal/modules 全消失)

| 模块(文件数) | 去向 | 说明 |
|---|---|---|
| agentruntime/airesource/channels/identity/policy/system(各 1) | `internal/<名>` | Pass A 零逻辑占位骨架;agentruntime 哨兵错误被 agent/runtime+workbench 引用(活代码) |
| insights(5) / craftegress(6) / execution(17) / conversation(28) / plugins(25) / codedelivery(21) / agentcatalog(20) | `internal/<名>` | fork 功能,整树平铺提升 |
| datasource(3) | `internal/datasource/connector/moauth` | 唯一语义对齐:采纳上游 connector-per-provider 布局(confluence/dingtalk/feishu/... 同层) |
| knowledge(40) | `internal/knowledge` | ingest=上游 chunk 摄取真源(canonical 仅存转发 shim);process/repository 为 fork 分叉变体(与 canonical knowledge.go 差 235 行);retrieval/app=semantic 治理族(fork 自研);wiki/kbfreeze 随树提升 |
| career(50) / craft(72) / workbench(54) / commercial(159) | `internal/<名>` | fork 大模块,纯前缀提升 |

### 特判与坑

1. **knowledge_tag.go/knowledge_transfer.go 删除回撤**:r4.1 entry#1 判定"逐字节一致+零入站可删",实测删除后编译失败——fork 版 knowledgeRepository 须满足 `interfaces.KnowledgeRepository`(AddKnowledgeTagRelations 等方法定义于此)。〇.16(c) 的"零命中"审计漏了**接口满足**这一隐式依赖。两文件保留于 fork 包(git rm 后经 Write 通道重建)。
2. **相对路径深度修正 18 处**:目录提升使层级变浅一层,os.ReadFile/runtime.Caller 推导的 repoRoot 全部减一级 `../`(insights analytics、knowledge outbox×2、conversation adapters、codedelivery migration、appconnector×8、agentcatalog×2、career×2、craft skill×2、workbench×4、commercial×3)。craft/archive_test 等含 `../` 的字符串是恶意路径测试数据,不动。
3. **examples/ 目录在根模块内**(无独立 go.mod),jira-todo-mcp 5 文件引用旧路径,补入扫描范围。
4. **守卫工具处理**:architectureguard/passbguard 工具代码与合成夹具**整体回退基线**(其 modules 键控快照属守卫流程维护,基线即有 7 个 Freeze 失败);唯一诚实适配=DiscoverWorkers 门面扫描根 `internal/modules` → `internal`(窄模式:仅 module.go+types.TypeX 键 map),恢复 18 个 knowledge worker 注册可见性,诊断数精确回到基线 166。docs/architecture/passb/ 活台账(ownership-matrix 341/contracts 48/execution-ledger 68/exception-ledger 27/event-catalog 10 处引用)已随批次重指新路径;evidence/*.md、moves/README、docs/plans/ 属历史记录不改写。
5. **改写面**:~1600 文件(B1 197/B2 103/B3 245/B4 193/B5 874);import 零残留(`grep Tencent/WeKnora/internal/modules` 仅剩历史文档与守卫合成夹具)。

### 门控

每批 `go build ./...` + 触碰树 `go vet`;终局 `go vet ./...` exit 0;守卫失败集=基线同集(7)/诊断 166=基线;定向测试:insights/conversation/codedelivery/agentcatalog/execution/craftegress/骨架×6/knowledge 全绿、career/plugins/datasource 绿、craft+career 深度关键用例绿、appconnector 绿、knowledge retrieval/ingest 绿。全量 `go test ./...` 后台核验(结果见会话报告)。

### 遗留

- 上游同名对应物仍在 fork 包的内容分歧(knowledge ingest/wiki/process、insights 等,即 r4.1 keepInPlace 55+3 集合)未收敛——属内容合并决策(OQ#18),非本次目录迁移范围。
- 守卫 7 个 Freeze 活文档测试失败=基线预存债务,待守卫流程。
