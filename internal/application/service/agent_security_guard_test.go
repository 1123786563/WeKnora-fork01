package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type stubReleaseSecurityGate struct{ blockedRelease string }

func (g stubReleaseSecurityGate) ReleaseAdmission(_ context.Context, _ uint64, releaseID string) error {
	if releaseID == g.blockedRelease {
		return fmt.Errorf("%w: release %s is security-revoked", ErrAgentSecurityReleaseBlocked, releaseID)
	}
	return nil
}

func TestAdoptionServiceReleaseSecurityGateRefusesAdoptVariantPublish(t *testing.T) {
	svc, db, _ := newAgentAdoptionServiceForTest(t)
	listingID, r1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	ctx := context.Background()

	// A nil gate leaves the existing flow unchanged.
	before, _, err := svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.NoError(t, err)
	require.NotEmpty(t, before.ID)

	svc.SetReleaseSecurityGate(stubReleaseSecurityGate{blockedRelease: r1})
	_, _, err = svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked, "a revoked current release must block a new adoption")

	// Reuse the successful adoption to isolate the gate from repository conflicts.
	adoptions := repository.NewAgentAdoptionRepository(db)
	_, err = svc.CreateVariant(ctx, 1, "admin", before.ID, interfaces.VariantDraftInput{Name: "Blocked"})
	require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked, "a revoked release must block a new variant")

	variant, err := adoptions.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{
		TenantID: 1, AdoptionID: before.ID, ReleaseID: r1, Name: "Pre-gate", State: AgentVariantStateTested, CreatedBy: "admin",
	})
	require.NoError(t, err)
	_, err = svc.PublishVariant(ctx, 1, "admin", variant.ID)
	require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked, "a revoked release must block local publication")
}

func TestAdoptionServiceReleaseSecurityGatePreservesDeprecationGuard(t *testing.T) {
	svc, db, _ := newAgentAdoptionServiceForTest(t)
	listingID, releaseID := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	deprecatedAt := time.Now().UTC()
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("id = ? AND tenant_id = ?", releaseID, 1).Updates(map[string]any{"deprecated_at": deprecatedAt, "successor_release_id": "next"}).Error)

	svc.SetReleaseSecurityGate(stubReleaseSecurityGate{blockedRelease: "different-release"})
	_, _, err := svc.Adopt(context.Background(), 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.ErrorIs(t, err, ErrAgentReleaseDeprecated, "an allowing security gate must preserve the deprecation predicate")
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)
}

func TestUpgradeServiceReleaseSecurityGateRefusesAccept(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoptUpgradeRelease(t, db, listingID, v1)
	_, v2 := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	ctx := context.Background()

	proposals, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)

	svc.SetReleaseSecurityGate(stubReleaseSecurityGate{blockedRelease: v2})
	_, _, err = svc.AcceptUpgradeProposal(ctx, 1, "admin", proposals[0].ID, interfaces.UpgradeVariantInput{Name: "Up"})
	require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked, "accepting a proposal for a revoked release must be refused")
}
