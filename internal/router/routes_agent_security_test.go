package router

// T34 #64 Task 9: AC1-AC3 end-to-end evidence over the real HTTP stack.
// Governance wiring (release/dependency revocation routes), the workbench
// execution entry, and the chat-turn entry (File B) must refuse new work for
// a security-revoked release or exact-dependency identity, dispose in-flight
// work per disposition, and keep the audit/history ledger intact.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appservice "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	session "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// agentSecurityLockQueue feeds the REAL HTTP submission flow a deterministic
// sequence of dependency locks, one per published release.
type agentSecurityLockQueue struct{ locks []types.DependencyLock }

func (q *agentSecurityLockQueue) Resolve(context.Context, uint64, types.AgentVersionSnapshot) (types.DependencyLock, error) {
	if len(q.locks) == 0 {
		return types.DependencyLock{Dependencies: []types.AgentReleaseDependency{}}, nil
	}
	lock := q.locks[0]
	q.locks = q.locks[1:]
	return lock, nil
}

// agentSecurityE2EApp mounts the real governance, marketplace, and workbench
// surfaces on one migrated SQLite database: revocation routes, the full
// adoption/release flow, and the workbench execution entry wired exactly like
// the container (WithBinding constructor + security gate + published-version
// resolver pins).
func agentSecurityE2EApp(t *testing.T, locks ...types.DependencyLock) (*gin.Engine, *gorm.DB, *appservice.AgentSecurityService) {
	t.Helper()
	db := openTenantAgentMarketplaceHTTPTestDB(t)
	require.NoError(t, db.Exec("INSERT INTO tenants (id, name, business) VALUES (1, 'security-e2e', 'test')").Error)
	require.NoError(t, db.Create(&types.CustomAgent{ID: "agent-owned", Name: "Security helper", TenantID: 1, CreatedBy: "contributor", Config: types.CustomAgentConfig{AgentMode: "smart-reasoning", SystemPrompt: "Be useful."}}).Error)
	// Workbench admission is actor-fenced: Admit requires an active users +
	// tenant_members row for the acting identity.
	require.NoError(t, db.Exec("INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('admin', 'admin', 'admin@test', 'x', 1)").Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at)
		VALUES (1,'admin','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	// Tenant 2 exists and has its own admin, but no access to tenant 1 rows.
	require.NoError(t, db.Exec("INSERT INTO tenants (id, name, business) VALUES (2, 'security-e2e-2', 'test')").Error)
	require.NoError(t, db.Exec("INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('admin2', 'admin2', 'admin2@test', 'x', 2)").Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at)
		VALUES (2,'admin2','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)

	customAgents := appservice.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil, nil)
	versions := appservice.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	market := appservice.NewAgentMarketplaceService(versions, &agentSecurityLockQueue{locks: locks}, repository.NewAgentMarketplaceRepository(db), t.TempDir())
	adoptionsRepo := repository.NewAgentAdoptionRepository(db)
	adoptions := appservice.NewAgentAdoptionService(adoptionsRepo, customAgents, versions)
	security := appservice.NewAgentSecurityService(repository.NewAgentSecurityStore(db), repository.NewAgentRunStore(db))
	security.SetAgentVersionService(versions)

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
	r.Use(newLifecycleIdentityMiddleware())
	v1 := r.Group("/api/v1")
	RegisterAgentVersionRoutes(v1, handler.NewAgentVersionHandler(versions), g)
	RegisterAgentMarketplaceRoutes(v1, handler.NewAgentMarketplaceHandler(market, versions), g)
	RegisterAgentAdoptionRoutes(v1, handler.NewAgentAdoptionHandler(adoptions), g)
	RegisterAgentMarketplaceLifecycleRoutes(v1, handler.NewAgentMarketplaceLifecycleHandler(appservice.NewAgentMarketplaceLifecycleService(adoptionsRepo, repository.NewAgentMarketplaceRepository(db))), g)
	RegisterAgentSecurityRoutes(v1, handler.NewAgentSecurityHandler(security), g)
	r.POST("/api/v1/workbench/executions", newLifecycleIdentityMiddleware(), session.NewWorkbenchStartHandler(coordinator).Start)
	return r, db, security
}

func agentSecuritySeedSessions(t *testing.T, db *gorm.DB, ids ...string) {
	t.Helper()
	for _, id := range ids {
		require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, 1, ?, 'admin', 'trpc')", id, id).Error)
	}
}

// agentSecurityStart posts a workbench execution start over the real engine.
func agentSecurityStart(t *testing.T, r *gin.Engine, tenant uint64, sessionID, requestID, agentID string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"session_id":"` + sessionID + `","agent_id":"` + agentID + `","target_id":"platform","request_id":"` + requestID + `","text":"e2e turn","budget_upper":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", "admin")
	req.Header.Set("X-Test-Actor", "admin")
	if tenant == 2 {
		req.Header.Set("X-Test-Actor", "admin2")
	}
	if tenant == 2 {
		req.Header.Set("X-Test-Tenant", "2")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func agentSecurityRevokeRelease(t *testing.T, r *gin.Engine, tenant uint64, body map[string]any) (int, agentSecurityRevocationBody) {
	t.Helper()
	resp := adoptionCall(r, tenant, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", "admin", "admin", body)
	var view agentSecurityRevocationBody
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &view))
	return resp.Code, view
}

type agentSecurityRevocationBody struct {
	Success bool `json:"success"`
	Data    struct {
		ID                   string `json:"id"`
		Kind                 string `json:"kind"`
		Reason               string `json:"reason"`
		RevokedBy            string `json:"revoked_by"`
		InFlightDisposition  string `json:"in_flight_disposition"`
		CanceledRunCount     int64  `json:"canceled_run_count"`
		RunCancellationState string `json:"run_cancellation_state"`
		ListingID            string `json:"listing_id"`
		ReleaseID            string `json:"release_id"`
		ReplacementReleaseID string `json:"replacement_release_id"`
		Scope                *struct {
			BlockedReleases []struct {
				ReleaseID string `json:"ReleaseID"`
				BlockedBy string `json:"BlockedBy"`
			} `json:"blocked_releases"`
			AffectedAdoptionIDs []string `json:"affected_adoption_ids"`
			AffectedVariants    []struct {
				VariantID    string `json:"VariantID"`
				AdoptionID   string `json:"AdoptionID"`
				ReleaseID    string `json:"ReleaseID"`
				State        string `json:"State"`
				LocalAgentID string `json:"LocalAgentID"`
			} `json:"affected_variants"`
		} `json:"scope"`
	} `json:"data"`
}

func agentSecurityRunRow(t *testing.T, db *gorm.DB, requestID string) map[string]any {
	t.Helper()
	var row map[string]any
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND request_id = ?", 1, requestID).Take(&row).Error)
	return row
}

// TestAgentSecurityE2EReleaseRevocationBlocksNewTaskRunAndDisposesInFlight
// proves AC1 for the workbench entry: an admitted in-flight run carries the
// server-resolved Version/Release pins, a cancel-disposition revocation
// disposes that run durably (status/wait_reason/event/session slot), and NEW
// work on the revoked release is refused with zero durable side effects.
func TestAgentSecurityE2EReleaseRevocationBlocksNewTaskRunAndDisposesInFlight(t *testing.T) {
	r, db, _ := agentSecurityE2EApp(t)
	listingID, releaseID := freezeAndPublishUpgradeRelease(t, r, "1.0.0", lifecycleMetadata())
	_, _, localAgentID := adoptAndPublishFirstVariant(t, r, listingID)
	agentSecuritySeedSessions(t, db, "s-wb", "s-wb-post")

	live := agentSecurityStart(t, r, 1, "s-wb", "req-wb-live", localAgentID)
	require.Equal(t, http.StatusAccepted, live.Code, live.Body.String())
	var liveBody struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(live.Body.Bytes(), &liveBody))
	runRow := agentSecurityRunRow(t, db, "req-wb-live")
	require.Equal(t, liveBody.Data.RunID, runRow["run_id"])
	var variant types.AgentAdoptionVariantEntity
	require.NoError(t, db.Where("tenant_id = ? AND local_agent_id = ?", 1, localAgentID).Take(&variant).Error)
	require.Equal(t, variant.LocalAgentVersionID, runRow["security_local_agent_version_id"], "run must pin the server-resolved Version")
	require.Equal(t, releaseID, runRow["security_release_id"], "run must pin the server-resolved Release")
	require.Equal(t, "queued", runRow["status"])

	code, view := agentSecurityRevokeRelease(t, r, 1, map[string]any{
		"release_id": releaseID, "reason": "e2e compromised bundle", "in_flight_disposition": "cancel"})
	require.Equal(t, http.StatusCreated, code)
	require.Equal(t, int64(1), view.Data.CanceledRunCount, "the in-flight pinned run must be disposed")
	require.Equal(t, "complete", view.Data.RunCancellationState)

	disposed := agentSecurityRunRow(t, db, "req-wb-live")
	require.Equal(t, "canceled", disposed["status"])
	require.Equal(t, "agent_security_revocation", disposed["wait_reason"])
	var cancellationEvents int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?", 1, liveBody.Data.RunID, "cancellation_requested").Count(&cancellationEvents).Error)
	require.EqualValues(t, 1, cancellationEvents, "disposal must append the run cancellation event")
	var slot map[string]any
	require.NoError(t, db.Table("sessions").Where("tenant_id = ? AND id = ?", 1, "s-wb").Select("active_agent_run_id").Take(&slot).Error)
	require.Nil(t, slot["active_agent_run_id"], "the disposed run must release the session slot")

	refused := agentSecurityStart(t, r, 1, "s-wb-post", "req-wb-post", localAgentID)
	require.Equal(t, http.StatusConflict, refused.Code, refused.Body.String())
	var deniedRequests, deniedRuns int64
	require.NoError(t, db.Table("workbench_requests").Where("tenant_id = ? AND request_id = ?", 1, "req-wb-post").Count(&deniedRequests).Error)
	require.Zero(t, deniedRequests, "denied start must not create a durable request")
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND session_id = ?", 1, "s-wb-post").Count(&deniedRuns).Error)
	require.Zero(t, deniedRuns, "denied start must not create a run")
}

// TestAgentSecurityE2EDependencyRevocationNeverSubstitutesByName proves the
// exact-dependency revocation semantics: only the locked (type,id,version,
// digest) tuple is blocked. A release that locks the same dependency NAME at
// a different version/digest stays runnable — revocation never widens to a
// name-level match.
func TestAgentSecurityE2EDependencyRevocationNeverSubstitutesByName(t *testing.T) {
	oldDep := types.AgentReleaseDependency{Type: "model", ID: "gpt-x", Version: "1.0.0", Digest: strings.Repeat("a", 64), LicenseID: "MIT"}
	newDep := types.AgentReleaseDependency{Type: "model", ID: "gpt-x", Version: "1.1.0", Digest: strings.Repeat("b", 64), LicenseID: "MIT"}
	r, db, _ := agentSecurityE2EApp(t,
		types.DependencyLock{Dependencies: []types.AgentReleaseDependency{oldDep}},
		types.DependencyLock{Dependencies: []types.AgentReleaseDependency{newDep}})
	listingID, releaseOld := freezeAndPublishUpgradeRelease(t, r, "1.0.0", lifecycleMetadata())
	require.NotEmpty(t, releaseOld)
	adoptionID, _, agentOld := adoptAndPublishFirstVariant(t, r, listingID)
	_, releaseNew := freezeAndPublishUpgradeRelease(t, r, "1.1.0", lifecycleMetadata())

	// A second variant on the same adoption pins to the newer release
	// explicitly, giving a distinct local Agent per release.
	variant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionID+"/variants", "admin", "admin", map[string]any{"name": "v-next", "release_id": releaseNew})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))
	agentNew := mapTestPublishUpgradeVariant(t, r, variantBody.Data.ID, "gpt-y", "kb-next")

	agentSecuritySeedSessions(t, db, "s-wb-dep-digest", "s-wb-dep-digest-b")
	resp := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/dependencies", "admin", "admin", map[string]any{
		"dependency": map[string]any{"type": oldDep.Type, "id": oldDep.ID, "version": oldDep.Version, "digest": oldDep.Digest},
		"reason":     "e2e vulnerable model", "in_flight_disposition": "cancel"})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	blocked := agentSecurityStart(t, r, 1, "s-wb-dep-digest", "req-dep-old", agentOld)
	require.Equal(t, http.StatusConflict, blocked.Code, blocked.Body.String())
	var blockedRuns int64
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND session_id = ?", 1, "s-wb-dep-digest").Count(&blockedRuns).Error)
	require.Zero(t, blockedRuns, "exact-dependency revocation must refuse new work")

	allowed := agentSecurityStart(t, r, 1, "s-wb-dep-digest-b", "req-dep-new", agentNew)
	require.Equal(t, http.StatusAccepted, allowed.Code, allowed.Body.String())
	pinned := agentSecurityRunRow(t, db, "req-dep-new")
	require.Equal(t, releaseNew, pinned["security_release_id"], "the same-name different-digest release must still admit and pin its own identity")
}

// TestAgentSecurityE2ERevocationKeepsHistoryReasonScopeReplacement proves the
// append-only ledger: revocation terminalizes in-flight work but keeps the
// historical run/request/messages rows, records reason, replacement release
// and scope in the audit view, and a committed request replays its original
// run instead of being denied or re-executed.
func TestAgentSecurityE2ERevocationKeepsHistoryReasonScopeReplacement(t *testing.T) {
	r, db, _ := agentSecurityE2EApp(t)
	listingID, v1 := freezeAndPublishUpgradeRelease(t, r, "1.0.0", lifecycleMetadata())
	adoptionID, variantID, localAgentID := adoptAndPublishFirstVariant(t, r, listingID)
	_, v2 := freezeAndPublishUpgradeRelease(t, r, "2.0.0", lifecycleMetadata())
	agentSecuritySeedSessions(t, db, "s-live")

	live := agentSecurityStart(t, r, 1, "s-live", "req-live", localAgentID)
	require.Equal(t, http.StatusAccepted, live.Code, live.Body.String())
	var liveBody struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(live.Body.Bytes(), &liveBody))
	historyRun := agentSecurityRunRow(t, db, "req-live")
	require.Equal(t, "queued", historyRun["status"])
	var userMessages []types.Message
	require.NoError(t, db.Where("session_id = ? AND role = ?", "s-live", "user").Find(&userMessages).Error)
	require.Len(t, userMessages, 1)

	code, view := agentSecurityRevokeRelease(t, r, 1, map[string]any{
		"release_id": v1, "reason": "e2e supply-chain compromise",
		"replacement_release_id": v2, "in_flight_disposition": "cancel"})
	require.Equal(t, http.StatusCreated, code)
	require.Equal(t, "e2e supply-chain compromise", view.Data.Reason)
	require.Equal(t, v2, view.Data.ReplacementReleaseID)
	require.Equal(t, v1, view.Data.ReleaseID)
	require.Equal(t, "admin", view.Data.RevokedBy)

	detail := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/security-revocations/"+view.Data.ID, "admin", "admin", nil)
	require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
	var fetched agentSecurityRevocationBody
	require.NoError(t, json.Unmarshal(detail.Body.Bytes(), &fetched))
	require.NotNil(t, fetched.Data.Scope)
	require.NotEmpty(t, fetched.Data.Scope.BlockedReleases)
	require.Equal(t, v1, fetched.Data.Scope.BlockedReleases[0].ReleaseID)
	require.Contains(t, fetched.Data.Scope.AffectedAdoptionIDs, adoptionID)
	require.Len(t, fetched.Data.Scope.AffectedVariants, 1)
	require.Equal(t, variantID, fetched.Data.Scope.AffectedVariants[0].VariantID)
	require.Equal(t, localAgentID, fetched.Data.Scope.AffectedVariants[0].LocalAgentID)

	after := agentSecurityRunRow(t, db, "req-live")
	require.Equal(t, liveBody.Data.RunID, after["run_id"], "history row must survive revocation")
	require.Equal(t, "canceled", after["status"])
	var requestAfter map[string]any
	require.NoError(t, db.Table("workbench_requests").Where("tenant_id = ? AND request_id = ?", 1, "req-live").Take(&requestAfter).Error)
	require.Equal(t, "admitted", requestAfter["state"], "the settled request must keep its admitted history")
	var userAfter []types.Message
	require.NoError(t, db.Where("session_id = ? AND role = ?", "s-live", "user").Find(&userAfter).Error)
	require.Len(t, userAfter, 1, "message history must survive revocation")
	var variantAfter types.AgentAdoptionVariantEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, variantID).Take(&variantAfter).Error)
	require.Equal(t, v1, variantAfter.ReleaseID, "variant lineage must not be rewritten by revocation")

	replay := agentSecurityStart(t, r, 1, "s-live", "req-live", localAgentID)
	require.Equal(t, http.StatusAccepted, replay.Code, replay.Body.String())
	var replayBody struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(replay.Body.Bytes(), &replayBody))
	require.Equal(t, liveBody.Data.RunID, replayBody.Data.RunID, "a committed request replays its original run, not a denial")
}

// TestAgentSecurityE2EGovernanceFloorAndTenantIsolation proves the governance
// floor (admin-only revocation, validated input, fail-closed unknown
// releases) and tenant isolation (another tenant cannot revoke a foreign
// release or reach a foreign session through the workbench entry).
func TestAgentSecurityE2EGovernanceFloorAndTenantIsolation(t *testing.T) {
	r, db, _ := agentSecurityE2EApp(t)
	listingID, releaseID := freezeAndPublishUpgradeRelease(t, r, "1.0.0", lifecycleMetadata())
	_, _, localAgentID := adoptAndPublishFirstVariant(t, r, listingID)
	agentSecuritySeedSessions(t, db, "s-wb")
	revokePath := "/api/v1/marketplace/tenant/security-revocations/releases"

	viewer := adoptionCall(r, 1, http.MethodPost, revokePath, "viewer", "viewer", map[string]any{"release_id": releaseID, "reason": "x"})
	require.Equal(t, http.StatusForbidden, viewer.Code, viewer.Body.String())
	contributor := adoptionCall(r, 1, http.MethodPost, revokePath, "contributor", "contributor", map[string]any{"release_id": releaseID, "reason": "x"})
	require.Equal(t, http.StatusForbidden, contributor.Code, contributor.Body.String())
	missingReason := adoptionCall(r, 1, http.MethodPost, revokePath, "admin", "admin", map[string]any{"release_id": releaseID})
	require.Equal(t, http.StatusBadRequest, missingReason.Code)
	unknownRelease := adoptionCall(r, 1, http.MethodPost, revokePath, "admin", "admin", map[string]any{"release_id": "missing-release", "reason": "x"})
	require.Equal(t, http.StatusBadRequest, unknownRelease.Code)
	foreignReplacement := adoptionCall(r, 1, http.MethodPost, revokePath, "admin", "admin", map[string]any{"release_id": releaseID, "reason": "x", "replacement_release_id": "other-listing-release"})
	require.Equal(t, http.StatusBadRequest, foreignReplacement.Code)

	crossTenant := adoptionCall(r, 2, http.MethodPost, revokePath, "admin", "admin", map[string]any{"release_id": releaseID, "reason": "foreign actor"})
	require.Equal(t, http.StatusBadRequest, crossTenant.Code, "a foreign tenant's release must read as unresolvable")
	var tenant1Revocations int64
	require.NoError(t, db.Table("agent_release_revocations").Where("tenant_id = ?", 1).Count(&tenant1Revocations).Error)
	require.Zero(t, tenant1Revocations, "refused governance calls must leave no ledger row")

	accepted := agentSecurityStart(t, r, 1, "s-wb", "req-gov-tenant", localAgentID)
	require.Equal(t, http.StatusAccepted, accepted.Code, accepted.Body.String())
	tenant2Start := agentSecurityStart(t, r, 2, "s-wb", "req-gov-tenant", localAgentID)
	require.NotEqual(t, http.StatusAccepted, tenant2Start.Code, "another tenant must not admit work on a foreign session/agent")
	var tenant2Runs int64
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ?", 2).Count(&tenant2Runs).Error)
	require.Zero(t, tenant2Runs)
}
