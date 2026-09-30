# Task report — forbidden submission history clears private state

## Scope and changes

- `apps/web/src/career/SubmissionPage.tsx`: an `applicationSubmissions` forbidden response now routes through `clearPrivate`, removing the full private read/form state and advancing `privateReadGeneration`. The existing read-generation and scope guards continue to reject late results.
- `apps/web/src/career/SubmissionPage.test.tsx`: added pane-reuse coverage that loads a bound version and populated form, receives forbidden history on refresh, then reuses the mounted pane for another accessible application and confirms neither prior version content nor form values return. Added a separate deferred version-read case to prove a late response cannot repopulate content after forbidden history.

## Verification

- `cd apps/web && node --import tsx --test --test-concurrency=2 src/career/SubmissionPage.test.tsx` — **19 passed, 0 failed** (exit 0). Covers loading, success, empty, forbidden/error states, review and refresh interactions, and private data clearing.
- `pnpm typecheck:web` — **passed** (exit 0; no diagnostics).
- `git diff --check -- apps/web/src/career/SubmissionPage.tsx apps/web/src/career/SubmissionPage.test.tsx` — **passed** (exit 0).

## Evidence and limits

The forbidden history handler now invokes the same full-state clear used by the other forbidden private-read paths. The regression checks that a subsequent accessible application read shows its own timeline, no prior material body, no prior form selections or note, and that a deferred version response remains fenced. Tests run in the component's jsdom harness; no separate browser or responsive layout check was applicable to this behavior-only change.
