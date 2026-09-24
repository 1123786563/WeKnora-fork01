# Craft Workspace Draft Seed S2 — Task 1 Report

## Scope and result

Implemented the typed, server-only `craft_workspace_seed` snapshot field and canonical validation for explicit empty and selected heads. Snapshot parsing stays compatible with generic snapshots and rows without a seed; a present null or malformed seed is rejected, and the durable Craft execution boundary rejects a Craft run whose seed is missing. The Craft request boundary now rejects every nonempty `BaseVersionID` with `craft.ErrUnsupported` before session lookup, input claims, or Run admission. `craftRunSnapshot` no longer inserts historical-version text into the model query. Existing raw retrieval query, selected input manifest, knowledge selection, and actor checks remain intact.

Updated the actor lease-recovery success-path fixture to include a typed explicit-empty seed. This was a narrow extra test-file change explicitly authorized by the root agent. No repository admission code, migrations, or HTTP request types were changed. No commit was made.

## RED → GREEN and verification

- RED: `go test ./internal/application/service -run 'CraftWorkspaceSeed|CraftRunSnapshotKeeps|CraftSessionStartRunValidatesWorkspaceState' -count=1` exited 1 at compile time because `CraftWorkspaceSeedSnapshot`, `DurableRunSnapshot.CraftWorkspaceSeed`, and the seed execution guard did not yet exist.
- GREEN: `go test ./internal/application/service -run 'Craft.*Seed|Durable.*Snapshot|Craft.*StartRun' -count=1` passed.
- Actor fixture: `go test ./internal/application/service -run '^TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery$' -count=1` passed after attaching the explicit empty seed.
- GREEN + race: `go test ./internal/application/service -run 'Craft.*Seed|Durable.*Snapshot|Craft.*StartRun|ExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery' -count=1` passed; the same command with `-race` passed.
- Broader service run: `go test ./internal/application/service -run 'Craft|DurableRunSnapshot' -count=1` exited 1 only on `TestCraftSnapshotRestoreRefusals` and `TestCraftRestoreIdempotentKey`. Both fail in `snapshotEnv.admitActiveRun` at `AgentRunStore.Admit` with `agent runtime conflict`, before snapshot restore code or this task's seed fence. Reproduced independently with `go test ./internal/application/service -run '^(TestCraftSnapshotRestoreRefusals|TestCraftRestoreIdempotentKey)$' -count=1`; same two failures. Per the root's instruction, these out-of-scope fixtures were left unchanged for separate triage.
- `gofmt -d` on all five task files produced no output; `git diff --check` passed.

## S1 prerequisite

Verified the current integration HEAD is `a5e9195acd6500c085c85d60c852148e7bbbbf34`, matching the S1 Task 2 checkpoint base. Read `2026-09-24-craft-107-workspace-draft-head-s1-task2-review.md`: Spec compliance PASS and code quality PASS with one Low CAS-test gap. The review states this does not certify S2 admission or S3 materialization.

## Checkpoint

- Exact task-local preimages and hashes: `2026-09-24-craft-107-workspace-draft-seed-s2-task1-preimage.tar.gz` and `2026-09-24-craft-107-workspace-draft-seed-s2-task1-pre.sha256`.
- Incremental task patch: `2026-09-24-craft-107-workspace-draft-seed-s2-task1-checkpoint.patch`.
- Machine-readable base, pre/post owned-file hashes, and patch hash: `2026-09-24-craft-107-workspace-draft-seed-s2-task1-checkpoint.json`.
- The checkpoint delta covers five files; `agent_run_graph_test.go` is limited to the explicitly authorized lease-recovery fixture. The S1/T05/T08 shared pre-existing edits are preserved in the preimage and are not presented as this task's delta.
- Index and commit state unchanged.

## Review and remaining risk

Independent Task review is pending with the root agent. Task 2 must remain gated on that review. The full `Craft|DurableRunSnapshot` service selection is not green because of the two separately reproduced admission-fixture conflicts above; no claim is made that the wider service suite passes. Task 1 does not freeze the actual admission head; repository-owned atomic seed selection remains Task 2, and object capture/materialization remains S3.
