# Task Office route review fixes — round 2

Addressed the R1 follow-up: revocation subscription was created in a `useState` render initializer, which could leak when React discarded a render or replayed effects.

## Changes

- Controller construction is now side-effect free. The mounted effect calls `controller.mount()`, which registers `openingLease.onRevoke`; effect cleanup removes the listener and invalidates pending reads.
- A lease already revoked before effect setup invokes the listener synchronously during mount, clearing list state before the first list read. The controller refuses reads after that revocation.
- Mount and cleanup can be replayed safely by StrictMode: each committed setup registers one callback and its cleanup releases it.
- Added a behavior test for no subscription at construction, immediate fail-closed behavior for an already revoked lease, and listener cleanup after unmount. Existing stale generation and revocation tests remain.

## Verification

- `pnpm --filter @weknora/mobile exec tsx --test src/app/task-office-state.test.ts` — pass, 5 tests.
- `pnpm --filter @weknora/mobile typecheck` — pass (`tsc --noEmit`).
- `git diff --check` — pass.
