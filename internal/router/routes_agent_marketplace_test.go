package router

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type marketplaceHTTPAgentSource map[string]*types.CustomAgent

func (s marketplaceHTTPAgentSource) GetAgentByIDAndTenant(_ context.Context, id string, tenantID uint64) (*types.CustomAgent, error) {
	if agent := s[filepath.Join(string(rune(tenantID)), id)]; agent != nil {
		return agent, nil
	}
	return nil, service.ErrAgentNotFound
}

type marketplaceHTTPResolver struct{}

func (marketplaceHTTPResolver) Resolve(context.Context, uint64, types.AgentVersionSnapshot) (types.DependencyLock, error) {
	return types.DependencyLock{}, nil
}

func TestTenantAgentMarketplaceLifecycleAndAuthorization(t *testing.T) {
	db := openTenantAgentMarketplaceHTTPTestDB(t)
	marketRepo := repository.NewAgentMarketplaceRepository(db)
	versionRepo := repository.NewAgentVersionRepository(db)
	agents := marketplaceHTTPAgentSource{
		filepath.Join(string(rune(1)), "agent-owned"): {ID: "agent-owned", Name: "Release helper", TenantID: 1, Config: types.CustomAgentConfig{SystemPrompt: "Be useful."}},
		filepath.Join(string(rune(1)), "agent-other"): {ID: "agent-other", Name: "Colleague helper", TenantID: 1, Config: types.CustomAgentConfig{SystemPrompt: "Be useful."}},
	}
	versions := service.NewAgentVersionService(agents, versionRepo)
	bundleRoot := t.TempDir()
	market := service.NewAgentMarketplaceService(versions, marketplaceHTTPResolver{}, marketRepo, bundleRoot)
	versionHandler := handler.NewAgentVersionHandler(versions)
	marketHandler := handler.NewAgentMarketplaceHandler(market, versions)

	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}, agentCreator: func(c *gin.Context) (string, error) {
		if c.Param("id") == "agent-owned" {
			return "contributor", nil
		}
		if c.Param("id") == "agent-other" {
			return "colleague", nil
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

	callTenant := func(tenantID uint64, method, path, role, actor string, body any) *httptest.ResponseRecorder {
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
	call := func(method, path, role, actor string, body any) *httptest.ResponseRecorder {
		return callTenant(1, method, path, role, actor, body)
	}

	frozen := call(http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
	require.Equal(t, http.StatusCreated, frozen.Code, frozen.Body.String())
	var frozenBody struct {
		Data struct {
			ID           string `json:"id"`
			SourceSHA256 string `json:"source_sha256"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(frozen.Body.Bytes(), &frozenBody))
	require.NotEmpty(t, frozenBody.Data.ID)
	require.NotEmpty(t, frozenBody.Data.SourceSHA256)
	wrongAgentVersionPath := call(http.MethodGet, "/api/v1/agents/agent-other/versions/"+frozenBody.Data.ID, "viewer", "viewer", nil)
	require.Equal(t, http.StatusNotFound, wrongAgentVersionPath.Code, wrongAgentVersionPath.Body.String())

	metadata := map[string]any{"semantic_version": "1.0.0", "display_name": "Release helper", "summary": "A helpful agent", "supported_languages": []string{"en"}, "use_cases": []string{"support"}, "minimum_weknora_capability": "1", "license_id": "MIT"}
	submitted := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "contributor", "contributor", map[string]any{"agent_version_id": frozenBody.Data.ID, "metadata": metadata})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID           string `json:"id"`
			BundleDigest string `json:"bundle_digest"`
			AuthorID     string `json:"author_id"`
			TenantID     uint64 `json:"tenant_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))
	require.NotEmpty(t, submissionBody.Data.ID)
	require.NotEmpty(t, submissionBody.Data.BundleDigest)
	require.Equal(t, "contributor", submissionBody.Data.AuthorID)
	require.Equal(t, uint64(1), submissionBody.Data.TenantID)

	viewerSubmit := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "viewer", "viewer", map[string]any{"agent_version_id": frozenBody.Data.ID, "metadata": metadata})
	require.Equal(t, http.StatusForbidden, viewerSubmit.Code)
	otherAgentFreeze := call(http.MethodPost, "/api/v1/agents/agent-other/versions", "admin", "admin", nil)
	require.Equal(t, http.StatusCreated, otherAgentFreeze.Code)
	var otherVersion struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(otherAgentFreeze.Body.Bytes(), &otherVersion))
	otherAgentSubmit := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "contributor", "contributor", map[string]any{"agent_version_id": otherVersion.Data.ID, "metadata": metadata})
	require.Equal(t, http.StatusForbidden, otherAgentSubmit.Code)
	crossTenantFreeze := call(http.MethodPost, "/api/v1/agents/missing/versions", "admin", "admin", nil)
	require.Equal(t, http.StatusNotFound, crossTenantFreeze.Code)
	crossTenantVersion := callTenant(2, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "admin", "admin", map[string]any{"agent_version_id": frozenBody.Data.ID, "metadata": metadata})
	require.Equal(t, http.StatusNotFound, crossTenantVersion.Code)

	queue := call(http.MethodGet, "/api/v1/marketplace/tenant/release-submissions/review-queue", "admin", "reviewer", nil)
	require.Equal(t, http.StatusOK, queue.Code, queue.Body.String())
	var queueBody struct {
		Data []struct {
			ID           string `json:"id"`
			BundleDigest string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(queue.Body.Bytes(), &queueBody))
	require.Len(t, queueBody.Data, 1)
	require.Equal(t, submissionBody.Data.ID, queueBody.Data[0].ID)
	require.Equal(t, submissionBody.Data.BundleDigest, queueBody.Data[0].BundleDigest)
	viewerQueue := call(http.MethodGet, "/api/v1/marketplace/tenant/release-submissions/review-queue", "viewer", "viewer", nil)
	require.Equal(t, http.StatusForbidden, viewerQueue.Code)

	approved := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "reviewer", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())
	var result struct {
		Data struct {
			Review struct {
				ReviewerID     string `json:"reviewer_id"`
				ReviewedDigest string `json:"reviewed_digest"`
			} `json:"review"`
			Release *struct {
				ID           string `json:"id"`
				BundleDigest string `json:"bundle_digest"`
			} `json:"release"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &result))
	require.Equal(t, "reviewer", result.Data.Review.ReviewerID)
	require.Equal(t, submissionBody.Data.BundleDigest, result.Data.Review.ReviewedDigest)
	require.NotNil(t, result.Data.Release)
	require.Equal(t, submissionBody.Data.BundleDigest, result.Data.Release.BundleDigest)
	storedRelease, err := marketRepo.GetRelease(context.Background(), 1, result.Data.Release.ID)
	require.NoError(t, err)
	require.NotNil(t, storedRelease)
	require.Equal(t, submissionBody.Data.BundleDigest, storedRelease.BundleDigest)
	bundlePath := filepath.Join(bundleRoot, "tenant-1", "releases", submissionBody.Data.ID, submissionBody.Data.BundleDigest)
	fileBytes, err := os.ReadFile(bundlePath)
	require.NoError(t, err)
	require.Equal(t, storedRelease.Bundle, fileBytes)
	digest := sha256.Sum256(fileBytes)
	require.Equal(t, submissionBody.Data.BundleDigest, hex.EncodeToString(digest[:]))
	viewerReview := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+submissionBody.Data.ID+"/review", "viewer", "viewer", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusForbidden, viewerReview.Code)
	spoofedReviewer := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "reviewer", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved", "reviewer_id": "spoofed"})
	require.Equal(t, http.StatusBadRequest, spoofedReviewer.Code)

	catalog := call(http.MethodGet, "/api/v1/marketplace/tenant/catalog", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, catalog.Code, catalog.Body.String())
	var catalogBody struct {
		Data []struct {
			CurrentReleaseID *string `json:"current_release_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(catalog.Body.Bytes(), &catalogBody))
	require.Len(t, catalogBody.Data, 1)
	require.NotNil(t, catalogBody.Data[0].CurrentReleaseID)
	require.Equal(t, result.Data.Release.ID, *catalogBody.Data[0].CurrentReleaseID)

	secondFrozen := call(http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
	require.Equal(t, http.StatusCreated, secondFrozen.Code, secondFrozen.Body.String())
	var secondVersion struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(secondFrozen.Body.Bytes(), &secondVersion))
	metadata["semantic_version"] = "2.0.0"
	secondSubmission := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "contributor", "contributor", map[string]any{"agent_version_id": secondVersion.Data.ID, "metadata": metadata})
	require.Equal(t, http.StatusCreated, secondSubmission.Code, secondSubmission.Body.String())
	var secondSubmissionBody struct {
		Data struct {
			ID           string `json:"id"`
			BundleDigest string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(secondSubmission.Body.Bytes(), &secondSubmissionBody))
	ownerQueue := call(http.MethodGet, "/api/v1/marketplace/tenant/release-submissions/review-queue", "owner", "owner", nil)
	require.Equal(t, http.StatusOK, ownerQueue.Code, ownerQueue.Body.String())
	require.Contains(t, ownerQueue.Body.String(), secondSubmissionBody.Data.ID)
	rejected := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+secondSubmissionBody.Data.ID+"/review", "owner", "owner", map[string]any{"expected_digest": secondSubmissionBody.Data.BundleDigest, "decision": "rejected", "reason": "needs edits"})
	require.Equal(t, http.StatusOK, rejected.Code, rejected.Body.String())
	require.NotContains(t, rejected.Body.String(), `"release":{`)
	catalogAfterReject := call(http.MethodGet, "/api/v1/marketplace/tenant/catalog", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, catalogAfterReject.Code, catalogAfterReject.Body.String())
	require.Contains(t, catalogAfterReject.Body.String(), *catalogBody.Data[0].CurrentReleaseID)

	// A second approval appends Release 2 and advances only the Listing pointer.
	listingID := storedRelease.ListingID
	var listingBefore types.AgentMarketplaceListingEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, listingID).First(&listingBefore).Error)
	firstReleaseID := storedRelease.ID
	firstReleaseBytes := append([]byte(nil), fileBytes...)
	thirdVersion := call(http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
	require.Equal(t, http.StatusCreated, thirdVersion.Code, thirdVersion.Body.String())
	var thirdVersionBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(thirdVersion.Body.Bytes(), &thirdVersionBody))
	metadata["semantic_version"] = "3.0.0"
	thirdSubmission := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "contributor", "contributor", map[string]any{"agent_version_id": thirdVersionBody.Data.ID, "metadata": metadata})
	require.Equal(t, http.StatusCreated, thirdSubmission.Code, thirdSubmission.Body.String())
	var thirdSubmissionBody struct {
		Data struct {
			ID           string `json:"id"`
			BundleDigest string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(thirdSubmission.Body.Bytes(), &thirdSubmissionBody))
	thirdApproved := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+thirdSubmissionBody.Data.ID+"/review", "admin", "reviewer-2", map[string]any{"expected_digest": thirdSubmissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusOK, thirdApproved.Code, thirdApproved.Body.String())
	var thirdResult struct {
		Data struct {
			Release *struct {
				ID           string `json:"id"`
				BundleDigest string `json:"bundle_digest"`
			} `json:"release"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(thirdApproved.Body.Bytes(), &thirdResult))
	require.NotNil(t, thirdResult.Data.Release)
	require.NotEqual(t, firstReleaseID, thirdResult.Data.Release.ID)
	secondRelease, err := marketRepo.GetRelease(context.Background(), 1, thirdResult.Data.Release.ID)
	require.NoError(t, err)
	require.Equal(t, 2, secondRelease.ReleaseNumber)
	require.NotEqual(t, storedRelease.BundleDigest, secondRelease.BundleDigest)
	firstReleaseAfterRepublish, err := marketRepo.GetRelease(context.Background(), 1, firstReleaseID)
	require.NoError(t, err)
	require.Equal(t, firstReleaseBytes, firstReleaseAfterRepublish.Bundle)
	firstFileAfterRepublish, err := os.ReadFile(bundlePath)
	require.NoError(t, err)
	require.Equal(t, firstReleaseBytes, firstFileAfterRepublish)
	require.Equal(t, storedRelease.BundleDigest, firstReleaseAfterRepublish.BundleDigest)
	var listingAfter types.AgentMarketplaceListingEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, listingID).First(&listingAfter).Error)
	require.Equal(t, listingBefore.ID, listingAfter.ID)
	require.Equal(t, listingBefore.TenantID, listingAfter.TenantID)
	require.Equal(t, listingBefore.SourceAgentID, listingAfter.SourceAgentID)
	require.Equal(t, listingBefore.DisplayName, listingAfter.DisplayName)
	require.Equal(t, listingBefore.Summary, listingAfter.Summary)
	require.Equal(t, listingBefore.State, listingAfter.State)
	require.Equal(t, listingBefore.CreatedAt, listingAfter.CreatedAt)
	require.NotEqual(t, listingBefore.CurrentReleaseID, listingAfter.CurrentReleaseID)
	require.Equal(t, thirdResult.Data.Release.ID, *listingAfter.CurrentReleaseID)
	t.Logf("republished immutable tenant release: release1_id=%s digest=%s bytes=%d; release2_id=%s digest=%s bytes=%d; listing_pointer=%s", firstReleaseID, storedRelease.BundleDigest, len(firstReleaseBytes), secondRelease.ID, secondRelease.BundleDigest, len(secondRelease.Bundle), *listingAfter.CurrentReleaseID)

	badDTO := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "contributor", "contributor", map[string]any{"agent_version_id": frozenBody.Data.ID, "metadata": metadata, "unexpected": "reject"})
	require.Equal(t, http.StatusBadRequest, badDTO.Code)
	spoofedPrincipal := call(http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "contributor", "contributor", map[string]any{"agent_version_id": frozenBody.Data.ID, "metadata": metadata, "tenant_id": 999, "actor_id": "spoofed"})
	require.Equal(t, http.StatusBadRequest, spoofedPrincipal.Code)

	reviewPolicy := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/:id/review")
	if !reviewPolicy.RequireFullAccess || !policyHasCapability(reviewPolicy, types.APIKeyCapabilityManageAgents) {
		t.Fatalf("review policy = %#v, want manage_agents/full-access", reviewPolicy)
	}
	queuePolicy := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/marketplace/tenant/release-submissions/review-queue")
	if !queuePolicy.RequireFullAccess || len(queuePolicy.Capabilities) != 0 {
		t.Fatalf("queue policy = %#v, want full-access only", queuePolicy)
	}
}

func openTenantAgentMarketplaceHTTPTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dbPath := filepath.Join(t.TempDir(), "tenant-marketplace.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, m.Up())
	_, _ = m.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, err := db.DB()
		if err == nil {
			_ = conn.Close()
		}
	})
	return db
}
