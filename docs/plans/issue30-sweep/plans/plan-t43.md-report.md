# plan-t43（T13 合规访问、Retention 与安全删除，Issue #43）第 8/8 任务报告

- **执行 worktree**：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t43`（分支 `codex/issue30-t43`）
- **执行日期**：2026-09-25
- **任务定位**：Issue #43 实施计划（`docs/plans/issue30-sweep/plans/plan-t43.md`）的第 8/8 个任务——Task 0-7 已由前序任务各自提交，本任务为**计划级收敛验证**：逐字执行计划末尾「计划级验证命令（testCommand）」，确认全部交付物在真实迁移 + 真实代码路径下收敛，并产出本最终报告。本任务**不新增生产代码**（计划 Task 7 之后无实现任务；计划文件结构总览中列出的最后一项交付物 `task_compliance_e2e_test.go` 已由 Task 7 提交）。

## 一、开工前状态核对

进入 worktree 后实测（非假设）：

- `git status`：`nothing to commit, working tree clean`（分支 `codex/issue30-t43`）。
- `git log --oneline` 确认 Task 0-7 的 8 个提交均在（见第四节提交列表）。
- 关键交付文件逐一 `ls` 在位：`internal/types/task_compliance.go`、`internal/application/repository/task_compliance_store.go`、`internal/application/service/task_compliance.go`、`internal/handler/session/workbench_task_compliance.go`、`internal/handler/session/task_deletion_guard_test.go`、`internal/handler/session/task_compliance_e2e_test.go`、`internal/container/task_compliance_wiring_test.go`。
- Task 0 迁移去重核实：`migrations/sqlite/000113_agent_adoption_variants.up.sql` 与 `migrations/versioned/000192_agent_adoption_variants.up.sql` 存在；旧同号文件 `migrations/sqlite/000112_agent_adoption_variants.up.sql`、`migrations/versioned/000191_agent_adoption_variants.up.sql` 已不存在（`ls` 报 `No such file or directory`）——同号双文件已消除，golang-migrate 全量装载前提成立。

## 二、测试命令与完整输出

### 命令 1：计划级验证命令（testCommand）——构建 + 四个受影响包定向测试

在 worktree 根逐字执行计划「计划级验证命令（testCommand）」一节规定的四段链式命令（分四步跑以便留证，命令原文逐段未改动；`go build ./...` 为链条首段）：

```bash
go build ./...
```

输出（尾部）：

```
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
（退出码 0，BUILD_OK）
```

说明：`ld: warning: ignoring duplicate libraries: '-lc++'` 是 darwin 工具链对 `-lc++` 重复出现的既有链接警告，非错误、非本计划引入（`cmd/desktop`、`cmd/server` 链接目标），构建成功。

```bash
go test ./internal/application/repository/ -run 'TestTaskCompliance|TestTaskPolicyStore|TestOpenAndActive|TestTaskMetadataFacts|TestListTaskMessages|TestPurgeTask' -v
```

完整逐测试输出（首轮）：

```
--- PASS: TestTaskComplianceTablesExistAfterMigrations (1.17s)
--- PASS: TestTaskPolicyStoreMissingReturnsNil (0.98s)
--- PASS: TestTaskPolicyStoreUpsertRoundTrip (0.92s)
--- PASS: TestOpenAndActiveComplianceAccessWindow (1.02s)
--- PASS: TestTaskMetadataFactsProjectsMetadataOnly (1.54s)
--- PASS: TestPurgeTaskDeletesInternalRowsOnlyKeepsAudit (1.01s)
--- PASS: TestListTaskMessagesScopedToSession (1.25s)
PASS
ok  	github.com/Tencent/WeKnora/internal/application/repository	(cached)
```

```bash
go test ./internal/application/service/ -run 'TestSetTaskPolicyRequiresAdmin|TestRequestContentAccessValidates|TestReadTaskContentRequires|TestComplianceAuditFailClosed|TestTaskMetadataIsAdminOnly|TestAllowsTaskDeletionUnderLegalHold|TestPurgeTaskPolicyChain' -v
```

完整逐测试输出（真实执行，非缓存）：

```
--- PASS: TestSetTaskPolicyRequiresAdminAndValidates (0.00s)
--- PASS: TestRequestContentAccessValidatesReasonAndTTL (0.00s)
--- PASS: TestReadTaskContentRequiresActiveWindow (0.00s)
--- PASS: TestComplianceAuditFailClosed (0.00s)
--- PASS: TestTaskMetadataIsAdminOnlyAndCarriesNoContent (0.00s)
--- PASS: TestAllowsTaskDeletionUnderLegalHold (0.00s)
--- PASS: TestPurgeTaskPolicyChain (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/application/service	3.182s
```

```bash
go test ./internal/handler/session/ -run 'TestCompliancePolicyEndpoints|TestComplianceAccessEndpoints|TestComplianceHandlerMapsErrorCodes|TestCompliancePurgeEndpoint|TestSessionDeleteBlockedByLegalHold|TestBatchAndClearBlockedByLegalHold|TestDeletionGuardNilKeepsFlow|TestDeleteSessionTombstonesCraftResources|TestComplianceEndToEnd' -v
```

完整逐测试输出（真实执行，非缓存）：

```
=== RUN   TestDeleteSessionTombstonesCraftResources
--- PASS: TestDeleteSessionTombstonesCraftResources (3.37s)
=== RUN   TestComplianceEndToEndAC1IndependentFlow
--- PASS: TestComplianceEndToEndAC1IndependentFlow (1.37s)
=== RUN   TestComplianceEndToEndAC2DeletionNeverCascadesOutward
--- PASS: TestComplianceEndToEndAC2DeletionNeverCascadesOutward (1.52s)
=== RUN   TestSessionDeleteBlockedByLegalHold
--- PASS: TestSessionDeleteBlockedByLegalHold (3.23s)
=== RUN   TestBatchAndClearBlockedByLegalHold
--- PASS: TestBatchAndClearBlockedByLegalHold (1.45s)
=== RUN   TestDeletionGuardNilKeepsFlowAndInfraErrorFailsClosed
--- PASS: TestDeletionGuardNilKeepsFlowAndInfraErrorFailsClosed (7.02s)
=== RUN   TestCompliancePolicyEndpoints
--- PASS: TestCompliancePolicyEndpoints (0.00s)
=== RUN   TestComplianceAccessEndpoints
--- PASS: TestComplianceAccessEndpoints (0.00s)
=== RUN   TestComplianceHandlerMapsErrorCodes
=== RUN   TestComplianceHandlerMapsErrorCodes/bad_request
=== RUN   TestComplianceHandlerMapsErrorCodes/forbidden
=== RUN   TestComplianceHandlerMapsErrorCodes/not_found
=== RUN   TestComplianceHandlerMapsErrorCodes/legal_hold
=== RUN   TestComplianceHandlerMapsErrorCodes/internal
--- PASS: TestComplianceHandlerMapsErrorCodes (0.00s)
=== RUN   TestCompliancePurgeEndpoint
--- PASS: TestCompliancePurgeEndpoint (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler/session	20.984s
```

```bash
go test ./internal/container/ -run TestTaskComplianceWiring -v
```

输出（首轮缓存命中）：

```
=== RUN   TestTaskComplianceWiringRegistered
--- PASS: TestTaskComplianceWiringRegistered (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/container	(cached)
```

**首轮合计：25/25 PASS，0 FAIL**（repository 7 + service 7 + handler 10 + container 1）。

### 命令 2：消除缓存疑虑——`-count=1` 强制真实重跑

首轮 repository 段与 container 段显示 `(cached)`。Go test 缓存条目本身由同输入的真实运行产生，但为使证据无懈可击，对这两段强制 `go test -count=1 ...` 真实重跑：

```bash
go test -count=1 ./internal/application/repository/ -run 'TestTaskCompliance|TestTaskPolicyStore|TestOpenAndActive|TestTaskMetadataFacts|TestListTaskMessages|TestPurgeTask' -v
```

```
--- PASS: TestTaskComplianceTablesExistAfterMigrations (1.79s)
--- PASS: TestTaskPolicyStoreMissingReturnsNil (1.37s)
--- PASS: TestTaskPolicyStoreUpsertRoundTrip (1.26s)
--- PASS: TestOpenAndActiveComplianceAccessWindow (1.13s)
--- PASS: TestTaskMetadataFactsProjectsMetadataOnly (3.23s)
--- PASS: TestPurgeTaskDeletesInternalRowsOnlyKeepsAudit (1.16s)
--- PASS: TestListTaskMessagesScopedToSession (1.07s)
PASS
ok  	github.com/Tencent/WeKnora/internal/application/repository	12.569s
```

```bash
go test -count=1 ./internal/container/ -run TestTaskComplianceWiring -v
```

```
=== RUN   TestTaskComplianceWiringRegistered
--- PASS: TestTaskComplianceWiringRegistered (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/container	3.775s
```

非缓存重跑同样 25/25 全绿。

### 验收标准对照（基于本次实跑的测试）

- **AC1（合规访问不把管理员加入 Task 协作列表）**：`TestComplianceEndToEndAC1IndependentFlow` PASS——e2e 中管理员开窗前后 `task_grants` 零行、owner 的 grants 列表不含 admin、run 读面两次 404（admin 角色与合规窗口都不授予协作身份）、过期后内容重封 403、非 admin 全端点 403。底层支撑：service 端口层不持有 grant 写路径（类型级结构性保证，Task 3 交付）。
- **AC2（删除内部 Task 不隐式删除外部文档、代码或审计）**：`TestComplianceEndToEndAC2DeletionNeverCascadesOutward` PASS——legal hold 下删除 409 且留痕 `task.delete_denied`；归档不受 hold 影响（真实 #34 archive handler 200）；软删除后 `craft_workspaces`/`messages` 行保留、审计只增；保留期内 purge 409；过保留期 purge 后五张内部表行清零而 `audit_logs` 只增（`task.purged` 追加）；purge 后再 purge 得统一 404。
- **AC3（端到端行为通过最高稳定 Interface 验证）**：两个 e2e 测试运行在真实全量迁移 sqlite（`openCraftHTTPDB`）+ 真实 store/service/handler + 真实审计（`repository.NewAuditLogRepository` + `service.NewAuditLogService`）之上，零 service mock 冒充；已知边界 `sqlBackedSessions`（最小 session-service 表面）在计划差异记录第 8 条中明示。

## 三、实现内容（本任务范围）

本任务无新增/修改的生产或测试代码——计划 Task 0-7 已交付全部文件（计划「文件结构总览」13 项逐一在位，见第一节核对）。本任务的工作是：

1. 核对前序 8 个任务的提交链与交付文件完整性（第一节）。
2. 逐字执行计划级验证命令（第二节），确认三条验收标准的最终证据在当前 HEAD 收敛。
3. 对缓存命中的两段补 `-count=1` 强制真实重跑（第二节命令 2）。
4. 产出本报告并提交。

## 四、提交

前序任务提交链（本任务开工前已存在于分支，本次仅核对未改动）：

| SHA | 主题 | 任务 |
|---|---|---|
| `a8559335e` | fix(migrations): dedupe adoption migration version (000191->000192 / 000112->000113) broken by b3 merge | Task 0 |
| `d5d1a57b9` | feat(compliance): tenant task policy tables + policy store (t13 #43 task 1) | Task 1 |
| `3bd2a8b2d` | feat(compliance): access windows + metadata/content projections (t13 #43 task 2) | Task 2 |
| `1ec983a39` | feat(compliance): reasoned+time-limited+audited access flow, fail-closed audit (t13 #43 task 3) | Task 3 |
| `869e5fce9` | fix(compliance): 卫兵声明改为如实——reflect 字段白名单承担真正的 content-free 强制 (t13 #43 task 3 fix 1) | Task 3 fix |
| `d7e39c424` | refactor(compliance): narrow store port to implemented six methods until Task 6 (ruling via escalation) | Task 4 前裁决 |
| `47dd17b81` | feat(compliance): admin HTTP surface + routes + dig providers (t13 #43 task 4) | Task 4 |
| `50c801dce` | feat(compliance): legal-hold deletion gate on all three delete entrances (t13 #43 task 5) | Task 5 |
| `b287d7662` | feat(compliance): policy-driven permanent deletion end-to-end (t13 #43 task 6) | Task 6 |
| `3917766aa` | test(compliance): end-to-end AC1/AC2 evidence over real migrations + real handlers (t13 #43 task 7) | Task 7 |

本任务新增提交：本报告文件一个（提交后补记 SHA，见 git log）。

## 五、自检发现

1. **`(cached)` 已消除**：首轮 repository/container 两段命中 Go test 缓存。缓存条目由真实运行产生，不算伪造，但为证据强度补跑了 `-count=1`，两段在非缓存下同样全绿（第二节命令 2）。报告以非缓存输出为准。
2. **链接警告如实记录**：`go build ./...` 输出 `ld: warning: ignoring duplicate libraries: '-lc++'`（`cmd/desktop`、`cmd/server`）。这是 darwin 工具链既有警告，非本计划引入、非失败；不掩盖、不计入测试失败。
3. **计划未要求 `go test ./...` 全量**：计划「计划级验证命令」一节明确说明不跑全量套件（避免无关 flaky），回归面由 `go build ./...` 全仓编译 + 四个受影响包定向测试覆盖。本次照此执行，未擅自扩大或缩小范围。
4. **主 worktree 与本 worktree 的区分**：主 worktree（`/Users/wuyongjun/trea/WeKnora-fork01`）存在未提交的 `craft_test.go` 修改等，属他人/其他任务工作面，本任务全程只在 `issue30-sweep-t43` worktree 内操作，未触碰。
5. **无阻塞项**：三条验收标准的最终证据（两个端到端测试）在本任务实跑中真实通过；Task 0-7 无遗留缺口。

## 六、结论

计划级验证命令（testCommand）全部收敛：`go build ./...` 成功 + 25/25 定向测试 PASS（含 AC1/AC2 两个端到端集成证据测试）。plan-t43 的 8 个任务（Task 0-7）全部交付完毕，产出可供 #71 证据矩阵消费。
