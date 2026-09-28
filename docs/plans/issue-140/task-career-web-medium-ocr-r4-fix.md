# Career Web medium OCR r4 repair report

## Scope and starting point

- Issue: #140; source evidence: `ocr/round4-highrisk-analysis.md` and `ocr/round4-resume-increment-analysis.md`.
- Spec/plan: Task 4 in `docs/plans/2026-09-28-issue-140-ocr-r4-repair.md`.
- Finding base: `76df0cee0bf3ae23c14411c345b151ad518077ee`.
- Integrated starting HEAD: `f35ed2427024aae214db4aa42ecb61fd541d1c21` (`codex: fix career web OCR r4 high`).
- SearchPage remains excluded because Task 2 owns it. Its medium URL import receipt `requestId` validation finding remains for parent dispatch.
- No ProgressPage, ExportDeletionPage, API client, miniprogram, or backend files were changed.

## Changes

- `MaterialPage.tsx`: a verified receipt remains successful if the subsequent material refresh fails; URL-based material restore preserves the material ID and URL on non-forbidden failures and exposes a retry action. Only forbidden clears private state.
- `SubmissionPage.tsx`: export-list errors are exposed with retry, the unknown-version option is unavailable while loading/error, and submission is guarded while export availability is unknown.
- `PreparationPage.tsx`: a receipt request ID mismatch clears the attempt and exits recovery with a definite error.
- `RulePage.tsx`: a mismatched receipt clears the attempt, sets an invalid-response error, and exits recovery.
- `OpportunityPage.tsx`: `not_found` URL and pasted-text import errors terminate with a specific error message and do not enter unknown recovery.
- Added a focused regression test for each behavior (MaterialPage has two: receipt refresh and URL restore).

## RED / GREEN and verification

The regression tests were added first. The initial focused run exposed the original failures for material receipt refresh classification, URL restore clearing, export-list fail-open, Opportunity `not_found`, and the two receipt mismatch paths. Test fixture/assertion issues were corrected before implementation verification.

| Command | Result |
|---|---|
| `node --import tsx --test apps/web/src/career/MaterialPage.test.tsx apps/web/src/career/SubmissionPage.test.tsx apps/web/src/career/PreparationPage.test.tsx apps/web/src/career/RulePage.test.tsx apps/web/src/career/OpportunityPage.test.tsx` | Exit 0; 99 tests passed, 0 failed. Includes all six new assertions (two MaterialPage cases and one per other page). |
| `pnpm typecheck:web` | Exit 0; web TypeScript check passed. |
| `git diff --check` | Exit 0; no whitespace errors in the current worktree diff. |

The existing MaterialPage DOM test emits jsdom's `Not implemented: navigation to another Document` notice while the process still exits successfully.

## Remaining work / limits

- SearchPage's request ID mismatch check is intentionally deferred to its owning task.
- No layout changes were introduced. New retry and error controls are native buttons; state is exposed through the existing `role="alert"` / `role="status"` patterns.
- No independent code review or OCR was run by this task; parent owns those stages.
