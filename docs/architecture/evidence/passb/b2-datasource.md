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
# MERGE_EXIT=0，无冲突（计划预判的 docs/plans/passb 冲突未发生：本侧 26-datasource.md 为本侧独有新增、K 系列 plan 文件为 K 侧独有新增，两侧文件集不相交，git 自动合并；internal/modules/knowledge/module.go 无冲突）
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
grep -c "^func RecordKBActivity|^func KBActivityTrigger|^func WithKBActivityTask|^func WithKBActivitySuppressed" internal/modules/knowledge/retrieval/app/kb_activity.go
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

## §差分（占位——T3/T4 包级复跑为每个删除 commit 的即时门，B2-DS.7 四面×四要素定稿填写）

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

## §计数基线（占位——B2-DS.5 例外行登记、B2-DS.7 三方一致复核时填写；本任务 P-6 快照已录 633/23+23/58/16、16 manifests、0 violations）

## §别名（占位——B2-DS.6 填写）
