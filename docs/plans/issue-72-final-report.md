# Issue #72 最终交付报告（Lago 计费迁移编排）

> 生成：2026-09-23（同日按独立读者评审意见修订）｜ Worktree：`.worktrees/issue72-lago` ｜ 分支：`codex/issue-72-lago`
> 基线：`29c1e56353b2b36be242018cecb43bcb3a5ef7c8`（docs: plan pass b contract and ownership freeze）｜ 实施终点：`7cf7554745`（issue-72: issues inventory and dag）
> **本报告自身以独立提交入库**：基线..HEAD 恒为 **2 个提交**（`7cf7554745` 清单+DAG ＋ 本报告提交）。报告提交经 amend 修订，哈希以 `git log -1` 实查为准，故文内不硬编码自身哈希；验收者执行 `git log --oneline 29c1e5635..HEAD` 应见且仅见上述 2 个提交。
> 本轮交付范围：33 个子 Issue 逐票调查结论 + 依赖 DAG + 本报告。**无生产代码提交**。
> 会话口径（本文多处引用）：**执行会话**＝产出清单/DAG 的本轮实施会话；**撰写会话**＝产出本报告的会话（修订版含第二轮证据补查）。两会话各自实跑的命令分列于 §12。

## 1. 执行摘要

本轮没有实施任何新的生产代码 Issue，交付物是**调查与编排制品**：33 票逐票清单（`docs/plans/issue-72-issues-inventory.md`，319 行）、依赖 DAG（`docs/plans/issue-72-dag.md`，295 行，33 节点/70 边，执行会话实跑 python3 Kahn 校验无环——脚本本体未归档，见 §7/§11）。结论：**7 票"已验证跳过"**（调查验证前轮已完成，未重复实施；GitHub 已 CLOSED，撰写会话 gh 复核仍为 CLOSED）、**26 票阻塞**（10 票直接阻塞 + 16 票依赖传播；gh 复核均为 OPEN，含父票 #72 OPEN）。主链停在两个硬阻塞点：**#74 传递阻塞 25 票**（缺 Stripe TEST 密钥 + T02 三选项裁决）、**#75-a2 传递阻塞 21 票**（充值批次到期归属未裁决）（`issue-72-dag.md:251`）。编排器给定状态枚举仅两值："已验证跳过"7 票 + "阻塞"26 票（后者 note 再分"直接阻塞原因"10 票/"前置未完成（依赖传播）"16 票，与 DAG blocked/todo 一一对应，映射表见 §2.3）。OCR 结论（编排器给定转述，仓库内无执行痕迹，见 §9）：没有任何已实施完成的 Issue，OCR 无交付范围可审；low findings 记 0 项。集成分支 `codex/issue-72-lago` **未推送远端**；基线落后 main 49 个提交但与本轮 2 个 docs 提交零路径重叠，合并风险极低（§5.3）。

## 2. 每个子 Issue 的状态与证据

状态口径两套并列：**本轮实施结果**（编排器给定 JSON）与**调查口径**（DAG 状态表 `issue-72-dag.md:257-291`：done 7 / blocked 10 / todo 16，统计 `issue-72-issues-inventory.md:46`）。

### 2.3 两套口径映射（先读）

| 编排器 JSON 状态 | 票数 | 对应 DAG 状态 | 票号 |
|---|---|---|---|
| 已验证跳过（note：调查已验证完成，未重复实施） | 7 | done | 73, 75, 76, 77, 78, 79, 80 |
| 阻塞（note 为直接阻塞原因：外部输入/前置裁决） | 10 | blocked | 74, 81, 82, 85, 87, 88, 94, 97, 100, 103 |
| 阻塞（note 标注"前置未完成（依赖传播）"） | 16 | todo（待实施，前置排队） | 83, 84, 86, 89, 90, 91, 92, 93, 95, 96, 98, 99, 101, 102, 104, 105 |

**GitHub 状态**：清单 §1 总览列（调查会话编写，其 §3 记录"gh CLI 可用（进度 7/33 实测来源）"，`issue-72-issues-inventory.md:319`——即调查会话曾以 gh 实测部分票，但未附命令输出与时间点）。**撰写会话已用 gh 全量复核**（2026-09-23，命令见 §12）：#72 OPEN；#73/75/76/77/78/79/80 **CLOSED**（7 票，已二遍点名复核）；#74、#81–#105 **OPEN**（26 票）——与清单列完全一致，无状态漂移。此后状态可能随时间漂移，验收时建议重跑（§12.3）。

### 2.1 已验证跳过（7 票，GitHub CLOSED，前轮完成、本轮未重复实施）

| Issue | 标题 | 前轮验证证据（清单 §2 逐票记录） |
|---|---|---|
| #73 | [Lago 01] 固定版本集成环境 | 四 AC 有代码+46 测试实跑+62 subtests；`deploy/lago/evidence/t01-images.txt`（5/5 digest MATCH）、`t01-health.json`（ready）、`t01-contract.json`；carryover 修复 b57be1602（`issue-72-issues-inventory.md:53-56`） |
| #75 | [Lago 03] Wallet 批次语义 | e1-e4 真实栈实测+104 测试实跑；`docs/migrations/lago/t03-wallet-semantics/verdict.md` 判 a1/b/c/d PASS-WITH-COORDINATION、**a2 BLOCKED**（第 7 活跃钱包 422，Lago 硬上限 6）（`:69-72`） |
| #76 | [Lago 04] Pricing Group 计价批次 | 60 测试实跑+6 份真实栈 JSON 证据（64/64 all_reconciled_exact、422 幂等、p95 6.893s）；4 处契约修正反哺 `lago.go:360-399`（`:77-80`） |
| #77 | [Lago 05] Commercial Platform seam | 冻结 seam `platform.go:221-230`+双适配器契约；`go build ./...` exit 0、`go test ./...` 126 包 ok；`t05-health.json`/`t05-run.txt`（`:85-88`） |
| #78 | [Lago 06] 空间独立 Lago Customer | 三层幂等+租户隔离，6 组 go test 实跑 PASS；`t06-account.txt`/`t06-status.json`；账本 `docs/plans/ledgers/lago-78.md`（`:93-96`） |
| #79 | [Lago 07] 不可变 Plan Version | 六轴校验+双方言 DB trigger 不可变，5 包测试实跑 PASS；`t07-run.txt`；账本 `docs/plans/ledgers/lago-79.md`（`:101-104`） |
| #80 | [Lago 08] Base Plan 权益与月度额度 | 懒链+配额 guard+迁移 000183，9 个点名测试实跑 PASS；`t08-run.txt`；账本 `docs/plans/ledgers/lago-80.md`（`:109-112`） |

> **证据性质声明（评审意见采纳）**：上表测试数字（46/104/60/126 包/6 组/5 包/9 个）均系清单 §2 的文字转述，为前轮实施会话实跑记录；`deploy/lago/evidence/t0*-run.txt` 等归档的是**真实栈操作时间线**（如 `t08-run.txt` 开头即"real-stack run — operator timeline"），**不是** pytest/go test 的 stdout。仓库内无这些测试的第一手运行输出。撰写会话未重跑任何测试套件（§12），验收者如需第一手证据须按 §12.3 命令重跑。

### 2.2 阻塞（26 票，GitHub 全 OPEN[撰写会话 gh 复核]，本轮零实施）

**直接阻塞（10 票）**——依据：编排器给定 JSON + `issue-72-dag.md:236-249` 阻塞原因表 + 清单 §2 逐票"处理决定"：

| Issue | 标题 | 阻塞原因（JSON note 摘要） | 依据 |
|---|---|---|---|
| #74 | [Lago 02] 外部付款激活 | AC1-AC3 运行时证据需 Stripe TEST 密钥（sk_test_…）重跑 `run_lab.py`（解锁命令已内联至 §10 D1）；T02 DECISION.md 三选项待 spec/ADR owner 裁决；传递阻塞其余 25 票 | `issue-72-issues-inventory.md:58-64`、`docs/migrations/lago/t02-payment-activation/DECISION.md` |
| #81 | [Lago 09] Quote→待付款 Invoice | Blocked by #74：付款激活路径未裁决，incomplete Subscription 创建机制无法定型；#78/#79 前置已满足 | `:114-120` |
| #82 | [Lago 10] 支付宝恰好一次激活 | Blocked by #74/#81：录入通道未裁决 + 无可激活实体（`lago.go:684-686` createSubscription 刻意不带 activation_rules，撰写会话已核原文注释） | `:122-128` |
| #85 | [Lago 13] 充值到账 | Blocked by #82/#83（传递 #74）且 #75-a2 设计决策未决（≤5 并发批次 vs 协调层承载+修订 ADR-0012） | `:146-152` |
| #87 | [Lago 15] 原子预占 | Blocked by #86：空间保守余额需 Lago Wallet 权威投影；#86→#87→#88 链被 #74/#75-a2 悬置 | `:162-168` |
| #88 | [Lago 16] 批次核对 | Blocked by #87（OPEN 未开工）：全部 AC 以已预占调用为对象 | `:170-176` |
| #94 | [Lago 22] 降级/年付/到期 | Blocked by #93：与升级共用同一 ExternalSubscriptionID 切换协调器 | `:218-224` |
| #97 | [Lago 25] 微信退款对齐 | Blocked by #83/#95/#96：P03 资格 seam 未实现、Credit Note 撤权模型未落地、REFUND.* 契约未实现 | `:242-248` |
| #100 | [Lago 28] Billing Center | Blocked by #91/#94/#97/#98/#99（全 OPEN 无账本） | `:266-272` |
| #103 | [Lago 31] 生产 Helm+延迟 | Blocked by #98/#99/#101：AGPL 上线门槛为法律前置门；Helm 交付未开始 | `:290-296` |

**依赖传播阻塞（16 票）**——JSON note「前置未完成（依赖传播）」，DAG 口径 todo：

| Issue | 标题 | 传播链（JSON） |
|---|---|---|
| #83 | [Lago 11] 微信复用激活 | [issue-82] |
| #84 | [Lago 12] 异常付款不扩大权益 | [issue-82, issue-83] |
| #86 | [Lago 14] 到期顺序消费 | [issue-85]（月度维度可先行，`:160`） |
| #89 | [Lago 17] 并发共享预算 | [issue-87] |
| #90 | [Lago 18] 延迟/超界暂停 | [issue-88] |
| #91 | [Lago 19] BYOK 豁免 | [issue-88] |
| #92 | [Lago 20] 用量修正不改历史 | [issue-88] |
| #93 | [Lago 21] 升级补差 | [issue-82, issue-86] |
| #95 | [Lago 23] 充值退款三段式 | [issue-82, issue-85, issue-86] |
| #96 | [Lago 24] 套餐退款 Credit Note | [issue-92, issue-93] |
| #98 | [Lago 26] Webhook 对账收敛 | [issue-84, issue-85, issue-93, issue-95, issue-96] |
| #99 | [Lago 27] 故障不丢用量 | [issue-90, issue-92] |
| #101 | [Lago 29] 最小化/AGPL | [issue-97, issue-99] |
| #102 | [Lago 30] 空间注销 | [issue-94, issue-96, issue-98, issue-101] |
| #104 | [Lago 32] 备份恢复对账 | [issue-102, issue-103] |
| #105 | [Lago 33] 切换/移除 OpenMeter | [issue-86, #89, #91, #92, #94, #97, #100, #102, #104]（9 个 blocker 全 OPEN，收官票） |

## 3. #72 总体验收标准覆盖情况

父 Issue #72 只提供总体目标与验收，不计入 DAG 节点（`issue-72-issues-inventory.md:5`）。其验收由 Spec `docs/specs/2026-09-20-lago-billing-migration-design.md` 承载：**行为与契约矩阵第 1–24 项位于 spec:212-237**（`### Behavior and contract matrix`，撰写会话已核原文；第 1 项 Community 能力+AGPL、第 5 项外部付款激活、第 8 项 Credits 顺序、第 24 项 OpenMeter 移除等）与 **Completion gate（spec:246-248）**：全部矩阵对 pinned Lago Community 通过 + 全部检查（单元/集成/并发/契约/端到端/静态/性能/安全/恢复）通过 + AGPL 审查接受 + 无未解决阻塞的独立 Spec Compliance 与代码审查 + 回退窗口关闭 + OpenMeter 运行时路径移除；且明示 mock/OpenAPI/事件接收响应/健康进程不构成完成证据。

覆盖情况（依据清单 §2 逐票证据；撰写会话已抽查核验下列关键代码引用原文）：

- **已覆盖（基座）**：33 票中 7 票（#73/#75/#76/#77/#78/#79/#80）按票级 AC 验证并 GitHub CLOSED（撰写会话 gh 复核确认），构成 pinned v1.53.0 环境与冻结 Commercial Platform seam 基座。对应矩阵项的部分覆盖：第 1 项的 Community 能力子集（AGPL 审查除外）、第 2 项租户隔离（#78）、第 3 项不可变目录（#79）、第 8 项的语义验证（#75 verdict）、第 10 项的批次核对形态（#76）。
- **未覆盖（主体）**：26/33 票未完成。Completion gate 关键项全部未达成——AGPL 审查结论不存在（`deploy/lago/README.md:186-192` 明确 open production gate，`issue-72-issues-inventory.md:277`）；OpenMeter 运行时仍装配（`internal/container/container.go:244`，**撰写会话已核原文**：`must(container.Provide(ommeter.NewGatewayFromEnv, dig.As(new(domain.CommercialGateway))))`；另 `:309` 为清单行号非代码行号）；第 5/6/7/9/11/12–24 项对应票全 OPEN；24 项 gate 无证据归档（DAG W15 未执行）；回退窗口关闭无流程（#105 未实施）。
- **判定**：#72 迁移整体**未完成**；进度 = 调查 33/33 完成、实施 7/33（均为前轮成果）、本轮新增实施 0。#72 GitHub OPEN（撰写会话 gh 复核）。

## 4. 制品与证据路径索引

| 类别 | 路径 | 说明 |
|---|---|---|
| 层级树 | `docs/plans/issue-72-issues-inventory.md:5` | 33 票均为 #72 直接子票，层级树退化为单层（无嵌套父子） |
| 子 Issue 清单 | `docs/plans/issue-72-issues-inventory.md` | 逐票状态/依据/依赖/处理决定（§1 总览 + §2 逐票 + §3 外部依赖核查） |
| DAG | `docs/plans/issue-72-dag.md` | Mermaid 图 + 70 条边依赖表 + 拓扑序 + 波次规划 + 阻塞节点 + 状态表 + 校验记录 |
| 编排计划（前轮） | `docs/plans/2026-09-20-lago-billing-waves.md` | Wave 2/3/4 完成记录、硬阻塞点、主/Worker 职责（L118-160） |
| 票级计划（前轮） | `docs/plans/2026-09-21-lago-t06-lago-customer.md`、`2026-09-21-lago-t07-plan-version-publish.md`、`2026-09-21-lago-t08-base-plan.md`（由账本头部引用，`docs/plans/ledgers/lago-78.md:5` 等） | #78/#79/#80 实施计划；**撰写会话未核验这三个计划文件是否存在于本 worktree**（账本转述引用） |
| Ledger | `docs/plans/ledgers/lago-78.md`、`lago-79.md`、`lago-80.md` | 本 worktree 现存仅这 3 份（`ls docs/plans/ledgers/` 实查）；断链分析见 §11.2 |
| Spec/ADR | `docs/specs/2026-09-20-lago-billing-migration-design.md`（矩阵 :212-237、Completion gate :246-248）；ADR-0012（批次到期，待修订）、ADR-0014（seam 冻结加法规则，经清单/账本引用） | 迁移事实源 |
| 真实栈证据（前轮） | `deploy/lago/evidence/`（t01-images/health/contract、t05-health/run、t06-account/status、t07-run、t08-run）；`docs/migrations/lago/{t02-payment-activation,t03-wallet-semantics,t04-pricing-group,t07-plan-version-publish,t08-base-plan}`；`deploy/lago-lab/{payment-activation,pricing-group,wallet-semantics}` | `ls` 实查存在；性质=真实栈运行时间线/判定文档，非测试 stdout（§2.1 声明） |
| 本轮流程证据目录 | `docs/plans/issue-72-flow-evidence-*/` | **不存在**（`find`/`ls` 实查无匹配）——本轮无已完成 Issue，故无流程验证证据目录（见 §8） |
| OCR 报告 | 无任何文件痕迹 | `find -iname '*ocr*'` 仅命中无关的 `docker/Dockerfile.docreader`/`docreader`；OCR 结论为编排器转述，性质见 §9 |

## 5. Git 事实：worktree / 分支 / 基线 / HEAD / 关键提交 / 与 main 的距离

### 5.1 基本事实（撰写会话在 worktree 实跑命令所得，清单见 §12）

- **Worktree**：`.worktrees/issue72-lago`，当前分支 `codex/issue-72-lago`（`git branch --show-current`）。
- **基线**：`29c1e56353b2b36be242018cecb43bcb3a5ef7c8`；**实施终点**：`7cf7554745`（"issue-72: issues inventory and dag"，2026-09-23 02:58 +0800，2 文件 +614 行，`git show --stat 7cf755474` 实查）；**本报告**为基线..HEAD 的第 2 个（末个）提交。
- **Issue 分支 `codex/issue-72-lago-*`**：**0 个**（`git branch --list 'codex/issue-72-lago-*'` 输出为空）。本轮未创建任何 per-issue 分支。
- **远端状态**：`codex/issue-72-lago` **未推送**——`git ls-remote --heads origin | grep lago` 仅有历史分支 `lago-73-community-env`（5c25dd376）与 `lago-integration`（22b58cbcc）；`git branch -vv` 无 upstream。历史 per-issue 分支（lago-74/78/79/80 等）本地与远端均已不存在（waves 文档 L134 记录其 rebase 并入历史）。
- **关键提交（历史，均为基线祖先，`git log --oneline --all | grep` 实查存在；`git merge-base --is-ancestor e87eb459 29c1e56353` 确认）**：
  - `b57be1602` fix(lago): probe lenient-cleans（#73 carryover 修复）
  - `d1eec16be` merge: #74 payment activation lab（squashed，secret 形状测试夹具 defuse 后并入）
  - `6ea0511f5` merge: #79 immutable Plan Version publish（W3，含 #78/#79 合并冲突 union 解决）
  - `e87eb459c` merge: #80 Base Plan benefits + monthly credits（W4，含 TOCTOU Important 当轮修复）

### 5.2 基线与 main 的距离（撰写会话实跑）

- `git rev-list --count 29c1e5635..main` = **49**（main 已前进 49 个提交）；`git rev-list --count main..HEAD` = **2**（本 worktree 仅领先上述 2 个 docs 提交）。
- 漂移体量：`git diff --stat 29c1e5635..main` 尾行 = **136 files changed, +28172, −8174**。
- 路径交集（冲突风险面）：
  - `git log --oneline 29c1e5635..main -- docs/plans/` = **0 个提交** → 本轮 2 个提交（均在 docs/plans/）与 main **零路径重叠，合并/变基冲突风险≈0**。
  - `git log 29c1e5635..main -- deploy/lago/ docs/migrations/lago/ docs/plans/ledgers/` = **0 个提交** → 前轮证据与账本在 main 上未被动过。
  - `git diff --stat 29c1e5635..main -- internal/modules/commercial/` = 仅 1 文件：`repository/commercial/benefits.go` +16 行 → 商业模块代码基本未漂移，后续实施波次变基到 main 的代码冲突面小。

### 5.3 合并评估与 worktree 保留计划（供"是否推送/合并回 main"决策）

- **合并风险评估：低**。依据 §5.2：本轮交付纯 docs 且 docs/plans 在 main 零漂移；`git merge-base` 同源，无 rebase 改写历史需求（本分支无远端，不存在强推问题）。建议路径：`codex/issue-72-lago` 推送 → PR 到 main（两提交均 docs-only，可直接快进合并）；或 cherry-pick 两个 docs 提交。
- **worktree 保留计划**：本 worktree 是 `issue-72-dag.md`/`issue-72-issues-inventory.md`/本报告的唯一载体（分支未推送前删除 worktree + 分支即丢失交付物）。**建议在推送远端之前保留**；推送并合并后，worktree 可安全删除（`git worktree remove`），后续实施波次按 DAG §4 建议从 main 新建 per-issue worktree（`.worktrees/lago-<NN>` 惯例，参照前轮）。
- **注意**：main 的 49 个提交漂移意味着后续任何实施波次都应以**当时最新 main** 为基（而非 29c1e5635），DAG §4 的文件改动范围表基于基线代码，实施前需按 §5.2 方式重查漂移（目前仅 benefits.go +16 影响）。

## 6. 并行批次组织（共 0 批）与集成 / revert 记录

- **本轮实际并行批次：0 批。** 未启动任何实施波次、未派发任何实施 Worker；依据（执行会话产物，撰写会话复核文档记录）：除 #74 外无可立即开工节点（`issue-72-dag.md:211` 波次表注），而 #74 本身 blocked-env 待外部输入；"无可开工节点"的判定链 = DAG §5 传递依赖计数（#74:25、#75:21，`issue-72-dag.md:251`）+ §4 波次模拟，计算载体为执行会话的 python3 脚本（**未归档**，见 §7、§11.2）。DAG §4 的 W1-W15 波次表（`issue-72-dag.md:213-232`）是 #74 解锁后的**前瞻规划**（含热点文件 `lago.go`/`order.go`/`commercial.ts`/`platform.go` 并行冲突预警），本轮无一执行；剩余工作量粗估与下轮启动条件见 §11.3。
- **本轮集成记录：无**（基线..HEAD 仅 2 个 docs 提交，无 merge commit）；**本轮 revert 记录：无**。
- **历史集成/revert 参考**（前轮 waves，均在基线内）：Wave 2 四票并行并入（含 #74 squash d1eec16be）；Wave 3 #78/#79 rebase 到 1b5b241 后并入 6ea0511f5；Wave 4 #80 集成 e87eb459c（`docs/plans/2026-09-20-lago-billing-waves.md:126-142`）。历史 revert：本轮证据范围内未发现任何 revert 提交记录。

## 7. 实际运行的测试与结果（按会话分列）

**执行会话**（产出清单/DAG 的会话）实跑，记录于 `issue-72-dag.md:295`：python3 校验脚本（Kahn 拓扑 + 边违例扫描 + 就绪波次模拟）——33 节点、70 条边；Kahn 完成=无环；DAG §3 拓扑序 70 条边零违例；波次模拟无停滞；传递依赖计数 #74:25、#75:21 与 waves 文档一致。**该脚本本体未归档**（撰写会话 `grep -rn -il 'kahn' --include='*.py'` 全库 0 命中、无 issue-72 相关 .py），属可复现性缺口，验收者只能选择信任文档记录或按 §12.3 重写复算。

**前轮实施会话**实跑（逐票记录于 `issue-72-issues-inventory.md` §2，撰写会话未重跑、仓库无第一手 stdout，见 §2.1 声明）：
- #73：`python3 -m pytest deploy/lago/ -q` → 46 passed + 62 subtests（:53）
- #75：wallet-semantics 2146 行预注册断言 → 104 passed（:69）
- #76：pricing-group → 60 passed；measure.py 18 单测 passed（:77、:293）
- #77：`go build ./...` exit 0；`go test ./...` 126 包 ok（:85）
- #78：service/repository/router/双适配器契约/seam 6 组 go test 全 PASS（:93）
- #79：5 个相关 Go 包测试全 PASS + `go build ./...` exit 0（:101）
- #80：9 个点名测试全 PASS（:109）
- 前轮 wave 级：Wave 2 `go test ./...` 零 FAIL + 4 个 python 套件全绿；Wave 3 120 包零 FAIL（waves L133-134）。

**已知失败**（前轮遗留，非本轮）：#87 范围 PG 并发测试 fixture 漂移——清单转述为"`budget_pg_test.go:56` 缺 000161 owner 列，TestBudgetPGConcurrentReservation 等 2 测试 FAIL"（`issue-72-issues-inventory.md:165`）。**撰写会话抽查不符**：该文件 `internal/modules/commercial/repository/commercial/budget_pg_test.go` 第 50-59 行为 GORM 连接池设置（第 56 行是 `t.Fatal(err)`），未见 owner 列/migration 字样——该引用行号未能复核，缺口本身以清单记录为准，验收者应实跑 PG 测试确认（§12.3）。

**撰写会话**：未运行任何测试套件；仅运行 §12 所列 git/ls/grep/gh/fetch 等验证命令。

## 8. 真实流程验证汇总（本轮完成 Issue：0 个）

- **本轮（基线..HEAD）没有任何状态为"已完成/已验证完成"的实施 Issue**：33 票 JSON 中枚举最高为"已验证跳过"（= 调查验证前轮已完成、本轮未实施），其余 26 票"阻塞"。因此**本轮未产生任何 `docs/plans/issue-72-flow-evidence-*` 证据目录**（worktree 内 `find`/`ls` 实查不存在），也没有本轮的浏览器操作序列或真实 API 链路验证可记录。
- 7 票"已验证跳过"的**历史**真实流程验证（来自前轮会话，记录于清单 §2；性质为后端/实验票，验证方式为真实 Lago API 链路而非浏览器 UI 操作；撰写会话仅核验证据文件存在，未重新执行）：
  - 环境：本地 pinned Lago Community v1.53.0 compose 栈（`deploy/lago/`，5 镜像 OCI digest 锁定）+ lago-lab 实验栈（`deploy/lago-lab/{payment-activation,pricing-group,wallet-semantics}`），ticket 隔离 Compose project、loopback-only 端口（`t08-run.txt` 头部记录）。
  - #73：`lago.sh` 启停 + `health.py` 分类健康 + `contract_probe.py` 唯一合成 Customer 创建/清理 → `deploy/lago/evidence/t01-*.txt/json`。
  - #75：harness.py e1-e4 真实 Wallet 批次到期/消费顺序/并发/void 实测 → `docs/migrations/lago/t03-wallet-semantics/verdict.md`（a2 判定依据 `evidence/e1-expiry.json`）。
  - #76：真实 Usage Event 投递→可查询→current usage 聚合→批次金额核对（64/64 精确、p95 6.893s）→ `docs/migrations/lago/t04-pricing-group/` 6 份 JSON。
  - #77：`GET /commercial/platform/readiness` 真实栈快照 → `t05-health.json`/`t05-run.txt`。
  - #78：空间懒 ensure_customer 真实栈链路 → `t06-account.txt`/`t06-status.json`。
  - #79：发布链路真实栈六轴校验 → `t07-run.txt`。
  - #80：Base Plan 订阅+月度额度 6 阶段真实栈 → `t08-run.txt`（operator timeline）。

## 9. Superpowers 任务审查与 OCR 结论、修复轮次

- **本轮 Superpowers 任务审查：无**。本轮交付范围只有清单+DAG+本报告，无生产代码任务，故无任务级审查记录；`docs/plans/ledgers/` 无本轮新增；`.superpowers/sdd/` 无 Lago 条目（本 worktree 与主检出均 `ls` 实查为空，远端两分支 ls-tree 亦无，见 §11.2）。
- **前轮审查记录**（waves，基线内）：#78/#79 review 均 Approved 无 Critical/Important；#80 TOCTOU Important fix 当轮完成并 re-review PASS（`docs/plans/2026-09-20-lago-billing-waves.md:134,142`）。
- **OCR 结论的性质（评审意见采纳，如实交代）**：结论"没有任何已实施完成的 Issue，OCR 无交付范围可审；low findings 0 项"由**编排器（本报告的任务下发方）给定**，属转述。仓库内无任何 OCR 执行痕迹（无报告文件、`find -iname '*ocr*'` 仅无关命中）。结合基线..HEAD 唯一代码外提交为纯 docs（无代码 diff 可审），合理读法是 **OCR 因无可审范围而未对代码执行审查，"0 项"是该情形下的记法，而非实际执行后产出的计数**——验收者**不可**将其当作"已执行且查得 0 问题"的证据。若需可查证的 OCR，应在合并前的代码提交上重新执行。
- **修复轮次：0 轮**（无交付范围即无 OCR 触发的修复轮）。前轮对照：#80 的 TOCTOU Important 为 1 轮当轮修复（waves L142）。

## 10. 全部 Ruling 与延期事项

### 10.1 Ruling（本轮调查/编排裁决，均可在制品中溯源）

| # | Ruling | 依据 |
|---|---|---|
| R1 | done 口径：已有完成证据（代码+实跑测试+运行时证据）的票不重复实施，标"已验证跳过" | `issue-72-issues-inventory.md:6` |
| R2 | 父票 #72 只提供目标与验收，不入 DAG 节点；层级树=范围归属、依赖边=交付顺序，二者不混淆 | `issue-72-dag.md:4`、inventory:5 |
| R3 | 只采信票面显式/高置信边共 70 条。四条边未采信，逐条理由：**80→94**（#94 票面未列且经 94←93←86←80 传递覆盖）；**81→85** 与 **81→93**（两条均为 medium 置信的接口推断、票面未声明，且经 82 传递覆盖）；**73→75**（实验栈复用属事实描述，非票面声明的依赖）——"少加不会错杀并行，多加假依赖会" | `issue-72-dag.md:4,120` |
| R4 | #30（移动 AI Office）与 Lago 迁移无代码耦合：仅记录核查结论，不入图、不加边 | `issue-72-issues-inventory.md:316` |
| R5 | OCR 无交付范围可审（本轮无已实施完成 Issue）——性质为未执行而非执行后 0 项，见 §9 | 编排器给定结论 |
| R6 | 历史运行时裁决（前轮 Wave 3）：create-on-external_id=UPSERT；GET /plans/{code} 可用 | `docs/plans/2026-09-20-lago-billing-waves.md:134` |

### 10.2 延期事项（等待用户输入）与倾向性建议

#### D1 — #74 解锁：Stripe TEST 密钥 + T02 三选项裁决

**解锁命令（已从 #74 Issue 唯一评论经 `gh issue view 74 --comments` 取回内联，撰写会话实跑）**：

```bash
# 评论原文指向 .worktrees/lago-74，该 worktree 已不存在（ls .worktrees/ 实查）；
# deploy/lago-lab/ 在本 worktree（及 main）均存在（漂移 0 提交），可在任一含该目录的检出执行，去掉原 cd 即可：
./deploy/lago-lab/payment-activation/lab.sh up
STRIPE_SECRET_KEY=<sk_test_...> ./deploy/lago-lab/payment-activation/run_lab.py --output-dir deploy/lago-lab/payment-activation/evidence
./deploy/lago-lab/payment-activation/lab.sh down
```

**密钥安全注入流程（结合 push-protection 教训，waves L136 曾拦截 secret 形状 canary）**：`run_lab.py` 只从调用方环境变量取键——按序读 `STRIPE_TEST_SECRET_KEY` / `STRIPE_SECRET_KEY`，**live 键（sk_live/rk_live）会被拒绝**（`deploy/lago-lab/payment-activation/run_lab.py:101-107`，撰写会话已核源码）。要求：① 密钥只经 shell 环境变量临时注入，不写入任何文件/脚本/Issue/日志；② 证据输出目录已有脱敏契约（spec 第 22 项 + inventory:277 日志脱敏基础设施），补跑后人工复核 JSON 无键形字符串再提交；③ 测试夹具继续使用拼接字面量 canary（d1eec16be 的 defuse 方式）。

**三选项对比与建议**（依据 `docs/migrations/lago/t02-payment-activation/DECISION.md` §5，撰写会话已核原文）：

| 选项 | 含义 | 代价/风险 | 证据状态 |
|---|---|---|---|
| (a) 采购 Lago Premium，渠道事实记为 manual Payment | WeKnora 保留微信/支付宝请求/回调/退款主权；每个已验证 Payment Fact 经 `POST /api/v1/payments` 登记，发票 payment_status 驱动同一 activation 规则 | Premium 许可成本+商业条款（lab 自认超出其能力范围）；manual→activation 流程仅源码级验证（Premium 超出 Community lab 范围） | 源码级验证（manual_create_service.rb Premium 门控+paid 状态传播） |
| (b) 受支持 Provider（如 Stripe）作真实扣款轨道 | Community 上 provider 执行的扣款可端到端激活 | Stripe 白名单**不含 wechat_pay/alipay**（DECISION.md §4 源码枚举：stripe/gocardless/adyen/cashfree/flutterwave/moneyhash）→ 微信/支付宝原生流无法走此轨，需产品决策渠道取舍；运行时证据同样待 TEST key | 源码级验证+运行时证据待补 |
| (c) 改 spec/ADR（放弃 Lago 作为付款激活权威，或在其上游加合规记录 seam） | 仅当 (a)(b) 均不可接受 | 改动范围最大（动事实源） | — |

**倾向性建议（仅供参考，裁决权在 spec/ADR owner 与产品方）**：采用 **(a)，并以 (b) 作局部互补**——这正是 DECISION.md 自己的推荐（"The lab recommends (a), with (b) as the partial complement"）：(a) 是唯一保持 spec 架构完整的选项（WeKnora 验证的渠道 Payment Fact 驱动 Lago、Lago 仍是唯一激活权威、无双写无本地强激活），且其依赖的激活机制已在 pinned 版本接线。决策前须补两件事：① Premium 商业条款评估（lab 明示 unproven）；② 若 CN 支付必须原生微信/支付宝，确认 (a) 覆盖全部渠道后再排除 (b)/(c)。

#### D2 — #75-a2：充值批次 12 个月到期归属

**两方案对比与建议**（依据 `docs/migrations/lago/t03-wallet-semantics/verdict.md`，撰写会话已核原文）：

事实基线（verdict L58-66）：v1.53.0 过期只存在于**钱包级**（`wallets.expiration_at`）→ 每批次一个到期 ⇒ 每批次一个钱包；`MAXIMUM_WALLETS_PER_CUSTOMER = 6` 为硬编码运行时上限（无配置覆盖写入方）；E1 实测第 7 个并发 active 钱包被 422 `wallet_limit_reached` 拒绝。verdict 总则（L21-22）：任一不变量 BLOCKED ⇒ Lago 原生对象**不能**在不引入协调层重设计的情况下承载已批准的 Credits 模型。另 L161：若同钱包合批，"同到期最早发放"不可精确表达。

| 方案 | 含义 | 代价/风险 |
|---|---|---|
| (a) 产品上限 ≤5 并发充值批次 | 每批一钱包（≤5 充值 + 套餐月度钱包 = 6，**恰好顶满硬上限、零余量**） | 产品侧永久限流；任何未来钱包需求（如赠送批次）立即撞顶；依赖 Lago 不改硬上限的版本假设 |
| (b) 批次到期权威移入 WeKnora 协调层 + 修订 ADR-0012 | Lago Wallet 只记金额，批次到期/消费顺序由协调层编码（T03 verdict 协调义务清单） | WeKnora 侧新增权威状态与对账义务；ADR-0012 需修订（自 ef2ccb24d 后未动） |

**倾向性建议**：**(b)**。理由：verdict 的判定本身就是"a2 BLOCKED ⇒ 必须引入协调层承载"（L21-22），方案 (a) 只是用产品限流绕开第 7 个钱包，并未消除"批次一钱包一到期"的结构性约束（顶满 6 钱包零余量，且合批即丢失 tie-break 精度 L161）；(b) 一次到位且与 b/c/d 各项 PASS-WITH-COORDINATION 的协调义务（L160-166）同向，可一并落进协调层。(a) 可作为 (b) 落地前的**临时产品限流**叠加使用。裁决权在用户/spec owner。

#### D3 — AGPL 生产批准（#101 范围）

`deploy/lago/README.md:186-192` 明确 open production gate；是 #103 生产部署的法律前置门。需法务给出可审计结论（Completion gate 硬性项）。

## 11. 未完成节点、遗留风险、断链与下轮启动

### 11.1 未完成节点与遗留风险

- **未完成节点（26/33）**：直接阻塞 10 票（#74/#81/#82/#85/#87/#88/#94/#97/#100/#103，DAG §5 `issue-72-dag.md:236-249`）+ 依赖传播/待实施 16 票（§2.2 表）。除 #74 外当前**无可立即开工节点**（`issue-72-dag.md:211`）。
- **遗留风险**（源清单 §2 逐票缺口记录）：
  1. #76 两处产品化缺口（422 不可区分、唯一性三元组作用域）移交 OPEN 的 #87/#88（:80）。
  2. #80 遗留（concurrent_tasks 执法、manual/passage guard、前端展示）移交 #87/#88/#100（:112）。
  3. #87 PG 并发测试 fixture 漂移致 2 测试 FAIL（清单:165 转述；撰写会话抽查行号不符，见 §7，需实跑确认）。
  4. 并行热点：`commercialplatform/lago.go`（12 票可能同改）等五文件冲突预警，未来并行须按 DAG §4 建议分文件/分块/按波次串行合入（`issue-72-dag.md:232`）。
  5. #74 测试内 secret 形状 canary 曾触发 GitHub push protection（已 defuse，waves L136）——后续补跑时须按 §10.2 D1 密钥流程操作。
  6. **low findings：0 项**——性质为"无交付范围"记法（§9），非执行后计数，不可独立查证。

### 11.2 账本与 Superpowers 过程链断链分析（评审意见采纳）

- **账本覆盖**：33 票中仅 #78/#79/#80 有账本（`docs/plans/ledgers/lago-78/79/80.md`）；#73/#75/#76/#77 四张已关票**无账本**（其完成证据以清单 §2 + evidence/verdict 文档承载）。这是前轮编排的选择，非本轮丢失。
- **`.superpowers/sdd/*` 过程记录缺失的实查结论**（撰写会话）：账本引用的路径如 `.superpowers/sdd/2026-09-21-lago-t06-lago-customer/progress.md`（`docs/plans/ledgers/lago-78.md:7`）在以下所有位置均**不存在**：① 本 worktree（`ls` 实查）；② 主检出 `/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/`（grep lago 无命中）；③ 远端 `lago-integration` 分支（`git fetch --depth=1` + `git ls-tree .superpowers/sdd/`：仅 7 个 2026-09-10~09-19 的非 lago 条目）；④ 远端 `lago-73-community-env` 分支（同法：无 lago 条目）。原 lago-78/79/80 worktree 已删除（`ls .worktrees/` 实查：现存 backend-mod-wave01/bm-t10/bm-t5/issue30-sweep/issue72-lago/passb-int/tdm-int/tdm-s2-chat/tdm-s3-kb，无 lago-*）。
- **判定**：过程级 progress.md **不可恢复**（载体已删除且从未提交）；可追溯的替代链 = 账本（已提交的汇总）+ waves 编排记录 + git 提交历史 + evidence 目录。**建议**：后续票（#74 补跑起）把 `.superpowers/sdd/<ticket>/` 一并纳入分支提交，避免再断链；如需抢救，唯一未排查载体是远端两分支的历史提交（本报告只查了分支头树，未做全历史对象扫描）。

### 11.3 解锁后剩余工作量与下轮启动条件

- **结构粗估（依据 DAG §4 波次表，撰写会话汇总）**：26 票分布在 15 个串行波次，并行宽度峰值 3：W1 #74（1）→ W2 #81（1）→ W3 #82（1）→ W4 #83（1）→ W5 #84+#85（2）→ W6 #86（1）→ W7 #87+#93+#95（3）→ W8 #88+#89+#94（3）→ W9 #90+#91+#92（3）→ W10 #96+#99（2）→ W11 #97+#98（2）→ W12 #100+#101（2）→ W13 #102+#103（2）→ W14 #104（1）→ W15 #105（1）。注意：D1（Stripe key+三选项）只解锁 #74→#83/#84 主链；**#85 起仍需 D2（75-a2）裁决**（`issue-72-dag.md:251`）。
- **节奏参考（不作承诺）**：前轮 2026-09-20~21 两天完成 Wave 2-4 共 7 票（waves L126-142）。按此节奏量级，26 票/15 波次为**多周级**工程；本报告不做工时承诺（无单票耗时基线数据）。
- **下轮启动条件与方式**：① D1 裁决+密钥到位后，先补跑 #74 AC1-AC3（§10.2 命令）并关票；② D2 裁决须在 W5（#85）前给出；③ **重启为手动**（未发现任何自动重启机制的证据），续跑基准制品 = `issue-72-dag.md` §6 状态表 + `issue-72-issues-inventory.md` §2——重启时先把 #74（及已裁决的依赖）状态改 done、重算 frontier（校验脚本未归档，需按 DAG §7 规格"33 节点/70 边/Kahn/边违例/波次模拟"重写复算）；④ 实施基线应取**当时最新 main**（当前领先基线 49 提交，商业模块仅 benefits.go +16 漂移，§5.2），而非沿用 29c1e5635；⑤ 合并回 main 前对代码提交重跑 OCR（§9）。

## 12. 验证命令清单（撰写会话实跑）

### 12.1 首版会话

1. `ls docs/plans/ | grep '^issue-72'` → 仅 dag、issues-inventory 2 文件。
2. `git log --oneline 29c1e56353b2b36be242018cecb43bcb3a5ef7c8..HEAD` → 实施终点 `7cf755474`（+报告提交后为 2 提交）。
3. `git branch --list 'codex/issue-72-lago-*'` → 空（0 个 Issue 分支）。
4. `git status` / `git branch --show-current` → `codex/issue-72-lago`。
5. `git branch -a | grep -i lago` / `git branch -vv` / `git ls-remote --heads origin | grep lago` → 远端仅历史 `lago-73-community-env`、`lago-integration`；本分支无 upstream 未推送。
6. `git show --stat 7cf755474` → 2 文件 +614 行。
7. `git log --oneline --all | grep`（b57be1602/d1eec16be/6ea0511f5/e87eb459c）→ 全部存在；`git merge-base --is-ancestor e87eb459 29c1e56353` → true。
8. `ls docs/plans/ledgers/` → lago-78/79/80；`ls .superpowers/sdd/ | grep -i lago` → 空；`ls .superpowers/sdd/2026-09-21-lago-t06-lago-customer` → No such file。
9. `find`（`*issue-72*`、`issue-72-flow-evidence-*`、`-iname '*ocr*'`、`t05-*/t06-*`）→ flow-evidence 不存在；OCR 无相关文件；t05/t06 证据在 `deploy/lago/evidence/`。
10. `ls docs/migrations/lago/`、`ls deploy/lago-lab/`、`ls deploy/lago/`、`ls deploy/lago/evidence/` → §4 索引路径存在。
11. `grep/sed` 读取 waves、spec、三份 ledger 头部 → §3/§5/§9/§10 引用内容。

### 12.2 修订版会话（本轮评审意见补查）

12. `git rev-list --count 29c1e5635..main` → **49**；`git rev-list --count main..HEAD` → **2**。
13. `git diff --stat 29c1e5635..main`（全库尾行 136 files +28172/−8174；`-- internal/modules/commercial/` 仅 benefits.go +16；`git log 29c1e5635..main -- docs/plans/` 与 `-- deploy/lago/ docs/migrations/lago/ docs/plans/ledgers/` 均 0 提交）。
14. `sed -n '240,248p' internal/container/container.go` → **:244 = ommeter.NewGatewayFromEnv 装配（已核）**；`sed -n '680,690p' .../commercialplatform/lago.go` → **:684-686 createSubscription 注释明示不带 activation_rules（已核）**。
15. Read `budget_pg_test.go:50-59` → 第 56 行为 `t.Fatal(err)`，清单:165 行号引用**未能复核**（§7）。
16. Read `docs/specs/...design.md:212-237` → 矩阵第 1-24 项完整行号（已核）；spec:246-248 Completion gate（已核）。
17. `head deploy/lago/evidence/t08-run.txt` → "real-stack run — operator timeline"（证据性质已核）。
18. `ls /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/` → 9 个 worktree，**无 lago-***；`ls 主检出 .superpowers/sdd/ | grep -i lago` → none。
19. `git fetch origin lago-integration --depth=1` + `git ls-tree FETCH_HEAD .superpowers/sdd/` → 仅 7 个非 lago 条目；同法 `lago-73-community-env` → 无 lago 条目（§11.2）。
20. `grep -n 'environ\|STRIPE\|sk_test' deploy/lago-lab/payment-activation/run_lab.py` → :101-107 环境变量取键、live 键拒绝（已核，§10.2）。
21. `sed DECISION.md §5`（:100-140）→ 三选项表 + "The lab recommends (a)..."（已核）；`grep verdict.md a2` → L58-66 阻断证据、L21-22 协调层总则、L160-166 义务表（已核）。
22. `gh issue view 74 --json state,comments` → **OPEN**；解锁命令已内联 §10.2（评论并记录实现于 lago-74 分支 e5294a2a..72aa7476、并入 lago-integration 4bfad830——该两分支现已不存在，进一步佐证 §11.2）。
23. `for i in 72 $(seq 73 105); do gh issue view $i --json state,number ...` 两遍 → **#72 OPEN；#73/75/76/77/78/79/80 CLOSED（7）；#74、#81-#105 OPEN（26）**。第一遍 2 个请求返回空（瞬时失败），第二遍点名复核 7 张 CLOSED 票全部确认；总数 34 查询无缺漏。

### 12.3 验收者第一步补验清单（建议命令）

1. **GitHub 状态复核**（状态随时可能漂移）：`for i in 72 $(seq 73 105); do gh issue view $i -R 1123786563/WeKnora-fork01 --json number,state --jq '"\(.number):\(.state)"'; done` → 期望 7 CLOSED / 26 OPEN / #72 OPEN。
2. **DAG 校验复算**：脚本未归档（§7），需按 DAG §7 规格重写（33 节点/70 边/Kahn 无环/拓扑序零违例/波次无停滞/传递计数 25、21）或信任文档记录。
3. **测试重跑（第一手证据）**：`python3 -m pytest deploy/lago/ -q`（期望 46 passed+62 subtests）；`go build ./... && go test ./...`；lab 套件 `python3 -m pytest deploy/lago-lab/ -q`（wallet-semantics 104 / pricing-group 60 的分项口径见清单 §2）；PG twin：`go test ./internal/modules/commercial/repository/commercial/ -run TestBudgetPG`（清单预测 2 FAIL，行号存疑，§7）。
4. **Git 事实复核**：`git log --oneline 29c1e5635..HEAD`（期望 2 提交）；`git rev-list --count 29c1e5635..main`（漂移复核）；`git ls-remote --heads origin | grep lago`（确认仍未推送）。
5. **代码引用抽查**：`sed -n '244p' internal/container/container.go`；`sed -n '684,686p' internal/modules/commercial/commercialplatform/lago.go`。
6. **#74 解锁预演（密钥到位后）**：按 §10.2 D1 命令，密钥仅经环境变量注入，证据提交前复核脱敏。

**未运行的检查**：任何测试套件（go test/pytest/DAG python3 校验）撰写会话均未运行；spec:212-248、DECISION.md §5、verdict.md a2、container.go:244、lago.go:684-686、run_lab.py:101-107 已核原文；账本引用的三个 `2026-09-21-lago-t0{6,7,8}` 计划文件存在性未单独核验（§4 注）。
