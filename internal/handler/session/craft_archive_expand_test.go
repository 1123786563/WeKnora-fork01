package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// craftExpandAPI embeds the handler's service view so only the seam under
// test needs a real implementation.
type craftExpandAPI struct {
	CraftSessionAPI

	scope craft.Scope
	ref   string
	calls int
}

func (f *craftExpandAPI) ExpandArchive(_ context.Context, scope craft.Scope, ref string) ([]craft.Input, error) {
	f.calls++
	f.scope = scope
	f.ref = ref
	if scope.TenantID != 1 || scope.UserID != "u1" || scope.SessionID != "s-expand" {
		return nil, craft.ErrForbidden
	}
	if ref != "resource://0000000000000000000001" {
		return nil, craft.ErrNotFound
	}
	return []craft.Input{
		{Ref: "resource://0000000000000000000002", Name: "ledger.json", SHA256: "aa", Bytes: 12,
			Recognition: &craft.InputRecognition{Accepted: true, Understood: true}},
		{Ref: "resource://0000000000000000000003", Name: "notes.bin", SHA256: "bb", Bytes: 4},
	}, nil
}

func TestCraftInputExpandHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &craftExpandAPI{}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.UserIDContextKey, "u1"))
		c.Next()
	})
	RegisterCraftSessionRoutes(nil, r.Group("/sessions"), NewCraftSessionHandler(fake), nil)

	req := httptest.NewRequest(http.MethodPost, "/sessions/s-expand/craft/inputs/expand",
		strings.NewReader(`{"resource_ref":"resource://0000000000000000000001"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	require.Equal(t, 1, fake.calls)
	require.Equal(t, "resource://0000000000000000000001", fake.ref)
	require.Equal(t, craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s-expand"}, fake.scope)

	var payload struct {
		Success bool             `json:"success"`
		Data    []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Len(t, payload.Data, 2, "the response reuses the Input projection for every extracted member")
	require.Equal(t, "ledger.json", payload.Data[0]["name"])
	require.Equal(t, "resource://0000000000000000000002", payload.Data[0]["ref"])
	require.Equal(t, map[string]any{"accepted": true, "understood": true, "reason": ""}, payload.Data[0]["recognition"])
	_, hasRecognition := payload.Data[1]["recognition"]
	require.False(t, hasRecognition, "unrecognized members project without a recognition object")
}

func TestCraftInputExpandHTTPRejectsUnknownRefAndCrossScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &craftExpandAPI{}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.UserIDContextKey, "u1"))
		c.Next()
	})
	RegisterCraftSessionRoutes(nil, r.Group("/sessions"), NewCraftSessionHandler(fake), nil)

	req := httptest.NewRequest(http.MethodPost, "/sessions/s-expand/craft/inputs/expand",
		strings.NewReader(`{"resource_ref":"resource://unknown"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code, "an unknown ref must surface the service's ErrNotFound")

	// A foreign session never reaches the service contract's happy path.
	req = httptest.NewRequest(http.MethodPost, "/sessions/s-other/craft/inputs/expand",
		strings.NewReader(`{"resource_ref":"resource://0000000000000000000001"}`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code, "a cross-scope expand must be forbidden")
}
