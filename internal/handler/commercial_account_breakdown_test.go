package handler

// GET /commercial/account 的余额分解契约回归（issue #86 Task 4）：
// credits 增广 held_micro/refund_locked_micro/available_micro/projected_at
// 与批次 source/granted_at；available = balance − held − refund_locked；
// closed-token 纪律（无 provider 词汇）保持。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/commercial/commercialplatform"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/types"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newBreakdownEnv builds the sqlite + fake-adapter + benefits-service
// handler environment with the budget tables present (the holds read face).
func newBreakdownEnv(t *testing.T, tenantID uint64) (*CommercialHandler, *gorm.DB, *commercialplatform.FakeAdapter) {
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
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(
		&repocommercial.BillingAccount{},
		&repocommercial.BudgetAccountRow{},
		&repocommercial.BudgetLotRow{},
	); err != nil {
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
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS commercial_resource_counters (
		tenant_id INTEGER NOT NULL, resource TEXT NOT NULL, used INTEGER NOT NULL DEFAULT 0,
		hard_limit INTEGER NULL, PRIMARY KEY (tenant_id, resource)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	accounts, err := commercialsvc.NewBillingAccountService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := commercialsvc.NewPlanVersionService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	benefits, err := commercialsvc.NewBenefitsService(db, accounts, plans, fake, repocommercial.NewBudgetStore(db))
	if err != nil {
		t.Fatal(err)
	}
	h := NewCommercialHandler(db)
	h.SetBenefitsService(benefits)
	return h, db, fake
}

// performAccountGet drives AccountStatus with the authenticated tenant/user
// scope the Auth middleware would install.
func performAccountGet(t *testing.T, h *CommercialHandler, tenantID uint64) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, "user-"+strconv.FormatUint(tenantID, 10))
	c.Request = httptest.NewRequest(http.MethodGet, "/commercial/account", nil).WithContext(ctx)
	h.AccountStatus(c)
	return w
}

// TestAccountCreditsBreakdownArithmetic (#86 Task 4, review-focus 4): the
// breakdown must answer the closed arithmetic — held and refund-locked from
// the budget projection, available = balance − held − refund_locked, the
// projection instant, and the batch source face (monthly).
func TestAccountCreditsBreakdownArithmetic(t *testing.T) {
	const tenant = uint64(601)
	h, db, _ := newBreakdownEnv(t, tenant)
	// One ensured chain first: the monthly batch (1_000_000 micro) lands.
	w := performAccountGet(t, h, tenant)
	if w.Code != http.StatusOK {
		t.Fatalf("first ensure: %d %s", w.Code, w.Body.String())
	}
	// A budget projection row carrying an in-flight hold (parameter-bound).
	if err := db.Exec(`INSERT INTO commercial_budget_accounts
		(tenant_id, verified_micro, unreflected_micro, held_micro, refund_locked_micro, watermark, version, verified_until)
		VALUES (?, 0, 0, 200000, 0, 'w1', 1, ?)`, tenant, time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	w = performAccountGet(t, h, tenant)
	if w.Code != http.StatusOK {
		t.Fatalf("breakdown read: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data struct {
			Benefits struct {
				Credits struct {
					BalanceMicro      string `json:"balance_micro"`
					HeldMicro         string `json:"held_micro"`
					RefundLockedMicro string `json:"refund_locked_micro"`
					AvailableMicro    string `json:"available_micro"`
					ProjectedAt       string `json:"projected_at"`
					Batches           []struct {
						Source       string `json:"source"`
						Period       string `json:"period"`
						GrantedAt    string `json:"granted_at"`
						BalanceMicro string `json:"balance_micro"`
						ExpiresAt    string `json:"expires_at"`
					} `json:"batches"`
				} `json:"credits"`
			} `json:"benefits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	credits := out.Data.Benefits.Credits
	if credits.BalanceMicro != "1000000" {
		t.Fatalf("balance_micro = %q, want 1000000", credits.BalanceMicro)
	}
	if credits.HeldMicro != "200000" {
		t.Fatalf("held_micro = %q, want 200000", credits.HeldMicro)
	}
	if credits.RefundLockedMicro != "0" {
		t.Fatalf("refund_locked_micro = %q, want 0", credits.RefundLockedMicro)
	}
	if credits.AvailableMicro != "800000" {
		t.Fatalf("available_micro = %q, want 800000 (balance − held − locked)", credits.AvailableMicro)
	}
	if credits.ProjectedAt == "" {
		t.Fatal("projected_at must answer the projection instant")
	}
	if _, err := time.Parse(time.RFC3339, credits.ProjectedAt); err != nil {
		t.Fatalf("projected_at must be RFC3339, got %q", credits.ProjectedAt)
	}
	if len(credits.Batches) == 0 {
		t.Fatalf("batches must answer, got %+v", credits)
	}
	if credits.Batches[0].Source != "monthly" {
		t.Fatalf("batch source = %q, want monthly", credits.Batches[0].Source)
	}
	if credits.Batches[0].GrantedAt == "" {
		t.Fatal("batch granted_at must answer")
	}
}

// TestBenefitsWireNoProviderVocabulary (#86 Task 4, review-focus 5): the
// breakdown answer carries no provider vocabulary — no lago tokens, no
// URLs, no wallet-name substrings.
func TestBenefitsWireNoProviderVocabulary(t *testing.T) {
	const tenant = uint64(602)
	h, _, _ := newBreakdownEnv(t, tenant)
	w := performAccountGet(t, h, tenant)
	if w.Code != http.StatusOK {
		t.Fatalf("ensure: %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, leak := range []string{"lago", "http://", "https://", "weknora-base-v1"} {
		if contains := len(body) >= len(leak) && stringContains(body, leak); contains {
			t.Fatalf("breakdown answer leaks %q: %s", leak, body)
		}
	}
}

// TestBenefitsWireCrossMonthBatchesAllCarryGrantedAt (OCR r1, CR-86-1):
// the wire face of the cross-month steady state — a lingering LAST-month
// registry batch (its terminated wallet left the snapshot, GrantedAt from
// the registry row) beside the current month's batch. Every monthly batch
// line must carry a granted_at the frontend contract accepts (present and
// RFC3339); a zero instant (the defensive omission branch) must leave the
// rest of the breakdown parseable.
func TestBenefitsWireCrossMonthBatchesAllCarryGrantedAt(t *testing.T) {
	registryGrant := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	status := commercialsvc.BenefitsStatus{
		Plan: &commercialsvc.PlanView{Key: "base", Version: 1, State: "active"},
		Credits: &commercialsvc.CreditsView{
			BalanceMicro: 1_000_000,
			ProjectedAt:  time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
			Batches: []commercialsvc.BatchView{
				// The lingering September batch: zero balance (expired), the
				// grant instant from the REGISTRY row (snapshot absent).
				{Period: "2026-09", BalanceMicro: 0,
					ExpiresAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
					Source:    "monthly", GrantedAt: registryGrant},
				// The current October batch.
				{Period: "2026-10", BalanceMicro: 1_000_000,
					ExpiresAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
					Source:    "monthly", GrantedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
			},
		},
	}
	wire := benefitsWire(status)
	creditsFace, ok := wire["credits"].(gin.H)
	if !ok {
		t.Fatalf("credits wire face = %+v", wire["credits"])
	}
	batches, ok := creditsFace["batches"].([]gin.H)
	if !ok || len(batches) != 2 {
		t.Fatalf("batches wire face = %+v", creditsFace["batches"])
	}
	for _, b := range batches {
		grantedAt, present := b["granted_at"]
		if !present || grantedAt == "" {
			t.Fatalf("CR-86-1: batch %v must carry granted_at (a missing one makes the frontend contract reject the whole breakdown)", b)
		}
		if _, err := time.Parse(time.RFC3339, grantedAt.(string)); err != nil {
			t.Fatalf("granted_at must be RFC3339, got %v", grantedAt)
		}
	}

	// The defensive branch: a zero instant omits the key — the batch line
	// must stay JSON-serializable and the credits object complete (the
	// frontend parser degrades the display field to '').
	status.Credits.Batches = append(status.Credits.Batches, commercialsvc.BatchView{
		Period: "2026-08", BalanceMicro: 0,
		ExpiresAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Source:    "monthly", GrantedAt: time.Time{},
	})
	wire = benefitsWire(status)
	if _, err := json.Marshal(wire); err != nil {
		t.Fatalf("the wire face must stay serializable with a zero-instant batch: %v", err)
	}
	creditsFace, _ = wire["credits"].(gin.H)
	batches, _ = creditsFace["batches"].([]gin.H)
	if len(batches) != 3 {
		t.Fatalf("batches = %d, want 3", len(batches))
	}
	if _, present := batches[2]["granted_at"]; present {
		t.Fatal("a zero instant must omit granted_at (the omission branch)")
	}
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
