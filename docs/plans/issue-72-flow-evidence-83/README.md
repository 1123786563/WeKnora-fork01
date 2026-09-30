# Issue #83 真实受控环境验收证据（微信付款复用同一激活流程）

- 日期：2026-09-27
- 分支：`codex/issue-72-lago-83`（worktree `.worktrees-issue72/issue-83`，含本票四个提交）
- 验证者：实现员-83（dynamic workflow），全链路真实环境（真实 Playwright chromium + 真实 vite 渲染 + 真实 Lago v1.53.0 + 真实 Stripe TEST 结算腿）
- 核心结论：**微信渠道走与 #82 完全相同的激活链（微信回调验签 → ConfirmPayment Payment Fact → outbox → PurchaseFulfiller settle → Lago 内建 webhook finalize → active），五幕全过**；渠道切轨（支付宝→微信）由真实浏览器面演练并覆盖关单竞态。

## 复验轮（v2，2026-09-27 12:2x，流程验证员-83，集成分支 `codex/issue-72-lago` @ `9ebd1c738`）

同拓扑独立重跑（新 sqlite 库 `data/issue83-flow-v2.db`、新一次性密钥、新主角租户 39/40、Lago 82flow 栈彼时已被外部轮占到 tenant-34 故占位 38 个）：**五幕全部关键断言在真实环境复现通过**。

| 幕 | v2 结果 | 证据 |
|---|---|---|
| 幕一 浏览器主链 | ✅（渠道单选/微信单 weixin:// 链接/stub NATIVE 9900/签名回调 200/paid_awaiting_activation 双页面/settle 真 Stripe PI `pi_3UK97Q...` succeeded/webhook 投递 200/active 双页面） | `01-05-*.png`（本轮 11:51/12:18 拍摄）、`act1-browser-run-v2.log`、`purchase-state-progression-v2.txt` |
| 幕二 漏通知查单恢复 | ✅ 4/4 | `recovery-no-notify.txt` |
| 幕三 重复通知幂等 | ✅ 3/3（fulfilled/fulfill/attempt/Lago payments 各恰 1） | `duplicate-notify-idempotent.txt` |
| 幕四 关单竞态+切轨+晚到成功 | ✅ 12/12 | `close-race-paid.txt`、`close-race-switch.txt` |
| 幕五 四对象+产品面 | ✅ 6/6 | `lago-four-objects-wechat.txt`、`account-after-wechat.json` |
| 回归+红线 | ✅ 8 包 ok、architectureguard 0 violations、stripe test key 前缀字面量 0 命中、本地强制 active 0 命中 | `act2345-api-run-v2.log` |

v2 轮勘误与脚本修复（均为**验证侧脚本/凭据注入问题**，非产品缺陷；产品代码零改动）：

1. 首次生成的运行时 Lago key env 文件把 key 截断到 10 字符（管道拼装截断）→ `publish_conflict` 409；分步生成+长度校验后恢复（`seed-run.txt` 有记录）。
2. webhook secret 经 rails runner 导出时混入尾部 Sidekiq-Pro 警告行，`tail -1` 取错 → 幕一脚本的 settle 轮询投递 24 轮全败；**settle 腿本身已真实成功**（Stripe PI 03:51:16Z succeeded），改用 `grep '^whsec_'` 精确导出后一次投递即 active（04/05 截图为同页面补拍，披露见 progression-v2）。
3. `api_recovery_83.mjs`/`seed_83.sh` 的轮次硬编码（tenant 20、订单号、db 路径、seed-login-f 依赖手工 cp）参数化——`FLOW83_DB`/`FLOW83_TENANT`/fulfilled 订单动态发现（重跑友好）。
4. README 原文「红线」行自身含 stripe test key 前缀字面量使 grep 无法归零——改为描述性文字，红线复跑 0 命中。
5. 新增 v2 运行脚本：`v2_gen_keys.sh`（一次性密钥）、`v2_up_stubs.sh`（stub 启动+冒烟）、`v2_up_backend.sh`（后端全套 env，密钥仅经运行时 0600 env 文件注入）、`v2_probe.sh`（run_in_background bash3.2 PATH 差异探测，排障留档）。


## 环境拓扑（全部真实运行）

| 组件 | 地址 | 说明 |
|---|---|---|
| Lago v1.53.0 | 127.0.0.1:48889 / :48890 | 复用运行中的 `weknora-lago-82flow` 栈（docker ps 实测 healthy；Stripe provider `weknora-stripe` 既有注册，真实 Stripe TEST key 仅环境变量注入） |
| WeKnora 后端 | 127.0.0.1:8095 | 本 worktree `go run ./cmd/server`；sqlite 独立新库 `data/issue83-flow.db`；商业平台 env → Lago :48889；Stripe 腿 `source ~/.zcode/issue72-stripe.env`（SETTLE pm_card_visa）；`SSRF_WHITELIST_EXTRA=127.0.0.1`（环回 stub 可审计豁免） |
| 微信 Native stub | 127.0.0.1:8296 | `wechat_native_stub.py`（本票交付）：真实 WechatProvider 的协议对端——出站 WECHATPAY2-SHA256-RSA2048 签名被 stub 用商户公钥验证（fail-closed 401）、AES-256-GCM 签名回调推送（平台私钥签名，断言 WeKnora 答 200 `{"code":"SUCCESS"}`）、可编排状态机（mark SUCCESS/CLOSED + close 撞在途支付回 400 ORDER_PAID——与 Task 1 pin 的错误形状同契约） |
| 支付宝 stub（切轨腿） | 127.0.0.1:8297 | 复用 `../issue-72-flow-evidence-82/alipay_gateway_stub.py` + 本地 RSA 密钥重生成（仅作切轨剧本的新渠道对端） |
| WeKnora 前端 | localhost:5196 | `VITE_DEV_PROXY_TARGET=http://127.0.0.1:8095 pnpm --filter @weknora/web exec vite --port 5196` |
| Playwright | headless chromium | 仓库 devDependencies `@playwright/test`（`browser_flow_83.mjs`） |
| Stripe→Lago webhook | 替身投递 | `../issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py`（D8 传输腿替身：从真实 Stripe API 读回 succeeded PI，HMAC 签名投 Lago 内建 webhook 路由——接收/验签/处理链 100% Lago 内建） |

一次性本地密钥（运行时目录 `${TMPDIR}issue83-keys/`，不进仓库）：微信商户 RSA、平台 RSA、32 字节 apiv3 key、支付宝 RSA。

## 种子数据（sqlite 独立新库，Lago 独立租户，零污染）

- 占位注册 14 个吸收 82flow 栈上已被早期轮占据的 Lago 租户号（实测该栈已被外部轮占到 weknora-tenant-15；主角最终落 tenant 20/21+）
- `issue83-flow-f@verify.local`（tenant 20）：浏览器主链主角
- `issue83-flow-b@verify.local`（tenant 16）：plan_publish 授权发布者；Plan `pro` v1（9900 分 CNY、monthly、无 charges、advanced_models）幂等发布落 Lago `weknora-pro-v1`（`seed-run.txt`/`api-0{1,2,3}-*.json`）

## 五幕断言结果（AC → 可观察结果）

| 幕 | 断言 | 结果 | 证据 |
|---|---|---|---|
| 幕一 AC1 主链（浏览器） | 渠道单选默认支付宝、切微信（页面显式切轨入口 → 后端关旧支付宝单+开新微信单）→「待付款（权益未开通）」+ `weixin://wxpay/` 前往支付链接 + stub NATIVE 9900 CNY | ✅ 12/12 | `01-checkout-wechat-awaiting-payment.png`、`browser_flow_83.mjs` PASS 行 |
| 幕一 | 签名回调推送（AES-GCM + 平台私钥签名）→ WeKnora 200 `{"code":"SUCCESS"}` →「已付款，权益处理中」/「已付款待激活」双页面 | ✅ | `02/03-*.png`、stub `NOTIFY PUSH` 日志 |
| 幕一 | settle（真 Stripe PI off-session confirm）→ webhook finalize → active →「权益已生效」双页面、无待付款/已付款残留 | ✅ | `04/05-*.png`、`purchase-state-progression.txt`（awaiting_payment → paid_awaiting_activation → active） |
| 幕二 AC2 | 漏通知：mark SUCCESS **不推 notify** → `GET /orders/:id` 查单恢复 → paid → settle → active | ✅ 4/4 | `recovery-no-notify.txt` |
| 幕三 AC2 | 同一签名通知重投两次：两次 200 SUCCESS；fulfilled/outbox fulfill/succeeded attempt 各恰 1；Lago succeeded payments 恰 1 | ✅ 4/4 | `duplicate-notify-idempotent.txt` |
| 幕四 AC3 | 关单撞在途支付：切支付宝时微信 CLOSE 回 400 ORDER_PAID → Query 决胜 SUCCESS → **答已付旧微信单**、支付宝 PRECREATE 零调用、fulfill 恰 1、竞态后仍 active | ✅ 6/6 | `close-race-paid.txt` |
| 幕四 AC3 | 干净切轨：微信 CLOSE 204 → 旧单 channel_failed+attempt closed、CurrentPending 只剩新支付宝单（qr.alipay.com 链接） | ✅ | `close-race-switch.txt` |
| 幕四 AC3 | 已关旧单晚到成功（防御性）：200 收单、事实保留、无第二次履约 | ✅ | 同上 |
| 幕五 AC4 | Lago 四对象：subscription active 恰 1、gating invoice finalized+succeeded（API 字符串枚举直读；fee 1320=4/30 日剪裁面，82 已披露的 proration 形状）、succeeded payment 恰 1、purchase 钱包 granted 9.9 恰 1 笔 | ✅ 6/6 | `lago-four-objects-wechat.txt` |
| 幕五 | 产品面：credits 10.9=base 1.0+购买 9.9、features `advanced_models:true` | ✅ | `account-after-wechat.json` |
| 回归+红线 | `go test ./internal/modules/commercial/... ./internal/handler/ -count=1` 8 包 ok；`make check-backend-architecture` 0 violations；stripe test key 前缀字面量 0 命中；本地强制 active 0 命中 | ✅ | 本 README「红线」节；Ledger |

## 验证过程中发现并修复的实现缺陷（TDD，commit 6ef004e3a）

**WechatProvider.Query 丢 transaction_id**：渠道查单响应同时携带 `out_trade_no` 与 `transaction_id`，原实现只解析前者——漏通知查单恢复路径把 attempt 的 `provider_transaction_id` 记成 out_trade_no，而晚到的真实回调携带真 `transaction_id`，ConfirmPayment 的 sameTxn 幂等判断失配：同一笔付款被当成两笔（假 over-payment 审计），或撞 `(provider, merchant, provider_transaction_id)` 唯一约束答 500（本验证首轮实证：回调 500 后由查单恢复救活）。修复后查单与回调对同一笔付款记录同一事实（spec L127 duplicate fact unity）。特征测试 `TestWechatQueryPrefersTransactionID`（RED→GREEN）钉住。

## 复现方式（概要）

```bash
# 密钥：openssl genrsa 2048 → mch_private/mch_public/platform_priv/platform_pub；head -c 32 /dev/urandom > apiv3.key
# 微信 stub：WECHAT_STUB_* env（key 目录/appid/mch/serial/notify url）python3 wechat_native_stub.py（:8296）
# 支付宝 stub：FLOW82_ALIPAY_PORT=8297 FLOW82_KEY_DIR=… alipay_gateway_stub.py
# 后端 :8095：source ~/.zcode/issue72-stripe.env + 全套 WEKNORA_WECHAT_*/WEKNORA_ALIPAY_*/平台 env（见 Ledger）
# 前端 :5196 → FLOW83_* env 后 node browser_flow_83.mjs && node api_recovery_83.mjs
```

## 披露边界（不伪造）

- **微信腿为本地 RSA/AES 协议 stub**（真实商户号/平台证书不可得）：证明「微信协议往返 + 出站/回调双向验签 + 同一激活链」，**不构成真实微信钱包付款证据**——对称适用 R-4 对支付宝的披露口径。AC4 的「不以支付宝证据替代」由本证据包的独立微信腿满足（支付宝 stub 仅作切轨剧本的新渠道对端，其付款链不在本票断言内）。
- **Stripe→Lago webhook 传输腿为替身**（D8）：Lago API 在环回，Stripe 云端无法直投；替身只做「读回真实 PI + HMAC 签名投递」这一腿，接收/验签/处理链 100% Lago 内建。settle 腿为真实 Stripe TEST API 调用（真实 PI confirm）。
- 共享 82flow 栈上存在其他验证轮的遗留数据（本验证以占位注册 + 独立租户隔离）；`deliver_stripe_webhook.py` 的 webhook secret 经容器内 rails runner 只读导出（不修改共享栈状态）。
