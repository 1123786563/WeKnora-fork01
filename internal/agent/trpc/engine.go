package trpc

import (
	"context"
	"encoding/json"
	"fmt"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/graphagent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/noop"
)

type GraphBindings struct {
	Model        model.Model
	Store        agentruntime.RunStore
	Tools        *agentruntime.ToolExecutor
	Finalize     func(context.Context, agentruntime.Fence, json.RawMessage) error
	InitialState State
}

type GraphRunner struct{ bindings GraphBindings }

func NewGraphRunner(b GraphBindings) (*GraphRunner, error) {
	if b.Model == nil {
		return nil, fmt.Errorf("model is required")
	}
	if b.Store == nil {
		return nil, fmt.Errorf("run store is required")
	}
	if b.InitialState.Version == 0 {
		b.InitialState.Version = StateVersion
	}
	b.InitialState = cloneState(b.InitialState)
	return &GraphRunner{bindings: b}, nil
}
func (r *GraphRunner) Run(ctx context.Context, fence agentruntime.Fence) error {
	if r == nil || fence.TenantID == 0 || fence.RunID == "" || fence.Owner == "" || fence.Epoch <= 0 {
		return fmt.Errorf("invalid graph run fence")
	}
	saver := NewCheckpointSaver(r.bindings.Store, fence)
	g, err := buildGraph(graphBindingsWithFence(r.bindings, fence))
	if err != nil {
		return err
	}
	ag, err := graphagent.New("weknora-trpc", g, graphagent.WithCheckpointSaver(saver))
	if err != nil {
		return err
	}
	rr := runner.NewRunner("weknora-trpc", ag, runner.WithSessionService(noop.NewService()))
	defer rr.Close()
	state := map[string]any{graph.CfgKeyLineageID: fence.RunID, graph.CfgKeyCheckpointNS: CheckpointNamespace(fence.RunKey)}
	if latest, e := graph.NewCheckpointManager(saver).Latest(ctx, fence.RunID, CheckpointNamespace(fence.RunKey)); e != nil {
		return e
	} else if latest != nil {
		state[graph.CfgKeyCheckpointID] = latest.Checkpoint.ID
	}
	initial := r.bindings.InitialState
	msg := model.NewUserMessage("")
	if len(initial.Messages) > 0 {
		for _, m := range initial.Messages {
			if m.Role == model.RoleUser {
				msg = m
				break
			}
		}
	}
	events, err := rr.Run(withFence(ctx, fence), fence.Owner, fence.RunID, msg, agent.WithRuntimeState(state))
	if err != nil {
		return err
	}
	for evt := range events {
		if evt == nil {
			continue
		}
		if evt.Error != nil {
			return fmt.Errorf("graph execution: %s", evt.Error.Message)
		}
	}
	return nil
}
func graphBindingsWithFence(b GraphBindings, f agentruntime.Fence) GraphBindings { return b }

func cloneState(in State) State {
	out := in
	out.Messages = make([]model.Message, len(in.Messages))
	for i, msg := range in.Messages {
		raw, _ := json.Marshal(msg)
		_ = json.Unmarshal(raw, &out.Messages[i])
	}
	out.PendingCallIDs = append([]string(nil), in.PendingCallIDs...)
	out.AppliedCallIDs = map[string]bool{}
	for k, v := range in.AppliedCallIDs {
		out.AppliedCallIDs[k] = v
	}
	out.CompactionState = append(json.RawMessage(nil), in.CompactionState...)
	out.UsageAttempts = map[string]json.RawMessage{}
	for k, v := range in.UsageAttempts {
		out.UsageAttempts[k] = append(json.RawMessage(nil), v...)
	}
	return out
}
