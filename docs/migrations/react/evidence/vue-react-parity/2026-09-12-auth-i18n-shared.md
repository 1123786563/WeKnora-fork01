# auth 文案迁入共享 i18n 包（2026-09-12，Round 7）

- packages/i18n/src/index.ts 新增 authMessages（73 键 × 5 locale，程序化提取自 frontend/src/i18n/locales/*.ts，字节级一致），并入 messages 合并链。
- apps/web/src/auth/LoginPage.tsx 改用共享 formatMessage(locale, key, params) 与 isLocale/Locale 类型；语言下拉与 localStorage 'locale' 持久化保持不变。
- 迁移后回归：i18n 测试 7/7（新增 authMessages.test.ts 3 例：全 locale 键存在、跨 locale 键集一致、bannerTitle {tenant} 插值）、web 131/131、typecheck:web 干净、浏览器交互（语言切换/轮播/切回）复测通过。
- JoinPage 与 WorkspaceOnboardingPage 的本地 MESSAGES 表待同法迁移（tenant.create.*/tenantInvitation.* 键不在本次 73 键提取范围）。
