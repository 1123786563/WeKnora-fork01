package opencode

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/modules/craft"
)

// RunSessionBinding is a server-resolved OpenCode session bound to the
// persisted RunView. Client must already carry the matching immutable
// directory binding; Directory is repeated here so the executor can verify
// that contract before making any runtime request.
type RunSessionBinding struct {
	Key       craft.RunViewKey
	Client    *Client
	SessionID string
	Directory string
}

// RunSessionResolver reloads the persisted RunView for the authenticated
// delegation's scope and Run ID on every operation. Implementations must not
// derive the binding from prompt text, workspace OpenCodeSessionID, or model
// output.
type RunSessionResolver interface {
	ResolveRunSession(context.Context, craft.Task) (RunSessionBinding, error)
}

func expectedRunViewKey(task craft.Task) (craft.RunViewKey, error) {
	key := craft.RunViewKey{
		TenantID: task.Scope.TenantID, OwnerID: task.Scope.UserID,
		SessionID: task.Scope.SessionID, RunID: task.Fence.RunID,
	}
	if task.Fence.TenantID != task.Scope.TenantID || task.Fence.Owner == "" || task.Fence.Epoch <= 0 {
		return craft.RunViewKey{}, fmt.Errorf("%w: delegation fence is incomplete or belongs to a different tenant", craft.ErrForbidden)
	}
	if err := craft.ValidateRunViewKey(key); err != nil {
		return craft.RunViewKey{}, err
	}
	return key, nil
}

func (e *Executor) resolveRunSession(ctx context.Context, task craft.Task) (RunSessionBinding, error) {
	if e.resolver == nil {
		return RunSessionBinding{}, fmt.Errorf("%w: OpenCode executor has no Run session resolver", craft.ErrUnsupported)
	}
	expected, err := expectedRunViewKey(task)
	if err != nil {
		return RunSessionBinding{}, err
	}
	binding, err := e.resolver.ResolveRunSession(ctx, task)
	if err != nil {
		return RunSessionBinding{}, err
	}
	if binding.Key != expected {
		return RunSessionBinding{}, fmt.Errorf("%w: resolved OpenCode session belongs to a different RunView", craft.ErrForbidden)
	}
	if binding.Client == nil || binding.Client.base == nil || binding.Client.http == nil ||
		!validSessionID(binding.SessionID) || binding.Directory == "" ||
		binding.Client.directory != binding.Directory || validateDirectory(binding.Directory) != nil {
		return RunSessionBinding{}, fmt.Errorf("%w: resolved OpenCode Run binding is incomplete or inconsistent", craft.ErrUnsupported)
	}
	return binding, nil
}
