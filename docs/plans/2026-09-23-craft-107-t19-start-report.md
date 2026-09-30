# T19 Task 1 — Journal-backed start coordinator

Status: DONE_WITH_CONCERNS

## Scope and evidence

Implemented only Task 1 of the T19 durable-start plan. Read the approved Craft Spec, T19 plan, fix2 review, migration review, and the central seam map at /Users/wuyongjun/.codex/worktrees/craft-107-t19/WeKnora-fork01/docs/plans/2026-09-23-craft-107-t19-central-seam-map.md. The seam map confirms the live model gateway is still on AuthorizeBinding, no production sandbox charge path calls StartBinding, and lease/epoch validation plus all Run writer coordination remain future gates.

The reviewed migration 000191/000112 is present and matches the required tenant/Run/activity primary key, tenant/reservation uniqueness, four-state constraint, Run FK, and unresolved scan index. CraftChargeStartJournalRow now maps that existing table.

## Changes

- internal/application/service/craft_budget.go: prepare transaction now commits the stable activity call record, G4 dispatched reservation, and journal intent; only then does it invoke the bounded callback. It records started, unknown, or definitely_unstarted with an intent-state compare-and-swap. A callback error after possible send maps to unknown; cancellation after possible send is unknown; cancellation before callback is definitely unstarted. A failed outcome write leaves the committed intent and dispatched hold. Duplicate journal activity is rejected before callback.
- internal/application/service/craft_budget_start_test.go: added tests for committed intent/hold visibility, lost response and replay refusal, outcome-write failure leaving intent/hold, and a cancellation-ignoring callback that does not retain the Run SQL lock.
- internal/application/service/craft_budget_t19_test.go: parent-approved ownership amendment updated the B-first race assertion. Baseline SHA-256 was f80b5ac32e6a288bf5aca9990343648d04d11ef222d0090645cbffa5219b0cd6; the exact change asserts that pause may commit while initiation is in flight, the intent and hold already exist, and the hold remains dispatched. The post-pause retry remains refused.
- internal/modules/commercial/repository/commercial/budget_reservation.go: unchanged. Its existing MarkReservationDispatchedInTx already provides the needed caller-transaction operation; no repository behavior change was necessary.

## Verification

- RED: go test ./internal/application/service -run TestCraftChargeStartCommitsIntentAndHoldBeforeCallbackAndReplayDoesNotSend -count=1 initially failed to compile because the journal model was absent.
- go test ./internal/application/service -run 'TestCraftCharge(Fence|Start)' -count=1 — PASS after final source changes (ok, 4.314s; an earlier pass also completed in 7.553s).
- go test ./internal/modules/commercial/repository/commercial -run 'TestBudget' -count=1 — PASS (ok, 0.639s).
- Initial broad run: go test ./internal/application/service ./internal/modules/commercial/repository/commercial -count=1 — commercial package PASS; application service package FAIL after 240.596s. A later single-package capture below gives the exact failure.
- PostgreSQL concurrency was not run: TRPC_TEST_POSTGRES_DSN is unset. No PostgreSQL coordinator claim is made.
- git diff --check -- internal/application/service/craft_budget.go internal/application/service/craft_budget_start_test.go internal/application/service/craft_budget_t19_test.go internal/modules/commercial/repository/commercial/budget_reservation.go — PASS.

## Exact checkpoint

Checkpoint: .superpowers/sdd/2026-09-23-craft-107-implementation/t19-start-checkpoint-01/

It contains the complete pre-task contents (from the matching T19 Worktree source hashes), complete post-task source files, the exact task patch, complete git status --short, HEAD, and SHA-256 manifests. Integration HEAD at checkpoint: a5e9195acd6500c085c85d60c852148e7bbbbf34. No commit was made.

## Remaining risks and handoff

- StartBinding has no lease-owner/epoch argument because the existing call contract has no such value; the central seam map explicitly identifies this as a stale-worker gate. Add the typed fence before routing production call sites through it.
- The current PauseRunForBudget records waiting_user while an already committed intent may still be in flight. This task proves the dispatched hold is retained and no new start passes the paused Run check, but it does not implement the journal-aware pause-requested/drain semantics owned by Task 2.
- Production model gateway and sandbox start wiring, unresolved-intent recovery/reconciliation, all lifecycle/Run writer fences, PostgreSQL coordinator races, independent Task review, and full T19 acceptance remain open. This report does not claim T19/F3 complete.

## Denial-fix review follow-up

Independent review identified that the max-call and mapped G4 denial branches returned nil from the prepare transaction while setting only a denial error; the function then reached the callback without an intent or hold. Added an explicit preparation status carrying the committed journal or denial. Callback invocation now requires the prepared status after the transaction returns successfully; a transaction error or any unprepared result exits without callback.

RED evidence: go test ./internal/application/service -run 'TestCraftChargeStart(DeniedPrepare|PrepareWriteFailure)' -count=1 failed both denial cases before the fix because the callback proceeded and the subsequent outcome CAS returned craft conflict: charge start intent outcome changed concurrently. Added assertions for max-call exhaustion, G4 reserve denial, and journal preparation write failure; callback counts must remain zero.

GREEN verification:

- go test ./internal/application/service -run 'TestCraftChargeStart(DeniedPrepare|PrepareWriteFailure|CommitsIntentAndHoldBeforeCallbackAndReplayDoesNotSend|ResultWriteFailureLeavesIntentAndHold)' -count=1 — PASS.
- go test ./internal/application/service -run 'TestCraftCharge(Fence|Start)' -count=1 — PASS (4.153s).
- go test ./internal/application/service -count=1 — FAIL (99.624s). Full output is saved at .superpowers/sdd/2026-09-23-craft-107-implementation/t19-start-checkpoint-01/full-service.log. The only failure is TestCraftSourceGuardDocumentContentCannotWidenPermissions, at internal/application/service/craft_source_guard_test.go:78; expected list is [Sources Truncated], actual list is [Sources Truncated Empty]. This is a concurrent T05-owned source guard test and is outside this Task's write scope.
- A later standard go test focused invocation could not compile because a concurrent T08-owned craft_session_acl_test.go currently references missing svc.access/taskList fields, an incomplete FileService fake, and an unused owner variable. No T08/T05 file was changed. To verify despite that unrelated compile break, ran the top-level service files excluding only craft_session_acl_test.go by explicit file list: go test $(rg --files . | rg '^\./[^/]+\.go$' | rg -v 'craft_session_acl_test.go$' | sed 's|^\./||') -run 'TestCraftCharge(Fence|Start)' -count=1 from internal/application/service — PASS (9.591s). The same explicit file-list invocation for TestCraftChargeStartCommitFailureNeverInvokesCallback passed (1.639s). These scoped results do not represent a complete service package run.

Denial-fix checkpoint: .superpowers/sdd/2026-09-23-craft-107-implementation/t19-start-denial-fix-checkpoint-01/. No commit was made. Independent re-review is still required before Task 2 proceeds.
