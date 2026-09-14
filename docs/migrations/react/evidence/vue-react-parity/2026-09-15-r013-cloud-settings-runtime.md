# R013 Web WeKnora Cloud settings runtime evidence

Date: 2026-09-15

## React authenticated browser

- URL: `http://localhost:5181/platform/settings?section=weknoracloud`
- The settings shell, WeKnora Cloud description, credential fields, refresh action and save action were reachable in the authenticated Chrome tab.
- The current Chinese locale exposed localized fallback labels: `模型`, `不可用`, `状态`, `服务端已返回`, and `保存后将验证服务可达性并加密存储凭证`.
- The server status value remained data-driven; when no status was returned the localized fallback was shown.

## Vue comparison boundary

- Navigating the available Vue tab to the same protected route redirected to `/login` because that tab had no authenticated session.
- Therefore this is React authenticated AX/runtime evidence only. It does not prove pixel parity or a same-session Vue comparison.

## Acceptance status

Code, typecheck, tests and build are green; protected same-session Vue comparison and backend credential success/failure scenarios remain open.
