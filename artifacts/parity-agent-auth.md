# Auth / onboarding parity report

日期：2026-09-15

## 范围

本次仅新增 `apps/web/src/auth/` 下的 auth 页面、API seam、纯状态校验、专属样式和测试，并在 `apps/web/src/main.tsx` 增加 `/login`、`/register`、`/onboarding/workspace` 的入口分流。未修改 `packages/ui` 或其他业务域；工作区既有修改保持不动。

## Vue 对齐结果

| 场景 | React 覆盖 | 证据 |
|---|---|---|
| 登录默认态、邮箱/密码校验 | 是 | `auth-state.test.ts` |
| 注册用户名、邮箱、密码、确认密码及复杂密码策略 | 是 | `auth-state.test.ts` + `RegisterPage` |
| 登录/注册提交中禁用字段和按钮 | 是 | `AuthPages.tsx` |
| 登录成功、失败提示及导航 | 是 | `AuthPages.tsx`、`api.ts` |
| 注册成功/失败提示 | 是 | `AuthPages.tsx` |
| `/auth/config` invite-only 注册门控 | 是 | `LoginPage` |
| onboarding 策略 loading / error / retry | 是 | `getOnboardingPresentation` 测试 |
| onboarding 可创建空间 / invitation-only | 是 | `WorkspaceOnboardingPage` |
| pending invitation count | 是 | `/api/v1/me/invitations/pending-count` |
| API 路径与 login token/refresh/tenant 持久化 | 是 | auth adapter 测试 |
| share-link token lookup / `register-by-invite` 自动登录 | 未闭环 | React 页面尚未复刻 Vue 分支 |
| 响应式与 reduced-motion | 是 | `auth.css` |

## 验证

- RED：在 `auth-state.ts` 尚不存在时运行 auth 测试，按预期因模块缺失失败。
- GREEN：auth 专项测试通过，包括校验、onboarding 分支、API 路径和会话持久化。
- 当前专项命令输出：68 passed，1 failed；唯一失败是既有 `platform/legacy-session.test.ts` 期望 refresh token 的基线失败，与本次 auth 文件无关。
- `tsc` auth 行无诊断。全项目 TypeScript 仍受既有问题阻塞：`packages/ui` 的 `react-dom` 类型解析、documents draft 类型、knowledge-bases states 类型等；未修改这些文件。
- 未执行真实浏览器/后端认证验收：当前任务环境没有提供可用的 auth 服务和 provider 回调，因此 loading、真实 API 成功/失败链路仍需独立审查时做 browser/runtime 验证。

## 独立审查注意项

请重点复核：React 入口当前仍是单页 pathname 分流而非完整 router；邀请 token 注册和 OIDC 回调尚未在 React 页面中复刻；创建空间/邀请列表按钮目前保留导航契约但对应业务页面不在本任务范围。上述内容不应被本次静态测试结果视为已完成的真实链路验收。
