package appconnector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ocMigrationSQL loads the sqlite twin migration production uses, so the
// repository tests exercise the real schema — composite foreign keys,
// uniqueness and check constraints included — rather than an AutoMigrate
// approximation.
func ocMigrationSQL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../../migrations/sqlite/000041_open_connector_bindings.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// testOCStore builds an isolated sqlite database per test: the parent
// installation tables are AutoMigrated exactly like the existing repository
// tests, then the real OC migration SQL is applied. _foreign_keys=1 enables
// composite foreign key enforcement so fixtures can never bypass the
// connections(tenant_id, id) parent.
func testOCStore(t *testing.T) (*OCStore, *InstallationStore) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000&_foreign_keys=1"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AppVersion{}, &InstallationRow{}, &ConnectionRow{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(ocMigrationSQL(t)).Error; err != nil {
		t.Fatal(err)
	}
	return NewOCStore(db), NewInstallationStore(db)
}

// ocSeedConnection creates the real parent rows for a tenant so binding
// fixtures satisfy the composite foreign key. One installation per tenant is
// created lazily; additional connections hang off the same installation.
func ocSeedConnection(t *testing.T, inst *InstallationStore, tenant uint64, connID string) {
	t.Helper()
	ctx := context.Background()
	instID := fmt.Sprintf("inst-%d", tenant)
	if _, _, err := inst.GetInstallation(ctx, tenant, "app-oc"); errors.Is(err, gorm.ErrRecordNotFound) {
		if err := inst.ApplyInstallation(ctx, appconnector.Installation{ID: instID, AppID: "app-oc", Version: "1.0.0", State: appconnector.InstallationActive, TenantID: tenant}, 0); err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	if err := inst.SaveConnection(ctx, appconnector.Connection{
		ID: connID, InstallationID: instID, Kind: appconnector.ConnectionKindSpace,
		OwnerID: "owner", CredentialRef: "cred/" + instID, State: appconnector.ConnectionActive,
		TenantID: tenant, AuthVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}
}

func ocPendingBinding(tenant uint64, connID, externalID, alias string) appconnector.OCBinding {
	return appconnector.OCBinding{
		TenantID: tenant, ConnectionID: connID,
		RuntimeID: "rt-1", Provider: "github",
		ExternalID: externalID, Alias: alias,
		AuthVersion: 1, BindingVersion: 1, State: appconnector.OCBindingPending,
	}
}

// TestOCBindingRejectsZeroTenant pins the plan's unscoped-lookup rejection:
// input validation happens before any database access, so even a store
// without a database rejects a zero tenant instead of panicking or leaking
// rows.
func TestOCBindingRejectsZeroTenant(t *testing.T) {
	// 本测试不需要数据库；输入必须在访问数据库前拒绝。
	s := NewOCStore(nil)
	if _, err := s.GetBinding(context.Background(), 0, "c1"); err == nil {
		t.Fatal("unscoped lookup")
	}
	if err := s.SaveBinding(context.Background(), ocPendingBinding(0, "c1", "e1", "a1")); err == nil {
		t.Fatal("unscoped save")
	}
}

// TestOCBindingRejectsInvalidInput keeps malformed bindings out of the store
// before the database is ever touched: every identity field must be present,
// versions positive, states from the closed set, and creation always starts
// at binding version 1.
func TestOCBindingRejectsInvalidInput(t *testing.T) {
	s, _ := testOCStore(t)
	ctx := context.Background()
	cases := []struct {
		name   string
		mutate func(*appconnector.OCBinding)
	}{
		{"missing connection", func(b *appconnector.OCBinding) { b.ConnectionID = "" }},
		{"missing runtime", func(b *appconnector.OCBinding) { b.RuntimeID = "" }},
		{"missing provider", func(b *appconnector.OCBinding) { b.Provider = "" }},
		{"missing external id", func(b *appconnector.OCBinding) { b.ExternalID = "" }},
		{"missing alias", func(b *appconnector.OCBinding) { b.Alias = "" }},
		{"zero auth version", func(b *appconnector.OCBinding) { b.AuthVersion = 0 }},
		{"zero binding version", func(b *appconnector.OCBinding) { b.BindingVersion = 0 }},
		{"unknown state", func(b *appconnector.OCBinding) { b.State = "live" }},
		{"creation past version 1", func(b *appconnector.OCBinding) { b.BindingVersion = 3 }},
	}
	for _, tc := range cases {
		b := ocPendingBinding(7, "c1", "e1", "a1")
		tc.mutate(&b)
		if err := s.SaveBinding(ctx, b); !errors.Is(err, ErrOCBindingInvalid) {
			t.Fatalf("%s: err=%v", tc.name, err)
		}
	}
	if _, err := s.GetBinding(ctx, 7, ""); err == nil {
		t.Fatal("empty connection id accepted")
	}
}

// TestOCBindingTenantScopedRead covers the core multi-tenant boundary: the
// same connection id exists in two tenants, only the owning tenant's binding
// is visible, and the row round-trips faithfully.
func TestOCBindingTenantScopedRead(t *testing.T) {
	s, inst := testOCStore(t)
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "c1")
	ocSeedConnection(t, inst, 8, "c1")
	want := appconnector.OCBinding{
		TenantID: 7, ConnectionID: "c1", RuntimeID: "rt-1", Provider: "github",
		ExternalID: "ext-9", Alias: "alias-9", AuthVersion: 1, BindingVersion: 1,
		State: appconnector.OCBindingPending,
	}
	if err := s.SaveBinding(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetBinding(ctx, 7, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("roundtrip mismatch: got %+v want %+v", got, want)
	}
	// tenant B reading tenant A's connection id must see nothing
	if _, err := s.GetBinding(ctx, 8, "c1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant read leaked: err=%v", err)
	}
}

// TestOCBindingRequiresParentConnection proves the composite foreign key:
// a binding needs the exact (tenant_id, connection_id) pair to exist in
// connections — a same-named connection in another tenant does not satisfy
// it.
func TestOCBindingRequiresParentConnection(t *testing.T) {
	s, inst := testOCStore(t)
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "c-real")
	ghost := ocPendingBinding(7, "c-ghost", "e1", "a1")
	if err := s.SaveBinding(ctx, ghost); err == nil {
		t.Fatal("binding for missing connection accepted")
	}
	// connection c-real exists only in tenant 7
	crossTenant := ocPendingBinding(8, "c-real", "e1", "a1")
	if err := s.SaveBinding(ctx, crossTenant); err == nil {
		t.Fatal("composite parent satisfied across tenants")
	}
	real := ocPendingBinding(7, "c-real", "e1", "a1")
	if err := s.SaveBinding(ctx, real); err != nil {
		t.Fatal(err)
	}
}

// TestOCBindingExternalIDUnique pins the physical uniqueness of an external
// connection: (runtime_id, external_id) can carry at most one local binding,
// across connections and across tenants.
func TestOCBindingExternalIDUnique(t *testing.T) {
	s, inst := testOCStore(t)
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "c1")
	ocSeedConnection(t, inst, 7, "c2")
	ocSeedConnection(t, inst, 8, "c3")
	first := ocPendingBinding(7, "c1", "ext-1", "alias-1")
	if err := s.SaveBinding(ctx, first); err != nil {
		t.Fatal(err)
	}
	dup := first
	dup.ConnectionID, dup.Alias = "c2", "alias-2"
	if err := s.SaveBinding(ctx, dup); err == nil {
		t.Fatal("external id re-bound to a second connection")
	}
	dupTenant := first
	dupTenant.TenantID, dupTenant.ConnectionID, dupTenant.Alias = 8, "c3", "alias-3"
	if err := s.SaveBinding(ctx, dupTenant); err == nil {
		t.Fatal("external id re-bound across tenants")
	}
	// external ids are namespaced per runtime
	otherRuntime := first
	otherRuntime.ConnectionID, otherRuntime.RuntimeID, otherRuntime.Alias = "c2", "rt-2", "alias-4"
	if err := s.SaveBinding(ctx, otherRuntime); err != nil {
		t.Fatal(err)
	}
}

// TestOCBindingAliasNeverReusedAfterRevoke proves alias immutability outlives
// revocation: the revoked row keeps occupying (runtime_id, provider, alias),
// so a fresh binding can never take the alias — not even from another
// generation. Alias uniqueness is per provider on the same runtime.
func TestOCBindingAliasNeverReusedAfterRevoke(t *testing.T) {
	s, inst := testOCStore(t)
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "c1")
	ocSeedConnection(t, inst, 7, "c2")
	b := ocPendingBinding(7, "c1", "ext-1", "alias-1")
	if err := s.SaveBinding(ctx, b); err != nil {
		t.Fatal(err)
	}
	b.State, b.BindingVersion = appconnector.OCBindingActive, 2
	if err := s.SaveBinding(ctx, b); err != nil {
		t.Fatal(err)
	}
	b.State, b.BindingVersion = appconnector.OCBindingRevoked, 3
	if err := s.SaveBinding(ctx, b); err != nil {
		t.Fatal(err)
	}
	fresh := ocPendingBinding(7, "c2", "ext-2", "alias-1")
	if err := s.SaveBinding(ctx, fresh); err == nil {
		t.Fatal("revoked alias reused by a new binding")
	}
	// the alias stays reserved only within its provider namespace
	otherProvider := ocPendingBinding(7, "c2", "ext-2", "alias-1")
	otherProvider.Provider = "notion"
	if err := s.SaveBinding(ctx, otherProvider); err != nil {
		t.Fatal(err)
	}
}

// TestOCBindingOptimisticVersions pins the concurrency contract: binding
// version is a strict compare-and-swap counter (no replays, no skipped
// generations) and the authorization version never regresses.
func TestOCBindingOptimisticVersions(t *testing.T) {
	s, inst := testOCStore(t)
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "c1")
	b := ocPendingBinding(7, "c1", "ext-1", "alias-1")
	if err := s.SaveBinding(ctx, b); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBinding(ctx, b); !errors.Is(err, ErrOCBindingConflict) {
		t.Fatalf("stale generation replay accepted: err=%v", err)
	}
	skip := b
	skip.BindingVersion = 3
	if err := s.SaveBinding(ctx, skip); !errors.Is(err, ErrOCBindingConflict) {
		t.Fatalf("generation skip accepted: err=%v", err)
	}
	next := b
	next.BindingVersion, next.State = 2, appconnector.OCBindingActive
	if err := s.SaveBinding(ctx, next); err != nil {
		t.Fatal(err)
	}
	bump := next
	bump.BindingVersion, bump.AuthVersion, bump.State = 3, 2, appconnector.OCBindingRevoked
	if err := s.SaveBinding(ctx, bump); err != nil {
		t.Fatal(err)
	}
	regress := bump
	regress.BindingVersion, regress.AuthVersion, regress.State = 4, 1, appconnector.OCBindingActive
	if err := s.SaveBinding(ctx, regress); !errors.Is(err, ErrOCBindingConflict) {
		t.Fatalf("auth version regression accepted: err=%v", err)
	}
	got, err := s.GetBinding(ctx, 7, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if got.AuthVersion != 2 || got.BindingVersion != 3 || got.State != appconnector.OCBindingRevoked {
		t.Fatalf("regressed row persisted: %+v", got)
	}
}

// TestOCBindingIdentityImmutable keeps the external identity of a binding
// fixed for its whole life: alias and external id can never be rewritten in
// place (a replacement is a new binding with a new alias). Runtime and
// provider moves are operational changes and ride the version counter.
func TestOCBindingIdentityImmutable(t *testing.T) {
	s, inst := testOCStore(t)
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "c1")
	b := ocPendingBinding(7, "c1", "ext-1", "alias-1")
	if err := s.SaveBinding(ctx, b); err != nil {
		t.Fatal(err)
	}
	retag := b
	retag.BindingVersion, retag.Alias = 2, "alias-2"
	if err := s.SaveBinding(ctx, retag); !errors.Is(err, ErrOCBindingConflict) {
		t.Fatalf("alias rewrite accepted: err=%v", err)
	}
	reswitch := b
	reswitch.BindingVersion, reswitch.ExternalID = 2, "ext-2"
	if err := s.SaveBinding(ctx, reswitch); !errors.Is(err, ErrOCBindingConflict) {
		t.Fatalf("external id rewrite accepted: err=%v", err)
	}
	move := b
	move.BindingVersion, move.RuntimeID, move.State = 2, "rt-2", appconnector.OCBindingActive
	if err := s.SaveBinding(ctx, move); err != nil {
		t.Fatal(err)
	}
}
