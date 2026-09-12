package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestSemanticStatusRequiresService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// No semantic service configured: fail closed (503), never fake.
	h := NewSemanticHandler(nil)
	router.GET("/knowledge-bases/:id/semantic/status", h.Status)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/knowledge-bases/kb-1/semantic/status?document_id=d1", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), "semantic")
}

func TestSemanticSearchWithoutServiceFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewSemanticHandler(nil)
	router.POST("/knowledge-bases/:id/semantic/search", h.Search)
	w := httptest.NewRecorder()
	// A body attempting to override the path scope changes nothing:
	// without the service the route still fails closed.
	req := httptest.NewRequest(http.MethodPost,
		"/knowledge-bases/kb-1/semantic/search",
		strings.NewReader(`{"query":"q","tenant_id":999,"kb_id":"other-kb"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSemanticReasonValidatesMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// Non-nil facade so mode validation runs before the backend call.
	h := NewSemanticHandler(&stubFacadeImpl{})
	router.POST("/knowledge-bases/:id/semantic/reason", h.Reason)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/knowledge-bases/kb-1/semantic/reason",
		strings.NewReader(`{"query":"q","mode":"magic"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

type stubFacadeImpl struct{}

func (f *stubFacadeImpl) Status(c *gin.Context, kbID, documentID string) (types.SemanticDocumentStatusWire, error) {
	return types.SemanticDocumentStatusWire{}, nil
}

func (f *stubFacadeImpl) Search(c *gin.Context, kbID, query string) (types.SemanticSearchResponseWire, error) {
	return types.SemanticSearchResponseWire{}, nil
}

func (f *stubFacadeImpl) Reason(c *gin.Context, kbID, query, mode string) (types.SemanticReasonResponseWire, error) {
	return types.SemanticReasonResponseWire{}, nil
}
