# T10 UI Review Fix R1 Independent Validation

- **Verdict:** DONE_WITH_CONCERNS
- **Validated code SHA:** `d2ab0698c26ca0f3595678afe7d900b33a137549` (`fix(web): re-evaluate saved career evidence`)
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-ui/WeKnora-fork01`
- **Observed HEAD:** `4e60423670bd1e398775a7223429e6f31f91e932`, documentation-only descendant. The three changed Task 4 UI paths are unchanged between assigned SHA and HEAD.
- **Tracked source/test changes by validator:** none.

## Evidence

Reused verification recorded in `task-4-web-review-fix-r1-report.md` at the same code SHA:

- `pnpm --filter @weknora/web exec node --import tsx --test src/career/OpportunityPage.test.tsx src/routes.test.ts src/router.test.tsx` — **PASS**, 43 passed, 0 failed.
- `pnpm typecheck:web` — **PASS**.
- `pnpm test:web` — **PASS**, 2,341 passed, 0 failed, 0 cancelled.
- `pnpm build:web` — **PASS**, completed in 16.22 seconds; existing CSS import ordering, `calc()` spacing, and chunk-size warnings recorded.
- `git diff --check` — **PASS**.

Read-only revision checks:

- `git rev-parse HEAD` — `4e60423670bd1e398775a7223429e6f31f91e932`.
- `git show --stat --oneline d2ab0698c26ca0f3595678afe7d900b33a137549` — assigned commit changes only Career UI component, its tests, and local CSS.
- `git diff --quiet d2ab0698c26ca0f3595678afe7d900b33a137549 HEAD -- apps/web/src/career/OpportunityPage.tsx apps/web/src/career/OpportunityPage.test.tsx apps/web/src/career/opportunity.css` — **PASS**, no UI source/test drift after assigned code SHA.

## Findings by focus

- **Stable saved-page re-evaluation:** tests open the fixed Opportunity evidence and prior Evaluation detail, evaluate against the same opportunity/snapshot IDs with a fresh request ID, show a distinct new evaluation link, and retain the historical profile revision and `ineligible` conclusion. The route continues to address each immutable result by evaluation ID.
- **Immediate conversation status:** the saved-JD card renders an `ineligible` alert before the links/actions, and renders `unknown` as a separate “待确认” state. It does not label unknown as eligible.
- **Unknown receipt and duplicate clicks:** focused tests show uncertain outcome recovery queries the original request ID and retains the recovered result; rapid repeated clicks while a request is pending issue one request. The page action uses a synchronous in-flight ref guard and the original request ID for uncertain retry/reconciliation.
- **Scope/logout:** the shared saved-page action listens to the current scope signal, clears its latest and historical results when that signal aborts, and fences late API responses using `scopeController.isCurrent`. Existing tests exercise scope clearing on the evaluation detail route and late response fencing on the conversation import UI. There is no dedicated test that switches scope while the new saved-page re-evaluation request itself is pending.
- **Narrow layout:** tests assert the responsive media query and wrapping rules on evidence/detail content. New status/action CSS uses box sizing, `min-width: 0`, and `overflow-wrap: anywhere`; a live narrow viewport was not exercised here.

## Limits / acceptance gaps

- No functional acceptance gap found in the reviewed fix.
- A dedicated pending re-evaluation scope-switch test and an actual browser narrow viewport/keyboard run were not performed. The plan assigns live authenticated browser and narrow layout acceptance to the controller on integrated code; DOM tests and CSS inspection do not prove those runtime behaviors.
- The saved-page action’s duplicate guard and logout/scope listener are established by direct component inspection plus adjacent same-SHA tests, rather than a dedicated end-to-end browser scenario.
