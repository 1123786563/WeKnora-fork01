package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
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
