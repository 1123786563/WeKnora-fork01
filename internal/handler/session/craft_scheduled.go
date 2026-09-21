package session

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// SP3 (C-23): the scheduled-task HTTP surface (spec §3) — seven endpoints on
// /craft/scheduled-tasks. Every route inherits the enclosing group's
// Viewer+/API-key guards; the per-task permission chain (owner write AND
// owner read — unlike craft sessions there is no admin read fallback) is
// enforced inside the service/store, so a wrong-owner, deleted and missing
// task all answer one indistinguishable 404. Tenant and owner are ALWAYS
// derived from the authenticated context, never from the body.

// CraftScheduledAPI is the handler's view of the scheduled-task service. The
// concrete *service.CraftScheduledService satisfies it.
type CraftScheduledAPI interface {
	CreateScheduledTask(context.Context, uint64, string, service.CraftScheduledTaskCreate) (*types.CraftScheduledTask, string, error)
	ListScheduledTasks(context.Context, uint64, string) ([]types.CraftScheduledTask, error)
	GetScheduledTask(context.Context, uint64, string, string) (*service.CraftScheduledTaskDetail, error)
	UpdateScheduledTask(context.Context, uint64, string, string, service.CraftScheduledTaskUpdate) (*types.CraftScheduledTask, error)
	DeleteScheduledTask(context.Context, uint64, string, string) error
	RunScheduledTaskNow(context.Context, uint64, string, string) (string, error)
	ListScheduledTaskRuns(context.Context, uint64, string, string, *time.Time, int) ([]types.CraftScheduledTaskRun, error)
}

var _ CraftScheduledAPI = (*service.CraftScheduledService)(nil)

// CraftScheduledHandler serves the scheduled-task table. A nil svc answers
// 503 on every endpoint without touching anything.
type CraftScheduledHandler struct {
	svc CraftScheduledAPI
}

// NewCraftScheduledHandler constructs the handler.
func NewCraftScheduledHandler(svc CraftScheduledAPI) *CraftScheduledHandler {
	return &CraftScheduledHandler{svc: svc}
}

// The route mounting is driven through the package-level registration like
// the craft session surface: the container builds the handler when the
// scheduled-task assembly is wired, routes_chat.go mounts it where it must
// live; without the registration no route exists (fail-closed, no 503 shims).
var registeredCraftScheduledHandler CraftScheduledAPI

// RegisterCraftScheduledHandler installs the scheduled-task handler for
// routing.
func RegisterCraftScheduledHandler(h CraftScheduledAPI) { registeredCraftScheduledHandler = h }

// RegisteredCraftScheduledHandler returns the registered handler (nil when
// the assembly is not wired — then the surface stays unmounted).
func RegisteredCraftScheduledHandler() CraftScheduledAPI { return registeredCraftScheduledHandler }

// craftScheduledRouteGroup is the route-mounting subset satisfied by both a
// raw gin group and the router's API-key policy wrapper, covering the four
// verbs the task table uses.
type craftScheduledRouteGroup interface {
	GET(string, ...gin.HandlerFunc) gin.IRoutes
	POST(string, ...gin.HandlerFunc) gin.IRoutes
	PATCH(string, ...gin.HandlerFunc) gin.IRoutes
	DELETE(string, ...gin.HandlerFunc) gin.IRoutes
}

// RegisterCraftScheduledTaskRoutes mounts the SP3 task table (spec §3). A
// nil group or handler leaves the surface unmounted — fail-closed, no
// silent 404 shims.
func RegisterCraftScheduledTaskRoutes(group craftScheduledRouteGroup, h *CraftScheduledHandler) {
	if group == nil || h == nil {
		return
	}
	group.GET("", h.ListScheduledTasks)
	group.POST("", h.CreateScheduledTask)
	group.GET("/:id", h.GetScheduledTask)
	group.PATCH("/:id", h.UpdateScheduledTask)
	group.DELETE("/:id", h.DeleteScheduledTask)
	group.POST("/:id/run-now", h.RunScheduledTaskNow)
	group.GET("/:id/runs", h.ListScheduledTaskRuns)
}

// craftScheduledScope derives the caller's tenant and owner identity from the
// authenticated context — the craftScope shape minus the session param: a
// scheduled task is user-owned, not session-scoped, and tenant/user never
// come from the request body.
func craftScheduledScope(c *gin.Context) (uint64, string, bool) {
	tenantVal, ok := c.Get(types.TenantIDContextKey.String())
	if !ok {
		return 0, "", false
	}
	tenantID, ok := tenantVal.(uint64)
	if !ok || tenantID == 0 {
		return 0, "", false
	}
	ctx := c.Request.Context()
	userID := types.SessionOwnerIDFromContext(ctx)
	if userID == "" {
		userID, _ = types.UserIDFromContext(ctx)
	}
	if userID == "" {
		return 0, "", false
	}
	return tenantID, userID, true
}

// craftScheduledPayload normalizes an editor payload's JSON null literal to
// absence (Task 2 review Minor-3): the service's pair rule reads presence as
// len(payload) > 0, and encoding/json hands a literal null through as the
// four bytes "null" — a client's explicit null must mean "field not
// supplied", never a malformed payload.
func craftScheduledPayload(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(string(raw)), "null") {
		return nil
	}
	return raw
}

// craftScheduledTaskDTO projects one recipe. Identity fields (tenant, owner)
// are server-derived and stay off the wire.
func craftScheduledTaskDTO(task types.CraftScheduledTask) gin.H {
	return gin.H{
		"id": task.ID, "name": task.Name, "prompt": task.Prompt,
		"cron_expression": task.CronExpression, "editor_mode": task.EditorMode,
		"status": task.Status, "next_run_at": task.NextRunAt, "last_run_at": task.LastRunAt,
		"created_at": task.CreatedAt, "updated_at": task.UpdatedAt,
	}
}

// craftScheduledRunDTO projects one fire's ledger row.
func craftScheduledRunDTO(run types.CraftScheduledTaskRun) gin.H {
	return gin.H{
		"id": run.ID, "task_id": run.TaskID, "session_id": run.SessionID,
		"status": run.Status, "trigger_source": run.TriggerSource,
		"skip_reason": run.SkipReason, "error_class": run.ErrorClass,
		"error_detail": run.ErrorDetail, "summary": run.Summary,
		"started_at": run.StartedAt, "finished_at": run.FinishedAt,
		"created_at": run.CreatedAt, "updated_at": run.UpdatedAt,
	}
}

// craftScheduledUnavailable answers 503 for an unwired assembly.
func craftScheduledUnavailable(c *gin.Context) {
	c.Error(apperrors.NewServiceUnavailableError("craft scheduled tasks are unavailable"))
}

// -----------------------------------------------------------------------------
// DTOs (snake_case only; every identity field is server-derived)
// -----------------------------------------------------------------------------

type craftScheduledCreateRequest struct {
	Name           string          `json:"name"`
	Prompt         string          `json:"prompt"`
	EditorMode     string          `json:"editor_mode"`
	Payload        json.RawMessage `json:"payload"`
	RunImmediately bool            `json:"run_immediately"`
}

type craftScheduledUpdateRequest struct {
	Name       *string         `json:"name"`
	Prompt     *string         `json:"prompt"`
	EditorMode *string         `json:"editor_mode"`
	Payload    json.RawMessage `json:"payload"`
	Status     *string         `json:"status"`
}

// -----------------------------------------------------------------------------
// Endpoints
// -----------------------------------------------------------------------------

// ListScheduledTasks serves GET /api/v1/craft/scheduled-tasks: the caller's
// live recipes newest-first (V1 carries no pagination — the Onyx shape).
func (h *CraftScheduledHandler) ListScheduledTasks(c *gin.Context) {
	if h == nil || h.svc == nil {
		craftScheduledUnavailable(c)
		return
	}
	tenantID, ownerID, ok := craftScheduledScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	tasks, err := h.svc.ListScheduledTasks(c.Request.Context(), tenantID, ownerID)
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	data := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		data = append(data, craftScheduledTaskDTO(task))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// CreateScheduledTask serves POST /api/v1/craft/scheduled-tasks: the editor
// mode+payload pair compiles server-side into the stored cron; an optional
// run_immediately additionally appends one manual queued run (its id rides
// the response) without touching the schedule's own ticket.
func (h *CraftScheduledHandler) CreateScheduledTask(c *gin.Context) {
	if h == nil || h.svc == nil {
		craftScheduledUnavailable(c)
		return
	}
	tenantID, ownerID, ok := craftScheduledScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	var body craftScheduledCreateRequest
	if !decodeCraftBody(c, &body) {
		return
	}
	task, runID, err := h.svc.CreateScheduledTask(c.Request.Context(), tenantID, ownerID,
		service.CraftScheduledTaskCreate{
			Name: body.Name, Prompt: body.Prompt, EditorMode: body.EditorMode,
			Payload: craftScheduledPayload(body.Payload), RunImmediately: body.RunImmediately,
		})
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	data := craftScheduledTaskDTO(*task)
	if runID != "" {
		data["run_id"] = runID
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": data})
}

// GetScheduledTask serves GET /api/v1/craft/scheduled-tasks/:id: the recipe
// plus its next three fires while active (a paused recipe previews nothing —
// nothing is scheduled until resumed).
func (h *CraftScheduledHandler) GetScheduledTask(c *gin.Context) {
	if h == nil || h.svc == nil {
		craftScheduledUnavailable(c)
		return
	}
	tenantID, ownerID, ok := craftScheduledScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	detail, err := h.svc.GetScheduledTask(c.Request.Context(), tenantID, ownerID, c.Param("id"))
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	data := craftScheduledTaskDTO(detail.Task)
	fires := make([]time.Time, 0, len(detail.NextFires))
	fires = append(fires, detail.NextFires...)
	data["next_fires"] = fires
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// UpdateScheduledTask serves PATCH /api/v1/craft/scheduled-tasks/:id: a
// partial update — editor_mode and payload MUST arrive as a pair (together
// they recompile the cron); an explicit payload:null is normalized to absence
// before the service sees it (Minor-3); status moves between active and
// paused with the ticket recomputed.
func (h *CraftScheduledHandler) UpdateScheduledTask(c *gin.Context) {
	if h == nil || h.svc == nil {
		craftScheduledUnavailable(c)
		return
	}
	tenantID, ownerID, ok := craftScheduledScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	var body craftScheduledUpdateRequest
	if !decodeCraftBody(c, &body) {
		return
	}
	task, err := h.svc.UpdateScheduledTask(c.Request.Context(), tenantID, ownerID, c.Param("id"),
		service.CraftScheduledTaskUpdate{
			Name: body.Name, Prompt: body.Prompt, EditorMode: body.EditorMode,
			Payload: craftScheduledPayload(body.Payload), Status: body.Status,
		})
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": craftScheduledTaskDTO(*task)})
}

// DeleteScheduledTask serves DELETE /api/v1/craft/scheduled-tasks/:id: the
// soft tombstone, idempotent — an already deleted, missing or foreign recipe
// answers the same 204 because the desired end state already holds.
func (h *CraftScheduledHandler) DeleteScheduledTask(c *gin.Context) {
	if h == nil || h.svc == nil {
		craftScheduledUnavailable(c)
		return
	}
	tenantID, ownerID, ok := craftScheduledScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	if err := h.svc.DeleteScheduledTask(c.Request.Context(), tenantID, ownerID, c.Param("id")); err != nil {
		craftHTTPError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RunScheduledTaskNow serves POST /api/v1/craft/scheduled-tasks/:id/run-now:
// one queued manual ledger row (202 with its id) — a paused recipe may be
// fired manually and the schedule's ticket never moves.
func (h *CraftScheduledHandler) RunScheduledTaskNow(c *gin.Context) {
	if h == nil || h.svc == nil {
		craftScheduledUnavailable(c)
		return
	}
	tenantID, ownerID, ok := craftScheduledScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	runID, err := h.svc.RunScheduledTaskNow(c.Request.Context(), tenantID, ownerID, c.Param("id"))
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": gin.H{"run_id": runID}})
}

// The run-history page (spec §3): keyset on started_at (the before cursor is
// exclusive), limit clamped 1..100 with 50 the default.
const (
	craftScheduledRunsDefaultLimit = 50
	craftScheduledRunsMaxLimit     = 100
)

// ListScheduledTaskRuns serves GET /api/v1/craft/scheduled-tasks/:id/runs:
// the task's fire ledger newest-first, paged by the before/limit keyset. The
// next_cursor is the last row's started_at (RFC3339 UTC) — pass it back as
// before; an empty cursor means the history is exhausted.
func (h *CraftScheduledHandler) ListScheduledTaskRuns(c *gin.Context) {
	if h == nil || h.svc == nil {
		craftScheduledUnavailable(c)
		return
	}
	tenantID, ownerID, ok := craftScheduledScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	limit := craftScheduledRunsDefaultLimit
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > craftScheduledRunsMaxLimit {
			c.Error(apperrors.NewBadRequestError(
				"limit must be between 1 and " + strconv.Itoa(craftScheduledRunsMaxLimit)))
			return
		}
		limit = parsed
	}
	var before *time.Time
	if raw := strings.TrimSpace(c.Query("before")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.Error(apperrors.NewBadRequestError("before must be an RFC3339 timestamp"))
			return
		}
		parsed = parsed.UTC()
		before = &parsed
	}
	runs, err := h.svc.ListScheduledTaskRuns(c.Request.Context(), tenantID, ownerID,
		c.Param("id"), before, limit)
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	data := make([]gin.H, 0, len(runs))
	for _, run := range runs {
		data = append(data, craftScheduledRunDTO(run))
	}
	next := ""
	if len(runs) == limit && runs[len(runs)-1].StartedAt != nil {
		next = runs[len(runs)-1].StartedAt.UTC().Format(time.RFC3339)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data, "next_cursor": next})
}
