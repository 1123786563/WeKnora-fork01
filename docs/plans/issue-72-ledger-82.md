# Issue-72 Ledger — 计划员 #82

## 计划身份

- **计划文件**：`docs/plans/issue-72-plan-82.md`（[Lago 10 / Issue #82] 支付宝付款后恰好一次激活套餐）
- **对应 GitHub Issue**：https://github.com/1123786563/WeKnora-fork01/issues/82（OPEN，Blocked by #74/#81——两者已在集成分支落地，#74 证据晋升 t02，#81 交付 gated create/PurchaseService/InvoiceFees 接口面）
- **编写者**：计划员-82（dynamic workflow），2026-09-24
- **worktree / 分支**：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n82`（`codex/issue-72-lago-82`）
- **集成基线**：`7a665bb577`（`issue-72: ocr issue-81 round 2`，工作区干净，`git log` 实测顶部提交）

## 本会话核验过的执行基线（全部实跑/实读）

- 事实源：`docs/specs/2026-09-20-lago-billing-migration-design.md`（L122-127、L169-171、矩阵 #5/#20）、`CONTEXT.md` 空间与商业章节、`docs/adr/0012-lago-as-commercial-billing-authority.md`（含 75-a2 修订+出处注记）、`docs/plans/issue-72-user-rulings.md` R-1/R-2/R-3。
- 证据：`docs/migrations/lago/t02-payment-activation/{DECISION.md,t02-*.json}`（含 retry_payment 405 invalid_status 原文）、`docs/migrations/lago/t09-quote-invoice/DECISION.md`（InvoiceFees 强制条件③）、`docs/plans/issue-72-flow-evidence-81/README.md`（3DS 稳定待付款窗口环境）、`deploy/lago-lab/payment-activation/{phases.py,evidence/}`。
- 代码：`internal/modules/commercial/platform.go`（冻结 seam）、`purchase_command.go`、`commercialplatform/{lago.go,lago_purchase.go,fake.go,config.go,contract_test.go,lago_purchase_integration_test.go}`、`service/commercial/{purchase.go,fulfillment.go,benefits.go,order.go}`、`repository/commercial/{order.go,outbox.go,benefits.go}`、`internal/handler/{payment_callbacks.go,commercial.go}`、`payment/alipay.go`、`packages/contracts/src/commercial.ts`、`apps/web/src/commercial/{Billing,Checkout}Page.tsx`。
- 命令实测：`go build ./...` exit 0；`go test ./internal/modules/commercial/service/commercial/ -count=1 -run TestPurchase` → ok 2.349s；`go test ./internal/modules/commercial/commercialplatform/ -count=1 -run 'TestFakePurchase|TestFakeAdapterPurchase'` → ok 0.470s；`npx playwright --version` → 1.63.0（较任务简报的 1.60.0 更新，以实测为准）；:8080 实测被占（lsof），:5272/:5273 按任务约束规避。
- pinned 源码核实（getlago/lago-api@591ae900，raw.githubusercontent 实取）：`invoices_controller.rb`（retry_payment + `payment_method{payment_method_type,payment_method_id}` 参数）、`payments/retry_service.rb`（draft/voided/succeeded 拒绝 + `ready_for_payment_processing?` 门 + `CreateService.call_async`）、`payment_serializer.rb`（`invoice_ids`/`provider_payment_id` 等）、`invoice_serializer.rb`（fees 集合）、`fee_serializer.rb`（fee 类型在 `item.type`，无顶层 fee_type）。

## 自检结果（writing-plans Self-Review）

1. **Spec 覆盖**：AC1-AC5 与 spec L122-127/L169-171、行为矩阵 #5/#7/#20 全部映射到 Task 1-10（追踪矩阵见计划内）；#81 分片边界（矩阵 5 不属 #81）由本票承接。
2. **占位符扫描**：`grep "TBD|TODO|待补|类似 Task|implement later|fill in"` 零命中；47 个执行步骤均含实际内容/代码/命令。
3. **类型一致性**：seam 新面（`CommandKindSettlePurchasePayment`/`SettlePurchasePaymentPayload`/`SettlePurchasePaymentCommandKey`/`PurchaseWalletName`/`WalletMetaPurchasePeriod`/`WalletName` 可选字段/`PurchaseStatePaidAwaitingActivation`）在 Task 2 Produces 定义、Task 3-8 消费处名称逐字一致；`errors`/`reflect` 等测试文件 import 由执行者按包惯例补全（既有同目录测试文件同款）。
4. **Review Focus**：6 条输入类均有归属测试（计划内 Review Focus 节 + 追踪矩阵）。
5. **诚实声明**：D2 激活触发机制的三处未决链接（payment 行 `invoice_ids` 字段、intent 取消解锁、retry 用新默认 pm 结算）**未在本会话于运行时实证**——已前置为 Task 1 的 probe（P1-P4）并写明证伪升级路径；F14/F9 明确排除 manual Payment 通道。除此之外全部机制均有本会话实测或 pinned 源码依据（计划「实证契约基线」F1-F14 逐条注明来源）。

## 裁决引用

- R-1（T02 选项②受支持 Provider）：决定付款激活通道设计；错误代价条款提醒 provider 路径收紧时 #82 需重议。
- R-2（75-a2 选项 B）：#85 充值批次口径，本票不触碰，但幂等键绑定渠道支付单号的原则在 D7 中对齐引用。
- R-3（L121 偏差选项 A）：强制条件③（InvoiceFees 付款时复核 = Task 4/7 落地）、F10（不得推迟 gated create——保住付款前匹配防线）。

## 遗留与风险

- Task 1 probe 若 P2 证伪：升级 spec/ADR owner，不实施替代猜测路径（升级路径已写入 Task 1）。
- 真实流程验证依赖外部可用性：Stripe TEST 出站（#81 曾遇间歇不可达）、支付宝沙箱凭据；两者均已在计划中给出 blocked-env 的诚实降级与证据标注方式。**AC4 残余（第 1 轮审查补充披露）**：支付宝沙箱凭据可用性未经本会话证实；凭据缺席时签名 notify 替身只证明"渠道验签→履约"链，不构成真实沙箱付款证据——failures 记 `ac4-sandbox-credentials-unavailable`，本票结论只能是「其余 AC 达成 + AC4 带残余」，不得写 AC4 达成。
- 并行批次：`internal/handler/session/craft_test.go`、`packages/career-core/` 等主 checkout 未跟踪改动与本票无关，计划已声明不触碰；#84/#85（order.go/commercial.ts 热点）在 DAG 上排后于本票，无并行写冲突。

## 审查记录

### 第 1 轮（2026-09-24，计划审查员）——8 条全部采纳并修订

| # | 发现（severity） | 复核（本 worktree 实测） | 修订 |
|---|---|---|---|
| 1 | Task 8 契约测试位置错误：`src/commercial.test.ts` 不存在且 test:shared 不收集（high） | `ls packages/contracts/src/*.test.ts` 仅 analytics/query-history/usage 三个孤儿；commercial.test.ts 在 `packages/contracts/test/`；root package.json:13 glob 确认不含 src | Task 8/Files 改为 `packages/contracts/test/commercial.test.ts` 追加，并明示勿模仿孤儿文件（防假绿） |
| 2 | Task 7 构造语义与现行 `gateway==nil → ErrFulfillmentGatewayMissing`（fulfillment.go:148-150）冲突，DI 装配未写明（medium） | sed 实测 :148-150 与 container.go:943 Provide 无显式实参 | Produces 写明四种组合的校验语义（nil+purchase 放行、fulfillEvent 非 purchase 分支 nil-gateway 防护）；container 装配改为注册 `NewPurchaseFulfiller` Provide + dig 自动注入；RED 形态改为编译错/ErrFulfillmentGatewayMissing 双形态 |
| 3 | Task 5 stub 订阅端点用路径段，实际身份读是 `/api/v1/subscriptions?external_id=...`（lago.go:650-656），happy path 必败于 404（medium） | sed 实测 lago.go:650-668 查询串形式 + 非 2xx 映射 invalid_response | stub 改注册裸路径并在 handler 内答 `{"subscriptions":[...]}`，calls 断言补查询串形状，注释写明原因 |
| 4 | `TestPurchaseFulfillCrashReplayConverges` 被矩阵/Review Focus 引用但无 Task 定义；`TestSyncReturnCannotConfirmPurchase` 无任何 Run 命令能执行（medium） | grep Task 7 测试清单确认缺定义；Run 命令只覆盖 service 包 | Task 7 补两个测试的定义代码与 Files（alipay_test.go）；Step 2/4 拆分为 service 包与 payment 包两条命令 |
| 5 | `paid_awaiting_activation` 落点三处矛盾（D3=seam 包 / Produces 注释=service 包 / Files 只列 service）（medium） | 复核三处原文确认矛盾 | 定版唯一落点：`internal/modules/commercial/purchase_command.go`（PurchaseState* 区之后，注明 coordinator-composed，readPurchaseSnapshot 永不产出）；D3/Produces/Files 三处同步 |
| 6 | AC4 沙箱凭据可用性未证实，降级路径下 AC4 严格意义上未闭环（low） | `~/.zcode/issue72-stripe.env` 存在（审查实测），ALIPAY_* 无凭据证据 | 验证方案第 3 步+通过判据+矩阵 AC4 行+Task 10 Step 1 四处加残余披露：failures 记 `ac4-sandbox-credentials-unavailable`，结论只能写「AC4 带残余」 |
| 7 | Task 6 骨架出现三个不存在的名字（newContractLagoStub/s.config/s.purchaseCount），既有/新建边界未标注（low） | 实测 contract_test.go:561-566 既有腿用 `newPurchaseStub()`+`purchaseAdapterWithPrefix(t, stub.server(t))`+`stub.countSubscriptionPosts` | Lago 入口改用既有名逐字同款，注释列明既有名（含行号）与新建名（runSettlementContract + purchaseStub 新扩展的 payments/payment_methods/invoices 分支） |
| 8 | 证据锚点错位：order_test.go:91 非 ConfirmPayment 先例；F3/F4/F6 仓库内不可复核；lab.sh 无 LAGO_FRONT_PORT（low） | 实测 order_test.go:85-97（是 TestOrderPipelineQuoteOrderRecover 段）；ConfirmPayment 先例在 fulfillment_test.go:140/234（grep 实证）；`grep 591ae900 deploy/` 仅 wallet-semantics 注释；`grep LAGO_FRONT_PORT` exit 1 | Task 7 注释改引 fulfillment_test.go:140/234 与 repository/commercial/order_test.go:97；F3/F4/F6 加「⚠️ 仓库内不可复核（本地无 lago-api clone），运行时兜底=Task 1 probe」标注；Task 1 改为「新增 LAGO_FRONT_PORT 变量（原 lab.sh 无此变量）」 |

修订后自检：占位符扫描 `grep 'TBD|TODO|待补|类似 Task'` 零命中；763 行；三个落点/文件名/测试引用与仓库实况一一核对。
