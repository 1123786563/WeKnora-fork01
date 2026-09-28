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
	const query = `SELECT
		(SELECT COUNT(DISTINCT i.tenant_id)
		 FROM tenant_introduced_releases AS i
		 WHERE i.public_release_id = ?) AS introductions,
		(SELECT COUNT(DISTINCT a.tenant_id)
		 FROM agent_adoptions AS a
		 JOIN tenant_introduced_releases AS i ON i.tenant_id = a.tenant_id AND i.id = a.accepted_release_id
		 WHERE i.public_release_id = ? AND a.state = ?) AS active_adopters,
		(SELECT COUNT(DISTINCT p.tenant_id)
		 FROM agent_upgrade_proposals AS p
		 JOIN tenant_introduced_releases AS i ON i.tenant_id = p.tenant_id AND i.id = p.to_release_id
		 WHERE i.public_release_id = ? AND p.created_at >= ? AND p.created_at < ?) AS upgrade_proposals,
		(SELECT COUNT(DISTINCT p.tenant_id)
		 FROM agent_upgrade_proposals AS p
		 JOIN tenant_introduced_releases AS i ON i.tenant_id = p.tenant_id AND i.id = p.to_release_id
		 WHERE i.public_release_id = ? AND p.created_at >= ? AND p.created_at < ? AND p.state = ?) AS accepted_upgrades`
	err := r.db.WithContext(ctx).Raw(query,
		publicReleaseID,
		publicReleaseID, "active",
		publicReleaseID, upgradeSince.UTC(), asOf.UTC(),
		publicReleaseID, upgradeSince.UTC(), asOf.UTC(), "accepted",
	).Scan(&result).Error
	return result, err
}
