package handler

import (
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	filesvc "github.com/Tencent/WeKnora/internal/application/service/file"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/airesource/storageurl"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// -----------------------------------------------------------------------------
// W27: isolated artifact preview (隔离预览).
//
// Generated artifacts (HTML pages, diagrams, code) render on an ISOLATED
// preview origin that shares nothing with the product origin: no product
// cookies, no Authorization header, no API-key capability. The handler pair
// mirrors the craft controlled-preview shape (W02) on top of the W26
// immutable artifact version store:
//
//   - IssueArtifactPreviewTicket runs on the MAIN origin inside the sessions
//     API group. The full W26 authorization re-runs on every issuance:
//     GetSession ownership (404 covers missing/foreign), then the
//     (tenant, session, ready) version triple. The opaque short-lived ticket
//     is returned in the authorized response body only.
//   - ArtifactPreviewFile runs on the ISOLATED origin with no main-site auth:
//     the ticket is the entire authorization. Redemption re-validates that
//     the bound version is still readable, then streams the object with the
//     hardened preview policy headers below.
//
// The origin never proxies a client-supplied URL and never fetches internal
// network addresses: the only thing it knows how to address is the ticket,
// which resolves exclusively to a version row the store already scoped.
// -----------------------------------------------------------------------------

// ArtifactPreviewTicketTTL bounds every preview ticket. Five minutes covers a
// working preview round; anything longer turns a leaked URL into a standing
// grant. Refreshing always goes back through the main origin's authenticated
// issuance, where revocation is re-checked (craft preview parity).
const ArtifactPreviewTicketTTL = 5 * time.Minute

// maxArtifactPreviewGrants bounds the in-memory ticket table (craft parity:
// maxCraftPreviewGrants). Expired-but-never-redeemed tickets are only
// reclaimable through the issuance-time sweep, so without a cap a runaway
// issuer could convert authenticated requests into unbounded memory; a full
// table refuses new tickets instead of evicting live ones.
const maxArtifactPreviewGrants = 1 << 16

// ArtifactPreviewOriginEnv configures the isolated preview origin (a bare
// https origin, no path). Unset or invalid values disable the feature
// fail-closed: issuance answers 400 and the preview path 404s.
const ArtifactPreviewOriginEnv = "WEKNORA_ARTIFACT_PREVIEW_ORIGIN"

// artifactPreviewPathSegment is the URL path prefix every preview is served
// under: /ap/<opaque-token>. Nothing else on the preview origin exists.
const artifactPreviewPathSegment = "ap"

// ArtifactPreviewCSP is served on EVERY preview response. It must stay
// byte-identical to the mobile client's PREVIEW_CSP constant
// (apps/mobile/sources/weknora/resources/preview-policy.ts); both suites pin
// the same directives so drift is caught on either side.
//
//	connect-src 'none'     — no fetch/XHR/WebSocket to ANY endpoint: the
//	                         preview cannot reach the product API or exfiltrate.
//	sandbox allow-scripts  — scripts run (diagrams need them) inside an opaque
//	                         origin: no allow-same-origin means cookies,
//	                         localStorage, sessionStorage and IndexedDB are all
//	                         unreadable, and the document cannot postMessage to
//	                         a same-origin receiver it never had.
//	form-action 'none'     — form submission is blocked.
//	frame-ancestors 'none' — the preview cannot be embedded anywhere.
const ArtifactPreviewCSP = "default-src 'none'; " +
	"script-src 'unsafe-inline'; " +
	"style-src 'unsafe-inline' data:; " +
	"img-src data: blob:; " +
	"font-src data:; " +
	"connect-src 'none'; " +
	"object-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'none'; " +
	"frame-ancestors 'none'; " +
	"worker-src 'none'; " +
	"sandbox allow-scripts"

// previewableArtifactMIME is the allowlist the preview origin serves inline.
// Anything else (archives, binaries, unknown types) stays on the W26 download
// endpoint — the isolated origin never sniffs and never serves opaque bytes
// as active content.
var previewableArtifactMIME = map[string]bool{
	"text/html":        true,
	"text/plain":       true,
	"text/markdown":    true,
	"text/csv":         true,
	"application/json": true,
	"image/svg+xml":    true,
	"image/png":        true,
	"image/jpeg":       true,
	"image/gif":        true,
	"image/webp":       true,
	"application/pdf":  true,
}

// normalizePreviewMIME lowercases and strips parameters ("text/html;
// charset=utf-8" -> "text/html"); empty results mean "not previewable".
func normalizePreviewMIME(mime string) string {
	if idx := strings.IndexByte(mime, ';'); idx >= 0 {
		mime = mime[:idx]
	}
	return strings.ToLower(strings.TrimSpace(mime))
}

// artifactPreviewGrant is one live ticket, tracked under its token digest so
// a memory or log disclosure cannot resurrect the grant.
type artifactPreviewGrant struct {
	tenantID  uint64
	sessionID string
	versionID string
	expiresAt time.Time
}

// ArtifactPreviewHandler serves the isolated artifact preview through two
// endpoints on two origins (see the package comment above).
type ArtifactPreviewHandler struct {
	sessions      interfaces.SessionService
	tenants       interfaces.TenantService
	files         interfaces.FileService
	storage       interfaces.StorageBackendResolver
	versions      sessionhandler.ArtifactVersionSource
	previewOrigin string
	ttl           time.Duration
	now           func() time.Time
	// maxGrants caps len(tickets); only tests lower it from the production
	// maxArtifactPreviewGrants default.
	maxGrants int

	mu      sync.Mutex
	tickets map[string]artifactPreviewGrant
}

// NewArtifactPreviewHandler constructs the production handler: the isolated
// origin comes from ArtifactPreviewOriginEnv and an unset/invalid origin
// leaves the feature disabled (fail-closed, craft parity).
func NewArtifactPreviewHandler(
	sessions interfaces.SessionService,
	tenants interfaces.TenantService,
	files interfaces.FileService,
	storage interfaces.StorageBackendResolver,
	versions sessionhandler.ArtifactVersionSource,
) *ArtifactPreviewHandler {
	return newArtifactPreviewHandler(sessions, tenants, files, storage, versions,
		validPreviewOrigin(os.Getenv(ArtifactPreviewOriginEnv)), ArtifactPreviewTicketTTL, time.Now)
}

// newArtifactPreviewHandler is the test seam: explicit origin, TTL and clock.
func newArtifactPreviewHandler(
	sessions interfaces.SessionService,
	tenants interfaces.TenantService,
	files interfaces.FileService,
	storage interfaces.StorageBackendResolver,
	versions sessionhandler.ArtifactVersionSource,
	origin string,
	ttl time.Duration,
	now func() time.Time,
) *ArtifactPreviewHandler {
	return &ArtifactPreviewHandler{
		sessions:      sessions,
		tenants:       tenants,
		files:         files,
		storage:       storage,
		versions:      versions,
		previewOrigin: validPreviewOrigin(origin),
		ttl:           ttl,
		now:           now,
		maxGrants:     maxArtifactPreviewGrants,
		tickets:       map[string]artifactPreviewGrant{},
	}
}

// putTicket stores one ticket grant, purging expired entries first and
// refusing when the table is at capacity — the craft putGrant semantics. A
// never-redeemed ticket is only reclaimable here: redemption deletes lazily,
// so without this sweep expired grants would accumulate for the process
// lifetime; the cap turns a runaway issuer into visible 503s instead of
// unbounded memory.
func (h *ArtifactPreviewHandler) putTicket(digest string, grant artifactPreviewGrant) error {
	now := h.now()
	h.mu.Lock()
	defer h.mu.Unlock()
	for d, g := range h.tickets {
		if now.After(g.expiresAt) {
			delete(h.tickets, d)
		}
	}
	if len(h.tickets) >= h.maxGrants {
		return fmt.Errorf("artifact preview: too many live grants")
	}
	h.tickets[digest] = grant
	return nil
}

// validPreviewOrigin accepts only a bare https origin (no userinfo, path,
// query or fragment). Everything else disables the feature.
func validPreviewOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return ""
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Scheme != "https" ||
		u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// Enabled reports whether the isolated preview origin is configured.
func (h *ArtifactPreviewHandler) Enabled() bool {
	return h != nil && h.previewOrigin != ""
}

// registeredArtifactPreviewHandler is installed by the container assembly
// (internal/container) and consumed by routes_chat.go / router.go at
// mounting time — the same fail-closed registration pattern as the W26
// versioned download handler and the craft routes.
var registeredArtifactPreviewHandler *ArtifactPreviewHandler

// RegisterArtifactPreviewHandler installs the W27 isolated preview handler
// for route mounting.
func RegisterArtifactPreviewHandler(h *ArtifactPreviewHandler) {
	registeredArtifactPreviewHandler = h
}

// RegisteredArtifactPreviewHandler returns the registered handler (nil when
// the assembly is not wired — then no preview route is mounted at all).
func RegisteredArtifactPreviewHandler() *ArtifactPreviewHandler {
	return registeredArtifactPreviewHandler
}

// artifactPreviewRouteGroup is the route-mounting subset satisfied by both a
// raw gin group and the router's API-key-policy wrapper, so the issue route
// can be mounted through whichever wrapper declares its auth policy (craft
// route parity).
type artifactPreviewRouteGroup interface {
	POST(string, ...gin.HandlerFunc) gin.IRoutes
}

// RegisterArtifactPreviewIssueRoute mounts the authenticated ticket issuance
// endpoint inside the sessions API group; the enclosing group's Viewer+ /
// API-key guards apply exactly as for the other session routes.
//
//	POST /sessions/:session_id/artifact-versions/:version_id/preview-ticket
func RegisterArtifactPreviewIssueRoute(sessions artifactPreviewRouteGroup, h *ArtifactPreviewHandler) {
	if h == nil {
		return
	}
	sessions.POST("/:session_id/artifact-versions/:version_id/preview-ticket", h.IssueArtifactPreviewTicket)
}

// RegisterArtifactPreviewRoutes mounts the isolated-origin preview endpoint
// on the engine root, BEFORE the global auth middleware — the same shape as
// the craft controlled preview and the sandbox terminal routes. The opaque
// ticket carries its own authorization, so no main-site credential is
// expected or read. Only /ap/<token> exists (plus HEAD for prefetch); a nil
// handler (preview not assembled) mounts nothing.
func RegisterArtifactPreviewRoutes(r *gin.Engine, h *ArtifactPreviewHandler) {
	if h == nil {
		return
	}
	r.GET("/"+artifactPreviewPathSegment+"/:token", h.ArtifactPreviewFile)
	r.HEAD("/"+artifactPreviewPathSegment+"/:token", h.ArtifactPreviewFile)
}

// previewParamSessionID resolves the session-id URL parameter across the
// route trees (POST binds :session_id, GET binds :id).
func previewParamSessionID(c *gin.Context) string {
	if v := c.Param("session_id"); v != "" {
		return v
	}
	return c.Param("id")
}

// IssueArtifactPreviewTicket mints one short-lived redemption ticket for a
// published artifact version in the authenticated session's scope. The ticket
// URL is the secret: it appears in this authorized response body only —
// never in a header, cookie, redirect, event or log line.
//
//	POST /sessions/{session_id}/artifact-versions/{version_id}/preview-ticket
func (h *ArtifactPreviewHandler) IssueArtifactPreviewTicket(c *gin.Context) {
	ctx := c.Request.Context()
	if h == nil || !h.Enabled() {
		c.Error(apperrors.NewBadRequestError("isolated artifact preview origin is not configured"))
		return
	}
	sessionID := secutils.SanitizeForLog(previewParamSessionID(c))
	versionID := secutils.SanitizeForLog(c.Param("version_id"))
	if sessionID == "" || versionID == "" {
		c.Error(apperrors.NewBadRequestError("session_id and version_id are required"))
		return
	}
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		c.Error(apperrors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Ownership check: identical to the W26 download handle — GetSession
	// returns ErrSessionNotFound when the session is outside the calling
	// tenant/user, so a 404 covers both "missing" and "revoked" without
	// leaking existence. This is also the gate that refuses NEW tickets
	// after the resource permission was revoked.
	if _, err := h.sessions.GetSession(ctx, sessionID); err != nil {
		if stderrors.Is(err, apperrors.ErrSessionNotFound) {
			c.Error(apperrors.NewNotFoundError(err.Error()))
			return
		}
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	if h.versions == nil {
		c.Error(apperrors.NewServiceUnavailableError("artifact versions unavailable"))
		return
	}
	// Only published (ready) versions scoped to this tenant and session are
	// previewable; everything else is a 404 with no state disclosure.
	version, err := h.versions.ReadableArtifactVersion(ctx, tenantID, sessionID, versionID)
	if err != nil {
		_ = c.Error(apperrors.NewNotFoundError("artifact version not accessible"))
		return
	}
	if !previewableArtifactMIME[normalizePreviewMIME(version.MIME)] {
		c.Error(apperrors.NewBadRequestError("artifact type does not support isolated preview; use the versioned download"))
		return
	}

	token, digest, err := newArtifactPreviewToken()
	if err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	expiresAt := h.now().Add(h.ttl)
	if err := h.putTicket(digest, artifactPreviewGrant{tenantID: tenantID, sessionID: sessionID, versionID: version.ID, expiresAt: expiresAt}); err != nil {
		// Craft parity: a full table answers ErrBusy/503 — a runaway issuer
		// surfaces as visible failures, never as unbounded memory.
		c.Error(apperrors.NewServiceUnavailableError("artifact preview ticket capacity exhausted"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"url":        h.previewOrigin + "/" + artifactPreviewPathSegment + "/" + token,
			"expires_at": expiresAt,
			"version_id": version.ID,
		},
	})
}

// ArtifactPreviewFile serves one request on the isolated preview origin: a
// live ticket whose bound version is still readable streams the object bytes
// with the full preview policy headers. Expired tickets, unknown or tampered
// tokens, and versions that stopped being readable all answer the same 404 —
// the preview origin never confirms which ticket ever existed.
//
//	GET /ap/{token}
func (h *ArtifactPreviewHandler) ArtifactPreviewFile(c *gin.Context) {
	if h == nil || !h.Enabled() || h.versions == nil || h.files == nil {
		c.Status(http.StatusNotFound)
		return
	}
	ctx := c.Request.Context()
	token := c.Param("token")
	digest := artifactPreviewTokenDigest(token)

	h.mu.Lock()
	grant, ok := h.tickets[digest]
	if ok && h.now().After(grant.expiresAt) {
		delete(h.tickets, digest) // expired is indistinguishable from unknown
		ok = false
	}
	if !ok {
		h.mu.Unlock()
		c.Status(http.StatusNotFound)
		return
	}
	h.mu.Unlock()

	// Redemption re-validation: the version must still be readable in the
	// grant's (tenant, session) scope. A resource pulled out of the readable
	// set after issuance also drops the ticket, so retries stay refused.
	version, err := h.versions.ReadableArtifactVersion(ctx, grant.tenantID, grant.sessionID, grant.versionID)
	if err != nil {
		h.mu.Lock()
		delete(h.tickets, digest)
		h.mu.Unlock()
		c.Status(http.StatusNotFound)
		return
	}

	// Resolve the owning tenant's storage, mirroring the W26 download
	// handle's backend selection. Grants are session-scoped to the issuing
	// tenant, so the execution tenant is that tenant.
	ctx = types.WithExecutionTenant(ctx, grant.tenantID)
	fileService := h.files
	if h.tenants != nil {
		tenant, lookupErr := h.tenants.GetTenantByID(ctx, grant.tenantID)
		if lookupErr != nil || tenant == nil {
			c.Status(http.StatusNotFound)
			return
		}
		backendID, providerPath, scoped := types.ParseStorageBackendPath(version.ObjectKey)
		if !scoped {
			providerPath = version.ObjectKey
		}
		var resolveOK bool
		fileService, _, resolveOK = filesvc.ResolveTenantFileServiceWithFallback(
			ctx,
			"artifact preview",
			tenant,
			backendID,
			types.ParseProviderScheme(providerPath),
			storageurl.LocalStorageBaseDir(),
			h.storage,
			h.files,
		)
		if !resolveOK {
			c.Status(http.StatusNotFound)
			return
		}
	}
	reader, err := fileService.GetFile(ctx, version.ObjectKey)
	if err != nil {
		logger.Warnf(ctx, "artifact preview read failed: version=%s err=%v", grant.versionID, err)
		c.Status(http.StatusNotFound)
		return
	}

	// The hardened preview policy — identical constants to the mobile
	// client's preview-policy.ts. Content-Type comes from the
	// server-validated version row, never sniffed from the bytes.
	c.Header("Content-Type", normalizePreviewMIME(version.MIME))
	c.Header("Content-Disposition", "inline")
	c.Header("Content-Security-Policy", ArtifactPreviewCSP)
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-store")
	c.Header("Cross-Origin-Resource-Policy", "same-origin")
	c.Header("Cross-Origin-Opener-Policy", "same-origin")
	c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=(), display-capture=()")
	if version.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(version.Size, 10))
	}
	c.Status(http.StatusOK)
	if c.Request.Method == http.MethodHead {
		_ = reader.Close()
		return
	}
	if _, err := io.Copy(c.Writer, newBoundedPreviewReader(reader, version.Size)); err != nil {
		// Headers are already sent; the short write surfaces as a truncated
		// response and the client re-authorizes on the main origin.
		logger.Warnf(ctx, "artifact preview stream failed: version=%s err=%v", grant.versionID, err)
	}
	_ = reader.Close()
}

// newArtifactPreviewToken mints one 256-bit random opaque token with the
// digest under which it is tracked (craft preview parity).
func newArtifactPreviewToken() (token, digest string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("mint artifact preview token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, artifactPreviewTokenDigest(token), nil
}

func artifactPreviewTokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// boundedPreviewReader caps the stream at the version row's recorded size;
// an overrun is dropped because the pinned row promised exactly these bytes.
type boundedPreviewReader struct {
	inner     io.Reader
	remaining int64
}

func newBoundedPreviewReader(inner io.Reader, n int64) *boundedPreviewReader {
	if n < 0 {
		n = 0
	}
	return &boundedPreviewReader{inner: inner, remaining: n}
}

func (b *boundedPreviewReader) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.inner.Read(p)
	b.remaining -= int64(n)
	return n, err
}
