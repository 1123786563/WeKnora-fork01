# Task 6 Repair Round 2 Report

## Finding addressed

The round 1 reviewer found that task/run plus recovery object identity did not represent the authorized session: `activeDeliveryRecovery` is cached by origin and tenant, so a sign-out/reauth can retain the same recovery object even for another user and a new runtime lease.

The route now subscribes to the `MobileRuntime` with `useSyncExternalStore`. It creates a route identity from deployment origin, user ID, active tenant ID, the opaque `ScopeLease` object (runtime scope generation), task ID, and run ID. Receipt and error state are tagged with that identity and are hidden synchronously when it changes. Async recovery and receipt reads apply only while the same identity remains current. Recovery actions reject after any identity change even if the cached recovery object remains identical. Scope changes also reset the Task Detail view/controller lifecycle.

The mounted lifecycle test registers through the runtime subscription, publishes sign-out, tenant switch, and same-origin/same-tenant reauthentication as a different user/new lease, then rerenders the route. It asserts prior receipt/error state is gone and the previous action rejects at every transition. The reauthentication transition deliberately retains the exact same cached `DeliveryRecovery` object, proving the lease/user identity check catches the case object identity cannot detect.

## Checkpoint and package

- Repair BASE: `7eb9f2ac368eb4fb5336e1b2fc5f4a9190145513`
- Repair HEAD: `19556f4fde344f06728c9e2156c4fcfbe14db6ec`
- Implementation commit: `19556f4fde344f06728c9e2156c4fcfbe14db6ec` — `fix(mobile): bind task detail delivery to runtime scope`
- Worktree: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-task6`
- Branch: `codex/issue30-b6-t55-task6`
- Review package: `.superpowers/sdd/plan-t55/task-6-fix-r2-review-package.patch`
- Review package SHA-256: `8e6877c55728d14078ac6f3346cfed6eff3b9af2cec8251ba3fc6d4e7feef525`

## TDD and verification

- RED: before the scope-bound route wiring, mounted route test failed because the authorized route did not expose the injected reader's receipt; earlier helper-only tests were insufficient to exercise route state across runtime changes.
- `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx` — PASS, 73 tests, 0 failures.
- `pnpm --filter @weknora/mobile test` — PASS, 297 total, 283 passed, 14 skipped, 0 failed. The 14 skipped tests are credential/opt-in live integration checks.
- `pnpm --filter @weknora/mobile typecheck` — PASS (`tsc --noEmit`).
- `git diff --check 7eb9f2ac368eb4fb5336e1b2fc5f4a9190145513 19556f4fde344f06728c9e2156c4fcfbe14db6ec` — PASS.

## Files and limits

- `apps/mobile/src/app/tasks/detail.tsx`
- `apps/mobile/src/app-smoke.test.tsx`

`paseo.json` remains unchanged. No device build or native browser session was run; verification uses the mounted route smoke harness with a controllable runtime port.
