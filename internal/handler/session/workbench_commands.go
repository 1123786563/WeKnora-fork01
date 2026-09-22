package session

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/approval"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// WorkbenchCommandHandler exposes typed execution interactions. It always
// copies identity from the authenticated context before invoking the service.
type WorkbenchCommandHandler struct {
	interactions *workbenchservice.Service
}

func NewWorkbenchCommandHandler(interactions *workbenchservice.Service) *WorkbenchCommandHandler {
	return &WorkbenchCommandHandler{interactions: interactions}
}

func commandContext(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	if tenant, ok := c.Get(types.TenantIDContextKey.String()); ok {
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
	}
	if principal, ok := c.Get(types.PrincipalContextKey.String()); ok {
		if p, valid := principal.(types.Principal); valid && p.Valid() {
			ctx = types.WithPrincipal(ctx, p)
		}
	}
	if actor, ok := c.Get(types.UserIDContextKey.String()); ok {
		ctx = context.WithValue(ctx, types.UserIDContextKey, actor)
	}
	return ctx
}

func (h *WorkbenchCommandHandler) ListInteractions(c *gin.Context) {
	if h == nil || h.interactions == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	items, err := h.interactions.List(commandContext(c), c.Param("run_id"))
	if err != nil {
		writeWorkbenchCommandError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *WorkbenchCommandHandler) DecideInteraction(c *gin.Context) {
	if h == nil || h.interactions == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	var input workbench.InteractionDecision
	if err := c.ShouldBindJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}
	// Wire takes the interaction id from the URL param; bind it before the
	// shared admission rule so client-side and server-side validation agree.
	input.ID = c.Param("id")
	if err := input.Validate(); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	decision, err := h.interactions.Decide(commandContext(c), c.Param("id"), input)
	if err != nil {
		writeWorkbenchCommandError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": decision})
}

func (h *WorkbenchCommandHandler) Command(c *gin.Context) {
	if h == nil || h.interactions == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	var command workbench.ExecutionCommand
	if err := c.ShouldBindJSON(&command); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}
	if err := h.interactions.Command(commandContext(c), c.Param("run_id"), command); err != nil {
		writeWorkbenchCommandError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": gin.H{"run_id": strings.TrimSpace(c.Param("run_id")), "action": command.Action}})
}

func writeWorkbenchCommandError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, workbenchservice.ErrCapabilityUnavailable):
		status = http.StatusNotImplemented
	case errors.Is(err, workbench.ErrInteractionActionMismatch), errors.Is(err, workbench.ErrCommandActionMismatch):
		status = http.StatusBadRequest
	case errors.Is(err, workbenchservice.ErrInteractionExpired):
		status = http.StatusGone
	case errors.Is(err, workbenchservice.ErrInteractionRevoked):
		status = http.StatusForbidden
	case errors.Is(err, approval.ErrTenantMismatch), errors.Is(err, approval.ErrUserMismatch):
		status = http.StatusForbidden
	case errors.Is(err, approval.ErrPendingNotFound):
		status = http.StatusNotFound
	case errors.Is(err, approval.ErrAlreadyResolved):
		status = http.StatusConflict
	case errors.Is(err, workbenchservice.ErrInteractionNotFound), errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, agentruntime.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, agentruntime.ErrConflict):
		status = http.StatusConflict
	case strings.Contains(err.Error(), "context is required") || strings.Contains(err.Error(), "actor context is required"):
		status = http.StatusUnauthorized
	}
	c.AbortWithStatusJSON(status, gin.H{"success": false, "error": err.Error()})
}
