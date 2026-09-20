package router

import (
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSemanticInternalScopeRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, enabled := range []bool{false, true} {
		r := gin.New()
		cfg := &config.Config{Semantic: &config.SemanticServiceConfig{Enabled: enabled, ServiceToken: "service-token", Audience: "semantic"}}
		RegisterSemanticInternalRoutes(r, cfg, handler.NewSemanticInternalHandler(cfg, nil))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/internal/semantic/scopes/resolve", nil))
		if enabled {
			require.Equal(t, 401, w.Code)
		} else {
			require.Equal(t, 404, w.Code)
		}
	}
}
