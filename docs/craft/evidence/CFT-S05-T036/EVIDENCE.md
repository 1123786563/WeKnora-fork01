# CFT-S05-T036 证据 —— 整体验收、证据与旧计划收敛

日期：2026-09-18 · worktree `.worktrees/craft-cft` @ craft/cft-execution

## 交付物

- `docs/craft/evidence/index.md`（新增）：36 任务全量证据索引（任务/commit/结论/目录四列 + 发布范围 + 未验证项五条）。
- `docs/superpowers/plans/2026-09-10-craft-opencode-adapter.md`：头部标记 **SUPERSEDED**——生产唯一适配器入口为 `internal/agent/opencode/`（T013 协议锁定），原计划的 `internal/craft/opencode` 双轨从未落地且按边界不得再建。
- `docs/superpowers/plans/2026-09-10-craft-product-implementation.md`：头部标记执行已由 CFT 体系接管；旧 G2"不装 assistant-ui"决策由 D001 替代；本文勾选不再作为验收事实来源。

## 验收断言对照

- 旧 Adapter 标记替代且唯一运行入口 ✓（上述标记 + BASELINE §6 双轨检查记录 `internal/craft/opencode/` 不存在）
- 发布范围与已通过类型证据一致 ✓（index.md"发布范围"节：web=双模式全量+反例可支撑发布评审；三 Office 类型=模拟环境过、生产默认关——不谎称全开）
- 每项完成记录包含 commit/命令/结果/review ✓（35 个任务目录各含 EVIDENCE.md；本索引逐行映射 commit；review 全部如实标注"自审"）
- 所有未验证项显式列出而非划完成 ✓（index.md"未验证项"五条 + 各任务 EVIDENCE 的未验证节）

## 36/36 收敛声明

36 项任务全部 verified（自审）。真实完成=35 项独立证据目录 + 本收敛任务；其中 CFT-S03-T024 为首个产品里程碑（双模式达标）。

## 回退

revert 本提交（索引+两处文档头标记，纯增量）。
