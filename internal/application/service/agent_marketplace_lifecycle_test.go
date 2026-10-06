package service

// T33 #63 lifecycle service tests over the REAL migration stream
// (openAgentVersionServiceTestDB) + the REAL repositories. Lower-interface
// evidence below the Task 6 HTTP e2e — labeled as such.

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newLifecycleServiceForTest(t *testing.T) (*AgentMarketplaceLifecycleService, *AgentAdoptionService, *AgentUpgradeService, *gorm.DB) {
	t.Helper()
	db := openAgentVersionServiceTestDB(t)
	adoptions := repository.NewAgentAdoptionRepository(db)
	customAgents := NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil, nil)
	versions := NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	return NewAgentMarketplaceLifecycleService(adoptions, repository.NewAgentMarketplaceRepository(db)),
		NewAgentAdoptionService(adoptions, customAgents, versions),
		NewAgentUpgradeService(repository.NewAgentUpgradeRepository(db)),
		db
}

func seedLifecycleFixture(t *testing.T, db *gorm.DB, wantVariants map[string]string) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test') ON CONFLICT(id) DO NOTHING`).Error)
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "a", DisplayName: "d", State: "listed"}).Error)
	seedLifecycleRelease(t, db, types.AgentReleaseEntity{TenantID: 1, ID: "r1", ListingID: "l1", SubmissionID: "s1", AgentVersionID: "av1", SourceAgentID: "a", ReleaseNumber: 1, SemanticVersion: "1.0.0", BundleDigest: "d1", ManifestJSON: `{"capability_requirements":[]}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b")})
	require.NoError(t, db.Model(&types.AgentMarketplaceListingEntity{}).Where("tenant_id = ? AND id = ?", 1, "l1").Update("current_release_id", "r1").Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "ad1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active", CreatedBy: "admin"}).Error)
	for id, state := range wantVariants {
		localAgentID := ""
		if id == "v1" {
			localAgentID = "agent-x"
		}
		require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{TenantID: 1, ID: id, AdoptionID: "ad1", ReleaseID: "r1", Name: id, State: state, LocalAgentID: localAgentID}).Error)
	}
}

// seedLifecycleRelease creates the schema-required version and submission
// parents before inserting a release; lifecycle tests still exercise the real
// migration schema with foreign keys enabled.
func seedLifecycleRelease(t *testing.T, db *gorm.DB, release types.AgentReleaseEntity) {
	t.Helper()
	if string(release.Bundle) == "b" {
		release.Bundle = []byte(upgradeBundleV1)
	}
	var existingVersions int64
	require.NoError(t, db.Model(&types.AgentVersionEntity{}).Where("tenant_id = ? AND agent_id = ?", release.TenantID, release.SourceAgentID).Count(&existingVersions).Error)
	require.NoError(t, db.Create(&types.AgentVersionEntity{ID: release.AgentVersionID, TenantID: release.TenantID, AgentID: release.SourceAgentID, VersionNumber: int(existingVersions) + 1, Snapshot: `{}`, SourceSHA256: "sha", FrozenBy: "admin"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseSubmissionEntity{ID: release.SubmissionID, TenantID: release.TenantID, ListingID: release.ListingID, AgentVersionID: release.AgentVersionID, SourceAgentID: release.SourceAgentID, AuthorID: "admin", SemanticVersion: release.SemanticVersion, BundleDigest: release.BundleDigest, ManifestJSON: release.ManifestJSON, DependencyLockJSON: release.DependencyLockJSON, Bundle: release.Bundle, Status: "approved"}).Error)
	require.NoError(t, db.Create(&release).Error)
}

func seedListingTwo(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l2", SourceAgentID: "a2", DisplayName: "d2", State: "listed"}).Error)
	for _, row := range []types.AgentReleaseEntity{
		{TenantID: 1, ID: "r2", ListingID: "l2", SubmissionID: "s2", AgentVersionID: "av2", SourceAgentID: "a2", ReleaseNumber: 1, SemanticVersion: "2.0.0", BundleDigest: "d2", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")},
		{TenantID: 1, ID: "r2b", ListingID: "l2", SubmissionID: "s2b", AgentVersionID: "av2b", SourceAgentID: "a2", ReleaseNumber: 2, SemanticVersion: "2.1.0", BundleDigest: "d2b", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")},
	} {
		seedLifecycleRelease(t, db, row)
	}
	require.NoError(t, db.Model(&types.AgentMarketplaceListingEntity{}).Where("tenant_id = ? AND id = ?", 1, "l2").Update("current_release_id", "r2").Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "ad-l2", ListingID: "l2", AcceptedReleaseID: "r2", State: "active", CreatedBy: "admin"}).Error)
}

func TestRetireVariantIsCASAndKeepsRows(t *testing.T) {
	lifecycle, adoptions, _, db := newLifecycleServiceForTest(t)
	ctx := context.Background()
	seedLifecycleFixture(t, db, map[string]string{"v1": "published"})
	retired, err := lifecycle.RetireVariant(ctx, 1, "admin", "v1")
	require.NoError(t, err)
	require.Equal(t, "retired", retired.State)
	require.Equal(t, "admin", retired.RetiredBy)
	require.NotNil(t, retired.RetiredAt)

	_, err = lifecycle.RetireVariant(ctx, 1, "admin", "v1")
	require.ErrorIs(t, err, repository.ErrAgentAdoptionVariantTransition)
	_, err = adoptions.UpdateCapabilityMapping(ctx, 1, "admin", "v1", nil)
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)
	_, err = adoptions.TestVariant(ctx, 1, "admin", "v1")
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)

	var variantCount, adoptionCount, releaseCount int64
	require.NoError(t, db.Model(&types.AgentAdoptionVariantEntity{}).Where("tenant_id = ?", 1).Count(&variantCount).Error)
	require.NoError(t, db.Model(&types.AgentAdoptionEntity{}).Where("tenant_id = ?", 1).Count(&adoptionCount).Error)
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("tenant_id = ?", 1).Count(&releaseCount).Error)
	require.EqualValues(t, 1, variantCount)
	require.EqualValues(t, 1, adoptionCount)
	require.EqualValues(t, 1, releaseCount)
}

func TestEndAdoptionGateAndPostconditions(t *testing.T) {
	lifecycle, adoptions, upgrades, db := newLifecycleServiceForTest(t)
	ctx := context.Background()
	seedLifecycleFixture(t, db, map[string]string{"v1": "draft"})
	_, err := lifecycle.EndAdoption(ctx, 1, "admin", "ad1")
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)
	require.NoError(t, db.Exec("UPDATE agent_adoption_variants SET state='retired' WHERE id='v1'").Error)
	view, err := lifecycle.EndAdoption(ctx, 1, "admin", "ad1")
	require.NoError(t, err)
	require.Equal(t, "ended", view.State)
	_, err = adoptions.CreateVariant(ctx, 1, "admin", "ad1", interfaces.VariantDraftInput{Name: "x"})
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)
	_, _, err = repository.NewAgentUpgradeRepository(db).FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
		TenantID: 1, AdoptionID: "ad1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r1", State: "open",
	})
	require.NoError(t, err)
	_, _, err = upgrades.AcceptUpgradeProposal(ctx, 1, "admin", "p1", interfaces.UpgradeVariantInput{Name: "y"})
	require.ErrorIs(t, err, ErrAgentUpgradeNotFound)
	proposals, err := upgrades.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	_, _, err = upgrades.AcceptUpgradeProposal(ctx, 1, "admin", proposals[0].ID, interfaces.UpgradeVariantInput{Name: "y"})
	require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)
}

func TestUnlistAndDeprecateBehaviorDiffers(t *testing.T) {
	lifecycle, adoptions, _, db := newLifecycleServiceForTest(t)
	ctx := context.Background()
	seedLifecycleFixture(t, db, nil)
	seedListingTwo(t, db)
	view, err := lifecycle.UnlistListing(ctx, 1, "admin", "l1")
	require.NoError(t, err)
	require.Equal(t, "unlisted", view.State)
	_, _, err = adoptions.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: "l1"})
	require.ErrorIs(t, err, ErrAgentAdoptionInvalidInput)
	release, err := lifecycle.DeprecateRelease(ctx, 1, "admin", "r2", "r2b")
	require.NoError(t, err)
	require.Equal(t, "r2b", release.SuccessorReleaseID)
	_, _, err = adoptions.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: "l2", ReleaseID: "r2"})
	require.ErrorIs(t, err, ErrAgentReleaseDeprecated)
	require.Contains(t, err.Error(), "r2b")
	_, err = adoptions.CreateVariant(ctx, 1, "admin", "ad-l2", interfaces.VariantDraftInput{Name: "z", ReleaseID: "r2"})
	require.ErrorIs(t, err, ErrAgentReleaseDeprecated)
}

func TestDeprecateSuccessorValidation(t *testing.T) {
	lifecycle, _, _, db := newLifecycleServiceForTest(t)
	ctx := context.Background()
	seedLifecycleFixture(t, db, nil)
	seedListingTwo(t, db)
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l3", SourceAgentID: "a3", DisplayName: "d3", State: "listed"}).Error)
	seedLifecycleRelease(t, db, types.AgentReleaseEntity{TenantID: 1, ID: "r3", ListingID: "l3", SubmissionID: "s3", AgentVersionID: "av3", SourceAgentID: "a3", ReleaseNumber: 1, SemanticVersion: "3.0.0", BundleDigest: "d3", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")})
	now := time.Now().UTC()
	seedLifecycleRelease(t, db, types.AgentReleaseEntity{TenantID: 1, ID: "r4", ListingID: "l2", SubmissionID: "s4", AgentVersionID: "av4", SourceAgentID: "a2", ReleaseNumber: 3, SemanticVersion: "2.2.0", BundleDigest: "d4", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b"), DeprecatedAt: &now, DeprecatedBy: "admin", SuccessorReleaseID: "r2b"})
	for _, tc := range []struct{ name, successor string }{
		{"missing", "nope"}, {"self", "r1"}, {"cross-listing", "r3"}, {"already-deprecated", "r4"}, {"empty", ""},
	} {
		_, err := lifecycle.DeprecateRelease(ctx, 1, "admin", "r1", tc.successor)
		require.ErrorIs(t, err, ErrAgentMarketplaceLifecycleInvalidInput, tc.name)
	}
	_, err := lifecycle.DeprecateRelease(ctx, 1, "admin", "missing-release", "r2b")
	require.ErrorIs(t, err, repository.ErrAgentMarketplaceNotFound)
}

func TestUpgradeReconcileSkipsDeprecatedTarget(t *testing.T) {
	lifecycle, _, upgrades, db := newLifecycleServiceForTest(t)
	ctx := context.Background()
	seedLifecycleFixture(t, db, nil)
	seedLifecycleRelease(t, db, types.AgentReleaseEntity{TenantID: 1, ID: "r2", ListingID: "l1", SubmissionID: "s2", AgentVersionID: "av2", SourceAgentID: "a", ReleaseNumber: 2, SemanticVersion: "1.1.0", BundleDigest: "d2", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")})
	require.NoError(t, db.Exec("UPDATE agent_marketplace_listings SET current_release_id='r2' WHERE id='l1'").Error)
	_, err := lifecycle.DeprecateRelease(ctx, 1, "admin", "r2", "r1")
	require.NoError(t, err)
	proposals, err := upgrades.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, proposals)
	require.NoError(t, db.Exec("UPDATE agent_releases SET deprecated_at=NULL, successor_release_id='' WHERE id='r2'").Error)
	proposals, err = upgrades.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	require.Equal(t, "r2", proposals[0].ToReleaseID)
}
