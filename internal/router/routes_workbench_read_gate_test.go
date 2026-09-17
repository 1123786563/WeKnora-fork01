package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
)

func w34ReadGateBool(v bool) *bool { return &v }

func serveWithReadGate(t *testing.T, cfg *config.Config) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	reached := false
	engine.GET("/probe", workbenchReadGate(cfg), func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe", nil))
	t.Cleanup(func() { _ = reached })
	return rec
}

// TestWorkbenchReadGateAnswers503WhenReadsDisabled is the W34 production
// wiring test for read_enabled: closing the read lane must answer 503 on the
// workbench read routes without reaching the handler.
func TestWorkbenchReadGateAnswers503WhenReadsDisabled(t *testing.T) {
	rec := serveWithReadGate(t, &config.Config{Workbench: &config.WorkbenchConfig{ReadEnabled: w34ReadGateBool(false)}})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "workbench reads are disabled") {
		t.Fatalf("body should name the closed lane, got %q", rec.Body.String())
	}
}

// TestWorkbenchReadGateOpenWhenUnset pins the safe-on default: nil config and
// an unset workbench section keep the read lane open.
func TestWorkbenchReadGateOpenWhenUnset(t *testing.T) {
	for _, cfg := range []*config.Config{nil, {}} {
		rec := serveWithReadGate(t, cfg)
		if rec.Code != http.StatusOK {
			t.Fatalf("cfg=%v: status = %d, want %d", cfg, rec.Code, http.StatusOK)
		}
	}
}

// TestWorkbenchSourceEventsReachableWhenReadsDisabled pins the final-review
// F2 contract on the production route wiring: POST
// /workbench/executions/:run_id/source-events is the Paseo bridge's
// authenticated write callback, so it must stay reachable while the read
// gate is closed, while sibling reads answer the gate's 503. The handler is
// built without an OwnedRunReader, so owned() answers 401 — reaching the
// handler's own 401 proves the read gate did not intercept the write.
func TestWorkbenchSourceEventsReachableWhenReadsDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	cfg := &config.Config{Workbench: &config.WorkbenchConfig{ReadEnabled: w34ReadGateBool(false)}}
	g := &rbacGuards{cfg: cfg}
	RegisterWorkbenchRoutes(engine.Group("/api/v1"), session.NewWorkbenchReadHandler(nil, nil), nil, g)

	read := httptest.NewRecorder()
	engine.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/v1/workbench/executions/run-1", nil))
	if read.Code != http.StatusServiceUnavailable || !strings.Contains(read.Body.String(), "workbench reads are disabled") {
		t.Fatalf("read path: status = %d body = %q, want the gate 503", read.Code, read.Body.String())
	}

	ingest := httptest.NewRecorder()
	engine.ServeHTTP(ingest, httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions/run-1/source-events", nil))
	if ingest.Code == http.StatusServiceUnavailable || strings.Contains(ingest.Body.String(), "workbench reads are disabled") {
		t.Fatalf("source-events: status = %d body = %q, want the write path to bypass the read gate", ingest.Code, ingest.Body.String())
	}
	if ingest.Code != http.StatusUnauthorized {
		t.Fatalf("source-events: status = %d, want the handler's own 401 (gate bypassed, handler reached)", ingest.Code)
	}
}
