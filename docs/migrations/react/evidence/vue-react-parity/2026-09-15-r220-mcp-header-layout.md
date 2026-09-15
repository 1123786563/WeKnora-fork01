# R220 MCP 抽屉标题区布局（2026-09-15）

## 修正

MCP 编辑抽屉标题区现在使用 Vue 对齐的固定头部结构：标题、传输/启用状态和两步导航集中在 104px 高的 sticky 区域；关闭按钮绝对定位到右上角；正文从头部下方开始滚动。桌面与移动抽屉均保留既定宽度和全高外壳。

## 验证

`pnpm exec tsx --test apps/web/src/settings/McpSettingsPanel.test.tsx`：13/13 通过，覆盖两步导航、字段顺序、关闭/保存操作和工具元数据步骤。

真实 MCP 同步、精确 computed-style、响应式截图以及 Wails/native 证据仍开放。
