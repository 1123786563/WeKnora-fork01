# R8 Task 2 Fulfilled-Exception Recovery Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Keep an already-fulfilled top-up event Pending while its invalid-winner exception is unresolved, and acknowledge it only after the corrected winner and expected benefit receipt are both verified.

**Architecture:** Preserve the current fulfilled-order fast path for events without an open fulfillment exception. For an open exception, validate the immutable winning payment attempt and transaction, then verify the deterministic top-up fulfillment key has a nonempty remote receipt. Only after those checks may the existing transaction atomically resolve the exception and mark the leased outbox event Sent.

**Tech Stack:** Go, GORM, SQLite-backed service tests, commercial fulfillment gateway.

**Spec:** R8 Task 2 brief in `.superpowers/sdd/issue-72-ocr-findings-r8/task-2-brief.md`; approved Lago payment/fulfillment Spec AC 6–7 and 22; ADR-0012; `CONTEXT.md`; architecture decision §2 in `/tmp/issue72-r7-conflict-attention-architecture-20260929.md`. Finding source: `.superpowers/sdd/issue-72-ocr-findings-r8/task-2-review.md`.

## Global Constraints

- Keep invalid or missing winners Pending and do not call `ApplyBenefit`.
- Never resolve an open exception or mark its event Sent without both exact winner validation and a nonempty receipt for `domain.FulfillmentKey(order.ID, "credits")`.
- Preserve the current fulfilled fast path for events with no open exception.
- Keep exception resolution and leased event Sent transition in the same database transaction through the existing `completeFulfilledEvent` boundary.
- Do not downgrade or rewrite an existing Applied `FulfillmentRecord`.
- Only modify `internal/modules/commercial/service/commercial/fulfillment.go` and `internal/modules/commercial/service/commercial/fulfillment_test.go`. Preserve the existing modified Task 2 report and all other worktree changes.
- Commit the implementation and regression test locally; do not push.

## Review Focus

- Already-fulfilled + open exception + invalid winner: event Pending, exception open, no ApplyBenefit call.
- Corrected exact winner without a discoverable nonempty receipt: event remains Pending and exception remains open.
- Corrected exact winner with the expected deterministic receipt: event becomes Sent and exception resolves atomically; no duplicate ApplyBenefit.
- Already-fulfilled order without an open exception retains existing fast-path behavior.
- Transient lookup errors remain retryable and do not acknowledge the event.

## Task 1 — Repair and prove fulfilled-order exception recovery

**Dependencies:** R8 Task 2 initial commit `11470a6a34d9be513aafd58c0db67aed4a5e4c15`; no shared DB/container dependency.  
**Owner:** backend_implementer. **Validator:** backend_validator.  
**Owned files:** the two Go files listed in Global Constraints.  
**Consumes:** `FulfillmentService.fulfillEvent`, `winningPaymentTransaction`, `completeFulfilledEvent`, `domain.FulfillmentKey`, the existing `stubGateway`.  
**Produces:** Corrected fulfilled-order exception recovery; regression `TestFulfillmentTopUpFulfilledOpenExceptionRequiresWinnerAndReceipt`.

- [ ] **Step 1 — RED:** Add the named regression. Seed a paid top-up and recover an invalid winner to create the open exception; force the order row to `fulfilled` to represent the reachable split state. Drain again and assert event Pending, exception open, and no ApplyBenefit. Correct the outbox payload to the registered winner and drain without a discoverable receipt; assert it remains Pending/open. Add the nonempty receipt to the gateway under the exact `FulfillmentKey`, mark it discoverable, drain again, and assert Sent/resolved with no duplicate ApplyBenefit. Run:
  `go test ./internal/modules/commercial/service/commercial -run '^TestFulfillmentTopUpFulfilledOpenExceptionRequiresWinnerAndReceipt$' -count=1`
  Expected before production edits: FAIL because the fulfilled fast path marks the event Sent while the exception stays open.

- [ ] **Step 2 — GREEN:** Change `FulfillmentService.fulfillEvent` so an already-fulfilled order first looks up an open exception for this event. If absent, preserve the current `completeEvent(..., Sent)` fast path. If present, validate `winningPaymentTransaction`; deterministic invalid/missing winner and absent/empty receipt must leave the event Pending and the exception open. Verify the top-up receipt by calling `FindBenefit` with `domain.FulfillmentKey(order.ID, "credits")`; transient lookup failures return/retry without acknowledgement. On valid winner plus nonempty receipt, call `completeFulfilledEvent` to resolve and mark Sent in one transaction. Do not call `ApplyBenefit` in this recovery branch.

- [ ] **Step 3 — Verification:** Re-run the focused regression and:
  `go test ./internal/modules/commercial/service/commercial -count=1`
  `git diff --check`
  Expected: both Go commands pass; the package reports `ok`, and diff-check has no output.

- [ ] **Step 4 — Report and commit:** Save the fix-round report at `.superpowers/sdd/issue-72-ocr-findings-r8/task-2-fix1-report.md`, including exact HEAD, before/after RED evidence, test output, changed-file hashes, and any remaining concern. Commit only the two owned files as `fix(commercial): verify fulfilled attention recovery`.

## Self-review

- **Coverage:** The medium review finding and Task 2's exact-winner/receipt atomic-resolution requirement are covered; the normal fulfilled path remains covered by existing tests.
- **Step clarity:** Every step has a specific test, branch condition, acceptance assertion, verification command, or commit scope.
- **Interface consistency:** Uses existing `fulfillEvent`, `winningPaymentTransaction`, `completeFulfilledEvent`, gateway interface, and deterministic top-up key.
- **Review focus:** Invalid winner, corrected winner without receipt, valid receipt, no duplicate application, no-exception fast path, and transient lookup handling are assigned to tests or scoped code review.
- **Scope:** Exactly one backend recovery path with no migration or API changes.
