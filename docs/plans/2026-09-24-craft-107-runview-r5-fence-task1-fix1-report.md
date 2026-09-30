# Craft #107 T01 R5 Fence Task1 Fix1 Report

## Result

Closed the reviewed same-Workspace stale snapshot identity gap at both `PrepareTask` and the production material resolver. The resolver now checks the original Task and Fence version/digest against the durable Run after recomputing identity from its snapshot bytes, before RunView allocation, engine calls, or OpenCode API calls. Admission replay now validates the stored digest against the persisted snapshot before returning the prior Craft Run. This is the read-time identity correction only; per-effect claims remain required to close the later TOCTOU window.

## TDD evidence

- RED, before the production changes: `go test ./internal/application/repository -run 'Test(CraftPrepareTaskRejectsChangedDurableSnapshotWithRecomputedDigest|CraftAdmissionReplayRejectsCorruptPersistedSnapshotDigest)$' -count=1` — failed as expected: changed same-Workspace Run was accepted (`PrepareTask` returned nil), and replay of a corrupt persisted digest was accepted (`Admit` returned nil).
- GREEN: `go test ./internal/application/repository -run 'Test(CraftPrepareTaskRejectsChangedDurableSnapshotWithRecomputedDigest|CraftAdmissionReplayRejectsCorruptPersistedSnapshotDigest|CraftPrepareTaskIdempotencyAndConflicts|DelegationIdentitySameKeySamePayloadReplays|CraftWorkspaceGuardStaleFenceCannotPublish)$' -count=1` — PASS (`3.143s`). This includes the stale original Task / changed valid snapshot + recomputed durable-row digest case, corrupt replay case, and positive preparation/replay compatibility coverage.
- GREEN: `go test ./internal/container -run 'TestCraftRunViewProductionAssembly(ResolvesOnlyPersistedBoundTaskRun|RejectsUnboundPersistedRunView|RejectsStaleTaskBeforeSideEffects|RejectsChangedSameWorkspaceSnapshotDigestBeforeSideEffects)$' -count=1` — PASS (`1.895s`). Stale cases assert zero RunView allocate/load, Docker engine, and OpenCode session calls; the positive persisted-bound Run path succeeds. Existing linker warning: duplicate `-lc++` libraries ignored.
- `gofmt -w` on the changed Go files — PASS.
- `git diff --check` on the owned Go paths — PASS.

The targeted package runs compiled their packages at the exact postimage. A full `internal/container` package run was not repeated in this task because repository/service package verification was coordinated with concurrent workers; the focused R5 tests passed after their release.

## Exact checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Preimage HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (no commit created).
- Exact preimage: `2026-09-24-craft-107-runview-r5-fence-task1-fix1-preimage.tar.gz`; manifest `2026-09-24-craft-107-runview-r5-fence-task1-fix1-pre.sha256`.
- Postimage: `2026-09-24-craft-107-runview-r5-fence-task1-fix1-postimage.tar.gz`.
- Task-local delta: `2026-09-24-craft-107-runview-r5-fence-task1-fix1-task-local.patch` (SHA-256 `2ec100cff998fa78711e6c68b633cd73c7fc2a628355fed1797d6748c685a24e`). Patch covers only the six captured owned files below; unrelated shared worktree changes are excluded.

### File hashes

| File | Preimage SHA-256 | Postimage SHA-256 |
| --- | --- | --- |
| `internal/application/repository/agent_run.go` | `c8313d9fda224a37e157dbf653bc289b66bb3724efe3409da1751eb64babb3b7` | `aec1b2be82e5570e38aed2010f854b430c479f4f59717540308c163aebe0848a` |
| `internal/application/repository/craft_workspace.go` | `74fb806ff41c972211a83104d363a5b554c9305bfcdbef1677063f7225bfc6aa` | `5bf01105a33cfaf60473bf99520bc7551e7421012db3e9d04c2b31578f1a4a7c` |
| `internal/application/repository/craft_workspace_test.go` | `4af38ce2b9d3be3af868c0f2e095d012d2e7410d21a5eaa46e5494d9eb2b2443` | `93038ec5f890f95646aeeed165df42716751673d918c388cdf6d3b4ef5870d4b` |
| `internal/application/repository/agent_run_craft_seed_test.go` | `230aff8f28026384c216464957af4b69c8d3537b58ca06ee0465c8be3edb4a73` | `0f9526f3847a201c769e8fc6432e3036b3524239bbfa3344422d8c6b754f86e8` |
| `internal/container/container.go` | `74dd3c54e33906940f8d25e348ac73352f823d514fac2fc57d5edebd4311e855` | `76220b95d4a78b892adc7d133762f1b994282538fc0e1d46fff3f192e3d59de4` |
| `internal/container/craft_runview_assembly_test.go` | `a1e0c0fa3255ce7bd01893ffab084299a806dedb3a89ab7c8bf3969d38aaa055` | `29ed59cb83ea857dd29e16c0c5100a6614c268dc93a7208d38199558fce4cb5c` |

## Remaining limits

- This Task1 check does not make the identity durable through external effects. The planned Task2 effect claims are still required to bind the effect to a live writer fence and prevent TOCTOU races.
- Default-off production behavior and missing-registry-digest fail-closed configuration remain unchanged.
- PostgreSQL runtime behavior was not independently exercised in this Fix1 window; the R5 fixture and repository tests use the shared test helper's dialect coverage where applicable.
