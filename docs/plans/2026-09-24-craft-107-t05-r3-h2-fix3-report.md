# Craft #107 T05/R3 H2 Fix3 Task Report

## Outcome and ruling

The Fix3 plan's requested published `seal.json` mutation is not possible with the current publisher format. Published Run packages contain their payload files (including `manifest.json`) under `knowledge/runs/<RunID>/`; `seal.json` belongs to the private `.craft-knowledge-candidates/<RunID>/<digest>/` candidate representation. Publication moves only the payload directory to the published path and removes the candidate. The Execute verifier intentionally rejects any candidate before calling `Resume`.

Following the parent-approved ruling, the Execute test now mutates the published `manifest.json` bytes after resolver success, leaves the accepted package root exact, and asserts that neither a candidate path nor published `seal.json` exists. `VerifyAcceptedForRun` reaches `Resume`, which rejects the changed package digest with `craft.ErrNotFound`; `inner.Execute` remains at zero. Candidate-only remains its own Execute denial case.

A separate direct publisher test creates a candidate, corrupts its seal digest, removes the published payload, then calls `Resume` and asserts `craft.ErrConflict`. This specifically proves seal parsing/rejection at the publisher resume seam without claiming the Execute verifier inspects candidate seals.

## Verification

- `go test ./internal/container -run 'TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver|TestCraftKnowledgePublisherResumeRejectsCorruptCandidateSeal|TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart|TestCraftRunViewContainerProviderRestartsSameContainerWithPersistentPrivateHome|TestCraftRunViewMaterialLayoutIdentityIsDurableBeforeCreateAndAcrossRestart' -count=1` — PASS.
- `go test -race ./internal/container -run 'TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver|TestCraftKnowledgePublisherResumeRejectsCorruptCandidateSeal|TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart|TestCraftRunViewContainerProviderRestartsSameContainerWithPersistentPrivateHome|TestCraftRunViewMaterialLayoutIdentityIsDurableBeforeCreateAndAcrossRestart' -count=1` — PASS.
- `git diff --check -- internal/container/craft_knowledge_runview_h2_test.go` — PASS.

## Checkpoint

Only `internal/container/craft_knowledge_runview_h2_test.go` changed. Exact pre/post hashes, unified patch and verification commands are recorded in [checkpoint.json](2026-09-24-craft-107-t05-r3-h2-fix3-checkpoint/checkpoint.json). Snapshot: `2026-09-24-craft-107-t05-r3-h2-fix3-checkpoint/postimage/`.

Independent review is pending. H3 remains gated until the parent records the verdict.
