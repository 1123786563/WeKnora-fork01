package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// AgentUpgradeService is the reviewable upgrade-proposal surface (T31,
// Ticket #61; spec §9). A Listing advancing to a new Release only ever
// GENERATES a proposal; acceptance creates a new Variant draft pinned to
// the proposal's release and the draft then flows through the #59
// mapping/test/publish state machine unchanged. The previously accepted
// release, other Variants and existing Tasks never change (CONTEXT.md
// 「Agent 升级建议」). HTTP authorization stays at the route boundary;
// Tenant and actor identity always come from the authenticated request
// context.
type AgentUpgradeService interface {
	// ListUpgradeProposals reconciles open proposals for the tenant's active
	// adoptions against each listing's current release, then lists all
	// proposals (open + resolved) ordered created_at ASC.
	ListUpgradeProposals(ctx context.Context, tenantID uint64) ([]UpgradeProposalView, error)
	GetUpgradeProposal(ctx context.Context, tenantID uint64, proposalID string) (UpgradeProposalView, error)
	// AcceptUpgradeProposal creates the upgrade Variant draft (state draft,
	// Release = proposal.ToReleaseID) and CAS-marks the proposal accepted
	// with the draft's id.
	AcceptUpgradeProposal(ctx context.Context, tenantID uint64, actorID, proposalID string, input UpgradeVariantInput) (AdoptionVariantView, UpgradeProposalView, error)
	// DismissUpgradeProposal CAS-marks the proposal dismissed. Both states
	// are terminal: reconcile never resurrects a resolved proposal.
	DismissUpgradeProposal(ctx context.Context, tenantID uint64, actorID, proposalID string) (UpgradeProposalView, error)
}

// UpgradeVariantInput names the upgrade Variant draft to create on
// acceptance (the proposal pins the release; the admin names the draft).
type UpgradeVariantInput struct{ Name string }

// UpgradeProposalView is the reviewable wire projection: the persisted
// proposal row plus its decoded four-dimension diff.
type UpgradeProposalView struct {
	types.AgentUpgradeProposalEntity
	Diff types.AgentUpgradeDiff `json:"diff"`
}
