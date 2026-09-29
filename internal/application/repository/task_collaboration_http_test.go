package repository_test

// End-to-end collaboration evidence (T12 #42). Everything here runs through
// real HTTP handlers over a fully migrated sqlite database: grants API,
// granted task reads, budget extension gate and the owner-scoped list. This
// is the AC3 evidence — no mocked service or hand-written projection.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type taskCollabEnv struct {
	db     *gorm.DB
	engine *gin.Engine
}

func newTaskCollabEnv(t *testing.T) *taskCollabEnv {
	t.Helper()
	db := openTaskGrantDB(t)
	runs := repository.NewAgentRunStore(db)
	_, err := runs.Admit(context.Background(), taskGrantAdmission())
	require.NoError(t, err)

	// Budget fixtures so the extension path reaches its terminal state.
	require.NoError(t, db.Exec(`INSERT INTO commercial_budget_accounts
		(tenant_id, verified_micro, verified_until, version) VALUES (1, 100000000, ?, 0)`,
		time.Now().Add(time.Hour).UTC()).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, limit_micro, deadline, version) VALUES (1, 'r1', 1000, ?, 1)`,
		time.Now().Add(time.Hour).UTC()).Error)

	grantsSvc := service.NewTaskGrantService(
		repository.NewTaskGrantStore(db),
		repository.NewSessionRepository(db),
		repository.NewTenantMemberRepository(db),
	)
	grantsHandler := session.NewWorkbenchTaskGrantsHandler(grantsSvc)
	readHandler := session.NewWorkbenchReadHandler(runs, repository.NewAgentRunSnapshotRepository(db)).WithGrantedRuns(runs)
	listHandler := session.NewWorkbenchListHandler(repository.NewWorkbenchListStore(db))
	commercialHandler := handler.NewCommercialHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.POST("/workbench/tasks/:task_id/grants", grantsHandler.Grant)
	v1.GET("/workbench/tasks/:task_id/grants", grantsHandler.List)
	v1.DELETE("/workbench/tasks/:task_id/grants/:grantee_id", grantsHandler.Revoke)
	v1.GET("/workbench/executions", listHandler.ListWorkbenchExecutions)
	v1.GET("/workbench/executions/:run_id", readHandler.GetWorkbenchExecution)
	v1.GET("/workbench/executions/:run_id/snapshot", readHandler.GetWorkbenchSnapshot)
	v1.POST("/commercial/tasks/:id/budget/extend", commercialHandler.ExtendTaskBudget)
	return &taskCollabEnv{db: db, engine: r}
}

func (e *taskCollabEnv) do(t *testing.T, method, path, body, userID string, tenant uint64, role types.TenantRole) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, tenant)
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func TestTaskCollaborationEndToEndAC1(t *testing.T) {
	env := newTaskCollabEnv(t)

	// The owner explicitly grants viewer(u2) and collaborator(u3).
	w := env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u2","role":"viewer"}`, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w = env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u3","role":"collaborator"}`, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// AC1 gate: the collaborator cannot raise the budget; the owner can.
	w = env.do(t, http.MethodPost, "/api/v1/commercial/tasks/r1/budget/extend",
		`{"additional_credits":10,"idempotency_key":"e2e-k1"}`, "u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusForbidden, w.Code, "Collaborator 不能扩额（AC1）: %s", w.Body.String())
	w = env.do(t, http.MethodPost, "/api/v1/commercial/tasks/r1/budget/extend",
		`{"additional_credits":10,"idempotency_key":"e2e-k1"}`, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, "任务 owner 可以扩额: %s", w.Body.String())

	// Read separation: viewer and collaborator read the task snapshot.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/snapshot", "", "u2", 1, types.TenantRoleViewer)
	require.Equal(t, http.StatusOK, w.Code, "Viewer 可读任务详情: %s", w.Body.String())
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1", "", "u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, "Collaborator 可读任务详情: %s", w.Body.String())

	// Grants management stays owner-only end to end.
	w = env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u4","role":"viewer"}`, "u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusForbidden, w.Code, "grants 管理仅 owner: %s", w.Body.String())
}

func TestTaskCollaborationEndToEndAC2SharedDoesNotWidenVisibility(t *testing.T) {
	env := newTaskCollabEnv(t)

	w := env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u2","role":"viewer"}`, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// The grantee's workbench list stays empty: a shared task does not widen
	// the list's owner scope (AC2).
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions", "", "u2", 1, types.TenantRoleViewer)
	require.Equal(t, http.StatusOK, w.Code)
	var listEnvelope struct {
		Data struct {
			Items []json.RawMessage `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listEnvelope))
	require.Empty(t, listEnvelope.Data.Items, "grantee 列表不含 owner 任务（AC2）")

	// The owner's list still shows the task.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions", "", "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listEnvelope))
	require.Len(t, listEnvelope.Data.Items, 1)

	// A bystander without a grant cannot read the task at all.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/snapshot", "", "u4", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusNotFound, w.Code, "无 grant 成员默认不可见（私有任务）")

	// Cross-tenant probe is a uniform 404.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/snapshot", "", "u2", 2, types.TenantRoleViewer)
	require.Equal(t, http.StatusNotFound, w.Code)
	w = env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u2","role":"viewer"}`, "u1", 2, types.TenantRoleOwner)
	require.Equal(t, http.StatusNotFound, w.Code)

	// Revoking closes the read on the very next request (no caching).
	w = env.do(t, http.MethodDelete, "/api/v1/workbench/tasks/s1/grants/u2", "", "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/snapshot", "", "u2", 1, types.TenantRoleViewer)
	require.Equal(t, http.StatusNotFound, w.Code, "撤销立即生效")
}
