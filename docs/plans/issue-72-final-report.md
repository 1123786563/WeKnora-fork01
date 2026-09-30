# Issue #72 最终交付报告（Lago 计费迁移：批次 1 #74 + 批次 2 #81/#82 + OCR 治理收官）

> 生成：2026-09-25（**rev2**：按独立读者评审意见修订——新增 §9.4 终审遗留逐条对 HEAD 对账表、§9.5 批次 1 轮 4 处置转录、§9.6 轮 4 low 清单、D1 机制输入、Completion gate 缺口清单、前轮抽验安排、执行决定归属；修正 #94 漏行与 5 处传播链、§5.2 提交分组账、#81 提交数与证据件数口径、实施口径与「D2」三义消歧）｜ Worktree：`.worktrees/issue72-lago` ｜ 集成分支：`codex/issue-72-lago`
> 基线：`29c1e56353b2b36be242018cecb43bcb3a5ef7c8`（docs: plan pass b contract and ownership freeze）｜ 实施终点：`ac84022048a9e01777ad1c1713fc939ac430c96d`（`ac8402204`，`issue-72: ocr round 4`，2026-09-25）｜ 基线..终点 **81 个提交**（本报告会话 `git rev-list --count 29c1e56353..ac8402204` 实查=81；本报告提交在此基础上，哈希以 `git log -1` 为准）。
> **本报告覆盖并取代两份早版**：`6649e06ba`（2026-09-23 03:09 调查轮报告）与 `89cc052e9`（2026-09-23 19:17 批次 1 报告，覆盖 #74 实施轮与其 OCR 1-4 轮治理；其 §9.2 轮 4 逐项处置表已转录至本报告 §9.5，接手者无需再挖 git 历史）。早版均可在 git 历史查证。
> 各票最终状态以编排器给定的实施结果 JSON 为准（**原文全文附录于 §13**）；GitHub 状态为本报告会话 `gh issue list` 全量实跑核对（§2.4）。
> **归档时序说明**：本报告仅存在于未推送分支 `codex/issue-72-lago`（§5.1）——推送/合并（§10.2-D5）前，验收者只能经本地 worktree `.worktrees/issue72-lago` 或主 Agent 转发的补丁读取本报告。
> **术语消歧（「D2」三义）**：① #81 计划的设计决策 D2（付款时 InvoiceFees 完整复核，R-3 出处）；② **#82 计划的设计决策 D2「结算触发链」**（被 t10 证伪的对象，与①无关，机制详见 §10.2-D1）；③ 本报告 §10.2 的延期事项编号 D2。除③为编号外，①②是两份不同计划里的条款代号，并非同一物。

## 1. 执行摘要

本轮（自基线起两个编排批次）在 33 个子 Issue 中：**已验证跳过 8 票**（#73–#80，前轮成果，调查复核后未重复实施，其中 #74 为批次 1 实施票、证据已在集成分支）；**已完成 1 票**（#81 [Lago 09]，本编排轮实施：Quote 冻结 → payment-gated incomplete Subscription → 待付款 gating Invoice → 渠道支付订单全链路落地，经 Playwright 无头真实浏览器 + 真实 API + Lago DB 直查三通道流程验证 9 组断言全过，增量 OCR 两轮修复后 r2 清零 passed）；**阻塞 24 票**（#82 直接阻塞——实现完成并 merge 后，真实流程验证 3 轮未通过：t10 契约探针证伪 #82 计划的 D2 结算触发链（Task 5-10 冻结、修订版计划产出权在 spec/ADR owner，机制见 §10.2-D1），三处高优产品缺陷虽经修复+真实栈复验解锁了回调入账链，但激活后半链（订单/Billing 变已生效、Credits/Entitlement 到账）在冻结下不可达，遂按裁定 **revert `e0364d196`**（实现保留在分支 `codex/issue-72-lago-82`）；#83–#105 共 23 票为依赖传播阻塞）。

治理线：批次 1 全量 OCR 4 轮（第 1–3 轮 16 项有效 findings 清偿，第 4 轮遗留 4 medium+1 low，处置转录见 §9.5）；#81 增量 OCR 两轮报告循环（r1→修复 23 项→r1 重做→修复 4 项→**r2=0 findings，passed-r2**）；批次 2 revert 后全量 OCR 再 4 轮（38/29/40/50 项 findings，第 1–3 轮分别修复 10/6/11 组，含 1 项 critical）。**终审给定「第 4 轮后 9 项有效 critical/high/medium 未解决 + 7 项 low」；本报告对终审 11 条枚举逐条核验 HEAD 后判定：6 项仍开放（4 项全量 + 2 项部分残留）、5 项已由批次 2 修复覆盖（对账表见 §9.4）**——两口径的差异（9 vs 6）源于终审 triage 未持久化，§9.4 以逐条证据为准。

集成分支与三个 Issue 分支**均未推送远端**（本报告会话 `git ls-remote --heads origin` 实查）；除前轮已关的 7 票外，GitHub 上 #72/#74/#81–#105 全部仍 OPEN（§2.4）。

## 2. 每个子 Issue 的状态与证据（33 票）

状态分布：**已验证跳过 8 ｜ 已完成 1 ｜ 阻塞 24**（#82 直接阻塞 + #83–#105 依赖传播 23）。

### 2.1 已完成（1 票）

| Issue | 标题 | 状态 | 证据 |
|---|---|---|---|
| #81 | [Lago 09] 从 Quote 创建待付款 Invoice 和 incomplete Subscription | **已完成** | 编排器 JSON：`HEAD=e26a90d61458c5be52a8d8bda3a2e64ec9c306bf`（集成分支 merge 提交；per-issue 分支 `codex/issue-72-lago-81` tip=`bf01c3d71`，本报告会话 `git rev-parse` 实查）。**12 个实现提交（Task 1–12，`1079a11ad`→`f7c3b3532`，TDD）** + 3 个计划/审查修订提交（`a408ed695`/`3ecfa3f79`/`ebdbc84bb`）+ 2 个代码审查修复（F1/F2=`69fd9b16d`、F8=`bf01c3d71`）后 merge `e26a90d61`（§5.2 第 8–9 行分组）。流程验证=Playwright 无头真实浏览器（browser_navigate/snapshot/click/screenshot，真实登录、真实页面渲染与断言）+ 真实 API 链路（curl 真 HTTP 调用 WeKnora :8091 与 Lago :48889）+ Lago DB psql 直查（open invoice 的 API 途径不可见，按计划 F3–F5 用 DB 权威计数），（11 步，证据 16 件——编排器口径，未逐件点名无法对账；持久化目录为 20 个 git 跟踪文件，构成见 §8.2，README 9 组断言全 ✅ 含 2 组负对照）。增量 OCR：r1→修复→r1 重做→修复→r2=0 findings（`issue-72-ocr-ledger.md` 行 81 verdict=passed-r2；轮次因果见 §9.1）。详见 §8.2 |

### 2.2 已验证跳过（8 票，前轮成果，未重复实施；GitHub：7 票 CLOSED + #74 OPEN〔§2.4〕）

| Issue | 标题 | 前轮验证证据（清单 §2 + 早版报告核验；本轮未重跑——证据性质声明与抽验安排见本节末） |
|---|---|---|
| #73 | [Lago 01] 固定版本集成环境 | 46 测试+62 subtests 实跑（前轮）；`deploy/lago/evidence/t01-{images,health,contract,run}` |
| #74 | [Lago 02] 外部付款激活实验 | **批次 1 实施票**：AC1–AC4 运行时证据全 pass（run11 `5d06a277`）+ 独立流程验证（run `3dc51207`，21/21 断言+DB-WATCH PASS，`docs/plans/issue-72-flow-evidence-74/`，§8.1）+ 主 Agent F1 裁决 accept + 批次 1 OCR 4 轮（轮 4 遗留处置见 §9.5）。编排器本轮 JSON 记「调查已验证完成，未重复实施」 |
| #75 | [Lago 03] Wallet 批次语义 | e1–e4 真实栈实测+104 测试；`docs/migrations/lago/t03-wallet-semantics/verdict.md`（a2 BLOCKED→裁决 B）；a2 已裁决（R-2） |
| #76 | [Lago 04] Pricing Group 计价批次 | 60 测试+6 份真实栈 JSON（64/64 精确核对、p95 6.893s）；`docs/migrations/lago/t04-pricing-group/` |
| #77 | [Lago 05] Commercial Platform seam | 冻结 seam `platform.go:221-230`；go build/test 126 包 ok；`deploy/lago/evidence/t05-{health,run}` + `docs/migrations/lago/t05-*`（若晋升） |
| #78 | [Lago 06] 空间独立 Lago Customer | 三层幂等+租户隔离；账本 `docs/plans/ledgers/lago-78.md`；`deploy/lago/evidence/t06-{account,status}` |
| #79 | [Lago 07] 不可变 Plan Version | 六轴校验+DB trigger；账本 `lago-79.md`；`docs/migrations/lago/t07-plan-version-publish/` + `deploy/lago/evidence/t07-run.txt` |
| #80 | [Lago 08] Base Plan 权益额度 | 懒链+迁移 000183；账本 `lago-80.md`；`docs/migrations/lago/t08-base-plan/` + `deploy/lago/evidence/t08-run.txt` |

> **证据性质声明**：上表测试数字系对前轮实跑的文字转述，本轮报告会话未重跑；仓库内归档的是真实栈操作时间线（`t0*-run.txt` 等）。#74 的流程验证与 OCR 重放证据为本分支 git 历史在案（`docs/plans/issue-72-flow-evidence-74/`、`issue-72-ocr{1,2,3}-replay/`，均 git 跟踪）。
> **抽验安排（建议，供验收方决定是否执行；本报告会话未运行——报告员只改报告文件）**：① #75——`cd deploy/lago-lab/wallet-semantics && python3 -m pytest -q`（前轮口径 104 passed）+ 复核 `verdict.md` 引用的 Lago DB 断言（e1–e4 时间线）；② #78——`go test ./internal/modules/commercial/... -count=1` + 抽查 `deploy/lago/evidence/t06-account.txt` 懒链时间线。抽验优先级依据：#75 的 a2 裁决（R-2）是 #85/#86 数据模型的直接输入、#78 的 Customer 身份是全部购买链的地基。

### 2.3 阻塞（24 票，GitHub OPEN〔§2.4〕；传播链逐字转录自 §13 编排器 JSON）

**#82（直接阻塞）｜ [Lago 10] 支付宝付款后恰好一次激活套餐** —— 真实流程验证 3 轮未通过，merge 已 revert（实现保留在分支 `codex/issue-72-lago-82` @ `7f1d3e21e`）。编排器 JSON 给定的三条结论：

1. **[机制级][计划已知边界，第 2 轮裁定维持] `d2-falsified-t10-p2-fail`**：激活后半链不可达——可信回调入账后订单 payment=paid，但 Lago 订阅保持 incomplete、无 succeeded Payment（DB payments 仅 failed=6/requires_action=6）、无套餐 Credits 钱包、Entitlements 404（`rv2-lago-after-paid.txt`）；「订单与 Billing 页变已生效」不可达，呈现停在「已付款，权益处理中」。第 2 轮裁定（`eea89716e`）核实：Task 5-10 冻结（t10 probe P1/P2d FAIL 证伪 D2 结算触发链，`ca04d7da7`；机制叙述见 §10.2-D1）、`paid_awaiting_activation` 全仓 grep 0 命中、「已付款，权益处理中」为既有诚实呈现 pin——修复它=越权重设计，升级归 **T02 §5 重议**（修订版计划产出权在 spec/ADR owner）。
2. **[披露] `ac4-sandbox-credentials-unavailable`**：仍无支付宝沙箱凭据；替身为本地 RSA+回环 stub+同源签名 notify——验证修复后完整渠道入账链，不构成真实沙箱钱包付款证据（AC4 只能「带残余」）。
3. **[读面观察，非缺陷]**：单租户双订单下 GET /purchase 的 CurrentPurchaseOrder 选中未付款的 wechat 单（ord_ee4a8f2453cc1ddc pending），alipay 单 paid 由 GET /orders/:id 直证——读面选择行为已如实补档（`rv2-lago-after-paid.txt`）。

过程账（git 在案）：#82 计划 2 轮+审查修订 8 条（`c17e7e7b9`/`8a940608a`）→ 7 个 Task 提交（settle 命令/伪确定性结算/购买钱包身份/finalized 发票读/契约探针/AC2 哨兵 pin/审查修复）→ merge `8cfc91975` → 流程验证第 1 轮发现 3 处高优产品缺陷+2 项机制/披露（`issue-72-flow-evidence-82/README.md`）→ 流程修复 `7324eb054`（回调白名单/渠道商户身份 MerchantID()/客户绑定 upsert，真实栈 14 断言复验全过）+ 第 1 轮 Ruling（`59465f8fe`）→ 复验第 1 轮（`reverify1/`）3 项失败逐项核实均非产品缺陷 → 第 2 轮裁定+环境清理+流程规则固化（`eea89716e`）→ 复验第 2 轮（`reverify2/`）维持失败 → **revert `e0364d196`**（34 文件，-3724 行：deploy/lago-lab/payment-trigger 实验栈、t10 证据、plan/ledger-82、settle_purchase_payment 命令与测试等全部移出集成分支）。

**#83–#105（依赖传播阻塞，23 票）**：

| Issue | 标题 | 传播链（JSON 原文） | Issue | 标题 | 传播链（JSON 原文） |
|---|---|---|---|---|---|
| #83 | 微信复用激活 | [82] | #95 | 充值退款三段式 | [82, 85, 86, 94] |
| #84 | 异常付款不扩大权益 | [82, 83, 85] | #96 | 套餐退款 Credit Note | [92, 93, 95] |
| #85 | 充值 Credits 到账 | [82, 83] | #97 | 微信退款对齐 | [83, 95, 96] |
| #86 | 到期顺序消费 | [85] | #98 | Webhook 对账收敛 | [84, 85, 93, 95, 96] |
| #87 | 原子预占 | [86] | #99 | 故障不丢用量 | [90, 92] |
| #88 | Settlement Batch 核对 | [87] | #100 | Billing Center | [91, 94, 97, 98, 99] |
| #89 | 并发共享 Task Budget | [87, 88] | #101 | 最小化/AGPL | [97, 98, 99] |
| #90 | 延迟/超界暂停 | [88, 92] | #102 | 空间注销 | [94, 96, 98, 101] |
| #91 | BYOK 豁免 | [88] | #103 | 生产 Lago+延迟目标 | [98, 99, 101] |
| #92 | 用量修正不改写历史 | [88] | #104 | 备份恢复对账 | [102, 103] |
| #93 | 升级补发差额 | [82, 86, 92] | #105 | 切换/移除 OpenMeter | [86, 89, 91, 92, 94, 97, 100, 102, 104]（收官票） |
| #94 | 降级、年付月发和到期回 Base Plan | [93] | | | |

> rev1 勘误：本表 rev1 漏列 #94 整行，且 #84/#89/#93/#95/#96 五处传播链与 JSON 不符（漏写前置），rev2 已逐字按 §13 JSON 重录（本报告会话逐票比对）。

### 2.4 GitHub 状态全量核对（本报告会话实跑）

`gh issue list -R 1123786563/WeKnora-fork01 --state all --limit 200 --json number,state`（过滤 72–105，34/34 无缺漏）→ **#72 OPEN；#73/75/76/77/78/79/80 CLOSED（7 票）；#74 及 #81–#105 全部 OPEN（26 票）**——与 JSON 状态分布一致，无漂移。即：#74 与 #81 虽已完成/验证，GitHub 关票动作尚未执行（§10.2-D6）。

## 3. #72 总体验收标准覆盖情况（矩阵 24 项全对照）

父票 #72 不入 DAG 节点，其验收由 Spec `docs/specs/2026-09-20-lago-billing-migration-design.md`（本报告会话 `ls` 实查存在，35745 字节）承载：行为与契约矩阵 1–24 项（spec:212-237）与 Completion gate（spec:246-248）。票号↔矩阵项映射依据 DAG 依赖表与清单 §2；「部分」=有实验/语义证据但产品化票未完成：

| 矩阵项 | 对应票 | 状态（较批次 1 报告的变化） |
|---|---|---|
| 1 Community 能力+AGPL | #73+#101/#103 | **部分**：能力子集✓；AGPL 审查结论不存在 |
| 2 租户隔离 | #78 | ✓（前轮） |
| 3 不可变目录 | #79 | ✓（前轮） |
| **4 Quote 匹配** | #81 | **✓（本轮完成）**：付款前权威订阅面硬校验（plan code/CNY/整数总额）+ 无 charges 切片单行推导 + 付款时 `InvoiceFees` 完整复核接口交付（R-3 批准偏差：spec L121 付款前全项 line-item 匹配在 pinned v1.53.0 上源码级实证不可实现——open invoice 对全部 API 不可见，F3–F5）；负对照（过期 quote 409、金额漂移 409 不拉起渠道）真实验证通过 |
| 5 外部付款激活 | #74（实验）+#82/#83/#84（产品化） | **部分**：provider 轨道运行时证实（批次 1）；**#82 本轮实现后因 D2 证伪 revert**——支付宝产品化链路未落地 |
| 6 充值到账 | #85/#86 | 未覆盖（OPEN） |
| 7 付款异常 | #84 | 未覆盖 |
| 8 Credits 顺序 | #75（语义）+#86（实现） | **部分**：语义 verdict✓；实现未开工 |
| 9 用量身份 | #88 | 未覆盖 |
| 10 Settlement Batch | #76（形态）+#88（实现） | **部分**：实验形态✓；实现未开工 |
| 11 延迟与规模 | #90/#103 | 未覆盖 |
| 12 并发准入 | #87/#89 | 未覆盖 |
| 13 预占生命周期 | #87/#88 | 未覆盖 |
| 14 上界违规 | #90 | 未覆盖 |
| 15 BYOK | #91 | 未覆盖 |
| 16 修正 | #92 | 未覆盖 |
| 17 Plan 生命周期 | #93/#94 | 未覆盖 |
| 18 退款 | #95/#96/#97 | 未覆盖 |
| 19 Webhook 对账 | #98 | 未覆盖 |
| 20 故障注入 | #99 | 未覆盖 |
| 21 权限 | #100 | 未覆盖 |
| 22 隐私与凭据 | #101 | **部分旁证**：流程验证红线自查（凭据 grep 零命中、无强制 active、architectureguard/modulemove OK）为实验链路旁证，正式项未覆盖 |
| 23 备份恢复 | #104 | 未覆盖 |
| 24 OpenMeter 移除 | #105 | 未覆盖 |

- **判定**：#72 迁移整体**未完成**。实施口径：**实施并保留在集成分支 9/33（7 前轮 + #74 批次 1 + #81 本轮）；若计入实施后按裁定回退的 #82 则实施动作 10/33**。#82 的实现与证据仅存于分支 `codex/issue-72-lago-82` 与 git 历史，不计入集成分支交付面。
- **Completion gate 缺口清单（#105 收官前需归档的 24 项 gate 证据，按矩阵编号）**：已有充分证据 3 项（#2/#3/#4）；部分 5 项（#1 缺 AGPL 结论、#5 缺产品化链、#8 缺实现、#10 缺实现、#22 缺正式审计——仅有实验旁证）；**完全缺证据 16 项（#6/#7/#9/#11/#12/#13/#14/#15/#16/#17/#18/#19/#20/#21/#23/#24）**。归档格式：仓库无既定模板（DAG W17 仅要求「24 项 gate 证据归档」），建议 #105 计划定义统一目录（如 `docs/migrations/lago/gates/gate-<nn>/`），每 gate 记录矩阵编号、对应票、判据引用（票号+证据路径）；#72 关票前逐项对号。

## 4. 层级树 / DAG / 计划 / Ledger / OCR 报告路径索引

| 类别 | 路径 | 说明（本报告会话 `ls`/`wc -l` 实查） |
|---|---|---|
| 层级树/子票清单 | `docs/plans/issue-72-issues-inventory.md`（330 行） | 33 票均为 #72 直接子票、单层无嵌套；§1 总览+§2 逐票+§3 外部依赖 |
| DAG | `docs/plans/issue-72-dag.md`（342 行） | 五次修订；Mermaid+75 业务边+6 调度边依赖表+拓扑序+W1–W17 波次+阻塞节点+状态表+Kahn 校验记录（33 节点 81 边无环） |
| 用户裁决 | `docs/plans/issue-72-user-rulings.md` | R-1（T02=选项②受支持 Provider）、R-2（75-a2=选项 B 协调层承载）、R-3（#81 spec L121 偏差=选项 A），各含决定/依据/错误代价 |
| 票级计划 | `docs/plans/issue-72-plan-74.md`（805 行）；`issue-72-plan-81.md`（1920 行） | #74/#81 实施计划（含审查修订记录）；#82 计划 `issue-72-plan-82.md`（777 行）**已随 revert 移出**，在 git 历史 `e0364d196^` 与分支 `codex/issue-72-lago-82` 可查 |
| Ledger | `issue-72-ledger-74.md`（229 行）；`issue-72-ledger-81.md`（143 行） | 执行账本：Ruling、AC 判定、测试记录、审查处置；#82 账本 `issue-72-ledger-82.md`（185 行）**已随 revert 移出**（历史可查，其流程修复两轮 Ruling 全文在 `59465f8fe`/`eea89716e` 提交中） |
| 增量 OCR（每 Issue） | `issue-72-ocr-issue-81-r1.md`（终版 37 findings）；`issue-72-ocr-issue-81-r2.md`（终版 **0 findings**） | #81 专用；文件被 `11fb134ae`/`d27a451cb`（r1）与 `7a665bb57`/`900041bb0`（r2）两轮写入（`git log` 实查，轮次因果见 §9.1）；#74 无单独增量 OCR（批次 1 全量 OCR 即其治理），#82 未进入增量 OCR（流程验证未过即 revert） |
| 增量 OCR 台账 | `issue-72-ocr-ledger.md` | 行：`81｜29c1e56353b→7a665bb57｜passed-r2｜issue-72-ocr-issue-81-r2.md`（backfill 提交 `759f386e1`；**未随终版 r2#2=900041bb0 更新**，见 §9.1） |
| 全量 OCR 报告（批次 1，#74 治理） | `issue-72-ocr-round-{1,2,3,4}.md`（`0ef4577b6`/`8b7e20772`/`83c6e8edf`/`c9e4033d3`） | 10/7/5/13 findings；第 1–3 轮 16 项清偿（`c7e404f3a`/`d95a93f0d`/`8e89d4bae`），轮 4 遗留处置转录见 §9.5 |
| 全量 OCR 报告（批次 2 收官，revert 后） | `issue-72-ocr-round-{1,2,3,4}.md`（`1c684ae68`/`0c6409f75`/`7b9db0314`/`ac8402204`，**文件名相同、内容为第二轮**） | 38/29/40/50 findings；轮 1–3 修复 `a2986794a`（10 组）/`cfa1b2452`（6 组）/`cd90d27ab`（11 组，含 R3-27 critical）；轮 4 无修复轮——终审遗留与 HEAD 对账见 §9.3/§9.4。⚠️ 同名两版以提交哈希区分 |
| OCR 修复记录 | `issue-72-ocr-fix-log.md`（1415 行） | 七个章节：批次 1 轮 1–3、#81 增量两批、批次 2 轮 1–3；逐条 Ruling/RED 证据/回归测试/重放记录 |
| 流程验证证据 | `issue-72-flow-evidence-74/`（23 个跟踪文件）；`issue-72-flow-evidence-81/`（20 个跟踪文件）；`issue-72-flow-evidence-82/`（51 个跟踪文件，含 `reverify1/` 15 + `reverify2/` 12） | §8 详述 |
| OCR 重放证据 | `issue-72-ocr{1,2,3}-replay/`（各 16 个跟踪文件） | 批次 1 每轮修复后的真实流程重放（run JSON+断言输出+DB-watch TSV）；批次 2 各修复轮对四副本 AC/DB-WATCH 重放 ×4（fix-log 各章节） |
| 运行时证据（前轮 lab 晋升/留存） | `docs/migrations/lago/`：`t02-payment-activation`、`t03-wallet-semantics`、`t04-pricing-group`、`t07-plan-version-publish`、`t08-base-plan`、`t09-quote-invoice`（`ls` 实查）；`deploy/lago/evidence/`：`t01-*`、`t05-health.json`+`t05-run.txt`、`t06-account.txt`+`t06-status.json`、`t07/t08/t09-run.txt`（`ls` 实查在案） | #77/#78 的 t05/t06 证据存于 `deploy/lago/evidence/`（本轮核对其存在；**t05/t06 无 docs/migrations 晋升目录**——前轮即未晋升，非本轮丢失）；t10-payment-trigger 已随 #82 revert 删除（历史 `e0364d196^` 可查） |
| Spec/ADR | `docs/specs/2026-09-20-lago-billing-migration-design.md`；`docs/adr/0012-lago-as-commercial-billing-authority.md`（含 75-a2 修订 `552d98d12`+出处注记） | 事实源 |
| 编排器状态 JSON | 本报告 §13 附录 | 33 票实施结果原文（任务下发方给定，附录即入库存档） |

## 5. Git 事实：worktree / 分支 / 基线 / HEAD / 关键提交

### 5.1 基本事实（本报告会话实跑，命令见 §7 末）

- **Worktree**：`.worktrees/issue72-lago`，分支 `codex/issue-72-lago`（`git worktree list`/`branch --show-current` 实查），工作区除未跟踪的 `config.yaml`/`data/`（本地运行产物，不属交付）外干净。实施所用 per-issue worktree `issue72-n74`/`issue72-n81`/`issue72-n82` **均已清理**（`git worktree list` 无条目），其分支保留本地。
- **基线**：`29c1e56353b2b36be242018cecb43bcb3a5ef7c8`；**实施终点**：`ac84022048a9e01777ad1c1713fc939ac430c96d`（2026-09-25）；**基线..终点=81 提交**（`git rev-list --count` 实查）。
- **Issue 分支**（`git branch --list 'codex/issue-72-lago-*'` + `git rev-parse` 实查，共 3 个）：
  - `codex/issue-72-lago-74` @ `ec14bbf94`（#74 主 Agent 裁决+偏差记录，批次 1 tip）
  - `codex/issue-72-lago-81` @ `bf01c3d71`（F8 审查修复，#81 tip）
  - `codex/issue-72-lago-82` @ `7f1d3e21e`（review-r1-f1 探针 lab 模块化绑定，#82 tip——**revert 后实现仅存于此**）
- **merge 拓扑**（`git log --graph --pretty='%h %p %s'` 实查）：`175de8b4f`=552d98d12+ec14bbf94（#74）；`e26a90d61`=639e6dc07+bf01c3d71（#81）；`8cfc91975`=900041bb0+7f1d3e21e（#82，其后被 revert）。#82 分支基于 `7a665bb57`（#81 增量 OCR r2#1 时点集成 tip），ledger-82 有基线勘误记录（任务简报称 900041bb05 与实测不符，落后 4 提交、无分叉）。
- **远端**：`codex/issue-72-lago` 与三个 Issue 分支**均未推送**（`git ls-remote --heads origin` 实查：远端仅 `main`/`lago-73-community-env`/`lago-integration`/移动端分支，无任何 issue-72 分支；`git branch -vv` 无 upstream）。

### 5.2 关键提交（基线..终点全部 81 个，按主题分组；行内哈希数=行首标注数，合计=81——本报告会话 `git log --pretty='%h' | cat -n` 逐条清点后归组复核）

> 注：表按主题分组、非严格拓扑序——#82 分支提交（第 11 行）与集成分支上 #81 增量 OCR 第二循环（第 12 行）在时间上交错（§6 批次 2 并行），交错正是两线并行的痕迹。

| 主题 | 提交（个数） |
|---|---|
| 清单+DAG（批次 1 两版） | `7cf755474`、`dafba8851`（2） |
| 早版报告（被本报告取代） | `6649e06ba`（调查轮）、`89cc052e9`（批次 1 报告）（2） |
| #74 计划/工具/ADR/证据/审查裁决 | `6f941c48d`、`42696a30f`（计划）、`52e22b366`、`4aa74ce34`（实验工具契约）、`552d98d12`（ADR-0012）、`9ff29b7ec`、`812bb241d`、`5ccdc7f3a`（运行时证据/晋升/ledger final）、`0aa976c64`（判据钉死）、`ba562d8e6`、`6bd1ef6fb`（F1 审查两轮）、`ec14bbf94`（主 Agent 裁决）（12） |
| #74 集成+流程验证 | `175de8b4f`（merge）、`905ba19b7`（2） |
| 批次 1 全量 OCR r1–r4 | `0ef4577b6`+`c7e404f3a`、`8b7e20772`+`d95a93f0d`、`83c6e8edf`+`8e89d4bae`、`c9e4033d3`（7） |
| 清单+DAG（批次 2 重订，DAG 五修） | `69d44513e`、`0f0d6492c`、`f6969fc00`（3） |
| #81 计划/审查/用户裁决 | `a408ed695`、`3ecfa3f79`（计划+审查修订两轮）、`ebdbc84bb`（round-2 修订）、`639e6dc07`（R-1/R-2/R-3 入库）（4） |
| #81 实施（Task 1–12，TDD） | `1079a11ad`、`8674614b7`、`7d5c82571`、`8a385e7a5`、`97fde2457`、`0fc66c1a9`、`811e06cc4`、`4474e2fd4`、`e4b09fe5b`、`e5d62daea`、`fb71032ff`、`f7c3b3532`（t09 证据/DECISION/ledger）（12） |
| #81 审查修复+集成+流程验证 | `69fd9b16d`（F1/F2）、`bf01c3d71`（F8）、`e26a90d61`（merge）、`81d486e27`（flow evidence）（4） |
| #81 增量 OCR 第一循环 | `11fb134ae`（r1#1）、`4435b183b`（修复 23 项）、`7a665bb57`（r2#1，35 findings）（3） |
| #82 计划/实施（Issue 分支） | `c17e7e7b9`、`8a940608a`（计划+审查修复）、`66ab920cc`、`1cf072701`、`782e316da`（task2-4）、`4a1e5b919`（计划）、`64362a529`（task1 探针）、`ca04d7da7`（t10 证伪证据+lab）、`098de74aa`（AC2 pin+ledger）、`81909d5fc`（lab.env 卫生）、`7f1d3e21e`（review-r1-f1）（11） |
| #81 增量 OCR 第二循环（含台账 backfill） | `759f386e1`（台账 backfill）、`d27a451cb`（r1#2 重做，37 findings）、`fc8448308`（修复 4 项）、`900041bb0`（r2#2，0 findings）（4） |
| #82 集成+流程验证/修复/复验/revert | `8cfc91975`（merge）、`ccec6fd3c`（flow evidence）、`7324eb054`（三缺陷修复）、`59465f8fe`（第 1 轮 Ruling）、`4292e69f6`（reverify1）、`eea89716e`（第 2 轮裁定+清理）、`bf7f62ce2`（reverify2）、`e0364d196`（revert）（8） |
| 批次 2 全量 OCR r1–r4 | `1c684ae68`+`a2986794a`、`0c6409f75`+`cfa1b2452`、`7b9db0314`+`cd90d27ab`、`ac8402204`（7） |

## 6. 并行批次组织（共 2 批）与集成 / revert 记录

- **批次 1（2026-09-23，前一编排轮）：{#74} 单票批次。** 依据当时 DAG 判定「唯一可立即开工节点=#74」（T02 未裁决前其余节点悬置）；1 个实施流（worktree `issue72-n74`〔已清理〕/ 分支 `codex/issue-72-lago-74`）。无热点文件并行冲突（单流）。基座 8 票（#73–#80）「已验证跳过」不派实施流。
- **批次 2（2026-09-23~25，本编排轮）：{#81 → #82} 两票、独立 worktree/分支。** R-1/R-2 裁决落地后 DAG 判定唯一就绪节点=#81（三前置 #74/#78/#79 全 done）；#81 先行实施（worktree `issue72-n81`〔已清理〕/ 分支 `codex/issue-72-lago-81`）→ merge `e26a90d61` 后进入流程验证+增量 OCR 线；**#82 分支自 `7a665bb57`（#81 后集成 tip）切出**（worktree `issue72-n82`〔已清理〕/ 分支 `codex/issue-72-lago-82`），与集成分支上 #81 的增量 OCR 线**时间重叠、无共享写面**（§5.2 第 11/12 行提交交错即其痕迹；DAG 边 81→82 保证实现先后，热点文件 `lago.go`/`contracts/commercial.ts` 无同波并行写）。下游波次（W3–W17）零执行。
- **集成记录：3 次 merge**——`175de8b4f`（#74；结果=Issue 分支全部内容+ADR-0012 修订）、`e26a90d61`（#81）、`8cfc91975`（#82）。
- **revert 记录：1 次**——`e0364d196`（`issue-72(revert): #82 流程验证未通过`，34 文件 +18/−3724）：#82 的实现（seam settle 命令、伪确定性结算、购买钱包、payment-trigger 实验栈、t10 证据、plan/ledger-82）整体移出集成分支，**保留在分支 `codex/issue-72-lago-82`**；#82 流程验证证据目录（`issue-72-flow-evidence-82/`）与 flowfix 两轮 Ruling 提交（`7324eb054`/`59465f8fe`/`eea89716e`）不在 revert 范围、仍在集成分支。对照：批次 1 曾有 F1 裁决**否决** revert 路径（`ledger-74` Ruling）；本次 #82 revert 是流程验证未过后的主动回退（裁定记录 `eea89716e`：冻结下修复激活链=越权重设计）。

## 7. 实际运行的测试与结果

> 口径：下表「归档」= 修复/验证/实施会话实跑并留痕于 Ledger/fix-log/README 的记录（本报告会话未重跑）；「★」= 本报告会话（含 rev2 会话）实跑。

| 会话 | 测试/检查 | 结果 | 出处 |
|---|---|---|---|
| #74 实施（run1–run11，归档） | `pytest test_lab.py test_phases.py -q`；`run_lab.py` 真实栈 | 60→64 passed；run11 `5d06a277` 九阶段全 pass，25 条 GREEN AC 断言 ALL PASS | `issue-72-ledger-74.md:125-137` |
| #74 流程验证（归档） | 离线回归+`run_lab.py`+`verify_ac_assertions.py`+`verify_db_watch.py` | 64 passed；run `3dc51207` 九阶段 pass、**21/21 断言**、DB-WATCH PASS（272 采样）；sk_test 形状扫描 0 命中 | `issue-72-flow-evidence-74/VERIFY-SUMMARY.txt` |
| 批次 1 OCR 1–3 修复（归档） | 同上+真实重放 | 74/78/82 passed；重放 21/21→23/23→25/25 + DB-WATCH PASS | `issue-72-ocr-fix-log.md` 各章节 |
| #81 实施（归档） | `go test ./...`；`make lint`；`make check-backend-architecture`；`pnpm test:shared`；`pnpm test:web`；`typecheck`；`TestLagoPurchaseIntegration`（env 门控真实栈）；红线 grep | exit 0 全包 ok（125 包，1 个基线 flaky 单独重跑 ok）；lint 零新增；architectureguard OK（F8 修复后 literal=566/total=635/hooks=59）；shared 979/983（3 失败=基线 kbDetail flaky，stash 对照）；web 2256/2294（38 失败=基线既有，stash 对照）；真实栈 **PASS 9.84s**；sk_test/rk_live CLEAN | `issue-72-ledger-81.md:91-107,137-142` |
| #81 增量 OCR 修复 1（`4435b183b`，归档） | commercial 7 包/handler/payment/`go build`/contracts 20/20/web 2297(2259+38 基线)/typecheck/DB-WATCH ×4/AC ×3/微信 stub 往返 | 全 ok / 20/20 / 38 失败与基线同集合 / 四副本 PASS / ALL PASS ×3 / code_url 正常 | `issue-72-ocr-fix-log.md` ocr-81-1 章节 |
| #81 增量 OCR 修复 2（`fc8448308`，归档） | RED×4 组+commercial 7 包+handler+`go build`+lago-lab 83 passed+AC ×4 | RED 全复现→GREEN；83 passed；ALL PASS ×4 | 同上 ocr-81-2 章节 |
| #82 流程验证/修复（归档，实现已 revert） | commercial 7 包 ok+AC2 pin；flowfix 三缺陷 RED/GREEN（router 匿名可达/handler 商户匹配/commercialplatform 绑定）+`go build`+7 包+handler/router/middleware+architectureguard+modulemoves+红线（353 行 diff 双扫描） | 7 包 ok；三缺陷 RED 复现→GREEN；真实栈复验 14 断言全 PASS；architectureguard OK（0 violations）；红线 0 命中 | `issue-72-flow-evidence-82/README.md` 断言 14-15；ledger-82（历史 `e0364d196^`）流程修复章节 |
| #82 复验第 2 轮（归档） | `rv2-gotest.txt`（10 包） | 10 包全 ok | `issue-72-flow-evidence-82/reverify2/` |
| 批次 2 全量 OCR 修复 1（`a2986794a`，归档） | RED×9 组+`go build`+commercial+handler 8 包+lago-lab 86 passed（669s）+TS 11/11+contracts 9/9+四副本 AC/DB-WATCH ×4+微信 stub | 全部通过；四副本重放全 PASS | `issue-72-ocr-fix-log.md` ocr-r1 章节 |
| 批次 2 全量 OCR 修复 2（`cfa1b2452`，归档） | RED×6 组+8 包+payment 15 轮稳定性+TS（CheckoutPage 7/7、contracts 21/21、errors 6/6）+四副本重放 | 全部通过（15/15 ok，flake 顺带修复） | 同上 ocr-r2 章节 |
| 批次 2 全量 OCR 修复 3（`cd90d27ab`，归档） | RED×10 组（R3-27 另有真实 PostgreSQL 17 容器方言 RED/GREEN）+8 包+lago-lab 87 passed（904s）+TS 10/10、8/8、21/21+四副本重放 | 全部通过；PG 17 实证：旧 DDL `operator does not exist: boolean = integer`→新 DDL CREATE INDEX 成功 | 同上 ocr-r3 章节 |
| 批次 2 全量 OCR 轮 4（`ac8402204`） | **无修复轮、无重跑**（仅报告） | — | §9.3 |
| ★ 本报告会话（rev1+rev2） | git：`rev-list --count`（=81）、`log --graph`（拓扑）、81 提交 `cat -n` 逐条清点归组（§5.2）、`branch -vv`/`rev-parse`（3 分支 tip）、`ls-remote`（未推送）、`worktree list`、`show e0364d196 --stat`（34 文件）、`show e0364d196^:…ledger-82.md`（历史账本）、`show …t10-payment-trigger/DECISION.md`（机制输入）；gh：34/34 票状态；证据目录 `ls`/`git ls-files`/`wc -l`（23/20/51/16/16/16 跟踪、文档行数、`deploy/lago/evidence/` 含 t05/t06）；rev2 对账核验：`sed -n`/`grep`（verify_db_watch 四副本 :33/:40/:37/:37、`verify_ac_assertions.py:40-62` AC1 分支、`purchase.go:120-289` 步骤 2b-8、`lago_purchase.go:36-62` 预算公式+found/422 分支+`waitForPaymentMethodSync` 函数体、`repository/order.go:292-412`、`handler/commercial.go:455-510` default 分支、`errors.ts:68-82`、`CheckoutPage.tsx:45-184`、`wechat.go:181-199`/`alipay.go:196-213`）；`grep -rn 'CancelPurchase\|cancel_purchase\|CancelSubscription' internal/ --include='*.go'`（**0 命中**）；轮 4 severity tally `grep -oE '\[[a-z]+ · (critical\|high\|medium\|low)\]' | sort | uniq -c`（2 high/12 medium/32 low）；r2#1↔r1#2 文件集 diff（12/16 文件相同） | 全部与正文引用一致 | 本报告会话 |
| **未运行** | go/pytest/pnpm 任何测试套件、任何真实栈重跑、`run_lab.py`（需 Stripe key）、PG twin 并发测试（#87 范围，`SAAS_TEST_PG_DSN` 缺失，悬置同早版报告 §7）、OCR 第 5 轮 | — | 报告员只写报告不执行测试；§9.4 对账为**静态核验**（源码形状+修复锚点），行为级验证未运行 |

## 8. 真实流程验证汇总（已验证 Issue：#74、#81；阻塞票 #82 的 3 轮验证一并记录）

### 8.1 #74（批次 1，已验证）

- **环境**：隔离实验栈 `weknora-lago-74`（回环 48891/48892，5 镜像 v1.53.0 digest 锁定，与 #73 主栈完全隔离）；Stripe TEST key 经 `~/.zcode/issue72-stripe.env` 注入环境变量（值不回显/不入库）；验证基线=集成分支 `175de8b4f`。
- **方式**：① 离线回归；② 清卷→`lab.sh init/up`（全 healthy）；③ `run_lab.py` 真实运行（9 阶段，run `3dc51207`，exit 0）——**串真实 Lago REST+GraphQL 与真实 Stripe TEST 扣款**（provider_payment_id=真实 PaymentIntent）；④ 并行 DB 观察者每 2s 只读 psql 采样（272 点 TSV）；⑤ `verify_ac_assertions.py` 21/21；⑥ `verify_db_watch.py` DB-WATCH PASS（succeeded 峰值恰 1、sub-b 4→1 不回退）；⑦ 前端 3 截图（Playwright 无头渲染，仅人工观察不作判据）；⑧ down+兄弟栈复核+密钥形状扫描 0 命中。
- **结果/证据**：AC1–AC4+负对照+DB 投影全部通过；`docs/plans/issue-72-flow-evidence-74/`（23 个跟踪文件：VERIFY-SUMMARY.txt、12 个 t02-*.json、3 截图、TSV/输出/校验脚本）；OCR 重放 ×3（`issue-72-ocr{1,2,3}-replay/`，21/21→23/23→25/25）。

### 8.2 #81（本编排轮，已验证——编排器口径「11 步，证据 16 件」）

- **环境**（`issue-72-flow-evidence-81/README.md`）：Lago v1.53.0 @127.0.0.1:48889/48890（operator 运行栈）；WeKnora 后端 @:8091（集成分支 `go run`，sqlite 独立库 `data/issue81-flow.db`——避开 #80 PG EnsureSchema 已知缺陷）；前端 vite @:5183；微信渠道真实 `WechatProvider`+本地 stub @:8291（仅 native/query，永不置已付）；**真 Stripe TEST**（`pm_card_threeDSecure2Required` 3DS 必验卡制造稳定待付款窗口：payment=requires_action→gating invoice 保持 open、订阅保持 incomplete）。
- **验证方式**（三通道）：**Playwright 无头真实浏览器**（真实登录、真实页面渲染与断言，4 截图）；**真实 API 链路**（curl 真 HTTP：发布 draft/publish 201→quote→purchase 201 awaiting_payment→checkout_url=weixin://…）；**Lago DB psql 直查**（open invoice 的 API 途径不可见——计划 F3–F5 源码级实证，故发票/订阅/计数以 DB 权威读数）。
- **结果**（README 9 组断言全 ✅）：发布不可变 Plan Version；Checkout 冻结面（版本/CNY 9900/权益/单行订阅费/+30min 过期）；购买→incomplete Subscription+open Invoice（DB 双证）；微信渠道请求创建；Billing「待付款（权益未开放）」+entitlements 404；同 quote 重试 ×2 幂等（同订单、Lago 计数 1 sub/1 invoice 不变、租户隔离 2/3 各 1/1）；过期 quote 409；金额漂移 409 且无渠道请求（负对照 ×2）。
- **证据**：`docs/plans/issue-72-flow-evidence-81/`（**20 个 git 跟踪文件** = 4 截图 + 12 份 api/ac/neg/lago 留存〔api×5+ac×3+neg×2+lago×2〕 + channel-and-backend-log + wechat_pay_stub.py + stripe_mock_notice.md + README；编排器「16 件」口径未逐件点名、无法与目录对账——同批次 1 报告对 #74「16 步 16 件」的处理，验收以目录实际文件为准）。
- **发现（failures，未改产品代码，由后续 OCR 承接）**：checkout_url 轮询丢失/provider 字段重放为空/admin UI blocked-env 边界。

### 8.3 #82（本编排轮，阻塞——3 轮验证均未通过，merge 已 revert）

- **环境**：Lago 独立栈 `weknora-lago-82flow`（独立数据卷）+Stripe provider（GraphQL 注册，真 TEST key）；WeKnora 后端 :8092（sqlite `issue82-flow.db`）；支付宝网关 stub :8292（真实 AlipayProvider 协议往返：请求 RSA2 签名被 stub 验证、响应签名被 provider 验证，同一本地密钥对）；微信 stub :8291；Playwright headless。复验轮：修复版二进制 :8093+DB 副本（第 1 轮，后裁定该自验方式违规并清理）、原栈复验（第 2 轮）。
- **第 1 轮**（`issue-72-flow-evidence-82/`，51 跟踪文件）：15 组断言——已落地面 10 ✅（发布/浏览器 checkout 待付款/Billing 行/gated 订阅+open invoice/支付宝下单二维码/同步返回不推进（AC2 渠道面）/回调验签通过/权益闭合/回归 7 包/红线）；**4 ❌+1 未达**：匿名回调 401（缺陷 B）、可信回调 404 merchant 错配（缺陷 C）、「已付款待激活」与「已生效+Credits 到账」不可达（冻结+`paid_awaiting_activation` 不存在）、重复回调幂等未达。
- **流程修复**（`7324eb054`+`59465f8fe`）：三处高优缺陷根因修复（回调白名单 `/api/v1/commercial/callbacks/*` 仅 POST；Provider 接口新增 `MerchantID()`；绑定 exists 分支改 POST upsert 不带 name 键——以 pinned v1.53.0 容器源码双实证纠正 #81 OCR R1-V05 的 PUT 假设），真实栈 14 断言复验全 PASS（含匿名回调可达、同源签名 notify→200 success→订单 paid→outbox 恰 1→重放幂等）。
- **第 2 轮复验**（`reverify1/`、`reverify2/` 各 15/12 文件）：三项失败逐项核实**均非产品缺陷**——`d2-falsified-t10-p2-fail` 维持冻结（激活链实施权在 T02 §5 重议）；沙箱凭据披露不变；环境痕迹根因=上轮修复员以 DB 副本写共享 Lago authority（external_id 撞名），已清理并固化流程规则（「验证/自验实例永远不得以复制的 WeKnora DB 对共享 Lago authority 写入」）。复验终态：可信回调入账后订单 paid，但 Lago 订阅 incomplete、无 succeeded Payment、无 Credits 钱包、Entitlements 404（`rv2-lago-after-paid.txt`）——「已生效」不可达。
- **结论**：#82 用户流程整体未达成 → revert `e0364d196`（实现保留分支 `codex/issue-72-lago-82`）。

## 9. OCR：每 Issue 增量结论、最终全量终审与修复轮次

### 9.1 #81 增量 OCR（passed-r2；轮次因果链）

提交时序（`git log --date` 实查）：`11fb134ae` r1#1（09-24）→ `4435b183b` 修复 23 项 → `7a665bb57` r2#1（09-24 21:59，**35 findings**）→ `759f386e1` 台账 backfill passed-r2（09-25 00:37，区间终点=7a665bb57）→ `d27a451cb` r1#2 重做（01:19，37 findings）→ `fc8448308` 修复 4 项（02:11）→ `900041bb0` r2#2（02:47，**0 findings**）。

| 轮 | 报告 | findings | 修复 | 修复后验证 |
|---|---|---|---|---|
| r1#1 | `11fb134ae` | —（首版） | `4435b183b`（ocr-81-1） | **23 项有效 R1-V01…R1-V23 全部修复**（含 V01 空串 getOrder、V02 rows[0]、V03 active/canceled 放行、V04 503 哨兵、V09 渠道 SSRF、V10 max_succeeded、V13 checkout_url 校验、V22 样本口径等）；commercial 7 包 ok、四副本重放 PASS |
| r2#1 | `7a665bb57` | 35 | —（见下注） | 台账 backfill passed-r2（`759f386e1`） |
| r1#2（重做） | `d27a451cb`（r1 文件终版） | 37 | `fc8448308`（ocr-81-2） | **4 项有效 medium（R1-08/R1-15/R1-24/R1-35）全部修复**：gate 不可见分支补 payments 断言；verify_ac 空 checks 空真守卫（四副本 16 处）；PM 轮询生产姿态跳过+ctx 取消不折算 Unreachable；checkout_url 持久化+verbatim 重放。RED×4 组→GREEN；83 passed |
| **r2#2（终版）** | `900041bb0` | **0** | — | **passed** |

> **轮次因果与口径说明（rev2 补）**：① r2#1 报 35 项后编排侧仍 backfill 了 passed-r2（对 raw findings 的有效裁量为 0 是可能的解释，但**该 35 项的逐条 triage 未持久化**，无法从仓库复核）；随后 r1 被重做——r1#2 的 37 项与 r2#1 的 35 项按**文件口径高度重叠**（两报告涉及 16/18 个文件、12 个相同；行号漂移使逐条对不上，本报告会话 diff 实测），其中 4 项被裁定有效并修复。② 增量 OCR 台账行（`issue-72-ocr-ledger.md`）的区间终点停留在 r2#1（7a665bb57），**未随终版 r2#2=900041bb0 更新**——终版清零以 `900041bb0` 提交的 r2 文件为准（0 findings）。③ #74 无单独增量 OCR（其治理即批次 1 全量 OCR）；#82 未进入增量 OCR（流程验证未过即 revert）。

### 9.2 全量 OCR（两轮四轮制）

| 轮次 | 报告/修复提交 | findings | 有效修复 | 修复后测试/重放 |
|---|---|---|---|---|
| 批次 1（#74 治理）r1–r4 | `0ef4577b6`/`c7e404f3a`、`8b7e20772`/`d95a93f0d`、`83c6e8edf`/`8e89d4bae`、`c9e4033d3` | 10/7/5/13 | 轮 1–3 共 **16 项**清偿 | 74→78→82 passed；真实重放 21/21→23/23→25/25+DB-WATCH PASS；**轮 4 无修复轮，遗留 4 medium+1 low（逐项处置转录见 §9.5）** |
| 批次 2（revert 后终审）r1 | `1c684ae68`/`a2986794a` | 38 | **10 组**：R1-22（high，跨报价双单）；R1-07/08/35/36（high，verify_db_watch 四副本守卫恒假）；R1-05/R1-09/R1-12/R1-14+16+17+19/R1-20/R1-24/R1-28+29+31/R1-37（medium） | 86 passed（669s）；TS 11/11；四副本重放 ×4 |
| r2 | `0c6409f75`/`cfa1b2452` | 29 | **6 组**：R2-26（high，三段读判写无原子性→部分唯一索引）；R2-28（high，渠道失败死单卡死购买）；R2-01/R2-17+19/R2-21/R2-27（medium） | 8 包 ok；payment 15/15；contracts 21/21 |
| r3 | `7b9db0314`/`cd90d27ab` | 40 | **11 组**：R3-27（**critical**，PG 启动必失败 boolean=integer，真实 PG17 RED/GREEN）；R3-26/R3-28（high）；R3-25/07/09/02/17/19/31/39（medium） | 87 passed（904s）；PG17 方言实证；四副本重放 ×4 |
| **r4（终审）** | `ac8402204`（无修复提交） | 50 | **无修复轮** | 无重跑；severity tally（本报告会话 grep 实数）：2 high/12 medium/32 low（4 项无标签不计）；终审结论见 §9.3 |

### 9.3 终审给定的遗留清单（编排器口径；对账见 §9.4）

**终审给定：第 4 轮后 9 项有效 critical/high/medium 未解决 + 7 项 low 未解决。** 任务指令同时给出 11 条枚举（2 high + 9 medium；其中**三处条目**标注同根因合并——第 2 条四副本合并、第 4 条两分支合并、第 8 条两渠道各两条发现合并），**「11 条枚举」与「9 项有效」之间的换算关系编排器未说明（无持久化 triage）**。要点：

1. **【high】步骤 8 的幂等维度是 quoteID，门控对象是购买身份（ExternalPurchaseSubscriptionID，租户 ID 纯函数），不一致导致跨报价重复开单**：同租户以 quote1 购买（order1 pending、渠道链接有效）后重新报价 quote2（同 plan 同价，CreateQuote 无限制）再 POST——命令幂等重放、匹配门通过、GetOrderByQuote(quote2) 未命中、p 仍 awaiting，遂开第二张独立渠道订单；两单分别付款即双倍扣款+双倍履约，且第一笔付款激活订阅后 order2 仍可付，恰好绕过 ErrPurchaseNotAwaiting（该守卫只拦新开单不拦已存在订单）。
2. **【high】四个 verify_db_watch 副本的 runs/ 回退守卫恒假、永不可达**（同根因合并：flow-evidence-74:104、ocr1:111、ocr2:108、ocr3:108）：`archived.exists() and path != str(archived)` 恒 False，MISSING-EVIDENCE 降级（WARNING+exit 2）永不触发；归档 TSV 缺失而 runs/ 残留外来回放 TSV 时，会在无法按 run 键隔离的整库 payments 聚合上裁决 max_succeeded()==1，产出与本次运行无关的假 PASS/CHECK。
3. **【medium】purchaseErrorMessage 的闭合 reason 分支在真实失败路径不可达**：后端失败（409/503/404）非 2xx，api-client request 层直接抛 ApiError 不进 parsePurchaseView，errorFromResult 只提取 error/message/code/requestId 不提取顶层 reason；成功路径后端恒带 Order 使 `!purchase.order` 恒假——闭合文案映射是死分支，用户在渠道未配置/平台不可达时看到英文原文。
4. **【medium】AC1 重放断言未覆盖 phases.py 文档化的 charge-failure endgame PASS 路径**（两条同根因合并）：(a) 发票不可见 endgame（phases.py:601-609，canceled+payment_failed 同样 PASS）——ac 不可见分支无条件断言 recheck_subscription_status=="incomplete"（endgame 时该查询已 404、recheck=None），会把合法 PASS 证据误判 FAIL；(b) 发票可见 failed endgame（phases.py:630-636）——ac else 分支要求 invoice_status=="open" 且 payments≤1（缺省 99），同样连报两条 FAIL。t02-gating.json 的 expected 契约明确写有 "(or ends canceled(payment_failed) when the charge fails)"。
5. **【medium】Purchase 的 default 分支以 400+err.Error() 接住未被哨兵覆盖的错误**：QuoteSnapshotForTenant/GetPublication/EnsureBillingAccount/OpenOrder 的 DB 错误与损坏快照 JSON 被误标客户端错误，驱动/SQL 原始文本在公共边界回传，违背 503 分支自述的闭合词汇约束（never raw error text）。
6. **【medium】PM 轮询预算组合缺陷（生产姿态已由 R1-24 修复消除，dev/test 姿态残留）**：purchaseRequestTimeout=25s 名义预算 vs 带 StripePmToken 的 dev/test 串行路径（≤3 次 Stripe 出站各 ≤15s+20s 轮询+多次 Lago 调用）名义总和远超；前序出站偏慢则 20s 轮询被 ctx 截断；waitForPaymentMethodSync 的 deadline 不从属 ctx 剩余预算。
7. **【medium】身份重放与 422 恢复分支只比较 PlanCode、不检查已持有订阅的 Status**（L216-224、L259-263）：canceled/terminated 的购买同样被回答成功回执；购买身份无任何重建/重开路径（全模块无 CancelPurchase/cancel_purchase/CancelSubscription 命中，Lago 不复用已取消订阅的 external_id）——购买一旦被取消，后续每次 create 都「成功」而第 8 步永久 ErrPurchaseNotAwaiting：租户购买能力不可恢复死锁且与其他失败不可区分。
8. **【medium】渠道支付客户端 SSRF/重定向防护不足**（alipay.go:200-208 与 wechat.go:188-196 同根因合并，各含两条发现）：(a) URL 级 net.LookupIP 校验后实际拨号由默认 Transport 再次独立解析 DNS，存在 DNS rebinding TOCTOU 窗口（每跳重定向同样受影响）；仓库其它出站路径均以 SSRFSafeDialContext 在拨号层兜底。(b) 自定义 CheckRedirect 禁用默认 10 跳上限且不查 len(via)：公网主机间循环 302 跟随到 Timeout 耗尽，拖垮支付/退款/查询。
9. **【medium】CurrentPurchaseOrder 以 (tenant, kind=purchase, amount_fen, currency) 匹配过于宽泛**：同价位历史购买、过期前废弃 pending 单都落入匹配集；id ASC+pending 优先在多 pending 时任意挑选，废弃 pending 无终态回收，PurchaseStatus 的 out.Order 可能长期指向错误/过期订单，与跨报价开单叠加后支付入口可能指向旧单。
10. **【medium】步骤 6 成功后整条链路无原子性兜底与回收路径**：快照读失败/CreateOrder DB 错误/报价在预检与 consumeQuoteTx 间过期，都会在权威侧留下无本地订单的 awaiting 订阅；计划改价重发布后所有重试命中 ErrInvoiceQuoteMismatch 且无取消动作——适配层以 timeout_hours:0 把取消时序交给协调者（lago_purchase.go:241-243），而 seam 只有 create 命令、purchase.go 无任何取消/回收，孤儿 awaiting 订阅永久阻塞该租户购买。
11. **【medium】第 6 步消歧条件要求 State==AwaitingPayment 才判 PurchasePlanConflict**：租户已有 ACTIVE 购买（已购 plan B）再提交 plan A 报价时，SubmitCommand 失败、消歧读到 active 不满足条件、落入 ErrPurchaseUnavailable——确定性计划冲突被误答为 503 暂时不可用（reason=invalid_response），客户端按瞬时故障重试永远无法成功。

### 9.4 【本报告核验】终审 11 条逐条对 HEAD 对账表（rev2 新增；静态核验——源码形状+修复锚点，行为级验证未运行）

| # | 终审枚举（severity） | HEAD 核验证据（本报告会话 sed/grep/git 实读） | 判定 |
|---|---|---|---|
| 1 | 跨报价双单（high） | `purchase.go:217-284` 三道防线在案：同 quote 重放（GetOrderByQuote）→ awaiting 守卫（:223）→ **跨报价同价面重放 CurrentPendingPurchaseOrder（:238，R1-22）** → CreateOrder 撞租户级部分唯一索引（`order.go:120`，R2-26）→ **不限价面胜者重放 CurrentPayablePendingOrder（:262）+ link-less 清扫 SweepStaleLinklessPending 重试一次（:276，R3-26）**。枚举所述场景（同 plan 同价 re-quote）在 :238 即被重放拦截；异价 re-quote 被索引拦截——第二张可付渠道订单在 HEAD 无法开出 | **已修复**（描述为提出时点形状） |
| 2 | verify_db_watch 守卫恒假（high） | 四副本均为 `if not archived.exists():` 选择期降级 + WARNING + exit 2（flow-evidence-74:33、ocr1:40、ocr2:37、ocr3:37；R1-07/08/35/36+R3-17 修复形态；runs/ 回退整体移除，外来 TSV 场景不存在） | **已修复** |
| 3 | purchaseErrorMessage 死分支（medium） | `errors.ts:68-82` errorFromResult 已提取顶层 reason 入 details（R1-24）；`CheckoutPage.tsx:105-107` 按 `details.reason` 映射闭合中文（R2-01 补 409/500 令牌映射 :70/98，R3-02 补网络层兜底）；`!purchase.order`（:164）在 R1-V04（失败=503 错误信封）后仅剩防御分支，2xx 恒带订单 | **已修复**（防御分支实践中不可达，无用户可见影响） |
| 4 | AC1 重放断言未覆盖 endgame（medium） | `verify_ac_assertions.py:44-53`（本报告会话 sed）：不可见分支仍**无条件**断言 `recheck_subscription_status == "incomplete"`；else 分支仍要求 `invoice_status=="open" and payments<=1`——两条 charge-failure endgame PASS 路径（phases.py 文档化）会被误判 FAIL | **未解决（与 HEAD 一致）** |
| 5 | Purchase default 400+err.Error()（medium） | `handler/commercial.go` Purchase switch default 分支= `log.Printf` 原始错误 + **500 + "purchase failed" 固定闭合文案**（R1-09 修复形态，注释自述 never raw error text） | **已修复** |
| 6 | PM 轮询预算（medium） | `lago_purchase.go:49-51`：`purchaseRequestTimeout()` 已为**调用时求值**的 `subscriptionRequestTimeout + 3*outboundProviderTimeout + pmSyncWait`（=80s，覆盖全链，R1-12）——枚举「名义总和远超 25s」已过时；但 `waitForPaymentMethodSync` 函数体内 `deadline := time.Now().Add(pmSyncWait)` **仍不从 ctx 剩余预算派生**（本报告会话 sed 实读）——极端时序下轮询窗口名义值与实际可用预算不一致，由外层 ctx 截断兜底 | **部分残留**（次要形状） |
| 7 | 身份重放/422 恢复不查 Status（medium） | `createPurchaseSubscription` found 分支仅比较 `sub.PlanCode != payload.PlanCode` 即回成功回执；422 恢复分支 `found && sub.PlanCode == payload.PlanCode → receipt`——**均无 Status 检查**；`grep -rn 'CancelPurchase\|cancel_purchase\|CancelSubscription' internal/ --include='*.go'` = **0 命中**（购买身份无重建路径的前提在 HEAD 成立） | **未解决（与 HEAD 一致）** |
| 8 | 渠道客户端 SSRF/重定向（medium） | `wechat.go:187/199`、`alipay.go:202/213`：两渠道 client 均为 `secutils.NewSSRFSafeHTTPClientWithTransport(...)` + `transport.Proxy = http.ProxyFromEnvironment`（R1-14/16/17/19+R2-17/19 修复形态：共享 SSRFSafeDialContext 拨号期 IP 钉扎、校验型 Transport、跳数上限、代理保留） | **已修复** |
| 9 | CurrentPurchaseOrder 宽泛匹配（medium） | 已修部分：`OrderRow` 有 `CreatedAt` 列且排序 `created_at DESC, id ASC`（`repository/order.go:292-312`，R1-20）；link-less 死单由 Sweep 清扫、channel_failed 不占唯一索引槽、重放读取统一可付判定（:325-412，R2-28/R3-26）。**仍开放**：`CurrentPurchaseOrder` 仍以价格面匹配（无 plan/quote/订阅关联——同价不同 plan 的历史单可命中 PurchaseStatus 投影）；**带 checkout_url 的废弃 pending 单无超时回收**（Sweep 仅清无链接行 :402-412，订单无 closed/过期迁移） | **部分残留** |
| 10 | 步骤 6 后无原子性兜底/回收（medium） | 步骤 7 快照读失败（`purchase.go:197-199`）与 CreateOrder 其他错误（:285-287）路径在案且均直接上抛——权威侧 awaiting 订阅无补偿；seam 仅 `CommandKindCreatePurchaseSubscription` 一个购买命令（cancel grep 0 命中）；改价重试命中 ErrInvoiceQuoteMismatch 无取消动作 | **未解决（与 HEAD 一致）** |
| 11 | 消歧条件限 AwaitingPayment（medium） | `purchase.go:178-184`：消歧探针仍要求 `held.Purchase.State == domain.PurchaseStateAwaitingPayment && PlanCode != pub.PlanCode` 才答 ErrPurchasePlanConflict——active+异 plan 的确定性冲突仍落入 ErrPurchaseUnavailable（503/invalid_response） | **未解决（与 HEAD 一致）** |

**对账结论（对「HEAD 上到底几项 ≥medium 真实开着」的回答）：6 项开放——4 项全量（第 4/7/10/11 条）+ 2 项部分残留（第 6/9 条）；5 项已由批次 2 修复覆盖（第 1/2/3/5/8 条，枚举描述的是提出时点形状）。** 该结论与终审「9 项有效」相差 3 项，差异即终审判有效而本报告核验为已修复的条目——终审 triage 未持久化，无法进一步对齐；**D2 清偿工作量建议按本表 6 项计（含 2 项部分残留各自的收尾面）**。核验方法限定：静态核验（源码形状+修复提交锚点+grep 佐证），未运行行为级测试。

### 9.5 批次 1 轮 4 遗留（4 medium+1 low）逐项处置转录（rev2 新增；源=早版报告 `89cc052e9` §9.2，本报告覆盖取代该报告故转录于此）

| 编号 | 位置（轮 4 报告） | severity | 摘要 | 处置（含其后批次 2 的覆盖情况，本报告会话补注） |
|---|---|---|---|---|
| 4-01 | clients.py:400-404 | low | `last_error` 死存储 | 未核（纯代码风格）；批次 2 轮 1-4 报告中仍重复出现（clients.py:390 low），未修 |
| 4-02 | evidence/t02-duplicates.json | medium | 权威快照缺 `baseline.invoice_count` 与 `deferred_recheck` 键 | 批次 1 会话独立核验成立；**批次 2 轮 4 再次指认「已核验成立但未修复」**（快照与 HEAD 生成器键集漂移）——仍开放 |
| 4-03 | evidence/t02-retries.json | medium | 快照为旧路径输出（404/not_found、缺 subscription_status_after） | 同上，批次 2 轮 4 再指认两条独立缺口——仍开放 |
| 4-04 | phases.py:1297-1300 | medium | not-settled 终复查只探 incomplete，endgame 竞态误判 FAIL | 与 §9.4 第 4 条同域（ac 断言未覆盖 endgame PASS）；批次 2 各修复轮未见对应修复——按 §9.4 计入开放面 |
| 4-05 | phases.py:1215-1219 | medium | retries 探针缺 duplicates 式延迟复查（漏报方向） | **已由批次 2 R3-39 修复**（retries deferred recheck + 回归测试 `test_retries_deferred_recheck_catches_late_termination`，fix-log ocr-r3 章节） |
| 4-06 | phases.py:254-258 | low | 传输错误双重 note | 未核；low 候选 |
| 4-07 | ocr1-replay/verify_db_watch.py:64-73 | low | max_succeeded 无 run 隔离 | 触发场景（runs/ 回退）**已由批次 2 R3-17 整体移除**（归档唯一化）——场景层面消除 |
| 4-08 | ocr1-replay/verify_db_watch.py:26-27 | low | 「(ocr-2)」注释残留 | 未核；low 候选 |
| 4-09 | ocr1-replay/verify_db_watch.py:111 | low | samples 计数口径 | **已由 R1-V22（ocr-81-1，`4435b183b`）修复**（四副本改 `<midrun>/<total>` 双口径打印） |
| 4-10 | ocr2-replay/verify_ac_assertions.py:10-11 | low | 「unmodified」声明失实 | **已由 R1-V23（ocr-81-1）修复**（docstring 如实列明副本差异） |
| 4-11 | ocr2-replay/verify_db_watch.py:32-35 | medium | 归档 TSV「未提交」 | **批次 1 会话证伪**（16 文件全 git 跟踪） |
| 4-12 | ocr3-replay/verify_db_watch.py:64-67 | medium（悬置） | max_succeeded 与 rows_for 隔离不一致 | 与 4-07 同型，触发场景已由 R3-17 消除 |
| 4-13 | ocr3-replay/verify_ac_assertions.py:69-71 | low | 直接下标健壮性 | 未核；low 候选 |

> 批次 1 轮 4 的「1 项未指认 low」即上表 4-01/06/08/13 之一（无持久化指认）；4-09/4-10 已由后续修复覆盖后，残余 low 候选=4-01/06/08/13。

### 9.6 批次 2 轮 4 的 low findings 清单（rev2 新增；编排器终审判 7 项有效、指认未持久化——以下为该轮 **32 项 raw low 全集**，本报告会话 `grep -oE '\[[a-z]+ · (critical|high|medium|low)\]' | sort | uniq -c` 实数：2 high/12 medium/32 low，处置时从中点名有效 7 项）

| 主题 | 位置（`issue-72-ocr-round-4.md`，即 `ac8402204` 版） |
|---|---|
| lab 死代码/断言 | clients.py:390（last_error）；phases.py:1414（settled 分支 checks 缺 no_succeeded_payment） |
| 证据快照键缺 | evidence/t02-gating.json:11（缺 payments_non_succeeded_count——gate 阶段证据全部生成于该修复落地前） |
| replay 脚本行号/死代码/输出 | ocr2/verify_ac_assertions.py:12（行号失准）；ocr2/verify_db_watch.py:15（import glob）+ :126（归档输出未刷新）；ocr3/verify_ac_assertions.py:112（下标 vs 守卫风格）；ocr3/verify_db_watch.py:15（glob）+:22（RUNS 残留）；ocr1-replay/verify_ac_assertions.py:0（AC1 标签误写 AC2）；ocr1-replay/verify_db_watch.py:18（glob/RUNS）+:11（docstring 矛盾）；flow-evidence-74/verify_db_watch.py:11（glob/RUNS） |
| 后端小缺陷 | handler/commercial.go:531（PurchaseStatus default 400+err.Error()）；lago_purchase.go:552（429 折叠终态 invalid_response）+:166（isPlausibleHostname 放行单标签名）；service/purchase.go:404（重放丢失渠道失败 202 姿态）；contracts/commercial.ts:199（currency null 守卫不对称） |
| 前端 | CheckoutPage.tsx:153（重试按钮文案与已删机制脱节）+:269（checkoutHref 安全判定双写） |
| flow-evidence-82 脚本 | alipay_gateway_stub.py:56 与 alipay_sandbox_notify.py:30（KEY_DIR `__file__.rsplit` 裸启动/Windows 失效）；wechat_pay_stub.py:12（"Endpooints" 拼写）；browser_flow_82.mjs:12（WEB/¥99 硬编码、三脚本配置不一致）+:13（`new URL(...).pathname` Windows 路径）+:17-21（三脚本 30 行重复骨架）+:61（try 无 catch）；browser_paid_face_82.mjs:12/+:38（冗余条件）/+:51；browser_sync_face_82.mjs:11/+:40 |

## 10. 全部 Ruling 与延期事项

### 10.1 Ruling（均可溯源）

| # | Ruling | 出处 |
|---|---|---|
| R-1 | T02 付款激活通道=选项②（受支持 Provider；Stripe TEST 已实证） | `issue-72-user-rulings.md`（`639e6dc07` 入库）；补录 `docs/migrations/lago/t02-payment-activation/DECISION.md` §5 末 |
| R-2 | 75-a2 充值批次并发模型=选项 B（协调层承载；Lago Wallet 交易到账时刻幂等创建）；ADR-0012 修订 `552d98d12`+出处注记（批次 1 报告 §9.3 的出处存疑已由裁决入库闭合） | 同上；`issue-72-ledger-74.md`「后续裁决记录」 |
| R-3 | #81 spec L121 付款前 line-item 比对=选项 A 批准偏差（权威订阅面硬校验+无 charges 切片单行推导+付款时 InvoiceFees 完整复核；四条强制条件；仅限此条不外溢）；escalation dwfq-e9e3798d-1 经 spec owner 确认 | `issue-72-user-rulings.md`；`issue-72-plan-81.md` D2；`issue-72-ledger-81.md` Ruling 节 |
| R4 | done 票不重复实施（「已验证跳过」）；DAG 边只采信显式/高置信（75 业务+6 调度约束边，medium 推断边不直连）；#30 与 Lago 无耦合不入图 | `issue-72-issues-inventory.md:6`；`issue-72-dag.md:4-5,219-232` |
| R5 | #74 计划内 rulings 1–8（健康快照 byte-identical、修 lab 契约越权已上报、负对照改 3DS 卡、判据对齐 v1.53.0、_form 重试+Idempotency-Key、canary 环境变量、db_watch 两缺陷、manual 403 探测目标） | `issue-72-ledger-74.md:81-119` |
| R6 | **#74 F1 主 Agent 裁决：接受计划偏差，不 revert**（证据生效、#81/#82 解锁） | `issue-72-ledger-74.md:186-212`；`ec14bbf94` |
| R7 | #81 计划审查两轮 8+2 条处置（stub 精确匹配/死锁修复/env 恢复路径/测试落点/escalate 取得 R-3 等）；执行期 R1（D2 选项 A 执行确认+F12 proration 佐证）/R2（F11 绑定链承载 PM attach/default/同步轮询） | `issue-72-ledger-81.md:43-59,109-112` |
| R8 | **#82 流程修复 Ruling**（4 条）：缺陷 2 修复=接口契约（MerchantID()）而非局部补丁；缺陷 3 修复=对 pinned 契约实证而非先例假设服从（纠正 R1-V05 的 PUT 假设）；回调匿名可达安全边界（仅 POST 前缀+签名验签 fail-closed+负控）；修复边界声明（回调入账链可用≠#82 达成，激活链仍冻结） | `59465f8fe`（ledger-82 流程修复章节，历史 `e0364d196^` 可查） |
| R9 | **#82 第 2 轮裁定：无产品缺陷**——d2-falsified 维持冻结（修复=越权重设计，升级归 T02 §5）；沙箱凭据披露不变；环境痕迹根因=DB 副本写共享 authority，清理+固化流程规则（「验证/自验实例永远不得以复制的 WeKnora DB 对共享 Lago authority 写入」）；证据缺口如实补跑（rv1-gotest-rerun-by-fixer.txt） | `eea89716e` |
| R10 | **#82 revert 裁定**：流程验证未过（激活链不可达）→ 实现移出集成分支、保留分支 | `e0364d196` |
| R11 | OCR 各轮逐条 Ruling（批次 1 轮 1–3 16 项、#81 增量 23+4 项、批次 2 轮 1–3 27 组，含级别升降、修复形态选择、副本同病同修等） | `issue-72-ocr-fix-log.md` 全文（1415 行） |

### 10.2 延期事项（等待用户/主 Agent 输入）

| # | 事项 | 状态与建议 |
|---|---|---|
| D1 | **T02 §5 重议（主链唯一卡点）**：t10 证伪 #82 计划的 D2 结算触发链，Task 5-10 冻结——激活机制 redesign 与修订版 #82 计划产出权在 spec/ADR owner。**机制输入（供 redesign 启动）**：被证伪的 D2 算法为「读到卡住的 payment 行 → 取 invoice id → 取消 Stripe intent → 切换 settle pm → `POST /api/v1/invoices/{id}/retry_payment`」。t10 探针（lab `deploy/lago-lab/payment-trigger/`，run `6f45ed2f`）逐链结果：**P1 FAIL（F5 证伪）**——`GET /api/v1/payments?external_customer_id=` 在整个卡住窗口 200+空列表（pinned `payments_query.rb` 的 `visible_payable_condition` 只返回可付发票处于 VISIBLE_STATUS 的行，stuck 3DS 收款把 gating invoice 停在 open→closed 即 INVISIBLE，payment 行恰在 settle 需要它时不可见）；**P2d FAIL（F3 收窄）**——`retry_payment` 对隐藏发票在进入服务逻辑前即 404 `invoice_not_found`（`invoices_controller.rb` 的 `.visible.find_by`）；P2c NOT TRIGGERED（pm 再导入的触发点是 provider-customer 绑定首次创建，upsert 不触发，本地栈无 webhook）。结论：pinned Community v1.53.0 上该链无可用窗口；manual `POST /api/v1/payments` 为 Premium 403、webhook 不可假设。**全文 93 行见 `git show e0364d196^:docs/migrations/lago/t10-payment-trigger/DECISION.md`（或分支 `codex/issue-72-lago-82`）**；另有 gating invoice proration 金额剪裁（首期 1980=9900×6/30）与 Lago customer create 422 不回滚两个观察项同为重议输入（`issue-72-flow-evidence-82/README.md` 观察节） | 未决 |
| D2 | **终审遗留清偿（工作量按 §9.4 对账=6 项开放：4 全量+2 部分）**：第 4 轮后无修复轮；终审「9 项有效」与对账「6 项」之差=终审判有效而核验已修复的 3 项（triage 未持久化） | 执行顺序建议：按 §9.4 表逐项处置（4 项全量修复或记录豁免 + 第 6/9 条各自收尾）→ low 按 §9.6 清单点名处置；判绿门槛沿用早版报告 D5（离线回归+真实重放+复审 0 项 ≥medium 遗留）。**执行人：主 Agent 派修复员（报告员无权改非报告文件）** |
| D3 | **支付宝沙箱凭据**（`ac4-sandbox-credentials-unavailable`）：替身链只证明验签→入账，不构成真实沙箱付款证据 | 未提供；#82 重启后 AC4 仍只能「带残余」 |
| D4 | **AGPL 生产批准**（#101/#103 范围） | 法务结论缺失；生产部署法律前置门（长期延期项） |
| D5 | **推送/合并（执行人：主 Agent/编排器）**：集成分支+3 Issue 分支均未推送（`git ls-remote` 实查） | 顺序：先清偿 D2 → `git push` 四分支 → PR `codex/issue-72-lago` 至 main；**本报告自身即在该未推送分支上（归档时序见卷首说明）**，合并 main 后方为仓库正式归档 |
| D6 | **GitHub 关票（执行人：编排侧 gh）**：#74/#81 已完成/验证但 OPEN（§2.4） | 建议：#74/#81 关票（评论引用 §9.4 对账表与 §9.5/§9.6 遗留清单，避免误读为零缺陷）；**#82 保持 OPEN**，阻塞原因更新为「T02 §5 重议」 |
| D7 | **`codex/issue-72-lago-82` 分支去留（决策人：用户）**：含可复用设计资产（settle 命令、伪确定性结算、购买钱包身份、t10 探针 lab、777 行计划+185 行账本） | 建议保留至 T02 §5 重议产出修订版计划后，由用户决定复用（rebase）或删除 |

## 11. 未完成节点、遗留风险、low findings、需要用户决策的内容

- **未完成节点（24/33）**：#82 直接阻塞（D2 证伪+T02 §5 重议）；#83–#105 依赖传播 23 票。除 D1/D3 外无其他外部输入缺口。
- **遗留风险**：① **§9.4 对账 6 项开放 findings**（其中身份死锁/孤儿 awaiting 订阅属可用性风险面、AC1 断言误判属证据工装风险）；② 权威证据快照与 HEAD 生成器键集漂移（t02-duplicates/t02-retries/gating——§9.5 4-02/4-03 仍开放，旧证据无法通过最新 verify 脚本）；③ #82 revert 后 `docs/plans/issue-72-plan-82.md`/`ledger-82.md` 仅存分支与历史，主链接手者需从分支取回设计资产（§10.2-D7）；④ 共享 Lago authority 栈的租户 id 撞名隐患——**上下文（rev2 补）**：#81 验证用租户 1/2/3/4（issue81-flow@/b/c/admin 对应）、#82 验证用 1/2（A/B）+复验 C=3/D=4；上轮修复员的 DB 副本实例用了租户 3/6 写共享 authority（3 撞 C 已回真、6 的顾客已删但其 gated 订阅 `weknora-tenant-6-purchase` 仍在——v1.53.0 共享 API 订阅无 destroy），故未来轮 WeKnora 租户 id 应 **≥5 且 ≠6**（即 5、7、8…），或换新 Lago 栈；⑤ #87 范围 PG 并发测试 2 FAIL 悬置（需 SAAS_TEST_PG_DSN，本报告会话未运行）；⑥ DAG/清单状态表停留于批次 2 开工前口径（#81=todo），未随完成刷新（可追溯性缺口，同早版报告）；⑦ `.worktrees/issue72-lago` 工作区有未跟踪 `config.yaml`/`data/`（本地运行产物）。
- **low findings：终审判 7 项有效未解决（指认未持久化）**；轮 4 raw low 全集 32 项已列于 §9.6，处置时从清单点名。
- **需要用户决策**：§10.2 D1–D7。
- **验收者复核命令（本报告会话实跑清单）**：见 §7 ★ 行；**未运行**：任何测试套件、真实栈重跑、OCR 第 5 轮、`run_lab.py`、PG twin 测试、§9.4 对账的行为级验证（对账为静态核验）。

## 12. 结论

两个编排批次的目标部分达成：**#74（批次 1）与 #81（批次 2）实施并经真实流程验证完成**（#81 增量 OCR passed-r2），#81 使 #72 主链推进到「Quote→待付款 Invoice→渠道订单」全链路可用、矩阵第 4 项闭合；**#82 实现完成但真实流程验证 3 轮未过**——#82 计划的 D2 结算触发链被 t10 证伪、激活后半链在计划冻结下不可达，按裁定 revert（实现保留分支 `codex/issue-72-lago-82`），#83–#105 全线依赖传播阻塞，主链当前卡点收敛为 **T02 §5 重议**（机制输入见 §10.2-D1）。治理面：批次 2 全量 OCR 4 轮、前 3 轮 27 组 findings（含 1 项 critical）修复并重放；**终审给定 9 项有效 critical/high/medium+7 项 low 未解决且无修复轮——本报告逐条对 HEAD 核验后判定实际开放 6 项（4 全量+2 部分残留），对账表与证据见 §9.4**；分支未推送、#74/#81 未关票。上述各项均如实上报，未粉饰。

## 13. 附录：编排器「各 Issue 实施结果 JSON」原文（33 票，任务下发方给定，本附录即其入库存档）

```json
[{"issueNumber":73,"title":"[Lago 01] 启动固定版本的 Lago Community 集成环境","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":74,"title":"[Lago 02] 证明外部付款可以激活 payment-gated Subscription","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":75,"title":"[Lago 03] 证明 Wallet 批次到期、消费顺序和撤回语义","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":76,"title":"[Lago 04] 证明 Task Pricing Group 可以形成可核对计价批次","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":77,"title":"[Lago 05] 用 Lago readiness 纵向切片扩展 Commercial Platform seam","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":78,"title":"[Lago 06] 一个空间自动获得独立 Lago Customer","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":79,"title":"[Lago 07] 从 WeKnora 管理后台发布不可变 Plan Version","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":80,"title":"[Lago 08] 新空间以 Base Plan 获得权益和月度额度","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":81,"title":"[Lago 09] 从 Quote 创建待付款 Invoice 和 incomplete Subscription","status":"已完成","note":"HEAD=e26a90d61458c5be52a8d8bda3a2e64ec9c306bf，流程验证=Playwright 无头真实浏览器（browser_navigate/snapshot/click/screenshot，真实登录、真实页面渲染与断言）+ 真实 API 链路（curl 真 HTTP 调用 WeKnora :8091 与 Lago :48889）+ Lago DB psql 直查（open invoice 的 API 途径不可见，按计划 F3-F5 用 DB 权威计数）。（11 步，证据 16 件）","unresolved":[]},{"issueNumber":82,"title":"[Lago 10] 支付宝付款后恰好一次激活套餐","status":"阻塞","note":"真实流程验证 3 轮未通过：[机制级][计划已知边界，第 2 轮裁定维持] d2-falsified-t10-p2-fail：激活后半链仍不可达——可信回调入账后订单 payment=paid，但 Lago 订阅保持 incomplete、无 succeeded Payment（DB payments 仅 failed=6/requires_action=6）、无套餐 Credits 钱包、Entitlements 404（rv2-lago-after-paid.txt）；「订单与 Billing 页变已生效」不可达，呈现停在「已付款，权益处理中」。第 2 轮裁定已核实此为计划冻结（Task 5-10 冻结、修订版计划产出权在 spec/ADR owner），修复它=越权重设计，升级归 T02 §5 重议。；[披露] ac4-sandbox-credentials-unavailable：仍无支付宝沙箱凭据，替身为本地 RSA+回环 stub+同源签名 notify——验证修复后完整渠道入账链，不构成真实沙箱钱包付款证据。；[读面观察，非缺陷] 单租户双订单下 GET /purchase 的 CurrentPurchaseOrder 选中未付款的 wechat 单（ord_ee4a8f2453cc1ddc pending），alipay 单 paid 由 GET /orders/:id 直证——读面选择行为已如实补档（rv2-lago-after-paid.txt）。（merge 已 revert，实现保留在分支 codex/issue-72-lago-82）","unresolved":[]},{"issueNumber":83,"title":"[Lago 11] 微信付款复用同一激活流程","status":"阻塞","note":"前置未完成（依赖传播）：[issue-82]","unresolved":[]},{"issueNumber":84,"title":"[Lago 12] 异常付款不会扩大权益","status":"阻塞","note":"前置未完成（依赖传播）：[issue-82, issue-83, issue-85]","unresolved":[]},{"issueNumber":85,"title":"[Lago 13] 购买 Credits 并在付款后到账","status":"阻塞","note":"前置未完成（依赖传播）：[issue-82, issue-83]","unresolved":[]},{"issueNumber":86,"title":"[Lago 14] 套餐额度与充值额度按到期顺序消费","status":"阻塞","note":"前置未完成（依赖传播）：[issue-85]","unresolved":[]},{"issueNumber":87,"title":"[Lago 15] 收费调用前原子预占 Task Budget 与空间 Credits","status":"阻塞","note":"前置未完成（依赖传播）：[issue-86]","unresolved":[]},{"issueNumber":88,"title":"[Lago 16] 一个 Usage Event 完成 Settlement Batch 核对","status":"阻塞","note":"前置未完成（依赖传播）：[issue-87]","unresolved":[]},{"issueNumber":89,"title":"[Lago 17] 并发和委派共享同一 Task Budget","status":"阻塞","note":"前置未完成（依赖传播）：[issue-87, issue-88]","unresolved":[]},{"issueNumber":90,"title":"[Lago 18] 计价延迟、未知结果和费用上界违规正确暂停 Task","status":"阻塞","note":"前置未完成（依赖传播）：[issue-88, issue-92]","unresolved":[]},{"issueNumber":91,"title":"[Lago 19] BYOK 只免除模型维度 Credits","status":"阻塞","note":"前置未完成（依赖传播）：[issue-88]","unresolved":[]},{"issueNumber":92,"title":"[Lago 20] 用量修正不会改写历史","status":"阻塞","note":"前置未完成（依赖传播）：[issue-88]","unresolved":[]},{"issueNumber":93,"title":"[Lago 21] 升级立即生效并补发当月 Credits 差额","status":"阻塞","note":"前置未完成（依赖传播）：[issue-82, issue-86, issue-92]","unresolved":[]},{"issueNumber":94,"title":"[Lago 22] 降级、年付月发和到期回 Base Plan","status":"阻塞","note":"前置未完成（依赖传播）：[issue-93]","unresolved":[]},{"issueNumber":95,"title":"[Lago 23] 充值退款先锁定、再退支付宝、最后撤回 Credits","status":"阻塞","note":"前置未完成（依赖传播）：[issue-82, issue-85, issue-86, issue-94]","unresolved":[]},{"issueNumber":96,"title":"[Lago 24] 套餐退款通过 Credit Note 和权益撤回完成","status":"阻塞","note":"前置未完成（依赖传播）：[issue-92, issue-93, issue-95]","unresolved":[]},{"issueNumber":97,"title":"[Lago 25] 微信退款达到与支付宝相同的商业语义","status":"阻塞","note":"前置未完成（依赖传播）：[issue-83, issue-95, issue-96]","unresolved":[]},{"issueNumber":98,"title":"[Lago 26] Webhook 与定期对账使商业投影收敛","status":"阻塞","note":"前置未完成（依赖传播）：[issue-84, issue-85, issue-93, issue-95, issue-96]","unresolved":[]},{"issueNumber":99,"title":"[Lago 27] worker 崩溃和 Lago 故障不会丢用量或提前释放预占","status":"阻塞","note":"前置未完成（依赖传播）：[issue-90, issue-92]","unresolved":[]},{"issueNumber":100,"title":"[Lago 28] Billing Center 统一展示稳定产品状态","status":"阻塞","note":"前置未完成（依赖传播）：[issue-91, issue-94, issue-97, issue-98, issue-99]","unresolved":[]},{"issueNumber":101,"title":"[Lago 29] 完成计费数据最小化、凭据隔离和 AGPL 上线门槛","status":"阻塞","note":"前置未完成（依赖传播）：[issue-97, issue-98, issue-99]","unresolved":[]},{"issueNumber":102,"title":"[Lago 30] 空间注销时停止收费、去标识化并保留财务历史","status":"阻塞","note":"前置未完成（依赖传播）：[issue-94, issue-96, issue-98, issue-101]","unresolved":[]},{"issueNumber":103,"title":"[Lago 31] 部署可观测的生产 Lago 并满足计费延迟目标","status":"阻塞","note":"前置未完成（依赖传播）：[issue-98, issue-99, issue-101]","unresolved":[]},{"issueNumber":104,"title":"[Lago 32] 从备份恢复 Lago 并完成商业对账","status":"阻塞","note":"前置未完成（依赖传播）：[issue-102, issue-103]","unresolved":[]},{"issueNumber":105,"title":"[Lago 33] 切换 Lago、关闭回退窗口并移除 OpenMeter","status":"阻塞","note":"前置未完成（依赖传播）：[issue-86, issue-89, issue-91, issue-92, issue-94, issue-97, issue-100, issue-102, issue-104]","unresolved":[]}]
```
