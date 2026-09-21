// Package appconnector_test hosts T08's revocation acceptance suite as an
// EXTERNAL test package: the control-worker tests must import
// internal/connectorcontrol, which itself imports the service package — an
// internal test file would form an import cycle. Everything here drives the
// EXPORTED surface of the service, repository and control-worker packages.
package appconnector_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/application/service/appconnector"
	cc "github.com/Tencent/WeKnora/internal/connectorcontrol"
	"github.com/Tencent/WeKnora/internal/types"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// local twins of the internal sqlite helpers (an external test package
// cannot reuse them). Bodies follow action_test.go exactly.
// ---------------------------------------------------------------------------

func memDSN(t *testing.T) string {
	t.Helper()
	return "file:" + t.Name() + "?mode=memory&cache=shared&_busy_timeout=5000"
}

func openActionDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// Single connection serializes transactions (shared-cache SQLITE_LOCKED
	// guard, same convention as the A02 suite).
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&repoappconn.ActionRow{}, &repoappconn.ApprovalRow{}, &repoappconn.PreAuthorizationRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// ---------------------------------------------------------------------------
// T08: revocation core, permission epochs, and the four carried
// orphan/re-bind remediations (coordinator ruling 1).
// ---------------------------------------------------------------------------

// TestOCRevokeVersionGuard is the plan's VERBATIM conditional-update guard:
// a stale revoke (wrong expected version) must change ZERO rows.
func TestOCRevokeVersionGuard(t *testing.T) {
	db := openActionDB(t, memDSN(t))
	if err := db.AutoMigrate(&repoappconn.ConnectionRow{}); err != nil {
		t.Fatal(err)
	}
	row := repoappconn.ConnectionRow{ID: "c", TenantID: 1, State: "active", AuthVersion: 2}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	res := db.Model(&row).Where("auth_version = ?", 1).Update("state", "revoked")
	if res.Error != nil || res.RowsAffected != 0 {
		t.Fatal("stale revoke changed connection")
	}
}

// ---------------------------------------------------------------------------
// authorizer fixture over real rows (exported surface only)
// ---------------------------------------------------------------------------

type revokeCredSource struct{ db *gorm.DB }

func (s *revokeCredSource) FindConnectionByID(ctx context.Context, connectionID string) (appconn.Connection, error) {
	var row repoappconn.ConnectionRow
	if err := s.db.WithContext(ctx).Where("id = ?", connectionID).First(&row).Error; err != nil {
		return appconn.Connection{}, err
	}
	return appconn.Connection{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind, OwnerID: row.OwnerID,
		CredentialRef: row.CredentialRef, State: row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}

func (s *revokeCredSource) LoadCredential(ctx context.Context, c appconn.Connection) ([]byte, error) {
	return nil, errors.New("authorization must never load credentials")
}

func (s *revokeCredSource) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	var n int64
	if err := s.db.WithContext(ctx).Model(&types.TenantMember{}).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, types.TenantMemberStatusActive).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *revokeCredSource) TryAcquireRefreshLease(ctx context.Context, c appconn.Connection, leaseID string, until time.Time) (bool, error) {
	return false, nil
}

type revokeInstallSource struct{ db *gorm.DB }

func (s *revokeInstallSource) InstallationActive(ctx context.Context, installationID string, tenantID uint64) (bool, error) {
	var row repoappconn.InstallationRow
	err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", installationID, tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.State == appconn.InstallationActive, nil
}

type revokeGrantSource struct{ granted map[string]bool }

func (s *revokeGrantSource) SpaceConnectionGranted(ctx context.Context, tenantID uint64, connectionID, actorID string) (bool, error) {
	return s.granted[fmt.Sprintf("%d/%s/%s", tenantID, connectionID, actorID)], nil
}

// openRevokeAuthzDB builds the authorizer's real-row database.
func openRevokeAuthzDB(t *testing.T) (*gorm.DB, appconn.OCAuthorizer) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(memDSN(t)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&repoappconn.InstallationRow{}, &repoappconn.ConnectionRow{}, &repoappconn.OCBindingRow{},
		&repoappconn.OCOperationsOutboxRow{}, &types.TenantMember{}); err != nil {
		t.Fatal(err)
	}
	authz := appconnectorsvc.NewOCAuthorizer(&revokeCredSource{db: db}, &revokeInstallSource{db: db}, &revokeGrantSource{}, repoappconn.NewOCStore(db))
	return db, authz
}

// seedRevokeFixture seeds tenant 7: active installation, alice as member, and
// alice's active personal connection at auth_version 2.
func seedRevokeFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&repoappconn.InstallationRow{
		ID: "inst-1", TenantID: 7, AppID: "mail", AppVersion: "1.0.0",
		State: appconn.InstallationActive, Version: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&types.TenantMember{TenantID: 7, UserID: "alice", Status: types.TenantMemberStatusActive}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repoappconn.ConnectionRow{
		TenantID: 7, ID: "c-personal", InstallationID: "inst-1", Kind: appconn.ConnectionKindPersonal,
		OwnerID: "alice", State: appconn.ConnectionActive, AuthVersion: 2,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func seedRevokeBinding(t *testing.T, db *gorm.DB, connID, state string, authVersion int64) {
	t.Helper()
	b := appconn.OCBinding{
		TenantID: 7, ConnectionID: connID, RuntimeID: "rt-1", Provider: "github",
		ExternalID: "ext-" + connID, Alias: "alias-" + connID,
		AuthVersion: authVersion, BindingVersion: 1, State: state,
	}
	if err := repoappconn.NewOCStore(db).SaveBinding(context.Background(), b); err != nil {
		t.Fatal(err)
	}
}

// TestOCRevokeRejectsNewRequestsWhileWorkerUnavailable (plan checkbox 1): the
// revocation TRANSACTION is the authority. After RevokeOCConnection commits,
// the authorizer rejects new requests on every path with NO worker involved
// — an unavailable control worker never re-opens a revoked connection.
func TestOCRevokeRejectsNewRequestsWhileWorkerUnavailable(t *testing.T) {
	db, authz := openRevokeAuthzDB(t)
	seedRevokeFixture(t, db)
	seedRevokeBinding(t, db, "c-personal", appconn.OCBindingActive, 2)
	subject := appconn.OCSubject{TenantID: 7, ActorID: "alice"}
	ctx := context.Background()

	// Pre-revoke: the live chain authorizes and returns the binding.
	binding, err := authz.Check(ctx, subject, "c-personal", 2)
	if err != nil || binding.ExternalID != "ext-c-personal" {
		t.Fatalf("pre-revoke Check = %+v %v", binding, err)
	}

	if err := repoappconn.NewOCStore(db).RevokeOCConnection(ctx, subject, "c-personal", 2); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// A caller still holding the pre-revoke generation is stale.
	if _, err := authz.Check(ctx, subject, "c-personal", 2); !errors.Is(err, appconnectorsvc.ErrConnectionVersionStale) {
		t.Fatalf("stale-generation Check err = %v, want ErrConnectionVersionStale", err)
	}
	// A caller that re-read the new generation sees the revoked connection.
	if _, err := authz.Check(ctx, subject, "c-personal", 3); !errors.Is(err, appconnectorsvc.ErrConnectionForbidden) {
		t.Fatalf("fresh-generation Check err = %v, want ErrConnectionForbidden (revoked connection)", err)
	}
	// The mirrored binding row is revoked too: every path rejects.
	b, err := repoappconn.NewOCStore(db).GetBinding(ctx, 7, "c-personal")
	if err != nil || b.State != appconn.OCBindingRevoked || b.AuthVersion != 3 {
		t.Fatalf("binding after revoke = %+v err=%v, want revoked@3", b, err)
	}
}

// TestOCRevokePermissionEpochInvalidatedOnEveryCheck (ruling 4): no cached
// positive authorization exists — the very next Check after an epoch change
// (installation disabled, member removal, binding revocation) fails.
func TestOCRevokePermissionEpochInvalidatedOnEveryCheck(t *testing.T) {
	firstCheckPasses := func(t *testing.T, db *gorm.DB, authz appconn.OCAuthorizer) {
		t.Helper()
		if _, err := authz.Check(context.Background(), appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", 2); err != nil {
			t.Fatalf("first Check must pass: %v", err)
		}
	}
	t.Run("installation disabled between checks", func(t *testing.T) {
		db, authz := openRevokeAuthzDB(t)
		seedRevokeFixture(t, db)
		seedRevokeBinding(t, db, "c-personal", appconn.OCBindingActive, 2)
		firstCheckPasses(t, db, authz)
		if err := db.Model(&repoappconn.InstallationRow{}).
			Where("id = ? AND tenant_id = ?", "inst-1", 7).
			Update("state", appconn.InstallationDisabled).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := authz.Check(context.Background(), appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", 2); !errors.Is(err, appconnectorsvc.ErrInstallationNotActive) {
			t.Fatalf("post-disable Check err = %v, want ErrInstallationNotActive", err)
		}
	})
	t.Run("member removed between checks", func(t *testing.T) {
		db, authz := openRevokeAuthzDB(t)
		seedRevokeFixture(t, db)
		seedRevokeBinding(t, db, "c-personal", appconn.OCBindingActive, 2)
		firstCheckPasses(t, db, authz)
		if err := db.Exec("DELETE FROM tenant_members WHERE tenant_id = ? AND user_id = ?", 7, "alice").Error; err != nil {
			t.Fatal(err)
		}
		if _, err := authz.Check(context.Background(), appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", 2); !errors.Is(err, appconnectorsvc.ErrSubjectNotMember) {
			t.Fatalf("post-removal Check err = %v, want ErrSubjectNotMember", err)
		}
	})
	t.Run("binding revoked between checks", func(t *testing.T) {
		db, authz := openRevokeAuthzDB(t)
		seedRevokeFixture(t, db)
		seedRevokeBinding(t, db, "c-personal", appconn.OCBindingActive, 2)
		firstCheckPasses(t, db, authz)
		if err := repoappconn.NewOCStore(db).RevokeOCConnection(context.Background(), appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", 2); err != nil {
			t.Fatal(err)
		}
		if _, err := authz.Check(context.Background(), appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", 2); !errors.Is(err, appconnectorsvc.ErrConnectionVersionStale) {
			t.Fatalf("post-revoke Check err = %v, want ErrConnectionVersionStale", err)
		}
	})
}

// ---------------------------------------------------------------------------
// ruling 1c: persistence-then-enqueue-failure leaves no silent stuck attempt
// ---------------------------------------------------------------------------

// openRevokeServiceDB builds the Begin/Confirm service over real rows.
func openRevokeServiceDB(t *testing.T) (*appconnectorsvc.OCConnectionService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(memDSN(t)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&repoappconn.AppVersion{}, &repoappconn.InstallationRow{}, &repoappconn.ConnectionRow{},
		&repoappconn.OCBindingRow{}, &repoappconn.OCAuthorizationAttemptRow{},
		&repoappconn.OCOperationsOutboxRow{}, &repoappconn.OCRuntimeRow{}, &types.TenantMember{}); err != nil {
		t.Fatal(err)
	}
	seedRevokeFixture(t, db)
	if err := db.Create(&repoappconn.OCRuntimeRow{ID: "rt-1", Address: "http://open-connector:8080", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	svc := appconnectorsvc.NewOCConnectionService(&revokeCredSource{db: db}, &revokeInstallLookup{db: db}, repoappconn.NewOCStore(db))
	return svc, db
}

type revokeInstallLookup struct{ db *gorm.DB }

func (l *revokeInstallLookup) GetInstallationByID(ctx context.Context, tenant uint64, installationID string) (appconn.Installation, error) {
	var row repoappconn.InstallationRow
	if err := l.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenant, installationID).First(&row).Error; err != nil {
		return appconn.Installation{}, err
	}
	return appconn.Installation{ID: row.ID, AppID: row.AppID, Version: row.AppVersion, State: row.State, TenantID: row.TenantID}, nil
}

// TestOCRevokeBeginEnqueueFailureFailsAttempt: when Begin persists the attempt
// but the authorize enqueue fails, the attempt transitions to failed instead
// of lingering in pending with no operation row (T07 spec F-04).
func TestOCRevokeBeginEnqueueFailureFailsAttempt(t *testing.T) {
	svc, db := openRevokeServiceDB(t)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()

	if err := db.Exec("DROP TABLE connector_operations_outbox").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", now); err == nil {
		t.Fatal("Begin must surface the enqueue failure")
	}
	var state string
	if err := db.Raw("SELECT state FROM connector_authorization_attempts WHERE tenant_id = ? AND connection_id = ?", 7, "c-personal").Row().Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != appconnectorsvc.OCAttemptFailed {
		t.Fatalf("attempt state after enqueue failure = %q, want %q (no silent stuck-pending)", state, appconnectorsvc.OCAttemptFailed)
	}
}

// TestOCRevokeConfirmEnqueueFailureFailsAttempt: same contract on the Confirm
// leg — the authorizing->verifying transition committed, the confirm enqueue
// failed, so the attempt terminates failed instead of staying verifying with
// no driver row.
func TestOCRevokeConfirmEnqueueFailureFailsAttempt(t *testing.T) {
	svc, db := openRevokeServiceDB(t)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()

	attemptID, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", now)
	if err != nil {
		t.Fatal(err)
	}
	store := repoappconn.NewOCStore(db)
	if ok, err := store.AdvanceOCAttempt(ctx, 7, attemptID, appconnectorsvc.OCAttemptPending, appconnectorsvc.OCAttemptAuthorizing, now); err != nil || !ok {
		t.Fatalf("advance: ok=%v err=%v", ok, err)
	}
	if err := db.Exec("DROP TABLE connector_operations_outbox").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmAt(ctx, attemptID, now.Add(time.Minute)); err == nil {
		t.Fatal("Confirm must surface the enqueue failure")
	}
	a, err := store.GetOCAttempt(ctx, 7, attemptID)
	if err != nil || a.State != appconnectorsvc.OCAttemptFailed {
		t.Fatalf("attempt state after confirm enqueue failure = %+v err=%v, want failed", a, err)
	}
}

// ---------------------------------------------------------------------------
// control-worker surface for the revocation matrix and rulings 1a/1b/1d.
// ---------------------------------------------------------------------------

// ocRevokeWorkerDB mirrors the T07 worker fixture convention: file-backed
// sqlite with the REAL production migration applied.
func ocRevokeWorkerDB(t *testing.T) *gorm.DB {
	t.Helper()
	raw, err := os.ReadFile("../../../../migrations/sqlite/000041_open_connector_bindings.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	dsn := filepath.Join(t.TempDir(), "oc-revoke.db") + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=1&_txlock=immediate"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:  gormlogger.Discard,
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&repoappconn.AppVersion{}, &repoappconn.InstallationRow{}, &repoappconn.ConnectionRow{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(raw)).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func ocRevokeSeedConnection(t *testing.T, db *gorm.DB, tenant uint64, connID string, authVersion int64) {
	t.Helper()
	instID := fmt.Sprintf("inst-%d", tenant)
	db.Exec("INSERT OR IGNORE INTO app_versions (app_id, version) VALUES ('app-oc','1.0.0')")
	db.Exec("INSERT OR IGNORE INTO installations (id, tenant_id, app_id, app_version, state) VALUES (?,?,'app-oc','1.0.0','active')", instID, tenant)
	if err := db.Exec("INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version) VALUES (?,?,?,?, 'owner', 'ref/cred', 'active', ?)",
		tenant, connID, instID, appconn.ConnectionKindSpace, authVersion).Error; err != nil {
		t.Fatal(err)
	}
}

func ocRevokeSeedBinding(t *testing.T, db *gorm.DB, tenant uint64, connID, externalID, alias, state string, authVersion int64) {
	t.Helper()
	b := appconn.OCBinding{
		TenantID: tenant, ConnectionID: connID, RuntimeID: "rt-1", Provider: "app-oc",
		ExternalID: externalID, Alias: alias,
		AuthVersion: authVersion, BindingVersion: 1, State: state,
	}
	if err := repoappconn.NewOCStore(db).SaveBinding(context.Background(), b); err != nil {
		t.Fatal(err)
	}
}

func ocRevokeSeedOp(t *testing.T, db *gorm.DB, id, kind string, tenant uint64, resource string, version int64) {
	t.Helper()
	if err := db.Exec("INSERT INTO connector_operations_outbox (id, tenant_id, resource_id, resource_version, kind, attempts, next_at) VALUES (?,?,?,?,?,0,?)",
		id, tenant, resource, version, kind, time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
}

func ocRevokeOutboxCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Raw("SELECT COUNT(*) FROM connector_operations_outbox").Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// ocRevokeAdmin is a local RuntimeAdmin fake with injectable delete behavior.
type ocRevokeAdmin struct {
	mu          sync.Mutex
	deleteErr   error
	deleteCalls []string
	revokeCalls []string
}

func (a *ocRevokeAdmin) CreateRuntimeToken(ctx context.Context, req cc.CreateTokenRequest) (cc.RuntimeTokenCreated, error) {
	return cc.RuntimeTokenCreated{TokenRecordID: "rec-1", Token: "tok-1"}, nil
}

func (a *ocRevokeAdmin) RevokeRuntimeToken(ctx context.Context, tokenRecordID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revokeCalls = append(a.revokeCalls, tokenRecordID)
	return a.deleteErr
}

func (a *ocRevokeAdmin) StartAuthorization(ctx context.Context, service, connectionName string) (cc.AuthorizationStart, error) {
	return cc.AuthorizationStart{URL: "https://ext.example/auth", State: "st-1"}, nil
}

func (a *ocRevokeAdmin) LookupRuntimeConnection(ctx context.Context, appID string) (cc.RuntimeConnection, error) {
	return cc.RuntimeConnection{ID: appID, Alias: "alias-1", ProviderAccountID: "acct-1"}, nil
}

func (a *ocRevokeAdmin) DeleteRuntimeConnection(ctx context.Context, appID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deleteCalls = append(a.deleteCalls, appID)
	return a.deleteErr
}

func (a *ocRevokeAdmin) deleted() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.deleteCalls...)
}

// ocRevokeCorrelator is a local correlator fake; onSubmit lets a test
// sabotage local state MID-FLIGHT (after the external submit succeeded) to
// exercise the post-submit failure windows.
type ocRevokeCorrelator struct {
	mu         sync.Mutex
	resolves   int
	submits    int
	resolve    cc.RuntimeConnection
	resolveErr error
	submit     cc.RuntimeConnection
	onSubmit   func()
}

func (c *ocRevokeCorrelator) ResolveExternalConnection(ctx context.Context, service, alias string) (cc.RuntimeConnection, error) {
	c.mu.Lock()
	c.resolves++
	c.mu.Unlock()
	if c.resolveErr != nil {
		return cc.RuntimeConnection{}, c.resolveErr
	}
	return c.resolve, nil
}

func (c *ocRevokeCorrelator) SubmitExternalCredential(ctx context.Context, service, alias, apiKey string) (cc.RuntimeConnection, error) {
	c.mu.Lock()
	c.submits++
	hook := c.onSubmit
	c.mu.Unlock()
	if hook != nil {
		hook()
	}
	return c.submit, nil
}

func (c *ocRevokeCorrelator) counts() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.resolves, c.submits
}

type ocRevokeSink struct{}

func (s *ocRevokeSink) PutSecret(ctx context.Context, ref, secret string) error { return nil }

func ocRevokeNewWorker(t *testing.T, db *gorm.DB, admin cc.RuntimeAdmin) *cc.ControlWorker {
	t.Helper()
	store := repoappconn.NewOCStore(db)
	w, err := cc.NewControlWorker(store, store, admin, &ocRevokeSink{}, func(context.Context) (string, error) {
		return "admin-secret", nil
	}, cc.DefaultWorkerConfig("t08-worker"))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// TestOCRevokeRemoteDeleteFailureKeepsCleanupQueued (matrix: remote-delete
// failure AND provider-without-revoke): when the remote delete fails —
// transport error or the unpinned endpoint (provider without revoke support)
// — the durable cleanup row STAYS queued for retry and the local state shows
// ONLY "locally disconnected": connection and binding revoked, version
// bumped, nothing re-opened.
func TestOCRevokeRemoteDeleteFailureKeepsCleanupQueued(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"transport failure", errors.New("connectorcontrol: admin transport: connection refused")},
		{"provider without revoke support", cc.ErrUnpinnedEndpoint},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := ocRevokeWorkerDB(t)
			ctx := context.Background()
			ocRevokeSeedConnection(t, db, 7, "conn1", 1)
			ocRevokeSeedBinding(t, db, 7, "conn1", "ext-42", "alias-1", appconn.OCBindingActive, 1)

			if err := repoappconn.NewOCStore(db).RevokeOCConnection(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "conn1", 1); err != nil {
				t.Fatal(err)
			}
			admin := &ocRevokeAdmin{deleteErr: tc.err}
			w := ocRevokeNewWorker(t, db, admin)
			if err := w.RunOnce(ctx); err == nil {
				t.Fatal("remote delete failure must surface")
			}
			if n := ocRevokeOutboxCount(t, db); n != 1 {
				t.Fatalf("cleanup rows after failed delete = %d, want 1 (durable, retried)", n)
			}
			var state string
			var version int64
			if err := db.Raw("SELECT state, auth_version FROM connections WHERE tenant_id = ? AND id = ?", 7, "conn1").Row().Scan(&state, &version); err != nil {
				t.Fatal(err)
			}
			if state != "revoked" || version != 2 {
				t.Fatalf("local state = %s@%d, want revoked@2 (locally disconnected regardless of remote)", state, version)
			}
			b, err := repoappconn.NewOCStore(db).GetBinding(ctx, 7, "conn1")
			if err != nil || b.State != appconn.OCBindingRevoked {
				t.Fatalf("binding = %+v err=%v, want revoked", b, err)
			}
		})
	}
}

// TestOCRevokeCleanupTreatsLostDeleteResponseAsSuccess (matrix: token deleted
// but response lost): an admin 404 on the delete path is idempotent success —
// the cleanup row completes instead of retrying forever.
func TestOCRevokeCleanupTreatsLostDeleteResponseAsSuccess(t *testing.T) {
	db := ocRevokeWorkerDB(t)
	ctx := context.Background()
	ocRevokeSeedConnection(t, db, 7, "conn1", 1)
	ocRevokeSeedBinding(t, db, 7, "conn1", "ext-42", "alias-1", appconn.OCBindingActive, 1)

	if err := repoappconn.NewOCStore(db).RevokeOCConnection(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "conn1", 1); err != nil {
		t.Fatal(err)
	}
	admin := &ocRevokeAdmin{deleteErr: &cc.AdminError{Status: 404, Code: "not_found"}}
	w := ocRevokeNewWorker(t, db, admin)
	if err := w.RunOnce(ctx); err != nil {
		t.Fatalf("lost-response delete must complete idempotently: %v", err)
	}
	if n := ocRevokeOutboxCount(t, db); n != 0 {
		t.Fatalf("cleanup rows after 404 delete = %d, want 0 (completed)", n)
	}
	if got := admin.deleted(); len(got) != 1 || got[0] != "ext-42" {
		t.Fatalf("delete calls = %v, want exactly [ext-42]", got)
	}
}

// TestOCRevokeAPIKeyHandoffVerifyingRetryCompletesWithoutResubmit (ruling 1a,
// T07 Q-3/F-01): an API-key authorize row retried after the attempt already
// reached verifying (external connection adopted) must COMPLETE through the
// correlation — no permanent drop, no orphaned attempt, no second key
// submission.
func TestOCRevokeAPIKeyHandoffVerifyingRetryCompletesWithoutResubmit(t *testing.T) {
	db := ocRevokeWorkerDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ocRevokeSeedConnection(t, db, 7, "conn1", 1)

	a, err := appconnectorsvc.NewOCAttempt(appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "conn1", "rt-1", "app-oc", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	store := repoappconn.NewOCStore(db)
	if err := store.SaveOCAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.AdvanceOCAttempt(ctx, 7, a.ID, appconnectorsvc.OCAttemptPending, appconnectorsvc.OCAttemptAuthorizing, now); err != nil || !ok {
		t.Fatalf("advance: %v", err)
	}
	if ok, err := store.AdoptOCAttemptAlias(ctx, 7, a.ID, a.Alias, "adopted-alias", now); err != nil || !ok {
		t.Fatalf("adopt: %v", err)
	}
	if ok, err := store.AdvanceOCAttempt(ctx, 7, a.ID, appconnectorsvc.OCAttemptAuthorizing, appconnectorsvc.OCAttemptVerifying, now); err != nil || !ok {
		t.Fatalf("advance to verifying: %v", err)
	}

	cipher, err := appconnectorsvc.NewTransientCipherFromKey("t08-test-key")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := cipher.Seal("ghp_plain_secret", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	ocRevokeSeedOp(t, db, "op-key-1", "authorize", 7, a.ID+"|"+sealed, 1)

	fc := &ocRevokeCorrelator{
		resolve: cc.RuntimeConnection{ID: "ext-42", Alias: "adopted-alias", ProviderAccountID: "acct-1", Provider: "app-oc", Status: "active"},
	}
	w := ocRevokeNewWorker(t, db, &ocRevokeAdmin{})
	w.SetRuntimeCorrelator(fc)
	w.SetTransientCipher(cipher)

	if err := w.RunOnce(ctx); err != nil {
		t.Fatalf("verifying retry must complete, got %v", err)
	}
	if _, submits := fc.counts(); submits != 0 {
		t.Fatalf("verifying retry submitted the key %d times, want 0 (resolve-first)", submits)
	}
	got, err := store.GetOCAttempt(ctx, 7, a.ID)
	if err != nil || got.State != appconnectorsvc.OCAttemptActive {
		t.Fatalf("attempt after verifying retry = %+v err=%v, want active (no orphan)", got, err)
	}
	b, err := store.GetBinding(ctx, 7, "conn1")
	if err != nil || b.State != appconn.OCBindingActive || b.ExternalID != "ext-42" || b.Alias != "adopted-alias" {
		t.Fatalf("binding after verifying retry = %+v err=%v", b, err)
	}
	if n := ocRevokeOutboxCount(t, db); n != 0 {
		t.Fatalf("outbox rows after completion = %d, want 0", n)
	}
}

// TestOCRevokeAPIKeyHandoffOrphanCleanupOnAdoptionFailure (ruling 1a, T07
// Q-2): when the external submit SUCCEEDS but the alias adoption loses (the
// stored alias moved under us), the just-created external connection is
// scheduled for deletion — the extended cleanup triple catches the
// adopted-alias orphan — and the operation terminates instead of looping.
func TestOCRevokeAPIKeyHandoffOrphanCleanupOnAdoptionFailure(t *testing.T) {
	db := ocRevokeWorkerDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ocRevokeSeedConnection(t, db, 7, "conn1", 1)

	a, err := appconnectorsvc.NewOCAttempt(appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "conn1", "rt-1", "app-oc", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	store := repoappconn.NewOCStore(db)
	if err := store.SaveOCAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.AdvanceOCAttempt(ctx, 7, a.ID, appconnectorsvc.OCAttemptPending, appconnectorsvc.OCAttemptAuthorizing, now); err != nil || !ok {
		t.Fatalf("advance: %v", err)
	}

	cipher, err := appconnectorsvc.NewTransientCipherFromKey("t08-test-key")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := cipher.Seal("ghp_plain_secret", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	ocRevokeSeedOp(t, db, "op-key-1", "authorize", 7, a.ID+"|"+sealed, 1)

	// Sabotage: between the successful external submit and the adoption
	// write, another writer moves the stored alias — the adoption conditional
	// then matches zero rows.
	fc := &ocRevokeCorrelator{
		submit: cc.RuntimeConnection{ID: "ext-ORPHAN", Alias: "runtime-minted", ProviderAccountID: "acct-1", Provider: "app-oc", Status: "active"},
	}
	fc.onSubmit = func() {
		if err := db.Exec("UPDATE connector_authorization_attempts SET alias = 'hijacked' WHERE id = ?", a.ID).Error; err != nil {
			t.Error(err)
		}
	}
	w := ocRevokeNewWorker(t, db, &ocRevokeAdmin{})
	w.SetRuntimeCorrelator(fc)
	w.SetTransientCipher(cipher)

	if err := w.RunOnce(ctx); err != nil {
		t.Fatalf("adoption-loss must be a permanent drop (nil return), got %v", err)
	}
	// The adopted-alias orphan is reconciled: exactly one delete_connection
	// row naming THIS attempt and the submitted external id.
	var resource string
	if err := db.Raw("SELECT resource_id FROM connector_operations_outbox WHERE kind = 'delete_connection'").Row().Scan(&resource); err != nil {
		t.Fatal("orphan cleanup row missing")
	}
	if want := a.ID + "|ext-ORPHAN"; resource != want {
		t.Fatalf("orphan cleanup resource = %q, want %q", resource, want)
	}
	// The hijacked attempt was NOT consumed by this execution.
	got, err := store.GetOCAttempt(ctx, 7, a.ID)
	if err != nil || got.State != appconnectorsvc.OCAttemptAuthorizing {
		t.Fatalf("attempt after adoption loss = %+v err=%v, want authorizing (untouched)", got, err)
	}
}

// TestOCRevokeReauthorizationNewAliasConflictsAndCleansUp (ruling 1b, matrix:
// re-authorize with new alias): after a revocation, a fresh authorization on
// the SAME connection resolves to a NEW external connection; activation is a
// CLEAR conflict (binding identity frozen), the new external connection is
// scheduled for remote deletion, and nothing retries forever.
func TestOCRevokeReauthorizationNewAliasConflictsAndCleansUp(t *testing.T) {
	db := ocRevokeWorkerDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ocRevokeSeedConnection(t, db, 7, "conn1", 1)
	ocRevokeSeedBinding(t, db, 7, "conn1", "ext-42", "alias-1", appconn.OCBindingActive, 1)
	store := repoappconn.NewOCStore(db)

	if err := store.RevokeOCConnection(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "conn1", 1); err != nil {
		t.Fatal(err)
	}

	// Fresh re-authorization attempt at the NEW generation, under a NEW alias.
	a, err := appconnectorsvc.NewOCAttempt(appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "conn1", "rt-1", "app-oc", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveOCAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.AdvanceOCAttempt(ctx, 7, a.ID, appconnectorsvc.OCAttemptPending, appconnectorsvc.OCAttemptAuthorizing, now); err != nil || !ok {
		t.Fatal(err)
	}
	if ok, err := store.AdvanceOCAttempt(ctx, 7, a.ID, appconnectorsvc.OCAttemptAuthorizing, appconnectorsvc.OCAttemptVerifying, now); err != nil || !ok {
		t.Fatal(err)
	}
	ocRevokeSeedOp(t, db, "op-confirm-1", "confirm", 7, a.ID, 2)

	fc := &ocRevokeCorrelator{
		resolve: cc.RuntimeConnection{ID: "ext-NEW", Alias: a.Alias, ProviderAccountID: "acct-1", Provider: "app-oc", Status: "active"},
	}
	admin := &ocRevokeAdmin{}
	w := ocRevokeNewWorker(t, db, admin)
	w.SetRuntimeCorrelator(fc)

	// First tick: the activation conflicts (re-bind on a frozen binding) and
	// the row drops permanently — NOT an infinite retry loop.
	if err := w.RunOnce(ctx); err != nil {
		t.Fatalf("re-bind conflict must be a permanent drop, got %v", err)
	}
	got, err := store.GetOCAttempt(ctx, 7, a.ID)
	if err != nil || got.State != appconnectorsvc.OCAttemptFailed {
		t.Fatalf("attempt after re-bind conflict = %+v err=%v, want failed (terminal transition)", got, err)
	}
	b, err := store.GetBinding(ctx, 7, "conn1")
	if err != nil || b.State != appconn.OCBindingRevoked || b.ExternalID != "ext-42" {
		t.Fatalf("frozen binding must stay revoked: %+v err=%v", b, err)
	}
	// Drain the queue: the revocation's own durable cleanup row (deleting
	// the OLD external connection) and the reconciliation row (deleting the
	// NEW one) both complete — nothing retries forever.
	for i := 0; ocRevokeOutboxCount(t, db) > 0; i++ {
		if i > 5 {
			t.Fatal("cleanup queue never drained")
		}
		if err := w.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	calls := admin.deleted()
	if len(calls) != 2 {
		t.Fatalf("remote deletes = %v, want exactly [ext-42 ext-NEW] (revocation + reconciliation)", calls)
	}
	seenNew, seenOld := false, false
	for _, c := range calls {
		seenNew = seenNew || c == "ext-NEW"
		seenOld = seenOld || c == "ext-42"
	}
	if !seenNew || !seenOld {
		t.Fatalf("remote deletes = %v, want both ext-42 and ext-NEW", calls)
	}
	if n := ocRevokeOutboxCount(t, db); n != 0 {
		t.Fatalf("outbox rows after reconciliation = %d, want 0 (no infinite backoff)", n)
	}
}

// TestOCRevokeFileSecretSinkRoundTrip (ruling 1d support): the file sink can
// READ back what it wrote — the token record-id reference the recast
// reconciliation resolves before minting a replacement token.
func TestOCRevokeFileSecretSinkRoundTrip(t *testing.T) {
	sink, err := cc.NewFileSecretSink(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := sink.PutSecret(ctx, "oc/runtime-token/7/conn1/1/record-id", "rec-42"); err != nil {
		t.Fatal(err)
	}
	got, err := sink.GetSecret(ctx, "oc/runtime-token/7/conn1/1/record-id")
	if err != nil || got != "rec-42" {
		t.Fatalf("GetSecret = %q err=%v, want rec-42 round trip", got, err)
	}
	if _, err := sink.GetSecret(ctx, "oc/runtime-token/7/conn1/1"); err == nil {
		t.Fatal("missing reference must fail closed, not return empty success")
	}
}
