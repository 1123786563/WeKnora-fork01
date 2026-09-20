package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type scopeResolverStub struct {
	calls int
	err   error
}

func (s *scopeResolverStub) Resolve(ctx context.Context, ref string) (service.SemanticScopeSnapshot, error) {
	s.calls++
	if ctx.Err() != nil {
		return service.SemanticScopeSnapshot{}, ctx.Err()
	}
	if ref != "valid" {
		return service.SemanticScopeSnapshot{}, service.ErrSemanticScopeInvalid
	}
	return service.SemanticScopeSnapshot{SubjectID: "member"}, s.err
}
func TestSemanticInternalScopeAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, token, audience, body string
		err                         error
		want, calls                 int
	}{
		{"missing", "", "semantic", `{"scope_ref":"valid"}`, nil, 401, 0},
		{"wrong", "Bearer user-jwt", "semantic", `{"scope_ref":"valid"}`, nil, 401, 0},
		{"audience", "Bearer service-secret", "wrong", `{"scope_ref":"valid"}`, nil, 401, 0},
		{"malformed", "Bearer service-secret", "semantic", `{"scope_ref":"broken"}`, nil, 401, 1},
		{"expired", "Bearer service-secret", "semantic", `{"scope_ref":"valid"}`, service.ErrSemanticScopeExpired, 401, 1},
		{"changed", "Bearer service-secret", "semantic", `{"scope_ref":"valid"}`, service.ErrSemanticScopeChanged, 401, 1},
		{"storage", "Bearer service-secret", "semantic", `{"scope_ref":"valid"}`, errors.New("service-secret signing-secret SQL failed"), 503, 1},
		{"unknown-field", "Bearer service-secret", "semantic", `{"scope_ref":"valid","tenant_id":1}`, nil, 400, 0},
		{"trailing", "Bearer service-secret", "semantic", `{"scope_ref":"valid"}{}`, nil, 400, 0},
		{"valid", "Bearer service-secret", "semantic", `{"scope_ref":"valid"}`, nil, 200, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &scopeResolverStub{err: tc.err}
			h := NewSemanticInternalHandler(&config.Config{Semantic: &config.SemanticServiceConfig{Enabled: true, ServiceToken: "service-secret", Audience: "semantic"}}, resolver)
			r := gin.New()
			r.POST("/resolve", h.Resolve)
			req := httptest.NewRequest(http.MethodPost, "/resolve", strings.NewReader(tc.body))
			req.Header.Set("Authorization", tc.token)
			req.Header.Set("X-WeKnora-Audience", tc.audience)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.want, w.Code)
			require.Equal(t, tc.calls, resolver.calls)
			require.NotContains(t, w.Body.String(), "service-secret")
			require.NotContains(t, w.Body.String(), "signing-secret")
		})
	}
}
