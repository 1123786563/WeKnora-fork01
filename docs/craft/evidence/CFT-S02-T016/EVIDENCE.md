# CFT-S02-T016 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/application/repository/craft_workspace_guard_test.go`（新增，4 tests，`TestCraftWorkspaceGuard*`）：真实 store + 真实 DB 的守卫矩阵汇总。落点说明：任务卡建议 service 层 `craft_workspace_guard.go`；核实时确认单写槽/CAS/fence 的实现全部在 `internal/application/repository/craft_workspace.go`（W01/W03 交付：PutWorkspace 事务 CAS + 唯一绑定、PrepareTask 的 lockToolRun fence 锁、SaveResult fence），断言是 store 行为——测试放 repository 包复用真实 fixture（与 T014 同理由）。无需新 guard 文件（同职责实现已存在，不平行创建）。

## 验收断言对照

- 并发修改只接受一个写入者 ✓（`GuardSingleWriter`：4 并发 PutWorkspace 恰 1 胜者、其余 ErrConflict）
- stale revision 失败且不写文件 ✓（`GuardStaleRevisionWritesNothing`：错误 CAS 后 workspace 行未动（sbx/revision 原样）且 craft_versions 零行）
- 旧 generation/fence 不能发布 ✓（`GuardStaleFenceCannotPublish`：epoch 提升后——旧 worker EnsureToolPlan 即"lease lost"、PrepareTask ErrConflict、SaveResult 报错；发布链上游全部被拒 → craft_versions 零行）
- 读旧版本不占写槽 ✓（`GuardReadDoesNotHoldWriteSlot`：Get 旧版本后同工作区 CAS 写入仍成功 revision 1→2）

## 与既有测试的关系

ConcurrentCreateProducesSingleBinding / RevisionCASAndReopen / SaveResultFenceAndIdempotency（W01/W03 既有）覆盖单点行为；本套件是四断言的**矩阵化汇总**（含新增的"读不占写槽"与"stale 后零版本行"组合断言）。

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/application/repository -run TestCraftWorkspaceGuard -count=1` | 0 | 4 PASS |
| `go test -count=1 ./internal/application/repository/` | 0 | ok（43.5s 全包无回归） |

## 回退

revert 本提交（单测试文件，纯增量）。
