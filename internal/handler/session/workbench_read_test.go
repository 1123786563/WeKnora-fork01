package session

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/workbench"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWorkbenchSSEFrameHasID(t *testing.T) {
	var b bytes.Buffer
	err := writeWorkbenchSSE(&b, 7, "text.delta", json.RawMessage(`{"seq":7}`))
	require.NoError(t, err)
	require.Equal(t, "id: 7\nevent: text.delta\ndata: {\"seq\":7}\n\n", b.String())
}

type workbenchRunReaderStub struct {
	run   agentruntime.Run
	err   error
	calls int
}

func (s *workbenchRunReaderStub) GetOwnedRun(_ context.Context, tenantID uint64, ownerID, runID string) (agentruntime.Run, error) {
	s.calls++
	if tenantID != 1 || ownerID != "u1" || runID != s.run.Key.RunID {
		return agentruntime.Run{}, agentruntime.ErrNotFound
	}
	return s.run, s.err
}

type workbenchSnapshotReaderStub struct {
	snapshotCalls int
	snapshot      workbench.ExecutionSnapshot
}

type workbenchStreamReaderStub struct {
	pages    [][]workbench.ExecutionEvent
	statuses []string
	reads    int
}

func (s *workbenchStreamReaderStub) ReadRunEvents(context.Context, agentruntime.RunKey, int64, int) ([]workbench.ExecutionEvent, int64, error) {
	if s.reads >= len(s.pages) {
		return nil, 0, nil
	}
	page := s.pages[s.reads]
	s.reads++
	return page, int64(len(page)), nil
}

func (s *workbenchStreamReaderStub) ReadRunSnapshot(context.Context, agentruntime.RunKey) (workbench.ExecutionSnapshot, error) {
	index := s.reads
	if index >= len(s.statuses) {
		index = len(s.statuses) - 1
	}
	status := "running"
	if index >= 0 && len(s.statuses) > 0 {
		status = s.statuses[index]
	}
	return workbench.ExecutionSnapshot{Execution: workbench.ExecutionDTO{SchemaVersion: 1, RunID: "r1", SessionID: "s1", Driver: "platform", RunStatus: status, ExecutionStatus: status, SettlementStatus: "pending", Capabilities: map[string]workbench.Capability{}}}, nil
}

func (s *workbenchSnapshotReaderStub) ReadRunSnapshot(context.Context, agentruntime.RunKey) (workbench.ExecutionSnapshot, error) {
	s.snapshotCalls++
	return s.snapshot, nil
}

func (s *workbenchSnapshotReaderStub) ReadRunEvents(context.Context, agentruntime.RunKey, int64, int) ([]workbench.ExecutionEvent, int64, error) {
	return nil, 0, nil
}

func workbenchRequest(t *testing.T, owner string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/executions/r1", nil)
	c.Params = gin.Params{{Key: "run_id", Value: "r1"}}
	ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, owner)
	c.Request = c.Request.WithContext(ctx)
	return c, recorder
}

func TestWorkbenchReadChecksOwnerBeforeSnapshot(t *testing.T) {
	runs := &workbenchRunReaderStub{run: agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}}}
	snapshots := &workbenchSnapshotReaderStub{}
	h := NewWorkbenchReadHandler(runs, snapshots)
	c, w := workbenchRequest(t, "other-user")
	h.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Equal(t, 1, runs.calls)
	require.Zero(t, snapshots.snapshotCalls)
}

func TestWorkbenchReadReturnsExecutionEnvelope(t *testing.T) {
	runs := &workbenchRunReaderStub{run: agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}}}
	snapshots := &workbenchSnapshotReaderStub{snapshot: workbench.ExecutionSnapshot{Execution: workbench.ExecutionDTO{SchemaVersion: 1, RunID: "r1", SessionID: "s1", Driver: "platform", RunStatus: "running", ExecutionStatus: "running", SettlementStatus: "pending", Capabilities: map[string]workbench.Capability{}}}}
	h := NewWorkbenchReadHandler(runs, snapshots)
	c, w := workbenchRequest(t, "u1")
	h.GetWorkbenchExecution(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"run_id":"r1"`)
	require.Equal(t, 1, snapshots.snapshotCalls)
}

func workbenchStreamRequest(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, context.CancelFunc) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/executions/r1/events", nil).WithContext(ctx)
	c.Params = gin.Params{{Key: "run_id", Value: "r1"}}
	return c, recorder, cancel
}

func testWorkbenchEvent(seq int64) workbench.ExecutionEvent {
	return workbench.ExecutionEvent{SchemaVersion: 1, RunID: "r1", AttemptID: "a1", Seq: seq, Type: "text.delta", OccurredAt: "2026-09-16T00:00:00Z", Payload: json.RawMessage(`{"seq":1}`)}
}

func TestStreamWorkbenchDrainsMoreThanOnePage(t *testing.T) {
	first := make([]workbench.ExecutionEvent, 256)
	for i := range first {
		first[i] = testWorkbenchEvent(int64(i + 1))
	}
	second := []workbench.ExecutionEvent{testWorkbenchEvent(257)}
	reader := &workbenchStreamReaderStub{pages: [][]workbench.ExecutionEvent{first, second}, statuses: []string{"succeeded", "succeeded"}}
	runs := &workbenchRunReaderStub{run: agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}}}
	c, recorder, cancel := workbenchStreamRequest(t)
	defer cancel()
	NewWorkbenchReadHandler(runs, reader).StreamWorkbenchEvents(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 257, strings.Count(recorder.Body.String(), "event: text.delta"))
}

func TestStreamWorkbenchEmitsCursorErrorAfterStreamStarts(t *testing.T) {
	first := make([]workbench.ExecutionEvent, 256)
	for i := range first {
		first[i] = testWorkbenchEvent(int64(i + 1))
	}
	reader := &workbenchStreamReaderStub{pages: [][]workbench.ExecutionEvent{first}, statuses: []string{"succeeded"}}
	// The next page is a cursor failure, injected by a wrapper below.
	readerWithError := &streamErrorReader{inner: reader}
	runs := &workbenchRunReaderStub{run: agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}}}
	c, recorder, cancel := workbenchStreamRequest(t)
	defer cancel()
	NewWorkbenchReadHandler(runs, readerWithError).StreamWorkbenchEvents(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "event: error")
	require.Contains(t, recorder.Body.String(), "cursor_expired")
}

type streamErrorReader struct {
	inner *workbenchStreamReaderStub
}

func (s *streamErrorReader) ReadRunSnapshot(ctx context.Context, key agentruntime.RunKey) (workbench.ExecutionSnapshot, error) {
	return s.inner.ReadRunSnapshot(ctx, key)
}

func (s *streamErrorReader) ReadRunEvents(ctx context.Context, key agentruntime.RunKey, cursor int64, limit int) ([]workbench.ExecutionEvent, int64, error) {
	if s.inner.reads > 0 {
		return nil, cursor, agentruntime.ErrCursorExpired
	}
	return s.inner.ReadRunEvents(ctx, key, cursor, limit)
}

func TestWorkbenchSSEFrameRejectsUntrustedInput(t *testing.T) {
	for name, tc := range map[string]struct {
		seq  int64
		kind string
		raw  string
	}{
		"zero sequence": {seq: 0, kind: "text.delta", raw: `{}`},
		"newline event": {seq: 1, kind: "text.delta\nretry: 0", raw: `{}`},
		"invalid JSON":  {seq: 1, kind: "text.delta", raw: `{bad`},
	} {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			require.Error(t, writeWorkbenchSSE(&b, tc.seq, tc.kind, json.RawMessage(tc.raw)))
			require.Empty(t, b.String())
		})
	}
}
