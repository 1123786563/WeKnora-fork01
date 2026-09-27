# Issue #86 执行账本（[Lago 14] 套餐额度与充值额度按到期顺序消费）

## 计划身份

- 计划文件：`docs/plans/issue-72-plan-86.md`（本 worktree）
- Issue：https://github.com/1123786563/WeKnora-fork01/issues/86
- Worktree：`.worktrees-issue72/issue-86`，分支 `codex/issue-72-lago-86`
- 集成基线：`ee02d3218`（issue-72: ocr issue-82 round 1；与 lago-int HEAD 同点，实测 `git -C .worktrees-issue72/lago-int log -1`）
- 计划编写日期：2026-09-28

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
