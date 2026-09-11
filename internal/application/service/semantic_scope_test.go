package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
)

// newSemanticScopeFixture opens a REAL migrated SQLite database (business
// migrations) and seeds one tenant, one KB and two documents, mirroring the
// plan's controlled-user fixture. Epoch/denial state goes through the REAL
// SemanticControlRepository transactions.
type semanticScopeFixture struct {
	t     *testing.T
	db    *gorm.DB
	repo  *apprepo.SemanticControlRepository
	svc   *SemanticScopeService
	clock *semanticScopeClock
	scope types.SemanticScopeKey
}

// semanticScopeClock is a controllable time source for expiry tests.
type semanticScopeClock struct{ value time.Time }

func newSemanticScopeFixture(t *testing.T) *semanticScopeFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))

	dbPath := filepath.Join(t.TempDir(), "semantic-scope.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})

	repo := apprepo.NewSemanticControlRepository(db)
	clock := &semanticScopeClock{value: time.Now().UTC()}
	svc := NewSemanticScopeService(repo, SemanticScopeConfig{
		Audience:   "weknora-semantic",
		TTL:        5 * time.Minute,
		IssuingKey: []byte("test-issuing-key-0123456789abcdef0123"), // 32+ bytes
		Now:        func() time.Time { return clock.value },
	})
	f := &semanticScopeFixture{t: t, db: db, repo: repo, svc: svc, clock: clock,
		scope: types.SemanticScopeKey{TenantID: 1, KBID: "kb-a"}}
	f.mutate("doc-public", false, "{\"chunks\":[\"c1\"]}")
	f.mutate("private-doc", false, "{\"chunks\":[\"c2\"]}")
	return f
}

func (f *semanticScopeFixture) mutate(documentID string, deleted bool, payload string) {
	f.t.Helper()
	_, err := f.repo.WithSemanticMutation(context.Background(), types.SemanticMutation{
		TenantID: f.scope.TenantID, KBID: f.scope.KBID, DocumentID: documentID,
		ExpectedRevision: 0, Deleted: deleted, Payload: []byte(payload),
	}, func(tx *gorm.DB) error { return nil })
	if err != nil {
		f.t.Fatalf("seed mutation for %s failed: %v", documentID, err)
	}
}

func (f *semanticScopeFixture) Issue(subjectID, kbID string) types.SemanticAccessScope {
	f.t.Helper()
	scope, err := f.svc.Issue(context.Background(), subjectID,
		types.SemanticScopeKey{TenantID: f.scope.TenantID, KBID: kbID}, "reason")
	require.NoError(f.t, err)
	return scope
}

func (f *semanticScopeFixture) RevokeDocument(subjectID, documentID string) {
	f.t.Helper()
	_, err := f.repo.WithSemanticMutation(context.Background(), types.SemanticMutation{
		TenantID: f.scope.TenantID, KBID: f.scope.KBID, DocumentID: documentID,
		ExpectedRevision: 1, Deleted: true, Payload: []byte("{\"deleted\":true}"),
	}, func(tx *gorm.DB) error { return nil })
	require.NoError(f.t, err)
}

func (f *semanticScopeFixture) ValidateDelivery(scope types.SemanticAccessScope) error {
	return f.svc.ValidateDelivery(context.Background(), scope)
}

func TestSemanticScopeShortKeyDisablesService(t *testing.T) {
	f := newSemanticScopeFixture(t)
	// A short HMAC key is indistinguishable from the publicly-known empty
	// key: the constructor must DISABLE the service, never silently sign.
	short := NewSemanticScopeService(f.repo, SemanticScopeConfig{
		Audience: "weknora-semantic", TTL: time.Minute,
		IssuingKey: []byte("short-key"), Now: func() time.Time { return time.Now().UTC() },
	})
	require.Nil(t, short, "short issuing keys must disable scope issuance entirely")
}

func TestSemanticScopeEmptyKeyForgedRefsNeverVerify(t *testing.T) {
	f := newSemanticScopeFixture(t)
	scope := f.Issue("member-a", "kb-a")
	require.NotEmpty(t, scope.ScopeRef)
	// Re-sign the SAME token payload with the empty (public) key: a valid
	// service must reject it as a bad signature.
	forged, err := forgeScopeRefWithEmptyKey(scope.ScopeRef)
	require.NoError(t, err)
	_, err = f.svc.Resolve(context.Background(), forged)
	require.ErrorIs(t, err, ErrSemanticScopeInvalid)
}

// forgeScopeRefWithEmptyKey re-signs the payload half of a real reference
// under the empty key, exactly what an attacker without the issuing key
// can produce.
func forgeScopeRefWithEmptyKey(ref string) (string, error) {
	encoded, _, ok := strings.Cut(ref, ".")
	if !ok {
		return "", errors.New("malformed ref")
	}
	mac := hmac.New(sha256.New, nil)
	mac.Write([]byte(encoded))
	return encoded + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

func TestSemanticScopeRejectsEpochChange(t *testing.T) {
	f := newSemanticScopeFixture(t) // 定义受控用户、KB、文档与真实epoch repo
	scope := f.Issue("member-a", "kb-a")
	f.RevokeDocument("member-a", "private-doc")
	if err := f.ValidateDelivery(scope); !errors.Is(err, ErrSemanticScopeChanged) {
		t.Fatalf("expected scope invalidation, got %v", err)
	}
}

func TestSemanticScopeValidateHappyPath(t *testing.T) {
	f := newSemanticScopeFixture(t)
	scope := f.Issue("member-a", "kb-a")
	require.NoError(t, f.ValidateDelivery(scope))
}

func TestSemanticScopeRejectsExpired(t *testing.T) {
	f := newSemanticScopeFixture(t)
	issued := f.Issue("member-a", "kb-a")
	require.NoError(t, f.ValidateDelivery(issued))
	// Advance the clock past the TTL: the same scope must now be rejected.
	f.clock.value = f.clock.value.Add(6 * time.Minute)
	require.ErrorIs(t, f.ValidateDelivery(issued), ErrSemanticScopeInvalid)
}

func TestSemanticScopeRejectsForged(t *testing.T) {
	f := newSemanticScopeFixture(t)
	scope := f.Issue("member-a", "kb-a")
	scope.ScopeRef = scope.ScopeRef + "tampered"
	require.ErrorIs(t, f.ValidateDelivery(scope), ErrSemanticScopeInvalid)
	scope2 := f.Issue("member-a", "kb-a")
	scope2.ScopeHash = "deadbeef"
	require.ErrorIs(t, f.ValidateDelivery(scope2), ErrSemanticScopeInvalid)
}

func TestSemanticScopeResolveBuildsSnapshot(t *testing.T) {
	f := newSemanticScopeFixture(t)
	scope := f.Issue("member-a", "kb-a")
	snapshot, err := f.svc.Resolve(context.Background(), scope.ScopeRef)
	require.NoError(t, err)
	require.Contains(t, snapshot.AllowedDocumentIDs, "doc-public")
	require.Contains(t, snapshot.AllowedDocumentIDs, "private-doc")
	require.Equal(t, uint64(1), snapshot.MaxSourceRevision["doc-public"])

	f.RevokeDocument("member-a", "private-doc")
	_, err = f.svc.Resolve(context.Background(), scope.ScopeRef)
	require.ErrorIs(t, err, ErrSemanticScopeChanged)

	fresh := f.Issue("member-a", "kb-a")
	freshSnapshot, err := f.svc.Resolve(context.Background(), fresh.ScopeRef)
	require.NoError(t, err)
	require.NotContains(t, freshSnapshot.AllowedDocumentIDs, "private-doc")
	require.Contains(t, freshSnapshot.AllowedDocumentIDs, "doc-public")
	require.Equal(t, uint64(2), freshSnapshot.DenyRevisions["private-doc"])
}

func TestSemanticScopeDriftWithoutEpochStillInvalidates(t *testing.T) {
	f := newSemanticScopeFixture(t)
	scope := f.Issue("member-a", "kb-a")
	// A plain revision bump (no delete, no epoch move) still widens the
	// snapshot - the issued scope must die on the digest recheck.
	_, err := f.repo.WithSemanticMutation(context.Background(), types.SemanticMutation{
		TenantID: 1, KBID: "kb-a", DocumentID: "doc-public",
		ExpectedRevision: 1, Payload: []byte("{\"chunks\":[\"c1\",\"c1b\"]}"),
	}, func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)
	_, err = f.svc.Resolve(context.Background(), scope.ScopeRef)
	require.ErrorIs(t, err, ErrSemanticScopeChanged)
}

func TestSemanticScopeRetainedPreviousVersions(t *testing.T) {
	f := newSemanticScopeFixture(t)
	_, err := f.repo.WithSemanticMutation(context.Background(), types.SemanticMutation{
		TenantID: 1, KBID: "kb-a", DocumentID: "doc-public",
		ExpectedRevision: 1, Payload: []byte("{\"chunks\":[\"c1\",\"c1b\"]}"),
	}, func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)

	scope := f.Issue("member-a", "kb-a")
	snapshot, err := f.svc.Resolve(context.Background(), scope.ScopeRef)
	require.NoError(t, err)
	require.True(t, snapshot.AllowRetainedPrevious)
	require.Equal(t, uint64(2), snapshot.MaxSourceRevision["doc-public"])
}
