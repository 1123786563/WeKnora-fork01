# Craft #107 T19 normal-output Task 3 Fix1 — independent review

Date: 2026-09-24. Read-only re-review of T3-1 against the approved Craft web Artifact Spec, Task 3 plan and review, and the Fix1 Task 1 brief. No OCR, Docker, source/test edit, commit, or remote issue action.

## Verdict

- **Scoped Spec compliance: PASS.** A provider start error with otherwise complete-looking process, transport, and sealed-output evidence now yields `RemoteOperationUnknown`, exact receipt, `OutputComplete=false`, `Output.Partial=true`, and partial transport. Claimed replay remains observation-only and does not issue another create or attach. The successful one-start path remains complete.
- **Code quality: PASS for T3-1.** The source change is a single error guard in the completeness predicate, with a focused regression that was RED before the change and GREEN after. No new blocking finding was identified.
- This verdict closes T3-1 only. Production routing and joined budget/build proof remain downstream gates; the service remains unrouted.

## Evidence

`CraftDockerNormalExecService.Execute` (`internal/application/service/craft_docker_normal_exec.go:193-207`) requires `startErr == nil` before clearing `Output.Partial`. On an error it still returns the original receipt and `RemoteOperationUnknown`, and downgrades complete-looking transport to partial. `TestCraftDockerNormalExecAttachResponseLossIsUnknownAndNeverResent` (`craft_docker_normal_exec_test.go:284-314`) supplies a nonnil start error with positive start, terminal success and complete transport; it asserts unknown error, exact exec ID, incomplete/partial output, partial transport and one start after replay. It checks the exec ID rather than the whole receipt in this narrow test; full-receipt persistence and replay belong to the previously reviewed coordinator tests. The existing success test asserts complete, nonpartial output and one create/start.

I independently ran `go test ./internal/application/service -run '^TestCraftDockerNormalExec(AttachResponseLossIsUnknownAndNeverResent|ComposesOneClaimedAttachAndDurableOutput)$' -count=1`: PASS. The worker report additionally records a passing SQLite/isolated PostgreSQL projection and full focused race run. No physical Docker rerun was needed for this result-classification fix; the prior Task 3 physical proof remains the bounded evidence for one actual attach, no-egress setup and cleanup.

## Exact incremental provenance

Current source/test SHA-256 hashes are `651f4931f67b9e057a27d38344b04d3ce8ba97ffc3067b1b49da830e0745ab90` and `de601e588e527a3b3bc3033b6e3ed7a5ad544f70cca1a1017d0819a1a1796f91`, matching the Fix1 checkpoint. I reversed the one source predicate insertion and the focused test fixture/assertion changes in memory; the resulting hashes exactly match the prior reviewed Task 3 postimage, `de0236971e91f92da17fdc8eb44d87333706f2c7f2590b80e5d208213dc5e430` and `24718949d5c56a96b20362614cea4f1414579e2c8eb11b1de2a84e2f2c64c31d`. The Fix1 report hash matches its checkpoint (`8bf0da577350c35406ede9c0c03b67bbc1d727f0d7470887616aa3732729abb5`). This establishes the exact two-file uncommitted increment rather than an empty `BASE..HEAD` diff. The original files were new at Task 3 start; their complete bytes were reviewed in the parent Task 3 review.
