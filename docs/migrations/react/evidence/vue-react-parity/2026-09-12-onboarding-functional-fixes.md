# /onboarding/workspace 功能化（2026-09-12，Round 3）

依据登录页差异报告阻断项 #9，对齐 Vue WorkspaceOnboarding.vue + CreateTenantDialog.vue + MyInvitationsDialog.vue：

## 修复内容
- 策略加载：auth.me + hydrate + 我的待处理邀请数（/api/v1/me/invitations/pending-count）；失败进入可重试错误态（role=alert + Retry），与 Vue policy-error 一致。
- 角色分叉：scopeRuntime.can('can_create_tenant') 决定标题/描述/按钮组（可建工作区 vs invite-only 仅邀请入口）；invite-only 提示条。
- 创建工作区：POST /api/v1/tenants（新增 api-client identity tenants.admin.create，运行时校验 name 1-128 / description ≤512，依据 internal/handler/tenant.go:89-90）；成功后刷新 auth.me 并跳 /platform/knowledge-bases；双击防护（creating 禁用）。
- 我的邀请：列表（listMine，仅 pending）、接受（accept → 刷新并自动离开 onboarding）/ 拒绝（decline），计数联动。
- 有效租户自动重定向（Vue watch hasValidTenant）；退出登录链接保留。

## 测试与证据
- 新增 apps/web/src/auth/onboarding.ts 纯模块 + 4 例回归测试（重定向、策略错误、invite-only 视图、创建校验规则镜像后端 binding）。
- web 128/128 通过；typecheck:web 通过。提交 b95e622、c5607f4。
- 仍开放：onboarding 文案 i18n 化与视觉（卡片 min(520px)/圆角20/双列按钮网格/断点 560px），记录于矩阵。
