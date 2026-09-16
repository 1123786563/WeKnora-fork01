# React/Vue parity evidence — mobile administration race guard

Date: 2026-09-15

Tenant administration reloads now use a generation token around the concurrent members, invitations, and audit-log requests. A superseded refresh cannot overwrite current management data, errors, or loading state.

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check`: passed.

Native administration interaction evidence remains pending.
