package service

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const marketplaceMetricsWindow = 30 * 24 * time.Hour

type MarketplaceMetricsService struct {
	repo repository.MarketplaceMetricsRepository
	now  func() time.Time
}

var _ interfaces.MarketplaceMetricsService = (*MarketplaceMetricsService)(nil)

func NewMarketplaceMetricsService(repo repository.MarketplaceMetricsRepository) *MarketplaceMetricsService {
	return &MarketplaceMetricsService{repo: repo, now: time.Now}
}

func (s *MarketplaceMetricsService) MetricsForRelease(ctx context.Context, releaseID string) (interfaces.MarketplaceMetricsView, error) {
	asOf := s.now().UTC()
	aggregate, err := s.repo.AggregateForRelease(ctx, releaseID, asOf, asOf.Add(-marketplaceMetricsWindow))
	if err != nil {
		return interfaces.MarketplaceMetricsView{}, err
	}
	return interfaces.MarketplaceMetricsView{
		IntroductionsBucket:       marketplaceMetricsBucket(aggregate.Introductions),
		ActiveAdoptersBucket:      marketplaceMetricsBucket(aggregate.ActiveAdopters),
		UpgradeProposalsBucket:    marketplaceMetricsBucket(aggregate.UpgradeProposals),
		AcceptedUpgradesBucket:    marketplaceMetricsBucket(aggregate.AcceptedUpgrades),
		ErrorCategoryAvailability: "not_collected",
	}, nil
}

func marketplaceMetricsBucket(count int64) string {
	switch {
	case count < 5:
		return "suppressed"
	case count < 10:
		return "5-9"
	case count < 20:
		return "10-19"
	default:
		return "20+"
	}
}
