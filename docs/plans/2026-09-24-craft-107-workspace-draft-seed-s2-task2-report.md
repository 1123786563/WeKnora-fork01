# S2 Task 2 — atomic Workspace seed admission report

## Result

Implemented repository-owned Craft Workspace seed choice under `AgentRunStore.Admit`'s existing Session writer transaction. The durable `craft_input_manifest` marker selects the Craft path; there is no optional flag a caller can omit. Admission rejects caller-supplied `craft_workspace_seed`, verifies the owner-scoped Workspace/head/revision/files through the S1 repository read, and checks a selected source Run belongs to the same owner and Session and is terminal. The seed and final hash are persisted with the Run and T01 claim transition. Replay checks actor, assistant identity, and the canonical original snapshot excluding only the server-selected seed, then returns the stored Run without rereading the current head.

Lock order retains the Session no-op writer fence, then performs a non-locking single-statement head/revision/file read, then reserves the slot and inserts the Run. S1 Advance may hold the head while awaiting Session; admission does not wait on head, so it cannot form a head/Session deadlock. The Session-fenced winner determines whether Advance commits or sees the active Run and rolls back.

The lease-recovery actor fixture now registers a real empty Workspace and verifies repository-selected seed. A worker test confirms a legacy registered Craft snapshot without a seed fails before model resolution.

## TDD and verification

- RED: `go test ./internal/application/repository -run 'CraftSeed|DraftHeadAdmission' -count=1` initially failed behaviorally: fresh admissions had no typed seed; missing-head, corrupt-head, owner/source denial cases were accepted; and the admission-vs-Advance path admitted without the expected frozen head. An earlier RED attempt caught an unused test local at compile time; it was fixed before the behavioral RED run.
- GREEN: `go test ./internal/application/repository -run 'CraftSeed|DraftHeadAdmission' -count=1` — PASS on SQLite. PostgreSQL subtests were skipped because `TRPC_TEST_POSTGRES_DSN` is unset.
- GREEN: `go test ./internal/application/service -run '^TestExecuteDurableCraftRunRejectsLegacySnapshotWithoutWorkspaceSeed$|^TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery$|Craft.*StartRun' -count=1` — PASS.
- Race: `go test -race ./internal/application/repository -run 'CraftSeed|DraftHeadAdmission' -count=1` — PASS on SQLite; PostgreSQL unavailable.
- Compile: `go test ./internal/application/repository ./internal/application/service -run '^$' -count=1` — PASS.
- Formatting/patch: `gofmt -d` on changed Go files emitted no output; `git diff --check` — PASS.
- Broader `go test ./internal/application/repository ./internal/application/service -count=1` was started then interrupted after roughly 2m30s while both package test binaries were CPU-bound and had emitted no package result. It is inconclusive and not counted as verification.
- PostgreSQL contention, rollback, and migrations could not be exercised because `TRPC_TEST_POSTGRES_DSN` is unset.

SQLite coverage includes empty A; B freezing exact D1 while keeping seed metadata out of query; seed-inclusive stored hash; same-key replay after D2 and database reopen; fresh-key D2 selection; actor/prompt/input/knowledge conflicts; missing head with unchanged Run/slot/claim/version state; owner mismatch; foreign-session source; corrupt digest/missing files; failed Run insert rollback of slot/T01 claim; caller-seed rejection; and admission-vs-Advance serial outcomes.

## Checkpoint

- Base HEAD, preimage hashes, postimage hashes, patch, and archive are recorded in `2026-09-24-craft-107-workspace-draft-seed-s2-task2-checkpoint.json`.
- Patch: `2026-09-24-craft-107-workspace-draft-seed-s2-task2-checkpoint.patch`.
- Postimage: `2026-09-24-craft-107-workspace-draft-seed-s2-task2-postimage.tar.gz`.
- Patch dry-run succeeded against the saved preimage. No commit created.

## Limits and handoff

PostgreSQL behavior remains unverified without its test DSN. The S2 Task1 snapshot restoration fixture conflicts reported separately by the root remain outside this Task2 change. The exact Task2 diff is ready for independent review.
