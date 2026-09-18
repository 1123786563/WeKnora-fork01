package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type workbenchListerStub struct {
	page    repository.WorkbenchExecutionPage
	err     error
	tenant  uint64
	owner   string
	filter  repository.WorkbenchExecutionFilter
	invoked bool
}

func (s *workbenchListerStub) ListOwnedExecutions(_ context.Context, tenantID uint64, ownerID string, filter repository.WorkbenchExecutionFilter) (repository.WorkbenchExecutionPage, error) {
	s.invoked = true
	s.tenant, s.owner, s.filter = tenantID, ownerID, filter
	return s.page, s.err
}

func listContext(c *gin.Context, tenant uint64, owner string) {
	ctx := c.Request.Context()
	if tenant != 0 {
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
	}
	if owner != "" {
		ctx = context.WithValue(ctx, types.UserIDContextKey, owner)
	}
	c.Request = c.Request.WithContext(ctx)
}

func TestListWorkbenchExecutionsRequiresAuthenticatedIdentity(t *testing.T) {
	for name, identity := range map[string][2]any{
		"no tenant": {uint64(0), "u1"},
		"no owner":  {uint64(1), ""},
	} {
		lists := &workbenchListerStub{}
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/workbench/executions", nil)
		listContext(c, identity[0].(uint64), identity[1].(string))
		NewWorkbenchListHandler(lists).ListWorkbenchExecutions(c)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, name)
		require.False(t, lists.invoked, name)
	}
}

func TestListWorkbenchExecutionsPassesQueryFacetsOnly(t *testing.T) {
	lists := &workbenchListerStub{page: repository.WorkbenchExecutionPage{Items: []repository.WorkbenchExecutionSummary{{
		RunID: "r1", SessionID: "s1", Status: "running", AgentID: "agent-x", TargetID: "platform", WorkspaceRef: "ws", SpaceID: "sp", CreatedAt: "2026-09-12T10:00:00Z",
	}}}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/executions?status=running&agent_id=a%26tenant_id%3Dother&limit=15&cursor=abc", nil)
	listContext(c, 1, "u1")
	NewWorkbenchListHandler(lists).ListWorkbenchExecutions(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, uint64(1), lists.tenant)
	require.Equal(t, "u1", lists.owner)
	require.Equal(t, "running", lists.filter.Status)
	require.Equal(t, "a&tenant_id=other", lists.filter.AgentID)
	require.Equal(t, 15, lists.filter.Limit)
	require.Equal(t, "abc", lists.filter.Cursor)

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Items      []repository.WorkbenchExecutionSummary `json:"items"`
			NextCursor string                                 `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "r1", envelope.Data.Items[0].RunID)
}

func TestListWorkbenchExecutionsRejectsBadCursorAndLimit(t *testing.T) {
	lists := &workbenchListerStub{err: repository.ErrWorkbenchCursor}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/executions?cursor=forged", nil)
	listContext(c, 1, "u1")
	NewWorkbenchListHandler(lists).ListWorkbenchExecutions(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	lists = &workbenchListerStub{}
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/executions?limit=-3", nil)
	listContext(c, 1, "u1")
	NewWorkbenchListHandler(lists).ListWorkbenchExecutions(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.False(t, lists.invoked)
}

func TestListWorkbenchExecutionsSurfacesServerError(t *testing.T) {
	lists := &workbenchListerStub{err: context.Canceled}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/executions", nil)
	listContext(c, 1, "u1")
	NewWorkbenchListHandler(lists).ListWorkbenchExecutions(c)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
