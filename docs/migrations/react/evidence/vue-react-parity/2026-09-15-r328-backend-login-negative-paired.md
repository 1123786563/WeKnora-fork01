# 2026-09-15 R328 — live backend login negative paired capture

## Conditions

- Backend: Docker `app` service, `http://localhost:8080/health` returned `{"status":"ok"}` after starting with container-safe `REDIS_ADDR=redis:6379` and `DB_HOST=postgres` overrides.
- Browser: Playwright Chromium, 1440×900, `zh-CN`, fresh contexts.
- Targets: Vue `http://127.0.0.1:5180/login` and React `http://127.0.0.1:5181/login`.

## Observed behavior

- Both clients reached the real backend auth/config endpoints.
- Both submitted `parity-test@local.dev` with the recorded test password; backend returned HTTP 401 and both stayed on `/login` with the localized invalid-credential message.
- Vue and React login screenshots were captured under the same viewport and locale:
  - `artifacts/parity-20260915/vue-backend-auth.png`
  - `artifacts/parity-20260915/react-backend-auth.png`

## Acceptance classification

This is live backend negative-path and paired login evidence. It does not prove authenticated page parity because the recorded account is not present in the currently attached database; protected-page success, RBAC transitions, and authenticated same-condition captures remain open.

