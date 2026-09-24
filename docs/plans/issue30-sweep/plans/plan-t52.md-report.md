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
