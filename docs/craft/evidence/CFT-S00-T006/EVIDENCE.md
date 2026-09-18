# CFT-S00-T006 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `@assistant-ui/react` **0.15.20 精确锁版**入 `packages/views/package.json`（peer `react ^18 || ^19` 与仓库 React 19.3 兼容；pnpm-lock 更新）。旧 G2"纯 React 模拟"决策由 D001 正式替代。
- `packages/views/src/craft/assistant-runtime.tsx`（新增）：
  - `craftThreadMessages`：W05 投影行 → `ThreadMessageLike[]`，**稳定 ID**（`turn-N-user/assistant`、`{runId}-user/assistant`，重放不铸造新 ID）；
  - `useCraftAssistantRuntime`：`useExternalStoreRuntime` 适配——`isRunning` 仅来自调用方主 Run 投影（浏览器不判定主 Run 成败）；`onNew` 路由到命令桥端口；`onCancel`/`isDisabled`（只读会话）可选；
  - `CraftAssistantThread`：最小真实挂载（`AssistantRuntimeProvider` + `ThreadPrimitive.Root/Viewport/Empty/Messages` + `ComposerPrimitive.Root/Input/Send`，样式类挂 `wk-craft-*` 承接 T003 令牌）。
- `packages/views/src/craft/assistant-runtime.test.tsx`（新增，5 tests，JSDOM + createRoot + act，同 packages/ui/interaction.test.tsx 先例）。
- 根 `package.json`：`test:craft:shared` 收集 `views/src/craft/*.test.tsx`；`typecheck:shared` 纳入 `assistant-runtime.tsx` 并补 `--lib ES2022,DOM --skipLibCheck`（radix 依赖 d.ts 需要）。

## 关键实现发现（SDK 0.15.20 实测）

- **ThreadMessageLike 输入必须提供 `convertMessage`**（恒等即可）：无 converter 时 runtime 原样采用消息行，其 state getter 在缺 `metadata` 的行上崩溃（`external-store-thread-runtime-core.ts` 的 `!store.convertMessage ? store.messages : ...` 分支实测确认）。
- JSDOM 需补：`ResizeObserver` stub、`MutationObserver`/`getComputedStyle`（借自 window）、`HTMLElement.prototype.scrollTo`、rAF。
- React 19 的 `useSyncExternalStore` 要求 store 的 `getSnapshot` 缓存引用（W05 log 本就如此，fixture 对齐）。

## 验收断言对照

- 真实渲染一条 user 和 assistant 消息 ✓（测试 2：PROMPT 文本与 assistant 文本出现在真实 DOM）
- StrictMode 挂载/卸载不重复订阅 ✓（测试 3：挂载 1-2 活跃订阅、重渲染不堆叠、卸载归零）
- 依赖图只有预期 React 实例 ✓（测试 5：views 与 @assistant-ui/react 解析同一 react 安装；React 19.3）
- 附加：isRunning 仅由外部主 Run 投影驱动（测试 4）；稳定 ID + 完成度仅来自主 Run 终态（测试 1）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `pnpm exec tsx --test packages/views/src/craft/assistant-runtime.test.tsx` | 0 | 5 pass / 0 fail |
| `pnpm run test:craft:shared`（含新 tsx 收集） | 0 | **73 pass / 0 fail**（68→73） |
| `pnpm run typecheck:shared`（含 assistant-runtime.tsx） | 0 | 通过 |
| `pnpm run typecheck:web` / `pnpm run build:web` | 0 / 0 | vite 真实构建通过（"真实 Provider 可构建"） |
| `bash e2e/craft-stack.sh up mock && run mock` | 0 | 6 passed (25.1s)——workbench 现有 DOM 未动，无回归 |

## 未验证事项 / 回退

- CraftAssistantThread 尚未替换 workbench 对话列 DOM（那是 T007/T010 的组合工作；本轮 e2e 仍验旧渲染，两者并存不冲突）。
- composer 中文 IME/Enter 语义在 assistant-ui Composer 上的适配留待 T010（现 placeholder 已中文）。
- 回退：revert 本提交 + `pnpm --filter @weknora/views remove @assistant-ui/react`。
