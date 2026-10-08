package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/workbench"
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

// grantedGinKeysContext injects identity ONLY through gin keys (c.Set), the
// fallback surface the auth middleware writes in lockstep with the request
// context. The request context stays background so these cases pin the
// gin-keys resolution path the request-context tests cannot reach.
func grantedGinKeysContext(t *testing.T, userID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(types.TenantIDContextKey.String(), uint64(1))
	c.Set(types.UserIDContextKey.String(), userID)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/executions/r1/snapshot", nil)
	c.Params = gin.Params{{Key: "run_id", Value: "r1"}}
	return c, recorder
}

func TestWorkbenchReadResolvesIdentityFromGinKeys(t *testing.T) {
	run := agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, SessionID: "s1", UserID: "u1"}

	// resolveOwnedRun serves an owner whose identity lives only in gin keys.
	ownedOK := &fakeOwnedRunReader{run: run}
	c, recorder := grantedGinKeysContext(t, "u1")
	resolved, ok := resolveOwnedRun(c, ownedOK)
	require.True(t, ok)
	require.Equal(t, "r1", resolved.Key.RunID)

	// The granted fallback also resolves its reader through gin keys only.
	ownedMiss := &fakeOwnedRunReader{err: agentruntime.ErrNotFound}
	h := NewWorkbenchReadHandler(ownedMiss, grantedSnapshots{}).WithGrantedRuns(&fakeGrantedRunReader{run: run})
	c, recorder = grantedGinKeysContext(t, "member-2")
	h.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	// Missing identity on both surfaces stays a 401 on every surface.
	c, recorder = grantedGinKeysContext(t, "")
	h.GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
