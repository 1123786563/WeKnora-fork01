# TDesign React 同构迁移 Playbook（Phase 2/3 页面平移 SOP）

- **适用范围**：Phase 2 pilot（agents 页）与 Phase 3 全量页面迁移——把 `frontend/src/views/**` 的 Vue SFC 同构平移为 `apps/web/src/**` 的 React + tdesign-react 页面。
- **事实源**：本文件（SOP）；spec `docs/specs/2026-09-21-tdesign-react-migration-design.md`；执行历史 `.superpowers/sdd/2026-09-21-tdesign-react-migration/`。
- **总原则**：**Vue 端是唯一事实源**。DOM 结构、类名、样式值、组件 props 一律以 Vue SFC 原文为准逐项复刻；React 端现状（Tailwind utilities、`@weknora/ui`、parity 补丁 CSS）是待替换的旧栈中间态，不作为对照基准。
- **状态**：初版（T7）。标注「pilot 实证回填」的条目在 Phase 2 agents 页迁移时验证并修订本文。

## 前置条件（Phase 1 基建，已完成）

| 基建 | 内容 | 出处 |
|---|---|---|
| T1 依赖与样式管线 | tdesign-react@1.18.3 + tdesign-icons-react@0.6.11；`tdesign.css` unlayered 且最先加载 | `apps/web/src/styles.css:2`；commit `1018555b6` |
| T4 主题 token | Vue `theme.css` 平移至 `packages/design-tokens/src/tdesign-theme.css`，apps/web 侧 **unlayered** 直引（级联压过库默认值） | `apps/web/src/styles.css:7`；commits `ff7380052`/`a5f57ca46` |
| T5 图标守卫 + React 19 adapter | 图标离线守卫 + 本地 sprite（防 tdesign.gtimg.com 外联）；`react-19-adapter` 已在应用入口引入——**命令式 API（MessagePlugin/NotificationPlugin/DialogPlugin）全局可用，页面内无需再引 adapter** | `apps/web/src/main.tsx:22-26`；commit `3bdca449b` |
| T6 ConfigProvider locale | 五语言（zh-CN/en-US/ja-JP/ko-KR/ru-RU）对齐 Vue 端 | `apps/web/src/tdesign-locale.tsx`、`main.tsx:14`；commit `78655aff2` |

页面代码从 `tdesign-react` 具名导入组件（`import { Button, Dialog } from 'tdesign-react'`），图标从 `tdesign-icons-react` 导入（对应 Vue 端 `Icon as TIcon`，见 `frontend/src/views/agent/AgentList.vue:817`）。

---

## 1. 组件映射表

左列为旧栈 `@weknora/ui`（`packages/ui/src/index.tsx:4-28` 导出，Phase 4 删除物），右列为 tdesign-react 1.18.3 实际导出（行号均指 `node_modules/tdesign-react/es/index.js`）。**迁移时的直接对照物是 Vue SFC 里的 `t-*` 标签 props（第 3 节规则 3.3）；本表用于把旧 React 页面的 `@weknora/ui` 用法翻译到同一目标。**

| # | `@weknora/ui`（左侧） | 关键 API（packages/ui 实测） | tdesign-react（右侧） | 关键 API（1.18.3 实测） | 平移要点 / 例外 |
|---|---|---|---|---|---|
| 1 | `Button`（button.tsx:35-55） | `variant: default\|primary\|text\|danger`；`size: small\|medium\|large`；`loading`；原生 button attrs | `Button`（index.js L15） | `variant: base\|outline\|dashed\|text`；`theme: default\|primary\|danger\|warning\|success`；`size: small\|medium\|large`；`loading`；`icon`（TElement） | 旧栈单 `variant` 拆成 `theme`+`variant` 两轴：`primary`→`theme="primary" variant="base"`；`default`→`theme="default"`；`text/danger`→`variant="text"` + `theme="default"/"danger"`。Vue 端本来就写双轴，直译即可 |
| 2 | `Input`（input.tsx:6-14） | 原生 `<input>` 全 attrs + `invalid`（aria-invalid） | `Input`（L36） | `value/onChange`、`clearable`、`status`、`placeholder`、`maxlength`、`prefixIcon/suffixIcon` | 受控模型一致（`value`+`onChange`）。`invalid` → `status="error"`（同时影响 aria；以 pilot 实证回填）。DOM 从单 input 变为带 wrapper 的组件——**以 Vue 端 t-input DOM 为准，不保留旧栈单标签结构** |
| 3 | `Textarea`（textarea.tsx:6-8） | 原生 `<textarea>` attrs | `Textarea`（L68） | `value/onChange`、`autosize`、`maxlength/maxcharacter`、`disabled` | 同上 |
| 4 | `Select`（select.tsx:8-15） | **原生 `<select>`** 封装，children 为 `<option>` | `Select`（L54） | `value/onChange`、`options`/`children`、`multiple`、`filterable`、`clearable`、`loading` | DOM 完全不同（原生下拉 vs 弹层）。以 Vue 端 `t-select` DOM 为准；Vue 模板里 `:options` / `t-option` 子组件两种写法都直译（T2 实证 `:options`/`:list` 数组绑定照搬，task-2-report.md:31） |
| 5 | `Checkbox`（checkbox.tsx:9-11） | 原生 `<input type=checkbox>` attrs | `Checkbox`（L19） | `checked/onChange`、`label`、`indeterminate`、`disabled` | 原生事件 `onChange(e)` → `(checked, ctx)` 签名；label 从 children 移到 `label` prop 或 children 均可（以 Vue 端写法为准） |
| 6 | `Radio`（radio.tsx:5-7） | 原生 `<input type=radio>` attrs | `Radio` + `RadioGroup`（L51） | `Radio`: `checked/onChange/label/value`；`RadioGroup`: `value/onChange/options` | 分组必须用 `RadioGroup` 包裹才能还原 Vue `t-radio-group` DOM |
| 7 | `Switch`（switch.tsx:4-10） | `checked` / `onCheckedChange` | `Switch`（L63） | `value/defaultValue`、`onChange(value)`、`label`、`disabled`、`loading`、`size` | `onCheckedChange(v)` → `onChange(v)`。T2 实证 Vue 端布尔裸属性失效须 `:default-value="true"`（task-2-report.md:30）——React 侧无此坑，`defaultValue` 直译。根标签库间差异见台账 #2 |
| 8 | `Dialog`（dialog.tsx:4-13） | `open/title/children/onClose/closeLabel/className/portal`（自研 portal + focus 陷阱） | `Dialog`（L26；命令式 `DialogPlugin`/`dialog` 同行导出） | `visible`、`header`、`body/children`、`footer`、`cancelBtn/confirmBtn`、`closeBtn`、`onClose/onCancel/onConfirm`、`dialogClassName`、`preventScrollThrough` | `open`→`visible`、`title`→`header`。默认 footer 的取消按钮 variant 差异见台账 #1（显式传 `cancelBtn={{ variant: 'base' }}` 补齐）。Vue 端 `v-model:visible` + `dialogClassName` 直译（AgentList.vue:719-720 实例） |
| 9 | `Dropdown/DropdownTrigger/DropdownContent/DropdownItem/DropdownSeparator`（dropdown-menu.tsx:5-58，Radix 组合式；`Menu*` 为别名） | Radix Root/Trigger/Content/Item/Separator 组合 | `Dropdown`（L29） | 声明式 `options: DropdownOption[]` 为主（`content/disabled/divider` 等）；`trigger: hover\|click\|focus\|context-menu`、`placement`、`popupProps`、`panel Top/BottomContent`、`on.onClick` | 组合式→声明式为主：旧栈 Trigger+Content+Item 树对应 `options` 数组；Vue 端若用 `t-dropdown :options` 则逐字直译。自定义面板内容走 `popupProps` 或 children（以 pilot 实证回填）。弹层类名定制经 `popupProps.overlayClassName` |
| 10 | `Tabs/TabsList/TabsTrigger/TabsContent`（tabs.tsx:6-40，Radix） | `value`（受控）+ 每面板 `TabsContent value=` | `Tabs`（L65；`Tabs.TabPanel` 挂在组件上，tabs/index.d.ts） | `value/defaultValue`、`onChange(value)`、`list: TabPanelProps[]` 或 `<Tabs.TabPanel>` 子组件、`theme: normal\|card`、`placement`、`destroyOnHide` | `TabsList/TabsTrigger` 层在 tdesign 中由库生成（DOM 见台账 #5）；tab 项从 Trigger 子元素变成 `TabPanel label/value`。受控模型 `onValueChange` → `onChange` |
| 11 | `TooltipProvider/Tooltip/TooltipTrigger/TooltipContent`（tooltip.tsx:5-24，Radix） | 组合式；Provider 包裹 | `Tooltip`（L71） | `content`、`placement`（PopupPlacement 全方位）、`theme`、`showArrow`、`delay/duration`、`destroyOnClose` | 组合式→单组件：Trigger 子元素直接作 children，内容进 `content` prop。无需 Provider |
| 12 | `Table/TableHead/TableBody/TableRow/TableHeader/TableCell`（table.tsx:5-22，原生 `<table>` 拼装） | 原生表格子组件 + HTML attrs | `Table`（L64；同源导出 `BaseTable/PrimaryTable/EnhancedTable/SimpleTable`） | `columns`（ColDef 数组）、`data`、`rowKey`（必填）、`bordered`、`empty`、`loading`、`pagination` | 从"手拼 thead/tbody"转为 `columns+data` 声明式；单元格渲染经 `cell` 函数列。Vue 端 `t-table :data :columns` 直译。附加节点差异见台账 #3 |
| 13 | `Badge`（badge.tsx:21-23） | `tone: neutral\|primary\|success\|warning\|danger` 的 `<span>` | `Badge`（L13） | `count`、`dot`、`color`、`shape: circle\|round`、`maxCount`、`size` | 旧栈是"色块标签"语义，tdesign Badge 是"计数/圆点"语义——**若 Vue 端该处用 `t-tag`，则应映射到 `Tag` 而非 Badge**（右侧备选：`Tag`/`CheckTag`，index.js `./tag` 导出）。tone 换算：neutral→`theme="default"`、success→`theme="success"` 等（以 pilot 实证回填） |
| 14 | `Alert`（alert.tsx:22-24） | `tone: neutral\|info\|success\|warning\|danger`，`role="alert"` div | `Alert`（L8） | `theme: success\|info\|warning\|error`、`message`、`title`、`icon`、`close/closeBtn`、`operation`、`maxLine` | tone 换算：`neutral` 无直接对应（用 `theme="info"` 或保留原生 div，以 pilot 实证回填）；`danger`→`error`；正文进 `message` prop |
| 15 | `Sheet`（sheet.tsx:5-27） | `open/title/children/onClose/side: left\|right/width/resizable/headerIcon` | `Drawer`（L28；命令式 `DrawerPlugin`/`drawer`） | `visible`、`header`、`children/body`、`placement: left\|right\|top\|bottom`、`size`、`footer`、`confirmBtn/cancelBtn`、`closeBtn`、`onClose` | `open`→`visible`、`title`→`header`、`side`→`placement`、`width`→`size`。`resizable`/`storageKey`（自研拖宽）无对应——行为属旧栈增强，迁移时以 Vue 端对应抽屉（多为自研 div 或 `t-drawer`）为准 |
| 16 | `NumberInput`（number-input.tsx:5-26） | `value: number\|''`、`min/max/step`、`onValueChange`（自研步进按钮） | `InputNumber`（L38） | `value`、`onChange(value, ctx)`、`min/max/step`、`theme: normal\|column\|row`、`align`、`decimalPlaces` | `onValueChange`→`onChange`。Vue 端 `t-input-number :min :max :step` 直译；step 按钮形态默认 column（上下），与旧栈一致 |
| 17 | `Range`（range.tsx:5-7） | 原生 `<input type=range>` + `min/max/step` | `Slider`（L57） | `value/onChange`、`min/max/step`、`range`（双滑块）、`marks`、`showStep`、`layout`、`tooltipProps` | 原生 range → tdesign 滑轨 DOM；Vue 端 `t-slider` props 直译 |
| 附 | `Label`（label.tsx:4-6） | 原生 `<label>` 样式封装 | 无 TDesign 对应 | — | 保留原生 `<label>`（或对照 Vue 端：若 Vue 用 `t-label` 不存在，则 Vue 端也是原生标签 + 类名，直译） |
| 附 | `Separator`（separator.tsx:4-6） | `role="separator"` div | `Divider`（L27） | `layout/align/content` | 仅当 Vue 端用 `t-divider` 时映射；否则保留原生 div + 类名 |
| 附 | `Card`/`Status`（index.tsx:31-37） | 语义 div/p 封装 | 无 TDesign 对应 | — | 保留原生标签 + Vue 端类名（样式走第 2 节平移 CSS） |

**16 组件之外、Vue SFC 中出现的其他 `t-*` 组件**同样按导出名直译（右侧均已在 `tdesign-react/es/index.js` 核实）：`t-popup`→`Popup`（L48）、`t-tag`→`Tag`（`./tag` 行）、`t-skeleton`→`Skeleton`、`t-loading`→`Loading`、`t-icon`→`tdesign-icons-react` 的 `Icon`、`MessagePlugin/NotifyPlugin`→`MessagePlugin/NotificationPlugin`（React 导出名，T2 实证 task-2-report.md:29）。

---

## 2. 样式平移规则

目标：Vue SFC `<style>` 块 → React 侧每页一个普通 CSS 文件。**类名不改**——Vue 端类名即事实源，且 `wk-*` hook 类是 apps/web 现有测试的查询锚点（spec §5「测试 hook」，design 文档 :62）。

以真实样例 `frontend/src/views/agent/AgentList.vue` 说明（该 SFC 含两个 style 块：scoped 块 :1612-2544、unscoped 块 :2546-2681）。

### 2.1 提取与落位

1. 每个 SFC 的 `<style>` 块全部提取到页面同目录的 `<Page>.td.css`（pilot 定稿：`apps/web/src/agents/agents.td.css`，页面 tsx 顶部 `import './agents.td.css'`）。弹层 portal 到 body 的组件样式（Dialog 内容、unscoped 块）也在同一文件，以弹层类名限定作用域。
2. `<style scoped lang="less">` 与 `<style lang="less">`（无 scoped）**分开处理**，见 2.2 / 2.5。

### 2.2 scoped → 显式根类前缀

Vue scoped 编译为每选择器追加 `[data-v-hash]` 属性限定；React 侧无此机制，改用**该 SFC 根元素类名**作前缀限定，作用域语义等价：

- 根元素自身的规则不加前缀：`.agent-list-container { … }`（AgentList.vue:1613）原样平移（类名本身已全局唯一）。
- 其余选择器挂到根类下：`.header .header-title` → `.agent-list-container .header .header-title`。
- 前缀必须逐级保留 Vue 源码的嵌套路径，不得只挂根类丢失中间层级（特异性变化会改写覆盖顺序）。

### 2.3 less 嵌套展平

把嵌套块展开为完整选择器链，`&` 拼接还原：

```
Vue（AgentList.vue:1663-1690）              → React CSS
.header {                                   .agent-list-container .header { … }
  …                                         .agent-list-container .header .header-title { … }
  .header-title { … }                       .agent-list-container .header .title-row { … }
  .title-row { … }                          .agent-list-container .header h2 { … }
  h2 { … }
}
```

- `&:hover` / `&.is-favorited` / `&::before` → 独立规则 `.agent-list-container .agent-favorite-star:hover { … }` 等（`&::before` 实例见 AgentList.vue:1725）。
- `:deep(x)` → 去掉 `:deep` 包装，保留其在嵌套中的位置：`:deep(.agent-create-btn)`（:1692）→ `.agent-list-container .agent-create-btn`。穿透语义由"不再有 scoped 属性"天然获得；块内 `!important` 与 `--td-*` 变量覆盖**原样保留**（如 :1710-1713 的 `--td-button-primary-*` 覆盖）。
- less 注释 `//` → CSS 注释 `/* */`；less 变量/颜色函数（darken 等）如遇到，求值为字面量后写入（本仓库 SFC 目前只有 `//` 注释用法，AgentList.vue:1628 实测）。

### 2.4 类名与变量

- **类名不改**：像素扫描以渲染结果为准，而页面 CSS 的选择器锚点就是 Vue 端类名；改类名 = 平移失真。
- **`wk-*` hook 类必须保留在 DOM 上**：apps/web 现有测试以其为查询 hook（spec :62「只换样式实现不删 hook」）。若 Vue 端模板本身没有 `wk-*` 而旧 React 页面有（如 `wk-agent-section-header`），保留该类名并让平移 CSS 命中它。
- `--td-*` / `--app-font-family` 变量原样平移（token 已由 T4 unlayered 接线，`apps/web/src/styles.css:7`）。
- `@keyframes` 随页面 CSS 平移，**动画名保留 Vue 端原名**（scoped 编译不改 keyframes 名；AgentList.vue:1746 的 `twinkle`）。旧栈 agents.css 里的改名版（`wk-agent-twinkle`，agents.css:18-20 注释）在迁移该页时废弃。

### 2.5 unscoped 块（服务 portal 到 body 的弹层）

无 scoped 的块（AgentList.vue:2546 起）通常服务于**渲染到 body 的弹层 DOM**（Dialog/Drawer/Popup 内容类，如 `.del-agent-dialog`、`.shared-detail-drawer-*`）：

- 类名本身已带页面语义前缀的，**原样平移、不加根类前缀**（scoped 前缀反而会让规则命不中 body 下的 portal DOM）。
- 若类名有全局冲突风险（无页面前缀的通用名），平移时保留在一个文件但在规则上方注释"来自 <Page>.vue unscoped 块，服务于 body 弹层"。

### 2.6 布局值归并（pilot 实证补充）

- **行高继承差**：React 端页面渲染在 PlatformShell 内（继承 21px 行高），Vue 端页面直挂 #app（body `line-height: normal`）。页面根（`.agent-list-container`）与 body 弹层根（`.settings-overlay`）需显式 `line-height: normal` 对齐 Vue 继承链，否则 nav/label 全线 +1px/行漂移。
- **Vue Teleport ↔ React createPortal**：弹层组件（编辑器 overlay）用 `createPortal(…, document.body)` 复刻 Vue `<Teleport to="body">` 的层叠/合成上下文，脱离 shell 的行高与渲染上下文（pilot 实证）。
- **共享组件的散装规则**：Vue `theme.css` 除 token 外还有全局组件规则（如 t-radio-button 选中态品牌色块）；React 侧 tdesign-theme.css 只平移了 token。页面迁移时把这类规则按页面作用域平移（agents.td.css §9），Phase 4 归全局。
- **未分层 `button { font-family: Arial }` 全局规则**（apps/web 侧遗留，批次 3 会再踩）：所有原生 `button` 的 computed font-family 是 Arial 而非 `--app-font-family`——Vue 端对应组件（如 Input-field.vue `.control-btn`）以 `font: inherit` 显式归位。迁移含文字/内联 svg 的按钮时必须补 `[font-family:inherit]`（svg 的基线对齐也随字体走，Arial 会造成亚像素位移）；creatchat composer 实证（task-11b §4.3）：agent chip 箭头因 Arial 基线 18px 偏移、chip 文本字形相位差，补 inherit 后归 0。Phase 4 清理候选：把该全局规则改为 `font-family: inherit`。

### 2.6 布局值归并（原文）

React 页面 tsx 中由 Tailwind utilities 承载的布局/视觉值，**不逐条翻译**——按第 4 节删除；页面布局以 Vue 端 style 块值为准写入本 CSS。Vue 端没有的值（React 多出的间距等）一律不保留。

---

## 3. DOM 复刻规则

React JSX 逐节点对照 Vue template：**标签、类名顺序、条件渲染分支、inline style、属性**逐项一致。

### 3.1 逐节点对照

- 标签名：原生元素（div/span/button/svg…）原样；`t-*` 组件按第 1 节映射换成 tdesign-react 组件（Vue 端 DOM 由库生成，React 端同名组件自然对齐）。
- 类名顺序：静态 `class` 在前、`:class` 绑定按对象键序追加，与 Vue 输出一致。例 AgentList.vue:156-161：`class="agent-card"` + `:class="{ 'is-builtin': …, 'agent-mode-normal': … }"` → `className={cn('agent-card', agent.is_builtin && 'is-builtin', …)}`。
- inline style 逐字复刻，包括 `--wails-draggable: drag` 这类自定义属性（AgentList.vue:7-10）。
- data-* / aria-* / title / tabindex 全部保留（`data-guide="agent-list-create"`，AgentList.vue:13；`role="button" tabindex="0"`，:76-77）。

### 3.2 控制流翻译

- `v-if / v-else-if / v-else` → JSX 三元/`&&`，分支顺序与模板一致（骨架屏分支 AgentList.vue:44 → `{(loading && agents.length === 0) ? <SkeletonCards/> : …}`）。
- `v-for` → `.map()`，`key` 保持原 `:key` 表达式（如 `agent.isMine ? agent.id : \`shared-${agent.share_id}\``，:67-68）。
- `v-show` → **保留 DOM + 隐藏**（Vue v-show 元素始终在 DOM，仅 display:none）；React 侧用 `style={{ display: hidden ? 'none' : undefined }}` 等价实现（:156 `v-show="!isAgentRowHidden(agent)"`）。
- `<Transition name="…">` → 保留同名 CSS 过渡类（平移 CSS 含对应 enter/leave 规则）；React 侧用条件渲染 + CSS animation 或保持结构等价（以 pilot 实证回填）。

### 3.3 t-* 组件 props 直译

- props 同名直译：`<t-button variant="text" theme="default" size="small">`（AgentList.vue:12）→ `<Button variant="text" theme="default" size="small">`。
- kebab-case 属性 → camelCase props：`:close-btn="false"` → `closeBtn={false}`、`dialog-class-name` → `dialogClassName`、`overlay-class-name` → `overlayClassName`（AgentList.vue:199 `t-popup` 实例）。
- 具名 slot → props/children：`<template #icon>` → `icon={<span class="btn-icon-wrapper">…</span>}`；popup 的 `<template #content>` → `content={<div class="popup-menu">…}</div>`（:206-222）；默认插槽 → children。
- 事件：`@click` → `onClick`；`@click.stop` → onClick 内 `e.stopPropagation()`；`@visible-change` → `onVisibleChange`；`@update:visible` 的回写逻辑并进 `onVisibleChange`。
- `v-model` / `v-model:visible="x"` → 受控 props + 回调：`visible={x}` + `onClose`/`onVisibleChange` 回写 state（t-dialog 实例 AgentList.vue:719）。
- 修饰符 `.prevent/.enter`：`@keydown.enter.prevent` → onKeyDown 中判 `e.key === 'Enter'` 后 `e.preventDefault()`（:78）。

### 3.4 库间差异处理

第 6 节台账列出的 DOM 差异是**库自身行为，页面代码无法消除**：有补齐方式的（如 Dialog cancelBtn variant）按台账执行；无补齐方式的（Switch 根标签等）不要在页面代码里塞补偿 DOM/hack 掩盖——那会造成新的 DOM 差异。台账外新发现的差异按第 6 节流程登记。

---

## 4. 删除规则（每页迁移时同步执行）

1. **删该页 tsx 中全部 Tailwind utility 类**：className 里的 utilities（`flex`、`px-4`、`text-[13px]`…）清零；其承载的布局/视觉值已由第 2 节平移 CSS 以 Vue 端值为准覆盖。删除后该页 tsx 不再出现 utility 类（`cn()` 里只剩 Vue 端语义类名与条件类）。
2. **删该页 `@weknora/ui` 引用**：`import { … } from '@weknora/ui'`（现状如 AgentsPage.tsx:3）整行移除，组件换 tdesign-react。已迁页面**禁止新增**旧栈引用（Phase 4 前旧栈仅对未迁页面可用）。
3. **删/并该页专属 parity 补丁 CSS**：`apps/web/src/agents/agents.css` 中 agents 相关段删除或并入平移 CSS。**共享规则例外**：`.wk-agent-section-header`/`.wk-agent-section-count` 同时被 `apps/web/src/configuration/ConfigurationPage.tsx` 消费（agents.css:5-8 头注「CSS retention criteria」），按既定保留准则保留，待 configuration 域迁移时清理（T9/T15 分段，见 sdd 预检冲突表 #6）。
4. **删页面级临时像素补丁**：为对像素加的 margin/transform hack、一次性 overlayClassName 补丁样式，一律以 Vue 端源样式重写替代后删除。
5. 删除物不越界：只动该页拥有的文件/段；全局样式（styles.css、design-tokens）属 Phase 4 清理，页面任务不碰。

---

## 5. 每页验证 SOP

一页一循环，全部通过才算该页完成（spec §7 Phase 2 SOP，design 文档 :91）：

1. **测试绿**：`pnpm --filter @weknora/web test`（apps/web/package.json `test` script：`node --import tsx --test 'src/**/*.test.ts' 'src/**/*.test.tsx'`；当前基线 2293+ 用例全绿）。`wk-*` hook 保留应使既有测试查询选择器无需改动；确需动测试时随本页同一提交。
2. **类型门禁**：`pnpm --filter @weknora/web exec tsc -b`（基线已由 T1 恢复全绿）。
3. **单页扫描**：仓库根执行 `PAGES=<id> node scripts/parity/auto-scan.mjs`。
   - `<id>` 取扫描清单 id（auto-scan.mjs:73 起 ALL_PAGES；如 `agents`、`kb-list`、`chat`、`ix-agents-create`），逗号分隔可多页。
   - 前置：Vue dev server `:5174`、React dev server `:5175`、后端 `:8084` 在线（可用 `PARITY_VUE_URL/PARITY_REACT_URL/PARITY_BACKEND` 覆盖）；凭据文件 `~/.weknora-parity-creds.env`。
   - 输出：`docs/migrations/react/evidence/vue-react-parity/auto-scan/<timestamp>/report.md`（像素对比，容差 8）。
4. **收敛标准**：该页**所有**相关扫描项（页面项 + 该页的 `ix-*` 交互项，如 agents 页的 `ix-agents-create`）diff **0.00%**。台账内已知库间差异按第 6 节豁免/补齐，不算页面不达标；除豁免外任何残差都必须归因到具体规则并修到 0。
5. **一页一提交**：格式 `feat(parity): <page-id> 页 TDesign 平移（PAGES=<id> 全项 0.00%）`。提交内容 = 该页 tsx 重写 + 平移 CSS + 旧样式/`@weknora/ui` 引用删除 + 相关测试调整；不掺其他页改动。

---

## 6. DOM 差异台账（tdesign-react 1.18.3 ↔ tdesign-vue-next 1.20.7）

格式：`| 组件 | react DOM/默认值 | vue-next DOM/默认值 | 补齐方式 |`。空行位次供 Phase 2 pilot 起逐条回填；登记流程：发现差异 → 归因（库行为 or 页面代码）→ 能补齐的写补齐方式并验证像素 → 库行为且不可消除的标记「扫描豁免」并在扫描解读时引用本表行号。

首批条目（Phase 0 实证，出处：task-3-report.md:30 / task-2-report.md:40-46）：

| # | 组件 | react DOM/默认值 | vue-next DOM/默认值 | 补齐方式 |
|---|---|---|---|---|
| 1 | Dialog 默认 footer 取消按钮 | `variant: "outline"` 白底（`tdesign-react/es/dialog/DialogCard.js:163`） | `theme: "default"` + Button 默认 `variant: "base"` 灰底（`tdesign-vue-next/es/dialog/hooks/useAction.mjs:67`、`es/button/props.mjs:78`） | 页面侧显式传 `cancelBtn={{ variant: 'base' }}` 补齐；残差 60×32px≈0.199%（task-3-report.md:30，R5 已裁决记入本台账） |
| 2 | Switch 根元素 | `<button role="switch" class="t-switch">` | `<div class="t-switch">` | 库行为不可消除；样式等价零视觉差异，扫描豁免（task-2-report.md:42） |
| 3 | Table 附加节点 | 无 | 多空 `.t-table__pagination-wrap` + `display:none` 的 `.t-table__resize-line` | 库行为不可消除；零占位，扫描豁免（task-2-report.md:43） |
| 4 | Dialog 关闭态挂载 | 不渲染 DOM | 挂载 `display:none` 的 `.t-dialog__ctx` | 库行为不可消除；不可见，扫描豁免（task-2-report.md:44） |
| 5 | Tabs 面板容器 | `.t-tabs__content.t-is-top` 包裹层存在 | `.t-tab-panel` 直挂 | 库行为不可消除；pilot 重点关注面板区间距/下边线（task-2-report.md:46，以 pilot 实证回填） |
| 6 | Button class 属性顺序 | `--theme-primary --variant-base` | `--variant-base --theme-primary` | 无需补齐：class 顺序不影响渲染与选择器匹配；禁止页面代码硬编码完整 class 串断言顺序（task-2-report.md:45） |
| 7 | Button disabled 根标签 | 无 tag 且 disabled 时渲染 `<div class="t-button t-is-disabled">` | 始终渲染 `<button disabled>` | 库行为；样式经 `.t-is-disabled` 类等价（测试断言走 classList 而非 .disabled 属性）；扫描零视觉差（pilot 实证） |
| 8 | Select 根元素属性透传 | 根 div 只取 className/style/onMouseEnter/Leave，`data-*` 等其余 props 丢弃 | t-select 未知 attrs 透传到根 | React 侧测试钩子改用语义 `className`（如 `.wk-ae-sel-model`）；InputNumber 经 `inputProps` 送到内层 t-input wrapper；Option 的额外 props 也不透传（li 无 title/data-*），弹层断言走 textContent（pilot 实证）。**S5 评审 Minor 补注（Tabs 同族）**：Tabs 同样只取 className/style（`tabs/Tabs.js` 根 div，TabNav/TabNavItem 根亦不透传）——aria-label 无法经 props 落到 t-tabs__nav（tablist 可访问名称）；analytics 页已注记（AnalyticsPage.tsx），如需 a11y 命名须库升级或 DOM 层挂载 |
| 9 | Select children 中的 null 项 | 内部按数组遍历 children 建立 value→option 映射，遇 `null` 项直接 crash（`handlerElement` 读 `null.type`） | 容忍 null | React 侧条件 OptionGroup 必须用数组展开拼装（`[...(cond ? [<OptionGroup/>] : [])]`），不得写 `cond ? <OptionGroup/> : null`（pilot 实证） |
| 10 | svg-sprite Icon 的 glyph 版本 | 本地 sprite（守卫拦 CDN）；`<use xlink:href>` | 运行时 svg-sprite Icon 实际加载 CDN 0.4.5，注入脚本按 body firstChild 前插使其 `use` 命中 0.4.5 变体 glyph | React 端 index.html 本地镜像 0.4.5 并只保留该版本（多版本并存时后插入者居首、先被 `use` 命中）；两端 glyph 需同版本（pilot 实证：chat 等 glyph 0.4.1↔0.4.5 几何不同）。**S3 评审补注（核查项 3，六枚图标逐字节抽验证实）**：同名图标存在两版 d——tdesign-icons 组件树内联版（高精度小数 d）与 sprite 版（3 位小数相对命令）几何不同；Vue 端 t-icon 实际几何以本地 sprite（frontend/public/tdesign-icons/0.4.1）的 symbol stroke path d 为准，且 linecap 逐 path butt/square 混合——迁移取材一律用 sprite 版 d，勿用组件树版 |
| 11 | label 必填星号的亚像素相位 | — | `{{ label }} <span class="required">*</span>`：空格是文本节点、星号独占 span | 空格放 span 内会使星号 glyph 落在不同亚像素相位（LCD AA 权重 ±5 灰阶，逐像素超容差 8）；React 侧 JSX 须 `{label}{' '}<span className="required">*</span>`（pilot 实证） |
| 12 | Input/Textarea 的 maxlength 原生属性 | JS 截断（`limitUnicodeMaxLength`），textarea/input 元素**不带** maxlength 属性 | 渲染原生 `maxlength` 属性（KnowledgeBaseEditorModal DOM 实测） | 行为等价（两端输入都截到上限），无视觉差异、无需补齐；测试断言走计数器（`.t-textarea__limit`）或 JS 截断行为，不依赖原生属性（kb-list 实证） |
| 13 | Textarea 计数器文本节点切分 | `.t-textarea__limit` 由 `renderLimitText` 渲染为**两个文本节点**（`"0"` + `"/500"`，`tdesign-react/es/textarea/Textarea.js:240-243`），跨节点字偶 kern 丢失 → 计数内后续字形步进比 vue-next 窄 1/64-3/64px | 单次插值单文本节点 `"0/500"`，整串 shaping | **可用 `count` render-prop 单文本节点复刻**（`type.d.ts:28-33`：`count?: boolean \| ((ctx:{value;count;maxLength?;maxCharacter?})=>TNode)`；函数分支经 parseTNode 整体替换默认 span）——页面传 `count={({count,maxLength})=><span className="t-textarea__limit">{\`${count}/${maxLength}\`}</span>}`，单模板字符串 child＝单文本节点，与 vue-next（`textarea.mjs:308-310` 单串 child）DOM 同构。orgs ix-orgs-create 实证：默认渲染残差 35px≈0.004%（单枚 12px 字形，分数 x 左缘时 AA 放大），加 count 后 **0.00%**。凡迁移页 tdesign Textarea 带 maxlength 计数器一律照此传 count（批次 2/3 处置先例） |
| 14 | Table maxHeight 固定表头启用时机 | 有 `maxHeight` 即拆分固定表头（`thead.t-table__header--fixed`），th 涂 `--td-bg-color-secondarycontainer` 灰底 | 仅当内容高度溢出 maxHeight 才启用固定表头；空态/短表 th 继承容器白底 | 页面侧条件传参对齐：空数据不传 `maxHeight`（apps 实证 R1 9.6%→R2 0%，灰 th 带 47×972 全带差异）；有数据行数时传回（两端此时均启用固定表头，灰底一致）。两端 CSS 规则文本相同（`.t-table__header--fixed:not(...) > tr > th`），差异在 JS 启用判定 |
| 15 | PopupPlacement 方位粒度 | `PopupPlacement = top/left/right/bottom/top-left/top-right/bottom-left/bottom-right/left-top/left-bottom/right-top/right-bottom`（无 `-start/-end` 后缀，popup/type.d.ts:134） | 含 `bottom-start`/`bottom-end` 等 `-start/-end` 粒度（Vue UserProfile.vue 密码弹层 / EnvVarSettings.vue hint 弹层 / MemorySettings.vue usage 弹层均用） | 库差异无 seam：React 侧以 `bottom-right`/`bottom-left` 近似（同为边缘对齐语义）；仅弹层开启态几何有影响，稳态扫描不可见。settings 域三处实证（task-12a fix-1） |
| 16 | Input autoWidth 宽度测量取整 | `calcWidth = width < offsetWidth ? offsetWidth + 1 : width`——分数文本宽恒走 `offsetWidth+1` 上取整（input/Input.js updateInputWidth；pagination 页大小 select 实测 input 47px） | 使用分数文本宽（同文本 `20 条/页` input 45.84px） | 库差异无 seam（测量分支内嵌组件）。传导：t-pagination 页大小 select 宽 +1.156px，由 flex 的 total 吸收（页码/跳至不位移）；残余=选框左缘+input 文字位移 x797-852（~377px/条）与 pager+跳至尾字 8px 切线，两表共 ~772px（settings-members 复审 fix-2 终值 0.084% 仅此一项，task-12a 实证）。fix-2 勘误：初版误记「页码/跳至整体左移」并把成员行高/share-link 空格列为伴生差——前者实为遗留硬编码行高 CSS（settings-wrapper.css 旧原生表 slice，已删）、后者为 CSS 平移缺漏（.share-link-title 未移植），均已页面级修复。亚像素家族注记（glyph AA 相位/权重类残差，扫描环境专属）——**fix round 勘误（T12b 评审 Important-1）**：初版把 settings-models 的 28px 残差记到 builtin-hint doc-link link 图标（x1038-1049/y83-94）并对其取证，元素指认错误，整套取证链跑错目标；doc-link 区域（x370-515/y192-204）复核 **0 差异**。实际超容差像素全数落在 models header 右上「模型测试」t-button（`.model-test-trigger`，#icon slot）的 **play-circle 图标**描边边缘（x1038-1049/y83-94）。对正确元素重取证（evidence `auto-scan/2026-09-22T10-28-36/headed-modeltest-*.png`，本地 gitignore 目录）：①双端按钮几何逐项一致（x1035/y73.1953125、82×32）；②`t-icon-play-circle` symbol 于 CDN 0.4.5（Vue 运行时源）与本地镜像逐字节一致——glyph 几何同构，排除资产差；③28px 超差像素沿描边边缘对称分布（y83/y94 镜像行对），vue 侧 AA 阶梯恒浅一级——AA 权重差定性；④headed 系统 Chrome（GPU 光栅化）元素截图对照：残差 28px→8px，仍全在图标 AA 边缘、幅值恰为一级阶梯位移（131,224,175/193,239,215/255,255,255 三级间错位），真机不可见（task-12b 实证 + fix round 复核） |
| 17 | viewer 角色面板门控语义 | React 部分已迁面板（如 SkillSettingsPanel）对 viewer 渲染只读回退分支（Card 列表/Status） | Vue Settings.vue 仅对 admin+ 挂载 SkillSettings 等组件；viewer 走 role-denied 块，组件不挂载 | 语义微差仅在无权限角色路径可达，parity 扫描以 admin 登录不可达；登记不改（T12b 评审 Minor-2；如需对齐应在 SettingsPage 门控层统一处理而非各面板） |
| 18 | sprite `<use>` 图标斜边 AA 单级差 | 双端 `t-icon-chevron-down` symbol 字节级一致（innerHTML 104B 同）、元素盒/字号一致（12px@429,123.1） | 同左 | 像素级归因：settings-integration-im/embed 各 14px——embed x431-438/y127-130、im x423-430/y127-130，沿 chevron 左斜边，恒一级灰阶差（219↔204，Δ15）；headed 系统 Chrome（GPU 光栅化）元素截图 14→12px 仍在同斜边同单级，真机不可见（headed 工件为临时 /tmp 截图未持久化入库，复验可按 task-12c 流程重跑）；库级定位：tdesign-icons sprite 经 `<use>` 12px 视口光栅化的 stroke 亚像素相位（Vue 端 CDN 脚本运行时注入——DOM symbol 计数 4703 为运行时全集计数、镜像文件实际 2356 枚，React index.html 静态镜像同 2356，symbol 内容逐字节相同、注入载体不同）；无页面 seam（inline width/height 移除、1em css、几何对齐均验证无效）——扫描豁免（T12c task-12c 实证，im/embed 各 0.002%；S1 评审勘误：编号恢复 #17 后顺序、坐标补 im、4703 注记澄清） |
| 19 | Dialog 弹窗圆角弧线 AA 阶梯错位 | 弹窗四角 border-radius 12px，弧线 AA 阶梯逐角错位（ix-kb-settings 残 0.003%≈31px） | 同一几何 | 引擎栅格伪影，扫描豁免留档（S3 评审 Important-2）。取证：①双端弹窗盒逐边一致（x140/1140，y54/666）；②computed 逐属性一致；③逐端自比 0px；④排除试验（半径 11-13/双弧线/clip-path 均不动差值）；S3 评审独立复算证实。同族 hack 挂注（可追溯）：KnowledgeSettingsPage.css `.kb-type-tab.is-checked` 的 2.5px 圆角像素补偿（:649-650）与 .wkbs-modal radius 12 双弧线叠合补丁（KnowledgeSettingsPage.tsx:973-975，机制详见 documents-list.css 注）同属本弧线栅格相位差家族的页面级对策（kb-settings 实证 a5da455a9） |
| 20 | 图标 stroke 亚像素宽光栅相位差 | fullscreen 图标 24 单位 viewBox ×0.583 缩放致 stroke 1.167px 亚像素宽，光栅相位差（ix-kb-doc-detail 残 0.005%≈47px） | 同一几何 | 引擎栅格伪影，扫描豁免留档（S3 评审 Important-2）。取证：双端同位同尺寸 14×14 + d 串与 sprite 逐字节一致（bff725eda 实证） |
| 21 | Form 最后一个表单项 margin | FormItem 自动加 `.t-form__item--last`，库 CSS 将其 margin 清零 | 不加该类；同名 CSS 靠 `:last-of-type` 兜底——登录/注册表单后还有 button/.register-cta 等 div 兄弟，`:last-of-type` 落不到表单项上，全部表单项保持 24px | 页面作用域中和（login.td.css：`.form-content .t-form__item--last { margin-bottom: var(--td-comp-margin-xxl) }`，等值恢复库默认；auth 页实证：React 密码项 0↔Vue 24px→修复后双端 24） |
| 22 | `<img>` 缩放光栅相位 | 共享图层直接绘制：下采样相位与独立图层不一致 | 独立合成图层（fixed 等）高质量光栅 | 资产/几何/`drawImage` 重采样逐字节一致仍差 ~154px（AA 阶梯）时，用 `opacity:.9999 + scale(1.0000001)` 提升为独立图层对齐光栅路径（login logo 实证 154px→0，3 次复跑稳定） |
| 23 | craft 域 tdesign 手写模拟层（td.tsx） | `packages/views/src/craft/td.tsx` 手写 Button/Drawer/Dialog 输出 t-* DOM（真实 tdesign-react 1.18.3 逐行核对），宿主 tdesign.css 全局加载下还原视觉 | 组件库直挂 | **模拟层登记（S5 评审 Minor，参照 chat/message-face.tsx t-button 同构判例口径）**：packages/views 刻意不带 tdesign-react 依赖（package.json 由并行流共享）；与真实组件的既知差异——无 CSSTransition 进场动画类、无 body portal（fixed 树内渲染）、关闭钮附加 role="button"+aria-label、CloseIcon 为 icons 0.6.11 close.js SVG 逐属性复刻。治理口径：**禁止扩散**（craft 域新弹层优先真实组件）；packages/views 获得 tdesign-react 依赖时删除本文件整体切换 |
| 24 | 会话行「更多」菜单「分享」项 | `session-sidebar.tsx` 渲染 6 项（置顶/修改标题/分享/清空消息/批量管理/删除记录，onShareSession 能力开关=canShareSessions 常真） | `menu.vue` buildSessionMenuOptions 固定 5 项（无分享；分享类能力在 chat header 菜单=复制对话链接） | **SP13 spec 强制的 React-only 功能豁免（panel-matrix Phase II Batch-1 实证）**：spec docs/specs/2026-09-19-onyx-platform-parity-design.md §SP13 前端明文「会话侧栏 session-sidebar.tsx 菜单加‘分享’项+分享弹窗」（commit e22b1a659）；像素级：菜单同位 (81,272) 同宽 162，React 高 219 vs Vue 188=多一枚 32px「分享」行，px-shell-session-more 恒 1.166% 即此一项。实施 Plan 与批准 Spec 冲突以 Spec 为准（AGENTS.md 冲突规则），不改码，扫描解读引用本行 |
| 25 | composer 附件按钮 tooltip 开启态图标 AA 相位 | `.wk-attach-tip` in-tree 手写 tooltip（同 wk-kb-tip 家族） | `Input-field.vue:2737` t-tooltip body portal | **引擎栅格伪影豁免（#16/#18/#22 亚像素家族，px-chat-attach-tooltip 终值 0.001%≈8px）**：像素级——8px 全部落在 tooltip 下方回形针/@ 图标 y666-667 描边 AA（Δ≤15 灰阶、双向、墨量守恒），tooltip 本体/悬停底色/图标几何与路径逐项核对一致，静态 chat（无 tooltip）同图标 0 差；hover+tooltip 开启态才出现（Vue body-portal 浮层改变其下图标光栅相位 vs React in-tree 不改变）。3 次复跑恒 8px 确定性；无页面 seam（图标已独立 svg、几何对齐验证无效） |
| 26 | headless 新插入元素 :hover 重算不一致（指针工件） | 点击触发点 (682,396) 落进双端弹层内容区；React 端插入后不重算 hover 链，Vue 端重算（Vue 上传区带 hover 品牌边框/底色，React 无） | 同左（两库对 headless hover 重算时机不同） | **扫描口径判例（px-kb-wiki-tab-wiki-newpage 先例、px-skills-add 复现）**：auto-scan 页项加 `mouseAway: true`——双端把指针移 (4,4) 清 hover 后截图；判据=触发点坐标落进双端面板几何 + 两端 hover 规则逐字相同 + 移走指针后双端 0%。不要在页面代码里删 hover 样式迁就扫描 |
| 27 | 交互项差异全量来自静态分区底差（弹层同构） | 弹层几何/文案逐字对齐 | 同左 | **静态底差转交判例（px-userprofile-change-password/px-ollama-redetect/px-members-rbac-hint 三项实证）**：判据=①交互项终值与对应 settings-{section} 静态基线同值；②像素级验证弹层 rect 内 diff=0/3/5px（AA 级）；③双端弹层 DOM 几何逐字段一致（popconfirm 566,228,320×132 / role-hint 620,118,360×120）。处置=交互面板项判同构，底差归静态页工作单池（与 settings-general 5.439/settings-integration-api 4.284 存量同性质），不在交互批次重复计欠账 ||   | （pilot 回填区） | | | |

| 28 | Button `#icon` 槽 vs icon children；Select 触发器内联 fake-arrow + 字体栈 | t-button `#icon` 槽 → 独立 `.t-button__icon`（margin-right 8px，按钮宽 98）；t-select 触发器显示文本是 `.t-input__inner` input（继承全站字体栈 `-apple-system,"system-ui",...` + line-height 22px），箭头是内联 `svg.t-fake-arrow`（16×16 viewBox 0 0 16 16，`M3.75 5.7998L7.99274 10.0425L12.2361 5.79921` stroke black .9 width 1.3，非 t-icon glyph） | tdesign-react Button 把 icon children 并入 `.t-button__text`（无 8px margin，按钮宽 90、svg y 上偏 3px）；自研 trigger 若挂 `<button>` 会拿 UA 字体栈 Arial | **icon 槽判例（px-models-model-card 残差 72px 实证）**：①icon 必须走 `icon={...}` prop（等价 Vue `#icon` 槽），不得作 children——否则丢 `.t-button__icon` margin 且 svg 落入 text span 基线偏移；②自研 select 触发器文本 span 须显式 `font-family: inherit` + `line-height: 22px`（+1px 补偿）对齐 input 渲染；③fake-arrow svg 逐属性内联复刻（含 stroke-opacity 0.9）。执行=ModelOptionSelect.tsx chevron + Model/Mcp 面板 add-header/generate-usage 按钮（19-08-35 轮三项 0.00% 实证） |
|   | （pilot 回填区） | | | |

### 已知跨域欠账（非页面平移差异，登记待归属域修复）

- **`packages/views/src/chat/markdown` 缺 Vue `repairFlankingEmphasis`**：Vue chat 渲染管线在 marked 前跑标点符尾强调修复（frontend/src/utils/chatMarkdownRenderer.ts:283-331；关键规则 FLANKING_ITALIC `(?<![*\p{L}\p{N}])\*(?=\S)([^*\n]*?\p{P})\*(?=[\p{L}\p{N}])` 会命中 `***x***` 的第 2 星——`*` 本身是 \p{P}），`***加粗斜体***` 在 Vue 端实际渲染 `<em><em></em>加粗斜体</em>**`（首 `**` 被吞、尾 `**` 字面量泄漏）；React 共享渲染器渲染标准 `<em><strong>X</strong></em>`——chat 生产路径（message-list / artifact-preview / message-face 等消费方）两端 `***x***` 渲染不同。技术约束：React 端 `renderer.html` 转义原始 HTML（allowlist 仅 `<kbd>`），源级预注入 `<em>` 会被转义（dev-markdown 实测 0.272%→0.414% 反例），需 html allowlist 扩展或 token 级变换。修复归属 chat 域（S4 已在 DevMarkdownPage 用终态 HTML 后置改写规避页面扫描，生产欠账仍在）。
