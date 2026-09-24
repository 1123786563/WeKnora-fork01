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

// newAgentAdoptionTestApp mounts the REAL release->adoption stack over the
// real migration stream: real CustomAgentService/AgentVersionService/
// AgentMarketplaceService/AgentAdoptionService and the real agent list
// endpoint (GET /api/v1/agents) that the mobile Resource Shelf consumes.
func newAgentAdoptionTestApp(t *testing.T) (*gin.Engine, *rbacGuards, *gorm.DB) {
	t.Helper()
	db := openTenantAgentMarketplaceHTTPTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{
		ID: "agent-owned", Name: "Release helper", TenantID: 1, CreatedBy: "contributor",
		Config: types.CustomAgentConfig{AgentMode: "smart-reasoning", SystemPrompt: "Be useful."},
	}).Error)

	marketRepo := repository.NewAgentMarketplaceRepository(db)
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil)
	versions := service.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	market := service.NewAgentMarketplaceService(versions, marketplaceHTTPResolver{}, marketRepo, t.TempDir())
	adoptions := service.NewAgentAdoptionService(repository.NewAgentAdoptionRepository(db), customAgents, versions)

	versionHandler := handler.NewAgentVersionHandler(versions)
	marketHandler := handler.NewAgentMarketplaceHandler(market, versions)
	adoptionHandler := handler.NewAgentAdoptionHandler(adoptions)
	agentListHandler := handler.NewCustomAgentHandler(customAgents, nil, repository.NewTenantDisabledSharedAgentRepository(db), nil, nil)

	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}, agentCreator: func(c *gin.Context) (string, error) {
		if c.Param("id") == "agent-owned" {
			return "contributor", nil
		}
		return "", nil
	}}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
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
	})
	v1 := r.Group("/api/v1")
	RegisterAgentVersionRoutes(v1, versionHandler, g)
	RegisterAgentMarketplaceRoutes(v1, marketHandler, g)
	RegisterAgentAdoptionRoutes(v1, adoptionHandler, g)
	RegisterCustomAgentRoutes(v1, agentListHandler, g)
	return r, g, db
}

func adoptionCall(r *gin.Engine, tenantID uint64, method, path, role, actor string, body any) *httptest.ResponseRecorder {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", role)
	req.Header.Set("X-Test-Actor", actor)
	if tenantID == 2 {
		req.Header.Set("X-Test-Tenant", "2")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// publishAdoptionRelease drives the real HTTP release workflow and returns
// the listing id plus the published release id.
func publishAdoptionRelease(t *testing.T, r *gin.Engine) (listingID, releaseID string) {
	t.Helper()
	frozen := adoptionCall(r, 1, http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
	require.Equal(t, http.StatusCreated, frozen.Code, frozen.Body.String())
	var frozenBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(frozen.Body.Bytes(), &frozenBody))
	require.NotEmpty(t, frozenBody.Data.ID)

	metadata := map[string]any{
		"semantic_version": "1.0.0", "display_name": "Release helper", "summary": "A helpful agent",
		"supported_languages": []string{"en"}, "use_cases": []string{"support"},
		"capability_requirements": []string{"model", "knowledge"},
		"minimum_weknora_capability": "1", "license_id": "MIT",
	}
	submitted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "contributor", "contributor", map[string]any{"agent_version_id": frozenBody.Data.ID, "metadata": metadata})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID           string `json:"id"`
			ListingID    string `json:"listing_id"`
			BundleDigest string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))

	approved := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "reviewer", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())
	var result struct {
		Data struct {
			Release *struct {
				ID string `json:"id"`
			} `json:"release"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &result))
	require.NotNil(t, result.Data.Release)
	return submissionBody.Data.ListingID, result.Data.Release.ID
}

func TestTenantAgentAdoptionRoutesAndAuthorization(t *testing.T) {
	r, g, _ := newAgentAdoptionTestApp(t)
	listingID, releaseID := publishAdoptionRelease(t, r)

	viewerAdopt := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "viewer", "viewer", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusForbidden, viewerAdopt.Code)
	crossTenantAdopt := adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusNotFound, crossTenantAdopt.Code, "another tenant's listing must read as absent")
	unknownListing := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": "missing-listing"})
	require.Equal(t, http.StatusNotFound, unknownListing.Code)
	badBody := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID, "unexpected": true})
	require.Equal(t, http.StatusBadRequest, badBody.Code)
	spoofed := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID, "tenant_id": 999})
	require.Equal(t, http.StatusBadRequest, spoofed.Code)

	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptionBody struct {
		Data struct {
			ID                string `json:"id"`
			ListingID         string `json:"listing_id"`
			AcceptedReleaseID string `json:"accepted_release_id"`
			State             string `json:"state"`
			Variants          []struct {
				ID string `json:"id"`
			} `json:"variants"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptionBody))
	require.Equal(t, listingID, adoptionBody.Data.ListingID)
	require.Equal(t, releaseID, adoptionBody.Data.AcceptedReleaseID)
	require.Equal(t, "active", adoptionBody.Data.State)
	require.Empty(t, adoptionBody.Data.Variants)

	reAdopt := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusOK, reAdopt.Code)
	require.Contains(t, reAdopt.Body.String(), adoptionBody.Data.ID, "re-adopting is idempotent")

	variant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": "Sales Assistant"})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID                  string   `json:"id"`
			AdoptionID          string   `json:"adoption_id"`
			ReleaseID           string   `json:"release_id"`
			Name                string   `json:"name"`
			State               string   `json:"state"`
			MissingCapabilities []string `json:"missing_capabilities"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))
	require.Equal(t, adoptionBody.Data.ID, variantBody.Data.AdoptionID)
	require.Equal(t, releaseID, variantBody.Data.ReleaseID)
	require.Equal(t, "Sales Assistant", variantBody.Data.Name)
	require.Equal(t, "draft", variantBody.Data.State)
	require.Equal(t, []string{"knowledge", "model"}, variantBody.Data.MissingCapabilities)

	viewerVariant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "viewer", "viewer", map[string]any{"name": "Nope"})
	require.Equal(t, http.StatusForbidden, viewerVariant.Code)
	crossTenantVariant := adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": "Nope"})
	require.Equal(t, http.StatusNotFound, crossTenantVariant.Code)
	emptyName := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": "  "})
	require.Equal(t, http.StatusBadRequest, emptyName.Code)

	list := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), variantBody.Data.ID)
	viewerList := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "viewer", "viewer", nil)
	require.Equal(t, http.StatusForbidden, viewerList.Code)

	adoptPolicy := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/marketplace/tenant/adoptions")
	if !adoptPolicy.RequireFullAccess || len(adoptPolicy.Capabilities) != 0 {
		t.Fatalf("adopt policy = %#v, want full-access only", adoptPolicy)
	}
	variantPolicy := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/:id/variants")
	if !variantPolicy.RequireFullAccess {
		t.Fatalf("variant policy = %#v, want full-access", variantPolicy)
	}
}
