# T08 Opportunity error-classification review fix

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep definite pre-write database failures distinct from ambiguous Opportunity import commits.

**Architecture:** The previous review fix added bounded receipt reconciliation after transaction errors. Narrow its entry condition to errors after the first possible write/commit. Preserve the original error when the first receipt lookup fails before persistence is possible. Keep all scoped replay and timeout behavior for genuinely uncertain writes.

**Tech Stack:** Go, GORM, Career Office tests.

**Sources:** T08 backend fix commit `b1eba2892bcca03b0737258034dd492ade177630`, independent re-review finding at `internal/modules/career/opportunity.go:181-191,228-241`, prior plan `docs/plans/2026-09-24-issue-140-t08-backend-review-fixes.md`.

## Global Constraints

- Change only T08 backend Career files/tests in the existing isolated worktree. No migration, route, Web, or guard changes.
- Local commit authorized; no push/merge/deploy. Keep request-ID recovery for every genuinely ambiguous commit and concurrent same-ID race.

## Review Focus

- An injected failure of the first receipt SELECT surfaces the original database error, not `outcome_unknown`.
- Cancellation/lost acknowledgement after a possible write still reconciles a scoped same-intent receipt or returns typed `outcome_unknown`.
- Changed-intent conflict, replay, size cap, and tenant fencing remain intact.

---

### Task 1: Distinguish pre-write failure from uncertain persistence

**Depends:** reviewed T08 first fix. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/opportunity.go`, focused Career tests. **Consumes:** ImportJD transaction stages and error. **Produces:** reliable error classification.

- [ ] RED: Add a focused injected first-receipt-query failure test; assert a normal persistence error and no record. Retain the prior ambiguous-write tests as regression checks.
- [ ] GREEN: Mark whether the transaction reached a possible write/commit. Return definite pre-write errors unchanged; invoke bounded receipt reconciliation only when an insert or commit may have happened. Avoid classifying the first read's cancellation as a possible commit.
- [ ] VERIFY: Focused tests, broader Career/router/database/handler/container suites, `git diff --check`; save exact SHA and evidence. Independent reviewer provides Spec and quality verdicts on the increment.

## Shared-file preflight

No Web or guard agent is writing while this backend fix runs. The Opportunity API contract remains unchanged; only error classification narrows. Existing report and first review-fix commit remain intact for audit.
