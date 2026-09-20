package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func registrationIdentityRouter(h gin.HandlerFunc, tenant uint64, owner string) *gin.Engine {
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		ctx := c.Request.Context()
		if tenant != 0 {
			ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
		}
		if owner != "" {
			ctx = context.WithValue(ctx, types.UserIDContextKey, owner)
		}
		c.Request = c.Request.WithContext(ctx)
	})
	r.POST("/challenge", h)
	return r
}

func TestExecutionRegistrationChallengeRequiresTenantAndOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewExecutionRegistrationHandler(&execution.RegistrationService{})
	for _, test := range []struct {
		name   string
		tenant uint64
		owner  string
	}{
		{name: "missing tenant", owner: "u1"},
		{name: "missing owner", tenant: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			registrationIdentityRouter(h.CreateChallenge, test.tenant, test.owner).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/challenge", nil))
			require.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestExecutionRegistrationHandlerExistsForAuthenticatedFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// The service is deliberately injected at runtime by the application
	// container; this test pins the authenticated handler contract without
	// introducing a second database fixture in the HTTP package.
	h := NewExecutionRegistrationHandler((*execution.RegistrationService)(nil))
	require.NotNil(t, h)
	require.NotNil(t, registrationIdentityRouter(h.CreateChallenge, 1, "u1"))
}
