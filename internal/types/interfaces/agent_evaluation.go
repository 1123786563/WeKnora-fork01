package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

type AgentEvaluationRepository interface {
	CreateEvaluation(ctx context.Context, evaluation *types.AgentEvaluationEntity) (*types.AgentEvaluationEntity, error)
	ListEvaluationsForReleases(ctx context.Context, releaseIDs []string) (map[string][]types.AgentEvaluationEntity, error)
}

// AgentEvaluationView contains only release-level review evidence.
type AgentEvaluationView struct {
	ID               string    `json:"id"`
	ReleaseID        string    `json:"release_id"`
	TestSetID        string    `json:"test_set_id"`
	TestSetVersion   string    `json:"test_set_version"`
	EnvironmentClass string    `json:"environment_class"`
	EvaluatorID      string    `json:"evaluator_id"`
	EvaluatedAt      time.Time `json:"evaluated_at"`
	ResultsJSON      string    `json:"results"`
}

type AgentEvaluationService interface {
	RecordEvaluation(ctx context.Context, reviewerID string, evaluation types.AgentEvaluationEntity) (AgentEvaluationView, error)
	ListEvaluationsForRelease(ctx context.Context, releaseID string) ([]AgentEvaluationView, error)
}
