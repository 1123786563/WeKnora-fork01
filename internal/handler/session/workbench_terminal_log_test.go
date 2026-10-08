package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type terminalReaderStub struct {
	terminal  []workbench.ExecutionEvent
	calls     int
	lastAfter int64
	lastLimit int
}

func (s *terminalReaderStub) ReadRunSnapshot(context.Context, agentruntime.RunKey) (workbench.ExecutionSnapshot, error) {
	return workbench.ExecutionSnapshot{}, nil
}

func (s *terminalReaderStub) ReadRunEvents(context.Context, agentruntime.RunKey, int64, int) ([]workbench.ExecutionEvent, int64, error) {
	return nil, 0, nil
}

func (s *terminalReaderStub) ReadRunTerminalEvents(_ context.Context, _ agentruntime.RunKey, after int64, limit int) ([]workbench.ExecutionEvent, error) {
	s.calls++
	s.lastAfter = after
	s.lastLimit = limit
	return s.terminal, nil
}

func terminalEvents() []workbench.ExecutionEvent {
	return []workbench.ExecutionEvent{
		{SchemaVersion: 1, RunID: "run-1", AttemptID: "a1", Seq: 3, Type: "tool.terminal", OccurredAt: "2026-09-24T00:00:03Z", Payload: json.RawMessage(`{"stream":"stdout","text":"$ cargo test\n"}`)},
		{SchemaVersion: 1, RunID: "run-1", AttemptID: "a1", Seq: 7, Type: "tool.terminal", OccurredAt: "2026-09-24T00:00:07Z", Payload: json.RawMessage(`{"stream":"stderr","text":"warning: unused import\n"}`)},
	}
}

func terminalLogContext(query string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/executions/run-1/terminal-log"+query, nil)
	ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	c.Request = c.Request.WithContext(ctx)
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	return c, recorder
}

func TestGetWorkbenchTerminalLogProjectsReadOnlyLines(t *testing.T) {
	reader := &terminalReaderStub{terminal: terminalEvents()}
	h := NewWorkbenchReadHandler(artifactRunStub(), reader)
	c, rec := terminalLogContext("")
	h.GetWorkbenchTerminalLog(c)

	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.EqualValues(t, 0, reader.lastAfter)
	require.Equal(t, 200, reader.lastLimit) // 默认页大小
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Lines []struct {
				Seq        int64  `json:"seq"`
				OccurredAt string `json:"occurred_at"`
				Stream     string `json:"stream"`
				Text       string `json:"text"`
			} `json:"lines"`
			NextCursor int64 `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Len(t, body.Data.Lines, 2)
	require.EqualValues(t, 3, body.Data.Lines[0].Seq)
	require.Equal(t, "stdout", body.Data.Lines[0].Stream)
	require.Equal(t, "$ cargo test\n", body.Data.Lines[0].Text)
	require.Equal(t, "stderr", body.Data.Lines[1].Stream)
	require.EqualValues(t, 7, body.Data.NextCursor)
}

func TestGetWorkbenchTerminalLogResumesAndClamps(t *testing.T) {
	reader := &terminalReaderStub{terminal: terminalEvents()[1:]}
	h := NewWorkbenchReadHandler(artifactRunStub(), reader)
	c, _ := terminalLogContext("?after=3&limit=9999")
	h.GetWorkbenchTerminalLog(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.EqualValues(t, 3, reader.lastAfter)
	require.Equal(t, 256, reader.lastLimit) // 超限钳到上限

	bad, _ := terminalLogContext("?after=-1")
	h.GetWorkbenchTerminalLog(bad)
	require.Equal(t, http.StatusBadRequest, bad.Writer.Status())

	badLimit, _ := terminalLogContext("?limit=0")
	h.GetWorkbenchTerminalLog(badLimit)
	require.Equal(t, http.StatusBadRequest, badLimit.Writer.Status())
}

func TestGetWorkbenchTerminalLogSkipsMalformedPayloadAndAdvancesCursor(t *testing.T) {
	events := append(terminalEvents(), workbench.ExecutionEvent{
		SchemaVersion: 1, RunID: "run-1", AttemptID: "a1", Seq: 9, Type: "tool.terminal",
		OccurredAt: "2026-09-24T00:00:09Z", Payload: json.RawMessage(`"not an object"`),
	})
	reader := &terminalReaderStub{terminal: events}
	h := NewWorkbenchReadHandler(artifactRunStub(), reader)
	c, rec := terminalLogContext("")
	h.GetWorkbenchTerminalLog(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	var body struct {
		Data struct {
			Lines      []json.RawMessage `json:"lines"`
			NextCursor int64             `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data.Lines, 2, "malformed payload must not be fabricated into output")
	require.EqualValues(t, 9, body.Data.NextCursor, "cursor must advance past the skipped chunk so paging cannot stall")
}

func TestGetWorkbenchTerminalLogIsOwnerScopedAndFailsClosed(t *testing.T) {
	reader := &terminalReaderStub{terminal: terminalEvents()}
	h := NewWorkbenchReadHandler(artifactRunStub(), reader)

	// 跨租户：run 读谓词先拒，终端读零调用。
	foreign, _ := terminalLogContext("")
	ctx := context.WithValue(foreign.Request.Context(), types.TenantIDContextKey, uint64(2))
	foreign.Request = foreign.Request.WithContext(ctx)
	h.GetWorkbenchTerminalLog(foreign)
	require.Equal(t, http.StatusNotFound, foreign.Writer.Status())
	require.Equal(t, 0, reader.calls)

	// snapshots 未实现终端 seam：诚实 501，不降级为空日志。
	h2 := NewWorkbenchReadHandler(artifactRunStub(), &workbenchSnapshotReaderStub{})
	c2, rec2 := terminalLogContext("")
	h2.GetWorkbenchTerminalLog(c2)
	require.Equal(t, http.StatusNotImplemented, c2.Writer.Status())
	require.Contains(t, rec2.Body.String(), "terminal_log_unavailable")
}

// terminalGrantedRunReaderStub satisfies GrantedRunReader: the task-grant
// fallback resolves the run for the given reader.
type terminalGrantedRunReaderStub struct {
	run agentruntime.Run
}

func (s *terminalGrantedRunReaderStub) GetRunForGrantedReader(_ context.Context, _ uint64, readerID, runID string) (agentruntime.Run, error) {
	if readerID == "viewer-1" && runID == "run-1" {
		return s.run, nil
	}
	return agentruntime.Run{}, agentruntime.ErrNotFound
}

func TestGetWorkbenchTerminalLogAdmitsGrantedReaders(t *testing.T) {
	reader := &terminalReaderStub{terminal: terminalEvents()}
	h := NewWorkbenchReadHandler(artifactRunStub(), reader).WithGrantedRuns(
		&terminalGrantedRunReaderStub{run: agentruntime.Run{
			Key:       agentruntime.RunKey{TenantID: 1, RunID: "run-1"},
			SessionID: "sess-1",
		}})

	// 调用方是被授权 viewer（owner 谓词 miss 后经 task-grant 回退命中）。
	c, _ := terminalLogContext("")
	viewerCtx := context.WithValue(c.Request.Context(), types.UserIDContextKey, "viewer-1")
	c.Request = c.Request.WithContext(viewerCtx)
	h.GetWorkbenchTerminalLog(c)
	require.Equal(t, http.StatusOK, c.Writer.Status(), "被授权 viewer 经 task-grant 回退必须能读 terminal-log（B3-F63）")
	require.Equal(t, 1, reader.calls)
}
