package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openActionPlanHandlerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repoappconn.ActionPlanRow{}, &repoappconn.ActionPlanItemRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func actionPlanTestEngine(t *testing.T, h *AppActionPlanHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "user-a")
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	v1 := engine.Group("/api/v1")
	// Mirror of RegisterAppActionPlanRoutes (internal/router) — the
	// router package is not importable from this handler-package test.
	v1.POST("/apps/action-plans", h.FormActionPlan)
	v1.POST("/apps/action-plans/:id/approve", h.ApproveActionPlan)
	v1.POST("/apps/action-plans/:id/execute", h.ExecuteActionPlan)
	v1.GET("/apps/action-plans/:id", h.GetActionPlan)
	return engine
}

func doActionPlan(t *testing.T, engine *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// TestActionPlanHandlerFailClosedWithoutService: until the container
// wires the plan service every endpoint refuses with 501 — the edge
// never fabricates plans, approvals or dispatches.
func TestActionPlanHandlerFailClosedWithoutService(t *testing.T) {
	db := openActionPlanHandlerDB(t)
	h := NewAppActionPlanHandler(db)
	engine := actionPlanTestEngine(t, h)
	w := doActionPlan(t, engine, http.MethodPost, "/api/v1/apps/action-plans",
		`{"items":[{"connection_id":"c","session_id":"s","artifact_version_id":"v","title":"T","parent_page_id":"p"}]}`)
	if w.Code != http.StatusNotImplemented || !strings.Contains(w.Body.String(), "ACTION_PLAN_PIPELINE_NOT_CONFIGURED") {
		t.Fatalf("form must fail closed: %d %s", w.Code, w.Body.String())
	}
	if err := db.Create(&repoappconn.ActionPlanRow{ID: "plan-1", TenantID: 7, ActorID: "user-a",
		Digest: "d", State: repoappconn.PlanStateAwaitingApproval}).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/apps/action-plans/plan-1/approve", `{"digest":"d"}`},
		{http.MethodPost, "/api/v1/apps/action-plans/plan-1/execute", `{"digest":"d"}`},
		{http.MethodGet, "/api/v1/apps/action-plans/plan-1", ""},
	} {
		w := doActionPlan(t, engine, tc.method, tc.path, tc.body)
		if w.Code != http.StatusNotImplemented || !strings.Contains(w.Body.String(), "ACTION_PLAN_PIPELINE_NOT_CONFIGURED") {
			t.Fatalf("%s %s must fail closed: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

// TestActionPlanHandlerValidationAndNotFound: malformed input is a 400
// before the service matters; missing plans are 404 with the plan code.
func TestActionPlanHandlerValidationAndNotFound(t *testing.T) {
	db := openActionPlanHandlerDB(t)
	h := NewAppActionPlanHandler(db)
	engine := actionPlanTestEngine(t, h)
	w := doActionPlan(t, engine, http.MethodPost, "/api/v1/apps/action-plans", `{"items":[]}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "INVALID_REQUEST") {
		t.Fatalf("zero items must be 400: %d %s", w.Code, w.Body.String())
	}
	w = doActionPlan(t, engine, http.MethodPost, "/api/v1/apps/action-plans", `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed body must be 400: %d %s", w.Code, w.Body.String())
	}
	w = doActionPlan(t, engine, http.MethodPost, "/api/v1/apps/action-plans/plan-x/approve", `{"digest":"d"}`)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "ACTION_PLAN_NOT_FOUND") {
		t.Fatalf("unknown plan must be 404: %d %s", w.Code, w.Body.String())
	}
	w = doActionPlan(t, engine, http.MethodGet, "/api/v1/apps/action-plans/plan-x", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown plan GET must be 404: %d %s", w.Code, w.Body.String())
	}
}
