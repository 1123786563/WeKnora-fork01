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

// newPublicMarketplaceTestApp mounts the REAL cross-tenant stack over the
// real migration stream: publisher tenant 1's release workflow, the
// platform public marketplace, tenant 2's adoption chain and the real
// GET /api/v1/agents list the mobile Resource Shelf consumes. The
// X-Test-System-Admin header injects the platform reviewer identity the
// same way the other tests inject tenant roles.
func newPublicMarketplaceTestApp(t *testing.T) (*gin.Engine, *rbacGuards, *gorm.DB) {
	t.Helper()
	db := openTenantAgentMarketplaceHTTPTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{
		ID: "agent-owned", Name: "Release helper", TenantID: 1, CreatedBy: "contributor",
		Config: types.CustomAgentConfig{AgentMode: "smart-reasoning", SystemPrompt: "Be portable.", KnowledgeBases: []string{"kb-publisher"}, ModelID: "model-publisher"},
	}).Error)

	marketRepo := repository.NewAgentMarketplaceRepository(db)
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil)
	versions := service.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	market := service.NewAgentMarketplaceService(versions, marketplaceHTTPResolver{}, marketRepo, t.TempDir())
	adoptions := service.NewAgentAdoptionService(repository.NewAgentAdoptionRepository(db), customAgents, versions)
	public := service.NewPublicMarketplaceService(repository.NewPublicMarketplaceRepository(db), marketRepo)

	versionHandler := handler.NewAgentVersionHandler(versions)
	marketHandler := handler.NewAgentMarketplaceHandler(market, versions)
	adoptionHandler := handler.NewAgentAdoptionHandler(adoptions)
	publicHandler := handler.NewPublicMarketplaceHandler(public)
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
		if c.GetHeader("X-Test-System-Admin") == "1" {
			ctx = context.WithValue(ctx, types.SystemAdminContextKey, true)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), tenantID)
		c.Next()
	})
	v1 := r.Group("/api/v1")
	RegisterAgentVersionRoutes(v1, versionHandler, g)
	RegisterAgentMarketplaceRoutes(v1, marketHandler, g)
	RegisterAgentAdoptionRoutes(v1, adoptionHandler, g)
	RegisterPublicMarketplaceRoutes(v1, publicHandler, g)
	RegisterCustomAgentRoutes(v1, agentListHandler, g)
	return r, g, db
}

func publicCall(r *gin.Engine, tenantID uint64, systemAdmin bool, method, path, role, actor string, body any) *httptest.ResponseRecorder {
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
	if systemAdmin {
		req.Header.Set("X-Test-System-Admin", "1")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// differentDigest flips the last hex char of a valid digest, yielding a
// DIFFERENT value that still passes the server's sha-256 format check.
func differentDigest(digest string) string {
	last := digest[len(digest)-1]
	replacement := "0"
	if last == '0' {
		replacement = "1"
	}
	return digest[:len(digest)-1] + replacement
}

// publishTenantRelease drives tenant 1's REAL tenant-release workflow and
// returns the listing id whose current release is portable.
func publishTenantRelease(t *testing.T, r *gin.Engine) string {
	t.Helper()
	frozen := publicCall(r, 1, false, http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
	require.Equal(t, http.StatusCreated, frozen.Code, frozen.Body.String())
	var frozenBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(frozen.Body.Bytes(), &frozenBody))
	require.NotEmpty(t, frozenBody.Data.ID)

	metadata := map[string]any{
		"semantic_version": "1.0.0", "display_name": "Public helper", "summary": "A portable helper",
		"supported_languages": []string{"en"}, "use_cases": []string{"support"},
		"capability_requirements":    []string{"knowledge"},
		"minimum_weknora_capability": "1", "license_id": "MIT",
	}
	submitted := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "contributor", "contributor", map[string]any{"agent_version_id": frozenBody.Data.ID, "metadata": metadata})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID           string `json:"id"`
			ListingID    string `json:"listing_id"`
			BundleDigest string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))

	reviewed := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "tenant-reviewer", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusOK, reviewed.Code, reviewed.Body.String())
	return submissionBody.Data.ListingID
}

func TestPublicMarketplacePublisherVerificationAndSubmissionAuthorization(t *testing.T) {
	r, g, db := newPublicMarketplaceTestApp(t)
	_ = g
	listingID := publishTenantRelease(t, r)

	// Verified Publisher 名册：仅 SystemAdmin
	forbidden := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "tenant-admin", map[string]any{"tenant_id": 1})
	require.Equal(t, http.StatusForbidden, forbidden.Code, forbidden.Body.String())

	verified := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1, "note": "identity checked"})
	require.Equal(t, http.StatusCreated, verified.Code, verified.Body.String())
	reverified := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1})
	require.Equal(t, http.StatusOK, reverified.Code, reverified.Body.String())

	list := publicCall(r, 1, true, http.MethodGet, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), `"tenant_id":1`)

	revoked := publicCall(r, 1, true, http.MethodDelete, "/api/v1/marketplace/public/verified-publishers/1", "admin", "sysadmin", nil)
	require.Equal(t, http.StatusOK, revoked.Code, revoked.Body.String())
	revokedAgain := publicCall(r, 1, true, http.MethodDelete, "/api/v1/marketplace/public/verified-publishers/1", "admin", "sysadmin", nil)
	require.Equal(t, http.StatusOK, revokedAgain.Code, revokedAgain.Body.String(), "重复撤销幂等（行仍在，state=revoked）")

	// 未验证发布者不得提交（撤销后同样拒绝）
	denied := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", map[string]any{"source_listing_id": listingID})
	require.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
	var count int64
	db.Table("public_release_submissions").Count(&count)
	require.Zero(t, count)

	// 重新验证后：Contributor 仍 403（Admin 门禁），Admin 201
	_ = publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1})
	contributor := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "contributor", "contributor", map[string]any{"source_listing_id": listingID})
	require.Equal(t, http.StatusForbidden, contributor.Code, contributor.Body.String())
	submitted := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", map[string]any{"source_listing_id": listingID})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID              string `json:"id"`
			PublicListingID string `json:"public_listing_id"`
			BundleDigest    string `json:"bundle_digest"`
			Status          string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))
	require.Equal(t, "submitted", submissionBody.Data.Status)

	// 发布者自查询只含本租户提交
	own := publicCall(r, 1, false, http.MethodGet, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", nil)
	require.Equal(t, http.StatusOK, own.Code, own.Body.String())
	require.Contains(t, own.Body.String(), submissionBody.Data.ID)
}

func TestPublicMarketplaceReviewAuthorization(t *testing.T) {
	r, _, _ := newPublicMarketplaceTestApp(t)
	listingID := publishTenantRelease(t, r)
	_ = publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1})
	submitted := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", map[string]any{"source_listing_id": listingID})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID           string `json:"id"`
			BundleDigest string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))

	// 审核队列与审核决定：仅 SystemAdmin（租户 Admin 403）
	tenantQueue := publicCall(r, 1, false, http.MethodGet, "/api/v1/marketplace/public/release-submissions/review-queue", "admin", "tenant-admin", nil)
	require.Equal(t, http.StatusForbidden, tenantQueue.Code, tenantQueue.Body.String())
	tenantReview := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "tenant-admin", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusForbidden, tenantReview.Code, tenantReview.Body.String())

	queue := publicCall(r, 1, true, http.MethodGet, "/api/v1/marketplace/public/release-submissions/review-queue", "admin", "sysadmin", nil)
	require.Equal(t, http.StatusOK, queue.Code, queue.Body.String())
	require.Contains(t, queue.Body.String(), submissionBody.Data.ID)

	// stale digest → 409
	stale := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "sysadmin", map[string]any{"expected_digest": differentDigest(submissionBody.Data.BundleDigest), "decision": "approved"})
	require.Equal(t, http.StatusConflict, stale.Code, stale.Body.String())

	// 无理由拒绝 → 400
	noReason := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "sysadmin", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "rejected"})
	require.Equal(t, http.StatusBadRequest, noReason.Code, noReason.Body.String())

	approved := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "sysadmin", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())

	// 重复审核 → 409
	duplicate := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "sysadmin", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "rejected", "reason": "late"})
	require.Equal(t, http.StatusConflict, duplicate.Code, duplicate.Body.String())
}
