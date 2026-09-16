package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
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

func TestExecutionTargetCreateRequiresTrustedNodeIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &executionTargetStoreStub{}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": c.Errors.Last().Error()})
		}
	})
	r.POST("/execution-targets", NewExecutionTargetHandler(stub).Create)
	body, _ := json.Marshal(map[string]any{"id": "t1", "kind": "managed_node", "runtime_id": "r1", "external_target_id": "x1", "credential_version": 2})
	request := func(ctx context.Context) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/execution-targets", bytes.NewReader(body)).WithContext(ctx)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	require.Equal(t, http.StatusUnauthorized, request(ctx).Code)
	ctx = execution.WithTrustedTargetIdentity(ctx, execution.TrustedTargetIdentity{RuntimeID: "r1", ExternalTargetID: "x1", CredentialVersion: 1})
	require.Equal(t, http.StatusUnauthorized, request(ctx).Code)
	ctx = execution.WithTrustedTargetIdentity(ctx, execution.TrustedTargetIdentity{RuntimeID: "r1", ExternalTargetID: "x1", CredentialVersion: 2})
	require.Equal(t, http.StatusCreated, request(ctx).Code)
}
