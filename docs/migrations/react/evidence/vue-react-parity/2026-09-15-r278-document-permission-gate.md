# R278 文档卡片与批量选择权限门控（2026-09-15）

按 Vue `DocumentCardView` / `DocumentListView` 的 `canEdit` 条件，React 只在 `canContribute` 时显示网格/列表文档选择框、全选和批量工具栏；只读用户仍可查看文档、文件夹和 hover 详情，不再看到可操作的选择控件。

验证：
- 文档 page-chrome：13/13
- 只读卡片 SSR：选择框不渲染
- `pnpm test:web`：906/906，0 失败、0 取消、0 跳过
- `pnpm typecheck:web`、`pnpm build:web`、`git diff --check`：通过

边界：当前证据覆盖权限门控和组件回归；受保护真实账号下的 viewer/contributor 双端浏览器截图及后端 RBAC 仍待补齐。
