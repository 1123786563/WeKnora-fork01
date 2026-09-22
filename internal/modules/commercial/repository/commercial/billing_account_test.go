package commercial

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func mustTime(iso string) time.Time {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		panic(err)
	}
	return t
}

func uitoa(v uint64) string { return strconv.FormatUint(v, 10) }

// testBillingAccountStore mirrors testAccountStore: shared-cache in-memory
// SQLite with the production single-writer pool (SetMaxOpenConns(1)) — the
// unique constraints, not connection racing, decide the winner.
func testBillingAccountStore(t *testing.T) *BillingAccountStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&BillingAccount{}); err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	return NewBillingAccountStore(db)
}

// TestBillingAccountEnsurePendingIsIdempotent: the same (tenant, identity)
// pair ensures exactly one pending row twice over; a DIFFERENT identity for
// the same tenant is a typed conflict (identity is immutable per tenant).
func TestBillingAccountEnsurePendingIsIdempotent(t *testing.T) {
	s := testBillingAccountStore(t)
	ctx := context.Background()

	row, err := s.EnsurePending(ctx, BillingAccount{TenantID: 7, ExternalCustomerID: "weknora-tenant-7"})
	if err != nil {
		t.Fatal(err)
	}
	if row.TenantID != 7 || row.ExternalCustomerID != "weknora-tenant-7" || row.State != "pending" {
		t.Fatalf("first ensure shape: %+v", row)
	}
	row2, err := s.EnsurePending(ctx, BillingAccount{TenantID: 7, ExternalCustomerID: "weknora-tenant-7"})
	if err != nil {
		t.Fatal(err)
	}
	if row2.State != "pending" {
		t.Fatalf("re-ensure keeps the row pending: %+v", row2)
	}

	if _, err := s.EnsurePending(ctx, BillingAccount{TenantID: 7, ExternalCustomerID: "weknora-tenant-other"}); !errors.Is(err, ErrBillingAccountConflict) {
		t.Fatalf("a different identity for the same tenant must be a typed conflict, got %v", err)
	}
}

// TestBillingAccountEnsurePendingCompetingSameTenantDecideOneWinner:
// concurrent ensures of the same tenant leave exactly one row.
func TestBillingAccountEnsurePendingCompetingSameTenantDecideOneWinner(t *testing.T) {
	s := testBillingAccountStore(t)
	ctx := context.Background()
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			// The identity is DERIVED, so every racer passes the identical
			// value — the constraint has nothing to arbitrate but the row.
			_, _ = s.EnsurePending(ctx, BillingAccount{TenantID: 9, ExternalCustomerID: "weknora-tenant-9"})
		}()
	}
	close(start)
	wg.Wait()

	got, err := s.Get(ctx, 9)
	if err != nil {
		t.Fatalf("exactly one row must survive: %v", err)
	}
	if got.ExternalCustomerID != "weknora-tenant-9" {
		t.Fatalf("row identity: %+v", got)
	}
}

// TestBillingAccountExternalIdentityIsUniqueAcrossTenants: two tenants can
// NEVER share one external customer id — the typed conflict fires.
func TestBillingAccountExternalIdentityIsUniqueAcrossTenants(t *testing.T) {
	s := testBillingAccountStore(t)
	ctx := context.Background()
	if _, err := s.EnsurePending(ctx, BillingAccount{TenantID: 10, ExternalCustomerID: "weknora-tenant-shared"}); err != nil {
		t.Fatal(err)
	}
	_, err := s.EnsurePending(ctx, BillingAccount{TenantID: 11, ExternalCustomerID: "weknora-tenant-shared"})
	if !errors.Is(err, ErrBillingAccountConflict) {
		t.Fatalf("cross-tenant identity sharing must be a typed conflict, got %v", err)
	}
	if _, err := s.Get(ctx, 11); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("the losing tenant must hold no row, got %v", err)
	}
}

// TestBillingAccountMarkLinkedIsIdempotent: pending→linked once, then
// linked→linked refreshes without error.
func TestBillingAccountMarkLinkedIsIdempotent(t *testing.T) {
	s := testBillingAccountStore(t)
	ctx := context.Background()
	if _, err := s.EnsurePending(ctx, BillingAccount{TenantID: 12, ExternalCustomerID: "weknora-tenant-12"}); err != nil {
		t.Fatal(err)
	}
	at := mustTime("2026-09-21T12:00:00Z")
	row, err := s.MarkLinked(ctx, 12, "weknora-tenant-12", at)
	if err != nil {
		t.Fatal(err)
	}
	if row.State != "linked" || row.ProviderCustomerRef != "weknora-tenant-12" || row.EnsuredAt == nil || !row.EnsuredAt.Equal(at) {
		t.Fatalf("linked shape: %+v", row)
	}
	at2 := mustTime("2026-09-21T13:00:00Z")
	row, err = s.MarkLinked(ctx, 12, "weknora-tenant-12", at2)
	if err != nil {
		t.Fatal(err)
	}
	if row.State != "linked" || !row.EnsuredAt.Equal(at2) {
		t.Fatalf("linked→linked refresh: %+v", row)
	}
}

// TestBillingAccountGetIsTenantScoped: every read is WHERE tenant_id = ?.
func TestBillingAccountGetIsTenantScoped(t *testing.T) {
	s := testBillingAccountStore(t)
	ctx := context.Background()
	for _, tenant := range []uint64{201, 202} {
		ext := "weknora-tenant-" + uitoa(tenant)
		if _, err := s.EnsurePending(ctx, BillingAccount{TenantID: tenant, ExternalCustomerID: ext}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tenant := range []uint64{201, 202} {
		got, err := s.Get(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		if got.TenantID != tenant || got.ExternalCustomerID != "weknora-tenant-"+uitoa(tenant) {
			t.Fatalf("tenant %d must read only its own row, got %+v", tenant, got)
		}
	}
	if _, err := s.Get(ctx, 999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("an unknown tenant has no row, got %v", err)
	}
}
