# CFT-S03-T021 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/application/service/craft_delegate_merge_test.go`（新增，3 tests，`TestCraftDelegateMerge*`）：主 Agent 归并语义 pin。
- 实现零改动：委派服务为 W04/R03 既有。

## 验收断言对照

- 子结束时主 Run 仍可继续修复 ✓（既有套件：`TestDelegateFreshGoesThroughPrepareTaskAndExecute`（子结果作为 ToolCall 答案回主）、投影不变量"childStatus 单独、complete 仅主 Run 终态"（presentation.test + T007 replay 测试）；子终态永不终结主 ToolCall——`PropagatesExecutorUnknownWithoutPersisting`）
- unknown 不终结 ToolCall ✓（新增 `MergeUnknownNeverTerminal`：ErrUnknown 透传、存储零持久化 + 既有 `KeepsPendingWhenObservationInconclusive`）
- 失败不替换旧交付版 ✓（新增 `MergeFailedRoundAnswersFailed`：定案 failed 是**答案**非错误；持久化结果无交付载荷（Files 空）——旧交付版本不受影响；版本侧"失败收集不发布"由 T019 pin + craft_runtime failed settle 持证）
- 最终回答引用真实已发布版本 ✓（新增 `MergeSucceededCarriesExecutorFactsOnly`：delegate 结果只携 executor 事实；版本引用只能来自容器在真实发布时发出的 artifact.published 事件——`TestCraftRunEventEmitterPayloadShape`（容器）+ W05 投影 artifactVersionIds 仅来自 artifact 事件 + e2e 工作台版本列表来自 versions API）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/application/service -run TestCraftDelegateMerge -count=1` | 0 | 3 PASS |
| `go test -count=1 ./internal/application/service/ ./internal/container/` | 0 | ok ×2 |

## 回退

revert 本提交（单测试文件，纯增量）。
