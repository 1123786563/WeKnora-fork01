# R231 文档页面表面层对齐（2026-09-15）

React 文档页原先使用共享 `Card` 包裹整个内容区，额外生成圆角、边框和内边距，与 Vue 文档页的平面内容表面不一致。现改为专用 `wk-documents-surface` 容器，保留文档筛选栏、上传入口、列表和状态结构，移除多余的 shadcn Card chrome。

验证：

- 文档页面 chrome 测试：9/9
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过

受保护文档数据、上传真实流程、响应式、多语言浏览器和桌面/native 证据仍待补齐。
