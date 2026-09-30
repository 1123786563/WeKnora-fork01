package session

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeTaskStateMutator struct {
	err     error
	lastArg struct {
		tenantID uint64
		ownerID  string
		taskID   string
		archived bool
	}
}

func (f *fakeTaskStateMutator) SetTaskArchived(_ context.Context, tenantID uint64, ownerID, taskID string, archived bool, _ time.Time) error {
	f.lastArg.tenantID, f.lastArg.ownerID, f.lastArg.taskID, f.lastArg.archived = tenantID, ownerID, taskID, archived
	return f.err
}

func taskStateContext(tenantID uint64, userID string) (context.Context, *gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := context.Background()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/workbench/tasks/s-1/archive", nil).WithContext(ctx)
	return ctx, c, recorder
}

func TestWorkbenchTaskStateHandlerMapsIdentityAndErrors(t *testing.T) {
	mutator := &fakeTaskStateMutator{}
	handler := &WorkbenchTaskStateHandler{states: mutator}

	// 无身份：401，mutator 未被调用。
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/workbench/tasks/s-1/archive", nil)
	handler.Archive(c)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Empty(t, mutator.lastArg.taskID)

	// 归属不存在：404（ErrWorkbenchTaskNotFound）。
	notFound := &fakeTaskStateMutator{err: repository.ErrWorkbenchTaskNotFound}
	nfHandler := &WorkbenchTaskStateHandler{states: notFound}
	_, c, recorder = taskStateContext(7, "u1")
	c.Params = gin.Params{{Key: "task_id", Value: "s-1"}}
	nfHandler.Archive(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)

	// 成功归档：身份取自 context（非路由参数），task_id 取自路由。
	_, c, recorder = taskStateContext(7, "u1")
	c.Params = gin.Params{{Key: "task_id", Value: "s-1"}}
	handler.Archive(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, uint64(7), mutator.lastArg.tenantID)
	require.Equal(t, "u1", mutator.lastArg.ownerID)
	require.Equal(t, "s-1", mutator.lastArg.taskID)
	require.True(t, mutator.lastArg.archived)
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			TaskID   string `json:"task_id"`
			Archived bool   `json:"archived"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	require.Equal(t, "s-1", envelope.Data.TaskID)
	require.True(t, envelope.Data.Archived)

	// 恢复：DELETE 同一路径，archived=false。
	_, c, recorder = taskStateContext(7, "u1")
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/workbench/tasks/s-1/archive", nil).WithContext(c.Request.Context())
	c.Params = gin.Params{{Key: "task_id", Value: "s-1"}}
	handler.Restore(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.False(t, mutator.lastArg.archived)

	// 其它仓储错误：500，不吞错。
	broken := &fakeTaskStateMutator{err: errors.New("db down")}
	brokenHandler := &WorkbenchTaskStateHandler{states: broken}
	_, c, recorder = taskStateContext(7, "u1")
	c.Params = gin.Params{{Key: "task_id", Value: "s-1"}}
	brokenHandler.Archive(c)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
