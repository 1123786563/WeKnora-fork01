package session

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	filesvc "github.com/Tencent/WeKnora/internal/application/service/file"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/filetransport"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/airesource/storageurl"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/modules/policy/access"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// paramSessionID resolves the session-id URL parameter regardless of which
// wildcard name the current route uses. GET-tree routes bind :id (to align
// with /sessions/:id), while POST-tree routes typically bind :session_id.
// Handlers call this helper instead of hard-coding one name so the same
// function serves both trees.
func paramSessionID(c *gin.Context) string {
	if v := c.Param("session_id"); v != "" {
		return v
	}
	return c.Param("id")
}

// ListSessionArtifacts godoc
// @Summary      列出会话生成的产物文件
// @Description  返回本会话中所有 assistant 消息产生的技能产物元数据（不含 URL）
// @Tags         会话
// @Produce      json
// @Param        session_id  path  string  true  "会话ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  errors.AppError
// @Security     Bearer
// @Router       /sessions/{session_id}/artifacts [get]
//
// The endpoint powers the drawer that lists every file generated in the
// session; it does NOT return the storage URL (only names/sizes/mtimes), so
// clients cannot reach around the download endpoint by reading a
// provider:// path from the API response.
//
// Deleted artifacts are skipped but still consume their index: the index IS the
// download address, so renumbering around a tombstone would point old links at
// the wrong file.
func (h *Handler) ListSessionArtifacts(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := secutils.SanitizeForLog(paramSessionID(c))
	if sessionID == "" {
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	// Ownership + tenant check: GetSession enforces both. Returning 404 for
	// unknown / non-owned sessions matches the rest of the session routes.
	if _, err := h.sessionService.GetSession(ctx, sessionID); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	artifacts, err := h.messageService.GetSessionArtifacts(ctx, sessionID)
	if err != nil {
		logger.Errorf(ctx, "list session artifacts failed: session=%s err=%v", sessionID, err)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	items := make([]artifactListItem, 0, len(artifacts))
	for i, a := range artifacts {
		if a.Deleted() {
			continue
		}
		items = append(items, artifactListItem{
			Index:      i,
			Handle:     artifactHandle(a),
			FileName:   a.FileName,
			FileType:   a.FileType,
			FileSize:   a.FileSize,
			SourcePath: a.SourcePath,
			ModTime:    a.ModTime,
			CreatedAt:  a.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
	})
}

// ListMessageArtifacts returns just the artifacts attached to a single
// assistant message. Used by the "download files from this reply" button on
// each bot message.
//
// Same-tenant/same-owner check flows through h.sessionService.GetSession
// exactly like ListSessionArtifacts.
//
// @Router /sessions/{session_id}/messages/{message_id}/artifacts [get]
func (h *Handler) ListMessageArtifacts(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := secutils.SanitizeForLog(paramSessionID(c))
	messageID := secutils.SanitizeForLog(c.Param("message_id"))
	if sessionID == "" || messageID == "" {
		c.Error(errors.NewBadRequestError("session_id and message_id are required"))
		return
	}

	if _, err := h.sessionService.GetSession(ctx, sessionID); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	msg, err := h.messageService.GetMessage(ctx, sessionID, messageID)
	if err != nil || msg == nil {
		c.Error(errors.NewNotFoundError("message not found"))
		return
	}

	items := make([]artifactListItem, 0, len(msg.Artifacts))
	for i, a := range msg.Artifacts {
		if a.Deleted() {
			continue
		}
		items = append(items, artifactListItem{
			Index:      i,
			Handle:     artifactHandle(a),
			FileName:   a.FileName,
			FileType:   a.FileType,
			FileSize:   a.FileSize,
			SourcePath: a.SourcePath,
			ModTime:    a.ModTime,
			CreatedAt:  a.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
	})
}

// DownloadMessageArtifact streams a single skill-generated file to the
// client. Clients reference the artifact by its position (:index) in the
// assistant message's Artifacts array; the storage URL never leaves the
// server so callers cannot pivot to arbitrary blobs.
//
// @Router /sessions/{session_id}/messages/{message_id}/artifacts/{index}/download [get]
func (h *Handler) DownloadMessageArtifact(c *gin.Context) {
	ctx := c.Request.Context()

	sessionID := secutils.SanitizeForLog(paramSessionID(c))
	messageID := secutils.SanitizeForLog(c.Param("message_id"))
	indexParam := c.Param("index")
	if sessionID == "" || messageID == "" || indexParam == "" {
		c.Error(errors.NewBadRequestError("session_id, message_id and index are required"))
		return
	}
	index, err := strconv.Atoi(indexParam)
	if err != nil || index < 0 {
		c.Error(errors.NewBadRequestError("invalid artifact index"))
		return
	}

	// Ownership check: GetSession returns ErrSessionNotFound when the
	// session doesn't belong to the calling tenant/user, so a 404 covers
	// both "not found" and "forbidden" without leaking existence.
	if _, err := h.sessionService.GetSession(ctx, sessionID); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	msg, err := h.messageService.GetMessage(ctx, sessionID, messageID)
	if err != nil || msg == nil {
		c.Error(errors.NewNotFoundError("message not found"))
		return
	}
	if index >= len(msg.Artifacts) {
		_ = c.Error(errors.NewNotFoundError("artifact index out of range"))
		return
	}
	artifact := msg.Artifacts[index]
	if artifact.Deleted() {
		_ = c.Error(errors.NewNotFoundError("artifact deleted"))
		return
	}
	if artifact.URL == "" {
		_ = c.Error(errors.NewNotFoundError("artifact storage path missing"))
		return
	}
	h.streamResolvedArtifact(c, ctx, msg, index, artifact, sessionID)
}

// streamResolvedArtifact resolves a message artifact to a file and streams
// the bytes. Shared by the authenticated per-message download and the signed
// workbench grant download so both paths go through the same tenant storage
// resolution and never expose provider:// URLs.
func (h *Handler) streamResolvedArtifact(c *gin.Context, ctx context.Context, msg *types.Message, index int, artifact types.MessageArtifact, logScope string) {
	if h.fileService == nil {
		c.Error(errors.NewInternalServerError("file service unavailable"))
		return
	}
	file, err := access.ResolveMessageArtifact(ctx, msg, index, h.agentShareService, h.resourceCatalog,
		access.MessageKBShareAuthorizer{ShareGuard: h.kbShareService, KBs: h.knowledgebaseService})
	if err != nil {
		_ = c.Error(errors.NewNotFoundError("artifact not accessible"))
		return
	}
	fileService, ctx, ok := h.resolveArtifactFileService(
		ctx, file.OwnerTenantID, file.Path, file.StorageBackendID, "artifact download",
	)
	if !ok {
		_ = c.Error(errors.NewNotFoundError("artifact storage unavailable"))
		return
	}
	reader, err := fileService.GetFile(ctx, file.Path)
	if err != nil {
		logger.Warnf(ctx, "artifact download read failed: session=%s message=%s idx=%d err=%v",
			logScope, msg.ID, index, err)
		_ = c.Error(errors.NewNotFoundError("artifact blob missing"))
		return
	}
	if err := filetransport.Serve(c.Writer, c.Request, reader, filetransport.Options{
		Filename: artifact.FileName, Download: true, ContentType: mimeTypeFor(artifact.FileName),
		Disposition:  buildAttachmentHeader(artifact.FileName),
		Size:         artifact.FileSize,
		CacheControl: "private, no-store",
	}); err != nil {
		logger.Warnf(
			ctx,
			"artifact download stream failed: session=%s message=%s idx=%d err=%v",
			logScope,
			msg.ID,
			index,
			err,
		)
	}
}

// DownloadWorkbenchArtifactGrant streams an artifact behind a short-lived
// HMAC grant minted by the workbench signed-URL endpoint. The route carries
// NO login session: the signature (tenant/session/message/index/expiry) is
// the authorization fact, so verification happens in constant time and an
// expired or tampered link is rejected with 401/404 — never refreshed
// implicitly. Clients re-authorize through the authenticated endpoint.
//
// The grant's tenant is applied as the execution tenant for storage
// resolution; the artifact is re-resolved from current message state so a
// link to a deleted message stops working even before expiry.
//
// @Router /workbench/artifacts/download [get]
func (h *Handler) DownloadWorkbenchArtifactGrant(c *gin.Context) {
	query := c.Request.URL.Query()
	if query.Get("grant_type") == "artifact_version" {
		versionHandler := RegisteredArtifactVersionDownloadHandler()
		if versionHandler == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		versionHandler.DownloadArtifactVersionGrant(c)
		return
	}
	tenantID, tenantErr := strconv.ParseUint(query.Get("tenant_id"), 10, 64)
	index, indexErr := strconv.Atoi(query.Get("index"))
	expiresAt, expErr := strconv.ParseInt(query.Get("expires_at"), 10, 64)
	sessionID := secutils.SanitizeForLog(query.Get("session_id"))
	messageID := secutils.SanitizeForLog(query.Get("message_id"))
	signature := query.Get("signature")
	if tenantErr != nil || indexErr != nil || expErr != nil || sessionID == "" || messageID == "" || signature == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	grant := workbench.ArtifactGrant{
		TenantID:  tenantID,
		SessionID: sessionID,
		MessageID: messageID,
		Index:     index,
		ExpiresAt: expiresAt,
	}
	secret, keyErr := workbench.ArtifactSigningKeyFromEnv()
	if keyErr != nil {
		c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{
			"success": false,
			"code":    "artifact_signing_disabled",
			"error":   "artifact signing key not configured",
		})
		return
	}
	if verifyErr := workbench.VerifyArtifactGrantAt(secret, grant, signature, time.Now()); verifyErr != nil {
		// Expired grants are reported distinctly so the client can offer
		// re-authorization instead of a generic failure.
		code := "artifact_grant_invalid"
		if strings.Contains(verifyErr.Error(), "expired") {
			code = "artifact_grant_expired"
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": code})
		return
	}
	// The caller identity here is the grant tenant — set explicitly via
	// WithCaller so storage scoping works without a login session.
	ctx := types.WithCaller(c.Request.Context(), types.Caller{TenantID: grant.TenantID})
	refs, refsErr := h.messageService.GetSessionArtifactRefs(ctx, grant.SessionID)
	if refsErr != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	var resolved *types.SessionArtifactRef
	for i := range refs {
		if refs[i].MessageID == grant.MessageID && refs[i].Index == grant.Index {
			resolved = &refs[i]
			break
		}
	}
	if resolved == nil || resolved.Artifact.URL == "" {
		// Deleted messages make the grant undeliverable; do not leak why.
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	// Minimal message shell: ResolveMessageArtifact only reads the artifacts
	// array and (for shared-agent fallbacks) message identity. Grants are
	// minted only after run-ownership checks, so the primary tenant-owner
	// path is what signed links exercise.
	msg := &types.Message{ID: resolved.MessageID, SessionID: grant.SessionID, Artifacts: types.MessageArtifacts{resolved.Artifact}}
	h.streamResolvedArtifact(c, ctx, msg, 0, resolved.Artifact, grant.SessionID)
}

// artifactListItem is the JSON shape returned by ListSessionArtifacts /
// ListMessageArtifacts. It carries the resource handle but never the storage
// path: the handle is the artifact's public identity — it is what the answer
// body references and what an authorizing proxy resolves — while the physical
// bucket/key stays server side.
type artifactListItem struct {
	Index int `json:"index"`
	// Handle is the artifact's `resource://<handle>` reference, matching the
	// destinations in the message body. Empty when the deployment runs without
	// a resource catalog, in which case the body references files by name.
	Handle     string `json:"handle,omitempty"`
	FileName   string `json:"file_name"`
	FileType   string `json:"file_type"`
	FileSize   int64  `json:"file_size"`
	SourcePath string `json:"source_path"`
	// time-typed fields serialise as RFC3339 strings — same convention as
	// the rest of the messages API.
	ModTime   any `json:"mod_time"`
	CreatedAt any `json:"created_at"`
}

// mimeTypeFor picks a Content-Type by extension and falls back to
// application/octet-stream so unknown types force a download prompt rather
// than being sniffed. Kept private to this file — the /files route has a
// stricter version with an SVG-neutralising branch; we don't need that here
// because Content-Disposition already blocks inline rendering.
func mimeTypeFor(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return "application/octet-stream"
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// buildAttachmentHeader returns a Content-Disposition value that preserves
// non-ASCII filenames (RFC 5987) while providing a safe fallback for
// ASCII-only clients.
func buildAttachmentHeader(name string) string {
	// Strip control characters + quotes; keep the human-readable name.
	ascii := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		if r == '"' || r == '\\' {
			return '_'
		}
		if r > 0x7e {
			return -1
		}
		return r
	}, name)
	if ascii == "" {
		ascii = "download"
	}
	encoded := (&urlPathEscaper{}).escape(name)
	return "attachment; filename=\"" + ascii + "\"; filename*=UTF-8''" + encoded
}

// urlPathEscaper is a minimal RFC 3986 percent-encoder for the subset of
// bytes allowed in a filename*= value. We inline it to avoid importing
// net/url just for a two-line call, and because url.PathEscape encodes
// spaces as "+" (form-encoding) which HTTP clients then decode as literal
// "+" characters in the filename.
type urlPathEscaper struct{}

// escape percent-encodes every byte outside the "attr-char" grammar of RFC 5987.
// See https://datatracker.ietf.org/doc/html/rfc5987#section-3.2.1
func (urlPathEscaper) escape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	const hex = "0123456789ABCDEF"
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case (c >= 'A' && c <= 'Z'), (c >= 'a' && c <= 'z'), (c >= '0' && c <= '9'):
			b.WriteByte(c)
		case c == '-' || c == '.' || c == '_' || c == '~':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
}

// artifactHandle returns the artifact's `resource://<handle>` reference, or ""
// when the deployment stores artifacts without a resource catalog.
func artifactHandle(artifact types.MessageArtifact) string {
	if handle, ok := types.ParseResourcePath(artifact.URL); ok {
		return types.BuildResourcePath(handle)
	}
	return ""
}

// -----------------------------------------------------------------------------
// Immutable artifact version downloads (W26)
//
// The message-indexed DownloadMessageArtifact above stays unchanged for
// compatibility: legacy clients keep addressing artifacts by their message
// index. Imported execution outputs are addressed instead by an explicit
// version ID on a dedicated handler, so a stale index can never resolve to a
// different file after a message is regenerated.
// -----------------------------------------------------------------------------

// ArtifactVersionSource reads published (scan-state ready) immutable artifact
// versions. The repository's *repository.ArtifactVersionStore satisfies it.
type ArtifactVersionSource interface {
	ReadableArtifactVersion(ctx context.Context, tenantID uint64, sessionID, versionID string) (repository.ArtifactVersion, error)
}

// ArtifactVersionDownloadHandler streams one published artifact version.
// Session ownership is checked exactly like the legacy handle (GetSession
// covers tenant and owner scoping), and the version row is additionally
// scoped to the caller's tenant and session, so an unpublishable (pending,
// quarantined) or foreign-workspace version is indistinguishable from a
// missing one.
type ArtifactVersionDownloadHandler struct {
	sessions interfaces.SessionService
	tenants  interfaces.TenantService
	files    interfaces.FileService
	storage  interfaces.StorageBackendResolver
	versions ArtifactVersionSource
	runs     OwnedRunReader
	members  artifactTenantMembershipReader
}

type artifactTenantMembershipReader interface {
	Get(ctx context.Context, userID string, tenantID uint64) (*types.TenantMember, error)
}

// NewArtifactVersionDownloadHandler constructs the versioned download
// endpoint. A nil version source fails closed on every request.
func NewArtifactVersionDownloadHandler(
	sessions interfaces.SessionService,
	tenants interfaces.TenantService,
	files interfaces.FileService,
	storage interfaces.StorageBackendResolver,
	versions ArtifactVersionSource,
	runs ...OwnedRunReader,
) *ArtifactVersionDownloadHandler {
	h := &ArtifactVersionDownloadHandler{sessions: sessions, tenants: tenants, files: files, storage: storage, versions: versions}
	if len(runs) > 0 {
		h.runs = runs[0]
	}
	return h
}

// WithTenantMembership installs the live membership lookup required by
// credential-free artifact grants. A missing reader denies every grant.
func (h *ArtifactVersionDownloadHandler) WithTenantMembership(members artifactTenantMembershipReader) *ArtifactVersionDownloadHandler {
	h.members = members
	return h
}

// registeredArtifactVersionDownloadHandler is installed by the container
// assembly (internal/container) and consumed by routes_chat.go at mounting
// time — the same fail-closed registration pattern the craft routes use.
var registeredArtifactVersionDownloadHandler *ArtifactVersionDownloadHandler

// RegisterArtifactVersionDownloadHandler installs the W26 versioned download
// handler for route mounting.
func RegisterArtifactVersionDownloadHandler(h *ArtifactVersionDownloadHandler) {
	registeredArtifactVersionDownloadHandler = h
}

// RegisteredArtifactVersionDownloadHandler returns the registered handler
// (nil when the assembly is not wired — then no version route is mounted at
// all, matching the craft mounting pattern).
func RegisteredArtifactVersionDownloadHandler() *ArtifactVersionDownloadHandler {
	return registeredArtifactVersionDownloadHandler
}

// DownloadArtifactVersion streams one immutable artifact version by its
// explicit version ID.
//
// @Router /sessions/{session_id}/artifact-versions/{version_id}/download [get]
func (h *ArtifactVersionDownloadHandler) DownloadArtifactVersion(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := secutils.SanitizeForLog(paramSessionID(c))
	versionID := secutils.SanitizeForLog(c.Param("version_id"))
	if sessionID == "" || versionID == "" {
		c.Error(errors.NewBadRequestError("session_id and version_id are required"))
		return
	}
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		c.Error(errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Ownership check: identical to the legacy handle — GetSession returns
	// ErrSessionNotFound when the session is outside the calling tenant/user,
	// so a 404 covers both "missing" and "forbidden" without leaking existence.
	if _, err := h.sessions.GetSession(ctx, sessionID); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	if h.versions == nil {
		c.Error(errors.NewServiceUnavailableError("artifact versions unavailable"))
		return
	}
	// Only published (ready) versions scoped to this tenant and session are
	// readable; everything else is a 404 with no state disclosure.
	version, err := h.versions.ReadableArtifactVersion(ctx, tenantID, sessionID, versionID)
	if err != nil {
		_ = c.Error(errors.NewNotFoundError("artifact version not accessible"))
		return
	}
	if h.files == nil {
		c.Error(errors.NewInternalServerError("file service unavailable"))
		return
	}

	// Resolve the owning tenant's storage, mirroring the legacy handle's
	// backend selection. Version objects are session-scoped to the caller's
	// tenant, so the execution tenant is the caller's own tenant.
	ctx = types.WithExecutionTenant(ctx, tenantID)
	fileService := h.files
	if h.tenants != nil {
		tenant, lookupErr := h.tenants.GetTenantByID(ctx, tenantID)
		if lookupErr != nil || tenant == nil {
			_ = c.Error(errors.NewNotFoundError("artifact workspace unavailable"))
			return
		}
		backendID, providerPath, scoped := types.ParseStorageBackendPath(version.ObjectKey)
		if !scoped {
			providerPath = version.ObjectKey
		}
		var resolveOK bool
		fileService, _, resolveOK = filesvc.ResolveTenantFileServiceWithFallback(
			ctx,
			"artifact version download",
			tenant,
			backendID,
			types.ParseProviderScheme(providerPath),
			storageurl.LocalStorageBaseDir(),
			h.storage,
			h.files,
		)
		if !resolveOK {
			_ = c.Error(errors.NewNotFoundError("artifact storage unavailable"))
			return
		}
	}
	reader, err := fileService.GetFile(ctx, version.ObjectKey)
	if err != nil {
		logger.Warnf(ctx, "artifact version download read failed: session=%s version=%s err=%v", sessionID, versionID, err)
		_ = c.Error(errors.NewNotFoundError("artifact blob missing"))
		return
	}
	name := artifactVersionFileName(version)
	if err := filetransport.Serve(c.Writer, c.Request, reader, filetransport.Options{
		Filename:     name,
		Download:     true,
		ContentType:  version.MIME,
		Disposition:  buildAttachmentHeader(name),
		Size:         version.Size,
		CacheControl: "private, no-store",
	}); err != nil {
		logger.Warnf(ctx, "artifact version download stream failed: session=%s version=%s err=%v", sessionID, versionID, err)
	}
}

// DownloadArtifactVersionGrant consumes a credential-free signed link. It
// rechecks run ownership and version publication state on every request.
func (h *ArtifactVersionDownloadHandler) DownloadArtifactVersionGrant(c *gin.Context) {
	query := c.Request.URL.Query()
	tenantID, tenantErr := strconv.ParseUint(query.Get("tenant_id"), 10, 64)
	expiresAt, expiryErr := strconv.ParseInt(query.Get("expires_at"), 10, 64)
	grant := workbench.ArtifactVersionGrant{TenantID: tenantID, OwnerID: query.Get("owner_id"), RunID: query.Get("run_id"), SessionID: query.Get("session_id"), VersionID: query.Get("version_id"), ExpiresAt: expiresAt}
	signature := query.Get("signature")
	if tenantErr != nil || expiryErr != nil || signature == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	secret, err := workbench.ArtifactSigningKeyFromEnv()
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if err := workbench.VerifyArtifactVersionGrantAt(secret, grant, signature, time.Now()); err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if h.runs == nil || h.versions == nil || h.members == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	run, err := h.runs.GetOwnedRun(c.Request.Context(), grant.TenantID, grant.OwnerID, grant.RunID)
	if err != nil || run.UserID != grant.OwnerID || run.SessionID != grant.SessionID {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	member, err := h.members.Get(c.Request.Context(), grant.OwnerID, grant.TenantID)
	if err != nil || member == nil || member.Status != types.TenantMemberStatusActive {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	version, err := h.versions.ReadableArtifactVersion(c.Request.Context(), grant.TenantID, grant.SessionID, grant.VersionID)
	if err != nil || version.RunID != grant.RunID {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	// Execute in the artifact tenant only after the signed tenant and owner
	// predicates have been revalidated.
	ctx := types.WithCaller(c.Request.Context(), types.Caller{TenantID: grant.TenantID, UserID: grant.OwnerID})
	ctx = types.WithExecutionTenant(ctx, grant.TenantID)
	h.streamArtifactVersion(c, ctx, grant.TenantID, version)
}

func (h *ArtifactVersionDownloadHandler) streamArtifactVersion(c *gin.Context, ctx context.Context, tenantID uint64, version repository.ArtifactVersion) {
	if h.files == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	fileService := h.files
	if h.tenants != nil {
		tenant, lookupErr := h.tenants.GetTenantByID(ctx, tenantID)
		if lookupErr != nil || tenant == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		backendID, providerPath, scoped := types.ParseStorageBackendPath(version.ObjectKey)
		if !scoped {
			providerPath = version.ObjectKey
		}
		var resolveOK bool
		fileService, _, resolveOK = filesvc.ResolveTenantFileServiceWithFallback(ctx, "artifact version download", tenant, backendID, types.ParseProviderScheme(providerPath), storageurl.LocalStorageBaseDir(), h.storage, h.files)
		if !resolveOK {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}
	reader, err := fileService.GetFile(ctx, version.ObjectKey)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	name := artifactVersionFileName(version)
	if err := filetransport.Serve(c.Writer, c.Request, reader, filetransport.Options{Filename: name, Download: true, ContentType: version.MIME, Disposition: buildAttachmentHeader(name), Size: version.Size, CacheControl: "private, no-store"}); err != nil {
		logger.Warnf(ctx, "artifact version grant stream failed: run=%s version=%s err=%v", version.RunID, version.ID, err)
	}
}

// artifactVersionFileName derives a stable download filename from the version
// identity and its server-validated MIME type.
func artifactVersionFileName(version repository.ArtifactVersion) string {
	ext := ".bin"
	switch strings.ToLower(strings.TrimSpace(version.MIME)) {
	case "text/plain":
		ext = ".txt"
	case "text/csv":
		ext = ".csv"
	case "application/json":
		ext = ".json"
	case "application/pdf":
		ext = ".pdf"
	case "image/png":
		ext = ".png"
	case "image/jpeg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	}
	return "artifact-" + version.ID + ext
}

// -----------------------------------------------------------------------------
// Version-bound source bundle export (T12, #132)
//
// The bundle download projects ONE immutable version: exactly its own
// artifact members (source), its recorded checks (build metadata) and the
// citation/source manifest derived from the version's pinned evidence. The
// service owns every authority check and the audit (member, Version,
// manifest digest); these routes only translate. Member bytes stream from
// the durable object refs through the file reader — never from the mutable
// Workspace.
// -----------------------------------------------------------------------------

// CraftExportAPI is the T12 bundle surface. Its implementation owns all
// current Task membership checks and the export manifest derivation; route
// guards alone are not ACLs.
type CraftExportAPI interface {
	ExportBundle(context.Context, craft.Scope, string) (service.CraftExportBundle, error)
}

// CraftExportHandler translates the bundle projection onto HTTP.
type CraftExportHandler struct {
	svc   CraftExportAPI
	files service.CraftExportFileReader
}

// NewCraftExportHandler constructs the bundle surface. A nil service or
// reader fails closed on every request.
func NewCraftExportHandler(svc CraftExportAPI, files service.CraftExportFileReader) *CraftExportHandler {
	return &CraftExportHandler{svc: svc, files: files}
}

// RegisterCraftExportFeature binds the T12 bundle surface to one router
// assembly's feature registry. Duplicate and post-mount registrations fail
// on that registry without retaining process-global handler state.
func RegisterCraftExportFeature(routes *CraftFeatureRoutes, svc CraftExportAPI, files service.CraftExportFileReader) error {
	if routes == nil || svc == nil || files == nil {
		return stderrors.New("Craft export feature unavailable")
	}
	return routes.Register("export", func(group CraftRouteGroup) {
		RegisterCraftExportRoutes(group, NewCraftExportHandler(svc, files))
	})
}

// MountCraftExportRoutes is called through the T00 constrained feature
// registry at central assembly time.
func (h *CraftExportHandler) MountCraftExportRoutes(group CraftRouteGroup) {
	RegisterCraftExportRoutes(group, h)
}

// RegisterCraftExportRoutes mounts the T12 routes onto one constrained
// feature group.
func RegisterCraftExportRoutes(group CraftRouteGroup, h *CraftExportHandler) {
	group.GET("/:id/craft/versions/:version_id/export", h.GetCraftExportBundle)
	group.GET("/:id/craft/versions/:version_id/export/download", h.DownloadCraftExportBundle)
}

// craftExportBody projects the bundle description: the manifest digest, the
// member list, the citation/source manifest and the build checks. It never
// carries original material — titles and durable refs only.
func craftExportBody(bundle service.CraftExportBundle) gin.H {
	files := make([]gin.H, 0, len(bundle.Manifest.Files))
	for _, f := range bundle.Manifest.Files {
		files = append(files, gin.H{"path": f.Path, "sha256": f.SHA256, "restricted": f.Restricted})
	}
	sources := make([]gin.H, 0, len(bundle.CitationManifest.Sources))
	for _, source := range bundle.CitationManifest.Sources {
		sources = append(sources, gin.H{
			"citation_id": source.CitationID, "ref": source.Ref,
			"digest": source.Digest, "acquired_at": source.AcquiredAt,
			"title": source.Title,
		})
	}
	checks := make([]gin.H, 0, len(bundle.Version.Checks))
	for _, check := range bundle.Version.Checks {
		checks = append(checks, gin.H{"name": check.Name, "status": check.Status, "detail": check.Detail})
	}
	return gin.H{
		"version_id": bundle.Version.ID, "run_id": bundle.Version.RunID, "kind": bundle.Version.Kind,
		"manifest_digest": bundle.Manifest.ManifestDigest,
		"files":           files, "sources": sources, "checks": checks,
		"empty": bundle.CitationManifest.Empty,
	}
}

// GetCraftExportBundle answers the bundle description for a current Task
// member: the digest-bound member list, the citation/source manifest and
// the build metadata.
//
// @Router /sessions/{session_id}/craft/versions/{version_id}/export [get]
func (h *CraftExportHandler) GetCraftExportBundle(c *gin.Context) {
	scope, ok := craftScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	bundle, err := h.svc.ExportBundle(c.Request.Context(), scope, c.Param("version_id"))
	if err != nil {
		craftShareHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, craftExportBody(bundle))
}

// DownloadCraftExportBundle streams the complete source bundle as one zip:
// exactly the version's immutable members plus the three fixed documents
// (export-manifest.json, sources.json, build.json). The service has already
// re-checked Task membership, re-validated every member path and computed
// the manifest digest; the member bytes stream from the durable object refs.
//
// @Router /sessions/{session_id}/craft/versions/{version_id}/export/download [get]
func (h *CraftExportHandler) DownloadCraftExportBundle(c *gin.Context) {
	scope, ok := craftScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}
	if h == nil || h.svc == nil || h.files == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	bundle, err := h.svc.ExportBundle(c.Request.Context(), scope, c.Param("version_id"))
	if err != nil {
		craftShareHTTPError(c, err)
		return
	}

	name := "craft-" + bundle.Version.ID + "-source.zip"
	c.Header("Content-Disposition", buildAttachmentHeader(name))
	c.Header("Content-Type", "application/zip")
	c.Header("Cache-Control", "private, no-store")
	c.Status(http.StatusOK)

	// Mid-stream failure contract: the manifest inside this zip declares
	// the exact member list and their content digests, so a silently
	// truncated-but-valid zip would deliver a "successful" download whose
	// bytes disagree with the manifest. Any failure after the 200 head is
	// sent therefore aborts the connection (the craftegress precedent):
	// the client sees a broken transfer instead of a degraded bundle.
	abortDownload := func(format string, args ...any) {
		logger.Warnf(c.Request.Context(), format, args...)
		panic(http.ErrAbortHandler)
	}

	zipWriter := zip.NewWriter(c.Writer)
	writeDoc := func(path string, payload []byte) {
		entry, err := zipWriter.CreateHeader(&zip.FileHeader{Name: path, Method: zip.Deflate})
		if err != nil {
			abortDownload("craft export bundle document %s create failed: %v", path, err)
			return
		}
		if _, err := entry.Write(payload); err != nil {
			abortDownload("craft export bundle document %s stream failed: %v", path, err)
		}
	}
	manifestJSON, merr := json.Marshal(bundle.Manifest)
	if merr != nil {
		abortDownload("craft export bundle manifest encode failed: %v", merr)
	}
	writeDoc(craft.BundleManifestPath, manifestJSON)
	sourcesJSON, serr := json.Marshal(bundle.CitationManifest)
	if serr != nil {
		abortDownload("craft export bundle sources encode failed: %v", serr)
	}
	writeDoc(craft.BundleSourcesPath, sourcesJSON)
	buildJSON, berr := json.Marshal(bundle.BundleBuildDocument())
	if berr != nil {
		abortDownload("craft export bundle build document encode failed: %v", berr)
	}
	writeDoc(craft.BundleBuildPath, buildJSON)
	for _, member := range bundle.Version.Files {
		// zip-slip variant: a Windows drive-letter first segment (c:/evil)
		// passes forward-slash relative-path validation but older or custom
		// extractors can resolve it as an absolute target outside the
		// extraction directory. Refuse it before the 200 head is written.
		if first := member.Path; len(first) >= 2 && first[1] == ':' &&
			((first[0] >= 'a' && first[0] <= 'z') || (first[0] >= 'A' && first[0] <= 'Z')) {
			abortDownload("craft export bundle member %q uses a drive-letter path", member.Path)
			continue
		}
		reader, err := h.files.GetFile(c.Request.Context(), member.Ref)
		if err != nil {
			abortDownload("craft export bundle member %s read failed: %v", member.Path, err)
			continue
		}
		entry, createErr := zipWriter.CreateHeader(&zip.FileHeader{Name: member.Path, Method: zip.Deflate})
		if createErr != nil {
			reader.Close()
			abortDownload("craft export bundle member %s create failed: %v", member.Path, createErr)
			continue
		}
		// The manifest's digest claim is verified against the ACTUAL bytes
		// as they stream: a corrupted object or a swapped ref truncates the
		// download instead of shipping bytes that disagree with the
		// manifest.
		hasher := sha256.New()
		_, copyErr := io.Copy(io.MultiWriter(entry, hasher), reader)
		reader.Close()
		if copyErr != nil {
			abortDownload("craft export bundle member %s stream failed: %v", member.Path, copyErr)
			continue
		}
		if streamed := hex.EncodeToString(hasher.Sum(nil)); streamed != member.SHA256 {
			abortDownload("craft export bundle member %s digest mismatch: manifest %s, streamed %s",
				member.Path, member.SHA256, streamed)
		}
	}
	if err := zipWriter.Close(); err != nil {
		abortDownload("craft export bundle finalize failed for version %s: %v", bundle.Version.ID, err)
	}
}
