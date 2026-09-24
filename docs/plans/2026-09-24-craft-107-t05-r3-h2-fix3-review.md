# Craft #107 T05 ↔ R3 H2 Fix 3 independent review

Date: 2026-09-24. Scope: Fix 3 test-only delta, its plan/report/checkpoint, the Fix 2 FAIL review, approved `docs/specs/2026-09-23-craft-web-artifact-spec.md`, `CONTEXT.md`, ADR-0004/0008/0009, and the T05/R3 handoff. No OCR or production/source/test edits. This review does not release H3 assembly or assess live-engine mount proof.

## Findings

No blocking findings in the Fix 3 delta.

## Spec and quality verdict

- **Scoped Spec compliance: PASS.** The approved Spec requires selected, authorized Run knowledge and an isolated delegate. The handoff requires exact accepted Run ID/digest/package verification before prompt and an identical package on retry. Fix 3 leaves H2 production behavior unchanged and accurately separates the published payload from the private candidate seal. `Publish` renames only candidate `payload/` to the published Run path and removes the candidate (`craft_knowledge_publisher.go:228-265`); a published `seal.json` is therefore not part of this package format.
- **Fix 3 quality/acceptance: PASS.** `newH2KnowledgeRuntimeFixture` calls the mutation callback only after `BuildForRun` succeeds and before returning the accepted token to `Execute` (`craft_knowledge_runview_h2_test.go:375-387`). The changed-published-bytes case appends a newline to the published `manifest.json`, preserving valid JSON and directory modes; it asserts the exact root remains valid, the accepted-digest candidate is absent, and no published seal exists (`:124-140`). `Execute` checks the root, invokes `VerifyAcceptedForRun`, then would call `inner.Execute` (`craft_runtime.go:230-242`). The verifier reaches `publisher.Resume` after its candidate guard (`craft_knowledge_runview.go:172-185`). `Resume` reads the published files and compares the recalculated digest to the accepted digest, returning `craft.ErrNotFound` on mismatch (`craft_knowledge_publisher.go:306-315, 557-581`). The test asserts that error and zero inner calls (`craft_knowledge_runview_h2_test.go:170-179`). The previous candidate-presence masking defect is removed.
- **Candidate seal is independently covered at its real path.** The direct Resume test constructs the candidate, changes its seal digest, removes the published payload, and calls `Resume` with the accepted digest (`craft_knowledge_runview_h2_test.go:183-205`). With no published directory, `Resume` reads the candidate; `readCandidateSeal` compares the seal digest with the requested digest and returns `craft.ErrConflict` (`craft_knowledge_publisher.go:317-324, 597-627`). This is correctly described as candidate-seal verification, not published-seal verification.
- **Fix 2 restart proof remains intact.** The task-local patch changes only the two mutation/seal tests and leaves `TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart` and `reconstructH2KnowledgeRuntime` unchanged (`craft_knowledge_runview_h2_test.go:208-291`). The latter constructs a fresh runtime, provider, binding map, builder, and accepted-record resolver; the test asserts identical manifest bytes/digest, one inner call, no search, and no save/republish.

## Checkpoint and verification

Only `internal/container/craft_knowledge_runview_h2_test.go` is in the task patch. The exact unified diff reconstructs the checkpoint postimage from the Fix 2 postimage. The Fix 2 preimage SHA-256 is `8db852b9737210516726c900066e9746572de68a10757824ea83780dc90d75f4`; the checkpoint postimage and current live file both hash to `bf57256860ddacf6f736ce99dd7df0e861a61c82794ebf3c845756f4a41e8b9b`; the patch hashes to `bca12c713c04af575090ab2973349fee1b8cd8bb631e9844521c374e397f626c`.

Independently reran the five focused H2 tests in the Fix 3 report with `go test ... -count=1` and `go test -race ... -count=1`: both passed. `git diff --check -- internal/container/craft_knowledge_runview_h2_test.go` passed. Both Go commands printed only the linker warning about duplicate `-lc++`. No OCR was invoked.
