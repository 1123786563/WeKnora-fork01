# T07 Backend Review Round 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining #147 backend privacy and availability findings by recovering returned file references, excluding identity-document fields from model payloads, and decoupling old-file cleanup from unrelated requests.

**Architecture:** Treat the existing `resources` catalog registration as the durable locator for every successful `SaveBytes` return. A Career-only scoped lookup ties candidate refs to server source ID, Tenant, digest and size; DB-uncertain outcomes preserve bytes for source-ID reconciliation, and cleanup never deletes a ref that a source may own. Model input uses explicit safe field taxonomy and separator-aware redaction. Opportunistic cleanup retains pending refs without failing source listing or unrelated uploads.

**Tech Stack:** Go, Gin, GORM, SQLite/PostgreSQL, ResourceCatalog/FileService, T03 Career source and receipt model.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; `docs/plans/issue-140/issues/issue-147.md`; root-cause record `docs/plans/issue-140/reviews/task-7-backend-review-r2.md`. Isolated code BASE `5d1b7b2f1a4d39852ad45e4455c3560ca744620a` in `/Users/wuyongjun/.codex/worktrees/issue-140-t07-intake/WeKnora-fork01` (report-only HEAD `8fdc30f2e`).

## Global Constraints

- Only confirmed facts may enter later model inputs; unrelated identity-document data must not enter final serialized JSON even when stored under an education or experience key.
- Every resource recovery query must be scoped to authenticated Tenant and server-issued Career source ID, with digest/size/name verification; no client-provided source name can widen it.
- If ownership or a DB write outcome is uncertain, retain the registered resource for reconciliation; do not release possibly referenced bytes.
- Background cleanup failure must remain visible as a pending state but cannot block unrelated owner source reads or fresh uploads.
- Local commits are authorized; no push, merge, deployment, Issue comment or closure. Web/TS files remain outside this backend repair.

## Review Focus

- Claim loss after a returned SaveBytes ref plus Release/Delete failure leaves a discoverable catalog candidate, and exact retry adopts/reconciles it without a second save.
- A DB timeout after ref persistence cannot cause deletion of bytes that the source now references.
- Source recovery never adopts or deletes another Tenant's or mismatched digest/name/size resource, nor deletes a ready source's active ref during a cleanup race.
- One cleanup-pending source does not turn GET sources or a separate new upload into HTTP 500.
- `education.passport_number=AB-12345678` and adjacent/space-separated ID formats are absent from final model JSON while ordinary education/experience facts remain.

---

### Task 1: Narrow model-field selection and nonblocking cleanup responses

**Depends:** code BASE. **Owner/validator:** upgraded backend implementer / backend_validator. **Files:** `internal/modules/career/model_input.go`, focused tests, `handler.go`, cleanup/list/upload tests. **Consumes:** T03 confirmed facts and T07 source state. **Produces:** purpose-specific safe model DTO and opportunistic cleanup semantics.

- [ ] **Step 1 RED:** Confirm an `education.passport_number` fact with `AB-12345678`, plus identity values with hyphen, space and adjacent letters. Serialize final model input; assert none of those document numbers or document fields appear, while a normal education and quantified achievement remain. Run focused test and observe failure.
- [ ] **Step 2 GREEN:** Select only known safe manual keys and server-generated per-item taxonomy keys for each purpose, explicitly reject identity-document field names under any category, then sanitize remaining values with separator-aware patterns. Do not rely solely on a broad category prefix or a contiguous-digit regex.
- [ ] **Step 3 RED/GREEN:** Inject persistent Release/Delete failure for one old source; `GET /career/sources` still returns scoped source history including cleanup-pending status and an unrelated new upload can progress. Retain ref, record/log cleanup failure, retry on a later bounded reconciliation without exposing raw paths. Do not hide a failure for the upload's own uncertain DB operation.
- [ ] **Step 4 VERIFY:** `go test -count=1 ./internal/modules/career/...` and `git diff --check`; failure blocks Task 2.

### Task 2: Reconcile all catalog-registered Career upload references

**Depends:** Task 1. **Owner/validator:** upgraded backend implementer / backend_validator. **Files:** `internal/modules/career/profile_intake.go`, `upload.go`, `handler.go` and tests; narrow read-only Career resource lookup helper in `internal/modules/career/` or repository layer, with focused SQLite tests. Touch generic ResourceCatalog only if a narrow optional interface is necessary, without changing the public FileService contract. **Consumes:** server source ID and digest, `resources` catalog rows created by SaveBytes, source claim token and resource bindings. **Produces:** scoped discovery/adoption/cleanup for refs returned but not persisted in Career source row.

- [ ] **Step 1 ROOT CAUSE/RED:** Build deterministic tests at the FileService→Career DB boundary: SaveBytes registers/returns ref, claim takeover makes Persist CAS fail, Release/Delete fails; assert ref remains discoverable by `(tenant, sourceID-derived original_name, digest, size)` and no raw object is lost from recovery. Simulate persistence-update commit with lost response plus failing follow-up read; assert zero deletion until authoritative reconciliation. Record observed RED results.
- [ ] **Step 2 GREEN locator:** Add a Career-private lookup of active `resources` rows by exact Tenant, server source ID embedded in original name, digest and size. Return only catalog refs (never physical paths in API/logs). Verify source row and resource bindings before any adoption or release. For a processing source with empty ref, claim-token CAS-adopt one matching candidate; for ready source, preserve its active ref. Keep extra candidates discoverable until guarded cleanup succeeds. A DB-unavailable or ambiguous state returns typed unknown and performs no destructive release.
- [ ] **Step 3 GREEN cleanup:** Once a ref is returned, bind it to the server source ID or otherwise give it a durable catalog locator before trying the Career source-ref CAS. If CAS loses, do not drop a failed Release; leave the registered candidate for source-ID reconciliation. Before deleting a candidate, establish under a guarded claim/transaction that no current source or other owner binding can adopt/use it; recheck that condition at the deletion boundary. On failed Release/Delete, leave the candidate active and retryable. Do not release merely because a prior read showed an empty ref.
- [ ] **Step 4 RACE/ISOLATION tests:** Cover old worker/new worker takeover, ready source duplicate candidates, cleanup versus concurrent adoption, other Tenant, wrong digest/name/size, DB read timeout, and failed physical deletion followed by successful later cleanup. Run SQLite migration/reopen tests if any schema changes are introduced. Preserve the already reviewed generic physical-write-before-catalog-registration limitation explicitly; this Task covers every ref that has a catalog registration/returned handle.
- [ ] **Step 5 VERIFY:** `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...`; `git diff --check`. Commit only assigned backend/migration files; append RED/GREEN commands, source SHA, exact residual and test coverage to task report. Any unresolved high privacy/ownership path keeps #147 unintegrated.

## Shared-file and interface preflight

Tasks 1 and 2 both edit Career `handler.go` and must run serially in the same isolated worktree. T03 `source.kind='user'`, upload 201/200/202/409 semantics and existing claim-token fencing remain intact. No other agent writes this worktree. The newly queried `resources` fields are private implementation details; no raw resource locator enters normal `CareerSource` or model DTO. The integration branch receives no T07 code until an independent validator and reviewer inspect exact `5d1b7b2..fix_HEAD`, both Spec and quality conclusions pass, and complete evidence is persisted.
