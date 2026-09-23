# Issue #72 最终交付报告（Lago 计费迁移编排：#74 实施轮 + OCR 治理）

> 生成：2026-09-23（同日修订 rev2：按独立读者评审意见 21 项修订——OCR-4 严重级别定论、轮 4 逐项处置表、断言计数体系说明、提交计数勘误 25→24、全量 GitHub 状态核对、矩阵 24 项全对照、编排器 JSON 原文附录等；修订以 amend 并入同一报告提交）｜ Worktree：`.worktrees/issue72-lago` ｜ 集成分支：`codex/issue-72-lago`
> 基线：`29c1e56353b2b36be242018cecb43bcb3a5ef7c8`（docs: plan pass b contract and ownership freeze）｜ 实施终点：`c9e4033d30`（`issue-72: ocr round 4`，基线..终点 **24 个提交**，`git rev-list --count` 实查=24；本报告提交为第 25 个，哈希以 `git log -1` 为准）｜ 早版勘误：rev1 曾误写「25 个提交」，实为 24，§5.2 表已逐哈希对齐。
> **本报告覆盖并取代早版**（`6649e06ba`，2026-09-23 03:09 的调查轮报告，当时基线..HEAD 仅 2 个 docs 提交、#74 尚未实施；早版仍可在 git 历史查证）。
> 本轮（基线..实施终点）交付：① 33 票清单+依赖 DAG 修订（`7cf755474`→`dafba8851`）；② **#74 [Lago 02] 唯一实施票：AC1–AC4 运行时证据全 pass 并经真实流程验证**（merge `175de8b4f`）；③ OCR 4 轮审查 + 3 轮修复（第 1–3 轮 16 项有效 findings 全部清偿；第 4 轮遗留 **4 项有效 critical/high/medium 类别未解决 + 1 项 low**，级别定论见 §9.2）。
> 口径说明：各票最终状态以编排器给定实施结果 JSON 为准（**原文全文附录于 §13，供逐字核对**；该 JSON 由任务下发方在本报告的任务指令中给定、未单独入库，附录即其入库存档）；GitHub 状态为本报告会话 `gh issue view` **全量 34 票核对**（§2）。

## 1. 执行摘要

本轮在 33 个子 Issue 中实施并完成 **1 票（#74）**，其余维持「7 票已验证跳过（前轮成果）+ 25 票阻塞」。#74 从 blocked-env（缺 Stripe TEST 密钥）起步：用户于本轮提供 `~/.zcode/issue72-stripe.env`（仅测试凭据），#74 改判 todo（`dafba8851`），随后在独立 Issue 分支 `codex/issue-72-lago-74` 上经 11 轮真实运行（1 轮 blocked-env + 9 轮 fail 暴露工具契约缺陷 + 最终 run11 `5d06a277` 九阶段全 pass）取得 AC1–AC4 运行时证据，主 Agent 裁决接受计划偏差后经 merge `175de8b4f` 并入集成分支，并由流程验证员在真实环境独立复跑验证（run `3dc51207`，21/21 断言 + DB-WATCH PASS，证据 `docs/plans/issue-72-flow-evidence-74/`）。其后 OCR 审查 4 轮：第 1–3 轮共 16 项有效 findings 全部根因修复并每轮重放真实流程（证据 `issue-72-ocr{1,2,3}-replay/`），**第 4 轮报告后无修复轮，遗留 4 项有效 critical/high/medium 类别 findings 未解决 + 1 项 low finding**（轮 4 原文无 critical/high 级条目，该 4 项按级别落位均为 medium，定论与逐项处置见 §9.2）。主链仍卡在两项用户输入：**T02 三选项裁决**（(a) Premium manual /(b) 受支持 Provider /(c) 改 spec，解锁 #81/#82）与 **#75-a2 充值批次设计**（注意：`552d98d12` 已在 ADR-0012 写入「75-a2 裁决 B：协调层承载」修订记录，但其裁决出处无法从过程文档溯源——与编排器 JSON 中 #85 仍称「裁决未提供」不一致，效力待用户确认，§9.3/§10.2-D2）。集成分支**未推送远端**（`git ls-remote` 实查，远端仅历史分支 `lago-73-community-env`/`lago-integration`）；#74 GitHub 仍 OPEN（全量核对见 §2；关票建议见 §10.2-D4）。

## 2. 每个子 Issue 的状态与证据（33 票，编排器给定 JSON 全量转录）

状态分布：**已验证跳过 7 ｜ 已完成 1 ｜ 阻塞 25**（直接阻塞 9 + 依赖传播 16）。

**GitHub 状态全量核对（本报告会话实跑）**：`for i in 72 $(seq 73 105); do gh issue view $i -R 1123786563/WeKnora-fork01 --json state; done`（首轮 5 票瞬时查询失败，隔 2s 重试全部成功，34/34 无缺漏）→ **#72 OPEN；#73/75/76/77/78/79/80 CLOSED（7 票）；#74 及 #81–#105 全部 OPEN（25 票）**——与 JSON 状态分布完全一致，无漂移。以下各节标题中的 GitHub 状态均以此全量核对为据。

### 2.1 已完成（1 票）

| Issue | 标题 | 状态 | 证据（编排器 note + 本报告会话核验） |
|---|---|---|---|
| #74 | [Lago 02] 证明外部付款可以激活 payment-gated Subscription | **已完成** | 编排器 JSON 记 `HEAD=175de8b4fd…`——该哈希是**集成分支的 merge 提交**（流程验证所基状态，`issue-72-flow-evidence-74/VERIFY-SUMMARY.txt:4`「环境: 集成分支 codex/issue-72-lago @ 175de8b4f」）；per-issue 分支 `codex/issue-72-lago-74` 自身 tip=`ec14bbf94`。两提交关系（本报告会话实查）：merge parents=`552d98d12`+`ec14bbf94`；`552d98d12`（ADR-0012 修订）**不是** `ec14bbf94` 的祖先（仅存在于集成分支线）；`git diff ec14bbf94 175de8b4f` 仅 `docs/adr/0012-…md` +17 行——即 merge 结果 = Issue 分支全部内容 + ADR-0012 修订，别无差异。流程验证=真实 API 链路 + DB 投影核验 + Playwright 无头真实渲染（前端仅人工观察不作判据）；`run_lab.py` 串真实 Lago REST/GraphQL 与真实 Stripe TEST 扣款；DB 观察者每 2s 只读 psql 采样投影（16 步/16 件口径说明见 §8）。落地证据：`issue-72-ledger-74.md`（AC1–AC4 全 pass 判定 `:59-62`）、`docs/migrations/lago/t02-payment-activation/`（8 JSON+DECISION.md）、`docs/plans/issue-72-flow-evidence-74/`（23 个 git 跟踪文件）。**遗留**：OCR 第 4 轮 4 项 medium 未解决（§9.2）；GitHub 未关票（关票建议 §10.2-D4） |

### 2.2 已验证跳过（7 票，前轮成果，本轮调查复核后未重复实施，GitHub CLOSED〔全量核对〕）

| Issue | 标题 | 前轮验证证据（清单 §2 + 早版报告核验） |
|---|---|---|
| #73 | [Lago 01] 固定版本集成环境 | 46 测试+62 subtests 实跑；`deploy/lago/evidence/t01-{images,health,contract}`（5/5 digest MATCH、ready） |
| #75 | [Lago 03] Wallet 批次语义 | e1-e4 真实栈实测+104 测试；`docs/migrations/lago/t03-wallet-semantics/verdict.md`（a2 BLOCKED：第 7 活跃钱包 422） |
| #76 | [Lago 04] Pricing Group 计价批次 | 60 测试+6 份真实栈 JSON（64/64 精确核对、p95 6.893s） |
| #77 | [Lago 05] Commercial Platform seam | 冻结 seam `platform.go:221-230`；go build/test 126 包 ok；`t05-health.json` |
| #78 | [Lago 06] 空间独立 Lago Customer | 三层幂等+租户隔离；账本 `docs/plans/ledgers/lago-78.md`；`t06-*` |
| #79 | [Lago 07] 不可变 Plan Version | 六轴校验+DB trigger；账本 `lago-79.md`；`t07-run.txt` |
| #80 | [Lago 08] Base Plan 权益额度 | 懒链+迁移 000183；账本 `lago-80.md`；`t08-run.txt` |

> **证据性质声明（重要）**：上表测试数字（46+62 subtests/104/60/126 包等）系清单 §2 对前轮实跑的**文字转述**，本轮与早版报告均未重跑；仓库内归档的是真实栈操作时间线（`t0*-run.txt` 等）而非测试 stdout。**8/33 完成票中的这 7 票在本轮证据链中无可直接核对的支撑——接手者不应将其视为已复核结论**；如需第一手证据须按 §11 命令重跑（`python3 -m pytest deploy/lago/ -q`、`go test ./...` 等）。

### 2.3 直接阻塞（9 票，GitHub OPEN〔全量核对〕，本轮零实施）

| Issue | 标题 | 阻塞原因（JSON note 摘要） |
|---|---|---|
| #81 | [Lago 09] Quote→待付款 Invoice | Blocked by #74：T02 三选项裁决仍未提供，payment-gated incomplete Subscription 创建机制无法定型（#78/#79 已满足；#74 运行时证据到位后此裁决仍是剩余卡点） |
| #82 | [Lago 10] 支付宝恰好一次激活 | Blocked by #74/#81：可信 Payment 录入通道未裁决+无可激活实体（`lago.go:684` 刻意不带 activation_rules，本报告会话已核原文） |
| #85 | [Lago 13] 充值到账 | Blocked by #75/#82/#83：#75-a2 设计裁决（12 个月到期撞六活跃钱包硬上限：≤5 并发批次 vs 协调层承载+修订 ADR-0012）未提供；付款链未通（另见 §9.3 不一致说明） |
| #87 | [Lago 15] 原子预占 | Blocked by #86（OPEN）：空间保守余额需 Lago Wallet 权威投影；#86→#87→#88 链被 #75-a2 悬置（waves L140） |
| #88 | [Lago 16] Settlement Batch 核对 | Blocked by #87（OPEN 未开工）：全部 AC 以已预占调用为对象；整链传递依赖 #74 与 #75-a2（waves L138-140） |
| #94 | [Lago 22] 降级/年付/到期 | Blocked by #93（OPEN）：与升级共用同一 ExternalSubscriptionID 订阅链切换协调器 |
| #97 | [Lago 25] 微信退款对齐 | Blocked by #83/#95/#96（全 OPEN）：P03 资格 seam 未实现、Credit Note 撤权模型未落地、微信 REFUND.* 契约与退款渠道测试为零 |
| #100 | [Lago 28] Billing Center | Blocked by #91/#94/#97/#98/#99（全 OPEN 无账本）：「等待同步」须 #98 对账收敛、「预占」须 #99 故障安全 |
| #103 | [Lago 31] 生产 Lago+延迟目标 | Blocked by #98/#99/#101（3/4 前置 OPEN）：Helm 生产交付未开始；AGPL 上线门槛为法律前置门（spec L179-180） |

### 2.4 依赖传播阻塞（16 票，note=「前置未完成（依赖传播）」，GitHub OPEN〔全量核对〕）

| Issue | 标题 | 传播链 |
|---|---|---|
| #83 | [Lago 11] 微信复用激活 | [82] |
| #84 | [Lago 12] 异常付款不扩大权益 | [82, 83] |
| #86 | [Lago 14] 到期顺序消费 | [85] |
| #89 | [Lago 17] 并发共享 Task Budget | [87] |
| #90 | [Lago 18] 延迟/超界暂停 | [88] |
| #91 | [Lago 19] BYOK 豁免 | [88] |
| #92 | [Lago 20] 用量修正不改历史 | [88] |
| #93 | [Lago 21] 升级补差 | [82, 86] |
| #95 | [Lago 23] 充值退款三段式 | [82, 85, 86] |
| #96 | [Lago 24] 套餐退款 Credit Note | [92, 93] |
| #98 | [Lago 26] Webhook 对账收敛 | [84, 85, 93, 95, 96] |
| #99 | [Lago 27] 故障不丢用量 | [90, 92] |
| #101 | [Lago 29] 最小化/AGPL | [97, 99] |
| #102 | [Lago 30] 空间注销 | [94, 96, 98, 101] |
| #104 | [Lago 32] 备份恢复对账 | [102, 103] |
| #105 | [Lago 33] 切换/移除 OpenMeter | [86, 89, 91, 92, 94, 97, 100, 102, 104]（收官票） |

> 注：DAG 状态表（`issue-72-dag.md:264-300`，最后修订于 `dafba8851`）仍标 #74=todo——该文件在 #74 开工前定稿后未再更新，最新状态以本节 JSON/账本为准（清单 `issue-72-issues-inventory.md` 同样停留在 #74=todo，均为可追溯性缺口，见 §11）。

## 3. #72 总体验收标准覆盖情况（矩阵 24 项全对照）

父票 #72 不入 DAG 节点，其验收由 Spec `docs/specs/2026-09-20-lago-billing-migration-design.md` 承载：行为与契约矩阵 1–24 项（spec:212-237）与 Completion gate（spec:246-248），本报告会话均已核原文。**逐项对照**（票号↔矩阵项映射依据 DAG 依赖表与清单 §2；「部分」=有实验/语义证据但产品化票未完成）：

| 矩阵项 | 对应票 | 状态 |
|---|---|---|
| 1 Community 能力+AGPL | #73（能力）+ #101/#103（AGPL） | **部分**：能力子集✓；AGPL 审查结论不存在 |
| 2 租户隔离 | #78 | ✓（前轮） |
| 3 不可变目录 | #79 | ✓（前轮） |
| **4 Quote 匹配** | #81 | **未覆盖**（OPEN；rev1 漏列，rev2 补） |
| 5 外部付款激活 | #74（实验证明）+ #82/#83/#84（产品化） | **部分**：provider 轨道运行时证实（本轮）；微信/支付宝产品链未开工 |
| 6 充值到账 | #85/#86 | 未覆盖（OPEN） |
| 7 付款异常 | #84 | 未覆盖 |
| 8 Credits 顺序 | #75（语义验证）+ #86（实现） | **部分**：语义 verdict✓；实现未开工 |
| 9 用量身份 | #88 | 未覆盖 |
| 10 Settlement Batch | #76（批次核对形态）+ #88（实现） | **部分**：实验形态✓；实现未开工 |
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
| 22 隐私与凭据 | #101 | **部分旁证**：#74 证据链密钥纪律（secrets scan 0 hits+形状扫描）仅为实验链路旁证，正式项未覆盖 |
| 23 备份恢复 | #104 | 未覆盖 |
| 24 OpenMeter 移除 | #105 | 未覆盖（`internal/container/container.go:244` 仍装配 `ommeter.NewGatewayFromEnv`，本报告会话已核原文） |

- **判定**：#72 迁移整体**未完成**；进度=调查 33/33、实施 8/33（7 前轮+本轮 #74）。Completion gate 关键项未达成：AGPL 审查、24 项 gate 证据归档（DAG W15 未执行）、回退窗口关闭流程（#105）均缺。#72 GitHub OPEN（全量核对）。

## 4. 制品与证据路径索引

| 类别 | 路径 | 说明 |
|---|---|---|
| 层级树 | `docs/plans/issue-72-issues-inventory.md:5` | 33 票均为 #72 直接子票，单层无嵌套（320 行） |
| 子 Issue 清单 | `docs/plans/issue-72-issues-inventory.md` | §1 总览（done 7/blocked 9/todo 17，#74=todo 系修订时点状态）+ §2 逐票 + §3 外部依赖 |
| DAG | `docs/plans/issue-72-dag.md`（308 行） | Mermaid + 74 条边依赖表 + 拓扑序 + W1–W15 波次 + 阻塞节点 + 状态表 + Kahn 校验记录 |
| 票级计划 | `docs/plans/issue-72-plan-74.md`（805 行） | #74 实施计划：调查结论/AC 原文/约束/文件地图/追踪矩阵/自检/第 1 轮审查修订 8 处/执行期计划偏差记录 |
| Ledger | `docs/plans/issue-72-ledger-74.md`（221 行） | #74 执行账本：快照/执行清单/AC 判定/Ruling 1–8/测试记录/终检/F1 三轮审查+主 Agent 裁决/上报事项 6 条 |
| OCR 报告 | `docs/plans/issue-72-ocr-round-{1,2,3,4}.md` | 4 轮原始 findings（10/7/5/13 项） |
| OCR 修复记录 | `docs/plans/issue-72-ocr-fix-log.md`（435 行） | 第 1–3 轮 16 项有效 findings 的逐条 Ruling/RED 证据/回归测试/真实重放记录；**无第 4 轮章节** |
| 流程验证证据 | `docs/plans/issue-72-flow-evidence-74/` | 23 个 git 跟踪文件：VERIFY-SUMMARY.txt、12 个 t02-*.json、t02-run.txt、3 张前端截图、TSV/输出/校验脚本 |
| OCR 重放证据 | `docs/plans/issue-72-ocr{1,2,3}-replay/`（各 16 个跟踪文件） | 每轮修复后的真实流程重放（run JSON+断言输出+DB-watch TSV） |
| 运行时证据（晋升） | `docs/migrations/lago/t02-payment-activation/`（8 JSON+README+DECISION.md） | #74 权威证据目录（deploy 侧 `deploy/lago-lab/payment-activation/evidence/` 的 byte-identical 晋升，cmp×8；**注：该目录快照已滞后于最终代码，见 §9.2 处置表 4-02/4-03**） |
| 前轮证据 | `deploy/lago/evidence/t0*`、`docs/migrations/lago/{t03,t04,t07,t08}`、`deploy/lago-lab/{payment-activation,pricing-group,wallet-semantics}` | 基座 7 票证据（`ls` 实查存在） |
| Spec/ADR | `docs/specs/2026-09-20-lago-billing-migration-design.md`；`docs/adr/0012-lago-as-commercial-billing-authority.md`（本轮新增修订记录节，`552d98d12`） | 事实源 |
| 编排器状态 JSON | 本报告 §13 附录 | 33 票实施结果原文（rev2 起入库存档） |

## 5. Git 事实：worktree / 分支 / 基线 / HEAD / 关键提交

### 5.1 基本事实（本报告会话实跑，命令见 §11）

- **Worktree**：`.worktrees/issue72-lago`，分支 `codex/issue-72-lago`（`git worktree list`/`branch --show-current` 实查），工作区干净。**#74 实施所用 worktree `.worktrees/issue72-n74`（ledger-74:7）现已不存在**（`ls` 实查 No such file or directory；`git worktree list` 亦无此条目）——实施后已清理，其分支 `codex/issue-72-lago-74` 仍保留本地。
- **基线**：`29c1e56353b2b36be242018cecb43bcb3a5ef7c8`；**实施终点**：`c9e4033d3079e6e1405ec3eb84444c4aef025806`（`c9e4033d3`，2026-09-23 19:05 +0800）；**基线..终点 = 24 个提交**（`git rev-list --count` 实查；`git log --oneline` 逐条列举见 §5.2，表内哈希数=24，与计数一致）；本报告提交为第 25 个。
- **Issue 分支**：`codex/issue-72-lago-*` 仅 **1 个**——`codex/issue-72-lago-74` @ `ec14bbf94`（`git branch --list` 实查）。
- **远端**：`codex/issue-72-lago` 与 `codex/issue-72-lago-74` 均**未推送**（`git ls-remote --heads origin` 仅历史 `lago-73-community-env`/`lago-integration`；`git branch -vv` 无 upstream）。
- **merge 拓扑（§2.1 已述，此处存目）**：`175de8b4f` parents=`552d98d12`+`ec14bbf94`；ADR 提交 `552d98d12` 仅在集成分支线（非 `ec14bbf94` 祖先）；`git diff ec14bbf94 175de8b4f` 仅 ADR-0012 +17 行。

### 5.2 关键提交（基线..终点全部 24 个，按主题分组；哈希数=24=§5.1 计数）

| 主题 | 提交（个数） |
|---|---|
| 清单+DAG（两版） | `7cf755474`、`dafba8851`（2） |
| 早版最终报告 | `6649e06ba`（1；被本报告覆盖取代） |
| #74 计划 | `6f941c48d`、`42696a30f`（2；plan + ledger 创建，含第 1 轮审查修订） |
| 实验工具契约修复 | `52e22b366`、`4aa74ce34`（2；对齐 Lago v1.53.0 + Stripe TEST 现行契约；计划原声明「不改动」，主 Agent 后裁决接受，见 §9.1） |
| ADR-0012 修订 | `552d98d12`（1；75-a2 裁决 B：充值批次协调层承载，+17 行；出处存疑见 §9.3） |
| #74 运行时证据 | `9ff29b7ec`（run11 `5d06a277` 九阶段全 pass）、`812bb241d`（晋升 8 JSON+DECISION）、`5ccdc7f3a`（ledger final，密钥双扫描 clean）（3） |
| 判据钉死 | `0aa976c64`（1；4 个 fail 侧回归测试，60→64 passed） |
| F1 审查与裁决 | `ba562d8e6`、`6bd1ef6fb`（审查 1/2 轮处置）、`ec14bbf94`（主 Agent ruling=accept + 计划偏差记录；Issue 分支 tip）（3） |
| 集成 | `175de8b4f`（1；merge #74 入集成分支） |
| 流程验证 | `905ba19b7`（1；`issue-72(#74): flow evidence`，§8 的验证记录） |
| OCR 治理 | `0ef4577b6`+`c7e404f3a`（轮 1 报告+修复 9 项）、`8b7e20772`+`d95a93f0d`（轮 2+修复 3 项）、`83c6e8edf`+`8e89d4bae`（轮 3+修复 4 项）、`c9e4033d3`（轮 4 报告，无后续修复提交）（7） |

## 6. 并行批次组织（共 1 批）与集成 / revert 记录

- **本轮实际并行批次：1 批（单票 #74）。** 依据：DAG §4 判定「当前可立即开工节点仅 #74」（`issue-72-dag.md:221`），其余节点均被 #74/T02 裁决/#75-a2 悬置；故仅派出 1 个实施流（Issue 分支 `codex/issue-72-lago-74`，worktree `.worktrees/issue72-n74`〔现已清理，见 §5.1〕）。无热点文件并行冲突问题（单流）。
- **集成记录**：1 次——merge 提交 `175de8b4f`（`issue-72(merge): #74`），将 Issue 分支 tip `ec14bbf94` 并入集成分支线 `552d98d12`；merge 结果=Issue 分支全部内容+ADR-0012 修订（§5.1 拓扑事实）。集成后流程验证、OCR 4 轮报告与 3 轮修复均直接在集成分支串行提交，无再分支。
- **revert 记录：无。** 基线..终点无任何 revert 提交（`git log --oneline` 实查）；且 F1 主 Agent 裁决**明确否决** revert 路径（拒绝路径 `git revert 52e22b366 4aa74ce34 0aa976c64` 被裁定不做，`issue-72-ledger-74.md:193-209`）。
- 下游波次（W2–W15）为解锁后前瞻规划，本轮零执行（`issue-72-dag.md:223-242`）。

## 7. 实际运行的测试与结果（分会话，均出自归档记录；标 ★ 为本报告会话重跑）

| 会话 | 测试/检查 | 结果 | 出处 |
|---|---|---|---|
| #74 实施（run1–run11） | `python3 -m pytest test_lab.py test_phases.py -q`（多次） | 基线 60 passed；`0aa976c64` 后 **64 passed** | `issue-72-ledger-74.md:125-137` |
| #74 实施 run11 | `run_lab.py`（真实栈，Stripe TEST） | 九阶段全 pass，exit 0，secrets scan 0 hits；**25 条 GREEN AC 断言** ALL PASS；DB-WATCH PASS（500 采样）；cmp×8 byte-identical | `issue-72-ledger-74.md:128-131` |
| 流程验证（905ba19b7） | 离线回归 + `run_lab.py` + `verify_ac_assertions.py` + `verify_db_watch.py` | 64 passed；run `3dc51207` 九阶段 pass、**21/21 断言**、DB-WATCH PASS（272 采样）；sk_test 形状扫描 0 命中 | `issue-72-flow-evidence-74/VERIFY-SUMMARY.txt` |
| OCR-1 修复（c7e404f3a） | 离线回归 + 真实重放（3 轮，含 1 轮外部传输故障 blocked-env） | **74 passed**；重放 **21/21** + DB-WATCH PASS（516 行 TSV/319 mid-run） | `issue-72-ocr-fix-log.md:147-188` |
| OCR-2 修复（d95a93f0d） | 同上 | **78 passed**；重放 **23/23** + DB-WATCH PASS（197 点） | `issue-72-ocr-fix-log.md:283-315` |
| OCR-3 修复（8e89d4bae） | 同上 | **82 passed**；重放 **25/25** + DB-WATCH PASS（326 点） | `issue-72-ocr-fix-log.md:400-431` |
| OCR-4 | **无修复轮、无重跑**（仅报告 `c9e4033d3`） | — | §9.2 |
| ★ 本报告会话 | `cd deploy/lago-lab/payment-activation && python3 -m pytest test_lab.py test_phases.py ../../../docs/plans/issue-72-flow-evidence-74/test_verify_db_watch.py -q` | **82 passed in 59.36s**（与 ocr-3 后基线一致，HEAD 状态复核通过） | 本报告会话实跑 |

**断言计数体系说明（25→21→21→23→25 的由来）**：存在**两套断言仪器**。① 实施会话的 GREEN 逐 AC 断言=25 条（构成：env 7+AC1 5+AC4 3+AC2 6+AC3 2+decline 2，`issue-72-ledger-74.md:129`）——一次性人工核查清单，早于流程验证员脚本存在，与后者无演进关系；② 流程验证员脚本 `verify_ac_assertions.py` 初版=21 条（构成：env 6+AC1 4+AC4 1+AC2 6+AC3 2+负对照 2，`VERIFY-SUMMARY.txt:43-59`），其后随 OCR 修复**逐轮加严**：ocr-2 +2 条（duplicates 延迟复查 clean、gate 重试探针 not_applicable）→23（`ocr-fix-log.md:307-309`）、ocr-3 +2 条（duplicates invoice count==baseline、AC3 gate 终态记录）→25（`ocr-fix-log.md:424-426`）。两个「25」构成不同（如 env 7 vs 6+AC4 1），不可互替；重放序列 21→23→25 是同一脚本加严后的单调演进，run11 的 25 是另一仪器，故整体呈非单调——非异常。

**#87 范围 PG 并发测试（2 FAIL 转述）的本轮状态**：清单:165 转述「`budget_pg_test.go` 缺 000161 owner 列，TestBudgetPGConcurrentReservation 等 2 测试 FAIL」；早版报告曾抽查该文件 `:50-59` 发现第 56 行实为 `t.Fatal(err)`、与清单所称「:56 缺 owner 列」**对不上**——即**清单对该缺陷的行号/描述引用与文件实际内容不符**（早版报告 §7 记录在案）；行号不符只能否定「引用位置」，不能证明或否定「2 FAIL」本身。**本轮未复跑**：该测试文件无 DSN 即 `t.Fatal("SAAS_TEST_PG_DSN is required…")`（`budget_pg_test.go:22-27`，本报告会话已核），本报告会话环境无 `SAAS_TEST_PG_DSN`（unset，实查），且报告员不新建数据库容器——**实际失败状态悬空（unknown）**，须由具备 PG twin 的会话实跑确认（§11）。

## 8. 真实流程验证汇总（已完成 Issue：#74）

- **环境**：隔离实验栈 `weknora-lago-74`（Compose project，回环端口 48891/48892，5 镜像 v1.53.0 digest 锁定，与 #73 主栈完全隔离）；Stripe TEST key 经 `source ~/.zcode/issue72-stripe.env` 注入环境变量（值从不回显/入库）；验证基线=集成分支 `175de8b4f`（VERIFY-SUMMARY.txt:4）。
- **验证方式**：「16 步，证据 16 件」系**编排器 JSON 给定口径**；持久化的逐步记录以 `VERIFY-SUMMARY.txt` 为准，其以六节组织（一、环境准备 5 步〔离线回归/清卷/init/up/status〕；二、真实运行〔run_lab+并行 DB 观察者〕；三、逐 AC 断言；四、DB 投影核验；五、前端 3 截图；六、环境回收与纪律）——本节下述 ①–⑧ 是对六节的**归并压缩，非与 16 步一一对应**；「16 件」未在持久化材料中逐件点名，无法对账，验收以目录实际 23 个 git 跟踪文件（12 run JSON+t02-run.txt+3 截图+TSV/输出 4 件+校验脚本 2+VERIFY-SUMMARY）为准。核心链路：① 离线回归 64 passed；② 清陈旧卷→`lab.sh init/up`（29.1s 全 healthy，`ready v1.53.0`）；③ `run_lab.py` 真实运行（9 阶段：setup/provider_setup/gate[AC1]/activate[AC2]/manual[AC4]/duplicates[AC2]/retries[AC3]/decline_control[负对照]/cleanup，run `3dc51207`，exit 0）——**串真实 Lago REST+GraphQL 与真实 Stripe TEST 扣款**（provider_payment_id=真实 PaymentIntent，1333 cents 整数分精确比对）；④ 并行 DB 观察者每 2s 只读 psql 采样 Lago postgres 投影（272 点 TSV）；⑤ `verify_ac_assertions.py` 21/21 PASS；⑥ `verify_db_watch.py` DB-WATCH PASS（succeeded 峰值恰 1、sub-b 4→1 不回退、sub-c 从未 active）；⑦ 前端 3 张截图（front-01/02/03，Playwright 无头渲染登录/incomplete/三态同屏，**仅人工观察不作判据**）；⑧ `lab.sh down`+**兄弟栈复核**（=对比 lab 启动前后非 `-74` 项目容器集合与健康状态与 Task 1 基线一致，防误伤 #73 主栈与开发容器，`issue-72-ledger-74.md:142`）+密钥形状扫描 0 命中。
- **结果与证据路径**：`docs/plans/issue-72-flow-evidence-74/`（VERIFY-SUMMARY.txt 总纲；t02-*.json 12 份；verify-{ac-assertions,db-watch}-output.txt；verify-db-watch-samples.tsv；front-0{1,2,3}-*.png）。结论：**AC1–AC4 + 负对照 + DB 投影全部通过**（VERIFY-SUMMARY.txt:92；置信度限定见 §9.2 末段）。
- **OCR 重放（同链路再验证 3 次）**：`issue-72-ocr1-replay/`（21/21）、`issue-72-ocr2-replay/`（23/23，duplicates 延迟复查落地）、`issue-72-ocr3-replay/`（25/25，baseline 发票计数锚点落地）——每轮修复后真实环境全绿，判定逻辑逐轮收紧而非放松。

## 9. Superpowers 任务审查、OCR 结论与修复轮次

### 9.1 #74 任务审查（3 轮 + 主 Agent 裁决）

审查对象为计划偏差 **F1（medium，spec-compliance）**：实施者修改了计划声明「明确不改动」的 6 个 lab 文件。3 轮审查一致结论：根因成立（计划前提「60 离线绿⇒契约正确」被 2026-09-20 首跑 blocked-env 证伪——六处线上契约从未真实执行、离线 fake 按同一组错误假设建模）；处置到位（`0aa976c64` 4 个 fail 侧回归钉死判据，64 passed）。**主 Agent 裁决：接受，维持现状，不执行 revert**（依据三条+错误代价，`issue-72-ledger-74.md:186-212`）；计划文件同步追加偏差记录（`issue-72-plan-74.md` 末节）。

### 9.2 OCR 4 轮、修复轮次与轮 4 逐项处置

| 轮 | 报告 | 原始 findings | 有效 findings | 修复提交 | 修复后测试/重放 |
|---|---|---|---|---|---|
| 1 | `0ef4577b6` | 10 | **9**（1 high+4 medium+4 low） | `c7e404f3a` | 74 passed；重放 21/21 |
| 2 | `8b7e20772` | 7（4 medium+3 low） | **3**（全 medium） | `d95a93f0d` | 78 passed；重放 23/23 |
| 3 | `83c6e8edf` | 5（1 medium+4 low） | **4**（2 medium+2 low） | `8e89d4bae` | 82 passed；重放 25/25 |
| 4 | `c9e4033d3` | 13（6 medium+7 low） | **编排器结论：4 项有效 critical/high/medium 类别未解决 + 1 项 low**（见下定论） | **无修复轮** | 无重跑 |

**严重级别定论（unclear#1 的回答）**：轮 4 原文**不存在 critical/high 级条目**（13 项 severity tally：6 medium+7 low，本报告会话 `grep -oE '· (critical|high|medium|low)\]' | sort | uniq -c` 实查）。故编排器「4 项有效 critical/high/medium」是**类别标签**（≥medium 的统称），其按级别落位**只能全部是 medium 级**——不存在 critical/high 级未解决项。该「4 项有效」判定本身未持久化 triage 记录，为编排器给定结论。

**轮 4 全部 13 项逐项处置表（本报告会话逐一对照 `issue-72-ocr-round-4.md` 原文与实查核验；「判定」栏中「编排器口径内」=按上定论推断属于 4 项有效 medium，标 ▲ 者为本报告推断、无持久化依据）**：

| 编号 | 位置（round-4 文件行） | severity | 摘要 | 本报告会话核验 | 判定 |
|---|---|---|---|---|---|
| 4-01 | clients.py:400-404（:3-4） | low（maintainability） | `last_error` 死存储 | 未核（纯代码风格） | low 候选 |
| 4-02 | evidence/t02-duplicates.json（:15-16） | medium（documentation） | 权威快照缺 `baseline.invoice_count` 与 `deferred_recheck` 键 | **已独立核验成立**：`grep -c deferred_recheck` deploy 副本=0 vs ocr3-replay=1；json diff 见 baseline.invoice_count 仅 replay 侧有；docs/migrations 晋升副本与 deploy byte-identical（cmp）→漂移同样存在于权威目录 | ▲有效 medium（编排器口径内） |
| 4-03 | evidence/t02-retries.json（:24-25） | medium（documentation） | 快照为旧路径输出（404/not_found、缺 subscription_status_after） | **已独立核验成立**：`grep -c 'subscription_status_after\|not_applicable'` deploy=0 vs replay=3；deploy 副本 `contract_notes: []` 而 replay 有 skip 说明 | ▲有效 medium（编排器口径内） |
| 4-04 | phases.py:1297-1300（:36-37） | medium（bug） | not-settled 终复查只探 incomplete，canceled(payment_failed) 竞态终局误判 FAIL | **已核代码形状**：`sed -n 1296,1298p` 确认复查仅 `_subscription_show(ctx, ext, "incomplete")` | ▲有效 medium（编排器口径内） |
| 4-05 | phases.py:1215-1219（:63-64） | medium（test） | retries 的重复注册探针缺 duplicates 式延迟复查（漏报方向） | **已核代码形状**：`grep -n deferred_recheck phases.py` 全文件仅 1 处（:1073，phase_duplicates 内），phase_retries 无 | ▲有效 medium（编排器口径内） |
| 4-06 | phases.py:254-258（:96-97） | low（maintainability） | 传输错误双重 note | 未核 | low 候选 |
| 4-07 | ocr1-replay/verify_db_watch.py:64-73（:110-111） | low（bug） | max_succeeded 无 run 隔离 | 未核 | low 候选 |
| 4-08 | ocr1-replay/verify_db_watch.py:26-27（:131-132） | low（documentation） | 「(ocr-2)」注释残留 | 未核 | low 候选 |
| 4-09 | ocr1-replay/verify_db_watch.py:111（:142-143） | low（maintainability） | samples 计数口径 | 未核 | low 候选 |
| 4-10 | ocr2-replay/verify_ac_assertions.py:10-11（:155-156） | low（documentation） | 「unmodified」声明失实 | 未核 | low 候选 |
| 4-11 | ocr2-replay/verify_db_watch.py:32-35（:169-170） | medium（other） | 归档 TSV「未在变更清单/疑似未提交」 | **已证伪**：`git ls-files docs/plans/issue-72-ocr2-replay/` 16 文件全跟踪（含 verify-db-watch-samples.tsv，本目录 16 tracked=16 on disk）——「全新检出不可复现」前提不成立（finding 自留「如确认已在先前提交中跟踪」出口） | 无效（本报告会话证伪） |
| 4-12 | ocr3-replay/verify_db_watch.py:64-67（:185-186） | medium（bug） | max_succeeded 与 rows_for 隔离不一致 | 未核（与 4-07 同型） | ▲疑似有效 medium——编排器「4 项」若不含此项，则 4 项=4-02/03/04/05；若含则另有 1 项 medium 被编排器判无效。**无法从持久化记录裁定，如实悬置** |
| 4-13 | ocr3-replay/verify_ac_assertions.py:69-71（:203-204） | low（test） | 直接下标访问的健壮性 | 未核 | low 候选 |

**对账结论**：6 项 medium 中，4-11 已被本报告会话证伪；剩 5 项（4-02/03/04/05/12）。编排器「4 项有效」与之**无法一一对账**（差 1 项），最可能映射=4-02/03/04/05（均直接作用于权威证据或核心判定器），但该映射为本报告推断（▲），无持久化 triage。**未解决的 1 项 low 亦未指认**（7 项 low 中具体哪项，无记录）。**未运行第 5 轮 OCR。**

**遗留项对「#74 已完成」结论的影响（置信度论证，rev2 重写）**：4-04 属**误报方向**（可能把合法终局判 FAIL——不推翻既有 pass 的方向，但削弱可复现性）；4-05 属**漏报方向**（exactly-once 违例若发生在 retries 探针后、cleanup 前的窗口，即时探测不可见）。既有结论的独立兜底=DB 投影观察者**不经 phases.py 判定器**、直读 psql（succeeded 峰值恰 1、sub-b 不回退），四次真实运行（run11+三次重放）DB-WATCH 均 PASS——4-05 所述违例若真实发生大概率在 DB 采样中表现为 succeeded>1 或状态回退；**但**观察者采样间隔 2s、且其自身 max_succeeded 存在未修复的隔离缺陷（4-07/4-12），兜底不完备；且 HEAD 状态未重跑真实栈。**因此本报告将 #74 维持编排器给定的「已完成」，但置信度定为「中」**：在 4-05 修复并重放前，exactly-once 结论依赖 DB 独立通道的部分兜底，不构成无风险结论；§12 的「运行时证据完成」均应在此限定下读取。

### 9.3 ADR-0012「75-a2 裁决 B」的出处核查（与编排器 JSON 冲突的效力问题）

编排器 JSON 对 #85 称「#75-a2 设计裁决…未提供」；但集成分支存在 `552d98d12`（2026-09-23 12:54，#74 实施窗口内）——ADR-0012 新增修订记录节，文本自称「**裁决（spec/ADR owner 2026-09-23）：采用协调层承载并发，不设产品级充值并发上限**」。**出处核查结果（本报告会话实查）**：① 提交作者=`wuyj <wuyj@yinhai.com>`（`git show --format` 实查，与本仓库全部提交同一作者）；② 过程文档零记录——`grep '552d98d12|75-a2'` 于 `issue-72-ledger-74.md`/`issue-72-plan-74.md`/`issue-72-ocr-fix-log.md` 均 **0 命中**（ledger 的上报事项与 Ruling 1–8 均未提及该裁决或其产生流程）；③ DAG/清单/DECISION.md 亦无引用。**结论：该裁决是「用户明示确认后写入」还是「实施期由某人随提交写入」无法从仓库证据判定**——这正是它与编排器 JSON 冲突时效力存疑的根源。本报告不替用户裁决：若该裁决经确认有效且覆盖「12 个月到期归属」全量设计，则 #85 阻塞理由应更新为仅付款链未通；否则维持 JSON 口径。列为 §10.2-D2。

## 10. 全部 Ruling 与延期事项

### 10.1 Ruling（均可溯源）

| # | Ruling | 出处 |
|---|---|---|
| R1 | done 票不重复实施，标「已验证跳过」 | inventory:6 |
| R2 | 父票 #72 只提供目标与验收，不入 DAG；层级=归属、依赖边=交付顺序 | dag:4 |
| R3 | 只采信显式/高置信边共 74 条；medium 推断边不直连（经传递覆盖） | dag:4-5,125-126 |
| R4 | #30（移动 AI Office）与 Lago 无代码耦合，不入图 | inventory:316 |
| R5 | #74 计划内 rulings 1–8：健康快照 byte-identical 不重复提交；修 lab 契约（越权已上报）；负对照改 3DS 卡；六处判据对齐 v1.53.0 真实契约；_form 传输重试+Idempotency-Key；canary 改环境变量读取；db_watch.sh 两缺陷修复；manual 403 探测目标改已结算发票 | ledger-74:81-119 |
| R6 | **F1 主 Agent 裁决：接受计划偏差，不 revert**（#74 证据生效、#74 可关票、#81/#82 解锁） | ledger-74:186-212 |
| R7 | OCR-1/2/3 逐条 findings Ruling（16 项，含 OCR3-05 low→medium 升级、OCR1-06 选择「修好+测试锁死」而非删除等） | ocr-fix-log 全文 |
| R8 | ADR-0012 写入「75-a2 裁决 B（协调层承载）」——**出处无法溯源（§9.3），效力待用户确认，暂不作为既成 Ruling 采信** | `552d98d12` diff + §9.3 核查 |

### 10.2 延期事项（等待用户/主 Agent 输入）

| # | 事项 | 状态与建议 |
|---|---|---|
| D1 | **T02 三选项裁决**：(a) Premium manual（lab 推荐，源码级验证）/(b) 受支持 Provider 真实轨道（本轮已运行时证实，但 Stripe 白名单不含微信/支付宝原生流）/(c) 改 spec | 未提供；DECISION.md §5 三选项表+推荐已就绪（`docs/migrations/lago/t02-payment-activation/DECISION.md:129-148`）。**这是 #81/#82 主链唯一剩余卡点**（#74 运行时证据已到位） |
| D2 | **#75-a2 充值批次设计**：≤5 并发批次 vs 协调层承载 | ADR-0012 修订记录（`552d98d12`）自称 spec/ADR owner 2026-09-23 已裁决方案 B，但**出处不可溯源（§9.3）**且 JSON #85 仍称未提供。需用户确认：① 该裁决是否真实经用户/spec owner 作出；② 是否覆盖「12 个月到期归属」全量设计。两者皆是→#85 阻塞理由更新为仅付款链未通 |
| D3 | **AGPL 生产批准**（#101/#103 范围） | 法务结论缺失，`deploy/lago/README.md` open production gate，生产部署法律前置门 |
| D4 | #74 关票决策等三项上报 | **本报告建议：关票**。理由：AC1–AC4 运行时证据+独立流程验证+主 Agent Ruling 已接受 F1；OCR-4 遗留属证据工装质量债，不否决票面 AC（AC 以真实运行为判据且已过，置信度限定见 §9.2）。**附带条件**：关票评论中显式引用 §9.2 处置表与 §10.2-D5 清偿安排，避免下游误以为零缺陷。另两项：`clients.worktree-variant.py` 处置——该文件是 `5e0c958e6` 引入的被跟踪历史旁支（与 `clients.py` 4 处差异：多 `import stat`、缺「拒绝覆盖既有 lab.env」守卫、多 `Stripe-Account: ""` 头、DELETE params selector；runner 实际导入 `clients.py`，背景见 `issue-72-plan-74.md:32`），**本报告建议删除或移入 docs 归档**以防误导；复跑凭据——本机依赖 `~/.zcode/issue72-stripe.env`（仅测试凭据不入库），CI/他人自备 TEST key |
| D5 | **推送/合并决策及其前置清偿**：集成分支未推送 | 顺序建议：**先清偿 OCR-4 遗留，再推送/合并**。清偿安排（本报告建议，责任主体须由主 Agent 指派——报告员无权改非报告文件）：① 4-02/4-03=用当前 phases.py 重跑 run_lab 并刷新 deploy evidence+docs/migrations 快照（cmp 晋升一致）；② 4-04=终复查改探 incomplete+canceled 双态、canceled(payment_failed) 按 settled 分支契约评估+fail 侧回归测试；③ 4-05=retries 镜像 duplicates 延迟复查+回归测试；④ 4-12=max_succeeded 补 run 隔离、runs/ 回退路径降级 exit 2；⑤ 7 项 low 逐项修复或记录豁免（含未指认的 1 项，处置时一并点名）。**判绿门槛**：离线回归（82+N passed）+真实重放（新证据目录）+第 5 轮 OCR 复审 0 项有效 ≥medium 遗留。推送后可 PR 至 main（本轮 25 提交含代码，合并前按上门槛清偿） |

## 11. 未完成节点、遗留风险、low finding、需要用户决策的内容

- **未完成节点（25/33）**：直接阻塞 9 票 + 传播 16 票（§2.3/§2.4）。除裁决输入外当前无可立即开工节点。
- **遗留风险**：① OCR-4 的 4 项有效 medium 未解决（证据快照漂移×2〔本报告会话已独立核验〕+ 判定竞态误报 + retries 漏报，§9.2）；② DAG/清单状态表停留在 #74=todo，未随完成刷新；③ #87 PG 并发测试 2 FAIL 悬空（行号引用已证实不符；本轮未复跑，需 `SAAS_TEST_PG_DSN` 环境，§7）；④ 并行热点文件预警（`lago.go` 等 5 文件，未来波次须按 DAG §4 分块，`issue-72-dag.md:242`）；⑤ `docs/plans/ledgers/` 仅覆盖 #78/79/80（前轮）+`issue-72-ledger-74`（本轮），#73/75/76/77 无账本；⑥ 早版报告记录的 `.superpowers/sdd/` 过程链断链问题未排查新载体；⑦ 实施worktree `issue72-n74` 已清理、分支未删未推（§5.1）。
- **low findings：1 项未解决**（OCR 第 4 轮结论，编排器给定；轮 4 共 7 项 low（4-01/06/07/08/09/10/13），具体指认未持久化，处置时按 D5-⑤ 一并点名）。
- **需要用户决策**：§10.2 D1–D5。
- **验收者复核命令（本报告会话实跑清单）**：`ls docs/plans/ | grep '^issue-72'`（15 项）；`git rev-list --count 29c1e56353..c9e4033d3`（=24）/ `..HEAD`（=25 含报告提交）；`git log --oneline 29c1e56353..HEAD`（24+1 提交）；`git branch --list 'codex/issue-72-lago-*'`（仅 -74）；`git ls-remote --heads origin | grep -i 'lago\|issue-72'`（未推送）；`git show --no-patch --format='%p' 175de8b4f`（552d98d12+ec14bbf94）；`git merge-base --is-ancestor 552d98d12 ec14bbf94`（false）；`git diff --stat ec14bbf94 175de8b4f`（仅 ADR +17）；`ls .worktrees/issue72-n74`（不存在）；gh 全量 34 票状态（首轮 5 票失败重试后 34/34）；OCR 各轮 severity tally grep（轮 4=6 medium+7 low，无 critical/high）；`git ls-files` 四个证据目录（23/16/16/16 跟踪，TSV 均入库→4-11 证伪）；`grep -c deferred_recheck` 与 `grep -c 'subscription_status_after\|not_applicable'`（快照漂移核验）；`cmp` docs/migrations 与 deploy 副本（byte-identical）；`sed -n 1296,1298p phases.py` + `grep -n deferred_recheck phases.py`（4-04/4-05 形状核验）；`grep '552d98d12|75-a2' ledger/plan/fix-log`（0 命中）；`grep -n Skip\|Getenv budget_pg_test.go`（无 DSN 即 Fatal；env unset）；★ 离线回归 82 passed in 59.36s；`sed -n` 核对 container.go:244 / lago.go:684-686 / spec:212-237,246 / DECISION.md §5 / waves L136-142。**未运行**：go test 与 pytest 其余套件、任何真实栈重跑、OCR 第 5 轮、`run_lab.py`（需 Stripe key）、PG twin 并发测试（需 SAAS_TEST_PG_DSN）。

## 12. 结论

本轮目标（解锁 #74 并恢复主链可裁决状态）的**实施部分达成**：#74 AC1–AC4 运行时证据完成并经独立真实流程验证（置信度「中」：判定器带未修复漏报缺陷 4-05，exactly-once 结论由 DB 独立通道部分兜底，见 §9.2），OCR 前 3 轮 16 项有效 findings 清偿；但 (a) T02 三选项裁决仍未提供，#81/#82 主链未恢复；(b) OCR 第 4 轮遗留 4 项有效 medium（无 critical/high）+ 1 项未指认 low 未解决且无修复轮；(c) ADR-0012 的 75-a2 裁决出处不可溯源，与编排器 JSON 口径冲突待确认；(d) 分支未推送、#74 未关票（本报告建议关票，附条件见 D4）。上述各项均如实上报，未粉饰。

## 13. 附录：编排器「各 Issue 实施结果 JSON」原文（33 票，任务下发方给定，本附录即其入库存档）

```json
[{"issueNumber":73,"title":"[Lago 01] 启动固定版本的 Lago Community 集成环境","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":75,"title":"[Lago 03] 证明 Wallet 批次到期、消费顺序和撤回语义","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":76,"title":"[Lago 04] 证明 Task Pricing Group 可以形成可核对计价批次","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":77,"title":"[Lago 05] 用 Lago readiness 纵向切片扩展 Commercial Platform seam","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":78,"title":"[Lago 06] 一个空间自动获得独立 Lago Customer","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":79,"title":"[Lago 07] 从 WeKnora 管理后台发布不可变 Plan Version","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":80,"title":"[Lago 08] 新空间以 Base Plan 获得权益和月度额度","status":"已验证跳过","note":"调查已验证完成，未重复实施","unresolved":[]},{"issueNumber":81,"title":"[Lago 09] 从 Quote 创建待付款 Invoice 和 incomplete Subscription","status":"阻塞","note":"Blocked by #74：T02 三选项裁决（(a) Premium manual /(b) 受支持 Provider /(c) 改 spec）仍未提供，payment-gated incomplete Subscription 创建机制无法定型（#78/#79 已满足；#74 运行时证据到位后此裁决仍是剩余卡点）","unresolved":[]},{"issueNumber":82,"title":"[Lago 10] 支付宝付款后恰好一次激活套餐","status":"阻塞","note":"Blocked by #74/#81：可信 Payment 录入通道未裁决+无可激活实体（待付款 Invoice/incomplete Subscription 未交付，lago.go:684 刻意不带 activation_rules）","unresolved":[]},{"issueNumber":85,"title":"[Lago 13] 购买 Credits 并在付款后到账","status":"阻塞","note":"Blocked by #75/#82/#83：#75-a2 设计裁决（充值批次 12 个月到期撞 Lago 六活跃钱包硬上限：≤5 并发批次 vs 协调层承载+修订 ADR-0012）未提供，充值批次数据模型无法定稿；付款链未通","unresolved":[]},{"issueNumber":87,"title":"[Lago 15] 收费调用前原子预占 Task Budget 与空间 Credits","status":"阻塞","note":"Blocked by #86（OPEN）：空间保守余额需 Lago Wallet 权威投影；#86→#87→#88 链被 #75-a2 裁决悬置（waves L140）；Lago 侧核心（价格版本投影+Wallet 权威余额预占）无法开工","unresolved":[]},{"issueNumber":88,"title":"[Lago 16] 一个 Usage Event 完成 Settlement Batch 核对","status":"阻塞","note":"Blocked by #87（OPEN 未开工）：全部 AC 以已预占调用为对象；整链传递依赖 #74 与 #75-a2 两项用户输入（waves L138-140）","unresolved":[]},{"issueNumber":94,"title":"[Lago 22] 降级、年付月发和到期回 Base Plan","status":"阻塞","note":"Blocked by #93（OPEN）：降级/到期切换与升级共用同一 ExternalSubscriptionID 订阅链切换协调器；上游链停在 #74/#75-a2 硬阻塞点（waves L138-141）","unresolved":[]},{"issueNumber":97,"title":"[Lago 25] 微信退款达到与支付宝相同的商业语义","status":"阻塞","note":"Blocked by #83/#95/#96（全 OPEN）：P03 资格核算 seam 未实现（Approve 停在 reviewing）、Credit Note 撤权模型未落地、微信 REFUND.* 通知契约与退款渠道测试为零，复用对象不存在","unresolved":[]},{"issueNumber":100,"title":"[Lago 28] Billing Center 统一展示稳定产品状态","status":"阻塞","note":"Blocked by #91/#94/#97/#98/#99（全 OPEN 且无账本）：『等待同步』须 #98 对账收敛存在才真实可展示、『预占』可靠性须 #99 故障安全，五类依赖状态机/余额分区未就位","unresolved":[]},{"issueNumber":103,"title":"[Lago 31] 部署可观测的生产 Lago 并满足计费延迟目标","status":"阻塞","note":"Blocked by #98/#99/#101（3/4 前置 OPEN）：核心 Helm 生产交付完全未开始；#101 的 AGPL 上线门槛按 spec L179-180 是生产部署法律前置门","unresolved":[]},{"issueNumber":74,"title":"[Lago 02] 证明外部付款可以激活 payment-gated Subscription","status":"已完成","note":"HEAD=175de8b4fdcef9494234d3fc127c24af3838ae0d，流程验证=真实 API 链路 + DB 投影核验 + Playwright 无头真实渲染（前端仅人工观察不作判据）。run_lab.py 串真实 Lago REST/GraphQL 与真实 Stripe TEST 扣款；DB 观察者每 2s 只读 psql 采样投影；前端用 Playwright MCP 无头浏览器登录并截图。（16 步，证据 16 件）","unresolved":[]},{"issueNumber":83,"title":"[Lago 11] 微信付款复用同一激活流程","status":"阻塞","note":"前置未完成（依赖传播）：[issue-82]","unresolved":[]},{"issueNumber":84,"title":"[Lago 12] 异常付款不会扩大权益","status":"阻塞","note":"前置未完成（依赖传播）：[issue-82, issue-83]","unresolved":[]},{"issueNumber":86,"title":"[Lago 14] 套餐额度与充值额度按到期顺序消费","status":"阻塞","note":"前置未完成（依赖传播）：[issue-85]","unresolved":[]},{"issueNumber":89,"title":"[Lago 17] 并发和委派共享同一 Task Budget","status":"阻塞","note":"前置未完成（依赖传播）：[issue-87]","unresolved":[]},{"issueNumber":90,"title":"[Lago 18] 计价延迟、未知结果和费用上界违规正确暂停 Task","status":"阻塞","note":"前置未完成（依赖传播）：[issue-88]","unresolved":[]},{"issueNumber":91,"title":"[Lago 19] BYOK 只免除模型维度 Credits","status":"阻塞","note":"前置未完成（依赖传播）：[issue-88]","unresolved":[]},{"issueNumber":92,"title":"[Lago 20] 用量修正不会改写历史","status":"阻塞","note":"前置未完成（依赖传播）：[issue-88]","unresolved":[]},{"issueNumber":93,"title":"[Lago 21] 升级立即生效并补发当月 Credits 差额","status":"阻塞","note":"前置未完成（依赖传播）：[issue-82, issue-86]","unresolved":[]},{"issueNumber":95,"title":"[Lago 23] 充值退款先锁定、再退支付宝、最后撤回 Credits","status":"阻塞","note":"前置未完成（依赖传播）：[issue-82, issue-85, issue-86]","unresolved":[]},{"issueNumber":96,"title":"[Lago 24] 套餐退款通过 Credit Note 和权益撤回完成","status":"阻塞","note":"前置未完成（依赖传播）：[issue-92, issue-93]","unresolved":[]},{"issueNumber":98,"title":"[Lago 26] Webhook 与定期对账使商业投影收敛","status":"阻塞","note":"前置未完成（依赖传播）：[issue-84, issue-85, issue-93, issue-95, issue-96]","unresolved":[]},{"issueNumber":99,"title":"[Lago 27] worker 崩溃和 Lago 故障不会丢用量或提前释放预占","status":"阻塞","note":"前置未完成（依赖传播）：[issue-90, issue-92]","unresolved":[]},{"issueNumber":101,"title":"[Lago 29] 完成计费数据最小化、凭据隔离和 AGPL 上线门槛","status":"阻塞","note":"前置未完成（依赖传播）：[issue-97, issue-99]","unresolved":[]},{"issueNumber":102,"title":"[Lago 30] 空间注销时停止收费、去标识化并保留财务历史","status":"阻塞","note":"前置未完成（依赖传播）：[issue-94, issue-96, issue-98, issue-101]","unresolved":[]},{"issueNumber":104,"title":"[Lago 32] 从备份恢复 Lago 并完成商业对账","status":"阻塞","note":"前置未完成（依赖传播）：[issue-102, issue-103]","unresolved":[]},{"issueNumber":105,"title":"[Lago 33] 切换 Lago、关闭回退窗口并移除 OpenMeter","status":"阻塞","note":"前置未完成（依赖传播）：[issue-86, issue-89, issue-91, issue-92, issue-94, issue-97, issue-100, issue-102, issue-104]","unresolved":[]}]
```
