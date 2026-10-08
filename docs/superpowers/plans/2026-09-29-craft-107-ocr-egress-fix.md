# OCR Round 3 Egress Findings Fix Plan

**Goal:** Resolve the actionable OCR finding in torn-response journal error handling and remove the redundant 502 exclusion while preserving fail-closed retry semantics.

**Architecture:** Keep the journal as the identity source. Both complete and torn gateway responses log durable resolution failures with the attempt identity and observed outcome; neither branch may silently erase a persistence failure. Status-only torn-response classification remains conservative.

**Tech Stack:** Go, `net/http`, existing `internal/craftegress` tests.

**Spec:** Craft Web Artifact Spec #107 and the OCR round 3 partial report `docs/plans/craft-107-ocr-source-round3-partial.md`. This is an OCR-scoped repair to `craftegress`; it does not resolve the separately blocked F08/T04 build receipt seam.

## Global Constraints

- Preserve same-ID retry behavior for unresolved outcomes and new identity behavior only for proven definitive outcomes.
- Do not claim a torn response is definitive from body shape.
- Preserve the existing externally visible 502 response for incomplete gateway bodies.
- Touch only `adapter.go` and focused tests in the same package, plus the task report.
- No commit, stage, push, external messaging, or changes to unrelated shared-worktree edits.

## Review Focus

- A failure to persist `Resolve` in the torn-body branch is observable in logs with the opaque attempt ID, definitive decision, and gateway status.
- Successful `Resolve` behavior and retry identity remain unchanged.
- The status fallback excludes 409 and all 5xx; the explicit 502 exclusion is redundant because 502 is already 5xx.
- Tests exercise public adapter behavior and a failing journal seam rather than only source text.

## Task 1: Make torn-response resolution failures visible

**Depends on:** OCR round 3 finding `OCR-R3-EGRESS-01`; no implementation dependency.

**Owner role:** `backend_implementer`; **validator:** `backend_validator`; **reviewer:** parent-dispatched read-only `reviewer`.

**Files owned:**

- Modify: `internal/craftegress/adapter.go`
- Modify: focused tests in `internal/craftegress/adapter_test.go` or `ocr_regression_test.go`
- Create: `docs/plans/2026-09-29-craft-107-ocr-egress-fix-report.md`

**Consumes:** Partial OCR report and current adapter/journal contracts.

**Produces:** Logged torn-response journal resolution errors, focused behavioral regression, removal of redundant status clause/comment correction, exact command/hash report.

1. Add a test-first failing-journal case for an HTTP 200 response whose body read tears; assert the response remains HTTP 502 and the journal error is logged with attempt ID/status/definitive fields.
2. Change the torn-body branch to capture and log the `Resolve` error consistently with the full-body branch.
3. Remove the redundant 502 test from `responseBodyReadOutcomeIsDefinitive`; preserve explicit 409 exclusion and blanket 5xx exclusion. Adjust the helper comment to state that all 5xx remain unresolved.
4. Run focused tests, package tests, formatting, and `git diff --check`; record hashes and results.

**Verification:** `go test ./internal/craftegress -run 'TestAdapterTornBody|TestGatewayOutcome' -count=1`; `go test ./internal/craftegress -count=1`; `gofmt` and `git diff --check`.

**Acceptance:** Tests prove the logging behavior for journal failure and preserve all retry identity/status behavior; package tests pass; no unrelated diff is changed.

**Failure handling:** If the existing journal cannot be reliably induced to fail, add a narrow test seam without changing production contracts; report any environment failure and do not suppress the finding.
