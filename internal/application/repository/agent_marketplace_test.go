package repository

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAgentMarketplaceSubmissionReviewPublishAndTenantScope(t *testing.T) {
	db := openRunTestDB(t)
	seedMarketplaceVersion(t, db, "version-a", "agent-a")
	seedMarketplaceVersionNumber(t, db, "version-b", "agent-a", 1, 2)
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	listing := &types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-a", DisplayName: "Assistant", Summary: "summary", State: "listed"}
	submission := &types.AgentReleaseSubmissionEntity{TenantID: 1, AgentVersionID: "version-a", SourceAgentID: "agent-a", AuthorID: "author", SemanticVersion: "1.0.0", BundleDigest: "digest-a", ManifestJSON: `{"name":"one"}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte(`{"content":"one"}`), Status: "submitted"}
	created, err := repo.CreateSubmission(ctx, listing, submission)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	queue, err := repo.ListReviewQueue(ctx, 1)
	require.NoError(t, err)
	require.Len(t, queue, 1)
	require.Equal(t, "digest-a", queue[0].BundleDigest)

	review, release, err := repo.ReviewAndPublishTx(ctx, 1, "", created.ID, "digest-a", types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved", Reason: "ok"})
	require.NoError(t, err)
	require.NotEmpty(t, review.ID)
	require.NotNil(t, release)
	require.Equal(t, 1, release.ReleaseNumber)
	retryReview, retryRelease, err := repo.ReviewAndPublishTx(ctx, 1, "", created.ID, "digest-a", types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved", Reason: "ok"})
	require.NoError(t, err, "retrying the same reviewer decision must return its durable result")
	require.Equal(t, review.ID, retryReview.ID)
	require.Equal(t, release.ID, retryRelease.ID)
	var storedStatus string
	require.NoError(t, db.Model(&types.AgentReleaseSubmissionEntity{}).Where("tenant_id = ? AND id = ?", 1, created.ID).Select("status").Scan(&storedStatus).Error)
	require.Equal(t, "submitted", storedStatus, "review outcome must be appended without mutating the Submission")
	queue, err = repo.ListReviewQueue(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, queue)

	// Review history is append-only, each Listing owns its own release sequence,
	// and an older immutable Release survives pointer advancement.
	second := *submission
	second.AgentVersionID = "version-b"
	second.SemanticVersion = "2.0.0"
	second.BundleDigest = "digest-b"
	second.ManifestJSON = `{"name":"two"}`
	second.Bundle = []byte(`{"content":"two"}`)
	secondCreated, err := repo.CreateSubmission(ctx, nil, &second)
	require.NoError(t, err)
	_, release2, err := repo.ReviewAndPublishTx(ctx, 1, release.ID, secondCreated.ID, "digest-b", types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.Equal(t, 2, release2.ReleaseNumber)
	catalog, err := repo.ListTenantCatalog(ctx, 1)
	require.NoError(t, err)
	require.Len(t, catalog, 1)
	require.Equal(t, release2.ID, *catalog[0].CurrentReleaseID)
	old, err := repo.GetRelease(ctx, 1, release.ID)
	require.NoError(t, err)
	require.Equal(t, []byte(`{"content":"one"}`), old.Bundle)
	_, err = repo.ListReviewQueue(ctx, 2)
	require.NoError(t, err)
}

func TestAgentMarketplacePublishRejectsStaleDigestAndPointer(t *testing.T) {
	db := openRunTestDB(t)
	seedMarketplaceVersion(t, db, "version-a", "agent-a")
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	submission, err := repo.CreateSubmission(ctx, &types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-a", DisplayName: "A", State: "listed"}, &types.AgentReleaseSubmissionEntity{TenantID: 1, AgentVersionID: "version-a", SourceAgentID: "agent-a", SemanticVersion: "1.0.0", BundleDigest: "digest", ManifestJSON: `{}`, DependencyLockJSON: `{}`, Bundle: []byte(`{}`), Status: "submitted"})
	require.NoError(t, err)
	_, _, err = repo.ReviewAndPublishTx(ctx, 1, "", submission.ID, "wrong", types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.ErrorIs(t, err, ErrAgentMarketplaceDigestMismatch)
	_, _, err = repo.ReviewAndPublishTx(ctx, 1, "not-current", submission.ID, "digest", types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.ErrorIs(t, err, ErrAgentMarketplacePointerConflict)
}

func TestAgentMarketplaceReviewRejectDoesNotPublish(t *testing.T) {
	db := openRunTestDB(t)
	seedMarketplaceVersion(t, db, "version-a", "agent-a")
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	submission, err := repo.CreateSubmission(ctx, &types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-a", DisplayName: "A", State: "listed"}, &types.AgentReleaseSubmissionEntity{TenantID: 1, AgentVersionID: "version-a", SourceAgentID: "agent-a", SemanticVersion: "1.0.0", BundleDigest: "digest", ManifestJSON: `{}`, DependencyLockJSON: `{}`, Bundle: []byte(`{}`), Status: "submitted"})
	require.NoError(t, err)
	review, release, err := repo.ReviewAndPublishTx(ctx, 1, "", submission.ID, "digest", types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "rejected", Reason: "unsafe"})
	require.NoError(t, err)
	require.NotNil(t, review)
	require.Nil(t, release)
	queue, err := repo.ListReviewQueue(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, queue)
}

func seedMarketplaceVersion(t *testing.T, db *gorm.DB, id, agentID string) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES (?, 1, ?, 1, '{}', 'sha', 'author')`, id, agentID).Error)
}

func TestAgentMarketplaceSubmissionIgnoresCallerReleasePointer(t *testing.T) {
	db := openRunTestDB(t)
	seedMarketplaceVersion(t, db, "version-pointer", "agent-pointer")
	repo := NewAgentMarketplaceRepository(db)
	callerPointer := "caller-controlled-release"
	submission, err := repo.CreateSubmission(context.Background(), &types.AgentMarketplaceListingEntity{
		TenantID: 1, SourceAgentID: "agent-pointer", DisplayName: "Pointer", State: "listed", CurrentReleaseID: &callerPointer,
	}, &types.AgentReleaseSubmissionEntity{
		TenantID: 1, AgentVersionID: "version-pointer", SourceAgentID: "agent-pointer", SemanticVersion: "1.0.0", BundleDigest: "pointer-digest", ManifestJSON: `{}`, DependencyLockJSON: `{}`, Bundle: []byte(`{}`),
	})
	require.NoError(t, err)
	listings, err := repo.ListTenantCatalog(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, listings, 1)
	require.Equal(t, submission.ListingID, listings[0].ID)
	require.Nil(t, listings[0].CurrentReleaseID, "new Listing pointer must be server-owned and empty")
}

func TestAgentMarketplaceSubmissionRequiresMatchingVersionAgent(t *testing.T) {
	db := openRunTestDB(t)
	seedMarketplaceVersion(t, db, "version-agent-a", "agent-a")
	repo := NewAgentMarketplaceRepository(db)
	_, err := repo.CreateSubmission(context.Background(), &types.AgentMarketplaceListingEntity{
		TenantID: 1, SourceAgentID: "agent-b", DisplayName: "Wrong source", State: "listed",
	}, &types.AgentReleaseSubmissionEntity{
		TenantID: 1, AgentVersionID: "version-agent-a", SourceAgentID: "agent-b", SemanticVersion: "1.0.0", BundleDigest: "wrong-source", ManifestJSON: `{}`, DependencyLockJSON: `{}`, Bundle: []byte(`{}`),
	})
	require.ErrorIs(t, err, ErrAgentMarketplaceVersionAgentMismatch, "same-Tenant AgentVersion cannot be submitted as another source Agent")
}

func TestAgentMarketplaceTenantIsolationWithPopulatedTenants(t *testing.T) {
	db := openRunTestDB(t)
	seedMarketplaceVersion(t, db, "version-tenant-one", "agent-a")
	seedMarketplaceVersionForTenant(t, db, "version-tenant-two", 2, "agent-a")
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	first, err := repo.CreateSubmission(ctx, &types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-a", DisplayName: "Tenant one", State: "listed"}, &types.AgentReleaseSubmissionEntity{TenantID: 1, AgentVersionID: "version-tenant-one", SourceAgentID: "agent-a", SemanticVersion: "1.0.0", BundleDigest: "tenant-one", ManifestJSON: `{}`, DependencyLockJSON: `{}`, Bundle: []byte(`{"tenant":1}`)})
	require.NoError(t, err)
	second, err := repo.CreateSubmission(ctx, &types.AgentMarketplaceListingEntity{TenantID: 2, SourceAgentID: "agent-a", DisplayName: "Tenant two", State: "listed"}, &types.AgentReleaseSubmissionEntity{TenantID: 2, AgentVersionID: "version-tenant-two", SourceAgentID: "agent-a", SemanticVersion: "1.0.0", BundleDigest: "tenant-two", ManifestJSON: `{}`, DependencyLockJSON: `{}`, Bundle: []byte(`{"tenant":2}`)})
	require.NoError(t, err)
	queueOneBeforeReview, err := repo.ListReviewQueue(ctx, 1)
	require.NoError(t, err)
	require.Len(t, queueOneBeforeReview, 1)
	require.Equal(t, first.ID, queueOneBeforeReview[0].ID)
	queueTwoBeforeReview, err := repo.ListReviewQueue(ctx, 2)
	require.NoError(t, err)
	require.Len(t, queueTwoBeforeReview, 1)
	require.Equal(t, second.ID, queueTwoBeforeReview[0].ID)
	catalogOneBeforeReview, err := repo.ListTenantCatalog(ctx, 1)
	require.NoError(t, err)
	require.Len(t, catalogOneBeforeReview, 1)
	require.Equal(t, first.ListingID, catalogOneBeforeReview[0].ID)
	catalogTwoBeforeReview, err := repo.ListTenantCatalog(ctx, 2)
	require.NoError(t, err)
	require.Len(t, catalogTwoBeforeReview, 1)
	require.Equal(t, second.ListingID, catalogTwoBeforeReview[0].ID)

	_, releaseOne, err := repo.ReviewAndPublishTx(ctx, 1, "", first.ID, "tenant-one", types.AgentReleaseReviewDecision{ReviewerID: "reviewer-one", Decision: "approved"})
	require.NoError(t, err)
	_, releaseTwo, err := repo.ReviewAndPublishTx(ctx, 2, "", second.ID, "tenant-two", types.AgentReleaseReviewDecision{ReviewerID: "reviewer-two", Decision: "approved"})
	require.NoError(t, err)

	queueOne, err := repo.ListReviewQueue(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, queueOne)
	queueTwo, err := repo.ListReviewQueue(ctx, 2)
	require.NoError(t, err)
	require.Empty(t, queueTwo)
	catalogOne, err := repo.ListTenantCatalog(ctx, 1)
	require.NoError(t, err)
	require.Len(t, catalogOne, 1)
	require.Equal(t, first.ListingID, catalogOne[0].ID)
	catalogTwo, err := repo.ListTenantCatalog(ctx, 2)
	require.NoError(t, err)
	require.Len(t, catalogTwo, 1)
	require.Equal(t, second.ListingID, catalogTwo[0].ID)
	crossTenantRelease, err := repo.GetRelease(ctx, 1, releaseTwo.ID)
	require.NoError(t, err)
	require.Nil(t, crossTenantRelease)
	ownedRelease, err := repo.GetRelease(ctx, 2, releaseTwo.ID)
	require.NoError(t, err)
	require.Equal(t, releaseTwo.ID, ownedRelease.ID)
	crossTenantOne, err := repo.GetRelease(ctx, 2, releaseOne.ID)
	require.NoError(t, err)
	require.Nil(t, crossTenantOne)
}

func TestAgentMarketplaceSQLiteDownWithPublishedListing(t *testing.T) {
	db := openRunTestDB(t)
	seedMarketplaceVersion(t, db, "version-down", "agent-down")
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	submission, err := repo.CreateSubmission(ctx, &types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-down", DisplayName: "Down", State: "listed"}, &types.AgentReleaseSubmissionEntity{TenantID: 1, AgentVersionID: "version-down", SourceAgentID: "agent-down", SemanticVersion: "1.0.0", BundleDigest: "down-digest", ManifestJSON: `{}`, DependencyLockJSON: `{}`, Bundle: []byte(`{}`)})
	require.NoError(t, err)
	_, _, err = repo.ReviewAndPublishTx(ctx, 1, "", submission.ID, "down-digest", types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	var foreignKeysEnabled int
	require.NoError(t, db.Raw("PRAGMA foreign_keys").Scan(&foreignKeysEnabled).Error)
	require.Equal(t, 1, foreignKeysEnabled)
	listed, err := repo.ListTenantCatalog(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, listed[0].CurrentReleaseID)

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	down, err := os.ReadFile(filepath.Join(repoRoot, "migrations/sqlite/000100_tenant_agent_marketplace.down.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(down)).Error)
	for _, table := range []string{"agent_marketplace_listings", "agent_release_submissions", "agent_release_reviews", "agent_releases"} {
		require.False(t, db.Migrator().HasTable(table), "down migration must remove %s", table)
	}
	require.True(t, db.Migrator().HasTable("agent_versions"), "rollback must preserve Agent-domain tables")
}

func TestAgentMarketplaceConcurrentPublishingUsesUniqueReleaseNumbers(t *testing.T) {
	db := openRunTestDB(t)
	seedMarketplaceVersionNumber(t, db, "version-concurrent-1", "agent-concurrent", 1, 1)
	seedMarketplaceVersionNumber(t, db, "version-concurrent-2", "agent-concurrent", 1, 2)
	seedMarketplaceVersionNumber(t, db, "version-concurrent-3", "agent-concurrent", 1, 3)
	repoOne := NewAgentMarketplaceRepository(db)
	ctx := context.Background()
	listing := &types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-concurrent", DisplayName: "Concurrent", State: "listed"}
	first, err := repoOne.CreateSubmission(ctx, listing, &types.AgentReleaseSubmissionEntity{TenantID: 1, AgentVersionID: "version-concurrent-1", SourceAgentID: "agent-concurrent", SemanticVersion: "1.0.0", BundleDigest: "concurrent-1", ManifestJSON: `{}`, DependencyLockJSON: `{}`, Bundle: []byte(`{"v":1}`)})
	require.NoError(t, err)
	_, baseRelease, err := repoOne.ReviewAndPublishTx(ctx, 1, "", first.ID, "concurrent-1", types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	second, err := repoOne.CreateSubmission(ctx, nil, &types.AgentReleaseSubmissionEntity{TenantID: 1, AgentVersionID: "version-concurrent-2", SourceAgentID: "agent-concurrent", SemanticVersion: "2.0.0", BundleDigest: "concurrent-2", ManifestJSON: `{}`, DependencyLockJSON: `{}`, Bundle: []byte(`{"v":2}`)})
	require.NoError(t, err)
	third, err := repoOne.CreateSubmission(ctx, nil, &types.AgentReleaseSubmissionEntity{TenantID: 1, AgentVersionID: "version-concurrent-3", SourceAgentID: "agent-concurrent", SemanticVersion: "3.0.0", BundleDigest: "concurrent-3", ManifestJSON: `{}`, DependencyLockJSON: `{}`, Bundle: []byte(`{"v":3}`)})
	require.NoError(t, err)

	var databasePath string
	var databaseList []struct {
		Seq  int
		Name string
		File string
	}
	require.NoError(t, db.Raw("PRAGMA database_list").Scan(&databaseList).Error)
	require.NotEmpty(t, databaseList)
	databasePath = databaseList[0].File
	dbTwo, err := gorm.Open(sqlite.Open("file:"+databasePath+"?_foreign_keys=on&_busy_timeout=5000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	defer func() { conn, _ := dbTwo.DB(); _ = conn.Close() }()
	repoTwo := NewAgentMarketplaceRepository(dbTwo)

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, item := range []struct {
		repo AgentMarketplaceRepository
		sub  *types.AgentReleaseSubmissionEntity
	}{{repoOne, second}, {repoTwo, third}} {
		go func(item struct {
			repo AgentMarketplaceRepository
			sub  *types.AgentReleaseSubmissionEntity
		}) {
			<-start
			_, _, publishErr := item.repo.ReviewAndPublishTx(ctx, 1, baseRelease.ID, item.sub.ID, item.sub.BundleDigest, types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
			results <- publishErr
		}(item)
	}
	close(start)
	firstErr, secondErr := <-results, <-results
	for _, publishErr := range []error{firstErr, secondErr} {
		if publishErr != nil {
			require.ErrorIs(t, publishErr, ErrAgentMarketplacePointerConflict)
		}
	}
	catalog, err := repoOne.ListTenantCatalog(ctx, 1)
	require.NoError(t, err)
	require.Len(t, catalog, 1)
	currentID := *catalog[0].CurrentReleaseID
	for _, pending := range []*types.AgentReleaseSubmissionEntity{second, third} {
		missing, lookupErr := repoOne.GetRelease(ctx, 1, pending.ID) // Submission IDs must never address Release rows.
		require.NoError(t, lookupErr)
		require.Nil(t, missing)
	}
	var releases []types.AgentReleaseEntity
	require.NoError(t, db.Where("tenant_id = ? AND listing_id = ?", 1, first.ListingID).Order("release_number").Find(&releases).Error)
	require.Len(t, releases, 2, "one concurrent contender must win the initial CAS")
	loser := second
	if releases[1].SubmissionID == second.ID {
		loser = third
	}
	_, nextRelease, err := repoOne.ReviewAndPublishTx(ctx, 1, currentID, loser.ID, loser.BundleDigest, types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.Equal(t, 3, nextRelease.ReleaseNumber)
	require.NoError(t, db.Where("tenant_id = ? AND listing_id = ?", 1, first.ListingID).Order("release_number").Find(&releases).Error)
	require.Len(t, releases, 3)
	require.Equal(t, []int{1, 2, 3}, []int{releases[0].ReleaseNumber, releases[1].ReleaseNumber, releases[2].ReleaseNumber})
	finalCatalog, err := repoOne.ListTenantCatalog(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, nextRelease.ID, *finalCatalog[0].CurrentReleaseID)
	original, err := repoOne.GetRelease(ctx, 1, baseRelease.ID)
	require.NoError(t, err)
	require.Equal(t, []byte(`{"v":1}`), original.Bundle)
}

func seedMarketplaceVersionForTenant(t *testing.T, db *gorm.DB, id string, tenantID uint64, agentID string) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES (?, ?, ?, 1, '{}', 'sha', 'author')`, id, tenantID, agentID).Error)
}

func seedMarketplaceVersionNumber(t *testing.T, db *gorm.DB, id, agentID string, tenantID uint64, versionNumber int) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES (?, ?, ?, ?, '{}', 'sha', 'author')`, id, tenantID, agentID, versionNumber).Error)
}
