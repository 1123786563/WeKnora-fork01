package commercial

import (
	"context"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testBenefitsStore mirrors the planversion_test.go SQLite harness: one
// pooled connection serializes goroutines at the driver level. EnsureSchema
// bootstraps both tables so the store is testable without the migrations.
func testBenefitsStore(t *testing.T) (*BenefitsStore, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	s := NewBenefitsStore(db)
	if err := s.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, db
}

func sampleProjection(tenantID uint64, at time.Time) BenefitsRow {
	return BenefitsRow{
		TenantID:               tenantID,
		ExternalCustomerID:     "weknora-tenant-" + itoa(tenantID),
		ExternalSubscriptionID: "weknora-tenant-" + itoa(tenantID) + "-sub",
		SubscriptionState:      "active",
		PlanCode:               "weknora-base-v1",
		PlanKey:                "base",
		PlanVersion:            1,
		FeaturesJSON:           `{"api_access":true}`,
		LimitsJSON:             `{"members":5,"storage_gb":10}`,
		CreditsBalanceMicro:    1_000_000,
		ProjectedAt:            at,
	}
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	return digits
}

// TestProjectionUpsertRoundTrip: insert → byte-equal read-back; an upsert
// overwrites with a fresh ProjectedAt and keeps ONE row per tenant.
func TestProjectionUpsertRoundTrip(t *testing.T) {
	s, db := testBenefitsStore(t)
	ctx := context.Background()
	first := sampleProjection(7, time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC))
	if err := s.UpsertProjection(ctx, first); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.GetProjection(ctx, 7)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got != first {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, first)
	}
	second := first
	second.SubscriptionState = "pending"
	second.FeaturesJSON = `{}`
	second.ProjectedAt = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	if err := s.UpsertProjection(ctx, second); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err = s.GetProjection(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.SubscriptionState != "pending" || got.ProjectedAt.Equal(first.ProjectedAt) {
		t.Fatalf("upsert must overwrite with a fresh ProjectedAt, got %+v", got)
	}
	var count int64
	db.Table("commercial_tenant_benefits").Where("tenant_id = ?", uint64(7)).Count(&count)
	if count != 1 {
		t.Fatalf("one row per tenant expected, got %d", count)
	}
}

// TestGetProjectionNotFound: an absent projection is the typed not-found
// error, never a fabricated row.
func TestGetProjectionNotFound(t *testing.T) {
	s, _ := testBenefitsStore(t)
	if _, err := s.GetProjection(context.Background(), 999); err != ErrProjectionNotFound {
		t.Fatalf("absent projection must answer ErrProjectionNotFound, got %v", err)
	}
}

// TestEnsureBatchExactlyOnce: the registry unique key is the first grant
// idempotency layer — the second EnsureBatch for the same (tenant, period)
// returns the EXISTING row without inserting; different periods coexist.
func TestEnsureBatchExactlyOnce(t *testing.T) {
	s, db := testBenefitsStore(t)
	ctx := context.Background()
	row := CreditBatchRow{
		TenantID:     7,
		Period:       "2026-09",
		CommandKey:   "grant_included_credits:weknora-tenant-7:2026-09",
		GrantedMicro: 1_000_000,
		ExpiresAt:    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		State:        "granted",
		CreatedAt:    time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
	}
	created, ok, err := s.EnsureBatch(ctx, row)
	if err != nil || !ok {
		t.Fatalf("first EnsureBatch must create: created=%v err=%v", ok, err)
	}
	if created.ID == 0 {
		t.Fatal("the created row must carry its id")
	}
	replay, ok, err := s.EnsureBatch(ctx, row)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if ok {
		t.Fatal("the replay must NOT create")
	}
	if replay.ID != created.ID || replay.GrantedMicro != created.GrantedMicro {
		t.Fatalf("the replay must return the EXISTING row, got %+v want %+v", replay, created)
	}
	var count int64
	db.Table("commercial_credit_batches").Where("tenant_id = ?", uint64(7)).Count(&count)
	if count != 1 {
		t.Fatalf("exactly one row expected after replay, got %d", count)
	}
	// A different period coexists.
	next := row
	next.Period = "2026-10"
	next.CommandKey = "grant_included_credits:weknora-tenant-7:2026-10"
	if _, ok, err := s.EnsureBatch(ctx, next); err != nil || !ok {
		t.Fatalf("a different period must create: %v", err)
	}
	db.Table("commercial_credit_batches").Where("tenant_id = ?", uint64(7)).Count(&count)
	if count != 2 {
		t.Fatalf("two periods must coexist, got %d rows", count)
	}
}

// TestEnsureBatchConcurrentSingleRow: two goroutines racing EnsureBatch for
// the same (tenant, period) converge on exactly one row — the DB unique
// constraint decides, not caller serialization.
func TestEnsureBatchConcurrentSingleRow(t *testing.T) {
	s, db := testBenefitsStore(t)
	ctx := context.Background()
	row := CreditBatchRow{
		TenantID:     8,
		Period:       "2026-09",
		CommandKey:   "grant_included_credits:weknora-tenant-8:2026-09",
		GrantedMicro: 1_000_000,
		ExpiresAt:    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		State:        "granted",
		CreatedAt:    time.Now().UTC(),
	}
	const racers = 2
	started := make(chan struct{})
	ids := make([]int64, racers)
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-started
			got, _, err := s.EnsureBatch(ctx, row)
			if err != nil {
				t.Errorf("racer %d: %v", i, err)
				return
			}
			ids[i] = got.ID
		}(i)
	}
	close(started)
	wg.Wait()
	var count int64
	db.Table("commercial_credit_batches").Where("tenant_id = ? AND period = ?", uint64(8), "2026-09").Count(&count)
	if count != 1 {
		t.Fatalf("the unique constraint must yield exactly one row, got %d", count)
	}
	if ids[0] == 0 || ids[0] != ids[1] {
		t.Fatalf("both racers must observe the same row id, got %v", ids)
	}
}

// TestActiveBatchesExpiryBoundary: the pre-dispatch registry read — a batch
// at expires_at == now is ALREADY expired (boundary is exclusive of
// availability); only strictly future ends are active.
func TestActiveBatchesExpiryBoundary(t *testing.T) {
	s, _ := testBenefitsStore(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	rows := []CreditBatchRow{
		{TenantID: 9, Period: "2026-07", ExpiresAt: now.Add(-time.Minute), State: "granted"},
		{TenantID: 9, Period: "2026-08", ExpiresAt: now, State: "granted"},
		{TenantID: 9, Period: "2026-09", ExpiresAt: now.Add(time.Hour), State: "granted"},
	}
	for i := range rows {
		if _, _, err := s.EnsureBatch(ctx, rows[i]); err != nil {
			t.Fatal(err)
		}
	}
	active, err := s.ActiveBatches(ctx, 9, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].Period != "2026-09" {
		t.Fatalf("only the strictly-future batch is active, got %+v", active)
	}
	// Other tenants' batches never leak into the read.
	foreign, err := s.ActiveBatches(ctx, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(foreign) != 0 {
		t.Fatalf("tenant isolation broken: %+v", foreign)
	}
}

// TestMarkBatchWalletIdempotent: the wallet ref is an identity — re-setting
// the SAME ref is a no-op success; a DIFFERENT ref on a non-empty column is
// an error (identity must never be silently rewritten).
func TestMarkBatchWalletIdempotent(t *testing.T) {
	s, _ := testBenefitsStore(t)
	ctx := context.Background()
	created, _, err := s.EnsureBatch(ctx, CreditBatchRow{
		TenantID: 11, Period: "2026-09", GrantedMicro: 1_000_000,
		ExpiresAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), State: "granted",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkBatchWallet(ctx, created.ID, "lago-wallet-abc"); err != nil {
		t.Fatalf("first wallet mark: %v", err)
	}
	if err := s.MarkBatchWallet(ctx, created.ID, "lago-wallet-abc"); err != nil {
		t.Fatalf("same-ref remark must be a no-op success: %v", err)
	}
	if err := s.MarkBatchWallet(ctx, created.ID, "lago-wallet-xyz"); err == nil {
		t.Fatal("a different ref must be rejected — wallet identity is immutable")
	}
	got, err := s.GetBatch(ctx, 11, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if got.WalletRef != "lago-wallet-abc" {
		t.Fatalf("wallet ref = %q", got.WalletRef)
	}
}

// TestSetBatchExpired flips the advisory state; expiry itself is decided by
// ExpiresAt, so this is bookkeeping only.
func TestSetBatchExpired(t *testing.T) {
	s, _ := testBenefitsStore(t)
	ctx := context.Background()
	created, _, err := s.EnsureBatch(ctx, CreditBatchRow{
		TenantID: 12, Period: "2026-09", GrantedMicro: 1_000_000,
		ExpiresAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), State: "granted",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetBatchExpired(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetBatch(ctx, 12, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "expired" {
		t.Fatalf("state = %q, want expired", got.State)
	}
}

// TestSchemaBootstrapsWithoutMigration: a fresh DB + EnsureSchema leaves
// both tables usable (dev/blocked-env posture — no migration required).
func TestSchemaBootstrapsWithoutMigration(t *testing.T) {
	s, db := testBenefitsStore(t)
	ctx := context.Background()
	for _, table := range []string{"commercial_tenant_benefits", "commercial_credit_batches"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("table %s unusable after EnsureSchema: %v", table, err)
		}
	}
	if err := s.UpsertProjection(ctx, sampleProjection(13, time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureBatch(ctx, CreditBatchRow{
		TenantID: 13, Period: "2026-09", GrantedMicro: 1,
		ExpiresAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), State: "granted",
	}); err != nil {
		t.Fatal(err)
	}
}
