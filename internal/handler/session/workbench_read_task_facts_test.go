package session

import (
	"context"
	"net/http"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/workbench"
	"github.com/stretchr/testify/require"
)

type stubTaskFactsReader struct {
	facts        repository.WorkbenchTaskFacts
	err          error
	calls        int
	seenOwnerIDs []string
}

// ReadTaskFactsForRun records the ownerID argument: the owner-field mismatch
// (lease owner vs business owner) is invisible to a stub that ignores it.
func (s *stubTaskFactsReader) ReadTaskFactsForRun(_ context.Context, _ uint64, ownerID, _ string) (repository.WorkbenchTaskFacts, error) {
	s.calls++
	s.seenOwnerIDs = append(s.seenOwnerIDs, ownerID)
	return s.facts, s.err
}

func snapshotHandlerWithFacts(facts OwnedTaskFactsReader) *WorkbenchReadHandler {
	// UserID is the business owner (agent_runs.owner_id, the HTTP caller);
	// Owner is the lease owner (agent_runs.lease_owner, the claiming worker id).
	// workbenchRunReaderStub.GetOwnedRun keeps its hardcoded "u1" ownership guard.
	runs := &workbenchRunReaderStub{run: agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, UserID: "u1", Owner: "worker-1", SessionID: "s1"}}
	snapshots := &workbenchSnapshotReaderStub{snapshot: workbench.ExecutionSnapshot{Execution: workbench.ExecutionDTO{SchemaVersion: 1, RunID: "r1", SessionID: "s1", Driver: "platform", RunStatus: "running", ExecutionStatus: "running", SettlementStatus: "pending", Capabilities: map[string]workbench.Capability{}}}}
	h := NewWorkbenchReadHandler(runs, snapshots)
	if facts != nil {
		h = h.WithTaskFacts(facts)
	}
	return h
}

func TestGetWorkbenchSnapshotCarriesTaskFacts(t *testing.T) {
	facts := &stubTaskFactsReader{facts: repository.WorkbenchTaskFacts{TaskID: "s1", Title: "session-1", Attention: "required", ArchivedAt: "2026-09-23T00:00:00Z"}}
	c, w := workbenchRequest(t, "u1")
	snapshotHandlerWithFacts(facts).GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"task":{"task_id":"s1"`)
	require.Contains(t, w.Body.String(), `"title":"session-1"`)
	require.Contains(t, w.Body.String(), `"attention":"required"`)
	require.Contains(t, w.Body.String(), `"archived_at":"2026-09-23T00:00:00Z"`)
	require.Equal(t, 1, facts.calls)
}

func TestGetWorkbenchSnapshotOmitsTaskSectionWithoutFactsReader(t *testing.T) {
	c, w := workbenchRequest(t, "u1")
	snapshotHandlerWithFacts(nil).GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), `"task"`)
}

func TestGetWorkbenchSnapshotSurfacesFactsReadFailure(t *testing.T) {
	facts := &stubTaskFactsReader{err: context.DeadlineExceeded}
	c, w := workbenchRequest(t, "u1")
	snapshotHandlerWithFacts(facts).GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusInternalServerError, w.Code, "enrichment failure must fail honestly, not silently drop the task section")
}

func TestGetWorkbenchSnapshotFactsReadIsOwnerScoped(t *testing.T) {
	facts := &stubTaskFactsReader{facts: repository.WorkbenchTaskFacts{TaskID: "s1", Attention: "none"}}
	c, w := workbenchRequest(t, "other-user")
	snapshotHandlerWithFacts(facts).GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Zero(t, facts.calls, "ownership is checked before any facts read")
}

func TestGetWorkbenchSnapshotFactsUseBusinessOwner(t *testing.T) {
	facts := &stubTaskFactsReader{facts: repository.WorkbenchTaskFacts{TaskID: "s1", Attention: "none"}}
	c, w := workbenchRequest(t, "u1")
	snapshotHandlerWithFacts(facts).GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"u1"}, facts.seenOwnerIDs, "task facts must be queried by the business owner (agent_runs.owner_id), not the lease owner (agent_runs.lease_owner)")
}
