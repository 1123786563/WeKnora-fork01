package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// fakeSkillMarket is the interfaces.SkillMarketService seam: scripted answers
// plus a record of what each handler passed down.
type fakeSkillMarket struct {
	searchQuery string
	searchLimit int
	searchErr   error
	search      *interfaces.SkillMarketListing

	rankKind string
	rankErr  error
	rank     *interfaces.SkillMarketListing

	installSkillTenant uint64
	installSkillSlug   string
	installSkillConfs  []string
	installSkillErr    error
	installSkill       *interfaces.MarketSkillInstallResult

	listSkillsetsTenant uint64
	listErr             error
	list                *interfaces.MarketSkillsetIndex

	detailSlug string
	detailErr  error
	detail     *interfaces.MarketSkillsetDetail

	installSetTenant uint64
	installSetSlug   string
	installSetReq    interfaces.InstantiateRequest
	installSetErr    error
	installSet       *interfaces.MarketSkillsetInstallResult
}

func (f *fakeSkillMarket) SearchMarketSkills(_ context.Context, query string, limit int) (*interfaces.SkillMarketListing, error) {
	f.searchQuery = query
	f.searchLimit = limit
	return f.search, f.searchErr
}

func (f *fakeSkillMarket) MarketSkillRankings(_ context.Context, kind string) (*interfaces.SkillMarketListing, error) {
	f.rankKind = kind
	return f.rank, f.rankErr
}

func (f *fakeSkillMarket) InstallMarketSkill(_ context.Context, tenantID uint64, slug string, ids []string) (*interfaces.MarketSkillInstallResult, error) {
	f.installSkillTenant = tenantID
	f.installSkillSlug = slug
	f.installSkillConfs = ids
	return f.installSkill, f.installSkillErr
}

func (f *fakeSkillMarket) ListMarketSkillsets(_ context.Context, tenantID uint64) (*interfaces.MarketSkillsetIndex, error) {
	f.listSkillsetsTenant = tenantID
	return f.list, f.listErr
}

func (f *fakeSkillMarket) GetMarketSkillset(_ context.Context, _ uint64, slug string) (*interfaces.MarketSkillsetDetail, error) {
	f.detailSlug = slug
	return f.detail, f.detailErr
}

func (f *fakeSkillMarket) InstallMarketSkillset(_ context.Context, tenantID uint64, slug string, req interfaces.InstantiateRequest) (*interfaces.MarketSkillsetInstallResult, error) {
	f.installSetTenant = tenantID
	f.installSetSlug = slug
	f.installSetReq = req
	return f.installSet, f.installSetErr
}

func newMarketRouter(withTenant bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	// Mirror applyAuthSession: the tenant rides BOTH surfaces — the gin key
	// (c.GetUint64 readers) and the typed request-context key
	// (types.TenantIDFromContext readers).
	r.Use(func(c *gin.Context) {
		if withTenant {
			c.Set(types.TenantIDContextKey.String(), testSkillTenantID)
			c.Request = c.Request.WithContext(
				context.WithValue(c.Request.Context(), types.TenantIDContextKey, testSkillTenantID))
		}
		c.Next()
	})
	return r
}

func doMarketRequest(r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		reader = bytes.NewReader(encoded)
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

// ---------------------------------------------------------------------------

func TestMarketSearchServesResultsAndStaleFlag(t *testing.T) {
	market := &fakeSkillMarket{search: &interfaces.SkillMarketListing{
		Results: []interfaces.SkillMarketResult{{Slug: "pdf", Name: "PDF", Version: "1.2"}},
		Stale:   true,
	}}
	r := newMarketRouter(true)
	r.GET("/skills/market/search", NewSkillMarketHandler(market).Search)

	w := doMarketRequest(r, http.MethodGet, "/skills/market/search?q=pdf", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Results []interfaces.SkillMarketResult `json:"results"`
			Stale   bool                           `json:"stale"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.True(t, body.Data.Stale)
	require.Len(t, body.Data.Results, 1)
	require.Equal(t, "pdf", body.Data.Results[0].Slug)
	require.Equal(t, "pdf", market.searchQuery)
}

func TestMarketSearchDefaultsLimitAndRejectsGarbage(t *testing.T) {
	market := &fakeSkillMarket{search: &interfaces.SkillMarketListing{}}
	r := newMarketRouter(true)
	r.GET("/skills/market/search", NewSkillMarketHandler(market).Search)

	w := doMarketRequest(r, http.MethodGet, "/skills/market/search", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, defaultMarketSearchLimit, market.searchLimit, "an absent limit falls back to the default")

	w = doMarketRequest(r, http.MethodGet, "/skills/market/search?limit=abc", nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
	w = doMarketRequest(r, http.MethodGet, "/skills/market/search?limit=0", nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
	w = doMarketRequest(r, http.MethodGet, "/skills/market/search?limit=7", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 7, market.searchLimit)
}

func TestMarketRankingsServesKind(t *testing.T) {
	market := &fakeSkillMarket{rank: &interfaces.SkillMarketListing{
		Results: []interfaces.SkillMarketResult{{Slug: "pdf"}},
	}}
	r := newMarketRouter(true)
	r.GET("/skills/market/rankings/:kind", NewSkillMarketHandler(market).Rankings)

	w := doMarketRequest(r, http.MethodGet, "/skills/market/rankings/hot", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "hot", market.rankKind)

	var body struct {
		Data struct {
			Results []interfaces.SkillMarketResult `json:"results"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data.Results, 1)
}

func TestMarketRankingsUnknownKindIs400(t *testing.T) {
	market := &fakeSkillMarket{rankErr: apperrors.NewBadRequestError("unsupported ranking kind")}
	r := newMarketRouter(true)
	r.GET("/skills/market/rankings/:kind", NewSkillMarketHandler(market).Rankings)

	w := doMarketRequest(r, http.MethodGet, "/skills/market/rankings/bogus", nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMarketInstallSkillAcceptsAndReturns202Shape(t *testing.T) {
	market := &fakeSkillMarket{installSkill: &interfaces.MarketSkillInstallResult{
		CatalogID:  "cat-1",
		InstallIDs: []string{"sk-1", "sk-2"},
	}}
	r := newMarketRouter(true)
	r.POST("/skills/market/install", NewSkillMarketHandler(market).InstallSkill)

	w := doMarketRequest(r, http.MethodPost, "/skills/market/install", map[string]any{
		"slug": "pdf-extract", "sandbox_config_ids": []string{"cfg-1", "cfg-2"},
	})
	require.Equal(t, http.StatusAccepted, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			CatalogID  string   `json:"catalog_id"`
			InstallIDs []string `json:"install_ids"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "cat-1", body.Data.CatalogID)
	require.Equal(t, []string{"sk-1", "sk-2"}, body.Data.InstallIDs)
	require.Equal(t, "pdf-extract", market.installSkillSlug)
	require.Equal(t, []string{"cfg-1", "cfg-2"}, market.installSkillConfs)
	require.Equal(t, testSkillTenantID, market.installSkillTenant)
}

func TestMarketInstallSkillPartialErrorsKeep202WithSuccessFalse(t *testing.T) {
	market := &fakeSkillMarket{installSkill: &interfaces.MarketSkillInstallResult{
		CatalogID:  "cat-1",
		InstallIDs: []string{"sk-1"},
		Errors:     map[string]string{"cfg-2": "sandbox config not found"},
	}}
	r := newMarketRouter(true)
	r.POST("/skills/market/install", NewSkillMarketHandler(market).InstallSkill)

	w := doMarketRequest(r, http.MethodPost, "/skills/market/install", map[string]any{
		"slug": "pdf-extract", "sandbox_config_ids": []string{"cfg-1", "cfg-2"},
	})
	require.Equal(t, http.StatusAccepted, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Errors map[string]string `json:"errors"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.False(t, body.Success)
	require.Equal(t, "sandbox config not found", body.Data.Errors["cfg-2"])
}

func TestMarketInstallSkillStrictBody(t *testing.T) {
	market := &fakeSkillMarket{installSkill: &interfaces.MarketSkillInstallResult{CatalogID: "cat-1"}}
	r := newMarketRouter(true)
	r.POST("/skills/market/install", NewSkillMarketHandler(market).InstallSkill)

	// Unknown fields are rejected.
	w := doMarketRequest(r, http.MethodPost, "/skills/market/install", map[string]any{
		"slug": "pdf", "sandbox_config_ids": []string{"cfg-1"}, "extra": true,
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Empty(t, market.installSkillSlug)

	// Trailing input is rejected.
	req := httptest.NewRequest(http.MethodPost, "/skills/market/install",
		bytes.NewReader([]byte(`{"slug":"pdf"}{"slug":"pdf"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// A body is required (unlike instantiate, the slug cannot be derived).
	w = doMarketRequest(r, http.MethodPost, "/skills/market/install", nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Empty(t, market.installSkillSlug)
}

func TestMarketInstallSkillUnreachableIs503(t *testing.T) {
	market := &fakeSkillMarket{installSkillErr: apperrors.NewServiceUnavailableError("the skill market is unreachable")}
	r := newMarketRouter(true)
	r.POST("/skills/market/install", NewSkillMarketHandler(market).InstallSkill)

	w := doMarketRequest(r, http.MethodPost, "/skills/market/install", map[string]any{
		"slug": "pdf", "sandbox_config_ids": []string{"cfg-1"},
	})
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), "unreachable")
}

func TestMarketListSkillsetsServesIndex(t *testing.T) {
	market := &fakeSkillMarket{list: &interfaces.MarketSkillsetIndex{
		Skillsets: []interfaces.MarketSkillset{{Slug: "pdf-tools", Installed: true}},
		Stale:     true,
	}}
	r := newMarketRouter(true)
	r.GET("/experts/market", NewSkillMarketHandler(market).ListSkillsets)

	w := doMarketRequest(r, http.MethodGet, "/experts/market", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, testSkillTenantID, market.listSkillsetsTenant)

	var body struct {
		Data struct {
			Skillsets []interfaces.MarketSkillset `json:"skillsets"`
			Stale     bool                        `json:"stale"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Data.Stale)
	require.True(t, body.Data.Skillsets[0].Installed)
}

func TestMarketGetSkillsetServesDetailAnd404(t *testing.T) {
	market := &fakeSkillMarket{detail: &interfaces.MarketSkillsetDetail{
		Slug: "pdf-tools", NameEn: "PDF Toolkit",
	}}
	r := newMarketRouter(true)
	r.GET("/experts/market/:slug", NewSkillMarketHandler(market).GetSkillset)

	w := doMarketRequest(r, http.MethodGet, "/experts/market/pdf-tools", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "pdf-tools", market.detailSlug)

	var body struct {
		Data interfaces.MarketSkillsetDetail `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "PDF Toolkit", body.Data.NameEn)

	market.detailErr = apperrors.NewNotFoundError("skillset not found")
	w = doMarketRequest(r, http.MethodGet, "/experts/market/nope", nil)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestMarketInstallSkillsetReturns201Shape(t *testing.T) {
	market := &fakeSkillMarket{installSet: &interfaces.MarketSkillsetInstallResult{
		InstantiateResult: interfaces.InstantiateResult{
			Agent:           &types.CustomAgent{ID: "agent-1", Name: "PDF 专家"},
			PendingSkills:   []string{"pdf-extract"},
			SkillInstallIDs: []string{"inst-1"},
		},
		ExpertID: "skillhub-skillset-pdf-tools",
	}}
	r := newMarketRouter(true)
	r.POST("/experts/market/:slug/install", NewSkillMarketHandler(market).InstallSkillset)

	w := doMarketRequest(r, http.MethodPost, "/experts/market/pdf-tools/install", map[string]any{
		"agent_name": "我的专家", "sandbox_config_id": "cfg-1",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Agent          *types.CustomAgent `json:"agent"`
			PendingSkills  []string           `json:"pending_skills"`
			SkillInstallID []string           `json:"skill_install_ids"`
			ExpertID       string             `json:"expert_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "agent-1", body.Data.Agent.ID)
	require.Equal(t, []string{"pdf-extract"}, body.Data.PendingSkills)
	require.Equal(t, []string{"inst-1"}, body.Data.SkillInstallID)
	require.Equal(t, "skillhub-skillset-pdf-tools", body.Data.ExpertID)

	require.Equal(t, "pdf-tools", market.installSetSlug)
	require.Equal(t, testSkillTenantID, market.installSetTenant)
	require.Equal(t, "我的专家", market.installSetReq.AgentName)
	require.Equal(t, "cfg-1", market.installSetReq.SandboxConfigID)
}

func TestMarketInstallSkillsetAllowsEmptyBody(t *testing.T) {
	market := &fakeSkillMarket{installSet: &interfaces.MarketSkillsetInstallResult{
		InstantiateResult: interfaces.InstantiateResult{Agent: &types.CustomAgent{ID: "agent-1"}},
		ExpertID:          "skillhub-skillset-pdf-tools",
	}}
	r := newMarketRouter(true)
	r.POST("/experts/market/:slug/install", NewSkillMarketHandler(market).InstallSkillset)

	w := doMarketRequest(r, http.MethodPost, "/experts/market/pdf-tools/install", nil)
	require.Equal(t, http.StatusCreated, w.Code)
	require.Empty(t, market.installSetReq.AgentName)
}

func TestMarketInstallSkillsetRejectsUnknownFieldsAndOversizedName(t *testing.T) {
	market := &fakeSkillMarket{installSet: &interfaces.MarketSkillsetInstallResult{
		InstantiateResult: interfaces.InstantiateResult{Agent: &types.CustomAgent{ID: "agent-1"}},
	}}
	r := newMarketRouter(true)
	r.POST("/experts/market/:slug/install", NewSkillMarketHandler(market).InstallSkillset)

	w := doMarketRequest(r, http.MethodPost, "/experts/market/pdf-tools/install", map[string]any{"bogus": 1})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Empty(t, market.installSetSlug)

	long := make([]byte, 256)
	for i := range long {
		long[i] = 'a'
	}
	w = doMarketRequest(r, http.MethodPost, "/experts/market/pdf-tools/install", map[string]any{
		"agent_name": string(long),
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "255")
}

func TestMarketInstallSkillsetRequiresTenantContext(t *testing.T) {
	market := &fakeSkillMarket{installSet: &interfaces.MarketSkillsetInstallResult{
		InstantiateResult: interfaces.InstantiateResult{Agent: &types.CustomAgent{ID: "agent-1"}},
	}}
	r := newMarketRouter(false) // no tenant on the context
	r.POST("/experts/market/:slug/install", NewSkillMarketHandler(market).InstallSkillset)

	w := doMarketRequest(r, http.MethodPost, "/experts/market/pdf-tools/install", nil)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Empty(t, market.installSetSlug, "the service is never reached without a tenant")
}
