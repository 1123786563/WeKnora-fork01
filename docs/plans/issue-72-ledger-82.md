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
