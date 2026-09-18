# 2026-09-16 Round R431 — 四项新差异修复 + 登录态双端浏览器配对取证(修复后)

- 环境：React `:5181`、Vue `:5180`（vite dev，热更新含本轮修复）、真实后端 docker `:8080`；parity 账号 tenant 10000 owner；视口 1440×900、浅色、简体中文。
- 证据目录：`screenshots/r431-20260916/`（react-kb-guide.png、react-kb-graph.png、vue-kb-graph.png、react-kb-graph-arrows-zoom.png）。
- 修复内容见 progress Round R431 与四份代理汇报；差异基线为 `2026-09-16-r430-browser-parity-pair.md` 新发现差异 1-4。
- fixture 说明：wiki KB `7cea6ec0-8a07-4c83-9309-f802a61b3d5c` 在本轮为 4 节点（新增 index 页，来源为共用后端的其他使用方），双端同数据源同条件。

## 四项差异的修复与验证（基线=Vue）

| # | 差异 | 修复 | 浏览器验证 |
|---|---|---|---|
| 1 | 图谱 tab 头部信息架构：Vue 为 KB 内嵌面包屑+文档/Wiki/图谱 tab 行+ⓘ/⚙+副标题；React 为独立页头 | 页头复用 DocumentsBreadcrumb（知识库 > KB 名 > 三 tab，图谱 tab 绿色高亮+`aria-current`+tabGraphTip tooltip；ⓘ 信息弹层、⚙ 走 React 既有 `/knowledgeBase/{id}/settings`），副标题无条件渲染，删除自创大标题与澄清段落 | `react-kb-graph.png` vs `vue-kb-graph.png`：头部结构一致 |
| 2 | 图谱搜索控件形态：Vue 为 filterable t-select（search 前缀图标+chevron 后缀）；React 为普通输入框 | 重构为 select 形态组合框外壳（32px/圆角 4px/td-shadow-1 阴影/chevron 展开旋转），空关键词回落节点快照面板（对齐 `graphSearchEffectiveOptions`），补 combobox aria 与键盘导航 | `react-kb-graph.png`：控件形态与 Vue 一致 |
| 3 | 同条件下 React 边无方向箭头 | 根因：边线圆心直连，节点(半径 8-24+白描边)绘制其上完全遮盖 marker。新增 `graphEdgeEndpoints()` 逐行复刻 Vue `setEdgePositions` 端点收缩（radius+4），marker fill 改直接属性，开关图标改眼形 SVG | `react-kb-graph.png` 整页可见箭头；`react-kb-graph-arrows-zoom.png` 放大确认三角箭头 |
| 4 | Vue 一次性 KB 引导浮层 React 未触发 | React 引导 UI 本已完整（packages/views/src/guides 三步 catalog+Spotlight），缺详情页进入触发：新增 `shouldArmKbDetailGuideOnEntry`（逐项对齐 KnowledgeBase.vue:339-345：非 FAQ/可编辑/加载完成/空库）+ `useKbDetailGuideTrigger` hook + `data-guide="kb-detail-add-doc"` 目标 | `react-kb-guide.png`：进入 fixture KB 即弹出 1/3"知识库还是空的"，跳过/下一步与 Vue 一致（Vue 侧基线为 r430 证据 `vue-kb-graph.png` 中的浮层） |

## 过程中发现并修复的回归

- **图谱画布窄居中列**（本轮 W1 引入，浏览器实测发现）：`main.wk-page` 的 `mx-auto` 在 flex-column 容器中禁用 stretch，页面 shrink-to-fit，W1 缩短副标题后画布缩到 ~544px。修复后为通栏 flex 布局（实测 canvas 1114×798，viewBox 动态同步），与 Vue `.wiki-main-area` 通栏一致。最终形态见 `react-kb-graph.png`。

## 已核查非问题

- 引导 intro 卡片"左上"位置：几何代码与 Vue `SpotlightGuide.vue` 逐字符同源（无 target 时视口居中），早期截图中偏位系采集时视口为 600×420 的状态所致，非回归。

## 记录的剩余偏差（R432 候选，均不阻塞本轮验收）

1. ⚙ 行为：Vue 打开全局设置抽屉（uiStore.openKBSettings），React 导航到既有 KB 设置页——surface 形态不同，落点等价。
2. tab 行门控：Vue 严格按 isWiki；React 用 `resolveKBSurfaceTabs`（wiki 关但 graph 开的库显示 文档/图谱 两 tab）。
3. 搜索框输入仍联动过滤画布节点（React 既有行为，Vue 不联动）；防抖 250ms/2 字符 vs Vue 200ms/任意字符。
4. Vue 选中/hover 边高亮交互（applyHighlight/箭头变色）React 未实现。
5. 引导触发在 React 挂在文档页组件，直接深链 `?tab=graph` 不触发（Vue tab 无关）；引导一次性完成后实际影响有限。
6. tab 切换 React 为链接导航（三个独立页面），Vue 为同组件内切换。

## 依赖标注

- 图谱页头依赖 `DocumentsPageChrome.tsx` 的 DocumentsBreadcrumb `tabs` 插槽（并发编排进程新增，已随 `6139859a` 提交）；若其回滚该改动，图谱页头编译即破。
- 本轮四切片代码由并发进程随 `6139859a` 提交（连同其 SPA navigation/KB 页改造）；画布布局定稿 delta 由本轮回后单独提交。

## 边界

- 未修改移动端；未修改 Vue；并发进程的 docs(sdd) 提交与本轮无关。
