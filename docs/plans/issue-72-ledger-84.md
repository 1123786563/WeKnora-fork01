# Issue #84 执行 Ledger（[Lago 12] 异常付款不会扩大权益）

## 计划身份

- 计划文件：`docs/plans/issue-72-plan-84.md`（从零编写——集成分支 HEAD 无预写稿，`git -C ../lago-int show HEAD:docs/plans/issue-72-plan-84.md` 实测 `path does not exist`）。
- 计划员会话：计划员-84（dynamic workflow subagent），2026-09-28。
- Worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees-issue72/issue-84`，分支 `codex/issue-72-lago-84`。

## 集成基线

- 基线提交：`ee02d3218`（= merge #82 `7d614752c` + OCR issue-82 round 1）。#81/#82/#83 均已合入并可达；#85 并行实施中、**未合入**（DAG S1 边声明 #85 先行）。
- 计划对 #85 的处置：Global Constraints 第 11 条（开工前合并/rebase 到 #85 合入后的集成分支 + 合并纪律：只动 Recover/Close/OrderView 投影区，`packages/contracts/src/commercial.ts` 零改动）。

## 计划员自检记录（2026-09-28 实测）

| 检查 | 命令 | 结果 |
|---|---|---|
| 预写稿探测 | `git -C ../lago-int show HEAD:docs/plans/issue-72-plan-84.md` | `path does not exist`（从零模式） |
| writing-plans 技能 | 读 `/Users/wuyongjun/.codex/plugins/cache/openai-curated-remote/superpowers/6.4.2/skills/writing-plans/SKILL.md`（任务给的 6.4.1 路径不存在，6.4.2 为实际在位版本） | 已遵循（header/任务结构/自检清单） |
| 基线测试 1 | `go test ./internal/modules/commercial/repository/commercial/ -count=1` | `ok ... 0.517s` |
| 基线测试 2 | `go test ./internal/handler/ -run 'TestWechatCallback|TestAlipayCallback|TestPaymentCallback' -count=1` | `ok ... 1.139s` |
| 基线测试 3 | `go test ./internal/modules/commercial/service/commercial/ -count=1` | `ok ... 0.790s` |

## 关键调查结论（支撑计划的事实，全部本会话实读）

1. **seam 零改动可行**：`platform.go:225-234` 三方法冻结；Lago Payment 由 Lago 经 Stripe webhook 自记（`purchase_settlement_command.go:16-24` 注释即此红线），#84 只需保证不重复驱动 settle。
2. **G1 确认**：`repository/commercial/order.go:699-713` mismatch → `ErrPaymentMismatch` → `payment_callbacks.go:115-118`/`:170-174` 409 → 事务回滚零落库；既有测试 `TestWechatCallbackAmountMismatchRejectedNoFact`（payment_callbacks_test.go:682）锁定旧行为，Task 2 改造。
3. **G2 确认**：`service/commercial/order.go:535-539`/`:629-633` 用 `att.AmountFen` 造 fact；`payment/provider.go:57-61` AttemptResult 无金额字段；两渠道 Query 响应携带金额但丢弃（wechat.go:368-373 有 `Amount.Total`、alipay.go:374/392 有 `TotalAmount`）——加法可修。
4. **G3 确认**：over_payment 生产方在 `order.go:777-782`，全仓无消费方；drain 租约谓词 `fulfillment.go:231-234` 只取 `kind='fulfill'`。
5. **G4 确认**：`handler/commercial.go:284-312` orderWire 永不出 attention；`order-state.ts:2-7` orderMessage 无异常分支；契约 parser 已接受 attention（contracts commercial.ts:3,36）→ 契约零改动。
6. **G5 确认**：`service/commercial/order_test.go` 无 RecoverOrderStatus 晚成功幂等用例。
7. **错币种可达性**：微信 Verify 回传 payload 实际币种（wechat.go:293-303）；Alipay 通知契约无币种字段（hardcode CNY）——错币种仅微信腿可测，计划已披露。
8. **既有资产**：`flow-evidence-83/wechat_native_stub.py` 可编排金额/状态；`flow-evidence-82/alipay_gateway_stub.py`、`settle-evidence/deliver_stripe_webhook.py`、`_browser_lib.mjs` 可复用；lago 集成测试惯例 `//go:build lago_integration` + `LAGO_INTEGRATION_*`（lago_benefits_integration_test.go:1-46）。

## Task 执行记录

（执行阶段逐 Task 追加：提交哈希、测试输出摘要、偏差与披露。）

| Task | 状态 | 提交 | 证据摘要 |
|---|---|---|---|
| T1 异常事实持久化 | 未开始 | — | — |
| T2 回调终态应答 | 未开始 | — | — |
| T3 over_payment 处置 | 未开始 | — | — |
| T4 Query 实收比对 | 未开始 | — | — |
| T5 UI attention | 未开始 | — | — |
| T6 Lago 恰一次 | 未开始 | — | — |
| T7 真栈四幕 | 未开始 | — | — |
| T8 收尾门禁 | 未开始 | — | — |

## 残留与披露（计划定稿时点）

- 支付宝错币种不可造（渠道契约无币种字段）：以文档披露，不以微信证据替代。
- 渠道验证为本地 RSA stub（用户裁决 R-4 已披露边界），非沙箱证据。
- Task 6 无真实栈时 SKIP 记 blocked-env，权威侧断言由 Task 7 真栈轮补齐。
