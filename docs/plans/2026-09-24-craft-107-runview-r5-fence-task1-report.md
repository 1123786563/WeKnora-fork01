# Craft #107 T01 R5 Fence Task1 Report

## Result

Implemented versioned frozen snapshot identity for Craft admission and propagation to `craft.Task`.

- The repository computes a domain-separated v1 SHA-256 over canonical JSON for the complete final admitted snapshot, after server-owned usage metadata and Workspace seed are present, and persists it atomically with the Craft Run.
- `ClaimDriver` recomputes and verifies the persisted snapshot digest before issuing the Run Fence. Legacy Craft rows with a missing digest, an unknown digest version, or snapshot/digest mismatch fail closed and roll back the lease claim.
- The typed digest travels in `runtime.Run` / `runtime.Fence`, through existing RunFence context propagation, and independently on `craft.Task`. Tool construction and the delegation service reject missing, malformed, unknown-version, or Fence-mismatched identity. The Task request hash also binds the admission digest.
- SQLite migration 000120 and PostgreSQL migration 000199 add nullable version/digest columns; legacy rows are not backfilled.
- No `container.go`, R4, T19 implementation, production feature gate, or image publishing changes were made for this task.

## Ownership expansion and checkpoint

The original Task1 brief did not include `internal/modules/agentruntime/agent/runtime/contracts.go`. Root approved adding that file and focused runtime propagation tests because `AgentRunStore.ClaimDriver` creates the only server-owned Fence and `CraftDelegateTool.Execute` receives it via RunFence context. A later `Get` from tool execution was deliberately not used.

- Preimage HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- At Task start, `internal/application/repository/agent_run.go` already had shared uncommitted changes; its preimage SHA-256 was `19291b4f6aca7ee54e15a6477a37cb53d2888880ae171dc5b44f787b61218262`. The exact pre-existing diff was saved at `/tmp/r5-fence-task1-preexisting-agent-run.diff`.
- At Task start, `internal/application/repository/agent_run_craft_seed_test.go` already existed as an untracked shared file; its preimage SHA-256 was `4b8d4a5a9cd3c0b38d9810c6eed8ba4319295b771a94ee45967646b12b9d1fba`.
- The migration files are ignored by the repository-wide `migrations/` ignore rule. Root requested leaving the shared index untouched and force-tracking them during integration.
- Review patch artifact: `docs/plans/2026-09-24-craft-107-runview-r5-fence-task1.patch`, SHA-256 `290b2c9b189f9ac185f469b590d3d7ba4be2af816474cb479279cfedc8755436`. It contains the Task1 tracked diffs, a delta against the reconstructed pre-existing `agent_run.go`, the new migration/runtime test contents, and the full postimage of the pre-existing untracked seed test file; the latter is included as a file snapshot because its preimage was untracked and is identified by its recorded preimage hash above.
- Postimage file SHA-256 values at checkpoint:

| File | SHA-256 |
| --- | --- |
| `internal/application/repository/agent_run.go` | `c8313d9fda224a37e157dbf653bc289b66bb3724efe3409da1751eb64babb3b7` |
| `internal/application/repository/agent_run_craft_seed_test.go` | `230aff8f28026384c216464957af4b69c8d3537b58ca06ee0465c8be3edb4a73` |
| `internal/modules/agentruntime/agent/runtime/contracts.go` | `7fcbb40e5d70f43f13d9fdd5f2ca2f968c3984393c6a60ae827c3bb0aaf710dd` |
| `internal/modules/agentruntime/agent/runtime/run_fence_snapshot_digest_test.go` | `308e5f7e99d9620008325094b138c8049ae9518480ba868121c290c5cfb8d513` |
| `internal/modules/craft/contracts.go` | `c208e9274bce8ca15a4d7b688d8b6ca0ad5e46e841ebe8f5ed9e007168edcba5` |
| `internal/modules/agentruntime/agent/tools/craft_delegate.go` | `4ef0c4679c61ea311fe74a94a299a296456895af5dd83a91797ca9495e46f47c` |
| `internal/modules/agentruntime/agent/tools/craft_delegate_test.go` | `54ff655a775af503057a78c100e9c184b0bca7020749fe2e4e1525f7b6af198b` |
| `internal/application/service/craft_delegate.go` | `eb73f37dbfa63b249c6fe84a08467a4f88bab56783d7f869fbc8c8380a49d8c2` |
| `internal/application/service/craft_delegate_test.go` | `be5ca5968e568d51d5ec97d2856307323fc4be5cde3347ec7e2b7931bef51272` |
| `internal/application/service/craft_guard_wiring_test.go` | `0d9df399c6389a17c0a4902bb32f66ecb4d4aee024a8451e6d71a6e593a43990` |
| `internal/application/service/craft_recovery_test.go` | `da3ee6a01e7a5d78fe8e9604d795137233c4d57e5c442e3fcd6ac4a98ad81259` |
| `internal/application/service/craft_recovery_process_test.go` | `a22e3b30d91d8b364b785a888f733252201979fbce06d8ecd36a9e4df587f36e` |
| `migrations/sqlite/000120_craft_run_snapshot_digest.up.sql` | `6d79fdd77f218e4c384f0692f23714e207c98a9d72da38181e861201d223227d` |
| `migrations/sqlite/000120_craft_run_snapshot_digest.down.sql` | `8d5f33b63ea73af3bf25d2b79def747cfc43c2a1cbeeb6338ad00d3d1a47bddd` |
| `migrations/versioned/000199_craft_run_snapshot_digest.up.sql` | `6d79fdd77f218e4c384f0692f23714e207c98a9d72da38181e861201d223227d` |
| `migrations/versioned/000199_craft_run_snapshot_digest.down.sql` | `8d5f33b63ea73af3bf25d2b79def747cfc43c2a1cbeeb6338ad00d3d1a47bddd` |

## RED → GREEN evidence

- RED: the new Craft tool tests failed to compile because `runtime.Fence` and `craft.Task` had no digest fields. This established the missing server-owned transport seam.
- GREEN: after adding the typed fields, repository persistence/claim validation, and fail-closed Task construction, the targeted assertions passed.

## Verification

- `go test ./internal/application/repository -run 'Test(CraftAdmittedSnapshotDigestCoversFinalSnapshotFields|CraftAdmissionSnapshotDigestPersistsAndFlowsThroughClaimFence|CraftClaimFenceRejectsLegacyOrMutatedSnapshotIdentity|AgentRunCraftSeedAdmissionFreezesAndReplaysOriginalHead)$' -count=1` — PASS (2.69s). This run covered the repository suite before a concurrent T19 output test file appeared.
- During the concurrent T19 output implementation, an unoverlaid repository rerun was blocked by its incomplete new symbols. A temporary Go overlay initially excluded those in-progress repository files and their service adapter to keep Task1 verification moving. Once T19 symbols landed, the final unoverlaid repository command was rerun on the current postimage and passed (2.506s); this is the final repository evidence.
- `go test ./internal/modules/agentruntime/agent/runtime ./internal/modules/agentruntime/agent/tools -run 'Test(RunFenceContextPreservesAdmittedSnapshotIdentity|CraftDelegateTool|CraftDelegateToolRequestHashCoversGoalInputsSkills)$' -count=1` — PASS. The linker emitted its existing duplicate `-lc++` warning.
- `go test ./internal/application/service -run 'Test(Delegate|CraftDelegate|CraftRecovery|Craft.*Guard|ValidateDelegateTaskRequiresFenceMatchedSnapshotIdentity)' -count=1` — PASS (7.272s).
- `go run /tmp/r5-task1-migrate.go` used the repository's `golang-migrate` SQLite driver on an isolated in-memory DB: full SQLite chain through version 119, then `Migrate(120)`, `Migrate(119)`, `Migrate(120)` — PASS; final version 120, `dirty=false`, digest columns absent after down and present after final up.
- `git diff --check` — PASS.
- `TRPC_TEST_POSTGRES_DSN` was unset, so PostgreSQL runtime migration verification was not available in this environment. The paired PostgreSQL migration files match the verified SQLite column operations and use nullable `INTEGER` / `TEXT` columns.

## Remaining limits

- Independent review is still required before downstream R5 Task2.
- The existing feature gate remains default-off. No production enablement is included.
- PostgreSQL migration up/down/up remains environment-blocked until a test DSN is available.
