# b2-k-integration 证据文件（Pass B / 20-knowledge-program）

> 节点：`b2-k-integration`（K5 Knowledge 集成）。计划：`docs/plans/passb/20-knowledge-program.md` §8。
> ALIGN_SHA = `b9c09f524`（P-K5-2 Case A 基线对齐 merge commit）；本节点 diff 基线裁定见 K5.3 Step 2。
> 本文件由 K5.1/K5.2/K5.3 递增填写；K5.3 收口时补全差分汇总、门禁命令台账与计数复核章节。

## §别名（K5.2 — 18 条 alias 义务收口删除）

**删行 commit**：`46447494a`（manifest + matrix 同 commit 成对删行 + convert_linked_test.go 注释修正，Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 行级删除权，delete_barrier: ib2 提前收口）。

### 删前逐条零 importer 复检（2026-09-26，worktree HEAD=1485a2480）

命令（对 §7.5 全部 18 条逐一执行）：

```bash
grep -rln "\"$p\"" --include="*.go" . | grep -v _test.go   # 每条 old_import_path
```

结论（18/18 全部通过，非测试与测试文本命中均为空）：

| # | old_import_path | 非测试 importer | 测试文本命中 |
|---|---|---|---|
| 1-12 | `internal/application/repository/retriever/{doris,elasticsearch,elasticsearch/v7,elasticsearch/v8,milvus,neo4j,opensearch,postgres,qdrant,sqlite,tencentvectordb,weaviate}` | 0 | 0 |
| 13 | `internal/application/service/retriever` | 0 | 0 |
| 14 | `internal/infrastructure/chunker` | 0 | 0 |
| 15 | `internal/infrastructure/docparser` | 0 | 0 |
| 16 | `internal/infrastructure/docparser/anydoc` | 0 | 0 |
| 17 | `internal/infrastructure/semantic` | 0 | 0 |
| 18 | `internal/searchutil` | 0 | 0 |

物理目录复检：`ls internal/application/repository/retriever internal/application/service/retriever internal/infrastructure/{chunker,docparser,semantic} internal/searchutil` → 全部 `No such file or directory`（退出码 1，旧路径 Pass A 已搬）。

任意形式文本残留全仓扫描（`grep -rn` 18 条旧路径字面量、含注释，`--include="*.go"`）：唯一命中 = `internal/modules/knowledge/docparser/anydoc/convert_linked_test.go:19`（build 指令注释 `go test -tags anydoc ./internal/infrastructure/docparser/...`，非 import）——与计划 §7.5 预检一致。

### 删行明细（commit 46447494a，+1/-92）

- `docs/architecture/moves/knowledge.yaml`：`alias_obligations:` 区整体删除（L41-L77 共 37 行 = 标题 + 18 条 × 2 行）；删除后 YAML 解析合法，`legacy_files` 53 条原样保留、衔接处无残缺。
- `docs/architecture/passb/ownership-matrix.yaml`：aliases 区 `plan: 20-knowledge-program` 的 18 个 3 行块（old_import_path/plan/delete_barrier）删除，删除前对每块断言 `plan: 20-knowledge-program` 与 `delete_barrier: ib2` 逐块通过；删除后 aliases 区余 81 条、plan-20 剩余 0 条，YAML 解析合法。
- `internal/modules/knowledge/docparser/anydoc/convert_linked_test.go:19`：注释内 `internal/infrastructure/docparser` → `internal/modules/knowledge/docparser`（conventions §10 机械缺口就地补齐）。

### 删行后 passbguard 快照比对（Step 2d）

```bash
go run ./tools/passbguard -root . >/dev/null 2>&1; echo "passbguard_exit=$?"   # → 1（预期，218 条基线诊断不在本节点修复面，P-K5-7）
go run ./tools/passbguard -root . 2>&1 | grep -v '^exit status' | sort | diff - /tmp/k5-readiness-baseline.txt
```

diff 输出恰为 P-K5-7 (ii) 预登记的 3 条新增（`<` 侧），逐字一致，消失集为空：

```
contract-consumer-unrecorded: knowledge.knowledge-base-service: production consumer internal/modules/knowledge/module.go references KnowledgeBaseService but is not recorded
contract-consumer-unrecorded: knowledge.service: production consumer internal/modules/knowledge/module.go references KnowledgeService but is not recorded
contract-consumer-unrecorded: knowledge.tag-service: production consumer internal/modules/knowledge/module.go references KnowledgeTagService but is not recorded
```

alias 双侧奇偶以 manifest↔matrix 行集对照为键（check.go:167-206 实读确认，无 18 字面计数断言）；删行前后该 3+0 差集恒定，奇偶保持。

删行后编译：`go build ./...` 退出码 0；`gofmt -l convert_linked_test.go` 空。

### 收口出册补齐（2026-09-26 审阅 finding 修复轮）

**finding**（reviewer，重要级）：`make verify-module-moves`（DAG b2-k-integration 五 gates 之一；K5.3 Step 2 :806 预期「退出码 0（16 manifests verified）」）由 BASE=1485a2480 绿转 HEAD 红——18 条 `alias-1to1: move from … 缺少对应 alias_obligation`。本轮 HEAD 复现确认（`make verify-module-moves` → 18 条诊断 + `make: Error 1`）。

**根因**：`tools/modulemove/verify.go:108-124` 要求每条 `move_packages[].from` 必须有对应 `alias_obligation`（old_import_path == from，1:1 硬校验）；K5.2 Step 2(a) 删除了 alias_obligations 18 行，但 move_packages 区的 18 条 from/to 仍在册，1:1 断裂。全仓 16 份 manifest 对照：其余 15 份均为 move==alias；identity/agentcatalog/system 为 0/0（`move_packages: []` + 空 owned_files 三区）——别名收口的完整终态是 move_packages 对应行**同窗出册**（计划 Step 2(a) 的「alias_obligations 18 行」为收口面的别名义务侧，出册侧是其成对面，属 conventions §10 机械缺口就地补齐）。

**修复**（commit 见本节末，manifest 单文件）：`docs/architecture/moves/knowledge.yaml` 三区出册为 `[]`——① `move_packages` 18 条（from/to）；② `owned_files.move_sources` 18 条（verify.go:295-300 move_sources == from 集合一致性强制，不同步删会新增 owned-move-sources 诊断）；③ `owned_files.move_targets` 36 条（18 to 树 + 「alias (no-logic forwarding) packages left at the old paths」注释段 + 18 旧路径别名登记；README.md:28-29 语义「== 全部 move_packages[].to 树 + 旧路径别名包」，两侧来源均已收口）。删除前对三区分别断言 36/18/36 条；删后 YAML 解析合法，`legacy_files` 53 / `importers` 12 / `module_files` 3 原样。matrix 无 move_sources/move_targets 区（grep 零命中），无联动。工具消费面排查：architectureguard 仅消费 `move_packages[].from`（check.go:1120-1122 → MoveFroms，:1459 legacy-guard 横向目录归属判定——18 个旧路径物理目录已不存在，不可能出现新文件，无影响）；passbguard 不读 move_packages/move_sources/move_targets。

**修复后回归**（命令原样，2026-09-26）：

| 命令 | 退出码 | 摘要 |
|---|---|---|
| `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)`（finding gate 转绿） |
| `go build ./...` | 0 | 仅 linker 既有 warning |
| `make check-backend-architecture` | 0 | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`，OK 0 violations |
| `go run ./tools/passbguard -root .` | 1（预期基线） | 快照 diff 基线仍恰为 P-K5-7 (ii) 预登记 3 条，消失集为空（manifest 出册不触及诊断面，实测确认） |
| `make check-passb-readiness` | 2（= go run 退出码 1 的 make 层包装，Makefile:262 recipe 失败标准行为；与上轮 go run 口径 1 同一命令） | 诊断集与上行相同 |
| `go test -count=1 ./internal/modules/knowledge/...` | 0 | 26 包全 ok、0 FAIL（含 kbfreeze 守卫、module 门面五测试） |

manifest 出册 commit：`a29abf40e`（refactor(passb) 同窗补齐，+3/-91）。

**OCR 修复轮 R1（2026-09-26）**：`alias_obligations:` 键行补回（`  []` 空列表两行，identity.yaml:6-7 形态）——46447494a 整键删除偏离 16 份 manifest 统一形态（唯一缺键），出册后语义不变仅对齐 schema；验收 `make verify-module-moves` exit 0（16 manifests verified）、16/16 键在位、YAML 合法、passbguard 快照 diff 仍恰 3 条。详见 `.superpowers/sdd/passb/b2-k-integration/ocr-fix-r1-report.md`。


## §例外/shim 收口核对（K5.2 Step 3，只读盘点 — ib2 删除批输入）

### 例外 30 行（K5 不删行，删除前置=airesource/policy 门面端口或 ADR 修订）

命令与结果（2026-09-26）：

```bash
grep -B4 "plan: 2[1-4]-knowledge" docs/architecture/passb/exception-ledger.yaml | grep -c "id: exc-"
# → 30
```

30 条 id 与计划 §7.6 表逐 id 一致：exc-0088、0089-0091、0106-0111、0112-0116、0117-0129、0130-0131；逐条 `remove_at: ib2`（30/30，脚本核对无缺漏）。双侧一致性：`go run ./tools/architectureguard` → 退出码 0、`OK (0 violations)`、`literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16`（importExceptions 与 ledger 双侧一致，无例外类诊断）。

登记：**K5.2 不删例外行**（conventions §3：行属主是 21/22/23/24 计划）；ib2 删除批前置 = airesource/policy 根门面暴露端口（K1 Brief §10、K2 Brief §7 处置建议照录）。本任务 `git diff -- docs/architecture/passb/exception-ledger.yaml` 为空。

### 宿主过渡 shim/compat 全清单（16 生产件 + 1 测试垫片；删除归 ib2，K5 只盘点）

同包残留口径：shim 所在宿主包内其余 `.go` 文件对 shim 符号的裸标识符引用（`grep` 全符号扫描，逐文件归档）；跨包消费者口径：import 宿主包的文件（service 91 / repository 194 / handler 51 / handler/session 24，其中大部分属宿主包既有业务面而非 shim 符号面，ib2 逐符号改写时按 shim 符号限定引用定位）。残留消费者全部可归入三类：**ib2 改写项**（rbac_lookups、knowledge.go、knowledgebase.go、qa.go、container、router 等集成面）、**推迟件**（K4 Brief (f) 推迟批留守文件）、**shim 互引**（K1 adapter ↔ K4 compat 等），无计划外残留。

| 件 | 文件 | 真源落位包 | 符号面 | 同包残留消费者（PROD/TEST） |
|---|---|---|---|---|
| K1-a | `internal/application/repository/chunk_ingest_shim.go` | `modules/knowledge/ingest`（repository 面） | 导出 3（NewChunkRepository、ErrChunkNotFound、ErrChunkRevisionConflict） | 0/0 |
| K1-b | `internal/application/service/chunk_ingest_shim.go` | `modules/knowledge/ingest`（service 面） | 导出 6 + 未导出 6（buildVLMCaptionPrompt、enqueueDataTableSummaryIfNeeded、isFinalAsynqAttempt、sameChunkDocument、validateParserEngineOverrideURLs、sanitizeOCRText、previewText*） | 5/2（knowledge_create/post_process/process/temporary_document + span_trace_seam_adapter；chunk_ingest_test_shim_test、knowledge_summary_test——均为推迟件与 shim 互引） |
| K1-c | `internal/application/service/span_trace_seam_adapter.go` | `modules/knowledge/ingest` + `modules/knowledge/process`（span seam） | 导出 13（Provider/Factory 族 + NewSpanTraceSeamAdapter） | 2/1（chunk_ingest_shim、kbprocess_passb_compat 互引；chunk_ingest_test_shim_test） |
| K1-d | `internal/handler/chunk_ingest_shim.go` | `modules/knowledge/ingest`（handler 面） | 导出 14（ChunkHandler + 方法族转发） | 2/2（rbac_lookups.go=ib2 改写项；rbac_lookups_test、knowledge_mutation_admission_test） |
| K2-a | `internal/application/repository/kbretrieval_passb_compat.go` | `modules/knowledge/retrieval/app/repository` | 导出 9（Semantic* Repository/Store 族、NewKnowledgeTagRepository） | 0/1（kbretrieval_passb_compat_test.go 垫片引用） |
| K2-b | `internal/application/service/kbretrieval_passb_compat.go` | `modules/knowledge/retrieval/app` | 导出 35 中生产转发 17 + 未导出 helper 族（recordKBActivity、semanticScopeGuard、requireKBWrite 等） | 18/8（kbshare/knowledge/knowledgebase/organization/tenant/tag 等留守宿主文件与推迟件、knowledge_faq_k3_delegate 互引；semantic_scope_test 等） |
| K2-c | `internal/handler/kbretrieval_passb_compat.go` | `modules/knowledge/retrieval/app/handler` | 导出 6（TagHandler、SemanticInternalHandler、SemanticModelPolicyHandler + 构造器） | 0/0（消费在 container/router=ib2 改写项，跨包口径） |
| K2-d（垫片） | `internal/application/repository/kbretrieval_passb_compat_test.go` | `modules/knowledge/retrieval/app/repository` | 最小符号 `knowledgeTagRepository`（Ruling 2026-09-24-TEST-SUPPORT-SHIM，remove_at: ib2；台账在 b2-k-retrieval.md） | 服务 knowledge_tag_test.go:219/:241（24 计划推迟件） |
| K3-W1 | `internal/application/service/wiki_k3_compat.go` | `modules/knowledge/wiki` | 导出 6（WikiIngestPayload、ErrWikiIngestConcurrent 等 var/type 别名）+ 未导出 9（enqueueWikiRetract、extractRealText 等） | 5/3（knowledge_clone_move/delete/post_process/process + chunk_ingest_shim 互引；housekeeping/move_wiki/summary 测试——推迟件） |
| K3-W2 | `internal/application/service/wiki_k3_ctor_compat.go` | `modules/knowledge/wiki` | 导出 3 构造器 + seam 接线 2（wikiK3Seams、wikiK3SpanAdapter） | 0/3（wiki_k3_compat_test、wiki_k3_span_adapter_test；生产消费在 container=ib2 改写项） |
| K3-W3 | `internal/application/repository/wiki_k3_repo_compat.go` | `modules/knowledge/wiki` | 导出 7（ErrWiki* 族、EscapeLikePattern） | 1/0（tenant_member.go 的 escapeLikePattern=ib2 改写项） |
| K3-H1 | `internal/handler/wiki_page_k3_compat.go` | `modules/knowledge/wiki`（handler） | 导出 2（WikiPageHandler、NewWikiPageHandler） | 1/1（rbac_lookups.go:105/:183=ib2 改写项；rbac_lookups_test=10-identity 推迟件） |
| K3-H2 | `internal/handler/faq_k3_compat.go` | `modules/knowledge/faq`（handler） | 导出 2（FAQHandler、NewFAQHandler） | 0/0（消费在 routes/container=ib2 改写项） |
| K3-S1 | `internal/handler/session/wiki_fixer_scope_compat.go` | `modules/knowledge/wiki`（ResolveBuiltinWikiFixerTenantScope） | 未导出方法 1（`*Handler.resolveWikiFixerTenantScope`，B0.3 去方法化裁定过渡） | 1/0（qa.go:205=conversation 属主 ib2 改写项） |
| K3-D1 | `internal/application/service/knowledge_faq_k3_delegate.go` | `modules/knowledge/faq`（Service） | knowledgeService FAQ 面方法 20（冻结接口 15 + 宿主未导出 4 + faqSvc） | 3/2（agent_service、kbretrieval_passb_compat、knowledge_clone_move；faq_k3_test_support_shim、knowledge_write_access_test） |
| K4-a | `internal/application/service/kbprocess_passb_compat.go` | `modules/knowledge/process` | 未导出 8（writeResourceIDs、loadKnowledgeWrite(Batch)、knowledgeWriteKB、documentProcessTaskOptions 等） | 9/1（knowledge{,_clone_move,_create,_delete,_delete_plan,_process,_reparse_scope,_replace,_transfer}.go + span_trace_seam_adapter/chunk_ingest_shim 互引；knowledge_index_content_test——全部为推迟件与 shim 互引） |
| K4-b | `internal/handler/kbprocess_passb_compat.go` | `modules/knowledge/process/handler` | 未导出 5（requireTaskProgressTenant、resolveHandlerKBAccess(For)、kbAccessHTTPError、resolvedKBAccess） | 4/1（knowledge.go、knowledgebase.go、knowledge_download.go、faq_k3_compat 互引；kb_access_test） |

旁证发现（不在 17 件清单、属 K3 随迁测试/垫片，随各自 Brief 删除批收口）：`wiki_k3_compat_test.go`、`wiki_k3_span_adapter_test.go`、`wiki_k3_test_support_shim_test.go`、`knowledge_faq_k3_test_support_shim_test.go`（internal/application/service）。

### container.go:36-38 三行旧路径 import 改写申请

已并入 K5.1 Brief (g)（`docs/architecture/passb/briefs/b2-k-integration.md:163`），本任务不重复登记。
