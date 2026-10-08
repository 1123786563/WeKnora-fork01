package session

// T11 (#128) HTTP seam: the share-consent surface's externally visible
// contract. The feature registers through the constrained registry exactly
// as central assembly mounts it, and these tests assert:
//
//   - the share summary is readable by a Task member and carries ONLY the
//     typed consent facts (version, restricted flag, evidence digest,
//     status) — never a source ref, excerpt or title;
//   - a non-owner consent POST answers a stable 403 with the generic body;
//   - an owner consent binds the CURRENT server-computed evidence: posting
//     a digest of other evidence is a 409 that changes nothing;
//   - a valid approval answers 200 with status consented and the decision
//     binding; a later revocation returns the version to private;
//   - a revoked owner can no longer consent (fresh TaskShare check).
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type t11HTTPVersionStore struct{ byID map[string]craft.Version }

func (s t11HTTPVersionStore) Get(_ context.Context, _ craft.Scope, id string) (craft.Version, error) {
	v, ok := s.byID[id]
	if !ok {
		return craft.Version{}, craft.ErrNotFound
	}
	return v, nil
}
func (s t11HTTPVersionStore) List(context.Context, craft.Scope) ([]craft.Version, error) {
	return nil, nil
}
func (s t11HTTPVersionStore) Publish(_ context.Context, _ craft.Scope, v craft.Version) (craft.Version, error) {
	return v, nil
}

type t11HTTPFiles struct{ objects map[string][]byte }

func (f t11HTTPFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	data, ok := f.objects[ref]
	if !ok {
		return nil, craft.ErrNotFound
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

type t11HTTPRecords struct{ record craft.KnowledgeRecord }

func (r t11HTTPRecords) Load(_ context.Context, _ craft.Scope, _ string) (craft.KnowledgeRecord, error) {
	return r.record, nil
}

type t11HTTPChecker struct{ roles map[string]craft.TaskRole }

func (c *t11HTTPChecker) CheckTaskAccess(_ context.Context, scope craft.Scope, action craft.TaskAction) error {
	role, ok := c.roles[scope.UserID]
	if !ok || !role.AllowsTaskAction(action) {
		return craft.ErrForbidden
	}
	return nil
}

func TestCraftT11ShareHTTPJourney(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:craft107_t11_http?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// The decision row's production migration belongs to central assembly
	// (T20); the seam test mirrors the durable schema it persists into.
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS sessions (id text PRIMARY KEY, tenant_id integer, user_id text)`).Error)
	require.NoError(t, db.Exec(`INSERT OR REPLACE INTO sessions (id, tenant_id, user_id) VALUES ('session-t11', 1, 'owner')`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS sessions (id text PRIMARY KEY, tenant_id integer, user_id text)`).Error)
	require.NoError(t, db.Exec(`INSERT OR REPLACE INTO sessions (id, tenant_id, user_id) VALUES ('session-t11', 1, 'owner')`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS craft_share_decisions (tenant_id integer, session_id text, version_id text, evidence_digest text, owner_id text, decision text, decided_at datetime, revoked_at datetime, created_at datetime, updated_at datetime, PRIMARY KEY (tenant_id, session_id, version_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`).Error)

	shared := craft.KnowledgeSourceRecord{
		ID: "kc_" + strings.Repeat("a", 24), Ref: "craftkb://kb/k-shared/knowledge/k-s/chunk/c-s",
		Digest: "d-shared", TenantID: 7, ExcerptBytes: 32,
	}
	record := craft.KnowledgeRecord{
		Scope:            craft.Scope{TenantID: 1, UserID: "owner", SessionID: "session-t11"},
		RunID:            "run-t11",
		PublicationState: craft.KnowledgePublicationPublished,
		Sources:          []craft.KnowledgeSourceRecord{shared},
	}
	entries := []craft.WebCitationEntry{{Kind: craft.WebCitationFact, CitationID: shared.ID, Claim: "shared fact"}}
	manifest, err := json.Marshal(craft.WebCitationManifest{Schema: craft.WebCitationSchema, Entries: entries})
	require.NoError(t, err)

	files := t11HTTPFiles{objects: map[string][]byte{"obj-v1": manifest}}
	versions := t11HTTPVersionStore{byID: map[string]craft.Version{
		"v-1": {ID: "v-1", WorkspaceID: "ws", RunID: "run-t11", Kind: craft.KindWeb,
			Files: []craft.File{{Path: craft.WebCitationsPath, Ref: "obj-v1", SHA256: "sha", MIME: "application/json", Bytes: 4096}}},
	}}
	checker := &t11HTTPChecker{roles: map[string]craft.TaskRole{
		"owner": craft.TaskRoleOwner, "viewer": craft.TaskRoleViewer,
	}}
	share, err := service.NewCraftShareService(service.CraftShareConfig{
		DB: db, Versions: versions, Files: files, Records: t11HTTPRecords{record: record}, TaskAccess: checker,
		Now: func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)

	features := NewCraftFeatureRoutes()
	require.NoError(t, RegisterCraftShareFeature(features, share))
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
	post := func(user, path string, body map[string]string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(raw)))
		req.Header.Set("X-Test-Auth", "yes")
		req.Header.Set("X-Test-User", user)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	view := "/sessions/session-t11/craft/versions/v-1/share"
	decide := view + "/decision"
	revocation := view + "/revocation"

	type shareBody struct {
		VersionID      string `json:"version_id"`
		Restricted     bool   `json:"restricted"`
		EvidenceDigest string `json:"evidence_digest"`
		Status         string `json:"status"`
		Decision       *struct {
			VersionID      string `json:"version_id"`
			EvidenceDigest string `json:"evidence_digest"`
			OwnerID        string `json:"owner_id"`
			Decision       string `json:"decision"`
		} `json:"decision"`
	}

	// --- a member reads the consent summary: restricted, private, and only
	// typed consent facts in the body.
	summary := get("viewer", view)
	require.Equal(t, http.StatusOK, summary.Code)
	var initial shareBody
	require.NoError(t, json.Unmarshal(summary.Body.Bytes(), &initial))
	require.True(t, initial.Restricted)
	require.Equal(t, "v-1", initial.VersionID)
	require.NotEmpty(t, initial.EvidenceDigest)
	require.Equal(t, "private", initial.Status)
	require.Nil(t, initial.Decision)
	require.NotContains(t, summary.Body.String(), "craftkb://", "no source ref in the share body")
	require.NotContains(t, summary.Body.String(), "excerpt", "no excerpt in the share body")

	// --- a non-owner consent POST is a stable 403.
	denied := post("viewer", decide, map[string]string{"decision": "approved", "evidence_digest": initial.EvidenceDigest})
	require.Equal(t, http.StatusForbidden, denied.Code)
	require.JSONEq(t, `{"error":"Forbidden"}`, denied.Body.String())

	// --- the owner posting a digest of other evidence is a 409 that leaves
	// the version private.
	stale := post("owner", decide, map[string]string{"decision": "approved", "evidence_digest": strings.Repeat("0", 64)})
	require.Equal(t, http.StatusConflict, stale.Code)
	still := get("owner", view)
	var stillBody shareBody
	require.NoError(t, json.Unmarshal(still.Body.Bytes(), &stillBody))
	require.Equal(t, "private", stillBody.Status)

	// --- the valid approval binds version and digest; status consented.
	approved := post("owner", decide, map[string]string{"decision": "approved", "evidence_digest": initial.EvidenceDigest})
	require.Equal(t, http.StatusOK, approved.Code)
	var consented shareBody
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &consented))
	require.Equal(t, "consented", consented.Status)
	require.NotNil(t, consented.Decision)
	require.Equal(t, "v-1", consented.Decision.VersionID)
	require.Equal(t, initial.EvidenceDigest, consented.Decision.EvidenceDigest)
	require.Equal(t, "owner", consented.Decision.OwnerID)
	require.Equal(t, "approved", consented.Decision.Decision)

	// --- the viewer reads the same live state and still no source material.
	viewerSees := get("viewer", view)
	require.Equal(t, http.StatusOK, viewerSees.Code)
	require.Contains(t, viewerSees.Body.String(), `"status":"consented"`)
	require.NotContains(t, viewerSees.Body.String(), "craftkb://")

	// --- revocation by the owner returns the version to private.
	revoked := post("owner", revocation, nil)
	require.Equal(t, http.StatusOK, revoked.Code)
	var afterRevoke shareBody
	require.NoError(t, json.Unmarshal(revoked.Body.Bytes(), &afterRevoke))
	require.Equal(t, "private", afterRevoke.Status)

	// --- a revoked owner can no longer consent (fresh TaskShare check).
	delete(checker.roles, "owner")
	final := post("owner", decide, map[string]string{"decision": "approved", "evidence_digest": initial.EvidenceDigest})
	require.Equal(t, http.StatusForbidden, final.Code)
	require.JSONEq(t, `{"error":"Forbidden"}`, final.Body.String())

	// --- a missing version answers a stable 404.
	missing := get("viewer", "/sessions/session-t11/craft/versions/v-none/share")
	require.Equal(t, http.StatusNotFound, missing.Code)
}

// TestCraftT11ShareDecisionBodyGuards pins the T11 OCR fix: the consent
// decision body goes through the shared strict decoder like every craft
// body. Injected authority fields and trailing data answer 400, an
// oversized body answers 413 before any consent work, and a valid minimal
// decision still records.
func TestCraftT11ShareDecisionBodyGuards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:craft107_t11_guards?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS sessions (id text PRIMARY KEY, tenant_id integer, user_id text)`).Error)
	require.NoError(t, db.Exec(`INSERT OR REPLACE INTO sessions (id, tenant_id, user_id) VALUES ('session-t11', 1, 'owner')`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS craft_share_decisions (tenant_id integer, session_id text, version_id text, evidence_digest text, owner_id text, decision text, decided_at datetime, revoked_at datetime, created_at datetime, updated_at datetime, PRIMARY KEY (tenant_id, session_id, version_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`).Error)

	shared := craft.KnowledgeSourceRecord{
		ID: "kc_" + strings.Repeat("a", 24), Ref: "craftkb://kb/k-shared/knowledge/k-s/chunk/c-s",
		Digest: "d-shared", TenantID: 7, ExcerptBytes: 32,
	}
	record := craft.KnowledgeRecord{
		Scope:            craft.Scope{TenantID: 1, UserID: "owner", SessionID: "session-t11"},
		RunID:            "run-t11",
		PublicationState: craft.KnowledgePublicationPublished,
		Sources:          []craft.KnowledgeSourceRecord{shared},
	}
	manifest, err := json.Marshal(craft.WebCitationManifest{Schema: craft.WebCitationSchema, Entries: []craft.WebCitationEntry{{Kind: craft.WebCitationFact, CitationID: shared.ID, Claim: "shared fact"}}})
	require.NoError(t, err)
	files := t11HTTPFiles{objects: map[string][]byte{"obj-v1": manifest}}
	versions := t11HTTPVersionStore{byID: map[string]craft.Version{
		"v-1": {ID: "v-1", WorkspaceID: "ws", RunID: "run-t11", Kind: craft.KindWeb,
			Files: []craft.File{{Path: craft.WebCitationsPath, Ref: "obj-v1", SHA256: "sha", MIME: "application/json", Bytes: 4096}}},
	}}
	checker := &t11HTTPChecker{roles: map[string]craft.TaskRole{"owner": craft.TaskRoleOwner, "viewer": craft.TaskRoleViewer}}
	share, err := service.NewCraftShareService(service.CraftShareConfig{
		DB: db, Versions: versions, Files: files, Records: t11HTTPRecords{record: record}, TaskAccess: checker,
		Now: func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)

	features := NewCraftFeatureRoutes()
	require.NoError(t, RegisterCraftShareFeature(features, share))
	router := gin.New()
	// The production error handler translates the strict decoder's
	// c.Error rejections into 400 responses.
	router.Use(middleware.ErrorHandler())
	group := router.Group("/sessions", func(c *gin.Context) {
		if c.GetHeader("X-Test-Auth") != "yes" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, c.GetHeader("X-Test-User"))
		c.Request = c.Request.WithContext(ctx)
	})
	require.NoError(t, features.Mount(group))
	do := func(method, user, path, rawBody string) *httptest.ResponseRecorder {
		var reader io.Reader
		if rawBody != "" {
			reader = strings.NewReader(rawBody)
		}
		req := httptest.NewRequest(method, path, reader)
		req.Header.Set("X-Test-Auth", "yes")
		req.Header.Set("X-Test-User", user)
		if rawBody != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	// The current evidence digest comes from the share summary itself.
	summary := do(http.MethodGet, "owner", "/sessions/session-t11/craft/versions/v-1/share", "")
	require.Equal(t, http.StatusOK, summary.Code)
	var initial struct {
		EvidenceDigest string `json:"evidence_digest"`
	}
	require.NoError(t, json.Unmarshal(summary.Body.Bytes(), &initial))
	require.NotEmpty(t, initial.EvidenceDigest)

	decide := "/sessions/session-t11/craft/versions/v-1/share/decision"

	// An injected authority field riding the consent body is rejected.
	stolen := do(http.MethodPost, "owner", decide, fmt.Sprintf(`{"decision":"approved","evidence_digest":%q,"tenant_id":2}`, initial.EvidenceDigest))
	require.Equal(t, http.StatusBadRequest, stolen.Code, "an unknown field never reaches the consent path: %s", stolen.Body.String())

	// Trailing data after the decision object is rejected.
	trailing := do(http.MethodPost, "owner", decide, fmt.Sprintf(`{"decision":"approved","evidence_digest":%q}{}`, initial.EvidenceDigest))
	require.Equal(t, http.StatusBadRequest, trailing.Code, "trailing data is rejected: %s", trailing.Body.String())

	// A body over the craft ceiling answers 413 before any consent work.
	huge := do(http.MethodPost, "owner", decide, `{"decision":"approved","evidence_digest":"`+strings.Repeat("a", service.MaxCraftRequestBodyBytes)+`"}`)
	require.Equal(t, http.StatusRequestEntityTooLarge, huge.Code, "the oversized body is refused: %s", huge.Body.String())

	// A valid minimal decision still records and consents.
	valid := do(http.MethodPost, "owner", decide, fmt.Sprintf(`{"decision":"approved","evidence_digest":%q}`, initial.EvidenceDigest))
	require.Equal(t, http.StatusOK, valid.Code, "the strict decoder did not break the happy path: %s", valid.Body.String())
	var consented struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(valid.Body.Bytes(), &consented))
	require.Equal(t, "consented", consented.Status)
}
