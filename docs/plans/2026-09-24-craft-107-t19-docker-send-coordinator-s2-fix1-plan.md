# Craft #107 T19 Docker send coordinator S2 Fix 1 Plan

> **For Codex:** Execute through SDD RED → GREEN → REFACTOR, exact uncommitted checkpoint, independent review and backend validation. No chargeable provider call or production enablement.

**Goal:** Close S2 review High S2-R1: a coordinator operation may currently adopt an outstanding legacy intent whose callback can independently send and resolve the row. Obtain PostgreSQL evidence for S2-R2. S2-R3's physical-send callback proof belongs to the later S3 adapter seam and remains an explicit unmet acceptance until exercised.

**Sources:** Approved Craft Spec Task Budget/recovery; Docker recovery design; `2026-09-24-craft-107-t19-docker-send-coordinator-s2-task1-review.md`; S1 persistence review.

## Global constraints

- Preserve existing legacy calls while preventing them from preparing/resolving a Docker coordinator-owned operation. No legacy API can grant a second send or release a claimed unknown hold/fence.
- Protocol ownership must be durable before any inert create and unambiguous on replay. A server-derived reserved activity-key namespace with explicit legacy rejection is acceptable only if all current preparation and resolution paths enforce it and existing/predeployment rows cannot collide; otherwise use an additive durable marker with paired PG/SQLite migration and tests.
- `resolveCraftChargeStart` must refuse receipt-bound/send-claimed rows in its conditional transition. Claimed or ambiguous rows remain unresolved/held. No SQL transaction surrounds Docker I/O.
- No commits/pushes. Preserve concurrent T01/H2 edits and do not edit their files.

## Task 1 — Exclusive protocol ownership and cross-path guard

**Role:** backend_implementer. **Owned files:** `internal/application/service/craft_docker_send_coordinator.go/_test.go`, `craft_budget.go` and focused budget tests, `internal/application/repository/craft_docker_send_claim.go/_test.go`; migration files only if the reserved namespace cannot be proven unique and require an explicit report before widening. **Consumes:** approved grant/binding, Task/Run/activity key and existing journal/hold. **Produces:** coordinator-only durable preparation and claim, with legacy paths unable to send/resolve the same operation.

1. RED interleaving: keep a `BeginBinding`/`StartBinding` legacy attempt outstanding, then ask coordinator to prepare same logical activity; coordinator must reject it without a send token. A coordinator-prepared operation must reject legacy preparation and legacy `Resolve` before and after receipt/claim. Verify no second callback permission and no claimed-row transition to `started`/`definitely_unstarted`; hold/fence remains. Cover process reconstruction and exact replay.
2. GREEN protocol ownership: persist an exclusive server-derived discriminator at preparation and check it at every legacy/coordinator entry; reject existing unmarked intent adoption. Add provider/receipt/send-claim guards to legacy resolve CAS. Keep current Run revision/status CAS and exact receipt binding intact.
3. Run focused SQLite normal/race tests. Bring up a disposable PostgreSQL 17 schema with narrowly scoped DSN, run S1+S2 repository concurrency/status tests and coordinator tests where applicable, record version and cleanup; if PG cannot run, mark S2-R2 open rather than claim PASS. Verify gofmt/diff-check and save hashes/task-local patch.

**Acceptance:** S2-R1 closed with observable cross-path race and hold proof, S2-R2 actual PG evidence. S2-R3 remains explicitly gated on S3 physical send callback tests; do not present one token counter as transport proof. Independent review must give separate scoped Spec/quality conclusions.

## Review focus

All preparation/replay/resolve paths for both protocols, durable marker collision, outstanding callback interleaving, send-claim permanence, and PG actual lock/CAS behavior. Require exact checkpoint coverage; do not infer physical Docker behavior from S2 unit tests.
