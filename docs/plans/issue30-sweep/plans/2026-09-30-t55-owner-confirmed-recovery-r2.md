# T55 Owner-Confirmed Recovery R2 — Complete Dispatched Settlement

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Complete the approved owner-confirmed crash recovery for dispatched deliveries across both A03 action states.

**Spec and context:** T55 owner-confirmation plan `2026-09-30-t55-owner-confirmed-recovery.md`, Issue #55 snapshot, approved mobile AI office spec, ADR-0008, and `CONTEXT.md`.

## Global Constraints

- No automatic retry after empty remote lookup. Only explicit owner confirmation can settle an ambiguous dispatched claim.
- Matching PR/MR settles delivered without confirmation; provider lookup errors remain errors.
- Resolve performs no create POST. A later dispatch remains separate.
- Preserve run and owner authorization before provider access and keep A03 Action and Delivery states consistent.
- Local commit only; no push, merge, deploy, publication, or Issue mutation.

## Review Focus

- Confirmation reaches both ActionUnknown and ActionSucceeded paths.
- Confirmed `dispatched` plus no PR and existing branch transitions to `pushed`; the CAS allows only the observed valid source states.
- Receipt and state are consistent; no silent partial settlement on a rejected CAS.
- JSON `null` is rejected as a non-object body; empty body and valid object remain supported.
- Add behavior tests for both A03 states and strict body shape.

## Task DAG

Single Task 1, no dependencies beyond R1 commit `30adf8b1afcd12fb0c175bbf16267bc6432a252d` and independent review findings R1-F1/F2/F3.

### Task 1 — Complete confirmed dispatched recovery

**Owner role:** `backend_implementer`. **Validator:** `backend_validator`. **Independent reviewer:** `reviewer`.

**Owned files:** `internal/modules/codedelivery/service.go`, `internal/modules/codedelivery/dispatcher.go`, `internal/modules/codedelivery/service_dispatch_test.go`, `internal/modules/codedelivery/service_gitlab_test.go`, `internal/handler/session/workbench_delivery.go`, `internal/handler/session/workbench_delivery_test.go`, `internal/modules/codedelivery/repository/codedelivery/store.go`, and `internal/modules/codedelivery/repository/codedelivery/store_test.go` (added after inspection proved separate receipt write followed by state CAS permits partial settlement; transaction seam plus regression test atomically verifies the row update).

**Consumes:** Explicit confirmation contract `{confirm_no_matching_pr: true}` and R1 attested A03 resolver seam.

**Produces:** owner-confirmed dispatched deliveries settle to pushed through both action states; strict request-body shape.

- [ ] Add tests proving action-succeeded dispatched recovery fails without confirmation and succeeds with confirmation, and action-unknown dispatched recovery settles Action and Delivery consistently.
- [ ] Change the succeeded-action path to pass `ConfirmNoMatchingPR`; allow the confirmed observed dispatched state in the transition CAS.
- [ ] Ensure receipt updates do not remain partially applied if state settlement fails; use an atomic store operation if required by repository seams.
- [ ] Store transaction test verifies rejected source-state CAS leaves both state and receipt unchanged; successful transition commits both together.
- [ ] Reject top-level JSON `null`; retain empty body, valid true/false object behavior and unknown-field rejection.
- [ ] Run focused regressions, `go test ./internal/modules/codedelivery/ -count=1`, `go test ./internal/handler/session/ -run TestResolveDeliveryRequiresStrictOwnerConfirmationBody -count=1`, appconnector tests, targeted recovery HTTP test, and `git diff --check`.
- [ ] Commit owned files and report exact BASE/HEAD, payload SHA, checks and residual delayed-provider risk.

**Failure handling:** If receipt/state atomicity cannot be achieved with the current store API, do not report success; identify the smallest repository seam and include only that owned file after updating this plan/brief and ledger.

## Plan self-review

- All three independent review findings are mapped to behavior assertions or strict input validation.
- No mobile work is included; the consumer contract is unchanged.
- Test evidence must be bound to the committed snapshot before independent review.
