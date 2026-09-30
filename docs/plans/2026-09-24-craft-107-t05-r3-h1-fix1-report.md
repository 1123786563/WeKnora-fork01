# H1 Fix 1 report — reverify RunView generation identity

## Finding addressed

The H1 review found that `BuildForRun` accepted a handle after checking only its Run key and path shape. It did not prove that the handle's generation-derived directory and host root still matched the persisted generation layout, or that the directories still had the inodes and modes captured when the handle was issued.

## Implementation

`MaterialHandle` now retains the issuing provider and a private copy of the verified layout identity manifest. A private provider method re-derives the exact generation spec and host layout, checks the handle's in-container directory and every material path against that layout, reruns `verifyGenerationLayout` (directory inode/mode and manifest checks), and compares the current identity bytes with the immutable identity captured at issuance. Symlinks, replaced directories, changed generation paths, and handles not issued by the matching provider fail closed.

The H1 builder invokes this provider verification after durable Run/snapshot and caller checks, immediately before constructing the per-Run Publisher. It then uses the exact verified generation root. No exported provider API, publisher behavior, runtime, central assembly, or other Ticket file was changed.

Focused tests now obtain genuine handles from `CraftRunViewContainerProvider.MaterialHandle`. They prove current handle acceptance, rejection of a changed generation/directory/root, and rejection when the generation root is replaced at the same path and rebuilt with a new otherwise-valid identity manifest.

## TDD and verification

- RED: `go test ./internal/container -run '^(TestCraftKnowledgeRunViewBuilderRejectsMismatchedVerifiedHandleFields|TestCraftKnowledgeRunViewBuilderRejectsRootRecreatedAfterHandleIssuance)$' -count=1` — FAIL as expected. All three mismatched-field subtests and the valid-manifest root replacement reached package build instead of returning `craft.ErrForbidden`.
- GREEN normal and race: `go test ./internal/container -run '^(TestCraftKnowledgeRunViewBuilder|TestCraftRunViewMaterialHandle|TestCraftRunViewMaterialLayoutIdentity)' -count=1 && go test -race ./internal/container -run '^(TestCraftKnowledgeRunViewBuilder|TestCraftRunViewMaterialHandle|TestCraftRunViewMaterialLayoutIdentity)' -count=1 && git diff --check -- internal/container/craft_knowledge_runview.go internal/container/craft_knowledge_runview_test.go internal/container/craft_runview_material.go` — PASS (`ok` normal 3.105s; `ok` race 5.205s; `git diff --check` emitted no output). Both Go test runs emitted the existing duplicate `-lc++` linker warning.
- `gofmt -w` ran on all three owned Go files before the final command.

No full container package test was run for this narrow fix. The prior H1 report records broader package failures in unrelated integration wiring. Independent two-part review is pending root coordination.

## Exact checkpoint

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`; HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit created.

The exact task delta is saved in `2026-09-24-craft-107-t05-r3-h1-fix1-task-local.patch`. The preimage for the H1 adapter and its test comes from the preceding H1 checkpoint's postimage. The provider material file was already untracked at Fix 1 start; its preimage is reconstructed by removing only this fix's provider, layout-identity capture, and verification-method additions, and matches the R3 material Task 1 fix1 checkpoint postimage SHA-256 `29884efdc6b4c1828eb20ce94bc600fbc49720aac505908856a297293a68b128`. Full reconstructed preimages and final postimages are saved under the adjacent `...preimage/` and `...postimage/` directories. See the JSON checkpoint for all per-file hashes.

## Limits

This verifies the provider-issued handle and filesystem layout at the H1 boundary before Publisher construction. It does not add H2 mount inspection, H3 dispatch recheck, or production wiring; those remain separate gates.
