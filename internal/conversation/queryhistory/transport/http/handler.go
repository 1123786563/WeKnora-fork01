// Package httptransport is the Query History module's HTTP transport (Wave 1,
// Task 8): the Admin+ audit snapshot and the async CSV export endpoints moved
// byte-identically from the legacy internal/handler/session package. The
// handler owns request parsing and response mapping only — no business logic.
// It consumes the application layer through the consumer-side interfaces
// declared here (satisfied structurally by application.AuditService and, once
// Task 6 lands, application.ExportService), so transport tests run on fakes.
// The AppError → HTTP status mapping is the legacy one: handlers attach the
// error with c.Error and the platform's error middleware renders it.
package httptransport

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
)

// AuditUseCases is the slice of the application layer the transport consumes
// for the audit surface. It matches application.AuditService exactly
// (Task 5); the export endpoints' privacy gate rides on CheckAccess too,
// replacing the legacy queryHistoryExport.CheckAccess call.
type AuditUseCases interface {
	// CheckAccess enforces the tenant query-history privacy policy: disabled
	// answers a ForbiddenError; anonymized / normal return the mode.
	CheckAccess(ctx context.Context, tenantID uint64) (domain.Mode, error)
	// Snapshot loads one session's audit snapshot, honoring the tenant's
	// query-history privacy policy (403 when disabled, masked when
	// anonymized).
	Snapshot(ctx context.Context, tenantID uint64, sessionID string) (*types.QueryHistorySnapshot, error)
}

// ExportUseCases is the slice of the application layer the transport consumes
// for the async CSV export. It matches application.ExportService's frozen
// signatures (Task 6); Open returns the stored bytes WITHOUT the UTF-8 BOM —
// the handler writes the BOM once before streaming — plus the download
// filename for the Content-Disposition header.
type ExportUseCases interface {
	// Start admits a pending export job and enqueues the worker task.
	Start(ctx context.Context, tenantID uint64, requestedBy string, filter domain.ExportFilter) (uint64, error)
	// Job loads one export job scoped to the caller's tenant.
	Job(ctx context.Context, tenantID uint64, jobID uint64) (*domain.ExportJob, error)
	// Open streams a finished export's bytes and answers its filename.
	Open(ctx context.Context, tenantID uint64, jobID uint64) (io.ReadCloser, string, error)
}

// Handler serves the Query History module's HTTP surface.
type Handler struct {
	audit   AuditUseCases
	exports ExportUseCases
}

// NewHandler wires the transport onto its use cases.
func NewHandler(audit AuditUseCases, exports ExportUseCases) *Handler {
	return &Handler{audit: audit, exports: exports}
}

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

	snapshot, err := h.audit.Snapshot(ctx, tenantID, sessionID)
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
	if _, err := h.audit.CheckAccess(c.Request.Context(), tenantID); err != nil {
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
	jobID, err := h.exports.Start(ctx, tenantID, requestedBy, domain.ExportFilter{
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

	job, err := h.exports.Job(ctx, tenantID, jobID)
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

	job, err := h.exports.Job(ctx, tenantID, jobID)
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
	if job.Status != domain.ExportDone || job.FilePath == "" {
		message := "export is not ready: status=" + job.Status
		if job.ErrorMessage != "" {
			message += ", error=" + job.ErrorMessage
		}
		c.Error(errors.NewBadRequestError(message))
		return
	}

	reader, filename, err := h.exports.Open(ctx, tenantID, jobID)
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
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Status(http.StatusOK)
	// BOM first so Excel detects UTF-8 (same contract as the usage export),
	// then stream the stored bytes — Open answers them WITHOUT the BOM, so
	// the marker is written exactly once, here.
	if _, err := c.Writer.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		logger.Warnf(ctx, "query history export %d: write BOM failed: %v", job.ID, err)
		return
	}
	if _, err := io.Copy(c.Writer, reader); err != nil {
		logger.Warnf(ctx, "query history export %d: stream failed: %v", job.ID, err)
	}
}

// parseSessionFilterTime parses an optional session-list timestamp filter.
// It accepts the same layouts as the audit listing's query filters (ported
// verbatim from the legacy session handler): RFC3339 (with or without
// fractional seconds), "2006-01-02 15:04:05", and the date-only "2006-01-02"
// form interpreted at start of day in the local timezone. An empty value
// leaves the range bound open.
func parseSessionFilterTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	var lastErr error
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}
