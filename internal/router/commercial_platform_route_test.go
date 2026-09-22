package router

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/handler"
	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
	commercialplatform "github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// sessionGate stands in for the production global Auth middleware: it
// attaches the authenticated identity when the request carries a session
// and aborts with 401 otherwise — an anonymous caller never reaches the
// commercial routes.
func sessionGate(c *gin.Context) {
	principal := c.GetHeader("X-WeKnora-Test-Session")
	if principal == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(101))
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole("viewer"))
	ctx = context.WithValue(ctx, types.UserIDContextKey, principal)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

// serveReadiness builds the commercial route engine (the commercial_scope
// harness pattern) with the given platform injected and performs ONE
// authenticated-or-anonymous GET /commercial/platform/readiness.
func serveReadiness(t *testing.T, auth gin.HandlerFunc, p commercial.CommercialPlatform) (*httptest.ResponseRecorder, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	h := handler.NewCommercialHandler(db)
	if p != nil {
		h.SetCommercialPlatform(p)
	}
	engine := gin.New()
	if auth != nil {
		engine.Use(auth)
	}
	v1 := engine.Group("/api/v1")
	RegisterCommercialRoutes(v1, h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/commercial/platform/readiness", nil)
	req.Header.Set("X-WeKnora-Test-Session", "session-user") // anonymous callers override below
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w, w.Body.String()
}

// leakyPlatform simulates the worst case the endpoint must survive: an
// adapter whose error string carries provider identifiers, URLs and paths.
// The handler must map it to the closed token and echo none of it.
type leakyPlatform struct{ reads int }

func (l *leakyPlatform) SubmitCommand(context.Context, commercial.Command) (commercial.CommandReceipt, error) {
	return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
}

func (l *leakyPlatform) ReadSnapshot(context.Context, commercial.SnapshotQuery) (commercial.Snapshot, error) {
	l.reads++
	return commercial.Snapshot{}, fmt.Errorf("%w: lago GET http://lago-marker-48897.invalid/health failed: dial tcp: lookup lago-api: no such host",
		commercial.ErrPlatformUnreachable)
}

func (l *leakyPlatform) Reconcile(context.Context, commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return commercial.ReconciliationPage{}, commercial.ErrPlatformUnsupported
}

// TestCommercialPlatformReadinessServesClosedEnvelope: the ready fake
// answers 200 with the exact closed field set state/release/checked_at/
// reason, and degraded/unavailable snapshots pass through with their tokens.
func TestCommercialPlatformReadinessServesClosedEnvelope(t *testing.T) {
	ready := commercialplatform.NewFakeAdapter()
	ready.SetReadiness(commercial.ReadinessSnapshot{
		State: commercial.ReadinessReady, Release: "v1.53.0",
		CheckedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
	})
	w, body := serveReadiness(t, authAs(101, "owner-user", "owner"), ready)
	if w.Code != http.StatusOK {
		t.Fatalf("ready status = %d body=%s", w.Code, body)
	}
	data := decodeEnvelope(t, body)
	if len(data) != 4 {
		t.Fatalf("the readiness envelope must carry exactly state/release/checked_at/reason, got %v", data)
	}
	wantString(t, data, "state", "ready")
	wantString(t, data, "release", "v1.53.0")
	wantString(t, data, "reason", "")
	checkedAt, _ := data["checked_at"].(string)
	if checkedAt == "" {
		t.Fatalf("checked_at must be present and non-empty: %s", body)
	}

	// An empty release is omitted rather than answered as an empty string.
	noRelease := commercialplatform.NewFakeAdapter()
	noRelease.SetReadiness(commercial.ReadinessSnapshot{
		State: commercial.ReadinessReady, CheckedAt: time.Now().UTC(),
	})
	w, body = serveReadiness(t, authAs(101, "owner-user", "owner"), noRelease)
	if w.Code != http.StatusOK {
		t.Fatalf("no-release status = %d body=%s", w.Code, body)
	}
	data = decodeEnvelope(t, body)
	if _, has := data["release"]; has {
		t.Fatalf("unknown release must be omitted, got %s", body)
	}
	wantString(t, data, "state", "ready")

	// Degraded/unavailable snapshots pass through with their closed tokens.
	for _, tc := range []struct{ state, reason string }{
		{string(commercial.ReadinessDegraded), "unreachable"},
		{string(commercial.ReadinessUnavailable), "unconfigured"},
	} {
		fake := commercialplatform.NewFakeAdapter()
		fake.SetReadiness(commercial.ReadinessSnapshot{
			State: commercial.ReadinessState(tc.state), Reason: tc.reason,
			CheckedAt: time.Now().UTC(),
		})
		w, body = serveReadiness(t, authAs(101, "owner-user", "owner"), fake)
		if w.Code != http.StatusOK {
			t.Fatalf("state %s status = %d body=%s", tc.state, w.Code, body)
		}
		data = decodeEnvelope(t, body)
		wantString(t, data, "state", tc.state)
		wantString(t, data, "reason", tc.reason)
	}
}

// TestCommercialPlatformReadinessFailsClosedAndNeverLeaks: nil platform and
// every adapter failure class answer 200 with the closed product envelope —
// never a raw error, never a provider identifier, URL, path or marker.
func TestCommercialPlatformReadinessFailsClosedAndNeverLeaks(t *testing.T) {
	assertNoLeak := func(t *testing.T, body string) {
		t.Helper()
		lower := body
		for _, marker := range []string{"lago-marker-48897", "/health", "lago", "invalid"} {
			if containsFold(lower, marker) {
				t.Fatalf("provider leakage in response body: %q contains %q", body, marker)
			}
		}
	}

	t.Run("nil platform is honest unconfigured unavailable", func(t *testing.T) {
		w, body := serveReadiness(t, authAs(101, "owner-user", "owner"), nil)
		if w.Code != http.StatusOK {
			t.Fatalf("nil platform status = %d body=%s", w.Code, body)
		}
		data := decodeEnvelope(t, body)
		wantString(t, data, "state", "unavailable")
		wantString(t, data, "reason", "unconfigured")
		assertNoLeak(t, body)
	})

	t.Run("unconfigured adapter maps to unconfigured", func(t *testing.T) {
		w, body := serveReadiness(t, authAs(101, "owner-user", "owner"), commercialplatform.NewFakeAdapter())
		if w.Code != http.StatusOK {
			t.Fatalf("unconfigured status = %d body=%s", w.Code, body)
		}
		data := decodeEnvelope(t, body)
		wantString(t, data, "state", "unavailable")
		wantString(t, data, "reason", "unconfigured")
		assertNoLeak(t, body)
	})

	t.Run("unreachable adapter carrying provider text leaks nothing", func(t *testing.T) {
		leaky := &leakyPlatform{}
		w, body := serveReadiness(t, authAs(101, "owner-user", "owner"), leaky)
		if w.Code != http.StatusOK {
			t.Fatalf("unreachable status = %d body=%s", w.Code, body)
		}
		data := decodeEnvelope(t, body)
		wantString(t, data, "state", "unavailable")
		wantString(t, data, "reason", "unreachable")
		assertNoLeak(t, body)
		if leaky.reads != 1 {
			t.Fatalf("the platform must be read exactly once, got %d", leaky.reads)
		}
	})

	t.Run("real lago adapter at an unreachable marker origin leaks nothing", func(t *testing.T) {
		p := commercialplatform.NewLagoAdapter(commercialplatform.Config{
			Provider: commercialplatform.ProviderLago,
			BaseURL:  "http://lago-marker-48897.invalid",
			APIKey:   "secret-for-test-only",
			Release:  "v1.53.0",
		})
		w, body := serveReadiness(t, authAs(101, "owner-user", "owner"), p)
		if w.Code != http.StatusOK {
			t.Fatalf("real-adapter status = %d body=%s", w.Code, body)
		}
		data := decodeEnvelope(t, body)
		wantString(t, data, "state", "unavailable")
		wantString(t, data, "reason", "unreachable")
		assertNoLeak(t, body)
	})
}

// TestCommercialPlatformReadinessAuthGates: anonymous callers are rejected
// by the auth layer, an API key without the explicit commercial capability
// gets 403, an explicit commercial capability passes, and a plain member's
// GET is not billing-write-gated (reads bypass the billing-role gate by
// design).
func TestCommercialPlatformReadinessAuthGates(t *testing.T) {
	ready := commercialplatform.NewFakeAdapter()
	ready.SetReadiness(commercial.ReadinessSnapshot{
		State: commercial.ReadinessReady, Release: "v1.53.0", CheckedAt: time.Now().UTC(),
	})

	t.Run("anonymous blocked at the auth layer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/commercial/platform/readiness", nil)
		gin.SetMode(gin.TestMode)
		db, err := gorm.Open(sqlite.Open("file:anon-readiness?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		t.Cleanup(func() {
			if sqlDB, err := db.DB(); err == nil {
				_ = sqlDB.Close()
			}
		})
		platform := &countingPlatform{inner: ready}
		h := handler.NewCommercialHandler(db)
		h.SetCommercialPlatform(platform)
		engine := gin.New()
		engine.Use(sessionGate)
		v1 := engine.Group("/api/v1")
		RegisterCommercialRoutes(v1, h)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous status = %d, want 401 body=%s", w.Code, w.Body.String())
		}
		if platform.reads != 0 {
			t.Fatalf("the platform must never be read for an anonymous caller, got %d", platform.reads)
		}
	})

	t.Run("api key without explicit commercial capability is 403", func(t *testing.T) {
		withScope := func(scope types.TenantAPIKeyScope) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.Request = c.Request.WithContext(types.WithTenantAPIKeyScope(c.Request.Context(), scope))
				c.Next()
			}
		}
		w, body := serveReadiness(t, withScope(types.TenantAPIKeyScope{FullAccess: true}), ready)
		if w.Code != http.StatusForbidden {
			t.Fatalf("full-access key status = %d, want 403 body=%s", w.Code, body)
		}
		w, body = serveReadiness(t, withScope(types.TenantAPIKeyScope{
			Capabilities: types.StringArray{string(handler.CommercialAPIKeyCapability)},
		}), ready)
		if w.Code != http.StatusOK {
			t.Fatalf("explicit commercial capability status = %d body=%s", w.Code, body)
		}
	})

	t.Run("plain member GET passes the billing write gate by design", func(t *testing.T) {
		// role "viewer" is a plain member: the RequireManageBillingForWrites
		// gate only applies to non-read methods, so this read succeeds.
		w, body := serveReadiness(t, authAs(101, "member-user", "viewer"), ready)
		if w.Code != http.StatusOK {
			t.Fatalf("member GET status = %d body=%s", w.Code, body)
		}
		if !containsFold(body, `"state":"ready"`) {
			t.Fatalf("member GET must reach the readiness answer, got %s", body)
		}
	})
}

// countingPlatform wraps a platform and counts readiness reads.
type countingPlatform struct {
	inner commercial.CommercialPlatform
	reads int
}

func (c *countingPlatform) SubmitCommand(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	return c.inner.SubmitCommand(ctx, cmd)
}

func (c *countingPlatform) ReadSnapshot(ctx context.Context, query commercial.SnapshotQuery) (commercial.Snapshot, error) {
	c.reads++
	return c.inner.ReadSnapshot(ctx, query)
}

func (c *countingPlatform) Reconcile(ctx context.Context, from commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return c.inner.Reconcile(ctx, from)
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
