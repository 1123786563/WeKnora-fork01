package repository

// T17 (#47) stores for read-only research delegations and version-pinned
// annotations. Every query binds tenant (and session where the projection is
// session-scoped) so a cross-tenant probe reads as an empty set or one
// uniform miss — never as another tenant's rows.

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

const maxAnnotationBaseVersionRunes = 128

// TaskResearchStore owns task_research_delegations.
type TaskResearchStore struct {
	db *gorm.DB
}

// NewTaskResearchStore constructs the delegation store.
func NewTaskResearchStore(db *gorm.DB) *TaskResearchStore {
	return &TaskResearchStore{db: db}
}

// CreateDelegation inserts one delegation row. The caller (handler) owns
// identity and scope validation; the store persists as given. The entity is
// passed by pointer so GORM's automatic timestamps are written back to the
// caller's entity (B5-F74) — the 201 response echoes the persisted
// created_at, never a zero value.
func (s *TaskResearchStore) CreateDelegation(ctx context.Context, d *types.TaskResearchDelegation) error {
	return s.db.WithContext(ctx).Create(d).Error
}

// GetDelegation loads one delegation; a miss and a cross-tenant id are the
// same uniform ErrTaskResearchNotFound.
func (s *TaskResearchStore) GetDelegation(ctx context.Context, tenantID uint64, id string) (types.TaskResearchDelegation, error) {
	var row types.TaskResearchDelegation
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return types.TaskResearchDelegation{}, types.ErrTaskResearchNotFound
	}
	if err != nil {
		return types.TaskResearchDelegation{}, err
	}
	return row, nil
}

// ListDelegationsBySession lists the task's delegations in creation order,
// paged by the (created_at, id) keyset: cursor is the id of the last row of
// the previous page and the page resumes strictly after that row (B5-F67).
// All parameters are bound.
func (s *TaskResearchStore) ListDelegationsBySession(ctx context.Context, tenantID uint64, sessionID string, limit int, cursor string) ([]types.TaskResearchDelegation, error) {
	if limit < 1 {
		limit = 50
	}
	query := s.db.WithContext(ctx).Model(&types.TaskResearchDelegation{}).
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID)
	if cursor != "" {
		query = query.Where(
			"(created_at, id) > (SELECT created_at, id FROM task_research_delegations WHERE tenant_id = ? AND session_id = ? AND id = ?)",
			tenantID, sessionID, cursor,
		)
	}
	var rows []types.TaskResearchDelegation
	err := query.Order("created_at ASC, id ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// CompleteDelegation moves assigned→completed and records the findings
// summary in one CAS. A replay after completion conflicts (the recorded
// summary is immutable); an unknown id is the uniform miss.
func (s *TaskResearchStore) CompleteDelegation(ctx context.Context, tenantID uint64, id, summary string) (types.TaskResearchDelegation, error) {
	updated := s.db.WithContext(ctx).Model(&types.TaskResearchDelegation{}).
		Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, types.TaskResearchAssigned).
		Updates(map[string]any{
			"status":     types.TaskResearchCompleted,
			"summary":    summary,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if updated.Error != nil {
		return types.TaskResearchDelegation{}, updated.Error
	}
	if updated.RowsAffected == 1 {
		return s.GetDelegation(ctx, tenantID, id)
	}
	if _, err := s.GetDelegation(ctx, tenantID, id); err != nil {
		return types.TaskResearchDelegation{}, err
	}
	return types.TaskResearchDelegation{}, types.ErrTaskResearchState
}

// TaskAnnotationStore owns task_artifact_annotations.
type TaskAnnotationStore struct {
	db *gorm.DB
}

// NewTaskAnnotationStore constructs the annotation store.
func NewTaskAnnotationStore(db *gorm.DB) *TaskAnnotationStore {
	return &TaskAnnotationStore{db: db}
}

func validateAnnotation(a types.TaskArtifactAnnotation) error {
	if strings.TrimSpace(a.MaterialID) == "" || strings.TrimSpace(a.BaseVersion) == "" || strings.TrimSpace(a.Body) == "" {
		return types.ErrTaskAnnotationInvalid
	}
	if utf8.RuneCountInString(a.BaseVersion) > maxAnnotationBaseVersionRunes {
		return types.ErrTaskAnnotationInvalid
	}
	return nil
}

// CreateAnnotation appends one annotation. Rows are never updated by this
// store: the reviewed version identity is frozen at insert. The entity is
// passed by pointer so GORM's automatic timestamps are written back to the
// caller's entity (B5-F75).
func (s *TaskAnnotationStore) CreateAnnotation(ctx context.Context, a *types.TaskArtifactAnnotation) error {
	if err := validateAnnotation(*a); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Create(a).Error
}

// ListAnnotationsBySession lists the task's annotations in creation order,
// paged by the (created_at, id) keyset with an id cursor (B5-F67). All
// parameters are bound.
func (s *TaskAnnotationStore) ListAnnotationsBySession(ctx context.Context, tenantID uint64, sessionID string, limit int, cursor string) ([]types.TaskArtifactAnnotation, error) {
	if limit < 1 {
		limit = 50
	}
	query := s.db.WithContext(ctx).Model(&types.TaskArtifactAnnotation{}).
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID)
	if cursor != "" {
		query = query.Where(
			"(created_at, id) > (SELECT created_at, id FROM task_artifact_annotations WHERE tenant_id = ? AND session_id = ? AND id = ?)",
			tenantID, sessionID, cursor,
		)
	}
	var rows []types.TaskArtifactAnnotation
	err := query.Order("created_at ASC, id ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListAnnotationsForMaterial lists the annotations of one material binding.
func (s *TaskAnnotationStore) ListAnnotationsForMaterial(ctx context.Context, tenantID uint64, sessionID, materialID string) ([]types.TaskArtifactAnnotation, error) {
	var rows []types.TaskArtifactAnnotation
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND material_id = ?", tenantID, sessionID, materialID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
