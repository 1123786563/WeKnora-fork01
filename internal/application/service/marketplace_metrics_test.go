package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
)

type metricsRepoStub struct {
	result      repository.MarketplaceMetricsAggregate
	err         error
	asOf, since time.Time
}

func (m *metricsRepoStub) AggregateForRelease(_ context.Context, _ string, asOf, since time.Time) (repository.MarketplaceMetricsAggregate, error) {
	m.asOf, m.since = asOf, since
	return m.result, m.err
}

func TestMarketplaceMetricsServiceBucketsEachMeasureAndFixesWindow(t *testing.T) {
	repo := &metricsRepoStub{result: repository.MarketplaceMetricsAggregate{Introductions: 4, ActiveAdopters: 5, UpgradeProposals: 9, AcceptedUpgrades: 20}}
	svc := NewMarketplaceMetricsService(repo)
	svc.now = func() time.Time { return time.Date(2026, 9, 28, 1, 2, 3, 0, time.FixedZone("offset", 8*3600)) }
	got, err := svc.MetricsForRelease(context.Background(), "public-release")
	require.NoError(t, err)
	require.Equal(t, "suppressed", got.IntroductionsBucket)
	require.Equal(t, "5-9", got.ActiveAdoptersBucket)
	require.Equal(t, "5-9", got.UpgradeProposalsBucket)
	require.Equal(t, "20+", got.AcceptedUpgradesBucket)
	require.Equal(t, "not_collected", got.ErrorCategoryAvailability)
	require.Equal(t, time.Date(2026, 9, 27, 17, 2, 3, 0, time.UTC), repo.asOf)
	require.Equal(t, repo.asOf.Add(-30*24*time.Hour), repo.since)
}

func TestMarketplaceMetricsBucketingBoundaries(t *testing.T) {
	for _, tc := range []struct {
		count int64
		want  string
	}{{0, "suppressed"}, {4, "suppressed"}, {5, "5-9"}, {9, "5-9"}, {10, "10-19"}, {19, "10-19"}, {20, "20+"}} {
		require.Equal(t, tc.want, marketplaceMetricsBucket(tc.count))
	}
}

func TestMarketplaceMetricsWireShapeIsClosedAndContainsNoRawValues(t *testing.T) {
	repo := &metricsRepoStub{result: repository.MarketplaceMetricsAggregate{Introductions: 5, ActiveAdopters: 10, UpgradeProposals: 19, AcceptedUpgrades: 20}}
	svc := NewMarketplaceMetricsService(repo)
	got, err := svc.MetricsForRelease(context.Background(), "public-release")
	require.NoError(t, err)
	body, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, `{"introductions_bucket":"5-9","active_adopters_bucket":"10-19","upgrade_proposals_bucket":"10-19","accepted_upgrades_bucket":"20+","error_category_availability":"not_collected"}`, string(body))
	for _, marker := range []string{"tenant-secret", "member-secret", "task-title-secret", "diff-secret", "error-secret", "metadata-secret", "input-secret", "output-secret", "mapping-secret"} {
		require.NotContains(t, string(body), marker)
	}
}
