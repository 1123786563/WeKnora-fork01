package native

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func nativeFailureCode(t *testing.T, err error) nativecontract.ErrorCode {
	t.Helper()
	var failure *nativecontract.Failure
	require.ErrorAs(t, err, &failure)
	return failure.Code
}

type nativeTestTool struct{ calls atomic.Int64 }

func (t *nativeTestTool) Declaration() *tool.Declaration            { return &tool.Declaration{Name: "write"} }
func (t *nativeTestTool) Call(context.Context, []byte) (any, error) { t.calls.Add(1); return "ok", nil }

type nativeTestStreamTool struct{ calls atomic.Int64 }

func (t *nativeTestStreamTool) Declaration() *tool.Declaration {
	return &tool.Declaration{Name: "stream-write"}
}
func (t *nativeTestStreamTool) StreamableCall(context.Context, []byte) (*tool.StreamReader, error) {
	t.calls.Add(1)
	return tool.NewStream(1).Reader, nil
}

type nativeTestScopeResolver struct {
	result nativecontract.Scope
	err    error
	calls  atomic.Int64
}

func (r *nativeTestScopeResolver) Resolve(context.Context) (nativecontract.Scope, error) {
	return r.result, r.err
}
func (r *nativeTestScopeResolver) Recheck(context.Context, nativecontract.Scope, []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	r.calls.Add(1)
	return r.result, r.err
}

type nativeTestReservation struct {
	err   error
	calls atomic.Int64
}

func (r *nativeTestReservation) ReserveAndConsume(context.Context, nativecontract.ToolDispatchRequest) error {
	r.calls.Add(1)
	return r.err
}

func nativeTestDispatchRequest() nativecontract.ToolDispatchRequest {
	run := nativecontract.RunIdentity{TenantID: 1, SessionID: "session-1", RunID: "run-1"}
	return nativecontract.ToolDispatchRequest{
		Scope:             nativecontract.Scope{TenantID: 1},
		Fence:             nativecontract.Fence{Run: run, Owner: "worker-1", Epoch: 1},
		Plan:              nativecontract.ToolPlan{Version: 1, Run: run, CallID: "call-1", ModelAttemptID: "model-1", Tool: nativecontract.ToolIdentity{Name: "write", SchemaHash: "schema", ConfigVersion: "config"}, Args: []byte(`{}`), ArgsHash: "hash", RequiredGrants: []nativecontract.ResourceGrant{{ResourceType: "connector", ResourceID: "install-1", Action: "write"}}},
		Attempt:           nativecontract.Attempt{ID: "tool-1", Run: run, Kind: nativecontract.ToolAttempt, LogicalCallID: "call-1", Number: 1, Epoch: 1},
		DecisionReference: "decision-1",
	}
}

func TestNativeToolRecoveryActionUsesOnlyConfirmedOrExplicitCapabilities(t *testing.T) {
	require.Equal(t, "reuse", RecoveryAction(nativecontract.RecoveryHold, true))
	require.Equal(t, "query", RecoveryAction(nativecontract.RecoveryQuery, false))
	require.Equal(t, "retry", RecoveryAction(nativecontract.RecoveryIdempotent, false))
	require.Equal(t, "retry", RecoveryAction(nativecontract.RecoveryReadOnly, false))
	require.Equal(t, "hold", RecoveryAction(nativecontract.RecoveryHold, false))
}

func TestNativeToolWrappersRejectRevokedCallableAndStreamableToolsBeforeDispatch(t *testing.T) {
	denied := func(context.Context, []byte) error { return errors.New("permission revoked") }
	callable := &nativeTestTool{}
	_, err := WrapCallableTool(callable, denied).Call(context.Background(), []byte(`{}`))
	require.Error(t, err)
	require.Zero(t, callable.calls.Load())

	streamable := &nativeTestStreamTool{}
	_, err = WrapStreamableTool(streamable, denied).StreamableCall(context.Background(), []byte(`{}`))
	require.Error(t, err)
	require.Zero(t, streamable.calls.Load())
}

func TestServerToolDispatchPreflightRejectsRevokedGrantBeforeReservation(t *testing.T) {
	request := nativeTestDispatchRequest()
	resolver := &nativeTestScopeResolver{result: nativecontract.Scope{TenantID: 1}}
	reservation := &nativeTestReservation{}
	preflight := ServerToolDispatchPreflight{Scopes: resolver, Reservations: reservation}

	err := preflight.Authorize(context.Background(), request)

	require.Equal(t, nativecontract.ErrForbidden, nativeFailureCode(t, err))
	require.Equal(t, int64(1), resolver.calls.Load())
	require.Zero(t, reservation.calls.Load())
}

func TestServerToolDispatchPreflightRejectsChangedOwnerOrPrincipalBeforeReservation(t *testing.T) {
	request := nativeTestDispatchRequest()
	request.Scope.SessionOwnerID = "owner-1"
	request.Scope.Principal = nativecontract.Principal{Type: "user", ID: "user-1"}
	for _, changed := range []nativecontract.Scope{
		{TenantID: 1, SessionOwnerID: "owner-2", Principal: request.Scope.Principal, Grants: request.Plan.RequiredGrants},
		{TenantID: 1, SessionOwnerID: request.Scope.SessionOwnerID, Principal: nativecontract.Principal{Type: "service", ID: "user-1"}, Grants: request.Plan.RequiredGrants},
	} {
		t.Run(changed.SessionOwnerID+changed.Principal.Type, func(t *testing.T) {
			reservation := &nativeTestReservation{}
			err := (ServerToolDispatchPreflight{Scopes: &nativeTestScopeResolver{result: changed}, Reservations: reservation}).Authorize(context.Background(), request)
			require.Equal(t, nativecontract.ErrForbidden, nativeFailureCode(t, err))
			require.Zero(t, reservation.calls.Load())
		})
	}
}

func TestServerToolDispatchPreflightReturnsBudgetAndFenceFailuresBeforeDelegate(t *testing.T) {
	request := nativeTestDispatchRequest()
	granted := nativecontract.Scope{TenantID: 1, Grants: request.Plan.RequiredGrants}
	for _, want := range []nativecontract.ErrorCode{nativecontract.ErrBudget, nativecontract.ErrLeaseLost} {
		t.Run(string(want), func(t *testing.T) {
			delegate := &nativeTestTool{}
			preflight := ServerToolDispatchPreflight{
				Scopes:       &nativeTestScopeResolver{result: granted},
				Reservations: &nativeTestReservation{err: &nativecontract.Failure{Code: want}},
			}
			_, err := WrapCallableTool(delegate, func(ctx context.Context, _ []byte) error {
				return preflight.Authorize(ctx, request)
			}).Call(context.Background(), request.Plan.Args)
			require.Equal(t, want, nativeFailureCode(t, err))
			require.Zero(t, delegate.calls.Load())
		})
	}
}

func TestRecoveryActionForPlanHoldsExpiredIdempotencyKey(t *testing.T) {
	plan := nativeTestDispatchRequest().Plan
	plan.Policy = nativecontract.RecoveryIdempotent
	plan.IdempotencyKey = "provider-key"
	plan.IdempotencyExpiresAt = time.Now().Add(-time.Second)
	require.Equal(t, "hold", RecoveryActionForPlan(plan, false, time.Now()))
}

func TestNativeStreamableToolIsUnsupportedBeforePreflightOrDelegate(t *testing.T) {
	delegate := &nativeTestStreamTool{}
	preflightCalls := atomic.Int64{}
	wrapped := WrapStreamableTool(delegate, func(context.Context, []byte) error {
		preflightCalls.Add(1)
		return nil
	})
	_, err := wrapped.StreamableCall(context.Background(), []byte(`{}`))
	require.Equal(t, nativecontract.ErrForbidden, nativeFailureCode(t, err))
	require.Zero(t, preflightCalls.Load())
	require.Zero(t, delegate.calls.Load())
}
