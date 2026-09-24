# Issue-72 Ledger — 计划员 #82

## 计划身份

- **计划文件**：`docs/plans/issue-72-plan-82.md`（[Lago 10 / Issue #82] 支付宝付款后恰好一次激活套餐）
- **对应 GitHub Issue**：https://github.com/1123786563/WeKnora-fork01/issues/82（OPEN，Blocked by #74/#81——两者已在集成分支落地，#74 证据晋升 t02，#81 交付 gated create/PurchaseService/InvoiceFees 接口面）
- **编写者**：计划员-82（dynamic workflow），2026-09-24
- **worktree / 分支**：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n82`（`codex/issue-72-lago-82`）
- **集成基线**：任务简报（2026-09-25 重入会话）称集成分支 tip 为 `900041bb05`；实测本分支基于 `7a665bb577`（`issue-72: ocr issue-81 round 2`，其提交时点 2026-09-24 21:59 即当时集成分支 tip）。`git merge-base --is-ancestor` 实测：`7a665bb57` 是 `900041bb05` 的祖先，即本分支落后集成 tip 4 个提交（#81 OCR 修复线），而非基于其后的分叉。勘误详情见下方「集成基线（2026-09-25 勘误）」节。

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

## 集成基线（2026-09-25 勘误）

- 任务简报声明本 worktree「基于集成分支 `900041bb05`」，实测不符：`git merge-base --is-ancestor 900041bb05 HEAD` → 否；`git merge-base HEAD 900041bb05` → `7a665bb57`；`git merge-base --is-ancestor 7a665bb57 900041bb05` → 是。结论：本分支基于集成分支的**祖先** `7a665bb57`（建 worktree 时的 tip），集成分支其后追加 4 个提交：`759f386e1`（ocr ledger #81 backfill）、`d27a451cb`（ocr round 1 重做）、`fc8448308`（ocr-81-1 增量 R1-08/15/24/35）、`900041bb0`（ocr round 2）。
- 该 4 提交的树差异（`git diff --stat 7a665bb57 900041bb05`：18 files，+931/−1084）触及本计划消费/修改的文件：`commercialplatform/lago.go`（R1-24 新增 `deriveProviderCustomer`/`syncPaymentMethods` 测试缝）、`lago_purchase.go`（`ensureProviderBinding` 在 `cfg.StripePmToken==""` 时跳过 pm 轮询、ctx 取消错误链改为 `ctx.Err()`）、`service/commercial/order.go` + `repository/commercial/order.go`（R1-35 `OrderRow.CheckoutURL` 持久化 + `SetCheckoutURL`）、`purchase.go`（`orderViewFromRow` 回带 CheckoutURL）、`lago_purchase_test.go`（+114）。
- **对计划的影响**：本计划行号锚点按本 worktree（7a665bb57 线）核实仍然成立；重入集成 rebase 到 900041bb05 线后，Task 5 实现需适配两点——① pm 同步轮询应经 `a.syncPaymentMethods` 缝（R1-24）而非直调 `waitForPaymentMethodSync`；② 结算 pm 切换路径不得依赖「StripePmToken=="" 时跳过轮询」的既有语义被误用。本计划不预先改写 Task 5 步骤（升级条款下 Task 5 本就处于停止态），适配点记录于此备查。

## 2026-09-25 重入复核（计划员-82 第二次会话）

- **重入现状**：本计划已在首次会话（2026-09-24）完成编写、提交（c17e7e7b9）与第 1 轮审查修订（8a940608a）；随后进入执行——Task 2/3/4 已落地（66ab920cc seam settle 命令、1cf072701 fake 确定性结算、782e316da finalized 行项目读），Task 1 lab 代码与证据已产出（工作区暂存未提交 + `docs/migrations/lago/t10-payment-trigger/` 已含 DECISION.md/README/evidence）。`lago_settlement.go` 不存在（Task 5 未实施）。
- **重大执行事实（Task 1 probe 结论）**：probe 在 pinned 栈实跑，**P1 FAIL（F5 被证伪：挂死窗口 `GET /payments` 返回 `payments: []`，付款行仅 authority DB 可见）+ P2d FAIL（F3 收窄：gating invoice 隐藏窗口 `retry_payment` → 404 `invoice_not_found`）**——D2 结算触发链被证伪，**Task 1 升级路径已触发**（t10 DECISION.md verdict「UP升级路径触发（P2 FAIL）」）。按计划纪律：Task 5+ 实现面停止，凭 t10 证据由 spec/ADR owner 重议 T02 §5 选项；不实施替代猜测路径。计划文件顶部已加「执行状态注记（2026-09-25）」如实记录。**本票当前结论：其余可交付面（Task 2-4 + t10 证据）已成，AC5 的「受支持通道」机制在 pinned Community 上缺受支持触发器，需上游裁决后才能续做 Task 5-10。**
- **本次实跑核验**（全部本会话执行）：`go build ./...` → exit 0；`go test ./internal/modules/commercial/... -count=1` → 全部 ok（commercial 0.058s / commercialplatform 70.227s / payment 7.303s / repository 0.247s / service 0.919s 等 7 包）；`cd deploy/lago-lab/payment-trigger && python3 -m unittest test_phases -v` → Ran 21 tests, OK。
- **工作区注意**：暂存区有 Task 1 的 `deploy/lago-lab/payment-trigger/` 14 个文件（属执行流产物，非本计划提交范围）；本次提交仅含 `docs/plans/` 两文件（pathspec commit，不触碰他人暂存状态）。
- **计划文件自检（本次复核后）**：占位符扫描零命中；执行状态注记为 additive，未改动任何 Task/接口/追踪矩阵条目；Task 2-4 的 Produces 签名与已落地代码一致（`purchase_settlement_command.go` 存在，提交信息与 Task 2/3/4 标题逐字对应）。

## 审查记录（续）

### 第 2 轮（2026-09-25，计划审查员）——critical 1 条 + medium 1 条 + low 3 条，全部处置

| # | 发现（severity） | 复核（本 worktree 实测） | 处置 |
|---|---|---|---|
| 1 | 【critical·阻断】t10 证伪后计划正文（Task 5-10、追踪矩阵、真实流程验证方案）仍按 D2 可交付写成，与头部注记/升级条款矛盾；AC1/AC3/AC4/AC5 在重议前不可达成，需 spec/ADR owner 重议并产出修订版计划（替换 D2 与 Task 5-10）后方可批准执行 | 复核 t10 DECISION.md verdict 与 p1/p2 实录属实（P1 `payments: []`、P2d 404 `invoice_not_found`、pm 重导 31 次耗尽） | 头部注记升级为「执行状态注记与计划处置声明」4 条（正文最高优先级约束）；Task 5-10 标题全部加【冻结——设计存档，禁止执行】及各自冻结理由（Task 7 例外声明：AC2 渠道 pin 不受冻结影响）；追踪矩阵重编+逐条 ⛔/✅ 现状列；验证方案第 5-8 步冻结标注+通过判据改写（failures 必记 `d2-falsified-t10-p2-fail`）；D2/F3/F5 三处加证伪/收窄标注防误读。修订版计划的产出权在 spec/ADR owner，本计划不自行设计替代路径 |
| 2 | 【medium】Task 7 AC2 测试骨架引用不存在的 helper（newAlipayProviderForTest/signedReturn*），真实构造是 `alipayNotifyFixture(t)`（alipay_test.go:59），既有近义测试 `TestAlipaySyncReturnNeverConfirms`（:228-235）只断言 err!=nil 未 pin 哨兵；alipay.go 行号应为 :341-342 而非 :330-332 | `grep -n 'func newAlipayProviderForTest\|func signedReturn'` 零命中；sed 实测 :228-235 既有测试与 :59 fixture；alipay.go 实测 `VerifySyncReturn` 函数体在 :340-342（注释 :336-339） | Task 7 测试骨架改写为 `alipayNotifyFixture(t)` + 既有同款字面量查询串；明示增量价值仅一条：`errors.Is(err, ErrAlipaySyncReturn)` 哨兵 pin（防泛化错误回退）；Review Focus 第 3 条与 alipay.go 锚点改为 :340-342 |
| 3 | 【low】追踪矩阵把 spec L125 派生条件编为"AC5"与 issue 正式 AC 混排；`gh issue view 82` 实取仅 4 条 AC | 本会话实跑 `gh issue view 82 --repo 1123786563/WeKnora-fork01 --json body` → Acceptance criteria 恰 4 条（三态呈现/同步返回不确认/不重复履约/真实沙箱证据） | 矩阵节重编：编号勘误说明 + 正式 AC1-AC4 各一行 + GC-3 独立行 + Review Focus 行；每行加「现状」列（⛔ 被证伪阻断 / ✅ 已落地），并声明 ⛔ 行不得宣称达成 |
| 4 | 【low】行号漂移：providerOutboundCall/providerAttachDefaultPaymentMethod 实际在 lago_purchase.go:619/:654（原引 :549-617）；Task 4 占位 :481-486 已被落地实现替换（:489 起） | grep -n 实测两函数确在 :619/:654；sed 实测 :489 起为 `readPurchaseInvoiceFees` 真实实现（404 空语义） | Task 5 Consumes 行号勘误并注明"第 2 轮审查勘误"；Task 4 Files/Step 3 注明【已执行】与现位置 |
| 5 | 【low·核实通过】头部执行状态注记与仓库事实一致；Task 2/3/4 落地与 Produces 契约逐条吻合；lago_settlement.go 不存在与升级纪律一致；行号抽查（container.go:943、order.go、fulfillment.go、purchase_test.go、contract_test.go 等）全部命中；ledger 基线勘误属实 | 审查员实测 + 本会话 `go build ./...` exit 0、`go test ./internal/modules/commercial/... -count=1` 全绿（7 包，含 commercialplatform 70.2s） | 无需改动；作为本轮基线可信度记录在案 |

第 2 轮修订后自检：占位符扫描零命中；冻结标记覆盖 Task 5-10 全部标题与验证方案第 5-8 步；追踪矩阵 ⛔ 行与头部声明第 2 条口径一致（AC1/AC3/AC4/GC-3 受阻、AC2 渠道面 ✅）；未发明任何替代激活路径。本计划待修订版计划替换 D2/Task 5-10 后方可解冻执行。
