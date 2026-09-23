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
