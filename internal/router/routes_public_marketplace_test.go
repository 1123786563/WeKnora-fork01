package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// publicCatalogRow 是目录行 wire 的严格镜像：DisallowUnknownFields 解码
// 使任何新增 adopter 派生字段（adoption 计数、adopter id、metrics）直接
// 破坏测试（AC2 结构性隐私钉）。
type publicCatalogRow struct {
	ID                string `json:"id"`
	DisplayName       string `json:"display_name"`
	Summary           string `json:"summary"`
	State             string `json:"state"`
	PublisherTenantID uint64 `json:"publisher_tenant_id"`
	PublisherVerified bool   `json:"publisher_verified"`
	CurrentRelease    *struct {
		ID              string          `json:"id"`
		SemanticVersion string          `json:"semantic_version"`
		BundleDigest    string          `json:"bundle_digest"`
		Manifest        json.RawMessage `json:"manifest"`
		DependencyLock  json.RawMessage `json:"dependency_lock"`
		CreatedAt       time.Time       `json:"created_at"`
	} `json:"current_release"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
