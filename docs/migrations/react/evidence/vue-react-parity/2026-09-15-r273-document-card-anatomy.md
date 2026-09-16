# R273 文档卡片内容与底部元信息区对齐（2026-09-15）

按 Vue `DocumentCardView.vue` 的 `.knowledge-card` computed 样式，React 文档卡片补齐 `240px` 最小宽度、`136px` 固定高度、`8px` 圆角、`10px 14px 8px` 内容内边距及独立 `32px` 底部元信息区。已保留文档打开、选择、标签和解析操作；文件夹路径在底部区域保持可点击导航。

验证：
- 文档 page-chrome：11/11
- `pnpm test:web`：904/904，0 失败、0 取消、0 跳过
- `pnpm typecheck:web`：通过
- `pnpm build:web`：通过；仅有既有大 chunk advisory
- `git diff --check`：通过

边界：本项为静态渲染与组件级交互结构验证；完整 Vue 卡片菜单、hover 详情浮层、真实非空后端操作、响应式截图及 Wails/native 仍未闭环。
