package session

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestLegacyTaskMigrationOnlyNewRunAdmissionGrantsRunCapabilities：spec
// Testing Decisions 的 Migration test——旧 Session（0 run）保持可读、可归档
// （同身份），但 Run 作用域能力在准入前不存在；唯一获得通道是新 Run 准入
// （POST /workbench/executions）。准入后 legacy 投影让位执行列表，Run 事实
// 才可证明。真实 sqlite 迁移库 + 真实 HTTP handler + 真实 AdmissionCoordinator。
func TestLegacyTaskMigrationOnlyNewRunAdmissionGrantsRunCapabilities(t *testing.T) {
	db := openWorkbenchHTTPDB(t) // 种下 tenant 1 / u1 / s1（engine trpc，0 run → 天然 legacy）
	ctx := context.Background()

	legacyStore := repository.NewWorkbenchLegacyListStore(db)
	runStore := repository.NewAgentRunStore(db)

	// 1) 旧 Session 可读：legacy 投影包含 s1，facts-only。
	page, err := legacyStore.ListOwnedLegacyTasks(ctx, 1, "u1", repository.WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, "s1", page.Items[0].TaskID)
	require.Equal(t, "legacy", page.Items[0].Kind)
	require.Equal(t, "none", page.Items[0].Attention)

	// 2) Run 作用域能力在准入前不存在：任何 run 读（含伪造 id）一律 ErrNotFound。
	_, err = runStore.GetOwnedRun(ctx, 1, "u1", "fabricated-run")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)

	// 3) 同身份归档回路（HTTP 层，真实 handler + 真实 store）。
	gin.SetMode(gin.TestMode)
	r := gin.New()
	states := NewWorkbenchTaskStateHandler(repository.NewWorkbenchTaskStateStore(db))
	r.POST("/api/v1/workbench/tasks/:task_id/archive", withIdentity(1, "u1"), states.Archive)
	r.DELETE("/api/v1/workbench/tasks/:task_id/archive", withIdentity(1, "u1"), states.Restore)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/v1/workbench/tasks/s1/archive", nil))
	require.Equal(t, http.StatusOK, resp.Code)
	archivedPage, err := legacyStore.ListOwnedLegacyTasks(ctx, 1, "u1", repository.WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Empty(t, archivedPage.Items, "archived legacy task leaves the default legacy view")
	restored, err := legacyStore.ListOwnedLegacyTasks(ctx, 1, "u1", repository.WorkbenchLegacyFilter{ArchivedOnly: true})
	require.NoError(t, err)
	require.Equal(t, "s1", restored.Items[0].TaskID)
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, httptest.NewRequest(http.MethodDelete, "/api/v1/workbench/tasks/s1/archive", nil))
	require.Equal(t, http.StatusOK, resp.Code)

	// 4) 显式升级 = 新 Run 准入（真实 AdmissionCoordinator，预算适配器复用
	// 既有 integrationBudget）。
	coordinator := workbenchservice.NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), &integrationBudget{}, nil)
	start := NewWorkbenchStartHandler(coordinator)
	r.POST("/api/v1/workbench/executions", withIdentity(1, "u1"), start.Start)
	body := `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"legacy-upgrade-1","text":"upgrade this legacy task","budget_upper":100}`
	resp = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusAccepted, resp.Code)
	var accepted struct {
		Success bool `json:"success"`
		Data    struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &accepted))
	require.True(t, accepted.Success)
	require.NotEmpty(t, accepted.Data.RunID)

	// 5) 准入后：legacy 投影让位执行列表；Run 事实此刻才可证明。
	legacyAfter, err := legacyStore.ListOwnedLegacyTasks(ctx, 1, "u1", repository.WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Empty(t, legacyAfter.Items, "the admitted Run is the explicit upgrade; the task is no longer legacy")
	executions, err := repository.NewWorkbenchListStore(db).ListOwnedExecutions(ctx, 1, "u1", repository.WorkbenchExecutionFilter{})
	require.NoError(t, err)
	require.Len(t, executions.Items, 1)
	require.Equal(t, "s1", executions.Items[0].SessionID)
	require.Equal(t, accepted.Data.RunID, executions.Items[0].RunID)
	admitted, err := runStore.GetOwnedRun(ctx, 1, "u1", accepted.Data.RunID)
	require.NoError(t, err)
	require.Equal(t, "s1", admitted.SessionID)
}
