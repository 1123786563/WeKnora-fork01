# T01 RunView R4 Task 2a — Run-bound output source (partial)

## Scope and status

This is the separately reviewable Task 2a source seam requested by the parent. It changes `internal/container/craft_runtime.go` and adds `internal/container/craft_runtime_r4_run_source_test.go`. It does not edit `craft_artifacts.go`, any H2/R5 fixture, migrations, repositories, or central assembly. HEAD is `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit was created.

**Partial only:** the run-bound source is implemented and tested, but it is not yet passed into the private-field `CraftArtifactService`. Task2b owns candidate collection and post-terminal draft capture. Accordingly, a successful inner Execute returns a fail-closed `ErrCraftRunViewRuntimeUnresolved` until Task2b assembles that seam; it no longer invokes `CollectForKind` on the legacy source, which follows `<workDir>/output` and can publish before T15 promotion. This deliberate stop avoids false candidate publication/current-version changes during the interim checkpoint.

No draft capture or `CraftDraftHeadStore.Advance` call is made. The repository currently requires terminal Run status, an empty session active-Run slot, and no pending tool/delegation writers; the runtime Execute boundary cannot establish those conditions. Parent has reserved that post-terminal/quiescent hook and candidate integration for Task2b.

## Implementation

- `runBoundCraftArtifactSource` binds one `craft.Task`, one opaque `CraftRunViewMaterialHandle`, and the fixed `output/` path. It checks tenant, owner, session, Run key, Workspace ID in the typed durable Run seed, current epoch, and the provider's live material/container/mount identity before and after each list/read.
- Listing opens the verified output directory through descriptor-relative `openat` with `O_NOFOLLOW`. It recursively accepts only canonical regular files (0644, one hard link, bounded size) and directories (0755), rejecting symlinks/devices and unsafe paths. It records device/inode/type/mode/size and a digest of each file's listed bytes.
- Reading requires a prior successful list. It opens each directory/file descriptor-relative with no-follow flags, compares recorded identities across preflight/open/read/path checks, verifies file length and listed digest, then rechecks the Run fence and material handle. A failed relist clears the prior identity snapshot.
- The source uses `material.output` derived from the current verified generation. It does not resolve a prompt-hash path, shared output symlink, sibling Run, HOME, inputs, or knowledge root.
- The runtime success path no longer calls the legacy collector or publishes an artifact version while the candidate/default seam is pending Task2b.

## Verification

| Command | Result |
| --- | --- |
| `go test ./internal/container -run '^$'` | PASS compile-only after fixing host `unix.Stat_t.Mode` width conversions |
| `go test ./internal/container -run '^TestCraftRunViewR4(RunBoundArtifactSource|ExecuteDoesNotPublish)' -count=1` | PASS |
| `go test ./internal/container -run '^TestCraftRunViewR4' -count=1` | PASS |
| `go test -race ./internal/container -run '^TestCraftRunViewR4' -count=1` | PASS |
| `gofmt -d internal/container/craft_runtime.go internal/container/craft_runtime_r4_run_source_test.go` | PASS; no output |
| `git diff --check` | PASS; no output |

Focused behavioral cases cover private Run output list/read; sibling/shared pointer and inputs/knowledge canaries; wrong session/path/Run; changed mount and generation layout; symlink/hard-link rejection; file, same-inode-content, and intermediate-directory list/read replacement; Run epoch changes between list and read; invalid relist invalidating old identities; and Execute refusing legacy collection/publication.

The full `internal/container` package was not run during this checkpoint because the parent requested coordination with the R5/H2 owner. No Docker test was run. After T19's concurrent normal-output compile fix and R5 assembly reconciliation, compile-only, focused R4, and focused race R4 tests were rerun and passed on the current tree. The focused suites are not evidence for real Linux mount isolation or Task2b candidate/default/draft persistence.

An intermediate rerun was briefly blocked first by container assembly signature changes and then by T19 normal-output edits; those owners reconciled their non-owned files. The final commands `go test ./internal/container -run '^$'`, `go test ./internal/container -run '^TestCraftRunViewR4' -count=1`, and `go test -race ./internal/container -run '^TestCraftRunViewR4' -count=1` pass afterward at the same owned-file hashes. No assembly or T19 file was edited here. The full package remains unrun pending coordination with the R5/H2 owner.

## Checkpoint artifacts

- Checkpoint JSON: `docs/plans/2026-09-24-craft-107-runview-r4-task2a-checkpoint.json`.
- Task-local patch: `docs/plans/2026-09-24-craft-107-runview-r4-task2a-task-local.patch`.
- The patch is generated against the Fix2 runtime post-image (reconstructed from its saved artifact and hash-verified); the source test is recorded as a newly added file.

## Remaining work / risks

- Task2b must add the per-run collector/candidate interface and invoke this source only after binding checks, then separately persist terminal/quiescent draft state with writer-fence protection. No current version/default pointer is advanced in Task2a.
- The source is currently a tested adapter but not yet consumed by a collector. Successful RunView Execute is intentionally unresolved until Task2b completes integration.
- Existing `localCraftArtifactSource` remains for legacy wiring/tests, but is no longer called from this RunView `Execute` success path.
