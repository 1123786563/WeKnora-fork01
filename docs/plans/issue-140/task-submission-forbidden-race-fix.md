# Task: fence private pane reads after forbidden export response

## Brief

Source: parent task assignment, based on `docs/plans/issue-140/task-career-web-medium-r4-review-fix-review.md` finding F1. Scope is limited to `apps/web/src/career/SubmissionPage.tsx`, its test, and this report. Add a deferred-promise regression: start material exports and application submissions reads, reject exports as forbidden first, then resolve submissions and ensure the private timeline remains cleared. Preserve existing fixes. Do not modify requirements or commit unrelated files.

## Relevant references

- Review finding F1: a late `applicationSubmissions` success may restore private records and switch the pane from forbidden to ready after export access is forbidden.
- Approved behavior referenced by the review: `docs/specs/2026-09-23-weknora-job-search-design.md` and `docs/adr/0017-immutable-job-and-application-evidence.md`.
- Existing forbidden clear behavior and tests in `SubmissionPage.tsx` / `SubmissionPage.test.tsx` were retained.

## Implementation

Added a private read generation fence for submission history, profile revision, and material export callbacks. Reads capture the current generation and verify it before applying either success or error state. `refresh` advances the generation before starting the next pane read cycle; `clearPrivate` advances it before clearing private state. This prevents in-flight sibling reads from restoring private pane state after a forbidden response.

Added a deferred-promise regression that resolves the forbidden export rejection before resolving the pending submissions response. It asserts the forbidden alert remains, the timeline remains absent, and the form stays hidden after the late history response.

## Verification evidence

- RED before implementation: `cd apps/web && node --import tsx --test --test-concurrency=2 src/career/SubmissionPage.test.tsx` — 14 passed, 1 failed. The new test failed after the late history response because the forbidden alert had been replaced with an empty state.
- GREEN: same focused test command — 15 passed, 0 failed.
- Typecheck: `cd apps/web && ../../node_modules/.bin/tsc -b --pretty false` — exit 0, no diagnostics.
- Diff hygiene: `git diff --check` — exit 0.
- Scope: only `SubmissionPage.tsx`, `SubmissionPage.test.tsx`, and this report are part of this task. Pre-existing and concurrent unrelated changes were left untouched.

## Commit

Commit: `fix(web): fence private submission reads after forbidden`. The exact SHA is recorded in the parent task report. This commit contains only the two assigned source/test files and this report.
