# R252 — React Native shared-layer regression

日期：2026-09-15

## 结果

- `pnpm test:mobile`: 190/190 passed。
- `pnpm typecheck:mobile`: passed。
- 覆盖认证/租户切换、聊天流恢复与停止、知识库/数据源、收藏/最近、上传进度、文档详情和多语言文案。

## 边界

这是 React Native 共享层测试与类型检查，不代表 DOM/shadcn 组件被复用到 Native，也不代表 iOS/Android 原生编译、安装、启动和真机逐页视觉/交互已通过。测试输出仍有 React DOM 渲染器对 Native-only props 的既有 warning；本项未修改移动端实现。
