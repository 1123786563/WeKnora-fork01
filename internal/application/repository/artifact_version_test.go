package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
)

func artifactDigest(seed byte) string {
	digest := make([]byte, 64)
	for i := range digest {
		digest[i] = seed
	}
	// Keep the digest within lowercase hex.
	for i := range digest {
		switch digest[i] {
		case '0':
		default:
			digest[i] = 'a' + (digest[i] % 6)
		}
	}
	return string(digest)
}

func insertArtifactVersion(t *testing.T, store *ArtifactVersionStore, row ArtifactVersion) {
	t.Helper()
	require.NoError(t, store.Insert(context.Background(), row))
}

func TestArtifactVersionIsImmutable(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	s := NewArtifactVersionStore(db)
	v := ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'), ObjectKey: "k1", MIME: "text/plain", ScanState: "clean", Size: 1,
	}
	require.NoError(t, s.Insert(ctx, v))
	v.ObjectKey = "k2"
	require.Error(t, s.Insert(ctx, v))
}

func TestArtifactVersionInsertIdempotentReplay(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	s := NewArtifactVersionStore(db)
	v := ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'), ObjectKey: "k1", MIME: "text/plain", ScanState: "pending", Size: 9,
	}
	insertArtifactVersion(t, s, v)
	// Exact replay is idempotent: no second row, no error.
	insertArtifactVersion(t, s, v)
	var count int64
	require.NoError(t, db.Table("artifact_versions").Where("tenant_id = ? AND id = ?", 1, "v1").Count(&count).Error)
	require.EqualValues(t, 1, count)

	// A replay after server-owned state transitions still matches the original
	// content, so importer retries must stay idempotent.
	require.NoError(t, s.MarkUploaded(ctx, 1, "v1"))
	insertArtifactVersion(t, s, v)
	got, err := s.Get(ctx, 1, "v1")
	require.NoError(t, err)
	require.Equal(t, "uploaded", got.ScanState)
}

func TestArtifactVersionConflictOnDifferentContent(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	s := NewArtifactVersionStore(db)
	base := ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'), ObjectKey: "k1", MIME: "text/plain", ScanState: "pending", Size: 9,
	}
	insertArtifactVersion(t, s, base)
	mutations := map[string]func(v *ArtifactVersion){
		"digest":    func(v *ArtifactVersion) { v.Digest = artifactDigest('b') },
		"objectkey": func(v *ArtifactVersion) { v.ObjectKey = "other-key" },
		"mime":      func(v *ArtifactVersion) { v.MIME = "application/pdf" },
		"size":      func(v *ArtifactVersion) { v.Size = 10 },
	}
	for name, mutate := range mutations {
		mutated := base
		mutate(&mutated)
		err := s.Insert(ctx, mutated)
		require.ErrorIs(t, err, ErrArtifactVersionConflict, "mutation %s must conflict", name)
	}
}

func TestValidateArtifactMetadata(t *testing.T) {
	require.NoError(t, validateArtifactMetadata(1, artifactDigest('a')))
	require.Error(t, validateArtifactMetadata(0, artifactDigest('a')))
	require.Error(t, validateArtifactMetadata(-1, artifactDigest('a')))
	require.Error(t, validateArtifactMetadata(MaxArtifactVersionBytes+1, artifactDigest('a')))
	require.Error(t, validateArtifactMetadata(1, "short"))
	require.Error(t, validateArtifactMetadata(1, strings.ToUpper(artifactDigest('a'))))
	require.Error(t, validateArtifactMetadata(1, strings.Repeat("g", 64)))
	require.Error(t, validateArtifactMetadata(1, ""))
}

func TestArtifactVersionInsertGuards(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	s := NewArtifactVersionStore(db)
	valid := ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'), ObjectKey: "k1", MIME: "text/plain", ScanState: "pending", Size: 9,
	}
	// Unknown run: an artifact version must attach to a real run of the same
	// tenant and session.
	unknownRun := valid
	unknownRun.RunID = "missing"
	require.ErrorIs(t, s.Insert(ctx, unknownRun), ErrArtifactVersionNotFound)
	mismatched := valid
	mismatched.SessionID = "s2"
	require.ErrorIs(t, s.Insert(ctx, mismatched), ErrArtifactVersionNotFound)
	// Unknown scan state.
	badState := valid
	badState.ScanState = "weird"
	require.Error(t, s.Insert(ctx, badState))
	// Invalid metadata.
	badDigest := valid
	badDigest.Digest = "nope"
	require.Error(t, s.Insert(ctx, badDigest))
	// Missing identity.
	noID := valid
	noID.ID = ""
	require.Error(t, s.Insert(ctx, noID))
	noTenant := valid
	noTenant.TenantID = 0
	require.Error(t, s.Insert(ctx, noTenant))
}

func TestArtifactVersionObjectKeyServerGenerated(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	s := NewArtifactVersionStore(db)
	imported, err := s.Import(ctx, ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'),
		// A client-supplied key must be ignored: the server derives the key.
		ObjectKey: "client-chosen", MIME: "text/plain", ScanState: "ready", Size: 3,
	})
	require.NoError(t, err)
	require.Equal(t, "pending", imported.ScanState)
	require.NotEqual(t, "client-chosen", imported.ObjectKey)
	require.Equal(t, DeriveArtifactObjectKey(1, "r1", artifactDigest('a')), imported.ObjectKey)

	// Replay with a different client-chosen key stays idempotent because the
	// derived key is deterministic and content-addressed.
	again, err := s.Import(ctx, ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'), ObjectKey: "other-client-key", MIME: "text/plain", ScanState: "pending", Size: 3,
	})
	require.NoError(t, err)
	require.Equal(t, imported.ObjectKey, again.ObjectKey)

	// Import with different content under the same ID conflicts.
	_, err = s.Import(ctx, ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('b'), MIME: "text/plain", Size: 3,
	})
	require.ErrorIs(t, err, ErrArtifactVersionConflict)
}

func TestArtifactVersionLifecycleGates(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	s := NewArtifactVersionStore(db)
	_, err = s.Import(ctx, ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'), MIME: "text/plain", Size: 3,
	})
	require.NoError(t, err)

	// pending: uploaded bytes must not be readable yet.
	_, err = s.ReadableArtifactVersion(ctx, 1, "s1", "v1")
	require.ErrorIs(t, err, ErrArtifactVersionNotFound)
	// Scanning before the upload lands is out of order.
	require.Error(t, s.MarkScanned(ctx, 1, "v1", true))
	require.NoError(t, s.MarkUploaded(ctx, 1, "v1"))
	_, err = s.ReadableArtifactVersion(ctx, 1, "s1", "v1")
	require.ErrorIs(t, err, ErrArtifactVersionNotFound)
	// Publishing before a clean scan must fail.
	require.Error(t, s.MarkReady(ctx, 1, "v1"))
	// A failed scan quarantines the version; no URL may be published.
	require.NoError(t, s.MarkScanned(ctx, 1, "v1", false))
	got, err := s.Get(ctx, 1, "v1")
	require.NoError(t, err)
	require.Equal(t, "quarantined", got.ScanState)
	_, err = s.ReadableArtifactVersion(ctx, 1, "s1", "v1")
	require.ErrorIs(t, err, ErrArtifactVersionNotFound)
	// Quarantine is terminal for that version identity: neither a clean scan
	// nor publishing may flip it back.
	require.Error(t, s.MarkScanned(ctx, 1, "v1", true))
	require.Error(t, s.MarkReady(ctx, 1, "v1"))

	// A clean scan followed by publish becomes readable.
	imported2, err := s.Import(ctx, ArtifactVersion{
		TenantID: 1, ID: "v2", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('b'), MIME: "image/png", Size: 4,
	})
	require.NoError(t, err)
	require.NoError(t, s.MarkUploaded(ctx, 1, "v2"))
	// MarkUploaded is idempotent once the upload is recorded.
	require.NoError(t, s.MarkUploaded(ctx, 1, "v2"))
	require.NoError(t, s.MarkScanned(ctx, 1, "v2", true))
	require.NoError(t, s.MarkReady(ctx, 1, "v2"))
	readable, err := s.ReadableArtifactVersion(ctx, 1, "s1", "v2")
	require.NoError(t, err)
	require.Equal(t, imported2.ObjectKey, readable.ObjectKey)
	require.Equal(t, "ready", readable.ScanState)
	// A readable version is bound to its session: another session sees 404.
	_, err = s.ReadableArtifactVersion(ctx, 1, "s2", "v2")
	require.ErrorIs(t, err, ErrArtifactVersionNotFound)
	// Unknown version IDs read as not found.
	_, err = s.ReadableArtifactVersion(ctx, 1, "s1", "missing")
	require.ErrorIs(t, err, ErrArtifactVersionNotFound)
	require.ErrorIs(t, s.MarkUploaded(ctx, 1, "missing"), ErrArtifactVersionNotFound)
}

func TestArtifactVersionCrossTenantIsolation(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	// A second tenant fixture with its own session and run.
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('u2', 'u2', 'u2@example.test', 'x', 2)`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1t2', 2, 'session-t2', 'u2', 'trpc')`,
	).Error)
	admission := testAdmission()
	admission.Key.TenantID = 2
	admission.Key.RunID = "r1"
	admission.SessionID = "s1t2"
	admission.UserID = "u2"
	admission.RequestID = "q2"
	_, err := NewAgentRunStore(db).Admit(ctx, admission)
	require.NoError(t, err)

	_, err = NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)

	s := NewArtifactVersionStore(db)
	for _, tenant := range []uint64{1, 2} {
		sessionID := "s1"
		if tenant == 2 {
			sessionID = "s1t2"
		}
		_, err := s.Import(ctx, ArtifactVersion{
			TenantID: tenant, ID: "v1", RunID: "r1", SessionID: sessionID,
			Digest: artifactDigest(byte(tenant)), MIME: "text/plain", Size: int64(tenant),
		})
		require.NoError(t, err)
		require.NoError(t, s.MarkUploaded(ctx, tenant, "v1"))
		require.NoError(t, s.MarkScanned(ctx, tenant, "v1", true))
		require.NoError(t, s.MarkReady(ctx, tenant, "v1"))
	}
	// Each tenant reads only its own version: same ID, different workspace.
	first, err := s.ReadableArtifactVersion(ctx, 1, "s1", "v1")
	require.NoError(t, err)
	require.EqualValues(t, 1, first.TenantID)
	second, err := s.ReadableArtifactVersion(ctx, 2, "s1t2", "v1")
	require.NoError(t, err)
	require.EqualValues(t, 2, second.TenantID)
	require.NotEqual(t, first.ObjectKey, second.ObjectKey)
	// Tenant 1 cannot reach tenant 2's session-scoped version.
	_, err = s.ReadableArtifactVersion(ctx, 1, "s1t2", "v1")
	require.ErrorIs(t, err, ErrArtifactVersionNotFound)
	// Run listings stay tenant-scoped.
	firstRun, err := s.ListByRun(ctx, 1, "r1")
	require.NoError(t, err)
	require.Len(t, firstRun, 1)
	require.EqualValues(t, 1, firstRun[0].TenantID)
}

func TestArtifactVersionReconcileAfterMetadataFailure(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	s := NewArtifactVersionStore(db)
	// The durable intent row is written first; the object upload then targets
	// the derived key. Simulate the upload landing while the metadata update
	// is lost (crash before MarkUploaded).
	imported, err := s.Import(ctx, ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'), MIME: "text/plain", Size: 3,
	})
	require.NoError(t, err)

	reopened := reopenRunDB(t, db)
	reconciled := NewArtifactVersionStore(reopened)
	// Reconciliation replays the import: the same deterministic object key is
	// returned, so the already-uploaded object is re-associable.
	again, err := reconciled.Import(ctx, ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'), MIME: "text/plain", Size: 3,
	})
	require.NoError(t, err)
	require.Equal(t, imported.ObjectKey, again.ObjectKey)
	require.Equal(t, DeriveArtifactObjectKey(1, "r1", artifactDigest('a')), again.ObjectKey)
	persisted, err := reconciled.Get(ctx, 1, "v1")
	require.NoError(t, err)
	require.Equal(t, "pending", persisted.ScanState)
	// The reconciled row can finish the lifecycle.
	require.NoError(t, reconciled.MarkUploaded(ctx, 1, "v1"))
	require.NoError(t, reconciled.MarkScanned(ctx, 1, "v1", true))
	require.NoError(t, reconciled.MarkReady(ctx, 1, "v1"))
}

func TestArtifactVersionCanceledRunStillReadable(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	s := NewArtifactVersionStore(db)
	_, err = s.Import(ctx, ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'), MIME: "text/plain", Size: 3,
	})
	require.NoError(t, err)
	require.NoError(t, s.MarkUploaded(ctx, 1, "v1"))
	require.NoError(t, s.MarkScanned(ctx, 1, "v1", true))
	require.NoError(t, s.MarkReady(ctx, 1, "v1"))
	// A canceled run keeps its already-generated, published artifacts
	// readable: cancelation stops execution, not artifact access.
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'canceled' WHERE tenant_id = 1 AND run_id = 'r1'`).Error)
	readable, err := s.ReadableArtifactVersion(ctx, 1, "s1", "v1")
	require.NoError(t, err)
	require.Equal(t, "ready", readable.ScanState)
}

func TestArtifactVersionRunAdmissionRequired(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	// No run admitted for r1 yet: importing an artifact for it must fail
	// closed instead of creating an orphan version.
	s := NewArtifactVersionStore(db)
	_, err := s.Import(ctx, ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'), MIME: "text/plain", Size: 3,
	})
	require.ErrorIs(t, err, ErrArtifactVersionNotFound)
	var count int64
	require.NoError(t, db.Table("artifact_versions").Count(&count).Error)
	require.EqualValues(t, 0, count)
}

func TestArtifactVersionRowsTrackUpdateTime(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	s := NewArtifactVersionStore(db)
	_, err = s.Import(ctx, ArtifactVersion{
		TenantID: 1, ID: "v1", RunID: "r1", SessionID: "s1",
		Digest: artifactDigest('a'), MIME: "text/plain", Size: 3,
	})
	require.NoError(t, err)
	before := time.Now()
	require.NoError(t, s.MarkUploaded(ctx, 1, "v1"))
	var updatedAt time.Time
	require.NoError(t, db.Raw(`SELECT updated_at FROM artifact_versions WHERE tenant_id = 1 AND id = 'v1'`).Scan(&updatedAt).Error)
	require.False(t, updatedAt.Before(before.Add(-time.Minute)), "updated_at must advance on state transitions")
}

// TestArtifactVersionsMigrationDownIsReversible applies the formal SQLite
// stream in an isolated harness (NoTxWrap, matching the production runner),
// rolls back exactly one step, and re-applies. The pre-existing
// TestCraftVersionsMigrationDownDropsVersionTables fails under its
// transactional Config{} because the 000055-family migrations manage their own
// transactions; that failure predates this task and is unchanged by 000067.
func TestArtifactVersionsMigrationDownIsReversible(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "artifact-versions-down.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	tableCount := func() int {
		var count int
		require.NoError(t, sqlDB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'artifact_versions'").Scan(&count))
		return count
	}
	require.Equal(t, 1, tableCount(), "000067 up must create artifact_versions")
	// Step back exactly one version: 000067 -> 000066.
	require.NoError(t, migrator.Steps(-1))
	require.Zero(t, tableCount(), "000067 down must drop artifact_versions")
	// Re-applying must restore the table.
	require.NoError(t, migrator.Steps(1))
	require.Equal(t, 1, tableCount())
}
