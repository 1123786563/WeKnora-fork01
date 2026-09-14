# R014 平台关键路由 AX 复核（2026-09-15）

## 环境

- React：`http://localhost:5181`，Chrome 已认证 parity owner，会话只读。
- Vue：`http://localhost:5173` 当前回到 `/login`，无法建立同账号双端视觉/computed-style 对照。

## React 结果

- `/platform/knowledge-bases`：侧栏显示“知识库”分组；正文包含“全部/收藏/最近/本空间”筛选、计数、新建知识库、空态提示。
- `/platform/agents`：正文包含“全部/收藏/最近/本空间”筛选及计数、创建智能体、内置智能体分组；四个内置智能体条目和管理入口可达。
- 共享侧栏在两条路由均保留“新对话/知识库/智能体/共享空间”、我的对话和用户菜单入口。

## 判定

- 关键平台路由在已认证 React 浏览器中可达，主要导航、筛选、空态和列表入口均有 AX 证据。
- 同条件 Vue 像素/computed-style 对照、真实创建/编辑写路径仍为 `blocked-env`，不以本记录替代双端验收。
