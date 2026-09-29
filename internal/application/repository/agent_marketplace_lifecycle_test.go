package repository

// T33 #63 lifecycle primitives + migration alignment tests.

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/types"
)

// openLifecycleMigrationDB applies the REAL sqlite migration stream so the
// lifecycle columns are the production schema, not an AutoMigrate sketch.
func openLifecycleMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "lifecycle.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, m.Up())
	_, _ = m.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestEndAdoptionRequiresAllVariantsRetiredAndIsTransactional(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "ad1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active", CreatedBy: "admin"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{TenantID: 1, ID: "v1", AdoptionID: "ad1", ReleaseID: "r1", Name: "sales", State: "published"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{TenantID: 1, ID: "v2", AdoptionID: "ad1", ReleaseID: "r1", Name: "legal", State: "retired", RetiredBy: "admin"}).Error)

	_, err := repo.EndAdoption(ctx, 1, "ad1", "active", "ended", map[string]any{"ended_by": "admin"})
	require.ErrorIs(t, err, ErrAgentAdoptionEndPrecondition)
	var unchanged types.AgentAdoptionEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, "ad1").First(&unchanged).Error)
	require.Equal(t, "active", unchanged.State)

	require.NoError(t, db.Exec("UPDATE agent_adoption_variants SET state='retired' WHERE id='v1'").Error)
	ended, err := repo.EndAdoption(ctx, 1, "ad1", "active", "ended", map[string]any{"ended_by": "admin"})
	require.NoError(t, err)
	require.Equal(t, "ended", ended.State)
	require.Equal(t, "admin", ended.EndedBy)
	require.NotNil(t, ended.EndedAt)

	_, err = repo.EndAdoption(ctx, 1, "ad1", "active", "ended", map[string]any{"ended_by": "admin"})
	require.ErrorIs(t, err, ErrAgentAdoptionTransition)
	_, err = repo.EndAdoption(ctx, 2, "ad1", "active", "ended", nil)
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound)
}

func TestTransitionListingStateIsCAS(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	row, err := repo.TransitionListingState(ctx, 1, "l1", "listed", "unlisted", map[string]any{"unlisted_by": "admin"})
	require.NoError(t, err)
	require.Equal(t, "unlisted", row.State)
	require.Equal(t, "admin", row.UnlistedBy)
	require.NotNil(t, row.UnlistedAt)
	_, err = repo.TransitionListingState(ctx, 1, "l1", "listed", "unlisted", nil)
	require.ErrorIs(t, err, ErrAgentMarketplaceListingTransition)
	_, err = repo.TransitionListingState(ctx, 2, "l1", "listed", "unlisted", nil)
	require.ErrorIs(t, err, ErrAgentMarketplaceNotFound)
}

func TestDeprecateReleaseIsCASAndPointsAtSuccessor(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentVersionEntity{TenantID: 1, ID: "av", AgentID: "a", VersionNumber: 1, Snapshot: "{}", SourceSHA256: "sha"}).Error)
	for i, id := range []string{"r1", "r2"} {
		submissionID := "s" + id
		require.NoError(t, db.Create(&types.AgentReleaseSubmissionEntity{TenantID: 1, ID: submissionID, ListingID: "l1", AgentVersionID: "av", SourceAgentID: "a", SemanticVersion: "1.0.0", BundleDigest: id, ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")}).Error)
		require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: id, ListingID: "l1", SubmissionID: submissionID, AgentVersionID: "av", SourceAgentID: "a", ReleaseNumber: i + 1, SemanticVersion: id + ".0.0", BundleDigest: id, ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")}).Error)
	}
	row, err := repo.DeprecateRelease(ctx, 1, "r1", "admin", "r2")
	require.NoError(t, err)
	require.Equal(t, "r2", row.SuccessorReleaseID)
	require.Equal(t, "admin", row.DeprecatedBy)
	require.NotNil(t, row.DeprecatedAt)
	_, err = repo.DeprecateRelease(ctx, 1, "r1", "admin", "r2")
	require.ErrorIs(t, err, ErrAgentReleaseDeprecateConflict)
	_, err = repo.DeprecateRelease(ctx, 2, "r1", "admin", "r2")
	require.ErrorIs(t, err, ErrAgentMarketplaceNotFound)
}

func TestRetiredVariantAgentExists(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentVersionEntity{TenantID: 1, ID: "av", AgentID: "a", VersionNumber: 1, Snapshot: "{}", SourceSHA256: "sha"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseSubmissionEntity{TenantID: 1, ID: "s1", ListingID: "l1", AgentVersionID: "av", SourceAgentID: "a", SemanticVersion: "1.0.0", BundleDigest: "d", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "r1", ListingID: "l1", SubmissionID: "s1", AgentVersionID: "av", SourceAgentID: "a", ReleaseNumber: 1, SemanticVersion: "1.0.0", BundleDigest: "d", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "ad1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{TenantID: 1, ID: "v1", AdoptionID: "ad1", ReleaseID: "r1", Name: "sales", State: "retired", LocalAgentID: "agent-x"}).Error)
	ok, err := repo.RetiredVariantAgentExists(ctx, 1, "agent-x")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = repo.RetiredVariantAgentExists(ctx, 1, "agent-other")
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = repo.RetiredVariantAgentExists(ctx, 2, "agent-x")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestAgentMarketplaceLifecycleMigrationColumns(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	expect := map[string]map[string]bool{
		"agent_marketplace_listings": {"unlisted_at": true, "unlisted_by": true},
		"agent_releases":             {"deprecated_at": true, "deprecated_by": true, "successor_release_id": true},
		"agent_adoptions":            {"ended_at": true, "ended_by": true},
		"agent_adoption_variants":    {"retired_at": true, "retired_by": true},
	}
	for table, columns := range expect {
		rows, err := db.Raw("SELECT name FROM pragma_table_info(?)", table).Rows()
		require.NoError(t, err)
		present := map[string]bool{}
		for rows.Next() {
			var name string
			require.NoError(t, rows.Scan(&name))
			present[name] = true
		}
		require.NoError(t, rows.Close())
		for column := range columns {
			require.Truef(t, present[column], "%s.%s 必须由迁移创建", table, column)
		}
	}
}
