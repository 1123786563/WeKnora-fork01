# Submission forbidden race follow-up

**Assigned review:** `docs/plans/issue-140/task-submission-forbidden-race-review.md`
**Base revision:** `cf19364ff3fa2c20f641745fcd6c5eb85acac58c`
**Scope:** `SubmissionPage.tsx`, `SubmissionPage.test.tsx`, and this report only.

## Findings fixed

- **F1 — Refresh stranded the profile revision read.** The revision effect now restarts on `reload`, so advancing the shared private-read generation during refresh also starts a read in that generation. A deferred-order regression resolves the superseded read first, confirms it cannot settle the current loading state, then resolves the refreshed read and verifies a submission uses its revision.
- **F2 — A late material-version review could restore cleared state.** `reviewVersion` now captures the private-read generation and checks it together with the active scope before applying either its success or error result. `clearPrivate` already clears both `versionDetail` and `versionMessage` while advancing the generation. The deferred regression starts a review, triggers a forbidden export response, resolves the review late, and reuses the pane for another application to check that private version content stays absent.

## Verification evidence

- **F1 RED:** `cd apps/web && node --import tsx --test --test-concurrency=2 src/career/SubmissionPage.test.tsx` — 16 passed, 1 failed. The new test failed because refresh did not start a second `open` read (`openCalls` remained 1).
- **GREEN focused suite:** same command — **17 passed, 0 failed**.
- **Typecheck:** `cd apps/web && ../../node_modules/.bin/tsc -b --pretty false` — exit 0, no diagnostics.
- **Diff hygiene:** `git diff --check` — exit 0.

The focused component suite covers loading, empty and populated timeline states, forbidden and transient export errors, retry/refresh interactions, version review, and explicit submission confirmation. Browser testing is not applicable to this component-level async-state repair; no browser behavior or layout was changed.

## Commit

Commit only the two assigned source/test files and this report. Preserve concurrent T26 files and reports in the shared worktree.
