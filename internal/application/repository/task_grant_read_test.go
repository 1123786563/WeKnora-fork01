package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// taskGrantAdmission returns a minimal admitted run for task s1 owned by u1.
func taskGrantAdmission() agentruntime.Admission {
	return agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, SessionID: "s1", UserID: "u1",
		RequestID: "q1", AssistantMessageID: "a1", RequestHash: "hash-1",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	}
}

func TestGetRunForGrantedReaderRespectsGrantsAndMembership(t *testing.T) {
	db := openTaskGrantDB(t)
	grants := repository.NewTaskGrantStore(db)
	runs := repository.NewAgentRunStore(db)
	ctx := context.Background()
	_, err := runs.Admit(ctx, taskGrantAdmission())
	require.NoError(t, err)

	_, err = grants.UpsertGrant(ctx, 1, "s1", "u2", types.TaskGrantRoleViewer, "u1")
	require.NoError(t, err)
	_, err = grants.UpsertGrant(ctx, 1, "s1", "u5", types.TaskGrantRoleViewer, "u1")
	require.NoError(t, err, "u5 holds a grant but is suspended — row kept to prove the read converges")

	// A grant holder reads the run (viewer and collaborator alike).
	run, err := runs.GetRunForGrantedReader(ctx, 1, "u2", "r1")
	require.NoError(t, err)
	require.Equal(t, "r1", run.Key.RunID)
	require.Equal(t, "s1", run.SessionID)

	// The owner also resolves through the granted path (same predicate shape).
	_, err = runs.GetRunForGrantedReader(ctx, 1, "u1", "r1")
	require.ErrorIs(t, err, agentruntime.ErrNotFound, "the owner holds no grant row; owner reads stay on GetOwnedRun")

	// A bystander without a grant cannot.
	_, err = runs.GetRunForGrantedReader(ctx, 1, "u4", "r1")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)

	// A suspended member's grant stops resolving immediately.
	_, err = runs.GetRunForGrantedReader(ctx, 1, "u5", "r1")
	require.ErrorIs(t, err, agentruntime.ErrNotFound,
		"deactivated membership kills the granted read without touching the grant row")

	// Cross-tenant reader cannot.
	_, err = runs.GetRunForGrantedReader(ctx, 2, "u2", "r1")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)

	// Unknown run id is a plain miss.
	_, err = runs.GetRunForGrantedReader(ctx, 1, "u2", "missing")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)

	// Revoking the grant closes the read on the next request.
	require.NoError(t, grants.DeleteGrant(ctx, 1, "s1", "u2"))
	_, err = runs.GetRunForGrantedReader(ctx, 1, "u2", "r1")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
}
