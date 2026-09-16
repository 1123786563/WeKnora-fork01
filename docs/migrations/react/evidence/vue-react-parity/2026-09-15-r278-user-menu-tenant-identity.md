# R278 用户菜单多空间身份信息（2026-09-15）

按 Vue `UserMenu.vue` 的 `showTenantIdentityLine` 规则，React 在多空间或 superuser 场景显示当前空间名，以及用户名和本地化角色；单空间用户继续显示姓名和邮箱。

实现：
- 从 `auth.me()` 读取当前 tenant、memberships 数量和当前租户角色。
- 多空间场景显示“空间名 / 用户名 · 角色”。
- 角色使用共享 `tenantMember.role.*` 本地化词条。

验证：
- shell 多空间身份测试：5/5
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过
