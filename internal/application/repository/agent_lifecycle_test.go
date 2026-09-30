package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestAgentLifecycleRepositoryTenantCASAndAdmission(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "lifecycle-agent", "1.0.0")
	adoptions := NewAgentAdoptionRepository(db)
	adoption, _, err := adoptions.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active"})
	require.NoError(t, err)
	variant, err := adoptions.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adoption.ID, ReleaseID: releaseID, Name: "Lifecycle", State: "published", LocalAgentID: "local-lifecycle", LocalAgentVersionID: "local-v1"})
	require.NoError(t, err)

	_, err = adoptions.RetireVariant(ctx, 2, variant.ID, "admin-2", "wrong tenant")
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound)
	_, err = adoptions.RetireVariant(ctx, 1, "missing", "admin", "missing")
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound)
	retired, err := adoptions.RetireVariant(ctx, 1, variant.ID, "admin", "replaced")
	require.NoError(t, err)
	require.Equal(t, "retired", retired.State)
	require.Equal(t, "admin", retired.RetiredBy)
	require.Equal(t, "replaced", retired.RetirementReason)
	require.NotNil(t, retired.RetiredAt)
	require.Equal(t, time.UTC, retired.RetiredAt.Location())
	_, err = adoptions.RetireVariant(ctx, 1, variant.ID, "other", "repeat")
	require.ErrorIs(t, err, ErrAgentAdoptionVariantTransition)
	invalid, err := adoptions.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adoption.ID, ReleaseID: releaseID, Name: "Invalid", State: "unknown"})
	require.NoError(t, err)
	_, err = adoptions.RetireVariant(ctx, 1, invalid.ID, "admin", "invalid state")
	require.ErrorIs(t, err, ErrAgentAdoptionVariantTransition)
	require.NoError(t, db.Delete(&types.AgentAdoptionVariantEntity{}, "tenant_id = ? AND id = ?", 1, invalid.ID).Error)

	retiredAdmission, err := adoptions.IsRetiredMarketplaceAgent(ctx, 1, "local-lifecycle")
	require.NoError(t, err)
	require.True(t, retiredAdmission)
	ordinaryAdmission, err := adoptions.IsRetiredMarketplaceAgent(ctx, 1, "ordinary-agent")
	require.NoError(t, err)
	require.False(t, ordinaryAdmission, "an ordinary local Agent must not require a marketplace row")
	otherTenantAdmission, err := adoptions.IsRetiredMarketplaceAgent(ctx, 2, "local-lifecycle")
	require.NoError(t, err)
	require.False(t, otherTenantAdmission)

	_, err = adoptions.EndAdoption(ctx, 1, adoption.ID, "admin", "done")
	require.NoError(t, err)
	ended, err := adoptions.GetAdoption(ctx, 1, adoption.ID)
	require.NoError(t, err)
	require.Equal(t, "ended", ended.State)
	require.Equal(t, "admin", ended.EndedBy)
	require.Equal(t, "done", ended.EndReason)
	require.NotNil(t, ended.EndedAt)
	_, err = adoptions.EndAdoption(ctx, 1, adoption.ID, "other", "repeat")
	require.ErrorIs(t, err, ErrAgentAdoptionTransition)
}

func TestAgentLifecycleRepositoryAdoptionEndRequiresRetiredVariants(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "active-variant-agent", "1.0.0")
	repo := NewAgentAdoptionRepository(db)
	adoption, _, err := repo.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID})
	require.NoError(t, err)
	_, err = repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adoption.ID, ReleaseID: releaseID, Name: "Draft", State: "draft"})
	require.NoError(t, err)
	_, err = repo.EndAdoption(ctx, 1, adoption.ID, "admin", "done")
	require.ErrorIs(t, err, ErrAgentAdoptionTransition)
}

func TestAgentLifecycleRepositoryConcurrentVariantRetirementCAS(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "race-agent", "1.0.0")
	repo := NewAgentAdoptionRepository(db)
	adoption, _, err := repo.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID})
	require.NoError(t, err)
	variant, err := repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adoption.ID, ReleaseID: releaseID, Name: "Race", State: "published", LocalAgentID: "race-local"})
	require.NoError(t, err)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, actor := range []string{"a", "b"} {
		wg.Add(1)
		go func(actor string) {
			defer wg.Done()
			_, err := repo.RetireVariant(ctx, 1, variant.ID, actor, "retired")
			results <- err
		}(actor)
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if err == ErrAgentAdoptionVariantTransition {
			conflicts++
		} else {
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	stored, err := repo.GetVariant(ctx, 1, variant.ID)
	require.NoError(t, err)
	require.Contains(t, []string{"a", "b"}, stored.RetiredBy)
}

func TestAgentLifecycleMarketplaceAndPublicCASPreserveReleaseBytes(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "tenant-listing-agent", "1.0.0")
	var tenantRelease types.AgentReleaseEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, releaseID).First(&tenantRelease).Error)
	secondTenantRelease := tenantRelease
	secondTenantRelease.ID = "tenant-replacement"
	secondTenantRelease.ReleaseNumber = tenantRelease.ReleaseNumber + 1
	secondTenantRelease.SemanticVersion = "2.0.0"
	secondTenantRelease.BundleDigest = "digest-tenant-replacement"
	require.NoError(t, db.Create(&secondTenantRelease).Error)

	tenantRepo := NewAgentMarketplaceRepository(db)
	_, err := tenantRepo.UnlistTenantListing(ctx, 2, listingID, "tenant-admin", "paused")
	require.ErrorIs(t, err, ErrAgentMarketplaceNotFound)
	_, err = tenantRepo.UnlistTenantListing(ctx, 1, listingID, "tenant-admin", "paused")
	require.NoError(t, err)
	_, err = tenantRepo.UnlistTenantListing(ctx, 1, listingID, "tenant-admin", "again")
	require.ErrorIs(t, err, ErrAgentMarketplaceLifecycleTransition)
	_, err = tenantRepo.DeprecateTenantRelease(ctx, 1, releaseID, releaseID, "tenant-admin", "replaced")
	require.ErrorIs(t, err, ErrAgentMarketplaceLifecycleInvalid)
	deprecated, err := tenantRepo.DeprecateTenantRelease(ctx, 1, releaseID, secondTenantRelease.ID, "tenant-admin", "replaced")
	require.NoError(t, err)
	require.Equal(t, secondTenantRelease.ID, deprecated.ReplacementReleaseID)
	require.Equal(t, "tenant-admin", deprecated.DeprecatedBy)
	require.Equal(t, "replaced", deprecated.DeprecationReason)
	require.NotNil(t, deprecated.DeprecatedAt)
	_, err = tenantRepo.DeprecateTenantRelease(ctx, 1, releaseID, secondTenantRelease.ID, "other", "repeat")
	require.ErrorIs(t, err, ErrAgentMarketplaceLifecycleTransition)
	storedTenantRelease, err := tenantRepo.GetRelease(ctx, 1, releaseID)
	require.NoError(t, err)
	require.Equal(t, tenantRelease.Bundle, storedTenantRelease.Bundle)
	require.Equal(t, tenantRelease.BundleDigest, storedTenantRelease.BundleDigest)

}

func TestAgentLifecyclePublicMarketplaceCASPreservesReleaseBytes(t *testing.T) {
	db := openPublicMarketplaceDB(t)
	ctx := context.Background()
	publicListing, publicRelease := seedApprovedPublicRelease(t, db, "1.0.0")
	_, replacement := seedApprovedPublicRelease(t, db, "2.0.0")
	publicRepo := NewPublicMarketplaceRepository(db)
	var err error
	_, err = publicRepo.UnlistPublicListing(ctx, "missing-listing", "platform-admin", "paused")
	require.ErrorIs(t, err, ErrPublicMarketplaceNotFound)
	_, err = publicRepo.UnlistPublicListing(ctx, publicListing.ID, "platform-admin", "paused")
	require.NoError(t, err)
	_, err = publicRepo.UnlistPublicListing(ctx, publicListing.ID, "platform-admin", "again")
	require.ErrorIs(t, err, ErrPublicMarketplaceLifecycleTransition)
	_, err = publicRepo.DeprecatePublicRelease(ctx, publicRelease.ID, replacement.ID, "platform-admin", "replaced")
	require.NoError(t, err)
	_, err = publicRepo.DeprecatePublicRelease(ctx, publicRelease.ID, replacement.ID, "other", "repeat")
	require.ErrorIs(t, err, ErrPublicMarketplaceLifecycleTransition)
	storedPublicRelease, err := publicRepo.GetPublicRelease(ctx, publicRelease.ID)
	require.NoError(t, err)
	require.Equal(t, publicRelease.Bundle, storedPublicRelease.Bundle)
	require.Equal(t, publicRelease.BundleDigest, storedPublicRelease.BundleDigest)
}
