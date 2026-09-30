# r5b-flowcheck — Issue #82 流程验证员独立复验（2026-09-28）

独立于 r5-verify（Task 17 执行轮）的流程验证员复验轮：以集成分支
`codex/issue-72-lago` HEAD=`7d614752c`（#82 merge 提交）为基线，从本
worktree 重新构建并真实运行全链，验证「支付宝付款后恰好一次激活套餐」
的用户流程。全部判据与计划「真实流程验证方案」一致。

## 环境（全部真实服务，独立数据空间）

| 组件 | 地址 | 说明 |
|---|---|---|
| Lago v1.53.0 | 127.0.0.1:48889 | 复用运行中的 `weknora-lago-82r5` 栈（docker compose，5 镜像 healthy；Stripe provider `weknora-stripe` 已注册，真实 TEST key 仅 source 注入） |
| WeKnora 后端 | 127.0.0.1:8093 | 本 worktree `go run ./cmd/server`（`start_backend_8093.sh`）；sqlite 独立库 `data/issue82-r5b-verify.db`；config.yaml 端口临时置 8093（验证后恢复） |
| 前端 vite | localhost:5194 | `VITE_DEV_PROXY_TARGET=http://127.0.0.1:8093` |
| 支付宝网关 stub | 127.0.0.1:8294 | `alipay_gateway_stub.py` + `/tmp/issue82-r5-keys` 本地 RSA 对（协议往返+验签证明；`ac4-sandbox-credentials-unavailable` 披露不变——非真实沙箱钱包证据） |
| webhook 投递 | — | `settle-evidence/deliver_stripe_webhook.py`（真实 PI 体 + provider 存储 secret 的真实 HMAC，接收/验签/处理 100% Lago 内建） |

种子（`seed.sh`，r5 修复版模板）：PAD=6 占位 + 主角 tenant 7
（`settle-r5b-a@verify.local` 浏览器）/ tenant 8（发布者）——**PAD 提升的
原因**：Lago 栈上 `weknora-tenant-3-purchase` 已被 r5 轮占用（active），
新库 tenant 3 撞 external_id 会命中 `purchase_not_awaiting` 409（reverify1
README 记录过的同款坑）。plan `pro` v1（9900 分 CNY）发布，Lago 侧
`weknora-pro-v1` 已存在且内容一致 → 幂等重放形状（`api-04-lago-plan-readback.json`
断言 priced identity 相等——seed 对「到达」的两种可接受面之一，另一面
fresh-stack count+1 保留）。

## 执行结果（订单 `ord_86c00340e158d726`，¥99.00 支付宝）

| 步骤 | 断言 | 结果 |
|---|---|---|
| seed | 租户 7/8、grant、publish receipt、Lago plan | `seed-run.txt` PASS |
| browser_01 下单 | 支付宝默认选中；wire 体 provider=alipay；等待付款+待付款（权益未开通）+报价明细 ¥99.00+订单号；billing 待付款（权益未开放） | **7/7 PASS**（rv5-01/02） |
| Lago gated 形状 | subscription incomplete + activation_rules=1（type=payment, pending） | `lago-gated-before-settle.txt` PASS |
| browser_02 同步面（AC2） | `?order=` 回访+显式刷新零推进；无已付款/已生效；同一订单身份 | **5/5 PASS**（rv5-03） |
| 签名回调 | TRADE_SUCCESS 99.00 → 200 `success`；purchase `paid_awaiting_activation` | `callback-01-first.txt` / `purchase-after-notify.json` PASS |
| D11 竞态面 | paid 窗口新 quote 提交 → 同一 paid 订单视图（HTTP 201 重放）；stub PRECREATE 计数 1→1 | `d11-race-repurchase.json` PASS |
| browser_03 已付款待激活（AC1 中间态） | 已付款，权益处理中；billing 已付款待激活；无已生效；同一订单 | **5/5 PASS**（rv5-04） |
| settle 驱动（α 轨道二） | Stripe gating PI `pi_3UKMmO…` **succeeded**（pm 4242 结算卡，1320=proration）；残留兄弟 `pi_3UKMmG…` requires_payment_method 不被驱动；customer default pm 仍为 3DS 卡（3220，D10 无 default 改写） | `settle-run-r5b.txt` PASS |
| webhook finalize | 投递 HTTP 200 → subscription **active**（5s 内）；invoice **finalized+numbered（WEK-48D1-004-001）+payment_status=succeeded**；payments 恰 1 succeeded（另 1 failed 为 3DS 自动收款历史入账）；wallets 购买批次 `weknora-tenant-7-purchase-2026-09`（990） | `lago-four-objects-r5b.txt` PASS |
| Credits/Entitlement | `/commercial/account` credits balance_micro=10900000（base 1000000+购买 9900000）；features advanced_models/api_access=true | `account-after-active.json` PASS |
| browser_04 已生效（AC1 终态） | billing 套餐base·已生效（detached 确定性等待）；checkout 深链权益已生效；同一订单 | **4/4 PASS**（rv5-05） |
| 重放面 1 重复回调 | 同 notify 重发 → 200 success；fulfilled_orders/fulfill_events/activations/attempts 全 1→1 | `replay-01-notify.txt` PASS |
| 重放面 2 重复 webhook | 新 event id 同 PI 体 → HTTP 200；四对象投影不变（active/finalized+succeeded/1 succeeded/wallets 不变）；本地 activation 仍 1 | `replay-02-webhook.txt` PASS |
| 重放面 3 后端重启 | kill+重启，drain 多轮重跑（attempt 6→8）→ 订单仍 fulfilled、activations=1、fulfill_events=1（outbox 终态 sent） | `replay-03-restart.txt` PASS |
| 红线 | sk_test/sk_live 字面量 0；完整口令值 0 命中（grep 标题文字中的 token 名非口令值）；force-active 直写 0；manual payments 调用 0 | `redline-checks-r5b.txt` PASS |

## 与 r5 轮记录的两处环境形状差异（如实披露，非产品缺陷）

1. **Lago plan 到达形状**：本栈已含 `weknora-pro-v1`，publish 走
   createPlan 的 422→verifyPlanReplay 幂等路径（内容相等 → 到达）；
   seed 断言适配为「count+1 或 read-back 内容一致」。
2. **重复 webhook 应答**：本轮 HTTP 200（r5 记录 400）。两种应答下
   四对象投影均 byte-identical——判定以投影不变为准。

## 运行方式（复现）

```bash
# Lago 栈（复用运行中的 weknora-lago-82r5；如需重建见 deploy/lago/lago.sh）
# 后端（凭据 env 注入，见 start_backend_8093.sh 头注）：
set -a; source ~/.zcode/issue72-stripe.env; set +a
export WEKNORA_COMMERCIAL_STRIPE_API_KEY="$STRIPE_SECRET_KEY" \
  WEKNORA_COMMERCIAL_PLATFORM_API_KEY="$(docker exec weknora-lago-82r5-db-1 \
    psql -U lago -tAc 'select value from api_keys order by created_at desc limit 1')" \
  FLOW82_KEY_DIR=/tmp/issue82-r5-keys
nohup bash docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/start_backend_8093.sh &
# stub / 前端：
FLOW82_ALIPAY_PORT=8294 FLOW82_KEY_DIR=/tmp/issue82-r5-keys nohup python3 docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py &
VITE_DEV_PROXY_TARGET=http://127.0.0.1:8093 nohup pnpm --filter @weknora/web exec vite --port 5194 --strictPort &
# 种子 + 四腿（截图/断言落本目录）：
FLOW82_R5_PW=… FLOW82_EMAIL_PREFIX=settle-r5b FLOW82_PAD_COUNT=6 \
  DB_PATH=data/issue82-r5b-verify.db bash docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/seed.sh
FLOW82_EMAIL_A=settle-r5b-a@verify.local FLOW82_R5_PW=… FLOW82_EXPECT_CNY='¥99.00' \
  FLOW82_WEB=http://localhost:5194 node docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_01_checkout.mjs
# notify → browser_02 → browser_03 → settle 观察 → webhook → browser_04（见上表顺序）
```

## 结论

Issue #82 用户流程（浏览器下单 → 支付宝扫码 → 同步返回页零推进 →
可信回调 → 已付款待激活 → settle/webhook → 已生效 + Credits/Entitlement
到账 → 重复回调/重放恰一次入账）在真实环境全链 PASS；后台 Lago 可查
关联 Invoice/Payment/active Subscription/购买批次 wallet。支付宝渠道为
本地 RSA stub（披露不变），沙箱凭据不可用残余 `ac4-sandbox-credentials-unavailable`。
