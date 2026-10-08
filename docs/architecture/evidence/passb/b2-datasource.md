# b2-datasource 证据文件（Pass B / 26-datasource）

> 节点：`b2-datasource`（B2 Data Source：4 legacy 文件迁入模块 + kbActivity 直连改写 + cleanup seam + 别名/例外收口）。计划：`docs/plans/passb/26-datasource.md`。
> ALIGN_SHA = `486d46b424b95d49903e3e9aaf23f1461ce5d863`（P-2 Case A 基线对齐 merge commit，2026-09-27；`git merge --no-ff codex/passb-b2-k-integration` 合入 461d8c4b2）——本节点全部 diff 检查的 PASSB_BASE_SHA 采用值（计划 §0.1 P-2 裁定，禁用 merge-base origin/main 缺省公式）。
> 本文件由 B2-DS.1 起累积；B2-DS.7 定稿差分与计数章节。

## §前置核验（B2-DS.1 Step 1；2026-09-27，worktree `.worktrees/passb-b2-datasource`，对齐后树）

### P-1 前置节点终态（DAG 实测）

判据来源：调度事实源 `.worktrees/passb-int/docs/plans/passb/execution-dag.json`（conventions.md 头注指定；本 worktree 分支上的 DAG 副本为撰写时点快照，b2-k-integration 状态滞后，不采用）。

| 节点 | status | review_status | head_sha | 判定 |
|---|---|---|---|---|
| `ib1` | done | approved | 8c45a8815（已回填） | 满足 |
| `b2-k-integration` | done | changes_requested | 461d8c4b2（已回填，= 分支实际 HEAD；task_status K5.1/K5.2/K5.3 全 done；notes 含 2026-09-26 门禁裁决：check-passb-readiness exit=2 系 P-K5-7 预裁定预期失败） | status+head_sha 满足；review_status 偏差见下 |

**偏差登记（如实，非阻塞）**：计划 §0.1 P-1 期望 `review_status=approved`，调度事实源实测为 `changes_requested`。判定继续开工的依据：① `status=done` 且 `head_sha` 已回填（终态登记完成）；② 调度事实源 DAG 已将本节点 `b2-datasource` 置为 `in_progress`（= 协调者恢复派发的行动裁定，与本次派发指令一致）；③ 计划 §0.1 P-1 自身口径「以开工时点 DAG 实测为准……残留文本为历史登记，不构成停工依据」。本偏差不属 conventions §5 门禁不可能通过情形，按「如实登记」处理，留协调者/审查者复核。

### P-2 基线对齐 merge（Ruling 2026-09-24-WAVE-DEP-BASELINE）

```bash
git -C .worktrees/passb-b2-datasource rev-parse codex/passb-b2-k-integration
# 461d8c4b2aa346519d94ea84cffaa5108ec45796
git -C .worktrees/passb-b2-datasource merge-base --is-ancestor codex/passb-b2-k-integration HEAD; echo "exit=$?"
# exit=1  → Case A（非祖先，执行对齐 merge）
git -C .worktrees/passb-b2-datasource merge --no-ff codex/passb-b2-k-integration -m "merge: passb b2-datasource P-2 基线对齐（合入 codex/passb-b2-k-integration @461d8c4b2，Ruling 2026-09-24-WAVE-DEP-BASELINE）"
# MERGE_EXIT=0，无冲突（计划预判的 docs/plans/passb 冲突未发生：本侧 26-datasource.md 为本侧独有新增、K 系列 plan 文件为 K 侧独有新增，两侧文件集不相交，git 自动合并；internal/knowledge/module.go 无冲突）
```

`git log --oneline -3` 留档：

```text
486d46b42 merge: passb b2-datasource P-2 基线对齐（合入 codex/passb-b2-k-integration @461d8c4b2，Ruling 2026-09-24-WAVE-DEP-BASELINE）
c0d380e14 docs(plan): passb b2-datasource R2 修订——purge_test 构造断言补编译性修正、appconnector 消费面更正、例外计数口径对齐 K 分支、行号/枚举勘误
afe266402 docs(plan): passb b2-datasource R1 修订——repo 测试随迁补全(105→115 用例)、-run 模式实测化、消费面补 router 三处、无环论证更正、行号勘误
```

**ALIGN_SHA = `486d46b424b95d49903e3e9aaf23f1461ce5d863`**。共同收尾判据：`git status` 干净 ✓、`go build ./...` exit 0 ✓（见 P-6）。

> 执行事故登记（如实）：P-2 首次 merge 因实施者 cwd 判断失误在主 checkout main 分支执行（产生错误 merge commit 871fd0d66，未推送）。已按「备份用户未提交修改（scripts/parity/auto-scan.mjs）→ `git reset --hard ae30162c5` → 恢复用户修改」完整还原主 checkout（HEAD=ae30162c5、status 仅余既有 `M scripts/parity/auto-scan.mjs`，与会话起始快照一致）；错误 commit 成悬空对象。worktree 全程未受影响（merge 前后均验证 clean、HEAD=c0d380e14→486d46b42 仅本节点对齐 merge）。后续全部命令改用 `git -C`/绝对路径执行。详见实施报告 §事故与修复。

### P-3 K 面产物在位（对齐后实测）

```bash
ls docs/architecture/passb/briefs/b2-k-integration.md docs/architecture/evidence/passb/b2-k-integration.md
# exit 0，两文件在位（25758B / 27780B）
grep -c "^func RecordKBActivity|^func KBActivityTrigger|^func WithKBActivityTask|^func WithKBActivitySuppressed" internal/knowledge/retrieval/app/kb_activity.go
# 4  ✓（K2 导出面就绪）
grep -n "^func withKnowledgeCleanup" internal/application/service/knowledge_delete_plan.go
# 22:func withKnowledgeCleanup(ctx context.Context, tenant uint64, bindings map[string]string) context.Context {  ✓（K4 推迟件留守未导出，§2.2 裁定前提成立）
```

### P-4 门禁工具可用

`Makefile` 三目标在位（sed 实读 248-265 行区段）：`verify-module-moves`（`go run ./tools/modulemove verify --all`）、`check-backend-architecture`（`go run ./tools/architectureguard`）、`check-passb-readiness`（`go run ./tools/passbguard -root .`）。✓

### P-5 worktree 干净

`git status --short` 无输出；分支 `codex/passb-b2-datasource`。✓

### P-6 基线门禁快照（ALIGN_SHA 树实跑）

| 命令 | 退出码 | 关键输出 |
|---|---|---|
| `go build ./...` | 0 | 仅 2 条已知 `ld: warning: ignoring duplicate libraries: '-lc++'`（cmd/desktop、cmd/server，K5 同款无害告警） |
| `go test -count=1 ./internal/modules/datasource/...` | 0 | 全 ok（datasource 根包 + connector/{dingtalk,feishu/{core,drive,wiki},gitlab,ima,moauth,notion,rss,yuque} 等） |
| `make check-backend-architecture` | 0 | `literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16`；`architectureguard: OK (0 violations)` |
| `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |

与计划 §0.1 P-6 预期值逐字一致（K 合并后计数不变）。✓

### 对齐后行号复核（计划 §0 头注义务）

**§2.2 的 24 调用点（datasource_service.go，对齐树实测）**——与计划表逐字一致，K 合并未漂移：

| 符号 | 实测计数 | 实测行号 | 计划预期 |
|---|---|---|---|
| `recordKBActivity(` | 17 | 258/374/420/462/525/778/915/1065/1111/1127/1201/1301/1325/1461/1529/1853/2425 | 17 处同 |
| `withKBActivityTask(` | 2 | :837、:1405 | 同 |
| `kbActivityTrigger(` | 1 | :837（一行双符号） | 同 |
| `withKBActivitySuppressed(` | 3 | :1642、:1790、:2106 | 同 |
| `withKnowledgeCleanup(` | 1 | :885 | 同 |

计 24 调用点 / 23 行（:837 一行双符号）✓。

**4 legacy 文件锚点抽查（对齐树实测）**：datasource_service.go `:30 type DataSourceService struct` / `:54 func NewDataSourceService(` / `:83 SyncSpaceStateResolver` / `:816 const dataSourcePurgeBatchSize = 200` / `:823 type dataSourceBindingCleaner` / `:857 PurgeDataSourceDocuments` / `:1157 ErrReindexDuplicateRequest` / `:1353 ErrSyncLogNotFound`——全部命中，行号稳定。✓

**purge_test 锚点**：`:116 svc *DataSourceService` / `:240 f.svc = &DataSourceService{...}` / `:467 svc := &DataSourceService{...}` / `:469 f.svc.knowledgeService`（字段读）/ `:472 PurgeDataSourceDocuments`——命中。**漂移登记**：bindingRows 断言实测 **:355-:356**（计划 §2.4/§5 记 :354-:355，1 行漂移；断言内容与锚定语义不变，后续任务按实测行号引用）。

## §特征化基线（B2-DS.1 Step 2；115 用例 = service 85 + repository 10 + handler 20）

计数口径（计划 T1 Step 2）：顶层用例 = `-v` 输出中不含 `/` 的 `=== RUN` 行。全部命令在 ALIGN_SHA 树（HEAD=486d46b42）实跑，日期 2026-09-27。

| # | 命令（原文） | 退出码 | 顶层 RUN | PASS | FAIL | SKIP |
|---|---|---|---|---|---|---|
| ① | `go test -count=1 ./internal/application/service/ -run 'TestDeleteDataSourceH|TestDeleteDataSourceP|TestDeleteDataSourceW|TestDeleteKnowledgeBaseCleansUpSQLiteDataSources|TestSetTaskInspector|TestPauseDataSource|TestManualSync|TestRefreshDataSourceCredential|TestProcessSync|TestIncrementAppDataSourceBindingAuthVersion|TestCursorAuthVersionStale|TestReindexItems|TestApplyFetchedItem|TestAllFetchedItemsFailedError|TestIngestItem|TestStream|TestSyncHeartbeat|TestCancelSyncLog|TestCheckpoint|TestCheckCancelRequested|TestPurgeWorker|TestProcessDataSourcePurge|TestDataSourceService|TestDataSourcePurgeQueueTopology|TestCountDataSourceDocumentsScopesToTenantKbDataSource' -v` | 0 | 85 | 85 | 0 | 0 |
| ② | `go test -count=1 ./internal/application/repository/ -run 'TestDataSourceRepository|TestSyncLogRepository|TestSyncLogLifecycle' -v` | 0 | 10 | 10 | 0 | 0 |
| ③ | `go test -count=1 ./internal/handler/ -run 'TestDataSource' -v` | 0 | 20（另 9 子测试，total RUN=29） | 20 | 0 | 0 |
| ④ | `go test -count=1 ./internal/modules/datasource/...` | 0 | —（模块既有面基线） | 全 ok | 0 | — |

完整日志留档（本会话产物）：`/tmp/ds-baseline-service.log`、`/tmp/ds-baseline-repo.log`、`/tmp/ds-baseline-handler.log`；用例清单 `/tmp/ds-baseline-{service,repo,handler}-cases.txt` ×3（115 行）。

### ① service 侧 85 用例清单（11 随迁文件 + purge 留守文件；85/85 PASS）

- `TestAllFetchedItemsFailedError`
- `TestAllFetchedItemsFailedErrorIgnoresPartialFailure`
- `TestAllFetchedItemsFailedErrorIgnoresSkippedItems`
- `TestAllFetchedItemsFailedErrorTruncatesLongDetail`
- `TestApplyFetchedItem_CapsErrorSampleButKeepsAccurateFailedCount`
- `TestApplyFetchedItem_EmbeddedImageIngestFailureCountsAsSkip`
- `TestApplyFetchedItem_FailureSamplesCarryExternalID`
- `TestApplyFetchedItem_ProducesLocalisableStructuredError`
- `TestApplyFetchedItem_SyncDeletionScopedPerDataSource`
- `TestCancelSyncLog_FlagsRunningLogWithinTenantAndDataSource`
- `TestCancelSyncLog_RejectsForeignOrTerminalLogs`
- `TestCheckCancelRequested_ReadsFlagFromRepository`
- `TestCheckpoint_CancelCheckRidesHeartbeatThrottle`
- `TestCheckpoint_CancelFlagEndsRunWithCursorPreserved`
- `TestCountDataSourceDocumentsScopesToTenantKbDataSource`
- `TestCursorAuthVersionStale`
- `TestDataSourcePurgeQueueTopology`
- `TestDataSourceServiceDeleteKeepsCleanupStateWhenSoftDeleteFails`
- `TestDataSourceServiceDeleteSQLiteCleansUpAfterSoftDelete`
- `TestDeleteDataSourceHardCancelsQueuedSyncTasksBeforeSweep`
- `TestDeleteDataSourcePurgeEnqueuesTaskAndWorkerDrainsDocuments`
- `TestDeleteDataSourceWithoutInspectorDegradesToSweep`
- `TestDeleteDataSourceWithoutPurgeKeepsDocuments`
- `TestDeleteKnowledgeBaseCleansUpSQLiteDataSources`
- `TestIncrementAppDataSourceBindingAuthVersion_Scoping`
- `TestIngestItem_NoSweepWhenDuplicateIsDifferentNode`
- `TestIngestItem_NoSweepWhenFlagUnset`
- `TestIngestItem_PersistsSourceUpdatedAt`
- `TestIngestItem_ReplacesSubtreeSweepsOnDuplicateParent`
- `TestIngestItem_ReplacesSubtreeSweepsStaleChildrenAfterCreate`
- `TestIngestItem_SubtreeKeepPreservesPresentChild`
- `TestIngestItem_SubtreeSweepSkipsOtherDataSourceChildren`
- `TestIngestItem_URLCreationAttachesDataSourceMetadata`
- `TestIngestItem_URLCreationMetadataAttachFailure`
- `TestManualSyncPassesForceFullToPayload`
- `TestManualSyncRecordsAsynqTaskID`
- `TestPauseDataSourceCancelsRunningAndQueuedSyncs`
- `TestProcessDataSourcePurgeRejectsInvalidPayload`
- `TestProcessSyncBatch_UserCancelAtItemBoundary`
- `TestProcessSyncCancelsWhenKnowledgeBaseDeleted`
- `TestProcessSyncStreaming_StaleAuthVersionDropsCursor`
- `TestProcessSyncStreaming_UserCancelIsGracefulSuccess`
- `TestProcessSync_ExpiredButRefreshedContinues`
- `TestProcessSync_ExpiredUnrefreshablePausesWithoutErrorFlip`
- `TestProcessSync_ExpiredWithoutRefresherPauses`
- `TestProcessSync_ExpiringCredentialsFullChain`
- `TestProcessSync_FarFromExpirySkipsRefresh`
- `TestProcessSync_LegacyUnboundRotationSkipsBindingBump`
- `TestProcessSync_LongLivedCredentialsSkipRefresh`
- `TestProcessSync_MatchingAuthVersionResumesCursor`
- `TestProcessSync_MissingKeyPersistsUnencrypted_ExistingBehavior`
- `TestProcessSync_NearExpiryRefreshFailureContinuesOnOldCredentials`
- `TestProcessSync_RotatedKeyLeavesEncryptedBlobUntouched`
- `TestProcessSync_ScopedReindexHappyPath`
- `TestProcessSync_ScopedReindexPerItemFailureCodes`
- `TestProcessSync_ScopedReindexSubtreeKeepContract`
- `TestProcessSync_ScopedReindexUnsupportedConnector`
- `TestProcessSync_SyncDeletionsAlreadyGoneCountsSkipped`
- `TestProcessSync_SyncDeletionsDeleteFailureCountsFailed`
- `TestProcessSync_SyncDeletionsDeletesMatchingKnowledge`
- `TestProcessSync_SyncDeletionsDisabledSkipsDeletion`
- `TestProcessSync_SyncDeletionsHardDeletesRow`
- `TestProcessSync_SyncDeletionsLookupFailureCountsFailed`
- `TestProcessSync_SyncDeletionsPartialWhenMixedResults`
- `TestPurgeWorkerDrainsAcrossBatches`
- `TestPurgeWorkerNoopOnPreCanceledContext`
- `TestPurgeWorkerRemovesOrphanAutoTag`
- `TestPurgeWorkerStopsBetweenBatchesWhenContextCanceled`
- `TestRefreshDataSourceCredential_GuardRejectsAfterKeyRotation`
- `TestRefreshDataSourceCredential_GuardRejectsUnconfigured`
- `TestRefreshDataSourceCredential_InputValidation`
- `TestRefreshDataSourceCredential_SingleKeyUpdateIsTheWholeContract`
- `TestReindexItems_BuildsScopedPayloadWithIdempotentTaskID`
- `TestReindexItems_DuplicateRequestIDIRejectedIdempotently`
- `TestReindexItems_EmptyRequestIDOmitsTaskID`
- `TestReindexItems_EnqueueFailureIsolatedToLogRow`
- `TestSetTaskInspectorInstallsHardCancel`
- `TestStreamHandler_CheckpointPersistsCursor`
- `TestStreamHandler_EmitAbortsOnCanceledContext`
- `TestStreamHandler_EmitClassifiesDeletedAndFailed`
- `TestStreamStartCursor_ForceFullFirstAttemptDropsCursor`
- `TestStreamStartCursor_IncrementalKeepsCursor`
- `TestStreamingFetchUsesFullStreamBaseline`
- `TestSyncHeartbeatCheckpointPersistsHeartbeat`
- `TestSyncHeartbeatMaybeHeartbeatThrottleWindow`

### ② repository 侧 10 用例清单（2 文件；10/10 PASS）

- `TestDataSourceRepositoryCreatePersistsDisabledSyncDeletions`
- `TestDataSourceRepositoryCreatePersistsEnabledSyncDeletions`
- `TestDataSourceRepositoryDeleteSoftDeletesOnSQLite`
- `TestDataSourceRepositoryUpdatePersistsDisabledSyncDeletions`
- `TestDataSourceRepositoryUpdateSyncStateClearsErrorMessage`
- `TestSyncLogLifecycleHasRunningSyncExcludesStalledRuns`
- `TestSyncLogLifecycleRequestCancel`
- `TestSyncLogLifecycleUpdateAsynqTaskID`
- `TestSyncLogLifecycleUpdateHeartbeat`
- `TestSyncLogRepositoryUpdateResultClearsErrorMessage`

### ③ handler 侧 20 顶层用例清单（4 文件；20/20 PASS，另 Delete_PurgeDocumentsParamMatrix×4 + ManualSync_ForceFullBodyMatrix×5 子测试全 PASS）

- `TestDataSourceCredentialsPut_ResponseCarriesExpiryMetadata`
- `TestDataSource_CancelSyncLog_Accepted`
- `TestDataSource_CancelSyncLog_NotFound`
- `TestDataSource_CountDocuments_ForbiddenCrossTenantKB`
- `TestDataSource_CountDocuments_NotFound`
- `TestDataSource_CountDocuments_ReturnsCount`
- `TestDataSource_CountDocuments_ServiceErrorIs500`
- `TestDataSource_Delete_PurgeDocumentsNotOwnedIs404`
- `TestDataSource_Delete_PurgeDocumentsParamMatrix`
- `TestDataSource_GetSyncLogs_LimitExceedingMaximum`
- `TestDataSource_GetSyncLogs_MissingLimitDefaultsCorrectly`
- `TestDataSource_GetSyncLogs_NegativeLimitRejected`
- `TestDataSource_GetSyncLogs_NonNumericLimitRejected`
- `TestDataSource_GetSyncLogs_ValidLimitWithinBounds`
- `TestDataSource_GetSyncLogs_ZeroLimitRejected`
- `TestDataSource_ManualSync_ForceFullBodyMatrix`
- `TestDataSource_ReindexItems_Accepted`
- `TestDataSource_ReindexItems_DuplicateRequestIs409`
- `TestDataSource_ReindexItems_EmptyIDsRejected`
- `TestDataSource_ReindexItems_NotOwnedIs404`

## §覆盖面核对（B2-DS.1 Step 3；高风险差分锚点定位）

grep 用例名实证（基线 -v 输出 + 源文件双核对），计划列出的全部锚点均在既有用例中锚定，**零新增**：

| 高风险行为 | 锚定用例/位置（实测存在） | 判定 |
|---|---|---|
| purge drain 跨批 | `TestPurgeWorkerDrainsAcrossBatches` | ✓ 基线内 |
| ctx 取消批次边界 | `TestPurgeWorkerStopsBetweenBatchesWhenContextCanceled` | ✓ |
| 墓碑/硬删 | `TestDeleteDataSourcePurgeEnqueuesTaskAndWorkerDrainsDocuments` | ✓ |
| 孤儿 tag | `TestPurgeWorkerRemovesOrphanAutoTag` | ✓ |
| 不 purge 保留 | `TestDeleteDataSourceWithoutPurgeKeepsDocuments` | ✓ |
| 绑定清理 + 兄弟行存活 | purge_test :355-:356 `bindingRows` 断言（实测行号，计划记 :354-:355） | ✓ |
| 硬取消顺序/无 inspector 降级 | `TestDeleteDataSourceHardCancelsQueuedSyncTasksBeforeSweep` / `TestDeleteDataSourceWithoutInspectorDegradesToSweep`（计划 §5 T1 Step 3 连写「...HardCancelsQueuedSyncTasksWithoutInspectorDegradesToSweep」系两用例名缩略连写；实测两用例均在基线） | ✓ |
| 暂停取消 | `TestPauseDataSourceCancelsRunningAndQueuedSyncs` | ✓ |
| asynq TaskID / force-full | `TestManualSyncRecordsAsynqTaskID` / `TestManualSyncPassesForceFullToPayload` | ✓ |
| 租户隔离 404 | sync_cancel :117/:119（另 :121/:122）`ErrSyncLogNotFound` 断言（grep 实测 5 处） | ✓ |
| 心跳 | service `TestSyncHeartbeat*` ×2 + repo `TestSyncLogLifecycleUpdateHeartbeat` | ✓ |
| SyncLog cancel 标志 / stall 窗口 | repo `TestSyncLogLifecycleRequestCancel` / `TestSyncLogLifecycleHasRunningSyncExcludesStalledRuns` | ✓ |
| 凭据轮换 / auth-version 游标 | refresh_test 4（`TestRefreshDataSourceCredential*`）+ trigger 14（`TestProcessSync_*` ×10 + `TestCursorAuthVersionStale` + `TestIncrementAppDataSourceBindingAuthVersion_Scoping` + `TestProcessSyncStreaming_StaleAuthVersionDropsCursor` 等，credential_refresh_trigger_test.go 逐用例实读 14 个全在基线）= 18 | ✓ |

## §差分（T3/T4 包级复跑为每个删除 commit 的即时门；B2-DS.7 小节为节点级四面×四要素终局比对）

### B2-DS.3 包级复跑（service 面；2026-09-27，迁移 commit 工作树实测）

T1 基线 service 侧 85 顶层用例 → 迁移后 **模块包 73 + 宿主留守 12 = 85 奇偶一致**：

| 命令（原文） | 退出码 | 关键输出 |
|---|---|---|
| `go test -count=1 ./internal/modules/datasource/service/` | 0 | `ok ... 2.955s`；`-v` 顶层 RUN（不含 `/`）= **73**，`--- PASS` = 73 |
| `go test -count=1 ./internal/application/service/ -run 'TestDeleteDataSourcePurge\|TestDeleteDataSourceWithoutPurge\|TestPurgeWorker\|TestProcessDataSourcePurge\|TestDataSourcePurgeQueueTopology\|TestCountDataSourceDocumentsScopesToTenantKbDataSource' -v`（计划 T3 Step 5 原命令） | 0 | 9/9 PASS（purge 留守 9 用例经 compat 装配运行，seam 接线等价性锚点） |
| `go test -count=1 ./internal/application/service/ -run 'TestDataSourceServiceDeleteSQLiteCleansUpAfterSoftDelete\|TestDataSourceServiceDeleteKeepsCleanupStateWhenSoftDeleteFails\|TestDeleteKnowledgeBaseCleansUpSQLiteDataSources' -v` | 0 | 3/3 PASS（delete_sqlite 留守处置，见报告偏差登记） |
| `go build ./...` | 0 | 仅链接器重复库警告（cmd/desktop、cmd/server 既有） |
| `go vet ./internal/modules/datasource/...` | 0 | 无输出 |
| `go vet ./internal/application/service/` | 0 | 无输出（含 K 属主留守测试 + Ruling 3 垫片编译） |
| `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)`（service 行级收口成对） |
| `make check-backend-architecture 2>&1 \| grep -c forbidden-import` | 1（guard 退出码） | 计数 **恰 3**（§2.3 三对：knowledge/retrieval/app、appconnector、policy/access——B2-DS.5 登记后归零；计划 T3 Step 5 预期窗口，非节点失败） |

K 属主留守测试断链面（DAG ppc 清单外显形）与 Ruling 2026-09-24-TEST-SUPPORT-SHIM 垫片锚定复跑：
`TestDataSourceTagCreationReceivesOnlyItsTaskKBGrant`、`TestSharedFAQWriteLoadsOwnerTenantInfoWithoutReplacingCaller`、`TestReplaceKnowledgeFile*` ×14 全 PASS（垫片文件 `internal/application/service/datasource_shim_test.go`，Ruling ID 头注，remove_at=ib2）。
宿主 service 全包 `go test -count=1 -timeout=20m ./internal/application/service/` → `ok 606.545s`（0 FAIL；默认 10m 超时不足以跑完 K 生态全量，属包固有耗时）。

24 调用点改写对账（模块侧 `internal/modules/datasource/service/datasource_service.go` grep 实测）：`app.RecordKBActivity(` = 17、`app.WithKBActivityTask(` = 2、`app.KBActivityTrigger(` = 1、`app.WithKBActivitySuppressed(` = 3、`s.knowledgeCleanup(` = 1（seam 调用点）、旧符号残留（`recordKBActivity(\|withKBActivityTask(\|kbActivityTrigger(\|withKBActivitySuppressed(\|withKnowledgeCleanup(`）= **0**。

### B2-DS.7 差分复跑比对（2026-09-27，HEAD=`52ae889a2` 树实跑；conventions §6 四要素=用例清单/双跑输出/比对结论/命令与退出码）

**要素①命令与退出码（Step 1 新旧同用例双跑，命令原文；④旧锚面为计划 T7 原命令、③为偏差面补跑）**：

| # | 面 | 命令（原文） | 退出码 | 顶层 RUN | 结果 |
|---|---|---|---|---|---|
| ① | 新实现（模块包：repository 10 + service 73 + handler 20 + 既有模块面） | `go test -count=1 ./internal/modules/datasource/... -v 2>&1 \| tee /tmp/ds-new.txt` | 0 | 483（其中迁移面 103 = repo 10 + service 73 + handler 20，其余为既有模块面/connector 等基线外用例） | 15 包全 `ok`，`--- FAIL`/`--- SKIP` = 0 |
| ② | 旧锚点（留守 purge_test，经 compat 装配运行同一实现，兼作 compat/seam 接线等价证据） | `go test -count=1 ./internal/application/service/ -run 'TestDeleteDataSourcePurge\|TestDeleteDataSourceWithoutPurge\|TestPurgeWorker\|TestProcessDataSourcePurge\|TestDataSourcePurgeQueueTopology\|TestCountDataSourceDocumentsScopesToTenantKbDataSource' -v 2>&1 \| tee /tmp/ds-old-anchor.txt` | 0 | 9 | 9/9 PASS，0 FAIL/SKIP |
| ③ | delete_sqlite 留守补跑（**偏差面**：计划 T7 注释预期「service 76」迁移，B2-DS.3 实际 73+3 分裂——3 用例按 B2-DS.3 报告偏差登记留守宿主，命令模式沿 evidence §差分 B2-DS.3 节第 3 行） | `go test -count=1 ./internal/application/service/ -run 'TestDataSourceServiceDeleteSQLiteCleansUpAfterSoftDelete\|TestDataSourceServiceDeleteKeepsCleanupStateWhenSoftDeleteFails\|TestDeleteKnowledgeBaseCleansUpSQLiteDataSources' -v 2>&1 \| tee /tmp/ds-old-sqlite.txt` | 0 | 3 | 3/3 PASS |

注：表中 `\|` 为 markdown 表格转义；实跑命令用 `|`（go test -run 正则交替）。首次误用 `\|` 字面量时输出 `[no tests to run]`（exit 0），已按正确正交替义重跑并留档 /tmp/ds-old-sqlite.txt。

**要素②用例清单（四面拆分，`-v` 顶层 RUN 行不含 `/` 口径）**：repository 10（`/tmp/ds-new-repository.txt`）、service 73（`/tmp/ds-new-service.txt`）、handler 20（`/tmp/ds-new-handler.txt`）、旧锚点 9+3（`/tmp/ds-old-purge9.txt` + `/tmp/ds-old-sqlite3.txt`）——与 §特征化基线 115 清单一一对应。

**要素③双跑输出**：T1 基线（ALIGN_SHA 树，§特征化基线：85+10+20 全 PASS）vs 本轮（HEAD=52ae889a2：73+9+3+10+20 全 PASS）。

**要素④比对结论（Step 2 逐用例比对，程序化 diff 实测）**：

```bash
diff /tmp/ds-baseline-repo.txt /tmp/ds-new-repository.txt        # exit 0（identical，10/10）
diff /tmp/ds-baseline-handler.txt /tmp/ds-new-handler.txt        # exit 0（identical，20/20）
cat /tmp/ds-new-service.txt /tmp/ds-old-purge9.txt /tmp/ds-old-sqlite3.txt | sort > /tmp/ds-service-union.txt
wc -l < /tmp/ds-service-union.txt                                 # 85
diff /tmp/ds-baseline-service.txt /tmp/ds-service-union.txt       # exit 0（identical）
cat /tmp/ds-service-union.txt /tmp/ds-new-repository.txt /tmp/ds-new-handler.txt | sort > /tmp/ds-now-115.txt
diff /tmp/ds-baseline-115.txt /tmp/ds-now-115.txt                 # exit 0（identical 115）
```

**115 用例零偏差**：集合侧五组 diff 全空；状态侧 T1 基线 115/115 PASS vs 本轮 115/115 PASS、双侧 0 FAIL/0 SKIP——PASS/FAIL/SKIP 逐用例一致，通过判据满足。

**高风险两面专门登记（计划 T7 Step 2）**：

- **knowledge 删除/purge 级联**（framework §14.3）：purge 9 用例全在旧锚点面②运行——drain 跨批 `TestPurgeWorkerDrainsAcrossBatches`、ctx 取消批次边界 `TestPurgeWorkerStopsBetweenBatchesWhenContextCanceled`、墓碑/硬删 `TestDeleteDataSourcePurgeEnqueuesTaskAndWorkerDrainsDocuments`、孤儿 tag `TestPurgeWorkerRemovesOrphanAutoTag`、不 purge 保留 `TestDeleteDataSourceWithoutPurgeKeepsDocuments`、预取消 noop `TestPurgeWorkerNoopOnPreCanceledContext`、无效载荷 `TestProcessDataSourcePurgeRejectsInvalidPayload`、队列拓扑 `TestDataSourcePurgeQueueTopology`、计数作用域 `TestCountDataSourceDocumentsScopesToTenantKbDataSource`。**绑定清理断言（purge_test `:349-:350` `bindingRows`，实测行号，属墓碑用例）同时证明模块 `datasource_service.go:898-899` seam 调用（`s.knowledgeCleanup`，`SetKnowledgeCleanup` 接线 `:121`）与迁移前 `withKnowledgeCleanup` 直引等价**。
- **Worker 状态机/取消/重试/幂等**（framework §14.3）：cancel_enqueue 6（`datasource_cancel_enqueue_test.go` 逐名：`TestDeleteDataSourceHardCancelsQueuedSyncTasksBeforeSweep`/`TestDeleteDataSourceWithoutInspectorDegradesToSweep`/`TestPauseDataSourceCancelsRunningAndQueuedSyncs`/`TestManualSyncRecordsAsynqTaskID`/`TestManualSyncPassesForceFullToPayload`/`TestSetTaskInspectorInstallsHardCancel`）+ sync_cancel 7（`datasource_sync_cancel_test.go`：`TestCancelSyncLog_*`×2 + `TestCheckpoint_*`×2 + `TestCheckCancelRequested_ReadsFlagFromRepository` + `TestProcessSyncStreaming_UserCancelIsGracefulSuccess` + `TestProcessSyncBatch_UserCancelAtItemBoundary`）+ sync_heartbeat 2（`TestSyncHeartbeat*`）+ stream 6（`TestStreamStartCursor_*`×2 + `TestStreamHandler_*`×3 + `TestStreamingFetchUsesFullStreamBaseline`）——文件级 `grep -c '^func Test'` 实测 6/7/2/6 与计划计数一致，全 PASS。
- **审计活动流**：宿主 `purgeAuditSink.findByAction`（`datasource_purge_test.go:95` 定义，`:319` 消费 `AuditActionDataSourceDeleted`）+ 模块侧 findByAction 断言（`datasource_credential_refresh_test.go:198`、`datasource_credential_refresh_trigger_test.go` 7 处，含 `AuditActionDataSourceCredentialAutoRefreshed`/`AuditActionDataSourceSyncFailed`）——覆盖 17 个 `app.RecordKBActivity` 直连改写点的行为面（:349-:350 绑定断言所在用例即经 seam 走完整清理链）。

**结论**：差分四要素齐备，四面（①repo 10 / ②service 73 / ③handler 20 / ④旧锚 9+补 3）115 用例零偏差，任一未归因偏差为零——B2-DS.7 通过判据满足。

## §计数基线

### 例外行登记（B2-DS.5，2026-09-27，任务 commit 工作树实测）

- **基值 X 实测**：`grep -c "id: exc-" docs/architecture/passb/exception-ledger.yaml` = **131**（登记前 HEAD=75e94da99 树实测；max `grep -o "id: exc-[0-9]*" | sort | tail -1` = exc-0131——与计划 §2.6「撰写时点 K 分支实测 131 行、max exc-0131」一致，id 顺延 exc-0132..0134，不硬编码）。
- **X+3 机械修正**：ledger 头注计数 131→**134** 并补登来源行（26-datasource B2-DS.5 3 条，同窗同 commit）；代码侧无计数断言（`grep -n "105\|131\|134\|len(g.Exceptions)" tools/passbguard/*.go` 零命中，Makefile:260 的 105 为 B0 期文档注释非断言）。
- **双侧一致（F1：check.go importExceptions ↔ ledger 逐字）**：passbguard exception-reason-drift / exception-unregistered / exception-overlap / exception-plan-unknown / exception-barrier 诊断均为 **0**（3 行 plan=26-datasource 在 KnownPlans{modules:[datasource], barrier:ib2}，remove_at=ib2 匹配）。
- **conventions §8 基线登记**：台账 `docs/architecture/evidence/pass-a-acceptance.md` 不改（B2 阶段口径），登记于本节；变更原因=26-datasource B2-DS.5 搬迁显形 3 对预存横向耦合登记（授权依据=Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY + 冻结计划 §2.3 种子表）；例外计数基线 131→134（ib2 收口回落）。
- **guard 归零复证（T5 Step 4，命令原文+退出码）**：

| 命令 | 退出码 | 关键输出 |
|---|---|---|
| `make check-backend-architecture` | 0 | `total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`、`OK (0 violations)`（B2-DS.3 时点恰 3 条 forbidden-import 全部被精确豁免吸收归零） |
| `make verify-module-moves` | 0 | `OK (16 manifests verified)` |
| `go run ./tools/architectureguard 2>&1 \| tail -3` | 0 | `OK (0 violations)`，无例外诊断新增 |
| `go build ./...` | 0 | 仅既有链接器重复库警告（cmd/desktop、cmd/server） |
| `gofmt -l tools/architectureguard/check.go` | 0 | 无输出（格式合规） |

- **passbguard 前后对照（非本节点 gate，诚实登记）**：诊断行数 259→248。①**−12 治愈**：基线 12 条 `contract-consumer-module-import` 全部指向 `internal/modules/datasource/service/datasource_service.go`（6 契约 consumer × knowledge/retrieval/app + policy/access 两导入，stash 复跑实证 /tmp/passbguard-baseline-ccmi.txt），本登记使 `excepted[from→to]` 命中而全部消失；②**+1 新增**：`exception-task-module: guard PassBTask "B2-DS.5" has no module mapping`（3 行同 check+path+message 去重为 1；`PassBTaskModule` 映射（tools/passbguard/check.go:83-93）仅含 B0 建制的 9 个模块级 id，无 B2-DS.5/B-datasource——计划 T5 Step 1 原文指定 `PassBTask: "B2-DS.5"`，本节点无权改 passbguard 补映射；移交 IB2：barrier 在 PassBTaskModule 增 `"B2-DS.5": "datasource"` 行或裁定改用模块级 id，归入 Brief (c) 例外收口编排）。check-passb-readiness 本就因 K 谱系继承债务 exit 1（计划 §12 实测在案），本任务净减 11 条诊断。

### 三方一致复核（B2-DS.7，2026-09-27，HEAD=`52ae889a2` 树实跑；conventions §8 / F5 口径）

| 指标 | guard 实测 | `pass-a-acceptance.md` 台账 | 发现值 | 一致 |
|---|---|---|---|---|
| 路由 | `make check-backend-architecture` exit 0：`literal=564 apiKeyRoute=69 handle=0 total=633` | :22「633（564 literal + 69 apiKeyRoute）」 | architectureguard 扫描即发现面（同左列命令输出） | ✅ |
| worker | `redis=23 lite=23` | :23「23 任务类型 + 6 池 / 23」 | 同上 | ✅ |
| hooks | `hooks=58` | :24「58」 | 同上 | ✅ |
| migrations | （guard 不扫 migrations） | :25「537 文件（270 assets）」 | `find migrations -type f \| wc -l` = **537**（versioned 346 + mysql/paradedb/sqlite 191，`ls`/`wc -l` 实测） | ✅ |

- P-6 基线快照（ALIGN_SHA 树）同值：633/23+23/58/16、16 manifests、0 violations——**B2-DS.1 至 B2-DS.7 全程计数零漂移**。
- 本节点零路由/worker/hook/migration 增删：`git diff "$ALIGN_SHA"...HEAD --name-only | grep -E "internal/router/|migrations/"` 计数 = **0**（实跑 exit 1 无匹配）。
- 例外行基线 131→134（B2-DS.5 §例外行登记，ib2 回落）；`make verify-module-moves` exit 0 `OK (16 manifests verified)`；`go build ./...` exit 0（仅 cmd/desktop、cmd/server 既有 `ld: warning: ignoring duplicate libraries: '-lc++'`）。

## §别名（B2-DS.6，2026-09-27，空义务核销——12 行成对删除）

**授权依据**：Ruling LEGACY-ROW-OWNERSHIP「可早删不可晚删」+ K5.2 先例（`46447494a` 删义务行 / `a29abf40e` 三区出册补齐 / `55e13524a` 补回空键）。本节点为 12 行属主（`plan: 26-datasource`）。

### Step 1 逐条零 importer 复检（12/12 全 zero）

`python3` 读 `datasource.yaml` alias_obligations 12 条 old_import_path，逐条 `grep -rln "\"<path>\"" --include="*.go" . | grep -v _test.go`：

| # | old_import_path | 非测试 importer |
|---|---|---|
| 1 | internal/datasource | (zero non-test importer) |
| 2 | internal/datasource/connector/confluence | (zero non-test importer) |
| 3 | internal/datasource/connector/dingtalk | (zero non-test importer) |
| 4 | internal/datasource/connector/feishu/core | (zero non-test importer) |
| 5 | internal/datasource/connector/feishu/drive | (zero non-test importer) |
| 6 | internal/datasource/connector/feishu/wiki | (zero non-test importer) |
| 7 | internal/datasource/connector/gitlab | (zero non-test importer) |
| 8 | internal/datasource/connector/ima | (zero non-test importer) |
| 9 | internal/datasource/connector/moauth | (zero non-test importer) |
| 10 | internal/datasource/connector/notion | (zero non-test importer) |
| 11 | internal/datasource/connector/rss | (zero non-test importer) |
| 12 | internal/datasource/connector/yuque | (zero non-test importer) |

- `ls internal/datasource` → `No such file or directory`（复跑留档，与计划 §2.4 实测一致）。
- 广义核验（§2.4 同口径）：`grep -rn "WeKnora/internal/datasource" --include="*.go" internal cmd | grep -v modules/datasource` → 0 行（exit 1）。

### Step 2 成对删行 + 三区出册补齐（同窗同 commit）

- **manifest**（`docs/architecture/moves/datasource.yaml`）：alias_obligations 12 行 → 空列表 `[]`（`55e13524a` 同型，保留键）；**机械缺口就地补齐**（K5.2 同因）：`tools/modulemove/verify.go:108-124` 强制 move_packages[].from 与 alias_obligations 1:1 成对，删义务行后 from 仍在册 → `alias-1to1` 违规；故 move_packages 12 条 from/to → `[]`、owned_files.move_sources 12 条 → `[]`（verify.go:295 集合一致）、move_targets 24 条（12 to 树 + 12 旧路径别名登记 + 注释段）→ `[]`；importers 区 12 条 `internal/datasource/connector/*` 旧路径行随迁删（旧路径物理不存在；verify.go 无 importers 校验；终态满足计划 Step 3 `grep -c` 归零口径），保留 3 条真实宿主 importer（application/service、container、handler——compat shim 所在）。
- **matrix**（`docs/architecture/passb/ownership-matrix.yaml`）：aliases 区 12 条 `old_import_path: internal/datasource*`（`plan: 26-datasource, delete_barrier: ib2`，删行时 :2456-:2491）成对删除。

### Step 3 验证（命令原文 + 退出码）

| 命令 | 退出码 | 输出 |
|---|---|---|
| `grep -c "internal/datasource" docs/architecture/moves/datasource.yaml` | 1（无匹配） | `0`（计划口径达成） |
| `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)`（双侧奇偶） |
| `make check-backend-architecture` | 0 | `total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`、`OK (0 violations)` |
| `grep -n "internal/datasource" docs/architecture/passb/ownership-matrix.yaml` | 1（无匹配） | 0 行（matrix 侧同步归零） |
| `go test ./tools/modulemove/... -count=1` | 0 | `ok` |
| `go test ./tools/passbguard/... -count=1` | FAIL | `TestRealRepoExceptionLedgerPlansMatchGuardTasks`：`guard 源 PassBTask "B2-DS.5" 缺模块映射`——**BASE 预存**（stash 复跑实证，90b93f321 树同 FAIL），B2-DS.5 已登记移交 IB2 债务（§计数基线「+1 新增」段），非本任务引入；本任务改动仅 manifest/matrix 数据行，与该测试断言的 PassBTaskModule 映射无交集 |

**结论**：12 条空义务全核销，manifest/matrix 双侧归零，guard 双绿，奇偶无破坏。
