package session

// T18 (#137) — workbench execution-stream reconnect seam.
//
// The craft workbench's second transport is the workbench execution stream
// (GET /api/v1/workbench/executions/:run_id/events). Its reconnect contract
// mirrors the run-event stream: the client first reads the authoritative
// execution snapshot (its watermark is the resume cursor) and then resumes
// the stream with Last-Event-ID at that watermark, so a refresh never
// redelivers events the snapshot already covers.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// t18CursorRecorder wraps the shared stream stub and records every cursor
// the handler asks the store to resume after.
type t18CursorRecorder struct {
	inner   *workbenchStreamReaderStub
	cursors []int64
}

func (r *t18CursorRecorder) ReadRunEvents(ctx context.Context, key agentruntime.RunKey, cursor int64, limit int) ([]workbench.ExecutionEvent, int64, error) {
	r.cursors = append(r.cursors, cursor)
	return r.inner.ReadRunEvents(ctx, key, cursor, limit)
}

func (r *t18CursorRecorder) ReadRunSnapshot(ctx context.Context, key agentruntime.RunKey) (workbench.ExecutionSnapshot, error) {
	return r.inner.ReadRunSnapshot(ctx, key)
}

// TestWorkbenchStreamReconnectResumesAfterSnapshotWatermark pins the
// snapshot-first reconnect: the execution snapshot answers the authoritative
// watermark, and a stream opened with Last-Event-ID at that watermark asks
// the store to resume exactly there.
func TestWorkbenchStreamReconnectResumesAfterSnapshotWatermark(t *testing.T) {
	// The snapshot is the authority: seq 4 is already in it, so the client
	// reconnects with Last-Event-ID: 4.
	reader := &workbenchStreamReaderStub{
		pages:    [][]workbench.ExecutionEvent{{testWorkbenchEvent(5)}},
		statuses: []string{"succeeded"},
	}
	recorder := &t18CursorRecorder{inner: reader}
	runs := &workbenchRunReaderStub{run: agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}}}
	h := NewWorkbenchReadHandler(runs, recorder)

	// Step 1 of reconnect: load the authoritative execution snapshot.
	snapshotCtx, sw := workbenchSnapshotRequest(t)
	h.GetWorkbenchSnapshot(snapshotCtx)
	require.Equal(t, http.StatusOK, sw.Code)
	var snapshot struct {
		Data workbench.ExecutionSnapshot `json:"data"`
	}
	require.NoError(t, json.Unmarshal(sw.Body.Bytes(), &snapshot))
	require.Equal(t, "r1", snapshot.Data.Execution.RunID)
	require.Equal(t, "succeeded", snapshot.Data.Execution.RunStatus)

	// Step 2: resume the stream after the snapshot's watermark (simulated as
	// 4 here): only seq 5 is delivered and the store read starts at 4.
	c, w, cancel := workbenchStreamRequest(t)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/executions/r1/events", nil).
		WithContext(c.Request.Context())
	req.Header.Set("Last-Event-ID", "4")
	c.Request = req
	h.StreamWorkbenchEvents(c)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	require.Contains(t, body, "id: 5", "only the post-watermark event is delivered as a frame")
	require.NotContains(t, body, "id: 4", "the stream must not redeliver events the snapshot watermark covers")
	require.NotEmpty(t, recorder.cursors)
	require.EqualValues(t, 4, recorder.cursors[0], "the first store read must resume after the snapshot watermark")
}

func workbenchSnapshotRequest(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/executions/r1/snapshot", nil).WithContext(ctx)
	c.Params = gin.Params{{Key: "run_id", Value: "r1"}}
	return c, recorder
}
