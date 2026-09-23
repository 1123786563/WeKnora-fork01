# [Lago 09] 从 Quote 创建待付款 Invoice 和 incomplete Subscription — 实施计划（Issue #81）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 账单管理员提交购买后，WeKnora 通过冻结 Commercial Platform seam 在 Lago 创建 payment-gated `incomplete` Subscription（伴随 Lago 自动生成的待付款 gating Invoice），把权威订阅面与 Quote 逐项核对一致后才创建渠道支付订单（Channel Payment Order）；产品状态显示「待付款」且 Entitlement 不开放；过期 Quote、并发套餐变更与命令重试不产生重复 Invoice/Subscription。

**Architecture:** 沿用 #77/#78/#79/#80 已冻结的 additive 模式：seam 新增一个命令 `create_purchase_subscription`（带 payment activation gating，与 #80 的免费 Base Plan 标准订阅 `ensure_subscription` 并存）与一个快照 `purchase`（读权威订阅面做匹配校验）。付费购买订阅使用独立的确定性身份 `weknora-tenant-<id>-purchase`，Base Plan 身份不动（待付款期间空间保留 Base 兜底权益，spec「transition to the Base Plan … while preserving existing data」）。服务层新增 `PurchaseService` 协调：Quote 校验 → ensure customer → gated 订阅创建（幂等 read-before-create）→ purchase 快照匹配校验（Plan Version/币种/总额）→ 一致才复用 `OrderService.CreateOrder` 开渠道订单。v1.53.0 上 open 状态 gating Invoice 对全部 API 途径不可见（本计划实证，见「实证契约基线」），因此创建支付请求前的匹配校验数据源是**权威订阅对象**（`plan_code` / `plan_amount_cents` / `plan_amount_currency`，t02 实证可见），行项目等价性由「首期订阅费单行 + 无 pay-in-advance charges」的定价模型切片保证，finalized 后的完整 Invoice 复核留给 #82/#84。

**Tech Stack:** Go 1.x（gin + gorm + httptest stub 契约测试）、TypeScript/React（apps/web + packages/contracts + packages/api-client，tsx --test 单测）、Playwright（真实浏览器验收）、Lago Community v1.53.0（deploy/lago，digest 锁定，:48889）、Stripe TEST（provider 轨道，凭据仅环境变量）。

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`（"Quotes, payments, and fulfillment"、"Public product states"、"Testing seams" 节）；决策依据 `docs/migrations/lago/t02-payment-activation/DECISION.md`（§3 provider 轨道、§5 裁决记录选项 (b)、§7 cross-ticket notes）；ADR-0012 / ADR-0014。

---

## Global Constraints（批准需求原文 + 安全约束，每个 Task 隐含遵守）

引用 `docs/specs/2026-09-20-lago-billing-migration-design.md`：

1. 「Before a Channel Payment Order is created, the Quote and actual Lago Invoice must match in Plan Version, currency, total, and line items. A mismatch aborts the purchase and creates no channel payment request.」（L121）
2. 「Initial subscriptions are pay-in-advance, have no trial, and use a payment activation rule. They remain incomplete until the gating payment succeeds.」（L122）
3. 「A supported Lago external-payment integration records the Payment. Only a Lago Subscription observed as active enables Entitlement and fulfillment.」（L124）
4. 「WeKnora maps provider state into stable product states: awaiting payment, paid awaiting activation, active, …」（L169）；「The public Billing API never exposes Lago credentials, internal URLs, Lago Organization identifiers, or raw status enums.」（L170）
5. 「The initial deployment supports one Billing Entity and CNY only. Monetary values use integer minor units and never pass through binary floating point.」（L105）
6. 「WeKnora-to-Lago commands use a durable Outbox and stable idempotency identity. Timeout and server error are indeterminate outcomes that must be queried before replay.」（L165）——#81 以 seam 命令确定性 Key + 适配器 read-before-create 落实（与 #78/#79/#80 同一先例）
7. 「A UI/API test asserts stable WeKnora product states and never snapshots raw Lago fields.」（L210）
8. 「Lago API credentials live only in server-side secret management. Browser, mobile, Agent context, logs, and generated artifacts cannot access them.」（L175）

用户裁决（2026-09-23）：「T02=选项② 受支持 Provider：付款激活通道采用 Lago 原生 Payment Provider 集成（Stripe TEST 已在 #74 实证；密钥在 ~/.zcode/issue72-stripe.env，凭据纪律：只 source 注入环境变量，任何产出不得含 sk_test 字面量）。#81 的 incomplete Subscription 创建与 #82 的 Payment 录入按 Provider 通道设计。」

安全约束（实现验收条件）：
- S1 服务端发起外部请求仅允许 http/https，请求前校验 host 并拒绝 localhost、环回、私有和保留地址（本计划新增的 Stripe 出站调用必须实现；目标 URL 默认 `https://api.stripe.com`）。
- S2 数据库查询一律参数绑定（gorm where/占位），禁止拼接/format/f-string 组装 SQL。
- S3 凭据只从环境变量读取（`WEKNORA_COMMERCIAL_STRIPE_API_KEY` 等），源码、示例和测试不得写入可用的凭据字面量；测试里的 secret 形状用拼接字面量 defuse（GitHub push protection 教训，waves 文档 L131）。

---

## 实证契约基线（本计划编写会话在 pinned v1.53.0 真实栈 + 源码上实证；执行者不得凭 OpenAPI 假设覆盖）

以下事实全部在 2026-09-23 会话中实证（栈：本机 docker `weknora-lago-api-1`，127.0.0.1:48889，v1.53.0）：

- **F1（实测）** `POST /api/v1/subscriptions` 带 `activation_rules:[{"type":"payment","timeout_hours":0}]`，若 customer 未绑定 payment provider → HTTP 422 `{"error_details":{"customer":["no_linked_payment_provider"]}}`。绑定途径：`POST /api/v1/customers` upsert `billing_configuration: {payment_provider:"stripe", provider_customer_id:"..."}`。
- **F2（实测）** org 未注册 provider 时，customer 绑定 → HTTP 422 `{"error_details":{"base":["payment_provider_not_found"]}}`。provider 注册只能走 GraphQL `AddStripePaymentProvider`（operator JWT，`loginUser` 获取），REST org API key 不够。
- **F3（源码级，容器内 /app/app/models/invoice.rb:100-101）** `VISIBLE_STATUS = {draft:0, finalized:1, voided:2, failed:4, pending:7}`；`INVISIBLE_STATUS = {generating:3, open:5, closed:6, deleted:8}`。gating Invoice 待付款时 status=open ∈ INVISIBLE。
- **F4（源码级，/app/app/queries/invoices_query.rb:122-129）** `with_status` 把调用方显式传入的 status 与 `visible_keys` **求交集**——任何参数组合都无法让列表返回 open invoice。实测 `GET /api/v1/invoices?...&status=open` → 422（枚举不含 open）。
- **F5（源码级）** `GET /api/v1/invoices/{lago_id}`（invoices_controller.rb:46-48 `invoices.visible.find_by`）与 GraphQL `InvoiceResolver`（`invoices.visible.find`）同样只查 visible。**结论：待付款（open）gating Invoice 在 v1.53.0 上对所有 API 途径不可读；「不产生重复 Invoice」的权威核验必须 DB 直查**（`docker exec weknora-lago-db-1 psql ... SELECT count(*) FROM invoices WHERE ...`）。
- **F6（t02 运行时证据）** `GET /api/v1/subscriptions?external_id=X&status[]=incomplete` 可读 incomplete 订阅，对象带 `plan_code`、`plan.amount_cents`、`plan_amount_currency`、`status:"incomplete"`；此时 customer entitlements 404（Entitlement 未开放）。订阅 index 默认 `status=active`，恢复读取必须显式传 `status[]`（DECISION.md §7）。
- **F7（t02-duplicates 运行时证据 + contract_notes）** 同 external_id 重 POST 在 v1.53.0 回 200 但存在**延迟 terminate + renewal invoice** 风险（「callers must enforce their own idempotency and never re-POST registrations」）→ #81 创建路径必须 identity read-before-create，绝不在读到已有订阅后重 POST。
- **F8（t02 源码知识，DECISION.md §7）** `timeout_hours: 0` = 永不自动过期；非零 timeout 由 Lago 每小时 clock（:20）取消 gated 订阅。#81 采用 `timeout_hours: 0`，取消时机由 WeKnora 协调层掌握（quote 过期/异常处理在 #84）。
- **F9（t02 DECISION §3）** gated 订阅创建成功即 `incomplete`，Lago 自动生成 open、无编号、`payment_status: pending` 的 pay-in-advance gating Invoice；provider 收款成功 → invoice finalized + subscription active；失败 → invoice closed + subscription canceled。
- **F10（t02 §7）** plan entitlement attach 端点要 hash 形状（`{"entitlements": {"<code>": {}}}`）；entitlement 序列化按 `code` 键。#79 适配器已按此实现。

## 设计决策记录（本计划定稿，执行者按此实现）

- **D1 购买订阅身份**：`ExternalPurchaseSubscriptionID(tenantID) = ExternalCustomerID(tenantID) + "-purchase"`。付费家族（首购/升级/降级/回落）共用此身份满足 spec L111 的 continuity；Base Plan 订阅（#80，`-sub` 身份）不终止，待付款期间空间保留 Base 兜底权益。投影层 purchase 状态与 Base benefits 并存呈现。
- **D2 匹配数据源（含用户裁决留痕）**：创建渠道支付请求前的匹配校验读 purchase 快照（权威订阅面：PlanCode/AmountFen/Currency，F6）。「actual Lago Invoice」的完整行项目在 open 阶段不可读（F3-F5）→ 行项目等价性由切片保证：购买仅接受「定义中无 charges」的 plan 版本（usage charges 是 pay-in-arrears，不进首期 invoice；publish 载荷里 `PayInAdvance:false`，lago.go:391 已定），gating Invoice 首期仅含一条 subscription fee = `plan_amount_cents`（F9 + t02-activation「totals matched in integer cents」）。
  **用户裁决（2026-09-23，计划审查第 1 轮升级，选项 A 批准）**：spec L121「Quote 与 actual Lago Invoice 在 Plan Version/currency/total/line items 逐项匹配后才创建 Channel Payment Order」中的 **line-item 付款前比对**在 pinned v1.53.0 上不可实现（外部硬约束，源码级实证：invoice.rb:100-101 INVISIBLE_STATUS 含 open:5；invoices_query.rb:122-129 显式 status 与 visible_keys 求交集；invoices_controller.rb:46-48 与 GraphQL InvoiceResolver 均 `.visible` 过滤），非实现选择。批准 #81 按「付款前对权威订阅面硬校验 Plan Version/币种/总额 + 无 charges 切片推导单行订阅费 + 付款时完整复核」实施。**强制条件（缺一不可，逐条落实到本计划）**：(1) t09 DECISION.md 完整记录 spec L121 原文、三处源码实证引用、替代校验链、本升级留痕（Task 12 Step 4 模板）；(2) Ledger 记 Ruling（决定/依据/错误代价——若裁决错误且 line items 付款前存在篡改面，代价是付款前防线弱化，由付款后复核与 #84 异常付款验收兜底）；(3) 「#82/#84 付款时完整 Invoice line-item 复核」是 #81 的显式接口交付（Task 4 Produces：`PurchaseSnapshot.InvoiceFees []InvoiceLineSnapshot`，open 阶段恒空、finalized 后权威填充），后续 Issue 计划 Consumes 必须引用——付款前不比对不得演变为永远不比对；(4) 本偏差仅限 line-item 付款前比对这一条，不外溢为其他 spec 条款的先例。用户同时声明将向 spec owner 呈报正式修订案，#81 无需等待其完成。
- **D3 provider 绑定归属**：`create_purchase_subscription` 适配器内部 ensure customer 的 provider 绑定。需要 Stripe customer id：env `WEKNORA_COMMERCIAL_STRIPE_API_KEY`（sk_test_）已设 → 调 `POST {WEKNORA_COMMERCIAL_STRIPE_API_BASE:-https://api.stripe.com}/v1/customers`（idempotency key = external customer id，S1 host 校验）取 `cus_` id；env `WEKNORA_COMMERCIAL_PROVIDER_CUSTOMER_PREFIX` 已设（dev/test 无 Stripe 时）→ 派生占位 `<prefix>-<external-customer-id>`；都缺 → `ErrPlatformUnconfigured`（fail closed，spec L171 停止新购买）。provider 词汇（stripe/billing_configuration）不越过 seam。
- **D4 产品状态**：Billing API 新增闭合 token `awaiting_payment`（映射 Lago incomplete）。付费 plan 的权益不进 benefits 投影（投影订阅身份仍是 Base 的 `-sub`，#80 不动）→ Entitlement 未开放是结构性结果，测试断言之。
- **D5 过期/并发/重试**：quote 过期与版本冲突复用现有守卫（`Quote.ValidateForUse`、`OpenOrder` 事务内 quote 消费守卫，order.go:217/quote.go:86-97）；并发套餐变更 = purchase 身份读到 incomplete+不同 plan → 闭合冲突错误（先到者胜，后到者重新报价，与旧链路 `ErrSubscriptionVersionConflict` 同语义）；命令重试 = 订阅 identity replay（F7 禁止重 POST）+ quote 已消费时查询并返回既有订单。

## Review Focus（spec 隐含但无任务测试会咬人的输入类；每条注明归属测试）

1. **重复提交/命令重试产生第二张 gating Invoice 或第二个订阅**（F7 延迟 terminate 风险）→ Task 3 stub 测试断言 identity read 后绝不重 POST；Task 7 重试返回既有订单；Task 11/12 真实栈 DB 计数恰 +1。
2. **Stripe/Lago 不可达或 key 缺失时留下半创建状态**（订阅建了但绑定失败 / 绑定失败但返回成功）→ Task 3 fail-closed 分类测试（unconfigured/unreachable/invalid_response 闭合哨兵）。
3. **金额不一致仍拉起渠道支付**（本地 Quote 价格 ≠ Lago plan 价格）→ Task 7 `TestPurchaseAbortsOnInvoiceMismatch` 断言渠道 provider stub 零调用；Task 8 HTTP 409。
4. **并发套餐变更双订阅**（两个不同 quote 同时提交）→ Task 3 conflict 分支 + Task 7 并发测试（版本守卫 + 身份冲突）。
5. **凭据泄漏**（Stripe key / Lago key 进错误串、日志、API 响应）→ Task 3 断言错误串仅闭合哨兵 + 短描述（lago.go:60-68 先例）；Task 12 证据文件扫描无 `sk_test_` 字面量。

## File Structure（新建/修改全景）

```
internal/modules/commercial/purchase_command.go                     [新建] seam additive 类型（D1 身份/命令/payload/snapshot）
internal/modules/commercial/purchase_command_test.go                [新建] payload 校验单测
internal/modules/commercial/commercialplatform/lago_purchase.go     [新建] Lago 适配器：create_purchase_subscription + purchase snapshot + Stripe ensure
internal/modules/commercial/commercialplatform/lago_purchase_test.go[新建] stub 契约测试（httptest）
internal/modules/commercial/commercialplatform/fake.go              [修改] fake 的 purchase 订阅/绑定模型 + 命令/快照实现
internal/modules/commercial/commercialplatform/contract_test.go     [修改] 双适配器共享契约新增 purchase 腿
internal/modules/commercial/commercialplatform/lago_purchase_integration_test.go [新建] //go:build lago_integration 真实栈证据
internal/modules/commercial/service/commercial/purchase.go          [新建] PurchaseService（匹配/幂等/状态）
internal/modules/commercial/service/commercial/purchase_test.go     [新建] sqlite 单测（AC2/AC4）
internal/modules/commercial/service/commercial/order.go             [修改] QuoteView/quoteSnapshot 扩展（AC1 行项目/权益/币种）
internal/modules/commercial/service/commercial/order_test.go        [修改] 新增 quote 冻结用例
internal/modules/commercial/repository/commercial/order.go          [修改] GetOrderByQuote（参数绑定）
internal/handler/commercial.go                                      [修改] Purchase/PurchaseStatus handler + SetPurchaseService + quoteWire 扩展
internal/router/commercial_purchase_route_test.go                      [新建] HTTP 面路由级测试（authAs + RegisterCommercialRoutes 先例）
internal/router/routes_commercial.go                                [修改] POST /commercial/purchases、GET /commercial/purchase
internal/container/container.go                                     [修改] PurchaseService 注册与注入（#80 块内）
packages/contracts/src/commercial.ts                                [修改] PurchaseView/parsePurchaseView + QuoteView 扩展
packages/api-client/src/commercial.ts                               [修改] purchase()/purchaseStatus()
apps/web/src/commercial/CheckoutPage.tsx                            [修改] 展示 Quote 行项目/权益/过期；提交走 purchases
apps/web/src/commercial/BillingPage.tsx                             [修改] 套餐行显示「待付款」
apps/web/src/commercial/*.test.ts(x)                                [修改/新建] web 单测
docs/migrations/lago/t09-quote-invoice/README.md + DECISION.md      [新建] 证据索引 + D2 偏差记录
deploy/lago/evidence/t09-run.txt                                    [新建] 真实栈运行证据
docs/plans/issue-72-ledger-81.md                                    [修改] 执行账本追加
```

---

### Task 1: seam additive 类型（purchase_command.go）

**Files:**
- Create: `internal/modules/commercial/purchase_command.go`
- Test: `internal/modules/commercial/purchase_command_test.go`

**Interfaces:**
- Consumes: `commercial.CommandKind`、`commercial.SnapshotKind`、`commercial.ExternalCustomerID`（platform.go:28/74/159）。
- Produces（后续任务全部依赖）:
```go
const CommandKindCreatePurchaseSubscription CommandKind = "create_purchase_subscription"
func ExternalPurchaseSubscriptionID(tenantID uint64) string // ExternalCustomerID(id) + "-purchase"
type CreatePurchaseSubscriptionPayload struct {
    TenantID uint64
    ExternalCustomerID           string // == ExternalCustomerID(TenantID)
    ExternalPurchaseSubscriptionID string // == ExternalPurchaseSubscriptionID(TenantID)
    PlanCode string
    AmountFen int64 // > 0（付费购买；Base 免费链路走 ensure_subscription）
    Currency string // 闭合 token "CNY"
}
func (p CreatePurchaseSubscriptionPayload) Validate() error
func CreatePurchaseSubscriptionCommandKey(externalPurchaseSubscriptionID, planCode string) string
const SnapshotKindPurchase SnapshotKind = "purchase"
const (
    PurchaseStateAbsent         = "absent"
    PurchaseStateAwaitingPayment = "awaiting_payment" // Lago incomplete → 产品「待付款」
    PurchaseStateActive         = "active"
    PurchaseStateCanceled       = "canceled"
)
type PurchaseSnapshot struct {
    TenantID uint64
    State    string // PurchaseState* 闭合集
    PlanCode string
    AmountFen int64
    Currency string
    InvoiceFees []InvoiceLineSnapshot // open 阶段恒空（F3-F5）；finalized 后权威可读——#82/#84 line-item 复核接口（裁决条件 3）
    CheckedAt time.Time
}
type InvoiceLineSnapshot struct {
    Kind      string // 闭合 "subscription_fee"
    Name      string
    AmountFen int64
}
// Snapshot 冻结文件只加一个 additive 字段（#78 Account / #80 Benefits 先例）：
// platform.go 的 Snapshot struct 增加 `Purchase *PurchaseSnapshot`
```

- [ ] **Step 1: 写失败测试**（`purchase_command_test.go`）

```go
package commercial

import (
	"strings"
	"testing"
)

func TestCreatePurchaseSubscriptionPayloadValidate(t *testing.T) {
	base := func() CreatePurchaseSubscriptionPayload {
		return CreatePurchaseSubscriptionPayload{
			TenantID: 42, ExternalCustomerID: ExternalCustomerID(42),
			ExternalPurchaseSubscriptionID: ExternalPurchaseSubscriptionID(42),
			PlanCode: "weknora-pro-v1", AmountFen: 9900, Currency: CurrencyCNY,
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	cases := map[string]func(*CreatePurchaseSubscriptionPayload){
		"zero tenant":            func(p *CreatePurchaseSubscriptionPayload) { p.TenantID = 0 },
		"customer id mismatch":   func(p *CreatePurchaseSubscriptionPayload) { p.ExternalCustomerID = "weknora-tenant-43" },
		"purchase id mismatch":   func(p *CreatePurchaseSubscriptionPayload) { p.ExternalPurchaseSubscriptionID = "weknora-tenant-43-purchase" },
		"empty plan code":        func(p *CreatePurchaseSubscriptionPayload) { p.PlanCode = "" },
		"zero amount":            func(p *CreatePurchaseSubscriptionPayload) { p.AmountFen = 0 },
		"negative amount":        func(p *CreatePurchaseSubscriptionPayload) { p.AmountFen = -1 },
		"non-cny currency":       func(p *CreatePurchaseSubscriptionPayload) { p.Currency = "USD" },
	}
	for name, mutate := range cases {
		p := base()
		mutate(&p)
		if err := p.Validate(); err == nil {
			t.Errorf("%s: expected rejection, got nil", name)
		}
	}
}

func TestExternalPurchaseSubscriptionIDDeterministic(t *testing.T) {
	if got := ExternalPurchaseSubscriptionID(7); got != "weknora-tenant-7-purchase" {
		t.Fatalf("identity = %q", got)
	}
	if ExternalPurchaseSubscriptionID(7) != ExternalPurchaseSubscriptionID(7) {
		t.Fatal("identity must be a pure function of tenant")
	}
}

func TestCreatePurchaseSubscriptionCommandKeyForm(t *testing.T) {
	got := CreatePurchaseSubscriptionCommandKey("weknora-tenant-7-purchase", "weknora-pro-v1")
	if got != "create_purchase_subscription:weknora-tenant-7-purchase:weknora-pro-v1" {
		t.Fatalf("key = %q", got)
	}
	if !strings.HasPrefix(got, string(CommandKindCreatePurchaseSubscription)) {
		t.Fatal("key must be prefixed with the command kind")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/ -run 'TestCreatePurchase|TestExternalPurchase' -count=1`
Expected: FAIL（`undefined: CreatePurchaseSubscriptionPayload` 等编译错误）

- [ ] **Step 3: 最小实现**（`purchase_command.go`）——类型与常量照 Interfaces 块逐字实现；`Validate` 报错文案模仿 subscription_command.go:64-78 风格（`invalid create_purchase payload: ...`）；`platform.go` 的 `Snapshot` struct 只追加 `Purchase *PurchaseSnapshot` 字段（带注释引用 ADR-0014 additive 规则），三个方法签名不动。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/modules/commercial/ -count=1`
Expected: PASS（全包，含既有 7 个文件测试不回归）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/commercial/purchase_command.go internal/modules/commercial/purchase_command_test.go internal/modules/commercial/platform.go
git commit -m "feat(commercial): #81 seam additive types for purchase subscription command and snapshot"
```

---

### Task 2: FakeAdapter 实现 purchase 命令与快照

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/fake.go`
- Test: `internal/modules/commercial/commercialplatform/fake.go` 同包既有 `fake_test.go`（新增用例；如无独立 fake 测试文件则并入 `lago_test.go` 旁新建 `fake_purchase_test.go`）

**Interfaces:**
- Consumes: Task 1 全部类型；fake 的既有 mutex/store 模式（fake.go:82-123）。
- Produces:
```go
// FakeAdapter 上的可观察状态（测试断言用）
type FakePurchaseSubscription struct {
    ExternalID       string
    ExternalCustomer string
    PlanCode         string
    Status           string // "incomplete"|"active"|"canceled" —— fake 建即 incomplete
}
func (f *FakeAdapter) PurchaseSubscriptions() []FakePurchaseSubscription
func (f *FakeAdapter) ProviderBindings() map[string]string // external customer id -> provider customer id
func (f *FakeAdapter) ActivatePurchase(extPurchaseSubscriptionID string) // 测试推进到 active（#82 前的手动推进钩子）
```

- [ ] **Step 1: 写失败测试**（`fake_purchase_test.go`）

```go
package commercialplatform

import (
	"context"
	"errors"
	"testing"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
)

func newPurchaseCommand(tenant uint64, planCode string, amount int64) commercial.Command {
	return commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key:  commercial.CreatePurchaseSubscriptionCommandKey(commercial.ExternalPurchaseSubscriptionID(tenant), planCode),
		Actor: "test", Reason: "purchase",
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
			PlanCode: planCode, AmountFen: amount, Currency: commercial.CurrencyCNY,
		},
	}
}

func TestFakeCreatePurchaseSubscriptionIdempotent(t *testing.T) {
	f := NewFakeAdapter()
	cmd := newPurchaseCommand(11, "weknora-pro-v1", 9900)
	first, err := f.SubmitCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if first.ExternalID != commercial.ExternalPurchaseSubscriptionID(11) {
		t.Fatalf("receipt identity = %q", first.ExternalID)
	}
	if _, err := f.SubmitCommand(context.Background(), cmd); err != nil {
		t.Fatalf("replay create: %v", err)
	}
	subs := f.PurchaseSubscriptions()
	if len(subs) != 1 {
		t.Fatalf("replay must not create a second subscription, got %d", len(subs))
	}
	if subs[0].Status != "incomplete" {
		t.Fatalf("fresh purchase subscription must be incomplete, got %q", subs[0].Status)
	}
}

func TestFakeCreatePurchaseConflictOnDifferentPlan(t *testing.T) {
	f := NewFakeAdapter()
	if _, err := f.SubmitCommand(context.Background(), newPurchaseCommand(12, "weknora-pro-v1", 9900)); err != nil {
		t.Fatal(err)
	}
	_, err := f.SubmitCommand(context.Background(), newPurchaseCommand(12, "weknora-max-v1", 19900))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("concurrent plan change must be a definitive conflict, got %v", err)
	}
	if n := len(f.PurchaseSubscriptions()); n != 1 {
		t.Fatalf("conflict must not create a second subscription, got %d", n)
	}
}

func TestFakePurchaseSnapshotStates(t *testing.T) {
	f := NewFakeAdapter()
	snap, err := f.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 13})
	if err != nil || snap.Purchase == nil || snap.Purchase.State != commercial.PurchaseStateAbsent {
		t.Fatalf("untouched tenant must answer absent, got %+v err=%v", snap.Purchase, err)
	}
	if _, err := f.SubmitCommand(context.Background(), newPurchaseCommand(13, "weknora-pro-v1", 9900)); err != nil {
		t.Fatal(err)
	}
	snap, err = f.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 13})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Purchase.State != commercial.PurchaseStateAwaitingPayment ||
		snap.Purchase.PlanCode != "weknora-pro-v1" ||
		snap.Purchase.AmountFen != 9900 ||
		snap.Purchase.Currency != commercial.CurrencyCNY {
		t.Fatalf("awaiting-payment snapshot mismatch: %+v", snap.Purchase)
	}
	f.ActivatePurchase(commercial.ExternalPurchaseSubscriptionID(13))
	snap, _ = f.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 13})
	if snap.Purchase.State != commercial.PurchaseStateActive {
		t.Fatalf("activated purchase must answer active, got %+v", snap.Purchase)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/commercialplatform/ -run TestFakePurchase -count=1`
Expected: FAIL（`f.SubmitCommand` 返回 `ErrPlatformUnsupported` / 方法未定义）

- [ ] **Step 3: 实现**——`fake.go` 增加 `purchaseSubs map[string]fakePurchase`（external id → 记录，含 PlanCode/AmountFen/Currency/Status，建即 `"incomplete"`）、`providerBindings map[string]string`；`SubmitCommand` switch 加 `CommandKindCreatePurchaseSubscription` 分支：payload 校验 → 存在且同 PlanCode → 回执；存在且不同 PlanCode → `ErrPlatformInvalidResponse` 冲突；不存在 → 写入 incomplete + 回执；`ReadSnapshot` 加 `SnapshotKindPurchase` → 组装 `PurchaseSnapshot`（状态映射 absent/incomplete→awaiting_payment/active→active/canceled→canceled）。`PurchaseSubscriptions()/ProviderBindings()/ActivatePurchase()` 观察器按既有 fake getter 模式（锁内拷贝）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/modules/commercial/commercialplatform/ -run TestFakePurchase -count=1 && go test ./internal/modules/commercial/commercialplatform/ -count=1`
Expected: PASS（新用例 + 既有套件不回归）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/commercial/commercialplatform/fake.go internal/modules/commercial/commercialplatform/fake_purchase_test.go
git commit -m "feat(commercialplatform): #81 fake adapter purchase command and snapshot"
```

---

### Task 3: Lago 适配器 create_purchase_subscription（含 provider 绑定 ensure 与 Stripe 出站）

**Files:**
- Create: `internal/modules/commercial/commercialplatform/lago_purchase.go`
- Create: `internal/modules/commercial/commercialplatform/lago_purchase_test.go`
- Modify: `internal/modules/commercial/commercialplatform/config.go`（新增 Stripe env 引用）
- Modify: `internal/modules/commercial/commercialplatform/lago.go`（`SubmitCommand` switch 加一个 case 分发；其余不动）

**Interfaces:**
- Consumes: Task 1 类型；`LagoAdapter.do/classifySubscriptionStatus/readSubscriptionByIdentity`（lago.go:531/711/641）；`subscriptionIndexStatuses`（lago.go:579）。
- Produces:
```go
// config.go 新增 env 常量与字段
const (
    EnvStripeAPIKey    = "WEKNORA_COMMERCIAL_STRIPE_API_KEY"     // sk_test_...（dev/test；生产走密钥服务同名的注入）
    EnvStripeAPIBase   = "WEKNORA_COMMERCIAL_STRIPE_API_BASE"    // 默认 https://api.stripe.com
    EnvProviderCustomerPrefix = "WEKNORA_COMMERCIAL_PROVIDER_CUSTOMER_PREFIX" // 无 Stripe 的 dev 栈占位绑定
)
// Config 增加 StripeAPIKey/StripeAPIBase/ProviderCustomerPrefix string 字段（ConfigFromEnv 读 env）
// lago_purchase.go
func (a *LagoAdapter) createPurchaseSubscription(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error)
func validateOutboundHost(rawURL string) error // S1：scheme ∈ {http,https}；host 拒绝 localhost/127.0.0.0-8/8、::1、10/8、172.16/12、192.168/16、169.254/16、fc00::/7、0.0.0.0/8 及字面 localhost
```

- [ ] **Step 1: 写失败测试**（`lago_purchase_test.go`，subscriptionsStub 模式扩展；给出核心用例全文。stub 要点：(1) customers 端点同时注册 `/api/v1/customers`（无尾斜杠，集合 POST——`createCustomer` 实际打的路径，lago.go:236）与 `/api/v1/customers/`（单查 GET 子树）两个 pattern，Go ServeMux 子树模式不匹配无尾斜杠路径；(2) 订阅 POST 的副作用（写入 s.subs）**总是发生**，`createNext` 只脚本化响应状态——模拟「对象已创建但响应 422」的竞态（t02 duplicates 语义）；(3) index 记录 RawQuery 供 Task 4 断言 `status[]`；无 `status[]` 时只返回 active（模拟 v1.53.0 默认过滤陷阱 F6）。复用既有 `respond`/`stubSubscription`/`subscriptionsJSON`（lago_subscription_test.go:26-49）。）

```go
package commercialplatform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
)

// purchaseStub 是带 provider 绑定语义的 mini authority：customers 集合 POST
// （billing_configuration upsert）与单查 GET、subscriptions POST（记录 body 与
// activation_rules，副作用总是发生）与 identity index（external_id + status[] 过滤，
// 记录 RawQuery）。createNext 脚本化响应状态码（422 分支）。
type purchaseStub struct {
	mu          sync.Mutex
	requests    []stubReq
	rawQueries  []string          // subscriptions index 的 RawQuery（status[] 断言面）
	customer    map[string]map[string]any // external_id -> customer body
	subs        []stubSubscription
	subBodies   []map[string]any   // 每次订阅 POST 的原始 body
	createNext  []int              // 脚本化状态；空则 200
}

func newPurchaseStub() *purchaseStub {
	s := &purchaseStub{customer: map[string]map[string]any{}}
	customers := func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		blob, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		ext := strings.TrimPrefix(r.URL.Path, "/api/v1/customers/")
		switch {
		case r.Method == http.MethodGet && ext != "": // GET /api/v1/customers/{id}
			c, ok := s.customer[ext]
			if !ok {
				respond(w, r, &s.requests, &s.mu, http.StatusNotFound, "{}", nil)
				return
			}
			b, _ := json.Marshal(map[string]any{"customer": c})
			respond(w, r, &s.requests, &s.mu, http.StatusOK, string(b), nil)
		case r.Method == http.MethodPost && ext == "": // POST /api/v1/customers（无尾斜杠）
			var parsed map[string]any
			_ = json.Unmarshal(blob, &parsed)
			if existing, ok := s.customer[ext]; ok { // upsert：合并 billing_configuration
				if bc, ok := parsed["billing_configuration"]; ok {
					existing["billing_configuration"] = bc
				}
				parsed = existing
			}
			s.customer[ext] = parsed
			b, _ := json.Marshal(map[string]any{"customer": parsed})
			respond(w, r, &s.requests, &s.mu, http.StatusOK, string(b), blob)
		default:
			http.NotFound(w, r)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/customers", customers)  // 集合端点（精确匹配，无尾斜杠）
	mux.HandleFunc("/api/v1/customers/", customers) // 单查子树
	mux.HandleFunc("/api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		blob, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		s.mu.Lock()
		defer s.mu.Unlock()
		switch r.Method {
		case http.MethodPost:
			var body map[string]any
			_ = json.Unmarshal(blob, &body)
			s.subBodies = append(s.subBodies, body)
			status := http.StatusOK
			if len(s.createNext) > 0 {
				status = s.createNext[0]
				s.createNext = s.createNext[1:]
			}
			// 副作用总是发生：脚本化 422 模拟「已创建但响应失败」的竞态。
			if sub, ok := body["subscription"].(map[string]any); ok {
				if id, _ := sub["external_id"].(string); id != "" {
					code, _ := sub["plan_code"].(string)
					cust, _ := sub["external_customer_id"].(string)
					s.subs = append(s.subs, stubSubscription{
						ExternalID: id, ExternalCustomer: cust, PlanCode: code, Status: "incomplete",
					})
				}
			}
			respond(w, r, &s.requests, &s.mu, status,
				subscriptionsJSON(append([]stubSubscription(nil), s.subs...)), blob)
		case http.MethodGet: // identity index：模拟 v1.53.0 默认 status=active 的过滤
			s.rawQueries = append(s.rawQueries, r.URL.RawQuery)
			q := r.URL.Query()
			want := q.Get("external_id")
			statuses := q["status[]"]
			out := []stubSubscription{}
			for _, sub := range s.subs {
				if sub.ExternalID != want {
					continue
				}
				if len(statuses) == 0 && sub.Status != "active" {
					continue // 默认过滤：不带 status[] 看不到 incomplete（F6 陷阱）
				}
				matched := len(statuses) == 0
				for _, st := range statuses {
					if st == sub.Status {
						matched = true
					}
				}
				if matched {
					out = append(out, sub)
				}
			}
			respond(w, r, &s.requests, &s.mu, http.StatusOK, subscriptionsJSON(out), nil)
		default:
			http.NotFound(w, r)
		}
	})
	return s
}

func (s *purchaseStub) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return srv
}

// ServeHTTP 让 purchaseStub 本身成为 handler（mux 逻辑挂 ServeHTTP 或包一层均可）。
func (s *purchaseStub) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.serve(w, r) }

func (s *purchaseStub) countSubscriptionPosts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, req := range s.requests {
		if req.Method == http.MethodPost && strings.HasSuffix(req.Path, "/api/v1/subscriptions") {
			n++
		}
	}
	return n
}

// purchaseAdapterWithPrefix 绑定走占位前缀（不依赖 Stripe env）。
func purchaseAdapterWithPrefix(t *testing.T, srv *httptest.Server) *LagoAdapter {
	t.Helper()
	return NewLagoAdapter(Config{
		Provider: ProviderLago, BaseURL: srv.URL, APIKey: "test-key", Release: "v1.53.0",
		ProviderCustomerPrefix: "cus-dev",
	})
}

func purchaseCmd(tenant uint64, planCode string, amount int64) commercial.Command {
	return commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(commercial.ExternalPurchaseSubscriptionID(tenant), planCode),
		Actor: "test", Reason: "purchase",
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
			PlanCode: planCode, AmountFen: amount, Currency: commercial.CurrencyCNY,
		},
	}
}

func TestLagoCreatePurchaseBindsProviderThenCreatesGated(t *testing.T) {
	stub := newPurchaseStub()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(21, "weknora-pro-v1", 9900)); err != nil {
		t.Fatalf("create: %v", err)
	}
	// 断言 1：customer 被 upsert 了 billing_configuration
	stub.mu.Lock()
	cust := stub.customer[commercial.ExternalCustomerID(21)]
	stub.mu.Unlock()
	if cust == nil {
		t.Fatal("customer must be present")
	}
	bc, _ := cust["billing_configuration"].(map[string]any)
	if bc == nil || bc["provider_customer_id"] != "cus-dev-"+commercial.ExternalCustomerID(21) {
		t.Fatalf("provider binding missing/wrong: %+v", bc)
	}
	// 断言 2：订阅 POST body 带 activation_rules payment/timeout 0（F1/F8）
	stub.mu.Lock()
	var lastBody map[string]any
	if len(stub.subBodies) > 0 {
		lastBody = stub.subBodies[len(stub.subBodies)-1]
	}
	stub.mu.Unlock()
	sub, _ := lastBody["subscription"].(map[string]any)
	rules, _ := sub["activation_rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("activation_rules must carry exactly one payment rule, body=%v", sub)
	}
	rule, _ := rules[0].(map[string]any)
	if rule["type"] != "payment" || fmt.Sprint(rule["timeout_hours"]) != "0" {
		t.Fatalf("rule = %+v, want payment/timeout_hours 0", rule)
	}
}

func TestLagoCreatePurchaseReplayNeverReposts(t *testing.T) {
	stub := newPurchaseStub()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	cmd := purchaseCmd(22, "weknora-pro-v1", 9900)
	if _, err := a.SubmitCommand(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SubmitCommand(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	if n := stub.countSubscriptionPosts(); n != 1 {
		t.Fatalf("replay must NEVER re-POST the subscription (F7 deferred-terminate risk), posts=%d", n)
	}
}

func TestLagoCreatePurchase422ResolvedByIdentityReread(t *testing.T) {
	stub := newPurchaseStub()
	stub.mu.Lock()
	// 首个 POST 响应 422，但副作用已发生（stub 总是记录）——模拟竞态创建成功。
	stub.createNext = []int{http.StatusUnprocessableEntity}
	stub.mu.Unlock()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(23, "weknora-pro-v1", 9900)); err != nil {
		t.Fatalf("422 must resolve by identity re-read: %v", err)
	}
	if n := stub.countSubscriptionPosts(); n != 1 {
		t.Fatalf("422 path must not issue a second create, posts=%d", n)
	}
}

func TestLagoCreatePurchaseDifferentPlanIsConflict(t *testing.T) {
	stub := newPurchaseStub()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(24, "weknora-pro-v1", 9900)); err != nil {
		t.Fatal(err)
	}
	_, err := a.SubmitCommand(context.Background(), purchaseCmd(24, "weknora-max-v1", 19900))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("concurrent plan change must be a definitive conflict, got %v", err)
	}
	if n := stub.countSubscriptionPosts(); n != 1 {
		t.Fatalf("conflict must not create a second subscription, posts=%d", n)
	}
}

func TestLagoCreatePurchaseFailsClosedWithoutBindingSource(t *testing.T) {
	stub := newPurchaseStub()
	srv := stub.server(t)
	a := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: srv.URL, APIKey: "k", Release: "v1.53.0"})
	_, err := a.SubmitCommand(context.Background(), purchaseCmd(25, "weknora-pro-v1", 9900))
	if !errors.Is(err, commercial.ErrPlatformUnconfigured) {
		t.Fatalf("no stripe key and no prefix must fail closed unconfigured, got %v", err)
	}
}

func TestValidateOutboundHost(t *testing.T) {
	for _, ok := range []string{"https://api.stripe.com", "http://api.stripe.com/v1"} {
		if err := validateOutboundHost(ok); err != nil {
			t.Errorf("%s must pass: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"https://localhost", "https://127.0.0.1", "https://127.1.2.3", "https://[::1]",
		"https://10.0.0.5", "https://172.16.0.1", "https://192.168.1.4", "https://169.254.1.1",
		"ftp://api.stripe.com", "https://0.0.0.0",
	} {
		if err := validateOutboundHost(bad); err == nil {
			t.Errorf("%s must be rejected", bad)
		}
	}
}
```
（import 里补 `"encoding/json"`；`ServeHTTP` 委托的 `serve` 即上文 mux 组装逻辑——实现时把 mux 组装放进一个 `serve(w,r)` 方法并由 `ServeHTTP` 调用，或直接 `httptest.NewServer(mux)` 结构等价均可，保持单一路径即可。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/commercialplatform/ -run 'TestLagoCreatePurchase|TestValidateOutboundHost' -count=1`
Expected: FAIL（`createPurchaseSubscription`/`validateOutboundHost`/`ProviderCustomerPrefix` 未定义）

- [ ] **Step 3: 实现**（`lago_purchase.go` + `config.go` + `lago.go` 一个 case）：

算法（fail-closed 分类与 lago.go:60-68 先例一致，错误串零 URL/状态/凭据）：
```
createPurchaseSubscription:
  configured() → payload.Validate() → ctx timeout（subscriptionRequestTimeout+10s，绑定+创建+可能的 Stripe 往返）
  1 ensureProviderBinding(ctx, payload.ExternalCustomerID):
      GET /api/v1/customers/{ext} → 200 且 body.billing_configuration.provider_customer_id 非空 → 返回
      否则计算 providerCustomerID：
        cfg.StripeAPIKey != "" → validateOutboundHost(cfg.StripeAPIBase 默认 https://api.stripe.com)
          → POST {base}/v1/customers  (Authorization: Basic base64(key+":")，body {"metadata":{"weknora_customer":ext}}，
             header Idempotency-Key: ext —— Stripe 幂等)
          → 2xx 解析 {"id":"cus_..."}；非 2xx/传输失败 → wrap ErrPlatformUnreachable/InvalidResponse（创建未发生，干净失败）
        cfg.ProviderCustomerPrefix != "" → "<prefix>-<ext>"
        都缺 → ErrPlatformUnconfigured
      POST /api/v1/customers {"customer":{"external_id":ext,"name":ext,"billing_configuration":{
        "payment_provider":"stripe","provider_customer_id":providerCustomerID}}}
      → 200/201 成功；422 payment_provider_not_found → ErrPlatformUnconfigured（org 未注册 provider，F2）
      → 5xx unreachable；其他 4xx invalid_response
  2 readSubscriptionByIdentity(ctx, payload.ExternalPurchaseSubscriptionID)（复用，显式 status 集 F6）
      found + 同 PlanCode → 回执（replay；绝不重 POST —— F7）
      found + 不同 PlanCode → ErrPlatformInvalidResponse "purchase plan conflict"（AC4 并发变更）
  3 absent → POST /api/v1/subscriptions
      body: {"subscription":{"external_customer_id","external_id","plan_code","name":"WeKnora Space <id> Purchase",
             "activation_rules":[{"type":"payment","timeout_hours":0}]}}
      2xx → 回执{Key, ExternalID: ExternalPurchaseSubscriptionID}
      422 → identity re-read（同 ensureSubscription lago.go:623-633 模式）：found+同 plan → 回执；否则 invalid_response
      5xx → unreachable；其他 4xx → invalid_response
```
`lago.go` 的 `SubmitCommand` switch 加 `case commercial.CommandKindCreatePurchaseSubscription: return a.createPurchaseSubscription(ctx, cmd)`。
`config.go`：三个 env 常量 + Config 字段 + ConfigFromEnv 读取（默认 StripeAPIBase 为空时实现层取 `https://api.stripe.com`）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1`
Expected: PASS（新用例 + 既有 73+ 测试不回归）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/commercial/commercialplatform/lago_purchase.go internal/modules/commercial/commercialplatform/lago_purchase_test.go internal/modules/commercial/commercialplatform/config.go internal/modules/commercial/commercialplatform/lago.go
git commit -m "feat(commercialplatform): #81 lago adapter payment-gated purchase subscription create with provider binding ensure"
```

---

### Task 4: Lago 适配器 purchase 快照

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/lago_purchase.go`
- Test: `internal/modules/commercial/commercialplatform/lago_purchase_test.go`（追加用例）

**Interfaces:**
- Consumes: `readSubscriptionByIdentity`（lago.go:641）。
- Produces: `LagoAdapter.ReadSnapshot` 支持 `SnapshotKindPurchase`；`lagoSubscription` 结构如需 `PlanAmountCents json.Number` 与 `PlanAmountCurrency string` 字段则在 lago.go:567-573 additive 追加（index 响应含 plan_amount_cents/plan_amount_currency，t02-duplicates 证据）。
- Produces（**#82/#84 必须消费的显式接口交付——用户裁决选项 A 强制条件 3**）：`PurchaseSnapshot` 携带 `InvoiceFees []InvoiceLineSnapshot` 字段，`type InvoiceLineSnapshot struct { Kind, Name string; AmountFen int64 }`。open（待付款）阶段恒为空（F3-F5：API 不可见，不得伪造）；finalized 后（付款时刻）由适配器从可见 Invoice 的 fees 填充——「付款前面校验 + 付款后全量复核」双段防线的后段接口在本 Task 定型，#82（Payment 录入/激活）与 #84（异常付款）的验收直接调用它做 line-item 完整复核，后续 Issue 计划的 Consumes 必须引用本条。fake 同步实现（`FakeAdapter` 增加 `SetPurchaseInvoiceFees(extPurchaseSubscriptionID string, fees []InvoiceLineSnapshot)` 注入器）。

- [ ] **Step 1: 写失败测试**（追加到 `lago_purchase_test.go`）

```go
func TestLagoPurchaseSnapshotMapsClosedStates(t *testing.T) {
	stub := newPurchaseStub()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	// 未创建：absent
	snap, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 26})
	if err != nil || snap.Purchase.State != commercial.PurchaseStateAbsent {
		t.Fatalf("absent expected, got %+v err=%v", snap.Purchase, err)
	}
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(26, "weknora-pro-v1", 9900)); err != nil {
		t.Fatal(err)
	}
	snap, err = a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 26})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Purchase.State != commercial.PurchaseStateAwaitingPayment {
		t.Fatalf("incomplete must map to awaiting_payment, got %+v", snap.Purchase)
	}
	if len(snap.Purchase.InvoiceFees) != 0 {
		t.Fatalf("open-stage invoice fees must be EMPTY (API-invisible, never fabricated), got %+v", snap.Purchase.InvoiceFees)
	}
	// 订阅 index 请求必须显式携带 status[]（F6 默认 active 陷阱——stub 无 status[] 只返回 active，
	// 结果正确 + RawQuery 含 status[]=incomplete 双重证明）
	stub.mu.Lock()
	raw := ""
	if len(stub.rawQueries) > 0 {
		raw = stub.rawQueries[len(stub.rawQueries)-1]
	}
	stub.mu.Unlock()
	if !strings.Contains(raw, "status%5B%5D=incomplete") && !strings.Contains(raw, "status[]=incomplete") {
		t.Fatalf("subscription index must pass explicit status[] (F6), raw query = %q", raw)
	}
	// stub 推进到 active（真实环境由 provider 收款驱动，F9）
	stub.mu.Lock()
	for i := range stub.subs {
		if stub.subs[i].ExternalID == commercial.ExternalPurchaseSubscriptionID(26) {
			stub.subs[i].Status = "active"
		}
	}
	stub.mu.Unlock()
	snap, err = a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 26})
	if err != nil || snap.Purchase.State != commercial.PurchaseStateActive {
		t.Fatalf("active expected, got %+v err=%v", snap.Purchase, err)
	}
}
```
注 1：`stubSubscription`/index 响应需携带 `plan_amount_cents`/`plan_amount_currency`（扩展 `subscriptionsJSON` 或新增 purchase 专用 JSON 组装）；快照的 AmountFen/Currency 从这两个字段解析（`json.Number` → int64，拒绝非数字 → invalid_response）。
注 2：`InvoiceLineSnapshot` 与 `PurchaseSnapshot.InvoiceFees` 在本 Task 一并加入 Task 1 的类型（若 Task 1 已提交，则在本 Task 的文件里 additive 追加并同步 fake）；单测覆盖「open 阶段恒空」，finalized 填充的适配器读取面（`GET /api/v1/invoices?external_customer_id=` 列表此时可见 finalized）留 `readPurchaseInvoiceFees` 私有方法骨架 + fake 注入器，#82 首个消费者补全真实断言。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/commercialplatform/ -run TestLagoPurchaseSnapshot -count=1`
Expected: FAIL（`ReadSnapshot` 对 purchase kind 返回 `ErrPlatformUnsupported`）

- [ ] **Step 3: 实现**——`ReadSnapshot` switch 加 `case commercial.SnapshotKindPurchase: return a.readPurchaseSnapshot(ctx, query.TenantID)`；实现：identity read（显式 status 集）→ 状态映射（absent / incomplete→awaiting_payment / active→active / canceled|terminated→canceled，未知状态→invalid_response fail closed）→ PlanCode/AmountFen/Currency 取自订阅对象。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/commercial/commercialplatform/lago_purchase.go internal/modules/commercial/commercialplatform/lago_purchase_test.go
git commit -m "feat(commercialplatform): #81 lago purchase snapshot with closed product states"
```

---

### Task 5: 双适配器共享契约腿（contract_test.go）

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/contract_test.go`
- Modify: `internal/modules/commercial/commercialplatform/lago_test.go`（Lago stub 契约入口把 purchase 腿纳入；找到现有 `TestLagoAdapterContract` 类入口按同法追加）

**Interfaces:**
- Consumes: Task 2/3/4 双适配器实现。
- Produces: `runPurchaseContract(t *testing.T, name string, p commercial.CommercialPlatform, creates func() int)`（fake 与 stub 各自传入 creates 计数器）。

- [ ] **Step 1: 写失败测试**（contract_test.go 追加）

```go
// runPurchaseContract 是 #81 的共享契约腿：创建幂等（同 Key 重放绝不第二次 create）、
// 状态闭合（absent → awaiting_payment）、并发换 plan 冲突。creates() 报告适配器实际发出的
// 外部 create 数（fake: 记录命令；Lago stub: subscriptions POST 计数）。
func runPurchaseContract(t *testing.T, name string, p commercial.CommercialPlatform, creates func() int) {
	t.Helper()
	tenant := uint64(910)
	cmd := commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(commercial.ExternalPurchaseSubscriptionID(tenant), "weknora-contract-v1"),
		Actor: "contract", Reason: "shared purchase contract",
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
			PlanCode: "weknora-contract-v1", AmountFen: 4200, Currency: commercial.CurrencyCNY,
		},
	}
	t.Run(name+"/purchase create is idempotent by identity", func(t *testing.T) {
		first, err := p.SubmitCommand(context.Background(), cmd)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if first.ExternalID != commercial.ExternalPurchaseSubscriptionID(tenant) {
			t.Fatalf("receipt identity = %q", first.ExternalID)
		}
		if _, err := p.SubmitCommand(context.Background(), cmd); err != nil {
			t.Fatalf("replay: %v", err)
		}
		if n := creates(); n != 1 {
			t.Fatalf("replay must not create a second external object, creates=%d", n)
		}
	})
	t.Run(name+"/purchase snapshot answers awaiting_payment", func(t *testing.T) {
		snap, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: tenant})
		if err != nil {
			t.Fatal(err)
		}
		if snap.Purchase == nil || snap.Purchase.State != commercial.PurchaseStateAwaitingPayment ||
			snap.Purchase.PlanCode != "weknora-contract-v1" ||
			snap.Purchase.AmountFen != 4200 || snap.Purchase.Currency != commercial.CurrencyCNY {
			t.Fatalf("snapshot mismatch: %+v", snap.Purchase)
		}
	})
	t.Run(name+"/purchase different plan is definitive conflict", func(t *testing.T) {
		other := cmd
		other.Key = commercial.CreatePurchaseSubscriptionCommandKey(
			commercial.ExternalPurchaseSubscriptionID(tenant), "weknora-other-v1")
		other.Payload = commercial.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
			PlanCode: "weknora-other-v1", AmountFen: 9900, Currency: commercial.CurrencyCNY,
		}
		if _, err := p.SubmitCommand(context.Background(), other); !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
			t.Fatalf("conflict expected, got %v", err)
		}
		if n := creates(); n != 1 {
			t.Fatalf("conflict must not create, creates=%d", n)
		}
	})
}
```
并把 `runPurchaseContract` 接入 fake 与 Lago stub 两个既有契约入口（`TestFakeAdapterContract`/`TestLagoAdapterContract` 所在处，按 `runPublishContract` 的接入方式）。

- [ ] **Step 2: 跑测试确认失败→实现→通过**

Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1`
Expected: 先 FAIL（入口未接入），实现后 PASS。

- [ ] **Step 3: Commit**

```bash
git add internal/modules/commercial/commercialplatform/contract_test.go internal/modules/commercial/commercialplatform/lago_test.go
git commit -m "test(commercialplatform): #81 shared purchase contract leg across fake and lago adapters"
```

---

### Task 6: Quote 冻结扩展（AC1：版本/金额/权益/行项目/过期）

**Files:**
- Modify: `internal/modules/commercial/service/commercial/order.go:44-49`（quoteSnapshot）与 `:129-188`（QuoteView/CreateQuote）
- Test: `internal/modules/commercial/service/commercial/order_test.go`（追加）

**Interfaces:**
- Consumes: `domain.PlanVersion`（含 Features map、Charges）；`CatalogStore.CreateQuote`。
- Produces:
```go
type QuoteLineItem struct {
    Kind      string `json:"kind"`       // 闭合 "subscription_fee"
    Name      string `json:"name"`       // plan 显示名
    AmountFen int64  `json:"amount_fen"` // 整数分
}
// quoteSnapshot 追加字段（JSON 向后兼容，旧快照解析出零值 → 购买路径按 legacy 拒绝）
type quoteSnapshot struct {
    PlanKey      string `json:"plan_key"`
    PlanVersion  int64  `json:"plan_version"`
    PriceFen     int64  `json:"price_fen"`
    CreditsMicro int64  `json:"credits_micro"`
    Currency     string                     `json:"currency"`  // "CNY"
    Features     map[string]bool            `json:"features,omitempty"`
    LineItems    []QuoteLineItem            `json:"line_items,omitempty"` // 首期恰一行 subscription_fee
}
// QuoteView 追加 Currency string / Features map[string]bool / LineItems []QuoteLineItem
```

- [ ] **Step 1: 写失败测试**（order_test.go 追加；复用既有 `newOrderTestEnv(t)`（order_test.go:59，返回 `*OrderService, *stubCheckoutProvider, *gorm.DB`）与 `seedPublishedPlan` 的同型模式（order_test.go:80 固定 seed pro/v3/9900 无 features——本用例需要 features，新增一个变体 helper））

```go
// seedPublishedProWithFeatures 按 seedPublishedPlan 的同型模式 seed 一个带 features 的版本。
func seedPublishedProWithFeatures(t *testing.T, db *gorm.DB) {
	t.Helper()
	def, _ := json.Marshal(domain.PlanVersion{Key: "pro", Version: 4, Price: 99_00, Monthly: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: domain.CurrencyCNY, Name: "Pro"})
	if err := db.Create(&repocommercial.PlanRow{PlanKey: "pro", Version: 4,
		DefinitionJSON: string(def), ExternalID: "ext-pro-4", State: domain.PlanStatePublished}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestCreateQuoteFreezesLineItemsFeaturesAndCurrency(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	seedPublishedProWithFeatures(t, db)
	q, err := svc.CreateQuote(context.Background(), 7, "pro")
	if err != nil {
		t.Fatal(err)
	}
	if q.Currency != "CNY" {
		t.Fatalf("currency = %q, want CNY", q.Currency)
	}
	if len(q.LineItems) != 1 || q.LineItems[0].Kind != "subscription_fee" || q.LineItems[0].AmountFen != 9900 {
		t.Fatalf("line items = %+v, want single subscription_fee 9900", q.LineItems)
	}
	if !q.Features["advanced_models"] {
		t.Fatalf("features = %+v, must freeze plan entitlements", q.Features)
	}
	if q.ExpiresAt == "" {
		t.Fatal("expiry must be present")
	}
}
```
（`domain.PlanVersion` 的 Features/Currency/Name 字段见 catalog.go:68-83，T07 additive 字段已存在。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/service/commercial/ -run TestCreateQuoteFreezes -count=1`
Expected: FAIL（`q.Currency`/`q.LineItems`/`q.Features` 未定义）

- [ ] **Step 3: 实现**——`CreateQuote` 从 `plan`（已 Unmarshal 的 PlanVersion）填 `Currency: domain.CurrencyCNY`、`Features: plan.Features`、`LineItems: []QuoteLineItem{{Kind: "subscription_fee", Name: plan 显示名（definition 字段名按 domain.PlanVersion 实际结构）, AmountFen: int64(plan.Price)}}`；QuoteView 与 wire（`quoteWire`，handler/commercial.go:232-244）透传新字段。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/modules/commercial/service/commercial/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/commercial/service/commercial/order.go internal/modules/commercial/service/commercial/order_test.go
git commit -m "feat(commercial): #81 quote freezes currency, features and subscription-fee line item"
```

---

### Task 7: PurchaseService（AC2/AC4 核心：匹配校验、幂等重试、过期/并发拒绝）

**Files:**
- Create: `internal/modules/commercial/service/commercial/purchase.go`
- Create: `internal/modules/commercial/service/commercial/purchase_test.go`
- Modify: `internal/modules/commercial/repository/commercial/order.go`（`GetOrderByQuote`）
- Modify: `internal/modules/commercial/service/commercial/order.go`（暴露 quote 读取：`QuoteForTenant` 公开化或新增公共方法）

**Interfaces:**
- Consumes: `OrderService.CreateOrder/ListOrders`（order.go:217/307）、`CatalogStore.GetQuote`、`PlanVersionService.GetPublication`（planversion.go:262）、Task 1 seam 类型、`BillingAccountService.EnsureBillingAccount`。
- Produces（Task 8/9/11 依赖）:
```go
var (
    ErrInvoiceQuoteMismatch  = errors.New("invoice_quote_mismatch")   // AC2：不创建支付请求
    ErrQuoteLegacySnapshot   = errors.New("quote_legacy_snapshot")    // 旧快照缺 currency/行项目 → 重新报价
    ErrPurchasePlanConflict  = errors.New("purchase_plan_conflict")   // AC4 并发套餐变更
    ErrPurchasePlanCharges   = errors.New("purchase_plan_charges")    // 切片限制：带 usage charges 的版本暂不开放购买
)
type PurchaseService struct{ /* accounts, plans, quotes, orders(*/OrderService), platform, now */ }
func NewPurchaseService(db *gorm.DB, accounts *BillingAccountService, plans *PlanVersionService,
    orders *OrderService, platform domain.CommercialPlatform) (*PurchaseService, error)
type PurchaseView struct {
    State       string     `json:"state"` // 闭合 awaiting_payment|active|absent|canceled
    Order       *OrderView `json:"order,omitempty"`
    PlanKey     string     `json:"plan_key,omitempty"`
    PlanVersion int64      `json:"plan_version,omitempty"`
    AmountFen   int64      `json:"amount_fen,omitempty"`
    Currency    string     `json:"currency,omitempty"`
    Reason      string     `json:"reason,omitempty"` // 闭合 unconfigured|unreachable|invalid_response|unsupported
}
func (s *PurchaseService) Purchase(ctx context.Context, tenantID uint64, quoteID, providerName, actor string) (PurchaseView, error)
func (s *PurchaseService) PurchaseStatus(ctx context.Context, tenantID uint64) (PurchaseView, error)
// repository：
func (s *OrderStore) GetOrderByQuote(ctx context.Context, tenantID uint64, quoteID string) (OrderRow, error) // 参数绑定 where tenant_id=? and quote_id=?
// order.go：
func (s *OrderService) QuoteSnapshotForTenant(ctx context.Context, tenantID uint64, quoteID string) (repocommercial.QuoteRow, quoteSnapshot, error) // 原 quoteForTenant 公开化
```

`Purchase` 算法（每步平台失败映射为 pending 视图 + 闭合 Reason，仅 DB 错误返回 error——#78 失败即状态先例）：
```
1 quote 读取（QuoteSnapshotForTenant）：租户不匹配→ErrQuoteTenantMismatch；快照 Currency==""→ErrQuoteLegacySnapshot
2 过期预检（q.ExpiresAt ≤ now → repocommercial.ErrQuoteExpired，与 OpenOrder 守卫一致）
3 publication 解析：plans.GetPublication(snap.PlanKey, snap.PlanVersion) → 缺→ErrPlanNotFound（要求先发布）
4 plan definition 校验：带 charges → ErrPurchasePlanCharges（D2 行项目切片限制）
5 EnsureBillingAccount（#78 惰性链；linked 才继续）
6 seam create_purchase_subscription（Key=CreatePurchaseSubscriptionCommandKey(extPurchase, pub.PlanCode)，
  AmountFen=snap.PriceFen, Currency=CNY）
  ─ ErrPlatformInvalidResponse 且身份读为不同 plan → ErrPurchasePlanConflict
7 purchase 快照读取 → 匹配校验（AC2）：
    snap2.PlanCode == pub.PlanCode && snap2.Currency == CurrencyCNY && snap2.Currency == snap.Currency
    && snap2.AmountFen == snap.PriceFen
  任一不成立 → ErrInvoiceQuoteMismatch（【不调用 CreateOrder，不产生渠道支付请求】）
8 匹配 → orders.GetOrderByQuote(tenant, quoteID)：
    已有订单 → 返回 {State: 订单状态映射, Order: 现有订单视图}（AC4 重试幂等；不二次开单）
    无 → orders.CreateOrder(ctx, tenantID, quoteID, providerName)（现有原子 quote 消费+开单+渠道调用）
      → ErrQuoteAlreadyUsed（并发竞态）→ 再读 GetOrderByQuote 返回既有订单
9 返回 {State: "awaiting_payment", Order, PlanKey, PlanVersion, AmountFen, Currency}
```
`PurchaseStatus`：seam purchase 快照 → 状态 + （有本地订单则附）。

- [ ] **Step 1: 写失败测试**（`purchase_test.go` 核心用例全文；fake 平台 + sqlite + 既有 `stubCheckoutProvider`（order_test.go:25-57，其 `createCalls []string` 就是渠道调用计数断言面））

```go
package commercial

import (
	"context"
	"errors"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPurchaseTestEnv(t *testing.T) (*PurchaseService, *commercialplatform.FakeAdapter, *stubCheckoutProvider, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
		&repocommercial.Subscription{}); err != nil {
		t.Fatal(err)
	}
	fake := commercialplatform.NewFakeAdapter()
	accounts, err := NewBillingAccountService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := NewPlanVersionService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	provider := &stubCheckoutProvider{queryState: payment.StateSucceeded}
	orders, err := NewOrderService(db, map[string]payment.Provider{"wechat": provider})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewPurchaseService(db, accounts, plans, orders, fake)
	if err != nil {
		t.Fatal(err)
	}
	return svc, fake, provider, db
}

// purchaseSeedPlan 走 #79 真实 draft→publish 流程落一个可购版本（含 publication 行）。
func purchaseSeedPlan(t *testing.T, plans *PlanVersionService, key string, amountFen int64) {
	t.Helper()
	view, err := plans.CreateDraft(context.Background(), "test:seed", DraftInput{
		PlanKey: key, Name: key + " Plan", AmountFen: amountFen, IncludedCreditsMicro: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: commercial.CurrencyCNY,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plans.Publish(context.Background(), "test:seed", "seed", key, view.Version); err != nil {
		t.Fatal(err)
	}
}

func purchaseQuote(t *testing.T, orders *OrderService, tenant uint64, planKey string) commercialsvcQuote {
	t.Helper()
	q, err := orders.CreateQuote(context.Background(), tenant, planKey)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func TestPurchaseHappyPathCreatesAwaitingPayment(t *testing.T) {
	svc, _, cp, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 31, "pro")
	view, err := svc.Purchase(context.Background(), 31, q.ID, "wechat", "billing-admin")
	if err != nil {
		t.Fatal(err)
	}
	if view.State != commercial.PurchaseStateAwaitingPayment || view.Order == nil || view.Order.CheckoutURL == "" {
		t.Fatalf("view = %+v", view)
	}
	if len(cp.createCalls) != 1 {
		t.Fatalf("channel creates = %d, want 1", len(cp.createCalls))
	}
}

func TestPurchaseAbortsOnInvoiceMismatch(t *testing.T) { // AC2
	svc, fake, cp, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 32, "pro")
	// 造偏差：先把订阅按 8800 建到 fake（模拟权威面金额与 Quote 不一致——如本地目录与权威目录漂移）
	fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(
			commercial.ExternalPurchaseSubscriptionID(32), commercial.DeterministicPlanCode("pro", 1)),
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 32, ExternalCustomerID: commercial.ExternalCustomerID(32),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(32),
			PlanCode: commercial.DeterministicPlanCode("pro", 1),
			AmountFen: 8800, Currency: commercial.CurrencyCNY,
		},
	})
	_, err := svc.Purchase(context.Background(), 32, q.ID, "wechat", "billing-admin")
	if !errors.Is(err, ErrInvoiceQuoteMismatch) {
		t.Fatalf("mismatch must abort, got %v", err)
	}
	if len(cp.createCalls) != 0 {
		t.Fatalf("NO channel payment request may be created on mismatch, got %d", len(cp.createCalls))
	}
}

func TestPurchaseExpiredQuoteRejected(t *testing.T) { // AC4 过期
	svc, _, _, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 33, "pro")
	// 参数绑定过期（S2：无拼接 SQL）
	if err := db.Exec(`UPDATE commercial_quotes SET expires_at = ? WHERE id = ?`,
		time.Now().Add(-time.Minute), q.ID).Error; err != nil {
		t.Fatal(err)
	}
	_, err := svc.Purchase(context.Background(), 33, q.ID, "wechat", "x")
	if !errors.Is(err, repocommercial.ErrQuoteExpired) {
		t.Fatalf("expired quote must be rejected, got %v", err)
	}
}

func TestPurchaseRetryReturnsExistingOrderWithoutDuplicates(t *testing.T) { // AC4 重试
	svc, fake, cp, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 34, "pro")
	first, err := svc.Purchase(context.Background(), 34, q.ID, "wechat", "a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Purchase(context.Background(), 34, q.ID, "wechat", "a")
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if first.Order.ID != second.Order.ID {
		t.Fatalf("retry must return the SAME order, got %q then %q", first.Order.ID, second.Order.ID)
	}
	if len(cp.createCalls) != 1 {
		t.Fatalf("retry must not open a second channel request, creates=%d", len(cp.createCalls))
	}
	if n := len(fake.PurchaseSubscriptions()); n != 1 {
		t.Fatalf("retry must not create a second subscription, got %d", n)
	}
}

func TestPurchaseConcurrentPlanChangeConflicts(t *testing.T) { // AC4 并发套餐变更
	svc, fake, cp, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	purchaseSeedPlan(t, svc.plans, "max", 19900)
	qPro := purchaseQuote(t, svc.orders, 35, "pro")
	qMax := purchaseQuote(t, svc.orders, 35, "max")
	if _, err := svc.Purchase(context.Background(), 35, qPro.ID, "wechat", "a"); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Purchase(context.Background(), 35, qMax.ID, "wechat", "a")
	if !errors.Is(err, ErrPurchasePlanConflict) && !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("second concurrent plan change must conflict, got %v", err)
	}
	if n := len(fake.PurchaseSubscriptions()); n != 1 {
		t.Fatalf("exactly one subscription, got %d", n)
	}
	if len(cp.createCalls) != 1 {
		t.Fatalf("exactly one channel request, got %d", len(cp.createCalls))
	}
}
```
（`commercialsvcQuote` 即 `QuoteView`——直接写 `QuoteView` 返回类型即可，此处避免与本包同名冲突时用别名 `type commercialsvcQuote = QuoteView`；`payment` import 补 `github.com/Tencent/WeKnora/internal/modules/commercial/payment`。若 `svc.plans`/`svc.orders` 为私有字段导致测试不可达，将 `newPurchaseTestEnv` 与 service 同包放置（本就在同包 `commercial`），私有字段可直访。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/service/commercial/ -run TestPurchase -count=1`
Expected: FAIL（`NewPurchaseService` 未定义）

- [ ] **Step 3: 实现**——按 Produces 块与算法实现 `purchase.go`；`order.go` 的 `quoteForTenant` 重命名为公共 `QuoteSnapshotForTenant`（保留旧私有别名调用点零改动）；`OrderStore.GetOrderByQuote` 参数绑定实现；platform 失败→`pending` 视图 + Reason（`platformReason` helper 已在 benefits.go 有先例可复用/提为共享）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/modules/commercial/service/commercial/ -count=1 && go test ./internal/modules/commercial/... -count=1`
Expected: PASS（含 commercial/repository、domain 包不回归）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/commercial/service/commercial/purchase.go internal/modules/commercial/service/commercial/purchase_test.go internal/modules/commercial/service/commercial/order.go internal/modules/commercial/repository/commercial/order.go
git commit -m "feat(commercial): #81 purchase service gates channel order behind quote-subscription match"
```

---

### Task 8: HTTP 面（handler + 路由 + container 装配）

**Files:**
- Modify: `internal/handler/commercial.go`（`SetPurchaseService`/`Purchase`/`PurchaseStatus` + `quoteWire` 扩展）
- Modify: `internal/router/routes_commercial.go:48-54`（路由组内追加）
- Modify: `internal/container/container.go`（#80 块后追加注册，模式同 L915-925）
- Test: `internal/router/commercial_purchase_route_test.go`（**新建，放 router 包**——HTTP 面测试的仓库先例是 `internal/router/commercial_benefits_route_test.go`（gin engine + `RegisterCommercialRoutes` + `authAs` 身份注入 + httptest），handler 包没有 commercial HTTP 测试先例；`authAs`/租户注入 helper 已在 `commercial_scope_test.go:137-146` 共享）

**Interfaces:**
- Consumes: Task 7 `PurchaseService`；路由组既有 guard（`RequireManageBillingForWrites`，routes_commercial.go:23-26）。
- Produces:
```
POST /api/v1/commercial/purchases   body {"quote_id":"qt_...","provider":"wechat"} → 201 {"success":true,"data":purchaseWire}
GET  /api/v1/commercial/purchase    → 200 {"success":true,"data":purchaseWire}（无购买时 {"state":"absent"}）
错误映射：ErrInvoiceQuoteMismatch→409 "invoice_quote_mismatch"；
  ErrQuoteExpired→409 "quote expired"；ErrQuoteVersionConflict→409 re-quote；ErrPurchasePlanConflict→409；
  ErrQuoteLegacySnapshot/ErrPurchasePlanCharges→409 说明重报价；ErrPaymentProviderUnconfigured→503；
  ErrQuoteTenantMismatch→404；ErrPlanNotFound→404
purchaseWire：{"state","order"?(orderWire),"plan_key","plan_version","amount_fen"(digit string),"currency","reason"}
handler: func (h *CommercialHandler) SetPurchaseService(s *commercialsvc.PurchaseService)
```

- [ ] **Step 1: 写失败测试**（`internal/router/commercial_purchase_route_test.go` 全文；`newPurchaseEngine` 仿 `newBenefitsEngine`（commercial_benefits_route_test.go:31-84），渠道 provider 需要一个 router 包内的计数 stub——`stubCheckoutProvider` 在 service 包测试内不可导入，按其形状（order_test.go:25-57）在本文件重定义一个最小计数版）

```go
package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/handler"
	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
	commercialplatform "github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
	"github.com/Tencent/WeKnora/internal/modules/commercial/payment"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	"github.com/gin-gonic/gin"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// routePurchaseProvider 计数渠道 Create（AC2「不拉起支付请求」断言面）。
type routePurchaseProvider struct{ creates int }

func (p *routePurchaseProvider) Create(context.Context, payment.OrderRequest) (payment.AttemptResult, error) {
	p.creates++
	return payment.AttemptResult{State: payment.StatePending, CheckoutURL: "https://pay.example/qr"}, nil
}
func (p *routePurchaseProvider) Query(context.Context, string) (payment.AttemptResult, error) {
	return payment.AttemptResult{State: payment.StatePending}, nil
}
func (p *routePurchaseProvider) Close(context.Context, string) error { return nil }
func (p *routePurchaseProvider) Verify(context.Context, http.Header, []byte) (commercial.PaymentFact, error) {
	return commercial.PaymentFact{}, nil
}
func (p *routePurchaseProvider) Refund(context.Context, payment.RefundRequest) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *routePurchaseProvider) QueryRefund(context.Context, string) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}

// newPurchaseEngine 组装真实 PurchaseService 链 + 渠道计数 stub。
func newPurchaseEngine(t *testing.T) (*gin.Engine, *gorm.DB, *commercialplatform.FakeAdapter, *routePurchaseProvider) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
		&repocommercial.Subscription{}, &repocommercial.BillingAccount{}); err != nil {
		t.Fatal(err)
	}
	fake := commercialplatform.NewFakeAdapter()
	accounts, err := commercialsvc.NewBillingAccountService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := commercialsvc.NewPlanVersionService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	provider := &routePurchaseProvider{}
	orders, err := commercialsvc.NewOrderService(db, map[string]payment.Provider{"wechat": provider})
	if err != nil {
		t.Fatal(err)
	}
	purchases, err := commercialsvc.NewPurchaseService(db, accounts, plans, orders, fake)
	if err != nil {
		t.Fatal(err)
	}
	h := handler.NewCommercialHandler(db)
	h.SetOrderService(orders)
	h.SetPurchaseService(purchases)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	RegisterCommercialRoutes(v1, h)
	return engine, db, fake, provider
}

// routeSeedPlanAndQuote 走真实 draft→publish→quote 链。
func routeSeedPlanAndQuote(t *testing.T, engine *gin.Engine, plans *commercialsvc.PlanVersionService, orders *commercialsvc.OrderService, tenant uint64, planKey string) commercialsvc.QuoteView {
	t.Helper()
	view, err := plans.CreateDraft(context.Background(), "route:seed", commercialsvc.DraftInput{
		PlanKey: planKey, Name: planKey, AmountFen: 9900, IncludedCreditsMicro: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: commercial.CurrencyCNY,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plans.Publish(context.Background(), "route:seed", "seed", planKey, view.Version); err != nil {
		t.Fatal(err)
	}
	q, err := orders.CreateQuote(context.Background(), tenant, planKey)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

// purchasePost 以 owner 身份 POST /commercial/purchases。
func purchasePost(t *testing.T, engine *gin.Engine, tenant uint64, body string) (*httptest.ResponseRecorder, *gin.Engine) {
	t.Helper()
	return purchaseReq(t, engine, tenant, http.MethodPost, "/api/v1/commercial/purchases", body)
}

func purchaseReq(t *testing.T, engine *gin.Engine, tenant uint64, method, path, body string) (*httptest.ResponseRecorder, *gin.Engine) {
	t.Helper()
	w := httptest.NewRecorder()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	served := gin.New()
	served.Use(authAs(tenant, "user-1", "owner"))
	served.Handle(method, "/api/v1/*rest", func(c *gin.Context) { engine.ServeHTTP(c.Writer, c.Request) })
	served.ServeHTTP(w, req)
	return w, served
}

// newPurchaseEngine 的返回签名实现为 (*gin.Engine, *gorm.DB, *commercialplatform.FakeAdapter,
// *routePurchaseProvider, *commercialsvc.PlanVersionService, *commercialsvc.OrderService)
// —— plans/orders 一并返回，供用例做 seed/报价。
func TestPurchaseRouteHappyPathAwaitingPayment(t *testing.T) {
	engine, _, _, provider, plans, orders := newPurchaseEngine(t)
	q := routeSeedPlanAndQuote(t, engine, plans, orders, 41, "pro")
	w, _ := purchasePost(t, engine, 41, `{"quote_id":"`+q.ID+`","provider":"wechat"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	for _, want := range []string{
		`"state":"awaiting_payment"`, `"checkout_url"`, `"amount_fen":"9900"`, `"plan_key":"pro"`,
	} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("body must contain %s, got %s", want, w.Body.String())
		}
	}
	if provider.creates != 1 {
		t.Fatalf("channel creates = %d, want 1", provider.creates)
	}
}
```
（happy 用例已给全文；用例 2-4 按下列断言体实现——同为完整断言，不允许缩水：
- 用例 1（happy）：seed pro → 报价 → `purchasePost(..., `{"quote_id":q.ID,"provider":"wechat"}`)` → `w.Code==http.StatusCreated`；`strings.Contains(w.Body.String(), "\"state\":\"awaiting_payment\"")`；`strings.Contains(w.Body.String(), "\"checkout_url\"")`；`strings.Contains(w.Body.String(), "\"amount_fen\":\"9900\"")`；`provider.creates==1`。
- 用例 2（AC2，全文如下；对应追踪矩阵 AC2 行的 router 断言）：
```go
func TestPurchaseRouteMismatchReturns409WithoutOrder(t *testing.T) {
	engine, db, fake, provider, plans, orders := newPurchaseEngine(t)
	q := routeSeedPlanAndQuote(t, engine, plans, orders, 42, "pro")
	// 预置偏差：权威面订阅金额 8800 ≠ Quote 9900（Task 7 TestPurchaseAbortsOnInvoiceMismatch 同法）
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(
			commercial.ExternalPurchaseSubscriptionID(42), commercial.DeterministicPlanCode("pro", 1)),
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 42, ExternalCustomerID: commercial.ExternalCustomerID(42),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(42),
			PlanCode: commercial.DeterministicPlanCode("pro", 1),
			AmountFen: 8800, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	w, _ := purchasePost(t, engine, 42, `{"quote_id":"`+q.ID+`","provider":"wechat"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invoice_quote_mismatch") {
		t.Fatalf("body must carry the closed mismatch token, got %s", w.Body.String())
	}
	if provider.creates != 0 {
		t.Fatalf("NO channel request may fire on mismatch, creates = %d", provider.creates)
	}
	w2, _ := purchaseReq(t, engine, 42, http.MethodGet, "/api/v1/commercial/orders", "")
	if w2.Code != http.StatusOK || strings.Contains(w2.Body.String(), `"id":"ord_`) {
		t.Fatalf("orders must be empty, code=%d body=%s", w2.Code, w2.Body.String())
	}
	_ = db
}
```
- 用例 3（过期）：`db.Exec("UPDATE commercial_quotes SET expires_at = ? WHERE id = ?", time.Now().Add(-time.Minute), q.ID)`（参数绑定）→ POST → `w.Code==http.StatusConflict`；body 含 `"quote expired"`。
- 用例 4（GET /commercial/purchase）：购买后 `purchaseReq(GET, "/api/v1/commercial/purchase", "")` → 200 且含 `"awaiting_payment"`；另一未购买租户 `authAs(tenant+1,...)` GET → 200 且含 `"absent"`。
（`authAs` 复用 commercial_scope_test.go:137-146，owner 角色满足 `RequireManageBillingForWrites` 与 capability gate 的既有放行路径——若 gate 另需 grant 表 seed，按 commercial_scope_test.go 中 owner 放行的实际做法对齐。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/router/ -run TestPurchaseRoute -count=1`
Expected: FAIL（`SetPurchaseService` 未定义 → 编译失败，或路由 404）

- [ ] **Step 3: 实现**——handler/路由/装配按 Produces 块；container 注册：
```go
must(container.Provide(commercialsvc.NewPurchaseService))
must(container.Invoke(func(h *handler.CommercialHandler, s *commercialsvc.PurchaseService) {
	h.SetPurchaseService(s)
}))
```
（放在 #80 BenefitsService 块之后，同一注释风格，注明依赖 BillingAccountService/PlanVersionService/OrderService 的 dig 解析。）

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/router/ -count=1 && go build ./...`
Expected: PASS + 编译通过（router 包既有套件不回归）

- [ ] **Step 5: Commit**

```bash
git add internal/handler/commercial.go internal/router/commercial_purchase_route_test.go internal/router/routes_commercial.go internal/container/container.go
git commit -m "feat(api): #81 purchase endpoints expose awaiting-payment state behind billing gate"
```

---

### Task 9: 前端契约与客户端（contracts + api-client + web 单测）

**Files:**
- Modify: `packages/contracts/src/commercial.ts`（`PurchaseView`/`parsePurchaseView`；`QuoteView` 扩展 currency/features/line_items）
- Modify: `packages/api-client/src/commercial.ts`（`purchase()`/`purchaseStatus()`）
- Test: `packages/contracts/test/commercial.test.ts`（**追加到既有测试文件**；root package.json:13 的 `test:shared` glob 只收 `packages/contracts/test/*.test.ts` 与 `packages/contracts/src/craft/*.test.ts`，放 `src/*.test.ts` 不会被执行——假信号。既有文件已 import `'../src/commercial.ts'` 并导出 parseQuoteView 等断言先例）

**Interfaces:**
- Consumes: Task 8 wire 形状。
- Produces:
```ts
export interface PurchaseLineItemView { kind:string; name:string; amount_fen:string }
export interface PurchaseView {
  state:'awaiting_payment'|'active'|'absent'|'canceled';
  order?:OrderView; plan_key?:string; plan_version?:number;
  amount_fen?:string; currency?:string; reason?:string;
}
export function parsePurchaseView(value:unknown):PurchaseView  // 闭合 state 枚举校验
// QuoteView 追加（可选字段向后兼容）：
//   currency?:string; features?:Record<string,boolean>; line_items?:PurchaseLineItemView[]
// api-client：
async purchase(input:{quote_id:string; provider:'wechat'|'alipay'}, signal?:AbortSignal):Promise<PurchaseView>
async purchaseStatus(signal?:AbortSignal):Promise<PurchaseView>
```

- [ ] **Step 1: 写失败测试**（**追加到既有** `packages/contracts/test/commercial.test.ts`；沿用该文件既有 import，只新增用例，不重复 import 语句——追加的用例体如下）

```ts
test('parsePurchaseView accepts awaiting_payment with order', () => {
  const v = parsePurchaseView({ state: 'awaiting_payment',
    order: { id: 'ord_1', quote_id: 'qt_1', state: 'pending', amount_fen: '9900', currency: 'CNY',
      payment: 'pending', fulfillment: 'pending', version: 1 },
    plan_key: 'pro', plan_version: 1, amount_fen: '9900', currency: 'CNY' });
  assert.equal(v.state, 'awaiting_payment');
  assert.equal(v.order?.id, 'ord_1');
});

test('parsePurchaseView rejects raw provider states', () => {
  assert.throws(() => parsePurchaseView({ state: 'incomplete' }));
  assert.throws(() => parsePurchaseView({ state: 'awaiting_payment', amount_fen: 9900 })); // 非数字串
});

test('parseQuoteView passes through frozen line items', () => {
  const q = parseQuoteView({ id: 'qt_1', amount_fen: '9900', credit_delta: '9900000',
    expires_at: '2026-09-23T12:00:00Z', currency: 'CNY',
    features: { advanced_models: true },
    line_items: [{ kind: 'subscription_fee', name: 'Pro', amount_fen: '9900' }] });
  assert.equal(q.line_items?.[0]?.kind, 'subscription_fee');
  assert.equal(q.features?.advanced_models, true);
});
```
（文件顶部既有 `import { parseCommercialSummary, parseCommercialUsageList, parseOrderView, parseQuoteView, parseRefundView } from '../src/commercial.ts';` 增补 `parsePurchaseView`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `pnpm test:shared`
Expected: FAIL（`parsePurchaseView` 未导出——TypeScript 解析报错使该测试文件失败）

- [ ] **Step 3: 实现**——按 Produces 块实现（解析器风格照 parseQuoteView：digitString/nonEmptyString 校验）。

- [ ] **Step 4: 跑测试确认通过**

Run: `pnpm test:shared && pnpm typecheck:shared`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/commercial.ts packages/api-client/src/commercial.ts packages/contracts/test/commercial.test.ts
git commit -m "feat(contracts): #81 purchase view parser and client methods"
```

---

### Task 10: 前端页面（Checkout 提交购买 + Billing 待付款状态）

**Files:**
- Modify: `apps/web/src/commercial/CheckoutPage.tsx`
- Modify: `apps/web/src/commercial/BillingPage.tsx`
- Test: `apps/web/src/commercial/CheckoutPage.test.tsx`（新建/扩展既有测试）

**Interfaces:**
- Consumes: Task 9 `client.commercial.purchase/purchaseStatus`；`parsePurchaseView`。
- Produces: CheckoutPage 提交走 `purchase()`（替代直接 `createOrder`），页面展示 Quote 的 `line_items`/`features`/`currency`/`expires_at`（AC1 用户可见）；BillingPage 套餐行在 `purchaseStatus().state==='awaiting_payment'` 时显示「待付款（权益未开放）」；页面文案不含任何 Lago 词汇（spec L170）。

- [ ] **Step 1: 写失败测试**（新建 `apps/web/src/commercial/CheckoutPage.test.tsx`——apps/web 的组件测试先例是 `apps/web/src/organizations/OrganizationsPage.test.tsx`（jsdom + node:test + createRoot + act + stub client + CSS/SVG module hook），commercial 目录既有测试只有 .test.ts（纯函数），组件测试按 organizations 先例建立；`pnpm test:web` 的 glob `src/**/*.test.ts(x)` 会收集 .tsx）

```tsx
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { act } from 'react';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (h: { resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/commercial/checkout' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});

const { createRoot, type Root } = await import('react-dom/client');

const quote = {
  id: 'qt_1', plan_key: 'pro', plan_version: 1, amount_fen: '9900',
  credit_delta: '9900000', expires_at: '2026-09-23T12:00:00Z',
  currency: 'CNY',
  features: { advanced_models: true },
  line_items: [{ kind: 'subscription_fee', name: 'Pro', amount_fen: '9900' }],
};
const order = {
  id: 'ord_1', quote_id: 'qt_1', state: 'pending', amount_fen: '9900', currency: 'CNY',
  payment: 'pending', fulfillment: 'pending', checkout_url: 'https://pay.example/qr', version: 1,
};

test('checkout renders frozen quote line items and submits a purchase', async () => {
  const calls: Array<{ quote_id: string; provider: string }> = [];
  const client = {
    commercial: {
      quote: async () => quote,
      purchase: async (input: { quote_id: string; provider: string }) => {
        calls.push(input);
        return { state: 'awaiting_payment', order, plan_key: 'pro', plan_version: 1, amount_fen: '9900', currency: 'CNY' };
      },
      getOrder: async () => order,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ tenantId: 31, role: 'owner' } as never);
  let root: Root | undefined;
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  const text = document.body.textContent ?? '';
  assert.match(text, /¥99\.00/);                       // 冻结金额（AC1）
  assert.match(text, /订阅费|subscription_fee/);          // 行项目（AC1）
  assert.match(text, /advanced_models|高级模型/);         // 冻结权益（AC1）
  assert.match(text, /待付款/);                          // 产品状态（AC3）
  assert.equal(calls.length, 1);                        // 提交恰好一次购买
  assert.equal(calls[0]?.quote_id, 'qt_1');
  assert.equal(calls[0]?.provider, 'wechat');
  await act(async () => { root?.unmount(); });
});
```
（`createScopeController` 的构造签名按 `@weknora/domain/scope` 实际导出调整——先例见 CheckoutPage.tsx:4/36 的用法；`scopeController.current()` 需要能返回 `{ scope: { tenantId: 31 }, signal }` 形状，测试里按 organizations 先例的 controller stub 方式构造即可，不追求真实实现。）

- [ ] **Step 2: 跑测试确认失败**

Run: `pnpm test:web`
Expected: FAIL（页面仍调 createOrder / 新文案不存在）

- [ ] **Step 3: 实现**——CheckoutPage：`quoteRef` 后渲染 `line_items`/`features`/`expires_at` 列表；提交分支改 `client.commercial.purchase({quote_id, provider:'wechat'})`（保留既有幂等键注释与重试语义：重试同 quote 复用同一 purchase 调用，由后端返回同一订单）；BillingPage：挂载时并行 `purchaseStatus()`，`awaiting_payment` 显示「待付款」。

- [ ] **Step 4: 跑测试确认通过**

Run: `pnpm test:web && pnpm typecheck:web`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/commercial/CheckoutPage.tsx apps/web/src/commercial/BillingPage.tsx apps/web/src/commercial/CheckoutPage.test.tsx
git commit -m "feat(web): #81 checkout submits gated purchase and billing shows awaiting-payment"
```

---

### Task 11: 真实栈集成测试（lago_integration build tag）

**Files:**
- Create: `internal/modules/commercial/commercialplatform/lago_purchase_integration_test.go`

**Interfaces:**
- Consumes: `newIntegrationBenefitsService` 模式（lago_benefits_integration_test.go:78-118）；Task 3/4 适配器；Task 7 服务。
- Produces: `TestLagoPurchaseIntegration`（env 门控：`LAGO_INTEGRATION_BASE_URL`+`LAGO_INTEGRATION_API_KEY`+`LAGO_INTEGRATION_PROVIDER_CUSTOMER_PREFIX`（或 `LAGO_INTEGRATION_STRIPE_API_KEY`）；缺任一 → `t.Skip("blocked-env: ...")`——与 #80 同一诚实姿态）。

- [ ] **Step 1: 写测试**（骨架，六阶段单测试顺序执行；每个断言映射 AC）

```
Phase 1（AC1 前置）：通过 PlanVersionService 发布 pro v1（9900 分 CNY、无 charges、features 含 advanced_models）；
  CreateQuote → 断言 QuoteView.Currency=="CNY"、LineItems 单行 9900、Features 冻结、ExpiresAt 非空。
Phase 2（AC3）：PurchaseService.Purchase(tenantA, quote, "wechat"——集成环境无渠道适配器时此步断言 503/ErrPaymentProviderUnconfigured？
  不——集成测试直接走 seam 层：SubmitCommand(create_purchase_subscription) → 断言回执身份；
  ReadSnapshot(purchase) → awaiting_payment + plan_code + amount 9900 + CNY。
  （渠道订单链在 PurchaseService 单测已覆盖；集成测试聚焦 Lago 真对象。）
Phase 3（AC3 Entitlement 未开放）：GET /api/v1/customers/{ext}/entitlements → 404（raw probe 断言，
  模式同 lago_benefits_integration_test.go rawIntegrationRequest）。
Phase 4（AC4 重试）：再次 SubmitCommand 同 Key → 断言成功；raw probe
  GET /api/v1/subscriptions?external_id={purchase}&status[]=incomplete → 恰一条（t02 显式 status 教训）。
Phase 5（AC4 不重复 Invoice，API 不可见的 DB 权威核验——F5）：raw probe 不可用，改为断言
  「subscriptions 列表同一身份仅一条 + 重放后 subscription updated_at 不变」；DB 直查留给 Task 12 真实流程验证
  （集成测试不持有 Lago DB 凭据）。
Phase 6（并发冲突）：不同 plan_code 的第二次 create → ErrPlatformInvalidResponse；订阅列表仍一条。
租户隔离：tenantB 全链同跑，A 的对象计数不变。
```
（写成可运行 Go 代码，raw probe helper 从 benefits 集成测试复制模式；每次运行用 `rand` 后缀 external id 清场。）

- [ ] **Step 2: 跑测试确认在无栈时 skip**

Run: `go test -tags lago_integration ./internal/modules/commercial/commercialplatform/ -run TestLagoPurchaseIntegration -count=1`
Expected: SKIP（blocked-env，无 LAGO_INTEGRATION_* env）

- [ ] **Step 3: 对真实栈跑（凭据来源先按 Task 12 Step 0 完成环境恢复——`deploy/lago/.env` 在本机已随 .worktrees/lago-73 删除而丢失，不能按文 source；org API key / operator 账号从运行容器 env 取或重建栈）**

Run:
```bash
# Step 0（Task 12 Step 0 的路径 A：栈仍在跑）——凭据只进 shell 环境，不落盘（S3）
export LAGO_API_KEY="$(docker exec weknora-lago-api-1 printenv LAGO_ORG_API_KEY)"
source ~/.zcode/issue72-stripe.env   # STRIPE_SECRET_KEY（sk_test_，仅环境变量）
LAGO_INTEGRATION_BASE_URL=http://127.0.0.1:48889 \
LAGO_INTEGRATION_API_KEY="$LAGO_API_KEY" \
LAGO_INTEGRATION_STRIPE_API_KEY="$STRIPE_SECRET_KEY" \
go test -tags lago_integration ./internal/modules/commercial/commercialplatform/ -run TestLagoPurchaseIntegration -count=1 -v -timeout 300s | tee /tmp/lago81-integration.txt
```
Expected: PASS（六阶段；输出 tee 到 /tmp，敏感值不落仓库）。若 provider 未注册（F2 的 `payment_provider_not_found`）→ 先跑 Task 12 的 provider 预备脚本再重试；若栈不在运行（docker ps 无 weknora-lago-*）→ 走 Task 12 Step 0 的路径 B 重建后再跑。

- [ ] **Step 4: Commit**

```bash
git add internal/modules/commercial/commercialplatform/lago_purchase_integration_test.go
git commit -m "test(commercialplatform): #81 real-stack purchase integration evidence (env-gated)"
```

---

### Task 12: 真实流程验证、证据与文档、Ledger、收尾提交

**Files:**
- Create: `docs/migrations/lago/t09-quote-invoice/README.md`
- Create: `docs/migrations/lago/t09-quote-invoice/DECISION.md`（D1/D2/D3 设计决策 + F1-F10 契约事实 + spec L121 行项目偏差与补偿记录）
- Create: `deploy/lago/evidence/t09-run.txt`（运行记录，格式照 t08-run.txt）
- Modify: `docs/plans/issue-72-ledger-81.md`（执行结果追加）

- [ ] **Step 0: Lago 环境恢复（前置现实：`deploy/lago/.env` 在本机任何 checkout 均不存在——原栈由已删除的 `.worktrees/lago-73` 启动，.env 随 worktree 删除而丢失；计划审查第 1 轮实测确认。二选一，操作与判据写入 t09-run.txt）**

```bash
# 路径 A（栈仍在跑——首选，零重建、保留既有 lab 对象）：
docker ps --format '{{.Names}}' | grep -q '^weknora-lago-api-1$' || echo "stack down -> use path B"
export LAGO_API_KEY="$(docker exec weknora-lago-api-1 printenv LAGO_ORG_API_KEY)"
export LAGO_ORG_USER_EMAIL="$(docker exec weknora-lago-api-1 printenv LAGO_ORG_USER_EMAIL)"
export LAGO_ORG_USER_PASSWORD="$(docker exec weknora-lago-api-1 printenv LAGO_ORG_USER_PASSWORD)"
curl -sf http://127.0.0.1:48889/health >/dev/null && echo "lago healthy"
# 注意：此路径下【不要】运行 lago.sh up——新生成的 .env 密钥与运行容器不一致，up 会触发
# recreate，LAGO_ENCRYPTION_DETERMINISTIC_KEY 变更会使已加密列不可解。只做只读健康检查。

# 路径 B（栈已停/需要重建）：
./deploy/lago/lago.sh init                      # 在【本 worktree】生成 deploy/lago/.env（随机密钥，mode 600）
# 首次 seed 追加（README L75-86 惯例）：LAGO_CREATE_ORG=true + LAGO_ORG_USER_EMAIL/PASSWORD +
# LAGO_ORG_API_KEY（random，python3 -c "import secrets; print(secrets.token_urlsafe(32))"）
./deploy/lago/lago.sh up                        # docker compose up -d --wait；冷启动数分钟（Rails migrate+seed）
./deploy/lago/lago.sh status                    # 全部 healthy
export LAGO_API_KEY="$(grep '^LAGO_ORG_API_KEY=' deploy/lago/.env | cut -d= -f2-)"
# （重建即新 org——旧 lab 对象不迁移，符合 spec「只处理开发数据，不迁移测试数据」）
```

- [ ] **Step 1: 环境预备（一次性，操作序列与判定全部写入 t09-run.txt；凭据全部来自 Step 0 的 shell 环境变量）**
```bash
# 1. provider 注册（F2：GraphQL + operator JWT）
source ~/.zcode/issue72-stripe.env    # STRIPE_SECRET_KEY（sk_test_，仅环境变量，S3）
JWT=$(curl -s -X POST http://127.0.0.1:48889/graphql -H 'Content-Type: application/json' \
  -d "{\"query\":\"mutation Login(\$i:LoginUserInput!){ loginUser(input:\$i){ token } }\",
       \"variables\":{\"i\":{\"email\":\"$LAGO_ORG_USER_EMAIL\",\"password\":\"$LAGO_ORG_USER_PASSWORD\"}}}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["loginUser"]["token"])')
curl -s -X POST http://127.0.0.1:48889/graphql -H "Authorization: Bearer $JWT" -H 'Content-Type: application/json' \
  -d "{\"query\":\"mutation Add(\$i:AddStripePaymentProviderInput!){ addStripePaymentProvider(input:\$i){ code name } }\",
       \"variables\":{\"i\":{\"code\":\"weknora-stripe\",\"name\":\"WeKnora Stripe\",\"secretKey\":\"$STRIPE_SECRET_KEY\"}}}"
# 判据：响应含 "code":"weknora-stripe"；JWT/stripe key 不回显到 t09-run.txt（记录时替换为 <redacted>）
# 2. WeKnora 后端（worktree 内）
cp .env.example .env && printf '\nDB_DRIVER=postgres\n# 复用宿主 dev 容器 WeKnora-postgres-dev:5432 / redis:6379（.env.example 已指向）\n' >> .env
export WEKNORA_COMMERCIAL_PLATFORM_PROVIDER=lago
export WEKNORA_COMMERCIAL_PLATFORM_URL=http://127.0.0.1:48889
export WEKNORA_COMMERCIAL_PLATFORM_API_KEY="$LAGO_API_KEY"        # 来自 Step 0（不再 source/grep .env）
export WEKNORA_COMMERCIAL_STRIPE_API_KEY="$STRIPE_SECRET_KEY"
./scripts/dev.sh app                # go run ./cmd/server，:8080（避开 :5272/:5273）；GET /health → 200
# 3. 前端
pnpm --filter @weknora/web dev      # vite :5173，/api 代理 :8080
```

- [ ] **Step 2: API 链路端到端验证（真实数据断言；记录进 t09-run.txt）**
```bash
# a. 管理后台发布 pro v1（走真实 API；平台操作员凭据按 .env 的 admin 种子——按 cmd/server 既有 seed 流程）
#    POST /api/v1/admin/plans/drafts → PATCH → POST .../publish（#79 面；或用既有 seed 脚本）
# b. 账单管理员报价：POST /api/v1/commercial/quotes {"plan_key":"pro"}
#    断言 201：data.plan_version==1、amount_fen=="9900"、currency=="CNY"、line_items 长度 1（kind subscription_fee）、
#    features 含 advanced_models、expires_at 在 +30min 内（AC1）
# c. 提交购买：POST /api/v1/commercial/purchases {"quote_id":"qt_...","provider":"wechat"}
#    断言 201：data.state=="awaiting_payment"、data.order.checkout_url 非空（渠道请求已建）、order.payment=="pending"
# d. 产品状态：GET /api/v1/commercial/purchase → data.state=="awaiting_payment"（AC3）
# e. Entitlement 未开放：curl -H "Authorization: Bearer $LAGO_ORG_API_KEY" \
#    "http://127.0.0.1:48889/api/v1/customers/weknora-tenant-<T>-purchase/../entitlements"（按真实路径
#    /api/v1/customers/{ext}/entitlements）→ 404 或不含 pro 的 feature（AC3；F6）
# f. 重试不重复：重复 c 一次 → 返回同一 order.id；然后 Lago 侧核验（g/h）
# g. Lago 订阅核验：GET "http://127.0.0.1:48889/api/v1/subscriptions?external_id=weknora-tenant-<T>-purchase&status[]=incomplete"
#    → subscriptions 数组长度恰 1（AC4）
# h. Invoice 不重复的 DB 权威核验（F5：API 读不到 open invoice）：
docker exec weknora-lago-db-1 psql -U lago -d lago -tAc \
  "SELECT count(*) FROM invoices i JOIN customers c ON c.id=i.customer_id \
   WHERE c.external_id='weknora-tenant-<T>' AND i.invoice_type='subscription'"
#    → 重试前后计数不变（恰 +1 相对购买前基线）（AC4）
# i. AC2 不一致真实验证：psql 参数化改本地目录价格制造偏差后重复报价购买 → 409 invoice_quote_mismatch、
#    渠道无新请求（GET /api/v1/commercial/orders 计数不变）；验证后还原目录行并复测 happy path
```
（`<T>` 为验证用空间 id；所有 SQL 走 `psql -v` 或固定字面量常量值——验证脚本不拼用户输入。）

- [ ] **Step 3: 浏览器端到端验证（Playwright headless，MCP 插件或 npx playwright）**

操作序列（页面 URL 与断言的可观察结果；截图存 `docs/migrations/lago/t09-quote-invoice/evidence/`）：
```
1. browser_navigate http://localhost:5173/admin/commercial/plans（管理后台）
   → 发布/确认 pro v1 已发布（复用 #79 页面）；截图 01-plan-published.png
2. browser_navigate http://localhost:5173/commercial/checkout
   → 断言页面展示：plan 版本 pro v1、¥99.00、行项目「订阅费 ¥99.00」、权益（含高级模型）、
     过期时间（30 分钟内）（AC1）；截图 02-quote-frozen.png
3. 点击「提交购买」
   → 断言：订单区出现、状态「待付款（权益未开通）」、支付跳转链接存在（渠道请求已创建）、
     不出现付费权益已开通文案（AC3）；截图 03-awaiting-payment.png
4. browser_navigate http://localhost:5173/commercial/billing
   → 断言套餐行显示「待付款」（AC3）；截图 04-billing-awaiting.png
5. 回 checkout 重复提交（重试语义）
   → 断言返回同一订单号、无第二条订单；再执行 Step 2 的 g/h 核验（AC4）；截图 05-retry-same-order.png
```
（MCP 不可用时等价 `npx playwright` 脚本：`apps/web` 已有 craft 配置惯例，但不新增 craft 命名空间——直接一次性脚本 `node` + playwright chromium headless 截图即可，不入仓。）

- [ ] **Step 4: 全量回归**

Run: `go test ./... 2>&1 | tail -5 && make lint && make check-backend-architecture && pnpm test:web && pnpm test:shared && pnpm typecheck:web`
Expected: go 全包 ok；golangci-lint 无新告警；架构守卫通过；前端全绿。

- [ ] **Step 5: 证据红线自查（S3）**

Run: `grep -rn "sk_test_\|rk_live_" docs/migrations/lago/t09-quote-invoice/ deploy/lago/evidence/t09-run.txt || echo CLEAN`
Expected: CLEAN（任何命中立即 defuse 为拼接字面量后重查）。

- [ ] **Step 6: 写文档 + Ledger + 提交**

`docs/migrations/lago/t09-quote-invoice/DECISION.md` 必须包含（用户裁决选项 A 强制条件 1 的完整清单，缺项即返工）：
1. spec L121 原文逐字引用（"Before a Channel Payment Order is created, … must match in Plan Version, currency, total, and line items."）；
2. 三处源码实证引用：invoice.rb:100-101（INVISIBLE_STATUS 含 open:5）、invoices_query.rb:122-129（显式 status 与 visible_keys 求交集）、invoices_controller.rb:46-48 与 GraphQL InvoiceResolver 的 `.visible` 过滤（含取得途径：运行容器 /app 内实读）；
3. 替代校验链全貌：付款前权威订阅面三项硬校验（版本/币种/总额）+ 无 charges 切片的单行订阅费推导 + 付款时（finalized 可见后）经 `PurchaseSnapshot.InvoiceFees` 的完整 line-item 复核（#82/#84 消费）；
4. 升级留痕：计划审查第 1 轮发现 → escalate → 用户 2026-09-23 批准选项 A + 四条强制条件原文要点；用户声明将另行向 spec owner 呈报正式修订案；
5. 适用边界：偏差仅限 line-item 付款前比对这一条，不构成其他 spec 条款的先例。

```bash
git add docs/migrations/lago/t09-quote-invoice docs/plans/issue-72-ledger-81.md
git commit -m "docs(lago): #81 t09 quote-invoice evidence and decision record"
# 最终收尾（含全部任务提交）：
git log --oneline codex/issue-72-lago-81 ^f6969fc008   # 核对提交序列
```

---

## 验收标准 → Task → 测试 追踪矩阵

| Issue #81 验收标准 | 实现落点 | 测试（命令/文件） |
|---|---|---|
| AC1 Quote 固定 Plan Version、CNY 金额、权益、行项目与过期时间 | Task 6（order.go 快照/视图）、Task 9/10（契约与页面展示） | `go test ./internal/modules/commercial/service/commercial/ -run TestCreateQuoteFreezes -count=1`；`pnpm test:web`；Task 12 Step 3 浏览器断言 02 |
| AC2 Invoice 与 Quote 不一致时不创建支付请求 | Task 7 `ErrInvoiceQuoteMismatch`（匹配校验先于 `CreateOrder`）、Task 8 409 | `go test ./internal/modules/commercial/service/commercial/ -run TestPurchaseAbortsOnInvoiceMismatch -count=1`（断言渠道 stub 零调用）；`go test ./internal/router/ -run TestPurchaseRoute -count=1`（用例 2：409 + orders 空 + creates==0）；Task 12 Step 2-i 真实 409 |
| AC3 创建成功后产品状态为待付款且 Entitlement 未开放 | Task 1 `PurchaseStateAwaitingPayment`、Task 4 状态映射、Task 7/8 `awaiting_payment`、D4 投影不动 Base | `go test ./internal/modules/commercial/commercialplatform/ -run 'TestLagoPurchaseSnapshot|TestFakePurchaseSnapshot' -count=1`；Task 11 Phase 3 entitlements 404；Task 12 Step 2-d/e + 浏览器断言 03/04 |
| AC4 过期、并发套餐变更和命令重试不产生重复 Invoice 或 Subscription | Task 3 identity read-before-create（F7）+ conflict 分支、Task 7 过期预检/`GetOrderByQuote` 重试返回既有订单 | `go test ./internal/modules/commercial/commercialplatform/ -run 'TestLagoCreatePurchaseReplay|TestLagoCreatePurchaseDifferentPlan' -count=1`；`go test ./internal/modules/commercial/service/commercial/ -run 'TestPurchaseExpired|TestPurchaseRetry|TestPurchaseConcurrent' -count=1`；Task 11 Phase 4/6；Task 12 Step 2-f/g/h（DB 权威计数） |

## Spec 行为矩阵覆盖对照（#81 分片）

- 矩阵 4「Quote match」（spec L217）：AC2 路径（D2 记录 line-item 偏差与补偿）。
- 矩阵 2「Tenant isolation」：Task 11 租户隔离阶段。
- 矩阵 21「Permissions」（部分）：复用路由组既有 billing gate（routes_commercial.go:23-26），Task 8 不新增旁路。
- 矩阵 5「External payment activation」：**不在 #81**（#82/#83），#81 只交付其前置的 incomplete 创建与待付款状态。

## Out of scope（#81 明确不做）

- 付款激活（provider 收款→active）＝#82/#83；异常付款＝#84；充值＝#85；升级/降级语义＝#93/#94。
- 旧本地 Quote/Order/Subscription 表的迁移退役（spec L162-163）＝后续清理票；#81 的购买走新 `PurchaseService`，不动旧链路写路径。
- finalized Invoice 的完整行项目复核（付款时刻）＝#82/#84（D2 补偿 (b)）。
- Webhook/对账＝#98；`Reconcile` 家族继续 fail closed。

## 执行注意事项（并行环境）

- 只在 worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n81`（分支 `codex/issue-72-lago-81`，基线 f6969fc008）提交。
- `deploy/lago/` 与 `deploy/lago-lab/` 只读复用（t02 §7 惯例）；新证据只进 `deploy/lago/evidence/t09-run.txt` 与 `docs/migrations/lago/t09-quote-invoice/`。
- 验证端口：后端 :8080、前端 :5173、Lago :48889/:48890；**避开 :5272/:5273**（其他会话占用）。
- migration 编号：本计划零 migration（全部走 EnsureSchema/AutoMigrate 先例——`commercial_quotes` 等表已存在，快照 JSON 向后兼容）；若实施中发现必须加列，先在 Ledger 登记编号段再动手（waves 文档 L109 三 lane 撞号教训）。
- 凭据纪律：Stripe key / Lago org key 只经环境变量；`~/.zcode/issue72-stripe.env` 只 `source`；任何提交前跑 Task 12 Step 5 红线自查。
