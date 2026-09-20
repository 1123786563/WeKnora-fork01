//go:build commercial_integration

package commercial

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestBillingAccountPGConcurrentEnsureDecidesOneRowPerTenant proves under
// REAL PostgreSQL concurrency that the tenant primary key + ON CONFLICT DO
// NOTHING admit exactly one billing account row per space, and that
// MarkLinked races converge on one linked row. Replacing the constraint or
// EnsurePending's conflict handling must fail this test. (The SQLite twin in
// billing_account_test.go serializes writers on purpose — the true
// concurrency verdict rests here.)
func TestBillingAccountPGConcurrentEnsureDecidesOneRowPerTenant(t *testing.T) {
	s := testBillingAccountPGStore(t)
	ctx := context.Background()
	const racers = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			// Every racer derives the identical identity, so the constraint
			// has nothing to arbitrate but the single row.
			_, _ = s.EnsurePending(ctx, BillingAccount{TenantID: 3001, ExternalCustomerID: "weknora-tenant-3001"})
		}()
	}
	close(start)
	wg.Wait()

	row, err := s.Get(ctx, 3001)
	if err != nil {
		t.Fatalf("exactly one row must survive the race: %v", err)
	}
	if row.ExternalCustomerID != "weknora-tenant-3001" || row.State != "pending" {
		t.Fatalf("row shape after race: %+v", row)
	}

	// Concurrent MarkLinked races converge on one linked row.
	at := time.Now().UTC()
	var wg2 sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			<-start
			_, _ = s.MarkLinked(ctx, 3001, "weknora-tenant-3001", at)
		}()
	}
	wg2.Wait()
	row, err = s.Get(ctx, 3001)
	if err != nil {
		t.Fatal(err)
	}
	if row.State != "linked" || row.EnsuredAt == nil || !row.EnsuredAt.Equal(at) {
		t.Fatalf("linked row after MarkLinked race: %+v", row)
	}
}

// TestBillingAccountPGCrossTenantIdentityConflict: two spaces can never
// share one external customer id under real PostgreSQL — the second bind
// loses with the typed conflict and holds no row.
func TestBillingAccountPGCrossTenantIdentityConflict(t *testing.T) {
	s := testBillingAccountPGStore(t)
	ctx := context.Background()
	if _, err := s.EnsurePending(ctx, BillingAccount{TenantID: 3002, ExternalCustomerID: "weknora-tenant-pg-shared"}); err != nil {
		t.Fatal(err)
	}
	_, err := s.EnsurePending(ctx, BillingAccount{TenantID: 3003, ExternalCustomerID: "weknora-tenant-pg-shared"})
	if !errors.Is(err, ErrBillingAccountConflict) {
		t.Fatalf("cross-tenant identity sharing must be a typed conflict, got %v", err)
	}
}

// testBillingAccountPGStore provisions an isolated schema and applies the
// production billing-account migration (the account_pg_test.go pattern).
func testBillingAccountPGStore(t *testing.T) *BillingAccountStore {
	t.Helper()
	raw := os.Getenv("SAAS_TEST_PG_DSN")
	if raw == "" {
		t.Skip("blocked-env: SAAS_TEST_PG_DSN is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		t.Fatal("SAAS_TEST_PG_DSN must be a PostgreSQL URL")
	}
	admin, err := gorm.Open(postgres.Open(raw), &gorm.Config{})
	if err != nil {
		t.Fatalf("open PostgreSQL admin connection: %v", err)
	}
	adminPool, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	var pool *sql.DB
	schema := fmt.Sprintf("saas_billing_account_%d", time.Now().UnixNano())
	schemaCreated := false
	cleanup := func() {
		if pool != nil {
			if err := pool.Close(); err != nil {
				t.Errorf("close isolated schema pool: %v", err)
			}
		}
		if schemaCreated {
			if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
				t.Errorf("drop isolated schema: %v", err)
			}
		}
		if err := adminPool.Close(); err != nil {
			t.Errorf("close PostgreSQL admin pool: %v", err)
		}
	}
	t.Cleanup(cleanup)
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	schemaCreated = true
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open isolated schema connection: %v", err)
	}
	pool, err = db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(20)
	migration, err := os.ReadFile("../../../../migrations/versioned/000178_commercial_billing_accounts.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(migration)).Error; err != nil {
		t.Fatalf("apply billing account migration: %v", err)
	}
	return NewBillingAccountStore(db)
}
