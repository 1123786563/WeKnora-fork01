# T05 Durable Knowledge-Base Selection Task 1 report

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Only Task1-owned source/test files changed: `internal/application/service/agent_run_graph.go` and new `internal/application/service/agent_run_snapshot_test.go`. `agent_run_graph_test.go` was not modified; its fix3 T08 hash stayed unchanged.
- Task-local patch: `docs/plans/2026-09-23-craft-107-t05-knowledge-snapshot-task1-task-local.patch`, SHA-256 `063fccf90976be9c3da77485d9e0551af0eeb3ad418564f025dd08607b017190`.
- `gofmt` applied to the modified Go files; `git diff --check` passed for the tracked graph source.
- No runtime role/model/reasoning metadata was exposed; none is inferred.

## Implementation

Added the typed `CraftKnowledgeSelectionSnapshot{Query, KnowledgeBaseIDs}` to `DurableRunSnapshot` as the server-only `craft_knowledge_selection` field. The Craft snapshot builder now records an explicit empty selection by default; the new `BuildDurableCraftRunSnapshotWithKnowledgeSelection` accepts a typed selection for Task2 admission wiring. It requires the selection query to equal the exact admitted prompt, bounds it to the existing Craft prompt size, canonicalizes selected KB IDs using the T05 KB validator, and stores a copied, sorted selection. The generic builder remains unchanged on the JSON wire because the optional field is omitted.

Strict parsing rejects an explicit null selection, null ID list, malformed selection object, duplicate/noncanonical IDs, invalid IDs, oversized lists, or a selection query that differs from the durable prompt. Old version-1 Craft snapshots with an input manifest but no KB field are interpreted as an explicit empty selection. Empty means no selected KB; it does not select all KBs. The existing T01 input manifest is retained independently. `durableUserMessage` continues to expose only the admitted prompt.

## TDD and verification

RED before source implementation:

```text
go test ./internal/application/service -run '^TestDurableCraftKnowledgeSelectionSnapshotRoundTrip$|^TestDurableCraftKnowledgeSelectionCompatibilityAndGenericSnapshot$|^TestDurableCraftKnowledgeSelectionRejectsMalformedSelection$' -count=1
```

Failed at compile time as expected because the typed selection, builder, and snapshot field did not yet exist (`undefined: CraftKnowledgeSelectionSnapshot`, undefined builder, and missing `DurableRunSnapshot.CraftKnowledgeSelection`).

Snapshot-focused GREEN:

```text
go test ./internal/application/service -run '^TestDurableCraftKnowledgeSelectionSnapshotRoundTrip$|^TestDurableCraftKnowledgeSelectionCompatibilityAndGenericSnapshot$|^TestDurableCraftKnowledgeSelectionRejectsMalformedSelection$' -count=1
```

Passed: `ok github.com/Tencent/WeKnora/internal/application/service 1.266s`.

Combined snapshot and adjacent compatibility/actor regressions:

```text
go test ./internal/application/service -run '^(TestDurableCraftKnowledgeSelectionSnapshotRoundTrip|TestDurableCraftKnowledgeSelectionCompatibilityAndGenericSnapshot|TestDurableCraftKnowledgeSelectionRejectsMalformedSelection|TestDurableRunSnapshotRoundTripKeepsRuntimeFields|TestDurableCraftRunSnapshotKeepsManifestOutOfModelMessage|TestParseDurableRunSnapshotRejectsUnknownVersion|TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor|TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete|TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery)$' -count=1
```

Passed: `ok github.com/Tencent/WeKnora/internal/application/service 5.469s`. The command included both independently named T08 active-Session and soft-deleted-Session regressions, plus worker lease actor recovery.

The RED/GREEN and final focused test commands all used anchored names. No full service suite was needed for this data-only snapshot change.

## File hashes

SHA-256 of complete contents at task entry and this checkpoint:

| File | Before | After |
|---|---|---|
| `internal/application/service/agent_run_graph.go` | `5bc1a7a7ddc882ee5b12dd068fbd96fa4c32c78ba0ef5c106288d413de90e4d0` | `a24824636b4a8a89ff38d55e5f108dfbd8771976c23bcd5d8fd7ac572e6597fa` |
| `internal/application/service/agent_run_graph_test.go` | `cdd3d0e2bdfd26bfcf70cbde454b2c0a5878838eb47e9c5e3043dfafb0f4437a` | `cdd3d0e2bdfd26bfcf70cbde454b2c0a5878838eb47e9c5e3043dfafb0f4437a` |
| `internal/application/service/agent_run_snapshot_test.go` | absent | `68dcacf91f35938e6db717be034ee188a50dbd3beab65c11a7642aad766a0dc4` |

The reconstructed graph-source preimage used for the task-local patch was formatted and verified to hash to the recorded task-entry value before diff generation. Existing concurrent T08 graph behavior is preserved.

## Scope and remaining gates

Task2 is still required to pass authenticated `CraftRunRequest.KnowledgeScope` and prompt into the new builder; this Task does not edit `craft_session.go` or prove admission/replay integration. Worker handoff to `BuildForKnowledgeBases`, current dispatch authorization, production Publisher, and per-Run filesystem read isolation remain later T05 gates. Independent review is pending.
