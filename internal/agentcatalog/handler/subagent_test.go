package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// fakeSubagentService stands in for interfaces.SubagentService. The catalog
// payloads mirror the subagents package's fixture library
// (internal/agent/subagents/testdata): one division tree with a zh+en role
// (product-manager) and a zh-only role (researcher), so the handler tests pin
// the projection rules (per-locale presence, installed flags, counts) against
// the shapes the real service produces.
type fakeSubagentService struct {
	catalog   *interfaces.SubagentCatalog
	details   map[string]*interfaces.SubagentCatalogDetail
	knownSlug map[string]bool

	// agents is the agentID → configured-subagents store the mutation
	// endpoints operate on; a missing key is the unknown-agent 404.
	agents map[string][]string

	// installs records every InstallForAgent call for assertions.
	installs []fakeSubagentInstall
	// rows is the tenant-row store: install appends, remove never touches.
	rows []string
}

type fakeSubagentInstall struct {
	tenantID uint64
	agentID  string
	slug     string
	locale   string
}

func (s *fakeSubagentService) ListCatalog(_ context.Context, _ uint64) (*interfaces.SubagentCatalog, error) {
	return s.catalog, nil
}

func (s *fakeSubagentService) GetCatalogEntry(_ context.Context, _ uint64, slug string) (*interfaces.SubagentCatalogDetail, error) {
	if d, ok := s.details[slug]; ok {
		return d, nil
	}
	return nil, service.ErrSubagentNotFound
}

func (s *fakeSubagentService) ListAgentSubagents(_ context.Context, agentID string) ([]string, error) {
	list, ok := s.agents[agentID]
	if !ok {
		return nil, service.ErrAgentNotFound
	}
	return append([]string(nil), list...), nil
}

func (s *fakeSubagentService) InstallForAgent(
	_ context.Context, tenantID uint64, agentID, slug, locale string,
) ([]string, error) {
	list, ok := s.agents[agentID]
	if !ok {
		return nil, service.ErrAgentNotFound
	}
	if !s.knownSlug[slug] {
		return nil, service.ErrSubagentNotFound
	}
	s.installs = append(s.installs, fakeSubagentInstall{tenantID: tenantID, agentID: agentID, slug: slug, locale: locale})
	s.rows = append(s.rows, slug)

	for _, existing := range list {
		if existing == slug {
			return append([]string(nil), list...), nil
		}
	}
	list = append(list, slug)
	s.agents[agentID] = list
	return append([]string(nil), list...), nil
}

func (s *fakeSubagentService) RemoveFromAgent(_ context.Context, agentID, slug string) ([]string, error) {
	list, ok := s.agents[agentID]
	if !ok {
		return nil, service.ErrAgentNotFound
	}
	kept := make([]string, 0, len(list))
	for _, existing := range list {
		if existing != slug {
			kept = append(kept, existing)
		}
	}
	s.agents[agentID] = kept
	return append([]string(nil), kept...), nil
}

// newSubagentTestRouter builds a gin engine mirroring the production subagent
// route tree at /api/v1, with the execution tenant (7) and locale (en-US) on
// the request context the way the auth/language middleware install them.
func newSubagentTestRouter(t *testing.T) (*gin.Engine, *fakeSubagentService) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	svc := &fakeSubagentService{
		catalog: &interfaces.SubagentCatalog{
			Divisions: []interfaces.SubagentCatalogDivision{
				{Slug: "product", Label: "产品", Icon: "Box", Color: "#D946EF", Count: 2},
				{Slug: "academic", Label: "学术", Icon: "GraduationCap", Color: "#8B5CF6", Count: 0},
				{Slug: "engineering", Label: "工程", Icon: "Code", Color: "#3B82F6", Count: 0},
			},
			Total: 2,
			Entries: []interfaces.SubagentCatalogEntry{
				{Slug: "product-manager", Division: "product", NameZh: "产品经理", NameEn: "Product Manager", Emoji: "🧭", Color: "blue", Installed: true},
				{Slug: "researcher", Division: "product", NameZh: "用户研究员", NameEn: "", Emoji: "🔍", Color: "green", Installed: false},
			},
		},
		details: map[string]*interfaces.SubagentCatalogDetail{
			"product-manager": {
				Slug: "product-manager", Division: "product",
				NameZh: "产品经理", NameEn: "Product Manager",
				Emoji: "🧭", Color: "blue", Vibe: "Ships the right thing.",
				ToolsRaw:  "WebFetch, WebSearch, Read",
				BodyZh:    "# 🧭 产品经理代理\n\n你是 Alex。",
				BodyEn:    "# 🧭 Product Manager Agent\n\nYou are Alex.",
				Installed: true,
			},
			"researcher": {
				Slug: "researcher", Division: "product",
				NameZh: "用户研究员", NameEn: "",
				Emoji: "🔍", Color: "green", Vibe: "先听，再判断。",
				ToolsRaw:  "阅读、写作、编辑",
				BodyZh:    "# 🔍 用户研究员代理\n\n你是 Sam。",
				BodyEn:    "",
				Installed: false,
			},
		},
		knownSlug: map[string]bool{"product-manager": true, "researcher": true},
		agents: map[string][]string{
			"a1": {"researcher"},
		},
	}
	h := NewSubagentHandler(svc)

	r := gin.New()
	r.Use(middleware.ErrorHandler())
	v1 := r.Group("/api/v1")
	catalog := v1.Group("/subagent-catalog")
	{
		catalog.GET("", h.ListCatalog)
		catalog.GET("/:slug", h.GetCatalogEntry)
	}
	agents := v1.Group("/agents")
	{
		agents.GET("/:id/subagents", h.ListAgentSubagents)
		agents.POST("/:id/subagents", h.InstallForAgent)
		agents.DELETE("/:id/subagents/:slug", h.RemoveFromAgent)
	}
	return r, svc
}

func subagentDo(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
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
	ctx := types.WithExecutionTenant(req.Context(), 7)
	ctx = context.WithValue(ctx, types.LanguageContextKey, "en-US")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestSubagentCatalogList pins the list shape against the fixture catalog:
// divisions in divisions.json order with per-division entry counts, total,
// entries sorted by slug with per-locale name presence and tenant installed
// flags.
func TestSubagentCatalogList(t *testing.T) {
	r, _ := newSubagentTestRouter(t)
	w := subagentDo(t, r, http.MethodGet, "/api/v1/subagent-catalog", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Divisions []struct {
				Slug  string `json:"slug"`
				Label string `json:"label"`
				Icon  string `json:"icon"`
				Color string `json:"color"`
				Count int    `json:"count"`
			} `json:"divisions"`
			Total   int `json:"total"`
			Entries []struct {
				Slug      string `json:"slug"`
				Division  string `json:"division"`
				NameZh    string `json:"name_zh"`
				NameEn    string `json:"name_en"`
				Emoji     string `json:"emoji"`
				Color     string `json:"color"`
				Installed bool   `json:"installed"`
			} `json:"entries"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatalf("success = false, body %s", w.Body.String())
	}
	if body.Data.Total != 2 {
		t.Fatalf("total = %d, want 2", body.Data.Total)
	}
	if len(body.Data.Divisions) != 3 {
		t.Fatalf("divisions = %d, want 3", len(body.Data.Divisions))
	}
	product := body.Data.Divisions[0]
	if product.Slug != "product" || product.Label != "产品" || product.Icon != "Box" || product.Color != "#D946EF" || product.Count != 2 {
		t.Fatalf("product division = %+v", product)
	}
	if body.Data.Divisions[1].Slug != "academic" || body.Data.Divisions[1].Count != 0 {
		t.Fatalf("academic division = %+v", body.Data.Divisions[1])
	}
	if len(body.Data.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(body.Data.Entries))
	}
	pm := body.Data.Entries[0]
	if pm.Slug != "product-manager" || pm.NameZh != "产品经理" || pm.NameEn != "Product Manager" || !pm.Installed {
		t.Fatalf("product-manager entry = %+v", pm)
	}
	res := body.Data.Entries[1]
	if res.Slug != "researcher" || res.NameZh != "用户研究员" || res.NameEn != "" || res.Installed {
		t.Fatalf("researcher entry = %+v (name_en must be empty when the locale is absent; installed reflects tenant rows)", res)
	}
}

// TestSubagentCatalogDetail pins the detail shape (both-locale bodies
// present-or-empty) and the 404 for an unknown slug.
func TestSubagentCatalogDetail(t *testing.T) {
	r, _ := newSubagentTestRouter(t)

	w := subagentDo(t, r, http.MethodGet, "/api/v1/subagent-catalog/product-manager", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Slug      string `json:"slug"`
			Division  string `json:"division"`
			NameZh    string `json:"name_zh"`
			NameEn    string `json:"name_en"`
			Emoji     string `json:"emoji"`
			Color     string `json:"color"`
			Vibe      string `json:"vibe"`
			ToolsRaw  string `json:"tools_raw"`
			BodyZh    string `json:"body_zh"`
			BodyEn    string `json:"body_en"`
			Installed bool   `json:"installed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	d := body.Data
	if d.Slug != "product-manager" || d.Division != "product" || !d.Installed {
		t.Fatalf("detail = %+v", d)
	}
	if d.BodyZh == "" || d.BodyEn == "" || d.ToolsRaw == "" || d.Vibe == "" {
		t.Fatalf("detail dropped a locale body or frontmatter field: %+v", d)
	}

	// zh-only role: en fields are empty strings, not omitted.
	w = subagentDo(t, r, http.MethodGet, "/api/v1/subagent-catalog/researcher", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("researcher status %d body %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.NameEn != "" || body.Data.BodyEn != "" {
		t.Fatalf("zh-only detail = %+v, want empty en fields", body.Data)
	}
	if body.Data.BodyZh == "" {
		t.Fatal("zh-only detail lost body_zh")
	}

	w = subagentDo(t, r, http.MethodGet, "/api/v1/subagent-catalog/nope", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown slug status = %d, want 404, body %s", w.Code, w.Body.String())
	}
}

// TestSubagentAgentList pins GET /agents/:id/subagents: config order
// preserved, unknown agent 404.
func TestSubagentAgentList(t *testing.T) {
	r, _ := newSubagentTestRouter(t)

	w := subagentDo(t, r, http.MethodGet, "/api/v1/agents/a1/subagents", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Subagents []string `json:"subagents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Subagents) != 1 || body.Data.Subagents[0] != "researcher" {
		t.Fatalf("subagents = %v, want [researcher]", body.Data.Subagents)
	}

	w = subagentDo(t, r, http.MethodGet, "/api/v1/agents/nope/subagents", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown agent status = %d, want 404, body %s", w.Code, w.Body.String())
	}
}

// TestSubagentInstallHappyAndIdempotent covers POST /agents/:id/subagents:
// happy install appends the slug, passes the ctx tenant and body locale to
// the service, and a second identical install is a 200 no-op returning the
// same list.
func TestSubagentInstallHappyAndIdempotent(t *testing.T) {
	r, svc := newSubagentTestRouter(t)

	w := subagentDo(t, r, http.MethodPost, "/api/v1/agents/a1/subagents",
		map[string]any{"slug": "product-manager", "locale": "zh-CN"})
	if w.Code != http.StatusOK {
		t.Fatalf("install status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Subagents []string `json:"subagents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Subagents) != 2 || body.Data.Subagents[0] != "researcher" || body.Data.Subagents[1] != "product-manager" {
		t.Fatalf("subagents = %v, want [researcher product-manager] (config order preserved)", body.Data.Subagents)
	}
	if len(svc.installs) != 1 {
		t.Fatalf("InstallForAgent called %d times, want 1", len(svc.installs))
	}
	if svc.installs[0].tenantID != 7 || svc.installs[0].locale != "zh-CN" {
		t.Fatalf("install call = %+v, want tenant 7 and locale passthrough", svc.installs[0])
	}

	// Idempotent second install: 200, same list, no duplicate.
	w = subagentDo(t, r, http.MethodPost, "/api/v1/agents/a1/subagents",
		map[string]any{"slug": "product-manager"})
	if w.Code != http.StatusOK {
		t.Fatalf("second install status %d body %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Subagents) != 2 {
		t.Fatalf("second install subagents = %v, want unchanged [researcher product-manager]", body.Data.Subagents)
	}
}

// TestSubagentInstallRejections covers the error paths: unknown slug 404 with
// no install recorded, unknown agent 404, invalid body 400 (unknown field,
// missing slug, trailing input).
func TestSubagentInstallRejections(t *testing.T) {
	r, svc := newSubagentTestRouter(t)

	w := subagentDo(t, r, http.MethodPost, "/api/v1/agents/a1/subagents",
		map[string]any{"slug": "ghost"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown slug status = %d, want 404, body %s", w.Code, w.Body.String())
	}
	if len(svc.installs) != 0 {
		t.Fatalf("InstallForAgent must not run for an unknown slug, got %d calls", len(svc.installs))
	}

	w = subagentDo(t, r, http.MethodPost, "/api/v1/agents/nope/subagents",
		map[string]any{"slug": "researcher"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown agent status = %d, want 404, body %s", w.Code, w.Body.String())
	}

	// Unknown body field → 400 (strict decode).
	w = subagentDo(t, r, http.MethodPost, "/api/v1/agents/a1/subagents",
		map[string]any{"slug": "researcher", "force": true})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want 400, body %s", w.Code, w.Body.String())
	}

	// Missing slug → 400.
	w = subagentDo(t, r, http.MethodPost, "/api/v1/agents/a1/subagents", map[string]any{"locale": "zh"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing slug status = %d, want 400, body %s", w.Code, w.Body.String())
	}

	// Trailing input after the JSON object → 400.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/a1/subagents",
		bytes.NewReader([]byte(`{"slug":"researcher"} {"slug":"more"}`)))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(types.WithExecutionTenant(req.Context(), 7))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("trailing input status = %d, want 400, body %s", rec.Code, rec.Body.String())
	}
	if len(svc.installs) != 0 {
		t.Fatalf("no install may run for invalid bodies, got %d calls", len(svc.installs))
	}
}

// TestSubagentRemoveIdempotent covers DELETE /agents/:id/subagents/:slug:
// removal, idempotent second call, and the tenant rows staying untouched
// (rows are shared across agents; row deletion is out of this endpoint's
// scope).
func TestSubagentRemoveIdempotent(t *testing.T) {
	r, svc := newSubagentTestRouter(t)
	svc.rows = []string{"researcher"}

	w := subagentDo(t, r, http.MethodDelete, "/api/v1/agents/a1/subagents/researcher", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("remove status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Subagents []string `json:"subagents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Subagents) != 0 {
		t.Fatalf("subagents = %v, want empty list", body.Data.Subagents)
	}

	// Absent slug → 200 idempotent, same (empty) list.
	w = subagentDo(t, r, http.MethodDelete, "/api/v1/agents/a1/subagents/researcher", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("second remove status = %d, want 200, body %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Subagents) != 0 {
		t.Fatalf("second remove subagents = %v, want still empty", body.Data.Subagents)
	}

	// The tenant row outlives the agent-config removal.
	if len(svc.rows) != 1 || svc.rows[0] != "researcher" {
		t.Fatalf("tenant rows = %v, want untouched [researcher]", svc.rows)
	}

	w = subagentDo(t, r, http.MethodDelete, "/api/v1/agents/nope/subagents/researcher", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown agent status = %d, want 404, body %s", w.Code, w.Body.String())
	}
}
