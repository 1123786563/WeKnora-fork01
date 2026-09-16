# R286 Trace 右侧抽屉几何

日期：2026-09-15

## Vue 基线

Vue `doc-content.vue` 将处理时间线挂载到 body 的右侧二级 drawer，默认宽度 820px，最小宽度 560px，遮罩层级高于普通弹窗，并支持拖拽调整及本地持久化宽度。

## React 实现

React 文档页将 trace 面板从居中 Dialog 调整为项目 `@weknora/ui` 的 Radix/shadcn `Sheet`，使用右侧 Portal、遮罩、`min(820px, 92vw)` 宽度、边框和滑入动效；保留 Escape、焦点恢复和关闭回调，继续使用 spans 阶段状态与轮询逻辑。

## 验证

- 文档专项：14/14；
- Web 全量：910/910；
- `pnpm typecheck:web`：通过；
- `pnpm build:web`：通过，仅有既有大 chunk advisory；
- `git diff --check`：通过。

这是组件结构、静态渲染、回归、类型和构建证据；尚未完成真实浏览器下 Portal/遮罩/焦点恢复的 trace 交互截图。

## 未闭环

尚未实现 Vue 的拖拽宽度调整、宽度持久化、完整 waterfall 节点详情、处理配置信息、失败重试和取消确认。真实后端、RBAC、响应式、Wails 与 React Native 原生验收仍待执行。
