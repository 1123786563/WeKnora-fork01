# T05 authenticated knowledge selection admission — Task 2 report

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- This Task changed the four authorized T05 service/test files below. Existing shared T08 actor-restoration changes in `agent_run_graph.go` and `craft_session.go` were preserved; this Task only changed snapshot retrieval-query validation and Craft snapshot admission/test coverage.
- `git diff --check -- <four files>` passed.

## Behavior

`craftRunSnapshot` now canonicalizes the zero-or-one `CraftRunRequest.KnowledgeScope` into a typed durable selection, with a non-nil empty ID slice for no selection. The selection query is the original authenticated `req.Prompt`. The top-level `DurableRunSnapshot.Query` remains the server-composed model query, including admitted input and base-version guidance. Thus selected KB IDs and raw retrieval query stay server-owned metadata and do not contaminate the model prompt.

The strict snapshot builder/parser accept distinct retrieval and composed model queries, validate that the retrieval query is non-empty and within the Craft prompt bound, and keep the selected KB cardinality/canonical-ID checks. Legacy Craft snapshots without typed selection continue to recover with explicit empty selection; generic snapshots are unchanged. The existing authenticated admission path continues to require live TaskWrite before Run admission and actor-bound request-key replay. The worker-facing `BuildForKnowledgeBases` path continues to check current Task/KB/document access before retrieval; this Task does not move retrieval into admission or claim the worker dispatch recheck/full T05 gate complete.

Tests cover a selected KB, original raw retrieval query versus composed model prompt, explicit empty selection, malformed scope rejection, actor-bound replay, changed-selection conflict, legacy/generic compatibility, malformed typed snapshots, and existing per-KB/document ACL, truncation, and revocation behavior.

## RED → GREEN and verification

RED for the approved raw-query/model-query distinction:

```text
go test ./internal/application/service -run '^TestDurableCraftKnowledgeSelectionSnapshotRoundTrip$' -count=1
```

Failed as expected before the graph change with `durable Craft knowledge selection query must match the admitted prompt` because the retrieval prompt must differ from the model's server-composed query.

GREEN focused snapshot, admission, and T05 ACL/replay tests:

```text
go test ./internal/application/service -run '^(TestDurableCraftKnowledgeSelectionSnapshotRoundTrip|TestDurableCraftKnowledgeSelectionCompatibilityAndGenericSnapshot|TestDurableCraftKnowledgeSelectionRejectsMalformedSelection|TestCraftSessionStartRunFreezesKnowledgeScopeAndActorBoundReplay|TestCraftSessionStartRunAdmitsThroughSubmit|TestCraftSessionStartRunValidatesWorkspaceState|TestCraftT05BuildForKnowledgeBasesScopesSearchAndIntersectsDocumentAccess|TestCraftT05BuildForKnowledgeBasesEmptyTruncatedAndRevokedReplay)$' -count=1
```

Passed: `ok github.com/Tencent/WeKnora/internal/application/service 2.822s`.

Broader package verification was attempted:

```text
go test ./internal/application/service -count=1
```

It produced no output and remained active beyond two minutes (exec session `65930`); it was interrupted to avoid holding shared test resources. This is not counted as a pass. The targeted GREEN suite above completed successfully.

## File hashes

Before hashes are task-entry values from the Task2 admission checkpoint and the immediately preceding reviewed Task1 fix1 report. After hashes are the exact checkpoint contents listed here.

| File | Before SHA-256 | After SHA-256 |
|---|---|---|
| `internal/application/service/craft_session.go` | `7dc59052fefc22a937fd3c77a5844b7b8dca14f6b1a179c89ff2b8938d3e391d` | `f23ab927af22e951ac2194b4ce6f2bf6410b1d83c4f4dbee05f746ace2af79eb` |
| `internal/application/service/craft_session_test.go` | `b9690a1bd0bd5a44a7c13aeae5e3ea268fbd1a161230c5c40740ba8198e410c5` | `f5c747391eff316411f2226e4b9b92fcdea3428f988eeac58b817b002b2ffbee` |
| `internal/application/service/agent_run_graph.go` | `eb070c701046d7b873b95d230bdf5e8b76f4004a0529f0aa330cbe53ac5d3c64` | `5f1fbd79245e0ec633db9f7202fe910aea3a68ff193b34db864f647f00b16fb3` |
| `internal/application/service/agent_run_snapshot_test.go` | `1e05041efba08cef6dff1f22f5f9911cbb9e8b6d80cb40c286b7028412d453bd` | `89108479b379fd7d42e275ce67c225fbe0e5457d765b33d7b3eea4f08247af35` |

## Remaining gates

This Task does not implement Task 3 worker handoff, production Publisher integration, per-Run filesystem read isolation, or full dispatch-time Task/KB/source revalidation. Those remain required before claiming production T05 complete. The independent review of this Task is pending.
