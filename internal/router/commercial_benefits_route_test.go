package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"context"

	"github.com/Tencent/WeKnora/internal/handler"
	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
	commercialplatform "github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	"github.com/gin-gonic/gin"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newBenefitsEngine builds the commercial route engine with the REAL
// BenefitsService chain (seed → account → subscription → grant →
// projection) behind GET /commercial/account, over shared-cache SQLite
// (the newAccountEngine convention).
func newBenefitsEngine(t *testing.T, platform commercial.CommercialPlatform) (*gin.Engine, *gorm.DB, *commercialplatform.FakeAdapter) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	name := t.Name() + "-engine"
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
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS tenants (
		id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '', storage_used INTEGER NOT NULL DEFAULT 0
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS tenant_members (
		id INTEGER PRIMARY KEY AUTOINCREMENT, user_id TEXT, tenant_id INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'active', deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	h := handler.NewCommercialHandler(db)
	var fake *commercialplatform.FakeAdapter
	if platform != nil {
		h.SetCommercialPlatform(platform)
		accounts, err := commercialsvc.NewBillingAccountService(db, platform)
		if err != nil {
			t.Fatal(err)
		}
		plans, err := commercialsvc.NewPlanVersionService(db, platform)
		if err != nil {
			t.Fatal(err)
		}
		benefits, err := commercialsvc.NewBenefitsService(db, accounts, plans, platform)
		if err != nil {
			t.Fatal(err)
		}
		h.SetBenefitsService(benefits)
		if f, ok := platform.(*commercialplatform.FakeAdapter); ok {
			fake = f
		}
	}
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	RegisterCommercialRoutes(v1, h)
	return engine, db, fake
}

// performBenefitsGet runs one authenticated GET /commercial/account through
// the given identity (the performAccount convention).
func performBenefitsGet(t *testing.T, engine *gin.Engine, tenantID uint64, userID, role string) (int, string) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/commercial/account", nil)
	served := gin.New()
	served.Use(authAs(tenantID, userID, role))
	served.Handle(http.MethodGet, "/api/v1/*rest", func(c *gin.Context) {
		engine.ServeHTTP(c.Writer, c.Request)
	})
	served.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// TestAccountBenefitsSection: an ensured space answers the additive
// benefits object — plan (base, v1, active), the seeded features/limits and
// one month's credits with the batch breakdown — in closed WeKnora
// vocabulary only; repeated access stays exactly one of everything.
func TestAccountBenefitsSection(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true, "advanced_models": false, "priority_support": false})
	engine, _, _ := newBenefitsEngine(t, fake)

	code, body := performBenefitsGet(t, engine, 301, "owner-user", "owner")
	if code != http.StatusOK {
		t.Fatalf("status = %d body=%s", code, body)
	}
	for _, want := range []string{
		`"benefits"`, `"plan"`, `"key":"base"`, `"version":1`, `"state":"active"`,
		`"api_access"`, `"members"`, `"storage_gb"`, `"concurrent_tasks"`,
		`"credits"`, `"balance_micro"`, `"batches"`, `"period"`, `"expires_at"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("benefits section must carry %s, got %s", want, body)
		}
	}
	// One month's included credits (1 credit = 1_000_000 micro), as a digit
	// string on the wire-amount convention.
	if !strings.Contains(body, fmt.Sprintf(`"%d"`, commercialsvc.BasePlanSeedIncludedCreditsMicro)) {
		t.Fatalf("the balance must answer one month's micro amount as a digit string: %s", body)
	}
	// Repeat access: exactly one subscription, exactly one wallet (the lazy
	// chain is idempotent at the API boundary).
	if _, _ = performBenefitsGet(t, engine, 301, "owner-user", "owner"); len(fake.Subscriptions()) != 1 || len(fake.Wallets()) != 1 {
		t.Fatalf("repeat access must not duplicate: subs=%d wallets=%d", len(fake.Subscriptions()), len(fake.Wallets()))
	}
}

// TestAccountBenefitsPendingAbsent: while the authority cannot answer, the
// account envelope stays the closed pending shape and the benefits object
// is ABSENT (no fabricated plan/credits) with the closed reason token.
func TestAccountBenefitsPendingAbsent(t *testing.T) {
	engine, _, _ := newBenefitsEngine(t, leakyBenefitsPlatform{})

	code, body := performBenefitsGet(t, engine, 302, "owner-user", "owner")
	if code != http.StatusOK {
		t.Fatalf("status = %d body=%s", code, body)
	}
	if strings.Contains(body, `"benefits"`) {
		t.Fatalf("a pending chain must NOT fabricate benefits: %s", body)
	}
	if !strings.Contains(body, `"pending"`) || !strings.Contains(body, `"unreachable"`) {
		t.Fatalf("the closed pending/reason tokens must answer: %s", body)
	}
}

// leakyBenefitsPlatform simulates the worst case: errors carrying provider
// markers, URLs and foreign identities.
type leakyBenefitsPlatform struct{}

func (leakyBenefitsPlatform) SubmitCommand(context.Context, commercial.Command) (commercial.CommandReceipt, error) {
	return commercial.CommandReceipt{}, fmt.Errorf("%w: lago POST http://lago-marker-90210.invalid/api/v1/subscriptions rejected: wallet lago-777",
		commercial.ErrPlatformUnreachable)
}

func (leakyBenefitsPlatform) ReadSnapshot(context.Context, commercial.SnapshotQuery) (commercial.Snapshot, error) {
	return commercial.Snapshot{}, fmt.Errorf("%w: GET http://lago-marker-90210.invalid/api/v1/customers/weknora-tenant-90210 failed",
		commercial.ErrPlatformUnreachable)
}

func (leakyBenefitsPlatform) Reconcile(context.Context, commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return commercial.ReconciliationPage{}, commercial.ErrPlatformUnsupported
}

// TestAccountBenefitsProviderNeutrality: no external identity, no provider
// marker, no URL, no raw error text ever crosses the benefits path — on the
// success answer AND the pending answer.
func TestAccountBenefitsProviderNeutrality(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	engine, _, _ := newBenefitsEngine(t, fake)
	_, body := performBenefitsGet(t, engine, 303, "owner-user", "owner")
	for _, leak := range []string{
		"weknora-base-v1",        // external plan code
		"weknora-tenant-303-sub", // external subscription identity
		"lago",                   // provider vocabulary
		"http://", "https://",    // URLs
		"invalid_request_error", // raw provider error text
	} {
		if strings.Contains(body, leak) {
			t.Fatalf("benefits answer leaks %q: %s", leak, body)
		}
	}

	leakyEngine, _, _ := newBenefitsEngine(t, leakyBenefitsPlatform{})
	_, body = performBenefitsGet(t, leakyEngine, 304, "owner-user", "owner")
	for _, leak := range []string{"lago-marker", "90210", "http://", "weknora-tenant-902"} {
		if strings.Contains(body, leak) {
			t.Fatalf("pending answer leaks %q: %s", leak, body)
		}
	}
}

// TestCrossTenantBenefitsIsolation: two spaces ensured on one engine — each
// answer reflects only its own space; neither body carries the other's
// identities or balances; the authority holds per-tenant objects only.
func TestCrossTenantBenefitsIsolation(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	engine, _, _ := newBenefitsEngine(t, fake)

	codeA, bodyA := performBenefitsGet(t, engine, 305, "user-a", "owner")
	codeB, bodyB := performBenefitsGet(t, engine, 306, "user-b", "owner")
	if codeA != http.StatusOK || codeB != http.StatusOK {
		t.Fatalf("statuses = %d/%d bodies %s %s", codeA, codeB, bodyA, bodyB)
	}
	if strings.Contains(bodyA, "weknora-tenant-306") || strings.Contains(bodyB, "weknora-tenant-305") {
		t.Fatalf("cross-tenant identity leak: %s | %s", bodyA, bodyB)
	}
	for _, body := range []string{bodyA, bodyB} {
		if strings.Count(body, `"period"`) != 1 {
			t.Fatalf("each space answers exactly its own batch: %s", body)
		}
	}
	if subs := fake.Subscriptions(); len(subs) != 2 {
		t.Fatalf("two spaces hold two subscriptions, got %d", len(subs))
	}
	if wallets := fake.Wallets(); len(wallets) != 2 {
		t.Fatalf("two spaces hold two wallets, got %d", len(wallets))
	}
}
