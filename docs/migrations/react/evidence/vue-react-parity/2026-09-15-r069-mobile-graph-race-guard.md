# React/Vue parity evidence — mobile graph stale-response guard

Date: 2026-09-15

## Change

Mobile graph loading now tracks a monotonically increasing request id. Late success or error results from an older filter/node request are ignored, and only the newest request can update graph, error, or loading state.

## Validation

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check` — pending final commit check.
