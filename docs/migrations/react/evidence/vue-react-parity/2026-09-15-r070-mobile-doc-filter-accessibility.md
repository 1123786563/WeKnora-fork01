# React/Vue parity evidence — mobile document filter accessibility

Date: 2026-09-15

## Change

Mobile document folder and tag filters now expose button roles and selected state. Tag filters also expose their visible names as accessibility labels, keeping the native filter chips understandable to screen readers while preserving the existing visual selection behavior.

## Validation

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check` — pending final commit check.
