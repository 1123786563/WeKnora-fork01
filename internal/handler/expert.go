package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/experts"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// maxExpertAgentNameLen mirrors the CustomAgent Name column cap
// (varchar(255)); rejecting earlier gives a clearer 400 than a service or
// database failure after skills were already resolved.
const maxExpertAgentNameLen = 255

// ExpertHandler serves the expert-template library API: the catalog of
// instantiable expert templates and the one-action instantiation that maps a
// template onto a tenant-owned custom agent via ExpertService (which itself
// delegates to CustomAgentService, so tenant scoping and persistence match
// every other agent-creation path).
type ExpertHandler struct {
	expertService interfaces.ExpertService
}

// NewExpertHandler creates a new expert handler instance.
func NewExpertHandler(expertService interfaces.ExpertService) *ExpertHandler {
	return &ExpertHandler{expertService: expertService}
}

// expertSummaryDTO is the list projection of one expert: the manifest fields
// a gallery card needs, with label/description resolved at the request
// locale (the same rule Instantiate applies, service.ResolveExpertLocaleText).
type expertSummaryDTO struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Description      string   `json:"description"`
	IconName         string   `json:"icon_name"`
	Color            string   `json:"color"`
	PersonaMBTI      string   `json:"persona_mbti"`
	QuickPromptCount int      `json:"quick_prompt_count"`
	Skills           []string `json:"skills"`
}

// expertQuickPromptDTO is one locale-resolved quick prompt of the detail view.
type expertQuickPromptDTO struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Color       string `json:"color"`
	IconName    string `json:"icon_name"`
}

// expertDetailDTO is the detail projection: the summary fields plus the
// persona markdown preview (prompt files joined with the same separator
// Instantiate uses) and the full locale-resolved quick prompts.
type expertDetailDTO struct {
	expertSummaryDTO
	PersonaMarkdown string                 `json:"persona_markdown"`
	QuickPrompts    []expertQuickPromptDTO `json:"quick_prompts"`
}

func expertSummary(e *experts.Expert, locale string) expertSummaryDTO {
	m := e.Manifest
	skills := make([]string, 0, len(m.Skills))
	skills = append(skills, m.Skills...)
	return expertSummaryDTO{
		ID:               m.ID,
		Label:            service.ResolveExpertLocaleText(m.Label, locale),
		Description:      service.ResolveExpertLocaleText(m.Description, locale),
		IconName:         m.IconName,
		Color:            m.Color,
		PersonaMBTI:      m.PersonaMBTI,
		QuickPromptCount: len(m.QuickPrompts),
		Skills:           skills,
	}
}

// expertPersonaMarkdown joins the expert's persona documents in manifest
// order with the instantiate separator. Files missing from the scanned
// package are skipped — the same rule buildAgentFromExpert applies — and an
// expert with no resolvable persona yields an empty preview.
func expertPersonaMarkdown(e *experts.Expert) string {
	parts := make([]string, 0, len(e.Manifest.PromptFiles))
	for _, file := range e.Manifest.PromptFiles {
		if content, ok := e.PersonaFiles[file]; ok {
			parts = append(parts, string(content))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, service.ExpertPersonaFileSeparator)
}

// ListExperts godoc
// @Summary      List expert templates
// @Description  The built-in expert-template catalog with locale-resolved
// @Description  labels and descriptions.
// @Tags         Experts
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Expert summaries"
// @Router       /experts [get]
func (h *ExpertHandler) ListExperts(c *gin.Context) {
	list, err := h.expertService.ListExperts(c.Request.Context())
	if err != nil {
		_ = c.Error(apperrors.NewInternalServerError("failed to list experts: " + err.Error()))
		return
	}
	locale := types.LanguageFromContextOrDefault(c.Request.Context())
	summaries := make([]expertSummaryDTO, 0, len(list))
	for _, e := range list {
		if e == nil {
			continue
		}
		summaries = append(summaries, expertSummary(e, locale))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"experts": summaries}})
}

// GetExpert godoc
// @Summary      Get one expert template
// @Description  Detail view of an expert: summary fields, the joined persona
// @Description  markdown preview, and the locale-resolved quick prompts.
// @Tags         Experts
// @Produce      json
// @Param        id  path  string  true  "Expert ID"
// @Success      200  {object}  map[string]interface{}  "Expert detail"
// @Failure      404  {object}  apperrors.AppError  "Unknown expert"
// @Router       /experts/{id} [get]
func (h *ExpertHandler) GetExpert(c *gin.Context) {
	e, err := h.expertService.GetExpert(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(expertServiceError(err))
		return
	}
	locale := types.LanguageFromContextOrDefault(c.Request.Context())
	dto := expertDetailDTO{
		expertSummaryDTO: expertSummary(e, locale),
		PersonaMarkdown:  expertPersonaMarkdown(e),
	}
	dto.QuickPrompts = make([]expertQuickPromptDTO, 0, len(e.Manifest.QuickPrompts))
	for _, qp := range e.Manifest.QuickPrompts {
		dto.QuickPrompts = append(dto.QuickPrompts, expertQuickPromptDTO{
			Title:       service.ResolveExpertLocaleText(qp.Title, locale),
			Description: service.ResolveExpertLocaleText(qp.Description, locale),
			Prompt:      service.ResolveExpertLocaleText(qp.Prompt, locale),
			Color:       qp.Color,
			IconName:    qp.IconName,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": dto})
}

// expertInstantiateBody is the POST /experts/:id/instantiate request. Every
// field is optional; blank strings are treated as absent. Decoding is strict
// (decodeExpertInstantiateBody): the two overrides are the only
// client-supplied shape — the tenant and every provenance field are derived
// server-side — so any additional field is rejected with a 400.
type expertInstantiateBody struct {
	AgentName       string `json:"agent_name"`
	SandboxConfigID string `json:"sandbox_config_id"`
}

// decodeExpertInstantiateBody strictly decodes one instantiate request,
// following the repo's DisallowUnknownFields pattern (DecodeOCPrepare in
// app_connector_oc.go): unknown fields are rejected, exactly one JSON value
// is allowed (trailing input is an error), and an empty body means
// "no overrides".
func decodeExpertInstantiateBody(r io.Reader) (interfaces.InstantiateRequest, error) {
	var body expertInstantiateBody
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			// Empty body: every field is optional, so this is a plain
			// no-override instantiation.
			return interfaces.InstantiateRequest{}, nil
		}
		return interfaces.InstantiateRequest{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return interfaces.InstantiateRequest{}, errors.New("trailing input")
	}
	return interfaces.InstantiateRequest{
		AgentName:       strings.TrimSpace(body.AgentName),
		SandboxConfigID: strings.TrimSpace(body.SandboxConfigID),
	}, nil
}

// Instantiate godoc
// @Summary      Instantiate an expert as a custom agent
// @Description  Creates a tenant-owned custom agent from an expert template.
// @Description  The tenant is derived from the authenticated context, never
// @Description  the request body. Locale-sensitive fields resolve from the
// @Description  request locale; agent_name (<= 255 chars) overrides the
// @Description  label, sandbox_config_id points skill scripts at a sandbox.
// @Tags         Experts
// @Accept       json
// @Produce      json
// @Param        id       path  string                   true  "Expert ID"
// @Param        request  body  expertInstantiateBody  true  "Optional overrides"
// @Success      201  {object}  map[string]interface{}  "Created agent + skill install state"
// @Failure      400  {object}  apperrors.AppError  "Invalid body or oversized agent_name"
// @Failure      404  {object}  apperrors.AppError  "Unknown expert"
// @Router       /experts/{id}/instantiate [post]
func (h *ExpertHandler) Instantiate(c *gin.Context) {
	// The hand-off contract from the service reviews: the tenant is the
	// ctx-derived one, passed explicitly to Instantiate — never body-borne.
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

	result, err := h.expertService.Instantiate(c.Request.Context(), tenantID, c.Param("id"), req)
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

// expertServiceError maps ExpertService rejections onto the HTTP error
// taxonomy, following the persona handler convention: the not-found sentinel
// (also wrapped by Instantiate) becomes a 404, wrapped AppErrors keep their
// status, and anything else is an internal error.
func expertServiceError(err error) error {
	if errors.Is(err, service.ErrExpertNotFound) {
		return apperrors.NewNotFoundError("expert not found")
	}
	if errors.Is(err, service.ErrAgentNameRequired) {
		return apperrors.NewBadRequestError(err.Error())
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return apperrors.NewInternalServerError(err.Error())
}
