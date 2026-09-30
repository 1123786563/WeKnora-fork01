# Task 7 Web Implementation Report

## Scope and handoff

Implemented the T07/#147 Web upload and review subtask in the assigned isolated worktree from BASE `85f950529`.

The frontend contract adds `CareerDocumentSource` and `CareerUpload` separately from the existing fact provenance and CareerDesk action receipt. The API client now exposes `career.sources()` and `career.upload(file, fileName, requestId, expectedRevision, signal)`. Browser uploads use multipart `file`, `requestId`, and `expectedRevision`; the shared request path adds no JSON Content-Type, leaving the browser multipart boundary intact. Uploads return a validated source and optional `intake_completed` proposal receipt.

The Career page supports file selection, progress/processing, source versions, missing categories, review flags, exact proposal evidence, individual proposal confirmation/dismissal via CareerDesk, and visible terminal parse failures while retaining confirmed facts. Ambiguous outcomes keep the same file/request ID/revision, poll `/sources`, prevent a second claim while processing, and only permit retry with the original intent. Revision conflicts refresh profile/source state and require a new explicit attempt. Scope changes and forbidden responses clear in-memory file, source, and unknown-outcome state; late source responses are fenced by a scope epoch.

Backend contract review: `internal/modules/career/profile_intake.go` and `internal/modules/career/handler.go` match the assigned brief. In particular, `CareerSource` omits private resource handles and extracted text; upload is `201` for completed intake, `202` for active processing, `200` for terminal replay, and `409` for conflicting request intent. The batch receipt kind is `intake_completed`, with exact optional evidence on proposals.

## Changed files

- `packages/career-core/src/contracts.ts`
- `packages/career-core/src/contracts.test.ts`
- `packages/api-client/src/career.ts`
- `packages/api-client/src/career.test.ts`
- `apps/web/src/career/CareerPage.tsx`
- `apps/web/src/career/CareerPage.test.tsx`

## Verification evidence

Commands run from repository root:

- `pnpm install --frozen-lockfile` — passed; workspace dependencies installed, lockfile unchanged.
- `pnpm exec tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts apps/web/src/career/CareerPage.test.tsx` — passed, 13 tests, 0 failures. Covers multipart fields and missing JSON Content-Type, source/receipt decoding, six resume categories with missing graduation and experience conflict, exact evidence, failure preserving confirmed facts, unknown upload identity/processing lock, revision conflict, and stale scope response clearing.
- `pnpm typecheck:web` — passed.
- `pnpm test:web` — passed, 2,314 tests, 0 failures.
- `pnpm build:web` — passed. Existing CSS `@import` ordering and `calc()` spacing warnings and Vite large chunk warning were emitted; no build failure.
- `git diff --check` — passed.

## Remaining limits

A local live-server browser upload was not run in this subtask; root coordinates the isolated browser acceptance after independent validation/review. No backend/router/migration or unrelated Web files were modified. No push, merge, or deploy performed.
