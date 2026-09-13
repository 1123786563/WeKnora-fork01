# N003 — 命令面板作用域 ⌘1-9 快捷键 + ResultItem ⌘N 徽标（2026-09-13，子代理切片）

范围：N003 既定 deferred 项（progress 账本 Round N+3 遗留登记、Round N+4 第四批派发）——把 Vue 命令面板打开期间的 ⌘1-9 快速跳转与每行 kbd 数字徽标补齐到 React。协调者裁决背景：Round N+3 曾否决 shell-sessions-header 切片的"全局 ⌘1"（该指纹属命令面板作用域），本切片以面板作用域实现，并把"关闭时 ⌘1 完全惰性"写进测试钉死。

Vue 权威实现：`frontend/src/components/GlobalCommandPalette.vue`（868 行）+ `frontend/src/components/GlobalCommandPalette/ResultItem.vue`（202 行）。React 落地：`apps/web/src/platform/GlobalCommandPalette.tsx` + `command-palette.ts` + `command-palette.css`。未 commit，等待协调者独立复核集成。

## 一、Vue ⌘1-9 语义核对表（逐行）

| # | 语义点 | Vue 证据（行号） | 结论 |
|---|--------|------------------|------|
| 1 | 作用域：仅面板打开时 | `GlobalCommandPalette.vue:4`（`<div class="cmdk" @keydown="onKeyDown">`，位于 t-dialog 内部，`destroy-on-close`（:3）→ 关闭即卸载）；全局监听 `onGlobalKey`（:560-574）只认 ⌘K 与裸 `/`，不含数字 | ⌘1-9 是**对话框作用域**，焦点在对话框内才生效；关闭后按 ⌘1 无任何行为（浏览器默认，Vue 不拦截） |
| 2 | 修饰键 | `onKeyDown` :508-511：`(e.metaKey || e.ctrlKey) && e.key >= '1' && e.key <= '9'` | ⌘（mac）或 Ctrl（win/linux）+ 数字键；对 Shift/Alt **无显式检查**——但美式布局下 Shift+数字的 `e.key` 是 '!'/'@' 等符号、Alt 组合大多改变 `e.key`，被 :509 的字符串区间**天然排除**。字符串区间比较 '1'..'9' 恰好只匹配单个数字字符 |
| 3 | 数字 → 第 N 个**可见**项 | :515-516 `const n = parseInt(e.key,10); const item = flatItems.value[n-1]`；`flatItems`（:321-368）按 `groupOrder`（:279-289）展平：空查询 → `[recent…, commands…]`（最近搜索在前）；带查询 → `[chunks, messages, kbs, agents, sessions, commands]`；KB 芯片作用域 → `[chunks]` | 不是"仅 quick actions"——是全部可见结果的第 N 项（含最近搜索行）。React 本切片时代面板仅有 recent+commands 两组，⌘N 映射语义与 Vue 空态/命令组一致 |
| 4 | 越界行为 | :516-520 `const item = flatItems.value[n-1]; if (item) { e.preventDefault(); item.run({cmd:false}) }` | 越界 = **no-op 且不 preventDefault**（按键放行给浏览器）；`run({cmd:false})` 即"普通回车"语义，对 chunk 行不会触发 ⌘⏎ 的"发起对话"分支 |
| 5 | 与 ⌘↵ 的优先级 | :512-514 注释："digits take precedence over ⌘Enter: digits take precedence because ⌘+digit can't be confused with ⌘+enter" | 分支顺序 ArrowDown → ArrowUp → ⌘数字 → Enter → Escape；数字与 Enter 互斥，无实际冲突 |
| 6 | 徽标渲染条件 | `shortcutFor(flatIndex)` :496-499：flat index 0-8 → 返回 1-9，>8 → undefined；每个 ResultItem 都传 `:shortcut`（:40,47,59-60,79,95,102,111,121）；`ResultItem.vue:32-34`：`<span v-if="shortcut" class="cmdk-item__shortcut"><kbd>⌘</kbd><kbd>{{shortcut}}</kbd></span>` | **仅前 9 个可见项**显示徽标（不限分组——recent 行同样有徽标）；⌘ 字符是固定字形，Windows 上（Ctrl 触发）也显示 ⌘（Vue 如此，照搬） |
| 7 | 徽标视觉 | `ResultItem.vue:169-194`：10px、默认 opacity .55、选中/悬停行 opacity 1、kbd 小方块（bg secondarycontainer、1px 描边、圆角 3px） | 已按此移植进 `command-palette.css`（见下） |
| 8 | 底部提示条 | :146-148 `⌘1-9` hint，i18n `commandPalette.hotkey.cmdNumber`（zh: 直接打开 / en: Jump to 等） | **仍 deferred**（矩阵 note 单列）：React 面板本就没有 footer，且 `packages/i18n` 缺 `commandPalette.hotkey.select/enter/cmdEnter/cmdNumber` 4 键 × 5 语言 → 按"不自造文案"约束向协调者提交需求，不在本切片实现 |

## 二、React 实现（等价语义）

- `command-palette.ts` 新增纯函数 `paletteShortcutDigit(event)`：逐字镜像 :508-511 判定（`(metaKey||ctrlKey) + e.key '1'..'9'`），返回 1-9 或 undefined。注释明确约束：**只许接入面板对话框的 onKeyDown，严禁进 `decideGlobalShortcutAction`**（全局窗口监听必须保持 digit-free——上一切片被否决的行为）。
- `GlobalCommandPalette.tsx` `onKeyDown` 插入 ⌘数字分支（位于 ArrowUp 之后、Escape/Enter 之前，与 Vue 分支顺序一致）：
  - `index = digit - 1`；`index < recentCount` → 最近搜索行：`pickRecent(value)`（填入输入框、面板保持打开，镜像 Vue recent 项 `run()` → `query = q`，不导航不关闭）；
  - 否则命令项：`runCommand(command)`（有查询时 `onSearch` 记录最近 + `onClose` + `onNavigate`，即 Vue quick action 的 `router.push + closePalette` 等价）；
  - 越界：直接 return，**不 preventDefault**（镜像 Vue `if (item)`）；
  - 命中：`event.preventDefault()`（镜像 Vue）。
- 徽标：recent 与 command 两类行均经 `shortcutDigitFor(flatIndex)` 渲染 `<span className="cmdk__item-shortcut"><kbd>⌘</kbd><kbd>{digit}</kbd></span>`（flat index 0-8 才渲染）；顺带补齐 Vue `ResultItem.vue:5` 的 `data-cmdk-index` DOM 契约；行内标签包 `<span className="cmdk__item-label">`（flex:1 + 省略号）。
- `command-palette.css`：`.cmdk__item` 由 `display:block` 改 flex 行布局（对齐 Vue `ResultItem.vue:70-84` 的行结构），新增 `.cmdk__item-label` 与 `.cmdk__item-shortcut`（含选中/悬停 opacity 1）。
  - **归属说明（提请协调者知悉）**：任务给的 CSS 归属是"shell.css 中面板相关块"，但 shell.css 内**没有任何 cmdk 块**（仅一行提及注释）；面板样式自 S03 起独立在 `command-palette.css`（文件头注释言明"Kept in its own file so PlatformShell.tsx (which owns shell.css) is not the only place the palette can be styled from"），且该文件仅被本切片独占的 `GlobalCommandPalette.tsx` 导入。因此徽标样式落在 `command-palette.css`，未触碰 shell.css，也未触碰 chat/settings 任何块。
- i18n：徽标为纯 kbd 数字 + 固定 ⌘ 字形，无文案；未新增任何文案键（footer 提示条需求见上表 #8，待协调者裁决）。
- PlatformShell.tsx **零改动**（只读遵守）：全局监听本就只认 ⌘K 与 `/`（`decideGlobalShortcutAction` 返回 'none'），对话框作用域按键无需壳层配合。

## 三、TDD 证据（先红后绿）

RED（实现前）：

- `command-palette.test.ts`：新增 3 例（`paletteShortcutDigit` 判定/拒绝/`decideGlobalShortcutAction` digit-free 钉死）→ 模块加载即红（`does not provide an export named 'paletteShortcutDigit'`）。
- 新建 `global-command-palette-shortcuts.test.tsx`（jsdom + react-dom/client，11 例 a-k）：**12 tests / pass 1 / fail 11**（(j) 关闭惰性 pin 在旧代码上即绿，属预期——它钉的是"不得回归"而非新行为；其余 10 例行为 + 模块加载失败全红）。
  - 过程修正 2 处**测试侧**错误（实现语义始终对齐 Vue）：(b) 两条最近搜索时首条命令在 flat index 2（⌘3 而非 ⌘2）；(i) 查询 'o' 因 'knowledge'/'bot'/'org'/'config' 均含 o 命中全部 5 项，改用 'set'（仅命中目录第 5 位的 open-settings，断言其徽标为 ⌘1）。

GREEN（实现后）：

- 切片两文件：**34/34**（`command-palette.test.ts` 23 例 = 原 20 + 新 3；`global-command-palette-shortcuts.test.tsx` 11 例）。
- 全平台基线命令：`cd apps/web && npx tsx --test "src/platform/"*.test.tsx "src/platform/"*.test.ts` → **115/115 pass**（基线 101 + 新增 14：3 单元 + 11 行为），0 fail / 0 skipped。
- `pnpm run typecheck:web`：**通过，0 错误**。

## 四、防回归钉死（Round N+3 否决行为）

- 测试 (j)：面板关闭时对 window 派发 ⌘1 与 Ctrl+1 → `onNavigate/onClose/onSearch` 全零调用、对话框始终不存在、**⌘1 连面板都不许打开**（Vue `onGlobalKey` 只认 ⌘K 与 `/`）。
- 单元：`decideGlobalShortcutAction` 对 ⌘1/⌘9（open 无论真假、无论是否编辑态）恒 'none'——全局窗口监听层面永久 digit-free。
- 测试 (k)：命中项 preventDefault = true；越界 ⌘9 defaultPrevented = false（Vue 放行语义逐位对齐）。
- 测试 (e)：无修饰键裸数字绝不触发跳转（输入框正常打字）。
- 既有 `shell-sessions-header.test.tsx (d)`（他人所有，未改动）继续从壳层 seam 钉"⌘1 全局不导航"。

## 五、修改文件清单（本切片）

- `apps/web/src/platform/command-palette.ts`（+18：`paletteShortcutDigit`）
- `apps/web/src/platform/GlobalCommandPalette.tsx`（keydown 分支 + 徽标 + data-cmdk-index）
- `apps/web/src/platform/command-palette.css`（行布局 flex 化 + 徽标样式）
- `apps/web/src/platform/command-palette.test.ts`（+3 单元）
- `apps/web/src/platform/global-command-palette-shortcuts.test.tsx`（新建，11 例）
- 工作树中 chat/**、settings/**、styles.css、composer.tsx 等改动属并行切片（R016/settings），本切片未触碰。

## 六、移交协调者的开放项

1. **footer 键盘提示条**仍 deferred：需 `commandPalette.hotkey.select/enter/cmdEnter/cmdNumber` 4 键 × 5 语言 i18n 回填（Vue locale 文件有权威译文），批准后可另行小切片。
2. 未来 command-palette-search 接入分组结果（chunks/messages/kbs/agents/sessions）时，⌘N 的"第 N 可见项"语义会自动覆盖这些组（实现按 flat 顺序取项，与 Vue `flatItems` 同构），无需改快捷键层；届时建议为混合分组补一条跨组 ⌘N 行为测试。
3. 徽标 ⌘ 字形在非 Apple 平台也显示 ⌘（Vue 同款，Ctrl 触发）——如需按 `navigator.platform` 切换字形，属视觉裁决，待协调者定夺。
