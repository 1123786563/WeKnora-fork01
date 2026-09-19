package session

// Session share endpoints (SP13 Task 5): mint/rotate and revoke a session's
// share token (owner-or-Admin+, enforced in the service), and the read-only
// shared snapshot a token opens for same-tenant logged-in members. The token
// is only ever revealed by POST /share — Session.ShareToken is json:"-", so
// no other session payload can leak it.

import (
	stderrors "errors"
	"net/http"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// shareSessionID resolves the session id from either wildcard name: the POST
// share route registers :session_id while the DELETE route must reuse :id
// (gin keeps one radix tree per verb and the existing DELETE /sessions/:id
// owns that wildcard position — same pattern as the pin routes).
func shareSessionID(c *gin.Context) string {
	if v := c.Param("session_id"); v != "" {
		return secutils.SanitizeForLog(v)
	}
	return secutils.SanitizeForLog(c.Param("id"))
}

// ShareSession godoc
// @Summary      生成/轮换会话分享链接
// @Description  为会话生成只读分享 token（仅 owner 本人或 Admin+）；会话已分享时再次调用会轮换 token，旧链接立即失效。token 仅本次响应返回，不会出现在任何会话载荷中
// @Tags         会话
// @Produce      json
// @Param        session_id  path  string  true  "会话ID"
// @Success      200  {object}  map[string]interface{}  "share_token"
// @Failure      400  {object}  errors.AppError         "会话ID为空"
// @Failure      403  {object}  errors.AppError         "非 owner 且非 Admin+"
// @Failure      404  {object}  errors.AppError         "会话不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{session_id}/share [post]
func (h *Handler) ShareSession(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := shareSessionID(c)
	if sessionID == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	token, err := h.sessionService.ShareSession(ctx, types.CallerFromContext(ctx), sessionID)
	if err != nil {
		shareWriteError(c, err, sessionID)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"share_token": token},
	})
}

// UnshareSession godoc
// @Summary      撤销会话分享链接
// @Description  撤销会话的分享 token（仅 owner 本人或 Admin+），旧链接立即失效；撤销未分享的会话同样是成功（幂等）
// @Tags         会话
// @Produce      json
// @Param        id  path  string  true  "会话ID"
// @Success      200  {object}  map[string]interface{}  "撤销成功"
// @Failure      400  {object}  errors.AppError         "会话ID为空"
// @Failure      403  {object}  errors.AppError         "非 owner 且非 Admin+"
// @Failure      404  {object}  errors.AppError         "会话不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id}/share [delete]
func (h *Handler) UnshareSession(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := shareSessionID(c)
	if sessionID == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	if err := h.sessionService.UnshareSession(ctx, types.CallerFromContext(ctx), sessionID); err != nil {
		shareWriteError(c, err, sessionID)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{}})
}

// shareWriteError maps a share-mutation service error onto the HTTP surface:
// AppErrors (403 gate, 404 miss, 400 validation) keep their own status, the
// legacy ErrSessionNotFound sentinel maps to 404, everything else is a 500
// without echoing internals.
func shareWriteError(c *gin.Context, err error, sessionID string) {
	if appErr, ok := errors.IsAppError(err); ok {
		c.Error(appErr)
		return
	}
	if stderrors.Is(err, errors.ErrSessionNotFound) {
		c.Error(errors.NewNotFoundError("session not found"))
		return
	}
	logger.ErrorWithFields(c.Request.Context(), err, map[string]interface{}{
		"session_id": sessionID,
	})
	c.Error(errors.NewInternalServerError(err.Error()))
}

// GetSharedSession godoc
// @Summary      通过分享 token 读取会话只读快照
// @Description  解析分享 token（同租户登录成员，Viewer+），返回与审计快照同构的只读数据（会话行 + 最近 200 条消息，不含反馈明细）。token 不存在、已撤销、跨租户或会话已删除统一返回 404，不区分原因
// @Tags         会话
// @Produce      json
// @Param        token  path  string  true  "分享token"
// @Success      200  {object}  map[string]interface{}  "SharedSessionSnapshot"
// @Failure      404  {object}  errors.AppError         "shared session not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /shared/sessions/{token} [get]
func (h *Handler) GetSharedSession(c *gin.Context) {
	ctx := c.Request.Context()
	token := c.Param("token")

	snapshot, err := h.sessionService.GetSharedSession(ctx, types.CallerFromContext(ctx), token)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		// A repository sentinel miss maps onto the same uniform 404 the
		// service answers, so the status never hints at the failure flavor.
		if stderrors.Is(err, errors.ErrSessionNotFound) || stderrors.Is(err, gorm.ErrRecordNotFound) {
			c.Error(errors.NewNotFoundError("shared session not found"))
			return
		}
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			// Never log the token itself: it is the share link's bearer
			// credential. Its length is enough for correlation.
			"token_length": len(token),
		})
		c.Error(errors.NewInternalServerError("failed to load shared session"))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    snapshot,
	})
}
