package session

// T10 (#127) HTTP seam: the per-open source route's externally visible
// contract. The route is registered through the constrained feature registry
// by central assembly; these tests mount it exactly that way and assert:
//
//   - a Task Viewer whose own knowledge permission covers the source opens it
//     (200) and the body carries ONLY the durable ref — no title, no excerpt,
//     no provider/storage URL;
//   - the same Viewer without that grant gets a stable 403 whose body is the
//     generic Forbidden text, while the citation row (placeholder + digest)
//     stays visible in the same response stream;
//   - a missing citation and a missing Run answer the SAME stable 404 body
//     (non-leaking), and a tampered URL-shaped recorded ref also denies 404;
//   - revoking the Task grant fails the next open with the same 403.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

type t10RoleHTTPChecker struct{ roles map[string]craft.TaskRole }

func (c *t10RoleHTTPChecker) CheckTaskAccess(_ context.Context, scope craft.Scope, action craft.TaskAction) error {
	role, ok := c.roles[scope.UserID]
	if !ok || !role.AllowsTaskAction(action) {
		return craft.ErrForbidden
	}
	return nil
}

func TestCraftT10SourceOpenHTTPJourney(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:craft107_t10_http?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.CraftKnowledgeRecordRow{}))
	records := repository.NewCraftKnowledgeRecordRepository(db)

	owner := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "session-t10"}
	resourceAllowed := map[string]bool{"owner": true, "viewer": false}
	access := func(ctx context.Context, _ uint64, ids []string) ([]*types.Knowledge, error) {
		caller := types.CallerFromContext(ctx)
		if !resourceAllowed[caller.UserID] {
			return nil, nil
		}
		var rows []*types.Knowledge
		for _, id := range ids {
			if id == "k-a" {
				rows = append(rows, &types.Knowledge{ID: "k-a", KnowledgeBaseID: "kb-a", TenantID: 1, Title: "Region Sales"})
			}
		}
		return rows, nil
	}
	checker := &t10RoleHTTPChecker{roles: map[string]craft.TaskRole{
		"owner": craft.TaskRoleOwner, "viewer": craft.TaskRoleViewer,
	}}
	svc, err := service.NewCraftKnowledgeService(service.CraftKnowledgeConfig{
		Store: t05HTTPStore{ws: craft.Workspace{ID: "ws-t10", Scope: owner}}, Access: access,
		Search: func(_ context.Context, _ string, _ types.SearchParams) ([]*types.SearchResult, error) {
			return []*types.SearchResult{{ID: "c-a", KnowledgeID: "k-a", KnowledgeBaseID: "kb-a", Content: "selected source"}}, nil
		},
		Writer: func(context.Context, craft.Workspace, string, []byte) error { return nil }, TaskAccess: checker, Records: records,
		Publisher: &t05HTTPPublisher{},
		Now:       func() time.Time { return time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)
	ownerCtx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "owner")
	bundle, err := svc.BuildForRun(ownerCtx, owner, "run-a", "sales", []string{"k-a"})
	require.NoError(t, err)
	require.Len(t, bundle.Sources, 1)

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
	list := "/sessions/session-t10/craft/runs/run-a/sources"
	open := list + "/" + bundle.Sources[0].ID + "/open"

	// --- the Viewer sees the citation row but the open denies (stable 403).
	listed := get("viewer", list)
	require.Equal(t, http.StatusOK, listed.Code)
	var payload struct {
		Sources []map[string]any `json:"sources"`
	}
	require.NoError(t, json.Unmarshal(listed.Body.Bytes(), &payload))
	require.Len(t, payload.Sources, 1)
	require.Equal(t, bundle.Sources[0].Digest, payload.Sources[0]["digest"], "the citation placeholder and integrity stay visible while the open is denied")

	denied := get("viewer", open)
	require.Equal(t, http.StatusForbidden, denied.Code)
	require.JSONEq(t, `{"error":"Forbidden"}`, denied.Body.String(), "the denial body is stable and carries no reason")

	// --- granting the Viewer their own permission makes the next open 200
	// with ONLY the durable ref in the body.
	resourceAllowed["viewer"] = true
	allowed := get("viewer", open)
	require.Equal(t, http.StatusOK, allowed.Code)
	var opened struct {
		Ref string `json:"ref"`
	}
	require.NoError(t, json.Unmarshal(allowed.Body.Bytes(), &opened))
	require.Equal(t, bundle.Sources[0].Ref, opened.Ref)
	require.NotContains(t, allowed.Body.String(), "http", "no provider/storage URL is exposed")
	require.NotContains(t, allowed.Body.String(), "Region Sales", "the open result never leaks the document title")
	require.NotContains(t, allowed.Body.String(), "selected source", "the open result never leaks the excerpt")

	// --- revoking the knowledge grant fails the next open; the citation row
	// is still listed (placeholder remains).
	resourceAllowed["viewer"] = false
	require.Equal(t, http.StatusForbidden, get("viewer", open).Code)
	require.Equal(t, http.StatusOK, get("viewer", list).Code)

	// --- revoking the Task grant fails the next open as the same 403.
	delete(checker.roles, "viewer")
	require.Equal(t, http.StatusForbidden, get("viewer", open).Code)
	checker.roles["viewer"] = craft.TaskRoleViewer

	// --- missing citation and missing Run answer the SAME 404 body; a
	// tampered URL-shaped recorded ref denies 404 too. All denials are
	// byte-identical: nothing distinguishes WHY.
	missingCitation := get("viewer", list+"/kc_0000000000000000000000ff/open")
	require.Equal(t, http.StatusNotFound, missingCitation.Code)
	missingRun := get("viewer", "/sessions/session-t10/craft/runs/run-none/sources/kc_x/open")
	require.Equal(t, http.StatusNotFound, missingRun.Code)
	require.Equal(t, missingCitation.Body.String(), missingRun.Body.String(), "missing outcomes are stable and non-leaking")

	_, err = svc.BuildForRun(ownerCtx, owner, "run-tamper", "sales", []string{"k-a"})
	require.NoError(t, err)
	tampered, err := records.Load(ownerCtx, owner, "run-tamper")
	require.NoError(t, err)
	tamperedRef := "https://storage.internal.example/bucket/k-a?signature=reusable"
	require.NoError(t, db.Exec(
		"UPDATE craft_knowledge_records SET record_json = replace(record_json, ?, ?) WHERE run_id = ?",
		tampered.Sources[0].Ref, tamperedRef, "run-tamper",
	).Error)
	// keep the row's integrity digest consistent with the tampered JSON so the
	// repository's digest check cannot be the reason for the denial.
	var rowJSON string
	require.NoError(t, db.Raw("SELECT record_json FROM craft_knowledge_records WHERE run_id = ?", "run-tamper").Scan(&rowJSON).Error)
	require.NoError(t, db.Exec("UPDATE craft_knowledge_records SET digest = ? WHERE run_id = ?",
		sha256Hex(rowJSON), "run-tamper").Error)
	tamperedOpen := get("owner", "/sessions/session-t10/craft/runs/run-tamper/sources/"+tampered.Sources[0].ID+"/open")
	require.Equal(t, http.StatusNotFound, tamperedOpen.Code, "a URL-shaped recorded ref denies and never becomes an openable URL")
	require.Equal(t, missingCitation.Body.String(), tamperedOpen.Body.String())
}
