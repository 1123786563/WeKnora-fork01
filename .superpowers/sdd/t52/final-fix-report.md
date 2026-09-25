# T52 最终修复轮报告——整计划最终审查五项发现一次修复

- 分支：`codex/issue30-t52`
- 日期：2026-09-25
- 范围：`internal/modules/codedelivery/**`、`internal/handler/session/workbench_delivery*.go`
- 结论：5 项发现（2 important + 3 minor）全部修复，每项附覆盖测试；相关包全绿；全量回归中所有失败均逐一验证为预存在（与本次改动无关，证据见 §6）。

---

## 发现 1（important）：同任务分支二次交付在真实 GitHub 必败

**审查指出**：CreateCommit 恒以 BaselineSHA 为 parent（dispatcher.go:145），EnsureBranch 对已存在分支 PATCH force:false（github_client.go:215-216），新提交与分支现 head 互不为后代，真实 GitHub 422 拒绝；模拟器 PATCH 无条件接受任何更新（github_wire_test.go:295-308）且该路径全测试套件零执行，「创建或更新草稿 PR」的迭代语义无证据。

**修复**：
1. `dispatcher.go:124-135`（deliver 推送半程）：派发前读 `BranchHead(material.Branch)`，分支已存在则 `parent = head`（新提交以分支现 head 为 parent），不存在才用 BaselineSHA——这正是 GitHub force:false ref 更新要求的 fast-forward 链。
2. `github_client.go:204-231`（EnsureBranch）：create 422 后先以 `BranchHead` 远端核实 ref 确实存在，才走 force:false PATCH；PATCH 本身语义不变。
3. `github_wire_test.go`：模拟器补 `commitParents` 记录与 `isAncestor` 判定；**PATCH 现按真实 GitHub 语义校验 fast-forward**——非 ff 更新返回 422 "Update is not a fast forward"，ref 不存在返回 422 "Reference does not exist"，sha 对象不存在返回 422 "Object does not exist"。

**覆盖测试与证据**：
- `TestGitHubClientEnsureBranchFastForwardAndRootCause`（github_wire_test.go:530-573）：
  - 以现 head 为 parent 的第二颗提交 ff 顶上去 ✓
  - 以 BaselineSHA 为 parent 的「平行」提交被 422 拒绝且 head 不动 ✓（此断言在旧模拟器「无条件接受 PATCH」上必失败——契约钉子）
  - create 422 但分支不存在时根因 "Object does not exist" 原样浮出 ✓
  - 运行：`go test ./internal/modules/codedelivery/ -run 'TestGitHubClientEnsureBranchFastForwardAndRootCause' -v` → `--- PASS`
- `TestSecondDeliveryOnSameTaskBranchFastForwards`（service_dispatch_test.go:220-271）：端到端二次交付——新 digest → 新批准 → 二次派发成功；断言 `ParentOf(second) == [first]`（ff 链）、PR 复用同一 PRNumber（迭代而非重开）、`GET /repos` 恰 4 次。运行 → `--- PASS`。
- **变异验证**（本次执行）：把 dispatcher 还原为「忽略现 head、恒以 BaselineSHA 为 parent」的旧语义，`TestSecondDeliveryOnSameTaskBranchFastForwards` 立即 `--- FAIL`（parent 链断言 Not equal）；还原修复后转绿。证明测试真正钉住该修复，迭代语义不再是无证据宣称。
- CONTEXT.md:239「创建或更新草稿 PR/MR」现由实现 + 模拟器契约 + 双层测试支撑。

## 发现 2（important）：前置门失败后交付行永久滞留 dispatched

**审查指出**：dispatcher 前置门失败（ErrDispatchNotStarted，如凭据行被删后 tokenFor 失败）时 A03 settleOutcome 落 action=failed 且 Execute 返回 nil（action.go:474-477，实为 settleOutcome action.go:576-589），service.DispatchDelivery prepared 分支掉出 switch，delivery 行永久滞留 dispatched，无自愈路径。

**修复**（`service.go:320-337`）：`Actions.Execute` 返回 nil 后复读 action 行——`action.State == failed` 时交付行同步 `TransitionState → failed`（记录失败原因），并返回新错误 `ErrDeliveryDispatchRejected`（service.go:284-289，"code_delivery_dispatch_rejected"）。语义：拒绝发生在任何出网之前、批准已消费，重试须重新 Prepare；行不再卡死中间态。

**覆盖测试与证据**：
- `TestPreSendGateFailureSettlesDeliveryFailedNotStranded`（service_dispatch_test.go:274-315）：`breakCreds` 注入凭据行被删 → 派发返回 `ErrDeliveryDispatchRejected`；交付行 = failed 且 Failure 非空；A03 侧行 = failed；派发期 `GET /git/ref`、`POST /git/blobs`、`POST /git/refs` 全 0（零出网）；两条旧死路现在是干净终态拒绝（再派发 → `ErrDeliveryState`；resolve → error）。运行：`go test ./internal/modules/codedelivery/ -run 'TestPreSendGateFailureSettlesDeliveryFailedNotStranded' -v` → `--- PASS`。
- handler 侧分类：`ErrDeliveryDispatchRejected` → 409 `code_delivery_dispatch_rejected`（见发现 3 测试表）。

## 发现 3（minor）：输入类错误分类失真

**审查指出**：ErrInvalidMaterial 族/ErrInvalidBaselineSHA/ErrRepoRefInvalid 落 default 500 code_delivery_failed；prepare 对非法字符分支先打 BranchProtected 远端读；http.NewRequest 构建失败报 ErrGitHubTransport→502 provider_unreachable。

**修复**：
1. `delivery.go:68-92` 新增纯本地 `branchShapeLegal`（空/超长/`..`/非法 refname 字符）：非法形状在任何远端读之前本地拒绝（`service.go:167-174`，ErrInvalidBranch）。AC1 次序不变——形状合法的 "main" 仍先撞 RefuseProtectedTarget。
2. `github.go:20-25` 新增 `ErrGitHubRequestInvalid`；`github_client.go:45-47` 请求构建失败改报该错误（可证未出网，不再伪装成传输不可观测）。
3. `workbench_delivery.go:219-236` 错误分类表补三档：输入类（ErrInvalidMaterial/ErrInvalidBranch/ErrInvalidBaselineSHA/ErrRepoRefInvalid/ErrBaselineTooLarge）→ 400 `code_delivery_invalid_material`；`ErrDeliveryDispatchRejected` → 409；`ErrGitHubRequestInvalid` → 500 `code_delivery_request_invalid`（非 502）。

**覆盖测试与证据**：
- `TestDeliveryErrorClassificationTable`（workbench_delivery_test.go:184-221）：10 档错误 × prepare/dispatch 双入口断言 status+code 全对（含 400/409/500/502 边界与 default 兜底）。运行：`go test ./internal/handler/session/ -run TestDelivery -v` → 全 PASS（含该表 10 子用例）。
- `TestPrepareDeliveryRejectsIllegalBranchShapeBeforeAnyRemoteRead`（service_prepare_test.go:255-268）：三种非法形状分支本地拒绝且 `GET /repos`、`GET /branches` 全 0。运行 → `--- PASS`。

## 发现 4（minor）：created_at 并列时读面取行次序未定

**审查指出**：LatestForRun/findByAction 按 created_at DESC 排序，sqlite DATETIME 秒级精度下并列行次序未定。

**修复**：两处查询均补 `Order("id DESC")` 决出全序：
- `repository/codedelivery/store.go:71-81`（LatestForRun）
- `dispatcher.go:274-285`（findByAction）

**覆盖测试与证据**：`TestLatestForRunDeterministicOnCreatedAtTie`（store_tiebreak_test.go，新增文件）：强制两行同一时刻，连续 5 次读取均稳定返回 id 较大行。运行：`go test ./internal/modules/codedelivery/repository/codedelivery/ -run TestLatestForRunDeterministicOnCreatedAtTie -v` → `--- PASS`。

## 发现 5（minor）：EnsureBranch 根因遮蔽 + deliver 重复远端读

**审查指出**：create 422 不区分「分支已存在」即 PATCH（根因被遮蔽）；deliver 推送半程与 PR 半程各调一次 Repository。

**修复**：
1. EnsureBranch 的 ref 存在性远端核实（见发现 1 修复点 2）：非「已存在」的 422 原样浮出 create 根因。
2. `dispatcher.go:101-110,176-184`：默认分支读一次，推送半程取到后 PR 半程直接复用；仅恢复半程（跳过推送半程）才自取。

**覆盖测试与证据**：
- 根因浮出：`TestGitHubClientEnsureBranchFastForwardAndRootCause` 第 4 段（"Object does not exist" 原样浮出，`refs` 中无该分支）→ `--- PASS`。
- 远端读计数：`TestSecondDeliveryOnSameTaskBranchFastForwards` 断言两次 prepare + 两次派发 `GET /repos` 恰 4 次（修复前每次派发读 2 次、总计 6）→ `--- PASS`。

---

## 6. 验证记录（本次执行）

| 命令 | 结果 |
|---|---|
| `go build ./...` | 通过（仅既有的 ld duplicate libraries 警告） |
| `go test ./internal/modules/codedelivery/...` | ok（1.909s + repository ok） |
| `go test ./internal/modules/appconnector/... ./internal/handler/session/...` | appconnector ok；session 仅 2 个预存在迁移失败（见下） |
| `go test ./internal/handler/session/ -run TestDelivery -v` | 全 PASS（4 测试 + 分类表 10 子用例） |
| 关键回归 `-run` 单项 ×5 | 全 PASS |
| 变异验证（旧 parent 语义） | 测试必败 → 证明契约钉子有效 |
| `go test ./internal/...`（全量） | 475 个 FAIL，全部归因预存在问题（下表） |

### 全量回归失败归因（全部经 stash 基线复跑验证为预存在）

- **471 个**：`duplicate migration file: 000112_task_grants.down.sql`（迁移编号 000112 被 `000112_agent_adoption_variants.*` 与 `000112_task_grants.*` 共用，来自已合入的 #42/#59，波及所有从 migrations 目录初始化 DB 的测试包：application/repository、application/service、container、database、handler/session、router 等）。stash 后基线复跑同败。**不在本批五项发现范围内，未动**——重编号迁移需要独立决策（可能影响已应用环境），建议单开任务。
- `TestSQLiteMigrationsCreateVersionedSchema`：同根因（"SQLite fixture has duplicate version 112"），基线复跑同败。
- `TestAdmissionTwentyConcurrentIdenticalRequestsCreateOneRun`：sqlite "database is locked" 并发，基线复跑同败。
- `TestDecisionSurvivesKillAfterSavedDecision` / `TestDecisionNoDuplicateAfterAcceptedButUnackedKill`：craft 进程 barrier 30s 超时（"provider barrier never appeared"），基线复跑同败。
- `TestTwoWorkerContentionSQLite`：基线复跑同败（133.9s）。
- `TestProvidersFromEnvRejectsPartialAlipay`：隔离运行（含全包单次）在当前改动下 PASS；`-count=3` 基线同样 FAIL——包内重复执行污染，非本批改动引入。

### 与本批改动直接相关的包（全绿）

- `internal/modules/codedelivery` + `internal/modules/codedelivery/repository/codedelivery`
- `internal/modules/appconnector/...`
- `internal/handler/session` 的全部 delivery 测试

## 7. 改动文件清单

| 文件 | 内容 |
|---|---|
| internal/modules/codedelivery/dispatcher.go | 发现 1（parent=head 链）+ 发现 4（findByAction id DESC）+ 发现 5（默认分支复用） |
| internal/modules/codedelivery/github_client.go | 发现 1/5（EnsureBranch ref 核实）+ 发现 3（构建失败改报 ErrGitHubRequestInvalid） |
| internal/modules/codedelivery/github.go | 发现 3（ErrGitHubRequestInvalid 定义） |
| internal/modules/codedelivery/delivery.go | 发现 3（branchShapeLegal 纯本地形状闸） |
| internal/modules/codedelivery/service.go | 发现 2（前置门失败自愈 + ErrDeliveryDispatchRejected）+ 发现 3（prepare 本地形状闸） |
| internal/modules/codedelivery/repository/codedelivery/store.go | 发现 4（LatestForRun id DESC） |
| internal/handler/session/workbench_delivery.go | 发现 3（错误分类表三档补齐） |
| internal/modules/codedelivery/github_wire_test.go | 发现 1/5 测试（模拟器 ff 契约 + 根因浮出） |
| internal/modules/codedelivery/service_dispatch_test.go | 发现 1/2 测试 |
| internal/modules/codedelivery/service_prepare_test.go | 发现 3 测试 |
| internal/handler/session/workbench_delivery_test.go | 发现 3 测试（错误分类表） |
| internal/modules/codedelivery/repository/codedelivery/store_tiebreak_test.go | 发现 4 测试（新增） |
