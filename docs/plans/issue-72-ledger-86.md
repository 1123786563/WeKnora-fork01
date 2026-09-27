# Issue #86 执行账本（[Lago 14] 套餐额度与充值额度按到期顺序消费）

## 计划身份

- 计划文件：`docs/plans/issue-72-plan-86.md`（本 worktree）
- Issue：https://github.com/1123786563/WeKnora-fork01/issues/86
- Worktree：`.worktrees-issue72/issue-86`，分支 `codex/issue-72-lago-86`
- 集成基线：`ee02d3218`（issue-72: ocr issue-82 round 1；与 lago-int HEAD 同点，实测 `git -C .worktrees-issue72/lago-int log -1`）
- 计划编写日期：2026-09-28；审查 R1 修订：2026-09-28（8 项 findings 全部处置，见下）

## 审查 R1 修订记录（2026-09-28，8 项全处置）

1. **High（消费顺序编码混合场景乱序）**：采纳。原「充值单类 priority=2 + 月度二元让位（1↔3）」在「老化充值+新充值+月度」三类共存时给出 A→B→M 而正确序为 A→M→B，已证伪。修订为两层：创建初值（原编码保留为初值）+ **刷新时权威重排**（`WalletRank` 按 (expires_at, created_at) 计秩 → 不一致者 `PUT /api/v1/wallets/:id {priority}`）。重排可行性有本会话实测源码证据：pinned v1.53.0 运行容器 `wallet_actions.rb` update_params permit `:priority`、`Wallets::UpdateService` 赋值 priority、terminated 钱包拒绝 update（docker exec weknora-lago-82r5-api-1 grep/sed 实读）。新增混合场景测试 `TestWalletRankMixedFamilies`/`TestLagoRebalancePutsMixedFamiliesInExpiryOrder`/`TestRefreshRebalancesMixedFamilies`/Task 6 阶段 d（真实栈重排 + BLOCKED gate：PUT 被拒则按 spec L132 升级，不静默降级）。矩阵 AC② 行同步扩充。
2. **Med（端口 48895/48896 被 t11 栈占用）**：采纳。docker ps 复核实测 t11 占 48895/48896，改选 48897/48898（实测空闲），端口纪律行加「执行前 docker ps 复核」指令。
3. **Med（TestLagoBenefitsChain 不存在）**：采纳。grep 实测唯一集成测试为 `TestLagoBasePlanIntegration`（lago_benefits_integration_test.go:148），引用已替换。
4. **Med（容器装配点错误）**：采纳。实测 container.go:2597 属 newMobileVoiceHandler 手动构造（无关）、BenefitsService 装配在 :915 fx Provide、fx 图无 BudgetStore provide。修订：`NewBenefitsService` 签名增第 5 参 + 新增 `must(container.Provide(repocommercial.NewBudgetStore))`，废弃 WithBudgetStore 链式法，计划写明两处改动与 2597 不相干勿改。
5. **Med（PG fixture 缺 000161 owner 列）**：采纳。Task 3 Step 4 增「修复先行」步骤（fixture 补读 000161_commercial_reservations_owner.up.sql），明确这是 issue inventory L180 登记、既有 2 测试 FAIL 的代码级根因修复，属本任务交付物；blocked-env 不得掩盖代码级缺陷。
6. **Low（矩阵 void/refund 无范围声明）**：采纳。矩阵下加范围声明：void/refund 归 #95/#96/#97，#105 矩阵 8 全绿由其闭合。
7. **Low（RED 失败解释矛盾）**：采纳。统一为「顺序未定义，任意命中行使断言失败、RED 稳定」。
8. **Low（purchase 首期批次归类未显式）**：采纳。Task 1 Step 3 改三分显式归类（monthly/purchase→monthly/topup），L9-13 contracts 失真注释列入 Task 5 修改项。


## 编写期核实记录（全部本次会话实测）

1. **无预写稿**：`git -C .worktrees-issue72/lago-int show HEAD:docs/plans/issue-72-plan-86.md` → `fatal: path ... does not exist`。从零模式编写。
2. **#85 未在基线**：编排指令声称前置接口已落地，实测基线无 #85 代码（`git log --grep` 无命中；`platform.go` 无 top-up 命令；`lago.go:1005-1011` top-up 钱包为 future 预期）；`.worktrees-issue72/issue-85` 并行 worktree 尚无提交。计划按「规格锚定 + 各任务 #85 对齐步骤」处理。
3. **writing-plans 技能**：任务指定路径 6.4.1 不存在，实际读取 6.4.2（`~/.codex/plugins/cache/openai-curated-remote/superpowers/6.4.2/skills/writing-plans/SKILL.md`）。
4. **关键代码事实**（计划引用的行号均经 Read 核实）：`createWallet` 未发送 priority（lago.go:895-919）；快照跳过 top-up 钱包（lago.go:1005-1011）；`lagoWallet` 无 created_at（lago.go:760-772）；lot 分配无 tie-break（budget_reservation.go:149-152）；无任何生产代码写 `commercial_budget_lots`（全仓 grep）；`MonthlyWalletPriority=1` 常量存在但从未上线（subscription_command.go:93-96）；预算行/契约/前端缺口与 Issue 调查一致。
5. **测试基线实跑**：`go test ./internal/modules/commercial/repository/commercial/ -count=1` → `ok 1.272s`（2026-09-28，本 worktree）。
6. **环境实测**：`docker ps` 显示 :48889 被 `weknora-lago-82r5-*` 回放栈占用（非主 `weknora-lago` 栈）——计划验证方案改用 48895/48896；`api-clock` 无时钟偏移 env（`docker inspect` 无 CLOCK 变量）——跨月真时间推进记为已知边界。
7. **t03 verdict 语义**（消费顺序设计依据）：`priority ASC, created_at ASC`（E2 实测）；a1 预派发注册表拒绝+settle-wait 义务；E3 无外部幂等；a2 六钱包上限→ADR-0012 2026-09-23 修订（充值批次协调层承载）。

## 计划自检结论（writing-plans Self-Review 五项）

1. **Spec 覆盖**：五条验收标准全部映射任务与测试（追踪矩阵）；充值批次「发放」属 #85，计划明示边界与对齐步骤，无缺口。
2. **Step 扫描**：无 TBD/占位/「处理边界情况」类空步骤；测试代码均为可落地形式（构造器/harness 名经核实：`newCombinedStub`/`subAdapter`/`grantCommand`/`testBudgetStore`/`seedBudget`/`budgetRequest`/`NewFakeAdapter`/`types.TenantIDContextKey` 注入法）。
3. **类型一致**：`BatchSourceMonthly/TopUp`、`MonthlyWalletPriorityFor`、`GrantIncludedCreditsPayload.Priority`、`LotSyncBatch`/`SyncLots`、wire credits 字段 ↔ TS `CommercialAccountCredits` 跨任务命名一致。
4. **Review Focus**：五类失败模式各有 owning task 的具名测试（见计划节）。
5. **比例**：代码块仅测试与签名，无实现转写；计划长度主要来自任务要求的测试代码与真实流程验证方案。

## 执行状态

- [ ] Task 1 批次快照增广
- [ ] Task 2 消费顺序 priority 编码
- [ ] Task 3 lot 同步与分配 tie-break
- [ ] Task 4 余额分解 API
- [ ] Task 5 契约/api-client/BillingPage
- [ ] Task 6 真实栈验证 + 文档收口

（执行时逐任务补：命令+输出摘要、证据路径、#85 对齐差异记录、OCR/Review 结论。）
