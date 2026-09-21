package session

// O04 HTTP tests: the usage endpoint's access matrix (owner / viewer /
// admin / cross-tenant), its response shape (usage fields + as_of, main/child
// calls, sandbox residency, checks — and structurally no money and no
// credentials), the fail-closed route absence, and the O03 tombstone entry
// at the session-deletion handler.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newCraftUsageHTTPEnv mounts the usage route over the real view service
// (real DB, real usage ledger, real lifecycle residency, real version store)
// with only the session read ACL faked exactly as the W03 harness fakes it.
func newCraftUsageHTTPEnv(t *testing.T) (*gin.Engine, *service.CraftUsageService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openCraftHTTPDB(t)
	require.NoError(t, db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('sess-usage', 1, 'usage', 'u1', 'trpc')").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, deadline, created_at, updated_at) VALUES (1, 'run-1', 'sess-usage', 'u1', 'req-1', 'am-1', 'rh', '{}', 'succeeded', ?, ?, ?)",
		time.Unix(1000, 0).UTC(), time.Unix(900, 0).UTC(), time.Unix(950, 0).UTC()).Error)

	sessionsACL := &craftHTTPSessions{db: db}
	store := repository.NewCraftStore(db)
	usage := service.NewCraftUsageService(repository.NewCraftUsageStore(db))

	// Facts: one known platform main call + one unknown byok child call.
	mainCall := usage.IssueCallID(1, "run-1", "", "m-main", "platform", 1)
	_, err := usage.RecordPhysicalCall(context.Background(), service.PhysicalCall{
		TenantID: 1, RunID: "run-1", CallID: mainCall, AttemptID: usage.IssueAttemptID(mainCall, 1),
		Runtime: craft.RuntimeMain, ModelID: "m-main", Funding: "platform",
		Usage: &craft.UsageTotals{Input: 10, Output: 20, Facts: 1},
	})
	require.NoError(t, err)
	ocCall := usage.IssueCallID(1, "run-1", "dlg-1", "m-child", "byok", 1)
	_, err = usage.RecordPhysicalCall(context.Background(), service.PhysicalCall{
		TenantID: 1, RunID: "run-1", DelegationID: "dlg-1", CallID: ocCall,
		AttemptID: usage.IssueAttemptID(ocCall, 1),
		Runtime:   craft.RuntimeOC, ModelID: "m-child", Funding: "byok", Usage: nil,
	})
	require.NoError(t, err)

	lifecycle, err := service.NewCraftLifecycle(service.CraftLifecycleConfig{
		DB: db, Store: store, Bindings: sandbox.NewMemorySessionSandboxBindingStore(),
		ActiveRuns:     service.CraftActiveRunsQuery(db),
		SessionExists:  service.NewCraftSessionExistence(db),
		SandboxDeleter: noopSandboxDeleter{}, // never reached: the read path records no deletions
	})
	require.NoError(t, err)
	_, err = lifecycle.RecordSandboxEvent(context.Background(), 1, "sess-usage", "sbx-1",
		craft.LifecycleEventSandboxStart, time.Unix(800, 0).UTC())
	require.NoError(t, err)
	_, err = lifecycle.RecordSandboxEvent(context.Background(), 1, "sess-usage", "sbx-1",
		craft.LifecycleEventSandboxStop, time.Unix(860, 0).UTC())
	require.NoError(t, err)

	versions := repository.NewCraftVersionStore(db)
	ws, err := store.PutWorkspace(context.Background(), craft.Workspace{
		Scope:             craft.Scope{TenantID: 1, UserID: "u1", SessionID: "sess-usage"},
		OpenCodeSessionID: "oc-1", RuntimeDigest: "sha256:runtime",
	}, 0)
	require.NoError(t, err)
	digest, err := craft.ManifestDigest(nil)
	require.NoError(t, err)
	_, err = versions.Publish(context.Background(), craft.Scope{TenantID: 1, UserID: "u1", SessionID: "sess-usage"}, craft.Version{
		ID: craft.VersionID(ws.ID, "run-1", digest), WorkspaceID: ws.ID, RunID: "run-1", Kind: craft.KindWeb,
		Checks: []craft.Check{{Name: craft.CheckPreview, Status: craft.CheckPassed, Detail: "controlled preview served this version's files"}},
	})
	require.NoError(t, err)

	view, err := service.NewCraftUsageViewService(service.CraftUsageViewConfig{
		DB: db, Sessions: sessionsACL, Usage: usage, Residency: lifecycle, Versions: versions,
	})
	require.NoError(t, err)
	RegisterCraftUsageHandler(view)
	t.Cleanup(func() { RegisterCraftUsageHandler(nil) })

	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.Use(func(c *gin.Context) {
		identity := c.GetHeader("X-Test-Identity")
		tenant := uint64(1)
		user, role := "u1", ""
		switch identity {
		case "viewer":
			user = "u2"
		case "admin":
			user, role = "u2", "admin"
		case "foreigntenant":
			tenant, user = 2, "u1"
		}
		c.Set(types.TenantIDContextKey.String(), tenant)
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenant)
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		if role == "admin" {
			ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	sessions := engine.Group("/api/v1/sessions")
	RegisterCraftSessionRoutes(nil, sessions, nil, nil)
	return engine, usage
}

type noopSandboxDeleter struct{}

func (noopSandboxDeleter) Delete(context.Context, uint64, string) error { return nil }

func getCraftUsage(t *testing.T, engine *gin.Engine, identity string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/sess-usage/craft/usage", nil)
	req.Header.Set("X-Test-Identity", identity)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w, w.Body.String()
}

// The access matrix: owner and tenant admin read the view; a plain member of
// another owner's session and a foreign tenant cannot even see it.
func TestCraftUsageHTTPAccessMatrix(t *testing.T) {
	engine, _ := newCraftUsageHTTPEnv(t)

	owner, _ := getCraftUsage(t, engine, "owner")
	require.Equal(t, http.StatusOK, owner.Code)
	viewer, _ := getCraftUsage(t, engine, "viewer")
	require.Equal(t, http.StatusNotFound, viewer.Code)
	admin, _ := getCraftUsage(t, engine, "admin")
	require.Equal(t, http.StatusOK, admin.Code)
	foreign, _ := getCraftUsage(t, engine, "foreigntenant")
	require.Equal(t, http.StatusNotFound, foreign.Code)
}

// The response shape: usage fields + explicit as_of, main/child calls with
// the unknown line visible, sandbox residency, checks — and NO money and NO
// credential material anywhere in the payload.
func TestCraftUsageHTTPShapeHidesMoneyAndSecrets(t *testing.T) {
	engine, _ := newCraftUsageHTTPEnv(t)
	w, raw := getCraftUsage(t, engine, "owner")
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			AsOf  string `json:"as_of"`
			Usage struct {
				KnownCalls   int    `json:"known_calls"`
				UnknownCalls int    `json:"unknown_calls"`
				InputTokens  int64  `json:"input_tokens"`
				OutputTokens int64  `json:"output_tokens"`
				CachedTokens int64  `json:"cached_tokens"`
				Funding      string `json:"funding"`
			} `json:"usage"`
			Byok bool `json:"byok_model_borne_by_space"`
			Runs []struct {
				RunID  string `json:"run_id"`
				Status string `json:"status"`
			} `json:"runs"`
			Calls []struct {
				Runtime string `json:"runtime"`
				Status  string `json:"status"`
			} `json:"calls"`
			Residency *struct {
				Starts       int     `json:"starts"`
				DwellSeconds float64 `json:"dwell_seconds"`
			} `json:"sandbox_residency"`
			Checks []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"checks"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	require.True(t, body.Success)
	_, err := time.Parse(time.RFC3339, body.Data.AsOf)
	require.NoError(t, err, "as_of must be an explicit RFC3339 stamp")
	require.Equal(t, 1, body.Data.Usage.KnownCalls)
	require.Equal(t, 1, body.Data.Usage.UnknownCalls)
	require.Equal(t, int64(10), body.Data.Usage.InputTokens)
	require.Equal(t, int64(20), body.Data.Usage.OutputTokens)
	require.Equal(t, "mixed", body.Data.Usage.Funding)
	require.True(t, body.Data.Byok)
	require.Len(t, body.Data.Runs, 1)
	require.Equal(t, "run-1", body.Data.Runs[0].RunID)
	require.Len(t, body.Data.Calls, 2)
	runtimes := []string{body.Data.Calls[0].Runtime, body.Data.Calls[1].Runtime}
	require.Contains(t, runtimes, "main")
	require.Contains(t, runtimes, "oc")
	for _, call := range body.Data.Calls {
		if call.Runtime == "oc" {
			require.Equal(t, craft.UsageStatusUnknown, call.Status, "the unknown child call stays visible as unknown")
		}
	}
	require.NotNil(t, body.Data.Residency)
	require.Equal(t, 1, body.Data.Residency.Starts)
	require.InDelta(t, 60.0, body.Data.Residency.DwellSeconds, 1e-9)
	require.Len(t, body.Data.Checks, 1)
	require.Equal(t, craft.CheckPreview, body.Data.Checks[0].Name)

	lower := strings.ToLower(raw)
	for _, forbidden := range []string{"amount", "cost", "price", "credit", "api_key", "apikey", "credential", "secret"} {
		require.NotContains(t, lower, forbidden, "usage payload must not carry %q", forbidden)
	}
}

// Fail-closed: without the registration the route does not exist at all.
func TestCraftUsageHTTPRouteAbsentWithoutRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	RegisterCraftUsageHandler(nil)
	engine := gin.New()
	sessions := engine.Group("/api/v1/sessions")
	RegisterCraftSessionRoutes(nil, sessions, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/any/craft/usage", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// The O03 tombstone-entry tests live in session_delete_tombstone_test.go
// (they belong to the lifecycle wiring commit, not the O04 usage surface).
