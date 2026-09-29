# Issue #82 T9 Concurrent PaymentIntent Fail-Closed Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent the T9 fixture from synthesizing a webhook when a newly appeared successful PaymentIntent means the production command may have selected a different PI than the fixture's pre-command prediction.

**Architecture:** Preserve the complete set of PaymentIntent IDs observed in the same pre-settle Stripe list response used to resolve the expected latest candidate. After settlement, require the expected target ID to be succeeded, and fail closed if any invoice-linked succeeded PI appears whose ID was absent from the pre-settle list. Keep this confined to the tagged integration-test harness.

**Tech Stack:** Go, existing `lago_integration` build tag, Stripe-compatible HTTP fixture.

**Spec:** `docs/plans/issue-72-plan-82-t9-fixture-r1.md`; final review finding F1 in `.superpowers/sdd/issue-72-plan-82-t9-fixture-r1/final-review.md`; approved billing design in `docs/specs/2026-09-20-lago-billing-migration-design.md`.

## Global Constraints

- Keep changes in `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go` and the task's plan/evidence records; do not change production settlement behavior or interfaces.
- The integration test remains behind the `lago_integration` build tag.
- The synthetic webhook may represent only the exact expected pre-settle latest PI ID, and only if that PI appears succeeded with nonempty `lago_invoice_id` after settle.
- If a post-settle invoice-linked succeeded PI was not present in the pre-settle response, fail before delivering any synthetic webhook.
- No Docker, PostgreSQL, network API, or live payment flow for this task.
- Preserve the existing explicit residuals: dedicated live T9 rerun and real Alipay sandbox AC4 evidence remain incomplete.
- Local commits are authorized; do not push, merge, or alter GitHub Issue state.

## Review Focus

- A new PI is created after the pre-settle read and then independently/command-side succeeds: do not emit a synthetic event for the old predicted target.
- Historical succeeded PI already present in the pre-settle list: preserve existing expected-target selection when no new PI appears.
- Newly appearing PI has no `lago_invoice_id`: ignore it for this settlement identity guard.
- Expected pre-settle latest PI is absent or not succeeded after settle: fail closed with expected and observed IDs.

---

### Task 1: Reject newly appeared succeeded PaymentIntent identities

**Dependencies:** The T9 identity repair branch and fix-round-1 review are already integrated in this worktree; this repair closes the final whole-branch review finding only.

**Role:** `backend_implementer`; validator `backend_validator`; independent reviewer `reviewer`.

**Owned files:**
- Modify: `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go`
- Modify: `docs/plans/issue-72-ledger-82.md` and `docs/plans/issue-72-dag.md` only for accurate repair evidence/checkpoint/residual status.
- Modify: `docs/plans/issue-72-execution-ledger.md` only for execution checkpoint and review evidence.

**Interfaces:**
- Consumes: pre-settle `expectedIntentID` and all PI IDs observed in the same pre-settle `data` response.
- Produces: a succeeded expected PI row only if its ID equals `expectedIntentID`, it has invoice metadata, and no post-settle succeeded invoice-linked PI has an ID absent from the pre-settle ID set.

**Implementation steps:**

- [ ] **Step 1: Add a pure regression for an interleaving PI.** Given pre-settle IDs containing the old/latest predicted PI and post-settle rows where that predicted PI and a newly appeared invoice-linked PI are both succeeded, assert selection fails and reports the newly observed identity. Also assert a pre-existing historical success is allowed beside the expected target, and a newly appeared success without invoice metadata does not trigger this invoice-identity guard.
- [ ] **Step 2: Run the tagged focused regression before implementation.** Run `go test -tags lago_integration ./internal/modules/commercial/commercialplatform -run 'Test.*PaymentIntent.*(Candidate|Selection)' -count=1`. Expected: the new interleaving assertion fails because the current selector does not receive pre-observed full PI IDs.
- [ ] **Step 3: Capture the pre-settle response identity set.** From the same `data` slice used by `preSettlePaymentIntentCandidates`, retain every nonempty PI ID, including already-succeeded rows, in a deterministic map/set. Pass that set and the exact `expectedIntentID` to post-settle selection.
- [ ] **Step 4: Fail closed on a newly appeared succeeded invoice PI.** Before returning the expected succeeded PI, scan post-settle rows. If a row has `status == succeeded`, nonempty `metadata.lago_invoice_id`, and an ID absent from the pre-settle set, return a descriptive error naming the unexpected ID(s). Keep exact expected-ID, status, and metadata checks.
- [ ] **Step 5: Verify the task.** Run the focused tagged selector regression, `go test ./internal/modules/commercial/commercialplatform -count=1`, `go test -tags lago_integration ./internal/modules/commercial/commercialplatform -run '^$'`, gofmt, and `git diff --check`. No command may contact a live service.
- [ ] **Step 6: Update durable evidence.** Record RED/GREEN results, exact commit/checkpoint, independent review and validation status, and keep live T9/AC4 gates incomplete.
- [ ] **Step 7: Commit only owned changes.** Use commit subject `test(commercial): fail closed on concurrent T9 intent`.

**Verification:** Pure selector regression, full target package tests, integration-tag compile-only check, gofmt and diff check. No external resources are required.

**Acceptance:** The fixture cannot deliver a synthetic success webhook when the post-settle list contains a newly appeared invoice-linked succeeded PI; the expected old target's success cannot mask that identity race. Existing pre-observed historical successes remain supported. Production code is unchanged, and live T9/AC4 remain explicitly incomplete.

**Failure handling:** If evidence shows the provider intentionally creates a replacement PI during the settle command, stop and report that changed contract; do not silently accept a newly appearing PI as the test target.
