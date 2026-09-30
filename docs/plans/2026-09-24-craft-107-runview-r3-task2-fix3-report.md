# T01 RunView R3 Task 2 fix3 report

## Implementation

Input publication now uses a platform-specific atomic, descriptor-relative no-replace rename: Linux calls `unix.Renameat2(..., RENAME_NOREPLACE)` and Darwin calls `unix.RenameatxNp(..., RENAME_EXCL)`. Other platforms return an explicit unsupported error. There is no ordinary rename fallback, hard-link publication, or pathname cleanup in the staging path. The operation moves the temporary entry atomically to the final name; no later unlink is used.

The writer records the open temporary inode before publication and verifies that the final entry has the same device/inode, regular-file type, mode `0644`, single link and expected size. A prepublication source-name swap is detected after rename; the moved foreign object is retained and the generation fails closed. Publication errors preserve errno identity; `EEXIST` also maps to `craft.ErrConflict`. File/dir syscall errors are separated from shape conflicts, and close errors are joined without hiding the primary cause.

Created digest directories stay in place on failures. `openOrCreateRunViewInputDigestDirWithOps` injects per-call open/chmod/sync/stat operations for tests; each failure preserves the original syscall error, returns `created=true, fd=-1`, retains the directory, closes a successfully opened descriptor, and skips later operations. Canonical empty digest directories can be completed only after staging reauthorizes the admitted manifest and validates the full tree. Extra or noncanonical entries fail closed.

## TDD and verification

- RED: `go test ./internal/container -run '^TestRunViewInputPublicationDoesNotUnlinkReplacedTemporaryName$' -count=1` failed on the previous Linkat+Unlinkat implementation: the injected concurrent replacement at the temp name was deleted.
- GREEN normal: `go test ./internal/container -run '^(TestStageWorkspaceInputsUsesOnlySelectedTaskSnapshot|TestRunViewInput|TestOpenOrCreateRunViewInputDigestDir|TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity))' -count=1 -timeout=120s` — PASS (`ok`, 8.371s; duplicate `-lc++` warning).
- GREEN race: same pattern with `go test -race ... -count=1 -timeout=120s` — PASS (`ok`, 7.276s; duplicate `-lc++` warning).
- Wrapper cross-compiles with repository x/sys v0.46.0 — PASS for Linux amd64, Linux arm64, Darwin amd64, and Darwin arm64 using the four plan commands.
- Native Darwin primitive tests: `go test internal/container/craft_input_publish_darwin.go internal/container/craft_input_publish_test.go -count=1` — PASS.
- Native Linux primitive tests: cross-compiled Linux/arm64 wrapper+test binary executed with `docker run --rm --platform linux/arm64 -v /tmp/craft-r3-fix3-publish-linux-arm64.test:/test:ro debian:trixie-slim /test` — PASS on the Docker Linux/aarch64 runtime. This verifies the primitive on that Linux filesystem; it is not a full Linux application or mount-isolation test.
- Unsupported wrapper compile: `GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go test -c -o /tmp/craft-r3-fix3-publish-freebsd-amd64.test internal/container/craft_input_publish_unsupported.go` — PASS (compile only; this does not claim package support on FreeBSD).
- `gofmt -d` on all six owned Go files — no output. `git diff --check` on modified tracked files — no output; explicit whitespace scan of all six owned Go files — PASS.
- Inspected staging source: no `Unlinkat`, `Linkat`, or ordinary `Renameat` remains in `craft_runtime.go`; only platform wrapper APIs perform publication.

Focused tests cover: temp-name replacement after publication, target conflicts for regular file/symlink/directory, prepublication source swap, injected `ENOTSUP` with no fallback and no Prompt, postpublication dirty-tree rejection before Prompt, created-directory open/chmod/fsync/fstat errors plus wrong mode, descriptor closure and operation short-circuit, canonical retained-empty-directory completion, and noncanonical/extra directory rejection.

No full Linux package test, live mount test, or production isolation claim is made. The Linux primitive was exercised in Docker; broader runtime/build gates remain with the parent ledger. No dependency files were changed.

## Exact checkpoint

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`; HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit created. Full preimages for existing owned files and the absence list for new wrapper files are saved under `2026-09-24-craft-107-runview-r3-task2-fix3-preimage/`. Full final file contents are in `...fix3-postimage/`. The exact task-local patch is `2026-09-24-craft-107-runview-r3-task2-fix3-task-local.patch`.

Preimage SHA-256:
- `craft_runtime.go`: `1235342dd6ecd458eb1de0251f6ee4f05faf1b5d8825b6fbd5badc75109cb2fe`
- `craft_runtime_test.go`: `55e9621c0536c9b1f859735050b9c3dc326b9962b8abded4999136b1771ebee3`
- New wrapper and primitive-test files were absent at task start, recorded in `preimage/new-owned-files-absent.txt`.

Final SHA-256:
- `craft_runtime.go`: `5af8180bf08ad03158642a00f8f735fa6d71c59171decce7900c20d9b02cf50e`
- `craft_runtime_test.go`: `e93add4c5ba830050cce06708ac595ac540537be05141d8e606e1effddc37f15`
- `craft_input_publish_linux.go`: `cea52bfb33d0ff37fd834cc2c6734fa45a5276512ceebccc7de5bb6a824429ba`
- `craft_input_publish_darwin.go`: `79cde617a7e8db8502044bea3ffae832a8dda3a2d140e2cc81e5198c6fe0c2d5`
- `craft_input_publish_unsupported.go`: `36084e2c602ee5852498f7381584724d37bbabd4b120f9ee48b2581554d60431`
- `craft_input_publish_test.go`: `b9e07f40cc497cf8e36dda803753314e99bf1c0703f325f16f9ea880232ae85c`
