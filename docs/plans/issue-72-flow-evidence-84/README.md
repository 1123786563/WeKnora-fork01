# Issue #84 真实流程验证（异常付款不会扩大权益）

四幕证据：真实 Lago v1.53（82r5 栈 :48889/:48890）+ 本 worktree 构建的 WeKnora
后端（:8096，sqlite data/issue84-flow.db）+ 前端（:5197）+ 微信 Native v3
anomaly 副本 stub（:8298，金额/币种可覆盖）+ 支付宝 RSA stub（:8299）+
Stripe TEST 结算腿（凭据只经 `~/.zcode/issue72-stripe.env` source 注入）。

## 栈组成与起栈

| 组件 | 来源 | 端口 |
|------|------|------|
| WeKnora 后端 | `up_backend_84.sh`（env 形态照 83 轮 v2；凭据经仓库外 0600 secrets env） | 8096 |
| WeKnora 前端 | `pnpm --filter @weknora/web dev`（VITE_DEV_PROXY_TARGET=http://localhost:8096） | 5197 |
| 微信 anomaly stub | `up_stubs_84.sh` → `wechat_native_stub_anomaly.py`（#83 本体副本 + Step 0 改造） | 8298 |
| 支付宝 stub | 复用 82 目录 `alipay_gateway_stub.py` | 8299 |
| Lago | 宿主既有 `weknora-lago-82r5` compose | 48889/48890 |

种子：`seed_84.sh`（照 seed_83.sh 形态：pad 账号吸收共享栈历史 Lago 租户位、
主角 a（浏览器买家）/b（发布人，platform grant：plan_publish + refund_review）、
发布 pro 9900、断言发布落地 Lago）。本轮实际主角：a=租户 21（第一幕+处置）、
c=租户 23（第二/三幕）、d=租户 24（第四幕）。

## Step 0 stub 副本（审查 R2）

`wechat_native_stub_anomaly.py` = 83 本体 + 两处改造：
1. `/stub/mark` 接受可选 `total`/`currency` 覆盖（写 `override_total`/`override_currency`）；
2. 回调体与 Query 响应的 amount 同源取 `effective_amount(order)`（覆盖值优先）。

冒烟自测 `--selftest`：无覆盖 → 与 83 构造一致；有覆盖 → 回调解密后
amount==覆盖值（输出 SELFTEST OK，见通过判据）。

**支付宝侧实测结论（计划 Step 0 末项）**：`alipay_gateway_stub.py` 无任何
/stub/* 编排端点（do_POST 仅 precreate/query/close），**无需副本**——错金额/
多收款通知经 `alipay_sandbox_notify.py` 的 `--total-amount`/`--trade-no`
任意值参数直接构造（第二/三幕全部走它）；支付宝通知契约无币种字段，
错币种仅微信腿可造（已按计划披露）。

## 第一幕：错金额/部分付款/错币种不激活 + 事实落库 + 幂等终态（AC1）

（租户 21，微信单 `ord_7d56b5483540655f`，证据 `leg1-*.txt/png`）

1. 浏览器购买（微信渠道，stub code_url `pr=issue84` 前缀证明走 :8298 副本）→
   `leg1-01-awaiting.png`。
2. 部分付款：stub 编排实收 5000（面额 9900）→ 已验签回调 → **200
   `{"code":"SUCCESS"}` 终态**；重投同通知 → 仍 200；anomaly 恰 1 行
   `partial_payment` 9900/5000 awaiting_disposition；订单 pending v1、
   fulfill 事件 0（`leg1-anomaly-facts.txt`）。
3. 错币种变体（同单恢复金额 9900 + 覆盖 USD）→ 终态 200 +
   `currency_mismatch`（CNY→USD）行，分类各归其位。
4. GET 订单（触发恢复读）→ `fulfillment:"attention" / payment:"pending" /
   payment_attention:true`；**恢复读后订单仍 pending**（真栈缺陷修复验证，
   见「实抓缺陷」）。
5. 权威侧：Lago sub 零激活、payments 零新增（`leg1-authority.txt`、
   `leg1-lago-payments-count.txt`）。
6. UI：Checkout「付款异常（资金事实已记录，待处理）」`leg1-02-anomaly.png`、
   Billing 后缀「 · 付款异常（待处理）」`leg1-03-billing-anomaly.png`。

**实抓缺陷（本轮修复，commit f1eb7c167）**：第一轮执行中错币种通知落
anomaly 后，浏览器轮询触发 RecoverOrderStatus——其 collected-amount 比对
**只比金额不比币种**（实收 9900 恰等于面额）→ fact 用 attempt 的 CNY 构造
→ 错币种收款经恢复路径被洗白成正常确认（订单 paid + fulfill 事件铸造）。
修复：`AttemptResult` 增 `AmountCurrency`，微信 Query 回传，恢复/关单两处
`collectedAmountMismatch` 双维比对（金额 OR 币种任一不符即分流 anomaly）；
回归锁 `TestRecoverOrderStatusWrongCurrencyRetainedAsAnomaly`。重置栈后
第一/四幕复跑通过（恢复读不再洗白）。

**异常单占槽披露（真栈链路事实，Ruling 记 Ledger）**：部分付款单进入
attention 后订单保持 payable pending（防双重收款的保守面），期间新购买
（含换渠道）被 R2-26 pending 不变量回放旧单。解锁路径 = 渠道侧关单（本轮
以 stub mark CLOSED 演示：CloseChannelOrder 干净落关 → 槽释放 → 新支付宝
单创建成功，见 `leg2-purchase.json` 前置步骤）。

## 第二幕：正常付款 + 重复通知幂等 + Lago 恰一次（AC3）

（租户 23——全新 Lago customer，规避共享栈历史 gating 残留导致的 settle
distinct-invoice 消歧闸 fail-closed；证据 `leg2c-*.txt/json/png`）

1. 支付宝正常成功通知（99.00）→ `success` 终态；订单 paid、fulfill 事件恰 1。
2. drain → settle（Stripe attach pm_card_visa + confirm off-session）→
   deliver_stripe_webhook 投真实 payment_intent.succeeded（HMAC 用 provider
   stored secret，rails runner 模型层读取）→ Lago finalize：sub status=1
   （active）、invoice payment_status=1（succeeded）（DB 只读参数化查询）。
3. WeKnora 侧：订单 fulfilled、purchase `active`（`leg2c-active.json`）。
4. 重复投递同一成功通知 ×3 → 每次 `success`；fulfillment_records
   purchase_activation 仍恰 1 applied；**Lago payments 表 14|3 基线不变**
   （`leg2c-idempotent.txt`、`leg2c-lago-payments-baseline.txt`）。
5. UI 两帧：processing（租户 21 的真实 paid 中间态）`leg2-01-processing.png`、
   active `leg2-02-active.png`。

**披露①（settle 结算腿的人工补步）**：租户 23 的 customer 在共享栈上命中
历史同名 customer 残留（多个 requires_* gating intent），settle 的
distinct-invoice 消歧闸 fail-closed（#82 防双扣设计正确触发）。为完成链路，
结算腿两步（attach + confirm）以等价 Stripe API 调用补齐（与 settle step
iii/iv 相同的参数面），后续轮 settle 幂等重放（F-1：invoice 已 succeeded 的
sibling 短路）。该补步已披露，非伪造——intent/发票/webhook 链全程真实。
**披露②（Payment 行形态）**：该 Lago 版本 webhook finalize 置 invoice
payment_status=succeeded 但不铸造 payments 表新行——AC3「不重复写 Lago
Payment」的权威验证面为：invoice payment_status 恰一次 succeeded + payments
表行数/状态基线在重投与多收款全程不变（第三幕复核仍 14|3）。

## 第三幕：多成功只履约一次 + 多收款进入处置（AC2）

（租户 23 同一支付宝单；证据 `leg3-*.txt`）

1. 主路径（L1 轻替代）：对已 fulfilled 单投**换交易号**的第二笔成功通知
   （同 out_trade_no，trade_no=txn84-leg3-b）→ `success`、订单仍 fulfilled、
   `over_payment` 事件落（event_key 携新交易号）。
2. drain ≤60s（实测 ~14s）消费 → anomaly `over_payment`（expected 0/actual
   9900）awaiting_disposition、事件 sent。
3. 权益不扩大：purchase_activation 仍恰 1 applied；**Lago payments 基线
   14|3 不变**（settle 从不被第二笔驱动——短路的结构性真栈对照）。
4. R4 裁决锁定：订单主状态仍 `fulfilled`（`state:"active"`），异常走
   `payment_attention:true` 附加字段。
5. 处置面（B 的 refund_review 平台守卫）：admin list 3 行闭合字段 →
   resolve（expected_version=1）→ 200 resolved；重放同 version → **409**；
   全部处置后 pending 异常单 fulfillment 回落非 attention（`leg3-resolved.txt`）；
   C 的 fulfilled 单 resolve 后主状态不变、attention 消失。

## 第四幕：恢复路径实收额比对（G2 真实链）

（租户 24 微信单 `ord_0d3fc75f0116519d`；证据 `leg4-recovery-mismatch.txt`）

1. 新微信单 → 不投回调（漏通知场景）；stub 编排实收 **19900**（>面额 9900）
   + SUCCESS。
2. GET 订单触发 RecoverOrderStatus → `fulfillment:"attention"`、订单
   pending、attempt pending（不确认）；anomaly `amount_mismatch`
   9900/19900（actual > expected 的闭合分类）。
3. Lago：D 的 sub status=4（gated incomplete 零激活）、payments 基线 14|3
   不变。

## 通过判据核对

- Step 0 冒烟双形态：SELFTEST OK（无覆盖与 83 一致；覆盖落回调解密 amount）✅
- 四幕断言全部命中，证据 txt/json/png 齐备于本目录 ✅
- `grep -rln sk_test / whsec_ 本目录` → 零命中；登录落盘 REDACTED ✅
- 权威侧计数（invoice payment_status 恰一次、payments 表基线不变、sub
  active 恰一次、activation 记录恰 1 applied）全部来自真实 Lago DB/API
  只读查询，非 mock ✅
- 披露边界：微信/支付宝为本地协议 stub（已裁决边界，非沙箱证据）；Alipay
  无币种字段（错币种仅微信腿）；settle 结算腿一次人工补步（披露①）；
  Payment 行形态与计划文字的偏差（披露②）；Task 6 tagged 集成测试仍
  blocked-env（见 Ledger）。

## 收栈

```bash
pkill -f 'wechat_native_stub_anomaly.py'; pkill -f 'alipay_gateway_stub.py'
kill <backend go run pid>; # 前端 dev 同理
```
（Lago 82r5 栈为宿主共享资产，不动。）
