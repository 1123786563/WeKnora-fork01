# R289 Trace Waterfall 树形节点

日期：2026-09-15

## Vue 基线

Vue `knowledge-processing-timeline.vue` 将 spans trace 展平为带深度的 waterfall 行，保留父子关系；非根节点支持展开/折叠，点击节点打开详情区，显示状态、耗时、身份和原始数据。

## React 实现

`packages/domain/src/knowledge/processing.ts` 新增 `flattenKnowledgeSpans`，以稳定 key 和 depth 保留后端 trace 树。React trace Sheet 现在同时显示五阶段摘要和可横向滚动的树形节点列表，支持展开/折叠、状态色、耗时、节点选中及原始 JSON 详情；刷新仍由 spans 轮询驱动，后端数据保持只读。

## 验证

- domain processing：7/7；
- Shared：470/470；
- 文档 page-chrome：14/14；
- Web 全量：910/910；
- `pnpm typecheck:web`、`pnpm typecheck:shared`：通过；
- `pnpm build:web`：通过，仅有既有大 chunk advisory；
- `git diff --check`：通过。

这是 domain/UI 回归、类型和构建证据；尚未完成真实后端 trace 树、浏览器节点展开/选中截图和 Vue waterfall 像素级比较。

## 未闭环

处理配置详情、失败重试/取消确认、真实后端/RBAC、响应式、Wails 与 React Native 原生验收仍待执行。
