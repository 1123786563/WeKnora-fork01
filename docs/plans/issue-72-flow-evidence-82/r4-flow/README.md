# Issue #82 R-4 复验（真实环境全链，2026-09-26）

- 验证者：流程验证员-82（dynamic workflow 复验轮）
- 分支：`codex/issue-72-lago` @ `4de51f8c0`（集成分支 worktree `.worktrees-issue72/lago-int`）
- 计划：`docs/plans/issue-72-plan-82.md`「真实流程验证方案」（R-4 双轨道修订版）
- 本轮补齐上轮遗留的**浏览器 UI 面**（上轮 README 声明「浏览器 UI 面本轮未实跑，留给复验轮」）

## 环境拓扑（全部真实运行）

| 组件 | 地址 | 说明 |
|---|---|---|
| Lago v1.53.0 | 127.0.0.1:48889 | 复用 `weknora-lago-82flow` 栈（healthy）；org `WeKnora Issue82 Flow`；Stripe provider `weknora-stripe`（webhook secret 本轮经 `prepare_82flow_webhook_secret.sh` 用真实 Stripe API mint 并经 Lago model 层存入——与 t9 harness 同边界） |
| WeKnora 后端 | 127.0.0.1:8093 | 集成分支 worktree `go build ./cmd/server`；`DB_DRIVER=sqlite DB_PATH=data/issue82-r4verify.db`（独立库）；`WEKNORA_COMMERCIAL_PLATFORM_URL=http://127.0.0.1:48889`；Stripe PM `pm_card_threeDSecure2Required`（gating）/`pm_card_visa`（settle）；`WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true`；`SSRF_WHITELIST_EXTRA=127.0.0.1` |
| WeKnora 前端 | localhost:5194 | vite dev，proxy→8093 |
| 支付宝网关 stub | 127.0.0.1:8294 | `../alipay_gateway_stub.py`（`FLOW82_ALIPAY_PORT`，本轮脚本修复：端口 env 化）；密钥对 `/tmp/flow82-keys`（本地 RSA，`FLOW82_KEY_DIR`） |

测试数据：租户 7 `settle-r4-a@verify.local`（浏览器主角）/ 租户 8 `settle-r4-b@verify.local`（发布者，plan_publish grant 种子行）；占位租户 1-6 吸收 82flow 栈上既有 Lago 身份（weknora-tenant-1..6 有旧 incomplete 残留，不触碰）。Plan `weknora-pro-v1`（9900 分 CNY monthly）经 `/api/v1/admin/plans/drafts`+`/publish` 发布并落 Lago（`api-01/02-*.json`、`api-03-lago-plans.json`）。

订单：`ord_4c6f2f41fb73f40e`（租户 7，支付宝渠道，99.00 CNY）。

## 断言结果（Issue #82 用户流程 → 计划判据 a-d）

| # | 断言 | 结果 | 证据 |
|---|---|---|---|
| 1 | 发布 Plan → Lago 落库 | ✅ | `api-02-publish.json` receipt `publish_plan_version:pro:1`；`api-03-lago-plans.json` 含 weknora-pro-v1 9900 |
| 2 | 浏览器登录 → checkout 选支付宝 → CreateOrder 出扫码 | ✅ | `browser_01_checkout.mjs` 9/9：默认+显式选支付宝、提交体 provider==alipay、待付款面、前往支付链接、报价明细 ¥99.00、订单号；stub `PRECREATE out_trade_no=mo_8f3230422cb1a35f total_amount=99.00`（`lago-gated-before-settle.txt`） |
| 3 | Billing 行「待付款（权益未开放）」 | ✅ | `02-billing-awaiting-payment.png` |
| 4 | Lago gated 形状（incomplete+activation_rules+open invoice） | ✅ | `lago-gated-before-settle.txt`：subscription incomplete + payment/timeout_hours:0/pending；invoice status=5(open) 1650 分（proration，F12 披露口径） |
| 5 | AC2 同步返回面不推进 | ✅ | `browser_02_sync_face.mjs` 3/3 + `03-sync-return-no-confirmation.png`：回访+刷新×3+reload+轮询后仍「待付款（权益未开通）」，全页无「已付款/已生效」 |
| 6 | 可信回调（匿名+验签）→ 订单 paid | ✅ | `callback-01-first.txt`：POST /callbacks/alipay → 200 success；purchase `paid_awaiting_activation`、order.payment=paid |
| 7 | 「已付款待激活」中间态（合成态） | ✅ | `browser_03_paid_face.mjs` 4/4 + `04/05-*.png`：checkout「已付款，权益处理中」无「已生效」；billing「已付款待激活」 |
| 8 | settle 驱动（α 双轨道轨道二）Stripe gated 扣款 | ✅（有缺陷 F-1，见下） | `settle-run.txt`：Stripe 侧 gating PI `pi_3UJxpV` 由 settle 驱动 attach pm_card_visa clone + update + confirm → **succeeded 1650 CNY**（latest_charge ch_3UJxpV…） |
| 9 | webhook 投递 → Lago 内建链 finalize | ✅ | `webhook-01-delivery.txt`：真实 PI 对象+真实 secret 签名 → `POST /webhooks/stripe/{org}` 200；订阅 **active**、invoice **finalized+numbered WEK-A27D-012-001+payment_status=succeeded** |
| 10 | 后端观察到 active → D6' 复核 → grant → fulfilled | ✅ | `settle-run.txt`：fulfillment_record `purchase_activation=applied`（external_id=weknora-tenant-7-purchase-2026-09）；purchase API state=active、order.fulfillment=fulfilled |
| 11 | 「已生效」面 | ✅ | `browser_04_active_face.mjs` 3/3（复跑稳定）+ `06/07-*.png`：checkout「权益已生效」、billing「已生效」 |
| 12 | Credits 到账（购买钱包） | ✅ | `lago-four-objects-after-settle.txt`：钱包 `weknora-tenant-7-purchase-2026-09` + wallet_transactions 恰 1 行 granted 9.9 |
| 13 | Entitlement 到账 | ⚠️ Lago 侧✅/产品读取❌ | Lago 侧 `/subscriptions/weknora-tenant-7-purchase/entitlements` 含 `advanced_models`；但产品代码 `readCustomerFeatures` 调 `/api/v1/customers/{id}/entitlements`（lago.go:1075）在 pinned v1.53 **无此路由**（entitlements 只挂 subscriptions/plans 嵌套）→ Benefits 面 features 读不到（缺陷 F-2） |
| 14 | 后台 Lago 四对象（Invoice/Payment/active Subscription/钱包） | ✅ | `lago-four-objects-after-settle.txt`：sub active / invoice finalized+numbered+succeeded 1650 / payments succeeded 恰 1（pi_3UJxpV；另有 gated-create 遗留 failed 行非本次产物）/ 购买钱包+批次 |
| 15 | AC3 重复回调只入账一次 | ✅ | `callback-02-replay.txt`：重放×2 均 200 success，orders/outbox/activation 计数不变 |
| 16 | AC3 重复 webhook no-op | ✅ | `webhook-02-replay.txt`：同 event id 重投 200，四对象 byte-identical |
| 17 | AC3 kill+重启 drain 收敛 | ✅ | `06-replay-idempotent.txt`：重启后 attempt 停 19、record 仍 applied；最终计数：订单 1（fulfilled）/outbox fulfill 1/activation 1（applied）/Lago succeeded payments 1/购买钱包批次 1 |
| 18 | (f) go 回归 9 包 | ✅ | `gotest-regression.txt` 全 ok（commercial 7 包+handler+router） |
| 19 | (h) T9 集成真实栈 | ✅ | `t9-integration-run.txt`：t11 栈（:48895）3 子测试 PASS 非 skip（24.5s） |
| 20 | (e) 红线 | ✅ | `redline-checks.txt`：凭据字面量 0 命中、本地 force-active 0 命中；modulemove OK(16)/architectureguard OK(0 violations) |
| 21 | (g) pnpm test:web + typecheck:web | ⚠️ | **全量 FAIL（预存/环境性，#82 无关面）**：失败于 Vue 消息/文档测试与 DevMarkdownPage/PlatformShell/mermaid 类型错（本机 node v22.22.3 < engine 要求 >=26；#82 merge 仅触碰 commercial 5 文件）；**#82 商业面子集全绿**：apps/web src/commercial 4 文件 22/22 PASS + packages/contracts commercial.test.ts PASS |

## 产品缺陷（failures，未修改产品代码）

1. **[高] F-1 settle 幂等键跨 PaymentIntent 碰撞 → 激活窗口内 fulfillment 误标 attention**
   Lago v1.53 gated create 会留下**两笔** unsettled gating PI（首次 confirm 3DS 失败剥离 pm 的 `requires_payment_method` PI#1 + `requires_action` PI#2，本单实证 pi_3UJxpK/pi_3UJxpV，同 `metadata.lago_invoice_id`）。settle（lago_settlement.go:140-156）用 `cmd.Key+:update`/`:confirm` 作 Stripe Idempotency-Key 驱动**最新** PI#2 → succeeded；下一轮 drain 的 settle 幂等重放时，PI#1 仍满足定位谓词（unsettled+metadata，`lago_settlement.go:210-215`）→ 同键打 PI#1 的不同 endpoint → Stripe 400 idempotency_error → `ErrPlatformInvalidResponse` → `markActivationState(attention)`（purchase_fulfillment.go:171）。本验证实证：webhook finalize 后 drain 重放恢复、attention 被 APPLIED 覆盖（L256 自愈语义成立），但 (a) settle-vs-finalize 窗口内 fulfillment 记录被误标（每 30s 一条 attention 警告）；(b) `hasSettledGatingIntent` 幂等窗口（lago_settlement.go:104-116）被残留 PI#1 短路（intents 永不空）；(c) 若 Stripe 键绑定行为变化或旧 PI 先于新 PI 被驱动，存在第二笔真实扣款风险面。修复方向（供修复员参考，未实施）：定位谓词或幂等回执应绑定 invoice 身份——同 customer 存在 succeeded intent 携带同 `lago_invoice_id` 时直接幂等返回（把 `hasSettledGatingIntent` 从「intents 空才查」改为谓词的一部分），或幂等键掺入 intent id。
2. **[中] F-2 `readCustomerFeatures` 路由在 pinned Lago v1.53 不存在 → Entitlement 权益面恒缺失**
   `internal/modules/commercial/commercialplatform/lago.go:1075` 调 `GET /api/v1/customers/{external_customer_id}/entitlements`；实测 pinned v1.53 的 entitlements index 只挂 `/api/v1/subscriptions/:external_id/entitlements` 与 `/api/v1/plans/:code/entitlements`（容器 routes 实读 + rails routes 列表）→ 404 → Benefits 快照 Features 恒空。Lago 侧 entitlement 事实已物化（本验证经 subscriptions 路径读到 `advanced_models`），仅产品读取路径失配。
3. **[低/环境披露] F-3 pnpm test:web / typecheck:web 全量失败（非 #82 面）**
   本机 node v22.22.3（engine 要求 >=26）。失败集中于 Vue 消息/文档测试（~18+ not ok：metadata editor/chunk retry/trace drawer/document detail 等）与 `DevMarkdownPage.tsx`/`PlatformShell.tsx`/`packages/views/src/chat/mermaid.ts` 类型错——#82 merge（4de51f8c0）只触碰 commercial 5 文件，且 commercial 子集全绿。判据 (g) 按全量口径**未过**，按 #82 范围口径过；建议在满足 engine 的 node 环境复跑确认。

## 观察项（非缺陷）

- Lago gated create 的 failed+requires_action 双 payment 行（含 unsettled PI 残留）是 F-1 的输入面，属 pinned 版本固有形状（上轮 flow evidence 同形态）。
- gating invoice 首期 1650 分 = 9900×5/30 proration 剪裁（9/26 下单、周期对齐 9/30）——F12/D6' 已披露口径，`0 < fee ≤ AmountFen` 判据通过（fulfilled 佐证）。
- 首次 `browser_04` 的 no-awaiting-leftover 断言曾 FAIL 一次（purchase 投影轮询切换瞬态：waitForSelector 命中「权益已生效」时旧「已付款，权益处理中」Status 行尚未被替换），复跑 3/3 PASS——UI 竞态记录在案，非状态机错误。

## 披露（不变）

- `ac4-sandbox-credentials-unavailable`：支付宝侧为本地 RSA stub 协议往返（真实 AlipayProvider 签名/验签链），**非真实沙箱钱包付款证据**。
- webhook 传输腿替身（D8）：Stripe 云端无法投递回环 URL，事件体=Stripe API 读回的真实 PI 对象、签名=provider 真实 secret 的真实 HMAC、接收/处理链 100% Lago 内建（`webhook-01-delivery.txt`）。

## 脚本（本目录，可重放）

- `seed.sh`（占位租户+主角+grant+plan 发布；`FLOW82_R4_PW`、`DB_PATH` env）
- `browser_01_checkout.mjs` / `browser_02_sync_face.mjs` / `browser_03_paid_face.mjs` / `browser_04_active_face.mjs`（`FLOW82_WEB/FLOW82_EMAIL_A/FLOW82_PASSWORD_A` env；01 输出 `order-info.json`）
- `prepare_82flow_webhook_secret.sh`（82flow 栈 provider secret mint+model 层存入）
- `notify-body.txt`（签名 notify，`../alipay_sandbox_notify.py` 生成）
- `../alipay_gateway_stub.py` 本轮修复：`FLOW82_ALIPAY_PORT` 端口 env 化（计划 Task 10 要求的端口参数化落地）
