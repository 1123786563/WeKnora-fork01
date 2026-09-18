# CFT-S01-T007 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `packages/views/src/craft/presentation-replay.test.ts`（新增，4 tests）：重放稳定性组合层断言——W05 log（seq 去重）× projectAssistant（主 Run 独占完成度）× craftThreadMessages（T006 稳定 ID）。
- `packages/views/src/craft/thread.tsx`（新增）：工具/交互卡渲染——`CraftToolFactList`（React key = 服务器 call_id，重放不重复）、`CraftToolFactCard`（用户可读状态文案，unknown 保持"结果待核对"不改判成功/失败）、`CraftInteractionSummary`（aria-live 播报，完整决策 UI 仍在 interaction.tsx）。渲染消费同一 projection，不解析 OpenCode 原始 SSE。
- 根 typecheck:shared 纳入 thread.tsx。

## 验收断言对照（fixture = 真实 SSE wire 帧：JSON data + craft payload 包络）

- 同 seq 重放两次消息不增加 ✓（测试 1：双重重放前后 craftThreadMessages deepEqual——数量/ID/正文一致；文本块各出现恰好一次）
- 增量文字不重复拼接 ✓（同上：`正在生成`/`销售报告` 各 1 次）
- 子任务完成后主消息仍 running ✓（测试 2：delegation.finished 后 childStatus='succeeded' 而 projection.complete=false、assistant status={type:'running'}；主 Run 终态才 complete）
- 附加：工具卡跨重放不重复 + 稳定 call_id（测试 3）；组件渲染稳定 key + 可读文案 + unknown 不改判（测试 4）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `pnpm exec tsx --test packages/views/src/craft/presentation-replay.test.ts`（任务卡名 .ts 保持一致） | 0 | 4 pass / 0 fail |
| `pnpm run test:craft:shared` | 0 | **90 pass / 0 fail**（86→90） |
| `pnpm run typecheck:shared`（含 thread.tsx） | 0 | 通过 |

## 说明

- 本任务的投影语义大部分已由 W05/W04 既有实现满足（presentation.test 的 dedupe/C03/main-run invariants）；本轮增量是**组合层契约钉死**（log×projection×threadMessages 跨层一致性）+ thread.tsx 卡组件（此前不存在）。
- thread.tsx 尚未挂进 workbench 对话列（T010 工作台整合时接线；组件契约已由测试锁定）。

## 回退

revert 本提交（两文件新增 + package.json 一处列表）。
