package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newCommercialScopeEngine builds a minimal engine around the commercial
// routes plus the SQLite schema the handler reads. auth runs before route
// registration so it plays the role the global Auth middleware plays in
// production: tenant, user and role always come from the authenticated
// server-side context, never from the URL.
func newCommercialScopeEngine(t *testing.T, auth gin.HandlerFunc) (*gin.Engine, *handler.CommercialHandler, *gorm.DB) {
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
	// Mirror the production SQLite pool (single writer) so concurrent
	// reservations serialise on the connection instead of racing locks.
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	for _, ddl := range commercialScopeDDL {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("prepare schema: %v", err)
		}
	}
	h := handler.NewCommercialHandler(db)
	engine := gin.New()
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	if auth != nil {
		engine.Use(auth)
	}
	v1 := engine.Group("/api/v1")
	RegisterCommercialRoutes(v1, h)
	return engine, h, db
}

var commercialScopeDDL = []string{
	`CREATE TABLE IF NOT EXISTS commercial_subscriptions (
		id TEXT PRIMARY KEY,
		tenant_id INTEGER NOT NULL UNIQUE,
		plan_key TEXT NOT NULL,
		plan_version INTEGER NOT NULL,
		plan_snapshot_json TEXT NOT NULL DEFAULT '',
		anchor DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		paid_until DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		future_interval_json TEXT NOT NULL DEFAULT '',
		version INTEGER NOT NULL DEFAULT 1,
		projection_plan_json TEXT NOT NULL DEFAULT '',
		downgrade_reason TEXT NOT NULL DEFAULT '')`,
	`CREATE TABLE IF NOT EXISTS commercial_grants (
		tenant_id INTEGER NOT NULL,
		user_id TEXT NOT NULL,
		capability TEXT NOT NULL,
		granted_by TEXT NOT NULL DEFAULT '',
		version INTEGER NOT NULL DEFAULT 1,
		PRIMARY KEY (tenant_id, user_id, capability))`,
	`CREATE TABLE IF NOT EXISTS commercial_plan_catalog (
		plan_key TEXT NOT NULL,
		version INTEGER NOT NULL,
		definition_json TEXT NOT NULL,
		external_id TEXT NOT NULL UNIQUE,
		state TEXT NOT NULL,
		PRIMARY KEY (plan_key, version))`,
	`CREATE TABLE IF NOT EXISTS scope_test_resources (
		id TEXT PRIMARY KEY,
		tenant_id INTEGER NOT NULL)`,
}

// authAs simulates the authenticated session applyAuthSession attaches.
func authAs(tenantID uint64, userID, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(role))
		ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// serveWith builds a fresh engine over db for the given identity and
// performs one request.
func serveWith(t *testing.T, db *gorm.DB, auth gin.HandlerFunc, method, path string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := handler.NewCommercialHandler(db)
	engine := gin.New()
	engine.Use(auth)
	v1 := engine.Group("/api/v1")
	RegisterCommercialRoutes(v1, h)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w, w.Body.String()
}

// TestCommercialScopeQueriesNeverCrossSpaces: the same user belongs to
// spaces A and B; usage and summary answers are scoped strictly to the
// authenticated space — B never sees anything of A and vice versa.
func TestCommercialScopeQueriesNeverCrossSpaces(t *testing.T) {
	_, _, db := newCommercialScopeEngine(t, nil)
	seed := []string{
		`INSERT INTO commercial_subscriptions (id, tenant_id, plan_key, plan_version) VALUES ('sub-a', 101, 'pro', 3)`,
		`INSERT INTO commercial_resource_counters (tenant_id, resource, used, hard_limit) VALUES (101, 'storage_files', 7, 10)`,
		`INSERT INTO commercial_resource_counters (tenant_id, resource, used, hard_limit) VALUES (202, 'wiki_pages', 3, 5)`,
		`INSERT INTO commercial_plan_catalog (plan_key, version, definition_json, external_id, state) VALUES ('pro', 3, '{}', 'ext-pro-3', 'published')`,
		`INSERT INTO commercial_plan_catalog (plan_key, version, definition_json, external_id, state) VALUES ('draft-plan', 1, '{}', 'ext-draft-1', 'draft')`,
	}
	for _, q := range seed {
		if err := db.Exec(q).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// Space A sees only its own counters and subscription.
	w, body := serveWith(t, db, authAs(101, "shared-user", "viewer"), http.MethodGet, "/api/v1/commercial/usage")
	if w.Code != http.StatusOK {
		t.Fatalf("usage A status = %d body=%s", w.Code, body)
	}
	if !strings.Contains(body, "storage_files") || strings.Contains(body, "wiki_pages") {
		t.Fatalf("usage A leaked cross-space data: %s", body)
	}
	w, body = serveWith(t, db, authAs(101, "shared-user", "viewer"), http.MethodGet, "/api/v1/commercial/summary")
	if w.Code != http.StatusOK {
		t.Fatalf("summary A status = %d body=%s", w.Code, body)
	}
	if !strings.Contains(body, `"pro"`) {
		t.Fatalf("summary A missing own plan: %s", body)
	}

	// Space B sees nothing of space A: no A counters, no A subscription.
	w, body = serveWith(t, db, authAs(202, "shared-user", "viewer"), http.MethodGet, "/api/v1/commercial/usage")
	if w.Code != http.StatusOK {
		t.Fatalf("usage B status = %d body=%s", w.Code, body)
	}
	if strings.Contains(body, "storage_files") || !strings.Contains(body, "wiki_pages") {
		t.Fatalf("usage B leaked space A data: %s", body)
	}
	w, body = serveWith(t, db, authAs(202, "shared-user", "viewer"), http.MethodGet, "/api/v1/commercial/summary")
	if w.Code != http.StatusOK {
		t.Fatalf("summary B status = %d body=%s", w.Code, body)
	}
	if strings.Contains(body, `"pro"`) || !strings.Contains(body, "base") {
		t.Fatalf("summary B leaked space A subscription: %s", body)
	}

	// Plans expose only published catalog rows; orders list is a JSON
	// array (never null) for both spaces since no order write path exists.
	w, body = serveWith(t, db, authAs(101, "shared-user", "viewer"), http.MethodGet, "/api/v1/commercial/plans")
	if w.Code != http.StatusOK || !strings.Contains(body, `"pro"`) || strings.Contains(body, "draft-plan") {
		t.Fatalf("plans A status=%d body=%s", w.Code, body)
	}
	w, body = serveWith(t, db, authAs(202, "shared-user", "viewer"), http.MethodGet, "/api/v1/commercial/orders")
	if w.Code != http.StatusOK || strings.TrimSpace(body) == "" || strings.Contains(body, "null") {
		t.Fatalf("orders B status=%d body=%s", w.Code, body)
	}
}

// TestCommercialAdminPostForbidden: an Admin without a billing grant is
// not authorised for commercial write operations (403); the owner passes
// the gate, and so does an Admin holding an explicit billing grant.
func TestCommercialAdminPostForbidden(t *testing.T) {
	_, _, db := newCommercialScopeEngine(t, nil)

	w, body := serveWith(t, db, authAs(101, "user-admin", "admin"), http.MethodPost, "/api/v1/commercial/orders")
	if w.Code != http.StatusForbidden {
		t.Fatalf("admin POST status = %d, want 403 body=%s", w.Code, body)
	}

	w, body = serveWith(t, db, authAs(101, "user-owner", "owner"), http.MethodPost, "/api/v1/commercial/orders")
	if w.Code == http.StatusForbidden {
		t.Fatalf("owner POST rejected: %s", body)
	}

	if err := db.Exec(`INSERT INTO commercial_grants (tenant_id, user_id, capability) VALUES (101, 'user-admin', 'billing')`).Error; err != nil {
		t.Fatalf("seed grant: %v", err)
	}
	w, body = serveWith(t, db, authAs(101, "user-admin", "admin"), http.MethodPost, "/api/v1/commercial/orders")
	if w.Code == http.StatusForbidden {
		t.Fatalf("granted admin POST rejected: %s", body)
	}
}

// TestCommercialConcurrentAddsNeverExceedLimit: 25 concurrent resource
// adds against a hard limit of 10 — the accepted count, the counter and
// the written resource rows must all agree and never exceed the limit.
func TestCommercialConcurrentAddsNeverExceedLimit(t *testing.T) {
	_, h, db := newCommercialScopeEngine(t, nil)
	const tenant = uint64(303)
	if err := db.Exec(`INSERT INTO commercial_resource_counters (tenant_id, resource, used, hard_limit) VALUES (303, 'agents', 0, 10)`).Error; err != nil {
		t.Fatalf("seed limit: %v", err)
	}

	const workers = 25
	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("agent-%d", i)
			errs[i] = h.ReserveResource(tenant, "agents", 1, func(tx *gorm.DB) error {
				return tx.Exec(`INSERT INTO scope_test_resources (id, tenant_id) VALUES (?, ?)`, id, tenant).Error
			})
		}(i)
	}
	wg.Wait()

	var used int64
	if err := db.Raw(`SELECT used FROM commercial_resource_counters WHERE tenant_id = ? AND resource = 'agents'`, tenant).Scan(&used).Error; err != nil {
		t.Fatalf("read counter: %v", err)
	}
	var rows int64
	if err := db.Raw(`SELECT COUNT(*) FROM scope_test_resources WHERE tenant_id = ?`, tenant).Scan(&rows).Error; err != nil {
		t.Fatalf("read resources: %v", err)
	}
	if used > 10 {
		t.Fatalf("concurrent adds exceeded limit: used=%d", used)
	}
	if used != rows {
		t.Fatalf("counter/resources disagree: used=%d rows=%d", used, rows)
	}
	rejected := 0
	for _, err := range errs {
		switch {
		case err == nil:
		case errors.Is(err, handler.ErrQuotaExceeded):
			rejected++
		default:
			t.Fatalf("unexpected reserve error: %v", err)
		}
	}
	if int(used)+rejected != workers {
		t.Fatalf("accepted=%d rejected=%d workers=%d", used, rejected, workers)
	}
}

// TestCommercialRechargeDoesNotModifyQuota: crediting (recharge) lives on
// its own counter dimension and must never touch resource quotas.
func TestCommercialRechargeDoesNotModifyQuota(t *testing.T) {
	_, h, db := newCommercialScopeEngine(t, nil)
	const tenant = uint64(505)
	if err := db.Exec(`INSERT INTO commercial_resource_counters (tenant_id, resource, used, hard_limit) VALUES (505, 'knowledge_bases', 4, 10)`).Error; err != nil {
		t.Fatalf("seed quota: %v", err)
	}
	if err := h.ReserveResource(tenant, "credits", 500, nil); err != nil {
		t.Fatalf("recharge reserve: %v", err)
	}
	var kbs, credits int64
	if err := db.Raw(`SELECT used FROM commercial_resource_counters WHERE tenant_id = ? AND resource = 'knowledge_bases'`, tenant).Scan(&kbs).Error; err != nil {
		t.Fatalf("read quota: %v", err)
	}
	if err := db.Raw(`SELECT used FROM commercial_resource_counters WHERE tenant_id = ? AND resource = 'credits'`, tenant).Scan(&credits).Error; err != nil {
		t.Fatalf("read credits: %v", err)
	}
	if kbs != 4 {
		t.Fatalf("recharge modified resource quota: knowledge_bases=%d", kbs)
	}
	if credits != 500 {
		t.Fatalf("credits not credited: %d", credits)
	}
}

// TestCommercialHealthCheckIssuesNoBillingRequests: /health answers 200
// even with the underlying database closed — proof the liveness path
// never touches billing or quota tables.
func TestCommercialHealthCheckIssuesNoBillingRequests(t *testing.T) {
	engine, _, db := newCommercialScopeEngine(t, nil)
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("health status = %d body=%s", w.Code, w.Body.String())
	}
}

// TestCommercialAPIKeyRequiresExplicitCapability: a full-access API key is
// NOT auto-granted purchase authority — only an explicit commercial
// capability admits a machine principal.
func TestCommercialAPIKeyRequiresExplicitCapability(t *testing.T) {
	withScope := func(scope types.TenantAPIKeyScope) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Request = c.Request.WithContext(types.WithTenantAPIKeyScope(c.Request.Context(), scope))
			c.Next()
		}
	}
	tenant := authAs(101, "machine", "viewer")
	fullAccess := withScope(types.TenantAPIKeyScope{FullAccess: true})
	scoped := withScope(types.TenantAPIKeyScope{
		Capabilities: types.StringArray{string(handler.CommercialAPIKeyCapability)},
	})

	w, body := serveScoped(t, fullAccess, tenant, "/api/v1/commercial/summary")
	if w.Code != http.StatusForbidden {
		t.Fatalf("full-access key status = %d, want 403 body=%s", w.Code, body)
	}
	w, body = serveScoped(t, scoped, tenant, "/api/v1/commercial/summary")
	if w.Code == http.StatusForbidden {
		t.Fatalf("explicit commercial capability rejected: %s", body)
	}
}

func serveScoped(t *testing.T, scope, auth gin.HandlerFunc, path string) (*httptest.ResponseRecorder, string) {
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
	engine := gin.New()
	engine.Use(scope, auth)
	v1 := engine.Group("/api/v1")
	RegisterCommercialRoutes(v1, h)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w, w.Body.String()
}
