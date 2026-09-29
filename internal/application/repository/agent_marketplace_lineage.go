package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Tencent/WeKnora/internal/types"
)

// Fork lineage + license registry SQL for the Marketplace repository
// (T32 #62). Methods live on agentMarketplaceRepository (declared in
// agent_marketplace.go) but sit in this file so the lineage feature stays
// one reviewable unit. All reads and writes are parameter-bound.

// FindDerivation resolves the adoption lineage of a variant-published
// local agent: the latest variant whose local_agent_id matches, its
// Adoption's Listing and the pinned source Release (tenant-local row
// first, the #60 introduction-ledger synthesis second). Returns
// (nil, nil) when the agent is not derived — original content has no
// lineage and is never gated.
func (r *agentMarketplaceRepository) FindDerivation(ctx context.Context, tenantID uint64, localAgentID string) (*types.AgentForkDerivation, error) {
	localAgentID = strings.TrimSpace(localAgentID)
	if tenantID == 0 || localAgentID == "" {
		return nil, nil
	}
	var variant types.AgentAdoptionVariantEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND local_agent_id = ?", tenantID, localAgentID).
		Order("updated_at DESC, id DESC").First(&variant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var adoption types.AgentAdoptionEntity
	err = r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, variant.AdoptionID).First(&adoption).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// The governance rows were archived away (#63 owns that flow):
		// without the Adoption there is no lineage left to preserve.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	release, err := r.getDerivationRelease(ctx, tenantID, variant.ReleaseID)
	if err != nil {
		return nil, err
	}
	return &types.AgentForkDerivation{Variant: variant, ListingID: adoption.ListingID, Release: release}, nil
}

// getDerivationRelease reads the pinned source Release: the tenant-local
// row first, the introduction-ledger fallback second (the same synthesis
// the #59 adoption chain consumes via GetRelease).
func (r *agentMarketplaceRepository) getDerivationRelease(ctx context.Context, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error) {
	var row types.AgentReleaseEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(releaseID)).First(&row).Error
	if err == nil {
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return introducedRelease(r.db.WithContext(ctx), tenantID, releaseID)
}

// UpsertLicense registers (or re-registers) one deployment license term.
// The upsert is how a license flip propagates to later submissions. Per the
// entity contract, a re-register updates the FLAGS ONLY: the first
// registrar and its created_at are audit facts and are never overwritten
// (R5-F3). The returned row is re-read from the store — the caller renders
// it directly, so it must be the stored truth, not the input copy.
func (r *agentMarketplaceRepository) UpsertLicense(ctx context.Context, license *types.AgentLicenseEntity) (*types.AgentLicenseEntity, error) {
	if license == nil || strings.TrimSpace(license.ID) == "" {
		return nil, ErrAgentLicenseIDRequired
	}
	now := time.Now().UTC()
	license.CreatedAt = now
	license.UpdatedAt = now
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "allows_redistribution", "updated_at"}),
		}).
		Create(license).Error
	if err != nil {
		return nil, err
	}
	return r.GetLicense(ctx, license.ID)
}

// GetLicense returns the license row, nil when unregistered (the service
// layer decides that fail closed).
func (r *agentMarketplaceRepository) GetLicense(ctx context.Context, licenseID string) (*types.AgentLicenseEntity, error) {
	var row types.AgentLicenseEntity
	err := r.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(licenseID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListLicenses returns the deployment license registry ordered by id.
func (r *agentMarketplaceRepository) ListLicenses(ctx context.Context) ([]types.AgentLicenseEntity, error) {
	rows := make([]types.AgentLicenseEntity, 0)
	err := r.db.WithContext(ctx).Order("id ASC").Find(&rows).Error
	return rows, err
}
