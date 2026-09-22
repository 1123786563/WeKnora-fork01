package session

import (
	stderrors "errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// CraftPreviewHandler serves W02's controlled preview (独立 origin 受控预览)
// through two endpoints on two origins:
//
//   - IssueCraftPreview runs on the MAIN origin behind the normal session
//     auth: the caller's tenant/user/session scope is resolved from the
//     authenticated context and the ticket is handed back in the response
//     body only.
//   - CraftPreviewFile runs on the ISOLATED preview origin with no main-site
//     auth at all: the browser cannot carry the main origin's Authorization
//     header or cookies there (Host-only credential policy), so the opaque,
//     short-lived, version-bound capability path is the entire authorization.
type CraftPreviewHandler struct {
	previews *service.CraftPreviewService
}

// NewCraftPreviewHandler constructs the preview handler. previews may be nil
// or disabled — both endpoints then answer 404/400 without touching anything.
func NewCraftPreviewHandler(previews *service.CraftPreviewService) *CraftPreviewHandler {
	return &CraftPreviewHandler{previews: previews}
}

// RegisterCraftPreviewIssueRoute mounts the authenticated ticket issuance
// endpoint inside the sessions API group; the enclosing group's Viewer+/API
// key guards apply exactly as for the other session routes.
//
//	POST /sessions/:session_id/craft/versions/:version_id/preview
func RegisterCraftPreviewIssueRoute(sessions *gin.RouterGroup, h *CraftPreviewHandler) {
	sessions.POST("/:session_id/craft/versions/:version_id/preview", h.IssueCraftPreview)
}

// RegisterCraftPreviewRoutes mounts the isolated-origin preview endpoint on
// the engine root, BEFORE the global auth middleware — the same shape as the
// sandbox terminal route. The capability path carries its own authorization,
// so no main-site credential is expected or read. Only /p/<token>/<file>
// exists: root-absolute references in generated HTML (e.g. "/style.css")
// address the preview origin root, which has no route and 404s — the version
// prefix cannot be bypassed.
func RegisterCraftPreviewRoutes(r *gin.Engine, h *CraftPreviewHandler) {
	r.GET("/p/:cap/*filepath", h.CraftPreviewFile)
	r.HEAD("/p/:cap/*filepath", h.CraftPreviewFile)
}

// IssueCraftPreview issues one redemption ticket for a published version in
// the authenticated session's scope.
func (h *CraftPreviewHandler) IssueCraftPreview(c *gin.Context) {
	ctx := c.Request.Context()
	if h == nil || h.previews == nil {
		c.Error(apperrors.NewInternalServerError("preview service unavailable"))
		return
	}
	sessionID := secutils.SanitizeForLog(paramSessionID(c))
	versionID := strings.TrimSpace(c.Param("version_id"))
	tenantVal, ok := c.Get(types.TenantIDContextKey.String())
	if !ok || sessionID == "" || versionID == "" {
		c.Error(apperrors.NewUnauthorizedError("tenant, session and version are required"))
		return
	}
	tenantID, ok := tenantVal.(uint64)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("invalid tenant context"))
		return
	}
	userID, _ := types.UserIDFromContext(ctx)
	scope := craft.Scope{TenantID: tenantID, UserID: userID, SessionID: sessionID}

	ticket, err := h.previews.Issue(ctx, scope, versionID)
	if err != nil {
		switch {
		case stderrors.Is(err, craft.ErrInvalidInput):
			c.Error(apperrors.NewBadRequestError(err.Error()))
		case stderrors.Is(err, craft.ErrUnsupported):
			c.Error(apperrors.NewBadRequestError(err.Error()))
		case stderrors.Is(err, craft.ErrNotFound):
			c.Error(apperrors.NewNotFoundError(err.Error()))
		case stderrors.Is(err, craft.ErrForbidden):
			c.Error(apperrors.NewForbiddenError(err.Error()))
		case stderrors.Is(err, craft.ErrBusy):
			c.Error(apperrors.NewServiceUnavailableError(err.Error()))
		default:
			c.Error(apperrors.NewInternalServerError(err.Error()))
		}
		return
	}
	// The ticket URL is the secret; it appears in this authorized response
	// body only — never in a header, redirect, event or log line.
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"url":        ticket.URL,
			"expires_at": ticket.ExpiresAt,
			"version_id": ticket.VersionID,
		},
	})
}

// CraftPreviewFile serves one request on the isolated preview origin: a live
// ticket is redeemed exactly once into a redirect; a capability streams one
// manifest file of its bound version with the full preview header set.
func (h *CraftPreviewHandler) CraftPreviewFile(c *gin.Context) {
	ctx := c.Request.Context()
	if h == nil || h.previews == nil || !h.previews.Enabled() {
		c.Status(http.StatusNotFound)
		return
	}
	token := c.Param("cap")
	requestPath := c.Param("filepath")

	open, err := h.previews.Open(ctx, token, requestPath)
	if err != nil {
		// Unknown grants, expired grants, traversal attempts, smuggled
		// encodings and files outside the bound version's manifest all
		// answer the same 404 — the preview origin never confirms which
		// capability ever existed, and no token ever reaches a log.
		switch {
		case stderrors.Is(err, craft.ErrNotFound),
			stderrors.Is(err, craft.ErrForbidden),
			stderrors.Is(err, craft.ErrInvalidInput),
			stderrors.Is(err, craft.ErrUnsupported):
			c.Status(http.StatusNotFound)
		default:
			c.Status(http.StatusInternalServerError)
		}
		return
	}

	if open.Redirect != "" {
		c.Header("Location", open.Redirect)
		c.Header("Cache-Control", "no-store")
		c.Status(http.StatusFound)
		return
	}
	defer open.Reader.Close()

	// Content-Type is the backend-verified MIME recorded in the version's
	// manifest at collection time — never sniffed from the bytes.
	c.Header("Content-Type", open.File.MIME)
	c.Header("Content-Length", strconv.FormatInt(open.File.Bytes, 10))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Security-Policy", h.previews.PreviewCSP())
	c.Header("Cache-Control", "private, no-store")
	c.Header("Cross-Origin-Resource-Policy", "same-origin")
	c.Header("Cross-Origin-Opener-Policy", "same-origin")
	c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=(), display-capture=()")
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, open.Reader); err != nil {
		// Headers are already sent; the short write surfaces as a truncated
		// response and the client retries through the main origin.
		_ = err
	}
}
