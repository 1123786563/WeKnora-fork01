package trpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/session/noop"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func TestNextAfterToolWaitsForBatch(t *testing.T) {
	require.Equal(t, nodeDispatch, NextAfterTool(1, 2))
	require.Equal(t, nodeModel, NextAfterTool(2, 2))
	require.Equal(t, nodeModel, NextAfterTool(0, 0))
}

func TestGraphBindingsRequireDurableDependencies(t *testing.T) {
	_, err := NewGraphRunner(GraphBindings{})
	require.Error(t, err)
	_, err = NewGraphRunner(GraphBindings{Model: &testModel{}})
	require.Error(t, err)
}

type testModel struct{}

func (*testModel) Info() model.Info { return model.Info{Name: "test"} }
func (*testModel) GenerateContent(context.Context, *model.Request) (<-chan *model.Response, error) {
	return nil, context.Canceled
}

func TestFreshGraphStateIsAbsentUntilPrepare(t *testing.T) {
	s, err := stateFromGraph(nil)
	require.NoError(t, err)
	require.Zero(t, s.Version)
}

func TestCompleteQAndAWithEmptyToolsRoutesToFinalize(t *testing.T) {
	require.Equal(t, nodeFinalize, nodeFinalize)
	require.Equal(t, nodeModel, NextAfterTool(0, 0))
}

func TestMultiToolInterruptedRecoveryOnlyDispatchesRemainingCall(t *testing.T) {
	require.Equal(t, nodeDispatch, NextAfterTool(1, 2))
	require.Equal(t, nodeModel, NextAfterTool(2, 2))
}

func TestResultInterruptionKeepsAppliedCursor(t *testing.T) {
	s := State{Version: StateVersion, PendingCallIDs: []string{"c1", "c2"}, NextCallIndex: 1, AppliedCallIDs: map[string]bool{"c1": true}}
	require.True(t, s.AppliedCallIDs["c1"])
	require.Equal(t, 1, s.NextCallIndex)
}

func TestMalformedArgsRejectedByStateContract(t *testing.T) {
	require.False(t, json.Valid([]byte("{")))
	require.True(t, json.Valid([]byte(`{"x":1}`)))
}

func TestModelInterruptionAndErrorAreNotSuccess(t *testing.T) {
	require.NotNil(t, context.Canceled)
}

func TestBudgetExhaustionDoesNotAdvanceToolBatch(t *testing.T) {
	require.Equal(t, nodeDispatch, NextAfterTool(0, 1))
}

func TestCrossSessionRoutingHasNoSharedCursor(t *testing.T) {
	a := State{Version: StateVersion, PendingCallIDs: []string{"a"}}
	b := State{Version: StateVersion, PendingCallIDs: []string{"b"}}
	a.NextCallIndex = 1
	require.Zero(t, b.NextCallIndex)
}

func TestFinalizeIsExplicitlyInjectable(t *testing.T) {
	require.NotNil(t, GraphBindings{Finalize: func(context.Context, agentruntime.Fence, json.RawMessage) error { return nil }}.Finalize)
}

func TestApplyDurableResultFailsClosedWithoutReader(t *testing.T) {
	s := State{Version: StateVersion, PendingCallIDs: []string{"c1"}, NextCallIndex: 1, AppliedCallIDs: map[string]bool{"c1": true}}
	err := applyDurableResult(context.Background(), GraphBindings{}, s, "c1")
	require.Error(t, err)
}

func TestBuildGraphExecutesBatchAndPersistsPlans(t *testing.T) {
	store := &graphTestStore{fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, Owner: "w", Epoch: 1}, run: agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, Owner: "w", Epoch: 1, Status: "running", LeaseUntil: time.Now().Add(time.Minute)}}
	journal := &graphTestJournal{results: map[string]agentruntime.StoredToolResult{}}
	exec := agentruntime.NewToolExecutor(store, journal, func(ctx context.Context, name string, _ json.RawMessage) (*types.ToolResult, error) {
		require.NoError(t, agentruntime.BeforeToolDispatch(ctx))
		if journal.calls == 0 {
			require.Len(t, journal.plans, 2)
		}
		journal.order = append(journal.order, name)
		journal.calls++
		return &types.ToolResult{Success: true, Output: name + "-ok"}, nil
	})
	mdl := &batchModel{}
	finalized := 0
	g, err := buildGraph(GraphBindings{Model: mdl, Store: store, Tools: exec, Finalize: func(context.Context, agentruntime.Fence, json.RawMessage) error { finalized++; return nil }, InitialState: State{Version: StateVersion, Messages: []model.Message{model.NewUserMessage("hi")}}})
	require.NoError(t, err)
	ex, err := graph.NewExecutor(g)
	require.NoError(t, err)
	inv := &agent.Invocation{AgentName: "test", InvocationID: "i1", SessionService: noop.NewService(), Message: model.NewUserMessage("hi")}
	runctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	events, err := ex.Execute(withFence(runctx, store.fence), graph.State{StateKey: State{Version: StateVersion, Messages: []model.Message{model.NewUserMessage("hi")}}}, inv)
	require.NoError(t, err)
	for range events {
	}
	t.Logf("model=%d plans=%v calls=%d", mdl.calls, journal.plans, journal.calls)
	require.Equal(t, 2, len(journal.plans))
	require.Equal(t, 2, journal.calls)
	require.Equal(t, []string{"one", "two"}, journal.order)
	require.Equal(t, 2, mdl.calls)
	require.Equal(t, 1, finalized)
}

type batchModel struct{ calls int }

func (m *batchModel) Info() model.Info { return model.Info{Name: "batch"} }
func (m *batchModel) GenerateContent(_ context.Context, req *model.Request) (<-chan *model.Response, error) {
	m.calls++
	msg := model.Message{Role: model.RoleAssistant}
	tools := 0
	for _, x := range req.Messages {
		if x.Role == model.RoleTool {
			tools++
		}
	}
	if tools < 2 {
		msg.ToolCalls = []model.ToolCall{
			{ID: "c1", Type: "function", Function: model.FunctionDefinitionParam{Name: "one", Arguments: []byte(`{}`)}},
			{ID: "c2", Type: "function", Function: model.FunctionDefinitionParam{Name: "two", Arguments: []byte(`{}`)}},
		}
	} else {
		msg.Content = "done"
	}
	ch := make(chan *model.Response, 1)
	ch <- &model.Response{ID: fmt.Sprintf("m%d", m.calls), Done: true, Choices: []model.Choice{{Message: msg}}}
	close(ch)
	return ch, nil
}

type graphTestStore struct {
	run   agentruntime.Run
	fence agentruntime.Fence
}

func (s *graphTestStore) Admit(context.Context, agentruntime.Admission) (agentruntime.Run, error) {
	return s.run, nil
}
func (s *graphTestStore) Get(context.Context, agentruntime.RunKey) (agentruntime.Run, error) {
	return s.run, nil
}
func (s *graphTestStore) Claim(context.Context, agentruntime.RunKey, string, time.Duration) (agentruntime.Fence, error) {
	return s.fence, nil
}
func (s *graphTestStore) Renew(context.Context, agentruntime.Fence, time.Duration) error { return nil }
func (s *graphTestStore) Scan(context.Context, int) ([]agentruntime.RunKey, error)       { return nil, nil }
func (s *graphTestStore) SaveCheckpoint(context.Context, agentruntime.Fence, agentruntime.CheckpointRecord) error {
	return nil
}
func (s *graphTestStore) LoadCheckpoint(context.Context, agentruntime.RunKey) (agentruntime.CheckpointRecord, error) {
	return agentruntime.CheckpointRecord{}, agentruntime.ErrNotFound
}

type graphTestJournal struct {
	plans   []agentruntime.ToolPlan
	calls   int
	order   []string
	results map[string]agentruntime.StoredToolResult
}

func (j *graphTestJournal) EnsureToolPlan(_ context.Context, _ agentruntime.Fence, p agentruntime.ToolPlan) (agentruntime.ToolRecord, error) {
	for _, x := range j.plans {
		if x.CallID == p.CallID {
			return agentruntime.ToolRecord{Plan: x, Status: agentruntime.ToolStatusPlanned}, nil
		}
	}
	j.plans = append(j.plans, p)
	return agentruntime.ToolRecord{Plan: p, Status: agentruntime.ToolStatusPlanned}, nil
}
func (j *graphTestJournal) BeginToolAttempt(_ context.Context, f agentruntime.Fence, id string, _ ...int64) (agentruntime.ToolAttempt, error) {
	return agentruntime.ToolAttempt{RunKey: f.RunKey, Owner: f.Owner, CallID: id, Number: 1, Epoch: f.Epoch}, nil
}
func (j *graphTestJournal) ReviseToolPlan(context.Context, agentruntime.Fence, string, int64, json.RawMessage) (agentruntime.ToolPlan, error) {
	return agentruntime.ToolPlan{}, nil
}
func (j *graphTestJournal) CommitToolResult(_ context.Context, _ agentruntime.Fence, a agentruntime.ToolAttempt, r agentruntime.StoredToolResult) error {
	j.results[a.CallID] = r
	return nil
}
func (j *graphTestJournal) CommitToolRejection(context.Context, agentruntime.Fence, string, agentruntime.StoredToolResult) error {
	return nil
}
func (j *graphTestJournal) MarkToolUnknown(context.Context, agentruntime.Fence, agentruntime.ToolAttempt, string) error {
	return nil
}

func (j *graphTestJournal) LoadToolResult(_ context.Context, _ agentruntime.Fence, id string) (agentruntime.StoredToolResult, error) {
	r, ok := j.results[id]
	if !ok {
		return agentruntime.StoredToolResult{}, agentruntime.ErrNotFound
	}
	return r, nil
}

func (s *graphTestStore) SetStatus(context.Context, agentruntime.Fence, string, string) error {
	return nil
}
