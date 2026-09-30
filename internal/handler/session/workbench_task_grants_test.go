package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeTaskGrantManager struct {
	grant     *types.TaskGrant
	grantErr  error
	revokeErr error
	list      []types.TaskGrant
	listErr   error
	lastGrant struct {
		taskID    string
		granteeID string
		role      types.TaskGrantRole
	}
	lastRevoke struct{ taskID, granteeID string }
}

func (f *fakeTaskGrantManager) GrantTaskAccess(
	_ context.Context, _ types.Caller, taskID, granteeID string, role types.TaskGrantRole,
) (*types.TaskGrant, error) {
	f.lastGrant.taskID, f.lastGrant.granteeID, f.lastGrant.role = taskID, granteeID, role
	if f.grantErr != nil {
		return nil, f.grantErr
	}
	return f.grant, nil
}

func (f *fakeTaskGrantManager) RevokeTaskAccess(
	_ context.Context, _ types.Caller, taskID, granteeID string,
) error {
	f.lastRevoke.taskID, f.lastRevoke.granteeID = taskID, granteeID
	return f.revokeErr
}

func (f *fakeTaskGrantManager) ListTaskGrants(
	_ context.Context, _ types.Caller, _ string,
) ([]types.TaskGrant, error) {
	return f.list, f.listErr
}

func taskGrantContext(t *testing.T, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := context.Background()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "owner-1")
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleContributor)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	c.Request = httptest.NewRequest(method, path, reader).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	return c, recorder
}

func TestTaskGrantsHandlerGrantMapsInputsAndErrors(t *testing.T) {
	manager := &fakeTaskGrantManager{grant: &types.TaskGrant{
		TenantID: 1, TaskID: "s1", GranteeID: "member-2",
		Role: types.TaskGrantRoleViewer, GrantedBy: "owner-1",
	}}
	handler := &WorkbenchTaskGrantsHandler{grants: manager}

	// Happy path: 201 + the persisted grant on the wire.
	c, recorder := taskGrantContext(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"member-2","role":"viewer"}`)
	c.Params = gin.Params{{Key: "task_id", Value: "s1"}}
	handler.Grant(c)
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"grantee_id":"member-2"`)
	require.Equal(t, "s1", manager.lastGrant.taskID)
	require.Equal(t, "member-2", manager.lastGrant.granteeID)
	require.Equal(t, types.TaskGrantRoleViewer, manager.lastGrant.role)
	// Identity comes from the context, never from the body.
	require.NotContains(t, recorder.Body.String(), "granted_by\":\"\"")

	// Malformed body / role: 400 before the service is reached.
	c, recorder = taskGrantContext(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"member-2","role":"owner"}`)
	c.Params = gin.Params{{Key: "task_id", Value: "s1"}}
	handler.Grant(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	c, recorder = taskGrantContext(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants", `not-json`)
	c.Params = gin.Params{{Key: "task_id", Value: "s1"}}
	handler.Grant(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	// Service 403/404 keep their AppError status; other errors are 500.
	for _, tc := range []struct {
		err  error
		code int
	}{
		{apperrors.NewForbiddenError("no"), http.StatusForbidden},
		{apperrors.NewNotFoundError("no"), http.StatusNotFound},
		{apperrors.NewBadRequestError("no"), http.StatusBadRequest},
	} {
		denied := &WorkbenchTaskGrantsHandler{grants: &fakeTaskGrantManager{grantErr: tc.err}}
		c, recorder = taskGrantContext(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
			`{"grantee_id":"member-2","role":"viewer"}`)
		c.Params = gin.Params{{Key: "task_id", Value: "s1"}}
		denied.Grant(c)
		require.Equal(t, tc.code, recorder.Code)
	}
}

func TestTaskGrantsHandlerRevokeAndList(t *testing.T) {
	manager := &fakeTaskGrantManager{}
	handler := &WorkbenchTaskGrantsHandler{grants: manager}

	c, recorder := taskGrantContext(t, http.MethodDelete, "/api/v1/workbench/tasks/s1/grants/member-2", "")
	c.Params = gin.Params{{Key: "task_id", Value: "s1"}, {Key: "grantee_id", Value: "member-2"}}
	handler.Revoke(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"revoked":true`)
	require.Equal(t, "member-2", manager.lastRevoke.granteeID)

	listed := &fakeTaskGrantManager{list: []types.TaskGrant{{
		TenantID: 1, TaskID: "s1", GranteeID: "member-2", Role: types.TaskGrantRoleCollaborator, GrantedBy: "owner-1",
	}}}
	listHandler := &WorkbenchTaskGrantsHandler{grants: listed}
	c, recorder = taskGrantContext(t, http.MethodGet, "/api/v1/workbench/tasks/s1/grants", "")
	c.Params = gin.Params{{Key: "task_id", Value: "s1"}}
	listHandler.List(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"role":"collaborator"`)

	// No identity: 401 without touching the service.
	bare := &WorkbenchTaskGrantsHandler{grants: manager}
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/tasks/s1/grants", nil)
	bare.List(c)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	// Unwired handler fails closed: 503, no route-mounted shims.
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	unwired := &WorkbenchTaskGrantsHandler{}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/tasks/s1/grants", nil)
	unwired.List(c)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}
