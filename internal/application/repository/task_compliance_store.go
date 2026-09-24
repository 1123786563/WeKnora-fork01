package repository

// T13 (#43): the persistence lane for tenant task-retention policy,
// compliance access windows, and the compliance metadata/content
// projections. Every statement is parameter-bound.

import (
	"context"
	"errors"
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
