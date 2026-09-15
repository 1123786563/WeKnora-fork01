# r097 Vue 设置重复挂载修复验收

## 根因与修复

`frontend/src/views/platform/index.vue` 同时渲染了平台级全局 `Settings` 和 `/platform/settings` 子路由的 `Settings`，造成设置抽屉重复出现。修复为：当当前路由名为 `settings` 时不挂载全局实例，由子路由实例单独承载设置页面；其他平台子路由继续使用全局实例。

## 浏览器证据

- Vue：`http://localhost:5173/platform/settings?section=mcp`
- 刷新后 DOM 查询结果：`.settings-overlay = 1`、`.settings-modal = 1`。
- 可访问性树中仅出现一组“关闭设置”、设置导航和“MCP 服务管理”。
- Vue 类型检查 `pnpm type-check` 通过。

## 结论

重复设置树已消除，设置 MCP 深链接保持可用。该项完成 Vue 基准结构修复；React 与 Vue 的完整视觉、角色和原生端验收仍按矩阵继续执行。
