# T01 R4 Task 2a Fix2 — whole listed-tree revalidation

## Scope and checkpoint

Implemented the assigned Fix2 Task1 in only `internal/container/craft_runtime.go` and `internal/container/craft_runtime_r4_run_source_test.go`. HEAD remained `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit was created.

This is an exact increment from Fix1 postimage SHA-256 `f1acfa7b3b5fabbc0299ff2a9298040514e08ffe688997dd87195d881f47fee1` (runtime) and `c8a05a8fb68a00d4cf1bdb0ba9cb5dc9a106a895a05c07b479a9993cf82146d3` (test). The preimage and postimage copies are saved under `docs/plans/2026-09-24-craft-107-runview-r4-task2a-fix2-checkpoint/`.

Postimage SHA-256: runtime `42f213533e3a5c303c872e4cba66c4dae71a9676b46acac81959632791f84cae`; test `4a05132168b19ec069ddaeb6f5aa4ae004c529f4512ae1a969bb37cc3c0e8098`. `task-local.patch` is an incremental two-file diff from the exact saved Fix1 preimage.

## Changes

- Added a bounded whole-tree verifier that walks every listed directory with no-follow descriptor-relative operations. It checks root and nested directory identity/fingerprint, exact immediate child-name sets, listed child identity, per-directory limit, and global 4,096-entry limit. Enumeration is sorted to make mutation hooks and traversal deterministic.
- Successful List now revalidates the whole tree after the enumeration and Run/material binding checks. A nested sibling changed after the original walker passed it is rejected before the listing is returned.
- Each Read now revalidates the complete listed tree before and after reading, not only the root and ancestors of the requested file. An insert/remove/rename in an unrelated sibling therefore rejects an otherwise unchanged requested file.
- Existing bounds, no-follow checks, regular-file/hard-link restrictions, digest checks, and generation/Run binding remain in place. No publication, draft advance, service wiring, or non-owned file changed.

## TDD evidence

RED was observed before production changes:

`go test ./internal/container -run '^TestCraftRunViewR4RunBoundArtifactSourceRevalidatesEveryDirectory$' -count=1`

It failed as expected in all four cases: list accepted mutation of an already-visited nested directory while visiting a later sibling, and Read accepted insert/remove/rename of an unrelated nested sibling.

## Verification

| Command | Result |
| --- | --- |
| `go test ./internal/container -run '^TestCraftRunViewR4RunBoundArtifactSourceRevalidatesEveryDirectory$' -count=1` | PASS |
| `go test ./internal/container -run '^TestCraftRunViewR4' -count=1` | PASS, 6.98s |
| `go test -race ./internal/container -run '^TestCraftRunViewR4' -count=1` | PASS, 6.22s |
| `go test ./internal/container -run '^$'` | PASS compile-only |
| `gofmt -d internal/container/craft_runtime.go internal/container/craft_runtime_r4_run_source_test.go` | PASS; no output |
| `git diff --check -- internal/container/craft_runtime.go internal/container/craft_runtime_r4_run_source_test.go` | PASS; no output |

Go test output included only the existing duplicate `-lc++` linker warning. The shared package test window was released to R5 after these commands; no full `internal/container` suite or Docker test was run.

## Remaining limit

The verifier detects observed mutation at List and Read boundaries and performs bounded extra walks. POSIX does not provide an atomic recursive directory snapshot. Task2b must still hold its approved terminal/quiescent writer fence throughout candidate collection; this task does not implement that fence and does not claim a live mutable tree is an atomic snapshot.
