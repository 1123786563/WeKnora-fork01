package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// maxExpertDescriptionLen caps the publish-body description override (the
// 2000-character house style for long text fields); the name cap reuses
// the agent name column limit (maxExpertAgentNameLen).
const maxExpertDescriptionLen = 2000

// TenantExpertMarketHandler serves the tenant-internal expert market API
// (M4 Task 5): a member exports one of their agents as an immutable expert
// snapshot every workspace member can list and install into their own
// agent through the M2 expert instantiation. Publishing honors the M2 §5
// whitelist — KB bindings, model keys, sandbox bindings and memory never
// enter the snapshot.
type TenantExpertMarketHandler struct {
	market interfaces.TenantExpertMarketService
}

// NewTenantExpertMarketHandler creates the tenant expert-market handler.
func NewTenantExpertMarketHandler(market interfaces.TenantExpertMarketService) *TenantExpertMarketHandler {
	return &TenantExpertMarketHandler{market: market}
}

// tenantExpertPublishBody is the POST /agents/:id/publish-expert request:
// optional display overrides for the publish row and the snapshot
// manifest. Decoding is strict — the overrides are the only
// client-supplied shape, the tenant and publisher are derived server-side —
// so unknown fields and trailing input are rejected.
type tenantExpertPublishBody struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func decodeTenantExpertPublishBody(r io.Reader) (interfaces.PublishAgentExpertRequest, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return interfaces.PublishAgentExpertRequest{}, err
	}
	var req interfaces.PublishAgentExpertRequest
	if len(strings.TrimSpace(string(raw))) == 0 {
		return req, nil // empty body: no overrides
	}
	// A pointer target so the `null` literal — which decodes into anything
	// without error — lands on a nil body and is refused like every other
	// non-object input (the decodeStrictEmptyBody rule).
	var body *tenantExpertPublishBody
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		return req, err
	}
	if body == nil {
		return req, errors.New("body must be an object")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return req, errors.New("trailing input")
	}
	if body.Name != nil {
		req.Name = strings.TrimSpace(*body.Name)
	}
	if body.Description != nil {
		req.Description = strings.TrimSpace(*body.Description)
	}
	return req, nil
}

// PublishAgentExpert godoc
// @Summary      Publish an agent as a workspace expert
// @Description  Exports the agent into an immutable expert snapshot
// @Description  (persona/system prompt, skill references, subagents,
// @Description  starters — never KB/model/sandbox/memory bindings) and
// @Description  lists it on this workspace's internal expert market.
// @Description  Re-publishing refreshes the same entry. The body is
// @Description  optional {name?, description?} overrides.
// @Tags         Experts
// @Accept       json
// @Produce      json
// @Param        id      path  string  true  "Agent ID"
// @Param        request body  object  false "Optional {name, description} overrides"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError  "Unknown agent"
// @Router       /agents/{id}/publish-expert [post]
func (h *TenantExpertMarketHandler) PublishAgentExpert(c *gin.Context) {
	limitJSONBody(c, skillSourceJSONMaxBytes)
	req, err := decodeTenantExpertPublishBody(c.Request.Body)
	if err != nil {
		if isRequestBodyTooLarge(err) {
			_ = c.Error(skillJSONRequestTooLargeError())
			return
		}
		_ = c.Error(apperrors.NewBadRequestError("invalid publish request: " + err.Error()))
		return
	}
	if utf8.RuneCountInString(req.Name) > maxExpertAgentNameLen {
		_ = c.Error(apperrors.NewBadRequestError("name exceeds 255 characters"))
		return
	}
	if utf8.RuneCountInString(req.Description) > maxExpertDescriptionLen {
		_ = c.Error(apperrors.NewBadRequestError("description exceeds 2000 characters"))
		return
	}
	// published_by is the ctx user; an absent id (machine principal) means
	// a system publish, recorded as the empty string.
	publishedBy, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.market.PublishAgentExpert(
		c.Request.Context(), sandboxConfigTenantID(c), c.Param("id"), publishedBy, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": view})
}

// UnpublishExpert godoc
// @Summary      Unpublish a workspace expert
// @Description  Removes the expert from this workspace's internal market
// @Description  (soft delete; the snapshot and any installed copies stay).
// @Tags         Experts
// @Produce      json
// @Param        id  path  string  true  "Published expert ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  apperrors.AppError
// @Router       /market/tenant/experts/{id} [delete]
func (h *TenantExpertMarketHandler) UnpublishExpert(c *gin.Context) {
	if err := h.market.UnpublishExpert(c.Request.Context(), sandboxConfigTenantID(c), c.Param("id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListPublishedExperts godoc
// @Summary      List this workspace's published experts
// @Description  Every expert published to the tenant-internal market with
// @Description  its publish-time metadata, the publisher and whether this
// @Description  workspace already installed it.
// @Tags         Experts
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /market/tenant/experts [get]
func (h *TenantExpertMarketHandler) ListPublishedExperts(c *gin.Context) {
	index, err := h.market.ListPublishedExperts(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	if index == nil || index.Experts == nil {
		index = &interfaces.PublishedExpertIndex{Experts: []interfaces.PublishedExpertEntry{}}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": index})
}

// InstallPublishedExpert godoc
// @Summary      Install a published expert as an agent
// @Description  Seeds this workspace's installed-experts root from the
// @Description  published snapshot and instantiates it as a custom agent
// @Description  (the POST /experts/{id}/instantiate contract: referenced
// @Description  skills resolve installed → selected, otherwise pending).
// @Tags         Experts
// @Accept       json
// @Produce      json
// @Param        id      path  string  true  "Published expert ID"
// @Param        request body  object  false "Optional {agent_name, sandbox_config_id}"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError  "Expert is not published"
// @Router       /market/tenant/experts/{id}/install [post]
func (h *TenantExpertMarketHandler) InstallPublishedExpert(c *gin.Context) {
	limitJSONBody(c, skillSourceJSONMaxBytes)
	req, err := decodeExpertInstantiateBody(c.Request.Body)
	if err != nil {
		if isRequestBodyTooLarge(err) {
			_ = c.Error(skillJSONRequestTooLargeError())
			return
		}
		_ = c.Error(apperrors.NewBadRequestError("invalid install request: " + err.Error()))
		return
	}
	if utf8.RuneCountInString(req.AgentName) > maxExpertAgentNameLen {
		_ = c.Error(apperrors.NewBadRequestError("agent_name exceeds 255 characters"))
		return
	}
	result, err := h.market.InstallPublishedExpert(
		c.Request.Context(), sandboxConfigTenantID(c), c.Param("id"), req)
	if err != nil {
		_ = c.Error(expertServiceError(err))
		return
	}
	pending := result.PendingSkills
	if pending == nil {
		pending = []string{}
	}
	installIDs := result.SkillInstallIDs
	if installIDs == nil {
		installIDs = []string{}
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{
		"agent":             result.Agent,
		"pending_skills":    pending,
		"skill_install_ids": installIDs,
	}})
}
