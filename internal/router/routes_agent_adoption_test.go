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
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil, nil)
	versions := service.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	market := service.NewAgentMarketplaceService(versions, marketplaceHTTPResolver{}, marketRepo, t.TempDir())
	adoptions := service.NewAgentAdoptionService(repository.NewAgentAdoptionRepository(db), customAgents, versions)

	versionHandler := handler.NewAgentVersionHandler(versions)
	marketHandler := handler.NewAgentMarketplaceHandler(market, versions)
	adoptionHandler := handler.NewAgentAdoptionHandler(adoptions)
	agentListHandler := handler.NewCustomAgentHandler(customAgents, nil, repository.NewTenantDisabledSharedAgentRepository(db), nil, nil, service.HostSandboxManager{})

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
		"capability_requirements":    []string{"model", "knowledge"},
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

func TestTenantAgentVariantCapabilityMappingAndTestGate(t *testing.T) {
	r, _, _ := newAgentAdoptionTestApp(t)
	listingID, _ := publishAdoptionRelease(t, r)
	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptionBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptionBody))
	variant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": "Sales Assistant"})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID                  string   `json:"id"`
			State               string   `json:"state"`
			MissingCapabilities []string `json:"missing_capabilities"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))

	assertConflict := func(response *httptest.ResponseRecorder, message string) {
		t.Helper()
		require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
		var envelope struct {
			Success bool `json:"success"`
			Error   struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		require.False(t, envelope.Success)
		require.Equal(t, 1005, envelope.Error.Code)
		require.Equal(t, message, envelope.Error.Message)
	}

	// AC2 over HTTP: missing required capabilities -> 409 with every missing
	// capability named in the message.
	assertConflict(
		adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "admin", nil),
		"agent adoption variant is not runnable: missing required capabilities: knowledge, model",
	)

	partial := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{{"capability": "model", "model_id": "gpt-x"}}})
	require.Equal(t, http.StatusOK, partial.Code, partial.Body.String())
	require.NoError(t, json.Unmarshal(partial.Body.Bytes(), &variantBody))
	require.Equal(t, "draft", variantBody.Data.State)
	require.Equal(t, []string{"knowledge"}, variantBody.Data.MissingCapabilities)
	assertConflict(
		adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "admin", nil),
		"agent adoption variant is not runnable: missing required capabilities: knowledge",
	)

	unknownCapability := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{{"capability": "sandbox"}}})
	require.Equal(t, http.StatusBadRequest, unknownCapability.Code, unknownCapability.Body.String())
	duplicateCapability := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{{"capability": "model", "model_id": "a"}, {"capability": "model", "model_id": "b"}}})
	require.Equal(t, http.StatusBadRequest, duplicateCapability.Code)
	emptyBinding := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{{"capability": "model"}, {"capability": "knowledge"}}})
	require.Equal(t, http.StatusOK, emptyBinding.Code)
	require.NoError(t, json.Unmarshal(emptyBinding.Body.Bytes(), &variantBody))
	require.Equal(t, []string{"knowledge", "model"}, variantBody.Data.MissingCapabilities, "empty bindings never cover a capability")
	viewerMapping := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "viewer", "viewer", map[string]any{"mappings": []map[string]any{}})
	require.Equal(t, http.StatusForbidden, viewerMapping.Code)
	crossTenantMapping := adoptionCall(r, 2, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{}})
	require.Equal(t, http.StatusNotFound, crossTenantMapping.Code)
	viewerTest := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "viewer", "viewer", nil)
	require.Equal(t, http.StatusForbidden, viewerTest.Code)

	complete := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{
		{"capability": "model", "model_id": "gpt-x"},
		{"capability": "knowledge", "knowledge_base_ids": []string{"kb-sales"}},
	}})
	require.Equal(t, http.StatusOK, complete.Code, complete.Body.String())
	require.NoError(t, json.Unmarshal(complete.Body.Bytes(), &variantBody))
	require.Equal(t, "mapped", variantBody.Data.State)
	require.Empty(t, variantBody.Data.MissingCapabilities)

	tested := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, tested.Code, tested.Body.String())
	require.NoError(t, json.Unmarshal(tested.Body.Bytes(), &variantBody))
	require.Equal(t, "tested", variantBody.Data.State)
	// Re-testing a tested variant is a state conflict, not a silent re-run.
	assertConflict(
		adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "admin", nil),
		"agent adoption variant state conflict: state is \"tested\"; complete capability mapping first",
	)
}

func TestTenantAgentAdoptionPublishesIndependentVariantsIntoMobileAvailableAgents(t *testing.T) {
	r, _, db := newAgentAdoptionTestApp(t)
	listingID, releaseID := publishAdoptionRelease(t, r)
	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptionBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptionBody))

	// AC2 over HTTP (publish gate): an un-mapped variant refuses publication
	// with the missing capabilities named.
	draft := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": "Draft Gate Probe"})
	require.Equal(t, http.StatusCreated, draft.Code, draft.Body.String())
	var draftBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(draft.Body.Bytes(), &draftBody))
	draftPublish := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+draftBody.Data.ID+"/publish", "admin", "admin", nil)
	require.Equal(t, http.StatusConflict, draftPublish.Code, draftPublish.Body.String())
	require.Contains(t, draftPublish.Body.String(), "missing required capabilities: knowledge, model")

	mapAndTest := func(name, model, kb string) string {
		variant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": name})
		require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
		var variantBody struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))
		mapping := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{
			{"capability": "model", "model_id": model},
			{"capability": "knowledge", "knowledge_base_ids": []string{kb}},
		}})
		require.Equal(t, http.StatusOK, mapping.Code, mapping.Body.String())
		tested := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "admin", nil)
		require.Equal(t, http.StatusOK, tested.Code, tested.Body.String())
		return variantBody.Data.ID
	}

	salesID := mapAndTest("Sales Assistant", "gpt-x", "kb-sales")
	legalID := mapAndTest("Legal Assistant", "gpt-legal", "kb-legal")

	// Nothing is published yet: the mobile agent projection carries no
	// variant agents and the read model is empty (the "not runnable" side
	// of the shelf gate — the agents simply do not exist as runnable).
	agentsBefore := adoptionCall(r, 1, http.MethodGet, "/api/v1/agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, agentsBefore.Code, agentsBefore.Body.String())
	require.NotContains(t, agentsBefore.Body.String(), "Sales Assistant")
	require.NotContains(t, agentsBefore.Body.String(), "Legal Assistant")
	availableBefore := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, availableBefore.Code, availableBefore.Body.String())
	var availableBeforeBody struct {
		Data []struct {
			AgentID string `json:"agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(availableBefore.Body.Bytes(), &availableBeforeBody))
	require.Empty(t, availableBeforeBody.Data)

	publish := func(variantID string) (localAgentID, localVersionID string) {
		published := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/publish", "admin", "admin", nil)
		require.Equal(t, http.StatusOK, published.Code, published.Body.String())
		// The publish response envelope is the unwrapped variant body
		// (B3-F86): data IS the variant, same as the other variant
		// endpoints — pinned by TestPublishVariantReturnsVariantBodyDirectly.
		var publishBody struct {
			Data struct {
				ID                  string `json:"id"`
				State               string `json:"state"`
				LocalAgentID        string `json:"local_agent_id"`
				LocalAgentVersionID string `json:"local_agent_version_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(published.Body.Bytes(), &publishBody))
		require.Equal(t, "published", publishBody.Data.State)
		require.NotEmpty(t, publishBody.Data.LocalAgentID)
		require.NotEmpty(t, publishBody.Data.LocalAgentVersionID)
		return publishBody.Data.LocalAgentID, publishBody.Data.LocalAgentVersionID
	}
	salesAgentID, salesVersionID := publish(salesID)
	legalAgentID, legalVersionID := publish(legalID)
	require.NotEqual(t, salesAgentID, legalAgentID)

	// AC3 (mobile entry): the REAL GET /api/v1/agents — the endpoint the #33
	// mobile Resource Shelf consumes — now carries both published local
	// agents with the wire fields the api-client maps.
	agentsAfter := adoptionCall(r, 1, http.MethodGet, "/api/v1/agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, agentsAfter.Code, agentsAfter.Body.String())
	type mobileAgentRow struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		IsBuiltin   bool   `json:"is_builtin"`
		Config      struct {
			KnowledgeBases []string `json:"knowledge_bases"`
			ModelID        string   `json:"model_id"`
			SystemPrompt   string   `json:"system_prompt"`
		} `json:"config"`
	}
	var agentsList struct {
		Data []mobileAgentRow `json:"data"`
	}
	require.NoError(t, json.Unmarshal(agentsAfter.Body.Bytes(), &agentsList))
	byID := map[string]struct{}{}
	var salesRow, legalRow *mobileAgentRow
	for i := range agentsList.Data {
		byID[agentsList.Data[i].ID] = struct{}{}
		switch agentsList.Data[i].ID {
		case salesAgentID:
			salesRow = &agentsList.Data[i]
		case legalAgentID:
			legalRow = &agentsList.Data[i]
		}
	}
	require.Contains(t, byID, salesAgentID)
	require.Contains(t, byID, legalAgentID)
	// AC1 (independence): the two variants' local agents carry different
	// local mappings from one shared adoption.
	require.NotNil(t, salesRow)
	require.NotNil(t, legalRow)
	require.Equal(t, "Sales Assistant", salesRow.Name)
	require.Equal(t, "Legal Assistant", legalRow.Name)
	require.Equal(t, []string{"kb-sales"}, salesRow.Config.KnowledgeBases)
	require.Equal(t, "gpt-x", salesRow.Config.ModelID)
	require.Equal(t, []string{"kb-legal"}, legalRow.Config.KnowledgeBases)
	require.Equal(t, "gpt-legal", legalRow.Config.ModelID)
	require.Equal(t, "Be useful.", salesRow.Config.SystemPrompt, "portable payload behavior reaches the local agent")
	require.False(t, salesRow.IsBuiltin)

	// The published local Agent Version is real and readable (#58 endpoint).
	frozenVersion := adoptionCall(r, 1, http.MethodGet, "/api/v1/agents/"+salesAgentID+"/versions/"+salesVersionID, "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, frozenVersion.Code, frozenVersion.Body.String())
	require.Contains(t, frozenVersion.Body.String(), salesVersionID)
	_ = legalVersionID

	// Domain read model: two available agents with full lineage.
	available := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, available.Code, available.Body.String())
	var availableBody struct {
		Data []struct {
			AgentID    string `json:"agent_id"`
			VariantID  string `json:"variant_id"`
			AdoptionID string `json:"adoption_id"`
			ReleaseID  string `json:"release_id"`
			Name       string `json:"name"`
			Capability struct {
				State  string `json:"state"`
				Reason string `json:"reason"`
			} `json:"capability"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(available.Body.Bytes(), &availableBody))
	require.Len(t, availableBody.Data, 2)
	require.Equal(t, salesAgentID, availableBody.Data[0].AgentID)
	require.Equal(t, adoptionBody.Data.ID, availableBody.Data[0].AdoptionID)
	require.Equal(t, releaseID, availableBody.Data[0].ReleaseID)
	require.Equal(t, "supported", availableBody.Data[0].Capability.State)

	// Governance view: one adoption, two published variants.
	listed := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
	require.Contains(t, listed.Body.String(), `"state":"published"`)

	// Review Focus 2: re-publishing refuses and does not duplicate the agent.
	rePublish := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+salesID+"/publish", "admin", "admin", nil)
	require.Equal(t, http.StatusConflict, rePublish.Code, rePublish.Body.String())
	var count int64
	require.NoError(t, db.Model(&types.CustomAgent{}).Where("tenant_id = ? AND name IN ?", 1, []string{"Sales Assistant", "Legal Assistant"}).Count(&count).Error)
	require.Equal(t, int64(2), count, "a refused re-publish must not create another local agent")

	// Review Focus 4: a tampered release bundle refuses publication (500,
	// fail closed) before any local agent is instantiated.
	tamperedID := mapAndTest("Compliance Assistant", "gpt-c", "kb-c")
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("tenant_id = ? AND id = ?", 1, releaseID).Update("bundle", []byte(`{"payload":"tampered"}`)).Error)
	tamperedPublish := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+tamperedID+"/publish", "admin", "admin", nil)
	require.Equal(t, http.StatusInternalServerError, tamperedPublish.Code, tamperedPublish.Body.String())
	require.NoError(t, db.Model(&types.CustomAgent{}).Where("tenant_id = ? AND name = ?", 1, "Compliance Assistant").Count(&count).Error)
	require.Equal(t, int64(0), count, "a tampered release must not instantiate a local agent")

	// Review Focus 5: soft-deleting a published local agent removes it from
	// the read model immediately.
	require.NoError(t, db.Delete(&types.CustomAgent{}, "tenant_id = ? AND id = ?", 1, legalAgentID).Error)
	afterDelete := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, afterDelete.Code)
	require.NotContains(t, afterDelete.Body.String(), legalAgentID)

	viewerPublish := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+salesID+"/publish", "viewer", "viewer", nil)
	require.Equal(t, http.StatusForbidden, viewerPublish.Code)
	crossTenantAvailable := adoptionCall(r, 2, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, crossTenantAvailable.Code)
	var crossTenantAvailableBody struct {
		Data []struct {
			AgentID string `json:"agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(crossTenantAvailable.Body.Bytes(), &crossTenantAvailableBody))
	require.Empty(t, crossTenantAvailableBody.Data, "another tenant never sees tenant-1 availability")
}

func TestAvailableAgentsAPIKeyFloorMatchesAgentList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// The available-agents read model is the mobile read-only surface (spec
	// §2); the API-key floor must mirror GET /api/v1/agents' OR stack instead
	// of admin=full-access (B3-F69).
	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")
	RegisterAgentAdoptionRoutes(v1, handler.NewAgentAdoptionHandler(nil), g)

	policy := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/marketplace/tenant/available-agents")
	require.True(t, policy.RequireFullAccess, "full-access 仍然放行（OR 语义）")
	require.True(t, policyHasCapability(policy, types.APIKeyCapabilityReadAgents), "持 read_agents 能力的集成 key 必须可读移动只读面（B3-F69）")
	require.True(t, policyHasCapability(policy, types.APIKeyCapabilityChat), "chat 能力镜像 GET /api/v1/agents 同样可读")
	require.True(t, policyHasCapability(policy, types.APIKeyCapabilityManageAgents), "manage_agents 同样放行（镜像端点 OR 栈）")
	require.False(t, policyHasCapability(policy, types.APIKeyCapabilityIngest), "无关能力不得放行")

	// 治理写面保持 admin/full-access 地板不变。
	publishPolicy := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/marketplace/tenant/variants/:id/publish")
	require.True(t, publishPolicy.RequireFullAccess, "publish 仍是 full-access 治理写面")
}
