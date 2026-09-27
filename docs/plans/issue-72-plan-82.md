# [Lago 10 / Issue #82] 支付宝付款后恰好一次激活套餐 — Implementation Plan（R-4 双轨道修订版，第 4 轮：OCR r2 清偿 + 浏览器 UI 复验收口）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在第 3 轮（R-4 双轨道重做）已全量落地并经两轮 flowfix 修复的基线上，清偿 OCR 第 2 轮报告（`docs/plans/issue-72-ocr-issue-82-r2.md`，101 findings，本轮会话 8 点抽查证实产品代码 high/medium findings 全部未修）中购买/订单/回调/settle 链路的有效 findings，并补上第 3 轮明确移交的浏览器 UI 面真实复验（R-19）——使「支付宝付款后恰好一次激活」在真实环境（Lago 栈 + worktree 后端/前端 + 回环渠道 stub）下经无头浏览器完整走通三态呈现，四对象证据与红线自查齐备。

**Architecture:** 不动第 3 轮定稿的 α 双轨道机制（渠道真实收款 → WeKnora 驱动 Stripe Provider gated 结算扣款 → Lago 内建 webhook finalize → active；机制依据 `docs/migrations/lago/t11-payment-settle-trigger/DECISION.md` 五判据）。本轮是**正确性/安全性收口轮**，四个收敛面：① settle 链把扣款对象消歧收紧到「unsettled 候选集内 distinct gating 发票身份唯一，否则 fail-closed」，并移除对 customer `default_payment_method` 的永久改写（消除跨发票错扣与续期发票错路由两个真实扣款风险面）；② 购买面堵住 paid-awaiting 窗口的第二张渠道订单（同一订阅双重收款）与并发 checkout 竞态的 500 误分类（409 + winner 重放）；③ 瞬态权威侧故障（429/5xx）与确定性 invalid_response 的分类对齐（避免限流抖动把已付款订单铸成终态 attention）+ 匿名回调面 body 封顶；④ 前端 Checkout 的渠道切换/守卫/报价级死循环令牌/canceled 三态收口，evidence 脚本安全红线清偿（明文口令、token 落盘、SQL 拼接），最后以修复后的脚本在真实栈完成 R-19 浏览器复验。

**Tech Stack:** Go（gin + gorm，SQLite/PostgreSQL 双方言）、pinned Lago Community v1.53.0（digest 锁定 compose）、Stripe TEST 作受支持 provider 轨道（凭据仅 `~/.zcode/issue72-stripe.env` source 注入）、React+Vite 前端（apps/web）、TypeScript 共享契约（packages/contracts）、Playwright 1.63.0 无头浏览器（本会话 `npx playwright --version` 实测）。

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`（"Quotes, payments, and fulfillment" L118-127、"Public product states" L167-171、"Local persistence and state projection" L159-165、"Behavior and contract matrix" #5 L218 / #20 L233）；决策依据 `docs/migrations/lago/t02-payment-activation/DECISION.md`、`docs/migrations/lago/t11-payment-settle-trigger/DECISION.md`、`docs/plans/issue-72-user-rulings.md` R-1/R-3/R-4、`docs/adr/0012-lago-as-commercial-billing-authority.md`（含 2026-09-23 修订）、ADR-0014（seam 冻结）；findings 输入 `docs/plans/issue-72-ocr-issue-82-r2.md`（基线提交 `c6d027df8`）。

**修订历史与基线声明（复核定稿，不从零重写）：**
- 第 1/2 轮计划（D2=cancel intent+切 pm+`retry_payment`）已被 t10 证伪并随 `e0364d196` revert。
- 第 3 轮（R-4 双轨道重做版）**已全量执行完毕**：Task 1-11 对应提交 `7ddc4deab`…`356b10772`，代码审查第 1 轮（R82-1/R82-2）已修（`8e6a7487b`），真实流程验证失败修复两轮（flowfix-82 `c5a1917bb`、flowfix-82 r2 `de4f5dd86`，Ruling R-20~R-26）已修。收尾判定 (a)-(h) 中仅 (a) 的浏览器 UI 断言面未实跑（R-19 移交，本计划 Task 17 承接）。
- 本计划为第 4 轮：以集成分支 `codex/issue-72-lago` @ `da4b544db` 为基线（本 worktree 已合并，fast-forward 无冲突），输入为 OCR r2 报告 101 findings 的有效子集（购买/订单/回调/settle 链路 + 安全红线级 evidence 脚本 findings）。Task 编号延续第 3 轮（T12-T17），与 Ledger 执行记录表衔接。
- 本会话核实的未修证据（8 点抽查，全部命中未修状态）：`browser_sync_face_82.mjs:35` 明文口令字面量 `'issue82-Flow-Pw-b'`；`internal/handler/commercial.go:515-516` Purchase switch default→500（`:812` 的 `ErrPurchasePendingExists` case 属 CreateOrder，Purchase 面无）；`lago_settlement.go:359` `invoice_settings[default_payment_method]` 写入仍在；`purchase.go:253` 仅探测 pending 无 paid 窗口探测；回调组无 `MaxBytesReader`（全仓 grep 零命中）；`order.go:217` 仅匹配带点索引名；`lago_settlement.go:445-447` 非 200 全 InvalidResponse；`lago.go:688-696` readSubscriptionByIdentity 429 落 default→InvalidResponse。

---

## Global Constraints（批准需求原文 + 安全约束，每个 Task 隐含遵守；L 号为本会话实读 spec 行）

1. spec L121：「Initial subscriptions are pay-in-advance, have no trial, and use a payment activation rule. They remain incomplete until the gating payment succeeds.」
2. spec L122：「WeKnora owns WeChat Pay and Alipay request creation, callback verification, query, close, and refund behavior. A verified channel result produces an immutable Payment Fact.」
3. spec L123-125：「A supported Lago external-payment integration records the Payment. Only a Lago Subscription observed as active enables Entitlement and fulfillment.」「If Lago manual Payment does not release the activation rule in the pinned Community version, the implementation must use a Lago-supported external Payment Provider adapter. It may not force the Subscription active locally.」——**产品代码禁止**：伪造 provider webhook、直写 Lago 数据库、对 subscription 状态的任何本地直写、调用 Premium manual `POST /api/v1/payments`。
4. spec L126：「Paid Wallet top-up is not spendable until Lago confirms the Wallet transaction. Response loss is recovered with the original invoice, channel, and idempotency identities.」
5. spec L169：「WeKnora maps provider state into stable product states: awaiting payment, paid awaiting activation, active, …」；L170：公开 Billing API 永不暴露 Lago 凭据/URL/Organization id/原始状态枚举；L210 惯例（页面词汇稳定、闭合中文文案、无平台词汇）同步适用于本轮前端改动。
6. spec L105：「Monetary values use integer minor units and never pass through binary floating point.」；初期 CNY only。
7. spec L165：「WeKnora-to-Lago commands use a durable Outbox and stable idempotency identity. Timeout and server error are indeterminate outcomes that must be queried before replay.」
8. R-1/R-4 裁决：付款激活通道 = Lago 原生 Payment Provider 集成（选项②）；激活链 = α 双轨道。凭据纪律：Stripe/支付宝凭据只从环境变量读取（`~/.zcode/issue72-stripe.env` 仅 source 注入），任何产出（代码、测试、文档、证据、日志）不得出现 `sk_test`/`whsec_` 等可用凭据字面量——**OCR r2 在 `browser_sync_face_82.mjs:35` 发现的明文登录口令同类违规，Task 16 首项清偿**。
9. 安全约束（实现验收条件）：服务端发起外部请求仅允许 http/https 且请求前校验 host（拒绝 localhost/环回/私有/保留地址；本地 stub 验证经既有 `WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true` 显式豁免）；数据库查询一律参数绑定（OCR r2 发现 seed.sh 以字符串拼接内插 SQL——同属本约束清偿面）；源码/示例/测试无凭据字面量。
10. 冻结 seam 纪律（ADR-0014）：`platform.go` 三方法签名不变，仅 additive；provider 词汇（payment_intents、webhooks 等）只存在于 `commercialplatform/` 包内，绝不进入 port 类型、服务层或前端契约。
11. R-3 强制条件④：一切偏差仅限本计划列明条目（见「评估后不修清单」），不得外溢。
12. R-4 已披露边界：支付宝真实沙箱凭据缺失——用本地 RSA stub 证明协议往返+验签，**不得伪造沙箱证据**；AC4 证据如实标注 `ac4-sandbox-credentials-unavailable` 残余（不变）。
13. 冻结证据纪律：`r4-flow*/`、`reverify*/`、`settle-evidence/` 等已提交证据目录中的**历史快照脚本不回改**（证据冻结）；脚本缺陷修复只落在仍会被复用的顶层脚本（`browser_*_82.mjs`、`*_stub.py`、`seed.sh` 模板的复用面）与本轮新增的复验轮脚本。

## 实证契约基线（本轮新增核实；第 3 轮 F1-F15 继续有效，不重复）

- **F16（本会话实读）**：`lago_settlement.go` 现有幂等语义分层——步骤 (ii) 单次 `stripeListGatingIntents` 同时回答 unsettled 候选与 succeeded invoice 集（R-20 修复形态）；`settledInvoices[intent.LagoInvoiceID]` 命中 → 幂等回执（残留兄弟场景，:155）；空候选 + succeeded 集非空 → 幂等回执（settle-vs-finalize 窗口，:119-123）；金额守卫 `0 < intent.Amount ≤ payload.AmountFen`（:138-142）。**缺的正是 distinct-invoice 消歧闸**（r2:139 finding）。
- **F17（本会话实读）**：`lago_settlement.go:357-359` 的 default 写入与下单面 `lago_purchase.go:833` 是全仓仅有的两个 `invoice_settings[default_payment_method]` 写入点；settle 的 confirm（步骤 (iv)）显式携带 `payment_method`，default 写入对本次扣款非必需（r2:357 finding 的实证基础）。
- **F18（本会话实读）**：`purchase.go:253` 防重开检查只调 `CurrentPendingPurchaseOrder`（state=pending）；`repository/commercial/order.go:340-352` 的 `CurrentPayablePendingOrderView` pending 偏好循环已排除 channel-failed（A-29/F109），但其后的 fallback `if len(rows) > 0 { return rows[0], nil }` 无条件返回最新行（r2:348 finding）；`uq_purchase_pending_per_tenant` 谓词只含 `state='pending'`，paid 单退出索引范围（r2:253 finding 的机制基础）。
- **F19（本会话实读）**：`purchase_fulfillment.go:175-181` 错误分类——`ErrPlatformUnreachable || ErrPlatformUnconfigured → return nil`（保持 pending），其余 → `markActivationState(attention)` 终态；因此适配器侧任何把 429/5xx 归入 InvalidResponse 的读取（`boundProviderCustomerID`、`readSubscriptionByIdentity`）都会把瞬态限流铸成终态 attention（r2:445/:688 findings）。
- **F20（本会话实测）**：docker daemon 当前**未运行**（`docker info` → `failed to connect to the docker API at unix:///Users/wuyongjun/.orbstack/run/docker.sock`，OrbStack 未启动；`docker ps -a` 同样失败）——任务简报所记「Lago 栈正在运行」已过时；Task 17 前置步骤为启动 OrbStack 并重建栈。`npx playwright --version` → **1.63.0**。端口 `:5272/:5273`（其他会话保留，禁用）与 `:8093/:5194/:8294` 本会话 `lsof` 实测空闲；`:48889/:48890` 空闲（82flow 栈已拆）。
- **F21（本会话实读）**：前端 `CheckoutPage.tsx` 现状与 r2 findings 逐点吻合——`submittedChannel` 初值 `'alipay'`（:151）；`orderIdRef.current = order.id` 与 error setState 无 active/isCurrent 守卫（:197-206 区域）；`channel` 在加载 effect 依赖数组（:226）；`QUOTE_LEVEL_CONFLICT_TOKENS` 缺 `invoice_quote_mismatch` 与 `this plan version is not purchasable yet; please re-quote later`（:101-106，后者的中文文案映射 :96 已存在）；`purchaseStateMessage` 无 `canceled` case（:28-38）；default 分支为三层嵌套三元（:33-36）。`purchaseErrorText` 映射表 :88-97。
- **F22（本会话实读）**：`repository/commercial/order.go:206-217` 索引名匹配注释自述 gorm 默认命名，实际匹配串是带点形式 `idx_commercial_orders.quote_id`；PG gorm NamingStrategy 生成的是下划线形式 `idx_commercial_orders_quote_id`；`order_test.go:596` 硬编码了同样的带点名字（测试通过但与真实驱动行为不符）。
- **F23（Ledger 实录，继续有效）**：node v26.4.0 下 `pnpm test:web` 全量 2305/2305 PASS、typecheck:web 残留 5 错（mermaid.ts/PlatformShell.tsx/DevMarkdownPage.tsx，预存、非本票引入，独立票清偿——本轮前端回归以「不新增错误」为判据）；`commercialplatform` 全包测试 ~72-77s。
- **F24（本会话实读）**：回调挂载形态——`routes_commercial.go:104-117` 注释明示「publicly reachable and authenticated by provider signature verification」，handler 直挂父组（无租户参数）；`payment_callbacks.go` 两分支 `io.ReadAll(c.Request.Body)` 无上限（r2 auth.go:71 finding 的落点基础）。

## 设计决策记录（本轮定稿，执行者按此实现）

- **D9 distinct-invoice 消歧闸（r2:139 high）**：`settlePurchasePayment` 在解析出 unsettled 候选后、驱动 latest 之前，统计候选集内 distinct `LagoInvoiceID`：**>1 → `ErrPlatformInvalidResponse` fail-closed，绝不驱动任何扣款**（attention 面交运营，r2 建议首选方向）。单一发票的残留兄弟（R-20 合法场景）不受影响——兄弟共享同一 invoice id，distinct==1。这是「latest-created 启发式」之外的硬闸：金额守卫（F16）只做 1..AmountFen 限幅，挡不住金额更小的异发票 intent。
- **D10 移除 settle 链 default 改写（r2:357 high）**：删除步骤 (iii) 中对 `POST /v1/customers/{id}` 的 `invoice_settings[default_payment_method]` 写入（attach 保留）；confirm 显式携带 payment_method，本次扣款不依赖 default。后果披露：续期发票的自动扣款路由回到下单面设定的 default（3DS pm → requires_action，fail-closed 不自动成功），优于「每月对结算工具真实扣款」的资金错配；续期收款路由本就是 R-4 生产前回议项 + #84 生命周期面（Ledger 已移交，本轮维持）。下单面 `lago_purchase.go:833` 的 default 写入**不动**（gated create 首次 auto-collection 依赖它，F1/t02 实证）。
- **D11 paid-awaiting 窗口防重开（r2:253 high）**：仓库层新增 `CurrentPaidAwaitingActivationPurchaseOrder(ctx, tenantID, amountFen, currency)`（kind=purchase + 价面匹配 + `state='paid'` + `fulfillment_state` 非 fulfilled，Order(`created_at DESC, id ASC`) 取最新）；服务层 `purchase.go` 在 pending 探测 NotFound 后追加 paid 探测：命中且 `quoteBoughtPlan` 匹配 snap.PlanKey → 直接返回该订单视图（走既有 `purchaseView` 合成 `paid_awaiting_activation`，与 PurchaseStatus 投影一致）；异 plan → `ErrPurchasePlanConflict`（与 pending 探测同防线）。**不再开第二张渠道订单**——堵住「支付成功但页面未激活时再次点购买 → 新 quote → 新渠道订单 → 双重收款」。
- **D12 并发 checkout 竞态收口（r2:306 medium + r2:506 medium）**：`purchase.go` CreateOrder 撞 `ErrPurchasePendingExists` 的 retry 链：重读一次 `CurrentPayablePendingOrderView`，读到（winner 多半已持久化 checkout_url）→ 重放其视图；读不到 → 返回 `ErrPurchasePendingExists` 哨兵；sweep 失败不再静默丢弃 serr（log 后仍返回原哨兵，保持确定性冲突面）。handler 侧 Purchase switch 补 `case errors.Is(err, repocommercial.ErrPurchasePendingExists): 409 "purchase pending exists"`（与 CreateOrder 面 :812 同族），并发竞态败者拿到 409 + 可重试语义，不再是「不可重试的 500」误分类。
- **D13 瞬态分类对齐（r2:445/:688 medium）**：`boundProviderCustomerID` 与 `readSubscriptionByIdentity` 的状态分类改为与 `customerProviderBound`（lago_purchase.go:505-506）/`finalizedInvoiceIDs`（A-25/F96 纪律）一致：`429 || >=500 → ErrPlatformUnreachable`（fulfiller :176 的 pending 分支因此覆盖瞬态限流），其余非 2xx/4xx → `ErrPlatformInvalidResponse`。`settle` 入口 unconfigured 检查同时纳入 `StripeAPIKey == ""`（r2:166 low，配置缺失不应表现为权威拒绝）。`fulfiller` 本身不改（D13 修适配器映射后 :175-181 分类自然正确）。
- **D14 回调面 body 封顶（r2 auth.go:71 medium）**：`payment_callbacks.go` handler 入口（两个 provider 分支共用处）`c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)`（1 MiB，Alipay/WeChat notify 报文仅数 KB）；超限 → `io.ReadAll` 报错 → 按既有失败面应答（非 5xx、零落库）。落点选 handler 而非路由组：与项目「各 handler 自行加限」惯例一致（craft.go/upload_limit.go 同款），测试可直接 httptest 构造。
- **D15 前端六点收口（r2 CheckoutPage ×5 + canceled）**：(a) `run()` 内 await 后的 `orderIdRef.current` 写入与 error setState 补 `active && scopeController.isCurrent(currentScope.scope)` 守卫（与 ready setState 对齐）；(b) `channel` 移出加载 effect 依赖——`channelRef` 承载最新选择，提交时读 ref，切换单选不再重跑加载/多发请求；(c) `submittedChannel` 初值改 `null`（深链打开既有订单不臆测渠道，失配横幅仅在真正提交后显示）；(d) getOrder 加载失败与购买创建失败文案分流（`订单加载失败，请稍后重试` vs `purchaseErrorText`），非 ready 兜底英文文案同步收敛为闭合中文；(e) `QUOTE_LEVEL_CONFLICT_TOKENS` 补 `invoice_quote_mismatch` 与 `this plan version is not purchasable yet; please re-quote later`（两成因都冻结在 quote 侧、重放必然同一 409，不补则重试死循环）；(f) `purchaseStateMessage` 增 `case 'canceled': return '该购买已取消，请重新发起购买'`，default 改 if 链（消嵌套三元），支付入口渲染处 canceled 时隐藏「前往支付」；`PURCHASE_STATE_LABEL` 共享映射抽到 `order-state.ts` 供 Checkout/Billing 两页复用（消两套措辞）。
- **D16 评估后不修（显式记录，防范围外溢）**：
  - `lago_settlement.go:118-123` 空候选幂等分支的客户维度绑定（r2:118 medium）：**保持现状**——该语义是 R-17/R-20 修复的必要条件（settle-vs-finalize 窗口内重放必须幂等回执，改 fail-closed 会重新引入「窗口内 attention 抢占唯一键」缺陷）；「本单 intent 被权威作废」场景由 D7 总预算（10min → attention）兜底收敛；完整 gating 发票身份绑定需要下单时冻结 invoice id 进 payload（跨 #81 seam 面 + #84 异常付款/生命周期面），Ledger 记录移交。
  - `readPurchaseInvoiceFees` N+1 读放大（r2:624 medium 的性能半边）：R-23 已披露「正确性优先、读路径非热点」；本轮只修正确性半边（created_at 显式比较，见 Task 12）。
  - `r4-flow*/`、`reverify*/` 历史冻结脚本不回改（A-12 锚点/catch 块/共享样板的缺陷由本轮新增复验脚本以 `_browser_lib.mjs` 共享库落实）；三份 seed.sh 收敛为单一脚本不做（冻结），security/bug 级缺陷逐份同修。
  - fieldset 内联样式等纯风格项；typecheck:web 预存 5 错（独立票）；#84/#92 已移交项（废弃 pending 单超时回收、cancel 命令、续期收款路由）。

## Review Focus（spec 隐含但无任务测试会咬人的输入类；每条注明归属测试）

1. **settle 时同 customer 名下混入另一张发票的 unsettled intent**（历史残留/续期）：fail-closed 零扣款，绝不凭 latest-created 盲扣——Task 12 `TestLagoSettleMultipleInvoiceCandidatesFailClosed`。
2. **支付成功但激活未落地的窗口内用户再次提交购买**（换新 quote 双击）：返回既有 paid 订单视图（paid_awaiting_activation 面），不开第二张渠道订单——Task 13 `TestPurchasePaidAwaitingWindowDoesNotOpenSecondOrder`。
3. **并发双标签页 checkout，败者撞 pending 唯一索引且 winner 尚在渠道 Create 窗口**：409 冲突令牌 + winner 订单重放，绝不 500 误分类——Task 13 `TestPurchaseConcurrentPendingExistsAnswers409WithReplay`。
4. **观察窗内一次 Lago 429 限流**：事件保持 pending 可重试，绝不懈成终态 attention——Task 12 `TestLagoSettleBindingRead429KeepsRetriable` + `TestLagoSubscriptionRead429Unreachable`。
5. **匿名超大回调体**（内存耗尽 DoS 面）：1 MiB 封顶、快速拒绝、零落库——Task 14 `TestPaymentCallbackRejectsOversizedBody`。

## File Structure（新建/修改全景）

- Modify: `internal/modules/commercial/commercialplatform/lago_settlement.go`（D9 消歧闸 / D10 删 default 写入 / D13 boundProviderCustomerID 分类 + StripeAPIKey unconfigured）——Task 12
- Modify: `internal/modules/commercial/commercialplatform/lago_settlement_test.go`（新增 4 用例 + 既有 3 用例断言调整）——Task 12
- Modify: `internal/modules/commercial/commercialplatform/lago.go`（readSubscriptionByIdentity 429→Unreachable）+ `lago_test.go`（新用例）——Task 12
- Modify: `internal/modules/commercial/commercialplatform/lago_purchase.go`（readPurchaseInvoiceFees created_at 显式比较）+ `lago_purchase_test.go`（乱序 index 用例）——Task 12
- Modify: `internal/modules/commercial/repository/commercial/order.go`（`CurrentPaidAwaitingActivationPurchaseOrder` 新增 / 索引名双形式 / fallback 跳 dead pending）+ `order_test.go`（3 组用例 + :596 修正）——Task 13
- Modify: `internal/modules/commercial/service/commercial/purchase.go`（paid 窗口探测 / 竞态 retry 链 / serr 不吞）+ `purchase_test.go`——Task 13
- Modify: `internal/handler/commercial.go`（Purchase switch 补 409 / legacy POST /orders 挂回 plan 校验 + 降级日志）+ 既有 handler 测试文件——Task 13
- Modify: `internal/handler/payment_callbacks.go`（MaxBytesReader）+ `payment_callbacks_test.go`——Task 14
- Modify: `internal/modules/commercial/service/commercial/fulfillment.go`（isSubscriptionPurchase NotFound 区分 + 日志）——Task 14
- Modify: `internal/modules/commercial/service/commercial/purchase_fulfillment.go`（quote snapshot 解析失败/publication NotFound → attention；瞬态 DB → nil）+ `purchase_fulfillment_test.go`——Task 14
- Modify: `apps/web/src/commercial/CheckoutPage.tsx` + `apps/web/src/commercial/BillingPage.tsx` + `apps/web/src/commercial/order-state.ts`（D15 全部）+ 对应 `*.test.ts(x)`——Task 15
- Modify: `docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs`（明文口令→PASSWORD 等）、`browser_flow_82.mjs`、`browser_paid_face_82.mjs`、`alipay_gateway_stub.py`、`wechat_pay_stub.py`——Task 16
- Modify: `docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh`、`r4-flow2/seed.sh`、`r4-flow3/seed.sh`（security/bug 级同修）、`settle-evidence/deliver_stripe_webhook.py`、`settle-evidence/prepare_t9_env.sh`——Task 16
- Modify: `deploy/lago-lab/payment-settle-trigger/phases.py`、`fixtures.py`、`run_lab.py`、`lab.sh`、`test_phases.py`（lab security/bug 级）——Task 16
- Create: `docs/plans/issue-72-flow-evidence-82/_browser_lib.mjs` + `r5-verify/`（复验轮目录：seed.sh、四腿脚本、证据）——Task 16/17
- Modify: `docs/plans/issue-72-ledger-82.md`（收尾判定）、`docs/plans/issue-72-flow-evidence-82/README.md`（复验轮记录）——Task 17

---

### Task 12: settle 链正确性收口——distinct-invoice 消歧闸、default 改写移除、瞬态分类对齐

**Files:**
- Modify: `internal/modules/commercial/commercialplatform/lago_settlement.go`（:104-176 settle 主链、:357-359 default 写入、:440-447 boundProviderCustomerID、:166 unconfigured 检查）
- Modify: `internal/modules/commercial/commercialplatform/lago_settlement_test.go`
- Modify: `internal/modules/commercial/commercialplatform/lago.go`（:686-696 readSubscriptionByIdentity）
- Modify: `internal/modules/commercial/commercialplatform/lago.go` 所属测试文件中订阅读取用例所在文件（`lago_test.go`，若无则建 `lago_subscription_read_test.go`）
- Modify: `internal/modules/commercial/commercialplatform/lago_purchase.go`（readPurchaseInvoiceFees 选行逻辑）

**Interfaces:**
- Consumes: 第 3 轮 Task 5 交付的 `settlePurchasePayment`、`stripeListGatingIntents`、`latestIntent`、`stripeAttachSettlePaymentMethod`、`TestLagoSettle*` 既有 17 用例（lago_settlement_test.go:268-643）；`commercial.ErrPlatformUnreachable/ErrPlatformInvalidResponse/ErrPlatformUnconfigured` 闭合哨兵。
- Produces: settle 主链新增行为契约——(1) unsettled 候选 distinct `LagoInvoiceID` >1 → `ErrPlatformInvalidResponse`（零扣款）；(2) settle 链不再调用 `POST /v1/customers/{id}`（default 改写移除，attach 保留）；(3) `boundProviderCustomerID`/`readSubscriptionByIdentity` 的 429/5xx → `ErrPlatformUnreachable`；(4) settle 入口 `StripeSettlePmToken=="" || StripeAPIKey==""` → `ErrPlatformUnconfigured`；(5) `readPurchaseInvoiceFees` 选中行按发票 `created_at` 显式比较（不信任 index 顺序）。签名零变化（内部行为收口）。

- [ ] **Step 1: 写失败测试**（`lago_settlement_test.go` 新增，复用既有 httptest 伪 Stripe/Lago 构造 helper）：

```go
func TestLagoSettleMultipleInvoiceCandidatesFailClosed(t *testing.T) {
    // Stripe stub: customer 名下 2 条 unsettled PI，metadata.lago_invoice_id 互不相同
    //   （pi_a created 较早 inv_old、pi_b created 较新 inv_cur，金额都在 1..AmountFen 内）
    // 断言: settle → ErrPlatformInvalidResponse；伪 Stripe 只收到 1 次 GET（列表），
    //   attach/update/confirm 写调用 0 次（零扣款硬闸）
}
func TestLagoSettleDoesNotTouchCustomerDefault(t *testing.T) {
    // 正常驱动路径（单 unsettled PI）→ settle 成功；
    // 断言: 伪 Stripe 请求序列为 GET intents + POST payment_methods/{pm}/attach
    //   + POST payment_intents/{pi} + POST payment_intents/{pi}/confirm，
    //   无任何 POST /v1/customers/{id}（default 永不改写，D10）
}
func TestLagoSettleBindingRead429KeepsRetriable(t *testing.T) {
    // Lago stub: GET /api/v1/customers/{ext} → 429
    // 断言: err 为 ErrPlatformUnreachable（非 InvalidResponse）——fulfiller 因此保持 pending
}
func TestLagoSettleFailsClosedWithoutStripeApiKey(t *testing.T) {
    // cfg.StripeAPIKey=""（SettlePmToken 有值）→ ErrPlatformUnconfigured，零出站
}
```

`lago_test.go`（或新文件）与 `lago_purchase_test.go`：

```go
func TestLagoSubscriptionRead429Unreachable(t *testing.T) {
    // GET /api/v1/subscriptions?external_id= → 429 → ErrPlatformUnreachable（原来落 default→InvalidResponse）
}
func TestLagoReadPurchaseInvoiceFeesExplicitCreatedAtOrdering(t *testing.T) {
    // index 返回顺序与 created_at 相反（旧在前）+ 两张发票详情（created_at 一新一旧）
    // 断言: 选中 created_at 最新的那张（不信任 index 行序）
}
```

既有用例调整（GREEN 后同步）：`TestLagoSettleDrivesProviderRails`（五次调用序列去 default → 四次）；`TestLagoSettleAttachRailDerivesIdempotencyKeys`（`:default` 键断言移除，attach 键保留）；`TestLagoSettleResidualSiblingIntentReplaysIdempotently`/`TestLagoSettleSettledWindowReplaysIdempotently`（兄弟/窗口语义不变，确认仍绿）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1 -run 'TestLagoSettleMultipleInvoice|TestLagoSettleDoesNotTouch|TestLagoSettleBindingRead429|TestLagoSettleFailsClosedWithoutStripeApiKey|TestLagoSubscriptionRead429|TestLagoReadPurchaseInvoiceFeesExplicit'`
Expected: FAIL（新用例断言与现行为相反：default 仍被写、429 归 InvalidResponse、无消歧闸）。

- [ ] **Step 3: 实现**（全部为 lago_settlement.go/lago.go/lago_purchase.go 内部修改，签名不动）：(i) `latestIntent` 解析前对 `intents` 统计 distinct `LagoInvoiceID`，>1 即返回包装 `ErrPlatformInvalidResponse` 的错误（文案含 distinct 计数）；(ii) 删除 default 写入调用及其幂等键（注释保留 D10 决策与 #84 回议指针）；(iii) 两处状态分类改 switch（429/5xx→Unreachable）；(iv) unconfigured 检查加 `StripeAPIKey`；(v) `readPurchaseInvoiceFees` 的发票详情解析 struct 增 `CreatedAt`，选行时显式比较。

- [ ] **Step 4: 跑测试确认通过 + 全包回归**

Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1 -run 'TestLagoSettle|TestLagoSubscriptionRead|TestLagoReadPurchaseInvoiceFees'` → PASS（含调整后的既有用例）。
Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1` → 全量 PASS（~72-77s）。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/commercial/commercialplatform/
git commit -m "issue-72(#82): (task12) settle-chain correctness — distinct-invoice fail-closed, drop default-pm rewrite, transient 429/5xx classification (OCR r2)"
```

---

### Task 13: 购买竞态面收口——paid-awaiting 防重开、并发 409、索引名与 fallback

**Files:**
- Modify: `internal/modules/commercial/repository/commercial/order.go`（:206-217 索引名双形式、:340-356 fallback 谓词、新增 `CurrentPaidAwaitingActivationPurchaseOrder`）
- Modify: `internal/modules/commercial/repository/commercial/order_test.go`（:596 修正 + 3 组新用例）
- Modify: `internal/modules/commercial/service/commercial/purchase.go`（:253 后追加 paid 探测、:306-312 竞态 retry 链）
- Modify: `internal/modules/commercial/service/commercial/purchase_test.go`
- Modify: `internal/handler/commercial.go`（Purchase switch :494-516 区域补 409 case；:815-825 legacy /orders 挂回 plan 校验 + 降级日志）+ `internal/handler/commercial_purchase_test.go`（新用例）

**Interfaces:**
- Consumes: `CurrentPendingPurchaseOrder`/`CurrentPayablePendingOrderView`（order.go 既有）；`quoteBoughtPlan`（purchase.go 既有）；`ErrPurchasePendingExists`/`ErrOrderNotFound`/`ErrQuoteNotFound`（repocommercial 既有）；`orderWire`（handler 既有）。
- Produces:
  - `func (r *OrderRepository) CurrentPaidAwaitingActivationPurchaseOrder(ctx context.Context, tenantID uint64, amountFen int64, currency string) (OrderRow, error)`——`kind='purchase' AND amount_fen=? AND currency=? AND state='paid' AND (fulfillment_state IS NULL OR fulfillment_state NOT IN ('fulfilled'))`，`Order("created_at DESC, id ASC")`；无行 → `ErrOrderNotFound`（参数绑定，安全约束 9）。
  - 服务层行为契约：pending NotFound 后 paid 命中且同 plan → 返回既有订单视图（不再 CreateOrder）；异 plan → `ErrPurchasePlanConflict`。
  - handler 行为契约：Purchase 对 `ErrPurchasePendingExists` 答 `409 {"error":"purchase pending exists"}`（可附 winner 订单，若服务层重放视图已覆盖则仅令牌）；legacy POST /orders 挂回前比对两侧 quote 的 PlanKey，异 plan → 裸 409 不附 order。
  - 仓库层行为契约：PG 唯一冲突匹配同时接受 `idx_commercial_orders_quote_id`（下划线，真实 gorm 命名）与 `idx_commercial_orders.quote_id`（防御）；fallback 只在存在非 dead 行时返回（跳过 channel-failed pending，取最新 paid/fulfilled 行）。

- [ ] **Step 1: 写失败测试**：

```go
// order_test.go
func TestQuoteAlreadyUsedMatchesUnderscoreIndexName(t *testing.T) {
    // errors.New(`duplicate key value violates unique constraint "idx_commercial_orders_quote_id"`)
    // → isQuoteAlreadyUsed == true（PG 真实驱动形态；带点形式用例保留为防御行）
}
func TestCurrentPayableFallbackSkipsDeadPending(t *testing.T) {
    // 同价面两行: O1 paid（较早）、O2 pending+channel_failed（较新）
    // 断言: 返回 O1（paid）而非 O2（dead pending 遮蔽 paid 的回归锁定）
}
func TestCurrentPaidAwaitingActivationPurchaseOrder(t *testing.T) {
    // paid+未 fulfilled 命中 / fulfilled 后 NotFound / 异 kind 或异价面 NotFound 三形态
}
// purchase_test.go
func TestPurchasePaidAwaitingWindowDoesNotOpenSecondOrder(t *testing.T) {
    // 种子: 租户有 paid 未 fulfilled 购买单（quote 同 plan）
    // 断言: Purchase(新 quote) 返回该订单视图（Order ID 不变），CreateOrder 未被调用，
    //   视图经 purchaseStatus 合成 paid_awaiting_activation
}
func TestPurchasePaidAwaitingWindowDifferentPlanConflicts(t *testing.T) {
    // paid 单 quote 是 plan A，新购买 plan B → ErrPurchasePlanConflict
}
func TestPurchaseConcurrentPendingExistsAnswers409WithReplay(t *testing.T) {
    // CreateOrder 第一次返回 ErrPurchasePendingExists 且 CurrentPayablePendingOrderView 可读
    // 断言: 服务层重放 winner 视图（不二次 CreateOrder）；不可读时返回哨兵，
    //   handler 层断言 409 + "purchase pending exists"（不再落 default 500）
}
```

- [ ] **Step 2:** `go test ./internal/modules/commercial/repository/commercial/ ./internal/modules/commercial/service/commercial/ ./internal/handler/ -count=1 -run 'TestQuoteAlreadyUsedMatches|TestCurrentPayableFallback|TestCurrentPaidAwaiting|TestPurchasePaidAwaiting|TestPurchaseConcurrentPending'` → Expected: FAIL。

- [ ] **Step 3: 实现**（三文件按 Produces 契约；handler 409 case 插在 ErrQuoteNotFound 之后 default 之前；legacy 挂回用 `QuoteSnapshotForTenant` 比对，读失败 log 一行后走裸 409）。

- [ ] **Step 4:** 同命令 → PASS；`go test ./internal/modules/commercial/... ./internal/handler/ ./internal/router/ -count=1` 全量 PASS。

- [ ] **Step 5: Commit** `git commit -m "issue-72(#82): (task13) purchase race hardening — paid-awaiting no-reopen, pending-exists 409, PG index name, fallback predicate (OCR r2)"`

---

### Task 14: 回调面封顶与 drain 保护

**Files:**
- Modify: `internal/handler/payment_callbacks.go`（handler 入口 MaxBytesReader）
- Modify: `internal/handler/payment_callbacks_test.go`
- Modify: `internal/modules/commercial/service/commercial/fulfillment.go`（:341-345 isSubscriptionPurchase）
- Modify: `internal/modules/commercial/service/commercial/purchase_fulfillment.go`（:119-123 publication 读 + quote snapshot 解析）
- Modify: `internal/modules/commercial/service/commercial/purchase_fulfillment_test.go`

**Interfaces:**
- Consumes: `payment_callbacks.go` 既有 POST 分支与失败面令牌；`FulfillmentStateAttention`/`markActivationState`（purchase_fulfillment.go 既有）；`gorm.ErrRecordNotFound`。
- Produces: (1) 回调 body 上限 1 MiB（`http.MaxBytesReader`，超限走既有失败应答、零落库）；(2) `isSubscriptionPurchase` 对非 NotFound 的 quote 读失败记 Warn 日志（gorm.ErrRecordNotFound 保持静默 false——legacy 充值种子语义）；(3) `Fulfill` 内 quote snapshot JSON 解析失败、publication NotFound → `markActivationState(attention)` + 返回 nil（确定性失败不再上抛中止共享 drain 批次）；publication 瞬态 DB 错误 → 返回 nil 保持 pending。

- [ ] **Step 1: 写失败测试**：

```go
// payment_callbacks_test.go
func TestPaymentCallbackRejectsOversizedBody(t *testing.T) {
    // 构造 2 MiB body POST /api/v1/commercial/callbacks/alipay（无需有效签名）
    // 断言: 响应非 5xx（既有失败令牌面）、DB attempts/orders 零新增行
}
// purchase_fulfillment_test.go
func TestPurchaseFulfillPublicationMissingTurnsAttentionNotAbort(t *testing.T) {
    // 订单 paid + 快照 plan_key/version 在 publications 表无行
    // 断言: Fulfill 返回 nil、fulfillment_records 记 attention、同批后续事件（top-up）仍被处理
}
func TestPurchaseFulfillQuoteSnapshotCorruptTurnsAttention(t *testing.T) {
    // quote SnapshotJSON 非法 JSON → 同上（不 panic、不上抛）
}
```

- [ ] **Step 2:** `go test ./internal/handler/ ./internal/modules/commercial/service/commercial/ -count=1 -run 'TestPaymentCallbackRejectsOversized|TestPurchaseFulfillPublication|TestPurchaseFulfillQuoteSnapshot'` → Expected: FAIL。
- [ ] **Step 3: 实现**（MaxBytesReader 放两 provider 分支共用的入口处，一次赋值；fulfiller 两处错误分类按 Produces）。
- [ ] **Step 4:** 同命令 → PASS；`go test ./internal/modules/commercial/... ./internal/handler/ ./internal/router/ -count=1` 全量 PASS；`make check-backend-architecture && make verify-module-moves` → 0 violations / 16 manifests OK。
- [ ] **Step 5: Commit** `git commit -m "issue-72(#82): (task14) callback body cap + drain-shape guards (OCR r2)"`

---

### Task 15: 前端 Checkout/Billing 收口（D15 六点）

**Files:**
- Modify: `apps/web/src/commercial/CheckoutPage.tsx`（:28-38 purchaseStateMessage、:88-106 令牌集、:145-156 channel/submittedChannel、:195-226 run() 守卫与 catch 分流、:226 effect 依赖、支付入口渲染处）
- Modify: `apps/web/src/commercial/order-state.ts`（`PURCHASE_STATE_LABEL` 共享映射）
- Modify: `apps/web/src/commercial/BillingPage.tsx`（:118 区域改用共享映射）
- Modify: `apps/web/src/commercial/CheckoutPage.test.tsx` + `apps/web/src/commercial/BillingPage.test.ts` + `apps/web/src/commercial/order-state.test.ts`（三文件均已实核存在）

**Interfaces:**
- Consumes: `PurchaseView['state']`（contracts 既有闭合集，含 `canceled`，无契约改动）；`PurchaseChannel` 类型；`isQuoteLevelConflict`（CheckoutPage 既有导出）。
- Produces: `export const PURCHASE_STATE_LABEL: Record<PurchaseView['state'], string>`（order-state.ts，两页复用）；CheckoutPage 行为契约——切渠道不重跑加载、深链不臆测渠道、getOrder 失败独立文案、死 quote 令牌重试走新报价、canceled 有专属文案且隐藏支付入口。Task 17 浏览器腿断言消费这些文案。

- [ ] **Step 1: RED**——jsdom 单测新增（对齐 `apps/web/src/commercial/` 既有测试模式）：
  - `purchaseStateMessage` canceled → 「该购买已取消，请重新发起购买」；canceled 态不渲染「前往支付」入口。
  - `isQuoteLevelConflict('invoice_quote_mismatch')` / `('this plan version is not purchasable yet; please re-quote later')` → true。
  - 渠道单选切换 → 加载 effect 不重跑（getOrder 调用计数不变）、提交体 provider 仍为所选值。
  - 深链打开（无提交）→ 无渠道失配横幅；提交后切换 → 横幅出现。
  - getOrder 404 → 错误面「订单加载失败，请稍后重试」（非「购买未能创建」）。
  Run: `npx tsx --test 'src/commercial/**/*.test.ts' 'src/commercial/**/*.test.tsx'`（apps/web 目录内）→ Expected: FAIL。
- [ ] **Step 2: GREEN 实现**（按 D15 (a)-(f)；order-state.ts 抽共享映射，两页接入）。
- [ ] **Step 3:** 同命令 → PASS（商业面子集 ≥ 22/22 既有 + 新增）；`node --import tsx --test 'src/**/*.test.ts' 'src/**/*.test.tsx'`（node v26.4.0，`nvm use 26` 或既有 node26 路径）全量 PASS；`pnpm typecheck:web` 错误数不增（基线 5 错为预存，F23）。
- [ ] **Step 4: Commit** `git commit -m "issue-72(#82): (task15) checkout/billing polish — guards, channel ref, canceled state, quote-conflict tokens (OCR r2)"`

---

### Task 16: evidence 脚本与 t11 lab 清偿（security/bug 级）+ 复验轮共享库

**Files:**
- Modify: `docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs`（:35 明文口令 → `PASSWORD` 变量【安全红线首项】；import 顶部化）
- Modify: `docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs`（EXPECT_CNY 必填 env、EV/TAG 参数化、catch 块、process.exitCode）
- Modify: `docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs`（同上参数化 + import 顶部化）
- Modify: `docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py`（except 收窄 + finally 局部 import 删）
- Modify: `docs/plans/issue-72-flow-evidence-82/wechat_pay_stub.py` + `docs/plans/issue-72-flow-evidence-81/wechat_pay_stub.py`（`FLOW82_WECHAT_PORT` env、81 目录 docstring 残句修正）
- Modify: `docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh`、`r4-flow2/seed.sh`、`r4-flow3/seed.sh`（逐份同修：token 落盘前 jq 脱敏、`DB_PATH :?` 守卫 + 文件存在检查、`SCRIPT_DIR` 先于 cd 解析、`LAGO_KEY` 非空断言 + `order by created_at`、PAD 正整数校验、reg/login `|| true` 保 000 诊断、TENANT 期望序号断言（r4-flow2 形态）、SQL 改 sqlite3 `?1` 参数绑定、密码经 `--data @-` stdin）
- Modify: `docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py`（`--base` 优先 `LAGO_INTEGRATION_BASE_URL`、`import urllib.error`、删死 `import sys`）
- Modify: `docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh`（GraphQL code 过滤 `if "code" in c`、export 前临时变量 + 非空校验）
- Modify: `deploy/lago-lab/payment-settle-trigger/phases.py`（provider_code 白名单 `re.fullmatch(r"[A-Za-z0-9_-]+")` 再内插、WSECRET 经 stdin、graphql 用 `clients._proxyless_opener` origin 绑定、cleanup 增 provider 删除或显式记录残留、PHASE_ORDER 注释指向契约测试）
- Modify: `deploy/lago-lab/payment-settle-trigger/fixtures.py`（删 `sys.path.insert`）
- Modify: `deploy/lago-lab/payment-settle-trigger/run_lab.py`（Timeline.text() 无副作用）+ `test_phases.py`（新增 `test_phase_order_contract_matches_runner_order`，照 payment-activation test_phases.py:1370 形态）
- Modify: `deploy/lago-lab/payment-settle-trigger/lab.sh`（Usage 路径/compose project 注释修正）
- Create: `docs/plans/issue-72-flow-evidence-82/_browser_lib.mjs`（共享：`login(page, ctx)`、`note/results`、`runLeg(fn)` 含 A-03 catch 与 RESULT/exitCode 收尾、A-12 订单身份锚点 helper、路由/超时常量）
- Create: `docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh`（薄包装消费共享约定：`FLOW82_EMAIL_PREFIX :?`、`FLOW82_R5_PW :?`、`DB_PATH :?`、token 脱敏落盘、sqlite3 参数绑定——以 r4-flow3 修复版为模板收敛）

**Interfaces:**
- Consumes: 顶层三 `.mjs` 既有 env 契约（`FLOW82_EMAIL*/PASSWORD*/WEB/EXPECT_CNY`）、`settle-evidence/prepare_82flow_webhook_secret.sh`（复验轮沿用其 R-11 secret 通道）、t11 lab 既有 `clients.LabError`/`_proxyless_opener`。
- Produces: Task 17 复验轮的脚本基座——`_browser_lib.mjs` 导出 `{ LOGIN, login, note, runLeg, assertOrderIdentity }`；`r5-verify/` 四腿脚本（`browser_01_checkout.mjs`…`browser_04_active_face.mjs`，Task 17 编写，消费本 Task 的共享库）。

- [ ] **Step 1: RED 冒烟**：
  - `cd docs/plans/issue-72-flow-evidence-82 && grep -n "issue82-Flow" browser_sync_face_82.mjs` → 命中 1 处（明文口令在案，证明待修）；修后同命令零命中。
  - `node browser_sync_face_82.mjs`（无 env）→ exit 2 + stderr 提示（env 契约不破坏）。
  - `cd deploy/lago-lab/payment-settle-trigger && python3 -m unittest test_phases -v` → 既有 26 用例 PASS；新增顺序契约用例首跑 FAIL（未实现）→ 实现 `run_lab.py` PHASE_SEQUENCE 由 PHASE_ORDER 派生后 PASS。
- [ ] **Step 2: 实现全部修改**（seed.sh 三份逐份同步同一组修复；冻结证据 JSON 不动）。
- [ ] **Step 3: 验证**：`bash -n` 三份 seed.sh 与两份 .sh（语法门）；`node --check` 不适用 .mjs（ESM）——以 Step 1 的执行面为准；`python3 -m py_compile` 三个 .py；`grep -rn "issue82-Flow\|sk_test_\|whsec_" docs/plans/issue-72-flow-evidence-82/ --include="*.mjs" --include="*.sh"` → 零命中。
- [ ] **Step 4: Commit** `git commit -m "issue-72(#82): (task16) evidence-script & t11-lab hygiene — credential/env/SQL redlines + shared browser lib (OCR r2)"`

---

### Task 17: 真实流程复验（R-19 浏览器 UI 面清偿）+ 回归 + 红线 + 收尾

**Files:**
- Create: `docs/plans/issue-72-flow-evidence-82/r5-verify/{browser_01_checkout.mjs, browser_02_sync_face.mjs, browser_03_paid_face.mjs, browser_04_active_face.mjs, seed.sh, README.md}` + 证据文件（截图/API JSON/`lago-four-objects-r5.txt`）
- Modify: `docs/plans/issue-72-flow-evidence-82/README.md`（复验轮记录）
- Modify: `docs/plans/issue-72-ledger-82.md`（第 4 轮收尾判定）

**Interfaces:**
- Consumes: Task 12-16 全部交付；「真实流程验证方案」章节的拓扑与判据；`_browser_lib.mjs`（Task 16）。
- Produces: 浏览器 UI 面端到端证据（R-19 清偿）+ Ledger 第 4 轮收尾判定。

- [ ] **Step 1: 按「真实流程验证方案」起链并执行四腿浏览器验证**（详见该章节执行序列；证据落 `r5-verify/`）。
- [ ] **Step 2: 四对象权威核验 + 重放面**（同章节步骤 7/9；`lago-four-objects-r5.txt`）。
- [ ] **Step 3: 回归**——`go test ./internal/modules/commercial/... ./internal/handler/ ./internal/router/ ./internal/container/ -count=1` 全 ok；`make check-backend-architecture && make verify-module-moves` OK；node26 下 web 全量 PASS、typecheck 错误数不增；`go vet -tags lago_integration ./internal/modules/commercial/commercialplatform/` clean（真实栈全量集成测试若 7 项 operator env 可得则实跑并记录，不可得则披露 skip≠pass）。
- [ ] **Step 4: 红线自查**（同章节通过判据 (e)）。
- [ ] **Step 5: 更新 README/Ledger，Commit** `git commit -m "issue-72(#82): (task17) r5 browser-flow re-verification — R-19 closure + final ledger"`

---

## 验收标准 → Task → 测试 追踪矩阵

| Issue 验收标准 | 状态（第 3 轮） | 本轮 Task | 测试/证据 |
|---|---|---|---|
| AC1 状态依次呈现待付款、已付款待激活、已生效 | API 面 PASS；浏览器面未实跑（R-19） | T15（canceled 补全+文案统一）、T17 | `purchaseStateMessage` canceled 用例；四腿浏览器截图（01/03/04）；`PURCHASE_STATE_LABEL` 两页一致 |
| AC2 同步返回页不能确认付款 | PASS（API+pin） | T17 复验 | `browser_02_sync_face.mjs`（同步面零推进截图）；既有 `TestSyncReturnCannotConfirmPurchase` 回归 |
| AC3 重复回调、Outbox 重放与响应丢失不重复履约 | PASS（API 面） | T12/T13（补强两真实扣款/重开风险面）、T17 复验 | `TestLagoSettleMultipleInvoiceCandidatesFailClosed`（跨发票零扣款）、`TestLagoSettleDoesNotTouchCustomerDefault`（续期错路由消除）、`TestPurchasePaidAwaitingWindowDoesNotOpenSecondOrder`（双重收款窗口关闭）、重放三面复验 |
| AC4 真实支付宝证据关联 Invoice/Payment/Subscription/Credits | PASS（stub 披露） | T17 复验 | `lago-four-objects-r5.txt`；沙箱残余 `ac4-sandbox-credentials-unavailable` 披露不变 |
| OCR r2 有效 findings 清偿（high 4 + medium ~14 产品代码 + security 级脚本） | — | T12-T16 | 各 Task Step 1 RED 用例清单（与 r2 报告行号一一对应，见各 Task Files 注记） |
| 评估后不修项 | — | D16 | Ledger 显式记录（R-17 语义保留/N+1 披露/冻结脚本/独立票） |

## OCR r2 关键 findings → Task 映射（防漏清偿）

| r2 位置 | 级别 | Task |
|---|---|---|
| lago_settlement.go:139（跨发票错扣）/ :118（空集绑定，折衷处置见 D16） | high / medium | T12 / D16 |
| lago_settlement.go:357（default 永久改写） | high | T12 |
| purchase.go:253（paid-awaiting 双重收款窗口） | high | T13 |
| browser_sync_face_82.mjs:35（明文口令） | high(安全) | T16 |
| CheckoutPage :197/:224/:150/:216/:103/:28 | medium×6 | T15 |
| lago_settlement.go:445 + lago.go:688（瞬态分类） | medium×2 | T12 |
| handler commercial.go:506（409）/:818（legacy plan 校验） | medium×2 | T13 |
| repository order.go:212（索引名）/:348（fallback） | medium×2 | T13 |
| purchase.go:306（竞态链） | medium | T13 |
| middleware/auth.go:71 → payment_callbacks body | medium | T14 |
| lago_purchase.go:624（排序显式比较；N+1 见 D16） | medium | T12 |
| fulfillment.go:341 / purchase_fulfillment.go:119 | low×2 | T14 |
| seed.sh ×3（token/DB_PATH/SCRIPT_DIR/LAGO_KEY/PAD/SQL 绑定）、stub/docstring、deliver/prepare、t11 lab | security/bug | T16 |

## Out of scope（#82 本轮明确不做）

- 微信渠道增量（#83 范围）；异常付款完整面与生命周期（#84：废弃 pending 单超时回收、cancel 命令、续期发票收款路由）；充值 Credits（#85）；升级/降级（#92-93）；settle 空集幂等分支的完整发票身份绑定（需 payload 冻结 invoice id，跨 #81 seam 面，随 #84 回议——D16）；readPurchaseInvoiceFees 读放大缓存（R-23 已披露）；typecheck:web 预存 5 错（独立票）；历史冻结证据脚本回改（D16/约束 13）。

---

## 真实流程验证方案（复验轮 r5；真实环境端到端，验证在本 worktree 构建+运行）

**前置（F20 实测：docker daemon 未运行）**：`docker info` 失败时先启动 OrbStack（`orb start` 或打开 OrbStack.app，等 daemon 就绪）。

**环境拓扑（全部真实运行；端口避开 :5272/:5273——其他会话保留）**

| 组件 | 地址 | 说明 |
|---|---|---|
| Lago v1.53.0 | 127.0.0.1:48889 / :48890 | 重建栈：`cd deploy/lago && ./lago.sh init && (追加 LAGO_CREATE_ORG=true + LAGO_ORG_USER_EMAIL/PASSWORD/NAME + LAGO_ORG_API_KEY seed) && ./lago.sh up`，compose project 独立命名（如 `weknora-lago-82r5`，端口沿用 48889/48890——本会话实测空闲；若冲突整体 +10）；健康 `./lago.sh status`；Stripe provider `weknora-stripe` 经 GraphQL 注册（真实 TEST key 仅 source `~/.zcode/issue72-stripe.env` 注入） |
| WeKnora 后端 | 127.0.0.1:8093 | 本 worktree 构建：`DB_DRIVER=sqlite DB_PATH=data/issue82-r5verify.db`（独立库）；`WEKNORA_COMMERCIAL_PLATFORM_*=lago→http://127.0.0.1:48889`；`WEKNORA_COMMERCIAL_STRIPE_API_KEY`（source 注入）/`WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN=pm_card_threeDSecure2Required`/`WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN=pm_card_visa`；`WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true`；`SSRF_WHITELIST_EXTRA=127.0.0.1`；`go run ./cmd/server`（首次编译数分钟） |
| WeKnora 前端 | localhost:5194 | `VITE_DEV_PROXY_TARGET=http://127.0.0.1:8093 pnpm --filter @weknora/web exec vite --port 5194` |
| 支付宝网关 stub | 127.0.0.1:8294 | `FLOW82_ALIPAY_PORT=8294 python3 docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py`（本地 RSA 密钥对，协议往返+验签证明，非沙箱证据——披露不变） |
| webhook 投递器 | — | `settle-evidence/deliver_stripe_webhook.py`（R-11 通道：`prepare_82flow_webhook_secret.sh` 铸造/落库 secret，事件体=Stripe API 读回的真实 PI 对象，签名=真实 secret 的真实 HMAC，接收与处理链 100% Lago 内建；D8 边界——替身仅此传输腿，生产形态由 Stripe 云端完成） |
| Playwright | headless chromium | 实测 1.63.0；`r5-verify/` 四腿脚本经 `_browser_lib.mjs` 驱动（A-03 catch + A-12 订单身份锚点 + RESULT 退出码） |

**种子数据**：sqlite 全新库（后端首启自迁移）；`r5-verify/seed.sh`（Task 16 修复版模板）注册主角 `settle-r5-a@verify.local`（浏览器主角）/`settle-r5-b@verify.local`（plan_publish grant）+ PAD 占位租户（正整数校验 + 期望序号断言）；Plan `weknora-pro-v1`（9900 分 CNY monthly 无 charges）。凭据全部 env 注入（`FLOW82_R5_PW` 等，无源码字面量）。

**执行序列（全部真实服务；截图/JSON 落 `r5-verify/`）**

1. **发布 Plan → Lago 落库**：b 租户 JWT `POST /api/v1/commercial/plans/draft` + `/publish` → 断言 200 + receipt；`curl :48889/api/v1/plans`（Bearer org key）含 `weknora-pro-v1` → `api-01/02-*.json`。
2. **浏览器下单 →「待付款」**（`browser_01_checkout.mjs`）：goto `http://localhost:5194/login`（env 凭据）→ `/platform/billing/checkout?plan=weknora-pro-v1` → 支付渠道单选确认「支付宝」为默认选中 → 提交（wire 断言 purchases 请求体 `provider=='alipay'`）→ 断言「等待付款」+「待付款（权益未开通）」+ 报价明细 ¥99.00（`FLOW82_EXPECT_CNY`）+ 订单号（A-12 锚点 note）→ `rv5-01-checkout-awaiting-payment.png`；`/platform/billing` 断言「待付款（权益未开放）」→ `rv5-02-billing-awaiting-payment.png`。
3. **Lago gated 形状核验**：`curl :48889/api/v1/subscriptions?external_id=weknora-tenant-N-purchase&status[]=incomplete` → incomplete + activation_rules → `lago-gated-before-settle.txt`。
4. **同步返回面（AC2）**（`browser_02_sync_face.mjs`）：以 `?order=<id>` 回访 checkout + 手动刷新订单（waitForResponse 确定性等待，非固定睡眠）→ 断言仍「待付款（权益未开通）」、全页无「已付款/已生效」→ `rv5-03-sync-return-no-confirmation.png`。
5. **可信回调 →「已付款待激活」**：`alipay_sandbox_notify.py` 造同源签名 notify（TRADE_SUCCESS 9900）→ `POST /api/v1/commercial/callbacks/alipay` → 200 `success`；GET `/api/v1/commercial/purchase` → `state=paid_awaiting_activation`；（竞态回归面：此刻再提交一次购买（新 quote）→ 断言返回既有 paid 订单视图、Lago 侧渠道订单不新增——D11 实证）→ `browser_03_paid_face.mjs` 断言「已付款，权益处理中」且无「已生效」+ A-12 锚点 → `rv5-04-paid-awaiting-activation.png`。
6. **settle 驱动（α 双轨道轨道二）**：outbox drain 自动驱动（后端 worker）→ 日志含 settle receipt；Stripe API 断言 gating PI `succeeded`（结算卡）且**无 customer default 改写调用残留**（Stripe 侧事件审计/时间线）→ `settle-run-r5.txt`。
7. **webhook finalize + 四对象（AC4）**：`deliver_stripe_webhook.py` 投递真实事件 → 轮询（≤120s）subscription `active`、invoice `finalized`+numbered+`payment_status=succeeded`（fee 金额按 D6' proration 判据：`0 < fee ≤ 订单 9900`，差额披露）、payments succeeded 恰 1、wallets 含 `<ext>-purchase-<period>` 批次 → `lago-four-objects-r5.txt`。
8. **「已生效」+ Credits 到账**：GET purchase → `state=active` + fees 非空；`browser_04_active_face.mjs` 断言 billing「已生效」+ 余额含购买批次（waitForSelector detached 确定性等待旧文案消失）→ `rv5-05-active-credits.png`。
9. **重复回调 + 重复 webhook + 重启重放（AC3）**：重发同一 notify → 200 success、outbox fulfill 事件计数不变；重投同一 webhook → 四对象 byte-identical；kill 后端再启 → drain 无第二笔 grant → `rv5-06-replay-idempotent.txt`。

**通过判据（全部满足才记 PASS）**

- (a) 步骤 2/5/8 三态文案浏览器断言全过（R-19 清偿；AC1）；
- (b) 步骤 4 同步面零推进（AC2）；
- (c) 步骤 9 三个重放面无第二次入账/发放/激活（AC3），DB 计数断言（orders paid==1、outbox fulfill==1、fulfillment_records purchase_activation==1、Lago payments succeeded==1）+ 步骤 5 竞态回归面（不开第二张渠道订单）；
- (d) 步骤 7 四对象齐全 + D6' 金额判据（AC4；`ac4-sandbox-credentials-unavailable` 如实披露）；
- (e) 红线自查：`grep -rE "sk_(test|live)_[A-Za-z0-9]{20,}" docs/plans/issue-72-flow-evidence-82/ internal/ deploy/` 零命中；`grep -rn "issue82-Flow" docs/plans/issue-72-flow-evidence-82/` 零命中（明文口令清偿验证）；本地 force-active 直写 grep 零命中；
- (f) Go 回归 10 包全 ok + 双架构门 OK；
- (g) 前端 node26 全量 PASS + typecheck 错误数不增（基线 5 预存）；
- (h) `go vet -tags lago_integration` clean；operator env 可得时 T9 集成测试真实栈 PASS（否则披露 skip≠pass）。

**失败处置**：任一判据不过 → 证据照存（失败也是证据）→ Ledger 记录 → 按 Task 归因；机制级失败（settle 后 Lago 不 finalize 等）→ `escalate`，不得以本地直写或伪造 webhook 兜底。

## 执行注意事项（并行环境）

- 本 worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees-issue72/issue-82`（分支 `codex/issue-72-lago-82`，已合并集成分支 `da4b544db`）；只在本 worktree 提交，提交信息前缀 `issue-72(#82): (taskNN)`。
- Task 顺序依赖：T12/T13/T14 相互独立可并行（包面不重叠：T12=commercialplatform、T13=repository+service+handler、T14=handler+service 两个文件——T13 与 T14 都改 `service/commercial/` 下文件但不同文件，可并行）；T15 独立；T16 独立；T17 依赖全部。
- 共享资源：Lago 栈用独立 compose project（R9 流程规则），绝不向共享 Lago 写复制数据；`commercialplatform` 全包测试 ~72-77s，定向 `-run` 先行、全包回归放各 Task 末步。
- 凭据：一切 `sk_*`/`whsec_*` 仅经 `~/.zcode/issue72-stripe.env` source 或 env 注入；`pm_card_visa`/`pm_card_threeDSecure2Required` 为 Stripe 公开测试 token 可入文档，`sk_*`/`whsec_*` 永不。
- node 版本：web 全量测试须 node ≥26（F23，本机默认 v22 会环境性失败——以 node26 口径跑并记录版本）。
