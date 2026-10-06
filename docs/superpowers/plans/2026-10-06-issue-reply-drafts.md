# 2026-10-06 Issue 回帖草稿（18 分支 / 55 票）

> 草稿供仓库所有者审阅后代发。18 个 fix 分支均基于 `merge/upstream-20261003`，未 push；17 个有票分支对应 issue 均未回帖，第 18 个为无票战役分支（见下）。
> 回帖目标仓库：**Tencent/WeKnora**（55 票全部在该仓库；fork `1123786563/WeKnora-fork01` 无独立 issue）。
> 事实来源：各分支 commit message（根因/修法/测试要点）+ `gh issue view` 核对票面语言；测试表述均与 commit 实际改动文件核对，无编造。
> 语言口径：跟随 issue 正文语言（英文票英文、中文票中文）。落款：中文票用「（fix 分支：…）」，英文票用同义英文（Fix branch: …, not yet merged upstream）。
> 并票说明：#3910 与 #3942 同签名同修（并票修复，同款回帖）。
> 增补轮（round14-16，2026-10-06 追加）：+3 分支 / 12 票（fake-success2 / sync-integrity / truncate），事实来源与语言口径同上。
> 并票说明（增补轮）：#3833/#3775 与 #3778/#3692 各为同 commit 并票修复，草帖互见。
> 终轮（2026-10-06 追加）：+2 分支 / 6 票（wiki-guard / final），事实来源与语言口径同上。
> 并票说明（终轮）：#3792 与 #3714 同 commit 修复（11176276b），草帖分列。
> 终态增补（2026-10-06 追加）：+1 战役分支 fix/migrations-dedup-preexisting（无票，6 commit；其中 3 个 commit 含真产品缺陷修复，详见文末「无票产品缺陷修复」节）。

## 合并预演结论

**12 分支按时间序合入零冲突（2026-10-06 预演）。** 时间序 = 各分支首 commit 时间序：copyindices → datasource-pagination → kg → agent-param → reparse → misc → misc2 → fake-success → misc3 → anydoc-ext/sandbox → misc4 → confluence。本编纂轮另用 `git merge-tree` 顺序模拟复核一次（对象写入隔离在临时目录，仓库零改动），12 步全部 clean merge，与预演结论一致。

**增补 3 分支的预演（round14-16，同款 merge-tree 顺序模拟、对象写入隔离）：** 时间序接续为 fake-success2 → sync-integrity → truncate（首 commit 均晚于 confluence，实测 06:33/06:46/07:03）。**上轮「零冲突」结论不再成立，新增两处已知冲突对**：① fake-success2 × copyindices——milvus/qdrant `repository_test.go` 两文件内容冲突（两分支各做同款 `allFields` 补 `fieldLanguage` 修复并追加相邻测试，合并时须手工择一）；② truncate × datasource-pagination——notion `client.go` 内容冲突（两分支都改 `paginatePages`：本票增 incomplete 检查，该票加 seen-cursor 防护，同函数交叠）。sync-integrity 与其余 14 分支两两零冲突，其余新组合均 clean。两处冲突均局限于上述文件、需合并时手工解，不改变各草帖内容。

**终轮 +2 分支（wiki-guard 首 commit 07:20:35、final 首 commit 07:37:01，均晚于 truncate 的 07:03）当时未纳入上述预演**——补演已完成、且新增战役分支，终态见下节（本节及上文的中间结论以终态为准）。

## 合并预演终态更新

**终态（2026-10-06）：18 分支 = 16 干净 + 2 对已知冲突（涉 4 分支）**，同款 `git merge-tree` 顺序模拟、对象写入隔离。相对增补轮三点变化：

- 终轮 +2 补演完成：wiki-guard（首 commit 07:20:35）与 final（07:37:01，含无票尾 commit ebe41d6a6）按时间序接入，与既有分支全组合 clean；
- 新增第 18 个战役分支 fix/migrations-dedup-preexisting（无票，首 commit bf1d0fc18 08:46:13，为当前最晚、时间序接续于 final 之后；共 6 commit），与既有 17 分支两两 clean；
- 仍存的两处已知冲突对不变：① fake-success2 × copyindices——milvus/qdrant `repository_test.go` 两文件内容冲突：两分支各做同款 `allFields` 补 `fieldLanguage` 修复并追加相邻测试，属同款双修，**合并时取任一侧即可**；② truncate × datasource-pagination——notion `client.go` 内容冲突：两分支都改 `paginatePages`（truncate 增 incomplete 检查即 #3836，datasource-pagination 加 seen-cursor 防护即 #3877），同函数互补，**合并时两侧都保留**。

除上述两对外，其余两两组合 clean；两处冲突仍局限于上述文件、需合并时手工解，不改变各草帖内容。原「17 分支整体合入前需补一轮同款 merge-tree 顺序模拟」的旧结论由本节替换。

## 分支 / 票号 / commit 对照

| 分支 | 票 | commit |
|---|---|---|
| fix/copyindices-3868-3871 | #3868 #3869 | 46d7a745e |
| | #3870 | ebdfe845d |
| | #3871 | f1cabe4e9 |
| fix/datasource-pagination-3838-3839-3877 | #3838 | e583a1226 |
| | #3839 | ff15836d7 |
| | #3877 | 8fb7d1f72 |
| fix/kg-3873-3945-3953 | #3873 #3953 | ee72e4431 |
| | #3945 | f892ee3be（另 ee72e4431 含工具侧配套） |
| fix/agent-param-3934-3935-3946 | #3934 | 6696455d4 |
| | #3935 #3946 | 7ca8517eb |
| fix/reparse-3851-qa-usage-3865 | #3851 | 4709f2887 |
| | #3865 | c575e1473（测试补交 4ac2d89a2） |
| fix/misc-3974-3947-3962 | #3974 | 7b4c9959b |
| | #3947 | 23aca1997 |
| | #3962 | b9c59afb7 |
| fix/misc2-3951-3940-3928 | #3928 | 09c632dbe |
| | #3951 | bc5e5c77b |
| | #3940 | 748b7257f |
| fix/fake-success-3880-3872-3854 | #3880 | 4231ffcf1 |
| | #3872 | dd4aaa86b |
| | #3854 | 270fbda9a |
| fix/misc3-3911-3918-3922-3926 | #3926 | fe0077992 |
| | #3922 | 5de9bcc6c |
| | #3918 | fbe78f98c |
| | #3911 | 42bd3d008 |
| fix/anydoc-ext-3932-sandbox-3942 | #3932 | fcd3fd789 |
| | #3942（并 #3910） | 7be97254c |
| fix/misc4-3941-3837-3878 | #3941 | 0441f2022 |
| | #3837 | 5c3b6a124 |
| | #3878 | 7f07f6448 |
| fix/confluence-3856-embed-3898-docx-3849 | #3849 | 93206a66a |
| | #3898 | e300fa6d4 |
| | #3856 | 826138fcf |
| fix/fake-success2-3832-3833-3834-3835-3775 | #3832 | c7847d11f |
| | #3833 #3775 | b39b6edd9 |
| | #3835 | ff0ad2362 |
| | #3834 | d21dd9731 |
| fix/sync-integrity-3778-3692-3690-3774 | #3774 | 23b11cd56 |
| | #3778 #3692 | 935ef6be7 |
| | #3690 | 753e8a025 |
| fix/truncate-3836-summary-3776-rss-3823 | #3836 | 8e044669c |
| | #3776 | 478c8659e |
| | #3823 | 686a4dac7 |
| fix/wiki-guard-3792-3714-rrf-3796 | #3792 #3714 | 11176276b |
| | #3796 | 7d091bfbb |
| fix/final-3828-3811-3806 | #3828 | 6a4b22a18 |
| | #3811 | 9937a2fe6 |
| | #3806 | 8c4bb008d |
| | 无票（见文末「无票产品缺陷修复」节） | ebe41d6a6 |
| fix/migrations-dedup-preexisting（战役，无票） | 无票（3 个 commit 含真缺陷修复，见文末同节） | bf1d0fc18 / bd942d25b / 001ac4b01 / f27b34e69 / e62e1317b / 6b0545efb |

---

## fix/copyindices-3868-3871

### #3868 Weaviate CopyIndices 游标不前进时同一页被无限重放（中文）

根因：`CopyIndices` 用每页最后一个对象的 `_additional.id` 反推 `after` 游标，服务端重复返回同一页时游标不动，又没有 seen 集合、无「游标必须前进」检测、无页数上限，形成不动点无限重放。

修法要点：
- 每页 Do 后检查 `result.Errors`，解包拆两次 comma-ok、`_additional.id` 改带检查断言，坏响应不再 panic（#3869 一并修复）；
- 新增 seen 游标集合：整页解析后、写入前检测重复，no-progress 即报错，重复页零写入；
- `maxCopyIndicesPages=10000` 页上限，触顶报错优于拖到任务超时。

测试：httptest 假 GraphQL 恒返同页 → 报错且 batch/objects 仅请求一次；errors 响应 → 返回错误不 panic。

（fix 分支：fix/copyindices-3868-3871，commit 46d7a745e，尚未合回上游，欢迎验证）

### #3869 Weaviate CopyIndices 不检查 GraphQL errors 时直接 panic（中文）

根因：GraphQL 错误响应是 HTTP 200 + `errors` + 无 `data`，SDK 不转成 Go error，`result.Data["Get"]` 为 nil，而 `.(map[string]interface{})` 是无检查单值断言，直接 panic 掉进程。

修法要点：
- 解包前先检查 `len(result.Errors) > 0`，返回带原始 message 的 `weaviate: copy indices query failed`；
- 解包拆成两次 comma-ok 断言，缺 `Get`/class 键返回 invalid response 而不是 panic；
- 同一 commit 另补游标防重放（seen 集合 + no-progress 报错）与 `maxCopyIndicesPages=10000` 页上限（即 #3868 的修法）。

测试：返回「HTTP 200 + errors + 无 data」的假 Weaviate 服务端走真实 `CopyIndices`：修复前 panic，修复后返回 `weaviate: copy indices query failed: <message>`；恒返同页场景报错且 batch/objects 仅一次。

（fix 分支：fix/copyindices-3868-3871，commit 46d7a745e，尚未合回上游，欢迎验证）

### #3870 Qdrant CopyIndices 页边界重复写入一个点（中文）

根因：手动翻页用**页尾点的 id** 当下一页 `offset`，而 Qdrant 的 `ScrollPoints.Offset` 是**包含**语义（"Start with this ID"），每个页边界重复写一个点；SDK 本来在 `ScrollAndOffset()` 的 `next_page_offset` 给了正确游标，代码调的 `Scroll()` 把它丢掉了。

修法要点：
- `Scroll()` → `ScrollAndOffset()`，以服务端游标推进，`next_page_offset == nil` 即完成；删除页尾 id 当 offset 与短页终止；
- `seenOffsets` 游标集合 + `maxCopyIndicesPages=10000` 上限：游标重复/超限报 no-progress，不静默截断。

测试：150 源点跨 3 页 → 恰好 150 目标点零重复；游标卡死 → 报错；短页不截断。

（fix 分支：fix/copyindices-3868-3871，commit ebdfe845d，尚未合回上游，欢迎验证）

### #3871 Milvus CopyIndices offset 窗口静默漏向量（中文）

根因：普通 `query` 的结果顺序不是 API 契约（segment 合并返回），拷贝本身每页都在往同一个 collection 写目标行、顺序持续漂移，offset 窗口漂移后跳行/重读，而「短页即结束」把剩余向量静默丢掉，返回仍是 nil。

修法要点：
- 遍历改为按 `sourceToTargetChunkIDMap` 以 64 个 chunk id 一批读取（`and(kb eq, chunk_id in)`），结果顺序漂移不再影响正确性；
- 删除 offset 窗口与短页终止；不设行 limit（QA 行与主行共用 chunk_id，limit=64 会截断）；无行的源 chunk 保持跳过 + Warn；
- `allFields` 补 `fieldLanguage`，多语言集合拷贝不再丢 language（顺带治愈预存的 float 向量转换测试）。

测试：gRPC fake Milvus 在每次 Upsert 后旋转 Query 顺序（行集合不变只改顺序），220 行（200 chunk + 20 QA）全量写入零重复。

（fix 分支：fix/copyindices-3868-3871，commit f1cabe4e9，尚未合回上游，欢迎验证）

## fix/datasource-pagination-3838-3839-3877

### #3838 飞书 bitable 字段分页重复 page_token 不终止（中文）

根因：`readBitableRecords` 的字段分页循环只有「本页 items 为空」与「`!has_more || page_token == ""`」两个退出条件，token 原地重复（或 A→B→A 环）时同一页字段被反复请求、反复 append 进表头，直到 deadline（实测 2 秒 5.5 万次请求）。

修法要点：
- 字段循环加 `seenFieldTokens` 集合 + `maxBitableFieldPages=10000`：token 重复（恒定/环）或超限即报错，表头不再无界重复追加；
- 空 items break 防御保留并更正失实注释；records 循环与 `maxTableRows` 不动。

测试：恒定 token 2 请求即报错；A→B→A 环 3 请求即报错；正常翻页表头完整。

（fix 分支：fix/datasource-pagination-3838-3839-3877，commit e583a1226，尚未合回上游，欢迎验证）

### #3839 GitLab 分页重复 X-Next-Page 不终止（中文）

根因：`projects()` 与 `tree()` 两条循环只依据响应头 `X-Next-Page` 判断是否继续，该值不做任何校验——既无「已取页码」去重也无页数上限，服务端反复返回同一页时同一页被反复追加（实测 2 秒 9.6 万次请求直到 context 到期）。

修法要点：
- 两条循环各加 seen 页码集合（播种 "1"）+ `maxPaginationHops=10000`，对齐 confluence 已有防护写法；
- `X-Next-Page` 重复或超限报错，不再无限重取。

测试：假 GitLab 恒返 `X-Next-Page: 2` → 2 请求即报错；正常翻页与既有行为不变。

（fix 分支：fix/datasource-pagination-3838-3839-3877，commit ff15836d7，尚未合回上游，欢迎验证）

### #3877 Notion 分页缺重复游标防护与页数上限（中文）

根因：`paginatePages`（SearchPages/QueryDatabaseAll 共用）与 `GetBlockChildrenFlat` 的循环只有 `!HasMore || NextCursor == ""` 一个退出条件，重复 `next_cursor` 或「has_more=true 但 results 空」时以 3 rps 稳定空转到任务 deadline（约 2.16 万次请求）。

修法要点：
- 两函数各加 seen cursor 集合 + `maxPaginationHops=10000`：next_cursor 重复或超限报错；
- `paginatePages` 另补连续空页防护（`maxEmptyResultPages=10`，只数连续、不误伤交替空页），消除零产出空转。

测试：恒定游标 / 恒空页 / 正常推进 / Flat 重复游标四条。

票外注记：`getBlockChildrenRecursive` 仍有同形裸游标循环（有 1000 块部分上限，恒空形态仍可自旋），本轮未修。

（fix 分支：fix/datasource-pagination-3838-3839-3877，commit 8fb7d1f72，尚未合回上游，欢迎验证）

## fix/kg-3873-3945-3953

### #3873 query_knowledge_graph 静默截断关系数（中文）

根因：30 条关系上限命中时关系在去重循环里被直接丢弃——不计数、不记日志、输出不留标记，首行还用截断后的长度汇报「Found N relations」，模型把被砍过的子图当成完整邻域作答。

修法要点：
- 命中上限时 `Data` 下发 `relations_total`/`relations_omitted`，主行改 `Found 30 of 42` 形式，Output 附截断说明；`annotateGraphResult` 渲染 `<graph_truncated>` 让模型可感知边界；
- chunk 10 条与实体词 8 个上限同理由 Data 键可见（`evidence_total/omitted`、`dropped_terms`）；
- 先校验后截断，`relations_total` 只计有效关系（关系证据校验对所有 scope 生效，见同 commit 的 #3953 修复）。

测试：新增 6 例，含 `TestQueryKnowledgeGraph_ReportsRelationTruncation`、`TestQueryKnowledgeGraph_ReportsDroppedSearchTerms`、`TestAnnotateGraphResultMarksTruncation` 等（tools 与 modelcontext 两域）。

（fix 分支：fix/kg-3873-3945-3953，commit ee72e4431，尚未合回上游，欢迎验证）

### #3945 Neo4j SearchNode reverses incoming edges and drops same-named entity sources（英文）

Root cause: the record decoder always assigned `n` to `Node1` and `m` to `Node2` regardless of the stored relationship direction, and deduplicated nodes by name alone, so a same-named instance in another document (with its chunks/attributes) was silently skipped.

What the fix does:
- Direction: compare `Relationship.StartElementId` against the endpoints' `ElementId`s so `Node1`/`Node2` always reflect the direction stored in the library; relations are deduplicated by relationship `ElementId`, so one physical relationship decodes exactly once while genuinely reciprocal relations survive.
- Same-name nodes across documents now merge their `chunks`/`attributes` (deduplicated, order-preserving) instead of keeping only the first instance.
- `GraphRelation` gains read-only `Node1Chunks`/`Node2Chunks` (endpoint-instance chunk evidence; write paths don't persist them), and the agent tool's `relationsBackedBy` now prefers this endpoint evidence (companion change in ee72e4431 on the same branch).

Tests: doc1/doc2 dual-instance scenario asserting direction, single decode of one physical relation, merged same-name node, and endpoint evidence.

(Fix branch: fix/kg-3873-3945-3953, commit f892ee3be with companion ee72e4431; not yet merged upstream — verification welcome.)

### #3953 query_knowledge_graph returns relations without valid endpoint evidence in whole-KB queries（英文）

Root cause: `relationsBackedBy` only ran for document/tag scopes, so a whole-knowledge-base target skipped relation filtering entirely and relations could reach the Agent even when neither endpoint had any live, enabled evidence chunk.

What the fix does:
- Relation evidence validation now applies to all scopes including whole-KB; the validation budget is independent of the 10-chunk display budget (all nodes' / endpoints' chunks are checked for liveness), and validation runs before truncation so `relations_total` counts only valid relations.
- When `chunkRepo` is unavailable, relations are dropped with a warning while text results are returned unchanged.
- `relationsBackedBy` now prefers endpoint evidence (`Node1Chunks`/`Node2Chunks`, added by #3945) with the former name-aggregation as fallback; truncation visibility for the 30-relation cap is the same commit's #3873 fix.

Tests: `TestQueryKnowledgeGraph_WholeKBScopeDropsRelationsWithDeadEvidence`, `TestQueryKnowledgeGraph_RelationEvidenceBeyondDisplayBudget`, `TestQueryKnowledgeGraph_DocumentScopeUsesEndpointChunks` among 6 new cases. (Note: the tools package has 21 pre-existing failures on the HEAD baseline — sandbox/journal/browser/contract_snapshot domains — unrelated to and unchanged by this change.)

(Fix branch: fix/kg-3873-3945-3953, commit ee72e4431; not yet merged upstream — verification welcome.)

## fix/agent-param-3934-3935-3946

### #3934 Casting one Agent tool argument silently rounds unrelated numeric arguments（英文）

Root cause: `CastParams` unmarshalled the whole argument object into `map[string]interface{}` (numbers become float64) and re-marshalled the entire object whenever any field was converted, so untouched arguments got round-tripped through float64 and silently rounded.

What the fix does:
- `json.Decoder` now streams top-level entries in their original key order; only parameters whose type is declared in the schema are individually decoded, converted, and replaced; every other entry is emitted with its original bytes preserved.
- Collateral rounding is gone: `9007199254740993`, nested big integers, and high-precision decimals stay intact; `castValue`'s forced-cast semantics are unchanged.
- No-op and invalid inputs return byte-identical output.

Tests: four suites — sibling-parameter precision preserved, nested high-precision preserved, no-op byte-identical, invalid input returned verbatim.

(Fix branch: fix/agent-param-3934-3935-3946, commit 6696455d4; not yet merged upstream — verification welcome.)

### #3935 Agent tool string length validation counts UTF-8 bytes instead of code points（英文）

Root cause: `ValidateParams` implemented JSON Schema `minLength`/`maxLength` with Go's `len(s)`, which counts UTF-8 bytes, so valid Chinese/emoji inputs could be rejected while under-length inputs could pass.

What the fix does:
- Both bounds now use `utf8.RuneCountInString` (Unicode code points); error wording and ASCII behavior are unchanged.
- The same commit replaces enum comparison via `fmt.Sprintf` text matching with recursive structural equality (the #3946 fix).

Tests: 14 table-driven length cases plus 8 structural enum suites (four mismatch cases rejected, three legal matches retained).

(Fix branch: fix/agent-param-3934-3935-3946, commit 7ca8517eb; not yet merged upstream — verification welcome.)

Out-of-scope note: integers beyond 2^53 still compare under float64 numeric-equality semantics (consistent with the package's min/max handling); `json.Number` was deliberately not introduced.

### #3946 Agent enum validation accepts distinct JSON values with identical formatted text（英文）

Root cause: enum members were compared through `fmt.Sprintf("%v", ...)`, so distinct JSON values rendering to identical text — `["a","b"]` vs an enum containing `["a b"]`, `1` vs `"1"`, `{"a":"b","c":"d"}` vs `{"a":"b c:d"}` — passed validation.

What the fix does:
- Enum comparison switches to recursive structural equality: types are strict (number ≠ string ≠ bool), arrays are ordered, object key order is irrelevant; `1 == 1.0` remains a legal numeric match.
- `minLength`/`maxLength` switch to code-point counting in the same commit (the #3935 fix).

Tests: 8 structural enum suites (four mismatch groups rejected + three legal matches retained) plus 14 length table cases.

(Fix branch: fix/agent-param-3934-3935-3946, commit 7ca8517eb; not yet merged upstream — verification welcome.)

## fix/reparse-3851-qa-usage-3865

### #3851 Stale Snapshot Shadows Current KB Config（英文）

Root cause: the upload Confirm dialog POSTed the entire form state as `process_config` and it was persisted verbatim, so the KB config at upload time became a snapshot that shadowed later KB-level reconfiguration on every reparse entry point.

What the fix does:
- New `explicitProcessOverrides`: `ResolveProcessConfig` compares field by field and keeps only explicit deviations (parsing results stay equivalent), plus `mergeProcessOverrides`: request overrides win, unset fields keep the upload-time explicit items.
- Wired into all three reparse entry points (manual, batch, background retry) via the shared service path, so a KB config change now takes effect on reparse while explicit per-document overrides survive.
- Caveat: historical full snapshots on already-uploaded documents are kept as-is (explicit items can't be separated retroactively); the optional "reset config" UI control for reprocessing was not in this change's scope.

Tests: a) KB reconfig respected on reparse; b) upload-time explicit override not clobbered by new KB values; c) reparse request override precedence — plus 3 config-layer unit tests. (The service package has pre-existing failures identical to the merge/upstream-20261003 baseline, caused by a duplicate sqlite migration number, unrelated to this change.)

(Fix branch: fix/reparse-3851-qa-usage-3865, commit 4709f2887; not yet merged upstream — verification welcome.)

### #3865 快速问答 complete 事件未返回 usage（中文）

根因：快速问答（KnowledgeQA）的流式链路没有捕获/透传模型 usage 的通道，SSE 结束的 `complete` 事件不含 token 用量字段，而智能推理链路有既有通道正常返回。

修法要点：
- QA 两处流消费（主答案流与模型回退流）last-wins 捕获 usage，`AgentFinalAnswerData` 增 `Usage` 字段；
- 新增 `answerUsage` + `foldQuickAnswerUsage`（Accumulate 口径镜像智能推理 TurnUsage），persist 前折叠——持久化行、SP12 记账、`complete` 事件三处数字一致；`complete` 顶层 `usage` 与 `data.usage` 由既有 handleComplete 通用逻辑提升，handler 通用路径零改动；
- 后端不返回 usage 时字段缺省不报错（保持现状）。

测试：流带 usage → Done 帧与 complete 两处均 `{10,5,15}`；无 usage 缺省；跨流累计；service/handler 两域新增 7 用例全绿（handler 侧 complete 事件形状用例由补交 commit 4ac2d89a2 落地）。

票外注记：query 改写（非流式）的 usage 无既有通道，本轮未聚合。

（fix 分支：fix/reparse-3851-qa-usage-3865，commit c575e1473（测试补交 4ac2d89a2），尚未合回上游，欢迎验证）

## fix/misc-3974-3947-3962

### #3974 Go SDK ContinueStream drops SSE events with a space after the colon（英文）

Root cause: the event-field parser assigned `line[6:]` directly to `eventType` instead of stripping the single leading space the SSE spec allows after the colon, so `event: message` frames never matched the `message` check and were silently discarded (including terminal error frames).

What the fix does:
- The event field now strips exactly one leading space, mirroring the existing data-field handling; tabs, extra/trailing spaces, and other event names are preserved as-is.

Tests: a spaced `answer` frame reaches the callback and returns nil; a spaced terminal error frame is delivered and returned as `SSEStreamError`; a table-driven case locks down stripping exactly one space.

(Fix branch: fix/misc-3974-3947-3962, commit 7b4c9959b; not yet merged upstream — verification welcome.)

### #3947 Agent message sanitization drops images when merging consecutive user messages（英文）

Root cause: `messageHasContent` treated a message as empty based only on `Content` and `ToolCalls` (image-only or MultiContent-only messages were dropped), and merging adjacent user messages appended only text, losing the later message's images/content parts — so parallel tool-result images and compaction summaries adjacent to multimodal messages lost content.

What the fix does:
- `messageHasContent`: non-empty `Images`, or a non-empty `text`/`image_url` part inside `MultiContent`, counts as content.
- `mergeAdjacentMessages`: if either side is multimodal, the merge normalizes to a single `MultiContent` (order: MultiContent parts → legacy images → legacy text) with no legacy double-write; merged slices are freshly built so inputs stay immutable (ToolCalls are always copied, fixing a shared-underlying-array issue).

Tests: seven suites all covered — dual-image merge, summary + multimodal, pure MultiContent, mixed normalization, idempotence, input immutability, and OpenAI request JSON in both streaming and non-streaming forms. (#3574 is out of scope and untouched.)

(Fix branch: fix/misc-3974-3947-3962, commit 23aca1997; not yet merged upstream — verification welcome.)

### #3962 Literal tilde range markers render as GFM strikethrough（英文）

Root cause: `marked` enables GFM by default and its strikethrough rule accepts a single tilde, so CJK range markers like `2020~2035` / `50~60°` were parsed as `<del>` with the tildes consumed whenever a line contained two of them.

What the fix does:
- A shared `doubleTildeDelTokenizer` copies the default `del` rule minus the single-tilde branch and reuses the rule set via `this.rules` (zero drift), applied through an idempotent `applyMarkdownOptions` (WeakSet).
- Wired into the Vue chat renderer (`chatMarkdownRenderer.ts`) and `WikiBrowser.vue` `renderMarkdown`; the React side (embed and wiki markdown modules) was fixed in parity via `apps/web/src/shared/markdownOptions.ts`. `~~text~~` strikethrough still works; tables/breaks/GFM behavior unchanged.

Tests (Node 26): Vue 7/7 new + 41/41 existing; React 3/3 + embed 18/18 + wiki 20/20 + 63 adjacent regression cases all green.

(Fix branch: fix/misc-3974-3947-3962, commit b9c59afb7; not yet merged upstream — verification welcome.)

Out-of-scope note: `frontend/src/utils/documentPreviewMarkdown.ts` (a separate Marked instance) has the same single-tilde issue and was left for a separate decision.

## fix/misc2-3951-3940-3928

### #3928 Test Connection fails on reasoning models; 400-as-reachable fallback dead since #3470（英文）

Root cause: #3470 dropped go-openai, so 400 errors now read "API request failed with status 400" while the fallback matched go-openai's old "status code: 400" wording — the "400 = endpoint reachable, auth OK" branch could never hit, and reasoning models legitimately 400 on the `MaxTokens: 1` probe.

What the fix does:
- `isEndpointBadRequest` uses `errors.As(*api.HTTPError)` to read `StatusCode == 400` structurally, with a textual fallback matching both the old and new wordings.
- The minimal `MaxTokens: 1` probe and all other error paths are unchanged.

Tests: fake gateway returning 400 in both wordings through the real path; 401 and network-error counter-examples; 7-case table-driven suite.

(Fix branch: fix/misc2-3951-3940-3928, commit 09c632dbe; not yet merged upstream — verification welcome.)

### #3951 Markdown code fences contaminate heading-based chunk boundaries and context（英文）

Root cause: the chunker's heading scans toggled fenced-code state on any trimmed line starting with three backticks — tilde fences were ignored, and a shorter fence or a fence-like line with trailing text could prematurely close a longer fence — so literal `#`/`##`/`###` lines inside code polluted `MdHeadingTotal`, section boundaries, and `ContextHeader`/breadcrumbs.

What the fix does:
- New `fenceState`/`fenceRun` helpers implementing CommonMark §4.5: backtick and tilde fences, ≥3 same characters, ≤3 leading spaces, backtick info-strings may not contain backticks, closing fence uses the same character, is at least as long as the opener with only trailing whitespace allowed, and an unclosed run extends to EOF.
- All five scan points (`ProfileDocument`, `findHeadingBoundaries`, `observeSubHeadings`, `sectionBreadcrumbs`, `findHeuristicBoundaries`) reuse the same helper; the same-family defect in `heuristic_splitter` is fixed too; rune-position invariants are preserved.

Tests: +10 new cases (issue reproduction, four-backtick fence containing a shorter one, fake closing fence, unclosed-to-EOF, recovery after a real closing fence, POSTFENCE breadcrumb) plus an 11-case table-driven `fenceState` suite.

(Fix branch: fix/misc2-3951-3940-3928, commit bc5e5c77b; not yet merged upstream — verification welcome.)

### #3940 QA dataset script defaults to gpt-4o-2024-05-13, shutting down October 23（英文）

Thanks for the heads-up. The default model in the offline QA dataset script has been switched from the `gpt-4o-2024-05-13` snapshot to the `gpt-4o` alias, which OpenAI keeps routing to the current snapshot, so dataset generation won't start failing when the snapshot is retired on October 23, 2026. Callers that pass an explicit model are unaffected. No tests were added — it is a one-line default-value change in an offline tooling script.

(Fix branch: fix/misc2-3951-3940-3928, commit 748b7257f; not yet merged upstream — verification welcome.)

## fix/fake-success-3880-3872-3854

### #3880 MCP 连接测试 tools/list 失败仍返回 success=true（中文）

根因：`TestMCPService` 忽略 `ListTools` 的错误、只把空结果返回给上层，handler 依 `Success` 判定，于是清单读取失败被报成「连接成功、只是没有工具」，排查方向被带偏。

修法要点：
- `ListTools` 失败 → `Success=false`，`message=Connected but failed to list tools: <原始错误>`（复用 mcpTestFailure，含 OAuth 判定）；
- 成功路径（含空工具列表）与 `ListResources` 容错不变；未新增 `tools_error` 字段（备选产品形态，注明未做）。

测试：假 JSON-RPC 服务 `initialize` 正常 + `tools/list` 返回 `-32603` → `success=false`；正常路径 tools 非空。

（fix 分支：fix/fake-success-3880-3872-3854，commit 4231ffcf1，尚未合回上游，欢迎验证）

### #3872 RebuildLinks 写失败只记日志、接口固定返回 200（中文）

根因：`RebuildLinks` 逐页 `UpdateMeta` 失败只 `Warnf` 一条，循环外无条件返回 `nil`，handler 的 500 分支永远进不去——全部页面写失败也回 200「Links rebuilt successfully」。

修法要点：
- 失败保留 Warnf 并收集，循环外 `errors.Join` 聚合上报（对齐 wiki_ingest/knowledge_delete 的逐项聚合惯例），错误信息含页 slug；
- handler 既有 500 分支自然生效；响应形状不变；`wiki_lint` 调用方 `_ =` 行为不变。

测试：单页/双页失败 → 错误含 slug 且其余页照常落库；全成功 → 返回 nil 且 version 不增。

（fix 分支：fix/fake-success-3880-3872-3854，commit dd4aaa86b，尚未合回上游，欢迎验证）

### #3854 已终态知识行残留 running span 永久「进行中」（中文）

根因：失败路径把 `knowledges` 行置终态却不关闭本次 attempt 的 span，而 Sweep A 只扫 `pending/processing/finalizing` 行，终态行的孤儿 span 永远停在 running；且 reparse 时 `OpenAttempt` 直接 `NextAttempt` 插新 root，上一 attempt 的开放 span 不清理，新旧 root 并存。

修法要点：
- 新增 Sweep D `closeOrphanedSpansOnTerminalRows`：终态行 ∩ 开放 span 且超 10 分钟宽限（防 reparse 竞态误伤）→ 按行终态对齐关闭（failed/cancelled/done）+ 补 `finished_at`；批量 500 行/次滚动排干历史孤儿，无需数据库迁移；
- `OpenAttempt` 在插新 root 前调 `CancelOpenSpansBeforeAttempt`（attempt<N 的开放 span → cancelled + TASK_SUPERSEDED）；
- Sweep A 原语义零改动；#2619 中「解析完成但 span 不收尾」的子类由本路径自愈（wiki 入队路径根因仍建议在该票单独跟进）。

测试：三终态回收 / 非终态不误收 / attempt 跨界关闭三条 + 既有 35 项相关用例全绿。

（fix 分支：fix/fake-success-3880-3872-3854，commit 270fbda9a，尚未合回上游，欢迎验证）

## fix/misc3-3911-3918-3922-3926

### #3926 后台无法删除嵌入渠道（embed channel）产生的会话（中文）

根因：嵌入会话的 `user_id` 被写成 embed principal 的 StorageID（`embed_session:<tenantID>:<channelID>:<sessionID>`），而后台删除的 owner scope 只匹配 `管理员账号ID OR NULL OR ''`，两者永远对不上 → `ErrSessionNotFound` → 前端「删除失败，请稍后再试」。

修法要点：
- `sessionManagementScope`：Admin+ 传空（`applySessionUserScope` 直通，仅保 tenant 过滤），接入 Delete / Update / BatchDelete；handler 三入口换 `loadSessionForManagement`（owner 未命中且 Admin+ 时回落 `GetSessionByID`）；
- 权限边界：跨租户仍 404；普通用户语义不变（fail-closed）；运行时写路径（QA/stream/附件）未放宽；
- `ClearSessionMessages` / `DeleteAllSessions` 未动（票外，blast radius 大）。

测试：Admin 删/批删 embed 会话成功、普通用户 Not Found、批删只删自己的、跨租户 404、Viewer 对 mutation 404。

（fix 分支：fix/misc3-3911-3918-3922-3926，commit fe0077992，尚未合回上游，欢迎验证）

### #3922 Wiki「被链接」列表显示拼音 slug 而非中文标题（中文）

根因：`in_links`/`out_links` 只存 slug，前端 `slugDisplayName` 依赖按需分页的侧栏桶，未加载桶的 concept/entity 页回落拼音 slug（summary 恰好在已加载桶里所以显示正常）。

修法要点：
- `WikiPageDetailResponse` 新增 `link_titles`（slug→title 映射），用既有 `ListBySlugs` 一次批量解析；缺失/空 title 不入 map（前端回落 slug）；查库失败降级不 500；
- `GetPage` 与 `UpdatePage` 均携带；前端 `wikiLinkLabel` 纯函数 + 页脚消费。

测试：三类页中文 title / 缺失回落 slug / 查库降级 3 条 + `wikiLinkLabel` Node26 单测 2 条。

票外注记：React 侧 `WikiPage.tsx` 同病未动（域外）。

（fix 分支：fix/misc3-3911-3918-3922-3926，commit 5de9bcc6c，尚未合回上游，欢迎验证）

### #3918 对话总结意图仍触发知识库检索（中文）

根因：`QueryIntent.NeedsKBRetrieval` 把 `IntentSummarize` 列为需要检索，导致「总结一下我们刚才讨论的方案」这类请求在 summarize 意图下仍执行 chunk_search/retrieve/rerank，检索内容可能混入总结并引入无谓开销。

修法要点：
- `NeedsKBRetrieval` 移除 summarize，总结对话走免检索渲染路径（与 greeting/chitchat 同款）；
- 核查全部 10 处消费点：`intent_prompts.yaml` 预置 summarize 模板由死配置激活，无消费点依赖旧假设；提示词不动（summarize=总结对话本身、涉库内容归 kb_search 的语义本就正确）。

测试：全意图表驱动钉死——kb_search/clarification/空 → true；summarize/greeting/chitchat/follow_up/image_only/doc_only → false；web_search 按开关聚合。

（fix 分支：fix/misc3-3911-3918-3922-3926，commit fbe78f98c，尚未合回上游，欢迎验证）

### #3911 图片送 VLM 前不做 EXIF 方向归一化，横躺扫描件整页 OCR 错位（中文）

根因：VLM 链路把知识库图片**原样字节** base64 直发模型（`Predict` → `data:` URI），全链路无任何 EXIF 方向处理；浏览器读 Orientation tag 所以 UI 看起来正常，而视觉模型读像素矩阵、普遍不读 tag——Orientation=6 的扫描件到模型是躺的，表格二维推理成片错位，且错的位置每次不同（模型在猜归属）。

修法要点：
- 纯 stdlib 实现 `exif_orientation.go`：JPEG APP1 EXIF IFD0 `0x0112` 解析 + 8 种朝向前向像素映射 + quality95 重编码，零新依赖（go.mod 不动）；
- `orientationVLM` 装饰器注入于 `NewVLM` 装配点最内层（先于 debug/langfuse/concurrency，观测到的是实际发送字节），覆盖 `Predict`/`GetModelName`/`GetModelID` 与全部三个实现，8 个生产调用点零改动；
- 直通规则：非 JPEG / 无 EXIF / Orientation=1 或非法值 / 解码编码失败一律原字节直通，绝不阻断调用。

测试：非方形四象限 JPEG + 手工 APP1（LE/BE）朝向 2-8 全表 + 直通 11 例 + 装饰器 fake + httptest 端到端。

（fix 分支：fix/misc3-3911-3918-3922-3926，commit 42bd3d008，尚未合回上游，欢迎验证）

## fix/anydoc-ext-3932-sandbox-3942

### #3932 anydoc 引擎下 EMF/WMF 图片链接与 ImageRef 对不上、图片被静默丢弃（中文）

根因：Go 侧 `extensionFor` 调 `mime.ExtensionsByType`，结果随宿主环境 `/etc/mime.types` 漂移（Debian 装 media-types 时 `image/emf→.emf`），而 Rust 侧 `asset_links.rs` 是固定映射、兜底 `.bin`；markdown 链接与 `refMap` 键是**精确匹配**，差一个字符即静默丢弃，图片字节永远进不了存储。

修法要点：
- `extensionFor` 移到无 build tag 的 `asset_extensions.go`，逐项照抄 Rust `asset_links.rs:72` 的映射（去 TrimSpace 对齐 Rust 行为），兜底 `.bin`；`image/emf`、`image/wmf` → `.bin` 钉死，不再随系统漂移；
- 加一致性护栏：测试读取 Rust 源码正则解析 `match` 分支，与 Go 表双向比对（做过变异验证），另断言系统 mime 表行为（text/html 仍 .bin）。

票外注记：#3933（anydoc 缺 EMF/WMF 光栅化）属基础设施决策，本轮只钉映射、不引光栅化。

（fix 分支：fix/anydoc-ext-3932-sandbox-3942，commit fcd3fd789，尚未合回上游，欢迎验证）

### #3942 docker 沙箱空闲清扫器与会话首操作竞态，附件装载无声失败（中文）

根因定论：清扫器与绑定存储零协调，而会话首操作（附件装载）恰好依赖「下次使用时替换」语义——首操作 exec 正是刷新活动标记的那个 exec，它在途被 `docker rm -f` SIGKILL 时以非零退出 + **空 stderr** 返回，被包成 `invalid_request` 且不在可替换错误黑名单里，于是永不换绑，用户侧得到一条 0 步 0 字符的空助手消息。

修法要点：
- 诊断增强：`MakeDir`/`Remove`/`ListDir`/文件操作失败且 stderr 为空时显式输出 `exit=N stderr=<empty>`，不再冒号后截断成空；
- 方向 B 落地：`WriteSessionInputFile` 失败且 `CanReplaceRemoteBinding` → 重新 Resolve 换绑 → 重试一次（staging 幂等：`mkdir -p` + 覆写）；非可替换错误原样上抛；
- 未把 137/124 + 空 stderr 重分类为可替换（与真超时/OOM 不可分，防绑定 churn）；方向 A（清扫前租约核对）留产品裁决。

测试：exec 404 → 重绑 container-2 → 成功；权限拒绝不换绑；sandbox 包 8 条预存失败与基线逐条一致（非本改动引入）。

（fix 分支：fix/anydoc-ext-3932-sandbox-3942，commit 7be97254c，尚未合回上游，欢迎验证）

### #3910 API 调用报 sandbox MakeDir invalid_request、附件装载失败（中文，并票修复，同款回帖）

本票与 #3942 同一签名（会话首操作 `stage attachment` 撞上空闲清扫器删除容器，`docker MakeDir: invalid_request` 且 stderr 为空），已并票修复，回帖与 #3942 同款：

根因定论：清扫器与绑定存储零协调，首操作 exec 在途被 `docker rm -f` SIGKILL 时非零退出 + 空 stderr → 包成 `invalid_request` 且不在可替换黑名单，永不换绑。修法：失败空 stderr 时显式 `exit=N stderr=<empty>` 便于定位；`WriteSessionInputFile` 失败且可替换时重新换绑并重试一次（staging 幂等）。测试：exec 404 → 重绑新容器 → 装载成功；权限拒绝不换绑。

（fix 分支：fix/anydoc-ext-3932-sandbox-3942，commit 7be97254c，尚未合回上游，欢迎验证）

## fix/misc4-3941-3837-3878

### #3941 技能安装验证触发 exec 单参数上限，重型依赖技能无法安装（中文）

根因：验证阶段把 bundle 里**所有** `.py/.js/.sh` 文件逐个拼进一条 `/bin/sh -c` 命令且作为单个参数传递，自带依赖树（`.venv`/`node_modules`）的技能动辄数千文件——实测 4,862 个文件、约 260KB 参数串超过 Linux `MAX_ARG_STRLEN`（128KiB），`/bin/sh` exec exit 255，树校验/parse 校验/运行时前置校验三处全部命中。

修法要点：
- `sortedScriptPaths` 单点过滤 `.venv`/`node_modules` 前缀（安装产物依赖树而非模型可调脚本，其健康归依赖验证环节），三处验证调用同愈；
- 前缀匹配兼容 `/` 与 `\` 分隔。

测试：含 128KiB 参数串 size 断言 + 千文件级端到端安装用例。

（fix 分支：fix/misc4-3941-3837-3878，commit 0441f2022，尚未合回上游，欢迎验证）

### #3837 Agent/MCP 子 goroutine panic 杀死后端进程（中文）

根因：`mcp_exposure.go` 的 catalog 预热 goroutine 与最多 8 个 worker 均无 `recover()`，而 Go 的 `recover` 只对调用它的 goroutine 生效——`registry.go` 与 `act.go` 两个 recover 点覆盖不到工具自起的子 goroutine，MCP 客户端一 panic 整个进程退出，对话/上传/同步全部中断。

修法要点：
- `mcp_exposure`：预热 goroutine `defer close(preloadDone)` 挪到最外层 + recover（panic 路径先 recover 再 close，select 必返）；worker 用 `current` 追踪在途服务，recover 时锁内 `store(service, nil, "error")` 标记失败，不卡 loading；
- 同形态最小补：`search_knowledge` 两处 `HybridSearch` 子 goroutine 补 recover → `fail(errRetrievalPanicked)`（文案脱敏对齐 executeRecovered）；`think.go` 看门狗 recover → 日志降级；
- `executeRecovered`/`act.go` 主路径未动。

测试：5 条新用例含 `-race`；在未修 HEAD 上这些用例直接 panic 退栈（load-bearing 实证）。tools 包 21 条预存失败与基线一致。

（fix 分支：fix/misc4-3941-3837-3878，commit 5c3b6a124，尚未合回上游，欢迎验证）

### #3878 云解析超时硬编码，PaddleOCR-VL Cloud 与 WeKnoraCloud 无视配置（中文）

根因：两个云 reader 的整批轮询上限写死——`paddleocr_vl_cloud` 固定 600s、`weknoracloud` 固定 20 分钟，运维调大 `WEKNORA_DOCREADER_CALL_TIMEOUT`（默认 30 分钟）也无效，后者甚至比默认值还短，配置项永远轮不到生效。

修法要点：
- 两 reader 照 `mineru` 先例接 `requestTimeoutFromEnv`：`WEKNORA_PADDLEOCR_VL_CLOUD_TIMEOUT`（默认 600s）/ `WEKNORA_WEKNORACLOUD_TIMEOUT`（默认 20m），空/非法/非正数回退默认；超时文案带具体时长；
- 轮询间隔改结构体字段；`.env.example` 补两变量（WeKnoraCloud 项注明应小于 `WEKNORA_DOCREADER_CALL_TIMEOUT`）；生产行为不变（baseURL 字段仅测试注入用）。

测试：假服务端恒 running + 200ms/300ms 配置超时即返；env 回退路径；完成路径不回归。

（fix 分支：fix/misc4-3941-3837-3878，commit 7f07f6448，尚未合回上游，欢迎验证）

## fix/confluence-3856-embed-3898-docx-3849

### #3849 docx 页眉无法解析（anydoc / markitdown / docreader 均失败）（中文）

根因：内置 docx 链走 FirstParser（markitdown/python-docx），markitdown 只遍历 body、DocxParser 只走 paragraphs/tables——两条子路都不读 section headers/footers，页眉文本在任何一路都不落正文。

修法要点（当前修复覆盖内置 docx 解析链）：
- 新增 `extract_header_footer_lines`：多段落折叠、空块跳过、(类型,文本) 去重（linked-to-previous 继承只保留一次）；docx2_parser 链式解析成功后前置 `[Page Header]`/`[Page Footer]` 标记行，一次覆盖两条子路；
- 零新依赖（python-docx>=1.2.0 已在清单）；docx_merge、正文顺序与既有格式不动；提取失败仅告警不阻断。

测试：`docreader/tests/test_docx_headers.py` 7 例（多 section / linked 去重 / 空跳过 / 折叠 / 无页眉零回归 / 兜底路径），既有用例不回归。

票外注记：markitdown / anydoc 引擎自身不读页眉的缺口仍在（见文末待裁决）。

（fix 分支：fix/confluence-3856-embed-3898-docx-3849，commit 93206a66a，尚未合回上游，欢迎验证）

### #3898 嵌入网页报错 embed session signing key is not configured（中文）

根因：embed 会话签名键完全依赖 `SYSTEM_SIGNING_KEY` 显式配置，未配置时直接 503，嵌入功能无法开箱使用。

修法要点：
- 首启 bootstrap（migrations 之后、幂等）：完全未配置时自动生成 32 字节随机键存入 `system_settings`（IsSecret 行 `security.embed_signing_key`），回读经 env 导出（Info 注明 auto-provisioned、不打印键值）；显式 `SYSTEM_SIGNING_KEY`/AES 回退恒优先；坏行/DB 失败响亮失败，绝不静默轮换；与 desktop 先例同款模式；
- `system_settings` List 对未注册的 IsSecret 行打码 `***`，堵住管理 UI 泄密面；503 文案补 docker compose 配置指引；签名校验逻辑零改动。

测试：自动供给后预签名可用 / 二次启动稳定 / 显式配置优先 / DB 失败响亮 / 竞态回读收敛 + List 打码 + 503 安全网。多副本同毫秒首启的极端交错下仅首启内短暂分歧、重启收敛（已注释说明）。

（fix 分支：fix/confluence-3856-embed-3898-docx-3849，commit e300fa6d4，尚未合回上游，欢迎验证）

### #3856 Confluence 导入丢表格/图片/富文本/mermaid/drawio，子页链接打不开（中文）

根因：`markdownItem` 用 html-to-markdown 的包级 `ConvertString`，默认只启用 base + commonmark 插件——没有 `table` 插件（单元格被拍平）、没有 `strikethrough` 插件（删除线丢失）、`<svg>`（Confluence 把 drawio/mermaid 输出为内联 svg）被直接丢弃、相对链接/图片未绝对化（站内死链）。

修法要点：
- `markdownConverter` 一次性构建复用（v2.5.1 `WithPlugins` API），启用 table 与 strikethrough，表格结构与删除线保留；
- `replaceInlineSVGs`：内联 svg → `[diagram: name]` 占位（aria-label → 父容器 data-name → 序号），不再静默丢弃 drawio/mermaid；
- `absolutizeRefs`：`<a href>`/`<img src>` 相对值经 `resolveEndpoint` 绝对化（锚点/绝对/协议相对跳过），站内死链消除。

测试：6 例——表格/删除线/三种 svg 占位/链接绝对化/既有格式不回归。

票外注记：rss 与 ocr_sanitizer 仍用包级 `ConvertString` 同病未动；父页 subtree 缺失问题不在本次根因内（均见文末待裁决）。

（fix 分支：fix/confluence-3856-embed-3898-docx-3849，commit 826138fcf，尚未合回上游，欢迎验证）

## fix/fake-success2-3832-3833-3834-3835-3775

### #3832 Elasticsearch 批量写入丢弃 _bulk 逐条错误，文档全部被拒绝时 BatchSave 仍返回成功（中文）

根因：`_bulk` 响应 HTTP 200 不代表逐条成功——v8 不检查 `res.Errors`/`Items` 的逐条错误，v7 只计数不报错，全部条目被 ES 拒绝（如 mapping 解析失败）时 `BatchSave` 仍返回 nil，上层以为写入完成、不再重试。

修法要点：
- v8：接收响应检查 `res.Errors`，`collectBulkItemErrors` 遍历 `Items` 收集失败计数与首例明细（op/id/status/type/reason）；`errors:true` 但零明细同样返回错误（不变式：逐条失败绝不 nil）；
- v7：`countBulkErrors` 升级为返回 `(count, first)`，`errors:true` 且确有失败从 Warn 升级为返回错误，口径与 v8 一致。

测试：v7/v8 两包 fake bulk 响应（200 + errors:true + item error）→ 返回错误含失败计数与 `mapper_parsing_exception` 样本；全成功路径不回归。

（fix 分支：fix/fake-success2-3832-3833-3834-3835-3775，commit c7847d11f，尚未合回上游，欢迎验证）

### #3833 SQLite 后端 vec0/FTS5 索引行写入错误被丢弃，chunk 行已提交而索引行缺失（中文）

根因：sqlite 后端 `insertVec`/`syncFTS5Insert`/`copyVec` 的错误被丢弃、事务照常提交——chunk 主行落库而 vec0/FTS5 索引行缺失，`BatchSave` 仍返回 nil，这些 chunk 从此检索不到。

修法要点：
- 三处写索引函数改返回 error 并穿透 `BatchSave`（事务回滚）与 `CopyIndices`；`copyVec` 遇 vec 表缺失由静默跳过改报错；
- FTS5 构建分治：无模块时 `initFTS5` 就绪态+跳过（tagless 门绿），`-tags sqlite_fts5` 下错误全量穿透（26 项 PASS）；
- 同 commit 一并修复 #3775 的批量更新吞错（见下票）。

测试：+11（失败注入 4 + 对齐上游 PR #3768 口径 7）。

票外注记：票面所引 knowledge_faq 顺序问题，fork 侧已在 `return err` 之后（票面为上游旧码），零改动；本轮审计另发现 faq/knowledge_faq.go :447/:610 两处 `IndexFAQChunks` 空体吞错与 `deleteRowsAndVecs` 四处 Exec 吞错，留裁决（见文末）。

（fix 分支：fix/fake-success2-3832-3833-3834-3835-3775，commit b39b6edd9，尚未合回上游，欢迎验证）

### #3775 SQLite 批量更新吞掉 gorm 错误永远返回 nil，标签/启用状态同步静默失败（中文）

根因：`BatchUpdateChunkEnabledStatus`/`BatchUpdateChunkTagID` 走 `updateChunkIndexColumn` 时不接收 gorm `result.Error`，任何失败被吞、恒返回 nil，标签/启用状态批量同步静默失效。

修法要点：
- `updateChunkIndexColumn` 返回 `result.Error` 并由两个批量入口透传，口径对齐 postgres（0 行受影响不算错、仅 Warn）；
- `errors.Join` 逐条累计错误并跑完整批，不因首错中断；
- 同 commit 一并修复 #3833 的索引行写入吞错（见上票）。

测试：+11（与 #3833 同套：失败注入 4 + PR #3768 口径 7）。

（fix 分支：fix/fake-success2-3832-3833-3834-3835-3775，commit b39b6edd9，尚未合回上游，欢迎验证）

### #3835 Qdrant/Milvus 关键词检索整批失败被当成「没有匹配」（中文）

根因：`KeywordsRetrieve` 对逐 collection 的失败只 Warn 不聚合，全部 collection 失败时仍返回空结果 + nil error——上层把「检索后端全挂」当成「没有匹配」继续生成回答。

修法要点：
- qdrant/milvus 逐 collection 失败计数 + 首错样本：全部失败 → 返回错误（含后端/collection 数/首错），上层走真实失败路径；部分失败 → Warn 记比例、结果照常；
- 错误语义对齐同文件 `VectorRetrieve`；milvus 用 `recordFailure` 覆盖 5 个失败分支；
- 顺带治愈 milvus 预存红 `TestConvertResultSetFloatVector`（`allFields` 补 `fieldLanguage`，与 copyindices 分支同款修复）。

测试：两后端各 4 例（全败/一成一败/全成功/零匹配 collection）；milvus fake 需真 gRPC server（WithBlock 拨号）+ 链式拦截器。

（fix 分支：fix/fake-success2-3832-3833-3834-3835-3775，commit ff0ad2362，尚未合回上游，欢迎验证）

### #3834 图片分块索引写入被静默吞掉，处理轨迹仍标 indexed: true（中文）

根因：图片 OCR/Caption 分块入库后的 `indexChunks` 六处失败分支（KB/嵌入模型/租户/引擎/批索引）全部吞错返回，`Handle` 无条件写 `indexed=true`，失败被报成成功且无重试。

修法要点：
- `indexChunks` 改返回 error，六处失败分支上抛（含阶段与 chunk 数）；`Handle` 按 `CreateChunks` 同款 handleErr 返回 → asynq 重试；`indexed=true` 仅成功后写；
- 失败不清理 chunks：与同文件 `CreateChunks` 失败形态一致（knowledge_process 的删除是知识库级，单图任务执行会误删兄弟图片分块）。

测试：5 例（批索引失败/KB 失败/模型失败/成功/跳过路径）；service 基线 338/90 一致。

票外注记：重试各次会留新 ID 未索引行（预存数据卫生缺口）。

（fix 分支：fix/fake-success2-3832-3833-3834-3835-3775，commit d21dd9731，尚未合回上游，欢迎验证）

## fix/sync-integrity-3778-3692-3690-3774

### #3774 FAQ JSON 导出每条 id 恒为 0，导出→编辑→再导入会更换 seq_id（中文）

根因：`ListAllFAQChunksForExport` 的 Select 列漏了 `seq_id`，导出条目 id 恒为 0；而导入端只在 id>0 时恢复原 seq_id，导出→编辑→再导入的 id 映射闭环断裂、seq_id 被换新。

修法要点：
- Select 补 `seq_id`，导出→编辑→再导入的 seq_id 恢复闭环恢复；
- CSV 字节不受影响（8 列无 id）；
- 修法参考上游 PR #3762。

测试：投影回归——导出 SeqID 与库内一致。

（fix 分支：fix/sync-integrity-3778-3692-3690-3774，commit 23b11cd56，尚未合回上游，欢迎验证）

### #3778 Notion 单页超 1000 block 静默截断，内容与子页面缺失且无提示（中文）

根因：`GetBlockChildrenAll` 的翻页有 1000 块上限，触及 cap 直接停止——无 Warn、无标记，页面尾部内容与深层 child_page 整段静默缺失。

修法要点：
- `blocksTruncated` helper：仅在 ≥1000 且确有下一页时 Warn（措辞对齐飞书同款 cap），恰好等于上限不误报；cap 与递归语义不变（只做截断可见化，不改变抓取行为）；
- 同 commit 另修复取块失败推进游标的问题（#3692，见下票）。

测试：截断 5+2 例（含恰好 1000 不告警）；401 失败 → 游标 nil；恢复重跑双页更新；稳态 0 变更。

（fix 分支：fix/sync-integrity-3778-3692-3690-3774，commit 935ef6be7，尚未合回上游，欢迎验证）

### #3692 Notion block-fetch failures advance the cursor and skip retries（英文）

Root cause: `fetchPage` treated block-fetch failures as warnings and returned no items, yet `FetchIncremental` still derived the cursor from the recorded `last_edited_time` — the failed page's new version was acknowledged and never retried until the next source edit.

What the fix does:
- `fetchPage` now returns an error: failures fetching a page's own blocks, non-404 `child_page` traversal failures, and recursive failures all propagate; `FetchIncremental` aborts the whole round on the first changed-page failure — no items produced, no cursor returned — so the task keeps the old cursor and retries, and after recovery both pages are re-evaluated;
- `child_page` 404 stays warn+skip (deletion window, avoids permanent failure); `FetchAll` gets the same don't-acknowledge-on-failure semantics;
- the same commit makes the 1000-block cap emit a warning when content is actually truncated (the #3778 fix).

Tests: a 401 failure → error with nil cursor; with the old cursor retained, recovery replays and picks up the new content across both pages; steady state reports 0 changes; truncation warning cases include exactly-1000 not being false-flagged.

(Fix branch: fix/sync-integrity-3778-3692-3690-3774, commit 935ef6be7; not yet merged upstream — verification welcome.)

### #3690 Yuque partial sync acknowledges failed document versions and skips recovery（英文）

Root cause: on incremental walks, a document-detail failure (client retries exhausted) was only logged while the walk continued, and the returned cursor still recorded that document's `content_updated_at` — the failed version was acknowledged, so the next incremental sync skipped it until another source edit.

What the fix does:
- A detail failure in the incremental walk now fails the whole round and returns a nil cursor, so the task keeps the old cursor and retries; full `FetchAll` keeps the placeholder-continue behavior (no cursor to pollute, pinned by existing tests);
- Rationale: the service layer persists a non-nil cursor even when a fetch error dropped items — a partial cursor would silently acknowledge successful documents while losing their content — so nil is required; this matches the whole-round error semantics of the Notion #3692 fix.

Tests: one-succeeds-one-fails → error + nil cursor with retries-exhausted evidence; with the old cursor retained, recovery replays and picks up the new content; all-details-fail → error; all-succeed is guarded by existing tests.

(Fix branch: fix/sync-integrity-3778-3692-3690-3774, commit 753e8a025; not yet merged upstream — verification welcome.)

## fix/truncate-3836-summary-3776-rss-3823

### #3836 Notion 查询撞 10,000 行上限静默截断，截断结果还被当作删除判定基线（中文）

根因：Notion 在查询结果超限（如 10,000 行）时返回 `request_status=incomplete` 而非报错，代码不解析该字段——截断的部分行集被当成完整结果，增量删除判定把「被截断没返回的行」当作源端已删除（#3682 修的是请求失败，本票是请求成功但被厂商截断）。

修法要点：
- `paginatedResponse` 增 `RequestStatus` 解析；`paginatePages` 每页检查 `type=incomplete`（search/query 共用，覆盖早页信号）→ 返回哨兵 `ErrQueryTruncated`（含 incomplete_reason/端点/收窄指引），绝不返回部分行集；
- `fetchDatabaseIncremental` 传播哨兵；`FetchIncremental` 收到即整轮报错 + nil 游标——删除判定循环不可达，被截断的行不再被当已删；
- `fetchDatabase` 全量路径仅 Warn 可见化（无删除基线，残留见文末待裁决）。

测试：5 例含端到端（r1..r4 源只回 r1/r2 + incomplete → 报错、零条目、无 IsDeleted、游标 nil；恢复后重跑全量）；变异验证过。

（fix 分支：fix/truncate-3836-summary-3776-rss-3823，commit 8e044669c，尚未合回上游，欢迎验证）

### #3776 摘要坏 JSON 原文当摘要落库进 RAG，且标 completed 不再重试（中文）

根因：要求 JSON 输出的摘要模板，模型返回裸文本/半截 JSON/LLM 截断（FinishReason=length）均无校验，坏 JSON 原文被当摘要落库、索引进 RAG，并标 completed 终止重试。

修法要点：
- `summaryPromptRequiresJSON`：仅 GenerateSummary 列表 default:true 条目（严格 JSON 契约）要求解析成功；自定义/不可解析模板保留纯文本兜底（既有刻意设计）；
- requireJSON 下解析失败或无可用文本 → `errInvalidSummaryOutput`；`validateSummaryOutput` 增 `FinishReason=length` 检查 → `errSummaryOutputTruncated`；两者走既有可重试路径（Pending→重试→耗尽首块兜底+Failed），不落库原文、不标 Completed、不进 RAG。

测试：模板判定 5 例 / 契约 5 例（裸文本拒/半截拒/length 拒/自定义保留/正常不变）/ 端到端不落原文；service 基线 338/90 一致（TestCraftLifecycleGuardSummary 预存红经基线复核）。

（fix 分支：fix/truncate-3836-summary-3776-rss-3823，commit 478c8659e，尚未合回上游，欢迎验证）

### #3823 RSS title and source link changes are lost when article text is unchanged（英文）

Root cause: the RSS change fingerprint hashed only the entry's markdown body, so an entry with a stable GUID and unchanged body but a changed title or source link produced zero updates — the stale title/link persisted in the knowledge document while the new feed signal was already recorded, keeping subsequent syncs skipping the item.

What the fix does:
- `itemFingerprint(title, link, markdown)` — a three-part fingerprint replacing the body-only hash: title-only and link-only changes each produce one update, the new title/URL/metadata.link get persisted, and body bytes stay untouched (downstream content hash and chunking unchanged);
- unchanged entries still produce 0 updates; summary-only and pubDate-only changes still produce 0 updates by design (the fingerprint deliberately excludes updatedAt — flagged in the pending-ruling list).

Tests: 5 cases plus a fingerprint unit test, including a mutation check that restores the old body-only fingerprint and reproduces the 0-update defect.

(Fix branch: fix/truncate-3836-summary-3776-rss-3823, commit 686a4dac7; not yet merged upstream — verification welcome.)

## fix/wiki-guard-3792-3714-rrf-3796

### #3792 wiki 唯一写出口 UpdatePage 无行丢弃保护，长表格页面被机器写入静默改短（中文）

根因：`UpdatePage` 是 wiki 页面唯一写出口，调用方内容原样落库——不比较新旧、没有「本次写入丢行/变短」的判断，六条机器写入路径（含 agent 整页重写）的截短写入全部被静默接受。

修法要点：
- 复用既有 `types.WikiEditSource` 上下文（零签名变更），六条写入方接线真实身份；
- 出口新增 `guardRowDrop`：比较新旧内容「每行第一个非空单元格」身份集合，非人类写入丢旧行即拒绝（`ErrWikiRowDropRejected`，含 slug/丢行数/长度）；user/revert 天然豁免，retract 与索引页重建以包内 shrink-allowed 标注豁免；行重排/加粗/改列值照常通过；
- 同 commit 另修复 ingest 源查找错误被当已删除的问题（即 #3714 的修法，见下票）。

测试：同 commit 新增 16 例（守卫 9 / 查找 5 / 工具 2）；wiki 包 3 条预存失败与基线一致。

（fix 分支：fix/wiki-guard-3792-3714-rrf-3796，commit 11176276b，尚未合回上游，欢迎验证）

### #3714 Wiki ingestion drops pending work when source-document lookups fail（英文）

Root cause: ingestion treated any source-document lookup error (context canceled, deadline exceeded, database error) as proof that the document was deleted, so Map/Reduce reported a successful skip and durable pending work was retired.

What the fix does:
- `isKnowledgeGone` is now three-valued: only `ErrKnowledgeNotFound` plus the nil/deleting/cancelled states count as "gone"; every other lookup error is an error;
- Map: a lookup failure fails the span and records failedOps, keeping the durable pending work intact; Reduce: `filterLiveUpdates` propagates the error into the existing requeue path instead of pretending a successful no-op.

Tests: 16 new cases in the same commit (9 row-drop guard — the #3792 fix above — plus 5 lookup-state and 2 tool-side cases); the wiki package's 3 pre-existing failures match the baseline.

(Fix branch: fix/wiki-guard-3792-3714-rrf-3796, commit 11176276b; not yet merged upstream — verification welcome.)

### #3796 RRF mixing a FAQ knowledge base with document knowledge bases hides the best document chunk（英文）

Note first: the main defect reported here — ranks taken from the chunk's position in the concatenated result slices — is already fixed upstream by #3687, which is present in this tree (`bestRanks` by score). This change fixes the determinism defect that still remained after that fix.

Root cause (the surviving residual): rank assignment itself was not deterministic — it depended on goroutine completion order and map iteration randomness, so same-score chunks drifted between runs, and `deduplicateByScore`/`fuseWithRRF` assembly plus the `MatchCount` truncation could differ run to run on identical inputs.

What the fix does:
- Every retriever's slices are sorted with `SortStableFunc` (stable descending) before ranks are assigned; `deduplicateByScore` and `fuseWithRRF` assemble by first-seen order — goroutine completion order and map iteration order no longer affect ranks or truncation;
- The RRF formula, k=60, the 0.7/0.3 weights, and truncation are untouched; single-KB behavior is equivalent.

Tests: 4 cases — a FAQ KB returning a low score first while the document's 0.96 chunk still fuses to the top (`HybridRanksPerListNotListPosition`); disordered same-score inputs re-fused 32 times with identical results (`SameInputFusesIdentically`); single-KB expectations pinned exactly; keyword dual-list ranking. Pre-existing service failures match the baseline.

(Fix branch: fix/wiki-guard-3792-3714-rrf-3796, commit 7d091bfbb; not yet merged upstream — verification welcome.)

## fix/final-3828-3811-3806

### #3828 新建 MCP 端点的知识库下拉框不显示共享知识库（中文）

根因：MCP 端点新建/编辑的知识库下拉只请求本空间库列表（GET /knowledge-bases），漏合并组织共享库来源（GET /shared-knowledge-bases，知识库列表页同源接口已在用），B 空间用户在下拉里永远看不到共享库。

修法要点：
- `loadOptions` 并行取两源；新增 `mergeKnowledgeBaseOptions` 纯函数去重收敛：owned 胜出、多组织取最高权限、可编辑先于仅查看稳定排序；
- 共享条目沿用列表页 t-select 分组与既有 i18n（5 语言零改动）；`kbNameById` 覆盖共享库，修正编辑回显；无共享库时下拉与现状一致；
- React 侧无对应页面（McpSettingsPanel 是客户端配置），不涉及。

测试：Node26 新增 13/13 + 相邻 27/27 全绿；vue-tsc 干净。

票外注记：API-key/IM 向导的知识库选择同形问题（同样只拉本空间库）属另票未动（见文末待裁决）。

（fix 分支：fix/final-3828-3811-3806，commit 6a4b22a18，尚未合回上游，欢迎验证）

### #3811 data analysis column reconciliation rewrites SQL string literals（英文）

Root cause: `reconcileSQLColumnsWithSchema` reconciled column names through a whole-string regex, which cannot tell a double-quoted identifier from double-quoted text inside a SQL string literal — so literals like `"orderstatus"` were rewritten as well, silently changing which rows the query matched.

What the fix does:
- The regex is replaced by a literal-aware four-state byte scan: single-quoted literals (including `''` doubling and escapes) and dollar-quoted literals (`$$…$$` / `$tag$…$tag$`, differently-tagged nesting handled, `$1`-style parameters never mistaken for delimiters) are copied through verbatim; only double-quoted identifiers outside literals are reconciled;
- Standard-compliance bonuses in the same pass: `""` escapes parsed inside identifiers, canonical names written back quoted-and-re-escaped; DuckDB's default backslash-is-not-an-escape behavior is conservatively tracked.

Tests: the issue's reproduction CSV end-to-end now returns `paid` (the literal untouched); 15 table-driven cases; the old regex's mis-rewrite behavior was confirmed by a probe before deletion. The tools package's 21 pre-existing failures match the baseline.

(Fix branch: fix/final-3828-3811-3806, commit 9937a2fe6; not yet merged upstream — verification welcome.)

### #3806 BrowserSkill 跨轮会话复用失效：每轮扩展侧报 session not_found（中文）

根因（两个独立来源叠加）：
1. 代码级：无血缘合并把上游 09-23 会话保活修复（#3563 + PR296）困在 `internal/browserskill/` 死副本（全仓无 import 者），而实际接线的 `internal/modules/execution/browserskill/` 仍是 09-21 修复前快照——轮末还在发已退役的 `task_idle`，扩展回 `unknown_method`，keepOpen 路径报错，每轮丢会话（7/7 系统性）；另带两条已废弃的 `task_preview`/`task_focus` 通道；
2. 配置级：`--session-idle 30m` 使跨轮间隙超过 30 分钟的 retained 会话被原生 GC，是丢会话的另一独立来源。

修法要点：
- 把上游修复移植进 live 包（focus/manager/cluster/authorization 及测试，+475/-127）：轮末 retained 仅设 host-side idle 标记、零 wire 动作；preview/focus 改走官方 `ui.*` 通道；补 preserveAgentWindow 与扩展版本上报；
- 保留 fork 侧 daemon 加固；`--session-idle` 属部署配置项，取值调整不在代码修复范围内。

测试：browserskill 域 19 例全绿。

票外注记：扩展端注销点缺 reason 日志（扩展源码不在本仓库，已记录建议）；`internal/browserskill/` 死副本删除属破坏性操作留裁决（均见文末待裁决）。

（fix 分支：fix/final-3828-3811-3806，commit 8c4bb008d，尚未合回上游，欢迎验证）

## 无票产品缺陷修复（合并伤亡战役中发现的真缺陷，无对应 GH 票——可作上游 PR 或开票材料）

以下逐项从各分支 commit message 提取（git log 核对，分支 + commit 短 hash + 根因 + 影响面）。前 4 项无对应 GH 票，不进回帖清单；第 5 项有票（存照防重复开票）。

### fix/final-3828-3811-3806 · ebe41d6a6——提示词 section 实现被合并截断（上游 #3475/#3549）

- 根因：无血缘合并取了上游 #3475（a3c7be10c）/#3549（22cbcdfa5）的**测试**却保留 fork 截断实现——`BuildSystemPromptSections` 缺 5 个 section 追加（sources/tools/output/skills/memory/protocol）与 host-workspace 分支、`!skillInstallMode` 守卫；`engine.buildSystemPrompt` 丢 persona 前置（trpc agent_capabilities.go:63 注释期望同款）。
- 影响面：生产提示词组装路径缺上述 5 个 section 与 persona 前置；按上游原文恢复实现，测试重钉合并后文案与现代工具名（ToolSearchKnowledge 等，list_knowledge_chunks 系废弃别名）；agent 包 17 个预存红用例全治愈、包首次全绿。
- 注：round20 browserskill import 切换实验与本项无关（失败集切换前后相同，系日志非 UTF-8 致 grep 二进制误读）；const.go 依赖死副本仍留裁决。

### fix/migrations-dedup-preexisting · 001ac4b01——completeAssistantMessage 吞错 + CreateForked 拼接坏

- handler/session qa.go `completeAssistantMessage`：丢弃 `UpdateMessage` 错误（`_ =`），**保存失败仍发完成事件**（publishCompletion）——commit 原文明示「真产品缺陷修复」；恢复上游错误传播（5→0）。
- repository session.go `CreateForked`：合并拼接出**空 if 块吞错** + 第二次无 Omit 的 Create 触 UNIQUE sessions.id（8→0）；按两父本并集复原（fork fd2b5e809 `Omit("share_token")` + 上游 bccb4b151 insertMessageArtifacts）。
- 同 commit 第三簇为测试面（memory persistence_message_paging_test 补 message_artifacts DDL，loader 全走 attachArtifacts，包全绿）。

### fix/migrations-dedup-preexisting · e62e1317b——GetSessionArtifactRefs 读已清空 legacy 列

- 根因：messages.artifacts 列已被迁移 000023（镜像上游 000103）迁入 message_artifacts 表并置 NULL，该读仍 Select legacy 列。
- 影响面：**生产面 4 处制品引用恒空返（静默）**——workbench artifacts 列表 / 签名链接 / artifact_download 验证 / annotation 版本解析。
- 修法：重写为镜像 `GetSessionArtifacts` 的表查询（JOIN 未删 messages、含 tombstone、created_at/id/position 稳定序，Index=position 对齐扁平列表）；测试 seed 补写 NewMessageArtifactRecords 行；repository 包 0 失败（战役起点 483→0）。

### fix/migrations-dedup-preexisting · 6b0545efb——共享运行未强制 MCP 隔离（安全语义）+ 推理力度接线丢失

- session_agent_qa.go：恢复上游 hunk——`SharedAgentReadOnly` 且 MCPSelectionMode 空时**强制 none**；共享运行不得暴露 owner 的 MCP 服务（安全语义，合并 e8677d4ea 丢弃，父本2 bccb4b151:359 有证）。
- `buildAgentConfig`：恢复 `applyRequestReasoningEffort` 调用（同族丢 hunk，TestRequestReasoningEffortPrecedence 治愈）。
- 影响面：service 全量套件恢复可跑完（372s）、6 条治愈；同 commit 第三处为测试面（agent_run_graph_test.go 替身补 GetLatestContextCheckpoint/ListMessagesBySessionBeforeCursor 两 stub——旧契约 nil 提升曾 panic 杀死测试二进制、掩蔽尾部 MCPDiscovery/BudgetExhaustion/AdmitsAfterFollowUp 等 4 条用例）。

### fix/anydoc-ext-3932-sandbox-3942 · 7be97254c——沙箱清扫竞态换绑重试 + 诊断（注：本项有票）

- 本项**非无票**：该 commit 即 #3942 修复、同 commit 并票 #3910（草帖见上文两票），列此存照防重复开票。
- 根因定论：清扫器与绑定存储零协调，首操作 exec（恰是刷新活动标记的 exec）在途被 `docker rm -f` SIGKILL 时非零退出 + 空 stderr → 包成 `invalid_request` 且不在可替换黑名单，永不换绑（②就绪窗口已由 ensureRunning 闭合）。
- 修法：诊断增强——`MakeDir`/`Remove`/`ListDir`/文件操作失败且 stderr 为空时显式 `exit=N stderr=<empty>`，不再冒号后截断成空；方向 B——`WriteSessionInputFile` 失败且 `CanReplaceRemoteBinding` → 重 Resolve 换绑 → 重试一次（staging 幂等：`mkdir -p` + 覆写）；非可替换错误原样上抛；未把 137/124+空 stderr 重分类为可替换（方向 A 留裁决）。

---

## 待所有者裁决

以下事项本轮明确未做/留白，回帖不代为承诺，需所有者拍板后再跟进：

1. **#3895（微信图片）**：需环境复现，本轮未修、未出草帖。
2. **#3933（anydoc EMF/WMF 光栅化设施）**：#3932 只钉死扩展名映射（emf/wmf→.bin，矢量图原样落盘），是否引入光栅化设施属基础设施决策（fcd3fd789 注记）。
3. **沙箱清扫方向 A（#3942/#3910 内）**：清扫器与绑定存储的协调（清扫前租约核对）未做；137/124+空 stderr 重分类为可替换错误未做（与真超时/OOM 不可分，防绑定 churn）（7be97254c 注记）。
4. **#3867（问句残留清理）**：清理属破坏性操作，未动、未出草帖。
5. **Vue `documentPreviewMarkdown` 与 React `WikiPage` 单波浪同病**：#3962 修复了 Vue 两个渲染点与 React parity 三处；Vue `documentPreviewMarkdown.ts`（独立 Marked 实例）与 React `WikiPage` 仍存单波浪删线同病，去留待裁决（b9c59afb7 注记）。
6. **rss 与 ocr_sanitizer 的 `ConvertString` 同病**：#3856 只改 confluence `markdownItem`；rss 连接器与 ocr_sanitizer 仍用包级 `ConvertString`（无 table/strikethrough 插件），同病未动（826138fcf 注记）。
7. **anydoc / markitdown 引擎侧 docx 页眉**：#3849 修的是内置 docx 解析链；anydoc 与 markitdown 引擎自身不读 section headers/footers 的缺口未动（93206a66a 注记）。
8. **faq/knowledge_faq.go :447/:610 两处 `IndexFAQChunks` 空体吞错（round14 新发现）**：#3833/#3775 修复时审计出的预存残缺，两处错误吞空的 if 体（连同 `deleteRowsAndVecs` 四处 Exec 吞错同批发现）未修、未出草帖（b39b6edd9 注记）。
9. **notion `fetchDatabase` 全量路径截断仅 Warn**：#3836 只把增量路径（fetchDatabaseIncremental → FetchIncremental）升级为哨兵报错 + nil 游标，全量路径保留 Warn 可见化（无删除基线，风险低），是否补齐报错语义待裁决（8e044669c 注记）。
10. **rss summary-only/pubDate 变更仍 0 update**：#3823 指纹刻意不含 updatedAt，summary-only/pubDate 变更不产出 update 属设计内行为，是否纳入指纹待所有者拍板（686a4dac7 注记）。
11. **`internal/browserskill/` 死副本删除**：#3806 把上游修复移植进实际接线的 live 包后，该目录与 live 包内容等价且全仓无 import 者；删除属破坏性操作，去留待裁决（8c4bb008d 注记）。
12. **React API-key/IM 向导知识库选择只拉本空间库（#3828 邻近同形，另票）**：React Integrations 页 IM/API-key 向导的 `loadKnowledgeBases()` 仅取 `client.knowledgeBases.list()`（本空间库），未合并共享库列表；与 #3828 同形但属另一张票，本轮未动（6a4b22a18 注记）。
13. **#3806 扩展端注销点 reason 日志建议**：跨轮会话被扩展注销时无 reason 日志，定位存在盲区；扩展源码不在本仓库，需在扩展仓跟进（8c4bb008d 注记）。

### 备查：commit 内注记的其他票外残差（非本清单，一并供裁决）

- #3877：notion `getBlockChildrenRecursive` 仍有同形裸游标循环（8fb7d1f72）。
- #3922：React `WikiPage.tsx` 的 link_titles 同病未动（5de9bcc6c）。
- #3926：`ClearSessionMessages`/`DeleteAllSessions` 未放宽（fe0077992）。
- #3935/#3946：>2^53 整数按 float64 数值相等语义（未引 json.Number）（7ca8517eb）。
- #3851：reparse 的 reset 配置 UI 选项未做（4709f2887）；存量全量快照原样保留。
- #3865：query 改写（非流式）usage 无既有通道未聚合（c575e1473）。
- #3854：#2619 的 wiki 入队路径根因仍建议该票单独跟进（270fbda9a）。
- #3880：未新增 `tools_error` 字段（产品备选注明）（4231ffcf1）。
- #3947：#3574 票外未动（23aca1997）。
- #3856：父页 subtree 缺失不在本次根因（826138fcf）。
- #3834：重试各次会留新 ID 未索引行（预存数据卫生缺口）（d21dd9731）。
- #3833：票面所引 knowledge_faq 审计顺序 fork 侧已在 return err 之后，零改动（b39b6edd9）。
