package session

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

type AgentRunDecisionRequest struct {
	PendingID        string         `json:"pending_id" binding:"required"`
	DecisionID       string         `json:"decision_id" binding:"required"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	ExpectedRevision int64          `json:"expected_revision"`
	Action           string         `json:"action" binding:"required"`
	ArgsHash         string         `json:"args_hash,omitempty"`
	ResourceRef      string         `json:"resource_ref,omitempty"`
	Reason           string         `json:"reason" binding:"required"`
	Result           map[string]any `json:"result,omitempty"`
}
type agentRunHandlerService interface {
	Get(context.Context, agentruntime.RunKey) (agentruntime.Run, error)
}

func (h *Handler) SetAgentRunService(s *service.AgentRunService) { h.agentRunService = s }
func (h *Handler) runService() *service.AgentRunService {
	if h.agentRunService != nil {
		return h.agentRunService
	}
	return service.RegisteredAgentRunService()
}

func (h *Handler) ownedRun(c *gin.Context) (context.Context, agentruntime.RunKey, agentruntime.Run, bool) {
	ctx := c.Request.Context()
	tenant, ok := types.TenantIDFromContext(ctx)
	if !ok || tenant == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return ctx, agentruntime.RunKey{}, agentruntime.Run{}, false
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(c.Param("id"))
	}
	runID := strings.TrimSpace(c.Param("run_id"))
	if sessionID == "" || runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id and run_id are required"})
		return ctx, agentruntime.RunKey{}, agentruntime.Run{}, false
	}
	if h.sessionService == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return ctx, agentruntime.RunKey{}, agentruntime.Run{}, false
	}
	{
		sess, err := h.sessionService.GetOwnedSession(ctx, sessionID)
		if err != nil || sess == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
			return ctx, agentruntime.RunKey{}, agentruntime.Run{}, false
		}
	}
	principal, principalOK := types.PrincipalFromContext(ctx)
	if !principalOK || principal.StorageID() == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return ctx, agentruntime.RunKey{}, agentruntime.Run{}, false
	}
	runs := h.runService()
	if runs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "durable agent runs are unavailable"})
		return ctx, agentruntime.RunKey{}, agentruntime.Run{}, false
	}
	key := agentruntime.RunKey{TenantID: tenant, RunID: runID}
	run, err := runs.Get(ctx, key)
	if err != nil || run.SessionID != sessionID {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return ctx, key, agentruntime.Run{}, false
	}
	ownerID := types.SessionOwnerIDFromContext(ctx)
	if (!principalOK || principal.StorageID() != run.UserID) && ownerID != run.UserID {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return ctx, key, agentruntime.Run{}, false
	}
	return ctx, key, run, true
}

func (h *Handler) GetAgentRun(c *gin.Context) {
	ctx, key, run, ok := h.ownedRun(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": runViewWithSeq(run, h.latestRunSeq(ctx, key))})
}
func (h *Handler) latestRunSeq(ctx context.Context, key agentruntime.RunKey) int64 {
	runs := h.runService()
	if runs == nil {
		return 0
	}
	es, ok := runs.Store().(interface {
		ReadEvents(context.Context, agentruntime.RunKey, int64, int) ([]agentruntime.RunEvent, error)
	})
	if !ok {
		return 0
	}
	events, err := es.ReadEvents(ctx, key, 0, 100000)
	if err != nil {
		return 0
	}
	var seq int64
	for _, e := range events {
		if e.Seq > seq {
			seq = e.Seq
		}
	}
	return seq
}
func runView(r agentruntime.Run) gin.H {
	v := gin.H{"run_id": r.Key.RunID, "session_id": r.SessionID, "status": r.Status, "wait_reason": r.WaitReason, "revision": r.Revision, "epoch": r.Epoch, "seq": int64(0), "capabilities": gin.H{"engine_type": "trpc", "durable_recovery": true}}
	if r.Status == "waiting_user" && r.WaitReason != "" {
		v["pending_id"] = r.WaitReason
	}
	return v
}
func runViewWithSeq(r agentruntime.Run, seq int64) gin.H { v := runView(r); v["seq"] = seq; return v }

func (h *Handler) GetAgentRunEvents(c *gin.Context) {
	ctx, key, run, ok := h.ownedRun(c)
	if !ok {
		return
	}
	es, ok := h.runService().Store().(interface {
		ReadEvents(context.Context, agentruntime.RunKey, int64, int) ([]agentruntime.RunEvent, error)
	})
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "run events are unavailable"})
		return
	}
	after := int64(0)
	raw := c.GetHeader("Last-Event-ID")
	if raw == "" {
		raw = c.Query("after")
	}
	if raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Last-Event-ID"})
			return
		}
		after = v
	}
	events, err := es.ReadEvents(ctx, key, after, 256)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, agentruntime.ErrNotFound) {
			status = http.StatusNotFound
		}
		if errors.Is(err, agentruntime.ErrCursorExpired) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "cursor_expired"})
			return
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Status(http.StatusOK)
	seen := map[int64]bool{}
	emit := func(page []agentruntime.RunEvent) int64 {
		latest := after
		for _, e := range page {
			if e.Seq <= after || seen[e.Seq] {
				continue
			}
			seen[e.Seq] = true
			if e.Seq > latest {
				latest = e.Seq
			}
			c.SSEvent(strconv.FormatInt(e.Seq, 10), e)
		}
		c.Writer.Flush()
		return latest
	}
	after = emit(events)
	if len(events) == 0 && run.Status != "succeeded" && run.Status != "failed" && run.Status != "canceled" {
		c.SSEvent("run", runViewWithSeq(run, after))
		c.Writer.Flush()
	}
	if c.Query("once") == "1" {
		return
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	keep := time.NewTicker(15 * time.Second)
	defer keep.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			page, readErr := es.ReadEvents(ctx, key, after, 256)
			if readErr != nil {
				code := "event_read_error"
				if errors.Is(readErr, agentruntime.ErrCursorExpired) {
					code = "cursor_expired"
				}
				c.SSEvent("error", gin.H{"code": code, "message": readErr.Error(), "seq": after})
				c.Writer.Flush()
				return
			}
			after = emit(page)
			current, getErr := h.runService().Get(ctx, key)
			if getErr == nil && (current.Status == "succeeded" || current.Status == "failed" || current.Status == "canceled") {
				return
			}
		case <-keep.C:
			c.SSEvent("keepalive", gin.H{"seq": after})
			c.Writer.Flush()
		}
	}
}

func (h *Handler) PostAgentRunDecision(c *gin.Context) {
	ctx, key, _, ok := h.ownedRun(c)
	if !ok {
		return
	}
	var req AgentRunDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	raw := json.RawMessage(nil)
	if req.Result != nil {
		var err error
		raw, err = json.Marshal(req.Result)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid result"})
			return
		}
	}
	decision := agentruntime.Decision{PendingID: req.PendingID, DecisionID: req.DecisionID, ToolCallID: req.ToolCallID, ExpectedRevision: req.ExpectedRevision, Action: req.Action, ArgsHash: req.ArgsHash, ResourceRef: req.ResourceRef, Reason: req.Reason, Result: raw}
	if err := service.ValidateDecision(decision); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	run, err := h.runService().Resolve(ctx, key, decision)
	if err != nil {
		status := http.StatusConflict
		if errors.Is(err, agentruntime.ErrNotFound) {
			status = http.StatusNotFound
		}
		if strings.Contains(err.Error(), "approval policy is unavailable") {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": runView(run)})
}
func (h *Handler) CancelAgentRun(c *gin.Context) {
	ctx, key, _, ok := h.ownedRun(c)
	if !ok {
		return
	}
	if err := h.runService().Cancel(ctx, key); err != nil {
		status := http.StatusConflict
		if errors.Is(err, agentruntime.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
