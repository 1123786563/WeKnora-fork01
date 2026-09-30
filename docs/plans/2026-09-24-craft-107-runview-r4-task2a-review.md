# T01 RunView R4 Task 2a — independent review

Reviewed 2026-09-24 against the approved Craft web-artifact Spec, ADR-0004/0008/0009, `CONTEXT.md`, the R4 version and Task 2a plans, the Task 2b architecture note, and the exact Task 2a checkpoint. Scope is only `internal/container/craft_runtime.go` and `craft_runtime_r4_run_source_test.go`. This is a read-only review; no source or test file was changed.

## Checkpoint and verification

- Current owned-file SHA-256 values match the checkpoint postimages: `9c1d171a9dbe50b7a9c5132f729dfd0cf987ca538da871676e8d4ea5ab82f7a1` and `73a4f14b2bb74b9a719e78747c61e778b5239fda07e50b7c0018b187d4d84489`. The patch and report hashes match too. Reverse-applying the saved patch to a temporary copy reconstructs the declared runtime preimage `7689c161d494de936965472714723768d57ecee37c1d2b0c9363f53278fb0757`; the new test file becomes empty under `patch -R`, consistent with an added file.
- `go test ./internal/container -run '^TestCraftRunViewR4(RunBoundArtifactSource|ExecuteDoesNotPublish)' -count=1`: PASS. The same command with `-race`: PASS. Both emitted only the existing duplicate `-lc++` linker warning.
- The increment adds a private output source bound to admitted tenant/owner/session/Run/fence and a material handle. It uses `openat` with `O_NOFOLLOW`, canonical regular-file/link-count checks, recorded device/inode/size/digest checks, and revalidates the provider binding before and after list/read. Tests cover wrong Run, shared pointer canary, traversal, symlink/hard link, file and directory replacement, changed epoch/layout/mount, and failed relist.
- The Execute success path no longer invokes `CollectForKind`, `DraftHeadStore.Advance`, or version/default publication. It returns `ErrCraftRunViewRuntimeUnresolved` after a successful inner execution, as the Task 2a partial plan requires. The source is not yet wired into a collector; Task 2b remains necessary.

## Findings

### F1 — Medium — unbounded output entry count permits resource exhaustion

**Evidence:** `walkRunBoundArtifactTree` calls `readRunViewDirEntries`, which performs `ReadDir(-1)` before any per-entry limit, then appends every directory and file to both a map and slice (`craft_runtime.go:1642-1647`, `1682-1685`, `1732-1735`; helper at `1234-1242`). The only aggregate cap is on *bytes* (`1697-1699`), so zero-byte files or empty directories never consume it. The sandbox writer controls output names and can create arbitrarily many such entries. None of the focused tests exercises a count limit.

**Impact:** Listing can consume unbounded process memory, descriptors over the walk, and CPU before the later collector has an opportunity to reject the output. A Run can deny service to other tenants. This also weakens the plan's bounded-source requirement.

**Smallest correction:** Add an explicit total entry/file count cap at the source, and enumerate directories in bounded batches so the limit applies before `ReadDir(-1)` allocates the whole directory. Add a focused over-limit test with empty entries.

### F2 — Medium — listing can omit a file added during the walk

**Evidence:** `walkRunBoundArtifactTree` reads each directory's entry list once (`craft_runtime.go:1642-1647`). Its after checks compare directory device/inode/mode only (`1693-1695`, `runViewDraftDirStatsMatch` at `927-930`); they do not compare the entry set or detect a new child. `ReadSessionFile` validates only a listed path (`1743-1769`). If a writer adds a valid file after a directory was enumerated, listing can succeed with that file absent; no current test covers this mutation.

**Impact:** A candidate assembled from this list could silently omit part of a still-changing Run output, contrary to the R4 plan's extra-file and sealed-output requirements. Per-file digest checks protect listed files but not the completeness of the tree. This becomes user-visible only when Task 2b consumes the source, so it must be resolved before that integration.

**Smallest correction:** Require authoritative writer quiescence at the Task 2b collection boundary, and have the source verify the directory entry sets against its recorded listing before returning a sealed collection. Add a deterministic insertion-during-list test. A directory timestamp alone is insufficient as the sole seal.

## Verdict

**Spec compliance for Task 2a: conditional.** Run/generation path isolation, no-follow regular-file reads, stale epoch and material checks, and absence of premature Version/default publication are supported by code and focused tests. The output set is not yet provably bounded or complete under concurrent writes (F1–F2). This review makes no claim for Task 2b candidate, draft capture, terminal quiescence, Linux mount isolation, or end-to-end A→B acceptance.

**Code quality: changes required before Task 2b integration.** F1 is a direct denial-of-service issue in the standalone source; F2 is a collection-consistency gap at its intended consumer boundary. No critical/high finding was identified in the reviewed increment.
