# Issue #84 Cross-Attempt Review Fix R2

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Implement one task from its Task Brief; do not dispatch additional agents.

**Goal:** Prevent any paid purchase fulfillment, including credit top-ups, from granting an entitlement when its immutable winning outbox payment identity contradicts the registered winning PaymentAttempt. Keep existing top-up tests aligned with the explicit legacy no-quote representation.

**Architecture:** `ConfirmPayment` writes the order-CAS winner into `fulfill:<orderID>`. That event is the payment fact for all purchase fulfillment routes. Before quote classification or any external benefit call, compare the event's attempt/provider/merchant/transaction and amount/currency against the tenant/order-scoped succeeded PaymentAttempt and order. Subscription fulfillment may continue using the same validated immutable transaction for settlement. Do not select another succeeded attempt or weaken the missing nonempty QuoteID quarantine. Legacy top-ups are represented by an empty QuoteID, not by a dangling nonempty quote reference.

**Tech Stack:** Go, Gorm, SQLite service tests; tagged PostgreSQL concurrency test remains separately environment-gated.

**Spec:** Issue #84; `docs/specs/2026-09-20-lago-billing-migration-design.md` User Story 8 and payment reconciliation acceptance; `docs/adr/0012-lago-as-commercial-billing-authority.md`; existing plan `docs/plans/issue-72-plan-84-cross-attempt-settlement-fix.md`; R1 independent review `/tmp/issue72-84-cross-attempt-task-review-r2.md`.

## Global Constraints

- Outbox winner identity and the `OrderStatePending -> OrderStatePaid` CAS remain authoritative.
- Contradictory/missing deterministic winner facts fail closed to attention/quarantine before any benefit or settlement command; transient DB errors remain retryable.
- Never mutate the recorded winning PaymentAttempt transaction when a later distinct payment succeeds.
- Do not alter #84 external payment APIs, schemas, provider callbacks, or behavior outside paid purchase fulfillment.
- Use only the isolated worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees-issue72/issue-84`; preserve existing four-file checkpoint and all unrelated work.
- Local commits are authorized only after independent task review passes. No push or remote Issue mutation.

## Task 1 — Gate top-up fulfillment on the immutable winning payment identity

**Dependencies:** R1 checkpoint at BASE `ae8f57c8cb33b15f7ec78cab3758ae00d3bd3cbe`; independent R1 task review `/tmp/issue72-84-cross-attempt-task-review-r2.md`. **Owner:** backend_implementer. **Validator:** backend_validator. **Independent reviewer:** reviewer. **Checkpoint:** uncommitted until review passes.

**R2 write-owned files:**
- `internal/modules/commercial/service/commercial/fulfillment.go`
- `internal/modules/commercial/service/commercial/purchase_fulfillment.go` for the shared winner-validation seam
- `internal/modules/commercial/service/commercial/purchase_fulfillment_test.go` for a real payment-attempt top-up fixture
- `internal/modules/commercial/service/commercial/fulfillment_test.go` for mismatch, missing-winner, and transient-query tests plus existing fixture alignment

**Cumulative review scope:** all R1 + R2 changes from original BASE `ae8f57c8cb33b15f7ec78cab3758ae00d3bd3cbe`, including `internal/modules/commercial/repository/commercial/order_pg_test.go` from R1. R2 does not own or edit that repository file.

**Interfaces:** Preserve `FulfillmentService.Recover(ctx context.Context) error`, `PurchaseFulfiller.Fulfill(ctx context.Context, ev repocommercial.OutboxEvent) error`, and outbox event format. Reuse/refactor the current winning-attempt validation without changing its rule: load by exact `attempt_id + order_id + tenant_id`; require succeeded state, exact provider, merchant, transaction, amount and currency. Any new helper remains internal to package `commercial` and returns the immutable `payload.Transaction` only after all checks pass.

- [ ] **Step 1 — RED: top-up contradiction coverage.** Add tests with a paid top-up order and (a) a valid winner A but event transaction B, (b) deterministic missing winning attempt, and (c) a one-shot transient attempt-query failure. Mismatch/missing identity must durably mark the `fulfill` event `dead`, leave the paid order and fulfillment records unchanged, issue zero FindBenefit/ApplyBenefit calls, and allow a following valid event in the same Recover pass to complete. A transient lookup error must make zero gateway calls, leave event pending/no fulfillment record, and allow that exact event to complete on the next Recover pass after the injected error clears. Run `go test ./internal/modules/commercial/service/commercial -run '^(TestFulfillmentTopUpRejectsContradictoryWinningAttempt|TestFulfillmentTopUpMissingWinningAttemptIsQuarantined|TestFulfillmentTopUpTransientWinningAttemptReadRetries)$' -count=1 -v`; these RED tests fail against current top-up routing.
- [ ] **Step 2 — GREEN: validate before route dispatch.** Validate the paid purchase's immutable event identity before `subscriptionPurchase` and before the top-up/upgrade benefit loop. Reuse the winner lookup/validation seam used by `PurchaseFulfiller`; avoid duplicate divergent predicates. For the top-up route, deterministic missing or contradictory identity uses the existing durable dead-event quarantine path, leaves the paid order unchanged, and returns without platform commands; the same Recover pass continues to the next valid event. Transient database errors keep this exact event pending/retryable. Preserve quote lookup policy: only empty QuoteID uses explicit legacy top-up semantics; nonempty missing/wrong-tenant quote remains quarantined.
- [ ] **Step 3 — RED/GREEN: align legacy test fixtures.** Update the `seedPaidOrder` fixture or affected tests so each “valid top-up” has a real registered succeeded PaymentAttempt and matching winner fields in its immutable fulfill payload; preserve `QuoteID == ""` only as explicit legacy top-up quote semantics. Do not add an empty-QuoteID payment identity bypass or weaken assertions. Align the five full-suite fixtures that used dangling nonempty QuoteIDs: `TestFulfillmentWorkerSavedThenDroppedRecoversExactlyOnce`, `TestFulfillmentWorkerConcurrentClaimsApplyExactlyOnce`, `TestFulfillmentWorkerPaidNotFulfilledStaysRecoverable`, `TestOverPaymentDrainConsumesEventIntoAwaitingDisposal`, `TestOverPaymentDisposeFailureDoesNotBlockFulfillDrain`. Also migrate the two `seedLegacyPaidTopUp` cases in `purchase_fulfillment_test.go` (`TestFulfillmentRecoverMissingTenantQuoteDoesNotRouteAsTopUp`, `TestFulfillmentRecoverQuarantinesMalformedPurchaseEventsAndContinues`) to real winning payment identity while retaining empty QuoteID. Run these seven named tests and the three new regressions; all pass.
- [ ] **Step 4 — verification.** Run `go test ./internal/modules/commercial/service/commercial -count=1`; all service tests pass. In addition, run the exact combined R1/R2 focused expression `go test ./internal/modules/commercial/service/commercial -run '^(TestPurchaseFulfillmentUsesOrderWinningAttemptAcrossAttempts|TestPurchaseFulfillmentFailsClosedOnContradictoryWinnerAttempt|TestPurchaseFulfillmentTransientAttemptReadKeepsEventPending|TestFulfillmentRecoverQuarantinesMalformedPurchaseEventsAndContinues|TestFulfillmentTopUpRejectsContradictoryWinningAttempt|TestFulfillmentTopUpMissingWinningAttemptIsQuarantined|TestFulfillmentTopUpTransientWinningAttemptReadRetries)$' -count=1 -v`. Run `go test ./internal/modules/commercial/repository/commercial ./internal/modules/commercial/service/commercial -count=1`; both packages pass. Run the focused four R1 tests and the new top-up mismatch test. Run `git diff --check`. Record exact commands, output, file hashes, and `SAAS_TEST_PG_DSN` limitation (tagged PG test is not required for this narrow consumer change but remains unverified if absent).
- [ ] **Step 5 — independent Review and ledger.** Generate a task checkpoint Review Package from the original recorded BASE and include all R1 + R2 changes, including the R1 tagged PostgreSQL test even though R2 does not edit it. Obtain separate Spec Compliance and Code Quality verdicts; fix valid findings and re-review. Update `docs/plans/issue-72-ledger-84.md` only after review passes. Leave uncommitted for controller acceptance; no push.

## Acceptance Mapping

| Requirement | Step | Evidence |
|---|---:|---|
| A contradictory winner never causes a top-up benefit | 1–2 | Mismatch regression proves zero gateway calls and no applied fulfillment record |
| Correct winning payment facts continue to fulfill | 2–3 | Real-attempt legacy top-up fixture and subscription tests pass |
| Missing winner and transient top-up attempt reads fail closed/retry safely | 1–2 | Durable-dead missing winner and pending-then-replay transient regressions |
| Legacy top-up semantics are explicit | 3 | Fixtures use empty QuoteID; production dangling-reference fail-closed test remains |
| Transient winner-store failures are retryable | 2 | Existing transient DB test plus package suite |
| No unrelated behavior changes | 4–5 | Four owned source/test files only, diff-check, independent Review |

## Plan Self-Review

- All R1 task findings are inherited as already implemented and must remain covered; R2 adds validation for the top-up route and repairs the fixture mismatch exposed by the fail-closed quote rule.
- The event winner remains the sole source of the settlement transaction; validation happens before every paid purchase dispatch branch.
- Work is one backend slice with no concurrent writer or shared runtime resources. PostgreSQL race verification is orthogonal to consumer routing; SQLite package behavior is fully required.
- No production changes outside the four owned files, no schema/API changes, no live Lago stack, and no external side effects are planned.
