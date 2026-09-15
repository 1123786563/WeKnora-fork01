# R304 邀请注册入口 fallback 本地化（2026-09-15）

JoinPage 缺少邀请 token、邀请链接无效/过期两个用户可见状态已接入 onboarding 五语言目录；服务端返回的具体错误仍优先显示。

- auth copy i18n tests：2/2。
- `pnpm run typecheck:web`：通过。
- `git diff --check`：通过。

真实邀请 provider 生命周期、五语言浏览器截图和 native 验收仍待补齐。
