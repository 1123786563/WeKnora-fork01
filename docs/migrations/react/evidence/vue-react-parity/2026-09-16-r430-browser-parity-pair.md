# 2026-09-16 Round R430 — 并行切片修复 + 登录态双端浏览器配对取证(修复后)

- 环境：React `:5181`、Vue `:5180`（vite dev，热更新含本轮修复）、真实后端 docker `:8080`；parity 账号 tenant 10000 owner；双端同视口 1440×900、浅色、简体中文。
- 证据目录：`screenshots/r430-20260916/`（12 张，react-*/vue-* 六对）。
- 修复内容见 progress Round R430 与五份代理汇报；本文件只记浏览器实测结论。

## 六对截图与结论（基线=Vue）

| 对 | 文件 | 结论 |
|---|---|---|
| KB 列表(含侧栏会话) | `react-kb-list.png` / `vue-kb-list.png` | **修复确认**：侧栏"我的对话"渲染"昨天"分组 + 知识库检索讨论/Wails 验收会话 B/A，无 "Loading..." 残留；R428 差异 1 关闭。 |
| MCP 设置空态 | `react-settings-mcp.png` / `vue-settings-mcp.png` | **修复确认**：owner 账号空态渲染虚线"添加服务"tile（+图标+文案），与 Vue 一致；R428 差异 2 关闭。 |
| 技能管理 | `react-settings-skills.png` / `vue-settings-skills.png` | **修复确认**：标题旁 ？ 帮助图标、插画 + "暂无数据"主行 + 两行说明 + 添加技能/去配置沙箱按钮，层级与 Vue 一致；R428 差异 3(skills) 关闭。 |
| 成员审计抽屉 | `react-members-audit-drawer.png` / `vue-members-audit-drawer.png` | **修复确认**：插画 + "暂无数据" + "暂无审计事件。"，头部与信息条一致；R428 差异 3(audit) 关闭。 |
| KB 创建对话框 | `react-kb-create-dialog.png` / `vue-kb-create-dialog.png` | **修复确认**：类型选择器为拼接单线边框组（文档绿底选中/问答）、Wiki 知识库卡带 NEW 徽标、描述右下 0/200 计数器；R428 差异 4 关闭。 |
| 图谱 canvas(wiki fixture) | `react-kb-graph.png` / `vue-kb-graph.png` | **fixture 解除**：wiki 型 KB `7cea6ec0-8a07-4c83-9309-f802a61b3d5c`（3 页面互链，graph 3 节点/5 边）双端同条件渲染：节点三色（摘要蓝/实体绿/概念橙）、图例五项、适应屏幕/隐藏箭头、全局概览 3/3 节点、搜索+? 帮助均一致；R428 `blocked-fixture` 解除。 |

## 新发现差异（留待 R431，N012 域）

1. **图谱 tab 头部结构**：Vue 是 KB 页内嵌 tab（面包屑 知识库 > Wiki图谱fixture + 文档/Wiki/图谱 tab 行 + ⓘ/⚙ 图标 + 上传说明副标题）；React `?tab=graph` 是独立页头（"知识库 · <kb-id>" + 大标题"知识图谱" + 概念澄清说明文案）。画布本体一致，页头信息架构不同。
2. **搜索控件形态**：Vue 搜索框带下拉箭头（select 形态）；React 为普通输入框。
3. **边箭头可见性**：Vue 同条件可见有向箭头；React 截图中边为无箭头线（需复核 React 箭头渲染/默认开关初值）。

以上均为本轮浏览器实测新发现，未在本轮修复（超出五切片范围），以截图为证。

## 交互验证（真实后端）

- 双端登录态下完成：设置域四个面板切换、审计抽屉开合（Esc/关闭按钮）、创建对话框打开、图谱 tab 加载；均无报错、无卡死。
- Vue 侧首次进入图谱有一次 1/3 引导浮层（跳过后消失），React 无对应浮层——确认 Vue 该引导为 KB 首次访问一次性引导，React 未实现（记 R431 候选，N004/N012 域）。

## 边界

- 未修改移动端；未修改 Vue；`docs/superpowers/plans/mobile-workbench-progress.md` 为外部并发进程改动，未纳入本轮。
