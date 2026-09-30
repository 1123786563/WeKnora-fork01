# T10 Task 4 Independent Web Validation

- **Status:** DONE_WITH_CONCERNS
- **Validated code SHA:** `6e6a903bd0a52435a9c9cd3d51e0a635dde3afd6` (`feat(web): add career evaluation detail flow`)
- **Validation worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-ui/WeKnora-fork01`
- **Current HEAD:** `5061c90d16da1008f7b3d4ae0dab4d880e25da54`, documentation-only descendant; `git diff --quiet 6e6a903bd0a52435a9c9cd3d51e0a635dde3afd6 HEAD -- <seven Task 4 source/test paths>` exited 0.
- **Source changes:** none.

## Acceptance evidence

Reused same-SHA implementation evidence in `task-4-web-report.md`:

- `pnpm --filter @weknora/web exec node --import tsx --test src/career/OpportunityPage.test.tsx src/routes.test.ts src/router.test.tsx` — **PASS**, 40 passed, 0 failed.
- `pnpm typecheck:web` — **PASS**.
- `pnpm test:web` — **PASS**, 2,336 passed, 0 failed, 0 cancelled.
- `pnpm build:web` — **PASS** (build completed; report notes existing CSS and chunk warnings).
- `git diff --check` — **PASS**.

Independent read-only inspection commands:

- `git rev-parse HEAD` — `5061c90d16da1008f7b3d4ae0dab4d880e25da54`.
- `git show -s --format='%H%n%T%n%P%n%s' 6e6a903bd0a52435a9c9cd3d51e0a635dde3afd6` — confirmed assigned code commit and parent `ed0a71919e1bdb0347e4fa4397db56bfa2578a9c`.
- `git diff --quiet 6e6a903bd0a52435a9c9cd3d51e0a635dde3afd6 HEAD -- apps/web/src/career/OpportunityPage.tsx apps/web/src/career/OpportunityPage.test.tsx apps/web/src/career/opportunity.css apps/web/src/routes.tsx apps/web/src/routes.test.ts apps/web/src/router.tsx apps/web/src/router.test.tsx` — **PASS**, no Task 4 path drift after the code SHA.

Coverage inspected in the existing focused tests and implementation:

- **Success / hard qualification:** `ineligible` is shown in the result heading/verdict before the soft match section; TypeScript evidence does not replace or soften the 2027-only / confirmed-2026 hard mismatch. The matching 2027 rule renders as “符合已识别条件.”
- **Unknown / absent profile:** `confirmed_graduation_year_missing` renders a “待确认” verdict and explicit missing-confirmed-graduation message, without presenting it as a positive match.
- **Provenance:** the evaluation page renders quoted JD excerpts and spans, links to the captured fact key/value/revision, and displays the pinned snapshot, profile revision, ruleset, and fact versions. The raw JD is rendered as text in a `<pre>`.
- **Stable old/new result:** re-evaluation uses a new request ID and retains links for both captured profile revisions; detail reads by stable evaluation ID and links to the fixed opportunity snapshot.
- **Loading, errors, malformed response, and 403:** detail has a loading status, retryable generic error, invalid-link state, explicit forbidden state, and malformed-wire coverage. Request receipt recovery and errors are covered for evaluation creation.
- **Scope/logout fencing:** scope-switch tests confirm clearing private evaluation data and no refetch under the new scope. The detail page also binds request cancellation to the active scope signal. Logout-specific integration is not exercised separately.
- **Accessibility / responsive:** semantic `main`, headings, labelled sections, ordered hard-rule list, blockquotes, status/alert live regions, busy state, and native links/buttons are present. Focused tests assert narrow-width media rule, wrapping, and preserved JD line breaks.
- **Routing / browser behavior:** route tests cover authenticated protection and route matching for stable evaluation URLs. Component tests run in a DOM harness; no real browser interaction or authenticated API session was performed.

## Gaps and risks

- **No acceptance gap found in the inspected UI criteria.** Hard `ineligible` prominence, missing-profile unknown, provenance display, stable old/new routes, scope clearing, 403 handling, responsive wrapping, and malformed/error rendering have same-SHA evidence.
- **Concern:** actual browser rendering, keyboard traversal/screen-reader announcement, logout event wiring, and authenticated API behavior remain unverified in this validation run. The Task 4 plan explicitly assigns live API/SQLite browser acceptance to the controller; the DOM/CSS tests do not establish those behaviors.
- **No change to requirements, tracked source, or test source was made.**
