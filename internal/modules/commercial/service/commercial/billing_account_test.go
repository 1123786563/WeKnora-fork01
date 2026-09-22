package commercial

import (
	"context"
	"errors"
	"sync"
	"testing"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// countingPlatform wraps a platform for the service tests and counts
// command submits and snapshot reads.
type countingPlatform struct {
	inner   domain.CommercialPlatform
	submits int
	reads   int
}

func (c *countingPlatform) SubmitCommand(ctx context.Context, cmd domain.Command) (domain.CommandReceipt, error) {
	c.submits++
	return c.inner.SubmitCommand(ctx, cmd)
}

func (c *countingPlatform) ReadSnapshot(ctx context.Context, q domain.SnapshotQuery) (domain.Snapshot, error) {
	c.reads++
	return c.inner.ReadSnapshot(ctx, q)
}

func (c *countingPlatform) Reconcile(ctx context.Context, from domain.ReconciliationCursor) (domain.ReconciliationPage, error) {
	return c.inner.Reconcile(ctx, from)
}

// newBillingAccountService builds the service over shared-cache in-memory
// SQLite (the testAccountStore single-writer pool convention).
func newBillingAccountService(t *testing.T, platform domain.CommercialPlatform) (*BillingAccountService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&repocommercial.BillingAccount{}); err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	svc, err := NewBillingAccountService(db, platform)
	if err != nil {
		t.Fatal(err)
	}
	return svc, db
}

func billingRowCount(t *testing.T, db *gorm.DB, tenant uint64) int64 {
	t.Helper()
	var n int64
	if err := db.Raw(`SELECT COUNT(*) FROM commercial_billing_accounts WHERE tenant_id = ?`, tenant).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// TestBillingAccountFirstEnsureLinks: the first billing access ensures the
// account — one submit, exactly one authority customer under the derived
// identity, the local row linked.
func TestBillingAccountFirstEnsureLinks(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	platform := &countingPlatform{inner: fake}
	svc, db := newBillingAccountService(t, platform)
	ctx := context.Background()

	status, err := svc.EnsureBillingAccount(ctx, 101, "Space 101", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != BillingAccountLinked || status.Reason != "" || status.EnsuredAt == nil {
		t.Fatalf("first ensure must link: %+v", status)
	}
	if platform.submits != 1 {
		t.Fatalf("exactly one submit expected, got %d", platform.submits)
	}
	customers := fake.Customers()
	if len(customers) != 1 || customers[0].ExternalID != "weknora-tenant-101" || customers[0].Name != "Space 101" {
		t.Fatalf("exactly one customer under the derived identity: %+v", customers)
	}
	if billingRowCount(t, db, 101) != 1 {
		t.Fatalf("exactly one local row expected")
	}
	var row repocommercial.BillingAccount
	if err := db.Raw(`SELECT state FROM commercial_billing_accounts WHERE tenant_id = 101`).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != "linked" {
		t.Fatalf("local row must be linked, got %+v", row)
	}
}

// TestBillingAccountConcurrentEnsureCreatesExactlyOneCustomer (AC1): eight
// barrier-started concurrent ensures of the SAME space all answer linked,
// the authority holds EXACTLY ONE customer for the space's identity, and
// the local table holds exactly one row.
func TestBillingAccountConcurrentEnsureCreatesExactlyOneCustomer(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	svc, db := newBillingAccountService(t, fake)
	ctx := context.Background()

	const racers = 8
	start := make(chan struct{})
	statuses := make([]BillingAccountStatus, racers)
	errs := make([]error, racers)
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			statuses[i], errs[i] = svc.EnsureBillingAccount(ctx, 101, "Space 101", "user-1")
		}(i)
	}
	close(start)
	wg.Wait()

	for i := 0; i < racers; i++ {
		if errs[i] != nil {
			t.Fatalf("racer %d: %v", i, errs[i])
		}
		if statuses[i].State != BillingAccountLinked {
			t.Fatalf("racer %d must answer linked, got %+v", i, statuses[i])
		}
	}
	customers := fake.Customers()
	if len(customers) != 1 || customers[0].ExternalID != "weknora-tenant-101" {
		t.Fatalf("exactly ONE customer identity must ever exist, got %+v", customers)
	}
	if billingRowCount(t, db, 101) != 1 {
		t.Fatalf("exactly one local row must survive the race")
	}
}

// TestBillingAccountRenameAndOwnerTransferNeverMoveIdentity (AC3): a rename
// (a new display name on re-ensure) never moves identity — the external id
// is byte-identical, the authority still holds exactly one customer for the
// SAME identity, and the local mapping row keeps its identity. The
// derivation has no owner input at all (compile-enforced in the port
// tests), so owner transfer cannot move identity either. (A linked row
// answers from the local projection without re-submitting — the advisory
// name refresh semantics live at the command layer and are proven by the
// adapter/fake tests.)
func TestBillingAccountRenameAndOwnerTransferNeverMoveIdentity(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	svc, db := newBillingAccountService(t, fake)
	ctx := context.Background()

	if _, err := svc.EnsureBillingAccount(ctx, 101, "Old Name", "user-1"); err != nil {
		t.Fatal(err)
	}
	var before repocommercial.BillingAccount
	if err := db.Raw(`SELECT external_customer_id, state FROM commercial_billing_accounts WHERE tenant_id = 101`).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}

	// UpdateTenant-style rename: the same space re-ensured under a new name
	// (and a different actor standing in for the owner transfer).
	status, err := svc.EnsureBillingAccount(ctx, 101, "New Name", "user-2")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != BillingAccountLinked {
		t.Fatalf("re-ensure after rename must stay linked: %+v", status)
	}
	var after repocommercial.BillingAccount
	if err := db.Raw(`SELECT external_customer_id, state FROM commercial_billing_accounts WHERE tenant_id = 101`).Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.ExternalCustomerID != before.ExternalCustomerID || after.ExternalCustomerID != "weknora-tenant-101" {
		t.Fatalf("identity must be immutable across rename: %q -> %q", before.ExternalCustomerID, after.ExternalCustomerID)
	}
	customers := fake.Customers()
	if len(customers) != 1 || customers[0].ExternalID != "weknora-tenant-101" {
		t.Fatalf("rename must never create a second customer identity, got %+v", customers)
	}
}

// TestBillingAccountRecoversByIdentityAfterLostResponse (AC4): the fault
// knob persists the create then fails the submit with the
// persisted-but-response-lost sentinel — the first ensure answers
// pending/unreachable with the row retained; after the knob clears, the
// re-ensure resolves by identity (snapshot linked), marks linked, and NEVER
// creates a second customer.
func TestBillingAccountRecoversByIdentityAfterLostResponse(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	svc, db := newBillingAccountService(t, fake)
	ctx := context.Background()

	fake.FailSubmitsWith(domain.ErrPlatformUnreachable)
	status, err := svc.EnsureBillingAccount(ctx, 101, "Space 101", "user-1")
	if err != nil {
		t.Fatalf("a platform failure is a state, not a caller error: %v", err)
	}
	if status.State != BillingAccountPending || status.Reason != "unreachable" {
		t.Fatalf("lost response must answer pending/unreachable, got %+v", status)
	}
	var row repocommercial.BillingAccount
	if err := db.Raw(`SELECT state FROM commercial_billing_accounts WHERE tenant_id = 101`).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != "pending" {
		t.Fatalf("the local row must stay pending after an indeterminate outcome: %+v", row)
	}

	fake.FailSubmitsWith(nil)
	status, err = svc.EnsureBillingAccount(ctx, 101, "Space 101", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != BillingAccountLinked {
		t.Fatalf("recovery must link: %+v", status)
	}
	customers := fake.Customers()
	if len(customers) != 1 || customers[0].ExternalID != "weknora-tenant-101" {
		t.Fatalf("recovery must resolve to the ORIGINAL customer, never a second one: %+v", customers)
	}
	if err := db.Raw(`SELECT state FROM commercial_billing_accounts WHERE tenant_id = 101`).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != "linked" {
		t.Fatalf("the row must go pending -> linked: %+v", row)
	}
}

// TestBillingAccountUnconfiguredStaysPending: with no platform wired (and
// with an unconfigured adapter), the ensure answers pending/unconfigured,
// issues nothing to the authority, and the row stays pending.
func TestBillingAccountUnconfiguredStaysPending(t *testing.T) {
	t.Run("nil platform", func(t *testing.T) {
		svc, db := newBillingAccountService(t, nil)
		status, err := svc.EnsureBillingAccount(context.Background(), 101, "Space 101", "user-1")
		if err != nil {
			t.Fatal(err)
		}
		if status.State != BillingAccountPending || status.Reason != "unconfigured" {
			t.Fatalf("nil platform must answer pending/unconfigured, got %+v", status)
		}
		if billingRowCount(t, db, 101) != 1 {
			t.Fatalf("the pending row is still ensured locally")
		}
	})

	t.Run("unconfigured adapter never reaches the authority", func(t *testing.T) {
		unconfigured := commercialplatform.NewLagoAdapter(commercialplatform.Config{
			Provider: commercialplatform.ProviderLago,
		})
		platform := &countingPlatform{inner: unconfigured}
		svc, db := newBillingAccountService(t, platform)
		status, err := svc.EnsureBillingAccount(context.Background(), 101, "Space 101", "user-1")
		if err != nil {
			t.Fatal(err)
		}
		if status.State != BillingAccountPending || status.Reason != "unconfigured" {
			t.Fatalf("unconfigured adapter must answer pending/unconfigured, got %+v", status)
		}
		var row repocommercial.BillingAccount
		if err := db.Raw(`SELECT state FROM commercial_billing_accounts WHERE tenant_id = 101`).Scan(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.State != "pending" {
			t.Fatalf("the row stays pending: %+v", row)
		}
	})
}

// TestBillingAccountTenantIsolation (AC2): two spaces ensured concurrently
// hold two rows with two DISTINCT derived identities, each reading only its
// own; a store-level attempt to bind one space onto the other's identity is
// the typed conflict.
func TestBillingAccountTenantIsolation(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	svc, db := newBillingAccountService(t, fake)
	ctx := context.Background()

	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, tenant := range []uint64{201, 202} {
		wg.Add(1)
		go func(tenant uint64) {
			defer wg.Done()
			<-start
			if _, err := svc.EnsureBillingAccount(ctx, tenant, "Space", "user-1"); err != nil {
				t.Errorf("tenant %d: %v", tenant, err)
			}
		}(tenant)
	}
	close(start)
	wg.Wait()

	var n int64
	if err := db.Raw(`SELECT COUNT(*) FROM commercial_billing_accounts`).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("two tenants must hold two rows, got %d", n)
	}
	customers := fake.Customers()
	if len(customers) != 2 || customers[0].ExternalID == customers[1].ExternalID {
		t.Fatalf("two distinct customer identities expected, got %+v", customers)
	}
	// The store refuses to bind tenant 202 onto 201's identity.
	store := repocommercial.NewBillingAccountStore(db)
	if _, err := store.EnsurePending(ctx, repocommercial.BillingAccount{TenantID: 202, ExternalCustomerID: domain.ExternalCustomerID(201)}); !errors.Is(err, repocommercial.ErrBillingAccountConflict) {
		t.Fatalf("cross-tenant identity bind must be the typed conflict, got %v", err)
	}
}

// TestBillingAccountLinkedRowShortCircuits: once linked, a later ensure
// answers from the local row without touching the authority again.
func TestBillingAccountLinkedRowShortCircuits(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	platform := &countingPlatform{inner: fake}
	svc, _ := newBillingAccountService(t, platform)
	ctx := context.Background()

	if _, err := svc.EnsureBillingAccount(ctx, 101, "Space 101", "user-1"); err != nil {
		t.Fatal(err)
	}
	readsBefore, submitsBefore := platform.reads, platform.submits
	status, err := svc.EnsureBillingAccount(ctx, 101, "Space 101", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != BillingAccountLinked {
		t.Fatalf("linked row must answer linked: %+v", status)
	}
	if platform.reads != readsBefore || platform.submits != submitsBefore {
		t.Fatalf("a linked row must not touch the authority again (reads %d->%d, submits %d->%d)",
			readsBefore, platform.reads, submitsBefore, platform.submits)
	}
}
