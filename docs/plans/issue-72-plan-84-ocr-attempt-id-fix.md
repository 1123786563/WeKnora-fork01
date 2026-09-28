# Issue #84 Payment Attempt Transaction Identity Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this repair task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Preserve the first successful provider transaction identity on a registered PaymentAttempt while retaining every later successful distinct transaction as an over-payment fact.

**Architecture:** Keep `PaymentAttempt.ProviderTransactionID` as the immutable identity of the first successful collection for that attempt. A later successful distinct transaction must not rewrite it; the existing guarded order transition and over-payment outbox/anomaly path continue to record and disposition the additional collection. Exact replay behavior remains idempotent.

**Tech Stack:** Go, Gorm, SQLite repository tests, commercial payment outbox.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` User Story 8; approved requirement: “Duplicate, mismatched, partial, wrong-currency, and multiple-success payment cases retain their external facts without automatically changing Invoice amount or delivered benefits.” Existing Issue #84 plan and evidence: `docs/plans/issue-72-plan-84.md`, `docs/plans/issue-72-ledger-84.md`, `docs/plans/issue-72-flow-evidence-84/README.md`. Independent findings: `/tmp/issue72-84-closeout-audit-20260928.md`, `/tmp/issue72-84-attempt-race-rationale.md`.

## Global Constraints

- Preserve the first successful provider transaction ID on one registered attempt; distinct subsequent provider transactions remain separately auditable.
- A second collection never creates a second fulfillment or changes the paid order amount/benefits.
- Do not change payment provider, external-payment, Lago settlement, Quote, Invoice, or subscription behavior outside this attempt identity invariant.
- Keep repository query parameters bound and credentials out of source/evidence.
- Do not modify shared Lago runtime or other Issue worktrees.
- Commit only after this task's independent review; do not push or change remote Issue state.

## Review Focus

1. Exact replay of the first transaction remains idempotent and emits no over-payment event.
2. A different successful transaction on the same attempt keeps the original attempt identity and emits exactly one over-payment audit for the new transaction.
3. The sequence A→B→A→B on one attempt preserves A, emits one over-payment fact for B, and emits no false over-payment for replayed winner A.
4. Concurrent distinct successes on one attempt record exactly one first-winner transaction ID; the loser remains auditable as over-payment and does not create another fulfillment.
5. A second channel attempt still records its own winning provider transaction while only one fulfillment is emitted.
6. Transaction uniqueness conflicts still roll back atomically without replacing the first transaction ID.

### Task 1: Make successful attempt identity immutable

**Dependencies:** None beyond existing #84 payment-attempt/outbox implementation at current worktree HEAD. This is a narrow follow-up to #84 AC2 (multiple successful payments retained without duplicate benefits).

**Role:** backend_implementer. **Validator:** backend_validator. **Independent Review:** reviewer.

**Files:**
- Modify: `internal/modules/commercial/repository/commercial/order.go`
- Test: `internal/modules/commercial/repository/commercial/order_test.go`
- Test: `internal/modules/commercial/repository/commercial/order_pg_test.go` (new, `commercial_integration` build tag)
- Test: `internal/modules/commercial/service/commercial/purchase_fulfillment_test.go`
- Update: `docs/plans/issue-72-ledger-84.md` with the checkpoint and evidence only after review.

**Interfaces:**
- Consumes: `OrderStore.ConfirmPayment(ctx context.Context, fact domain.PaymentFact) error` and current `PaymentAttemptRow` identity.
- Produces: unchanged method signature; the first successful `ProviderTransactionID` remains stable for the registered attempt; later distinct successful transaction is preserved in the existing over-payment event payload.

- [x] Add `TestPaymentSecondSuccessfulTransactionOnSameAttemptPreservesOriginal` beside `TestPaymentConfirmReplayAfterResponseLossKeepsSingleEvent`. Register one attempt, confirm transaction `txn_first`, then confirm `txn_second` on the same attempt, then replay `txn_first` and `txn_second`. Assert stored attempt remains `txn_first`; over-payment rows/payloads contain only `txn_second` exactly once; order version and fulfillment remain single. This reproduces the false anomaly for replaying the winner.
- [x] Run `go test ./internal/modules/commercial/repository/commercial -run '^TestPaymentSecondSuccessfulTransactionOnSameAttemptPreservesOriginal$' -count=1 -v`; confirm RED because the current second success overwrites `ProviderTransactionID`.
- [x] Change the attempt success write to a conditional `WHERE state = pending AND provider_transaction_id IS NULL` update. Check `RowsAffected`; when another success has already claimed the attempt, reload its immutable winning transaction and continue through the existing guarded order transition using the incoming fact only for duplicate-versus-overpayment classification. Preserve transaction rollback semantics if a provider-transaction uniqueness conflict occurs.
- [x] Run the focused sequence regression plus `TestPaymentConfirmReplayAfterResponseLossKeepsSingleEvent` and `TestPaymentConcurrentChannelsYieldSingleFulfillmentPlusAudit`; all must PASS. In new `order_pg_test.go` with `//go:build commercial_integration`, create an isolated per-test schema from `SAAS_TEST_PG_DSN`, AutoMigrate `OrderRow`, `PaymentAttemptRow`, and `OutboxEvent`, and create the same unique indexes as `testOrderStore`. In `TestPaymentConcurrentDistinctTransactionsOnSameAttemptKeepsOneWinner`, register a Gorm `Before("gorm:update")` callback filtered to `commercial_payment_attempts`; count arrivals atomically and block both transactions at the conditional attempt-claim UPDATE until both have already read pending, then release them together. Bound the barrier wait with a test timeout so a missing second arrival fails instead of hanging. Register a same-table `After("gorm:update")` callback to collect each claim UPDATE's `tx.RowsAffected`; require exactly one value of 1 and one of 0, with neither update returning an error. Assert one immutable first winner, one fulfillment event, one over-payment event carrying only the losing transaction, and no transaction/outbox duplication. The callbacks are scoped to this test's isolated DB. This exact callback barrier ensures both transactions reach the pre-claim seam; SQLite serialized repository tests are not evidence for this race.
- [x] In `purchase_fulfillment_test.go`, add a `recordingCommercialPlatform` wrapper that records each `SubmitCommand` and delegates to the fake, except that its first `SettlePurchasePayment` returns the retryable `ErrPlatformUnreachable` after recording and before delegation. For `TestPurchaseFulfillmentUsesFirstSuccessfulTransactionAfterLaterOverpayment`, seed a paid purchase with transaction A, confirm B on the same attempt before any fulfillment pass, and use the existing `primeFakePurchaseWithFees` helper to seed the fake's incomplete purchase and invoice fee. Pass the same paid outbox event to `PurchaseFulfiller.Fulfill` twice: the first pass leaves the order paid/event pending, and the second delegates settlement and completes activation/grant. Assert both recorded settle commands carry A in `ChannelTransaction` and `SettlePurchasePaymentCommandKey`, none carries B, and the fake has exactly one grant. Do not alter the production fulfillment protocol.
- [x] Add `TestPaymentTransactionUniqueConflictDoesNotMutateAttemptOrOrder`: make attempt A claim transaction X, then submit transaction X for a separate registered attempt/order B; assert the unique-index error propagates and B remains pending with nil transaction, order B remains pending/version 1, and no fulfillment/over-payment event for B persists.
- [x] Run `go test ./internal/modules/commercial/repository/commercial -count=1`, `go test -tags commercial_integration ./internal/modules/commercial/repository/commercial -run '^TestPaymentConcurrentDistinctTransactionsOnSameAttemptKeepsOneWinner$' -count=1 -v` using an isolated PostgreSQL DSN, and `go test ./internal/modules/commercial/service/commercial -run '^TestPurchaseFulfillmentUsesFirstSuccessfulTransactionAfterLaterOverpayment$' -count=1 -v`; all must PASS with no skip. Run `git diff --check`; record exact outputs and source hashes in the task report.
- [x] Prepare a Review Package against the recorded Task BASE; obtain separate Spec Compliance and Code Quality review. Fix only valid findings and repeat affected tests/review before integration.

## Issue Acceptance Mapping

| Acceptance | Task | Evidence |
|---|---|---|
| #84 AC2: multiple successful payments never duplicate benefits; later money is retained for review | Task 1 | First transaction remains on PaymentAttempt; one fulfillment; second transaction appears in one over-payment audit; repository and PostgreSQL race regressions plus the delayed-settlement service test pass |
| Spec User Story 8: duplicate money receipt does not duplicate a subscription | Task 1 | Guarded order version/outbox assertions, repository suite, and two-drive settlement identity/grant assertion |

## Global Consistency / Review Focus

This repair does not alter the payment fact interface, webhook mapping, over-payment disposition API, or fulfillment protocol. The same attempt ID remains the logical attempt identity; provider transaction IDs remain distinct external facts. Existing #84 issue-level closeout/OCR is still required separately after the checkpoint is reviewed and integrated.
