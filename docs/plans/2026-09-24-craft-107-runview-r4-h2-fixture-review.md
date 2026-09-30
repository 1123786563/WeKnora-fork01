# T01 R4 H2 fixture — independent Spec and quality review

## Scope and evidence

Read the approved Craft web Artifact Spec, ADR-0004/0008/0009, `CONTEXT.md`, corrected R4 Fix1 Task 2 brief, R4 Fix2 review, and the H2 report, checkpoint, preimage, patch and live owned test. This review did not run OCR or edit source/tests.

The live file SHA-256 is `923b7009bd15712e9e65dcf455997d1a79e3c833128a5ed73f8391c3ec852219`; saved preimage is `bf57256860ddacf6f736ce99dd7df0e861a61c82794ebf3c845756f4a41e8b9b`; task patch is `2d1463deeafaba5c0eff1655080a547e47178c84780ff182048fedf8b77ca433`. These match `checkpoint.json`, as does report hash `3b602df18bba6a0a39812af2c9ac9a279276a10dab4ec71b04b513fe1c896a8a`; HEAD matches `base.txt` at `a5e9195acd6500c085c85d60c852148e7bbbbf34`. The patch hunks match the live difference from `preimage.go` and touch only `internal/container/craft_knowledge_runview_h2_test.go`.

Independent `go test ./internal/container -run '^(TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver|TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart)$' -count=1` passed. `gofmt -d` and `git diff --check --no-index` on the owned file/preimage were clean. Independent `go test ./internal/container -count=1` failed only in `TestCraftAccessFeatureRegistriesAreAssemblyOwned` (missing `craft.TaskAccessChecker`) and `TestWireCraftInteractionRegistrarRegistersPendingInteractions` (`agent runtime conflict`); neither H2 test failed. The full package gate remains red for those separately owned assembly failures.

## Findings

1. **Low — the report's postimage and patch hashes are stale.** `docs/plans/2026-09-24-craft-107-runview-r4-h2-fixture/report.md:29–30` lists postimage `d5e0b3b0…` and patch `fb958b48…`, whereas the live file and patch are `923b7009…` and `2d1463de…`. The checkpoint and independently computed hashes agree. **Impact:** a reader using the report alone cannot reproduce the stated checkpoint identity, although the saved checkpoint remains internally consistent. **Smallest correction:** update the two report hash lines to the checkpoint values and recompute its hash in `checkpoint.json` after that documentation change.

## Reviewed behavior

The fixture persists positive epoch `1` in `agent_runs`, gives the task fence and runtime Run the same owner/epoch, and saves a delegation row with the exact task JSON and message ID (`craft_knowledge_runview_h2_test.go:332–343, 380–387`). It creates a revision-zero `empty` head and origin for the fixture Workspace, and adds the typed `CraftWorkspaceSeedSnapshot` to the durable Run snapshot (`:344–379`). This is consistent with `CraftDraftHeadStore.ReadRevision`'s explicit origin/head checks and the snapshot parser; production fence and parser code were untouched. The fixture's abbreviated SQLite tables do not test migration constraints, but this Task 2 only repairs the runtime H2 fixture.

The patch does not remove or relax any original H2 assertions. The mutation test still requires the published-byte mismatch to reach the final verifier as `craft.ErrNotFound` and requires zero inner dispatch on every invalid knowledge case (`:111–218`). The retry/restart test still checks accepted digest, no new search or record publication, and identical sealed manifest bytes (`:242–270`). Both now reach those assertions and pass.

## Verdict

**Assigned Task 2 Spec compliance: PASS.** The fixture supplies a durable writer epoch, typed empty D0 predecessor and origin, and preserves the original knowledge assertions without changing production admission.

**Assigned Task 2 code quality: PASS with one low documentation finding.** No blocking correctness, security, concurrency, or data-consistency defect was found in the owned fixture delta. The two independent full-package assembly failures prevent an overall container-package PASS and remain outside this task's file ownership.
