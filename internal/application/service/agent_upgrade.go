package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrAgentUpgradeInvalidInput  = errors.New("invalid agent upgrade proposal request")
	ErrAgentUpgradeNotFound      = errors.New("agent upgrade proposal not found")
	ErrAgentUpgradeStateConflict = errors.New("agent upgrade proposal state conflict")
)

// Proposal lifecycle states (CONTEXT.md「Agent 升级建议」). accepted 和
// dismissed 都是终态：materialize 对已存在行幂等，resolved 建议不会被
// reconcile 复活（End Adoption 的「禁止新升级建议」由 #63 在 adoption 状态
// 上落闸，本服务的对账只作用于 active Adoption）。
const (
	AgentUpgradeProposalStateOpen      = "open"
	AgentUpgradeProposalStateAccepted  = "accepted"
	AgentUpgradeProposalStateDismissed = "dismissed"
)

type AgentUpgradeService struct {
	repo repository.AgentUpgradeRepository
	now  func() time.Time
}

var _ interfaces.AgentUpgradeService = (*AgentUpgradeService)(nil)

func NewAgentUpgradeService(repo repository.AgentUpgradeRepository) *AgentUpgradeService {
	return &AgentUpgradeService{repo: repo, now: time.Now}
}

func (s *AgentUpgradeService) ListUpgradeProposals(ctx context.Context, tenantID uint64) ([]interfaces.UpgradeProposalView, error) {
	if tenantID == 0 {
		return nil, ErrAgentUpgradeInvalidInput
	}
	if err := s.reconcileProposals(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListProposals(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.UpgradeProposalView, 0, len(rows))
	for i := range rows {
		view, err := decodeUpgradeProposal(&rows[i])
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *AgentUpgradeService) GetUpgradeProposal(ctx context.Context, tenantID uint64, proposalID string) (interfaces.UpgradeProposalView, error) {
	if tenantID == 0 || strings.TrimSpace(proposalID) == "" {
		return interfaces.UpgradeProposalView{}, ErrAgentUpgradeInvalidInput
	}
	if err := s.reconcileProposals(ctx, tenantID); err != nil {
		return interfaces.UpgradeProposalView{}, err
	}
	row, err := s.repo.GetProposal(ctx, tenantID, strings.TrimSpace(proposalID))
	if err != nil {
		return interfaces.UpgradeProposalView{}, err
	}
	if row == nil {
		return interfaces.UpgradeProposalView{}, ErrAgentUpgradeNotFound
	}
	return decodeUpgradeProposal(row)
}

func (s *AgentUpgradeService) AcceptUpgradeProposal(ctx context.Context, tenantID uint64, actorID, proposalID string, input interfaces.UpgradeVariantInput) (interfaces.AdoptionVariantView, interfaces.UpgradeProposalView, error) {
	proposalID, input.Name, actorID = strings.TrimSpace(proposalID), strings.TrimSpace(input.Name), strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || proposalID == "" || input.Name == "" {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, ErrAgentUpgradeInvalidInput
	}
	row, err := s.repo.GetProposal(ctx, tenantID, proposalID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	if row == nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, ErrAgentUpgradeNotFound
	}
	if row.State != AgentUpgradeProposalStateOpen {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{},
			fmt.Errorf("%w: proposal is %q, not %q", ErrAgentUpgradeStateConflict, row.State, AgentUpgradeProposalStateOpen)
	}
	adoption, err := s.repo.GetAdoption(ctx, tenantID, row.AdoptionID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	if adoption == nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, ErrAgentUpgradeNotFound
	}
	if adoption.State != AgentAdoptionStateActive {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{},
			fmt.Errorf("%w: adoption state is %q", ErrAgentUpgradeStateConflict, adoption.State)
	}
	toRelease, err := s.repo.GetRelease(ctx, tenantID, row.ToReleaseID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	if toRelease == nil || toRelease.ListingID != row.ListingID {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{},
			fmt.Errorf("%w: proposed release does not belong to the listing", ErrAgentUpgradeInvalidInput)
	}
	// 接受 = 以新 Release 创建一个新的草稿 Variant（spec §9）。随后走 #59
	// 既有 mapping/test/publish 流程；本方法绝不触碰其他 Variant 或既有
	// 本地 Agent Version。
	variant, err := s.repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{
		TenantID: tenantID, AdoptionID: row.AdoptionID, ReleaseID: row.ToReleaseID,
		Name: input.Name, State: AgentVariantStateDraft, CreatedBy: actorID,
	})
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	manifest, err := releaseManifest(ctx, s.repo, tenantID, row.ToReleaseID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	mappings, err := s.repo.ListCapabilityMappings(ctx, tenantID, variant.ID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	updated, err := s.repo.TransitionProposal(ctx, tenantID, row.ID, []string{AgentUpgradeProposalStateOpen}, AgentUpgradeProposalStateAccepted,
		map[string]any{"accepted_variant_id": variant.ID, "resolved_by": actorID})
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	proposal, err := decodeUpgradeProposal(updated)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	return interfaces.AdoptionVariantView{
		AgentAdoptionVariantEntity: *variant,
		MissingCapabilities:        missingCapabilities(manifest.CapabilityRequirements, mappings),
	}, proposal, nil
}

func (s *AgentUpgradeService) DismissUpgradeProposal(ctx context.Context, tenantID uint64, actorID, proposalID string) (interfaces.UpgradeProposalView, error) {
	proposalID, actorID = strings.TrimSpace(proposalID), strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || proposalID == "" {
		return interfaces.UpgradeProposalView{}, ErrAgentUpgradeInvalidInput
	}
	row, err := s.repo.GetProposal(ctx, tenantID, proposalID)
	if err != nil {
		return interfaces.UpgradeProposalView{}, err
	}
	if row == nil {
		return interfaces.UpgradeProposalView{}, ErrAgentUpgradeNotFound
	}
	if row.State != AgentUpgradeProposalStateOpen {
		return interfaces.UpgradeProposalView{},
			fmt.Errorf("%w: proposal is %q, not %q", ErrAgentUpgradeStateConflict, row.State, AgentUpgradeProposalStateOpen)
	}
	updated, err := s.repo.TransitionProposal(ctx, tenantID, row.ID, []string{AgentUpgradeProposalStateOpen}, AgentUpgradeProposalStateDismissed,
		map[string]any{"resolved_by": actorID})
	if err != nil {
		return interfaces.UpgradeProposalView{}, err
	}
	return decodeUpgradeProposal(updated)
}

// reconcileProposals materializes one open proposal per (active adoption,
// newer listing release) pair. Tenant-local and introduced listings both
// resolve through the embedded adoption repository (local rows win, the
// #60 introduction ledger synthesizes the rest). A release that fails to
// decode is skipped with a log line — fail closed, never a half-built
// proposal, never a failing listing read.
func (s *AgentUpgradeService) reconcileProposals(ctx context.Context, tenantID uint64) error {
	adoptions, err := s.repo.ListAdoptions(ctx, tenantID)
	if err != nil {
		return err
	}
	for i := range adoptions {
		adoption := adoptions[i]
		if adoption.State != AgentAdoptionStateActive {
			continue
		}
		listing, err := s.repo.GetMarketplaceListing(ctx, tenantID, adoption.ListingID)
		if err != nil {
			return err
		}
		if listing == nil || listing.CurrentReleaseID == nil {
			continue
		}
		toReleaseID := *listing.CurrentReleaseID
		if toReleaseID == adoption.AcceptedReleaseID {
			continue
		}
		fromRelease, err := s.repo.GetRelease(ctx, tenantID, adoption.AcceptedReleaseID)
		if err != nil {
			return err
		}
		if fromRelease == nil {
			continue
		}
		toRelease, err := s.repo.GetRelease(ctx, tenantID, toReleaseID)
		if err != nil {
			return err
		}
		if toRelease == nil {
			continue
		}
		diff, err := diffUpgradeBundles(fromRelease, toRelease)
		if err != nil {
			logger.WarnWithFields(ctx, logger.Fields{
				"tenant_id": tenantID, "adoption_id": adoption.ID,
				"from_release_id": adoption.AcceptedReleaseID, "to_release_id": toReleaseID,
				"reason": err.Error(),
			}, "agent upgrade: skip proposal materialization, release payload unreadable")
			continue
		}
		raw, err := json.Marshal(diff)
		if err != nil {
			return err
		}
		if _, _, err := s.repo.FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
			TenantID: tenantID, AdoptionID: adoption.ID, ListingID: adoption.ListingID,
			FromReleaseID: adoption.AcceptedReleaseID, ToReleaseID: toReleaseID,
			ToSemanticVersion: toRelease.SemanticVersion, DiffJSON: string(raw),
			State: AgentUpgradeProposalStateOpen,
		}); err != nil {
			return err
		}
	}
	return nil
}

// decodeUpgradeProposal decodes the stored DiffJSON. A malformed payload is
// a storage defect: it surfaces as an error (500 path) instead of a silent
// empty diff.
func decodeUpgradeProposal(row *types.AgentUpgradeProposalEntity) (interfaces.UpgradeProposalView, error) {
	view := interfaces.UpgradeProposalView{
		AgentUpgradeProposalEntity: *row,
		Diff: types.AgentUpgradeDiff{
			Behavior:     []types.UpgradeFieldChange{},
			Dependencies: []types.UpgradeDependencyChange{},
			Security:     []types.UpgradeFieldChange{},
			License:      []types.UpgradeLicenseChange{},
		},
	}
	if strings.TrimSpace(row.DiffJSON) != "" && row.DiffJSON != "{}" {
		if err := json.Unmarshal([]byte(row.DiffJSON), &view.Diff); err != nil {
			return interfaces.UpgradeProposalView{}, fmt.Errorf("decode upgrade proposal %s diff: %w", row.ID, err)
		}
	}
	if view.Diff.Behavior == nil {
		view.Diff.Behavior = []types.UpgradeFieldChange{}
	}
	if view.Diff.Dependencies == nil {
		view.Diff.Dependencies = []types.UpgradeDependencyChange{}
	}
	if view.Diff.Security == nil {
		view.Diff.Security = []types.UpgradeFieldChange{}
	}
	if view.Diff.License == nil {
		view.Diff.License = []types.UpgradeLicenseChange{}
	}
	return view, nil
}
