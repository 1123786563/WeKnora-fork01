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

## 计划自检结果（writing-plans 技能 Self-Review 四项）

1. **Spec coverage**：#81 四条验收标准 ↔ Task 1-12 映射齐备（计划内「验收标准 → Task → 测试追踪矩阵」）；spec L121 匹配前置于渠道订单（Task 7）、L122 activation rule（Task 3）、L169 awaiting payment 状态（Task 1/4/7/8）、L170 闭合产品词汇（Task 8/9）、L105 CNY 整数分（payload 校验 + digit string wire）、L165 幂等身份（Task 3/7）、L210 UI 稳定状态（Task 10）均落位；L121 的 line-item 逐项比对在 v1.53.0 不可实现于付款前（F3-F5 实证）→ 记录为 D2 spec 偏差 + 补偿设计，任务 12 产出 DECISION.md。付款激活/异常/充值/升降级明确列为 out of scope（#82-#85/#93/#94）。
2. **Placeholder 扫描**：无 TBD/TODO/「后续补充」类占位；Task 6/7 测试代码为完整可编译用例（复用既有 `newOrderTestEnv`/`stubCheckoutProvider`，签名已核对 order_test.go:25-90）；Task 8 测试以四组明确断言规格给出（依赖 commercial_benefits_test.go 的认证注入 helper，其命名以该文件为准——已注明参照路径而非虚构函数名）；Task 11 集成测试以阶段规格给出（env 门控与既有 lago_benefits_integration_test.go:148-152 同模式）。
3. **类型一致性**：`ExternalPurchaseSubscriptionID` / `CreatePurchaseSubscriptionPayload`（六字段）/ `CreatePurchaseSubscriptionCommandKey` / `SnapshotKindPurchase` / `PurchaseStateAwaitingPayment="awaiting_payment"` / `PurchaseSnapshot`（六字段）/ `ErrInvoiceQuoteMismatch` / `PurchaseService.Purchase/PurchaseStatus` / `GetOrderByQuote` 在 Task 1/2/3/4/5/7/8/9 间引用一致；前端 `PurchaseView` TS 形状与 handler `purchaseWire` 字段一致；测试代码中 `payment.Provider` 接口按 provider.go:82-89 真实形状（AttemptResult/Verify/QueryRefund）使用既有 `stubCheckoutProvider`。
4. **Review Focus**：五条（重放重复、fail-closed 半创建、金额不一致、并发变更、凭据泄漏）每条均注明归属测试任务；额外覆盖安全约束 S1（`validateOutboundHost` 测试用例表）、S2（参数绑定 helper 示例）、S3（Task 12 Step 5 红线自查命令）。

## 执行状态

- [x] 计划编写完成（2026-09-23）
- [ ] Task 1-12 实施（执行者按计划推进，逐 Task 勾选并在此追加执行记录）
- [ ] 真实栈验证 + 证据（deploy/lago/evidence/t09-run.txt + docs/migrations/lago/t09-quote-invoice/）
- [ ] 集成回主分支

### 执行记录（实施时追加）

（空——待实施会话填写；每 Task 记录：提交哈希、测试命令与结果、偏差与理由。）
