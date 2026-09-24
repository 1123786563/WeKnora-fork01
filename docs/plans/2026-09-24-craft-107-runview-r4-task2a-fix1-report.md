# T01 R4 Task 2a Fix1 — bounded listing and directory-set validation

## Scope and checkpoint

Implemented the assigned Fix1 Task1 in `internal/container/craft_runtime.go` and `internal/container/craft_runtime_r4_run_source_test.go` only. The task began at HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit was created. The prior Task2a checkpoint owned-file hashes at task start were runtime `9c1d171a9dbe50b7a9c5132f729dfd0cf987ca538da871676e8d4ea5ab82f7a1` and test `73a4f14b2bb74b9a719e78747c61e778b5239fda07e50b7c0018b187d4d84489`.

Postimage SHA-256: runtime `f1acfa7b3b5fabbc0299ff2a9298040514e08ffe688997dd87195d881f47fee1`; test `c8a05a8fb68a00d4cf1bdb0ba9cb5dc9a106a895a05c07b479a9993cf82146d3`. Exact postimage copies are under `docs/plans/2026-09-24-craft-107-runview-r4-task2a-fix1-checkpoint/postimage/`. Checkpoint JSON and patch are adjacent to this report. The patch is the complete owned-file delta from the task worktree HEAD; it includes the already-checkpointed Task2a source baseline in `craft_runtime.go` as well as this Fix1 increment. The prior Task2a checkpoint/patch supplies the preceding checkpoint reference.

## Changes

- Output directory enumeration now uses `File.ReadDir(limit+1)` on an opened directory descriptor, never `ReadDir(-1)`. It rejects more than 1,024 immediate entries in any directory and more than 4,096 entries in the whole tree, counting regular files, empty files, and directories before retaining descendants. The extra entry is only used to prove overflow.
- Each listed directory records device, inode, mode, link count, size, and nanosecond mtime/ctime. List checks the metadata before/after traversal, compares the exact sorted child-name set after recursion, and rejects drift. A deterministic inert-by-default callback seam allows tests to insert an entry exactly after enumeration.
- Each Read reopens the exact output generation with no-follow descriptors, validates the saved directory identities and exact immediate child sets for the root and every ancestor before accepting file bytes, and rechecks those sets and identities after reading. Insertions, removals, and renames after List therefore fail with `craft.ErrConflict`; existing digest, file identity, and no-follow checks remain active.
- No collector, publication, draft advance, or non-owned file was changed.

## TDD and verification

RED was observed before production changes:

`go test ./internal/container -run '^TestCraftRunViewR4RunBoundArtifactSource(BoundsZeroByteAndDirectoryEntries|RejectsDirectorySetChanges)$' -count=1`

It failed as intended: the per-directory 1,025 zero-byte file case returned nil, the 6,144-entry tree of nested empty directories returned nil, and adding a file after listing still allowed a listed file read to return nil error.

After implementation, these commands passed:

| Command | Result |
| --- | --- |
| `go test ./internal/container -run '^TestCraftRunViewR4RunBoundArtifactSource(BoundsZeroByteAndDirectoryEntries|RejectsDirectorySetChanges)$' -count=1` | PASS |
| `go test ./internal/container -run '^TestCraftRunViewR4' -count=1` | PASS |
| `go test -race ./internal/container -run '^TestCraftRunViewR4' -count=1` | PASS |
| `go test ./internal/container -run '^$'` | PASS compile-only |
| `gofmt -d internal/container/craft_runtime.go internal/container/craft_runtime_r4_run_source_test.go` | PASS; no output |
| `git diff --check -- internal/container/craft_runtime.go internal/container/craft_runtime_r4_run_source_test.go` | PASS; no output |

All Go test commands emitted only the existing duplicate `-lc++` linker warning. Full `internal/container` tests were not run because the shared package is concurrently owned by R5/T19; the owners were notified, and R5 independently confirmed its coordinated focused assembly/provider tests pass. No Docker or real mount-isolation test was run.

## Coverage and limits

Focused regressions cover per-directory zero-byte overflow, global overflow from nested empty directories, insertion immediately after root enumeration, nested insertion after List, and insert/remove/rename between List and Read. Existing RunView tests continue covering list/read replacement, symlink/hard-link rejection, wrong Run/session/path, epoch/material identity, and fail-closed Execute behavior.

This source detects drift across its list traversal and before/during reads; POSIX does not provide an atomic recursive directory snapshot. Task2b must consume it while holding the approved terminal/quiescent writer fence. This checkpoint does not claim that directory timestamps alone make a live, concurrently writable tree immutable, and it does not implement or bypass that fence.
