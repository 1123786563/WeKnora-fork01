package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestAgentEvaluationReleasePinnedImmutableAndUnique(t *testing.T) {
	db := openRunTestDB(t)
	seedMarketplaceVersion(t, db, "eval-version", "eval-agent")
	require.NoError(t, db.Exec(`INSERT INTO agent_marketplace_listings (id,tenant_id,source_agent_id,display_name) VALUES ('eval-listing',1,'eval-agent','Eval')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_release_submissions (id,tenant_id,listing_id,agent_version_id,source_agent_id,semantic_version,bundle_digest,manifest_json,dependency_lock_json,bundle) VALUES ('eval-sub',1,'eval-listing','eval-version','eval-agent','1.0.0','eval-digest','{}','{}','{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_releases (id,tenant_id,listing_id,submission_id,agent_version_id,source_agent_id,release_number,semantic_version,bundle_digest,manifest_json,dependency_lock_json,bundle) VALUES ('eval-source-release',1,'eval-listing','eval-sub','eval-version','eval-agent',1,'1.0.0','eval-digest','{}','{}','{}')`).Error)
	publicRepo := NewPublicMarketplaceRepository(db)
	listing := &types.PublicMarketplaceListingEntity{PublisherTenantID: 1, SourceListingID: "eval-listing", DisplayName: "Eval", State: "listed"}
	sub, err := publicRepo.CreatePublicSubmission(context.Background(), listing, &types.PublicReleaseSubmissionEntity{PublisherTenantID: 1, SourceListingID: "eval-listing", SourceReleaseID: "eval-source-release", SemanticVersion: "1.0.0", BundleDigest: "pub-digest", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("{}")})
	require.NoError(t, err)
	_, releaseOne, err := publicRepo.ReviewAndPublishPublicTx(context.Background(), "", sub.ID, "pub-digest", types.AgentReleaseReviewDecision{ReviewerID: "sysadmin", Decision: "approved"})
	require.NoError(t, err)
	repo := NewAgentEvaluationRepository(db)
	ctx := context.Background()
	row := types.AgentEvaluationEntity{ID: "ev1", ReleaseID: releaseOne.ID, TestSetID: "gold", TestSetVersion: "v3", EnvironmentClass: "standard", EvaluatorID: "admin", EvaluatedAt: time.Now().UTC(), ResultsJSON: `{"checks":[{"code":"task_success","status":"passed"}]}`}
	created, err := repo.CreateEvaluation(ctx, &row)
	require.NoError(t, err)
	require.Equal(t, row.ReleaseID, created.ReleaseID)
	row.ResultsJSON = `{"checks":[{"code":"task_success","status":"failed"}]}`
	_, err = repo.CreateEvaluation(ctx, &row)
	require.ErrorIs(t, err, ErrAgentEvaluationConflict)

	rows, err := repo.ListEvaluationsForReleases(ctx, []string{releaseOne.ID, "different-release"})
	require.NoError(t, err)
	require.Len(t, rows[releaseOne.ID], 1)
	require.Empty(t, rows["different-release"])
	require.Equal(t, releaseOne.ID, rows[releaseOne.ID][0].ReleaseID)
	require.Equal(t, `{"checks":[{"code":"task_success","status":"passed"}]}`, rows[releaseOne.ID][0].ResultsJSON)
}

func TestAgentEvaluationRejectsInvalidRows(t *testing.T) {
	db := openRunTestDB(t)
	repo := NewAgentEvaluationRepository(db)
	base := types.AgentEvaluationEntity{ID: "ev-invalid", ReleaseID: "release-1", TestSetID: "gold", TestSetVersion: "v1", EnvironmentClass: "standard", EvaluatorID: "admin", EvaluatedAt: time.Now().UTC(), ResultsJSON: `{"checks":[{"code":"task_success","status":"passed"}]}`}
	for _, mutate := range []func(*types.AgentEvaluationEntity){
		func(v *types.AgentEvaluationEntity) { v.ReleaseID = "" },
		func(v *types.AgentEvaluationEntity) { v.TestSetID = "" },
		func(v *types.AgentEvaluationEntity) { v.TestSetVersion = "" },
		func(v *types.AgentEvaluationEntity) { v.EnvironmentClass = "" },
		func(v *types.AgentEvaluationEntity) { v.EvaluatedAt = time.Time{} },
		func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"checks":[{"code":"unknown","status":"passed"}]}`
		},
		func(v *types.AgentEvaluationEntity) {
			v.ResultsJSON = `{"checks":[{"code":"task_success","status":"unknown"}]}`
		},
		func(v *types.AgentEvaluationEntity) { v.ResultsJSON = `{"checks":[],"task_id":"secret"}` },
		func(v *types.AgentEvaluationEntity) { v.ResultsJSON = `{` },
	} {
		row := base
		mutate(&row)
		_, err := repo.CreateEvaluation(context.Background(), &row)
		require.Error(t, err)
	}
}
