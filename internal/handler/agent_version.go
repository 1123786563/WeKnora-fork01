package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// AgentVersionHandler serves the Agent domain's immutable version surface
// (T28 Wave 1): freezing one of the tenant's agents appends an immutable
// snapshot row; reads return the frozen versions. Tenant and actor are
// always derived server-side from the authenticated context — the freeze
// takes no request body, so nothing client-supplied can influence the
// snapshot identity.
type AgentVersionHandler struct {
	versions interfaces.AgentVersionService
}

// NewAgentVersionHandler creates the agent version handler.
func NewAgentVersionHandler(versions interfaces.AgentVersionService) *AgentVersionHandler {
	return &AgentVersionHandler{versions: versions}
}

// FreezeAgentVersion godoc
// @Summary      Freeze an immutable version of an agent
// @Description  Appends an immutable snapshot of the agent (canonical JSON
// @Description  plus source digest) as the next version number within this
// @Description  workspace. Frozen versions never change; later edits to the
// @Description  agent do not rewrite them.
// @Tags         Agents
// @Produce      json
// @Param        id   path  string  true  "Agent ID"
// @Success      201  {object}  map[string]interface{}
// @Failure      404  {object}  apperrors.AppError  "Unknown agent"
// @Router       /agents/{id}/versions [post]
func (h *AgentVersionHandler) FreezeAgentVersion(c *gin.Context) {
	// actor is the ctx user; an absent id (machine principal) means a
	// system freeze, recorded as the empty string.
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.versions.FreezeAgentVersion(
		c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": view})
}

// ListAgentVersions godoc
// @Summary      List an agent's frozen versions
// @Description  Every immutable version of the agent inside this workspace,
// @Description  ascending by version number.
// @Tags         Agents
// @Produce      json
// @Param        id   path  string  true  "Agent ID"
// @Success      200  {object}  map[string]interface{}
// @Router       /agents/{id}/versions [get]
func (h *AgentVersionHandler) ListAgentVersions(c *gin.Context) {
	views, err := h.versions.ListAgentVersions(
		c.Request.Context(), sandboxConfigTenantID(c), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	if views == nil {
		views = []interfaces.AgentVersionView{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": views})
}

// GetAgentVersion godoc
// @Summary      Read one frozen agent version
// @Description  The immutable version with its decoded agent snapshot,
// @Description  digest-verified against the stored bytes. Unknown and
// @Description  other-workspace version ids read as not-found.
// @Tags         Agents
// @Produce      json
// @Param        id          path  string  true  "Agent ID"
// @Param        versionId   path  string  true  "Frozen version ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  apperrors.AppError  "Unknown version"
// @Router       /agents/{id}/versions/{versionId} [get]
func (h *AgentVersionHandler) GetAgentVersion(c *gin.Context) {
	snapshot, err := h.versions.GetAgentVersion(
		c.Request.Context(), sandboxConfigTenantID(c), c.Param("versionId"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": snapshot})
}
