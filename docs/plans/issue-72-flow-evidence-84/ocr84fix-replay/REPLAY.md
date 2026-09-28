# OCR84-R1 修复批次真栈重放记录（2026-09-28）

修复改变了已验证用户流程的行为，按批次要求重放 docs/plans/issue-72-flow-evidence-84/
的可重放脚本（run_84.sh 步骤 0-6 + 第二/三幕核心链路）。本目录留档，不覆盖既有
r1/r2 证据。

## 环境

- Lago 82r5 栈（宿主共享，:48889 healthy）；密钥复用 `${TMPDIR}issue84-keys`、
  0600 `${TMPDIR}issue84-secrets.env`、`~/.zcode/issue72-stripe.env`（均仓库外）。
- 后端 ：8096 = 本批次修复后的 worktree 代码（`up_backend_84.sh`，独立新库
  `data/issue84-ocr84fix-replay.db`）；stub ：8298/:8299（`up_stubs_84.sh`）。
- 前端 :5197 未起（本重放走 API 面，不涉浏览器断言）。

## 起栈 + 种子（验证 R1-17/R1-18/R1-34/R1-11）

| 步骤 | 结果 |
|---|---|
| stub selftest 负形态（缺 5 个 WECHAT_STUB_* env） | `missing required env: …` 退出码 **2**（R1-34 快速失败生效） |
| stub selftest 正形态（env 齐备） | `SELFTEST OK`（无覆盖一致 + 覆盖落两脸） |
| `up_stubs_84.sh` | 微信 stub 401 拒未签名 / 支付宝 stub 200 |
| `up_backend_84.sh`（修复后代码） | 启动成功——R1-11 姿态守卫：BaseURL=127.0.0.1:48889 环回 + StripeAPIBase 空 ⇒ 合法放行 |
| `seed_84.sh`（python3 ? 参数绑定 + EV 先行解析版，PAD=48） | 48 pad + a=49/b=50 注册 201；grants **2 行**（参数绑定写入+计数）；publish `publish_plan_version:pro:1`；Lago 断言 `weknora-pro-v1` 9900/monthly 命中；**SEED OK** |

## 第二幕式链路（验证 R1-01/R1-02/R1-05/R1-09-14 的主路径不被破坏）

订单 `ord_cbc3fa07f8a1304b`（租户 49，支付宝，面额 9900）：

1. quote + purchase → 订单落 pending + `https://qr.alipay.com/issue82flow…`。
2. 签名成功通知（trade_no=txn84-ocr84fix-a，99.00）→ 200 `success` → 订单
   **paid v2**、fulfill 事件恰 1。
3. drain 驱动 settle：本客户在共享 Stripe TEST 账号上存在历史残留 intent
   （挂在不存在的 Lago 发票 381b5764 上，requires_payment_method）——D9
   distinct-invoice 消歧闸**正确 fail-closed**（invalid_response → attention），
   与 r2 轮披露①同源的共享栈环境残留，非本批代码回归。作废该死 intent
   （`pi_3UKApQ…/cancel`，其发票已不存在于权威侧）后重驱动：
   - 真实 Stripe 腿全链：attach pm_card_visa → update → confirm →
     `pi_3UKRnt…0MyL` **succeeded**（1320 prorated，发票 0b688583）。
   - 真实 F-1 形态对照：settledInvoices 含历史发票 381b5764，候选发票
     0b688583 与之不同 → 不短路、驱动真实 gate（R1-01 收紧后行为正确）。
4. `deliver_stripe_webhook.py` 补投 payment_intent.succeeded（secret 取 rails
   runner stdout **末行**，README 既有披露）→ HTTP 200 → Lago
   sub status=**1**（active）、invoice payment_status=**1**。
5. 下一轮 drain 自动收尾：**attention 被 APPLIED 覆盖**（purchase_activation
   applied 带回执）、订单 **fulfilled**、fulfill 事件 **sent**——即 R1-01 收紧
   的 settle-vs-finalize 窗口回放路径（事件保持 pending，权威激活后终态覆盖）
   在真栈成立。

## 第三幕式链路（验证 R1-13）

同单换交易号第二笔成功通知（txn84-ocr84fix-b）→ 200 `success` →
over_payment 事件 drain 后 **sent**；anomaly 行：

```
attempt_id=mo_217172aafc638e01  kind=over_payment  expected=0  actual=9900  state=awaiting_disposition
```

attempt_id 落 **mo_ 渠道身份**（非内部 att_ 主键）——与 buildMismatchAnomaly/
recoverMismatchedCollection 同口径，R1-13 真栈命中。

## 结论

- 修改后的 seed/stub/后端在真实栈起得来、种子全绿。
- 第二/三幕核心链路（付款→settle→webhook→active→applied/fulfilled；
  多收款→over_payment→处置面）全部命中既有断言面。
- 本轮披露：①共享 Stripe TEST 账号的历史残留 intent 一次人工作废（D9 闸
  正确触发后清理死对象，与 r2 披露①同类）；②webhook 补投为既有裁决边界
  （Stripe 云→本地 Lago 不可达腿）；③前端浏览器帧未重放（API 面已覆盖本批
  行为变更点；前端改动由 apps/web 36 项 commercial node:test 全绿覆盖）。
