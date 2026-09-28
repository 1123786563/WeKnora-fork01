package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubSecurityService struct {
	interfaces.AgentSecurityService
	revokeView interfaces.AgentSecurityRevocationView
	revokeErr  error
	getView    interfaces.AgentSecurityRevocationView
	getErr     error
	listRows   []interfaces.AgentSecurityRevocationView
}

func (s *stubSecurityService) RevokeRelease(_ context.Context, _ uint64, _ string, _ interfaces.ReleaseRevocationInput) (interfaces.AgentSecurityRevocationView, error) {
	return s.revokeView, s.revokeErr
}
func (s *stubSecurityService) RevokeDependency(_ context.Context, _ uint64, _ string, _ interfaces.DependencyRevocationInput) (interfaces.AgentSecurityRevocationView, error) {
	return s.revokeView, s.revokeErr
}
func (s *stubSecurityService) ListRevocations(_ context.Context, _ uint64) ([]interfaces.AgentSecurityRevocationView, error) {
	return s.listRows, nil
}
func (s *stubSecurityService) GetRevocation(_ context.Context, _ uint64, _ string) (interfaces.AgentSecurityRevocationView, error) {
	return s.getView, s.getErr
}

func securityHandlerContext(t *testing.T, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "rev-1"}}
	return c, rec
}

func serveSecurityHandler(t *testing.T, method, path, body string, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Handle(method, path, h)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestRevokeReleaseWireEnvelope(t *testing.T) {
	h := NewAgentSecurityHandler(&stubSecurityService{revokeView: interfaces.AgentSecurityRevocationView{
		ID: "rev-1", Kind: "release", Reason: "CVE", RevokedBy: "sec-admin", RevokedAt: time.Now().UTC(),
		InFlightDisposition: "cancel", CanceledRunCount: 2, ReleaseID: "r1", ReplacementReleaseID: "r2",
	}})
	c, rec := securityHandlerContext(t, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases",
		`{"release_id":"r1","reason":"CVE","replacement_release_id":"r2"}`)
	h.RevokeRelease(c)

	require.Equal(t, http.StatusCreated, c.Writer.Status())
	var body struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "rev-1", body.Data["id"])
	require.Equal(t, "release", body.Data["kind"])
	require.Equal(t, "r2", body.Data["replacement_release_id"])
	require.EqualValues(t, 2, body.Data["canceled_run_count"])
}

func TestRevokeWireRejectsMalformedBody(t *testing.T) {
	h := NewAgentSecurityHandler(&stubSecurityService{revokeErr: service.ErrAgentSecurityInvalidInput})
	rec := serveSecurityHandler(t, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", `{"reason":"x"}`, h.RevokeRelease)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	h2 := NewAgentSecurityHandler(&stubSecurityService{revokeErr: service.ErrAgentSecurityReleaseUnresolvable})
	rec = serveSecurityHandler(t, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", `{"release_id":"nope","reason":"x"}`, h2.RevokeRelease)
	require.Equal(t, http.StatusBadRequest, rec.Code, "不可解析 Release = 400")

	h3 := NewAgentSecurityHandler(&stubSecurityService{getErr: service.ErrAgentSecurityNotFound})
	rec = serveSecurityHandler(t, http.MethodGet, "/api/v1/marketplace/tenant/security-revocations/:id", "", h3.GetRevocation)
	require.Equal(t, http.StatusNotFound, rec.Code, "跨租户/不存在同形 404")

	h4 := NewAgentSecurityHandler(&stubSecurityService{})
	rec = serveSecurityHandler(t, http.MethodPost, "/api/v1/marketplace/tenant/security-revocations/releases", `{"release_id":"r1","reason":"x","extra":1}`, h4.RevokeRelease)
	require.Equal(t, http.StatusBadRequest, rec.Code, "未知字段拒绝（spoof 面收敛）")
}
