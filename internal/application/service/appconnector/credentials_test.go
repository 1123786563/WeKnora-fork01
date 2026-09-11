package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newResolverFixture builds the resolver over the REAL repository store on
// an in-memory SQLite database, so the guard chain is exercised against the
// actual persistence semantics rather than a stub.
func newResolverFixture(t *testing.T) (*credentialResolver, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// Single pooled connection, mirroring commercial/budget_test.go: SQLite
	// is single-writer anyway, and shared-cache multi-connection access
	// raises SQLITE_LOCKED ("database table is locked") that busy_timeout
	// does not retry. The guarded UPDATE — never a process mutex — still
	// decides the single lease winner.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&apprepo.MCPOAuthBindingRow{},
		&appconnectorrepo.AppVersion{},
		&appconnectorrepo.InstallationRow{},
		&appconnectorrepo.ConnectionRow{},
		&types.TenantMember{},
		&types.MCPOAuthToken{},
	); err != nil {
		t.Fatal(err)
	}
	store := apprepo.NewMCPOAuthBindingStore(db)
	return &credentialResolver{src: store}, db
}

func seedMember(t *testing.T, db *gorm.DB, tenant uint64, user string) {
	t.Helper()
	if err := db.Create(&types.TenantMember{
		UserID: user, TenantID: tenant, Role: types.TenantRoleContributor,
		Status: types.TenantMemberStatusActive, JoinedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func seedToken(t *testing.T, db *gorm.DB, tenant uint64, user, serviceID, secret string) {
	t.Helper()
	if err := db.Create(&types.MCPOAuthToken{
		TenantID: tenant, UserID: user, PrincipalType: types.PrincipalWebUser,
		PrincipalID: user, ServiceID: serviceID, AccessToken: secret,
		TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func seedConnectionRow(t *testing.T, db *gorm.DB, c appconn.Connection) {
	t.Helper()
	if err := db.Create(&appconnectorrepo.ConnectionRow{
		TenantID: c.TenantID, ID: c.ID, InstallationID: c.InstallationID, Kind: c.Kind,
		OwnerID: c.OwnerID, CredentialRef: c.CredentialRef, State: c.State, AuthVersion: c.AuthVersion,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func activeConnection(version int64) appconn.Connection {
	return appconn.Connection{
		ID: "c1", InstallationID: "inst-1", Kind: appconn.ConnectionKindPersonal,
		OwnerID: "u1", CredentialRef: apprepo.CredentialRefPrefix + "svc-1",
		State: appconn.ConnectionActive, TenantID: 7, AuthVersion: version,
	}
}

// TestOAuthCredentialResolveValidReturnsSecret pins the happy path: an
// active, version-matched, member-owned connection resolves to the decrypted
// secret for the calling adapter.
func TestOAuthCredentialResolveValidReturnsSecret(t *testing.T) {
	r, db := newResolverFixture(t)
	seedMember(t, db, 7, "u1")
	seedToken(t, db, 7, "u1", "svc-1", "super-secret-ak")
	seedConnectionRow(t, db, activeConnection(3))

	secret, err := r.Resolve(context.Background(), "c1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if string(secret) != "super-secret-ak" {
		t.Fatalf("unexpected secret %q", string(secret))
	}
}

// TestOAuthCredentialResolveRevokedNeverResolves pins that a revoked
// connection never resolves and never refreshes.
func TestOAuthCredentialResolveRevokedNeverResolves(t *testing.T) {
	r, db := newResolverFixture(t)
	seedMember(t, db, 7, "u1")
	seedToken(t, db, 7, "u1", "svc-1", "super-secret-ak")
	c := activeConnection(2)
	c.State = appconn.ConnectionRevoked
	seedConnectionRow(t, db, c)

	if _, err := r.Resolve(context.Background(), "c1", 2); !errors.Is(err, ErrConnectionRevoked) {
		t.Fatalf("revoked connection resolved: %v", err)
	}
	won, err := r.RefreshLeaseGuarded(context.Background(), "c1", 2, "lease-1", time.Now().Add(time.Minute))
	if won || !errors.Is(err, ErrConnectionRevoked) {
		t.Fatalf("revoked connection attempted refresh: won=%v err=%v", won, err)
	}
}

// TestOAuthCredentialResolveStaleVersionRejected pins the auth_version check.
func TestOAuthCredentialResolveStaleVersionRejected(t *testing.T) {
	r, db := newResolverFixture(t)
	seedMember(t, db, 7, "u1")
	seedToken(t, db, 7, "u1", "svc-1", "super-secret-ak")
	seedConnectionRow(t, db, activeConnection(2))

	if _, err := r.Resolve(context.Background(), "c1", 1); !errors.Is(err, ErrConnectionVersionStale) {
		t.Fatalf("stale version resolved: %v", err)
	}
}

// TestOAuthCredentialResolveOwnerRemovedRejected pins the pre-dispatch
// membership check: once the owner loses membership the connection is dead.
func TestOAuthCredentialResolveOwnerRemovedRejected(t *testing.T) {
	r, db := newResolverFixture(t)
	seedToken(t, db, 7, "u1", "svc-1", "super-secret-ak")
	seedConnectionRow(t, db, activeConnection(1))

	if _, err := r.Resolve(context.Background(), "c1", 1); !errors.Is(err, ErrConnectionOwnerNotMember) {
		t.Fatalf("removed owner's connection resolved: %v", err)
	}
}

// TestOAuthCredentialRefreshLeaseSingleWinner pins that concurrent guarded
// refreshes yield exactly one lease winner via the EXISTING lease.
func TestOAuthCredentialRefreshLeaseSingleWinner(t *testing.T) {
	r, db := newResolverFixture(t)
	seedMember(t, db, 7, "u1")
	seedToken(t, db, 7, "u1", "svc-1", "super-secret-ak")
	seedConnectionRow(t, db, activeConnection(1))

	const workers = 8
	var wg sync.WaitGroup
	wins := make(chan bool, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			won, err := r.RefreshLeaseGuarded(context.Background(), "c1", 1,
				"lease-"+string(rune('a'+i)), time.Now().Add(time.Minute))
			if err != nil {
				t.Errorf("guarded refresh: %v", err)
			}
			wins <- won
		}(i)
	}
	wg.Wait()
	close(wins)
	winners := 0
	for w := range wins {
		if w {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("guarded refresh had %d winners, want exactly 1", winners)
	}
}

// TestOAuthCredentialResolveOutputRedacted pins that the resolved credential
// never appears in any serialized response or model-context shape: the safe
// view and every sentinel error marshal without the secret, and the
// CredentialResolver return value (raw bytes for the adapter only) is the
// only place the credential exists.
func TestOAuthCredentialResolveOutputRedacted(t *testing.T) {
	r, db := newResolverFixture(t)
	seedMember(t, db, 7, "u1")
	const secret = "super-secret-ak"
	seedToken(t, db, 7, "u1", "svc-1", secret)
	conn := activeConnection(1)
	seedConnectionRow(t, db, conn)

	got, err := r.Resolve(context.Background(), "c1", 1)
	if err != nil || string(got) != secret {
		t.Fatalf("resolve: %q %v", string(got), err)
	}

	// The redacted view marshals without the secret AND without the
	// credential reference.
	viewJSON, err := json.Marshal(SafeConnectionView(conn))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(viewJSON), secret) {
		t.Fatalf("view leaked credential: %s", viewJSON)
	}
	if strings.Contains(string(viewJSON), conn.CredentialRef) {
		t.Fatalf("view leaked credential reference: %s", viewJSON)
	}
	if strings.Contains(strings.ToLower(string(viewJSON)), "credential") {
		t.Fatalf("view mentions credential fields: %s", viewJSON)
	}

	// No sentinel error text carries the secret, even marshaled.
	for _, e := range []error{ErrConnectionRevoked, ErrConnectionVersionStale, ErrConnectionOwnerNotMember} {
		if strings.Contains(e.Error(), secret) {
			t.Fatalf("error %q leaked credential", e.Error())
		}
		blob, _ := json.Marshal(map[string]string{"error": e.Error()})
		if strings.Contains(string(blob), secret) {
			t.Fatalf("marshaled error leaked credential: %s", blob)
		}
	}
}
