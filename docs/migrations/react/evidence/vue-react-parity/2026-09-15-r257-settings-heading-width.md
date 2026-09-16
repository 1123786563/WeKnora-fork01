# R257 设置标题容器宽度（2026-09-15）

Vue 的空间信息 section 标题节点占满 760px 内容列，React 原先只有标题文本宽度 176px。标题内部容器已设为全宽，使标题和说明在同一内容边界内布局，并保持 Vue 的 20px/600 typography。

验证：
- React 浏览器复测：标题宽度 760px，内部标题容器宽度 760px
- 设置聚焦测试：16/16
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过
