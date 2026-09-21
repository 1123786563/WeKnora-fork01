package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/middleware"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Test doubles: a scope-ACL'd in-memory version store, a byte store, a clock.
// -----------------------------------------------------------------------------

// previewVersionStore mirrors the real store's scope semantics: cross-tenant
// and cross-session versions do not exist for the caller, a foreign owner's
// version is forbidden.
type previewVersionStore struct {
	mu        sync.Mutex
	byID      map[string]craft.Version
	order     []string
	ownerByID map[string]craft.Scope
}

func newPreviewVersionStore() *previewVersionStore {
	return &previewVersionStore{byID: map[string]craft.Version{}, ownerByID: map[string]craft.Scope{}}
}

func (m *previewVersionStore) Publish(_ context.Context, scope craft.Scope, v craft.Version) (craft.Version, error) {
	digest, err := craft.ManifestDigest(v.Files)
	if err != nil {
		return craft.Version{}, err
	}
	id := craft.VersionID(v.WorkspaceID, v.RunID, digest)
	m.mu.Lock()
	defer m.mu.Unlock()
	if stored, ok := m.byID[id]; ok {
		return stored, nil
	}
	v.ID = id
	if v.Checks == nil {
		v.Checks = []craft.Check{}
	}
	m.byID[id] = v
	m.order = append(m.order, id)
	m.ownerByID[id] = scope
	return v, nil
}

func (m *previewVersionStore) List(_ context.Context, scope craft.Scope) ([]craft.Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []craft.Version
	for i := len(m.order) - 1; i >= 0; i-- {
		id := m.order[i]
		owner := m.ownerByID[id]
		if owner.TenantID != scope.TenantID || owner.SessionID != scope.SessionID {
			continue
		}
		out = append(out, m.byID[id])
	}
	return out, nil
}

// replaceCheck rewrites one named check of a stored version in place, like
// the real PreviewCheckStore does through checks_json.
func (m *previewVersionStore) replaceCheck(versionID string, check craft.Check) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.byID[versionID]
	if !ok {
		return
	}
	replaced := false
	checks := make([]craft.Check, 0, len(v.Checks)+1)
	for _, c := range v.Checks {
		if c.Name == check.Name {
			checks = append(checks, check)
			replaced = true
			continue
		}
		checks = append(checks, c)
	}
	if !replaced {
		checks = append(checks, check)
	}
	v.Checks = checks
	m.byID[versionID] = v
}

func (m *previewVersionStore) Get(_ context.Context, scope craft.Scope, id string) (craft.Version, error) {
	m.mu.Lock()
	v, ok := m.byID[id]
	owner := m.ownerByID[id]
	m.mu.Unlock()
	if !ok || owner.TenantID != scope.TenantID || owner.SessionID != scope.SessionID {
		return craft.Version{}, craft.ErrNotFound
	}
	if owner.UserID != scope.UserID {
		return craft.Version{}, craft.ErrForbidden
	}
	return v, nil
}

// previewFileStore stores bytes under refs; the preview reads through GetFile.
// The embedded interface satisfies the rest of interfaces.FileService; a
// stray call to an un-stubbed method nil-panics loudly.
type previewFileStore struct {
	interfaces.FileService
	mu    sync.Mutex
	blobs map[string][]byte
}

func newPreviewFileStore() *previewFileStore { return &previewFileStore{blobs: map[string][]byte{}} }

func (f *previewFileStore) save(data []byte, name string) string {
	// Content-addressed like the real collector's object names, so v2's
	// index.html never overwrites v1's pinned object.
	sum := sha256.Sum256(data)
	ref := "preview://obj/craft_" + hex.EncodeToString(sum[:]) + "_" + name
	f.mu.Lock()
	f.blobs[ref] = data
	f.mu.Unlock()
	return ref
}

func (f *previewFileStore) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	f.mu.Lock()
	data, ok := f.blobs[ref]
	f.mu.Unlock()
	if !ok {
		return nil, stderrors.New("object missing")
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

// previewCheckStore records updates AND mutates the version store, mirroring
// the real store's read-back semantics: after an update, List/Get answer the
// rewritten checks.
type previewCheckStore struct {
	mu       sync.Mutex
	updated  map[string]craft.Check
	versions *previewVersionStore
}

func newPreviewCheckStore() *previewCheckStore {
	return &previewCheckStore{updated: map[string]craft.Check{}}
}

func (p *previewCheckStore) UpdatePreviewCheck(_ context.Context, _ craft.Scope, versionID string, check craft.Check) (craft.Version, error) {
	if check.Name != craft.CheckPreview {
		return craft.Version{}, craft.ErrInvalidInput
	}
	p.mu.Lock()
	p.updated[versionID] = check
	p.mu.Unlock()
	if p.versions != nil {
		p.versions.replaceCheck(versionID, check)
	}
	return craft.Version{ID: versionID, Checks: []craft.Check{check}}, nil
}

// fakePreviewClock controls grant expiry deterministically.
type fakePreviewClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakePreviewClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakePreviewClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

// -----------------------------------------------------------------------------
// Harness
// -----------------------------------------------------------------------------

type previewEnv struct {
	store   *previewVersionStore
	files   *previewFileStore
	checks  *previewCheckStore
	clock   *fakePreviewClock
	service *service.CraftPreviewService
	issue   *gin.Engine
	preview *gin.Engine
	scope   craft.Scope
}

func newPreviewEnv(t *testing.T) *previewEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	env := &previewEnv{
		store:  newPreviewVersionStore(),
		files:  newPreviewFileStore(),
		checks: newPreviewCheckStore(),
		clock:  &fakePreviewClock{now: time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)},
		scope:  craft.Scope{TenantID: 42, UserID: "user-1", SessionID: "sess-1"},
	}
	env.checks.versions = env.store
	env.service = service.NewCraftPreviewService(env.store, env.files, env.checks, service.CraftPreviewConfig{
		AppOrigin:     "https://app.test",
		PreviewOrigin: "https://preview.test",
		TTL:           craft.PreviewTicketTTL,
		Now:           env.clock.Now,
	})
	h := NewCraftPreviewHandler(env.service)

	// Main-origin issuance router: authenticated context + house error
	// middleware, exactly like the production sessions group.
	env.issue = gin.New()
	env.issue.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		// Production auth middleware sets both the gin KV store (the
		// handler's c.Get path) and the request context (context helpers).
		c.Set(types.TenantIDContextKey.String(), uint64(42))
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(42))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "user-1")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	RegisterCraftPreviewIssueRoute(env.issue.Group("/sessions"), h)

	// Isolated-origin preview router through the real registration.
	env.preview = gin.New()
	RegisterCraftPreviewRoutes(env.preview, h)
	return env
}

// publishPreviewVersion publishes a web version with real manifest identities.
func (env *previewEnv) publishPreviewVersion(t *testing.T, ws, run string, entries [][2]string) craft.Version {
	t.Helper()
	files := make([]craft.File, 0, len(entries))
	for _, e := range entries {
		sum := sha256.Sum256([]byte(e[1]))
		files = append(files, craft.File{
			Path: e[0], Ref: env.files.save([]byte(e[1]), e[0]),
			SHA256: hex.EncodeToString(sum[:]), MIME: previewMIME(e[0]), Bytes: int64(len(e[1])),
		})
	}
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	v, err := env.store.Publish(context.Background(), env.scope, craft.Version{
		ID: craft.VersionID(ws, run, digest), WorkspaceID: ws, RunID: run,
		Kind: craft.KindWeb, Files: files,
		Checks: craft.BuildChecks(craft.KindWeb, files, craft.ArtifactEvidence{}),
	})
	require.NoError(t, err)
	return v
}

func previewMIME(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "text/javascript; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// issueTicket requests a ticket for a version through the authenticated
// endpoint and returns the response data object.
func (env *previewEnv) issueTicket(t *testing.T, versionID string) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sessions/sess-1/craft/versions/"+versionID+"/preview", nil)
	env.issue.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "issue body: %s", w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "issue body: %s", w.Body.String())
	require.True(t, strings.HasPrefix(data["url"].(string), "https://preview.test/p/"), "ticket URL %v", data["url"])
	return data
}

// previewGet performs one request on the isolated origin.
func (env *previewEnv) previewGet(path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	env.preview.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

// redeem exchanges a ticket URL into the capability path it redirects to.
func (env *previewEnv) redeem(t *testing.T, ticketURL string) string {
	t.Helper()
	u, err := url.Parse(ticketURL)
	require.NoError(t, err)
	w := env.previewGet(u.Path)
	require.Equal(t, http.StatusFound, w.Code, "redeem body: %s", w.Body.String())
	loc := w.Header().Get("Location")
	require.True(t, strings.HasPrefix(loc, "/p/"), "redirect %q", loc)
	return loc
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

// TestCraftPreviewIssueRedeemServe walks the full closed loop: issuance on the
// authenticated main origin, one-time redemption on the isolated origin, and
// relative resources served under the inherited capability path with the full
// security header set.
func TestCraftPreviewIssueRedeemServe(t *testing.T) {
	env := newPreviewEnv(t)
	version := env.publishPreviewVersion(t, "ws-1", "run-1", [][2]string{
		{"index.html", "<html><link rel=stylesheet href=style.css><script src=app.js></script></html>"},
		{"style.css", "body{color:#123}"},
		{"app.js", "console.log(1)"},
	})

	ticket := env.issueTicket(t, version.ID)
	require.Equal(t, version.ID, ticket["version_id"])
	ticketURL := ticket["url"].(string)

	capPath := env.redeem(t, ticketURL)
	base := strings.TrimSuffix(capPath, "index.html")

	// Entry file with backend-verified MIME and the preview header set.
	w := env.previewGet(capPath)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	require.Contains(t, w.Body.String(), "<html>")
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	require.Contains(t, w.Header().Get("Content-Security-Policy"), "connect-src 'none'")
	require.Contains(t, w.Header().Get("Content-Security-Policy"), "worker-src 'none'")
	require.Contains(t, w.Header().Get("Content-Security-Policy"), "frame-ancestors https://app.test")
	require.Equal(t, "same-origin", w.Header().Get("Cross-Origin-Resource-Policy"))

	// Relative resources inherit the capability path.
	w = env.previewGet(base + "style.css")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "text/css; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "body{color:#123}", w.Body.String())
	w = env.previewGet(base + "app.js")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "text/javascript; charset=utf-8", w.Header().Get("Content-Type"))

	// The ticket no longer exists after its one redemption.
	u, err := url.Parse(ticketURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, env.previewGet(u.Path).Code)
}

// TestCraftPreviewIssueCrossTenantNotFound pins the scope ACL: a version
// outside the caller's tenant does not exist for the caller, and a session
// the workspace is not bound to answers 404 over HTTP too.
func TestCraftPreviewIssueCrossTenantNotFound(t *testing.T) {
	env := newPreviewEnv(t)
	version := env.publishPreviewVersion(t, "ws-1", "run-1", [][2]string{{"index.html", "<h1>x</h1>"}})

	foreign := env.scope
	foreign.TenantID = 43
	_, err := env.service.Issue(context.Background(), foreign, version.ID)
	require.ErrorIs(t, err, craft.ErrNotFound)

	otherSession := env.scope
	otherSession.SessionID = "sess-other"
	_, err = env.service.Issue(context.Background(), otherSession, version.ID)
	require.ErrorIs(t, err, craft.ErrNotFound)

	// Over HTTP the issuance endpoint maps the same store answer to 404.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sessions/sess-x/craft/versions/"+version.ID+"/preview", nil)
	env.issue.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
}

// TestCraftPreviewExpiredGrantsReturn404 pins the short-lived grants: an
// expired ticket and an expired capability both 404, and refresh means
// re-authorizing on the main origin.
func TestCraftPreviewExpiredGrantsReturn404(t *testing.T) {
	env := newPreviewEnv(t)
	version := env.publishPreviewVersion(t, "ws-1", "run-1", [][2]string{{"index.html", "<h1>x</h1>"}})

	ticket := env.issueTicket(t, version.ID)
	env.clock.advance(craft.PreviewTicketTTL + time.Second)
	u, err := url.Parse(ticket["url"].(string))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, env.previewGet(u.Path).Code)

	// A capability minted now dies at its own deadline too.
	env.clock.advance(-time.Minute) // back inside a fresh window
	ticket = env.issueTicket(t, version.ID)
	capPath := env.redeem(t, ticket["url"].(string))
	env.clock.advance(craft.PreviewTicketTTL + time.Second)
	require.Equal(t, http.StatusNotFound, env.previewGet(capPath).Code)

	// Re-issuance on the main origin works again after expiry.
	ticket = env.issueTicket(t, version.ID)
	capPath = env.redeem(t, ticket["url"].(string))
	require.Equal(t, http.StatusOK, env.previewGet(capPath).Code)
}

// TestCraftPreviewRejectsTraversal pins the serving-side path defenses:
// dot-dot segments, absolute escapes and smuggled encodings all 404.
func TestCraftPreviewRejectsTraversal(t *testing.T) {
	env := newPreviewEnv(t)
	version := env.publishPreviewVersion(t, "ws-1", "run-1", [][2]string{{"index.html", "<h1>x</h1>"}})
	ticket := env.issueTicket(t, version.ID)
	capPath := env.redeem(t, ticket["url"].(string))
	token := strings.Split(strings.TrimPrefix(capPath, "/p/"), "/")[0]

	// Direct service-level rejections of the smuggle classes.
	for _, bad := range []string{"../secret", "a/../../etc/passwd", "a%2e%2e%2fb", "//etc/passwd", "a\b"} {
		_, err := env.service.Resolve(context.Background(), token, bad)
		require.Error(t, err, "Resolve accepted %q", bad)
	}

	// Handler-level: still 404, never a redirect or a directory escape.
	for _, path := range []string{
		"/p/" + token + "/../secret",
		"/p/" + token + "/a/../../etc/passwd",
		"/p/" + token + "/%2e%2e/%2e%2e/etc/passwd",
		"/p/" + token + "/%252e%252e/secret",
	} {
		require.Equal(t, http.StatusNotFound, env.previewGet(path).Code, "path %q", path)
	}

	// Unknown capabilities never existed as far as the origin is concerned.
	require.Equal(t, http.StatusNotFound, env.previewGet("/p/bogus-token/index.html").Code)
	require.Equal(t, http.StatusNotFound, env.previewGet("/p/"+token+"/not-in-manifest.css").Code)
}

// TestCraftPreviewServesOnlyTheBoundVersion pins immutability: a capability
// for v1 serves v1's pinned bytes, and files that only exist in a NEWER
// version of the same workspace stay 404 through it.
func TestCraftPreviewServesOnlyTheBoundVersion(t *testing.T) {
	env := newPreviewEnv(t)
	v1 := env.publishPreviewVersion(t, "ws-1", "run-1", [][2]string{{"index.html", "<h1>v1</h1>"}})
	env.publishPreviewVersion(t, "ws-1", "run-2", [][2]string{
		{"index.html", "<h1>v2</h1>"},
		{"style.css", "body{}"},
	})

	ticket := env.issueTicket(t, v1.ID)
	capPath := env.redeem(t, ticket["url"].(string))

	w := env.previewGet(capPath)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "v1") // pinned bytes, not the newest version's

	// style.css exists only in v2: the v1 capability cannot reach it.
	base := strings.TrimSuffix(capPath, "index.html")
	require.Equal(t, http.StatusNotFound, env.previewGet(base+"style.css").Code)

	// The workspace lists both versions; only the bound one is addressable.
	list, err := env.store.List(context.Background(), env.scope)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

// TestCraftPreviewUnsupportedKindRefused pins that app-like kinds are refused
// at issuance: the preview never becomes a proxy for backing services.
func TestCraftPreviewUnsupportedKindRefused(t *testing.T) {
	env := newPreviewEnv(t)
	sum := sha256.Sum256([]byte("{}"))
	files := []craft.File{{
		Path: "app.json", Ref: env.files.save([]byte("{}"), "app.json"),
		SHA256: hex.EncodeToString(sum[:]), MIME: "application/json", Bytes: 2,
	}}
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	appish := craft.Version{
		ID: craft.VersionID("ws-1", "run-1", digest), WorkspaceID: "ws-1", RunID: "run-1",
		Kind: "app", Files: files, Checks: []craft.Check{},
	}
	_, err = env.store.Publish(context.Background(), env.scope, appish)
	require.NoError(t, err)

	_, err = env.service.Issue(context.Background(), env.scope, appish.ID)
	require.ErrorIs(t, err, craft.ErrUnsupported)
	require.Contains(t, err.Error(), "不支持此预览类型")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sessions/sess-1/craft/versions/"+appish.ID+"/preview", nil)
	env.issue.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "不支持此预览类型")
}

// TestCraftPreviewDisabledWhenOriginUnconfigured pins that the feature stays
// off until the preview origin is configured.
func TestCraftPreviewDisabledWhenOriginUnconfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewCraftPreviewService(newPreviewVersionStore(), newPreviewFileStore(), nil, service.CraftPreviewConfig{
		AppOrigin: "https://app.test",
	})
	require.False(t, svc.Enabled())
	_, err := svc.Issue(context.Background(), craft.Scope{TenantID: 1, UserID: "u", SessionID: "s"}, "ver_"+strings.Repeat("a", 64))
	require.ErrorIs(t, err, craft.ErrUnsupported)

	h := NewCraftPreviewHandler(svc)
	r := gin.New()
	RegisterCraftPreviewRoutes(r, h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/p/anything/index.html", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestCraftPreviewVerdictChannelAndEvidence pins the W01-review items: the
// preview check update channel (no re-Publish) and the ArtifactEvidenceSource
// injection point that keeps re-collection truthful and idempotent.
func TestCraftPreviewVerdictChannelAndEvidence(t *testing.T) {
	env := newPreviewEnv(t)
	version := env.publishPreviewVersion(t, "ws-1", "run-1", [][2]string{{"index.html", "<h1>x</h1>"}})
	task := previewTask(env.scope, "ws-1", "run-1")

	// No verdict yet: evidence is empty and the check stays not_run.
	require.Equal(t, craft.ArtifactEvidence{}, env.service.EvidenceSource()(context.Background(), task))

	// A pass without a run is a fact that never happened.
	err := env.service.RecordPreviewVerdict(context.Background(), env.scope, version.ID, false, true)
	require.ErrorIs(t, err, craft.ErrInvalidInput)

	// Recording the real verdict updates exactly the preview check.
	require.NoError(t, env.service.RecordPreviewVerdict(context.Background(), env.scope, version.ID, true, true))
	env.checks.mu.Lock()
	recorded := env.checks.updated[version.ID]
	env.checks.mu.Unlock()
	require.Equal(t, craft.PreviewCheckFromEvidence(true, true), recorded)

	// The evidence source now reports the recorded fact for this run.
	evidence := env.service.EvidenceSource()(context.Background(), task)
	require.True(t, evidence.PreviewRan)
	require.True(t, evidence.PreviewPassed)

	// A second version under the same run means content changed: no verdict
	// is carried into a fresh collection of that run.
	env.publishPreviewVersion(t, "ws-1", "run-1", [][2]string{
		{"index.html", "<h1>y</h1>"},
		{"m.css", "x"},
	})
	require.Equal(t, craft.ArtifactEvidence{}, env.service.EvidenceSource()(context.Background(), task))

	// Issuing a ticket alone never records a verdict.
	env.checks.mu.Lock()
	before := len(env.checks.updated)
	env.checks.mu.Unlock()
	ticket := env.issueTicket(t, version.ID)
	require.NotEmpty(t, ticket["url"].(string))
	env.checks.mu.Lock()
	after := len(env.checks.updated)
	env.checks.mu.Unlock()
	require.Equal(t, before, after)
}

// previewTask builds the delegation task the evidence source receives.
func previewTask(scope craft.Scope, ws, run string) craft.Task {
	return craft.Task{Scope: scope, WorkspaceID: ws,
		Fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: scope.TenantID, RunID: run}, Owner: "worker", Epoch: 1}}
}
