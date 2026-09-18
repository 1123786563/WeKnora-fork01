# CFT-S02-T014 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/application/repository/delegation_identity_test.go`（新增，3 tests，`TestDelegationIdentity*`）：钉死委派身份三断言。
- 落点说明：任务卡建议 `internal/agent/opencode/delegation_identity_test.go`；核实时确认 PrepareTask 的幂等/冲突实现在 `internal/application/repository/craft_workspace.go`（W01/R02 交付，唯一 (tenant, run, tool_call) 行 + OnConflict 竞态分支 + sameDelegationRequest 冲突判定），断言 1/2 已有既有测试（craft_workspace_test.go:182 起）。本轮增量是断言 3（重启读回）+ 三断言的命名化钉死，放 repository 包复用真实 DB fixture（openCraftDB/seedCraftRun/putCraftWorkspace）——executor（craft_delegate.go:150）消费同一 Store.PrepareTask，协议面由 T013 pin 测试覆盖。

## 验收断言对照

- 重复键相同 payload 返回同 Task ✓（`SameKeySamePayloadReplays`：同 ID/PromptMessageID/RequestHash；**库中恰一行** craft_delegations——幂等边界是数据库唯一行，非内存 map）
- 相同键不同 payload 返回冲突 ✓（`SameKeyDifferentPayloadConflicts`：ErrConflict，绝不覆盖）
- 进程在准备后退出仍可定位远端消息 ✓（`SurvivesProcessExit`：新 CraftStore 实例同库读回——task id、PromptMessageID（远端消息绑定）、RequestHash、Run/Workspace 关联全部持久化；行状态如实为 'prepared'，先持久化后派发的顺序由状态语义承载）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/application/repository -run TestDelegationIdentity -count=1` | 0 | 3 PASS |
| `go test -count=1 ./internal/application/repository/ ./internal/craft/` | 0 | ok ×2（无回归） |

## 不允许项检查

- 无内存 map 去重（数据库唯一约束 + 事务内竞态分支）；未把 messageID 当 exactly-once 证明（PromptMessageID 只是绑定事实，delivery 由 T015 的核对语义裁定）。

## 回退

revert 本提交（单测试文件，纯增量）。
