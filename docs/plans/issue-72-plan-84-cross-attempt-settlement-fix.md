# Issue #84 Cross-Attempt Settlement Identity Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every retry of a paid order settles with the exact registered provider transaction that won the atomic pending-to-paid transition, even when later payment attempts also succeed.

**Architecture:** The unique `fulfill:<orderID>` outbox event is written in the same transaction as the winning order transition and already carries the winning attempt and payment fact. Purchase fulfillment will validate that immutable event identity against its order and registered PaymentAttempt, then use its transaction for the settlement payload and command key. It will never choose a succeeded attempt by lexical ID. No schema or CommercialPlatform API change is needed.

**Tech Stack:** Go, Gorm, SQLite repository/service tests, tagged PostgreSQL integration test.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` User Story 8 and acceptance 7 (duplicate, partial, mismatched, wrong-currency and multiple-success payments never duplicate benefits); `docs/adr/0012-lago-as-commercial-billing-authority.md`; `docs/adr/0014-commercial-platform-single-deep-seam.md`; `CONTEXT.md`. Source issue: https://github.com/1123786563/WeKnora-fork01/issues/84. Prior narrow repair: `docs/plans/issue-72-plan-84-ocr-attempt-id-fix.md`, commit `ae8f57c8cb33b15f7ec78cab3758ae00d3bd3cbe`. Triggering review: `/tmp/issue72-84-attempt-identity-final-review.md`; read-only design: `/tmp/issue72-84-cross-attempt-design-20260928.md`.

## Global Constraints

- “I want payment success and entitlement activation to be shown as separate states.” (Spec User Story 5.)
- “Duplicate, partial, mismatched, wrong-currency, late, and multiple-success payments never duplicate benefits.” (Spec acceptance 7.)
- Retain every external payment fact for audit; do not change Invoice amount, delivered benefits, provider callbacks, Quote rules or Lago settlement semantics.
- The order's guarded pending-to-paid transition and its atomic `fulfill:<orderID>` outbox event define the local winning payment fact. Never infer it from timestamp ordering or attempt IDs.
- Missing, malformed or contradictory winner identity must issue zero settlement/benefit commands. Transient storage failures remain retryable; deterministic bad outbox envelopes are quarantined as terminal `dead` events before routing or benefit work, with a warning that records the event key and bounded reason while preserving the original payload for investigation. Dead events remain queryable through the recovery queue; do not replay until the underlying data is verified.
- Keep SQL parameters bound. No schema migration or public interface change is authorized by this repair.
- Work only in the existing isolated Issue #84 worktree; do not alter shared Lago services, another worktree, remote Issues, or push.

## Review Focus

1. **Two attempts with inverse lexical IDs:** Task 1 must prove later overpayment cannot change the winner's settlement payload/key.
2. **Either callback wins:** Task 1's PostgreSQL race must prove exactly one order-CAS winner and a fulfill event that carries the same payment fact.
3. **Retry after authority timeout:** Task 1 must replay the same fulfill outbox event and assert every settlement attempt uses the winner transaction and key.
4. **Corrupt or mismatched event/attempt identity:** Task 1 must fail closed before route selection, with no settlement/benefit command, a durable dead-event quarantine, and continued processing of later events in the same drain.
5. **Integration test setup failure:** Task 1 must register cleanup immediately after schema creation and not leak the schema if later setup fails.

## Task 1: Bind purchase settlement to the order-CAS winning payment fact

**Dependencies:** Prior attempt-identity repair `ae8f57c8cb33b15f7ec78cab3758ae00d3bd3cbe` in this same worktree; approved Spec User Story 8. **Owner:** backend implementation (controller uses the documented fallback below if the specialist role cannot progress); **Validator:** backend_validator; **Independent review:** reviewer.

**Files:**
- Modify: `internal/modules/commercial/service/commercial/fulfillment.go` — extend private `fulfillEventPayload` to decode the registered payment identity and optional quote; validate kind, canonical event key, payload/order/tenant/amount/currency consistency before any routing; compare quote when present, tenant-scope quote lookup, and quarantine malformed or contradictory envelopes as `dead` without aborting the remaining drain batch.
- Modify: `internal/modules/commercial/service/commercial/purchase_fulfillment.go` — validate and consume the outbox winner; remove lexical `succeededAttempt` selection.
- Test: `internal/modules/commercial/service/commercial/purchase_fulfillment_test.go` — multi-attempt winner/retry and fail-closed identity regressions.
- Test: `internal/modules/commercial/repository/commercial/order_pg_test.go` — order-CAS concurrency and outbox winner assertions; make setup cleanup-safe.
- Update after independent task review: `docs/plans/issue-72-ledger-84.md` with checkpoint and evidence.

**Consumes:** `repocommercial.OutboxEvent`, private `fulfillEventPayload`, `PaymentAttemptRow`, `PurchaseFulfiller.Fulfill(ctx, ev)`, existing `markActivationState`, and `domain.SettlePurchasePaymentCommandKey`.

**Produces:** For every valid paid fulfillment event, the exact transaction in its `fulfill:<orderID>` payload is cross-checked against its registered succeeded attempt and used for both `ChannelTransaction` and command key. No successful-attempt query may select another attempt by ID ordering.

- [ ] **Step 1 — RED, two-attempt delayed retry.** Add `TestPurchaseFulfillmentUsesOrderWinningAttemptAcrossAttempts`. Register winner `z-winner` and later over-payment `a-later`. Confirm winner transaction A so the order CAS creates the fulfill event. Run the first fulfillment pass while only A has succeeded; capture its settle command/key and make the authority return `ErrPlatformUnreachable`, leaving the event pending. Then confirm transaction B on `a-later`, producing one over-payment fact, and replay the same fulfill event. Assert both settle commands carry A and `SettlePurchasePaymentCommandKey(..., A)`, the outbox payload identifies A, the order fulfills once, and exactly one grant exists. This puts B's success specifically between the two settlement attempts and makes the pre-fix lexical selector choose B on retry.
- [ ] **Step 2 — Characterization, whichever callback wins.** Add `TestPaymentConcurrentDistinctAttemptsStoreOnlyOrderCASWinner` under `//go:build commercial_integration`. Use an isolated per-test PostgreSQL schema and a Gorm callback barrier immediately before the guarded `commercial_orders` update so two distinct registered attempts have both claimed their own transaction and are waiting at the order CAS. After release, assert one paid transition, one fulfill event whose `attempt_id` and `transaction` match the winning attempt, one over-payment event containing the other transaction, and no duplicate events. Ensure each goroutine result has a bounded deadline. The current producer already writes the CAS winner atomically, so this characterization test should pass before the reader repair; do not alter correct producer behavior to force a RED.
- [ ] **Step 3 — RED, invalid routing envelope and transient winner reads.** Add a `FulfillmentService.Recover` regression with a purchase fulfill event whose payload names another existing order, another with a contradictory tenant, and another with malformed JSON. The event key remains tied to the original purchase. Assert each is quarantined as `dead` before route selection, no command/grant or wrong-order activation occurs from its payload, and a following valid top-up event in the same pass is still processed. Add `TestPurchaseFulfillmentFailsClosedOnContradictoryWinnerAttempt` for a well-formed envelope whose attempt transaction/ID contradicts the registered winner; assert zero settlement commands, no grant, and attention. Add `TestPurchaseFulfillmentTransientAttemptReadKeepsEventPending`: inject one query error only for `commercial_payment_attempts`, call Recover, and assert the event remains pending with zero settle commands; remove the callback and recover again to verify it can fulfill.
- [ ] **Step 4 — Implement validation before routing and winner use.** Derive the order ID only from an exact canonical `fulfill:<orderID>` event key. Before quote-based subscription routing, a gateway or purchaser, validate the event kind/key and decode the envelope; require payload order ID, tenant, amount and currency to match the event and the row loaded using the event-key order ID plus tenant. Compare `quote_id` when present; older serialized fulfill payloads may omit it, so the verified order row remains the quote authority. A malformed/noncanonical or contradictory envelope is completed to `OutboxStateDead` (durable quarantine), logs only its event key and a bounded reason, and returns nil so the shared drain continues; it must never route using payload order ID. Read quotes with both quote ID and verified order tenant. Return a distinct invalid-shape result when a purchase quote is missing, malformed or contradictory so it cannot fall through to top-up fulfillment. Preserve a legacy top-up exception only when the order/quote shape is explicitly verified by tests. For a valid subscription purchase, `PurchaseFulfiller.Fulfill` cross-checks any supplied payment identity fields and requires a complete winning attempt/provider/merchant/transaction tuple, then loads the attempt by bound `attempt_id` plus order and tenant. Require succeeded state, provider, merchant, provider transaction, amount and currency to match the event, attempt and order. Deterministic missing/contradictory attempt identity calls the existing attention transition and returns without a platform command; transient DB failure returns nil with event pending/retryable. Use the validated transaction for the stable settle command key and payload. Delete or narrow `succeededAttempt`; no fallback to `ORDER BY id` is allowed. Historical events without winner fields require a deployment-side census from retained database evidence before rollout; if evidence is unavailable or any event is ambiguous, explicitly block rollout or leave that event in attention for verified reconciliation. An isolated test DB is not evidence of deployed queue contents.
- [ ] **Step 5 — Fix integration-test cleanup on every setup path.** In `testOrderPGStore`, immediately after `admin.DB()` succeeds register `adminPool.Close`; after schema creation succeeds register schema drop; after schema-scoped `db.DB()` succeeds register scoped pool close. Since Go cleanup runs LIFO, register resources in that order so cleanup closes scoped pool, drops schema, then closes admin pool. Ensure each resource closes once, including failures in schema creation, scoped connection, migration and index creation. Keep internally generated schema identifiers as the only unbound SQL identifiers; data queries remain parameterized.
- [ ] **Step 6 — GREEN and regression verification.** Run:
  - `go test ./internal/modules/commercial/service/commercial -run '^(TestPurchaseFulfillmentUsesOrderWinningAttemptAcrossAttempts|TestPurchaseFulfillmentFailsClosedOnContradictoryWinnerAttempt|TestPurchaseFulfillmentTransientAttemptReadKeepsEventPending|TestFulfillmentRecoverQuarantinesMalformedPurchaseEventsAndContinues)$' -count=1 -v` — focused regressions PASS.
  - `go test ./internal/modules/commercial/repository/commercial -run '^TestPaymentConcurrentDistinctAttemptsStoreOnlyOrderCASWinner$' -count=1 -v` — excluded by build tag in ordinary run; report exact selection.
  - `go test -tags commercial_integration ./internal/modules/commercial/repository/commercial -run '^TestPaymentConcurrentDistinctAttemptsStoreOnlyOrderCASWinner$' -count=1 -v` with isolated `SAAS_TEST_PG_DSN` — PASS without skip.
  - `go test ./internal/modules/commercial/repository/commercial ./internal/modules/commercial/service/commercial -count=1` — both packages PASS.
  - `git diff --check` — no output.
  Record output, current source hashes and exact commit/checkpoint range in the task report.
- [ ] **Step 7 — Review and ledger.** Prepare a task Review Package from the recorded Task BASE, obtain separate Spec Compliance and Code Quality review, fix only valid findings within SDD limits, run affected tests and scoped re-review, then append evidence to `docs/plans/issue-72-ledger-84.md`. Commit only after independent task review passes; do not push.

## Acceptance Mapping

| Requirement | Step | Evidence |
|---|---|---|
| Multiple-success payments do not duplicate benefits and excess payment stays auditable | 1, 2 | One winning fulfill event, one loser audit, one grant |
| Settlement key remains idempotent under delayed/retried fulfillment | 1 | Every retry uses the order-CAS winner transaction and key |
| Contradictory source facts fail closed before routing | 3, 4 | No outbound benefit/settle command; durable dead-event quarantine; subsequent drain event still processed |
| PostgreSQL order-CAS winner determines durable outbox identity | 2 | Isolated barrier test ties the emitted outbox payload to exactly one winning transition |
| Integration test resources clean up on setup failure | 5 | Scoped test setup and cleanup review; tagged test passes |

## Plan Self-Review

- Spec coverage: the plan handles the multiple-success/no-duplicate-benefit requirement and delayed settlement idempotency without modifying payment collection, Quote, Invoice, provider or grant contracts. It does not redefine “winner”: the existing successful pending-to-paid CAS remains the rule.
- Task consistency: all files belong to one task because the immutable winner producer, outbox decoder, consumer, race evidence and service behavior form one testable invariant. There are no downstream tasks or undeclared interfaces.
- Failure handling: transient database read failures keep the outbox retryable; malformed/contradictory event envelopes are durably quarantined before routing and log a bounded diagnostic; deterministic winner-attempt contradictions surface attention. Legacy events without a recoverable winner remain a deployment gate. No lexical or timestamp fallback exists.
- Review focus coverage: inverse IDs, either order-CAS winner, settlement retry, contradictory payload and setup cleanup each have a named regression or direct test assertion.
