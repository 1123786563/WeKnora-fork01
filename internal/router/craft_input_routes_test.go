package router

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

type craftInputRouteAPI struct {
	accepted  []service.CraftInputUpload
	decisions int
}

func (f *craftInputRouteAPI) AcceptInputRound(_ context.Context, scope craft.Scope, uploads []service.CraftInputUpload) ([]craft.Input, error) {
	if scope.TenantID != 1 || scope.UserID != "owner" || scope.SessionID != "task-1" {
		return nil, craft.ErrForbidden
	}
	f.accepted = append([]service.CraftInputUpload(nil), uploads...)
	return []craft.Input{{Ref: "opaque://input-1", Name: uploads[0].Name, SHA256: uploads[0].SHA256, Bytes: int64(len(uploads[0].Content))}}, nil
}

func (f *craftInputRouteAPI) DecideInput(_ context.Context, scope craft.Scope, ref, action string) error {
	if scope.TenantID != 1 || scope.UserID != "owner" || scope.SessionID != "task-1" {
		return craft.ErrForbidden
	}
	if ref != "opaque://input-1" || action != "continue" {
		return errors.New("unexpected decision")
	}
	f.decisions++
	return nil
}

func TestCraftInputRoutesInheritProductionSessionAuthAndAPIKeyPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if c.GetHeader("X-Test-User") == "" && c.GetHeader("X-Test-Key") == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		user, tenant := c.GetHeader("X-Test-User"), uint64(1)
		if c.GetHeader("X-Test-Cross-Tenant") == "yes" {
			user, tenant = "other", 2
		}
		if user != "" {
			ctx = types.WithCaller(ctx, types.Caller{TenantID: tenant, UserID: user, Role: types.TenantRoleContributor})
			ctx = context.WithValue(ctx, types.UserIDContextKey, user)
			c.Set(types.TenantIDContextKey.String(), tenant)
		}
		if key := c.GetHeader("X-Test-Key"); key != "" {
			scope := types.TenantAPIKeyScope{KeyID: 9}
			if key == "chat" {
				scope.Capabilities = types.StringArray{string(types.APIKeyCapabilityChat)}
			}
			ctx = types.WithTenantAPIKeyScope(ctx, scope)
			c.Set(types.TenantIDContextKey.String(), uint64(1))
			ctx = context.WithValue(ctx, types.UserIDContextKey, "owner")
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(g.ensureAPIKeyAuthorizer().Middleware())
	api := &craftInputRouteAPI{}
	features := session.NewCraftFeatureRoutes()
	if err := features.Register("input", func(group session.CraftRouteGroup) {
		session.RegisterCraftInputRoutes(group, session.NewCraftInputHandler(api))
	}); err != nil {
		t.Fatal(err)
	}
	RegisterSessionRoutes(r.Group("/api/v1"), nil, nil, g, features)

	upload := func(path, user, key, cross string) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		file, err := form.CreateFormFile("files", "plan.md")
		if err != nil {
			t.Fatal(err)
		}
		content := []byte("input")
		if _, err = file.Write(content); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(content)
		if err = form.WriteField("sha256", hex.EncodeToString(digest[:])); err != nil {
			t.Fatal(err)
		}
		if err = form.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, path, &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		if user != "" {
			req.Header.Set("X-Test-User", user)
		}
		if key != "" {
			req.Header.Set("X-Test-Key", key)
		}
		if cross != "" {
			req.Header.Set("X-Test-Cross-Tenant", cross)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	owner := upload("/api/v1/sessions/task-1/craft/input-rounds", "owner", "", "")
	if owner.Code != http.StatusCreated {
		t.Fatalf("owner upload status = %d: %s", owner.Code, owner.Body.String())
	}
	decision := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/task-1/craft/inputs/decision", bytes.NewBufferString(`{"ref":"opaque://input-1","action":"continue"}`))
	decision.Header.Set("Content-Type", "application/json")
	decision.Header.Set("X-Test-User", "owner")
	decisionResponse := httptest.NewRecorder()
	r.ServeHTTP(decisionResponse, decision)
	if decisionResponse.Code != http.StatusOK {
		t.Fatalf("owner decision status = %d: %s", decisionResponse.Code, decisionResponse.Body.String())
	}
	if len(api.accepted) != 1 || api.decisions != 1 {
		t.Fatalf("service calls: accepted=%d decisions=%d", len(api.accepted), api.decisions)
	}

	if got := upload("/api/v1/sessions/task-1/craft/input-rounds", "", "", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("anonymous upload status = %d, want %d", got, http.StatusUnauthorized)
	}
	if got := upload("/api/v1/sessions/task-1/craft/input-rounds", "", "no-chat", "").Code; got != http.StatusForbidden {
		t.Fatalf("scoped key without chat status = %d, want %d", got, http.StatusForbidden)
	}
	if got := upload("/api/v1/sessions/task-1/craft/input-rounds", "", "chat", "").Code; got != http.StatusCreated {
		t.Fatalf("scoped key with chat status = %d, want %d", got, http.StatusCreated)
	}
	if got := upload("/api/v1/sessions/task-1/craft/input-rounds", "other", "", "yes").Code; got != http.StatusForbidden {
		t.Fatalf("cross-tenant input status = %d, want %d", got, http.StatusForbidden)
	}

	paths := map[string]bool{}
	for _, route := range r.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	if !paths["POST /api/v1/sessions/:session_id/craft/input-rounds"] || !paths["POST /api/v1/sessions/:session_id/craft/inputs/decision"] {
		t.Fatalf("T01 input route set not mounted: %#v", paths)
	}
	for _, path := range []string{
		"/api/v1/sessions/:session_id/craft/input-rounds",
		"/api/v1/sessions/:session_id/craft/inputs/decision",
	} {
		policy := mustLookupAPIKeyPolicy(t, g, http.MethodPost, path)
		if !policy.RequireFullAccess || !policyHasCapability(policy, types.APIKeyCapabilityChat) {
			t.Fatalf("API-key policy for %s = %#v", path, policy)
		}
	}
}
