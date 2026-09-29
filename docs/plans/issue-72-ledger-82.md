# Issue #82 执行 Ledger（Lago 10 · 支付宝付款后恰好一次激活套餐 · R-4 重做轮）

## T9 duplicate webhook replay assertion — 2026-09-30

- After activation and InvoiceFees validation, the test captures the active authority snapshot and requires exactly one succeeded Lago payment, redelivers the same event object through the same route/provider/secret, requires HTTP 200, then requires a byte-identical authority snapshot and exactly one succeeded payment.
- Offline verification covers selector regressions, package tests, and integration-tag compilation only. The environment-gated live T9 replay was not run; AC3 replay acceptance remains OPEN pending a non-skip run. AC4 real Alipay sandbox evidence remains unavailable.

## T9 fixture identity repair — 2026-09-30

- Test-only repair in `lago_settlement_integration_test.go`: the synthetic success webhook now uses only the exact invoice-linked unsettled PaymentIntent captured before settle; unrelated historical successes are not fallback candidates. Missing or ambiguous pre-settle identity and missing post-settle match fail closed. No production source changed.
- RED evidence: focused tagged selector test failed by assertion for the old succeeded PI (`pi_old` selected instead of `pi_expected`) and for newly appearing linked PI (unexpected success accepted).
- GREEN evidence: `go test -tags lago_integration ./internal/modules/commercial/commercialplatform -run 'Test.*PaymentIntent.*(Candidate|Selection)' -count=1`; `go test ./internal/modules/commercial/commercialplatform`; `go test -tags lago_integration ./internal/modules/commercial/commercialplatform -run '^$'`; `git diff --check` all pass. The tagged full live T9 flow was not run.
- Residuals remain OPEN: live T9 acceptance still requires an environment-backed non-skip run; AC4 real Alipay sandbox evidence remains unavailable (`ac4-sandbox-credentials-unavailable`). This test-harness repair does not close either gate.

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

## 第 4 轮计划身份（2026-09-27 计划员会话追加）

| 项 | 值 |
|---|---|
| 计划文件 | `docs/plans/issue-72-plan-82.md`（R-4 双轨道修订版，**第 4 轮：OCR r2 清偿 + 浏览器 UI 复验收口**，Task 编号 T12-T17 延续第 3 轮） |
| 集成基线 | `codex/issue-72-lago` @ `da4b544db`（本会话 worktree 创建后 fast-forward 合并，无冲突；第 3 轮基线 f50c705074 已被完全包含） |
| findings 输入 | `docs/plans/issue-72-ocr-issue-82-r2.md`（101 findings，基线提交 `c6d027df8`）——本会话 8 点抽查证实关键产品代码 findings 在 da4b544db 上全部未修：明文口令 `browser_sync_face_82.mjs:35`、Purchase switch default→500（commercial.go:515-516）、default_payment_method 写入（lago_settlement.go:359）、paid-awaiting 窗口（purchase.go:253 仅探测 pending）、回调无 MaxBytesReader、order.go:217 带点索引名、boundProviderCustomerID 非 200 全 InvalidResponse（:445-447）、readSubscriptionByIdentity 429 落 default（lago.go:688-696） |
| 环境实测 | docker daemon 未运行（OrbStack 停，`docker info` 连接失败——任务简报「栈在跑」已过时，Task 17 前置为起栈）；`npx playwright --version`=1.63.0；:5272/:5273 禁用、:8093/:5194/:8294/:48889 空闲 |
| 主要范围 | T12 settle 链正确性（distinct-invoice 消歧闸 D9 / default 改写移除 D10 / 瞬态分类对齐 D13）；T13 购买竞态（paid-awaiting 防重开 D11 / 409+winner 重放 D12 / PG 索引名 / fallback 谓词）；T14 回调 body 封顶 D14 + drain 保护；T15 前端六点 D15；T16 evidence 脚本与 t11 lab security/bug 级清偿 + `_browser_lib.mjs`；T17 真实流程复验（R-19 浏览器面清偿） |
| 评估后不修（D16） | settle 空集幂等分支客户维度绑定（R-17 语义必要 + D7 兜底，完整绑定随 #84 回议）；fees N+1（R-23 已披露）；冻结证据脚本回改；typecheck 预存 5 错（独立票） |

### 第 4 轮自检结论（writing-plans Self-Review，2026-09-27）

1. **Spec 覆盖**：AC1-AC4 追踪矩阵逐行标注第 3 轮已证面与本轮补强点；GC 1-13 引 spec 原文（L105/L121-126/L165/L169-170，行号本会话实读核对）。
2. **Step 扫描**：T12-T17 全部 RED→GREEN→回归→Commit；无 TBD/TODO/占位（「以实际为准」两处已消除，测试文件名经 `ls` 实核：CheckoutPage.test.tsx/BillingPage.test.ts/order-state.test.ts/purchase_test.go/purchase_fulfillment_test.go/order_test.go/commercial_purchase_test.go/payment_callbacks_test.go 均在案）。
3. **类型一致性**：`CurrentPaidAwaitingActivationPurchaseOrder(ctx, tenantID uint64, amountFen int64, currency string) (OrderRow, error)`（T13 定义/消费）；`PURCHASE_STATE_LABEL`（T15 order-state.ts）；`_browser_lib.mjs` 导出 `{ LOGIN, login, note, runLeg, assertOrderIdentity }`（T16 产/T17 消费）。
4. **Review Focus**：5 条输入类（跨发票混入/paid 窗口重开/并发 409/限流误终态/超大回调体）均有归属测试名。
5. **比例**：以签名、测试名、判据为主；D9-D16 决策各一段；无实现体转写。

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
| Task 1 t11 探针 | `7ddc4deab` | `python3 -m unittest test_phases -v` 26/26（RED 先行：5 新用例未实现时 FAIL）；`python3 run_lab.py` **exit 0**（gated_3ds/settle_probe/settle_trigger/cleanup 全 pass，secrets scan 0 hits） | **PASS** | 五判据全实证：P-A 实测形态 `requires_payment_method`；P-C（D2' 未实证链接）confirm→succeeded；P-D webhook 200+inbound 落库；P-E active+finalized+恰 1 succeeded+重投 byte-identical。独立栈 weknora-lago-t11（:48895/:48896）。**F10 证伪**：webhook_secret 非 psql 列且本地栈为空（Stripe 拒回环 URL 注册，worker 日志实证）——见 Ruling R-11 |
| Task 2 seam additive | `9a75d2c56` | `go test ./internal/modules/commercial/ -count=1` ok（RED 先行：未定义编译失败→GREEN 3 用例 PASS） | PASS | settle_purchase_payment 命令/键、PurchaseWalletName、paid_awaiting_activation 常量、WalletName 可选字段 |
| Task 3 fake settle | `063470294` | 定向 3 用例 PASS + 全包 ok（70.5s） | PASS | fake settle 幂等/unknown fail-closed/terminal fail-closed（Ruling R-14）/购买钱包并存 |
| Task 4 fees 读取 | `b73301786` | 4 用例（映射/空/多张 fail-closed/无 sub 行）PASS + 两包 ok | PASS | 后按 t9 实测改为两步读（index 定位→单张读 fees，t10 真实栈实证列表不带 fees） |
| Task 5 Lago settle | `696f96395` | 7 用例 PASS + 全包 ok（73.3s） | PASS | D2' 五步；幂等键**端点限定**（Ruling R-13） |
| Task 6 契约腿 | `1b30ad3c8` | SettleContract fake+lago 双腿 PASS + 全包 ok | PASS | 共享腿：激活恰一次/同键重放等价回执/payload 类型 unsupported/非法 payload 拒绝 |
| Task 7 编排 | `c8b0fdd7e` | 7 用例（Settles/OverPayment/CrashReplay/Canceled/ShortWindow/GrantPeriod/PaidAwaiting+SyncReturn pin）PASS；7 包全量 ok；`make check-backend-architecture` OK（0 violations）；`make verify-module-moves` OK（16 manifests） | PASS | D5 分流判据=quote line_items（Ruling R-15）；D7 短窗跨轮；handler/order/sweep OCR r4 |
| Task 8/9/11 Go 侧 | `356b10772`、`9e3f199a3` | TestConfigured*/TestOutboundHost/TestNoDefaultPm PASS；`-tags lago_integration` 无 env SKIP（门控正确）；Task 11 六用例（ReplayCanceled/ReplayActive/Disambiguation/SkipsForeign/MismatchAudit/DeadlineDerived）PASS | PASS | 配置侧 loopback 旁路（仅放行 loopback）；429 并入 unreachable；单标签 host 拒绝；终审 6/7/9/10/11 清偿 |
| Task 9 真实栈 | 见 `356b10772`+`settle-evidence/t9-integration-run.txt` | `go test -tags lago_integration -run TestLagoIntegrationSettle -v` **PASS（17.6s，非 skip）**；证据 grep sk_test/whsec_ = 0 | **PASS** | 全链：gated create→等 unsettled PI→settle→真实 PI 事件+真实 secret 签名投递→active+D6'→重放 no-op。暴露并修复两缺陷（Ruling R-13 幂等键、R-16 两步 fees 读） |
| Task 10 脚本 OCR | 见收尾提交 | `node browser_flow_82.mjs`（缺 env）exit 2 + stderr 提示（三脚本同口径）；`python3 -c "import alipay_gateway_stub/alipay_sandbox_notify"` 冒烟 OK；`verify_ac_assertions.py`（74 目录）既有证据重放 ALL PASS | PASS | 三 .mjs 凭据 env 必需+FLOW82_WEB/EXPECT_CNY 参数化+waitForSelector+fileURLToPath+paid 冗余条件删；KEY_DIR env 化；四副本 endgame 放宽；81 docstring 修正 |
| Task 10 真实流程 | 见收尾提交 | API 链全通（t11 栈+支付宝 stub+真后端 :8093）：purchase→precreate→**同步面**→签名回调 success→**paid_awaiting_activation**→drain settle→webhook 投递→**active+fulfilled+applied 回执**；重复回调 success 且 fulfill 事件不增（AC3a）；四对象证据 `lago-four-objects-after-settle.txt`（subscription active/invoice finalized+numbered+succeeded+fee 1650 proration/payments 恰 1 succeeded/wallet 购买批次） | PASS（API 面） | 浏览器 UI 面未实跑（前端 jsdom 测试 12/12 覆盖逻辑；真实浏览器断言留给复验）；沙箱残余披露不变 |
| Task 12 settle 收口 | `0782c31fc` | RED 7 用例全 FAIL（形态与 r2 findings 一一对应）→ GREEN：定向 PASS + 全包 `go test ./internal/modules/commercial/commercialplatform/ -count=1` **ok 72.337s**；`go vet -tags lago_integration` clean | PASS | D9 消歧闸（跨发票 fail-closed 零扣款）/D10 删 default 改写（调用序列 5→4）/D13 瞬态分类（binding+subscription 429→Unreachable）/r2:166 key 检查提前（零出站）；既有用例同步调整 5 处（PicksLatest 改同 invoice 形态——D9 语义） |
| Task 13 购买竞态 | `2cbea7f64` | RED（409 复现 500 形态精确命中 r2:306）→ GREEN + 回归 `go test ./internal/modules/commercial/... ./internal/handler/ ./internal/router/ -count=1` **9 包全 ok** | PASS | D11 paid 窗口防重开（`CurrentPaidAwaitingActivationPurchaseOrder`，paid→fulfilled 由 OrderRow.State 承载——谓词收敛为 state='paid'，计划所写 fulfillment_state 列在仓库层不存在，Ruling R-27）/D12 409+winner 重放+retry 链分类/PG 下划线索引名/fallback 跳死 pending/r2:818 legacy 挂回 plan 校验/r2:327 doc 错位顺手修 |
| Task 14 回调封顶 | `d0f3b9c89` | RED 3 用例 → GREEN + 回归 9 包全 ok + `make check-backend-architecture`（0 violations）+ `make verify-module-moves`（16 manifests） | PASS | D14 MaxBytesReader 1MiB（入口处，两分支共用）；r2:119 三读分类（quote NotFound/快照损坏/publication NotFound→attention+nil 不中止 drain 批；瞬态→pending）；r2:341 isSubscriptionPurchase Warn |
| Task 15 前端收口 | `cfd20ebc1` | RED 5 用例+order-state SyntaxError → GREEN 25/25；`pnpm test:web` node26 **2311/2311**（新增 6 测试）；`pnpm typecheck:web` **5 错=F23 基线不增** | PASS | D15 六点：run() 守卫/channelRef 出依赖/submittedChannel null 深链不臆测/文案分流（订单加载失败）/令牌集补 2/canceled 专属文案+隐藏支付入口；PURCHASE_STATE_LABEL 共享词表（BillingPage 接入） |
| Task 16 脚本清偿 | `6cc9d7c40` | `python3 -m unittest test_phases -v` **27/27**（26 既有+1 新增顺序契约）；三 .mjs env 冒烟 exit 2+stderr；`bash -n`×5/`py_compile`×8 OK；红线 grep（issue82-Flow/sk_/whsec_）**零命中** | PASS | 明文口令→PASSWORD 变量（安全红线首项清偿）；token 落盘 REDACTED；密码 jq 构造+stdin；DB_PATH :? 前置守卫；LAGO_KEY order by created_at+非空断言；TENANT 序号断言；provider_code 白名单+WSECRET stdin（phases.py）；PHASE_SEQUENCE 由 PHASE_ORDER 派生；Timeline.text() 无副作用；`_browser_lib.mjs` 共享库+`r5-verify/seed.sh` |
| Task 17 真实复验 | 见收尾提交 | 四腿浏览器 21/21 PASS（真栈：Lago weknora-lago-82r5 v1.53.0+后端 :8093+vite :5194+stub :8294+Playwright 1.63.0）；四对象 `lago-four-objects-r5.txt`；重放三面 `rv5-06-replay-idempotent.txt`；Go 回归 10 包全 ok+双架构门 OK+vet(-tags lago_integration) clean；web 2311 中 2310 PASS（1 个未触碰 timing flake 单跑 7/7）+typecheck 5=基线；红线四项全零 | **PASS**（R-19 清偿；(h) T9 如实披露） | 判据 (a)-(g) 全过；(h) T9 实跑尝试：r5 共享栈 FAIL（webhook 400——测试取列表首个 succeeded PI，共享栈上属重复应用被 Lago 正当拒绝——环境形状缺陷）+t11 专属栈 lab.env 缺失——不以 skip 冒充 pass，等价覆盖由真实栈四腿+四对象+重放承担 |

## Ruling（执行期裁决）

- **R-11（t11，P-D secret 通道）**：计划 F10 假设「psql 直读 payment_providers.webhook_secret」——实测该列不存在（secret 在 settings jsonb accessor）且本地栈为空（Stripe 拒绝注册回环 URL："URL must be publicly accessible"，worker 日志实证）。裁决：harness 经同一 Stripe API（占位公网 URL 的 WebhookEndpoint.create，永不收推送）mint **Stripe 真实生成的 secret**，经 Lago **模型层**（rails runner update!，非 SQL）写入 provider，再以它签名投递真实 PI 事件。事件体/验签/处理链 100% Lago 内建；Stripe endpoint 用后即删。错误代价：若 owner 要求 Stripe 云端直投（需公网隧道），本腿需重做。**已记 t11 DECISION.md。**
- **R-12（lab key 引号）**：t11 lab.env 的值带双引号，后端 env 直传会 401——`prepare_t9_env.sh`/run 脚本统一 `tr -d '"'`。
- **R-13（Stripe 幂等键端点限定）**：计划 D2' 任务级 Produces 说「Stripe Idempotency-Key = cmd.Key」——实测 Stripe 把一个 key 绑定到**首个请求的 endpoint**，update 与 confirm 共用一键必 400 idempotency_error（t9 集成实证）。修正为 `<cmd.Key>:update` / `<cmd.Key>:confirm`（同一命令重放仍确定性成对）。单测断言同步。
- **R-14（fake settle terminal 语义）**：计划 fake 契约未定义 canceled 订阅的 settle 行为——定为 ErrPlatformInvalidResponse fail-closed（与 Task 11 第 7 条 seam 收紧一致）。
- **R-15（D5 分流判据）**：OrderKind 无法区分订阅购买单与充值单（domain 语义「purchase settles as credit top-up」）——分流判据定为 quote snapshot 的 line_items 含 subscription_fee（#81 冻结面），读不到 quote 视为非订阅购买单走既有 top-up。
- **R-16（fees 两步读）**：pinned v1.53 的 invoice **index** 应答不带 fees（t9 实测），单张 GET 才带——readPurchaseInvoiceFees 改两步（index 定位恰 1 张→单张读 fees+payment_status），stub 同步对齐。
- **R-17（settle-vs-finalize 窗口，t10 流程实证的产品缺陷修复）**：settle 已 confirm、webhook 未落地的窗口内重放 settle，原实现把「无 unsettled PI」判为 invalid_response→attention，且 attention 抢占唯一键阻断终局 applied。修复：succeeded+invoice-identity 的 PI 存在→幂等回执返回（`TestLagoSettleSettledWindowReplaysIdempotently` 锁定）；终局 applied 记录 UPSERT 覆盖 transient attention。
- **R-18（Lago grant WalletName 消费遗漏）**：Task 3 只改了 fake 的钱包身份分流，Lago 适配器 grantIncludedCredits 仍恒用 MonthlyWalletName——t10 四对象核验发现钱包名错（weknora-tenant-2-2026-09 而非 -purchase-2026-09）。修复：Lago grant 同步消费 payload.WalletName+period meta 键分流（purchase_period/period，F11 防撞）。
- **R-19（浏览器 UI 面未实跑）**：Task 10 的 Playwright 浏览器断言未执行（jsdom 单测 12/12 覆盖三态/渠道选择器逻辑；系统多会话高负载下完整 vite+chromium 链未起）。API 面（purchase→回调→drain→webhook→四对象）真实栈全通。UI 浏览器面留给复验轮。


## 收尾判定（2026-09-26 执行会话）

**通过判据（计划「真实流程验证方案」）**：

- (a) 三态文案：API 面 `awaiting_payment → paid_awaiting_activation → active` 全程实测（purchase 接口各态逐段核验）；前端三态渲染由 jsdom 测试 12/12 覆盖（checkout 三态/渠道选择器）+ Billing 7/7 —— **PASS（浏览器 UI 断言未实跑，R-19）**
- (b) 同步返回页零推进：同步面（checkout 刷新形态）实测不推进；`VerifySyncReturn` 恒 `ErrAlipaySyncReturn` pin（TestSyncReturnCannotConfirmPurchase）—— **PASS**
- (c) 三个重放面：重复回调 success 且 fulfill 事件计数不变（实测 2 单 2 事件）；settle 重放 no-op（T9 子测试 + TestLagoSettleSettledWindowReplaysIdempotently + TestPurchaseFulfillCrashReplayConverges）；Lago payments 恰 1 succeeded（四对象证据）—— **PASS**
- (d) 四对象齐全 + D6' 金额判据：`lago-four-objects-after-settle.txt`（active/finalized+numbered+succeeded/恰 1 succeeded/购买钱包批次；fee 1650 ≤ 订单 9900 = proration 剪裁实证披露）；支付宝侧 stub 披露 `ac4-sandbox-credentials-unavailable` —— **PASS（沙箱残余如实标注）**
- (e) 红线自查：`grep sk_(test|live)_{20,}` 零命中；本地 force-active 直写零命中；whsec_ 无可用凭据 —— **PASS**
- (f) 回归：9 包（commercial 7 + handler + router）全 ok；`make check-backend-architecture` OK（0 violations）；`make verify-module-moves` OK（16）—— **PASS**
- (g) 前端：commercial 22/22；typecheck commercial 相关错误清零（DevMarkdownPage/PlatformShell/mermaid 为基线既有、本票零触碰）—— **PASS**
- (h) T9 集成测试真实栈 env 下 **PASS（非 skip）** —— **PASS**

**明确移交/残余**：

1. 浏览器 UI 断言（Playwright 三脚本实测）未跑（R-19）——脚本已 OCR 修复并 env 参数化，复验轮以 `FLOW82_*` env 起链即可。
2. 终审第 9 条残余（带 checkout_url 的废弃 pending 单超时回收）与第 10 条 cancel 命令 → #84/#92（生命周期票，需渠道关单/terminate 契约实证）。
3. Stripe 结算款生产手续费/风控对冲 → R-4 生产前回议项。


## 代码审查第 1 轮处置（2026-09-26）

| # | 级别 | 问题 | 处置 | 验证 |
|---|---|---|---|---|
| R82-1 | medium | D7 总预算（settleObserveTotalBudget=10min 后转 attention）未实现：budget 字段只赋值不读；observeActivation 注释声称 attempt_count 治理但代码从不读；ErrPlatformUnconfigured 也恒 nil 保持 pending——激活永不落地时事件无限重放、已付款订单停留 paid_awaiting_activation 且无告警面 | **已修**：Fulfill 入口实现 overBudget 门（`attempt_count × (drainInterval 30s + firstPass 15s) ≥ budget` 即 D7 的累计轮次治理，attempt_count 是 drain 循环已在递增的持久计数）；超限→单次快照探测未 active 则 markActivationState(attention) 并停止驱动（事件留 pending、订单留 paid，可恢复）；快照已 active 则照常履约（恢复路径）；误导注释改为指向真实治理位置；unreachable/unconfigured 的可重试 nil 语义保留——预算到点由该门兜底转 attention | `TestPurchaseFulfillBudgetExceededTurnsAttention`（40 轮超限→attention、零 grant、≤10s 快速返回、订单留 paid）+ `TestPurchaseFulfillBudgetRecoveryWhenActive`（超限但 authority 已 active→fulfilled+恰 1 grant+applied 覆盖）双绿 |
| R82-2 | medium | 计划 Task 7 承诺的 `ErrQuoteNotFound→404 "quote not found"` 分支未补：不存在/失效的 quote_id 落 default 答 500，客户端输入错误被误分类为服务器错误 | **已修**：Purchase handler switch 补 `case errors.Is(err, repocommercial.ErrQuoteNotFound): 404 "quote not found"`（ErrQuoteTenantMismatch 404 之后、ErrPlanNotFound 404 之前的同族位） | `TestPurchaseHandlerAnswersNotFoundOnMissingQuote`（missing quote→404+closed token）绿 |

修复后全量回归：`go test ./internal/modules/commercial/... ./internal/handler/ ./internal/router/ ./internal/container/ -count=1` → **10 包全 ok**；`make check-backend-architecture` → OK (0 violations)。

## 真实流程验证失败修复（flowfix-82，2026-09-27）

验证环境（Issue #82 复验轮）：集成分支 worktree（codex/issue-72-lago @ 4de51f8c0 构建），Lago v1.53.0 栈 :48889、后端 :8093（sqlite data/issue82-r4verify.db）、前端 vite :5194、支付宝回环 stub :8294。失败三项 F-1/F-2/F-3 的定位、修复与复验如下；修复提交 `c5a1917bb`。

### R-20（F-1，高）settle 幂等键跨 PaymentIntent 碰撞 → 激活窗口内 fulfillment 误标 attention

- **根因（systematic-debugging 实读）**：`settlePurchasePayment` 步骤 (ii) 把「unsettled 候选非空」当作「settle 未完成」——`hasSettledGatingIntent` 幂等窗口只在 intents 为空时触发（lago_settlement.go 修复前 :104-116）。Lago v1.53 gated create + 首 confirm 3DS 失败固有留下**两笔** unsettled gating PI（requires_payment_method 的 pi_3UJxpK + requires_action 的 pi_3UJxpV，同 `metadata.lago_invoice_id`）：第一轮 settle 按 latestIntent 驱动最新 PI#2 → succeeded（幂等键 `cmd.Key+:update/:confirm` 绑定到 PI#2 的 endpoint）；第二轮重放时残留 PI#1 仍满足定位谓词 → 同键打到**不同 endpoint** → Stripe 400 idempotency_error → `ErrPlatformInvalidResponse` → `markActivationState(attention)`（purchase_fulfillment.go:171，attempt=2 起每 30s 一条警告共 19 次）。后果 (a) settle-vs-finalize 窗口内 fulfillment 误标 (b) 幂等窗口被残留 PI 短路 (c) 若幂等键未碰撞则确认 PI#1 = **第二笔真实扣款**风险面。
- **修复（采纳验证报告首选方向：幂等判定绑定 invoice 身份，而非掺 intent id——掺 intent id 会令每笔 PI 各得一键，反而放大 (c)）**：`stripeListUnsettledIntents`+`hasSettledGatingIntent` 合并为单次 `stripeListGatingIntents`（一次列表读同时回答 unsettled 候选与已 succeeded 的 invoice 身份集，窗口探测少一次 GET）；`latestIntent` 解析出的候选 invoice 已 ∈ succeeded 集 → 直接幂等回执（残留兄弟 PI 不再被驱动/重键）；intents 为空 + succeeded 集非空的原 t10 窗口语义保留。tie/created 不可解析仍 fail-closed 不变。
- **回归锁定**：`TestLagoSettleResidualSiblingIntentReplaysIdempotently`（残留兄弟+同 invoice succeeded → 幂等回执、仅 1 次只读列表、零写调用）、`TestLagoSettleDrivesWhenSucceededInvoiceDiffers`（invoice 不同 → 照常驱动，防过触发）；`TestLagoSettleSettledWindowReplaysIdempotently` 随合并读改为断言 1 次 GET。已在本会话实跑 PASS。
- **边界披露**：不主动 cancel 残留 PI（多一笔 provider 写反而引入选错对象的风险）；残留 PI 永久停留 requires_payment_method，不影响后续购买（新一轮 settle 的 latest 候选 invoice 不同于旧 succeeded 集 → 正常驱动，负向对照测试锁定）。

### R-21（F-2，中）readCustomerFeatures 路由在 pinned v1.53 不存在 → Entitlement 权益面恒缺失

- **根因**：`readCustomerFeatures` 调 `GET /api/v1/customers/{id}/entitlements`（lago.go 修复前 :1071）——验证轮容器内 routes 实读证实 pinned v1.53 无此嵌套路由（恒 404）→ `readBenefitsSnapshot` 静默吞错 → Benefits.Features 恒空。**单测 stub 实现了这条不存在路由（lago_subscription_test.go walletsStub），故漏测**；Lago 侧 entitlement 事实已物化，仅产品读取路径失配。
- **修复**：改走 v1.53 实际挂载的 `/api/v1/subscriptions/:external_id/entitlements`（验证轮经此路径实证读到 advanced_models），对两条 WeKnora 订阅身份（base `-sub` + purchase `-purchase`）各读一次取**并集**；某腿 404 = 该订阅尚不存在 → 贡献空、不报错。解析保持双形状容错（feature_code / 嵌套 feature.code）。
- **回归锁定**：`TestLagoBenefitsFeaturesReadSubscriptionEntitlementRoutes`（并集 + 断言请求走 subscriptions 路由、customers 嵌套路由零调用）、`TestLagoBenefitsFeaturesTolerateMissingPurchaseLeg`（purchase 腿 404 → base 特性照答、不伪造）；walletsStub 的 customers-entitlements 分支移除、新增 `/api/v1/subscriptions/` 真实形状路由（purchaseEntitlements 空 = 404 建模「无购买订阅」）。lago_integration 购买测试 Phase3 断言同步改 subscriptions 路由（原 customers 路径 404 断言在真实栈恒真、空洞）。
- **未跑披露**：`-tags lago_integration` 集成测试本轮未在真实栈复跑（验证轮栈已拆除）；已通过 `go vet -tags lago_integration` 编译门。

### R-22（F-3，低/环境披露）test:web 与 typecheck:web 全量失败 —— 判定成立，非 #82 引入，不改代码

- **复验（node v26.4.0，满足根 package.json engines ">=26"；本机默认 v22.22.3）**：web 全量 `node --import tsx --test` **2305/2305 PASS**（报告所列 ~18+ Vue 消息/文档类失败在 node26 下全部不复现 → 环境致败获证）；商业面子集 22/22 PASS。
- **typecheck 残留**：node26 下 `tsc -p tsconfig.json --noEmit` 仍有 5 错（mermaid.ts ×2、PlatformShell.tsx ×2、DevMarkdownPage.tsx ×1）——与报告所列同文件；`git log` 证实三文件最后改动 89e17fb6a（全域 UI 对齐轮，远早于 #82 merge 4de51f8c0，且 #82 仅触碰 commercial 文件）→ **预存缺陷、非本票引入**，按「只改问题相关文件」不在本轮修复，建议开独立票清偿。

### flowfix-82 回归清单（本会话实跑）

| 检查 | 命令 | 结果 |
|---|---|---|
| commercialplatform 全包 | `go test ./internal/modules/commercial/commercialplatform/ -count=1` | ok（77.5s） |
| 定向新/邻接用例 | `go test -run 'TestLagoSettle…|TestLagoBenefitsFeatures…' -v` | 12/12 PASS（含 4 个新增） |
| 10 包回归 | `go test ./internal/modules/commercial/... ./internal/handler/ ./internal/router/ ./internal/container/ -count=1` | 全 ok |
| 架构门 | `make check-backend-architecture` / `make verify-module-moves` | 0 violations / 16 manifests OK |
| 集成编译门 | `go vet -tags lago_integration ./internal/modules/commercial/commercialplatform/` | clean |
| web 全量（node26） | `node --import tsx --test 'src/**/*.test.ts' 'src/**/*.test.tsx'` | 2305/2305 PASS |
| web 商业面（node26） | `npx tsx --test 'src/commercial/**/*.test.ts(x)'` | 22/22 PASS |
| 安全红线自查 | diff 审查 | 无新增 SQL/凭据/外呼 host；外呼沿用既有 S1 host 校验路径 |

## 真实流程验证失败修复 第 2 轮（flowfix-82 r2，2026-09-27）

验证环境（复验轮 1 之后的失败项 F-5/F-2'/F-4/F-3'）：worktree @ ed779a2a8 重建后端（sqlite data/issue82-r4v2.db）、Lago 82flow 栈 :48889、支付宝 stub、node v26.4.0；主角 tenant 9、订单 ord_450538d2bbae2c55；证据提交 591af74b8（r4-flow2/）。修复提交 `de4f5dd86`。本轮修复期间 82flow 栈仍存活，**全部判据先在活栈实证再落码**。

### R-23（F-5，高）base 订阅的 finalized 发票使购买投影永久坍塌

- **根因**：`readPurchaseInvoiceFees` 按 `external_customer_id` 全查 finalized 并把「>1 张」当数据异常 fail-closed——lazy base onboarding（`GET /account` 触发，生产常态）会建 `-sub` 订阅并产生 0 元月度发票 auto-finalize，customer 名下恒有 ≥2 张 finalized → active 分支整体 `invalid_response` → `/purchase` 恒 `{state:absent,reason:invalid_response}`、billing/checkout「已生效」不可达（f5-purchase-collapse.txt：两张发票 d4d69920/ba5b262d）。
- **活栈实证（本轮修复前探测）**：v1.53 invoices 索引行不带订阅身份、`external_subscription_ids[]`/`subscription_external_id` 过滤参数均被忽略（两参试查仍返回全集）；**发票详情的 fee 带 `external_subscription_id`**（购买发票 fee=weknora-tenant-9-purchase/1650，base 发票 fee=weknora-tenant-9-sub/0），索引分页 `meta.next_page` 数字/null。
- **修复（采纳报告方向 fees→subscription）**：`finalizedInvoiceIDs` 分页拉索引（per_page=100、10 页预算，超限 fail-closed）；逐张读详情，取 `item.type=="subscription" && fee.external_subscription_id==购买订阅` 的 fee 行；恰 1 张购买发票→fees+payment_status、0 张→awaiting 空、>1 张→数据异常 fail-closed（原 fail-closed 语义保留，仅定位谓词收紧到购买订阅）。代价披露：每次快照对每张 finalized 发票一次详情读（长历史租户读放大），正确性优先、读路径非热点。
- **活栈端到端实证（修复后，tenant 9 失败现场原样）**：purchase face `state=active`、`fees=[{subscription_fee, Pro, 1650}]`、`payment=succeeded`（修复前 invalid_response 塌缩）。
- **回归锁定**：`TestLagoReadPurchaseInvoiceFeesIgnoresBaseSubscriptionInvoices`（base 0 元发票+购买发票并存→恰购买行、两张详情各读一次）；既有「两张购买发票 fail-closed」用例语义不变。**T9 集成测试全量本轮未跑**（7 个 operator-owned env 含真实 Stripe TEST key，容器内不可得，r4 轮证据为修复前代码）——披露。

### R-24（F-2'，中）entitlements 解析字段与 pinned v1.53 形状不符 + 值语义收口

- **根因**：c5a1917bb 只改对了路由，解析仍读 `feature_code`/`feature.code`；活栈两腿 curl 实证 v1.53 形状为 `{"entitlements":[{"code":…,"name":…,"privileges":[],"overrides":{}}]}` → features 仍恒空。
- **修复**：解析接受 `code` 为主形状、旧形状容错（`TestLagoBenefitsFeaturesParseToleratedShapes` 三形状表驱动）；walletsStub 默认 entitlements 改真实形状（既有测试即实证真形状解析）。
- **值语义深挖（本轮新发现并在服务层收口）**：活栈 base 腿 entitlements 含 **advanced_models/api_access/priority_support 全部三个 code**——发布腿 `attachEntitlements` 对 map 的**每个 key**（含 false 特性）都挂 entitlement，故 entitlement **存在性 ≠ 产品布尔真值**；纯存在性映射会让 base-only 租户误显示 advanced_models。收口（benefits.go `effectiveFeatures`）：**发布定义是值真值**（base 定义布尔为准）；购买 ACTIVE 时 OR 其 plan 的定义（经 purchase 快照 PlanCode→FindPublicationByCode→definitionOf；购买腿读取失败不破坏 benefits 面，静默退回 base 真值）；entitlement 物化仅补充定义未知的 code。`TestBenefitsFeaturesPurchaseFaceORsPurchasePlanDefinition` 锁定：base-only advanced_models=false、购买后 true、api_access 恒 true。
- **活栈实证**：修复后 adapter 快照 features=`{advanced_models:true, api_access:true, priority_support:true}`（购买者腿并集）；产品面经 effectiveFeatures 得 base OR pro 定义。

### R-25（F-4，中）购买 Credits 批次不在产品 benefits.credits 视图

- **根因（两层）**：① 适配器批次过滤器只认 `weknora_period` meta/`<ext>-<YYYY-MM>` 名——购买钱包（meta 键 `weknora_purchase_period`、名 `<ext>-purchase-<YYYY-MM>`，D4 设计）被当 foreign `continue`；② 服务层 BatchView 由本地注册表（`commercial_credit_batches`，UNIQUE(tenant,period)）驱动且 `balances[period]` 为覆盖赋值——购买授权走 PurchaseFulfiller **不经协调器注册表**，即便适配器修好也会被注册表缺行/同月覆盖隐藏。
- **修复（三层）**：lago.go 认 `weknora_purchase_period` meta 与 purchase 名前缀（fake.go `fakeWalletPeriod` 同修保持契约对等）；benefits.go 批次余额按 period **求和**（同月 base+购买聚合为一行月度余额，杜绝 last-write 任意性）+ 注册表缺失的快照独有 period 以快照过期时间并入视图（ oldest-first 确定性）。
- **活栈实证**：tenant 9 的 2026-09 月度批次=2 条、合计 10_900_000 micro（base 1.0+购买 9.9），总余额 10.9。
- **回归锁定**：`TestLagoBenefitsSnapshotIncludesPurchaseBatch`（适配器双批次+双倍余额）、`TestEnsureBenefitsPurchaseBatchJoinsCreditsView`（服务层 SUM+总额）、`TestRefreshProjectionUnionsSnapshotOnlyPurchasePeriod`（注册表缺行并入）、fake `TestFakeGrantPurchaseWalletNoMonthlyCollision` 扩批次断言。

### R-26（F-3'，低）typecheck:web 残留 5 错——维持 R-22 判定，不改码

与 R-22 同判：DevMarkdownPage.tsx/PlatformShell.tsx/mermaid.ts 共 5 错，最后改动 89e17fb6a（预存、#82 未触碰该三文件），本轮 web 面零改动未复跑全量（r4 轮 node26 test:web 全量 exit=0 已证测试面环境致败不成立）。建议独立票清偿。

### flowfix-82 r2 回归清单（本会话实跑）

| 检查 | 命令 | 结果 |
|---|---|---|
| commercialplatform 全包 | `go test ./internal/modules/commercial/commercialplatform/ -count=1` | ok（75s） |
| service 全包 | `go test ./internal/modules/commercial/service/commercial/ -count=1` | ok |
| 定向新/邻接用例 | `-run 'TestLagoReadPurchaseInvoiceFees…|TestLagoBenefits…|TestFakeGrantPurchaseWallet…|TestBenefitsFeaturesPurchaseFace…'` 等 -v | 全 PASS（含 6 个新增） |
| 10 包回归 | `go test ./internal/modules/commercial/... ./internal/handler/ ./internal/router/ ./internal/container/ -count=1` | 全 ok |
| 架构门 | `make check-backend-architecture` / `make verify-module-moves` | 0 violations / 16 OK |
| 集成编译门 | `go vet -tags lago_integration ./internal/modules/commercial/commercialplatform/` | clean |
| **活栈端到端（82flow :48889，tenant 9 失败现场）** | 临时 `-tags flowverify_live` 测试（验后已删）经真实 adapter 读 purchase/benefits 快照 | F-5 active+1650+succeeded；F-2' features 三 code；F-4 月度 10.9 —— 全 PASS |
| T9 真实栈全量 | — | **未跑**（Stripe key 等 7 env 不可得；活栈验证覆盖被改读取路径） |
| 安全红线自查 | diff 审查 | 无 SQL 改动（既有参数绑定不动）、无凭据入文件（活栈 key 仅 shell env）、无新外呼路径 |

## 第 4 轮收尾判定（2026-09-28 执行会话）

**通过判据（计划 r5 复验轮）**：

- (a) 三态文案浏览器断言：四腿 21/21 PASS（rv5-01~05 截图；R-19 清偿）—— **PASS**
- (b) 同步面零推进：browser_02 回访+显式刷新仍待付款（AC2）—— **PASS**
- (c) 重放三面（重复 notify 计数不变/重复 webhook 四对象 STRICT-BYTE-IDENTICAL/重启 drain 零二次发放）+ D11 竞态面（paid 窗口重购返回同一订单、渠道侧恰 1 次 PRECREATE）—— **PASS**
- (d) 四对象齐全 + D6' 判据（fee 1320，0<1320≤9900 proration 剪裁披露）；`ac4-sandbox-credentials-unavailable` 残余不变 —— **PASS**
- (e) 红线自查：sk_/whsec_ 字面量、明文口令、force-active 直写、manual payments 调用四项全零 —— **PASS**
- (f) Go 回归 10 包全 ok；architectureguard 0 violations；modulemove 16 manifests —— **PASS**
- (g) web node26 全量 2311 中 2310 PASS（唯一失败为未触碰的 debounce timing 测试，单文件重跑 7/7——负载 flake 披露）；typecheck 5 错=F23 基线 —— **PASS**
- (h) `go vet -tags lago_integration` clean；T9 集成测试实跑尝试如实披露：r5 共享栈 FAIL（webhook 400——测试取 Stripe 列表首个 succeeded PI，在已 finalize 过浏览器轮 PI 的共享栈上属重复应用、被 Lago 正当拒绝——测试的环境形状缺陷，产品侧该 400 正是幂等拒绝的表现）；专属 t11 栈 lab.env 本地文件缺失无法构造 env —— **披露（skip≠pass 纪律）**，等价覆盖由真实栈四腿+四对象+重放三面承担

**第 4 轮 Ruling**：

- **R-27（fulfillment_state 列不存在）**：计划 D11 谓词写「state='paid' + fulfillment_state 非 fulfilled」——仓库层 OrderRow 无 fulfillment_state 列（paid→fulfilled 由 OrderRow.State 承载，order.go MarkFulfilled）。`CurrentPaidAwaitingActivationPurchaseOrder` 谓词收敛为 `state='paid'`（语义等价：fulfilled 行自然不命中）。
- **R-28（sqlite3 CLI 无位置参数绑定）**：Task 16 按计划把 seed SQL 改 `?1` 绑定——真跑实证 homebrew sqlite3 3.51.2 对 shell 调用不绑定（`?1` 落 NULL 被 NOT NULL 拒、额外 argv 被当第二条 SQL）。回退为 A-08 UUID 白名单（^[0-9a-f-]{36}$）+ 内插（r2 建议的等效形态），四份 seed 同步。
- **R-29（重复注册的 400 形态）**：本 worktree 当前后端对重复注册答 `400 + already exists`（r4 轮记录的 409 属当时集成分支）。r5 seed 容忍集扩为 409 或 400-with-marker；r4 三份 seed 为已冻结轮次证据不动。
- **R-30（环境处置）**：OrbStack 重启自动恢复历史 weknora-lago-82flow 栈占 48889/48890——`docker compose down`（无 -v，卷保留，该栈证据已冻结）释放端口；本轮新栈 weknora-lago-82r5。后端 env 补 `WEKNORA_COMMERCIAL_STRIPE_API_KEY=$STRIPE_SECRET_KEY` 映射（gated create 需要它，首次 503 unconfigured 的根因）。
- **R-31（T9 共享栈形状）**：见判据 (h)——T9 测试取「Stripe 列表首个 succeeded PI」在共享栈上会拿到已 finalize 的旧 PI（Lago 正当 400）。修复测试（按本轮锚定 invoice 过滤）属第 3 轮冻结证据面改造，超出本轮计划范围，移交复验。

**明确移交/残余（第 4 轮）**：

1. T9 集成测试的共享栈形状修复（按 gated create 的 invoice 锚定过滤 PI）+ 专属栈 lab.env 重建——下轮或集成会话。
2. AC4 沙箱残余 `ac4-sandbox-credentials-unavailable`（不变，R-4 已披露边界）。
3. #84/#92 已移交项不变（废弃 pending 单回收、cancel 命令、续期收款路由）；D16 不修项不变。

## T9 webhook replay review repair — 2026-09-30

- Fixed the async false-pass window by reading the pinned Lago `inbound_webhooks` row through the local DB container, scoped by organization, source, provider code, and exact Stripe event ID. The test verifies one succeeded baseline row and waits under a bounded context for exactly one additional row to become succeeded before post-state reads.
- Added canonical API snapshots of the exact active purchase subscription and finalized+succeeded invoice from `PaymentIntent.metadata.lago_invoice_id`, all invoice-filtered payments (exactly one succeeded), all customer wallets, and every wallet’s transactions. Paginated collections require `meta.total_count` consistency and are sorted by Lago ID before JSON byte comparison. Existing authority snapshot/payment count assertions remain additional checks.
- Test remains env-gated. No live services were started; T9 replay/AC3 and AC4 remain unverified. Source contract pin: Lago API `591ae9005110346f1c6034ec72ea9046625668cf`.
- Verification and commit evidence: `.superpowers/sdd/issue-72-plan-82-t9-fixture-r1/t9-webhook-replay-fix-task-1-report.md`.

- Fix-round contract correction: Lago PaymentSerializer exposes `invoice_ids`; exact target invoice selection now uses array membership with malformed shape rejection. Additional validator fixes send psql SQL over stdin, parse pagination metadata independently of collection arrays, fail immediately on terminal failed rows, assert the active purchase wallet, and use effective DB labels for the prep organization query. See Task 1 report for fix-round checks and hashes.
