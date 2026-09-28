# T55 Task8 Review Fix — Truthful recovery evidence and executable behavior tests

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Narrow review-fix wave for Task8 checkpoint `3cf6b7be01d1f9d44a6b6889c79ed1dd2ab98dde`.

**Goal:** Report recovery only when the smoke segment actually examined and acted on a recoverable delivery, and behavior-test every recovery evidence outcome without contacting a live deployment.

**Findings:** R1 MEDIUM: run loop stops at the first non-null delivery, while `createDeliveryRecovery` returns an already-delivered record without writes; smoke unconditionally labels any resolved call `recovered`, and may skip later pushed/unknown records. R2 MEDIUM: tests only parse config and never invoke the recovery evidence segment or assert recovered/not-needed/failed outcomes. R3 LOW: function comment says the complete flow never dispatches, conflicting with new opt-in dispatch behavior.

## Global Constraints

- Existing isolated T55 Task8 worktree only, based on `3cf6b7be01d1f9d44a6b6889c79ed1dd2ab98dde`.
- Owned files only: `apps/mobile/src/delivery-integration-smoke.ts`, `apps/mobile/src/delivery-integration-smoke.test.ts`.
- No API-client/mobile-core production changes, no other test files, no live URL, credentials, network service or deployment.
- Preserve default-off behavior, authorized Runtime transport, current Scope Lease and failure classification.
- Local commit authorized. Independent reviewer and frontend validator follow. No further delegation by implementer.

## Review Focus

1. Already-delivered and other ineligible rows are `not-needed`, never `recovered`; scanning continues to later task rows until one pushed/unknown candidate is handled or all rows are exhausted.
2. `recovered` requires the recovery operation on an eligible pushed/unknown candidate to complete successfully; evidence identifies the attempted Run and Delivery.
3. Conflict/invalid-input race becomes not-needed; backend/scope failures remain failed with truthful error evidence.
4. Tests execute the production recovery-segment orchestration with deterministic fake reader/recovery ports, assert call order/count and evidence fields, and do not need secrets/network. The top-level smoke must visibly invoke that same segment only when opt-in is enabled.
5. Flow documentation says default path is read-only and opt-in path may resolve/dispatch recovery.

## Task 1 — Extract and behavior-test the recovery evidence segment

**Dependencies:** Task8 checkpoint `3cf6b7be01d1f9d44a6b6889c79ed1dd2ab98dde`; R1/R2 MEDIUM and R3 LOW.

**Role:** `frontend_implementer`; independent reviewer + frontend validator. **Worktree:** existing isolated `codex/issue30-t55-t8`. **Owned files:** only the two Task8 files above.

1. **RED:** Add behavior tests for: first delivery already delivered then a later pushed/unknown row is recovered; all rows delivered/ineligible returns not-needed and calls recovery zero times; state conflict/invalid input records not-needed and may continue scanning; backend failure records failed and stops; recovered/not-needed/failed evidence includes the correct runId/deliveryId/recoveryState. Test the production segment function using deterministic fake ports, no live service. Show at least the current false-recovered/first-row behavior fails.
2. **Implement:** Extract an exported or module-visible `runDeliveryRecoveryEvidence` orchestration seam accepting delivery candidates/reader and recovery port; `runDeliveryIntegration` must call this exact seam only when `config.recover` is true. Skip delivered and other ineligible states as not-needed candidates, continue scanning, and label recovered only after a pushed/unknown operation was actually attempted and completed. Preserve first eligible candidate policy: a backend/scope failure is recorded and stops to avoid repeated ambiguous writes; state conflict/invalid input may continue because it proves no recovery write completed. Add `recoveryRunId` and `recoveryDeliveryId` (or equivalent explicit fields) to evidence for the actual attempted candidate; when no attempt occurred record the candidate inspected as not-needed if useful, and never imply a write. Keep non-opt-in recovery skipped.
3. **Correct comments:** Describe default mode as read-only; opted-in mode may issue recovery requests (resolve/dispatch) and is intended only for an authorized test deployment.
4. **GREEN:** Run focused smoke tests, mobile typecheck, full mobile tests if bounded, and `git diff --check`. Do not set real deployment env vars or contact any service.
5. **Commit/review:** Commit only the two owned files; report SHA, RED/GREEN evidence, behavior matrix, and live-deployment limitation.

**Acceptance:** No already-delivered row is mislabeled recovered; later eligible rows are inspected; all outcome branches are behavior-tested through the same orchestration invoked by the opt-in top-level smoke; no credentials or network were used.

**Failure handling:** If a fake-port seam cannot be made to match production orchestration, retain a failing behavior test and report the missing test seam; do not replace it with source-text assertions or claim configuration parsing verifies the behavior.

## Interface and Scope Preflight

- The seam is local to the smoke module and does not alter mobile-core/API-client interfaces.
- T55 Task8 remains unverified until this checkpoint passes independent review and validation; downstream final B6 integration stays blocked.
