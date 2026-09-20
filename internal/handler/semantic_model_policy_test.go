package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSemanticModelPolicyHandlerRejectsUnknownAndTrailingJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSemanticModelPolicyHandler(nil)
	for _, body := range []string{`{"unknown":true}`, `{} {}`} {
		r := gin.New()
		r.Use(middleware.ErrorHandler())
		r.PUT("/knowledge-bases/:id/semantic-model-policy", h.Put)
		req := httptest.NewRequest(http.MethodPut, "/knowledge-bases/kb-1/semantic-model-policy", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
}
