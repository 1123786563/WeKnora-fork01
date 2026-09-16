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

var ErrProviderUnavailable = fmt.Errorf("remote provider unavailable")

// RemoteProvider is implemented by the real Paseo bridge adapter. There is no
// callback fallback: a missing provider fails closed before the dispatch
// intent can be sent to an external process.
type RemoteProvider = agentruntime.RemoteProvider

func NewRemoteDispatcher(dispatch *repository.ExecutionDispatchStore) *RemoteDispatcher {
	return &RemoteDispatcher{dispatch: dispatch}
}

func (d *RemoteDispatcher) Dispatch(ctx context.Context, key agentruntime.RunKey, commandID, payloadHash, worker string, lease time.Duration, epoch int64, provider RemoteProvider) (string, error) {
	if provider == nil {
		return "", ErrProviderUnavailable
	}
	record, err := d.dispatch.ClaimDispatchWithPayloadHash(ctx, key, commandID, payloadHash, worker, lease)
	if err != nil {
		return "", err
	}
	if record.State == "completed" || record.State == "reconciled" {
		return record.ExternalID, nil
	}
	if !record.New {
		return "", repository.ErrDispatchBusy
	}
	var externalID string
	request := agentruntime.RemoteStartRequest{Fence: agentruntime.Fence{RunKey: key, Epoch: epoch}, CommandID: commandID, PayloadHash: payloadHash, AttemptID: commandID}
	commandProvider, ok := provider.(agentruntime.RemoteCommandProvider)
	if !ok {
		return "", fmt.Errorf("%w: provider does not support fenced commands", ErrProviderUnavailable)
	}
	externalID, err = commandProvider.StartCommand(ctx, request)
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
