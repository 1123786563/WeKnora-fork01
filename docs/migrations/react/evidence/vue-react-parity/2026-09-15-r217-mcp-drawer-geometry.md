# R217 MCP 抽屉几何一致性（2026-09-15）

## 修正

MCP 服务编辑器现在使用 Vue 同款右侧抽屉外壳：桌面端宽度 `min(680px, 100%)`、全视口高度、无圆角、右侧贴边；移动端宽度 100%，同样占满视口。遮罩改为右侧抽屉布局且取消额外内边距。

## 验证

- MCP focused tests：13/13，通过 `mcp-editor-overlay` 结构断言。
- Web typecheck、Web 896/896 回归均通过。
- CSS 仅作用于带 `wks-mcp-overlay` / `wks-mcp-drawer` 的 MCP 编辑器，不影响其他设置弹窗。

真实 MCP 服务交互、响应式截图和 Wails/native 证据仍开放。
