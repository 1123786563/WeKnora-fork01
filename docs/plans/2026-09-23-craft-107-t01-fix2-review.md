# T01 / #120 fix round 2 independent review

Date: 2026-09-23. Read-only review of T01 worktree HEAD `0c17125fe002f02838841f5133e4f6b9d476fec8` plus uncommitted `internal/container/craft_runtime.go`, `craft_runtime_test.go`, `internal/application/service/craft_t01_test.go`, and untracked fix-1 report, fix-2 plan and fix-2 report. `git status --short` contained exactly those six paths; `git diff --check` passed. The current three source SHA-256 values match the implementation report: `a24213ce1a6a288dd58ab04f647dfdd66f52e490c4b17edbaab7d083856a8c46`, `90d5ba55d1b0fb49cf1b444d29e2e95c693c33ef3ea8a8cfe252e441406def7f`, and `6e3670a15fb43e80d77a1d91d1cad5e87b9deb158d10279d12c297bdaf39af6c`. The untracked fix-1 report, fix-2 plan and fix-2 report hashes are `bbd09dd95cfdef4ddd69dfbf543ad3da0997451ec19a864ba885282fa0a0f56c`, `4dad5e831559f1e08605e3543a41f614f8ffb0ec6b9c9c454e37b25bde635c9b`, and `aac0adf7f08d5d17ef6fb61a34be8b08df2bb0fc64e4333e011fdf670a6f46ab`. Sources: approved Craft Spec, `CONTEXT.md`, ADR-0004/0009, #120/T01 brief and plan, fix-1 independent review, fix-2 plan and report. No OCR was run.

## Verdict

- **Scoped fix-2 Spec compliance: PARTIAL PASS.** R2's same-size staged-byte bypass is corrected for a quiescent existing file: the runtime now hashes existing bytes and atomically replaces mismatches from a verified object-store copy. R3 abandoned admission remains a real failing behavior and is deliberately represented by a RED restart test. R1 selected-only per-Run filesystem isolation remains outside this delta and open.
- **Full #120 Spec compliance: FAIL.** The persistent shared `inputs/` tree still exposes previously selected but now omitted material, and an interrupted pre-Submit claim can still block cancellation or a new Run. These violate selected-input and durable recovery acceptance.
- **Code quality: CONDITIONAL PASS for R2, FAIL for the overall T01 checkpoint.** I independently ran `go test ./internal/container -run '^TestStageWorkspaceInputsUsesOnlySelectedTaskSnapshot$' -count=1` (exit 0, with the existing duplicate `-lc++` linker warning). The implementer's full service suite exited 1 only on the intentionally preserved R3 regression; I did not rerun that long suite. The safe-recovery proposal requires schema and `RunStore.Admit` work before it can be credited as a fix.

## Findings

### 1. High — R1 selected-only read boundary remains absent

**Evidence / affected symbols:** `localCraftRuntime.Execute` stages into the configured persistent `workDir` before delegating (`internal/container/craft_runtime.go:179-195,374-426`). An empty next-Run selection returns without removing earlier files (`:383-385`), and the same local OpenCode serve/work directory is reused (`:95-153`). The new digest/rename code verifies selected targets but does not hide old sibling paths. The existing fix-1 review gives the reachable Run A → Run B path.

**Impact:** Run B can read Run A's raw uploaded input even when the user omitted it from B, including after choosing to continue without an unrecognized file. A digest-correct old file is still unauthorized material for B.

**Smallest defensible correction:** Give each admitted Run a fresh execution root with only its selected verified input package, excluding prior Run inputs and session state. Prove from inside Run B that A's old path cannot be opened. Coordinate this with T05's per-Run sandbox boundary; do not mark #120 verified from staging tests alone.

### 2. Medium — R3 interrupted admission claim remains unrecoverable

**Evidence / affected symbols:** `TestCraftT01RestartRecoversAdmissionClaimWithNoDurableRun` in `internal/application/service/craft_t01_test.go:76-106` creates a durable claim with no `agent_runs` row, reassembles the service, and expects cancel to succeed. The fix-2 report records this test as the sole failure in the full service suite: `DecideInput` still returns the admission-fence conflict. The proposed `admission_run_id`, token, state and lease columns plus an atomic `RunStore.Admit` claim check were not implemented in this checkpoint.

**Impact:** A process crash before Submit can indefinitely deny cancel and new request IDs despite no Run being admitted. Releasing the marker based only on an absence read would race a delayed Submit; the report correctly avoids that unsafe shortcut.

**Smallest defensible correction:** Add the proposed dialect migrations and a transactionally shared claim/Run admission seam. `Admit` must verify the current claim token and transition it alongside Run insertion/replay; recovery must rotate the token by CAS after lease expiry and exact scoped Run check, fencing a delayed submitter. Re-run the preserved RED test plus concurrent reclaim/Submit tests. The committed-Run replay test passing does not resolve the abandoned pre-Submit case.

### 3. Medium — selected staged files are still writable by the local delegate

**Evidence / affected symbols:** `replaceCraftStagedFile` creates the verified file with mode `0644` (`internal/container/craft_runtime.go:463-483`) in the same `workDir` consumed by the local OpenCode runtime. `craftStagedFileMatches` hashes a file once before delegation (`:433-460`); there is no immutable mount or subsequent verification before the delegate reads it. The `0644` owner-write bit remains available when runtime and delegate share the local user.

**Impact:** The exact bytes checked by R2 can be changed after verification by a delegated process with filesystem write access. Atomic rename prevents a partial write, but does not make an uploaded input read-only during execution as the approved Spec requires.

**Smallest defensible correction:** Materialize inputs in a private per-Run read-only volume or copy with permissions/ownership enforced against the delegate, and verify the digest at the handoff boundary. Test a delegate write attempt and assert denial. This should be implemented with R1 isolation rather than a standalone chmod assertion in the current shared root.

## R2 evidence and R3 design status

`craftStagedFileMatches` uses `Lstat`, rejects non-regular/size-mismatched entries, reads at the input bound, and compares SHA-256 (`craft_runtime.go:433-460`). A mismatch loads the authoritative blob, validates exact length and digest, writes to a same-directory temporary file, and renames it over the target (`:405-423,463-483`). The focused test changes staged bytes to a different value of equal length and verifies restoration (`craft_runtime_test.go:110-129`). The committed-Run restart test checks same request replay, while the abandoned-claim test remains RED. The proposed atomic admission contract is technically directed at the right race, but has no migration, claim token, or `RunStore.Admit` transaction in this snapshot; it is a handoff requirement, not evidence of R3 closure.
