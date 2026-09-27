# [Lago 11] 微信付款复用同一激活流程 实施计划（Issue #83）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 微信渠道付款走与支付宝完全相同的激活链（微信回调验签 → ConfirmPayment Payment Fact → outbox → PurchaseFulfiller settle → Lago 内建 finalize → active，恰好一次），并补齐微信自己的渠道语义：Native 下单/查单/关单协议单测、回调 HTTP 层测试、关单-晚成功竞态编排，以及微信专属真实受控环境验收。

**Architecture:** #82 已在集成分支（本 worktree 基线 `c6d027df8`）交付渠道无关的激活链：`PaymentCallbacksHandler`（渠道分发）→ `OrderStore.ConfirmPayment`（单事务校验+幂等+outbox）→ `PurchaseFulfiller`（`CommandKindSettlePurchasePayment` → Stripe Provider 结算 → Lago webhook finalize → active）。微信渠道 Provider（`WechatProvider` 的 Create/Query/Close/Refund/Verify）也已实现并经 env 注册。#83 不新造任何激活路径，只做四件事：(1) 用协议级单测 pin 住微信 Create/Query/Close 契约；(2) 用真实 `WechatProvider.Verify` 走通回调 HTTP 层到 outbox 的微信腿测试；(3) 新增唯一的渠道侧生产行为——竞态安全的关单编排 `OrderService.CloseChannelOrder`，并接入 checkout 渠道切换（`Purchase` 重放分支检测 provider 换轨时先关旧渠道单）；(4) 微信专属真实受控环境（完整微信 Native v3 协议 stub：签名请求双向对验 + AES-GCM 签名回调推送 + 可编排状态机）跑浏览器/API 全链验收。

**Tech Stack:** Go（gin + gorm + sqlite 测试库 / `//go:build lago_integration` 真栈门控）、Python 3（微信协议 stub，参照 `docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py` 惯例）、Playwright（`@playwright/test`，仓库 devDependencies，参照 `browser_flow_82.mjs` 惯例）、真实 Lago v1.53.0 compose 栈 + Stripe TEST（R-4 α 双轨道结算腿）。

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`（L123/L124/L125/L127/L165/L218「Behavior and contract matrix #5」）；ADR-0012（`docs/adr/0012-lago-as-commercial-billing-authority.md`，含 2026-09-23 修订）；ADR-0014（`docs/adr/0014-commercial-platform-single-deep-seam.md`，seam 只许加法）；用户裁决 R-4（2026-09-26，α 双轨道：渠道真实收款 + WeKnora 驱动 Stripe gated 结算 → Lago 内建 finalize；渠道沙箱凭据缺失属已披露边界，用本地 RSA stub 证明协议往返+验签，**不得伪造沙箱证据**）。

## 基线核实（计划编写时实读代码结论，修正调查简报的滞后描述）

Issue 调查简报称「lago.go:684-687 不带 activation_rules、CommercialGateway seam 无 Payment 命令、激活链完全缺失」——该描述对应**旧基线**。本 worktree（集成分支 `c6d027df8`，含 `issue-72: ocr issue-82 round 2`）实况：

- `internal/modules/commercial/purchase_settlement_command.go:25` 已有 `CommandKindSettlePurchasePayment`；`internal/modules/commercial/service/commercial/purchase_fulfillment.go:63` 已有 `PurchaseFulfiller.Fulfill`（settle→观察→D6'→grant→activation claim）。
- `internal/handler/payment_callbacks.go:67-114` 微信回调分发→`Verify`→`resolveByMerchantOrderID`→`ConfirmPayment` 已通（#82 缺陷 B/C 已修：`internal/middleware/auth.go:71` 回调路径匿名放行；`internal/modules/commercial/service/commercial/order.go:403-405` attempt 注册 `Merchant: provider.MerchantID()`）。
- `internal/handler/payment_callbacks_test.go` 已存在（2 个支付宝腿测试）；微信腿无覆盖。
- `internal/modules/commercial/payment/wechat.go` Create(:399)/Query(:428)/Close(:446)/Refund(:464)/Verify(:249) 全部已实现；`wechat_test.go` 仅覆盖 Verify 验签与 SSRF 出站门，**Create/Query/Close 无任何单测**。
- `WechatProvider.Close` 与 `AlipayProvider.Close` 均无生产调用方——关单编排是真缺口（AC3 关单侧）。
- 真实受控环境：`docs/plans/issue-72-flow-evidence-81/wechat_pay_stub.py` 只覆盖 Create+Query(NOTPAY)，无 Close、无签名回调推送——不满足 #83 验收。
- 本会话实跑：`go test ./internal/modules/commercial/payment/ -run TestWechat -count=1` → `ok ... 2.103s`（基线绿）。

因此 #83 = 「补微信渠道证据与渠道侧关单语义」，不是重建激活链。

## Global Constraints（批准需求原文引用）

- spec L123：「WeKnora owns WeChat Pay and Alipay request creation, callback verification, query, close, and refund behavior. A verified channel result produces an immutable Payment Fact.」
- spec L124：「A supported Lago external-payment integration records the Payment. Only a Lago Subscription observed as active enables Entitlement and fulfillment.」
- spec L125：「If Lago manual Payment does not release the activation rule in the pinned Community version, the implementation must use a Lago-supported external Payment Provider adapter. It may not force the Subscription active locally.」
- spec L127：「Duplicate, mismatched, partial, wrong-currency, and multiple-success payment cases retain their external facts without automatically changing Invoice amount or delivered benefits.」
- spec L165：「WeKnora-to-Lago commands use a durable Outbox and stable idempotency identity. Timeout and server error are indeterminate outcomes that must be queried before replay.」
- spec L218（矩阵 #5）：「a verified WeChat Pay or Alipay result reaches Lago through a supported integration and moves an incomplete subscription to active exactly once.」
- spec「Public product states」节：「The public Billing API never exposes Lago credentials, internal URLs, Lago Organization identifiers, or raw status enums.」
- 用户裁决 R-4（2026-09-26）：激活链一律按「渠道真实收款 + WeKnora 驱动 Stripe Provider gated 结算扣款 → Lago 内建 finalize → active」设计；微信/支付宝真实沙箱凭据缺失为已披露边界，用本地 RSA stub 证明协议往返+验签，不得伪造沙箱证据。
- ADR-0014：Commercial Platform seam 只许加法；本计划**零 seam 改动**（`SettlePurchasePaymentPayload.ChannelTransaction` 是 string，微信 transaction_id 直接复用）。
- 安全约束（实现验收条件）：渠道出站仅 http/https 且过 SSRF 校验（环回 stub 需显式 `SSRF_WHITELIST_EXTRA=127.0.0.1` 豁免，`wechat.go:545` 已实现）；全部 SQL 用参数绑定（既有 gorm 参数绑定惯例，新增语句同守）；凭据只从环境变量/文件路径注入，源码、测试与证据目录**不得出现任何可用凭据字面量**（含 `sk_test_`、真实商户密钥；本地一次性 RSA 测试密钥在 `t.TempDir()`/运行时目录生成，不进仓库）。
- 端口纪律：验证环境避开 `:5272/:5273`（其他会话）及既往占用口径 `:8080/:8091/:8092/:8093/:5183/:5192/:5194/:8291/:8292/:8294`；本计划统一用后端 `:8095`、前端 `:5196`、微信 stub `:8296`、支付宝 stub `:8297`。

## Review Focus（spec 隐含但任务测试未覆盖、最可能咬人的五类输入；每行已归属到 owning task 的测试）

1. **平台证书轮换窗口的未知 serial 回调**（spec L123 验签职责）：`Wechatpay-Serial` 不在配置集 → 必须 401 且零持久化 → Task 2 `TestWechatCallbackRejectsUnknownSerialNothingPersisted`。
2. **金额/币种不符的微信成功通知**（用户实付与订单面不符，spec L127）：必须 409 `ErrPaymentMismatch`、零 outbox、订单仍 pending → Task 2 `TestWechatCallbackAmountMismatchRejectedNoFact`。
3. **关单撞在途支付（ORDER_PAID 竞态，spec L165 不定结局必须查单）**：Close 失败必须以 Query 决胜，资金事实保留、恰好一次履约 → Task 3 `TestCloseChannelOrderPaidRaceConfirmsFundFact`。
4. **渠道 Create 超时**（spec L165 不定结局）：必须回 `StateUnknown` 且键回原 `merchant_order_id`，恢复查原单、绝不重键 → Task 1 `TestWechatCreateTimeoutReturnsUnknownWithOriginalKey`。
5. **关单后旧渠道晚到成功（跨渠道 multiple-success，spec L127）**：重复成功只落 over-payment 审计事件恰一条，不二次履约 → Task 3 `TestLateSuccessAfterCloseAuditsWithoutSecondFulfillment`。

---

## File Structure（创建/修改全清单）

- Create: `internal/modules/commercial/payment/wechat_native_test.go` —— 微信 Native v3 协议 pin 测试（Create/Query/Close，Task 1）。
- Modify: `internal/handler/payment_callbacks_test.go` —— 追加微信腿 fixture 与测试（Task 2，文件已存在，保留支付宝腿）。
- Create: `internal/modules/commercial/service/commercial/order_close_test.go` —— 关单编排与渠道切换测试（Task 3）。
- Modify: `internal/modules/commercial/repository/commercial/order.go` —— `PaymentAttemptStateClosed` 常量 + `MarkAttemptClosed`（Task 3）。
- Modify: `internal/modules/commercial/service/commercial/order.go` —— `CloseChannelOrder`（Task 3）。
- Modify: `internal/modules/commercial/service/commercial/purchase.go` —— 重放分支 provider 换轨关单（Task 3，:253-261 区域）。
- Create: `docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py` —— 完整微信 Native v3 受控 stub（Task 4）。
- Create: `docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs` 等 —— 验收脚本与证据（Task 4，产出时落盘）。
- Create: `docs/plans/issue-72-flow-evidence-83/README.md` —— 证据索引（Task 4）。
- Modify: `docs/plans/issue-72-ledger-83.md` —— 执行账本续写（Task 4 收尾）。

不改：`platform.go`/`purchase_settlement_command.go`/`lago*.go`（seam 与激活链零改动——#83 的「复用」承诺）、`apps/web/**`（渠道选择器与三态呈现 #82 已交付：`CheckoutPage.tsx:188` 提交 `provider: channel`，`:301-314` 微信单选，`packages/contracts/src/commercial.ts:52` `weixin://wxpay/` scheme 已放行）、`internal/handler/payment_callbacks.go`（分发逻辑已渠道通用）。

---

### Task 1: 微信 Native 协议 pin 测试（Create/Query/Close）

**Files:**
- Create: `internal/modules/commercial/payment/wechat_native_test.go`

**Interfaces:**
- Consumes（全部既有，零改动）：`WechatProvider.Create(ctx, OrderRequest) (AttemptResult, error)`（wechat.go:399）、`Query(ctx, providerID string) (AttemptResult, error)`（:428）、`Close(ctx, providerID string) error`（:446）、`newWechatProvider(cfg WechatConfig, platformKeys map[string]*rsa.PublicKey, apiv3Key []byte) *WechatProvider`（wechat.go:179，包内可见）、`WechatConfig{AppID, MchID, MchSerial, MchKeyPath, APIBaseURL, Timeout}`、`ErrInvalidRequest`、`StatePending/StateSucceeded/StateClosed/StateUnknown`、`secutils.ResetSSRFWhitelistForTest()`。
- Produces: 测试 helper `newNativeFixture(t, handler http.HandlerFunc) *WechatProvider`（生成商户 RSA 私钥写 `t.TempDir()` PEM、`t.Setenv("SSRF_WHITELIST","127.0.0.1")` + reset、httptest 服务、构造 provider）与 `captureWechat{Method, Path, Body string, Auth string}` 请求捕获器——Task 2/3 不直接用（handler/service 层用各自的 stub），仅本文件内复用；行为 pin 供 Task 3/4 依赖（ORDER_PAID 错误形状 `wechat api status 400 code=ORDER_PAID`）。

- [ ] **Step 1: 写测试（全部落 `wechat_native_test.go`；这是对已实现行为的特征测试——落盘后如与微信 v3 契约不符即实现缺陷，修实现到 GREEN）**

```go
// newNativeFixture：商户私钥 PEM（PKCS1）写 TempDir → WechatConfig{MchSerial:"MCH-SERIAL-1",
// MchKeyPath:…, APIBaseURL: srv.URL} → newWechatProvider(cfg, nil, nil)。
// handler 同时把 {Method, Path, string(Body), r.Header.Get("Authorization")} 记入 *[]captureWechat。

func TestWechatCreatePostsNativeOrderAndMapsCodeURL(t *testing.T) {
	// stub 应 {"code_url":"weixin://wxpay/bizpayurl?pr=n83"}
	// res, err := p.Create(ctx, OrderRequest{OrderID:"ord_83", MerchantOrderID:"mo_83", AmountFen:9900, Currency:"CNY"})
	// 断言 err==nil; res.State==StatePending; res.ProviderID=="mo_83"; res.CheckoutURL=="weixin://wxpay/bizpayurl?pr=n83"
	// 断言捕获：Method POST；Path "/v3/pay/transactions/native"
	// Authorization 前缀 "WECHATPAY2-SHA256-RSA2048" 且含 `mchid="1900000001"`、`serial_str="MCH-SERIAL-1"`
	// body JSON：appid=="wx-test-app" 且 mchid=="1900000001" 且 out_trade_no=="mo_83"
	//           且 amount.total==9900 且 amount.currency=="CNY" 且 description 含 "ord_83"
}

func TestWechatCreateRejectsInvalidRequestLocally(t *testing.T) {
	// 空 MerchantOrderID、AmountFen<=0 两例 → errors.Is(err, ErrInvalidRequest)；stub 收到 0 次请求
}

func TestWechatCreateTimeoutReturnsUnknownWithOriginalKey(t *testing.T) {
	// cfg.Timeout=200ms，stub sleep 1s → err!=nil 且 res.State==StateUnknown 且 res.ProviderID=="mo_t"
	// （spec L165：不定结局键回原 merchant_order_id，恢复必须 Query 原单）
}

func TestWechatQueryMapsTradeStates(t *testing.T) {
	// 表驱动：响应 trade_state SUCCESS/NOTPAY/CLOSED/REFUND + amount.total
	// 期望 State==StateSucceeded/StatePending/StateClosed/StateSucceeded（REFUND 语义：成功过一次）
	// 且响应缺 out_trade_no 时 res.ProviderID==请求 id（回退原单号）
}

func TestWechatQueryEmptyProviderIDRejected(t *testing.T) { // "" → ErrInvalidRequest，零请求 }

func TestWechatClosePostsMchIDBody(t *testing.T) {
	// stub 204 空体 → err==nil；断言 Method POST；Path "/v3/pay/transactions/out-trade-no/mo_c/close"；body {"mchid":"1900000001"}
}

func TestWechatClosePropagatesOrderPaidCode(t *testing.T) {
	// stub 400 {"code":"ORDER_PAID","message":"订单已支付，禁止关单"} → err!=nil 且 strings.Contains(err.Error(),"ORDER_PAID")
	// （Task 3 的关单竞态编排靠此错误形状触发 Query 决胜；Task 4 stub 同契约）
}
```

- [ ] **Step 2: 运行（特征测试，期望 PASS；任何 FAIL 即实现与契约偏差，修 `wechat.go` 后复跑）**

Run: `go test ./internal/modules/commercial/payment/ -run 'TestWechat(Create|Query|Close)' -count=1 -v`
Expected: 全部 PASS（若 FAIL：按微信 APIv3 契约修正 `wechat.go` 对应实现后 PASS，并在 Ledger 记录偏差）。

- [ ] **Step 3: 全包回归 + Commit**

Run: `go test ./internal/modules/commercial/payment/ -count=1`
Expected: `ok github.com/Tencent/WeKnora/internal/modules/commercial/payment`

```bash
git add internal/modules/commercial/payment/wechat_native_test.go
git commit -m "issue-72(#83): (task1) pin wechat native create/query/close protocol contract"
```

---

### Task 2: 微信回调 HTTP 层测试（真实 Verify → ConfirmPayment → outbox → fulfiller 链）

**Files:**
- Modify: `internal/handler/payment_callbacks_test.go`（追加；保留既有支付宝腿与 `newCallbackTestEnv` 惯例——真实 OrderService 落库 + 真实 handler + sqlite 内存库）

**Interfaces:**
- Consumes: `handler.PaymentCallbacksHandler.HandleProviderCallback`（payment_callbacks.go:67）；`NewWechatProvider(cfg WechatConfig) (*WechatProvider, error)`（wechat.go:136，公钥构造器：从 `PlatformCerts` 路径加载平台公钥、`APIv3KeyPath` 读 32 字节 key）；`VerifyWechatSignature` 语义（wechat.go:66）；`repocommercial.OutboxEvent{Kind: OutboxKindFulfill, EventKey: "fulfill:"+orderID}`；`commercialsvc.NewPurchaseFulfiller(db, platform, nil) (*PurchaseFulfiller, error)` + `Fulfill(ctx, ev) error`（purchase_fulfillment.go:57/63）；`commercialplatform.NewFakeAdapter(...)`（contract_test.go 既有 fake，Task 2 仅用其 settle→active 钩子 `ActivatePurchase(extSubID)`，fake.go:265 既有）。
- Produces（测试内 fixture，供本文件微信腿共享）：`newWechatCallbackEnv(t) (*gorm.DB, *WechatProvider, *gin.Engine, *commercialsvc.OrderService, signing helpers)`——生成平台 RSA 密钥对（私钥留测试侧签名、公钥 PEM 写 TempDir 经 `NewWechatProvider` 加载）、apiv3 32 字节 key 写文件、`buildWechatNotify(t, p, outTradeNo, txnID, totalFen, state)` 构造签名回调（复刻 `wechat_test.go:90-155` 的 `buildPaymentResource`/`signCallback`/`callbackHeaders`，因跨包不可导入，复制实现并注明同源）。AutoMigrate 在既有 `newCallbackTestEnv` 五表（payment_callbacks_test.go:64-65）基础上**追加 `repocommercial.FulfillmentRecord` 与 `repocommercial.PublicationRow`**（组合测试 `TestWechatCallbackDrainsToFulfilledWithFakePlatform` 需要——fulfiller 写 activation claim 并读 publication）。

- [ ] **Step 1: RED——追加微信腿测试（`CloseChannelOrder` 无关，此处全部针对既有链路；新测试名落盘后先跑，未知项预期 PASS、若链路有缺陷则 FAIL 即缺陷证据）**

```go
// 落单：真实 OrderService + wechatCreateStub（MerchantID()=="1900000001"，Create 返回 Pending+URL）
// ——照 callbackAlipayStub 形状新建 callbackWechatCreateStub；回调侧用真实 WechatProvider.Verify。

func TestWechatCallbackConfirmsOrderAndEmitsFulfillEvent(t *testing.T) {
	// 前置：openOrder 落 pending 订单 + attempt（provider=wechat, merchant=1900000001, 9900 CNY）
	// POST /api/v1/commercial/callbacks/wechat，签名 TRANSACTION.SUCCESS（out_trade_no=attempt.MerchantOrderID,
	// transaction_id="wx_txn_83", amount.total=9900, mchid/appid 与配置一致）
	// 断言：HTTP 200 且 body {"code":"SUCCESS"}
	// DB：attempt.state==succeeded 且 provider_transaction_id=="wx_txn_83"；order.state==paid
	// outbox：OutboxKindFulfill 事件恰 1 条，EventKey=="fulfill:"+orderID，payload JSON provider=="wechat"
}

func TestWechatCallbackDrainsToFulfilledWithFakePlatform(t *testing.T) {
	// 同上落单+回调后：NewPurchaseFulfiller(db, fakeAdapter, nil)；读出 outbox 事件直接 Fulfill(ctx, ev)
	// fake 先 ActivatePurchase(commercial.ExternalPurchaseSubscriptionID(tenant))（webhook finalize 替身）
	// 断言：Fulfill 返回 nil；订单 fulfilled；fulfillment_records 恰 1 条 applied（purchase_activation）
	// ——AC1「复用同一激活流程」的组合级 pin：微信事实驱动同一条 #82 链
}

func TestWechatCallbackDuplicateDeliveryIdempotent(t *testing.T) {
	// 同一签名通知体 POST 两次 → 两次 200 {"code":"SUCCESS"}
	// 计数不变：outbox fulfill 恰 1、attempt succeeded 恰 1、订单 paid（version 不再 +1）
}

func TestWechatCallbackDifferentTransactionOverPaidAudit(t *testing.T) {
	// 回调 txn A 确认后，再投同 out_trade_no 不同 transaction_id 的成功通知
	// → 200；outbox 新增 OutboxKindOverPaid 恰 1（键含 txn）；fulfill 仍恰 1；订单不被二次履约
}

func TestWechatCallbackRejectsTamperedSignature(t *testing.T) {
	// 篡改 body 一字节（签名失配）→ 401；DB 零写入（attempt/order/outbox 全不变）
}

func TestWechatCallbackRejectsUnknownSerialNothingPersisted(t *testing.T) {
	// Wechatpay-Serial 用未配置 serial → 401；零持久化（Review Focus #1）
}

func TestWechatCallbackUnknownAttemptNotFound(t *testing.T) {
	// out_trade_no 未注册 → 404 {"code":"FAIL"}；零持久化
}

func TestWechatCallbackAmountMismatchRejectedNoFact(t *testing.T) {
	// 通知 amount.total=100（订单 9900）→ 409；attempt/order/outbox 全不变（Review Focus #2）
}
```

- [ ] **Step 2: 运行**

Run: `go test ./internal/handler/ -run 'TestWechatCallback' -count=1 -v`
Expected: PASS（这是对已修复链路的 pin；任何 FAIL 即真实缺陷——记录 Ledger 并按 ConfirmPayment/Verify 契约修复后复跑）。

- [ ] **Step 3: 回归 + Commit**

Run: `go test ./internal/handler/ ./internal/modules/commercial/service/commercial/ -count=1`
Expected: 两包 `ok`。

```bash
git add internal/handler/payment_callbacks_test.go
git commit -m "issue-72(#83): (task2) wechat callback http leg pins — verify/confirm/outbox/fulfiller reuse"
```

---

### Task 3: 关单竞态编排 CloseChannelOrder 与渠道切换接线（AC3 核心，唯一新生产行为）

**Files:**
- Modify: `internal/modules/commercial/repository/commercial/order.go`（:31-32 状态常量区 + `MarkChannelFailed`(:422) 附近）
- Modify: `internal/modules/commercial/service/commercial/order.go`（`RecoverOrderStatus`(:498) 之后）
- Modify: `internal/modules/commercial/service/commercial/purchase.go`（:253-261 重放分支）
- Create: `internal/modules/commercial/service/commercial/order_close_test.go`

**Interfaces:**
- Consumes: `payment.Provider` 接口（Close/Query）；`OrderStore.GetOrder/FirstPendingAttempt/MarkChannelFailed/ConfirmPayment`（order.go:481/469/422/584）；`domain.PaymentFact`；`ErrOrderTenantMismatch`、`ErrPaymentProviderUnconfigured`（service 既有）。
- Produces:
  - `const PaymentAttemptStateClosed = "closed"`（repository/order.go，与 `PaymentAttemptStatePending/Succeeded` 并列）。
  - `func (s *OrderStore) MarkAttemptClosed(ctx context.Context, orderID string) error` —— `UPDATE commercial_payment_attempts SET state='closed' WHERE order_id = ? AND state = 'pending'`（gorm 参数绑定；幂等）。
  - `func (s *OrderService) CloseChannelOrder(ctx context.Context, tenantID uint64, orderID string) (OrderView, error)` —— 竞态安全关单编排（算法见 Step 3，签名+测试不足以决定的部分）。
  - `PurchaseService.Purchase` 重放分支新增语义（不新增导出符号）：待付单 provider 与请求 provider 不同 → 先 `CloseChannelOrder`（paid → 直接作答 paid 单；closed → 落到 `CreateOrder(quoteID, providerName)` 开新渠道单）。
  - `MarkChannelFailed` 注释更新（覆盖 closed 语义：「渠道入口不可再支付——渠道失败或已关单」；索引行为不变）。

- [ ] **Step 1: RED——写 `order_close_test.go`（sqlite 内存库 + 可编排 wechat/alipay stub provider：`Close` 按 `closeResult` 字段返回 nil/ORDER_PAID 错误，`Query` 按 `queryState` 返回）**

```go
type closeRaceStub struct {
	merchant    string
	closeErr    error            // nil=204 成功；非 nil=渠道错误（含 ORDER_PAID 形状）
	queryRes    payment.AttemptResult
	queryErr    error
	closeCalls  int
	queryCalls  int
}
// 实现 payment.Provider 全接口；Create 返回 Pending+URL；MerchantID 返回渠道商户号。

func TestCloseChannelOrderClosesPendingAndFreesSlot(t *testing.T) {
	// 落 pending 微信单（checkout_url 有值）→ CloseChannelOrder(tenant, orderID)
	// 断言 err==nil；view.State==pending；attempt.state=="closed"；order.channel_failed==true
	// CurrentPendingPurchaseOrder(tenant, 9900, "CNY") → ErrOrderNotFound（支付槽释放）
}

func TestCloseChannelOrderPaidRaceConfirmsFundFact(t *testing.T) {
	// stub.closeErr=ORDER_PAID 形状；stub.queryRes={State:StateSucceeded, ProviderID:"wx_txn_race"}
	// → CloseChannelOrder 返回 view.State==paid
	// DB：attempt succeeded+provider_transaction_id=="wx_txn_race"；订单 paid；outbox fulfill 恰 1
	// stub.closeCalls==1 且 queryCalls==1（Review Focus #3：Close 失败必须查单决胜）
}

func TestLateSuccessAfterCloseAuditsWithoutSecondFulfillment(t *testing.T) {
	// 关单成功（closeErr=nil）后：订单已被 ConfirmPayment 走到 paid+outbox1 的前提下
	// （先回调确认成功、再关单撞已付）投递不同 txn 的第二笔成功回调/ConfirmPayment
	// → outbox 恰新增 1 条 OverPaid 审计、fulfill 仍恰 1、无第二次履约（Review Focus #5）
	// 反向时序同样断言：先关单成功，再投首笔晚到成功 → ConfirmPayment 幂等落账，订单 paid 恰一次履约
}

func TestCloseChannelOrderIndeterminateStaysPending(t *testing.T) {
	// closeErr=transport error 且 queryErr=error → 返回 err；attempt 仍 pending、channel_failed 不变、订单仍 payable
	// closeErr=error 且 queryRes=NOTPAY → 返回原 close err；同上零标记（不定结局不得收口）
}

func TestCloseChannelOrderQueryClosedIdempotentMark(t *testing.T) {
	// closeErr=ORDER_CLOSED 已关错误；queryRes={State:StateClosed} → 同成功路径落 closed+channel_failed
}

func TestCloseChannelOrderNonPendingNoop(t *testing.T) {
	// 已 paid 订单 → 返回当前 view（state==paid）；stub.closeCalls==0
}

func TestCloseChannelOrderTenantMismatch(t *testing.T) {
	// 租户 2 关租户 1 的单 → ErrOrderTenantMismatch；零渠道调用
}

// ——渠道切换接线（purchase 层，落单经真实 PurchaseService）——
func TestPurchaseSwitchesChannelByClosingOldOrder(t *testing.T) {
	// 表驱动 [old=wechat→new=alipay, old=alipay→new=wechat]：
	// 先 purchase(quoteA, old) 落待付单（stub Create 成功）；再 quote(新) + purchase(quoteB, new)
	// 断言：旧单 channel_failed==true 且 attempt closed；新单 provider==new 且 checkout_url 非空
	// CurrentPendingPurchaseOrder 恰返回新单（一张可付单不变量保持）
}

func TestPurchaseSwitchPaidRaceAnswersPaidOrder(t *testing.T) {
	// 旧微信单在途支付（stub Query=SUCCESS, Close=ORDER_PAID）→ purchase(quoteB, alipay)
	// 断言：答 paid 的旧单（order.provider==wechat, state==paid）；新渠道 Create 零调用；outbox 恰 1
}
```

Run: `go test ./internal/modules/commercial/service/commercial/ -run 'TestCloseChannelOrder|TestPurchaseSwitch' -count=1`
Expected: **FAIL（编译错误：`CloseChannelOrder`/`MarkAttemptClosed`/`PaymentAttemptStateClosed` 未定义）**——这是真 RED。

- [ ] **Step 2: GREEN——实现（`CloseChannelOrder` 算法体，Step 1 测试与签名不能唯一决定处）**

```go
// CloseChannelOrder 竞态安全地退役一张 pending 订单的渠道入口（spec L123 close、L165 不定结局查单）：
// 1. row := GetOrder(orderID)；row.TenantID != tenantID → ErrOrderTenantMismatch
// 2. row.State != OrderStatePending → 返回当前 view（无渠道入口可关；调用方按现状作答，零渠道调用）
// 3. att := FirstPendingAttempt(orderID)；ErrPaymentAttemptNotFound → MarkChannelFailed(orderID)
//    （无 attempt 的 pending 残留本就不可付）→ 返回 pending view
// 4. provider := s.providers[att.Provider]；缺 → ErrPaymentProviderUnconfigured
// 5. cerr := provider.Close(ctx, att.MerchantOrderID)
//    - cerr == nil → MarkAttemptClosed(orderID) + MarkChannelFailed(orderID) → 返回 pending view
//    - cerr != nil →（含 ORDER_PAID 竞态与超时——一律查单决胜，不解析渠道错误码）
//        res, qerr := provider.Query(ctx, att.MerchantOrderID)
//        - qerr != nil → 返回 qerr（零标记，保持可重试）
//        - res.State == StateSucceeded → txn := res.ProviderID（空则 att.MerchantOrderID）
//          ConfirmPayment(PaymentFact{Provider: att.Provider, Merchant: att.Merchant,
//            AttemptID: att.MerchantOrderID, OrderID: orderID, TenantID: tenantID,
//            Amount: CNYFen(att.AmountFen), Currency: att.Currency, Transaction: txn,
//            State: StateSucceeded.String()})
//          → 重读订单返回 paid view（资金事实保留；履约交既有 outbox→PurchaseFulfiller 恰好一次）
//        - res.State == StateClosed → 同 cerr==nil 落账分支
//        - 其他（NOTPAY/Unknown）→ 返回 cerr（未定不收口）
```

`purchase.go:253` 重放分支改造（在 `quoteBoughtPlan` 校验之后、`orderViewFromRow(existing)` 之前）：

```go
if existing.Provider != providerName {
	closeView, cerr := s.orders.CloseChannelOrder(ctx, tenantID, existing.ID)
	if cerr != nil {
		return PurchaseView{}, cerr // 未定关单结局：显式失败，调用方重试（绝不在旧单未收口时开第二渠道单）
	}
	if closeView.State == domain.OrderStatePaid {
		return s.purchaseView(p, snap, pub, &closeView), nil // 在途支付落账：答已付旧单，不开新单
	}
	// 关单成功：支付槽已释放，落入下方 CreateOrder(quoteID, providerName) 开新渠道单
} else {
	ov := orderViewFromRow(existing)
	return s.purchaseView(p, snap, pub, &ov), nil // 同渠道重放：现状不变（#82 冻结语义）
}
```

- [ ] **Step 3: 复跑 Step 1 命令 → 全部 PASS。**

Run: `go test ./internal/modules/commercial/service/commercial/ -run 'TestCloseChannelOrder|TestPurchaseSwitch' -count=1 -v`
Expected: PASS。

- [ ] **Step 4: 全量回归（商业域 + handler + architecture guard）**

Run: `go test ./internal/modules/commercial/... ./internal/handler/ -count=1 && make check-backend-architecture`
Expected: 全 `ok`；architecture guard `0 violations`。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/commercial/repository/commercial/order.go internal/modules/commercial/service/commercial/order.go internal/modules/commercial/service/commercial/purchase.go internal/modules/commercial/service/commercial/order_close_test.go
git commit -m "issue-72(#83): (task3) race-safe channel close orchestration + checkout channel-switch wiring"
```

---

### Task 4: 真实受控环境全链验收、证据与收尾

**Files:**
- Create: `docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py`（见「真实流程验证方案」stub 规格）
- Create: `docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs`（Playwright 主链：下单→待付款→支付→已付款待激活→已生效）
- Create: `docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs`（漏通知查单恢复 + 关单竞态 + 重放幂等 API 腿）——或合并为一个脚本分幕执行，证据文件名不变
- Create: `docs/plans/issue-72-flow-evidence-83/README.md` + 证据落盘（截图/JSON/txt，见验证方案）
- Modify: `docs/plans/issue-72-ledger-83.md`（执行记录续写）

**Interfaces:**
- Consumes: 运行中的 `weknora-lago-82flow` Lago 栈（:48889/:48890，本会话 `docker ps` 实测 healthy，Stripe provider `weknora-stripe` 已注册）；`~/.zcode/issue72-stripe.env`（source 注入 STRIPE TEST key——任何产出不得含 `sk_test_` 字面量）；`docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py`（切轨腿的新渠道 Create 所需，verbatim 复用 + 本地 RSA 密钥重生成）；`docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs`（登录/grant/轮询断言的脚本模板）。
- Produces: `docs/plans/issue-72-flow-evidence-83/` 证据包（Issue AC1-AC4 判定输入）；`wechat_native_stub.py` 可复用于 #84 生命周期票。

- [ ] **Step 1: 写 `wechat_native_stub.py`（完整微信 Native v3 受控 stub；规格见「真实流程验证方案」§stub）+ 冒烟**：`python3 -c "import ast; ast.parse(open('docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py').read())"` → exit 0；启动后 `curl -s -X POST 127.0.0.1:8296/v3/pay/transactions/native -d '{}'` 返回 401（缺 Authorization 签名被拒——stub 验签 fail-closed）。
- [ ] **Step 2: 按「真实流程验证方案」全量执行五幕（主链浏览器 / 漏通知恢复 / 关单竞态 / 重放幂等 / Lago 四对象核验），证据落盘到 `docs/plans/issue-72-flow-evidence-83/`。**
- [ ] **Step 3: 回归 + 红线自查**：`go test ./internal/modules/commercial/... ./internal/handler/ -count=1` 全 ok；`make check-backend-architecture` 0 violations；`grep -rn "sk_test_" docs/plans/issue-72-flow-evidence-83/ internal/ | wc -l` → 0；`grep -rniE "force.*active|直接.*(写|置).*active" internal/modules/commercial/commercialplatform/*.go | grep -v _test | wc -l` → 0（AC4' 红线：无本地强制 active）。
- [ ] **Step 4: 写 README.md（断言结果总览表 + 环境拓扑 + 披露：微信真实商户凭据不可得——本地 RSA stub 证明协议往返+验签，不构成真实微信钱包付款证据，对称适用 R-4 对支付宝的披露口径）+ 续写 Ledger。**
- [ ] **Step 5: Commit**

```bash
git add docs/plans/issue-72-flow-evidence-83 docs/plans/issue-72-ledger-83.md
git commit -m "issue-72(#83): (task4) real-env wechat activation flow evidence + docs + ledger"
```

---

## 真实流程验证方案（真实环境端到端；在集成分支 worktree 构建+运行）

### 环境拓扑（全部真实运行；端口避开 :5272/:5273 及既往占用口径，统一 +1/+2 漂移）

| 组件 | 地址 | 说明 |
|---|---|---|
| Lago v1.53.0 | 127.0.0.1:48889 / :48890 | **复用运行中的 `weknora-lago-82flow` 栈**（docker ps 实测 healthy；seed org 与 Stripe provider `weknora-stripe` 已注册，真实 Stripe TEST key 仅环境变量注入）。若需重建：`cd deploy/lago && ./lago.sh init && (追加 LAGO_CREATE_ORG=true 等 seed) && ./lago.sh up`，`./lago.sh status` 健康检查 |
| WeKnora 后端 | 127.0.0.1:8095 | 本 worktree `go run ./cmd/server`（首次编译数分钟）；`DB_DRIVER=sqlite DB_PATH=data/issue83-flow.db`（独立新库）；商业平台 env：`WEKNORA_COMMERCIAL_PLATFORM_PROVIDER=lago WEKNORA_COMMERCIAL_PLATFORM_URL=http://127.0.0.1:48889 WEKNORA_COMMERCIAL_PLATFORM_API_KEY=<deploy/lago/.env 的 key>`；Stripe 腿：`source ~/.zcode/issue72-stripe.env` 注入 `WEKNORA_COMMERCIAL_STRIPE_API_KEY`，`WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN=pm_card_threeDSecure2Required`（3DS 必验卡稳待付窗口）、`WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN=pm_card_visa`；微信渠道 env 见下；`SSRF_WHITELIST_EXTRA=127.0.0.1`（环回 stub 可审计豁免）；`WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true`；健康检查 `curl -s 127.0.0.1:8095/health` |
| 微信 Native stub | 127.0.0.1:8296 | `wechat_native_stub.py`（本票交付）：真实 WechatProvider 的协议对端 |
| 支付宝 stub（切轨腿） | 127.0.0.1:8297 | 复用 `../issue-72-flow-evidence-82/alipay_gateway_stub.py`，本地 RSA 密钥重生成 |
| WeKnora 前端 | localhost:5196 | `VITE_DEV_PROXY_TARGET=http://127.0.0.1:8095 pnpm --filter @weknora/web exec vite --port 5196`（/api 代理 :8095） |
| Playwright | headless chromium | 仓库 devDependencies `@playwright/test`（`browser_flow_83.mjs`）；本会话亦挂 playwright MCP 插件可交互复核 |

### 微信渠道 env（一次性本地密钥，运行时目录 `${TMPDIR}/issue83-keys/`，**不进仓库**）

```
WEKNORA_WECHAT_APP_ID=wx83issue83flow
WEKNORA_WECHAT_MCH_ID=1900000830
WEKNORA_WECHAT_MCH_SERIAL=MCH-SERIAL-83
WEKNORA_WECHAT_MCH_KEY_PATH=$KEYS/mch_private.pem        # openssl genrsa 2048
WEKNORA_WECHAT_APIV3_KEY_PATH=$KEYS/apiv3.key            # head -c 32 /dev/urandom | base64 解回 32 字节
WEKNORA_WECHAT_PLATFORM_CERTS=PLAT-SERIAL-83:$KEYS/platform_pub.pem
WEKNORA_WECHAT_API_BASE_URL=http://127.0.0.1:8296
WEKNORA_WECHAT_NOTIFY_URL=http://127.0.0.1:8095/api/v1/commercial/callbacks/wechat
```

### `wechat_native_stub.py` 规格（AC4「真实受控环境」的渠道对端）

- 仅绑定 127.0.0.1。渠道端点（只实现 adapter 会调用的）：
  - `POST /v3/pay/transactions/native`：**验证 WeKnora 出站签名**（Authorization `WECHATPAY2-SHA256-RSA2048`，用商户公钥 `$KEYS/mch_public.pem` 验 `METHOD\nPATH\nts\nnonce\nbody\n` RSA-SHA256；失配 401）→ 登记 `out_trade_no → {total, state=NOTPAY}` → `{"code_url":"weixin://wxpay/bizpayurl?pr=issue83"+随机}`。
  - `GET /v3/pay/transactions/out-trade-no/{id}?mchid=`：按状态表回答 `{"out_trade_no":id,"trade_state":<state>,"amount":{"total":<total>,"currency":"CNY"}}`。
  - `POST /v3/pay/transactions/out-trade-no/{id}/close`：state==SUCCESS → 400 `{"code":"ORDER_PAID",...}`（竞态形状，与 Task 1 pin 一致）；否则置 CLOSED → 204。
- 编排端点（仅 loopback）：`POST /stub/mark`（body `{"out_trade_no":..,"state":"SUCCESS"|"CLOSED","transaction_id":..}`）改状态；`POST /stub/notify`（body `{"out_trade_no":..}`）**推送签名回调**：构造 `TRANSACTION.SUCCESS` envelope，resource 以 APIv3 key AES-256-GCM 加密（nonce 12 字节、AAD `"transaction"`），平台私钥对 `ts\nnonce\nbody\n` RSA-SHA256 签名，携 `Wechatpay-Serial/Timestamp/Nonce/Signature` 头 POST `WEKNORA_WECHAT_NOTIFY_URL`，断言答 `{"code":"SUCCESS"}` 200。
- 凭据纪律：key 路径全部经 env/CLI 注入；stub 源码零密钥字面量。

### 种子数据（sqlite 独立库，Lago 独立租户，零污染）

| 账号 | 租户 | 角色 |
|---|---|---|
| issue83-flow-a@verify.local | 新 tenant（自增） | owner（浏览器主链主角：Billing→Checkout 选微信→支付→三态） |
| issue83-flow-b@verify.local | 新 tenant | owner + `plan_publish` grant（API 发布 plan） |

Plan：`pro` v1（9900 分 CNY、monthly、pay_in_advance、无 charges、features advanced_models）——经 `POST /api/v1/commercial/admin/plans/draft` + `/publish`（照 #82 evidence 脚本 api-01/02 模板；发布幂等落到 Lago `pro:1` 既有计划码，r4-flow3 已验证同码重发布可行）。注册/grant 流程照 `browser_flow_82.mjs` 既有机制。

### 五幕验证与通过判据（AC → 可观察结果）

**幕一（AC1 主链，浏览器）**——`browser_flow_83.mjs`（Playwright headless，截图存 `docs/plans/issue-72-flow-evidence-83/`）：
1. 登录 issue83-flow-a → `http://localhost:5196/platform/billing` → 进 Checkout（`/platform/billing/checkout`；路由见 `apps/web/src/routes.tsx:80`）。
2. 断言「支付渠道」单选存在且默认支付宝；**选微信支付** → 提交。
3. 断言页面呈现「等待付款/待付款（权益未开通）」且存在 `前往支付` 链接，`href` 以 `weixin://wxpay/` 开头（Native code_url；stub 日志同时记录 NATIVE out_trade_no 与金额 99.00 CNY）。截图 `01-checkout-wechat-awaiting-payment.png`。
4. `curl -X POST 127.0.0.1:8296/stub/mark -d '{"out_trade_no":"<mo_..>","state":"SUCCESS","transaction_id":"wx_txn_83_main"}'` + `curl -X POST 127.0.0.1:8296/stub/notify -d '{"out_trade_no":"<mo_..>"}'`（模拟真实扫码付款+官方通知；stub 推送日志与 WeKnora 200 回执存 `stub-callback-01.txt`）。
5. 页面轮询（3s）后断言「已付款，权益处理中」（paid_awaiting_activation）。截图 `02-checkout-paid-awaiting-activation.png`；Billing 页断言「待付款（权益未开通）」消失、呈现中态。截图 `03-billing-paid-awaiting.png`。
6. 等待 settle→webhook（`PurchaseFulfiller` 后台 drain，秒级~分钟级；期间后端日志零 `attention`）→ 断言「权益已生效」。截图 `04-checkout-active.png` + `05-billing-active.png`。
通过判据：3/5/6 三态断言全中 + 5 张截图 + `GET /api/v1/commercial/purchase`（带 JWT）依次观测 `awaiting_payment → paid_awaiting_activation → active`（curl 留档 `purchase-state-progression.txt`）。

**幕二（AC2 漏通知查单恢复，API）**：
新租户/新单（或清库重跑）：stub `mark SUCCESS` 但**不推 notify** → 浏览器/`curl` 触发 `GET /api/v1/commercial/orders/:id`（即 `RecoverOrderStatus` 查单）→ 断言订单转 paid、`GET /purchase` → `paid_awaiting_activation` → settle 后 active。留档 `recovery-no-notify.txt`（渠道 Query 日志 + 状态跃迁）。

**幕三（AC2 重复通知不重复激活，API）**：
幕一结束后，对同一 out_trade_no 重推 stub notify 两次 → 两次 WeKnora 200 `{"code":"SUCCESS"}`；DB 计数不变：`commercial_orders` 1、fulfilled 1、outbox fulfill 1、`commercial_payment_attempts` succeeded 1、Lago payments succeeded 恰 1。留档 `duplicate-notify-idempotent.txt`（含 sqlite 查询输出与 Lago API 读数）。

**幕四（AC3 关单-晚成功竞态，API）**：
1. 新租户落微信待付单 → stub mark SUCCESS（在途）→ 调 `POST /api/v1/commercial/purchases`（新 quote，`provider:"alipay"`）触发渠道切换。
2. 断言：答的是**已付旧微信单**（state=paid、provider=wechat）；支付宝 stub `PRECREATE` 日志零调用；微信 stub 日志 `CLOSE → 400 ORDER_PAID` 后 `QUERY → SUCCESS`。
3. `GET /purchase` → `paid_awaiting_activation` → settle → active（资金事实保留、恰好一次履约——「复用同一激活流程」在竞态下成立）。
4. 反向剧本：另一租户落待付单 → stub 保持 NOTPAY → 切换 `provider:"alipay"` → 断言微信 stub `CLOSE → 204`、旧单 `channel_failed` 不可付、新支付宝单 `checkout_url=qr.alipay.com/...` 生成（`CurrentPendingPurchaseOrder` 只见新单）。
5. 晚到成功再补一刀：反向剧本后对已关旧单推 stub notify（防御性）→ 订单仍按 ConfirmPayment 幂等语义处理，无二次履约、无第二 fulfill 事件。
留档 `close-race-paid.txt` / `close-race-switch.txt`（stub 双侧日志 + DB 计数）。

**幕五（AC4 四对象 + 红线核验）**：
1. Lago API（`deploy/lago/.env` 的 key）：`GET /api/v1/subscriptions?external_id=weknora-tenant-<N>-purchase&status[]=active` 恰 1 条 active；gating invoice finalized+succeeded、fees 恰 1 行 subscription_fee 9900；payments succeeded 恰 1；purchase 钱包 granted 9.9 恰 1 笔。留档 `lago-four-objects-wechat.txt`。
2. 产品面：`GET /api/v1/commercial/account` credits 含购买批次（9.9）、features `advanced_models:true`。留档 `account-after-wechat.json`。
3. 红线：Task 4 Step 3 三条 grep + architecture guard（凭据字面量 0、本地强制 active 0）。
通过判据：四对象齐 + 红线 0 命中 + 前四幕全过。

**披露边界（写进 README，不伪造）**：微信腿为本地 RSA/AES 协议 stub（真实商户号/平台证书不可得）——证明「微信协议往返 + 验签 + 同一激活链」，**不构成真实微信钱包付款证据**；对称适用 R-4 对支付宝的披露口径。AC4 的「不以支付宝证据替代」由本票独立微信证据包满足。

---

## 验收标准 → Task → 测试 追踪矩阵

| Issue 验收标准 | Task | 测试/证据 |
|---|---|---|
| AC1 微信真实回调验签、商户、金额和币种校验通过后才形成 Payment Fact | T2（+T1 协议 pin） | `TestWechatCallbackConfirmsOrderAndEmitsFulfillEvent`（验签+商户解析+金额一致→Fact+outbox）、`TestWechatCallbackRejectsTamperedSignature`/`RejectsUnknownSerial`（401 零持久化）、`TestWechatCallbackAmountMismatchRejectedNoFact`（409）、`TestWechatCallbackUnknownAttemptNotFound`；幕一 stub 回调 200 回执 |
| AC2 漏通知通过查单恢复，重复通知不重复激活 | T2/T3/T4 | `TestWechatCallbackDrainsToFulfilledWithFakePlatform`（微信事实驱动同一 #82 链恰一次）、`TestWechatCallbackDuplicateDeliveryIdempotent`、`TestWechatQueryMapsTradeStates`（查单状态映射）；幕二 `recovery-no-notify.txt`、幕三 `duplicate-notify-idempotent.txt` |
| AC3 关单与晚成功竞态保留资金事实并只履约一次 | T3（+T4 幕四） | `TestCloseChannelOrderPaidRaceConfirmsFundFact`、`TestLateSuccessAfterCloseAuditsWithoutSecondFulfillment`、`TestCloseChannelOrderClosesPendingAndFreesSlot`、`TestPurchaseSwitchPaidRaceAnswersPaidOrder`、`TestPurchaseSwitchesChannelByClosingOldOrder`、`TestCloseChannelOrderIndeterminateStaysPending`；幕四 `close-race-*.txt` |
| AC4 微信验收使用真实受控环境，不以支付宝证据替代 | T4 | `docs/plans/issue-72-flow-evidence-83/` 全套微信腿独立证据（五幕 + 四对象 + 截图）；披露边界如实标注；支付宝 stub 仅作切轨腿的新渠道对端 |
| 复用同一激活流程（Issue 标题/正文核心） | T2/T4 | `TestWechatCallbackDrainsToFulfilledWithFakePlatform`（组合级）；幕一 `purchase-state-progression.txt` 三态 + 幕五 Lago 四对象（真 Stripe settle + webhook finalize，零本地强制 active——红线 grep） |

## Spec 行为矩阵覆盖对照（#83 分片）

- **#5 External payment activation**（L218）微信腿：T2 组合测试 + T4 幕一/五真栈。
- **#7 Payment anomalies**（L220）duplicate/late/multiple-success：T2 `Duplicate/DifferentTransaction` + T3 `LateSuccessAfterClose`；partial 归 #84。
- **#20 Fault injection**（L233）：T1 超时 StateUnknown、T3 不定结局不收口、T2/T3 幂等重放。
- **#22 Privacy and secrets**（L235）：T4 红线 grep（sk_test 0、密钥文件不进仓库）。

## Out of scope（#83 明确不做，防与并行票冲突）

- 废弃 pending 单**超时回收**调度与公开 cancel 命令（`CommandKindCancelPurchaseSubscription`）——#84/#92 生命周期票（#82 计划 Task 11 已显式移交；#83 的 `CloseChannelOrder` 即其可复用 seam，Ledger 记移交）。
- `RecoverOrderStatus` 查单见 CLOSED 时的本地收口呈现（当前如实回 pending+原链接；渠道自动关单 UX 归 #84，本票只在切轨路径收口）。
- partial/multiple-success 异常付款完整处置面（#84）；充值 Credits 批次（#85）。
- 前端改动：渠道选择器、三态呈现、`weixin://` scheme 白名单均 #81/#82 已交付，零改动。
- Commercial Platform seam 与 Lago 适配器改动（零改动——`ChannelTransaction` 为 string，微信 transaction_id 直接复用）。

## Self-Review 结论（编写时自检）

1. **Spec 覆盖**：L123（T1/T2）、L124+L218（T2/T4）、L125（红线）、L127（T2/T3）、L165（T1/T3）、L220 #7 部分（T2/T3，partial 显式移交 #84）——全覆盖或显式移交。2. **Step 扫描**：每步一个可核查动作；CloseChannelOrder 算法体是唯一「签名+测试不定」的代码块，其余为测试断言与精确修改点。3. **类型一致**：`CloseChannelOrder(ctx, tenantID uint64, orderID string) (OrderView, error)` 在 T3 定义、purchase 接线与 T4 幕四消费一致；`PaymentAttemptStateClosed`/`MarkAttemptClosed(ctx, orderID)` 全文一致。4. **Review Focus 五行**均已映射 owning task 测试。5. **比例**：计划长度由「真实流程验证方案」与追踪矩阵（任务书强制章节）构成，任务本体保持签名+断言粒度。
