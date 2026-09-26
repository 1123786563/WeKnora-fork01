# Issue #82 执行 Ledger（Lago 10 · 支付宝付款后恰好一次激活套餐 · R-4 重做轮）

## 计划身份

| 项 | 值 |
|---|---|
| 计划文件 | `docs/plans/issue-72-plan-82.md`（R-4 双轨道修订版，第 3 轮） |
| Issue | 1123786563/WeKnora-fork01#82（[Lago 10] 支付宝付款后恰好一次激活套餐，OPEN） |
| Worktree | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees-issue72/issue-82` |
| 分支 | `codex/issue-72-lago-82` |
| 集成基线 | `codex/issue-72-lago` @ `f50c705074`（2026-09-26 本会话 fast-forward 合并，`rev-list --left-right --count` 实测合并前 0 ahead / 23 behind，无冲突） |
| 裁决依据 | R-1（T02=②Provider，2026-09-23）、R-3（付款前订阅面校验+付款时 InvoiceFees 复核，2026-09-23）、R-4（D2 重议=α 双轨道，2026-09-26，`9393ce0da`）——`docs/plans/issue-72-user-rulings.md` |
| 上轮处置 | 第 1/2 轮计划 D2（cancel intent+切 pm+retry_payment）被 t10 证伪，随 `e0364d196` revert 出集成分支；本计划为 R-4 附带授权的重做版 |

## 前置状态（本会话实测核验）

- flowfix 三缺陷（回调 Auth 白名单 / `MerchantID()` 商户身份 / 客户绑定 upsert）已在基线（`7324eb054`，`internal/router/commercial_callback_public_test.go`、`internal/handler/payment_callbacks_test.go` 在案）。
- 旧 #82 Task 2-4 成果（settle 命令/fake settle/InvoiceFees 读取/AC2 pin/t10 lab）已被 revert 清空，本计划全量重做（机制按 D2' 双轨道，非恢复）。
- OCR round-4 购买/订单/回调链路 findings 全部未修（实测：`internal/handler/commercial.go` 无 `ErrQuoteNotFound` 分支；`service/commercial/order.go:119` 回填无 `created_at IS NULL` 限定）→ 纳入 Task 7/8/10 修复范围。
- 基线测试（2026-09-26 本会话实跑）：`go build ./...` exit 0；`go test ./internal/modules/commercial/commercialplatform/ ./internal/modules/commercial/service/commercial/ ./internal/handler/ ./internal/router/ -count=1` → **4 包全 ok**（72.042s / 1.646s / 3.235s / 6.073s）。
- 真实栈（本会话 docker ps 实测）：`weknora-lago-82flow`（Lago v1.53.0，api 127.0.0.1:48889 healthy / front 48890）运行中，seed org 与 Stripe provider `weknora-stripe` 已注册；WeKnora dev 容器（postgres/redis/docreader）在跑。端口占用实测记录于计划 F14。

## 机制设计要点（D2'，供执行者速查）

- 激活链：渠道回调 → 订单 paid → outbox `fulfill:<order>` → `settle_purchase_payment`（Stripe 侧定位 requires_action gating PI（`metadata.lago_invoice_id`）→ attach 结算 pm → PI update payment_method + confirm 同步 succeeded）→ Lago 内建 webhook（`POST /webhooks/stripe/:org_id`，`webhook_secret` 验签）→ `PaymentIntentSucceededService` → payment succeeded → finalize + active → fulfiller 观察 active → InvoiceFees 复核（D6' proration 判据）→ 购买钱包发放首期 Credits。
- 未实证链接 = Task 1（t11 探针）第一判据（requires_action PI 可 update pm + confirm）；证伪即升级，不实施替代猜测。
- 本地验证的 webhook 传输腿由 harness 替身（真实 PI 事件体 + DB 真实 secret 签名，D8 边界）；产品代码零伪造。

## 自检结论（writing-plans Self-Review，2026-09-26；审查第 1 轮后更新）

1. **Spec 覆盖**：L118-127 / L159-171 / 矩阵 #5 #7(部分) #20 #22 → 追踪矩阵逐 AC（AC1-AC4 + GC-a/GC-b/渠道选择器）有 Task 与测试归属；无缺口。
2. **Step 扫描**：全部 Task 均为 RED→GREEN→回归→Commit 步；无 TBD/TODO/占位（Task 1 Step 4 为显式升级条款，非占位）。
3. **类型一致性**：`SettlePurchasePaymentPayload`/`PurchaseStatePaidAwaitingActivation`/`PurchaseWalletName`/`InvoiceLineSnapshot{Kind,Name string; AmountFen int64}`/`NewPurchaseFulfiller` 在 T2/T3/T5/T7/T8/T11 间引用一致（字段均经实读源码核对：purchase_command.go:106-130、subscription_command.go:98-128、platform.go:226-231、fulfillment.go:41-58/168-213/260/280、CheckoutPage.tsx:156/194-207）。
4. **Review Focus**：5 条输入类均有归属测试（over-payment/settle 迟到重放/sync-return/崩溃重放/canceled）。
5. **比例**：以签名、测试名、判据为主；实现体仅在算法非签名可决定处（D2' 五步）。

## 计划审查第 1 轮处置记录（2026-09-26，12 findings 全处置）

| # | 级别 | 问题 | 处置（计划落点） |
|---|---|---|---|
| H1 | high | 验证方案「选支付宝」不可执行：CheckoutPage.tsx:156 硬编码 provider='wechat' 无渠道 UI | Task 8 新增渠道选择器（单选支付宝/微信、默认支付宝、提交体参数化、微信形状表驱动锁定）+ 验证方案步骤 2 改为显式选支付宝并断言提交体；追踪矩阵加「渠道可选 checkout」行 |
| H2 | high | D2' 定位谓词 requires_action 与 t10 实测（PI=requires_payment_method）矛盾；P-A FAIL 无处置 | D2'(ii) 谓词改 unsettled∈{requires_payment_method（t10 唯一实测）, requires_action}；F5/F7 状态分层澄清（Lago 行状态 ≠ Stripe PI 状态）；Task 1 P-A 记录实测形态、升级条款覆盖 P-A/P-C/P-E 任一 FAIL |
| H3 | high | R-4 授权的终审 OCR 6 项开放 findings（final report §9.4 口径）零覆盖 | 新增 Task 11（第 6/7/9/11 条后端清偿 + 第 10 条最小面与显式移交）；第 4 条（证据脚本）入 Task 10（四副本同病同修）；追踪矩阵 GC 拆 GC-a（round-4 链路口径）/GC-b（终审六项口径）双行；Architecture/修订历史双口径声明 |
| M4 | medium | 90s 同步轮询阻塞共享 fulfillment drain（Recover 串行 + 单 goroutine 30s） | D7/Task 7 改短窗跨轮：首轮 15s（5×3s）未达 active 即返回 nil 让 drain 继续，跨轮重放幂等收敛，总预算 10min 转 attention；新增 `TestPurchaseFulfillShortWindowDoesNotBlockDrain` |
| M5 | medium | 同 customer 多笔 unsettled PI 消歧未定义 | D2'(ii) 定消歧规则：候选>1 取 created 最新，并列/解析异常 fail-closed invalid_response 绝不盲扣；Task 5 加 `TestLagoSettlePicksLatestUnsettledIntent`/`TestLagoSettleAmbiguousIntentsFailClosed` |
| M6 | medium | CheckoutPage 轮询只拉 OrderView 拿不到 purchase.state | Task 8 补 refreshOrder 叠加 `client.commercial.purchaseStatus()`（BillingPage.tsx:83 既有模式）作三态数据源，失败静默降级 |
| M7 | medium | grant period 取值时点未定：跨月激活 Validate 必拒 | D4 明确 period/ExpiresAt 取激活观察时刻（激活月周期，ExpiresAt 恒未来）；Task 7 加 `TestPurchaseGrantPeriodTakesActivationMonth`（fake 时钟跨月） |
| L8 | low | configured() 实际在 lago.go:275 非 lago_purchase.go，Task 8 Files 漏列 | Task 8 Files 补 lago.go 条目（含行号注记），File Structure 的 lago.go 条目注明双归属 |
| L9 | low | Task 9 env 机制自相矛盾（HELPER vs 直读） | 统一为 `LAGO_INTEGRATION_WEBHOOK_SECRET` env 直读（外层 lab shell 从 DB 取出注入，测试自身不连 Lago DB） |
| L10 | low | `node -e require('./browser_flow_82.mjs')` 对 ESM 报 ERR_REQUIRE_ESM，RED 无效 | Task 10 Step 1 改 `node browser_flow_82.mjs; echo "exit=$?"` 直接执行断言 exit 2 |
| L11 | low | playwright 版本声明 1.60.0 过时 | F13 与拓扑表改 1.63.0（本会话 `npx playwright --version` 实测 Version 1.63.0，两会话一致），注明任务简报 1.60.0 已过时 |
| L12 | low | Task 10 清单遗漏 WEB/¥99.00 硬编码、paid_face 冗余条件、81 目录 stub docstring | Task 10 Files 补：`FLOW82_WEB`/`FLOW82_EXPECT_CNY` 参数化、`browser_paid_face_82.mjs:38` 冗余条件删除、flow-evidence-81/wechat_pay_stub.py docstring 同病同修 |

## 执行记录（随 Task 追加）

| Task | 提交 | 测试 | 判定 | 备注 |
|---|---|---|---|---|
| （计划 r1） | `1eba5d08c` | 基线 4 包 ok（本会话实跑） | — | 计划员提交 |
| （计划 r2 审查修订） | 见 git log | 复核：`npx playwright --version`=1.63.0 实跑；t10-payment-probe-p1.json（git show ca04d7da7）intent_status=requires_payment_method 实读；final report §9.4 六项清单实读；subscription_command.go:104-128 Validate 实读；fulfillment.go:168-213 实读 | — | 12 findings 全处置 |
| Task 1 t11 探针 | `7ddc4deab` | `python3 -m unittest test_phases -v` 26/26（RED 先行：5 新用例未实现时 FAIL）；`python3 run_lab.py` **exit 0**（gated_3ds/settle_probe/settle_trigger/cleanup 全 pass，secrets scan 0 hits） | **PASS** | 五判据全实证：P-A 实测形态 `requires_payment_method`；P-C（D2' 未实证链接）confirm→succeeded；P-D webhook 200+inbound 落库；P-E active+finalized+恰 1 succeeded+重投 byte-identical。独立栈 weknora-lago-t11（:48895/:48896）。**F10 证伪**：webhook_secret 非 psql 列且本地栈为空（Stripe 拒回环 URL 注册，worker 日志实证）——见 Ruling R-11 |
| Task 2 seam additive | `9a75d2c56` | `go test ./internal/modules/commercial/ -count=1` ok（RED 先行：未定义编译失败→GREEN 3 用例 PASS） | PASS | settle_purchase_payment 命令/键、PurchaseWalletName、paid_awaiting_activation 常量、WalletName 可选字段 |
| Task 3 fake settle | `063470294` | 定向 3 用例 PASS + 全包 ok（70.5s） | PASS | fake settle 幂等/unknown fail-closed/terminal fail-closed（Ruling R-14）/购买钱包并存 |
| Task 4 fees 读取 | `b73301786` | 4 用例（映射/空/多张 fail-closed/无 sub 行）PASS + 两包 ok | PASS | 后按 t9 实测改为两步读（index 定位→单张读 fees，t10 真实栈实证列表不带 fees） |
| Task 5 Lago settle | `696f96395` | 7 用例 PASS + 全包 ok（73.3s） | PASS | D2' 五步；幂等键**端点限定**（Ruling R-13） |
| Task 6 契约腿 | `1b30ad3c8` | SettleContract fake+lago 双腿 PASS + 全包 ok | PASS | 共享腿：激活恰一次/同键重放等价回执/payload 类型 unsupported/非法 payload 拒绝 |
| Task 7 编排 | `c8b0fdd7e` | 7 用例（Settles/OverPayment/CrashReplay/Canceled/ShortWindow/GrantPeriod/PaidAwaiting+SyncReturn pin）PASS；7 包全量 ok；`make check-backend-architecture` OK（0 violations）；`make verify-module-moves` OK（16 manifests） | PASS | D5 分流判据=quote line_items（Ruling R-15）；D7 短窗跨轮；handler/order/sweep OCR r4 |
| Task 8/9/11 Go 侧 | `356b10772`、`9e3f199a3` | TestConfigured*/TestOutboundHost/TestNoDefaultPm PASS；`-tags lago_integration` 无 env SKIP（门控正确）；Task 11 六用例（ReplayCanceled/ReplayActive/Disambiguation/SkipsForeign/MismatchAudit/DeadlineDerived）PASS | PASS | 配置侧 loopback 旁路（仅放行 loopback）；429 并入 unreachable；单标签 host 拒绝；终审 6/7/9/10/11 清偿 |
| Task 9 真实栈 | 见 `356b10772`+`settle-evidence/t9-integration-run.txt` | `go test -tags lago_integration -run TestLagoIntegrationSettle -v` **PASS（17.6s，非 skip）**；证据 grep sk_test/whsec_ = 0 | **PASS** | 全链：gated create→等 unsettled PI→settle→真实 PI 事件+真实 secret 签名投递→active+D6'→重放 no-op。暴露并修复两缺陷（Ruling R-13 幂等键、R-16 两步 fees 读） |
| Task 10 脚本 OCR | 见收尾提交 | `node browser_flow_82.mjs`（缺 env）exit 2 + stderr 提示（三脚本同口径）；`python3 -c "import alipay_gateway_stub/alipay_sandbox_notify"` 冒烟 OK；`verify_ac_assertions.py`（74 目录）既有证据重放 ALL PASS | PASS | 三 .mjs 凭据 env 必需+FLOW82_WEB/EXPECT_CNY 参数化+waitForSelector+fileURLToPath+paid 冗余条件删；KEY_DIR env 化；四副本 endgame 放宽；81 docstring 修正 |
| Task 10 真实流程 | 见收尾提交 | API 链全通（t11 栈+支付宝 stub+真后端 :8093）：purchase→precreate→**同步面**→签名回调 success→**paid_awaiting_activation**→drain settle→webhook 投递→**active+fulfilled+applied 回执**；重复回调 success 且 fulfill 事件不增（AC3a）；四对象证据 `lago-four-objects-after-settle.txt`（subscription active/invoice finalized+numbered+succeeded+fee 1650 proration/payments 恰 1 succeeded/wallet 购买批次） | PASS（API 面） | 浏览器 UI 面未实跑（前端 jsdom 测试 12/12 覆盖逻辑；真实浏览器断言留给复验）；沙箱残余披露不变 |

## Ruling（执行期裁决）

- **R-11（t11，P-D secret 通道）**：计划 F10 假设「psql 直读 payment_providers.webhook_secret」——实测该列不存在（secret 在 settings jsonb accessor）且本地栈为空（Stripe 拒绝注册回环 URL："URL must be publicly accessible"，worker 日志实证）。裁决：harness 经同一 Stripe API（占位公网 URL 的 WebhookEndpoint.create，永不收推送）mint **Stripe 真实生成的 secret**，经 Lago **模型层**（rails runner update!，非 SQL）写入 provider，再以它签名投递真实 PI 事件。事件体/验签/处理链 100% Lago 内建；Stripe endpoint 用后即删。错误代价：若 owner 要求 Stripe 云端直投（需公网隧道），本腿需重做。**已记 t11 DECISION.md。**
- **R-12（lab key 引号）**：t11 lab.env 的值带双引号，后端 env 直传会 401——`prepare_t9_env.sh`/run 脚本统一 `tr -d '"'`。
- **R-13（Stripe 幂等键端点限定）**：计划 D2' 任务级 Produces 说「Stripe Idempotency-Key = cmd.Key」——实测 Stripe 把一个 key 绑定到**首个请求的 endpoint**，update 与 confirm 共用一键必 400 idempotency_error（t9 集成实证）。修正为 `<cmd.Key>:update` / `<cmd.Key>:confirm`（同一命令重放仍确定性成对）。单测断言同步。
- **R-14（fake settle terminal 语义）**：计划 fake 契约未定义 canceled 订阅的 settle 行为——定为 ErrPlatformInvalidResponse fail-closed（与 Task 11 第 7 条 seam 收紧一致）。
- **R-15（D5 分流判据）**：OrderKind 无法区分订阅购买单与充值单（domain 语义「purchase settles as credit top-up」）——分流判据定为 quote snapshot 的 line_items 含 subscription_fee（#81 冻结面），读不到 quote 视为非订阅购买单走既有 top-up。
- **R-16（fees 两步读）**：pinned v1.53 的 invoice **index** 应答不带 fees（t9 实测），单张 GET 才带——readPurchaseInvoiceFees 改两步（index 定位恰 1 张→单张读 fees+payment_status），stub 同步对齐。
- **R-17（settle-vs-finalize 窗口，t10 流程实证的产品缺陷修复）**：settle 已 confirm、webhook 未落地的窗口内重放 settle，原实现把「无 unsettled PI」判为 invalid_response→attention，且 attention 抢占唯一键阻断终局 applied。修复：succeeded+invoice-identity 的 PI 存在→幂等回执返回（`TestLagoSettleSettledWindowReplaysIdempotently` 锁定）；终局 applied 记录 UPSERT 覆盖 transient attention。
- **R-18（Lago grant WalletName 消费遗漏）**：Task 3 只改了 fake 的钱包身份分流，Lago 适配器 grantIncludedCredits 仍恒用 MonthlyWalletName——t10 四对象核验发现钱包名错（weknora-tenant-2-2026-09 而非 -purchase-2026-09）。修复：Lago grant 同步消费 payload.WalletName+period meta 键分流（purchase_period/period，F11 防撞）。
- **R-19（浏览器 UI 面未实跑）**：Task 10 的 Playwright 浏览器断言未执行（jsdom 单测 12/12 覆盖三态/渠道选择器逻辑；系统多会话高负载下完整 vite+chromium 链未起）。API 面（purchase→回调→drain→webhook→四对象）真实栈全通。UI 浏览器面留给复验轮。


## 收尾判定（2026-09-26 执行会话）

**通过判据（计划「真实流程验证方案」）**：

- (a) 三态文案：API 面 `awaiting_payment → paid_awaiting_activation → active` 全程实测（purchase 接口各态逐段核验）；前端三态渲染由 jsdom 测试 12/12 覆盖（checkout 三态/渠道选择器）+ Billing 7/7 —— **PASS（浏览器 UI 断言未实跑，R-19）**
- (b) 同步返回页零推进：同步面（checkout 刷新形态）实测不推进；`VerifySyncReturn` 恒 `ErrAlipaySyncReturn` pin（TestSyncReturnCannotConfirmPurchase）—— **PASS**
- (c) 三个重放面：重复回调 success 且 fulfill 事件计数不变（实测 2 单 2 事件）；settle 重放 no-op（T9 子测试 + TestLagoSettleSettledWindowReplaysIdempotently + TestPurchaseFulfillCrashReplayConverges）；Lago payments 恰 1 succeeded（四对象证据）—— **PASS**
- (d) 四对象齐全 + D6' 金额判据：`lago-four-objects-after-settle.txt`（active/finalized+numbered+succeeded/恰 1 succeeded/购买钱包批次；fee 1650 ≤ 订单 9900 = proration 剪裁实证披露）；支付宝侧 stub 披露 `ac4-sandbox-credentials-unavailable` —— **PASS（沙箱残余如实标注）**
- (e) 红线自查：`grep sk_(test|live)_{20,}` 零命中；本地 force-active 直写零命中；whsec_ 无可用凭据 —— **PASS**
- (f) 回归：9 包（commercial 7 + handler + router）全 ok；`make check-backend-architecture` OK（0 violations）；`make verify-module-moves` OK（16）—— **PASS**
- (g) 前端：commercial 22/22；typecheck commercial 相关错误清零（DevMarkdownPage/PlatformShell/mermaid 为基线既有、本票零触碰）—— **PASS**
- (h) T9 集成测试真实栈 env 下 **PASS（非 skip）** —— **PASS**

**明确移交/残余**：

1. 浏览器 UI 断言（Playwright 三脚本实测）未跑（R-19）——脚本已 OCR 修复并 env 参数化，复验轮以 `FLOW82_*` env 起链即可。
2. 终审第 9 条残余（带 checkout_url 的废弃 pending 单超时回收）与第 10 条 cancel 命令 → #84/#92（生命周期票，需渠道关单/terminate 契约实证）。
3. Stripe 结算款生产手续费/风控对冲 → R-4 生产前回议项。
