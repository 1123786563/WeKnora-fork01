# R043 · MCP 服务端说明弹层

- 日期：2026-09-14
- Vue 基线：`frontend/src/views/settings/components/McpMetadataPanel.vue:20-67`；存在 `instructions/server_description` 时显示点击弹层，不存在时显示帮助提示。
- React：`apps/web/src/settings/McpSettingsPanel.tsx` 与 `apps/web/src/styles.css`。

## 已验证

- 元数据摘要增加服务端说明触发器；弹层显示服务端 description 与 initialize instructions，并支持再次点击收起。
- 无服务端说明时显示 `noServerDocumentation` 帮助按钮；同步/刷新、工具策略和 stale gating 保持不变。
- MCP 专测 11/11、`typecheck:web` 通过。

## 未覆盖

- 尚未在认证浏览器中用真实 MCP snapshot 对比 Portal/computed-style；真实同步状态仍依赖可达 MCP 服务，Wails 证据缺失。
