# Issue #82 T9 PaymentIntent Fixture Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the #82 T9 Lago settlement integration test emit its synthetic success webhook only for the PaymentIntent observed before settlement, so shared Stripe customers cannot select an unrelated historical success.

**Architecture:** Keep this test-only and bind the pre-settle unsettled, invoice-tagged PaymentIntent IDs to the post-settle succeeded list. Fail with diagnostics if the pre-settle gate or matching success is missing. No production code, payment behavior, or live stack operation changes.

**Tech Stack:** Go, existing `lago_integration` build tag, Stripe-compatible HTTP test fixture.

**Spec:** Approved billing behavior in `docs/specs/2026-09-20-lago-billing-migration-design.md`; Issue #82 accepted scope and residuals in `docs/plans/issue-72-ledger-82.md`; integration recovery target in `docs/plans/issue-72-dag.md` and `docs/plans/issue-72-execution-ledger.md`.

## Global Constraints

- The integration test remains build-tagged `lago_integration`.
- Keep production package behavior and public interfaces unchanged.
- A synthetic success webhook must identify the same existing unsettled PaymentIntent selected by settlement.
- Missing or ambiguous fixture identity fails closed; never fall back to an arbitrary succeeded PaymentIntent.
- Do not run Docker, PostgreSQL, network APIs, or live payment flows for this task.
- Preserve disclosed #82 limitations: real Alipay sandbox evidence remains unavailable and this fixture fix alone does not pass T9 live acceptance.
- Commit local task changes; do not push, merge, or change GitHub Issue state.

## Review Focus

- Historical succeeded PaymentIntents for a reused customer: only a pre-settle candidate ID may produce the webhook event.
- Candidate without invoice metadata or outside unsettled statuses: it must not qualify.
- No pre-settle candidate within the existing wait bound: fail with a useful timeout diagnostic.
- Candidate remains unsettled after settle or disappears: fail with expected and observed IDs.
- Multiple pre-settle candidates: allow only a post-settle success whose ID is in the captured candidate set and preserve existing server-side ambiguity behavior.

---

### Task 1: Bind the T9 synthetic webhook to pre-settle PaymentIntent identity

**Dependencies:** None. Independent repair of the integrated #82 test harness residual.

**Role:** `backend_implementer`; validator `backend_validator`; independent reviewer `reviewer`.

**Owned files:**
- Modify: `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go`
- Modify: `docs/plans/issue-72-ledger-82.md` only to record exact test-only repair evidence and retain live T9/AC4 gates.
- Modify: `docs/plans/issue-72-dag.md` and `docs/plans/issue-72-execution-ledger.md` only to record checkpoint, verification, and residual status.

**Interfaces:**
- Consumes: existing pre-settle bounded Stripe PaymentIntent list and current settle request.
- Produces: a set of candidate PI IDs observed before settle; after settle, a webhook event selected only from succeeded list rows whose PI ID belongs to that set and whose `lago_invoice_id` is nonempty.

**Implementation steps:**

- [ ] **Step 1: Add regression coverage for candidate identity selection.** Extract a small test-local helper if required to test selection. Assert that an older succeeded PI is rejected, a succeeded PI with a captured pre-settle ID is selected, and no match returns a descriptive error containing expected and observed IDs.
- [ ] **Step 2: Run the focused regression before the implementation.** Run `go test -tags lago_integration ./internal/modules/commercial/commercialplatform -run 'Test.*PaymentIntent.*(Candidate|Selection)' -count=1`. Expected: the new regression fails because selection currently accepts an unrelated succeeded row or the helper does not exist.
- [ ] **Step 3: Capture pre-settle candidates in the existing bounded wait.** Collect IDs only for rows with nonempty `metadata.lago_invoice_id` and status in `requires_payment_method`, `requires_action`, or `requires_confirmation`. Stop when the existing gate is observed; if the bound expires without candidates, fail with timeout and useful diagnostics.
- [ ] **Step 4: Filter post-settle succeeded rows by captured ID.** Keep the existing invoice metadata guard. Select only a succeeded row whose `id` belongs to the captured set. If no row matches, fail with expected candidate IDs and observed succeeded IDs. Do not use any historical succeeded PI as fallback.
- [ ] **Step 5: Run unit and integration-tag compilation checks.** Run `go test -tags lago_integration ./internal/modules/commercial/commercialplatform -run 'Test.*PaymentIntent.*(Candidate|Selection)' -count=1`, `go test ./internal/modules/commercial/commercialplatform`, and `go test -tags lago_integration ./internal/modules/commercial/commercialplatform -run '^$'`. Expected: focused regression and package suite pass; tagged test binary compiles without invoking live tests. Run `gofmt` on changed Go file and `git diff --check`.
- [ ] **Step 6: Update durable records.** Record the commit/checkpoint and exact passing commands in the #82 ledger and #72 execution ledger. Keep T9 live execution and real Alipay sandbox evidence marked incomplete until those separate environment-gated runs pass.
- [ ] **Step 7: Commit the reviewed task.** Commit only the owned files with message `test(commercial): bind Lago settlement webhook to gated intent`.

**Verification:** Test-local deterministic selection regression; full target Go package tests; integration-tag compile-only check; formatting and diff checks. No external resources are required.

**Acceptance:** An unrelated pre-existing succeeded PaymentIntent can never be selected for the synthetic webhook. The same pre-settle candidate succeeds selection after settle. Missing linkage fails closed. Production code is unchanged. The task records but does not claim to resolve the live T9 environment gate or AC4 sandbox residual.

**Failure handling:** If tagged compilation or package tests reveal an unrelated baseline failure, capture exact output and compare against BASE without editing unrelated code. If repository behavior shows settle may replace PI identities, stop and report evidence; do not weaken to arbitrary succeeded selection.
