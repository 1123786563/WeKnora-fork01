# Issue #83 执行账本（[Lago 11] 微信付款复用同一激活流程）

## 计划身份

- 计划文件：`docs/plans/issue-72-plan-83.md`
- Issue：https://github.com/1123786563/WeKnora-fork01/issues/83（`[Lago 11] 微信付款复用同一激活流程`）
- 编写者：计划员-83（dynamic workflow subagent，2026-09-27）
- 执行方式：superpowers:subagent-driven-development（计划头部声明）

## 集成基线

- Worktree：`.worktrees-issue72/issue-83`，分支 `codex/issue-72-lago-83`
- 基线提交：`c6d027df8`（`issue-72: ocr issue-82 round 2`；集成分支 `codex/issue-72-lago`）
- 基线核实（编写时实读实跑）：
  - #82 激活链已在基线交付：`CommandKindSettlePurchasePayment`（`purchase_settlement_command.go:25`）、`PurchaseFulfiller.Fulfill`（`purchase_fulfillment.go:63`）、回调匿名放行（`middleware/auth.go:71`）、attempt 商户身份修复（`service/commercial/order.go:403-405`）。
  - `go test ./internal/modules/commercial/payment/ -run TestWechat -count=1` → `ok ... 2.103s`（基线绿，本会话实跑）。
  - Issue 调查简报中「激活链完全缺失/lago.go 不带 activation_rules」的描述对应旧基线，已由本计划「基线核实」节修正。

## 计划自检结果（编写时执行）

- 占位符扫描：计划正文无 TBD/TODO/占位内容（`grep -n "TBD\|TODO\|待定\|占位" docs/plans/issue-72-plan-83.md` → 仅 Out-of-scope 与披露边界的明确声明，无未决项）。
- 接口一致性：`CloseChannelOrder(ctx context.Context, tenantID uint64, orderID string) (OrderView, error)`、`MarkAttemptClosed(ctx context.Context, orderID string) error`、`PaymentAttemptStateClosed = "closed"` 在 Task 3 定义、purchase 接线与追踪矩阵消费处签名一致；`purchase.go` 的 `domain` 别名与 `s.orders` 字段名已对源码核实。
- 追踪矩阵：4 条 Issue 验收标准 + 「复用同一激活流程」核心目标均映射到 Task/测试/证据（计划「验收标准 → Task → 测试 追踪矩阵」节）。
- 路径核实：计划引用的全部代码路径/行号在本 worktree 实读（wechat.go:399/428/446/449、payment_callbacks.go:67-114、order.go:584 ConfirmPayment、purchase.go:253-261、routes.tsx:80 等）；环境命令对齐 #82 r4-flow3 验证口径。
- 端口：:8095/:5196/:8296/:8297，避开 :5272/:5273 及既往占用。

## 移交记录（执行者须写入收尾）

- 废弃 pending 单超时回收调度 + 公开 cancel 命令：#84/#92（`CloseChannelOrder` 为其可复用 seam）。
- `RecoverOrderStatus` 查单见 CLOSED 的本地收口呈现：#84（本票仅切轨路径收口）。
- partial/multiple-success 异常付款完整面：#84。
- 微信真实商户凭据不可得：本地 RSA stub 披露边界（README 如实标注，不伪造沙箱证据）。

## 执行记录（执行者续写）

| Task | Commit | 结果 |
|------|--------|------|
| （待执行） | | |
