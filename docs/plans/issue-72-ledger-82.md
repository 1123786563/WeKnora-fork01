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

## 自检结论（writing-plans Self-Review，2026-09-26）

1. **Spec 覆盖**：L118-127 / L159-171 / 矩阵 #5 #7(部分) #20 #22 → 追踪矩阵逐 AC（AC1-AC4 + GC）有 Task 与测试归属；无缺口。
2. **Step 扫描**：全部 Task 均为 RED→GREEN→回归→Commit 步；无 TBD/TODO/占位（Task 1 Step 4 为显式升级条款，非占位）。
3. **类型一致性**：`SettlePurchasePaymentPayload`/`PurchaseStatePaidAwaitingActivation`/`PurchaseWalletName`/`InvoiceLineSnapshot{Kind,Name string; AmountFen int64}`/`NewPurchaseFulfiller` 在 T2/T3/T5/T7/T8 间引用一致（字段均经实读源码核对：purchase_command.go:106-130、subscription_command.go:98-153、platform.go:226-231、fulfillment.go:41-58/260/280）。
4. **Review Focus**：5 条输入类均有归属测试（over-payment/settle 迟到重放/sync-return/崩溃重放/canceled）。
5. **比例**：以签名、测试名、判据为主；实现体仅在算法非签名可决定处（D2' 五步）。

## 执行记录（随 Task 追加）

| Task | 提交 | 测试 | 判定 | 备注 |
|---|---|---|---|---|
| （计划） | 见 git log | 基线 4 包 ok | — | 本行由计划员填写，后续由执行者逐行追加 |
