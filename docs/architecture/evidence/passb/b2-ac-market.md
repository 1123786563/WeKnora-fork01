# b2-ac-market（25c Marketplace）证据 — Pass B

> 节点：`b2-ac-market`（work 节点，serial 于 b2-ac-definition / b2-ac-skills 之后；IB2 按 25a→25b→25c 集成）
> 分支：`codex/passb-b2-ac-market`；worktree：`.worktrees/passb-b2-ac-market`
> 本文件由 T1 建骨架并落基线记录；等价双跑比对结论与差分三锚定于 T5 补齐。

## 0. 基线对齐（Ruling 2026-09-24-WAVE-DEP-BASELINE）

派发基线 `dfcec6067`（= integration 头 `e586552d1` + 本计划 docs commit）**不含** 25a/25b 模块树（实跑：`internal/agentcatalog/` 仅 B0 脚手架 3 文件；`git merge-base --is-ancestor` 实测 25a `938087598` 与 25b `5ed64d324` 均非 HEAD 祖先；两分支互不为祖先，merge-base `8c45a8815`——当时仓库不存在已含两者的 commit）。经协调者裁定 **Ruling 2026-09-24-WAVE-DEP-BASELINE（选项 A）**：在本节点分支按框架顺序做**基线对齐 merge**（非集成合并，不触碰 `codex/passb-integration`，IB2 正式职责不变）：

| 动作 | commit | 内容 |
|---|---|---|
| merge 25a | `ad53c2185` | `codex/passb-b2-ac-definition`（938087598）→ 零冲突 |
| merge 25b | `c0ddf768a` | `codex/passb-b2-ac-skills`（5ed64d324）→ 唯一冲突 `docs/architecture/passb/execution-ledger.md`（纯追加型台账条目，机械解决：HEAD 侧 b0 01:46 条目 + 25b 侧 T3 垫片/回滚两条**全保留**，时序排列；裁定 §3 授权范围内，无签名/注册/语义取舍） |

**对齐后基线（本节点任务起点）= `c0ddf768abc8e0f61e643bbdfba41a1606251fc6`**。后续 T2–T5 的 `PASSB_BASE_SHA` 一律取本值；tail OCR 将重复覆盖 25a/25b diff（裁定 §4 已接受，不为省它动台账）。

## 1. 前置门核验（T1 Step 2）

- **门面存在性**：对齐后 `internal/agentcatalog/service/tenant_skill_install.go` 与宿主残差 `internal/application/service/tenant_skill_service.go` 均在；别名行实测 `tenant_skill_service.go:48` = `CatalogInstallResult      = acatsvc.CatalogInstallResult`（gofmt 对齐多空格）。
- **逐字命令链 vs 实况（如实记录）**：计划前置条件 2 的字面链 `grep -q "CatalogInstallResult = "` 输出 **FACADES-MISSING**——单空格模式不匹配 gofmt 对齐后的多空格排版，属字面模式过严，非门面缺失；空白容忍等价链 `grep -Eq "CatalogInstallResult[[:space:]]+="` 输出 **FACADES-OK**。语义判定：门面在位，前置门通过（两跑均留痕，T5 报告沿用本结论）。

## 2. 基线门禁（T1 Step 3，对齐基线 c0ddf768a 实跑，`-count=1`）

| # | 命令 | 退出码 | 关键输出 |
|---|---|---|---|
| 1 | `go build ./...` | 0 | 仅 `cmd/desktop`、`cmd/server` 两条 `ld: warning: ignoring duplicate libraries: '-lc++'`（非错误，与计划前置条件 4 预期一致） |
| 2 | `go test -count=1 ./internal/application/repository -run 'TestAgentMarketplace\|TestExpertInstall\|TestPublishedExpertRepo\|TestPublishedSkillRepo'` | 0 | `ok ... 14.425s` |
| 3 | `go test -count=1 ./internal/application/service -run 'TestAgentMarketplaceSubmit\|…\|TestTenantMarket'`（七段原式） | 0 | `ok ... 2.446s` |
| 4 | `go test -count=1 ./internal/router -run 'TestTenantAgentMarketplaceLifecycleAndAuthorization'` | 0 | `ok ... 2.136s` |
| 5 | `go test -count=1 ./internal/agentcatalog/...` | 0 | 根 `[no test files]`；`handler ok 0.980s`、`repository ok 1.555s`、`service ok 7.126s`（25a/25b 随迁测试经对齐后全绿） |
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

## 4. 等价双跑（T5 Step 1，2026-09-25 终态实跑；conventions §6 格式）

> 旧实现侧 = 本文件 §3（T1 Step 4，宿主原文件被测，39 用例全 PASS）；新实现侧 = 分支 HEAD `53646cb10`（T2/T3/T4 搬迁完成后）。随迁使用例物理位置从宿主两包转至模块两包，同 `-run` 模式改指模块包；宿主侧同模式复跑为 `[no tests to run]`（25c.3 复核轮命令 2 在案），即用例零丢失、零复制双跑。

### 4.1 新实现侧命令与输出（全部本会话实跑，`-count=1 -v`）

| # | 命令 | 退出码 | 输出摘要 |
|---|---|---|---|
| 1 | `go test -count=1 -v ./internal/agentcatalog/repository -run 'TestAgentMarketplace\|TestExpertInstall\|TestPublishedExpertRepo\|TestPublishedSkillRepo'` | 0 | 21 用例全 `--- PASS`（§3.1 同名清单），`ok 7.332s`（首轮 `9.866s`） |
| 2 | `go test -count=1 -v ./internal/agentcatalog/service -run 'TestAgentMarketplaceSubmit\|TestAgentMarketplaceReview\|TestAgentMarketplaceApproval\|TestAgentMarketplaceConcurrent\|TestAgentMarketplaceExclusive\|TestAgentMarketplaceCatalog\|TestTenantMarket'`（七段原式） | 0 | 17 用例全 `--- PASS`（§3.2 同名清单），`ok 1.478s` |
| 3 | `go test -count=1 -v ./internal/router -run 'TestTenantAgentMarketplaceLifecycleAndAuthorization'` | 0 | `--- PASS: TestTenantAgentMarketplaceLifecycleAndAuthorization (0.89s)`，`ok 2.657s` |
| 4 | `go test -count=1 -v ./internal/agentcatalog/repository -run 'TestIsUniqueViolationParity'` | 0 | 8/8 子用例全 PASS（nil/`gorm.ErrDuplicatedKey` 及 wrap/sqlite/postgres 消息/23505/无关错误/哨兵），`ok 1.010s` |

### 4.2 逐用例比对结论

- **repository 21 用例**：新旧两侧用例名逐条一致（§3.1 清单 1-21 ↔ 终态同名 21 条），双侧全 PASS。含并发唯一发布号 `TestAgentMarketplaceConcurrentPublishingUsesUniqueReleaseNumbers`（唯一冲突重试路径走模块本地 `isUniqueViolation` 副本）与 `TestAgentMarketplaceSQLiteDownWithPublishedListing`（机制适配注记见 4.4-②）。
- **service 17 用例**：新旧两侧用例名逐条一致（§3.2 清单 1-17 ↔ 终态同名 17 条），双侧全 PASS。含补偿 `…CompensatesFinalObjectWhenTransactionFails`、独占发布 `…ExclusivePublishVerifiesExistingDigest`、并发审批 `…ConcurrentApprovalFailureKeepsPeerCommittedBundle`（注入位改 `s.adapters.BuildReleaseBundle` 后期望值同源）。
- **router 1 用例**：`TestTenantAgentMarketplaceLifecycleAndAuthorization` 双侧 PASS——经宿主 shim 链零改动执行真实构造链 `NewAgentMarketplaceRepository`（shim var 转发）→ `NewAgentMarketplaceService`（残差 4 参构造器绑真源 `experts.BuildAgentReleaseBundle`）→ `NewAgentMarketplaceHandler`（shim var 转发），HTTP 面（路由/状态码/RBAC 守卫/错误码映射）等价。
- **计数奇偶**：21+17+1 = 39 用例两侧一致，无丢失、无新增、无跳过（无 SKIP）。

### 4.3 §5 高风险差分三项结论（conventions §6 / spec §14.3 口径）

1. **① 搬迁等价双跑（38 用例 + 路由 1 例）**：双侧全 PASS、用例名逐条一致（§4.2）——**通过**。
2. **② 发布重试/幂等差分（`ReviewAndPublishTx` 唯一冲突重试路径）**：repo 侧 `…ConcurrentPublishingUsesUniqueReleaseNumbers`、service 侧 `…ConcurrentApprovalFailureKeepsPeerCommittedBundle`、路由侧全生命周期用例三锚定搬迁后全绿；模块本地 `isUniqueViolation` 副本分类结果由 parity 表（8 子用例）+ 上述并发用例双锚定——**通过**。
3. **③ 注入位差分（`MarketHostAdapters.BuildReleaseBundle`）**：宿主路由测试经残差构造器绑真源 builder 全链路执行（命令 3），模块 service 17 用例同源注入（测试 8 处构造点均绑 `experts.BuildAgentReleaseBundle` 真源，25c.3 复核轮 §1 Step 2 diff 实证），两侧期望值同源——**通过**。

### 4.4 机制/装置差异注记（不影响用例级结论，如实登记）

1. **`openRunTestDB` 测试装置副本**（T2 偏差 1）：随迁 `agent_marketplace_test.go` 内的最小装置副本（sqlite 分支逐字 + `remove_at: ib2`，TEST-SUPPORT-SHIM 裁定族）——装置差异不进入被测行为面。
2. **down 迁移执行机制适配**（T2 偏差 2）：`TestAgentMarketplaceSQLiteDownWithPublishedListing` 旧侧动态 `db.Exec(string(down))`、新侧 migrate 引擎 `Steps(-1)`（同一份已入库 000109 down 文件）；断言零改动，用例级结论可比对（双侧 PASS；探针实证 before 109 clean → after 108、`agent_versions` 保留、`agent_releases` 删除）。
3. **`requireAppErrorStatus` 包内副本**（T3 偏差 1，:189-200，与留宿 `skill_market_service_test.go:292` 逐字同体）与 **`fakePublisherNames` 宿主垫片追加**（T3 偏差 2，`tenant_skill_testsupport_test.go` +26 行，Ruling 2026-09-24-TEST-SUPPORT-SHIM）——均为测试装置，不改变被测实现语义。

## 5. 终态门禁（T5 Step 2，分支 HEAD `53646cb10` 实跑）

| # | 命令 | 退出码 | 关键输出 |
|---|---|---|---|
| 1 | `go build ./...` | 0 | 仅 cmd/server、cmd/desktop 两条 `ld: warning: ignoring duplicate libraries: '-lc++'`（在案非错误） |
| 2 | `go test -count=1 ./internal/agentcatalog/...` | 0 | 根 `[no test files]`；handler `ok 1.781s`、repository `ok 11.508s`、service `ok 8.879s` |
| 3 | `make check-backend-architecture` | 0 | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16` + `OK (0 violations)`——计数零漂移 |
| 4 | `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |
