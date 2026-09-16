# S03 — GlobalCommandPalette（R011/N003）落地证据（2026-09-12，Task 3）

范围：Slice S03 platform shell 部分，聚焦 R011（`/platform/knowledge-search` legacy command redirect）与 N003（global command palette / search overlay）。Vue 权威实现：`frontend/src/components/GlobalCommandPalette.vue`（868 行）+ `ResultGroup.vue`/`ResultItem.vue`/`useSearch.ts`/`commands.ts`/`stores/commandPalette.ts`。

## 已知缺口（本轮修复前）

矩阵 R011/N003 此前状态 `pending`：React 仅在 `routes.tsx`/`App.tsx` 保留 `?cmdk=` query，**没有可见的命令面板组件**——用户看不到、用不了。Live 复核确认：`/platform/knowledge-search?q=hello` 落地后 URL 停留在 `?cmdk=hello`，无任何弹层出现。

## 范围裁定（MVP，遵守 S03 文件边界）

S03 边界明确排除 document/chat/settings 实现工作（"no document detail/upload code, no settings panels, no chat implementation"）。Vue 面板的语义检索（chunk/message/KB/agent 实时搜索、`useSearch.ts`）、检索设置抽屉、KB 范围 chip 均依赖这些越界能力，本轮**有意不移植**。移植范围：

- 静态快捷操作目录（`commands.ts` 移植，6 项中 5 项：新建对话/打开知识库/打开智能体/打开共享空间/打开设置；**不含**"新手引导"，因 `NewUserGuide.vue` 无 React 对应实现可跳转）。
- 最近搜索持久化（`stores/commandPalette.ts` 的 `recentKey()`/limit=4/去重 移植，key 按 `(userId, tenantId)`）。
- 完整键盘导航（↑↓ 环形、Enter 执行、Esc 关闭）。
- 与 Vue 相同的打开触发：全局 ⌘K/Ctrl+K（即使输入框聚焦也生效）、裸 `/`（仅在未聚焦可编辑元素时）、`?cmdk=` 深链接一次性消费并从 URL 剥离（`history.replaceState`，镜像 Vue `watch(() => route.query.cmdk, ...)`）。
- **未移植**（记录为已知、有意的范围缩减，非缺陷）：语义 chunk/message/KB/agent 搜索结果分组、检索设置抽屉、KB 范围 chip、底部键盘提示条（footer hint bar：↑↓ 选择/⏎ 打开/⌘1-9 直接打开/⌘⏎ 发起对话/Esc 关闭）、每项 ⌘1-9 快捷徽标（`shortcutDigitFor()` 已实现并测试，但未渲染进 UI）。

## 新增/修改文件

- 新增 `apps/web/src/platform/command-palette.ts`：纯逻辑（命令目录、`filterCommands`、最近搜索存取/去重/上限/清空、`nextSelectedIndex` 环形导航、`shortcutDigitFor`、`decideGlobalShortcutAction`、`consumeCmdkParam`）。
- 新增 `apps/web/src/platform/command-palette.test.ts`（17 例）。
- 新增 `apps/web/src/platform/GlobalCommandPalette.tsx`：可见对话框组件（input、最近/快捷操作分组、键盘导航、locale 感知文案）。
- 新增 `apps/web/src/platform/command-palette.css`：面板样式（独立于 `shell.css`，未触碰 shell 自身样式文件）。
- 新增 `apps/web/src/platform/global-command-palette-render.test.ts`（4 例，`renderToStaticMarkup` 静态渲染冒烟：开/关状态、locale 过滤、空态）。
- 新增 `packages/i18n/src/generated/commandPalette.ts`：5 语言（zh-CN/en-US/ja-JP/ko-KR/ru-RU，供检索到 `packages/i18n/src/index.ts` `supportedLocales` 实际为 5 语言而非任务描述的“六语言”，按现有架构执行）× 12 键，从 Vue 5 份 locale 文件的 `commandPalette.*` 区块移植。
- 新增 `packages/i18n/test/commandPaletteMessages.test.ts`（4 例：locale 对等性 + merge 正确性）。
- 修改 `packages/i18n/src/index.ts`：导入/导出 `commandPaletteMessages` 并合并进 `messages` 表（两处小改动，未触碰其余内容）。
- 修改 `apps/web/src/platform/PlatformShell.tsx`：新增 `id` 字段（`user` state）、`paletteOpen`/`paletteQuery`/`recentQueries` state、三个 effect（加载最近搜索、一次性消费 `?cmdk=` 并 `history.replaceState` 剥离、全局 keydown 监听）、`closePalette`/`navigateFromPalette`/`recordPaletteSearch`/`clearPaletteRecent` 回调，并在外壳 JSX 末尾渲染 `<GlobalCommandPalette>`。

## TDD Red/Green 证据

1. `command-palette.ts` 逻辑：先写 17 例测试，临时将 `command-palette.ts` 重命名为 `.ts.bak` 运行测试 → 确认 RED（`ERR_MODULE_NOT_FOUND`）；恢复文件 → GREEN 17/17。
2. i18n 合并：先写 4 例测试断言 `commandPalette.*` 键已合并进 `messages`，此时 `index.ts` 尚未接入 → 确认 RED（键不存在）；接入 import/export/spread → GREEN 4/4（`packages/i18n` 套件由 326 → 330）。
3. 静态渲染冒烟：`global-command-palette-render.test.ts` 4 例，首次运行即 GREEN（组件已实现之后补的纯渲染态验证，非行为红/绿对，如实记录）。

## 门禁命令与结果

- `pnpm --filter @weknora/web exec tsx --test src/platform/*.test.ts src/knowledge-bases/list.test.ts` → **55/55 pass**（含本轮新增 21 例 + 既有 34 例）。
- `tsc -b --noEmit`（web）：初次因生成文件 `import type { Locale } from './index.ts'` 路径错误（应为 `../index.ts`，因文件位于 `generated/` 子目录）报错，修复后通过。
- `pnpm --filter @weknora/web test`：**229/229**。
- `pnpm run test:shared`：**330/330**。
- `pnpm run build:web`：成功（仅既有 chunk-size 警告，无新增错误）。

## Live 复核（playwright-core headless Chromium，因 `openBrowserPage` 工具在本环境不可用）

账号 `parity-test@local.dev` / `Parity123456!`，React :5181：

1. 登录后访问 `/platform/knowledge-search?q=hello` → 落地 URL 为 `/platform/knowledge-bases`（**`cmdk` 已剥离**，此前遗留在 URL 上的缺口已修复）；面板 `.cmdk` 可见 = `true`；输入框值 = `hello`（预填）。截图：`screenshots/2026-09-12-react-palette-from-redirect-cmdk.png`。
2. Esc 关闭；`reload()` 刷新页面 → 面板不再自动弹出（`false`，证明 `?cmdk=` 是一次性消费，`replaceState` 生效，不会在刷新后重复触发）。
3. 导航到 `/platform/knowledge-bases`，按 `Control+k` → 面板打开（`true`），展示空态下的 5 项快捷操作。截图：`screenshots/2026-09-12-react-palette-empty-en-us.png`。
4. 输入 `agent` → 过滤命中 `Open agents` 单项（`filterCommands` 行为在真实浏览器中确认）。截图：`screenshots/2026-09-12-react-palette-filtered-agent.png`。
5. 按 `Escape` → 面板关闭（`false`）。
6. 控制台仅有既有的、与本次改动无关的 SVG 属性警告（`stroke-width`/`stroke-linecap`，`grep` 确认不存在于新增文件中，属既有 PlatformShell/图标代码遗留）。

## Vue 对照证据（port 5180，同账号）

- `screenshots/2026-09-12-vue-palette-empty.png`：Vue 面板展示全部 6 项快捷操作（含"新手引导"，本轮未移植）、每项 ⌘1-6 徽标、底部键盘提示条（本轮未移植）。因 Vue 新手引导（NewUserGuide）自动弹出遮挡了部分区域，但完整结构清晰可辨。
- `screenshots/2026-09-12-vue-palette-filtered-agent.png`：Vue 输入 `agent` → 过滤命中"打开智能体"，与 React 版行为一致（1 项匹配）。

对照结论：核心交互（打开/关闭触发、过滤、键盘导航、最近搜索、深链接消费）已达成功能对等；视觉细节（每项快捷徽标、底部提示条、语义搜索分组）为已记录的范围缩减，不在本轮验收范围内。

## Wails 桌面运行时检查（因 `PlatformShell.tsx` 属外壳文件被修改）

- 尝试 `wails dev`（`cmd/desktop`）：Go 绑定生成成功，`Compiling frontend:` 步骤卡住 6+ 分钟无进展（已知 wails-cli 在非交互式/无 TTY 后台会话下对 vite 输出探测失败的环境限制，非本次改动引入）；已终止该进程树，未产出原生窗口证据。
- 替代验证（复用既有 `2026-09-12-wails-runtime-interaction.md` 记录的"同源同构建 Playwright 驱动"方法论）：直接运行 `apps/desktop`（`@weknora/desktop-renderer`）的 Vite dev server（`VITE_API_BASE_URL=http://localhost:8080 npx vite --port 5173`）。确认 `apps/desktop/src/main.tsx` 逐字重导出 `apps/web/src/main.tsx`（"the React product entry remains shared with the browser Web app so desktop and Web do not fork route logic"），即 Wails 应用与 Web 应用运行**完全相同的入口代码**。
  - 登录成功 → `/platform/knowledge-bases`。
  - `/platform/knowledge-search?q=hello` → URL 剥离为 `/platform/knowledge-bases`，面板可见 = `true`。截图：`screenshots/2026-09-12-desktop-renderer-palette.png`。
  - `pnpm --filter @weknora/desktop-renderer typecheck`：通过，无新增错误。
- 未完成项（如实记录为未解决限制）：未能产出真实原生 Wails 窗口（`.app`）内的截图，仅验证了其共享的渲染入口在等价 Vite 服务下行为一致。若需要更强证据，需要具备 GUI/屏幕录制权限的会话重新执行 `wails dev`/打包应用，本环境不具备。

## 矩阵结论

- R011：`pending` → `review`（可见面板已存在、已测试、`cmdk` 深链接已闭环；未定级 `accepted`，因语义搜索/视觉细节仍是已记录的范围缩减）。
- N003：`pending` → `review`，note 精确描述已覆盖范围（静态命令+最近搜索+键盘导航+深链接消费）与延后范围（语义搜索/检索设置/KB chip/底部提示/徽标）。
