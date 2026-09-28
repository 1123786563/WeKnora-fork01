# T26 mini-program OCR round-4 fixes

## Scope and base

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`
- Base / starting HEAD: `76df0cee0bf3ae23c14411c345b151ad518077ee`
- Source findings: `round4-highrisk-analysis.md` (application hard-ineligible gate), `round4-resume-increment-analysis.md` (N2 abandon recovery, N3 stale partial after unknown delete), and `ocr-round-4-resume2.md`.
- Behavior references: T26 ticket `docs/design/job-search/tickets-draft/26-mini-application-material-submission.md` requires explicit continuation for hard conflicts and reconciliation for unknown writes; T32 ticket `docs/design/job-search/tickets-draft/32-mini-export-deletion.md` covers mini-program export/deletion and local cleanup.
- Scope is limited to the two assigned page files, their tests, and this report. No OCR was run.

## Changes

- `application-material.tsx`: the create button is disabled for an ineligible evaluation until acknowledged; `continueDespiteHardFailure` is true only when the evaluation is ineligible and the user acknowledged it.
- `export-deletion.tsx`: an `outcome_unknown` from starting deletion marks recovery unresolved, so an old in-memory `partial` receipt cannot unlock another delete. The current persisted request remains available in the pending deletion recovery block. Added confirmed abandon actions for pending export and deletion. Each confirmation says only the local recovery record is cleared and the original operation may already have taken effect. After confirmation, local state updates immediately; deletion state and acknowledgement are reset. Abandon actions are disabled while related recovery or deletion requests are in flight.
- Tests: added page wiring assertions and a service-plus-gating scenario covering partial deletion followed by an ambiguous new delete.

## Verification evidence

TDD RED was observed before the page edits: `node --experimental-strip-types --test tests/application-material.test.mjs tests/export-deletion.test.mjs` reported the two new UI contract tests failing against the old source (57 passed, 2 failed). After implementation, the targeted command passed (60 passed, 0 failed).

| Command | Result |
| --- | --- |
| `pnpm --filter @weknora/miniprogram test` | PASS — 191 passed, 0 failed |
| `node --experimental-strip-types --test tests/application-material.test.mjs tests/export-deletion.test.mjs` | PASS — 60 passed, 0 failed |
| `pnpm --filter @weknora/miniprogram typecheck` | BLOCKED by pre-existing errors in `apps/miniprogram/src/features/account/pages.tsx`: 12 TS2339 errors because `CommercialSummary` lacks `available`, `held`, `refund_locked`, `stale`, `as_of`, `plan_name`, and `paid_until`. No diagnostics referenced the changed files. |
| `pnpm --filter @weknora/miniprogram build:weapp` | PASS — Webpack compiled successfully. Existing bundle warnings: `common.js` is 477 KiB (over 244 KiB recommendation) and async chunks are not configured. |
| `git diff --check` | PASS — no whitespace errors |

## Coverage and remaining limits

- There is no page-level React/Taro rendering harness in `apps/miniprogram/tests`; the suite exercises real career service behavior and the shared pure deletion gate, while focused source assertions verify the JSX-to-state wiring and confirmation calls. The Taro build verifies page compilation, but no WeChat DevTools/device interaction run was available.
- Typecheck remains blocked by the unrelated account-page `CommercialSummary` contract mismatch above.
