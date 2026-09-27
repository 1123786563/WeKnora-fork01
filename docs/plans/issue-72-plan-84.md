# [Lago 12] 异常付款不会扩大权益 实施计划（Issue #84）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把重复、错金额、错币种、部分付款、晚成功和多个 PaymentAttempt 成功映射为明确的用户/运营状态：异常资金事实被持久化但绝不修改 Invoice 或扩大权益；多个成功只履约一次、其余进入多收款处置；重复通知幂等应答且不重复产生 Lago Payment；Billing UI 清楚区分付款异常、履约处理中与已生效。

**Architecture:** 不动 α 双轨道激活链（#82 交付：渠道真实收款 → WeKnora 驱动 `settle_purchase_payment` gated 结算 → Lago 内建 webhook finalize → active；Lago Payment 由 Lago 自身经 Stripe webhook 记录，WeKnora 永不直写）。本计划在其两侧补「异常面」：①回调/恢复链上被 `ErrPaymentMismatch` 拒绝的已验签事实不再整体丢弃，而是幂等落库到新的 `commercial_payment_anomalies` 表并终态应答（渠道停止重试风暴）；②`over_payment` outbox 事件获得消费方（多收款 → 同表落库 → admin 处置面）；③订单读路径投影 `fulfillment=attention`（契约 token 已存在，后端从未输出）供 UI 区分。零 Commercial Platform seam 改动（ADR-0014；`platform.go:225-234` 三方法签名不变）。

**Tech Stack:** Go（gin + gorm；SQLite 测试库 + PostgreSQL 运行库，参数绑定）；React + Vite（apps/web，`@weknora/contracts` 契约解析器）；Playwright `@playwright/test`（仓库 devDependencies）+ 本会话 playwright MCP 插件（真实浏览器验证）；真实 Lago v1.53.0 compose（:48889/:48890）+ Stripe TEST 结算腿 + 微信 Native v3 / 支付宝 RSA 本地协议 stub（`docs/plans/issue-72-flow-evidence-83/` 既有资产）。

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` —— L127（异常付款保留外部事实，不自动改 Invoice 金额或已交付权益）、L169（operator attention 属闭合产品状态集）、L170-171（公开 Billing API 不暴露平台原始词汇；平台故障期仍持久接受已验签回调）、L220（行为矩阵 #7：duplicate/partial/mismatched/wrong-currency/late/multiple-success 永不重复发放权益）；ADR-0012（`docs/adr/0012-lago-as-commercial-billing-authority.md`，含 2026-09-23 修订）、ADR-0014（`docs/adr/0014-commercial-platform-single-deep-seam.md`，seam 只许加法）；用户裁决 R-4（2026-09-26，α 双轨道）。

## 基线核实（计划员 2026-09-28 于本 worktree HEAD `ee02d3218` 实读实跑）

- 本 worktree 分支 `codex/issue-72-lago-84`，基于集成分支 `ee02d3218`（= merge #82 + OCR 轮）。集成分支无本 Issue 预写稿（`git -C ../lago-int show HEAD:docs/plans/issue-72-plan-84.md` → `path does not exist`），本计划为从零编写。
- #82 已合入：`CommandKindSettlePurchasePayment`（`internal/modules/commercial/purchase_settlement_command.go:25`）、`CurrentPaidAwaitingActivationPurchaseOrder`（`internal/modules/commercial/repository/commercial/order.go:388`）、回调 1 MiB body cap（`internal/handler/payment_callbacks.go:77`）。
- #83 已合入（其计划为复核型，实现在位）：微信 Provider 全链（`payment/wechat.go:435` Query）、`CloseChannelOrder` 关单编排（`service/commercial/order.go:567`）、双渠道回调测试（`payment_callbacks_test.go` WeChat 腿 :452-694）。
- #85 **并行实施中、尚未合入**（DAG 调度边 S1：#85 先行，因双改 `service/commercial/order.go` 订单状态机核心区 + `contracts/commercial.ts`）。**执行前置门：本计划 Task 1 开工前先 `git -C <worktree> merge` 或 rebase 到 #85 合入后的集成分支；若 #85 仍未合入，先跑下方合并纪律核对。**
- 本会话实跑基线（全绿）：
  - `go test ./internal/modules/commercial/repository/commercial/ -count=1` → `ok ... 0.517s`
  - `go test ./internal/handler/ -run 'TestWechatCallback|TestAlipayCallback|TestPaymentCallback' -count=1` → `ok ... 1.139s`
  - `go test ./internal/modules/commercial/service/commercial/ -count=1` → `ok ... 0.790s`

**缺口清单（本计划对象，全部实读核实）：**

| # | 缺口 | 位置 |
|---|------|------|
| G1 | 错金额/错币种/部分付款回调 409 且事务整体回滚，外部资金事实未落库，渠道无限重试 | `repository/commercial/order.go:699-713`（attempt 比对 + `ValidatePayment` → `ErrPaymentMismatch`）→ `handler/payment_callbacks.go:115-118`（409）/`:170-174`（Alipay 409） |
| G2 | 恢复/关单路径用 attempt 金额构造 PaymentFact，未与渠道 Query 实收额比对（Query 响应携带金额但被丢弃） | `service/commercial/order.go:535-539`（Recover）/`:629-633`（Close 决胜）；`payment/wechat.go:443-453`、`payment/alipay.go:462-488` 均不回传 AmountFen（`payment/provider.go:57-61` AttemptResult 无金额字段） |
| G3 | `over_payment` outbox 事件无消费方（drain 只租约 `kind='fulfill'`），多收款不可见、无处置 | `repository/commercial/order.go:777-782`（生产方）；`service/commercial/fulfillment.go:231-234`（租约谓词） |
| G4 | orderWire 永不输出 `fulfillment=attention`，orderMessage 无付款异常分支，UI 不可见 | `internal/handler/commercial.go:284-312`（orderWire）；`apps/web/src/commercial/order-state.ts:2-7`（orderMessage）。契约 parser 已接受 attention token（`packages/contracts/src/commercial.ts:3,36`）——无需改契约 |
| G5 | 晚成功经 RecoverOrderStatus 的幂等无测试（现仅覆盖回调并发） | `service/commercial/order_test.go` 无对应用例 |

## Global Constraints（批准需求原文引用，逐条约束所有 Task）

1. spec L127：「Duplicate, mismatched, partial, wrong-currency, and multiple-success payment cases retain their external facts without automatically changing Invoice amount or delivered benefits.」——异常事实必须落库；Invoice/权益绝不自动扩大。
2. spec L169：「WeKnora maps provider state into stable product states: awaiting payment, paid awaiting activation, active, fulfillment processing, waiting for billing synchronization, insufficient Credits, refund review, channel refund pending, revocation pending, and operator attention.」——attention 是闭合产品状态。
3. spec L170：「The public Billing API never exposes Lago credentials, internal URLs, Lago Organization identifiers, or raw status enums.」——anomaly 面只出 WeKnora 闭合词汇。
4. spec L171：「During a Lago outage, WeKnora … continues to durably accept verified payment callbacks …」——回调面落库不依赖 Lago 可达。
5. spec L220 矩阵 #7：「duplicate, partial, mismatched, wrong-currency, late, and multiple-success payments never duplicate benefits.」
6. spec L123-125：WeKnora 拥有渠道回调验签/查单/关单；受支持 Lago 外部支付集成记录 Payment；**禁止本地强制 Subscription active、禁止伪造 provider webhook、禁止调用 Premium manual `POST /api/v1/payments`、禁止直写 Lago 数据库**——「不重复写 Lago Payment」只能通过不重复驱动 settle 实现，不能通过本地抹除。
7. ADR-0014：seam 三方法签名（`SubmitCommand/ReadSnapshot/Reconcile`，`platform.go:225-234`）冻结不变；本计划**零 seam 改动**。
8. 用户裁决 R-4（2026-09-26）：激活链一律「渠道真实收款 + WeKnora 驱动 Stripe Provider gated 结算 → Lago 内建 finalize → active」；渠道沙箱凭据缺失为已披露边界，用本地 RSA stub 证明协议往返+验签，不得伪造沙箱证据。
9. 安全约束（实现验收条件）：服务端出站请求仅 http/https 且 host 校验拒绝 localhost/环回/私有/保留地址（环回 stub 需显式 `SSRF_WHITELIST_EXTRA=127.0.0.1` 豁免，`payment/wechat.go` `do`/`alipay.go` `call` 既有惯例）；数据库查询一律参数绑定（`resolveByMerchantOrderID` `payment_callbacks.go:48-51` 与本计划全部新查询遵守）；凭据只从环境变量/密钥服务读取，源码/示例/测试不得写入可用凭据字面量（Stripe key 仅 `source ~/.zcode/issue72-stripe.env`）。
10. 端口纪律：真实验证环境避开 `:5272/:5273`（其他会话占用中）及既往占用口径 `:8080/:8091-8095/:5183/:5192-5196/:8291-8297`；本计划分配：后端 `:8096`、前端 `:5197`、微信 stub `:8298`、支付宝 stub `:8299`（起栈前 `lsof -iTCP:<port> -sTCP:LISTEN` 确认空闲，被占则整体 +10 顺延并同步脚本 env）。
11. 与 #85 的合并纪律（S1 边）：`service/commercial/order.go` 为双改文件——本计划只动 `RecoverOrderStatus`/`CloseChannelOrder`/`OrderView` 投影区，不碰 #85 的充值批次状态机新增区；`packages/contracts/src/commercial.ts` 本计划**零改动**（attention token 已在 `OrderView.fulfillment` 闭集内）。#85 合入后先跑 `go test ./internal/modules/commercial/... -count=1` 再开工。

## Review Focus（spec 隐含但无 Task 测试覆盖、最可能咬人的五类输入；每行已归属 owning Task）

1. **已验签但非 succeeded 状态的通知**（如 `trade_status=TRADE_CLOSED`）：既非履约也非异常收款——维持 409/非 2xx 既有行为（渠道重试是正确姿态），不落 anomaly、不履约 → Task 2 测试 `TestWechatCallbackNonSucceededStatusStaysRejectedNoAnomaly`。
2. **anomaly 落库失败时既无事实又终态应答**：`RecordPaymentAnomaly` DB 故障必须让回调面回退非 2xx（渠道重试兜底），绝不能 200 却无事实 → Task 1 测试 `TestConfirmPaymentMismatchAnomalyInsertFailurePropagates` + Task 2 集成断言。
3. **anomaly 唯一键幂等**：同 `(provider, merchant, transaction)` 的重复回调 / over_payment 事件重复消费不得产生第二行 → Task 1 测试 `TestRecordPaymentAnomalyIdempotentOnUniqueKey`、Task 3 测试 `TestOverPaymentDrainReplayYieldsSingleAnomaly`。
4. **attention 投影误伤终态**：已 fulfilled/active 订单若残留 resolved anomaly 不得显示付款异常；anomaly resolve 后 attention 消失 → Task 5 测试 `TestOrderWireAttentionOnlyWhileUnresolved` + `TestResolvePaymentAnomalyClearsAttention`。
5. **over_payment 消费不得阻塞共享 drain**：单条 dispose 失败中止整批会饿死普通 fulfill 事件（A-32 同型纪律）→ Task 3 测试 `TestOverPaymentDisposeFailureDoesNotBlockFulfillDrain`。

---

## File Structure

- **Create** `internal/modules/commercial/repository/commercial/payment_anomaly.go` —— `PaymentAnomalyRow` 模型 + 闭合 kind/state 常量 + `ClassifyPaymentAnomaly` + `RecordPaymentAnomaly`/`HasUnresolvedPaymentAnomaly`/`ListPaymentAnomalies`/`ResolvePaymentAnomaly`（全部参数绑定）。
- **Create** `internal/modules/commercial/repository/commercial/payment_anomaly_test.go` —— 表级幂等/分类/处置测试。
- **Modify** `internal/modules/commercial/repository/commercial/order.go` —— `ConfirmPayment` 两个 mismatch 检测点携带 anomaly 快照出事务、独立事务落库（G1）。
- **Modify** `internal/modules/commercial/repository/commercial/order_test.go` —— mismatch 保留事实/不履约/幂等测试。
- **Modify** `internal/handler/payment_callbacks.go` —— `ErrPaymentMismatch` 分支改终态幂等 200 应答（G1 收口）。
- **Modify** `internal/handler/payment_callbacks_test.go` —— 改造既有 mismatch 测试 + 新增幂等/非 succeeded 维持测试。
- **Modify** `internal/modules/commercial/payment/provider.go` —— `AttemptResult` 增 `AmountFen int64`（加法，0=渠道未报告）。
- **Modify** `internal/modules/commercial/payment/wechat.go` / `alipay.go` —— Query 回传实收金额（G2）。
- **Modify** `internal/modules/commercial/payment/wechat_native_test.go` / `alipay_test.go` —— Query 金额回传测试。
- **Modify** `internal/modules/commercial/service/commercial/order.go` —— Recover/Close 两处 succeeded 分支先比实收额，不符走 anomaly（G2）；`OrderView` 增 `PaymentAttention` 并在读路径填充（G4 后端半）。
- **Modify** `internal/modules/commercial/service/commercial/order_test.go` —— 恢复路径金额比对/晚成功幂等测试（G5）。
- **Modify** `internal/modules/commercial/service/commercial/fulfillment.go` —— drain 租约扩展 over_payment kind + `disposeOverPayment`（G3）。
- **Modify** `internal/modules/commercial/service/commercial/fulfillment_test.go` —— 消费/幂等/不阻塞测试。
- **Modify** `internal/handler/commercial.go` —— orderWire 输出 `attention`；新增 `AdminListPaymentAnomalies`/`AdminResolvePaymentAnomaly` handler。
- **Modify** `internal/router/routes_commercial.go` —— 挂 `/admin/payment-anomalies` 组（直挂父组 + 平台守卫，refund-review 先例 :77-83）。
- **Modify** `apps/web/src/commercial/order-state.ts` + `order-state.test.ts` —— orderMessage 增 attention 分支（G4 前端半）。
- **Modify** `apps/web/src/commercial/BillingPage.tsx` —— 购买行后缀读 `purchase.order?.fulfillment==='attention'`。
- **Create** `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go` 内追加用例 —— active 后第二 settle 短路、Lago payments 不增（AC3 权威侧）。
- **Create** `docs/plans/issue-72-flow-evidence-84/` —— 真实栈验证脚本与证据（Task 7）。

---

### Task 1: 异常付款事实持久化（repository 层）

**Files:**
- Create: `internal/modules/commercial/repository/commercial/payment_anomaly.go`
- Test: `internal/modules/commercial/repository/commercial/payment_anomaly_test.go`
- Modify: `internal/modules/commercial/repository/commercial/order.go:688-788`（ConfirmPayment）
- Test: `internal/modules/commercial/repository/commercial/order_test.go`

**Interfaces:**
- Consumes: `domain.PaymentFact`（`internal/modules/commercial/order.go:39-49`：TenantID/OrderID/AttemptID/Provider/Merchant/Transaction/Amount/Currency/State）、`domain.ErrPaymentMismatch`（order.go:25）、既有测试 harness（order_test.go 的 sqlite 内存库构造形态）。
- Produces（后续 Task 依赖的确切签名）:
```go
// payment_anomaly.go
type PaymentAnomalyRow struct {
    ID               string     `gorm:"primaryKey;column:id"` // "anom_"+newLeaseToken()
    TenantID         uint64     `gorm:"column:tenant_id;not null"`
    OrderID          string     `gorm:"column:order_id;not null;index"`
    AttemptID        string     `gorm:"column:attempt_id;not null"` // merchant_order_id
    Provider         string     `gorm:"column:provider;not null"`
    Merchant         string     `gorm:"column:merchant;not null"`
    Transaction      string     `gorm:"column:transaction;not null"`
    Kind             string     `gorm:"column:kind;not null"`
    ExpectedAmountFen int64     `gorm:"column:expected_amount_fen;not null"`
    ActualAmountFen  int64     `gorm:"column:actual_amount_fen;not null"`
    ExpectedCurrency string     `gorm:"column:expected_currency;not null"`
    ActualCurrency   string     `gorm:"column:actual_currency;not null"`
    State            string     `gorm:"column:state;not null;default:'awaiting_disposition'"`
    Version          int64      `gorm:"column:version;not null;default:1"`
    CreatedAt        time.Time  `gorm:"column:created_at"`
    ResolvedAt       *time.Time `gorm:"column:resolved_at"`
}
func (PaymentAnomalyRow) TableName() string { return "commercial_payment_anomalies" }
// 唯一索引（gorm tag 于 Transaction 字段组合实现，见 Step 3）: uq_payment_anomaly_txn ON (provider, merchant, transaction)

const (
    PaymentAnomalyKindPartial   = "partial_payment"    // actual < expected
    PaymentAnomalyKindAmount    = "amount_mismatch"    // actual ≠ expected（含 actual > expected 的单笔错额）
    PaymentAnomalyKindCurrency  = "currency_mismatch"
    PaymentAnomalyKindOverPaid  = "over_payment"       // 多成功/晚成功第二笔
    PaymentAnomalyStateAwaiting = "awaiting_disposition"
    PaymentAnomalyStateResolved = "resolved"
)

// ClassifyPaymentAnomaly: 闭合分类。币种不符→currency_mismatch；actual<expected→partial_payment；
// 其余→amount_mismatch。over_payment 由消费方显式指定，不经此函数。
func ClassifyPaymentAnomaly(expectedFen, actualFen int64, expectedCurrency, actualCurrency string) string

// RecordPaymentAnomaly 幂等落库：独立事务；唯一索引冲突视为已记录返回 nil。
func (s *OrderStore) RecordPaymentAnomaly(ctx context.Context, row PaymentAnomalyRow) error
// HasUnresolvedPaymentAnomaly: state='awaiting_disposition' 且 order_id 命中。参数绑定。
func (s *OrderStore) HasUnresolvedPaymentAnomaly(ctx context.Context, orderID string) (bool, error)
// ListPaymentAnomalies: admin 全量，created_at DESC。参数绑定。
func (s *OrderStore) ListPaymentAnomalies(ctx context.Context) ([]PaymentAnomalyRow, error)
// ResolvePaymentAnomaly: state+version 守卫翻 resolved（RowsAffected==0 → ErrPaymentAnomalyNotFound）。
func (s *OrderStore) ResolvePaymentAnomaly(ctx context.Context, id string) (PaymentAnomalyRow, error)

var ErrPaymentAnomalyNotFound = errors.New("payment_anomaly_not_found")
```
- ConfirmPayment 行为契约变更（对外签名不变）：mismatch（attempt 对比 :699-702 或 `ValidatePayment` :711-713 任一失败）时，事务照旧回滚（不履约、attempt 不动、无 outbox 事件），**事务外**用快照独立事务 `RecordPaymentAnomaly`；落库失败则返回该错误（覆盖 ErrPaymentMismatch 上抛），成功则仍返回 `ErrPaymentMismatch`。

- [ ] **Step 1: 写失败测试（payment_anomaly_test.go）**

```go
func TestClassifyPaymentAnomalyClosedKinds(t *testing.T) {
    if got := ClassifyPaymentAnomaly(9900, 5000, "CNY", "CNY"); got != PaymentAnomalyKindPartial {
        t.Fatalf("partial: got %s", got)
    }
    if got := ClassifyPaymentAnomaly(9900, 19900, "CNY", "CNY"); got != PaymentAnomalyKindAmount {
        t.Fatalf("amount: got %s", got)
    }
    if got := ClassifyPaymentAnomaly(9900, 9900, "CNY", "USD"); got != PaymentAnomalyKindCurrency {
        t.Fatalf("currency: got %s", got)
    }
}

func TestRecordPaymentAnomalyIdempotentOnUniqueKey(t *testing.T) {
    db, store := newOrderTestDB(t) // 既有 harness 形态：sqlite 内存 + AutoMigrate(PaymentAnomalyRow)
    row := PaymentAnomalyRow{ID: "anom_1", TenantID: 7, OrderID: "ord_1", AttemptID: "mo_1",
        Provider: "wechat", Merchant: "mch", Transaction: "txn_a",
        Kind: PaymentAnomalyKindPartial, ExpectedAmountFen: 9900, ActualAmountFen: 5000,
        ExpectedCurrency: "CNY", ActualCurrency: "CNY"}
    if err := store.RecordPaymentAnomaly(ctx, row); err != nil { t.Fatal(err) }
    row.ID = "anom_2" // 同 (provider, merchant, transaction) 不同主键 → 唯一冲突 → nil
    if err := store.RecordPaymentAnomaly(ctx, row); err != nil { t.Fatal(err) }
    var n int64
    db.Model(&PaymentAnomalyRow{}).Count(&n)
    if n != 1 { t.Fatalf("want 1 row, got %d", n) }
    ok, err := store.HasUnresolvedPaymentAnomaly(ctx, "ord_1")
    if err != nil || !ok { t.Fatalf("want unresolved, got %v %v", ok, err) }
}
```

order_test.go 追加（RED：现状 mismatch 整体回滚、无表）：

```go
func TestConfirmPaymentMismatchRetainsExternalFactWithoutFulfillment(t *testing.T) {
    // 既有 harness：openOrder 建单 + RegisterAttempt（沿用 TestPaymentConfirmRejectsUnregisteredOrMismatchedMerchant 的搭建，order_test.go:483）
    fact := domain.PaymentFact{Provider: "wechat", Merchant: att.Merchant, AttemptID: att.MerchantOrderID,
        OrderID: order.ID, TenantID: 1, Amount: domain.CNYFen(5000), Currency: "CNY",
        Transaction: "txn_short", State: "succeeded"}
    err := store.ConfirmPayment(ctx, fact)
    if !errors.Is(err, domain.ErrPaymentMismatch) { t.Fatalf("want ErrPaymentMismatch, got %v", err) }
    // G1 断言：异常事实已落库
    ok, _ := store.HasUnresolvedPaymentAnomaly(ctx, order.ID)
    if !ok { t.Fatal("mismatch fact must be retained as unresolved anomaly") }
    // 不履约断言：订单仍 pending、无 fulfill outbox 事件、attempt 仍 pending
    row, _ := store.GetOrder(ctx, order.ID)
    if row.State != domain.OrderStatePending { t.Fatalf("state %s", row.State) }
    if n := countOutbox(t, db, OutboxKindFulfill); n != 0 { t.Fatalf("fulfill events %d", n) }
}

func TestConfirmPaymentMismatchAnomalyInsertFailurePropagates(t *testing.T) {
    // 预置同 (provider,merchant,transaction) 但 Kind 不同的占用行无法触发（唯一冲突→nil），
    // 故用 DropTable 使插入报错：断言 ConfirmPayment 返回非 ErrPaymentMismatch 的错误
    // （调用方因此回退非 2xx，渠道重试兜底——Review Focus 2）。
    db.Migrator().DropTable(&PaymentAnomalyRow{})
    err := store.ConfirmPayment(ctx, mismatchedFact)
    if err == nil || errors.Is(err, domain.ErrPaymentMismatch) { t.Fatalf("want raw insert error, got %v", err) }
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/repository/commercial/ -run 'TestClassifyPaymentAnomaly|TestRecordPaymentAnomaly|TestConfirmPaymentMismatch' -count=1 -v`
Expected: FAIL（`undefined: PaymentAnomalyRow` / `HasUnresolvedPaymentAnomaly`；mismatch 用例在无表处报错）。

- [ ] **Step 3: 实现**

新建 `payment_anomaly.go` 按上方签名。AutoMigrate 挂点：`NewOrderService`（`service/commercial/order.go:105` 既有 `db.AutoMigrate(&OrderRow{}, &Subscription{})` 追加 `&repocommercial.PaymentAnomalyRow{}`）+ 测试 harness。唯一索引用 gorm struct tag：`Transaction string \`gorm:"column:transaction;not null;uniqueIndex:uq_payment_anomaly_txn,priority:3"\``（Provider priority:1、Merchant priority:2）。`RecordPaymentAnomaly` 检测 `isUniqueConflict(err)`（复用 order.go:192-221 冲突匹配形态，新增匹配 `uq_payment_anomaly_txn`）→ 返回 nil。`ResolvePaymentAnomaly` 用 `WHERE id = ? AND state = ? AND version = ?` 参数绑定更新 `state/version+1/resolved_at`。

`ConfirmPayment` 改造（order.go:688 起）：方法体开头声明 `var anomaly *PaymentAnomalyRow`；两个 mismatch 返回点改为 `anomaly = buildMismatchAnomaly(attempt, row, fact); return domain.ErrPaymentMismatch`（`buildMismatchAnomaly` 为包内函数：以 attempt.AmountFen/Currency 为 expected、fact 为 actual、`ClassifyPaymentAnomaly` 定 kind，ID 生成 `anom_`+随机 hex——注意随机 ID 须在**事务外**落库时生成，避免回滚后重试换 ID，直接在 RecordPaymentAnomaly 内部当 row.ID=="" 时生成）；事务闭包返回后：
```go
if err != nil && anomaly != nil && errors.Is(err, domain.ErrPaymentMismatch) {
    if rerr := s.RecordPaymentAnomaly(ctx, *anomaly); rerr != nil { return rerr }
}
return err
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/modules/commercial/repository/commercial/ -count=1`
Expected: PASS（含既有 19 个 order 用例不回归）。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/commercial/repository/commercial/payment_anomaly.go internal/modules/commercial/repository/commercial/payment_anomaly_test.go internal/modules/commercial/repository/commercial/order.go internal/modules/commercial/repository/commercial/order_test.go internal/modules/commercial/service/commercial/order.go
git commit -m "issue-72(#84): retain abnormal payment facts (task1)"
```

---

### Task 2: 回调面终态幂等应答

**Files:**
- Modify: `internal/handler/payment_callbacks.go:115-122`（WeChat 分支）、`:170-177`（Alipay 分支）
- Test: `internal/handler/payment_callbacks_test.go`（改造 :682 既有用例 + 新增）

**Interfaces:**
- Consumes: Task 1 `RecordPaymentAnomaly`（经 ConfirmPayment 内联）与 `ErrPaymentMismatch` 落库语义。
- Produces: 回调面行为契约——已验签但 mismatch 的通知：HTTP 200 终态应答（WeChat `{"code":"SUCCESS","message":"OK"}` / Alipay 纯文本 `success`），事实已在库；重复投递同通知 → 同样 200，anomaly 行数不变。非 succeeded 状态通知维持非 2xx（Review Focus 1）。

- [ ] **Step 1: 写失败测试**

改造 `TestWechatCallbackAmountMismatchRejectedNoFact`（payment_callbacks_test.go:682）为 `TestWechatCallbackAmountMismatchRecordsAnomalyAndAcksIdempotently`，并新增两例（沿用该文件既有 WeChat stub 构造形态 :452-494 与 Alipay 形态 :172-219）：

```go
func TestWechatCallbackAmountMismatchRecordsAnomalyAndAcksIdempotent(t *testing.T) {
    // 既有 harness：注册 attempt（金额 9900），stub 验签通知携带 total=5000
    resp := postWechatNotify(t, notifyWithAmount(5000)) // 复用既有构造器
    if resp.Code != http.StatusOK { t.Fatalf("want 200 terminal ack, got %d", resp.Code) }
    if n := countAnomalies(t, db); n != 1 { t.Fatalf("anomalies %d", n) }
    resp2 := postWechatNotify(t, notifyWithAmount(5000)) // 重投
    if resp2.Code != http.StatusOK { t.Fatalf("replay want 200, got %d", resp2.Code) }
    if n := countAnomalies(t, db); n != 1 { t.Fatalf("replay anomalies %d", n) }
    // 订单仍 pending、无 fulfill 事件（AC1）
    assertOrderPendingNoFulfillEvent(t, db, orderID)
}

func TestAlipayCallbackAmountMismatchRecordsAnomalyAndAcksSuccess(t *testing.T) {
    // Alipay 腿：total_amount 篡改为 50.00 → 断言 body=="success"、状态 200、anomaly 1 行、订单 pending
}

func TestWechatCallbackNonSucceededStatusStaysRejectedNoAnomaly(t *testing.T) {
    // trade_state=TRADE_CLOSED 的已验签通知：断言非 2xx（409 形态不变）、零 anomaly 行、订单 pending（Review Focus 1）
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/handler/ -run 'TestWechatCallbackAmountMismatch|TestAlipayCallbackAmountMismatch|TestWechatCallbackNonSucceeded' -count=1 -v`
Expected: FAIL（现状 409；重命名用例的 200 断言失败；NonSucceeded 用例可能直接 PASS——若 PASS 保留为回归锁定）。

- [ ] **Step 3: 实现**

WeChat 分支（payment_callbacks.go:115-119）：
```go
if err := h.orders.ConfirmPayment(c.Request.Context(), fact); err != nil {
    if errors.Is(err, domain.ErrPaymentMismatch) {
        // (G1/#84) 事实已独立事务落库（Task 1）：终态应答停止渠道重试风暴；
        // 落库失败（非 ErrPaymentMismatch 的原始错误）走 500 分支让渠道重试兜底。
        c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "OK"})
        return
    }
    callbackFail(c, http.StatusInternalServerError, "payment confirmation failed")
    return
}
```
Alipay 分支（:170-174）同型：`alipayFail` 之外新增 mismatch → `c.String(http.StatusOK, "success")`。两分支注释引用 spec L127/L171。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/handler/ -run 'TestWechatCallback|TestAlipayCallback|TestPaymentCallback' -count=1`
Expected: PASS（含既有 `TestWechatCallbackDuplicateDeliveryIdempotent` :562、`TestWechatCallbackDifferentTransactionOverPaidAudit` :592 不回归）。

- [ ] **Step 5: 提交**

```bash
git add internal/handler/payment_callbacks.go internal/handler/payment_callbacks_test.go
git commit -m "issue-72(#84): idempotent terminal ack for abnormal callbacks (task2)"
```

---

### Task 3: over_payment 消费方与多收款处置面

**Files:**
- Modify: `internal/modules/commercial/service/commercial/fulfillment.go:231-234`（租约谓词）+ 新增 `disposeOverPayment`
- Modify: `internal/handler/commercial.go`（新增两个 admin handler）
- Modify: `internal/router/routes_commercial.go`（:83 后追加路由组）
- Test: `internal/modules/commercial/service/commercial/fulfillment_test.go`、`internal/handler/commercial_test.go`（admin 用例落点；若该文件过大则新建 `internal/handler/commercial_anomaly_test.go`）

**Interfaces:**
- Consumes: Task 1 `RecordPaymentAnomaly`/`ListPaymentAnomalies`/`ResolvePaymentAnomaly`/`PaymentAnomalyKindOverPaid`；既有 `paymentEventPayload`（repository order.go:658-669，含 OrderID/TenantID/AttemptID/Provider/Merchant/Transaction/AmountFen/Currency）、`completeEvent`/租约机制（fulfillment.go:229-262）、`RequirePlatformRefundReviewer` 守卫先例（routes_commercial.go:81）。
- Produces:
  - drain 行为契约：`leasePendingEvents` 谓词改 `kind IN (?, ?)`（fulfill, over_payment）；`disposeOverPayment(ctx, ev, now) error`——解析 payload → `RecordPaymentAnomaly(kind=over_payment, ExpectedAmountFen=0, ActualAmountFen=payload.AmountFen, 币种照 payload)`（ExpectedAmountFen=0 表示「非单笔错额、是多收款第二笔」，expected 面在订单行上）→ `completeEvent(sent)`；RecordPaymentAnomaly 返回错误时事件保持 pending（下一轮重试），**不中止同批 fulfill 事件**（Review Focus 5）。
  - admin 端点：`GET /api/v1/commercial/admin/payment-anomalies`（`AdminListPaymentAnomalies`，返回 `success:true,data:[{id,order_id,tenant_id,provider,kind,expected_amount_fen,actual_amount_fen,currency,state,version,created_at}]`，闭合词汇）与 `POST /api/v1/commercial/admin/payment-anomalies/:id/resolve`（`AdminResolvePaymentAnomaly`，body `{"expected_version": <int>}`，409 `{"error":"anomaly changed since read"}` / 404）。挂载：`r.Group("/admin/payment-anomalies", commercialHandler.RequirePlatformRefundReviewer())` 直挂父组（:77-83 refund 先例）。

- [ ] **Step 1: 写失败测试**

fulfillment_test.go（沿用既有 fake 平台 + sqlite harness）：

```go
func TestOverPaymentDrainConsumesEventIntoAwaitingDisposal(t *testing.T) {
    // 搭建：订单已 paid（ConfirmPayment 正常腿）→ 再以第二渠道 transaction 走 ConfirmPayment
    // （TestPaymentConcurrentChannelsYieldSingleFulfillmentPlusAudit 形态，order_test.go:202）
    // → outbox 出现 kind=over_payment 事件 → 驱动一轮 drain（既有 drainOne/harness 入口）
    ok, _ := store.HasUnresolvedPaymentAnomaly(ctx, order.ID)
    if !ok { t.Fatal("over_payment must land as unresolved anomaly") }
    assertOutboxState(t, db, OutboxKindOverPaid, overEventKey, OutboxStateSent)
    // 权益不扩大：fulfillment_records purchase_activation 仍恰 1 行 applied
}

func TestOverPaymentDrainReplayYieldsSingleAnomaly(t *testing.T) {
    // 手工把同一 over_payment 事件重置为 pending 再 drain 一轮 → anomaly 行数仍 1（Review Focus 3）
}

func TestOverPaymentDisposeFailureDoesNotBlockFulfillDrain(t *testing.T) {
    // DropTable commercial_payment_anomalies + 队列中同时放一个 fulfill 事件与一个 over_payment 事件
    // → drain 一轮：fulfill 事件仍被处理（订单推进），over_payment 事件仍 pending，整体不 panic
}
```

handler 测试（`commercial_anomaly_test.go`）：

```go
func TestAdminPaymentAnomaliesListAndResolve(t *testing.T) {
    // 平台守卫路由（既有 admin 测试形态）：种 2 行 anomaly（1 awaiting 1 resolved）
    // GET → data 长度 2、闭合字段名断言
    // POST resolve {expected_version:1} → 200 state=resolved；重复 resolve 同 version → 409
}
func TestAdminPaymentAnomaliesRequiresPlatformReviewer(t *testing.T) {
    // 无守卫凭据 GET → 403（space Admin 亦 403）
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/service/commercial/ -run 'TestOverPayment' -count=1 -v && go test ./internal/handler/ -run 'TestAdminPaymentAnomalies' -count=1 -v`
Expected: FAIL（drain 不租约 over_payment → HasUnresolved false；admin 路由 404）。

- [ ] **Step 3: 实现**

`leasePendingEvents` 谓词 `.Where("kind IN ? AND state = ? AND lease_until <= ?", []string{OutboxKindFulfill, OutboxKindOverPaid}, ...)`；drain 循环内按 `ev.Kind` 分派：fulfill → 既有 `fulfillEvent`；over_payment → `disposeOverPayment`（新私有方法，payload 解析失败 → `completeEvent(sent)` 丢弃畸形事件并 log Warn——不可重试的确定性畸形；RecordPaymentAnomaly 错误 → return err 保守保持 pending 且由循环既有 per-event 错误隔离纪律处理，若当前循环是 fail-fast 则在该分派点捕获错误继续下一事件，测试锁定行为）。handler 与路由按 Produces 签名落文件。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/modules/commercial/service/commercial/ -count=1 && go test ./internal/handler/ -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/commercial/service/commercial/fulfillment.go internal/modules/commercial/service/commercial/fulfillment_test.go internal/handler/commercial.go internal/handler/commercial_anomaly_test.go internal/router/routes_commercial.go
git commit -m "issue-72(#84): over_payment disposal drain + admin surface (task3)"
```

---

### Task 4: 恢复/关单路径实收金额比对

**Files:**
- Modify: `internal/modules/commercial/payment/provider.go:57-61`（AttemptResult）
- Modify: `internal/modules/commercial/payment/wechat.go:435-454`（Query）、`alipay.go:462-489`（Query）
- Modify: `internal/modules/commercial/service/commercial/order.go:526-547`（Recover）、`:617-643`（Close 决胜）
- Test: `internal/modules/commercial/payment/wechat_native_test.go`、`alipay_test.go`、`internal/modules/commercial/service/commercial/order_test.go`

**Interfaces:**
- Consumes: Task 1 `RecordPaymentAnomaly`/`ClassifyPaymentAnomaly`；`ParseCNYAmount`（alipay.go 既有）；wechat 响应结构 `wechatTransactionResponse.Amount.Total`（wechat.go:368-373）、`alipayQueryResponse.TotalAmount`（alipay.go:374/392）。
- Produces:
```go
// provider.go —— 加法字段（0 = 渠道未报告金额，比对跳过）
type AttemptResult struct {
    State       AttemptState
    ProviderID  string
    CheckoutURL string
    AmountFen   int64 // #84: 查单实收额（WECHAT amount.total / ALIPAY total_amount）
}
```
- Recover/Close 行为契约：`res.State == StateSucceeded` 时若 `res.AmountFen > 0 && res.AmountFen != att.AmountFen` → **不 ConfirmPayment**，改 `RecordPaymentAnomaly`（expected=att.AmountFen, actual=res.AmountFen, Transaction=res.ProviderID, Classify 定 kind），返回的 OrderView 标 `PaymentAttention: true`（见 Task 5 字段——本 Task 先落比对与 anomaly，attention 字段随 Task 5 接线；两 Task 顺序执行时在本 Task 一并置位亦可，以 Task 5 的测试为最终断言）。

- [ ] **Step 1: 写失败测试**

payment 层：

```go
// wechat_native_test.go
func TestWechatQueryReportsCollectedAmount(t *testing.T) {
    // 既有 httptest stub 形态：响应体 amount.total=5000 → AttemptResult.AmountFen==5000
}
// alipay_test.go
func TestAlipayQueryReportsCollectedAmount(t *testing.T) {
    // 响应 total_amount="50.00" → AmountFen==5000
}
```

service 层（order_test.go，复用既有 fake provider harness）：

```go
func TestRecoverOrderStatusAmountMismatchRecordsAnomalyWithoutConfirm(t *testing.T) {
    // attempt 注册 9900；fake provider Query 返回 succeeded + AmountFen 5000
    view, err := svc.RecoverOrderStatus(ctx, 1, order.ID)
    if err != nil { t.Fatal(err) }
    if row.State == domain.OrderStatePaid { t.Fatal("wrong-amount recovery must not confirm") }
    ok, _ := store.HasUnresolvedPaymentAnomaly(ctx, order.ID)
    if !ok { t.Fatal("query-path mismatch must retain fact") }
}

func TestRecoverOrderStatusLateSuccessAfterFulfilledIsIdempotentOverPayment(t *testing.T) {
    // G5：订单已 fulfilled（正常链走完）→ 第二渠道 attempt late succeeded
    // → RecoverOrderStatus/ConfirmPayment 不产生第二个 fulfill 事件、不二次履约，
    //   over_payment 事件出现（由 Task 3 drain 消费为 anomaly）
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/payment/ -run 'TestWechatQueryReports|TestAlipayQueryReports' -count=1 -v && go test ./internal/modules/commercial/service/commercial/ -run 'TestRecoverOrderStatus' -count=1 -v`
Expected: FAIL（AmountFen 字段不存在编译错 → 先加字段后测试转 FAIL 于断言；recover 用例现状会把 5000 场景按 attempt 金额确认成功——正是 G2 的行为证据）。

- [ ] **Step 3: 实现**

provider.go 加字段（默认零值兼容既有调用/实现）；wechat Query 返回处 `AmountFen: out.Amount.Total`；alipay Query `fen, err := ParseCNYAmount(out.TotalAmount)`（err → 保持 AmountFen=0 并不失败——金额缺失不阻断状态映射）。service 两处 succeeded 分支前插比对（见 Produces 契约），不符走 anomaly；`PaymentAttention` 置位逻辑与 Task 5 的 OrderView 字段联动（本 Task 提交内置字段亦可，Task 5 完成投影消费）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/modules/commercial/payment/ -count=1 && go test ./internal/modules/commercial/service/commercial/ -count=1`
Expected: PASS（既有 `TestWechatQueryMapsTradeStates`、`TestWechatQueryPrefersTransactionID`、`TestAlipayQuery*` 不回归——AmountFen 为加法）。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/commercial/payment/provider.go internal/modules/commercial/payment/wechat.go internal/modules/commercial/payment/alipay.go internal/modules/commercial/payment/wechat_native_test.go internal/modules/commercial/payment/alipay_test.go internal/modules/commercial/service/commercial/order.go internal/modules/commercial/service/commercial/order_test.go
git commit -m "issue-72(#84): compare collected amount on recovery paths (task4)"
```

---

### Task 5: 付款异常 UI 投影（attention 通道打通）

**Files:**
- Modify: `internal/modules/commercial/service/commercial/order.go`（OrderView 结构 :287-298 + Recover/GetOrder 读路径）
- Modify: `internal/modules/commercial/service/commercial/purchase.go:539-553`（orderViewFromRow / PurchaseStatus 投影）
- Modify: `internal/handler/commercial.go:284-312`（orderWire）
- Modify: `apps/web/src/commercial/order-state.ts`、`apps/web/src/commercial/BillingPage.tsx:117-122`
- Test: `internal/handler/commercial_test.go`（orderWire 用例落点）、`apps/web/src/commercial/order-state.test.ts`

**Interfaces:**
- Consumes: Task 1 `HasUnresolvedPaymentAnomaly`；契约 parser 既有 attention token（`packages/contracts/src/commercial.ts:3,36`——**契约文件零改动**）。
- Produces:
```go
// OrderView 加法字段（JSON: payment_attention，omitempty）
PaymentAttention bool `json:"payment_attention,omitempty"`
```
- orderWire 行为契约：`o.PaymentAttention == true` 时 `fulfillment = "attention"`（覆盖 pending 默认；payment 轴保持真实——错额单未正确收款仍 pending，已付款多收单为 paid）。`PurchaseStatus` 的 order 投影同源填充（`purchase.order.fulfillment` 不直接出 attention——purchase 轴五态闭集不动，attention 走 order 子对象）。
- 前端：`orderMessage` 增分支（闭合文案，无平台词汇，spec L210）：`if(order.fulfillment==='attention') return '付款异常（资金事实已记录，待处理）';`——置于 fulfilled 判定之后、paid 之前（attention 优先级高于处理中）。BillingPage 购买行后缀：`purchase.order?.fulfillment==='attention'` → 追加 `' · 付款异常（待处理）'`。

- [ ] **Step 1: 写失败测试**

后端（handler 层，stub service 或直构 OrderView 走 orderWire——照既有 orderWire 测试形态；若 orderWire 无直测则在 commercial_test.go 新增表驱动小用例）：

```go
func TestOrderWireProjectsAttentionWhileUnresolved(t *testing.T) {
    w := orderWire(commercialsvc.OrderView{ID: "o1", State: "pending", AmountFen: 9900, Currency: "CNY", PaymentAttention: true})
    if w["fulfillment"] != "attention" { t.Fatalf("got %v", w["fulfillment"]) }
    // 已 fulfilled 且 anomaly resolved → 不再 attention（Review Focus 4 反向：
    // Service 层 HasUnresolved=false → PaymentAttention=false → fulfillment=fulfilled）
}
```

service 层：`TestGetOrderSurfacesUnresolvedAnomaly`（种 anomaly 行 → RecoverOrderStatus 返回 view.PaymentAttention=true；`ResolvePaymentAnomaly` 后再读 → false）。

前端（order-state.test.ts，node --test 形态）：

```ts
test('orderMessage distinguishes anomaly vs processing vs fulfilled', () => {
  assert.equal(orderMessage({ id:'o', payment:'pending', fulfillment:'attention', amount_fen:'9900', currency:'CNY' }), '付款异常（资金事实已记录，待处理）');
  assert.equal(orderMessage({ id:'o', payment:'paid', fulfillment:'processing', amount_fen:'9900', currency:'CNY' }), '已付款，权益处理中');
  assert.equal(orderMessage({ id:'o', payment:'paid', fulfillment:'fulfilled', amount_fen:'9900', currency:'CNY' }), '权益已生效');
});
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd apps/web && npx tsx --test src/commercial/order-state.test.ts`（或根目录 `pnpm test:web`）与 `go test ./internal/handler/ -run TestOrderWire -count=1 -v`、`go test ./internal/modules/commercial/service/commercial/ -run TestGetOrderSurfaces -count=1 -v`
Expected: FAIL（orderMessage 现 fallback「等待付款」；PaymentAttention 字段不存在）。

- [ ] **Step 3: 实现**

按 Produces：OrderView 加字段；`RecoverOrderStatus` 与 `GetOrder` 相关读路径（含 purchase.go `orderViewFromRow` 调用处）追加 `HasUnresolvedPaymentAnomaly` 查询（单条参数绑定索引查询，量级：每订单页轮询一次，可接受；不做 join）；orderWire 加三行分派；order-state.ts 加分支；BillingPage 后缀条件追加。**不改** `packages/contracts/src/commercial.ts`、**不改** `PURCHASE_STATE_LABEL` 五态词表。

- [ ] **Step 4: 跑测试确认通过**

Run: `pnpm test:web && pnpm typecheck:web && go test ./internal/handler/ ./internal/modules/commercial/service/commercial/ -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/commercial/service/commercial/order.go internal/modules/commercial/service/commercial/purchase.go internal/handler/commercial.go internal/handler/commercial_test.go apps/web/src/commercial/order-state.ts apps/web/src/commercial/order-state.test.ts apps/web/src/commercial/BillingPage.tsx
git commit -m "issue-72(#84): surface payment anomaly attention in billing UI (task5)"
```

---

### Task 6: Lago Payment 恰好一次（权威侧短路验证）

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go`（追加用例；该文件已有 `//go:build lago_integration` tag 与 `LAGO_INTEGRATION_*` env 门控惯例，照 `lago_benefits_integration_test.go:1-46` 形态）
- Test: 同文件（tag 隔离，无栈时 skip 记 blocked-env）

**Interfaces:**
- Consumes: `CommandKindSettlePurchasePayment`/`SettlePurchasePaymentPayload`/`SettlePurchasePaymentCommandKey`（purchase_settlement_command.go:25-80）；lago 适配器 step (i) already-active 短路（lago_settlement.go:68-80）。
- Produces: 集成证据契约——同一订阅经 settle→active 后，第二个**不同** ChannelTransaction 的 settle 命令（多收款第二笔若被误驱动的形态）必须短路返回 receipt 而**零新增 Stripe confirm/零新增 Lago Payment**：断言 `GET /api/v1/payments`（或 GraphQL 等价查询，照该文件既有权威读形态）succeeded 计数不变。

- [ ] **Step 1: 写失败测试（先证短路缺失即失败；若短路已完备则测试首跑即 PASS，保留为回归锁）**

```go
func TestSettleSecondChannelTransactionShortCircuitsWhenActive(t *testing.T) {
    // env 门控：LAGO_INTEGRATION_BASE_URL/API_KEY/STRIPE_* 缺失 → t.Skip("blocked-env")
    // 阶段1：既有链路把租户 A 推到 active（复用本文件既有 settle 成功用例的搭建）
    // 阶段2：before := countLagoSucceededPayments(t, ...)
    //        SubmitCommand(settle, Key=settle:<sub>:txn_OTHER, Payload{ChannelTransaction:"txn_OTHER", ...})
    // 阶段3：after := countLagoSucceededPayments(t, ...)
    //        assert after == before；且命令返回 nil 错误 + receipt（幂等短路非报错）
}
```

- [ ] **Step 2: 跑测试（有真实栈时）确认行为**

Run: `LAGO_INTEGRATION_BASE_URL=http://127.0.0.1:48889 LAGO_INTEGRATION_API_KEY=$LAGO_API_KEY go test -tags lago_integration ./internal/modules/commercial/commercialplatform/ -run TestSettleSecondChannelTransactionShortCircuitsWhenActive -count=1 -v`（凭据经环境变量注入，`source ~/.zcode/issue72-stripe.env`）
Expected: PASS；无栈时 SKIP（blocked-env 记入 Ledger，由 Task 7 真栈轮补权威断言）。

- [ ] **Step 3: 若 FAIL：修复短路**

落点 `lago_settlement.go` step (i)：already-active 分支必须先于任何 Stripe 读/写发生（核对现有实现顺序；若顺序正确但幂等键分叉导致第二键进入扣款路径，则把「already-active 短路」提前到 StripeAPIKey 预检之后、intents 检索之前）。修复后重跑至 PASS。

- [ ] **Step 4: 提交**

```bash
git add internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go internal/modules/commercial/commercialplatform/lago_settlement.go
git commit -m "issue-72(#84): prove settle short-circuit keeps one lago payment (task6)"
```

---

### Task 7: 真实流程验证（真实栈端到端）

**Files:**
- Create: `docs/plans/issue-72-flow-evidence-84/`（脚本 + 证据输出；见下方「真实流程验证方案」）

**Interfaces:**
- Consumes: `docs/plans/issue-72-flow-evidence-83/` 的 `wechat_native_stub.py`（Native v3 协议 stub：可编排交易金额/状态、`/stub/mark`+`/stub/notify`）、`../issue-72-flow-evidence-82/alipay_gateway_stub.py`、`settle-evidence/deliver_stripe_webhook.py`、`seed_83.sh`/`v2_up_backend.sh`/`v2_up_stubs.sh` 起栈形态、`_browser_lib.mjs` 的 `{LOGIN,login,note,runLeg,assertOrderIdentity}`（#82 交付摘要）。
- Produces: `docs/plans/issue-72-flow-evidence-84/{README.md,run_84.sh,four-legs 断言输出,*.png}`——四幕证据 + Ledger 判定输入。

- [ ] **Step 1: 编写证据脚本**（照 #83 v2 形态：起栈脚本 env 化端口 :8096/:5197/:8298/:8299 + lsof 预检 +10 顺延；凭据只从 env 注入；DB 断言用参数绑定只读查询）
- [ ] **Step 2: 执行四幕**（细则见「真实流程验证方案」章节，全部通过判据落 README）
- [ ] **Step 3: 提交**

```bash
git add docs/plans/issue-72-flow-evidence-84/
git commit -m "issue-72(#84): real-stack anomaly evidence (task7)"
```

---

### Task 8: 收尾（全量门禁 + Ledger）

- [ ] **Step 1: 全量回归**

Run: `make test && make lint && make check-backend-architecture && pnpm test:web && pnpm typecheck:web && pnpm test:shared`
Expected: 全部 PASS（`make verify-module-moves` 若触及模块搬迁清单文件则一并跑——本计划不新建模块目录，预期不需要）。

- [ ] **Step 2: 更新 Ledger**（`docs/plans/issue-72-ledger-84.md`：逐 Task 证据、真实栈四幕结果、残留披露）
- [ ] **Step 3: 提交**

```bash
git add docs/plans/issue-72-ledger-84.md
git commit -m "issue-72(#84): ledger closeout"
```

---

## 真实流程验证方案（Task 7 细则）

**目标：** 在真实环境（运行中的 docker compose Lago + 本 worktree 构建的 WeKnora 后端/前端 + 本地协议 stub 模拟渠道收款）端到端验证四条验收链路。本 Issue 无独立管理页面，走「浏览器可见的 Billing/Checkout 页 + 真实 API 链路」混合验证。

### 环境

| 组件 | 来源 | 端口 |
|------|------|------|
| Lago v1.53.0 | 宿主已在跑的 `deploy/lago` compose（`./deploy/lago/lago.sh status` 确认 healthy；未跑则 `init`+`.env` 追加 seed 值+`up`） | 48889/48890（不动） |
| WeKnora 后端 | 本 worktree `cp .env.example .env`（DB_DRIVER=postgres）复用宿主 `WeKnora-postgres-dev:5432/redis:6379/docreader:50051`，`./scripts/dev.sh app` 或 `go run ./cmd/server`，env 覆盖端口 | **8096** |
| WeKnora 前端 | `pnpm --filter @weknora/web dev`（VITE_DEV_PROXY_TARGET=http://localhost:8096） | **5197** |
| 微信 Native stub | 复用 `flow-evidence-83/wechat_native_stub.py` | **8298** |
| 支付宝 stub | 复用 `flow-evidence-82/alipay_gateway_stub.py` | **8299** |
| Stripe TEST 结算腿 | `source ~/.zcode/issue72-stripe.env`（只 env 注入，产出零 `sk_test` 字面量） | — |

- 端口纪律：避开 `:5272/:5273`（他会话占用）与既往口径（:8080/:8091-8095/:5183/:5192-5196/:8291-8297）；起栈前 `lsof -iTCP:<port> -sTCP:LISTEN` 逐口确认，被占整体 +10 顺延并同步脚本 env。
- 种子数据：照 `seed_83.sh` 形态——平台管理员/空间 owner 账号（口令仅 env）、发布 `weknora-pro-v1`（¥99.00）、Lago 侧 org/plan 经 #82 既有 seed（`LAGO_CREATE_ORG` 值已在 `.env`）。后端 `.env` 指向 stub 的商户/网关配置（`WECHAT_*`/`ALIPAY_*` 指到 :8298/:8299，公钥按 stub 生成的临时密钥对，运行时 `${TMPDIR}issue84-keys*`）。

### 第一幕：错金额/部分付款不激活 + 事实落库 + 幂等终态应答（AC1 前半 + 事实保留）

1. Playwright（MCP `browser_navigate` 或 `@playwright/test` 脚本，登录态经 seed 账号）：`http://localhost:5197/platform/billing` → 选 `weknora-pro-v1` → 提交购买（渠道选微信）→ 断言「待付款（权益未开通）」+ 订单号；`docs/plans/issue-72-flow-evidence-84/leg1-01-awaiting.png`。
2. 从网络层取 `POST /api/v1/commercial/purchases` 响应的 order id；DB 只读参数绑定查 `commercial_payment_attempts` 得 `merchant_order_id`。
3. `curl -X POST http://127.0.0.1:8298/stub/mark -d '{"out_trade_no":"<mo>","amount_total":5000,"trade_state":"SUCCESS","transaction_id":"txn84-leg1-a"}'`（编排实收 50.00 元 = 部分付款）→ `curl -X POST http://127.0.0.1:8096/api/v1/commercial/callbacks/wechat -H 'Wechatpay-*: …' --data @notify.json`（stub `/stub/notify` 产出已验签报文）。
4. **断言**：回调 HTTP 200 + `{"code":"SUCCESS"}`；重复投递同通知再发一次 → 仍 200；`commercial_payment_anomalies` 恰 1 行（kind=`partial_payment`，expected 9900/actual 5000，state=awaiting_disposition）；订单仍 `pending`、无 fulfill outbox 事件；`GET /api/v1/commercial/orders/<id>` → `fulfillment:"attention"`、`payment:"pending"`。
5. **权威侧**：Lago `GET /api/v1/subscriptions?external_id=<ext-purchase-id>` 仍 incomplete/无激活；`GET /api/v1/payments?external_customer_id=<ext-cust>` 本幕零新增 Payment（curl 经 `LAGO_API_KEY` env）。
6. UI 终态：浏览器轮询 Checkout 页 → 断言文案「付款异常（资金事实已记录，待处理）」→ `leg1-02-anomaly.png`；`http://localhost:5197/platform/billing` 购买行含「付款异常」后缀 → `leg1-03-billing-anomaly.png`。
7. 错币种变体：微信 stub 编排 `amount.currency:"USD"`（stub 支持 amount 字段透传；若 stub 需小改，改副本入 evidence 目录，不改 83 资产本体）→ 断言 kind=`currency_mismatch` 同套落库/不激活/200。

### 第二幕：正常付款 + 重复通知幂等 + Lago Payment 恰一条（AC3）

1. 新购买单（支付宝渠道）→ stub 编排正确金额成功 → 回调 200。
2. `deliver_stripe_webhook.py` 投递（或等待 Lago worker 消化 Stripe 结算腿）→ 轮询 ≤120s：`GET /api/v1/commercial/purchase` → `state:"active"`；订单 `fulfilled`。
3. **重复投递同一成功回调 3 次** → 每次均 200（Alipay `success` 纯文本）；DB：fulfill 事件仍 1、fulfillment_records purchase_activation 仍 1；**Lago payments succeeded 计数==1**（权威侧 curl 计数断言，`leg2-payments-count.txt` 留档）。
4. UI：Billing/Checkout 文案走「已付款，权益处理中」→「权益已生效」两帧截图 `leg2-01-processing.png`/`leg2-02-active.png`（轮询间隔内捕获，错过则以 API 状态序列留档并披露）。

### 第三幕：多成功只履约一次 + 多收款进入处置（AC2）

1. 承第二幕已 active 的租户：对**另一新购买单**先正常付款激活（或对已 paid 未 fulfilled 窗口单操作——取其一，脚本固定为「新单正常链」以保证确定性），随后以第二渠道（微信）对**同一订单的第二个 attempt** 编排成功（stub 不同 transaction_id）→ 回调 200。
2. **断言**：anomaly 表新增 kind=`over_payment` 行（drain ≤60s 内消费，脚本轮询 outbox sent + anomaly 行）；权益不扩大——Lago wallets 的 purchase 批次仍恰 1、`fulfillment_records` applied 仍 1；**Lago payments succeeded 仍==1**（settle 短路，Task 6 单测的真栈对照）；`leg3-overpay-anomaly.txt`（DB 只读查询输出）。
3. 处置面：平台守卫凭据 `GET /api/v1/commercial/admin/payment-anomalies` → 列表含第一/三幕行；`POST .../resolve`（expected_version 正确）→ 200 resolved；随后 `GET /orders/<leg1单>` → `fulfillment` 回落非 attention → `leg3-resolved.txt`。

### 第四幕：恢复路径实收额比对（G2 真实链）

1. 新购买单（微信）→ 不投递回调（模拟漏通知）→ 浏览器/`curl GET /api/v1/commercial/orders/<id>`（触发 RecoverOrderStatus）之前，stub 先编排 `amount_total` 为错额 + `trade_state:SUCCESS`。
2. **断言**：GET 返回 `fulfillment:"attention"`；订单仍 pending；anomaly 表 kind=`amount_mismatch`（expected=9900, actual=篡改值）；Lago 侧无激活、无 Payment 新增；`leg4-recovery-mismatch.txt`。

### 通过判据（全部满足才记 PASS）

- 四幕全部断言命中；截图/文本证据齐备于 `docs/plans/issue-72-flow-evidence-84/`。
- 全程 `grep -rn "sk_test" docs/plans/issue-72-flow-evidence-84/` 零命中（凭据纪律）。
- 权威侧计数断言（payments==1、subscription active 恰一次、wallets 批次不增）全部来自真实 Lago API/DB 只读查询，非 mock。
- 披露边界：微信/支付宝为本地协议 stub（已裁决的披露边界，非沙箱证据）；Alipay 通知契约无币种字段——错币种仅微信腿可造，Alipay 错币种以文档披露。

---

## 验收标准 → Task → 测试 追踪矩阵

| Issue #84 验收标准 | 覆盖 Task | 测试（命令级锚点） |
|---|---|---|
| AC1a 错金额不能激活 | T1/T2/T4 | `TestConfirmPaymentMismatchRetainsExternalFactWithoutFulfillment`（repo）；`TestWechatCallbackAmountMismatchRecordsAnomalyAndAcksIdempotent`（handler）；`TestRecoverOrderStatusAmountMismatchRecordsAnomalyWithoutConfirm`（service）；真栈第一幕 |
| AC1b 错币种不能激活 | T1/T2 | `TestClassifyPaymentAnomalyClosedKinds`（currency_mismatch）+ 第一幕 7 变体 |
| AC1c 部分付款不能激活 | T1/T2 | `ClassifyPaymentAnomaly(9900,5000)→partial_payment` + 第一幕步骤 3-6 |
| AC2 多个成功只履约一次，其余进入多收款处置 | T3 | `TestOverPaymentDrainConsumesEventIntoAwaitingDisposal`、`TestOverPaymentDrainReplayYieldsSingleAnomaly`、`TestRecoverOrderStatusLateSuccessAfterFulfilledIsIdempotentOverPayment`；真栈第三幕 |
| AC3 重复通知幂等应答且不重复写 Lago Payment | T2/T6 | `TestWechatCallbackDuplicateDeliveryIdempotent`（既有回归）+ `TestSettleSecondChannelTransactionShortCircuitsWhenActive`（lago_integration）；真栈第二幕（payments==1） |
| AC4 Billing UI 区分付款异常/履约处理中/已生效 | T5 | `TestOrderWireProjectsAttentionWhileUnresolved`、`TestGetOrderSurfacesUnresolvedAnomaly`、order-state.test.ts 三态断言；真栈第一/二幕截图 |
| 事实保留（spec L127，AC 的前提） | T1 | `TestRecordPaymentAnomalyIdempotentOnUniqueKey`、`TestConfirmPaymentMismatchAnomalyInsertFailurePropagates` |
| 运营处置可见 | T3 | `TestAdminPaymentAnomaliesListAndResolve`、`TestAdminPaymentAnomaliesRequiresPlatformReviewer`；真栈第三幕步骤 3 |

## 文档与提交步骤

1. 每个 Task 独立提交（信息见各 Task Step 5）；全部完成后 `make test && make lint && pnpm test:web` 全绿。
2. Ledger：`docs/plans/issue-72-ledger-84.md` 持续记录（Task 8 收尾节定稿）。
3. 证据目录不写凭据；Stripe key 仅 env；截图仅含 UI 闭合文案。
4. 本计划文件本身的修订（若执行中发现接口偏差）直接改本文件并在 Ledger 记偏差行，不另开文档。

## 自检结论（计划员落盘前）

- 占位符/TBD：无（所有文件路径、签名、测试名、命令均为实读核实或显式新建）。
- 接口一致性：`PaymentAnomalyRow`/`RecordPaymentAnomaly`/`HasUnresolvedPaymentAnomaly` 在 T1 定义、T2-T5 消费，签名一致；`AttemptResult.AmountFen` T4 定义自消费；orderWire `attention` 与契约 token `packages/contracts/src/commercial.ts:3` 既有闭集一致，契约零改动。
- 追踪矩阵：8 行覆盖 4 条 AC + spec L127 前提 + 处置面，每行有测试命令与真栈幕次双锚点。
- 依赖披露：#85 并行未合入（S1 边）——Global Constraints 第 11 条定义了开工前合并纪律；若 #85 最终未合入即开工，`service/commercial/order.go` 的合并冲突由集成分支主会话仲裁。
