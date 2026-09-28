package interfaces

import "context"

// MarketplaceMetricsService exposes a fixed, privacy-preserving projection
// for one public release. Callers cannot choose dimensions or time windows.
type MarketplaceMetricsService interface {
	MetricsForRelease(ctx context.Context, releaseID string) (MarketplaceMetricsView, error)
}

// MarketplaceMetricsView is intentionally a closed set of coarse buckets.
type MarketplaceMetricsView struct {
	IntroductionsBucket       string `json:"introductions_bucket"`
	ActiveAdoptersBucket      string `json:"active_adopters_bucket"`
	UpgradeProposalsBucket    string `json:"upgrade_proposals_bucket"`
	AcceptedUpgradesBucket    string `json:"accepted_upgrades_bucket"`
	ErrorCategoryAvailability string `json:"error_category_availability"`
}
