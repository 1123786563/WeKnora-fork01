# T01 RunView R3 material Task 1 fix1 report

**Status:** implementation and focused verification complete; ready for independent re-review. No commit created. This fix addresses findings M1 and L1 from `2026-09-24-craft-107-runview-r3-material-task1-review.md` only.

## Change

Container creation now initializes each generation layout only after the engine confirms there is no existing container and no prior create marker. Before the first engine `CreateContainer`, it writes a private `.layout-identity.json` manifest atomically using a same-directory temporary file, hard-link publication, file sync, and directory sync. The manifest binds the generation and canonical paths to device/inode identities for the generation root and all mount directories (`inputs`, `knowledge`, `output`, and private HOME directories).

When an existing container is resolved or a `MaterialHandle` is requested, verification is read-only. It rejects a missing root/child, symlink, wrong mode, absent/tampered manifest, or changed device/inode. It never recreates a path beneath an existing bound container. A provider restart with an intact manifest and directory tree can re-inspect and resume the same generation.

The focused material tests now cover same-path replacement of the root and input/knowledge/output children, missing root/child, manifest tampering, manifest presence before engine Create, restart with intact identity, and exact-session GET errors, absence, and ID/project/directory mismatch.

## RED → GREEN and verification

- RED: `go test ./internal/container -run '^TestCraftRunViewMaterial(HandleRejectsRecreatedOrMissingBoundDirectories|LayoutIdentityIsDurableBeforeCreateAndAcrossRestart|HandleRejectsForgedStaleAndUnsafeBindings)$' -count=1` failed to compile because the identity manifest/layout path and engine create hook did not yet exist.
- GREEN: the same focused material command passed.
- Provider + material suite: `go test ./internal/container -run '^TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity)' -count=1` — PASS.
- Race check: `go test -race ./internal/container -run '^TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity)' -count=1 -timeout=120s` — PASS.
- Formatting: `gofmt -d internal/container/craft_runview_container_provider.go internal/container/craft_runview_material.go internal/container/craft_runview_container_provider_test.go` — PASS (no output).
- Whitespace: `git diff --no-index --check /dev/null` against each untracked owned source file produced no whitespace diagnostics.

No full package run was repeated for this narrow fix. The previous full package run failed in two assembly tests outside Task 1 ownership; those failures are recorded in the Task 1 report and independent review.

## Exact checkpoint

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`.
Starting/checkpoint HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit created.

The pre-task source hashes below were verified by reconstructing the prior state from the R1 provider checkpoint and R3 Task 1 task-local patch. The fix-only increment is saved in `2026-09-24-craft-107-runview-r3-material-fix1-task-local.patch`; current hashes and patch hash are in the adjacent JSON checkpoint manifest.

- `internal/container/craft_runview_container_provider.go` — pre `2872bb3360bd4f85c52185ae8ea24419089a4b8f4369fedac7ff88fa1d98ae6d`.
- `internal/container/craft_runview_container_provider_test.go` — pre `519da4d8c7a3f1eef02ed92cfb40a5c1c41fce0e0f9a669e851f52c9d8c59ebe`.
- `internal/container/craft_runview_material.go` — pre `676902fc87f2267249c0abc909e3019df1c9d5ee86ef2cd44148e7d514e6a8b9`.

## Assumptions and limits

The configured filesystem must expose stable device/inode identity and support same-directory hard links and directory sync; otherwise creation fails closed. The directory tree and manifest live under the provider's private sandbox root. Fake-engine tests do not prove that a live Linux container retained a particular mounted inode; production engine/mount inspection and end-to-end Run A→B evidence remain unverified.
