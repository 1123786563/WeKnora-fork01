# React/Vue parity evidence — mobile document preview race guard

Date: 2026-09-15

Document preview downloads now use a generation token. Repeated preview opens, document changes, or closing the preview invalidate prior downloads, so stale content cannot reopen or replace the active preview.

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check`: passed.

Native file-preview interaction and expiry evidence remain pending.
