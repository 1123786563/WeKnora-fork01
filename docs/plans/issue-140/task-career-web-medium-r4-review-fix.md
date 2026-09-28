# Career web OCR r4 review repairs

**Scope:** Findings F1, F2, and F3 from `task-career-web-medium-ocr-r4-review.md`.
**Workspace:** `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`
**Base:** `50932d609f3a81666ec6efd5d6ae8ba8ebefae64`
**Implementation commit:** `535c414db17d5812daa5907793b80b90836f8195`

## Changes

- A failed URL material restore keeps the URL pointer and retry action, while locking the editor and save/confirm actions until a successful restore establishes the draft body.
- A forbidden material-export read clears submission records, revision/export state, and composed private fields, then displays the existing forbidden state. Transient export failures retain the retry state.
- Successful export refreshes clear a selected export that is no longer in the deliverable list. Both the submit button and write constructor require a non-unknown selection to resolve to a current verified export.
- Added one regression test for each finding. No SearchPage files were changed; that task is assigned separately.

## Verification

- RED: `node --import tsx --test apps/web/src/career/MaterialPage.test.tsx apps/web/src/career/SubmissionPage.test.tsx` — the three new regression tests failed before implementation at the expected assertions (restore controls enabled, forbidden state not shown, stale export still allowed).
- GREEN: `node --import tsx --test apps/web/src/career/MaterialPage.test.tsx apps/web/src/career/SubmissionPage.test.tsx` — **37 passed, 0 failed**. The existing JSDOM download test emits its pre-existing `Not implemented: navigation to another Document` notice while passing.
- TypeScript: `pnpm --filter @weknora/web exec tsc -b --pretty false` — passed.
- Whitespace: `git diff --check` — passed.
- Staged implementation paths were explicitly limited to `MaterialPage.tsx`, `MaterialPage.test.tsx`, `SubmissionPage.tsx`, and `SubmissionPage.test.tsx`. Other agents' changes remained unstaged.

## Remaining risks

- Browser-level visual/responsive checks were not run; these changes affect editor/action gating and submission authorization/error states, and the targeted component tests cover the user-visible transitions.
- The test harness reports the JSDOM navigation notice above during the existing authenticated-download case; it does not fail the suite.
