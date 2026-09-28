package service

import (
	"context"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var ErrAgentEvaluationInvalidInput = repository.ErrAgentEvaluationInvalid

type AgentEvaluationService struct {
	repo repository.AgentEvaluationRepository
}

var _ interfaces.AgentEvaluationService = (*AgentEvaluationService)(nil)

func NewAgentEvaluationService(repo repository.AgentEvaluationRepository) *AgentEvaluationService {
	return &AgentEvaluationService{repo: repo}
}

func (s *AgentEvaluationService) RecordEvaluation(ctx context.Context, reviewerID string, evaluation types.AgentEvaluationEntity) (interfaces.AgentEvaluationView, error) {
	reviewerID = strings.TrimSpace(reviewerID)
	if reviewerID == "" {
		return interfaces.AgentEvaluationView{}, ErrAgentEvaluationInvalidInput
	}
	evaluation.EvaluatorID = reviewerID
	row, err := s.repo.CreateEvaluation(ctx, &evaluation)
	if err != nil {
		return interfaces.AgentEvaluationView{}, err
	}
	return evaluationView(*row), nil
}

func (s *AgentEvaluationService) ListEvaluationsForRelease(ctx context.Context, releaseID string) ([]interfaces.AgentEvaluationView, error) {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return nil, ErrAgentEvaluationInvalidInput
	}
	rows, err := s.repo.ListEvaluationsForReleases(ctx, []string{releaseID})
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.AgentEvaluationView, 0, len(rows[releaseID]))
	for _, row := range rows[releaseID] {
		views = append(views, evaluationView(row))
	}
	return views, nil
}

func evaluationView(row types.AgentEvaluationEntity) interfaces.AgentEvaluationView {
	return interfaces.AgentEvaluationView{ID: row.ID, ReleaseID: row.ReleaseID, TestSetID: row.TestSetID, TestSetVersion: row.TestSetVersion, EnvironmentClass: row.EnvironmentClass, EvaluatorID: row.EvaluatorID, EvaluatedAt: row.EvaluatedAt, ResultsJSON: row.ResultsJSON}
}
