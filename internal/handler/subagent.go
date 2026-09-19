package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// SubagentHandler serves the builtin sub-agent role catalog API: the library
// views (list + detail, with the tenant's installed flags) and the
// per-agent install/remove endpoints that copy a catalog role into the
// tenant's storage and reference it from the agent's config. All state
// changes go through SubagentService, which itself delegates agent
// persistence to CustomAgentService — so tenant scoping matches every other
// agent-config path (the persona Apply pattern).
type SubagentHandler struct {
	subagentService interfaces.SubagentService
}

// NewSubagentHandler creates a new subagent handler instance.
func NewSubagentHandler(subagentService interfaces.SubagentService) *SubagentHandler {
	return &SubagentHandler{subagentService: subagentService}
}

// tenantFromContext extracts the execution tenant the way the expert
// instantiate endpoint does: the tenant is ctx-derived, never body-borne.
func tenantFromContext(c *gin.Context) (uint64, bool) {
	return types.TenantIDFromContext(c.Request.Context())
}

// ListCatalog godoc
// @Summary      List the builtin sub-agent catalog
// @Description  The builtin role library for this tenant's view: divisions in
// @Description  divisions.json order with per-division counts, slug-sorted
// @Description  entries carrying both locales' names, and the tenant's
// @Description  installed flags. Locale filtering is client-side.
// @Tags         Subagents
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Catalog with installed flags"
// @Router       /subagent-catalog [get]
func (h *SubagentHandler) ListCatalog(c *gin.Context) {
	tenantID, ok := tenantFromContext(c)
	if !ok {
		_ = c.Error(apperrors.NewUnauthorizedError("missing workspace context"))
		return
	}
	catalog, err := h.subagentService.ListCatalog(c.Request.Context(), tenantID)
	if err != nil {
		_ = c.Error(subagentServiceError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": catalog})
}

// GetCatalogEntry godoc
// @Summary      Get one sub-agent catalog entry
// @Description  Detail view of one role: frontmatter scalars and BOTH locale
// @Description  bodies (present-or-empty), plus the tenant's installed state.
// @Tags         Subagents
// @Produce      json
// @Param        slug  path  string  true  "Role slug"
// @Success      200  {object}  map[string]interface{}  "Role detail"
// @Failure      404  {object}  apperrors.AppError  "Unknown slug"
// @Router       /subagent-catalog/{slug} [get]
func (h *SubagentHandler) GetCatalogEntry(c *gin.Context) {
	tenantID, ok := tenantFromContext(c)
	if !ok {
		_ = c.Error(apperrors.NewUnauthorizedError("missing workspace context"))
		return
	}
	detail, err := h.subagentService.GetCatalogEntry(c.Request.Context(), tenantID, c.Param("slug"))
	if err != nil {
		_ = c.Error(subagentServiceError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": detail})
}

// ListAgentSubagents godoc
// @Summary      List an agent's configured sub-agents
// @Description  The agent's Config.Subagents slugs, config order preserved.
// @Description  The caller must own the agent (or be Admin+), matching the
// @Description  agent-update guard.
// @Tags         Subagents
// @Produce      json
// @Param        id  path  string  true  "Agent ID"
// @Success      200  {object}  map[string]interface{}  "Configured slugs"
// @Failure      404  {object}  apperrors.AppError  "Agent not found"
// @Router       /agents/{id}/subagents [get]
func (h *SubagentHandler) ListAgentSubagents(c *gin.Context) {
	list, err := h.subagentService.ListAgentSubagents(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(subagentServiceError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"subagents": list}})
}

// subagentInstallBody is the POST /agents/:id/subagents request. slug is
// required; locale optionally pins the library locale ("zh"/"en" — any
// Accept-Language-style value is normalized), defaulting to the request
// locale. Decoding is strict (decodeSubagentInstallBody, the M2 expert
// instantiate precedent): unknown fields and trailing input are rejected.
type subagentInstallBody struct {
	Slug   string `json:"slug"`
	Locale string `json:"locale"`
}

// decodeSubagentInstallBody strictly decodes one install request. An empty
// body is NOT valid here (unlike the expert instantiate body) because the
// slug is required — EOF decodes to an empty request which the caller
// rejects as a 400.
func decodeSubagentInstallBody(r io.Reader) (subagentInstallBody, error) {
	var body subagentInstallBody
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			return subagentInstallBody{}, nil // empty body → empty slug → caller's 400
		}
		return subagentInstallBody{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return subagentInstallBody{}, errors.New("trailing input")
	}
	return subagentInstallBody{
		Slug:   strings.TrimSpace(body.Slug),
		Locale: strings.TrimSpace(body.Locale),
	}, nil
}

// InstallForAgent godoc
// @Summary      Install a sub-agent on an agent
// @Description  Resolves the catalog role (locale → en → zh fallback), copies
// @Description  it verbatim into the tenant's storage, and appends the slug
// @Description  to the agent's config. Idempotent: an already-configured slug
// @Description  refreshes the stored copy and returns 200. The tenant is
// @Description  derived from the authenticated context, never the body.
// @Tags         Subagents
// @Accept       json
// @Produce      json
// @Param        id       path  string                true  "Agent ID"
// @Param        request  body  subagentInstallBody   true  "Role slug and optional locale"
// @Success      200  {object}  map[string]interface{}  "The agent's subagent slugs"
// @Failure      400  {object}  apperrors.AppError  "Invalid body or missing slug"
// @Failure      404  {object}  apperrors.AppError  "Unknown agent or slug"
// @Router       /agents/{id}/subagents [post]
func (h *SubagentHandler) InstallForAgent(c *gin.Context) {
	tenantID, ok := tenantFromContext(c)
	if !ok {
		_ = c.Error(apperrors.NewUnauthorizedError("missing workspace context"))
		return
	}

	body, err := decodeSubagentInstallBody(c.Request.Body)
	if err != nil {
		_ = c.Error(apperrors.NewBadRequestError("invalid body: " + err.Error()))
		return
	}
	if body.Slug == "" {
		_ = c.Error(apperrors.NewBadRequestError("slug required"))
		return
	}

	list, err := h.subagentService.InstallForAgent(
		c.Request.Context(), tenantID, c.Param("id"), body.Slug, body.Locale)
	if err != nil {
		_ = c.Error(subagentServiceError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"subagents": list}})
}

// RemoveFromAgent godoc
// @Summary      Remove a sub-agent from an agent
// @Description  Drops the slug from the agent's config; idempotent (an absent
// @Description  slug is a 200 no-op). The tenant's stored role copy is KEPT:
// @Description  copies are shared across agents, and deleting them is
// @Description  tenant-admin scope, not this endpoint.
// @Tags         Subagents
// @Produce      json
// @Param        id    path  string  true  "Agent ID"
// @Param        slug  path  string  true  "Role slug"
// @Success      200  {object}  map[string]interface{}  "The agent's subagent slugs"
// @Failure      404  {object}  apperrors.AppError  "Agent not found"
// @Router       /agents/{id}/subagents/{slug} [delete]
func (h *SubagentHandler) RemoveFromAgent(c *gin.Context) {
	list, err := h.subagentService.RemoveFromAgent(c.Request.Context(), c.Param("id"), c.Param("slug"))
	if err != nil {
		_ = c.Error(subagentServiceError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"subagents": list}})
}

// subagentServiceError maps SubagentService rejections onto the HTTP error
// taxonomy, following the persona handler convention: the two not-found
// sentinels become 404s, wrapped AppErrors keep their status, and anything
// else is an internal error.
func subagentServiceError(err error) error {
	if errors.Is(err, service.ErrSubagentNotFound) {
		return apperrors.NewNotFoundError("subagent not found")
	}
	if errors.Is(err, service.ErrAgentNotFound) {
		return apperrors.NewNotFoundError("agent not found")
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return apperrors.NewInternalServerError(err.Error())
}
