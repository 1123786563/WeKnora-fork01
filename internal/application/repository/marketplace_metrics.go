package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// MarketplaceMetricsAggregate contains internal raw counts. It must never be
// returned from a handler or serialized to an API response.
type MarketplaceMetricsAggregate struct {
	Introductions    int64
	ActiveAdopters   int64
	UpgradeProposals int64
	AcceptedUpgrades int64
}

type MarketplaceMetricsRepository interface {
	AggregateForRelease(ctx context.Context, publicReleaseID string, asOf, upgradeSince time.Time) (MarketplaceMetricsAggregate, error)
}

type marketplaceMetricsRepository struct{ db *gorm.DB }

func NewMarketplaceMetricsRepository(db *gorm.DB) MarketplaceMetricsRepository {
	return &marketplaceMetricsRepository{db: db}
}

// AggregateForRelease computes each measure at its specified grain. Joins
// resolve adopter-local release ids by both tenant_id and local id.
func (r *marketplaceMetricsRepository) AggregateForRelease(ctx context.Context, publicReleaseID string, asOf, upgradeSince time.Time) (MarketplaceMetricsAggregate, error) {
	var result MarketplaceMetricsAggregate
	if err := r.db.WithContext(ctx).Table("tenant_introduced_releases").
		Where("public_release_id = ?", publicReleaseID).
		Distinct("tenant_id").Count(&result.Introductions).Error; err != nil {
		return result, err
	}
	if err := r.db.WithContext(ctx).Table("agent_adoptions AS a").
		Joins("JOIN tenant_introduced_releases AS i ON i.tenant_id = a.tenant_id AND i.id = a.accepted_release_id").
		Where("i.public_release_id = ? AND a.state = ?", publicReleaseID, "active").
		Distinct("a.tenant_id").Count(&result.ActiveAdopters).Error; err != nil {
		return result, err
	}
	proposalQuery := func() *gorm.DB {
		return r.db.WithContext(ctx).Table("agent_upgrade_proposals AS p").
			Joins("JOIN tenant_introduced_releases AS i ON i.tenant_id = p.tenant_id AND i.id = p.to_release_id").
			Where("i.public_release_id = ? AND p.created_at >= ? AND p.created_at < ?", publicReleaseID, upgradeSince.UTC(), asOf.UTC())
	}
	if err := proposalQuery().Distinct("p.tenant_id").Count(&result.UpgradeProposals).Error; err != nil {
		return result, err
	}
	if err := proposalQuery().Where("p.state = ?", "accepted").Distinct("p.tenant_id").Count(&result.AcceptedUpgrades).Error; err != nil {
		return result, err
	}
	return result, nil
}
