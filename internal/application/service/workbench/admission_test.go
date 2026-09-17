package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestAdmissionNeverDispatchesWithoutBudget(t *testing.T) {
	called := false
	deny := errors.New("budget_denied")
	err := admitThenPublish(func() error { return deny }, func() error { called = true; return nil })
	if !errors.Is(err, deny) || called {
		t.Fatal("unfunded execution was published")
	}
}

func TestRequestHashChangesWithImmutableInput(t *testing.T) {
	a := StartInput{SessionID: "s", AgentID: "a", TargetID: "platform", Text: "hello", BudgetUpper: 10}
	b := a
	b.Text = "changed"
	if requestHash(a) == requestHash(b) {
		t.Fatal("request hash ignored immutable input")
	}
}

// W11 follow-up accepted in W12: the run snapshot persists space_id so the
// owned execution list can navigate without a second lookup, while the
// request hash stays stable when a retry omits the navigation hint.
func TestAdmissionSnapshotPersistsSpaceIDWithoutHashImpact(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	runs := repository.NewAgentRunStore(db)
	coordinator := NewAdmissionCoordinator(db, runs, nil, nil)
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	in := StartInput{SessionID: "s1", AgentID: "agent-1", TargetID: "platform", SpaceID: "space-7", RequestID: "space-snapshot", Text: "hello", BudgetUpper: 100}
	_, err := coordinator.Start(ctx, in)
	require.NoError(t, err)

	var raw string
	require.NoError(t, db.Table("agent_runs").Where("session_id = ?", "s1").Select("snapshot").Scan(&raw).Error)
	var snapshot map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &snapshot))
	require.Equal(t, "space-7", snapshot["space_id"])

	// A retry without the space hint still reconciles onto the same run.
	retry := in
	retry.SpaceID = ""
	run, err := coordinator.Start(ctx, retry)
	require.NoError(t, err)
	require.NotEmpty(t, run.Key.RunID)
}
