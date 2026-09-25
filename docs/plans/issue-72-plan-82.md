# [Lago 10 / Issue #82] 支付宝付款后恰好一次激活套餐 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让一次经渠道验签的支付宝付款，通过受支持的 Lago Payment Provider 轨道（R-1 裁决选项②）驱动 payment-gated 订阅恰好一次激活：渠道回调只入账一次、Outbox 重放与响应丢失不重复履约、产品状态按「待付款→已付款待激活→已生效」稳定呈现，激活后套餐 Entitlement 与首期套餐 Credits 到账，并留存关联 Invoice/Payment/Subscription/Credits 的真实栈证据。

**Architecture:** 沿用 #81 已冻结的 additive 模式。seam 新增一个命令 `settle_purchase_payment`（把「可信渠道付款事实」翻译成 authority 侧 provider 轨道的收款结算触发：读 authority 付款行定位 gating invoice → 取消挂死的 provider 意图 → 切换结算凭据为已配置的结算 pm → `POST /invoices/{id}/retry_payment`（pinned 源码 `Invoices::Payments::RetryService` 链）；幂等身份绑定渠道支付单号）并补全 `readPurchaseInvoiceFees` 的 finalized 阶段真实读取（T09 强制条件③的 #82 消费义务）。服务层把 `OrderKindPurchase` 订单的 fulfill 事件从旧 OpenMeter gateway 路由到新的 `PurchaseFulfiller`：settle 触发 → 观察 purchase 快照到 active → finalized 行项目完整复核 → 本地订阅投影切换到已购套餐 → 以购买身份发放首期套餐 Credits（幂等 wallet 名 + 独立 meta 键，绝不与 Base 月度批次撞身份）。产品态新增协调层合成的 `paid_awaiting_activation`（订单已付 + authority 未观察到 active），spec L169 产品词表闭合集随之扩一项。激活的"恰好一次"由三层合力保证：渠道层 ConfirmPayment 幂等单事务（既有）→ outbox `fulfill:<order>` 唯一事件（既有）→ authority 侧唯一 pending payment 部分唯一索引 + ResolveService no-op（T02 实测）。

**Tech Stack:** Go（gin + gorm，SQLite/PostgreSQL 双方言）、pinned Lago Community v1.53.0（`getlago/lago-api@591ae900`，digest 锁定 compose）、Stripe TEST 作为受支持 provider 轨道（密钥仅环境变量）、React+Vite 前端（apps/web）、TypeScript 共享契约（packages/contracts、packages/api-client）、Playwright（真实流程验证）。

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`（"Quotes, payments, and fulfillment" L122-127、"Public product states" L169-171、"Behavior and contract matrix" #5 L218 / #20 L233）；决策依据 `docs/migrations/lago/t02-payment-activation/DECISION.md`（§3 provider 轨道实测、§4 渠道阻断、§5 裁决记录）、`docs/migrations/lago/t09-quote-invoice/DECISION.md`（§3 强制条件③ InvoiceFees 接口）、`docs/plans/issue-72-user-rulings.md` R-1/R-3、ADR-0012、ADR-0014。

> **执行状态注记与计划处置声明（2026-09-25，第 2 轮审查后定稿；本节为计划正文的最高优先级约束）**
>
> **1. D2 结算触发链已在 pinned v1.53.0 上被证伪，Task 5-10 冻结为设计存档，禁止执行。** Task 1 probe 在 pinned 栈实跑：P1 FAIL（F5 被证伪——挂死窗口内 `GET /payments?external_customer_id=` 返回 `payments: []`，付款行仅落 authority DB 不可见）且 P2d FAIL（F3 收窄——gating invoice 隐藏窗口内 `POST /invoices/{id}/retry_payment` 返回 404 `invoice_not_found`，另两次 pm 重导轮询 31 次耗尽不产生导入）。逐链实录见 `docs/migrations/lago/t10-payment-trigger/DECISION.md`（verdict："The D2 settlement-trigger chain is FALSIFIED on the pinned stack"）与 `t10-payment-probe-p1.json`/`t10-trigger-p2.json`。按 Task 1 升级条款：**Task 5 及其后全部实现面改动停止，不得实施替代猜测路径**。Task 5-10 正文与「真实流程验证方案」第 5-8 步按原 D2 机制写成，现全部标记【冻结——设计存档】，仅作重议输入，任何执行者不得按其施工；各节标题处有同款标记。
>
> **2. 本票在当前形态下的结论（诚实账面）。** 已落地且不受证伪影响：Task 2（seam additive：settle 命令与钱包身份，66ab920cc）、Task 3（fake 确定性语义，1cf072701）、Task 4（InvoiceFees finalized 读，782e316da）、Task 1 probe 证据（t10）。**Issue #82 四条正式 AC（gh issue view 82 实取：三态呈现/同步返回页不确认/不重复履约/真实沙箱证据四对象关联）在重议裁决前均无法经本计划达成**——激活后半链（AC1 后半、AC3 的 authority 侧防线、AC4 的四对象关联、GC-3 的受支持触发器）全部依赖被证伪的 D2 机制。追踪矩阵已按 AC1-AC4 重编并逐条标注受阻状态（见「验收标准 → Task → 测试 追踪矩阵」节）。
>
> **3. 批准前置条件。** 本计划须经 spec/ADR owner 完成 T02 §5 选项重议（t10 DECISION.md 已列为升级输入）并产出修订版计划（替换 D2 与 Task 5-10）后，方可批准执行被冻结部分。修订版计划产出前，本文件 Task 1-4 的已完成内容与 t10 证据即为 #82 的当前可交付全集。
>
> **4. 不受冻结影响的独立面（仅此一项可继续）。** Task 7 内的 AC2 渠道面 pin 测试 `TestSyncReturnCannotConfirmPurchase`（`internal/modules/commercial/payment/alipay_test.go`）只 pin 既有渠道语义（`VerifySyncReturn` 恒返 `ErrAlipaySyncReturn`，alipay.go:340-342），不依赖任何 authority 机制，保持可执行；其测试骨架已于本轮按真实 helper 名修订（见 Task 7 Step 1）。

---

## Global Constraints（批准需求原文 + 安全约束，每个 Task 隐含遵守）

1. spec L122：「Initial subscriptions are pay-in-advance, have no trial, and use a payment activation rule. They remain incomplete until the gating payment succeeds.」
2. spec L123：「WeKnora owns WeChat Pay and Alipay request creation, callback verification, query, close, and refund behavior. A verified channel result produces an immutable Payment Fact.」
3. spec L124-125：「A supported Lago external-payment integration records the Payment. Only a Lago Subscription observed as active enables Entitlement and fulfillment.」「If Lago manual Payment does not release the activation rule in the pinned Community version, the implementation must use a Lago-supported external Payment Provider adapter. It may not force the Subscription active locally.」——实现中**禁止**任何对 subscription 状态的本地直写（无该 API、不得伪造 webhook、不得写 authority 数据库）。
4. spec L126：「Paid Wallet top-up is not spendable until Lago confirms the Wallet transaction. Response loss is recovered with the original invoice, channel, and idempotency identities.」——settle 与 grant 的幂等身份绑定渠道支付单号 / 确定性身份。
5. spec L169：「WeKnora maps provider state into stable product states: awaiting payment, paid awaiting activation, active, …」——产品词表闭合，Lago 原始状态枚举（incomplete 等）永不过线（spec L170）。
6. spec L105：「Monetary values use integer minor units and never pass through binary floating point.」；首期 CNY only。
7. R-1 裁决（2026-09-23）：付款激活通道 = Lago 原生 Payment Provider 集成（选项②）；Stripe TEST 已实证。凭据纪律：Stripe/支付宝凭据只从环境变量读取（`~/.zcode/issue72-stripe.env` 仅 source 注入），任何产出（代码、测试、文档、证据、日志）不得出现 `sk_test`/`sk_te`+`st_` 类可用凭据字面量。
8. 安全约束（实现验收条件）：服务端发起外部请求仅允许 http/https 且请求前校验 host（复用 `validateOutboundHost`，lago_purchase.go:69）；数据库查询一律参数绑定；源码/示例/测试无凭据字面量。
9. 冻结 seam 纪律（ADR-0014）：`platform.go` 三方法签名不变，仅 additive（新 CommandKind/新可选字段）；provider 词汇（payments/invoices/retry 等 URL 与字段）只存在于 `commercialplatform/` 包内，绝不进入 port 类型、服务层或前端契约。
10. 本偏差纪律（R-3 强制条件④）：一切偏差仅限本计划列明条目，不得外溢。

## 实证契约基线（本计划编写会话在 pinned v1.53.0 证据 + 源码上核实；执行者不得凭 OpenAPI 假设覆盖）

- **F1（运行时，t02-activation.json observed）**：gated 订阅在 provider 扣款成功后 → `subscription_status: active`、恰好 1 个 `succeeded` provider payment（`provider_payment_id: pi_…`）、invoice `finalized`+编号+`payment_status: succeeded`、entitlements 200、`totals_match`。激活由 authority 自身 worker 异步驱动，无需任何 WeKnora 侧 API 调用去"登记"付款。
- **F2（运行时，t02-duplicates.json responses.retry_payment）**：对已支付 invoice `POST /api/v1/invoices/{lago_id}/retry_payment` → HTTP 405 `{"code":"invalid_status","error":"Method Not Allowed"}`——**POST 动词路由存在**（错误体是 controller 渲染的 Lago 错误，非 Rails 路由 404），只是"已成功不可重试"。
- **F3（源码核实，pinned `app/controllers/api/v1/invoices_controller.rb`；⚠️ 仓库内不可复核——本地无 lago-api clone，全仓 `find`/`grep 591ae900` 于 deploy/ 仅命中 wallet-semantics 的注释引用；本会话经 raw.githubusercontent 实取，运行时兜底＝Task 1 probe P2）**：`retry_payment` action 存在，调 `Invoices::Payments::RetryService.new(invoice:, payment_method_params: retry_payment_params[:payment_method]).call`，成功 `head(:ok)`；`retry_payment_params` 允许 `payment_method: {payment_method_type, payment_method_id}`。**【t10 运行时收窄】gating invoice 隐藏窗口内实调返回 404 `invoice_not_found`（t10-trigger-p2.json）——源码核实成立但运行时前置（发票可见性）不成立，本条不可作为激活触发依据。**
- **F4（源码核实，pinned `app/services/invoices/payments/retry_service.rb`；⚠️ 同 F3 仓库内不可复核，运行时兜底＝Task 1 probe P2/P3）**：retry 对 draft/voided/已 succeeded 支付的 invoice 拒绝；`ready_for_payment_processing?` 为假（正在处理中）时拒绝（`payment_processor_is_currently_handling_payment`）；否则 enqueue `Invoices::Payments::CreateService.call_async(invoice:, payment_method_params:)`——与 gated 创建时自动扣款是**同一服务**，用当前解析到的 provider 默认支付方式发起新收款尝试。
- **F5（源码核实，pinned `app/serializers/v1/payment_serializer.rb`）**：Payment API 对象渲染 `lago_id`、`invoice_ids`（payable 为 invoice 时是含该 invoice lago_id 的数组）、`lago_payable_id`/`payable_type`、`status`、`payment_status`、`provider_payment_id`、`payment_provider_code`。`GET /api/v1/payments?external_customer_id=` **不做 visible 过滤**（lab `phases.py:287-306,638-642` 实测列出过非 succeeded 付款）——产品代码可从挂死的付款行拿到不可见 gating invoice 的 lago_id（open invoice 本身对全部 API 不可见，t09 F3-F5）。**【t10 运行时证伪】P1 实测：挂死窗口内该端点对该客户返回 `payments: []`，付款行仅落 authority DB（failed/canceled）不可见（t10-payment-probe-p1.json）——本条整段作废，不得据其设计读取路径。**
- **F6（源码核实，pinned `app/serializers/v1/invoice_serializer.rb` + `fee_serializer.rb`；⚠️ 同 F3 仓库内不可复核，运行时兜底＝Task 1 probe P2 的留档响应 + Task 9 集成测试的真实读断言）**：invoice 对象渲染 `lago_id/number/invoice_type/status/payment_status/total_amount_cents` 与 `fees` 集合；每个 fee 渲染 `item{type,code,name,…}`、`amount_cents`、`amount_currency`、`units`、`payment_status`——**v1.53 fee 顶层没有 `fee_type` 字段，费目类型在 `item.type`**（订阅费 = `"subscription"`）。
- **F7（运行时，#81 flow evidence `README.md` 环境表第 15 行）**：`pm_card_threeDSecure2Required` 作 authority 结算凭据时收款停 `requires_action`、Lago payment=processing、gating invoice 保持 open、订阅保持 incomplete——**稳定待付款窗口**；`timeout_hours:0` 下订阅不因首扣失败自取消（t02-decline：`canceled(payment_failed)` 仅在非零超时的整点 job 后出现）。
- **F8（运行时，lab customer A/B 对照 + `phase_retries`）**：3DS 卡的挂死收款最终走向终态（invoice closed/failed），期间 `retry_payment` 因 `ready_for_payment_processing?` 可能被拒——**"先取消挂死的 provider 意图再 retry"是本计划的 probe 验证点 P2**（见 Task 1）。
- **F9（源码核实，pinned payments_controller）**：manual `POST /api/v1/payments` 仅 Community+Premium（运行时 403，t02-manual.json）——WeKnora 绝不调用它；Payment 录入的唯一受支持通道是 provider 集成自身创建的 Payment（F1）。
- **F10（运行时，#81 flow evidence 断言 5-8 + `purchase.go:159-241`）**：#81 的 Purchase POST 在订单创建时即完成 gated create（匹配校验的数据源是权威订阅面，R-3 强制条件）；**本计划不得把这个创建推迟到付款后**（那会消灭付款前匹配防线，违反 R-3）。
- **F11（本会话实读，`lago.go:754-874` + `fake.go:589-643`）**：`grant_included_credits` 的钱包身份两层匹配 `byName`（`MonthlyWalletName`）+`byMeta`（`meta[tenant]==ext && meta[period]==period`），且 `createWallet` 无条件写 period meta——**同租户同周期的"套餐 Credits"发放若不区分钱包名与 meta 键，必与 Base 月度批次撞身份触发 `grant content conflict`**。购买发放必须用独立钱包名 + 独立 meta 键（设计 D4）。
- **F12（本会话实跑）**：`go build ./...` exit 0（仅 ld 重复库告警）；`go test ./internal/modules/commercial/service/commercial/ -count=1 -run TestPurchase` → ok 2.349s；`go test ./internal/modules/commercial/commercialplatform/ -count=1 -run 'TestFakePurchase|TestFakeAdapterPurchase'` → ok 0.470s；`npx playwright --version` → **1.63.0**（比任务简报记录的 1.60.0 新，以实测为准）；`apps/web/playwright.craft.config.ts` 存在。
- **F13（本会话实测）**：宿主 :8080 已被其他会话占用（`lsof` 实测 node LISTEN）；本计划验证端口全程避开 :5272/:5273（任务约束）、:8080/:8084/:8091/:5183（其他会话与 #81 证据）与 :48889-48892（operator 栈与 #74 lab）。
- **F14（源码核实，pinned payments_controller.create_params + t02-manual）**：manual payment 合同仅 `invoice_id/amount_cents/reference/paid_at`——#82 不使用该通道（F9），此行仅作排除性证据。

## 设计决策记录（本计划定稿，执行者按此实现）

- **D1 并存口径（R-1 的 #82 细化）**：支付宝渠道是**收款事实**通道（spec L123，WeKnora 拥有请求/验签/查询/关单）；Lago Payment 是**激活结算事实**，由受支持 provider 集成（Stripe）在 authority 内部创建（F1/F9）。两者由 WeKnora 协调层关联：可信渠道付款事实（ConfirmPayment 落账 + outbox 事件）→ `settle_purchase_payment` 命令触发 provider 轨道结算 → 观察 purchase 快照 active 才开放履约。**订单创建时的 gated create 不动**（F10）。
- **D2 激活触发机制【已被 t10 证伪——保留原文仅作重议输入，禁止据此实现】**：订单时结算是凭据是**非结算 pm**（默认 `WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN`，#81 已接线，验证环境用 3DS 必验卡造成 F7 稳定待付款窗口）；付款确认后 `settle_purchase_payment` 用**结算 pm**（新配置 `WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN`，如 `pm_card_visa` 类即结算测试卡）走 provider API attach+默认 → 等 Lago 导入 → 对 gating invoice `POST retry_payment`（F3/F4）。该机制的三个未决链接由 Task 1 probe 在 pinned 栈实证——**实测结果：P1 FAIL（payment 行不可见，D2 的发票定位步骤无输入）+ P2c 不触发（pm 重导不发生）+ P2d FAIL（retry 404 invoice_not_found），全链证伪（t10 DECISION.md）**；按升级条款回到 T02 §5 选项重议，不实施替代猜测路径。
- **D3 产品三态**：`paid_awaiting_activation` 是**协调层合成态**（本地订单 paid + authority 快照非 active），不是 authority 真值。常量落点唯一：**seam 包 `internal/modules/commercial/purchase_command.go`**（`PurchaseState*` 权威集同区，purchase_command.go:106-117 之后），注释注明"coordinator-composed（订单 paid + authority 非 active），永不来自 authority 观察"；`service/commercial/purchase.go` 仅消费该常量（经既有 `domain.` 别名）。seam 的 authority 观察集不变——`readPurchaseSnapshot` 永不产出该值。authority canceled + 本地 paid = `canceled` 呈现 + fulfillment attention（异常付款面留给 #84，本票只保证不扩大权益）。
- **D4 套餐首期 Credits 发放身份**：命令键 `GrantCreditsCommandKey(ExternalPurchaseSubscriptionID(t), period)`（独立键族）；钱包名 `PurchaseWalletName(t) = ExternalPurchaseSubscriptionID(t)+"-"+period`；钱包 meta 写 `tenant` + 新键 `purchase_period`（**不写** `period` 键，F11 防撞 Base 批次；benefits 快照把它计入余额、不计入月度批次——批次分解是 #86 面）。`GrantIncludedCreditsPayload` additive 新增可选 `WalletName string`（空 = 现行 `MonthlyWalletName`，全部既有调用零改动）。
- **D5 旧 gateway 路由**：`FulfillmentService.fulfillEvent` 对 `row.Kind == OrderKindPurchase` 分流到注入的 `*PurchaseFulfiller`（构造参数新增，container 与测试显式传 nil/实例）；top-up/upgrade 等既有路径一行不动。
- **D6 行项目复核判据**（T09 强制条件③的落地）：finalized 后 `PurchaseSnapshot.InvoiceFees` 必须恰有 1 条 `Kind=="subscription"`（F6 的 `item.type`）且 `AmountFen==Purchase.AmountFen`，同时 invoice `total_amount_cents==AmountFen`；否则 fulfillment 记 `refused`（权益不发放，事实保留，#84 面）。open 阶段恒为空（不可见行永不伪造，#81 既有语义）。
- **D7 恰好一次的三层台账**：渠道层 `ConfirmPayment` 幂等单事务（`repository/commercial/order.go:347-447` 既有）；outbox `fulfill:<order>` 唯一键（既有）；settle/grant 命令的确定性幂等身份 + authority 唯一 pending payment 索引（T02 实测）。FulfillmentRecord（`fulfillment.go:41-58`）扩展一行 `purchase_activation` 记录承载 setle→grant 的外部回执。

## Review Focus（spec 隐含但无任务测试会咬人的输入类；每条注明归属测试）

1. **重复回调中第二笔不同交易号的成功付款**（用户连刷两次付款）：第一笔赢得 fulfill 权，第二笔只入 over_payment 审计事件、绝不二次履约——归属 Task 7 `TestPurchaseFulfillOverPaymentNoSecondBenefit`。
2. **settle 命令在 authority 已 active 后的迟到重放**（响应丢失 + outbox 重放叠加）：必须零副作用返回同一回执，不得再 attach pm / 再 retry——归属 Task 5 `TestLagoSettleAlreadyActiveNoOutbound`。
3. **同步返回页与一切未验签入口**：`VerifySyncReturn` 恒返 `ErrAlipaySyncReturn`（alipay.go:340-342，注释 :336-339），任何伪造"已付"查询串不得推进状态——归属 Task 7 `TestSyncReturnCannotConfirmPurchase`（该 pin 不依赖被冻结的 Task 5-7 编排面，可执行，见头部注记第 4 条）。
4. **settle 执行到一半进程崩溃**（pm 已切换、retry 未发）：下一轮 Recover 重放必须收敛到同一终态且不产生第二张 payment 行——归属 Task 1 probe P3 + Task 7 `TestPurchaseFulfillCrashReplayConverges`。
5. **authority 已取消（canceled）却收到渠道成功付款**（超时边界竞态）：订单已付但权益永不开放，状态诚实呈现 canceled、记录 attention，不伪造生效——归属 Task 7 `TestPurchaseFulfillCanceledStaysClosed`。
6. **Base 批次与套餐批次同周期并存**：Base 月度 grant 的重放不得因套餐 wallet 存在而 content-conflict——归属 Task 3 `TestFakeGrantPurchaseWalletNoMonthlyCollision` + Task 6 契约腿。

## File Structure（新建/修改全景）

- Create: `deploy/lago-lab/payment-trigger/`（lab.sh / run_lab.py / phases.py / fixtures.py / test_phases.py / evidence/）——Task 1 探针
- Create: `docs/migrations/lago/t10-payment-trigger/`（DECISION.md + README.md + 证据 JSON）——Task 1/10
- Create: `internal/modules/commercial/purchase_settlement_command.go` + `purchase_settlement_command_test.go`——Task 2
- Modify: `internal/modules/commercial/subscription_command.go`（payload 可选字段 + `PurchaseWalletName` + meta 常量）
- Modify: `internal/modules/commercial/commercialplatform/fake.go`（settle 分支 + grant walletName/meta 区分）
- Modify: `internal/modules/commercial/commercialplatform/lago.go`（SubmitCommand 分发 + grant 钱包名/meta + SubmitCommand 处 settle）
- Create: `internal/modules/commercial/commercialplatform/lago_settlement.go`（settle_purchase_payment 实现）+ `lago_settlement_test.go`
- Modify: `internal/modules/commercial/commercialplatform/lago_purchase.go`（`readPurchaseInvoiceFees` 真实实现）+ `lago_purchase_test.go`（fees stub 腿）
- Modify: `internal/modules/commercial/commercialplatform/config.go`（`WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN`）
- Modify: `internal/modules/commercial/commercialplatform/contract_test.go`（settle/fees 共享契约腿）
- Create: `internal/modules/commercial/service/commercial/purchase_fulfillment.go` + `purchase_fulfillment_test.go`
- Modify: `internal/modules/commercial/service/commercial/fulfillment.go`（purchase 路由 + 构造参数 + nil-gateway 语义，见 Task 7）
- Modify: `internal/modules/commercial/service/commercial/purchase.go`（合成态消费）
- Modify: `internal/modules/commercial/purchase_command.go`（`paid_awaiting_activation` 常量唯一落点，见 D3）
- Modify: `internal/modules/commercial/payment/alipay_test.go`（AC2 渠道面 pin）
- Modify: `internal/container/container.go`（PurchaseFulfiller Provide + dig 自动注入）
- Modify: `packages/contracts/src/commercial.ts` + `packages/api-client`（无签名变化，仅类型）+ `packages/contracts/test/commercial.test.ts`（契约测试唯一收集点，见 Task 8）+ `apps/web/src/commercial/{Billing,Checkout}Page.tsx`
- Create: `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go`（lago_integration tag）
- Create: `docs/plans/issue-72-flow-evidence-82/`（真实流程验证证据）——Task 10

---

### Task 1: 激活触发契约探针（t10 lab，先于一切实现）

**Files:**
- Create: `deploy/lago-lab/payment-trigger/lab.sh`、`run_lab.py`、`phases.py`、`fixtures.py`、`test_phases.py`、`evidence/`（运行产出）
- Create: `docs/migrations/lago/t10-payment-trigger/DECISION.md`、`README.md`

**Interfaces:**
- Consumes: `deploy/lago`（只读复用 compose/镜像锁）、`deploy/lago-lab/payment-activation/{clients.py,fixtures.py}`（sys.path 导入）、Stripe TEST key（env：`STRIPE_SECRET_KEY`，`source ~/.zcode/issue72-stripe.env` 注入）。
- Produces: `docs/migrations/lago/t10-payment-trigger/` 五份 phase JSON（P1-P4 判定）+ DECISION.md（D2 三链接的实测结论）；后续 Task 5 的实现按 P1-P4 结论执行。

- [ ] **Step 1: 建 lab 骨架**。`lab.sh` 复制 `deploy/lago-lab/payment-activation/lab.sh` 的 init/up/down/status 骨架，改 `COMPOSE_PROJECT="weknora-lago-82"`、默认 `LAGO_API_PORT=48895`，并**新增** `LAGO_FRONT_PORT=48896`（原 lab.sh 无该变量——`grep -n LAGO_FRONT_PORT` 实测无命中；按 `LAGO_API_PORT` 同款 env_value 读取模式加入 front 端口透传与 init 的 URL/端口一致性校验），全部避开 48889-48892。`phases.py` 顶部 `sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "payment-activation"))` 后 `import clients, fixtures`。

- [ ] **Step 2: 写 probe phases**（每个 phase 断言一个链接，payload 构造函数全部进 `fixtures.py` 并配 `test_phases.py` 纯函数单测）：
  - `phase_gated_3ds`：注册 provider + 客户 A（attach `pm_card_threeDSecure2Required`）→ gated create（`activation_rules:[{type:"payment",timeout_hours:0}]`）→ 断言 incomplete + payment_methods 有导入的 pm（F7 复证）。
  - `phase_payment_probe`（P1）：`GET /api/v1/payments?external_customer_id=<A>` → 断言存在非 succeeded 付款行且**对象带非空 `invoice_ids`**（F5）；记录该 invoice lago_id 与付款 `status`（processing/failed 都接受，如实记录）。
  - `phase_trigger`（P2）：(a) 若付款 `status=="processing"`：provider API `POST /v1/payment_intents/{provider_payment_id}/cancel`（凭据仅 Authorization 头）；(b) attach `pm_card_visa` 并置默认（复用 payment-activation 的 attach+default 两步）；(c) 重新 `PUT /api/v1/customers/<ext>`（同 billing_configuration，触发 Lago 侧默认 pm 重导），轮询 `GET /api/v1/customers/<ext>/payment_methods` 直到出现第 2 个 pm（上限 60s）；(d) `POST /api/v1/invoices/{invoice_id}/retry_payment` body `{}` → 期望 200（F3）；(e) 轮询订阅（显式 `status=active`）120s → 断言 active + 恰好 1 个 succeeded payment（F1 全套：invoice finalized+编号+payment_status succeeded+entitlements 200）。任一步拒绝码（405/422 payment_processor_is_currently_handling_payment 等）如实记录为该链接 FAIL 证据。
  - `phase_retry_dup`（P3）：重复 (d) 两次 → 断言 succeeded payment 计数不变（部分唯一索引防线，T02 §3）。
  - `phase_paid_retry`（P4）：对已支付 invoice 再 retry → 断言 405 invalid_status（F2 复证）。
- [ ] **Step 3: 写单测并跑**。`test_phases.py` 至少覆盖：`fixtures.gated_subscription_payload` 携带 `timeout_hours:0`、`fixtures.retry_body()` 为 `{}`、P2 轮询函数对"已有 2 个 pm"立即返回 True（纯函数）。

Run: `cd deploy/lago-lab/payment-trigger && python3 -m unittest test_phases -v`
Expected: 全部 PASS（未跑真实栈前这也是绿的——单测只测纯函数）。

- [ ] **Step 4: 真实栈运行**（需 Docker 与 Stripe TEST key；无 key 记 blocked-env，不得伪造 pass）：

Run: `cd deploy/lago-lab/payment-trigger && source ~/.zcode/issue72-stripe.env && ./lab.sh init && ./lab.sh up && python3 run_lab.py`
Expected: 五 phase 产出 `evidence/t10-*.json`；P1/P2 必须出现 `status: pass`（P2 的三个链接任一 FAIL → 走「升级路径」）；拷贝证据到 `docs/migrations/lago/t10-payment-trigger/` 并写 DECISION.md（D2 三链接逐条 pass/fail + 请求/响应摘录，凭据脱敏）。

- [ ] **Step 5: 提交**

```bash
git add deploy/lago-lab/payment-trigger docs/migrations/lago/t10-payment-trigger
git commit -m "issue-82(t10): settlement trigger contract probe on pinned v1.53.0"
```

**升级路径（P2 FAIL 时执行，不实施替代设计）**：停止后续 Task 的实现面改动，凭 t10 证据（该链接的具体拒绝码/响应）escalate 给 spec/ADR owner：内容 = 「retry_payment 触发链在 pinned 栈被证伪，唯一受支持触发器缺席，回到 T02 §5 选项重议」。已完成的 probe 证据与 lab 骨架照常提交。

### Task 2: seam additive——settle_purchase_payment 命令与 grant 钱包名（provider-neutral）

**Files:**
- Create: `internal/modules/commercial/purchase_settlement_command.go`、`internal/modules/commercial/purchase_settlement_command_test.go`
- Modify: `internal/modules/commercial/subscription_command.go`（`GrantIncludedCreditsPayload` 增可选字段、`PurchaseWalletName`、`WalletMetaPurchasePeriod` 常量）

**Interfaces:**
- Consumes: `platform.go` 冻结三方法（不变）、`purchase_command.go` 的 `ExternalPurchaseSubscriptionID`/`ExternalCustomerID`/`CurrencyCNY`、`subscription_command.go:145` `GrantCreditsCommandKey`。
- Produces（后续 Task 依赖的精确签名）:

```go
// purchase_settlement_command.go
const CommandKindSettlePurchasePayment CommandKind = "settle_purchase_payment"

type SettlePurchasePaymentPayload struct {
    TenantID                       uint64
    ExternalCustomerID             string // 必须 == ExternalCustomerID(TenantID)
    ExternalPurchaseSubscriptionID string // 必须 == ExternalPurchaseSubscriptionID(TenantID)
    ChannelTransactionID           string // 渠道支付单号，非空；幂等身份锚（spec L126）
}
func (p SettlePurchasePaymentPayload) Validate() error
func SettlePurchasePaymentCommandKey(externalPurchaseSubscriptionID, channelTransactionID string) string
// 形如 "settle_purchase_payment:weknora-tenant-<t>-purchase:<txn>"

// subscription_command.go additive
func PurchaseWalletName(tenantID uint64, period string) string
// = ExternalPurchaseSubscriptionID(tenantID) + "-" + period
const WalletMetaPurchasePeriod = "purchase_period" // 购买钱包专用 meta 键（F11：绝不写 period 键）
// GrantIncludedCreditsPayload 新增： WalletName string // 空 = MonthlyWalletName（既有调用零改动）
```

- [ ] **Step 1: 写失败测试**（`purchase_settlement_command_test.go`）：

```go
package commercial

import "testing"

func TestSettlePurchasePaymentPayloadValidate(t *testing.T) {
	good := SettlePurchasePaymentPayload{
		TenantID: 7, ExternalCustomerID: ExternalCustomerID(7),
		ExternalPurchaseSubscriptionID: ExternalPurchaseSubscriptionID(7),
		ChannelTransactionID:           "txn-1",
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	wrongIdentity := good
	wrongIdentity.ExternalCustomerID = ExternalCustomerID(8)
	if err := wrongIdentity.Validate(); err == nil {
		t.Fatal("mismatched customer identity must be refused")
	}
	noTxn := good
	noTxn.ChannelTransactionID = ""
	if err := noTxn.Validate(); err == nil {
		t.Fatal("empty channel transaction id must be refused (idempotency anchor)")
	}
}

func TestSettlePurchasePaymentCommandKey(t *testing.T) {
	got := SettlePurchasePaymentCommandKey(ExternalPurchaseSubscriptionID(7), "txn-1")
	if got != "settle_purchase_payment:weknora-tenant-7-purchase:txn-1" {
		t.Fatalf("unexpected key %q", got)
	}
}

func TestPurchaseWalletName(t *testing.T) {
	if got, want := PurchaseWalletName(7, "2026-09"), "weknora-tenant-7-purchase-2026-09"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
```

- [ ] **Step 2: 跑失败**。Run: `go test ./internal/modules/commercial/ -count=1 -run 'TestSettlePurchasePayment|TestPurchaseWalletName' -v` → Expected: FAIL（undefined）。
- [ ] **Step 3: 最小实现**两个文件（按 Produces 块逐字实现；`Validate` 语义对照 `CreatePurchaseSubscriptionPayload.Validate`，purchase_command.go:63-83）。`subscription_command.go` 只加字段/常量/函数，不改既有成员注释语义。
- [ ] **Step 4: 跑绿**（同 Step 2 命令）→ PASS。再跑 `go test ./internal/modules/commercial/... -count=1` 全绿（既有 payload 断言不受 additive 字段影响）。
- [ ] **Step 5: 提交** `git add internal/modules/commercial && git commit -m "issue-82(seam): settle_purchase_payment command + purchase wallet identity"`

### Task 3: FakeAdapter 实现 settle 与购买钱包发放（应用行为测试的确定性 authority）

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/fake.go`
- Test: `internal/modules/commercial/commercialplatform/fake_purchase_test.go`（追加）、`fake.go` 既有 grant 用例处追加防撞用例

**Interfaces:**
- Consumes: Task 2 的全部类型；fake 既有 `purchaseSubs`/`wallets` 存储（fake.go:70-122,644-694）与 `ActivatePurchase` 钩子（保留，fake.go:263-271）。
- Produces: fake 对 `CommandKindSettlePurchasePayment` 的语义——purchase 不存在 → `ErrPlatformInvalidResponse`；已 active → 同回执零副作用；incomplete → 置 `Status:"active"` 并回执（fake 以确定性方式模拟 F1 的 provider 结算成功）。grant 分支按 `payload.WalletName` 匹配（空回退 `MonthlyWalletName`），购买钱包写入 `purchase_period` meta 且**不写** period meta。

- [ ] **Step 1: 写失败测试**（`fake_purchase_test.go` 追加）：

```go
func TestFakeSettlePurchasePayment(t *testing.T) {
	f := NewFakeAdapter()
	ctx := context.Background()
	create := commercial.Command{Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(commercial.ExternalPurchaseSubscriptionID(9), "plan-p"),
		Payload: commercial.CreatePurchaseSubscriptionPayload{TenantID: 9,
			ExternalCustomerID: commercial.ExternalCustomerID(9),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(9),
			PlanCode: "plan-p", AmountFen: 9900, Currency: "CNY"}}
	if _, err := f.SubmitCommand(ctx, create); err != nil {
		t.Fatalf("create: %v", err)
	}
	settle := func(txn string) commercial.Command {
		return commercial.Command{Kind: commercial.CommandKindSettlePurchasePayment,
			Key: commercial.SettlePurchasePaymentCommandKey(commercial.ExternalPurchaseSubscriptionID(9), txn),
			Payload: commercial.SettlePurchasePaymentPayload{TenantID: 9,
				ExternalCustomerID: commercial.ExternalCustomerID(9),
				ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(9),
				ChannelTransactionID: txn}}
	}
	r1, err := f.SubmitCommand(ctx, settle("ch-txn-1"))
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	snap, err := f.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 9})
	if err != nil || snap.Purchase.State != commercial.PurchaseStateActive {
		t.Fatalf("settle must activate, got %+v err=%v", snap.Purchase, err)
	}
	// 迟到重放（不同交易号 / 同交易号）：零副作用同回执语义（Review Focus 2）
	if _, err := f.SubmitCommand(ctx, settle("ch-txn-2")); err != nil {
		t.Fatalf("late replay must no-op, got %v", err)
	}
	if got := len(f.PurchaseSubscriptions()); got != 1 {
		t.Fatalf("replay must not create objects, got %d", got)
	}
	// 不存在的 purchase：fail closed
	missing := settle("ch-txn-3")
	missing.Payload = commercial.SettlePurchasePaymentPayload{TenantID: 42,
		ExternalCustomerID: commercial.ExternalCustomerID(42),
		ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(42),
		ChannelTransactionID: "ch-txn-3"}
	if _, err := f.SubmitCommand(ctx, missing); !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("absent purchase must fail closed, got %v", err)
	}
}
```

（`TestFakeGrantPurchaseWalletNoMonthlyCollision`：先以 Base 身份 grant 同 (tenant,period)，再以 `WalletName: PurchaseWalletName(9,"2026-09")` grant 不同金额 → 两笔都必须成功且 `f.Wallets()` 出现两条记录；随后重放 Base grant 不得 content-conflict。）

- [ ] **Step 2: 跑失败**。Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1 -run 'TestFakeSettlePurchasePayment|TestFakeGrantPurchaseWalletNoMonthlyCollision' -v` → FAIL（unsupported kind）。
- [ ] **Step 3: 实现 fake.go**：SubmitCommand switch 加 `case commercial.CommandKindSettlePurchasePayment`（validate → 查 `purchaseSubs` → 缺席 invalid / active 回执复用 / incomplete 置 active 回执）；grant 分支的 `walletName` 改为 `payload.WalletName` 为空时回退 `MonthlyWalletName`，且非空时记录 `PurchasePeriod: payload.Period`（fakeWallet 若无该字段则 additive 增加）而**不**参与 Base byMeta 判断——fake 的匹配本就按 name，保持并在注释注明与 Lago meta 键的对应。
- [ ] **Step 4: 跑绿**（同 Step 2）+ 全包回归 `go test ./internal/modules/commercial/commercialplatform/ -count=1` → ok。
- [ ] **Step 5: 提交** `git commit -m "issue-82(fake): deterministic settle + purchase wallet grant semantics"`

### Task 4: Lago 适配器——readPurchaseInvoiceFees finalized 阶段真实读取（T09 强制条件③）

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/lago_purchase.go`（原引 :481-486 占位——**该 Task 已执行完毕**：占位已被真实实现替换，`readPurchaseInvoiceFees` 现位于 :489 起，404 归入空语义）
- Test: `internal/modules/commercial/commercialplatform/lago_purchase_test.go`（追加 stub 腿）

**Interfaces:**
- Consumes: 既有 `readPurchaseSnapshot` 的 active 阶段调用点（lago_purchase.go:461-470）、`a.do` 客户端（lago.go:535）、F5/F6 契约。
- Produces: `readPurchaseInvoiceFees(ctx, tenantID) ([]commercial.InvoiceLineSnapshot, error)`——`GET /api/v1/invoices?external_customer_id=<ext>`；取 `invoice_type in (null,"subscription")` 且 `status=="finalized"` 的第一张（gating invoice；finalized 后 visible，lab `_gating_invoice` 同款读法）；`fees[]` 逐条映射 `Kind=fee.item.type`、`Name=fee.item.name`、`AmountFen=fee.amount_cents`；无 finalized invoice → `(nil, nil)`（open 阶段恒空，语义不变）；5xx → `ErrPlatformUnreachable`、4xx/畸形 → `ErrPlatformInvalidResponse`（fail-closed 契约，与文件头注释一致）。

- [ ] **Step 1: 写失败测试**（`lago_purchase_test.go` 追加，复用既有 `purchaseStub`/`respond`/`subscriptionsJSON` 模式，lago_purchase_test.go:51、lago_subscription_test.go:26-49）：

```go
func TestLagoReadPurchaseInvoiceFees(t *testing.T) {
	var mu sync.Mutex
	var invoiceReqs []string
	s := newPurchaseStub(t) // 既有构造：内部含 customers/subscriptions 端点
	s.invoiceHandler = func(w http.ResponseWriter, r *http.Request) {
		mu.Lock(); invoiceReqs = append(invoiceReqs, r.URL.String()); mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invoices":[{"lago_id":"inv_1","invoice_type":"subscription",
			"status":"finalized","payment_status":"succeeded","number":"WK-001",
			"total_amount_cents":9900,
			"fees":[{"item":{"type":"subscription","code":"plan-p","name":"Pro"},
				"amount_cents":9900,"amount_currency":"CNY","units":"1"}]}]}`))
	}
	s.mux.HandleFunc("/api/v1/invoices", s.invoiceHandler)
	a := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: s.server.URL, APIKey: "k"})
	lines, err := a.readPurchaseInvoiceFees(context.Background(), 11)
	if err != nil || len(lines) != 1 {
		t.Fatalf("finalized fees must read, got %v lines=%v err=%v", lines, len(lines), err)
	}
	if lines[0].Kind != "subscription" || lines[0].AmountFen != 9900 || lines[0].Name != "Pro" {
		t.Fatalf("fee mapping wrong: %+v", lines[0])
	}
	// open 阶段：无 finalized invoice → 空（不可见行永不伪造）
	s.invoiceHandler = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"invoices":[]}`))
	}
	lines, err = a.readPurchaseInvoiceFees(context.Background(), 11)
	if err != nil || lines != nil {
		t.Fatalf("open stage must answer empty, got %v err=%v", lines, err)
	}
}
```

（stub 若与既有 `purchaseStub` 结构不合，允许就地构造独立 `httptest.Server`——以既有测试文件实际形状为准，断言不动。）
- [ ] **Step 2: 跑失败**。Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1 -run TestLagoReadPurchaseInvoiceFees -v` → FAIL（占位实现恒返回 nil, nil）。
- [ ] **Step 3: 实现**（替换 lago_purchase.go 原 :481-486 占位——【已执行】真实实现落于 :489 起；JSON 解析结构体按 F6 键名；`item.type` 缺失的 fee 记 `Kind:"unknown"` 并继续——复核判据只认 `subscription`）。
- [ ] **Step 4: 跑绿**（同 Step 2）+ 快照回归 `go test ./internal/modules/commercial/commercialplatform/ -count=1 -run TestLagoPurchase -v` → ok（既有 active 快照用例的 InvoiceFees 空断言若依赖 stub 无 invoices 端点会得到 404 → 实现为 4xx→ErrPlatformInvalidResponse 会破坏既有用例；处理：invoices 端点 404 视为"无可见 invoice"返回空——把非 500 的 404 归入空语义，其余 4xx 仍 invalid_response，注释写明 v1.53.0 无该客户时 404 的边界）。
- [ ] **Step 5: 提交** `git commit -m "issue-82(lago): finalized-stage purchase invoice fees read (T09 condition 3)"`

### Task 5: Lago 适配器——settle_purchase_payment（provider 轨道触发，D2）【冻结——设计存档，禁止执行：D2 已被 t10 证伪，待 T02 §5 重议裁决后由修订版计划替换】

**Files:**
- Create: `internal/modules/commercial/commercialplatform/lago_settlement.go`、`internal/modules/commercial/commercialplatform/lago_settlement_test.go`
- Modify: `internal/modules/commercial/commercialplatform/lago.go`（SubmitCommand 分发，lago.go:147-163 处加 case）、`internal/modules/commercial/commercialplatform/config.go`（`EnvStripeSettlePmToken = "WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM" + "_TOKEN"`，Config 增 `StripeSettlePmToken string`，ConfigFromEnv 读取）

**Interfaces:**
- Consumes: Task 2 类型；`validateOutboundHost`/`providerOutboundCall`/`providerAttachDefaultPaymentMethod`（lago_purchase.go:69,167-180 及 :619、:654——第 2 轮审查勘误，原引 :549-617 已漂移）；`readSubscriptionByIdentity`（lago.go:650）；F3-F5 契约；Task 1 的 P1-P4 实测结论（**P1/P2d 已 FAIL，本 Task 整体冻结**）。
- Produces: `func (a *LagoAdapter) settlePurchasePayment(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error)`，算法（每步 fail-closed 语义与文件头契约一致）：
  1. `configured()` + payload 校验 + `purchaseRequestTimeout` 预算；
  2. 身份读订阅（显式 status 集）：`active` → 零出站回执（Review Focus 2）；`canceled/terminated` → `ErrPlatformInvalidResponse`；缺席 → `ErrPlatformInvalidResponse`；
  3. `GET /api/v1/payments?external_customer_id=<ext>`：已有 `status=="succeeded"` 付款 → 回执（结算已发生，观察滞后）；否则取非 succeeded 付款行的 `invoice_ids[0]` 与 `provider_payment_id`（字段缺席 → `ErrPlatformInvalidResponse`，P1 证伪出口）；
  4. 付款 `status=="processing"` 且有 provider_payment_id：provider `POST /v1/payment_intents/{id}/cancel`（S1 host 校验内建于 providerOutboundCall；已终态 4xx 视为成功继续）；
  5. 结算凭据切换：`cfg.StripeSettlePmToken == ""` → `ErrPlatformUnconfigured`（fail closed，不伪造激活）；否则 provider customer 读（billing_configuration.provider_customer_id）→ `providerAttachDefaultPaymentMethod(ctx, pcid, settlePm)` → `PUT /api/v1/customers/<ext>`（原 billing_configuration 重写）触发 Lago 重导 → `waitForPaymentMethodSync` 变体轮询 `payment_methods` 出现第 2 个条目（复用 20s 预算变量 `pmSyncWait`）；
  6. `POST /api/v1/invoices/{invoiceID}/retry_payment` body `{}`：非 2xx → 405/`invalid_status` 映射 `ErrPlatformInvalidResponse`、5xx 映射 `ErrPlatformUnreachable`；
  7. 回执 `{Key: cmd.Key, ExternalID: invoiceID, RecordedAt: now}`——激活观察由 fulfillment 快照轮询承接，命令内**不**长轮询订阅。

- [ ] **Step 1: 写失败测试**（`lago_settlement_test.go`，就地自建 stub——一个 mux 同时挂 Lago 面 /api/v1/* 与 provider 面 /v1/*，`BaseURL` 与 `StripeAPIBase` 都指向它；路径与调用次序记入 slice 供断言）：

```go
func TestLagoSettlePurchasePaymentHappyPath(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	mux := http.NewServeMux()
	reg := func(pattern, body string) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			mu.Lock(); calls = append(calls, r.Method+" "+r.URL.Path); mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
	}
	// 注意：身份读走裸路径+查询串（readSubscriptionByIdentity，lago.go:650-656 发
	// GET /api/v1/subscriptions?external_id=...&status[]=...），不是路径段——
	// Go ServeMux 的路径模式不感知 query，必须注册裸 "/api/v1/subscriptions" 并
	// 在 handler 内按 external_id 分支（404/非2xx 会被适配器映射为
	// ErrPlatformInvalidResponse，happy path 第一步就会死在那里）。
	mux.HandleFunc("/api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock(); calls = append(calls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery); mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"subscriptions":[{"lago_id":"lsub","status":"incomplete","plan_code":"plan-p"}]}`))
	})
	reg("GET /api/v1/payments", `{"payments":[{"lago_id":"pay_1","status":"processing",
		"provider_payment_id":"pi_stuck","invoice_ids":["inv_g1"]}]}`)
	reg("POST /v1/payment_intents/pi_stuck/cancel", `{"id":"pi_stuck","status":"canceled"}`)
	reg("GET /api/v1/customers/weknora-tenant-5", `{"customer":{"billing_configuration":{"provider_customer_id":"cus_1"}}}`)
	reg("POST /v1/payment_methods/pm_settle/attach", `{"id":"pm_cus_1"}`)
	reg("POST /v1/customers/cus_1", `{"id":"cus_1","invoice_settings":{"default_payment_method":"pm_cus_1"}}`)
	reg("PUT /api/v1/customers/weknora-tenant-5", `{"customer":{"external_id":"weknora-tenant-5"}}`)
	reg("GET /api/v1/customers/weknora-tenant-5/payment_methods", `{"payment_methods":[{"id":"pm_old"},{"id":"pm_cus_1"}]}`)
	reg("POST /api/v1/invoices/inv_g1/retry_payment", `{}`)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	a := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: srv.URL, APIKey: "k",
		StripeAPIKey: "provider-key", StripeAPIBase: srv.URL, StripeSettlePmToken: "pm_settle"})
	r, err := a.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindSettlePurchasePayment,
		Key:  commercial.SettlePurchasePaymentCommandKey(commercial.ExternalPurchaseSubscriptionID(5), "ch-txn-9"),
		Payload: commercial.SettlePurchasePaymentPayload{TenantID: 5,
			ExternalCustomerID: commercial.ExternalCustomerID(5),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(5),
			ChannelTransactionID: "ch-txn-9"}})
	if err != nil || r.ExternalID != "inv_g1" {
		t.Fatalf("settle receipt wrong: %+v err=%v", r, err)
	}
	mu.Lock(); defer mu.Unlock()
	joined := strings.Join(calls, "\n")
	for _, want := range []string{
		"GET /api/v1/subscriptions?external_id=weknora-tenant-5-purchase",
		"POST /v1/payment_intents/pi_stuck/cancel",
		"POST /v1/payment_methods/pm_settle/attach",
		"POST /api/v1/invoices/inv_g1/retry_payment",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing call %s in:\n%s", want, joined)
		}
	}
}

func TestLagoSettleAlreadyActiveNoOutbound(t *testing.T) {
	// 订阅直接答 active：除身份读外零出站（Review Focus 2）
	// …stub 仅挂裸 "/api/v1/subscriptions"（查询串形状，答
	// {"subscriptions":[{...,"status":"active"}]}），calls 断言仅 1 次身份读…
}

func TestLagoSettleUnconfiguredFailsClosed(t *testing.T) {
	// StripeSettlePmToken 空 → errors.Is(err, ErrPlatformUnconfigured)，
	// 且 retry/attach 零出站（stub 调用记录不含 /v1/payment_methods 与 retry_payment）
}
```

- [ ] **Step 2: 跑失败**。Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1 -run TestLagoSettle -v` → FAIL（unsupported kind）。
- [ ] **Step 3: 实现** `lago_settlement.go` + config + 分发 case（按 Produces 算法逐步；出站一律走 `providerOutboundCall`/`a.do`，URL 校验不得绕过；错误字符串零 URL/零响应体）。
- [ ] **Step 4: 跑绿**（同 Step 2）+ `go test ./internal/modules/commercial/commercialplatform/ -count=1` 全绿 + `go run ./tools/architectureguard`（Makefile:255 同款）守卫模块边界 → 通过。
- [ ] **Step 5: 提交** `git commit -m "issue-82(lago): settle_purchase_payment via supported provider rails"`

### Task 6: 双适配器共享契约腿（contract_test.go）【冻结——设计存档，禁止执行：消费 Task 5 的 settle 面，随 D2 重议一并处置】

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/contract_test.go`

**Interfaces:** Consumes Task 2/3/5 全部面；`runPurchaseContract` 先例（contract_test.go:494）与 `TestFakeAdapterPurchaseContract`/`TestLagoAdapterPurchaseContract` 入口对（:553,:561）。Produces: fake 与 Lago stub 对 `settle_purchase_payment` + 带 `WalletName` 的 grant 跑**同一组断言**（契约对称性：同一 Key 重放回执一致、缺席/未配置 fail-closed 同哨兵）。

- [ ] **Step 1: 在 contract_test.go 追加 `runSettlementContract` 与两个入口**（与 purchase 对完全同构）：

```go
func runSettlementContract(t *testing.T, name string, p commercial.CommercialPlatform,
	purchaseCount func() int) {
	t.Helper()
	ctx := context.Background()
	const tenant = uint64(83)
	// 0) 缺席 purchase 的 settle：fail closed（同哨兵）
	missing := commercial.Command{Kind: commercial.CommandKindSettlePurchasePayment,
		Key: commercial.SettlePurchasePaymentCommandKey(commercial.ExternalPurchaseSubscriptionID(tenant), "ghost"),
		Payload: commercial.SettlePurchasePaymentPayload{TenantID: tenant,
			ExternalCustomerID:             commercial.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
			ChannelTransactionID:           "ghost"}}
	if _, err := p.SubmitCommand(ctx, missing); !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("%s: absent purchase settle must fail closed, got %v", name, err)
	}
	// 1) 建purchase（复用 runPurchaseContract 的建单命令构造）后 settle → active
	//    （create 命令构造与既有 purchase leg 逐字一致：plan "plan-c"、9900 CNY）
	// 2) settle（"ch-txn-1"）→ 快照 active；回执 ExternalID 非空
	// 3) 同 Key 重放 → 回执一致 && purchaseCount()==1（恰好一次的契约面）
	// （三步的完整代码照 runPurchaseContract :494-551 的逐步断言风格展开；
	//   settle/grant 命令一律经 SettlePurchasePaymentCommandKey/GrantCreditsCommandKey
	//   构造，禁止本地拼 Key。）
	_ = purchaseCount
}

func TestFakeAdapterSettlementContract(t *testing.T) {
	f := NewFakeAdapter()
	runSettlementContract(t, "fake", f, func() int { return len(f.PurchaseSubscriptions()) })
}

// 既有名与新建名边界（防误读）：
//   既有：newPurchaseStub()（lago_purchase_test.go:68，无 t 参）、purchaseAdapterWithPrefix(t, stub.server(t))、
//         stub.countSubscriptionPosts、stub.planAmount —— 与 TestLagoAdapterPurchaseContract
//         （contract_test.go:561-566）逐字同款；
//   新建：runSettlementContract 函数本体，以及 purchaseStub 上为本腿新扩展的
//         payments/payment_methods/invoices(retry) 三个处理分支（stub 新字段，随本 Task 实现）。
func TestLagoAdapterSettlementContract(t *testing.T) {
	stub := newPurchaseStub()
	stub.mu.Lock()
	stub.planAmount = map[string]int64{"weknora-contract-v1": 4200}
	stub.mu.Unlock()
	runSettlementContract(t, "lago", purchaseAdapterWithPrefix(t, stub.server(t)), stub.countSubscriptionPosts)
}
```

grant+WalletName 腿并入 `runGrantContract` 风格的追加断言（contract_test.go:327 同款回调形参）：无 `WalletName` 的 grant 回执 `ExternalID==MonthlyWalletName`；带 `WalletName==PurchaseWalletName(tenant,period)`、不同 CreditsMicro 的 grant 成功；随后重放第一笔不得 content-conflict（F11 防撞的契约面）。
- [ ] **Step 2: 跑**。Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1 -run TestContract -v` → Expected: FAIL（legs 未全实现/lago stub 缺端点）→ 补 stub 后 PASS。
- [ ] **Step 3: 全包回归 + 提交**。Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1` → ok。`git commit -m "issue-82(contract): settle + purchase grant shared contract legs"`

### Task 7: 服务编排——PurchaseFulfiller、purchase 路由与三态产品状态【冻结——设计存档，除 Step 1 末尾的 AC2 渠道面 pin 测试外禁止执行：Fulfill 编排消费被证伪的 settle 面；AC2 pin 只依赖既有渠道语义（头部注记第 4 条）】

**Files:**
- Create: `internal/modules/commercial/service/commercial/purchase_fulfillment.go`、`purchase_fulfillment_test.go`
- Modify: `internal/modules/commercial/service/commercial/fulfillment.go`（构造校验放宽 + fulfillEvent 路由 + 非 purchase 分支 nil-gateway 防护）、`internal/modules/commercial/service/commercial/purchase.go`（三态合成消费）、`internal/modules/commercial/purchase_command.go`（常量唯一落点，D3）、`internal/container/container.go`（`must(container.Provide(commercialsvc.NewPurchaseFulfiller))` 注册——dig 按类型自动注入 NewFulfillmentService 的第三参，container.go:943 的 Provide 行本身无需显式实参）
- Test: `internal/modules/commercial/service/commercial/purchase_fulfillment_test.go`、既有 `fulfillment_test.go` 回归、`internal/modules/commercial/payment/alipay_test.go`（AC2 渠道面 pin）

**Interfaces:**
- Consumes: `OrderStore.ConfirmPayment/MarkFulfilled/GetOrder`（order.go:347,451）、`FulfillmentRecord`/`FulfillmentKey`（fulfillment.go:41,383）、Task 2/5 命令、`PurchaseService` 既有读面（purchase.go:246-282）、`CurrentPurchaseOrder`、本地 `repocommercial.Subscription`（prepareUpgrade 先例，fulfillment.go:350-353,413-427）、`repocommercial.PlanRow`/`PlanVersionService.GetVersion`（ensureNoCharges 先例，purchase.go:286-304）。
- Produces（精确签名）:

```go
// purchase_fulfillment.go
type PurchaseFulfiller struct { /* db, orders, plans, platform */ }
func NewPurchaseFulfiller(db *gorm.DB, plans *PlanVersionService,
	platform domain.CommercialPlatform) (*PurchaseFulfiller, error)

// Fulfill drives ONE paid purchase order to its authority outcome for this
// pass. 返回 (applied bool, err error)。
//   1. settle 命令（Key 绑定渠道交易号）：ErrPlatformUnreachable → (false, nil)
//      释放租约下轮再试；ErrPlatformInvalidResponse → 记 refused（权益不发放）。
//   2. purchase 快照观察：awaiting → (false, nil)；absent/canceled → attention；
//      active → 继续。
//   3. D6 行项目复核：InvoiceFees 恰 1 条 Kind=="subscription" 且 AmountFen==
//      order.AmountFen → 继续；否则 refused。
//   4. 本地订阅投影切换（version 守卫，prepareUpgrade 先例）到已购 plan。
//   5. 首期套餐 Credits：SubmitCommand(grant_included_credits{
//      Key: GrantCreditsCommandKey(ExternalPurchaseSubscriptionID(t), period),
//      WalletName: PurchaseWalletName(t, period), CreditsMicro: def.Monthly,
//      Period: 当前 UTC 月, ExpiresAt: PeriodEnd})；回执 ExternalID 即钱包名。
//   6. FulfillmentRecord(key=domain.FulfillmentKey(orderID,"purchase_activation"))
//      记 applied + 外部回执；返回 applied。
func (f *PurchaseFulfiller) Fulfill(ctx context.Context, orderID string, now time.Time) (bool, error)

// fulfillment.go 修改（构造校验语义，现行 fulfillment.go:148-150 的 gateway==nil
// 即 ErrFulfillmentGatewayMissing 必须放宽为组合判断）：
//   db==nil → ErrFulfillmentDatabaseMissing（不变）；
//   gateway==nil && purchase==nil → ErrFulfillmentGatewayMissing（现行语义保留，
//     全部既有测试/调用零改动）；
//   gateway==nil && purchase!=nil → 放行（购买域 blocked-env 合法：purchase
//     路径不依赖 legacy gateway），fulfillEvent 的非 purchase 分支遇 gateway==nil
//     时 completeEvent(pending) 原样放回、绝不 panic、绝不经 gateway；
//   gateway!=nil && purchase==nil → 放行（现行行为：全部事件走 legacy 路径）。
func NewFulfillmentService(db *gorm.DB, gateway domain.CommercialGateway,
	purchase *PurchaseFulfiller) (*FulfillmentService, error)
// fulfillEvent 开头：row.Kind==OrderKindPurchase && s.purchase!=nil →
//   applied, err := s.purchase.Fulfill(...); applied → MarkFulfilled+event sent

// purchase_command.go（seam 包，PurchaseState* 常量区之后——唯一落点，D3）：
const PurchaseStatePaidAwaitingActivation = "paid_awaiting_activation"
// Coordinator-composed product state (order paid + authority NOT yet active):
// NEVER produced by readPurchaseSnapshot — the authority observation set is
// unchanged. Consumers map it in service/commercial/purchase.go.

// purchase.go 修改（消费，不定义常量）：
// PurchaseStatus / purchaseView：authority awaiting_payment 且当前购买订单
// state==OrderStatePaid → State 置 domain.PurchaseStatePaidAwaitingActivation
```

- [ ] **Step 1: 写失败测试**（`purchase_fulfillment_test.go`；装复用既有真 helper：`newPurchaseTestEnv`（purchase_test.go:18，返回 PurchaseService+FakeAdapter+db）、`purchaseSeedPlan`（:55）、`insertOrderRow`（:230）、sqlite 内存建库与 AutoMigrate 清单照 fulfillment_test.go:105-112,189）：

```go
func TestPurchaseFulfillExactlyOnce(t *testing.T) {
	svc, fake, _, db := newPurchaseTestEnv(t) // purchase_test.go:18 既有装配方言
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	ctx := context.Background()
	tenant := uint64(21)
	// 建购买订单并经真实 ConfirmPayment 置 paid（先建 quote+order，再插入
	// PaymentAttemptRow 并以 PaymentFact 确认——先例：fulfillment_test.go:140/234
	// 与 repository/commercial/order_test.go:97 的 ConfirmPayment(domain.PaymentFact{...})；
	// order_test.go:91 附近是 TestOrderPipelineQuoteOrderRecover 的建单段，不是
	// ConfirmPayment 先例）
	orderID := seedPaidPurchaseOrder(t, db, svc, tenant, "ch-txn-1")
	ful := mustFulfillmentWithPurchase(t, db, svc) // NewFulfillmentService(db, nil, NewPurchaseFulfiller(db, svc.plans, fake))
	if err := ful.Recover(ctx); err != nil {
		t.Fatalf("first recover: %v", err)
	}
	// 断言一：FulfillmentRecord 恰 1 条 applied（key=fulfill:<order>/purchase_activation）、
	//   outbox 事件 sent、订单 fulfilled——直查 db 断言。
	// 断言二：fake 收到的 grant 命令钱包名==PurchaseWalletName(tenant,当期)、
	//   命令 Key==GrantCreditsCommandKey(ExternalPurchaseSubscriptionID(tenant),当期)、
	//   余额恰一次（fake.Wallets() 过滤该名恰 1 条）。
	// 第二轮全量 Recover（outbox 重放）：直查快照逐字段比对零变化（AC3）。
	before := snapshotBenefitLedger(t, db, fake)
	if err := ful.Recover(ctx); err != nil {
		t.Fatalf("second recover: %v", err)
	}
	if after := snapshotBenefitLedger(t, db, fake); !reflect.DeepEqual(before, after) {
		t.Fatalf("replay changed state: before=%+v after=%+v", before, after)
	}
}

func TestPurchaseStatusPaidAwaitingActivation(t *testing.T) {
	// newPurchaseTestEnv + purchaseSeedPlan + purchaseQuote：
	// 1) fake 建 purchase（incomplete）、insertOrderRow(state=pending) →
	//    PurchaseStatus=="awaiting_payment"；
	// 2) 同订单置 paid → =="paid_awaiting_activation"；
	// 3) fake.ActivatePurchase(ExternalPurchaseSubscriptionID(tenant)) → =="active"。
	// （D3 全分支，复用 TestPurchaseStatusAttachesOnlyTheCurrentPurchaseOrder 的
	//   断言手法，purchase_test.go:241。）
}
```

（`seedPaidPurchaseOrder`/`snapshotBenefitLedger`/`mustFulfillmentWithPurchase` 为本测试文件内新写的最小 fixture：建 quote→order（真实 `openOrder` 路径或 `insertOrderRow`）→ 插 PaymentAttemptRow → `OrderStore.ConfirmPayment`；直查三表组快照结构体。除这三个新 fixture 外不发明其他 helper。）

func TestPurchaseFulfillOverPaymentNoSecondBenefit(t *testing.T) {
	// 同订单第二笔不同交易号 ConfirmPayment → over_payment 事件；
	// Recover 后 grant/记录不翻倍（Review Focus 1）
}

func TestPurchaseFulfillCrashReplayConverges(t *testing.T) {
	// 崩溃/响应丢失重放收敛（Review Focus 4，AC3）：
	// 1) seedPaidPurchaseOrder 后把 fake 的 settle 提程置为"对象已变更但响应丢失"
	//    形态（fake 先置 purchase active，再让 Fulfill 首轮在 settle 回执处返回
	//    unreachable——复用 fake 既有 failSubmits 注入点）；
	// 2) 首轮 Recover → 订单仍 paid、无 applied 记录、无 grant（attention/pending）；
	// 3) 解除注入后第二轮 Recover → 收敛到与 TestPurchaseFulfillExactlyOnce
	//    完全相同的终态快照（applied 恰 1 条、grant 恰 1 次、事件 sent）；
	// 4) 第三轮 Recover → 零变化。
}

func TestPurchaseFulfillCanceledStaysClosed(t *testing.T) {
	// fake 把 purchase 置 canceled（模拟 authority 终态）后 Recover：
	// 记录 attention、无 grant、无投影切换；PurchaseStatus 呈 canceled（Review Focus 5）
}

func TestPurchaseFulfillLineMismatchRefused(t *testing.T) {
	// lago 腿（stub）或 fake 注入 fees 含 2 条 subscription 行 → refused、无 grant
}
```

`TestSyncReturnCannotConfirmPurchase` 落在 `internal/modules/commercial/payment/alipay_test.go`（AC2 渠道面 pin；**不依赖被冻结的 Task 5-7 编排面，本测试保持可执行**——头部注记第 4 条）。第 2 轮审查勘误：包内**不存在** `newAlipayProviderForTest`/`signedReturnHeader`/`signedReturnBody`（grep 零命中）；既有构造是 `alipayNotifyFixture(t)`（alipay_test.go:59），且该文件已有几乎同义的 `TestAlipaySyncReturnNeverConfirms`（alipay_test.go:228-235，仅断言 `err==nil` 与 fact 零值，**未 pin 哨兵**）。本测试的增量价值因此只有一条：把失败模式 pin 到 `ErrAlipaySyncReturn` 哨兵本身（防止未来实现改成泛化错误导致调用方无法区分"同步返回不可信"与"渠道故障"）：

```go
// alipay_test.go 追加（与既有 TestAlipaySyncReturnNeverConfirms 并列）：
// pin the SENTINEL, not just failure (incremental over :228-235).
func TestSyncReturnCannotConfirmPurchase(t *testing.T) {
	p, _, _ := alipayNotifyFixture(t) // 包内既有构造，alipay_test.go:59
	fact, err := p.VerifySyncReturn(context.Background(), http.Header{},
		[]byte("out_trade_no=out-1&trade_no=trade-1"))
	if !errors.Is(err, ErrAlipaySyncReturn) || fact != (commercial.PaymentFact{}) {
		t.Fatalf("sync return must fail with the dedicated sentinel: fact=%+v err=%v",
			fact, err)
	}
}
```

（payload 用既有 `TestAlipaySyncReturnNeverConfirms` 同款字面量查询串——同步通道本就不验签，无需签名 fixture；`errors`/`http` import 按包既有测试文件惯例补全。）

- [ ] **Step 2: 跑失败**（RED 分两步）：

Run: `go test ./internal/modules/commercial/service/commercial/ -count=1 -run 'TestPurchaseFulfill|TestPurchaseStatusPaidAwaiting' -v`
Expected: 编译错 `undefined: NewPurchaseFulfiller`（第三参构造/装配未落地；改造构造校验后若先跑旧签名用例，RED 形态是 `ErrFulfillmentGatewayMissing`——两者都是合法 RED，不是假绿）。

Run: `go test ./internal/modules/commercial/payment/ -count=1 -run TestSyncReturnCannotConfirmPurchase -v`
Expected: 编译错 `undefined: TestSyncReturnCannotConfirmPurchase`（该测试与其 helper 未写）。

- [ ] **Step 3: 实现**（`purchase_fulfillment.go` 全量 + fulfillment.go 三参构造/校验放宽/路由/nil-gateway 防护 + purchase_command.go 常量 + purchase.go 合成消费 + alipay_test.go 的 AC2 pin + container.go 注册 `NewPurchaseFulfiller` Provide——dig 自动注入第三参，container.go:943 无需显式实参）。事务与租约语义完全复用既有 `leasePendingEvents/completeEvent`。
- [ ] **Step 4: 跑绿**（同 Step 2 两条命令）→ PASS；回归 `go test ./internal/modules/commercial/service/commercial/... -count=1`、`go test ./internal/modules/commercial/payment/ -count=1` 与 `go test ./internal/handler/... -count=1 -run Commercial` → ok（既有 fulfillment gateway 用例不受校验放宽影响）。
- [ ] **Step 5: 提交** `git commit -m "issue-82(service): purchase fulfillment orchestration + paid_awaiting_activation state"`

### Task 8: 契约与前端三态呈现【冻结——设计存档，禁止执行：三态的激活出口路径在 D2 证伪后无裁决依据，先行放开会造成只有入口没有出口的产品态】

**Files:**
- Modify: `packages/contracts/src/commercial.ts:177-204`（`PurchaseView.state` 联合与 `PURCHASE_STATES` 增 `'paid_awaiting_activation'`）
- Modify: `apps/web/src/commercial/BillingPage.tsx`（套餐行文案分支）、`apps/web/src/commercial/CheckoutPage.tsx`（`purchaseErrorMessage` 对新态不按错误渲染）
- Test: `packages/contracts/test/commercial.test.ts`（追加——**唯一被 `pnpm test:shared` 收集的 commercial 契约测试位置**：root package.json:13 的 glob 是 `packages/contracts/test/*.test.ts …`，不收集 `src/*.test.ts`；src 下现存 analytics/query-history/usage 三个孤儿测试文件是历史遗留，**不要模仿**，否则 RED 步静默假绿）

**Interfaces:** Consumes Task 7 的线面（`GET /purchase` 状态集扩一项）。Produces: 前端把 `paid_awaiting_activation` 渲染为「已付款待激活（权益未开放）」；`awaiting_payment` 仍为「待付款」；不出现任何 Lago 原始枚举（spec L170）。

- [ ] **Step 1: 写失败测试**（追加到既有 `packages/contracts/test/commercial.test.ts` 的 commercial describe/文件尾部，import 行复用该文件既有的 `parsePurchaseView` 引入方式）：

```ts
// packages/contracts/test/commercial.test.ts 追加
it('accepts paid_awaiting_activation and rejects unknown purchase states', () => {
	const v = parsePurchaseView({ state: 'paid_awaiting_activation', amount_fen: '9900', currency: 'CNY' });
	assert.equal(v.state, 'paid_awaiting_activation');
	assert.throws(() => parsePurchaseView({ state: 'incomplete' }), /invalid purchase/);
});
```

（web 页面测试照该包既有 node:test+tsx 形状断言 BillingPage 套餐行文案映射函数对新态输出「已付款待激活」。）
- [ ] **Step 2: 跑失败**。Run: `pnpm test:shared 2>&1 | tail -5` → FAIL（parse 拒绝新态）。
- [ ] **Step 3: 实现**契约与页面分支（BillingPage.tsx:116-117 处文案条件扩为新态；CheckoutPage `purchaseErrorMessage` 仅对 `reason!=""` 生效，新态不带 reason，无需额外分支——补一条断言防回归）。
- [ ] **Step 4: 跑绿**。Run: `pnpm test:shared && pnpm --filter @weknora/web test 2>&1 | tail -3` → 全 PASS；`pnpm typecheck:shared && pnpm typecheck:web` → 零错误。
- [ ] **Step 5: 提交** `git commit -m "issue-82(web): paid_awaiting_activation product state across contracts and pages"`

### Task 9: 真实栈集成测试（lago_integration build tag）【冻结——设计存档，禁止执行：验证对象即被证伪的 settle→active 全链】

**Files:**
- Create: `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go`

**Interfaces:** Consumes: `LAGO_INTEGRATION_BASE_URL/API_KEY/STRIPE_API_KEY/STRIPE_PM_TOKEN` 既有门（lago_purchase_integration_test.go:5-7,53-66）+ 新增 `LAGO_INTEGRATION_STRIPE_SETTLE_PM_TOKEN`（结算 pm，缺省 SKIP）。Produces: pinned 栈上「gated 创建（3DS pm）→ settle 命令 → active + InvoiceFees + 购买钱包」的全链证据测试。

- [ ] **Step 1: 写测试**（照 lago_purchase_integration_test.go 的 env 门与阶段注释风格；一条测试内顺序阶段）：

```
阶段 1（AC1 前置）：真实 gated create（StripePmToken=3DS 必验卡）→ 快照 awaiting_payment；
阶段 2（GC-3，重编前旧号 AC5）：提交 settle_purchase_payment（ChannelTransactionID="it-txn-1"）→ 轮询快照
        （显式 status 集，10s×24 次）到 active；期间断言快照 active 后 InvoiceFees
        恰 1 条 subscription 行且 AmountFen==创建金额（D6 在真实 authority 上的成立）；
阶段 3（AC4/AC3）：同 Key 重放 settle → 回执一致、零新增对象；grant 命令（购买钱包名）
        → 重放零余额变化；
阶段 4：独立租户 B 的 gated 订阅在**不提交 settle**的对照下 120s 内保持 incomplete
        （激活只由 settle 触发的负对照）。
```

- [ ] **Step 2: 无栈运行确认 SKIP 语义**。Run: `go test ./internal/modules/commercial/commercialplatform/ -tags lago_integration -count=1 -run TestLagoSettlementIntegration -v` → Expected: SKIP（blocked-env 说明，不 FAIL——无栈环境与 CI 的诚实姿态）。
- [ ] **Step 3: 有栈运行**（Task 10 的 lab 栈起好后，env 指向 :48895）：Expected: PASS（真实结算证据，输出附 Lago payment provider_payment_id）。
- [ ] **Step 4: 提交** `git commit -m "issue-82(integration): real-Lago settlement chain evidence test"`

### Task 10: 真实流程验证、证据、文档、Ledger、收尾提交【冻结——设计存档，仅「真实流程验证方案」第 1-4 步（已落地能力面）可执行留证，第 5-8 步与收尾提交待修订版计划】

**Files:**
- Create: `docs/plans/issue-72-flow-evidence-82/`（README.md + 截图 + API/Lago 留档 + `alipay_sandbox_notify.py`）
- Modify: `docs/migrations/lago/t10-payment-trigger/`（Task 1 证据如 Task 5 实现后契约有微调则回填）
- Modify: `docs/plans/issue-72-user-rulings.md`（若升级路径触发则记录；否则无改动）
- Ledger: `docs/plans/issue-72-ledger-82.md`（执行账本）

- [ ] **Step 1: 按本计划「真实流程验证方案」章节执行全链验证**，产出该章节列明的全部证据文件并写 README.md（断言表逐条给证据路径）。AC4 残余判定照该章节通过判据：沙箱凭据缺席时 failures 记 `ac4-sandbox-credentials-unavailable`，结论写「AC4 带残余」，不写达成。
- [ ] **Step 2: 红线自查**：`grep -rn "sk_te" docs/plans/issue-72-flow-evidence-82 deploy/lago-lab/payment-trigger docs/migrations/lago/t10-payment-trigger` → 仅允许出现在"凭据纪律说明文字"中；`grep -rn "force.*active\|status.*active.*UPDATE" internal/modules/commercial/ | grep -v _test` 无本地强制 active 写路径；`go run ./tools/architectureguard`、`make verify-module-moves` 通过。
- [ ] **Step 3: 全量回归**：`make test`（Makefile:107-108）全绿；`pnpm gates`（scripts/run-gates.mjs）通过。
- [ ] **Step 4: Ledger 更新**（身份/裁决/证据/遗留），终态提交：

```bash
git add docs/plans docs/migrations/lago/t10-payment-trigger
git commit -m "issue-82(#82): alipay settlement exactly-once activation — flow evidence + ledger"
```

---

## 验收标准 → Task → 测试 追踪矩阵（第 2 轮审查后重编）

编号勘误（第 2 轮审查·low）：`gh issue view 82 --repo 1123786563/WeKnora-fork01` 实取 Acceptance criteria 共 **4 条**；原矩阵的"AC5 受支持通道记录 Payment"实为 Global Constraint 3（spec L125）的派生约束，非 issue 正文条目，现改列 **GC-3**，不再与正式 AC 混排。**现状列含义**：✅=已落地可验证；⛔=被 t10 证伪阻断（Task 5-10 冻结），待 T02 §5 重议裁决后的修订版计划承接——任何执行者不得宣称 ⛔ 行达成。

| Issue #82 正式 AC（实取原文摘要） | Task | 测试（命令可跑） | 现状 |
|---|---|---|---|
| AC1 状态依次呈现待付款、已付款待激活、已生效 | 前置：Task 2-4（已落地）；后半：Task 7、8（冻结） | `TestPurchaseStatusPaidAwaitingActivation`（service 包，冻结）；`commercial.test.ts` parse 新态（冻结）；BillingPage 文案单测（冻结） | ⛔ 三态的激活出口依赖被证伪的 D2；「待付款→已付款待激活」前半的读侧素材（InvoiceFees/快照）已落地 |
| AC2 同步返回页不能确认付款，只有可信渠道事实可推进流程 | Task 7 内 AC2 pin（**不受冻结影响**）；渠道语义既有 | `TestSyncReturnCannotConfirmPurchase`（payment/alipay_test.go，骨架已按真实 helper 修订，可执行）；`TestAlipaySyncReturnNeverConfirms`（既有 :228-235）；`TestPurchaseFulfillCanceledStaysClosed`（冻结） | ✅ 渠道层语义既有且 pin 测试可执行；消费侧闭环（履约推进）仍 ⛔ |
| AC3 重复回调、Outbox 重放与响应丢失不重复履约 | 渠道层+fake：Task 3（已落地）；authority 侧防线：Task 5、7、9（冻结） | `TestFakeSettlePurchasePayment`（重放分支，已落地）；`TestLagoSettleAlreadyActiveNoOutbound`（冻结）；`TestPurchaseFulfillExactlyOnce`/`TestPurchaseFulfillOverPaymentNoSecondBenefit`/`TestPurchaseFulfillCrashReplayConverges`（冻结）；集成测试阶段 3（冻结） | ⛔ 渠道幂等与 fake 重放语义已落地；authority 侧「恰好一次」防线（唯一 pending payment 索引路径）即 t10 证伪标的 |
| AC4 真实支付宝沙箱证据关联 Invoice、Payment、Subscription 和 Credits | Task 1（probe 已执行）、9、10 | probe P1/P2 证据（t10，**verdict=FAIL**）；集成测试（冻结）；flow evidence README 断言表（冻结）。沙箱凭据缺席时 failures 记 `ac4-sandbox-credentials-unavailable`，不写达成 | ⛔ 四对象关联证据依赖激活链；t10 已产出的是证伪证据而非达成证据 |
| GC-3（spec L125 派生约束）只有受支持的 Lago 外部付款集成可记录 Payment；manual Payment 不能释放激活规则时必须换受支持 Provider 适配器，禁止本地强制 active | 排除性证据：F9/F14（既有）；正面通道：Task 5、6（冻结） | `TestLagoSettlePurchasePaymentHappyPath`（冻结，断言只调受支持端点）；契约 settle leg（冻结）；红线 grep 与 architectureguard（Task 10 Step 2，冻结） | ⛔ 本行正是 T02 §5 重议标的：pinned Community v1.53.0 上受支持触发器缺席（t10 实测 404 `invoice_not_found`） |
| Review Focus 1-6 | Task 3、5、7、9 | 见「Review Focus」各行归属 | 同上各行，随归属 Task 的冻结状态 |

## Spec 行为矩阵覆盖对照（#82 分片）

- 矩阵 5「External payment activation：…moves an incomplete subscription to active exactly once」＝**本票主体**（D1/D2/D7 + 全部 Task）。
- 矩阵 7「Payment anomalies」＝本票只交付 refused/attention 不扩大权益的最小面，完整异常面＝#84。
- 矩阵 20「Fault injection」＝Recover 重放/响应丢失/进程崩溃用例（Task 7）+ probe P3。
- 矩阵 6「Top-up fulfillment」＝#85（75-a2 裁决 B 口径），本票不做。

## Out of scope（#82 明确不做）

- 微信渠道复用（#83）、异常付款验收与 over-payment 运营面（#84）、充值批次与到账（#85）、月度续费发放/额度消费顺序（#86/#93）、provider checkout 收银台 UI（生产真实卡的到达面，#83+/后续票）。
- 任何 Lago webhook 接收器（D2 机制不需要；若 Task 1 证伪后重议，属升级事项）。
- 本地/authority 数据库直写的一切形式（含"修状态"式运维脚本）。

## 真实流程验证方案（真实环境端到端，集成分支/worktree 构建运行）

> **【冻结标注（第 2 轮审查）】本方案第 5-8 步依赖被证伪的 D2 激活链（Task 5-7 冻结），在 T02 §5 重议裁决并产出修订版计划前不可执行、不可宣称达成。** 仅第 1-4 步（登录/发布/下单/回调入账+「已付款待激活」呈现）验证的是**已落地能力**（#81 交付 + 本计划 Task 2-4 + 既有渠道语义），可在无裁决情形下执行留证，但其结论只能是「部分面证据」，不得作为 AC1/AC3/AC4 的达成证据（AC4 的四对象关联证据需要激活，属第 5-6 步）。第 5-8 步原文保留为修订版计划的重议输入。

**环境拓扑（全部端口避开 :5272/:5273；:8080 本会话实测已被占，:8084/:8091/:5183 为其他会话与 #81 遗留，:48889-48892 为 operator 栈与 #74 lab）**

| 组件 | 地址 | 启动方式 |
|---|---|---|
| Lago v1.53.0 独立 lab 栈 | 127.0.0.1:48895（API）/ :48896（前端） | `cd deploy/lago-lab/payment-trigger && source ~/.zcode/issue72-stripe.env && ./lab.sh init && ./lab.sh up`（compose 项目 `weknora-lago-82`，冷启动数分钟，`./lab.sh status` 全 healthy 才继续；LAGO_CREATE_ORG 种子按 deploy/lago 惯例写在 lab.env） |
| WeKnora 后端 | 127.0.0.1:8092 | worktree 内 `cp .env.example .env`（`DB_DRIVER=sqlite`、独立库文件 `data/issue82-flow.db`、`SERVER_PORT=8092`），`WEKNORA_COMMERCIAL_PLATFORM_PROVIDER=lago`、`WEKNORA_COMMERCIAL_PLATFORM_URL=http://127.0.0.1:48895`、`WEKNORA_COMMERCIAL_PLATFORM_API_KEY=<lab seed>`、`WEKNORA_COMMERCIAL_STRIPE_API_KEY` 与 `WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN=pm_card_threeDSecure2Required`、`WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN=pm_card_visa`（全部 env 注入，先 `source ~/.zcode/issue72-stripe.env`）；`./scripts/dev.sh app` 或 `go run ./cmd/server`（首编译数分钟）；健康 `GET /health`（internal/router/router.go:175） |
| WeKnora 前端 | localhost:5192 | `pnpm --filter @weknora/web exec vite --port 5192`，`VITE_DEV_PROXY_TARGET=http://127.0.0.1:8092`（apps/web/vite.config.ts:29-44 代理 /api 与 /files） |
| 支付宝沙箱 | 沙箱网关（真实外网） | 支付宝开放平台沙箱 env（ALIPAY_*，.env 注入）；无沙箱凭据时该链路记 blocked-env，用签名 notify 替身（下述第 4 步）并如实标注 |
| Playwright | 无头 chromium | `npx playwright --version` 实测 1.63.0；browser 二进制在 ~/Library/Caches/ms-playwright |

**种子数据**：管理员/ Owner 账号经注册接口自然产生（sqlite 独立实例零冲突）；Plan 发布走 #79 管理面 API（draft→publish，`weknora-pro-v1` 9900 分 CNY 无 charges，#81 flow evidence 同款脚本可复用）；发布 grant 记录进 ledger（不落仓库凭据）。

**操作序列（浏览器 Playwright + 真实 API 链路，每步断言可观察结果并截图/留档到 `docs/plans/issue-72-flow-evidence-82/`）**

1. **登录与发布**（API）：owner 账号登录 → `POST /api/v1/admin/plans` draft + publish（201+receipt）→ Lago 侧确认 `weknora-pro-v1` 落库（`:48895` API 读）。留档 `api-01-draft.json`/`api-02-publish.json`。
2. **购买下单（AC1 前半）**：浏览器 `http://localhost:5192/platform/billing/checkout` 选 Pro → 提交购买 → 断言：Checkout 呈「待付款」+ 报价明细（版本/CNY 9900/行项目/有效期）；Lago 显式 `status[]` 读到 `weknora-tenant-<t>-purchase` = incomplete；`:48895` DB 直查（证据专用，非产品路径）gating invoice `status=open`。截图 `01-checkout-awaiting-payment.png`；留档 `lago-after-purchase.txt`。
3. **支付宝沙箱下单**：`POST /api/v1/commercial/purchases` provider=alipay → 响应 `checkout_url` 为沙箱 precreate 返回的真实二维码串；有沙箱钱包则真机扫码付款（截图 `02-alipay-qr.png`）。留档 `api-03-purchase-alipay.json`。**残余风险披露（AC4）**：支付宝沙箱凭据（ALIPAY_*）的可用性未在本计划编写会话证实——凭据缺席时第 3-4 步降级为「真实 precreate（或 precreate 亦不可用时的本地构造）+ 与渠道同源签名链路构造的 notify」，该替身只能证明"渠道验签→履约"链路，**不构成真实沙箱钱包付款证据**；此情形下 AC4 不得宣称达成：failures 节必须记录 `ac4-sandbox-credentials-unavailable`，收尾报告如实披露，由编排层裁决补验或带残余合并。
4. **可信异步回调（AC2）**：同步返回页/GET 断言**不变**（仍待付款，权益未开放——截图 `03-sync-return-no-confirmation.png`）；随后以沙箱密钥用与渠道一致的签名链路构造 trade_status=TRADE_SUCCESS 的 notify（`alipay_sandbox_notify.py`，签名代码路径与 `payment/alipay_sandbox_test.go` 同源）POST 到 `http://127.0.0.1:8092/api/v1/commercial/callbacks/alipay` → 断言响应体恰为 `success`；重复 POST 同一 notify → 仍 `success` 且订单/事件计数不变（AC3 渠道层）。留档 `callback-01-first.txt`/`callback-02-replay.txt`。
5. **已付款待激活（AC1 中态）**：`GET /api/v1/commercial/purchase` → `state=="paid_awaiting_activation"`；浏览器 Billing 页套餐行呈「已付款待激活（权益未开放）」（截图 `04-billing-paid-awaiting.png`）；此时 entitlements 探针仍 404（权益未开放）。
6. **恰好一次激活（AC1 后半 + GC-3）【第 5-8 步：冻结，依赖被证伪的 D2 激活链，禁止执行】**：fulfillment 轮询内 settle 触发 provider 轨道 → 120s 内 `GET /purchase` 变 `active`；Billing 呈「已生效」+ 套餐 Credits 余额出现（截图 `05-billing-active-credits.png`）；Lago 侧四对象关联留档 `lago-after-settlement.txt`：API 读 payments（`provider_payment_id`、`invoice_ids`）+ 订阅 active + invoice finalized/编号/`payment_status: succeeded` + customer wallets（购买钱包 `weknora-tenant-<t>-purchase-<period>`）+ entitlements 200；DB 直查 invoice/payment/wallet 行关联 ID 一致（AC4 核心证据）。
7. **重复回调/重放不重复履约（AC3 终验）**：重放 notify ×1、重启后端进程一次（Recover 全量重放）→ Lago succeeded payment 计数、购买钱包余额、本地 FulfillmentRecord 数全部不变（`idempotent-after-restart.txt`）。
8. **负对照**：独立租户 B 下单后不付款、不回调 → 30 分钟观察窗内保持待付款且无任何 Lago 付款行（`negative-control.txt`；配合集成测试阶段 4 的 120s 负对照）。

**通过判据（第 2 轮审查后改写）**：原「1-8 全部断言绿」的整链判据**随 D2 证伪失效**。本版计划下有效判据为：(a) 第 1-4 步在真实栈全绿且证据入档——它证明的是「下单→可信回调入账→协调层呈现『已付款待激活』」的已落地面，failures 如实记录；(b) Task 2-4 已落地测试全绿（`go test ./internal/modules/commercial/... -count=1`）；(c) 红线自查（Task 10 Step 2 的 grep 与 architectureguard）零命中；(d) **无论 (a)-(c) 多绿，本票不得宣称 AC1/AC3/AC4/GC-3 达成**——激活链缺席，failures 节必须记 `d2-falsified-t10-p2-fail`（机制级，优先于沙箱凭据项 `ac4-sandbox-credentials-unavailable`）；本票结论只能是「已落地面达成 + 激活链受阻，待 T02 §5 重议」。任何一步不绿，flow evidence README 的 failures 节如实记录，不伪造通过。

## 执行注意事项（并行环境）

- 本票在 worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n82`（分支 `codex/issue-72-lago-82`，基线 7a665bb577）实施；每个 Task 一个 commit，收尾按 Task 10。
- Docker lab 栈（`weknora-lago-82`）与端口 48895/48896/8092/5192 为本票专用，绝不 teardown 他人的 compose 项目；进程回收按 PID（craft-stack.sh 惯例），绝不按名字杀。
- Stripe/支付宝凭据只经环境变量注入；证据/日志/截图入档前跑红线 grep。
- `internal/handler/session/craft_test.go`、`packages/career-core/` 等主 checkout 未跟踪改动与本票无关，不得触碰。
