# T07 Backend Review Round 5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Career source cleanup idempotently complete when the physical resource was already deleted but a previous source-row clear failed.

**Architecture:** Keep the guarded resource delete claim and all binding exclusion behavior. Let its narrow tenant-scoped outcome distinguish `already deleted` from unknown/not found, so the Career adapter can treat a confirmed deleted terminal state as success and clear the stale source ref on a later reconciliation. Do not treat an arbitrary unknown ref as deleted.

**Tech Stack:** Go, GORM, SQLite ResourceCatalog/file decorator and Career cleanup.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; `docs/plans/issue-140/issues/issue-147.md`; `docs/plans/issue-140/reviews/task-7-backend-review-r4.md`. Isolated worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t07-intake/WeKnora-fork01`, code BASE `8efd70dc0e188159eb2fa80592ad93e997cd30eb` (report-only HEAD `5c36749a3`). This is fix round 5/5.

## Global Constraints

- Only a verified catalog row for the expected Tenant and exact resource handle in terminal `deleted` state counts as already cleaned; foreign Tenant, never-registered and still-active refs must not be treated as success.
- No change to current Web fact keys, Career upload/receipt/provenance, guarded Bind/delete exclusion or normal Artifact/Task download behavior.
- A failed physical deletion retains deleting state for retry. A successful deletion cannot leave a Career source permanently cleanup-pending because a later source-row update failed.
- Local commit authorized; no push, merge, deployment, Issue comment or closure.

## Review Focus

- Real guarded physical deletion succeeds; injected Career `ClearSourceResource` failure leaves pending state; next scoped reconcile sees catalog deleted and clears the Career ref.
- A mismatched Tenant, unknown handle or active resource never passes through the already-deleted branch.
- Retry after physical Delete failure still performs deletion rather than falsely reporting already deleted.
- Foreign binding protection, two-cleaner exclusion and ordinary file access remain intact.
- Targeted tests and independent final Spec/quality Review pass on the exact same SHA.

---

### Task 1: Idempotent terminal resource cleanup

**Depends:** reviewed R4 checkpoint. **Owner/validator:** upgraded backend implementer / backend_validator. **Files:** narrowly `internal/application/repository/resource.go`, `internal/application/service/resource.go`, `internal/application/service/file/resource_catalog.go`, `internal/modules/career/upload.go`, `handler.go` if strictly needed, and focused real SQLite tests beside catalog/Career. **Consumes:** guarded DeleteUnbound and Career cleanup-pending source ref. **Produces:** scoped `already deleted` cleanup outcome that lets later Career reconciliation finish.

- [ ] **Step 1 RED:** Use the real catalog-backed FileService and Career source DB. Delete a pending source's physical resource via guarded operation, fail only `ClearSourceResource` once, then retry source reconciliation. Assert current code remains stuck, and that wrong-Tenant or unknown handle does not get a success outcome.
- [ ] **Step 2 GREEN:** Add a narrow tenant+handle state lookup or guarded result variant that can confirm a catalog row is `deleted`, including after physical deletion. Make `DeleteUnbound`/Career adapter treat that exact terminal state as idempotent success while keeping `(false,nil)` or error for active/in-use/unknown/foreign refs. Then the existing conditional Career clear can finish on retry. Do not expose physical paths or allow a caller to claim someone else's deletion.
- [ ] **Step 3 REGRESSION:** Re-run physical deletion failure/retry, Bind-before-claim, claim-before-Bind, two cleaners, ready source and foreign Tenant tests. Test unknown DB result remains non-destructive. Use exact assertions on final catalog state, physical file presence/absence, source status and ref.
- [ ] **Step 4 VERIFY:** `go test -race -count=1 ./internal/application/service/file -run 'TestGuardedDelete'`; `go test -count=1 ./internal/modules/career/... ./internal/application/repository/... ./internal/application/service/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...`; `git diff --check`. Commit assigned files only and append RED/GREEN evidence, exact code SHA and residual to the T07 task report. Independent reviewer must give both Spec and quality PASS, and validator must bind checks to this SHA; any valid medium/high finding leaves T07 unintegrated and must be reported, not silently waived.

## Shared-file and interface preflight

Only this upgraded worker edits the isolated T07 worktree. The change is deliberately narrow; it does not own architectureguard manifests (separate integration plan) or Web/TS. The generic FileService pre-catalog-registration crash window is still an infrastructure residual. The independent reviewer will compare `8efd70d..fix_HEAD` and also consider the accumulated T07 backend implementation before approving integration.
