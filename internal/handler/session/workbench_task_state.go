package session

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// OwnedTaskStateMutator is the ownership-scoped archive port. Tenant and owner
// are always derived from the authenticated context; implementations bind
// every mutation to that identity.
type OwnedTaskStateMutator interface {
	SetTaskArchived(ctx context.Context, tenantID uint64, ownerID, taskID string, archived bool, now time.Time) error
}

// WorkbenchTaskStateHandler serves the task archive lifecycle (T04). Archive
// is a write, so it is NOT mounted behind the W34 read gate — one switch must
// never cut query and archive at the same time.
type WorkbenchTaskStateHandler struct {
	states OwnedTaskStateMutator
}

func NewWorkbenchTaskStateHandler(states OwnedTaskStateMutator) *WorkbenchTaskStateHandler {
	return &WorkbenchTaskStateHandler{states: states}
}

func (h *WorkbenchTaskStateHandler) mutate(c *gin.Context, archived bool) {
	if h == nil || h.states == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	tenantID, tenantOK := types.TenantIDFromContext(c.Request.Context())
	if !tenantOK || tenantID == 0 {
		if value, exists := c.Get(types.TenantIDContextKey.String()); exists {
			tenantID, tenantOK = value.(uint64)
		}
	}
	userID, userOK := types.UserIDFromContext(c.Request.Context())
	if !userOK || userID == "" {
		if value, exists := c.Get(types.UserIDContextKey.String()); exists {
			userID, userOK = value.(string)
		}
	}
	if !tenantOK || !userOK || tenantID == 0 || userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return
	}
	taskID := c.Param("task_id")
	err := h.states.SetTaskArchived(c.Request.Context(), tenantID, userID, taskID, archived, time.Now().UTC())
	switch {
	case errors.Is(err, repository.ErrWorkbenchTaskNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "error": "task not found for owner"})
	case err != nil:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
	default:
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"task_id": taskID, "archived": archived}})
	}
}

func (h *WorkbenchTaskStateHandler) Archive(c *gin.Context) { h.mutate(c, true) }
func (h *WorkbenchTaskStateHandler) Restore(c *gin.Context) { h.mutate(c, false) }
