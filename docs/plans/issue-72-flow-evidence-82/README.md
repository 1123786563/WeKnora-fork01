# Issue #82 真实流程验证证据（flow verifier）

- 日期：2026-09-25
- 分支：`codex/issue-72-lago`（集成分支 worktree `.worktrees/issue72-lago`，顶部提交 `8cfc91975 issue-72(merge): #82`）
- 验证者：流程验证员-82（dynamic workflow），全链路真实环境
- 计划处置声明：`issue-72-plan-82.md` 头部注记——**D2 结算触发链已被 t10 证伪，Task 5-10 冻结，「真实流程验证方案」第 5-8 步（激活链）不可执行**；本验证执行第 1-4 步（已落地能力面）+ 已落地测试回归 + 红线自查，failures 按判据 (d) 记录 `d2-falsified-t10-p2-fail`。

## 环境拓扑（全部真实运行）

| 组件 | 地址 | 说明 |
|---|---|---|
| Lago v1.53.0 | 127.0.0.1:48889（API）/ :48890（前端） | compose 项目 `weknora-lago-82flow`（**独立数据卷**——宿主遗留卷属旧栈，密码不匹配已绕开，未触碰任何旧卷）；digest 锁定 5 镜像全 healthy；seed org `WeKnora Issue82 Flow`（一次性本地凭据仅 deploy/lago/.env）；Stripe provider `weknora-stripe` 经 GraphQL AddStripePaymentProvider 注册（真实 Stripe TEST key，仅环境变量注入） |
| WeKnora 后端 | 127.0.0.1:8092 | 集成分支 worktree `go run ./cmd/server`；`DB_DRIVER=sqlite`、`DB_PATH=data/issue82-flow.db`（#82 专用独立库）；`WEKNORA_COMMERCIAL_PLATFORM_*=lago→:48889`、`WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN=pm_card_threeDSecure2Required`（3DS 必验卡，F7 稳定待付款窗口） |
| WeKnora 前端 | localhost:5192 | `pnpm --filter @weknora/web exec vite --port 5192`，`VITE_DEV_PROXY_TARGET=http://127.0.0.1:8092` |
| 支付宝网关 stub | 127.0.0.1:8292 | `alipay_gateway_stub.py`（本目录）：真实 AlipayProvider 协议往返——出站请求 RSA2 签名被 stub 验证、precreate 响应 inner-JSON 签名被 provider 验证；**同一密钥对扮演商户/支付宝两侧**（本地 RSA，`alipay_verify_local*.pem`）；SSRF 豁免 `SSRF_WHITELIST_EXTRA=127.0.0.1` |
| 微信渠道 stub | 127.0.0.1:8291 | `wechat_pay_stub.py`（#81 helper 复用）：浏览器 checkout 固定 provider='wechat'（CheckoutPage.tsx:95），微信渠道走回环 stub |
| Playwright | headless chromium | 仓库 devDependencies `@playwright/test`（`browser_flow_82.mjs` / `browser_sync_face_82.mjs`，真实渲染 vite 页面） |

端口避开 :5272/:5273（其他会话）、:8080/:8084/:8091/:5183（#81 遗留口径）。

## 测试数据（sqlite 独立实例，Lago 独立栈，零污染）

| 账号 | 租户 | 角色 |
|---|---|---|
| issue82-flow-a@verify.local | 1 | owner（浏览器全链路主角，checkout 自动微信下单） |
| issue82-flow-b@verify.local | 2 | owner + plan_publish grant（API 主角：发布 plan + 支付宝下单） |

Plan：`weknora-pro-v1`（9900 分 CNY、monthly、pay_in_advance、无 charges、features advanced_models）。

## 断言结果总览（对应 Issue #82 用户流程 + 计划第 1-4 步）

| # | 断言 | 结果 | 证据 |
|---|---|---|---|
| 1 | 发布 Plan → Lago 落库 | ✅ | `api-01-draft.json`（201）/`api-02-publish.json`（200 + receipt `publish_plan_version:pro:1`）；`lago-plan-after-publish.txt`（plans 含 weknora-pro-v1 9900 CNY） |
| 2 | 浏览器 checkout 下单 → 「待付款」+ 报价明细 | ✅ | `01-checkout-awaiting-payment.png`（等待付款 + 待付款（权益未开通）+ 前往支付 + 报价明细 订阅费（Pro）¥99.00 + 高级模型 + 有效期 + 订单号）；`browser_flow_82.mjs` 5/5 PASS |
| 3 | Billing 页套餐行「待付款（权益未开放）」 | ✅ | `02-billing-awaiting-payment.png`（base · 待付款（权益未开放）） |
| 4 | Lago gated 订阅 incomplete + gating invoice open | ✅ | `lago-after-purchase.txt`（API 显式 status[] 读 `weknora-tenant-1-purchase`=incomplete、activation_rules payment/timeout_hours:0/pending；DB：订阅恰 1（A 两次下单不重复）、invoice invoice_type=0/status=5=open/pending） |
| 5 | 支付宝下单 → checkout_url 二维码（CreateOrder） | ✅ | `api-04-purchase-alipay.json`（201 awaiting_payment，`checkout_url=https://qr.alipay.com/issue82flow7d49bb9d3eb76ca4`）；stub 日志 `PRECREATE out_trade_no=mo_25b21f144b2453b3 total_amount=99.00 subject=WeKnora order ord_98bb65420987f6fe`（金额与订单一致） |
| 6 | 同步返回面不推进（AC2 渠道面） | ✅ | `03-sync-return-no-confirmation.png` + `browser_sync_face_82.mjs` 2/2 PASS（同一订单回访+轮询刷新仍「等待付款/待付款（权益未开通）」，全页无「已付款/已生效」） |
| 7 | 回调验签（真实 AlipayProvider.Verify）接受签名 notify | ✅ | `callback-01-first.txt`：404 `failure`（attempt 解析步）而非 401 signature rejected——RSA2 验签通过（签名链路与 `payment/alipay_sandbox_test.go` 同源） |
| 8 | 匿名渠道回调可达 | ❌ | **缺陷 B**：`callback-01-first.txt` 匿名 POST → 401 `missing authentication`（设计意图 publicly reachable，见 routes_commercial.go:104-108 注释） |
| 9 | 可信回调 → 入账 → 订单 paid | ❌ | **缺陷 C**：`callback-01-first.txt` 带 JWT POST → 404 `failure`；DB attempt 行 `merchant='alipay'`（order.go:320 写 providerName）vs notify `fact.Merchant=SellerID(2088...)` → `resolveByMerchantOrderID` 永不匹配 |
| 10 | 已付款待激活呈现（AC1 中态） | ❌ | 双重不可达：缺陷 B/C 阻断「已付款」+ `paid_awaiting_activation` 常量不存在（grep 零命中，Task 7/8 冻结）；`purchase-state-entitlements-closed.txt`：回调后 GET /purchase 仍 `awaiting_payment` |
| 11 | 订单/订阅/权益已生效 + Credits 到账（AC1 后半/AC4） | ❌ | `lago-four-objects-after-blocked-callback.txt`：订阅 incomplete、wallets=0、payments 仅 provider 尝试行（failed/requires_action）无 succeeded——D2 冻结（t10 证伪）+ 回调缺陷双重阻断 |
| 12 | 重复回调只入账一次（AC3） | 未达 | 入账链路被缺陷 C 阻断，幂等语义无从触发；`callback-02-replay.txt` 重放同为 failure 404、订单/outbox/fulfillment 计数 0 不变（失败面幂等） |
| 13 | 权益闭合（entitlements 不开放） | ✅ | `purchase-state-entitlements-closed.txt`：Lago entitlements raw probe 404 |
| 14 | 已落地测试回归 | ✅ | `go test ./internal/modules/commercial/... -count=1` 7 包全 ok；AC2 pin `TestSyncReturnCannotConfirmPurchase` + 既有 `TestAlipaySyncReturnNeverConfirms` PASS |
| 15 | 红线自查 | ✅ | `redline-checks.txt`：凭据字面量 grep 零命中、本地强制 active 零命中、architectureguard OK（0 violations）、modulemove OK（16 manifests） |

## 发现产品缺陷（failures，未修改产品代码）

1. **[高] 首购 503 `invalid_response`——PUT customer 在 pinned v1.53.0 无路由**（`defect-put-customer-404.txt`）
   `ensureProviderBinding` 对「已存在但未绑定」客户发 `PUT /api/v1/customers/:external_id`（lago_purchase.go:307）；v1.53.0 路由 `resources :customers, only: %i[create index show destroy]`（容器内 config/routes/shared_api.rb:19 实读）**无 update** → 404 → 503 invalid_response。**每个新租户首购必撞**（onboarding 先建无绑定 customer）。该路径由 #81 验证后的 OCR 修复 R1-V05（4435b183b）引入，未经真实栈验证。本验证以删除 Lago customer 走 POST 原子绑定分支绕过（副作用：丢失 onboarding DisplayName，仅测试环境操作）。
2. **[高] 渠道回调端点被全局 Auth 拦截（401）**（`callback-01-first.txt`）
   `POST /api/v1/commercial/callbacks/:provider` 设计为「publicly reachable and authenticated by provider signature verification」（routes_commercial.go:104-108），但 `middleware.Auth` 全局挂载（router.go:269）且 `noAuthAPI` 白名单（internal/middleware/auth.go:36-65）不含该路径——真实支付宝/微信服务器匿名回调一律 401，渠道推送不可达。
3. **[高] 回调 attempt 解析 merchant 错配（404 `failure`）**
   下单时 `commercial_payment_attempts.merchant` 写 `providerName` 字面量（internal/modules/commercial/service/commercial/order.go:320 `Merchant: providerName`），回调侧 `fact.Merchant` 为支付宝 `SellerID`（alipay.go:328 `Merchant: p.cfg.SellerID`）——`resolveByMerchantOrderID`（payment_callbacks.go:48-51 按 provider+merchant+merchant_order_id 查询）永不匹配 → 「no registered payment attempt」。微信渠道同理（Merchant=MchID vs 'wechat'）。**可信回调入账链路（#82 核心）在集成分支当前代码上不可用。**
4. **[机制级，计划已知] `d2-falsified-t10-p2-fail`**：激活链（settle→provider 轨道→active→Credits/Entitlement→已生效）按计划 t10 证据冻结，Task 5-8 未实施（`lago_settlement.go`/`purchase_fulfillment.go` 不存在、`paid_awaiting_activation` grep 零命中、BillingPage 仅 awaiting_payment 分支）。即使缺陷 2/3 修复，AC1 后半/AC3 authority 面/AC4 四对象关联/GC-3 仍不可达。
5. **[披露] `ac4-sandbox-credentials-unavailable`**：无支付宝沙箱凭据（本会话未持有 ALIPAY_* 沙箱配置）。替身：本地 RSA 密钥对 + 回环网关 stub + 同源签名 notify（`alipay_gateway_stub.py`/`alipay_sandbox_notify.py`）——证明的是「渠道协议往返 + 验签→ConfirmPayment 链路的验签步」，**不构成真实沙箱钱包付款证据**。

## 观察项（非缺陷，重议输入）

- **gating invoice 金额剪裁**：订阅 9/25 创建、周期对齐 9/30，invoice `total_amount_cents=1980`（=9900×6/30 日剪裁，payments 同额）。D6 复核判据「InvoiceFees AmountFen==AmountFen(9900)」在真实 authority 首期将失配（激活后 refused 风险）——Lago calendar billing 的 proration 行为，属 T02 §5 重议输入。
- **Lago customer create 422 不回滚**：provider 未注册时 POST customers 报 422 `payment_provider_not_found` 但 customer 行仍落库（实测两行同 external_id 并存，软删除语义），留下「存在但未绑定」的脏客户——叠加缺陷 1 使后续购买固定走 PUT 死路。

## 测试脚本（本目录）

- `alipay_gateway_stub.py`：支付宝网关回环 stub（precreate/query/close，请求验签+响应签名）
- `alipay_sandbox_notify.py`：ALI-02 同源签名 notify 构造器（openssl CLI，无三方依赖）
- `wechat_pay_stub.py`：#81 微信 Native stub 复用（浏览器 checkout wechat 渠道依赖）
- `browser_flow_82.mjs` / `browser_sync_face_82.mjs`：Playwright 真实渲染驱动（租户 A 下单/B 同步面）
- 密钥文件 `alipay_verify_local.pem`/`alipay_verify_local_pub.pem`/`alipay_merchant_local.pem`：本地测试密钥对（无真实凭据价值）

## 运行方式（复现）

```bash
# Lago（独立项目名避开旧栈数据卷）
cd deploy/lago && ./lago.sh init && cat >> .env <<EOF  # seed 见 README.md
LAGO_CREATE_ORG=true / LAGO_ORG_USER_EMAIL / PASSWORD / NAME / LAGO_ORG_API_KEY
EOF
./lago.sh up && ./lago.sh status
# GraphQL 注册 weknora-stripe provider（loginUser + x-lago-organization header）
# 后端（:8092）：source ~/.zcode/issue72-stripe.env 后按上文拓扑 export 全部 env，go run ./cmd/server
# stubs：python3 alipay_gateway_stub.py & python3 wechat_pay_stub.py &
# 前端：VITE_DEV_PROXY_TARGET=http://127.0.0.1:8092 pnpm --filter @weknora/web exec vite --port 5192
node browser_flow_82.mjs && node browser_sync_face_82.mjs ord_98bb65420987f6fe
```

## Settle 链验证记录（R-4 双轨道重做轮，2026-09-26）

前次冻结后按 R-4 裁决重做（t11 探针 P-A..P-E 全 PASS → settle 链落地），
本目录 `settle-evidence/` 与真实栈（weknora-lago-t11 :48895）完成：

- **t11 机制探针**：`docs/migrations/lago/t11-payment-settle-trigger/`（五判据
  全 PASS；D2' step (iv)——挂死 PI update pm + off-session confirm→succeeded——
  实证；webhook_secret 本地边界与 harness 替身见其 DECISION.md）。
- **T9 集成测试**：`settle-evidence/t9-integration-run.txt`（gated create→
  settle→真实 PI 事件投递→active+D6' 行项目复核→重放 no-op，真实栈 PASS 非
  skip）。
- **端到端 API 链**（真后端 :8093 + 支付宝回环 stub + t11 栈）：发布 plan→
  浏览器同款 API 下单（支付宝）→precreate→同步面不推进→签名回调 success→
  订单 paid、purchase 态 **paid_awaiting_activation**→outbox drain 驱动
  settle→webhook 投递→**active + fulfilled + 购买钱包（weknora-tenant-3-
  purchase-2026-09）applied 回执**；重复回调 success 且 fulfill 事件不增。
  四对象证据：`settle-evidence/lago-four-objects-after-settle.txt`
  （subscription active / invoice finalized+numbered+succeeded、fee 1650=
  proration 剪裁 F12 实证 / payments 恰 1 succeeded / purchase 钱包批次）。
- **披露不变**：`ac4-sandbox-credentials-unavailable`——支付宝侧仍为本地
  RSA stub（协议往返+验签证明），非真实沙箱钱包付款证据；浏览器 UI 面
  （Playwright 断言）本轮未实跑（jsdom 单测 12/12 覆盖三态与渠道选择器
  逻辑），留给复验轮。

## 结论

已落地面达成（步骤 1-3 + 步骤 4 同步面 + 验签面 + 回归 + 红线）；**回调入账与激活链不可达**（缺陷 2/3 实证阻断 + D2 冻结）——Issue #82 用户流程「可信异步回调写入 Lago Payment 后订单与 Billing 页变已生效，Credits/Entitlement 到账且重复回调只入账一次，后台 Lago 可查关联 Invoice/Payment/active Subscription」**整体未达成**，待缺陷修复与 T02 §5 重议后的修订版计划承接。

## 复验轮 r5-verify（2026-09-28，第 4 轮）

`r5-verify/README.md` 承接 R-19 移交的浏览器 UI 面：四腿 Playwright 断言
（待付款 → 已付款待激活 → 已生效 三态 + 同步面零推进）全部真实环境 PASS，
四对象（active subscription / finalized+numbered+succeeded invoice / 恰 1
succeeded payment / 购买批次 wallet）与重放三面（重复 notify / 重复
webhook / 后端重启）幂等实证，D11 paid 窗口防重开在真实栈复现（渠道侧整轮
恰 1 次 PRECREATE）。判据 (a)-(g) PASS（(g) 含 1 个本票未触碰的负载
timing flake 披露）；(h) 的 T9 集成测试实跑尝试如实披露（共享栈形状缺陷 +
专属栈 lab.env 缺失，不以 skip 冒充 pass）。沙箱残余
`ac4-sandbox-credentials-unavailable` 不变。**Issue #82 用户流程整体达成**
（R-4 双轨道 α：渠道真实收款 → Stripe Provider gated 结算 → Lago 内建
webhook finalize → active）。
