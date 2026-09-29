# Task 0B review fix report — round 1b

## Scope and baseline

- Review source: `task-0b-r1-review.md`, findings R1-F1 (High), R1-F2 (High), R1-F3 (Medium).
- Workspace: `/Users/wuyongjun/.codex/worktrees/issue-140-p0b-task/WeKnora-fork01`.
- BASE: `a9d7d2029eb6cf7d0d0944e2140839be13e877db`.
- Ownership stayed within mobile-core Runtime lease/Task Office detail and the mobile Task Detail controller/tests. No Expo route registry, global route or app composition edits.

## Changes

- Added optional synchronous `ScopeLease.onRevoke()` notification implemented by `RuntimeScopeLease`. Task detail subscribes for its lifetime, clears cached detail/events and publishes `undefined` immediately when Runtime revokes the opening lease. The mobile detail controller maps that notification to a cleared view and scope-changed guidance, so a mounted consumer drops the previous task without refresh.
- `flushPersisted()` now skips work unless the exact opening lease is still the current active lease, and checks again after save. Both `close()` and interrupt/terminal flushes use this guarded function. Regression coverage advances an event below the persistence stride, replaces the active lease while leaving the old lease active, then closes and asserts no save is sent through the replacement scope.
- `act()` and queued-intent flush require `ports.lease() === openingLease && leaseActive(openingLease)` after awaited command completion, including error paths. Regressions cover replaced-but-still-active leases for `act()` and queued acknowledgements; no stale acknowledgement is recorded.
- Added controller coverage proving scope-loss notification clears its mounted view immediately.

## Verification evidence

- `pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-detail.test.ts` — passed, 53/53.
- `pnpm --filter @weknora/mobile exec tsx --test src/task-detail-view.test.ts` — passed, 5/5.
- `pnpm --filter @weknora/mobile typecheck` — passed (`tsc --noEmit`).
- `git diff --check` — passed.

## Remaining gate

- F3 remains unresolved: native route registration and real iOS/Android authenticated device acceptance were not part of this repair. No claim is made that Issue #145 is verified.
- Scope-loss clearing is driven by Runtime lease revocation. A consumer that supplies a non-Runtime custom `ScopeLease` without `onRevoke` still clears on the next guarded operation; Runtime-minted leases provide the synchronous notification used in production.
