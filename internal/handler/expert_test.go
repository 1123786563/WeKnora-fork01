package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/agent/experts"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// fixtureExpertCatalog mirrors the shape of the shipped pilot experts: one
// rich expert (locale text, two persona files, quick prompts, one bundled
// skill) plus three minimal ones, so list/detail projections exercise both
// the populated and the empty-field paths.
func fixtureExpertCatalog() []*experts.Expert {
	quickPrompts := make([]experts.QuickPrompt, 0, 2)
	for i := 0; i < 2; i++ {
		quickPrompts = append(quickPrompts, experts.QuickPrompt{
			Title:       experts.LocaleText{"zh": "标题", "en": "Title"},
			Description: experts.LocaleText{"zh": "描述", "en": "Description"},
			Prompt: experts.LocaleText{
				"zh": "中文快捷指令 " + string(rune('A'+i)),
				"en": "english quick prompt " + string(rune('A'+i)),
			},
			Color:    "#e8f4ff",
			IconName: "line-chart",
		})
	}
	rich := &experts.Expert{
		Manifest: experts.ExpertManifest{
			ID:           "stock-assistant",
			Label:        experts.LocaleText{"zh": "老钱 · 证券观察员", "en": "Lao Qian · Market Observer"},
			Description:  experts.LocaleText{"zh": "市场观察者", "en": "Market observer"},
			IconName:     "candlestick-chart",
			Color:        "#e74c3c",
			PersonaMBTI:  "ISTP",
			PromptFiles:  []string{"SOUL.md", "IDENTITY.md"},
			QuickPrompts: quickPrompts,
			Skills:       []string{"stock-info"},
		},
		PersonaFiles: map[string][]byte{
			"SOUL.md":     []byte("SOUL persona content"),
			"IDENTITY.md": []byte("IDENTITY persona content"),
		},
		SkillDirs: map[string]string{"stock-info": "/experts/stock-assistant/skills/stock-info"},
	}
	minimal := func(id, label string) *experts.Expert {
		return &experts.Expert{
			Manifest: experts.ExpertManifest{
				ID:          id,
				Label:       experts.LocaleText{"zh": label, "en": label + " EN"},
				Description: experts.LocaleText{"zh": label + " 描述", "en": label + " description"},
				IconName:    "sparkles",
				Color:       "#3b82f6",
			},
		}
	}
	return []*experts.Expert{
		rich,
		minimal("travel-planner", "行程规划师"),
		minimal("data-analyst", "数据分析师"),
		minimal("meeting-recorder", "会议记录员"),
	}
}

// fakeInstantiateCall records one Instantiate invocation with exactly the
// arguments the handler passed (tenantID derivation included).
type fakeInstantiateCall struct {
	tenantID uint64
	expertID string
	req      interfaces.InstantiateRequest
}

// fakeExpertService stands in for interfaces.ExpertService: a static catalog
// plus a recording Instantiate that fabricates the agent the way the real
// service does (ID + ExpertSource provenance).
type fakeExpertService struct {
	catalog []*experts.Expert

	calls   []fakeInstantiateCall
	crafted *types.CustomAgent
}

var _ interfaces.ExpertService = (*fakeExpertService)(nil)

func (s *fakeExpertService) ListExperts(context.Context) ([]*experts.Expert, error) {
	return s.catalog, nil
}

func (s *fakeExpertService) GetExpert(_ context.Context, id string) (*experts.Expert, error) {
	for _, e := range s.catalog {
		if e != nil && e.Manifest.ID == id {
			return e, nil
		}
	}
	return nil, service.ErrExpertNotFound
}

func (s *fakeExpertService) Instantiate(
	_ context.Context, tenantID uint64, expertID string, req interfaces.InstantiateRequest,
) (*interfaces.InstantiateResult, error) {
	if _, err := s.GetExpert(context.Background(), expertID); err != nil {
		return nil, err
	}
	s.calls = append(s.calls, fakeInstantiateCall{tenantID: tenantID, expertID: expertID, req: req})
	agent := &types.CustomAgent{
		ID:       "agent-9",
		TenantID: tenantID,
		Config:   types.CustomAgentConfig{},
	}
	agent.Config.ExpertSource = &types.ExpertSourceStruct{ExpertID: expertID, Source: "builtin"}
	if s.crafted != nil {
		agent = s.crafted
	}
	return &interfaces.InstantiateResult{
		Agent:           agent,
		PendingSkills:   []string{"web-search"},
		SkillInstallIDs: []string{"install-1"},
	}, nil
}

// newExpertTestRouter builds a gin engine mirroring the production expert
// route tree at /api/v1/experts (guards omitted — handler behaviour only).
func newExpertTestRouter(t *testing.T) (*gin.Engine, *fakeExpertService) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	svc := &fakeExpertService{catalog: fixtureExpertCatalog()}
	h := NewExpertHandler(svc)

	r := gin.New()
	r.Use(middleware.ErrorHandler())

	v1 := r.Group("/api/v1")
	expertGroup := v1.Group("/experts")
	{
		expertGroup.GET("", h.ListExperts)
		expertGroup.GET("/:id", h.GetExpert)
		expertGroup.POST("/:id/instantiate", h.Instantiate)
	}
	return r, svc
}

// expertDo issues one request with the locale and tenant the auth/language
// middlewares would have attached to the request context.
func expertDo(t *testing.T, r *gin.Engine, method, path string, body any, locale string, tenantID uint64) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		if raw, ok := body.(string); ok {
			reader = bytes.NewReader([]byte(raw))
		} else {
			payload, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			reader = bytes.NewReader(payload)
		}
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := context.WithValue(req.Context(), types.LanguageContextKey, locale)
	if tenantID != 0 {
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	}
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestExpertListSummaries covers the list route: 4 experts from the fixture
// catalog, each with the summary field set and label/description resolved at
// the request locale.
func TestExpertListSummaries(t *testing.T) {
	r, _ := newExpertTestRouter(t)

	w := expertDo(t, r, http.MethodGet, "/api/v1/experts", nil, "zh-CN", 7)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Experts []struct {
				ID               string   `json:"id"`
				Label            string   `json:"label"`
				Description      string   `json:"description"`
				IconName         string   `json:"icon_name"`
				Color            string   `json:"color"`
				PersonaMBTI      string   `json:"persona_mbti"`
				QuickPromptCount int      `json:"quick_prompt_count"`
				Skills           []string `json:"skills"`
			} `json:"experts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatalf("success = false, body %s", w.Body.String())
	}
	if len(body.Data.Experts) != 4 {
		t.Fatalf("want 4 experts, got %d", len(body.Data.Experts))
	}
	first := body.Data.Experts[0]
	if first.ID != "stock-assistant" || first.Label != "老钱 · 证券观察员" || first.Description != "市场观察者" {
		t.Fatalf("first summary = %+v, want zh-resolved stock-assistant", first)
	}
	if first.IconName != "candlestick-chart" || first.Color != "#e74c3c" || first.PersonaMBTI != "ISTP" {
		t.Fatalf("first summary = %+v", first)
	}
	if first.QuickPromptCount != 2 {
		t.Fatalf("quick_prompt_count = %d, want 2", first.QuickPromptCount)
	}
	if len(first.Skills) != 1 || first.Skills[0] != "stock-info" {
		t.Fatalf("skills = %v, want [stock-info]", first.Skills)
	}
	// Minimal expert: empty fields must serialize as zero values / [], not null.
	last := body.Data.Experts[3]
	if last.ID != "meeting-recorder" || last.QuickPromptCount != 0 {
		t.Fatalf("last summary = %+v", last)
	}
	if last.Skills == nil || len(last.Skills) != 0 {
		t.Fatalf("skills = %#v, want non-nil empty", last.Skills)
	}
}

// TestExpertListLocaleResolution pins that the same catalog resolves english
// strings under an en-US request locale.
func TestExpertListLocaleResolution(t *testing.T) {
	r, _ := newExpertTestRouter(t)

	w := expertDo(t, r, http.MethodGet, "/api/v1/experts", nil, "en-US", 7)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Experts []struct {
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"experts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Experts[0].Label != "Lao Qian · Market Observer" {
		t.Fatalf("label = %q, want en label", body.Data.Experts[0].Label)
	}
	if body.Data.Experts[0].Description != "Market observer" {
		t.Fatalf("description = %q, want en description", body.Data.Experts[0].Description)
	}
}

// TestExpertDetail covers the detail route: summary fields plus the joined
// persona markdown and locale-resolved quick prompts; unknown id 404s.
func TestExpertDetail(t *testing.T) {
	r, _ := newExpertTestRouter(t)

	w := expertDo(t, r, http.MethodGet, "/api/v1/experts/stock-assistant", nil, "zh-CN", 7)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			ID           string `json:"id"`
			Label        string `json:"label"`
			PersonaMBTI  string `json:"persona_mbti"`
			PersonaMD    string `json:"persona_markdown"`
			QuickPrompts []struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				Prompt      string `json:"prompt"`
				Color       string `json:"color"`
				IconName    string `json:"icon_name"`
			} `json:"quick_prompts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success || body.Data.ID != "stock-assistant" || body.Data.Label != "老钱 · 证券观察员" {
		t.Fatalf("detail header = %+v", body.Data)
	}
	if body.Data.PersonaMBTI != "ISTP" {
		t.Fatalf("persona_mbti = %q", body.Data.PersonaMBTI)
	}
	wantMD := "SOUL persona content\n\n---\n\nIDENTITY persona content"
	if body.Data.PersonaMD != wantMD {
		t.Fatalf("persona_markdown = %q, want %q", body.Data.PersonaMD, wantMD)
	}
	if len(body.Data.QuickPrompts) != 2 {
		t.Fatalf("quick_prompts = %d items, want 2", len(body.Data.QuickPrompts))
	}
	qp := body.Data.QuickPrompts[0]
	if qp.Title != "标题" || qp.Description != "描述" || qp.Prompt != "中文快捷指令 A" {
		t.Fatalf("quick prompt = %+v, want zh-resolved", qp)
	}
	if qp.Color != "#e8f4ff" || qp.IconName != "line-chart" {
		t.Fatalf("quick prompt = %+v", qp)
	}

	// Unknown expert → 404.
	w = expertDo(t, r, http.MethodGet, "/api/v1/experts/nope", nil, "zh-CN", 7)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown expert status = %d, want 404, body %s", w.Code, w.Body.String())
	}
}

// TestExpertInstantiateHappy covers the happy path: 201 with the created
// agent JSON (same shape ListAgents returns), pending skills and install
// ids, and — the T3/T4 hand-off constraint — the tenant derived from the
// request context (never the body) passed explicitly to Instantiate.
func TestExpertInstantiateHappy(t *testing.T) {
	r, svc := newExpertTestRouter(t)

	// Empty body: every field is optional and must be treated as absent.
	w := expertDo(t, r, http.MethodPost, "/api/v1/experts/stock-assistant/instantiate", nil, "zh-CN", 7)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Agent struct {
				ID     string `json:"id"`
				Config struct {
					ExpertSource struct {
						ExpertID string `json:"expert_id"`
						Source   string `json:"source"`
					} `json:"expert_source"`
				} `json:"config"`
			} `json:"agent"`
			PendingSkills   []string `json:"pending_skills"`
			SkillInstallIDs []string `json:"skill_install_ids"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatalf("success = false, body %s", w.Body.String())
	}
	if body.Data.Agent.ID != "agent-9" {
		t.Fatalf("agent.id = %q, want agent-9", body.Data.Agent.ID)
	}
	if body.Data.Agent.Config.ExpertSource.ExpertID != "stock-assistant" ||
		body.Data.Agent.Config.ExpertSource.Source != "builtin" {
		t.Fatalf("agent expert_source = %+v", body.Data.Agent.Config.ExpertSource)
	}
	if len(body.Data.PendingSkills) != 1 || body.Data.PendingSkills[0] != "web-search" {
		t.Fatalf("pending_skills = %v", body.Data.PendingSkills)
	}
	if len(body.Data.SkillInstallIDs) != 1 || body.Data.SkillInstallIDs[0] != "install-1" {
		t.Fatalf("skill_install_ids = %v", body.Data.SkillInstallIDs)
	}
	if len(svc.calls) != 1 {
		t.Fatalf("Instantiate called %d times, want 1", len(svc.calls))
	}
	call := svc.calls[0]
	if call.tenantID != 7 {
		t.Fatalf("Instantiate tenantID = %d, want 7 (ctx-derived)", call.tenantID)
	}
	if call.expertID != "stock-assistant" {
		t.Fatalf("Instantiate expertID = %q, want path id", call.expertID)
	}
	if call.req.AgentName != "" || call.req.SandboxConfigID != "" {
		t.Fatalf("empty body must map to absent fields, got %+v", call.req)
	}

	// Body overrides are passed through (trimmed).
	svc.calls = nil
	w = expertDo(t, r, http.MethodPost, "/api/v1/experts/stock-assistant/instantiate",
		map[string]any{"agent_name": " 我的老钱 ", "sandbox_config_id": "sandbox-9"}, "zh-CN", 7)
	if w.Code != http.StatusCreated {
		t.Fatalf("override status %d body %s", w.Code, w.Body.String())
	}
	if len(svc.calls) != 1 {
		t.Fatalf("Instantiate called %d times, want 1", len(svc.calls))
	}
	call = svc.calls[0]
	if call.req.AgentName != "我的老钱" {
		t.Fatalf("AgentName = %q, want trimmed override", call.req.AgentName)
	}
	if call.req.SandboxConfigID != "sandbox-9" {
		t.Fatalf("SandboxConfigID = %q, want sandbox-9", call.req.SandboxConfigID)
	}
}

// TestExpertInstantiateValidation covers the 400/401/404 rejection paths;
// none of them may reach the service.
func TestExpertInstantiateValidation(t *testing.T) {
	r, svc := newExpertTestRouter(t)

	// Malformed JSON → 400.
	w := expertDo(t, r, http.MethodPost, "/api/v1/experts/stock-assistant/instantiate", "{", "zh-CN", 7)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed body status = %d, want 400, body %s", w.Code, w.Body.String())
	}

	// Oversized agent_name (> 255 chars, the varchar(255) column cap) → 400.
	long := strings.Repeat("名", 256)
	w = expertDo(t, r, http.MethodPost, "/api/v1/experts/stock-assistant/instantiate",
		map[string]any{"agent_name": long}, "zh-CN", 7)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("long name status = %d, want 400, body %s", w.Code, w.Body.String())
	}

	// Blank strings are treated as absent.
	w = expertDo(t, r, http.MethodPost, "/api/v1/experts/stock-assistant/instantiate",
		map[string]any{"agent_name": "   ", "sandbox_config_id": ""}, "zh-CN", 7)
	if w.Code != http.StatusCreated {
		t.Fatalf("blank fields status = %d, want 201, body %s", w.Code, w.Body.String())
	}
	if got := svc.calls[len(svc.calls)-1]; got.req.AgentName != "" || got.req.SandboxConfigID != "" {
		t.Fatalf("blank strings must be absent, got %+v", got.req)
	}

	// Unknown expert → 404, service lookup only.
	w = expertDo(t, r, http.MethodPost, "/api/v1/experts/missing/instantiate",
		map[string]any{"agent_name": "x"}, "zh-CN", 7)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown expert status = %d, want 404, body %s", w.Code, w.Body.String())
	}

	// Missing tenant context → unauthorized, service untouched.
	before := len(svc.calls)
	w = expertDo(t, r, http.MethodPost, "/api/v1/experts/stock-assistant/instantiate", nil, "zh-CN", 0)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing tenant status = %d, want 401, body %s", w.Code, w.Body.String())
	}
	if len(svc.calls) != before {
		t.Fatalf("Instantiate reached without tenant context (%d calls)", len(svc.calls))
	}
}

// TestExpertInstantiateServiceError pins that non-sentinel service failures
// surface as 500 without leaking a stack.
func TestExpertInstantiateServiceError(t *testing.T) {
	_, svc := newExpertTestRouter(t)
	// A wrapper service whose Instantiate fails, to pin the 500 mapping.
	broken := &brokenInstantiateService{inner: svc}
	h := NewExpertHandler(broken)
	gin.SetMode(gin.TestMode)
	r2 := gin.New()
	r2.Use(middleware.ErrorHandler())
	v1 := r2.Group("/api/v1")
	eg := v1.Group("/experts")
	{
		eg.POST("/:id/instantiate", h.Instantiate)
	}

	w := expertDo(t, r2, http.MethodPost, "/api/v1/experts/stock-assistant/instantiate", nil, "zh-CN", 7)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500, body %s", w.Code, w.Body.String())
	}
}

// brokenInstantiateService delegates everything but Instantiate, which fails.
type brokenInstantiateService struct {
	inner *fakeExpertService
}

func (b *brokenInstantiateService) ListExperts(ctx context.Context) ([]*experts.Expert, error) {
	return b.inner.ListExperts(ctx)
}

func (b *brokenInstantiateService) GetExpert(ctx context.Context, id string) (*experts.Expert, error) {
	return b.inner.GetExpert(ctx, id)
}

func (b *brokenInstantiateService) Instantiate(
	context.Context, uint64, string, interfaces.InstantiateRequest,
) (*interfaces.InstantiateResult, error) {
	return nil, errors.New("boom")
}
