# Pass B 节点 b2-ac-skills 差分证据（25b 租户 Skill 目录/安装/运行时验证/reaper）

> 节点：`b2-ac-skills`（DAG `docs/plans/passb/execution-dag.json`，role=work，depends_on=ib1）。
> 计划：`docs/plans/passb/25b-skill-catalog-install.md`；公约：`.superpowers/sdd/passb/conventions.md`。
> 节点基线（base_sha，派发登记）：worktree 分支头 `8592f2aacbe49e43583bf433e045b98cefd44c6f`（集成头 `8c45a8815` + 本计划两笔 docs 提交）。

## 0. 前置条件核验（T1 实测，2026-09-23）

| 项 | 口径 | 实测 | 结论 |
|---|---|---|---|
| b0 状态 | `execution-dag.json`（派发事实源 `.worktrees/passb-int/` 同路径） | `status=done head_sha=d57a2fa708c3fecf5f0510553ea26db3de51d3c9 review_status=approved` | 满足（计划 §前置条件 1；节点 notes 中「BLOCKED（2026-09-23）：前置 b0 阻塞」为当日早间历史登记，notes 尾条已为「恢复。」） |
| ib1 状态 | 同上 | `status=done head_sha=8c45a8815… review_status=approved` | 满足（计划 §前置条件 2 的「派发 gate 未收口」已由协调者收口） |
| b2-ac-skills 派发登记 | 同上 | `status=in_progress base_sha=8c45a8815… head_sha=null review_status=pending` | 协调者已放行派发，本节点按派发 BASE=`8592f2aac` 开工 |
| worktree 状态 | `git rev-parse HEAD` / `git status --short` | `8592f2aac…` / 空 | 干净，等于派发 BASE |

## 1. 基线（T1，2026-09-23，worktree `codex/passb-b2-ac-skills` @ `8592f2aac`，未改动工作树实跑）

复跑计划 §前置条件 4 全部命令，输出逐条摘录（命令原文 + 退出码）：

| # | 命令 | 退出码 | 关键输出 |
|---|---|---|---|
| 1 | `go build ./...` | 0 | 无错误（仅 `cmd/desktop`、`cmd/server` 链接期 `ld: warning: ignoring duplicate libraries: '-lc++'`，与计划基线一致） |
| 2 | `go test ./internal/application/service -count=1` | 0 | `ok  	github.com/Tencent/WeKnora/internal/application/service	415.098s`（计划撰写时实测 154.512s；同为 `ok`，耗时差异为机器负载，不影响基线判定） |
| 3 | `go test ./internal/application/service -run 'Skill' -count=1` | 0 | `ok  	github.com/Tencent/WeKnora/internal/application/service	5.106s` |
| 4 | `go test ./internal/handler -run 'Skill' -count=1` | 0 | `ok  	github.com/Tencent/WeKnora/internal/handler	1.769s` |
| 5 | `go test ./internal/router -run 'ApiKey|Skill' -count=1` | 0 | `ok  	github.com/Tencent/WeKnora/internal/router	1.236s` |
| 6 | `make check-backend-architecture` | 0 | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16` + `architectureguard: OK (0 violations)` —— 与 §前置条件 4 数值逐项一致（633/23+23/58/16） |
| 7 | `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` —— 与 §前置条件 4 一致 |
| 8 | `go test ./internal/application/service -run 'TestReap\|TestPrune\|TestReconcile' -count=1 -v` | 0 | 28 个用例全部 `--- PASS`，`ok ... 1.220s`，用例清单见 §1.1 |

**结论：8/8 绿，与 §前置条件 4 数值一致，基线成立；无红灯项，节点继续开工（T2 起）。**

### 1.1 reaper 相关用例清单（命令 #8，`-v` 摘录）

`=== RUN` 共 28 条、`--- PASS` 共 28 条、`--- FAIL` 0 条。按前缀分组：

- `TestReapStuckRuns*`（9）：`DeletesAbandonedRemovalAfterPointerMoved`、`DeletesAbandonedRemovalThatNeverReachedAnImage`、`FailsAbandonedInstalls`、`HealsInstallThatDiedAfterThePointerSwitched`、`HealsInstallingRowWhoseSnapshotIsStillLive`、`IgnoresFreshRuns`、`LeavesRemovalAloneWhenTheChainCannotBeFollowed`、`RestoresAbandonedRemovals`、`RestoresRemovalOfSkillInheritedByALaterSnapshot`
- `TestPruneSupersededSnapshots*`（17）：`BuildsNoProviderClientWithNothingToPrune`、`DeletesOldLedgerSnapshots`、`DeletesStaleActiveLeftovers`、`HonoursALongerConfiguredSandboxTTL`、`LeavesTheRowWhenDeleteFails`、`NeverDeletesTheLiveImage`、`NeverDeletesUnknownProviderSnapshots`、`RetriesWhenSnapshotStillInUse`、`SkipsAConfigBuiltByAnotherAccount`、`TreatsMissingProviderSnapshotAsDeleted` 等（完整清单存 `-v` 原始输出）
- `TestReconcileSnapshots*`（1）：`WarnsExtrasWithoutDeleting`
- 另含 `TestPruneEmptyFolderChains*` 等同前缀正则命中的非 reaper 面 `Prune` 用例（命令字面匹配范围，如实记录）

### 1.2 基线留档

- `-v` 原始输出暂存执行机 `/tmp/b2ac_reap_v.log`（T5 差分时以同命令重跑比对，不以该临时文件为事实源；事实源为本节摘录 + T5 终态重跑输出）。

## 2. 搬迁等价差分（T5 填写）

> 占位：T1 基线 vs T5 终态同命令双跑，逐包 `ok/FAIL` 比对结论（conventions §6；计划 §7.1）。legacy 残差删除前本节必须已通过。

## 3. 导出化差分（T5 填写）

> 占位：`tenant_skill_export_parity_test.go`（T3 产出）用例清单与运行结论（计划 §7.2）。

## 4. 消费方面差分（T5 填写）

> 占位：execution/conversation/RBAC 消费方测试零回归证据（计划 §7.3）。
