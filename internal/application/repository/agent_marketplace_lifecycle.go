package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/types"
)

var (
	ErrAgentAdoptionEndPrecondition = errors.New("agent adoption cannot end while variants remain active")

	ErrAgentMarketplaceListingTransition = errors.New("agent marketplace listing state transition failed")
	ErrAgentReleaseDeprecateConflict     = errors.New("agent release deprecation conflict")
)

// guardAdoptionState is the shared lock-first write used by every operation
// that can add a Variant or end its parent Adoption. The guarded no-op UPDATE
// is deliberately the first SQL in the transaction: it locks the parent row
// on PostgreSQL and obtains SQLite's writer reservation before either caller
// reads child rows or inserts a child.
func guardAdoptionState(tx *gorm.DB, tenantID uint64, adoptionID, expectedState string) error {
	guard := tx.Model(&types.AgentAdoptionEntity{}).
		Where("tenant_id = ? AND id = ? AND state = ?", tenantID, adoptionID, expectedState).
		UpdateColumn("state", gorm.Expr("state"))
	if guard.Error != nil {
		return guard.Error
	}
	if guard.RowsAffected == 1 {
		return nil
	}

	var current types.AgentAdoptionEntity
	err := tx.Where("tenant_id = ? AND id = ?", tenantID, adoptionID).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrAgentAdoptionNotFound
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: state is %q, expected %q", ErrAgentAdoptionTransition, current.State, expectedState)
}

// RetiredVariantAgentExists checks for retired ownership within one Tenant.
func (r *agentAdoptionRepository) RetiredVariantAgentExists(ctx context.Context, tenantID uint64, localAgentID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&types.AgentAdoptionVariantEntity{}).
		Where("tenant_id = ? AND local_agent_id = ? AND state = ?", tenantID, localAgentID, "retired").
		Count(&count).Error
	return count > 0, err
}

// TransitionListingState changes a Listing state with a tenant-scoped CAS.
func (r *agentMarketplaceRepository) TransitionListingState(ctx context.Context, tenantID uint64, listingID, expectedFrom, nextState string, updates map[string]any) (*types.AgentMarketplaceListingEntity, error) {
	values := cloneLifecycleUpdates(updates)
	values["state"] = nextState
	now := time.Now().UTC()
	if nextState == "unlisted" {
		values["unlisted_at"] = now
	}
	values["updated_at"] = now
	updated := r.db.WithContext(ctx).Model(&types.AgentMarketplaceListingEntity{}).
		Where("tenant_id = ? AND id = ? AND state = ?", tenantID, listingID, expectedFrom).
		Updates(values)
	if updated.Error != nil {
		return nil, updated.Error
	}
	if updated.RowsAffected != 1 {
		var current types.AgentMarketplaceListingEntity
		err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, listingID).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentMarketplaceNotFound
		}
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: state is %q, expected %q", ErrAgentMarketplaceListingTransition, current.State, expectedFrom)
	}
	var result types.AgentMarketplaceListingEntity
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, listingID).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

// DeprecateRelease marks a Release deprecated once and preserves its content.
func (r *agentMarketplaceRepository) DeprecateRelease(ctx context.Context, tenantID uint64, releaseID, deprecatedBy, successorReleaseID string) (*types.AgentReleaseEntity, error) {
	now := time.Now().UTC()
	updated := r.db.WithContext(ctx).Model(&types.AgentReleaseEntity{}).
		Where("tenant_id = ? AND id = ? AND deprecated_at IS NULL", tenantID, releaseID).
		Updates(map[string]any{
			"deprecated_at":        now,
			"deprecated_by":        deprecatedBy,
			"successor_release_id": successorReleaseID,
		})
	if updated.Error != nil {
		return nil, updated.Error
	}
	if updated.RowsAffected != 1 {
		var current types.AgentReleaseEntity
		err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, releaseID).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentMarketplaceNotFound
		}
		if err != nil {
			return nil, err
		}
		return nil, ErrAgentReleaseDeprecateConflict
	}
	var result types.AgentReleaseEntity
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, releaseID).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func cloneLifecycleUpdates(updates map[string]any) map[string]any {
	values := make(map[string]any, len(updates)+2)
	for key, value := range updates {
		values[key] = value
	}
	return values
}
