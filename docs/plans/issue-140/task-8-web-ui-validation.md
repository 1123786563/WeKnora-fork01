# T08 Web UI independent validation

- Status: `DONE_WITH_CONCERNS`
- Revision: `fbc3afac310a40e2e661722f49fd97318d068dd7`
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t08-web/WeKnora-fork01`
- Scope: read-only validation of the assigned #146 Web UI task. No source or test files changed.

## Commands and results

- `git rev-parse HEAD` — PASS; exact assigned revision.
- `git status --short` — PASS; clean before report creation.
- `git show --stat --oneline --decorate --no-renames fbc3afac310a40e2e661722f49fd97318d068dd7` — PASS; reviewed UI, chat slot, route and focused test file scope.
- `git diff --check fbc3afac310a40e2e661722f49fd97318d068dd7^ fbc3afac310a40e2e661722f49fd97318d068dd7` — PASS; no whitespace errors.
- `pnpm --filter @weknora/web exec tsx --test src/career/OpportunityPage.test.tsx src/routes.test.ts src/router.test.tsx src/chat/chat-route-page-send.test.ts` — PASS, 35 tests, 0 failures. Includes import/result path, unknown-outcome receipt lookup and same-intent retry, evidence reload and inert rendering, scope-switch late-response fencing, forbidden clearing, idempotency conflict/new request, malformed wire and route IDs, protected route matching, and chat slot wiring. Existing esbuild warnings about `import.meta` under CJS occurred in unrelated chat tests.
- `pnpm typecheck:web` — PASS.
- Same-revision implementation report `.superpowers/sdd/2026-09-24-issue-140-implementation/task-8-web-ui-report.md` records `pnpm test:web` PASS (2328 tests, 0 failures) and `pnpm build:web` PASS. Its build notes pre-existing CSS `@import` ordering, invalid `calc()` and large bundle warnings. These commands were not rerun because the report identifies this exact code commit and the final CSS extraction was included in it.

## Acceptance and state review

- The chat page provides the Career paste panel through `conversationActionSlot`; successful import displays the typed receipt and a fixed opportunity + snapshot link. Router parsing requires both identifiers and the protected router entry is covered.
- Initial/empty state disables Save until non-whitespace input. Busy state disables edits and announces progress. Definite errors and forbidden responses are announced; forbidden/scope changes clear private JD text. Ambiguous outcomes retain the same request ID and exact input for receipt lookup or replay. Success links to the immutable evidence identifiers.
- Evidence loading uses a status region and `aria-busy`; invalid IDs, unavailable evidence and forbidden scope render distinct alert states, with retry for transient failures. Ready state displays source, acquisition time, review status, unknown extracted values and exact raw JD as React text (not HTML). Tests use malicious-looking `<system>` text and verify it remains text.
- Labels are associated with textarea/inputs; heading relationships, status/alert regions, native buttons/links and visible keyboard focus outlines are present. This is code/test inspection, not a screen-reader audit.
- CSS uses fluid widths, wrapping long IDs/text, scrollable raw-text content and a `max-width: 640px` single-column metadata/header layout. Responsive behavior is inspected from CSS; no actual viewport screenshot was captured.
- Browser-level behavior remains unverified: no real browser E2E or API-backed paste → result → reopen evidence run exists in the task evidence. JSDOM and router tests cover component behavior and route matching, but do not prove browser navigation, rendering/layout, or the live integration. This is the reason for `DONE_WITH_CONCERNS`.

## Acceptance gaps and risks

- No observed source-level acceptance failure in the assigned Web scope.
- Risk/gap: the Issue's requested paste-to-reopen browser E2E evidence is absent. The parent integration controller should close this with a real browser run against the integrated local API, including unknown fields/`needs_review` and scope fencing.
- Validation was limited to Web UI; backend persistence, security and server-side replay semantics are outside this assignment.
