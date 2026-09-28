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
	inserted := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&created)
	if inserted.Error != nil {
		return nil, false, inserted.Error
	}
	if inserted.RowsAffected == 1 {
		return &created, true, nil
	}
	// 输给了 uq_agent_upgrade_proposals_scope：重读赢家行，与顺序重放同收敛。
	winner, err := key()
	if err != nil {
		return nil, false, err
	}
	return winner, false, nil
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
	set := map[string]any{"state": nextState, "updated_at": time.Now().UTC()}
	for key, value := range updates {
		set[key] = value
	}
	if nextState == "accepted" {
		var updated *types.AgentUpgradeProposalEntity
		err := withTenantSecurityGuard(ctx, r.db, tenantID, func(tx *gorm.DB) error {
			var proposal types.AgentUpgradeProposalEntity
			if err := tx.Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(proposalID)).Take(&proposal).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrAgentUpgradeProposalNotFound
				}
				return err
			}
			var release types.AgentReleaseEntity
			err := tx.Where("tenant_id = ? AND id = ? AND listing_id = ?", tenantID, proposal.ToReleaseID, proposal.ListingID).Take(&release).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				var introduced types.TenantIntroducedReleaseEntity
				err = tx.Where("tenant_id = ? AND id = ? AND public_listing_id = ?", tenantID, proposal.ToReleaseID, proposal.ListingID).Take(&introduced).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrAgentUpgradeProposalTransition
				}
			}
			if err != nil {
				return err
			}
			if err := checkReleaseAdmissionTx(tx, tenantID, proposal.ToReleaseID); err != nil {
				return err
			}
			return transitionProposalTx(tx, tenantID, proposalID, expectedFrom, nextState, set, &updated)
		})
		return updated, err
	}
	result := r.db.WithContext(ctx).Model(&types.AgentUpgradeProposalEntity{}).
		Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, strings.TrimSpace(proposalID), expectedFrom).
		Updates(set)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		var current types.AgentUpgradeProposalEntity
		err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(proposalID)).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentUpgradeProposalNotFound
		}
		if err != nil {
			return nil, err
		}
		return nil, ErrAgentUpgradeProposalTransition
	}
	return r.GetProposal(ctx, tenantID, proposalID)
}

func transitionProposalTx(tx *gorm.DB, tenantID uint64, proposalID string, expectedFrom []string, nextState string, set map[string]any, updated **types.AgentUpgradeProposalEntity) error {
	result := tx.Model(&types.AgentUpgradeProposalEntity{}).
		Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, strings.TrimSpace(proposalID), expectedFrom).
		Updates(set)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		var current types.AgentUpgradeProposalEntity
		err := tx.Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(proposalID)).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAgentUpgradeProposalNotFound
		}
		if err != nil {
			return err
		}
		return ErrAgentUpgradeProposalTransition
	}
	var current types.AgentUpgradeProposalEntity
	if err := tx.Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(proposalID)).Take(&current).Error; err != nil {
		return err
	}
	*updated = &current
	return nil
}
