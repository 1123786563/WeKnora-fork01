# T08 Opportunity backend review fixes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the two medium findings from the first independent T08/#146 backend review before integration.

**Architecture:** Keep the existing scoped request-ID receipt and fixed-snapshot persistence model. Bound HTTP JSON decoding before binding, and classify transaction outcomes that may have committed as recoverable `outcome_unknown` after bounded receipt reconciliation.

**Tech Stack:** Go, Gin, GORM, SQLite/PostgreSQL.

**Sources:** `docs/plans/issue-140/issues/issue-146.md`, `docs/plans/issue-140/task-8-backend-brief.md`, `docs/plans/issue-140/task-8-architecture.md`, first independent review of `11d90ce67..7a29b3336`.

## Global Constraints

- Preserve tenant/owner scope, exact request-intent replay, immutable raw JD/snapshot, and no automatic profile facts or external fetch.
- Local commits authorized; no push/merge/deploy. Implement only within the T08 backend worktree and owned Career files.
- Treat a definite validation error separately from an ambiguous database commit. Never claim failure if a same-intent receipt may have persisted.

## Review Focus

- A cancellation, SQLite busy path, or concurrent same-ID insert cannot silently become an unrecoverable 500 if persistence remains uncertain. Return the existing receipt when found; otherwise expose the request ID as `outcome_unknown` for recovery.
- An authenticated request larger than the intended import limit is rejected before JSON binder allocates the whole body.
- Replay, changed-intent conflict, tenant fencing, and normal import still pass.

---

### Task 1: Bound the import request body

**Depends:** initial T08 backend implementation. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/handler.go` and focused handler tests. **Consumes:** POST `/api/v1/career/opportunities/import`. **Produces:** bounded JSON decode and deterministic oversized-request response.

- [ ] RED: Add handler test with a body exceeding 1 MiB plus JSON overhead; assert bounded rejection and no import record.
- [ ] GREEN: Install `http.MaxBytesReader` or equivalent before JSON binding with a small documented envelope overhead. Preserve the service's exact raw-text byte limit and existing valid request behavior.
- [ ] VERIFY: Focused handler and Career tests; document exact response/status.

### Task 2: Recover ambiguous writes

**Depends:** initial T08 backend implementation; independent of Task 1 in behavior but same owner/worktree. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/opportunity.go`, related tests and handler error mapping only as needed. **Consumes:** scoped request ID and intent hash. **Produces:** receipt or `OutcomeUnknownError{RequestID}` after bounded reconciliation.

- [ ] RED: Add concurrent same-ID tests and injected cancellation/ambiguous commit tests that expose raw 500 or duplicate ambiguity. Include changed-intent conflict behavior.
- [ ] GREEN: Reconcile with bounded scoped receipt lookups using a usable context; return found same-intent receipt, conflict for different intent, or `OutcomeUnknownError` when commit cannot be ruled out. Keep finite retry/backoff and cancellation response bounded.
- [ ] VERIFY: Focused Career/router/handler/database/container suites, race-sensitive test if feasible, `git diff --check`; report exact SHA and limitations. Independent reviewer must re-evaluate both medium findings and absence of regressions.

## Shared-file preflight

Both changes are backend-only and run serially in the existing T08 isolated worktree. T08 Web work has not started. Architectureguard route-count update starts after these fixes are reviewed and integrated; it owns no Career file. The low observation-append test suggestion is deferred until an append API exists because current T08 only creates one initial observation per opportunity.
