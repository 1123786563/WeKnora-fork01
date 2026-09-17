package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// -----------------------------------------------------------------------------
// W27: isolated artifact preview harness.
//
// Two entries are under test, mirroring the craft controlled-preview shape
// (W02) on top of the W26 immutable artifact version store:
//
//   - IssueArtifactPreviewTicket runs on the MAIN origin inside the sessions
//     API group: session ownership + (tenant, session, ready) version scoping
//     gate every ticket, and the ticket URL is returned in the authorized
//     response body only.
//   - ArtifactPreviewFile runs on the ISOLATED preview origin with no
//     main-site auth at all: the opaque short-lived ticket is the entire
//     authorization, redemption re-validates the version's readable state,
//     and every response carries the hardened preview policy headers.
// -----------------------------------------------------------------------------

const previewTestOrigin = "https://preview.example.com"

// Guard: the interface is satisfied by the real repository store type at
// compile time, so wiring cannot drift from the handler contract (W26
// precedent — repeated here because this package consumes the source too).
var _ sessionhandler.ArtifactVersionSource = (*repository.ArtifactVersionStore)(nil)

// stubPreviewSessionService answers GetSession from a closure so tests can
// flip a session from owned to revoked mid-scenario.
type stubPreviewSessionService struct {
	interfaces.SessionService
	getSession func(ctx context.Context, id string) (*types.Session, error)
}

func (s *stubPreviewSessionService) GetSession(ctx context.Context, id string) (*types.Session, error) {
	return s.getSession(ctx, id)
}

// stubPreviewVersionSource mirrors ReadableArtifactVersion's contract: only
// the exact (tenant, session, ready version) triple answers; `fail` flips
// every later read to not-found (resource revocation scenario).
type stubPreviewVersionSource struct {
	tenant   uint64
	session  string
	version  repository.ArtifactVersion
	fail     bool
	calls    int
	lastCall struct {
		tenant  uint64
		session string
		version string
	}
}

func (s *stubPreviewVersionSource) ReadableArtifactVersion(
	_ context.Context, tenantID uint64, sessionID, versionID string,
) (repository.ArtifactVersion, error) {
	s.calls++
	s.lastCall.tenant, s.lastCall.session, s.lastCall.version = tenantID, sessionID, versionID
	if s.fail {
		return repository.ArtifactVersion{}, repository.ErrArtifactVersionNotFound
	}
	if tenantID == s.tenant && sessionID == s.session && versionID == s.version.ID {
		return s.version, nil
	}
	return repository.ArtifactVersion{}, repository.ErrArtifactVersionNotFound
}

// fakePreviewFileService serves canned bytes for a single object key.
type fakePreviewFileService struct {
	interfaces.FileService
	key  string
	data []byte
}

func (f *fakePreviewFileService) GetFile(_ context.Context, key string) (io.ReadCloser, error) {
	if key != f.key {
		return nil, apperrors.NewNotFoundError("blob missing")
	}
	return io.NopCloser(strings.NewReader(string(f.data))), nil
}

// mutablePreviewClock gives tests control over ticket expiry.
type mutablePreviewClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *mutablePreviewClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *mutablePreviewClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func previewTestVersion(mime string, size int64) repository.ArtifactVersion {
	return repository.ArtifactVersion{
		TenantID: 42, ID: "v1", RunID: "r1", SessionID: "sess-1",
		Digest:    strings.Repeat("a", 64),
		ObjectKey: "artifact-versions/42/r1/" + strings.Repeat("a", 64),
		MIME:      mime, ScanState: repository.ArtifactScanReady, Size: size,
	}
}

func newPreviewTestHandler(
	source sessionhandler.ArtifactVersionSource,
	sessions interfaces.SessionService,
	origin string,
	ttl time.Duration,
	now func() time.Time,
) *ArtifactPreviewHandler {
	return newArtifactPreviewHandler(sessions, nil, &fakePreviewFileService{
		key:  "artifact-versions/42/r1/" + strings.Repeat("a", 64),
		data: []byte("<svg>PREVIEW-BYTES</svg>"),
	}, nil, source, origin, ttl, now)
}

// newArtifactPreviewTestRouter mirrors production mounting: the issue route
// sits inside the sessions API group (auth chain + tenant context), the
// redemption route sits on the engine root with no auth at all.
func newArtifactPreviewTestRouter(t *testing.T, tenant uint64, h *ArtifactPreviewHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Request = c.Request.WithContext(
			context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenant))
		c.Next()
	})
	sessions := r.Group("/api/v1/sessions")
	RegisterArtifactPreviewIssueRoute(sessions, h)
	RegisterArtifactPreviewRoutes(r, h)
	return r
}

func ownedPreviewSessions() interfaces.SessionService {
	return &stubPreviewSessionService{
		getSession: func(_ context.Context, _ string) (*types.Session, error) {
			return &types.Session{ID: "sess-1", TenantID: 42}, nil
		},
	}
}

type previewTicketResponse struct {
	Success bool `json:"success"`
	Data    struct {
		URL       string    `json:"url"`
		ExpiresAt time.Time `json:"expires_at"`
		VersionID string    `json:"version_id"`
	} `json:"data"`
}

func issuePreviewTicket(t *testing.T, r *gin.Engine, sessionID, versionID string) previewTicketResponse {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost,
		"/api/v1/sessions/"+sessionID+"/artifact-versions/"+versionID+"/preview-ticket", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("issue status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp previewTicketResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("issue response not json: %v (%s)", err, w.Body.String())
	}
	return resp
}

func previewTokenFromURL(t *testing.T, url string) string {
	t.Helper()
	token := strings.TrimPrefix(url, previewTestOrigin+"/ap/")
	if token == url || token == "" {
		t.Fatalf("ticket url %q must be %s/ap/<token>", url, previewTestOrigin)
	}
	return token
}

func redeemPreview(r *gin.Engine, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ap/"+token, nil))
	return w
}

// -----------------------------------------------------------------------------
// Issuance (main origin, session-scoped)
// -----------------------------------------------------------------------------

func TestIssueArtifactPreviewTicket_HappyPath(t *testing.T) {
	source := &stubPreviewVersionSource{tenant: 42, session: "sess-1", version: previewTestVersion("image/svg+xml", 25)}
	clock := &mutablePreviewClock{now: time.Now()}
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(source, ownedPreviewSessions(), previewTestOrigin, ArtifactPreviewTicketTTL, clock.Now))

	ticket := issuePreviewTicket(t, r, "sess-1", "v1")

	if !strings.HasPrefix(ticket.Data.URL, previewTestOrigin+"/ap/") {
		t.Fatalf("ticket url = %q, want preview origin path %s/ap/<token>", ticket.Data.URL, previewTestOrigin)
	}
	if len(previewTokenFromURL(t, ticket.Data.URL)) < 32 {
		t.Fatal("ticket token must carry at least 256 bits of entropy")
	}
	if ticket.Data.VersionID != "v1" {
		t.Fatalf("version id = %q, want v1", ticket.Data.VersionID)
	}
	if want := clock.Now().Add(ArtifactPreviewTicketTTL); !ticket.Data.ExpiresAt.Equal(want) {
		t.Fatalf("expires_at = %v, want %v (short-lived)", ticket.Data.ExpiresAt, want)
	}
	if source.calls != 1 || source.lastCall.tenant != 42 || source.lastCall.session != "sess-1" || source.lastCall.version != "v1" {
		t.Fatalf("version source calls = %+v, want one tenant/session/version-scoped read", source.lastCall)
	}
	// The ticket lives in the authorized response body only: no header,
	// cookie or redirect may ever carry it.
	leak := httptest.NewRecorder()
	r.ServeHTTP(leak, httptest.NewRequest(http.MethodPost, "/api/v1/sessions/sess-1/artifact-versions/v1/preview-ticket", nil))
	if leak.Header().Get("Set-Cookie") != "" || leak.Header().Get("Location") != "" {
		t.Fatalf("ticket must never travel in a cookie or redirect: %v", leak.Header())
	}
	if !strings.Contains(leak.Body.String(), "expires_at") {
		t.Fatal("issue response must carry the short-lived expiry to the client")
	}
}

func TestIssueArtifactPreviewTicket_SessionNotOwnedIs404(t *testing.T) {
	revoked := &stubPreviewSessionService{
		getSession: func(_ context.Context, _ string) (*types.Session, error) {
			return nil, apperrors.ErrSessionNotFound
		},
	}
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(&stubPreviewVersionSource{tenant: 42, session: "sess-1", version: previewTestVersion("image/svg+xml", 25)}, revoked, previewTestOrigin, ArtifactPreviewTicketTTL, time.Now))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/sessions/sess-x/artifact-versions/v1/preview-ticket", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for non-owned session", w.Code)
	}
}

// Resource permission revocation: the session the ticket would be minted for
// disappears — NEW ticket issuance must be refused (the W27 boundary).
func TestIssueArtifactPreviewTicket_RefusedAfterRevocation(t *testing.T) {
	owned := true
	sessions := &stubPreviewSessionService{
		getSession: func(_ context.Context, id string) (*types.Session, error) {
			if !owned || id != "sess-1" {
				return nil, apperrors.ErrSessionNotFound
			}
			return &types.Session{ID: "sess-1", TenantID: 42}, nil
		},
	}
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(&stubPreviewVersionSource{tenant: 42, session: "sess-1", version: previewTestVersion("image/svg+xml", 25)}, sessions, previewTestOrigin, ArtifactPreviewTicketTTL, time.Now))

	issuePreviewTicket(t, r, "sess-1", "v1") // before revocation: succeeds

	owned = false // session access revoked
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/sessions/sess-1/artifact-versions/v1/preview-ticket", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: new tickets must be refused after permission revocation", w.Code)
	}
}

func TestIssueArtifactPreviewTicket_NotReadableVersionIs404(t *testing.T) {
	source := &stubPreviewVersionSource{tenant: 42, session: "sess-1"} // no version row
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(source, ownedPreviewSessions(), previewTestOrigin, ArtifactPreviewTicketTTL, time.Now))
	for _, versionID := range []string{"v1", ""} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost,
			"/api/v1/sessions/sess-1/artifact-versions/"+versionID+"/preview-ticket", nil))
		if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d for version %q, want 404/400", w.Code, versionID)
		}
	}
}

func TestIssueArtifactPreviewTicket_UnpreviewableMimeIs400(t *testing.T) {
	source := &stubPreviewVersionSource{tenant: 42, session: "sess-1", version: previewTestVersion("application/zip", 100)}
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(source, ownedPreviewSessions(), previewTestOrigin, ArtifactPreviewTicketTTL, time.Now))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/sessions/sess-1/artifact-versions/v1/preview-ticket", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for non-previewable artifact type", w.Code)
	}
}

func TestIssueArtifactPreviewTicket_FailClosedWithoutSource(t *testing.T) {
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(nil, ownedPreviewSessions(), previewTestOrigin, ArtifactPreviewTicketTTL, time.Now))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/sessions/sess-1/artifact-versions/v1/preview-ticket", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when the version source is not wired", w.Code)
	}
}

func TestIssueArtifactPreviewTicket_DisabledOriginIsRejected(t *testing.T) {
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(&stubPreviewVersionSource{}, ownedPreviewSessions(), "", ArtifactPreviewTicketTTL, time.Now))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/sessions/sess-1/artifact-versions/v1/preview-ticket", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when no isolated preview origin is configured", w.Code)
	}
}

// -----------------------------------------------------------------------------
// Redemption (isolated origin, ticket-only authorization)
// -----------------------------------------------------------------------------

func TestArtifactPreviewFile_ServesBytesWithPolicyHeaders(t *testing.T) {
	source := &stubPreviewVersionSource{tenant: 42, session: "sess-1", version: previewTestVersion("image/svg+xml", 25)}
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(source, ownedPreviewSessions(), previewTestOrigin, ArtifactPreviewTicketTTL, time.Now))

	token := previewTokenFromURL(t, issuePreviewTicket(t, r, "sess-1", "v1").Data.URL)
	w := redeemPreview(r, token)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "<svg>PREVIEW-BYTES</svg>" {
		t.Fatalf("body = %q, want the artifact bytes", got)
	}
	// The MIME comes from the server-validated version row, never sniffed.
	if got := w.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Fatalf("Content-Type = %q, want image/svg+xml", got)
	}
	// The full hardened preview policy — identical constants to the mobile
	// client's preview-policy.ts (cross-language parity is pinned by both
	// suites asserting the same directives).
	csp := w.Header().Get("Content-Security-Policy")
	if csp != ArtifactPreviewCSP {
		t.Fatalf("CSP = %q, want the pinned preview policy %q", csp, ArtifactPreviewCSP)
	}
	if !strings.Contains(csp, "connect-src 'none'") || !strings.Contains(csp, "sandbox allow-scripts") {
		t.Fatal("CSP must deny all connects and sandbox the document")
	}
	if strings.Contains(csp, "allow-same-origin") {
		t.Fatal("sandbox must not re-grant same-origin storage access")
	}
	if got := w.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q, want no-referrer", got)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q, want private, no-store", got)
	}
	if got := w.Header().Get("Content-Disposition"); got != "inline" {
		t.Fatalf("Content-Disposition = %q, want inline", got)
	}
	if got := w.Header().Get("Cross-Origin-Resource-Policy"); got != "same-origin" {
		t.Fatalf("Cross-Origin-Resource-Policy = %q, want same-origin", got)
	}
}

func TestArtifactPreviewFile_ExpiredTicketIs404(t *testing.T) {
	source := &stubPreviewVersionSource{tenant: 42, session: "sess-1", version: previewTestVersion("image/svg+xml", 25)}
	clock := &mutablePreviewClock{now: time.Now()}
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(source, ownedPreviewSessions(), previewTestOrigin, ArtifactPreviewTicketTTL, clock.Now))

	token := previewTokenFromURL(t, issuePreviewTicket(t, r, "sess-1", "v1").Data.URL)
	clock.Advance(ArtifactPreviewTicketTTL + time.Second)

	if w := redeemPreview(r, token); w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for expired ticket", w.Code)
	}
	// The expired grant must also be dropped, not just answered.
	if w := redeemPreview(r, token); w.Code != http.StatusNotFound {
		t.Fatalf("status = %d on retry, want the expired grant to stay gone", w.Code)
	}
}

// Resource revocation at redemption time: the ticket is live but the version
// is no longer readable in its scope — the isolated origin refuses.
func TestArtifactPreviewFile_RevokedResourceIs404(t *testing.T) {
	source := &stubPreviewVersionSource{tenant: 42, session: "sess-1", version: previewTestVersion("image/svg+xml", 25)}
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(source, ownedPreviewSessions(), previewTestOrigin, ArtifactPreviewTicketTTL, time.Now))

	token := previewTokenFromURL(t, issuePreviewTicket(t, r, "sess-1", "v1").Data.URL)
	source.fail = true // version pulled out of the readable set after issuance

	if w := redeemPreview(r, token); w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when the bound version stops being readable", w.Code)
	}
}

func TestArtifactPreviewFile_UnknownOrTamperedTokenIs404(t *testing.T) {
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(&stubPreviewVersionSource{tenant: 42, session: "sess-1", version: previewTestVersion("image/svg+xml", 25)}, ownedPreviewSessions(), previewTestOrigin, ArtifactPreviewTicketTTL, time.Now))
	for _, token := range []string{"unknown-token", "", strings.Repeat("A", 64)} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ap/"+token, nil))
		if token == "" {
			continue // gin routing: empty segment is a 404 by the router itself
		}
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d for token %q, want 404", w.Code, token)
		}
	}
}

func TestArtifactPreviewFile_DisabledOrUnwiredIs404(t *testing.T) {
	// No origin configured: the isolated endpoint answers 404, nothing else.
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(&stubPreviewVersionSource{}, ownedPreviewSessions(), "", ArtifactPreviewTicketTTL, time.Now))
	if w := redeemPreview(r, "any-token"); w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when preview origin is not configured", w.Code)
	}

	// Fail-closed mounting: a nil handler registers no route at all.
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterArtifactPreviewRoutes(engine, nil)
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/ap/") {
			t.Fatalf("no /ap route may exist without assembly, found %s %s", route.Method, route.Path)
		}
	}
}

func TestArtifactPreviewFile_MissingBlobIs404(t *testing.T) {
	source := &stubPreviewVersionSource{tenant: 42, session: "sess-1", version: previewTestVersion("image/svg+xml", 25)}
	h := newArtifactPreviewHandler(ownedPreviewSessions(), nil,
		&fakePreviewFileService{key: "other-key", data: []byte("x")}, nil, source,
		previewTestOrigin, ArtifactPreviewTicketTTL, time.Now)
	r := newArtifactPreviewTestRouter(t, 42, h)
	token := previewTokenFromURL(t, issuePreviewTicket(t, r, "sess-1", "v1").Data.URL)
	if w := redeemPreview(r, token); w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for missing blob", w.Code)
	}
}

// The preview origin serves one object per version and never proxies a
// client-supplied URL: the only addressing it understands is the ticket.
func TestArtifactPreviewFile_NeverProxiesArbitraryPaths(t *testing.T) {
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(&stubPreviewVersionSource{}, ownedPreviewSessions(), previewTestOrigin, ArtifactPreviewTicketTTL, time.Now))
	for _, path := range []string{
		"/ap/",
		"/ap/https://internal-service:8080/admin",
		"/ap/http://169.254.169.254/latest/meta-data/",
		"/ap/file:///etc/passwd",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d for %s, want 404/400 (no arbitrary URL proxying)", w.Code, path)
		}
		if strings.Contains(w.Body.String(), "meta-data") || strings.Contains(w.Body.String(), "root:") {
			t.Fatalf("response body must not contain fetched bytes for %s: %s", path, w.Body.String())
		}
	}
}

// HEAD answers the same policy headers without a body (WebView prefetches).
func TestArtifactPreviewFile_HEADCarriesHeadersWithoutBody(t *testing.T) {
	source := &stubPreviewVersionSource{tenant: 42, session: "sess-1", version: previewTestVersion("text/html", 12)}
	r := newArtifactPreviewTestRouter(t, 42,
		newPreviewTestHandler(source, ownedPreviewSessions(), previewTestOrigin, ArtifactPreviewTicketTTL, time.Now))
	token := previewTokenFromURL(t, issuePreviewTicket(t, r, "sess-1", "v1").Data.URL)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodHead, "/ap/"+token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for HEAD", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("HEAD body = %q, want empty", w.Body.String())
	}
	if w.Header().Get("Content-Security-Policy") != ArtifactPreviewCSP {
		t.Fatal("HEAD must carry the same preview policy")
	}
}
