# T65 Task 3 Review Fix R1 — Independent Review

**Reviewed range:** `0b0057f78dffa4b2db62f0f4cc9de99f27b010a5..8ddee80892c278b85c4ff5d5e6de5c26c640734b` in the Task 3 metrics worktree. The worktree was clean at review. The range changes only `internal/application/repository/marketplace_metrics.go`, its test, and `internal/application/service/marketplace_metrics_test.go`, as assigned.

**Fact sources:** approved `docs/specs/2026-09-20-agent-marketplace-domain-model.md` §12; `CONTEXT.md` Marketplace Metrics; ADR 0011; Issue #65 snapshot; `plan-t65.md` Task 3; R1 fix plan and Task 1 brief. This is a read-only source review. I did not run OCR or repeat the implementer's tests.

## Original findings

| Finding | Disposition | Evidence |
| --- | --- | --- |
| T3-R1-1 (Medium), inconsistent snapshots | **Resolved** | `AggregateForRelease` now executes one parameterized `SELECT` with four scalar `COUNT(DISTINCT tenant_id)` subqueries. Each measure retains its own grain and the local-release join on both tenant and local release ID. The test logger asserts one executed statement. One SQL statement supplies a coherent database snapshot for all four counts. |
| T3-R1-2 (Low), ineffective time-boundary fixtures | **Resolved** | Both excluded boundary proposals now have matching `TenantIntroducedReleaseEntity` rows for their own tenants/local IDs. The repository test expects 22 introductions and only 10 in-window proposal tenants, so changing the half-open date predicate to include either boundary would fail. |
| T3-R1-3 (Low), missing duplicates and real privacy path | **Resolved** | Tenant 1 has a second active Adoption and accepted Proposal, while expected adopter/proposal counts remain 9/10/10. The service JSON test uses SQLite, the real repository and service, seeds private introduction/proposal fields, asserts the exact five-field suppressed response and checks marker absence. |

## New findings

None at critical, high, medium or low severity in the reviewed delta.

## Verdict

**Spec compliance: PASS for this R1 scope.** The query preserves lifetime introductions, current active Adoptions, and the fixed UTC half-open proposal window. The service still returns only coarse buckets and `not_collected`, with no caller-selected dimensions or raw records. The approved Spec's broader error-category metric remains an explicitly recorded T65 gap; this R1 does not claim to complete Issue #65.

**Code quality: PASS.** The single statement removes the identified cross-measure race; parameter binding avoids interpolated input; the new fixtures exercise real joins and distinct-tenant behavior. The implementer's report records passing focused repository/service tests, package tests, `git diff --check`, and `go build ./...` at this HEAD. Those commands were not rerun during this independent review.
