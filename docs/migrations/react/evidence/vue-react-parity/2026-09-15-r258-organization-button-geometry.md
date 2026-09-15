# R258 共享空间操作按钮几何对齐（2026-09-15）

同认证用户、共享空间空状态、zh-CN 和相同视口下，React 创建按钮原先为 140px 宽、6px 圆角、16px 内边距，并保留 1px 透明边框和 6px 图标间隙；Vue 实测为 130px × 32px、3px 圆角、0/15px 内边距、无边框和无额外间隙。

React 的共享空间按钮常量已按 Vue 基线修正，加入/创建空状态按钮及组织设置中的同类主按钮复用该值。

验证：
- React 浏览器复测：`130px × 32px`、`border-radius: 3px`、`padding: 0 15px`、`border-width: 0`、`gap: 0`
- Vue：`130px × 32px`、`border-radius: 3px`、`padding: 0 15px`
- 共享空间聚焦测试：14/14
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过
