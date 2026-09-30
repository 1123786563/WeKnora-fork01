# T19 Normal Output Task2b Task 2 Report

## Scope

Implemented Task 2 from `2026-09-24-craft-107-t19-normal-output-task2b-plan.md`. Changes are confined to `internal/application/service/craft_docker_send_coordinator.go` and its focused test file. No provider, output store, repository production source, R4/R5, container, or routing files changed. No Docker/provider I/O was added and no commit was created.

## Coordinator behavior

- Added a distinct `DockerNormalSendOperation` API. `PrepareNormal` uses the existing S2 Prepare/hold path, verifies the request tenant/Run/activity against the grant, and then stages the exact request through the reviewed normal-input repository before returning it. The coordinator has no provider dependency.
- `StagedInput` returns a defensive copy of the request, preserving nil-vs-empty distinctions for stdin/environment and guarding concurrent read/update of the cached stage.
- Normal `Bind` and `Claim` use the full staged receipt repository methods. The existing outputless `Bind`/`Claim` contract remains unchanged and cannot claim a staged normal operation.
- `ResumeBoundNormal` uses S2's preclaim-only `ResumeBound`, exact-replays the supplied request against the encrypted stage, verifies the complete receipt against persisted journal IDs, and returns no permission for claimed work.
- Closed the interrupted-bind recovery gap: if the S2 journal IDs were committed but the process stopped before full normal receipt fields were written, recovery reconstructs those fields only from the verified immutable stage and exact persisted Docker IDs, then idempotently completes the same receipt bind. This path does not recreate an exec or contact Docker.

## TDD and verification

- Initial API RED: `2026-09-24-craft-107-t19-normal-output-task2b-task2-red.txt` records compilation failing on the missing `PrepareNormal` and `ResumeBoundNormal` APIs.
- Interrupted full-receipt RED: `2026-09-24-craft-107-t19-normal-output-task2b-task2-red-partial-bind.txt` records the controlled SQLite test failing because the journal contained the exact Docker IDs but full receipt fields had not yet been persisted. This test failure motivated the resume repair.
- Focused test command: `TRPC_TEST_POSTGRES_DSN='<dedicated local PG17 DSN>' SYSTEM_AES_KEY='<32-byte test key>' go test ./internal/application/service -run '^TestCraftDockerNormalCoordinator' -count=1 -v` — PASS on SQLite and isolated PostgreSQL. It covers exact stage replay, changed command/timeout/grant binding, staged-input defensive projection, no claim before full bind, stale Run fencing, complete receipt recovery, interrupted receipt-bind recovery, altered receipt rejection, and concurrent one-winner claim. Output: `2026-09-24-craft-107-t19-normal-output-task2b-task2-test-output.txt`.
- Race command with the same DSN/key and selector using `go test -race` — PASS on SQLite and PostgreSQL. Output: `2026-09-24-craft-107-t19-normal-output-task2b-task2-race-output.txt`.
- Outputless S2 regression: `SYSTEM_AES_KEY='<32-byte test key>' go test ./internal/application/service -run '^TestCraftDockerSendCoordinator' -count=1` — PASS. Output: `2026-09-24-craft-107-t19-normal-output-task2b-task2-s2-regression-output.txt`.
- `go test ./internal/application/service -run '^$' -count=1`, `gofmt -d`, and whitespace checks on the two owned files — PASS.
- PostgreSQL tests create a unique schema and run the project migration chain with the unrelated embeddings migration disabled (`app.skip_embedding=true`). The local server lacks the optional vector/search extensions; a schema-local UUID function supports the base migration. This verifies the coordinator behavior against PostgreSQL without claiming optional embedding migration coverage.

## Checkpoint and evidence limitation

`2026-09-24-craft-107-t19-normal-output-task2b-task2-checkpoint.json` records final source hashes and evidence hashes. `2026-09-24-craft-107-t19-normal-output-task2b-task2-task-local.patch` replays successfully from the prior independently reviewed S2 Fix1 snapshot and reproduces both final source hashes.

At task start, the coordinator source hash was `6fabda3c…40658a` and the coordinator test source hash was recorded as `151abf50…bf659b`. The retained reviewed S2 Fix1 snapshot has the same coordinator source hash, but its test source hash is `dfdbe996…9610f7`. The task-start test bytes were not archived before edits; they could not be reconstructed from that recorded hash. The checkpoint therefore distinguishes the captured task-start hash from the stable S2 snapshot used for patch replay and does not claim the patch replays against the unrecoverable differing test preimage. This evidence limitation is disclosed for independent review.

## Remaining boundary

Task 2 provides the typed service seam only. Physical provider execution, durable output projection and production routing remain outside this task and are not enabled.
