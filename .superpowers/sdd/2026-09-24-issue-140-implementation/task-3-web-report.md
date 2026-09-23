# T03/#141 Career Web frontend report

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t03-web/WeKnora-fork01`
- Branch/base: `codex/issue-140-t03-web` / `2e50f6b1e`
- Brief: `.superpowers/sdd/2026-09-24-issue-140-implementation/task-3-web-brief.md`
- Product facts: approved Spec `docs/specs/2026-09-23-weknora-job-search-design.md`; #141 snapshot `docs/plans/issue-140/issues/issue-141.md`; `CONTEXT.md`; ADR-0015/0017/0018; integrated `internal/modules/career/handler.go`, `internal/router/routes_career.go`, and `packages/career-core/src/contracts.ts`.
- Ownership expansion: parent authorized changes to `packages/api-client/src/errors.ts` and `errors.test.ts` to expose `currentRevision`; no backend wire changes were made.

## Delivered

- Added authenticated career API methods for open/list/changes/receipt/act and connected them to the existing WeKnora client transport.
- Added a scope-bound Career Desk that aborts active requests and discards late reads after user/Tenant scope changes, clears private data on scope change/logout, applies only durable receipts, and reconciles unknown mutations by their original request ID.
- Added reachable `/platform/career` routing and shell navigation.
- Added a TDesign career profile page with confirmed facts and provenance separated from pending proposals, direct confirmation, proposal/confirm/dismiss actions with request ID and expected revision, refresh after conflict/receipt, forbidden/empty/loading/error/unknown states, and responsive auto-fit form/card layout.
- Preserved backend `currentRevision` and `requestId` in typed `ApiError` fields.

## Verification

- RED: `pnpm exec tsx --test packages/api-client/src/errors.test.ts` — failed on the new assertion (`undefined !== 7`) because parser discarded `currentRevision`.
- GREEN: `pnpm exec tsx --test apps/web/src/routes.test.ts packages/api-client/src/errors.test.ts packages/api-client/src/career.test.ts packages/career-core/src/desk.test.ts apps/web/src/platform/platform-shell-nav.test.ts` — 29 passed, 0 failed.
- `pnpm typecheck:web` — passed.
- `pnpm build:web` — passed (`✓ built in 12.95s`). Existing build warnings remain: stylesheet `@import` ordering, CSS `calc()` whitespace, and >500 kB chunks.
- `git diff --check` — passed.
- `pnpm test:web` — ran ~243 seconds, reported 2287 passed, one navigation expectation failure, and two pending-test cancellations (`apps/web/src/agents/agent-editor.test.tsx` and `apps/web/src/settings/GeneralPreferencesPanel.test.tsx`, both “Promise resolution is still pending but the event loop has already resolved”). The navigation expectation was updated for the new Career nav item and all four nav tests passed in the focused rerun. The two hanging full-suite files are outside this task; the full suite was not rerun after fixing that expectation.

## Behavior evidence and limitations

- Desk tests show pending proposals remain out of confirmed facts, durable receipt recovery after unknown mutation, current-revision conflict propagation, forbidden scope failure without cached facts, and delayed response discard after scope change.
- API-client test confirms exact authenticated route paths, query encoding, and action body shape.
- Route/nav tests confirm `/platform/career` resolves and is protected by the existing authenticated Tenant guard without requiring the unrelated Agents capability.
- No real-account browser acceptance was performed; it is assigned to the separate frontend validator gate. Responsive behavior was implemented with wrapping/auto-fit CSS and build/type checks, but not manually inspected in a live browser.
