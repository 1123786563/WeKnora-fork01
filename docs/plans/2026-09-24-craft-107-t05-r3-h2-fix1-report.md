# Craft #107 T05/R3 H2 Fix1 Task Report

## Outcome

Implemented the H2 Fix1 final knowledge-boundary verification and live provider mount proof. The H1 narrow verifier uses the provider-issued handle, the scoped published record, and the exact accepted Run ID/digest. It verifies the published sealed package at the final dispatch boundary and does not search or republish there. Runtime dispatch rejects an inexact `knowledge/` root, revalidates the accepted package, then re-inspects the live provider binding immediately before `inner.Execute`.

Whole-root verification accepts only `knowledge/runs/<current RunID>` and rejects siblings, extra files, symlinks, missing/candidate-only packages, foreign digests, changed bytes/seals, changed bind source/read-only state, extra mounts, recreated containers, and Run A-to-B carryover. Retry/restart coverage confirms same accepted bytes can be reused while denied cases do not dispatch to the inner executor.

The actual Linux proof now uses the provider-created container, inspects its real bind source/destination/read-only flag, reads the accepted Run manifest in that container, and attempts a root write through the mount. The write fails, the host probe remains unchanged and is removed, and the test confirms the container and private network are gone after cleanup.

## Docker/provider adjustments discovered by live proof

- Docker does not report a created container's network ID/IP until it starts. The adapter now reads `HostConfig.NetworkMode`; provider validation binds the created state to the expected private network mode and performs strict ID/private-IP checks after start.
- Docker's container `Mounts` projection omits configured tmpfs mounts. The test adapter adds the inspected `HostConfig.Tmpfs` entries when projecting engine facts.
- The pinned OpenCode image writes state/cache under XDG locations. The provider now routes those locations into its generation-persisted writable OpenCode data mount so the actual read-only-rootfs provider container starts. This change is covered by exact environment assertions and the live proof.

## Verification

- `go test ./internal/container -run 'TestVerifyRunViewKnowledgeRootRejectsSiblingsFilesAndSymlinks|TestCraftKnowledgeRunViewFinalVerifierRequiresPublishedExactPackage|TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver|TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart|TestCraftRunViewContainerProvider' -count=1` — PASS.
- `go test -race ./internal/container -run 'TestVerifyRunViewKnowledgeRootRejectsSiblingsFilesAndSymlinks|TestCraftKnowledgeRunViewFinalVerifierRequiresPublishedExactPackage|TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver|TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart|TestCraftRunViewContainerProvider' -count=1` — PASS.
- `CRAFT_RUNVIEW_LINUX_MOUNT_TEST=1 go test ./internal/container -run '^TestCraftRunViewProviderCreatedLinuxContainerKnowledgeMount$' -count=1 -v` — PASS against a Linux Docker daemon and pinned image `weknora-craft-runtime@sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`.
- `gofmt -w` on owned Go files — PASS.
- `git diff --check` on owned container files — PASS.

## Scope and checkpoint

No `internal/container/container.go`, T05 service, T01 worker source, commit, or push was performed. Concurrent worktree changes were preserved. The exact postimage files and hashes, task-local patch, and command record are in [checkpoint.json](2026-09-24-craft-107-t05-r3-h2-fix1-checkpoint/checkpoint.json); snapshots are in `2026-09-24-craft-107-t05-r3-h2-fix1-checkpoint/postimage/`. The incremental patch is `2026-09-24-craft-107-t05-r3-h2-fix1-checkpoint/task-local.patch` and is SHA-256 bound by the checkpoint.

Independent review remains pending. H3 production assembly remains out of scope and must wire the provided final verifier seam before enablement.
