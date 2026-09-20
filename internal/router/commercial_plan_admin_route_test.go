package router

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	commercialsvc "github.com/Tencent/WeKnora/internal/application/service/commercial"
	commercial "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/handler"
	commercialplatform "github.com/Tencent/WeKnora/internal/infrastructure/commercialplatform"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// planAdminEnv is the plan-admin route harness: one sqlite database, the
// commercial grants table, the (optional) PlanVersionService platform, and
// a fresh engine per request — gin freezes middleware chains at route
// registration, so per-request auth needs a per-request engine.
type planAdminEnv struct {
	db          *gorm.DB
	platform    commercial.CommercialPlatform
	wireService bool
	fake        *commercialplatform.FakeAdapter
}

// platformSession admits the human carrying the platform-scope plan_publish
// grant (seeded by newPlanAdminEnv).
func platformSession(t *testing.T, db *gorm.DB) gin.HandlerFunc {
	t.Helper()
	mustExec(t, db, `INSERT OR REPLACE INTO commercial_grants (tenant_id, user_id, capability, granted_by)
		VALUES (0, 'publisher-user', 'plan_publish', 'test')`)
	return authAs(0, "publisher-user", "owner")
}

func mustExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatal(err)
	}
}

// newPlanAdminEnv builds the environment. platform nil + wireService true
// wires the service with a nil platform (blocked-env); wireService false
// leaves the handler without the service (fail-closed 503).
func newPlanAdminEnv(t *testing.T, platform commercial.CommercialPlatform, wireService bool) *planAdminEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS commercial_grants (
		tenant_id INTEGER NOT NULL,
		user_id TEXT NOT NULL,
		capability TEXT NOT NULL,
		granted_by TEXT NOT NULL,
		version INTEGER NOT NULL DEFAULT 1,
		PRIMARY KEY (tenant_id, user_id, capability))`)
	fake, _ := platform.(*commercialplatform.FakeAdapter)
	return &planAdminEnv{db: db, platform: platform, wireService: wireService, fake: fake}
}

// engine builds a fresh engine with the given auth middleware standing in
// for the production global Auth layer.
func (e *planAdminEnv) engine(auth gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.NewCommercialHandler(e.db)
	if e.wireService {
		svc, err := commercialsvc.NewPlanVersionService(e.db, e.platform)
		if err != nil {
			panic(err)
		}
		h.SetPlanVersionService(svc)
	}
	engine := gin.New()
	engine.Use(auth)
	v1 := engine.Group("/api/v1")
	RegisterCommercialRoutes(v1, h)
	return engine
}

// withPlatformKey returns an auth middleware attaching a platform API key
// scope (scope.IsPlatform()).
func withPlatformKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(types.WithTenantAPIKeyScope(c.Request.Context(),
			types.TenantAPIKeyScope{ScopeType: types.APIKeyScopePlatform}))
		c.Next()
	}
}

// withTenantBillingGrant admits a tenant user carrying the space billing
// grant — the identity that must NEVER reach the plan admin surface.
func withTenantBillingGrant(t *testing.T, db *gorm.DB) gin.HandlerFunc {
	t.Helper()
	mustExec(t, db, `INSERT OR REPLACE INTO commercial_grants (tenant_id, user_id, capability, granted_by)
		VALUES (101, 'tenant-biller', 'billing', 'test')`)
	return authAs(101, "tenant-biller", "admin")
}

func (e *planAdminEnv) do(t *testing.T, auth gin.HandlerFunc, method, path string, body string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.engine(auth).ServeHTTP(w, req)
	return w, w.Body.String()
}

const planAdminDraftBody = `{
	"plan_key": "pro",
	"name": "Pro",
	"amount_fen": "9900",
	"currency": "CNY",
	"included_credits_micro": "9900000",
	"features": {"api_access": true},
	"limits": {"members": 10}
}`

// TestPlanAdminRequiresPlatformPublisher: catalog operations are
// cross-space — a space admin with a tenant billing grant, a tenant owner,
// and anonymous callers are all rejected; only the platform-scope
// plan_publish grant or a platform API key is admitted.
func TestPlanAdminRequiresPlatformPublisher(t *testing.T) {
	endpoints := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/admin/plans/drafts"},
		{http.MethodPatch, "/api/v1/admin/plans/drafts/pro/1"},
		{http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/validate"},
		{http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/publish"},
		{http.MethodGet, "/api/v1/admin/plans/versions"},
		{http.MethodGet, "/api/v1/admin/plans/versions/pro/1"},
	}

	t.Run("tenant billing grant is 403 everywhere", func(t *testing.T) {
		env := newPlanAdminEnv(t, commercialplatform.NewFakeAdapter(), true)
		auth := withTenantBillingGrant(t, env.db)
		for _, ep := range endpoints {
			w, body := env.do(t, auth, ep.method, ep.path, planAdminDraftBody)
			if w.Code != http.StatusForbidden {
				t.Fatalf("%s %s: status = %d, want 403 (body %s)", ep.method, ep.path, w.Code, body)
			}
		}
	})

	t.Run("tenant owner is 403", func(t *testing.T) {
		env := newPlanAdminEnv(t, commercialplatform.NewFakeAdapter(), true)
		w, body := env.do(t, authAs(101, "owner-user", "owner"), http.MethodPost, "/api/v1/admin/plans/drafts", planAdminDraftBody)
		if w.Code != http.StatusForbidden {
			t.Fatalf("owner status = %d, want 403 (body %s)", w.Code, body)
		}
	})

	t.Run("anonymous is rejected at the auth layer", func(t *testing.T) {
		env := newPlanAdminEnv(t, commercialplatform.NewFakeAdapter(), true)
		// sessionGate aborts 401 when no session header rides along.
		w, body := env.do(t, sessionGate, http.MethodPost, "/api/v1/admin/plans/drafts", planAdminDraftBody)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous status = %d, want 401 (body %s)", w.Code, body)
		}
	})

	t.Run("platform grant is admitted", func(t *testing.T) {
		env := newPlanAdminEnv(t, commercialplatform.NewFakeAdapter(), true)
		w, body := env.do(t, platformSession(t, env.db), http.MethodPost, "/api/v1/admin/plans/drafts", planAdminDraftBody)
		if w.Code != http.StatusCreated {
			t.Fatalf("platform grant status = %d, want 201 (body %s)", w.Code, body)
		}
	})

	t.Run("platform API key is admitted", func(t *testing.T) {
		env := newPlanAdminEnv(t, commercialplatform.NewFakeAdapter(), true)
		w, body := env.do(t, withPlatformKey(), http.MethodPost, "/api/v1/admin/plans/drafts", planAdminDraftBody)
		if w.Code != http.StatusCreated {
			t.Fatalf("platform key status = %d, want 201 (body %s)", w.Code, body)
		}
	})
}

// TestPlanAdminLifecycleOverFake: create draft → validate (itemized axes) →
// publish → the version row is published and list/get serve the receipt
// block and state.
func TestPlanAdminLifecycleOverFake(t *testing.T) {
	env := newPlanAdminEnv(t, commercialplatform.NewFakeAdapter(), true)
	auth := platformSession(t, env.db)

	w, body := env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts", planAdminDraftBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("create draft status = %d body=%s", w.Code, body)
	}
	data := decodeEnvelope(t, body)
	wantString(t, data, "plan_key", "pro")
	wantString(t, data, "state", "draft")
	wantString(t, data, "amount_fen", "9900")
	wantString(t, data, "currency", "CNY")
	if _, leaked := data["plan_code"]; leaked {
		t.Fatal("the external plan code must never appear in an admin response")
	}

	w, body = env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/validate", `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("validate status = %d body=%s", w.Code, body)
	}
	data = decodeEnvelope(t, body)
	validation, ok := data["validation"].(map[string]any)
	if !ok {
		t.Fatalf("validate must answer the itemized report, got %s", body)
	}
	if valid, _ := validation["valid"].(bool); !valid {
		t.Fatalf("clean draft must validate: %s", body)
	}
	axes, _ := validation["axes"].([]any)
	if len(axes) != 6 {
		t.Fatalf("the report must itemize six axes, got %d (%s)", len(axes), body)
	}

	w, body = env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/publish", `{"reason":"t07 test"}`)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("publish status = %d body=%s", w.Code, body)
	}
	data = decodeEnvelope(t, body)
	wantString(t, data, "state", "published")
	receipt, ok := data["receipt"].(map[string]any)
	if !ok {
		t.Fatalf("publish must answer the receipt block, got %s", body)
	}
	if received, _ := receipt["received"].(bool); !received {
		t.Fatalf("receipt.received must be true: %s", body)
	}
	if _, has := receipt["published_at"]; !has {
		t.Fatalf("receipt must carry published_at: %s", body)
	}
	wantString(t, receipt, "command_key", "publish_plan_version:pro:1")

	w, body = env.do(t, auth, http.MethodGet, "/api/v1/admin/plans/versions", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", w.Code, body)
	}
	if !strings.Contains(body, `"state":"published"`) || !strings.Contains(body, `"command_key":"publish_plan_version:pro:1"`) {
		t.Fatalf("list must serve state + receipt: %s", body)
	}

	w, body = env.do(t, auth, http.MethodGet, "/api/v1/admin/plans/versions/pro/1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", w.Code, body)
	}
	data = decodeEnvelope(t, body)
	if data["receipt"] == nil {
		t.Fatalf("get must carry the receipt block: %s", body)
	}
}

// leakyPublishPlatform simulates the worst case: adapter errors carrying
// provider identifiers, URLs and raw text the handler must never echo.
type leakyPublishPlatform struct {
	mode string // unreachable | conflict
}

func (l *leakyPublishPlatform) SubmitCommand(context.Context, commercial.Command) (commercial.CommandReceipt, error) {
	switch l.mode {
	case "unreachable":
		return commercial.CommandReceipt{}, fmt.Errorf("%w: lago POST http://lago-marker-79001.invalid/api/v1/plans failed: dial tcp: lookup lago-api: no such host",
			commercial.ErrPlatformUnreachable)
	default:
		return commercial.CommandReceipt{}, fmt.Errorf("%w: lago 422 at http://lago-marker-79001.invalid/api/v1/plans body value_already_exist",
			commercial.ErrPlatformInvalidResponse)
	}
}

func (l *leakyPublishPlatform) ReadSnapshot(context.Context, commercial.SnapshotQuery) (commercial.Snapshot, error) {
	return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
}

func (l *leakyPublishPlatform) Reconcile(context.Context, commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return commercial.ReconciliationPage{}, commercial.ErrPlatformUnsupported
}

// assertPlanAdminNoLeak: every response body must be provider-neutral — no
// external plan code, no provider name, no marker, no URL, no raw text.
func assertPlanAdminNoLeak(t *testing.T, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	for _, marker := range []string{"weknora-pro-v", "lago", "lago-marker", "http://", "/api/v1/plans", "value_already_exist", "no such host"} {
		if strings.Contains(lower, strings.ToLower(marker)) {
			t.Fatalf("provider leakage in response body: %q contains %q", body, marker)
		}
	}
}

// TestPlanAdminProviderNeutrality: forced unconfigured / unreachable /
// conflict paths answer closed tokens only — never the external plan code,
// a provider identifier, a URL or raw error text.
func TestPlanAdminProviderNeutrality(t *testing.T) {
	seed := func(t *testing.T, env *planAdminEnv, auth gin.HandlerFunc) {
		w, body := env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts", planAdminDraftBody)
		if w.Code != http.StatusCreated {
			t.Fatalf("seed draft: %d %s", w.Code, body)
		}
	}

	t.Run("unreachable leaky platform maps to closed 503", func(t *testing.T) {
		env := newPlanAdminEnv(t, &leakyPublishPlatform{mode: "unreachable"}, true)
		auth := platformSession(t, env.db)
		seed(t, env, auth)
		w, body := env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/publish", `{"reason":"r"}`)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("unreachable status = %d, want 503 (body %s)", w.Code, body)
		}
		if !strings.Contains(body, "platform_unreachable") {
			t.Fatalf("the closed token must appear: %s", body)
		}
		assertPlanAdminNoLeak(t, body)
	})

	t.Run("conflict leaky platform maps to closed 409", func(t *testing.T) {
		env := newPlanAdminEnv(t, &leakyPublishPlatform{mode: "conflict"}, true)
		auth := platformSession(t, env.db)
		seed(t, env, auth)
		w, body := env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/publish", `{"reason":"r"}`)
		if w.Code != http.StatusConflict {
			t.Fatalf("conflict status = %d, want 409 (body %s)", w.Code, body)
		}
		if !strings.Contains(body, "publish_conflict") {
			t.Fatalf("the closed token must appear: %s", body)
		}
		assertPlanAdminNoLeak(t, body)
	})

	t.Run("real lago adapter at a marker origin leaks nothing", func(t *testing.T) {
		env := newPlanAdminEnv(t, commercialplatform.NewLagoAdapter(commercialplatform.Config{
			Provider: commercialplatform.ProviderLago,
			BaseURL:  "http://lago-marker-79001.invalid",
			APIKey:   "secret-for-test-only",
			Release:  "v1.53.0",
		}), true)
		auth := platformSession(t, env.db)
		seed(t, env, auth)
		w, body := env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/publish", `{"reason":"r"}`)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("unreachable lago status = %d, want 503 (body %s)", w.Code, body)
		}
		assertPlanAdminNoLeak(t, body)
	})

	t.Run("unconfigured nil platform maps to closed 503", func(t *testing.T) {
		env := newPlanAdminEnv(t, nil, true)
		auth := platformSession(t, env.db)
		seed(t, env, auth)
		w, body := env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/publish", `{"reason":"r"}`)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("nil platform status = %d, want 503 (body %s)", w.Code, body)
		}
		if !strings.Contains(body, "platform_unconfigured") {
			t.Fatalf("the closed token must appear: %s", body)
		}
		assertPlanAdminNoLeak(t, body)
	})
}

// TestPlanAdminImmutabilitySurface: PATCH on a published version answers
// 409 published_plan_immutable; PATCH on a draft is 200; a doubled publish
// is one receipt, no error.
func TestPlanAdminImmutabilitySurface(t *testing.T) {
	env := newPlanAdminEnv(t, commercialplatform.NewFakeAdapter(), true)
	auth := platformSession(t, env.db)

	if w, body := env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts", planAdminDraftBody); w.Code != http.StatusCreated {
		t.Fatalf("seed: %d %s", w.Code, body)
	}
	if w, body := env.do(t, auth, http.MethodPatch, "/api/v1/admin/plans/drafts/pro/1", planAdminDraftBody); w.Code != http.StatusOK {
		t.Fatalf("patch draft: %d %s", w.Code, body)
	}
	if w, body := env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/publish", `{"reason":"r"}`); w.Code/100 != 2 {
		t.Fatalf("publish: %d %s", w.Code, body)
	}

	w, body := env.do(t, auth, http.MethodPatch, "/api/v1/admin/plans/drafts/pro/1", planAdminDraftBody)
	if w.Code != http.StatusConflict {
		t.Fatalf("patch published status = %d, want 409 (body %s)", w.Code, body)
	}
	if !strings.Contains(body, "published_plan_immutable") {
		t.Fatalf("the closed token must appear: %s", body)
	}

	w, body = env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/publish", `{"reason":"replay"}`)
	if w.Code/100 != 2 {
		t.Fatalf("publish replay must be a no-op success: %d %s", w.Code, body)
	}
	first := decodeEnvelope(t, body)
	// The replay returns the SAME receipt — command key identical.
	if receipt, ok := first["receipt"].(map[string]any); ok {
		wantString(t, receipt, "command_key", "publish_plan_version:pro:1")
	} else {
		t.Fatalf("replay must still carry the receipt: %s", body)
	}
	if count := len(env.fake.Commands()); count != 1 {
		t.Fatalf("the doubled publish must issue exactly one seam command, got %d", count)
	}
}

// TestPlanAdminNilServiceFailsClosed: with no service wired the draft
// endpoints answer a closed 503 — never a fabricated success.
func TestPlanAdminNilServiceFailsClosed(t *testing.T) {
	env := newPlanAdminEnv(t, nil, false)
	auth := platformSession(t, env.db)
	for _, ep := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/admin/plans/drafts", planAdminDraftBody},
		{http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/validate", `{}`},
		{http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/publish", `{}`},
		{http.MethodGet, "/api/v1/admin/plans/versions", ""},
	} {
		w, body := env.do(t, auth, ep.method, ep.path, ep.body)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s: status = %d, want 503 (body %s)", ep.method, ep.path, w.Code, body)
		}
		if strings.Contains(body, "success\":true") {
			t.Fatalf("no fabricated success allowed: %s", body)
		}
	}
}

// TestPlanAdminNotOnTenantGroup: the tenant-scoped member view keeps
// listing PUBLISHED versions only and is unaffected by admin drafts; after
// an admin publish the member list sees the version.
func TestPlanAdminNotOnTenantGroup(t *testing.T) {
	env := newPlanAdminEnv(t, commercialplatform.NewFakeAdapter(), true)
	admin := platformSession(t, env.db)
	member := authAs(101, "member-user", "viewer")

	if w, body := env.do(t, admin, http.MethodPost, "/api/v1/admin/plans/drafts", planAdminDraftBody); w.Code != http.StatusCreated {
		t.Fatalf("seed: %d %s", w.Code, body)
	}
	w, body := env.do(t, member, http.MethodGet, "/api/v1/commercial/plans", "")
	if w.Code != http.StatusOK {
		t.Fatalf("member plans status = %d body=%s", w.Code, body)
	}
	if strings.Contains(body, "\"plan_key\":\"pro\"") {
		t.Fatalf("a DRAFT must never appear in the member catalog: %s", body)
	}

	if w, body := env.do(t, admin, http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/publish", `{"reason":"r"}`); w.Code/100 != 2 {
		t.Fatalf("publish: %d %s", w.Code, body)
	}
	w, body = env.do(t, member, http.MethodGet, "/api/v1/commercial/plans", "")
	if w.Code != http.StatusOK || !strings.Contains(body, "\"plan_key\":\"pro\"") {
		t.Fatalf("the PUBLISHED version must appear in the member catalog: %d %s", w.Code, body)
	}
}

// TestPlanAdminValidationFailureAnswers422Itemized: publishing an invalid
// draft answers 422 with the per-axis report and the row stays draft.
func TestPlanAdminValidationFailureAnswers422Itemized(t *testing.T) {
	env := newPlanAdminEnv(t, commercialplatform.NewFakeAdapter(), true)
	auth := platformSession(t, env.db)
	invalid := strings.Replace(planAdminDraftBody, `"amount_fen": "9900"`, `"amount_fen": "12345"`, 1)
	if w, body := env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts", invalid); w.Code != http.StatusCreated {
		t.Fatalf("seed invalid draft: %d %s", w.Code, body)
	}
	w, body := env.do(t, auth, http.MethodPost, "/api/v1/admin/plans/drafts/pro/1/publish", `{"reason":"r"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid publish status = %d, want 422 (body %s)", w.Code, body)
	}
	if !strings.Contains(body, "base_price_tier") {
		t.Fatalf("the itemized report must name the failed axis: %s", body)
	}
	// Zero seam calls: the fake recorded nothing.
	if env.fake != nil && len(env.fake.Commands()) != 0 {
		t.Fatalf("an invalid draft must never reach the seam, got %d", len(env.fake.Commands()))
	}
	// The row stays draft.
	var state string
	if err := env.db.Raw(`SELECT state FROM commercial_plan_catalog WHERE plan_key='pro' AND version=1`).Scan(&state).Error; err != nil {
		t.Fatal(err)
	}
	if state != commercial.PlanStateDraft {
		t.Fatalf("state = %q, want draft", state)
	}
}
