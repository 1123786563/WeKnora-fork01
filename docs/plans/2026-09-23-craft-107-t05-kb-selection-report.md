# T05 Typed Knowledge-Base Selection Report

## Checkpoint and scope

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD remained `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit was created.
- At task entry, `craft_knowledge.go` and `craft_knowledge_test.go` already had uncommitted changes, and `craft_knowledge_t05_test.go` was already untracked from earlier T05 rounds. Those inherited edits were preserved. This round extended those files and added this report.
- In-scope files at completion: `internal/application/service/craft_knowledge.go`, `internal/application/service/craft_knowledge_test.go`, `internal/application/service/craft_knowledge_t05_test.go`, and this report. No UI, admission/snapshot, container, runtime, migration, or other agent files were edited by this task.

## Change

- Added explicit `CraftKnowledgeService.BuildForKnowledgeBases`; KB IDs are canonicalized, bounded, unique and sorted before typed digesting. Existing document-ID `BuildForRun` remains unchanged at its public contract and has a separate request digest namespace.
- Each selected KB gets its own `Search` call with exactly one `KnowledgeBaseIDs` entry, the selected KB as the method ID, bounded `MatchCount`, and context-enrichment disabled. Returned document IDs are resolved through the existing shared-aware `Access` port; only documents whose current row is authorized and whose actual `KnowledgeBaseID` matches the selected KB are included. Denied and cross-KB/noisy results are omitted.
- Both paths now share the immutable record and publisher pipeline and package/manifest serialization. The typed path preserves Empty and Truncated facts.
- Same-Run replay remains digest-bound and resumes the accepted package. It reauthorizes recorded documents/KBs through existing authority ports. For selected KBs with no recorded source rows, it re-enters that KB's existing guarded `HybridSearch` path and discards results, so an empty material record cannot imply continuing KB permission.
- The approved Spec's KB selection is now represented by an explicit backend service API, but request snapshot freezing and production worker wiring are still later integration work.

## RED → GREEN and verification

RED command before adding the service method:

```text
go test ./internal/application/service -run 'TestCraftT05BuildForKnowledgeBases' -count=1
FAIL: BuildForKnowledgeBases undefined at the new assertions. The same compile attempt also encountered an unrelated concurrent shared edit at craft_session_acl_test.go:133-134 (undefined err); no source was changed in response to that finding.
```

GREEN focused test command after implementation:

```text
go test ./internal/application/service -run 'TestCraftT05BuildForKnowledgeBases' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/application/service 1.659s
```

Full T05 regression command:

```text
go test ./internal/application/service -run '^TestCraftT05' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/application/service 1.773s
```

Final focused regression after the last callback cleanup:

```text
go test ./internal/application/service -run '^TestCraftT05(BuildForKnowledgeBases|Journey|BuildForRun|SequentialRuns|Excerpt|Publish|Prepared|Retry|Writer|Record|SQLite|SameRun)' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/application/service 1.156s
git diff --check
PASS (no output)
```

The broad package command `go test ./internal/application/service -count=1` was started, but interrupted after 130.826s to avoid maintaining a duplicate high-CPU package run in the shared Worktree; process inspection showed another `service.test` instance was already active. It ended with `signal: interrupt`, so it is not counted as passing evidence. Focused T05 tests and the package compile passed.

New behavior cases in `craft_knowledge_t05_test.go`:

- selected KB only; explicit per-KB filter; no KB/document ID confusion;
- shared-aware document ACL denial and cross-KB returned-document filtering;
- authorized empty KB result, honest truncation, and empty-record replay after KB revocation;
- same-Run replay from accepted immutable package and conflict on changed KB selection digest.

The local fake search in `craft_knowledge_test.go` was adjusted so empty `KnowledgeIDs` means a KB-wide search, matching the production `HybridSearch` contract used by this typed path.

## File hashes at this checkpoint

```text
d2eeb2952a1b3904065c9487a0c47b053a5dafad48286368beabf8b6699653e6  internal/application/service/craft_knowledge.go
7c2683f9a4b8d17175d6ccd09fb9ec355efb217ec0dc25860c59064300144ba5  internal/application/service/craft_knowledge_test.go
a7072bdb720fa3979cb916468d2248f0d297b0e9652897f5914220262b8f4cd5  internal/application/service/craft_knowledge_t05_test.go
```

These file hashes cover the complete current Worktree files, including inherited T05-round edits present before this task. They are not hashes of this task's isolated increment.

## Remaining integration gates

- `CraftRunRequest.KnowledgeScope` is not yet frozen as a typed KB selection in `DurableRunSnapshot`; do not wire this service from mutable request/workspace state.
- `ExecuteDurableRun` / capability assembly / `CraftDelegateService.Delegate` do not yet carry the immutable selection into retrieval or perform final TaskWrite/source-authority dispatch rechecks.
- Container assembly does not yet inject durable records, TaskAccess and an isolated per-Run publisher. Shared `localCraftRuntime.workDir` is not an acceptable read boundary.
- RunView still needs an enforceable isolated runtime filesystem and read-only selected-material mount/copy before Run A/Run B non-readability can be proven.
- This round proves focused service behavior only; it does not claim production T05 or end-to-end completion.
