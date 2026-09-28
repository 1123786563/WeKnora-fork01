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

（执行阶段逐 Task 追加：提交哈希、测试输出摘要、偏差与披露。执行员：实现员-84，2026-09-28，baseSha `de1ab4cd9`。）

### 前置门与 Ruling（执行员）

- **Ruling 0（#85 未合入，按合并纪律路径开工）**：前置门要求 merge/rebase 到 #85 合入后的集成分支，但 lago-int HEAD 仍为 `ee02d3218`（#85 未合入，issue-85 worktree 当前为 OCR-83 内容）。按 Global Constraint 11 替代路径：开工前 `go test ./internal/modules/commercial/... -count=1` 全绿（7 包 ok，2026-09-28 实跑）；只动授权区域（service/commercial/order.go 仅 Recover/Close/OrderView/withAttention 区、contracts 零改动）。集成冲突由集成分支主会话仲裁。错误代价：#85 合入时该文件可能需手工合解。
- **Ruling 1（ResolvePaymentAnomaly 带 expectedVersion）**：计划签名 `(ctx, id)` 与其自身「state+version 守卫」描述及 Task 3 的 expected_version/409 契约矛盾——定为 `(ctx, id string, expectedVersion int64)` + `ErrPaymentAnomalyNotFound`(404)/`ErrPaymentAnomalyVersionConflict`(409) 双哨兵。
- **Ruling 2（admin 路径取 /api/v1/admin/payment-anomalies）**：计划 Produces 文字 `/api/v1/commercial/admin/...` 与挂载指令「直挂父组（refund 先例）」矛盾——以挂载指令+refund-review 实际路径一致方为准。
- **Ruling 3（Task 6 用例并为既有用例子测试）**：独立函数需重跑 6 分钟 gated→settle→webhook 链（禁止本地强制 active），按计划「复用既有搭建」许可以 `t.Run("TestSettleSecondChannelTransactionShortCircuitsWhenActive")` 挂既有用例尾部，断言不变。
- **Ruling 4（真栈异常单占槽=正确保守行为，披露不改行为）**：attention 单保持 payable pending（防双重收款），新购买被 R2-26 回放；解锁=渠道侧关单（真栈演示完整闭环）。与 spec L127 一致，README 披露。
- **Ruling 5（settle 消歧闸 fail-closed 时人工补步结算腿，披露）**：共享栈历史同名 customer 残留触发 #82 distinct-invoice 闸正确 fail-closed；以与 settle step iii/iv 相同参数面的 Stripe API 两步补齐，后续轮 settle 幂等重放（F-1）。README 披露①。
- **Ruling 6（AC3 权威验证面随栈版本 Payment 行形态调整）**：82r5 栈 finalize 置 invoice payment_status=succeeded 但不铸 payments 新行——断言面= invoice 恰一次 succeeded + payments 基线（14|3）全程不变。README 披露②。
- **Ruling 7（前端基线既有失败披露）**：`pnpm test:web` 38 fail、typecheck 5 错——baseSha 临时 worktree（de1ab4cd9）实跑**完全一致**，失败全在 #84 未触碰域；commercial 三文件 7/7 过零新增错误；主 checkout 44/44 过但其工作区代码状态与集成分支不同（main 无 packages/ui/src/sheet.tsx、lockfile 不同）非有效对照。

### Task 1 异常付款事实持久化 —— commit cbf232d4c

- RED：`go test ./internal/modules/commercial/repository/commercial/ -run 'TestClassifyPaymentAnomaly|TestRecordPaymentAnomaly|TestConfirmPaymentMismatch|TestConfirmPaymentNotSucceeded|TestPaymentAnomalyList' -count=1` → build failed（`undefined: PaymentAnomalyRow` 等，实跑）。
- GREEN：`go test ./internal/modules/commercial/repository/commercial/ -count=1` → ok（新增 6 用例 + 既有不回归；修两处：`transaction` SQL 关键字带引号、`anomaly=&snap` 取地址）。
- 交付：payment_anomaly.go（表+闭合分类+Record/Has/List/Resolve 全参数绑定+唯一键幂等）、domain.ErrPaymentNotSucceeded、ConfirmPayment 入口先判状态+两 mismatch 点事务外独立落库、NewOrderService AutoMigrate 挂点。

### Task 2 回调面终态幂等应答 —— commit 1f69c94e7

- RED：mismatch 两用例 409≠200 FAIL；NonSucceeded 因 Task 1 新分类暂 500（任务顺序预期 RED）。
- GREEN：`go test ./internal/handler/ -run 'TestWechatCallback|TestAlipayCallback|TestPaymentCallback|TestCallbackMerchant' -count=1` → ok（13 用例全过含既有回归）。
- 交付：双渠道双分支（NotSucceeded→409 维持重试；Mismatch→200 终态）；落库失败自然走 500 渠道重试兜底。

### Task 3 over_payment 消费方与处置面 —— commit 1e2444142

- RED：drain 不租约 over_payment FAIL；handler 编译错（AdminList 未定义）。
- GREEN：`go test ./internal/modules/commercial/service/commercial/ -count=1` ok + `go test ./internal/handler/ -count=1` ok（修：grants granted_by NOT NULL、gin Use 须先于路由注册）。
- 交付：lease 谓词 kind IN (fulfill, over_payment)、disposeOverPayment（畸形→sent+Warn；Record 幂等；失败保持 pending 不阻塞同批 fulfill）、admin list/resolve 端点（平台守卫、409/404 闭合面）。

### Task 4 恢复/关单路径实收金额比对 —— commit 2c64cff86

- RED：AmountFen/PaymentAttention 编译错（预期）。
- GREEN：`go test ./internal/modules/commercial/payment/ -count=1` ok + service 全量 ok（修断言：5000<9900 闭合分类为 partial_payment）。
- 交付：AttemptResult.AmountFen、双渠道 Query 回传（alipay 不可解析降级 0）、Recover/Close 先比后确认、recoverMismatchedCollection（落 anomaly+attention）。

### Task 5 付款异常 UI 投影 —— commit 9b98c1229

- RED：orderWire 两用例 FAIL、GetOrderSurfaces FAIL、前端 not ok 1。
- GREEN：`go test ./internal/handler/ -run 'TestOrderWire|TestAdminPaymentAnomalies'` ok + service ok + `npx tsx --test src/commercial/order-state.test.ts` 7 pass 0 fail；全量 test:web/typecheck 的 38 fail/5 错均为基线既有（Ruling 7）。
- 交付：OrderView.PaymentAttention、withAttention 读路径（Recover 三返回点）、PurchaseStatus 投影点、orderWire R4 分派表（attention 仅 pending、flag 全态透传）、order-state attention 前置分支、BillingPage 两形态后缀（M1 局部类型断言，契约零改动）。

### Task 6 Lago Payment 恰好一次 —— commit 687867d52（blocked-env）

- 常规：`go test ./internal/modules/commercial/commercialplatform/ -count=1` → ok（72.5s）。
- tagged：`go test -tags lago_integration ./internal/modules/commercial/commercialplatform/ -run TestLagoIntegrationSettleActivatesGatedSubscription -count=1 -v` → **SKIP（lago integration env not configured）**——lab.env（gitignored）缺失，webhook secret mint 链（prepare_t9_env.sh 依赖 lab.env）不可完整重组；skip≠pass 披露。短路单测锁定在位（lago_settlement_test.go:278 already-active 零 provider 调用）；真栈第三幕 payments 基线不变为结构性权威证据。

### Task 7 真实流程验证 —— commits f1eb7c167（实抓缺陷修复）+ 767970cdd（证据）

- **实抓缺陷（f1eb7c167）**：第一轮第一幕错币种通知落 anomaly 后，浏览器轮询触发 RecoverOrderStatus——collected 比对只比金额不比币种（实收恰等面额）→ fact 用 attempt 的 CNY 构造 → **错币种收款经恢复路径洗白成正常确认**。修复：AttemptResult.AmountCurrency + 微信 Query 回传 + collectedAmountMismatch 双维比对（金额或币种任一不符分流 anomaly）；回归锁 `TestRecoverOrderStatusWrongCurrencyRetainedAsAnomaly`（RED→GREEN）；`go test ./internal/modules/commercial/... -count=1` 7 包 ok。
- **四幕全过**（重置栈复跑，证据 `docs/plans/issue-72-flow-evidence-84/`，README 含判据核对）：
  - 第一幕（租户 21 微信单）：部分付款 5000→200 终态+重投 200+partial_payment；USD→currency_mismatch；恢复读后仍 pending（缺陷修复验证）；Lago 零激活零支付；UI 三截图。
  - 第二幕（租户 23 全新 Lago customer）：正常通知→paid→settle→真实 PI webhook（provider stored secret HMAC）→sub active+invoice succeeded→fulfilled+active；重投 ×3 全 success、records 恰 1 applied、Lago payments 基线 14|3 不变；UI 两帧。
  - 第三幕（同单换交易号）：over_payment 事件→drain ~14s→anomaly(0/9900)；records 仍 1、Lago 基线不变；主状态 fulfilled+payment_attention:true（R4）；处置面 list/resolve 200/重放 409/全处置后 attention 回落。
  - 第四幕（租户 24 漏通知+实收 19900）：GET 恢复读→attention+pending+amount_mismatch(9900/19900)+attempt pending；Lago sub status=4 零激活、基线不变。
- 凭据纪律：sk_test/whsec 零命中、登录 REDACTED、密钥只经 shell 变量。
- 栈侧过程：secrets env 裸 KEY=value 未导出→unconfigured（改 export 前缀+tmp-probe 定位，探针已删）；支付宝通知交易号复用撞 provider_transaction_id 唯一索引 500（渠道全局唯一约束正确拦截，换号重投）。

### Task 8 收尾（门禁实跑记录，2026-09-28）

| 命令 | 结果 | 说明 |
|---|---|---|
| `make test` | **PASS**（exit 0，126 包 ok） | 首跑 architectureguard 2 用例失败——新增 2 条 admin 路由使计数基线 635→637 偏移；按「基线随代码同步」惯例更新 discovery_test.go 基线（568/637）后全绿 |
| `make lint` | exit 2（438 项） | **基线既有**（knowledge/knowledgebase 等未触碰域 + worktree 缓存 warning）；#84 改动域唯一引入项（order_test.go DropTable errcheck）已修，`golangci-lint run` 改动域（commercial/handler/router）零 #84 引入残留（余项 alipay_test:273/order_test:147/152/commercial.go:917-963 均经 git diff 基线行核实为既有） |
| `make check-backend-architecture` | **PASS**（0 violations，literal=568 total=637） | 基线同步后过 |
| `pnpm test:web` | 2274 pass / 38 fail | 38 fail 与 baseSha 完全一致（Ruling 7，临时 worktree 实跑取证）；commercial 域 7/7 过 |
| `pnpm typecheck:web` | 5 error | 与 baseSha 完全一致（Ruling 7）；#84 触碰文件零错误 |
| `pnpm test:shared` | 988 pass / 3 fail | 3 fail（kbDetail 域）与 baseSha 实跑**完全一致**（992 tests / 988 pass / 3 fail，临时 worktree 对照）——基线既有 |
| `make verify-module-moves` | 未跑 | 本计划未触及模块搬迁清单文件（计划预期不需要） |

栈收尾：stub/后端/前端已停（8096/5197/8298/8299 down）；Lago 82r5 栈为宿主共享资产未动。

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

## OCR attempt identity repair checkpoint — 2026-09-28

A narrow post-audit SDD repair was completed in `codex/issue-72-lago-84` at BASE `c3279f232f6ea38ae5d59960a43f8864f0103fdf`, using plan `docs/plans/issue-72-plan-84-ocr-attempt-id-fix.md`. The repair preserves the first successful transaction ID on each payment attempt. A later distinct success remains one over-payment audit fact and cannot replace the idempotency identity used by delayed settlement.

- TDD sequence: `txn_first → txn_second → txn_first replay → txn_second replay` verifies immutable winner, one fulfillment, and only `txn_second` in one over-payment audit. A separate unique transaction collision verifies attempt/order/outbox rollback.
- PostgreSQL tagged race test uses a two-arrival pre-claim barrier and observes exactly one conditional claim update with `RowsAffected=1`, one with `RowsAffected=0`; it checks one immutable winner, one fulfillment and one loser audit.
- Delayed settlement service test retries the paid outbox event twice and verifies both settlement commands use the first transaction and stable command key; one grant results.
- Verification commands and outputs: see `.superpowers/sdd/issue-72-plan-84-ocr-attempt-id-fix/task-1-report.md`. Targeted repo regressions, full `repository/commercial` package, delayed-settlement test, tagged PostgreSQL race and `git diff --check` all passed. The PostgreSQL test ran against an isolated local PostgreSQL 17 container; it was stopped after the run.
- Independent task review R1 `/tmp/issue72-84-attempt-identity-task-review-r1.md`: Spec PASS, Code Quality PASS with one low test-harness timeout/race-diagnostic finding. Fix round 1 addressed it; scoped R2 review `/tmp/issue72-84-attempt-identity-task-review-r2.md` confirms Q1 addressed and both Spec Compliance and Code Quality PASS. Frozen checkpoint hashes and full diff are under `.superpowers/sdd/issue-72-plan-84-ocr-attempt-id-fix/task-1-checkpoint-r1/` (diff SHA-256 `5dcaa14992976873a6fbbb6156e95f29960c1d3b43a241765a68df374fa7624b`).
- This is a verified code checkpoint for the narrow transaction identity defect. It does not close Issue #84 as a whole: inherited issue-level OCR and acceptance disclosures remain separately tracked; no push or GitHub Issue mutation occurred.

## Cross-attempt settlement R4 checkpoint (2026-09-29)

- R4 plan `docs/plans/issue-72-plan-84-cross-attempt-settlement-fix-r4.md` is independently plan-reviewed PASS after three scoped amendments; final confirmation `/tmp/issue72-84-cross-attempt-plan-r4-final-approval-20260929.md`. It supersedes the rejected R3 plan for this repair.
- **Ruling:** For a deterministic invalid/missing winner on a nonempty-QuoteID order, persist the existing `purchase_activation` Attention fact before quote/purchaser routing and keep the event Pending; for the explicitly legacy empty-QuoteID top-up, retain Dead/no-record behavior. This preserves the established subscription operator state even if the quote store or purchaser wiring is unavailable, while neither path may grant or settle. Cost if this conservatively includes a quoted top-up: that top-up may receive a purchase-activation Attention fact and require operator reconciliation instead of immediate Dead quarantine.
- Cumulative implementation checkpoint remains based on `ae8f57c8cb33b15f7ec78cab3758ae00d3bd3cbe`; uncommitted five-file diff SHA-256 `34862162fad2a6c4d5beeaa7ff487240790cd7b943f942c6a502db61242f4dbc`. R4 write ownership was `fulfillment.go`, `purchase_fulfillment.go`, and `fulfillment_test.go`; the earlier R1/R2 `order_pg_test.go` and `purchase_fulfillment_test.go` deltas remain included in the cumulative review package.
- Verification: independent cumulative task review `/tmp/issue72-84-r4-cumulative-task-review-20260929.md` — Spec Compliance PASS and Code Quality PASS, no blocking finding. Independent validator `/tmp/issue72-84-r4-validation-20260929.md` — all 18 named regressions, full service package, repository+service packages, gofmt, diff check and checkpoint hashes PASS.
- Environment limit: `SAAS_TEST_PG_DSN` is absent; ordinary test selection excluded the `commercial_integration` race case (`[no tests to run]`), and tagged PostgreSQL runtime was not run. This is not a pass for real PostgreSQL concurrency. Deployment-side census of historical fulfill events without winner identity remains a rollout gate. No live service, remote Issue, push, or deployment action occurred.
