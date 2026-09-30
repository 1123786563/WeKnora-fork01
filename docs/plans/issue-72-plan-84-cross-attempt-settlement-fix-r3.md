# Issue #84 Cross-Attempt Review Fix R3

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Implement one task from its Task Brief; do not dispatch additional agents.

**Goal:** Validate the immutable order-CAS winning payment identity before quote lookup or subscription wiring can defer disposition, and prove transient attempt-read retries reuse the same persisted winner.

**Architecture:** Keep `winningPaymentTransaction` as the single exact-match predicate for attempt, order, tenant, provider, merchant, transaction, amount, and currency. Run it for every paid purchase before route classification; deterministic identity failures are quarantined through the existing durable fulfillment-event path, while transient store errors keep the event pending. Subscription settlement continues to use `PurchaseFulfiller` after the outer event has passed this guard.

**Tech Stack:** Go, Gorm, SQLite service tests.

**Spec:** Issue #84; `docs/specs/2026-09-20-lago-billing-migration-design.md` User Story 8 and payment reconciliation acceptance; ADR-0012; prior #84 plans `docs/plans/issue-72-plan-84-cross-attempt-settlement-fix.md` and `docs/plans/issue-72-plan-84-ocr-cross-attempt-fix-r2.md`; failed independent review `/tmp/issue72-84-cross-attempt-task-review-r3.md`.

## Global Constraints

- The order-CAS winner persisted in the `fulfill:<orderID>` outbox event is authoritative for every paid purchase route.
- Contradictory or missing deterministic winner facts must be durably quarantined before a benefit or settlement command; transient attempt-store failures remain retryable.
- Never select a different succeeded attempt or mutate the immutable winner to make validation pass.
- A nonempty missing or wrong-tenant QuoteID remains invalid; only an empty QuoteID represents the explicit legacy top-up form.
- Use only `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees-issue72/issue-84`; preserve the cumulative R1/R2 checkpoint and unrelated files.
- This is an uncommitted SDD checkpoint until independent task review passes; local commit is authorized after that review. No push or remote Issue mutation.

## Review Focus

1. A paid subscription event with a contradictory/missing winner and nil `purchaser` must be quarantined rather than remain pending; no outbound settlement occurs.
2. A quote-store read error must not mask a deterministic invalid winner; the winner check precedes quote classification.
3. A transient attempt lookup must leave the same event pending, and replay must succeed against the exact unchanged registered winner.
4. Valid subscription and top-up routes continue to pass through their existing fulfillment behavior after the shared guard.
5. Exact attempt/order/tenant scoping and all payment identity fields remain required; no fallback attempt is selected.

---

## Task 1 — Validate subscription winners before route dependencies

**Dependencies:** Cumulative R1/R2 checkpoint at BASE `ae8f57c8cb33b15f7ec78cab3758ae00d3bd3cbe`; failed R2 review `/tmp/issue72-84-cross-attempt-task-review-r3.md`; previous R2 report `.superpowers/sdd/issue-72-plan-84-ocr-cross-attempt-fix-r2/task-1-report.md`. **Owner:** backend_implementer. **Validator:** backend_validator. **Independent reviewer:** reviewer. **Checkpoint:** uncommitted until review passes.

**Write-owned files:**
- `internal/modules/commercial/service/commercial/fulfillment.go`
- `internal/modules/commercial/service/commercial/fulfillment_test.go`

**Cumulative review scope:** all R1/R2/R3 changes from BASE `ae8f57c8cb33b15f7ec78cab3758ae00d3bd3cbe`, including `internal/modules/commercial/repository/commercial/order_pg_test.go`.

**Consumes:** package-internal `winningPaymentTransaction(ctx, db, order, payload) (string, error)` and `errInvalidWinningAttempt` from `purchase_fulfillment.go`; current durable `quarantineFulfillEvent` and `completeEvent` paths.

**Produces:** an outer paid-purchase guard in `FulfillmentService.processEvent` before `subscriptionPurchase`, nil-`purchaser` pending return, or any benefit work. Deterministic invalid identity uses the existing durable quarantine disposition; any other attempt-store error leaves the exact event pending. The validated immutable transaction remains the payload consumed by `PurchaseFulfiller` on subscription fulfillment.

- [ ] **Step 1 — RED: nil-purchaser subscription mismatch.** Add `TestFulfillmentSubscriptionRejectsContradictoryWinnerBeforeNilPurchaser` using a paid subscription order, a real succeeded registered winner A, and the immutable fulfill event changed to contradictory transaction B. Construct the service with a nil purchaser. Assert the event becomes dead through the existing quarantine path, the paid order is unchanged, there are no fulfillment records, and no gateway or settlement calls occur. Run the named test and confirm it fails because the event currently remains pending.
- [ ] **Step 2 — RED: quote lookup cannot mask winner contradiction.** Add `TestFulfillmentSubscriptionRejectsContradictoryWinnerWhenQuoteReadFails`. Use a valid nonempty subscription QuoteID and contradictory event winner. Install a uniquely named Gorm query callback scoped to the quote table that returns a transient error, invoke `Recover`, and remove the callback with test cleanup. Assert deterministic winner quarantine still occurs, no purchaser/gateway calls occur, and the callback proves quote classification was not reached before winner validation. Confirm the test fails against the current ordering.
- [ ] **Step 3 — GREEN: move the common guard before route classification.** In `processEvent`, after verifying a paid order and before `subscriptionPurchase`, call `winningPaymentTransaction` for `OrderKindPurchase`. Quarantine only the deterministic invalid-winner and not-found cases using the established reason; keep other store errors pending. Remove the now-duplicated non-subscription-only guard. Keep subscription `PurchaseFulfiller` idempotency and top-up dispatch unchanged after validation. Run both new tests; expect PASS.
- [ ] **Step 4 — RED/GREEN: replay the unchanged winner after transient read failure.** Replace the destructive table-drop/re-register flow in `TestFulfillmentTopUpTransientWinningAttemptReadRetries` with a one-shot test-only Gorm callback that injects an error only for the exact payment-attempt lookup. After the first `Recover`, assert the event is pending, order remains paid, and no fulfillment records or gateway calls exist. Remove the callback, verify the original attempt row and immutable event payload hashes/fields are unchanged, replay the same event, and assert one successful fulfillment. Run this test and confirm the new unchanged-winner assertion fails before implementing the callback-based seam, then passes after.
- [ ] **Step 5 — Verify cumulative behavior.** Run `go test ./internal/modules/commercial/service/commercial -run '^(TestFulfillmentSubscriptionRejectsContradictoryWinnerBeforeNilPurchaser|TestFulfillmentSubscriptionRejectsContradictoryWinnerWhenQuoteReadFails|TestFulfillmentTopUpTransientWinningAttemptReadRetries|TestFulfillmentTopUpRejectsContradictoryWinningAttempt|TestFulfillmentTopUpMissingWinningAttemptIsQuarantined|TestPurchaseFulfillmentUsesOrderWinningAttemptAcrossAttempts|TestPurchaseFulfillmentFailsClosedOnContradictoryWinnerAttempt|TestPurchaseFulfillmentTransientAttemptReadKeepsEventPending)$' -count=1 -v`; expect PASS. Run `go test ./internal/modules/commercial/service/commercial -count=1` and `go test ./internal/modules/commercial/repository/commercial ./internal/modules/commercial/service/commercial -count=1`; expect both PASS. Run `git diff --check`; expect PASS. Record exact commands, output, cumulative file hashes, and that tagged PostgreSQL concurrency remains unverified if `SAAS_TEST_PG_DSN` is absent.
- [ ] **Step 6 — Independent review and ledger.** Generate the cumulative Review Package from the original BASE and include all R1/R2/R3 changes. Obtain separate Spec Compliance and Code Quality verdicts; resolve valid findings and re-review. Only after PASS, append the R3 checkpoint and evidence to `docs/plans/issue-72-ledger-84.md` in the integration worktree; do not mutate that ledger before passing review.

## Acceptance Mapping

| Requirement | Step | Evidence |
|---|---:|---|
| Every paid purchase validates its order-CAS winner before route dependencies | 1–3 | Nil-purchaser and quote-read-failure subscription regressions |
| Invalid deterministic winner is stopped before any benefit or settlement command | 1–3 | Dead event, unchanged paid order, no records or outbound calls |
| Transient attempt reads retry with the same persisted winner | 4 | Original attempt and payload remain unchanged across pending/replay |
| Valid top-up and subscription behavior remains intact | 5 | Existing focused regressions and service/repository package suites |
| No unrelated payment, schema, or API behavior changes | 5–6 | Two-file task ownership, diff check, cumulative independent review |

## Plan Self-Review

- The R2 review's medium finding maps to Steps 1–3; its low test-strength finding maps to Step 4.
- The shared predicate remains exact and is run before quote/purchaser dependencies; quote errors cannot hide deterministic winner contradictions.
- Transient DB errors stay pending; no value is inferred from missing rows or changed attempts.
- The task touches only the fulfillment event router and its tests, uses SQLite only, and owns no shared service or database state outside its isolated worktree.
- The tagged PostgreSQL race evidence remains distinct and is reported as an environment limit if its DSN is absent.
