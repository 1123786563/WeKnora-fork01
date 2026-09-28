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

type metricsStatementLogger struct {
	logger.Interface
	statements int
}

func (l *metricsStatementLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.statements++
	l.Interface.Trace(ctx, begin, fc, err)
}

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
	// Boundary proposal tenants have real matching introduction rows. This
	// ensures the boundary exclusion is due to created_at, not a failed join.
	for _, boundary := range []struct {
		tenantID uint64
		localID  string
		proposal string
		created  time.Time
	}{{30, "local-upper", "upper", base}, {31, "local-before", "old", start.Add(-time.Nanosecond)}} {
		require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{ID: boundary.localID, TenantID: boundary.tenantID, PublicReleaseID: pub, PublicListingID: "listing", SemanticVersion: "1.0.0", BundleDigest: "digest", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("{}")}).Error)
		require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{ID: boundary.proposal, TenantID: boundary.tenantID, AdoptionID: "a", ListingID: "listing", ToReleaseID: boundary.localID, CreatedAt: boundary.created, UpdatedAt: base}).Error)
	}
	// Extra valid rows for tenant 1 must not increase the per-measure grain.
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{ID: "adoption-extra", TenantID: 1, ListingID: "listing-extra", AcceptedReleaseID: "local-" + time.Unix(1, 0).UTC().Format("05"), State: "active"}).Error)
	require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{ID: "proposal-extra", TenantID: 1, AdoptionID: "adoption-extra", ListingID: "listing", ToReleaseID: "local-" + time.Unix(1, 0).UTC().Format("05"), DiffJSON: `{"private":"SECOND_DIFF_MARKER"}`, State: "accepted", CreatedAt: start.Add(time.Hour), UpdatedAt: base}).Error)

	got, err := NewMarketplaceMetricsRepository(db).AggregateForRelease(context.Background(), pub, base, start)
	require.NoError(t, err)
	require.EqualValues(t, 22, got.Introductions)
	require.EqualValues(t, 9, got.ActiveAdopters)
	require.EqualValues(t, 10, got.UpgradeProposals)
	require.EqualValues(t, 10, got.AcceptedUpgrades)
}

func TestMarketplaceMetricsAggregateUsesOneSelectStatement(t *testing.T) {
	statementLogger := &metricsStatementLogger{Interface: logger.Default.LogMode(logger.Info)}
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: statementLogger})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.TenantIntroducedReleaseEntity{}, &types.AgentAdoptionEntity{}, &types.AgentUpgradeProposalEntity{}))
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	statementLogger.statements = 0

	_, err = NewMarketplaceMetricsRepository(db).AggregateForRelease(context.Background(), "public-release", time.Now().UTC(), time.Now().UTC().Add(-30*24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, 1, statementLogger.statements, "all four metrics must share one SQL snapshot")
}
