# React/Vue parity evidence — mobile onboarding invitation race guard

Date: 2026-09-15

Invitation list loads on the mobile onboarding screen now use a generation token. Repeatedly opening or refreshing the invitation panel cannot let an older response replace the newest invitation state or error.

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check`: passed.

Native onboarding invitation interaction evidence remains pending.
