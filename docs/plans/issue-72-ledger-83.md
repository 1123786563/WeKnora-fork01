# Issue #83 执行账本（[Lago 11] 微信付款复用同一激活流程）

## 计划身份

- 计划文件：`docs/plans/issue-72-plan-83.md`
- Issue：https://github.com/1123786563/WeKnora-fork01/issues/83（`[Lago 11] 微信付款复用同一激活流程`）
- 编写者：计划员-83（dynamic workflow subagent，2026-09-27）
- 执行方式：superpowers:subagent-driven-development（计划头部声明）

## 集成基线

- Worktree：`.worktrees-issue72/issue-83`，分支 `codex/issue-72-lago-83`
- 基线提交：`c6d027df8`（`issue-72: ocr issue-82 round 2`；集成分支 `codex/issue-72-lago`）
- 基线核实（编写时实读实跑）：
  - #82 激活链已在基线交付：`CommandKindSettlePurchasePayment`（`purchase_settlement_command.go:25`）、`PurchaseFulfiller.Fulfill`（`purchase_fulfillment.go:63`）、回调匿名放行（`middleware/auth.go:71`）、attempt 商户身份修复（`service/commercial/order.go:403-405`）。
  - `go test ./internal/modules/commercial/payment/ -run TestWechat -count=1` → `ok ... 2.103s`（基线绿，本会话实跑）。
  - Issue 调查简报中「激活链完全缺失/lago.go 不带 activation_rules」的描述对应旧基线，已由本计划「基线核实」节修正。

## 计划自检结果（编写时执行）

- 占位符扫描：计划正文无 TBD/TODO/占位内容（`grep -n "TBD\|TODO\|待定\|占位" docs/plans/issue-72-plan-83.md` → 仅 Out-of-scope 与披露边界的明确声明，无未决项）。
- 接口一致性：`CloseChannelOrder(ctx context.Context, tenantID uint64, orderID string) (OrderView, error)`、`MarkAttemptClosed(ctx context.Context, orderID string) error`、`PaymentAttemptStateClosed = "closed"` 在 Task 3 定义、purchase 接线与追踪矩阵消费处签名一致；`purchase.go` 的 `domain` 别名与 `s.orders` 字段名已对源码核实。
- 追踪矩阵：4 条 Issue 验收标准 + 「复用同一激活流程」核心目标均映射到 Task/测试/证据（计划「验收标准 → Task → 测试 追踪矩阵」节）。
- 路径核实：计划引用的全部代码路径/行号在本 worktree 实读（wechat.go:399/428/446/449、payment_callbacks.go:67-114、order.go:584 ConfirmPayment、purchase.go:253-261、routes.tsx:80 等）；环境命令对齐 #82 r4-flow3 验证口径。
- 端口：:8095/:5196/:8296/:8297，避开 :5272/:5273 及既往占用。

## 移交记录（执行者须写入收尾）

- 废弃 pending 单超时回收调度 + 公开 cancel 命令：#84/#92（`CloseChannelOrder` 为其可复用 seam）。
- `RecoverOrderStatus` 查单见 CLOSED 的本地收口呈现：#84（本票仅切轨路径收口）。
- partial/multiple-success 异常付款完整面：#84。
- 微信真实商户凭据不可得：本地 RSA stub 披露边界（README 如实标注，不伪造沙箱证据）。

## 执行记录（执行者续写）

| Task | Commit | 结果 |
|------|--------|------|
| Task 1 微信 Native 协议 pin | `9e11d7170` | `wechat_native_test.go` 8 测试（Create 签名头/请求体映射、本地拒绝、超时 StateUnknown 键回原单、Query 状态映射表驱动+回退、Close mchid 体、ORDER_PAID 错误形状）全 PASS；`go test ./internal/modules/commercial/payment/ -count=1` ok（全包 12.771s）。特征测试与既有实现零偏差，未改实现。 |
| Task 2 微信回调 HTTP 层 | `9131c32c7` | `payment_callbacks_test.go` 追加微信腿 8 测试：真实 `WechatProvider.Verify`（平台密钥经 `NewWechatProvider` 从 TempDir 加载、apiv3 key 文件注入）→ resolve → ConfirmPayment → outbox → `TestWechatCallbackDrainsToFulfilledWithFakePlatform` 组合级 pin（微信事实驱动同一 #82 fulfiller 到 fulfilled，purchase_activation applied 恰 1）。`go test ./internal/handler/ -run 'TestWechatCallback' -count=1` 8/8 PASS；两包回归 ok。 |
| Task 3 关单编排+切轨接线 | `ebf64a905` | RED（编译错误：CloseChannelOrder/MarkAttemptClosed/PaymentAttemptStateClosed 未定义）→ GREEN：repository `MarkAttemptClosed`（幂等参数绑定 UPDATE pending→closed）+ `OrderService.CloseChannelOrder`（竞态安全算法体：非 pending 零渠道调用直答；无 attempt 残留收口；Close 失败一律 Query 决胜——SUCCEEDED 走同一 ConfirmPayment/资金事实保留、CLOSED 同干净落账、其他未定不收口）+ purchase 重放分支换轨关单。`order_close_test.go` 9 测试全 PASS；`go test ./internal/modules/commercial/... ./internal/handler/ -count=1` 8 包 ok；`make check-backend-architecture` 0 violations。 |
| 修复（Task 4 过程中发现） | `6ef004e3a` | **WechatProvider.Query 丢 transaction_id**（真缺陷，见 Ruling R-83c）：`TestWechatQueryPrefersTransactionID` RED→GREEN；同时 purchase 接线对无 attempt 的 pending 单保持 #82 冻结重放语义（R-83d）。三包回归 ok。 |
| Task 4 真实受控环境验收 | （本提交） | 五幕全过：幕一浏览器主链 12/12（真实浏览器面演练切轨：默认支付宝单 → 显式「改用微信支付重新发起支付」→ 关旧开新 → weixin:// 待付款 → 签名回调 200 SUCCESS → paid_awaiting_activation → settle(真 Stripe PI)+webhook(D8 替身) → active「权益已生效」双页面、无残留）；幕二漏通知查单恢复 4/4；幕三重复通知幂等 4/4（fulfilled/fulfill/attempt/Lago payments 各恰 1）；幕四关单竞态 6/6（ORDER_PAID→Query 决胜答已付旧单/支付宝零调用；干净切轨 204/旧单退役/新单唯一可付；已关单晚到成功无二次履约）；幕五四对象+产品面 6/6（subscription active、invoice finalized+succeeded fee 恰 1（1320=4/30 剪裁面，82 已披露形状）、succeeded payment 恰 1、purchase 钱包 9.9 恰 1、credits 10.9、advanced_models:true）。红线：商业域+handler 8 包 ok、architectureguard 0 violations、`sk_test_` 0 命中、本地强制 active 0 命中。证据包 `docs/plans/issue-72-flow-evidence-83/`（5 截图+5 txt+JSON+脚本+README）。 |

### 测试命令与结果（Task 4 实跑收口）

- `go test ./internal/modules/commercial/payment/ -run 'TestWechat(Create|Query|Close)' -count=1 -v` → 全 PASS（含修复新增 `TestWechatQueryPrefersTransactionID`）
- `go test ./internal/handler/ -run 'TestWechatCallback' -count=1 -v` → 8/8 PASS
- `go test ./internal/modules/commercial/service/commercial/ -run 'TestCloseChannelOrder|TestPurchaseSwitch|TestLateSuccessAfterClose' -count=1 -v` → 9/9 PASS
- `go test ./internal/modules/commercial/... ./internal/handler/ -count=1` → 8 包全 ok（exit=0）
- `make check-backend-architecture` → `architectureguard: OK (0 violations)`
- `grep -rn "sk_test_" docs/plans/issue-72-flow-evidence-83/ internal/ | wc -l` → 0；`grep -rniE "force.*active|直接.*(写|置).*active" internal/modules/commercial/commercialplatform/*.go | grep -v _test | wc -l` → 0
- 真实环境：`node browser_flow_83.mjs` 12/12 PASS（exit 0）；`node api_recovery_83.mjs` 26/26 PASS（exit 0）

## Ruling（执行者裁决）

- **R-83a（Lago 身份隔离）**：82flow 共享栈的 Lago 租户号已被外部验证轮占到 `weknora-tenant-15`（实测 DB subscriptions），首版 seed（主角落 tenant 1/15）会撞旧轮身份（`purchase_not_awaiting`）。裁决：占位注册 14 个吸收已占号 + 幕一换新租户（最终 tenant 20）一次跑通；整脚本重跑会在已购租户上叠加新单（幂等键是 quote，非租户），验证脚本改为每轮唯一 email。错误代价：占位数若再不足需加大重 seed。
- **R-83b（webhook secret 获取）**：82flow 栈的 `deploy/lago/.env` 已失，provider secret 在 DB 为密文。裁决：经容器内 rails runner **只读**导出 `webhook_secret`（不修改共享栈任何状态，值只进 shell 环境变量，不落仓库不进证据）。代价：无（只读）；若未来栈重建，按 t11 lab.env 路径取。
- **R-83c（Query transaction_id 缺陷修复）**：真实环境实证微信回调答 500——根因是 `WechatProvider.Query` 只解析 `out_trade_no` 丢 `transaction_id`：漏通知查单恢复把 attempt 的 `provider_transaction_id` 记成 out_trade_no，晚到的真实回调（携带真 transaction_id）在 ConfirmPayment 的 sameTxn 幂等判断上失配——同一笔付款被当成两笔（假 over-payment 审计）或撞 `(provider, merchant, provider_transaction_id)` 唯一约束直接 500。裁决：属 #83 微信渠道语义内的实现缺陷（spec L127「同笔付款的外部事实归一」），按 TDD 修复——AC1/AC2 的真实回调链路依赖它，不修则验收范围被砍。代价：若微信某场景 Query 不回 transaction_id，修复保留了回退链（transaction_id→out_trade_no→请求 id）。
- **R-83d（无 attempt 的 pending 单重放语义）**：Task 3 接线初版在重放分支 `FirstPendingAttempt` 失败即报错，撞 #82 既有测试 `TestConcurrentFreshQuoteConflictReplaysWinner`（裸订单行无 attempt 的形状 pin「仍答旧单」）。裁决：attempt 缺失时保持 #82 冻结语义（原子 open 永远同时写订单+attempt，该形状只可能是异常残留，交既有恢复机制）；仅 attempt 存在且渠道不同才走关单切轨。代价：渠道未知的异常残留单需 #84 生命周期票收口。
- **R-83e（计划 run 正则笔误）**：计划 Task 3 Step 1/3 的 `-run 'TestCloseChannelOrder|TestPurchaseSwitch'` 不含 `TestLateSuccessAfterCloseAuditsWithoutSecondFulfillment`（Review Focus #5 的 owning test）。裁决：补跑该测试（PASS），命令记入本账本。代价：无。
- **R-83f（幕三计数断言语义）**：真实主链租户合法持有两行订单（切轨被关的支付宝 pending 单 + 已 fulfilled 微信单）——计划「commercial_orders 1」按字面会假 FAIL。裁决：exactly-once 断言落在 fulfilled/outbox-fulfill/succeeded-attempt 各恰 1，并在证据中如实记录两行形状。代价：无（语义更强）。

## 移交确认（收尾）

- `CloseChannelOrder` 为 #84/#92 生命周期票可复用 seam（超时关单调度/公开 cancel 命令不在本票范围）。
- `RecoverOrderStatus` 查单见 CLOSED 的渠道自动关单 UX 收口：#84。
- partial/multiple-success 异常付款完整处置面：#84。
- 微信真实商户凭据不可得：本地 RSA stub 披露边界（README 已如实标注，不伪造沙箱证据）。
