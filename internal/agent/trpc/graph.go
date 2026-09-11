package trpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

const (
	nodePrepare     = "prepare"
	nodeModel       = "model"
	nodePersistPlan = "persist_tool_plan"
	nodeDispatch    = "dispatch_one_tool"
	nodeApply       = "apply_result"
	nodeFinalize    = "finalize"
)

// NextAfterTool is the single routing rule for a serial tool batch.
func NextAfterTool(index, count int) string {
	if index < count {
		return nodeDispatch
	}
	return nodeModel
}

func stateFromGraph(in graph.State) (State, error) {
	raw, ok := in[StateKey]
	if !ok {
		return State{}, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, err
	}
	if s.AppliedCallIDs == nil {
		s.AppliedCallIDs = map[string]bool{}
	}
	return s, nil
}
func stateUpdate(s State) graph.State { return graph.State{StateKey: s} }

func buildGraph(b GraphBindings) (*graph.Graph, error) {
	schema := graph.MessagesStateSchema().AddField(StateKey, graph.StateField{Type: reflect.TypeOf(State{}), Reducer: graph.DefaultReducer})
	sg := graph.NewStateGraph(schema)
	sg.AddNode(nodePrepare, func(ctx context.Context, in graph.State) (any, error) {
		s, err := stateFromGraph(in)
		if err != nil {
			return nil, err
		}
		persistedCapabilities := !s.Capabilities.IsEmpty()
		if s.Version == 0 {
			s = b.InitialState
			if s.Version == 0 {
				s.Version = StateVersion
			}
		}
		if persistedCapabilities && !b.Capabilities.IsEmpty() {
			if err := s.Capabilities.CompatibleWith(b.Capabilities); err != nil {
				return nil, fmt.Errorf("capability compatibility: %w", err)
			}
		}
		if s.AppliedCallIDs == nil {
			s.AppliedCallIDs = map[string]bool{}
		}
		if s.Capabilities.IsEmpty() {
			s.Capabilities = b.Capabilities
		}
		for _, prompt := range []string{s.Capabilities.SystemPrompt, s.Capabilities.MemoryPrompt} {
			if prompt == "" {
				continue
			}
			found := false
			for _, m := range s.Messages {
				if m.Role == model.RoleSystem && m.Content == prompt {
					found = true
					break
				}
			}
			if !found {
				s.Messages = append([]model.Message{{Role: model.RoleSystem, Content: prompt}}, s.Messages...)
			}
		}
		return stateUpdate(s), nil
	})
	sg.AddNode(nodeModel, func(ctx context.Context, in graph.State) (any, error) {
		s, err := stateFromGraph(in)
		if err != nil {
			return nil, err
		}
		if b.Model == nil {
			return nil, fmt.Errorf("trpc model is required")
		}
		// A committed attempt id on re-entry means the previous model attempt
		// never completed. Announce the replacement so replay clients drop the
		// abandoned partial text instead of concatenating both attempts.
		if s.ModelAttemptID != "" {
			payload := map[string]string{"previous_attempt_id": s.ModelAttemptID}
			if raw, merr := json.Marshal(payload); merr == nil {
				b.emitRunEvent(ctx, agentruntime.RunEvent{
					AttemptID: s.ModelAttemptID, Type: "attempt_replaced", Payload: raw,
				})
			}
			s.ModelAttemptID = ""
		}
		// Safe node boundary: consume durable inject inputs before the model
		// call. The appended message and the steer id land in the same
		// checkpoint the SDK saves after this node, which is the exactly-once
		// boundary; consumed rows are then marked processed so the steer
		// queue depth guard does not saturate. A crash between the mark and
		// the checkpoint is safe: the resumed state still filters by
		// AppliedSteerIDs.
		consumed := make([]string, 0)
		if b.Inputs != nil {
			pending, perr := b.Inputs.ListPendingInputs(ctx, b.fenceFromContext(ctx).RunKey, "inject")
			if perr != nil {
				return nil, fmt.Errorf("list steering inputs: %w", perr)
			}
			applied := make(map[string]bool, len(s.AppliedSteerIDs))
			for _, id := range s.AppliedSteerIDs {
				applied[id] = true
			}
			for _, input := range pending {
				if applied[input.SteerID] {
					consumed = append(consumed, input.SteerID)
					continue
				}
				var payload struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}
				if err := json.Unmarshal(input.Message, &payload); err != nil ||
					payload.Role != string(model.RoleUser) || payload.Content == "" {
					return nil, fmt.Errorf("invalid steering input %s", input.SteerID)
				}
				s.Messages = append(s.Messages, model.NewUserMessage(payload.Content))
				s.AppliedSteerIDs = append(s.AppliedSteerIDs, input.SteerID)
				applied[input.SteerID] = true
				consumed = append(consumed, input.SteerID)
				evtPayload, _ := json.Marshal(map[string]string{"steer_id": input.SteerID})
				b.emitRunEvent(ctx, agentruntime.RunEvent{Type: "steer_injected", Payload: evtPayload})
			}
		}
		if len(consumed) > 0 {
			if consumer, ok := b.Inputs.(agentruntime.RunInputConsumer); ok {
				markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				if err := consumer.MarkInputsProcessed(markCtx, b.fenceFromContext(ctx).RunKey, consumed...); err != nil {
					logger := ctx
					_ = logger
				}
				cancel()
			}
		}
		// Stream keeps provider parity with the builtin engine; the frozen
		// tool projection lets the model plan calls the journal can execute.
		request := &model.Request{Messages: s.Messages, Tools: b.ModelTools}
		request.Stream = true
		responseCh, err := b.Model.GenerateContent(ctx, request)
		if err != nil {
			return nil, err
		}
		var response *model.Response
		for response = range responseCh {
			if response != nil && response.Done {
				break
			}
		}
		if response == nil || !response.Done || response.IsPartial || response.Error != nil || len(response.Choices) == 0 {
			if response != nil && response.Error != nil {
				return nil, fmt.Errorf("model response error: %w", response.Error)
			}
			return nil, fmt.Errorf("model returned partial or incomplete response")
		}
		msg := response.Choices[0].Message
		s.ModelAttemptID = response.ID
		s.Messages = append(s.Messages, msg)
		s.UsageAttempts = ensureUsage(s.UsageAttempts, response)
		return stateUpdate(s), nil
	})
	sg.AddNode(nodePersistPlan, func(ctx context.Context, in graph.State) (any, error) {
		s, err := stateFromGraph(in)
		if err != nil {
			return nil, err
		}
		if len(s.Messages) == 0 {
			return nil, fmt.Errorf("model plan is missing")
		}
		calls := s.Messages[len(s.Messages)-1].ToolCalls
		if len(calls) == 0 {
			s.PendingCallIDs = nil
			s.NextCallIndex = 0
			return stateUpdate(s), nil
		}
		s.PendingCallIDs = make([]string, len(calls))
		fence := b.fenceFromContext(ctx)
		for i, c := range calls {
			if c.ID == "" || c.Function.Name == "" || !json.Valid(c.Function.Arguments) || string(c.Function.Arguments) == "null" {
				return nil, fmt.Errorf("invalid tool call at index %d", i)
			}
			s.PendingCallIDs[i] = c.ID
			if b.Tools == nil {
				return nil, fmt.Errorf("tool executor is required")
			}
			if err := b.Tools.PreparePlan(ctx, fence, agentruntime.ToolPlan{Version: 1, CallID: c.ID, Name: c.Function.Name, Identity: c.Function.Name, ArgsHash: hashArgs(c.Function.Arguments), Args: c.Function.Arguments}); err != nil {
				return nil, err
			}
		}
		s.NextCallIndex = 0
		return stateUpdate(s), nil
	})
	sg.AddNode(nodeDispatch, func(ctx context.Context, in graph.State) (any, error) {
		s, err := stateFromGraph(in)
		if err != nil {
			return nil, err
		}
		if s.NextCallIndex >= len(s.PendingCallIDs) {
			return stateUpdate(s), nil
		}
		if b.Tools == nil {
			return nil, fmt.Errorf("tool executor is required")
		}
		assistantCalls := append([]model.ToolCall(nil), s.Messages[len(s.Messages)-1].ToolCalls...)
		for s.NextCallIndex < len(assistantCalls) {
			id := assistantCalls[s.NextCallIndex].ID
			var call model.ToolCall
			for _, c := range assistantCalls {
				if c.ID == id {
					call = c
					break
				}
			}
			if raw, merr := json.Marshal(map[string]string{"call_id": id, "tool": call.Function.Name}); merr == nil {
				b.emitRunEvent(ctx, agentruntime.RunEvent{Type: "tool_dispatched", Payload: raw})
			}
			result, err := b.Tools.Execute(ctx, b.fenceFromContext(ctx), agentruntime.ToolPlan{Version: 1, CallID: id, Name: call.Function.Name, Identity: call.Function.Name, ArgsHash: hashArgs(call.Function.Arguments), Args: call.Function.Arguments})
			if err != nil {
				return nil, err
			}
			if !s.AppliedCallIDs[id] {
				s.Messages = append(s.Messages, model.Message{
					Role: model.RoleTool, ToolID: id, Content: result.Result.Output,
				})
				s.AppliedCallIDs[id] = true
				payload := map[string]string{"call_id": id, "tool": call.Function.Name}
				if raw, merr := json.Marshal(payload); merr == nil {
					b.emitRunEvent(ctx, agentruntime.RunEvent{Type: "tool_result", Payload: raw})
				}
			}
			s.NextCallIndex++
		}
		return stateUpdate(s), nil
	})
	sg.AddNode(nodeApply, func(ctx context.Context, in graph.State) (any, error) {
		s, err := stateFromGraph(in)
		if err != nil {
			return nil, err
		}
		if s.NextCallIndex > 0 {
			for _, id := range s.PendingCallIDs {
				if err := applyDurableResult(ctx, b, s, id); err != nil {
					return nil, err
				}
			}
			id := s.PendingCallIDs[s.NextCallIndex-1]
			if !s.AppliedCallIDs[id] {
				return nil, fmt.Errorf("tool result %s was not durably applied", id)
			}
			found := false
			for _, m := range s.Messages {
				if m.Role == model.RoleTool && m.ToolID == id {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("tool result %s is missing from state", id)
			}
		}
		return stateUpdate(s), nil
	})
	sg.AddNode(nodeFinalize, func(ctx context.Context, in graph.State) (any, error) {
		s, err := stateFromGraph(in)
		if err != nil {
			return nil, err
		}
		if len(s.Messages) == 0 {
			return nil, fmt.Errorf("empty final state")
		}
		answer, err := json.Marshal(s.Messages[len(s.Messages)-1])
		if err != nil {
			return nil, err
		}
		if b.Finalize != nil {
			if err := b.Finalize(ctx, b.fenceFromContext(ctx), answer); err != nil {
				return nil, err
			}
		}
		return stateUpdate(s), nil
	})
	sg.AddEdge(graph.Start, nodePrepare).AddEdge(nodePrepare, nodeModel).AddEdge(nodeModel, nodePersistPlan)
	sg.AddConditionalEdges(nodePersistPlan, func(ctx context.Context, in graph.State) (string, error) {
		s, e := stateFromGraph(in)
		if e != nil {
			return "", e
		}
		if len(s.PendingCallIDs) == 0 {
			return nodeFinalize, nil
		}
		return nodeDispatch, nil
	}, map[string]string{nodeFinalize: nodeFinalize, nodeDispatch: nodeDispatch})
	sg.AddEdge(nodeDispatch, nodeApply)
	sg.AddConditionalEdges(nodeApply, func(ctx context.Context, in graph.State) (string, error) {
		s, e := stateFromGraph(in)
		if e != nil {
			return "", e
		}
		return NextAfterTool(s.NextCallIndex, len(s.PendingCallIDs)), nil
	}, map[string]string{nodeModel: nodeModel, nodeDispatch: nodeDispatch})
	sg.AddEdge(nodeFinalize, graph.End)
	return sg.SetEntryPoint(nodePrepare).SetFinishPoint(nodeFinalize).Compile()
}

// context values are private to this package and avoid putting live bindings in checkpoints.
type fenceKey struct{}
type resultKey struct{}

func withFence(ctx context.Context, f agentruntime.Fence) context.Context {
	return context.WithValue(ctx, fenceKey{}, f)
}
func (b GraphBindings) fenceFromContext(ctx context.Context) agentruntime.Fence {
	f, _ := ctx.Value(fenceKey{}).(agentruntime.Fence)
	return f
}

type toolResult struct {
	id     string
	result agentruntime.StoredToolResult
}

func withResult(ctx context.Context, id string, r agentruntime.StoredToolResult) context.Context {
	return context.WithValue(ctx, resultKey{}, toolResult{id: id, result: r})
}
func resultFromContext(ctx context.Context, id string) (agentruntime.StoredToolResult, bool) {
	v, ok := ctx.Value(resultKey{}).(toolResult)
	return v.result, ok && v.id == id
}

func hashArgs(args []byte) string { h := sha256.Sum256(args); return hex.EncodeToString(h[:]) }
func ensureUsage(m map[string]json.RawMessage, r *model.Response) map[string]json.RawMessage {
	if m == nil {
		m = map[string]json.RawMessage{}
	}
	if r == nil || r.ID == "" || r.Usage == nil {
		return m
	}
	raw, _ := json.Marshal(ModelAttempt{Version: 1, Response: r})
	m[r.ID] = raw
	return m
}

func toolOutputForCall(s State, id string) string {
	for _, m := range s.Messages {
		if m.Role == model.RoleTool && m.ToolID == id {
			return m.Content
		}
	}
	return ""
}

func applyDurableResult(ctx context.Context, b GraphBindings, s State, id string) error {
	if b.Tools == nil {
		return fmt.Errorf("durable tool result %s cannot be verified without tool executor", id)
	}
	durable, err := b.Tools.VerifyResult(ctx, b.fenceFromContext(ctx), id)
	if err != nil {
		return fmt.Errorf("durable tool result %s unavailable: %w", id, err)
	}
	if durable.Result.Output != toolOutputForCall(s, id) {
		return fmt.Errorf("durable tool result %s does not match applied result", id)
	}
	return nil
}
