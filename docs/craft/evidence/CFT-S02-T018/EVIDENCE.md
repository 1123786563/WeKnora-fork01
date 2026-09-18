# CFT-S02-T018 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/application/service/craft_execution_budget_test.go`（新增，3 tests，`TestCraftExecutionBudget*`）：预算×用量接线矩阵汇总（真实 budget + usage service）。
- 实现零改动：有限预算/计量为 O04/G4 既有（`craft_budget.go`：Admit/AuthorizeBinding（并发末额恰一、撤销/过期/未注资拒绝、调用上限、商业预留）+ `craft_usage.go`：物理请求 UsageKey 去重、DeriveAttemptID 重试新事实、unknown 无数字规则、OC 汇总不重复计费 HandoffNeverRecounts）。client.go 的请求归属接缝（每物理请求身份）已由 usage 服务的 IssueCallID/IssueAttemptID 承载，无需改动。

## 验收断言对照

- 预算拒绝时无模型请求 ✓（`DeniedIssuesNoModelCalls`：未注资租户 Admit 允许但 AuthorizeBinding ErrBudgetDenied；零预留、零调用行——没有任何模型请求被放行/计量）
- 同物理请求事件重放不重复记 ✓（`ReplayAndRetryMetering` 前半：同 call/attempt 二次 Record 后聚合不变（Input 仍 100、Facts 仍 1）——UsageKey 去重）
- 实际重试不同请求如实计量 ✓（后半：新 attempt 记为自有事实（Input 160、Facts 2）——重试的真实成本如实入账）
- 未知 usage 不显示 0 ✓（`UnknownNeverZero`：断流事实 Facts=0/Unknown=1——未知计入 unknown 计数而非捏造 0 token；UI 层"待核对"由 T012 交付与 usage_view 既有 UnknownStaysVisible 持证）

## 既有覆盖枚举（本任务复用其实现）

budget：AdmitRegistersGrant/ReservesBudgetAndCountsCalls/LastQuotaConcurrentExactlyOne/Revoked/Expired/Unfunded/Extend/DifferentTenantsNeverMix；usage：CountsPhysicalAttempts/UnknownStaysSeparateAndLateUsageCorrects/HandoffNeverRecountsChildTokens/FundingAndUnknownChildVerification；主子归属不重复收费由 HandoffNeverRecounts 持证。

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/application/service -run TestCraftExecutionBudget -count=1` | 0 | 3 PASS |
| `go test -count=1 ./internal/application/service/` | 0 | ok（121.5s 全包无回归） |

## 回退

revert 本提交（单测试文件，纯增量）。
