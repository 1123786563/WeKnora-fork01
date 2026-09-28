package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMarketplaceMetricsAggregatesDistinctTenantsAtFixedGrains(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.TenantIntroducedReleaseEntity{}, &types.AgentAdoptionEntity{}, &types.AgentUpgradeProposalEntity{}))
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })

	const pub = "public-release"
	base := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	start := base.Add(-30 * 24 * time.Hour)
	for i := uint64(1); i <= 20; i++ {
		local := "local-" + time.Unix(int64(i), 0).UTC().Format("05")
		require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{ID: local, TenantID: i, PublicReleaseID: pub, PublicListingID: "listing", DisplayName: "private marker", SemanticVersion: "1.0.0", BundleDigest: "digest", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("{}"), IntroducedBy: "TENANT_MEMBER_MARKER"}).Error)
		// Duplicate source rows for tenant 1 must not increase any measure.
		if i == 1 {
			require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{ID: "duplicate", TenantID: i, PublicReleaseID: pub, PublicListingID: "listing", SemanticVersion: "1.0.0", BundleDigest: "digest", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("{}"), IntroducedBy: "TENANT_MEMBER_MARKER"}).Error)
		}
		state := "ended"
		if i <= 9 {
			state = "active"
		}
		require.NoError(t, db.Create(&types.AgentAdoptionEntity{ID: "adoption-" + local, TenantID: i, ListingID: "listing", AcceptedReleaseID: local, State: state, CreatedAt: base.Add(-365 * 24 * time.Hour)}).Error)
		if i <= 10 {
			require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{ID: "proposal-" + local, TenantID: i, AdoptionID: "adoption-" + local, ListingID: "listing", FromReleaseID: "old", ToReleaseID: local, DiffJSON: `{"private":"DO_NOT_LEAK"}`, State: "accepted", CreatedAt: start, UpdatedAt: base}).Error)
		}
	}
	// Half-open upper boundary excluded; a proposal before the lower boundary is excluded.
	require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{ID: "upper", TenantID: 30, AdoptionID: "a", ListingID: "listing", ToReleaseID: "local-" + time.Unix(1, 0).UTC().Format("05"), CreatedAt: base, UpdatedAt: base}).Error)
	require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{ID: "old", TenantID: 31, AdoptionID: "a", ListingID: "listing", ToReleaseID: "local-" + time.Unix(1, 0).UTC().Format("05"), CreatedAt: start.Add(-time.Nanosecond)}).Error)

	got, err := NewMarketplaceMetricsRepository(db).AggregateForRelease(context.Background(), pub, base, start)
	require.NoError(t, err)
	require.EqualValues(t, 20, got.Introductions)
	require.EqualValues(t, 9, got.ActiveAdopters)
	require.EqualValues(t, 10, got.UpgradeProposals)
	require.EqualValues(t, 10, got.AcceptedUpgrades)
}
