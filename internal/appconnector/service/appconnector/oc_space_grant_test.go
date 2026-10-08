package appconnector

// T23 (#53): the REAL grant store rides the A02 authorizer. These tests pin
// the three adjudication facts that matter to a person using the product:
// a space connection needs its own grant; a revoked (or foreign-tenant)
// grant stops resolving on the very next Check; and a space grant NEVER
// opens a personal connection (AC1: 个人与空间连接不能互相替代).

import (
	"context"
	"errors"
	"testing"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	"gorm.io/gorm"
)

// newDBGrantAuthorizer is newAuthorizerFixture's twin, with the REAL
// SpaceConnectionGrantStore in place of the in-memory map source. Reuses the
// same AutoMigrate table set (newResolverFixture) plus the grant model.
func newDBGrantAuthorizer(t *testing.T) (appconn.OCAuthorizer, *appconnectorrepo.SpaceConnectionGrantStore, *gorm.DB) {
	t.Helper()
	_, db := newResolverFixture(t)
	// OCBindingRow joins the set because the bindings source (NewOCStore)
	// needs connector_connection_bindings — the same table the authorizer
	// fixture in oc_authorizer_test.go migrates but newResolverFixture omits.
	if err := db.AutoMigrate(&appconnectorrepo.SpaceConnectionGrantRow{}, &appconnectorrepo.OCBindingRow{}); err != nil {
		t.Fatal(err)
	}
	authz := NewOCAuthorizer(
		apprepo.NewMCPOAuthBindingStore(db),
		&dbInstallationSource{db: db},
		appconnectorrepo.NewSpaceConnectionGrantStore(db),
		appconnectorrepo.NewOCStore(db),
	)
	return authz, appconnectorrepo.NewSpaceConnectionGrantStore(db), db
}

func TestSpaceGrantAuthorizerGrantAdmitsAndRevokeConverges(t *testing.T) {
	authz, grants, db := newDBGrantAuthorizer(t)
	seedOCSubjectFixture(t, db) // tenant 7: alice(owner) + bob, c-personal + c-space, auth_version 2
	ctx := context.Background()
	bob := appconn.OCSubject{TenantID: 7, ActorID: "bob"}

	// No grant: the space connection fails closed (the pre-#53 default).
	_, err := authz.Check(ctx, bob, "c-space", 2)
	if !errors.Is(err, ErrConnectionForbidden) {
		t.Fatalf("want ErrConnectionForbidden, got %v", err)
	}

	// An explicit grant admits bob's very next Check (native branch: the
	// binding miss returns an empty binding and no error).
	if err := grants.GrantSpaceConnection(ctx, 7, "c-space", "bob", "alice"); err != nil {
		t.Fatal(err)
	}
	binding, err := authz.Check(ctx, bob, "c-space", 2)
	if err != nil {
		t.Fatalf("granted space connection must pass: %v", err)
	}
	if binding.ConnectionID != "" {
		t.Fatalf("native branch returns an empty binding, got %+v", binding)
	}

	// Revocation converges on the very next Check (no cached positive).
	if err := grants.RevokeSpaceConnection(ctx, 7, "c-space", "bob"); err != nil {
		t.Fatal(err)
	}
	_, err = authz.Check(ctx, bob, "c-space", 2)
	if !errors.Is(err, ErrConnectionForbidden) {
		t.Fatalf("revoked grant must stop resolving immediately, got %v", err)
	}
}

func TestSpaceGrantNeverOpensPersonalConnection(t *testing.T) {
	authz, grants, db := newDBGrantAuthorizer(t)
	seedOCSubjectFixture(t, db)
	ctx := context.Background()
	bob := appconn.OCSubject{TenantID: 7, ActorID: "bob"}

	// bob holds a grant row on the SPACE connection; it must not leak onto
	// alice's personal connection (the authorizer only reads grants for
	// Kind='space'; CanUseConnection's personal branch only admits the owner).
	if err := grants.GrantSpaceConnection(ctx, 7, "c-space", "bob", "alice"); err != nil {
		t.Fatal(err)
	}
	_, err := authz.Check(ctx, bob, "c-personal", 2)
	if !errors.Is(err, ErrConnectionForbidden) {
		t.Fatalf("AC1: 空间连接的 grant 不能替代个人连接的 owner 谓词, got %v", err)
	}

	// The owner still passes her own personal connection with NO grant row.
	alice := appconn.OCSubject{TenantID: 7, ActorID: "alice"}
	if _, err := authz.Check(ctx, alice, "c-personal", 2); err != nil {
		t.Fatalf("owner's personal connection must pass: %v", err)
	}
}

func TestSpaceGrantAuthorizerCrossTenantGrantUnreachable(t *testing.T) {
	authz, grants, db := newDBGrantAuthorizer(t)
	seedOCSubjectFixture(t, db)
	ctx := context.Background()

	// A grant written under another tenant id never admits tenant 7's check.
	if err := grants.GrantSpaceConnection(ctx, 8, "c-space", "bob", "alice"); err != nil {
		t.Fatal(err)
	}
	_, err := authz.Check(ctx, appconn.OCSubject{TenantID: 7, ActorID: "bob"}, "c-space", 2)
	if !errors.Is(err, ErrConnectionForbidden) {
		t.Fatalf("cross-tenant grant row must be unreachable, got %v", err)
	}
}
