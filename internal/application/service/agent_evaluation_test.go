package service

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type evaluationRepoFake struct{ row *types.AgentEvaluationEntity }

func (f *evaluationRepoFake) CreateEvaluation(_ context.Context, row *types.AgentEvaluationEntity) (*types.AgentEvaluationEntity, error) {
	copy := *row
	f.row = &copy
	return &copy, nil
}
func (f *evaluationRepoFake) ListEvaluationsForReleases(_ context.Context, ids []string) (map[string][]types.AgentEvaluationEntity, error) {
	result := map[string][]types.AgentEvaluationEntity{}
	for _, id := range ids {
		result[id] = []types.AgentEvaluationEntity{}
	}
	if f.row != nil {
		result[f.row.ReleaseID] = append(result[f.row.ReleaseID], *f.row)
	}
	return result, nil
}

func TestAgentEvaluationServiceBindsReviewerAndReturnsReleaseEvidence(t *testing.T) {
	repo := &evaluationRepoFake{}
	svc := NewAgentEvaluationService(repo)
	input := types.AgentEvaluationEntity{ID: "eval-service", ReleaseID: "rel-service", TestSetID: "gold", TestSetVersion: "1", EnvironmentClass: "ci", EvaluatorID: "forged", EvaluatedAt: time.Now().UTC(), ResultsJSON: `{"checks":[{"code":"safety","status":"passed"}]}`}
	view, err := svc.RecordEvaluation(context.Background(), "system-admin", input)
	require.NoError(t, err)
	require.Equal(t, "system-admin", view.EvaluatorID)
	listed, err := svc.ListEvaluationsForRelease(context.Background(), "rel-service")
	require.NoError(t, err)
	require.Equal(t, []string{"eval-service"}, []string{listed[0].ID})
}
