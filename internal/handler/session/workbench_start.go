package session

import (
	"context"
	"errors"
	"net/http"
	"strings"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type WorkbenchStartHandler struct {
	admission *workbenchservice.AdmissionCoordinator
}

func NewWorkbenchStartHandler(admission *workbenchservice.AdmissionCoordinator) *WorkbenchStartHandler {
	return &WorkbenchStartHandler{admission: admission}
}

func (h *WorkbenchStartHandler) Start(c *gin.Context) {
	if h == nil || h.admission == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	var input workbenchservice.StartInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}
	if strings.TrimSpace(input.RequestID) == "" || strings.TrimSpace(input.SessionID) == "" || strings.TrimSpace(input.Text) == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "session_id, request_id and text are required"})
		return
	}
	ctx := c.Request.Context()
	// Unit handlers and a few legacy middleware paths populate Gin keys while
	// the auth middleware normally also copies them into context.Context.
	// Normalize both forms before crossing into the service boundary.
	if tenant, ok := c.Get(types.TenantIDContextKey.String()); ok {
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
	}
	if actor, ok := c.Get(types.UserIDContextKey.String()); ok {
		ctx = context.WithValue(ctx, types.UserIDContextKey, actor)
	}
	run, err := h.admission.Start(ctx, input)
	if err != nil {
		writeWorkbenchAdmissionError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": gin.H{"run_id": run.Key.RunID, "request_id": run.RequestID, "status": run.Status}})
}

func (h *WorkbenchStartHandler) Lookup(c *gin.Context) {
	if h == nil || h.admission == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	ctx := c.Request.Context()
	if tenant, ok := c.Get(types.TenantIDContextKey.String()); ok {
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
	}
	if actor, ok := c.Get(types.UserIDContextKey.String()); ok {
		ctx = context.WithValue(ctx, types.UserIDContextKey, actor)
	}
	state, err := h.admission.LookupRequest(ctx, c.Param("request_id"))
	if err != nil {
		writeWorkbenchAdmissionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": state})
}

func writeWorkbenchAdmissionError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		status = http.StatusNotFound
	case errors.Is(err, agentruntime.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, agentruntime.ErrRunActive):
		status = http.StatusConflict
	case errors.Is(err, workbenchservice.ErrRequestPending):
		status = http.StatusAccepted
	case errors.Is(err, workbenchservice.ErrRequestRejected):
		status = http.StatusConflict
	case strings.Contains(err.Error(), "context is required"):
		status = http.StatusUnauthorized
	}
	c.AbortWithStatusJSON(status, gin.H{"success": false, "error": err.Error()})
}
