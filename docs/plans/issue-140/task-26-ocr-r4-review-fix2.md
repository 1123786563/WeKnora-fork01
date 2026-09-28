# T26 OCR R4 review repair 2

## Scope

Addressed the assigned findings in `task-26-ocr-r4-review-fix-review.md`:

- **F1:** reserve the whole-space deletion slot synchronously before the first transport await. Another caller in the same scope receives `unresolved_action`; partial and ambiguous outcomes retain the original request ID, while a definite failure or `deleted` releases the reservation.
- **F2:** track active deletion reconciliation/retry by scope, kind, and request ID. Conditional abandonment refuses while a matching recovery is active, including the race where the service has already removed an intent before the recovery promise settles.
- **F3:** extract `createDeletionPageController`, used by the actual page for accepted receipts, initial unknown outcomes, recovery failures, abandonment reset, and gating. Behavior tests cover the controller transitions and confirmation outcomes.

## Verification

Commands run from `apps/miniprogram/` unless stated otherwise:

| Command | Result |
| --- | --- |
| `node --experimental-strip-types --test tests/export-deletion.test.mjs` | Passed: 31 tests, 0 failures. Includes deferred concurrent POST (one transport, recoverable ID), deferred retry + abandon (busy refusal, ambiguous response retains ID), cancel/changed/busy/success confirmation, and actual page-controller unknown gating transitions. |
| `pnpm test` | Passed: 200 tests, 0 failures. |
| `pnpm build:weapp` | Passed: Taro/Webpack compiled successfully. Existing bundle-size warnings: `common.js` is 478 KiB and async chunks are not used. |
| `pnpm typecheck` | Failed on 13 existing errors in untouched `src/features/account/pages.tsx` (missing `CommercialSummary` fields: `available`, `held`, `refund_locked`, `stale`, `as_of`, `plan_name`, `paid_until`). No reported errors in assigned files. |
| `git diff --check` (repository root) | Passed. |

## Evidence and limits

The page now delegates deletion transition state to the controller exercised in tests; the tests also retain a small source assertion that the initial unknown handler invokes it. No rendered Mini Program or device interaction harness is available in this package, so visual/browser behavior was not exercised. Loading, empty and generic error presentation were not changed by this repair.

Unrelated untracked review and plan files were present in the shared worktree and were left untouched.
