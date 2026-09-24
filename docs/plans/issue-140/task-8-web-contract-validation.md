# T08 Web Contract Validation

- Result: **DONE**
- Validated revision: `efd18d67feaac09d8ee6347df509f537ee0cf10e` (`HEAD` at validation)
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t08-web/WeKnora-fork01`
- Scope: assigned API client and shared contract code only; no source or test edits made.

## Checks

| Command / inspection | Result |
| --- | --- |
| `pnpm exec tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts` | PASS, 9 tests, 0 failed |
| `pnpm typecheck:web` | PASS, exit 0 |
| `go test ./internal/modules/career ./internal/router` | PASS |
| `git diff --check efd18d67^ efd18d67` | PASS |
| `git status --short` before report | clean |
| `git diff --name-status efd18d67^ efd18d67` | Only `packages/api-client/src/career{,.test}.ts` and `packages/career-core/src/contracts{,.test}.ts` modified |

The attempted commands `pnpm --filter @weknora/career-core test` and `pnpm --filter @weknora/api-client test` exited 0 without running tests because those workspace packages define no `test` script. They are not counted as verification; the direct `tsx --test` command above ran the targeted files.

## Contract comparison

- Go registers `POST /api/v1/career/opportunities/import`, `GET /api/v1/career/opportunities/receipt`, and `GET /api/v1/career/opportunities/:opportunityId`; the client uses those methods and paths and passes `snapshotId` as the evidence query parameter.
- `ImportJDInput` JSON tags match the client body fields `requestId`, `rawText`, optional `sourceLabel`, and optional `sourceReference`.
- Go `OpportunityReceipt` JSON tags and discriminator `opportunity_imported` match the decoder fields: request/opportunity/observation/snapshot IDs, status, and acquired timestamp. Go status constants are `stored` and `needs_review`, both accepted by the client decoder.
- `OpportunityEvidence` JSON tags match the decoder including raw text and SHA-256, extracted title/company/location/batch/requirements, source, acquired time, and status. The Go extracted value uses `state` and optional `value`; the client accepts `known` with a nonblank value and `unknown` without a value.
- Recovery and evidence ID inputs are checked for blank values, and request/query/path components are URL encoded. Existing Go Career and router tests passed on the same worktree revision.

## Acceptance / concerns

No contract or typecheck gap found in assigned scope. Runtime loading, empty, error, success UI, accessibility, responsive behavior, and browser interactions belong to the T08 Web feature validation brief and were not evaluated by this contract-only validation.
