package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TenantSkillMarketHandler serves the tenant-internal skill market API:
// admins publish/unpublish workspace catalog skills and every member of the
// SAME tenant lists them and installs them onto sandbox configs. The
// publish/unpublish pair flags visibility on the shared catalog definition —
// no content is copied — and install composes with the plain catalog
// install, so its 202 answer shape is identical.
type TenantSkillMarketHandler struct {
	market interfaces.TenantSkillMarketService
}

// NewTenantSkillMarketHandler creates the tenant skill-market handler.
func NewTenantSkillMarketHandler(market interfaces.TenantSkillMarketService) *TenantSkillMarketHandler {
	return &TenantSkillMarketHandler{market: market}
}

// decodeStrictEmptyBody accepts only an empty or `{}` JSON body (publish
// takes no client input). Unknown fields, non-object input (including the
// null literal) and trailing values are refused — the same strictness
// marketSkillInstallBody set.
func decodeStrictEmptyBody(r io.Reader) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	// A map (not a struct) so `null` — which decodes into anything without
	// error — lands on the nil branch and is refused like every other
	// non-object body.
	var body map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	if err := dec.Decode(&body); err != nil {
		return err
	}
	if body == nil {
		return errors.New("body must be an object")
	}
	if len(body) > 0 {
		return errors.New("unknown fields")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("trailing input")
	}
	return nil
}

// tenantMarketInstallBody is the POST /market/tenant/skills/:catalogId/install
// request. Decoding is strict — the sandbox list is the only client-supplied
// shape; the tenant is derived server-side — so unknown fields and trailing
// input are rejected.
type tenantMarketInstallBody struct {
	SandboxConfigIDs []string `json:"sandbox_config_ids"`
}

func decodeTenantMarketInstallBody(r io.Reader) (tenantMarketInstallBody, error) {
	var body tenantMarketInstallBody
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		return tenantMarketInstallBody{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return tenantMarketInstallBody{}, errors.New("trailing input")
	}
	return body, nil
}

// PublishSkill godoc
// @Summary      Publish a workspace skill to the tenant market
// @Description  Flags a catalog skill as visible in this workspace's internal
// @Description  market. Re-publishing refreshes the publisher and timestamp
// @Description  (idempotent). The body must be empty or {}.
// @Tags         Skills
// @Accept       json
// @Produce      json
// @Param        id      path  string  true  "Catalog skill ID"
// @Param        request body  object  false "Must be empty or {}"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError  "Unknown skill"
// @Router       /skills/catalog/{id}/publish [post]
func (h *TenantSkillMarketHandler) PublishSkill(c *gin.Context) {
	limitJSONBody(c, skillSourceJSONMaxBytes)
	if err := decodeStrictEmptyBody(c.Request.Body); err != nil {
		if isRequestBodyTooLarge(err) {
			_ = c.Error(skillJSONRequestTooLargeError())
			return
		}
		_ = c.Error(apperrors.NewBadRequestError("publish takes no body: send an empty or {} request"))
		return
	}
	// published_by is the ctx user; an absent id (machine principal) means
	// a system publish, recorded as the empty string.
	publishedBy, _ := types.UserIDFromContext(c.Request.Context())
	row, err := h.market.PublishSkill(c.Request.Context(), sandboxConfigTenantID(c), c.Param("id"), publishedBy)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": row})
}

// UnpublishSkill godoc
// @Summary      Unpublish a workspace skill from the tenant market
// @Description  Removes the market flag. Idempotent while the catalog skill
// @Description  exists; unknown skills answer 404.
// @Tags         Skills
// @Produce      json
// @Param        id  path  string  true  "Catalog skill ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  apperrors.AppError
// @Router       /skills/catalog/{id}/publish [delete]
func (h *TenantSkillMarketHandler) UnpublishSkill(c *gin.Context) {
	if err := h.market.UnpublishSkill(c.Request.Context(), sandboxConfigTenantID(c), c.Param("id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListPublishedSkills godoc
// @Summary      List this workspace's published skills
// @Description  Every skill published to the tenant-internal market, with
// @Description  its definition metadata, the publisher and whether any
// @Description  sandbox in this workspace already runs it.
// @Tags         Skills
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /market/tenant/skills [get]
func (h *TenantSkillMarketHandler) ListPublishedSkills(c *gin.Context) {
	index, err := h.market.ListPublishedSkills(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	if index == nil || index.Skills == nil {
		index = &interfaces.PublishedSkillIndex{Skills: []interfaces.PublishedSkillEntry{}}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": index})
}

// InstallPublishedSkill godoc
// @Summary      Install a published skill onto sandboxes
// @Description  Runs the catalog install of a market-published skill onto
// @Description  each named sandbox config. Only published skills install
// @Description  through this surface; accepted while installs run and
// @Description  per-config failures ride the 202 body (the
// @Description  POST /skills/catalog/{id}/install contract).
// @Tags         Skills
// @Accept       json
// @Produce      json
// @Param        catalogId  path  string  true  "Catalog skill ID"
// @Param        request    body  tenantMarketInstallBody  true  "Sandbox configs"
// @Success      202  {object}  map[string]interface{}
// @Failure      400  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError  "Skill is not published"
// @Router       /market/tenant/skills/{catalogId}/install [post]
func (h *TenantSkillMarketHandler) InstallPublishedSkill(c *gin.Context) {
	limitJSONBody(c, skillSourceJSONMaxBytes)
	body, err := decodeTenantMarketInstallBody(c.Request.Body)
	if err != nil {
		if isRequestBodyTooLarge(err) {
			_ = c.Error(skillJSONRequestTooLargeError())
			return
		}
		_ = c.Error(apperrors.NewBadRequestError("invalid install request"))
		return
	}
	result, err := h.market.InstallPublishedSkill(
		c.Request.Context(), sandboxConfigTenantID(c), c.Param("catalogId"), body.SandboxConfigIDs,
	)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// Partial per-config failures keep the 202 (the catalog-install mirror):
	// the accepted installs are running, the errors say which configs never
	// started.
	success := true
	if result == nil {
		result = &interfaces.TenantSkillInstallResult{Installs: map[string]string{}}
	} else if len(result.Errors) > 0 {
		success = false
	}
	data := gin.H{"installs": result.Installs}
	if len(result.Errors) > 0 {
		data["errors"] = result.Errors
	}
	c.JSON(http.StatusAccepted, gin.H{"success": success, "data": data})
}
