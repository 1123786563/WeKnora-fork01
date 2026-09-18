# CFT-S05-T032 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/agent/opencode/audit_redaction_test.go`（新增，2 tests，`TestCraftAuditRedaction*`）：审计面脱敏 pin。
- 实现零改动：关联观测面为 O04 既有（usage view Runs 带 status/failure reason/residency/as_of；metrics 低基数封闭词汇）。

## 验收断言对照

- 日志含关联 ID 与阶段 ✓（既有：craft_delegate.go 的 dispatch 行带 tenant/session/run/tool_call/delegation/request-hash；executor 事件 delegation.started{text/finished 按阶段；usage view Runs 从 UI 关联 run→status→failureReason→residency（`TestCraftUsageViewFailureResidencyAndAsOf`：failed+budget exceeded → 定位到执行与沙箱驻留阶段）；metrics 封闭词汇（`TestCraftMetricsLowCardinalityVocabulary`））
- 日志不含 secret/ticket/敏感附件正文 ✓（新增 `PayloadVocabularyIsClosed`：**事件载荷键封闭**——delegation.started 仅 prompt_message_id、delegation.text 仅 text、delegation.finished 仅 status；系统凭据/预览票据/原始工具输入输出没有任何代码路径进入事件（结构上不可能）；**交付文本有界**（64KiB bound，800 字文章不整篇传输）；预览侧票据摘要存储不落日志为 W02 既有语义）
- unknown 与取消超时可定位 ✓（新增 `UnknownIsLocatable`：unknown 轮仍发 started（可从 UI 追到 executor）但绝不发 finished（不虚构终态）；deadline 诊断由 T015/T023 持证）
- 未知 usage 单独可观测 ✓（既有 `TestCraftUsageViewUnknownStaysVisible` + T018 `UnknownNeverZero`：unknown 计数独立、不带伪造数字）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/agent/opencode -run TestCraftAuditRedaction -count=1` | 0 | 2 PASS |
| `go test -count=1 ./internal/agent/opencode/ ./internal/metrics/ ./internal/application/service/ -run "TestCraftAudit|TestCraftMetrics|TestCraftUsageView|TestCraftExecutionBudget"` | 0 | ok ×3 |

## 回退

revert 本提交（单测试文件，纯增量）。
