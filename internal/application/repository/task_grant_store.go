package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrTaskGrantTaskNotFound marks a task-id lookup that found no session row
// inside the tenant. Grant surfaces answer a uniform miss so a cross-tenant
// probe learns nothing.
var ErrTaskGrantTaskNotFound = errors.New("task not found in tenant")

// TaskGrantStore persists task_grants rows. Every method binds to the exact
// (tenant, task) pair; the task owner authority is sessions.user_id and is
// resolved by TaskOwnerID, never by a grant row.
type TaskGrantStore struct{ db *gorm.DB }

func NewTaskGrantStore(db *gorm.DB) *TaskGrantStore {
	return &TaskGrantStore{db: db}
}

// UpsertGrant inserts or rewrites one grant. Re-granting an existing grantee
// flips the role in place (one row per grantee), mirroring "share-again
// rotates" semantics without a second row.
func (s *TaskGrantStore) UpsertGrant(
	ctx context.Context, tenantID uint64, taskID, granteeID string,
	role types.TaskGrantRole, grantedBy string,
) (types.TaskGrant, error) {
	now := time.Now().UTC()
	grant := types.TaskGrant{
		TenantID: tenantID, TaskID: taskID, GranteeID: granteeID,
		Role: role, GrantedBy: grantedBy, CreatedAt: now, UpdatedAt: now,
	}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"}, {Name: "task_id"}, {Name: "grantee_id"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"role":       role,
			"granted_by": grantedBy,
			"updated_at": now,
		}),
	}).Create(&grant).Error
	if err != nil {
		return types.TaskGrant{}, err
	}
	return grant, nil
}

// DeleteGrant removes one grant. Deleting an absent grant is a success
// (idempotent revoke).
func (s *TaskGrantStore) DeleteGrant(ctx context.Context, tenantID uint64, taskID, granteeID string) error {
	return s.db.WithContext(ctx).
		Where("tenant_id = ? AND task_id = ? AND grantee_id = ?", tenantID, taskID, granteeID).
		Delete(&types.TaskGrant{}).Error
}

// ListGrants returns the task's grants, ordered deterministically by grantee.
func (s *TaskGrantStore) ListGrants(ctx context.Context, tenantID uint64, taskID string) ([]types.TaskGrant, error) {
	var grants []types.TaskGrant
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND task_id = ?", tenantID, taskID).
		Order("grantee_id ASC").Find(&grants).Error
	return grants, err
}

// RoleForGrantee resolves one grantee's role on the task. found=false covers
// no grant, cross-tenant task and unknown task alike.
func (s *TaskGrantStore) RoleForGrantee(
	ctx context.Context, tenantID uint64, taskID, granteeID string,
) (types.TaskGrantRole, bool, error) {
	if s == nil || s.db == nil || tenantID == 0 ||
		strings.TrimSpace(taskID) == "" || strings.TrimSpace(granteeID) == "" {
		return "", false, nil
	}
	var grant types.TaskGrant
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND task_id = ? AND grantee_id = ?", tenantID, taskID, granteeID).
		Take(&grant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return grant.Role, true, nil
}

// TaskOwnerID resolves the task's owner: sessions.user_id inside the tenant
// (ADR-0004). A miss — unknown task, cross-tenant probe — is one uniform
// ErrTaskGrantTaskNotFound.
func (s *TaskGrantStore) TaskOwnerID(ctx context.Context, tenantID uint64, taskID string) (string, error) {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(taskID) == "" {
		return "", ErrTaskGrantTaskNotFound
	}
	var owner string
	err := s.db.WithContext(ctx).Table("sessions").
		Where("tenant_id = ? AND id = ?", tenantID, taskID).
		Select("user_id").Scan(&owner).Error
	if err != nil {
		return "", err
	}
	if owner == "" {
		return "", ErrTaskGrantTaskNotFound
	}
	return owner, nil
}
