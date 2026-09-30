# T10 Task 3 independent Web contract validation

- **Status:** DONE_WITH_CONCERNS
- **Validated revision:** `cdf2cb0e14072ff6e0f06da64e22b6528aba5d4b`
- **Scope:** `contracts.ts` / `contracts.test.ts`, `career.ts` / `career.test.ts`; contract/client only, so browser, responsive, accessibility, loading, and UI interaction states do not apply.
- **Plan/criteria:** Task 3 of `docs/plans/2026-09-24-issue-140-t10-evaluation.md`.

## Evidence

- Same-SHA implementation report at `task-10-web-report.md` records the exact focused command `pnpm exec tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts` passing 12 tests, `pnpm typecheck:web` passing, and `git diff --check` passing.
- I independently ran the focused test command at the exact SHA in a detached worktree after linking workspace dependencies: **12 passed, 0 failed**.
- I independently inspected the receipt/detail decoder and API methods. Three-value statuses are enumerated with no favorable default; unknown remains `unknown`. Receipt discriminator, unrecognized enum values, missing fact revision/confirmation provenance, mismatched profile/snapshot refs, non-quoted/invalid spans, and unexpected score-like fields are rejected. Raw JD/evidence remain strings. API methods use POST `/api/v1/career/evaluations`, GET receipt with encoded `requestId`, and GET detail with encoded ID; blank IDs and invalid pinned revisions are rejected before transport. Existing shared request handling maps non-2xx responses through `errorFromResult` to `ApiError`, preserving timeout/cancel behavior.
- Exact revision command: `git diff --check cdf2cb0e14072ff6e0f06da64e22b6528aba5d4b^ cdf2cb0e14072ff6e0f06da64e22b6528aba5d4b` (exit 0).
- My attempt to independently rerun `pnpm typecheck:web` in the detached validation worktree **failed for environment setup**, with missing web dependencies (`react`, `tdesign-react`, `@weknora/i18n`, CSS declarations). The same-SHA implementation report documents a passing typecheck using its configured temporary dependency symlinks; I reused that evidence.

## Acceptance gaps and risks

- No contract/client acceptance gap found in the assigned cases. Method tests cover create, receipt recovery, immutable detail path, path/query encoding, and body.
- Independent typecheck reproduction is limited by incomplete dependency links in the detached worktree; rely on the recorded same-SHA pass for that gate.
- No live backend/browser behavior was exercised; those are outside Task 3's typed API scope and belong to controller integration / Task 4.
