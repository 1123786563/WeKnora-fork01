package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	marketrepo "github.com/Tencent/WeKnora/internal/application/repository"
	marketservice "github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type AgentMarketplaceLifecycleHandler struct {
	lifecycle interfaces.AgentMarketplaceLifecycleService
}

func NewAgentMarketplaceLifecycleHandler(lifecycle interfaces.AgentMarketplaceLifecycleService) *AgentMarketplaceLifecycleHandler {
	return &AgentMarketplaceLifecycleHandler{lifecycle: lifecycle}
}

type deprecateReleaseBody struct {
	SuccessorReleaseID string `json:"successor_release_id"`
}

type lifecycleReleaseResponse struct {
	ID                 string     `json:"id"`
	ListingID          string     `json:"listing_id"`
	SemanticVersion    string     `json:"semantic_version"`
	DeprecatedAt       *time.Time `json:"deprecated_at,omitempty"`
	DeprecatedBy       string     `json:"deprecated_by,omitempty"`
	SuccessorReleaseID string     `json:"successor_release_id,omitempty"`
}

func lifecycleClientError(err error) error {
	switch {
	case errors.Is(err, marketservice.ErrAgentAdoptionNotFound),
		errors.Is(err, marketrepo.ErrAgentAdoptionNotFound),
		errors.Is(err, marketrepo.ErrAgentMarketplaceNotFound):
		return apperrors.NewNotFoundError("agent marketplace lifecycle resource not found")
	case errors.Is(err, marketservice.ErrAgentAdoptionStateConflict),
		errors.Is(err, marketservice.ErrAgentAdoptionVariantNotRunnable),
		errors.Is(err, marketservice.ErrAgentReleaseDeprecated),
		errors.Is(err, marketrepo.ErrAgentAdoptionVariantTransition),
		errors.Is(err, marketrepo.ErrAgentAdoptionTransition),
		errors.Is(err, marketrepo.ErrAgentAdoptionRemapStateConflict),
		errors.Is(err, marketrepo.ErrAgentAdoptionEndPrecondition),
		errors.Is(err, marketrepo.ErrAgentMarketplaceListingTransition),
		errors.Is(err, marketrepo.ErrAgentReleaseDeprecateConflict):
		return apperrors.NewConflictError(err.Error())
	case errors.Is(err, marketservice.ErrAgentAdoptionInvalidInput),
		errors.Is(err, marketservice.ErrAgentMarketplaceInvalidInput),
		errors.Is(err, marketservice.ErrAgentMarketplaceLifecycleInvalidInput):
		return apperrors.NewValidationError(err.Error())
	default:
		return err
	}
}

func (h *AgentMarketplaceLifecycleHandler) RetireVariant(c *gin.Context) {
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.lifecycle.RetireVariant(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"))
	if err != nil {
		_ = c.Error(lifecycleClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": adoptionVariantDTO(view)})
}

func (h *AgentMarketplaceLifecycleHandler) EndAdoption(c *gin.Context) {
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.lifecycle.EndAdoption(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"))
	if err != nil {
		_ = c.Error(lifecycleClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": adoptionDTO(view)})
}

func (h *AgentMarketplaceLifecycleHandler) UnlistListing(c *gin.Context) {
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.lifecycle.UnlistListing(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"))
	if err != nil {
		_ = c.Error(lifecycleClientError(err))
		return
	}
	row := view.AgentMarketplaceListingEntity
	c.JSON(http.StatusOK, gin.H{"success": true, "data": marketplaceListingResponse{
		ID: row.ID, TenantID: row.TenantID, SourceAgentID: row.SourceAgentID,
		DisplayName: row.DisplayName, Summary: row.Summary, State: row.State,
		CurrentReleaseID: row.CurrentReleaseID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}})
}

func (h *AgentMarketplaceLifecycleHandler) DeprecateRelease(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *deprecateReleaseBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.SuccessorReleaseID) == "" {
		invalidMarketplaceBody(c, errors.New("successor_release_id is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	release, err := h.lifecycle.DeprecateRelease(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"), body.SuccessorReleaseID)
	if err != nil {
		_ = c.Error(lifecycleClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": lifecycleReleaseResponse{
		ID: release.ID, ListingID: release.ListingID, SemanticVersion: release.SemanticVersion,
		DeprecatedAt: release.DeprecatedAt, DeprecatedBy: release.DeprecatedBy,
		SuccessorReleaseID: release.SuccessorReleaseID,
	}})
}
