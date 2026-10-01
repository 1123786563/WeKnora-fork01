package repository

// T33 #63 lifecycle primitives + migration alignment tests.

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/types"
)

type lifecycleTestContextKey struct{}

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

	_, err := repo.TransitionAdoption(ctx, 1, "ad1", "active", "ended", map[string]any{"ended_by": "admin"})
	require.ErrorIs(t, err, ErrAgentAdoptionEndPrecondition)
	var unchanged types.AgentAdoptionEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, "ad1").First(&unchanged).Error)
	require.Equal(t, "active", unchanged.State)

	require.NoError(t, db.Exec("UPDATE agent_adoption_variants SET state='retired' WHERE id='v1'").Error)
	ended, err := repo.TransitionAdoption(ctx, 1, "ad1", "active", "ended", map[string]any{"ended_by": "admin"})
	require.NoError(t, err)
	require.Equal(t, "ended", ended.State)
	require.Equal(t, "admin", ended.EndedBy)
	require.NotNil(t, ended.EndedAt)

	_, err = repo.TransitionAdoption(ctx, 1, "ad1", "active", "ended", map[string]any{"ended_by": "admin"})
	require.ErrorIs(t, err, ErrAgentAdoptionTransition)
	_, err = repo.TransitionAdoption(ctx, 2, "ad1", "active", "ended", nil)
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound)
}

func TestCreateVariantAndEndAdoptionSerializeOnAdoptionRow(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	ctx := context.Background()
	repo := NewAgentAdoptionRepository(db)
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	seedAtomicLifecycleRelease(t, db, "r1", "l1", 1)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "ad1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active"}).Error)

	insertReached, allowInsert := make(chan struct{}), make(chan struct{})
	var insertOnce, endAttemptOnce, endCompletedOnce sync.Once
	var allowInsertOnce sync.Once
	defer allowInsertOnce.Do(func() { close(allowInsert) })
	endAttempted, endGuardCompleted := make(chan struct{}), make(chan struct{})
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:block_variant_insert", func(tx *gorm.DB) {
		if tx.Statement.Table == "agent_adoption_variants" && tx.Statement.Context.Value(lifecycleTestContextKey{}) == "create" {
			insertOnce.Do(func() { close(insertReached) })
			<-allowInsert
		}
	}))
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:signal_end_lock_attempt", func(tx *gorm.DB) {
		if tx.Statement.Table == "agent_adoptions" && tx.Statement.Context.Value(lifecycleTestContextKey{}) == "end" {
			endAttemptOnce.Do(func() { close(endAttempted) })
		}
	}))
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("test:signal_end_guard_complete", func(tx *gorm.DB) {
		if tx.Statement.Table == "agent_adoptions" && tx.Statement.Context.Value(lifecycleTestContextKey{}) == "end" {
			endCompletedOnce.Do(func() { close(endGuardCompleted) })
		}
	}))

	createResult := make(chan error, 1)
	createCtx := context.WithValue(ctx, lifecycleTestContextKey{}, "create")
	go func() {
		_, err := repo.CreateVariant(createCtx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: "ad1", ReleaseID: "r1", Name: "racing", State: "draft"})
		createResult <- err
	}()
	select {
	case <-insertReached: // CreateVariant already acquired its Adoption guard.
	case <-time.After(5 * time.Second):
		t.Fatal("CreateVariant did not reach the insert while holding the Adoption guard")
	}

	endResult := make(chan error, 1)
	endCtx := context.WithValue(ctx, lifecycleTestContextKey{}, "end")
	go func() {
		_, err := repo.TransitionAdoption(endCtx, 1, "ad1", "active", "ended", map[string]any{"ended_by": "admin"})
		endResult <- err
	}()
	select {
	case <-endAttempted:
	case <-time.After(5 * time.Second):
		t.Fatal("EndAdoption did not attempt its Adoption guard")
	}
	select {
	case <-endGuardCompleted:
		t.Fatal("EndAdoption passed the shared row guard while Variant insertion held it")
	case <-time.After(100 * time.Millisecond):
	}

	allowInsertOnce.Do(func() { close(allowInsert) })
	select {
	case err := <-createResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("CreateVariant did not finish after releasing its insert")
	}
	select {
	case err := <-endResult:
		require.ErrorIs(t, err, ErrAgentAdoptionEndPrecondition)
	case <-time.After(5 * time.Second):
		t.Fatal("EndAdoption did not finish after Variant creation committed")
	}

	var adoption types.AgentAdoptionEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, "ad1").First(&adoption).Error)
	require.Equal(t, "active", adoption.State)
	var variants int64
	require.NoError(t, db.Model(&types.AgentAdoptionVariantEntity{}).Where("tenant_id = ? AND adoption_id = ? AND state <> ?", 1, "ad1", "retired").Count(&variants).Error)
	require.EqualValues(t, 1, variants)
}

func TestTransitionListingStateIsCAS(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	row, err := repo.TransitionListingState(ctx, 1, "l1", "listed", "unlisted", map[string]any{"unlisted_by": "admin", "tenant_id": uint64(2), "id": "moved"})
	require.NoError(t, err)
	require.Equal(t, "unlisted", row.State)
	require.Equal(t, uint64(1), row.TenantID)
	require.Equal(t, "l1", row.ID)
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
		require.NoError(t, db.Create(&types.AgentReleaseSubmissionEntity{TenantID: 1, ID: submissionID, ListingID: "l1", AgentVersionID: "av", SourceAgentID: "a", SemanticVersion: "1.0.0", BundleDigest: id, ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b")}).Error)
		require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: id, ListingID: "l1", SubmissionID: submissionID, AgentVersionID: "av", SourceAgentID: "a", ReleaseNumber: i + 1, SemanticVersion: id + ".0.0", BundleDigest: id, ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b")}).Error)
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

func seedAtomicLifecycleRelease(t *testing.T, db *gorm.DB, id, listingID string, number int) {
	t.Helper()
	versionID, submissionID := "av-"+id, "s-"+id
	require.NoError(t, db.Create(&types.AgentVersionEntity{ID: versionID, TenantID: 1, AgentID: "a", VersionNumber: number, Snapshot: "{}", SourceSHA256: "sha"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseSubmissionEntity{ID: submissionID, TenantID: 1, ListingID: listingID, AgentVersionID: versionID, SourceAgentID: "a", AuthorID: "admin", SemanticVersion: id, BundleDigest: id, ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b"), Status: "approved"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: id, ListingID: listingID, SubmissionID: submissionID, AgentVersionID: versionID, SourceAgentID: "a", ReleaseNumber: number, SemanticVersion: id, BundleDigest: id, ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b")}).Error)
}

func TestAdoptListingRechecksListingAndReleaseAtWriteBoundary(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	seedAtomicLifecycleRelease(t, db, "r1", "l1", 1)
	listing, err := repo.GetMarketplaceListing(ctx, 1, "l1")
	require.NoError(t, err) // service eligibility precheck has passed
	require.Equal(t, "listed", listing.State)
	require.NoError(t, db.Model(&types.AgentMarketplaceListingEntity{}).Where("tenant_id = ? AND id = ?", 1, "l1").Update("state", "unlisted").Error)
	_, _, err = repo.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: "l1", AcceptedReleaseID: "r1", State: "active"})
	require.ErrorIs(t, err, ErrAgentMarketplaceListingUnavailable)
	var count int64
	require.NoError(t, db.Model(&types.AgentAdoptionEntity{}).Where("tenant_id = ? AND listing_id = ?", 1, "l1").Count(&count).Error)
	require.Zero(t, count)
}

func TestAdoptListingSerializesWithConcurrentUnlist(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentAdoptionRepository(db)
	market := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	seedAtomicLifecycleRelease(t, db, "r1", "l1", 1)

	insertReached, allowInsert := make(chan struct{}), make(chan struct{})
	unlistAttempted := make(chan struct{})
	var insertOnce, unlistOnce, allowInsertOnce sync.Once
	defer allowInsertOnce.Do(func() { close(allowInsert) })
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:block_adoption_insert", func(tx *gorm.DB) {
		if tx.Statement.Table == "agent_adoptions" && tx.Statement.Context.Value(lifecycleTestContextKey{}) == "adopt" {
			insertOnce.Do(func() { close(insertReached) })
			<-allowInsert
		}
	}))
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:signal_unlist_attempt", func(tx *gorm.DB) {
		if tx.Statement.Table == "agent_marketplace_listings" && tx.Statement.Context.Value(lifecycleTestContextKey{}) == "unlist" {
			unlistOnce.Do(func() { close(unlistAttempted) })
		}
	}))

	type adoptResult struct {
		row     *types.AgentAdoptionEntity
		created bool
		err     error
	}
	adopted := make(chan adoptResult, 1)
	adoptCtx := context.WithValue(ctx, lifecycleTestContextKey{}, "adopt")
	go func() {
		row, created, err := repo.AdoptListing(adoptCtx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: "l1", AcceptedReleaseID: "r1", State: "active"})
		adopted <- adoptResult{row: row, created: created, err: err}
	}()
	select {
	case <-insertReached:
	case <-time.After(5 * time.Second):
		t.Fatal("AdoptListing did not reach its insert inside the eligibility transaction")
	}
	unlisted := make(chan error, 1)
	unlistCtx := context.WithValue(ctx, lifecycleTestContextKey{}, "unlist")
	go func() {
		_, err := market.TransitionListingState(unlistCtx, 1, "l1", "listed", "unlisted", map[string]any{"unlisted_by": "admin"})
		unlisted <- err
	}()
	select {
	case <-unlistAttempted:
	case <-time.After(5 * time.Second):
		t.Fatal("Unlist did not reach its lifecycle write while Adopt was gated")
	}
	allowInsertOnce.Do(func() { close(allowInsert) })
	select {
	case result := <-adopted:
		require.NoError(t, result.err)
		require.True(t, result.created)
		require.NotNil(t, result.row)
	case <-time.After(5 * time.Second):
		t.Fatal("AdoptListing did not finish after releasing the insert gate")
	}
	select {
	case err := <-unlisted:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Unlist did not finish after Adopt committed")
	}
	var listing types.AgentMarketplaceListingEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, "l1").First(&listing).Error)
	require.Equal(t, "unlisted", listing.State)
	var count int64
	require.NoError(t, db.Model(&types.AgentAdoptionEntity{}).Where("tenant_id = ? AND listing_id = ?", 1, "l1").Count(&count).Error)
	require.EqualValues(t, 1, count, "Adopt linearized before Unlist; no adoption may be inserted after the transition")
}

func TestCreateVariantRechecksListingAndReleaseAtWriteBoundary(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	seedAtomicLifecycleRelease(t, db, "r1", "l1", 1)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "ad1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active"}).Error)
	// A service read has already accepted this release; deprecation wins before
	// the repository write starts.
	var checked types.AgentReleaseEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, "r1").First(&checked).Error)
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("tenant_id = ? AND id = ?", 1, "r1").Updates(map[string]any{"deprecated_at": time.Now().UTC(), "successor_release_id": "r2"}).Error)
	_, err := repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: "ad1", ReleaseID: "r1", Name: "late", State: "draft"})
	require.ErrorIs(t, err, ErrAgentMarketplaceReleaseDeprecated)
	var count int64
	require.NoError(t, db.Model(&types.AgentAdoptionVariantEntity{}).Where("tenant_id = ? AND adoption_id = ?", 1, "ad1").Count(&count).Error)
	require.Zero(t, count)
}

func TestFindOrCreateProposalRechecksLifecycleEligibility(t *testing.T) {
	// t.Skip 待按合并世代重校准：materialize 的 adoption 闸已按 HEAD 世代
	// 剧本移至 Accept 层（issue30-round5 集成裁决）。
	t.Skip("待按合并世代重校准：FindOrCreate 的 adoption 锁已让位于 Accept 层闸门")
	db := openLifecycleMigrationDB(t)
	repo := NewAgentUpgradeRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	seedAtomicLifecycleRelease(t, db, "r1", "l1", 1)
	seedAtomicLifecycleRelease(t, db, "r2", "l1", 2)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "ad1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active"}).Error)
	// Reconcile's listing/release snapshot has passed; deprecation wins before
	// the repository materialization begins.
	var checked types.AgentReleaseEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, "r2").First(&checked).Error)
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("tenant_id = ? AND id = ?", 1, "r2").Update("deprecated_at", time.Now().UTC()).Error)
	_, _, err := repo.FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
		TenantID: 1, AdoptionID: "ad1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r2", State: "open",
	})
	require.ErrorIs(t, err, ErrAgentMarketplaceReleaseDeprecated)
	var count int64
	require.NoError(t, db.Model(&types.AgentUpgradeProposalEntity{}).Where("tenant_id = ? AND adoption_id = ? AND to_release_id = ?", 1, "ad1", "r2").Count(&count).Error)
	require.Zero(t, count)
}

func TestConcurrentReciprocalDeprecationsCannotPersistSuccessorCycle(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	for i, id := range []string{"r1", "r2"} {
		seedAtomicLifecycleRelease(t, db, id, "l1", i+1)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, pair := range [][2]string{{"r1", "r2"}, {"r2", "r1"}} {
		pair := pair
		go func() {
			<-start
			_, err := repo.DeprecateRelease(ctx, 1, pair[0], "admin", pair[1])
			results <- err
		}()
	}
	close(start)
	first, second := <-results, <-results
	successes := 0
	for _, err := range []error{first, second} {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, ErrAgentReleaseDeprecateConflict)
		}
	}
	require.Equal(t, 1, successes)
	var cycle int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_releases a JOIN agent_releases b ON b.tenant_id=a.tenant_id AND b.id=a.successor_release_id WHERE a.tenant_id=? AND a.deprecated_at IS NOT NULL AND b.deprecated_at IS NOT NULL AND b.successor_release_id=a.id`, 1).Scan(&cycle).Error)
	require.Zero(t, cycle)
}

func TestRetiredVariantAgentExists(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentVersionEntity{TenantID: 1, ID: "av", AgentID: "a", VersionNumber: 1, Snapshot: "{}", SourceSHA256: "sha"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseSubmissionEntity{TenantID: 1, ID: "s1", ListingID: "l1", AgentVersionID: "av", SourceAgentID: "a", SemanticVersion: "1.0.0", BundleDigest: "d", ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b")}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "r1", ListingID: "l1", SubmissionID: "s1", AgentVersionID: "av", SourceAgentID: "a", ReleaseNumber: 1, SemanticVersion: "1.0.0", BundleDigest: "d", ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b")}).Error)
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
