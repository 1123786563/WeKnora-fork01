package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAgentMarketplaceSubmissionReviewPublishAndTenantScope(t *testing.T) {
	db := openRunTestDB(t)
	seedMarketplaceVersion(t, db, "version-a", "agent-a")
	seedMarketplaceVersion(t, db, "version-b", "agent-b")
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
