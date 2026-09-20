package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// fakeTenantSkillMarket is the interfaces.TenantSkillMarketService seam:
// scripted answers plus a record of what each handler passed down.
type fakeTenantSkillMarket struct {
	publishTenant uint64
	publishID     string
	publishBy     string
	publishErr    error
	publish       *interfaces.PublishedSkillView

	unpublishTenant uint64
	unpublishID     string
	unpublishErr    error

	listTenant uint64
	listErr    error
	list       *interfaces.PublishedSkillIndex

	installTenant uint64
	installID     string
	installConfs  []string
	installErr    error
	install       *interfaces.TenantSkillInstallResult
}

func (f *fakeTenantSkillMarket) PublishSkill(
	_ context.Context, tenantID uint64, catalogID, publishedBy string,
) (*interfaces.PublishedSkillView, error) {
	f.publishTenant = tenantID
	f.publishID = catalogID
	f.publishBy = publishedBy
	return f.publish, f.publishErr
}

func (f *fakeTenantSkillMarket) UnpublishSkill(_ context.Context, tenantID uint64, catalogID string) error {
	f.unpublishTenant = tenantID
	f.unpublishID = catalogID
	return f.unpublishErr
}

func (f *fakeTenantSkillMarket) ListPublishedSkills(
	_ context.Context, tenantID uint64,
) (*interfaces.PublishedSkillIndex, error) {
	f.listTenant = tenantID
	return f.list, f.listErr
}

func (f *fakeTenantSkillMarket) InstallPublishedSkill(
	_ context.Context, tenantID uint64, catalogID string, sandboxConfigIDs []string,
) (*interfaces.TenantSkillInstallResult, error) {
	f.installTenant = tenantID
	f.installID = catalogID
	f.installConfs = sandboxConfigIDs
	return f.install, f.installErr
}

// newTenantMarketRouter mirrors the auth middleware: the tenant and the user
// ride both surfaces (gin keys and the typed request context).
func newTenantMarketRouter(withTenant, withUser bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if withTenant {
			c.Set(types.TenantIDContextKey.String(), testSkillTenantID)
			c.Request = c.Request.WithContext(
				context.WithValue(c.Request.Context(), types.TenantIDContextKey, testSkillTenantID))
		}
		if withUser {
			c.Request = c.Request.WithContext(
				context.WithValue(c.Request.Context(), types.UserIDContextKey, "user-1"))
		}
		c.Next()
	})
	return r
}

func doTenantMarketRequest(r *gin.Engine, method, path, body, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------

func TestTenantMarketPublishAcceptsEmptyAndEmptyObjectBody(t *testing.T) {
	market := &fakeTenantSkillMarket{publish: &interfaces.PublishedSkillView{CatalogID: "cat-a"}}
	r := newTenantMarketRouter(true, true)
	r.POST("/skills/catalog/:id/publish", NewTenantSkillMarketHandler(market).PublishSkill)

	for _, body := range []string{"", "{}"} {
		w := doTenantMarketRequest(r, http.MethodPost, "/skills/catalog/cat-a/publish", body, "application/json")
		require.Equal(t, http.StatusOK, w.Code, "body %q: %s", body, w.Body.String())
	}

	// The body is strict: unknown fields and trailing input are refused.
	for _, body := range []string{`{"note":"hi"}`, `{}{}`, `null`, `[]`} {
		w := doTenantMarketRequest(r, http.MethodPost, "/skills/catalog/cat-a/publish", body, "application/json")
		require.Equal(t, http.StatusBadRequest, w.Code, "body %q must be refused", body)
	}
}

func TestTenantMarketPublishCapsBodySize(t *testing.T) {
	market := &fakeTenantSkillMarket{publish: &interfaces.PublishedSkillView{CatalogID: "cat-a"}}
	r := newTenantMarketRouter(true, true)
	r.POST("/skills/catalog/:id/publish", NewTenantSkillMarketHandler(market).PublishSkill)

	big := strings.Repeat("a", skillSourceJSONMaxBytes+1)
	w := doTenantMarketRequest(r, http.MethodPost, "/skills/catalog/cat-a/publish", big, "text/plain")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "too large")
}

func TestTenantMarketPublishDerivesTenantAndUser(t *testing.T) {
	market := &fakeTenantSkillMarket{publish: &interfaces.PublishedSkillView{CatalogID: "cat-a", PublishedBy: "user-1"}}
	r := newTenantMarketRouter(true, true)
	r.POST("/skills/catalog/:id/publish", NewTenantSkillMarketHandler(market).PublishSkill)

	w := doTenantMarketRequest(r, http.MethodPost, "/skills/catalog/cat-a/publish", "", "application/json")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, testSkillTenantID, market.publishTenant)
	require.Equal(t, "cat-a", market.publishID)
	require.Equal(t, "user-1", market.publishBy)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			CatalogID   string `json:"catalog_id"`
			PublishedBy string `json:"published_by"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "cat-a", body.Data.CatalogID)
	require.Equal(t, "user-1", body.Data.PublishedBy)
}

func TestTenantMarketPublishMapsNotFound(t *testing.T) {
	market := &fakeTenantSkillMarket{publishErr: apperrors.NewNotFoundError("skill not found")}
	r := newTenantMarketRouter(true, true)
	r.POST("/skills/catalog/:id/publish", NewTenantSkillMarketHandler(market).PublishSkill)

	w := doTenantMarketRequest(r, http.MethodPost, "/skills/catalog/nope/publish", "", "application/json")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestTenantMarketUnpublishRespondsAndMapsErrors(t *testing.T) {
	market := &fakeTenantSkillMarket{}
	r := newTenantMarketRouter(true, true)
	r.DELETE("/skills/catalog/:id/publish", NewTenantSkillMarketHandler(market).UnpublishSkill)

	w := doTenantMarketRequest(r, http.MethodDelete, "/skills/catalog/cat-a/publish", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, testSkillTenantID, market.unpublishTenant)
	require.Equal(t, "cat-a", market.unpublishID)

	market.unpublishErr = apperrors.NewNotFoundError("skill not found")
	w = doTenantMarketRequest(r, http.MethodDelete, "/skills/catalog/nope/publish", "", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestTenantMarketListServesPublishedSkills(t *testing.T) {
	market := &fakeTenantSkillMarket{list: &interfaces.PublishedSkillIndex{
		Skills: []interfaces.PublishedSkillEntry{{
			CatalogID: "cat-a", Name: "pdf-extract", Version: "1.0",
			PublisherName: "alice", Installed: true,
		}},
	}}
	r := newTenantMarketRouter(true, false)
	r.GET("/market/tenant/skills", NewTenantSkillMarketHandler(market).ListPublishedSkills)

	w := doTenantMarketRequest(r, http.MethodGet, "/market/tenant/skills", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, testSkillTenantID, market.listTenant)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Skills []interfaces.PublishedSkillEntry `json:"skills"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Len(t, body.Data.Skills, 1)
	require.Equal(t, "pdf-extract", body.Data.Skills[0].Name)
	require.True(t, body.Data.Skills[0].Installed)
}

func TestTenantMarketInstallDecodesStrictBodyAndAccepts(t *testing.T) {
	market := &fakeTenantSkillMarket{install: &interfaces.TenantSkillInstallResult{
		Installs: map[string]string{"cfg-1": "sk-1"},
	}}
	r := newTenantMarketRouter(true, false)
	r.POST("/market/tenant/skills/:catalogId/install", NewTenantSkillMarketHandler(market).InstallPublishedSkill)

	w := doTenantMarketRequest(r, http.MethodPost, "/market/tenant/skills/cat-a/install",
		`{"sandbox_config_ids":["cfg-1"]}`, "application/json")
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	require.Equal(t, testSkillTenantID, market.installTenant)
	require.Equal(t, "cat-a", market.installID)
	require.Equal(t, []string{"cfg-1"}, market.installConfs)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Installs map[string]string `json:"installs"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "sk-1", body.Data.Installs["cfg-1"])

	// Strict: unknown fields, trailing input and oversized bodies are 400s.
	for _, raw := range []string{
		`{"sandbox_config_ids":["cfg-1"],"extra":true}`,
		`{"sandbox_config_ids":["cfg-1"]}{"sandbox_config_ids":["cfg-2"]}`,
	} {
		w = doTenantMarketRequest(r, http.MethodPost, "/market/tenant/skills/cat-a/install", raw, "application/json")
		require.Equal(t, http.StatusBadRequest, w.Code, "body %q must be refused", raw)
	}
	big := bytes.Repeat([]byte(" "), skillSourceJSONMaxBytes+1)
	w = doTenantMarketRequest(r, http.MethodPost, "/market/tenant/skills/cat-a/install", string(big), "application/json")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "too large")
}

func TestTenantMarketInstallRidesPartialFailuresOn202(t *testing.T) {
	market := &fakeTenantSkillMarket{install: &interfaces.TenantSkillInstallResult{
		Installs: map[string]string{"cfg-1": "sk-1"},
		Errors:   map[string]string{"cfg-2": "missing config"},
	}}
	r := newTenantMarketRouter(true, false)
	r.POST("/market/tenant/skills/:catalogId/install", NewTenantSkillMarketHandler(market).InstallPublishedSkill)

	w := doTenantMarketRequest(r, http.MethodPost, "/market/tenant/skills/cat-a/install",
		`{"sandbox_config_ids":["cfg-1","cfg-2"]}`, "application/json")
	require.Equal(t, http.StatusAccepted, w.Code, "partial failures keep the 202 (catalog-install mirror)")

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Installs map[string]string `json:"installs"`
			Errors   map[string]string `json:"errors"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.False(t, body.Success, "per-config failures clear the success flag")
	require.Equal(t, "sk-1", body.Data.Installs["cfg-1"])
	require.Equal(t, "missing config", body.Data.Errors["cfg-2"])
}

func TestTenantMarketInstallMapsServiceErrors(t *testing.T) {
	market := &fakeTenantSkillMarket{installErr: apperrors.NewNotFoundError("skill is not published")}
	r := newTenantMarketRouter(true, false)
	r.POST("/market/tenant/skills/:catalogId/install", NewTenantSkillMarketHandler(market).InstallPublishedSkill)

	w := doTenantMarketRequest(r, http.MethodPost, "/market/tenant/skills/nope/install",
		`{"sandbox_config_ids":["cfg-1"]}`, "application/json")
	require.Equal(t, http.StatusNotFound, w.Code)
}
