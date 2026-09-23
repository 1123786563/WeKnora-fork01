# Issue #72 子 Issue 清单（Lago 计费迁移）

> 生成：2026-09-23（同日第二次修订：#74 实施完成改判 done；T02/75-a2 两项用户裁决落地，9 票 blocked 全部转 todo，阻塞清零）｜ 分支：`codex/issue-72-lago`
> 来源：33 个子 Issue（#73–#105）逐票调查结果（代码证据为调查会话实查/实跑）；GitHub 状态本次修订会话经 `gh issue list --state all` 全量复测：#73/75/76/77/78/79/80 CLOSED（7 票），#74 及 #81–#105 OPEN（26 票）。
> 父 Issue：[#72](https://github.com/1123786563/WeKnora-fork01/issues/72)（Lago 替换 OpenMeter 迁移 Spec 总票）——只提供总体目标与验收，**不计入 DAG 节点**；33 个子 Issue 均为 #72 直接子票，层级树退化为单层（无嵌套父子）。
> 状态口径：`done`=已有完成证据不重新实施；`blocked`=依赖缺失（外部输入或前置未决），保留原因；`todo`=待实施（前置未齐属正常排期，不算 blocked）。

## 1. 总览

| 编号 | 标题（短） | GitHub | DAG status | 一句话依据 |
|---|---|---|---|---|
| #73 | Lago01 固定版本集成环境 | CLOSED | **done** | 四 AC 有代码+46 测试实跑+t01 证据 |
| #74 | Lago02 外部付款激活实验 | OPEN | **done** | AC1-AC4 运行时证据全 pass+流程验证 21/21（本轮实施交付，见 §2） |
| #75 | Lago03 Wallet 批次语义验证 | CLOSED | **done** | e1-e4 实测+104 测试实跑+verdict（a2 已裁决选项 B） |
| #76 | Lago04 Pricing Group 计价批次 | CLOSED | **done** | 60 测试实跑+6 份真实栈证据（p95 6.893s） |
| #77 | Lago05 Commercial Platform seam | CLOSED | **done** | 冻结 seam+双适配器契约+build/test 全绿 |
| #78 | Lago06 空间独立 Lago Customer | CLOSED | **done** | 三层幂等+租户隔离，6 组测试实跑通过 |
| #79 | Lago07 发布不可变 Plan Version | CLOSED | **done** | 六轴校验+DB trigger 不可变，5 包测试实跑 |
| #80 | Lago08 Base Plan 权益与月度额度 | CLOSED | **done** | 懒链+配额 guard+迁移 000183，9 测试实跑 |
| #81 | Lago09 Quote→待付款 Invoice | OPEN | **todo** | 三前置全满足（#74 done、#78/#79 done），T02 已裁决② |
| #82 | Lago10 支付宝恰好一次激活 | OPEN | **todo** | T02 已裁决②；待 #81 交付可激活实体 |
| #83 | Lago11 微信复用激活流程 | OPEN | todo | 渠道层已实现，Lago 激活链路复用待 #82 |
| #84 | Lago12 异常付款不扩大权益 | OPEN | todo | 本地校验已有，Lago Payment 路径待 #82/#83 |
| #85 | Lago13 购买 Credits 付款到账 | OPEN | **todo** | 75-a2 已裁决选项 B（协调层承载），待 #82/#83 |
| #86 | Lago14 额度按到期顺序消费 | OPEN | todo | 月度维度可先行；充值维度待 #85 |
| #87 | Lago15 收费前原子预占 | OPEN | **todo** | 75-a2 悬置已解除，待 #86（唯一实质前置） |
| #88 | Lago16 Usage Event 批次核对 | OPEN | **todo** | 外部输入已全解除，待 #87（调查自述"依赖解锁后即可按 todo 实施"） |
| #89 | Lago17 并发/委派共享 Task Budget | OPEN | todo | 本地机制为旧域先期成果，Lago 语境待 #87 |
| #90 | Lago18 计价延迟/超上界暂停 | OPEN | todo | 5/15 分钟门槛与超上界检查全缺，待 #88 |
| #91 | Lago19 BYOK 只免模型维度 | OPEN | todo | 语义层已有，Lago 端可观察性待 #88 |
| #92 | Lago20 用量修正不改写历史 | OPEN | todo | 本地 append-only 已有，补偿/Credit Note 待 #88 |
| #93 | Lago21 升级补发当月差额 | OPEN | todo | 本地域完整，Lago 切换/匹配待 #82/#86 |
| #94 | Lago22 降级/年付月发/到期回 Base | OPEN | **todo** | 上游硬阻塞点已解除，待 #93（共用订阅链切换协调器） |
| #95 | Lago23 充值退款三段式 | OPEN | todo | 骨架完整，P03/Lago void 待 #82/#85/#86 |
| #96 | Lago24 套餐退款 Credit Note | OPEN | todo | Credit Note 全链路零实现，待 #92/#93 |
| #97 | Lago25 微信退款对齐支付宝 | OPEN | **todo** | 原 blocked 仅因前置 OPEN（已编码为依赖边），待 #83/#95/#96 |
| #98 | Lago26 Webhook+对账收敛 | OPEN | todo | 消费端/迁移/worker 全缺，待 5 前置 |
| #99 | Lago27 崩溃/Lago 故障不丢用量 | OPEN | todo | 幂等基础已有，Lago 链路与注入矩阵待 #90/#92 |
| #100 | Lago28 Billing Center 统一展示 | OPEN | **todo** | 原 blocked 仅因 5 前置 OPEN（依赖边），待 #91/#94/#97/#98/#99 |
| #101 | Lago29 数据最小化/AGPL 门槛 | OPEN | todo | 审计未执行、法务结论缺失，待 #97/#99 |
| #102 | Lago30 空间注销停收费 | OPEN | todo | 注销与商业域零集成，待 #94/#96/#98/#101 |
| #103 | Lago31 生产 Helm+延迟目标 | OPEN | **todo** | AGPL 前置门即 #101 交付物（依赖边），待 #98/#99/#101 |
| #104 | Lago32 备份恢复+商业对账 | OPEN | todo | 工具链/演练全缺，待 #102/#103 |
| #105 | Lago33 切换 Lago 移除 OpenMeter | OPEN | todo | 收官票，9 个 blocker 全 OPEN |

统计：done 8 ｜ blocked 0 ｜ todo 25（合计 33）。

状态变更记录（2026-09-23 第二次修订，三组）：
1. **#74：todo → done（本轮实施交付）**。下发指令曾要求 #74 判 todo（剩余交付=以 Stripe TEST 密钥重跑 `deploy/lago-lab/payment-activation/run_lab.py` 收集 AC1-AC3 运行时证据）；本次修订会话核实该交付**已在集成分支完成**：`git log` 实查 9ff29b7ec（09-23 13:31，real-stack t02 runtime evidence with Stripe TEST key，AC1-AC4）→ 812bb241d（promote 证据+DECISION 更新）→ 0aa976c64（契约 verdict 钉回归测试）→ 5ccdc7f3a（ledger final，evidence complete/secrets clean）→ merge `175de8b4f`（14:08）→ 905ba19b7（flow evidence）；`docs/plans/issue-72-flow-evidence-74/VERIFY-SUMMARY.txt` 本会话读取：run_id 3dc51207、退出码 0、九阶段全 pass（gate[AC1]/activate[AC2]/duplicates[AC2]/retries[AC3]/manual[AC4]/decline_control 负对照）、逐 AC 断言 **21/21 PASS**、密钥扫描 16 文件 0 命中；最终报告 `issue-72-final-report.md` §1/§2.1 将 #74 记为本轮唯一实施完成票。按"已有完成证据的节点标 done，不重新实施"改判 **done**。残留两项均不重开验收：①GitHub 关票（本会话 `gh issue view 74` 实测仍 OPEN，属编排侧动作，建议随下轮编排执行）；②OCR 第 4 轮 4 项 medium+1 项 low findings 未修复（最终报告 §9.2 已定级记录，属治理遗留）。
2. **用户裁决落地（2026-09-23，两项）→ #81/#82/#85 由 blocked 改判 todo**：①T02 三选一裁决为**选项②（受支持 Provider；Stripe TEST 已在 #74 实证）**——#81/#82 的裁决型阻塞解除，依赖边不变（#81 前置 #74/#78/#79 现全部满足，成为**当前唯一可立即开工节点**）；②75-a2 裁决为**选项 B（充值批次状态机由 WeKnora 协调层承载、Lago Wallet 交易仅在到账时刻幂等创建）**，ADR-0012 已在集成分支追加修订（提交 552d98d12，本会话 `git log` 实查存在）——#85 裁决型阻塞解除，依赖 #82/#83 不变。
3. **派生解锁 → #87/#88/#94/#97/#100/#103 由 blocked 改判 todo**：上述两裁决+#74 完成后，DAG 已无任何外部输入缺口；这六票原 blocked 理由只剩"前置 Issue OPEN"（已编码为依赖边，属正常排期等待）与已解除的 #74/#75-a2/T02 悬置，故一并转 todo（#88 调查原文亦自述"依赖解锁后即可按 todo 实施"）。blocked 由此清零。

## 2. 逐票清单

### issue-73 ｜ [Lago 01] 启动固定版本的 Lago Community 集成环境
- URL: https://github.com/1123786563/WeKnora-fork01/issues/73 ｜ 父节点: #72 ｜ GitHub: CLOSED
- 需求依据：一条受支持命令启停 pinned v1.53.0 Lago Community（PG event store，无 Kafka/ClickHouse），读取分类健康（API/worker/db/redis 而非仅 HTTP 存活），最小真实契约探测（创建+清理隔离对象、脱敏证据），与 OpenMeter 的 db/Redis/Kafka/ClickHouse 完全隔离；产出 Community 边界清单与 AGPL 审查输入。
- 实现现状：已实现并验证。`deploy/lago/lago.sh`、`compose.yaml`（5 镜像 OCI digest 锁定）、`images.lock.json`、`health.py`（分类健康+overall ready/degraded/unavailable）、`contract_probe.py`（唯一合成 Customer+finally 清理）；调查会话实跑 `python3 -m pytest deploy/lago/ -q` → 46 passed + 62 subtests；证据 `evidence/t01-images.txt`（5/5 MATCH）、`t01-health.json`（ready）、`t01-contract.json`（pass）；probe cleanup 宽容性 carryover 已由 b57be1602 修复并有测试。
- 验收标准：①命令启停+版本/摘要可审计 ②分类健康 ③最小契约探测+清理+脱敏 ④OpenMeter 零读写。
- 依赖：前置无；下游 #74/#76/#77。
- 处理决定：**done**——四 AC 均有代码+实跑测试+运行时证据；AGPL 审查与 Community 能力矩阵按 spec 属整体迁移 completion gate，不归本票。

### issue-74 ｜ [Lago 02] 证明外部付款可以激活 payment-gated Subscription
- URL: https://github.com/1123786563/WeKnora-fork01/issues/74 ｜ 父节点: #72 ｜ GitHub: OPEN（关票待编排侧执行）
- 需求依据：在 pinned v1.53.0 隔离栈中通过受支持外部付款接入登记 Payment，观察 Subscription 恰好一次 active、Entitlement 可用；决定 #81/#82 采用 manual Payment 还是 Lago 支持的外部 Payment Provider。
- 实现现状：**已实现并完成运行时验证（本轮实施交付）**。实施链：per-issue 分支 `codex/issue-72-lago-74` 11 轮真实运行（1 轮 blocked-env+9 轮 fail 暴露工具契约缺陷+run11 `5d06a277` 九阶段全 pass），主 Agent 裁决接受计划偏差后 merge `175de8b4f` 并入集成分支；流程验证员独立复跑（run `3dc51207`，21/21 逐 AC 断言+DB-WATCH PASS，`docs/plans/issue-72-flow-evidence-74/`）；OCR 4 轮审查（1-3 轮 16 项有效 findings 全部清偿并重放真实流程，第 4 轮遗留 4 项 medium+1 项 low）。落地证据：`docs/migrations/lago/t02-payment-activation/`（8 JSON+DECISION.md，AC1-AC4 pass）、`issue-72-ledger-74.md`、离线回归 64 passed（基线 60+新增 4 fail 侧回归）。本会话实查：`git log` 提交链在案、VERIFY-SUMMARY.txt 与 t02-{activation,duplicates}.json（status=pass）读取确认。
- 验收标准：①付款前 incomplete/Entitlement 不可用 ②登记后恰好一次 active、重复不重复激活 ③响应丢失/超时/重试同身份可恢复 ④manual 不能激活时给出替代路径或 blocker。
- 依赖：前置 #73（已满足）；下游 #81/#82（blocking，timeline 交叉证实）。
- 处理决定：**done**（2026-09-23 第二次修订由 todo 改判）——下发指令所列剩余交付（重跑 run_lab.py 收 AC1-AC3 运行时证据）经本会话核实已完成并经独立流程验证，按"已有完成证据不重新实施"收口；T02 裁决（选项②）亦已基于该证据作出。残留：GitHub 关票（编排侧）与 OCR-4 治理 findings（4 medium+1 low，最终报告 §9.2），均不重开本票验收。

### issue-75 ｜ [Lago 03] 证明 Wallet 批次到期、消费顺序和撤回语义
- URL: https://github.com/1123786563/WeKnora-fork01/issues/75 ｜ 父节点: #72 ｜ GitHub: CLOSED
- 需求依据：用真实 Lago Wallet 与 traceable transactions 验证已批准 Credits 不变量：套餐额度按月到期不结转、充值批次到账起 12 个月到期、多批次按最早到期/同到期最早发放消费、并发与重试不重复扣减不透支、void/refund 只撤未消费部分；任一不变量无法保持则明确阻断迁移。
- 实现现状：已实现（验证型交付，非生产代码票）。`deploy/lago-lab/wallet-semantics/`（lab.sh/harness.py/e1-e4/2146 行预注册断言测试，调查会话实跑 104 passed）；`docs/migrations/lago/t03-wallet-semantics/verdict.md` 判定 a1/b/c/d 均 PASS-WITH-COORDINATION，**a2 十二个月批次到期 BLOCKED**（第 7 个活跃钱包实测 422 wallet_limit_reached，Lago 硬上限 6），须协调层承载。
- 验收标准：四条不变量判定（a1/a2/b/c/d）+ 阻断规则执行。
- 依赖：前置无 DAG 边（仅父 #72 提供不变量定义）；下游 #85/#86。
- 处理决定：**done**——全部验收有真实栈证据+实跑测试。遗留的 a2 设计决策**已于 2026-09-23 裁决为选项 B**（协调层承载批次状态机、Lago Wallet 交易到账时刻幂等创建；ADR-0012 修订 552d98d12），解锁 #85 起充值链；协调层义务清单属 #85/#86 等后续票范围。

### issue-76 ｜ [Lago 04] 证明 Task Pricing Group 可以形成可核对计价批次
- URL: https://github.com/1123786563/WeKnora-fork01/issues/76 ｜ 父节点: #72 ｜ GitHub: CLOSED
- 需求依据：向真实 Lago v1.53.0 投递带 Task/Pricing Group 的 Usage Event，贯通接收→可查询→current usage 聚合→批次金额核对，验证 subscription-per-task 模型；解除 vendor blocker『Task/Pricing Group rating latency and cardinality』。
- 实现现状：已实现。`deploy/lago-lab/pricing-group/`（fixture/lago_client/run_phase/measure，调查会话实跑 60 passed）+ `docs/migrations/lago/t04-pricing-group/` 6 份 JSON 证据（A0 reconciled:false、422 幂等、64/64 all_reconciled_exact、p95 6.893s、基数 65 订阅）；4 处实测契约修正已反哺 `internal/modules/commercial/commercialplatform/lago.go:360-399`。
- 验收标准：①acceptance≠rating 完成 ②重复事件不重复计量/身份冲突拒绝 ③current usage 与 fixture 精确核对 ④p50/p95/峰值吞吐/基数记录。
- 依赖：前置 #73（已满足）；下游 #87/#88/#103。
- 处理决定：**done**——两处产品化缺口（422 不可区分、唯一性三元组作用域）已移交 OPEN 的 #87/#88 跟踪，属后续票范围。

### issue-77 ｜ [Lago 05] 用 Lago readiness 纵向切片扩展 Commercial Platform seam
- URL: https://github.com/1123786563/WeKnora-fork01/issues/77 ｜ 父节点: #72 ｜ GitHub: CLOSED
- 需求依据：旧 OpenMeter Gateway 仍可运行前提下，建立深的 provider-neutral Commercial Platform 模块（唯一供应商 seam），受保护 Billing API 操作经 seam 读真实 Lago readiness/version 快照；为 W3 冻结接口。
- 实现现状：已实现。冻结 seam `internal/modules/commercial/platform.go:221-230`（SubmitCommand/ReadSnapshot/Reconcile 三方法族）；fake+lago 双适配器同表契约 `commercialplatform/contract_test.go`；`GET /commercial/platform/readiness` 封闭信封；调查会话实跑 `go build ./...`（exit 0）、`go test ./...`（126 包 ok）及关键包点名测试全 PASS；真实栈证据 t05-health.json/t05-run.txt。旧 Gateway 保留（移除归 #105）。
- 验收标准：①只见封闭产品信封 ②fake/Lago 同一契约 ③旧 Gateway 路径仍运行主干绿色 ④三族接口无逐对象浅封装。
- 依赖：前置 #73；下游 #78/#79（seam 冻结+ADR-0014 加法规则）。
- 处理决定：**done**——仅 ADR-0014 路径表述过时的小文档漂移（模块化 pass A 后路径变化），不影响完成判定。

### issue-78 ｜ [Lago 06] 一个空间自动获得独立 Lago Customer
- URL: https://github.com/1123786563/WeKnora-fork01/issues/78 ｜ 父节点: #72 ｜ GitHub: CLOSED
- 需求依据：创建空间或首次访问计费功能时，幂等建立空间 Billing Account 与唯一 Lago Customer 映射，Billing API 显示 linked/pending 封闭信封状态。
- 实现现状：已实现。`ExternalCustomerID=weknora-tenant-<十进制ID>` 纯函数（platform.go:159）；三层幂等（DB 唯一约束+command key+read-before-create）；租户全 WHERE 隔离；失败保留 pending；`GET /api/v1/commercial/account` 懒 ensure；迁移 000181/000102；调查会话实跑 service/repository/router/双适配器契约/seam 6 组 go test 全 PASS；真实栈证据 t06-account.txt。
- 验收标准：①并发只建一个 Customer ②跨空间隔离 ③改名/Owner 转移不改身份 ④失败/响应丢失按原身份恢复。
- 依赖：前置 #77；下游 #80/#81。
- 处理决定：**done**——残余（BillingPage 未消费 /account、US-59 名称刷新、PG twin 无 CI 环境）均验收外且已声明移交（统一展示归 #100）。

### issue-79 ｜ [Lago 07] 从 WeKnora 管理后台发布不可变 Plan Version
- URL: https://github.com/1123786563/WeKnora-fork01/issues/79 ｜ 父节点: #72 ｜ GitHub: CLOSED
- 需求依据：套餐运营人员在 WeKnora 创建草稿、校验商业约束并发布新的 Lago Plan Version；发布回执与不可变映射可从管理面读取。
- 实现现状：已实现。六轴校验（catalog.go:197-218）、确定性 plan code `weknora-<slug>-v<n>`+幂等发布（service planversion.go:181-198）、双方言 DB trigger 不可变（迁移 versioned/000182、sqlite/000103）、发布零订阅写（byte-stable）；Admin API `/admin/plans/*`；调查会话实跑 5 个相关 Go 包测试全 PASS、`go build ./...` exit 0；真实栈证据 t07-run.txt。
- 验收标准：①六轴 fail-closed 校验 ②独立 plan code+发布幂等 ③已发布不可原地修改 ④已有 Subscription 不变。
- 依赖：前置 #77；下游 #81。
- 处理决定：**done**——gaps 为低危外围项（Web UI 未接线 router.tsx:704 未传 props、迁移编号漂移 000179→000182、5 Minor 不可追溯、真实栈未复跑）。

### issue-80 ｜ [Lago 08] 新空间以 Base Plan 获得权益和月度额度
- URL: https://github.com/1123786563/WeKnora-fork01/issues/80 ｜ 父节点: #72 ｜ GitHub: CLOSED
- 需求依据：新空间经同一 Subscription 链（#78 ensure_customer → 本票 ensure_subscription）进入 Base Plan，Billing 面看到生效权益、Resource Quota 与月度套餐内 Credits；配额超限只阻新增；租户隔离；重复初始化不重复发放。月度额度=每自然月短 TTL 钱包、协调者幂等发放（T03 verdict 义务）。
- 实现现状：已实现。`subscription_command.go`（ensure_subscription+grant_included_credits）、`service/commercial/benefits.go` 懒链（seed→account→subscription→grant→projection）、registry+迁移 000183/000104、`GET /commercial/account` benefits 段、配额 guard 三处接线（knowledge_create.go:132,394,639）；调查会话实跑 9 个点名测试全 PASS；真实栈证据 t08-run.txt（6 阶段）。
- 验收标准：①重复初始化恰好一次 ②Base Tier 决定功能与上限 ③超限只阻新增 ④租户完全隔离。
- 依赖：前置 #78；下游 #86/#87/#94（#94 票面亦声明 blocked by #80，经 80→93→94 与直连边 80→94 入图）。
- 处理决定：**done**——遗留（concurrent_tasks 执法、manual/passage guard 缺口、前端展示）已显式移交 OPEN 的 #87/#88/#100。

### issue-81 ｜ [Lago 09] 从 Quote 创建待付款 Invoice 和 incomplete Subscription
- URL: https://github.com/1123786563/WeKnora-fork01/issues/81 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：账单管理员 Checkout 选定 Plan Version 获得有效 Quote；系统创建 Lago incomplete Subscription 与待付款 Invoice，Quote 与 Invoice 逐项匹配（版本/币种/总额/行项）后才创建 Channel Payment Order；付款前 Entitlement 不开放；过期/并发变更/重试不产生重复。
- 实现现状：未实现（核心交付零代码，无 lago-81 账本）。可复用基础：#80 幂等订阅创建算法（lago.go:587-637，subscriptionIndexStatuses 显式含 incomplete）、本地 CreateQuote/CreateOrder/Quote.ValidateForUse、前端 CheckoutPage。缺口：payment-gated 订阅命令（ensure_subscription 是免费 Base Plan，lago.go:685-686 不带 activation_rules）、Lago Invoice 创建与 Quote↔Invoice 匹配、待付款产品状态、旧本地商业表退役。
- 验收标准：①Quote 固定版本/金额/权益/行项/过期 ②不一致不创建支付请求 ③待付款且 Entitlement 未开放 ④过期/并发/重试不重复。
- 依赖：前置 #74（**done**，T02 裁决②已作出）/#78（done）/#79（done）——**三前置全部满足，当前唯一可立即开工节点**；下游 #82。
- 处理决定：**todo**（2026-09-23 由 blocked 改判）——T02 三选一已裁决为选项②（受支持 Provider，Stripe TEST 已在 #74 实证），裁决型阻塞解除；按选项②实施：payment-gated incomplete Subscription 走 Lago 支持的外部 Provider 通道，禁止本地强制 active。

### issue-82 ｜ [Lago 10] 支付宝付款后恰好一次激活套餐
- URL: https://github.com/1123786563/WeKnora-fork01/issues/82 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：可信 Payment Fact 写入 Lago，仅当 Subscription 观察为 active 才开放 Entitlement 与套餐 Credits；状态『待付款→已付款待激活→已生效』；同步返回页不确认付款；重复回调/Outbox 重放/响应丢失不重复履约；真实支付宝沙箱证据关联四对象。
- 实现现状：部分实现（仅渠道侧与读侧）：`payment/alipay.go` Verify（验签/app_id/seller_id/金额）+VerifySyncReturn 永不确认（AC2 渠道层满足）、`handler/payment_callbacks.go` 回调入口、`repository/commercial/order.go` ConfirmPayment 单事务（版本守卫/幂等/晚成功审计）、outbox 存储原语、benefits 快照仅 active 开放。缺口：record_payment 类 Lago 命令（seam 无）、payment-gated 订阅创建、三态映射（契约仍 pending|paid|closed）、恰好一次激活编排、支付宝沙箱证据目录。
- 验收标准：①三态依次呈现 ②同步返回页不能确认 ③重复回调/重放/响应丢失不重复履约 ④真实沙箱证据关联 Invoice/Payment/Subscription/Credits。
- 依赖：前置 #74（**done**，裁决②）/#81（todo）；下游 #83/#84/#85/#93/#95。
- 处理决定：**todo**（2026-09-23 由 blocked 改判）——录入通道已裁决为选项②受支持 Provider；前置对象（待付款 Invoice+incomplete Subscription）由 #81 交付后即可开工。

### issue-83 ｜ [Lago 11] 微信付款复用同一激活流程
- URL: https://github.com/1123786563/WeKnora-fork01/issues/83 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：微信支付复用已验证的 Quote/Invoice/Payment Fact/Lago activation/状态机，保留微信独立签名、商户、查单、关单与通知语义；真实受控环境验收，不以支付宝证据替代。
- 实现现状：部分实现。渠道 Provider 层（wechat.go Verify：serial/时钟窗口/RSA-SHA256/AES-GCM/mchid-appid；Create/Query/Close/Refund L376-478）与渠道无关确认链路（ConfirmPayment 幂等单事务、Recover 查单恢复）已实现，调查会话实测 TestWechat 系/ConfirmPayment 幂等并发/Recover 测试 PASS。缺口：Lago 激活链路复用（依赖 #82）、WechatProvider.Close 无调用方（超时关单未接线）、Create/Query/Close 无单测、无微信真实受控环境栈。
- 验收标准：①验签/商户/金额/币种通过才形成 Fact ②漏通知查单恢复、重复通知不重复激活 ③关单与晚成功竞态只履约一次 ④微信真实环境验收。
- 依赖：前置 #82；下游 #84/#85/#97。
- 处理决定：**todo**——范围明确；核心『复用同一激活流程』须 #82（及其前置 #74/#81）先建成。

### issue-84 ｜ [Lago 12] 异常付款不会扩大权益
- URL: https://github.com/1123786563/WeKnora-fork01/issues/84 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：重复、错金额、错币种、部分付款、晚成功与多 Attempt 成功映射为明确用户/运营状态：异常资金事实记录但不修改 Invoice/不扩大权益；多个成功只履约一次、其余进多收款处置；重复通知幂等且不重复写 Lago Payment；UI 区分付款异常/处理中/已生效。
- 实现现状：部分实现。ValidatePayment+ConfirmPayment 金额币种校验、版本守卫单次履约、sameTxn 幂等重放（测试实跑通过）。缺口：异常回调 409 后外部事实未落库（spec L127 要求保留）、RecoverOrderStatus 未与渠道实收比对、over_payment outbox 事件无消费方、Lago Payment 写入路径不存在、orderWire 永不输出 attention/前端无付款异常分支。
- 验收标准：①错金额/币种/部分付款不激活 ②多成功只履约一次 ③重复通知幂等 ④UI 区分三态。
- 依赖：前置 #82/#83；下游 #98。
- 处理决定：**todo**——本地订单域已有基础，核心范围待 #82/#83 完成后实施。

### issue-85 ｜ [Lago 13] 购买 Credits 并在付款后到账
- URL: https://github.com/1123786563/WeKnora-fork01/issues/85 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：账单管理员选充值商品、确认 Quote、微信或支付宝付款；Lago 在付款确认后恰好一次创建可消费 Wallet Credits；到账延迟可观察可恢复；付费充值在 Lago 确认 Wallet transaction 前不可消费；批次到账起 12 个自然月到期（spec L126/L131/L169/L219）。
- 实现现状：部分实现（与旧 OpenMeter 路径共享的框架已实测通过）：OrderKindPurchase/FulfillmentKey 幂等身份/TopUpOrderLines/processRecord 先查后应用/前端终态轮询与 12 个月文案。缺口：seam 无 top-up 命令（grant_included_credits 是月度赠送非充值）、CommercialGateway 仍绑 openmeter（container.go:244）、前端硬编码 pro 订阅+wechat、到账来源/到期明细缺失。
- 验收标准：①未付款不增可消费 Credits ②恰好一次到账+来源与到期 ③处理中状态 ④响应丢失复用原身份恢复。
- 依赖：前置 #75（done，a2 已裁决 B）/#82/#83；下游 #86/#95/#98。
- 处理决定：**todo**（2026-09-23 由 blocked 改判）——75-a2 已裁决为**选项 B**：充值批次状态机由 WeKnora 协调层承载、Lago Wallet 交易仅在到账时刻幂等创建（ADR-0012 修订 552d98d12），批次数据模型可定稿；待 #82/#83 交付付款链后实施。

### issue-86 ｜ [Lago 14] 套餐额度与充值额度按到期顺序消费
- URL: https://github.com/1123786563/WeKnora-fork01/issues/86 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：余额页面展示套餐/充值/即将到期/预占/退款锁定分解；真实收费严格按最早到期、同到期最早发放选批次；套餐月额度不结转；充值跨月保留至 12 个月到期；并发不重复消费；与 Lago traceable state 对账一致。
- 实现现状：部分实现。月度短 TTL 钱包（priority=1、到期=期末）、批次读回（benefitsWire credits.batches）、过期 overlay 归零、本地最早到期预占（测试实跑 PASS）。缺口：充值 12 个月维度（top-up wallet 在 lago.go:1010 仅为 future 预期）、混合到期秩编码（T03 verdict 义务）、预占/退款锁定无暴露端点（contracts 注释明示）、前端无余额分解、无对账流程。
- 验收标准：①月度不结转、充值 12 个月 ②统一最早到期消费 ③并发/到期任务不重复 ④对账一致。
- 依赖：前置 #75/#80/#85；下游 #87/#93/#95/#105。
- 处理决定：**todo**——月度消费顺序接线与余额分解展示可先行（不依赖 #85）；充值维度待 #85（a2 裁决 B 已定协调层承载口径）。

### issue-87 ｜ [Lago 15] 收费调用前原子预占 Task Budget 与空间 Credits
- URL: https://github.com/1123786563/WeKnora-fork01/issues/87 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：Task Owner 设有限 Task Budget；每次真实收费动作前按同一 Lago 价格版本保守上界，原子检查 Task Budget 与空间保守 Credits 余额并预占；拒绝不派发；分别展示与限制；价格投影缺失/过期/不可计算 fail closed；重试不重复预占（spec L131-137、测试矩阵 12/13/14）。
- 实现现状：部分实现——本地原子预占完整且有测试（Begin 先 Reserve、双 CAS guarded UPDATE、幂等重放、fail-closed 价格解析、上界=(MaxIn+MaxOut)×费率；7 包测试实跑 ok）。缺口：上界投影基于本地 PriceVersionRates 非 Lago 价格版本只读投影（seam 无 pricing/wallet-admission kind）、空间余额仍本地 BudgetAccountRow 权威（待 #86）、BillingPage 无空间余额字段、remote_usage.go:99-100 缺版本回退默认 remote-v1 违 AC3 精神、PG 并发测试 fixture 漂移（budget_pg_test.go:56 缺 000161 owner 列，TestBudgetPGConcurrentReservation 等 2 测试 FAIL）。
- 验收标准：①预占先于真实调用 ②预算/余额分别展示 ③价格投影 fail closed ④重试不重复预占（+矩阵 12/13/14）。
- 依赖：前置 #76（done）/#80（done）/#86；下游 #88/#89。
- 处理决定：**todo**（2026-09-23 由 blocked 改判）——原 blocked 两依据中"#86→#87→#88 链被 75-a2 悬置"已随裁决 B 解除，剩"#86 OPEN"为正常依赖边排队；#86 交付后即可开工。

### issue-88 ｜ [Lago 16] 一个 Usage Event 完成 Settlement Batch 核对
- URL: https://github.com/1123786563/WeKnora-fork01/issues/88 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：已预占的收费调用产生不可变 Usage Fact，经 Outbox 投递进 Lago（transaction_id 全局稳定身份）；acceptance 仅证接收、预占保留；current usage 出现确定权威金额后 capture 实际并释放差额；重复投递幂等、响应丢失可重放、同身份异内容判冲突（spec §Usage, rating, and reconciliation）。
- 实现现状：未实现（Lago 侧核心为零）。旧 OpenMeter 路径有近似语义且测试绿（不可变 Usage Fact、per-revision 幂等键、Finalize 单事务、accepted≠confirmed、unknown 重放、内容冲突拒绝）；但 seam 无 usage 命令/快照、lago.go 无 events/current_usage、全库无 SettlementBatch/PricingGroup 产品代码、生产仍装配 openmeter（container.go:244）。#76 T04 证据为直接输入。
- 验收标准：①Fact/Lago transaction/批次端到端可追踪 ②acceptance 后预占仍存在 ③确定 rating 后 capture+释放差额 ④重复/丢失/冲突稳定结果。
- 依赖：前置 #76（done）/#87；下游 #90/#91/#92。
- 处理决定：**todo**（2026-09-23 由 blocked 改判）——调查原文自述"范围无歧义、依赖解锁后即可按 todo 实施"；原 blocked 的两项用户输入（#74 密钥/T02、#75-a2）均已落地，剩 #87 前置排队。

### issue-89 ｜ [Lago 17] 并发和委派共享同一 Task Budget
- URL: https://github.com/1123786563/WeKnora-fork01/issues/89 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：多 worker、子 Agent 与 Connector 同时竞争最后 Credits 时共享父 Task Budget 与空间余额，总获准上界不超支（spec 验收门槛 12 Concurrent admission、L133-134）。
- 实现现状：部分实现（均为 Lago 迁移前旧商业域先期成果 a89920d6b，早于 Lago spec，spec:241 列为 prior art）：并发 CAS 预占（account/task/lot 三层）、父子共享（AttachChildRun 零 limit 指向 root）、(tenant,key) 幂等/异内容冲突、生产接线（execution/workbench/agentruntime/appconnector）、SQLite 11 测试+PG 3 并发测试。缺口：余额投影未接 Lago Wallet 权威对账（属 #87）、PG 测试未在 Lago waves 证据链下运行、无 #89 账本/实验栈。
- 验收标准：①最后额度竞争最多一个获准 ②父子/Connector 不复制预算 ③重试幂等/不同调用不同身份 ④真实事务库高并发不变量。
- 依赖：前置 #87；下游 #105。
- 处理决定：**todo**——范围无歧义；Lago 语境验证待 #87 交付权威余额对接后实施。

### issue-90 ｜ [Lago 18] 计价延迟、未知结果和费用上界违规正确暂停 Task
- URL: https://github.com/1123786563/WeKnora-fork01/issues/90 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：异步计价期间保持预占；5 分钟未核对暂停当前 Task 并拒绝新收费；15 分钟或 events.errors 产生运营告警；未知结果不释放预占、其他安全 Task 可继续；实际费用超预占上界必暂停并保留用量（User Story 22-26，W11）。
- 实现现状：部分实现（仅 AC3『不释放预占』半边）：KeepProtection 非 confirmed 全保护、unknown 保留待核对、显式确认才推进 watermark（测试实跑 ok）。缺口：5/15 分钟门槛、Task 暂停状态机、后台核对轮询、Finalize 无 delta>upper 比较（budget_settlement.go:60-74 注释显式假设 upper≥delta）、events.errors 告警、waiting-for-billing-sync 产品态。
- 验收标准：①5 分钟暂停+拒新收费 ②15 分钟/events.errors 告警 ③未知不释放、他 Task 可继续 ④超上界必暂停。
- 依赖：前置 #88；下游 #99。
- 处理决定：**todo**——依赖 #88 属 waves DAG 正常前置排队（W10→W11），非异常阻塞。

### issue-91 ｜ [Lago 19] BYOK 只免除模型维度 Credits
- URL: https://github.com/1123786563/WeKnora-fork01/issues/91 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：同一 Task 组合平台模型/BYOK/解析/沙箱/Connector 用量；服务端资金来源逐维度决定进入 Lago 的收费事件；客户端不能伪造资金来源；模型失败不静默回退平台凭据。
- 实现现状：部分实现（语义层，来自 Semantica A03/craft O01-O02，非 Lago W11 执行）：豁免谓词（BillableModel 仅 platform）、RecordRawModelUsage BYOK 零扣费不出 settlement outbox、ValidateFunding 词表、TrustedUsageFact 防伪造、semantic/craft 双网关 funding 服务端签入、上游不可用 fail closed（调查会话实跑相关测试全 ok）。缺口：Lago 端事件过滤可观察性（依赖 #88 的 Settle 链路——LagoAdapter 无 Settle）、无 byok 实验栈、前端无 BYOK 展示（归 #100）、craft 网关 funding 声明强度需复核。
- 验收标准：①BYOK 维度零模型 Credits ②其他维度照常计费 ③不能伪造资金来源 ④失败不回退平台凭据。
- 依赖：前置 #88；下游 #100/#105。
- 处理决定：**todo**——关键验收『逐维度决定进入 Lago 的收费事件』需 #88 交付 Lago 结算链路后才可观察。

### issue-92 ｜ [Lago 20] 用量修正不会改写历史
- URL: https://github.com/1123786563/WeKnora-fork01/issues/92 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：迟到/更正的 Usage Fact 以新 revision 追加、原 fact 永不覆盖删除；Invoice finalized 前用经验证的 metric 补偿事件调整，finalized 后用 Credit Note 或追加 Invoice；不支持的 Metric 修正进人工核对；可追踪可审计；同一 identity 重试复用、真修正新 identity、同 identity 异内容冲突（spec L142/L149/L229）。
- 实现现状：部分实现（本地 append-only 基础）：UsageFact Revision+UNIQUE、同 revision 异内容冲突拒绝、指针仅前进、负冲销独立 SettlementKey（测试实跑 ok）。缺口：seam 无补偿事件/Credit Note kind、lago.go 无实现、finalized 状态读取与不重算保护、人工核对队列、contracts/前端零修正端点。
- 验收标准：①原 Fact 永不覆盖 ②补偿事件可追踪 ③不支持 Metric 进人工核对 ④finalized 不重算、Credit Note/追加 Invoice 可审计。
- 依赖：前置 #88；下游 #96/#99/#105（+waves 记录 92→102，经 96/98 传递覆盖未直连）。
- 处理决定：**todo**——补偿事件须挂在 #88 的 Settlement Batch 核对链路上。

### issue-93 ｜ [Lago 21] 升级立即生效并补发当月 Credits 差额
- URL: https://github.com/1123786563/WeKnora-fork01/issues/93 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：确认升级 Quote 并付款后，同一 Subscription 身份立即切换新 Plan Version，只补发当前周期剩余时间对应 included Credits 差额（floor 到 credit 精度、随当月到期）；重试不重复补发、不重置锚点（spec L42/L112/L121）。
- 实现现状：部分实现（本地域完整，调查会话实跑 TestChangePlan|TestFulfillmentUpgrade|TestProrate 全 PASS）：proration 升级 Quote、prepareUpgrade 幂等 marker+version-guard 保留 anchor/paid_until、POST /commercial/plans/change。缺口：Lago plan 切换（lago.go:611-612 不同 plan code 直接冲突报错）、grant 走 OpenMeter 非 Lago Wallet、Quote↔Lago Invoice 匹配、升级命令 kind、前端/契约无升级入口、无 lago-93 账本与实验栈。
- 验收标准：①Quote-Invoice 精确匹配 ②付款+activation 后生效 ③补发/到期/舍入符合版本 ④重试不重复补发。
- 依赖：前置 #82/#86；下游 #94/#96/#98。
- 处理决定：**todo**——两个声明依赖均 OPEN；waves 编排排 W9 待前置。

### issue-94 ｜ [Lago 22] 降级、年付月发和到期回 Base Plan
- URL: https://github.com/1123786563/WeKnora-fork01/issues/94 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：降级在已付周期边界后恰好切换一次、不追回已发当月 Credits；年付 included Credits 每月恰好发放一次；到期未续费经同一 Subscription 链回 Base Plan、不清未到期充值额度与数据；超限只阻新增资源（US10/13/14/30、矩阵 17）。
- 实现现状：部分实现（两层）：旧本地模型完整有测试（ScheduledPlanChange、LifecycleService.Tick 周期边界恰一次/逐月幂等/到期投影回 BaseTier、仓储原子守卫；调查会话实跑 ok）但无生产装配且走本地权威；Lago 侧仅 #80 基座。缺口：切换命令 kind、年付月发（EnsureMonthlyCredits 仅 BasePlanSeed 1 credit 不读付费套餐）、LifecycleService 无装配点/tick worker、到期回 Base 的 Lago 链路、真实栈证据。
- 验收标准：①降级周期边界切换 ②年付月发恰一次 ③到期回 Base 不清数据 ④超限只阻新增（+矩阵 17）。
- 依赖：前置 #80（done）/#93；下游 #100/#102。
- 处理决定：**todo**（2026-09-23 由 blocked 改判）——原 blocked 的"上游链停在 #74/#75-a2 硬阻塞点"已解除，剩 #93 前置排队（共用订阅链切换协调器）。

### issue-95 ｜ [Lago 23] 充值退款先锁定、再退支付宝、最后撤回 Credits
- URL: https://github.com/1123786563/WeKnora-fork01/issues/95 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：批准前从权威 traceable Wallet 重算可退余额（消费/预占/到期/既有退款）；批准先建 Refund Lock；稳定退款身份退支付宝（重试不重复出款）；渠道成功但 Lago 失败保持 revocation pending 与锁定；Lago 撤回未消费 Credits 才完成释放锁（spec L153-156、US32-35、验收 18）。
- 实现现状：部分实现：退款状态机（revocation_pending）、Refund Lock 存储与 admission 协调、服务编排（稳定 key/只重试撤回）、审核 API+前端骨架、支付宝退款原语（测试实跑 ok）。缺口：P03 occupancy-refundable eligibility 唯一实现为默认拒绝（refund.go:127-134）、Lago 适配器无 Revoke/void/wallet_transactions、生产接线 NewRefundService(db,nil,nil,nil) 全 nil、依赖 #82/#85/#86 均 OPEN。Lago 撤回语义已由 T03 E4 实验验证（PASS-WITH-COORDINATION）。
- 验收标准：①重核算 ②Refund Lock 先于渠道退款 ③渠道成功 Lago 失败保持 pending ④稳定身份不重复出款（+验收 18）。
- 依赖：前置 #82/#85/#86；下游 #97/#98。
- 处理决定：**todo**——范围无歧义、骨架已就位；端到端须待三个前置完成（a2 裁决 B 已定协调层承载批次状态机，重核算口径可据此定稿）。

### issue-96 ｜ [Lago 24] 套餐退款通过 Credit Note 和权益撤回完成
- URL: https://github.com/1123786563/WeKnora-fork01/issues/96 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：对已生效套餐按实际使用量与未使用预付周期计算退款；finalized Invoice 修正只能用 Credit Note 或后续账单；渠道退款成功仅 revocation pending，Lago void 剩余 Credits/调整 Subscription/创建 Credit Note 均确认后才完成；并发退款不能锁定同一权益两次。
- 实现现状：部分实现（本地退款骨架已有且测试 ok）：生命周期状态机、编排（稳定 key/只重试撤回）、并发锁、审核 API。缺口：Credit Note 全链路零匹配（Go/TS grep 无）、LagoAdapter 无退款命令（仅 4 命令）、资格计算未接 Lago 权威（默认拒绝）、撤回走旧 openmeter Gateway、生产接线全 nil、无账本/实验证据。
- 验收标准：①已发生/未结算用量进资格计算 ②Credit Note 修正不改写历史 ③双确认才完成 ④并发不超锁。
- 依赖：前置 #92/#93；下游 #97/#98/#102。
- 处理决定：**todo**——两个前置均 OPEN 未实现，退款金额与剩余权益无权威口径。

### issue-97 ｜ [Lago 25] 微信退款达到与支付宝相同的商业语义
- URL: https://github.com/1123786563/WeKnora-fork01/issues/97 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：充值和套餐退款复用同一 Refund Lock、资格核算、Credit Note、撤权与恢复模型，同时使用微信独立的退款请求/查询/通知契约（REFUND.* 事件）。
- 实现现状：部分实现：Provider 契约（Refund/QueryRefund）、微信 Refund（稳定 out_refund_no）/QueryRefund/状态映射、退款状态机、编排（indeterminate park 后原始键重查）、Refund Lock 仓储（测试实跑 ok）。缺口：微信退款测试为零（wechat_test.go 仅 9 个验签用例）、REFUND.* 通知被 ErrMalformedCallback 拒绝（wechat.go:257 仅 TRANSACTION.*）、ABNORMAL→Unknown→reviewing 映射无测试、无真实微信退款证据、复用前提（P03/Credit Note）未落地。
- 验收标准：①稳定键+状态映射 ②受理≠成功、unknown 保持锁定 ③重复请求/通知不重复出款 ④真实微信证据覆盖两路径。
- 依赖：前置 #83/#95/#96；下游 #100/#101。
- 处理决定：**todo**（2026-09-23 由 blocked 改判）——原 blocked 理由即三个前置 OPEN（P03 属 #95 交付、Credit Note 属 #96 交付），均为依赖边排队，无外部输入缺口。

### issue-98 ｜ [Lago 26] Webhook 与定期对账使商业投影收敛
- URL: https://github.com/1123786563/WeKnora-fork01/issues/98 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：Lago Webhook 经签名与 unique key 验证后只作变更通知触发权威对象重读；重复/乱序不使投影版本倒退；周期对账发现并修复丢失通知；五类本地投影（Subscription/Wallet/Invoice/Payment/Credit Note）收敛于权威；差异/修复/水位可审计（US43-45、gate 19）。
- 实现现状：部分实现（仅 seam 骨架）：ReconciliationCursor/CommercialChange 类型与 Reconcile 接口方法已定义，但双适配器 fail-closed 返回 ErrPlatformUnsupported（契约测试仅断言此行为）；内部 outbox 可作去重表设计参考。缺口：webhook 消费端零（commercial 模块 grep -i webhook 无匹配）、inbox/游标/投影审计迁移零（commercial 迁移止于 000183）、对账 worker 零、deploy/lago 无 webhook 配置。
- 验收标准：①伪造签名拒绝 ②重复/乱序不倒退 ③丢失通知可对账修复 ④差异/修复/水位可审计。
- 依赖：前置 #84/#85/#93/#95/#96；下游 #100/#102/#103。
- 处理决定：**todo**——五个前置全 OPEN，排期等待；收敛目标状态机由前置定型。

### issue-99 ｜ [Lago 27] worker 崩溃和 Lago 故障不丢用量或提前释放预占
- URL: https://github.com/1123786563/WeKnora-fork01/issues/99 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：在 Usage Event、current usage 与本地确认的交接点注入故障（worker 崩溃、Lago 不可用、堆积），证明重启后按原幂等身份前向收敛：不漏计不重复、堆积时等待/告警非假成功、不可用时新收费 fail closed、已发生事实继续持久化。
- 实现现状：部分实现（崩溃幂等基础已有且部分测试实跑通过）：Outbox exactly-once、Finalize 原子、Dispatch 幂等 drain+unknown 保持 pending、KeepProtection、Reserve 恢复原语、Fulfillment/Recovery worker 已装配、fake 故障旋钮、Lago readiness 探测。缺口：LagoAdapter 无 Settle/ConfirmSettlement（生产仍 openmeter）、Dispatch 无生产 worker、无系统化故障注入矩阵/真实进程 kill 集成测试、readiness 未联动新收费闸门（CheckNewConsumption 仅静态 rollout 开关）、无 #99 实验栈。
- 验收标准：①交接点崩溃不漏不重 ②堆积进入等待/告警 ③不可用 fail closed ④原身份前向收敛。
- 依赖：前置 #90/#92；下游 #100/#101/#103。
- 处理决定：**todo**——堆积等待/告警依赖 #90、修正身份语义依赖 #92；Lago 链路本体依赖 #88（经 #90 传递）。

### issue-100 ｜ [Lago 28] Billing Center 统一展示稳定产品状态
- URL: https://github.com/1123786563/WeKnora-fork01/issues/100 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：统一 Web Billing Center/Checkout/设置摘要/Task 费用区域的 provider-neutral 状态与操作入口；覆盖 10 个稳定产品状态；余额四分区（可用/预占/退款锁定/即将到期）；权限差异化；响应不含 Lago ID/URL/凭据/原始枚举（spec L169-170、验收 21）。
- 实现现状：主体未实现：无余额四分区端点（contracts 注释明示 served by no endpoint）、无 Billing Center 页面、等待同步/余额不足无 UI 映射、attention 不渲染、can_manage_billing 未消费。已有基础：summary/account 端点、repository 四分区原料、order/refund 状态文案、权限守卫、响应卫生（前端 13 用例+go 测试通过）。
- 验收标准：①覆盖七组状态 ②余额四分区 ③三类角色权限操作 ④响应卫生。
- 依赖：前置 #91/#94/#97/#98/#99；下游 #105。
- 处理决定：**todo**（2026-09-23 由 blocked 改判）——原 blocked 理由即五个依赖全 OPEN（等待同步须 #98、预占可靠性须 #99 等均为前置交付物），属依赖边排队，无外部输入缺口。

### issue-101 ｜ [Lago 29] 完成计费数据最小化、凭据隔离和 AGPL 上线门槛
- URL: https://github.com/1123786563/WeKnora-fork01/issues/101 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：从购买、用量、Webhook、退款和日志端到端验证敏感数据最小化与凭据隔离；形成 Community/Premium 功能清单和 AGPL 生产批准记录。
- 实现现状：部分实现（底层代码姿态良好）：凭据仅服务端 env（config.go/providers_env.go）、UsageFact 最小化模型（无 prompt/输出字段）、wallet metadata 仅 tenant/period、日志脱敏基础设施、evidence 净化契约。缺口：端到端审计未执行、Lago usage event 通道不存在（AC2 无法验证）、Community/Premium 清单不存在、AGPL 法务结论不存在（deploy/lago/README.md:186-192 明确 open production gate）、外部 secret manager 未接。
- 验收标准：①密钥仅在批准的服务端 secret 管理 ②敏感内容不进 Lago metadata ③客户端/日志/Task 不泄凭据 ④AGPL 可审计结论。
- 依赖：前置 #97/#99；下游 #102/#103。
- 处理决定：**todo**——审计对象（微信退款链路、usage→Lago 通道）须前置交付后才存在。

### issue-102 ｜ [Lago 30] 空间注销时停止收费、去标识化并保留财务历史
- URL: https://github.com/1123786563/WeKnora-fork01/issues/102 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：注销空间时停止新购买与收费执行、按保留策略处置 Subscription/Entitlement/Wallet、去标识化可删除 Customer 展示数据，合规保留 Invoice/Payment/Refund/Credit Note 与审计历史；Customer identity 永不复用（US54）。
- 实现现状：未实现（仅可复用基础）：seam 无 close/terminate/de-identify 命令、DeleteTenant 与商业域零集成（internal/modules 下无调用点）、无终态（SubscriptionState 仅 active/pending）、Credit Note/Invoice 投影缺失、去标识化零实现、无注销测试。可复用：ExternalCustomerID 纯函数+身份不可变守卫、跨租户 uniqueIndex、鉴权副作用。
- 验收标准：①注销后新命令/收费被拒 ②按保留策略进入正确状态 ③财务记录合规可查 ④身份永不复用。
- 依赖：前置 #94/#96/#98/#101；下游 #104。
- 处理决定：**todo**——四个 blocker 全 OPEN；终态语义/Credit Note 对象/对账收敛/最小化决定均为前置交付。

### issue-103 ｜ [Lago 31] 部署可观测的生产 Lago 并满足计费延迟目标
- URL: https://github.com/1123786563/WeKnora-fork01/issues/103 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：固定版本 Helm 部署生产 Lago，连接独立高可用 PostgreSQL/Sidekiq Redis/cache Redis/对象存储；readiness 识别依赖故障；队列/Webhook/负余额/商业超时指标告警；目标负载下 event-to-reconcilable-batch p95≤60s（spec L180-182、L224、L277）。
- 实现现状：部分实现（核心未开始）：helm/ 全目录 grep lago 零匹配（仅 WeKnora 主 chart）；readiness 仅本地 compose 雏形；无 Prometheus/告警管道（仅 WeKnora 恢复告警抽象，测试实跑通过）；p95 仅 lab 方法学（measure.py 18 单测实跑通过）无生产验证；deploy/lago loopback-only 无 TLS/备份/容量。
- 验收标准：①生产依赖 TLS/备份/容量 ②readiness 分类 ③指标与告警 ④p95≤60s。
- 依赖：前置 #76（done）/#98/#99/#101；下游 #104。
- 处理决定：**todo**（2026-09-23 由 blocked 改判）——原 blocked 理由即 #98/#99/#101 OPEN；AGPL 上线门槛是 #101 的交付物（依赖边），无独立外部输入缺口。

### issue-104 ｜ [Lago 32] 从备份恢复 Lago 并完成商业对账
- URL: https://github.com/1123786563/WeKnora-fork01/issues/104 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：staging 全链路备份恢复：重建 Lago 商业对象/对象存储/待处理工作/WeKnora Billing Projections；恢复后权威对象与投影对账；pending 对象恢复后不重复副作用；RPO≤5 分钟、RTO≤60 分钟演练证据。
- 实现现状：未实现：Reconcile 双适配器 fail-closed、lago.sh 无 backup/restore、对象存储未配置（LAGO_USE_AWS_S3 默认 false）、无 webhook 接收（AC2 缺载体）、无 RPO/RTO 演练脚本与证据。可复用：seam Reconcile 接口定义、named volume 持久化雏形、recovery.go WeKnora 侧重放语义。
- 验收标准：①恢复七类对象 ②pending 不重复副作用 ③恢复后对账 ④RPO/RTO 证据。
- 依赖：前置 #102/#103；下游 #105。
- 处理决定：**todo**——恢复对象集需 #102 注销语义、演练拓扑需 #103 生产部署。

### issue-105 ｜ [Lago 33] 切换 Lago、关闭回退窗口并移除 OpenMeter
- URL: https://github.com/1123786563/WeKnora-fork01/issues/105 ｜ 父节点: #72 ｜ GitHub: OPEN
- 需求依据：全部验收门槛（24 项矩阵+Completion gate）通过后，默认 Commercial Platform adapter 切换为 Lago；无真实商业数据时完成回退演练并关闭回退窗口；expand-contract 的 contract 阶段删除 OpenMeter（adapter/部署资产/镜像锁/探测/环境变量/worker 写路径），历史证据标记 deprecated，此后仅前向修复。
- 实现现状：部分实现（前置就位、主体未实施）：seam 默认可指向 Lago（config.go:72-82）、fake+Lago 契约、pinned v1.53.0 栈。未实现：旧 OpenMeter Gateway 全链路仍在（container.go:244 装配，execution/fulfillment/settlement/refund 四 service 消费）、deploy/openmeter/ 未删、Reconcile 冻结、docs 无 deprecated 标记、24 项 gate 无证据、回退窗口关闭无流程。
- 验收标准：①全部 gate 通过 ②默认只写 Lago 无长期双写 ③OpenMeter 全链路删除 ④deprecated 标记+回退窗口关闭。
- 依赖：前置 #86/#89/#91/#92/#94/#97/#100/#102/#104（9 个 blocker 全 OPEN）；下游无（收官票）。
- 处理决定：**todo**——范围无歧义；须等 9 个 blocker 的 gate 证据齐备后实施。

## 3. 外部依赖核查（范围外，不纳入 DAG 节点）

- **#30（移动 AI Office Spec，OPEN）**：成果部分已在基线——`docs/specs/2026-09-20-mobile-ai-office-design.md`、`mobile-module-seams.md`、`apps/mobile`、`packages/mobile-core`、`tests/mobile-v2`、`deploy/mobile-workbench` 均在当前 main；issue30-sweep worktree 分支 `codex/issue30-mobile-office` 仅领先 1 个 docs 提交（16dddb2d5，未合并）。与 Lago 迁移**无代码耦合**，仅 CONTEXT.md 移动/账单权限语义与 Billing API provider-neutral 契约存在交集。结论：仅记录，不建节点、不加依赖边。
- **#74 外部输入（已全部闭合）**：①Stripe TEST 密钥——已提供（`~/.zcode/issue72-stripe.env`，本会话 `ls -la` 核实存在，仅测试凭据）且**已被 #74 实施轮消费**（run 3dc51207 九阶段全 pass）；②T02 三选项——**已裁决为选项②（受支持 Provider）**（2026-09-23 用户裁决），#81/#82 解锁。
- **#75-a2 设计决策（已闭合）**：充值批次 12 个月到期归属——**已裁决为选项 B**：批次状态机由 WeKnora 协调层承载、Lago Wallet 交易仅在到账时刻幂等创建；ADR-0012 已在集成分支追加修订记录（提交 552d98d12，本会话 `git log` 实查在案），#85 起充值链（传递 21 票）解锁。
- gh CLI 可用（本会话 `gh issue list --state all` 全量复测 72–105 号状态）；`docs/plans/ledgers/` 现存 lago-78/79/80 + 本 worktree 的 issue-72-ledger-74（`ls` 实查）。
