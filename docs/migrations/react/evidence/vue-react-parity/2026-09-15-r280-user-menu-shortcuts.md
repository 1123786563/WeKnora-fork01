# R280 用户菜单设置快捷入口（2026-09-15）

按 Vue `UserMenu.vue` 的账号菜单结构，React 用户菜单补齐：

- `设置 > 空间信息`：进入 `/platform/settings?section=tenant`
- 具备 owner/admin 或跨空间权限时显示 `成员管理`：进入 `/platform/settings?section=members`
- 保留个人设置、新手引导和退出入口。

入口复用现有本地化词条与 React Settings 路由，成员入口继续受现有角色门禁控制。

验证：
- shell 聚焦测试：8/8
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过
