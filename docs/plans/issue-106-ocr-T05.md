Review complete: 1 finding(s) across 1 selected item(s).

─── internal/router/routes_plugins.go:32-35 ───
[documentation · low] 注释与实际行为不一致：(1) "Ops-only" —— 本代码库没有独立的 Ops 角色，该路由实际由租户级
Admin()（RequireRole(TenantRoleAdmin)，见 internal/router/rbac.go:203）放行，任何空间管理员都能调用；(2) "removes a
failed confirm's leftover rows" —— handler 与 service（plugin_install_service.go
UninstallInstallation）均未校验安装状态，可对任意状态的安装做硬级联删除，而不仅是清理 confirm
失败的残留行。建议改写为与实际授权和影响面一致的表述，避免后续维护者据此注释误判该端点的安全边界（例如："self-heal/ops 补偿入口：任意空间管理员可调用，硬删任意安装并释放
(tenant, plugin) 槽位；用户面治理入口仍是 disable，Web UI 不得暴露卸载操作"）。

- 		// Ops-only self-heal channel (rulings.md R4): removes a failed
- 		// confirm's leftover rows. NOT a user-facing feature — the
- 		// user-visible governance endpoint stays "disable" and the web UI
- 		// must never surface a delete/uninstall entry.
+ 		// Self-heal/ops remediation channel (rulings.md R4): gated at tenant
+ 		// Admin and hard-deletes ANY installation regardless of state —
+ 		// intended for cleaning a failed confirm's leftover rows. NOT a
+ 		// user-facing feature: the governance endpoint stays "disable" and
+ 		// the web UI must never surface a delete/uninstall entry.

