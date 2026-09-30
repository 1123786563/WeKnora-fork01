# S2 Task 1 independent Spec and quality review

## Scope and evidence

Reviewed the approved Craft web artifact Spec (especially lines 64–66), `CONTEXT.md`, ADR-0004/0008/0009, the corrected draft-seed seam, the S2 plan and Task 1 brief/report, the S1 Task 2 independent PASS review, the Task 1 incremental patch and preimages, and the five live owned files. This is a review of **Task 1's typed snapshot and Craft request boundary**, not S2 atomic admission or S3 materialization. No OCR or child agent was used.

The five live file SHA-256 values match `post_sha256` in the Task 1 checkpoint JSON exactly; the incremental patch is `0ad96362d6143f8d5bf70809e6e2360f44e44da9abd7fdc46d4d3ec0b255cd03`, matching the checkpoint. The archive hash is `58173b88f4537e877e28e9e9cc984307fa2c691981707efd2d433dde6e3dc39c`; inspected archived preimages match the recorded pre-hashes. `git diff --check` passed for the five files. Independently ran `go test ./internal/application/service -run 'Craft.*Seed|Durable.*Snapshot|Craft.*StartRun|ExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery' -count=1 -race`: PASS (`ok`, 6.897s).

## Finding

**Low — the legacy Craft execution assertion is at a helper seam, not the worker boundary.** `TestDurableCraftWorkspaceSeedValidationAndExecutionBoundary` in `internal/application/service/agent_run_snapshot_test.go` lines 142–152 invokes `requireCraftWorkspaceSeed` directly for the missing-seed case. It does not admit and execute a marked legacy Craft Run through `ExecuteDurableRun`, where `durableRunActorContext` classifies the row and line 509 applies the guard. The production guard is present and correctly ordered before model resolution, but the test would not catch a future omission or bypass of that call. **Impact:** weaker behavioral proof of the S2 plan's “marked legacy Craft snapshot cannot execute” acceptance; no demonstrated production bypass in this patch. **Smallest correction:** add one service-level worker test with a valid Craft actor/Task fixture, a marked legacy snapshot without `craft_workspace_seed`, and assert execution refuses before model/tool/prompt activity; retain the generic snapshot compatibility assertion.

## Broader test classification

Independently ran `go test ./internal/application/service -run '^(TestCraftSnapshotRestoreRefusals|TestCraftRestoreIdempotentKey)$' -count=1`: FAIL in both cases with `agent runtime conflict` at `craft_snapshot_test.go:333`, from `snapshotEnv.admitActiveRun`, before either restore path. The helper at lines 321–332 admits a Craft Run without `ActorUserID`; current `AgentRunStore.Admit` rejects that at `agent_run.go:318–321`. The `openSnapshotServiceDB` fixture inserts users and sessions but no `tenant_members` rows, which would also fail the subsequent active-actor check. These failures are reproducible, outside the five-file Task 1 delta, and **do not count as passing**. They block a green broader service selection and should be repaired in a separately owned fixture change; add the valid actor and active tenant membership rather than weakening admission.

## Verified boundaries and limits

- `CraftWorkspaceSeedSnapshot.Validate` enforces a nonblank canonical Workspace ID, exact empty revision zero with no source/digest, and selected revision ≥1 with a source Run and lowercase SHA-256 digest. `ParseDurableRunSnapshot` rejects present null or invalid seeds while allowing generic and legacy missing-field bytes to parse. `ExecuteDurableRun` then requires a seed for a classified Craft Task before model/capability resolution. S3 still must verify the selected revision and material objects against authorized Workspace scope; Task 1's structural validation alone cannot do that.
- `StartRun` rejects every nonempty `BaseVersionID`, including whitespace and an existing Version, immediately after request shape validation and before input claims or admission. The test checks unchanged Run and claim row counts. `craftRunSnapshot` no longer injects Version/seed metadata into `Query`; it retains approved input guidance. `CraftKnowledgeSelectionSnapshot.Query` remains the raw prompt at `craft_session.go:993`, preserving T05 retrieval behavior.
- Task 1 leaves actor identity and authorization code intact; the lease-recovery success fixture now supplies an explicit empty seed, and its focused race test passes. The fixture's synthetic Workspace ID proves only the structural guard, not S3 Workspace binding. The request snapshot still has no seed until Task 2 chooses and persists one atomically; no A→B continuation, same-key replay, or admission/head race is certified here.

## Verdict

**Spec compliance: PASS for the bounded Task 1 seam.** The historical-Version request is refused, model query excludes seed/version metadata, generic snapshots remain parseable, and marked Craft execution has a fail-closed guard. This verdict is conditional on Task 2 atomically adding the authorized seed and S3 verifying/materializing it.

**Code quality: PASS with one Low behavioral test gap.** The exact checkpoint and focused race run pass. The broader CraftSnapshot selection is red for the separately classified legacy fixture conflict, so it is not evidence of a green service suite or complete S2 acceptance.
