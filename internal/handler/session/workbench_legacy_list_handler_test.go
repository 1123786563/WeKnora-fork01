package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type legacyListerStub struct {
	page    repository.WorkbenchLegacyPage
	err     error
	tenant  uint64
	owner   string
	filter  repository.WorkbenchLegacyFilter
	invoked bool
}

func (s *legacyListerStub) ListOwnedLegacyTasks(_ context.Context, tenantID uint64, ownerID string, filter repository.WorkbenchLegacyFilter) (repository.WorkbenchLegacyPage, error) {
	s.invoked = true
	s.tenant, s.owner, s.filter = tenantID, ownerID, filter
	return s.page, s.err
}

func TestListLegacyTasksRequiresAuthenticatedIdentity(t *testing.T) {
	for name, identity := range map[string][2]any{
		"no tenant": {uint64(0), "u1"},
		"no owner":  {uint64(1), ""},
	} {
		lists := &legacyListerStub{}
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/workbench/legacy-tasks", nil)
		listContext(c, identity[0].(uint64), identity[1].(string))
		NewWorkbenchLegacyListHandler(lists).ListLegacyTasks(c)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, name)
		require.False(t, lists.invoked, name)
	}
}

func TestListLegacyTasksPassesQueryFacetsOnly(t *testing.T) {
	lists := &legacyListerStub{page: repository.WorkbenchLegacyPage{Items: []repository.WorkbenchLegacyTaskSummary{{
		TaskID: "lg-1", Title: "旧聊天", Attention: "none", UpdatedAt: "2026-09-20T08:00:00Z", Kind: "legacy",
	}}, NextCursor: "cursor-2"}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/legacy-tasks?q=%E5%91%A8%E6%8A%A5&archived=true&limit=15&cursor=abc", nil)
	listContext(c, 1, "u1")
	NewWorkbenchLegacyListHandler(lists).ListLegacyTasks(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, uint64(1), lists.tenant)
	require.Equal(t, "u1", lists.owner)
	require.Equal(t, "周报", lists.filter.Query)
	require.True(t, lists.filter.ArchivedOnly)
	require.Equal(t, 15, lists.filter.Limit)
	require.Equal(t, "abc", lists.filter.Cursor)

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Items      []repository.WorkbenchLegacyTaskSummary `json:"items"`
			NextCursor string                                  `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "lg-1", envelope.Data.Items[0].TaskID)
	require.Equal(t, "legacy", envelope.Data.Items[0].Kind)
	require.Equal(t, "cursor-2", envelope.Data.NextCursor)
}

func TestListLegacyTasksRejectsBadCursorLimitAndSurfacesServerFailure(t *testing.T) {
	lists := &legacyListerStub{err: repository.ErrWorkbenchCursor}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/legacy-tasks?cursor=forged", nil)
	listContext(c, 1, "u1")
	NewWorkbenchLegacyListHandler(lists).ListLegacyTasks(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	badLimit := &legacyListerStub{}
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/legacy-tasks?limit=-3", nil)
	listContext(c, 1, "u1")
	NewWorkbenchLegacyListHandler(badLimit).ListLegacyTasks(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.False(t, badLimit.invoked)

	failed := &legacyListerStub{err: context.Canceled}
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/legacy-tasks", nil)
	listContext(c, 1, "u1")
	NewWorkbenchLegacyListHandler(failed).ListLegacyTasks(c)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
