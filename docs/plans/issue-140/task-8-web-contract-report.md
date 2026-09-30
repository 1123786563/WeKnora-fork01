# T08 Web Task 1 — Opportunity contract/client report

## Scope and wire precheck

Implemented additive Opportunity contract decoders and client methods in the four assigned source/test files. Existing profile Career contracts and methods were left unchanged.

Read actual integrated Go wire in `internal/modules/career/opportunity.go`, `internal/modules/career/handler.go`, and `internal/router/routes_career.go`:

- `POST /api/v1/career/opportunities/import` accepts JSON `requestId`, `rawText`, optional `sourceLabel`, optional `sourceReference`; success is HTTP 200 with `kind: "opportunity_imported"`, request/opportunity/observation/snapshot IDs, status `stored|needs_review`, and RFC3339 `acquiredAt`.
- `GET /api/v1/career/opportunities/receipt?requestId=...` returns the same receipt JSON for the authenticated owner and Tenant scope.
- `GET /api/v1/career/opportunities/:opportunityId?snapshotId=...` returns the fixed evidence IDs, exact `rawText`, lowercase SHA-256 digest, five extracted values whose state is `known` with a non-empty `value` or `unknown` without a value, source `{kind,label?,referenceId?}`, acquisition time and the same status enum.
- Handler errors retain the normal API client's structured `ApiError` data: `forbidden` 403, `idempotency_conflict` 409, `invalid_request` 400, `not_found` 404, `outcome_unknown` 504 with the same `requestId`; oversize input is `request_too_large` 413. The client does not transform or hide transport errors.
- Backend stores `sourceReference` as provenance; this client only transmits it and never fetches it.

## RED / GREEN

- Contract tests first failed at module loading because `decodeOpportunityEvidence` was not exported yet; after implementing the decoders, the focused tests passed.
- With the API test cases present and the Opportunity methods temporarily absent, the client test failed with `TypeError: api.importOpportunity is not a function` (2 expected failing client cases). Restoring the methods made all focused tests pass.
- One earlier RED attempt was blocked by missing worktree dependency links (`ERR_MODULE_NOT_FOUND: @weknora/contracts`); `pnpm install --offline --frozen-lockfile` initialized only ignored `node_modules`, then the behavioral RED was repeated successfully.

## Verification

Commands run from the task worktree:

- `pnpm exec tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts` — PASS, 9 tests, 0 failures.
- `pnpm typecheck:web` — PASS.
- `git diff --check` — PASS.

Focused tests cover valid/malformed receipts, required IDs, receipt status and time, malformed evidence/snapshot, explicit unknown and known extracted values, literal raw text preservation, digest format, import body and route, URL encoding for receipt/path/query IDs, and blank input IDs.

## Changed files

- `packages/career-core/src/contracts.ts`
- `packages/career-core/src/contracts.test.ts`
- `packages/api-client/src/career.ts`
- `packages/api-client/src/career.test.ts`

## Commit

Local task commit: `efd18d67feaac09d8ee6347df509f537ee0cf10e` (`feat(career): add opportunity api contract`), based on `d830e261dc465ac9b734040922d0979cf7acfffc`.
