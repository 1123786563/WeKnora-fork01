package session

// T12 (#132) OCR regressions: the download stream's mid-flight failure
// contract. The manifest inside the zip declares the exact member list and
// their digests, so a member that cannot be read — or whose streamed bytes
// disagree with the manifest's digest claim — must ABORT the connection
// (http.ErrAbortHandler): the client sees a broken transfer instead of a
// structurally valid zip that silently disagrees with its own manifest.

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func t12AbortHex(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

// t12FailingFiles fails one named ref; other refs stream normally.
type t12FailingFiles struct {
	objects map[string][]byte
	failRef string
}

func (f t12FailingFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	if ref == f.failRef {
		return nil, craft.ErrNotFound
	}
	data, ok := f.objects[ref]
	if !ok {
		return nil, craft.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// t12CorruptFiles streams bytes that do NOT match the manifest's digest
// claim for the named ref (a swapped or corrupted object).
type t12CorruptFiles struct {
	objects    map[string][]byte
	corruptRef string
}

func (f t12CorruptFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	data := f.objects[ref]
	if ref == f.corruptRef {
		data = []byte("swapped bytes")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// newT12AbortFixture builds a one-member bundle service whose member
// "index.html" carries a known digest.
func newT12AbortFixture(t *testing.T) (craft.Version, craft.VersionEvidence) {
	t.Helper()
	pinned := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	versionID := "ver_" + t12AbortHex("t12-abort")
	evidence := craft.VersionEvidence{
		VersionID: versionID, RunID: "run-abort", Empty: true,
		RequestDigest: t12AbortHex("req"), PackageDigest: t12AbortHex("pkg"), PinnedAt: pinned,
	}
	version := craft.Version{
		ID: versionID, WorkspaceID: "ws-abort", RunID: "run-abort", Kind: craft.KindWeb,
		Files:  []craft.File{{Path: "index.html", Ref: "obj-index", SHA256: t12AbortHex("index"), MIME: "text/html", Bytes: 8}},
		Checks: []craft.Check{{Name: craft.CheckBuild, Status: craft.CheckPassed, Detail: "exit 0"}},
	}
	return version, evidence
}

func newT12AbortContext(t *testing.T, versionID, user string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/sessions/s-abort/craft/versions/"+versionID+"/export/download", nil)
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, user)
	req = req.WithContext(ctx)
	c.Request = req
	// craftScope reads the tenant from the gin KV and the session from the
	// route params, exactly as the mounted route would provide them.
	c.Set(types.TenantIDContextKey.String(), uint64(1))
	c.Params = gin.Params{
		{Key: "session_id", Value: "s-abort"},
		{Key: "version_id", Value: versionID},
	}
	return c, w
}

// TestCraftT12DownloadAbortsWhenMemberUnreadable pins the abort contract: a
// mid-stream GetFile failure truncates the transfer instead of shipping a
// zip that silently omits the member its manifest declares.
func TestCraftT12DownloadAbortsWhenMemberUnreadable(t *testing.T) {
	version, evidence := newT12AbortFixture(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	svc, err := service.NewCraftExportService(service.CraftExportConfig{
		DB: db, Versions: &t12VersionReader{version: version, evidence: evidence},
		Evidence:   &t12VersionReader{version: version, evidence: evidence},
		TaskAccess: &t10RoleHTTPChecker{roles: map[string]craft.TaskRole{"owner": craft.TaskRoleOwner}},
	})
	require.NoError(t, err)
	handler := NewCraftExportHandler(svc, t12FailingFiles{
		objects: map[string][]byte{"obj-index": []byte("<h1>x</h1>")}, failRef: "obj-index",
	})

	c, _ := newT12AbortContext(t, version.ID, "owner")
	require.PanicsWithValue(t, http.ErrAbortHandler, func() { handler.DownloadCraftExportBundle(c) },
		"an unreadable member aborts the transfer, never a valid-but-incomplete zip")
}

// TestCraftT12DownloadAbortsOnDigestMismatch pins the byte-side provenance:
// streamed bytes that disagree with the manifest's digest claim (a
// corrupted object or a swapped ref) abort the download.
func TestCraftT12DownloadAbortsOnDigestMismatch(t *testing.T) {
	version, evidence := newT12AbortFixture(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	svc, err := service.NewCraftExportService(service.CraftExportConfig{
		DB: db, Versions: &t12VersionReader{version: version, evidence: evidence},
		Evidence:   &t12VersionReader{version: version, evidence: evidence},
		TaskAccess: &t10RoleHTTPChecker{roles: map[string]craft.TaskRole{"owner": craft.TaskRoleOwner}},
	})
	require.NoError(t, err)
	handler := NewCraftExportHandler(svc, t12CorruptFiles{
		objects: map[string][]byte{"obj-index": []byte("<h1>good</h1>")}, corruptRef: "obj-index",
	})

	c, _ := newT12AbortContext(t, version.ID, "owner")
	require.PanicsWithValue(t, http.ErrAbortHandler, func() { handler.DownloadCraftExportBundle(c) },
		"bytes disagreeing with the manifest digest abort the transfer")
}

// TestCraftT12HappyDownloadStreamsIntactZip pins the counterpart: with the
// object store intact the same download completes without aborting and the
// zip carries the fixed documents plus the member.
func TestCraftT12HappyDownloadStreamsIntactZip(t *testing.T) {
	version, evidence := newT12AbortFixture(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	svc, err := service.NewCraftExportService(service.CraftExportConfig{
		DB: db, Versions: &t12VersionReader{version: version, evidence: evidence},
		Evidence:   &t12VersionReader{version: version, evidence: evidence},
		TaskAccess: &t10RoleHTTPChecker{roles: map[string]craft.TaskRole{"owner": craft.TaskRoleOwner}},
	})
	require.NoError(t, err)
	// The served bytes hash to exactly the manifest's digest claim
	// (t12AbortHex("index")), so the streaming verification passes.
	handler := NewCraftExportHandler(svc, t12Files{objects: map[string][]byte{"obj-index": []byte("index")}})

	c, w := newT12AbortContext(t, version.ID, "owner")
	require.NotPanics(t, func() { handler.DownloadCraftExportBundle(c) })
	require.Equal(t, http.StatusOK, w.Code)
	zipReader, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	require.NoError(t, err)
	entries := map[string]string{}
	for _, f := range zipReader.File {
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		rc.Close()
		require.NoError(t, err)
		entries[f.Name] = string(data)
	}
	for _, want := range []string{"export-manifest.json", "sources.json", "build.json", "index.html"} {
		require.Contains(t, entries, want, "the intact bundle carries %s", want)
	}
	require.Equal(t, "index", entries["index.html"], "the member's bytes stream verbatim")
	// build.json carries the shared snake_case key shape (not craft.Check's
	// Go field-name keys) — the same keys the describe endpoint projects.
	require.Contains(t, entries["build.json"], `"name":`, "build.json checks use the lowercase wire keys")
	require.False(t, strings.Contains(entries["build.json"], `"Name"`), "no Go field-name keys leak into the fixed documents")
}
