# Issue #82 复验第 1 轮（reverify-1）— 修复后全流程重验

- 日期：2026-09-25（修复提交 `7324eb054` + ledger `59465f8fe` 之后）
- 分支/代码：`codex/issue-72-lago` HEAD=`59465f8fe`（集成分支最新；复验前旧二进制已按 PID 终止，后端 :8092 以新代码重编译启动——19s 就绪，日志 /tmp/issue82-backend-rv1.log）
- 复验者：流程验证员-82（dynamic workflow），独立重跑，不复用修复员自验结论
- 环境：与第 1 轮同一套真实栈——Lago `weknora-lago-82flow`（:48889，v1.53.0 全 healthy，weknora-stripe provider + 真 Stripe TEST key）、前端 vite :5192、alipay stub :8292 / wechat stub :8291、sqlite `data/issue82-flow.db`（复用第 1 轮库，新增租户 C(3)/D(4)）

## 修复复验结果（对应第 1 轮三缺陷 + 冻结面）

| 第 1 轮缺陷 | 复验步骤 | 结果 | 证据 |
|---|---|---|---|
| A：新租户首购 503 invalid_response（PUT /customers/:id 在 v1.53 无 update 路由） | 租户 C/D **均不删 Lago customer**（onboarding 未绑定前置原样保留——C 的 customer 名 `fix82-1790308166's Workspace` 为修复员复验遗留，恰是「已存在未绑定」的真实形态）直接下单 | ✅ C 浏览器腿 201（ord_b4d7323c9410f0a3）、D 支付宝腿 201（ord_bbf32eee096f1abb）——上轮同前置 503，本轮全通 | `rv1-01-checkout-awaiting-payment.png`、`rv1-api-02-purchase-alipay.json` |
| A 附带：显示名保留（R1-V05 意图） | 购买后读 Lago customer | ✅ name 仍 `fix82-1790308166's Workspace`（upsert 未带 name 键不覆写）+ binding 落库 `stripe/weknora-stripe/cus_VJo6y3G3zUa1oE` | `rv1-lago-tenant3-upsert-keeps-name.txt` |
| B：匿名渠道回调 401（auth 白名单缺失） | **无 Authorization 头** POST 签名 notify（真实渠道形态） | ✅ HTTP 200 + 响应体恰 `success` | `rv1-callback-01-anon-success.txt` |
| C：attempt merchant 错配（'alipay' vs SellerID → 404 failure） | notify 入账后直查 attempt 行 | ✅ `merchant=2088000000000000`（SellerID）+ state=succeeded；订单 `payment=paid, state=paid`；outbox 恰 1 | `rv1-callback-01-anon-success.txt` |
| AC3 渠道幂等（重放不重复入账） | 同一 notify 重放 | ✅ 仍 `success` 200，outbox/fulfillment 计数前后不变（1/0） | `rv1-callback-02-replay-and-negative.txt` |
| 匿名面安全边界（负控） | 伪造签名（篡改 total_amount + 假 sign）notify | ✅ `failure` 401（验签 fail-closed，未入账） | 同上 |
| 同步面/付款后呈现 | 浏览器（真实渲染）：D 回访 checkout?order= 与 Billing | ✅ checkout「已付款，权益处理中」（order.payment=paid 呈现）；全页无「已生效」虚假声明；Billing 套餐行不虚报（激活链冻结下诚实呈现） | `rv1-03-checkout-paid-awaiting-activation.png`、`rv1-04-billing-after-paid.png`（browser_paid_face_82.mjs 4/4 PASS） |
| Lago 侧四对象（AC4/GC-3） | 订阅/钱包/invoice/payment | ❌ 维持冻结边界：tenant-4 订阅仍 `incomplete`、wallets=0、payments 全栈 failed=5/requires_action=5 无 succeeded——**d2-falsified-t10-p2-fail**（Task 5-8 未实施，激活链缺席，待 T02 §5 重议修订版计划承接） | `rv1-lago-four-objects-after-paid.txt` |
| 权益闭合 | tenant-4 entitlements raw probe | ✅ 404（权益未开放——激活前正确闭合） | 同上 |
| 回归 | `go test ./internal/modules/commercial/... ./internal/handler/ ./internal/router/ ./internal/middleware/ -count=1` | ✅（见 rv1-gotest.txt 摘录；含修复新增回调链回归 payment_callbacks_test.go 202 行与 router 匿名可达测试） | `rv1-gotest.txt` |
| 红线 | 凭据 grep / 本地强制 active grep / architectureguard / modulemove | ✅（同第 1 轮四项，见 rv1-redline.txt） | `rv1-redline.txt` |

## AC4 披露（不变）

`ac4-sandbox-credentials-unavailable`：无支付宝沙箱凭据。替身仍为本地 RSA 密钥对 + 回环网关 stub + 同源签名 notify——本轮它验证的是**修复后的完整渠道入账链**（匿名可达→验签→attempt 解析→ConfirmPayment 幂等→outbox 恰一），不构成真实沙箱钱包付款证据。

## 环境共享痕迹披露

修复员复验曾以临时实例（:8093/flowfix.db）对同一 Lago 栈写入：customer 名 `fix82-1790308166's Workspace`（tenant-3）、wallets `weknora-tenant-6-2026-09`/`weknora-tenant-3-2026-09`（其租户 6/3 的 Base 月度 grant）。本轮租户 C 撞 external_id `weknora-tenant-3`（C 的 gated 订阅 created 04:03:04Z 属本轮；tenant-3 wallet 非本轮产物）。核心断言全部落在无污染的 tenant-4（D 支付宝全链）与租户 C 的本地实例数据面上，不受影响。

## 结论

第 1 轮三产品缺陷（回调 401 白名单 / merchant 错配 / PUT customer 404）**全部修复并独立复验通过**；「可信异步回调入账→订单 paid→重复回调只入账一次」链路在真实栈端到端可用。激活后半链（订单/订阅变已生效、Credits/Entitlement 到账、Lago Payment/active Subscription 关联）仍受 `d2-falsified-t10-p2-fail` 冻结约束不可达（Task 5-8 未实施、`paid_awaiting_activation` 仍不存在——付款后呈现为「已付款，权益处理中」，诚实不虚报）。Issue #82 整体达成的判定仍归修订版计划。
