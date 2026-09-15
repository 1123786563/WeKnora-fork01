# R261 成员管理标题行几何对齐（2026-09-15）

同认证用户、同空间、zh-CN 和相同视口下，React 成员管理标题行原先使用浏览器 inline-flex 行盒，导致标题行高 35px、标题向下偏移 5px，审计按钮也使用默认 35px 按钮样式。Vue 基线为 25px 标题行，审计按钮 86×24px。

修正内容：
- 标题行和标题包裹改为 flex 布局，并将审计按钮移到右侧 sibling。
- 标题固定 20px/600/25px、字距 -0.4px、无 margin。
- 审计按钮固定 86×24px、3px 圆角、0/7px 内边距、16px 图标。

浏览器复测：
- React 标题：`x=383,y=52,w=78,h=25`
- React 标题行：`x=383,y=52,w=799,h=25`
- React 审计按钮：`86×24px`、`border-radius:3px`、`padding:0 7px`
- Vue 标题与标题行同为 `y=52,h=25`，审计按钮为 `86×24px`

验证：
- TenantMembersPanel 聚焦测试：9/9
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过
