# Task Office route review fixes — round 1

Assigned findings from `taskoffice-route-review.md` on integration HEAD `685641f3d`.

## Changes

- F1: extracted the route list lifecycle into `apps/mobile/src/app/task-office-state.ts`. It subscribes to the opening lease's synchronous `onRevoke` event, increments the read generation, and immediately publishes an empty list. A late response cannot repopulate rows. The route lifecycle key includes deployment origin, tenant ID, user ID, and a stable per-lease ID, so same-tenant user changes and lease rotations mount a fresh empty list.
- F2: every list/refresh call advances a generation. Only the newest request can publish rows, error, or loading state. Disposal invalidates outstanding requests and removes the revocation subscription.
- Added focused behavioral tests for revocation while a refresh is pending, same-key reauthorization identity boundaries, stale success and failure ordering, and unmount invalidation.

## Verification

- `pnpm --filter @weknora/mobile exec tsx --test src/app/task-office-state.test.ts` — pass, 4 tests.
- `pnpm --filter @weknora/mobile typecheck` — pass (`tsc --noEmit`).
- `git diff --check` — pass.

Tests exercise the extracted controller with injected deferred reads and revocable lease doubles. No device route, backend, native build, or real authentication flow was changed or verified; those remain outside this assigned route fix.
