# Issue #72 / #84 Merge Review Findings Repair Plan R1

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Implement one task from its Task Brief; do not dispatch additional agents.

**Goal:** Close the verified merge-review gaps in purchase quote routing, recoverable Attention state, over-payment event identity validation, and cancellation propagation.

**Architecture:** Keep the order-CAS winner as the single paid-purchase identity authority. Quote classification will accept only recognized singleton `subscription_fee` or `top_up` shapes for nonempty QuoteIDs; malformed, missing, unknown, or mixed quoted shapes will persist the existing `purchase_activation` Attention record and keep the same event Pending. Confirmation will atomically persist an anomaly row as the durable identity binding for every later successful collection before writing its over-payment outbox event. Disposal will validate the event envelope and registered attempt, then compare it with that independently stored immutable anomaly fact; it accepts either awaiting-disposition or already-resolved status because operators can resolve the synchronously visible anomaly before asynchronous outbox drain. It never resets disposition. This preserves the first transaction on the attempt while retaining later same-attempt collections. Direct purchase fulfillment will propagate cancellation from its defense-in-depth winner read.

**Tech Stack:** Go, Gorm, SQLite service/repository tests, existing outbox and fulfillment record stores.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` User Story 8 and acceptance 7; `docs/adr/0012-lago-as-commercial-billing-authority.md`; `docs/adr/0014-commercial-platform-single-deep-seam.md`; `CONTEXT.md`; #84 plans `issue-72-plan-84-cross-attempt-settlement-fix.md` and `issue-72-plan-84-cross-attempt-settlement-fix-r4.md`; merge review `/tmp/issue72-84-merge-delta-review-20260929.md`; validator `/tmp/issue72-84-merge-delta-validation-20260929.md`.

## Global Constraints

- “Duplicate, partial, mismatched, wrong-currency, late, and multiple-success payments never duplicate benefits.”
- “The order's guarded pending-to-paid transition and its atomic `fulfill:<orderID>` outbox event define the local winning payment fact. Never infer it from timestamp ordering or attempt IDs.”
- “Missing, malformed or contradictory winner identity must issue zero settlement/benefit commands.”
- “Transient storage failures remain retryable; deterministic bad outbox envelopes are quarantined as terminal `dead` events before routing or benefit work.”
- “A nonempty missing or wrong-tenant QuoteID remains invalid; only an empty QuoteID represents the explicit legacy top-up form.”
- Preserve the already reviewed exact winner guard, #82 subscription lifecycle delegation, over-payment `MerchantOrderID` semantics, and #86 monthly wallet priority.
- Modify only the four service implementation/test files listed in Task 1; do not change schema, public APIs, remote Issues, or other worktrees.

## Review Focus

1. Unknown or mixed frozen quote line items must never grant book-rate top-up benefits; test unknown and mixed item arrays.
2. Deterministic invalid quoted purchases must remain operator-visible and replayable; test Attention+Pending, idempotent replay, and zero benefits for missing/corrupt/empty/unknown/mixed snapshots.
3. A valid singleton `top_up` quote and an explicitly empty legacy QuoteID must preserve their existing top-up route; test both.
4. A contradictory over-payment envelope or attempt must create no anomaly and become Dead; test key, tenant, transaction, and MerchantOrderID mismatches while retaining the valid inline and legacy fallback cases.
5. Cancellation/deadline returned by the direct winner query must propagate rather than being classified as a retryable nil result; inject both sentinels.

## Task 1 — Fail closed on invalid quoted purchases and audit facts

**Dependencies:** The staged #84 merge and R4 exact-winner guard in worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees-issue72/lago-int`; baseline `626f83f7273a3ccdbb33a20b29354cdcd4a65935`. **Owner:** `backend_implementer`; **Validator:** `backend_validator`; **Review:** independent `reviewer`.

**Files:**
- Modify: `internal/modules/commercial/repository/commercial/order.go`
- Modify: `internal/modules/commercial/repository/commercial/payment_anomaly.go`
- Test: `internal/modules/commercial/repository/commercial/order_test.go`
- Modify: `internal/modules/commercial/service/commercial/fulfillment.go`
- Test: `internal/modules/commercial/service/commercial/fulfillment_test.go`
- Modify: `internal/modules/commercial/service/commercial/purchase_fulfillment.go`
- Test: `internal/modules/commercial/service/commercial/purchase_fulfillment_test.go`

**Consumes:** `winningPaymentTransaction(ctx, db, order, payload) (string, error)`, `persistPurchaseActivationState(ctx, db, now, orderID, tenantID, state) error`, `quarantineFulfillEvent`, `completeEvent`, `repocommercial.PaymentAttemptRow`, `PaymentAnomalyRow`, `recordPaymentAnomalyTx`, `OrderStore.ConfirmPayment`, and the current `overPaymentPayload`.

**Produces:** A quote classifier returning `(subscription bool, invalid bool, err error)` with tenant-scoped reads. Empty QuoteID is the explicit legacy top-up form. For nonempty QuoteID, accept exactly one `subscription_fee` item as subscription or exactly one `top_up` item as top-up; any other kind, mixed/multiple items, empty list, malformed JSON, or missing/wrong-tenant quote is invalid. A quote-store transient error remains retryable without Attention. Deterministic invalid quoted purchases use the shared `purchase_activation` Attention record and leave the same outbox event Pending; persistence errors propagate without acknowledging. Each later successful collection atomically writes its exact over-payment anomaly fact in `ConfirmPayment` with the outbox event, without changing the attempt's first `ProviderTransactionID`. Disposal validates canonical key and event/payload/order identity, then verifies the exact succeeded attempt and exact persisted immutable anomaly fact; both `awaiting_disposition` and `resolved` states are valid because the anomaly can be resolved before the outbox consumer runs. Other states or contradictory facts are invalid. For a second transaction on the same attempt, the anomaly row is the durable transaction binding; for a different attempt, both the attempt transaction and anomaly fact must match. Deterministic contradictions go Dead without creating a new anomaly; transient reads/writes remain Pending. The inner winner lookup returns context cancellation/deadline errors unchanged.

### Steps

- [x] **RED — quoted shape coverage.** Retain the existing singleton `top_up` fixture. Add tests for unknown kind, mixed kinds, multiple items, empty items, corrupt JSON, missing quote, and wrong-tenant quote. Before implementation, assert the malformed quoted cases do not fulfill, do not call the gateway, and surface one `purchase_activation` Attention record with a Pending event; replay to prove the unique record remains one. Assert transient quote-store failure stays Pending without Attention.
- [x] **RED — same-attempt collection and over-payment envelope coverage.** Extend `TestPaymentSecondSuccessfulTransactionOnSameAttemptPreservesOriginal` to assert the second transaction gets exactly one durable anomaly binding while the attempt retains the first transaction; duplicate confirmation creates no second anomaly. Add service coverage that confirms A then B on the same registered attempt, drains the B event to Sent, and records B with the registered `mo_` identity. Resolve the anomaly before draining and assert the event still becomes Sent without changing its resolved state. Keep table-driven mutations for canonical key, event/payload tenant/order, inline MerchantOrderID, and attempt binding; each deterministic mismatch becomes Dead without a new anomaly. Retain valid distinct-attempt and old no-inline fallback assertions.
- [x] **RED — direct cancellation coverage.** Inject `context.Canceled` and `context.DeadlineExceeded` from the payment-attempt query callback during direct `PurchaseFulfiller.Fulfill`; assert `errors.Is` for each.
- [x] **GREEN — classifier and disposition.** Restrict quoted line items to exact singleton recognized kinds. For deterministic invalid quote classification, persist shared Attention and return Pending; propagate write failure. Keep transient DB errors Pending without Attention and preserve empty QuoteID legacy routing.
- [x] **GREEN — durable later-collection binding.** Add transaction-local anomaly insertion using the existing `(provider, merchant, transaction)` unique identity and verify exact existing facts on conflict. In `ConfirmPayment`, on the order-already-paid successful-collection path, insert the `over_payment` anomaly fact and its outbox event in the same transaction while never replacing the first attempt transaction. In `disposeOverPayment`, validate canonical key and event/payload/order tenant/order/state, load the exact attempt, and verify the exact immutable over-payment row; accept only `awaiting_disposition` or `resolved`, preserving the stored state. If this is the winner attempt, require the anomaly row to bind the later transaction while the attempt row still binds the fulfill winner; if it is another attempt, require its recorded transaction to equal the event. Legacy payloads may recover MerchantOrderID only from the validated attempt. Contradictions quarantine Dead without creating an anomaly; transient DB errors preserve Pending. Keep the consumer idempotent against the anomaly already written by `ConfirmPayment`.
- [x] **GREEN — cancellation.** In the direct winner error branch, propagate cancellation and deadline before the generic retryable read classification.
- [x] **REVIEW CLEANUP — source comments.** Ensure `ClassifyPaymentAnomaly`, the `ConfirmPayment` payload comment, and `TestOverPaymentDrainConsumesEventIntoAwaitingDisposal` accurately say the producer atomically stores the anomaly with its outbox event and the consumer only validates/acknowledges it. In the linkless-pending test comment, describe the predicate as a non-empty `checkout_url` without embedding SQL single quotes that gofmt rewrites.
- [x] **Verify.** Run same-attempt and distinct-attempt over-payment regressions, legacy/inline MerchantOrderID cases, quoted-shape and invalid-quote Attention cases, cancellation cases, `TestFulfillEventFailsClosedWhenDiscriminationUnprovable`, all R4 winner tests, the service package, repository+service packages, `gofmt` over all owned files including `order_test.go`, and `git diff --check`. Expected result: all applicable checks pass; tagged PostgreSQL integration remains separately environment-gated.
- [x] **Report.** Implementation and validation reports record commands, results, touched files, source hashes and the PostgreSQL-tag limitation. The SDD progress ledger below retains both repair rulings and review history.

## Acceptance Mapping

| Requirement | Test evidence |
|---|---|
| Only recognized quote shapes route subscription or top-up | Singleton subscription/top_up pass; unknown/mixed/multiple/empty/corrupt fail closed |
| Invalid quoted paid orders remain operator-visible and replayable | Attention+Pending; exactly one activation record; no gateway calls |
| Transient quote store errors remain retryable | Pending; no Attention; no benefits |
| Over-payment fact identity cannot be spoofed by outbox fields while preserving same-attempt later transactions | Exact atomically committed anomaly binding plus contradictory-envelope cases |
| Merchant order identity semantics remain stable | Valid inline path and legacy re-read path write the registered `mo_` value |
| Cancellation remains a pass-ending signal | Both context sentinels propagated from direct winner query |

## Rulings

- **Ruling:** Treat a singleton `top_up` quote line as the recognized quoted top-up shape because the existing regression fixture already routes that exact shape and the task plan explicitly requires preserving verified quote behavior; reject unknown/mixed/multiple shapes. **Cost if wrong:** If that fixture encoded accidental behavior rather than a supported format, a narrow quoted shape remains payable until the Quote contract is formally tightened. The countermeasure is exact singleton matching and explicit tests.
- **Ruling:** Deterministic nonempty quoted-shape failures use Attention+Pending rather than Dead so operators can repair/reconcile the quote and replay without losing the payment activation recovery surface; transient reads stay Pending without Attention. **Cost if wrong:** A permanently invalid quote remains in the recoverable queue and needs operator handling rather than terminal quarantine.
- **Ruling (review-fix round 1):** The attempt row is the immutable winner record and cannot prove a second transaction on that same attempt. Persist the existing `PaymentAnomalyRow` atomically with the later-collection outbox event in `ConfirmPayment`; the consumer checks that row as the independent durable binding before acknowledging. Preserve exact transaction matching against the attempt for a different attempt. This avoids a schema migration and keeps the final operator anomaly surface. **Cost if wrong:** The over-payment anomaly becomes visible synchronously before outbox drain; if the event cannot later be consumed, the durable anomaly still needs reconciliation and may appear earlier than consumers expect. Requiring only the outbox payload/event key instead would silently trust two mutable fields and was rejected by review.
- **Ruling (review-fix round 1 amendment):** The consumer validates the immutable anomaly fact but accepts `awaiting_disposition` or `resolved`. `ConfirmPayment` now exposes the anomaly atomically before the asynchronous consumer drains its outbox; the existing admin resolve endpoint can legitimately change its state in that interval. A resolved exact fact therefore still proves the collection and the consumer marks the event Sent without reopening it. **Cost if wrong:** An unknown future state will be treated as a deterministic contradiction and quarantined until code explicitly supports it; the consumer must never rewrite anomaly disposition.

## Plan Self-Review

- Spec coverage: no unrecognized or mismatched payment identity can produce settlement/benefit; every valid later collection has an atomic durable anomaly binding; valid subscription, quoted top-up and explicit legacy top-up routes remain covered.
- Type and interface consistency: the existing quote classifier signature and shared winner/attention helpers are retained; no public or schema interface changes.
- Review Focus is mapped to concrete regression tests above.
- No placeholders remain; one task owns the shared service seam and its behavioral tests because those changes form one reviewable invariant.
