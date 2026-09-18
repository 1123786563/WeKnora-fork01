# CFT-S01-T010 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `packages/views/src/craft/workbench.tsx`：对话列从 W05 自绘消息切到**真实 assistant-ui Thread**（T006 `CraftAssistantThread`）：
  - store 适配：W05 log + projectAssistant 派生 `{text, complete}` 快照（ref 缓存稳定引用，React 19 useSyncExternalStore 安全）；isRunning 仅来自主 Run 状态（`isMainRunTerminal` 取反），子委派 finish 永不翻转；
  - archived turns → `archivedTurns` 行（prompt 可空）；live 轮走 prompt+runId 稳定 ID；
  - `composer={false}`：宿主保留 W05 表单 composer（草稿机、附件 chips、craft-prompt/craft-send/craft-upload 锚点全部不动）；thread 内置 Composer 关闭，无双输入框；
  - 工具卡换 T007 `CraftToolFactList`（稳定 callId key）；产物版本卡 / workspaceUnavailable / 交互卡（InteractionCardView）保留为 thread 下方 supplement 区（独立限高滚动）。
- `packages/views/src/craft/assistant-runtime.tsx`：`composer?: boolean` 开关；`CraftThreadRow.prompt` 放宽为可空。
- `packages/views/src/craft/craft.css`：thread 布局（viewport flex:1 overflow-y:auto、supplement 限高滚动、消息样式）；`.wk-craft-page` 补 `min-width:0; overflow-x:clip`（390px 溢出防护——由本任务测试抓到的真实缺口）。
- `apps/web/e2e/craft-report.spec.ts`：spec 05 容器定位器 `.wk-craft-msgs` → `.wk-craft-thread-viewport`（类名更新；`craft-*` data-testid 锚点未动）。
- 渲染链组件（preview/files/sources/document/spreadsheet/slides/interaction）补 React 默认导入（classic-JSX 测试运行时需要；vite automatic 下无害）。
- `packages/views/src/craft/workbench-layout.test.ts`（新增，3 tests，整组件 SSR + stub controller/log）。

## 验收断言对照

- Composer 位置与独立滚动不冲突 ✓（测试 1：viewport 与 form 为兄弟节点、composer 不在滚动子树内；CSS 规则 viewport/supplement 各自 overflow-y:auto；四个 craft-* 锚点存在）
- 切面板保留草稿与运行订阅 ✓（测试 2：窄屏切换是 `data-narrow-hidden` 属性 + CSS display:none，两个 pane 始终挂载——React 不卸载子树，draft state 与 SSE 订阅保留）
- 长标题与 390px 下按钮不越界 ✓（测试 3：72 字长标题逐字渲染；title overflow-wrap:anywhere、actions flex-wrap、page overflow-x:clip、消息 overflow-wrap 规则齐备）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `pnpm exec tsx --test packages/views/src/craft/workbench-layout.test.ts` | 0 | 3 pass / 0 fail |
| `pnpm run test:craft:shared` | 0 | **97 pass / 0 fail**（94→97） |
| `pnpm run typecheck:web` | 0 | 通过 |
| `bash e2e/craft-stack.sh up mock && run mock` | 0 | **6 passed (22.4s)**——对话列真实运行在 assistant-ui Thread 上，全链路（创建/上传/生成/预览/修改/下载/重开/隔离/幂等）浏览器验证 |

## 未验证事项 / 回退

- 真实视口 1440/1280/1024/768/390 基线截图（T034 视觉回归轮统一执行；本轮类合同 + e2e 已覆盖结构与行为）。
- assistant-ui Composer 的 IME/Enter 语义未启用（宿主 composer 沿用 W05 行为；后续如切换再验）。
- 回退：revert 本提交（workbench 对话段单块替换 + 组件增量 + CSS 追加 + spec 定位器一行）。
