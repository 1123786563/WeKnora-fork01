package session

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/workbench"
	"github.com/gin-gonic/gin"
)

// TerminalLogReader pages the read-only terminal projection of a run's
// durable event log. It is deliberately its own seam: the subsequence is
// sparse, so the contiguous ReadRunEvents contract does not apply. The
// production repository implements it, so the handler resolves the seam by
// assertion at request time — an assembly without it fails closed (501)
// instead of degrading to a fabricated empty log.
type TerminalLogReader interface {
	ReadRunTerminalEvents(ctx context.Context, key agentruntime.RunKey, after int64, limit int) ([]workbench.ExecutionEvent, error)
}

const (
	workbenchTerminalLogDefaultLimit = 200
	workbenchTerminalLogMaxLimit     = 256
)

// workbenchTerminalLine is the wire shape of one read-only terminal chunk.
type workbenchTerminalLine struct {
	Seq        int64  `json:"seq"`
	OccurredAt string `json:"occurred_at"`
	Stream     string `json:"stream"`
	Text       string `json:"text"`
}

// GetWorkbenchTerminalLog godoc
// @Summary      分页读取只读终端日志
// @Description  按 run 归属分页返回只读终端输出（事件类型 tool.terminal 的稀疏子序列，按 seq 升序）；无任何输入通道——移动面终端只读，交互式 PTY 仅存在于 Web 沙箱面
// @Tags         工作台
// @Produce      json
// @Param        run_id  path  string  true  "执行ID"
// @Param        after   query int     false "上一页最后一条 seq（默认 0）"
// @Param        limit   query int     false "页大小（默认 200，上限 256）"
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  errors.AppError
// @Failure      400  {object}  errors.AppError
// @Failure      404  {object}  errors.AppError
// @Failure      501  {object}  errors.AppError
// @Security     Bearer
// @Router       /workbench/executions/{run_id}/terminal-log [get]
func (h *WorkbenchReadHandler) GetWorkbenchTerminalLog(c *gin.Context) {
	// Read-only surface: the same readable-run predicate as snapshot/execution
	// — strict owner first, task-grant fallback when wired. The snapshot already
	// exposes full tool.terminal payloads, so owner-only here had no secrecy
	// value, only a capability gap for granted Viewers/Collaborators (B3-F63).
	run, ok := h.resolveReadableRun(c)
	if !ok {
		return
	}
	reader, supported := h.snapshots.(TerminalLogReader)
	if !supported || reader == nil {
		c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{
			"success": false,
			"code":    "terminal_log_unavailable",
			"error":   "terminal log projection not wired",
		})
		return
	}
	after := int64(0)
	if raw := strings.TrimSpace(c.Query("after")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		after = parsed
	}
	limit := workbenchTerminalLogDefaultLimit
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	if limit > workbenchTerminalLogMaxLimit {
		limit = workbenchTerminalLogMaxLimit
	}
	events, err := reader.ReadRunTerminalEvents(c.Request.Context(), run.Key, after, limit)
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	lines := make([]workbenchTerminalLine, 0, len(events))
	next := after
	for _, event := range events {
		var payload struct {
			Stream string `json:"stream"`
			Text   string `json:"text"`
		}
		jsonErr := json.Unmarshal(event.Payload, &payload)
		if jsonErr != nil || (payload.Text == "" && payload.Stream == "") {
			// A payload that is not a terminal-chunk object is never
			// fabricated into output, but the cursor still advances past it
			// so paging cannot stall on the same malformed seq.
			next = event.Seq
			continue
		}
		if payload.Stream != "stdout" && payload.Stream != "stderr" {
			payload.Stream = "stdout"
		}
		lines = append(lines, workbenchTerminalLine{
			Seq:        event.Seq,
			OccurredAt: event.OccurredAt,
			Stream:     payload.Stream,
			Text:       payload.Text,
		})
		next = event.Seq
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"lines": lines, "next_cursor": next}})
}
