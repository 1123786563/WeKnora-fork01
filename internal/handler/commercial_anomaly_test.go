package handler

// #84 Task 3: the platform payment-anomaly disposition surface —
// GET /api/v1/admin/payment-anomalies and POST .../:id/resolve,
// gated by the platform refund-reviewer authority (the exact refund-review
// precedent: space administration NEVER admits; spec L170 closed vocabulary
// only). Task 5 appends the orderWire attention projection cases here.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/types"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newAnomalyAdminEnv builds a sqlite db with the anomaly table, the grants
// table and an engine carrying the admin routes behind the production
// platform guard (mirroring the RegisterCommercialRoutes mount). The
// optional auth middleware installs BEFORE the routes (gin only chains
// middleware registered ahead of the route).
func newAnomalyAdminEnv(t *testing.T, auth ...gin.HandlerFunc) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PaymentAnomalyRow{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS commercial_grants (
		tenant_id INTEGER NOT NULL,
		user_id TEXT NOT NULL,
		capability TEXT NOT NULL,
		granted_by TEXT NOT NULL,
		version INTEGER NOT NULL DEFAULT 1,
		PRIMARY KEY (tenant_id, user_id, capability))`).Error; err != nil {
		t.Fatal(err)
	}
	h := NewCommercialHandler(db)
	engine := gin.New()
	engine.Use(auth...)
	v1 := engine.Group("/api/v1")
	// The production mount (routes_commercial.go) hangs the group DIRECTLY
	// on the /api/v1 parent — the refund-review precedent: cross-space
	// money handling never sits under the tenant commercial group.
	admin := v1.Group("/admin/payment-anomalies", h.RequirePlatformRefundReviewer())
	admin.GET("", h.AdminListPaymentAnomalies)
	admin.POST("/:id/resolve", h.AdminResolvePaymentAnomaly)
	fulfillmentAdmin := v1.Group("/admin/fulfillment-attentions", h.RequirePlatformRefundReviewer())
	fulfillmentAdmin.GET("", h.AdminListFulfillmentAttentions)
	return engine, db
}

func TestAdminFulfillmentAttentionsSanitizedAndPlatformGuarded(t *testing.T) {
	engine, db := newAnomalyAdminEnv(t, authAsAnomalyAdmin(0, "platform-ops", "viewer"))
	if err := db.AutoMigrate(&commercialsvc.FulfillmentExceptionRow{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO commercial_grants (tenant_id, user_id, capability, granted_by) VALUES (0, 'platform-ops', 'refund_review', 'seed')`).Error; err != nil {
		t.Fatal(err)
	}
	row := commercialsvc.FulfillmentExceptionRow{EventKey: "fulfill:internal-secret", ID: "opaque-1", TenantID: 12, OrderID: "order-1", Kind: "top_up", Reason: "invalid_winning_payment", State: "open", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/fulfillment-attentions", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, field := range []string{`"id":"opaque-1"`, `"order_id":"order-1"`, `"tenant_id":12`, `"kind":"top_up"`, `"reason":"invalid_winning_payment"`, `"state":"open"`, `"created_at"`, `"updated_at"`} {
		if !strings.Contains(body, field) {
			t.Fatalf("missing %s in %s", field, body)
		}
	}
	for _, secret := range []string{"internal-secret", "event_key", "provider", "merchant", "transaction", "payload", "credentials"} {
		if strings.Contains(body, secret) {
			t.Fatalf("response leaked %q: %s", secret, body)
		}
	}
	unauth, _ := newAnomalyAdminEnv(t)
	w = httptest.NewRecorder()
	unauth.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/fulfillment-attentions", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated status=%d", w.Code)
	}
	spaceAdmin, _ := newAnomalyAdminEnv(t, authAsAnomalyAdmin(7, "tenant-admin", "admin"))
	w = httptest.NewRecorder()
	spaceAdmin.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/fulfillment-attentions", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("space admin status=%d", w.Code)
	}
}

// authAsAnomalyAdmin installs the authenticated-session context the Auth
// middleware would attach.
func authAsAnomalyAdmin(tenantID uint64, userID, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(role))
		ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func seedAnomalyRow(t *testing.T, db *gorm.DB, id, orderID, txn, state string) {
	t.Helper()
	now := time.Now().UTC()
	row := repocommercial.PaymentAnomalyRow{
		ID: id, TenantID: 7, OrderID: orderID, AttemptID: "mo_" + id,
		Provider: "wechat", Merchant: "1900000109", Transaction: txn,
		Kind: repocommercial.PaymentAnomalyKindPartial, ExpectedAmountFen: 9900, ActualAmountFen: 5000,
		ExpectedCurrency: "CNY", ActualCurrency: "CNY", State: state, Version: 1, CreatedAt: now,
	}
	if state == repocommercial.PaymentAnomalyStateResolved {
		resolved := now
		row.ResolvedAt = &resolved
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
}

func TestAdminPaymentAnomaliesListAndResolve(t *testing.T) {
	// The platform operator's authenticated session (the grant below carries
	// the refund_review capability at platform scope).
	engine, db := newAnomalyAdminEnv(t, authAsAnomalyAdmin(0, "platform-ops", "viewer"))
	if err := db.Exec(`INSERT INTO commercial_grants (tenant_id, user_id, capability, granted_by)
		VALUES (0, 'platform-ops', 'refund_review', 'seed')`).Error; err != nil {
		t.Fatal(err)
	}
	seedAnomalyRow(t, db, "anom_admin_1", "ord_a1", "txn_a1", repocommercial.PaymentAnomalyStateAwaiting)
	seedAnomalyRow(t, db, "anom_admin_2", "ord_a2", "txn_a2", repocommercial.PaymentAnomalyStateResolved)

	// List: closed fields only, both rows present.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/payment-anomalies", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, field := range []string{`"id"`, `"order_id"`, `"tenant_id"`, `"provider"`, `"kind"`,
		`"expected_amount_fen"`, `"actual_amount_fen"`, `"currency"`, `"state"`, `"version"`, `"created_at"`} {
		if !strings.Contains(body, field) {
			t.Fatalf("list must carry the closed field %s, got %s", field, body)
		}
	}
	if !strings.Contains(body, "anom_admin_1") || !strings.Contains(body, "anom_admin_2") {
		t.Fatalf("both anomalies must be listed, got %s", body)
	}

	// Resolve with the correct expected_version: 200, state flips resolved.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost,
		"/api/v1/admin/payment-anomalies/anom_admin_1/resolve", strings.NewReader(`{"expected_version":1}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"state":"resolved"`) {
		t.Fatalf("resolve status = %d body=%s", w.Code, w.Body.String())
	}

	// Resolving again with the SAME (now stale) version: 409.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost,
		"/api/v1/admin/payment-anomalies/anom_admin_1/resolve", strings.NewReader(`{"expected_version":1}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "anomaly changed since read") {
		t.Fatalf("stale resolve must answer 409 with the closed message, got %d %s", w.Code, w.Body.String())
	}

	// Unknown id: 404.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost,
		"/api/v1/admin/payment-anomalies/anom_ghost/resolve", strings.NewReader(`{"expected_version":1}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown anomaly must answer 404, got %d %s", w.Code, w.Body.String())
	}
}

// ---- #84 Task 5: orderWire attention projection (R4 dispatch table) ----

// TestOrderWireProjectsAttentionWhileUnresolved（AC4）：pending 订单带未处置
// 异常 → wire fulfillment=attention 且必须与 payment=pending 成对出现（契约
// 不变量：attention 只与 pending 同时出现，前端文案互斥由此获得保证）。
func TestOrderWireProjectsAttentionWhileUnresolved(t *testing.T) {
	w := orderWire(commercialsvc.OrderView{ID: "o1", State: "pending", AmountFen: 9900, Currency: "CNY", PaymentAttention: true})
	if w["fulfillment"] != "attention" {
		t.Fatalf("got %v", w["fulfillment"])
	}
	if w["payment"] != "pending" {
		t.Fatalf("attention must pair pending, got %v", w["payment"])
	}
	if w["payment_attention"] != true {
		t.Fatal("the attention flag must ride along")
	}
}

// TestOrderWireAttentionNeverOverridesPaidOrFulfilled（审查 R4 裁决锁定）：
// 多收款典型形态——订单已 fulfilled/paid 且仍有未处置 anomaly，用户主状态
// 必须如实（fulfilled/processing），异常面走 admin + payment_attention 附加
// 字段；attention 绝不改写履约事实。
func TestOrderWireAttentionNeverOverridesPaidOrFulfilled(t *testing.T) {
	w := orderWire(commercialsvc.OrderView{ID: "o2", State: "fulfilled", AmountFen: 9900, Currency: "CNY", PaymentAttention: true})
	if w["fulfillment"] != "fulfilled" {
		t.Fatalf("got %v", w["fulfillment"])
	}
	if w["payment_attention"] != true {
		t.Fatal("attention flag must still ride along")
	}
	w2 := orderWire(commercialsvc.OrderView{ID: "o3", State: "paid", AmountFen: 9900, Currency: "CNY", PaymentAttention: true})
	if w2["fulfillment"] != "processing" {
		t.Fatalf("got %v", w2["fulfillment"])
	}
	// A pending order WITHOUT attention keeps the plain pending projection.
	w3 := orderWire(commercialsvc.OrderView{ID: "o4", State: "pending", AmountFen: 9900, Currency: "CNY"})
	if w3["fulfillment"] != "pending" {
		t.Fatalf("plain pending must stay pending, got %v", w3["fulfillment"])
	}
}

// TestAdminPaymentAnomaliesRequiresPlatformReviewer：平台守卫——无
// refund_review 凭据的普通用户与 space Admin 均 403（处置面是跨空间运营
// 面，租户权威绝不放行——refund-review 同型纪律）。
func TestAdminPaymentAnomaliesRequiresPlatformReviewer(t *testing.T) {
	engine, _ := newAnomalyAdminEnv(t)
	// No authenticated session at all: 403.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/payment-anomalies", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated GET must be 403, got %d", w.Code)
	}
	// An authenticated space ADMIN (session installed) is still 403: tenant
	// authority never admits the cross-space disposition surface.
	engine2, _ := newAnomalyAdminEnv(t, authAsAnomalyAdmin(7, "user-admin", "admin"))
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/payment-anomalies", nil)
	engine2.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("a space admin must never reach the anomaly surface, got %d %s", w.Code, w.Body.String())
	}
}
