# CFT-S00-T004 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `packages/views/src/craft/shell.tsx`（新增）：craft 宿主壳组合层——`CraftShell`（.wk-craft 作用域 + overflow guard）、`CraftPanelHeader`（标题/副文本/动作簇，disabled 动作保留可读原因）、`CraftNotice`（aria-live + 6 种 kind，状态不只靠颜色）、`CraftDrawer`（460px 右抽屉，**焦点/Escape 全部委托共享 Sheet**，不建第二套焦点系统）。
- `packages/views/src/craft/shell.test.tsx`（新增，4 tests，JSDOM + createRoot + act + css 短路 hook，先例 packages/ui）。
- `packages/views/src/craft/craft.css`：追加 shell/panel-header/notice/drawer 规则（全部走 T003 语义别名）。
- 根 `package.json`：typecheck:shared 纳入 shell.tsx + packages/ui/src/styles.d.ts（`declare module '*.css'`，让根 tsc 理解 ui 入口的 css side-effect）。

## 验收断言对照

- Dialog Escape 关闭并归还焦点 ✓（测试 1：真实 Sheet DOM，Escape 前显式聚焦 panel——Sheet 的 Escape 是焦点作用域的；焦点归还由 Sheet 的 restoreRef 契约承载，packages/ui/interaction.test.tsx 既有覆盖）
- 只读/disabled 不触发命令 ✓（测试 2：disabled 按钮原生不可点，click 不触发 onAction；原因文本在 DOM）
- 1440 与 390 视口无页面级横向溢出 ✓（类合同层：测试 4 断言 `.wk-craft-shell` 携带 + craft.css 有 `overflow-x` 规则；**真实视口证明在 e2e（T024/T034）**，单测不伪造浏览器断言）
- 附加：6 种 Notice 全部以文本表达状态（测试 3）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `pnpm exec tsx --test packages/views/src/craft/shell.test.tsx` | 0 | 4 pass / 0 fail |
| `pnpm run test:craft:shared` | 0 | **82 pass / 0 fail**（78→82） |
| `pnpm run typecheck:shared` | 0 | 通过（含 shell.tsx） |
| `pnpm run typecheck:web` | 0 | 通过 |
| `pnpm exec tsx --test packages/design-tokens/src/craft.test.ts` | 0 | 4 pass（craft.css 增量仍零品牌字面值） |

## 未验证事项 / 回退

- Shell 组件尚未被 home/workbench 实际采用（T009/T010 组合时接线；CraftDrawer 460px 的移动端全宽断点在 CSS media query，由 T034 视觉回归验证）。
- ui Button 宿主旧样式未改动（仅消费；craft.css 增量规则只作用于 .wk-craft 后代）。
- 回退：revert 本提交（shell 两文件纯新增 + craft.css 追加段 + package.json 两处列表）。
