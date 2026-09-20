package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// defaultMarketSearchLimit is the page size served when the caller omits
// ?limit=; the client clamps anything out of [1, 100].
const defaultMarketSearchLimit = 20

// SkillMarketHandler serves the SkillHub market API: skill search/rankings,
// the remote skill install, the skillset (expert) index and the
// skillset-to-expert install. Every mutating flow derives the tenant from the
// authenticated context, never the body.
type SkillMarketHandler struct {
	market interfaces.SkillMarketService
}

// NewSkillMarketHandler creates a new skill-market handler.
func NewSkillMarketHandler(market interfaces.SkillMarketService) *SkillMarketHandler {
	return &SkillMarketHandler{market: market}
}

// Search godoc
// @Summary      Search the skill market
// @Description  Queries the remote SkillHub registry. A cached answer served
// @Description  after a failed refresh answers 200 with stale=true.
// @Tags         Skills
// @Produce      json
// @Param        q     query  string  false  "Search query (blank coerced by the registry)"
// @Param        limit query  int    false  "Page size (default 20, clamped to [1,100])"
// @Success      200  {object}  map[string]interface{}
// @Failure      503  {object}  apperrors.AppError  "Registry unreachable and no cached value"
// @Router       /skills/market/search [get]
func (h *SkillMarketHandler) Search(c *gin.Context) {
	limit := defaultMarketSearchLimit
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			_ = c.Error(apperrors.NewBadRequestError("limit must be a positive integer"))
			return
		}
		limit = parsed
	}
	listing, err := h.market.SearchMarketSkills(c.Request.Context(), c.Query("q"), limit)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": listing})
}

// Rankings godoc
// @Summary      Skill market rankings
// @Description  One SkillHub showcase list (hot|featured|newest|recommended|trending|paid).
// @Tags         Skills
// @Produce      json
// @Param        kind  path  string  true  "Ranking kind"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  apperrors.AppError  "Unknown ranking kind"
// @Router       /skills/market/rankings/{kind} [get]
func (h *SkillMarketHandler) Rankings(c *gin.Context) {
	listing, err := h.market.MarketSkillRankings(c.Request.Context(), c.Param("kind"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": listing})
}

// marketSkillInstallBody is the POST /skills/market/install request. Decoding
// is strict — slug and the sandbox list are the only client-supplied shape;
// the tenant is derived server-side — so unknown fields and trailing input
// are rejected, and a body is required (the slug cannot be defaulted).
type marketSkillInstallBody struct {
	Slug             string   `json:"slug"`
	SandboxConfigIDs []string `json:"sandbox_config_ids"`
}

func decodeMarketSkillInstallBody(r io.Reader) (marketSkillInstallBody, error) {
	var body marketSkillInstallBody
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		return marketSkillInstallBody{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return marketSkillInstallBody{}, errors.New("trailing input")
	}
	return body, nil
}

// InstallSkill godoc
// @Summary      Install a market skill
// @Description  Downloads the slug's package, validates it, records it in the
// @Description  workspace catalog and installs it onto the named sandbox
// @Description  configs (the same pipeline POST /skills/catalog/:id/install
// @Description  finishes). Accepted while installs run; per-config failures
// @Description  ride the 202 body.
// @Tags         Skills
// @Accept       json
// @Produce      json
// @Param        request  body  marketSkillInstallBody  true  "Slug + sandbox configs"
// @Success      202  {object}  map[string]interface{}
// @Failure      400  {object}  apperrors.AppError
// @Failure      503  {object}  apperrors.AppError  "Registry unreachable"
// @Router       /skills/market/install [post]
func (h *SkillMarketHandler) InstallSkill(c *gin.Context) {
	body, err := decodeMarketSkillInstallBody(c.Request.Body)
	if err != nil {
		_ = c.Error(apperrors.NewBadRequestError("invalid install request"))
		return
	}
	result, err := h.market.InstallMarketSkill(
		c.Request.Context(), sandboxConfigTenantID(c),
		strings.TrimSpace(body.Slug), body.SandboxConfigIDs,
	)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// Partial per-config failures keep the 202 (mirrors the catalog install
	// handler): the accepted installs are running, the errors say which
	// configs never started.
	success := true
	if result == nil {
		result = &interfaces.MarketSkillInstallResult{InstallIDs: []string{}}
	} else if len(result.Errors) > 0 {
		success = false
	}
	c.JSON(http.StatusAccepted, gin.H{"success": success, "data": result})
}

// ListSkillsets godoc
// @Summary      List market skillsets
// @Description  The SkillHub skillset index for this workspace, marking the
// @Description  slugs already installed. Stale answers carry stale=true.
// @Tags         Experts
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      503  {object}  apperrors.AppError  "Registry unreachable and no cached value"
// @Router       /experts/market [get]
func (h *SkillMarketHandler) ListSkillsets(c *gin.Context) {
	index, err := h.market.ListMarketSkillsets(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": index})
}

// GetSkillset godoc
// @Summary      Get one market skillset
// @Description  The skillset detail view (zh/en metadata, bundled skill slugs,
// @Description  install state).
// @Tags         Experts
// @Produce      json
// @Param        slug  path  string  true  "Skillset slug"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  apperrors.AppError  "Unknown skillset"
// @Router       /experts/market/{slug} [get]
func (h *SkillMarketHandler) GetSkillset(c *gin.Context) {
	detail, err := h.market.GetMarketSkillset(c.Request.Context(), sandboxConfigTenantID(c), c.Param("slug"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": detail})
}

// InstallSkillset godoc
// @Summary      Install a market skillset as an expert
// @Description  Materializes the skillset as a workspace expert (manifest +
// @Description  persona + bundled skills) and instantiates it as a custom
// @Description  agent — the same one-action flow as POST /experts/:id/instantiate.
// @Description  agent_name (<= 255 chars) overrides the label,
// @Description  sandbox_config_id points skill scripts at a sandbox.
// @Tags         Experts
// @Accept       json
// @Produce      json
// @Param        slug     path  string                   true  "Skillset slug"
// @Param        request  body  expertInstantiateBody  true  "Optional overrides"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  apperrors.AppError
// @Failure      401  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError  "Unknown skillset"
// @Router       /experts/market/{slug}/install [post]
func (h *SkillMarketHandler) InstallSkillset(c *gin.Context) {
	// The tenant is the ctx-derived one, passed explicitly down — never
	// body-borne (the hand-off contract the expert instantiate handler set).
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok {
		_ = c.Error(apperrors.NewUnauthorizedError("missing workspace context"))
		return
	}

	req, err := decodeExpertInstantiateBody(c.Request.Body)
	if err != nil {
		_ = c.Error(apperrors.NewBadRequestError("invalid body: " + err.Error()))
		return
	}
	if utf8.RuneCountInString(req.AgentName) > maxExpertAgentNameLen {
		_ = c.Error(apperrors.NewBadRequestError("agent_name exceeds 255 characters"))
		return
	}

	result, err := h.market.InstallMarketSkillset(c.Request.Context(), tenantID, c.Param("slug"), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// The expert instantiate handler normalizes the pending/install arrays to
	// [] rather than omitted; mirror it (and keep the keys present) so both
	// install surfaces answer the same shape.
	data := gin.H{
		"agent":             nil,
		"pending_skills":    []string{},
		"skill_install_ids": []string{},
	}
	if result != nil {
		data["agent"] = result.Agent
		if result.PendingSkills != nil {
			data["pending_skills"] = result.PendingSkills
		}
		if result.SkillInstallIDs != nil {
			data["skill_install_ids"] = result.SkillInstallIDs
		}
		if result.ExpertID != "" {
			data["expert_id"] = result.ExpertID
		}
		if result.SnapshotSHA256 != "" {
			data["snapshot_sha256"] = result.SnapshotSHA256
		}
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": data})
}
