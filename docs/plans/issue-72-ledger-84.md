# Issue #84 执行 Ledger（[Lago 12] 异常付款不会扩大权益）

## 计划身份

- 计划文件：`docs/plans/issue-72-plan-84.md`（从零编写——集成分支 HEAD 无预写稿，`git -C ../lago-int show HEAD:docs/plans/issue-72-plan-84.md` 实测 `path does not exist`）。
- 计划员会话：计划员-84（dynamic workflow subagent），2026-09-28。
- Worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees-issue72/issue-84`，分支 `codex/issue-72-lago-84`。
- 修订史：初版 `9e3df0fbd`；第 1 轮审查修订（7 条反馈，全部实读核实后修订，见下节）；**第 2 轮审查修订**（2 条反馈，均实读核实，见第 2 轮节）。

## 第 2 轮审查修订记录（2026-09-28，2 条反馈逐条核实）

| 反馈 | 严重度 | 核实手段（本会话实读） | 修订落点 |
|---|---|---|---|
| M1 `purchase.order?.payment_attention` 与契约零改动冲突（TS2339，typecheck:web 必 FAIL） | medium | 属实：`packages/contracts/src/commercial.ts:1-7` OrderView 字段集（id/payment/fulfillment/amount_fen/currency/checkout_url）无 payment_attention；PurchaseView.order?:OrderView 同（:176-183） | 采纳解法②（前端局部类型断言，契约零改动口径与 Global Constraint 11/S1 边均保持）：Task 5 Produces 与 Step 3 写明 `(purchase.order as (OrderView & { payment_attention?: boolean }) | undefined)?.payment_attention`（运行时 parseOrderView 未知字段透传保证在位）；自检结论新增「跨层类型一致性」条目 |
| L1 第三幕种子两细节未披露 + 存在更轻替代 | low | 属实：①stub 预下单端点 `POST /v3/pay/transactions/native` 受 `verify_outbound_signature` 保护（stub :256-263，401 SIGN_ERROR）——种子脚本预下单须实现商户 RSA-SHA256 出站签名；②`/stub/mark` 的 `transaction_id` 字段（stub :27、:243-244 直接写订单表）+ notify 推送即触发 sameTxn=false → over_payment，`TestWechatCallbackDifferentTransactionOverPaidAudit`（payment_callbacks_test.go:592）锁定同形态 | 第三幕主路径改为轻替代（同单换交易号 mark+notify，全真实回调链路，无需 DB 种子/无需预下单签名）；DB 种子降为可选补充（覆盖 AC2「多个 PaymentAttempt 成功」字面形态）并披露签名要求；Task 7 Produces 标注 seed 脚本可选；Task 3 引言同步；自检结论「真栈可执行性」更新 |

## 第 1 轮审查修订记录（2026-09-28，7 条反馈逐条核实）

| 反馈 | 严重度 | 核实手段（本会话实读） | 修订落点 |
|---|---|---|---|
| R1 非 succeeded 状态误判为异常+200 | high | 属实：`payment/wechat.go:304` Verify 直接 `mapWechatTradeState` 不过滤；`domain/order.go:54-60` ValidatePayment 把 State≠succeeded 与金额/币种统一 ErrPaymentMismatch | 新哨兵 `domain.ErrPaymentNotSucceeded`（`internal/modules/commercial/order.go`）；ConfirmPayment 入口先判状态（零落库回滚）；handler 双分支（NotSucceeded→409 维持 / Mismatch→200 终态）；Task 1 新测试 `TestConfirmPaymentNotSucceededStaysRollbackNoAnomaly`；Task 2 Step 2 预判修正（NonSucceeded 用例改造前后均须 PASS，回归锁定） |
| R2 stub `/stub/mark` 无金额字段，错金额主链不可执行 | high | 属实：`flow-evidence-83/wechat_native_stub.py:27`（mark 只收 out_trade_no/state/transaction_id）、:148/:228（amount 取 order["total"]，币种硬编码 CNY） | 真实验证方案新增**第 0 步 stub 副本改造**（`wechat_native_stub_anomaly.py`：mark 增 total/currency 覆盖，回调与 Query 用覆盖值，附双形态冒烟）；第一幕步骤 3 / 第四幕命令改为实际接口（字段名 `state`，金额走覆盖字段）；错币种变体并入同一覆盖机制；通过判据补冒烟前置 |
| R3 第二 attempt 无真实创建路径 | medium | 属实：grep 全仓 `OpenOrderCommand{` 非测试仅 `service/commercial/order.go:395` 一处；`RegisterAttempt`（repository :637）无非测试调用方 | Task 3 测试引言注明单测层直调 `store.RegisterAttempt` 的边界；第三幕新增种子脚本 `seed_84_second_attempt.sh`（参数绑定 INSERT 直种第二渠道 attempt，验证设施非产品路径，README 明示） |
| R4 orderWire 对 fulfilled+anomaly 行为未裁决 | medium | 属实：`handler/commercial.go:284-312` 三态 switch；`order-state.ts:2-7` 顺序 fulfilled→paid→closed | Task 5 Produces 显式**分派表**（attention 仅 pending 读数覆盖；paid→processing、fulfilled→fulfilled 不被覆盖；不变量：attention 只与 payment=pending 同现；PaymentAttention 字段随 fulfilled 单携带）；新测试 `TestOrderWireAttentionNeverOverridesPaidOrFulfilled`；BillingPage 两种后缀形态；第三幕补 fulfilled 单主状态断言 |
| R5 commercial_test.go 不存在 + Create/Modify 矛盾 | medium | 属实：`ls internal/handler/` 仅 commercial.go/commercial_benefits_test.go/commercial_purchase_test.go/commercial_task_budget.go；lago_settlement_integration_test.go 已存在 | Task 3 直接新建 `commercial_anomaly_test.go`（删 fallback 措辞）；Task 5 落点与 git add 同步改；File Structure lago_settlement_integration_test.go 改 Modify；L523 残留同步清理 |
| R6 buildMismatchAnomaly(attempt,row,fact) row 未声明 | low | 属实：第一个 mismatch 返回点 `order.go:699-702` 先于 `var row OrderRow`（:704） | 签名改 `(attempt PaymentAttemptRow, fact domain.PaymentFact) PaymentAnomalyRow`，Task 1 Step 3 注明理由 |
| R7 路径引用错置 | low | 部分属实：`deliver_stripe_webhook.py` 实测在 `flow-evidence-82/settle-evidence/`（与反馈一致）；`_browser_lib.mjs` **实测在 `flow-evidence-82/` 根目录**（反馈称 83/settle-evidence/，与 find 实测不符——`find docs/plans -name _browser_lib.mjs` 唯一命中 82 根；83 目录仅 api_recovery_83.mjs/browser_flow_83.mjs），「83 根下不存在」的指正成立 | platform.go 全路径化；deliver_stripe_webhook.py 与 _browser_lib.mjs 按实测路径改（82 目录）；Task 7 Consumes 注明实测依据 |
| R8 Task 4 测试 row 未声明 | low | 属实：RecoverOrderStatus 返回 OrderView 无 row 暴露（service :535-547） | 改 `view.State`/`view.PaymentAttention` |

## 集成基线

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
