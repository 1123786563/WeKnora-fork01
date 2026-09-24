package session

import (
	"context"
	"net/http"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// TaskGrantManager is the task-grant service port. Identity always arrives
// from the authenticated context; implementations bind every operation to
// the caller's tenant and the owner-only predicate.
type TaskGrantManager interface {
	GrantTaskAccess(ctx context.Context, caller types.Caller, taskID, granteeID string, role types.TaskGrantRole) (*types.TaskGrant, error)
	RevokeTaskAccess(ctx context.Context, caller types.Caller, taskID, granteeID string) error
	ListTaskGrants(ctx context.Context, caller types.Caller, taskID string) ([]types.TaskGrant, error)
}

// WorkbenchTaskGrantsHandler serves the per-task collaboration grants (T12).
// Same Viewer+/chat-capability route boundary as the other workbench lanes;
// the owner-only predicate lives in the service.
type WorkbenchTaskGrantsHandler struct {
	grants TaskGrantManager
}

func NewWorkbenchTaskGrantsHandler(grants TaskGrantManager) *WorkbenchTaskGrantsHandler {
	return &WorkbenchTaskGrantsHandler{grants: grants}
}

// taskGrantWriteError maps service AppErrors onto the workbench envelope.
func taskGrantWriteError(c *gin.Context, err error) {
	if appErr, ok := apperrors.IsAppError(err); ok {
		status := http.StatusInternalServerError
		switch appErr.Code {
		case apperrors.ErrBadRequest:
			status = http.StatusBadRequest
		case apperrors.ErrForbidden:
			status = http.StatusForbidden
		case apperrors.ErrNotFound:
			status = http.StatusNotFound
		}
		c.AbortWithStatusJSON(status, gin.H{"success": false, "error": appErr.Message})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "task grant operation failed"})
}

// taskGrantCaller derives the authenticated caller the same way the other
// workbench lanes do (resolveOwnedRun / workbench_task_state shape): tenant
// and user come only from the request context, never from the URL or body.
// An empty identity is refused before the service is reached.
func taskGrantCaller(c *gin.Context) (types.Caller, bool) {
	caller := types.CallerFromContext(c.Request.Context())
	if caller.TenantID == 0 || strings.TrimSpace(caller.UserID) == "" {
		return types.Caller{}, false
	}
	return caller.Normalize(), true
}

// Grant POST /workbench/tasks/:task_id/grants — assign or rewrite one
// member's role. grantee_id and role come from the body; identity never does.
func (h *WorkbenchTaskGrantsHandler) Grant(c *gin.Context) {
	if h == nil || h.grants == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	caller, ok := taskGrantCaller(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return
	}
	var input struct {
		GranteeID string `json:"grantee_id"`
		Role      string `json:"role"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "grantee_id and role are required"})
		return
	}
	role := types.TaskGrantRole(input.Role)
	if !role.IsValid() {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "role must be viewer or collaborator"})
		return
	}
	grant, err := h.grants.GrantTaskAccess(c.Request.Context(), caller, c.Param("task_id"), input.GranteeID, role)
	if err != nil {
		taskGrantWriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"grant": grant}})
}

// Revoke DELETE /workbench/tasks/:task_id/grants/:grantee_id — idempotent.
func (h *WorkbenchTaskGrantsHandler) Revoke(c *gin.Context) {
	if h == nil || h.grants == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	caller, ok := taskGrantCaller(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return
	}
	taskID := c.Param("task_id")
	granteeID := c.Param("grantee_id")
	if err := h.grants.RevokeTaskAccess(c.Request.Context(), caller, taskID, granteeID); err != nil {
		taskGrantWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"task_id": taskID, "grantee_id": granteeID, "revoked": true}})
}

// List GET /workbench/tasks/:task_id/grants — owner-only member list.
func (h *WorkbenchTaskGrantsHandler) List(c *gin.Context) {
	if h == nil || h.grants == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	caller, ok := taskGrantCaller(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return
	}
	grants, err := h.grants.ListTaskGrants(c.Request.Context(), caller, c.Param("task_id"))
	if err != nil {
		taskGrantWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"grants": grants}})
}
