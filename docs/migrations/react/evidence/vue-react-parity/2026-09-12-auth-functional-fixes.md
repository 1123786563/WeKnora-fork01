# /login /register /join 鉴权功能修复（2026-09-12，Round 2）

依据登录页差异报告（阻断项 1-5、7 的功能部分）修复 React 实现：

## 修复内容
1. 邀请 token 流（Vue Login.vue:773-810）：/login?token= 与 /register?token= 现解析邀请（lookupInvitation）、显示邀请横幅；已登录访问直接 acceptInvitationByToken 兑换后进入应用；OIDC 跳转前将 token 存入 sessionStorage（weknora_pending_invite_token），回调后重定向回 /login?token= 继续兑换。
2. /join 不再死路：携带 token 时重定向 /register?token=...（保留 token）。
3. 登录持久化（Vue persistLoginResponse:547-600）：completeAuthentication 现解析 active_tenant/memberships，按 user.tenant_id(home) 与 active 差异设置租户 override（scopeRuntime.setTenant → X-Tenant-ID），无租户跳 /onboarding/workspace，其余 honour ?next。
4. autoSetup（Vue:817-831）：/login 挂载时调用，成功进入应用并置 weknora_lite_mode；失败置 weknora_auto_setup_failed 且不再重试。
5. 校验规则（Vue formRules/newPasswordRules）：新增 apps/web/src/auth/validation.ts 纯模块 —— email 格式、密码 8-32、注册用户名 2-20 + ^[a-zA-Z0-9_一-鿿]+$、确认密码匹配、复杂密码模式（大小写/数字/特殊字符，读取 registrationConfig.complexPasswordEnabled）。JoinPage minLength 6→8 并增加确认密码。
6. OIDC：回调成功现 honour ?next（不再硬跳 /platform/knowledge-bases）；oidcUrl 失败经 successEnvelope 抛错并展示。
7. 注册成功后切回登录卡并预填 email（Vue:744-746）；提交中禁用输入框。

## 新增共享层（有真实后端依据）
- packages/api-client/src/auth/endpoints.ts: acceptInvitationByToken（POST /api/v1/me/invitations/accept-by-token，internal/router/routes_auth_tenant.go:175），运行时校验 membership.tenant_id。

## 测试与证据
- 新增测试：apps/web/src/auth/{validation,session-persist,invite-flow}.test.ts 共 12 例，先证明旧实现不符（如 minLength=6、无确认密码、tenant 丢弃），修复后全部通过。
- web 全量 124/124 通过；typecheck:web、typecheck:shared 通过。
- 截图：screenshots/register-react-round2.png（确认密码字段已出现）；screenshots/login-react-round2.png。
- 仍开放（记录于矩阵）：auth/inviteRegister/workspaceOnboarding 文案 i18n 化（5 locale）、语言切换下拉、品牌视觉区/轮播、WorkspaceOnboardingPage 功能化（差异报告 #6/#9/#10-13）。
