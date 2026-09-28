package handler

import (
	stderrors "errors"
	"net/http"
	"strings"

	marketservice "github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// AgentSecurityHandler exposes the tenant security-revocation audit ledger.
type AgentSecurityHandler struct {
	security interfaces.AgentSecurityService
}

func NewAgentSecurityHandler(security interfaces.AgentSecurityService) *AgentSecurityHandler {
	return &AgentSecurityHandler{security: security}
}

type revokeReleaseBody struct {
	ReleaseID            string `json:"release_id"`
	Reason               string `json:"reason"`
	ReplacementReleaseID string `json:"replacement_release_id,omitempty"`
	InFlightDisposition  string `json:"in_flight_disposition,omitempty"`
}

type revokeDependencyBody struct {
	Dependency          *agentSecurityDependencyBody `json:"dependency"`
	Reason              string                       `json:"reason"`
	ReplacementVersion  string                       `json:"replacement_version,omitempty"`
	InFlightDisposition string                       `json:"in_flight_disposition,omitempty"`
}

type agentSecurityDependencyBody struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

func securityClientError(err error) error {
	switch {
	case stderrors.Is(err, marketservice.ErrAgentSecurityInvalidInput), stderrors.Is(err, marketservice.ErrAgentSecurityReleaseUnresolvable):
		return apperrors.NewValidationError("invalid agent security revocation request")
	case stderrors.Is(err, marketservice.ErrAgentSecurityNotFound):
		return apperrors.NewNotFoundError("agent security revocation not found")
	default:
		return err
	}
}

func (h *AgentSecurityHandler) RevokeRelease(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *revokeReleaseBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.ReleaseID) == "" || strings.TrimSpace(body.Reason) == "" {
		invalidMarketplaceBody(c, stderrors.New("release_id and reason are required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.security.RevokeRelease(c.Request.Context(), sandboxConfigTenantID(c), actorID, interfaces.ReleaseRevocationInput{
		ReleaseID: body.ReleaseID, Reason: body.Reason, ReplacementReleaseID: body.ReplacementReleaseID, InFlightDisposition: body.InFlightDisposition,
	})
	if err != nil {
		_ = c.Error(securityClientError(err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": view})
}

func (h *AgentSecurityHandler) RevokeDependency(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *revokeDependencyBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || body.Dependency == nil || strings.TrimSpace(body.Reason) == "" {
		invalidMarketplaceBody(c, stderrors.New("dependency and reason are required"))
		return
	}
	dep := body.Dependency
	if strings.TrimSpace(dep.Type) == "" || strings.TrimSpace(dep.ID) == "" || strings.TrimSpace(dep.Version) == "" || strings.TrimSpace(dep.Digest) == "" {
		invalidMarketplaceBody(c, stderrors.New("dependency type, id, version, and digest are required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.security.RevokeDependency(c.Request.Context(), sandboxConfigTenantID(c), actorID, interfaces.DependencyRevocationInput{
		Dependency: types.AgentReleaseDependency{Type: dep.Type, ID: dep.ID, Version: dep.Version, Digest: dep.Digest},
		Reason:     body.Reason, ReplacementVersion: body.ReplacementVersion, InFlightDisposition: body.InFlightDisposition,
	})
	if err != nil {
		_ = c.Error(securityClientError(err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": view})
}

func (h *AgentSecurityHandler) ListRevocations(c *gin.Context) {
	views, err := h.security.ListRevocations(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(securityClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": views})
}

func (h *AgentSecurityHandler) GetRevocation(c *gin.Context) {
	view, err := h.security.GetRevocation(c.Request.Context(), sandboxConfigTenantID(c), c.Param("id"))
	if err != nil {
		_ = c.Error(securityClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}
