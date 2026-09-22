package commercial

import (
	"context"
	"errors"
	"sync"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testAccountStore(t *testing.T) *AccountStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Account{}, &Grant{}); err != nil {
		t.Fatal(err)
	}
	// Mirror the production SQLite pool (single writer): shared-cache
	// in-memory SQLite reports SQLITE_LOCKED ("database table is locked")
	// when two connections write concurrently, and busy_timeout does not
	// retry that error. Serialising on one connection matches the
	// deployed topology, where the unique constraints — not connection
	// racing — decide the winner.
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	return NewAccountStore(db)
}

func TestAccountBindIsIdempotentAndScoped(t *testing.T) {
	s := testAccountStore(t)
	ctx := context.Background()
	a := Account{TenantID: 7, CustomerID: "cus_7"}
	if err := s.Bind(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := s.Bind(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if err := s.Bind(ctx, Account{TenantID: 7, CustomerID: "cus_other"}); !errors.Is(err, ErrAccountMappingConflict) {
		t.Fatalf("got %v", err)
	}
	if err := s.Bind(ctx, Account{TenantID: 8, CustomerID: "cus_7"}); !errors.Is(err, ErrAccountMappingConflict) {
		t.Fatalf("got %v", err)
	}
}

func TestAccountBindCompetingMappingsRemainUnique(t *testing.T) {
	s := testAccountStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, a := range []Account{{TenantID: 9, CustomerID: "cus_a"}, {TenantID: 9, CustomerID: "cus_b"}} {
		wg.Add(1)
		go func(a Account) { defer wg.Done(); errs <- s.Bind(ctx, a) }(a)
	}
	wg.Wait()
	close(errs)
	var success int
	for err := range errs {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrAccountMappingConflict) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("successes=%d", success)
	}
}

func TestAccountBindCompetingTenantsSameCustomerRemainUnique(t *testing.T) {
	s := testAccountStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, a := range []Account{{TenantID: 10, CustomerID: "cus_shared"}, {TenantID: 11, CustomerID: "cus_shared"}} {
		wg.Add(1)
		go func(a Account) { defer wg.Done(); errs <- s.Bind(ctx, a) }(a)
	}
	wg.Wait()
	close(errs)
	var success int
	for err := range errs {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrAccountMappingConflict) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("successes=%d", success)
	}
}
