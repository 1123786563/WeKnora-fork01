# [Lago 10 / Issue #82] 支付宝付款后恰好一次激活套餐 — Implementation Plan（R-4 双轨道修订版，第 3 轮）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让一次经渠道验签的支付宝付款，按 R-4 裁决的 α 双轨道（渠道真实收款 + WeKnora 驱动 Stripe Provider gated 结算扣款 → Lago 内建 webhook finalize → active）恰好一次激活 payment-gated 订阅：渠道回调只入账一次、Outbox 重放与响应丢失不重复履约，产品状态按「待付款 → 已付款待激活 → 已生效」稳定呈现，激活后套餐 Entitlement 与首期套餐 Credits 到账，并留存关联 Invoice/Payment/Subscription/Credits 的真实栈证据（支付宝侧为本地 RSA stub 协议往返，已披露边界，不伪造沙箱证据）。

**Architecture:** 沿用 #81 冻结的 additive seam 模式。seam 新增命令 `settle_purchase_payment`：读 authority 订阅快照（active 即幂等返回）→ Stripe 侧按 customer+`metadata.lago_invoice_id` 定位挂死的 gating PaymentIntent（t10 P1' 实证）→ attach 结算 pm → 对该 PI `update payment_method + confirm`（off-session 同步扣款）→ 激活由 Lago **内建** webhook 链（`POST /webhooks/stripe/:org_id` → `PaymentIntentSucceededService` → payment succeeded → `handle_payment_gated_activation` → invoice finalized + subscription active）完成，WeKnora 不触碰任何 Lago 状态写入。服务层把 `OrderKindPurchase` 的 fulfill 事件从旧 gateway 路由到新 `PurchaseFulfiller`：settle 触发 → 观察 purchase 快照 active → finalized 行项目复核（proration 剪裁判据修订）→ 本地投影切换 → 以购买身份发放首期套餐 Credits（独立钱包名+独立 meta 键）。产品态新增协调层合成的 `paid_awaiting_activation`（订单 paid + authority 未观察到 active）。恰好一次由三层合力：渠道层 ConfirmPayment 幂等单事务（既有）→ outbox `fulfill:<order>` 唯一键（既有）→ Lago 侧 payments 部分唯一索引 + `ResolveService` no-op 除非订阅仍 incomplete 且 invoice 仍 open（t02 实证）。同时清偿终审 OCR round-4 购买/订单/回调链路全部有效 findings（双重扣款竞态、SSRF Lago 侧出站校验、错误分类、凭据字面量 env 化等）。

**Tech Stack:** Go（gin + gorm，SQLite/PostgreSQL 双方言）、pinned Lago Community v1.53.0（`getlago/lago-api@591ae900`，digest 锁定 compose，栈 `weknora-lago-82flow` 实测在跑：48889 API healthy / 48890 front）、Stripe TEST 作受支持 provider 轨道（凭据仅 `~/.zcode/issue72-stripe.env` source 注入）、React+Vite 前端（apps/web）、TypeScript 共享契约（packages/contracts、packages/api-client）、Playwright 真实浏览器验证。

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`（"Quotes, payments, and fulfillment" L118-127、"Public product states" L167-171、"Local persistence and state projection" L159-165、"Behavior and contract matrix" #5 L218 / #20 L233）；决策依据 `docs/migrations/lago/t02-payment-activation/DECISION.md`（§3 provider 轨道、§5 裁决记录）、`docs/plans/issue-72-user-rulings.md` R-1/R-3/R-4、`docs/adr/0012-lago-as-commercial-billing-authority.md`（含 2026-09-23 修订）、ADR-0014（seam 冻结）、`docs/plans/issue-72-flow-evidence-82/README.md`（三缺陷已修状态 + 两个观察项）。

**修订历史：** 第 1/2 轮计划（D2=cancel intent+切 pm+`retry_payment`）已被 t10 证伪并随 `e0364d196` revert（git 历史 `e0364d196^` 可查）；本版为 R-4 裁决（2026-09-26，`9393ce0da`）授权的重做版，吸收集成分支 flowfix 修复（`7324eb054`：回调 Auth 白名单 / `MerchantID()` 商户身份 / 客户绑定 upsert）与终审 OCR round-4 findings 修复范围。基线：集成分支 `codex/issue-72-lago` @ `f50c705074`（本 worktree 已 fast-forward 合并，无冲突）。

---

## Global Constraints（批准需求原文 + 安全约束，每个 Task 隐含遵守）

1. spec L122：「Initial subscriptions are pay-in-advance, have no trial, and use a payment activation rule. They remain incomplete until the gating payment succeeds.」
2. spec L123：「WeKnora owns WeChat Pay and Alipay request creation, callback verification, query, close, and refund behavior. A verified channel result produces an immutable Payment Fact.」
3. spec L124-125：「A supported Lago external-payment integration records the Payment. Only a Lago Subscription observed as active enables Entitlement and fulfillment.」「If Lago manual Payment does not release the activation rule in the pinned Community version, the implementation must use a Lago-supported external Payment Provider adapter. It may not force the Subscription active locally.」——**产品代码禁止**：伪造 provider webhook、直写 Lago 数据库、对 subscription 状态的任何本地直写、调用 Premium manual `POST /api/v1/payments`（Community 403，t02-manual 实证）。
4. spec L126：「Paid Wallet top-up is not spendable until Lago confirms the Wallet transaction. Response loss is recovered with the original invoice, channel, and idempotency identities.」——settle 与 grant 的幂等身份绑定渠道支付单号 / 确定性身份。
5. spec L169：「WeKnora maps provider state into stable product states: awaiting payment, paid awaiting activation, active, …」；L170：公开 Billing API 永不暴露 Lago 凭据/URL/Organization id/原始状态枚举（incomplete 等永不过线）。
6. spec L105：「Monetary values use integer minor units and never pass through binary floating point.」；初期 CNY only。
7. spec L165：「WeKnora-to-Lago commands use a durable Outbox and stable idempotency identity. Timeout and server error are indeterminate outcomes that must be queried before replay.」
8. R-1/R-4 裁决：付款激活通道 = Lago 原生 Payment Provider 集成（选项②）；激活链 = α 双轨道。凭据纪律：Stripe/支付宝凭据只从环境变量读取（`~/.zcode/issue72-stripe.env` 仅 source 注入），任何产出（代码、测试、文档、证据、日志）不得出现 `sk_test` 等可用凭据字面量。
9. 安全约束（实现验收条件）：服务端发起外部请求仅允许 http/https 且请求前校验 host（拒绝 localhost/环回/私有/保留地址，复用并**补全覆盖** `validateOutboundHost`）；数据库查询一律参数绑定；源码/示例/测试无凭据字面量。
10. 冻结 seam 纪律（ADR-0014）：`platform.go` 三方法签名（`SubmitCommand(ctx, cmd Command) (CommandReceipt, error)` 等，platform.go:226-231）不变，仅 additive（新 CommandKind / 新可选字段）；provider 词汇（payment_intents、webhooks、invoices URL 与字段）只存在于 `commercialplatform/` 包内，绝不进入 port 类型、服务层或前端契约。
11. R-3 强制条件④：一切偏差仅限本计划列明条目（判据 proration 修订见 D6'，已由 R-4 附带授权吸收 flow-evidence 观察项），不得外溢。
12. R-4 已披露边界：支付宝真实沙箱凭据缺失——用本地 RSA stub 证明协议往返+验签，**不得伪造沙箱证据**；AC4 证据如实标注 `ac4-sandbox-credentials-unavailable` 残余。

## 实证契约基线（本计划编写会话核实；执行者不得凭 OpenAPI 假设覆盖）

- **F1（运行时，t02-activation.json）**：gated 订阅 + default pm 直接可扣（pm_card_visa）→ Lago auto-collection **同步** succeeded → `subscription_status: active`、恰 1 笔 succeeded provider payment（`provider_payment_id: pi_…`）、invoice `finalized`+编号+`payment_status: succeeded`、entitlements 200、totals_match。
- **F2（运行时，t02-duplicates.json）**：重复探测无害——同 external_id 再 POST 订阅 422；对已支付 invoice `retry_payment` 405 `invalid_status`；manual 403；最终状态 byte-identical。
- **F3【t10 证伪】**：gating invoice 隐藏窗口（open/closed ∈ INVISIBLE_STATUS）内 `POST /api/v1/invoices/{id}/retry_payment` → 404 `invoice_not_found`（t10-trigger-p2.json）。**禁止据此设计触发**。
- **F4【t10 证伪】**：挂死窗口内 `GET /api/v1/payments?external_customer_id=` → 200 `payments: []`（payment 行仅落 authority DB，`visible_payable_condition` 过滤）。**禁止据此定位 invoice**。
- **F5（运行时，t10 P1' PASS）**：Stripe 侧 `GET /v1/payment_intents?customer=` 可找到 unsettled intent，且每笔 Lago provider payment 的 PI 携带 `metadata.lago_invoice_id`（pinned `Invoices::Payments::CreateService` 打的 metadata）——**这是受支持的 provider 侧定位器**。
- **F6（运行时，t10 P2a/P2b PASS）**：Stripe 侧 cancel 挂死 intent 200；attach 结算 pm + 设 customer default 200。
- **F7（运行时，#81 flow evidence）**：`pm_card_threeDSecure2Required` 作下单 pm → 收款停 `requires_action`、Lago payment=processing、gating invoice 保持 open、订阅保持 incomplete——**稳定待付款窗口**；`timeout_hours:0` 下不自动取消。
- **F8（源码核实，本会话经 raw.githubusercontent 实取 pinned 591ae900）**：挂死 payment 的推进**只有 webhook 一条路**——`Invoices::Payments::CreateService`：`if processing_payment ... return result.payment ...` 注释原文 "Status will be updated via webhooks"。
- **F9（源码核实，本会话实取）**：Stripe webhook 接收链 `POST /webhooks/stripe/:organization_id`（config/routes.rb:67）→ `WebhooksController#stripe` → `InboundWebhooks::CreateService` → `ValidateIncomingWebhookService` 以 `Stripe::Webhook::Signature.verify_header(payload, signature, provider.webhook_secret)` 验签 → 落库 + `ProcessJob`；`payment_intent.succeeded` → `PaymentProviders::Stripe::Webhooks::PaymentIntentSucceededService` → `update_payment_status!("succeeded")` → payment 行 succeeded →（t02 源码链）invoice `payment_status: succeeded` → `handle_payment_gated_activation` → `Payment::ResolveJob` → invoice **finalized** + subscription **active**。
- **F10（源码核实，本会话实取）**：`RegisterWebhookService` 在 provider 注册时 `Stripe::WebhookEndpoint.create`（URL 基于 `LAGO_API_URL`）并把 Stripe 返回的 `secret` 存 `payment_providers.webhook_secret`。本地栈 Stripe 云端无法投递回环 URL → **本地验证需 harness 用 DB 中真实 secret 对真实 PI 事件计算 `Stripe-Signature` 投递到 `/webhooks/stripe/:org_id`**（传输腿替身；事件体=Stripe API 读回的真实 PI 对象，签名=真实 secret 的真实 HMAC，接收与处理链 100% Lago 内建）。
- **F11（本会话实读代码）**：`grant_included_credits` 钱包身份两层匹配 `byName(MonthlyWalletName)`+`byMeta(meta[tenant]==ext && meta[period]==period)`（lago.go/fake.go）——购买发放必须用独立钱包名+独立 meta 键，否则与 Base 月度批次撞身份触发 `grant content conflict`。
- **F12（运行时，flow-evidence-82 观察项）**：gating invoice 首期金额可能被 Lago calendar proration 剪裁（实测 9/25 下单、周期对齐 9/30 → `total_amount_cents=1980` ≠ plan 9900）。**判据修订输入**（D6'）。
- **F13（本会话实跑）**：合并基线 `go build ./...` exit 0（仅 ld 重复库告警）；`go test ./internal/modules/commercial/commercialplatform/ ./internal/modules/commercial/service/commercial/ ./internal/handler/ ./internal/router/ -count=1` → **4 包全 ok**（72.0s/1.6s/3.2s/6.1s）。`npx playwright --version` → 1.60.0（实测）。
- **F14（本会话实测）**：`docker ps` —— `weknora-lago-82flow` 栈正在运行（api 127.0.0.1:48889 healthy、front 48890，44 分钟前启动，seed org `WeKnora Issue82 Flow` + Stripe provider `weknora-stripe` 已注册）；WeKnora dev 容器 postgres:5432/redis:6379/docreader:50051 在跑。端口占用实测：48889/48890/50051/5432/6379 等；**:5272/:5273 当前未占用但为其他会话保留，禁用**；:8080/:8084/:8091/:8092/:5183/:5192 为既往验证口径，避开。
- **F15（本会话实读）**：flowfix 三缺陷已在基线修复：回调匿名可达（`noAuthAPI` 白名单 + `internal/router/commercial_callback_public_test.go`）；`payment.Provider.MerchantID()`（provider.go:97）+ openOrder 注册 attempt 用商户身份；客户绑定走 collection POST upsert（lago_purchase.go:301-376）。终审 OCR round-4 购买/订单/回调链路 findings **全部未修**（实核：commercial.go 无 `ErrQuoteNotFound` 分支、order.go:119 回填无 `created_at IS NULL` 限定）。

## 设计决策记录（本计划定稿，执行者按此实现）

- **D1 并存口径（R-4 的机制落点）**：支付宝渠道是**收款事实**通道（spec L123）；Lago Payment 是**激活结算事实**，由 Stripe Provider 在 authority 内部创建。WeKnora 协调层关联两者：可信渠道付款事实（ConfirmPayment 落账 + outbox 事件）→ `settle_purchase_payment` 驱动 Stripe 侧 gated PI 扣款 → Lago 内建 webhook finalize → 观察 purchase 快照 active 才开放履约。下单时 gated create 不动（#81 形状，R-3 付款前匹配防线保留）。
- **D2' 激活触发机制（替换被证伪的 D2）**：settle 命令算法——(i) 读 purchase 快照：active → 幂等回执、零出站；(ii) Stripe 侧 `GET /v1/payment_intents?customer={pcid}` 定位 `status==requires_action` 且 `metadata.lago_invoice_id` 非空的 gating PI（F5）；(iii) attach 结算 pm（新 env `WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN`，如 `pm_card_visa`）得 clone id，设 customer default；(iv) `POST /v1/payment_intents/{pi}` update `payment_method` → `POST /v1/payment_intents/{pi}/confirm` → succeeded（off-session 同步扣款，与 F1 的 confirm 语义同类）；(v) 激活交还 Lago 内建 webhook 链（F9），WeKnora 不再调用任何 Lago 写 API。**步骤 (iv) 的「requires_action PI 可 update payment_method + confirm」未经运行时实证——Task 1（t11 探针）第一判据；若实证失败，按升级条款停止 Task 5+ 并升级，不实施替代猜测路径。**
- **D3 产品三态**：`paid_awaiting_activation` 是**协调层合成态**（本地订单 paid + authority 快照非 active），永不来自 authority 观察。常量落点唯一：`internal/modules/commercial/purchase_command.go`（`PurchaseState*` 权威集同区，:106-116 之后），注释注明 coordinator-composed；`service/commercial/purchase.go` 仅消费。seam 的 authority 观察集不变——`readPurchaseSnapshot` 永不产出该值。authority canceled + 本地 paid = `canceled` 呈现 + fulfillment `attention`（异常付款面留给 #84，本票只保证不扩大权益）。
- **D4 套餐首期 Credits 发放身份**：命令键 `GrantCreditsCommandKey(ExternalPurchaseSubscriptionID(t), period)`（独立键族）；钱包名 `PurchaseWalletName(t) = ExternalPurchaseSubscriptionID(t)+"-"+period`；钱包 meta 写 `tenant` + 新键 `purchase_period`（**不写** `period` 键，F11 防撞 Base 批次）。`GrantIncludedCreditsPayload` additive 可选 `WalletName string`（空 = 现行 `MonthlyWalletName`，既有调用零改动）。
- **D5 旧 gateway 路由**：`FulfillmentService.fulfillEvent` 对 `row.Kind == domain.OrderKindPurchase` 分流到注入的 `*PurchaseFulfiller`（fulfillment.go:280 `OrderKindUpgrade` 分流点之后的对称位置）；top-up/upgrade 路径一行不动。
- **D6' 行项目复核判据（proration 修订，flow-evidence 观察项输入）**：finalized 后 `PurchaseSnapshot.InvoiceFees` 必须恰 1 条 `Kind=="subscription"` 行，且满足 `0 < fee.AmountFen == invoice.total_amount_cents ≤ Purchase.AmountFen`（proration 只会剪小，F12），同时该 invoice `payment_status=="succeeded"`；金额精确相等断言（旧 D6 的 `==AmountFen`）改为**订单面额一致性独立判据**：渠道收款金额 == 报价 PriceFen == 订单 AmountFen（渠道层，`ConfirmPayment` 既有校验）。proration 差额（fee < 订单面额）记入证据披露（T02 §5 输入），不阻断履约、不据此扩权。复核失败 → fulfillment `refused`（权益不发放，事实保留，#84 面）。
- **D7 恰好一次的三层台账**：渠道层 `ConfirmPayment` 幂等单事务（repository/commercial/order.go:531 既有）；outbox `fulfill:<order>` 唯一键（既有）；settle/grant 命令确定性幂等身份 + Lago 侧 payments 部分唯一索引 + `ResolveService` no-op（F1/F2）。`FulfillmentRecord`（fulfillment.go:41-58）扩展一行 `purchase_activation` 承载 settle→grant 的外部回执。
- **D8 webhook 投递替身边界**：产品代码**绝不**投递/伪造 webhook（GC-3）；仅验证 harness（Task 10 的 `deliver_stripe_webhook.py`）在本地栈替代「Stripe 云端 → 回环 Lago」这一传输腿——事件体从 Stripe API 读回的真实 PI 对象构造，签名用 Lago DB 中该 provider 的真实 `webhook_secret` 计算，接收端是 Lago 真实路由与内建处理链。生产形态下该腿由 Stripe 自身完成（F10），harness 不进任何产品路径。

## Review Focus（spec 隐含但无任务测试会咬人的输入类；每条注明归属测试）

1. **重复回调中第二笔不同交易号的成功付款**（用户连刷两次付款）：第一笔赢得 fulfill 权，第二笔只入审计事实、绝不二次履约/二次 settle——归属 Task 7 `TestPurchaseFulfillOverPaymentNoSecondBenefit`。
2. **settle 命令在 authority 已 active 后的迟到重放**（响应丢失 + outbox 重放叠加）：零出站、同回执返回，不得再 attach/再 confirm——归属 Task 5 `TestLagoSettleAlreadyActiveNoOutbound` + Task 3 `TestFakeSettleIdempotentAfterActive`。
3. **同步返回页与一切未验签入口**：`VerifySyncReturn` 恒返 `ErrAlipaySyncReturn`（alipay.go 既有），任何伪造「已付」查询串不得推进状态——归属 Task 7 `TestSyncReturnCannotConfirmPurchase`（恢复 #82 冻结前已落的 pin，`internal/modules/commercial/payment/alipay_test.go`）。
4. **settle 执行到一半进程崩溃**（PI 已 confirm、fulfiller 未观察 active）：下一轮 drain 重放 settle → active no-op 幂等 → 履约收敛不产生第二笔 payment——归属 Task 7 `TestPurchaseFulfillCrashReplayConverges` + Task 1 探针 P-E（Lago 侧 no-op 证据）。
5. **authority 已 canceled 却收到渠道成功付款**（超时边界竞态）：订单已付但权益永不开放，状态诚实呈现 canceled、fulfillment `attention`，不伪造生效——归属 Task 7 `TestPurchaseFulfillCanceledStaysClosed`。

## File Structure（新建/修改全景）

- Create: `deploy/lago-lab/payment-settle-trigger/`（lab.sh / run_lab.py / phases.py / fixtures.py / test_phases.py / evidence/）——Task 1 t11 探针（骨架自 `git show ca04d7da7:deploy/lago-lab/payment-trigger/` 恢复后扩展）
- Create: `docs/migrations/lago/t11-payment-settle-trigger/`（DECISION.md + 证据 JSON）——Task 1/10
- Create: `internal/modules/commercial/purchase_settlement_command.go` + `purchase_settlement_command_test.go`——Task 2
- Modify: `internal/modules/commercial/purchase_command.go`（`PurchaseStatePaidAwaitingActivation` 常量，D3 落点）
- Modify: `internal/modules/commercial/subscription_command.go`（`GrantIncludedCreditsPayload.WalletName` 可选字段 + `PurchaseWalletName` + `purchase_period` meta 常量）
- Modify: `internal/modules/commercial/commercialplatform/fake.go`（settle 分支 + grant walletName/meta 区分；`ActivatePurchase`/`SetPurchaseInvoiceFees` 钩子已存在）
- Modify: `internal/modules/commercial/commercialplatform/lago.go`（SubmitCommand 分发 settle）
- Create: `internal/modules/commercial/commercialplatform/lago_settlement.go` + `lago_settlement_test.go`——Task 5
- Modify: `internal/modules/commercial/commercialplatform/lago_purchase.go`（`readPurchaseInvoiceFees` 真实实现 + OCR：错误分类/429/单标签 host/S1 Lago 侧校验）——Task 4/8
- Modify: `internal/modules/commercial/commercialplatform/config.go`（`WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN` + 出站校验 dev 旁路开关）
- Modify: `internal/modules/commercial/commercialplatform/contract_test.go`（settle/fees 共享契约腿）——Task 6
- Create: `internal/modules/commercial/service/commercial/purchase_fulfillment.go` + `purchase_fulfillment_test.go`——Task 7
- Modify: `internal/modules/commercial/service/commercial/fulfillment.go`（purchase 路由 + 构造参数 + nil 语义）
- Modify: `internal/modules/commercial/service/commercial/purchase.go`（合成态消费 + OCR：重放 202 姿态）
- Modify: `internal/modules/commercial/service/commercial/order.go`（OCR：回填 `created_at IS NULL` 限定 + sweep 时限）
- Modify: `internal/handler/commercial.go`（OCR：404/409/500 错误分类分支）
- Modify: `internal/handler/payment_callbacks.go`（无签名变化；Task 7 测试消费）
- Modify: `internal/container/container.go`（PurchaseFulfiller Provide + dig 注入）
- Modify: `packages/contracts/src/commercial.ts`（`paid_awaiting_activation` 入集 + currency null 守卫）+ `packages/contracts/test/commercial.test.ts`
- Modify: `apps/web/src/commercial/CheckoutPage.tsx` / `BillingPage.tsx`（三态呈现 + OCR 前端 findings）
- Create: `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go`（lago_integration tag）——Task 9
- Create: `docs/plans/issue-72-flow-evidence-82/`（追加 settle 链证据：`settle-run.txt`、`deliver_stripe_webhook.py`、`lago-four-objects-after-settle.txt`、截图）——Task 10
- Modify: `docs/plans/issue-72-flow-evidence-82/browser_*.mjs` + stub 脚本（OCR evidence findings 修复）——Task 10

---

### Task 1: t11 结算驱动契约探针（先于一切实现；机制实证门）

**Files:**
- Create: `deploy/lago-lab/payment-settle-trigger/{lab.sh,run_lab.py,phases.py,fixtures.py,test_phases.py}`
- Create: `deploy/lago-lab/payment-settle-trigger/evidence/t11-*.json`
- Create: `docs/migrations/lago/t11-payment-settle-trigger/{DECISION.md,README.md}`

**Interfaces:**
- Consumes: `git show ca04d7da7:deploy/lago-lab/payment-trigger/` 的 lab 骨架（lab.sh/run_lab.py/phases.py/fixtures.py——t10 探针代码，含 `_cancel_intent`、gated-3DS fixture、DB observer）；运行中的 `weknora-lago-82flow` 栈（48889）；`~/.zcode/issue72-stripe.env`（source 注入 STRIPE key）。
- Produces: t11 机制实证（五判据 P-A..P-E）+ `docs/migrations/lago/t11-payment-settle-trigger/DECISION.md`——Task 5/9/10 的机制依据；`fixtures.py` 的 `stripe_get(path)`/`stripe_post(path, form)`/`stripe_webhook_secret(db_dsn)/sign_stripe_event(secret, payload, ts)` helper（Task 10 harness 复用同构逻辑）。

- [ ] **Step 1: 恢复 t10 lab 骨架并写 t11 探针的 RED 单测**

```bash
git show ca04d7da7:deploy/lago-lab/payment-trigger/lab.sh > deploy/lago-lab/payment-settle-trigger/lab.sh
# 同法恢复 run_lab.py/phases.py/fixtures.py/test_phases.py，改名 lab 工程 weknora-lago-t11
```

`test_phases.py` 新增（对纯函数先行 TDD）：

```python
def test_stripe_signature_shape(self):
    # Stripe-Signature: t=<ts>,v1=hex(hmac_sha256(secret, f"{ts}.{payload}"))
    sig = sign_stripe_event("whsec_test", b'{"id":"evt_1"}', 1700000000)
    self.assertTrue(sig.startswith("t=1700000000,v1="))
    self.assertEqual(len(sig.split("v1=")[1]), 64)

def test_payment_intent_succeeded_event_body(self):
    pi = {"id": "pi_x", "status": "succeeded", "amount": 1980, "currency": "jpy",
          "metadata": {"lago_invoice_id": "inv_lago"}}
    evt = build_pi_succeeded_event(pi)
    self.assertEqual(evt["type"], "payment_intent.succeeded")
    self.assertEqual(evt["data"]["object"]["id"], "pi_x")
```

- [ ] **Step 2: 运行单测确认先失败再通过**

Run: `cd deploy/lago-lab/payment-settle-trigger && python3 -m unittest test_phases -v`
Expected: 首跑 FAIL（`sign_stripe_event` 未定义）→ 实现 `fixtures.py` 的 `sign_stripe_event`/`build_pi_succeeded_event`/`stripe_list_payment_intents(customer)`/`stripe_update_pi_payment_method(pi, pm)`/`stripe_confirm_pi(pi)` 后 PASS。

- [ ] **Step 3: 实现 `phases.py` 的 `phase_settle_trigger`（五判据）**

按 D2' 算法：(P-A) 复用 t10 gated-3DS 阶段建挂死订阅 → Stripe 侧列出 customer 的 requires_action PI 且带 `metadata.lago_invoice_id`；(P-B) attach `LAB_SETTLE_PM`（env 注入，如 pm_card_visa）+ 设 default；(P-C) `update payment_method` + `confirm`，断言返回 `status=="succeeded"`；(P-D) 从 Lago DB 读 `payment_providers.webhook_secret`，用真实 PI 对象构造 `payment_intent.succeeded` 事件并签名投递 `POST :48889/webhooks/stripe/{org_id}`，断言 HTTP 200 且 `inbound_webhooks` 表落库；(P-E) 轮询（≤120s）断言 `subscription active` + invoice `finalized`/`payment_status succeeded`（API status[]=finalized 可见）+ payments succeeded==1 + entitlements 200；随后重复投递同一事件，断言四对象状态 byte-identical（no-op）。
lab.env.example 只写变量名与占位 `__SET_IN_SHELL__`（凭据纪律）。

- [ ] **Step 4: 实跑探针并留证**

Run: `cd deploy/lago-lab/payment-settle-trigger && source ~/.zcode/issue72-stripe.env && ./lab.sh init && ./lab.sh up && python3 run_lab.py`（复用 82flow 栈或独立 project `weknora-lago-t11`，端口避开 5272/5273）
Expected: 五判据全 PASS，证据落 `evidence/t11-*.json`。**若 P-C 或 P-E FAIL：停止，写 DECISION.md 记录证伪细节，直接执行 Step 6 升级，不得继续 Task 2+ 的 settle 面**（Task 2/3/4 与机制无关，可独立继续）。

- [ ] **Step 5: 写 `docs/migrations/lago/t11-payment-settle-trigger/DECISION.md` + README**

内容：问题（α 双轨道驱动链是否成立）、五判据表、源码链引用（F5/F6/F8/F9/F10 的文件行号）、对本计划 D2' 的背书或证伪、运行方式。

- [ ] **Step 6: Commit（或升级）**

```bash
git add deploy/lago-lab/payment-settle-trigger docs/migrations/lago/t11-payment-settle-trigger
git commit -m "issue-72(#82): (task1) t11 settle-trigger probe — dual-track P-A..P-E verified on pinned v1.53.0"
```
若证伪：commit 证据 + DECISION 后调用 `escalate`（R-4 机制在 pinned 版本不可达，交回 owner），停止本计划 Task 5-10。

---

### Task 2: seam additive——settle_purchase_payment 命令、购买钱包身份与三态常量（provider-neutral）

**Files:**
- Create: `internal/modules/commercial/purchase_settlement_command.go`
- Create: `internal/modules/commercial/purchase_settlement_command_test.go`
- Modify: `internal/modules/commercial/purchase_command.go`（`PurchaseStatePaidAwaitingActivation` 常量）
- Modify: `internal/modules/commercial/subscription_command.go`（`WalletName` 字段 + `PurchaseWalletName` + meta 常量）

**Interfaces:**
- Consumes: `domain.Command{Kind, Key, Actor, Reason, Payload}`（platform.go:53 区域既有）；`CommandReceipt{Key, ExternalID string; RecordedAt time.Time}`；`GrantCreditsCommandKey(externalCustomerID, period string) string`（subscription_command.go:145）；`MonthlyWalletName(tenantID uint64, period string) string`（:153）。
- Produces:
  - `CommandKindSettlePurchasePayment CommandKind = "settle_purchase_payment"`
  - `SettlePurchasePaymentPayload struct { TenantID uint64; ExternalCustomerID, ExternalPurchaseSubscriptionID, PlanCode, ChannelTransaction string; AmountFen int64; Currency string }`，`Validate() error`（全字段非空 + `ExternalCustomerID == ExternalCustomerID(TenantID)` + `ExternalPurchaseSubscriptionID == ExternalPurchaseSubscriptionID(TenantID)` + `AmountFen > 0` + `Currency == CurrencyCNY`）
  - `SettlePurchasePaymentCommandKey(extPurchaseSubscriptionID, channelTransaction string) string` → `"settle:" + extPurchaseSubscriptionID + ":" + channelTransaction`
  - `PurchaseStatePaidAwaitingActivation = "paid_awaiting_activation"`（coordinator-composed 注释，D3）
  - `PurchaseWalletName(tenantID uint64, period string) string` → `ExternalPurchaseSubscriptionID(tenantID) + "-" + period`
  - 常量 `walletMetaPurchasePeriod = "purchase_period"`；`GrantIncludedCreditsPayload` 新增 `WalletName string`（空 = MonthlyWalletName）

- [ ] **Step 1: 写失败测试** `purchase_settlement_command_test.go`：

```go
func TestSettlePurchasePaymentPayloadValidate(t *testing.T) {
    base := SettlePurchasePaymentPayload{TenantID: 7,
        ExternalCustomerID: ExternalCustomerID(7),
        ExternalPurchaseSubscriptionID: ExternalPurchaseSubscriptionID(7),
        PlanCode: "pro-v1", ChannelTransaction: "2026092622001471", AmountFen: 9900, Currency: CurrencyCNY}
    if err := base.Validate(); err != nil { t.Fatalf("valid payload rejected: %v", err) }
    bad := base; bad.TenantID = 8 // identity mismatch
    if err := bad.Validate(); !errors.Is(err, ErrPlatformInvalidResponse) { t.Fatalf("want invalid_response, got %v", err) }
    zero := base; zero.ChannelTransaction = ""
    if err := zero.Validate(); err == nil { t.Fatal("empty channel transaction accepted") }
}
func TestSettlePurchasePaymentCommandKeyStable(t *testing.T) {
    if SettlePurchasePaymentCommandKey("weknora-tenant-7-purchase", "txn1") !=
        "settle:weknora-tenant-7-purchase:txn1" { t.Fatal("key shape drifted") }
}
func TestPurchaseWalletNameDistinctFromMonthly(t *testing.T) {
    if PurchaseWalletName(7, "2026-09") == MonthlyWalletName(7, "2026-09") { t.Fatal("purchase wallet collides with base monthly batch") }
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/ -count=1 -run 'TestSettlePurchase|TestPurchaseWalletName'`
Expected: FAIL（未定义）。

- [ ] **Step 3: 实现三个文件的 additive 声明**（无逻辑体——Validate 按注释规则逐字段校验；GrantCreditsCommandKey 的 payload→wallet 解析在 Task 3 消费）。

- [ ] **Step 4: 跑测试确认通过 + 全包回归**

Run: `go test ./internal/modules/commercial/ -count=1`
Expected: PASS（含既有全量）。

- [ ] **Step 5: Commit** `git commit -m "issue-72(#82): (task2) seam additive settle_purchase_payment + purchase wallet identity + paid_awaiting_activation constant"`

---

### Task 3: FakeAdapter settle 语义与购买钱包发放（确定性 authority）

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/fake.go`（SubmitCommand 新增 `case commercial.CommandKindSettlePurchasePayment`；grant 分支消费 `WalletName`）
- Create: `internal/modules/commercial/commercialplatform/fake_purchase_test.go`（恢复 `git show e0364d196^:.../fake_purchase_test.go` 骨干后按本版签名重写）

**Interfaces:**
- Consumes: Task 2 全部 Produces；fake 既有 `purchaseSubs` 状态、`ActivatePurchase(extSubID)` 钩子（fake.go:265）、`receipts` 幂等表。
- Produces: fake settle 契约（Task 6 共享腿消费）——(a) 未激活订阅的 settle → 订阅置 active + 回执幂等；(b) 已 active 再 settle → 同回执、状态不变；(c) 订阅不存在 → `ErrPlatformInvalidResponse`；(d) grant 带 `WalletName` 时以该名建钱包且 meta 写 `purchase_period`，与 MonthlyWalletName 批次并存不冲突。

- [ ] **Step 1: 写失败测试**（关键断言，恢复旧测试名）：

```go
func TestFakeSettleActivatesOnceThenIdempotent(t *testing.T) { // D2'(a)(b)
    // prime: CreatePurchaseSubscription → incomplete；settle #1 → ReadSnapshot Purchase.State==active
    // settle #2 (same Key) → 同回执 RecordedAt；Commands() 记录恰 2 条；状态仍 active
}
func TestFakeSettleUnknownSubscriptionFailsClosed(t *testing.T) { // (c) ErrPlatformInvalidResponse }
func TestFakeGrantPurchaseWalletNoMonthlyCollision(t *testing.T) { // (d)：同 tenant 同周期先 grant 月度再 grant 购买（WalletName=PurchaseWalletName），两次都成功且 Wallets() 有两个独立钱包
}
```

- [ ] **Step 2:** `go test ./internal/modules/commercial/commercialplatform/ -count=1 -run 'TestFakeSettle|TestFakeGrantPurchase'` → Expected FAIL。
- [ ] **Step 3:** fake.go 实现（settle case：锁内查 `purchaseSubs[extSubID]`，缺 → invalid_response；已有回执 → 返回原回执；否则置 active、记回执；grant case：`name := payload.WalletName; if name == "" { name = MonthlyWalletName(...) }` + meta 键按 name 来源分流 `period`/`purchase_period`）。
- [ ] **Step 4:** 同命令跑 → PASS；再 `go test ./internal/modules/commercial/commercialplatform/ -count=1` 全量 PASS。
- [ ] **Step 5:** Commit `"issue-72(#82): (task3) fake deterministic settle + purchase wallet semantics"`。

---

### Task 4: Lago 适配器 readPurchaseInvoiceFees finalized 阶段真实读取

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/lago_purchase.go:517-522`（占位 nil 替换为真实实现）
- Modify: `internal/modules/commercial/commercialplatform/lago_purchase_test.go`（新增 fees stub 腿）

**Interfaces:**
- Consumes: `a.do(ctx, method, path, body)`（lago.go 既有）；`commercial.InvoiceLineSnapshot{Kind, Name string; AmountFen int64}`（purchase_command.go:126-130）。
- Produces: `readPurchaseInvoiceFees(ctx, tenantID) ([]commercial.InvoiceLineSnapshot, error)` 真实契约——`GET /api/v1/invoices?external_customer_id={id}&status[]=finalized`，恰 1 张时取 `fees[]` 中 `item.type=="subscription"` 的行映射 `Kind:"subscription_fee"`（**注意 port 闭合词，不透传 provider 原词**）、`Name=fee.item.name`、`AmountFen=fee.amount_cents`；0 张 → `nil, nil`（awaiting 语义）；>1 张 → `ErrPlatformInvalidResponse`（数据异常 fail-closed）。同时把 invoice `payment_status` 读入快照（`PurchaseSnapshot` additive `InvoicePaymentStatus string` 字段，本 Task 一并加）供 D6' 判据。

- [ ] **Step 1: 写失败测试**（httptest 伪 Lago：finalized invoice 带 1 条 subscription fee 1980 + payment_status succeeded；0 张；2 张；fee 无 subscription 行四种形态，断言映射/nil/invalid_response/refused 输入）。
- [ ] **Step 2:** `go test ./internal/modules/commercial/commercialplatform/ -count=1 -run 'TestLagoReadPurchaseInvoiceFees'` → FAIL。
- [ ] **Step 3:** 实现（JSON 解析 struct 内嵌于函数；金额 `strconv.ParseInt` 整数分，无浮点——GC-6）。
- [ ] **Step 4:** 同命令 → PASS；全包回归 PASS。
- [ ] **Step 5:** Commit `"issue-72(#82): (task4) finalized-stage purchase invoice fees read (T09 condition 3, proration-aware)"`。

---

### Task 5: Lago 适配器 settle_purchase_payment（α 双轨道驱动，D2'）

**Files:**
- Create: `internal/modules/commercial/commercialplatform/lago_settlement.go`
- Create: `internal/modules/commercial/commercialplatform/lago_settlement_test.go`
- Modify: `internal/modules/commercial/commercialplatform/lago.go:161-171`（SubmitCommand 分发新增 case）
- Modify: `internal/modules/commercial/commercialplatform/config.go`（`EnvStripeSettlePmToken = "WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN"` + `StripeSettlePmToken string` 字段 + load；`EnvOutboundAllowLoopback = "WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK"` dev 旁路，见 Task 8 S1 补全）

**Interfaces:**
- Consumes: Task 2 Produces；`readPurchaseSnapshot(ctx, tenantID)`（lago_purchase.go:458）；`validateOutboundHost`/`providerOutboundClient`（lago_purchase.go:78/176）；`providerOutboundCall`（:585，**需扩展方法参数**——新增 `providerOutboundRequest(ctx, method, path, form, idempotencyKey string)`，原 POST 函数改为薄包装，既有调用零改动）；`providerAttachDefaultPaymentMethod`（:620）；customer 的 provider id（`customerProviderBound` :422 读到的 `provider_customer_id`——settle 需先 GET customer 取 pcid）。
- Produces: `func (a *LagoAdapter) settlePurchasePayment(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error)`——D2' 五步算法；错误分类全走闭合 sentinel（Stripe 429/5xx → `ErrPlatformUnreachable`；4xx → `ErrPlatformInvalidResponse`；`StripeSettlePmToken==""` → `ErrPlatformUnconfigured: no settle payment method configured` fail-closed）；成功回执 `{Key: cmd.Key, ExternalID: payload.ExternalPurchaseSubscriptionID, RecordedAt: now}`；**不等待 active**（观察归 Task 7 fulfiller）。
- 任务级 Produces（供 Task 7/9/10）：settle 幂等语义——同 Key 重放在 adapter 内不自持久化（无本地状态），幂等由「快照已 active → 零出站返回回执」+「Stripe PI 已 succeeded 时 update/confirm 幂等（Stripe Idempotency-Key = cmd.Key）」共同保证。

- [ ] **Step 1: 写失败测试**（httptest 伪 Stripe 状态机 + 伪 Lago 快照）：

```go
func TestLagoSettleAlreadyActiveNoOutbound(t *testing.T) { // Review-Focus 2
    // Lago stub: purchase snapshot state=active → settle 调用 → 断言伪 Stripe 收到 0 次请求
    // 回执 ExternalID==extSubID；err==nil
}
func TestLagoSettleDrivesProviderRails(t *testing.T) {
    // Stripe stub 序列: GET /v1/payment_intents?customer=cus_x → 1 条 requires_action+metadata.lago_invoice_id
    //   POST /v1/payment_methods/pm_settle/attach → {id:"pm_clone"}
    //   POST /v1/customers/cus_x (default) → 200
    //   POST /v1/payment_intents/pi_1 (payment_method=pm_clone) → 200
    //   POST /v1/payment_intents/pi_1/confirm → {status:"succeeded"}
    // Lago stub: state=incomplete → 断言五次调用顺序与 Idempotency-Key==cmd.Key（attach/default 之外）
}
func TestLagoSettleFailsClosedWithoutSettlePm(t *testing.T) { // ErrPlatformUnconfigured
}
func TestLagoSettleNoStuckIntentInvalidResponse(t *testing.T) { // GET 空/无 metadata → invalid_response，不再发起扣款
}
func TestLagoSettleClassifiesStripe429Unreachable(t *testing.T) { // confirm 返回 429 → ErrPlatformUnreachable
}
```

- [ ] **Step 2:** `go test ./internal/modules/commercial/commercialplatform/ -count=1 -run TestLagoSettle` → FAIL。
- [ ] **Step 3: 实现 `lago_settlement.go`**（约 150 行；provider 词汇不越包；每步出站前 `validateOutboundHost`；lago 侧 GET customer 同样经 a.do——a.do 的 BaseURL 校验在 Task 8 一并补，本 Task 先在该函数入口直接调用一次 `validateOutboundHost(a.cfg.BaseURL)`）。
- [ ] **Step 4:** 同命令 → PASS；`go test ./internal/modules/commercial/commercialplatform/ -count=1` 全量 PASS。
- [ ] **Step 5:** Commit `"issue-72(#82): (task5) lago settle_purchase_payment — provider-rail driven, webhook-finalized (D2' per t11)"`。

---

### Task 6: 双适配器共享契约腿（contract_test.go）

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/contract_test.go`（`TestFakeAdapterPurchaseContract`/`TestLagoAdapterPurchaseContract` 旁新增 settle 共享腿）

**Interfaces:**
- Consumes: Task 3/5 的 settle 面；contract_test 既有 harness（`TestLagoAdapterPurchaseContract` :561 的 stub priming 模式）。
- Produces: 共享断言集 `settleContractLeg(t, adapter, prime func())`——(1) 未激活 settle 后快照 active；(2) 同 Key 重放同回执；(3) `cmd.Payload` 类型错 → `ErrPlatformUnsupported`；(4) `cmd.Validate()` 失败拒绝。fake 与 lago 两腿同跑。

- [ ] **Step 1: 写失败测试**（共享腿先在 fake 腿上写，lago 腿用 httptest 伪栈 prime incomplete→active 状态机）。
- [ ] **Step 2:** `go test ./internal/modules/commercial/commercialplatform/ -count=1 -run 'SettleContract'` → FAIL（lago 腿缺 stub 序列）。
- [ ] **Step 3:** 补 lago 腿 prime（复用 Task 5 测试的伪 Stripe/Lago 构造 helper，提升为 `lago_settlement_test.go` 顶部共享 `newSettleStubStack(t)`）。
- [ ] **Step 4:** → PASS；全包 PASS。
- [ ] **Step 5:** Commit `"issue-72(#82): (task6) shared settle contract leg across fake and lago adapters"`。

---

### Task 7: 服务编排——PurchaseFulfiller、purchase 路由、三态合成与后端 OCR 清偿

**Files:**
- Create: `internal/modules/commercial/service/commercial/purchase_fulfillment.go`
- Create: `internal/modules/commercial/service/commercial/purchase_fulfillment_test.go`
- Modify: `internal/modules/commercial/service/commercial/fulfillment.go`（:260 `fulfillEvent` 内 `row.Kind == domain.OrderKindPurchase` 分流 + `NewFulfillmentService` 新增 `purchaser *PurchaseFulfiller` 构造参数，nil = purchase 事件转 `attention` 不阻断 drain）
- Modify: `internal/modules/commercial/service/commercial/purchase.go`（`PurchaseStatus` 合成 `paid_awaiting_activation`；OCR：`orderViewFromRow` 重建渠道失败 202 姿态）
- Modify: `internal/modules/commercial/service/commercial/order.go`（OCR:119 回填加 `AND created_at IS NULL`；`SweepStaleLinklessPending` 加时限参数——仅清扫 `created_at` 早于 `sweepStaleAge`（默认 15 分钟，const）的行）
- Modify: `internal/handler/commercial.go`（OCR:497 补 `case errors.Is(err, repocommercial.ErrQuoteNotFound): 404 "quote not found"`、`ErrPurchasePendingExists: 409 "purchase pending exists"`；OCR:531 残差分支 500 + `log.Printf` 闭合文案）
- Modify: `internal/container/container.go`（Provide `NewPurchaseFulfiller` + dig 注入 FulfillmentService）
- Modify: `internal/modules/commercial/payment/alipay_test.go`（恢复 AC2 pin，`git show e0364d196^:internal/modules/commercial/payment/alipay_test.go` 取回 `TestSyncReturnCannotConfirmPurchase`）

**Interfaces:**
- Consumes: Task 2/3/5/6 Produces；`ConfirmPayment`/`OutboxKindFulfill`（repository/commercial/order.go:37/531）；`FulfillmentRecord`/`fulfillmentRecordPending`（fulfillment.go:31/41）；`domain.OrderKindPurchase`（order.go:19）；`OrderStatePending/OrderStatePaid`（domain 既有）。
- Produces:
  - `type PurchaseFulfiller struct` + `func NewPurchaseFulfiller(db *gorm.DB, platform domain.CommercialPlatform, accounts *BillingAccountService) (*PurchaseFulfiller, error)`（nil platform → error，容器 blocked-env 显式传 nil FulfillmentService purchaser）
  - `func (p *PurchaseFulfiller) Fulfill(ctx context.Context, ev repocommercial.OutboxEvent, rec *FulfillmentRecord) error`——编排：① 重读订单（paid 才继续）；② `SubmitCommand(SettlePurchasePayment)`（幂等键绑 `fact.Transaction`，payload 从订单+quote 派生）；③ 有界轮询 `ReadSnapshot`（`settleObserveBudget = 90s`，tick 3s，t11 P-E 实测内）至 `PurchaseStateActive`；canceled → `attention`；超时 → `pending`（下一轮 drain 重放，settle 已幂等）；④ `InvoiceFees` D6' 复核（恰 1 条 subscription 行 + `0 < fee == invoice.total ≤ 订单.AmountFen` + `InvoicePaymentStatus=="succeeded"`；失败 → `refused`）；⑤ `SubmitCommand(GrantIncludedCredits{WalletName: PurchaseWalletName, meta purchase_period})`；⑥ 写 `FulfillmentRecord{Key: "purchase_activation:" + orderID}` external receipt + `MarkFulfilled`。
  - `PurchaseStatus` 合成：authority 观察为 `PurchaseStateAwaitingPayment` 且本地存在 `State == OrderStatePaid` 的购买订单时，`out.State = PurchaseStatePaidAwaitingActivation`（D3：永不来自 authority 观察）。
- Review-Focus 测试名（全部落 `purchase_fulfillment_test.go`，sqlite 内存库 + fake adapter）：`TestPurchaseFulfillOverPaymentNoSecondBenefit`（第二笔不同 txn 的成功回调：ConfirmPayment 幂等拒绝后不产生第二个 outbox 事件——渠道层既有，此处断言 fulfill 只跑一次、settle 命令恰 1 次）、`TestPurchaseFulfillCrashReplayConverges`（settle 已执行后进程崩溃模拟：直接调 Fulfill 两次，settle 恰 1 次出站、grant 恰 1 次、终态 fulfilled）、`TestPurchaseFulfillCanceledStaysClosed`、`TestSyncReturnCannotConfirmPurchase`（alipay_test.go 恢复）、`TestPaidAwaitingActivationSynthesized`（fake: incomplete + 订单 paid → PurchaseStatus 输出合成态；active 后输出 active）。

- [ ] **Step 1: 写失败测试**（上述 5 测试，RED 断言合成态常量与编排终态）。
- [ ] **Step 2:** `go test ./internal/modules/commercial/service/commercial/ ./internal/modules/commercial/payment/ -count=1 -run 'TestPurchaseFulfill|TestPaidAwaiting|TestSyncReturnCannotConfirm'` → FAIL。
- [ ] **Step 3: 实现**（purchase_fulfillment.go + fulfillment.go 分流 + purchase.go 合成 + container 接线；OCR 四处修改同 commit）。
- [ ] **Step 4:** 同命令 → PASS；`go test ./internal/modules/commercial/... ./internal/handler/ ./internal/router/ -count=1` 全量 PASS（本会话基线口径 7 包）。
- [ ] **Step 5:** `make check-backend-architecture && make verify-module-moves` → 均 OK（0 violations / 16 manifests）。
- [ ] **Step 6:** Commit `"issue-72(#82): (task7) purchase fulfiller orchestration + paid_awaiting_activation + OCR r4 backend findings"`。

---

### Task 8: Lago 适配器安全/错误分类 OCR 清偿 + 契约与前端三态呈现

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/lago_purchase.go`（OCR:250 结构性 `no_default_payment_method`（`StripeAPIKey=="" || StripePmToken==""`）→ `ErrPlatformUnconfigured`；OCR:552 `status == http.StatusTooManyRequests` 并入 unreachable（两处分类点）；OCR:148 `isPlausibleHostname` 要求 ≥2 标签；OCR:337 `configured()` 入口对 `BaseURL` `validateOutboundHost`，dev/test 环回由新 env `WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true` 显式豁免（默认拒绝，豁免仅在 `a.do` 与 provider 出站入口校验处放行 `127.0.0.1`/`localhost` host，日志一行警告））
- Modify: `internal/modules/commercial/commercialplatform/config.go`（旁路 env + 文档注释：仅限本地 stub 验证，生产必须关闭）
- Modify: `packages/contracts/src/commercial.ts`（`PURCHASE_STATES` 增 `'paid_awaiting_activation'`；:199 `v.currency!==null` 守卫；`PurchaseView.state` 类型注释同步闭合集）
- Modify: `packages/contracts/test/commercial.test.ts`（合成态 parse 接受 + currency null 不炸）
- Modify: `apps/web/src/commercial/CheckoutPage.tsx`（:258 区域——`state.order.payment==='pending'` 显示「待付款（权益未开通）」改为按 purchase 状态三态：`awaiting_payment`→待付款（权益未开通）/`paid_awaiting_activation`→已付款，权益处理中/`active`→权益已生效；OCR:164 setState 补 `active && scopeController.isCurrent` 守卫；OCR:153 重试按钮文案改「重试（同一报价服务端幂等；报价失效时自动重新报价）」；OCR:269 `checkoutHref` 提取局部常量单一来源）
- Modify: `apps/web/src/commercial/BillingPage.tsx`（:117 区域——套餐行三态：`awaiting_payment`→待付款（权益未开放）/`paid_awaiting_activation`→已付款待激活/`active`→已生效）
- Modify: `apps/web/src/commercial/order-state.ts`（若三态文案集中于此则改此；否则随两页）

**Interfaces:**
- Consumes: Task 7 的合成态（wire 上 `purchase.state`）；contracts `parsePurchaseView`（commercial.ts:177-190）。
- Produces: 前端三态呈现 + 契约闭合集扩展（`PurchaseState = 'awaiting_payment'|'paid_awaiting_activation'|'active'|'absent'|'canceled'`）——Task 10 浏览器断言消费「待付款（权益未开通）/已付款，权益处理中/权益已生效」三文案。

- [ ] **Step 1: RED**——`pnpm --filter @weknora/contracts test`（`paid_awaiting_activation` parse 目前抛错）+ apps/web 单测（CheckoutPage 三态渲染——按 `apps/web/src/commercial/*.test.tsx` 既有模式新增用例）。Expected: FAIL。
- [ ] **Step 2: GREEN 实现**（Go 侧四点 + 契约 + 两页 + 文案）。
- [ ] **Step 3:** `pnpm test:shared && pnpm typecheck:web && pnpm typecheck:shared && pnpm test:web` → 全 PASS。
- [ ] **Step 4:** `go test ./internal/modules/commercial/commercialplatform/ -count=1` → PASS（含新分类测试：`TestSettleClassifies429`（Task 5 已含）、`TestConfiguredRejectsPrivateBaseURL`、`TestOutboundHostRejectsSingleLabel`、`TestNoDefaultPmTerminalWhenUnconfigured`——RED 先行于各自实现）。
- [ ] **Step 5:** Commit `"issue-72(#82): (task8) OCR r4 adapter hardening + contracts/web three-state presentation"`。

---

### Task 9: 真实栈集成测试（lago_integration build tag）

**Files:**
- Create: `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go`

**Interfaces:**
- Consumes: `//go:build lago_integration` 惯例 + `LAGO_INTEGRATION_*` env 门控（照 `lago_benefits_integration_test.go` 既有 harness：LAGO_INTEGRATION_BASE_URL/API_KEY + 本票新增 `LAGO_INTEGRATION_STRIPE_KEY`/`LAGO_INTEGRATION_STRIPE_SETTLE_PM`/`LAGO_INTEGRATION_WEBHOOK_SECRET_HELPER`（可执行命令或 psql DSN，从 Lago DB 取 webhook_secret 的方式在文件头注释固定为 `LAGO_INTEGRATION_WEBHOOK_SECRET` env 直读——由外层 lab shell 从 DB 取出注入，测试自身不连 Lago DB））。
- Produces: `TestLagoIntegrationSettleActivatesGatedSubscription`——真实栈全链：ensure customer+binding（3DS pm env）→ gated create → 等 requires_action（Stripe 侧真实读）→ settle → 投递真实事件（`deliverWebhookEvent` 测试内 helper，用 env secret 签名 POST `/webhooks/stripe/{org}`；org id 经 `LAGO_INTEGRATION_ORG_ID` env）→ 轮询 `ReadSnapshot` active + `InvoiceFees` D6' 断言。缺任一 env → `t.Skip("lago integration env not configured")`（skip≠pass，证据责任在 Task 10）。

- [ ] **Step 1: 写测试**（单条大测试 + 3 个负控子测试：settle 幂等重放零二次扣款/重复 webhook no-op/canceled 不激活）。
- [ ] **Step 2:** `go test ./internal/modules/commercial/commercialplatform/ -tags lago_integration -count=1 -run TestLagoIntegrationSettle -v`（无 env）→ SKIP（确认门控正确）。
- [ ] **Step 3:** 真实栈实跑（环境变量按 Task 10 拓扑导出；Stripe TEST key source 注入）→ PASS。
- [ ] **Step 4:** Commit `"issue-72(#82): (task9) lago_integration settle→webhook→active full-chain test"`。

---

### Task 10: 真实流程验证、证据、evidence 脚本 OCR 清偿、文档与收尾

**Files:**
- Create: `docs/plans/issue-72-flow-evidence-82/settle-evidence/`（新证据子目录：`deliver_stripe_webhook.py`、`settle-run.txt`、`lago-four-objects-after-settle.txt`、`browser-*.png`、`api-*.json`）
- Modify: `docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs` / `browser_sync_face_82.mjs` / `browser_paid_face_82.mjs`（OCR：三脚本凭据改必需 env `FLOW82_EMAIL(_B/_D)`/`FLOW82_PASSWORD(_B/_D)` 无源码兜底、`waitForTimeout` → `waitForSelector` 目标文案、`fileURLToPath` 截图目录、try/catch 输出 RESULT、共享 `_lib.mjs`）
- Modify: `docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py` / `alipay_sandbox_notify.py`（OCR:`KEY_DIR = os.path.dirname(os.path.abspath(__file__))`）
- Modify: `docs/plans/issue-72-flow-evidence-82/wechat_pay_stub.py`（OCR: "Endpooints"→"Endpoints"；端口 env 化）
- Modify: `docs/plans/issue-72-flow-evidence-82/README.md`（追加 settle 链验证记录）
- Modify: `docs/plans/issue-72-ledger-82.md`（收尾判定）
- Modify: `docs/testing/` 惯例文档（若需登记新 harness，按 `docs/testing/craft/web-acceptance.md` 体例）

**Interfaces:**
- Consumes: 全部前置 Task；「真实流程验证方案」章节的拓扑与判据。
- Produces: 端到端证据包（Issue AC1-AC4 判定输入）+ Ledger 收尾记录。

- [ ] **Step 1: 先修 evidence 脚本 OCR findings**（RED：`node -e "require('./browser_flow_82.mjs')"` 缺 env 时 exit 2——OCR 建议的报错行为；`python3 -m unittest` 无，脚本用 `python3 -c "import alipay_gateway_stub"` 冒烟）。
- [ ] **Step 2: 按「真实流程验证方案」执行全链，产出证据文件与截图。**
- [ ] **Step 3: 四对象证据核验**（`lago-four-objects-after-settle.txt`：Invoice finalized + Payment succeeded + Subscription active + Wallet transactions 含 purchase wallet——AC4 沙箱残余如实标注）。
- [ ] **Step 4: 回归 + 红线自查**（命令与预期见验证方案判据 (k)/(l)）。
- [ ] **Step 5: 更新 README/Ledger，Commit** `git commit -m "issue-72(#82): (task10) real-flow settle evidence + OCR r4 evidence-script hygiene + docs"`。

---

## 验收标准 → Task → 测试 追踪矩阵

| Issue 验收标准 | Task | 测试/证据 |
|---|---|---|
| AC1a 状态依次呈现待付款、已付款待激活、已生效 | T2（常量）T7（合成）T8（契约+前端）T10 | `TestPaidAwaitingActivationSynthesized`；contracts test；`browser_paid_face_82.mjs`（「已付款，权益处理中」）+ 新 active 断言；Checkout/Billing 截图 |
| AC1b 仅 active 开放 Entitlement/履约（spec L124） | T7 | `TestPurchaseFulfillCanceledStaysClosed`；fulfiller ④⑤ 仅在 `PurchaseStateActive` 后执行（代码路径断言） |
| AC2 同步返回页不能确认付款 | T7 | `TestSyncReturnCannotConfirmPurchase`（`VerifySyncReturn` 恒 `ErrAlipaySyncReturn` pin）+ `browser_sync_face_82.mjs` |
| AC3a 重复回调不重复履约 | T7 | `TestPurchaseFulfillOverPaymentNoSecondBenefit`；`callback-02-replay.txt` 重放幂等 |
| AC3b Outbox 重放/响应丢失不重复履约 | T5/T6/T7 | `TestLagoSettleAlreadyActiveNoOutbound`、`TestFakeSettleActivatesOnceThenIdempotent`、`TestPurchaseFulfillCrashReplayConverges`、contract settle 腿；T9 负控子测试 |
| AC4 真实支付宝证据关联 Invoice/Payment/Subscription/Credits | T1/T9/T10 | t11 五判据；T9 集成测试；`lago-four-objects-after-settle.txt` + `settle-run.txt`；沙箱残余披露（README + Ledger） |
| AC4'（spec L125）受支持 Provider、禁本地强制 active | T5/T8 | settle 只触 Stripe API + webhook 内建链（代码红线 grep「无本地 active 直写」）；`TestLagoSettleFailsClosedWithoutSettlePm` |
| GC：OCR r4 有效 findings 清偿 | T7/T8/T10 | 各 Step 1 RED 测试 + 修改点清单（见 File Structure） |
| 行项目复核（R-3 强制条件③/T09 条件③） | T4/T7 | `TestLagoReadPurchaseInvoiceFees*` + D6' 复核路径（`refused` 分支测试） |

## Spec 行为矩阵覆盖对照（#82 分片）

- **#5 External payment activation**（L218）：渠道验签事实 → 受支持 provider 轨道 → incomplete→active 恰一次——T5/T7/T9/T10 全链。
- **#7 Payment anomalies**（L220，部分）：duplicate → T7 over-payment 测试；mismatch → `ConfirmPayment` 既有 + T7 canceled 测试（partial/multiple-success 完整面归 #84）。
- **#20 Fault injection**（L233）：响应丢失/重放/崩溃 → T5 幂等 + T7 crash replay + T9 重放负控。
- **#22 Privacy and secrets**（L235）：凭据 env-only 纪律 → 各 Task 评审点 + T10 红线 grep。

## Out of scope（#82 明确不做）

- 微信渠道接入（#83）；异常付款完整面（partial/multiple-success 处置，#84）；充值 Credits（#85）；套餐批次分解与月度到期（#86）；升级/降级（#92-93）；Stripe 结算款的生产手续费/风控对冲设计（R-4 错误代价，生产前回议项）；支付宝真实沙箱凭据补采（环境阻塞，披露不伪造）。

---

## 真实流程验证方案（真实环境端到端；验证在集成分支 worktree 构建+运行）

**环境拓扑（全部真实运行；端口避开 :5272/:5273，及既往占用 :8080/:8084/:8091/:8092/:5183/:5192——本会话 `lsof` 实测后者当前空闲但为既往验证口径，统一 +1/+2 漂移）**

| 组件 | 地址 | 说明 |
|---|---|---|
| Lago v1.53.0 | 127.0.0.1:48889 / :48890 | **复用正在运行的 `weknora-lago-82flow` 栈**（本会话 docker ps 实测 healthy；seed org `WeKnora Issue82 Flow`、Stripe provider `weknora-stripe` 已注册）。若需重建：`cd deploy/lago && ./lago.sh init && (追加 LAGO_CREATE_ORG=true 等 seed) && ./lago.sh up`，独立 compose project 隔离数据卷（R9 流程规则） |
| WeKnora 后端 | 127.0.0.1:8093 | 本 worktree `DB_DRIVER=sqlite DB_PATH=data/issue82-settle.db`（独立库，绝不复制共享 DB 写共享 authority）；`WEKNORA_COMMERCIAL_PLATFORM_*=lago→http://127.0.0.1:48889`；`WEKNORA_COMMERCIAL_STRIPE_API_KEY/WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN=pm_card_threeDSecure2Required/WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN=pm_card_visa`（key source ~/.zcode/issue72-stripe.env）；`WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true`（本地 stub 豁免，Task 8 新 env）；`SSRF_WHITELIST_EXTRA=127.0.0.1`；`./scripts/dev.sh app` 或 `go run ./cmd/server`（首次编译数分钟） |
| WeKnora 前端 | localhost:5194 | `VITE_DEV_PROXY_TARGET=http://127.0.0.1:8093 pnpm --filter @weknora/web exec vite --port 5194` |
| 支付宝网关 stub | 127.0.0.1:8294 | 复用 `docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py`（本地 RSA 密钥对，OCR 修复后），端口 env 化为 8294 |
| webhook 投递器 | — | `settle-evidence/deliver_stripe_webhook.py`：从 Stripe API 读回真实 PI → 构造 `payment_intent.succeeded` → 用 Lago DB `webhook_secret`（`docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select webhook_secret from payment_providers where code='weknora-stripe'"`）计算 `Stripe-Signature` → `POST :48889/webhooks/stripe/{org_id}`（org_id 从 `select id from organizations` 取）。D8 边界：替身仅此传输腿 |
| Playwright | headless chromium | `npx playwright --version` 实测 1.60.0；`apps/web` devDependencies 含 @playwright/test；驱动脚本为本目录 `.mjs`（chromium.launch 直连，无需 CRAFT_* env） |

**种子数据**：sqlite 全新库（后端首启自迁移）；两租户经真实 API 注册：`settle-a@verify.local`（浏览器主角）`settle-b@verify.local`（API 主角，plan_publish grant）；Plan `weknora-pro-v1`（9900 分 CNY monthly 无 charges，`api-01-draft`/`api-02-publish` 复用既有脚本形状）。凭据全部 `FLOW82_EMAIL*/FLOW82_PASSWORD*` env 注入（无源码字面量）。

**执行序列（浏览器链 + API 链 + 权威核验，全部真实服务）**

1. **发布 Plan → Lago 落库**：`POST /api/v1/commercial/plans/draft` + `/publish`（b 租户 JWT）→ 断言 200 + receipt `publish_plan_version:pro:1`；`curl :48889/api/v1/plans`（Bearer LAGO_ORG_API_KEY）含 `weknora-pro-v1` → `api-01/02-*.json`。
2. **浏览器下单 →「待付款」**：Playwright headless（脚本 `browser_flow_82.mjs` 修复版）：goto `http://localhost:5194/auth`（env 凭据登录）→ `/platform/billing/checkout?plan=weknora-pro-v1` → 选支付宝 → 提交 → 断言「等待付款」+「待付款（权益未开通）」+ 报价明细 ¥99.00 + 订单号 → 截图 `settle-evidence/01-checkout-awaiting-payment.png`；`/platform/billing` 断言「待付款（权益未开放）」→ `02-billing-awaiting-payment.png`。
3. **Lago gated 形状核验**：`curl :48889/api/v1/subscriptions?external_id=weknora-tenant-1-purchase&status[]=incomplete` → incomplete + activation_rules；DB 直查 invoice `status=5(open)` → `lago-gated-before-settle.txt`。
4. **支付宝 stub 付款 + 同步返回面（AC2）**：`alipay_sandbox_notify.py` 造同源签名 notify（TRADE_SUCCESS 9900）→ 先访问 checkout 同步回跳 URL 刷新 → 断言仍「待付款（权益未开放）」、全页无「已付款/已生效」→ `03-sync-return-no-confirmation.png`（`browser_sync_face_82.mjs`）。
5. **可信回调 → 订单 paid →「已付款待激活」**：`POST /api/v1/commercial/callbacks/alipay`（stub 签名体）→ 200 `success`；GET `/api/v1/commercial/purchase`（a 租户 JWT）→ `state=paid_awaiting_activation`；浏览器 checkout 轮询页断言「已付款，权益处理中」且无「已生效」→ `04-paid-awaiting-activation.png`。
6. **settle 驱动（α 双轨道轨道二）**：outbox drain（后端 worker 自动或 `POST` 恢复触发——按 fulfiller 接线实测）→ 后端日志含 `settle:weknora-tenant-1-purchase:<txn>` receipt；Stripe TEST dashboard/API 断言 PI `succeeded`（结算卡 pm_card_visa）→ `settle-run.txt`。
7. **webhook finalize + 四对象（AC4/AC1 后半）**：`deliver_stripe_webhook.py` 投递真实事件 → 轮询（≤120s）`GET /api/v1/subscriptions?...&status[]=active` → active；`curl :48889/api/v1/invoices?external_customer_id=weknora-tenant-1&status[]=finalized` → finalized+numbered+`payment_status=succeeded`、fees 1 条 subscription（金额按 D6' 判据与 proration 披露）；payments succeeded==1；wallets/transactions 含 `weknora-tenant-1-purchase-<period>` 钱包与到期批次 → 汇总 `lago-four-objects-after-settle.txt`。
8. **「已生效」+ Credits 到账**：GET purchase → `state=active` + `invoice_fees` 非空；浏览器 billing 断言「已生效」+ 余额含购买批次 → `05-active-credits.png`；entitlements `curl :48889/api/v1/entitlements?external_customer_id=weknora-tenant-1` → 200 含 plan feature。
9. **重复回调 + 重复 webhook + 重放（AC3）**：重发同一 notify（支付宝 retry 语义）→ 200 `success`、outbox 事件计数不变；重投同一 webhook 事件 → 四对象 byte-identical（diff t11 P-E no-op 证据）；`TestPurchaseFulfillCrashReplayConverges` 类真面：kill 后端再启 → drain 无第二笔 grant → `06-replay-idempotent.txt`。

**通过判据（全部满足才记 PASS）**

- (a) 步骤 2/5/8 三态文案断言全过（AC1）；
- (b) 步骤 4 同步面零推进（AC2）；
- (c) 步骤 9 三个重放面均无第二次入账/发放/激活（AC3），DB 计数断言（orders paid==1、outbox fulfill==1、fulfillment_records purchase_activation==1、Lago payments succeeded==1）；
- (d) 步骤 7 四对象齐全且 D6' 金额判据过（AC4；支付宝侧 stub 披露 `ac4-sandbox-credentials-unavailable`）；
- (e) 红线自查：`grep -rE "sk_(test|live)_[A-Za-z0-9]{20,}" docs/plans/issue-72-flow-evidence-82/ internal/ deploy/` 零命中；`grep -rn "force.*active\|status.*=.*'active'" internal/modules/commercial/commercialplatform/lago*.go | grep -v _test` 零本地直写命中；
- (f) 回归：`go test ./internal/modules/commercial/... ./internal/handler/ ./internal/router/ -count=1` 全 ok；`make check-backend-architecture && make verify-module-moves` OK；
- (g) 前端：`pnpm test:web && pnpm typecheck:web` PASS；
- (h) T9 集成测试在真实栈 env 下 PASS（非 skip）。

**失败处置**：任一判据不过 → 证据照存（失败也是证据）→ Ledger 记录 → 按 t11/Task 归因；机制级失败（settle 后 Lago 不 finalize）→ `escalate`，不得以本地直写或伪造 webhook 兜底。

## 执行注意事项（并行环境）

- 本 worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees-issue72/issue-82`（分支 `codex/issue-72-lago-82`，已合并集成分支 f50c705074）；只在本 worktree 提交，提交信息前缀 `issue-72(#82): (taskN)`。
- 共享资源纪律：`weknora-lago-82flow` 栈如被其他会话占用/拆除，重建用独立 project 名 `weknora-lago-82r4`（端口不变或整体 +10）；绝不向共享 Lago 写入来自复制 WeKnora DB 的数据（R9）。
- `internal/modules/commercial/commercialplatform/` 全包测试耗时 ~72s（实测），Task 5/8 的局部 `-run` 先行、全包回归放 Task 末步。
- 凭据：一切 `sk_test`/`pm_card_*`（后者非凭据但同纪律）仅经 `~/.zcode/issue72-stripe.env` source 或 `LAGO_INTEGRATION_*` env 注入；`pm_card_visa`/`pm_card_threeDSecure2Required` 为 Stripe 公开测试 token，可写入文档（非凭据），`sk_*` 永不。
