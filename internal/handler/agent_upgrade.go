package handler

import (
	stderrors "errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	marketrepo "github.com/Tencent/WeKnora/internal/application/repository"
	marketservice "github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// AgentUpgradeHandler is the HTTP boundary for reviewable upgrade proposals
// (T31 #61, spec §9). Same governance floor as the adoption routes: listing
// proposals, accepting (creating the upgrade draft) and dismissing are
// admin-grade acts, so every endpoint is Admin+ with the full-access
// API-key floor. Tenant and actor identity always come from the
// authenticated request context; request bodies never carry principals
// (strict decoding rejects spoof attempts).
type AgentUpgradeHandler struct {
	upgrades interfaces.AgentUpgradeService
}

func NewAgentUpgradeHandler(upgrades interfaces.AgentUpgradeService) *AgentUpgradeHandler {
	return &AgentUpgradeHandler{upgrades: upgrades}
}

type acceptUpgradeProposalBody struct {
	Name string `json:"name"`
}

type upgradeProposalResponse struct {
	ID                string                 `json:"id"`
	AdoptionID        string                 `json:"adoption_id"`
	ListingID         string                 `json:"listing_id"`
	FromReleaseID     string                 `json:"from_release_id"`
	ToReleaseID       string                 `json:"to_release_id"`
	ToSemanticVersion string                 `json:"to_semantic_version"`
	State             string                 `json:"state"`
	AcceptedVariantID string                 `json:"accepted_variant_id,omitempty"`
	ResolvedBy        string                 `json:"resolved_by,omitempty"`
	Diff              types.AgentUpgradeDiff `json:"diff"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

func upgradeProposalDTO(view interfaces.UpgradeProposalView) upgradeProposalResponse {
	return upgradeProposalResponse{
		ID: view.ID, AdoptionID: view.AdoptionID, ListingID: view.ListingID,
		FromReleaseID: view.FromReleaseID, ToReleaseID: view.ToReleaseID,
		ToSemanticVersion: view.ToSemanticVersion, State: view.State,
		AcceptedVariantID: view.AcceptedVariantID, ResolvedBy: view.ResolvedBy,
		Diff: view.Diff, CreatedAt: view.CreatedAt, UpdatedAt: view.UpdatedAt,
	}
}

// acceptUpgradeResponse pairs the resolved proposal with the freshly
// created upgrade draft. Deliberately NOT the adoption publish envelope
// (B3-F86 applies to variant-state endpoints): acceptance creates a draft
// AND resolves the proposal, so the response names both.
type acceptUpgradeResponse struct {
	Proposal upgradeProposalResponse `json:"proposal"`
	Variant  adoptionVariantResponse `json:"variant"`
}

func upgradeClientError(err error) error {
	switch {
	case stderrors.Is(err, marketservice.ErrAgentUpgradeNotFound),
		stderrors.Is(err, marketrepo.ErrAgentAdoptionNotFound),
		stderrors.Is(err, marketrepo.ErrAgentUpgradeProposalNotFound):
		return apperrors.NewNotFoundError("agent upgrade proposal not found")
	case stderrors.Is(err, marketservice.ErrAgentUpgradeStateConflict),
		stderrors.Is(err, marketrepo.ErrAgentUpgradeProposalTransition),
		stderrors.Is(err, marketrepo.ErrAgentAdoptionVariantTransition):
		// The refusal message IS the explicit reason (terminal state,
		// conflicting adoption state) — pass it through verbatim.
		return apperrors.NewConflictError(err.Error())
	case stderrors.Is(err, marketservice.ErrAgentUpgradeInvalidInput):
		return apperrors.NewValidationError("invalid agent upgrade proposal request")
	default:
		return err
	}
}

func (h *AgentUpgradeHandler) ListUpgradeProposals(c *gin.Context) {
	views, err := h.upgrades.ListUpgradeProposals(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(upgradeClientError(err))
		return
	}
	data := make([]upgradeProposalResponse, 0, len(views))
	for _, view := range views {
		data = append(data, upgradeProposalDTO(view))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *AgentUpgradeHandler) GetUpgradeProposal(c *gin.Context) {
	view, err := h.upgrades.GetUpgradeProposal(c.Request.Context(), sandboxConfigTenantID(c), c.Param("id"))
	if err != nil {
		_ = c.Error(upgradeClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": upgradeProposalDTO(view)})
}

func (h *AgentUpgradeHandler) AcceptUpgradeProposal(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *acceptUpgradeProposalBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.Name) == "" {
		invalidMarketplaceBody(c, stderrors.New("name is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	variant, proposal, err := h.upgrades.AcceptUpgradeProposal(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"), interfaces.UpgradeVariantInput{Name: body.Name})
	if err != nil {
		_ = c.Error(upgradeClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": acceptUpgradeResponse{
		Proposal: upgradeProposalDTO(proposal), Variant: adoptionVariantDTO(variant),
	}})
}

func (h *AgentUpgradeHandler) DismissUpgradeProposal(c *gin.Context) {
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.upgrades.DismissUpgradeProposal(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"))
	if err != nil {
		_ = c.Error(upgradeClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": upgradeProposalDTO(view)})
}
