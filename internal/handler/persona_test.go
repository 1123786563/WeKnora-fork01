package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// fakePersonaAgentService stands in for interfaces.CustomAgentService. Only
// the two methods the persona endpoints use are implemented; the rest panic
// via the embedded nil interface if reached.
type fakePersonaAgentService struct {
	interfaces.CustomAgentService

	// agents is the ID-addressed store GetAgentByID reads.
	agents map[string]*types.CustomAgent
	// updated records every agent passed to UpdateAgent, in call order.
	updated []*types.CustomAgent
}

func (s *fakePersonaAgentService) GetAgentByID(_ context.Context, id string) (*types.CustomAgent, error) {
	if a, ok := s.agents[id]; ok {
		return a, nil
	}
	return nil, service.ErrAgentNotFound
}

func (s *fakePersonaAgentService) UpdateAgent(_ context.Context, agent *types.CustomAgent) (*types.CustomAgent, error) {
	s.updated = append(s.updated, agent)
	return agent, nil
}

// newPersonaTestRouter builds a gin engine mirroring the production persona
// route tree at /api/v1. The plain PUT /agents/:id stub reproduces the
// existing agentsWrite registration from routes_agent.go: registering the
// persona sub-routes beside it with the same wildcard name proves gin does
// not hit a wildcard conflict (the reason the production routes reuse :id).
func newPersonaTestRouter(t *testing.T) (*gin.Engine, *fakePersonaAgentService) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	svc := &fakePersonaAgentService{
		agents: map[string]*types.CustomAgent{
			"a1": {ID: "a1", TenantID: 1, Name: "Agent One"},
		},
	}
	h := NewPersonaHandler(svc)

	r := gin.New()
	r.Use(middleware.ErrorHandler())

	v1 := r.Group("/api/v1")
	mbti := v1.Group("/mbti")
	{
		mbti.GET("/types", h.ListTypes)
		mbti.GET("/types/:code", h.GetType)
		mbti.GET("/preview/:code", h.Preview)
		mbti.GET("/test/questions", h.TestQuestions)
		mbti.POST("/test/submit", h.TestSubmit)
	}
	agents := v1.Group("/agents")
	{
		// Stand-in for routes_agent.go's PUT /agents/:id (agentsWrite).
		agents.PUT("/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
		agents.PUT("/:id/persona", h.ApplyPersona)
		agents.DELETE("/:id/persona", h.RemovePersona)
	}
	return r, svc
}

func personaDo(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestMBTITypesList covers brief Step 1.1: 16 items, success=true.
func TestMBTITypesList(t *testing.T) {
	r, _ := newPersonaTestRouter(t)
	w := personaDo(t, r, http.MethodGet, "/api/v1/mbti/types", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Types []json.RawMessage `json:"types"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatalf("success = false, body %s", w.Body.String())
	}
	if len(body.Data.Types) != 16 {
		t.Fatalf("want 16 types, got %d", len(body.Data.Types))
	}
}

// TestMBTITypeDetail covers brief Step 1.2: INTJ nickname, unknown code 404.
func TestMBTITypeDetail(t *testing.T) {
	r, _ := newPersonaTestRouter(t)

	w := personaDo(t, r, http.MethodGet, "/api/v1/mbti/types/INTJ", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Code       string `json:"code"`
			NicknameZh string `json:"nickname_zh"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Code != "INTJ" {
		t.Fatalf("code = %q", body.Data.Code)
	}
	if body.Data.NicknameZh != "紫老头" {
		t.Fatalf("nickname_zh = %q, want 紫老头", body.Data.NicknameZh)
	}

	w = personaDo(t, r, http.MethodGet, "/api/v1/mbti/types/XXYY", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("XXYY status = %d, want 404, body %s", w.Code, w.Body.String())
	}
}

// TestMBTIPreview covers brief Step 1.3: profile header and _default fallback.
func TestMBTIPreview(t *testing.T) {
	r, _ := newPersonaTestRouter(t)

	w := personaDo(t, r, http.MethodGet, "/api/v1/mbti/preview/INTJ", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Code     string `json:"code"`
			Markdown string `json:"markdown"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Code != "INTJ" {
		t.Fatalf("code = %q", body.Data.Code)
	}
	if !strings.HasPrefix(body.Data.Markdown, "# Persona: INTJ") {
		t.Fatalf("markdown = %q, want prefix %q", body.Data.Markdown, "# Persona: INTJ")
	}

	w = personaDo(t, r, http.MethodGet, "/api/v1/mbti/preview/_default", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("_default status = %d body %s", w.Code, w.Body.String())
	}
	body.Data.Code = ""
	body.Data.Markdown = ""
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(body.Data.Markdown, "# Persona: Default") {
		t.Fatalf("_default markdown = %q, want prefix %q", body.Data.Markdown, "# Persona: Default")
	}
}

// TestMBTITestQuestions pins the shipped question count (28) and payload shape.
func TestMBTITestQuestions(t *testing.T) {
	r, _ := newPersonaTestRouter(t)
	w := personaDo(t, r, http.MethodGet, "/api/v1/mbti/test/questions", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Questions []json.RawMessage `json:"questions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Questions) != 28 {
		t.Fatalf("want 28 questions, got %d", len(body.Data.Questions))
	}
}

// TestMBTISubmitScores covers brief Step 1.4: 28 A answers score ESTJ with
// every axis at 85; 2 answers are rejected with 400.
func TestMBTISubmitScores(t *testing.T) {
	r, _ := newPersonaTestRouter(t)

	answers := map[string]string{}
	for i := 1; i <= 28; i++ {
		answers[strconv.Itoa(i)] = "A"
	}
	w := personaDo(t, r, http.MethodPost, "/api/v1/mbti/test/submit", map[string]any{"answers": answers})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Code       string `json:"code"`
			Dimensions struct {
				EI struct {
					Dominant string `json:"dominant"`
					Percent  int    `json:"percent"`
				} `json:"ei"`
				SN struct {
					Dominant string `json:"dominant"`
					Percent  int    `json:"percent"`
				} `json:"sn"`
				TF struct {
					Dominant string `json:"dominant"`
					Percent  int    `json:"percent"`
				} `json:"tf"`
				JP struct {
					Dominant string `json:"dominant"`
					Percent  int    `json:"percent"`
				} `json:"jp"`
			} `json:"dimensions"`
			Profile struct {
				Code string `json:"code"`
			} `json:"profile"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Code) != 4 {
		t.Fatalf("code = %q, want 4 chars", body.Data.Code)
	}
	if body.Data.Code != "ESTJ" {
		t.Fatalf("code = %q, want ESTJ (28 A answers)", body.Data.Code)
	}
	if body.Data.Dimensions.EI.Dominant != "E" || body.Data.Dimensions.EI.Percent != 85 {
		t.Fatalf("ei = %+v, want E/85", body.Data.Dimensions.EI)
	}
	if body.Data.Dimensions.SN.Percent != 85 || body.Data.Dimensions.TF.Percent != 85 || body.Data.Dimensions.JP.Percent != 85 {
		t.Fatalf("dimensions = %+v, want all percent 85", body.Data.Dimensions)
	}
	if body.Data.Profile.Code != "ESTJ" {
		t.Fatalf("profile.code = %q, want ESTJ", body.Data.Profile.Code)
	}

	w = personaDo(t, r, http.MethodPost, "/api/v1/mbti/test/submit",
		map[string]any{"answers": map[string]string{"1": "A", "2": "A"}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("2-answers status = %d, want 400, body %s", w.Code, w.Body.String())
	}
}

// TestMBTIApplyPersona covers brief Step 1.5: invalid code 400, unknown agent
// 404, valid apply persists PersonaMBTI/PersonaStyle via UpdateAgent.
func TestMBTIApplyPersona(t *testing.T) {
	r, svc := newPersonaTestRouter(t)

	// Invalid code → 400, service untouched.
	w := personaDo(t, r, http.MethodPut, "/api/v1/agents/a1/persona",
		map[string]any{"code": "ZZZZ", "style": ""})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid code status = %d, want 400, body %s", w.Code, w.Body.String())
	}
	if len(svc.updated) != 0 {
		t.Fatalf("UpdateAgent must not be called for an invalid code, got %d calls", len(svc.updated))
	}

	// Unknown agent → 404.
	w = personaDo(t, r, http.MethodPut, "/api/v1/agents/nope/persona",
		map[string]any{"code": "INTJ", "style": ""})
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown agent status = %d, want 404, body %s", w.Code, w.Body.String())
	}
	if len(svc.updated) != 0 {
		t.Fatalf("UpdateAgent must not be called for an unknown agent, got %d calls", len(svc.updated))
	}

	// Valid apply → 200, fields persisted, echoed back.
	w = personaDo(t, r, http.MethodPut, "/api/v1/agents/a1/persona",
		map[string]any{"code": "intj", "style": " be concise "})
	if w.Code != http.StatusOK {
		t.Fatalf("valid apply status = %d, want 200, body %s", w.Code, w.Body.String())
	}
	if len(svc.updated) != 1 {
		t.Fatalf("UpdateAgent called %d times, want 1", len(svc.updated))
	}
	applied := svc.updated[0]
	if applied.Config.PersonaMBTI != "INTJ" {
		t.Fatalf("PersonaMBTI = %q, want INTJ (normalized to upper)", applied.Config.PersonaMBTI)
	}
	if applied.Config.PersonaStyle != "be concise" {
		t.Fatalf("PersonaStyle = %q, want trimmed %q", applied.Config.PersonaStyle, "be concise")
	}
	var body struct {
		Data struct {
			PersonaMBTI  string `json:"persona_mbti"`
			PersonaStyle string `json:"persona_style"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.PersonaMBTI != "INTJ" || body.Data.PersonaStyle != "be concise" {
		t.Fatalf("echo = %+v", body.Data)
	}
}

// TestMBTIRemovePersona covers brief Step 1.6: DELETE clears both fields.
func TestMBTIRemovePersona(t *testing.T) {
	r, svc := newPersonaTestRouter(t)
	svc.agents["a1"].Config.PersonaMBTI = "INTJ"
	svc.agents["a1"].Config.PersonaStyle = "be concise"

	w := personaDo(t, r, http.MethodDelete, "/api/v1/agents/a1/persona", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if len(svc.updated) != 1 {
		t.Fatalf("UpdateAgent called %d times, want 1", len(svc.updated))
	}
	cleared := svc.updated[0]
	if cleared.Config.PersonaMBTI != "" || cleared.Config.PersonaStyle != "" {
		t.Fatalf("persona fields not cleared: %+v", cleared.Config)
	}
	var body struct {
		Data struct {
			PersonaMBTI  string `json:"persona_mbti"`
			PersonaStyle string `json:"persona_style"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.PersonaMBTI != "" || body.Data.PersonaStyle != "" {
		t.Fatalf("echo = %+v, want empty persona fields", body.Data)
	}
}
