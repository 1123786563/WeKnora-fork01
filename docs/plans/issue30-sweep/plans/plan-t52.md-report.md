# Task 5 实施报告：Go——交付编排 A（工作区端口、基线物化与 PrepareDelivery）

> T22 / Issue #52 实施计划第 5/11 个任务。工作目录：`.worktrees/issue30-sweep-t52`（分支同 worktree）。
> 日期：2026-09-25

## 1. 实现内容

按计划 Task 5 的 Files/Interfaces 逐项落地，全部公开签名与计划 Produces 段逐字一致：

| 文件 | 内容 |
|---|---|
| `internal/modules/codedelivery/workspace.go` | 工作区端口 `WorkspaceFileSource`（List/Read/Write）+ `WorkspaceDirEntry`/`WorkspaceFileWrite` + `ErrWorkspaceUnavailable`。端口注释钉死路径约定（List 返回以清洗后请求目录为前缀的路径，与沙箱同约定）。生产适配器（`*sandbox.SessionBoundManager`）按计划归 Task 7 容器包，本模块零 sandbox 导入。 |
| `internal/modules/codedelivery/workspace_local.go` | `NewLocalWorkspaceSource` 本地目录适配器：`resolve` 拒绝含 `..` 的路径（与 `ParseDeliveryMaterial` 文件路径同一白名单姿态）；缺失目录列出为空（尚未物化），不报错。 |
| `internal/modules/codedelivery/service.go` | `CodeDeliveryService`：`MaterializeBaseline`（owner-only + 基线 sha 校验 + 基线树写入固定根 `/workspace/<owner>/<name>/`，16MiB/500 文件上限）、`PrepareDelivery`（护栏 1 先撞默认分支/远端 protected 再验任务分支前缀白名单 → 护栏 2 工作区 diff 按真实 git blob sha 判定 → `Actions.Prepare` 以 `RiskDeliver`/`github.deliver` 锚定归一化材料 → 结构性断言 action 必生而 `awaiting_approval` → `code_deliveries` 追溯行落账）、`GetDeliveryForRun`/`GetDelivery`/`viewOf`（join A03 行 + 审批者，AC2 可追溯读面）、`authorize`（run 归属 + 个人连接 owner-only）、`tokenFor`（权限链之后才解凭据，字节不进任何响应/日志/沙箱）。 |

测试 `internal/modules/codedelivery/service_prepare_test.go`：真实 sqlite（AutoMigrate，绕开预存在 000112 撞号——计划差异记录 5）+ 真实 A03 `ActionStore`/`ocAuthorizer`（`NewSubjectGuard`）+ Task 2 httptest GitHub 模拟器真实 HTTP 字节 + 本地目录工作区，7 个用例：

1. `TestMaterializeBaselineWritesFixedTreeIntoWorkspace` — 物化 2 文件、固定根正确
2. `TestMaterializeBaselineRejectsNonOwnerAndForeignConnection` — 非 owner run / 非本人连接拒绝
3. `TestPrepareDeliveryAnchorsApprovalAndDiff` — 审批锚定 + diff 语义（含新增断言：修改/新增文件不得被标 Deleted，钉 D2 修复）
4. `TestPrepareDeliveryRefusesProtectedBranchWithZeroRemoteWrites` — AC1：默认分支与远端 protected 均拒 + 零 ref 写 + 零 PR
5. `TestPrepareDeliveryAlwaysAwaitsApprovalEvenWithWritePreAuthorization` — write 预授权永不预授权交付
6. `TestLocalWorkspaceSourceRefusesTraversal` — 路径穿越拒绝
7. `TestStoreStateConstantsMirrorDeliveryState` —（新增）store 镜像常量与模块级 `DeliveryState` 等值钉（见偏差 D0）

## 2. TDD 证据

### RED

命令：`go test ./internal/modules/codedelivery/ -run 'TestMaterializeBaseline|TestPrepareDelivery|TestLocalWorkspace|TestStoreStateConstants' -count=1`

首次实跑暴露**导入环**（见偏差 D0，先行修复）：

```
# github.com/Tencent/WeKnora/internal/modules/codedelivery
package github.com/Tencent/WeKnora/internal/modules/codedelivery
	imports github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery from service_prepare_test.go
	imports github.com/Tencent/WeKnora/internal/modules/codedelivery from store.go: import cycle not allowed in test
FAIL	github.com/Tencent/WeKnora/internal/modules/codedelivery [setup failed]
```

断环后重跑（期望失败形态，与本任务符号对应）：

```
internal/modules/codedelivery/service_prepare_test.go:81:14: undefined: WorkspaceFileSource
internal/modules/codedelivery/service_prepare_test.go:118:20: undefined: NewLocalWorkspaceSource
internal/modules/codedelivery/service_prepare_test.go:133:9: undefined: NewCodeDeliveryService
internal/modules/codedelivery/service_prepare_test.go:133:32: undefined: CodeDeliveryDeps
internal/modules/codedelivery/service_prepare_test.go:143:22: undefined: BaselineInput
internal/modules/codedelivery/service_prepare_test.go:151:21: undefined: PrepareInput
internal/modules/codedelivery/service_prepare_test.go:191:26: undefined: ErrConnectionNotUsable
FAIL	github.com/Tencent/WeKnora/internal/modules/codedelivery [build failed]
```

失败原因符合预期：Task 5 的符号（服务/端口/适配器）尚未实现。

### GREEN（修复夹具保护标记后）

命令：`go test ./internal/modules/codedelivery/ -run 'TestMaterializeBaseline|TestPrepareDelivery|TestLocalWorkspace|TestStoreStateConstants' -count=1 -v`

```
=== RUN   TestMaterializeBaselineWritesFixedTreeIntoWorkspace
--- PASS: TestMaterializeBaselineWritesFixedTreeIntoWorkspace (0.01s)
=== RUN   TestMaterializeBaselineRejectsNonOwnerAndForeignConnection
--- PASS: TestMaterializeBaselineRejectsNonOwnerAndForeignConnection (0.01s)
=== RUN   TestPrepareDeliveryAnchorsApprovalAndDiff
--- PASS: TestPrepareDeliveryAnchorsApprovalAndDiff (0.01s)
=== RUN   TestPrepareDeliveryRefusesProtectedBranchWithZeroRemoteWrites
--- PASS: TestPrepareDeliveryRefusesProtectedBranchWithZeroRemoteWrites (0.01s)
=== RUN   TestPrepareDeliveryAlwaysAwaitsApprovalEvenWithWritePreAuthorization
--- PASS: TestPrepareDeliveryAlwaysAwaitsApprovalEvenWithWritePreAuthorization (0.01s)
=== RUN   TestLocalWorkspaceSourceRefusesTraversal
--- PASS: TestLocalWorkspaceSourceRefusesTraversal (0.00s)
=== RUN   TestStoreStateConstantsMirrorDeliveryState
--- PASS: TestStoreStateConstantsMirrorDeliveryState (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/codedelivery	0.452s
```

## 3. 检查命令与完整输出

| 检查 | 命令 | 结果 |
|---|---|---|
| 本任务用例 | `go test ./internal/modules/codedelivery/ -run 'TestMaterializeBaseline\|TestPrepareDelivery\|TestLocalWorkspace\|TestStoreStateConstants' -count=1 -v` | 7/7 PASS（上文完整输出） |
| 全包回归（Task 1–5 全部套件 + store + 迁移对齐） | `go test ./internal/modules/codedelivery/... -count=1` | `ok … codedelivery 0.767s` / `ok … repository/codedelivery 0.161s`（全绿；无 skip——真实 GitHub env 门控测试属 Task 2 文件，本包内该测试因缺 `WEKNORA_GITHUB_TEST_TOKEN/REPO` skip，与计划 blocked-env 声明一致） |
| 静态检查 | `go vet ./internal/modules/codedelivery/...` | exit=0 |
| 格式 | `gofmt -l`（仅对本任务两个新文件执行 `-w`） | 本任务文件已格式化；`delivery_test.go`/`store_test.go`/`migration_align_test.go` 三个前序任务文件 gofmt 仍标记，未动（非本任务授权文件） |

## 4. 偏差记录（计划稿内部缺陷的修正，均以仓库现状证据为准）

- **D0（结构性，唯一动他人文件处）— store.go 导入环**：计划 Task 3 稿（plan-t52.md:1581、1711-1719）让 `repository/codedelivery/store.go` 导入父包只为镜像 `State*` 常量；计划 Task 5 稿（plan-t52.md:2423）又让父包 service.go 导入 repository——Go 下互斥，RED 首跑即 `import cycle not allowed in test`。仓库既有分层先例是「域根包永不导入 repo」（`internal/modules/appconnector` 域根 0 处导入 repository，service 层才导入，亲核 `internal/modules/appconnector/service/appconnector/action.go:11`）。修复：store.go 镜像常量改为**等值字符串字面量**并摘除父包导入（-8/+12 行，公开面同名同值零损失）；全仓 grep 证实这些常量此前零消费者、计划全篇（含 Task 6/7 稿）也无引用；等值性由本任务测试 7 永久钉住。非回退他人行为，公开 API 不变。
- **D1 — 基线树读取须先 CommitTree**：计划稿 service 直接 `client.Tree(ctx, in.BaselineSHA)`（plan-t52.md:2500/2577），但 `/git/trees` 只接受 tree sha——真实 GitHub 语义，且计划自己的 Task 2 wire 契约注释已钉死（`github_wire_test.go:365`「git/trees 端点只接受 tree sha（CommitTree 的返回值）」），模拟器同样按 tree sha 键存储，直传基线 commit sha 必 404、计划的 Step 1 测试（receipt.Files==2）必失败。实现为 `baselineTree`：`CommitTree(baselineSHA)` → `Tree(treeSHA)`，两处调用点共用。
- **D2 — 工作区列表路径前缀**：计划稿 `ListSessionFiles` 用 `path.Clean(TrimPrefix(dir,"/workspace"))` 作前缀（产出 `/octocat/hello/…`），与计划同一函数注释自述的约定（「以请求目录为前缀（"/workspace/<owner>/<name>/…"）」，plan-t52.md:2352-2353）及 `workspaceTree` 的 rootPrefix 裁剪矛盾——按稿实现会让裁剪永不命中、diff 退化为「全量删除」，Task 6 派发将推出清空仓库树的 PR。实现改为前缀=清洗后的请求目录本身（`/workspace/octocat/hello/…`），测试 3 加断言（修改/新增不得标 Deleted）钉死语义。
- **D3 — workspaceTree 调用点笔误**：计划稿调用 `s.workspaceTree(ctx, in.Repo)`（plan-t52.md:2584）与定义 `workspaceTree(ctx, sessionID, repo)`（plan-t52.md:2737）不符，按三参定义实现并修正调用点。
- **D4 — 夹具保护标记笔误**：计划稿测试 `f.github.protectBranch("prod-branch")`（plan-t52.md:2226）标记裸名，而客户端按真实 GitHub 语义查询完整分支名 `weknora/task/prod-branch`（模拟器 `isProtected` 精确匹配，`github_wire_test.go:109-118`），保护标记不命中、请求穿透（GREEN 首跑该用例 nil 错误实锤）。在我授权的测试文件内改为 `protectBranch("weknora/task/prod-branch")` 并更新注释。

## 5. 自检发现

- **完整性**：计划 Produces 的全部符号逐一落地（`WorkspaceDirEntry/WorkspaceFileWrite/WorkspaceFileSource/NewLocalWorkspaceSource/ErrNotDeliveryOwner/ErrWorkspaceUnavailable/ErrConnectionNotUsable/RunReader/ConnectionReader/CodeDeliveryDeps/NewCodeDeliveryService/BaselineInput/BaselineReceipt/MaterializeBaseline/PrepareInput/DeliveryView/PrepareDelivery/GetDeliveryForRun/GetDelivery/viewOf`），Task 6/7 可直接按冻结签名消费。`GetDelivery` 计划允许 Task 6 补实现，本任务顺手实现（薄封装，减少后续改动面）。
- **安全约束核对**：服务端出站 URL 只由 `GitHubAPIBaseURL`（生产钉 `https://api.github.com`）+ 经 `ParseRepoRef` 校验的 RepoRef 派生，无任何模型输出进 host；测试凭据 `gho_testtoken` 仅对本地 httptest 有效，非可用凭据字面量；令牌仅在 `tokenFor` → GitHub 工厂间流动，不进响应/日志/工作区；SQL 全为 gorm 参数绑定。
- **YAGNI**：未实现 Task 6 的 Dispatch/Resolve（计划明确归 Task 6）；未动路由/容器（Task 7）。
- **顾虑（不阻塞）**：①三个前序任务文件的 gofmt 标记系既有状态，未处置（超授权）；②模拟器 blackout 语义与 `ErrGitHubTransport` 分类在 Task 6 的 unknown 路径才会被服务层消费，本任务未覆盖（计划如此分工）；③ store.go 的 D0 修复是本任务对共享文件的最小适配，建议审查轮重点核对该文件 diff。

## 6. 文件清单

- Create：`internal/modules/codedelivery/workspace.go`
- Create：`internal/modules/codedelivery/workspace_local.go`
- Create：`internal/modules/codedelivery/service.go`
- Test：`internal/modules/codedelivery/service_prepare_test.go`
- 修改（D0 最小适配）：`internal/modules/codedelivery/repository/codedelivery/store.go`
- 计划 Step 5 列出的 `github_wire_test.go` 最终**零改动**（模拟器无需扩展，D1 走服务侧修正）

## 7. 提交

见提交记录：`feat(codedelivery): baseline materialization + delivery prepare ...`（本报告写作时随 commit 落盘）。

---

# Task 6 实施报告：Go——交付编排 B（DispatchDelivery、部分完成与 unknown 收敛）

> T22 / Issue #52 实施计划第 6/11 个任务。工作目录：`.worktrees/issue30-sweep-t52`（分支 `codex/issue30-t52`）。
> 日期：2026-09-25 · 提交：`25b32789e`（7 files changed, 638 insertions, 18 deletions）

## 1. 实现内容

### 1.1 新建 `internal/modules/codedelivery/dispatcher.go`

- `DispatcherDeps`：Connections / Creds（CredentialResolver）/ Guard（A02Guard）/ GitHub（GitHubClientFactory）/ Workspace / Store / ActionRows / Runs——按计划冻结签名。
- `DeliveryDispatcher` 同时实现 A03 `ActionDispatcher`（`Dispatch`）与 `UnknownResolver`（`QueryProvider`），是唯一出站边界（令牌在此解析、GitHub 调用在此发出、回执在此落账；无任何 merge 路径）。
- `Dispatch`：前置闸门（快照精确解析 → A02 复验 → 凭据解析 → 交付行查找）失败一律 `ErrDispatchNotStarted`（可证未起网，允许落 failed）；GitHub 确定性响应=确定性结果；传输错误=`ErrGitHubTransport`（不可观测→上层落 unknown）。
- `deliver` 推送半程：Repository/BranchProtected → `RefuseProtectedTarget`（AC1 双保险，派发前复验默认分支/远端 protected）→ 工作区逐文件读内容（令牌永不进沙箱）→ CreateBlob → CommitTree → CreateTree（删除项 SHA=""=null wire sha，与 Task 2 权威形态一致）→ CreateCommit(parent=基线) → RecordReceipts(commit_sha) → EnsureBranch → 状态 CAS → `pushed`。
- `deliver` PR 半程：DraftPullRequest（head 复用不重建）→ CurrentLogin（实际远端身份）→ RecordReceipts → 状态 CAS → `delivered`。确定性 PR 失败且行已 `pushed` 时返回成功 + partial 回执（部分完成落账，不算失败）；否则 `ErrDispatchNotStarted`。
- `RecoverPullRequest`：仅 `pushed` 行可恢复（否则 `ErrDeliveryState`）；从 `ActionRows.FindAction` 重建批准快照（同一批准），A02 复验后只走 PR 半程——结构上不可能重发 blobs/tree/commit/ref。
- `QueryProvider`：只读远端事实——task head 的开放 draft PR 存在→`delivered`（回执落账）；否则 `BranchHead` 存在→`pushed`（commit sha 落账）；两者皆无→`ErrDispatchUnknown`。绝不重发任何写。

### 1.2 `service.go` 追加（最小修改）

- `CodeDeliveryDeps` 增 `Dispatcher *DeliveryDispatcher` 字段（nil 保持 prepare-only 装配可用，pushed 恢复在未接线时 fail closed）。
- `DispatchInput` / `ErrDeliveryState` / `DispatchDelivery`（owner 谓词先行；`prepared`→CAS `dispatched` 后 `Actions.Execute` 消费批准；`ErrDispatchUnknown`→CAS `unknown` 并返回视图；Execute 失败但行已 `pushed`→按部分完成返回；其余失败→CAS `failed` 并返回错误；`pushed`→`RecoverPullRequest`；其余状态→`ErrDeliveryState`）/ `ResolveDeliveryUnknown`（owner 谓词 + `Actions.ResolveUnknown`）/ `viewAfter`。
- `GetDelivery` 无需改动：Task 5 已实现（`service.go:251`，签名冻结），Task 6 直接复用。

### 1.3 端口补 `BranchHead`（Task 2 共享文件三处小改）

- `github.go`：`GitHubClient` 接口加 `BranchHead(ctx, branch) (string, bool, error)`（`EnsureBranch` 之前）。
- `github_client.go`：实现走 `GET /repos/{o}/{r}/git/ref/heads/{b}`（与真实 GitHub `GET /git/ref/{ref}` 同形），404→`(sha,false,nil)`，其余错误透传。
- `github_wire_test.go` 模拟器：加 `GET /git/ref/heads/<branch>` 分支（无 ref 时 404）；加 `blackoutAfterRef` 字段；`blackoutAfterRefCreate()` 语义改为「下一次成功的 POST /git/refs 之后所有后续请求 hijack 断连」（在 ref 写成功路径上置位 `blackout`）——精确模拟「推送已完成、PR 创建中途网络不可观测」。

### 1.4 夹具改造（`service_prepare_test.go`，计划指定 3 处小改）

1. `deliveryFixture` 加 `dispatcher *DeliveryDispatcher` 字段。
2. `newDeliveryFixture` 去掉变参 `dispatcher ...appconnectorsvc.ActionDispatcher`，改为 dispatcher 内部单实例构造：`store` 先声明，`NewDeliveryDispatcher`、`NewActionService(actionStore, guard, nil, dispatcher, dispatcher)` 与 `NewCodeDeliveryService` 共享同一 db/emulator/connections/workspace/store/actionStore 实例——绝无两套底层件。Task 5 调用点（`newDeliveryFixture(t, nil)` / `(t, func(root string){…})`）不传变参，删除后全部照常编译。
3. `dispatchFixture` 落在 `service_dispatch_test.go`，退化为直通别名。

## 2. TDD 证据

### RED（实现前）

命令：`go test ./internal/modules/codedelivery/ -run 'TestDispatch|TestPartialPush|TestUnknownOutcome|TestTampered' -count=1`

实际输出（节选，完整为编译失败）：

```
# github.com/Tencent/WeKnora/internal/modules/codedelivery [github.com/Tencent/WeKnora/internal/modules/codedelivery.test]
internal/modules/codedelivery/service_dispatch_test.go:34:39: undefined: DispatchInput
internal/modules/codedelivery/service_dispatch_test.go:49:21: f.svc.DispatchDelivery undefined (type *CodeDeliveryService has no field or method DispatchDelivery)
internal/modules/codedelivery/service_dispatch_test.go:177:14: f.dispatcher undefined (type *deliveryFixture has no field or method dispatcher)
...
FAIL	github.com/Tencent/WeKnora/internal/modules/codedelivery [build failed]
```

失败原因与计划 Step 2 预期完全一致：`DispatchDelivery`/`DispatchInput`/`NewDeliveryDispatcher`/`f.dispatcher` 未定义。

### GREEN（实现后）

计划 Step 4 原命令：

```
$ go test ./internal/modules/codedelivery/ -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/codedelivery	1.460s
```

扩展到 store 子包（`-v` 摘要，全部实跑）：

```
$ go test ./internal/modules/codedelivery/... -count=1 -v
--- PASS: TestTaskBranchOfAndValidation (0.00s)
--- PASS: TestRefuseProtectedTarget (0.00s)
--- PASS: TestParseRepoRefAndWorkspaceRoot (0.00s)
--- PASS: TestParseRepoRefRefusesDotDotSegments (0.00s)
--- PASS: TestGitBlobSHAMatchesRealGit (0.04s)
--- PASS: TestDiffAgainstBaseline (0.00s)
--- PASS: TestParseDeliveryMaterialExactFields (0.00s)
--- SKIP: TestGitHubClientAgainstRealGitHub (0.00s)   ← blocked-env：缺 WEKNORA_GITHUB_TEST_TOKEN/REPO，按计划 skip 不伪造
--- PASS: TestGitHubClientWireChainCreatesBranchAndDraftPR (0.01s)
--- PASS: TestGitHubClientCreateTreeDeleteEntryUsesEmptySHA (0.00s)
--- PASS: TestGitHubClientClassifiesDefiniteVsUnobservable (0.00s)
--- PASS: TestDispatchDeliversAndRecordsTraceableReceipts (0.05s)
--- PASS: TestDispatchNeverWritesProtectedBranchOrMerges (0.03s)
--- PASS: TestPartialPushPRFailureRecoversWithoutRepush (0.03s)
--- PASS: TestDispatchRefusesSecondDeliveryPreparedWithDifferentFiles (0.04s)
--- PASS: TestDispatchWithoutApprovalConsumesNothing (0.03s)
--- PASS: TestTamperedSnapshotNeverReachesGitHub (0.02s)
--- PASS: TestUnknownOutcomeResolvesFromRemoteFacts (0.03s)
--- PASS: TestDispatchFailsClosedWhenConnectionUnusable (0.02s)
--- PASS: TestMaterializeBaselineWritesFixedTreeIntoWorkspace (0.02s)
--- PASS: TestMaterializeBaselineRejectsNonOwnerAndForeignConnection (0.01s)
--- PASS: TestPrepareDeliveryAnchorsApprovalAndDiff (0.01s)
--- PASS: TestPrepareDeliveryRefusesProtectedBranchWithZeroRemoteWrites (0.01s)
--- PASS: TestPrepareDeliveryAlwaysAwaitsApprovalEvenWithWritePreAuthorization (0.02s)
--- PASS: TestLocalWorkspaceSourceRefusesTraversal (0.00s)
--- PASS: TestStoreStateConstantsMirrorDeliveryState (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/codedelivery	2.211s
--- PASS: TestCodeDeliveriesMigrationSQLMatchesModel (0.01s)
--- PASS: TestDeliveryStoreLifecycleAndCAS (0.01s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery	0.629s
```

Task 6 新增 8 个测试全绿；Task 1–5 既有测试无回归；blocked-env 真实 GitHub 测试按计划 skip。

## 3. 自检附加检查（本任务实跑）

| 检查 | 命令 | 结果 |
|---|---|---|
| 静态检查 | `go vet ./internal/modules/codedelivery/...` | 通过（无输出） |
| 编译 | `go build ./internal/modules/codedelivery/... ./internal/modules/appconnector/...` | BUILD OK |
| 格式 | `gofmt -l internal/modules/codedelivery/` | 标出 3 个**前序任务既有文件**（delivery_test.go / repository/codedelivery/migration_align_test.go / store_test.go；`git status` 证实不在本任务 diff 内）；本任务新建/修改的 7 个文件均通过 gofmt |

## 4. 与计划稿的偏差（1 处，测试字面量笔误修正）

`TestDispatchDeliversAndRecordsTraceableReceipts` 中计划硬编码字面量 `"b0000000000000000000000000000000000000000"`（41 字符）比夹具基线 sha 多一个 0——夹具/`prepareInput()`/模拟器 seed 的基线均为 `"b"+strings.Repeat("0", 39)`（40 字符 40-hex），首次 GREEN 实跑暴露该不等：

```
Error: Not equal:
    expected: "b0000000000000000000000000000000000000000" (41 chars)
    actual  : "b000000000000000000000000000000000000000" (40 chars)
```

修正为 `"b"+strings.Repeat("0", 39)`（与夹具同源表达式，附注释说明）。这是计划测试代码的笔误而非实现缺陷；语义不变（断言默认分支 main 仍指向基线提交、纹丝不动）。

## 5. 计划关键断言的验证方式（全部来自上述实跑）

- **部分完成恢复绝不重推**：`TestPartialPushPRFailureRecoversWithoutRepush` 用模拟器调用计数断言 recovery 前后 `POST /git/blobs|trees|commits|refs`、`PATCH /git/refs` 全部相等、仅 `POST /pulls` +1。
- **unknown 以远端事实收敛**：`TestUnknownOutcomeResolvesFromRemoteFacts` 走 blackoutAfterRefCreate→`unknown` 落账（delivery state 与 action_state 双断言）→liftBlackout→Resolve→`pushed`→PR-only 恢复→`delivered`。
- **AC1 零 merge/零保护分支写**：`TestDispatchNeverWritesProtectedBranchOrMerges` + 全部派发测试尾部 `require.Empty(f.github.Violations())`（模拟器对 merge 尝试与越权 ref 写记违规）。
- **审批锚点不可变**：`TestDispatchRefusesSecondDeliveryPreparedWithDifferentFiles`——内容变更必须新 Prepare 新 digest，旧 digest 批准被拒，未获自身 digest 批准的派发拒绝且零 GitHub 写调用。
- **篡改快照零远端触达**：`TestTamperedSnapshotNeverReachesGitHub`——`ErrDispatchNotStarted` 且零 blob 调用。
- **A02 fail closed**：`TestDispatchFailsClosedWhenConnectionUnusable`——成员资格撤销后动作不消费、零远端调用。
- **AC2 全链路追溯**：`TestDispatchDeliversAndRecordsTraceableReceipts`——commit_sha/PR 回执/remote_login/approver/digest 全部落账并可从 Run 读回。

## 6. 文件清单（7 个，全部在计划授权范围内）

- Create：`internal/modules/codedelivery/dispatcher.go`
- Create：`internal/modules/codedelivery/service_dispatch_test.go`
- Modify：`internal/modules/codedelivery/service.go`（Deps 加 Dispatcher 字段 + 追加派发面）
- Modify：`internal/modules/codedelivery/service_prepare_test.go`（夹具单实例改造）
- Modify：`internal/modules/codedelivery/github.go`（接口加 BranchHead）
- Modify：`internal/modules/codedelivery/github_client.go`（BranchHead 实现）
- Modify：`internal/modules/codedelivery/github_wire_test.go`（模拟器 ref-read 分支 + blackoutAfterRef 语义）

## 7. 遗留与关注点

- blocked-env：真实 GitHub 端到端（`TestGitHubClientAgainstRealGitHub`）本地缺 env，按计划 skip，未伪造；具备 env 的运行自动产出真实证据。
- 预存在失败（非本计划引入，计划差异记录第 5 条）：全量迁移轨道因 migrations/sqlite 000112 撞号不可用；本计划沿用自包含 sqlite 夹具（AutoMigrate）策略，未触碰迁移与全量迁移测试轨道。
- 3 个前序任务文件的 gofmt 标记系既有状态，未处置（超本任务授权）。
- 未运行（超本任务授权范围，留 Task 7/11）：workbench HTTP 面测试、移动端测试、`pnpm` 侧任何检查。


---

# Task 7 实施报告：workbench HTTP 面与容器接线（T22 #52 task 7）

## 1. 实现内容

严格 TDD（RED→GREEN），实现 Task 7 全部交付：

1. **`internal/handler/session/workbench_delivery.go`（Create）**：`DeliveryService` 接口（5 方法，与计划冻结签名逐字一致）+ `WorkbenchDeliveryHandler` 五个端点方法：
   - `MaterializeBaseline`：POST `:run_id/baseline`（owner-only，201）
   - `PrepareDelivery`：POST `:run_id/delivery`（owner-only，201，branch 可选 TrimSpace）
   - `GetDelivery`：GET `:run_id/delivery`（owner + granted 读）
   - `DispatchDelivery`：POST `:run_id/delivery/:delivery_id/dispatch`（owner-only）
   - `ResolveDeliveryUnknown`：POST `:run_id/delivery/:delivery_id/resolve`（owner-only）
   - `writeDeliveryError` 固定错误码表：403 forbidden / 409 protected_branch / 409 state_conflict / 404 not_found / 502 provider_refused（`*GitHubAPIError`）/ 502 provider_unreachable（`ErrGitHubTransport`）/ 500 兜底——上游文本与凭据邻接字符串不越线。
2. **`internal/handler/session/workbench_delivery_test.go`（Create）**：3 个 gin 直调测试（沿用 `workbench_terminal_log_test.go` 范式），stub 注入 `OwnedRunReader`/`GrantedRunReader`/`DeliveryService`。
3. **`internal/router/routes_workbench.go`（Modify，文件尾 +18 行）**：`RegisterWorkbenchDeliveryRoutes`——读组挂 `Viewer()+workbenchReadGate`，写组挂 `Viewer()`（与 terminal-log/source-events 同款边界）。
4. **`internal/router/router.go`（Modify，+2 行）**：`RouterParams` 加 `WorkbenchDeliveryHandler`（`optional:"true"`）+ `RegisterWorkbenchCommandRoutes` 调用行后加注册行。
5. **`internal/container/code_delivery.go`（Create）**：`sandboxWorkspaceSource` 适配器（租户 resolver 优先于进程默认；仅 `*sandbox.SessionBoundManager` 可服务，其余 fail closed `ErrWorkspaceUnavailable`；租户从请求 ctx 取，缺租户 fail closed）+ `newCodeDeliveryService` dig provider（专用 `DeliveryDispatcher` + 专用 `NewActionService(actionStore, guard, nil, dispatcher, dispatcher)`，全局 OC 管线不动）+ `NewWorkbenchDeliveryHandler`（runs/granted 同为 `*AgentRunStore`）。
6. **`internal/container/container.go`（Modify，+3 行）**：`NewWorkbenchListHandler` Provide 行后加 `newCodeDeliveryService` + `NewWorkbenchDeliveryHandler` 两个 Provide。

## 2. TDD 证据

**RED**（`go test ./internal/handler/session/ -run 'TestDelivery' -count=1`，实现前）：

```
internal/handler/session/workbench_delivery_test.go:94:7: undefined: NewWorkbenchDeliveryHandler
internal/handler/session/workbench_delivery_test.go:130:7: undefined: NewWorkbenchDeliveryHandler
internal/handler/session/workbench_delivery_test.go:159:7: undefined: NewWorkbenchDeliveryHandler
FAIL    github.com/Tencent/WeKnora/internal/handler/session [build failed]
```

失败原因符合预期：`NewWorkbenchDeliveryHandler` 未定义（编译错误）。

**GREEN**（计划 Step 4 指定验证命令，实现后实跑）：

```
$ go test ./internal/handler/session/ -run 'TestDelivery' -count=1 && go build ./... && go test ./internal/modules/codedelivery/ -count=1
ok      github.com/Tencent/WeKnora/internal/handler/session     4.950s
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
ok      github.com/Tencent/WeKnora/internal/modules/codedelivery        4.447s
```

`TestDelivery` 三个测试逐个 `-v` 确认（同轮实跑）：

```
--- PASS: TestDeliveryPrepareOwnerOnly (0.00s)
--- PASS: TestDeliveryReadOpenToGrantedViewer (0.00s)
--- PASS: TestDeliveryDispatchOwnerOnlyAndBaselineMaterializes (0.00s)
```

`ld: warning: ignoring duplicate libraries` 为预存在链接器噪音（本任务之前即有），非测试失败。

## 3. 额外自检证据（计划未要求、本任务实跑）

- **gin 路由挂载无通配符冲突**（临时自检测试，实跑后已删除不提交）：将既有 `RegisterWorkbenchRoutes`/`RegisterWorkbenchCommandRoutes` 与新增 `RegisterWorkbenchDeliveryRoutes` 挂同一 `*gin.Engine`，注册无 panic；`GET /api/v1/workbench/executions/run-1/delivery`、`POST …/baseline`、`POST …/delivery/dlv-1/dispatch` 均命中 401（无身份请求被 auth 边界拦截=路由已挂载），路由表无 404 miss。
- **router 包既有测试无回归**：`go test ./internal/router/ -run 'Workbench|ApiKey|Rbac' -count=1` → `ok 3.414s`。
- **gofmt**：`gofmt -l` 对全部 6 个触碰文件零输出（合规）。

## 4. 与计划稿的偏差（3 处，均为机械修正/语义对齐，无行为改变）

1. **测试 stub 错误值**：计划稿 stub 返回 `errors.New("run_not_found")`；但计划 Consumes 明确要求复用包级 `resolveOwnedRun`，其 `strictOwnedRun`（`workbench_read.go:163-175`）只把 `agentruntime.ErrNotFound` 归类为 scope miss（→404），其余 error 记存储故障（→500）。计划稿的 handler（调 `resolveOwnedRun`）与测试 stub（返回普通 error）互斥：GREEN 首轮实跑 u2 dispatch 得 500 而非断言的 404。修正：stub 改返回 `runtime.ErrNotFound`（附注释），handler 与全部行为断言（201/404/200/404）逐字保持计划原样。
2. **`caller` 取法**：计划稿 `caller` 用 `c.Value(types.TenantIDContextKey)`；实现改用同包既定 `workbenchCaller(c)` helper（request context 优先、gin keys 兜底）——计划括号注释自述「与 `resolveOwnedRun` 相同的取法」，而 `resolveOwnedRun` 的取法就是 `workbenchCaller`（`workbench_read.go:141-155`），语义一致且与包内其他谓词零漂移。
3. **容器 `source()` 类型修正**：计划稿让 `source()` 直接把 `*sandbox.SessionBoundManager` 当 `codedelivery.WorkspaceFileSource` 返回，编译不过（方法签名不同构：`[]sandbox.RemoteDirEntry` vs `[]codedelivery.WorkspaceDirEntry`、`[]sandbox.SessionWorkspaceFile` vs `[]codedelivery.WorkspaceFileWrite`）。改为 `manager()` 返回具体 `*SessionBoundManager`，三个端口方法内做映射；fail closed 语义与计划完全一致。

## 5. 文件清单（6 个，全部在计划授权范围内）

- Create：`internal/handler/session/workbench_delivery.go`
- Create：`internal/handler/session/workbench_delivery_test.go`
- Create：`internal/container/code_delivery.go`
- Modify：`internal/router/routes_workbench.go`（文件尾追加）
- Modify：`internal/router/router.go`（params 字段 + 注册行）
- Modify：`internal/container/container.go`（2 个 Provide + 1 行注释）

## 6. 遗留与关注点

- `resolveReadable` 按计划稿实现（owner 读失败→granted 兜底→404）：与包内 `resolveReadableRun` 相比，把「真实存储故障」也收敛为 404 而非 500。计划稿如此（且其测试如此锚定），生产 wiring 下 runs/granted 同为 `*AgentRunStore`，故障面一致；如需 500 语义需改 `strictOwnedRun` 路径并同步改计划测试 stub——超出本任务授权，如实记录不擅改。
- `newCodeDeliveryService` 按计划稿返回 `(*CodeDeliveryService, error)`，实现体恒返回 nil error（dig 支持 error 返回，形状保留给未来装配失败路径）。
- 容器装配的运行时验证由 `go build ./...`（dig provider 签名编译正确性）+ 路由挂载自检覆盖；完整 `BuildContainer` Invoke 需要 DB/Redis 等真实依赖，不在本地无环境可跑范围（计划 Step 4 即以 `go build ./...` 为容器接线的验证面）。
- blocked-env：真实 GitHub 端到端依赖 env（`WEKNORA_GITHUB_TEST_TOKEN/_REPO`），本地缺 env，前序 Task 2 已按计划 skip，本任务未触碰该面，无伪造。
