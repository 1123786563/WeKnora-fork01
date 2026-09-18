# CFT-S03-T022 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/application/service/craft_snapshot_guard_test.go`（新增，1 test）：缺源恢复守卫组合 pin。
- 实现零改动：快照捕获/恢复为 C05 既有。

## 验收断言对照

- 缺源允许下载但拒绝恢复 ✓（新增 `RestoreGuardMissingSource`：不存在的快照 → Restore 拒绝（非 Busy——缺源而非槽位）；同版本 v2 的 files 仍可 Get（下载不需要快照）；真实 v1 快照恢复不受扰）
- 重复 restore request 只提升一次 revision ✓（既有 `TestCraftRestoreIdempotentKey`：同 key 重试 Replayed、generation 不变、restoreCount 恒 1；HTTP 面 `TestCraftHTTPRestoreCompetitions` 的幂等段）
- 恢复 v1 不产生 v4 ✓（既有 `TestCraftSnapshotRestoreV1EditV3KeepsV2`：恢复只移基线+revision，versions 集不变）
- 并发 run 导致恢复冲突 ✓（既有 `TestCraftSnapshotRestoreRefusals`：queued/running/recovering/waiting_user 全部 ErrBusy + revision 竞态 ErrConflict；HTTP RestoreCompetitions 的活动锁段）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/application/service -run TestCraftSnapshotRestoreGuard -count=1` | 0 | 1 PASS |
| `go test -count=1 ./internal/application/service/` | 0 | ok（33.8s 全包） |

## 回退

revert 本提交（单测试文件，纯增量）。
