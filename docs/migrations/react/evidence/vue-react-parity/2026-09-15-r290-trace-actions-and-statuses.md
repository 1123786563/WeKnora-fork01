# R290 Trace 操作与状态兼容

日期：2026-09-15

## Vue 基线

Vue 时间线在解析中提供取消确认，在失败状态提供重试，顶部提供手动刷新，并展示服务端最后错误；重建继续进入带处理配置的确认流程。节点状态兼容 running、success/completed、failed 等后端变体。

## React 实现

React trace Sheet 现在提供手动刷新、失败重建、解析中取消确认和最后错误提示。失败重建复用现有 `reparseOne` 与 UploadConfirm 状态，取消复用 `cancelParse` API 与项目 Dialog。树节点状态映射补齐 `success`、`finish`、`ok` 等服务端状态，避免成功节点误显示为等待。

## 验证

- 文档 page-chrome：14/14；
- domain processing：7/7；
- Web 全量：910/910；
- `pnpm typecheck:web`：通过；
- `pnpm build:web`：通过，仅有既有大 chunk advisory；
- `git diff --check`：通过。

这是组件/回归、类型和构建证据；真实后端失败/重试/取消确认、浏览器交互截图和多语言运行时尚未验证。

## 未闭环

Vue 完整 waterfall 时间轴、处理配置摘要、重试/取消的真实后端流程、RBAC、响应式、Wails 与 React Native 原生验收仍待执行。
