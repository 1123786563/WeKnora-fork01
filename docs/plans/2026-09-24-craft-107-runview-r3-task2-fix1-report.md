# T01 RunView R3 Task 2 fix1 report

## Scope and rulings

Closed the four Medium findings from `2026-09-24-craft-107-runview-r3-material-task2-review.md` within the owned files `internal/container/craft_runtime.go` and `internal/container/craft_runtime_test.go`:

1. Workspace tenant/owner/session association is now verified before the empty-manifest return; added revoked-workspace/empty-manifest regression coverage.
2. File publication now uses same-directory `Linkat` hard-link creation, which atomically fails with `EEXIST` instead of replacing a target. The writer retains the temporary file descriptor, verifies the published inode, unlinks the temporary name, and syncs the directory. Rollback records device/inode identity and removes a file only when the path still names that inode. Created digest directories are also removed only when their captured identity still matches.
3. Distinct refs sharing the same canonical SHA/name path are rejected as ambiguous before FileService reads or material writes; exact same-ref/same-content duplicate canonical entries remain idempotent.
4. Existing staged files must be regular, single-link, exact mode `0644`, correct size and digest; executable/special permission bits are rejected on retry.

TDD: added the four finding-specific regressions and observed the expected RED compile failure while the new publication seam/identity fields were not implemented. After implementation all focused tests passed. The competing-writer test creates the target after the pre-publication point and verifies it remains byte-for-byte unchanged; a rollback test replaces a published path with a different inode and verifies the replacement survives.

## Verification

- `go test ./internal/container -run '^(TestStageWorkspaceInputsUsesOnlySelectedTaskSnapshot|TestRunViewInputStaging|TestRunViewInputPublicationDoesNotReplaceRacingTarget|TestRunViewInputRollbackPreservesReplacementInode|TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity))' -count=1` — PASS (`ok`, 2.626s; linker emitted duplicate `-lc++` warning).
- `go test -race ./internal/container -run '^(TestStageWorkspaceInputsUsesOnlySelectedTaskSnapshot|TestRunViewInputStaging|TestRunViewInputPublicationDoesNotReplaceRacingTarget|TestRunViewInputRollbackPreservesReplacementInode|TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity))' -count=1 -timeout=120s` — PASS (`ok`, 4.360s; same linker warning).
- `gofmt -d internal/container/craft_runtime.go internal/container/craft_runtime_test.go` — no output.
- `git diff --check -- internal/container/craft_runtime.go internal/container/craft_runtime_test.go` — PASS, no output.
- Full package cross compile `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go test -c ... ./internal/container` — BLOCKED by unrelated package dependencies (`sqlite-vec-go-bindings/cgo` and DuckDB Darwin bindings excluded with CGO disabled).
- Isolated cross compile of `unix.Linkat` using cached `golang.org/x/sys v0.47.0`: `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 GOPROXY=off go build ...` — PASS. The x/sys Darwin amd64 and arm64 sources both define `Linkat`; current host focused/race tests exercise the Linux implementation. Full Darwin package validation remains an environment limitation.

No production isolation or live mount proof is claimed. Runtime assembly remains fail closed pending verified material resolver wiring.

## Exact checkpoint

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`; HEAD stayed `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit created. Both owned files were uncommitted at fix1 start; the exact full preimage files are stored under `2026-09-24-craft-107-runview-r3-task2-fix1-preimage/`, and exact final files under `2026-09-24-craft-107-runview-r3-task2-fix1-postimage/`.

Preimage SHA-256:
- `craft_runtime.go`: `5de7ed8a5000e886b5d4600ffaa1aade78eab6863b8cd1a615f81608041095b1`
- `craft_runtime_test.go`: `710f85863704677413e37fef8a7621a9a6232c9b0dbafc9df8f77a7de776010c`

Final SHA-256:
- `craft_runtime.go`: `cff467c93c3d432d5bf483e642c8927c275e579fcedc7454c49493e460a343e1`
- `craft_runtime_test.go`: `02ef2d02a7137b94c9be66c229d5941bad9f9fa5f02dce336c2968047a3cd2e9`

`2026-09-24-craft-107-runview-r3-task2-fix1-task-local.patch` is generated from those full preimage files to final source/test files, not from HEAD. The preimage directory also preserves the start staged patch (empty) and cumulative unstaged diff from HEAD for attribution.
