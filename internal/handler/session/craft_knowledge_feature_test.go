package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type t05HTTPStore struct {
	craft.Store
	ws craft.Workspace
}

func (s t05HTTPStore) GetWorkspace(context.Context, craft.Scope) (craft.Workspace, error) {
	return s.ws, nil
}

type t05HTTPChecker struct{ allowed map[string]bool }

func (c *t05HTTPChecker) CheckTaskAccess(_ context.Context, scope craft.Scope, _ craft.TaskAction) error {
	if !c.allowed[scope.UserID] {
		return craft.ErrForbidden
	}
	return nil
}

type t05HTTPPublisher struct {
	candidates map[string]service.CraftKnowledgeMaterialPackage
	visible    map[string]service.CraftKnowledgeMaterialPackage
}

func (p *t05HTTPPublisher) Prepare(_ context.Context, _ craft.Workspace, pkg service.CraftKnowledgeMaterialPackage) error {
	if p.candidates == nil {
		p.candidates = map[string]service.CraftKnowledgeMaterialPackage{}
	}
	if p.visible == nil {
		p.visible = map[string]service.CraftKnowledgeMaterialPackage{}
	}
	p.candidates[pkg.RunID] = pkg
	return nil
}

func (p *t05HTTPPublisher) Publish(_ context.Context, _ craft.Workspace, pkg service.CraftKnowledgeMaterialPackage) error {
	candidate, ok := p.candidates[pkg.RunID]
	if !ok || candidate.Digest != pkg.Digest {
		return craft.ErrConflict
	}
	p.visible[pkg.RunID] = candidate
	delete(p.candidates, pkg.RunID)
	return nil
}

func (p *t05HTTPPublisher) Discard(_ context.Context, _ craft.Workspace, pkg service.CraftKnowledgeMaterialPackage) error {
	delete(p.candidates, pkg.RunID)
	return nil
}

func TestCraftT05SourceHTTPJourney(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:craft107_t05_http?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.CraftKnowledgeRecordRow{}))
	records := repository.NewCraftKnowledgeRecordRepository(db)
	owner := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "session-a"}
	checker := &t05HTTPChecker{allowed: map[string]bool{"owner": true, "viewer": true}}
	resourceAllowed := map[string]bool{"owner": true, "viewer": false}
	access := func(ctx context.Context, _ uint64, ids []string) ([]*types.Knowledge, error) {
		caller := types.CallerFromContext(ctx)
		if !resourceAllowed[caller.UserID] {
			return nil, nil
		}
		if len(ids) == 1 && ids[0] == "k-a" {
			return []*types.Knowledge{{ID: "k-a", KnowledgeBaseID: "kb-a", TenantID: 1, Title: "A"}}, nil
		}
		return nil, nil
	}
	svc, err := service.NewCraftKnowledgeService(service.CraftKnowledgeConfig{
		Store: t05HTTPStore{ws: craft.Workspace{ID: "ws-a", Scope: owner}}, Access: access,
		Search: func(_ context.Context, _ string, _ types.SearchParams) ([]*types.SearchResult, error) {
			return []*types.SearchResult{{ID: "c-a", KnowledgeID: "k-a", KnowledgeBaseID: "kb-a", Content: "selected source"}, {ID: "c-b", KnowledgeID: "k-b", KnowledgeBaseID: "kb-b", Content: "unselected source"}}, nil
		},
		Writer: func(context.Context, craft.Workspace, string, []byte) error { return nil }, TaskAccess: checker, Records: records,
		Publisher: &t05HTTPPublisher{},
		Now:       func() time.Time { return time.Date(2026, 9, 23, 1, 2, 3, 0, time.UTC) },
	})
	require.NoError(t, err)
	ownerCtx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "owner")
	bundle, err := svc.BuildForRun(ownerCtx, owner, "run-a", "sales", []string{"k-a"})
	require.NoError(t, err)
	require.Len(t, bundle.Sources, 1)
	require.Equal(t, "selected source", bundle.Sources[0].Excerpt)
	features := NewCraftFeatureRoutes()
	require.NoError(t, features.Register("knowledge", NewCraftKnowledgeHandler(svc).MountCraftKnowledgeRoutes))
	router := gin.New()
	group := router.Group("/sessions", func(c *gin.Context) {
		if c.GetHeader("X-Test-Auth") != "yes" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		user := c.GetHeader("X-Test-User")
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		c.Request = c.Request.WithContext(ctx)
	})
	require.NoError(t, features.Mount(group))
	get := func(user, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Test-Auth", "yes")
		req.Header.Set("X-Test-User", user)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	base := "/sessions/session-a/craft/runs/run-a/sources"
	ownerList := get("owner", base)
	require.Equal(t, http.StatusOK, ownerList.Code)
	var payload struct {
		Sources []map[string]any `json:"sources"`
		Empty   bool             `json:"empty"`
	}
	require.NoError(t, json.Unmarshal(ownerList.Body.Bytes(), &payload))
	require.Len(t, payload.Sources, 1)
	require.Equal(t, bundle.Sources[0].Ref, payload.Sources[0]["ref"])
	require.NotContains(t, ownerList.Body.String(), "unselected source")
	// A task Viewer can see the historical source fact, but cannot open the original.
	require.Equal(t, http.StatusOK, get("viewer", base).Code)
	open := base + "/" + bundle.Sources[0].ID + "/open"
	require.Equal(t, http.StatusForbidden, get("viewer", open).Code)
	require.Equal(t, http.StatusOK, get("owner", open).Code)
	resourceAllowed["owner"] = false
	require.Equal(t, http.StatusForbidden, get("owner", open).Code, "resource revocation is checked at every open")
	checker.allowed["viewer"] = false
	require.Equal(t, http.StatusForbidden, get("viewer", base).Code, "Task revocation is checked at every read")
	require.Equal(t, http.StatusUnauthorized, func() int {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, base, nil))
		return w.Code
	}())
}
