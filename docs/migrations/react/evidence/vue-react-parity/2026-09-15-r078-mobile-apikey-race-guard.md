# React/Vue parity evidence — mobile API-key reload race guard

Date: 2026-09-15

Mobile API-key list refreshes now use a generation token, so a superseded response cannot replace the current key list or loading/error state after create, revoke, or manual refresh.

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check`: passed.

Native API-key management interaction evidence remains pending.
