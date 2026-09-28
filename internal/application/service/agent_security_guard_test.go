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
	"gorm.io/gorm"
)

type stubReleaseSecurityGate struct{ blockedRelease string }

func (g stubReleaseSecurityGate) ReleaseAdmission(_ context.Context, _ uint64, releaseID string) error {
	if releaseID == g.blockedRelease {
		return fmt.Errorf("%w: release %s is security-revoked", ErrAgentSecurityReleaseBlocked, releaseID)
	}
	return nil
}

type countingReleaseSecurityGate struct {
	blockedRelease string
	calls          []string
}

func (g *countingReleaseSecurityGate) ReleaseAdmission(_ context.Context, _ uint64, releaseID string) error {
	g.calls = append(g.calls, releaseID)
	if releaseID == g.blockedRelease {
		return fmt.Errorf("%w: release %s is security-revoked", ErrAgentSecurityReleaseBlocked, releaseID)
	}
	return nil
}

func TestAdoptionServiceValidatesReleaseMembershipBeforeSecurityGate(t *testing.T) {
	svc, db, _ := newAgentAdoptionServiceForTest(t)
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test') ON CONFLICT(id) DO NOTHING`).Error)
	listingID, validRelease := seedAdoptionServiceRelease(t, db)
	require.NoError(t, db.Create(&types.Tenant{ID: 2, Name: "tenant-2"}).Error)
	_, foreignRelease := seedScopeTestRelease(t, db, 2, "foreign-agent")
	_, wrongListingRelease := seedScopeTestRelease(t, db, 1, "other-agent")
	gate := &countingReleaseSecurityGate{blockedRelease: validRelease}
	svc.SetReleaseSecurityGate(gate)

	for name, releaseID := range map[string]string{
		"missing": "missing-release", "foreign tenant": foreignRelease, "wrong listing": wrongListingRelease,
	} {
		t.Run("adopt/"+name, func(t *testing.T) {
			_, _, err := svc.Adopt(context.Background(), 1, "admin", interfaces.AdoptInput{ListingID: listingID, ReleaseID: releaseID})
			require.ErrorIs(t, err, ErrAgentAdoptionInvalidInput)
			require.NotErrorIs(t, err, ErrAgentSecurityReleaseBlocked)
		})
	}
	require.Empty(t, gate.calls, "invalid release identifiers must not be adjudicated")
	_, _, err := svc.Adopt(context.Background(), 1, "admin", interfaces.AdoptInput{ListingID: listingID, ReleaseID: validRelease})
	require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked)
	require.Equal(t, []string{validRelease}, gate.calls)

	svc.SetReleaseSecurityGate(nil)
	adoption, _, err := svc.Adopt(context.Background(), 1, "admin", interfaces.AdoptInput{ListingID: listingID, ReleaseID: validRelease})
	require.NoError(t, err)
	gate.calls = nil
	svc.SetReleaseSecurityGate(gate)
	for name, releaseID := range map[string]string{
		"missing": "missing-release", "foreign tenant": foreignRelease, "wrong listing": wrongListingRelease,
	} {
		t.Run("variant/"+name, func(t *testing.T) {
			_, err := svc.CreateVariant(context.Background(), 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "invalid", ReleaseID: releaseID})
			require.ErrorIs(t, err, ErrAgentAdoptionInvalidInput)
			require.NotErrorIs(t, err, ErrAgentSecurityReleaseBlocked)
		})
	}
	require.Empty(t, gate.calls, "invalid variant releases must not be adjudicated")
	_, err = svc.CreateVariant(context.Background(), 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "blocked", ReleaseID: validRelease})
	require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked)
	require.Equal(t, []string{validRelease}, gate.calls)
}

func seedScopeTestRelease(t *testing.T, db *gorm.DB, tenantID uint64, agentID string) (string, string) {
	t.Helper()
	ctx := context.Background()
	versionID := "scope-version-" + agentID
	require.NoError(t, db.Exec(`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES (?, ?, ?, 1, '{}', 'sha', 'author')`, versionID, tenantID, agentID).Error)
	manifest := `{"semantic_version":"1.0.0","display_name":"Scope test","summary":"s","supported_languages":["en"],"use_cases":["u"],"capability_requirements":[],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"` + versionID + `","version_number":1,"source_sha256":"sha"}}`
	digest := "digest-" + agentID
	bundle := []byte(`{"payload":{"system_prompt":"p"},"manifest":` + manifest + `,"dependency_lock":{"dependencies":[]}}`)
	marketplace := repository.NewAgentMarketplaceRepository(db)
	submission, err := marketplace.CreateSubmission(ctx,
		&types.AgentMarketplaceListingEntity{TenantID: tenantID, SourceAgentID: agentID, DisplayName: "Scope test", State: "listed"},
		&types.AgentReleaseSubmissionEntity{TenantID: tenantID, AgentVersionID: versionID, SourceAgentID: agentID, AuthorID: "author", SemanticVersion: "1.0.0", BundleDigest: digest, ManifestJSON: manifest, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted"})
	require.NoError(t, err)
	_, release, err := marketplace.ReviewAndPublishTx(ctx, tenantID, "", submission.ID, digest, types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, release)
	return submission.ListingID, release.ID
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
