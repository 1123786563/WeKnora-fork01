# Issue #82 复验第 1 轮（修复后代码，2026-09-27）

- 验证者：流程验证员-82（复验轮 1）
- 分支：`codex/issue-72-lago` @ `ed779a2a8`（含修复 `c5a1917bb`：F-1 settle 幂等绑定 invoice 身份 + F-2 entitlements 改走 v1.53 subscriptions 路由；`ed779a2a8` Ledger R-20/21/22）
- 环境：后端 :8093 从集成分支 worktree **重新构建**（sqlite 新库 `data/issue82-r4v2.db`）；Lago 82flow 栈 :48889（healthy）；vite :5194；支付宝 stub :8294（新本地 RSA 密钥对）；主角 tenant 9（`settle-r4v2-a@verify.local`，浏览器）/ tenant 10（发布者），占位 1-8 吸收前轮 Lago 身份。订单 `ord_450538d2bbae2c55`（99.00 CNY 支付宝）。

## 上轮三项 failures 的修复判定

| 上轮缺陷 | 修复判定 | 证据 |
|---|---|---|
| F-1 settle 幂等键跨 PI 碰撞→窗口内误标 attention | ✅ **修复获证** | 全程 `activation landed attention` 日志 **0 条**（上轮 19 条）；Stripe 侧兄弟 PI `pi_3UJyoK`（requires_payment_method，同 invoice d4d69920）**未被驱动**（pm=null），settle 驱动的新 PI `pi_3UJyoT` succeeded 1650——`settle-run` 证据在 `lago-four-objects-after-settle.txt`；outbox 稳定 pending→sent、activation applied |
| F-2 readCustomerFeatures 路由不存在 | ⚠️ **修复不完整（F-2' 残留）** | 路由已改对（subscriptions 双腿，404 腿容忍），但**响应解析字段错**：代码读 `feature_code`/`feature.code`（lago.go readCustomerFeatures），pinned v1.53 实际形状是 `{"entitlements":[{"code":…}]}`（两腿 curl 实证）→ features 仍恒空 → account 的 features 面仍显示 base definition fallback（`advanced_models:false`），Lago 侧 entitlement 已物化但产品面不可见 |
| F-3 pnpm 全量失败（node v22） | ✅ **获证（node26 复跑）** | 本机 nvm node v26.4.0 下 `pnpm test:web` 全量 **exit=0**（~2305 测试，506s）；`pnpm typecheck:web` 残留 **5 个 tsc 错**（DevMarkdownPage.tsx:383、PlatformShell.tsx:1065/1066、mermaid.ts:127/158）——与修复员 Ledger R-22 判定一致（预存、最后改动 89e17fb6a、#82 无关面） |

## 本轮新发现缺陷（failures）

1. **[高] F-5 base 订阅的 finalized 发票使购买投影永久坍塌（正常用户路径必撞）**
   调 `GET /api/v1/commercial/account` 触发 lazy base onboarding（EnsureBenefits 建 `weknora-tenant-9-sub` 订阅 + 0 元月度发票 auto-finalize）后，customer 名下有 **2 张 finalized subscription invoice**；`readPurchaseInvoiceFees`（lago_purchase.go，按 `external_customer_id` 全查 finalized）fail-closed（>1 张 → `invalid_response`）→ `readPurchaseSnapshot` active 分支整体失败 → **`GET /purchase` 恒 `{"state":"absent","reason":"invalid_response"}`**（`f5-purchase-collapse.txt`）→ 前端三态数据源崩：billing 套餐行失去「已生效」后缀、checkout 的 purchaseStateMessage 回退显示「已付款，权益处理中」（与 orderMessage 行的「权益已生效」并存，见 06 截图）。上轮未触发只因从未调用 /account（无 base 订阅）；订单/权益事实不受影响（order fulfilled、Lago active、wallet 正常）。修复方向：finalized 定位谓词应按购买订阅（fees→subscription）过滤而非按 customer 全查。
2. **[中] F-2' readCustomerFeatures 解析字段与 pinned v1.53 实际形状不符（F-2 修复残留）**
   见上表。`{entitlements:[{code}]}` vs 代码的 `feature_code`/`feature.code`。一行解析修正即可收口。
3. **[中] F-4 购买 Credits 批次不在产品 benefits.credits 视图（上轮已存在、本轮新发现）**
   Lago 侧购买钱包 `weknora-tenant-9-purchase-2026-09` 9.9 credits granted 实证在账，但 `readBenefitsSnapshot` 的批次过滤器（lago.go:1042-1055）只认 `period` meta 或 `<ext>-<YYYY-MM>` 名——购买钱包的 meta 键是 `purchase_period`、名后缀 `purchase-2026-09`，被当作 foreign wallet `continue` → `GET /account` 的 credits.batches/balance 仅含 base 1.0 批次，**购买 9.9 批次不可见**。与 D4 钱包身份设计冲突。
4. **[低] 观察：首次 /account 调用返回 pending/unreachable**——lazy base onboarding 进行中（多条 Lago 命令）超时所致，第二次成功；非独立缺陷，但 onboarding 首调延迟值得留意。
5. **[低] 观察：T9 集成测试对栈内残留敏感**——同一栈连续跑失败运行后，下一次运行的固定租户身份读到残留（absent/unreachable 瞬态），干净重跑 PASS（`t9-integration-run.txt` 记录三次运行）。

## 全链断言结果（Issue #82 用户流程）

| # | 断言 | 结果 | 证据 |
|---|---|---|---|
| 1 | 发布 plan→Lago 落库 | ✅ | api-01/02/03（receipt publish_plan_version:pro:1；weknora-pro-v1 9900） |
| 2 | 浏览器选支付宝下单→扫码→待付款 | ✅ | browser_01 9/9、01/02 png、lago-gated-before-settle.txt（incomplete+activation_rules+open invoice 1650、PRECREATE 99.00 关联订单） |
| 3 | AC2 同步面不推进 | ✅ | browser_02 3/3、03 png |
| 4 | 可信回调→paid_awaiting_activation | ✅ | callback-01（200 success；state=paid_awaiting_activation）、browser_03 4/4、04/05 png |
| 5 | settle→Stripe 真实扣款→webhook→finalize→active | ✅ | webhook-01（200）、四对象：sub active / gating invoice finalized+numbered WEK-A27D-013-001+succeeded 1650 / payments succeeded 恰 1 / applied 记录 |
| 6 | 订单 fulfilled | ✅ | order API state=fulfilled、DB fulfilled=1、outbox sent、activation applied（全程 attention 0） |
| 7 | Billing/checkout「已生效」 | ❌ **被 F-5 破坏** | Lago/订单事实已生效，但 purchase 投影坍塌后 billing 行无「已生效」、checkout 残留回退中间态（06/07 png 留存失败现场；checkout orderMessage 行仍显示「权益已生效」） |
| 8 | Credits 到账 | ⚠️ Lago✅/产品面❌ | Lago 购买钱包 9.9 granted（恰 1 笔）；产品 benefits.credits 不含（F-4） |
| 9 | Entitlement 到账 | ⚠️ Lago✅/产品面❌ | subscriptions 路径实证 advanced_models；产品 features 面仍空（F-2'） |
| 10 | AC3 重复回调/重复 webhook/kill 重启 | ✅ | callback-02（计数不变）、webhook-02（IDENTICAL PASS）、06-replay-idempotent（attempt 4 稳定、applied 唯一、attention 0、终计数订单1/fulfilled1/outbox1/activation1/succeeded payments1/购买批次1） |
| 11 | (f) go 回归 9 包 + make×2 | ✅ | gotest-regression.txt（9 ok exit=0）、redline-checks.txt |
| 12 | (h) T9 真实栈 | ✅ | t9-integration-run.txt（干净重跑 3 子测试 PASS 26.7s） |
| 13 | (e) 红线 | ✅ | redline-checks.txt（凭据 0、force-active 0） |
| 14 | (g) 前端全量 | ✅(test)/⚠️(tsc) | node26 test:web exit=0 全量；typecheck 5 错预存（同修复员判定） |

## 结论

F-1/F-3 修复获证；F-2 修复不完整（F-2'）；新发现 F-4/F-5（F-5 高危：任何触发过 base onboarding 的租户（生产常态）购买后三态呈现崩坏）。**Issue 用户流程「订单与 Billing 页变已生效」在正常用户路径（访问过 account/billing）上不可达**——判 failed，待 F-2'/F-4/F-5 修复。

支付宝侧边界不变（本地 RSA stub，ac4-sandbox-credentials-unavailable）；webhook 传输腿替身按 D8。
