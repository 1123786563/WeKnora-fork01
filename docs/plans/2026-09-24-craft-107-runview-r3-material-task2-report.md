# T01 RunView R3 Task 2 report — exact selected input staging

## Scope and result

Implemented selected input preparation under the verified RunView generation handle in `internal/container/craft_runtime.go`, with focused behavior coverage in `internal/container/craft_runtime_test.go`. The staging path checks task/material tenant, owner, session and Run identity; obtains only the admitted Run snapshot manifest; verifies the current workspace association before reads; validates source bytes, declared size and SHA-256 before publishing; rejects unsafe names, extra objects, symlinked digest directories/targets, foreign Run handles and corrupt existing files; reuses identical staged objects; and atomically publishes files through no-follow directory descriptors and same-directory temporary files. It leaves the shared runtime work directory untouched. Empty manifests remain empty only when the snapshot contains an explicit typed empty manifest.

`Execute` fails closed with `ErrCraftRunViewRuntimeUnresolved` until runtime assembly injects the verified material resolver. Production assembly is outside this task's ownership and remains an integration prerequisite. This task does not claim Linux mount isolation or production isolation.

## TDD and validation

- RED: focused test invocation initially failed to compile because the updated staging API expected the material handle; after implementation an empty-manifest test caught that its nil Go slice serialized as a missing snapshot field, and the fixture was corrected to serialize an explicit empty array.
- GREEN: `go test ./internal/container -run '^(TestStageWorkspaceInputsUsesOnlySelectedTaskSnapshot|TestRunViewInputStaging|TestRunViewInputStagingVerifiesObjectBytesBeforeWritingAndRollsBack|TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity))' -count=1` — PASS (`ok`, 2.648s; linker emitted duplicate `-lc++` warning).
- GREEN + race: `go test -race ./internal/container -run '^(TestStageWorkspaceInputsUsesOnlySelectedTaskSnapshot|TestRunViewInputStaging|TestRunViewInputStagingVerifiesObjectBytesBeforeWritingAndRollsBack|TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity))' -count=1 -timeout=120s` — PASS (`ok`, 4.462s; same linker warning).
- `gofmt -d internal/container/craft_runtime.go internal/container/craft_runtime_test.go` — no output.
- `git diff --check -- internal/container/craft_runtime.go internal/container/craft_runtime_test.go` — PASS, no output.

The full `internal/container` package suite was not run because concurrent container wiring is in progress; no claim is made for that suite.

## Exact checkpoint

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
HEAD remained `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit created.

These two owned files were already modified when Task 2 started. Captured Task 2 start SHA-256:

- `internal/container/craft_runtime.go`: `a24213ce1a6a288dd58ab04f647dfdd66f52e490c4b17edbaab7d083856a8c46`
- `internal/container/craft_runtime_test.go`: `90d5ba55d1b0fb49cf1b444d29e2e95c693c33ef3ea8a8cfe252e441406def7f`

Final SHA-256:

- `internal/container/craft_runtime.go`: `5de7ed8a5000e886b5d4600ffaa1aade78eab6863b8cd1a615f81608041095b1`
- `internal/container/craft_runtime_test.go`: `710f85863704677413e37fef8a7621a9a6232c9b0dbafc9df8f77a7de776010c`

`2026-09-24-craft-107-runview-r3-material-task2-task-local.patch` records the cumulative current diff of the two owned source/test files from HEAD. Because they had pre-existing modifications at Task 2 start, it is not a delta reconstructed from the captured start content; the start/final hashes above bind the checkpoint and the RED/GREEN changes were limited to the assigned source/test files. The only other files created for this task are this report, checkpoint JSON, and the patch artifact.
