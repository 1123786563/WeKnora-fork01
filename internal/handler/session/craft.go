package session

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/metrics"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// CraftSessionHandler serves W03's craft session HTTP surface. Every route
// inherits the enclosing sessions group's Viewer+/API-key guards; the
// session-level permission chain (owner write, shared read) is enforced inside
// the service the same way the existing session endpoints enforce it.
type CraftSessionHandler struct {
	svc CraftSessionAPI
}

// CraftSessionAPI is the handler's view of the craft session service. The
// concrete *service.CraftSessionService satisfies it.
type CraftSessionAPI interface {
	Create(context.Context, craft.Scope, craft.CreateRequest) (craft.Workspace, error)
	View(context.Context, craft.Scope) (service.CraftWorkspaceView, error)
	List(context.Context, craft.Scope, string, int) ([]service.CraftSessionSummary, string, error)
	AssociateInput(context.Context, craft.Scope, string, string) (craft.Input, error)
	StartRun(context.Context, craft.Scope, service.CraftRunRequest) (agentruntime.Run, error)
	ListVersions(context.Context, craft.Scope) ([]craft.Version, error)
	GetVersion(context.Context, craft.Scope, string) (craft.Version, error)
	OpenVersionFile(context.Context, craft.Scope, string, string) (craft.File, io.ReadCloser, error)
	// Capabilities projects the deployment gate for view consumption
	// (CFT-S00-T005); the server keeps re-validating through Allows.
	Capabilities() service.CraftGateCapabilities
}

// NewCraftSessionHandler constructs the handler. svc may be nil: every
// endpoint then answers 503 without touching anything.
func NewCraftSessionHandler(svc CraftSessionAPI) *CraftSessionHandler {
	return &CraftSessionHandler{svc: svc}
}

// The route registrations are driven through package-level registrations: the
// container builds the handlers when the craft assembly is wired, and the
// router picks them up where they must live (the sessions group inherits the
// normal auth guards; the preview origin route must precede global auth).
var (
	registeredCraftSessionHandler CraftSessionAPI
	registeredCraftPreviewHandler *CraftPreviewHandler
)

// RegisterCraftSessionHandler installs the craft session handler for routing.
func RegisterCraftSessionHandler(h CraftSessionAPI) { registeredCraftSessionHandler = h }

// RegisteredCraftSessionHandler returns the registered handler (nil when the
// craft assembly is not wired — then no craft route is mounted at all).
func RegisteredCraftSessionHandler() CraftSessionAPI { return registeredCraftSessionHandler }

// RegisterCraftPreviewRouteHandler installs W02's preview handler for routing.
func RegisterCraftPreviewRouteHandler(h *CraftPreviewHandler) { registeredCraftPreviewHandler = h }

// RegisteredCraftPreviewRouteHandler returns the registered preview handler.
func RegisteredCraftPreviewRouteHandler() *CraftPreviewHandler { return registeredCraftPreviewHandler }

// CraftSnapshotAPI is the handler's view of the craft snapshot service
// (C05). The concrete *service.CraftSnapshotService satisfies it.
type CraftSnapshotAPI interface {
	RestoreIdempotent(context.Context, craft.Scope, service.CraftRestoreRequest) (service.CraftRestoreOutcome, error)
	ListSnapshots(context.Context, craft.Scope) ([]craft.StoredSnapshot, error)
}

var registeredCraftSnapshotHandler CraftSnapshotAPI

// RegisterCraftSnapshotHandler installs the C05 snapshot service for
// routing; the restore routes mount with the craft session table.
func RegisterCraftSnapshotHandler(h CraftSnapshotAPI) { registeredCraftSnapshotHandler = h }

// RegisteredCraftSnapshotHandler returns the registered snapshot API (nil
// when the C05 assembly is not wired — then no restore route mounts).
func RegisteredCraftSnapshotHandler() CraftSnapshotAPI { return registeredCraftSnapshotHandler }

// CraftSnapshotHandler serves C05's recovery snapshot HTTP surface. The
// routes inherit the enclosing sessions group's guards; the owner write
// ACL is enforced inside the service like every craft write entry.
type CraftSnapshotHandler struct {
	svc CraftSnapshotAPI
}

// NewCraftSnapshotHandler constructs the handler. svc may be nil: every
// endpoint then answers 503 without touching anything.
func NewCraftSnapshotHandler(svc CraftSnapshotAPI) *CraftSnapshotHandler {
	return &CraftSnapshotHandler{svc: svc}
}

// craftRouteGroup is the route-mounting subset satisfied by both a raw gin
// group and the router's API-key-policy wrapper, so the craft routes can be
// mounted through whichever wrapper declares their auth policy.
type craftRouteGroup interface {
	GET(string, ...gin.HandlerFunc) gin.IRoutes
	POST(string, ...gin.HandlerFunc) gin.IRoutes
}

// RegisterCraftSessionRoutes mounts W03's craft API table. craftSessions is
// the /craft/sessions group (create + list); sessions is the existing
// /sessions group whose guards the per-session craft routes inherit. Nil
// handlers leave their surface unmounted: fail-closed, no silent 404 shims.
func RegisterCraftSessionRoutes(craftSessions, sessions craftRouteGroup, craftHandler *CraftSessionHandler, previewHandler *CraftPreviewHandler) {
	if craftSessions != nil && craftHandler != nil {
		craftSessions.POST("", craftHandler.CreateCraftSession)
		craftSessions.GET("", craftHandler.ListCraftSessions)
	}
	if sessions == nil {
		return
	}
	if craftHandler != nil {
		// The GET tree binds :id (see RegisterSessionRoutes); handlers accept
		// both names. POST routes use :session_id like their siblings.
		sessions.GET("/:id/craft", craftHandler.GetCraftWorkspace)
		sessions.GET("/:id/craft/versions", craftHandler.ListCraftVersions)
		sessions.GET("/:id/craft/versions/:version_id", craftHandler.GetCraftVersion)
		sessions.GET("/:id/craft/versions/:version_id/files/*file_path", craftHandler.DownloadCraftVersionFile)
		sessions.POST("/:session_id/craft/inputs", craftHandler.PostCraftInput)
		sessions.POST("/:session_id/craft/runs", craftHandler.PostCraftRun)
	}
	if usageHandler := RegisteredCraftUsageHandler(); usageHandler != nil {
		// O04: the usage + execution-diagnostics read (aggregation, as_of,
		// main/child calls, sandbox residency, checks, failure reasons).
		usageHolder := NewCraftUsageHandler(usageHandler)
		sessions.GET("/:id/craft/usage", usageHolder.GetCraftUsage)
	}
	if snapshotHandler := RegisteredCraftSnapshotHandler(); snapshotHandler != nil {
		// C05: the recovery snapshot surface — the workbench's "continue
		// from this version" entrance and its idempotent restore.
		holder := NewCraftSnapshotHandler(snapshotHandler)
		sessions.GET("/:id/craft/snapshots", holder.ListCraftSnapshots)
		sessions.POST("/:session_id/craft/restore", holder.RestoreCraftSnapshot)
	}
	if previewHandler != nil {
		// W02's authenticated ticket issuance endpoint, mounted at its exact
		// path (same route as RegisterCraftPreviewIssueRoute, which stays
		// available for raw gin groups).
		sessions.POST("/:session_id/craft/versions/:version_id/preview",
			craftPreviewFailureMetrics(previewHandler.IssueCraftPreview))
	}
}

// craftPreviewFailureMetrics counts a failed craft preview issuance at the
// HTTP seam (O04's craft_preview_failures_total): any 4xx/5xx answer is one
// failure event. The preview handler itself stays untouched — the metric
// lives at the mounting seam this file owns.
func craftPreviewFailureMetrics(next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		next(c)
		if c.Writer.Status() >= http.StatusBadRequest {
			metrics.CraftPreviewFailure()
		}
	}
}

// craftScope derives the caller's craft scope from the authenticated context.
// Tenant and user never come from the request body.
func craftScope(c *gin.Context) (craft.Scope, bool) {
	tenantVal, ok := c.Get(types.TenantIDContextKey.String())
	if !ok {
		return craft.Scope{}, false
	}
	tenantID, ok := tenantVal.(uint64)
	if !ok || tenantID == 0 {
		return craft.Scope{}, false
	}
	ctx := c.Request.Context()
	userID := types.SessionOwnerIDFromContext(ctx)
	if userID == "" {
		userID, _ = types.UserIDFromContext(ctx)
	}
	if userID == "" {
		return craft.Scope{}, false
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(c.Param("id"))
	}
	return craft.Scope{TenantID: tenantID, UserID: userID, SessionID: sessionID}, true
}

// decodeCraftBody strictly decodes one craft request body: unknown fields —
// in particular injected authority fields like tenant_id, user_id, worker or
// sandbox identity — are rejected, and so is trailing data. Bodies over the
// craft body limit answer 413.
func decodeCraftBody(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxCraftRequestBodyBytes)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if stderrors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"error":   gin.H{"code": "payload_too_large", "message": "craft request body exceeds 1MiB"},
			})
			return false
		}
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		c.Error(apperrors.NewBadRequestError("craft request body rejected: " + err.Error()))
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		c.Error(apperrors.NewBadRequestError("craft request body rejected: trailing data"))
		return false
	}
	return true
}

// craftHTTPError maps the craft sentinel vocabulary onto the plan's generic
// error codes: 400 input, 403 no permission, 404 invisible, 409 conflict,
// 503 dependency not ready.
func craftHTTPError(c *gin.Context, err error) {
	switch {
	case stderrors.Is(err, craft.ErrInvalidInput):
		c.Error(apperrors.NewBadRequestError(err.Error()))
	case stderrors.Is(err, craft.ErrForbidden):
		c.Error(apperrors.NewForbiddenError(err.Error()))
	case stderrors.Is(err, craft.ErrNotFound):
		c.Error(apperrors.NewNotFoundError(err.Error()))
	case stderrors.Is(err, craft.ErrConflict):
		c.Error(apperrors.NewConflictError(err.Error()))
	case stderrors.Is(err, craft.ErrBusy):
		var conflict *service.ActiveRunConflict
		detail := gin.H{"code": "run_active", "message": err.Error()}
		if stderrors.As(err, &conflict) {
			detail["run_id"] = conflict.RunID
		}
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": detail})
	case stderrors.Is(err, craft.ErrUnsupported):
		c.Error(apperrors.NewServiceUnavailableError(err.Error()))
	default:
		c.Error(apperrors.NewInternalServerError(err.Error()))
	}
}

// craftUnauthorized answers 401 for requests without an authenticated scope.
func craftUnauthorized(c *gin.Context) {
	c.Error(apperrors.NewUnauthorizedError("tenant and user are required"))
}

// -----------------------------------------------------------------------------
// DTOs (snake_case only; every identity field is server-derived)
// -----------------------------------------------------------------------------

type createCraftSessionRequest struct {
	RequestID string `json:"request_id"`
	Title     string `json:"title"`
	Kind      string `json:"kind"`
}

type craftInputRequest struct {
	ResourceRef    string `json:"resource_ref"`
	ExpectedSHA256 string `json:"expected_sha256"`
}

type craftRunRequestDTO struct {
	RequestID      string   `json:"request_id"`
	Prompt         string   `json:"prompt"`
	InputRefs      []string `json:"input_refs"`
	KnowledgeScope string   `json:"knowledge_scope"`
	BaseVersionID  string   `json:"base_version_id"`
}

func craftWorkspaceDTO(ws craft.Workspace) gin.H {
	return gin.H{
		"id": ws.ID, "session_id": ws.SessionID, "user_id": ws.UserID,
		"sandbox_id": ws.SandboxID, "generation": ws.Generation,
		"runtime_digest": ws.RuntimeDigest, "revision": ws.Revision,
	}
}

func craftFileDTO(f craft.File) gin.H {
	return gin.H{"path": f.Path, "ref": f.Ref, "sha256": f.SHA256, "mime": f.MIME, "bytes": f.Bytes}
}

func craftCheckDTO(check craft.Check) gin.H {
	return gin.H{"name": check.Name, "status": check.Status, "detail": check.Detail}
}

func craftVersionDTO(v craft.Version) gin.H {
	files := make([]gin.H, 0, len(v.Files))
	for _, f := range v.Files {
		files = append(files, craftFileDTO(f))
	}
	checks := make([]gin.H, 0, len(v.Checks))
	for _, check := range v.Checks {
		checks = append(checks, craftCheckDTO(check))
	}
	return gin.H{
		"id": v.ID, "workspace_id": v.WorkspaceID, "run_id": v.RunID,
		"kind": v.Kind, "files": files, "checks": checks,
	}
}

func craftInputDTO(in craft.Input) gin.H {
	return gin.H{
		"ref": in.Ref, "name": in.Name, "sha256": in.SHA256,
		"bytes": in.Bytes, "citation_id": in.CitationID,
	}
}

// -----------------------------------------------------------------------------
// Endpoints
// -----------------------------------------------------------------------------

// CreateCraftSession serves POST /api/v1/craft/sessions.
func (h *CraftSessionHandler) CreateCraftSession(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft sessions are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID != "" {
		craftUnauthorized(c)
		return
	}
	var body createCraftSessionRequest
	if !decodeCraftBody(c, &body) {
		return
	}
	workspace, err := h.svc.Create(c.Request.Context(), scope, craft.CreateRequest{
		RequestID: body.RequestID, Title: body.Title, Kind: body.Kind,
	})
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	caps := h.svc.Capabilities()
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"session_id": workspace.SessionID, "workspace_id": workspace.ID,
			"engine_type": "trpc",
			// CFT-S00-T005: the gate snapshot rides along so the UI can offer
			// exactly the open kinds (snake_case wire, struct fields stay Go).
			"capabilities": gin.H{"enabled": caps.Enabled, "allowed_kinds": caps.Kinds},
		},
	})
}

// ListCraftSessions serves GET /api/v1/craft/sessions?cursor=...&limit=...
func (h *CraftSessionHandler) ListCraftSessions(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft sessions are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	limit := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			c.Error(apperrors.NewBadRequestError("limit must be between 1 and 100"))
			return
		}
		limit = parsed
	}
	sessions, next, err := h.svc.List(c.Request.Context(), scope, c.Query("cursor"), limit)
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	data := make([]gin.H, 0, len(sessions))
	for _, s := range sessions {
		data = append(data, gin.H{
			"session_id": s.SessionID, "workspace_id": s.WorkspaceID,
			"kind": s.Kind, "title": s.Title, "engine_type": s.EngineType,
			"updated_at": s.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data, "next_cursor": next})
}

// GetCraftWorkspace serves GET /api/v1/sessions/:session_id/craft.
func (h *CraftSessionHandler) GetCraftWorkspace(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft sessions are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	view, err := h.svc.View(c.Request.Context(), scope)
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	data := gin.H{
		"session_id": view.SessionID, "workspace_id": view.WorkspaceID,
		"kind": view.Kind, "title": view.Title, "engine_type": view.EngineType,
		"workspace":     craftWorkspaceDTO(view.Workspace),
		"active_run_id": view.ActiveRunID, "pending_id": view.PendingID,
		"last_seq": view.LastSeq,
		// CFT-S00-T005: the gate snapshot rides along; reading history never
		// depends on it, submits do.
		"capabilities": func() gin.H {
			caps := h.svc.Capabilities()
			return gin.H{"enabled": caps.Enabled, "allowed_kinds": caps.Kinds}
		}(),
	}
	if view.ActiveRun != nil {
		data["active_run"] = runView(*view.ActiveRun)
	}
	if view.CurrentVersion != nil {
		data["current_version"] = craftVersionDTO(*view.CurrentVersion)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// PostCraftInput serves POST /api/v1/sessions/:session_id/craft/inputs.
func (h *CraftSessionHandler) PostCraftInput(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft sessions are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	var body craftInputRequest
	if !decodeCraftBody(c, &body) {
		return
	}
	input, err := h.svc.AssociateInput(c.Request.Context(), scope, body.ResourceRef, body.ExpectedSHA256)
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": craftInputDTO(input)})
}

// PostCraftRun serves POST /api/v1/sessions/:session_id/craft/runs.
func (h *CraftSessionHandler) PostCraftRun(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft sessions are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	var body craftRunRequestDTO
	if !decodeCraftBody(c, &body) {
		return
	}
	run, err := h.svc.StartRun(c.Request.Context(), scope, service.CraftRunRequest{
		RequestID: body.RequestID, Prompt: body.Prompt, InputRefs: body.InputRefs,
		KnowledgeScope: body.KnowledgeScope, BaseVersionID: body.BaseVersionID,
	})
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": runView(run)})
}

// ListCraftVersions serves GET /api/v1/sessions/:session_id/craft/versions.
func (h *CraftSessionHandler) ListCraftVersions(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft sessions are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	versions, err := h.svc.ListVersions(c.Request.Context(), scope)
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	data := make([]gin.H, 0, len(versions))
	for _, v := range versions {
		data = append(data, craftVersionDTO(v))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data, "next_cursor": nil})
}

// GetCraftVersion serves GET /api/v1/sessions/:session_id/craft/versions/:version_id.
func (h *CraftSessionHandler) GetCraftVersion(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft sessions are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	version, err := h.svc.GetVersion(c.Request.Context(), scope, c.Param("version_id"))
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": craftVersionDTO(version)})
}

// DownloadCraftVersionFile serves
// GET /api/v1/sessions/:session_id/craft/versions/:version_id/files/:file_path.
// The path is the version-relative manifest member; the whole
// session→workspace→version→file chain is re-verified before any byte flows.
func (h *CraftSessionHandler) DownloadCraftVersionFile(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft sessions are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	filePath := strings.TrimPrefix(c.Param("file_path"), "/")
	file, reader, err := h.svc.OpenVersionFile(c.Request.Context(), scope, c.Param("version_id"), filePath)
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	defer reader.Close()
	if file.MIME != "" {
		c.Header("Content-Type", file.MIME)
	}
	c.Header("Content-Length", strconv.FormatInt(file.Bytes, 10))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-store")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader)
}

// -----------------------------------------------------------------------------
// C05: recovery snapshot endpoints
// -----------------------------------------------------------------------------

type craftRestoreRequestDTO struct {
	RequestID  string `json:"request_id"`
	SnapshotID string `json:"snapshot_id"`
	Revision   int64  `json:"revision"`
}

// craftSnapshotDTO projects one stored snapshot: the identity the workbench
// keys its "continue from this version" affordance on, plus the facts that
// decide whether the affordance may open (quiescent, runtime digest).
func craftSnapshotDTO(s craft.StoredSnapshot) gin.H {
	return gin.H{
		"snapshot_id": s.ID, "version_id": s.VersionID,
		"workspace_id": s.WorkspaceID,
		"files_digest": s.FilesDigest, "session_digest": s.SessionDigest,
		"runtime_digest": s.RuntimeDigest, "quiescent": s.Quiescent,
		"records": s.Manifest.Records, "created_at": s.CreatedAt,
	}
}

// ListCraftSnapshots serves GET /api/v1/sessions/:session_id/craft/snapshots:
// the workspace's stored recovery snapshots, newest first.
func (h *CraftSnapshotHandler) ListCraftSnapshots(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft snapshots are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	snapshots, err := h.svc.ListSnapshots(c.Request.Context(), scope)
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	data := make([]gin.H, 0, len(snapshots))
	for _, s := range snapshots {
		data = append(data, craftSnapshotDTO(s))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data, "next_cursor": nil})
}

// RestoreCraftSnapshot serves POST /api/v1/sessions/:session_id/craft/restore.
// request_id is the idempotency key, revision the workspace revision the
// client prepared the restore at; the service refuses active runs, revision
// races, untrusted runtimes, corrupt objects and non-owner scopes.
func (h *CraftSnapshotHandler) RestoreCraftSnapshot(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft snapshots are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	var body craftRestoreRequestDTO
	if !decodeCraftBody(c, &body) {
		return
	}
	outcome, err := h.svc.RestoreIdempotent(c.Request.Context(), scope, service.CraftRestoreRequest{
		RequestID: body.RequestID, SnapshotID: body.SnapshotID, Revision: body.Revision,
	})
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	data := gin.H{
		"workspace":   craftWorkspaceDTO(outcome.Workspace),
		"snapshot_id": outcome.Snapshot.ID,
		"version_id":  outcome.Snapshot.VersionID,
		"replayed":    outcome.Replayed,
		"generation":  outcome.Workspace.Generation,
		"revision":    outcome.Workspace.Revision,
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
