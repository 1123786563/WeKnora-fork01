package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type resourceRepository struct{ db *gorm.DB }

var _ interfaces.GuardedResourceDeleteRepository = (*resourceRepository)(nil)

const guardedDeleteLease = time.Minute

// NewResourceRepository creates the persistence adapter for resource metadata.
func NewResourceRepository(db *gorm.DB) interfaces.ResourceRepository {
	return &resourceRepository{db: db}
}

func (r *resourceRepository) Create(ctx context.Context, resource *types.StoredResource) error {
	return r.db.WithContext(ctx).Create(resource).Error
}

func (r *resourceRepository) GetByID(ctx context.Context, id string) (*types.StoredResource, error) {
	var resource types.StoredResource
	err := r.db.WithContext(ctx).Where("id = ? AND state = ?", id, types.ResourceStateActive).First(&resource).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &resource, err
}

func (r *resourceRepository) GetByHandle(ctx context.Context, handle string) (*types.StoredResource, error) {
	var resource types.StoredResource
	err := r.db.WithContext(ctx).
		Where("handle = ? AND state = ?", handle, types.ResourceStateActive).
		First(&resource).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &resource, err
}

func (r *resourceRepository) GetByTenantLocation(
	ctx context.Context,
	tenantID uint64,
	locationHash string,
) (*types.StoredResource, error) {
	var resource types.StoredResource
	err := r.db.WithContext(ctx).
		Where(
			"tenant_id = ? AND location_hash = ? AND state = ?",
			tenantID,
			locationHash,
			types.ResourceStateActive,
		).
		First(&resource).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &resource, err
}

func (r *resourceRepository) MarkDeleted(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&types.StoredResource{}).Where("id = ?", id).
		Updates(map[string]interface{}{"state": types.ResourceStateDeleted, "deleted_at": time.Now()}).Error
}

func (r *resourceRepository) CreateBinding(ctx context.Context, binding *types.ResourceBinding) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The no-op update reserves SQLite's writer and locks the PostgreSQL
		// resource row before insertion. Deletion claims use the same boundary.
		locked := tx.Exec("UPDATE resources SET state=state WHERE id=? AND tenant_id=? AND state=?", binding.ResourceID, binding.TenantID, types.ResourceStateActive)
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected != 1 {
			return interfaces.ErrResourceUnavailable
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(binding).Error
	})
}

// ClaimUnboundResource is the write-serialized decision that permits physical
// deletion. A fresh deleting claim is busy; a stale or explicitly retried one
// can resume after a crashed/failing cleaner without reopening binding.
func (r *resourceRepository) ClaimUnboundResource(ctx context.Context, tenantID uint64, handle string) (*types.StoredResource, bool, error) {
	var resource types.StoredResource
	claimed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked := tx.Exec("UPDATE resources SET state=state WHERE handle=? AND tenant_id=? AND state IN (?,?)", handle, tenantID, types.ResourceStateActive, types.ResourceStateDeleting)
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected == 0 {
			return nil
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("handle=? AND tenant_id=?", handle, tenantID).First(&resource).Error; err != nil {
			return err
		}
		var bindings int64
		if err := tx.Model(&types.ResourceBinding{}).Where("resource_id=?", resource.ID).Count(&bindings).Error; err != nil {
			return err
		}
		if bindings != 0 {
			return nil
		}
		now := time.Now().UTC()
		if resource.State == types.ResourceStateDeleting && resource.UpdatedAt.After(now.Add(-guardedDeleteLease)) {
			return nil
		}
		res := tx.Model(&types.StoredResource{}).Where("id=? AND tenant_id=? AND state IN (?,?)", resource.ID, tenantID, types.ResourceStateActive, types.ResourceStateDeleting).Updates(map[string]any{"state": types.ResourceStateDeleting, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return interfaces.ErrResourceUnavailable
		}
		claimed = true
		return nil
	})
	if err != nil || !claimed {
		return nil, false, err
	}
	return &resource, true, nil
}

// IsDeletedResource checks the terminal tombstone after a source-row clear
// failed. GORM's ordinary scope hides deleted rows, so this lookup is scoped
// explicitly to the tenant, exact handle, and terminal state.
func (r *resourceRepository) IsDeletedResource(ctx context.Context, tenantID uint64, handle string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Unscoped().Model(&types.StoredResource{}).
		Where("tenant_id=? AND handle=? AND state=?", tenantID, handle, types.ResourceStateDeleted).
		Count(&count).Error
	return count == 1, err
}

func (r *resourceRepository) FinishUnboundResourceDelete(ctx context.Context, tenantID uint64, resourceID string) error {
	res := r.db.WithContext(ctx).Model(&types.StoredResource{}).Where("id=? AND tenant_id=? AND state=?", resourceID, tenantID, types.ResourceStateDeleting).Updates(map[string]any{"state": types.ResourceStateDeleted, "deleted_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return interfaces.ErrResourceUnavailable
	}
	return nil
}

func (r *resourceRepository) RetryUnboundResourceDelete(ctx context.Context, tenantID uint64, resourceID string) error {
	return r.db.WithContext(ctx).Model(&types.StoredResource{}).Where("id=? AND tenant_id=? AND state=?", resourceID, tenantID, types.ResourceStateDeleting).Update("updated_at", time.Now().UTC().Add(-guardedDeleteLease-time.Second)).Error
}

// DeleteBinding removes one owner's claim on a resource. Deleting a claim that
// was never recorded is not an error: callers release optimistically, from a
// content scan that cannot know which references were bound.
func (r *resourceRepository) DeleteBinding(ctx context.Context, resourceID, ownerType, ownerID string) error {
	return r.db.WithContext(ctx).
		Where("resource_id = ? AND owner_type = ? AND owner_id = ?", resourceID, ownerType, ownerID).
		Delete(&types.ResourceBinding{}).Error
}

func (r *resourceRepository) CountBindings(ctx context.Context, resourceID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&types.ResourceBinding{}).
		Where("resource_id = ?", resourceID).
		Count(&count).Error
	return count, err
}

func (r *resourceRepository) CreateGrant(ctx context.Context, grant *types.ResourceAccessGrant) error {
	return r.db.WithContext(ctx).Create(grant).Error
}

func (r *resourceRepository) GetValidGrant(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (*types.ResourceAccessGrant, error) {
	var grant types.ResourceAccessGrant
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", tokenHash, now).
		First(&grant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &grant, err
}

// DeleteExpiredGrants drops grants that are past their expiry. A revoked grant
// is kept until then on purpose: it is the tombstone that stops a token derived
// for the same resource and window from re-creating the row and reviving access.
func (r *resourceRepository) DeleteExpiredGrants(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("expires_at <= ?", before).
		Delete(&types.ResourceAccessGrant{}).Error
}
