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

// TestAccountPGConcurrentSameTenantDifferentCustomers proves that the
// tenant primary key admits exactly one Customer mapping under real PostgreSQL
// concurrency. Replacing the database constraint or Bind's conflict handling
// would make this test fail.
func TestAccountPGConcurrentSameTenantDifferentCustomers(t *testing.T) {
	s := testAccountPGStore(t)
	assertAccountPGExactlyOneWinner(t, s, []Account{
		{TenantID: 1001, CustomerID: "cus_pg_a"},
		{TenantID: 1001, CustomerID: "cus_pg_b"},
	})
}

// TestAccountPGConcurrentDifferentTenantsSameCustomer proves that the
// Customer unique key cannot be bound by two spaces at once under real
// PostgreSQL concurrency.
func TestAccountPGConcurrentDifferentTenantsSameCustomer(t *testing.T) {
	s := testAccountPGStore(t)
	assertAccountPGExactlyOneWinner(t, s, []Account{
		{TenantID: 1002, CustomerID: "cus_pg_shared"},
		{TenantID: 1003, CustomerID: "cus_pg_shared"},
	})
}

func assertAccountPGExactlyOneWinner(t *testing.T, s *AccountStore, accounts []Account) {
	t.Helper()
	start := make(chan struct{})
	errs := make(chan error, len(accounts))
	var wg sync.WaitGroup
	for _, account := range accounts {
		wg.Add(1)
		go func(account Account) {
			defer wg.Done()
			<-start
			errs <- s.Bind(context.Background(), account)
		}(account)
	}
	close(start)
	wg.Wait()
	close(errs)

	winners := 0
	for err := range errs {
		if err == nil {
			winners++
			continue
		}
		if !errors.Is(err, ErrAccountMappingConflict) {
			t.Fatalf("Bind returned %v; want ErrAccountMappingConflict", err)
		}
	}
	if winners != 1 {
		t.Fatalf("winners=%d; want exactly 1", winners)
	}
}

// testAccountPGStore provisions an isolated schema and applies the production
// commercial-account migration. It is test-only because schema lifecycle is
// not a production AccountStore responsibility.
func testAccountPGStore(t *testing.T) *AccountStore {
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
	schema := fmt.Sprintf("saas_account_%d", time.Now().UnixNano())
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
	migration, err := os.ReadFile("../../../../migrations/versioned/000110_commercial_accounts.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(migration)).Error; err != nil {
		t.Fatalf("apply commercial account migration: %v", err)
	}
	return NewAccountStore(db)
}
