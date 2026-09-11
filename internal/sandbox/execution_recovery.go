package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("sandbox execution resource not found")

// ExecutionRef is the durable identity of work submitted to a sandbox. All
// fields are provider scoped except TenantID, which is part of the security
// boundary and must be checked before any provider call.
type ExecutionRef struct {
	TenantID    uint64 `json:"tenant_id"`
	SessionID   string `json:"session_id"`
	Provider    string `json:"provider"`
	ConfigID    string `json:"config_id"`
	InstanceID  string `json:"instance_id"`
	Generation  string `json:"generation"`
	TaskID      string `json:"task_id"`
	WorkspaceID string `json:"workspace_id"`
}

// Validate checks that a reference is specific enough to query safely.
func (r ExecutionRef) Validate() error {
	if r.TenantID == 0 || strings.TrimSpace(r.SessionID) == "" {
		return errors.New("sandbox execution reference requires tenant and session")
	}
	for name, value := range map[string]string{"provider": r.Provider, "config": r.ConfigID, "instance": r.InstanceID, "generation": r.Generation, "task": r.TaskID, "workspace": r.WorkspaceID} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("sandbox execution reference requires %s", name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("sandbox execution reference %s is invalid", name)
		}
	}
	return nil
}

type ExecutionObservation struct {
	State  string
	Result json.RawMessage
}

// Normalize validates provider observations. Unknown states are deliberately
// retained as unknown; callers must never infer completion from a missing task.
func (o ExecutionObservation) Normalize() ExecutionObservation {
	o.State = strings.ToLower(strings.TrimSpace(o.State))
	switch o.State {
	case "running", "succeeded", "failed", "unknown", "missing":
	default:
		o.State = "unknown"
	}
	return o
}

type ExecutionRecovery interface {
	Observe(context.Context, ExecutionRef) (ExecutionObservation, error)
	CancelExecution(context.Context, ExecutionRef) error
}

func CanImportObservation(o ExecutionObservation) bool {
	o = o.Normalize()
	return (o.State == "succeeded" || o.State == "failed") && len(o.Result) > 0
}

// ObserveOnlyRecovery is a small guard used by recovery paths. It intentionally
// exposes no Create operation: a recovery attempt may reconnect or observe an
// existing instance, but can never manufacture an empty workspace.
type ObserveOnlyRecovery struct{ Recovery ExecutionRecovery }

func (r ObserveOnlyRecovery) Observe(ctx context.Context, ref ExecutionRef) (ExecutionObservation, error) {
	if r.Recovery == nil {
		return ExecutionObservation{State: "unknown"}, errors.New("sandbox execution recovery is unavailable")
	}
	if err := ref.Validate(); err != nil {
		return ExecutionObservation{State: "unknown"}, err
	}
	return r.Recovery.Observe(ctx, ref)
}
func (r ObserveOnlyRecovery) CancelExecution(ctx context.Context, ref ExecutionRef) error {
	if r.Recovery == nil {
		return errors.New("sandbox execution recovery is unavailable")
	}
	if err := ref.Validate(); err != nil {
		return err
	}
	return r.Recovery.CancelExecution(ctx, ref)
}
