# Issue 140 Task26 Review Repair R2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining high-risk Task21 upload recovery finding and directly test deterministic key selection at each configured cloud provider adapter.

**Architecture:** An upload whose physical write outcome is unknown and whose reference is absent remains unresolved across stale cleanup. The same idempotent request can reclaim the expired processing row and replay the write with the same stable Career object key; only a durable locator or verified physical deletion permits claim release. Provider tests exercise the actual SaveBytes key-selection branch for COS, TOS, OSS and OBS.

**Tech Stack:** Go, Career Office lifecycle claims, local/cloud FileService SDK adapters, SQLite/PostgreSQL migration history.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`, `CONTEXT.md`, ADR-0015 through ADR-0018; prior plans `docs/superpowers/plans/2026-09-30-issue140-career-review-repairs-r2.md` Task21 and `docs/superpowers/plans/2026-09-30-issue140-task21-review-repairs.md` Task26; review `/tmp/issue140-task26-review.md`.

## Global Constraints

- Keep all existing migration IDs unchanged: #30 SQLite112–123/versioned191–202; #140 SQLite124–144/versioned203–223. This repair adds no schema migration.
- Never infer “no physical write” from an object-storage error; the service may have accepted bytes before its response was lost.
- A `deleted` receipt requires all uncertain Career upload effects to be reconciled or physically removed.
- Retry identity remains request ID + intent hash + digest + expected revision; duplicate attempts use one stable tenant-scoped physical key.
- No DB transaction may span a network/object-storage operation.
- Local commits only; no push, shared merge, deploy, publication, or GitHub mutation.

## Review Focus

- SaveBytes writes the object then returns an error; a stale sweep runs; the identical request retries; object bytes are reused at the same key; deletion cannot pass until the known object is cataloged or removed.
- Stale cleanup does not convert an unresolved no-ref write into a terminal result that permits claim release.
- Replays and deletion remain serialized across Office instances by the execution guard.
- COS, TOS, OSS and OBS SaveBytes each choose the deterministic Career path through the actual adapter branch; ordinary and temporary writes retain their existing UUID behavior.

---

## Task 30: Retain unknown upload claims across stale cleanup and verify provider key wiring

**Dependency:** Task26 commit `79425fd1fc51f6510b21a7017502be16f285ce30`; finding F1 High in `/tmp/issue140-task26-review.md`.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Owned files:** `internal/career/{profile_intake.go,handler.go,upload_test.go,handler_test.go}` and focused `internal/application/service/file` provider tests or narrowly scoped test seams. No migration files.

**Consumes / produces:** `FailStaleUploads` processes lease-expired sources under the existing Office transaction. For a processing row with empty `ResourceRef`, the prior storage outcome is unknown; it must remain retryable/unresolved, not become terminal `failed` with a releasable lifecycle claim. `ClaimUpload` on the exact same request after lease expiry must reacquire ownership and replay `SaveBytes`; deterministic path `career_source_<sourceID><ext>` returns the same provider object key. If a ref is already durable, existing stale cleanup and compensation remain unchanged.

**Steps:**

- [ ] Add one full lifecycle RED test using a fake FileService that records bytes at a stable ref and returns `(empty, error)` on first save, then returns the ref on retry. Trigger a different-request stale sweep after lease expiry, retry the exact original request, and assert both attempts target the same physical ref, the claim stays unresolved until retry result is durable, and deletion remains busy before reconciliation.
- [ ] Add a second assertion that after successful same-request retry and durable ref/catalog binding, deletion either removes the object before its terminal receipt or remains busy if removal fails; it must never report deleted while the test object exists.
- [ ] Add direct adapter-level tests or narrow injected-client seams proving COS/TOS/OSS/OBS SaveBytes produce deterministic tenant/prefix Career keys across two retries. Also assert ordinary names and `temp=true` retain unique/temporary behavior. The test must exercise each adapter's selected key, not only the shared key formatter.
- [ ] Run all new tests RED against commit 79425 and save output in `/tmp/issue140-task26-review-repairs-r2-report.md`.
- [ ] Change stale processing finalization so an empty-ref unknown storage outcome retains an expired/retryable processing claim; ensure it is not returned as a releasable stale resource. Keep any known-ref stale cleanup behavior unchanged.
- [ ] Implement adapter-level key test seams without changing production storage contracts or unrelated object naming.
- [ ] Run the full stale/replay/deletion sequence, provider adapter tests, `go test -count=1 ./internal/career ./internal/application/service/file ./internal/workbench/service/workbench ./internal/container ./internal/database`, relevant race tests, migration uniqueness checks, and `git diff --check`. Record PostgreSQL/cloud service unavailability if applicable.
- [ ] Commit the repair and append exact commands/results to `/tmp/issue140-r2-task21-report.md`.

**Acceptance:** F1 is closed: after an accepted write with lost response, stale maintenance and same-request replay cannot remove the only deletion barrier; retry reconciles the identical physical key and a deleted receipt is impossible until the private object is referenced and removed. Each configured provider's SaveBytes adapter path has direct deterministic-key regression coverage.
