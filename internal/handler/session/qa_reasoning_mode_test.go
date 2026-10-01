package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseQARequestRejectsUnknownReasoningMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "session_id", Value: "sess-1"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/knowledge-chat/sess-1", strings.NewReader(`{"query":"q","reasoning_mode":"fast"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h := &Handler{}
	_, _, err := h.parseQARequest(c, "KnowledgeQA", false)
	require.Error(t, err, "reasoning_mode 白名单外取值必须在进入任何服务调用前以 400 拒绝")
}
