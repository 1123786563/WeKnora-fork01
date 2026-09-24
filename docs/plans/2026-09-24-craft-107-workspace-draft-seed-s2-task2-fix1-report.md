# S2 Task 2 Fix1 — registered Craft marker fence report

## Result

Fixed the reviewed High finding by making registered Craft Session identity authoritative at admission. `AgentRunStore.Admit` now requires a valid `craft_input_manifest` for every registered Craft admission before input claims are locked or transitioned. Missing, null, wrong-type/malformed, and incomplete manifest values conflict. Caller seed remains rejected; accepted marked Craft runs still select the owner-scoped head under the Session writer transaction and persist the typed seed/hash. Non-Craft generic admission remains unchanged. Valid marked same-key replays still use canonical intent comparison and the stored original seed without rereading the head.

The legacy worker guard test now inserts a historical persisted `agent_runs` row directly; no fresh unseeded Run is admitted for that test.

## TDD and verification

- RED: `go test ./internal/application/repository -run 'RequiresMarkerForRegisteredCraftSession|GenericAdmissionDoesNotRequireCraftSeed' -count=1` failed as expected on `/missing`: Admit returned nil error. Null, wrong-type, incomplete, and generic cases passed against the old code.
- GREEN: `go test ./internal/application/repository -run 'CraftSeed|DraftHeadAdmission' -count=1` — PASS (SQLite). PostgreSQL subtests skip because `TRPC_TEST_POSTGRES_DSN` is unset.
- Service: `go test ./internal/application/service -run '^TestExecuteDurableCraftRunRejectsLegacySnapshotWithoutWorkspaceSeed$|^TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery$|Craft.*StartRun' -count=1` — PASS. The worker test uses a directly persisted legacy Run and asserts failure before model resolution.
- Compile: `go test ./internal/application/repository ./internal/application/service -run '^$' -count=1` — PASS.
- Race: `go test -race ./internal/application/repository -run 'CraftSeed|DraftHeadAdmission' -count=1` — PASS (SQLite).
- `gofmt -d` on changed Go files emitted no output; `git diff --check` — PASS.
- PostgreSQL behavior remains unverified because its DSN is unavailable.

Repository assertions cover no Run/slot/input-claim state transition for each invalid marker, unchanged generic admission, same-key replay after head advance, request/actor fences, and existing rollback/head-concurrency behavior.

## Remaining Low review gap

The retained `TestAgentRunCraftSeedAdmissionSerializesAgainstDraftAdvance` remains scheduler-driven and does not force both transaction orderings. I did not add a production test hook or alter S1 lock ordering in this narrow Fix1 because the repository seams expose no deterministic pause point between Session fencing and head read. This remains an outstanding Low test gap for independent re-review; PostgreSQL contention evidence is also unavailable locally.

## Checkpoint

Base HEAD, exact pre/post hashes, patch and archive are recorded in `2026-09-24-craft-107-workspace-draft-seed-s2-task2-fix1-checkpoint.json`. Patch dry-run succeeds against the saved preimage. No commit created.
