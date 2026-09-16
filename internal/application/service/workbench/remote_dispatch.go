package workbench

import (
	"context"
	"fmt"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
)

// RemoteDispatcher is the narrow worker seam around the Paseo bridge. The
// intent is durable before start and an unknown result is persisted on every
// provider/transport error, so callers cannot accidentally retry with a new
// command id.
type RemoteDispatcher struct {
	dispatch *repository.ExecutionDispatchStore
}

func NewRemoteDispatcher(dispatch *repository.ExecutionDispatchStore) *RemoteDispatcher {
	return &RemoteDispatcher{dispatch: dispatch}
}

func (d *RemoteDispatcher) Dispatch(ctx context.Context, key agentruntime.RunKey, commandID, worker string, lease time.Duration, start func(context.Context) (string, error)) (string, error) {
	record, err := d.dispatch.ClaimDispatch(ctx, key, commandID, worker, lease)
	if err != nil {
		return "", err
	}
	if record.State == "completed" || record.State == "reconciled" {
		return record.ExternalID, nil
	}
	externalID, err := start(ctx)
	if err != nil {
		if reconcileErr := d.dispatch.ReconcileUnknown(ctx, record, "unknown", ""); reconcileErr != nil {
			return "", fmt.Errorf("remote dispatch failed (%v); durable unknown recovery failed: %w", err, reconcileErr)
		}
		return "", err
	}
	if err := d.dispatch.SaveReceipt(ctx, record, externalID); err != nil {
		if reconcileErr := d.dispatch.ReconcileUnknown(ctx, record, "receipt_persist_failed", externalID); reconcileErr != nil {
			return "", fmt.Errorf("receipt persistence failed (%v); durable reconciliation failed: %w", err, reconcileErr)
		}
		return "", err
	}
	return externalID, nil
}
