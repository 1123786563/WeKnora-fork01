# R031 · Sandbox inventory drawer

- 日期：2026-09-14
- Vue 基线：`frontend/src/views/settings/SandboxSettings.vue:140-206` 使用不可调整宽度的 400px SettingDrawer，遮罩外点击关闭；会话标题加载完成前不显示原始 ID，行可导航到聊天。
- React：`apps/web/src/settings/SandboxSettingsPanel.tsx` 与 `apps/web/src/settings/sandbox-settings.css`。

## 已验证

- inventory 从内嵌 Card 改为右侧 400px drawer overlay，保留遮罩关闭、取消关闭、会话标题去重/失败回退、会话行导航和无 `onOpenSession` 时的 inert 行。
- Sandbox 专测 28/28 通过；既有编辑器、校验、删除冲突和 inventory 行为均回归通过。

## 未覆盖

- 当前仍缺认证浏览器 computed-style、真实后端 inventory、Wails/native 证据；R031 仍为 `implementing`。
