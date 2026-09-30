# R5 Effect Task2b Task1 independent transition-fence review

## Scope and checkpoint

Reviewed the approved Craft #107 R5 effect requirements, ADR-0008, the Task2b plan/report/checkpoint, Task2a Fix1 effect authority and R4 terminal-capture trigger. The task patch SHA-256 is `2dde61aa9f05138b7dbc73cce93d7bf1c1369f5dced8731fff41a605c61c39c9`; pre/post archives have the checkpoint hashes `436bfe006b885ef4487b487d32b0d430d177f54c45cbb1eb1377c8798e70ecfc` / `cefc83719599c2531ec25617458a165ccaae42368907c52a8bc7b5b9c34e2945`. I verified each captured preimage against its manifest, applied the patch in a temporary tree, and verified all five outputs byte-for-byte against both the post manifest and current integration files. The fifth file is the new test; other concurrent worktree changes were excluded.

The report records passing focused SQLite and isolated PostgreSQL 17 tests, race selector, compile-only test, R4 capture/charge compatibility selector, and diff check at those postimage hashes. I did not repeat the controller's database or race runs. PostgreSQL was an isolated schema with `app.skip_embedding=true`; this is focused evidence, not a full migration-chain or production integration test.

## Mutation-path review

- `agent_run.go:355-451,492-493,945-976`: `Admit` holds the session row before testing unresolved effects across all Runs in the session and before a new slot reservation. `BeginEffect` takes the effect Run lock and then the same session lock before inserting intent, so a claim cannot commit behind a session-slot transfer that read no intent. Exact existing-admission replay remains read-only.
- `agent_run_lifecycle.go:14-133`: direct/owned cancellation and each `DeleteSessionRuns` cancellation use `cancelRunTx`, which locks the Run and checks pending/unknown before revoking a lease, writing a cancellation event, or clearing the slot. The final deletion cleanup can clear the slot only after each Run passed that guard. A tombstone can be persisted before the later guarded cancellation fails; the Run/slot stay fenced.
- `agent_run.go:1004-1141,1171-1204,1361-1397`: waiting-user pause, charge-start reconciling finalization, `ClaimDriver`, and terminal `SetStatus` check under the Run lock before status/lease/epoch/slot changes. The effect error remains typed and is not treated as a charge settlement. `PauseForCraftBudgetInTx` depends on its documented caller-held Run lock; the production budget caller uses `lockCraftRun` in the same transaction.
- `agent_run_events.go:112-166` and `agent_run_decisions.go:88-147`: `Finalize` and new decision writes check under their Run lock before assistant/tool state, status, event, or slot mutation. Exact stored decision replay remains read-only. R4's `AFTER UPDATE OF status` terminal-capture trigger is still reached once after a finished effect; pending/unknown prevents the terminal update.
- `Renew`, checkpoint/event append and read/retention paths do not transfer the writer, change status/epoch, or clear the slot. I also searched repository session-slot writes: the other relevant source path locks without changing the slot; cleanup purge remains subject to existing retained-row/FK and durable observation gates rather than a new transition in this patch.

## Finding

**R5-T2B-1 — Low — deterministic barriers cover only cancellation.** `agent_run_effect_fence_test.go:21-121` constructs claim-first and transition-first barriers for `CancelRun`; `:122-270` exercises the other named transitions with committed pending/unknown intents or transition-first outcomes sequentially. This is useful behavioral coverage, but it does not fulfill the plan's explicit deterministic claim-first/transition-first barrier test for every named path, particularly admission/session deletion and lease recovery. **Impact:** a future lock-order regression in one of those entry points may escape the focused race proof even though this code currently places its guard under the Run/session lock. **Smallest correction:** extend the table-driven barrier harness to the remaining mutation entry points, at least admission, deletion, `ClaimDriver`, waiting pause and terminal finalize; retain their existing state assertions.

## Verdicts and remaining gate

- **Scoped Spec compliance: PASS.** The reviewed increment guards the enumerated Run/slot transitions while pending or unknown effects retain authority; finished effects permit the intended later transition. I found no path in this increment that clears the active slot or transfers an epoch past an unresolved intent. The R4 capture trigger and charge-start reconciling semantics remain compatible at the examined transaction boundaries.
- **Code quality: PASS with the low test-coverage finding above.** The patch is exact, the common typed guard is narrow, and the focused SQLite/PostgreSQL/race evidence is bound to the checkpoint. The remaining deterministic barrier coverage should be added before relying on this as a long-term regression gate.

This review does not establish physical Docker/OpenCode effect routing, external-call TOCTOU safety, or a complete T01 release. Those remain downstream gates.
