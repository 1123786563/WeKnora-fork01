package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	agentcatalogservice "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// Characterization tests for the user resource favorite handler, anchored on
// the legacy host-package implementation before the Pass B (25a) move to
// internal/modules/agentcatalog/handler.

type fakeFavoriteService struct {
	listResult []*types.UserResourceFavorite
	addErr     error
	removeErr  error

	listCalls   []string
	addCalls    []string
	removeCalls []string
}

func (f *fakeFavoriteService) List(_ context.Context, userID string, _ uint64, resourceType string) ([]*types.UserResourceFavorite, error) {
	f.listCalls = append(f.listCalls, userID+"|"+resourceType)
	return f.listResult, nil
}

func (f *fakeFavoriteService) Add(_ context.Context, userID string, _ uint64, resourceType, resourceID string) error {
	f.addCalls = append(f.addCalls, userID+"|"+resourceType+"|"+resourceID)
	return f.addErr
}

func (f *fakeFavoriteService) Remove(_ context.Context, userID string, _ uint64, resourceType, resourceID string) error {
	f.removeCalls = append(f.removeCalls, userID+"|"+resourceType+"|"+resourceID)
	return f.removeErr
}

// favoriteAuthStub installs the (user, workspace) gin context keys the
// handler resolves, mirroring what the auth middleware provides in
// production. Empty values are withheld to exercise the 401 branches.
func favoriteAuthStub(userID string, tenantID uint64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if userID != "" {
			c.Set(types.UserIDContextKey.String(), userID)
		}
		if tenantID != 0 {
			c.Set(types.TenantIDContextKey.String(), tenantID)
		}
		c.Next()
	}
}

func newFavoriteTestRouter(svc interfaces.UserResourceFavoriteService, userID string, tenantID uint64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewUserResourceFavoriteHandler(svc)
	r := gin.New()
	r.Use(favoriteAuthStub(userID, tenantID))
	r.Use(middleware.ErrorHandler())
	v1 := r.Group("/api/v1")
	favs := v1.Group("/user/favorites")
	{
		favs.GET("", h.ListFavorites)
		favs.POST("", h.AddFavorite)
		favs.DELETE("/:type/:id", h.RemoveFavorite)
	}
	return r
}

func TestFavoriteHandlerRequiresUserAndWorkspaceContext(t *testing.T) {
	svc := &fakeFavoriteService{}

	// No user id on the gin context → 401.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/favorites?type=kb", nil)
	newFavoriteTestRouter(svc, "", 0).ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// User present but no workspace → 401.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/user/favorites?type=kb", nil)
	newFavoriteTestRouter(svc, "user-1", 0).ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Empty(t, svc.listCalls, "rejected requests never reach the service")
}

func TestFavoriteHandlerListReturnsServiceFavorites(t *testing.T) {
	svc := &fakeFavoriteService{
		listResult: []*types.UserResourceFavorite{{ResourceID: "kb-1", ResourceType: types.ResourceTypeKB}},
	}
	r := newFavoriteTestRouter(svc, "user-1", 7)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/favorites?type=kb", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    []struct {
			ResourceID string `json:"resource_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Len(t, body.Data, 1)
	require.Equal(t, "kb-1", body.Data[0].ResourceID)
	require.Equal(t, []string{"user-1|kb"}, svc.listCalls, "the query type reaches the service with the ctx identity")
}

func TestFavoriteHandlerAddMapsSentinelsToBadRequest(t *testing.T) {
	post := func(r *gin.Engine, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/user/favorites", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	// The invalid-type sentinel maps to 400.
	r := newFavoriteTestRouter(&fakeFavoriteService{addErr: agentcatalogservice.ErrFavoriteInvalidType}, "user-1", 7)
	require.Equal(t, http.StatusBadRequest, post(r, `{"type":"kb","id":"kb-1"}`).Code)

	// The empty-id sentinel maps to 400.
	r = newFavoriteTestRouter(&fakeFavoriteService{addErr: agentcatalogservice.ErrFavoriteEmptyID}, "user-1", 7)
	require.Equal(t, http.StatusBadRequest, post(r, `{"type":"kb","id":""}`).Code)

	// A non-sentinel service error stays an internal error.
	r = newFavoriteTestRouter(&fakeFavoriteService{addErr: context.DeadlineExceeded}, "user-1", 7)
	require.Equal(t, http.StatusInternalServerError, post(r, `{"type":"kb","id":"kb-1"}`).Code)

	// Happy path: 200 {"success":true} with the decoded body verbatim.
	svc := &fakeFavoriteService{}
	r = newFavoriteTestRouter(svc, "user-1", 7)
	w := post(r, `{"type":"agent","id":"agent-1"}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"success":true}`, w.Body.String())
	require.Equal(t, []string{"user-1|agent|agent-1"}, svc.addCalls)

	// A malformed body is a 400 before the service runs.
	svc2 := &fakeFavoriteService{}
	r = newFavoriteTestRouter(svc2, "user-1", 7)
	require.Equal(t, http.StatusBadRequest, post(r, `{invalid`).Code)
	require.Empty(t, svc2.addCalls)
}

func TestFavoriteHandlerRemoveMapsSentinelsToBadRequest(t *testing.T) {
	// The invalid-type sentinel maps to 400.
	svc := &fakeFavoriteService{removeErr: agentcatalogservice.ErrFavoriteInvalidType}
	r := newFavoriteTestRouter(svc, "user-1", 7)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/user/favorites/kb/kb-1", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Happy path: path params reach the service verbatim, 200 {"success":true}.
	svc = &fakeFavoriteService{}
	r = newFavoriteTestRouter(svc, "user-1", 7)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/user/favorites/agent/agent-9", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"success":true}`, w.Body.String())
	require.Equal(t, []string{"user-1|agent|agent-9"}, svc.removeCalls)
}
