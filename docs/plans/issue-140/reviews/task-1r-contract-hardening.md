# Task 1R implementation evidence and Review Package

## Scope and checkpoint

- Task: `Task 1R: Harden the versioned Career contract, API surface, and Desk protocol` from `docs/superpowers/plans/2026-09-28-issue-140-task1-contract-hardening.md`.
- Review findings: `.superpowers/sdd/2026-09-28-issue-140-main-port/task-1-review.md`.
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t01-contracts/WeKnora-fork01`.
- Branch: `codex/issue-140-t01-contracts`.
- BASE: `ce7e2f09268271646c2e99b77e9c4d4c03929a71`.
- Implementation HEAD: `a1be4d51b1dbf501d14de3c17c9eaa297b86a4a9` (`fix: harden career contracts and desk protocol`).
- Exact implementation review range: `ce7e2f09268271646c2e99b77e9c4d4c03929a71..a1be4d51b1dbf501d14de3c17c9eaa297b86a4a9`.
- Patch Review Package: `docs/plans/issue-140/reviews/task-1r-contract-hardening.patch.gz` (decompress to obtain the exact unified diff).
- Patch SHA-256: `b13eaca03ec79ea1e3a6cb43cbee004846f74a1dee7a67f82e01c631832624cd`.
- `git diff --check BASE..implementation HEAD`: PASS. The implementation range changes 23 files (944 insertions, 188 deletions). Changes are limited to Career contracts, API client, `career-core`, exports, and tests/fixtures; no apps or backend files changed.

## Findings addressed

1. **High: failed and malformed envelopes.** Added strict `{ success: true, data, requestId? }` envelope decoding, exact envelope keys, failed-envelope rejection, optional read correlation, required write receipts, and request ID matching. A negative test exercises failed envelopes for every exported Career endpoint.
2. **High: nested application/snapshot validation.** Application and immutable snapshot reference parsing now lives in canonical contracts and checks non-empty identifiers, positive safe revisions, SHA-256 digests, exact fields, and nested authority fields. Career profile provenance/confirmation, opportunity completeness/failure state, evaluation evidence/version binding, material/submission, timeline/reminder, search and privacy receipts have strict parsers and valid/hostile fixtures.
3. **High: incomplete API/Desk seam.** `CareerApi` exposes profile, search, opportunity/evaluation, application, material/submission, timeline, reminder, export/delete and request lookup operations at canonical `/api/v1/career/...` paths. `CareerApi` structurally implements the Desk `CareerRemote` interface, verified by a compile-time assembly test that passes it directly to `createCareerDesk` without a second adapter.
4. **High: unknown/conflict/forbidden write protocol.** Desk receipts distinguish applied, unknown, conflict and forbidden outcomes. Unknown recovery looks up the original ID before any retry. Conflicts retain the original intent until explicit rebase; rebase requires a newer current revision, creates a distinct request ID, and stores the new intent before removing the old one. Applied/forbidden intents are removed; unknown intents remain recoverable.
5. **Medium: write request trust boundary.** Every write validates non-empty request ID, positive safe expected revision where required, JSON shape, and recursively rejects tenant/owner/actor fields before transport. Profile facts and material-version references receive DTO validation. No-send tests cover empty ID, invalid revision and authority-bearing inputs.
6. **Medium: DTO variant coverage.** Added strict parsers and shared wire fixtures for all DTO families, including confirmed facts with provenance, source-completeness snapshot, tri-state evaluation evidence, search receipt, application snapshot, immutable material digests, actual/unknown submission version, append-only event, reminder, export and delete receipts. Unknown fields and nested authority are rejected.
7. **Medium: deployment/tenant/actor scope lifecycle.** Every API request, Desk remote call and intent-store operation carries the structured `CareerScope`. Deployment origin is checked against the configured API client; tenant/actor IDs remain local scope metadata and are not sent as authority headers or body fields. Scope changes abort requests, clear only the old scope's pending intents/projection/cursor, and discard late persistence, dispatch, lookup and read results. Deferred-promise tests cover each boundary.
8. **Low: ADR traceability.** The approved `docs/adr/0019-career-desk-shared-cross-client-state.md` is present at this BASE and was followed without changing its invariants. It defines the structured scope, Desk operations, same-ID unknown recovery, conflict rebase and revision-hint behavior.

## TDD and verification

- RED: `pnpm exec tsx --test packages/contracts/src/career/hardening.test.ts packages/api-client/src/career/hardening.test.ts packages/career-core/test/desk-hardening.test.ts` — failed before implementation: missing DTO parser exports, missing API surface/strict-envelope behavior, and missing scoped `remote`/store interfaces. This was the expected feature-missing failure.
- Focused GREEN: `pnpm exec tsx --test packages/contracts/src/career/*.test.ts packages/api-client/src/career/*.test.ts packages/career-core/test/*.test.ts` — PASS, 15 tests, 0 failures.
- Strict implementation type check: `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/contracts/src/index.ts packages/api-client/src/client.ts packages/career-core/src/index.ts` — PASS.
- Strict assembly and behavior test type check: `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions --types node packages/contracts/src/career/hardening.test.ts packages/api-client/src/career/hardening.test.ts packages/career-core/test/desk-hardening.test.ts packages/career-core/test/assembly.test.ts` — PASS.
- Shared regression: `pnpm test:shared` — PASS, 1141 tests total, 1137 passed, 4 skipped, 0 failed.
- Required shared type check: `pnpm typecheck:shared` — still fails only at the accepted baseline errors in unrelated `packages/views/src/chat/mermaid.ts:127` and `:158` (dark-mode literal mismatch and unrecognized `themeVariables`). No files in `packages/views` were changed. The focused Career strict checks above pass.
- `git diff --check BASE..implementation HEAD` — PASS.

## Commit and handoff

- Implementation commit: `a1be4d51b1dbf501d14de3c17c9eaa297b86a4a9`.
- Evidence/report commit is separate and follows the implementation commit.
- The plan's required independent Spec and quality review remains outstanding; this report does not mark Task 1R verified.
