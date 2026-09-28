# Issue #84 Cross-Attempt Settlement Repair R4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Implement one task from its Task Brief; do not dispatch additional agents.

**Goal:** Validate the order-CAS winning payment identity before quote or purchaser routing can defer disposition, while preserving subscription Attention and replaying transient reads against unchanged durable facts.

**Architecture:** Keep `winningPaymentTransaction(ctx, db, order, payload)` as the exact predicate for registered attempt, order, tenant, succeeded state, provider, merchant, transaction, amount, and currency. Run it in `FulfillmentService.fulfillEvent` before quote classification for every paid purchase. A deterministic mismatch on an explicitly legacy empty-QuoteID top-up remains a dead event; a mismatch on a quoted purchase records the existing `purchase_activation` Attention state and leaves the event pending, even when quote storage or purchaser wiring is unavailable. Transient attempt reads remain pending without an Attention record.

**Tech Stack:** Go, Gorm, SQLite service tests, tagged PostgreSQL integration test.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` User Story 8 and payment reconciliation acceptance; `docs/adr/0012-lago-as-commercial-billing-authority.md`; `docs/adr/0014-commercial-platform-single-deep-seam.md`; `CONTEXT.md`; prior plans `docs/plans/issue-72-plan-84-cross-attempt-settlement-fix.md`, `docs/plans/issue-72-plan-84-ocr-cross-attempt-fix-r2.md`, and `docs/plans/issue-72-plan-84-cross-attempt-settlement-fix-r3.md`; reviews `/tmp/issue72-84-cross-attempt-task-review-r2.md`, `/tmp/issue72-84-cross-attempt-task-review-r3.md`, and `/tmp/issue72-84-cross-attempt-plan-r3-review.md`.

## Global Constraints

- The order-CAS winner persisted in the `fulfill:<orderID>` outbox event is authoritative for every paid purchase route.
- Contradictory or missing deterministic winner facts must be durably surfaced before a benefit or settlement command; transient attempt-store failures remain retryable.
- Never select a different succeeded attempt or mutate the immutable winner to make validation pass.
- A nonempty missing or wrong-tenant QuoteID remains invalid; only an empty QuoteID represents the explicit legacy top-up form.
- Use only `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees-issue72/issue-84`; preserve all cumulative R1/R2 edits and unrelated files.
- This remains an uncommitted SDD checkpoint until independent review and validation pass. Local commit is authorized after those gates. Do not push, merge, deploy, call live payment services, or mutate remote Issues.

**Preflight Ruling:** When the order has a nonempty QuoteID, a deterministic invalid winner is surfaced as the existing purchase-activation Attention record and the event remains Pending before quote lookup, even if the Quote row is unavailable or purchaser wiring is nil. This preserves the established operator-visible subscription failure state without permitting benefit or settlement calls. Cost if this is broader than the affected subscription cases: a quoted top-up with an invalid winner may also acquire a purchase-activation Attention record and require operator reconciliation rather than being Dead-lettered immediately. An explicitly legacy empty-QuoteID top-up retains Dead/no-record behavior.

## Review Focus

1. Subscription winner mismatch must create the established Attention record before quote or purchaser dependencies and leave the same outbox event Pending.
2. Transient winner lookup errors must leave the original attempt row and immutable event payload unchanged, with no Attention record or benefit calls; replay after recovery must fulfill once.
3. Empty-QuoteID top-up winner mismatch or missing attempt must be Dead with no fulfillment record or gateway calls, while the next valid event in the same drain continues.
4. Nonempty missing/wrong-tenant quote never authorizes top-up fallback; valid subscription and top-up behavior remains intact.
5. Every stored or outbound settlement transaction remains the exact order-CAS winner; no lexical attempt selection or provider API change is introduced.

---

## Task 1 — Move winner validation ahead of purchase route dependencies

**Dependencies:** Cumulative R1/R2 checkpoint at `ae8f57c8cb33b15f7ec78cab3758ae00d3bd3cbe`; R3 plan review rejected before implementation. **Role:** `backend_implementer`; **validator:** `backend_validator`; **reviewer:** independent `reviewer`.

**Write-owned files:**
- `internal/modules/commercial/service/commercial/fulfillment.go`
- `internal/modules/commercial/service/commercial/purchase_fulfillment.go`
- `internal/modules/commercial/service/commercial/fulfillment_test.go`

**Cumulative Review Package scope:** all changes from BASE `ae8f57c8cb33b15f7ec78cab3758ae00d3bd3cbe`, including the already changed `internal/modules/commercial/repository/commercial/order_pg_test.go`, `internal/modules/commercial/service/commercial/purchase_fulfillment_test.go`, and the three write-owned files above. No other file may be added to the checkpoint.

**Consumes:** `FulfillmentService.fulfillEvent`, `winningPaymentTransaction`, `errInvalidWinningAttempt`, `PurchaseFulfiller.markActivationState`, `completeEvent`, `quarantineFulfillEvent`, and existing `FulfillmentRecord` key/state.

**Produces:** a shared pre-route winner guard. Extract the DB insertion currently implemented by `PurchaseFulfiller.markActivationState` into a package-private helper accepting `db`, `now`, verified order ID, tenant ID, and state; keep `PurchaseFulfiller.markActivationState` delegating to that helper. For an invalid/missing winner, an explicitly legacy order with `row.QuoteID == ""` is durably quarantined as Dead and creates no fulfillment record. A quoted order with nonempty QuoteID is conservatively surfaced as `purchase_activation` Attention and leaves the outbox event Pending before reading the quote; this includes nil purchaser and quote-read failure cases. A transient attempt read returns Pending with no Attention record. A persistence error while writing Attention is propagated and must not acknowledge the event. Valid winners continue to quote classification and existing routing; the inner `PurchaseFulfiller` check stays as defense in depth for direct calls.

- [ ] **Step 1 — Correct the transient test seam.** Replace `TestFulfillmentTopUpTransientWinningAttemptReadRetries` table drop/recreate/re-register logic with a one-shot Gorm query callback scoped to `commercial_payment_attempts`. Capture the original attempt row and exact fulfill `PayloadJSON` before first recovery. Assert callback fired once, event is Pending, same original attempt and payload remain byte/field-identical, no Attention record exists, order stays Paid, and no gateway/fulfillment effects occurred. Remove callback, replay the same event and assert one successful fulfillment and one benefit. This is a test-strength correction; production already leaves transient lookup errors retryable, so no false RED claim is required.
- [ ] **Step 2 — RED: subscription mismatch before missing purchaser.** Add `TestFulfillmentSubscriptionRejectsContradictoryWinnerBeforeNilPurchaser`. Seed a real quoted subscription, succeeded registered winner A, and alter only the leased event’s winner transaction to B. Construct the service with `purchaser == nil`. Assert Pending event, one durable `purchase_activation` Attention record, paid order unchanged, no fulfillment/grant/gateway/settlement side effect. Demonstrate the existing code returns Pending without the Attention record.
- [ ] **Step 3 — RED: quote store outage cannot mask mismatch.** Add `TestFulfillmentSubscriptionRejectsContradictoryWinnerWhenQuoteReadFails` with the same real paid subscription and mismatched event winner. Install a uniquely named Gorm query callback for the quote table that counts/fails reads. Assert it is never reached, the event is Pending with one Attention record, and no external command/benefit is issued. Demonstrate failure before code change.
- [ ] **Step 4 — GREEN: extract and call the shared Attention persistence helper.** Add the private helper in `purchase_fulfillment.go` and delegate `PurchaseFulfiller.markActivationState` to it. In `fulfillEvent`, run `winningPaymentTransaction` after canonical envelope/order/state validation and before `subscriptionPurchase`. For deterministic invalid winner, use the empty/nonempty QuoteID disposition above; return the same pending/dead states and propagate helper/database errors. For other attempt-store errors, leave pending. Only a valid winner reaches quote lookup or purchaser nil handling. Add `TestFulfillmentSubscriptionAttentionWriteFailureDoesNotAcknowledge`: inject a one-shot GORM Create error scoped to `commercial_fulfillment_records`; assert first `Recover` returns that error, the same event remains `Pending` with its lease token nonempty and lease expiry still in the future, there is no Attention row, and no gateway/settlement calls occur. Remove the callback, advance `svc.now` past that captured lease expiry, recover again against the unchanged attempt/event payload, and assert the same event remains `Pending`, its `lease_until` is due (the existing `completeEvent` releases by expiry and does not blank the token), exactly one Attention record exists, and no outbound calls occurred. Extend the successful nil-purchaser mismatch test to call `Recover` again and assert the unique activation key still has exactly one Attention record and the event remains Pending.
- [ ] **Step 5 — Verify cumulative regression behavior.** Run the focused R4 tests and all prior named #84 cross-attempt regressions. Run the five historical service cases addressed by the current fixture migration: `TestFulfillmentWorkerSavedThenDroppedRecoversExactlyOnce`, `TestFulfillmentWorkerConcurrentClaimsApplyExactlyOnce`, `TestFulfillmentWorkerPaidNotFulfilledStaysRecoverable`, `TestOverPaymentDrainConsumesEventIntoAwaitingDisposal`, `TestOverPaymentDisposeFailureDoesNotBlockFulfillDrain`, plus `TestOverPaymentDrainReplayYieldsSingleAnomaly`. Then run `go test ./internal/modules/commercial/service/commercial -count=1`, `go test ./internal/modules/commercial/repository/commercial ./internal/modules/commercial/service/commercial -count=1`, and `git diff --check`; expect PASS.
- [ ] **Step 6 — Record environment-gated evidence and review.** Run the ordinary package selection for `TestPaymentConcurrentDistinctAttemptsStoreOnlyOrderCASWinner` and record build-tag exclusion. Attempt the tagged test only if `SAAS_TEST_PG_DSN` is already configured for an isolated database; otherwise report it BLOCKED without inventing a DSN. Generate the cumulative BASE-to-checkpoint Review Package with the three exact verdicts (Spec Compliance, Code Quality, verification status). Fix valid findings within the SDD loop and obtain scoped re-review. Only after task review passes, update `docs/plans/issue-72-ledger-84.md`; do not push.

## Acceptance Mapping

| Requirement | Step | Evidence |
|---|---:|---|
| Every paid purchase validates the immutable order-CAS winner before route dependencies | 2–4 | Nil-purchaser and quote-read-error subscription regressions |
| Invalid quoted purchase winner remains operator-visible under the established Attention contract | 2–4 | One Attention record; event Pending; no external effects |
| Transient attempt read retries the same persisted winner | 1, 5 | Same attempt/payload before and after the injected read error; one eventual fulfillment |
| Legacy top-up mismatch/missing identity never grants benefits | 5 | Dead event, no record or gateway call, later valid event continues |
| Existing fulfillment behavior remains green | 5 | Named fixture regressions and both full Go packages |
| PostgreSQL race evidence is not overstated | 6 | Tagged run result or explicit environment block |

## Plan Self-Review

- The R2 medium fixture failure is addressed in the already present `fulfillment_test.go` checkpoint by explicitly setting the legacy `QuoteID` empty while retaining each real registered/confirmed winner; the six relevant names and reproducible commands are listed in Step 5.
- The R2 high finding's top-up mismatch remains Dead/no record; the R3 high finding's subscription attention contract is preserved through a shared helper usable with nil purchaser.
- The R3 transient replay expectation is corrected to a characterization test: the callback is test-only, so it is not claimed to expose a production RED; the durable attempt and payload are measured unchanged.
- The exact production seam is `FulfillmentService.fulfillEvent`, not a nonexistent `processEvent`.
- Scope is one task because winner validation, route ordering, durable Attention state and event replay form one indivisible invariant. No public API, schema or payment producer change is authorized.
