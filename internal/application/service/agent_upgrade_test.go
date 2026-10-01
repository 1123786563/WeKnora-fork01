package service

// Agent-upgrade service tests (T31 #61): reconcile-on-read materialization
// over the REAL repository (real migration-stream DB via
// openAgentVersionServiceTestDB), acceptance/dismissal state machine, and
// the #60 introduced-ledger fallback. Lower-interface evidence below the
// Task 6 HTTP e2e — labeled as such, not passed off as AC3.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	upgradeManifestV1 = `{"semantic_version":"1.0.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["model","knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"version-1.0.0","version_number":1,"source_sha256":"sha"}}`
	upgradeManifestV2 = `{"semantic_version":"1.1.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["model","knowledge","sandbox"],"data_categories":["chat_content"],"external_side_effects":["web_search"],"minimum_weknora_capability":"1","license_id":"Apache-2.0","source":{"agent_version_id":"version-1.1.0","version_number":2,"source_sha256":"sha"}}`
	upgradeManifestV3 = `{"semantic_version":"1.2.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["model","knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"version-1.2.0","version_number":3,"source_sha256":"sha"}}`
	upgradeLockV1     = `{"dependencies":[]}`
	upgradeLockV2     = `{"dependencies":[{"type":"skill","id":"weather","version":"1.0.0","digest":"aa","license_id":"MIT"}]}`
	upgradeBundleV1   = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be useful.","allowed_tools":["search"]},"manifest":` + upgradeManifestV1 + `,"dependency_lock":` + upgradeLockV1 + `}`
	upgradeBundleV2   = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be extra useful.","allowed_tools":["search","mail"],"skills":["weather"]},"manifest":` + upgradeManifestV2 + `,"dependency_lock":` + upgradeLockV2 + `}`
	upgradeBundleV3   = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be useful v3."},"manifest":` + upgradeManifestV3 + `,"dependency_lock":{"dependencies":[]}}`
)

func newAgentUpgradeServiceForTest(t *testing.T) (*AgentUpgradeService, *gorm.DB) {
	t.Helper()
	db := openAgentVersionServiceTestDB(t)
	return NewAgentUpgradeService(repository.NewAgentUpgradeRepository(db)), db
}

type gatedUpgradeCreateVariantRepository struct {
	repository.AgentUpgradeRepository
	reached chan struct{}
	resume  <-chan struct{}
}

type gatedUpgradeTransitionRepository struct {
	repository.AgentUpgradeRepository
	reached chan struct{}
	resume  <-chan struct{}
}

func (r *gatedUpgradeTransitionRepository) TransitionProposal(ctx context.Context, tenantID uint64, proposalID string, expectedFrom []string, nextState string, updates map[string]any) (*types.AgentUpgradeProposalEntity, error) {
	close(r.reached)
	<-r.resume
	return r.AgentUpgradeRepository.TransitionProposal(ctx, tenantID, proposalID, expectedFrom, nextState, updates)
}

func (r *gatedUpgradeCreateVariantRepository) CreateVariant(ctx context.Context, variant *types.AgentAdoptionVariantEntity) (*types.AgentAdoptionVariantEntity, error) {
	close(r.reached)
	<-r.resume
	return r.AgentUpgradeRepository.CreateVariant(ctx, variant)
}

// publishUpgradeServiceRelease publishes ONE release on the agent-a listing
// through the real repository path (CreateSubmission + ReviewAndPublishTx —
// the same shape seedAdoptionServiceRelease uses). The listing is
// find-or-created by SourceAgentID; expectedPriorReleaseID is read from the
// listing row, mirroring the service's ReviewSubmission. Returns listing id
// and release id.
func publishUpgradeServiceRelease(t *testing.T, db *gorm.DB, versionNumber int, semanticVersion, manifest, lock, bundle string) (listingID, releaseID string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, db.Exec(
		`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES (?, 1, 'agent-a', ?, '{}', 'sha', 'author')`,
		"version-"+semanticVersion, versionNumber).Error)
	raw := []byte(bundle)
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])

	priorRelease := ""
	var prior types.AgentMarketplaceListingEntity
	err := db.Where("tenant_id = ? AND source_agent_id = ?", uint64(1), "agent-a").First(&prior).Error
	if err == nil && prior.CurrentReleaseID != nil {
		priorRelease = *prior.CurrentReleaseID
	} else if err != nil {
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	}

	repo := repository.NewAgentMarketplaceRepository(db)
	sub, err := repo.CreateSubmission(ctx,
		&types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-a", DisplayName: "Helper", Summary: "s", State: "listed"},
		&types.AgentReleaseSubmissionEntity{
			TenantID: 1, AgentVersionID: "version-" + semanticVersion, SourceAgentID: "agent-a", AuthorID: "author",
			SemanticVersion: semanticVersion, BundleDigest: digest, ManifestJSON: manifest,
			DependencyLockJSON: lock, Bundle: raw, Status: "submitted",
		})
	require.NoError(t, err)
	_, rel, err := repo.ReviewAndPublishTx(ctx, 1, priorRelease, sub.ID, digest, types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, rel)
	return sub.ListingID, rel.ID
}

func adoptUpgradeRelease(t *testing.T, db *gorm.DB, listingID, releaseID string) *types.AgentAdoptionEntity {
	t.Helper()
	repo := repository.NewAgentAdoptionRepository(db)
	adoption, _, err := repo.AdoptListing(context.Background(), &types.AgentAdoptionEntity{
		TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin",
	})
	require.NoError(t, err)
	return adoption
}

func TestAgentUpgradeServiceMaterializesFourDimensionDiff(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoptUpgradeRelease(t, db, listingID, v1)
	ctx := context.Background()

	// Listing 指针仍在 v1、Adoption 已接受 v1：无升级建议。
	before, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, before)

	// v2 发布（ReviewAndPublishTx 前移指针）：读取时对账物化建议。
	_, v2 := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	proposals, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	p := proposals[0]
	require.Equal(t, listingID, p.ListingID)
	require.Equal(t, v1, p.FromReleaseID)
	require.Equal(t, v2, p.ToReleaseID)
	require.Equal(t, "1.1.0", p.ToSemanticVersion)
	require.Equal(t, AgentUpgradeProposalStateOpen, p.State)

	// 四维差异（AC2）：
	require.Equal(t, []types.UpgradeFieldChange{
		{Field: "system_prompt", From: "Be useful.", To: "Be extra useful."},
		{Field: "allowed_tools", From: "search", To: "mail,search"},
		{Field: "skills", From: "", To: "weather"},
	}, p.Diff.Behavior)
	require.Equal(t, []types.UpgradeDependencyChange{
		{Type: "skill", ID: "weather", Change: "added", ToVersion: "1.0.0"},
	}, p.Diff.Dependencies)
	require.Equal(t, []types.UpgradeFieldChange{
		{Field: "capability_requirements", From: "knowledge,model", To: "knowledge,model,sandbox"},
		{Field: "data_categories", From: "", To: "chat_content"},
		{Field: "external_side_effects", From: "", To: "web_search"},
	}, p.Diff.Security)
	require.Equal(t, []types.UpgradeLicenseChange{
		{Scope: "dependency", ID: "weather", From: "", To: "MIT"},
		{Scope: "release", ID: "license_id", From: "MIT", To: "Apache-2.0"},
	}, p.Diff.License)

	// 对账幂等：再列一次仍恰好一条（唯一索引收敛，无重复行）。
	again, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, again, 1)
	require.Equal(t, p.ID, again[0].ID)

	// 单条读取与列表一致。
	single, err := svc.GetUpgradeProposal(ctx, 1, p.ID)
	require.NoError(t, err)
	require.Equal(t, p.ID, single.ID)
	require.Equal(t, p.Diff, single.Diff)
}

func TestAgentUpgradeServiceResolvesAndRefusesOutOfStateOperations(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoption := adoptUpgradeRelease(t, db, listingID, v1)
	_, v2 := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	ctx := context.Background()

	proposals, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	proposalID := proposals[0].ID

	// 非法输入：空名字拒绝。
	_, _, err = svc.AcceptUpgradeProposal(ctx, 1, "admin", proposalID, interfaces.UpgradeVariantInput{Name: "  "})
	require.ErrorIs(t, err, ErrAgentUpgradeInvalidInput)
	// 未知 proposal：not found。
	_, _, err = svc.AcceptUpgradeProposal(ctx, 1, "admin", "missing-proposal", interfaces.UpgradeVariantInput{Name: "X"})
	require.ErrorIs(t, err, ErrAgentUpgradeNotFound)
	_, err = svc.DismissUpgradeProposal(ctx, 1, "admin", "missing-proposal")
	require.ErrorIs(t, err, ErrAgentUpgradeNotFound)

	// 接受：只创建固定到 to_release 的草稿 Variant，proposal 落 accepted 终态。
	variant, accepted, err := svc.AcceptUpgradeProposal(ctx, 1, "admin", proposalID, interfaces.UpgradeVariantInput{Name: "Sales v1.1"})
	require.NoError(t, err)
	require.Equal(t, AgentVariantStateDraft, variant.State)
	require.Equal(t, v2, variant.ReleaseID)
	require.Equal(t, adoption.ID, variant.AdoptionID)
	require.Equal(t, AgentUpgradeProposalStateAccepted, accepted.State)
	require.Equal(t, variant.ID, accepted.AcceptedVariantID)
	require.Equal(t, "admin", accepted.ResolvedBy)
	// 新 Release 要求三项能力而新草稿尚无映射：missing 逐项点名（复用 #59 裁决）。
	require.Equal(t, []string{"knowledge", "model", "sandbox"}, variant.MissingCapabilities)

	// 终态互斥：重复 accept / dismiss 已 accepted，一律显式冲突。
	_, _, err = svc.AcceptUpgradeProposal(ctx, 1, "admin", proposalID, interfaces.UpgradeVariantInput{Name: "Again"})
	require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)
	_, err = svc.DismissUpgradeProposal(ctx, 1, "admin", proposalID)
	require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)

	// 已 resolved 的建议不复活：再列仍恰好一条，state=accepted。
	after, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, after, 1)
	require.Equal(t, AgentUpgradeProposalStateAccepted, after[0].State)

	// dismiss 路径：新 Release v3 → 新 open 建议 → dismiss → 终态保留。
	_, v3 := publishUpgradeServiceRelease(t, db, 3, "1.2.0", upgradeManifestV3, `{"dependencies":[]}`, upgradeBundleV3)
	reopened, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, reopened, 2)
	var openID string
	for _, row := range reopened {
		if row.State == AgentUpgradeProposalStateOpen {
			openID = row.ID
			require.Equal(t, v3, row.ToReleaseID)
		}
	}
	require.NotEmpty(t, openID)
	dismissed, err := svc.DismissUpgradeProposal(ctx, 1, "admin", openID)
	require.NoError(t, err)
	require.Equal(t, AgentUpgradeProposalStateDismissed, dismissed.State)
	require.Equal(t, "admin", dismissed.ResolvedBy)
	still, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, still, 2, "dismissed 行保留且不复活")
	for _, row := range still {
		require.NotEqual(t, AgentUpgradeProposalStateOpen, row.State)
	}
	_, err = svc.DismissUpgradeProposal(ctx, 1, "admin", openID)
	require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)
}

func TestAcceptUpgradeProposalMapsConcurrentAdoptionEndToConflict(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, fromReleaseID := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoption := adoptUpgradeRelease(t, db, listingID, fromReleaseID)
	_, toReleaseID := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	proposals, err := svc.ListUpgradeProposals(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)

	baseRepo := svc.repo
	resume := make(chan struct{})
	var resumeOnce sync.Once
	defer resumeOnce.Do(func() { close(resume) })
	gate := &gatedUpgradeCreateVariantRepository{
		AgentUpgradeRepository: baseRepo,
		reached:                make(chan struct{}),
		resume:                 resume,
	}
	svc.repo = gate
	type result struct {
		err error
	}
	accepted := make(chan result, 1)
	go func() {
		_, _, err := svc.AcceptUpgradeProposal(context.Background(), 1, "admin", proposals[0].ID, interfaces.UpgradeVariantInput{Name: "Upgrade draft"})
		accepted <- result{err: err}
	}()
	select {
	case <-gate.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptUpgradeProposal did not reach CreateVariant after its state prechecks")
	}

	_, err = repository.NewAgentAdoptionRepository(db).EndAdoption(context.Background(), 1, adoption.ID, "active", "ended", map[string]any{"ended_by": "admin"})
	require.NoError(t, err)
	resumeOnce.Do(func() { close(resume) })
	select {
	case got := <-accepted:
		require.ErrorIs(t, got.err, ErrAgentUpgradeStateConflict)
		require.ErrorIs(t, got.err, repository.ErrAgentAdoptionTransition)
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptUpgradeProposal did not return after resuming CreateVariant")
	}

	variants, err := baseRepo.ListVariantsByAdoption(context.Background(), 1, adoption.ID)
	require.NoError(t, err)
	require.Empty(t, variants, "rejected upgrade acceptance must not create a Variant after Adoption ended")
	proposal, err := baseRepo.GetProposal(context.Background(), 1, proposals[0].ID)
	require.NoError(t, err)
	require.NotNil(t, proposal)
	require.Equal(t, AgentUpgradeProposalStateOpen, proposal.State)
	require.NotEmpty(t, toReleaseID)
}

func TestAcceptUpgradeProposalRejectsConcurrentUnlist(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, fromReleaseID := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoption := adoptUpgradeRelease(t, db, listingID, fromReleaseID)
	_, _ = publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	proposals, err := svc.ListUpgradeProposals(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)

	baseRepo := svc.repo
	resume := make(chan struct{})
	var resumeOnce sync.Once
	defer resumeOnce.Do(func() { close(resume) })
	gate := &gatedUpgradeCreateVariantRepository{AgentUpgradeRepository: baseRepo, reached: make(chan struct{}), resume: resume}
	svc.repo = gate
	accepted := make(chan error, 1)
	go func() {
		_, _, err := svc.AcceptUpgradeProposal(context.Background(), 1, "admin", proposals[0].ID, interfaces.UpgradeVariantInput{Name: "late upgrade"})
		accepted <- err
	}()
	select {
	case <-gate.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptUpgradeProposal did not reach the repository after its eligibility prechecks")
	}
	_, err = repository.NewAgentMarketplaceRepository(db).TransitionListingState(context.Background(), 1, listingID, "listed", "unlisted", map[string]any{"unlisted_by": "admin"})
	require.NoError(t, err)
	resumeOnce.Do(func() { close(resume) })
	select {
	case err := <-accepted:
		require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptUpgradeProposal did not return after unlisting")
	}
	variants, err := baseRepo.ListVariantsByAdoption(context.Background(), 1, adoption.ID)
	require.NoError(t, err)
	require.Empty(t, variants, "acceptance must not create a Variant after its Listing is unlisted")
}

func TestAcceptUpgradeProposalRechecksLifecycleAfterVariantCreation(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, fromReleaseID := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoption := adoptUpgradeRelease(t, db, listingID, fromReleaseID)
	_, _ = publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	proposals, err := svc.ListUpgradeProposals(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)

	baseRepo := svc.repo
	resume := make(chan struct{})
	var resumeOnce sync.Once
	defer resumeOnce.Do(func() { close(resume) })
	gate := &gatedUpgradeTransitionRepository{AgentUpgradeRepository: baseRepo, reached: make(chan struct{}), resume: resume}
	svc.repo = gate
	accepted := make(chan error, 1)
	go func() {
		_, _, err := svc.AcceptUpgradeProposal(context.Background(), 1, "admin", proposals[0].ID, interfaces.UpgradeVariantInput{Name: "orphan after unlist"})
		accepted <- err
	}()
	select {
	case <-gate.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptUpgradeProposal did not reach TransitionProposal after creating its Variant")
	}
	_, err = repository.NewAgentMarketplaceRepository(db).TransitionListingState(context.Background(), 1, listingID, "listed", "unlisted", map[string]any{"unlisted_by": "admin"})
	require.NoError(t, err)
	resumeOnce.Do(func() { close(resume) })
	select {
	case err := <-accepted:
		require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)
		require.ErrorIs(t, err, repository.ErrAgentUpgradeProposalTransition)
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptUpgradeProposal did not return after its transition guard resumed")
	}
	proposal, err := baseRepo.GetProposal(context.Background(), 1, proposals[0].ID)
	require.NoError(t, err)
	require.NotNil(t, proposal)
	require.Equal(t, AgentUpgradeProposalStateOpen, proposal.State)
	require.Empty(t, proposal.AcceptedVariantID)
	variants, err := baseRepo.ListVariantsByAdoption(context.Background(), 1, adoption.ID)
	require.NoError(t, err)
	require.Len(t, variants, 1, "the rejected acceptance may leave the explicitly allowed orphan draft")
	require.Equal(t, AgentVariantStateDraft, variants[0].State)
}

func TestAcceptUpgradeProposalRechecksAdoptionEndAfterVariantCreation(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, fromReleaseID := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoption := adoptUpgradeRelease(t, db, listingID, fromReleaseID)
	_, _ = publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	proposals, err := svc.ListUpgradeProposals(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)

	baseRepo := svc.repo
	resume := make(chan struct{})
	var resumeOnce sync.Once
	defer resumeOnce.Do(func() { close(resume) })
	gate := &gatedUpgradeTransitionRepository{AgentUpgradeRepository: baseRepo, reached: make(chan struct{}), resume: resume}
	svc.repo = gate
	accepted := make(chan error, 1)
	go func() {
		_, _, err := svc.AcceptUpgradeProposal(context.Background(), 1, "admin", proposals[0].ID, interfaces.UpgradeVariantInput{Name: "retired orphan"})
		accepted <- err
	}()
	select {
	case <-gate.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptUpgradeProposal did not reach TransitionProposal after draft creation")
	}
	var draft types.AgentAdoptionVariantEntity
	require.NoError(t, db.Where("tenant_id = ? AND adoption_id = ?", 1, adoption.ID).First(&draft).Error)
	require.NoError(t, db.Model(&types.AgentAdoptionVariantEntity{}).Where("tenant_id = ? AND id = ?", 1, draft.ID).Update("state", "retired").Error)
	_, err = repository.NewAgentAdoptionRepository(db).EndAdoption(context.Background(), 1, adoption.ID, "active", "ended", map[string]any{"ended_by": "admin"})
	require.NoError(t, err)
	resumeOnce.Do(func() { close(resume) })
	select {
	case err := <-accepted:
		require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)
		require.ErrorIs(t, err, repository.ErrAgentUpgradeProposalTransition)
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptUpgradeProposal did not return after ending Adoption")
	}
	proposal, err := baseRepo.GetProposal(context.Background(), 1, proposals[0].ID)
	require.NoError(t, err)
	require.Equal(t, AgentUpgradeProposalStateOpen, proposal.State)
	require.Empty(t, proposal.AcceptedVariantID)
}

func TestAcceptUpgradeProposalRechecksAcceptedReleaseAfterVariantCreation(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, fromReleaseID := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoption := adoptUpgradeRelease(t, db, listingID, fromReleaseID)
	_, targetReleaseID := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	proposals, err := svc.ListUpgradeProposals(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	_, advancedTo := publishUpgradeServiceRelease(t, db, 3, "1.2.0", upgradeManifestV3, upgradeLockV1, upgradeBundleV3)

	baseRepo := svc.repo
	resume := make(chan struct{})
	var resumeOnce sync.Once
	defer resumeOnce.Do(func() { close(resume) })
	gate := &gatedUpgradeTransitionRepository{AgentUpgradeRepository: baseRepo, reached: make(chan struct{}), resume: resume}
	svc.repo = gate
	accepted := make(chan error, 1)
	go func() {
		_, _, err := svc.AcceptUpgradeProposal(context.Background(), 1, "admin", proposals[0].ID, interfaces.UpgradeVariantInput{Name: "stale source"})
		accepted <- err
	}()
	select {
	case <-gate.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptUpgradeProposal did not reach TransitionProposal after draft creation")
	}
	_, _, err = repository.NewAgentAdoptionRepository(db).AdoptListing(context.Background(), &types.AgentAdoptionEntity{
		TenantID: 1, ListingID: listingID, AcceptedReleaseID: advancedTo, State: "active", CreatedBy: "admin",
	})
	require.NoError(t, err)
	resumeOnce.Do(func() { close(resume) })
	select {
	case err := <-accepted:
		require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)
		require.ErrorIs(t, err, repository.ErrAgentUpgradeProposalTransition)
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptUpgradeProposal did not return after advancing the accepted Release")
	}
	proposal, err := baseRepo.GetProposal(context.Background(), 1, proposals[0].ID)
	require.NoError(t, err)
	require.Equal(t, AgentUpgradeProposalStateOpen, proposal.State)
	require.Empty(t, proposal.AcceptedVariantID)
	var current types.AgentAdoptionEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, adoption.ID).First(&current).Error)
	require.Equal(t, advancedTo, current.AcceptedReleaseID)
	require.NotEqual(t, targetReleaseID, advancedTo)
}

func TestAgentUpgradeServiceCoversIntroducedLedgerUpgrades(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	ctx := context.Background()

	// #60 引入台账：同一 public listing 的两个引入 Release（旧→新），
	// Adoption 停留在旧引入版；listing 读取经 introducedListing 合成、
	// 两侧 Release 读取经 introducedRelease 合成。
	// 真实迁移流上 tenant_introduced_releases 挂 public 父表外键（000114：
	// public_listing_id → public_marketplace_listings；public_release_id →
	// public_agent_releases，后者再挂 submissions；submissions 再挂
	// agent_releases(publisher_tenant, source_release)）。tenant 侧两份
	// Release 走真实发布流（publishUpgradeServiceRelease）取得真实
	// agent_releases 行；public 侧父行是 FK 陪衬脚手架——服务回退只读
	// tenant_introduced_releases，父行内容不参与断言。
	_, localRel1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	_, localRel2 := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	base := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&types.PublicMarketplaceListingEntity{
		ID: "pub-listing-1", PublisherTenantID: 9, SourceListingID: "src-listing-1", DisplayName: "Helper", State: "listed",
	}).Error)
	for i, ver := range []struct{ sub, rel, sem, digest, src string }{
		{"pub-sub-1", "public-rel-1", "1.0.0", "d1", localRel1},
		{"pub-sub-2", "public-rel-2", "1.1.0", "d2", localRel2},
	} {
		require.NoError(t, db.Create(&types.PublicReleaseSubmissionEntity{
			ID: ver.sub, PublisherTenantID: 1, PublicListingID: "pub-listing-1", SourceListingID: "src-listing-1",
			SourceReleaseID: ver.src, SemanticVersion: ver.sem, BundleDigest: ver.digest,
			ManifestJSON: upgradeManifestV1, DependencyLockJSON: upgradeLockV1, Bundle: []byte(upgradeBundleV1),
		}).Error)
		require.NoError(t, db.Create(&types.PublicAgentReleaseEntity{
			ID: ver.rel, ListingID: "pub-listing-1", SubmissionID: ver.sub, PublisherTenantID: 9,
			ReleaseNumber: i + 1, SemanticVersion: ver.sem, BundleDigest: ver.digest,
			ManifestJSON: upgradeManifestV1, DependencyLockJSON: upgradeLockV1, Bundle: []byte(upgradeBundleV1),
		}).Error)
	}
	require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{
		ID: "intro-rel-1", TenantID: 1, PublicListingID: "pub-listing-1", PublicReleaseID: "public-rel-1",
		DisplayName: "Helper", Summary: "s", SemanticVersion: "1.0.0",
		BundleDigest: "d1", ManifestJSON: upgradeManifestV1, DependencyLockJSON: upgradeLockV1,
		Bundle: []byte(upgradeBundleV1), IntroducedBy: "admin", IntroducedAt: base,
	}).Error)
	require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{
		ID: "intro-rel-2", TenantID: 1, PublicListingID: "pub-listing-1", PublicReleaseID: "public-rel-2",
		DisplayName: "Helper", Summary: "s", SemanticVersion: "2.0.0",
		BundleDigest: "d2", ManifestJSON: upgradeManifestV2, DependencyLockJSON: upgradeLockV2,
		Bundle: []byte(upgradeBundleV2), IntroducedBy: "admin", IntroducedAt: base.Add(time.Hour),
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		TenantID: 1, ID: "adopt-intro", ListingID: "pub-listing-1",
		AcceptedReleaseID: "intro-rel-1", State: "active", CreatedBy: "admin",
	}).Error)

	proposals, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	p := proposals[0]
	require.Equal(t, "pub-listing-1", p.ListingID)
	require.Equal(t, "intro-rel-1", p.FromReleaseID)
	require.Equal(t, "intro-rel-2", p.ToReleaseID, "listing 当前指针 = 最新引入行")
	require.Equal(t, "2.0.0", p.ToSemanticVersion)
	require.Len(t, p.Diff.Behavior, 3, "引入链与本地链共用同一差异计算")
}

// TestAgentUpgradeServiceAcceptRaceLoserLeavesBenignOrphanDraft pins the
// documented accept race window (final review finding: CreateVariant runs
// before the CAS TransitionProposal, so a concurrent double-accept loser's
// draft persists as an "orphan"). The design is deliberate — this test
// proves the orphan stays BENIGN instead of pretending the window cannot
// happen: state=draft pinned to to_release, absent from the available-agents
// surface, unreferenced by the proposal's accepted_variant_id, and the
// proposal keeps its accepted terminal state (no resurrection). If a future
// requirement demands orphan reclamation, that belongs in a repository-level
// transactional primitive, not a silent service-side delete.
func TestAgentUpgradeServiceAcceptRaceLoserLeavesBenignOrphanDraft(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	repo := repository.NewAgentUpgradeRepository(db)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoption := adoptUpgradeRelease(t, db, listingID, v1)
	_, v2 := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	ctx := context.Background()

	proposals, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	proposalID := proposals[0].ID

	// 竞态败者：其 CreateVariant 先于双方 CAS 落库（与生产调用序列同形）。
	loserDraft, err := repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{
		TenantID: uint64(1), AdoptionID: adoption.ID, ReleaseID: v2,
		Name: "Race loser draft", State: AgentVariantStateDraft, CreatedBy: "admin-b",
	})
	require.NoError(t, err)

	// CAS 赢家：服务 accept 正常收敛 accepted 终态，accepted_variant_id
	// 指向赢家自己的草稿。
	winnerVariant, accepted, err := svc.AcceptUpgradeProposal(ctx, 1, "admin-a", proposalID, interfaces.UpgradeVariantInput{Name: "Sales v1.1"})
	require.NoError(t, err)
	require.Equal(t, AgentUpgradeProposalStateAccepted, accepted.State)
	require.Equal(t, winnerVariant.ID, accepted.AcceptedVariantID)

	// 败者的 CAS 重放（accept）收到显式冲突——冲突但不重复终态落库。
	_, _, err = svc.AcceptUpgradeProposal(ctx, 1, "admin-b", proposalID, interfaces.UpgradeVariantInput{Name: "Sales v1.1 too"})
	require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)

	// 孤儿草稿良性不变量 1：state 停留 draft、固定 to_release，
	// 且不被终态行的 accepted_variant_id 引用。
	var loserRow types.AgentAdoptionVariantEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), loserDraft.ID).First(&loserRow).Error)
	require.Equal(t, AgentVariantStateDraft, loserRow.State, "孤儿草稿不自行离开 draft")
	require.Equal(t, v2, loserRow.ReleaseID, "孤儿草稿固定在 to_release")
	require.NotEqual(t, accepted.AcceptedVariantID, loserRow.ID)

	// 孤儿草稿良性不变量 2：不进入 available-agents 面（published 专属）。
	available, err := repository.NewAgentAdoptionRepository(db).PublishedAvailableAgents(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, available, "draft（含孤儿）不进入 available-agents 面")

	// 孤儿草稿良性不变量 3：建议保持 accepted 终态，不因孤儿复活。
	after, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, after, 1)
	require.Equal(t, AgentUpgradeProposalStateAccepted, after[0].State)
	require.Equal(t, winnerVariant.ID, after[0].AcceptedVariantID)
}

func TestAgentUpgradeServiceSkipsInactiveAdoptionsAndCorruptReleases(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoption := adoptUpgradeRelease(t, db, listingID, v1)
	_, v2 := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	ctx := context.Background()

	// 非 active Adoption（End Adoption 归 #63，此处只钉状态过滤）。
	require.NoError(t, db.Model(&types.AgentAdoptionEntity{}).
		Where("tenant_id = ? AND id = ?", uint64(1), adoption.ID).Update("state", "ended").Error)
	proposals, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, proposals, "非 active Adoption 不生成升级建议")

	// 恢复 active：ended 过滤下的空列表对后续断言是空真，恢复后损坏/修复
	// 两段才能真正到达 release 解码路径（fail-closed 跳过与恢复后物化）。
	require.NoError(t, db.Model(&types.AgentAdoptionEntity{}).
		Where("tenant_id = ? AND id = ?", uint64(1), adoption.ID).Update("state", "active").Error)

	// 损坏的 to-Release bundle：fail closed——跳过物化并记日志，列表端点不炸。
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).
		Where("tenant_id = ? AND id = ?", uint64(1), v2).Update("bundle", []byte(`{broken`)).Error)
	corrupt, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, corrupt)

	// 修复后（模拟恢复）建议照常出现。
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).
		Where("tenant_id = ? AND id = ?", uint64(1), v2).Update("bundle", []byte(upgradeBundleV2)).Error)
	healed, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, healed, 1)
}

// countingUpgradeRepo wraps the REAL repository and counts the expensive
// reads reconcile performs (per pair: one listing read + two release reads
// + a full two-bundle decode for the diff). Sequential calls only — no lock.
type countingUpgradeRepo struct {
	repository.AgentUpgradeRepository
	listingReads int
	releaseReads int
}

func (c *countingUpgradeRepo) GetMarketplaceListing(ctx context.Context, tenantID uint64, listingID string) (*types.AgentMarketplaceListingEntity, error) {
	c.listingReads++
	return c.AgentUpgradeRepository.GetMarketplaceListing(ctx, tenantID, listingID)
}

func (c *countingUpgradeRepo) GetRelease(ctx context.Context, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error) {
	c.releaseReads++
	return c.AgentUpgradeRepository.GetRelease(ctx, tenantID, releaseID)
}

// TestReconcileSkipsMaterializedPairsWithoutDiff pins R5-F1: a materialized
// (adoption, to_release) pair — in ANY state — must not recompute the diff
// on every read request. The listing row read remains (the current release
// pointer is only knowable from it), but both release reads and the full
// bundle decode are skipped.
func TestReconcileSkipsMaterializedPairsWithoutDiff(t *testing.T) {
	db := openAgentVersionServiceTestDB(t)
	counting := &countingUpgradeRepo{AgentUpgradeRepository: repository.NewAgentUpgradeRepository(db)}
	svc := NewAgentUpgradeService(counting)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoptUpgradeRelease(t, db, listingID, v1)
	publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	ctx := context.Background()

	first, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Greater(t, counting.releaseReads, 0, "setup: the first materialization performs the expensive reads")

	counting.listingReads, counting.releaseReads = 0, 0
	second, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, second, 1, "the materialized row still lists — the skip only saves the recompute")
	require.Equal(t, first[0].ID, second[0].ID)
	require.Zero(t, counting.releaseReads, "a materialized pair must short-circuit before BOTH release reads and the bundle diff")
}

// TestReconcileDismissesStaleOpenProposalsAfterReAdopt pins R5-F2 upper
// half: re-adopting advances AcceptedReleaseID, and the open proposal whose
// from_release is no longer the accepted pointer must migrate to a terminal
// dismissed state (system marker) instead of lingering forever as open.
func TestReconcileDismissesStaleOpenProposalsAfterReAdopt(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	repo := repository.NewAgentUpgradeRepository(db)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoptUpgradeRelease(t, db, listingID, v1)
	_, v2 := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	ctx := context.Background()

	first, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	staleID := first[0].ID // open, from=v1, to=v2

	// Re-adopt advances the accepted pointer to v2; v3 ships afterwards.
	_, _, err = repo.AdoptListing(ctx, &types.AgentAdoptionEntity{
		TenantID: 1, ListingID: listingID, AcceptedReleaseID: v2, State: "active", CreatedBy: "admin",
	})
	require.NoError(t, err)
	_, v3 := publishUpgradeServiceRelease(t, db, 3, "1.2.0", upgradeManifestV3, `{"dependencies":[]}`, upgradeBundleV3)

	second, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, second, 2)
	for _, row := range second {
		switch row.ToReleaseID {
		case v2:
			require.Equal(t, AgentUpgradeProposalStateDismissed, row.State,
				"the stale open (from_release no longer the accepted pointer) must migrate to a terminal state")
			require.Equal(t, AgentUpgradeSystemResolvedBy, row.ResolvedBy,
				"the system dismissal must be distinguishable from a human dismiss")
		case v3:
			require.Equal(t, AgentUpgradeProposalStateOpen, row.State)
		default:
			t.Fatalf("unexpected proposal: %+v", row)
		}
	}
	_ = staleID
	// 不再以 open 呈现：对该行的 Accept 被终态拒绝（state guard），无孤儿草稿。
	_, _, err = svc.AcceptUpgradeProposal(ctx, 1, "admin", staleID, interfaces.UpgradeVariantInput{Name: "late"})
	require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)
}

// TestAcceptRejectsProposalFromStaleReleasePointer pins R5-F2 lower half:
// an open proposal whose from_release is no longer the adoption's accepted
// pointer must be refused at Accept time — accepting it would create a
// redundant draft Variant pinned to an already-superseded release.
func TestAcceptRejectsProposalFromStaleReleasePointer(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	repo := repository.NewAgentUpgradeRepository(db)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoptUpgradeRelease(t, db, listingID, v1)
	_, v2 := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	ctx := context.Background()

	first, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	stale := first[0] // open, from=v1, to=v2

	// Re-adopt advances past v1 WITHOUT a reconcile run in between (Accept
	// itself never reconciles — the guard must live in Accept).
	_, _, err = repo.AdoptListing(ctx, &types.AgentAdoptionEntity{
		TenantID: 1, ListingID: listingID, AcceptedReleaseID: v2, State: "active", CreatedBy: "admin",
	})
	require.NoError(t, err)

	_, _, err = svc.AcceptUpgradeProposal(ctx, 1, "admin", stale.ID, interfaces.UpgradeVariantInput{Name: "stale draft"})
	require.ErrorIs(t, err, ErrAgentUpgradeStateConflict,
		"accepting a proposal whose from release is no longer the accepted pointer must be a state conflict")

	var variants int64
	require.NoError(t, db.Model(&types.AgentAdoptionVariantEntity{}).
		Where("tenant_id = ? AND adoption_id = ?", uint64(1), stale.AdoptionID).Count(&variants).Error)
	require.Zero(t, variants, "no draft variant may be created off a superseded release pointer")
}
