package session

// T13 (#43): the compliance HTTP surface. Admin+ lane: metadata by default,
// reasoned+time-limited+audited windows for private content, and the tenant
// policy switches. Identity always arrives from the authenticated context
// (taskGrantCaller shape). The permanent-deletion endpoint joins in Task 6.

import (
	"context"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// TaskComplianceManager is the compliance service port (service.TaskComplianceService).
// Task 6 extends it with PurgeTask once the service side exists.
type TaskComplianceManager interface {
	GetTaskPolicy(ctx context.Context, caller types.Caller) (*types.TenantTaskPolicy, error)
	SetTaskPolicy(ctx context.Context, caller types.Caller, retentionDays int, legalHold bool) (*types.TenantTaskPolicy, error)
	TaskMetadata(ctx context.Context, caller types.Caller, taskID string) (types.TaskMetadataView, error)
	RequestContentAccess(ctx context.Context, caller types.Caller, taskID, reason string, ttl time.Duration) (types.TaskComplianceAccess, error)
	ReadTaskContent(ctx context.Context, caller types.Caller, taskID string) (types.TaskContentView, error)
}

// WorkbenchTaskComplianceHandler serves the T13 compliance lanes.
type WorkbenchTaskComplianceHandler struct {
	compliance TaskComplianceManager
}

// NewWorkbenchTaskComplianceHandler constructs the handler.
func NewWorkbenchTaskComplianceHandler(compliance TaskComplianceManager) *WorkbenchTaskComplianceHandler {
	return &WorkbenchTaskComplianceHandler{compliance: compliance}
}

// complianceWriteError maps service AppErrors onto the workbench envelope.
func complianceWriteError(c *gin.Context, err error) {
	if appErr, ok := apperrors.IsAppError(err); ok {
		status := http.StatusInternalServerError
		switch appErr.Code {
		case apperrors.ErrBadRequest:
			status = http.StatusBadRequest
		case apperrors.ErrForbidden:
			status = http.StatusForbidden
		case apperrors.ErrNotFound:
			status = http.StatusNotFound
		case apperrors.ErrConflict:
			status = http.StatusConflict
		case apperrors.ErrServiceUnavailable:
			status = http.StatusServiceUnavailable
		}
		c.AbortWithStatusJSON(status, gin.H{"success": false, "error": appErr.Message})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "compliance operation failed"})
}

func (h *WorkbenchTaskComplianceHandler) refuseUnassembled(c *gin.Context) bool {
	if h == nil || h.compliance == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return true
	}
	return false
}

// refuseIdentity derives the caller exactly like taskGrantCaller
// (workbench_task_grants.go:55 shape) — tenant and user come only from the
// request context — and repeats the admin predicate so an unguarded mount
// still fails closed (production routes carry g.Admin()). A missing
// identity is 401; an identifiable non-admin is 403.
func (h *WorkbenchTaskComplianceHandler) refuseIdentity(c *gin.Context) (types.Caller, bool) {
	caller := types.CallerFromContext(c.Request.Context()).Normalize()
	if caller.TenantID == 0 || strings.TrimSpace(caller.UserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return types.Caller{}, false
	}
	if caller.Role.Level() < types.TenantRoleAdmin.Level() {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "compliance access requires the tenant admin role"})
		return types.Caller{}, false
	}
	return caller, true
}

// GetTaskPolicy GET /workbench/compliance/task-policy
func (h *WorkbenchTaskComplianceHandler) GetTaskPolicy(c *gin.Context) {
	if h.refuseUnassembled(c) {
		return
	}
	caller, ok := h.refuseIdentity(c)
	if !ok {
		return
	}
	policy, err := h.compliance.GetTaskPolicy(c.Request.Context(), caller)
	if err != nil {
		complianceWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"policy": policy}})
}

// SetTaskPolicy PUT /workbench/compliance/task-policy
func (h *WorkbenchTaskComplianceHandler) SetTaskPolicy(c *gin.Context) {
	if h.refuseUnassembled(c) {
		return
	}
	caller, ok := h.refuseIdentity(c)
	if !ok {
		return
	}
	var input struct {
		RetentionDays int  `json:"retention_days"`
		LegalHold     bool `json:"legal_hold"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "retention_days and legal_hold are required"})
		return
	}
	if input.RetentionDays < 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "retention_days must be >= 0"})
		return
	}
	policy, err := h.compliance.SetTaskPolicy(c.Request.Context(), caller, input.RetentionDays, input.LegalHold)
	if err != nil {
		complianceWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"policy": policy}})
}

// TaskMetadata GET /workbench/compliance/tasks/:task_id — the default,
// metadata-only view.
func (h *WorkbenchTaskComplianceHandler) TaskMetadata(c *gin.Context) {
	if h.refuseUnassembled(c) {
		return
	}
	caller, ok := h.refuseIdentity(c)
	if !ok {
		return
	}
	view, err := h.compliance.TaskMetadata(c.Request.Context(), caller, c.Param("task_id"))
	if err != nil {
		complianceWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"task": view}})
}

// RequestContentAccess POST /workbench/compliance/tasks/:task_id/access
// body: {"reason": string, "ttl_hours": int (1..168)}
func (h *WorkbenchTaskComplianceHandler) RequestContentAccess(c *gin.Context) {
	if h.refuseUnassembled(c) {
		return
	}
	caller, ok := h.refuseIdentity(c)
	if !ok {
		return
	}
	var input struct {
		Reason   string `json:"reason"`
		TTLHours int    `json:"ttl_hours"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "reason and ttl_hours are required"})
		return
	}
	if strings.TrimSpace(input.Reason) == "" || input.TTLHours < 1 || input.TTLHours > 168 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "reason is required and ttl_hours must be within 1..168"})
		return
	}
	access, err := h.compliance.RequestContentAccess(c.Request.Context(), caller, c.Param("task_id"), input.Reason, time.Duration(input.TTLHours)*time.Hour)
	if err != nil {
		complianceWriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"access": access}})
}

// ReadTaskContent GET /workbench/compliance/tasks/:task_id/content
func (h *WorkbenchTaskComplianceHandler) ReadTaskContent(c *gin.Context) {
	if h.refuseUnassembled(c) {
		return
	}
	caller, ok := h.refuseIdentity(c)
	if !ok {
		return
	}
	content, err := h.compliance.ReadTaskContent(c.Request.Context(), caller, c.Param("task_id"))
	if err != nil {
		complianceWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"content": content}})
}
