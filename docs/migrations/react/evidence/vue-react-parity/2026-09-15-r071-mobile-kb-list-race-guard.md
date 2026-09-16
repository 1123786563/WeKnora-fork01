# React/Vue parity evidence — mobile KB list race guard

Date: 2026-09-15

## Change

Mobile knowledge-base list loading now uses a generation token. Refreshes triggered by app resume, upload completion, or create flow cannot let an older response overwrite newer list data, error copy, or loading state.

## Validation

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check` — pending final commit check.
