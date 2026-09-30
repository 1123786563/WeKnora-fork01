# Issue #82 T9 PaymentIntent Identity-Set Coverage Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a deterministic regression proving the T9 fixture retains every nonempty PaymentIntent ID from the pre-settle response, including historical succeeded and unlinked intents.

**Architecture:** Reuse one pre-settle response slice for candidate parsing and `paymentIntentIDSet`, then assert the exact identities retained before passing the set into post-settle selection. Keep the change in the existing `lago_integration` test harness; production behavior stays unchanged.

**Tech Stack:** Go, existing `lago_integration` build tag, Go standard testing package.

**Spec:** Approved billing design `docs/specs/2026-09-20-lago-billing-migration-design.md`; Issue #82 T9 plans `docs/plans/issue-72-plan-82-t9-fixture-r1.md` and `docs/plans/issue-72-plan-82-t9-concurrent-pi-fail-closed-r1.md`; review finding L1 in `/tmp/issue72-82-t9-candidate-independent-review-20260930.md`; current acceptance ledger `docs/plans/issue-72-ledger-82.md`.

## Global Constraints

- Keep changes in `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go` and the task's plan/evidence records; do not change production settlement behavior or interfaces.
- The integration test remains behind the `lago_integration` build tag.
- The synthetic webhook may represent only the exact expected pre-settle latest PI ID, and only if that PI appears succeeded with nonempty `lago_invoice_id` after settle.
- If a post-settle invoice-linked succeeded PI was not present in the pre-settle response, fail before delivering any synthetic webhook.
- No Docker, PostgreSQL, network API, or live payment flow for this task.
- Preserve the explicit residuals: dedicated live T9 rerun and real Alipay sandbox AC4 evidence remain incomplete.
- Local commits are authorized; do not push, merge, or alter GitHub Issue state.

## Review Focus

- `paymentIntentIDSet` must retain all nonempty IDs independent of status or invoice metadata, including `pi_already_done` and `pi_unlinked`.
- Candidate parsing and the ID-set helper must consume the same pre-settle response slice.
- The test must fail under the plausible regression that the ID set filters to unsettled invoice-linked candidates.
- No production code or live test path may be changed or invoked.
- Preserve the live T9 and AC4 acceptance gaps as open.

## Task DAG

```text
Task 1: direct pre-settle ID-set regression -> final whole-range review
```

### Task 1: Assert complete pre-settle identity capture

**Dependencies:** The reviewed T9 fixture checkpoint is `d8d21cd967c94c09d32c34a2981904758f442330` on `codex/issue-72-82-t9-test-fixture-r1`. The independent whole-range review passed with one retained Low L1: the test manually constructs `preObservedIDs` rather than exercising `paymentIntentIDSet`.

**Role:** `backend_implementer`; validator `backend_validator`; independent reviewer `reviewer`.

**Owned files:**

- Modify `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go`.
- Append one concise result to `docs/plans/issue-72-ledger-82.md` and `docs/plans/issue-72-execution-ledger.md`.
- Do not modify production Go files, earlier candidate worktrees, or any other path.

**Interfaces:**

- Consumes: `paymentIntentIDSet(rows []any) map[string]struct{}` and `preSettlePaymentIntentCandidates(rows []any)`.
- Produces: a test that proves IDs `pi_older`, `pi_newest`, `pi_unlinked`, and `pi_already_done` all survive the helper from the same pre-settle `[]any`; the existing `succeededPaymentIntent` assertions then use that actual helper result.

**TDD note:** This is a test-only coverage task for behavior already implemented. The clean BASE focused test passes. Do not introduce a production change solely to manufacture RED. After writing the regression first, run it against the clean helper, then perform a negative-control check in the isolated task worktree by temporarily mutating the helper to keep only unsettled invoice-linked rows; the new assertion must fail naming the omitted `pi_unlinked` and `pi_already_done`. Restore the exact original helper before GREEN verification and confirm the tracked diff contains only the new/updated test plus evidence files.

**Implementation steps:**

- [x] Store the four-row pre-settle response in a `preSettleRows` variable and pass that same value to both candidate parsing and `paymentIntentIDSet`.
- [x] Assert the helper result contains each of the four IDs and has exactly four entries; then replace the hard-coded `preObservedIDs` map in the existing candidate-selection test with this helper result.
- [x] Run the focused tagged test `go test -tags lago_integration ./internal/modules/commercial/commercialplatform -run 'Test.*PaymentIntent.*(Candidate|Selection)' -count=1`; PASS before and after the negative control.
- [x] In a temporary local mutation only, make the helper discard `pi_unlinked` and `pi_already_done`; the focused test failed with both missing IDs and the 2-vs-4 count. Restore the helper exactly from BASE; SHA-256 matched `c30f71b65fb2c8af44ded2abcac00479ae8af299d0fc45f6cd0e271d686235d7`.
- [x] Run the focused tagged test again (PASS), `go test ./internal/modules/commercial/commercialplatform -count=1` (PASS, 72.497s), `go test -tags lago_integration ./internal/modules/commercial/commercialplatform -run '^$'` (PASS), `gofmt -w internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go` (PASS), and `git diff --check` (PASS). No external service calls.
- [x] Update both owned Issue ledgers with implementation commit `11abb192a36eef9526ad3134d29385163fc310f6`, verification commands/outcomes, mutation-control failure, retained L1 disposition, and unchanged live T9/AC4 gates.
- [x] Commit task-owned paths as `11abb192a36eef9526ad3134d29385163fc310f6` (`test(commercial): cover T9 pre-settle identity set`). Controller evidence/plan update: `921be32c944749bd5c94e3d87ad6906445905932`.

**Review and OCR:** Task reviewer and final whole-range reviewer both returned Spec compliance PASS and code quality PASS with no open findings after the ledger evidence fix. Backend validator independently reran the focused tagged test successfully. OCR whole range `d8d21cd967c94c09d32c34a2981904758f442330..921be32c944749bd5c94e3d87ad6906445905932` selected zero files and reported `Review skipped: no items were selected`; OCR coverage is unavailable, not passed. Live T9 and AC4 remain open.

**Verification:** The complete response slice drives both parsing and identity capture; all four required IDs are asserted; the negative-control mutant fails the regression; the restored helper hash equals BASE; focused/full package/compile-only tests, formatting, and diff check pass.

**Acceptance:** The regression directly covers the helper used at the live fixture call site and would detect accidental filtering of historical or unlinked response identities. Production behavior is unchanged, and no live T9/AC4 completion is claimed.

**Failure handling:** If the negative-control test remains green, the assertion does not cover the finding; revise the test before proceeding. If restoring the helper does not reproduce the BASE hash, stop and restore from the exact BASE before any commit.

## Self-review

- **Spec coverage:** Covers the one retained Low test-coverage item; no broader runtime guarantee is claimed.
- **Step scan:** Each step has a specific test or hash outcome; no behavior change is introduced.
- **Type consistency:** Existing helper signatures are unchanged and named exactly as in the candidate checkpoint.
- **Review Focus:** Each line maps to the four-ID assertion, same-slice construction, negative control, non-live test command, or residual status update.
- **Proportion:** One regression and two ledger entries are bounded to one already-reviewed fixture finding.

## Status

- Task 1: verified at implementation commit `11abb192a36eef9526ad3134d29385163fc310f6`; evidence and plan checkpoint `921be32c944749bd5c94e3d87ad6906445905932`. Final SDD reviews pass; OCR has no file coverage. Live T9 and AC4 acceptance remain incomplete.
