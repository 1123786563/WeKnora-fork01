# Issue #83 OCR 修复批次（ocr-83-1）活栈重放证据

- 验证者：修复员（dynamic workflow，本批次）
- 分支：`codex/issue-72-lago`（worktree `.worktrees-issue72/lago-int`，含 C-01~C-12 修复）
- 触发理由：本批次 Go 生产代码修复（C-07/08/09/10：原子关单对、alipay Query 事务同源、切换分支排除式判定、attempt 读错误分流）改变了已验证用户流程的行为，按批次要求重放真实流程验证脚本。

## 环境拓扑（全部真实运行）

| 组件 | 地址 | 说明 |
|---|---|---|
| Lago v1.53.0 | 127.0.0.1:48889 | 复用 `weknora-lago-82flow` 栈；本轮重放前经 Lago API 软删除旧 `weknora-pro-v1`（seed 形状断言可验证本轮重建），Lago identity 已被历史轮占到 43 → 本轮 `FLOW83_PAD_COUNT=45`（主角租户 46/47） |
| WeKnora 后端 | 127.0.0.1:8095 | 修复后代码 `go build ./cmd/server`；新库 `data/issue83-ocrfix.db`；`WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true` + loopback 权威 |
| 微信 Native stub | 127.0.0.1:8296 | `../wechat_native_stub.py`（v2_up_stubs.sh）；密钥 `/tmp/issue83-ocrfix-keys`（v2_gen_keys.sh **修复版**生成——C-06 边界断言 `first=0x6b last=0x65` 非空白通过） |
| 支付宝 stub | 127.0.0.1:8297 | `../issue-72-flow-evidence-82/alipay_gateway_stub.py` |
| vite dev | localhost:5196 | `FRONTEND_BACKEND_URL=http://127.0.0.1:8095`（act1 浏览器面） |
| Stripe 真实 API | api.stripe.com | test key（source-only 注入） |

## 重放步骤与结果

1. **v2_gen_keys.sh（C-06 修复版）**：生成通过——`apiv3.key = 32 bytes OK` + `boundary bytes non-whitespace OK (first=0x6b last=0x65)`（构造性首尾折叠生效）。
2. **v2_up_stubs.sh**：微信 stub 未签名 native 401、支付宝 stub precreate-unauth 200（冒烟通过）。
3. **seed_83.sh（C-01/11/12 修复版）**：45×占位 + 主角注册 HTTP 201 断言、plan 发布、**C-11 形状断言**（`plan weknora-pro-v1 present at the exact 9900/monthly face (round-specific shape assertion)`）、**C-01 落盘脱敏**（seed-login-a/f.json 的 token/refresh_token 均为 `REDACTED`，`git status` 无活体令牌回归——.gitignore 规则同步挡住未来新文件）。主角租户 A=46 B=47。
4. **browser_flow_83.mjs（act1，C-05/C-02/C-03 修复版）**：前 8 项 PASS——login、channel-radio（alipay 默认+wechat 单选）、切换入口（`改用微信支付重新发起支付`）→ **wechat-awaiting-payment（weixin:// 链接）**、订单号 `ord_b6ed625a2f5feb01`、**stub-native-order（NOTPAY/9900 过滤后取键 C-03）**、签名 notify → WeKnora 200、**paid_awaiting_activation**、billing `已付款待激活`。`settle-webhook-active` 两项 FAIL 归因于**运行环境 env 注入遗漏**（deliver_stripe_webhook.py 需要 `STRIPE_SECRET_KEY`/`LAGO_INTEGRATION_WEBHOOK_SECRET` export，本次启动 node 时未 source——上轮 README 的 source 纪律），非代码回归；链路本身以手动补投完成：Stripe gating intent（**1320 分 proration**）经后端 drain settle → **succeeded**，webhook 补投 → `{"state":"active","order":"fulfilled"}`。期间两次中途失败（guide 弹窗挡点击、租户 identity 撞历史 active 订阅）均被 **C-05 的 uncaught-flow-error 捕获并完整输出 RESULT**——失败面不再丢失（C-05 的直接收益）。
5. **api_recovery_83.mjs（act2-5，C-02/03/04 修复版）**：**26/26 PASS，exit 0**（完整输出 `act2345-ocrfix-run3.log`）——
   - act2：missed-notify → GET /orders/:id 查询恢复 → paid → active；
   - act3：双投递 200、fulfilled/outbox-fulfill/succeeded-attempt 恰 1、Lago succeeded payment 恰 1；
   - act4a（**C-09 paid 形态活栈验证**）：close 撞在途支付 → ORDER_PAID → 查单决胜 → **答旧的 paid 订单、alipay 零创建**、fulfill 恰 1、race-active；
   - act4b（**C-07 原子对活栈验证**）：干净切换 → CLOSE 204 → **old order channel_failed=1 attempt closed（单事务对落地）**、one-payable-entry（新单唯一）、迟到的 closed 单成功通知幂等（无第二次 fulfillment、无 over-payment）；
   - act5：四对象（订阅 active、gating invoice finalized+succeeded、fee 恰 1、payment 恰 1）+ 产品面（credits 10900000、advanced_models=true）。
6. 重放中白名单首版（UUID 形态）被真实数据纠正：Lago 侧 provider 绑定是 Stripe `cus_` id——**C-02 的 sqlShape 拦截到自身缺陷并终止**（fail-fast 生效的直接证据），修正正则后全绿。

## 本目录文件

- `act2345-ocrfix-run3.log`：act2-5 完整输出（26 PASS + RESULT JSON）。
- `seed-run.txt` / `api-02-publish.json` / `purchase-state-progression.txt` / `recovery-no-notify.txt` / `duplicate-notify-idempotent.txt` / `close-race-paid.txt` / `close-race-switch.txt` / `lago-four-objects-wechat.txt` / `account-after-wechat.json`：本轮重放副本（原始输出落在 #83 目录的历史文件已恢复原样）。

## 披露

- act1 的 settle→active 段以手动补投 webhook 完成（D8 替身腿，同上轮纪律）；FAIL 两项归因 env 注入遗漏而非代码回归，已如实记录。
- C-08（alipay.Query tradeNo 优先）的恢复路径与 C-09 的 fulfilled 形态、C-10 的错误分流未在活栈直接触发（正常流程不经过），由单测锁定（TestAlipayQueryReconcilesMissedNotification / TestAlipayQueryFallsBackWhenTradeNoAbsent / TestPurchaseSwitchAnswersFulfilledOrderNotNewChannel / TestPurchaseSwitchAttemptReadErrorPropagates）。
