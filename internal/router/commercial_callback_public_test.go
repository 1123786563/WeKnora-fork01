package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/gin-gonic/gin"
)

// TestProviderCallbackRouteIsAnonymouslyReachable (issue #82 flow defect 1):
// channel servers (Alipay/WeChat) POST payment notifications with NO session,
// API key or bearer token — the callback endpoint's authenticity model is the
// provider signature verified inside the handler. The REAL global Auth
// middleware must therefore let an anonymous POST reach the handler instead of
// answering 401 before it (the validation run hit exactly that: every real
// channel push died with "missing authentication").
//
// The reachability proof uses the fail-closed handler posture: with no
// providers wired, an anonymous POST that REACHES the handler answers 503
// (unknown payment provider / alipay branch) — a 401 means the Auth
// middleware blocked it first.
func TestProviderCallbackRouteIsAnonymouslyReachable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// The production posture: the global Auth middleware mounted on the
	// engine BEFORE the /api/v1 tree (router.go does exactly this), with no
	// service dependencies — an anonymous request has no bearer/API key, so
	// every non-whitelisted path 401s.
	engine.Use(middleware.Auth(nil, nil, nil, nil, nil))
	v1 := engine.Group("/api/v1")
	RegisterCommercialRoutes(v1, handler.NewCommercialHandler(nil), nil)

	for _, provider := range []string{"alipay", "wechat"} {
		t.Run(provider, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/commercial/callbacks/"+provider, strings.NewReader("body=1"))
			engine.ServeHTTP(w, req)
			if w.Code == http.StatusUnauthorized {
				t.Fatalf("anonymous channel callback must reach the handler, got 401: %s", w.Body.String())
			}
			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("nil-provider handler must fail closed 503 once reached, got %d: %s", w.Code, w.Body.String())
			}
		})
	}

	// Negative control: the whitelist admits ONLY the callback prefix — an
	// anonymous POST anywhere else under /commercial stays behind Auth.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/orders", strings.NewReader("{}"))
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("non-callback commercial route must still require auth, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCommercialWebhookRouteIsAnonymouslyReachable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.Auth(nil, nil, nil, nil, nil))
	v1 := engine.Group("/api/v1")
	RegisterCommercialRoutes(v1, handler.NewCommercialHandler(nil), handler.NewCommercialWebhookHandler(nil))

	t.Run("anonymous POST reaches fail-closed webhook handler", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/webhooks/lago", strings.NewReader("body=1"))
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("anonymous webhook POST must reach the nil handler and fail closed with 503, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("webhook GET remains authenticated", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/commercial/webhooks/lago", nil)
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous webhook GET must remain behind auth, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("unrelated commercial POST remains authenticated", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/orders", strings.NewReader("{}"))
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous unrelated commercial POST must remain behind auth, got %d: %s", w.Code, w.Body.String())
		}
	})
}
