package repository

// T13 (#43): the persistence lane for tenant task-retention policy,
// compliance access windows, and the compliance metadata/content
// projections. Every statement is parameter-bound.

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// TaskComplianceStore persists the T13 lanes.
type TaskComplianceStore struct{ db *gorm.DB }

// NewTaskComplianceStore constructs the production store.
func NewTaskComplianceStore(db *gorm.DB) *TaskComplianceStore { return &TaskComplianceStore{db: db} }

// GetTaskPolicy returns the tenant's policy, or (nil, nil) when the tenant
// never set one — the ungated default that keeps every existing tenant's
// deletion flow unchanged.
func (s *TaskComplianceStore) GetTaskPolicy(ctx context.Context, tenantID uint64) (*types.TenantTaskPolicy, error) {
	if s == nil || s.db == nil || tenantID == 0 {
		return nil, errors.New("task compliance store is not assembled")
	}
	var policy types.TenantTaskPolicy
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Take(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// UpsertTaskPolicy writes the tenant policy row (PK = tenant_id). The
// read-then-write transaction keeps one dialect-neutral upsert; the row is
// single-tenant config with no concurrent writers in practice.
func (s *TaskComplianceStore) UpsertTaskPolicy(ctx context.Context, policy types.TenantTaskPolicy) (types.TenantTaskPolicy, error) {
	if s == nil || s.db == nil || policy.TenantID == 0 {
		return types.TenantTaskPolicy{}, errors.New("task compliance store is not assembled")
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing types.TenantTaskPolicy
		err := tx.Where("tenant_id = ?", policy.TenantID).Take(&existing).Error
		now := time.Now().UTC()
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			policy.CreatedAt, policy.UpdatedAt = now, now
			return tx.Create(&policy).Error
		case err != nil:
			return err
		default:
			policy.CreatedAt, policy.UpdatedAt = existing.CreatedAt, now
			return tx.Model(&types.TenantTaskPolicy{}).
				Where("tenant_id = ?", policy.TenantID).
				Updates(map[string]any{
					"retention_days": policy.RetentionDays,
					"legal_hold":     policy.LegalHold,
					"updated_by":     policy.UpdatedBy,
					"updated_at":     now,
				}).Error
		}
	})
	if err != nil {
		return types.TenantTaskPolicy{}, err
	}
	return policy, nil
}

// OpenComplianceAccess inserts one administrator access window.
func (s *TaskComplianceStore) OpenComplianceAccess(ctx context.Context, access types.TaskComplianceAccess) (types.TaskComplianceAccess, error) {
	if s == nil || s.db == nil {
		return types.TaskComplianceAccess{}, errors.New("task compliance store is not assembled")
	}
	if access.CreatedAt.IsZero() {
		access.CreatedAt = time.Now().UTC()
	}
	if err := s.db.WithContext(ctx).Create(&access).Error; err != nil {
		return types.TaskComplianceAccess{}, err
	}
	return access, nil
}

// ActiveComplianceAccess returns the administrator's newest unexpired window
// on the task at `now`, or (nil, nil) when none covers the instant. The
// real-time filter is the whole enforcement — no grace, no cache.
func (s *TaskComplianceStore) ActiveComplianceAccess(ctx context.Context, tenantID uint64, taskID, adminID string, now time.Time) (*types.TaskComplianceAccess, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("task compliance store is not assembled")
	}
	var window types.TaskComplianceAccess
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND task_id = ? AND admin_id = ? AND expires_at > ?", tenantID, taskID, adminID, now).
		Order("id DESC").Limit(1).Take(&window).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &window, nil
}

// TaskMetadataFacts projects ONE task's metadata (title/owner/state/run
// counts). It includes soft-deleted tasks — the administrator must see that
// a task was deleted — and carries no message content. A miss (unknown or
// cross-tenant task) is one uniform ErrTaskComplianceNotFound.
func (s *TaskComplianceStore) TaskMetadataFacts(ctx context.Context, tenantID uint64, taskID string) (*types.TaskMetadataFacts, error) {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task compliance store is not assembled")
	}
	taskID = strings.TrimSpace(taskID)
	facts := types.TaskMetadataFacts{TaskID: taskID}
	row := struct {
		Title      *string
		UserID     *string
		CreatedAt  *time.Time
		ArchivedAt *time.Time
		DeletedAt  *time.Time
	}{}
	err := s.db.WithContext(ctx).Table("sessions").
		Select("title, user_id, created_at, archived_at, deleted_at").
		Where("tenant_id = ? AND id = ?", tenantID, taskID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, types.ErrTaskComplianceNotFound
	}
	if err != nil {
		return nil, err
	}
	if row.Title != nil {
		facts.Title = *row.Title
	}
	if row.UserID != nil {
		facts.OwnerID = *row.UserID
	}
	if row.CreatedAt != nil {
		facts.CreatedAt = *row.CreatedAt
	}
	facts.ArchivedAt, facts.DeletedAt = row.ArchivedAt, row.DeletedAt

	if err := s.db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND session_id = ?", tenantID, taskID).
		Count(&facts.RunCount).Error; err != nil {
		return nil, err
	}
	var lastStatus *string
	if err := s.db.WithContext(ctx).Table("agent_runs").Select("status").
		Where("tenant_id = ? AND session_id = ?", tenantID, taskID).
		Order("created_at DESC").Limit(1).Scan(&lastStatus).Error; err != nil {
		return nil, err
	}
	if lastStatus != nil {
		facts.LastRunState = *lastStatus
	}
	return &facts, nil
}

// ListTaskMessages reads the task's private conversation rows (the content
// projection). Scoped by session_id; the caller proves the session's tenant
// ownership first (TaskMetadataFacts).
func (s *TaskComplianceStore) ListTaskMessages(ctx context.Context, taskID string, limit int) ([]types.TaskMessageFact, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("task compliance store is not assembled")
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows []types.TaskMessageFact
	err := s.db.WithContext(ctx).Table("messages").
		Select("id, role, content, created_at").
		Where("session_id = ?", taskID).
		Order("created_at ASC, id ASC").Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
