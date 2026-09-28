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
	ErrAgentAdoptionEndPrecondition      = errors.New("agent adoption cannot end while variants remain active")
	ErrAgentAdoptionTransition           = errors.New("agent adoption state transition failed")
	ErrAgentMarketplaceListingTransition = errors.New("agent marketplace listing state transition failed")
	ErrAgentReleaseDeprecateConflict     = errors.New("agent release deprecation conflict")
)

// EndAdoption ends an Adoption only after every Variant is retired. The
// precondition query and guarded update share one transaction so a concurrent
// Variant creation cannot slip between the check and the state transition.
func (r *agentAdoptionRepository) EndAdoption(ctx context.Context, tenantID uint64, adoptionID string, expectedFrom, nextState string, updates map[string]any) (*types.AgentAdoptionEntity, error) {
	var result types.AgentAdoptionEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := guardAdoptionState(tx, tenantID, adoptionID, expectedFrom); err != nil {
			return err
		}

		var remaining int64
		if err := tx.Model(&types.AgentAdoptionVariantEntity{}).
			Where("tenant_id = ? AND adoption_id = ? AND state <> ?", tenantID, adoptionID, "retired").
			Count(&remaining).Error; err != nil {
			return err
		}
		if remaining > 0 {
			var first types.AgentAdoptionVariantEntity
			if err := tx.Select("id").Where("tenant_id = ? AND adoption_id = ? AND state <> ?", tenantID, adoptionID, "retired").
				Order("id ASC").First(&first).Error; err != nil {
				return err
			}
			return fmt.Errorf("%w: %d variant(s) remain, first variant %s", ErrAgentAdoptionEndPrecondition, remaining, first.ID)
		}

		values := cloneLifecycleUpdates(updates)
		values["state"] = nextState
		now := time.Now().UTC()
		values["ended_at"] = now
		values["updated_at"] = now
		updated := tx.Model(&types.AgentAdoptionEntity{}).
			Where("tenant_id = ? AND id = ? AND state = ?", tenantID, adoptionID, expectedFrom).
			Updates(values)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrAgentAdoptionTransition
		}
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, adoptionID).First(&result).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

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
	var result types.AgentMarketplaceListingEntity
	err := withTenantSecurityGuard(ctx, r.db, tenantID, func(tx *gorm.DB) error {
		updated := tx.Model(&types.AgentMarketplaceListingEntity{}).
			Where("tenant_id = ? AND id = ? AND state = ?", tenantID, listingID, expectedFrom).
			Updates(values)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			var current types.AgentMarketplaceListingEntity
			err := tx.Where("tenant_id = ? AND id = ?", tenantID, listingID).First(&current).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentMarketplaceNotFound
			}
			if err != nil {
				return err
			}
			return fmt.Errorf("%w: state is %q, expected %q", ErrAgentMarketplaceListingTransition, current.State, expectedFrom)
		}
		return tx.Where("tenant_id = ? AND id = ?", tenantID, listingID).First(&result).Error
	})
	if errors.Is(err, ErrTenantNotFound) {
		return nil, ErrAgentMarketplaceNotFound
	}
	if err != nil {
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
