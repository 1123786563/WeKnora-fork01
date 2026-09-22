package session

// O04: the usage + execution-diagnostics HTTP surface.
//
// GET /api/v1/sessions/:session_id/craft/usage answers what a space
// administrator or an entitled member needs to reconcile one craft session:
// the UsageView aggregation (known/unknown calls, tokens, funding mix) with
// an explicit as_of stamp, plus the execution detail — main/child physical
// calls, sandbox residency, the current version's checks and the runs'
// recorded failure reasons.
//
// The response NEVER carries money: amounts come only from the commercial
// view, and this endpoint does not fabricate pricing. BYOK involvement is
// stated as a boolean so the UI can say plainly that those model calls are
// borne by the space's own credentials. Credentials and knowledge material
// contents are structurally absent — no field of this view can carry them.

import (
	"context"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/gin-gonic/gin"
)

// CraftUsageViewAPI is the handler's view of the usage view service. The
// concrete *service.CraftUsageViewService satisfies it.
type CraftUsageViewAPI interface {
	SessionUsage(ctx context.Context, scope craft.Scope) (service.CraftSessionUsageView, error)
}

var registeredCraftUsageHandler CraftUsageViewAPI

// RegisterCraftUsageHandler installs the O04 usage view service for routing.
func RegisterCraftUsageHandler(h CraftUsageViewAPI) { registeredCraftUsageHandler = h }

// RegisteredCraftUsageHandler returns the registered usage view API (nil
// when the O04 assembly is not wired — then the usage route never mounts).
func RegisteredCraftUsageHandler() CraftUsageViewAPI { return registeredCraftUsageHandler }

// CraftUsageHandler serves O04's usage HTTP surface. The routes inherit the
// enclosing sessions group's guards; the session read ACL is enforced inside
// the service, exactly like the W03 workspace GET.
type CraftUsageHandler struct {
	svc CraftUsageViewAPI
}

// NewCraftUsageHandler constructs the handler. svc may be nil: the endpoint
// then answers 503 without touching anything.
func NewCraftUsageHandler(svc CraftUsageViewAPI) *CraftUsageHandler {
	return &CraftUsageHandler{svc: svc}
}

// GetCraftUsage serves GET /api/v1/sessions/:session_id/craft/usage.
func (h *CraftUsageHandler) GetCraftUsage(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft usage view is unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok {
		craftUnauthorized(c)
		return
	}
	view, err := h.svc.SessionUsage(c.Request.Context(), scope)
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	calls := make([]gin.H, 0, len(view.Calls))
	for _, call := range view.Calls {
		calls = append(calls, gin.H{
			"call_id": call.CallID, "attempt_id": call.AttemptID, "run_id": call.RunID,
			"runtime": call.Runtime, "delegation_id": call.DelegationID,
			"model_id": call.ModelID, "funding": call.Funding, "status": call.Status,
			"input_tokens": call.Input, "output_tokens": call.Output, "cached_tokens": call.Cached,
		})
	}
	runs := make([]gin.H, 0, len(view.Runs))
	for _, run := range view.Runs {
		runs = append(runs, gin.H{
			"run_id": run.RunID, "status": run.Status, "failure_reason": run.FailureReason,
			"created_at": run.CreatedAt.Format(time.RFC3339), "updated_at": run.UpdatedAt.Format(time.RFC3339),
		})
	}
	data := gin.H{
		"as_of": view.AsOf.Format(time.RFC3339),
		"usage": gin.H{
			"known_calls":   view.KnownCalls,
			"unknown_calls": view.UnknownCalls,
			"input_tokens":  view.InputTokens,
			"output_tokens": view.OutputTokens,
			"cached_tokens": view.CachedTokens,
			"funding":       view.Funding,
		},
		"byok_model_borne_by_space": view.ByokModelBorneBySpace,
		"runs":                      runs,
		"calls":                     calls,
	}
	if view.Residency != nil {
		data["sandbox_residency"] = gin.H{
			"starts": view.Residency.Starts, "stops": view.Residency.Stops,
			"open_starts": view.Residency.OpenStarts, "dwell_seconds": view.Residency.DwellSeconds,
			"storage_bytes_day": view.Residency.StorageBytesDay,
		}
	}
	if len(view.Checks) > 0 {
		checks := make([]gin.H, 0, len(view.Checks))
		for _, check := range view.Checks {
			checks = append(checks, gin.H{
				"version_id": check.VersionID, "name": check.Name,
				"status": check.Status, "detail": check.Detail,
			})
		}
		data["checks"] = checks
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
