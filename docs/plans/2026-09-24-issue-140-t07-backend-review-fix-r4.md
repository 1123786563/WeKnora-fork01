# T07 Backend Review Round 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve the four current Web profile facts in safe model input and close the resource bind versus physical-delete race in Career surplus cleanup.

**Architecture:** Map existing T03 Chinese keys into the purpose-specific model whitelist without permitting identity-document fields. Add a narrow guarded-delete operation at the shared ResourceCatalog/file decorator boundary: a repository transaction serializes binding with an active-to-deleting claim only when no bindings exist, then the decorator deletes physical bytes and marks the resource deleted. A failed physical deletion retains a retryable deleting record and never reopens binding.

**Tech Stack:** Go, Gin, GORM, SQLite/PostgreSQL, ResourceCatalog/FileService, Career model input.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; `docs/plans/issue-140/issues/issue-147.md`; `docs/plans/issue-140/reviews/task-7-backend-review-r3.md`. Isolated worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t07-intake/WeKnora-fork01`, code BASE `53b30d2472fa80c6d7598ce2812f54791fd43511` (report-only HEAD `b5644f3f3`).

## Global Constraints

- T03 Web's confirmed keys `毕业时间`, `学历`, `城市`, `意向` must remain consumable by relevant evaluation/material purposes; identity-document keys and values remain excluded/redacted.
- Tenant/owner source scope and current upload replay, claim-token fencing, receipt/confirmation behavior remain unchanged.
- A resource with any live binding cannot be physically deleted. Once deletion is claimed, a new binding must fail; failed physical deletion remains retryable without exposing paths to Career or clients.
- Use local commits only; no push, merge, deployment, Issue comment or closure. No Web/TS edits in this round.

## Review Focus

- Final model JSON includes each of the four current Web facts when relevant and excludes a confirmed `education.passport_number` with `AB-12345678`.
- Concurrent Bind before deletion claim retains bytes; deletion claim before Bind rejects Bind and then safely deletes bytes.
- Two cleaners cannot independently delete a live resource; physical Delete failure retains deleting state and a later retry succeeds.
- Tenant mismatch, ready Career source ref, or foreign owner binding cannot enter guarded deletion.
- Existing resource registration, binding, release and download flows still pass their focused tests.

---

### Task 1: Restore current Web fact-key compatibility

**Depends:** code BASE. **Owner/validator:** upgraded backend implementer / backend_validator. **Files:** `internal/modules/career/model_input.go` and focused tests. **Consumes:** T03 Career confirmed facts with Chinese Web keys; T07 generated taxonomy keys. **Produces:** purpose-scoped safe model DTO preserving both sources.

- [ ] **Step 1 RED:** Build a confirmed profile using actual Web keys `毕业时间`, `学历`, `城市`, `意向` plus an `education.passport_number` fact. Assert final serialized JSON for each relevant purpose keeps the four Web facts and excludes the document field/value. Run focused test and observe current omission.
- [ ] **Step 2 GREEN:** Add explicit mappings for those four existing keys to purpose categories; retain the narrow generated-key pattern and identity-document exclusions. Do not reopen broad arbitrary category prefixes.
- [ ] **Step 3 VERIFY:** `go test -count=1 ./internal/modules/career/...`; `git diff --check`.

### Task 2: Guard resource deletion against concurrent binding

**Depends:** Task 1. **Owner/validator:** upgraded backend implementer / backend_validator. **Files:** `internal/modules/career/resource_recovery.go` and tests; narrowly `internal/application/repository/resource.go`, `internal/application/service/resource.go`, `internal/application/service/file/resource_catalog.go`, `internal/types/interfaces/resource.go`, resource types/state and focused tests. SQL resource state column already accepts string values; add a migration only if a checked schema actually requires it. **Consumes:** active catalog ref, bindings and FileService physical locator, Career scoped candidate. **Produces:** optional `DeleteUnbound` or equivalent guarded operation, used by Career surplus cleanup, without changing ordinary FileService methods.

- [ ] **Step 1 ROOT CAUSE/RED:** Use a deterministic barrier test to interleave Career surplus cleanup with a new foreign Bind after old unbind/count. Observe the old path deleting bytes after the new binding. Add tests for two cleaners and physical Delete failure/retry. Run them and record RED.
- [ ] **Step 2 GREEN repository:** Add a resource state `deleting` and a transactionally guarded claim: verify expected Tenant, lock/serialize resource row, require active and zero bindings, then atomically set deleting and return the internal physical locator. Make `CreateBinding` share that serialization boundary and require active state; SQLite must reserve the write before checking bindings, PostgreSQL uses row lock. Bind after deleting/deleted returns a typed unavailable error. Existing binding wins => claim reports in-use without deleting. Do not widen public resource access.
- [ ] **Step 3 GREEN decorator:** Add a narrow guarded physical delete method to the catalog-backed file decorator (or equivalent optional interface), carrying the private locator from the claim. On physical Delete failure, retain deleting state/path for later retry. On success, mark deleted conditionally on deleting state. A second cleaner safely observes/retries an existing deleting record; it cannot bypass the binding guard. Do not use normal `Resolve` after state changes because it exposes active resources only.
- [ ] **Step 4 CAREER:** Replace surplus-candidate Release-count-DeleteFile with the guarded deletion. Before the claim, revalidate Career source state/ref, Tenant and candidate identity. A resource already used by a ready source or any foreign binding stays intact; cleanup may remain pending without making source listing/new upload fail. Do not change normal T03/T04 resource paths to use this new operation without explicit tests.
- [ ] **Step 5 VERIFY:** Run focused repository/catalog/Career concurrency tests, `go test -count=1 ./internal/modules/career/... ./internal/application/repository/... ./internal/application/service/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...`, and `git diff --check`. Commit assigned files only; append exact code SHA, RED/GREEN command output, any residual and interface behavior to T07 report. No integration until independent reviewer gives Spec and quality PASS and validator binds evidence to the same SHA.

## Shared-file and interface preflight

Task 1 is Career model input; Task 2 crosses Career cleanup and shared ResourceCatalog, so execute serially in this isolated worktree. No concurrent worker edits resource files. Keep `ResourceCatalog.Release` semantics for existing callers unless Task 2's tests require an invariant fix; the new guarded path must not silently alter T04 artifact download or T03 profile behavior. The generic physical-write-before-catalog-registration crash window remains separately documented. Full `go test ./...` currently fails architectureguard manifest/route baseline checks recorded in the R3 Review; that gate has its own ownership and must be resolved before final #140 verification, rather than hidden by this resource fix.
