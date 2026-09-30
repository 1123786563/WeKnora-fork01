# Task 0B review fix report — round 1c

## Scope and baseline

- Review source: `task-0b-r1b-review.md`, finding R1B-F1 (High).
- Workspace: `/Users/wuyongjun/.codex/worktrees/issue-140-p0b-task/WeKnora-fork01`.
- BASE: `89fe623f0bd4279ba390cf4824fda563c78f4fb9`.
- Ownership: only the Task Detail controller and its tests/report were changed. No route or app composition wiring changed.

## Changes

- The controller records a scope epoch and increments it synchronously when it receives `updates(undefined)`.
- Initial hydrate and refresh success continuations no longer publish their captured Promise result. They require the same scope epoch and read the handle's current guarded `view()` before publishing. If scope was cleared or the current view is unavailable, the controller keeps the cleared state with scope-changed guidance.
- Added deferred Promise regressions for both orderings: hydrate/refresh resolves, then scope clear notification arrives before the controller continuation runs. Both assert the old view remains absent after settlement.

## Verification evidence

- `pnpm --filter @weknora/mobile exec tsx --test src/task-detail-view.test.ts` — passed, 7/7.
- `pnpm --filter @weknora/mobile typecheck` — passed (`tsc --noEmit`, exit 0).
- `git diff --check` — passed.

## Remaining gate

- F3 remains unresolved: native route registration and authenticated iOS/Android device acceptance were not part of this repair. No claim is made that Issue #145 is verified.
