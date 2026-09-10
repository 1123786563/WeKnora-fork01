# SaaS 02 支付与履约 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成订单付款、权益到账和人工退款的可恢复链路。

**Architecture:** 订单和 Outbox 原子落地；微信与支付宝独立验签；官方商业适配在回调外恢复，退款与任务占用共用同一额度保护。

**Tech Stack:** 当前仓库 Go 1.26、Gin、GORM、PostgreSQL／SQLite 版本化迁移；React 19、TypeScript 6、pnpm 10.28.2；官方 OpenMeter 固定版本；优先复用现有依赖。

**Spec:** [完整规格](../specs/2026-09-10-saas-billing-connectors-design.md)；[接口验证清单](../specs/2026-09-10-saas-billing-connectors-interface-verification.md)。执行者必须同时阅读本计划引用的规格章节与上下游契约。

## Global Constraints

- 每个空间始终独立购买、付款并承担费用；共享组织不改变归属，不共享余额或套餐。
- 费用预算批准和外部写入批准是独立条件，两者都满足才能执行。
- 首期不包含：跨空间合并付款或余额池、自动扣款、独立席位与存储加购包、嵌入式第三方 Web 应用、公开开发者上架与交易分成。
- 固定候选基线：`v1.0.0-beta.232`，提交 `887e0cac903ccd06e74d61ed23c651651d10c7a9`。
- 未在验证清单取得真实通过证据的外部能力，不得标记为生产可用；不使用旧本地 OpenMeter fork 接口。
- 保留已有 React、Craft、移动端和其他任务的工作；仅提交本任务明确拥有的文件，不使用 `git add .`。
- 下列内部接口是计划新增契约，不声称当前已存在；外部请求字段以 P00 的已验证契约为准。
- 时区、精度与分段报价采用完整规格的推荐默认值；外部模型无法无损表达时在 P00 停止依赖任务并修正规格，不能自行换商业语义。

---

## 依赖与执行边界

F01～F05 与 V03；C05 的可退锁定还依赖 U02/U03（预算计划中的任务编号）。真实渠道测试在对应商户与外部付款授权具备后进行。

本文件仅编写计划，不表示已实现或已测试。执行时先使用 using-git-worktrees 建立隔离工作区；所有命令从仓库根执行。每一步只做所列一个动作；较长代码修改按列出的文件／函数逐项执行，完成该任务的 RED→GREEN 与独立 review 后再进入下一任务。

## 文件结构与职责

payment：渠道协议；repository/commercial：交易与 Outbox；service/commercial：履约与退款；infrastructure/openmeter：唯一官方商业模型适配。

### Task C01: 订单、支付尝试与持久化 Outbox

**Files:**
- Create: `internal/commercial/order.go`、`internal/commercial/order_test.go`
- Create: `internal/application/repository/commercial/order.go`、`internal/application/repository/commercial/outbox.go`
- Create: `migrations/versioned/000113_commercial_orders.up.sql`、`migrations/versioned/000113_commercial_orders.down.sql`
- Create: `migrations/sqlite/000033_commercial_orders.up.sql`、`migrations/sqlite/000033_commercial_orders.down.sql`

**Interfaces:**
- Consumes: F02 AccountStore；F03 Quote/CNYFen；F05 商业路由。
- Produces: `PaymentFact{TenantID uint64; OrderID,AttemptID,Provider,Merchant,Transaction string; Amount CNYFen; Currency,State string}`；`Order{ID string; TenantID uint64; Amount CNYFen; Currency,State string; Version int64}`；`ValidatePayment(Order,PaymentFact) error`；repository `ConfirmPayment(context.Context,PaymentFact) error`。

**行为与边界：** 增加真实 DB 事务测试：付款确认与 Outbox 写入之间故障必须整体回滚；响应丢失重放不双事件；并发两渠道成功只一权益。

**验收映射：** B15；COM-03/04；AC-03/04/05

- [ ] **Step 1: 在 `internal/commercial/order_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestPaymentCannotChangeOrderAmount(t *testing.T) {
    order:=Order{ID:"o1",TenantID:7,Amount:100,Currency:"CNY"}
    fact:=PaymentFact{OrderID:"o1",TenantID:7,Amount:99,Currency:"CNY",State:"succeeded"}
    if ValidatePayment(order,fact)==nil { t.Fatal("amount mismatch accepted") }
    fact.Amount=100; fact.TenantID=8
    if ValidatePayment(order,fact)==nil { t.Fatal("space mismatch accepted") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/application/repository/commercial -run "Test(Payment|Order|Outbox)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func ValidatePayment(o Order,f PaymentFact) error {
    if o.ID!=f.OrderID || o.TenantID!=f.TenantID || o.Amount!=f.Amount ||
       o.Currency!=f.Currency || f.State!="succeeded" {
        return errors.New("payment_mismatch")
    }
    return nil
}
```

订单 SQL 字段 ID、tenant、quote_id UNIQUE、amount_fen、currency、state、version；attempt 表包含 provider、merchant、merchant_order_id UNIQUE、provider_transaction_id，后两者按 provider/merchant 组合唯一。Outbox 保存 event_key UNIQUE、tenant、kind、payload_json、state、lease_token、lease_until、attempt_count。

ConfirmPayment 在同一事务验证已注册的商户与 attempt、查订单版本、保存付款事实、将订单标 paid 并 INSERT 唯一 fulfill 事件。重复同渠道交易返回原结果；不同 attempt 都成功只新增多收款审计事件，不重复 fulfill。报价只能被一个订单消费。GET 已付订单应区分 paid 与 fulfilled。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/application/repository/commercial -run "Test(Payment|Order|Outbox)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/order.go internal/commercial/order_test.go internal/application/repository/commercial/order.go internal/application/repository/commercial/outbox.go migrations/versioned/000113_commercial_orders.up.sql migrations/versioned/000113_commercial_orders.down.sql migrations/sqlite/000033_commercial_orders.up.sql migrations/sqlite/000033_commercial_orders.down.sql docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: persist payment facts with fulfillment outbox"
```

### Task C02: 支付适配接口与微信验签链路

**Files:**
- Create: `internal/payment/provider.go`、`internal/payment/wechat.go`、`internal/payment/wechat_test.go`
- Create: `internal/handler/payment_callbacks.go`
- Modify: `internal/router/routes_commercial.go`

**Interfaces:**
- Consumes: C01 PaymentFact/ConfirmPayment；清单 WX-01 至 WX-05；现有安全配置加载。
- Produces: payment `OrderRequest{OrderID,MerchantOrderID string; AmountFen int64; Currency string}`；`AttemptResult{State,ProviderID,CheckoutURL string}`；`RefundRequest{RefundID,ProviderID string; AmountFen int64}`；`RefundResult{State,ProviderID string}`；`Provider` 的 Create/Query/Close/Verify/Refund/QueryRefund 接口（下列完整签名）。

**行为与边界：** 分别完成 WX-01～05；单测覆盖原始 body 篡改、错 key/Serial、错商户/金额、重复通知。真实付款只在用户已授权的测试商户与金额内执行；无条件则记录 blocked-env，不能用生成 RSA 签名代替渠道验收。

**验收映射：** WX-01 至 WX-05；AC-03

- [ ] **Step 1: 在 `internal/payment/wechat_test.go` 写入以下失败断言。**

```go
package payment
import("testing";"crypto/rand";"crypto/rsa";"crypto/sha256";"crypto";"encoding/base64")
func TestWechatRejectsChangedCallbackBody(t *testing.T) {
    key,err:=rsa.GenerateKey(rand.Reader,2048); if err!=nil { t.Fatal(err) }
    message:=[]byte("1\nn\n{}\n"); digest:=sha256.Sum256(message)
    sig,err:=rsa.SignPKCS1v15(rand.Reader,key,crypto.SHA256,digest[:]); if err!=nil { t.Fatal(err) }
    if VerifyWechatSignature(&key.PublicKey,"1","n",[]byte("{bad}"),base64.StdEncoding.EncodeToString(sig))==nil { t.Fatal("tampered body accepted") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/payment -run TestWechat -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

接口代码放 provider.go；import context/net/http 和 internal/commercial：

```go
type Provider interface {
 Create(context.Context,OrderRequest)(AttemptResult,error)
 Query(context.Context,string)(AttemptResult,error)
 Close(context.Context,string) error
 Verify(context.Context,http.Header,[]byte)(commercial.PaymentFact,error)
 Refund(context.Context,RefundRequest)(RefundResult,error)
 QueryRefund(context.Context,string)(RefundResult,error)
}
func VerifyWechatSignature(key *rsa.PublicKey,ts,nonce string,body []byte,signature string) error {
 raw,err:=base64.StdEncoding.DecodeString(signature); if err!=nil { return err }
 digest:=sha256.Sum256([]byte(ts+"\n"+nonce+"\n"+string(body)+"\n"))
 return rsa.VerifyPKCS1v15(key,crypto.SHA256,digest[:],raw)
}
```

签名函数依赖由 wechat.go imports 声明。实际商户证书／公钥按 Serial 选择，时间窗口／防重放、AES-GCM 解密和商户应用匹配按 WX-02 官方版本核验，不能用客户端传入公钥。配置只保存密钥引用。callback 读取一次原始 body、Verify 成功后按 merchant_order_id 查本地订单重建可信 tenant，调用 ConfirmPayment 持久化后才应答。创建／查单／退款请求字段由 WX-01 的实际已开通产品固定；请求超时返回 unknown 并查原标识，不换键。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/payment -run TestWechat -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/payment/provider.go internal/payment/wechat.go internal/payment/wechat_test.go internal/handler/payment_callbacks.go internal/router/routes_commercial.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: verify WeChat payments before fulfillment"
```

### Task C03: 支付宝独立支付与退款适配

**Files:**
- Create: `internal/payment/alipay.go`、`internal/payment/alipay_test.go`
- Modify: `internal/handler/payment_callbacks.go`

**Interfaces:**
- Consumes: C02 Provider 及请求／结果类型；C01 ConfirmPayment；ALI-01 至 ALI-05。
- Produces: `AlipayProvider` 实现同一 Provider；`ParseCNYAmount(string) (int64,error)` 精确把渠道十进制元转换为整数分。

**行为与边界：** 金额格式、签名篡改、错 app/卖家、不同状态、重复通知、漏通知查单分别测；完成 ALI-01～05 并记录具体方法/版本；缺官方可读正文或商户环境时保留门槛，不能猜字段。

**验收映射：** ALI-01 至 ALI-05；AC-04

- [ ] **Step 1: 在 `internal/payment/alipay_test.go` 写入以下失败断言。**

```go
package payment
import "testing"
func TestAlipayAmountIsExactFen(t *testing.T) {
    got,err:=ParseCNYAmount("10.01"); if err!=nil || got!=1001 { t.Fatalf("%d %v",got,err) }
    if _,err:=ParseCNYAmount("10.001"); err==nil { t.Fatal("fractional fen accepted") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/payment -run TestAlipay -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func ParseCNYAmount(s string)(int64,error) {
 r,ok:=new(big.Rat).SetString(s)
 if !ok || r.Sign()<0 { return 0,errors.New("invalid_amount") }
 r.Mul(r,big.NewRat(100,1))
 if !r.IsInt() || !r.Num().IsInt64() { return 0,errors.New("invalid_amount_precision") }
 return r.Num().Int64(),nil
}
```

alipay.go import math/big/errors，生产入参先限制为 `^[0-9]+(\.[0-9]{1,2})?$`，拒绝分数表达与指数。API 产品、签名规范、应用和卖家校验按 ALI-01/02 的实际官方契约固定；异步通知原始表单值不被二次 decode 后错误重签。只有可信成功状态生成 PaymentFact；同步返回不确认付款。退款稳定标识与查询映射单独验证，不复用微信字段。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/payment -run TestAlipay -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/payment/alipay.go internal/payment/alipay_test.go internal/handler/payment_callbacks.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: add independent Alipay payment verification"
```

### Task C04: 付款到权益的幂等履约工作进程

**Files:**
- Create: `internal/commercial/fulfillment.go`、`internal/commercial/fulfillment_test.go`
- Create: `internal/infrastructure/openmeter/commercial.go`、`internal/infrastructure/openmeter/commercial_test.go`
- Create: `internal/application/service/commercial/fulfillment.go`
- Modify: `internal/container/container.go`

**Interfaces:**
- Consumes: C01 Outbox/Order；F03 Quote；F04 月度 key；V03 selected-model.json。
- Produces: `FulfillmentKey(orderID,lineID string) string`；`BenefitRequest{Key string; TenantID uint64; CustomerID,Kind,PlanRef string; Credits Credits; EffectiveAt,ExpiresAt time.Time}`；`BenefitReceipt{ExternalID string; EffectiveAt time.Time}`；`CommercialGateway.ApplyBenefit(context.Context,BenefitRequest)(BenefitReceipt,error)`、`FindBenefit(context.Context,string)(BenefitReceipt,error)`。

**行为与边界：** 用接口 stub 证明状态转换，用 V03真实模型重跑 OM-04/09证明幂等；两种证据分开。进程重启、多个 worker、下单者离开空间仍能完成原付款履约。

**验收映射：** COM-04；OM-04/09；AC-05

- [ ] **Step 1: 在 `internal/commercial/fulfillment_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestFulfillmentKeySeparatesOrderLines(t *testing.T) {
    if FulfillmentKey("o1","l1")==FulfillmentKey("o1","l2") { t.Fatal("line collision") }
    if FulfillmentKey("o1","l1")!=FulfillmentKey("o1","l1") { t.Fatal("unstable") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/infrastructure/openmeter ./internal/application/service/commercial -run "Test(Fulfillment|ApplyBenefit)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func FulfillmentKey(orderID,lineID string) string {
 sum:=sha256.Sum256([]byte(orderID+"\x00"+lineID))
 return "fulfill:"+hex.EncodeToString(sum[:])
}
```

gateway 只实现 V03 选定的一套模型；真实字段映射与 contract-cases 使用同一已验证 schema。工作进程持久化 claim token，先 FindBenefit 后按唯一键 ApplyBenefit；超时保持 attention/unknown，不将业务错误当作成功。所有订单行返回明确 external_id 才变 fulfilled。paid 和履约状态分离；充值 effective_at 第一次成功固定，后续重试复用；订阅与充值分别选择已验证结算路径，不重复 Invoice/settlement。

用 httptest.Server 模拟“远端保存后断开响应”，再真实 DB 重启 worker；断言外部仅一份 benefit、本地 fulfilled、Outbox 无重复。注册后台恢复任务而非阻塞支付回调。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/infrastructure/openmeter ./internal/application/service/commercial -run "Test(Fulfillment|ApplyBenefit)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/fulfillment.go internal/commercial/fulfillment_test.go internal/infrastructure/openmeter/commercial.go internal/infrastructure/openmeter/commercial_test.go internal/application/service/commercial/fulfillment.go internal/container/container.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: recover paid orders without duplicate benefits"
```

### Task C05: 退款锁定、渠道出款与精确撤权

**Files:**
- Create: `internal/commercial/refund.go`、`internal/commercial/refund_test.go`
- Create: `internal/application/repository/commercial/refund.go`、`internal/application/service/commercial/refund.go`
- Create: `migrations/versioned/000114_commercial_refunds.up.sql`、`migrations/versioned/000114_commercial_refunds.down.sql`
- Create: `migrations/sqlite/000034_commercial_refunds.up.sql`、`migrations/sqlite/000034_commercial_refunds.down.sql`
- Modify: `internal/handler/commercial.go`、`internal/router/routes_commercial.go`

**Interfaces:**
- Consumes: C02 Provider.Refund/QueryRefund；C04 CommercialGateway；P03 的占用可退核对，未完成前退款只能保留 requested。
- Produces: `RefundRequestState{ID,OrderID,State string; TenantID uint64; Amount CNYFen; CreditAmount Credits}`；`CanUnlockRefund(channelState string) bool`；`CommercialGateway.RevokeBenefit(ctx context.Context,key string,credits Credits) error`（在 C04 接口新增）；`RefundService.Approve(ctx context.Context,id,reviewer string) error`。

**行为与边界：** 部分退款与用量竞争必须有真实 DB 并发测试；退款成功撤权失败的恢复不得再次出款；记录未开始周期与已生效周期人工审批依据，未配置政策不自动同意。

**验收映射：** B18/B19；COM-05/06；AC-10

- [ ] **Step 1: 在 `internal/commercial/refund_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestRefundUnknownKeepsLock(t *testing.T) {
    for _,s:=range []string{"pending","unknown","succeeded","revocation_pending"} {
        if CanUnlockRefund(s) { t.Fatalf("unlocked %s",s) }
    }
    if !CanUnlockRefund("failed_confirmed") { t.Fatal("confirmed failure not released") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/application/service/commercial -run "Test(Refund|RevokeBenefit)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func CanUnlockRefund(channelState string) bool {
 return channelState=="failed_confirmed" || channelState=="not_created_confirmed"
}
```

refund 表保存原订单行、金额、额度、审批人、provider_refund_id UNIQUE、state、version；refund_allocation 保存 lot ID 与 locked_micro，多个退款通过 P03同一准入事务协调器核算，不能单独读余额后退款。批准事务检查已用、在途、旧退款与资格，再写锁和 Outbox；未知使用先停 requested/reviewing，不强退。

渠道成功转 revocation_pending；只在 RevokeBenefit 的精确额度撤回确认后 completed。失败确认才释放锁，unknown 重查原退款键。人工可申请重试但不能换键重复出款；整批 void 不满足部分退款时依赖 V03已验证补偿。POST `/refunds`、POST `/admin/refunds/:id/review` 分离空间申请与平台权限。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/application/service/commercial -run "Test(Refund|RevokeBenefit)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/refund.go internal/commercial/refund_test.go internal/application/repository/commercial/refund.go internal/application/service/commercial/refund.go migrations/versioned/000114_commercial_refunds.up.sql migrations/versioned/000114_commercial_refunds.down.sql migrations/sqlite/000034_commercial_refunds.up.sql migrations/sqlite/000034_commercial_refunds.down.sql internal/handler/commercial.go internal/router/routes_commercial.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: coordinate refunds with locked commercial benefits"
```

