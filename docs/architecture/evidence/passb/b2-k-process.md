# Evidence — b2-k-process（K4 Knowledge 处理流水线域）

- 节点：`b2-k-process`（Pass B B2，work 节点，plan `docs/plans/passb/24-knowledge-process.md`）
- 分支：`codex/passb-b2-k-process`；`PRE_MERGE_SHA`（= 派发 BASE/`PASSB_BASE_SHA`）`b650e2040`；任务链 commit：`41cc45071`/`6bb284e4a`/`5fbd4d789`（P2 基线对齐三 merge）→ `a57c43a6f`（K4.0-R replan）→ `5c3131e60`（K4.0-R ledger 对齐）→ `3665fff5c`（K4.1）→ `ff5ae6d21`（K4.2）→ `fbd40129b`（K4.2-R）→ `34418f700`（K4.3）→ `60ef71061`（K4.4）→ K4.5（本文件所在 commit）
- 证据日期：2026-09-26；执行者：实施-b2-k-process-K4.5（子 Agent，未派生下级）
- 前置证据：K4.0–K4.4 任务报告与审查包（`.superpowers/sdd/passb/b2-k-process/K4.{0,1,2,3,4}-report.md`，git-ignored 区，命令台账原文在册）

## 1. 高风险差分（conventions §6 / 计划 §8.3 / spec §14.3；K4.5 Step 1）

**口径（按实际收缩执行形态如实登记）**：计划 §8.3 四面的锚定用例在 Ruling 2026-09-25-DEFERRED-FILE-SPLIT 收缩后的分布为三类——(i) 已随迁子集（T0 宿主包 → T1 落位包双跑）；(ii) 成对推迟件（生产+测试同留守宿主，T0/T1 同为宿主包双跑，等价判据=文件对零改动 + 双跑用例集一致全绿；随迁双跑在补迁窗口按 §5.3 蓝本执行）；(iii) 宿主留守锚点（在宿主包 T0/T1 双跑，经 compat 委托触达新实现）。

- **T0 树** = `5c3131e60`（P2 基线对齐 + K4.0-R ledger 修复后的搬迁前状态；detached worktree `git worktree add --detach /tmp/k45-t0 5c3131e60`，跑毕已 `git worktree remove --force` 清理）。K4.0 全量 T0 套件（四宿主包+knowledge 模块树全绿）见 K4.0 报告 §3，同为该树。
- **T1 树** = `60ef71061`（K4.4 后、K4.5 前 HEAD）。
- 差分失败修复策略：本轮零失败（全 PASS），无「只修新实现」事件。

### 1.1 差分命令台账（原文 + 退出码 + 用例计数）

T0 侧（/tmp/k45-t0 @5c3131e60；`SVC_RUN`/`REPO_RUN`/`HD_RUN` 为下表各 -run 选择的完整正则，逐字在 K4.5 报告 §1 备档）：

| # | 命令 | 退出码 | 输出摘要 |
|---|---|---|---|
| T0-1 | `go test -count=1 -v -run "$SVC_RUN" ./internal/application/service/` | 0 | `ok ... 1.808s`；**37 PASS / 0 FAIL** |
| T0-2 | `go test -count=1 -v -run "$REPO_RUN" ./internal/application/repository/` | 0 | `ok ... 1.391s`；**24 PASS / 0 FAIL** |
| T0-3 | `go test -count=1 -v -run "$HD_RUN" ./internal/handler/` | 0 | `ok ... 1.968s`；**9 PASS / 0 FAIL** |
| T0-4 | `go test -count=1 -v -run "TestCopyAdmissionReserves…\|TestCopyAdmissionRequires…\|TestCopyAdmissionKeeps…" ./internal/handler/`（首轮 `HD_RUN` 模式笔误 `TestCopyAdmission_` 带下划线未匹配实际函数名，补跑 3 用例，两轮结果均在册） | 0 | **3 PASS / 0 FAIL** |
| T0-5 | `go test -count=1 -v -run "TestNewChunkExtractServiceNilSeamFailFast\|TestNewChunkExtractServiceNilSpanTraceAllowed" ./internal/knowledge/ingest/` | 0 | `ok ... 1.030s`；**2 PASS / 0 FAIL** |

T1 侧（worktree @60ef71061）：

| # | 命令 | 退出码 | 输出摘要 |
|---|---|---|---|
| T1-1 | `go test -count=1 -v -run "TestDocumentProcessTaskOptions_\|TestKnowledgePostProcessTaskOptions" ./internal/knowledge/process/` | 0 | `ok ... 0.785s`；**4 PASS / 0 FAIL** |
| T1-2 | `go test -count=1 -v -run "TestCreateKnowledgeDefaultsCustomMetadataToEmptyObject\|TestKnowledgeSourceSchemaAllowsObjectStorageURLs" ./internal/knowledge/process/repository/` | 0 | `ok ... 0.796s`；**2 PASS / 0 FAIL** |
| T1-3 | `go test -count=1 -v -run "TestRequireTaskProgressTenant_" ./internal/knowledge/process/handler/` | 0 | `ok ... 0.929s`；**3 PASS / 0 FAIL** |
| T1-4 | `go test -count=1 -v -run "$SVC_RUN 减随迁 4 用例" ./internal/application/service/` | 0 | `ok ... 1.542s`；**33 PASS / 0 FAIL** |
| T1-5 | `go test -count=1 -v -run "$REPO_RUN 减随迁 2 用例" ./internal/application/repository/` | 0 | `ok ... 0.913s`；**22 PASS / 0 FAIL** |
| T1-6 | `go test -count=1 -v -run "$HD_RUN 减随迁 3 用例" ./internal/handler/` | 0 | `ok ... 0.970s`；**6 PASS / 0 FAIL** |
| T1-7 | 同 T0-4 补跑 3 用例（T1 侧） | 0 | **3 PASS / 0 FAIL** |
| T1-8 | 同 T0-5（T1 侧） | 0 | `ok ... 0.892s`；**2 PASS / 0 FAIL** |

逐用例集合比对（`grep '^--- PASS' | awk '{print $3}' | sort` 后 `diff`，K4.5 会话实跑）：

```
service     : diff T0(37) vs T1(33)+模块侧(4)  → IDENTICAL
repository  : diff T0(24) vs T1(22)+模块侧(2)  → IDENTICAL
handler     : diff T0(9+3) vs T1(6+3)+模块侧(3) → IDENTICAL
ingest      : diff T0(2) vs T1(2)              → IDENTICAL
```

合计 **80 用例次/侧（唯一用例 75 个，kb_access 5 用例为第二/三面共用锚点）、全 PASS、零 FAIL、T0/T1 用例名集合逐项一致**。

### 1.2 四面差分结论（锚定用例清单 + 等价判据对照）

| 面（计划 §8.3） | 锚定用例（本会话 `-v` 输出在册） | 分布 | 双跑比对 |
|---|---|---|---|
| **① 巡检置败**（`knowledge.processing.failed` producer，`TypeManualProcess` 清扫） | `TestHousekeeping_RecoversAbandoned/_RecoversPendingTaskMissingFromQueue/_PreservesPendingTaskStillQueued/_NoFalseKill_ActiveSpan/_NoFalseKill_StaleSpanRecovers/_NoFalseKill_TasksStillQueued/_NoFalseKill_DurableWikiIngestPending/_StillRecoversWhenNoDurableOp/_QueueProbeError_FailsSafe/_PreservesRecentlyTouched`（10 用例，覆盖 stale 阈值、filterByLastSpanActivity/filterOutQueued、cron Start/Stop） | **成对推迟**（knowledge_housekeeping.go + _test 同留守宿主，Ruling DEFERRED-FILE-SPLIT，Mimosa DDL 测试常量误报全通道阻断——K4.2 报告 §3 实录） | T0 宿主 10 == T1 宿主 10，全 PASS，文件对零改动（不在第一父链变更并集）——非回归等价成立；**sweep 候选集/置败路径/failure_reason metadata 的随迁双跑顺延至补迁窗口**（§5.3 蓝本第 5 点，Brief (h) 登记） |
| **② write-family 守卫**（K1 KnowledgeWriteGuard seam 依赖面） | span_tracker 13 用例（`TestSpanTracker_OpenAttempt_AllocatesFreshNumbers/_FailSpan_CascadesDownstream/_LookupStage_FindsAcrossProcesses`、`TestFitSpanName`、`TestSpanTracker_BeginSubSpan_LongWikiPageName/_LookupSpanByName_FitsLongName/_HangsUnderParent`、`TestSpanTracker_BeginStage_ReentryIsIdempotent/_FailSpan_CascadesDependentSubspans`、`TestPostprocessSubspan_AttachesUnderPostProcessStage/_MissingParentFallsThrough`、`TestChunkExtractPayload_AttemptRoundTrip`、`TestSummaryQuestionPayload_AttemptRoundTrip`）+ task_options 4 用例（`TestDocumentProcessTaskOptions_defaults/_configuredTimeout/_extraMaxRetry`、`TestKnowledgePostProcessTaskOptionsUseDedicatedQueue`）+ K1 落位包锚点 2 用例（`TestNewChunkExtractServiceNilSeamFailFast/_NilSpanTraceAllowed`，跨 plan 只跑不迁）+ 宿主留守锚点 write_access 10 用例（`TestFAQAndTagWritesRejectUnscopedServiceCalls` 等 10，构造 `&knowledgeService{}` 经 compat 委托触达新实现）+ kb_access 5 用例（与③共用） | span_tracker 对**成对推迟**；task_options 已随迁（T0 宿主→T1 `process` 包）；其余宿主双跑 | 守卫判定序列（task_options 随迁 4 用例 T0/T1 一致）、span 语义（span_tracker 13 用例留守对 T0==T1 全绿，随迁双跑顺延补迁窗）、委托等价（write_access 10 + kb_access 5 宿主经 compat 全绿）三判据覆盖 |
| **③ handler 面**（task 进度防枚举 + KB 门） | task_progress 3 用例（`TestRequireTaskProgressTenant_RejectsCrossTenant/_AllowsOwnTenant/_InvalidTaskID`）+ 宿主 `kb_access_test.go` 5 用例（`TestKBGuardAndHandlersShareResolution`、`TestKBHandlerDoesNotReuseGrantForAnotherResourceOrCaller`、`TestKBHandlerAPIKeyScopeStillAppliesToCachedGrant`（R2 直连 api-key 门等价锚点）、`TestKBHandlersHonorExplicitAgentWithoutRouteGuard`、`TestKBHandlerEnforcesAccessWhenRouteRBACIsDisabled`）+ `knowledge_transfer_test.go` 4 用例（`TestCopyAdmissionReserves…/Requires…/Keeps…`、`TestMoveAdmissionDeduplicatesIDsAndRequiresBothKBs`） | task_progress 已随迁（T0 宿主 handler→T1 `process/handler`）；kb_access/transfer 留守宿主经 compat 委托 | 状态码映射、RBAC/apiKey 门、跨租户 404 防枚举（RejectsCrossTenant）、宿主委托与新实现等价四判据覆盖：12 用例 T0==T1 全 PASS |
| **④ repository 面**（模型写入/查询/事件 producer 纯函数面） | finalize 9 用例（`TestFinalizeSubtask_Concurrent_ExactlyOnePromote`（CompleteProcessingWithoutSubtasks 原子 promote 语义锚点）等）+ span_repo 7 用例（`TestKnowledgeSpanRepo_UpsertAndList` 等）+ tag 6 用例（`TestBatchCountReferences_ScopedToKnowledgeBase` 等）+ create/source_schema 2 用例（`TestCreateKnowledgeDefaultsCustomMetadataToEmptyObject`、`TestKnowledgeSourceSchemaAllowsObjectStorageURLs`） | 三测试文件**成对推迟**（Mimosa DDL 阻断 + setupKnowledgeTestDB 闭包，K4.1 §0；模块侧生产为副本、宿主原件保留）；create/source_schema 已随迁（T0 宿主→T1 `process/repository`） | DB 结果、跃迁恰一次语义：留守对 22 用例 T0==T1 全绿；随迁 2 用例 T0/T1 一致全 PASS；**tag BatchCountReferences 经直连 kbretrieval 构造的用例顺延补迁窗**（tag_test 推迟） |

**判「四面双跑逐用例一致」成立**；四面中随推迟批顺延的随迁双跑义务（housekeeping 对、span_tracker 对、repository 三测试文件对）已在 Brief (c)/(h) 登记解除窗口义务，**legacy 删除（manifest 行删除）均未发生**（推迟件行保留，Ruling 6 第 4 点），符合 framework:31「legacy 删除前差分必须已通过」。

### 1.3 节点门禁（K4.5 Step 2；conventions §2 禁替代——命令原文逐条实跑）

| 命令 | 退出码 | 关键输出 |
|---|---|---|
| `go build ./...` | **0** | 仅 `ld: warning: ignoring duplicate libraries: '-lc++'`（cmd/desktop、cmd/server 链接噪音，基线固有，K4.1 T0 起在册） |
| `go test ./internal/knowledge/... -count=1` | **0** | 25 包 `ok`（含 `process` 3.593s 量级、`process/repository`、`process/handler` 新包）、0 FAIL、4 包 `[no test files]`（retriever/elasticsearch、neo4j、postgres、knowledge 根——K4.0 T0 同态） |
| `make check-backend-architecture` | **0** | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`；`OK (0 violations)` |
| `make verify-module-moves` | **0** | `modulemove: OK (16 manifests verified)` |

附加（非替代，T1 台账）：`go test -count=1 ./internal/application/repository/ ./internal/application/service/ ./internal/handler/` 退出 **0**（`ok 300.833s` / `ok 250.704s` / `ok 1.582s`）——与 K4.0 T0（同四包全绿）零新失败。

## 2. 计数奇偶与基线登记（conventions §8；K4.5 Step 4 (i)；K4.4 §8 登记落盘）

### 2.1 route/worker/hook/migration 四值（architectureguard 输出原值）

`633 路由 / 23 Redis + 23 Lite 任务类型 / 58 生命周期挂点 / 537 迁移文件` —— 本节点全程零变化（四任务报告逐任务复验 + K4.5 终态复跑 §1.3）；worker 实现体迁移数 **0**（§3.2）。

### 2.2 legacy_files 台账（pass-a-acceptance.md §6，本节点两行在册）

| 日期 | 变更 | 依据 |
|---|---|---|
| 2026-09-25 | 391 → **389**（−3 K4.2 物理迁移行；+1 service compat 过渡 shim 成对补行）；ownership_test wantPerModule knowledge 79→77 | Ruling LEGACY-ROW-OWNERSHIP + TRANSITION-SHIM-ROW-REGISTRATION + DEFERRED-FILE-SPLIT（收缩依据） |
| 2026-09-26 | 389 → **388**（−2 K4.3 物理迁移行；+1 handler compat 过渡 shim 成对补行）；wantPerModule knowledge 77→76 | 同上 |

**K4.5 终态三方复验（本会话实跑）**：matrix `legacy_files` 总数 **388**（`grep -c '^  - path: internal/' ownership-matrix.yaml`）、knowledge 模块行 **76** == `tools/passbguard/ownership_test.go` wantPerModule `knowledge: 76`（:324-329）；plan=24 行 **25**（28 − K4.2 删 3 − K4.3 删 2 + 2 compat 行）；manifest `moves/knowledge.yaml` legacy 行 53；manifests 全量发现与 matrix 的 23 行结构性差（33 条 K2/K3 已迁残留行 − 10 条 compat 缺配对行）为 BASE 既有漂移（K4.0-R Step 4 处置表 + K4.3 #20 passbguard 诊断条目级 IDENTICAL 实证，本节点零贡献；ib2 回写批）。

### 2.3 exception-ledger 台账（K4.4 登记落盘 + P2 重编号衔接）

- P2 基线对齐重编号映射（K4.0 报告 §1.2/§1.3 在册，此处为 Brief (i) 的事实源）：
  - **K2 块**：exc-0106→**0112**、0107→0113、0108→0114、0109→0115、0110→0116（105→111→116）；
  - **K3 块**：exc-0106→**0117**、0107→0118、0108→0119、0109→0120、0110→0121、0111→0122、0112→0123、0113→0124、0114→0125、0115→0126、0116→0127、0117→0128、0118→0129（116→**129** 终值）；
  - 各前置分支 evidence 文件中的历史 exc 编号不回改（历史台账），映射关系以本表为准（K4.0 报告建议协调者记 DAG notes）。
- **K4.4 新行衔接**：exc-**0130**（`process/handler/kb_access.go → modules/policy/access`）、exc-**0131**（`process/knowledge_write.go → modules/policy/access`），ID 顺延 P2 终值 0129；头部计数注释 129→**131**；ledger 现总条目 **131**（`grep -c '^  - id: exc-'` 实测）。
- **K4.4 登记的原因与 Ruling 引用（K4.4 报告 Step 2 备妥内容，此处落盘）**：两对均为「搬迁前宿主原文件既有 import 纯移动显形」（`git show 34418f700^:internal/handler/kb_access.go:10`、`git show ff5ae6d21^:internal/application/service/knowledge_write.go:8` 实证），依据 **Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY**（精确 file→package 豁免、数据行非逻辑、同窗 ledger 行 owner=24-knowledge-process、remove_at=ib2）；根门面消费评估不可行（`internal/policy` 根仅 module.go 门面注释、零 access 符号再导出），K3 `wiki_fixer_scope.go→policy/access` 同目标包豁免先例（check.go:108-114）；PassBTask 取 `B-knowledge`（映射表唯一 knowledge 键，passbguard/check.go:83-93；计划文本「K4.4」与 guard 事实源矛盾，按 Ruling 6 末句勘误——K4.4 报告 §3.1）。
- **K4.4 多退少补勘误（在册）**：计划 §5.5 种子 3 对（横向包目标 internal/common ×2、internal/application/repository ×1）经 guard 判定面实证（check.go:1044 moduleImportBase + :1378-1382 前缀短路）**零诊断零登记**；表首「faq→repository 先例」引用不实（既有 129 条豁免目标 100% 为 internal/modules/ 路径）。
- K4.0-R：exc-0117..0127 共 11 行 reason 对 check.go 事实源逐字对齐（F1 裁定；K3 分支继承漂移），`exception-reason-drift`=0（commit `5c3131e60`）。

## 3. 结构等价核验（R100 偏差的机械等价替代证据汇总）

Mimosa 环境安全策略阻断 `git mv` 写 .go 文件（K4.1/K4.2/K4.3 阻断实录在各报告），物理迁移退化为 Write（落位侧）+ `git rm`（宿主侧）两步、M2/M3 同 commit，rename 相似度为：

| 文件 | git 相似度 | 等价核验 |
|---|---|---|
| process/repository/knowledge_create_test.go、knowledge_source_schema_test.go | **R100** | `git diff --summary --find-renames`（K4.1） |
| process/knowledge_task_options_test.go | R091（K4.2-R 修复后 range 级） | `git diff --name-status 3665fff5c..HEAD` 呈现 R091；改动类别=package 子句+4 处导出名调用点 |
| process/knowledge_write.go / knowledge_index_content.go / knowledge_task_options.go | R79 / R74 / R77 | `git diff --no-index -U0 宿主原件 模块副本` 逐行类别核验（K4.2 #8）：改动行恰为 package/import/导出改名/kbretrieval 直连/注释名五类 |
| process/handler/{kb_access,task_progress_auth,task_progress_auth_test}.go | R82 / R85 / R86 | `diff <(git show HEAD:<原件>) <副本>` ×3（K4.3 #11）：改动行恰三类（注释名/M3 改名/R2 直连），函数体其余行逐字一致 |
| process/repository/{knowledge,knowledge_span_repo,knowledge_tag,knowledge_transfer}.go | 纯 A（宿主原件**保留**，K4.1 收缩形态） | `cmp` ×6 全 IDENTICAL（M3 改名前）；模块副本与宿主原件 diff 仅 EscapeLikeKeyword/LikeEscapeChar 改名 8 处（K4.1 §1 Step 2）；**收口（删原件+删行+compat）排期补迁轮**（Brief (e)/(f)） |

## 4. 环境事件（如实）

1. Mimosa 安全 hook 对含 CREATE TABLE DDL 测试常量的 .go 文件全通道拦截（Write/Edit/Bash/git mv，K4.1 §0/K4.2 §3/K4.3 §4 实录）——根因类已由 conventions §10 DEFERRED-FILE-SPLIT 根因类补充（K4.1 实证）预先裁定为可登记根因；**用户裁定（2026-09-26 选项 1）已批准 K4.1 报告所列 3 个 DDL 测试文件逐字补迁**，执行通道=协调者经工作流 world.run（不经 Bash 工具钩子）执行 git mv 纯重命名 + 引用批准的独立 commit；实施侧 Agent 不自行变换写法绕过扫描（conventions §10 ENV-BLOCKED 解除登记原文）。补迁时点：在途任务落定后的间隙或 ib2 前——**待执行，登记于报告「未完成项」与 Brief (h)**。
2. Mimosa hook 多次报 `scanner_enobufs`（K4.0 merge 3/K4.0-R/K4.3/K4.4 commit 时）——按兼容策略放行提交，不据此宣称安全面结论。
