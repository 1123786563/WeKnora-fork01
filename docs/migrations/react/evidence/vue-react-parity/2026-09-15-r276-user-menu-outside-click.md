# R276 用户菜单外部点击关闭（2026-09-15）

Vue `UserMenu.vue` 使用 document click listener，在菜单外部点击时关闭下拉菜单。React 原先只处理菜单项点击，点击页面其他区域后菜单会继续保持打开。

修正：
- React `PlatformShell` 为用户菜单容器增加 ref。
- 菜单打开期间注册 document click listener。
- 点击容器内部保留菜单状态，点击外部自动关闭并清理 listener。

Chrome 实测：
- 点击用户菜单后，AX 树出现 `新手引导 / 个人设置 / 退出` 菜单项。
- 点击页面空白区域后，AX 树移除菜单项，用户菜单回到 collapsed 状态。

验证：
- PlatformShell 聚焦测试：25/25
- Web typecheck：通过
- `git diff --check`：通过
