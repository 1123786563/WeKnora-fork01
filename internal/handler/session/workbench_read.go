package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
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

type WorkbenchSourceIngestor interface {
	IngestSourceEvent(ctx context.Context, bindingID string, source repository.SourceObservation) (workbench.ExecutionEvent, error)
}

// WorkbenchReadHandler is the ownership boundary for the mobile workbench.
// Every operation resolves the run through GetOwnedRun before reading a
// snapshot or event projection.
type WorkbenchReadHandler struct {
	runs      OwnedRunReader
	snapshots WorkbenchSnapshotReader
	ingestor  WorkbenchSourceIngestor
}

const (
	workbenchEventPageSize = 256
	workbenchPollInterval  = 250 * time.Millisecond
	workbenchHeartbeat     = 15 * time.Second
)

func NewWorkbenchReadHandler(runs OwnedRunReader, snapshots WorkbenchSnapshotReader, ingestor ...WorkbenchSourceIngestor) *WorkbenchReadHandler {
	var source WorkbenchSourceIngestor
	if len(ingestor) > 0 {
		source = ingestor[0]
	}
	return &WorkbenchReadHandler{runs: runs, snapshots: snapshots, ingestor: source}
}

type sourceEventRequest struct {
	BindingID   string          `json:"binding_id"`
	Generation  string          `json:"generation"`
	EventID     string          `json:"event_id"`
	AttemptID   string          `json:"attempt_id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	PayloadHash string          `json:"payload_hash"`
	SourceSeq   int64           `json:"source_seq"`
}

// IngestWorkbenchSourceEvent is the authenticated callback boundary used by a
// Paseo bridge. Ownership is checked before the source is persisted; the
// repository then re-resolves binding tenant/run inside its write transaction.
func (h *WorkbenchReadHandler) IngestWorkbenchSourceEvent(c *gin.Context) {
	key, ok := h.owned(c)
	if !ok || h.ingestor == nil {
		return
	}
	var request sourceEventRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	event, err := h.ingestor.IngestSourceEvent(c.Request.Context(), request.BindingID, repository.SourceObservation{BindingID: request.BindingID, Generation: request.Generation, EventID: request.EventID, AttemptID: request.AttemptID, Type: request.Type, Payload: request.Payload, PayloadHash: request.PayloadHash, SourceSeq: request.SourceSeq})
	if errors.Is(err, repository.ErrSourceBinding) || (err == nil && event.RunID != key.RunID) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if errors.Is(err, repository.ErrSourceConflict) || errors.Is(err, agentruntime.ErrConflict) {
		c.AbortWithStatus(http.StatusConflict)
		return
	}
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": event})
}

func (h *WorkbenchReadHandler) owned(c *gin.Context) (agentruntime.RunKey, bool) {
	run, ok := resolveOwnedRun(c, h.runs)
	return run.Key, ok
}

// resolveOwnedRun is the shared ownership predicate for workbench read
// surfaces. It re-reads the run through GetOwnedRun so tenant/owner scoping
// is enforced by the durable store, not by URL trust. The full run is
// returned because artifact surfaces additionally need the session binding.
func resolveOwnedRun(c *gin.Context, runs OwnedRunReader) (agentruntime.Run, bool) {
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
	if !ok || !ownerOK || tenantID == 0 || ownerID == "" || runs == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return agentruntime.Run{}, false
	}
	runID := strings.TrimSpace(c.Param("run_id"))
	run, err := runs.GetOwnedRun(c.Request.Context(), tenantID, ownerID, runID)
	if errors.Is(err, agentruntime.ErrNotFound) {
		c.AbortWithStatus(http.StatusNotFound)
		return agentruntime.Run{}, false
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return agentruntime.Run{}, false
	}
	return run, true
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
	// Product stream version negotiation: v1 keeps the legacy payload-only
	// data line for existing callers; v2 carries the full envelope and
	// explicit control frames (MX-004 byte contract).
	version := 1
	if raw := strings.TrimSpace(c.Query("version")); raw != "" {
		if raw != "1" && raw != "2" {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		version, _ = strconv.Atoi(raw)
	}
	// Perform the first read before committing the response. This preserves the
	// HTTP 409 cursor-expired contract for reconnects whose history was trimmed.
	events, _, err := h.snapshots.ReadRunEvents(c.Request.Context(), key, cursor, workbenchEventPageSize)
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
	if !flushWorkbenchEvents(c, events, version) {
		return
	}
	for _, event := range events {
		if event.Seq > cursor {
			cursor = event.Seq
		}
	}

	// The loop drains every retained page, then waits for live events while the
	// run is non-terminal. Request cancellation exits the loop; it never calls
	// a run mutation or cancels the durable worker.
	poll := time.NewTicker(workbenchPollInterval)
	defer poll.Stop()
	heartbeat := time.NewTicker(workbenchHeartbeat)
	defer heartbeat.Stop()
	for {
		snapshot, snapshotErr := h.snapshots.ReadRunSnapshot(c.Request.Context(), key)
		if snapshotErr != nil {
			writeWorkbenchStreamError(c, cursor, snapshotErr, version)
			return
		}
		if isTerminalWorkbenchStatus(snapshot.Execution.RunStatus) && len(events) < workbenchEventPageSize {
			return
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = io.WriteString(c.Writer, ": heartbeat\n\n")
			flushWorkbenchWriter(c)
		case <-poll.C:
			next, _, readErr := h.snapshots.ReadRunEvents(c.Request.Context(), key, cursor, workbenchEventPageSize)
			if readErr != nil {
				writeWorkbenchStreamError(c, cursor, readErr, version)
				return
			}
			if len(next) == 0 {
				events = nil // the current cursor is fully drained
				continue
			}
			if !flushWorkbenchEvents(c, next, version) {
				return
			}
			for _, event := range next {
				if event.Seq > cursor {
					cursor = event.Seq
				}
			}
			events = next
		}
	}
}

func isTerminalWorkbenchStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "canceled":
		return true
	default:
		return false
	}
}

func flushWorkbenchEvents(c *gin.Context, events []workbench.ExecutionEvent, version int) bool {
	events = normalizeWorkbenchEvents(events)
	for _, event := range events {
		var err error
		if version == 2 {
			err = writeWorkbenchSSEV2(c.Writer, event)
		} else {
			err = writeWorkbenchSSE(c.Writer, event.Seq, event.Type, event.Payload)
		}
		if err != nil {
			return false
		}
		flushWorkbenchWriter(c)
	}
	return true
}

// normalizeWorkbenchEvents is a defensive client-stream boundary. Durable
// ingestion already de-duplicates source events, but reconnect adapters may
// still hand us a repeated or out-of-order page. Product seq is authoritative:
// emit each seq once and in ascending order without dropping unknown types.
func normalizeWorkbenchEvents(events []workbench.ExecutionEvent) []workbench.ExecutionEvent {
	seen := make(map[int64]struct{}, len(events))
	result := make([]workbench.ExecutionEvent, 0, len(events))
	for _, event := range events {
		if _, exists := seen[event.Seq]; exists {
			continue
		}
		seen[event.Seq] = struct{}{}
		result = append(result, event)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Seq < result[j].Seq })
	return result
}

func flushWorkbenchWriter(c *gin.Context) {
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
}

func writeWorkbenchStreamError(c *gin.Context, cursor int64, err error, version int) {
	code := "stream_error"
	if errors.Is(err, agentruntime.ErrCursorExpired) {
		code = "cursor_expired"
	}
	if version >= 2 {
		// v2: explicit control frame without a business id; never advances the cursor
		_ = writeWorkbenchControlSSE(c.Writer, code, err.Error())
		flushWorkbenchWriter(c)
		return
	}
	seq := cursor
	if seq < 1 {
		seq = 1
	}
	payload, marshalErr := json.Marshal(map[string]string{"code": code, "message": err.Error()})
	if marshalErr == nil {
		_ = writeWorkbenchSSE(c.Writer, seq, "error", payload)
		flushWorkbenchWriter(c)
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

// writeWorkbenchSSEV2 is the product v2 byte contract: the data line carries
// the full ExecutionEvent envelope (schema_version/run_id/attempt_id/seq/type/
// occurred_at/payload) so native parsers never reconstruct lost semantics.
// id stays the business seq and event stays the business type.
func writeWorkbenchSSEV2(w io.Writer, event workbench.ExecutionEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	envelope, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Seq, event.Type, envelope); err != nil {
		return err
	}
	return nil
}

// writeWorkbenchControlSSE writes an explicit control frame. Control frames
// carry no business id and must never advance the business cursor; clients
// classify them separately from business events.
func writeWorkbenchControlSSE(w io.Writer, code, message string) error {
	if strings.TrimSpace(code) == "" || strings.ContainsAny(code, "\r\n") || strings.ContainsAny(message, "\r\n") {
		return errors.New("invalid_control")
	}
	payload, err := json.Marshal(map[string]string{"code": code, "message": message})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: control\ndata: %s\n\n", payload)
	return err
}
