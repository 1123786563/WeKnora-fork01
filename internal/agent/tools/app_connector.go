package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
)

// ToolAppConnector is the agent-facing open-connector app action tool.
const ToolAppConnector = "app_connector"

// OCActionFacade is the trusted application seam the open-connector tool
// talks to. Both faces are identity-bound: the subject arrives from the
// authenticated caller and the session/tool-call identity from the engine's
// exec context — neither can be supplied by the model. The facade
// deliberately has NO approve and NO execute face: the tool can only request
// preparation and query state, so the model can never approve its own call
// and the ActionService budget entry is entered exactly once (never from the
// tool layer — no GatedAdapter, no second Begin).
type OCActionFacade interface {
	// PrepareForTool binds one logical tool call to one persisted action:
	// identical arguments return the SAME action; changed arguments conflict.
	PrepareForTool(ctx context.Context, subject appconn.OCSubject, sessionID, toolCallID, connectionID, actionID string, input json.RawMessage) (id string, err error)
	// StatusForTool reports the action's lifecycle state to the calling
	// tenant only (cross-space invisible).
	StatusForTool(ctx context.Context, subject appconn.OCSubject, actionID string) (state string, err error)
}

// appConnectorSchema is the frozen model-facing argument schema: ONLY
// connection_id, action_id and input. No identity field, no approve or
// execute control. The schema governs the model prompt only — undeclared
// fields a model still sends are dropped by the three-field argument
// envelope, not rejected by the registry; identity comes exclusively from
// the engine-injected context, never from arguments.
var appConnectorSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"connection_id": {"type": "string", "description": "The open-connector connection to act through, as configured in the workspace app connections."},
		"action_id": {"type": "string", "description": "The reviewed action to prepare, for example github.search."},
		"input": {"type": "object", "description": "The action arguments, matching the action's reviewed input schema."}
	},
	"required": ["connection_id", "action_id", "input"],
	"additionalProperties": false
}`)

const appConnectorDescription = "Prepare an open-connector app action and report its approval lifecycle. " +
	"The tool only prepares the action and queries its state: approval and execution belong to the user on the app actions surface — you can NEVER approve or execute it yourself. " +
	"Awaiting approval means asking the user to review the prepared action; once they approve and run it, calling again with the SAME arguments reports the recorded outcome. " +
	"Replaying the same call with different arguments is rejected as a conflict — start a new call instead."

// AppConnectorTool exposes open-connector app actions to the agent.
type AppConnectorTool struct {
	BaseTool
	actions OCActionFacade
}

// NewAppConnectorTool builds the tool over the trusted facade. A nil facade
// still yields a tool that enforces the identity contract (and refuses any
// execution attempt) — used by tests proving rejection happens BEFORE any
// facade call.
func NewAppConnectorTool(actions OCActionFacade) *AppConnectorTool {
	return &AppConnectorTool{
		BaseTool: NewBaseTool(ToolAppConnector, appConnectorDescription, appConnectorSchema),
		actions:  actions,
	}
}

// appConnectorArgs is the model-facing argument envelope.
type appConnectorArgs struct {
	ConnectionID string          `json:"connection_id"`
	ActionID     string          `json:"action_id"`
	Input        json.RawMessage `json:"input"`
}

// Execute derives the caller identity from the engine context (NEVER from
// the model's arguments), prepares the action through the facade, and maps
// the lifecycle state onto the tool contract:
//
//   - awaiting_approval / unknown park the call with the OC-specific wait
//     error (agentruntime.ErrOCActionApprovalWait — its own sentinel, never
//     MCP OAuth/approval); the run settles into the existing durable
//     waiting_user instead of manufacturing an outcome;
//   - every other state is a plain status answer the model can relay.
func (t *AppConnectorTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	// Identity first, exactly per the plan sketch: reject BEFORE any facade
	// call. A nil facade must be unreachable past this point.
	meta, ok := ToolExecFromContext(ctx)
	if !ok || meta.UserID == "" || meta.SessionID == "" || meta.ToolCallID == "" {
		return nil, errors.New("missing tool execution identity")
	}
	tenant, ok := types.TenantIDFromContext(ctx)
	if !ok || tenant == 0 {
		return nil, errors.New("missing tenant")
	}
	if t.actions == nil {
		return nil, errors.New("open-connector facade is not wired")
	}
	var call appConnectorArgs
	if err := json.Unmarshal(args, &call); err != nil {
		return nil, fmt.Errorf("invalid app_connector arguments: %w", err)
	}
	if call.ConnectionID == "" || call.ActionID == "" {
		return nil, errors.New("connection_id and action_id are required")
	}
	subject := appconn.OCSubject{TenantID: tenant, ActorID: meta.UserID}
	actionID, err := t.actions.PrepareForTool(ctx, subject, meta.SessionID, meta.ToolCallID, call.ConnectionID, call.ActionID, call.Input)
	if err != nil {
		return nil, err
	}
	state, err := t.actions.StatusForTool(ctx, subject, actionID)
	if err != nil {
		return nil, err
	}
	switch state {
	case appconn.ActionAwaitingApproval:
		// Pending: park the call for the HUMAN approval. The model can never
		// approve; the wait is the OC-specific error, never MCP OAuth.
		return nil, &agentruntime.OCActionWaitError{
			ActionID: actionID, ToolCallID: meta.ToolCallID, State: state,
		}
	case appconn.ActionUnknown:
		// Unknown provider outcome: wait + read-only check only. Never
		// re-dispatch, never claim the call is idempotent — the provider
		// query path resolves it.
		return nil, &agentruntime.OCActionWaitError{
			ActionID: actionID, ToolCallID: meta.ToolCallID, State: state,
		}
	}
	// Durable dispatch boundary, owned HERE like the MCP wrappers: the
	// registry withholds this tool from the eager boundary so the waits above
	// park BEFORE any attempt (never journaled as an unknown dispatch), while
	// every observable answer below begins its attempt after all gates — a
	// success therefore always carries its durable attempt
	// (ErrToolNotDispatched contract). On the builtin engine path (no
	// dispatch state in ctx) this is a no-op.
	if err := agentruntime.BeforeToolDispatch(ctx); err != nil {
		return nil, err
	}
	return ocStatusResult(actionID, state), nil
}

// ocStatusResult renders a queryable lifecycle state as a successful status
// answer: the tool succeeded at preparing/querying; the Data block carries
// what the model needs to relay (and, for terminal states, the recorded
// provider result is fetched on the next query via the same read-only path).
func ocStatusResult(actionID, state string) *types.ToolResult {
	note := ""
	switch state {
	case appconn.ActionAuthorized:
		note = "approved by the user; execution follows on the app actions surface"
	case appconn.ActionQueued, appconn.ActionDispatched:
		note = "execution in flight; query again to see the outcome"
	case appconn.ActionSucceeded:
		note = "the action completed successfully"
	case appconn.ActionFailed:
		note = "the action failed; see the app actions surface for details"
	default:
		note = "unexpected action state"
	}
	return &types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("app action %s is %s: %s", actionID, state, note),
		Data: map[string]interface{}{
			"action_id": actionID,
			"state":     state,
		},
	}
}
