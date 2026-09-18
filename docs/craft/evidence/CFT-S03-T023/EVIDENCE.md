# CFT-S03-T023 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/craft/stop_status_test.go`（新增，1 test × 5 矩阵）：取消受理语义 pin。
- 实现零改动：决策持久化/送达/竞态为 C02 既有。

## 验收断言对照

- 取消后到达批准被拒绝 ✓（store 语义：已 decided/canceled 的交互是终态 → **ErrGone(410)**——`TestInteractionStoreDecisionIdempotencyAndConflicts` 的 terminal 段；delivery 侧 `TestDeliveryTargetsPendingFilterAndGoneRun`：canceled run 后决定不转发（replier 零调用））
- 决定写入成功但响应丢失不重复许可 ✓（既有**进程级 SIGKILL 故障注入矩阵**：`TestDecisionSurvivesKillAfterSavedDecision`（决定+outbox 已提交未转发）+ `TestDecisionNoDuplicateAfterAcceptedButUnackedKill`（OC 已接受、ack 未持久化）——重启后恰一次送达、同 decision id、零重复）
- payload revision 变化令旧批准失效 ✓（既有 `IdempotencyAndConflicts`：stale expected revision → ErrConflict；同 id 异 payload → Conflict；`TestDeliveryRefusesSupersededDecision`：参数已变 → 拒绝转发）
- abort 受理不直接声称已终止 ✓（新增 `StopStatusAcceptanceNeverClaimsCancellation` 五态矩阵：无请求→running；**受理未证实→stopping**；aborted 未 idle→stopping；idle 未 aborted→stopping；aborted+idle 双证→canceled；前端配对：T012 的 stopping→'正在停止' 投影 pin）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/craft -run TestStopStatus -count=1` | 0 | 1 PASS（5 矩阵） |
| `go test -count=1 ./internal/craft/ ./internal/application/service/` | 0 | ok ×2（含 SIGKILL 故障注入矩阵） |

## 回退

revert 本提交（单测试文件，纯增量）。
