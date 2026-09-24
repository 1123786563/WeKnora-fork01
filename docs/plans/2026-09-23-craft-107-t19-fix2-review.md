# T19 / #138 fix round 2 independent review

Date: 2026-09-23. Read-only review of T19 worktree HEAD `d9008dcb6fcf59067778710e8e90b10ba8230571` (external fix-1 commit) plus the uncommitted fix-2 changes in `internal/application/service/craft_budget.go`, `craft_budget_t19_test.go`, and `internal/modules/commercial/repository/commercial/budget_reservation.go`, and untracked `docs/plans/2026-09-23-craft-107-t19-fix-2-report.md`. `git status --short` contained exactly those four paths; `git diff --check` passed. Current source SHA-256 values match the report: `f880a3ffbbef3f573d832b78cc1827cc689c2dbe80f6ad36b75d3edc6bce3216`, `f80b5ac32e6a288bf5aca9990343648d04d11ef222d0090645cbffa5219b0cd6`, and `2cd15e9f7221268a44e92b0c82a7c73797be73334e99580e374900e84d35b57c`; report SHA-256 `2a7364fdb5a7b5aee9edef4c9bdd11154bba07628d87a79024bedec94eb9057c`. Sources: approved Craft Spec, `CONTEXT.md`, ADR-0004, #138/T19 brief and plan, original independent review, fix-2 plan/report, and pause-fence/journal design. No OCR was run.

## Verdict

- **Spec compliance: FAIL for #138/F3; service-side progress only.** `StartBinding` and budget pause acquire the same tenant/Run row write fence. In the B-first in-process path, callback initiation precedes a pause commit; after pause, a new `StartBinding` is refused. The service still cannot guarantee a durable commercial hold after provider acceptance followed by process/SQL failure, and no production model or sandbox start calls `StartBinding`. The report correctly leaves F3 open. The remaining #138 API/UI, promotion, and charge-attribution gates are outside this fix-2 delta.
- **Code quality: FAIL for completion of the durable start invariant.** The typed outcomes and G4 transaction-scoped methods are useful seams, but the external callback is inside an uncommitted SQL transaction and has no durable pre-send journal. I independently ran `go test ./internal/application/service -run 'TestCraftChargeFence' -count=1` (exit 0); the implementer's focused G4/service and race runs are recorded in the fix-2 report, not repeated here. PostgreSQL locking and actual provider paths were not tested.

## Findings

### 1. High — provider start can survive rollback of its only accounting evidence

**Evidence / affected symbols:** `CraftBudgetService.StartBinding` creates a call, reserves G4 budget, and marks it dispatched inside one transaction (`internal/application/service/craft_budget.go:168-229`), then invokes `start` before that transaction commits (`230-239`). `BudgetStore.ReserveInTx` uses the caller transaction (`internal/modules/commercial/repository/commercial/budget_reservation.go:45-54`). A crash, cancellation, or commit error after provider acceptance rolls back the call and dispatched reservation while the external effect can continue. The typed `Started`/`Unknown` result is in memory only. The fix-2 report explicitly acknowledges this gap.

**Impact:** The effect can consume model/sandbox resources without a durable Task hold or stable retry marker. A later retry can start it again, violating once-attribution and safe recovery required by #138.

**Smallest defensible correction:** Add the proposed tenant/Run/activity-key journal and commit an `intent` plus G4 hold before any external send. Reconcile unresolved intent as unknown after crash, retain its hold, and block duplicate starts/pause completion until start or definite non-start is durably resolved. Verify injected post-send crash/commit failure and same-key recovery before using the seam.

### 2. High — chargeable production call sites still bypass the new fence

**Evidence / affected symbols:** `rg 'StartBinding\(' internal --glob '!**/*_test.go'` finds only its definition at `craft_budget.go:150`; no Lead model gateway or sandbox lifecycle caller invokes it. Existing `AuthorizeBinding`/`AuthorizeCall` paths remain in the service (`craft_budget.go:~500-580`) and are not tied to the new `start` callback. The fix-2 plan and report reserve central gateway/sandbox wiring for later work.

**Impact:** A paused Run can still initiate chargeable work through an old authorize-then-forward path even if the new two-worker test passes. The new fence is optional until every outbound start is routed through it.

**Smallest defensible correction:** Have central gateway and sandbox owners supply persisted activity keys and call the coordinator at the actual external initiation boundary; remove or narrow old authority paths so they cannot authorize an unfenced send. Test both model and sandbox starts racing a committed pause on the integrated path.

### 3. Medium — concurrency and callback-duration proof is narrower than the plan

**Evidence / affected symbols:** `TestCraftChargeFenceAFirstPausePreventsStart` pauses to completion and only then calls `StartBinding` (`craft_budget_t19_test.go:235-258`); it does not create a controlled two-worker A-first race around the final slot. The B-first test uses two service objects over one SQLite `*gorm.DB` and barriers inside the callback (`261-323`), which is useful but not PostgreSQL row-lock evidence. `StartBinding` passes `context.WithTimeout` to `start` (`craft_budget.go:230-232`), but the outer transaction uses the caller context and waits for callback return; a callback that ignores cancellation can hold the Run row lock beyond 30 seconds. No test covers this.

**Impact:** The current tests do not establish the intended A-first interleaving or a bounded pause wait under a slow transport. A stuck callback can stall budget pause and other Run state writers.

**Smallest defensible correction:** Add barriers that race A's pause commit against B's lock acquisition, and a cancellation-ignoring callback test or enforceable transport deadline. Run the same lock-order and failure cases against PostgreSQL before claiming production row-lock behavior.

## Supported behavior and remaining gates

The new no-op `agent_runs.revision` UPDATE (`craft_budget.go:271-285`) is a concrete shared write-lock seam on the Run row; `pauseCraftRunTx` and `ExtendAndResume` advance revision on state changes (`298-310`, `~1035-1060`). A previous activity key is rejected before a second callback (`185-190`). `ReserveInTx` and `MarkReservationDispatchedInTx` preserve G4 writes in the caller's transaction, subject to transaction-commit success. `DefinitelyNotStarted` currently retains a dispatched hold (`craft_budget.go:246-249`), conservatively avoiding accidental release but requiring reconciliation and a durable outcome before final policy is complete. The original T19 review's extension, Task-deadline, and central UI/API concerns should be tracked through their separate fix-1 and integration reviews; this document does not credit them as resolved by fix-2.
