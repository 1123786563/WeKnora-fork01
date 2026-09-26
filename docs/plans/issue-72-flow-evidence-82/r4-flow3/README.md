# Issue #82 复验第 2 轮（修复后代码，2026-09-27）

- 验证者：流程验证员-82（复验轮 2）
- 分支：`codex/issue-72-lago` @ `d30c3e241`（含第 2 轮修复 `de4f5dd86`：F-5 finalized 定位按购买订阅 fees→subscription、F-2' 解析 v1.53 `{entitlements:[{code}]}` 形状+发布定义为值真值、F-4 购买批次进 credits 视图；Ledger R-23..R-26）
- 环境：后端 :8093 从集成分支 worktree **重新构建**（sqlite 新库 `data/issue82-r4v3.db`）；Lago 82flow 栈 :48889；vite :5194；支付宝 stub :8294（新本地 RSA 密钥对）；主角 tenant 11（`settle-r4v3-a@verify.local`，浏览器）/ tenant 12（发布者），占位 1-10 吸收前两轮身份。订单 `ord_53d9388fb2e4aa98`（99.00 CNY 支付宝，gating invoice `8a48722a`）。

## 上轮三项 failures 的修复判定（全部获证）

| 上轮缺陷 | 修复判定 | 证据 |
|---|---|---|
| F-5 base 发票致 purchase 投影坍塌 | ✅ **修复获证（两阶段）** | ① **base onboarding 后、finalize 前**：调 /account 建 base 订阅+0 元 finalized 发票（DB 实证 2 张 invoice：gating open + base finalized/succeeded）后，`GET /purchase` 仍 `paid_awaiting_activation`（上轮此点已塌 absent+invalid_response）；② **active 后**：webhook finalize → `GET /purchase` 稳定 `active`、D6' 复核过（activation applied）——fees→subscription 定位谓词在两阶段都活着 |
| F-2' entitlement 解析字段错 | ✅ **修复获证** | 购买 active 后 `GET /account` features=`{advanced_models:true, api_access:true, priority_support:false}`——advanced_models 由「购买 active OR 其 plan 定义」点亮（修复语义：发布定义为值真值、购买 active 时 OR pro 定义）；Lago 侧 entitlement 物化（purchase 腿 [advanced_models]）与产品面一致 |
| F-4 购买批次不在 credits 视图 | ✅ **修复获证** | `GET /account` credits：`balance_micro=10900000`（=base 1.0+购买 9.9）、batches 含 period 2026-09（同月双批按 period 合并展示）——与 Lago 真值（两钱包 1.0/9.9）一致；上轮仅见 1.0 |

（第 1 轮修复项 F-1/F-3 维持获证：本轮 settle 窗口 attention 全程 0、Stripe 兄弟 PI 未被驱动；node26 下 test:web 全量 exit=0。）

## 全链断言结果（Issue #82 用户流程，全部真实环境）

| # | 断言 | 结果 | 证据 |
|---|---|---|---|
| 1 | 发布 plan→Lago 落库 | ✅ | api-01/02/03（receipt publish_plan_version:pro:1；weknora-pro-v1 9900） |
| 2 | 浏览器选支付宝下单→扫码→待付款 | ✅ | browser_01 9/9、01/02 png、lago-gated-before-settle.txt（incomplete+activation_rules+gating invoice 8a48722a open 1650、PRECREATE 99.00 关联订单） |
| 3 | AC2 同步面不推进 | ✅ | browser_02 3/3、03 png |
| 4 | 可信回调→paid_awaiting_activation | ✅ | callback-01（200 success；state=paid_awaiting_activation）、browser_03 4/4、04/05 png |
| 5 | settle→Stripe 真实扣款（attention 全程 0、兄弟 PI 未驱动） | ✅ | pi_3UK0pF（兄弟）requires_payment_method 未驱动；pi_3UK0pL succeeded 1650（pm_card_visa clone）；backend-v3.log attention=0 |
| 6 | webhook→Lago 内建链 finalize→active | ✅ | webhook-01（200）；sub active、invoice WEK-A27D-014-002 finalized+succeeded+fee 1650、payments succeeded 恰 1 |
| 7 | **「已生效」浏览器面（F-5 修复后）** | ✅ | browser_04 3/3：checkout「权益已生效」无待付款/已付款残留、**billing「套餐base · 已生效」**（上轮被 F-5 破坏的呈现完整恢复）、06/07 png |
| 8 | Credits 到账（Lago+产品面） | ✅ | Lago 购买钱包 9.9 granted 恰 1 笔；产品 credits 10.9=1.0+9.9（F-4 修复） |
| 9 | Entitlement 到账（Lago+产品面） | ✅ | Lago purchase 腿 [advanced_models]；产品 features advanced_models:true（F-2' 修复） |
| 10 | 后台 Lago 四对象 | ✅ | lago-four-objects-after-settle.txt（sub active/invoice finalized+numbered+succeeded 1650/payments succeeded 1/双钱包+批次） |
| 11 | AC3 重复回调/重复 webhook/kill 重启 | ✅ | callback-02（计数不变）、webhook-02（IDENTICAL PASS）、06-replay-idempotent（attention 0、终计数 orders1/fulfilled1/outbox1/applied1/购买批次1/succeeded payments1） |
| 12 | (f) go 回归 9 包 + make×2 | ✅ | gotest-regression.txt（9 ok exit=0）、redline-checks.txt |
| 13 | (h) T9 真实栈 | ✅ | t9-integration-run.txt（首次即 PASS 50.9s 3 子测试） |
| 14 | (e) 红线 | ✅ | redline-checks.txt（凭据 0/force-active 0） |
| 15 | (g) 前端 | ✅ | node v26.4.0：test:web 全量 exit=0；typecheck 5 tsc 错为预存（同 R-22/R-26 判定，非 #82 面） |

## 观察项（非缺陷）

- webhook finalize 瞬间（try1）`GET /purchase` 一次 `absent+unreachable`（Lago 处理事件时 API 短忙），1-2 个轮询周期后稳定 active——瞬态自愈，非投影缺陷。
- 首次 /account 调用（base onboarding 进行中）两次即成功（上轮首次 pending/unreachable 后第二次成功；本轮两次均成功）。

## 结论

三轮复验的全部关键断言首次齐绿：**支付宝付款后恰好一次激活套餐的完整用户流程在真实环境端到端达成**——下单/扫码→同步面不推进→可信回调→已付款待激活→settle 真实扣款（幂等、零双扣、零 attention 误标）→webhook finalize→订单与 Billing 页「已生效」→Credits（Lago+产品面）与 Entitlement（Lago+产品面）到账→三重放面幂等（计数恰 1）→后台 Lago 四对象可查→回归与红线全过。typecheck 残留 5 错为 #82 无关预存面（历轮一致）。

披露不变：支付宝侧本地 RSA stub（ac4-sandbox-credentials-unavailable）；webhook 传输腿按 D8 替身。
