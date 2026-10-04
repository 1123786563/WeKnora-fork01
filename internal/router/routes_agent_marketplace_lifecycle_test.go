package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	session "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/middleware"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
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
	require.NoError(t, db.Exec("INSERT INTO tenants (id, name, business) VALUES (1, 'lifecycle-e2e', 'test')").Error)
	// Admit 的 actor 校验要求 actor 为本租户活跃成员（users JOIN tenant_members）。
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, is_active) VALUES ('admin', 'admin', 'admin@lifecycle.test', 'x', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (1, 'admin', 'admin', 'active')`).Error)
	require.NoError(t, db.Create(&types.CustomAgent{ID: "agent-owned", Name: "Lifecycle helper", TenantID: 1, CreatedBy: "contributor", Config: types.CustomAgentConfig{AgentMode: "smart-reasoning", SystemPrompt: "Be useful."}}).Error)
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil, nil)
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

// newLifecycleCoordinator mirrors the container wiring (server-owned binding
// resolver + security gate + published-version pins), see internal/container/workbench.go.
func newLifecycleCoordinator(t *testing.T, db *gorm.DB) *workbenchservice.AdmissionCoordinator {
	t.Helper()
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil, nil)
	versions := service.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	security := service.NewAgentSecurityService(repository.NewAgentSecurityStore(db), repository.NewAgentRunStore(db))
	security.SetAgentVersionService(versions)
	coordinator, err := workbenchservice.NewAdmissionCoordinatorWithBinding(db, repository.NewAgentRunStore(db), workbenchservice.NewDurableTaskBudget(db), nil, workbenchservice.NewDatabaseAdmissionBindingResolver(nil))
	require.NoError(t, err)
	coordinator.SetAgentSecurityGate(security)
	coordinator.SetPublishedAgentVersionResolver(security)
	return coordinator
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

// decodeAdoptionID reads the adoption id from an Adopt/List response row.
func decodeAdoptionID(t *testing.T, raw []byte) string {
	t.Helper()
	var body struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.NotEmpty(t, body.Data.ID)
	return body.Data.ID
}

// adoptAndPublishFirstVariant adopts listingID then drives the variant to
// published via the existing publishUpgradeVariant helper; returns the
// adoption id, variant id and instantiated local agent id.
func adoptAndPublishFirstVariant(t *testing.T, r *gin.Engine, listingID string) (adoptionID, variantID, localAgentID string) {
	t.Helper()
	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	adoptionID = decodeAdoptionID(t, adopted.Body.Bytes())
	variantID, localAgentID = publishUpgradeVariant(t, r, adoptionID, "Sales", "gpt-x", "kb-sales")
	return adoptionID, variantID, localAgentID
}

// httptestPostJSON posts a JSON body through the real engine.
func httptestPostJSON(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", "admin")
	req.Header.Set("X-Test-Actor", "admin")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// newRealAgentUseGate wraps the repository predicate in the same seam used by
// the production workbench wiring.
func newRealAgentUseGate(db *gorm.DB) func(context.Context, uint64, string) error {
	adoptions := repository.NewAgentAdoptionRepository(db)
	return func(ctx context.Context, tenantID uint64, agentID string) error {
		retired, err := adoptions.RetiredVariantAgentExists(ctx, tenantID, agentID)
		if err != nil {
			return err
		}
		if retired {
			return fmt.Errorf("%w: agent %s belongs to a retired variant", workbenchservice.ErrAgentUseDenied, agentID)
		}
		return nil
	}
}

func seedLifecycleWorkbenchSessions(t *testing.T, db *gorm.DB, ids ...string) {
	t.Helper()
	for _, id := range ids {
		require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, 1, ?, 'admin', 'trpc')", id, id).Error)
	}
}

func submissionIDs(rows []types.AgentReleaseSubmissionEntity) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func TestLifecycleExitDeletesNothingAcrossGovernanceRows(t *testing.T) {
	r, _, db := newLifecycleTestApp(t)
	listingID, v1 := freezeAndPublishUpgradeRelease(t, r, "1.0.0", lifecycleMetadata())
	adoptionID, variantID, localAgentID := adoptAndPublishFirstVariant(t, r, listingID)
	_, v2 := freezeAndPublishUpgradeRelease(t, r, "2.0.0", lifecycleMetadata())

	// Preserve a real Task/Run row across the lifecycle exits, so these counts
	// prove historical work remains instead of comparing two empty tables.
	seedLifecycleWorkbenchSessions(t, db, "lifecycle-history")
	coordinator := newLifecycleCoordinator(t, db)
	accepted, err := coordinator.Start(context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "admin"), workbenchservice.StartInput{
		SessionID: "lifecycle-history", AgentID: localAgentID, TargetID: "platform", RequestID: "lifecycle-history-request", Text: "historical task", BudgetUpper: 100,
	})
	require.NoError(t, err)
	require.NotEmpty(t, accepted.Key.RunID)
	require.NoError(t, db.Exec(`INSERT INTO artifact_versions (tenant_id, id, run_id, session_id, digest, object_key, mime, size, scan_state) VALUES (1, 'lifecycle-artifact', ?, 'lifecycle-history', 'digest', 'artifact/lifecycle', 'text/plain', 1, 'ready')`, accepted.Key.RunID).Error)

	// Capture the exact history rows and their links before applying the exits.
	var seededVariant types.AgentAdoptionVariantEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, variantID).Take(&seededVariant).Error)
	var seededAdoption types.AgentAdoptionEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, adoptionID).Take(&seededAdoption).Error)
	var seededReleases []types.AgentReleaseEntity
	require.NoError(t, db.Where("tenant_id = ? AND id IN ?", 1, []string{v1, v2}).Find(&seededReleases).Error)
	require.Len(t, seededReleases, 2)
	var seededSubmissions []types.AgentReleaseSubmissionEntity
	require.NoError(t, db.Where("tenant_id = ? AND listing_id = ?", 1, listingID).Find(&seededSubmissions).Error)
	require.NotEmpty(t, seededSubmissions)
	var seededReviews []types.AgentReleaseReviewEntity
	require.NoError(t, db.Where("tenant_id = ? AND submission_id IN ?", 1, submissionIDs(seededSubmissions)).Find(&seededReviews).Error)
	require.NotEmpty(t, seededReviews)
	require.NoError(t, db.Create(&types.AgentLicenseEntity{ID: "MIT", Name: "MIT", AllowsRedistribution: true, CreatedBy: "admin"}).Error)
	var seededLicense types.AgentLicenseEntity
	require.NoError(t, db.Where("id = ?", "MIT").Take(&seededLicense).Error)
	var seededListing types.AgentMarketplaceListingEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, listingID).Take(&seededListing).Error)
	var seededAgent types.CustomAgent
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, localAgentID).Take(&seededAgent).Error)
	var seededRun map[string]any
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, accepted.Key.RunID).Take(&seededRun).Error)
	var seededArtifact map[string]any
	require.NoError(t, db.Table("artifact_versions").Where("tenant_id = ? AND id = ?", 1, "lifecycle-artifact").Take(&seededArtifact).Error)

	count := func(table string) int64 {
		t.Helper()
		var n int64
		if table == "agent_licenses" {
			require.NoError(t, db.Table(table).Count(&n).Error)
		} else {
			require.NoError(t, db.Table(table).Where("tenant_id = ?", 1).Count(&n).Error)
		}
		return n
	}
	before := map[string]int64{
		"agent_releases": count("agent_releases"), "agent_release_reviews": count("agent_release_reviews"),
		"agent_versions": count("agent_versions"), "agent_adoptions": count("agent_adoptions"),
		"agent_adoption_variants": count("agent_adoption_variants"), "custom_agents": count("custom_agents"),
		"agent_licenses": count("agent_licenses"), "agent_release_submissions": count("agent_release_submissions"),
		"agent_runs": count("agent_runs"), "workbench_requests": count("workbench_requests"), "artifact_versions": count("artifact_versions"),
	}

	require.Equal(t, http.StatusOK, adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/retire", "admin", "admin", nil).Code)
	require.Equal(t, http.StatusOK, adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionID+"/end", "admin", "admin", nil).Code)
	require.Equal(t, http.StatusOK, adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/listings/"+listingID+"/unlist", "admin", "admin", nil).Code)
	deprecated := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/releases/"+v2+"/deprecate", "admin", "admin", map[string]any{"successor_release_id": v1})
	require.Equal(t, http.StatusOK, deprecated.Code, deprecated.Body.String())

	for table, n := range before {
		require.EqualValues(t, n, count(table), "退出不得删除 %s 行", table)
	}
	var runAfter map[string]any
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, accepted.Key.RunID).Take(&runAfter).Error)
	require.Equal(t, "lifecycle-history", runAfter["session_id"])
	var requestAfter map[string]any
	require.NoError(t, db.Table("workbench_requests").Where("tenant_id = ? AND request_id = ?", 1, "lifecycle-history-request").Take(&requestAfter).Error)
	require.Equal(t, accepted.Key.RunID, requestAfter["run_id"])
	require.Equal(t, localAgentID, requestAfter["agent_id"])
	var artifactAfter map[string]any
	require.NoError(t, db.Table("artifact_versions").Where("tenant_id = ? AND id = ?", 1, "lifecycle-artifact").Take(&artifactAfter).Error)
	require.Equal(t, accepted.Key.RunID, artifactAfter["run_id"])
	var variantAfter types.AgentAdoptionVariantEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, seededVariant.ID).Take(&variantAfter).Error)
	require.Equal(t, adoptionID, variantAfter.AdoptionID)
	require.Equal(t, seededVariant.ReleaseID, variantAfter.ReleaseID)
	require.Contains(t, []string{v1, v2}, variantAfter.ReleaseID)
	require.Equal(t, localAgentID, variantAfter.LocalAgentID)
	require.Equal(t, seededVariant.LocalAgentVersionID, variantAfter.LocalAgentVersionID)
	var adoptionAfter types.AgentAdoptionEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, adoptionID).Take(&adoptionAfter).Error)
	require.Equal(t, seededAdoption.ListingID, adoptionAfter.ListingID)
	require.Equal(t, seededAdoption.AcceptedReleaseID, adoptionAfter.AcceptedReleaseID)
	require.Equal(t, "ended", adoptionAfter.State)
	var versionAfter types.AgentVersionEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, seededVariant.LocalAgentVersionID).Take(&versionAfter).Error)
	require.Equal(t, localAgentID, versionAfter.AgentID)
	for _, release := range seededReleases {
		var releaseAfter types.AgentReleaseEntity
		require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, release.ID).Take(&releaseAfter).Error)
		require.Equal(t, listingID, releaseAfter.ListingID)
		require.Equal(t, release.SubmissionID, releaseAfter.SubmissionID)
		require.Equal(t, release.AgentVersionID, releaseAfter.AgentVersionID, "release source version must remain unchanged")
		var submissionAfter types.AgentReleaseSubmissionEntity
		require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, releaseAfter.SubmissionID).Take(&submissionAfter).Error)
		require.Equal(t, releaseAfter.AgentVersionID, submissionAfter.AgentVersionID)
		var sourceVersion types.AgentVersionEntity
		require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, releaseAfter.AgentVersionID).Take(&sourceVersion).Error)
		require.Equal(t, releaseAfter.SourceAgentID, sourceVersion.AgentID)
		var manifest struct {
			LicenseID string `json:"license_id"`
		}
		require.NoError(t, json.Unmarshal([]byte(releaseAfter.ManifestJSON), &manifest))
		require.Equal(t, seededLicense.ID, manifest.LicenseID)
	}
	for _, submission := range seededSubmissions {
		var submissionAfter types.AgentReleaseSubmissionEntity
		require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, submission.ID).Take(&submissionAfter).Error)
		require.Equal(t, listingID, submissionAfter.ListingID)
		require.Equal(t, submission.AgentVersionID, submissionAfter.AgentVersionID, "submission source version must remain unchanged")
		for _, review := range seededReviews {
			if review.SubmissionID != submission.ID {
				continue
			}
			var reviewAfter types.AgentReleaseReviewEntity
			require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, review.ID).Take(&reviewAfter).Error)
			require.Equal(t, submission.ID, reviewAfter.SubmissionID)
		}
	}
	var licenseAfter types.AgentLicenseEntity
	require.NoError(t, db.Where("id = ?", seededLicense.ID).Take(&licenseAfter).Error)
	require.Equal(t, seededLicense.Name, licenseAfter.Name)
	var listingAfter types.AgentMarketplaceListingEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, seededListing.ID).Take(&listingAfter).Error)
	require.Equal(t, listingID, listingAfter.ID)
	require.Equal(t, seededListing.SourceAgentID, listingAfter.SourceAgentID)
	var deletedCount int64
	require.NoError(t, db.Table("custom_agents").Where("id = ? AND deleted_at IS NOT NULL", localAgentID).Count(&deletedCount).Error)
	require.Zero(t, deletedCount)
	var liveAgentCount int64
	require.NoError(t, db.Table("custom_agents").Where("id = ? AND deleted_at IS NULL", localAgentID).Count(&liveAgentCount).Error)
	require.EqualValues(t, 1, liveAgentCount, "the published local Agent must remain usable after lifecycle exits")
}

func TestLifecycleUnlistedVersusDeprecatedBehaviorDiffers(t *testing.T) {
	r, _, _ := newLifecycleTestApp(t)
	meta := lifecycleMetadata()
	listingID, _ := freezeAndPublishUpgradeRelease(t, r, "1.0.0", meta)

	unlistedAdopter := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, unlistedAdopter.Code)
	require.Equal(t, http.StatusOK, adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/listings/"+listingID+"/unlist", "admin", "admin", nil).Code)
	catalog := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/catalog", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, catalog.Code)
	require.NotContains(t, catalog.Body.String(), listingID)
	reAdopt := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusBadRequest, reAdopt.Code)
	_, v3 := freezeAndPublishUpgradeRelease(t, r, "1.2.0", meta)
	proposals := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	// G2 裁决（task-2 复审）：unlisted = 发行方全面撤回，reconcile 不再物化新
	// 升级提案（无错跳过）；与 deprecated 保持可见带 reason/replacement 不同。
	require.Equal(t, http.StatusOK, proposals.Code, proposals.Body.String())
	require.Contains(t, proposals.Body.String(), `"data":[]`)
	require.NotContains(t, proposals.Body.String(), `"to_release_id":"`+v3+`"`)

	// Use a fresh real-stack fixture: each Release published from one source
	// Agent reuses its Listing, and the first scenario intentionally unlisted it.
	r2, _, _ := newLifecycleTestApp(t)
	listing2, w1 := freezeAndPublishUpgradeRelease(t, r2, "2.0.0", lifecycleMetadata())
	_, w2 := freezeAndPublishUpgradeRelease(t, r2, "2.1.0", lifecycleMetadata())
	require.Equal(t, http.StatusOK, adoptionCall(r2, 1, http.MethodPost, "/api/v1/marketplace/tenant/releases/"+w1+"/deprecate", "admin", "admin", map[string]any{"successor_release_id": w2}).Code)
	catalog2 := adoptionCall(r2, 1, http.MethodGet, "/api/v1/marketplace/tenant/catalog", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, catalog2.Code)
	require.Contains(t, catalog2.Body.String(), listing2)
	adoptCurrent := adoptionCall(r2, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listing2})
	require.Equal(t, http.StatusCreated, adoptCurrent.Code, adoptCurrent.Body.String())
	adoptDeprecated := adoptionCall(r2, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listing2, "release_id": w1})
	require.Equal(t, http.StatusConflict, adoptDeprecated.Code)
	require.Contains(t, adoptDeprecated.Body.String(), w2)
	variantOnDeprecated := adoptionCall(r2, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+decodeAdoptionID(t, adoptCurrent.Body.Bytes())+"/variants", "admin", "admin", map[string]any{"name": "x", "release_id": w1})
	require.Equal(t, http.StatusConflict, variantOnDeprecated.Code)
}

func TestLifecycleRetireBlocksNewWorkEndToEnd(t *testing.T) {
	r, _, db := newLifecycleTestApp(t)
	listingID, _ := freezeAndPublishUpgradeRelease(t, r, "1.0.0", lifecycleMetadata())
	_, variantID, localAgentID := adoptAndPublishFirstVariant(t, r, listingID)

	available := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Contains(t, available.Body.String(), localAgentID)
	seedLifecycleWorkbenchSessions(t, db, "retire-admission-before", "retire-admission-after")
	coordinator := newLifecycleCoordinator(t, db)
	coordinator.SetAgentUseGate(newRealAgentUseGate(db))
	r.POST("/api/v1/workbench/executions", newLifecycleIdentityMiddleware(), session.NewWorkbenchStartHandler(coordinator).Start)
	startBody := func(sessionID, requestID string) string {
		return `{"session_id":"` + sessionID + `","agent_id":"` + localAgentID + `","target_id":"platform","request_id":"` + requestID + `","text":"hi","budget_upper":100}`
	}
	resp := httptestPostJSON(t, r, "/api/v1/workbench/executions", startBody("retire-admission-before", "req-before"))
	require.Equal(t, http.StatusAccepted, resp.Code, resp.Body.String())

	require.Equal(t, http.StatusOK, adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/retire", "admin", "admin", nil).Code)
	available2 := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.NotContains(t, available2.Body.String(), localAgentID)
	resp2 := httptestPostJSON(t, r, "/api/v1/workbench/executions", startBody("retire-admission-after", "req-after"))
	require.Equal(t, http.StatusConflict, resp2.Code, resp2.Body.String())
	require.Contains(t, resp2.Body.String(), "retired")
	var requests, runs int64
	require.NoError(t, db.Table("workbench_requests").Where("tenant_id = ? AND request_id = ?", 1, "req-after").Count(&requests).Error)
	require.Zero(t, requests, "denied retired-agent start must not create a durable request")
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND session_id = ?", 1, "retire-admission-after").Count(&runs).Error)
	require.Zero(t, runs, "denied retired-agent start must not create a run")
}
