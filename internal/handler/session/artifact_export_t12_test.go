package session

// T12 (#132) HTTP seam: the version-bound source bundle's externally
// visible contract. The feature registers through the constrained registry
// exactly as central assembly mounts it, and these tests assert:
//
//   - a Task member reads the bundle description (manifest digest, member
//     list, citation/source manifest, build checks) and downloads a zip
//     whose members are exactly the version's immutable files plus the
//     three fixed bundle documents — no traversal entry, no original
//     knowledge bytes;
//   - the zip's export-manifest.json digest equals the described digest;
//   - a non-member download is a stable 403 whose body never says why, and
//     the refusal is audited;
//   - traversal-shaped version ids answer the same stable 404 as a missing
//     version (non-leaking);
//   - the manifest's authenticated source reference stays authenticated:
//     holding the downloaded manifest grants no original access — the
//     viewer without knowledge permission is refused by the T10 open route.
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

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// t12VersionReader serves the version and its pinned evidence from one row.
type t12VersionReader struct {
	version  craft.Version
	evidence craft.VersionEvidence
}

func (s *t12VersionReader) Get(_ context.Context, _ craft.Scope, id string) (craft.Version, error) {
	if s.version.ID == id {
		return s.version, nil
	}
	return craft.Version{}, craft.ErrNotFound
}
func (s *t12VersionReader) List(context.Context, craft.Scope) ([]craft.Version, error) {
	return nil, nil
}
func (s *t12VersionReader) Publish(_ context.Context, _ craft.Scope, v craft.Version) (craft.Version, error) {
	return v, nil
}
func (s *t12VersionReader) PublishWithEvidence(_ context.Context, _ craft.Scope, v craft.Version, ev craft.VersionEvidence) (craft.Version, error) {
	return v, nil
}
func (s *t12VersionReader) VersionEvidence(_ context.Context, scope craft.Scope, versionID string) (craft.VersionEvidence, error) {
	if s.evidence.VersionID != versionID || scope.TenantID == 0 {
		return craft.VersionEvidence{}, craft.ErrNotFound
	}
	return s.evidence, nil
}

type t12Files struct{ objects map[string][]byte }

func (f t12Files) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	data, ok := f.objects[ref]
	if !ok {
		return nil, craft.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func TestCraftT12ExportDownloadHTTPJourney(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:craft107_t12_http?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.CraftKnowledgeRecordRow{}))
	require.NoError(t, db.Exec(`CREATE TABLE audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`).Error)

	// --- the world: tenant 1 task with owner+viewer; one own-tenant and one
	// cross-tenant (tenant 7) recorded original; the viewer holds no
	// knowledge permission of their own.
	owner := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "session-t12"}
	acquired := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)
	evidence := craft.VersionEvidence{
		VersionID: "ver_" + strings.Repeat("3", 64), RunID: "run-t12",
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
		ID: evidence.VersionID, WorkspaceID: "ws-t12", RunID: "run-t12", Kind: craft.KindWeb,
		Files: []craft.File{
			{Path: "index.html", Ref: "obj-index", SHA256: sha256Hex("<h1>t12</h1>"), MIME: "text/html", Bytes: 12},
			{Path: "citations.json", Ref: "obj-cit", SHA256: sha256Hex(`{"schema":1}`), MIME: "application/json", Bytes: 13},
		},
		Checks: []craft.Check{{Name: craft.CheckBuild, Status: craft.CheckPassed, Detail: "build exited 0"}},
	}
	versions := &t12VersionReader{version: version, evidence: evidence}
	files := t12Files{objects: map[string][]byte{
		"obj-index": []byte("<h1>t12</h1>"),
		"obj-cit":   []byte(`{"schema":1}`),
	}}
	checker := &t10RoleHTTPChecker{roles: map[string]craft.TaskRole{
		"owner": craft.TaskRoleOwner, "viewer": craft.TaskRoleViewer,
	}}
	exportSvc, err := service.NewCraftExportService(service.CraftExportConfig{
		DB: db, Versions: versions, Evidence: versions,
		Titles: func(_ context.Context, _ uint64, ids []string) (map[string]string, error) {
			out := map[string]string{}
			for _, id := range ids {
				switch id {
				case "k-own":
					out[id] = "Own Region Sales"
				case "k-shared":
					out[id] = "Shared Region Sales"
				}
			}
			return out, nil
		},
		TaskAccess: checker,
		Now:        func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)

	// The T10 knowledge feature rides the same registry so the downloaded
	// manifest's refs can be probed against the real open route.
	resourceAllowed := map[string]bool{"owner": true, "viewer": false}
	knowledge, err := service.NewCraftKnowledgeService(service.CraftKnowledgeConfig{
		Store: t05HTTPStore{ws: craft.Workspace{ID: "ws-t12", Scope: owner}},
		Access: func(ctx context.Context, _ uint64, ids []string) ([]*types.Knowledge, error) {
			caller := types.CallerFromContext(ctx)
			if !resourceAllowed[caller.UserID] {
				return nil, nil
			}
			var rows []*types.Knowledge
			for _, id := range ids {
				switch id {
				case "k-own":
					rows = append(rows, &types.Knowledge{ID: "k-own", KnowledgeBaseID: "kb-own", TenantID: 1, Title: "Own Region Sales"})
				case "k-shared":
					rows = append(rows, &types.Knowledge{ID: "k-shared", KnowledgeBaseID: "kb-shared", TenantID: 7, Title: "Shared Region Sales"})
				}
			}
			return rows, nil
		},
		Search: func(_ context.Context, _ string, _ types.SearchParams) ([]*types.SearchResult, error) {
			return []*types.SearchResult{
				{ID: "c-own", KnowledgeID: "k-own", KnowledgeBaseID: "kb-own", Content: "own excerpt"},
				{ID: "c-shared", KnowledgeID: "k-shared", KnowledgeBaseID: "kb-shared", Content: "shared excerpt"},
			}, nil
		},
		Writer:     func(context.Context, craft.Workspace, string, []byte) error { return nil },
		TaskAccess: checker, Records: repository.NewCraftKnowledgeRecordRepository(db),
		Publisher: &t05HTTPPublisher{},
		Now:       func() time.Time { return time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)

	features := NewCraftFeatureRoutes()
	require.NoError(t, RegisterCraftExportFeature(features, exportSvc, files))
	require.NoError(t, features.Register("knowledge", NewCraftKnowledgeHandler(knowledge).MountCraftKnowledgeRoutes))
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
	describe := "/sessions/session-t12/craft/versions/" + version.ID + "/export"
	download := describe + "/download"

	// --- a member reads the bundle description: digest, members, citation
	// manifest with titles and authenticated refs, build checks.
	summary := get("viewer", describe)
	require.Equal(t, http.StatusOK, summary.Code, summary.Body.String())
	var described struct {
		VersionID      string `json:"version_id"`
		RunID          string `json:"run_id"`
		ManifestDigest string `json:"manifest_digest"`
		Files          []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
			Bytes  int64  `json:"bytes"`
		} `json:"files"`
		Sources []struct {
			CitationID string    `json:"citation_id"`
			Ref        string    `json:"ref"`
			Digest     string    `json:"digest"`
			AcquiredAt time.Time `json:"acquired_at"`
			Title      string    `json:"title"`
		} `json:"sources"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(summary.Body.Bytes(), &described))
	require.Equal(t, version.ID, described.VersionID)
	require.Len(t, described.Files, 2)
	require.Len(t, described.Sources, 2)
	require.Equal(t, "Own Region Sales", described.Sources[0].Title)
	require.Equal(t, evidence.Sources[0].Digest, described.Sources[0].Digest)
	require.NotEmpty(t, described.Sources[0].Ref)
	require.NotContains(t, summary.Body.String(), "excerpt", "no original material in the description")

	// --- the member downloads the bundle zip: members plus the three fixed
	// documents; every entry name is canonical.
	zipResp := get("viewer", download)
	require.Equal(t, http.StatusOK, zipResp.Code, zipResp.Body.String())
	require.Contains(t, zipResp.Header().Get("Content-Type"), "application/zip")
	reader, err := zip.NewReader(bytes.NewReader(zipResp.Body.Bytes()), int64(zipResp.Body.Len()))
	require.NoError(t, err)
	entries := map[string]string{}
	for _, f := range reader.File {
		require.NotContains(t, f.Name, "..", "no traversal entry in the bundle")
		rc, err := f.Open()
		require.NoError(t, err)
		raw, err := io.ReadAll(rc)
		require.NoError(t, err)
		rc.Close()
		entries[f.Name] = string(raw)
	}
	for _, member := range version.Files {
		require.Contains(t, entries, member.Path, "member %s rides the bundle", member.Path)
	}
	require.Equal(t, "<h1>t12</h1>", entries["index.html"])
	require.Contains(t, entries, craft.BundleManifestPath)
	require.Contains(t, entries, craft.BundleSourcesPath)
	require.Contains(t, entries, craft.BundleBuildPath)
	var embedded craft.ExportManifest
	require.NoError(t, json.Unmarshal([]byte(entries[craft.BundleManifestPath]), &embedded))
	require.Equal(t, described.ManifestDigest, embedded.ManifestDigest, "the zip's manifest digest equals the described digest")
	require.Equal(t, version.ID, embedded.VersionID)
	var sourcesDoc craft.BundleCitationManifest
	require.NoError(t, json.Unmarshal([]byte(entries[craft.BundleSourcesPath]), &sourcesDoc))
	require.Len(t, sourcesDoc.Sources, 2)
	require.Equal(t, "Shared Region Sales", sourcesDoc.Sources[1].Title)
	require.NotContains(t, entries[craft.BundleSourcesPath], "excerpt", "no original bytes ride the sources document")
	var buildDoc struct {
		VersionID string `json:"version_id"`
		RunID     string `json:"run_id"`
		Kind      string `json:"kind"`
		Checks    []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	require.NoError(t, json.Unmarshal([]byte(entries[craft.BundleBuildPath]), &buildDoc))
	require.Equal(t, version.ID, buildDoc.VersionID)
	require.NotEmpty(t, buildDoc.Checks)

	// --- a non-member download is a stable 403 and the refusal is audited.
	stranger := get("stranger", download)
	require.Equal(t, http.StatusForbidden, stranger.Code)
	require.JSONEq(t, `{"error":"Forbidden"}`, stranger.Body.String())
	var deniedRows int64
	require.NoError(t, db.Raw("SELECT count(*) FROM audit_logs WHERE action = 'craft.export_denied:task_access' AND outcome = 'denied'").Scan(&deniedRows).Error)
	require.EqualValues(t, 1, deniedRows, "the proven refusal is audited")

	// --- the member download is audited with member, Version and digest.
	var grantedRows []map[string]any
	require.NoError(t, db.Raw("SELECT actor_user_id, target_id, details FROM audit_logs WHERE action = 'craft.export_bundle'").Scan(&grantedRows).Error)
	require.NotEmpty(t, grantedRows)
	for _, row := range grantedRows {
		require.Equal(t, version.ID, row["target_id"])
		require.Contains(t, row["details"], described.ManifestDigest)
	}

	// --- traversal-shaped and missing version ids answer the same stable
	// 404 body (non-leaking), for both describe and download.
	traversal := get("viewer", "/sessions/session-t12/craft/versions/../export/download")
	missing := get("viewer", "/sessions/session-t12/craft/versions/ver_none/export/download")
	require.Equal(t, http.StatusNotFound, traversal.Code, traversal.Body.String())
	require.Equal(t, http.StatusNotFound, missing.Code)
	require.Equal(t, missing.Body.String(), traversal.Body.String(), "denials are stable and non-leaking")

	// --- the manifest reference remains authenticated: the viewer holding
	// the downloaded manifest still cannot open the cited original — every
	// open re-runs the viewer's own current knowledge permission (T10).
	ownerCtx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "owner")
	_, err = knowledge.BuildForRun(ownerCtx, owner, "run-t12-open", "sales", []string{"k-own", "k-shared"})
	require.NoError(t, err)
	recorded, err := knowledge.Sources(ownerCtx, owner, "run-t12-open")
	require.NoError(t, err)
	require.Len(t, recorded.Sources, 2)
	openDenied := get("viewer", "/sessions/session-t12/craft/runs/run-t12-open/sources/"+recorded.Sources[0].ID+"/open")
	require.Equal(t, http.StatusForbidden, openDenied.Code, "holding the downloaded manifest grants no original access")
	require.JSONEq(t, `{"error":"Forbidden"}`, openDenied.Body.String())
	// Granting the viewer their own permission makes the next open succeed:
	// the reference resolves only through fresh authorization.
	resourceAllowed["viewer"] = true
	openAllowed := get("viewer", "/sessions/session-t12/craft/runs/run-t12-open/sources/"+recorded.Sources[0].ID+"/open")
	require.Equal(t, http.StatusOK, openAllowed.Code)
	var opened struct {
		Ref string `json:"ref"`
	}
	require.NoError(t, json.Unmarshal(openAllowed.Body.Bytes(), &opened))
	require.Equal(t, recorded.Sources[0].Ref, opened.Ref)
}
