# T55 Task 8 review repair — round 2

Source: round-1 repair re-review of `.superpowers/sdd/plan-t55/task-8-fix-r1-review-package.patch`, SHA-256 `db08287db1cc960b5c2fc47658f29de3abfe5f1842c164df4956a2f07ba2c642`. Spec passes; one medium test coverage gap remains.

## Finding F1 — Medium: assert full action request set

`some(...)` assertions only prove an expected action exists and do not exclude duplicate or wrong-ID actions; conflict/failure paths lack action call assertions. Update `runDeliveryIntegration` tests to assert the complete ordered list of recovery action method+URL calls for success (dispatch and resolve), conflict, invalid input, backend failure, opt-out, and no-read. Each recovery attempt must produce exactly one expected action with exact read run/delivery IDs; skipped paths produce zero actions. No recovery behavior changes expected.

## Task

- Owner: frontend_implementer; validator: frontend_validator; independent reviewer: reviewer.
- Worktree `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-task8`; branch `codex/issue30-b6-t55-task8`.
- Repair base: source commit `fcdcb6dd756c77a7e28357ab84836fb717038e04`; docs commit `4a9b16734` follows.
- Owned files: `apps/mobile/src/delivery-integration-smoke.test.ts` only unless an assertion seam is impossible, then report before widening.
- Make test assertions fail before correction and pass after. Run focused suite, mobile typecheck, diff-check. Commit, record exact package/hash/report. Do not run live deployment or read secrets.

## Review focus

Exact zero/one request count, ordered path+method, correct read IDs, no extra or wrong action in every outcome branch. No production behavior changed.
