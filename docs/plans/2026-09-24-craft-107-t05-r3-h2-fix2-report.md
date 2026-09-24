# Craft #107 T05/R3 H2 Fix2 Task Report

## Outcome

Closed both Fix1 review proof gaps in focused H2 tests only. No production source files changed.

The restart test now assembles a new `localCraftRuntime`, new `CraftRunViewContainerProvider` with a fresh binding map, fresh material handle obtained inside the new runtime's resolver from the existing provider engine and generation layout, a new knowledge builder/verifier, and a new accepted-record resolver that reloads the published Run record. It confirms exactly one delegate call after reconstruction, the same persisted digest and manifest bytes, no new searches, and no `Save`/`MarkPublished` calls.

The `Execute` denial table now includes (1) a valid candidate package/seal with the published package removed, and (2) a changed candidate `seal.json` after the resolver has returned while the accepted published package remains. Both assert an error and zero `inner.Execute` calls. These mutations run in the existing after-resolver callback.

## Verification

- `go test ./internal/container -run 'TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver|TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart|TestCraftRunViewContainerProviderRestartsSameContainerWithPersistentPrivateHome|TestCraftRunViewMaterialLayoutIdentityIsDurableBeforeCreateAndAcrossRestart' -count=1` — PASS.
- `go test -race ./internal/container -run 'TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver|TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart|TestCraftRunViewContainerProviderRestartsSameContainerWithPersistentPrivateHome|TestCraftRunViewMaterialLayoutIdentityIsDurableBeforeCreateAndAcrossRestart' -count=1` — PASS.
- `git diff --check -- internal/container/craft_knowledge_runview_h2_test.go` — PASS.

## Checkpoint

Exact test file pre/post hashes, unified patch, and command results are recorded in [checkpoint.json](2026-09-24-craft-107-t05-r3-h2-fix2-checkpoint/checkpoint.json). Snapshot path: `2026-09-24-craft-107-t05-r3-h2-fix2-checkpoint/postimage/`. The patch hash is recorded alongside the file hashes.

Independent review is pending. H3 remains gated.
