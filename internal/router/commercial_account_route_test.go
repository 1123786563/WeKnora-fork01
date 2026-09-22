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
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newAccountEngine builds the commercial route engine over shared-cache
// SQLite with the commercial_billing_accounts projection table, the given
// platform wired through the REAL BillingAccountService (lazy ensure at the
// API seam), and an optional tenants table for the display-name path. name
// keeps the shared-cache DSN distinct when one test builds several engines.
func newAccountEngine(t *testing.T, name string, platform commercial.CommercialPlatform, seedTenantNames bool) (*gin.Engine, *gorm.DB, *commercialplatform.FakeAdapter) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&repocommercial.BillingAccount{}); err != nil {
		t.Fatal(err)
	}
	if seedTenantNames {
		if err := db.Exec(`CREATE TABLE IF NOT EXISTS tenants (id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '')`).Error; err != nil {
			t.Fatal(err)
		}
		_ = db.Exec(`INSERT INTO tenants (id, name) VALUES (101, 'Space One')`).Error
		_ = db.Exec(`INSERT INTO tenants (id, name) VALUES (102, 'Space Two')`).Error
	}
	h := handler.NewCommercialHandler(db)
	var fake *commercialplatform.FakeAdapter
	if platform == nil {
		fake = nil
	} else if f, ok := platform.(*commercialplatform.FakeAdapter); ok {
		fake = f
	}
	if platform != nil {
		h.SetCommercialPlatform(platform)
		svc, err := commercialsvc.NewBillingAccountService(db, platform)
		if err != nil {
			t.Fatal(err)
		}
		h.SetBillingAccountService(svc)
	}
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	RegisterCommercialRoutes(v1, h)
	return engine, db, fake
}

// performAccount runs ONE authenticated-or-anonymous GET /commercial/account
// through the given auth middleware.
func performAccount(t *testing.T, engine *gin.Engine, auth gin.HandlerFunc) (*httptest.ResponseRecorder, string) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/commercial/account", nil)
	if auth != nil {
		// attach the identity by wrapping the engine's handler chain
		served := gin.New()
		served.Use(auth)
		served.Handle(http.MethodGet, "/api/v1/*rest", func(c *gin.Context) {
			engine.ServeHTTP(c.Writer, c.Request)
		})
		served.ServeHTTP(w, req)
		return w, w.Body.String()
	}
	engine.ServeHTTP(w, req)
	return w, w.Body.String()
}

// TestCommercialAccountLazyEnsureLinksAtTheAPISeam: an authenticated member
// GET on an unlinked space answers 200 linked with ensured_at, the authority
// (fake) holds EXACTLY one customer under weknora-tenant-101, and the local
// projection row was created — the lazy ensure fired at the API boundary.
// A repeat GET stays at exactly one customer (idempotent at the boundary).
func TestCommercialAccountLazyEnsureLinksAtTheAPISeam(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	engine, db, _ := newAccountEngine(t, t.Name()+"-engine", fake, true)

	w, body := performAccount(t, engine, authAs(101, "member-user", "viewer"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, body)
	}
	data := decodeEnvelope(t, body)
	wantString(t, data, "state", "linked")
	wantString(t, data, "reason", "")
	if ensured, _ := data["ensured_at"].(string); ensured == "" {
		t.Fatalf("linked answer must carry ensured_at: %s", body)
	}
	customers := fake.Customers()
	if len(customers) != 1 || customers[0].ExternalID != "weknora-tenant-101" {
		t.Fatalf("exactly one customer under the derived identity, got %+v", customers)
	}
	if customers[0].Name != "Space One" {
		t.Fatalf("the display name rides the ensure command, got %+v", customers[0])
	}
	var n int64
	if err := db.Raw(`SELECT COUNT(*) FROM commercial_billing_accounts WHERE tenant_id = 101`).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("the local projection row must be created, got %d", n)
	}

	// Repeat GET: idempotent at the boundary — still exactly one customer.
	w, body = performAccount(t, engine, authAs(101, "member-user", "viewer"))
	if w.Code != http.StatusOK {
		t.Fatalf("repeat status = %d body=%s", w.Code, body)
	}
	wantString(t, decodeEnvelope(t, body), "state", "linked")
	if len(fake.Customers()) != 1 {
		t.Fatalf("repeat GET must never create a second customer, got %+v", fake.Customers())
	}
}

// TestCommercialAccountCrossTenantNeverLeaks (AC2 at the API): two spaces
// authed in turn on one engine — each answer reflects only its own space,
// and neither body carries the other space's identity.
func TestCommercialAccountCrossTenantNeverLeaks(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	engine, _, _ := newAccountEngine(t, t.Name()+"-engine", fake, true)

	w, bodyA := performAccount(t, engine, authAs(101, "user-a", "owner"))
	if w.Code != http.StatusOK {
		t.Fatalf("A status = %d body=%s", w.Code, bodyA)
	}
	w, bodyB := performAccount(t, engine, authAs(102, "user-b", "owner"))
	if w.Code != http.StatusOK {
		t.Fatalf("B status = %d body=%s", w.Code, bodyB)
	}
	if strings.Contains(bodyA, "weknora-tenant-102") {
		t.Fatalf("A's body must never carry B's identity: %s", bodyA)
	}
	if strings.Contains(bodyB, "weknora-tenant-101") {
		t.Fatalf("B's body must never carry A's identity: %s", bodyB)
	}
	if len(fake.Customers()) != 2 {
		t.Fatalf("two spaces hold two customers, got %+v", fake.Customers())
	}
}

// accountLeakyPlatform simulates the worst case on the account path: an
// adapter whose snapshot AND submit errors carry provider identifiers, URLs
// and paths. The endpoint must map everything to closed tokens and echo
// none of it.
type accountLeakyPlatform struct{ reads, submits int }

func (l *accountLeakyPlatform) SubmitCommand(context.Context, commercial.Command) (commercial.CommandReceipt, error) {
	l.submits++
	return commercial.CommandReceipt{}, fmt.Errorf("%w: lago POST http://lago-marker-48899.invalid/api/v1/customers failed: dial tcp: lookup lago-api: no such host",
		commercial.ErrPlatformUnreachable)
}

func (l *accountLeakyPlatform) ReadSnapshot(context.Context, commercial.SnapshotQuery) (commercial.Snapshot, error) {
	l.reads++
	return commercial.Snapshot{}, fmt.Errorf("%w: lago GET http://lago-marker-48899.invalid/health failed: dial tcp: lookup lago-api: no such host",
		commercial.ErrPlatformUnreachable)
}

func (l *accountLeakyPlatform) Reconcile(context.Context, commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return commercial.ReconciliationPage{}, commercial.ErrPlatformUnsupported
}

func assertAccountNoLeak(t *testing.T, body string) {
	t.Helper()
	for _, marker := range []string{
		"lago", "marker-48899", "/api/v1/customers", "/health",
		"external_id", "weknora-tenant-", "dial tcp", "invalid/unreachable",
	} {
		if containsFold(body, marker) {
			t.Fatalf("provider leakage in account body: %q contains %q", body, marker)
		}
	}
}

// TestCommercialAccountFailsClosedAndNeverLeaks: nil service, an
// unreachable fault-injected fake, a leaky adapter and a real Lago adapter
// at a marker origin all answer 200 with the CLOSED envelope only — never a
// raw error, never a provider identifier, URL, path or identity.
func TestCommercialAccountFailsClosedAndNeverLeaks(t *testing.T) {
	t.Run("nil service is honest pending unconfigured", func(t *testing.T) {
		engine, db, _ := newAccountEngine(t, t.Name()+"-engine", nil, false)
		_ = db
		// No platform, no service: the handler answers the closed envelope.
		w, body := performAccount(t, engine, authAs(101, "owner-user", "owner"))
		if w.Code != http.StatusOK {
			t.Fatalf("nil-service status = %d body=%s", w.Code, body)
		}
		data := decodeEnvelope(t, body)
		wantString(t, data, "state", "pending")
		wantString(t, data, "reason", "unconfigured")
		if _, has := data["ensured_at"]; has {
			t.Fatalf("never-ensured pending must omit ensured_at, got %s", body)
		}
		assertAccountNoLeak(t, body)
	})

	t.Run("unreachable fault maps to pending unreachable without leak", func(t *testing.T) {
		fake := commercialplatform.NewFakeAdapter()
		fake.FailSubmitsWith(commercial.ErrPlatformUnreachable)
		engine, _, _ := newAccountEngine(t, t.Name()+"-engine", fake, false)
		w, body := performAccount(t, engine, authAs(101, "owner-user", "owner"))
		if w.Code != http.StatusOK {
			t.Fatalf("unreachable status = %d body=%s", w.Code, body)
		}
		data := decodeEnvelope(t, body)
		wantString(t, data, "state", "pending")
		wantString(t, data, "reason", "unreachable")
		if _, has := data["ensured_at"]; has {
			t.Fatalf("never-ensured pending must omit ensured_at, got %s", body)
		}
		assertAccountNoLeak(t, body)
	})

	t.Run("leaky adapter on both failure classes leaks nothing", func(t *testing.T) {
		leaky := &accountLeakyPlatform{}
		engine, _, _ := newAccountEngine(t, t.Name()+"-engine", leaky, false)
		w, body := performAccount(t, engine, authAs(101, "owner-user", "owner"))
		if w.Code != http.StatusOK {
			t.Fatalf("leaky status = %d body=%s", w.Code, body)
		}
		data := decodeEnvelope(t, body)
		wantString(t, data, "state", "pending")
		wantString(t, data, "reason", "unreachable")
		assertAccountNoLeak(t, body)
		if leaky.reads != 1 || leaky.submits != 1 {
			t.Fatalf("the recovery flow reads then submits, got reads=%d submits=%d", leaky.reads, leaky.submits)
		}
	})

	t.Run("real lago adapter at an unreachable marker origin leaks nothing", func(t *testing.T) {
		p := commercialplatform.NewLagoAdapter(commercialplatform.Config{
			Provider: commercialplatform.ProviderLago,
			BaseURL:  "http://lago-marker-48899.invalid",
			APIKey:   "secret-for-test-only",
			Release:  "v1.53.0",
		})
		engine, _, _ := newAccountEngine(t, t.Name()+"-engine", p, false)
		w, body := performAccount(t, engine, authAs(101, "owner-user", "owner"))
		if w.Code != http.StatusOK {
			t.Fatalf("real-adapter status = %d body=%s", w.Code, body)
		}
		data := decodeEnvelope(t, body)
		wantString(t, data, "state", "pending")
		wantString(t, data, "reason", "unreachable")
		assertAccountNoLeak(t, body)
	})
}

// TestCommercialAccountAuthGates: anonymous callers are rejected at the
// auth layer with the platform never read; a full-access API key without
// the explicit commercial capability is 403; a plain member GET passes the
// billing write gate by design (read).
func TestCommercialAccountAuthGates(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()

	t.Run("anonymous blocked before the platform is read", func(t *testing.T) {
		engine, _, _ := newAccountEngine(t, t.Name()+"-engine", fake, false)
		w, body := performAccount(t, engine, sessionGate)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous status = %d, want 401 body=%s", w.Code, body)
		}
		if len(fake.Customers()) != 0 {
			t.Fatalf("the platform must never be touched for an anonymous caller, got %+v", fake.Customers())
		}
	})

	t.Run("full-access api key without commercial capability is 403", func(t *testing.T) {
		engine, _, _ := newAccountEngine(t, t.Name()+"-engine", fake, false)
		withScope := func(scope types.TenantAPIKeyScope) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.Request = c.Request.WithContext(types.WithTenantAPIKeyScope(c.Request.Context(), scope))
				c.Next()
			}
		}
		w, body := performAccount(t, engine, withScope(types.TenantAPIKeyScope{FullAccess: true}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("full-access key status = %d, want 403 body=%s", w.Code, body)
		}
		if len(fake.Customers()) != 0 {
			t.Fatalf("a 403 must never ensure an account, got %+v", fake.Customers())
		}
	})

	t.Run("plain member GET is not billing-write gated", func(t *testing.T) {
		engine, _, _ := newAccountEngine(t, t.Name()+"-engine", fake, false)
		w, body := performAccount(t, engine, authAs(101, "member-user", "viewer"))
		if w.Code != http.StatusOK {
			t.Fatalf("member GET status = %d body=%s", w.Code, body)
		}
		if !containsFold(body, `"state":"linked"`) {
			t.Fatalf("member GET must reach the account answer, got %s", body)
		}
	})
}

// TestCommercialAccountEnvelopeIsExactlyClosed: the linked field set is
// exactly state/reason/ensured_at — no provider correlation identity ever
// crosses; the pending-never-ensured set is exactly state/reason.
func TestCommercialAccountEnvelopeIsExactlyClosed(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	engine, _, _ := newAccountEngine(t, t.Name()+"-engine", fake, false)
	_, body := performAccount(t, engine, authAs(101, "owner-user", "owner"))
	data := decodeEnvelope(t, body)
	if len(data) != 3 {
		t.Fatalf("linked envelope must carry exactly state/reason/ensured_at, got %v", data)
	}
	wantString(t, data, "state", "linked")
	if ensured, _ := data["ensured_at"].(string); ensured == "" || !isRFC3339(ensured) {
		t.Fatalf("ensured_at must be RFC3339, got %q", ensured)
	}

	failing := commercialplatform.NewFakeAdapter()
	failing.FailSubmitsWith(commercial.ErrPlatformUnreachable)
	engine2, _, _ := newAccountEngine(t, t.Name()+"-engine2", failing, false)
	_, body2 := performAccount(t, engine2, authAs(101, "owner-user", "owner"))
	data2 := decodeEnvelope(t, body2)
	if len(data2) != 2 {
		t.Fatalf("pending-never-ensured envelope must carry exactly state/reason, got %v", data2)
	}
	wantString(t, data2, "state", "pending")
	wantString(t, data2, "reason", "unreachable")
}

func isRFC3339(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}
