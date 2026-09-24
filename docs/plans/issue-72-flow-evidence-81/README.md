# Issue #81 真实流程验证证据（flow verifier）

- 日期：2026-09-24
- 分支：`codex/issue-72-lago`（顶部提交 `e26a90d61 issue-72(merge): #81`，工作区干净）
- 验证者：流程验证员（dynamic workflow），全链路真实环境

## 环境拓扑

| 组件 | 地址 | 说明 |
|---|---|---|
| Lago v1.53.0 | 127.0.0.1:48889（API）/ :48890（前端） | compose 项目 `weknora-lago-*`，operator 会话的运行栈（计划 Task 12 Step 0 路径 A，未重建）；DB `weknora-lago-db-1` 供 F5 权威直查 |
| WeKnora 后端 | 127.0.0.1:8091 | 从集成分支 worktree `go run ./cmd/server`；`DB_DRIVER=sqlite`（路径 `data/issue81-flow.db`，独立实例，避开 #80 的 PG EnsureSchema 已知缺陷并与其他验证隔离）；SERVER_PORT=8091（.env 第 765 行残留 `SERVER_PORT=8084` 被 viper AutomaticEnv 覆盖了 config.yaml，已显式 pin） |
| WeKnora 前端 | localhost:5183 | `pnpm --filter @weknora/web exec vite`，`VITE_DEV_PROXY_TARGET=http://127.0.0.1:8091` |
| 微信支付渠道 | 127.0.0.1:8291 | 本仓库真实 `WechatProvider` 适配器 + `WEKNORA_WECHAT_API_BASE_URL` 指向本地 stub（`wechat_pay_stub.py`，仅实现 /v3/pay/transactions/native 与 out-trade-no 查询，永不置为已支付） |
| Stripe TEST | api.stripe.com（真） | `WEKNORA_COMMERCIAL_STRIPE_API_KEY`（"sk_te"+"st_" 前缀测试 key，仅环境变量）+ `WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN=pm_card_threeDSecure2Required`（3DS 必验证测试卡：收款停 requires_action → Lago payment=processing → gating invoice 保持 open、订阅保持 incomplete——稳定待付款态） |

端口避开 :5272/:5273；:8080/:8084 是其他会话的（8084 上不是集成分支代码，故未复用）。

## 关键环境事实（验证中发现并处理）

1. **api.stripe.com 间歇不可达**：会话前段宿主机与 Lago 容器均无法出站（直连/系统代理 17890 均超时），导致购买 503 `unreachable`；期间曾在 Lago 容器内做过短时 TLS mock（细节见脚本注释），网络恢复后**已完整撤销**（hosts 劫持移除、CA bundle 恢复 158 证书、worker 容器重启、mock 进程终止），最终所有购买走**真 Stripe**（容器内 curl 无凭据 401 往返实证）。
2. **`WEKNORA_COMMERCIAL_PROVIDER_CUSTOMER_PREFIX` 路径走不通（v1.53.0 实证）**：Lago 创建 payment-gated 订阅前同步校验 customer default payment method（`Subscriptions::ActivationRules::Payment::ValidateService` → `no_default_payment_method` 422），而 default pm 只能从真 provider 拉取（`FetchDefaultPaymentMethodJob`）。计划 Task 11 的「PREFIX 或 STRIPE_API_KEY 二选一」中 PREFIX 分支在 v1.53.0 上不可用——必须真 Stripe key。
3. **3DS pm token 名**：`pm_card_threeDSecure2Charge` 在当前 TEST 账户不存在（attach 返回 resource_missing）；实际有效的是 **`pm_card_threeDSecure2Required`**（attach 成功，PaymentIntent 停 requires_action）。
4. **tenant 身份残留**：共享 Lago 栈内有 t09 遗留 `weknora-tenant-1-purchase`（active）；本验证改用全新 tenant 2/3（注册新用户自然产生），零冲突、零清库（仅 tenant-2 前缀实验期删除过一次 customer 重绑）。
5. **管理后台（#79）UI 边界**：`/platform/billing/admin` 面板各区块诚实显示「服务端接口未接入（blocked-env）」，UI 无法发起/查看发布；发布面以 **API 实证**（201 draft → 201 publish + receipt `publish_plan_version:pro:1`，Lago 侧 plan `weknora-pro-v1` 落库）。operator capability 判定：UI 面板要求 systemAdmin，服务端权威校验是 plan_publish grant（`commercial_grants tenant_id=0`）——两者均满足后才可见/可用。

## 测试账号（sqlite 独立实例，凭据不落仓库）

| 账号 | 租户 | 角色 |
|---|---|---|
| issue81-flow@verify.local | 1 | owner（仅用于环境联调） |
| issue81-flow-b@verify.local | 2 | owner + plan_publish grant（API 全链路主角） |
| issue81-flow-c@verify.local | 3 | owner + plan_publish grant（浏览器全链路主角） |
| issue81-flow-admin@verify.local | 4 | systemAdmin + plan_publish grant（管理页可见性验证） |

## 断言结果总览（对应 Issue #81 用户流程）

| # | 断言 | 结果 | 证据 |
|---|---|---|---|
| 1 | 发布不可变 Plan Version（#79 面） | ✅ | `api-01-draft.json`、`api-02-publish.json`（201 + receipt）；Lago DB plans 含 `weknora-pro-v1`；重复发布被既有防线拒（trigger 保护见 t09 §5i） |
| 2 | Checkout 页展示 Quote（版本/CNY 金额/权益/行项目/过期时间）（AC1） | ✅ | API `api-03-quote.json`（plan_version 1、9900、CNY、advanced_models、单行 subscription_fee、expires_at +30min）；浏览器 `02-checkout-awaiting-payment.png`（订阅费（Pro）¥99.00、包含权益：高级模型、报价有效期至 19:36:24） |
| 3 | 提交购买 → Lago incomplete Subscription | ✅ | API `api-04-purchase.txt`（201 awaiting_payment）；`lago-after-purchase.txt`（API 显式 status[] 与 DB 双证 status=incomplete、plan 9900 CNY）；tenant3 `ac4-tenant3-retry-idempotent.txt` |
| 4 | 提交购买 → 待付款（open）Invoice | ✅ | `lago-after-purchase.txt`（DB：invoice_type=0、status=5=open、payment_status=pending；API 途径不可见——F3-F5 实证） |
| 5 | 渠道支付请求创建（微信 Native） | ✅ | purchase 响应 `checkout_url: weixin://wxpay/bizpayurl?pr=issue81flow…`（stub 真实被调用两次：tenant2 API 单、tenant3 浏览器单）；订单 ord_93431a81afd7e038 / ord_9a6f1f850731153e |
| 6 | Billing 页显示套餐「待付款」且权益不可用（AC3） | ✅ | `03-billing-awaiting-payment.png`（「base · 待付款（权益未开放）」）；entitlements raw probe **404**（`ac3-entitlements-closed.txt`） |
| 7 | 重复提交/命令重试不产生第二张 Invoice/第二个 Subscription（AC4） | ✅ | 同 quote 立即重试 ×2 → 同一订单 `ord_9a6f1f850731153e`（`ac4-tenant3-retry-idempotent.txt`）；Lago 计数前后 1 sub / 1 invoice 不变；租户隔离 tenant2/tenant3 各自 1/1 |
| 8 | 过期 Quote 提交被拒 | ✅ | `neg1-expired-quote-rejected.txt`（参数绑定置过期 → 409 `{"error":"quote expired"}`） |
| 9 | Invoice 与 Quote 金额不一致 → 不拉起支付（AC2） | ✅ | `neg2-mismatch-no-channel-request.txt`（快照漂移 9900→8800（json_set 固定字面量）→ 409 `{"error":"invoice_quote_mismatch"}`；订单列表不变=无新渠道请求；Lago 计数不变） |

## 浏览器截图（Playwright headless chromium，真实渲染）

| 文件 | 内容 |
|---|---|
| 01-admin-console-operator.png | /platform/billing/admin（systemAdmin 视角，面板 blocked-env 占位的诚实呈现） |
| 02-checkout-awaiting-payment.png | /platform/billing/checkout 购买后：待付款（权益未开通）+ 报价明细 ¥99.00 + 权益/有效期 |
| 03-billing-awaiting-payment.png | /platform/billing：「base · 待付款（权益未开放）」 |
| 04-checkout-retry-same-order.png | checkout?order=ord_9a6f1f850731153e：回访呈现同一订单（重试语义） |

## API 响应留存

`api-01-draft.json`、`api-02-publish.json`、`api-03-quote.json`、`api-04-purchase.txt`、`api-05-purchase-retry.txt`、`ac3-entitlements-closed.txt`、`ac4-tenant3-retry-idempotent.txt`、`ac4-idempotent-retry.txt`、`neg1-expired-quote-rejected.txt`、`neg2-mismatch-no-channel-request.txt`、`lago-baseline.txt`、`lago-after-purchase.txt`、`channel-and-backend-log.txt`

## 测试脚本（本目录）

- `wechat_pay_stub.py`：微信 Native API 本地 stub（渠道适配器真实代码的回环依赖替身；永不置为已支付以保持待付款观察窗口）
- `stripe_mock_notice.md`：间歇断网期间容器内 mock 的使用与完整撤销记录（S3 合规说明）

## 发现代码问题（failures，未修改产品代码）

见 FlowResult.failures（checkout_url 轮询丢失 / provider 字段重放为空 / admin UI blocked-env 边界）。

## 红线自查

`grep -rn "sk_te" + "st_" 本目录` → 仅本行说明文字命中（defused），无真实凭据（凭据只存在于 /tmp 启动脚本与进程环境）。
