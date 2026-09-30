# T19 Normal Output Task2b Fix1 Task1 Report

## Scope

Implemented the sole Task 1 from `2026-09-24-craft-107-t19-normal-output-task2b-task1-fix1-plan.md` to address review findings T19-NI-1 and T19-NI-2. Production and test changes are limited to the legacy send-claim repository, normal-input repository, focused repository tests, and SQLite 125/PostgreSQL 204 up migrations. No coordinator, provider, output store, R4/R5 implementation, or production routing files were changed. No commit was created.

## Changes

- Legacy `Bind` and `Claim` now check for an existing normal-input stage inside their transaction, after acquiring the same Run transition lock used by `Stage`, and before journal mutation. The normal-specific APIs remain allowed to process their own stage.
- Added a 2 MiB exact canonical request byte limit checked before marshaling/encryption allocation. The byte counter matches compact `encoding/json` escaping, including HTML characters, U+2028/U+2029 and invalid UTF-8 behavior. Command and environment count are bounded too.
- Added a compatible maximum encrypted blob size and enforce it before decryption and during recovery. SQLite 125 and PostgreSQL 204 up migrations constrain ciphertext length to the same bound. Exact maximum canonical input remains accepted; over-limit command/environment requests fail before key lookup/encryption and are not inserted.
- Added deterministic transaction-interleaving coverage for Stage racing legacy Bind and Claim, plus exact-boundary, escaping, oversized persisted ciphertext and zero-length identity coverage.

## TDD and verification

- RED log: `2026-09-24-craft-107-t19-normal-output-task2b-fix1-task1-red-interleave.txt`. The test was run against the old pre-lock Bind/Claim code on SQLite and isolated PostgreSQL; both outcomes failed the new expected conflict assertion (Bind returned nil; Claim reached the later journal check). The fixed code was restored after this controlled RED run.
- Focused command: `TRPC_TEST_POSTGRES_DSN='<dedicated local PostgreSQL 17 DSN>' go test ./internal/application/repository -run '^TestCraftDocker(NormalInput|LegacyBindAndClaimRecheckStageAfterRunLock)' -count=1 -v` — PASS, SQLite and isolated PostgreSQL. Output: `2026-09-24-craft-107-t19-normal-output-task2b-fix1-task1-test-output.txt`.
- Race command: `TRPC_TEST_POSTGRES_DSN='<dedicated local PostgreSQL 17 DSN>' go test -race ./internal/application/repository -run '^TestCraftDocker(NormalInput|LegacyBindAndClaimRecheckStageAfterRunLock)' -count=1` — PASS (`ok .../internal/application/repository 16.358s`). Output: `2026-09-24-craft-107-t19-normal-output-task2b-fix1-task1-race-output.txt`.
- The focused PostgreSQL test uses its isolated unique schema, minimal parent tables and actual migration 204 up/down/up; it is evidence for this migration/store path, not a full project migration-chain claim.
- `gofmt -d` on changed Go files produced no output. `git diff --check` on changed Go files and whitespace checks for both ignored SQL up migrations passed.

## Exact checkpoint

`2026-09-24-craft-107-t19-normal-output-task2b-fix1-task1-checkpoint.json` records base HEAD, eight owned file pre/post hashes, evidence hashes, and the incremental patch hash. The patch `2026-09-24-craft-107-t19-normal-output-task2b-fix1-task1-task-local.patch` is the delta from the prior Task2b Task1 checkpoint, not a full-file addition. It was reconstructed from the S2 Fix1 snapshot, prior Task2b task-local patch applied and verified against all eight prior postimage hashes; applying this Fix1 patch to that exact baseline reproduced all eight current postimage hashes. The four migration files are included in the patch and manifest even though ignored by Git.

## Remaining boundary

The coordinator/physical provider integration remains outside this task. The normal stage and legacy journal exclusion are now serialized at the repository boundary; production routing was not enabled. No unresolved task-local finding is known; independent review is pending.
