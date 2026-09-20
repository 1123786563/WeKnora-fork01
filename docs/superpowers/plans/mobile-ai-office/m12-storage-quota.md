# M12 Storage Quota Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Track checkboxes as work proceeds.

**Goal:** Enforce space storage bytes atomically through reservations while preserving file read, export, and delete paths when capacity is exceeded.

**Architecture:** Add a storage-specific ledger and reservation port owned by the Craft workspace service boundary. Reservation occurs before new bytes become visible, commits with the file transaction, and releases on failure; it is intentionally independent from Credits/commercial consumption. The quota view returns only capacity facts and does not grant file permissions; M14 later composes this port with mutations.

**Tech Stack:** Go/GORM transactions, Gin, Expo/TypeScript/Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; design `docs/superpowers/specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- M09 is required; this is storage-byte accounting, not Craft budget or Credits accounting.
- Reserve/check/commit must be atomic per tenant; concurrent writes may not oversubscribe the configured byte limit.
- At or above quota, the quota service does not change existing read/export/delete authorization; it only refuses new-byte reservations.
- This track declares migration requirements but controller allocates real versioned/sqlite migration numbers during serial integration.

## Review Focus

- Two concurrent 60-byte reservations under a 100-byte limit must leave at most one held or committed.
- A failed write workflow must release its reservation exactly once.
- Held reservations count against admission but are distinct from committed usage; later M14 owns mutation and deletion integration tests.
- A tenant cannot debit another tenant’s reservation by guessing its workspace ID.
- Quota error responses must expose recovery-safe facts (limit/used/requested), never storage paths or credentials.

---

### Task 1: Atomic workspace storage quota vertical slice

**Files:**
- Create: `internal/application/service/mobileworkspace/quota.go`
- Create: `internal/application/service/mobileworkspace/quota_test.go`
- Create: `internal/application/repository/mobile_workspace_quota.go`
- Create: `internal/application/repository/mobile_workspace_quota_test.go`
- Create: `internal/handler/session/mobile_workspace_quota.go`
- Create: `internal/handler/session/mobile_workspace_quota_test.go`
- Create: `migrations/versioned/<controller-assigned>_mobile_workspace_quota.up.sql` and `.down.sql`
- Create: `migrations/sqlite/<controller-assigned>_mobile_workspace_quota.up.sql` and `.down.sql`
- Create: `apps/mobile-next/src/features/storage-quota/WorkspaceQuotaClient.ts`
- Create: `apps/mobile-next/tests/features/storage-quota/WorkspaceQuotaClient.test.ts`
- Create: `apps/mobile-next/src/features/storage-quota/QuotaRecoveryState.ts`
- Create: `apps/mobile-next/tests/features/storage-quota/QuotaRecoveryState.test.ts`

**Interfaces:**
- Consumes: M09 owner-scoped `Binding`, `craft.Scope`, and the existing transaction-capable GORM repository pattern.
- Produces: **proposed** `StorageReservation{ID string,TenantID uint64,WorkspaceID string,Bytes int64,State "held"|"committed"|"released"}`; `Reserve(ctx, scope, bytes, requestID string) (StorageReservation,error)`, `Commit(ctx,id) error`, `Release(ctx,id) error`, `Usage(ctx,scope) (QuotaView,error)` and `ProjectQuota(view QuotaView, current ExistingCapabilities) ExistingCapabilities`; **proposed** `GET /sessions/:session_id/mobile-workspace/quota`.
- DDL contract: `mobile_storage_usage(tenant_id primary key, limit_bytes, committed_bytes, held_bytes, revision, updated_at)` and `mobile_storage_reservations(id primary key, tenant_id, workspace_id, request_id, bytes, state, expires_at, created_at, updated_at, unique(tenant_id,workspace_id,request_id))`; down deletes reservations then usage. Versioned DB locks the usage row with `SELECT ... FOR UPDATE`; SQLite performs `BEGIN IMMEDIATE`, reads/creates that tenant row, and writes held reservation plus `held_bytes` in one transaction. `Commit`/`Release` use `WHERE state='held'` so identical retries are no-ops; a different replay request conflicts. Startup and every Usage call reconcile expired held rows by atomically subtracting held bytes and marking `released` before new admission.

- [ ] **Step 1: Write concurrent repository and handler RED tests.**

```go
func TestQuotaReserveIsAtomicAcrossConcurrentWriters(t *testing.T) {
  // newQuotaStoreForTest opens one shared SQLite database with the existing
  // repository pattern: sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000").
  store := newQuotaStoreForTest(t, 7, 100) // helper opens shared SQLite, executes this track's minimal quota DDL, then calls NewQuotaStore
  var wg sync.WaitGroup; results := make(chan error, 2)
  scope := craft.Scope{TenantID:7, UserID:"owner", SessionID:"s1"}
  for range 2 { wg.Add(1); go func() { defer wg.Done(); _, err := store.Reserve(context.Background(), scope, 60); results <- err }() }
  wg.Wait(); close(results)
  accepted := 0; for err := range results { if err == nil { accepted++ } }; require.Equal(t, 1, accepted)
  require.Equal(t, int64(0), store.CommittedBytes(t, 7))
  require.Equal(t, int64(60), store.HeldBytes(t, 7)) // held bytes also count against new admission
}
func TestQuotaViewDoesNotExpandExistingCapabilities(t *testing.T) {
  got := mobileworkspace.ProjectQuota(QuotaView{UsedBytes:100, LimitBytes:100}, ExistingCapabilities{CanRead:false, CanExport:true, CanDelete:false, CanWrite:true})
  require.Equal(t, ExistingCapabilities{CanRead:false, CanExport:true, CanDelete:false, CanWrite:false}, got)
}
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/application/repository ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(QuotaReserveIsAtomicAcrossConcurrentWriters|QuotaViewDoesNotExpandExistingCapabilities)' -count=1`
Expected: FAIL because storage reservation does not exist.

- [ ] **Step 3: Write mobile RED tests.**

```ts
it("only removes write capability after quota_exceeded", () => {
  expect(reduceQuota({ code: "quota_exceeded", used: 100, limit: 100 }, { canRead:false, canExport:true, canDelete:false, canWrite:true }))
    .toEqual({ canRead:false, canExport:true, canDelete:false, canWrite:false });
});
```

- [ ] **Step 4: Implement atomic reservation semantics.**

```go
func (s *QuotaStore) Reserve(ctx context.Context, scope craft.Scope, bytes int64, requestID string) (StorageReservation, error) {
  // Postgres/MySQL: lock tenant usage row; SQLite: BEGIN IMMEDIATE. Reconcile expired held rows,
  // replay the same (tenant,workspace,requestID), then require committed+held+bytes <= limit.
  // Insert held reservation and increment held_bytes in the same transaction.
  return s.reserveAtomically(ctx, scope.TenantID, scope.SessionID, bytes, requestID)
}
func (s *QuotaStore) Commit(ctx context.Context, id string) error { return s.transition(ctx, id, "held", "committed") }
```

```ts
export function reduceQuota(error: QuotaError | null, capabilities: FileActions) {
  return error?.code === "quota_exceeded" ? { ...capabilities, canWrite:false } : capabilities;
}
```

- [ ] **Step 5: Run GREEN gates.**

Run: `go test ./internal/application/repository ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(Quota|MobileWorkspaceQuota)' -count=1`
Run: `cd apps/mobile-next && npm test -- --runInBand tests/features/storage-quota && npm run typecheck && npm run check:isolation`
Expected: atomic admission accounting and non-escalating capability projection pass; no Credits service appears in the dependency graph.

- [ ] **Step 6: Review and demo gate.** Use a 100-byte tenant to reserve 60 bytes concurrently twice: exactly one reservation is held, committed usage remains zero before a caller commits it, and capacity facts do not add read/export/delete rights. Record DB-transaction evidence. M14 supplies the later end-to-end write/delete/release demonstration.

## Execution Handoff

Fresh-context ownership is limited to listed feature/service/repository/handler files and DI declaration. M14 calls the reservation port; do not add file mutation endpoints here.
