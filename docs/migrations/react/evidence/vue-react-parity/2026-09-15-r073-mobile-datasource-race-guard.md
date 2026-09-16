# React/Vue parity evidence — mobile data-source race guards

Date: 2026-09-15

## Change

Mobile data-source inventory refreshes now use a generation token, and resource, child-resource, and Drive root loading ignores superseded responses, errors, and loading cleanup. Switching sources or refreshing while requests are in flight can no longer reintroduce stale rows or stale error state.

## Validation

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check`: passed.

Native authenticated data-source interaction evidence remains pending.
