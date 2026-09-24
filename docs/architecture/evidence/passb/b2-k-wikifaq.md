# Evidence — b2-k-wikifaq（Pass B 23-knowledge-wikifaq）

> 状态：K3.1 草稿（差分章节由 K3.1 建立，K3.4 终稿收口四行全表）。基线 BASE=4649630dfdfc5a0a08e451b61c9cbf547ffe44af；节点分支 codex/passb-b2-k-wikifaq。

## 0. T0 旧实现基线（迁移前，2026-09-25 实跑）

| 命令 | 结果 | 计数 |
|---|---|---|
| `go test -count=1 ./internal/application/repository/ -run 'Wiki'` | ok | 2 PASS |
| `go test -count=1 ./internal/application/service/ -run 'Wiki\|Linkify\|Slug'` | ok | 85 PASS / 0 FAIL / 0 SKIP |
| `go test -count=1 ./internal/handler/session/ -run 'WikiFixer\|ResolveBuiltin'` | ok | 5 PASS |
| `go test -count=1 ./internal/handler/session/... ./internal/application/service/ -run 'Wiki'` | ok（两包） | service 侧 48 PASS |

## 1. K3.1 差分（Wiki 摄取/Finalize/页面操作 + wiki fixer，双跑比对）

### 1.1 用例清单与双跑结果

迁移后命令（新实现同用例运行）：

| 命令 | 结果 | 计数 | 与 T0 比对 |
|---|---|---|---|
| `go test -count=1 ./internal/modules/knowledge/wiki/...` | ok | 130 PASS / 0 FAIL / 0 SKIP | 集合 = T0 基线 85（repo 2 + service 83）+ session 5 + 测试类目展开（`-v` 顶层 RUN 计数口径：T0 顶层 92 → 迁入后同断言全 PASS；TestRepairContentLinks 移至宿主真实 seam 双跑，见 §1.3） |
| `go test -count=1 ./internal/handler/session/... ./internal/application/service/ -run 'Wiki'` | ok | — | K4 留驻（knowledge_post_process_wiki_enqueue_test.go、knowledge_move_wiki_test.go、knowledge_summary_test.go、knowledge_housekeeping_test.go）经 W1/W2 PASS |
| `go test -count=1 ./internal/modules/knowledge/kbfreeze/` | ok | 2 守卫 | R0 冻结面零违反 |
| `go test -count=1 ./internal/application/repository/`（全包） | ok（357s） | — | escapeLikePattern/哨兵留驻拓扑下 tenant_member 等全量绿 |
| `make verify-module-moves` | `modulemove: OK (16 manifests verified)` | — | 12 wiki 行已删、5 兼容行已登记 |
| `make check-backend-architecture` | `OK (0 violations)`，`total=633 \| redis=23 lite=23 \| hooks=58` | — | 路由/worker/hook 计数与 pass-a-acceptance 台账一致（§8 三方一致） |

### 1.2 TypeWikiIngest / TypeWikiFinalize 状态机逐用例等价

- **claim/peek/finalize 合并（`wiki-finalize-<kbID>` TaskID 去重）**：`TestEnqueueWikiFinalizeOnlySchedulesAcceptedRows`（accepted=true 断言恰 1 个 TypeWikiFinalize 任务入队）+ `TestProcessWikiFinalizeDefersFolderPruneWhileIngestIsPending`（ingest 未排空→不剪枝、durable prune 行保留、恰 1 个重排任务）双跑 PASS；`scheduleFinalize` 的 `asynq.TaskID("wiki-finalize-"+kbID)` 与 ErrTaskIDConflict 吸收分支未改动（纯移动，diff 仅 package/import）。
- **5-docs-per-batch fan-out**：`WikiMaxDocsPerBatch=5` 常量与 `claimPendingList`/`peekPendingList` 调用点零变化（wiki_ingest.go 常量区逐字保留）。
- **MaxRetry 10 / Timeout 60/30 分钟**：`wikiIngestMaxRetry=10`、`asynq.Timeout(60*time.Minute)`（ingest）/`30*time.Minute`（finalize）逐字保留；`TestIsTransientLLMError_*` 5 用例（含 RateLimit403 网关语义）PASS。
- **ErrWikiIngestConcurrent 指数退避重试语义**：`TestWikiDeletedKnowledgeBaseCleanupFailureRetries`、guard 双任务类型 drain 用例 PASS；哨兵经 W1 `var ErrWikiIngestConcurrent = wiki.ErrWikiIngestConcurrent` 保持同一实例（router/task.go:124 `errors.Is` 语义不变——同变量引用）。

### 1.3 TestRepairContentLinks 宿主真实 seam 双跑（跨包迁移声明）

原 service/wiki_page_test.go:176-228（断言 resolveDeadSlug 的 bigram 模糊复活真实行为）。迁移后落在 `internal/application/service/wiki_k3_compat_test.go`（宿主包可引用未导出 `resolveDeadSlug` 与 W2 `wikiK3Seams()` 生产同构接线），驱动 `wiki.NewWikiPageService(..., wikiK3Seams()).RepairContentLinks`：

```
=== RUN   TestRepairContentLinks
--- PASS: TestRepairContentLinks (0.00s)   [3/3 子用例 PASS：mangled-uuid bigram 修复 / live links 不动 / 不可解析死链保留]
```

被测实现 = wiki 包生产代码路径；seam 目标函数 = 迁移前同包直引的同一 `resolveDeadSlug`（slug_fuzzy.go:91）。等价成立。

### 1.4 wiki fixer 共享租户作用域

`wiki_fixer_scope_test.go` 5 用例随迁 package wiki，调用点改导出名 `ResolveBuiltinWikiFixerTenantScope`，断言逐字未动：5/5 PASS（T0 基线 5 PASS）。`qa.go:205` 经 S1 薄委托同一实现——方法体仅转发，逐分支行为不变。

### 1.5 纯移动验收（spec §14.2）

`git diff --summary` 全部识别 rename（26 个 R 标记：12 生产 + 14 测试，含 wiki_page.go→wiki_page_repository.go、handler/wiki_page.go→wiki_page_handler.go 两处防冲突更名）；生产文件 diff 除 package 子句、import 块、§5.1 导出改名（8 符号 + 2 常量族）、seam 接线点（6 处调用改 `s.seams.X`/`s.prompts 不涉`）外零函数体变化。逐文件 `diff <(git show BASE:<old>) <new>` 复核：wiki_ingest.go 86 行差异全部落于上述类别（详见审查包）；dedup/cite/linkify/taxonomy/lint/slug_handles 仅 package 行差异。

### 1.6 已知拓扑裁定（非行为差异，登记供 IB2 收口）

1. **Ruling 2026-09-25-CYCLE-FORCED-COMPOSITION**：container.go:311 provider 目标切至 `knowledgeWiki.NewWikiPageRepository`（import 环强制，Brief (a) 同款切换提前落地；IB2 转核验项）。
2. **minTextContentRunes 双副本**：wiki.MinTextContentRunes 与 W1 `minTextContentRunes` 初始值同为 10；生产零改写；宿主测试（knowledge_summary_test）改写副本与 K4 读点同侧，行为不变；IB2 K4 迁出时收口单一变量。
3. **Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY**：wiki 包 11 条 file→package 豁免登记（check.go importExceptions + exception-ledger exc-0106..0116，owner=23-knowledge-wikifaq，RemoveAt=ib2）；例外计数 105→116 机械修正。
4. **Ruling 2026-09-24-TEST-SUPPORT-SHIM**：宿主 `wiki_k3_test_support_shim_test.go` 垫片（wikiGuardTaskQueue 最小重建，remove_at ib2）。
5. **§5.2 实测修订**：`contains` seam 无调用点（grep 零命中）未建；`IsKnowledgeBaseNotFound` seam 替代 wiki→repository 哨兵 import（计划 §10.4 二选一备选路径，已选 seam 注入）。

## 2. K3.2/K3.3/K3.4 章（占位，后续任务填充）

- FAQ 面 TypeFAQImport、冻结端口委托、hook 恢复等价：K3.2/K3.4 填充。
