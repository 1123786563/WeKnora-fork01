package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/workbench"
	"github.com/gin-gonic/gin"
)

type OwnedRunReader interface {
	GetOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string) (agentruntime.Run, error)
}

type WorkbenchSnapshotReader interface {
	ReadRunSnapshot(ctx context.Context, key agentruntime.RunKey) (workbench.ExecutionSnapshot, error)
	ReadRunEvents(ctx context.Context, key agentruntime.RunKey, cursor int64, limit int) ([]workbench.ExecutionEvent, int64, error)
}

// WorkbenchReadHandler is the ownership boundary for the mobile workbench.
// Every operation resolves the run through GetOwnedRun before reading a
// snapshot or event projection.
type WorkbenchReadHandler struct {
	runs      OwnedRunReader
	snapshots WorkbenchSnapshotReader
}

func NewWorkbenchReadHandler(runs OwnedRunReader, snapshots WorkbenchSnapshotReader) *WorkbenchReadHandler {
	return &WorkbenchReadHandler{runs: runs, snapshots: snapshots}
}

func (h *WorkbenchReadHandler) owned(c *gin.Context) (agentruntime.RunKey, bool) {
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
	if !ok || !ownerOK || tenantID == 0 || ownerID == "" || h == nil || h.runs == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return agentruntime.RunKey{}, false
	}
	runID := strings.TrimSpace(c.Param("run_id"))
	run, err := h.runs.GetOwnedRun(c.Request.Context(), tenantID, ownerID, runID)
	if errors.Is(err, agentruntime.ErrNotFound) {
		c.AbortWithStatus(http.StatusNotFound)
		return agentruntime.RunKey{}, false
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return agentruntime.RunKey{}, false
	}
	return run.Key, true
}

func writeWorkbenchJSON(c *gin.Context, value any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": value})
}

func (h *WorkbenchReadHandler) GetWorkbenchExecution(c *gin.Context) {
	key, ok := h.owned(c)
	if !ok || h.snapshots == nil {
		return
	}
	snapshot, err := h.snapshots.ReadRunSnapshot(c.Request.Context(), key)
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	writeWorkbenchJSON(c, snapshot.Execution)
}

func (h *WorkbenchReadHandler) GetWorkbenchSnapshot(c *gin.Context) {
	key, ok := h.owned(c)
	if !ok || h.snapshots == nil {
		return
	}
	snapshot, err := h.snapshots.ReadRunSnapshot(c.Request.Context(), key)
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	writeWorkbenchJSON(c, snapshot)
}

func (h *WorkbenchReadHandler) StreamWorkbenchEvents(c *gin.Context) {
	key, ok := h.owned(c)
	if !ok || h.snapshots == nil {
		return
	}
	cursor := int64(0)
	if raw := strings.TrimSpace(c.GetHeader("Last-Event-ID")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		cursor = parsed
	}
	events, _, err := h.snapshots.ReadRunEvents(c.Request.Context(), key, cursor, 256)
	if err != nil {
		if errors.Is(err, agentruntime.ErrCursorExpired) {
			c.AbortWithStatus(http.StatusConflict)
			return
		}
		writeWorkbenchError(c, err)
		return
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	for _, event := range events {
		if err := writeWorkbenchSSE(c.Writer, event.Seq, event.Type, event.Payload); err != nil {
			return
		}
		if f, ok := c.Writer.(http.Flusher); ok {
			f.Flush()
		}
	}
	// A short heartbeat keeps proxies from expiring a newly-opened stream;
	// clients reconnect using Last-Event-ID. It is intentionally bounded so a
	// request disconnect never owns a goroutine after this handler returns.
	if len(events) == 0 {
		_, _ = io.WriteString(c.Writer, ": heartbeat\n\n")
	}
}

func writeWorkbenchError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, agentruntime.ErrNotFound) {
		status = http.StatusNotFound
	}
	c.AbortWithStatusJSON(status, gin.H{"success": false, "error": err.Error()})
}

func writeWorkbenchSSE(w io.Writer, seq int64, kind string, raw json.RawMessage) error {
	if seq < 1 || strings.TrimSpace(kind) == "" || strings.ContainsAny(kind, "\r\n") || !json.Valid(raw) {
		return errors.New("invalid_event")
	}
	var data bytes.Buffer
	if err := json.Compact(&data, raw); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", seq, kind, data.Bytes()); err != nil {
		return err
	}
	return nil
}
