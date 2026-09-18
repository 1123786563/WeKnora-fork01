package session

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// OwnedExecutionLister is the ownership-scoped list port. Implementations must
// bind every row to the authenticated tenant and owner; the handler never
// forwards a caller-supplied tenant or owner into the filter.
type OwnedExecutionLister interface {
	ListOwnedExecutions(ctx context.Context, tenantID uint64, ownerID string, filter repository.WorkbenchExecutionFilter) (repository.WorkbenchExecutionPage, error)
}

// WorkbenchListHandler serves GET /workbench/executions: the mobile workbench
// entry list. Authentication and ownership come from the request context, the
// same boundary the single-run read handler relies on.
type WorkbenchListHandler struct {
	lists OwnedExecutionLister
}

func NewWorkbenchListHandler(lists OwnedExecutionLister) *WorkbenchListHandler {
	return &WorkbenchListHandler{lists: lists}
}

func (h *WorkbenchListHandler) ListWorkbenchExecutions(c *gin.Context) {
	if h == nil || h.lists == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		if value, exists := c.Get(types.TenantIDContextKey.String()); exists {
			tenantID, ok = value.(uint64)
		}
	}
	ownerID, ownerOK := types.UserIDFromContext(c.Request.Context())
	if !ownerOK || ownerID == "" {
		if value, exists := c.Get(types.UserIDContextKey.String()); exists {
			ownerID, ownerOK = value.(string)
		}
	}
	if !ok || !ownerOK || tenantID == 0 || ownerID == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	filter := repository.WorkbenchExecutionFilter{
		Status:  strings.TrimSpace(c.Query("status")),
		AgentID: strings.TrimSpace(c.Query("agent_id")),
		Cursor:  strings.TrimSpace(c.Query("cursor")),
	}
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "limit must be a non-negative integer"})
			return
		}
		filter.Limit = limit
	}
	page, err := h.lists.ListOwnedExecutions(c.Request.Context(), tenantID, ownerID, filter)
	if errors.Is(err, repository.ErrWorkbenchCursor) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid cursor"})
		return
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": page})
}
