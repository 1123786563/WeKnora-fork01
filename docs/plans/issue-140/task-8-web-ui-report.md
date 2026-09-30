# T08 Web Task 2 — Opportunity import and evidence UI

## Interface preflight

Smallest chat integration: added an optional `conversationActionSlot?: ReactNode` to shared `ChatPageProps`. It renders in the active conversation content above chat action cards and in the new conversation view below the title. `ChatRoutePage` owns and passes the Career Opportunity panel; the panel owns import/recovery state and the typed result link. The feature does not use the chat draft, submit, or stream interfaces. The protected evidence route loads by both immutable IDs.

## Wire alignment

Consumed Task 1 client methods `importOpportunity`, `opportunityReceipt`, and `opportunityEvidence`. The imported reference navigates with both `opportunityId` path and `snapshotId` query. No assistant prose is parsed and no source reference is fetched.

## RED → GREEN

Added focused tests first; they failed before implementation because the Career UI module and the protected route did not exist. GREEN coverage exercises exact inert JD import, fixed-ID result URL, ambiguous outcome receipt lookup and same-request/same-input retry, evidence reload with exact plain text and metadata, explicit unknown fields and `needs_review`, scope-switch late-response fencing, forbidden clearing, definite idempotency conflict and new attempt ID, malformed API response recovery, and malformed route IDs.

## Changed files

- `apps/web/src/career/OpportunityPage.tsx`, `OpportunityPage.test.tsx`, `opportunity.css`
- `apps/web/src/chat/ChatRoutePage.tsx`, `chat-route-page-send.test.ts`
- `apps/web/src/main.tsx` (load the dedicated Opportunity stylesheet)
- `apps/web/src/router.tsx`, `router.test.tsx`, `routes.tsx`, `routes.test.ts`
- `packages/views/src/chat/page.tsx` (minimal optional action slot)

## Verification

- `pnpm --filter @weknora/web exec tsx --test src/career/OpportunityPage.test.tsx src/routes.test.ts src/router.test.tsx src/chat/chat-route-page-send.test.ts` — PASS, 35 tests.
- `pnpm typecheck:web` — PASS.
- `pnpm test:web` — PASS, 2328 tests, 0 failures (full run completed before final stylesheet extraction; extraction changed only CSS import location).
- `pnpm build:web` — PASS, Vite built successfully. Existing warnings remain for a PostCSS `@import` ordering warning in the app CSS pipeline, existing invalid `calc()` expressions, and the pre-existing large bundle warning.
- `git diff --check` — PASS.

## Remaining limits

Browser acceptance against the integrated local API/SQLite remains the controller’s responsibility per brief. No browser end-to-end screenshot/protocol evidence was captured in this task. Final code commit SHA is recorded after commit below.

## Commit

- `fbc3afac310a40e2e661722f49fd97318d068dd7` — `feat(web): add career opportunity evidence flow`
- `git diff --check` — PASS before commit.
