package workbench

import (
	"context"
	"fmt"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/commercial"
)

// RemoteDispatcher is the narrow worker seam around the Paseo bridge. The
// intent is durable before start and an unknown result is persisted on every
// provider/transport error, so callers cannot accidentally retry with a new
// command id.
type RemoteDispatcher struct {
	store *repository.ExecutionDispatchStore
	usage *RemoteUsageService
}

var ErrProviderUnavailable = fmt.Errorf("remote provider unavailable")

// RemoteProvider is implemented by the real Paseo bridge adapter. There is no
// callback fallback: a missing provider fails closed before the dispatch
// intent can be sent to an external process.
type RemoteProvider = agentruntime.RemoteProvider

func NewRemoteDispatcher(dispatch *repository.ExecutionDispatchStore) *RemoteDispatcher {
	return &RemoteDispatcher{store: dispatch}
}

func NewRemoteDispatcherWithUsage(dispatch *repository.ExecutionDispatchStore, usage *RemoteUsageService) *RemoteDispatcher {
	return &RemoteDispatcher{store: dispatch, usage: usage}
}

func (d *RemoteDispatcher) Dispatch(ctx context.Context, key agentruntime.RunKey, commandID, payloadHash, worker string, lease time.Duration, epoch int64, provider RemoteProvider) (string, error) {
	return d.dispatchRemote(ctx, agentruntime.Fence{RunKey: key, Owner: worker, Epoch: epoch}, commandID, payloadHash, worker, lease, provider)
}

func (d *RemoteDispatcher) DispatchFence(ctx context.Context, fence agentruntime.Fence, commandID, payloadHash string, lease time.Duration, provider RemoteProvider) (string, error) {
	return d.dispatchRemote(ctx, fence, commandID, payloadHash, fence.Owner, lease, provider)
}

func (d *RemoteDispatcher) dispatchRemote(ctx context.Context, fence agentruntime.Fence, commandID, payloadHash, worker string, lease time.Duration, provider RemoteProvider) (string, error) {
	key := fence.RunKey
	if provider == nil || d.store == nil || d.usage == nil {
		return "", ErrProviderUnavailable
	}
	record, err := d.store.ClaimDispatchWithPayloadHash(ctx, key, commandID, payloadHash, worker, lease)
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
	if fence.TargetID == "" || fence.WorkspaceRef == "" || fence.Prompt == "" || fence.Provider == "" {
		return d.reconcileClaimed(ctx, record, "fenced_context_missing", fmt.Errorf("%w: fenced command context is incomplete", ErrProviderUnavailable))
	}
	request := agentruntime.RemoteStartRequest{Fence: fence, CommandID: commandID, PayloadHash: payloadHash, AttemptID: commandID, TargetID: fence.TargetID, WorkspaceRef: fence.WorkspaceRef, Prompt: fence.Prompt, Provider: fence.Provider}
	commandProvider, ok := provider.(agentruntime.RemoteCommandProvider)
	if !ok {
		return d.reconcileClaimed(ctx, record, "provider_capability_missing", fmt.Errorf("%w: provider does not support fenced commands", ErrProviderUnavailable))
	}
	usageHandle, err := d.usage.BeginRemote(ctx, fence, commandID)
	if err != nil {
		return d.reconcileClaimed(ctx, record, "usage_begin_failed", err)
	}
	var observation *agentruntime.RemoteUsageObservation
	if usageProvider, ok := provider.(agentruntime.RemoteUsageProvider); ok {
		result, startErr := usageProvider.StartCommandWithUsage(ctx, request)
		externalID, observation, err = result.ExternalID, result.Usage, startErr
	} else {
		// A provider that cannot return a trusted observation may have started
		// the remote process. Preserve the intent as unknown; never synthesize
		// a billable success from an absent usage payload.
		externalID, err = commandProvider.StartCommand(ctx, request)
	}
	if err != nil {
		if reconcileErr := d.store.ReconcileUnknown(ctx, record, "unknown", ""); reconcileErr != nil {
			return "", fmt.Errorf("remote dispatch failed (%v); durable unknown recovery failed: %w", err, reconcileErr)
		}
		return "", err
	}
	if observation == nil {
		return d.reconcileClaimedWithExternal(ctx, record, externalID, "usage_missing", repository.ErrDispatchUnknown)
	}
	if observation.Status == "" || observation.Status == commercial.UsageStatusUnknown || observation.Status == commercial.UsageStatusPartial || observation.Status == commercial.UsageStatusDisplayOnly {
		return d.reconcileClaimedWithExternal(ctx, record, externalID, "usage_unknown", repository.ErrDispatchUnknown)
	}
	if err := d.usage.FinishRemoteObservation(ctx, usageHandle, observation); err != nil {
		if reconcileErr := d.store.ReconcileUnknown(ctx, record, "usage_settlement_failed", externalID); reconcileErr != nil {
			return "", fmt.Errorf("remote usage settlement failed (%v); durable reconciliation failed: %w", err, reconcileErr)
		}
		return "", err
	}
	if err := d.store.SaveReceipt(ctx, record, externalID); err != nil {
		if reconcileErr := d.store.ReconcileUnknown(ctx, record, "receipt_persist_failed", externalID); reconcileErr != nil {
			return "", fmt.Errorf("receipt persistence failed (%v); durable reconciliation failed: %w", err, reconcileErr)
		}
		return "", err
	}
	return externalID, nil
}

func (d *RemoteDispatcher) reconcileClaimed(ctx context.Context, record repository.DispatchRecord, reason string, cause error) (string, error) {
	return d.reconcileClaimedWithExternal(ctx, record, record.ExternalID, reason, cause)
}

func (d *RemoteDispatcher) reconcileClaimedWithExternal(ctx context.Context, record repository.DispatchRecord, externalID, reason string, cause error) (string, error) {
	if err := d.store.ReconcileUnknown(ctx, record, reason, externalID); err != nil {
		return "", fmt.Errorf("%v; durable reconciliation failed: %w", cause, err)
	}
	return "", cause
}
