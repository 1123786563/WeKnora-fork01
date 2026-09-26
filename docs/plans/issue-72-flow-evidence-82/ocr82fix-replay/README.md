# Issue #82 OCR 修复批次（ocr-82-1）活栈重放证据

- 验证者：修复员（dynamic workflow，本批次）
- 分支：`codex/issue-72-lago`（worktree `.worktrees-issue72/lago-int`，含 A-01~A-34 修复）
- 触发理由：本批次 Go 生产代码修复（settle/purchase/fulfillment/order/config）改变了已验证用户流程的行为，按批次要求重放真实流程验证脚本。

## 环境拓扑（全部真实运行）

| 组件 | 地址 | 说明 |
|---|---|---|
| Lago v1.53.0 | 127.0.0.1:48889 | 复用 `weknora-lago-82flow` 栈（healthy）；本轮重放前经 Lago API 软删除旧 `weknora-pro-v1`（A-09 严格递增断言的基线前提），本轮 publish 重建 |
| WeKnora 后端 | 127.0.0.1:8093 | 修复后代码 `go build ./cmd/server`；`DB_DRIVER=sqlite DB_PATH=data/issue82-ocr82fix.db`（独立库）；`WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true` + loopback 权威（A-27 守卫的 lab 姿态——启动成功即守卫不误伤的活栈验证） |
| Stripe 真实 API | api.stripe.com | test key（source-only 注入）；gating `pm_card_threeDSecure2Required` / settle `pm_card_visa` |
| 支付宝网关 stub | 127.0.0.1:8294 | `../alipay_gateway_stub.py`；本地 RSA 密钥对（`/tmp/flow82-keys-r5`，仅本机，不入库） |

测试数据：占位租户 1-14（吸收 82flow 栈既有 weknora-tenant-1..14 身份，`FLOW82_PAD_COUNT=14`——A-10 修复后脚本默认正确但本轮显式导出），主角 `ocr82fix-r5-a@verify.local`（租户 15）/ `ocr82fix-r5-b@verify.local`（租户 16，发布者）。

## 重放步骤与结果

1. **seed.sh（r4-flow3 修复版）**：全链 PASS——注册断言（A-13：16×HTTP 201 asserted）、UUID/TOKEN 白名单（A-08）、draft 断言+版本驱动 publish（A-14：v=1→receipt `publish_plan_version:pro:1` 匹配）、Lago 落库严格递增（A-09：baseline 0 → 1，`api-03-lago-plans-baseline.json`/`api-03-lago-plans.json`，本轮副本在本目录）。
2. **quote → purchase（API 面）**：`POST /commercial/quotes {plan_key:pro}` → `qt_2556ff92d9242d4e`；`POST /commercial/purchases {provider:alipay}` → **201** `ord_c1a2c0e2afa2bb18` + `checkout_url` 落盘（stub PRECREATE `mo_f4f862134b9e6155`）。
3. **签名 notify → ConfirmPayment**：`alipay_sandbox_notify.py`（本地 RSA）→ `POST /commercial/callbacks/alipay` → HTTP 200 `success`；`PurchaseStatus` 合成 **`paid_awaiting_activation`**（A-28/A-29 投影路径正常——paid 订单不被遮蔽、计划归属证明成立）。
4. **settle（fulfiller 30s drain）**：修复后 settle 驱动真实 Stripe——gating intent `pi_3UK3eK2X9knJyx9h1Np69q9r`（**amount=1650，本月 proration**）经 attach/update/confirm（A-18 派生幂等键 `:attach`/`:default`/`:update`/`:confirm`）→ **succeeded**；残留兄弟 intent（`requires_payment_method`，同 invoice）未被驱动（F-1 invoice 幂等窗）。
5. **webhook（D8 替身腿）**：Stripe API 读回真实 PI 对象 + provider 真实 secret（`PaymentProviders::StripeProvider` model 层读取）HMAC 签名 → `POST /webhooks/stripe/{org}?code=weknora-stripe` → **HTTP 200**；Lago 内建链 finalize：订阅 **active**、invoice **finalized + payment_status=succeeded**。
6. **收口**：下一轮 drain `observeActivation` → active → grant → **`PurchaseStatus = {"state":"active","order":"fulfilled"}`**；fulfillment 记录 **attention → applied**（终态 APPLIED 覆盖早前瞬态 attention，自愈语义；A-33 后 attention 日志仅首次落地一次）。
7. **AC3 幂等重放**：同 event id 重投 webhook 两次均 200 no-op，状态机不变。

## 重放过程中发现并修正的回归（A-16 首版）

A-16 首版按 fixHint 实现为强等校验（`intent.Amount == payload.AmountFen`）。活栈重放抓到回归：真实 82flow 栈的 gating PaymentIntent 金额是 **proration 后的 1650**（9900 冻结价的本月按比例），强等校验把 settle 打死（InvalidResponse → attention，`pi` 状态不推进）。已改为 **proration-aware 上界守卫**（`1 ≤ intent.Amount ≤ payload.AmountFen`，对齐 D6' 的"小于面额、绝不大于、绝不归零"纪律），并补单测 `TestLagoSettleProratedIntentStillSettles`（1650-of-9900 放行并驱动 confirm）与 `TestLagoSettleAmountGuardFailsClosed`（超面额/零额 fail closed）。修正后本重放第 4 步成功。

## 本目录文件

- `seed-run.txt` / `api-01-draft.json` / `api-02-publish.json` / `api-03-lago-plans-baseline.json` / `api-03-lago-plans.json`：本轮 seed 副本（原始输出落在 r4-flow3/ 已恢复历史原样）。
- 其余断言输出以本 README 记录为准（API 面未走 Playwright——浏览器脚本本身是本批修复对象而非生产行为）。

## 披露

- `ac4-sandbox-credentials-unavailable`：支付宝侧为本地 RSA stub 协议往返（真实 AlipayProvider 签名/验签链），非真实沙箱钱包付款证据。
- webhook 传输腿替身（D8）：Stripe 云端无法投递回环 URL，事件体=Stripe API 读回的真实 PI 对象、签名=provider 真实 secret 的真实 HMAC、接收/处理链 100% Lago 内建。
