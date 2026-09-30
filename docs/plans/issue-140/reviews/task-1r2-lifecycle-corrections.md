# Issue 140 Task 1R2 lifecycle correction evidence

## Status

Implementation complete for this correction wave; Task 1 remains **unverified**, and Task 2 remains blocked pending independent review. This package records implementation evidence, not approval.

## Review package

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t01-contracts/WeKnora-fork01`
- Branch: `codex/issue-140-t01-contracts`
- BASE: `dcb49b344bf051e94f558db2134cadbf45ccbab5`
- Implementation HEAD: `e6a4ce588ddb393956c2bfe24eff6a5dd20c0cc0`
- Implementation commit: `e6a4ce588 fix: close career desk lifecycle review findings`
- Patch: `docs/plans/issue-140/reviews/task-1r2-lifecycle-corrections.patch.gz`
- Patch SHA-256: `9e896f9a2f60c6dc9d3aef9b293174b0a9729bde722fee485ee6150be413c38d`
- Evidence commit is separate from implementation commit.

## Findings addressed

- Career API maps authenticated 403 action/lookup responses to correlated `forbidden`; an action 409 only becomes `conflict` when its details decode as a conflict receipt for the same request ID. Timeouts and malformed responses remain unknown.
- Rebase requires a same-ID authoritative conflict receipt. Timeout and unknown lookup preserve the original pending ID even after a newer open read.
- Same-scope opens keep the highest revision when responses resolve out of order; stale commands are rejected.
- Removed the public non-persisting `submit` method; new commands continue through save-before-dispatch `act`.
- Revision-hint refresh rejections are caught and reported through `onRefreshError`; stale-scope refresh errors are contained. `createWeKnoraClient` now forwards the configured `careerObserver` into its Career API.

## TDD evidence

- Initial `pnpm exec tsx --test packages/career-core/test/desk-hardening.test.ts` before core fixes: **4 failed** (unverified rebase permitted; revision regressed 3→2; submit exposed; refresh rejection escaped).
- The assembled production-client regression was run against the prior API behavior by temporarily disabling only the status mapping: **failed as expected**, reporting `unknown` instead of `forbidden`; the status mapping was restored and committed.
- The newly added timeout/newer-read test confirms the original key remains in the scoped store and can be looked up; the existing authoritative-conflict test confirms rebase then creates a new ID.

## Verification

- `pnpm exec tsx --test packages/contracts/src/career/*.test.ts packages/api-client/src/career/*.test.ts packages/career-core/test/*.test.ts packages/api-client/src/client.test.ts`: **pass**, 38/38.
- `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/contracts/src/index.ts packages/api-client/src/client.ts packages/career-core/src/index.ts`: **pass**.
- `pnpm test:shared`: **pass**, 1,143 passed, 4 skipped (1,147 total).
- `pnpm typecheck:shared`: **fails only at the recorded baseline** in unchanged `packages/views/src/chat/mermaid.ts:127,158` (theme `darkMode` literal mismatch and unsupported `themeVariables`). No unrelated failure was modified.
- `git diff --check dcb49b344bf051e94f558db2134cadbf45ccbab5..e6a4ce588ddb393956c2bfe24eff6a5dd20c0cc0`: **pass**.
- Focused final production seam check for assembled classification and observer forwarding: **pass**, 2/2.

## Scope and limitations

Implementation changes are confined to `packages/api-client/src/career/index.ts`, `packages/api-client/src/client.ts`, `packages/api-client/src/client.test.ts`, `packages/career-core/src/index.ts`, and `packages/career-core/test/desk-hardening.test.ts`. The observer is supplied through the client's `careerObserver` option; the existing transport contract has no server event endpoint or built-in revision stream. A deployment must provide that observer source. No backend/app code was changed. Independent review is still required; do not release Task 2 based on this report.
