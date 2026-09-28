package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAgentEvaluationReleasePinnedImmutableAndUnique(t *testing.T) {
	db := openRunTestDB(t)
	releaseID := seedPublicEvaluationReleaseFixture(t, db)
	repo := NewAgentEvaluationRepository(db)
	ctx := context.Background()
	row := types.AgentEvaluationEntity{ID: "ev1", ReleaseID: releaseID, TestSetID: "gold", TestSetVersion: "v3", EnvironmentClass: "standard", EvaluatorID: "admin", EvaluatedAt: time.Now().UTC(), ResultsJSON: `{"status":"pass","checks":[{"code":"manifest_completeness","status":"pass"}]}`}
	created, err := repo.CreateEvaluation(ctx, &row)
	require.NoError(t, err)
	require.Equal(t, row.ReleaseID, created.ReleaseID)
	row.ResultsJSON = `{"status":"fail","checks":[{"code":"manifest_completeness","status":"fail"}]}`
	_, err = repo.CreateEvaluation(ctx, &row)
	require.ErrorIs(t, err, ErrAgentEvaluationConflict)

	rows, err := repo.ListEvaluationsForReleases(ctx, []string{releaseID, "different-release"})
	require.NoError(t, err)
	require.Len(t, rows[releaseID], 1)
	require.Empty(t, rows["different-release"])
	require.Equal(t, releaseID, rows[releaseID][0].ReleaseID)
	require.Equal(t, `{"status":"pass","checks":[{"code":"manifest_completeness","status":"pass"}]}`, rows[releaseID][0].ResultsJSON)

	secondReleaseID := publishSecondPublicEvaluationRelease(t, db, releaseID)
	releases, err := repo.ListEvaluationsForReleases(ctx, []string{releaseID, secondReleaseID})
	require.NoError(t, err)
	require.Len(t, releases[releaseID], 1)
	require.Empty(t, releases[secondReleaseID], "moving the listing pointer must not move prior release evidence")
}

func seedPublicEvaluationReleaseFixture(t *testing.T, db *gorm.DB) string {
	t.Helper()
	seedMarketplaceVersion(t, db, "eval-version", "eval-agent")
	require.NoError(t, db.Exec(`INSERT INTO agent_marketplace_listings (id,tenant_id,source_agent_id,display_name) VALUES ('eval-listing',1,'eval-agent','Eval')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_release_submissions (id,tenant_id,listing_id,agent_version_id,source_agent_id,semantic_version,bundle_digest,manifest_json,dependency_lock_json,bundle) VALUES ('eval-sub',1,'eval-listing','eval-version','eval-agent','1.0.0','eval-digest','{}','{}','{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_releases (id,tenant_id,listing_id,submission_id,agent_version_id,source_agent_id,release_number,semantic_version,bundle_digest,manifest_json,dependency_lock_json,bundle) VALUES ('eval-source-release',1,'eval-listing','eval-sub','eval-version','eval-agent',1,'1.0.0','eval-digest','{}','{}','{}')`).Error)
	publicRepo := NewPublicMarketplaceRepository(db)
	listing := &types.PublicMarketplaceListingEntity{PublisherTenantID: 1, SourceListingID: "eval-listing", DisplayName: "Eval", State: "listed"}
	sub, err := publicRepo.CreatePublicSubmission(context.Background(), listing, &types.PublicReleaseSubmissionEntity{PublisherTenantID: 1, SourceListingID: "eval-listing", SourceReleaseID: "eval-source-release", SemanticVersion: "1.0.0", BundleDigest: "pub-digest", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("{}")})
	require.NoError(t, err)
	_, release, err := publicRepo.ReviewAndPublishPublicTx(context.Background(), "", sub.ID, "pub-digest", types.AgentReleaseReviewDecision{ReviewerID: "sysadmin", Decision: "approved"})
	require.NoError(t, err)
	return release.ID
}

func TestAgentEvaluationRejectsInvalidRows(t *testing.T) {
	db := openRunTestDB(t)
	releaseID := seedPublicEvaluationReleaseFixture(t, db)
	repo := NewAgentEvaluationRepository(db)
	base := types.AgentEvaluationEntity{ID: "ev-invalid", ReleaseID: releaseID, TestSetID: "gold", TestSetVersion: "v1", EnvironmentClass: "standard", EvaluatorID: "admin", EvaluatedAt: time.Now().UTC(), ResultsJSON: `{"status":"pass","checks":[{"code":"manifest_completeness","status":"pass"}]}`}
	cases := []struct {
		name   string
		mutate func(*types.AgentEvaluationEntity)
	}{
		{name: "blank release", mutate: func(v *types.AgentEvaluationEntity) { v.ReleaseID = "" }},
		{name: "blank test set", mutate: func(v *types.AgentEvaluationEntity) { v.TestSetID = "" }},
		{name: "blank test version", mutate: func(v *types.AgentEvaluationEntity) { v.TestSetVersion = "" }},
		{name: "blank environment", mutate: func(v *types.AgentEvaluationEntity) { v.EnvironmentClass = "" }},
		{name: "zero timestamp", mutate: func(v *types.AgentEvaluationEntity) { v.EvaluatedAt = time.Time{} }},
		{name: "unknown check code", mutate: func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"pass","checks":[{"code":"unknown","status":"pass"}]}`
		}},
		{name: "unknown check status", mutate: func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"pass","checks":[{"code":"security","status":"unknown"}]}`
		}},
		{name: "unknown overall status", mutate: func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"unknown","checks":[{"code":"security","status":"not_run"}]}`
		}},
		{name: "empty checks", mutate: func(v *types.AgentEvaluationEntity) { v.ResultsJSON = `{"status":"pass","checks":[]}` }},
		{name: "freeform check field", mutate: func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"pass","checks":[{"code":"security","status":"pass","message":"freeform"}]}`
		}},
		{name: "forbidden top-level field", mutate: func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"pass","checks":[],"task_id":"secret"}`
		}},
		{name: "trailing json", mutate: func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"pass","checks":[{"code":"security","status":"pass"}]} {}`
		}},
		{name: "malformed json", mutate: func(v *types.AgentEvaluationEntity) { v.ResultsJSON = `{` }},
	}
	for _, c := range cases {
		row := base
		c.mutate(&row)
		_, err := repo.CreateEvaluation(context.Background(), &row)
		require.ErrorIsf(t, err, ErrAgentEvaluationInvalid, "case %s", c.name)
	}
}

func TestAgentEvaluationAcceptsApprovedCheckAndStatusAllowlist(t *testing.T) {
	db := openRunTestDB(t)
	release := seedPublicEvaluationReleaseFixture(t, db)
	repo := NewAgentEvaluationRepository(db)
	codes := []string{"manifest_completeness", "compatibility", "license", "security", "dependency_integrity", "privacy"}
	for _, code := range codes {
		row := types.AgentEvaluationEntity{ID: "allow-" + code, ReleaseID: release, TestSetID: code, TestSetVersion: "1", EnvironmentClass: "ci", EvaluatorID: "admin", EvaluatedAt: time.Now().UTC(), ResultsJSON: `{"status":"inconclusive","checks":[{"code":"` + code + `","status":"not_run"}]}`}
		_, err := repo.CreateEvaluation(context.Background(), &row)
		require.NoError(t, err, "approved check code %s with not_run result must be accepted", code)
	}
}

func publishSecondPublicEvaluationRelease(t *testing.T, db *gorm.DB, priorReleaseID string) string {
	t.Helper()
	// Reuse the fixture listing and create a second immutable tenant Release.
	seedMarketplaceVersionNumber(t, db, "eval-version-2", "eval-agent", 1, 2)
	require.NoError(t, db.Exec(`INSERT INTO agent_release_submissions (id,tenant_id,listing_id,agent_version_id,source_agent_id,semantic_version,bundle_digest,manifest_json,dependency_lock_json,bundle) VALUES ('eval-sub-2',1,'eval-listing','eval-version-2','eval-agent','2.0.0','eval-digest-2','{}','{}','{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_releases (id,tenant_id,listing_id,submission_id,agent_version_id,source_agent_id,release_number,semantic_version,bundle_digest,manifest_json,dependency_lock_json,bundle) VALUES ('eval-source-release-2',1,'eval-listing','eval-sub-2','eval-version-2','eval-agent',2,'2.0.0','eval-digest-2','{}','{}','{}')`).Error)
	publicRepo := NewPublicMarketplaceRepository(db)
	first, err := publicRepo.GetPublicRelease(context.Background(), priorReleaseID)
	require.NoError(t, err)
	sub, err := publicRepo.CreatePublicSubmission(context.Background(), nil, &types.PublicReleaseSubmissionEntity{PublisherTenantID: 1, PublicListingID: first.ListingID, SourceListingID: "eval-listing", SourceReleaseID: "eval-source-release-2", SemanticVersion: "2.0.0", BundleDigest: "pub-digest-2", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("{}")})
	require.NoError(t, err)
	_, second, err := publicRepo.ReviewAndPublishPublicTx(context.Background(), priorReleaseID, sub.ID, "pub-digest-2", types.AgentReleaseReviewDecision{ReviewerID: "sysadmin", Decision: "approved"})
	require.NoError(t, err)
	listing, err := publicRepo.GetPublicListing(context.Background(), first.ListingID)
	require.NoError(t, err)
	require.NotNil(t, listing.CurrentReleaseID)
	require.Equal(t, second.ID, *listing.CurrentReleaseID)
	return second.ID
}
