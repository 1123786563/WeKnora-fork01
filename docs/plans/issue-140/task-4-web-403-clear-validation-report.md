# T10 Transient Evaluation 403 Clear Independent Validation

- **Verdict:** DONE_WITH_CONCERNS
- **Validated code SHA:** `59fdec92c4ebf9d990d234b3328f5aaa1bc0f0b7` (`fix(web): clear career state on eval forbidden`)
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-ui/WeKnora-fork01`
- **Observed HEAD:** `1e8e1797aa944e4db5a5bcfee82cac7e255511e4`, documentation-only descendant. The two Task 4 UI source/test paths are unchanged between assigned SHA and HEAD.
- **Tracked source/test changes by validator:** none.

## Checks run at the assigned revision

- `pnpm --filter @weknora/web exec node --import tsx --test src/career/OpportunityPage.test.tsx` — **PASS**, 27 passed, 0 failed. React emitted `act(...)` warnings in some tests; test process exited 0.
- `pnpm typecheck:web` — **PASS**, exit 0.
- `git diff --quiet 59fdec92c4ebf9d990d234b3328f5aaa1bc0f0b7 HEAD -- apps/web/src/career/OpportunityPage.tsx apps/web/src/career/OpportunityPage.test.tsx` — **PASS**, no code/test drift after assigned SHA.
- `git rev-parse HEAD` — `1e8e1797aa944e4db5a5bcfee82cac7e255511e4`.

Reused same-SHA evidence from `task-4-web-403-clear-report.md`:

- `pnpm --filter @weknora/web exec node --import tsx --test src/platform/global-command-palette-live-search.test.tsx` — **PASS**, 7 passed.
- `pnpm build:web` — **PASS**, with existing CSS import-ordering, `calc()` spacing, and large-chunk warnings.
- `git diff --check` — **PASS**.
- `pnpm test:web` — **2347/2348** on the first run; the one failure was an unrelated debounced-search timing assertion, which passed in its isolated 7/7 run. A full-suite rerun stalled without output for over 8 minutes with an agent-editor test active and was interrupted; it is not counted as passing. Per assignment, the full suite was not repeated.

## Acceptance findings

- **Evaluation POST 403 after prior success:** focused test first saves the JD and evaluation, then returns `forbidden` on a later re-evaluation. It asserts the textarea is cleared, prior JD/evaluation links and qualification status are gone, and the forbidden message is shown.
- **Receipt lookup 403 after prior success:** focused test creates prior saved state, causes a timeout, returns 403 on receipt lookup, and asserts the same private JD and evaluation state is removed.
- **Pending intent / late response fencing:** shared `clearPrivate` clears the draft, metadata, import attempt/receipt, evaluation receipt/history, request ID, and evaluation state. The late-response scope-change test resolves an earlier evaluation after switching scope and verifies it cannot restore old private content or links.
- **Code path:** both evaluation POST and receipt lookup `forbidden` branches call `clearPrivate`; successful paths retain fixed-ID checks and receipt presentation.

## Limits and risks

- No functional gap found in the assigned 403 clearing behavior.
- Full Web suite evidence remains imperfect as recorded above; targeted Career tests and typecheck pass on the assigned SHA. Full-suite retry was intentionally avoided per assignment.
- No live authenticated browser session was used. Targeted checks are DOM-level component tests.
