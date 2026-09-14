# React/Vue parity evidence — mobile detail/reference race guards

Date: 2026-09-15

Document detail refreshes (including foreground resume) and Wiki/FAQ reference reloads now use generation tokens. Superseded responses cannot replace the active document/reference rows or clear a newer loading/error state.

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check`: passed.

Authenticated iOS/Android interaction evidence remains pending.
