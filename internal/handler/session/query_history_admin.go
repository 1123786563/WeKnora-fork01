package session

import (
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// Admin query-history audit surface (SP13): the per-session snapshot and the
// async CSV export. The privacy policy (disabled / anonymized) is enforced
// here at the HTTP entrance via the export service's CheckAccess — one
// choke point per endpoint — while the asynq worker re-checks at processing
// time so an export enqueued before a policy change never leaks.

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

// queryHistoryExportRequest is the optional filter body of the export POST.
// All fields mirror the audit listing's query filters; an absent body means
// "export the whole tenant".
type queryHistoryExportRequest struct {
	UserID    string `json:"user_id"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Feedback  string `json:"feedback"`
}

// queryHistoryExportTenantID resolves the caller's workspace, answering
// false (with the response already written) on failure.
func queryHistoryExportTenantID(c *gin.Context) (uint64, bool) {
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		logger.Error(c.Request.Context(), "Workspace ID not found in context")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidTenantID.Error()))
		return 0, false
	}
	return tenantID, true
}

// queryHistoryExportGate enforces the tenant's query-history privacy policy.
// It returns false once it has written the error response (403 policy denial
// or 500 lookup failure).
func (h *Handler) queryHistoryExportGate(c *gin.Context, tenantID uint64) bool {
	if _, err := h.queryHistoryExport.CheckAccess(c.Request.Context(), tenantID); err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return false
		}
		logger.ErrorWithFields(c.Request.Context(), err, map[string]interface{}{
			"tenant_id": tenantID,
		})
		c.Error(errors.NewInternalServerError(err.Error()))
		return false
	}
	return true
}

// parseQueryHistoryExportJobID reads the :job_id path parameter.
func parseQueryHistoryExportJobID(c *gin.Context) (uint64, bool) {
	raw := strings.TrimSpace(c.Param("job_id"))
	jobID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || jobID == 0 {
		c.Error(errors.NewBadRequestError("invalid job_id"))
		return 0, false
	}
	return jobID, true
}

// StartQueryHistoryExport godoc
// @Summary      发起查询历史 CSV 导出
// @Description  Admin+ 审计导出：创建导出任务并异步生成 CSV（每会话一行汇总，含消息数与点赞/点踩计数）；过滤条件同 source=all 审计列表；租户查询历史策略为 disabled 时返回 403，anonymized 时导出中的用户标识脱敏为 anonymous
// @Tags         会话
// @Accept       json
// @Produce      json
// @Param        request  body  queryHistoryExportRequest  false  "可选过滤：user_id / start_time / end_time / feedback（like 或 dislike）"
// @Success      200  {object}  map[string]interface{}  "job_id"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      403  {object}  errors.AppError         "租户停用查询历史"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /admin/sessions/export [post]
func (h *Handler) StartQueryHistoryExport(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := queryHistoryExportTenantID(c)
	if !ok {
		return
	}

	var request queryHistoryExportRequest
	// An empty body is a valid "export everything" request.
	if err := c.ShouldBindJSON(&request); err != nil && !stderrors.Is(err, io.EOF) {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	startTime, err := parseSessionFilterTime(request.StartTime)
	if err != nil {
		c.Error(errors.NewBadRequestError("invalid start_time: " + err.Error()))
		return
	}
	endTime, err := parseSessionFilterTime(request.EndTime)
	if err != nil {
		c.Error(errors.NewBadRequestError("invalid end_time: " + err.Error()))
		return
	}
	feedback := strings.TrimSpace(request.Feedback)
	switch feedback {
	case "", types.FeedbackRatingLike, types.FeedbackRatingDislike:
	default:
		c.Error(errors.NewBadRequestError("invalid feedback: must be like or dislike"))
		return
	}

	if !h.queryHistoryExportGate(c, tenantID) {
		return
	}

	requestedBy, _ := types.UserIDFromContext(ctx)
	jobID, err := h.queryHistoryExport.StartExport(ctx, tenantID, requestedBy, types.SessionListQuery{
		UserID:         strings.TrimSpace(request.UserID),
		StartTime:      startTime,
		EndTime:        endTime,
		FeedbackRating: feedback,
	})
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"tenant_id": tenantID})
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"job_id": jobID},
	})
}

// GetQueryHistoryExportStatus godoc
// @Summary      查询导出任务状态
// @Description  Admin+ 查询查询历史导出任务的状态（pending/running/done/failed）与错误信息；任务不存在或跨租户返回 404
// @Tags         会话
// @Produce      json
// @Param        job_id  path  string  true  "导出任务ID"
// @Success      200  {object}  map[string]interface{}  "status, error_message"
// @Failure      400  {object}  errors.AppError         "job_id 非法"
// @Failure      403  {object}  errors.AppError         "租户停用查询历史"
// @Failure      404  {object}  errors.AppError         "任务不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /admin/sessions/export/{job_id}/status [get]
func (h *Handler) GetQueryHistoryExportStatus(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := queryHistoryExportTenantID(c)
	if !ok {
		return
	}
	jobID, ok := parseQueryHistoryExportJobID(c)
	if !ok {
		return
	}
	if !h.queryHistoryExportGate(c, tenantID) {
		return
	}

	job, err := h.queryHistoryExport.GetExportJob(ctx, tenantID, jobID)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenantID,
			"job_id":    jobID,
		})
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"job_id":        job.ID,
			"status":        job.Status,
			"error_message": job.ErrorMessage,
			"file_path":     job.FilePath,
			"requested_by":  job.RequestedBy,
			"created_at":    job.CreatedAt,
			"updated_at":    job.UpdatedAt,
		},
	})
}

// DownloadQueryHistoryExport godoc
// @Summary      下载已完成的导出 CSV
// @Description  Admin+ 下载查询历史导出文件（仅 status=done 可下载，流式回吐并附 BOM 便于 Excel 打开）；pending/running/failed 返回 400
// @Tags         会话
// @Produce      text/csv
// @Param        job_id  path  string  true  "导出任务ID"
// @Success      200  {string}  string  "CSV 文件流"
// @Failure      400  {object}  errors.AppError  "任务未完成或 job_id 非法"
// @Failure      403  {object}  errors.AppError  "租户停用查询历史"
// @Failure      404  {object}  errors.AppError  "任务不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /admin/sessions/export/{job_id}/download [get]
func (h *Handler) DownloadQueryHistoryExport(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := queryHistoryExportTenantID(c)
	if !ok {
		return
	}
	jobID, ok := parseQueryHistoryExportJobID(c)
	if !ok {
		return
	}
	if !h.queryHistoryExportGate(c, tenantID) {
		return
	}

	job, err := h.queryHistoryExport.GetExportJob(ctx, tenantID, jobID)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenantID,
			"job_id":    jobID,
		})
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	if job.Status != types.QueryHistoryExportDone || job.FilePath == "" {
		message := "export is not ready: status=" + job.Status
		if job.ErrorMessage != "" {
			message += ", error=" + job.ErrorMessage
		}
		c.Error(errors.NewBadRequestError(message))
		return
	}

	reader, err := h.fileService.GetFile(ctx, job.FilePath)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenantID,
			"job_id":    jobID,
			"file_path": job.FilePath,
		})
		c.Error(errors.NewInternalServerError("export file is no longer available"))
		return
	}
	defer func() { _ = reader.Close() }()

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition",
		fmt.Sprintf("attachment; filename=query_history_export_%d.csv", job.ID))
	c.Status(http.StatusOK)
	// BOM first so Excel detects UTF-8 (same contract as the usage export),
	// then stream the stored bytes.
	if _, err := c.Writer.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		logger.Warnf(ctx, "query history export %d: write BOM failed: %v", job.ID, err)
		return
	}
	if _, err := io.Copy(c.Writer, reader); err != nil {
		logger.Warnf(ctx, "query history export %d: stream failed: %v", job.ID, err)
	}
}
