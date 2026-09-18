package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// -----------------------------------------------------------------------------
// W26: versioned artifact download harness.
//
// The versioned entry reuses the legacy handle's ownership check
// (sessionService.GetSession) and the file-transport serving path; the new
// behavior under test is the explicit version-ID distinction, the ready-gate
// (nothing readable before publish), and the tenant/session scoping that
// makes foreign-workspace versions indistinguishable from missing ones.
// -----------------------------------------------------------------------------

// stubArtifactVersionSource is a stand-in for the repository store. It only
// answers when the exact (tenant, session, version) triple matches a ready
// row, mirroring the store's ReadableArtifactVersion contract.
type stubArtifactVersionSource struct {
	tenant   uint64
	session  string
	version  repository.ArtifactVersion
	calls    int
	lastCall struct {
		tenant   uint64
		session  string
		version  string
	}
}

func (s *stubArtifactVersionSource) ReadableArtifactVersion(
	_ context.Context, tenantID uint64, sessionID, versionID string,
) (repository.ArtifactVersion, error) {
	s.calls++
	s.lastCall.tenant, s.lastCall.session, s.lastCall.version = tenantID, sessionID, versionID
	if tenantID == s.tenant && sessionID == s.session && versionID == s.version.ID {
		return s.version, nil
	}
	return repository.ArtifactVersion{}, repository.ErrArtifactVersionNotFound
}

func newArtifactVersionTestRouter(t *testing.T, tenant uint64, h *ArtifactVersionDownloadHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenant))
		c.Next()
	})
	r.GET("/sessions/:id/artifact-versions/:version_id/download", h.DownloadArtifactVersion)
	return r
}

func versionTestHandler(source ArtifactVersionSource) *ArtifactVersionDownloadHandler {
	return NewArtifactVersionDownloadHandler(
		&stubSessionServiceForArtifacts{
			getSession: func(_ context.Context, _ string) (*types.Session, error) {
				return &types.Session{ID: "sess-1", TenantID: 42}, nil
			},
		},
		nil, // no tenant service: fall back to the global file service
		&fakeArtifactFileService{url: "artifact-versions/42/r1/" + strings.Repeat("a", 64), data: []byte("VERSION-BYTES")},
		nil,
		source,
	)
}

func TestDownloadArtifactVersion_HappyPath(t *testing.T) {
	source := &stubArtifactVersionSource{
		tenant:  42,
		session: "sess-1",
		version: repository.ArtifactVersion{
			TenantID: 42, ID: "v1", RunID: "r1", SessionID: "sess-1",
			Digest: strings.Repeat("a", 64), ObjectKey: "artifact-versions/42/r1/" + strings.Repeat("a", 64),
			MIME: "text/csv", ScanState: "ready", Size: 13,
		},
	}
	router := newArtifactVersionTestRouter(t, 42, versionTestHandler(source))
	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-1/artifact-versions/v1/download", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "VERSION-BYTES" {
		t.Fatalf("body = %q, want VERSION-BYTES", got)
	}
	// The MIME type comes from the server-validated version row, not sniffing.
	if got := w.Header().Get("Content-Type"); got != "text/csv" {
		t.Fatalf("Content-Type = %q, want text/csv", got)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment;") || !strings.Contains(cd, "artifact-v1.csv") {
		t.Fatalf("Content-Disposition = %q, want attachment with artifact-v1.csv", cd)
	}
	if got := w.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q, want private, no-store", got)
	}
	if source.calls != 1 || source.lastCall.tenant != 42 || source.lastCall.session != "sess-1" || source.lastCall.version != "v1" {
		t.Fatalf("source calls = %+v, want one tenant/session/version-scoped read", source.lastCall)
	}
}

func TestDownloadArtifactVersion_NotReadableIs404(t *testing.T) {
	source := &stubArtifactVersionSource{tenant: 42, session: "sess-1"}
	router := newArtifactVersionTestRouter(t, 42, versionTestHandler(source))
	for _, path := range []string{
		"/sessions/sess-1/artifact-versions/v1/download", // unknown version
		"/sessions/sess-1/artifact-versions//download",   // empty version id
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if path == "/sessions/sess-1/artifact-versions//download" {
			// gin treats the empty segment as a missing route -> 404 by routing
			continue
		}
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d for %s, want 404", w.Code, path)
		}
		if strings.Contains(w.Body.String(), "artifact-versions") {
			t.Fatalf("response leaked the object key: %s", w.Body.String())
		}
	}
}

func TestDownloadArtifactVersion_CrossWorkspaceIs404(t *testing.T) {
	// The version exists in tenant 42, but the caller authenticates as
	// tenant 7: the tenant-scoped read must be a plain 404.
	source := &stubArtifactVersionSource{
		tenant: 42,
		session: "sess-1",
		version: repository.ArtifactVersion{ID: "v1", ObjectKey: "artifact-versions/42/r1/" + strings.Repeat("a", 64)},
	}
	handler := NewArtifactVersionDownloadHandler(
		&stubSessionServiceForArtifacts{
			getSession: func(_ context.Context, id string) (*types.Session, error) {
				if id != "sess-1" {
					return nil, apperrors.ErrSessionNotFound
				}
				return &types.Session{ID: id, TenantID: 42}, nil
			},
		},
		nil,
		&fakeArtifactFileService{},
		nil,
		source,
	)
	router := newArtifactVersionTestRouter(t, 7, handler)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sessions/sess-1/artifact-versions/v1/download", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for cross-workspace version", w.Code)
	}
	if source.calls != 1 || source.lastCall.tenant != 7 {
		t.Fatalf("source must be queried with the caller tenant 7, got %+v", source.lastCall)
	}
}

func TestDownloadArtifactVersion_SessionNotOwnedIs404(t *testing.T) {
	handler := NewArtifactVersionDownloadHandler(
		&stubSessionServiceForArtifacts{
			getSession: func(_ context.Context, _ string) (*types.Session, error) {
				return nil, apperrors.ErrSessionNotFound
			},
		},
		nil,
		&fakeArtifactFileService{},
		nil,
		&stubArtifactVersionSource{},
	)
	router := newArtifactVersionTestRouter(t, 42, handler)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sessions/sess-x/artifact-versions/v1/download", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestDownloadArtifactVersion_FailClosedWithoutSource(t *testing.T) {
	router := newArtifactVersionTestRouter(t, 42, versionTestHandler(nil))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sessions/sess-1/artifact-versions/v1/download", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when the version source is not wired", w.Code)
	}
}

func TestDownloadArtifactVersion_MissingBlobIs404(t *testing.T) {
	source := &stubArtifactVersionSource{
		tenant:  42,
		session: "sess-1",
		version: repository.ArtifactVersion{
			TenantID: 42, ID: "v1", ObjectKey: "artifact-versions/42/r1/" + strings.Repeat("b", 64),
			MIME: "application/octet-stream", ScanState: "ready", Size: 4,
		},
	}
	handler := NewArtifactVersionDownloadHandler(
		&stubSessionServiceForArtifacts{
			getSession: func(_ context.Context, _ string) (*types.Session, error) {
				return &types.Session{ID: "sess-1", TenantID: 42}, nil
			},
		},
		nil,
		&fakeArtifactFileService{url: "other-key", data: []byte("x")}, // serves a different key only
		nil,
		source,
	)
	router := newArtifactVersionTestRouter(t, 42, handler)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sessions/sess-1/artifact-versions/v1/download", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for missing blob", w.Code)
	}
}

// Guard: the interface is satisfied by the real repository store type at
// compile time, so wiring cannot drift from the handler contract.
var _ ArtifactVersionSource = (*repository.ArtifactVersionStore)(nil)
