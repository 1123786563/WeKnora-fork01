package session

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
