package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

var (
	ErrAgentAdoptionEndPrecondition       = errors.New("agent adoption cannot end while variants remain active")
	ErrAgentAdoptionTransition            = errors.New("agent adoption state transition failed")
	ErrAgentMarketplaceListingTransition  = errors.New("agent marketplace listing state transition failed")
	ErrAgentReleaseDeprecateConflict      = errors.New("agent release deprecation conflict")
	ErrAgentMarketplaceListingUnavailable = errors.New("agent marketplace listing is unavailable for new use")
	ErrAgentMarketplaceReleaseDeprecated  = errors.New("agent marketplace release is deprecated")
	ErrAgentReleaseSuccessorInvalid       = errors.New("agent release successor is invalid")
)

func (r *agentAdoptionRepository) TransitionAdoption(ctx context.Context, tenantID uint64, adoptionID string, expectedFrom, nextState string, updates map[string]any) (*types.AgentAdoptionEntity, error) {
	var result types.AgentAdoptionEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockAdoptionState(tx, tenantID, adoptionID, expectedFrom); err != nil {
			return err
		}
		var current types.AgentAdoptionEntity
		err := tx.Where("tenant_id = ? AND id = ?", tenantID, adoptionID).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAgentAdoptionNotFound
		}
		if err != nil {
			return err
		}
		if current.State != expectedFrom {
			return fmt.Errorf("%w: adoption %s is %q, expected %q", ErrAgentAdoptionTransition, adoptionID, current.State, expectedFrom)
		}

		var residualCount int64
		if err := tx.Model(&types.AgentAdoptionVariantEntity{}).
			Where("tenant_id = ? AND adoption_id = ? AND state <> ?", tenantID, adoptionID, "retired").Count(&residualCount).Error; err != nil {
			return err
		}
		if residualCount > 0 {
			var first types.AgentAdoptionVariantEntity
			if err := tx.Select("id").Where("tenant_id = ? AND adoption_id = ? AND state <> ?", tenantID, adoptionID, "retired").Order("id ASC").First(&first).Error; err != nil {
				return err
			}
			return fmt.Errorf("%w: %d variant(s) remain; first variant %s", ErrAgentAdoptionEndPrecondition, residualCount, first.ID)
		}

		now := time.Now().UTC()
		values := copyLifecycleUpdates(updates)
		values["state"] = nextState
		values["updated_at"] = now
		if nextState == "ended" && values["ended_at"] == nil {
			values["ended_at"] = now
		}
		write := tx.Model(&types.AgentAdoptionEntity{}).
			Where("tenant_id = ? AND id = ? AND state = ?", tenantID, adoptionID, expectedFrom).Updates(values)
		if write.Error != nil {
			return write.Error
		}
		if write.RowsAffected != 1 {
			return ErrAgentAdoptionTransition
		}
		return tx.Where("tenant_id = ? AND id = ?", tenantID, adoptionID).First(&result).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// lockAdoptionState uses a guarded no-op UPDATE as the shared serialization
// point for creating Variants and ending an Adoption. PostgreSQL locks the
// matching row until commit; SQLite takes its write lock before the caller
// checks/inserts Variants. Both callers must use this guard inside their
// transaction before proceeding.
func lockAdoptionState(tx *gorm.DB, tenantID uint64, adoptionID, expectedState string) error {
	guard := tx.Model(&types.AgentAdoptionEntity{}).
		Where("tenant_id = ? AND id = ? AND state = ?", tenantID, adoptionID, expectedState).
		UpdateColumn("updated_at", gorm.Expr("updated_at"))
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
	return fmt.Errorf("%w: adoption %s is %q, expected %q", ErrAgentAdoptionTransition, adoptionID, current.State, expectedState)
}

func (r *agentAdoptionRepository) RetiredVariantAgentExists(ctx context.Context, tenantID uint64, localAgentID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&types.AgentAdoptionVariantEntity{}).
		Where("tenant_id = ? AND local_agent_id = ? AND state = ?", tenantID, localAgentID, "retired").Count(&count).Error
	return count > 0, err
}

func (r *agentMarketplaceRepository) TransitionListingState(ctx context.Context, tenantID uint64, listingID, expectedFrom, nextState string, updates map[string]any) (*types.AgentMarketplaceListingEntity, error) {
	var row types.AgentMarketplaceListingEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockListingState(tx, tenantID, listingID, expectedFrom); err != nil {
			return err
		}
		// The listing row lock precedes the tenant guard: same-tenant adoption
		// paths never take the guard, and on SQLite the guard write would
		// block behind any open writer before the listing lifecycle write.
		// The guard still serializes this transition with cross-tenant
		// IntroduceRelease custody rechecks (T35 #65).
		if err := acquireTenantSecurityGuardTx(tx, tenantID); err != nil {
			return err
		}
		now := time.Now().UTC()
		values := copyLifecycleUpdates(updates)
		values["state"] = nextState
		values["updated_at"] = now
		if nextState == "unlisted" && values["unlisted_at"] == nil {
			values["unlisted_at"] = now
		}
		write := tx.Model(&types.AgentMarketplaceListingEntity{}).
			Where("tenant_id = ? AND id = ? AND state = ?", tenantID, listingID, expectedFrom).Updates(values)
		if write.Error != nil {
			return write.Error
		}
		if write.RowsAffected != 1 {
			return ErrAgentMarketplaceListingTransition
		}
		return tx.Where("tenant_id = ? AND id = ?", tenantID, listingID).First(&row).Error
	})
	if errors.Is(err, ErrTenantNotFound) {
		return nil, ErrAgentMarketplaceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *agentMarketplaceRepository) DeprecateRelease(ctx context.Context, tenantID uint64, releaseID, deprecatedBy, successorReleaseID string) (*types.AgentReleaseEntity, error) {
	var row types.AgentReleaseEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ids := []string{releaseID, successorReleaseID}
		if ids[0] > ids[1] {
			ids[0], ids[1] = ids[1], ids[0]
		}
		locked := make(map[string]*types.AgentReleaseEntity, 2)
		for _, id := range ids {
			release, err := lockReleaseRow(tx, tenantID, id)
			if err != nil {
				return err
			}
			locked[id] = release
		}
		source, successor := locked[releaseID], locked[successorReleaseID]
		if source.DeprecatedAt != nil {
			return fmt.Errorf("%w: release %s is already deprecated", ErrAgentReleaseDeprecateConflict, releaseID)
		}
		if successor.DeprecatedAt != nil {
			return fmt.Errorf("%w: successor release %s was deprecated concurrently", ErrAgentReleaseDeprecateConflict, successorReleaseID)
		}
		if successor.ListingID != source.ListingID || successor.ID == source.ID {
			return ErrAgentReleaseSuccessorInvalid
		}
		now := time.Now().UTC()
		write := tx.Model(&types.AgentReleaseEntity{}).
			Where("tenant_id = ? AND id = ? AND deprecated_at IS NULL", tenantID, releaseID).
			Updates(map[string]any{"deprecated_at": now, "deprecated_by": deprecatedBy, "successor_release_id": successorReleaseID})
		if write.Error != nil {
			return write.Error
		}
		if write.RowsAffected != 1 {
			return ErrAgentReleaseDeprecateConflict
		}
		return tx.Where("tenant_id = ? AND id = ?", tenantID, releaseID).First(&row).Error
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// lockListingState performs a guarded no-op write before checking lifecycle
// eligibility. It serializes new marketplace use with UnlistListing on both
// PostgreSQL and SQLite; the tenant and ID predicates are always retained.
func lockListingState(tx *gorm.DB, tenantID uint64, listingID, expectedState string) error {
	guard := tx.Model(&types.AgentMarketplaceListingEntity{}).
		Where("tenant_id = ? AND id = ? AND state = ?", tenantID, listingID, expectedState).
		UpdateColumn("id", gorm.Expr("id"))
	if guard.Error != nil {
		return guard.Error
	}
	if guard.RowsAffected == 1 {
		return nil
	}
	var current types.AgentMarketplaceListingEntity
	err := tx.Where("tenant_id = ? AND id = ?", tenantID, listingID).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Public listings introduced into a tenant are virtual rows backed by
		// the introduction ledger and cannot be unlisted by the tenant.
		introduced, ierr := introducedListing(tx, tenantID, listingID)
		if ierr != nil {
			return ierr
		}
		if introduced != nil && introduced.State == expectedState {
			return nil
		}
		if introduced != nil {
			return fmt.Errorf("%w: listing %s is %q, expected %q", ErrAgentMarketplaceListingTransition, listingID, introduced.State, expectedState)
		}
		return ErrAgentMarketplaceNotFound
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: listing %s is %q, expected %q", ErrAgentMarketplaceListingTransition, listingID, current.State, expectedState)
}

func requireListedListing(tx *gorm.DB, tenantID uint64, listingID string) error {
	err := lockListingState(tx, tenantID, listingID, "listed")
	if errors.Is(err, ErrAgentMarketplaceListingTransition) {
		return ErrAgentMarketplaceListingUnavailable
	}
	return err
}

// lockReleaseRow locks an existing release row with a no-op update, then
// returns its committed lifecycle state. Introduced releases have no mutable
// local Release row and are represented by an immutable tenant ledger entry.
func lockReleaseRow(tx *gorm.DB, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error) {
	guard := tx.Model(&types.AgentReleaseEntity{}).
		Where("tenant_id = ? AND id = ?", tenantID, releaseID).
		UpdateColumn("id", gorm.Expr("id"))
	if guard.Error != nil {
		return nil, guard.Error
	}
	var row types.AgentReleaseEntity
	err := tx.Where("tenant_id = ? AND id = ?", tenantID, releaseID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		introduced, ierr := introducedRelease(tx, tenantID, releaseID)
		if ierr != nil {
			return nil, ierr
		}
		if introduced == nil {
			return nil, ErrAgentMarketplaceNotFound
		}
		return introduced, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func requireActiveRelease(tx *gorm.DB, tenantID uint64, releaseID, listingID string) error {
	release, err := lockReleaseRow(tx, tenantID, releaseID)
	if err != nil {
		return err
	}
	if release.ListingID != listingID {
		return ErrAgentMarketplaceNotFound
	}
	if release.DeprecatedAt != nil {
		return ErrAgentMarketplaceReleaseDeprecated
	}
	return nil
}

func copyLifecycleUpdates(updates map[string]any) map[string]any {
	values := make(map[string]any, len(updates)+3)
	for key, value := range updates {
		// These columns define the row's identity or creation history. Never
		// let a transition payload move a row outside the tenant-scoped CAS.
		switch key {
		case "id", "tenant_id", "created_at":
			continue
		}
		values[key] = value
	}
	return values
}

var _ AgentAdoptionRepository = (*agentAdoptionRepository)(nil)
var _ AgentMarketplaceRepository = (*agentMarketplaceRepository)(nil)
