# T07 Backend Review Round 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close five high findings from T07 backend fix round 1 without changing the reviewed source-backed proposal model.

**Architecture:** Keep the current T03 Web `user` source value as a wire alias, resolve upload replay from the stored request before defaulting revision, and use the same strong redaction on every model string. Rotate a private database claim token at lease takeover; fence source/resource/batch mutations with it, preserve known file refs until successful cleanup, and reconcile at actual expiry.

**Tech Stack:** Go, Gin, GORM, SQLite/PostgreSQL migrations, FileService/ResourceCatalog.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; `docs/plans/issue-140/issues/issue-147.md`; `docs/plans/issue-140/reviews/task-7-backend-review-r1.md`. Isolated worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t07-intake/WeKnora-fork01`, fix BASE `074021e5676d0c2b5f3045ed6b991a82f16ed312` (report-only commit after it does not change code).

## Global Constraints

- T03 Web at `apps/web/src/career/CareerPage.tsx` currently sends `source.kind='user'` for propose, confirm, confirm_proposal and dismiss; all four must continue working until a coordinated client migration.
- System-extracted facts remain pending until explicit user confirmation; tenant and owner scope is rechecked on every request and retry.
- A known private resource ref cannot be dropped before actual file/catalog cleanup succeeds. Unknown database outcome cannot trigger destructive cleanup.
- No new raw resume retention duration is set. Local commits are authorized; no push, merge, deploy, Issue comment or closure.

## Review Focus

- Current Web payloads continue to mutate the profile while a forged resume source is still rejected.
- Exact upload retry without `expectedRevision` returns original receipt even after revision advances; changed bytes/intent conflicts before storage.
- A lease takeover cannot let the former worker write a ref, complete a batch, or fail/release the current worker's source.
- Expired cleanup runs at the actual lease deadline and cannot race a renewal or erase a ref when deletion fails.
- An identity number adjoining ASCII letters never appears in final serialized model input.

---

### Task 1: Preserve the public wire contract and complete privacy/replay fixes

**Depends:** reviewed fix checkpoint `074021e`. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/handler.go`, `handler_test.go`, `model_input.go`, model-input tests and focused upload tests. **Consumes:** T03 Web `CareerAction` source wire and owner-scoped upload claim. **Produces:** compatible server-normalized provenance, replay-safe omitted-revision semantics, uniformly redacted model DTO.

- [ ] **Step 1 RED:** At the handler boundary send the actual T03 Web `source.kind='user'` payload for each of four actions; assert same responses/authority as before T07. Also send forged `resume_extraction` with direct confirm; assert denial. Run focused tests, observe current Web payload failure.
- [ ] **Step 2 GREEN:** Accept the existing `user` alias only as user/manual intent and normalize it server-side; never treat it as proof of extracted provenance. Preserve current UI behavior and forbidden-source denial.
- [ ] **Step 3 RED/GREEN:** Add HTTP test for exact multipart request ID + bytes with **no** `expectedRevision`, after first intake advances profile revision. Resolve existing claim/intent before defaulting the revision, compare against stored expected revision, and return same source/receipt. Changed content/filename/MIME/explicit revision under the ID remains 409 before any new file save.
- [ ] **Step 4 RED/GREEN:** Put an identity-like value such as `110105199001011234A` and an adjacent-letter variant in confirmed fact values/keys, serialize the final model DTO, assert neither ID survives. Apply the strong in-token redactor to all serialized strings, not just keys.
- [ ] **Step 5 VERIFY:** Run focused Career/handler tests and `git diff --check`; failures block Task 2.

### Task 2: Fence lease ownership and retain cleanup references

**Depends:** Task 1. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/profile_intake.go`, `upload.go`, `handler.go` and focused tests; `migrations/sqlite/000114_career_source_revisions.{up,down}.sql`, `migrations/versioned/000193_career_source_revisions.{up,down}.sql`, `internal/database/career_migration_test.go`. **Consumes:** scoped request claim, stable source ID, catalog-bound private resource. **Produces:** private `claim_token`/lease epoch, fenced source transition and retriable cleanup.

- [ ] **Step 1 RED:** Add deterministic race tests: old token cannot persist ref, complete proposal batch, or mark failed after takeover; exactly one receipt and no replacement of the original resource ref. Test active duplicate returns 202; expiry at `lease_until <= now` is reconciled immediately, without a 30-minute extra delay.
- [ ] **Step 2 GREEN:** Add private `claim_token` column and `UploadClaim{SourceID,Token,ExistingRef}`. Rotate token with conditional UPDATE scoped by tenant/user/source/status/old token/expired lease and check one affected row. Fencing must apply to `PersistUploadResource`, `CompleteIntake` (inside its transaction), `FinishSource`, and stale cleanup. A takeover with an existing ref reuses it after scoped digest verification; it never saves a second blob or overwrites the ref. An old worker that loses the claim re-reads source/receipt instead of mutating or deleting the new worker's data.
- [ ] **Step 3 RED/GREEN:** Inject catalog Release and file Delete failures. Once a ref is persisted, remove adapter-owned unconditional deferred cleanup. Terminal failure retains exact ref as cleanup-pending; perform detached bounded Release; clear ref only after success through conditional update matching current ref/state. If cleanup fails, keep ref and expose/retry it on reconciliation. A newly saved ref that loses Persist CAS may be released separately, but never release a previously persisted ref. Test stale cleaner racing token renewal and old worker parse failure after takeover.
- [ ] **Step 4 MIGRATION:** Update SQLite and PostgreSQL source migrations/model for claim token (private, not in wire JSON); ensure down migration restores pre-T07 schema. Run SQLite migration up/down/up and reopen. Run isolated PostgreSQL migration test if the existing harness is available; never modify a shared DB.
- [ ] **Step 5 VERIFY:** `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...`; `git diff --check`. Commit only assigned backend/migration files and append exact BASE/HEAD, RED/GREEN evidence, interface changes and remaining generic FileService pre-return crash window to the T07 report. Any unresolved high finding keeps T07 unintegrated.

## Shared-file and interface preflight

Tasks 1 and 2 both edit `handler.go`; execute serially in the same isolated T07 worktree. No Web/TS files or T08 migration numbers are owned by this round. The private claim token must never be emitted in `CareerSource`, receipt, log or model DTO. Current `requestId` requirement and 201/200/202/409 upload responses remain the frontend contract unless a reviewed change is explicitly recorded. Review Package is the actual `074021e..fix_HEAD`; reviewer and validator inspect exact code SHA. Independent Spec and quality review must pass before cherry-picking into the integration branch and dispatching Web.
