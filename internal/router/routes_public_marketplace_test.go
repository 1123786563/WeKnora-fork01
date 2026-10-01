package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

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

// newPublicMarketplaceTestApp mounts the REAL cross-tenant stack over the
// real migration stream: publisher tenant 1's release workflow, the
// platform public marketplace (T35 #65: + Evaluation/Metrics/#64 security
// surfaces), adopter tenants 2..6's adoption chain, the #64 security
// revocation routes and the workbench execution entry. The X-Test-Tenant
// header accepts any numeric tenant; X-Test-System-Admin injects the
// platform reviewer identity the same way the other tests inject roles.
func newPublicMarketplaceTestApp(t *testing.T) (*gin.Engine, *rbacGuards, *gorm.DB) {
	t.Helper()
	db := openTenantAgentMarketplaceHTTPTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{
		ID: "agent-owned", Name: "Release helper", TenantID: 1, CreatedBy: "contributor",
		Config: types.CustomAgentConfig{AgentMode: "smart-reasoning", SystemPrompt: "Be portable.", KnowledgeBases: []string{"kb-publisher"}, ModelID: "model-publisher"},
	}).Error)
	// Tenant rows are required by the custody tenant guard (49df7e424).
	// Adopter tenants 2..6 carry marker names: the catalog/detail/metrics
	// wires must never surface adopter identity (T35 #65 marker evidence).
	for tenantID := uint64(1); tenantID <= 6; tenantID++ {
		require.NoError(t, db.Exec(`INSERT OR IGNORE INTO tenants (id, name, business) VALUES (?, ?, 'test')`, tenantID, fmt.Sprintf("MK-TENANT-NAME-%d", tenantID)).Error)
	}
	// Workbench admission is actor-fenced: tenant 2's execution actor needs an
	// active users + tenant_members row (53c79fc27 seed pattern).
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id, is_active) VALUES ('pub-adopter-admin', 'pub-adopter-admin', 'pub-adopter-admin@test', 'x', 2, 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (2, 'pub-adopter-admin', 'admin', 'active')`).Error)

	marketRepo := repository.NewAgentMarketplaceRepository(db)
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil)
	versions := service.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	market := service.NewAgentMarketplaceService(versions, marketplaceHTTPResolver{}, marketRepo, t.TempDir())
	adoptionsRepo := repository.NewAgentAdoptionRepository(db)
	adoptions := service.NewAgentAdoptionService(adoptionsRepo, customAgents, versions)
	evaluations := service.NewAgentEvaluationService(repository.NewAgentEvaluationRepository(db))
	metrics := service.NewMarketplaceMetricsService(repository.NewMarketplaceMetricsRepository(db))
	public := service.NewPublicMarketplaceService(repository.NewPublicMarketplaceRepository(db), marketRepo, evaluations, metrics)
	security := service.NewAgentSecurityService(repository.NewAgentSecurityStore(db), repository.NewAgentRunStore(db))
	security.SetAgentVersionService(versions)

	versionHandler := handler.NewAgentVersionHandler(versions)
	marketHandler := handler.NewAgentMarketplaceHandler(market, versions)
	adoptionHandler := handler.NewAgentAdoptionHandler(adoptions)
	publicHandler := handler.NewPublicMarketplaceHandler(public)
	lifecycleHandler := handler.NewAgentMarketplaceLifecycleHandler(service.NewAgentMarketplaceLifecycleService(adoptionsRepo, repository.NewAgentMarketplaceRepository(db)))
	securityHandler := handler.NewAgentSecurityHandler(security)
	agentListHandler := handler.NewCustomAgentHandler(customAgents, nil, repository.NewTenantDisabledSharedAgentRepository(db), nil, nil)

	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}, agentCreator: func(c *gin.Context) (string, error) {
		if c.Param("id") == "agent-owned" {
			return "contributor", nil
		}
		return "", nil
	}}
	coordinator, err := workbenchservice.NewAdmissionCoordinatorWithBinding(db, repository.NewAgentRunStore(db), workbenchservice.NewDurableTaskBudget(db), nil, workbenchservice.NewDatabaseAdmissionBindingResolver(nil))
	require.NoError(t, err)
	coordinator.SetAgentUseGate(newRealAgentUseGate(db))
	coordinator.SetAgentSecurityGate(security)
	coordinator.SetPublishedAgentVersionResolver(security)

	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		tenantID := uint64(1)
		if raw := strings.TrimSpace(c.GetHeader("X-Test-Tenant")); raw != "" {
			if parsed, perr := strconv.ParseUint(raw, 10, 64); perr == nil && parsed > 0 {
				tenantID = parsed
			}
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
	RegisterAgentMarketplaceLifecycleRoutes(v1, lifecycleHandler, g)
	RegisterAgentSecurityRoutes(v1, securityHandler, g)
	RegisterCustomAgentRoutes(v1, agentListHandler, g)
	r.POST("/api/v1/workbench/executions", func(c *gin.Context) {
		tenantID := uint64(1)
		if raw := strings.TrimSpace(c.GetHeader("X-Test-Tenant")); raw != "" {
			if parsed, perr := strconv.ParseUint(raw, 10, 64); perr == nil && parsed > 0 {
				tenantID = parsed
			}
		}
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenantID)
		ctx = context.WithValue(ctx, types.UserIDContextKey, c.GetHeader("X-Test-Actor"))
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(c.GetHeader("X-Test-Role")))
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), tenantID)
		c.Next()
	}, session.NewWorkbenchStartHandler(coordinator).Start)
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
	if tenantID != 1 {
		req.Header.Set("X-Test-Tenant", strconv.FormatUint(tenantID, 10))
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

// publicCatalogRow 是目录行 wire 的严格镜像：DisallowUnknownFields 解码
// 使任何新增 adopter 派生字段（adoption 计数、adopter id、metrics）直接
// 破坏测试（AC2 结构性隐私钉）。T35 #65 Task 4 追加信任/指标字段的
// 允许清单：review 摘要（无 Reason）、manifest 投影、Evaluation 摘要与
// 五字段 MarketplaceMetricsView。
type publicReviewSummaryMirror struct {
	SubmissionID string    `json:"submission_id"`
	ReviewerID   string    `json:"reviewer_id"`
	Decision     string    `json:"decision"`
	ReviewedAt   time.Time `json:"reviewed_at"`
}

type publicEvaluationMirror struct {
	ID               string    `json:"id"`
	ReleaseID        string    `json:"release_id"`
	TestSetID        string    `json:"test_set_id"`
	TestSetVersion   string    `json:"test_set_version"`
	EnvironmentClass string    `json:"environment_class"`
	EvaluatorID      string    `json:"evaluator_id"`
	EvaluatedAt      time.Time `json:"evaluated_at"`
	Results          struct {
		Status string `json:"status"`
		Checks []struct {
			Code   string `json:"code"`
			Status string `json:"status"`
		} `json:"checks"`
	} `json:"results"`
}

type publicMetricsMirror struct {
	IntroductionsBucket       string `json:"introductions_bucket"`
	ActiveAdoptersBucket      string `json:"active_adopters_bucket"`
	UpgradeProposalsBucket    string `json:"upgrade_proposals_bucket"`
	AcceptedUpgradesBucket    string `json:"accepted_upgrades_bucket"`
	ErrorCategoryAvailability string `json:"error_category_availability"`
}

type publicCatalogRow struct {
	ID                string `json:"id"`
	DisplayName       string `json:"display_name"`
	Summary           string `json:"summary"`
	State             string `json:"state"`
	PublisherTenantID uint64 `json:"publisher_tenant_id"`
	PublisherVerified bool   `json:"publisher_verified"`
	CurrentRelease    *struct {
		ID                       string          `json:"id"`
		SemanticVersion          string          `json:"semantic_version"`
		BundleDigest             string          `json:"bundle_digest"`
		Manifest                 json.RawMessage `json:"manifest"`
		DependencyLock           json.RawMessage `json:"dependency_lock"`
		MinimumWeKnoraCapability string          `json:"minimum_weknora_capability"`
		CapabilityRequirements   []string        `json:"capability_requirements"`
		LicenseID                string          `json:"license_id"`
		CreatedAt                time.Time       `json:"created_at"`
	} `json:"current_release"`
	CurrentReleaseReview *publicReviewSummaryMirror `json:"current_release_review"`
	Evaluations          []publicEvaluationMirror   `json:"evaluations"`
	Metrics              *publicMetricsMirror       `json:"metrics"`
	CreatedAt            time.Time                  `json:"created_at"`
	UpdatedAt            time.Time                  `json:"updated_at"`
}

func decodeStrictCatalogRows(t *testing.T, body []byte) []publicCatalogRow {
	t.Helper()
	var envelope struct {
		Success bool             `json:"success"`
		Data    []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.True(t, envelope.Success)
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var rows []publicCatalogRow
	require.NoError(t, dec.Decode(&rows))
	return rows
}

func decodeStrictCatalogDetail(t *testing.T, body []byte) publicCatalogRow {
	t.Helper()
	var envelope struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.True(t, envelope.Success)
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var row publicCatalogRow
	require.NoError(t, dec.Decode(&row))
	return row
}

// approvePublicRelease walks verify -> submit -> review-approve and returns
// the public listing id + release id.
func approvePublicRelease(t *testing.T, r *gin.Engine, listingID string) (publicListingID, publicReleaseID string) {
	t.Helper()
	_ = publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1})
	submitted := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", map[string]any{"source_listing_id": listingID})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID              string `json:"id"`
			PublicListingID string `json:"public_listing_id"`
			BundleDigest    string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))
	approved := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "sysadmin", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())
	var reviewBody struct {
		Data struct {
			Release struct {
				ID string `json:"id"`
			} `json:"release"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &reviewBody))
	require.NotEmpty(t, reviewBody.Data.Release.ID)
	return submissionBody.Data.PublicListingID, reviewBody.Data.Release.ID
}

func TestPublicMarketplaceCatalogAndAdoptAuthorization(t *testing.T) {
	r, _, _ := newPublicMarketplaceTestApp(t)
	listingID := publishTenantRelease(t, r)
	publicListingID, publicReleaseID := approvePublicRelease(t, r, listingID)

	// 目录：Viewer+ 可读；严格解码钉死字段集
	catalog := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/public/catalog", "viewer", "viewer-2", nil)
	require.Equal(t, http.StatusOK, catalog.Code, catalog.Body.String())
	rows := decodeStrictCatalogRows(t, catalog.Body.Bytes())
	require.Len(t, rows, 1)
	require.Equal(t, publicListingID, rows[0].ID)
	require.True(t, rows[0].PublisherVerified)
	require.NotNil(t, rows[0].CurrentRelease)
	require.Equal(t, publicReleaseID, rows[0].CurrentRelease.ID)
	require.NotEmpty(t, rows[0].CurrentRelease.Manifest)

	// Listing 详情：Viewer+ 可读
	detail := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/public/listings/"+publicListingID, "viewer", "viewer-2", nil)
	require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())

	// Adopt：Viewer 403，Admin 201；幂等重放 200
	viewerAdopt := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "viewer", "viewer-2", map[string]any{})
	require.Equal(t, http.StatusForbidden, viewerAdopt.Code, viewerAdopt.Body.String())
	adopted := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-admin", map[string]any{})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptBody struct {
		Data struct {
			Introduction struct {
				ID              string `json:"id"`
				PublicListingID string `json:"public_listing_id"`
				PublicReleaseID string `json:"public_release_id"`
				BundleDigest    string `json:"bundle_digest"`
			} `json:"introduction"`
			Adoption struct {
				ID                string `json:"id"`
				ListingID         string `json:"listing_id"`
				AcceptedReleaseID string `json:"accepted_release_id"`
				State             string `json:"state"`
			} `json:"adoption"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptBody))
	require.Equal(t, publicListingID, adoptBody.Data.Introduction.PublicListingID)
	require.Equal(t, publicReleaseID, adoptBody.Data.Introduction.PublicReleaseID)
	require.Equal(t, adoptBody.Data.Introduction.ID, adoptBody.Data.Adoption.AcceptedReleaseID)
	require.Equal(t, "active", adoptBody.Data.Adoption.State)

	readopt := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-admin", map[string]any{})
	require.Equal(t, http.StatusOK, readopt.Code, readopt.Body.String())

	// 采用方租户经 #59 端点看到引入关系
	adoptions := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "admin", "adopter-admin", nil)
	require.Equal(t, http.StatusOK, adoptions.Code, adoptions.Body.String())
	require.Contains(t, adoptions.Body.String(), adoptBody.Data.Adoption.ID)

	// 未知 listing / release → 404
	missing := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/no-such-listing/adopt", "admin", "adopter-admin", map[string]any{})
	require.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())
	missingRelease := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-admin", map[string]any{"release_id": "no-such-release"})
	require.Equal(t, http.StatusNotFound, missingRelease.Code, missingRelease.Body.String())
}

// TestPublicMarketplaceCrossTenantEndToEndAndPrivacy 是 Issue #60 三条验收
// 标准的最高稳定 Interface 证据（真实 sqlite 迁移流 + 真实服务栈 + 真实
// HTTP）：
//
//	AC1 跨 Tenant 只传播可移植 Release —— 发布者 Agent 的 KB/模型绑定
//	     （kb-publisher / model-publisher）不进入采用方本地 Agent；采用方
//	     的知识绑定完全来自本地映射（kb-tenant-2）。
//	AC2 发布者和源 Tenant 无法读取采用方身份、映射或 Task 内容 ——
//	     发布者视角（tenant 1）全部可见响应不含任何采用方标识；公共目录
//	     行字段集被严格解码钉死。
//	AC3 全链真实 HTTP：freeze → tenant release → tenant review → verify →
//	     public submit → platform review → catalog → adopt → variant →
//	     mapping → test → publish → GET /api/v1/agents（#33 移动 Resource
//	     Shelf wire）→ available-agents。
func TestPublicMarketplaceCrossTenantEndToEndAndPrivacy(t *testing.T) {
	r, _, db := newPublicMarketplaceTestApp(t)
	listingID := publishTenantRelease(t, r)
	// publicReleaseID 在本测试无断言消费，用 _ 接收（编译修复，断言语义不变）
	publicListingID, _ := approvePublicRelease(t, r, listingID)

	// --- 采用方（tenant 2）发现并引入 ---
	catalog := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/public/catalog", "viewer", "viewer-2", nil)
	require.Equal(t, http.StatusOK, catalog.Code, catalog.Body.String())
	rows := decodeStrictCatalogRows(t, catalog.Body.Bytes())
	require.Len(t, rows, 1)

	adopted := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-admin", map[string]any{})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptBody struct {
		Data struct {
			Introduction struct {
				ID string `json:"id"`
			} `json:"introduction"`
			Adoption struct {
				ID                string `json:"id"`
				AcceptedReleaseID string `json:"accepted_release_id"`
			} `json:"adoption"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptBody))
	introductionID := adoptBody.Data.Introduction.ID
	adoptionID := adoptBody.Data.Adoption.ID
	require.NotEmpty(t, introductionID)
	require.Equal(t, introductionID, adoptBody.Data.Adoption.AcceptedReleaseID)

	// --- 采用方本地 Variant 链（#59 端点原样） ---
	variant := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionID+"/variants", "admin", "adopter-admin", map[string]any{"name": "Imported helper"})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID                  string   `json:"id"`
			ReleaseID           string   `json:"release_id"`
			State               string   `json:"state"`
			MissingCapabilities []string `json:"missing_capabilities"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))
	require.Equal(t, introductionID, variantBody.Data.ReleaseID, "Variant 固定引入 release")
	require.Equal(t, []string{"knowledge"}, variantBody.Data.MissingCapabilities)

	mapped := publicCall(r, 2, false, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "adopter-admin", map[string]any{"mappings": []map[string]any{{"capability": "knowledge", "knowledge_base_ids": []string{"kb-tenant-2"}}}})
	require.Equal(t, http.StatusOK, mapped.Code, mapped.Body.String())
	tested := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "adopter-admin", nil)
	require.Equal(t, http.StatusOK, tested.Code, tested.Body.String())
	published := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/publish", "admin", "adopter-admin", nil)
	require.Equal(t, http.StatusOK, published.Code, published.Body.String())
	var publishedBody struct {
		Data struct {
			LocalAgentID string `json:"local_agent_id"`
			State        string `json:"state"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(published.Body.Bytes(), &publishedBody))
	require.Equal(t, "published", publishedBody.Data.State)
	localAgentID := publishedBody.Data.LocalAgentID
	require.NotEmpty(t, localAgentID)

	// --- AC1/AC3：移动 Resource Shelf wire（#33）中传播产物只含可移植内容 ---
	agents2 := publicCall(r, 2, false, http.MethodGet, "/api/v1/agents", "viewer", "viewer-2", nil)
	require.Equal(t, http.StatusOK, agents2.Code, agents2.Body.String())
	var agentsBody struct {
		Data []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Config struct {
				SystemPrompt   string   `json:"system_prompt"`
				KnowledgeBases []string `json:"knowledge_bases"`
				ModelID        string   `json:"model_id"`
			} `json:"config"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(agents2.Body.Bytes(), &agentsBody))
	var imported *struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Config struct {
			SystemPrompt   string   `json:"system_prompt"`
			KnowledgeBases []string `json:"knowledge_bases"`
			ModelID        string   `json:"model_id"`
		} `json:"config"`
	}
	for i := range agentsBody.Data {
		if agentsBody.Data[i].ID == localAgentID {
			imported = &agentsBody.Data[i]
		}
	}
	require.NotNil(t, imported, "发布后的本地 Agent 必须出现在 GET /api/v1/agents（移动 Resource Shelf wire）")
	require.Equal(t, "Be portable.", imported.Config.SystemPrompt, "可移植 system prompt 随传播")
	require.Equal(t, []string{"kb-tenant-2"}, imported.Config.KnowledgeBases, "AC1：知识绑定只来自采用方本地映射")
	require.Equal(t, "", imported.Config.ModelID, "AC1：发布者模型绑定（model-publisher）不得传播")
	require.NotContains(t, agents2.Body.String(), "kb-publisher", "AC1：发布者 KB 不外泄")
	require.NotContains(t, agents2.Body.String(), "model-publisher", "AC1：发布者模型不外泄")

	available := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer-2", nil)
	require.Equal(t, http.StatusOK, available.Code, available.Body.String())
	require.Contains(t, available.Body.String(), localAgentID)

	// --- AC2：发布者（tenant 1）与源租户可见面零采用方痕迹 ---
	publisherAgents := publicCall(r, 1, false, http.MethodGet, "/api/v1/agents", "admin", "tenant-admin", nil)
	require.Equal(t, http.StatusOK, publisherAgents.Code)
	require.NotContains(t, publisherAgents.Body.String(), localAgentID)
	require.NotContains(t, publisherAgents.Body.String(), "Imported helper")
	require.NotContains(t, publisherAgents.Body.String(), "kb-tenant-2")

	publisherAdoptions := publicCall(r, 1, false, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "admin", "tenant-admin", nil)
	require.Equal(t, http.StatusOK, publisherAdoptions.Code)
	var publisherAdoptionsEnvelope struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(publisherAdoptions.Body.Bytes(), &publisherAdoptionsEnvelope))
	require.Empty(t, publisherAdoptionsEnvelope.Data, "AC2：发布者视角无任何（含采用方的）Adoption 行")

	publisherCatalog := publicCall(r, 1, false, http.MethodGet, "/api/v1/marketplace/public/catalog", "viewer", "viewer-1", nil)
	require.Equal(t, http.StatusOK, publisherCatalog.Code)
	for _, secret := range []string{adoptionID, variantBody.Data.ID, introductionID, localAgentID, "kb-tenant-2", "Imported helper", "adopter-admin"} {
		require.NotContains(t, publisherCatalog.Body.String(), secret, "AC2：公共目录不得含采用方标识")
	}

	publisherSubmissions := publicCall(r, 1, false, http.MethodGet, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", nil)
	require.Equal(t, http.StatusOK, publisherSubmissions.Code)
	require.NotContains(t, publisherSubmissions.Body.String(), adoptionID)

	// 平台审核面也不含采用方数据（结构钉死：队列行=提交行字段集）
	queue := publicCall(r, 1, true, http.MethodGet, "/api/v1/marketplace/public/release-submissions/review-queue", "admin", "sysadmin", nil)
	require.Equal(t, http.StatusOK, queue.Code)
	require.NotContains(t, queue.Body.String(), adoptionID)
	require.NotContains(t, queue.Body.String(), localAgentID)

	// db 侧证：采用方行只存在于 tenant 2 作用域
	var adoptionRows int64
	db.Table("agent_adoptions").Where("listing_id = ?", publicListingID).Count(&adoptionRows)
	require.Equal(t, int64(1), adoptionRows)
	db.Table("agent_adoptions").Where("tenant_id = ? AND listing_id = ?", uint64(2), publicListingID).Count(&adoptionRows)
	require.Equal(t, int64(1), adoptionRows)
}

func TestPublicMarketplaceAdoptRejectsTamperedPublicRelease(t *testing.T) {
	r, _, db := newPublicMarketplaceTestApp(t)
	listingID := publishTenantRelease(t, r)
	publicListingID, publicReleaseID := approvePublicRelease(t, r, listingID)

	// 公共 release 字节被篡改（digest 不再匹配）：引入必须拒绝且零行落库
	require.NoError(t, db.Exec("UPDATE public_agent_releases SET bundle = ? WHERE id = ?", []byte(`{"payload":{"system_prompt":"evil"}}`), publicReleaseID).Error)
	rejected := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-admin", map[string]any{})
	require.Equal(t, http.StatusConflict, rejected.Code, rejected.Body.String())

	var introducedCount, adoptionCount int64
	db.Table("tenant_introduced_releases").Where("tenant_id = ?", uint64(2)).Count(&introducedCount)
	db.Table("agent_adoptions").Where("tenant_id = ?", uint64(2)).Count(&adoptionCount)
	require.Zero(t, introducedCount, "AC1：篡改的公共 release 不得产生引入行")
	require.Zero(t, adoptionCount, "AC1：篡改的公共 release 不得产生 Adoption")
}

// approvePublicReleaseWithReason walks verify -> submit -> review-approve,
// carrying an optional review Reason (approved submissions may keep one).
func approvePublicReleaseWithReason(t *testing.T, r *gin.Engine, listingID, reason string) (publicListingID, publicReleaseID, submissionID string) {
	t.Helper()
	_ = publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1})
	submitted := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", map[string]any{"source_listing_id": listingID})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID              string `json:"id"`
			PublicListingID string `json:"public_listing_id"`
			BundleDigest    string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))
	body := map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"}
	if reason != "" {
		body["reason"] = reason
	}
	approved := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "sysadmin", body)
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())
	var reviewBody struct {
		Data struct {
			Release struct {
				ID string `json:"id"`
			} `json:"release"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &reviewBody))
	require.NotEmpty(t, reviewBody.Data.Release.ID)
	return submissionBody.Data.PublicListingID, reviewBody.Data.Release.ID, submissionBody.Data.ID
}

// publicEvaluationBody builds a POST /marketplace/public/evaluations body.
func publicEvaluationBody(releaseID, testSetID string, results map[string]any) map[string]any {
	if results == nil {
		results = map[string]any{"status": "pass", "checks": []map[string]any{{"code": "manifest_completeness", "status": "pass"}, {"code": "privacy", "status": "pass"}}}
	}
	return map[string]any{"release_id": releaseID, "test_set_id": testSetID, "test_set_version": "v1", "environment_class": "standard", "results": results}
}

func decodeStrictEvaluation(t *testing.T, body []byte) publicEvaluationMirror {
	t.Helper()
	var envelope struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.True(t, envelope.Success)
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var view publicEvaluationMirror
	require.NoError(t, dec.Decode(&view))
	return view
}

// decodePublicAdoption reads the introduction/adoption ids from an adopt
// response.
func decodePublicAdoption(t *testing.T, body []byte) (introductionID, adoptionID string) {
	t.Helper()
	var adoptBody struct {
		Data struct {
			Introduction struct {
				ID string `json:"id"`
			} `json:"introduction"`
			Adoption struct {
				ID string `json:"id"`
			} `json:"adoption"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &adoptBody))
	require.NotEmpty(t, adoptBody.Data.Introduction.ID)
	require.NotEmpty(t, adoptBody.Data.Adoption.ID)
	return adoptBody.Data.Introduction.ID, adoptBody.Data.Adoption.ID
}

// TestPublicMarketplaceCatalogTrustEvaluationAndMetrics is the T35 #65 Task 4
// 最高稳定 HTTP 证据：真实迁移 SQLite + 真实服务栈上的信任与隐私读模型。
//
//   - POST /marketplace/public/evaluations 仅 SystemAdmin；普通 Admin 与
//     API key 拒绝；ErrAgentEvaluationInvalid→400、Conflict→409；响应是
//     闭合的强类型 {status,checks} 结构（非双重编码 JSON）。
//   - 目录/详情带 review 摘要（decision/reviewer/time，永无 Reason）、
//     manifest 兼容下限/能力/许可证投影、Release 钉定 Evaluation 摘要、
//     五字段 MarketplaceMetricsView（阈值桶 + not_collected）。
//   - Listing 指针前进后 Evaluation 仍钉在其源 Release 上。
//   - 源行内唯一标记串（租户名/DiffJSON/任务/提示词/输出/审核理由）在
//     目录与详情 wire 上零出现。
func TestPublicMarketplaceCatalogTrustEvaluationAndMetrics(t *testing.T) {
	r, g, db := newPublicMarketplaceTestApp(t)
	listingID := publishTenantRelease(t, r)
	publicListingID, release1, submission1 := approvePublicReleaseWithReason(t, r, listingID, "MK-REVIEW-REASON-1")

	// --- 权限矩阵：Evaluation authoring 仅 SystemAdmin（checkbox 6）---
	viewerPost := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/evaluations", "viewer", "viewer-1", publicEvaluationBody(release1, "ts-suite", nil))
	require.Equal(t, http.StatusForbidden, viewerPost.Code, viewerPost.Body.String())
	adminPost := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/evaluations", "admin", "tenant-admin", publicEvaluationBody(release1, "ts-suite", nil))
	require.Equal(t, http.StatusForbidden, adminPost.Code, adminPost.Body.String())
	_, declared := g.apiKeyAuthorizer.Lookup(http.MethodPost, "/api/v1/marketplace/public/evaluations")
	require.False(t, declared, "平台 Evaluation 不得对 API key 声明策略（门默认拒绝）")
	spoofed := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/evaluations", "admin", "sysadmin", map[string]any{"release_id": release1, "test_set_id": "ts-suite", "test_set_version": "v1", "environment_class": "standard", "evaluator_id": "spoof", "results": map[string]any{"status": "pass", "checks": []map[string]any{{"code": "privacy", "status": "pass"}}}})
	require.Equal(t, http.StatusBadRequest, spoofed.Code, "body 不得携带评审人身份（严格解码拒绝）")

	recorded := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/evaluations", "admin", "sysadmin", publicEvaluationBody(release1, "ts-suite", nil))
	require.Equal(t, http.StatusCreated, recorded.Code, recorded.Body.String())
	view := decodeStrictEvaluation(t, recorded.Body.Bytes())
	require.Equal(t, release1, view.ReleaseID)
	require.Equal(t, "sysadmin", view.EvaluatorID, "评审人身份来自认证上下文")
	require.Equal(t, "pass", view.Results.Status)
	require.Len(t, view.Results.Checks, 2)
	require.Equal(t, "manifest_completeness", view.Results.Checks[0].Code)
	require.False(t, view.EvaluatedAt.IsZero())

	conflict := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/evaluations", "admin", "sysadmin", publicEvaluationBody(release1, "ts-suite", nil))
	require.Equal(t, http.StatusConflict, conflict.Code, conflict.Body.String())
	invalid := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/evaluations", "admin", "sysadmin",
		publicEvaluationBody(release1, "ts-bad", map[string]any{"status": "pass", "checks": []map[string]any{{"code": "evil", "status": "pass"}}}))
	require.Equal(t, http.StatusBadRequest, invalid.Code, invalid.Body.String())

	// --- 多采用租户 + 升级提案（带标记的源行） ---
	introductions := map[uint64]string{}
	adoptionIDs := map[uint64]string{}
	for tenantID := uint64(2); tenantID <= 6; tenantID++ {
		adopted := publicCall(r, tenantID, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", fmt.Sprintf("MK-ADOPTER-%d", tenantID), map[string]any{})
		require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
		introID, adoptionID := decodePublicAdoption(t, adopted.Body.Bytes())
		introductions[tenantID] = introID
		adoptionIDs[tenantID] = adoptionID
		require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{
			ID: fmt.Sprintf("MK-PROPOSAL-%d", tenantID), TenantID: tenantID, AdoptionID: adoptionID, ListingID: publicListingID,
			FromReleaseID: introID, ToReleaseID: introID, ToSemanticVersion: "1.0.0",
			DiffJSON: `{"private":"MK-DIFF-JSON","task_title":"MK-TASK-TITLE","prompt":"MK-PROMPT","output":"MK-OUTPUT"}`,
			State:    "open", ResolvedBy: "MK-RESOLVER", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}).Error)
	}

	// --- 目录（Viewer+）：严格 DTO + 信任 + 指标桶 ---
	catalog := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/public/catalog", "viewer", "MK-VIEWER-ACTOR", nil)
	require.Equal(t, http.StatusOK, catalog.Code, catalog.Body.String())
	rows := decodeStrictCatalogRows(t, catalog.Body.Bytes())
	require.Len(t, rows, 1)
	row := rows[0]
	require.NotNil(t, row.CurrentRelease)
	require.Equal(t, release1, row.CurrentRelease.ID)
	require.Equal(t, "1", row.CurrentRelease.MinimumWeKnoraCapability)
	require.Equal(t, []string{"knowledge"}, row.CurrentRelease.CapabilityRequirements)
	require.Equal(t, "MIT", row.CurrentRelease.LicenseID)
	require.NotNil(t, row.CurrentReleaseReview)
	require.Equal(t, "approved", row.CurrentReleaseReview.Decision)
	require.Equal(t, "sysadmin", row.CurrentReleaseReview.ReviewerID)
	require.Equal(t, submission1, row.CurrentReleaseReview.SubmissionID)
	require.False(t, row.CurrentReleaseReview.ReviewedAt.IsZero())
	require.Len(t, row.Evaluations, 1)
	require.Equal(t, release1, row.Evaluations[0].ReleaseID)
	require.Equal(t, "ts-suite", row.Evaluations[0].TestSetID)
	require.Equal(t, "pass", row.Evaluations[0].Results.Status)
	require.NotNil(t, row.Metrics)
	require.Equal(t, "5-9", row.Metrics.IntroductionsBucket)
	require.Equal(t, "5-9", row.Metrics.ActiveAdoptersBucket)
	require.Equal(t, "5-9", row.Metrics.UpgradeProposalsBucket)
	require.Equal(t, "suppressed", row.Metrics.AcceptedUpgradesBucket)
	require.Equal(t, "not_collected", row.Metrics.ErrorCategoryAvailability, "error-category 永远显式 not_collected，绝不伪造零计数")

	// --- 详情（Viewer+）与目录同构 ---
	detail := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/public/listings/"+publicListingID, "viewer", "MK-VIEWER-ACTOR", nil)
	require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
	detailRow := decodeStrictCatalogDetail(t, detail.Body.Bytes())
	require.Equal(t, release1, detailRow.CurrentRelease.ID)
	require.NotNil(t, detailRow.Metrics)
	require.Equal(t, "5-9", detailRow.Metrics.IntroductionsBucket)
	require.Len(t, detailRow.Evaluations, 1)

	// --- 指针前进：Evaluation 仍钉在源 Release（checkbox 2）---
	upgradeMeta := func() map[string]any {
		return map[string]any{"supported_languages": []string{"en"}, "use_cases": []string{"support"},
			"capability_requirements": []string{"knowledge"}, "minimum_weknora_capability": "2", "license_id": "Apache-2.0"}
	}
	freezeAndPublishUpgradeRelease(t, r, "1.1.0", upgradeMeta())
	_, release2, submission2 := approvePublicReleaseWithReason(t, r, listingID, "MK-REVIEW-REASON-2")
	require.NotEqual(t, release1, release2)
	recorded2 := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/evaluations", "admin", "sysadmin", publicEvaluationBody(release2, "ts-suite-v2", nil))
	require.Equal(t, http.StatusCreated, recorded2.Code, recorded2.Body.String())
	evaluation2 := decodeStrictEvaluation(t, recorded2.Body.Bytes())

	detail2 := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/public/listings/"+publicListingID, "viewer", "MK-VIEWER-ACTOR", nil)
	require.Equal(t, http.StatusOK, detail2.Code, detail2.Body.String())
	detail2Row := decodeStrictCatalogDetail(t, detail2.Body.Bytes())
	require.Equal(t, release2, detail2Row.CurrentRelease.ID)
	require.Equal(t, "2", detail2Row.CurrentRelease.MinimumWeKnoraCapability)
	require.Equal(t, "Apache-2.0", detail2Row.CurrentRelease.LicenseID)
	require.Equal(t, submission2, detail2Row.CurrentReleaseReview.SubmissionID, "review 摘要跟随当前 release")
	require.Len(t, detail2Row.Evaluations, 1)
	require.Equal(t, release2, detail2Row.Evaluations[0].ReleaseID, "指针前进后 Evaluation 不随 listing 移动")
	require.Equal(t, evaluation2.ID, detail2Row.Evaluations[0].ID)
	require.NotNil(t, detail2Row.Metrics)
	require.Equal(t, "suppressed", detail2Row.Metrics.IntroductionsBucket, "新 release 零采用租户必须抑制")

	var pinned types.AgentEvaluationEntity
	require.NoError(t, db.Where("id = ?", view.ID).Take(&pinned).Error)
	require.Equal(t, release1, pinned.ReleaseID, "历史 Evaluation 行仍钉在其源 Release")

	// --- 标记串零泄漏（checkbox 3）---
	for name, body := range map[string]string{"catalog": catalog.Body.String(), "detail": detail2.Body.String()} {
		for _, marker := range []string{"MK-REVIEW-REASON", "MK-DIFF-JSON", "MK-TASK-TITLE", "MK-PROMPT", "MK-OUTPUT", "MK-TENANT-NAME", "MK-RESOLVER", "MK-ADOPTER"} {
			require.NotContains(t, body, marker, "%s wire 不得泄漏源行标记 %s", name, marker)
		}
	}
}

// TestPublicMarketplacePublisherCustody drives the three custody triggers
// over the real HTTP stack: publisher revocation, source tenant unlist and
// platform public-listing unlist. Each hides the listing from catalog AND
// direct detail, denies new adoption (explicit release pin included) with
// zero Introduction/Adoption writes, while the existing adopter keeps its
// local immutable package and provenance (#65 AC2).
func TestPublicMarketplacePublisherCustody(t *testing.T) {
	scenarios := []struct {
		name    string
		trigger func(t *testing.T, r *gin.Engine, db *gorm.DB, publicListingID, sourceListingID string)
	}{
		{name: "publisher revoke", trigger: func(t *testing.T, r *gin.Engine, _ *gorm.DB, _, _ string) {
			revoked := publicCall(r, 1, true, http.MethodDelete, "/api/v1/marketplace/public/verified-publishers/1", "admin", "sysadmin", nil)
			require.Equal(t, http.StatusOK, revoked.Code, revoked.Body.String())
		}},
		{name: "source listing unlist", trigger: func(t *testing.T, r *gin.Engine, _ *gorm.DB, _, sourceListingID string) {
			unlisted := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/listings/"+sourceListingID+"/unlist", "admin", "tenant-admin", nil)
			require.Equal(t, http.StatusOK, unlisted.Code, unlisted.Body.String())
		}},
		{name: "public listing unlist", trigger: func(t *testing.T, r *gin.Engine, db *gorm.DB, publicListingID, _ string) {
			_, err := repository.NewPublicMarketplaceRepository(db).UnlistPublicListing(context.Background(), publicListingID, "sysadmin", "t65 custody platform unlist")
			require.NoError(t, err)
		}},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			r, _, db := newPublicMarketplaceTestApp(t)
			sourceListingID := publishTenantRelease(t, r)
			publicListingID, releaseID, _ := approvePublicReleaseWithReason(t, r, sourceListingID, "approved")

			adopted := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-admin", map[string]any{})
			require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
			introductionID, adoptionID := decodePublicAdoption(t, adopted.Body.Bytes())
			var before types.TenantIntroducedReleaseEntity
			require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(2), introductionID).Take(&before).Error)

			scenario.trigger(t, r, db, publicListingID, sourceListingID)

			catalog := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/public/catalog", "viewer", "viewer-2", nil)
			require.Equal(t, http.StatusOK, catalog.Code, catalog.Body.String())
			require.Empty(t, decodeStrictCatalogRows(t, catalog.Body.Bytes()), "custody 触发后目录必须隐藏")
			detail := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/public/listings/"+publicListingID, "viewer", "viewer-2", nil)
			require.Equal(t, http.StatusNotFound, detail.Code, "直接详情必须同口径 404")

			newAdopt := publicCall(r, 3, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-3-admin", map[string]any{"release_id": releaseID})
			require.Equal(t, http.StatusNotFound, newAdopt.Code, newAdopt.Body.String())
			var introducedCount, adoptionCount int64
			db.Table("tenant_introduced_releases").Where("tenant_id = ?", uint64(3)).Count(&introducedCount)
			db.Table("agent_adoptions").Where("tenant_id = ?", uint64(3)).Count(&adoptionCount)
			require.Zero(t, introducedCount, "显式 release 钉定的新采用不得留下 Introduction")
			require.Zero(t, adoptionCount, "显式 release 钉定的新采用不得留下 Adoption")

			// 既有采用方：本地不可变包与出处保持原样，采用视图仍可读。
			var after types.TenantIntroducedReleaseEntity
			require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(2), introductionID).Take(&after).Error)
			require.Equal(t, before.Bundle, after.Bundle)
			require.Equal(t, before.ManifestJSON, after.ManifestJSON)
			require.Equal(t, before.PublicListingID, after.PublicListingID)
			require.Equal(t, before.PublicReleaseID, after.PublicReleaseID)
			require.Equal(t, before.IntroducedBy, after.IntroducedBy)
			adoptions := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "admin", "adopter-admin", nil)
			require.Equal(t, http.StatusOK, adoptions.Code, adoptions.Body.String())
			require.Contains(t, adoptions.Body.String(), adoptionID, "既有采用方仍能读取本地 Adoption")
		})
	}
}

// publicWorkbenchStart posts a tenant-2 workbench execution start over the
// real engine (T35 #65 #64-override evidence).
func publicWorkbenchStart(t *testing.T, r *gin.Engine, sessionID, requestID, agentID string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"session_id":"` + sessionID + `","agent_id":"` + agentID + `","target_id":"platform","request_id":"` + requestID + `","text":"t65 turn","budget_upper":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", "admin")
	req.Header.Set("X-Test-Actor", "pub-adopter-admin")
	req.Header.Set("X-Test-Tenant", "2")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// publishAdopterVariant drives tenant 2's introduced release through the
// real #59 variant chain to a published local agent.
func publishAdopterVariant(t *testing.T, r *gin.Engine, adoptionID string) (variantID, localAgentID string) {
	t.Helper()
	variant := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionID+"/variants", "admin", "pub-adopter-admin", map[string]any{"name": "T65 imported"})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))
	mapped := publicCall(r, 2, false, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "pub-adopter-admin", map[string]any{"mappings": []map[string]any{{"capability": "knowledge", "knowledge_base_ids": []string{"kb-tenant-2"}}}})
	require.Equal(t, http.StatusOK, mapped.Code, mapped.Body.String())
	tested := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "pub-adopter-admin", nil)
	require.Equal(t, http.StatusOK, tested.Code, tested.Body.String())
	published := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/publish", "admin", "pub-adopter-admin", nil)
	require.Equal(t, http.StatusOK, published.Code, published.Body.String())
	var publishBody struct {
		Data struct {
			LocalAgentID string `json:"local_agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(published.Body.Bytes(), &publishBody))
	require.NotEmpty(t, publishBody.Data.LocalAgentID)
	return variantBody.Data.ID, publishBody.Data.LocalAgentID
}

// TestPublicMarketplaceSecurityRevocationOverridesCustody proves the #64
// hard override (#65 Task 4): after a security revocation of the adopter's
// introduced release, custody retention (the immutable local package stays)
// does NOT reopen admission — the adopter's re-adoption is refused and new
// workbench execution is denied with zero durable side effects.
func TestPublicMarketplaceSecurityRevocationOverridesCustody(t *testing.T) {
	r, _, db := newPublicMarketplaceTestApp(t)
	sourceListingID := publishTenantRelease(t, r)
	publicListingID, releaseID, _ := approvePublicReleaseWithReason(t, r, sourceListingID, "approved")

	adopted := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "pub-adopter-admin", map[string]any{})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	introductionID, adoptionID := decodePublicAdoption(t, adopted.Body.Bytes())
	_, localAgentID := publishAdopterVariant(t, r, adoptionID)

	for _, sessionID := range []string{"t65-sec-live", "t65-sec-after"} {
		require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, 2, ?, 'pub-adopter-admin', 'trpc')", sessionID, sessionID).Error)
	}
	live := publicWorkbenchStart(t, r, "t65-sec-live", "req-t65-live", localAgentID)
	require.Equal(t, http.StatusAccepted, live.Code, live.Body.String())

	revoked := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", "admin", "pub-adopter-admin", map[string]any{"release_id": introductionID, "reason": "t65 compromised bundle", "in_flight_disposition": "cancel"})
	require.Equal(t, http.StatusCreated, revoked.Code, revoked.Body.String())

	// Custody retention：不可变引入包与出处仍在，未被撤销清除。
	var retained types.TenantIntroducedReleaseEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(2), introductionID).Take(&retained).Error)
	require.Equal(t, releaseID, retained.PublicReleaseID)
	require.NotEmpty(t, retained.Bundle)

	// 保留不重开采用面：同 release 再采用被安全闸拒绝（409，不是 200/404）。
	reAdopt := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "pub-adopter-admin", map[string]any{"release_id": releaseID})
	require.Equal(t, http.StatusConflict, reAdopt.Code, reAdopt.Body.String())

	// 新执行被拒绝且零持久副作用。
	denied := publicWorkbenchStart(t, r, "t65-sec-after", "req-t65-after", localAgentID)
	require.Equal(t, http.StatusConflict, denied.Code, denied.Body.String())
	var deniedRequests, deniedRuns int64
	db.Table("workbench_requests").Where("tenant_id = ? AND request_id = ?", uint64(2), "req-t65-after").Count(&deniedRequests)
	require.Zero(t, deniedRequests)
	db.Table("agent_runs").Where("tenant_id = ? AND session_id = ?", uint64(2), "t65-sec-after").Count(&deniedRuns)
	require.Zero(t, deniedRuns)
}
