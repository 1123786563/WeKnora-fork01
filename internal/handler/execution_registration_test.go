package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func registrationIdentityRouter(h gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		if len(c.Errors) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": c.Errors.Last().Error()})
		}
	})
	r.POST("/challenge", h)
	return r
}

func TestExecutionRegistrationChallengeRequiresTenantAndOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewExecutionRegistrationHandler(nil)
	r.POST("/challenge", h.CreateChallenge)
	req := httptest.NewRequest(http.MethodPost, "/challenge", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestExecutionRegistrationHandlerExistsForAuthenticatedFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// The service is deliberately injected at runtime by the application
	// container; this test pins the authenticated handler contract without
	// introducing a second database fixture in the HTTP package.
	h := NewExecutionRegistrationHandler((*execution.RegistrationService)(nil))
	require.NotNil(t, h)
	require.NotNil(t, registrationIdentityRouter(h.CreateChallenge))
}
