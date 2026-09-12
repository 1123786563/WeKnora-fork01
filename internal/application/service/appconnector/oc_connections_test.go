package appconnector

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// plan T07: expiry + forged-correlation rejection (verbatim sketch)
// ---------------------------------------------------------------------------

func TestOAuthCompletionRejectsWrongAliasAndExpiry(t *testing.T) {
	now := time.Unix(100, 0)
	sub := appconn.OCSubject{TenantID: 1, ActorID: "alice"}
	a, err := NewOCAttempt(sub, "c", "r", "github", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	a.State = "verifying"
	if CanCompleteOCAttempt(a, sub, "someone-else", 1, now) {
		t.Fatal("alias substitution")
	}
	if CanCompleteOCAttempt(a, sub, a.Alias, 1, a.ExpiresAt) {
		t.Fatal("expired callback")
	}
}

func TestNewOCAttemptShapesUUIDAttemptAndAlias(t *testing.T) {
	now := time.Unix(1000, 0)
	sub := appconn.OCSubject{TenantID: 7, ActorID: "alice"}
	a, err := NewOCAttempt(sub, "conn-1", "rt-1", "github", 3, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(a.ID); err != nil {
		t.Fatalf("attempt id is not a UUID: %q", a.ID)
	}
	if _, err := uuid.Parse(a.Alias); err != nil {
		t.Fatalf("attempt alias is not a UUID: %q", a.Alias)
	}
	if a.ID == a.Alias {
		t.Fatal("attempt id and alias must be independent UUIDs")
	}
	if a.State != OCAttemptPending {
		t.Fatalf("fresh attempt state = %q, want pending", a.State)
	}
	if !a.ExpiresAt.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("attempt expiry = %v, want %v (15 minutes)", a.ExpiresAt, now.Add(15*time.Minute))
	}
	if a.Subject != sub || a.ConnectionID != "conn-1" || a.RuntimeID != "rt-1" ||
		a.Provider != "github" || a.AuthVersion != 3 {
		t.Fatalf("attempt fields not echoed: %+v", a)
	}
}

func TestNewOCAttemptRejectsMalformedInput(t *testing.T) {
	now := time.Unix(1000, 0)
	good := appconn.OCSubject{TenantID: 7, ActorID: "alice"}
	cases := []struct {
		name       string
		subject    appconn.OCSubject
		connection string
		runtime    string
		provider   string
		version    int64
		zeroNow    bool
	}{
		{"zero tenant", appconn.OCSubject{ActorID: "alice"}, "c", "r", "github", 1, false},
		{"empty actor", appconn.OCSubject{TenantID: 7}, "c", "r", "github", 1, false},
		{"empty connection", good, "", "r", "github", 1, false},
		{"empty runtime", good, "c", "", "github", 1, false},
		{"empty provider", good, "c", "r", "", 1, false},
		{"zero version", good, "c", "r", "github", 0, false},
		{"negative version", good, "c", "r", "github", -1, false},
		{"zero now", good, "c", "r", "github", 1, true},
	}
	for _, tc := range cases {
		usedNow := now
		if tc.zeroNow {
			usedNow = time.Time{}
		}
		if _, err := NewOCAttempt(tc.subject, tc.connection, tc.runtime, tc.provider, tc.version, usedNow); err == nil {
			t.Fatalf("%s: expected rejection", tc.name)
		}
	}
}

func TestCanCompleteOCAttemptMatrix(t *testing.T) {
	now := time.Unix(5000, 0)
	sub := appconn.OCSubject{TenantID: 7, ActorID: "alice"}
	a, err := NewOCAttempt(sub, "c", "r", "github", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	a.State = OCAttemptVerifying

	if !CanCompleteOCAttempt(a, sub, a.Alias, 2, now.Add(time.Second)) {
		t.Fatal("valid completion rejected")
	}
	for _, state := range []string{OCAttemptPending, OCAttemptAuthorizing, OCAttemptActive, OCAttemptFailed, OCAttemptExpired, OCAttemptRevoked} {
		mut := a
		mut.State = state
		if CanCompleteOCAttempt(mut, sub, mut.Alias, 2, now) {
			t.Fatalf("state %q must never complete", state)
		}
	}
	if CanCompleteOCAttempt(a, appconn.OCSubject{TenantID: 8, ActorID: "alice"}, a.Alias, 2, now) {
		t.Fatal("tenant forgery completed")
	}
	if CanCompleteOCAttempt(a, appconn.OCSubject{TenantID: 7, ActorID: "bob"}, a.Alias, 2, now) {
		t.Fatal("actor forgery completed")
	}
	if CanCompleteOCAttempt(a, sub, a.Alias, 3, now) {
		t.Fatal("stale version completed")
	}
	if CanCompleteOCAttempt(a, sub, a.Alias, 2, a.ExpiresAt) {
		t.Fatal("expiry boundary must reject (now.Before(expiry) is strict)")
	}
}

// ---------------------------------------------------------------------------
// R11 carry: full subject guard + T04-QF-5 nil-semantics pin
// ---------------------------------------------------------------------------

// TestNewOCSubjectGuardNilSemanticsPin is the T04-QF-5 carry: the new full
// guard constructor nil-source semantics are pinned IN REPO. nil
// installations skip that check, nil grants fail space connections closed,
// nil bindings skip OC verification (native branch) - and the fully-wired
// variant performs every check. Native OAuth connections stay usable under
// the full wiring (no binding row means the native branch).
func TestNewOCSubjectGuardNilSemanticsPin(t *testing.T) {
	_, src, grants, db := newAuthorizerFixture(t)
	seedOCSubjectFixture(t, db)
	alice := appconn.OCSubject{TenantID: 7, ActorID: "alice"}

	// Interim semantics preserved: nil installations/grants/bindings.
	interim := NewOCSubjectGuard(src, nil, nil, nil)
	if err := interim.Check(context.Background(), alice, "c-personal", 2); err != nil {
		t.Fatalf("interim guard must keep the native personal path: %v", err)
	}
	if err := interim.Check(context.Background(), alice, "c-space", 2); err == nil {
		t.Fatal("interim guard must fail space connections closed without a grant source")
	}

	// Full wiring: binding store + installation source live.
	full := NewOCSubjectGuard(src, NewInstallationStateSource(&dbInstallationLookup{db: db}), grants, appconnectorrepo.NewOCStore(db))
	seedOCBinding(t, db, "c-personal", appconn.OCBindingActive, 2)
	if err := full.Check(context.Background(), alice, "c-personal", 2); err != nil {
		t.Fatalf("active binding must pass the full guard: %v", err)
	}
	// Space connection without a grant still fails; with the explicit grant
	// it passes (the grant source is live in the full wiring).
	if err := full.Check(context.Background(), alice, "c-space", 2); err == nil {
		t.Fatal("space connection without grant must fail under the full guard")
	}
	grants.set(7, "c-space", "alice")
	if err := full.Check(context.Background(), alice, "c-space", 2); err != nil {
		t.Fatalf("granted space connection must pass the full guard: %v", err)
	}
	// A revoked binding rejects even its own owner.
	if err := db.Model(&appconnectorrepo.OCBindingRow{}).
		Where("tenant_id = ? AND connection_id = ?", 7, "c-personal").
		Update("state", appconn.OCBindingRevoked).Error; err != nil {
		t.Fatal(err)
	}
	if err := full.Check(context.Background(), alice, "c-personal", 2); err == nil {
		t.Fatal("revoked binding must reject")
	}
	// Disabled installation rejects even a granted actor.
	if err := db.Model(&appconnectorrepo.InstallationRow{}).
		Where("id = ? AND tenant_id = ?", "inst-1", 7).
		Update("state", appconn.InstallationDisabled).Error; err != nil {
		t.Fatal(err)
	}
	if err := full.Check(context.Background(), alice, "c-space", 2); err == nil {
		t.Fatal("disabled installation must reject even with a grant")
	}
}

// ---------------------------------------------------------------------------
// store mechanics: attempt persistence, conditional transitions, enqueue
// ---------------------------------------------------------------------------

func openOCConnectionDBMigrated(t *testing.T) *gorm.DB {
	t.Helper()
	db := openActionDB(t, memDSN(t))
	if err := db.AutoMigrate(
		&appconnectorrepo.AppVersion{},
		&appconnectorrepo.InstallationRow{},
		&appconnectorrepo.ConnectionRow{},
		&appconnectorrepo.OCBindingRow{},
		&appconnectorrepo.OCAuthorizationAttemptRow{},
		&appconnectorrepo.OCOperationsOutboxRow{},
		&appconnectorrepo.OCRuntimeRow{},
		&types.TenantMember{},
		&types.MCPOAuthToken{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedOCRuntime(t *testing.T, db *gorm.DB, id string, enabled bool) {
	t.Helper()
	if err := db.Create(&appconnectorrepo.OCRuntimeRow{ID: id, Address: "http://open-connector:8080", Enabled: enabled}).Error; err != nil {
		t.Fatal(err)
	}
}

func seedOCAttempt(t *testing.T, db *gorm.DB, a appconn.OCAuthorizationAttempt) {
	t.Helper()
	if err := appconnectorrepo.NewOCStore(db).SaveOCAttempt(context.Background(), a); err != nil {
		t.Fatal(err)
	}
}

func attemptRow(t *testing.T, db *gorm.DB, id string) appconn.OCAuthorizationAttempt {
	t.Helper()
	a, err := appconnectorrepo.NewOCStore(db).GetOCAttempt(context.Background(), 7, id)
	if err != nil {
		t.Fatalf("load attempt %s: %v", id, err)
	}
	return a
}

func outboxRows(t *testing.T, db *gorm.DB, kind string) []appconnectorrepo.OCOperationsOutboxRow {
	t.Helper()
	var rows []appconnectorrepo.OCOperationsOutboxRow
	if err := db.Where("kind = ?", kind).Order("created_at, id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestStoreAttemptLifecycleTransitions(t *testing.T) {
	db := openOCConnectionDBMigrated(t)
	store := appconnectorrepo.NewOCStore(db)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()

	a, err := NewOCAttempt(appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c", "rt-1", "github", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveOCAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetOCAttempt(ctx, 8, a.ID); err == nil {
		t.Fatal("cross-tenant attempt read must fail")
	}

	// Forward transitions are conditional and expiry-guarded.
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a.ID, OCAttemptPending, OCAttemptAuthorizing, now.Add(time.Minute)); !ok {
		t.Fatal("pending to authorizing must advance")
	}
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a.ID, OCAttemptPending, OCAttemptAuthorizing, now.Add(time.Minute)); ok {
		t.Fatal("replayed transition must lose (state already moved)")
	}
	if ok, _ := store.AdvanceOCAttempt(ctx, 8, a.ID, OCAttemptAuthorizing, OCAttemptVerifying, now.Add(time.Minute)); ok {
		t.Fatal("tenant forgery must never transition another tenant attempt")
	}
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a.ID, OCAttemptAuthorizing, OCAttemptVerifying, a.ExpiresAt.Add(time.Second)); ok {
		t.Fatal("expired attempt must not advance")
	}
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a.ID, OCAttemptAuthorizing, OCAttemptVerifying, now.Add(2*time.Minute)); !ok {
		t.Fatal("authorizing to verifying must advance inside the window")
	}

	// Terminal states never revive.
	if ok, _ := store.TerminateOCAttempt(ctx, 7, a.ID, OCAttemptRevoked, now.Add(3*time.Minute)); !ok {
		t.Fatal("cancel must terminate")
	}
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a.ID, OCAttemptRevoked, OCAttemptVerifying, now.Add(3*time.Minute)); ok {
		t.Fatal("revoked attempt must never revive")
	}
	if ok, _ := store.TerminateOCAttempt(ctx, 7, a.ID, OCAttemptActive, now.Add(4*time.Minute)); ok {
		t.Fatal("terminal attempt must not terminate into a different lifecycle state")
	}
}

func TestStoreEnqueueOperationNormalizesUTC(t *testing.T) {
	db := openOCConnectionDBMigrated(t)
	store := appconnectorrepo.NewOCStore(db)
	ctx := context.Background()

	// A non-UTC zone must still land as the same instant in UTC (T05-Q-03).
	zone := time.FixedZone("behind", -3*3600)
	local := time.Date(2026, 9, 13, 1, 0, 0, 0, zone)
	if err := store.EnqueueOCOperation(ctx, "op-1", 7, "att-1", 1, KindOCAuthorize, local); err != nil {
		t.Fatal(err)
	}
	rows := outboxRows(t, db, KindOCAuthorize)
	if len(rows) != 1 {
		t.Fatalf("expected one enqueued authorize op, got %d", len(rows))
	}
	if got := rows[0].NextAt; !got.Equal(local) || got.Location() != time.UTC {
		t.Fatalf("next_at = %v (loc %v), want %v in UTC", got, got.Location(), local.UTC())
	}
	if rows[0].ResourceID != "att-1" || rows[0].ResourceVersion != 1 || rows[0].TenantID != 7 {
		t.Fatalf("enqueued row fields wrong: %+v", rows[0])
	}

	if err := store.EnqueueOCOperation(ctx, "", 7, "r", 1, KindOCAuthorize, local); err == nil {
		t.Fatal("blank id must be rejected")
	}
	if err := store.EnqueueOCOperation(ctx, "op-2", 0, "r", 1, KindOCAuthorize, local); err == nil {
		t.Fatal("zero tenant must be rejected")
	}
	if err := store.EnqueueOCOperation(ctx, "op-2", 7, "r", 0, KindOCAuthorize, local); err == nil {
		t.Fatal("zero version must be rejected")
	}
	if err := store.EnqueueOCOperation(ctx, "op-2", 7, "r", 1, KindOCAuthorize, time.Time{}); err == nil {
		t.Fatal("zero next_at must be rejected")
	}
}

// ---------------------------------------------------------------------------
// OCConnectionService.Begin / Confirm / Cancel
// ---------------------------------------------------------------------------

func newOCConnectionServiceFixture(t *testing.T) (*OCConnectionService, *gorm.DB) {
	t.Helper()
	db := openOCConnectionDBMigrated(t)
	seedOCSubjectFixture(t, db) // tenant 7: inst-1 (app "mail"), alice+bob, c-personal/c-space at v2
	seedOCRuntime(t, db, "rt-1", true)
	svc := NewOCConnectionService(
		&dbCredentialSource{db: db},
		&dbInstallationLookup{db: db},
		appconnectorrepo.NewOCStore(db),
	)
	return svc, db
}

// dbCredentialSource answers the Begin chain from real rows: connection by
// id plus tenant membership.
type dbCredentialSource struct{ db *gorm.DB }

func (s *dbCredentialSource) FindConnectionByID(ctx context.Context, connectionID string) (appconn.Connection, error) {
	var row appconnectorrepo.ConnectionRow
	if err := s.db.WithContext(ctx).Where("id = ?", connectionID).First(&row).Error; err != nil {
		return appconn.Connection{}, err
	}
	return appconn.Connection{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind, OwnerID: row.OwnerID,
		CredentialRef: row.CredentialRef, State: row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}

func (s *dbCredentialSource) LoadCredential(ctx context.Context, c appconn.Connection) ([]byte, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *dbCredentialSource) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	var n int64
	if err := s.db.WithContext(ctx).Model(&types.TenantMember{}).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, types.TenantMemberStatusActive).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *dbCredentialSource) TryAcquireRefreshLease(ctx context.Context, c appconn.Connection, leaseID string, until time.Time) (bool, error) {
	return false, nil
}

// dbInstallationLookup adapts the installation store to the service provider
// catalog lookup (the provider identity comes ONLY from the installation
// catalog, never from client input).
type dbInstallationLookup struct{ db *gorm.DB }

func (l *dbInstallationLookup) GetInstallationByID(ctx context.Context, tenant uint64, installationID string) (appconn.Installation, error) {
	var row appconnectorrepo.InstallationRow
	if err := l.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenant, installationID).First(&row).Error; err != nil {
		return appconn.Installation{}, err
	}
	return appconn.Installation{ID: row.ID, AppID: row.AppID, Version: row.AppVersion, State: row.State, TenantID: row.TenantID}, nil
}

func TestBeginCreatesAttemptAndEnqueuesAuthorizeUTC(t *testing.T) {
	svc, db := newOCConnectionServiceFixture(t)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()

	attemptID, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", now)
	if err != nil {
		t.Fatal(err)
	}
	a := attemptRow(t, db, attemptID)
	if a.State != OCAttemptPending {
		t.Fatalf("fresh attempt state = %q", a.State)
	}
	// Provider comes from the installation catalog (app "mail"), runtime from
	// the enabled registry row, version from the connection.
	if a.Provider != "mail" || a.RuntimeID != "rt-1" || a.AuthVersion != 2 {
		t.Fatalf("attempt correlation fields wrong: %+v", a)
	}
	if a.Subject != (appconn.OCSubject{TenantID: 7, ActorID: "alice"}) {
		t.Fatalf("attempt subject not persisted: %+v", a.Subject)
	}
	rows := outboxRows(t, db, KindOCAuthorize)
	if len(rows) != 1 {
		t.Fatalf("Begin must enqueue exactly one authorize op, got %d", len(rows))
	}
	if rows[0].ResourceID != attemptID {
		t.Fatalf("authorize op resource_id = %q, want the attempt id %q", rows[0].ResourceID, attemptID)
	}
	if rows[0].ResourceVersion != 2 {
		t.Fatalf("authorize op version = %d, want 2", rows[0].ResourceVersion)
	}
	if !rows[0].NextAt.Equal(now) || rows[0].NextAt.Location() != time.UTC {
		t.Fatalf("authorize op next_at = %v, want %v UTC", rows[0].NextAt, now)
	}
}

func TestBeginFailsClosed(t *testing.T) {
	svc, db := newOCConnectionServiceFixture(t)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()

	// Tenant forgery: the connection belongs to tenant 7, the caller claims 8.
	if _, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 8, ActorID: "alice"}, "c-personal", now); err == nil {
		t.Fatal("cross-tenant Begin must fail")
	}
	// Subject must be a live member.
	if _, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "ghost"}, "c-personal", now); err == nil {
		t.Fatal("non-member Begin must fail")
	}
	// Unknown connection.
	if _, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "nope", now); err == nil {
		t.Fatal("unknown connection Begin must fail")
	}
	// Disabled installation.
	if err := db.Model(&appconnectorrepo.InstallationRow{}).
		Where("id = ? AND tenant_id = ?", "inst-1", 7).
		Update("state", appconn.InstallationDisabled).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", now); err == nil {
		t.Fatal("disabled installation Begin must fail")
	}
	// No enabled runtime registered: fail closed (blocked-env style).
	if err := db.Model(&appconnectorrepo.OCRuntimeRow{}).Where("id = ?", "rt-1").Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&appconnectorrepo.InstallationRow{}).
		Where("id = ? AND tenant_id = ?", "inst-1", 7).
		Update("state", appconn.InstallationActive).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", now); err == nil {
		t.Fatal("Begin without an enabled runtime must fail closed")
	}
}

func TestConfirmConsumesAuthorizingAttemptOnce(t *testing.T) {
	svc, db := newOCConnectionServiceFixture(t)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()

	attemptID, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", now)
	if err != nil {
		t.Fatal(err)
	}
	// Confirm before the worker started the external authorization.
	if err := svc.ConfirmAt(ctx, attemptID, now.Add(time.Minute)); err == nil {
		t.Fatal("pending attempt must not confirm")
	}
	if rows := outboxRows(t, db, KindOCConfirm); len(rows) != 0 {
		t.Fatalf("rejected confirm must not enqueue, got %d", len(rows))
	}

	store := appconnectorrepo.NewOCStore(db)
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, attemptID, OCAttemptPending, OCAttemptAuthorizing, now); !ok {
		t.Fatal("seed transition failed")
	}
	if err := svc.ConfirmAt(ctx, attemptID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if a := attemptRow(t, db, attemptID); a.State != OCAttemptVerifying {
		t.Fatalf("confirmed attempt state = %q, want verifying", a.State)
	}
	rows := outboxRows(t, db, KindOCConfirm)
	if len(rows) != 1 || rows[0].ResourceID != attemptID {
		t.Fatalf("confirm must enqueue exactly one op for the attempt: %+v", rows)
	}

	// Duplicate callback: the transition is one-consume, so the second
	// Confirm is rejected and never enqueues a second activation op.
	if err := svc.ConfirmAt(ctx, attemptID, now.Add(2*time.Minute)); err == nil {
		t.Fatal("duplicate confirm must fail")
	}
	if rows := outboxRows(t, db, KindOCConfirm); len(rows) != 1 {
		t.Fatalf("duplicate confirm must not enqueue again, got %d", len(rows))
	}
}

func TestConfirmRejectsExpiredAndCancelledAttempts(t *testing.T) {
	svc, db := newOCConnectionServiceFixture(t)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()

	attemptID, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", now)
	if err != nil {
		t.Fatal(err)
	}
	store := appconnectorrepo.NewOCStore(db)
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, attemptID, OCAttemptPending, OCAttemptAuthorizing, now); !ok {
		t.Fatal("seed transition failed")
	}

	// Late callback after cancel: the attempt is revoked, the confirm fails,
	// nothing is enqueued.
	if err := svc.CancelAt(ctx, attemptID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmAt(ctx, attemptID, now.Add(2*time.Minute)); err == nil {
		t.Fatal("late callback after cancel must fail")
	}
	if rows := outboxRows(t, db, KindOCConfirm); len(rows) != 0 {
		t.Fatalf("cancelled confirm must not enqueue, got %d", len(rows))
	}

	// Expired attempt: rejected AND marked expired for the cleanup path.
	expiredID, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", now)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, expiredID, OCAttemptPending, OCAttemptAuthorizing, now); !ok {
		t.Fatal("seed transition failed")
	}
	if err := svc.ConfirmAt(ctx, expiredID, now.Add(16*time.Minute)); err == nil {
		t.Fatal("expired attempt confirm must fail")
	}
	if a := attemptRow(t, db, expiredID); a.State != OCAttemptExpired {
		t.Fatalf("expired confirm must mark the attempt expired, got %q", a.State)
	}
}

// ---------------------------------------------------------------------------
// activation: atomic consume + binding insert (repository evidence)
// ---------------------------------------------------------------------------

func TestStoreActivateOCAttemptAtomic(t *testing.T) {
	db := openOCConnectionDBMigrated(t)
	store := appconnectorrepo.NewOCStore(db)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()

	if err := db.Create(&appconnectorrepo.InstallationRow{
		ID: "inst-9", TenantID: 7, AppID: "github", AppVersion: "1.0.0",
		State: appconn.InstallationActive, Version: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&appconnectorrepo.ConnectionRow{
		TenantID: 7, ID: "c-oc", InstallationID: "inst-9", Kind: appconn.ConnectionKindPersonal,
		OwnerID: "alice", State: appconn.ConnectionActive, AuthVersion: 4,
	}).Error; err != nil {
		t.Fatal(err)
	}
	a, err := NewOCAttempt(appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-oc", "rt-1", "github", 4, now)
	if err != nil {
		t.Fatal(err)
	}
	seedOCAttempt(t, db, a)
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a.ID, OCAttemptPending, OCAttemptAuthorizing, now); !ok {
		t.Fatal("seed authorizing failed")
	}
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a.ID, OCAttemptAuthorizing, OCAttemptVerifying, now); !ok {
		t.Fatal("seed verifying failed")
	}

	if err := store.ActivateOCAttempt(ctx, 7, a.ID, "ext-42", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got := attemptRow(t, db, a.ID)
	if got.State != OCAttemptActive {
		t.Fatalf("activated attempt state = %q", got.State)
	}
	b, err := store.GetBinding(ctx, 7, "c-oc")
	if err != nil {
		t.Fatal(err)
	}
	if b.State != appconn.OCBindingActive || b.BindingVersion != 1 ||
		b.ExternalID != "ext-42" || b.Alias != a.Alias || b.RuntimeID != "rt-1" ||
		b.Provider != "github" || b.AuthVersion != 4 {
		t.Fatalf("activation binding wrong: %+v", b)
	}

	// One-consume: replaying the activation (duplicate callback racing the
	// same attempt) affects ZERO rows and never writes a second binding.
	if err := store.ActivateOCAttempt(ctx, 7, a.ID, "ext-42", now.Add(2*time.Minute)); err == nil {
		t.Fatal("duplicate activation must be rejected")
	}
	var bindings int64
	if err := db.Model(&appconnectorrepo.OCBindingRow{}).Where("tenant_id = ? AND connection_id = ?", 7, "c-oc").Count(&bindings).Error; err != nil {
		t.Fatal(err)
	}
	if bindings != 1 {
		t.Fatalf("duplicate activation wrote %d bindings", bindings)
	}

	// Version drift: a stale op against a bumped connection version rejects
	// without writing anything.
	a2, err := NewOCAttempt(appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-oc", "rt-1", "github", 4, now)
	if err != nil {
		t.Fatal(err)
	}
	seedOCAttempt(t, db, a2)
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a2.ID, OCAttemptPending, OCAttemptAuthorizing, now); !ok {
		t.Fatal("seed authorizing failed")
	}
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a2.ID, OCAttemptAuthorizing, OCAttemptVerifying, now); !ok {
		t.Fatal("seed verifying failed")
	}
	if err := db.Model(&appconnectorrepo.ConnectionRow{}).
		Where("tenant_id = ? AND id = ?", 7, "c-oc").Update("auth_version", 5).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateOCAttempt(ctx, 7, a2.ID, "ext-43", now.Add(time.Minute)); err == nil {
		t.Fatal("activation against a drifted connection version must fail")
	}
	if a2got := attemptRow(t, db, a2.ID); a2got.State != OCAttemptVerifying {
		t.Fatalf("failed activation must leave the attempt unconsumed, got %q", a2got.State)
	}

	// Disabled installation: activation re-checks and refuses.
	if err := db.Model(&appconnectorrepo.ConnectionRow{}).
		Where("tenant_id = ? AND id = ?", 7, "c-oc").Update("auth_version", 4).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&appconnectorrepo.InstallationRow{}).
		Where("id = ? AND tenant_id = ?", "inst-9", 7).
		Update("state", appconn.InstallationDisabled).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateOCAttempt(ctx, 7, a2.ID, "ext-44", now.Add(time.Minute)); err == nil {
		t.Fatal("activation against a disabled installation must fail")
	}
}

// TestStoreCleanupMatchesAttemptAliasExternalAndNeverNewest pins the cleanup
// rule: reconciliation enqueues a remote delete only when attempt, alias and
// external id all match THIS attempt, and never when a newer activation
// already bound that external connection.
func TestStoreCleanupMatchesAttemptAliasExternalAndNeverNewest(t *testing.T) {
	db := openOCConnectionDBMigrated(t)
	store := appconnectorrepo.NewOCStore(db)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()

	if err := db.Create(&appconnectorrepo.InstallationRow{
		ID: "inst-9", TenantID: 7, AppID: "github", AppVersion: "1.0.0",
		State: appconn.InstallationActive, Version: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&appconnectorrepo.ConnectionRow{
		TenantID: 7, ID: "c-oc", InstallationID: "inst-9", Kind: appconn.ConnectionKindPersonal,
		OwnerID: "alice", State: appconn.ConnectionActive, AuthVersion: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	mkAttempt := func() appconn.OCAuthorizationAttempt {
		t.Helper()
		a, err := NewOCAttempt(appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-oc", "rt-1", "github", 1, now)
		if err != nil {
			t.Fatal(err)
		}
		seedOCAttempt(t, db, a)
		if ok, _ := store.AdvanceOCAttempt(ctx, 7, a.ID, OCAttemptPending, OCAttemptAuthorizing, now); !ok {
			t.Fatal("seed authorizing failed")
		}
		if ok, _ := store.AdvanceOCAttempt(ctx, 7, a.ID, OCAttemptAuthorizing, OCAttemptVerifying, now); !ok {
			t.Fatal("seed verifying failed")
		}
		return a
	}

	// Remote success / local failure on an unbound attempt: cleanup enqueues
	// the remote delete matching (attempt, alias, external id).
	orphan := mkAttempt()
	if err := store.CleanupFailedOCAttempt(ctx, 7, orphan.ID, orphan.Alias, "ext-orphan", now); err != nil {
		t.Fatal(err)
	}
	if got := attemptRow(t, db, orphan.ID); got.State != OCAttemptFailed {
		t.Fatalf("cleaned attempt state = %q, want failed", got.State)
	}
	deletes := outboxRows(t, db, KindOCDeleteConnection)
	if len(deletes) != 1 {
		t.Fatalf("cleanup must enqueue exactly one delete_connection, got %d", len(deletes))
	}
	if deletes[0].ResourceID != orphan.ID+"|ext-orphan" {
		t.Fatalf("cleanup delete must carry attempt|external, got %q", deletes[0].ResourceID)
	}

	// Never-newest: the same external id now has an ACTIVE binding from a
	// newer attempt - cleanup must NOT enqueue another remote delete.
	newest := mkAttempt()
	if err := store.ActivateOCAttempt(ctx, 7, newest.ID, "ext-orphan", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	loser := mkAttempt()
	if err := store.CleanupFailedOCAttempt(ctx, 7, loser.ID, loser.Alias, "ext-orphan", now); err != nil {
		t.Fatal(err)
	}
	if got := attemptRow(t, db, loser.ID); got.State != OCAttemptFailed {
		t.Fatalf("losing attempt must still be marked failed, got %q", got.State)
	}
	if deletes := outboxRows(t, db, KindOCDeleteConnection); len(deletes) != 1 {
		t.Fatalf("cleanup must never delete the newest connection, got %d delete ops", len(deletes))
	}

	// Alias attribution: a mismatched alias never triggers deletion even
	// without an active binding.
	odd := mkAttempt()
	if err := store.CleanupFailedOCAttempt(ctx, 7, odd.ID, "not-"+odd.Alias, "ext-other", now); err != nil {
		t.Fatal(err)
	}
	if deletes := outboxRows(t, db, KindOCDeleteConnection); len(deletes) != 1 {
		t.Fatalf("alias-mismatched cleanup must not enqueue a delete, got %d", len(deletes))
	}
}

// ---------------------------------------------------------------------------
// API-key transient ciphertext (15-minute handoff, purpose-separated key)
// ---------------------------------------------------------------------------

// TestAttemptExpiryComparisonsNormalizeTimeZone pins the T07 quality Q-1
// fix: stored expires_at is always UTC, so every store-level expires_at
// predicate must compare against the UTC-normalized instant. A caller
// supplying a non-UTC clock (UTC+8 desktop, UTC-5 host) must neither get a
// valid transition rejected nor an expired one accepted.
func TestAttemptExpiryComparisonsNormalizeTimeZone(t *testing.T) {
	db := openOCConnectionDBMigrated(t)
	store := appconnectorrepo.NewOCStore(db)
	ctx := context.Background()
	base := time.Unix(1700000000, 0).UTC()
	cst := time.FixedZone("CST", 8*3600)  // UTC+8
	est := time.FixedZone("EST", -5*3600) // UTC-5

	if err := db.Create(&appconnectorrepo.InstallationRow{
		ID: "inst-tz", TenantID: 7, AppID: "github", AppVersion: "1.0.0",
		State: appconn.InstallationActive, Version: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&appconnectorrepo.ConnectionRow{
		TenantID: 7, ID: "c-tz", InstallationID: "inst-tz", Kind: appconn.ConnectionKindPersonal,
		OwnerID: "alice", State: appconn.ConnectionActive, AuthVersion: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	mkAttempt := func() appconn.OCAuthorizationAttempt {
		t.Helper()
		a, err := NewOCAttempt(appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-tz", "rt-1", "github", 1, base)
		if err != nil {
			t.Fatal(err)
		}
		seedOCAttempt(t, db, a)
		return a
	}
	seedVerified := func() appconn.OCAuthorizationAttempt {
		t.Helper()
		a := mkAttempt()
		if ok, _ := store.AdvanceOCAttempt(ctx, 7, a.ID, OCAttemptPending, OCAttemptAuthorizing, base); !ok {
			t.Fatal("seed authorizing failed")
		}
		if ok, _ := store.AdvanceOCAttempt(ctx, 7, a.ID, OCAttemptAuthorizing, OCAttemptVerifying, base.Add(time.Minute)); !ok {
			t.Fatal("seed verifying failed")
		}
		return a
	}

	// (a) UTC+8 wall clock, valid instants: every transition and the
	// activation must succeed (broken code serializes the +8 wall clock,
	// which exceeds the UTC-stored expiry and rejects them).
	a1 := mkAttempt()
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a1.ID, OCAttemptPending, OCAttemptAuthorizing, base.Add(time.Minute).In(cst)); !ok {
		t.Fatal("UTC+8 clock: valid advance rejected")
	}
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a1.ID, OCAttemptAuthorizing, OCAttemptVerifying, base.Add(2*time.Minute).In(cst)); !ok {
		t.Fatal("UTC+8 clock: valid confirm transition rejected")
	}
	if ok, _ := store.AdoptOCAttemptAlias(ctx, 7, a1.ID, a1.Alias, "runtime-alias-tz", base.Add(3*time.Minute).In(cst)); !ok {
		t.Fatal("UTC+8 clock: valid alias adoption rejected")
	}
	a2 := seedVerified()
	if err := store.ActivateOCAttempt(ctx, 7, a2.ID, "ext-tz", base.Add(3*time.Minute).In(cst)); err != nil {
		t.Fatalf("UTC+8 clock: valid activation rejected: %v", err)
	}

	// (b) UTC-5 wall clock, instants AFTER expiry: nothing may transition or
	// activate (broken code reads the -5 wall clock as earlier than the
	// UTC-stored expiry and wrongly satisfies the predicate).
	past := base.Add(2 * time.Hour) // 1h45m after the 15-minute expiry
	a3 := mkAttempt()
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, a3.ID, OCAttemptPending, OCAttemptAuthorizing, past.In(est)); ok {
		t.Fatal("UTC-5 clock: expired advance accepted")
	}
	if ok, _ := store.AdoptOCAttemptAlias(ctx, 7, a3.ID, a3.Alias, "alias-late", past.In(est)); ok {
		t.Fatal("UTC-5 clock: expired alias adoption accepted")
	}
	a4 := seedVerified()
	if err := store.ActivateOCAttempt(ctx, 7, a4.ID, "ext-late", past.In(est)); err == nil {
		t.Fatal("UTC-5 clock: expired activation accepted")
	}
	if got := attemptRow(t, db, a4.ID); got.State != OCAttemptVerifying {
		t.Fatalf("expired activation must leave the attempt unconsumed, got %q", got.State)
	}

	// Service path: Confirm runs on the service clock; a deployment whose
	// clock carries a non-UTC zone (UTC+8) must still confirm inside the
	// window (this was the reviewer's public-path breakage).
	seedOCSubjectFixture(t, db)
	seedOCRuntime(t, db, "rt-1", true)
	svc := NewOCConnectionService(&dbCredentialSource{db: db}, &dbInstallationLookup{db: db}, store,
		WithNow(func() time.Time { return base.Add(time.Minute).In(cst) }))
	attemptID, err := svc.BeginAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", base)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := store.AdvanceOCAttempt(ctx, 7, attemptID, OCAttemptPending, OCAttemptAuthorizing, base); !ok {
		t.Fatal("seed authorizing failed")
	}
	if err := svc.Confirm(ctx, attemptID); err != nil {
		t.Fatalf("Confirm on a UTC+8 service clock must succeed inside the window: %v", err)
	}
	if got := attemptRow(t, db, attemptID); got.State != OCAttemptVerifying {
		t.Fatalf("Confirm state = %q, want verifying", got.State)
	}
}

func TestTransientCipherRoundTripAndSeparation(t *testing.T) {
	c1, err := NewTransientCipherFromKey("test-key-one")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := NewTransientCipherFromKey("test-key-two")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := c1.Seal("ghp_secret_api_key", "att-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "ghp_secret_api_key") {
		t.Fatal("sealed form must not contain plaintext")
	}
	got, err := c1.Open(sealed, "att-1")
	if err != nil || got != "ghp_secret_api_key" {
		t.Fatalf("round trip failed: %q %v", got, err)
	}
	// The attempt id is bound as AAD: a ciphertext sealed for one attempt
	// cannot be replayed against another.
	if _, err := c1.Open(sealed, "att-2"); err == nil {
		t.Fatal("cross-attempt replay must fail")
	}
	if _, err := c2.Open(sealed, "att-1"); err == nil {
		t.Fatal("a different key material must never decrypt")
	}
	// Sealing is randomized: two seals of the same plaintext differ.
	other, err := c1.Seal("ghp_secret_api_key", "att-1")
	if err != nil {
		t.Fatal(err)
	}
	if other == sealed {
		t.Fatal("seal must be randomized")
	}
}

func TestBeginAPIKeyEncryptsIntoOutbox(t *testing.T) {
	db := openOCConnectionDBMigrated(t)
	seedOCSubjectFixture(t, db)
	seedOCRuntime(t, db, "rt-1", true)
	cipher, err := NewTransientCipherFromKey("test-key-one")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewOCConnectionService(&dbCredentialSource{db: db}, &dbInstallationLookup{db: db}, appconnectorrepo.NewOCStore(db), WithTransientCipher(cipher))
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()

	attemptID, err := svc.BeginAPIKeyAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", "ghp_super_secret", now)
	if err != nil {
		t.Fatal(err)
	}
	rows := outboxRows(t, db, KindOCAuthorize)
	if len(rows) != 1 {
		t.Fatalf("api-key Begin must enqueue one authorize op, got %d", len(rows))
	}
	res := rows[0].ResourceID
	if !strings.HasPrefix(res, attemptID+"|") {
		t.Fatalf("api-key authorize op must carry attempt|sealed, got %q", res)
	}
	if strings.Contains(res, "ghp_super_secret") {
		t.Fatal("outbox must never carry the plaintext api key")
	}
	// The sealed blob decrypts only under the attempt AAD.
	if _, err := cipher.Open(strings.TrimPrefix(res, attemptID+"|"), attemptID); err != nil {
		t.Fatal("sealed blob must open under its attempt id")
	}

	// Fail closed without a cipher.
	bare := NewOCConnectionService(&dbCredentialSource{db: db}, &dbInstallationLookup{db: db}, appconnectorrepo.NewOCStore(db))
	if _, err := bare.BeginAPIKeyAt(ctx, appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "c-personal", "k", now); err == nil {
		t.Fatal("BeginAPIKey without a transient cipher must fail closed")
	}
}

func TestTransientCipherFromEnvFailClosed(t *testing.T) {
	old, had := os.LookupEnv("WEKNORA_OC_TRANSIENT_KEY")
	defer func() {
		if had {
			os.Setenv("WEKNORA_OC_TRANSIENT_KEY", old)
		} else {
			os.Unsetenv("WEKNORA_OC_TRANSIENT_KEY")
		}
	}()
	os.Unsetenv("WEKNORA_OC_TRANSIENT_KEY")
	if c, err := NewTransientCipherFromEnv(); err == nil || c != nil {
		t.Fatal("missing env key must fail closed with a nil cipher")
	}
	os.Setenv("WEKNORA_OC_TRANSIENT_KEY", "env-test-key")
	if _, err := NewTransientCipherFromEnv(); err != nil {
		t.Fatalf("valid env key must construct: %v", err)
	}
}
