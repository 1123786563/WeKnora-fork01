# React/Vue parity evidence — mobile configuration refresh state

Date: 2026-09-15

Configuration catalog refreshes now enter the same loading state as the initial load while retaining generation protection. Manual refreshes no longer display stale rows as if the request were idle.

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check`: passed.
