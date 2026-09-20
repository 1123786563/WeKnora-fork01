package native

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// RecoveryAction chooses the only safe next step from a durable tool policy.
// A confirmed outcome is reused. Unknown non-idempotent effects are held for
// human resolution; this package does not wire any provider SDK or retry path.
func RecoveryAction(policy nativecontract.RecoveryPolicy, confirmed bool) string {
	if confirmed {
		return "reuse"
	}
	switch policy {
	case nativecontract.RecoveryQuery:
		return "query"
	case nativecontract.RecoveryIdempotent, nativecontract.RecoveryReadOnly:
		return "retry"
	default:
		return "hold"
	}
}

// RecoveryActionForPlan adds the provider idempotency-key lifetime check that
// RecoveryAction's policy-only signature cannot express. An expired key is
// never evidence that a retry is safe.
func RecoveryActionForPlan(plan nativecontract.ToolPlan, confirmed bool, now time.Time) string {
	action := RecoveryAction(plan.Policy, confirmed)
	if action == "retry" && plan.Policy == nativecontract.RecoveryIdempotent &&
		(plan.IdempotencyKey == "" || !plan.IdempotencyExpiresAt.After(now)) {
		return "hold"
	}
	return action
}

// ServerToolDispatchPreflight is the minimal server-owned governance seam.
// The resolver observes current authority; Reservations owns one atomic
// durable operation for the fence, budget reservation, and P2.4 decision
// consumption. No SDK runner, provider, connector, or MCP integration is
// constructed here.
type ServerToolDispatchPreflight struct {
	Scopes       nativecontract.ScopeResolver
	Reservations nativecontract.ToolDispatchReservation
}

var _ nativecontract.ToolDispatchPreflight = ServerToolDispatchPreflight{}

func (p ServerToolDispatchPreflight) Authorize(ctx context.Context, request nativecontract.ToolDispatchRequest) error {
	if !validDispatchRequest(request) || p.Scopes == nil || p.Reservations == nil {
		return nativeToolFailure(nativecontract.ErrStore, "server-owned tool dispatch preflight is unavailable")
	}
	current, err := p.Scopes.Recheck(ctx, request.Scope, request.Plan.RequiredGrants)
	if err != nil {
		return err
	}
	if current.TenantID != request.Scope.TenantID || current.SessionOwnerID != request.Scope.SessionOwnerID ||
		current.Principal != request.Scope.Principal || !containsGrants(current.Grants, request.Plan.RequiredGrants) {
		return nativeToolFailure(nativecontract.ErrForbidden, "tool dispatch grants were revoked")
	}
	return p.Reservations.ReserveAndConsume(ctx, request)
}

func validDispatchRequest(request nativecontract.ToolDispatchRequest) bool {
	return request.DecisionReference != "" && request.Scope.TenantID != 0 &&
		request.Scope.TenantID == request.Fence.Run.TenantID && request.Fence.Run.RunID != "" &&
		request.Fence.Owner != "" && request.Fence.Epoch > 0 && request.Plan.Version > 0 &&
		request.Plan.Run == request.Fence.Run && request.Attempt.Run == request.Fence.Run &&
		request.Attempt.Kind == nativecontract.ToolAttempt && request.Attempt.ID != "" &&
		request.Attempt.Epoch == request.Fence.Epoch && request.Attempt.LogicalCallID == request.Plan.CallID
}

func containsGrants(have, required []nativecontract.ResourceGrant) bool {
	for _, wanted := range required {
		found := false
		for _, grant := range have {
			if grant == wanted {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func nativeToolFailure(code nativecontract.ErrorCode, message string) error {
	return &nativecontract.Failure{Code: code, Message: message, Effect: nativecontract.EffectNotDispatched}
}

// ToolPreflight is the last application-owned validation before a tool can
// reach an observable external action. P3 supplies the durable plan, current
// authorization, budget, and fence checks; the wrapper only enforces their
// ordering for both SDK callable forms.
type ToolPreflight func(context.Context, []byte) error

type guardedCallableTool struct {
	tool.CallableTool
	preflight ToolPreflight
}

func WrapCallableTool(delegate tool.CallableTool, preflight ToolPreflight) tool.CallableTool {
	return &guardedCallableTool{CallableTool: delegate, preflight: preflight}
}

func (t *guardedCallableTool) Call(ctx context.Context, args []byte) (any, error) {
	if err := t.preflight(ctx, args); err != nil {
		return nil, err
	}
	return t.CallableTool.Call(ctx, args)
}

type guardedStreamableTool struct {
	tool.StreamableTool
	preflight ToolPreflight
}

func WrapStreamableTool(delegate tool.StreamableTool, preflight ToolPreflight) tool.StreamableTool {
	return &guardedStreamableTool{StreamableTool: delegate, preflight: preflight}
}

func (t *guardedStreamableTool) StreamableCall(ctx context.Context, args []byte) (*tool.StreamReader, error) {
	return nil, nativeToolFailure(nativecontract.ErrForbidden, "streamable tools require a completion-aware durable outcome adapter")
}
