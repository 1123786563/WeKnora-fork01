package session

// T13 (#133) HTTP seam: the restricted derived-export consent's externally
// visible contract. The consent feature and the consent-gated export
// feature register through the constrained registry exactly as central
// assembly mounts them, and these tests assert:
//
//   - a Task member reads the consent view: the exact files with their
//     recorded origins, the restricted derived classification, the manifest
//     digest and the current state — no original knowledge material;
//   - without a live owner consent the download streams the SAFE bundle
//     only: the three fixed documents, zero restricted derived members;
//   - the owner's approved decision (bound to the manifest digest they saw)
//     unlocks the full bundle whose manifest digest equals the described
//     one; an explicit rejection returns to the safe bundle;
//   - a collaborator's decision is a stable 403 and is audited; a replayed
//     digest is a 409; the decision body goes through the strict decoder;
//   - a non-member sees the stable non-leaking 403/404 vocabulary.
import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
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

func TestCraftT13ExportConsentHTTPJourney(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:craft107_t13_http?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE sessions (id text primary key, tenant_id integer, user_id text)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, user_id) VALUES ('session-t13', 1, 'owner')`).Error)
	// The durable decision table the service persists into (the production
	// migration central assembly owns mirrors exactly this column shape).
	require.NoError(t, db.Exec(`CREATE TABLE craft_export_decisions (
		tenant_id integer NOT NULL,
		session_id varchar(128) NOT NULL,
		version_id varchar(128) NOT NULL,
		manifest_digest char(64) NOT NULL,
		owner_id varchar(512) NOT NULL,
		decision varchar(16) NOT NULL,
		decided_at datetime NOT NULL,
		created_at datetime,
		updated_at datetime,
		PRIMARY KEY (tenant_id, session_id, version_id)
	)`).Error)

	// --- the world: one version whose members derive from one own-tenant
	// and one cross-tenant (restricted) recorded original.
	acquired := time.Date(2026, 9, 26, 1, 2, 3, 0, time.UTC)
	versionID := "ver_" + strings.Repeat("3", 64)
	evidence := craft.VersionEvidence{
		VersionID: versionID, RunID: "run-t13",
		RequestDigest: sha256Hex("req"), PackageDigest: sha256Hex("pkg"),
		AcquiredAt: acquired, PinnedAt: acquired.Add(time.Hour),
		Sources: []craft.KnowledgeSourceRecord{
			{ID: "kc_" + strings.Repeat("a", 24), Ref: craft.KnowledgeRef("kb-own", "k-own", "c-own"),
				Digest: sha256Hex("own"), TenantID: 1, AcquiredAt: acquired, ExcerptBytes: 32},
			{ID: "kc_" + strings.Repeat("b", 24), Ref: craft.KnowledgeRef("kb-shared", "k-shared", "c-shared"),
				Digest: sha256Hex("shared"), TenantID: 7, AcquiredAt: acquired.Add(time.Minute), ExcerptBytes: 32},
		},
	}
	version := craft.Version{
		ID: versionID, WorkspaceID: "ws-t13", RunID: "run-t13", Kind: craft.KindWeb,
		Files: []craft.File{
			{Path: "index.html", Ref: "obj-index", SHA256: sha256Hex("<h1>t13</h1>"), MIME: "text/html", Bytes: 12},
			{Path: "citations.json", Ref: "obj-cit", SHA256: sha256Hex(`{"schema":1}`), MIME: "application/json", Bytes: 13},
		},
		Checks: []craft.Check{{Name: craft.CheckBuild, Status: craft.CheckPassed, Detail: "build exited 0"}},
	}
	versions := &t12VersionReader{version: version, evidence: evidence}
	files := t12Files{objects: map[string][]byte{
		"obj-index": []byte("<h1>t13</h1>"),
		"obj-cit":   []byte(`{"schema":1}`),
	}}
	checker := &t10RoleHTTPChecker{roles: map[string]craft.TaskRole{
		"owner": craft.TaskRoleOwner, "collab": craft.TaskRoleCollaborator, "viewer": craft.TaskRoleViewer,
	}}
	now := func() time.Time { return time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC) }
	exportSvc, err := service.NewCraftExportService(service.CraftExportConfig{
		DB: db, Versions: versions, Evidence: versions, TaskAccess: checker, Now: now,
	})
	require.NoError(t, err)
	consentSvc, err := service.NewCraftExportConsentService(service.CraftExportConsentConfig{
		DB: db, Versions: versions, Evidence: versions, TaskAccess: checker, Now: now,
	})
	require.NoError(t, err)
	gated, err := service.NewConsentGatedExportService(exportSvc, consentSvc)
	require.NoError(t, err)

	features := NewCraftFeatureRoutes()
	// The gated wrapper stands in for the raw T12 service: the download
	// surface stays T12's, the consent gate decides what it may stream.
	require.NoError(t, RegisterCraftExportFeature(features, gated, files))
	require.NoError(t, RegisterCraftExportConsentFeature(features, consentSvc))
	router := gin.New()
	// The production error handler translates the strict decoder's
	// c.Error rejections into 400 responses.
	router.Use(middleware.ErrorHandler())
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
	post := func(user, path string, payload any) *httptest.ResponseRecorder {
		var body io.Reader
		if raw, ok := payload.(string); ok {
			body = strings.NewReader(raw)
		} else if payload != nil {
			encoded, merr := json.Marshal(payload)
			require.NoError(t, merr)
			body = bytes.NewReader(encoded)
		}
		req := httptest.NewRequest(http.MethodPost, path, body)
		req.Header.Set("X-Test-Auth", "yes")
		req.Header.Set("X-Test-User", user)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	zipEntries := func(w *httptest.ResponseRecorder) map[string]string {
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		reader, zerr := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
		require.NoError(t, zerr)
		out := map[string]string{}
		for _, f := range reader.File {
			rc, oerr := f.Open()
			require.NoError(t, oerr)
			raw, rerr := io.ReadAll(rc)
			rc.Close()
			require.NoError(t, rerr)
			out[f.Name] = string(raw)
		}
		return out
	}
	consentPath := "/sessions/session-t13/craft/versions/" + versionID + "/export/consent"
	decisionPath := consentPath + "/decision"
	downloadPath := "/sessions/session-t13/craft/versions/" + versionID + "/export/download"

	// --- a member reads the consent view: exact files, origins,
	// classification, digest and state.
	view := get("viewer", consentPath)
	require.Equal(t, http.StatusOK, view.Code, view.Body.String())
	var consentBody struct {
		VersionID         string   `json:"version_id"`
		ManifestDigest    string   `json:"manifest_digest"`
		State             string   `json:"state"`
		RestrictedDerived []string `json:"restricted_derived"`
		Files             []struct {
			Path    string `json:"path"`
			SHA256  string `json:"sha256"`
			Origins []struct {
				Kind       string `json:"kind"`
				Ref        string `json:"ref"`
				SHA256     string `json:"sha256"`
				Restricted bool   `json:"restricted"`
			} `json:"origins"`
		} `json:"files"`
		Decision *struct {
			OwnerID  string `json:"owner_id"`
			Decision string `json:"decision"`
		} `json:"decision"`
	}
	require.NoError(t, json.Unmarshal(view.Body.Bytes(), &consentBody))
	require.Equal(t, versionID, consentBody.VersionID)
	require.Equal(t, "awaiting", consentBody.State)
	require.Nil(t, consentBody.Decision)
	require.ElementsMatch(t, []string{"index.html", "citations.json"}, consentBody.RestrictedDerived)
	require.Len(t, consentBody.Files, 2)
	for _, f := range consentBody.Files {
		require.Len(t, f.Origins, 2, "every member carries its recorded origins")
		for _, origin := range f.Origins {
			require.NotContains(t, origin.Ref, "http", "origins stay opaque authenticated refs")
		}
	}

	// --- without consent the download is the SAFE bundle: three fixed
	// documents, no restricted derived member byte.
	safe := zipEntries(get("viewer", downloadPath))
	require.Len(t, safe, 3, "exactly the three fixed bundle documents")
	for _, doc := range []string{"export-manifest.json", "sources.json", "build.json"} {
		require.Contains(t, safe, doc)
	}
	var safeManifest craft.ExportManifest
	require.NoError(t, json.Unmarshal([]byte(safe["export-manifest.json"]), &safeManifest))
	require.Empty(t, safeManifest.Files, "no restricted derived member rides the safe bundle")
	require.NotEqual(t, consentBody.ManifestDigest, safeManifest.ManifestDigest)
	require.NoError(t, safeManifest.Validate())

	// --- a collaborator's decision is a stable audited refusal.
	refused := post("collab", decisionPath, map[string]string{"decision": "approved", "manifest_digest": consentBody.ManifestDigest})
	require.Equal(t, http.StatusForbidden, refused.Code, refused.Body.String())
	require.Contains(t, refused.Body.String(), http.StatusText(http.StatusForbidden), "the refusal never says why")
	var deniedRows int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM audit_logs WHERE action = 'craft.export_denied:task_access' AND outcome = 'denied'`).Scan(&deniedRows).Error)
	require.Equal(t, int64(1), deniedRows, "the proven refusal is audited")

	// --- a replayed digest is a proven conflict.
	replayed := post("owner", decisionPath, map[string]string{"decision": "approved", "manifest_digest": strings.Repeat("f", 64)})
	require.Equal(t, http.StatusConflict, replayed.Code, replayed.Body.String())

	// --- the strict decoder guards the decision body.
	badField := post("owner", decisionPath, `{"decision":"approved","manifest_digest":"x","tenant_id":7}`)
	require.Equal(t, http.StatusBadRequest, badField.Code)
	trailing := post("owner", decisionPath, `{"decision":"approved","manifest_digest":"x"} trailing`)
	require.Equal(t, http.StatusBadRequest, trailing.Code)

	// --- the owner's approval binds the digest they saw and unlocks the
	// full bundle.
	approved := post("owner", decisionPath, map[string]string{"decision": "approved", "manifest_digest": consentBody.ManifestDigest})
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())
	var approvedBody struct {
		State    string `json:"state"`
		Decision *struct {
			OwnerID        string `json:"owner_id"`
			ManifestDigest string `json:"manifest_digest"`
			Decision       string `json:"decision"`
		} `json:"decision"`
	}
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &approvedBody))
	require.Equal(t, "consented", approvedBody.State)
	require.NotNil(t, approvedBody.Decision)
	require.Equal(t, "owner", approvedBody.Decision.OwnerID)
	require.Equal(t, consentBody.ManifestDigest, approvedBody.Decision.ManifestDigest)

	full := zipEntries(get("owner", downloadPath))
	require.Len(t, full, 5, "the three fixed documents plus both derived members")
	require.Equal(t, "<h1>t13</h1>", full["index.html"])
	require.Equal(t, `{"schema":1}`, full["citations.json"])
	var fullManifest craft.ExportManifest
	require.NoError(t, json.Unmarshal([]byte(full["export-manifest.json"]), &fullManifest))
	require.Equal(t, consentBody.ManifestDigest, fullManifest.ManifestDigest, "the consented bundle carries the exact bound digest")
	require.Len(t, fullManifest.Files, 2)
	// No original knowledge bytes ever ride the bundle: the cited original
	// appears only as the authenticated ref inside the citation manifest.
	require.NotContains(t, full["sources.json"], "own excerpt")
	require.NotContains(t, full["sources.json"], "shared excerpt")

	// --- the explicit rejection returns to the safe bundle.
	rejected := post("owner", decisionPath, map[string]string{"decision": "rejected", "manifest_digest": consentBody.ManifestDigest})
	require.Equal(t, http.StatusOK, rejected.Code, rejected.Body.String())
	require.Equal(t, "declined", jsonGet(t, rejected.Body.String(), "state"))
	safeAgain := zipEntries(get("owner", downloadPath))
	require.Len(t, safeAgain, 3, "a rejection yields the safe bundle without restricted derived files")

	// --- a non-member sees the stable non-leaking vocabulary.
	strangerView := get("stranger", consentPath)
	require.Equal(t, http.StatusForbidden, strangerView.Code)
	strangerDownload := get("stranger", downloadPath)
	require.Equal(t, http.StatusForbidden, strangerDownload.Code)
	missing := get("owner", "/sessions/session-t13/craft/versions/ver_"+strings.Repeat("9", 64)+"/export/consent")
	require.Equal(t, http.StatusNotFound, missing.Code)
}

// jsonGet extracts one top-level string field from a JSON body.
func jsonGet(t *testing.T, body, field string) string {
	t.Helper()
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &parsed))
	value, ok := parsed[field].(string)
	require.True(t, ok, "field %s missing in %s", field, body)
	return value
}
