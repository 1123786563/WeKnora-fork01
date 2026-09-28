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
	repo := NewAgentEvaluationRepository(db)
	base := types.AgentEvaluationEntity{ID: "ev-invalid", ReleaseID: "release-1", TestSetID: "gold", TestSetVersion: "v1", EnvironmentClass: "standard", EvaluatorID: "admin", EvaluatedAt: time.Now().UTC(), ResultsJSON: `{"status":"pass","checks":[{"code":"manifest_completeness","status":"pass"}]}`}
	for _, mutate := range []func(*types.AgentEvaluationEntity){
		func(v *types.AgentEvaluationEntity) { v.ReleaseID = "" },
		func(v *types.AgentEvaluationEntity) { v.TestSetID = "" },
		func(v *types.AgentEvaluationEntity) { v.TestSetVersion = "" },
		func(v *types.AgentEvaluationEntity) { v.EnvironmentClass = "" },
		func(v *types.AgentEvaluationEntity) { v.EvaluatedAt = time.Time{} },
		func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"pass","checks":[{"code":"unknown","status":"pass"}]}`
		},
		func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"pass","checks":[{"code":"security","status":"unknown"}]}`
		},
		func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"inconclusive","checks":[{"code":"security","status":"not_run"}]}`
		},
		func(v *types.AgentEvaluationEntity) { v.ResultsJSON = `{"status":"pass","checks":[]}` },
		func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"pass","checks":[{"code":"security","status":"pass","message":"freeform"}]}`
		},
		func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"pass","checks":[],"task_id":"secret"}`
		},
		func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"status":"pass","checks":[{"code":"security","status":"pass"}]} {}`
		},
		func(v *types.AgentEvaluationEntity) { v.ResultsJSON = `{` },
	} {
		row := base
		mutate(&row)
		_, err := repo.CreateEvaluation(context.Background(), &row)
		require.Error(t, err)
	}
}

func TestAgentEvaluationAcceptsApprovedCheckAndStatusAllowlist(t *testing.T) {
	db := openRunTestDB(t)
	release := seedPublicEvaluationReleaseFixture(t, db)
	repo := NewAgentEvaluationRepository(db)
	codes := []string{"manifest_completeness", "compatibility", "license", "security", "dependency_integrity", "privacy"}
	statuses := []string{"pass", "fail", "not_run"}
	overallStatuses := []string{"pass", "fail", "inconclusive"}
	for i, code := range codes {
		row := types.AgentEvaluationEntity{ID: "allow-" + code, ReleaseID: release, TestSetID: code, TestSetVersion: "1", EnvironmentClass: "ci", EvaluatorID: "admin", EvaluatedAt: time.Now().UTC(), ResultsJSON: `{"status":"` + overallStatuses[i%len(overallStatuses)] + `","checks":[{"code":"` + code + `","status":"` + statuses[i%len(statuses)] + `"}]}`}
		_, err := repo.CreateEvaluation(context.Background(), &row)
		require.NoError(t, err, "approved check code %s must be accepted", code)
	}
}
