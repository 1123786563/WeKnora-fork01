package session

// T13 (#43) Task 4: the compliance HTTP surface. Identity always comes from
// the authenticated context (taskGrantCaller shape); every service error
// maps onto the workbench envelope (400/403/404/409/500).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeComplianceManager struct {
	policy     *types.TenantTaskPolicy
	policyErr  error
	facts      *types.TaskMetadataFacts
	setCalls   int
	access     types.TaskComplianceAccess
	accessErr  error
	content    types.TaskContentView
	contentErr error
	lastAccess struct {
		taskID string
		reason string
		ttl    time.Duration
	}
	lastContentTask string
}

func (f *fakeComplianceManager) GetTaskPolicy(context.Context, types.Caller) (*types.TenantTaskPolicy, error) {
	return f.policy, f.policyErr
}

func (f *fakeComplianceManager) SetTaskPolicy(_ context.Context, _ types.Caller, retentionDays int, legalHold bool) (*types.TenantTaskPolicy, error) {
	f.setCalls++
	if f.policyErr != nil {
		return nil, f.policyErr
	}
	return &types.TenantTaskPolicy{TenantID: 1, RetentionDays: retentionDays, LegalHold: legalHold}, nil
}

func (f *fakeComplianceManager) TaskMetadata(_ context.Context, _ types.Caller, taskID string) (types.TaskMetadataView, error) {
	if f.policyErr != nil {
		return types.TaskMetadataView{}, f.policyErr
	}
	return types.TaskMetadataView{Metadata: *f.facts}, nil
}

func (f *fakeComplianceManager) RequestContentAccess(_ context.Context, _ types.Caller, taskID, reason string, ttl time.Duration) (types.TaskComplianceAccess, error) {
	f.lastAccess.taskID, f.lastAccess.reason, f.lastAccess.ttl = taskID, reason, ttl
	return f.access, f.accessErr
}

func (f *fakeComplianceManager) ReadTaskContent(_ context.Context, _ types.Caller, taskID string) (types.TaskContentView, error) {
	f.lastContentTask = taskID
	return f.content, f.contentErr
}

func complianceRequest(t *testing.T, method, path, body string, role types.TenantRole) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u9")
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	return req.WithContext(ctx), httptest.NewRecorder()
}

func newComplianceTestEngine(t *testing.T, fake *fakeComplianceManager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewWorkbenchTaskComplianceHandler(fake)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.GET("/workbench/compliance/task-policy", h.GetTaskPolicy)
	v1.PUT("/workbench/compliance/task-policy", h.SetTaskPolicy)
	v1.GET("/workbench/compliance/tasks/:task_id", h.TaskMetadata)
	v1.POST("/workbench/compliance/tasks/:task_id/access", h.RequestContentAccess)
	v1.GET("/workbench/compliance/tasks/:task_id/content", h.ReadTaskContent)
	return r
}

func TestCompliancePolicyEndpoints(t *testing.T) {
	fake := &fakeComplianceManager{}
	engine := newComplianceTestEngine(t, fake)

	req, w := complianceRequest(t, http.MethodPut, "/api/v1/workbench/compliance/task-policy",
		`{"retention_days":30,"legal_hold":true}`, types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, fake.setCalls)

	// A negative retention is refused before the service (defense in depth:
	// the service repeats the check).
	req, w = complianceRequest(t, http.MethodPut, "/api/v1/workbench/compliance/task-policy",
		`{"retention_days":-5,"legal_hold":false}`, types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	req, w = complianceRequest(t, http.MethodGet, "/api/v1/workbench/compliance/task-policy", "", types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// A viewer never reaches the service (route-level role is Admin+ in
	// production; the handler repeats the predicate for unguarded mounts).
	req, w = complianceRequest(t, http.MethodGet, "/api/v1/workbench/compliance/task-policy", "", types.TenantRoleViewer)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "handler repeats the admin predicate: %s", w.Body.String())
}

func TestComplianceAccessEndpoints(t *testing.T) {
	fake := &fakeComplianceManager{
		facts:   &types.TaskMetadataFacts{TaskID: "s1", Title: "task-s1", OwnerID: "u1"},
		access:  types.TaskComplianceAccess{ID: 7, TenantID: 1, TaskID: "s1", AdminID: "u9", Reason: "audit"},
		content: types.TaskContentView{TaskID: "s1", Messages: []types.TaskMessageFact{{ID: "m1", Role: "user", Content: "secret"}}},
	}
	engine := newComplianceTestEngine(t, fake)

	req, w := complianceRequest(t, http.MethodPost, "/api/v1/workbench/compliance/tasks/s1/access",
		`{"reason":"audit trail","ttl_hours":72}`, types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Equal(t, "s1", fake.lastAccess.taskID)
	require.Equal(t, "audit trail", fake.lastAccess.reason)
	require.Equal(t, 72*time.Hour, fake.lastAccess.ttl)

	req, w = complianceRequest(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1/content", "", types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "secret")
	require.Equal(t, "s1", fake.lastContentTask)

	// Malformed bodies are 400, never a panic.
	req, w = complianceRequest(t, http.MethodPost, "/api/v1/workbench/compliance/tasks/s1/access",
		`{"reason":"x","ttl_hours":0}`, types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestComplianceHandlerMapsErrorCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"bad request", apperrors.NewBadRequestError("x"), http.StatusBadRequest},
		{"forbidden", apperrors.NewForbiddenError("x"), http.StatusForbidden},
		{"not found", apperrors.NewNotFoundError("x"), http.StatusNotFound},
		{"legal hold", apperrors.NewConflictError("x"), http.StatusConflict},
		{"internal", apperrors.NewInternalServerError("x"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeComplianceManager{policyErr: tc.err}
			engine := newComplianceTestEngine(t, fake)
			req, w := complianceRequest(t, http.MethodGet, "/api/v1/workbench/compliance/task-policy", "", types.TenantRoleAdmin)
			engine.ServeHTTP(w, req)
			require.Equal(t, tc.want, w.Code, "%s: %s", tc.name, w.Body.String())
		})
	}
}
