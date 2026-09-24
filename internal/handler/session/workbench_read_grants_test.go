package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeOwnedRunReader struct {
	run agentruntime.Run
	err error
}

func (f *fakeOwnedRunReader) GetOwnedRun(context.Context, uint64, string, string) (agentruntime.Run, error) {
	return f.run, f.err
}

type fakeGrantedRunReader struct {
	run agentruntime.Run
	err error
}

func (f *fakeGrantedRunReader) GetRunForGrantedReader(context.Context, uint64, string, string) (agentruntime.Run, error) {
	return f.run, f.err
}

// grantedSnapshots is the minimal snapshot reader for the seam tests: it
// returns one fixed execution snapshot for any key.
type grantedSnapshots struct{}

func (grantedSnapshots) ReadRunSnapshot(ctx context.Context, key agentruntime.RunKey) (workbench.ExecutionSnapshot, error) {
	return workbench.ExecutionSnapshot{Execution: workbench.ExecutionDTO{RunID: key.RunID}}, nil
}

func (grantedSnapshots) ReadRunEvents(ctx context.Context, key agentruntime.RunKey, cursor int64, limit int) ([]workbench.ExecutionEvent, int64, error) {
	return nil, 0, nil
}

func grantedReadContext(t *testing.T, userID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/executions/r1/snapshot", nil).WithContext(ctx)
	c.Params = gin.Params{{Key: "run_id", Value: "r1"}}
	return c, recorder
}

func TestGetWorkbenchSnapshotFallsBackToGrantedReader(t *testing.T) {
	run := agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, SessionID: "s1", UserID: "u1"}

	// Owner miss + grantee hit: the snapshot is served to the grantee.
	owned := &fakeOwnedRunReader{err: agentruntime.ErrNotFound}
	granted := &fakeGrantedRunReader{run: run}
	h := NewWorkbenchReadHandler(owned, grantedSnapshots{}).WithGrantedRuns(granted)
	c, recorder := grantedReadContext(t, "member-2")
	h.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	// No granted reader wired: fail closed, exactly today's behavior.
	hClosed := NewWorkbenchReadHandler(owned, grantedSnapshots{})
	c, recorder = grantedReadContext(t, "member-2")
	hClosed.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)

	// Granted reader wired but the reader holds no grant: still 404.
	hMiss := NewWorkbenchReadHandler(owned, grantedSnapshots{}).WithGrantedRuns(&fakeGrantedRunReader{err: agentruntime.ErrNotFound})
	c, recorder = grantedReadContext(t, "member-4")
	hMiss.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)

	// The owner keeps the direct path (no fallback involved).
	ownerOK := &fakeOwnedRunReader{run: run}
	hOwner := NewWorkbenchReadHandler(ownerOK, grantedSnapshots{}).WithGrantedRuns(&fakeGrantedRunReader{err: agentruntime.ErrNotFound})
	c, recorder = grantedReadContext(t, "u1")
	hOwner.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	// A repository error on the granted path is a 500, never a silent 404.
	hErr := NewWorkbenchReadHandler(owned, grantedSnapshots{}).WithGrantedRuns(&fakeGrantedRunReader{err: errors.New("db down")})
	c, recorder = grantedReadContext(t, "member-2")
	hErr.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
