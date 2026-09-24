# T01 RunView R3 Task 2 fix2 report

## Changes

Removed published-file and digest-directory rollback from `stageWorkspaceInputs`. If any write, sync, cancellation, or final validation fails, the generation is retained in place and the error returns directly. A failed `Execute` returns before `ensureWorkspace` and the inner prompt executor. A retry rechecks the current workspace association and admitted manifest, validates the entire existing input tree, then can complete a valid manifest subset; corrupt or extra objects fail closed. A no-replace publication conflict leaves its uniquely named temporary object in the tree as a dirty marker, so a retry rejects that generation rather than deleting an object by pathname.

The created-directory path now handles the helper error before any follow-up operation. The helper closes descriptors on mode/sync/validation errors; no `Fstat(-1)` can mask the original error. A deterministic helper fault test removes the just-created directory and returns a sentinel error, verifying the returned error identity and absence of an orphan.

A deterministic failed-preparation test publishes the first exact input, swaps its target with a concurrent replacement immediately before the second publication fails, and verifies: the replacement remains; the original failure is returned; the inner executor/prompt is never called; and a retry rejects the dirty tree. RED evidence: `go test ./internal/container -run '^TestRunViewInputRollbackDoesNotUnlinkAfterIdentityCheckRace$' -count=1` failed because rollback removed the replacement created at the injected identity-check/unlink interleaving. The final design removes that unsafe cleanup operation entirely.

## Verification

- `go test ./internal/container -run '^(TestStageWorkspaceInputsUsesOnlySelectedTaskSnapshot|TestRunViewInputStaging|TestRunViewInputFailedPreparationKeepsConcurrentReplacementAndBlocksExecution|TestOpenOrCreateRunViewInputDigestDirPreservesCreateFailure|TestRunViewInputPublicationDoesNotReplaceRacingTarget|TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity))' -count=1` — PASS (`ok`, 2.987s; duplicate `-lc++` linker warning).
- `go test -race ./internal/container -run '^(TestStageWorkspaceInputsUsesOnlySelectedTaskSnapshot|TestRunViewInputStaging|TestRunViewInputFailedPreparationKeepsConcurrentReplacementAndBlocksExecution|TestOpenOrCreateRunViewInputDigestDirPreservesCreateFailure|TestRunViewInputPublicationDoesNotReplaceRacingTarget|TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity))' -count=1 -timeout=120s` — PASS (`ok`, 5.010s; same linker warning).
- `gofmt -d internal/container/craft_runtime.go internal/container/craft_runtime_test.go` — no output.
- `git diff --check -- internal/container/craft_runtime.go internal/container/craft_runtime_test.go` — PASS, no output.
- `rg -n 'Unlinkat|Renameat|rollbackRunView' internal/container/craft_runtime.go` — the only remaining `Unlinkat` is for the owned temporary name after successful hard-link publication; no final input or digest directory rollback remains.

No full package, cross-platform package build, live mount, or production isolation claim is made in this fix2 checkpoint. The runtime assembly still fails closed until verified material resolver wiring is supplied.

## Exact checkpoint

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`; HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit created. The full preimage and postimage files are in `2026-09-24-craft-107-runview-r3-task2-fix2-preimage/` and `...fix2-postimage/`; start staged and unstaged patches are also retained in the preimage directory. `2026-09-24-craft-107-runview-r3-task2-fix2-task-local.patch` is the exact preimage-to-postimage delta, not a diff from HEAD.

Preimage SHA-256:
- `internal/container/craft_runtime.go`: `cff467c93c3d432d5bf483e642c8927c275e579fcedc7454c49493e460a343e1`
- `internal/container/craft_runtime_test.go`: `02ef2d02a7137b94c9be66c229d5941bad9f9fa5f02dce336c2968047a3cd2e9`

Final SHA-256:
- `internal/container/craft_runtime.go`: `1235342dd6ecd458eb1de0251f6ee4f05faf1b5d8825b6fbd5badc75109cb2fe`
- `internal/container/craft_runtime_test.go`: `55e9621c0536c9b1f859735050b9c3dc326b9962b8abded4999136b1771ebee3`
