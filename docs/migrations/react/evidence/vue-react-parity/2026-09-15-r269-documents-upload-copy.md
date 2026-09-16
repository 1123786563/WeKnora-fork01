# R269 文档上传入口图标与 fallback 文案（2026-09-15）

文档页上传来源菜单、文件夹选择器、URL 列表和新建文件夹入口改用项目内联 SVG 图标；拖放提示复用 Vue-derived `knowledgeBase.emptyKnowledgeDragDrop`，上传错误 fallback 复用 `common.error`，不再向用户暴露 Unicode/Emoji 或硬编码英文。

验证：
- 文档 page-chrome：10/10
- `pnpm test:web`：903/903，0 失败、0 取消、0 跳过
- `pnpm build:web`：通过；仅有既有大 chunk advisory
- `pnpm typecheck:web`：通过
- `git diff --check`：通过

边界：本项验证了组件渲染、文案解析和构建回归；真实非空知识库上传、provider/backend 错误响应、响应式浏览器矩阵及 Wails/native 仍未作为本项通过条件。
