# T01 / #120 fix 1 independent Spec and quality review

**Reviewed scope:** committed delta `43dcff3454d8f7ec70a37a8c06ba3087b7947b98..0c17125fe002f02838841f5133e4f6b9d476fec8` in the T01 worktree, plus its untracked `2026-09-23-craft-107-t01-fix-1-report.md`. I compared the prior independent T01 review, the T01 brief/plan, approved web-artifact Spec and scope design, CONTEXT.md, and ADR-0004/0009. The eight source/test file SHA-256 values match the implementer report; `git diff --check` for the commit range passed. The T01 worktree had only the untracked report. I did not run OCR or duplicate the reported Go suites.

## Verdict

- **Spec compliance: FAIL.** The fixes establish a typed selected-input field in the durable Run snapshot, validate its presence at runtime, classify new associations, and constrain DB row writes to one transaction. However, staging into a persistent shared `inputs/` tree leaves previously staged but currently unselected material accessible, contrary to the approved selected-input boundary and the member's choice to continue without an unrecognized file.
- **Code quality: FAIL.** The persistent-tree leak is a high severity authorization failure. A size-only staging shortcut can also accept altered existing bytes, and durable admission claims have no stale-claim recovery. The focused and full service test passes in the implementer report support the changed paths, but do not cover these remaining cases.

## Findings

### R1 — High: omitted input remains in the shared runtime workspace

**Evidence / affected symbols:** `newCraftRuntimeExecutor` takes one configured OpenCode serve `workDir` and assigns it to every `localCraftRuntime` execution (`internal/container/craft_runtime.go:101-110, 139-150`). `stageWorkspaceInputs` writes each selected input to `e.workDir/inputs/<digest>/<name>` (`craft_runtime.go:378-420`) without clearing that directory or isolating it per Run. `Execute` calls staging before the OpenCode executor (`craft_runtime.go:179-195`). The new test starts with an empty temporary directory and checks only one Run (`craft_runtime_test.go:31-116`).

**Reachable Run 1 → Run 2 path / impact:** `StartRun` accepts Run 1 with `InputRefs=[A]` and freezes A in its snapshot (`craft_session.go:726-727, 762-763, 913`); `Execute` stages A at the shared `workDir/inputs/<digest-A>/<name-A>` (`craft_runtime.go:184, 391-420`). After Run 1 finishes, `StartRun` accepts Run 2 with `InputRefs=[]` (or only B), so its typed snapshot excludes A. `selectedWorkspaceInputs` returns only Run 2's selected manifest (`craft_runtime.go:438-459`), but `stageWorkspaceInputs` performs no deletion of A; with an empty selection it returns immediately (`craft_runtime.go:383-385`). The subsequent OpenCode execution uses the same configured serve working directory (`craft_runtime.go:101-110, 148-153, 195`). Thus A remains readable despite Run 2's exclusion. This can also cross sessions because the configured work directory is shared. The typed snapshot controls new writes, but does not control what the runtime can read.

**Smallest defensible correction:** The pending shared T01/T05 per-Run sandbox isolation support can own this boundary: give each Run an isolated `inputs/` view populated exclusively from its verified snapshot and point its OpenCode execution at that view. If it cannot isolate the serve directory, atomically replace the accessible input tree while serializing every user of that directory. Test two sequential Runs in the same configured deployment, with the second excluding the first Run's file, and assert the excluded path is inaccessible during execution. Until that support is integrated and verified, T01 cannot pass independently.

### R2 — High: existing staged file bypasses digest verification

**Evidence / affected symbol:** `stageWorkspaceInputs` skips `GetFile` and SHA-256 verification whenever `os.Stat(target)` reports the declared size (`internal/container/craft_runtime.go:400-402`). The new snapshot and DB checks verify metadata, not the bytes already at `target`; the digest check runs only for a fresh write (`craft_runtime.go:403-420`).

**Impact:** A same-size altered file at a content-addressed path is supplied to a later Run as if it matched the immutable admitted input. This weakens the stated content-addressed identity and read-only materialization guarantee, especially because the directory persists across executions.

**Smallest defensible correction:** Verify the existing file's digest before reuse, and replace or reject it on mismatch using an atomic write. Add a test that changes staged bytes without changing length, then restages and verifies the authoritative bytes or a closed failure.

### R3 — Medium: interrupted admission can block decisions indefinitely

**Evidence / affected symbols:** `claimInputDecisions` durably changes a continue row and inserts an `input_admission` marker before `Submit` (`internal/application/service/craft_inputs.go:325-369`; `craft_session.go:779-797`). `DecideInput` rejects any decision while the marker exists (`craft_inputs.go:294-316`). The only marker release is in the current `StartRun` process after `Submit` returns (`craft_session.go:795-807`); the report itself acknowledges that a crash can leave the claim permanently in place. The new controlled test manually releases a claim and does not simulate process interruption.

**Impact:** A crash between claim and Submit can leave zero Run but permanently reject cancel and all new Run request IDs. That prevents the documented recovery choice even though the requested Run never started.

**Smallest defensible correction:** Add a durable reconciliation rule tied to authoritative Run/request state, with a bounded lease or recovery transaction. Only release an abandoned claim after proving its request has no admitted Run; test restart recovery for both pre-Submit and committed-Submit states.

## Prior findings and test evidence

- **F1 partly corrected:** The selected manifest is a typed `DurableRunSnapshot.CraftInputManifest`, omitted from ordinary snapshots and not appended as a serialized marker to `Query` (`agent_run_graph.go:54-61, 70-96, 144-153`). Runtime rejects a missing Craft manifest and verifies Run tenant/session/owner, workspace scope, selected ref metadata, and blob digest on fresh writes (`craft_runtime.go:431-527`). R1/R2 mean the selected material guarantee remains incomplete.
- **F2 corrected for new writes:** `AssociateInput` now sets recognition fields from bounded bytes before insertion (`craft_session.go:590-636`); legacy all-null rows remain readable. The service test checks that this path requires a continue decision. An HTTP old-route journey is still absent and belongs in integration verification.
- **F3 substantially corrected:** `AcceptInputRound` inserts all rows in one DB transaction, verifies conflict metadata, and checks for existing associations before deleting objects after a failed transaction (`craft_inputs.go:167-227`). The injected second-row failure/retry test covers DB atomicity and an existing referenced blob. Object-store deletion errors and canceled-context cleanup can leave unassociated objects, as the implementer report discloses; there is no evidence here of a dangling committed DB row after the fix.
- **F4 race closed for the live process:** claim and cancel contend on the decision row, and cancel is rejected during an active admission fence (`craft_inputs.go:294-369`). The controlled interleaving test covers that order. R3 is the outstanding recovery case.
- **F5 corrected conservatively:** valid bounded JSON is parsed; malformed JSON and text extensions without a parser remain accepted but not understood (`craft_inputs.go:230-252`). The focused recognition test covers valid and malformed JSON.

The implementer reports passing focused service/runtime tests and `go test ./internal/application/service/... -count=1` at the listed eight-file hashes. I verified those hashes and the committed diff's whitespace check, but did not rerun the suites. Production router and workbench composition remain controller-owned gates from the prior review.
