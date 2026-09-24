# b2-ac-market（25c Marketplace）证据 — Pass B

> 节点：`b2-ac-market`（work 节点，serial 于 b2-ac-definition / b2-ac-skills 之后；IB2 按 25a→25b→25c 集成）
> 分支：`codex/passb-b2-ac-market`；worktree：`.worktrees/passb-b2-ac-market`
> 本文件由 T1 建骨架并落基线记录；等价双跑比对结论与差分三锚定于 T5 补齐。

## 0. 基线对齐（Ruling 2026-09-24-WAVE-DEP-BASELINE）

派发基线 `dfcec6067`（= integration 头 `e586552d1` + 本计划 docs commit）**不含** 25a/25b 模块树（实跑：`internal/modules/agentcatalog/` 仅 B0 脚手架 3 文件；`git merge-base --is-ancestor` 实测 25a `938087598` 与 25b `5ed64d324` 均非 HEAD 祖先；两分支互不为祖先，merge-base `8c45a8815`——当时仓库不存在已含两者的 commit）。经协调者裁定 **Ruling 2026-09-24-WAVE-DEP-BASELINE（选项 A）**：在本节点分支按框架顺序做**基线对齐 merge**（非集成合并，不触碰 `codex/passb-integration`，IB2 正式职责不变）：

| 动作 | commit | 内容 |
|---|---|---|
| merge 25a | `ad53c2185` | `codex/passb-b2-ac-definition`（938087598）→ 零冲突 |
| merge 25b | `c0ddf768a` | `codex/passb-b2-ac-skills`（5ed64d324）→ 唯一冲突 `docs/architecture/passb/execution-ledger.md`（纯追加型台账条目，机械解决：HEAD 侧 b0 01:46 条目 + 25b 侧 T3 垫片/回滚两条**全保留**，时序排列；裁定 §3 授权范围内，无签名/注册/语义取舍） |

**对齐后基线（本节点任务起点）= `c0ddf768abc8e0f61e643bbdfba41a1606251fc6`**。后续 T2–T5 的 `PASSB_BASE_SHA` 一律取本值；tail OCR 将重复覆盖 25a/25b diff（裁定 §4 已接受，不为省它动台账）。

## 1. 前置门核验（T1 Step 2）

- **门面存在性**：对齐后 `internal/modules/agentcatalog/service/tenant_skill_install.go` 与宿主残差 `internal/application/service/tenant_skill_service.go` 均在；别名行实测 `tenant_skill_service.go:48` = `CatalogInstallResult      = acatsvc.CatalogInstallResult`（gofmt 对齐多空格）。
- **逐字命令链 vs 实况（如实记录）**：计划前置条件 2 的字面链 `grep -q "CatalogInstallResult = "` 输出 **FACADES-MISSING**——单空格模式不匹配 gofmt 对齐后的多空格排版，属字面模式过严，非门面缺失；空白容忍等价链 `grep -Eq "CatalogInstallResult[[:space:]]+="` 输出 **FACADES-OK**。语义判定：门面在位，前置门通过（两跑均留痕，T5 报告沿用本结论）。

## 2. 基线门禁（T1 Step 3，对齐基线 c0ddf768a 实跑，`-count=1`）

| # | 命令 | 退出码 | 关键输出 |
|---|---|---|---|
| 1 | `go build ./...` | 0 | 仅 `cmd/desktop`、`cmd/server` 两条 `ld: warning: ignoring duplicate libraries: '-lc++'`（非错误，与计划前置条件 4 预期一致） |
| 2 | `go test -count=1 ./internal/application/repository -run 'TestAgentMarketplace\|TestExpertInstall\|TestPublishedExpertRepo\|TestPublishedSkillRepo'` | 0 | `ok ... 14.425s` |
| 3 | `go test -count=1 ./internal/application/service -run 'TestAgentMarketplaceSubmit\|…\|TestTenantMarket'`（七段原式） | 0 | `ok ... 2.446s` |
| 4 | `go test -count=1 ./internal/router -run 'TestTenantAgentMarketplaceLifecycleAndAuthorization'` | 0 | `ok ... 2.136s` |
| 5 | `go test -count=1 ./internal/modules/agentcatalog/...` | 0 | 根 `[no test files]`；`handler ok 0.980s`、`repository ok 1.555s`、`service ok 7.126s`（25a/25b 随迁测试经对齐后全绿） |
| 6 | `make check-backend-architecture` | 0 | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16` + `OK (0 violations)`（计数零漂移） |
| 7 | `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |

## 3. 等价双跑「旧实现」侧基线（T1 Step 4，`-count=1 -v`，39 用例全 PASS）

搬迁前（模块侧文件尚不存在，被测实现 = 宿主原文件）用例清单；T5 终态同命令复跑逐用例比对（§5 高风险差分 ①）。

### 3.1 repository 侧 21 用例（`go test -count=1 -v ./internal/application/repository -run '…'`，exit 0）

1. TestAgentMarketplaceSubmissionReviewPublishAndTenantScope
2. TestAgentMarketplacePublishRejectsStaleDigestAndPointer
3. TestAgentMarketplaceReviewRejectDoesNotPublish
4. TestAgentMarketplaceFirstApprovedSubmissionOwnsListingMetadata
5. TestAgentMarketplaceSubmissionIgnoresCallerReleasePointer
6. TestAgentMarketplaceSubmissionRequiresMatchingVersionAgent
7. TestAgentMarketplaceTenantIsolationWithPopulatedTenants
8. TestAgentMarketplaceSQLiteDownWithPublishedListing
9. TestAgentMarketplaceConcurrentPublishingUsesUniqueReleaseNumbers
10. TestExpertInstallUpsertIsIdempotentPerTenantSlug
11. TestExpertInstallListIsTenantScopedAndOrdered
12. TestExpertInstallGetByTenantAndSlug
13. TestExpertInstallDeleteIsSoftAndSlotFrees
14. TestPublishedExpertRepoUpsertIsIdempotentPerAgent
15. TestPublishedExpertRepoListIsTenantScoped
16. TestPublishedExpertRepoGetByIDIsTenantScoped
17. TestPublishedExpertRepoDeleteSoftDeletesAndFreesScope
18. TestPublishedSkillRepoUpsertIsIdempotentPerScope
19. TestPublishedSkillRepoListIsTenantScoped
20. TestPublishedSkillRepoGetIsTenantScoped
21. TestPublishedSkillRepoDeleteSoftDeletesAndFreesScope

### 3.2 service 侧 17 用例（同命令族，exit 0）

1. TestAgentMarketplaceSubmitBindsFrozenSnapshotAndLocksEveryPayloadReference
2. TestAgentMarketplaceSubmitRejectsAnyUnlockedPayloadReference
3. TestAgentMarketplaceReviewRejectRequiresReasonAndStaleDigestDoesNotPublish
4. TestAgentMarketplaceApprovalStagesVerifiedReleaseBytesAndRetryIsIdempotent
5. TestAgentMarketplaceApprovalCompensatesFinalObjectWhenTransactionFails
6. TestAgentMarketplaceConcurrentApprovalFailureKeepsPeerCommittedBundle
7. TestAgentMarketplaceExclusivePublishVerifiesExistingDigest
8. TestAgentMarketplaceCatalogHidesPendingListing
9. TestTenantMarketPublishCreatesRowAndRefreshesOnRepublish
10. TestTenantMarketPublishUnknownOrWrongTenantCatalogIs404
11. TestTenantMarketUnpublishSoftDeletesAndIsIdempotent
12. TestTenantMarketUnpublishClearsOrphanedRowAfterCatalogDelete
13. TestTenantMarketListJoinsCatalogPublisherAndInstallState
14. TestTenantMarketListSkipsRowsWithoutLiveCatalog
15. TestTenantMarketInstallDelegatesToCatalogPipeline
16. TestTenantMarketInstallRequiresPublishedCatalog
17. TestTenantMarketInstallMapsPartialFailures

### 3.3 router 侧 1 用例（HTTP 面，经宿主构造链，exit 0）

1. TestTenantAgentMarketplaceLifecycleAndAuthorization（1.53s）

## 4. 差分锚定备注（T5 填结论）

- §5 高风险差分 ①（38+1 用例双跑）：基线 = 本文件 §3；终态待 T5。
- §5 高风险差分 ②（发布重试/幂等，`ReviewAndPublishTx` 唯一冲突重试路径）：本基线含 repo `…ConcurrentPublishingUsesUniqueReleaseNumbers`、service `…ConcurrentApprovalFailureKeepsPeerCommittedBundle`、路由全生命周期用例；模块本地 `isUniqueViolation` 副本落位后由 parity 测试 + 并发用例双锚定（T2）。
- §5 高风险差分 ③（注入位）：宿主路由测试经残差构造链绑真源 `experts.BuildAgentReleaseBundle`；模块 service 测试同源注入（T3 落地后复跑）。
