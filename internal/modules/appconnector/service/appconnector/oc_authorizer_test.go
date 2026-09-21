package appconnector

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// countingCredentialSource decorates the REAL credential store so every test
// can assert that the authorization path never loads credential material.
type countingCredentialSource struct {
	ConnectionCredentialSource
	loads int32
}

func (c *countingCredentialSource) LoadCredential(ctx context.Context, conn appconn.Connection) ([]byte, error) {
	atomic.AddInt32(&c.loads, 1)
	return c.ConnectionCredentialSource.LoadCredential(ctx, conn)
}

func (c *countingCredentialSource) loadCount() int32 { return atomic.LoadInt32(&c.loads) }

// dbInstallationSource answers installation-active lookups from the same
// real database rows the repository persists (the dedicated store method
// arrives with the OC wiring tasks). A missing row fails closed.
type dbInstallationSource struct{ db *gorm.DB }

func (s *dbInstallationSource) InstallationActive(ctx context.Context, installationID string, tenantID uint64) (bool, error) {
	var row appconnectorrepo.InstallationRow
	err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", installationID, tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.State == appconn.InstallationActive, nil
}

// mapGrantSource models explicit per-actor space grants (no grant table is
// wired yet); a missing key means no grant, exactly like a missing row.
type mapGrantSource struct{ granted map[string]bool }

func (s *mapGrantSource) SpaceConnectionGranted(ctx context.Context, tenantID uint64, connectionID, actorID string) (bool, error) {
	return s.granted[fmt.Sprintf("%d/%s/%s", tenantID, connectionID, actorID)], nil
}

func (s *mapGrantSource) set(tenantID uint64, connectionID, actorID string) {
	if s.granted == nil {
		s.granted = map[string]bool{}
	}
	s.granted[fmt.Sprintf("%d/%s/%s", tenantID, connectionID, actorID)] = true
}

// failingBindingStore pins the T03 quality-review caller contract: a store
// that surfaces a RAW driver error (as concurrent losers may) and counts
// calls so tests can prove Check never retries internally.
type failingBindingStore struct {
	err   error
	calls int32
}

func (f *failingBindingStore) GetBinding(ctx context.Context, tenant uint64, connection string) (appconn.OCBinding, error) {
	atomic.AddInt32(&f.calls, 1)
	return appconn.OCBinding{}, f.err
}

// newAuthorizerFixture builds the OCAuthorizer over REAL stores on an
// in-memory SQLite database: the real credential/member/connection store,
// the real OC binding store, and db-backed installation/grant lookups.
func newAuthorizerFixture(t *testing.T) (appconn.OCAuthorizer, *countingCredentialSource, *mapGrantSource, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// Single pooled connection: shared-cache multi-connection access raises
	// SQLITE_LOCKED that busy_timeout does not retry (same convention as the
	// resolver suite).
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
		&appconnectorrepo.OCBindingRow{},
		&types.TenantMember{},
		&types.MCPOAuthToken{},
	); err != nil {
		t.Fatal(err)
	}
	src := &countingCredentialSource{ConnectionCredentialSource: apprepo.NewMCPOAuthBindingStore(db)}
	grants := &mapGrantSource{}
	authz := NewOCAuthorizer(src, &dbInstallationSource{db: db}, grants, appconnectorrepo.NewOCStore(db))
	return authz, src, grants, db
}

// seedOCSubjectFixture seeds tenant 7 with an active installation, owner
// alice and fellow member bob (never an owner), a live token secret, alice's
// active personal connection and alice's active space connection — both at
// auth_version 2.
func seedOCSubjectFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&appconnectorrepo.InstallationRow{
		ID: "inst-1", TenantID: 7, AppID: "mail", AppVersion: "1.0.0",
		State: appconn.InstallationActive, Version: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	seedMember(t, db, 7, "alice")
	seedMember(t, db, 7, "bob")
	seedToken(t, db, 7, "alice", "svc-1", "super-secret-ak")
	seedConnectionRow(t, db, appconn.Connection{
		ID: "c-personal", InstallationID: "inst-1", Kind: appconn.ConnectionKindPersonal,
		OwnerID: "alice", CredentialRef: apprepo.CredentialRefPrefix + "svc-1",
		State: appconn.ConnectionActive, TenantID: 7, AuthVersion: 2,
	})
	seedConnectionRow(t, db, appconn.Connection{
		ID: "c-space", InstallationID: "inst-1", Kind: appconn.ConnectionKindSpace,
		OwnerID: "alice", CredentialRef: apprepo.CredentialRefPrefix + "svc-1",
		State: appconn.ConnectionActive, TenantID: 7, AuthVersion: 2,
	})
}

func seedOCBinding(t *testing.T, db *gorm.DB, connectionID, state string, authVersion int64) appconn.OCBinding {
	t.Helper()
	b := appconn.OCBinding{
		TenantID: 7, ConnectionID: connectionID, RuntimeID: "rt-1", Provider: "github",
		ExternalID: "ext-" + connectionID, Alias: "alias-" + connectionID,
		AuthVersion: authVersion, BindingVersion: 1, State: state,
	}
	if err := appconnectorrepo.NewOCStore(db).SaveBinding(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	return b
}

// TestOCAuthorizerCheckRejectsForbiddenUse is the T04 table-driven RED/GREEN
// suite: every forbidden shape fails with its own sentinel AND never touches
// the secret store (load count 0). The plan's five mandatory rows come
// first: cross-actor personal connection, member-left, install-disabled,
// missing space grant, AuthVersion change.
func TestOCAuthorizerCheckRejectsForbiddenUse(t *testing.T) {
	alice := appconn.OCSubject{TenantID: 7, ActorID: "alice"}
	bob := appconn.OCSubject{TenantID: 7, ActorID: "bob"}
	cases := []struct {
		name            string
		mutate          func(t *testing.T, db *gorm.DB, grants *mapGrantSource)
		subject         appconn.OCSubject
		connectionID    string
		expectedVersion int64
		want            error
	}{
		{
			name:            "personal connection used by another member",
			subject:         bob, // bob is a LIVE member, but not the owner
			connectionID:    "c-personal",
			expectedVersion: 2,
			want:            ErrConnectionForbidden,
		},
		{
			name: "subject left the tenant",
			mutate: func(t *testing.T, db *gorm.DB, grants *mapGrantSource) {
				grants.set(7, "c-space", "bob") // even WITH a grant row
				if err := db.Where("tenant_id = ? AND user_id = ?", 7, "bob").
					Delete(&types.TenantMember{}).Error; err != nil {
					t.Fatal(err)
				}
			},
			subject:         bob,
			connectionID:    "c-space",
			expectedVersion: 2,
			want:            ErrSubjectNotMember,
		},
		{
			name: "installation disabled",
			mutate: func(t *testing.T, db *gorm.DB, grants *mapGrantSource) {
				if err := db.Model(&appconnectorrepo.InstallationRow{}).
					Where("id = ? AND tenant_id = ?", "inst-1", 7).
					Update("state", appconn.InstallationDisabled).Error; err != nil {
					t.Fatal(err)
				}
			},
			subject:         alice,
			connectionID:    "c-personal",
			expectedVersion: 2,
			want:            ErrInstallationNotActive,
		},
		{
			name:            "space connection without explicit grant",
			subject:         bob, // live member, no grant row
			connectionID:    "c-space",
			expectedVersion: 2,
			want:            ErrConnectionForbidden,
		},
		{
			name:            "auth version changed",
			subject:         alice, // connection sits at 2, caller believes 1
			connectionID:    "c-personal",
			expectedVersion: 1,
			want:            ErrConnectionVersionStale,
		},
		{
			name:            "cross-tenant subject",
			subject:         appconn.OCSubject{TenantID: 8, ActorID: "alice"},
			connectionID:    "c-personal",
			expectedVersion: 2,
			want:            ErrConnectionVersionStale,
		},
		{
			name:            "subject without tenant",
			subject:         appconn.OCSubject{TenantID: 0, ActorID: "alice"},
			connectionID:    "c-personal",
			expectedVersion: 2,
			want:            ErrMissingSubject,
		},
		{
			name:            "subject without actor",
			subject:         appconn.OCSubject{TenantID: 7, ActorID: ""},
			connectionID:    "c-personal",
			expectedVersion: 2,
			want:            ErrMissingSubject,
		},
		{
			name: "revoked connection",
			mutate: func(t *testing.T, db *gorm.DB, grants *mapGrantSource) {
				if err := db.Model(&appconnectorrepo.ConnectionRow{}).
					Where("tenant_id = ? AND id = ?", 7, "c-personal").
					Update("state", appconn.ConnectionRevoked).Error; err != nil {
					t.Fatal(err)
				}
			},
			subject:         alice,
			connectionID:    "c-personal",
			expectedVersion: 2,
			want:            ErrConnectionForbidden,
		},
		{
			name: "revoked OC binding",
			mutate: func(t *testing.T, db *gorm.DB, grants *mapGrantSource) {
				seedOCBinding(t, db, "c-personal", appconn.OCBindingRevoked, 2)
			},
			subject:         alice,
			connectionID:    "c-personal",
			expectedVersion: 2,
			want:            ErrConnectionRevoked,
		},
		{
			name: "pending OC binding does not authorize",
			mutate: func(t *testing.T, db *gorm.DB, grants *mapGrantSource) {
				seedOCBinding(t, db, "c-personal", appconn.OCBindingPending, 2)
			},
			subject:         alice,
			connectionID:    "c-personal",
			expectedVersion: 2,
			want:            ErrConnectionRevoked,
		},
		{
			name: "OC binding auth version behind the connection",
			mutate: func(t *testing.T, db *gorm.DB, grants *mapGrantSource) {
				seedOCBinding(t, db, "c-personal", appconn.OCBindingActive, 1)
			},
			subject:         alice,
			connectionID:    "c-personal",
			expectedVersion: 2,
			want:            ErrConnectionVersionStale,
		},
		{
			name:            "unknown connection id passes the store error through",
			subject:         alice,
			connectionID:    "nope",
			expectedVersion: 2,
			want:            gorm.ErrRecordNotFound,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			authz, src, grants, db := newAuthorizerFixture(t)
			seedOCSubjectFixture(t, db)
			if tc.mutate != nil {
				tc.mutate(t, db, grants)
			}
			binding, err := authz.Check(context.Background(), tc.subject, tc.connectionID, tc.expectedVersion)
			if err == nil {
				t.Fatalf("forbidden use authorized: binding %+v", binding)
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if n := src.loadCount(); n != 0 {
				t.Fatalf("authorization loaded credentials %d times, want 0", n)
			}
		})
	}
}

// TestOCAuthorizerCheckAllowsAuthorizedUse pins the two allow shapes: an
// open-connector use returns the ACTIVE tenant-scoped binding that
// authorized it, and a native use (no binding row) returns a zero binding —
// both without ever touching the secret store.
func TestOCAuthorizerCheckAllowsAuthorizedUse(t *testing.T) {
	t.Run("owner with active binding gets the binding back", func(t *testing.T) {
		authz, src, _, db := newAuthorizerFixture(t)
		seedOCSubjectFixture(t, db)
		want := seedOCBinding(t, db, "c-personal", appconn.OCBindingActive, 2)

		got, err := authz.Check(context.Background(),
			appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", 2)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("binding = %+v, want %+v", got, want)
		}
		if n := src.loadCount(); n != 0 {
			t.Fatalf("authorization loaded credentials %d times, want 0", n)
		}
	})
	t.Run("granted member on native space connection", func(t *testing.T) {
		authz, src, grants, db := newAuthorizerFixture(t)
		seedOCSubjectFixture(t, db)
		grants.set(7, "c-space", "bob")

		got, err := authz.Check(context.Background(),
			appconn.OCSubject{TenantID: 7, ActorID: "bob"}, "c-space", 2)
		if err != nil {
			t.Fatal(err)
		}
		if (got != appconn.OCBinding{}) {
			t.Fatalf("native check returned a binding: %+v", got)
		}
		if n := src.loadCount(); n != 0 {
			t.Fatalf("authorization loaded credentials %d times, want 0", n)
		}
	})
}

// TestOCAuthorizerCheckTreatsBindingStoreErrorAsFailure carries QR-F1/QR-F2
// from the T03 quality review: ANY non-nil binding-store error — including a
// RAW driver error a concurrent loser may see — is a failure. Check maps no
// driver errors into authorization outcomes and retries nothing internally.
func TestOCAuthorizerCheckTreatsBindingStoreErrorAsFailure(t *testing.T) {
	_, src, _, db := newAuthorizerFixture(t)
	seedOCSubjectFixture(t, db)
	raw := errors.New("database table is locked")
	store := &failingBindingStore{err: raw}
	full := NewOCAuthorizer(src, nil, nil, store)

	if _, err := full.Check(context.Background(),
		appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", 2); !errors.Is(err, raw) {
		t.Fatalf("binding store failure was rewritten or authorized: %v", err)
	}
	if got := atomic.LoadInt32(&store.calls); got != 1 {
		t.Fatalf("binding lookups = %d, want exactly 1 (no internal retry)", got)
	}
	if n := src.loadCount(); n != 0 {
		t.Fatalf("authorization loaded credentials %d times, want 0", n)
	}
}

// TestPersonalConnectionCannotCrossActor pins the domain predicate the
// service chain reuses (plan sketch): personal connections are owner-only
// regardless of any space grant.
func TestPersonalConnectionCannotCrossActor(t *testing.T) {
	c := appconn.Connection{ID: "c", TenantID: 1, Kind: appconn.ConnectionKindPersonal,
		OwnerID: "alice", State: appconn.ConnectionActive}
	if appconn.CanUseConnection(c, 1, "bob", false) {
		t.Fatal("personal connection shared")
	}
}
