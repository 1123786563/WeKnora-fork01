package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
)

// capturingFacade records what the tool derived from the context and answers
// prepare/status from a script, so tests can pin the exact identity contract
// without touching persistence.
type capturingFacade struct {
	prepareErr    error
	prepareID     string
	statusState   string
	statusErr     error
	lastSubject   appconn.OCSubject
	lastSessionID string
	lastToolCall  string
	lastConnID    string
	lastActionRef string
	lastInput     json.RawMessage
	calls         int
}

func (f *capturingFacade) PrepareForTool(
	_ context.Context, subject appconn.OCSubject, sessionID, toolCallID, connectionID, actionID string, input json.RawMessage,
) (string, error) {
	f.calls++
	f.lastSubject = subject
	f.lastSessionID = sessionID
	f.lastToolCall = toolCallID
	f.lastConnID = connectionID
	f.lastActionRef = actionID
	f.lastInput = append(json.RawMessage(nil), input...)
	if f.prepareErr != nil {
		return "", f.prepareErr
	}
	return f.prepareID, nil
}

func (f *capturingFacade) StatusForTool(_ context.Context, _ appconn.OCSubject, _ string) (string, error) {
	if f.statusErr != nil {
		return "", f.statusErr
	}
	return f.statusState, nil
}

// toolExecCtx builds the context the agent engine attaches for one tool call.
func toolExecCtx(t *testing.T, tenant uint64, userID, sessionID, toolCallID string) context.Context {
	t.Helper()
	ctx := context.Background()
	if tenant != 0 {
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
	}
	if userID == "" && sessionID == "" && toolCallID == "" {
		return ctx
	}
	return WithToolExecContext(ctx, &ToolExecContext{
		UserID: userID, SessionID: sessionID, ToolCallID: toolCallID,
	})
}

func TestAppConnectorRejectsMissingExecIdentity(t *testing.T) {
	tool := NewAppConnectorTool(nil)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"connection_id":"c","action_id":"a","input":{}}`))
	if err == nil {
		t.Fatal("anonymous tool execution")
	}
}

// A context with exec metadata but no tenant is rejected BEFORE any facade
// call: the facade here is nil, so reaching it would panic the test.
func TestAppConnectorRejectsMissingTenantBeforeFacade(t *testing.T) {
	tool := NewAppConnectorTool(nil)
	_, err := tool.Execute(
		toolExecCtx(t, 0, "user-1", "sess-1", "call-1"),
		json.RawMessage(`{"connection_id":"c","action_id":"a","input":{}}`),
	)
	if err == nil {
		t.Fatal("expected missing-tenant rejection")
	}
	if got := err.Error(); got != "missing tenant" {
		t.Fatalf("unexpected error %q", got)
	}
}

// The identity handed to the facade comes from the context (engine-attached
// exec metadata + tenant), never from model-supplied arguments: the schema
// has no field for any of them.
func TestAppConnectorDerivesIdentityFromContext(t *testing.T) {
	facade := &capturingFacade{prepareID: "ocact_1", statusState: "authorized"}
	tool := NewAppConnectorTool(facade)
	result, err := tool.Execute(
		toolExecCtx(t, 7, "user-1", "sess-1", "call-1"),
		json.RawMessage(`{"connection_id":"conn-9","action_id":"send_message","input":{"target":"u","body":"hi"}}`),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %+v", result)
	}
	if facade.lastSubject != (appconn.OCSubject{TenantID: 7, ActorID: "user-1"}) {
		t.Fatalf("subject mismatch: %+v", facade.lastSubject)
	}
	if facade.lastSessionID != "sess-1" || facade.lastToolCall != "call-1" {
		t.Fatalf("exec identity mismatch: session=%q call=%q", facade.lastSessionID, facade.lastToolCall)
	}
	if facade.lastConnID != "conn-9" || facade.lastActionRef != "send_message" {
		t.Fatalf("args mismatch: conn=%q action=%q", facade.lastConnID, facade.lastActionRef)
	}
	if string(facade.lastInput) != `{"target":"u","body":"hi"}` {
		t.Fatalf("input not passed through verbatim: %s", facade.lastInput)
	}
}

// A pending action parks the call with the OC-specific wait error. It must be
// classifiable on its own sentinel and must NEVER masquerade as MCP OAuth or
// MCP tool approval: those drive reconnect/approval flows that do not exist
// for open-connector actions.
func TestAppConnectorAwaitingApprovalParksAsOCWait(t *testing.T) {
	facade := &capturingFacade{prepareID: "ocact_42", statusState: "awaiting_approval"}
	tool := NewAppConnectorTool(facade)
	_, err := tool.Execute(
		toolExecCtx(t, 7, "user-1", "sess-1", "call-1"),
		json.RawMessage(`{"connection_id":"c","action_id":"a","input":{}}`),
	)
	if err == nil {
		t.Fatal("awaiting_approval must surface as an error, not a success")
	}
	if !errors.Is(err, agentruntime.ErrOCActionApprovalWait) {
		t.Fatalf("error is not the OC wait sentinel: %v", err)
	}
	if errors.Is(err, agentruntime.ErrMCPOAuthWait) || errors.Is(err, agentruntime.ErrMCPApprovalWait) {
		t.Fatalf("OC wait masqueraded as an MCP wait: %v", err)
	}
	var wait *agentruntime.OCActionWaitError
	if !errors.As(err, &wait) {
		t.Fatalf("error does not carry the OC wait type: %v", err)
	}
	if wait.ActionID != "ocact_42" || wait.ToolCallID != "call-1" {
		t.Fatalf("wait identity mismatch: %+v", wait)
	}
	if !errors.Is(wait.Unwrap(), agentruntime.ErrOCActionApprovalWait) {
		t.Fatal("unwrap must reach the sentinel")
	}
}

// An unknown provider outcome maps to the same durable wait family: the tool
// performs only the read-only status check and never re-dispatches; the
// outcome resolves through the provider query path, so the call parks.
func TestAppConnectorUnknownOutcomeParksAsOCWait(t *testing.T) {
	facade := &capturingFacade{prepareID: "ocact_43", statusState: "unknown"}
	tool := NewAppConnectorTool(facade)
	_, err := tool.Execute(
		toolExecCtx(t, 7, "user-1", "sess-1", "call-1"),
		json.RawMessage(`{"connection_id":"c","action_id":"a","input":{}}`),
	)
	if err == nil || !errors.Is(err, agentruntime.ErrOCActionApprovalWait) {
		t.Fatalf("unknown must park with the OC wait error: %v", err)
	}
	var wait *agentruntime.OCActionWaitError
	if !errors.As(err, &wait) || wait.State != "unknown" {
		t.Fatalf("wait must carry the unknown state: %v", err)
	}
}

// Terminal and in-flight states are plain query answers: the tool succeeds at
// reporting them and the model learns the action outcome without any ability
// to approve or re-execute.
func TestAppConnectorReportsQueryableStates(t *testing.T) {
	cases := []struct {
		state string
	}{
		{"authorized"}, {"queued"}, {"dispatched"},
		{"succeeded"}, {"failed"},
	}
	for _, tc := range cases {
		facade := &capturingFacade{prepareID: "ocact_1", statusState: tc.state}
		tool := NewAppConnectorTool(facade)
		result, err := tool.Execute(
			toolExecCtx(t, 7, "user-1", "sess-1", "call-1"),
			json.RawMessage(`{"connection_id":"c","action_id":"a","input":{}}`),
		)
		if err != nil {
			t.Fatalf("state %s: unexpected error %v", tc.state, err)
		}
		if !result.Success {
			t.Fatalf("state %s: expected successful status answer: %+v", tc.state, result)
		}
		if result.Data["state"] != tc.state || result.Data["action_id"] != "ocact_1" {
			t.Fatalf("state %s: data mismatch: %+v", tc.state, result.Data)
		}
	}
}

// Changed args under the same tool call id are a conflict: the model must
// start a NEW call instead of mutating a bound one, and nothing may hint that
// retrying the same call could rebind it.
func TestAppConnectorSurfacesArgsConflict(t *testing.T) {
	facade := &capturingFacade{prepareErr: appconn.ErrInvalidArgs}
	tool := NewAppConnectorTool(facade)
	_, err := tool.Execute(
		toolExecCtx(t, 7, "user-1", "sess-1", "call-1"),
		json.RawMessage(`{"connection_id":"c","action_id":"a","input":{}}`),
	)
	if err == nil {
		t.Fatal("expected conflict error to surface")
	}
}

// The schema exposes ONLY connection_id/action_id/input. No identity, no
// approve/execute control surface: the model can never approve its own call
// and can never supply tenant/session/caller identity.
func TestAppConnectorSchemaExposesOnlyCallFields(t *testing.T) {
	tool := NewAppConnectorTool(nil)
	schema := tool.Parameters()
	if len(schema) == 0 {
		t.Fatal("tool must publish a parameters schema")
	}
	var parsed struct {
		Type                 string                     `json:"type"`
		Properties           map[string]json.RawMessage `json:"properties"`
		Required             []string                   `json:"required"`
		AdditionalProperties json.RawMessage            `json:"additionalProperties"`
	}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if parsed.Type != "object" {
		t.Fatalf("schema root must be object, got %q", parsed.Type)
	}
	if len(parsed.Properties) != 3 {
		t.Fatalf("schema must expose exactly 3 properties, got %d: %v", len(parsed.Properties), parsed.Properties)
	}
	for _, name := range []string{"connection_id", "action_id", "input"} {
		if _, ok := parsed.Properties[name]; !ok {
			t.Fatalf("schema is missing property %q", name)
		}
	}
	for _, banned := range []string{"tenant_id", "session_id", "user_id", "tool_call_id", "approve", "approved", "execute", "actor_id"} {
		if _, ok := parsed.Properties[banned]; ok {
			t.Fatalf("schema must never expose %q", banned)
		}
	}
	if strings.TrimSpace(string(parsed.AdditionalProperties)) != "false" {
		t.Fatalf("schema should reject undeclared fields (additionalProperties=false), got %s", parsed.AdditionalProperties)
	}
	if tool.Name() != "app_connector" {
		t.Fatalf("unexpected tool name %q", tool.Name())
	}
}

// ---------------------------------------------------------------------------
// T13-F-5 carry: the OC wait must not widen the Begin-claim orphan window.
// The open-connector tool performs no external write and allocates nothing;
// on the durable path its approval wait parks the run at the EXISTING
// waiting_user without ever beginning a dispatch attempt - so there is no
// new pre-allocation that could orphan. Driven through the REAL ToolExecutor
// + REAL ToolRegistry over a stub facade and a recording journal.
// ---------------------------------------------------------------------------

// ocF5RunStore is the minimal RunStore the executor requires.
type ocF5RunStore struct{ status string }

func (s *ocF5RunStore) Get(context.Context, agentruntime.RunKey) (agentruntime.Run, error) {
	return agentruntime.Run{
		Key:   agentruntime.RunKey{TenantID: 7, RunID: "run-1"},
		Owner: "worker-1", Epoch: 1, Status: s.status,
	}, nil
}

func (*ocF5RunStore) Admit(context.Context, agentruntime.Admission) (agentruntime.Run, error) {
	return agentruntime.Run{}, agentruntime.ErrConflict
}

func (*ocF5RunStore) Claim(context.Context, agentruntime.RunKey, string, time.Duration) (agentruntime.Fence, error) {
	return agentruntime.Fence{}, agentruntime.ErrConflict
}

func (*ocF5RunStore) Renew(context.Context, agentruntime.Fence, time.Duration) error { return nil }

func (*ocF5RunStore) Scan(context.Context, int) ([]agentruntime.RunKey, error) { return nil, nil }

func (*ocF5RunStore) SaveCheckpoint(context.Context, agentruntime.Fence, agentruntime.CheckpointRecord) error {
	return nil
}

func (*ocF5RunStore) SetStatus(context.Context, agentruntime.Fence, string, string) error { return nil }

func (*ocF5RunStore) LoadCheckpoint(context.Context, agentruntime.RunKey) (agentruntime.CheckpointRecord, error) {
	return agentruntime.CheckpointRecord{}, agentruntime.ErrNotFound
}

// ocF5Journal records every transition the executor asks for.
type ocF5Journal struct {
	mu          sync.Mutex
	beginCalls  int
	markUnknown int
	plans       int
	records     map[string]agentruntime.ToolRecord
}

func (j *ocF5Journal) EnsureToolPlan(_ context.Context, _ agentruntime.Fence, plan agentruntime.ToolPlan) (agentruntime.ToolRecord, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.records == nil {
		j.records = map[string]agentruntime.ToolRecord{}
	}
	record, ok := j.records[plan.CallID]
	if !ok {
		j.plans++
		record = agentruntime.ToolRecord{Plan: plan, Status: agentruntime.ToolStatusPlanned}
		j.records[plan.CallID] = record
	}
	return record, nil
}

func (j *ocF5Journal) BeginToolAttempt(context.Context, agentruntime.Fence, string, ...int64) (agentruntime.ToolAttempt, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.beginCalls++
	return agentruntime.ToolAttempt{Number: j.beginCalls}, nil
}

func (j *ocF5Journal) ReviseToolPlan(_ context.Context, _ agentruntime.Fence, callID string, _ int64, args json.RawMessage) (agentruntime.ToolPlan, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	record := j.records[callID]
	record.Plan.Args = args
	j.records[callID] = record
	return record.Plan, nil
}

func (j *ocF5Journal) CommitToolResult(_ context.Context, _ agentruntime.Fence, _ agentruntime.ToolAttempt, _ agentruntime.StoredToolResult) error {
	return nil
}

func (j *ocF5Journal) CommitToolRejection(context.Context, agentruntime.Fence, string, agentruntime.StoredToolResult) error {
	return nil
}

func (j *ocF5Journal) MarkToolUnknown(context.Context, agentruntime.Fence, agentruntime.ToolAttempt, string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.markUnknown++
	return nil
}

// TestAppConnectorWaitParksDurablyWithoutDispatchAttempt proves the T14
// durable mapping and the T13-F-5 no-widen property together: awaiting
// approval parks the run at waiting_user via the existing decision waiter,
// NO dispatch attempt is begun (nothing to pair with a budget Begin), the
// outcome is never classified unknown, and resuming the same call re-queries
// the action - the same logical tool call, idempotently.
func TestAppConnectorWaitParksDurablyWithoutDispatchAttempt(t *testing.T) {
	facade := &capturingFacade{prepareID: "ocact_park", statusState: "awaiting_approval"}
	registry := NewToolRegistry()
	registry.RegisterTool(NewAppConnectorTool(facade))

	journal := &ocF5Journal{}
	runs := &ocF5RunStore{status: "running"}
	var waited []string
	var waitMu sync.Mutex
	// Mirror the production durable path (agent_run_graph.go): the graph
	// executor attaches the session exec identity before each registry call.
	execute := func(tctx context.Context, name string, args json.RawMessage) (*types.ToolResult, error) {
		meta := &ToolExecContext{SessionID: "sess-1", UserID: "user-1"}
		if dispatch, ok := agentruntime.ToolDispatchFromContext(tctx); ok {
			meta.ToolCallID = dispatch.CallID
		}
		return registry.ExecuteTool(WithToolExecContext(tctx, meta), name, args)
	}
	executor := agentruntime.NewToolExecutor(runs, journal, execute)
	executor.SetWaitForDecision(func(_ context.Context, _ agentruntime.Fence, pendingID string) error {
		waitMu.Lock()
		defer waitMu.Unlock()
		waited = append(waited, pendingID)
		return nil
	})

	plan := agentruntime.ToolPlan{
		CallID: "call-park", Name: ToolAppConnector, Identity: "app_connector",
		ArgsHash: "args-hash-1", Args: json.RawMessage(`{"connection_id":"c","action_id":"a","input":{}}`),
	}
	fence := agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 7, RunID: "run-1"}, Owner: "worker-1", Epoch: 1}

	// The durable executor installs the dispatch context itself; the tool
	// reads identity from it (tenant comes from the outer ctx).
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	_, err := executor.Execute(ctx, fence, plan)
	if err == nil || !errors.Is(err, agentruntime.ErrOCActionApprovalWait) {
		t.Fatalf("expected the OC wait error to surface, got %v", err)
	}
	if len(waited) != 1 || waited[0] != "call-park" {
		t.Fatalf("run must park at waiting_user keyed by the tool call: %v", waited)
	}
	if journal.beginCalls != 0 {
		t.Fatalf("an approval wait must never begin a dispatch attempt (T13-F-5: no new pre-allocation), began %d", journal.beginCalls)
	}
	if journal.markUnknown != 0 {
		t.Fatalf("an approval wait must never be classified unknown, marked %d", journal.markUnknown)
	}

	// Resume after the human approved and ran the action: the same logical
	// call re-prepares idempotently and reads the recorded outcome.
	facade.statusState = "succeeded"
	stored, err := executor.Execute(ctx, fence, plan)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !stored.Result.Success || stored.Result.Data["state"] != "succeeded" {
		t.Fatalf("resume must read back the recorded outcome: %+v", stored.Result)
	}
	if facade.calls != 2 {
		t.Fatalf("resume must re-query the action through the same tool call, calls=%d", facade.calls)
	}
	if journal.beginCalls != 1 || journal.markUnknown != 0 {
		t.Fatalf("exactly the observable answer begins one durable attempt, never an unknown: begins=%d unknowns=%d", journal.beginCalls, journal.markUnknown)
	}
}
