package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type craftAccessHTTPFake struct{ seen craft.Scope }

func (f *craftAccessHTTPFake) Grant(_ context.Context, scope craft.Scope, user string, role craft.TaskRole) error {
	f.seen = scope
	if scope.UserID != "owner" {
		return craft.ErrForbidden
	}
	return nil
}
func (f *craftAccessHTTPFake) Revoke(_ context.Context, scope craft.Scope, user string) error {
	f.seen = scope
	return nil
}
func (f *craftAccessHTTPFake) ListMembers(_ context.Context, scope craft.Scope) ([]craft.TaskMember, error) {
	f.seen = scope
	if scope.UserID != "owner" {
		return nil, craft.ErrForbidden
	}
	return []craft.TaskMember{{UserID: "owner", Role: craft.TaskRoleOwner}}, nil
}

func TestCraftAccessHTTPDerivesAuthority(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &craftAccessHTTPFake{}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		ctx := context.WithValue(c.Request.Context(), types.UserIDContextKey, "owner")
		c.Request = c.Request.WithContext(ctx)
	})
	RegisterCraftAccessRoutes(r.Group("/sessions"), NewCraftAccessHandler(fake))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sessions/task-1/craft/access", strings.NewReader(`{"user_id":"viewer","role":"viewer","tenant_id":99}`))
	r.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusOK, w.Code)
	require.Empty(t, fake.seen.UserID)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/sessions/task-1/craft/access", strings.NewReader(`{"user_id":"viewer","role":"viewer"}`))
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, craft.Scope{TenantID: 7, UserID: "owner", SessionID: "task-1"}, fake.seen)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sessions/task-1/craft/access", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data []craft.TaskMember `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, craft.TaskRoleOwner, body.Data[0].Role)
}
