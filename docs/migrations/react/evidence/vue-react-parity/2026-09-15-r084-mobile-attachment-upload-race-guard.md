# React/Vue parity evidence — mobile attachment upload race guard

Date: 2026-09-15

Mobile chat attachment uploads now use an upload generation and active-session check. A late upload from a previous session or superseded picker action cannot append to the current attachment list or replace its error/loading state.

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check`: passed.

Native attachment picker/upload interaction evidence remains pending.
