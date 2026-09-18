package session

import (
	"net/http"

	workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// WorkbenchOverviewHandler exposes the aggregate workbench read model (MX-013).
// Identity always comes from the authenticated context; the service applies the
// tenant+owner predicate.
type WorkbenchOverviewHandler struct {
	overview *workbenchservice.OverviewService
}

func NewWorkbenchOverviewHandler(overview *workbenchservice.OverviewService) *WorkbenchOverviewHandler {
	return &WorkbenchOverviewHandler{overview: overview}
}

func (h *WorkbenchOverviewHandler) Overview(c *gin.Context) {
	if h == nil || h.overview == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	// 与 WorkbenchReadHandler.owned 相同的双通道身份提取（context 优先，gin 回退）
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
	data, err := h.overview.Overview(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
