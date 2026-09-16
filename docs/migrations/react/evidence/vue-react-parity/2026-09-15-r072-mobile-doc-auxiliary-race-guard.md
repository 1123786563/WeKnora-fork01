# React/Vue parity evidence — mobile document auxiliary-load guard

Date: 2026-09-15

## Change

Folder and tag auxiliary loading on the mobile document page now uses a generation token. A superseded folder/tag request cannot overwrite newer filter state or surface an obsolete error while the page is being refreshed or resumed.

## Validation

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check` — pending final commit check.
