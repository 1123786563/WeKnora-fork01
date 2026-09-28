package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrAgentEvaluationInvalid  = errors.New("invalid agent evaluation")
	ErrAgentEvaluationConflict = errors.New("agent evaluation identity already exists")
)

type AgentEvaluationRepository interface {
	CreateEvaluation(context.Context, *types.AgentEvaluationEntity) (*types.AgentEvaluationEntity, error)
	ListEvaluationsForReleases(context.Context, []string) (map[string][]types.AgentEvaluationEntity, error)
}

type agentEvaluationRepository struct{ db *gorm.DB }

func NewAgentEvaluationRepository(db *gorm.DB) AgentEvaluationRepository {
	return &agentEvaluationRepository{db: db}
}

var allowedEvaluationCodes = map[string]bool{"manifest_completeness": true, "compatibility": true, "license": true, "security": true, "dependency_integrity": true, "privacy": true}
var allowedEvaluationStatuses = map[string]bool{"pass": true, "fail": true, "not_run": true}
var allowedEvaluationOverallStatuses = map[string]bool{"pass": true, "fail": true, "inconclusive": true}

func validateEvaluation(e *types.AgentEvaluationEntity) error {
	if e == nil || strings.TrimSpace(e.ReleaseID) == "" || strings.TrimSpace(e.TestSetID) == "" || strings.TrimSpace(e.TestSetVersion) == "" || strings.TrimSpace(e.EnvironmentClass) == "" || strings.TrimSpace(e.EvaluatorID) == "" || e.EvaluatedAt.IsZero() {
		return ErrAgentEvaluationInvalid
	}
	var result struct {
		Status string `json:"status"`
		Checks []struct {
			Code   string `json:"code"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(e.ResultsJSON)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil || len(result.Checks) == 0 || !allowedEvaluationOverallStatuses[result.Status] {
		return ErrAgentEvaluationInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ErrAgentEvaluationInvalid
	}
	for _, check := range result.Checks {
		if !allowedEvaluationCodes[check.Code] || !allowedEvaluationStatuses[check.Status] {
			return ErrAgentEvaluationInvalid
		}
	}
	return nil
}

func (r *agentEvaluationRepository) CreateEvaluation(ctx context.Context, evaluation *types.AgentEvaluationEntity) (*types.AgentEvaluationEntity, error) {
	if err := validateEvaluation(evaluation); err != nil {
		return nil, err
	}
	row := *evaluation
	if row.ID == "" {
		row.ID = uuid.NewString()
	}
	err := r.db.WithContext(ctx).Create(&row).Error
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, fmt.Errorf("%w: %v", ErrAgentEvaluationConflict, err)
		}
		return nil, err
	}
	return &row, nil
}

func (r *agentEvaluationRepository) ListEvaluationsForReleases(ctx context.Context, releaseIDs []string) (map[string][]types.AgentEvaluationEntity, error) {
	result := make(map[string][]types.AgentEvaluationEntity, len(releaseIDs))
	ids := make([]string, 0, len(releaseIDs))
	for _, id := range releaseIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			result[id] = []types.AgentEvaluationEntity{}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return result, nil
	}
	var rows []types.AgentEvaluationEntity
	if err := r.db.WithContext(ctx).Where("release_id IN ?", ids).Order("evaluated_at DESC, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ReleaseID] = append(result[row.ReleaseID], row)
	}
	return result, nil
}
