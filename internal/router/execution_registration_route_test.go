package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type routeTargetStore struct{ revoked []string }

func (*routeTargetStore) CreateTarget(context.Context, execution.Target, string) error { return nil }
func (*routeTargetStore) CreateTargetIfTrusted(context.Context, execution.Target, string) error {
	return nil
}
func (*routeTargetStore) GetOwnedTarget(context.Context, uint64, string, string) (execution.Target, error) {
	return execution.Target{}, nil
}
func (*routeTargetStore) ListOwnedTargets(context.Context, uint64, string) ([]execution.Target, error) {
	return nil, nil
}
func (s *routeTargetStore) RevokeTarget(_ context.Context, _ uint64, _ string, id string) error {
	s.revoked = append(s.revoked, id)
	return nil
}
func (*routeTargetStore) CreateWorkspace(context.Context, execution.Workspace) error { return nil }
func (*routeTargetStore) GetOwnedWorkspace(context.Context, uint64, string, string) (execution.Workspace, error) {
	return execution.Workspace{}, nil
}

func TestExecutionRegistrationRoutesUseTargetFacadeForContractRevoke(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &routeTargetStore{}
	target := handler.NewExecutionTargetHandler(store, nil)
	registration := handler.NewExecutionRegistrationHandler(nil)
	g := &rbacGuards{}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "owner")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	api := engine.Group("/api/v1")
	RegisterExecutionRegistrationRoutes(api, registration, g, target)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/node-1/revoke", nil)
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	require.Equal(t, http.StatusNoContent, res.Code)
	require.Equal(t, []string{"node-1"}, store.revoked)
}
