# R044 · MCP 编辑态凭证卡片

- 日期：2026-09-14
- Vue 基线：`frontend/src/views/settings/components/McpServiceDialog.vue:244-275`；编辑模式由 CredentialResource 管理独立 `/credentials` 子资源，新建模式才使用普通密码输入。
- React：`apps/web/src/settings/McpSettingsPanel.tsx` 与 `apps/web/src/styles.css`。

## 已验证

- 编辑已有 API Key 时显示独立凭证卡片、已配置状态、替换输入和删除操作；新建模式继续显示普通密码输入。
- 凭证写入/删除沿用 `configuration.mcp.credentials` 子资源，主 MCP 配置 payload 不携带编辑态 secret。
- MCP 专测 12/12、`typecheck:web` 通过。

## 未覆盖

- 尚未在认证浏览器中对比 CredentialResource 的完整文案、焦点/弹层和 computed-style；Wails 证据缺失。
