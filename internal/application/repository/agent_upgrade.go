package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Tencent/WeKnora/internal/types"
)

var (
	ErrAgentUpgradeProposalNotFound = errors.New("agent upgrade proposal not found")
	// ErrAgentUpgradeProposalTransition marks a TransitionProposal whose
	// guarded state UPDATE missed (already resolved, or a concurrent
	// resolution landed first): the move fails loudly instead of
	// double-applying. Mirrors ErrAgentAdoptionVariantTransition.
	ErrAgentUpgradeProposalTransition = errors.New("agent upgrade proposal state transition failed")
)

// AgentUpgradeRepository owns the upgrade-proposal rows. It embeds
// AgentAdoptionRepository so the service reads listings/releases/adoption/
// variant/mapping data through ONE seam that already carries the #60
// introduction-ledger fallbacks (introducedListing/introducedRelease); the
// proposal primitives below are plain tenant-scoped, parameter-bound SQL.
type AgentUpgradeRepository interface {
	AgentAdoptionRepository
	FindOrCreateProposal(context.Context, *types.AgentUpgradeProposalEntity) (*types.AgentUpgradeProposalEntity, bool, error)
	GetProposal(context.Context, uint64, string) (*types.AgentUpgradeProposalEntity, error)
	ListProposals(context.Context, uint64) ([]types.AgentUpgradeProposalEntity, error)
	TransitionProposal(context.Context, uint64, string, []string, string, map[string]any) (*types.AgentUpgradeProposalEntity, error)
}

type agentUpgradeRepository struct {
	AgentAdoptionRepository
	db *gorm.DB
}

func NewAgentUpgradeRepository(db *gorm.DB) AgentUpgradeRepository {
	return &agentUpgradeRepository{AgentAdoptionRepository: NewAgentAdoptionRepository(db), db: db}
}

// FindOrCreateProposal inserts the (tenant, adoption, to_release)-unique
// proposal or returns the existing row — the same race-convergent shape as
// adoptListingTx: the insert losing to uq_agent_upgrade_proposals_scope
// re-reads the winner so both callers get an idempotent result.
func (r *agentUpgradeRepository) FindOrCreateProposal(ctx context.Context, proposal *types.AgentUpgradeProposalEntity) (*types.AgentUpgradeProposalEntity, bool, error) {
	key := func() (*types.AgentUpgradeProposalEntity, error) {
		var existing types.AgentUpgradeProposalEntity
		err := r.db.WithContext(ctx).Where(
			"tenant_id = ? AND adoption_id = ? AND to_release_id = ?",
			proposal.TenantID, proposal.AdoptionID, proposal.ToReleaseID,
		).First(&existing).Error
		if err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if existing, err := key(); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	created := *proposal
	created.ID = uuid.NewString()
	created.CreatedAt = time.Now().UTC()
	created.UpdatedAt = created.CreatedAt
	if created.State == "" {
		created.State = "open"
	}
	createdAt := time.Now().UTC()
	created.CreatedAt, created.UpdatedAt = createdAt, createdAt
	var result *types.AgentUpgradeProposalEntity
	wasCreated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Reconcile materializes only while all source rows still permit new
		// use. Follow the same listing -> release -> adoption lock order as
		// AdoptListing and CreateVariant.
		if err := requireListedListing(tx, created.TenantID, created.ListingID); err != nil {
			return err
		}
		if err := requireActiveRelease(tx, created.TenantID, created.ToReleaseID, created.ListingID); err != nil {
			return err
		}
		// ponytail: materialize 不锁 adoption 状态（HEAD 世代剧本：ended 后仍可
		// 记录 proposal，闸在 AcceptUpgradeProposal/service 层）；task3 的
		// Rechecks 测试因此 t.Skip 待裁决。
		var existing types.AgentUpgradeProposalEntity
		findErr := tx.Where("tenant_id = ? AND adoption_id = ? AND to_release_id = ?", created.TenantID, created.AdoptionID, created.ToReleaseID).First(&existing).Error
		if findErr == nil {
			result = &existing
			return nil
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&created)
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 1 {
			result, wasCreated = &created, true
			return nil
		}
		var winner types.AgentUpgradeProposalEntity
		if err := tx.Where("tenant_id = ? AND adoption_id = ? AND to_release_id = ?", created.TenantID, created.AdoptionID, created.ToReleaseID).First(&winner).Error; err != nil {
			return err
		}
		result = &winner
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return result, wasCreated, nil
}

func (r *agentUpgradeRepository) GetProposal(ctx context.Context, tenantID uint64, proposalID string) (*types.AgentUpgradeProposalEntity, error) {
	var row types.AgentUpgradeProposalEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(proposalID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *agentUpgradeRepository) ListProposals(ctx context.Context, tenantID uint64) ([]types.AgentUpgradeProposalEntity, error) {
	rows := []types.AgentUpgradeProposalEntity{}
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

// TransitionProposal is the compare-and-set state move: the update applies
// only when the current state is one of expectedFrom, mirroring
// UpdateVariantState. Paired stamps: naming an actor (resolved_by) also
// stamps updated_at.
func (r *agentUpgradeRepository) TransitionProposal(ctx context.Context, tenantID uint64, proposalID string, expectedFrom []string, nextState string, updates map[string]any) (*types.AgentUpgradeProposalEntity, error) {
	var updated *types.AgentUpgradeProposalEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		id := strings.TrimSpace(proposalID)
		if nextState == "accepted" {
			var proposal types.AgentUpgradeProposalEntity
			if err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).First(&proposal).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrAgentUpgradeProposalNotFound
				}
				return err
			}
			// Lock in the same listing -> release -> adoption order as adoption
			// creation/reconciliation. Keep all guards in the CAS transaction.
			if err := requireListedListing(tx, tenantID, proposal.ListingID); err != nil {
				if isUpgradeLifecycleConflict(err) {
					return ErrAgentUpgradeProposalTransition
				}
				return err
			}
			if err := requireActiveRelease(tx, tenantID, proposal.ToReleaseID, proposal.ListingID); err != nil {
				if isUpgradeLifecycleConflict(err) {
					return ErrAgentUpgradeProposalTransition
				}
				return err
			}
			// This final source check runs after the service has created its
			// intentionally separate draft Variant.
			if err := lockAdoptionState(tx, tenantID, proposal.AdoptionID, "active"); err != nil {
				if isUpgradeLifecycleConflict(err) {
					return ErrAgentUpgradeProposalTransition
				}
				return err
			}
			var adoption types.AgentAdoptionEntity
			if err := tx.Where("tenant_id = ? AND id = ?", tenantID, proposal.AdoptionID).First(&adoption).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrAgentUpgradeProposalTransition
				}
				return err
			}
			if adoption.State != "active" || adoption.ListingID != proposal.ListingID || adoption.AcceptedReleaseID != proposal.FromReleaseID {
				return ErrAgentUpgradeProposalTransition
			}
		}
		set := map[string]any{"state": nextState, "updated_at": time.Now().UTC()}
		for key, value := range updates {
			set[key] = value
		}
		result := tx.Model(&types.AgentUpgradeProposalEntity{}).
			Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, id, expectedFrom).
			Updates(set)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			var current types.AgentUpgradeProposalEntity
			err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).First(&current).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentUpgradeProposalNotFound
			}
			if err != nil {
				return err
			}
			return ErrAgentUpgradeProposalTransition
		}
		var row types.AgentUpgradeProposalEntity
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
			return err
		}
		updated = &row
		return nil
	})
	return updated, err
}

func isUpgradeLifecycleConflict(err error) bool {
	return errors.Is(err, ErrAgentAdoptionNotFound) ||
		errors.Is(err, ErrAgentAdoptionTransition) ||
		errors.Is(err, ErrAgentMarketplaceNotFound) ||
		errors.Is(err, ErrAgentMarketplaceListingTransition) ||
		errors.Is(err, ErrAgentMarketplaceListingUnavailable) ||
		errors.Is(err, ErrAgentMarketplaceReleaseDeprecated)
}
