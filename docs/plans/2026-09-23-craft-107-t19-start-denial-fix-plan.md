# T19 Start Coordinator Denial Fix Plan

> **For Codex:** SDD RED → GREEN → REFACTOR, exact checkpoint and independent Review. No commit.

**Goal:** Close the critical Task 1 review finding: denied/exhausted G4 prepare paths may return nil from the transaction with `startErr` set, after which `StartBinding` still calls the external callback with no committed journal/hold.

**Sources:** Approved Spec #107/#138, `t19-durable-start-plan.md` Task 1, `t19-start-report.md` and its checkpoint, pending `t19-start-review.md`. Integration Worktree contains concurrent T08 B1/T05 Publisher changes in disjoint files.

**Global Constraints:** backend_implementer owns only `internal/application/service/craft_budget.go`, `craft_budget_start_test.go`, any required `craft_budget_t19_test.go` focused expectation and report. No edits to T08/T05/RunView/other files, commits or subagents. Other agents share workspace; preserve all changes. Never make a transport callback reachable unless a committed matching intent and G4 hold are verified in the prepare result.

**Review Focus:** max-call exhausted, G4 reserve denial and every non-error `prepare` early return must not call callback. Pause/denial status persists as intended. No call ID/intent/hold should be fabricated after denial. A valid prepared start still calls once only after commit. A DB commit error cannot call callback. Distinguish error before send from unknown after send.

## Task 1 — prevent callback after denied prepare

1. RED: add table-driven tests with callback counter for max-call exhausted and reserve-denied prepare. Assert callback zero, no committed intent/hold for refused activity, durable waiting/pause state where policy requires, and correct denial error. Add a transaction-commit failure injection if feasible. Observe failure on current checkpoint.
2. GREEN: return an explicit typed prepared/result state from the transaction and check it before callback. Do not rely on `err == nil` alone; `startErr` is insufficiently handled today. Require committed intent+hold identity before send. Keep after-send CAS/unknown logic unchanged except necessary consistency fixes.
3. Run focused denial and start-fence tests, service package if shared sources stable (capture full log to file), diff check, complete source hashes/checkpoint/report. Report unresolved Task 2/production caller/PG gates. Independent reviewer must recheck this fix before Task 2 proceeds.
