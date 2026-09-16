# React/Vue parity evidence — chat model persistence

Date: 2026-09-15

## Change

The React chat route now persists the user's selected chat model in local storage under an origin/user/tenant-scoped key. A subsequent chat route restores the selection, while model loading still replaces stale selections with the first valid catalog entry.

## Validation

- `pnpm run test:web` — 895/895 passed.
- `pnpm run typecheck:web` — passed.
- `git diff --check` — passed.

## Remaining evidence

The authenticated browser tenant has no valid multi-model catalog, so persistence across changing model options remains covered by code and tests rather than a live multi-model interaction.
