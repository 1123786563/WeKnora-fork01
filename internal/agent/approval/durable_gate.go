package approval

import (
	"context"
	"errors"
	"fmt"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/google/uuid"
)

// DurableOAuthPark persists a pre-execution wait before the tool is
// dispatched: a durable run event for the recovery card, a linkage marker on
// the planned tool call, and the waiting_user park itself. It runs under the
// claiming worker's fence; a failed park aborts the wait.
type DurableOAuthPark func(
	ctx context.Context,
	fence agentruntime.Fence,
	dispatch agentruntime.ToolDispatch,
	pendingID string,
	req OAuthPendingRequest,
) error

var durableOAuthPark DurableOAuthPark

// SetDurableOAuthPark installs the process-wide park hook. It is installed
// once at boot by the session service, mirroring the graph executor
// registration; the builtin engine path never carries a run fence and is
// unaffected.
func SetDurableOAuthPark(fn DurableOAuthPark) { durableOAuthPark = fn }

// DurableGate wraps the live approval gate for durable runs. Without a run
// fence in the execution context it delegates every call, so the builtin
// engine keeps its in-memory waiting semantics. With a fence and a not-yet-
// dispatched tool it parks the run durably instead of blocking a goroutine:
// a restart cannot lose the wait, and an explicit user decision resumes it.
type DurableGate struct{ inner MCPApproval }

// NewDurableGate returns a gate that parks durable pre-execution waits. inner
// may be nil; the wrapper then only implements the parking paths.
func NewDurableGate(inner MCPApproval) *DurableGate { return &DurableGate{inner: inner} }

// NeedsApproval delegates to the wrapped live gate.
func (g *DurableGate) NeedsApproval(ctx context.Context, tenantID uint64, serviceID, toolName string) bool {
	if g == nil || g.inner == nil {
		return false
	}
	return g.inner.NeedsApproval(ctx, tenantID, serviceID, toolName)
}

// IsEnabled delegates to the wrapped live gate.
func (g *DurableGate) IsEnabled(ctx context.Context, tenantID uint64, serviceID, toolName string) (bool, error) {
	if g == nil || g.inner == nil {
		return false, errors.New("approval gate is unavailable")
	}
	return g.inner.IsEnabled(ctx, tenantID, serviceID, toolName)
}

// RequestAndWait parks a durable run when the human-approval wait happens
// before dispatch, mirroring the OAuth path: the run parks at waiting_user
// with an mcp_approve_ pending id and an explicit user decision resumes it.
// Without a fence (builtin engine) it delegates to the wrapped live gate.
func (g *DurableGate) RequestAndWait(ctx context.Context, req PendingRequest) (Decision, error) {
	park := durableOAuthPark
	fence, hasFence := agentruntime.RunFenceFromContext(ctx)
	dispatch, hasDispatch := agentruntime.ToolDispatchFromContext(ctx)
	if park == nil || !hasFence || !hasDispatch || dispatch.Attempt != 0 {
		if g != nil && g.inner != nil {
			return g.inner.RequestAndWait(ctx, req)
		}
		return Decision{}, errors.New("approval gate is unavailable")
	}
	pendingID := "mcp_approve_" + uuid.NewString()
	if err := park(ctx, fence, dispatch, pendingID, OAuthPendingRequest{
		TenantID: req.TenantID, UserID: req.UserID, SessionID: req.SessionID,
		AssistantMessageID: req.AssistantMessageID, RequestID: req.RequestID,
		EventBus: req.EventBus, ServiceID: req.ServiceID, ServiceName: req.ServiceName,
		MCPToolName: req.MCPToolName, ToolCallID: dispatch.CallID,
	}); err != nil {
		return Decision{}, err
	}
	return Decision{}, &agentruntime.ApprovalWaitError{
		PendingID: pendingID, ServiceID: req.ServiceID, ToolCallID: dispatch.CallID,
	}
}

// RequestOAuthAndWait parks a durable run when the OAuth reauthorization wait
// happens before dispatch; otherwise it delegates to the live gate so the
// in-conversation popup flow keeps working for the builtin engine.
func (g *DurableGate) RequestOAuthAndWait(ctx context.Context, req OAuthPendingRequest) (Decision, error) {
	park := durableOAuthPark
	fence, hasFence := agentruntime.RunFenceFromContext(ctx)
	dispatch, hasDispatch := agentruntime.ToolDispatchFromContext(ctx)
	if park == nil || !hasFence || !hasDispatch || dispatch.Attempt != 0 {
		if g != nil && g.inner != nil {
			if waiter, ok := g.inner.(interface {
				RequestOAuthAndWait(context.Context, OAuthPendingRequest) (Decision, error)
			}); ok {
				return waiter.RequestOAuthAndWait(ctx, req)
			}
		}
		return Decision{}, fmt.Errorf("mcp oauth authorization required for service %s", req.ServiceID)
	}
	pendingID := "mcp_oauth_" + uuid.NewString()
	if err := park(ctx, fence, dispatch, pendingID, req); err != nil {
		return Decision{}, err
	}
	return Decision{}, &agentruntime.OAuthWaitError{
		PendingID: pendingID, ServiceID: req.ServiceID, ToolCallID: dispatch.CallID,
	}
}
