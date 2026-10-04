package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestCommercialWebhookForgedSignatureIsRejected (#98 AC1, HTTP face): a
// bad signature answers 401 — nothing lands in the inbox.
func TestCommercialWebhookForgedSignatureIsRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	hooks := commercialsvc.NewWebhookService(db, map[string][]byte{"lago": []byte("whsec-http")}, nil)
	engine := gin.New()
	engine.POST("/api/v1/commercial/webhooks/:provider", NewCommercialWebhookHandler(hooks).HandleWebhook)

	body := `{"event_id":"evt-1","webhook_type":"subscription","object_id":"sub_ext_1"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/webhooks/lago", strings.NewReader(body))
	req.Header.Set("X-WeKnora-Signature", "sha256=deadbeef")
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("forged signature: status %d, want 401", w.Code)
	}
}

// TestCommercialWebhookUnconfiguredProviderFailsClosed: no secret wired
// for the provider → honest 503, provider retries.
func TestCommercialWebhookUnconfiguredProviderFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	hooks := commercialsvc.NewWebhookService(db, nil, nil)
	engine := gin.New()
	engine.POST("/api/v1/commercial/webhooks/:provider", NewCommercialWebhookHandler(hooks).HandleWebhook)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/webhooks/lago", strings.NewReader(`{}`))
	req.Header.Set("X-WeKnora-Signature", "sha256=deadbeef")
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured provider: status %d, want 503", w.Code)
	}
}
