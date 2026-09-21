package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/persona"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// PersonaHandler serves the MBTI persona API: the 16-type catalog, rendering
// preview, the 28-question test, and applying/removing a persona on a custom
// agent. Catalog and test endpoints are read-only views over the embedded
// persona package; the agent-scoped endpoints go through CustomAgentService
// so tenant scoping and persistence match every other agent mutation.
type PersonaHandler struct {
	agentService interfaces.CustomAgentService
}

// NewPersonaHandler creates a new persona handler instance.
func NewPersonaHandler(agentService interfaces.CustomAgentService) *PersonaHandler {
	return &PersonaHandler{agentService: agentService}
}

// mbtiTypeResponse is the outward projection of persona.MBTIProfile: the same
// snake_case field set, with the four dimensions flattened to axis objects.
type mbtiTypeResponse struct {
	Code          string               `json:"code"`
	NameZh        string               `json:"name_zh"`
	NameEn        string               `json:"name_en"`
	NicknameZh    string               `json:"nickname_zh"`
	SummaryZh     string               `json:"summary_zh"`
	SummaryEn     string               `json:"summary_en"`
	DescriptorsZh string               `json:"descriptors_zh"`
	DescriptorsEn string               `json:"descriptors_en"`
	Dimensions    map[string]axisDTO   `json:"dimensions"`
	Behavior      persona.MBTIBehavior `json:"behavior"`
	Color         string               `json:"color"`
	Symbol        string               `json:"symbol"`
}

// axisDTO is one MBTI dimension: the dominant pole and its strength.
type axisDTO struct {
	Pole    string `json:"pole"`
	Percent int    `json:"percent"`
}

func toTypeResponse(p persona.MBTIProfile) mbtiTypeResponse {
	return mbtiTypeResponse{
		Code:          p.Code,
		NameZh:        p.NameZh,
		NameEn:        p.NameEn,
		NicknameZh:    p.NicknameZh,
		SummaryZh:     p.SummaryZh,
		SummaryEn:     p.SummaryEn,
		DescriptorsZh: p.DescriptorsZh,
		DescriptorsEn: p.DescriptorsEn,
		Dimensions: map[string]axisDTO{
			"ei": {Pole: p.Dimensions.EI.Pole, Percent: p.Dimensions.EI.Percent},
			"sn": {Pole: p.Dimensions.SN.Pole, Percent: p.Dimensions.SN.Percent},
			"tf": {Pole: p.Dimensions.TF.Pole, Percent: p.Dimensions.TF.Percent},
			"jp": {Pole: p.Dimensions.JP.Pole, Percent: p.Dimensions.JP.Percent},
		},
		Behavior: p.Behavior,
		Color:    p.Color,
		Symbol:   p.Symbol,
	}
}

// ListTypes godoc
// @Summary      List MBTI persona types
// @Description  All 16 built-in MBTI persona profiles.
// @Tags         Persona
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "MBTI type catalog"
// @Router       /mbti/types [get]
func (h *PersonaHandler) ListTypes(c *gin.Context) {
	types := make([]mbtiTypeResponse, 0, 16)
	for _, p := range persona.AllProfiles() {
		types = append(types, toTypeResponse(p))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"types": types}})
}

// GetType godoc
// @Summary      Get one MBTI persona type
// @Description  A single built-in MBTI persona profile.
// @Tags         Persona
// @Produce      json
// @Param        code  path  string  true  "MBTI code, e.g. INTJ"
// @Success      200   {object}  map[string]interface{}  "MBTI profile"
// @Failure      404   {object}  apperrors.AppError  "Unknown MBTI code"
// @Router       /mbti/types/{code} [get]
func (h *PersonaHandler) GetType(c *gin.Context) {
	p, ok := persona.Profile(strings.ToUpper(strings.TrimSpace(c.Param("code"))))
	if !ok {
		_ = c.Error(apperrors.NewNotFoundError("mbti type not found"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": toTypeResponse(p)})
}

// Preview godoc
// @Summary      Preview a rendered persona
// @Description  Renders the persona markdown for an MBTI code (locale-aware,
// @Description from the Accept-Language context). "_default" renders the
// @Description persona-free default template.
// @Tags         Persona
// @Produce      json
// @Param        code  path  string  true  "MBTI code or _default"
// @Success      200   {object}  map[string]interface{}  "Rendered persona markdown"
// @Router       /mbti/preview/{code} [get]
func (h *PersonaHandler) Preview(c *gin.Context) {
	code := strings.TrimSpace(c.Param("code"))
	locale := types.LanguageFromContextOrDefault(c.Request.Context())
	md := persona.RenderPersona(code, locale, persona.RenderInput{AgentName: "Agent"})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"code": code, "markdown": md}})
}

// TestQuestions godoc
// @Summary      List the MBTI test questions
// @Description  The 28 forced-choice A/B items. Both language variants ship
// @Description in each item; presentation-side filtering is the client's job.
// @Tags         Persona
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Test questions"
// @Router       /mbti/test/questions [get]
func (h *PersonaHandler) TestQuestions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"questions": persona.Questions()}})
}

// testSubmitBody is the POST /mbti/test/submit request: question id → "A"/"B".
type testSubmitBody struct {
	Answers map[string]string `json:"answers" binding:"required"`
}

// TestSubmit godoc
// @Summary      Score submitted MBTI test answers
// @Description  Scores at least 20 valid answers into the four axis results,
// @Description the derived MBTI code, and the matching profile.
// @Tags         Persona
// @Accept       json
// @Produce      json
// @Param        request  body  testSubmitBody  true  "Question id → A/B answers"
// @Success      200  {object}  map[string]interface{}  "Score result"
// @Failure      400  {object}  apperrors.AppError  "Too few or invalid answers"
// @Router       /mbti/test/submit [post]
func (h *PersonaHandler) TestSubmit(c *gin.Context) {
	var body testSubmitBody
	if err := c.ShouldBindJSON(&body); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("invalid body: " + err.Error()))
		return
	}
	score, err := persona.ScoreAnswers(body.Answers)
	if err != nil {
		_ = c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	profile, _ := persona.Profile(score.Code)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"code":       score.Code,
		"dimensions": gin.H{"ei": score.EI, "sn": score.SN, "tf": score.TF, "jp": score.JP},
		"profile":    toTypeResponse(profile),
	}})
}

// personaApplyBody is the PUT /agents/:id/persona request.
type personaApplyBody struct {
	Code  string `json:"code" binding:"required"`
	Style string `json:"style"`
}

// ApplyPersona godoc
// @Summary      Apply an MBTI persona to an agent
// @Description  Sets the agent's persona_mbti/persona_style config fields.
// @Description  The caller must own the agent (or be Admin+), matching the
// @Description  agent-update guard.
// @Tags         Persona
// @Accept       json
// @Produce      json
// @Param        id       path  string             true  "Agent ID"
// @Param        request  body  personaApplyBody   true  "MBTI code and optional style"
// @Success      200  {object}  map[string]interface{}  "Applied persona fields"
// @Failure      400  {object}  apperrors.AppError  "Invalid body or unknown MBTI code"
// @Failure      404  {object}  apperrors.AppError  "Agent not found"
// @Router       /agents/{id}/persona [put]
func (h *PersonaHandler) ApplyPersona(c *gin.Context) {
	var body personaApplyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("invalid body: " + err.Error()))
		return
	}
	code := strings.ToUpper(strings.TrimSpace(body.Code))
	if _, ok := persona.Profile(code); !ok {
		_ = c.Error(apperrors.NewBadRequestError("unknown mbti code: " + code))
		return
	}
	agent, err := h.agentService.GetAgentByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(personaAgentServiceError(err))
		return
	}
	if agent == nil {
		_ = c.Error(apperrors.NewNotFoundError("agent not found"))
		return
	}
	agent.Config.PersonaMBTI = code
	agent.Config.PersonaStyle = strings.TrimSpace(body.Style)
	updated, err := h.agentService.UpdateAgent(c.Request.Context(), agent)
	if err != nil {
		_ = c.Error(personaAgentServiceError(err))
		return
	}
	if updated == nil {
		updated = agent
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"persona_mbti":  updated.Config.PersonaMBTI,
		"persona_style": updated.Config.PersonaStyle,
	}})
}

// RemovePersona godoc
// @Summary      Remove an agent's MBTI persona
// @Description  Clears the agent's persona_mbti/persona_style config fields.
// @Description  Same ownership guard as agent updates.
// @Tags         Persona
// @Produce      json
// @Param        id  path  string  true  "Agent ID"
// @Success      200  {object}  map[string]interface{}  "Cleared persona fields"
// @Failure      404  {object}  apperrors.AppError  "Agent not found"
// @Router       /agents/{id}/persona [delete]
func (h *PersonaHandler) RemovePersona(c *gin.Context) {
	agent, err := h.agentService.GetAgentByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(personaAgentServiceError(err))
		return
	}
	if agent == nil {
		_ = c.Error(apperrors.NewNotFoundError("agent not found"))
		return
	}
	agent.Config.PersonaMBTI = ""
	agent.Config.PersonaStyle = ""
	if _, err := h.agentService.UpdateAgent(c.Request.Context(), agent); err != nil {
		_ = c.Error(personaAgentServiceError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"persona_mbti": "", "persona_style": ""}})
}

// personaAgentServiceError maps CustomAgentService rejections onto the HTTP
// error taxonomy, following the CustomAgentHandler convention: the
// not-found sentinel becomes a 404, AppErrors keep their status, and
// anything else is an internal error.
func personaAgentServiceError(err error) error {
	if err == service.ErrAgentNotFound {
		return apperrors.NewNotFoundError("agent not found")
	}
	if appErr, ok := err.(*apperrors.AppError); ok {
		return appErr
	}
	return apperrors.NewInternalServerError(err.Error())
}
