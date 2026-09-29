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

// OwnedLegacyTaskLister is the ownership-scoped legacy task list port.
// Implementations must bind every row to the authenticated tenant and owner;
// the handler never forwards a caller-supplied tenant or owner.
type OwnedLegacyTaskLister interface {
	ListOwnedLegacyTasks(ctx context.Context, tenantID uint64, ownerID string, filter repository.WorkbenchLegacyFilter) (repository.WorkbenchLegacyPage, error)
}

// WorkbenchLegacyListHandler serves GET /workbench/legacy-tasks (T14): the
// facts-only projection of sessions that never had a Run. Authentication and
// ownership come from the request context, the same boundary the execution
// list relies on.
type WorkbenchLegacyListHandler struct {
	lists OwnedLegacyTaskLister
}

func NewWorkbenchLegacyListHandler(lists OwnedLegacyTaskLister) *WorkbenchLegacyListHandler {
	return &WorkbenchLegacyListHandler{lists: lists}
}

func (h *WorkbenchLegacyListHandler) ListLegacyTasks(c *gin.Context) {
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

	filter := repository.WorkbenchLegacyFilter{
		Query:  c.Query("q"),
		Cursor: strings.TrimSpace(c.Query("cursor")),
	}
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "limit must be a non-negative integer"})
			return
		}
		filter.Limit = limit
	}
	if raw := strings.TrimSpace(c.Query("archived")); raw != "" {
		archived, err := strconv.ParseBool(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "archived must be a boolean"})
			return
		}
		filter.ArchivedOnly = archived
	}
	page, err := h.lists.ListOwnedLegacyTasks(c.Request.Context(), tenantID, ownerID, filter)
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
