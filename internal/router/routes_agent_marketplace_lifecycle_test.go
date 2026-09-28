package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAgentMarketplaceLifecycleRoutesRequireAdminAndFullAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")
	RegisterAgentMarketplaceLifecycleRoutes(v1, handler.NewAgentMarketplaceLifecycleHandler(nil), g)
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/marketplace/tenant/variants/:id/retire"},
		{http.MethodPost, "/api/v1/marketplace/tenant/adoptions/:id/end"},
		{http.MethodPost, "/api/v1/marketplace/tenant/listings/:id/unlist"},
		{http.MethodPost, "/api/v1/marketplace/tenant/releases/:id/deprecate"},
	} {
		policy := mustLookupAPIKeyPolicy(t, g, route.method, route.path)
		require.Truef(t, policy.RequireFullAccess, "%s %s 必须要求 full-access", route.method, route.path)
	}
	bare := gin.New()
	RegisterAgentMarketplaceLifecycleRoutes(bare.Group("/api/v1"), nil, &rbacGuards{})
	require.Empty(t, bare.Routes())
}

func newLifecycleTestApp(t *testing.T) (*gin.Engine, *rbacGuards, *gorm.DB) {
	t.Helper()
	db := openTenantAgentMarketplaceHTTPTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{ID: "agent-owned", Name: "Lifecycle helper", TenantID: 1, CreatedBy: "contributor", Config: types.CustomAgentConfig{AgentMode: "smart-reasoning", SystemPrompt: "Be useful."}}).Error)
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil)
	versions := service.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	market := service.NewAgentMarketplaceService(versions, marketplaceHTTPResolver{}, repository.NewAgentMarketplaceRepository(db), t.TempDir())
	adoptionsRepo := repository.NewAgentAdoptionRepository(db)
	adoptions := service.NewAgentAdoptionService(adoptionsRepo, customAgents, versions)
	upgrades := service.NewAgentUpgradeService(repository.NewAgentUpgradeRepository(db))
	lifecycle := service.NewAgentMarketplaceLifecycleService(adoptionsRepo, repository.NewAgentMarketplaceRepository(db))

	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}, agentCreator: func(c *gin.Context) (string, error) {
		if c.Param("id") == "agent-owned" {
			return "contributor", nil
		}
		return "", nil
	}}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(newLifecycleIdentityMiddleware())
	v1 := r.Group("/api/v1")
	RegisterAgentVersionRoutes(v1, handler.NewAgentVersionHandler(versions), g)
	RegisterAgentMarketplaceRoutes(v1, handler.NewAgentMarketplaceHandler(market, versions), g)
	RegisterAgentAdoptionRoutes(v1, handler.NewAgentAdoptionHandler(adoptions), g)
	RegisterAgentUpgradeRoutes(v1, handler.NewAgentUpgradeHandler(upgrades), g)
	RegisterAgentMarketplaceLifecycleRoutes(v1, handler.NewAgentMarketplaceLifecycleHandler(lifecycle), g)
	return r, g, db
}

func newLifecycleIdentityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := uint64(1)
		if c.GetHeader("X-Test-Tenant") == "2" {
			tenantID = 2
		}
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenantID)
		ctx = context.WithValue(ctx, types.UserIDContextKey, c.GetHeader("X-Test-Actor"))
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(c.GetHeader("X-Test-Role")))
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), tenantID)
		c.Next()
	}
}

func lifecycleMetadata() map[string]any {
	return map[string]any{"supported_languages": []string{"en"}, "use_cases": []string{"support"},
		"capability_requirements": []string{"model", "knowledge"}, "minimum_weknora_capability": "1", "license_id": "MIT"}
}

func TestLifecycleEndpointsHappyPathOverRealStack(t *testing.T) {
	r, _, _ := newLifecycleTestApp(t)
	listingID, v1 := freezeAndPublishUpgradeRelease(t, r, "1.0.0", lifecycleMetadata())
	_, v2 := freezeAndPublishUpgradeRelease(t, r, "1.1.0", lifecycleMetadata())

	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptionBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptionBody))
	variantID, _ := publishUpgradeVariant(t, r, adoptionBody.Data.ID, "Sales", "gpt-x", "kb-sales")

	retired := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/retire", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, retired.Code, retired.Body.String())
	require.Contains(t, retired.Body.String(), `"state":"retired"`)

	ended := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/end", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, ended.Code, ended.Body.String())
	require.Contains(t, ended.Body.String(), `"state":"ended"`)

	unlisted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/listings/"+listingID+"/unlist", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, unlisted.Code, unlisted.Body.String())
	require.Contains(t, unlisted.Body.String(), `"state":"unlisted"`)
	catalog := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/catalog", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, catalog.Code, catalog.Body.String())
	require.NotContains(t, catalog.Body.String(), listingID)

	deprecated := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/releases/"+v1+"/deprecate", "admin", "admin", map[string]any{"successor_release_id": v2})
	require.Equal(t, http.StatusOK, deprecated.Code, deprecated.Body.String())
	var deprecatedBody struct {
		Success bool `json:"success"`
		Data    struct {
			ID                 string `json:"id"`
			ListingID          string `json:"listing_id"`
			SemanticVersion    string `json:"semantic_version"`
			DeprecatedAt       string `json:"deprecated_at"`
			DeprecatedBy       string `json:"deprecated_by"`
			SuccessorReleaseID string `json:"successor_release_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(deprecated.Body.Bytes(), &deprecatedBody))
	require.True(t, deprecatedBody.Success)
	require.Equal(t, v1, deprecatedBody.Data.ID)
	require.Equal(t, listingID, deprecatedBody.Data.ListingID)
	require.Equal(t, "1.0.0", deprecatedBody.Data.SemanticVersion)
	require.NotEmpty(t, deprecatedBody.Data.DeprecatedAt)
	require.Equal(t, "admin", deprecatedBody.Data.DeprecatedBy)
	require.Equal(t, v2, deprecatedBody.Data.SuccessorReleaseID)
	repeat := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/releases/"+v1+"/deprecate", "admin", "admin", map[string]any{"successor_release_id": v2})
	require.Equal(t, http.StatusConflict, repeat.Code, repeat.Body.String())

	require.Equal(t, http.StatusNotFound, adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/retire", "admin", "admin", nil).Code)
	require.Equal(t, http.StatusNotFound, adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/end", "admin", "admin", nil).Code)
	require.Equal(t, http.StatusNotFound, adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/listings/"+listingID+"/unlist", "admin", "admin", nil).Code)
	require.Equal(t, http.StatusNotFound, adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/releases/"+v1+"/deprecate", "admin", "admin", map[string]any{"successor_release_id": v2}).Code)
}

func TestLifecycleHTTPAuthorizationAndDeprecateBodyValidation(t *testing.T) {
	r, _, _ := newLifecycleTestApp(t)
	_, releaseID := freezeAndPublishUpgradeRelease(t, r, "1.0.0", lifecycleMetadata())
	path := "/api/v1/marketplace/tenant/releases/" + releaseID + "/deprecate"

	forbidden := adoptionCall(r, 1, http.MethodPost, path, "viewer", "viewer", map[string]any{"successor_release_id": "next"})
	require.Equal(t, http.StatusForbidden, forbidden.Code, forbidden.Body.String())
	missing := adoptionCall(r, 1, http.MethodPost, path, "admin", "admin", map[string]any{})
	require.Equal(t, http.StatusBadRequest, missing.Code, missing.Body.String())
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(bytes.Repeat([]byte(" "), (1<<20)+1)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Test-Role", "admin")
	request.Header.Set("X-Test-Actor", "admin")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
}
