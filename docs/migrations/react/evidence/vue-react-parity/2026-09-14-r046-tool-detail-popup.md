# R046 · MCP 工具详情浮层

- 日期：2026-09-14
- Vue 基线：`frontend/src/views/settings/components/McpToolsList.vue:17-81` 使用 attach-to-body 的 popup；详情包含描述、参数、完整定义三个标签页，并在打开新工具/翻页时重置。
- React：`apps/web/src/settings/McpToolsDirectory.tsx` 与 `apps/web/src/styles.css`。

## 已验证

- 工具详情改为独立 dialog 浮层样式，保留三个详情标签页、参数 required 标记、空态文案和策略开关。
- 增加 Escape 关闭；搜索/分页仍重置打开项；stale 和 policy busy 门控未改变。
- `typecheck:web` 通过，Web 全量 674/674 通过。

## 未覆盖

- 尚未在认证浏览器与真实同步工具目录中复核 Portal/computed-style；可达 MCP 服务与 Wails 证据仍缺失。
