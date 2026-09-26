# T24 #54 Task 2 实现报告：提供者路由、派发器权威读回与服务编排核心测试

- worktree：`.worktrees/issue30-sweep-t54`（分支 `codex/issue30-t54`，起点 HEAD `f54ed2502` = Task 1 已提交）
- 授权文件内完成全部修改；未触碰任务外文件；无子代理；未推送远端。

## 一、实现内容

### 1. `internal/modules/codedelivery/service.go`（修改）

- `CodeDeliveryDeps` 增两字段（service.go:56-62 附近）：`GitLab CodePlatformClientFactory`、`Providers ProviderSource`；注释声明缺失适配器/提供者源一律 fail closed。
- `authorize` 签名改为三元返回 `(string, appconnector.Connection, error)`（原返回 sessionID 单值）；错误路径全部返回零值 `appconnector.Connection{}`；两个调用点（`MaterializeBaseline`、`PrepareDelivery`）同步改写。
- 新增 `platformProvider(ctx, conn)`：`deps.Providers == nil` → `ErrUnsupportedProvider`（"provider source not wired"）；经 `GetInstallationByID(conn.TenantID, conn.InstallationID)` 服务端权威解析安装 app id；app ∉ {github, gitlab} → `ErrUnsupportedProvider`（"connection app %q is not a code platform"）。
- 新增 `clientFor(provider, token, repo)`：委托 Task 1 的唯一平台 switch `clientForPlatform`。
- `MaterializeBaseline`：authorize → baselineSHALegal → **platformProvider** → tokenFor → **clientFor**（替换原 `client := s.deps.GitHub(token, in.Repo)`）；其余（MaxDeliveryFiles、WorkspaceRepoRoot、逐文件 Blob、maxBaselineBytes、WriteSessionWorkspaceFiles、BaselineReceipt）逐字未动。
- `PrepareDelivery`：同段替换（provider 解析 + clientFor 路由）；A03 锚定段删除原 `conn, err := s.deps.Connections.FindConnectionByID(...)` 四行重取（conn 已由 authorize 装载），`Target: DeliveryActionTarget` 改为 `Target: DeliveryTargetOf(provider)`——A03 行 target 从硬编码 `github.deliver` 变为按提供者权威生成（`gitlab.deliver`/`github.deliver`）。护栏 1/branchShapeLegal/Repository/BranchProtected/RefuseProtectedTarget/ValidateTaskBranch/diff/材料校验逐字未动。
- `DeliveryActionTarget` 常量定义保留（service.go:18，导出符号，全仓唯一使用点已替换；未使用导出常量不产生编译/vet 错误，最小 diff 原则不删）。

### 2. `internal/modules/codedelivery/dispatcher.go`（修改）

- `DispatcherDeps` 增 `GitLab CodePlatformClientFactory` 字段。
- 新增 `clientForTarget(target, token, repo)`：`ProviderOfTarget` 解析 target → `clientForPlatform` 路由——派发面唯一的平台 switch；未知 target 返回 `ErrUnsupportedProvider` 包装。
- `Dispatch` 前置门顺序调整为 parse → A02 → token → **平台路由** → delivery row（平台路由先于 row 读，均在出网前拒绝）。
- `RecoverPullRequest`：`d.deps.GitHub(token, material.Repo)` 替换为 `d.clientForTarget(snap.Target, token, material.Repo)`，错误直接返回。
- `QueryProvider`：同替换，错误包 `appconnectorsvc.ErrDispatchUnknown`（unknown 收敛面的路由失败也按不可观测处理）。
- `deliver` 推送半程尾部（关键行为增强）：原「RecordReceipts(commitSHA) → EnsureBranch」顺序对调为 **EnsureBranch → BranchHead 权威读回 → RecordReceipts(读回 head) → TransitionState(pushed)**；读回 `!pushed` 时返回 `ErrGitHubTransport` 包装（branch absent after push）——GitHub 上读回结果恒等于 CreateCommit 结果（既有测试相对断言全兼容），GitLab 上服务端自定 SHA 的本地占位值（`gl-commit-placeholder`）绝不进入台账。

### 3. `internal/modules/codedelivery/service_prepare_test.go`（夹具扩展，计划 7 处逐字）

① `deliveryFixture` 增 `gitlab *gitLabEmulator` 字段；② GitHub 连接行后追加 inst-gl/conn-gl 两行（AppID=gitlab、Personal、owner=u1、active、tenant 7）；③ `e := newGitHubEmulator(t)` 后装配 `gl := newGitLabEmulator(t)`、`gitlabFactory := NewGitLabClientFactory(http.DefaultClient, gl.srv.URL)`、`providers := appconnectorrepo.NewInstallationStore(db)`；④ dispatcher deps 增 `GitLab: gitlabFactory`；⑤ svc deps 增 `GitLab: gitlabFactory, Providers: providers`；⑥ return 追加 `gitlab: gl`；⑦ `LoadCredential` 按 `:gitlab` 后缀、`Resolve` 按 `-gl` 后缀返回 `glpat-testtoken`（夹具自约定假值，非真实凭据）。

### 4. `internal/modules/codedelivery/service_gitlab_test.go`（新建，核心组 4 测试 + 4 助手）

按计划逐字：`gitlabPrepareInput`/`seededGitLabFixture`/`snapshotGitLabCalls`/`mustBranchCommit`/`mustMaterialJSON` 助手；`TestGitLabDeliveryE2E_RecordsTraceableReceipts`（A03 行 target=gitlab.deliver、回执提交 SHA=读回远端 head、main 纹丝不动、远端身份/批准人落账）；`TestGitLabMaterializeBaselineWritesFixedTree`（基线物化走同一 seam）；`TestGitLabUnsupportedProviderFailsClosed`（Review Focus 1：notion 连接 prepare 面与 notion.deliver 派发面均 fail closed 零远端调用）；`TestGitLabTamperedSnapshotNeverReachesGitLab`（Review Focus 3：篡改快照派发前后调用计数恒等）。Task 3 的语义组测试不在本任务范围。

## 二、测试命令与完整输出（实跑证据）

**Step 2 RED**（写完测试未实现时）：

```
$ go test ./internal/modules/codedelivery/ -count=1
# github.com/Tencent/WeKnora/internal/modules/codedelivery [github.com/Tencent/WeKnora/internal/modules/codedelivery.test]
internal/modules/codedelivery/service_prepare_test.go:144:20: unknown field GitLab in struct literal of type DispatcherDeps
internal/modules/codedelivery/service_prepare_test.go:151:20: unknown field GitLab in struct literal of type CodeDeliveryDeps
internal/modules/codedelivery/service_prepare_test.go:151:43: unknown field Providers in struct literal of type CodeDeliveryDeps
FAIL	github.com/Tencent/WeKnora/internal/modules/codedelivery [build failed]
```

**Step 4 GREEN——Task 2 核心组四测**（`go test ./internal/modules/codedelivery/ -count=1 -v -run 'TestGitLabDeliveryE2E|TestGitLabMaterializeBaseline|TestGitLabUnsupportedProvider|TestGitLabTamperedSnapshot'`）：

```
=== RUN   TestGitLabDeliveryE2E_RecordsTraceableReceipts
--- PASS: TestGitLabDeliveryE2E_RecordsTraceableReceipts (0.04s)
=== RUN   TestGitLabMaterializeBaselineWritesFixedTree
--- PASS: TestGitLabMaterializeBaselineWritesFixedTree (0.02s)
=== RUN   TestGitLabUnsupportedProviderFailsClosed
--- PASS: TestGitLabUnsupportedProviderFailsClosed (0.02s)
=== RUN   TestGitLabTamperedSnapshotNeverReachesGitLab
--- PASS: TestGitLabTamperedSnapshotNeverReachesGitLab (0.03s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/codedelivery	2.189s
```

**计划指定检查**（`go test ./internal/modules/codedelivery/ -count=1`）：

```
ok  	github.com/Tencent/WeKnora/internal/modules/codedelivery	3.278s
```

全量 `-v` 统计：**36 PASS / 0 FAIL / 2 SKIP**。SKIP 均为 env 门控 blocked-env 测试（`TestGitHubClientAgainstRealGitHub`、`TestGitLabClientAgainstRealGitLab`，缺 env 如实 skip，与计划预期一致），未伪造通过。

**权威读回对既有 GitHub 测试的兼容证据**（`go test ./internal/modules/codedelivery/ -count=1 -v -run 'TestDispatch|TestPartialPush|TestUnknown|TestSecondDelivery|TestPreSendGate|TestPrepareDelivery|TestMaterializeBaseline|TestStoreState'`）：

```
--- PASS: TestDispatchDeliversAndRecordsTraceableReceipts (0.04s)
--- PASS: TestDispatchNeverWritesProtectedBranchOrMerges (0.03s)
--- PASS: TestPartialPushPRFailureRecoversWithoutRepush (0.03s)
--- PASS: TestDispatchRefusesSecondDeliveryPreparedWithDifferentFiles (0.03s)
--- PASS: TestDispatchWithoutApprovalConsumesNothing (0.02s)
--- PASS: TestUnknownOutcomeResolvesFromRemoteFacts (0.03s)
--- PASS: TestDispatchFailsClosedWhenConnectionUnusable (0.02s)
--- PASS: TestSecondDeliveryOnSameTaskBranchFastForwards (0.04s)
--- PASS: TestPreSendGateFailureSettlesDeliveryFailedNotStranded (0.03s)
--- PASS: TestMaterializeBaselineWritesFixedTreeIntoWorkspace (0.02s)
--- PASS: TestMaterializeBaselineRejectsNonOwnerAndForeignConnection (0.02s)
--- PASS: TestPrepareDeliveryAnchorsApprovalAndDiff (0.03s)
--- PASS: TestPrepareDeliveryRefusesProtectedBranchWithZeroRemoteWrites (0.01s)
--- PASS: TestPrepareDeliveryRejectsIllegalBranchShapeBeforeAnyRemoteRead (0.01s)
--- PASS: TestPrepareDeliveryAlwaysAwaitsApprovalEvenWithWritePreAuthorization (0.02s)
--- PASS: TestStoreStateConstantsMirrorDeliveryState (0.00s)
ok  	github.com/Tencent/WeKnora/internal/modules/codedelivery	1.925s
```

**全仓编译**（`go build ./...`）：`BUILD_OK`（仅无关的 `cmd/desktop`/`cmd/server` ld duplicate-library 警告，改动前即存在）。容器 `code_delivery.go` 未接线新字段——按计划⑩，新字段零值（nil）是 GitHub-only 部署的合法现状，接线放 Task 4。

## 三、提交

- `git add` 范围：`internal/modules/codedelivery/service.go`、`dispatcher.go`、`service_prepare_test.go`、`service_gitlab_test.go`（+ 本报告文件按仓库惯例随任务提交）
- commit message：`feat(codedelivery): server-authoritative provider routing + remote head read-back; GitLab orchestration core tests (T24 #54 task 2)`

## 四、自检发现

1. **`DeliveryActionTarget` 成死常量**：PrepareDelivery 改用 `DeliveryTargetOf(provider)` 后，该导出常量全仓无使用点（grep 实证仅 service.go:18 定义 + 原 :223 使用）。保留未删（最小 diff；未使用导出常量合法）；若审查者希望删除，属一行机械改动。
2. **既有测试零断言改动**：夹具扩展后全部既有 GitHub 侧测试未改一行断言即通过——权威读回对 GitHub 语义恒等（CreateCommit 结果=分支 head），计划第 48 条「作者逐条核对断言」的声明在本次实跑中得到复现。
3. **Task 3 依赖的派发语义已就位**：EnsureBranch → BranchHead 读回 → RecordReceipts(远端 head) 的顺序、`Dispatch` 前置门 parse → A02 → token → 平台路由 → row 的次序，均为 Task 3 语义测试（部分完成恢复/unknown 收敛/护栏次序）断言所依赖的形状，已按计划落定。
4. **blocked-env 声明**：真实 GitLab 端到端（真实 OAuth/仓库）本地不可得，本任务证据链为「真实 sqlite + 真实 A03 ActionService/ocAuthorizer + httptest GitLab 模拟器真实 HTTP 字节 + 本地工作区」，符合计划验收标准 3 的本地可验证性说明；`TestGitLabClientAgainstRealGitLab` skip 未伪造。
