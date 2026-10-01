package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

// AgentUpgradeSystemResolvedBy 标记 reconcile 的系统级终态迁移（stale open
// 随 Adoption 前进迁 dismissed），与人工 dismiss 的 resolved_by（actor id）
// 可区分；varchar(255) 宽度内。
const AgentUpgradeSystemResolvedBy = "system:adoption-advanced"

type AgentUpgradeService struct {
	repo                repository.AgentUpgradeRepository
	releaseSecurityGate ReleaseSecurityGate
}

var _ interfaces.AgentUpgradeService = (*AgentUpgradeService)(nil)

func NewAgentUpgradeService(repo repository.AgentUpgradeRepository) *AgentUpgradeService {
	return &AgentUpgradeService{repo: repo}
}

// SetReleaseSecurityGate installs the optional security admission check used
// before an upgrade proposal can create a local variant.
func (s *AgentUpgradeService) SetReleaseSecurityGate(gate ReleaseSecurityGate) {
	s.releaseSecurityGate = gate
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
	// 过期建议不可 Accept（R5-F2）：from_release 已不是 Adoption 现行接受
	// 指针时，接受只会创建指向已被越过 Release 的冗余草稿 Variant。
	if row.FromReleaseID != adoption.AcceptedReleaseID {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{},
			fmt.Errorf("%w: proposal's from release no longer matches the adoption's accepted release", ErrAgentUpgradeStateConflict)
	}
	toRelease, err := s.repo.GetRelease(ctx, tenantID, row.ToReleaseID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	if toRelease == nil || toRelease.ListingID != row.ListingID {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{},
			fmt.Errorf("%w: proposed release does not belong to the listing", ErrAgentUpgradeInvalidInput)
	}
	if s.releaseSecurityGate != nil {
		if err := s.releaseSecurityGate.ReleaseAdmission(ctx, tenantID, row.ToReleaseID); err != nil {
			return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
		}
	}
	if toRelease.DeprecatedAt != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{},
			fmt.Errorf("%w: proposed release %s is deprecated; successor: %s", ErrAgentUpgradeStateConflict, toRelease.ID, successorHint(toRelease.SuccessorReleaseID))
	}
	// 接受 = 以新 Release 创建一个新的草稿 Variant（spec §9）。随后走 #59
	// 既有 mapping/test/publish 流程；本方法绝不触碰其他 Variant 或既有
	// 本地 Agent Version。
	//
	// 已知非事务窗口（计划自身的显式设计，最终审查 minor 已契约化）：
	// CreateVariant 先于 CAS TransitionProposal 落库。并发双 accept 的 CAS
	// 败者收到 ErrAgentUpgradeStateConflict（HTTP 409），但其已创建的草稿
	// Variant 留存为「孤儿草稿」。该孤儿被如下不变量约束为良性：
	//   1. state 停留 draft、固定在 proposal 的 to_release——不进入
	//      available-agents（PublishedAvailableAgents 只读 published）；
	//   2. proposal.accepted_variant_id 只指向 CAS 赢家的草稿，孤儿不被
	//      任何终态行引用；建议保持 accepted 终态、不因孤儿复活；
	//   3. spec §9 允许升级草稿共存，管理员可经 #59 既有流程处置或搁置。
	// TestAgentUpgradeServiceAcceptRaceLoserLeavesBenignOrphanDraft 钉死
	// 这些不变量。若未来要求「败者草稿必须回收」，应在 repository 层新增
	// 事务原语（CreateVariant+CAS 同事务）并升级计划，而非在本服务层静默
	// 补删。
	variant, err := s.repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{
		TenantID: tenantID, AdoptionID: row.AdoptionID, ReleaseID: row.ToReleaseID,
		Name: input.Name, State: AgentVariantStateDraft, CreatedBy: actorID,
	})
	if err != nil {
		if errors.Is(err, repository.ErrAgentMarketplaceListingUnavailable) || errors.Is(err, repository.ErrAgentMarketplaceReleaseDeprecated) {
			return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, fmt.Errorf("%w: source listing or release is no longer eligible", ErrAgentUpgradeStateConflict)
		}
		if errors.Is(err, repository.ErrAgentAdoptionTransition) {
			return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, fmt.Errorf("%w: %w", ErrAgentUpgradeStateConflict, err)
		}
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
		if errors.Is(err, repository.ErrAgentUpgradeProposalTransition) {
			return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, fmt.Errorf("%w: %w: source listing or release is no longer eligible", ErrAgentUpgradeStateConflict, err)
		}
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
//
// Lifecycle discipline: (a) already-materialized pairs (ANY state)
// short-circuit before the expensive release reads + two-bundle decode —
// the read path stays O(new pairs) instead of recomputing every row on
// every request; (b) an open proposal whose from_release is no longer the
// adoption's accepted pointer (re-adopt advanced it) migrates to the
// terminal dismissed state with the system marker — stale opens never
// linger as acceptable.
func (s *AgentUpgradeService) reconcileProposals(ctx context.Context, tenantID uint64) error {
	adoptions, err := s.repo.ListAdoptions(ctx, tenantID)
	if err != nil {
		return err
	}
	rows, err := s.repo.ListProposals(ctx, tenantID)
	if err != nil {
		return err
	}
	materialized := make(map[string]struct{}, len(rows))
	for i := range rows {
		materialized[rows[i].AdoptionID+"\x00"+rows[i].ToReleaseID] = struct{}{}
	}
	for i := range adoptions {
		adoption := adoptions[i]
		if adoption.State != AgentAdoptionStateActive {
			continue
		}
		// Stale open 迁移（re-adopt 前进后 from_release 脱节）：迁终态
		// dismissed，resolved_by 用系统标记区分人工 dismiss。
		for j := range rows {
			row := &rows[j]
			if row.AdoptionID != adoption.ID || row.State != AgentUpgradeProposalStateOpen {
				continue
			}
			if row.FromReleaseID == adoption.AcceptedReleaseID {
				continue
			}
			if _, terr := s.repo.TransitionProposal(ctx, tenantID, row.ID,
				[]string{AgentUpgradeProposalStateOpen}, AgentUpgradeProposalStateDismissed,
				map[string]any{"resolved_by": AgentUpgradeSystemResolvedBy}); terr != nil {
				return terr
			}
			row.State = AgentUpgradeProposalStateDismissed
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
		if _, done := materialized[adoption.ID+"\x00"+toReleaseID]; done {
			// 已物化（任意状态）：昂贵 diff 之前短路——diff 是既有行的稳定
			// 记录（release 不可变），重算只是读路径的 O(N) 税。
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
		if toRelease.DeprecatedAt != nil {
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
			if errors.Is(err, repository.ErrAgentMarketplaceListingUnavailable) || errors.Is(err, repository.ErrAgentMarketplaceReleaseDeprecated) || errors.Is(err, repository.ErrAgentAdoptionTransition) {
				// Eligibility changed after the reconcile snapshot; the transactional
				// repository guard deliberately refuses stale materialization.
				continue
			}
			return err
		}
		materialized[adoption.ID+"\x00"+toReleaseID] = struct{}{}
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
