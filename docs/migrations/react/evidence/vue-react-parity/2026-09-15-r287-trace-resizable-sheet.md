# R287 Trace 抽屉可调宽度与持久化

日期：2026-09-15

## Vue 基线

Vue `doc-content.vue` 的 trace drawer 默认 820px、最小 560px，最大值受视口约束；左边缘拖拽改变宽度，并将宽度保存到 `weknora-trace-drawer-width`。拖拽期间锁定 body 光标和文本选择。

## React 实现

`packages/ui/src/sheet.tsx` 新增可选 `resizable`、`minWidth`、`maxWidth` 和 `storageKey` 接口，默认关闭，不改变其他 Sheet。trace 面板启用右侧拖拽手柄、560–1400px 限制、`weknora-trace-drawer-width` 持久化，并在拖拽期间设置 `col-resize` 与禁止文本选择；宽度逻辑仍由 Tailwind/shadcn 项目 Sheet 承载。

## 验证

- UI interaction：2/2；
- Shared：469/469；
- 文档 page-chrome：14/14；
- Web typecheck：通过；
- Web build：通过，仅有既有大 chunk advisory；
- `git diff --check`：通过。

当前证据为组件/静态与回归验证，尚未有真实浏览器下拖拽、localStorage 恢复及 Vue/React 同视口截图证据。

## 未闭环

完整 waterfall 节点详情、处理配置、失败重试/取消确认、真实 trace 数据、RBAC、响应式、Wails 与 React Native 原生验收仍待执行。
