# R288 Trace Sheet 回归

日期：2026-09-15

## 变更范围

R287 的共享 `Sheet` 可调宽度能力已由文档页 trace 面板启用：右侧 Portal、560–1400px 限制、鼠标拖拽、拖拽期间 body 光标/选择锁定和 localStorage 宽度恢复。其他 Sheet 默认不启用该行为。

## 验证

- `packages/ui/src/interaction.test.tsx`：2/2；
- `pnpm test:shared`：469/469；
- 文档 page-chrome：14/14；
- `pnpm typecheck:web`：通过；
- `pnpm build:web`：通过，仅有既有大 chunk advisory；
- `pnpm test:web`：910/910；
- `git diff --check`：通过。

这些是共享组件、静态渲染、单元/回归、类型和构建证据。真实浏览器下的拖拽、localStorage 恢复、Portal 层级和 Vue 同条件截图尚未执行。

## 未闭环

Vue 完整 waterfall 节点树与选中详情、处理配置、失败重试/取消确认、真实后端/RBAC、响应式视口、Wails 与 React Native 原生验收仍待执行。
