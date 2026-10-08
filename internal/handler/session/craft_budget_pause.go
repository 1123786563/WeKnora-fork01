package session

// T20 (#139): the budget-pause HTTP surface. A budget-exhausted Run parks
// durably (waiting_user / budget_exhausted) with the member-visible reason
// and the allowed action; this file exposes exactly the two decisions the
// Spec assigns to members: READING the pause (Task readers — the owner sees
// the extension affordance, everyone else sees the contact-owner copy) and
// REQUESTING an extension (the service re-runs the Task-owner/tenant
// billing-admin check itself, so the entrance is not the authority).

import (
	"context"
	stderrors "errors"
	"net/http"

	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/gin-gonic/gin"
)

// CraftBudgetPauseAPI is the service seam the central assembly provides.
// A nil registration leaves the routes unmounted (fail-closed, no stub).
type CraftBudgetPauseAPI interface {
	// BudgetPause returns the durable pause view of one Run, or
	// craft.ErrNotFound when the Run is not currently budget-paused.
	BudgetPause(ctx context.Context, scope craft.Scope, runID string) (craft.BudgetPause, error)
	// ExtendAndResume extends a paused Run's grant after reconciliation and
	// durably resumes it (owner/tenant billing-admin only).
	ExtendAndResume(ctx context.Context, scope craft.Scope, runID, key string, extraCalls int, extraCredits commercial.Credits) error
	// MayExtendBudget projects the caller's CURRENT extension authority
	// without mutating anything.
	MayExtendBudget(ctx context.Context, scope craft.Scope) error
}

var registeredCraftBudgetPauseHandler CraftBudgetPauseAPI
var registeredCraftBudgetPauseAccess craft.TaskAccessChecker

// RegisterCraftBudgetPauseHandler installs the budget-pause service (and the
// Task read gate) for route mounting. Either nil fails closed: the routes
// stay unmounted instead of serving an unauthenticated stub.
func RegisterCraftBudgetPauseHandler(h CraftBudgetPauseAPI, access craft.TaskAccessChecker) {
	registeredCraftBudgetPauseHandler = h
	registeredCraftBudgetPauseAccess = access
}

// RegisteredCraftBudgetPauseHandler returns the registered pause API.
func RegisteredCraftBudgetPauseHandler() CraftBudgetPauseAPI {
	return registeredCraftBudgetPauseHandler
}

// RegisteredCraftBudgetPauseAccess returns the registered Task gate.
func RegisteredCraftBudgetPauseAccess() craft.TaskAccessChecker {
	return registeredCraftBudgetPauseAccess
}

// CraftBudgetPauseHandler serves the budget-pause surface. The routes
// inherit the enclosing sessions group's guards; Task membership is
// enforced here through the T00 frozen checker (nil → 403 fail-closed).
type CraftBudgetPauseHandler struct {
	svc    CraftBudgetPauseAPI
	access craft.TaskAccessChecker
}

// NewCraftBudgetPauseHandler constructs the handler. svc/access may be nil:
// every endpoint then answers fail-closed without touching anything.
func NewCraftBudgetPauseHandler(svc CraftBudgetPauseAPI, access craft.TaskAccessChecker) *CraftBudgetPauseHandler {
	return &CraftBudgetPauseHandler{svc: svc, access: access}
}

// craftBudgetPauseBody is the budget pause wire. The extension action is
// projected only when the current caller may extend; it contains the exact
// server-owned idempotency key and quantum required by the POST endpoint.
type craftBudgetPauseBody struct {
	RunID           string                       `json:"run_id"`
	Reason          string                       `json:"reason"`
	Limit           int64                        `json:"limit"`
	Used            int64                        `json:"used"`
	ExtensionAction *craft.BudgetExtensionAction `json:"extension_action"`
}

// GetCraftBudgetPause serves GET /api/v1/sessions/:id/craft/runs/:run_id/budget/pause.
func (h *CraftBudgetPauseHandler) GetCraftBudgetPause(c *gin.Context) {
	if h == nil || h.svc == nil {
		craftHTTPError(c, craft.ErrNotFound)
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	if err := craft.RequireTaskAccess(c.Request.Context(), h.access, scope, craft.TaskRead); err != nil {
		craftHTTPError(c, err)
		return
	}
	pause, err := h.svc.BudgetPause(c.Request.Context(), scope, c.Param("run_id"))
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	// can_extend is the SERVER's current projection of the Task
	// owner/billing-admin authority — a projection failure degrades to
	// false (the contact-owner copy), never to a client-side guess.
	canExtend := h.svc.MayExtendBudget(c.Request.Context(), scope) == nil
	var extensionAction *craft.BudgetExtensionAction
	if canExtend {
		extensionAction = pause.ExtensionAction
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"data":       craftBudgetPauseBody{RunID: pause.RunID, Reason: pause.Reason, Limit: pause.Limit, Used: pause.Used, ExtensionAction: extensionAction},
		"can_extend": canExtend,
	})
}

// craftBudgetExtendRequest echoes one pending server-owned pause action. A
// retry must send the same key and exact call/credit quantum returned by GET.
type craftBudgetExtendRequest struct {
	Key          string `json:"key"`
	ExtraCalls   int    `json:"extra_calls"`
	ExtraCredits int64  `json:"extra_credits"`
}

// PostCraftBudgetExtend serves POST /api/v1/sessions/:session_id/craft/runs/:run_id/budget/extend.
// The extension authority is the service's own current-row check — Task
// membership only gates who may ATTEMPT the request.
func (h *CraftBudgetPauseHandler) PostCraftBudgetExtend(c *gin.Context) {
	if h == nil || h.svc == nil {
		craftHTTPError(c, craft.ErrNotFound)
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	if err := craft.RequireTaskAccess(c.Request.Context(), h.access, scope, craft.TaskRead); err != nil {
		craftHTTPError(c, err)
		return
	}
	var body craftBudgetExtendRequest
	if !decodeCraftBody(c, &body) {
		return
	}
	if body.Key == "" || body.ExtraCalls <= 0 || body.ExtraCredits <= 0 {
		craftHTTPError(c, craft.ErrInvalidInput)
		return
	}
	err := h.svc.ExtendAndResume(c.Request.Context(), scope, c.Param("run_id"),
		body.Key, body.ExtraCalls, commercial.Credits(body.ExtraCredits))
	if err != nil {
		// A pending reconciliation is client-actionable: settle the unknown
		// dispatched effect before retrying — 409, not an opaque 500.
		if stderrors.Is(err, craft.ErrReconcilePending) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": gin.H{
				"code": "RECONCILE_PENDING", "message": "dispatched budget effects are unconfirmed; reconcile before extending"}})
			return
		}
		craftHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
