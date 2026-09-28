package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.TenantIntroducedReleaseEntity{}, &types.AgentAdoptionEntity{}, &types.AgentUpgradeProposalEntity{}))
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	for tenantID := uint64(1); tenantID <= 3; tenantID++ {
		localID := "local-" + string(rune('0'+tenantID))
		require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{ID: localID, TenantID: tenantID, PublicReleaseID: "public-release", PublicListingID: "listing", DisplayName: "PRIVATE_DISPLAY_MARKER", Summary: "PRIVATE_SUMMARY_MARKER", SemanticVersion: "1.0.0", BundleDigest: "digest", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("{}"), IntroducedBy: "PRIVATE_MEMBER_MARKER", IntroducedAt: now}).Error)
		if tenantID == 1 {
			require.NoError(t, db.Create(&types.AgentAdoptionEntity{ID: "adoption-1", TenantID: tenantID, ListingID: "listing", AcceptedReleaseID: localID, State: "active"}).Error)
			require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{ID: "proposal-1", TenantID: tenantID, AdoptionID: "adoption-1", ListingID: "listing", ToReleaseID: localID, DiffJSON: `{"private":"PRIVATE_DIFF_MARKER"}`, State: "accepted", ResolvedBy: "PRIVATE_RESOLVER_MARKER", CreatedAt: now.Add(-time.Hour), UpdatedAt: now}).Error)
		}
	}
	repo := repository.NewMarketplaceMetricsRepository(db)
	svc := NewMarketplaceMetricsService(repo)
	svc.now = func() time.Time { return now }
	got, err := svc.MetricsForRelease(context.Background(), "public-release")
	require.NoError(t, err)
	body, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, `{"introductions_bucket":"suppressed","active_adopters_bucket":"suppressed","upgrade_proposals_bucket":"suppressed","accepted_upgrades_bucket":"suppressed","error_category_availability":"not_collected"}`, string(body))
	for _, marker := range []string{"PRIVATE_DISPLAY_MARKER", "PRIVATE_SUMMARY_MARKER", "PRIVATE_MEMBER_MARKER", "PRIVATE_DIFF_MARKER", "PRIVATE_RESOLVER_MARKER", "1", "3"} {
		require.NotContains(t, string(body), marker)
	}
}
