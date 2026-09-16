package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type executionTargetStoreStub struct{ targets []execution.Target }

func (s *executionTargetStoreStub) CreateTarget(context.Context, execution.Target, string) error {
	return nil
}
func (s *executionTargetStoreStub) GetOwnedTarget(context.Context, uint64, string, string) (execution.Target, error) {
	return s.targets[0], nil
}
func (s *executionTargetStoreStub) ListOwnedTargets(context.Context, uint64, string) ([]execution.Target, error) {
	return s.targets, nil
}
func (s *executionTargetStoreStub) RevokeTarget(context.Context, uint64, string, string) error {
	return nil
}
func (s *executionTargetStoreStub) CreateWorkspace(context.Context, execution.Workspace) error {
	return nil
}
func (s *executionTargetStoreStub) GetOwnedWorkspace(context.Context, uint64, string, string) (execution.Workspace, error) {
	return execution.Workspace{}, nil
}

func TestExecutionTargetListDoesNotExposeNodeRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &executionTargetStoreStub{targets: []execution.Target{{ID: "t1", TenantID: 1, OwnerID: "u1", Kind: "managed_node", State: "active", CredentialVersion: 1}}}
	r := gin.New()
	r.GET("/execution-targets", NewExecutionTargetHandler(stub).List)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	req := httptest.NewRequest("GET", "/execution-targets", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
	require.NotContains(t, w.Body.String(), "root_ref")
}
