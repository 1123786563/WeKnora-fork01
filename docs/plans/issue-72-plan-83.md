预写基线（待实现阶段复核定稿）

# [Lago 11] 微信付款复用同一激活流程 复核与回归计划（Issue #83，第二轮预写）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **本文件取代 `707d730aa`（issue-72(#83): plan）落盘的第一版预写计划。** 该版已执行完毕（Task 1-4 全交付，执行记录见 `docs/plans/issue-72-ledger-83.md`），历史文本在 git（`git show 707d730aa:docs/plans/issue-72-plan-83.md`）。本版按预写员于当前集成分支 HEAD `7d614752c`（2026-09-28 实读）重新核定基线后编写，形态从「初次实施」转为「交付物在位核对 + #82 后续合入后的微信腿回归」。

**Goal:** 确认 Issue #83（微信付款复用同一激活流程）的全部交付物在当前集成分支 HEAD 上在位且行为未回归——特别是 #83 验证轮之后合入的 #82 task12-17 改动了 25 个共享链路文件、其 r5 浏览器复验只走了支付宝腿，微信腿五幕（浏览器主链/漏通知恢复/重复通知幂等/关单竞态/四对象）需在 HEAD 上以真实受控环境复跑一遍；若回归发现破坏，按 TDD 修补。

**Architecture:** #83 的实现形态（已集成，本节为核对基准）：微信渠道 Provider（`WechatProvider` Create/Query/Close/Refund/Verify）走与支付宝完全相同的渠道无关激活链——回调 `POST /api/v1/commercial/callbacks/:provider`（`internal/router/routes_commercial.go:123` 挂载，匿名可达）→ `PaymentCallbacksHandler.HandleProviderCallback`（`internal/handler/payment_callbacks.go:61`，1 MiB body cap）→ `WechatProvider.Verify`（RSA-SHA256 验签 + AES-GCM 解密 + mchid/appid 匹配，`internal/modules/commercial/payment/wechat.go:249`）→ `resolveByMerchantOrderID`（按 `(provider, merchant, merchant_order_id)` 参数绑定查本地 attempt，绝不信任 payload 身份）→ `OrderStore.ConfirmPayment`（单事务金额/币种校验 + sameTxn 幂等 + outbox fulfill 事件）→ `PurchaseFulfiller.Fulfill`（`internal/modules/commercial/service/commercial/purchase_fulfillment.go:86`）→ `CommandKindSettlePurchasePayment`（`lago_settlement.go:54`，WeKnora 驱动 Stripe gated 结算扣款，幂等键绑定渠道 attempt）→ Lago 内建 webhook finalize → Subscription active，恰好一次。渠道侧生产行为只有一个：竞态安全关单编排 `OrderService.CloseChannelOrder`（`internal/modules/commercial/service/commercial/order.go:567`，Close 失败一律 Query 决胜）+ `purchase.go` 重放分支换轨关单。**零 Commercial Platform seam 改动**（ADR-0014 加法纪律）。

**Tech Stack:** Go（gin + gorm；sqlite 测试库）、Playwright `@playwright/test`（仓库 devDependencies）、Python 3 微信 Native v3 协议 stub（`docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py`，已交付）、真实 Lago v1.53.0 compose（:48889/:48890）+ Stripe TEST 结算腿（R-4 双轨道）。

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`（L123/L124/L125/L127/L165/L218 矩阵 #5）；ADR-0012（`docs/adr/0012-lago-as-commercial-billing-authority.md`，含 2026-09-23 修订）；ADR-0014（seam 只许加法）；用户裁决 R-4（2026-09-26：渠道真实收款 + WeKnora 驱动 Stripe gated 结算 → Lago 内建 finalize → active；渠道沙箱凭据缺失属已披露边界，用本地 RSA stub 证明协议往返+验签，**不得伪造沙箱证据**）。

## 基线核实（预写员 2026-09-28 实读实跑，修正任务调查简报的滞后描述）

任务调查简报称「#83 OPEN、Payment Fact→Lago activation 链路在适配器与 seam 上完全缺失（lago.go:684-687 不带 activation_rules）、微信 Create/Query/Close 无单测、回调无 HTTP 测试、无微信受控环境栈」——**该描述对应旧基线，与当前集成分支实况不符**。实况（本会话 `git log --graph` 实查）：

- #83 已完整交付并合并：`707d730aa`(plan) → `9e11d7170`(task1 协议 pin) → `9131c32c7`(task2 回调 HTTP 腿) → `ebf64a905`(task3 关单编排+切轨) → `6ef004e3a`(fix: Query 键 transaction_id) → `1f51e425d`(task4 真栈证据) → `9ebd1c738`(merge #83) → `3ebc70347`/`85cd62761`/`da4b544db`(OCR r1/r1修复/r2)。
- **当前 HEAD `7d614752c` = merge #82**，且 `git merge-base --is-ancestor 9ebd1c738 HEAD` 为真（#83 全部在 HEAD 可达）。
- 调查简报所列缺口在 HEAD 的实况：
  - 「无 activation_rules」：`internal/modules/commercial/commercialplatform/lago_purchase.go:226` `createPurchaseSubscription` 已携带 `"activation_rules": [{"type":"payment","timeout_hours":0}]`（F1/F8）。免费 Base Plan 的 `ensureSubscription`（`lago.go:621` 起）不带 activation_rules 是 T02 决策，非缺口。
  - 「微信 Create/Query/Close 无单测」：`wechat_native_test.go` 已有 10 个测试（`TestWechatCreatePostsNativeOrderAndMapsCodeURL`、`TestWechatQueryMapsTradeStates`、`TestWechatQueryPrefersTransactionID`、`TestWechatClosePropagatesOrderPaidCode` 等）。
  - 「回调无 HTTP 层测试」：`payment_callbacks_test.go` 已有微信腿（`:417` 直接 POST `/api/v1/commercial/callbacks/wechat`，真实 `WechatProvider.Verify` 全链到 outbox）。
  - 「无微信受控环境栈」：`docs/plans/issue-72-flow-evidence-83/` 已有 v1+v2 两轮五幕验收（微信协议 stub + 真实 Lago + 真实 Stripe TEST + Playwright），全过。
- **真正的残留缺口（本计划的对象）**：v2 复验轮时点是 `9ebd1c738`；其后 #82 的 task12-17（`2cbacd763..00ff79aec`）+ merge 改动 64 个文件，其中 25 个落在 #83 链路直接依赖的文件上（`payment_callbacks.go`、`purchase.go`、`purchase_fulfillment.go`、`lago_settlement.go`、`lago_purchase.go`、`repository/commercial/order.go`、`internal/handler/commercial.go`、`apps/web/src/commercial/{CheckoutPage,BillingPage,order-state}.tsx|ts` 等）。#82 自己的 r5 浏览器复验走的是**支付宝腿**（`issue-72-flow-evidence-82/r5-verify/`）——Issue #83 AC4 明确「不以支付宝证据替代」，微信腿在 HEAD 上缺真栈回归记录。
- 本会话在 HEAD 上实跑（基线绿）：
  - `go test ./internal/modules/commercial/payment/ -count=1` → `ok ... 20.128s`
  - `go test ./internal/modules/commercial/service/commercial/ -count=1` → `ok ... 2.603s`
  - `go test ./internal/handler/ -run 'TestWechatCallback|TestAlipayCallback|TestPaymentCallback' -count=1` → `ok ... 4.431s`

## Global Constraints

- spec L123：WeKnora 拥有微信/支付宝的下单、回调验签、查单、关单、退款行为；已验证渠道结果产生不可变 Payment Fact。
- spec L124：受支持的 Lago 外部支付集成记录 Payment；只有观察为 active 的 Lago Subscription 才开通 Entitlement 与履约。
- spec L125：不许本地强制 Subscription active（红线 grep）。
- spec L127：重复/错额/错币/多成功付款保留外部事实，不自动改 Invoice 金额或已交付权益。
- spec L165：命令走 durable Outbox + 稳定幂等身份；超时与 5xx 是不定结局，必须查单后重放。
- spec L218（矩阵 #5）：已验证微信/支付宝结果经受支持集成到达 Lago 并恰好一次地把 incomplete 订阅推到 active。
- spec「Public product states」：Billing API 不暴露 Lago 凭据、内部 URL、Organization 标识或原始状态枚举。
- 用户裁决 R-4（2026-09-26）：激活链一律「渠道真实收款 + WeKnora 驱动 Stripe Provider gated 结算 → Lago 内建 finalize」；微信真实沙箱凭据缺失为已披露边界，用本地 RSA stub 证明协议往返+验签，不得伪造沙箱证据。
- ADR-0014：Commercial Platform seam 只许加法；本计划预期**零 seam 改动**。
- 安全约束（实现验收条件）：渠道/平台出站仅 http/https 且过 SSRF 校验（环回 stub 需显式 `SSRF_WHITELIST_EXTRA=127.0.0.1` 豁免，`wechat.go` `do` 已实现）；SQL 一律参数绑定（`resolveByMerchantOrderID` 与 `ConfirmPayment` 既有惯例）；凭据只从环境变量/密钥文件注入——Stripe key 仅 `source ~/.zcode/issue72-stripe.env`，本地一次性 RSA/apiv3 密钥生成在运行时临时目录（`${TMPDIR}issue83-keys*`），源码、测试、证据目录不得出现任何可用凭据字面量（含 `sk_test_` 前缀）。
- 端口纪律：验证环境避开 `:5272/:5273`（其他会话）及既往占用口径 `:8080/:8091/:8092/:8093/:5183/:5192/:5194/:8291/:8292/:8294`；沿用 v2 口径：后端 `:8095`、前端 `:5196`、微信 stub `:8296`、支付宝 stub `:8297`（复验前以 `lsof -iTCP:<port> -sTCP:LISTEN` 确认空闲，被占则整体 +10 顺延并同步脚本 env）。

## Review Focus（复核视角：HEAD 上最可能被 #82 后续合入咬到的五类行为；每行已归属 owning task）

1. **切轨关单路径被 purchase 竞态加固改坏**（#82 task13「paid-awaiting no-reopen / pending-exists 409」重写了 `purchase.go` 重放分支，而 #83 的换轨关单正挂在该分支）：微信待付单切支付宝时仍必须先 `CloseChannelOrder` 旧单 → Task 2 `TestPurchaseSwitchesChannelByClosingOldOrder`/`TestCloseChannelOrderPaidRaceConfirmsFundFact` + Task 3 幕四。
2. **settle 链正确性改动影响微信腿**（#82 task12 distinct-invoice fail-closed、429/5xx 分类改了 `lago_settlement.go`/`purchase_fulfillment.go`）：微信回调驱动的 settle 仍必须恰好一次到 active → Task 3 幕一/幕五（`purchase-state-progression` 三态 + Lago payments 恰 1）。
3. **回调 body cap 与微信通知兼容**（#82 task14 的 1 MiB `MaxBytesReader`）：正常微信通知（KB 级）不受影响、超限体非 5xx 拒收 → Task 2 既有 `TestWechatCallback*` 族回归 + Task 3 幕一签名回调 200 回执。
4. **Checkout 前端 polish 改坏微信呈现**（#82 task15 channel ref/canceled state 改了 `CheckoutPage.tsx`）：`weixin://` code_url 仍呈现「前往支付」入口（`isSafeCheckoutUrl` 白名单须仍含 weixin scheme，`CheckoutPage.tsx:367`）→ Task 2 `pnpm test:web`（`CheckoutPage.test.tsx`/`order-state.test.ts`）+ Task 3 幕一浏览器断言。
5. **`order.go` repository 索引/谓词改动影响 ConfirmPayment 幂等**（#82 task13 PG index、fallback predicate）：微信重复通知仍必须 exactly-once（无假 over-payment、无 500）→ Task 2 `TestWechatCallbackDuplicateDeliveryIdempotent` + Task 3 幕三。

---

## File Structure（本计划为复核+条件修补型；预期零新建产品代码）

- 核对对象（不修改，Task 1/2/3 只读或测试运行）：`internal/modules/commercial/payment/wechat.go` 与三个测试文件、`internal/handler/payment_callbacks.go`(+test)、`internal/modules/commercial/service/commercial/{order,purchase}.go`(+tests)、`internal/modules/commercial/repository/commercial/order.go`(+test)、`internal/modules/commercial/commercialplatform/{lago_purchase,lago_settlement}.go`(+tests)、`internal/router/routes_commercial.go`、`apps/web/src/commercial/{CheckoutPage,BillingPage}.tsx`、`docs/plans/issue-72-flow-evidence-83/*`、`docs/plans/issue-72-ledger-83.md`。
- 条件修改（仅当 Task 2/3 发现回归，Task 4）：对应破损文件 + 其特征测试文件；`docs/plans/issue-72-ledger-83.md` 续写复验记录；`docs/plans/issue-72-flow-evidence-83/` 增补本轮证据（`r6-head-verify/` 子目录，不覆盖 v2 证据）。

---

### Task 1: 交付物在位核对（HEAD 上 #83 全链构件逐一确认）

**Files:** 只读核对，无修改。

**Interfaces:**
- Consumes: 本计划「基线核实」节的提交清单与文件路径。
- Produces: 核对结论（写入 ledger 续写）：后续 Task 的执行前提。

- [ ] **Step 1: 确认 #83 提交链在 HEAD 可达**

Run: `git merge-base --is-ancestor 9ebd1c738 HEAD && echo IN && git log --oneline -1`
Expected: `IN` 且 HEAD 为 `7d614752c` 或其后续（若 HEAD 已前移，以 `git log --oneline --grep="#83" -15` 确认 task1-4/fix/OCR 提交仍在）。

- [ ] **Step 2: 确认关键构件文件在位且非空壳**

Run: `grep -c "func Test" internal/modules/commercial/payment/wechat_native_test.go && grep -c "func Test" internal/handler/payment_callbacks_test.go && grep -n "func (s \*OrderService) CloseChannelOrder" internal/modules/commercial/service/commercial/order.go && grep -n "activation_rules" internal/modules/commercial/commercialplatform/lago_purchase.go && ls docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py`
Expected: wechat_native_test 计数 ≥ 10；callbacks_test 含微信腿；`CloseChannelOrder` 命中 `order.go:567` 附近；`activation_rules` 命中 `lago_purchase.go:285` 附近；两个验收脚本存在。

- [ ] **Step 3: 确认回调路由与环境注册**

Run: `grep -n "callbacksGroup.POST" internal/router/routes_commercial.go && grep -n "WEKNORA_WECHAT_MCH_ID" internal/modules/commercial/payment/providers_env.go && grep -n "commercial/callbacks" internal/middleware/auth.go`
Expected: 三处均命中（挂载、`wechatConfigFromEnv` 必填集、匿名放行表）。

### Task 2: 单测与守卫回归（HEAD 基线复核）

**Files:** 只读运行；证据写入 ledger。

**Interfaces:**
- Consumes: Task 1 的在位结论。
- Produces: HEAD 上的回归绿基线（命令+输出截录），作为 Task 3 真栈复验的前置门。

- [ ] **Step 1: 微信渠道三包全量**

Run: `go test ./internal/modules/commercial/payment/ ./internal/modules/commercial/service/commercial/ ./internal/handler/ -count=1`
Expected: 三包 `ok`（exit 0）。任一 FAIL → 转 Task 4 修补流程，禁止带病进 Task 3。

- [ ] **Step 2: 定点回归 #83 特征测试（Review Focus 五行的单测腿）**

Run: `go test ./internal/handler/ -run 'TestWechatCallback' -count=1 -v && go test ./internal/modules/commercial/service/commercial/ -run 'TestCloseChannelOrder|TestPurchaseSwitch|TestLateSuccessAfterClose' -count=1 -v && go test ./internal/modules/commercial/payment/ -run 'TestWechat(Create|Query|Close)' -count=1 -v`
Expected: 全 PASS；逐条对照 Review Focus 1/3/5 的测试名在场且绿。

- [ ] **Step 3: 架构守卫与前端回归**

Run: `make check-backend-architecture && pnpm test:web && pnpm typecheck:web`
Expected: `architectureguard: OK (0 violations)`；web 测试与类型检查全过（Review Focus 4 的单测腿）。

- [ ] **Step 4: 红线 grep**

Run: `grep -rn "sk_test_" docs/plans/issue-72-flow-evidence-83/ internal/ | wc -l && grep -rniE "force.*active|直接.*(写|置).*active" internal/modules/commercial/commercialplatform/*.go | grep -v _test | wc -l`
Expected: 两计数均 `0`。

### Task 3: 真实受控环境微信五幕复验（HEAD 上，AC4 独立微信证据）

**Files:** 证据落 `docs/plans/issue-72-flow-evidence-83/r6-head-verify/`（新建目录）；复用既有脚本（见下）。

**Interfaces:**
- Consumes: `docs/plans/issue-72-flow-evidence-83/` 既有资产——`wechat_native_stub.py`（Native v3 协议 stub：验出站签名/AES-GCM 签名回调/可编排状态机/`/stub/mark`+`/stub/notify`）、`browser_flow_83.mjs`、`api_recovery_83.mjs`、`seed_83.sh`、`v2_gen_keys.sh`、`v2_up_stubs.sh`、`v2_up_backend.sh`；`../issue-72-flow-evidence-82/alipay_gateway_stub.py`（切轨腿对端）与 `settle-evidence/deliver_stripe_webhook.py`（D8 webhook 投递替身）。
- Produces: `r6-head-verify/` 证据包（截图 5 + txt 留档 + README 增补轮次记录）；ledger 续写。

- [ ] **Step 1: 栈起（端口纪律见 Global Constraints；先 `lsof` 探空闲）**

一次性密钥经 `v2_gen_keys.sh` 生成于运行时临时目录（不进仓库）；微信 stub `:8296`、支付宝 stub `:8297`、后端 `:8095`（`go run ./cmd/server`，sqlite 独立新库 `data/issue83-flow-r6.db`，`source ~/.zcode/issue72-stripe.env` 注入结算腿，`WEKNORA_WECHAT_*`/`WEKNORA_ALIPAY_*`/平台 env 全套，`SSRF_WHITELIST_EXTRA=127.0.0.1`）；前端 `:5196`（`VITE_DEV_PROXY_TARGET=http://127.0.0.1:8095`）。Lago 复用 `deploy/lago` pinned v1.53.0 栈（:48889/:48890，docker ps 确认 healthy；若 82flow 栈的 Lago 租户号又被外部轮占据，按 R-83a 占位注册吸收）。
Expected: 各组件冒烟通过（stub `/v3/pay/transactions/native` 签名验证 401/200 行为正常、后端健康检查 200）。

- [ ] **Step 2: 幕一（AC1 主链，浏览器）+ 幕二/三（AC2）+ 幕四（AC3）+ 幕五（AC4 四对象）**

Run: `node browser_flow_83.mjs && node api_recovery_83.mjs`（env 按 v2 README：`FLOW83_DB`/`FLOW83_TENANT` 参数化；每轮唯一 email 规避 R-83a）。
Expected（与 v2 同判据，全部必须在本轮 HEAD 复现）：
- 幕一 12/12：渠道单选默认支付宝、显式切微信（页面换轨入口触发关旧开新）→「待付款（权益未开通）」+ `weixin://wxpay/` 前往支付链接 → stub mark SUCCESS + 签名 notify → 200 `{"code":"SUCCESS"}` → paid_awaiting_activation 双页面 → settle（真 Stripe PI succeeded）→ webhook finalize → active 双页面（截图 01-05）。
- 幕二 4/4：mark SUCCESS 不推 notify → `GET /orders/:id` 查单恢复（`RecoverOrderStatus`，`internal/handler/commercial.go:866` 接线）→ paid → active（`recovery-no-notify.txt`）。
- 幕三 4/4：同一签名通知重投两次均 200；fulfilled/outbox fulfill/succeeded attempt/Lago payments 各恰 1（`duplicate-notify-idempotent.txt`）。
- 幕四 12/12：关单撞在途支付（CLOSE 400 ORDER_PAID → Query 决胜 → 答已付旧微信单、支付宝 PRECREATE 零调用、fulfill 恰 1）；干净切轨（CLOSE 204 → 旧单退役、新支付宝单唯一可付）；已关旧单晚到成功无二次履约（`close-race-*.txt`）。
- 幕五 6/6：Lago 四对象（subscription active 恰 1、gating invoice finalized+succeeded、succeeded payment 恰 1、purchase 钱包 granted 恰 1 笔）+ 产品面 credits/features（`lago-four-objects-wechat.txt`、`account-after-wechat.json`）。

- [ ] **Step 3: 收口与留档**

回归命令复跑一遍（Task 2 Step 1 三包）+ 红线 grep；`r6-head-verify/README.md` 记录拓扑、端口、轮次结论与任何勘误；`docs/plans/issue-72-ledger-83.md` 续写「r6 HEAD 复验」行（Commit+结果）。全过 → Issue #83 按「已交付+HEAD 回归通过」收口；有 FAIL → 转 Task 4。
Expected: 证据包完整、ledger 有本轮记录。

### Task 4:（条件触发）回归破坏的 TDD 修补

**Files:** 视破损点而定（见 Interfaces 的候选面）；特征测试加在对应既有 `_test.go`。

**Interfaces:**
- Consumes: Task 2/3 的 FAIL 现象与最小复现；#82 task12-17 的改动面（`git diff --name-only da4b544db..7d614752c` 中 25 个商业域文件为首要嫌疑：`purchase.go` 重放分支、`lago_settlement.go` 分类、`repository/commercial/order.go` 谓词、`payment_callbacks.go` body cap、`CheckoutPage.tsx`）。
- Produces: 修补提交（测试先行），不改变 #83 冻结契约：`CloseChannelOrder(ctx context.Context, tenantID uint64, orderID string) (OrderView, error)`（`order.go:567`）、`WechatProvider` 六方法签名（`provider.go:90` `Provider` 接口：`Create(context.Context, OrderRequest) (AttemptResult, error)`、`Query(context.Context, string) (AttemptResult, error)`、`Close(context.Context, string) error`、`Verify(context.Context, http.Header, []byte) (commercial.PaymentFact, error)`、`Refund`/`QueryRefund`、`MerchantID() string`）、`CommandKindSettlePurchasePayment` 幂等键绑定 attempt。

- [ ] **Step 1（RED）:** 以 FAIL 现象写特征测试（加到破损文件的对应 `_test.go`；测试名前缀 `TestRegression83`），运行确认其失败且信息指向根因。Run: `go test ./<pkg>/ -run 'TestRegression83' -count=1` → FAIL。
- [ ] **Step 2（GREEN）:** 在嫌疑文件做最小修复（保持既有签名与幂等语义；零 seam 改动）。Run 同上 → PASS。
- [ ] **Step 3（REFACTOR+回归）:** `go test ./internal/modules/commercial/... ./internal/handler/ -count=1` 八包 ok + `make check-backend-architecture`；若修在前端，`pnpm test:web && pnpm typecheck:web`。
- [ ] **Step 4:** 重跑 Task 3 的失败幕（单幕即可，但幕一主链必须整幕重跑）确认闭合；ledger 记 Ruling（根因/裁决/代价，格式照 R-83a..f）。
- [ ] **Step 5: Commit**（消息格式 `issue-72(#83-r6): regression fix — <现象>`）。

---

## 验收标准 → Task → 测试追踪矩阵

| Issue #83 验收标准 | 既有交付（核对基准） | 本计划复核动作 |
|---|---|---|
| AC1 微信真实回调验签、商户、金额和币种校验通过后才形成 Payment Fact | `TestWechatCallback*` 族（`payment_callbacks_test.go` 微信腿，真实 `WechatProvider.Verify`）、`wechat_native_test.go` 10 测试、v2 幕一 | Task 1 Step 2 在位核对；Task 2 Step 1/2 回归；Task 3 幕一（签名回调 200 → Fact → 三态） |
| AC2 漏通知通过查单恢复，重复通知不重复激活 | `TestWechatCallbackDuplicateDeliveryIdempotent`、`TestWechatQueryMapsTradeStates`/`PrefersTransactionID`、`RecoverOrderStatus`（`order.go:498`）、v2 幕二/三 | Task 2 Step 2；Task 3 幕二（`recovery-no-notify.txt`）+ 幕三（`duplicate-notify-idempotent.txt`，恰 1 计数） |
| AC3 关单与晚成功竞态保留资金事实并只履约一次 | `order_close_test.go` 竞态族（`TestCloseChannelOrderPaidRaceConfirmsFundFact` 等 9 测试，微信 stub）、`CloseChannelOrder`（`order.go:567`）、v2 幕四 | Task 2 Step 2；Task 3 幕四 12/12（ORDER_PAID 决胜/干净切轨/晚到成功） |
| AC4 微信验收使用真实受控环境，不以支付宝证据替代 | `issue-72-flow-evidence-83/` v1+v2 独立微信证据包（协议 stub+真实 Lago+真实 Stripe TEST+Playwright） | Task 3 全五幕在 HEAD 重跑，证据落 `r6-head-verify/`；支付宝 stub 仅作切轨腿对端 |
| 核心目标：复用同一激活流程（Fact→Lago activation→active 恰好一次） | `TestWechatCallbackDrainsToFulfilledWithFakePlatform`（组合级）、`createPurchaseSubscription` activation_rules + `settlePurchasePayment` | Task 2 Step 1/2；Task 3 幕一 `purchase-state-progression` 三态 + 幕五 Lago 四对象 + 红线 grep（零本地强制 active） |

## 真实流程验证方案（Task 3 展开）

- **链路**：真实浏览器（Playwright chromium + 真实 vite 渲染 `:5196`）→ Billing 页选付费套餐 → Checkout 页切「微信支付」（`CheckoutPage.tsx:343-345` 微信单选，提交带 `provider: channelRef.current`）→ 后端 `openOrder`（`order.go:378`，`Merchant: provider.MerchantID()`=MchID）→ `WechatProvider.Create`（`wechat.go:400`，POST `/v3/pay/transactions/native`）→ stub 验出站 WECHATPAY2 签名后回 `code_url` → 页面呈现 `weixin://wxpay/`「前往支付」（`CheckoutPage.tsx:367` `isSafeCheckoutUrl` 放行）→ stub `/stub/mark` SUCCESS + `/stub/notify` 推 AES-GCM+平台私钥签名回调 → `POST /api/v1/commercial/callbacks/wechat` 验签 → ConfirmPayment Fact → outbox → PurchaseFulfiller settle（真实 Stripe TEST PI off-session confirm）→ `deliver_stripe_webhook.py` 投 Lago 内建 webhook → finalize → active「权益已生效」。漏通知走 `GET /orders/:id` 查单恢复；竞态走页面换轨入口（`CheckoutPage.tsx:355-357`）触发 `CloseChannelOrder`。
- **与 v2 的差异仅三点**：HEAD 换成 `7d614752c`+、sqlite 库换 `data/issue83-flow-r6.db`、主角 email 每轮唯一（R-83a）；其余拓扑、密钥纪律、判据逐字沿用 v2 README。
- **披露边界（不伪造）**：微信腿为本地 RSA/AES 协议 stub（真实商户号/平台证书不可得）——证明「微信协议往返+验签+同一激活链」，不构成真实微信钱包付款证据；对称适用 R-4 对支付宝的口径。AC4 的「不以支付宝证据替代」由本票独立微信证据包满足。

## 待复核项（实现阶段定稿时必须逐条裁决）

1. **【待复核】HEAD 前移**：本计划基线是 `7d614752c`；若实现阶段 HEAD 已含 #84+ 提交，Task 1 Step 1 的祖先检查改为「#83 链可达即可」，Task 3 的回归价值随共享链路新改动相应扩展（`git diff --name-only 7d614752c..HEAD -- internal/modules/commercial internal/handler` 增量对照 Review Focus）。
2. **【待复核】Lago 共享栈租户占用**：82flow 栈（:48889）的租户号被多轮验证占据（R-83a 已至 tenant-34 口径）；r6 起栈前实测 `subscriptions` 表已占号，占位注册量按实况调整，或改起独立 `deploy/lago` compose 实例（端口仍避开清单）。
3. **【待复核】`deliver_stripe_webhook.py` 依赖**：webhook secret 经 rails runner 只读导出（R-83b）；若共享栈重建致路径失效，按 `deploy/lago-lab/payment-settle-trigger/` 的 lab.env 口径重取。注意 #82 r4/r5 改过该栈 `fixtures.py`/`phases.py`（diff 清单在案），与 r6 脚本无直接耦合，但起栈前 `git status` 确认无未提交漂移。
4. **【待复核】`v2_up_backend.sh` 的 env 面**：#82 task12-17 若新增了影响 settle 的环境变量（如 429/5xx 分类开关），r6 起栈 env 需比对 `providers_env.go`/`commercialplatform/config.go` 当前必填集补齐，避免假 FAIL。
5. **【待复核】AC4 字面「真实微信」与 stub 边界的最终口径**：若实现阶段取得真实微信商户测试凭据，则 Task 3 升级为真实商户小额付款验收（回调公网可达需内网穿透，端口纪律同样适用）；否则维持 R-4 披露边界口径收口。

## Out of scope（防与并行票冲突）

- 废弃 pending 单超时回收调度与公开 cancel 命令（`CommandKindCancelPurchaseSubscription`）——#84/#92 生命周期票（`CloseChannelOrder` 为其可复用 seam）。
- `RecoverOrderStatus` 查单见 CLOSED 的本地收口 UX——#84。
- partial/multiple-success 异常付款完整处置面——#84；充值 Credits 批次——#85。
- Commercial Platform seam 与 Lago 适配器改动（本计划预期零改动；Task 4 修补也不得触碰 seam 形状）。
- 前端新功能（渠道选择器/三态呈现/`weixin://` 白名单均已在 #81/#82/#83 交付；Task 4 只允许回归性修复）。

## Self-Review 结论

1. **Spec 覆盖**：AC1-AC4 与「复用同一激活流程」均映射到「既有交付核对基准 + 本计划复核动作」两列；spec L123/L124/L125/L127/L165/L218 逐条落在矩阵或 Global Constraints。2. **Step 扫描**：每个 Step 均为一条可运行命令+期望输出；Task 4 为条件触发的 TDD 模板（RED/GREEN/REFACTOR 各一步），未预设不存在的破损。3. **类型一致**：`CloseChannelOrder`/`WechatProvider` 方法/`CommandKindSettlePurchasePayment` 均按 HEAD 实读签名引用（`order.go:567`、`provider.go:90`、`purchase_settlement_command.go`）。4. **Review Focus 五行**均已归属 Task 2/3 的具体测试或幕。5. **比例**：本计划主体为复核命令、判据与待复核清单，无函数体转写；实现细节引用既有交付与 v2 证据，不重复。
