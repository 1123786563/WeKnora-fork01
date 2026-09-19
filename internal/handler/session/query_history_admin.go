package session

import (
	stderrors "errors"
	"net/http"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// Admin query-history audit surface (SP13). This file carries the per-session
// snapshot endpoint; the async export endpoints (Task 4) will live here too.

// GetQueryHistorySnapshot godoc
// @Summary      获取会话审计快照
// @Description  Admin+ 审计视图：返回会话行、最近消息（上限 200，超出截断并置 truncated=true）以及该会话的全部反馈；租户查询历史策略为 disabled 时返回 403，为 anonymized 时会话与反馈的用户标识脱敏为 anonymous
// @Tags         会话
// @Produce      json
// @Param        session_id  path  string  true  "会话ID"
// @Success      200  {object}  map[string]interface{}  "会话快照"
// @Failure      403  {object}  errors.AppError         "租户停用查询历史"
// @Failure      404  {object}  errors.AppError         "会话不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /admin/sessions/{session_id}/snapshot [get]
func (h *Handler) GetQueryHistorySnapshot(c *gin.Context) {
	ctx := c.Request.Context()

	sessionID := secutils.SanitizeForLog(c.Param("session_id"))
	if sessionID == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		logger.Error(ctx, "Workspace ID not found in context")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidTenantID.Error()))
		return
	}

	snapshot, err := h.sessionService.GetQueryHistorySnapshot(ctx, tenantID, sessionID)
	if err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			logger.Warnf(ctx, "Session not found, ID: %s", sessionID)
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		if appErr, ok := errors.IsAppError(err); ok {
			// Policy denials (query history disabled) keep their own status.
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": sessionID,
			"tenant_id":  tenantID,
		})
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    snapshot,
	})
}
