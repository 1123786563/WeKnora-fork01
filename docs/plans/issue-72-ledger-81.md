# Issue #81 执行 Ledger — [Lago 09] 从 Quote 创建待付款 Invoice 和 incomplete Subscription

## 计划身份

| 项 | 值 |
|---|---|
| Issue | https://github.com/1123786563/WeKnora-fork01/issues/81（OPEN，父 Issue #72） |
| 计划文件 | `docs/plans/issue-72-plan-81.md`（本 worktree） |
| Worktree / 分支 | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n81` / `codex/issue-72-lago-81` |
| 集成基线 | `f6969fc008`（issue-72: issues inventory and dag；本 worktree HEAD 实测一致） |
| 编写会话 | 2026-09-23，计划员-81（writing-plans 技能） |
| 依赖裁决 | T02 三选一 = 选项 (b) 受支持 Provider 轨道（用户裁决 2026-09-23；docs/migrations/lago/t02-payment-activation/DECISION.md:149-154 裁决记录） |

## 前置 Issue 接口消费（计划 Consumes）

- #78（CLOSED）：`ensure_customer` 命令 + `ExternalCustomerID(tenantID)`（internal/modules/commercial/platform.go:35,159）。
- #79（CLOSED）：`publish_plan_version` + `DeterministicPlanCode`（internal/modules/commercial/plan_command.go:28,161）；`PlanVersionService`/`PublicationRow`（repository/commercial/planversion.go:30-40）。
- #80（CLOSED）：`ensure_subscription`（Base Plan 标准订阅，无 activation_rules——subscription_command.go:31-35）、幂等 identity-read 算法先例（lago.go:587-637）、`BenefitsService` 惰性链。
- #74（t02 lab）：v1.53.0 payment-gated 订阅契约证据（evidence/ 九份 JSON 全 pass）。

## 计划编写期新增实证（已写入计划「实证契约基线」F1-F10，全部本会话执行）

- F1（实测 422）：gated 订阅创建要求 customer 已绑 payment provider（`no_linked_payment_provider`）。
- F2（实测 422）：org 未注册 provider 时绑定失败（`payment_provider_not_found`）；注册仅 GraphQL + operator JWT。
- F3/F4/F5（源码级，运行容器 /app 内 invoice.rb:100-101、invoices_query.rb:122-129、invoices_controller.rb:46-48、invoice_resolver.rb）：open gating Invoice 对全部 API 途径不可见（显式 status 过滤与 visible 集合求交集）→「不重复 Invoice」权威核验须 DB 直查；付款前匹配校验数据源改用权威订阅面（设计决策 D2）。
- F6/F7/F8/F9/F10：t02 lab 证据 + DECISION §7（订阅 index 显式 status[]、重 POST 延迟 terminate 风险、timeout_hours:0 语义、gating invoice 生命周期、entitlement hash 形状）。

## 计划自检结果（writing-plans 技能 Self-Review 四项；2026-09-23 计划审查第 1 轮后更新）

1. **Spec coverage**：#81 四条验收标准 ↔ Task 1-12 映射齐备（计划内「验收标准 → Task → 测试追踪矩阵」）；spec L121 匹配前置于渠道订单（Task 7）、L122 activation rule（Task 3）、L169 awaiting payment 状态（Task 1/4/7/8）、L170 闭合产品词汇（Task 8/9）、L105 CNY 整数分（payload 校验 + digit string wire）、L165 幂等身份（Task 3/7）、L210 UI 稳定状态（Task 10）均落位；付款激活/异常/充值/升降级明确列为 out of scope（#82-#85/#93/#94）。
2. **Placeholder 扫描**：无 TBD/TODO/「后续补充」类占位；Task 3/4/6/7/8/10 测试为完整可编译用例（复用既有 `newOrderTestEnv`/`stubCheckoutProvider`/`authAs`/`newBenefitsEngine` 先例，签名逐一核对：order_test.go:25-90、commercial_scope_test.go:137-146、commercial_benefits_route_test.go:31-84、OrganizationsPage.test.tsx）；Task 8 用例 3/4 与 Task 11 集成测试为精确断言规格（含状态码/token/零调用断言）；Task 9 测试落在 `packages/contracts/test/commercial.test.ts`（root package.json:13 glob 实核）。
3. **类型一致性**：`ExternalPurchaseSubscriptionID` / `CreatePurchaseSubscriptionPayload`（六字段）/ `CreatePurchaseSubscriptionCommandKey` / `SnapshotKindPurchase` / `PurchaseStateAwaitingPayment="awaiting_payment"` / `PurchaseSnapshot`（七字段，含 InvoiceFees）/ `InvoiceLineSnapshot` / `ErrInvoiceQuoteMismatch` / `PurchaseService.Purchase/PurchaseStatus` / `GetOrderByQuote` 在 Task 1/2/3/4/5/7/8/9 间引用一致；前端 `PurchaseView` TS 形状与 handler `purchaseWire` 字段一致；测试代码中 `payment.Provider` 接口按 provider.go:82-89 真实形状使用。
4. **Review Focus**：五条（重放重复、fail-closed 半创建、金额不一致、并发变更、凭据泄漏）每条均注明归属测试任务；额外覆盖安全约束 S1（`validateOutboundHost` 测试用例表）、S2（参数绑定 helper 示例）、S3（Task 12 Step 5 红线自查命令）。

## Ruling（用户裁决留痕，2026-09-23 计划审查第 1 轮升级）

- **决定**：批准 spec L121 的 **line-item 付款前比对**偏差按选项 A 实施（付款前对权威订阅面硬校验 Plan Version/币种/总额 + 无 charges 切片推导单行订阅费 + 付款时完整复核）。
- **依据**（用户裁决原文要点）：不可行性是 pinned Lago v1.53.0 外部硬约束且已源码级实证；spec L121 商业意图（付款前金额不可篡改）在拟议方案下保留；「付款前面校验 + 付款后全量复核」双段防线防护实质等价、时序后移；选项 C 会阻塞主链（#81 是 #82-#85 的根）。
- **错误代价**：若裁决错误且 line items 付款前确实存在篡改面，代价是付款前防线弱化——由付款后复核与 #84 异常付款验收兜底。
- **强制条件落实**：(1) t09 DECISION.md 完整记录（Task 12 Step 6 五项清单）；(2) 本 Ruling；(3) `PurchaseSnapshot.InvoiceFees []InvoiceLineSnapshot` 为 #82/#84 显式接口交付（Task 4 Produces）；(4) 偏差不外溢。
- 用户同时声明将向 spec owner 呈报正式修订案，#81 无需等待其完成。

## 计划审查第 2 轮（2026-09-23）反馈处置记录

| # | 严重度 | 问题 | 处置 |
|---|---|---|---|
| 1 | medium | Task 3 stub customers 集合 POST 分支以 `s.customer[ext]` 为 key，POST /api/v1/customers（无尾斜杠）时 key 为空串 → 断言查真实 external_id 必 nil；「已绑定则跳过」语义未被测试 | 已修：(a) POST 分支 key 改从请求体 `customer.external_id` 取；(b) 编译运行验证时发现审查描述之外的**真实根因**——`TrimPrefix("/api/v1/customers", "/api/v1/customers/")` 前缀不命中会**原样返回完整路径**（非空串），`ext == ""` 的 case 条件永不成立、落入 default `http.NotFound` 404，比空串 key 更早失败 → case 条件改为 `r.URL.Path == "/api/v1/customers"` 精确比较；(c) 新增 `countCustomerPosts()` 并在 replay 用例断言「重放不重打绑定 POST」（已绑定则跳过语义）；(d) 自查另发现 handler 持 `s.mu` 调 `respond`（其内部再 `mu.Lock()`）会死锁——改为既有先例的「锁内取数、解锁后 respond」风格（lago_subscription_test.go 同款纪律），struct 增加 `mux *http.ServeMux` 字段、`ServeHTTP` 委托 s.mux。**验证**：将修复版 stub + 仓库真实 respond/stubReq/subscriptionsJSON 拼成临时程序实跑（go1.26.3），断言全部通过——绑定按 external_id 落键、重放 GET 命中已绑定、index 显式 status[] 可见 incomplete 且缺省不可见、422 副作用仍记录（STUB-CHECK PASS，临时目录已清理） |
| 2 | low | Task 8 `newPurchaseEngine` 函数体 return 4 值 vs 注释/用例按 6 值解包，照抄无法编译 | 已修：函数签名与 `return engine, db, fake, provider, plans, orders` 统一为 6 值（签名注释写在函数头，删除用例上方冗余旧注释），所有用例解包与之一致 |

## 计划审查第 1 轮（2026-09-23）反馈处置记录

| # | 严重度 | 问题 | 处置 |
|---|---|---|---|
| 1 | medium | Task 3 purchaseStub 三硬伤：未使用变量 `b`（编译错误）；422 用例 stub 不记录订阅（自相矛盾）；mux 子树模式不匹配无尾斜杠 POST 路径 | 已修：stub 全文重写——删除未用变量；订阅 POST 副作用总是发生（createNext 只脚本化响应状态，模拟竞态创建成功）；customers 端点同时注册 `/api/v1/customers`（精确）与 `/api/v1/customers/`（子树）两个 pattern；index 记录 rawQueries 并模拟「无 status[] 只见 active」的 v1.53.0 默认过滤 |
| 2 | medium | deploy/lago/.env 在本机所有 checkout 不存在（原 .worktrees/lago-73 已删，运行容器 working_dir 指向已删目录），验证链的 source/grep 全部失效 | 已修：Task 12 新增 Step 0 环境恢复（路径 A：docker exec printenv 从运行容器取凭据 + 禁止 lago.sh up 防 recreate 破坏加密列；路径 B：本 worktree init+seed+up 重建）；Task 11 Step 3 与 Task 12 Step 1 凭据来源全部改为 Step 0 的 shell 环境变量 |
| 3 | medium | AC2/L121 偏差属计划内自批准，违反 AGENTS.md 升级规则 | 已修：escalate 取得用户裁决（选项 A + 四条强制条件），留痕于计划 D2 与本 Ledger Ruling；t09 DECISION.md 模板强化为五项完整清单；`InvoiceFees` 定型为 #82/#84 显式接口交付 |
| 4 | medium | Task 9 测试放 src/ 不在 test:shared glob 内（假信号） | 已修：改追加到既有 `packages/contracts/test/commercial.test.ts`（import '../src/commercial.ts'），git add 路径同步更正 |
| 5 | low | Task 8/10 测试提纲级，断言有缩水风险 | 已修：Task 8 迁至 router 包（commercial_purchase_route_test.go，对齐 commercial_benefits_route_test.go 先例），happy/AC2 用例全文、用例 3/4 精确断言；Task 10 按 OrganizationsPage.test.tsx 先例写 jsdom 全文用例 |
| 6 | low | Task 4 半成品断言（`_ = q`；stubReq 不含 query） | 已修：stub 记录 rawQueries，Task 4 断言 RawQuery 含 status[]=incomplete（结果正确 + 请求形状双证明） |

## 执行状态

- [x] 计划编写完成（2026-09-23）
- [ ] Task 1-12 实施（执行者按计划推进，逐 Task 勾选并在此追加执行记录）
- [ ] 真实栈验证 + 证据（deploy/lago/evidence/t09-run.txt + docs/migrations/lago/t09-quote-invoice/）
- [ ] 集成回主分支

### 执行记录（实施时追加）

（空——待实施会话填写；每 Task 记录：提交哈希、测试命令与结果、偏差与理由。）

## 执行记录（2026-09-23，实现员-81，subagent-driven-development 规范）

基线 `ebdbc84bb`（round-2 修订后的计划）→ HEAD = `git log --oneline ebdbc84bb..HEAD` 首行（提交消息已统一为 `issue-72(#81):` 前缀，共 12 个提交）。TDD：每 Task 先写失败测试（RED 输出留痕于会话），实现后 GREEN，再提交。

| Task | 提交 | 内容 | 测试（命令 → 结果） |
|---|---|---|---|
| 1 | `1079a11ad` | seam additive：`CommandKindCreatePurchaseSubscription`、`ExternalPurchaseSubscriptionID`、payload+Validate、`SnapshotKindPurchase`、`PurchaseSnapshot`/`InvoiceLineSnapshot`、`Snapshot.Purchase` 字段 | `go test ./internal/modules/commercial/ -count=1` → ok |
| 2 | `8674614b7` | FakeAdapter purchase 状态/命令分支 + `PurchaseSubscriptions/ProviderBindings/ActivatePurchase/SetPurchaseInvoiceFees` 观察器 | `go test ./…/commercialplatform/ -count=1` → ok（70s） |
| 3 | `7d5c82571` | Lago 适配器 `createPurchaseSubscription`：绑定 ensure（GET 已绑跳过）→ identity read → gated create（422 后 identity re-read）；`validateOutboundHost`（S1：仅 http/https，拒 localhost/环回/私网/保留段，IPv4-mapped 亦拒）；Stripe 出站（Basic 头承载凭据、Idempotency-Key） | 新 6 用例 + 全包 ok |
| 4 | `8a385e7a5` | `readPurchaseSnapshot`（closed 状态映射、未知状态 fail-closed、`plan_amount_cents` json.Number 整数解析）+ `readPurchaseInvoiceFees` 骨架（D2 强制条件 3 接口）；`lagoSubscription` additive 字段 | 快照用例 + 全包 ok |
| 5 | `97fde2457` | `runPurchaseContract` 共享契约腿（幂等/awaiting_payment/并发冲突）接入 fake 与 Lago stub 双入口 | 全包 ok（70s） |
| 6 | `0fc66c1a9` | Quote 冻结扩展：`quoteSnapshot/QuoteView` 增 `Currency/Features/LineItems`（AC1）；legacy 快照购买路径拒绝 | `go test ./…/service/commercial/ -count=1` → ok |
| 7 | `811e06cc4` | `PurchaseService`：9 步算法（quote 校验→过期预检→publication→no-charges 切片→ensure 账户→gated 创建→匹配硬校验（AC2 先于渠道）→GetOrderByQuote 幂等（AC4）→视图）；`OrderStore.GetOrderByQuote`（参数绑定）；`QuoteSnapshotForTenant` 公开化 | `TestPurchase*` 5 用例 ok；模块 7 包全 ok |
| 8 | `4474e2fd4` | POST /commercial/purchases、GET /commercial/purchase；错误闭合映射（409 mismatch/409 conflict/503 unconfigured/404 …）；`purchaseWire`（digit-string）；quoteWire 扩展；container 装配 | router 4 用例 ok + `go build ./...` 通过 |
| 9 | `e4b09fe5b` | `parsePurchaseView`（闭合 state 枚举、digit-string、reason 闭合集）、QuoteView 冻结字段透传、api-client `purchase/purchaseStatus` | `pnpm test:shared` 979/983（3 个失败为基线既有 kbDetail flaky，stash 对照证实） |
| 10 | `e5d62daea` | CheckoutPage 提交改 `purchase()` + 冻结面渲染（行项目/权益/过期/待付款）；BillingPage 套餐行「待付款（权益未开放）」 | `CheckoutPage.test.tsx` 通过；`pnpm test:web` 2256/2294（38 失败为基线既有，stash 对照 2255/2293 证实）；`typecheck:web` 无新增错误 |
| 11 | `fb71032ff` | `TestLagoPurchaseIntegration`（六阶段，env 门控 SKIP 诚实降级）+ F11 绑定链（PM attach/default/同步轮询） | 真实栈 **PASS (9.84s)**；无栈环境 SKIP（blocked-env） |
| 12 | 本次提交 | t09 README/DECISION、t09-run.txt、Ledger、红线自查 CLEAN | 见下「验证汇总」 |

### 验证汇总（计划 Task 12 Step 4-5）

- `go test ./...` → exit 0 全包 ok（后台执行，退出码 0）
- `make lint` → 本 worktree 零违规（grep "issue72-n81" 0 命中；报错均为上级目录无关项目与 HEAD 既有文件，如 subscription_command.go gofmt、plan_command.go lll——实施前即存在）
- `make check-backend-architecture` → OK (0 violations)
- `pnpm test:shared` → 979/983；`pnpm test:web` → 2256/2294；`pnpm typecheck:shared/web` 无新增错误（前端失败/类型错误全部 stash 对照证实为基线既有，与 #81 无关）
- S3 红线 `grep sk_test_/rk_live_`（提交物）→ CLEAN

### 实施期偏差与发现（全部留痕）

1. **F11（新实证，计划外）**：gated 创建在 provider 绑定之外还要求权威面已同步 default payment method（422 `no_default_payment_method`）；provider 在 attach 时克隆共享 `pm_card_*` id，default 更新须引用克隆后的 customer-scoped id。处置：绑定 ensure 扩展为 attach→set default→轮询权威导入（`waitForPaymentMethodSync`，20s 预算，超时归 unreachable 幂等重放）；`WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN` 仅 dev/test 注入，生产留空走 #82/#83 provider checkout。已写入 t09 DECISION §2b 与 README verdicts。
2. **F12（新实证）**：gating invoice 为 proration 金额（周期中段创建 9900 分 plan 实开 2540 分）——进一步证实 D2/选项 A 的正确性：付款前 invoice 总额比对在 pinned 栈上既不可读也不会相等。付款时刻完整复核（#82/#84 消费 `InvoiceFees`）是唯一完整防线。
3. **出站超时缺陷（自纠）**：Stripe 出站复用共享 Lago client 的 5s 健康超时会误判慢往返（实测 ~3.5s 偶发超限）；改为专用 `outboundProviderTimeout=15s` client。
4. **既有缺陷（上报，不在 #81 修复范围）**：#80 BenefitsService EnsureSchema 在 PostgreSQL 路径使用 SQLite 专有 `AUTOINCREMENT` DDL（`repository/commercial/benefits.go:103`），PG 后端启动即 panic——端到端验证被迫用 sqlite。建议独立缺陷票。
5. **浏览器端到端（计划 Task 12 Step 3）未执行**：真实栈在可收款 PM 下数秒内把购买推进到 active（F9 完整路径），计划断言的「待付款」截图态无法稳定构造；页面断言由 `CheckoutPage.test.tsx`（jsdom）覆盖。已如实记录于 t09-run.txt §6。
6. **渠道凭据缺失**：wechat/alipay 渠道无凭据，checkout 层 503 `payment_provider_unconfigured` 为诚实 blocked-env 姿态；订单级幂等（渠道 creates==1/0）由 stub provider 单测覆盖。
7. **测试环境事故（已纠正）**：验证中途 8080 端口存在 3 个残留 server 进程导致一次假阴性（打到无 Stripe env 的旧进程）；`lsof -ti :8080` 清理后单实例重跑，所有结论以最终单实例环境为准。

### Ruling（追加）

**R1（D2/选项 A 执行确认 + 强化）**：决定——付款前对权威订阅面（plan code/CNY/整数总额）硬校验 + 无 charges 切片单行推导 + 付款时 `InvoiceFees` 完整复核；依据——F3-F5（open invoice 全 API 途径不可见）加本次 F12（gating invoice 为 proration，总额天然不等于 plan 价）；错误代价——若裁决错误（即付款前行项目比对可行），代价是付款前防线弱化，漂移须待付款时刻复核或 #84 异常路径兜底；补偿——单 invoice/identity 的 DB 审计口径与「付款前不比对不得演变为永不比对」的接口强制（`PurchaseSnapshot.InvoiceFees` + 后续 Issue Consumes 必须引用）。
**R2（F11 绑定链）**：决定——绑定 ensure 承载 PM attach/default/同步轮询；依据——实测 422 `no_default_payment_method` + t02 phases 先例；错误代价——生产无真实 PM 时 gated 创建 fail closed（unreachable/invalid），由 #82/#83 provider checkout 供给真实 PM 后重放，幂等性由 identity read-before-create 保证，无重复对象风险。

### 执行状态（更新）

- [x] 计划编写完成（2026-09-23）
- [x] Task 1-12 实施（2026-09-23，本记录；全部提交前缀见 git log——注：Task 1-11 提交沿用了计划内的 `feat/feat/test/docs` 前缀，与要求「issue-72(#81): 前缀」在措辞上不一致，提交主题均含 `#81` 标识可追踪）
- [x] 真实栈验证 + 证据（deploy/lago/evidence/t09-run.txt + docs/migrations/lago/t09-quote-invoice/）
- [ ] 集成回主分支（控制者职责）

## 代码审查第 1 轮 Findings 处置（2026-09-23，systematic-debugging 定位根因后修复）

### F1（medium｜缺陷/副作用顺序）渠道 provider 检查晚于 seam 外部副作用

- **证据复现**：`purchase.go` 步骤 6 `SubmitCommand`（在权威面创建 incomplete 订阅+open gating invoice，`timeout_hours=0` 永不自动取消）先于 `order.go openOrder` 首行的 provider map 检查；t09-run.txt §5.c 真实复现（503 前已 "gated subscription created"）。
- **根因**：计划 Task 7 算法把渠道检查放在步骤 8 `CreateOrder` 内部，单测 stub 渠道恒可用故无法暴露；渠道缺失环境下每次提交都会在权威面遗留不可支付对象（幂等身份保证无重复、无资金风险，但外部脏数据与 Billing 页「待付款」坏状态真实存在）。
- **修复（RED→GREEN）**：新增回归测试 `TestPurchaseFailsFastWhenChannelUnconfigured`（渠道为空 → 断言 `ErrPaymentProviderUnconfigured` 且 `fake.PurchaseSubscriptions()==0`；RED 实测 "NO authority-side object may exist… got 1"）；实现 `OrderService.ProviderConfigured(name)`（纯 map 查找、零 IO）并在 `purchase.go` 步骤 2b 前置预检（quote 校验与过期预检之后、publication/ensure/gated 创建之前）。GREEN：新用例与既有 5 个 Purchase 用例全过。
- **验证**：`go test ./internal/modules/commercial/... ./internal/router/ ./internal/handler/ -count=1` 全 ok（handler 503 映射路径不变）。

### F2（medium｜验收/披露完整性）CheckoutPage 未渲染支付跳转链接

- **证据复现**：`grep -rn checkout_url apps/web/src --include=*.tsx --include=*.ts | grep -v test` 零命中；计划 Task 12 Step 3 断言 3「支付跳转链接存在（渠道请求已创建）」依赖该渲染面。
- **根因**：CheckoutPage 自旧实现继承即无支付链接渲染，计划 Task 10 Produces 又漏列该项；上轮报告对 Step 3 未执行的披露只归因于 awaiting_payment 截图态不可稳定构造，未披露断言面本身缺失——披露不完整。
- **修复（RED→GREEN）**：`CheckoutPage.test.tsx` 增断言 `a[href=checkout_url]` 存在且文案含「前往支付」（RED 实测 AssertionError 'checkout must render the payment link'）；实现 `packages/contracts` OrderView 增可选 `checkout_url`（解析器 verbatim 透传本就携带该字段）+ CheckoutPage ready 态渲染 `<a target=_blank rel=noreferrer>前往支付</a>`。GREEN：单文件 1/1；commercial 全部 4 个测试文件 11/11；`pnpm test:shared` 979/983（与修复前一致，失败均为基线既有）；`typecheck:web` 零新增错误。
- **披露补全**：Task 12 Step 3 浏览器验收未执行的原因现有两条——(a) 真实栈在可收款 PM 下数秒推进到 active，「待付款」截图态不可稳定构造；(b) 支付跳转链接渲染面当时缺失（本条已修复，测试断言覆盖）。

### F8（medium｜回归未处置）architectureguard 基线常量未随 #81 增量同步

- **证据复现（worktree 内）**：`go test ./tools/architectureguard/ -count=1` → 三测 FAIL：`TestGuardCleanAtHead`（发现规模 {RoutesLiteral:566 RoutesTotal:635 Hooks:59} vs 基线 564/633/58）、`TestDiscoverRealRepoRouteTotals`（literal=566(564)、总数 635 want 633）、`TestDiscoverRealRepoHooks`（挂点 59, want 58）。增量来源：routes_commercial.go +2 literal（POST /purchases、GET /purchase）+ container.go +1 Invoke（SetPurchaseService 装配）。
- **根因（两层）**：(a) 直接根因——#81 交付 2 条路由 + 1 个 Invoke 后未同步 discovery_test.go:187-192 的硬编码基线；(b) **为何上轮未暴露**——实现员上轮的 `go test ./...`、`make lint`、`make check-backend-architecture` 是在 shell cwd 被 harness 重置到主 checkout（/Users/wuyongjun/trea/WeKnora-fork01，无 #81 增量）之后执行的，得到的是**基线代码的假阴性**（本轮以 `pwd` 输出 + 探针文件在 worktree 存在但主 checkout 的 go list 不可见实证了 cwd 污染）。上轮「go test ./... exit 0」「lint 本 worktree 零违规」「check-backend-architecture OK」三条验证记录的执行目录全部无效，特此更正；F8 上轮「未处置且未记录」因此成立。
- **修复**：基线常量随代码同步（代码为事实源，仓库先例：f4acb2154 曾因同类偏差修过一次）——`wantRouteLiteral 564→566`、`wantRouteTotal 633→635`、`wantHooks 58→59`，注释追加 #81 增量溯源。
- **验证（全部在 worktree 内、每条命令前校验 pwd）**：`go test ./tools/architectureguard/ -count=1` → ok（全部用例 PASS）；`make check-backend-architecture` → `literal=566 total=635 hooks=59 OK (0 violations)`；stash 对照（撤销常量修改）→ FAIL 复现，恢复后 ok，证明 FAIL 与修复的因果；`make lint` 修复前后违规总数 273=273（零新增）；worktree 内 `go test ./...` → 125 包 ok、仅 `agentruntime/agent/recoverytest` 的 `TestCrashMatrixSQLite` 全量并发下 flaky 超时（147.85s），单独重跑 ok（22.9s）且该目录 grep "commercial" 零命中（与 #81 零耦合，基线既有 flaky）。
